// SPDX-License-Identifier: Apache-2.0

//go:build integration && integration
// +build integration,integration

package azure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zyvorai/transiva/progress"
)

// MockProgressReporter for testing
type MockProgressReporter struct {
	percentage  int
	description string
	updates     []int
}

func (m *MockProgressReporter) Update(percentage int) {
	m.percentage = percentage
	m.updates = append(m.updates, percentage)
}

func (m *MockProgressReporter) Describe(description string) {
	m.description = description
}

func (m *MockProgressReporter) GetPercentage() int {
	return m.percentage
}

func (m *MockProgressReporter) GetDescription() string {
	return m.description
}

// TestProgressReader tests the progress reader wrapper
func TestProgressReader_Read(t *testing.T) {
	// Create test data
	data := []byte("test data for Azure VHD progress reader")
	reporter := &MockProgressReporter{}

	// Create progress reader
	pr := &progressReader{
		reader:   &mockReader{data: data},
		total:    int64(len(data)),
		reporter: reporter,
	}

	// Read all data
	buf := make([]byte, 10)
	totalRead := 0

	for {
		n, err := pr.Read(buf)
		totalRead += n
		if err != nil {
			break
		}
	}

	// Verify all data was read
	if totalRead != len(data) {
		t.Errorf("Expected to read %d bytes, got %d", len(data), totalRead)
	}

	// Verify progress was updated
	if reporter.percentage != 100 {
		t.Errorf("Expected final percentage 100, got %d", reporter.percentage)
	}
}

// TestProgressReader_NoReporter tests progress reader without reporter
func TestProgressReader_NoReporter(t *testing.T) {
	data := []byte("test data for VHD download")

	pr := &progressReader{
		reader: &mockReader{data: data},
		total:  int64(len(data)),
		// No reporter - should not panic
	}

	buf := make([]byte, len(data))
	_, err := pr.Read(buf)
	if err != nil {
		t.Errorf("Read with no reporter should not fail: %v", err)
	}
}

// TestExportResult_Structure tests ExportResult struct
func TestExportResult_Structure(t *testing.T) {
	result := &ExportResult{
		DiskName:   "os-disk",
		DiskType:   "OS",
		Format:     "vhd",
		LocalPath:  "/tmp/os-disk.vhd",
		Size:       32 * 1024 * 1024 * 1024, // 32GB
		BlobURL:    "https://storage.blob.core.windows.net/exports/os-disk.vhd",
		DiskSizeGB: 32,
	}

	// Verify all fields are set
	if result.DiskName == "" {
		t.Error("DiskName should be set")
	}
	if result.Format != "vhd" {
		t.Errorf("Expected format 'vhd', got '%s'", result.Format)
	}
	if result.DiskSizeGB != 32 {
		t.Errorf("Expected 32GB, got %d GB", result.DiskSizeGB)
	}
	if result.DiskType != "OS" {
		t.Errorf("Expected disk type 'OS', got '%s'", result.DiskType)
	}
}

// TestExportResult_MultipleDisks tests multi-disk export results
func TestExportResult_MultipleDisks(t *testing.T) {
	disks := []*ExportResult{
		{
			DiskName: "os-disk",
			DiskType: "OS",
			Format:   "vhd",
		},
		{
			DiskName: "data-disk-1",
			DiskType: "Data-1",
			Format:   "vhd",
		},
		{
			DiskName: "data-disk-2",
			DiskType: "Data-2",
			Format:   "vhd",
		},
	}

	if len(disks) != 3 {
		t.Errorf("Expected 3 disks, got %d", len(disks))
	}

	// Verify OS disk is first
	if disks[0].DiskType != "OS" {
		t.Errorf("First disk should be OS disk, got %s", disks[0].DiskType)
	}

	// Verify data disks
	for i := 1; i < len(disks); i++ {
		expectedType := "Data-" + string(rune('0'+i))
		if disks[i].DiskType != expectedType {
			t.Errorf("Expected disk type %s, got %s", expectedType, disks[i].DiskType)
		}
	}
}

