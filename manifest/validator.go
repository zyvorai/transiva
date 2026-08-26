// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"os"
	"path/filepath"
)

// Validate validates an Artifact Manifest against the v1.0 schema
func Validate(m *ArtifactManifest) error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}

	// Validate manifest version
	if m.ManifestVersion != CurrentVersion {
		return fmt.Errorf("unsupported manifest version %q: expected %q", m.ManifestVersion, CurrentVersion)
	}

	// Validate disks (REQUIRED)
	if len(m.Disks) == 0 {
		return fmt.Errorf("manifest must have at least one disk")
	}

	// Track disk IDs for duplicate detection
	diskIDs := make(map[string]bool)

	for i, disk := range m.Disks {
		if err := validateDisk(disk, i, diskIDs); err != nil {
			return err
		}
	}

	// Validate VM metadata if present
	if m.VM != nil {
		if err := validateVMMetadata(m.VM); err != nil {
			return err
		}
	}

	// Validate NICs if present
	for i, nic := range m.NICs {
		if err := validateNIC(nic, i); err != nil {
			return err
		}
	}

	return nil
}

// validateDisk validates a single disk artifact
func validateDisk(disk DiskArtifact, index int, diskIDs map[string]bool) error {
	// Validate ID (REQUIRED)
	if disk.ID == "" {
		return fmt.Errorf("disks[%d].id is required", index)
	}

	if !ValidDiskIDPattern.MatchString(disk.ID) {
		return fmt.Errorf("disks[%d].id %q must match pattern ^[a-zA-Z0-9_-]+$", index, disk.ID)
	}

	// Check for duplicate IDs
	if diskIDs[disk.ID] {
		return fmt.Errorf("duplicate disk ID: %q", disk.ID)
	}
	diskIDs[disk.ID] = true

	// Validate source format (REQUIRED)
	validFormats := map[string]bool{
		"vmdk": true, "qcow2": true, "raw": true,
		"vhd": true, "vhdx": true, "vdi": true,
	}
	if !validFormats[disk.SourceFormat] {
		return fmt.Errorf("disks[%d].source_format %q must be one of: vmdk, qcow2, raw, vhd, vhdx, vdi", index, disk.SourceFormat)
	}

	// Validate bytes (REQUIRED, non-negative)
	if disk.Bytes < 0 {
		return fmt.Errorf("disks[%d].bytes must be non-negative (got: %d)", index, disk.Bytes)
	}

	// Validate local path (REQUIRED)
	if disk.LocalPath == "" {
		return fmt.Errorf("disks[%d].local_path is required", index)
	}

	// Check if file exists
	absPath, err := filepath.Abs(disk.LocalPath)
	if err != nil {
		return fmt.Errorf("disks[%d].local_path %q is invalid: %w", index, disk.LocalPath, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("disks[%d].local_path not found: %q: %w", index, absPath, err)
	}

	// Validate checksum format if present (OPTIONAL)
	if disk.Checksum != "" {
		if !ValidChecksumPattern.MatchString(disk.Checksum) {
			return fmt.Errorf("disks[%d].checksum must match format sha256:<hexdigest> (got: %q)", index, disk.Checksum)
		}
	}

	// Validate boot order hint (non-negative)
	if disk.BootOrderHint < 0 {
		return fmt.Errorf("disks[%d].boot_order_hint must be non-negative (got: %d)", index, disk.BootOrderHint)
	}

	// Validate disk type if present
	if disk.DiskType != "" {
		validTypes := map[string]bool{
			"boot": true, "data": true, "unknown": true,
		}
		if !validTypes[disk.DiskType] {
			return fmt.Errorf("disks[%d].disk_type %q must be one of: boot, data, unknown", index, disk.DiskType)
		}
	}

	return nil
}

// validateVMMetadata validates VM metadata
func validateVMMetadata(vm *VMMetadata) error {
	// Validate firmware if present
	if vm.Firmware != "" {
		validFirmware := map[string]bool{
			"bios": true, "uefi": true, "unknown": true,
		}
		if !validFirmware[vm.Firmware] {
			return fmt.Errorf("vm.firmware %q must be one of: bios, uefi, unknown", vm.Firmware)
		}
	}

	// Validate CPU count
	if vm.CPU < 0 {
		return fmt.Errorf("vm.cpu must be non-negative (got: %d)", vm.CPU)
	}

	// Validate memory
	if vm.MemGB < 0 {
		return fmt.Errorf("vm.mem_gb must be non-negative (got: %d)", vm.MemGB)
	}

	return nil
}

// validateNIC validates a network interface
func validateNIC(nic NICInfo, index int) error {
	// MAC address validation is optional but if present, should match pattern
	if nic.MAC != "" {
		// Simple MAC address pattern check
		// Format: XX:XX:XX:XX:XX:XX or XX-XX-XX-XX-XX-XX
		// More lenient than JSON schema to allow various formats
		if len(nic.MAC) < 17 {
			return fmt.Errorf("nics[%d].mac %q appears to be invalid (too short)", index, nic.MAC)
		}
	}

	return nil
}

// VerifyChecksums verifies checksums for all disks that have them
func VerifyChecksums(m *ArtifactManifest) (map[string]bool, error) {
	results := make(map[string]bool)

	for _, disk := range m.Disks {
		if disk.Checksum == "" {
			continue // Skip disks without checksums
		}

		// Extract expected checksum (strip "sha256:" prefix)
		expectedChecksum := disk.Checksum[7:] // Remove "sha256:"

		// Compute actual checksum
		actualChecksum, err := ComputeSHA256(disk.LocalPath)
		if err != nil {
			return results, fmt.Errorf("compute checksum for disk %q: %w", disk.ID, err)
		}

		// Compare
		match := actualChecksum == expectedChecksum
		results[disk.ID] = match

		if !match {
			return results, fmt.Errorf("checksum mismatch for disk %q: expected %s, got %s", disk.ID, expectedChecksum, actualChecksum)
		}
	}

	return results, nil
}
