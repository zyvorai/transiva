// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"

	"github.com/zyvorai/transiva/progress"
	"github.com/zyvorai/transiva/retry"
)

// ExportOptions contains options for exporting an OpenStack instance
type ExportOptions struct {
	OutputDir         string                    // Local output directory
	Format            string                    // Export format: qcow2, vmdk, raw
	SnapshotName      string                    // Custom snapshot name
	UploadToSwift     bool                      // Upload to Swift storage
	Container         string                    // Swift container name
	DeleteAfterExport bool                      // Delete snapshot after export
	ProgressReporter  progress.ProgressReporter // Progress reporter
}

// ExportResult contains the result of an export operation
type ExportResult struct {
	InstanceID   string
	InstanceName string
	ImageID      string
	ImageName    string
	ExportFormat string
	LocalPath    string
	SwiftURL     string
	Size         int64
	Duration     time.Duration
}

// ExportInstance exports an OpenStack instance to a snapshot and downloads it
func (c *Client) ExportInstance(ctx context.Context, instanceID string, opts *ExportOptions) (*ExportResult, error) {
	startTime := time.Now()

	c.logger.Info("starting OpenStack instance export",
		"instance_id", instanceID,
		"format", opts.Format)

	// Step 1: Get instance details
	c.logger.Info("retrieving instance details", "instance_id", instanceID)
	instance, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}

	// Step 2: Create snapshot
	snapshotName := opts.SnapshotName
	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-export-%d", instance.Name, time.Now().Unix())
	}

	c.logger.Info("creating snapshot", "snapshot_name", snapshotName)
	imageID, err := c.CreateSnapshot(ctx, instanceID, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	result := &ExportResult{
		InstanceID:   instanceID,
		InstanceName: instance.Name,
		ImageID:      imageID,
		ImageName:    snapshotName,
		ExportFormat: opts.Format,
	}

	// Cleanup snapshot if requested
	if opts.DeleteAfterExport {
		defer func() {
			c.logger.Info("deleting snapshot", "image_id", imageID)
			if err := c.DeleteImage(context.Background(), imageID); err != nil {
				c.logger.Error("failed to delete snapshot", "image_id", imageID, "error", err)
			}
		}()
	}

	// Step 3: Wait for snapshot to be active
	c.logger.Info("waiting for snapshot to be active", "image_id", imageID)
	if err := c.WaitForImageStatus(ctx, imageID, "active", 30*time.Minute); err != nil {
		return nil, fmt.Errorf("wait for snapshot: %w", err)
	}

	// Step 4: Download snapshot if output directory specified
	if opts.OutputDir != "" {
		c.logger.Info("downloading snapshot to local filesystem", "output_dir", opts.OutputDir)

		localPath, size, err := c.downloadImage(ctx, imageID, opts)
		if err != nil {
			return nil, fmt.Errorf("download image: %w", err)
		}

		result.LocalPath = localPath
		result.Size = size
	}

	result.Duration = time.Since(startTime)

	c.logger.Info("instance export completed",
		"instance_id", instanceID,
		"duration", result.Duration,
		"size", result.Size)

	return result, nil
}

// downloadImage downloads an image from Glance
func (c *Client) downloadImage(ctx context.Context, imageID string, opts *ExportOptions) (string, int64, error) {
	// Get image details
	image, err := images.Get(c.imageClient, imageID).Extract()
	if err != nil {
		return "", 0, fmt.Errorf("get image: %w", err)
	}

	// Determine file extension based on format
	ext := opts.Format
	if ext == "" {
		if image.DiskFormat != "" {
			ext = image.DiskFormat
		} else {
			ext = "qcow2"
		}
	}

	localPath := filepath.Join(opts.OutputDir, fmt.Sprintf("%s.%s", opts.SnapshotName, ext))

	c.logger.Info("downloading image",
		"image_id", imageID,
		"local_path", localPath,
		"size", image.SizeBytes)

	size, err := c.downloadFromGlance(ctx, imageID, localPath, image.SizeBytes, opts.ProgressReporter)
	if err != nil {
		return "", 0, fmt.Errorf("download from glance: %w", err)
	}

	c.logger.Info("image downloaded", "path", localPath, "size", size)

	return localPath, size, nil
}

// downloadFromGlance downloads an image from Glance image service
func (c *Client) downloadFromGlance(ctx context.Context, imageID, localPath string, expectedSize int64, reporter progress.ProgressReporter) (int64, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying download", "image_id", imageID, "attempt", attempt)
		}

		// Download image data
		reader, err := imagedata.Download(c.imageClient, imageID).Extract()
		if err != nil {
			return nil, fmt.Errorf("download image: %w", err)
		}
		defer func() {
			if closeErr := reader.Close(); closeErr != nil {
				c.logger.Warn("failed to close glance image reader", "error", closeErr)
			}
		}()

		// Create output file. localPath is built from the export options' OutputDir/SnapshotName,
		// which are supplied by the local operator invoking the export, not remote input.
		// #nosec G304 -- localPath is derived from operator-supplied ExportOptions, not remote input
		file, err := os.Create(localPath)
		if err != nil {
			return nil, retry.IsNonRetryable(fmt.Errorf("create file: %w", err))
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				c.logger.Warn("failed to close downloaded image file", "error", closeErr)
			}
		}()

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := reader.Read(buf)
			if nr > 0 {
				nw, ew := file.Write(buf[0:nr])
				if nw > 0 {
					written += int64(nw)
					if reporter != nil && expectedSize > 0 {
						pct := int64(float64(written) / float64(expectedSize) * 100)
						reporter.Update(pct)
					}
				}
				if ew != nil {
					return nil, retry.IsNonRetryable(fmt.Errorf("write file: %w", ew))
				}
			}
			if er != nil {
				if er != io.EOF {
					return nil, fmt.Errorf("read from glance: %w", er)
				}
				break
			}
		}

		return written, nil
	}, fmt.Sprintf("download image %s", imageID))

	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// UploadImage uploads an image to Glance
func (c *Client) UploadImage(ctx context.Context, localPath, imageName string, properties map[string]string) (string, error) {
	// Open file. localPath is supplied by the local operator invoking the upload, not remote input.
	// #nosec G304 -- localPath is caller-supplied by the local operator, not remote input
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			c.logger.Warn("failed to close file", "error", cerr)
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}

	// Determine disk format from file extension
	diskFormat := "qcow2"
	ext := filepath.Ext(localPath)
	if len(ext) > 1 {
		diskFormat = ext[1:] // Remove leading dot
	}

	// Create image
	createOpts := images.CreateOpts{
		Name:            imageName,
		DiskFormat:      diskFormat,
		ContainerFormat: "bare",
		Properties:      properties,
	}

	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying create image", "image_name", imageName, "attempt", attempt)
		}

		image, err := images.Create(c.imageClient, createOpts).Extract()
		if err != nil {
			return nil, fmt.Errorf("create image: %w", err)
		}

		c.logger.Info("image created",
			"image_id", image.ID,
			"image_name", imageName,
			"size", fileInfo.Size())

		// Upload image data
		if err := imagedata.Upload(c.imageClient, image.ID, file).ExtractErr(); err != nil {
			// Cleanup failed image
			images.Delete(c.imageClient, image.ID)
			return nil, fmt.Errorf("upload image data: %w", err)
		}

		c.logger.Info("image uploaded", "image_id", image.ID)
		return image.ID, nil
	}, fmt.Sprintf("upload image %s", imageName))

	if err != nil {
		return "", err
	}

	return result.(string), nil
}
