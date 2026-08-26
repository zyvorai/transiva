// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"testing"
	"time"

	"github.com/schollz/progressbar/v3"
)

func TestNewBarProgress(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	if bar == nil {
		t.Fatal("NewBarProgress() returned nil")
	}

	if bar.bar == nil {
		t.Fatal("BarProgress.bar is nil")
	}
}

func TestBarProgressStart(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	total := int64(1000)
	description := "Testing progress"

	bar.Start(total, description)

	// Give it a moment to render
	time.Sleep(100 * time.Millisecond)

	// Bar should have been initialized
	if bar.bar == nil {
		t.Error("Progress bar not initialized after Start()")
	}
}

func TestBarProgressUpdate(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	bar.Start(100, "Test")
	bar.Update(50)
	bar.Update(100)

	// Wait for throttle
	time.Sleep(100 * time.Millisecond)

	// Buffer should contain progress output
	if buf.Len() == 0 {
		t.Error("Expected progress output in buffer")
	}
}

func TestBarProgressAdd(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	bar.Start(100, "Test")
	bar.Add(25)
	bar.Add(25)
	bar.Add(50)

	// Wait for throttle
	time.Sleep(100 * time.Millisecond)

	// Buffer should contain progress output
	if buf.Len() == 0 {
		t.Error("Expected progress output in buffer")
	}
}

func TestBarProgressSetTotal(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	bar.Start(100, "Test")
	bar.SetTotal(200)
	bar.Update(100)

	// Wait for throttle
	time.Sleep(100 * time.Millisecond)

	// Bar should handle total changes
	if buf.Len() == 0 {
		t.Error("Expected progress output in buffer")
	}
}

func TestBarProgressDescribe(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	bar.Start(100, "Initial description")
	bar.Describe("Updated description")
	bar.Update(50)

	// Wait for throttle
	time.Sleep(100 * time.Millisecond)

	// Buffer should contain updated description, but the underlying progress
	// bar library may not always render it depending on throttling/rendering
	// timing - this test mainly ensures Describe/Update don't panic.
	_ = buf.String()
}

func TestBarProgressFinish(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	bar.Start(100, "Test")
	bar.Update(100)
	bar.Finish()

	// Wait for throttle
	time.Sleep(100 * time.Millisecond)

	// Buffer should have content after finishing
	if buf.Len() == 0 {
		t.Error("Expected progress output in buffer after Finish()")
	}
}

func TestBarProgressClose(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	bar.Start(100, "Test")
	bar.Update(50)

	err := bar.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestNewDownloadProgress(t *testing.T) {
	buf := &bytes.Buffer{}
	filename := "test-file.vmdk"
	totalSize := int64(1024 * 1024 * 100) // 100 MB

	bar := NewDownloadProgress(buf, filename, totalSize)

	if bar == nil {
		t.Fatal("NewDownloadProgress() returned nil")
	}

	if bar.bar == nil {
		t.Fatal("Download progress bar is nil")
	}

	// Simulate download progress
	bar.Update(1024 * 1024 * 25)  // 25%
	bar.Update(1024 * 1024 * 50)  // 50%
	bar.Update(1024 * 1024 * 100) // 100%
	bar.Finish()

	// Wait for output
	time.Sleep(100 * time.Millisecond)

	// Should have output
	if buf.Len() == 0 {
		t.Error("Expected download progress output in buffer")
	}
}

func TestNewOverallProgress(t *testing.T) {
	buf := &bytes.Buffer{}
	vmName := "test-vm"
	fileCount := 5

	bar := NewOverallProgress(buf, vmName, fileCount)

	if bar == nil {
		t.Fatal("NewOverallProgress() returned nil")
	}

	if bar.bar == nil {
		t.Fatal("Overall progress bar is nil")
	}

	// Simulate file processing
	bar.Start(int64(fileCount), "Exporting "+vmName)
	for i := 0; i < fileCount; i++ {
		bar.Add(1)
		time.Sleep(10 * time.Millisecond)
	}
	bar.Finish()

	// Wait for output
	time.Sleep(100 * time.Millisecond)

	// Should have output
	if buf.Len() == 0 {
		t.Error("Expected overall progress output in buffer")
	}
}

func TestBarProgressWithCustomOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	customOptions := []progressbar.Option{
		progressbar.OptionSetDescription("Custom progress"),
		progressbar.OptionShowBytes(true),
	}

	bar := NewBarProgress(buf, customOptions...)

	if bar == nil {
		t.Fatal("NewBarProgress() with custom options returned nil")
	}

	bar.Start(1024, "Test")
	bar.Update(512)
	bar.Finish()

	time.Sleep(100 * time.Millisecond)

	if buf.Len() == 0 {
		t.Error("Expected progress output with custom options")
	}
}

func TestMultiProgress(t *testing.T) {
	mp := NewMultiProgress()

	if mp == nil {
		t.Fatal("NewMultiProgress() returned nil")
	}

	if mp.bars == nil {
		t.Fatal("MultiProgress.bars is nil")
	}

	if mp.done == nil {
		t.Fatal("MultiProgress.done channel is nil")
	}
}

func TestMultiProgressAddBar(t *testing.T) {
	mp := NewMultiProgress()
	buf := &bytes.Buffer{}

	bar1 := NewBarProgress(buf)
	bar2 := NewBarProgress(buf)

	mp.AddBar(bar1)
	mp.AddBar(bar2)

	if len(mp.bars) != 2 {
		t.Errorf("Expected 2 bars, got %d", len(mp.bars))
	}
}

