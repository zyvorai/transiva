// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/common"
)

func TestDetectHyper2KVMBinary(t *testing.T) {
	// This test will only pass if hyper2kvm is installed
	// Skip if not available
	_, err := exec.LookPath("hyper2kvm")
	if err != nil {
		t.Skip("hyper2kvm not available in PATH, skipping test")
	}

	binary, err := detectHyper2KVMBinary()
	if err != nil {
		t.Fatalf("Failed to detect hyper2kvm: %v", err)
	}

	if binary == "" {
		t.Error("Binary path is empty")
	}

	t.Logf("Detected hyper2kvm at: %s", binary)
}

func TestValidateBinary(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() string
		wantErr bool
	}{
		{
			name: "valid executable",
			setup: func() string {
				// Create temporary executable
				tmpDir, _ := os.MkdirTemp("", "converter-test-*")
				exePath := filepath.Join(tmpDir, "test-exec")
				_ = os.WriteFile(exePath, []byte("#!/bin/sh\necho test"), 0755)
				return exePath
			},
			wantErr: false,
		},
		{
			name: "non-executable file",
			setup: func() string {
				tmpDir, _ := os.MkdirTemp("", "converter-test-*")
				filePath := filepath.Join(tmpDir, "test-file")
				_ = os.WriteFile(filePath, []byte("test"), 0644)
				return filePath
			},
			wantErr: true,
		},
		{
			name: "directory instead of file",
			setup: func() string {
				tmpDir, _ := os.MkdirTemp("", "converter-test-*")
				return tmpDir
			},
			wantErr: true,
		},
		{
			name: "non-existent file",
			setup: func() string {
				return "/non/existent/path"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			err := validateBinary(path)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateBinary() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Cleanup
			if _, err := os.Stat(path); err == nil {
				_ = os.RemoveAll(filepath.Dir(path))
			}
		})
	}
}

func TestNewHyper2KVMConverter(t *testing.T) {
	log := logger.New("info")

	// Test with explicit binary path (create mock)
	tmpDir, err := os.MkdirTemp("", "converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mockBinary := filepath.Join(tmpDir, "hyper2kvm")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\necho 'hyper2kvm v1.0.0'"), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	converter, err := NewHyper2KVMConverter(mockBinary, log)
	if err != nil {
		t.Fatalf("NewHyper2KVMConverter() failed: %v", err)
	}

	if converter.binaryPath != mockBinary {
		t.Errorf("Binary path = %s, want %s", converter.binaryPath, mockBinary)
	}

	t.Logf("✅ Converter initialized successfully")
	t.Logf("   Binary: %s", converter.binaryPath)
}

func TestNewHyper2KVMConverter_AutoDetect(t *testing.T) {
	log := logger.New("info")

	// Try auto-detection (will only work if hyper2kvm is installed)
	converter, err := NewHyper2KVMConverter("", log)
	if err != nil {
		// Expected if hyper2kvm is not installed
		t.Skipf("Auto-detection failed (hyper2kvm not installed): %v", err)
	}

	if converter.binaryPath == "" {
		t.Error("Binary path is empty after auto-detection")
	}

	t.Logf("✅ Auto-detected hyper2kvm at: %s", converter.binaryPath)
}

func TestConvertOptions(t *testing.T) {
	opts := common.ConvertOptions{
		StreamOutput: true,
		Verbose:      true,
		DryRun:       false,
	}

	if !opts.StreamOutput {
		t.Error("StreamOutput should be true")
	}

	if !opts.Verbose {
		t.Error("Verbose should be true")
	}

	if opts.DryRun {
		t.Error("DryRun should be false")
	}

	t.Log("✅ ConvertOptions struct test passed")
}

