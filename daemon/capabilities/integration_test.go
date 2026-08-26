// SPDX-License-Identifier: Apache-2.0

package capabilities

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// TestIntegration_FullCapabilityDetection tests the complete capability detection flow
func TestIntegration_FullCapabilityDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.New("info")
	detector := NewDetector(log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run full detection
	err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Detection failed: %v", err)
	}

	// Verify we have all 4 methods detected
	caps := detector.GetCapabilities()
	if len(caps) != 4 {
		t.Errorf("Expected 4 capabilities, got %d", len(caps))
	}

	// Web should always be available
	if !detector.IsAvailable(ExportMethodWeb) {
		t.Error("Web method should always be available")
	}

	// Get best method
	bestMethod := detector.GetBestMethod()
	if bestMethod == "" {
		t.Error("GetBestMethod returned empty string")
	}

	// Verify best method is actually available
	if !detector.IsAvailable(bestMethod) {
		t.Errorf("Best method %s is not marked as available", bestMethod)
	}

	// Verify priorities are correct
	expectedPriorities := map[ExportMethod]int{
		ExportMethodCTL:     1,
		ExportMethodGovc:    2,
		ExportMethodOvftool: 3,
		ExportMethodWeb:     4,
	}

	for method, expectedPriority := range expectedPriorities {
		cap, ok := caps[method]
		if !ok {
			t.Errorf("Missing capability for %s", method)
			continue
		}

		if cap.Priority != expectedPriority {
			t.Errorf("Method %s: expected priority %d, got %d",
				method, expectedPriority, cap.Priority)
		}
	}
}

// TestIntegration_DetectionWithTimeout tests detection with various timeouts
func TestIntegration_DetectionWithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"very short timeout", 100 * time.Millisecond},
		{"short timeout", 1 * time.Second},
		{"normal timeout", 5 * time.Second},
		{"long timeout", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.New("info")
			detector := NewDetector(log)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := detector.Detect(ctx)

			// Very short timeouts might fail, but shouldn't crash
			if err != nil && err != context.DeadlineExceeded {
				t.Logf("Detection with %v timeout: %v", tt.timeout, err)
			}

			// Should still have at least web capability
			caps := detector.GetCapabilities()
			if len(caps) == 0 {
				t.Error("No capabilities detected, expected at least web")
			}
		})
	}
}

// TestIntegration_ConcurrentAccess tests thread-safe access to capabilities
func TestIntegration_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.New("info")
	detector := NewDetector(log)

	ctx := context.Background()
	detector.Detect(ctx)

	// Simulate concurrent access
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			// Read capabilities
			caps := detector.GetCapabilities()
			if len(caps) == 0 {
				t.Error("No capabilities found")
			}

			// Check availability
			_ = detector.IsAvailable(ExportMethodWeb)

			// Get best method
			_ = detector.GetBestMethod()

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestIntegration_ReDetection tests running detection multiple times
func TestIntegration_ReDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.New("info")
	detector := NewDetector(log)

	ctx := context.Background()

	// Run detection multiple times
	for i := 0; i < 3; i++ {
		err := detector.Detect(ctx)
		if err != nil {
			t.Fatalf("Detection %d failed: %v", i, err)
		}

		caps := detector.GetCapabilities()
		if len(caps) != 4 {
			t.Errorf("Detection %d: expected 4 capabilities, got %d", i, len(caps))
		}

		// Brief pause between detections
		time.Sleep(100 * time.Millisecond)
	}

	// Final verification
	if !detector.IsAvailable(ExportMethodWeb) {
		t.Error("Web method should be available after re-detection")
	}
}

// TestIntegration_CapabilityTimestamps tests that timestamps are updated
func TestIntegration_CapabilityTimestamps(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	log := logger.New("info")
	detector := NewDetector(log)

	ctx := context.Background()

	// First detection
	err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("First detection failed: %v", err)
	}

	caps1 := detector.GetCapabilities()
	firstTimestamp := caps1[ExportMethodWeb].LastChecked

	// Wait a moment
	time.Sleep(100 * time.Millisecond)

	// Second detection
	err = detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Second detection failed: %v", err)
	}

	caps2 := detector.GetCapabilities()
	secondTimestamp := caps2[ExportMethodWeb].LastChecked

	if !secondTimestamp.After(firstTimestamp) {
		t.Error("Timestamp should be updated after re-detection")
	}
}
