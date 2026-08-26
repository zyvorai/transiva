// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal filename",
			input:    "my-vm",
			expected: "my-vm",
		},
		{
			name:     "filename with forward slash",
			input:    "folder/vm",
			expected: "folder_vm",
		},
		{
			name:     "filename with backslash",
			input:    "folder\\vm",
			expected: "folder_vm",
		},
		{
			name:     "filename with parent directory",
			input:    "../vm",
			expected: ".._vm",
		},
		{
			name:     "filename with colon",
			input:    "vm:name",
			expected: "vm_name",
		},
		{
			name:     "filename with asterisk",
			input:    "vm*test",
			expected: "vm_test",
		},
		{
			name:     "filename with question mark",
			input:    "vm?test",
			expected: "vm_test",
		},
		{
			name:     "filename with quotes",
			input:    "\"vm\"",
			expected: "_vm_",
		},
		{
			name:     "filename with angle brackets",
			input:    "<vm>",
			expected: "_vm_",
		},
		{
			name:     "filename with pipe",
			input:    "vm|test",
			expected: "vm_test",
		},
		{
			name:     "complex path traversal attempt",
			input:    "../../etc/passwd",
			expected: ".._.._etc_passwd",
		},
		{
			name:     "multiple invalid characters",
			input:    "vm:name/test\\file*.txt",
			expected: "vm_name_test_file_.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{
			name:     "string shorter than max",
			input:    "short",
			max:      10,
			expected: "short",
		},
		{
			name:     "string equal to max",
			input:    "exactly10c",
			max:      10,
			expected: "exactly10c",
		},
		{
			name:     "string longer than max",
			input:    "this is a very long string",
			max:      10,
			expected: "this is...",
		},
		{
			name:     "empty string",
			input:    "",
			max:      10,
			expected: "",
		},
		{
			name:     "max of 3",
			input:    "test",
			max:      3,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
			}
		})
	}
}
