// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"fmt"
	"time"
)

type ExportOptions struct {
	Format          string        // "vmdk" or "raw"
	OutputPath      string        // Local download path
	GCSBucket       string        // Google Cloud Storage bucket
	GCSPrefix       string        // GCS object prefix
	DownloadFromGCS bool          // Download to local after export
	DeleteFromGCS   bool          // Delete from GCS after download
	CreateImage     bool          // Create image from disk before export
	ImageTimeout    time.Duration // Timeout for image creation
	ShowProgress    bool          // Show progress bars

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
		Format:          "vmdk",
		GCSPrefix:       "exports/",
		DownloadFromGCS: true,
		DeleteFromGCS:   false,
		CreateImage:     true,
		ImageTimeout:    30 * time.Minute,
		ShowProgress:    true,
	}
}

// Validate checks if the export options are valid
func (opts *ExportOptions) Validate() error {
	if opts.Format != "vmdk" && opts.Format != "raw" {
		return fmt.Errorf("format must be 'vmdk' or 'raw', got '%s'", opts.Format)
	}

	if opts.GCSBucket == "" {
		return fmt.Errorf("GCS bucket cannot be empty")
	}

	if opts.OutputPath == "" && opts.DownloadFromGCS {
		return fmt.Errorf("output path cannot be empty when DownloadFromGCS is true")
	}

	if opts.ImageTimeout <= 0 {
		return fmt.Errorf("image timeout must be > 0, got %v", opts.ImageTimeout)
	}

	return nil
}
