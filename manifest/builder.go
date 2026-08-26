// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	// CurrentVersion is the current Artifact Manifest version
	CurrentVersion = "1.0"

	// DefaultBootOrderHint is the default boot order hint for disks
	DefaultBootOrderHint = 999
)

var (
	// ValidDiskIDPattern is the regex pattern for valid disk IDs
	ValidDiskIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// ValidChecksumPattern is the regex pattern for valid checksums
	ValidChecksumPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)

// Builder provides a fluent API for building Artifact Manifests
type Builder struct {
	manifest *ArtifactManifest
	errors   []error
}

// NewBuilder creates a new manifest builder
func NewBuilder() *Builder {
	now := time.Now()
	return &Builder{
		manifest: &ArtifactManifest{
			ManifestVersion: CurrentVersion,
			Disks:           []DiskArtifact{},
			NICs:            []NICInfo{},
			Notes:           []string{},
			Warnings:        []Warning{},
			Metadata: &ManifestMetadata{
				CreatedAt: &now,
				Tags:      make(map[string]string),
			},
		},
		errors: []error{},
	}
}

// WithSource sets the source metadata
func (b *Builder) WithSource(provider, vmID, vmName, datacenter, exportMethod string) *Builder {
	now := time.Now()
	b.manifest.Source = &SourceMetadata{
		Provider:        provider,
		VMID:            vmID,
		VMName:          vmName,
		Datacenter:      datacenter,
		ExportTimestamp: &now,
		ExportMethod:    exportMethod,
	}
	return b
}

// WithVM sets the VM metadata
func (b *Builder) WithVM(cpu, memGB int, firmware, osHint, osVersion string, secureBoot bool) *Builder {
	b.manifest.VM = &VMMetadata{
		CPU:        cpu,
		MemGB:      memGB,
		Firmware:   firmware,
		SecureBoot: secureBoot,
		OSHint:     osHint,
		OSVersion:  osVersion,
	}
	return b
}

// AddDisk adds a disk artifact to the manifest
func (b *Builder) AddDisk(id, sourceFormat, localPath string, bytes int64, bootOrderHint int, diskType string) *Builder {
	// Validate disk ID
	if !ValidDiskIDPattern.MatchString(id) {
		b.errors = append(b.errors, fmt.Errorf("invalid disk ID %q: must match pattern ^[a-zA-Z0-9_-]+$", id))
		return b
	}

	// Check for duplicate IDs
	for _, disk := range b.manifest.Disks {
		if disk.ID == id {
			b.errors = append(b.errors, fmt.Errorf("duplicate disk ID: %q", id))
			return b
		}
	}

	// Validate source format
	validFormats := map[string]bool{
		"vmdk": true, "qcow2": true, "raw": true,
		"vhd": true, "vhdx": true, "vdi": true,
	}
	if !validFormats[sourceFormat] {
		b.errors = append(b.errors, fmt.Errorf("invalid source format %q: must be one of vmdk, qcow2, raw, vhd, vhdx, vdi", sourceFormat))
		return b
	}

	// Validate file exists
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		b.errors = append(b.errors, fmt.Errorf("invalid local path %q: %w", localPath, err))
		return b
	}

	if _, err := os.Stat(absPath); err != nil {
		b.errors = append(b.errors, fmt.Errorf("disk file not found: %q: %w", absPath, err))
		return b
	}

	disk := DiskArtifact{
		ID:            id,
		SourceFormat:  sourceFormat,
		Bytes:         bytes,
		LocalPath:     absPath,
		BootOrderHint: bootOrderHint,
		Label:         id,
		DiskType:      diskType,
	}

	b.manifest.Disks = append(b.manifest.Disks, disk)
	return b
}

// AddDiskWithChecksum adds a disk artifact with checksum verification
func (b *Builder) AddDiskWithChecksum(id, sourceFormat, localPath string, bytes int64, bootOrderHint int, diskType string, computeChecksum bool) *Builder {
	b.AddDisk(id, sourceFormat, localPath, bytes, bootOrderHint, diskType)
	if len(b.errors) > 0 {
		return b // Don't compute checksum if there were errors
	}

	if computeChecksum {
		// Get the disk we just added
		if len(b.manifest.Disks) > 0 {
			disk := &b.manifest.Disks[len(b.manifest.Disks)-1]
			checksum, err := ComputeSHA256(disk.LocalPath)
			if err != nil {
				b.errors = append(b.errors, fmt.Errorf("compute checksum for %q: %w", disk.ID, err))
				return b
			}
			disk.Checksum = fmt.Sprintf("sha256:%s", checksum)
		}
	}

	return b
}

