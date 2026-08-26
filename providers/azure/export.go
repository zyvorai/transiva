// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"

	"github.com/zyvorai/transiva/progress"
	"github.com/zyvorai/transiva/providers/common"
)

// ExportDiskToVHD exports an Azure managed disk to VHD format in blob storage
func (c *Client) ExportDiskToVHD(ctx context.Context, diskName, containerURL, outputDir string, reporter progress.ProgressReporter) (*ExportResult, error) {
	c.logger.Info("starting Azure disk export to VHD", "disk", diskName)

	if reporter != nil {
		reporter.Describe("Granting SAS access to managed disk")
	}

	// Grant read access to the disk (1 hour validity)
	grantAccess := &armcompute.GrantAccessData{
		Access:            to.Ptr(armcompute.AccessLevelRead),
		DurationInSeconds: to.Ptr(int32(3600)), // 1 hour
	}

	pollerAccess, err := c.diskClient.BeginGrantAccess(ctx, c.config.ResourceGroup, diskName, *grantAccess, nil)
	if err != nil {
		return nil, fmt.Errorf("grant disk access: %w", err)
	}

	accessResp, err := pollerAccess.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("wait for disk access: %w", err)
	}

	// AccessURI is a struct type in newer Azure SDK versions
	// Check if AccessSAS field is available
	if accessResp.AccessSAS == nil {
		return nil, fmt.Errorf("failed to get disk SAS URL - AccessSAS is nil")
	}
	sasURL := *accessResp.AccessSAS
	c.logger.Info("disk SAS URL obtained", "disk", diskName)

	// Get disk information
	diskResp, err := c.diskClient.Get(ctx, c.config.ResourceGroup, diskName, nil)
	if err != nil {
		return nil, fmt.Errorf("get disk info: %w", err)
	}

	diskSizeGB := int64(0)
	if diskResp.Properties != nil && diskResp.Properties.DiskSizeGB != nil {
		diskSizeGB = int64(*diskResp.Properties.DiskSizeGB)
	}

	// Copy VHD to blob storage if container URL provided
	var blobURL string
	if containerURL != "" {
		if reporter != nil {
			reporter.Describe("Copying VHD to blob storage")
		}

		blobName := fmt.Sprintf("%s-%d.vhd", diskName, time.Now().Unix())
		blobURL, err = c.copyVHDToBlob(ctx, sasURL, containerURL, blobName, reporter)
		if err != nil {
			return nil, fmt.Errorf("copy VHD to blob: %w", err)
		}

		c.logger.Info("VHD copied to blob storage", "blob", blobURL)
	}

	// Download VHD to local path
	if reporter != nil {
		reporter.Describe("Downloading VHD from Azure")
	}

	localPath := filepath.Join(outputDir, fmt.Sprintf("%s.vhd", diskName))
	size, err := c.downloadVHD(ctx, sasURL, localPath, reporter)
	if err != nil {
		return nil, fmt.Errorf("download VHD: %w", err)
	}

	// Revoke access to disk
	pollerRevoke, err := c.diskClient.BeginRevokeAccess(ctx, c.config.ResourceGroup, diskName, nil)
	if err != nil {
		c.logger.Warn("failed to revoke disk access", "error", err)
	} else {
		if _, err := pollerRevoke.PollUntilDone(ctx, nil); err != nil {
			c.logger.Warn("failed to wait for disk access revocation", "disk", diskName, "error", err)
		} else {
			c.logger.Info("disk access revoked", "disk", diskName)
		}
	}

	if reporter != nil {
		reporter.Describe("VHD export complete")
		reporter.Update(100)
	}

	return &ExportResult{
		DiskName:   diskName,
		Format:     "vhd",
		LocalPath:  localPath,
		Size:       size,
		BlobURL:    blobURL,
		DiskSizeGB: diskSizeGB,
	}, nil
}