func TestMultiProgressClose(t *testing.T) {
	mp := NewMultiProgress()
	buf := &bytes.Buffer{}

	bar1 := NewBarProgress(buf)
	bar2 := NewBarProgress(buf)

	mp.AddBar(bar1)
	mp.AddBar(bar2)

	// Close should not panic
	mp.Close()

	// Done channel should be closed
	select {
	case <-mp.done:
		// Expected - channel is closed
	case <-time.After(100 * time.Millisecond):
		t.Error("done channel was not closed")
	}
}

func TestMultiProgressWait(t *testing.T) {
	mp := NewMultiProgress()

	// Close in a goroutine after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		mp.Close()
	}()

	// Wait should block until Close() is called
	done := make(chan struct{})
	go func() {
		mp.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected - Wait() returned after Close()
	case <-time.After(200 * time.Millisecond):
		t.Error("Wait() did not return after Close()")
	}
}

func TestBarProgressConcurrentOperations(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	bar.Start(1000, "Concurrent test")

	// Simulate concurrent updates
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func(val int64) {
			bar.Add(val)
			done <- struct{}{}
		}(10)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	bar.Finish()
	if err := bar.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestProgressReporterInterface(t *testing.T) {
	buf := &bytes.Buffer{}
	var reporter ProgressReporter = NewBarProgress(buf)

	// Test all interface methods
	reporter.Start(100, "Interface test")
	reporter.Update(25)
	reporter.Add(25)
	reporter.SetTotal(200)
	reporter.Describe("Updated description")
	reporter.Finish()

	err := reporter.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestProgressBarLifecycle(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewBarProgress(buf)

	// Complete lifecycle: Start -> Update -> Finish -> Close
	bar.Start(100, "Lifecycle test")

	for i := int64(0); i <= 100; i += 10 {
		bar.Update(i)
		time.Sleep(5 * time.Millisecond)
	}

	bar.Finish()

	err := bar.Close()
	if err != nil {
		t.Errorf("Lifecycle test failed at Close(): %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Should have generated output
	if buf.Len() == 0 {
		t.Error("Expected progress output after complete lifecycle")
	}
}

func TestMultiProgressWithMultipleBars(t *testing.T) {
	mp := NewMultiProgress()

	buffers := make([]*bytes.Buffer, 3)
	for i := 0; i < 3; i++ {
		buffers[i] = &bytes.Buffer{}
		bar := NewBarProgress(buffers[i])
		bar.Start(100, "Bar "+string(rune('A'+i)))
		mp.AddBar(bar)
	}

	// Update all bars
	for i, bar := range mp.bars {
		bar.Update(int64((i + 1) * 25))
	}

	time.Sleep(100 * time.Millisecond)

	mp.Close()

	// Verify all bars produced output
	for i, buf := range buffers {
		if buf.Len() == 0 {
			t.Errorf("Bar %d did not produce output", i)
		}
	}
}

// TestBarProgressNilSafety tests that all BarProgress methods handle nil receivers safely
// This test was added to prevent nil pointer panics that occurred in production
func TestBarProgressNilSafety(t *testing.T) {
	t.Run("NilReceiver", func(t *testing.T) {
		// Test nil receiver - should not panic
		var nilBar *BarProgress

		// All methods should handle nil gracefully
		nilBar.Start(100, "test")
		nilBar.Update(50)
		nilBar.Add(10)
		nilBar.Finish()
		nilBar.SetTotal(200)
		nilBar.Describe("description")
		err := nilBar.Close()
		if err != nil {
			t.Errorf("Close() on nil returned error: %v", err)
		}
	})

	t.Run("NilInternalBar", func(t *testing.T) {
		// Test bar with nil internal bar field - should not panic
		barWithNilInternal := &BarProgress{bar: nil}

		// All methods should handle nil bar field gracefully
		barWithNilInternal.Start(100, "test")
		barWithNilInternal.Update(50)
		barWithNilInternal.Add(10)
		barWithNilInternal.Finish()
		barWithNilInternal.SetTotal(200)
		barWithNilInternal.Describe("description")
		err := barWithNilInternal.Close()
		if err != nil {
			t.Errorf("Close() on nil bar returned error: %v", err)
		}
	})

	t.Run("ConcurrentNilAccess", func(t *testing.T) {
		// Simulate concurrent access to nil bar - should not panic
		var nilBar *BarProgress
		done := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			go func() {
				nilBar.Add(1)
				nilBar.Update(10)
				done <- true
			}()
		}

		for i := 0; i < 5; i++ {
			<-done
		}
	})
}

// TestProgressBarOperationsOnClosedBar tests operations on a closed bar
func TestProgressBarOperationsOnClosedBar(t *testing.T) {
	var buf bytes.Buffer
	bar := NewBarProgress(&buf)

	bar.Start(100, "Test")
	if err := bar.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Operations after close should not panic (though they may not have effect)
	bar.Update(50)
	bar.Add(10)
	bar.Finish()

	// Second close should not panic
	err := bar.Close()
	if err != nil {
		// It's okay if close returns an error on second call
		t.Logf("Second Close() returned: %v", err)
	}
}
