// SPDX-License-Identifier: Apache-2.0

package capabilities

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewDetector(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	if detector == nil {
		t.Fatal("NewDetector returned nil")
	}

	if detector.capabilities == nil {
		t.Error("capabilities map not initialized")
	}

	if detector.logger == nil {
		t.Error("logger not set")
	}
}

func TestDetectWithMockBinaries(t *testing.T) {
	// Create temporary directory for mock binaries
	tmpDir := t.TempDir()

	// Create mock binaries
	mockCTL := filepath.Join(tmpDir, "hyperctl")
	mockGovc := filepath.Join(tmpDir, "govc")
	mockOvftool := filepath.Join(tmpDir, "ovftool")

	// Create executable mock files
	for _, path := range []string{mockCTL, mockGovc, mockOvftool} {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("failed to create mock binary %s: %v", path, err)
		}
		f.Close()

		if err := os.Chmod(path, 0755); err != nil {
			t.Fatalf("failed to chmod mock binary %s: %v", path, err)
		}
	}

	// Modify PATH to include our mock directory
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	// Create detector and run detection
	log := logger.New("info")
	detector := NewDetector(log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Verify all methods were detected
	caps := detector.GetCapabilities()

	// Web should always be available
	if cap, ok := caps[ExportMethodWeb]; !ok || !cap.Available {
		t.Error("Web export method should always be available")
	}

	// Check that detection ran for all methods
	if len(caps) < 4 {
		t.Errorf("Expected 4 export methods, got %d", len(caps))
	}
}

func TestGetBestMethod(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// Initially, web should be the best method (only available)
	detector.capabilities[ExportMethodWeb] = &ExportCapability{
		Method:    ExportMethodWeb,
		Available: true,
		Priority:  4,
	}

	best := detector.GetBestMethod()
	if best != ExportMethodWeb {
		t.Errorf("Expected web, got %s", best)
	}

	// Add CTL with higher priority
	detector.capabilities[ExportMethodCTL] = &ExportCapability{
		Method:    ExportMethodCTL,
		Available: true,
		Priority:  1,
	}

	best = detector.GetBestMethod()
	if best != ExportMethodCTL {
		t.Errorf("Expected ctl, got %s", best)
	}

	// Add govc
	detector.capabilities[ExportMethodGovc] = &ExportCapability{
		Method:    ExportMethodGovc,
		Available: true,
		Priority:  2,
	}

	best = detector.GetBestMethod()
	if best != ExportMethodCTL {
		t.Errorf("Expected ctl (highest priority), got %s", best)
	}
}

func TestGetDefaultMethod(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	detector.capabilities[ExportMethodCTL] = &ExportCapability{
		Method:    ExportMethodCTL,
		Available: true,
		Priority:  1,
	}

	defaultMethod := detector.GetDefaultMethod()
	bestMethod := detector.GetBestMethod()

	if defaultMethod != bestMethod {
		t.Errorf("GetDefaultMethod and GetBestMethod should return same value, got %s and %s", defaultMethod, bestMethod)
	}
}

func TestIsAvailable(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// Initially nothing is available
	if detector.IsAvailable(ExportMethodCTL) {
		t.Error("CTL should not be available before detection")
	}

	// Mark CTL as available
	detector.capabilities[ExportMethodCTL] = &ExportCapability{
		Method:    ExportMethodCTL,
		Available: true,
		Priority:  1,
	}

	if !detector.IsAvailable(ExportMethodCTL) {
		t.Error("CTL should be available after adding to capabilities")
	}

	// Check unavailable method
	if detector.IsAvailable(ExportMethodGovc) {
		t.Error("Govc should not be available")
	}
}

func TestGetCapabilities(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// Add some capabilities
	detector.capabilities[ExportMethodCTL] = &ExportCapability{
		Method:    ExportMethodCTL,
		Available: true,
		Priority:  1,
		Path:      "/usr/bin/hyperctl",
	}

	detector.capabilities[ExportMethodWeb] = &ExportCapability{
		Method:    ExportMethodWeb,
		Available: true,
		Priority:  4,
		Path:      "internal",
	}

	caps := detector.GetCapabilities()

	if len(caps) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(caps))
	}

	if cap, ok := caps[ExportMethodCTL]; !ok {
		t.Error("CTL capability not found")
	} else {
		if cap.Priority != 1 {
			t.Errorf("Expected priority 1, got %d", cap.Priority)
		}
	}
}

func TestDetectCTL(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	cap := detector.detectCTL()

	if cap == nil {
		t.Fatal("detectCTL returned nil")
	}

	if cap.Method != ExportMethodCTL {
		t.Errorf("Expected method ctl, got %s", cap.Method)
	}

	if cap.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", cap.Priority)
	}

	// Check if hyperctl is actually available in PATH
	_, err := exec.LookPath("hyperctl")
	if err == nil && !cap.Available {
		t.Error("hyperctl found in PATH but marked as unavailable")
	}

	if err != nil && cap.Available {
		t.Error("hyperctl not found in PATH but marked as available")
	}
}

func TestDetectGovc(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	cap := detector.detectGovc()

	if cap == nil {
		t.Fatal("detectGovc returned nil")
	}

	if cap.Method != ExportMethodGovc {
		t.Errorf("Expected method govc, got %s", cap.Method)
	}

	if cap.Priority != 2 {
		t.Errorf("Expected priority 2, got %d", cap.Priority)
	}
}

