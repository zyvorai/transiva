// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/zyvorai/transiva/progress"
)

// gcpResourceNameRegex matches the safe character set for a GCP resource
// name (images/disks are always lowercase letters, digits, and hyphens).
// Rejecting anything else prevents a crafted identifier from injecting
// extra flags into the gcloud command line.
var gcpResourceNameRegex = regexp.MustCompile(`^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$`)

// ExportImageToGCS exports a GCP disk image to Google Cloud Storage as VMDK
// Uses gcloud CLI for export as the GCP SDK doesn't provide direct VMDK export
func (c *Client) ExportImageToGCS(ctx context.Context, imageName, bucket, outputDir string, reporter progress.ProgressReporter) (*ExportResult, error) {
	c.logger.Info("GCP image export requested", "image", imageName, "bucket", bucket)

	// Check if gcloud CLI is available
	if err := c.checkGcloudCLI(); err != nil {
		return nil, fmt.Errorf("gcloud CLI required for image export: %w", err)
	}

	if reporter != nil {
		reporter.Describe("Exporting image to GCS as VMDK")
	}

	// Validate the image name before it reaches the gcloud command line. GCP
	// resource names are always lowercase letters/digits/hyphens; rejecting
	// anything else prevents a crafted identifier (which may ultimately be
	// influenced by an API caller's VM/instance identifier) from injecting
	// an unexpected gcloud flag.
	if !gcpResourceNameRegex.MatchString(imageName) {
		return nil, fmt.Errorf("invalid GCP image name: %q", imageName)
	}

	// Generate GCS destination path
	objectName := fmt.Sprintf("%s-%d.vmdk", imageName, time.Now().Unix())
	gcsURI := fmt.Sprintf("gs://%s/%s", bucket, objectName)

	// Build gcloud command
	// gcloud compute images export --image=IMAGE_NAME --destination-uri=GCS_URI --export-format=vmdk
	// #nosec G204 -- imageName is validated above against GCP's resource
	// naming pattern; bucket/objectName are internal config/generated values
	// and c.config.ProjectID comes from the local operator's provider config.
	cmd := exec.CommandContext(ctx, "gcloud", "compute", "images", "export",
		"--image="+imageName,
		"--destination-uri="+gcsURI,
		"--export-format=vmdk",
		"--project="+c.config.ProjectID,
	)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.logger.Info("executing gcloud image export",
		"image", imageName,
		"destination", gcsURI)

	// Execute command
	err := cmd.Run()
	if err != nil {
		c.logger.Error("gcloud export failed",
			"error", err,
			"stdout", stdout.String(),
			"stderr", stderr.String())
		return nil, fmt.Errorf("gcloud export failed: %w\nStderr: %s", err, stderr.String())
	}

	c.logger.Info("gcloud export completed", "gcs_uri", gcsURI)

	// Download from GCS to local path if outputDir provided
	var localPath string
	var size int64
	if outputDir != "" {
		if reporter != nil {
			reporter.Describe("Downloading VMDK from GCS")
		}

		localPath = filepath.Join(outputDir, objectName)
		size, err = c.downloadFromGCS(ctx, bucket, objectName, localPath, reporter)
		if err != nil {
			return nil, fmt.Errorf("download from GCS: %w", err)
		}

		c.logger.Info("VMDK downloaded successfully", "path", localPath, "size_bytes", size)
	}

	if reporter != nil {
		reporter.Describe("Image export complete")
		reporter.Update(100)
	}

	return &ExportResult{
		ImageName: imageName,
		Format:    "vmdk",
		LocalPath: localPath,
		Size:      size,
		GCSBucket: bucket,
		GCSObject: objectName,
		GCSURI:    gcsURI,
	}, nil
}

// checkGcloudCLI checks if gcloud CLI is installed and configured
func (c *Client) checkGcloudCLI() error {
	cmd := exec.Command("gcloud", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud CLI not found - please install Google Cloud SDK: %w", err)
	}

	c.logger.Debug("gcloud CLI version", "output", string(output))
	return nil
}

