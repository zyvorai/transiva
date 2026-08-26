// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"testing"
)

func TestIsOSSNotFoundError(t *testing.T) {
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
			name:     "NoSuchKey error",
			err:      errors.New("NoSuchKey: The specified key does not exist"),
			expected: true,
		},
		{
			name:     "404 error",
			err:      errors.New("HTTP 404 Not Found"),
			expected: true,
		},
		{
			name:     "Not Found error",
			err:      errors.New("Error: Not Found"),
			expected: true,
		},
		{
			name:     "404 in middle of message",
			err:      errors.New("Request failed with status code 404"),
			expected: true,
		},
		{
			name:     "NoSuchKey at end",
			err:      errors.New("OSS service error: NoSuchKey"),
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "access denied error",
			err:      errors.New("AccessDenied"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOSSNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("isOSSNotFoundError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
