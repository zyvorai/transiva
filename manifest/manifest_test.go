// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()

	if builder == nil {
		t.Fatal("NewBuilder() returned nil")
	}

	if builder.manifest == nil {
		t.Fatal("Builder manifest is nil")
	}

	if builder.manifest.ManifestVersion != CurrentVersion {
		t.Errorf("Expected manifest version %q, got %q", CurrentVersion, builder.manifest.ManifestVersion)
	}
}

func TestBuilderWithSource(t *testing.T) {
	builder := NewBuilder()
	builder.WithSource("vsphere", "vm-1234", "test-vm", "DC1", "govc-export")

	if builder.manifest.Source == nil {
		t.Fatal("Source metadata is nil")
	}

	if builder.manifest.Source.Provider != "vsphere" {
		t.Errorf("Expected provider %q, got %q", "vsphere", builder.manifest.Source.Provider)
	}

	if builder.manifest.Source.VMID != "vm-1234" {
		t.Errorf("Expected VM ID %q, got %q", "vm-1234", builder.manifest.Source.VMID)
	}
}

func TestBuilderWithVM(t *testing.T) {
	builder := NewBuilder()
	builder.WithVM(4, 16, "uefi", "linux", "Ubuntu 22.04", false)

	if builder.manifest.VM == nil {
		t.Fatal("VM metadata is nil")
	}

	if builder.manifest.VM.CPU != 4 {
		t.Errorf("Expected CPU count 4, got %d", builder.manifest.VM.CPU)
	}

	if builder.manifest.VM.Firmware != "uefi" {
		t.Errorf("Expected firmware %q, got %q", "uefi", builder.manifest.VM.Firmware)
	}
}

func TestBuilderAddDisk(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test-disk.vmdk")
	if err := os.WriteFile(diskPath, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create test disk: %v", err)
	}

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	if len(builder.manifest.Disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(builder.manifest.Disks))
	}

	disk := builder.manifest.Disks[0]
	if disk.ID != "disk-0" {
		t.Errorf("Expected disk ID %q, got %q", "disk-0", disk.ID)
	}

	if disk.SourceFormat != "vmdk" {
		t.Errorf("Expected source format %q, got %q", "vmdk", disk.SourceFormat)
	}

	if disk.Bytes != 1024 {
		t.Errorf("Expected bytes 1024, got %d", disk.Bytes)
	}

	if disk.BootOrderHint != 0 {
		t.Errorf("Expected boot order hint 0, got %d", disk.BootOrderHint)
	}

	if disk.DiskType != "boot" {
		t.Errorf("Expected disk type %q, got %q", "boot", disk.DiskType)
	}
}

func TestBuilderAddDiskInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk with spaces", "vmdk", diskPath, 1024, 0, "boot")

	if len(builder.errors) == 0 {
		t.Error("Expected error for invalid disk ID, got none")
	}

	if !strings.Contains(builder.errors[0].Error(), "invalid disk ID") {
		t.Errorf("Expected 'invalid disk ID' error, got: %v", builder.errors[0])
	}
}

func TestBuilderAddDiskDuplicateID(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	if len(builder.errors) == 0 {
		t.Error("Expected error for duplicate disk ID, got none")
	}

	if !strings.Contains(builder.errors[0].Error(), "duplicate disk ID") {
		t.Errorf("Expected 'duplicate disk ID' error, got: %v", builder.errors[0])
	}
}

func TestBuilderAddDiskInvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "invalid", diskPath, 1024, 0, "boot")

	if len(builder.errors) == 0 {
		t.Error("Expected error for invalid source format, got none")
	}

	if !strings.Contains(builder.errors[0].Error(), "invalid source format") {
		t.Errorf("Expected 'invalid source format' error, got: %v", builder.errors[0])
	}
}

func TestBuilderAddDiskFileNotFound(t *testing.T) {
	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", "/nonexistent/path/disk.vmdk", 1024, 0, "boot")

	if len(builder.errors) == 0 {
		t.Error("Expected error for nonexistent file, got none")
	}

	if !strings.Contains(builder.errors[0].Error(), "disk file not found") {
		t.Errorf("Expected 'disk file not found' error, got: %v", builder.errors[0])
	}
}

func TestBuilderAddDiskWithChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	testData := []byte("test data for checksum")
	if err := os.WriteFile(diskPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	builder := NewBuilder()
	builder.AddDiskWithChecksum("disk-0", "vmdk", diskPath, int64(len(testData)), 0, "boot", true)

	if len(builder.errors) > 0 {
		t.Fatalf("Unexpected error: %v", builder.errors[0])
	}

	if len(builder.manifest.Disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(builder.manifest.Disks))
	}

	disk := builder.manifest.Disks[0]
	if disk.Checksum == "" {
		t.Error("Expected checksum to be computed, got empty string")
	}

	if !strings.HasPrefix(disk.Checksum, "sha256:") {
		t.Errorf("Expected checksum to start with 'sha256:', got %q", disk.Checksum)
	}

	if len(disk.Checksum) != 71 { // "sha256:" + 64 hex characters
		t.Errorf("Expected checksum length 71, got %d", len(disk.Checksum))
	}
}

func TestBuilderAddNIC(t *testing.T) {
	builder := NewBuilder()
	builder.AddNIC("eth0", "00:50:56:ab:cd:ef", "VM Network")

	if len(builder.manifest.NICs) != 1 {
		t.Fatalf("Expected 1 NIC, got %d", len(builder.manifest.NICs))
	}

	nic := builder.manifest.NICs[0]
	if nic.ID != "eth0" {
		t.Errorf("Expected NIC ID %q, got %q", "eth0", nic.ID)
	}

	if nic.MAC != "00:50:56:ab:cd:ef" {
		t.Errorf("Expected MAC %q, got %q", "00:50:56:ab:cd:ef", nic.MAC)
	}
}