func TestDetectOvftool(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	cap := detector.detectOvftool()

	if cap == nil {
		t.Fatal("detectOvftool returned nil")
	}

	if cap.Method != ExportMethodOvftool {
		t.Errorf("Expected method ovftool, got %s", cap.Method)
	}

	if cap.Priority != 3 {
		t.Errorf("Expected priority 3, got %d", cap.Priority)
	}
}

func TestDetectWeb(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	cap := detector.detectWeb()

	if cap == nil {
		t.Fatal("detectWeb returned nil")
	}

	if cap.Method != ExportMethodWeb {
		t.Errorf("Expected method web, got %s", cap.Method)
	}

	if !cap.Available {
		t.Error("Web method should always be available")
	}

	if cap.Priority != 4 {
		t.Errorf("Expected priority 4, got %d", cap.Priority)
	}

	if cap.Path != "internal" {
		t.Errorf("Expected path 'internal', got %s", cap.Path)
	}

	if cap.Version != "built-in" {
		t.Errorf("Expected version 'built-in', got %s", cap.Version)
	}
}

func TestConcurrentDetection(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run detection
	err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Verify we got results for all methods
	caps := detector.GetCapabilities()

	expectedMethods := []ExportMethod{
		ExportMethodCTL,
		ExportMethodGovc,
		ExportMethodOvftool,
		ExportMethodWeb,
	}

	for _, method := range expectedMethods {
		if _, ok := caps[method]; !ok {
			t.Errorf("Missing capability for method: %s", method)
		}
	}
}

func TestDetectContextCancellation(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Detection should still complete but might have partial results
	err := detector.Detect(ctx)

	// Should not error on cancellation, just log warnings
	if err != nil && err != context.Canceled {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestMethodPriorities(t *testing.T) {
	tests := []struct {
		method   ExportMethod
		priority int
	}{
		{ExportMethodCTL, 1},
		{ExportMethodGovc, 2},
		{ExportMethodOvftool, 3},
		{ExportMethodWeb, 4},
	}

	log := logger.New("info")
	detector := NewDetector(log)

	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			var cap *ExportCapability

			switch tt.method {
			case ExportMethodCTL:
				cap = detector.detectCTL()
			case ExportMethodGovc:
				cap = detector.detectGovc()
			case ExportMethodOvftool:
				cap = detector.detectOvftool()
			case ExportMethodWeb:
				cap = detector.detectWeb()
			}

			if cap.Priority != tt.priority {
				t.Errorf("Expected priority %d for %s, got %d", tt.priority, tt.method, cap.Priority)
			}
		})
	}
}

func TestGetBestMethodFallbackToWeb(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// No capabilities set - should fall back to web
	best := detector.GetBestMethod()
	if best != ExportMethodWeb {
		t.Errorf("Expected fallback to web when no capabilities available, got %s", best)
	}
}

func TestGetBestMethodWithUnavailableMethods(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// Add methods but mark them as unavailable
	detector.capabilities[ExportMethodCTL] = &ExportCapability{
		Method:    ExportMethodCTL,
		Available: false,
		Priority:  1,
	}

	detector.capabilities[ExportMethodGovc] = &ExportCapability{
		Method:    ExportMethodGovc,
		Available: false,
		Priority:  2,
	}

	detector.capabilities[ExportMethodOvftool] = &ExportCapability{
		Method:    ExportMethodOvftool,
		Available: false,
		Priority:  3,
	}

	// Should still fall back to web
	best := detector.GetBestMethod()
	if best != ExportMethodWeb {
		t.Errorf("Expected fallback to web when all methods unavailable, got %s", best)
	}
}

func TestGetBestMethodSkipsUnavailable(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// CTL unavailable, govc available
	detector.capabilities[ExportMethodCTL] = &ExportCapability{
		Method:    ExportMethodCTL,
		Available: false,
		Priority:  1,
	}

	detector.capabilities[ExportMethodGovc] = &ExportCapability{
		Method:    ExportMethodGovc,
		Available: true,
		Priority:  2,
	}

	best := detector.GetBestMethod()
	if best != ExportMethodGovc {
		t.Errorf("Expected govc (skipping unavailable CTL), got %s", best)
	}
}

func TestDetectCTLNotFound(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// Ensure hyperctl is not in PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", oldPath)

	cap := detector.detectCTL()

	if cap.Available {
		t.Error("Expected CTL to be unavailable when binary not found")
	}

	if cap.Method != ExportMethodCTL {
		t.Errorf("Expected method to be CTL, got %s", cap.Method)
	}

	if cap.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", cap.Priority)
	}

	if cap.Path != "" {
		t.Errorf("Expected empty path when binary not found, got %s", cap.Path)
	}
}

func TestDetectCTLVersionCheckFails(t *testing.T) {
	log := logger.New("info")
	detector := NewDetector(log)

	// Create a mock binary that will fail version check
	tmpDir := t.TempDir()
	mockCTL := filepath.Join(tmpDir, "hyperctl")

	// Create a script that exits with error
	script := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(mockCTL, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	// Modify PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	cap := detector.detectCTL()

	// Should still be available even if version check fails
	if !cap.Available {
		t.Error("Expected CTL to be available even when version check fails")
	}

	if cap.Version != "unknown" {
		t.Errorf("Expected version to be 'unknown', got %s", cap.Version)
	}

	if cap.Path != mockCTL {
		t.Errorf("Expected path to be %s, got %s", mockCTL, cap.Path)
	}
}
