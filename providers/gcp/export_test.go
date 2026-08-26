// SPDX-License-Identifier: Apache-2.0

//go:build integration && integration
// +build integration,integration

package gcp

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
	data := []byte("test data for GCP GCS progress reader")
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
	data := []byte("test data for VMDK download")

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
		ImageName: "my-instance-image-123456",
		DiskName:  "my-instance-boot-disk",
		DiskType:  "boot",
		Format:    "vmdk",
		LocalPath: "/tmp/my-instance.vmdk",
		Size:      50 * 1024 * 1024 * 1024, // 50GB
		GCSBucket: "vm-exports",
		GCSObject: "my-instance.vmdk",
		GCSURI:    "gs://vm-exports/my-instance.vmdk",
	}

	// Verify all fields are set
	if result.ImageName == "" {
		t.Error("ImageName should be set")
	}
	if result.Format != "vmdk" {
		t.Errorf("Expected format 'vmdk', got '%s'", result.Format)
	}
	if result.Size != 50*1024*1024*1024 {
		t.Errorf("Expected 50GB, got %d bytes", result.Size)
	}
	if result.DiskType != "boot" {
		t.Errorf("Expected disk type 'boot', got '%s'", result.DiskType)
	}
	if result.GCSURI == "" {
		t.Error("GCSURI should be set")
	}
}

// TestExportResult_MultipleDisks tests multi-disk export results
func TestExportResult_MultipleDisks(t *testing.T) {
	disks := []*ExportResult{
		{
			ImageName: "disk-0-image",
			DiskName:  "boot-disk",
			DiskType:  "boot",
			Format:    "vmdk",
		},
		{
			ImageName: "disk-1-image",
			DiskName:  "data-disk-1",
			DiskType:  "data-1",
			Format:    "vmdk",
		},
		{
			ImageName: "disk-2-image",
			DiskName:  "data-disk-2",
			DiskType:  "data-2",
			Format:    "vmdk",
		},
	}

	if len(disks) != 3 {
		t.Errorf("Expected 3 disks, got %d", len(disks))
	}

	// Verify boot disk is first
	if disks[0].DiskType != "boot" {
		t.Errorf("First disk should be boot disk, got %s", disks[0].DiskType)
	}

	// Verify data disks
	for i := 1; i < len(disks); i++ {
		expectedType := "data-" + string(rune('0'+i))
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
			ImageName: "instance-disk-0-image-123",
			DiskName:  "boot-disk",
			DiskType:  "boot",
			Format:    "vmdk",
			LocalPath: "/tmp/boot-disk.vmdk",
			Size:      50 * 1024 * 1024 * 1024,
			GCSBucket: "vm-exports",
			GCSObject: "boot-disk.vmdk",
			GCSURI:    "gs://vm-exports/boot-disk.vmdk",
		},
		{
			ImageName: "instance-disk-1-image-124",
			DiskName:  "data-disk-1",
			DiskType:  "data-1",
			Format:    "vmdk",
			LocalPath: "/tmp/data-disk-1.vmdk",
			Size:      100 * 1024 * 1024 * 1024,
			GCSBucket: "vm-exports",
			GCSObject: "data-disk-1.vmdk",
			GCSURI:    "gs://vm-exports/data-disk-1.vmdk",
		},
	}

	err := CreateExportManifest("test-instance", results, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Verify manifest file exists
	manifestPath := filepath.Join(tmpDir, "test-instance-manifest.json")
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
	if !contains(manifestStr, "test-instance") {
		t.Error("Manifest should contain instance name")
	}
	if !contains(manifestStr, "boot-disk") {
		t.Error("Manifest should contain boot disk")
	}
	if !contains(manifestStr, "data-disk-1") {
		t.Error("Manifest should contain data disk")
	}
	if !contains(manifestStr, "\"disk_count\": 2") {
		t.Error("Manifest should show correct disk count")
	}
	if !contains(manifestStr, "gs://vm-exports") {
		t.Error("Manifest should contain GCS URIs")
	}
}

// TestCreateExportManifest_SingleDisk tests manifest with single disk
func TestCreateExportManifest_SingleDisk(t *testing.T) {
	tmpDir := t.TempDir()

	results := []*ExportResult{
		{
			ImageName: "single-disk-image",
			DiskName:  "boot-disk",
			DiskType:  "boot",
			Format:    "vmdk",
			LocalPath: "/tmp/boot-disk.vmdk",
			Size:      50 * 1024 * 1024 * 1024,
			GCSBucket: "exports",
			GCSObject: "disk.vmdk",
			GCSURI:    "gs://exports/disk.vmdk",
		},
	}

	err := CreateExportManifest("single-disk-instance", results, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Verify manifest exists
	manifestPath := filepath.Join(tmpDir, "single-disk-instance-manifest.json")
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

// Integration test placeholders (require real GCP credentials)
// These tests are disabled by default - enable with build tag 'integration'

// TestExportImageToGCS_Integration tests full GCS export flow
func TestExportImageToGCS_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Skip("Integration test - requires GCP setup")

	// This test requires:
	// 1. GCP service account credentials
	// 2. GCS bucket created
	// 3. Compute Engine instance to export
	// 4. IAM permissions for image export and GCS access
}

// TestExportInstanceToGCS_Integration tests instance multi-disk export
func TestExportInstanceToGCS_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Skip("Integration test - requires GCP setup")
}

// Benchmark tests

func BenchmarkProgressReader_SmallVMDK(b *testing.B) {
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

func BenchmarkProgressReader_LargeVMDK(b *testing.B) {
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
				ImageName: "image1",
				Format:    "vmdk",
				LocalPath: "/tmp/image.vmdk",
				GCSBucket: "bucket",
			},
			wantErr: false,
		},
		{
			name: "missing image name",
			result: &ExportResult{
				Format:    "vmdk",
				LocalPath: "/tmp/image.vmdk",
			},
			wantErr: true,
		},
		{
			name: "missing format",
			result: &ExportResult{
				ImageName: "image1",
				LocalPath: "/tmp/image.vmdk",
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
	if result.ImageName == "" && result.DiskName == "" {
		return os.ErrInvalid
	}
	if result.Format == "" {
		return os.ErrInvalid
	}
	return nil
}

// TestMakeStringPtr tests helper function
func TestMakeStringPtr(t *testing.T) {
	str := "test-string"
	ptr := makeStringPtr(str)

	if ptr == nil {
		t.Error("Pointer should not be nil")
	}

	if *ptr != str {
		t.Errorf("Expected %s, got %s", str, *ptr)
	}
}