func TestBuilderAddNote(t *testing.T) {
	builder := NewBuilder()
	builder.AddNote("Test note 1")
	builder.AddNote("Test note 2")

	if len(builder.manifest.Notes) != 2 {
		t.Fatalf("Expected 2 notes, got %d", len(builder.manifest.Notes))
	}

	if builder.manifest.Notes[0] != "Test note 1" {
		t.Errorf("Expected note %q, got %q", "Test note 1", builder.manifest.Notes[0])
	}
}

func TestBuilderAddWarning(t *testing.T) {
	builder := NewBuilder()
	builder.AddWarning("export", "Test warning message")

	if len(builder.manifest.Warnings) != 1 {
		t.Fatalf("Expected 1 warning, got %d", len(builder.manifest.Warnings))
	}

	warning := builder.manifest.Warnings[0]
	if warning.Stage != "export" {
		t.Errorf("Expected stage %q, got %q", "export", warning.Stage)
	}

	if warning.Message != "Test warning message" {
		t.Errorf("Expected message %q, got %q", "Test warning message", warning.Message)
	}

	if warning.Timestamp == nil {
		t.Error("Expected warning timestamp to be set, got nil")
	}
}

func TestBuilderWithPipeline(t *testing.T) {
	builder := NewBuilder()
	builder.WithPipeline(true, true, true, true)

	if builder.manifest.Pipeline == nil {
		t.Fatal("Pipeline config is nil")
	}

	if !builder.manifest.Pipeline.Inspect.Enabled {
		t.Error("Expected inspect stage to be enabled")
	}

	if !builder.manifest.Pipeline.Fix.Enabled {
		t.Error("Expected fix stage to be enabled")
	}

	if !builder.manifest.Pipeline.Convert.Enabled {
		t.Error("Expected convert stage to be enabled")
	}

	if !builder.manifest.Pipeline.Validate.Enabled {
		t.Error("Expected validate stage to be enabled")
	}
}

func TestBuilderBuild(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.WithSource("vsphere", "vm-1234", "test-vm", "DC1", "govc-export")
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")
	builder.WithPipeline(true, true, true, true)

	manifest, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	if manifest == nil {
		t.Fatal("Build() returned nil manifest")
	}

	if manifest.ManifestVersion != CurrentVersion {
		t.Errorf("Expected version %q, got %q", CurrentVersion, manifest.ManifestVersion)
	}

	if len(manifest.Disks) != 1 {
		t.Errorf("Expected 1 disk, got %d", len(manifest.Disks))
	}
}

func TestBuilderBuildNoDisks(t *testing.T) {
	builder := NewBuilder()

	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error when building with no disks, got nil")
	}

	if !strings.Contains(err.Error(), "at least one disk") {
		t.Errorf("Expected 'at least one disk' error, got: %v", err)
	}
}

func TestBuilderBuildWithErrors(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("invalid id with spaces", "vmdk", diskPath, 1024, 0, "boot")

	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error when building with invalid disk ID, got nil")
	}

	if !strings.Contains(err.Error(), "invalid disk ID") {
		t.Errorf("Expected 'invalid disk ID' error, got: %v", err)
	}
}

func TestValidate(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	manifest, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}

	if err := Validate(manifest); err != nil {
		t.Errorf("Validate() failed: %v", err)
	}
}

func TestValidateInvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	manifest, _ := builder.Build()
	manifest.ManifestVersion = "99.0"

	err := Validate(manifest)
	if err == nil {
		t.Error("Expected error for invalid version, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported manifest version") {
		t.Errorf("Expected 'unsupported manifest version' error, got: %v", err)
	}
}

func TestValidate_NilManifest(t *testing.T) {
	err := Validate(nil)
	if err == nil {
		t.Error("Expected error for nil manifest, got nil")
	}

	if !strings.Contains(err.Error(), "manifest is nil") {
		t.Errorf("Expected 'manifest is nil' error, got: %v", err)
	}
}

func TestValidate_NoDisks(t *testing.T) {
	manifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Disks:           []DiskArtifact{},
	}

	err := Validate(manifest)
	if err == nil {
		t.Error("Expected error for manifest with no disks, got nil")
	}

	if !strings.Contains(err.Error(), "must have at least one disk") {
		t.Errorf("Expected 'must have at least one disk' error, got: %v", err)
	}
}

func TestValidate_WithVMMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")
	builder.WithVM(4, 8, "uefi", "linux", "Ubuntu 22.04", false)

	manifest, _ := builder.Build()

	err := Validate(manifest)
	if err != nil {
		t.Errorf("Validate failed for manifest with valid VM metadata: %v", err)
	}
}

func TestValidate_WithInvalidVMMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	manifest, _ := builder.Build()
	manifest.VM = &VMMetadata{
		CPU:      -1,
		MemGB:    8,
		Firmware: "uefi",
	}

	err := Validate(manifest)
	if err == nil {
		t.Error("Expected error for negative CPU, got nil")
	}

	if !strings.Contains(err.Error(), "cpu must be non-negative") {
		t.Errorf("Expected 'cpu must be non-negative' error, got: %v", err)
	}
}