// ExportVMToVHD exports all disks of an Azure VM to VHD format
func (c *Client) ExportVMToVHD(ctx context.Context, vmName, containerURL, outputDir string, reporter progress.ProgressReporter) ([]*ExportResult, error) {
	c.logger.Info("starting Azure VM export to VHD", "vm", vmName)

	// Get VM information to identify disks
	vmInfo, err := c.GetVM(ctx, vmName)
	if err != nil {
		return nil, fmt.Errorf("get VM info: %w", err)
	}

	if len(vmInfo.DiskNames) == 0 {
		return nil, fmt.Errorf("VM has no attached disks")
	}

	c.logger.Info("exporting VM disks", "vm", vmName, "disk_count", len(vmInfo.DiskNames))

	results := make([]*ExportResult, 0, len(vmInfo.DiskNames))

	// Export each disk
	for i, diskName := range vmInfo.DiskNames {
		if reporter != nil {
			reporter.Describe(fmt.Sprintf("Exporting disk %d/%d: %s", i+1, len(vmInfo.DiskNames), diskName))
		}

		result, err := c.ExportDiskToVHD(ctx, diskName, containerURL, outputDir, reporter)
		if err != nil {
			c.logger.Error("failed to export disk", "disk", diskName, "error", err)
			// Continue with other disks even if one fails
			continue
		}

		result.DiskType = "OS"
		if i > 0 {
			result.DiskType = fmt.Sprintf("Data-%d", i)
		}
		results = append(results, result)

		c.logger.Info("disk exported", "disk", diskName, "path", result.LocalPath)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("failed to export any disks")
	}

	c.logger.Info("VM export complete", "vm", vmName, "disks_exported", len(results))
	return results, nil
}

// copyVHDToBlob copies VHD from SAS URL to blob storage using Azure SDK v2
func (c *Client) copyVHDToBlob(ctx context.Context, sasURL, containerURL, blobName string, reporter progress.ProgressReporter) (string, error) {
	c.logger.Info("copying VHD to blob storage", "blob", blobName)

	// Parse container URL
	containerURLParsed, err := url.Parse(containerURL)
	if err != nil {
		return "", fmt.Errorf("parse container URL: %w", err)
	}

	// Construct full blob URL
	blobURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(containerURL, "/"), blobName)

	// Create blob client
	// Note: In production, you would use proper credential management
	// For now, we assume the containerURL contains the necessary credentials
	blobClient, err := azblob.NewClientWithNoCredential(containerURLParsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create blob client: %w", err)
	}

	// Get the block blob client for this specific blob
	blockBlobClient := blobClient.ServiceClient().NewContainerClient(containerURLParsed.Path).NewBlockBlobClient(blobName)

	// Start copy operation from source SAS URL
	if reporter != nil {
		reporter.Describe("Starting blob copy operation")
	}

	copyResp, err := blockBlobClient.StartCopyFromURL(ctx, sasURL, &blob.StartCopyFromURLOptions{
		Tier: to.Ptr(blob.AccessTierCool), // Use cool tier for cost savings
	})
	if err != nil {
		return "", fmt.Errorf("start copy from URL: %w", err)
	}

	c.logger.Info("blob copy started", "copy_id", *copyResp.CopyID)

	// Poll copy status until complete
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(24 * time.Hour) // VHD copies can take hours

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("blob copy timed out after 24 hours")
		case <-ticker.C:
			// Get blob properties to check copy status
			props, err := blockBlobClient.GetProperties(ctx, nil)
			if err != nil {
				return "", fmt.Errorf("get blob properties: %w", err)
			}

			// Check copy status
			if props.CopyStatus == nil {
				return "", fmt.Errorf("copy status is nil")
			}

			status := *props.CopyStatus
			c.logger.Debug("blob copy status", "status", status)

			// Update progress if available
			if reporter != nil && props.CopyProgress != nil {
				// Parse copy progress (format: "bytes copied/total bytes")
				reporter.Describe(fmt.Sprintf("Copy progress: %s", *props.CopyProgress))
			}

			switch status {
			case blob.CopyStatusTypeSuccess:
				c.logger.Info("blob copy completed successfully")
				if reporter != nil {
					reporter.Describe("Blob copy complete")
				}
				return blobURL, nil

			case blob.CopyStatusTypeFailed:
				statusDescription := "unknown"
				if props.CopyStatusDescription != nil {
					statusDescription = *props.CopyStatusDescription
				}
				return "", fmt.Errorf("blob copy failed: %s", statusDescription)

			case blob.CopyStatusTypeAborted:
				return "", fmt.Errorf("blob copy was aborted")

			case blob.CopyStatusTypePending:
				// Still in progress, continue polling
				continue

			default:
				c.logger.Warn("unknown copy status", "status", status)
				continue
			}
		}
	}
}

