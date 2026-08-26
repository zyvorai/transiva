// SPDX-License-Identifier: Apache-2.0

package oci

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
			name:     "error message with NotAuthorizedOrNotFound",
			s:        "Error: NotAuthorizedOrNotFound - The resource does not exist",
			substr:   "NotAuthorizedOrNotFound",
			expected: true,
		},
		{
			name:     "error message with 404",
			s:        "HTTP 404 Not Found",
			substr:   "404",
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
			name:     "NotAuthorizedOrNotFound error",
			err:      errors.New("NotAuthorizedOrNotFound: The resource does not exist"),
			expected: true,
		},
		{
			name:     "404 error",
			err:      errors.New("HTTP 404 Not Found"),
			expected: true,
		},
		{
			name:     "404 in middle of message",
			err:      errors.New("Error 404: Resource not found"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "partial match not found",
			err:      errors.New("NotAuthorized but not NotFound"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFoundError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