func TestValidate_WithNICs(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")
	builder.AddNIC("nic-0", "00:11:22:33:44:55", "test-network")

	manifest, _ := builder.Build()

	err := Validate(manifest)
	if err != nil {
		t.Errorf("Validate failed for manifest with valid NICs: %v", err)
	}
}

func TestValidate_WithInvalidNIC(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	manifest, _ := builder.Build()
	manifest.NICs = []NICInfo{
		{
			MAC:     "invalid",
			Network: "test-network",
		},
	}

	err := Validate(manifest)
	if err == nil {
		t.Error("Expected error for invalid NIC MAC, got nil")
	}

	if !strings.Contains(err.Error(), "mac") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Expected MAC validation error, got: %v", err)
	}
}

func TestSerializeJSON(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	builder := NewBuilder()
	builder.WithSource("vsphere", "vm-1234", "test-vm", "DC1", "govc-export")
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	manifest, _ := builder.Build()

	data, err := ToJSON(manifest)
	if err != nil {
		t.Fatalf("ToJSON() failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("ToJSON() returned empty data")
	}

	// Verify it's valid JSON
	if !strings.Contains(string(data), "manifest_version") {
		t.Error("JSON output doesn't contain manifest_version")
	}
}

func TestWriteAndReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	manifestPath := filepath.Join(tmpDir, "manifest.json")

	builder := NewBuilder()
	builder.WithSource("vsphere", "vm-1234", "test-vm", "DC1", "govc-export")
	builder.AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot")

	originalManifest, _ := builder.Build()

	// Write to file
	if err := WriteToFile(originalManifest, manifestPath); err != nil {
		t.Fatalf("WriteToFile() failed: %v", err)
	}

	// Read back
	loadedManifest, err := ReadFromFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFromFile() failed: %v", err)
	}

	if loadedManifest.ManifestVersion != originalManifest.ManifestVersion {
		t.Error("Loaded manifest version doesn't match original")
	}

	if len(loadedManifest.Disks) != len(originalManifest.Disks) {
		t.Errorf("Expected %d disks, got %d", len(originalManifest.Disks), len(loadedManifest.Disks))
	}
}

func TestComputeSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test data for checksum")
	os.WriteFile(filePath, testData, 0644)

	checksum, err := ComputeSHA256(filePath)
	if err != nil {
		t.Fatalf("ComputeSHA256() failed: %v", err)
	}

	if len(checksum) != 64 {
		t.Errorf("Expected checksum length 64, got %d", len(checksum))
	}

	// Verify it's hexadecimal
	for _, c := range checksum {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Checksum contains invalid character: %c", c)
		}
	}
}

func TestBuilderMetadata(t *testing.T) {
	builder := NewBuilder()
	tags := map[string]string{
		"env": "production",
		"app": "webserver",
	}
	builder.WithMetadata("0.1.0", "job-123", tags)

	if builder.manifest.Metadata.HyperSDKVersion != "0.1.0" {
		t.Errorf("Expected transiva version %q, got %q", "0.1.0", builder.manifest.Metadata.HyperSDKVersion)
	}

	if builder.manifest.Metadata.JobID != "job-123" {
		t.Errorf("Expected job ID %q, got %q", "job-123", builder.manifest.Metadata.JobID)
	}

	if builder.manifest.Metadata.Tags["env"] != "production" {
		t.Error("Tags not set correctly")
	}
}

func TestBuilderChaining(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	// Test method chaining
	manifest, err := NewBuilder().
		WithSource("vsphere", "vm-1234", "test-vm", "DC1", "govc-export").
		WithVM(4, 16, "uefi", "linux", "Ubuntu 22.04", false).
		AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot").
		AddNIC("eth0", "00:50:56:ab:cd:ef", "VM Network").
		AddNote("Exported from vSphere 7.0").
		WithPipeline(true, true, true, true).
		WithMetadata("0.1.0", "job-123", map[string]string{"env": "prod"}).
		Build()

	if err != nil {
		t.Fatalf("Chained build failed: %v", err)
	}

	if manifest.Source == nil || manifest.Source.Provider != "vsphere" {
		t.Error("Source not set correctly in chained build")
	}

	if manifest.VM == nil || manifest.VM.CPU != 4 {
		t.Error("VM not set correctly in chained build")
	}

	if len(manifest.Disks) != 1 {
		t.Error("Disk not added correctly in chained build")
	}

	if len(manifest.NICs) != 1 {
		t.Error("NIC not added correctly in chained build")
	}

	if len(manifest.Notes) != 1 {
		t.Error("Note not added correctly in chained build")
	}

	if manifest.Pipeline == nil {
		t.Error("Pipeline not set correctly in chained build")
	}

	if manifest.Metadata == nil || manifest.Metadata.HyperSDKVersion != "0.1.0" {
		t.Error("Metadata not set correctly in chained build")
	}
}

func TestTimestamps(t *testing.T) {
	builder := NewBuilder()

	// Check that created_at is set
	if builder.manifest.Metadata.CreatedAt == nil {
		t.Error("Expected metadata.created_at to be set, got nil")
	}

	// Check that timestamp is recent
	now := time.Now()
	diff := now.Sub(*builder.manifest.Metadata.CreatedAt)
	if diff > time.Second {
		t.Errorf("Timestamp is too old: %v", diff)
	}

	// Add a warning and check its timestamp
	builder.AddWarning("export", "test warning")
	if len(builder.manifest.Warnings) > 0 {
		if builder.manifest.Warnings[0].Timestamp == nil {
			t.Error("Expected warning timestamp to be set, got nil")
		}
	}
}