// ExportDiskToGCS exports a persistent disk to GCS as VMDK
func (c *Client) ExportDiskToGCS(ctx context.Context, diskName, bucket, outputDir string, reporter progress.ProgressReporter) (*ExportResult, error) {
	c.logger.Info("starting GCP disk export to GCS", "disk", diskName, "bucket", bucket)

	if reporter != nil {
		reporter.Describe("Creating image from disk")
	}

	// Create image from disk first
	imageName := fmt.Sprintf("%s-image-%d", diskName, time.Now().Unix())

	createImageReq := &computepb.InsertImageRequest{
		Project: c.config.ProjectID,
		ImageResource: &computepb.Image{
			Name: &imageName,
			SourceDisk: makeStringPtr(fmt.Sprintf("projects/%s/zones/%s/disks/%s",
				c.config.ProjectID, c.config.Zone, diskName)),
		},
	}

	op, err := c.imagesClient.Insert(ctx, createImageReq)
	if err != nil {
		return nil, fmt.Errorf("create image from disk: %w", err)
	}

	// Wait for image creation
	err = op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("wait for image creation: %w", err)
	}

	c.logger.Info("image created from disk", "image", imageName, "disk", diskName)

	// Export the image to GCS
	return c.ExportImageToGCS(ctx, imageName, bucket, outputDir, reporter)
}

// ExportInstanceToGCS exports a GCP instance by creating an image and exporting to GCS
func (c *Client) ExportInstanceToGCS(ctx context.Context, instanceName, bucket, outputDir string, reporter progress.ProgressReporter) ([]*ExportResult, error) {
	c.logger.Info("starting GCP instance export to GCS", "instance", instanceName, "bucket", bucket)

	// Get instance to identify disks
	vmInfo, err := c.GetInstance(ctx, instanceName)
	if err != nil {
		return nil, fmt.Errorf("get instance info: %w", err)
	}

	if len(vmInfo.DiskNames) == 0 {
		return nil, fmt.Errorf("instance has no attached disks")
	}

	c.logger.Info("exporting instance disks", "instance", instanceName, "disk_count", len(vmInfo.DiskNames))

	results := make([]*ExportResult, 0, len(vmInfo.DiskNames))

	// Export each disk
	for i, diskName := range vmInfo.DiskNames {
		if reporter != nil {
			reporter.Describe(fmt.Sprintf("Exporting disk %d/%d: %s", i+1, len(vmInfo.DiskNames), diskName))
		}

		result, err := c.ExportDiskToGCS(ctx, diskName, bucket, outputDir, reporter)
		if err != nil {
			c.logger.Error("failed to export disk", "disk", diskName, "error", err)
			// Continue with other disks
			continue
		}

		result.DiskName = diskName
		result.DiskType = "boot"
		if i > 0 {
			result.DiskType = fmt.Sprintf("data-%d", i)
		}
		results = append(results, result)

		c.logger.Info("disk exported", "disk", diskName, "path", result.LocalPath)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("failed to export any disks")
	}

	// Create manifest for multi-disk exports
	if len(results) > 1 {
		if err := CreateExportManifest(instanceName, results, outputDir); err != nil {
			c.logger.Warn("failed to create export manifest", "error", err)
		}
	}

	c.logger.Info("instance export complete", "instance", instanceName, "disks_exported", len(results))
	return results, nil
}

// downloadFromGCS downloads a file from Google Cloud Storage
func (c *Client) downloadFromGCS(ctx context.Context, bucket, object, localPath string, reporter progress.ProgressReporter) (int64, error) {
	// Create GCS client
	var opts []option.ClientOption
	if c.config.CredentialsJSON != "" {
		if _, err := os.Stat(c.config.CredentialsJSON); err == nil {
			opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, c.config.CredentialsJSON))
		} else {
			opts = append(opts, option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(c.config.CredentialsJSON)))
		}
	}

	gcsClient, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return 0, fmt.Errorf("create GCS client: %w", err)
	}
	defer func() { _ = gcsClient.Close() }()

	// Get object attributes to determine size
	bucketHandle := gcsClient.Bucket(bucket)
	objectHandle := bucketHandle.Object(object)

	attrs, err := objectHandle.Attrs(ctx)
	if err != nil {
		return 0, fmt.Errorf("get object attributes: %w", err)
	}

	totalSize := attrs.Size
	c.logger.Info("downloading from GCS", "object", object, "size_bytes", totalSize)

	// Create local file
	if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}

	// #nosec G304 -- localPath is built from the operator-supplied output
	// directory joined with a server-generated object name (see
	// ExportImageToGCS), not taken directly from an HTTP request.
	file, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create reader
	reader, err := objectHandle.NewReader(ctx)
	if err != nil {
		return 0, fmt.Errorf("create GCS reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Create progress reader wrapper if reporter provided
	var ioReader io.Reader = reader
	if reporter != nil && totalSize > 0 {
		ioReader = &progressReader{
			reader:   reader,
			total:    totalSize,
			reporter: reporter,
		}
	}

	// Copy with progress tracking
	written, err := io.Copy(file, ioReader)
	if err != nil {
		return 0, fmt.Errorf("download file: %w", err)
	}

	if written != totalSize {
		return 0, fmt.Errorf("incomplete download: expected %d bytes, got %d", totalSize, written)
	}

	return written, nil
}

// ListGCSObjects lists objects in a GCS bucket with a given prefix
func (c *Client) ListGCSObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	// Create GCS client
	var opts []option.ClientOption
	if c.config.CredentialsJSON != "" {
		if _, err := os.Stat(c.config.CredentialsJSON); err == nil {
			opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, c.config.CredentialsJSON))
		} else {
			opts = append(opts, option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(c.config.CredentialsJSON)))
		}
	}

	gcsClient, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create GCS client: %w", err)
	}
	defer func() { _ = gcsClient.Close() }()

	bucketHandle := gcsClient.Bucket(bucket)

	// List objects
	query := &storage.Query{Prefix: prefix}
	it := bucketHandle.Objects(ctx, query)

	var objects []string
	for {
		attrs, err := it.Next()
		if err == storage.ErrObjectNotExist || (err != nil && strings.Contains(err.Error(), "no more items")) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate objects: %w", err)
		}

		objects = append(objects, attrs.Name)
	}

	return objects, nil
}

