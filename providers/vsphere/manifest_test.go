// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zyvorai/transiva/manifest"
)

func TestManifestGeneration(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "vsphere-manifest-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test VMDK file
	vmdkPath := filepath.Join(tmpDir, "test-disk.vmdk")
	testData := []byte("test vmdk data")
	if err := os.WriteFile(vmdkPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test VMDK: %v", err)
	}

	// Create mock VM properties
	props := &vmProperties{
		NumCPU:   4,
		MemoryGB: 16,
		GuestOS:  "ubuntu64Guest",
		Firmware: "bios",
	}

	// Test manifest generation
	builder := manifest.NewBuilder()

	builder.WithSource(
		"vsphere",
		"vm-123",
		"test-vm",
		"test-datacenter",
		"transiva-govc",
	)

	builder.WithVM(
		int(props.NumCPU),
		int(props.MemoryGB),
		props.Firmware,
		"linux",
		props.GuestOS,
		false,
	)

	// Add disk
	builder.AddDisk(
		"disk-0",
		"vmdk",
		vmdkPath,
		int64(len(testData)),
		0,
		"boot",
	)

	builder.WithMetadata(
		"0.1.0",
		"test-job-123",
		map[string]string{
			"provider": "vsphere",
			"test":     "true",
		},
	)

	builder.WithPipeline(true, true, true, true)
	builder.WithOutput(tmpDir, "qcow2", "")

	// Build manifest
	m, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Validate manifest
	if err := manifest.Validate(m); err != nil {
		t.Errorf("Manifest validation failed: %v", err)
	}

	// Check required fields
	if m.ManifestVersion != "1.0" {
		t.Errorf("Expected manifest version 1.0, got %s", m.ManifestVersion)
	}

	if len(m.Disks) != 1 {
		t.Errorf("Expected 1 disk, got %d", len(m.Disks))
	}

	if m.Disks[0].ID != "disk-0" {
		t.Errorf("Expected disk ID 'disk-0', got %s", m.Disks[0].ID)
	}

	if m.Disks[0].SourceFormat != "vmdk" {
		t.Errorf("Expected source format 'vmdk', got %s", m.Disks[0].SourceFormat)
	}

	if m.VM == nil {
		t.Error("VM metadata is nil")
	} else {
		if m.VM.CPU != 4 {
			t.Errorf("Expected 4 CPUs, got %d", m.VM.CPU)
		}
		if m.VM.MemGB != 16 {
			t.Errorf("Expected 16 GB memory, got %d", m.VM.MemGB)
		}
	}

	// Write manifest to file
	manifestPath := filepath.Join(tmpDir, "artifact-manifest.json")
	if err := manifest.WriteToFile(m, manifestPath); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("Manifest file was not created")
	}

	// Read manifest back
	loadedManifest, err := manifest.ReadFromFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	// Verify loaded manifest
	if loadedManifest.ManifestVersion != m.ManifestVersion {
		t.Errorf("Loaded manifest version mismatch")
	}

	if len(loadedManifest.Disks) != len(m.Disks) {
		t.Errorf("Loaded manifest disk count mismatch")
	}

	t.Logf("✅ Manifest generation test passed")
	t.Logf("   Manifest: %s", manifestPath)
	t.Logf("   Disks: %d", len(m.Disks))
	t.Logf("   VM: %d CPUs, %d GB RAM", m.VM.CPU, m.VM.MemGB)
}

func TestManifestWithChecksums(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "vsphere-manifest-checksum-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test VMDK file with known content
	vmdkPath := filepath.Join(tmpDir, "test-disk.vmdk")
	testData := []byte("test vmdk data for checksum")
	if err := os.WriteFile(vmdkPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test VMDK: %v", err)
	}

	// Create manifest with checksum
	builder := manifest.NewBuilder()

	builder.WithSource("vsphere", "vm-456", "test-vm-checksum", "DC1", "transiva-govc")
	builder.WithVM(2, 8, "bios", "linux", "ubuntu64Guest", false)

	// Add disk with checksum computation
	builder.AddDiskWithChecksum(
		"disk-0",
		"vmdk",
		vmdkPath,
		int64(len(testData)),
		0,
		"boot",
		true, // compute checksum
	)

	builder.WithPipeline(true, true, true, true)
	builder.WithOutput(tmpDir, "qcow2", "")

	// Build manifest
	m, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Verify checksum was computed
	if m.Disks[0].Checksum == "" {
		t.Error("Checksum was not computed")
	}

	// Verify checksum format
	if len(m.Disks[0].Checksum) != 71 { // "sha256:" (7 chars) + 64 hex chars
		t.Errorf("Invalid checksum format: %s (length: %d)", m.Disks[0].Checksum, len(m.Disks[0].Checksum))
	}

	// Write manifest
	manifestPath := filepath.Join(tmpDir, "artifact-manifest.json")
	if err := manifest.WriteToFile(m, manifestPath); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	// Verify checksums
	results, err := manifest.VerifyChecksums(m)
	if err != nil {
		t.Fatalf("Failed to verify checksums: %v", err)
	}

	if !results["disk-0"] {
		t.Error("Checksum verification failed for disk-0")
	}

	t.Logf("✅ Manifest checksum test passed")
	t.Logf("   Checksum: %s", m.Disks[0].Checksum[:20]+"...")
	t.Logf("   Verification: passed")
}

func TestManifestPipelineConfiguration(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "vsphere-manifest-pipeline-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test VMDK file
	vmdkPath := filepath.Join(tmpDir, "test-disk.vmdk")
	if err := os.WriteFile(vmdkPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test VMDK: %v", err)
	}

	// Create manifest with pipeline configuration
	builder := manifest.NewBuilder()

	builder.WithSource("vsphere", "vm-789", "test-pipeline", "DC1", "transiva-govc")
	builder.WithVM(4, 16, "uefi", "linux", "fedora64Guest", false)
	builder.AddDisk("disk-0", "vmdk", vmdkPath, 1024, 0, "boot")

	// Configure pipeline: INSPECT → FIX → CONVERT → VALIDATE
	builder.WithPipeline(
		true, // inspect
		true, // fix
		true, // convert
		true, // validate
	)

	builder.WithOutput(tmpDir, "qcow2", "")

	// Build manifest
	m, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Verify pipeline configuration
	if m.Pipeline == nil {
		t.Fatal("Pipeline configuration is nil")
	}

	if !m.Pipeline.Inspect.Enabled {
		t.Error("INSPECT stage should be enabled")
	}

	if !m.Pipeline.Fix.Enabled {
		t.Error("FIX stage should be enabled")
	}

	if !m.Pipeline.Convert.Enabled {
		t.Error("CONVERT stage should be enabled")
	}

	if !m.Pipeline.Validate.Enabled {
		t.Error("VALIDATE stage should be enabled")
	}

	// Verify output configuration
	if m.Output == nil {
		t.Fatal("Output configuration is nil")
	}

	if m.Output.Format != "qcow2" {
		t.Errorf("Expected target format 'qcow2', got %s", m.Output.Format)
	}

	t.Logf("✅ Pipeline configuration test passed")
	t.Logf("   INSPECT: %v", m.Pipeline.Inspect.Enabled)
	t.Logf("   FIX: %v", m.Pipeline.Fix.Enabled)
	t.Logf("   CONVERT: %v", m.Pipeline.Convert.Enabled)
	t.Logf("   VALIDATE: %v", m.Pipeline.Validate.Enabled)
	t.Logf("   Target Format: %s", m.Output.Format)
}