func TestToYAML(t *testing.T) {
	// Create a simple manifest to test YAML serialization
	manifest := &ArtifactManifest{
		ManifestVersion: "1.0.0",
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
		VM: &VMMetadata{
			CPU:   4,
			MemGB: 16,
		},
	}

	// Convert to YAML
	yamlBytes, err := ToYAML(manifest)
	if err != nil {
		t.Fatalf("Failed to convert to YAML: %v", err)
	}

	if len(yamlBytes) == 0 {
		t.Error("Expected non-empty YAML output")
	}

	// Verify YAML contains expected content
	yamlStr := string(yamlBytes)
	if !strings.Contains(yamlStr, "TestVM") {
		t.Error("YAML should contain VM name")
	}
	if !strings.Contains(yamlStr, "vsphere") {
		t.Error("YAML should contain source provider")
	}
}

func TestFromYAML(t *testing.T) {
	// Create a test YAML
	yamlContent := `manifest_version: "1.0"
metadata:
  created_at: 2024-01-01T00:00:00Z
source:
  provider: vsphere
  vm_id: vm-1234
  vm_name: TestVM
  datacenter: DC1
vm:
  cpu: 4
  mem_gb: 16
  firmware: uefi
output:
  directory: /output
  format: qcow2
  filename: test.qcow2
`

	manifest, err := FromYAML([]byte(yamlContent))
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if manifest.Source.VMName != "TestVM" {
		t.Errorf("Expected VM name 'TestVM', got '%s'", manifest.Source.VMName)
	}

	if manifest.Source.Provider != "vsphere" {
		t.Errorf("Expected provider 'vsphere', got '%s'", manifest.Source.Provider)
	}

	if manifest.Output.Format != "qcow2" {
		t.Errorf("Expected format 'qcow2', got '%s'", manifest.Output.Format)
	}
}

func TestFromYAML_InvalidYAML(t *testing.T) {
	invalidYAML := []byte("invalid: yaml: content: :\n")

	_, err := FromYAML(invalidYAML)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestWithOutput(t *testing.T) {
	builder := NewBuilder()
	builder.WithOutput("/output", "vmdk", "test.vmdk")

	// Check the output was set in the manifest (don't call Build which requires disks)
	if builder.manifest.Output == nil {
		t.Fatal("Expected Output to be set")
	}

	if builder.manifest.Output.Format != "vmdk" {
		t.Errorf("Expected output format 'vmdk', got '%s'", builder.manifest.Output.Format)
	}

	if builder.manifest.Output.Directory != "/output" {
		t.Errorf("Expected output directory '/output', got '%s'", builder.manifest.Output.Directory)
	}

	if builder.manifest.Output.Filename != "test.vmdk" {
		t.Errorf("Expected output filename 'test.vmdk', got '%s'", builder.manifest.Output.Filename)
	}
}

func TestWithOptions(t *testing.T) {
	builder := NewBuilder()
	builder.WithOptions(true, 2)

	// Check the options were set in the manifest (don't call Build which requires disks)
	if builder.manifest.Options == nil {
		t.Fatal("Expected options to be set")
	}

	// Verify options are set correctly
	if !builder.manifest.Options.DryRun {
		t.Error("Expected DryRun option to be true")
	}

	if builder.manifest.Options.Verbose != 2 {
		t.Errorf("Expected Verbose to be 2, got %d", builder.manifest.Options.Verbose)
	}

	// Verify report config is initialized
	if builder.manifest.Options.Report == nil {
		t.Error("Expected Report config to be initialized")
	}
}

func TestWithOptions_FalseValues(t *testing.T) {
	builder := NewBuilder()
	builder.WithOptions(false, 0)

	// Check the options were set
	if builder.manifest.Options == nil {
		t.Fatal("Expected options to be set")
	}

	if builder.manifest.Options.DryRun {
		t.Error("Expected DryRun to be false")
	}

	if builder.manifest.Options.Verbose != 0 {
		t.Errorf("Expected Verbose to be 0, got %d", builder.manifest.Options.Verbose)
	}
}

func TestValidateVMMetadata_ValidFirmware(t *testing.T) {
	validFirmwareValues := []string{"bios", "uefi", "unknown"}

	for _, firmware := range validFirmwareValues {
		vm := &VMMetadata{
			Firmware: firmware,
			CPU:      4,
			MemGB:    16,
		}

		err := validateVMMetadata(vm)
		if err != nil {
			t.Errorf("validateVMMetadata failed for firmware %q: %v", firmware, err)
		}
	}
}

func TestValidateVMMetadata_InvalidFirmware(t *testing.T) {
	vm := &VMMetadata{
		Firmware: "invalid-firmware",
		CPU:      4,
		MemGB:    16,
	}

	err := validateVMMetadata(vm)
	if err == nil {
		t.Error("Expected error for invalid firmware value")
	}

	expectedError := `vm.firmware "invalid-firmware" must be one of: bios, uefi, unknown`
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}

func TestValidateVMMetadata_NegativeCPU(t *testing.T) {
	vm := &VMMetadata{
		Firmware: "bios",
		CPU:      -1,
		MemGB:    16,
	}

	err := validateVMMetadata(vm)
	if err == nil {
		t.Error("Expected error for negative CPU count")
	}

	if err.Error() != "vm.cpu must be non-negative (got: -1)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestValidateVMMetadata_NegativeMemory(t *testing.T) {
	vm := &VMMetadata{
		Firmware: "uefi",
		CPU:      4,
		MemGB:    -8,
	}

	err := validateVMMetadata(vm)
	if err == nil {
		t.Error("Expected error for negative memory")
	}

	if err.Error() != "vm.mem_gb must be non-negative (got: -8)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestValidateVMMetadata_EmptyFirmware(t *testing.T) {
	vm := &VMMetadata{
		Firmware: "",
		CPU:      4,
		MemGB:    16,
	}

	err := validateVMMetadata(vm)
	if err != nil {
		t.Errorf("Empty firmware should be valid (optional field): %v", err)
	}
}

func TestValidateNIC_ValidMAC(t *testing.T) {
	validMACs := []string{
		"00:11:22:33:44:55",
		"AA:BB:CC:DD:EE:FF",
		"00-11-22-33-44-55",
		"aa:bb:cc:dd:ee:ff",
	}

	for i, mac := range validMACs {
		nic := NICInfo{
			MAC:     mac,
			Network: "test-network",
		}

		err := validateNIC(nic, i)
		if err != nil {
			t.Errorf("validateNIC failed for valid MAC %q: %v", mac, err)
		}
	}
}

func TestValidateNIC_InvalidMAC(t *testing.T) {
	nic := NICInfo{
		MAC:     "invalid",
		Network: "test-network",
	}

	err := validateNIC(nic, 0)
	if err == nil {
		t.Error("Expected error for invalid MAC address")
	}

	expectedError := `nics[0].mac "invalid" appears to be invalid (too short)`
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}

func TestValidateNIC_EmptyMAC(t *testing.T) {
	nic := NICInfo{
		MAC:     "",
		Network: "test-network",
	}

	err := validateNIC(nic, 0)
	if err != nil {
		t.Errorf("Empty MAC should be valid (optional field): %v", err)
	}
}

func TestValidateDisk_EmptyID(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:           "",
		SourceFormat: "vmdk",
		Bytes:        1024,
		LocalPath:    diskPath,
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for empty disk ID, got nil")
	}

	if !strings.Contains(err.Error(), "id is required") {
		t.Errorf("Expected 'id is required' error, got: %v", err)
	}
}

