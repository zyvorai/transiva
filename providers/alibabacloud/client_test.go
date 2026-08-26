// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"errors"
	"testing"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "test",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substring at start",
			s:        "teststring",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "stringtest",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "atestb",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substring not found",
			s:        "hello world",
			substr:   "test",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "test",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "test",
			expected: false,
		},
		{
			name:     "both empty",
			s:        "",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "test",
			substr:   "testing",
			expected: false,
		},
		{
			name:     "error message with InvalidImageId.NotFound",
			s:        "Error: InvalidImageId.NotFound - The specified image does not exist",
			substr:   "InvalidImageId.NotFound",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "substring at start",
			s:        "teststring",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "atestb",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "stringtest",
			substr:   "test",
			expected: true,
		},
		{
			name:     "substring not found",
			s:        "hello world",
			substr:   "test",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "test",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "ab",
			substr:   "abc",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("findSubstring(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "InvalidImageId.NotFound error",
			err:      errors.New("InvalidImageId.NotFound: The specified image does not exist"),
			expected: true,
		},
		{
			name:     "InvalidInstanceId.NotFound error",
			err:      errors.New("InvalidInstanceId.NotFound: The specified instance does not exist"),
			expected: true,
		},
		{
			name:     "InvalidSnapshotId.NotFound error",
			err:      errors.New("Error: InvalidSnapshotId.NotFound"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "partial match not found",
			err:      errors.New("InvalidImageId is invalid"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("isNotFoundError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
