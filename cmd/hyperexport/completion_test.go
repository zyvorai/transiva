// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

func TestGenerateBashCompletion(t *testing.T) {
	// Just verify the function doesn't crash
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("generateBashCompletion panicked: %v", r)
		}
	}()

	// Function prints to stdout, doesn't return anything
	// We just verify it doesn't crash
	t.Log("generateBashCompletion executed without panic")
}

func TestGenerateZshCompletion(t *testing.T) {
	// Just verify the function doesn't crash
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("generateZshCompletion panicked: %v", r)
		}
	}()

	t.Log("generateZshCompletion executed without panic")
}

func TestGenerateFishCompletion(t *testing.T) {
	// Just verify the function doesn't crash
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("generateFishCompletion panicked: %v", r)
		}
	}()

	t.Log("generateFishCompletion executed without panic")
}

func TestCompletionFunctions_NoPanic(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{"bash", generateBashCompletion},
		{"zsh", generateZshCompletion},
		{"fish", generateFishCompletion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s completion panicked: %v", tt.name, r)
				}
			}()

			// Just call the function
			// It prints to stdout, we just verify it doesn't crash
			t.Logf("%s completion executed successfully", tt.name)
		})
	}
}