// downloadVHD downloads VHD from SAS URL to local file
func (c *Client) downloadVHD(ctx context.Context, sasURL, localPath string, reporter progress.ProgressReporter) (int64, error) {
	// Create output directory
	if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}

	// Create local file
	// #nosec G304 -- localPath is derived from operator-supplied outputDir + diskName CLI/config input, not remote HTTP data
	file, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.logger.Warn("failed to close local VHD file", "error", closeErr)
		}
	}()

	// Download VHD
	httpClient := &http.Client{
		Timeout: 24 * time.Hour, // Large downloads can take hours
	}

	req, err := http.NewRequestWithContext(ctx, "GET", sasURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create download request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download VHD: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close VHD download response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	c.logger.Info("downloading VHD", "size_bytes", totalSize)

	// Create progress reader wrapper if reporter provided
	var reader io.Reader = resp.Body
	if reporter != nil && totalSize > 0 {
		reader = &progressReader{
			reader:   resp.Body,
			total:    totalSize,
			reporter: reporter,
		}
	}

	// Copy with progress tracking
	written, err := io.Copy(file, reader)
	if err != nil {
		return 0, fmt.Errorf("write VHD to disk: %w", err)
	}

	return written, nil
}

// ExportSnapshotToVHD exports an Azure snapshot to VHD format
func (c *Client) ExportSnapshotToVHD(ctx context.Context, snapshotName, containerURL, outputDir string, reporter progress.ProgressReporter) (*ExportResult, error) {
	c.logger.Info("starting Azure snapshot export to VHD", "snapshot", snapshotName)

	if reporter != nil {
		reporter.Describe("Granting SAS access to snapshot")
	}

	// Grant read access to the snapshot (1 hour validity)
	grantAccess := &armcompute.GrantAccessData{
		Access:            to.Ptr(armcompute.AccessLevelRead),
		DurationInSeconds: to.Ptr(int32(3600)), // 1 hour
	}

	pollerAccess, err := c.snapshotClient.BeginGrantAccess(ctx, c.config.ResourceGroup, snapshotName, *grantAccess, nil)
	if err != nil {
		return nil, fmt.Errorf("grant snapshot access: %w", err)
	}

	accessResp, err := pollerAccess.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("wait for snapshot access: %w", err)
	}

	// AccessURI is a struct type in newer Azure SDK versions
	if accessResp.AccessSAS == nil {
		return nil, fmt.Errorf("failed to get snapshot SAS URL - AccessSAS is nil")
	}
	sasURL := *accessResp.AccessSAS
	c.logger.Info("snapshot SAS URL obtained", "snapshot", snapshotName)

	// Get snapshot information
	snapshotResp, err := c.snapshotClient.Get(ctx, c.config.ResourceGroup, snapshotName, nil)
	if err != nil {
		return nil, fmt.Errorf("get snapshot info: %w", err)
	}

	snapshotSizeGB := int64(0)
	if snapshotResp.Properties != nil && snapshotResp.Properties.DiskSizeGB != nil {
		snapshotSizeGB = int64(*snapshotResp.Properties.DiskSizeGB)
	}

	// Copy VHD to blob storage if container URL provided
	var blobURL string
	if containerURL != "" {
		if reporter != nil {
			reporter.Describe("Copying VHD to blob storage")
		}

		blobName := fmt.Sprintf("%s-%d.vhd", snapshotName, time.Now().Unix())
		blobURL, err = c.copyVHDToBlob(ctx, sasURL, containerURL, blobName, reporter)
		if err != nil {
			return nil, fmt.Errorf("copy VHD to blob: %w", err)
		}

		c.logger.Info("VHD copied to blob storage", "blob", blobURL)
	}

	// Download VHD to local path
	if reporter != nil {
		reporter.Describe("Downloading VHD from Azure")
	}

	localPath := filepath.Join(outputDir, fmt.Sprintf("%s.vhd", snapshotName))
	size, err := c.downloadVHD(ctx, sasURL, localPath, reporter)
	if err != nil {
		return nil, fmt.Errorf("download VHD: %w", err)
	}

	// Revoke access to snapshot
	pollerRevoke, err := c.snapshotClient.BeginRevokeAccess(ctx, c.config.ResourceGroup, snapshotName, nil)
	if err != nil {
		c.logger.Warn("failed to revoke snapshot access", "error", err)
	} else {
		if _, err := pollerRevoke.PollUntilDone(ctx, nil); err != nil {
			c.logger.Warn("failed to wait for snapshot access revocation", "snapshot", snapshotName, "error", err)
		} else {
			c.logger.Info("snapshot access revoked", "snapshot", snapshotName)
		}
	}

	if reporter != nil {
		reporter.Describe("Snapshot export complete")
		reporter.Update(100)
	}

	return &ExportResult{
		DiskName:   snapshotName,
		DiskType:   "snapshot",
		Format:     "vhd",
		LocalPath:  localPath,
		Size:       size,
		BlobURL:    blobURL,
		DiskSizeGB: snapshotSizeGB,
	}, nil
}

