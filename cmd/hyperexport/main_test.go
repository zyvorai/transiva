// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeForPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal-name", "normal-name"},
		{"name with spaces", "name with spaces"},
		{"name/with/slashes", "name_with_slashes"},
		{"name:with:colons", "name_with_colons"},
		{"name<with>brackets", "name_with_brackets"},
		{"name|with|pipes", "name_with_pipes"},
		{"name?with*wildcards", "name_with_wildcards"},
		{"name\"with\"quotes", "name_with_quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeForPath(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeForPath(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
		{1099511627776, "1.0 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %q; want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestGetOutputDir(t *testing.T) {
	// Save original flag value
	originalOutputDir := *outputDir
	defer func() {
		*outputDir = originalOutputDir
	}()

	tests := []struct {
		name      string
		flagValue string
		vmName    string
		expected  string
	}{
		{
			name:      "default output dir",
			flagValue: "",
			vmName:    "test-vm",
			expected:  "./export-test-vm",
		},
		{
			name:      "custom output dir",
			flagValue: "/custom/path",
			vmName:    "test-vm",
			expected:  "/custom/path",
		},
		{
			name:      "vm with special chars",
			flagValue: "",
			vmName:    "vm/with/slashes",
			expected:  "./export-vm_with_slashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*outputDir = tt.flagValue
			result := getOutputDir(tt.vmName)
			if result != tt.expected {
				t.Errorf("getOutputDir(%q) with flag=%q = %q; want %q",
					tt.vmName, tt.flagValue, result, tt.expected)
			}
		})
	}
}

func TestFilterByFolder(t *testing.T) {
	vms := []string{
		"/datacenter/vm/production/web-01",
		"/datacenter/vm/production/web-02",
		"/datacenter/vm/test/db-01",
		"/datacenter/vm/dev/app-01",
	}

	tests := []struct {
		name     string
		folder   string
		expected int
	}{
		{"production folder", "/production/", 2},
		{"test folder", "/test/", 1},
		{"dev folder", "/dev/", 1},
		{"nonexistent folder", "/staging/", 0},
		{"datacenter root", "/datacenter/", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterByFolder(vms, tt.folder)
			if len(result) != tt.expected {
				t.Errorf("filterByFolder with folder %q = %d VMs; want %d",
					tt.folder, len(result), tt.expected)
			}
		})
	}
}

func TestCalculateSHA256(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	hash, err := calculateSHA256(testFile)
	if err != nil {
		t.Fatalf("calculateSHA256 failed: %v", err)
	}

	// Expected SHA256 of "Hello, World!"
	expected := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	if hash != expected {
		t.Errorf("calculateSHA256 = %q; want %q", hash, expected)
	}

	// Test with nonexistent file
	_, err = calculateSHA256(filepath.Join(tmpDir, "nonexistent.txt"))
	if err == nil {
		t.Error("calculateSHA256 with nonexistent file should return error")
	}
}

func TestSaveChecksums(t *testing.T) {
	tmpDir := t.TempDir()
	checksumFile := filepath.Join(tmpDir, "checksums.txt")

	checksums := map[string]string{
		"file1.ovf":  "abc123",
		"file2.vmdk": "def456",
		"file3.mf":   "ghi789",
	}

	if err := saveChecksums(checksumFile, checksums); err != nil {
		t.Fatalf("saveChecksums failed: %v", err)
	}

	// Read and verify
	content, err := os.ReadFile(checksumFile)
	if err != nil {
		t.Fatalf("Failed to read checksum file: %v", err)
	}

	contentStr := string(content)
	for filename, hash := range checksums {
		expectedLine := hash + "  " + filename
		if !strings.Contains(contentStr, expectedLine) {
			t.Errorf("Checksum file missing entry: %q", expectedLine)
		}
	}
}

func TestPowerStateIcon(t *testing.T) {
	tests := []struct {
		state    string
		contains string
	}{
		{"poweredOn", "●"},
		{"poweredOff", "●"},
		{"suspended", "●"},
		{"unknown", "●"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			result := getPowerStateIcon(tt.state)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("getPowerStateIcon(%q) = %q; should contain %q",
					tt.state, result, tt.contains)
			}
		})
	}
}
