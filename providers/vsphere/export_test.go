// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"testing"
)

func TestDefaultExportOptions(t *testing.T) {
	opts := DefaultExportOptions()

	if opts.ParallelDownloads != 3 {
		t.Errorf("Expected default ParallelDownloads 3, got %d", opts.ParallelDownloads)
	}

	if !opts.RemoveCDROM {
		t.Error("Expected RemoveCDROM to be true by default")
	}

	if opts.ShowIndividualProgress {
		t.Error("Expected ShowIndividualProgress to be false by default")
	}

	if !opts.ShowOverallProgress {
		t.Error("Expected ShowOverallProgress to be true by default")
	}

	if opts.Format != "ovf" {
		t.Errorf("Expected default format 'ovf', got '%s'", opts.Format)
	}
}

func TestExportOptionsValidation(t *testing.T) {
	tests := []struct {
		name     string
		parallel int
		wantErr  bool
	}{
		{"valid_parallel_4", 4, false},
		{"valid_parallel_8", 8, false},
		{"valid_parallel_16", 16, false},
		{"invalid_parallel_0", 0, true},
		{"invalid_parallel_negative", -1, true},
		{"invalid_parallel_too_high", 32, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultExportOptions()
			opts.ParallelDownloads = tt.parallel
			opts.OutputPath = "/tmp/test-output" // Set required field

			err := opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExportOptionsOutputPath(t *testing.T) {
	opts := DefaultExportOptions()
	opts.OutputPath = "/tmp/test-export"

	if opts.OutputPath != "/tmp/test-export" {
		t.Errorf("Expected OutputPath '/tmp/test-export', got '%s'", opts.OutputPath)
	}
}

func TestExportOptionsFlags(t *testing.T) {
	opts := DefaultExportOptions()

	// Test RemoveCDROM flag
	opts.RemoveCDROM = true
	if !opts.RemoveCDROM {
		t.Error("Failed to set RemoveCDROM to true")
	}

	// Test ShowIndividualProgress flag
	opts.ShowIndividualProgress = true
	if !opts.ShowIndividualProgress {
		t.Error("Failed to set ShowIndividualProgress to true")
	}
}

// TestSanitizeVMName tests the VM name sanitization function for security
func TestSanitizeVMName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid_simple_name",
			input:    "my-vm",
			expected: "my-vm",
		},
		{
			name:     "path_traversal_dot_dot_slash",
			input:    "../../../etc/passwd",
			expected: "etc-passwd", // Leading dashes trimmed
		},
		{
			name:     "path_traversal_backslash",
			input:    "..\\..\\windows\\system32",
			expected: "windows-system32", // Leading dashes trimmed
		},
		{
			name:     "forward_slash_separator",
			input:    "folder/vm/name",
			expected: "folder-vm-name",
		},
		{
			name:     "backslash_separator",
			input:    "folder\\vm\\name",
			expected: "folder-vm-name",
		},
		{
			name:     "null_byte_injection",
			input:    "vm\x00malicious",
			expected: "vmmalicious",
		},
		{
			name:     "special_characters",
			input:    "vm:name*with?special<chars>|test",
			expected: "vm-name-with-special-chars--test",
		},
		{
			name:     "leading_dots_and_dashes",
			input:    "...---vm-name---...",
			expected: "vm-name",
		},
		{
			name:     "empty_after_sanitization",
			input:    "...---...",
			expected: "unnamed-vm",
		},
		{
			name:     "spaces_allowed_in_vm_names",
			input:    "my vm name",
			expected: "my vm name", // Spaces are valid in filenames
		},
		{
			name:     "quotes_removed",
			input:    "my\"vm\"name",
			expected: "my-vm-name",
		},
		{
			name:     "mixed_dangerous_chars",
			input:    "../folder/vm:name*test",
			expected: "folder-vm-name-test", // Leading dashes trimmed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeVMName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeVMName(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			// Verify result doesn't contain dangerous characters
			if result != "" && result != "unnamed-vm" {
				validateSafeString(t, result)
			}
		})
	}
}

// TestSanitizeVMNameLength tests that very long names are truncated
func TestSanitizeVMNameLength(t *testing.T) {
	// Create a 300-character name
	longName := ""
	for i := 0; i < 300; i++ {
		longName += "a"
	}

	result := sanitizeVMName(longName)
	if len(result) > 255 {
		t.Errorf("sanitizeVMName() produced name longer than 255 chars: %d", len(result))
	}
	if len(result) != 255 {
		t.Errorf("sanitizeVMName() should truncate to exactly 255 chars, got %d", len(result))
	}
}

// validateSafeString checks that a sanitized string doesn't contain dangerous characters
func validateSafeString(t *testing.T, s string) {
	t.Helper()

	dangerousChars := []string{"/", "\\", "..", "\x00", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerousChars {
		if containsChar(s, char) {
			t.Errorf("Sanitized string contains dangerous character %q: %q", char, s)
		}
	}
}

// containsChar checks if string contains a substring (helper for testing)
func containsChar(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
