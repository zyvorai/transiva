// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"fmt"
	"time"
)

type ExportOptions struct {
	Format         string        // "vhd" (Azure default)
	OutputPath     string        // Local download path
	ContainerURL   string        // Optional blob storage container URL
	CopyToBlob     bool          // Copy VHD to blob storage
	DownloadLocal  bool          // Download VHD to local path
	RevokeAccess   bool          // Revoke disk access after export
	AccessDuration time.Duration // SAS access duration
	ShowProgress   bool          // Show progress bars

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
		Format:         "vhd",
		CopyToBlob:     false,
		DownloadLocal:  true,
		RevokeAccess:   true,
		AccessDuration: 1 * time.Hour,
		ShowProgress:   true,
	}
}

// Validate checks if the export options are valid
func (opts *ExportOptions) Validate() error {
	if opts.Format != "vhd" {
		return fmt.Errorf("format must be 'vhd', got '%s'", opts.Format)
	}

	if opts.OutputPath == "" && opts.DownloadLocal {
		return fmt.Errorf("output path cannot be empty when DownloadLocal is true")
	}

	if opts.CopyToBlob && opts.ContainerURL == "" {
		return fmt.Errorf("container URL cannot be empty when CopyToBlob is true")
	}

	if opts.AccessDuration <= 0 {
		return fmt.Errorf("access duration must be > 0, got %v", opts.AccessDuration)
	}

	return nil
}
