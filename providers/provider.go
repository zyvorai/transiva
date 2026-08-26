// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	"time"
)

// ProviderType identifies the type of virtualization/cloud provider
type ProviderType string

const (
	ProviderVSphere  ProviderType = "vsphere"
	ProviderAWS      ProviderType = "aws"
	ProviderAzure    ProviderType = "azure"
	ProviderGCP      ProviderType = "gcp"
	ProviderHyperV   ProviderType = "hyperv"
	ProviderProxmox  ProviderType = "proxmox"
	ProviderKubeVirt ProviderType = "kubevirt"
	ProviderNutanix  ProviderType = "nutanix"
)

// Provider defines the unified interface for all virtualization/cloud providers
type Provider interface {
	// Identity
	Name() string
	Type() ProviderType

	// Connection lifecycle
	Connect(ctx context.Context, config ProviderConfig) error
	Disconnect() error
	ValidateCredentials(ctx context.Context) error

	// VM Discovery
	ListVMs(ctx context.Context, filter VMFilter) ([]*VMInfo, error)
	GetVM(ctx context.Context, identifier string) (*VMInfo, error)
	SearchVMs(ctx context.Context, query string) ([]*VMInfo, error)

	// VM Export
	ExportVM(ctx context.Context, identifier string, opts ExportOptions) (*ExportResult, error)
	GetExportCapabilities() ExportCapabilities
}

// ProviderConfig holds provider-specific configuration
type ProviderConfig struct {
	Type ProviderType

	// Common fields
	Endpoint string
	Host     string // Alternative to Endpoint for providers that need separate host/port
	Port     int    // Port number (used with Host)
	Username string
	Password string
	Region   string // AWS region, Azure location, GCP zone, Proxmox realm, etc.
	Insecure bool
	Timeout  time.Duration

	// Provider-specific fields (stored as key-value pairs)
	Metadata map[string]interface{}
}

// VMFilter defines filtering criteria for VM listing
type VMFilter struct {
	NamePattern string            // Glob pattern for VM name
	State       string            // VM state (running, stopped, suspended, etc.)
	PowerState  string            // Alternative to State for backward compatibility
	Tags        map[string]string // Filter by tags/labels
	Location    string            // Datacenter, region, resource group, etc.
	MinMemoryMB int64             // Minimum memory in MB
	MinCPUs     int               // Minimum number of CPUs
}

// VMInfo represents metadata about a virtual machine
type VMInfo struct {
	Provider    ProviderType
	ID          string // Provider-specific unique identifier
	Name        string
	State       string // running, stopped, suspended, etc.
	Location    string // Datacenter, region, availability zone, etc.
	PowerState  string
	GuestOS     string
	MemoryMB    int64
	NumCPUs     int
	StorageGB   int64
	IPAddresses []string
	Tags        map[string]string
	Metadata    map[string]interface{} // Provider-specific metadata
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

// ExportOptions defines options for VM export
type ExportOptions struct {
	OutputPath       string // Local path or cloud storage URL
	Format           string // ovf, ova, vmdk, vhd, raw, etc.
	Compress         bool
	CompressionLevel int
	IncludeSnapshots bool
	RemoveCDROM      bool
	Metadata         map[string]interface{} // Provider-specific options
	Progress         ProgressReporter       `json:"-"` // In-process progress callback
}

// ExportResult represents the result of a VM export operation
type ExportResult struct {
	Provider       ProviderType
	VMName         string
	VMID           string
	Format         string
	OutputPath     string   // Main output file or directory
	Files          []string // All exported files
	Size           int64    // Total size in bytes
	CompressedSize int64    // Compressed size (if compression was used)
	Checksum       string   // SHA-256 checksum of main file
	Duration       time.Duration
	Metadata       map[string]interface{} // Provider-specific result metadata
}

// ExportCapabilities describes what export formats and features a provider supports
type ExportCapabilities struct {
	SupportedFormats    []string // List of supported export formats
	SupportsCompression bool
	SupportsStreaming   bool     // Can stream directly to cloud storage
	SupportsSnapshots   bool     // Can export VM snapshots
	MaxVMSizeGB         int64    // Maximum VM size that can be exported (0 = unlimited)
	SupportedTargets    []string // cloud storage types: s3, azure-blob, gcs, local
}

// ProgressReporter is a callback for reporting export progress
type ProgressReporter interface {
	Update(phase string, percentComplete float64, message string)
}