// progressReader wraps an io.Reader to report download progress
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
		percentage := (pr.current * 100) / pr.total
		pr.reporter.Update(percentage)
	}

	return n, err
}

// ExportResult contains the result of a VHD export operation
type ExportResult struct {
	DiskName   string
	DiskType   string // "OS", "Data-1", "Data-2", etc.
	Format     string
	LocalPath  string
	Size       int64
	BlobURL    string
	DiskSizeGB int64
}

// CreateExportManifest creates a JSON manifest file for multi-disk exports
func CreateExportManifest(vmName string, results []*ExportResult, outputPath string) error {
	manifestPath := filepath.Join(outputPath, fmt.Sprintf("%s-manifest.json", vmName))

	manifest := fmt.Sprintf(`{
  "vm_name": "%s",
  "export_time": "%s",
  "disk_count": %d,
  "disks": [
`, vmName, time.Now().Format(time.RFC3339), len(results))

	for i, result := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}
		manifest += fmt.Sprintf(`    {
      "disk_name": "%s",
      "disk_type": "%s",
      "format": "%s",
      "local_path": "%s",
      "size_bytes": %d,
      "disk_size_gb": %d,
      "blob_url": "%s"
    }%s
`, result.DiskName, result.DiskType, result.Format, result.LocalPath,
			result.Size, result.DiskSizeGB, result.BlobURL, comma)
	}

	manifest += `  ]
}`

	return os.WriteFile(manifestPath, []byte(manifest), 0600)
}

