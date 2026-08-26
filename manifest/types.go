// SPDX-License-Identifier: Apache-2.0

// Package manifest implements the Artifact Manifest v1.0 integration contract
// between transiva and hyper2kvm. This provides a versioned, structured format
// for describing VM disk artifacts and migration pipeline configuration.
package manifest

import (
	"time"
)

// ArtifactManifest represents the complete Artifact Manifest v1.0 structure.
// This is the integration contract between transiva (export/fetch) and
// hyper2kvm (offline fix/convert).
type ArtifactManifest struct {
	// ManifestVersion is the semantic version of this manifest schema (REQUIRED)
	// Current version: "1.0"
	ManifestVersion string `json:"manifest_version" yaml:"manifest_version"`

	// Source contains metadata about where the VM came from (OPTIONAL but recommended)
	Source *SourceMetadata `json:"source,omitempty" yaml:"source,omitempty"`

	// VM contains VM hardware and firmware metadata (OPTIONAL but recommended)
	VM *VMMetadata `json:"vm,omitempty" yaml:"vm,omitempty"`

	// Disks contains the disk artifacts (REQUIRED - must have at least one)
	Disks []DiskArtifact `json:"disks" yaml:"disks"`

	// NICs contains network interface information (OPTIONAL)
	NICs []NICInfo `json:"nics,omitempty" yaml:"nics,omitempty"`

	// Notes contains informational notes from the export process (OPTIONAL)
	Notes []string `json:"notes,omitempty" yaml:"notes,omitempty"`

	// Warnings contains non-fatal warnings from the export process (OPTIONAL)
	Warnings []Warning `json:"warnings,omitempty" yaml:"warnings,omitempty"`

	// Metadata contains additional metadata (OPTIONAL)
	Metadata *ManifestMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Pipeline contains pipeline stage configuration (OPTIONAL)
	Pipeline *PipelineConfig `json:"pipeline,omitempty" yaml:"pipeline,omitempty"`

	// Configuration contains guest OS configuration injection (OPTIONAL)
	Configuration *GuestConfiguration `json:"configuration,omitempty" yaml:"configuration,omitempty"`

	// Output contains output configuration (OPTIONAL)
	Output *OutputConfig `json:"output,omitempty" yaml:"output,omitempty"`

	// Options contains global runtime options (OPTIONAL)
	Options *RuntimeOptions `json:"options,omitempty" yaml:"options,omitempty"`
}

// SourceMetadata contains information about the source system
type SourceMetadata struct {
	// Provider identifies the source provider (vsphere, aws, azure, gcp, hyperv, etc.)
	Provider string `json:"provider" yaml:"provider"`

	// VMID is the provider-specific VM identifier
	VMID string `json:"vm_id,omitempty" yaml:"vm_id,omitempty"`

	// VMName is the human-readable VM name
	VMName string `json:"vm_name,omitempty" yaml:"vm_name,omitempty"`

	// Datacenter is the datacenter or region
	Datacenter string `json:"datacenter,omitempty" yaml:"datacenter,omitempty"`

	// ExportTimestamp is when the export/fetch occurred (ISO 8601)
	ExportTimestamp *time.Time `json:"export_timestamp,omitempty" yaml:"export_timestamp,omitempty"`

	// ExportMethod describes how artifacts were obtained
	// Examples: "govc-export", "ovftool", "snapshot", "direct-download"
	ExportMethod string `json:"export_method,omitempty" yaml:"export_method,omitempty"`
}

// VMMetadata contains VM hardware and firmware metadata
type VMMetadata struct {
	// CPU is the number of vCPUs
	CPU int `json:"cpu,omitempty" yaml:"cpu,omitempty"`

	// MemGB is the memory in GB
	MemGB int `json:"mem_gb,omitempty" yaml:"mem_gb,omitempty"`

	// Firmware is the firmware type (bios, uefi, unknown)
	// Default: "bios"
	Firmware string `json:"firmware,omitempty" yaml:"firmware,omitempty"`

	// SecureBoot indicates if secure boot is enabled
	SecureBoot bool `json:"secureboot,omitempty" yaml:"secureboot,omitempty"`

	// OSHint provides an OS hint for decision-making
	// Examples: "linux", "windows", "unknown"
	OSHint string `json:"os_hint,omitempty" yaml:"os_hint,omitempty"`

	// OSVersion is the OS version string
	// Examples: "Ubuntu 22.04", "Windows Server 2019"
	OSVersion string `json:"os_version,omitempty" yaml:"os_version,omitempty"`
}