func TestConversionResult(t *testing.T) {
	result := &common.ConversionResult{
		Success:        true,
		ConvertedFiles: []string{"/work/disk-0.qcow2", "/work/disk-1.qcow2"},
		ReportPath:     "/work/report.json",
		Duration:       15 * time.Minute,
		Error:          "",
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if len(result.ConvertedFiles) != 2 {
		t.Errorf("ConvertedFiles count = %d, want 2", len(result.ConvertedFiles))
	}

	if result.Duration != 15*time.Minute {
		t.Errorf("Duration = %v, want 15m", result.Duration)
	}

	t.Log("✅ ConversionResult struct test passed")
	t.Logf("   Success: %v", result.Success)
	t.Logf("   Converted files: %d", len(result.ConvertedFiles))
	t.Logf("   Duration: %v", result.Duration)
}

func TestParseConversionResults(t *testing.T) {
	log := logger.New("info")

	// Create temporary directory with mock report
	tmpDir, err := os.MkdirTemp("", "converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create mock report.json
	reportPath := filepath.Join(tmpDir, "report.json")
	reportContent := `{
  "success": true,
  "pipeline": {
    "stages": {
      "inspect": {"success": true},
      "fix": {"success": true},
      "convert": {"success": true},
      "validate": {"success": true}
    }
  },
  "artifacts": {
    "converted_disks": [
      {"path": "/work/disk-0.qcow2"},
      {"path": "/work/disk-1.qcow2"}
    ]
  }
}`

	if err := os.WriteFile(reportPath, []byte(reportContent), 0644); err != nil {
		t.Fatalf("Failed to write report: %v", err)
	}

	// Create mock converter
	mockBinary := filepath.Join(tmpDir, "hyper2kvm")
	_ = os.WriteFile(mockBinary, []byte("#!/bin/sh\necho 'mock'"), 0755)

	converter, err := NewHyper2KVMConverter(mockBinary, log)
	if err != nil {
		t.Fatalf("Failed to create converter: %v", err)
	}

	// Parse results
	result, err := converter.parseConversionResults(tmpDir)
	if err != nil {
		t.Fatalf("parseConversionResults() failed: %v", err)
	}

	if !result.Success {
		t.Error("Result should be successful")
	}

	if len(result.ConvertedFiles) != 2 {
		t.Errorf("ConvertedFiles count = %d, want 2", len(result.ConvertedFiles))
	}

	if result.ReportPath != reportPath {
		t.Errorf("ReportPath = %s, want %s", result.ReportPath, reportPath)
	}

	t.Log("✅ Parse conversion results test passed")
	t.Logf("   Success: %v", result.Success)
	t.Logf("   Converted files: %d", len(result.ConvertedFiles))
	t.Logf("   Report path: %s", result.ReportPath)
}

func TestGetVersion(t *testing.T) {
	log := logger.New("info")

	// Create mock binary that returns version
	tmpDir, err := os.MkdirTemp("", "converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mockBinary := filepath.Join(tmpDir, "hyper2kvm")
	mockScript := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "hyper2kvm v1.0.0"
fi
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	converter, err := NewHyper2KVMConverter(mockBinary, log)
	if err != nil {
		t.Fatalf("Failed to create converter: %v", err)
	}

	version, err := converter.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion() failed: %v", err)
	}

	if version != "hyper2kvm v1.0.0" {
		t.Errorf("Version = %q, want 'hyper2kvm v1.0.0'", version)
	}

	t.Log("✅ GetVersion test passed")
	t.Logf("   Version: %s", version)
}

func TestConvert_ContextTimeout(t *testing.T) {
	log := logger.New("info")

	// Create mock binary that sleeps
	tmpDir, err := os.MkdirTemp("", "converter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mockBinary := filepath.Join(tmpDir, "hyper2kvm")
	mockScript := `#!/bin/sh
sleep 10
`
	if err := os.WriteFile(mockBinary, []byte(mockScript), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	converter, err := NewHyper2KVMConverter(mockBinary, log)
	if err != nil {
		t.Fatalf("Failed to create converter: %v", err)
	}

	// Create mock manifest
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	_ = os.WriteFile(manifestPath, []byte("{}"), 0644)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opts := common.ConvertOptions{
		StreamOutput: false,
		Verbose:      false,
		DryRun:       false,
	}

	_, err = converter.Convert(ctx, manifestPath, opts)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if err != nil && err.Error() != "conversion timeout: context deadline exceeded" {
		t.Logf("Got expected timeout error: %v", err)
	}

	t.Log("✅ Context timeout test passed")
}
