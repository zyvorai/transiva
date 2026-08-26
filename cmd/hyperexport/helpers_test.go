// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"time"
)

func TestCountWarnings(t *testing.T) {
	tests := []struct {
		name     string
		report   *ValidationReport
		expected int
	}{
		{
			name: "no warnings",
			report: &ValidationReport{
				Checks: []ValidationResult{
					{Name: "check1", Passed: true, Warning: false},
					{Name: "check2", Passed: true, Warning: false},
				},
			},
			expected: 0,
		},
		{
			name: "some warnings",
			report: &ValidationReport{
				Checks: []ValidationResult{
					{Name: "check1", Passed: true, Warning: true},
					{Name: "check2", Passed: false, Warning: false},
					{Name: "check3", Passed: true, Warning: true},
				},
			},
			expected: 2,
		},
		{
			name: "all warnings",
			report: &ValidationReport{
				Checks: []ValidationResult{
					{Name: "check1", Passed: true, Warning: true},
					{Name: "check2", Passed: true, Warning: true},
				},
			},
			expected: 2,
		},
		{
			name: "empty report",
			report: &ValidationReport{
				Checks: []ValidationResult{},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countWarnings(tt.report)
			if result != tt.expected {
				t.Errorf("countWarnings() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestCountFailed(t *testing.T) {
	tests := []struct {
		name     string
		report   *ValidationReport
		expected int
	}{
		{
			name: "no failures",
			report: &ValidationReport{
				Checks: []ValidationResult{
					{Name: "check1", Passed: true},
					{Name: "check2", Passed: true},
				},
			},
			expected: 0,
		},
		{
			name: "some failures",
			report: &ValidationReport{
				Checks: []ValidationResult{
					{Name: "check1", Passed: true},
					{Name: "check2", Passed: false},
					{Name: "check3", Passed: false},
				},
			},
			expected: 2,
		},
		{
			name: "all failures",
			report: &ValidationReport{
				Checks: []ValidationResult{
					{Name: "check1", Passed: false},
					{Name: "check2", Passed: false},
				},
			},
			expected: 2,
		},
		{
			name: "empty report",
			report: &ValidationReport{
				Checks:    []ValidationResult{},
				Timestamp: time.Now(),
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countFailed(tt.report)
			if result != tt.expected {
				t.Errorf("countFailed() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name          string
		slice         []string
		val           string
		expectedIndex int
		expectedFound bool
	}{
		{
			name:          "value found at start",
			slice:         []string{"a", "b", "c"},
			val:           "a",
			expectedIndex: 0,
			expectedFound: true,
		},
		{
			name:          "value found in middle",
			slice:         []string{"a", "b", "c"},
			val:           "b",
			expectedIndex: 1,
			expectedFound: true,
		},
		{
			name:          "value found at end",
			slice:         []string{"a", "b", "c"},
			val:           "c",
			expectedIndex: 2,
			expectedFound: true,
		},
		{
			name:          "value not found",
			slice:         []string{"a", "b", "c"},
			val:           "d",
			expectedIndex: -1,
			expectedFound: false,
		},
		{
			name:          "empty slice",
			slice:         []string{},
			val:           "a",
			expectedIndex: -1,
			expectedFound: false,
		},
		{
			name:          "duplicate values",
			slice:         []string{"a", "b", "a", "c"},
			val:           "a",
			expectedIndex: 0, // returns first occurrence
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, found := contains(tt.slice, tt.val)
			if index != tt.expectedIndex {
				t.Errorf("contains() index = %d, want %d", index, tt.expectedIndex)
			}
			if found != tt.expectedFound {
				t.Errorf("contains() found = %v, want %v", found, tt.expectedFound)
			}
		})
	}
}

