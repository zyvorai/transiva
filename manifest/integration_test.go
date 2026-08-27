// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package manifest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zyvorai/transiva/manifest"
)

// TestIntegrationWithFedoraVMDK demonstrates creating an Artifact Manifest v1.0
// for a real Fedora/Photon OS VMDK image.
//
// This test uses a real VMDK file to validate the complete workflow:
// 1. Creating a manifest with real disk artifacts
// 2. Computing SHA-256 checksums
// 3. Serializing to JSON
// 4. Validating the manifest
// 5. Verifying checksums
//
// Run with: go test -tags=integration -v ./manifest/...
func TestIntegrationWithFedoraVMDK(t *testing.T) {
	// Path to the Fedora/Photon OS VMDK (adjust as needed)
	// This file is from the h2kvm repository
	vmdkPath := "/home/ssahani/tt/h2kvm/photon.vmdk"

	// Check if the VMDK exists
	if _, err := os.Stat(vmdkPath); os.IsNotExist(err) {
		t.Skipf("Skipping integration test: %s not found", vmdkPath)
	}

	// Get file size
	fileInfo, err := os.Stat(vmdkPath)
	if err != nil {
		t.Fatalf("Failed to stat VMDK: %v", err)
	}

	// Create output directory for test
	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, "artifact-manifest.json")

	t.Logf("Creating Artifact Manifest v1.0 for Fedora VMDK")
	t.Logf("  VMDK: %s", vmdkPath)
	t.Logf("  Size: %.1f MB", float64(fileInfo.Size())/(1024*1024))

	// Build the manifest
	builder := manifest.NewBuilder()

	// Set source metadata
	builder.WithSource(
		"local",                // provider (local disk, not exported from cloud)
		"fedora-test",          // VM ID
		"fedora-photon-os-5.0", // VM name
		"local",                // datacenter
		"local-disk",           // export method
	)

	// Set VM metadata (based on Photon OS)
	builder.WithVM(
		4,               // CPUs
		8,               // memory GB
		"bios",          // firmware
		"linux",         // OS hint
		"Photon OS 5.0", // OS version
		false,           // secure boot
	)

	// Add the disk with checksum computation
	t.Log("Computing SHA-256 checksum...")
	builder.AddDiskWithChecksum(
		"boot-disk",     // disk ID
		"vmdk",          // source format
		vmdkPath,        // local path
		fileInfo.Size(), // bytes
		0,               // boot order hint (primary)
		"boot",          // disk type
		true,            // compute checksum (this will take a while for large files)
	)

	// Add NIC information (example)
	builder.AddNIC(
		"eth0",              // ID
		"00:50:56:ab:cd:ef", // MAC address (example)
		"VM Network",        // network name
	)

	// Add notes
	builder.AddNote("VMware Photon OS 5.0 test image")
	builder.AddNote("Kernel: 6.1.10-11.ph5")
	builder.AddNote("Network drivers: virtio_net required for KVM migration")

	// Configure transiva metadata
	builder.WithMetadata(
		"0.1.0",    // transiva version
		"test-123", // job ID
		map[string]string{
			"test":        "integration",
			"os":          "photon-5.0",
			"description": "Fedora/Photon OS VMDK integration test",
		},
	)

	// Configure h2kvm pipeline
	// This tells h2kvm to run all stages:
	// INSPECT → FIX → CONVERT → VALIDATE
	builder.WithPipeline(
		true, // inspect (detect OS, kernel version)
		true, // fix (inject virtio_net drivers, update fstab/grub)
		true, // convert (vmdk → qcow2)
		true, // validate (verify bootability)
	)

	// Configure output
	builder.WithOutput(
		outputDir, // output directory
		"qcow2",   // target format
		"",        // filename (auto-generated)
	)

	// Build the manifest
	t.Log("Building manifest...")
	m, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Validate the manifest
	t.Log("Validating manifest...")
	if err := manifest.Validate(m); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	// Write to file
	t.Logf("Writing manifest to: %s", manifestPath)
	if err := manifest.WriteToFile(m, manifestPath); err != nil {
		t.Fatalf("WriteToFile() failed: %v", err)
	}

	// Verify the written file can be read back
	t.Log("Reading manifest back...")
	loadedManifest, err := manifest.ReadFromFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFromFile() failed: %v", err)
	}

	// Verify checksum was computed and matches
	if len(loadedManifest.Disks) == 0 {
		t.Fatal("No disks in loaded manifest")
	}

	disk := loadedManifest.Disks[0]
	if disk.Checksum == "" {
		t.Error("Expected checksum to be computed, got empty string")
	}

	t.Logf("✅ Disk checksum: %s...", disk.Checksum[:20])

	// Verify checksums
	t.Log("Verifying checksums...")
	results, err := manifest.VerifyChecksums(loadedManifest)
	if err != nil {
		t.Fatalf("VerifyChecksums() failed: %v", err)
	}

	for diskID, valid := range results {
		if !valid {
			t.Errorf("Checksum verification failed for disk: %s", diskID)
		} else {
			t.Logf("✅ Disk %s: checksum verified", diskID)
		}
	}

	// Print summary
	t.Log("\n=== Artifact Manifest v1.0 Summary ===")
	t.Logf("Manifest Version: %s", loadedManifest.ManifestVersion)
	t.Logf("Source Provider: %s", loadedManifest.Source.Provider)
	t.Logf("VM Name: %s", loadedManifest.Source.VMName)
	t.Logf("OS: %s", loadedManifest.VM.OSVersion)
	t.Logf("Firmware: %s", loadedManifest.VM.Firmware)
	t.Logf("Disks: %d", len(loadedManifest.Disks))
	t.Logf("  - ID: %s", disk.ID)
	t.Logf("  - Format: %s", disk.SourceFormat)
	t.Logf("  - Size: %.1f MB", float64(disk.Bytes)/(1024*1024))
	t.Logf("  - Boot Order: %d", disk.BootOrderHint)
	t.Logf("  - Type: %s", disk.DiskType)
	t.Logf("Pipeline Stages:")
	t.Logf("  - INSPECT: %v", loadedManifest.Pipeline.Inspect.Enabled)
	t.Logf("  - FIX: %v", loadedManifest.Pipeline.Fix.Enabled)
	t.Logf("  - CONVERT: %v", loadedManifest.Pipeline.Convert.Enabled)
	t.Logf("  - VALIDATE: %v", loadedManifest.Pipeline.Validate.Enabled)

	// Print the manifest content for inspection
	jsonData, _ := manifest.ToJSON(loadedManifest)
	t.Logf("\n=== Generated Manifest (excerpt) ===\n%s\n", string(jsonData[:min(len(jsonData), 500)])+"...")

	t.Log("\n✅ Integration test completed successfully!")
	t.Log("   The generated manifest is compatible with h2kvm ManifestLoader")
	t.Log("   Next step: Pass this manifest to h2kvm for conversion")
	t.Logf("   Command: h2kvm --manifest %s", manifestPath)
}