func TestValidateDisk_InvalidIDPattern(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:           "disk@invalid#chars",
		SourceFormat: "vmdk",
		Bytes:        1024,
		LocalPath:    diskPath,
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for invalid disk ID pattern, got nil")
	}

	if !strings.Contains(err.Error(), "must match pattern") {
		t.Errorf("Expected 'must match pattern' error, got: %v", err)
	}
}

func TestValidateDisk_DuplicateID(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:           "disk-0",
		SourceFormat: "vmdk",
		Bytes:        1024,
		LocalPath:    diskPath,
	}

	diskIDs := map[string]bool{"disk-0": true}
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for duplicate disk ID, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate disk ID") {
		t.Errorf("Expected 'duplicate disk ID' error, got: %v", err)
	}
}

func TestValidateDisk_InvalidSourceFormat(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:           "disk-0",
		SourceFormat: "invalid-format",
		Bytes:        1024,
		LocalPath:    diskPath,
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for invalid source format, got nil")
	}

	if !strings.Contains(err.Error(), "source_format") {
		t.Errorf("Expected 'source_format' error, got: %v", err)
	}
}

func TestValidateDisk_NegativeBytes(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:           "disk-0",
		SourceFormat: "vmdk",
		Bytes:        -100,
		LocalPath:    diskPath,
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for negative bytes, got nil")
	}

	if !strings.Contains(err.Error(), "must be non-negative") {
		t.Errorf("Expected 'must be non-negative' error, got: %v", err)
	}
}

func TestValidateDisk_EmptyLocalPath(t *testing.T) {
	disk := DiskArtifact{
		ID:           "disk-0",
		SourceFormat: "vmdk",
		Bytes:        1024,
		LocalPath:    "",
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for empty local path, got nil")
	}

	if !strings.Contains(err.Error(), "local_path is required") {
		t.Errorf("Expected 'local_path is required' error, got: %v", err)
	}
}

func TestValidateDisk_NonexistentPath(t *testing.T) {
	disk := DiskArtifact{
		ID:           "disk-0",
		SourceFormat: "vmdk",
		Bytes:        1024,
		LocalPath:    "/nonexistent/path/test.vmdk",
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for nonexistent path, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestValidateDisk_InvalidChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:           "disk-0",
		SourceFormat: "vmdk",
		Bytes:        1024,
		LocalPath:    diskPath,
		Checksum:     "invalid-checksum-format",
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for invalid checksum format, got nil")
	}

	if !strings.Contains(err.Error(), "checksum must match format") {
		t.Errorf("Expected 'checksum must match format' error, got: %v", err)
	}
}

func TestValidateDisk_NegativeBootOrder(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:            "disk-0",
		SourceFormat:  "vmdk",
		Bytes:         1024,
		LocalPath:     diskPath,
		BootOrderHint: -1,
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for negative boot order, got nil")
	}

	if !strings.Contains(err.Error(), "boot_order_hint must be non-negative") {
		t.Errorf("Expected 'boot_order_hint must be non-negative' error, got: %v", err)
	}
}

func TestValidateDisk_InvalidDiskType(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	disk := DiskArtifact{
		ID:           "disk-0",
		SourceFormat: "vmdk",
		Bytes:        1024,
		LocalPath:    diskPath,
		DiskType:     "invalid-type",
	}

	diskIDs := make(map[string]bool)
	err := validateDisk(disk, 0, diskIDs)
	if err == nil {
		t.Error("Expected error for invalid disk type, got nil")
	}

	if !strings.Contains(err.Error(), "disk_type") {
		t.Errorf("Expected 'disk_type' error, got: %v", err)
	}
}

