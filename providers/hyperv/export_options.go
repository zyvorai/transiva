// SPDX-License-Identifier: Apache-2.0

package hyperv

import (
	"fmt"
	"time"
)

type ExportOptions struct {
	Format           string        // "vhdx" or "vhd"
	OutputPath       string        // Local export path
	ExportType       string        // "vm" or "vhd-only"
	IncludeSnapshots bool          // Include VM snapshots in export
	ExportTimeout    time.Duration // Timeout for export operation
	ShowProgress     bool          // Show progress bars

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
		Format:           "vhdx",
		ExportType:       "vm",
		IncludeSnapshots: false,
		ExportTimeout:    2 * time.Hour,
		ShowProgress:     true,
	}
}

// Validate checks if the export options are valid
func (opts *ExportOptions) Validate() error {
	if opts.Format != "vhdx" && opts.Format != "vhd" {
		return fmt.Errorf("format must be 'vhdx' or 'vhd', got '%s'", opts.Format)
	}

	if opts.ExportType != "vm" && opts.ExportType != "vhd-only" {
		return fmt.Errorf("export type must be 'vm' or 'vhd-only', got '%s'", opts.ExportType)
	}

	if opts.OutputPath == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	if opts.ExportTimeout <= 0 {
		return fmt.Errorf("export timeout must be > 0, got %v", opts.ExportTimeout)
	}

	return nil
}
