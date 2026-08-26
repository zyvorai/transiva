// SPDX-License-Identifier: Apache-2.0

//go:build integration && integration
// +build integration,integration

package aws

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
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
	data := []byte("test data for progress reader")
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

	// Verify progress increased
	if len(reporter.updates) == 0 {
		t.Error("Expected progress updates, got none")
	}

	lastUpdate := 0
	for _, update := range reporter.updates {
		if update < lastUpdate {
			t.Errorf("Progress should increase monotonically, got %d after %d", update, lastUpdate)
		}
		lastUpdate = update
	}
}

// TestProgressReader_NoReporter tests progress reader without reporter
func TestProgressReader_NoReporter(t *testing.T) {
	data := []byte("test data")

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
		InstanceID: "i-1234567890abcdef0",
		ImageID:    "ami-1234567890abcdef0",
		SnapshotID: "snap-1234567890abcdef0",
		Format:     "vmdk",
		LocalPath:  "/tmp/export.vmdk",
		Size:       1024 * 1024 * 1024,
		S3Bucket:   "my-exports",
		S3Key:      "exports/instances/i-1234567890abcdef0.vmdk",
	}

	// Verify all fields are set
	if result.InstanceID == "" {
		t.Error("InstanceID should be set")
	}
	if result.Format != "vmdk" {
		t.Errorf("Expected format 'vmdk', got '%s'", result.Format)
	}
	if result.Size != 1024*1024*1024 {
		t.Errorf("Expected size 1GB, got %d", result.Size)
	}
}

// TestExportResult_MultipleFormats tests different export formats
func TestExportResult_MultipleFormats(t *testing.T) {
	formats := []string{"vmdk", "vhd", "raw"}

	for _, format := range formats {
		result := &ExportResult{
			Format: format,
		}

		if result.Format != format {
			t.Errorf("Expected format %s, got %s", format, result.Format)
		}
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

// Integration test placeholders (require real AWS credentials)
// These tests are disabled by default - enable with build tag 'integration'

// TestExportInstanceToS3_Integration tests full S3 export flow
func TestExportInstanceToS3_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This test requires:
	// 1. AWS credentials configured (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// 2. S3 bucket created
	// 3. EC2 instance to export
	// 4. IAM permissions for VM Import/Export

	t.Skip("Integration test - requires AWS setup")

	// Example implementation:
	/*
		ctx := context.Background()
		log := logger.New("debug")

		client, err := NewClient(ctx, "us-east-1", "", "", log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		tmpDir := t.TempDir()
		reporter := &MockProgressReporter{}

		result, err := client.ExportInstanceToS3(ctx, "i-test123", tmpDir, reporter)
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		// Verify result
		if result.LocalPath == "" {
			t.Error("LocalPath should be set")
		}

		if _, err := os.Stat(result.LocalPath); os.IsNotExist(err) {
			t.Error("Exported file should exist")
		}

		// Verify progress was reported
		if reporter.percentage != 100 {
			t.Errorf("Expected 100%% progress, got %d%%", reporter.percentage)
		}
	*/
}

// TestExportSnapshotToS3_Integration tests snapshot export
func TestExportSnapshotToS3_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Skip("Integration test - requires AWS setup")
}

// Benchmark tests for performance analysis

func BenchmarkProgressReader_SmallData(b *testing.B) {
	data := make([]byte, 1024) // 1KB
	reporter := &MockProgressReporter{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr := &progressReader{
			reader:   &mockReader{data: data},
			total:    int64(len(data)),
			reporter: reporter,
		}

		buf := make([]byte, 512)
		for {
			_, err := pr.Read(buf)
			if err != nil {
				break
			}
		}
	}
}

func BenchmarkProgressReader_LargeData(b *testing.B) {
	data := make([]byte, 10*1024*1024) // 10MB
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

// Test helper functions

func TestCreateTestExportResult(t *testing.T) {
	result := createTestExportResult()

	if result.InstanceID == "" {
		t.Error("Test export result should have InstanceID")
	}

	if result.Format == "" {
		t.Error("Test export result should have Format")
	}
}

func createTestExportResult() *ExportResult {
	return &ExportResult{
		InstanceID: "i-test123",
		ImageID:    "ami-test123",
		Format:     "vmdk",
		LocalPath:  "/tmp/test-export.vmdk",
		Size:       1024 * 1024,
		S3Bucket:   "test-bucket",
		S3Key:      "exports/test.vmdk",
	}
}

// TestExportResult_Validation tests result validation
func TestExportResult_Validation(t *testing.T) {
	tests := []struct {
		name    string
		result  *ExportResult
		wantErr bool
	}{
		{
			name: "valid result",
			result: &ExportResult{
				InstanceID: "i-123",
				Format:     "vmdk",
				LocalPath:  "/tmp/export.vmdk",
			},
			wantErr: false,
		},
		{
			name: "missing instance ID",
			result: &ExportResult{
				Format:    "vmdk",
				LocalPath: "/tmp/export.vmdk",
			},
			wantErr: true,
		},
		{
			name: "missing format",
			result: &ExportResult{
				InstanceID: "i-123",
				LocalPath:  "/tmp/export.vmdk",
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
	if result.InstanceID == "" && result.SnapshotID == "" {
		return os.ErrInvalid
	}
	if result.Format == "" {
		return os.ErrInvalid
	}
	return nil
}

// TestWaitForExportTask_Timeout tests timeout behavior
func TestWaitForExportTask_Timeout(t *testing.T) {
	// This would test the timeout logic in waitForExportTask
	// Requires mocking AWS SDK calls
	t.Skip("Requires AWS SDK mocking infrastructure")
}

// TestWaitForExportTask_Cancellation tests context cancellation
func TestWaitForExportTask_Cancellation(t *testing.T) {
	// This would test context cancellation in waitForExportTask
	// Requires mocking AWS SDK calls
	t.Skip("Requires AWS SDK mocking infrastructure")
}