func TestValidateDisk_ValidDiskTypes(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	validTypes := []string{"boot", "data", "unknown"}

	for _, diskType := range validTypes {
		disk := DiskArtifact{
			ID:           "disk-" + diskType,
			SourceFormat: "vmdk",
			Bytes:        1024,
			LocalPath:    diskPath,
			DiskType:     diskType,
		}

		diskIDs := make(map[string]bool)
		err := validateDisk(disk, 0, diskIDs)
		if err != nil {
			t.Errorf("validateDisk failed for valid disk type %q: %v", diskType, err)
		}
	}
}

func TestValidateDisk_ValidSourceFormats(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")
	os.WriteFile(diskPath, []byte("test"), 0644)

	validFormats := []string{"vmdk", "qcow2", "raw", "vhd", "vhdx", "vdi"}

	for _, format := range validFormats {
		disk := DiskArtifact{
			ID:           "disk-" + format,
			SourceFormat: format,
			Bytes:        1024,
			LocalPath:    diskPath,
		}

		diskIDs := make(map[string]bool)
		err := validateDisk(disk, 0, diskIDs)
		if err != nil {
			t.Errorf("validateDisk failed for valid format %q: %v", format, err)
		}
	}
}

func TestVerifyChecksums_MatchingChecksum(t *testing.T) {
	// Create a temporary file with known content
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test-disk.vmdk")

	content := []byte("test disk content for checksum")
	if err := os.WriteFile(diskPath, content, 0644); err != nil {
		t.Fatalf("Failed to create test disk file: %v", err)
	}

	// Compute the actual checksum
	actualChecksum, err := ComputeSHA256(diskPath)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	// Create manifest with matching checksum
	manifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Disks: []DiskArtifact{
			{
				ID:           "disk1",
				SourceFormat: "vmdk",
				Bytes:        int64(len(content)),
				LocalPath:    diskPath,
				Checksum:     "sha256:" + actualChecksum,
			},
		},
	}

	results, err := VerifyChecksums(manifest)
	if err != nil {
		t.Fatalf("VerifyChecksums failed: %v", err)
	}

	if !results["disk1"] {
		t.Error("Expected checksum to match for disk1")
	}
}

func TestVerifyChecksums_MismatchingChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test-disk.vmdk")

	content := []byte("test disk content")
	if err := os.WriteFile(diskPath, content, 0644); err != nil {
		t.Fatalf("Failed to create test disk file: %v", err)
	}

	// Create manifest with wrong checksum
	manifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Disks: []DiskArtifact{
			{
				ID:           "disk1",
				SourceFormat: "vmdk",
				Bytes:        int64(len(content)),
				LocalPath:    diskPath,
				Checksum:     "sha256:0000000000000000000000000000000000000000000000000000000000000000",
			},
		},
	}

	results, err := VerifyChecksums(manifest)
	if err == nil {
		t.Error("Expected error for mismatching checksum")
	}

	if results["disk1"] {
		t.Error("Expected checksum not to match for disk1")
	}
}

func TestVerifyChecksums_NoChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test-disk.vmdk")

	content := []byte("test disk content")
	if err := os.WriteFile(diskPath, content, 0644); err != nil {
		t.Fatalf("Failed to create test disk file: %v", err)
	}

	// Create manifest without checksum
	manifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Disks: []DiskArtifact{
			{
				ID:           "disk1",
				SourceFormat: "vmdk",
				Bytes:        int64(len(content)),
				LocalPath:    diskPath,
				Checksum:     "", // No checksum
			},
		},
	}

	results, err := VerifyChecksums(manifest)
	if err != nil {
		t.Fatalf("VerifyChecksums should not fail for disks without checksums: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected no results for disk without checksum, got %d results", len(results))
	}
}

func TestVerifyChecksums_InvalidDiskPath(t *testing.T) {
	manifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Disks: []DiskArtifact{
			{
				ID:           "disk1",
				SourceFormat: "vmdk",
				Bytes:        1024,
				LocalPath:    "/nonexistent/path/disk.vmdk",
				Checksum:     "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			},
		},
	}

	_, err := VerifyChecksums(manifest)
	if err == nil {
		t.Error("Expected error for nonexistent disk file")
	}
}

func TestToJSON(t *testing.T) {
	manifest := &ArtifactManifest{
		ManifestVersion: "1.0.0",
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
		VM: &VMMetadata{
			CPU:   4,
			MemGB: 16,
		},
	}

	jsonBytes, err := ToJSON(manifest)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("Expected non-empty JSON output")
	}

	// Verify it's valid JSON by parsing it back
	_, err = FromJSON(jsonBytes)
	if err != nil {
		t.Errorf("Generated JSON is not valid: %v", err)
	}
}

func TestFromJSON(t *testing.T) {
	jsonData := []byte(`{
		"manifest_version": "1.0.0",
		"source": {
			"provider": "vsphere",
			"vm_name": "TestVM"
		},
		"vm": {
			"cpu": 4,
			"mem_gb": 16
		}
	}`)

	manifest, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if manifest.ManifestVersion != "1.0.0" {
		t.Errorf("Expected ManifestVersion '1.0.0', got '%s'", manifest.ManifestVersion)
	}

	if manifest.Source.VMName != "TestVM" {
		t.Errorf("Expected VMName 'TestVM', got '%s'", manifest.Source.VMName)
	}
}