// TestMultiDiskIntegration tests creating a multi-disk manifest
func TestMultiDiskIntegration(t *testing.T) {
	// This test demonstrates multi-disk VM support
	// Create temporary test disks

	outputDir := t.TempDir()

	// Create example disk files
	bootDiskPath := filepath.Join(outputDir, "boot.vmdk")
	dataDisk1Path := filepath.Join(outputDir, "data1.vmdk")
	dataDisk2Path := filepath.Join(outputDir, "data2.vmdk")

	// Create minimal test files
	testData := []byte("test disk data")
	os.WriteFile(bootDiskPath, testData, 0644)
	os.WriteFile(dataDisk1Path, testData, 0644)
	os.WriteFile(dataDisk2Path, testData, 0644)

	// Build multi-disk manifest
	m, err := manifest.NewBuilder().
		WithSource("vsphere", "vm-multi-disk", "multi-disk-vm", "DC1", "govc-export").
		WithVM(8, 32, "uefi", "linux", "Fedora 39", false).
		// Boot disk (boot_order_hint = 0)
		AddDiskWithChecksum("boot-disk", "vmdk", bootDiskPath, int64(len(testData)), 0, "boot", true).
		// Data disk 1 (boot_order_hint = 1)
		AddDiskWithChecksum("data-disk-1", "vmdk", dataDisk1Path, int64(len(testData)), 1, "data", true).
		// Data disk 2 (boot_order_hint = 2)
		AddDiskWithChecksum("data-disk-2", "vmdk", dataDisk2Path, int64(len(testData)), 2, "data", true).
		WithPipeline(true, true, true, true).
		Build()

	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify multi-disk setup
	if len(m.Disks) != 3 {
		t.Fatalf("Expected 3 disks, got %d", len(m.Disks))
	}

	// Verify boot order
	bootDisk := m.Disks[0]
	if bootDisk.BootOrderHint != 0 {
		t.Errorf("Boot disk should have boot_order_hint=0, got %d", bootDisk.BootOrderHint)
	}

	if bootDisk.DiskType != "boot" {
		t.Errorf("Expected disk_type=boot, got %s", bootDisk.DiskType)
	}

	// Verify checksums for all disks
	results, err := manifest.VerifyChecksums(m)
	if err != nil {
		t.Fatalf("VerifyChecksums() failed: %v", err)
	}

	for diskID, valid := range results {
		if !valid {
			t.Errorf("Checksum verification failed for disk: %s", diskID)
		}
	}

	t.Log("✅ Multi-disk manifest created and verified successfully")
	t.Logf("   Boot disk: %s (boot_order_hint=%d)", bootDisk.ID, bootDisk.BootOrderHint)
	t.Logf("   Data disks: %d", len(m.Disks)-1)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestManifestCompatibilityWithH2KVM(t *testing.T) {
	// This test verifies that the generated manifest matches
	// the structure expected by h2kvm ManifestLoader

	outputDir := t.TempDir()
	diskPath := filepath.Join(outputDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	// Create a manifest that matches h2kvm's reference example
	m, err := manifest.NewBuilder().
		WithSource("vsphere", "vm-1234", "production-webserver-01", "DC1", "govc-export").
		WithVM(4, 16, "uefi", "linux", "Ubuntu 22.04 LTS", false).
		AddDisk("boot-disk", "vmdk", diskPath, 107374182400, 0, "boot").
		AddNIC("eth0", "00:50:56:ab:cd:ef", "VM Network").
		WithMetadata("0.1.0", "job-123", map[string]string{
			"environment": "production",
			"team":        "ops",
		}).
		WithPipeline(true, true, true, true).
		WithOutput(outputDir, "qcow2", "").
		Build()

	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	// Verify structure matches h2kvm expectations
	if m.ManifestVersion != "1.0" {
		t.Errorf("Expected manifest_version '1.0', got %q", m.ManifestVersion)
	}

	if len(m.Disks) == 0 {
		t.Fatal("Manifest must have at least one disk")
	}

	disk := m.Disks[0]
	if disk.ID == "" {
		t.Error("Disk ID must not be empty")
	}

	if disk.SourceFormat == "" {
		t.Error("Disk source_format must not be empty")
	}

	if disk.LocalPath == "" {
		t.Error("Disk local_path must not be empty")
	}

	if disk.Bytes <= 0 {
		t.Error("Disk bytes must be positive")
	}

	// Verify pipeline configuration
	if m.Pipeline == nil {
		t.Fatal("Pipeline configuration must be present")
	}

	if m.Pipeline.Inspect == nil || !m.Pipeline.Inspect.Enabled {
		t.Error("INSPECT stage should be enabled")
	}

	if m.Pipeline.Fix == nil || !m.Pipeline.Fix.Enabled {
		t.Error("FIX stage should be enabled")
	}

	if m.Pipeline.Convert == nil || !m.Pipeline.Convert.Enabled {
		t.Error("CONVERT stage should be enabled")
	}

	if m.Pipeline.Validate == nil || !m.Pipeline.Validate.Enabled {
		t.Error("VALIDATE stage should be enabled")
	}

	t.Log("✅ Manifest structure is compatible with h2kvm ManifestLoader")
}