// AddNIC adds a network interface
func (b *Builder) AddNIC(id, mac, network string) *Builder {
	b.manifest.NICs = append(b.manifest.NICs, NICInfo{
		ID:      id,
		MAC:     mac,
		Network: network,
	})
	return b
}

// AddNote adds an informational note
func (b *Builder) AddNote(note string) *Builder {
	b.manifest.Notes = append(b.manifest.Notes, note)
	return b
}

// AddWarning adds a warning
func (b *Builder) AddWarning(stage, message string) *Builder {
	now := time.Now()
	b.manifest.Warnings = append(b.manifest.Warnings, Warning{
		Stage:     stage,
		Message:   message,
		Timestamp: &now,
	})
	return b
}

// WithMetadata sets metadata fields
func (b *Builder) WithMetadata(hypersdkVersion, jobID string, tags map[string]string) *Builder {
	if b.manifest.Metadata == nil {
		now := time.Now()
		b.manifest.Metadata = &ManifestMetadata{
			CreatedAt: &now,
		}
	}
	b.manifest.Metadata.HyperSDKVersion = hypersdkVersion
	b.manifest.Metadata.JobID = jobID
	if tags != nil {
		b.manifest.Metadata.Tags = tags
	}
	return b
}

// WithPipeline sets pipeline configuration
func (b *Builder) WithPipeline(inspect, fix, convert, validate bool) *Builder {
	b.manifest.Pipeline = &PipelineConfig{
		Inspect: &StageConfig{
			Enabled:          inspect,
			CollectGuestInfo: true,
		},
		Fix: &FixStageConfig{
			Enabled:           fix,
			Backup:            true,
			UpdateGrub:        true,
			RegenInitramfs:    true,
			FstabMode:         "stabilize-all",
			RemoveVMwareTools: true,
		},
		Convert: &ConvertStageConfig{
			Enabled:  convert,
			Compress: true,
		},
		Validate: &ValidateStageConfig{
			Enabled:             validate,
			CheckImageIntegrity: true,
		},
	}
	return b
}

// WithFixEnableRDP sets whether hyper2kvm firstboot should enable Windows RDP.
func (b *Builder) WithFixEnableRDP(enable bool) *Builder {
	if b.manifest.Pipeline == nil {
		b.WithPipeline(false, false, false, false)
	}
	if b.manifest.Pipeline.Fix == nil {
		b.manifest.Pipeline.Fix = &FixStageConfig{Enabled: true}
	}
	v := enable
	b.manifest.Pipeline.Fix.EnableRDP = &v
	return b
}

// WithOutput sets output configuration
func (b *Builder) WithOutput(directory, format, filename string) *Builder {
	b.manifest.Output = &OutputConfig{
		Directory: directory,
		Format:    format,
		Filename:  filename,
	}
	return b
}

// WithOptions sets runtime options
func (b *Builder) WithOptions(dryRun bool, verbose int) *Builder {
	b.manifest.Options = &RuntimeOptions{
		DryRun:  dryRun,
		Verbose: verbose,
		Report: &ReportConfig{
			Enabled: true,
			Path:    "report.json",
		},
	}
	return b
}

// Build returns the constructed manifest or an error
func (b *Builder) Build() (*ArtifactManifest, error) {
	// Check for build errors
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("manifest build failed with %d error(s): %v", len(b.errors), b.errors[0])
	}

	// Validate required fields
	if len(b.manifest.Disks) == 0 {
		return nil, fmt.Errorf("manifest must have at least one disk")
	}

	return b.manifest, nil
}

// ComputeSHA256 computes the SHA-256 checksum of a file
func ComputeSHA256(filePath string) (string, error) {
	// #nosec G304 -- filePath is a locally-configured disk path supplied by the operator
	// building/validating an export manifest (manifest/builder.go, manifest/validator.go), not remote input.
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("compute hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