func TestFromJSON_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{invalid json`)

	_, err := FromJSON(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestWriteToFile_InvalidPath(t *testing.T) {
	manifest := &ArtifactManifest{
		ManifestVersion: "1.0.0",
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
	}

	// Try to write to a directory that doesn't exist
	err := WriteToFile(manifest, "/nonexistent/directory/manifest.json")
	if err == nil {
		t.Error("Expected error when writing to invalid path")
	}
}

func TestWriteToFile_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test.yaml")

	manifest := &ArtifactManifest{
		ManifestVersion: "1.0.0",
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
		VM: &VMMetadata{
			CPU:   4,
			MemGB: 16,
		},
	}

	// Write as YAML
	err := WriteToFile(manifest, yamlPath)
	if err != nil {
		t.Fatalf("WriteToFile (YAML) failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		t.Error("YAML file was not created")
	}

	// Read back and verify
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("Failed to read YAML file: %v", err)
	}

	if len(data) == 0 {
		t.Error("YAML file is empty")
	}
}

func TestWriteToFile_YML(t *testing.T) {
	tmpDir := t.TempDir()
	ymlPath := filepath.Join(tmpDir, "test.yml")

	manifest := &ArtifactManifest{
		ManifestVersion: "1.0.0",
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
	}

	// Write as YML (also YAML)
	err := WriteToFile(manifest, ymlPath)
	if err != nil {
		t.Fatalf("WriteToFile (YML) failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
		t.Error("YML file was not created")
	}
}

func TestWriteToFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "test.json")

	manifest := &ArtifactManifest{
		ManifestVersion: "1.0.0",
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
	}

	// Write as JSON
	err := WriteToFile(manifest, jsonPath)
	if err != nil {
		t.Fatalf("WriteToFile (JSON) failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("JSON file was not created")
	}

	// Read back and verify it's valid JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	_, err = FromJSON(data)
	if err != nil {
		t.Errorf("Generated JSON file is not valid: %v", err)
	}
}

func TestReadFromFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test-disk.vmdk")

	// Create a test disk file
	if err := os.WriteFile(diskPath, []byte("test disk"), 0644); err != nil {
		t.Fatalf("Failed to create disk file: %v", err)
	}

	manifestPath := filepath.Join(tmpDir, "test.json")

	// Create a valid manifest
	manifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
		Disks: []DiskArtifact{
			{
				ID:           "disk1",
				SourceFormat: "vmdk",
				Bytes:        9,
				LocalPath:    diskPath,
			},
		},
	}

	// Write to file
	if err := WriteToFile(manifest, manifestPath); err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	// Read it back
	loaded, err := ReadFromFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFromFile failed: %v", err)
	}

	if loaded.Source.VMName != "TestVM" {
		t.Errorf("Expected VMName 'TestVM', got '%s'", loaded.Source.VMName)
	}

	if len(loaded.Disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(loaded.Disks))
	}
}

func TestReadFromFile_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test-disk.vmdk")

	// Create a test disk file
	if err := os.WriteFile(diskPath, []byte("test disk"), 0644); err != nil {
		t.Fatalf("Failed to create disk file: %v", err)
	}

	manifestPath := filepath.Join(tmpDir, "test.yaml")

	// Create a valid manifest
	manifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM-YAML",
		},
		Disks: []DiskArtifact{
			{
				ID:           "disk1",
				SourceFormat: "vmdk",
				Bytes:        9,
				LocalPath:    diskPath,
			},
		},
	}

	// Write to file
	if err := WriteToFile(manifest, manifestPath); err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	// Read it back
	loaded, err := ReadFromFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFromFile failed: %v", err)
	}

	if loaded.Source.VMName != "TestVM-YAML" {
		t.Errorf("Expected VMName 'TestVM-YAML', got '%s'", loaded.Source.VMName)
	}
}

func TestReadFromFile_NonexistentFile(t *testing.T) {
	_, err := ReadFromFile("/nonexistent/manifest.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestReadFromFile_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "invalid.json")

	// Write an invalid manifest (missing disks, which is required)
	invalidManifest := &ArtifactManifest{
		ManifestVersion: CurrentVersion,
		Source: &SourceMetadata{
			Provider: "vsphere",
			VMName:   "TestVM",
		},
		Disks: []DiskArtifact{}, // Empty disks - will fail validation
	}

	if err := WriteToFile(invalidManifest, manifestPath); err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	// Try to read it back - should fail validation
	_, err := ReadFromFile(manifestPath)
	if err == nil {
		t.Error("Expected validation error for manifest without disks")
	}
}

func TestAddDiskWithChecksum_ComputeTrue(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")

	// Create a test disk file
	content := []byte("test disk content")
	if err := os.WriteFile(diskPath, content, 0644); err != nil {
		t.Fatalf("Failed to create disk file: %v", err)
	}

	builder := NewBuilder()
	builder.WithSource("vsphere", "vcenter.example.com", "", "", "TestVM")
	builder.AddDiskWithChecksum("disk1", "vmdk", diskPath, int64(len(content)), 0, "boot", true)

	if len(builder.errors) > 0 {
		t.Fatalf("Builder errors: %v", builder.errors)
	}

	if len(builder.manifest.Disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(builder.manifest.Disks))
	}

	disk := builder.manifest.Disks[0]
	if disk.Checksum == "" {
		t.Error("Expected checksum to be computed")
	}

	if !strings.HasPrefix(disk.Checksum, "sha256:") {
		t.Errorf("Expected checksum to start with 'sha256:', got '%s'", disk.Checksum)
	}
}

func TestAddDiskWithChecksum_ComputeFalse(t *testing.T) {
	tmpDir := t.TempDir()
	diskPath := filepath.Join(tmpDir, "test.vmdk")

	// Create a test disk file
	content := []byte("test disk content")
	if err := os.WriteFile(diskPath, content, 0644); err != nil {
		t.Fatalf("Failed to create disk file: %v", err)
	}

	builder := NewBuilder()
	builder.WithSource("vsphere", "vcenter.example.com", "", "", "TestVM")
	builder.AddDiskWithChecksum("disk1", "vmdk", diskPath, int64(len(content)), 0, "boot", false)

	if len(builder.manifest.Disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(builder.manifest.Disks))
	}

	disk := builder.manifest.Disks[0]
	if disk.Checksum != "" {
		t.Errorf("Expected no checksum when computeChecksum=false, got '%s'", disk.Checksum)
	}
}

func TestAddDiskWithChecksum_InvalidPath(t *testing.T) {
	builder := NewBuilder()
	builder.WithSource("vsphere", "vcenter.example.com", "", "", "TestVM")
	builder.AddDiskWithChecksum("disk1", "vmdk", "/nonexistent/disk.vmdk", 1024, 0, "boot", true)

	if len(builder.errors) == 0 {
		t.Error("Expected error for nonexistent disk file")
	}
}

func TestComputeSHA256_NonexistentFile(t *testing.T) {
	_, err := ComputeSHA256("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestWithMetadata(t *testing.T) {
	builder := NewBuilder()
	builder.WithSource("vsphere", "vcenter.example.com", "", "", "TestVM")

	tags := map[string]string{
		"environment": "production",
		"team":        "platform",
		"owner":       "admin",
	}

	builder.WithMetadata("1.0.0", "job-123", tags)

	if builder.manifest.Metadata == nil {
		t.Fatal("Expected metadata to be set")
	}

	if builder.manifest.Metadata.HyperSDKVersion != "1.0.0" {
		t.Errorf("Expected HyperSDKVersion='1.0.0', got '%s'", builder.manifest.Metadata.HyperSDKVersion)
	}

	if builder.manifest.Metadata.JobID != "job-123" {
		t.Errorf("Expected JobID='job-123', got '%s'", builder.manifest.Metadata.JobID)
	}

	if len(builder.manifest.Metadata.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(builder.manifest.Metadata.Tags))
	}

	if builder.manifest.Metadata.Tags["environment"] != "production" {
		t.Errorf("Expected tag environment='production', got '%s'", builder.manifest.Metadata.Tags["environment"])
	}

	if builder.manifest.Metadata.Tags["team"] != "platform" {
		t.Errorf("Expected tag team='platform', got '%s'", builder.manifest.Metadata.Tags["team"])
	}

	if builder.manifest.Metadata.Tags["owner"] != "admin" {
		t.Errorf("Expected tag owner='admin', got '%s'", builder.manifest.Metadata.Tags["owner"])
	}
}

func TestWithMetadata_NilTags(t *testing.T) {
	builder := NewBuilder()
	builder.WithSource("vsphere", "vcenter.example.com", "", "", "TestVM")

	// Call WithMetadata with nil tags
	builder.WithMetadata("1.0.0", "job-123", nil)

	if builder.manifest.Metadata == nil {
		t.Fatal("Expected metadata to be set")
	}

	if builder.manifest.Metadata.HyperSDKVersion != "1.0.0" {
		t.Errorf("Expected HyperSDKVersion='1.0.0', got '%s'", builder.manifest.Metadata.HyperSDKVersion)
	}

	if builder.manifest.Metadata.JobID != "job-123" {
		t.Errorf("Expected JobID='job-123', got '%s'", builder.manifest.Metadata.JobID)
	}

	// Tags should not be set when nil is passed
	if builder.manifest.Metadata.Tags != nil && len(builder.manifest.Metadata.Tags) > 0 {
		t.Error("Expected tags to remain empty when nil is passed")
	}
}

func TestWithMetadata_UpdateExisting(t *testing.T) {
	builder := NewBuilder()
	builder.WithSource("vsphere", "vcenter.example.com", "", "", "TestVM")

	// First call to WithMetadata
	tags1 := map[string]string{"env": "dev"}
	builder.WithMetadata("1.0.0", "job-123", tags1)

	// Second call to WithMetadata (metadata already exists)
	tags2 := map[string]string{"env": "prod", "team": "ops"}
	builder.WithMetadata("2.0.0", "job-456", tags2)

	if builder.manifest.Metadata == nil {
		t.Fatal("Expected metadata to be set")
	}

	// Should be updated to latest values
	if builder.manifest.Metadata.HyperSDKVersion != "2.0.0" {
		t.Errorf("Expected HyperSDKVersion='2.0.0', got '%s'", builder.manifest.Metadata.HyperSDKVersion)
	}

	if builder.manifest.Metadata.JobID != "job-456" {
		t.Errorf("Expected JobID='job-456', got '%s'", builder.manifest.Metadata.JobID)
	}

	// Tags should be replaced with new tags
	if len(builder.manifest.Metadata.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(builder.manifest.Metadata.Tags))
	}

	if builder.manifest.Metadata.Tags["env"] != "prod" {
		t.Errorf("Expected tag env='prod', got '%s'", builder.manifest.Metadata.Tags["env"])
	}

	if builder.manifest.Metadata.Tags["team"] != "ops" {
		t.Errorf("Expected tag team='ops', got '%s'", builder.manifest.Metadata.Tags["team"])
	}
}