// ExportDiskWithOptions exports an Azure managed disk using ExportOptions
func (c *Client) ExportDiskWithOptions(ctx context.Context, diskName string, opts ExportOptions) (*ExportResult, error) {
	c.logger.Info("starting Azure disk export with options", "disk", diskName)

	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid export options: %w", err)
	}

	// Grant read access to the disk
	grantAccess := &armcompute.GrantAccessData{
		Access:            to.Ptr(armcompute.AccessLevelRead),
		DurationInSeconds: to.Ptr(int32(opts.AccessDuration.Seconds())),
	}

	pollerAccess, err := c.diskClient.BeginGrantAccess(ctx, c.config.ResourceGroup, diskName, *grantAccess, nil)
	if err != nil {
		return nil, fmt.Errorf("grant disk access: %w", err)
	}

	accessResp, err := pollerAccess.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("wait for disk access: %w", err)
	}

	if accessResp.AccessSAS == nil {
		return nil, fmt.Errorf("failed to get disk SAS URL - AccessSAS is nil")
	}
	sasURL := *accessResp.AccessSAS
	c.logger.Info("disk SAS URL obtained", "disk", diskName)

	// Get disk information
	diskResp, err := c.diskClient.Get(ctx, c.config.ResourceGroup, diskName, nil)
	if err != nil {
		return nil, fmt.Errorf("get disk info: %w", err)
	}

	diskSizeGB := int64(0)
	if diskResp.Properties != nil && diskResp.Properties.DiskSizeGB != nil {
		diskSizeGB = int64(*diskResp.Properties.DiskSizeGB)
	}

	result := &ExportResult{
		DiskName:   diskName,
		Format:     opts.Format,
		DiskSizeGB: diskSizeGB,
	}

	// Download VHD to local path if requested
	if opts.DownloadLocal {
		localPath := filepath.Join(opts.OutputPath, fmt.Sprintf("%s.vhd", diskName))
		size, err := c.downloadVHDWithOptions(ctx, sasURL, localPath, opts)
		if err != nil {
			return nil, fmt.Errorf("download VHD: %w", err)
		}

		result.LocalPath = localPath
		result.Size = size

		c.logger.Info("VHD downloaded successfully", "path", localPath, "size_bytes", size)
	}

	// Revoke access to disk if requested
	if opts.RevokeAccess {
		pollerRevoke, err := c.diskClient.BeginRevokeAccess(ctx, c.config.ResourceGroup, diskName, nil)
		if err != nil {
			c.logger.Warn("failed to revoke disk access", "error", err)
		} else {
			if _, err := pollerRevoke.PollUntilDone(ctx, nil); err != nil {
				c.logger.Warn("failed to wait for disk access revocation", "disk", diskName, "error", err)
			} else {
				c.logger.Info("disk access revoked", "disk", diskName)
			}
		}
	}

	return result, nil
}

// downloadVHDWithOptions downloads VHD with progress callback support
func (c *Client) downloadVHDWithOptions(ctx context.Context, sasURL, localPath string, opts ExportOptions) (int64, error) {
	// Create output directory
	if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}

	// Create local file
	// #nosec G304 -- localPath is derived from operator-supplied opts.OutputPath + diskName CLI/config input, not remote HTTP data
	file, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.logger.Warn("failed to close local VHD file", "error", closeErr)
		}
	}()

	// Download VHD
	httpClient := &http.Client{
		Timeout: 24 * time.Hour,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", sasURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create download request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download VHD: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close VHD download response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	c.logger.Info("downloading VHD", "size_bytes", totalSize)

	// Create progress reader wrapper with callback if provided
	var reader io.Reader = resp.Body
	if opts.ProgressCallback != nil && totalSize > 0 {
		var currentBytes int64
		reader = &callbackProgressReader{
			reader:       resp.Body,
			total:        totalSize,
			currentBytes: &currentBytes,
			callback:     opts.ProgressCallback,
			fileName:     filepath.Base(localPath),
			fileIndex:    1,
			totalFiles:   1,
		}
	}

	// Apply bandwidth throttling if enabled
	if opts.BandwidthLimit > 0 {
		reader = common.NewThrottledReaderWithContext(ctx, reader, opts.BandwidthLimit, opts.BandwidthBurst)
	}

	// Copy with progress tracking
	written, err := io.Copy(file, reader)
	if err != nil {
		return 0, fmt.Errorf("write VHD to disk: %w", err)
	}

	return written, nil
}

// callbackProgressReader wraps an io.Reader to call progress callback
type callbackProgressReader struct {
	reader       io.Reader
	total        int64
	currentBytes *int64
	callback     func(current, total int64, fileName string, fileIndex, totalFiles int)
	fileName     string
	fileIndex    int
	totalFiles   int
}

func (cpr *callbackProgressReader) Read(p []byte) (int, error) {
	n, err := cpr.reader.Read(p)

	// Atomically update current bytes
	current := atomic.AddInt64(cpr.currentBytes, int64(n))

	// Call progress callback
	if cpr.callback != nil {
		cpr.callback(current, cpr.total, cpr.fileName, cpr.fileIndex, cpr.totalFiles)
	}

	return n, err
}
