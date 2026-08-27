// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"fmt"
	"time"
)

type ExportOptions struct {
	Format                 string // "ovf" or "ova"
	OutputPath             string
	RemoveCDROM            bool
	ShutdownTimeout        time.Duration
	ParallelDownloads      int
	ValidateChecksum       bool // renamed from Validate to avoid conflict with Validate() method
	ShowIndividualProgress bool
	ShowOverallProgress    bool
	CleanupOVF             bool // Remove OVF files after OVA creation
	Compress               bool // Enable gzip compression for OVA
	CompressionLevel       int  // Gzip compression level (0-9, default 6)

	// Artifact Manifest v1.0 options
	GenerateManifest        bool   // Generate Artifact Manifest v1.0
	VerifyManifest          bool   // Verify manifest after generation
	ManifestComputeChecksum bool   // Compute SHA-256 checksums for all disks
	ManifestTargetFormat    string // Target format for h2kvm conversion (e.g., "qcow2")

	// Pipeline integration options
	EnablePipeline         bool          // Enable h2kvm pipeline after export
	H2KVMPath          string        // Path to h2kvm binary (auto-detect if empty)
	PipelineTimeout        time.Duration // Timeout for pipeline process
	StreamPipelineOutput   bool          // Stream h2kvm output to console
	PipelineDryRun         bool          // Run h2kvm in dry-run mode

	// Pipeline stage configuration
	PipelineInspect        bool          // Enable INSPECT stage (collect guest info)
	PipelineFix            bool          // Enable FIX stage (fix fstab, grub, initramfs)
	PipelineConvert        bool          // Enable CONVERT stage (convert to qcow2)
	PipelineValidate       bool          // Enable VALIDATE stage (check image integrity)
	PipelineCompress       bool          // Enable qcow2 compression
	PipelineCompressLevel  int           // Compression level 1-9 (default 6)

	// EnableRDP enables Windows RDP in h2kvm firstboot (nil = auto: true for Windows guests).
	EnableRDP *bool

	// Libvirt integration options
	LibvirtIntegration     bool          // Enable libvirt VM definition after conversion
	LibvirtURI             string        // Libvirt connection URI (default: qemu:///system)
	LibvirtAutoStart       bool          // Enable VM auto-start in libvirt
	LibvirtNetworkBridge   string        // Network bridge (default: virbr0)
	LibvirtStoragePool     string        // Storage pool (default: default)

	// h2kvm daemon options
	H2KVMDaemon        bool          // Use systemd daemon instead of direct execution
	H2KVMInstance      string        // Systemd instance name (e.g., "vsphere-prod")
	H2KVMWatchDir      string        // Watch directory for daemon mode (default: /var/lib/h2kvm/queue)
	H2KVMOutputDir     string        // Output directory for daemon mode (default: /var/lib/h2kvm/output)
	H2KVMPollInterval  int           // Poll interval in seconds (default: 5)
	H2KVMDaemonTimeout int           // Daemon timeout in minutes (default: 60)

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
		Format:                 "ovf",
		RemoveCDROM:            true,
		ShutdownTimeout:        5 * time.Minute,
		ParallelDownloads:      3,
		ValidateChecksum:       true,
		ShowIndividualProgress: false,
		ShowOverallProgress:    true,

		// Pipeline defaults
		EnablePipeline:        false,
		PipelineTimeout:       30 * time.Minute,
		StreamPipelineOutput:  true,
		PipelineInspect:       true,
		PipelineFix:           true,
		PipelineConvert:       true,
		PipelineValidate:      true,
		PipelineCompress:       true,
		PipelineCompressLevel:  6,
		LibvirtURI:             "qemu:///system",
		LibvirtNetworkBridge:   "virbr0",
		LibvirtStoragePool:     "default",

		// Daemon defaults
		H2KVMDaemon:        false,
		H2KVMWatchDir:      "/var/lib/h2kvm/queue",
		H2KVMOutputDir:     "/var/lib/h2kvm/output",
		H2KVMPollInterval:  5,
		H2KVMDaemonTimeout: 60,
	}
}

// Validate checks if the export options are valid
func (opts *ExportOptions) Validate() error {
	if opts.ParallelDownloads <= 0 {
		return fmt.Errorf("parallel downloads must be > 0, got %d", opts.ParallelDownloads)
	}

	if opts.ParallelDownloads > 16 {
		return fmt.Errorf("parallel downloads must be <= 16, got %d", opts.ParallelDownloads)
	}

	if opts.Format != "ovf" && opts.Format != "ova" {
		return fmt.Errorf("format must be 'ovf' or 'ova', got '%s'", opts.Format)
	}

	if opts.OutputPath == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	return nil
}