// progressReader wraps an io.Reader to report download/upload progress
type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	reporter progress.ProgressReporter
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	if pr.reporter != nil && pr.total > 0 {
		percentage := int((pr.current * 100) / pr.total)
		pr.reporter.Update(int64(percentage))
	}

	return n, err
}

// ExportResult contains the result of a GCS export operation
type ExportResult struct {
	ImageName string
	DiskName  string
	DiskType  string // "boot", "data-1", "data-2", etc.
	Format    string
	LocalPath string
	Size      int64
	GCSBucket string
	GCSObject string
	GCSURI    string
}

// CreateExportManifest creates a JSON manifest file for multi-disk exports
func CreateExportManifest(instanceName string, results []*ExportResult, outputPath string) error {
	manifestPath := filepath.Join(outputPath, fmt.Sprintf("%s-manifest.json", instanceName))

	manifest := fmt.Sprintf(`{
  "instance_name": "%s",
  "export_time": "%s",
  "disk_count": %d,
  "disks": [
`, instanceName, time.Now().Format(time.RFC3339), len(results))

	for i, result := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}
		manifest += fmt.Sprintf(`    {
      "image_name": "%s",
      "disk_name": "%s",
      "disk_type": "%s",
      "format": "%s",
      "local_path": "%s",
      "size_bytes": %d,
      "gcs_bucket": "%s",
      "gcs_object": "%s",
      "gcs_uri": "%s"
    }%s
`, result.ImageName, result.DiskName, result.DiskType, result.Format, result.LocalPath,
			result.Size, result.GCSBucket, result.GCSObject, result.GCSURI, comma)
	}

	manifest += `  ]
}`

	return os.WriteFile(manifestPath, []byte(manifest), 0600)
}

// Helper function to create string pointer
func makeStringPtr(v string) *string {
	return &v
}

// ExportDiskWithOptions exports a GCP persistent disk using ExportOptions
func (c *Client) ExportDiskWithOptions(ctx context.Context, diskName string, opts ExportOptions) (*ExportResult, error) {
	c.logger.Info("starting GCP disk export with options", "disk", diskName)

	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid export options: %w", err)
	}

	// Create image from disk if requested
	var imageName string
	if opts.CreateImage {
		imageName = fmt.Sprintf("%s-image-%d", diskName, time.Now().Unix())

		createImageReq := &computepb.InsertImageRequest{
			Project: c.config.ProjectID,
			ImageResource: &computepb.Image{
				Name: &imageName,
				SourceDisk: makeStringPtr(fmt.Sprintf("projects/%s/zones/%s/disks/%s",
					c.config.ProjectID, c.config.Zone, diskName)),
			},
		}

		op, err := c.imagesClient.Insert(ctx, createImageReq)
		if err != nil {
			return nil, fmt.Errorf("create image from disk: %w", err)
		}

		// Wait for image creation with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, opts.ImageTimeout)
		defer cancel()

		err = op.Wait(timeoutCtx)
		if err != nil {
			return nil, fmt.Errorf("wait for image creation: %w", err)
		}

		c.logger.Info("image created from disk", "image", imageName, "disk", diskName)
	}

	result := &ExportResult{
		ImageName:  imageName,
		DiskName:   diskName,
		Format:     opts.Format,
		GCSBucket:  opts.GCSBucket,
	}

	// Note: Actual export to GCS requires gcloud CLI or Import/Export tools
	// For now, this is a placeholder that would be implemented with those tools

	return result, nil
}