// DiskArtifact represents a single disk artifact
type DiskArtifact struct {
	// ID is a unique identifier for this disk within the manifest (REQUIRED)
	// Must match pattern: ^[a-zA-Z0-9_-]+$
	// Examples: "disk-0", "boot-disk", "data-disk-1"
	ID string `json:"id" yaml:"id"`

	// SourceFormat is the disk image format (REQUIRED)
	// Supported: vmdk, qcow2, raw, vhd, vhdx, vdi
	SourceFormat string `json:"source_format" yaml:"source_format"`

	// Bytes is the disk size in bytes (REQUIRED)
	Bytes int64 `json:"bytes" yaml:"bytes"`

	// LocalPath is the absolute path to the disk file (REQUIRED)
	LocalPath string `json:"local_path" yaml:"local_path"`

	// Checksum is the SHA-256 checksum (OPTIONAL but recommended)
	// Format: "sha256:<64 hex characters>"
	// Example: "sha256:a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	Checksum string `json:"checksum,omitempty" yaml:"checksum,omitempty"`

	// BootOrderHint indicates boot priority (0=primary boot disk, 1=secondary, etc.)
	// Default: 999
	BootOrderHint int `json:"boot_order_hint,omitempty" yaml:"boot_order_hint,omitempty"`

	// Label is a human-readable disk label (OPTIONAL)
	Label string `json:"label,omitempty" yaml:"label,omitempty"`

	// DiskType is a hint about the disk type (OPTIONAL)
	// Values: "boot", "data", "unknown"
	// Default: "unknown"
	DiskType string `json:"disk_type,omitempty" yaml:"disk_type,omitempty"`
}

// NICInfo contains network interface information
type NICInfo struct {
	// ID is the NIC identifier
	ID string `json:"id,omitempty" yaml:"id,omitempty"`

	// MAC is the MAC address
	// Format: XX:XX:XX:XX:XX:XX or XX-XX-XX-XX-XX-XX
	MAC string `json:"mac,omitempty" yaml:"mac,omitempty"`

	// Network is the network name from the source
	Network string `json:"network,omitempty" yaml:"network,omitempty"`
}