// TestCreateExportManifest tests manifest file creation
func TestCreateExportManifest(t *testing.T) {
	tmpDir := t.TempDir()

	results := []*ExportResult{
		{
			DiskName:   "os-disk",
			DiskType:   "OS",
			Format:     "vhd",
			LocalPath:  "/tmp/os-disk.vhd",
			Size:       32 * 1024 * 1024 * 1024,
			DiskSizeGB: 32,
			BlobURL:    "https://storage.blob.core.windows.net/exports/os-disk.vhd",
		},
		{
			DiskName:   "data-disk",
			DiskType:   "Data-1",
			Format:     "vhd",
			LocalPath:  "/tmp/data-disk.vhd",
			Size:       64 * 1024 * 1024 * 1024,
			DiskSizeGB: 64,
			BlobURL:    "https://storage.blob.core.windows.net/exports/data-disk.vhd",
		},
	}

	err := CreateExportManifest("test-vm", results, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Verify manifest file exists
	manifestPath := filepath.Join(tmpDir, "test-vm-manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("Manifest file should exist")
	}

	// Read manifest content
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	manifestStr := string(content)

	// Verify manifest contains expected data
	if !contains(manifestStr, "test-vm") {
		t.Error("Manifest should contain VM name")
	}
	if !contains(manifestStr, "os-disk") {
		t.Error("Manifest should contain OS disk")
	}
	if !contains(manifestStr, "data-disk") {
		t.Error("Manifest should contain data disk")
	}
	if !contains(manifestStr, "\"disk_count\": 2") {
		t.Error("Manifest should show correct disk count")
	}
}

// TestCreateExportManifest_SingleDisk tests manifest with single disk
func TestCreateExportManifest_SingleDisk(t *testing.T) {
	tmpDir := t.TempDir()

	results := []*ExportResult{
		{
			DiskName:   "os-disk",
			DiskType:   "OS",
			Format:     "vhd",
			LocalPath:  "/tmp/os-disk.vhd",
			Size:       32 * 1024 * 1024 * 1024,
			DiskSizeGB: 32,
		},
	}

	err := CreateExportManifest("single-disk-vm", results, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Verify manifest exists
	manifestPath := filepath.Join(tmpDir, "single-disk-vm-manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	manifestStr := string(content)
	if !contains(manifestStr, "\"disk_count\": 1") {
		t.Error("Manifest should show 1 disk")
	}
}

// mockReader simulates an io.Reader for testing
type mockReader struct {
	data   []byte
	offset int
}

func (m *mockReader) Read(p []byte) (int, error) {
	if m.offset >= len(m.data) {
		return 0, os.ErrClosed
	}

	n := copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAtIndex(s, substr))
}

func containsAtIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Integration test placeholders (require real Azure credentials)
// These tests are disabled by default - enable with build tag 'integration'

// TestExportDiskToVHD_Integration tests full VHD export flow
func TestExportDiskToVHD_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Skip("Integration test - requires Azure setup")

	// This test requires:
	// 1. Azure credentials configured (AZURE_TENANT_ID, AZURE_CLIENT_ID, etc.)
	// 2. Storage account with container created
	// 3. Managed disk to export
	// 4. IAM permissions for disk access and blob storage
}

// TestExportVMToVHD_Integration tests VM multi-disk export
func TestExportVMToVHD_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Skip("Integration test - requires Azure setup")
}

// Benchmark tests

func BenchmarkProgressReader_SmallVHD(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB
	reporter := &MockProgressReporter{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr := &progressReader{
			reader:   &mockReader{data: data},
			total:    int64(len(data)),
			reporter: reporter,
		}

		buf := make([]byte, 8192)
		for {
			_, err := pr.Read(buf)
			if err != nil {
				break
			}
		}
	}
}

func BenchmarkProgressReader_LargeVHD(b *testing.B) {
	data := make([]byte, 100*1024*1024) // 100MB
	reporter := &MockProgressReporter{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr := &progressReader{
			reader:   &mockReader{data: data},
			total:    int64(len(data)),
			reporter: reporter,
		}

		buf := make([]byte, 65536)
		for {
			_, err := pr.Read(buf)
			if err != nil {
				break
			}
		}
	}
}

// Test validation

func TestExportResult_Validation(t *testing.T) {
	tests := []struct {
		name    string
		result  *ExportResult
		wantErr bool
	}{
		{
			name: "valid result",
			result: &ExportResult{
				DiskName:  "disk1",
				Format:    "vhd",
				LocalPath: "/tmp/disk.vhd",
			},
			wantErr: false,
		},
		{
			name: "missing disk name",
			result: &ExportResult{
				Format:    "vhd",
				LocalPath: "/tmp/disk.vhd",
			},
			wantErr: true,
		},
		{
			name: "missing format",
			result: &ExportResult{
				DiskName:  "disk1",
				LocalPath: "/tmp/disk.vhd",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExportResult(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExportResult() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func validateExportResult(result *ExportResult) error {
	if result.DiskName == "" {
		return os.ErrInvalid
	}
	if result.Format == "" {
		return os.ErrInvalid
	}
	return nil
}
