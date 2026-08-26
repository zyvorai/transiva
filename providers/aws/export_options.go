// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"fmt"
	"time"
)

type ExportOptions struct {
	Format                 string        // "vmdk" (AWS default)
	OutputPath             string        // Local download path
	S3Bucket               string        // S3 bucket for export task
	S3Prefix               string        // S3 prefix for exported files
	ExportTimeout          time.Duration // Timeout for export task
	DownloadFromS3         bool          // Download to local after export
	DeleteFromS3AfterDownload bool       // Clean up S3 after download
	ShowProgress           bool          // Show progress bars

	// Progress callback for TUI/API integration
	ProgressCallback func(current, total int64, fileName string, fileIndex, totalFiles int)

	// Bandwidth throttling
	BandwidthLimit int64 // bytes per second (0 = unlimited)
	BandwidthBurst int   // burst size in bytes (0 = auto)

	// Export resumption
	EnableCheckpoints    bool          // Enable checkpoint-based resumption
	CheckpointInterval   time.Duration // How often to save checkpoints (0 = after each file)
	ResumeFromCheckpoint bool          // Resume from existing checkpoint if found
	CheckpointPath       string        // Custom checkpoint file path (empty = auto)
}

func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		Format:                    "vmdk",
		S3Prefix:                  "exports/instances/",
		ExportTimeout:             2 * time.Hour,
		DownloadFromS3:            true,
		DeleteFromS3AfterDownload: false,
		ShowProgress:              true,
	}
}

// Validate checks if the export options are valid
func (opts *ExportOptions) Validate() error {
	if opts.Format != "vmdk" {
		return fmt.Errorf("format must be 'vmdk', got '%s'", opts.Format)
	}

	if opts.S3Bucket == "" {
		return fmt.Errorf("S3 bucket cannot be empty")
	}

	if opts.OutputPath == "" && opts.DownloadFromS3 {
		return fmt.Errorf("output path cannot be empty when DownloadFromS3 is true")
	}

	if opts.ExportTimeout <= 0 {
		return fmt.Errorf("export timeout must be > 0, got %v", opts.ExportTimeout)
	}

	return nil
}