// Warning represents a non-fatal warning
type Warning struct {
	// Stage is the export stage where the warning occurred
	Stage string `json:"stage" yaml:"stage"`

	// Message is the warning message
	Message string `json:"message" yaml:"message"`

	// Timestamp is when the warning occurred (ISO 8601)
	Timestamp *time.Time `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

// ManifestMetadata contains additional metadata
type ManifestMetadata struct {
	// HyperSDKVersion is the transiva version that created this manifest
	HyperSDKVersion string `json:"hypersdk_version,omitempty" yaml:"hypersdk_version,omitempty"`

	// JobID is the transiva job identifier
	JobID string `json:"job_id,omitempty" yaml:"job_id,omitempty"`

	// CreatedAt is when the manifest was created (ISO 8601)
	CreatedAt *time.Time `json:"created_at,omitempty" yaml:"created_at,omitempty"`

	// Tags contains user-defined tags
	Tags map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// PipelineConfig contains pipeline stage configuration
type PipelineConfig struct {
	// Inspect stage configuration
	Inspect *StageConfig `json:"inspect,omitempty" yaml:"inspect,omitempty"`

	// Fix stage configuration
	Fix *FixStageConfig `json:"fix,omitempty" yaml:"fix,omitempty"`

	// Convert stage configuration
	Convert *ConvertStageConfig `json:"convert,omitempty" yaml:"convert,omitempty"`

	// Validate stage configuration
	Validate *ValidateStageConfig `json:"validate,omitempty" yaml:"validate,omitempty"`
}

// StageConfig is basic stage configuration
type StageConfig struct {
	// Enabled controls whether this stage runs
	Enabled bool `json:"enabled" yaml:"enabled"`

	// CollectGuestInfo uses libguestfs to inspect guest OS details
	CollectGuestInfo bool `json:"collect_guest_info,omitempty" yaml:"collect_guest_info,omitempty"`
}

// FixStageConfig contains FIX stage configuration
type FixStageConfig struct {
	// Enabled controls whether this stage runs
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Backup creates backups before modifications
	Backup bool `json:"backup,omitempty" yaml:"backup,omitempty"`

	// PrintFstab prints /etc/fstab before and after
	PrintFstab bool `json:"print_fstab,omitempty" yaml:"print_fstab,omitempty"`

	// UpdateGrub updates GRUB configuration
	UpdateGrub bool `json:"update_grub,omitempty" yaml:"update_grub,omitempty"`

	// RegenInitramfs regenerates initramfs with virtio drivers
	RegenInitramfs bool `json:"regen_initramfs,omitempty" yaml:"regen_initramfs,omitempty"`

	// FstabMode controls fstab rewrite mode
	// Values: "stabilize-all", "bypath-only", "noop"
	FstabMode string `json:"fstab_mode,omitempty" yaml:"fstab_mode,omitempty"`

	// RemoveVMwareTools removes VMware tools packages
	RemoveVMwareTools bool `json:"remove_vmware_tools,omitempty" yaml:"remove_vmware_tools,omitempty"`

	// EnableRDP enables Windows Remote Desktop via hyper2kvm firstboot (registry, firewall, TermService).
	// When omitted, hyper2kvm defaults to true when vm.os_hint is "windows".
	EnableRDP *bool `json:"enable_rdp,omitempty" yaml:"enable_rdp,omitempty"`
}

// ConvertStageConfig contains CONVERT stage configuration
type ConvertStageConfig struct {
	// Enabled controls whether this stage runs
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Compress enables qcow2 compression (qcow2 only)
	Compress bool `json:"compress,omitempty" yaml:"compress,omitempty"`

	// CompressLevel is the compression level 1-9 (qcow2 only)
	CompressLevel int `json:"compress_level,omitempty" yaml:"compress_level,omitempty"`
}

// ValidateStageConfig contains VALIDATE stage configuration
type ValidateStageConfig struct {
	// Enabled controls whether this stage runs
	Enabled bool `json:"enabled" yaml:"enabled"`

	// CheckImageIntegrity verifies image can be opened by qemu-img
	CheckImageIntegrity bool `json:"check_image_integrity,omitempty" yaml:"check_image_integrity,omitempty"`
}

// GuestConfiguration contains guest OS configuration injection
type GuestConfiguration struct {
	// Users contains user account configuration
	Users map[string]interface{} `json:"users,omitempty" yaml:"users,omitempty"`

	// Services contains systemd service configuration
	Services map[string]interface{} `json:"services,omitempty" yaml:"services,omitempty"`

	// Hostname contains hostname and hosts file configuration
	Hostname map[string]interface{} `json:"hostname,omitempty" yaml:"hostname,omitempty"`

	// Network contains network configuration injection
	Network map[string]interface{} `json:"network,omitempty" yaml:"network,omitempty"`
}

// OutputConfig contains output configuration
type OutputConfig struct {
	// Directory is the output directory (created if doesn't exist)
	Directory string `json:"directory,omitempty" yaml:"directory,omitempty"`

	// Format is the output format (qcow2, raw, vdi)
	// Default: "qcow2"
	Format string `json:"format,omitempty" yaml:"format,omitempty"`

	// Filename is the output filename (auto-generated if not specified)
	Filename string `json:"filename,omitempty" yaml:"filename,omitempty"`
}

// RuntimeOptions contains global runtime options
type RuntimeOptions struct {
	// DryRun doesn't modify guest or write output
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`

	// Verbose is the verbosity level (0=quiet, 1=normal, 2=verbose, 3=debug)
	// Default: 1
	Verbose int `json:"verbose,omitempty" yaml:"verbose,omitempty"`

	// Report contains report configuration
	Report *ReportConfig `json:"report,omitempty" yaml:"report,omitempty"`
}

// ReportConfig contains report configuration
type ReportConfig struct {
	// Enabled controls whether to generate report.json
	// Default: true
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Path is the report filename (relative to output directory)
	// Default: "report.json"
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}
