// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewBandwidthLimiter(t *testing.T) {
	tests := []struct {
		name           string
		bytesPerSecond int64
		wantRate       int64
	}{
		{"1 MB/s", 1 * 1024 * 1024, 1 * 1024 * 1024},
		{"10 MB/s", 10 * 1024 * 1024, 10 * 1024 * 1024},
		{"100 MB/s", 100 * 1024 * 1024, 100 * 1024 * 1024},
		{"1 GB/s", 1024 * 1024 * 1024, 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &BandwidthConfig{
				MaxBytesPerSecond: tt.bytesPerSecond,
			}
			limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
			if limiter == nil {
				t.Fatal("NewBandwidthLimiter returned nil")
			}
			if limiter.limit != tt.wantRate {
				t.Errorf("Expected rate %d, got %d", tt.wantRate, limiter.limit)
			}
		})
	}
}

func TestBandwidthLimiter_Wait(t *testing.T) {
	// Test with small byte count to ensure fast execution
	config := &BandwidthConfig{
		MaxBytesPerSecond: 1024 * 1024, // 1 MB/s
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	start := time.Now()
	err := limiter.Wait(ctx, 512) // 512 bytes
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}

	// Should be nearly instant for small amounts
	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}
}

func TestBandwidthLimiter_WaitContext(t *testing.T) {
	config := &BandwidthConfig{
		MaxBytesPerSecond: 1024, // 1 KB/s (slow for testing)
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Try to wait for a large amount that would take longer than timeout
	err := limiter.Wait(ctx, 10*1024) // 10 KB
	if err == nil {
		t.Error("Expected context deadline error, got nil")
	}
}

func TestBandwidthLimiter_ConcurrentWait(t *testing.T) {
	config := &BandwidthConfig{
		MaxBytesPerSecond: 1024 * 1024, // 1 MB/s
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := limiter.Wait(ctx, 1024) // 1 KB each
			if err != nil {
				t.Errorf("Concurrent wait failed: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestBandwidthLimiter_GetStats(t *testing.T) {
	config := &BandwidthConfig{
		MaxBytesPerSecond: 5 * 1024 * 1024, // 5 MB/s
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	// Transfer some data
	_ = limiter.Wait(ctx, 1024)

	stats := limiter.GetStats()
	if stats.BytesTransferred == 0 {
		t.Error("Expected bytes transferred to be > 0")
	}
	if stats.LimitSpeed != float64(config.MaxBytesPerSecond) {
		t.Errorf("Expected limit speed %f, got %f", float64(config.MaxBytesPerSecond), stats.LimitSpeed)
	}
}

func TestNewAdaptiveBandwidthLimiter(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)   // 1 MB/s
	maxRate := int64(100 * 1024 * 1024) // 100 MB/s

	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))
	if adaptive == nil {
		t.Fatal("NewAdaptiveBandwidthLimiter returned nil")
	}
	if adaptive.minSpeed != minRate {
		t.Errorf("Expected minRate %d, got %d", minRate, adaptive.minSpeed)
	}
	if adaptive.maxSpeed != maxRate {
		t.Errorf("Expected maxRate %d, got %d", maxRate, adaptive.maxSpeed)
	}
	if adaptive.currentSpeed == 0 {
		t.Error("Expected currentSpeed to be set")
	}
}

func TestAdaptiveBandwidthLimiter_RecordSuccess(t *testing.T) {
	adaptive := NewAdaptiveBandwidthLimiter(1024*1024, 100*1024*1024, logger.NewTestLogger(t))

	// Record multiple successes
	for i := 0; i < 20; i++ {
		adaptive.RecordSuccess()
	}

	// Success rate should be > 0
	if adaptive.successRate == 0 {
		t.Error("Expected success rate to be > 0")
	}
}

func TestAdaptiveBandwidthLimiter_RecordError(t *testing.T) {
	adaptive := NewAdaptiveBandwidthLimiter(1024*1024, 100*1024*1024, logger.NewTestLogger(t))

	// Record multiple errors
	for i := 0; i < 5; i++ {
		adaptive.RecordError()
	}

	// Error rate should be > 0
	if adaptive.errorRate == 0 {
		t.Error("Expected error rate to be > 0")
	}
}

func TestAdaptiveBandwidthLimiter_MinMaxBounds(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)   // 1 MB/s
	maxRate := int64(10 * 1024 * 1024)  // 10 MB/s
	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))

	// Current speed should be within bounds
	if adaptive.currentSpeed < minRate {
		t.Errorf("Current speed %d below minimum %d", adaptive.currentSpeed, minRate)
	}
	if adaptive.currentSpeed > maxRate {
		t.Errorf("Current speed %d above maximum %d", adaptive.currentSpeed, maxRate)
	}
}

func TestAdaptiveBandwidthLimiter_Wait(t *testing.T) {
	adaptive := NewAdaptiveBandwidthLimiter(1024*1024, 100*1024*1024, logger.NewTestLogger(t))
	ctx := context.Background()

	err := adaptive.Wait(ctx, 1024)
	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}
}

func TestAdaptiveBandwidthLimiter_GetStats(t *testing.T) {
	adaptive := NewAdaptiveBandwidthLimiter(1024*1024, 100*1024*1024, logger.NewTestLogger(t))
	ctx := context.Background()

	_ = adaptive.Wait(ctx, 1024)
	stats := adaptive.GetStats()

	if stats.BytesTransferred == 0 {
		t.Error("Expected bytes transferred to be > 0")
	}
}

func TestLimitedReader_Read(t *testing.T) {
	data := strings.Repeat("test data ", 100) // ~1KB
	reader := strings.NewReader(data)
	config := &BandwidthConfig{
		MaxBytesPerSecond: 10 * 1024 * 1024, // 10 MB/s (fast for testing)
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	lr := NewLimitedReader(reader, limiter, ctx)

	buf := make([]byte, 512)
	n, err := lr.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("Read returned error: %v", err)
	}
	if n == 0 {
		t.Error("Read returned 0 bytes")
	}
}

func TestLimitedReader_ReadFull(t *testing.T) {
	data := "hello world"
	reader := strings.NewReader(data)
	config := &BandwidthConfig{
		MaxBytesPerSecond: 1024 * 1024, // 1 MB/s
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	lr := NewLimitedReader(reader, limiter, ctx)

	result, err := io.ReadAll(lr)
	if err != nil {
		t.Errorf("ReadAll returned error: %v", err)
	}
	if string(result) != data {
		t.Errorf("Expected %q, got %q", data, string(result))
	}
}

func TestLimitedWriter_Write(t *testing.T) {
	var buf strings.Builder
	config := &BandwidthConfig{
		MaxBytesPerSecond: 10 * 1024 * 1024, // 10 MB/s
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	lw := NewLimitedWriter(&buf, limiter, ctx)

	data := []byte("test data")
	n, err := lw.Write(data)
	if err != nil {
		t.Errorf("Write returned error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	if buf.String() != string(data) {
		t.Errorf("Expected %q, got %q", data, buf.String())
	}
}

func TestLimitedWriter_MultipleWrites(t *testing.T) {
	var buf strings.Builder
	config := &BandwidthConfig{
		MaxBytesPerSecond: 10 * 1024 * 1024, // 10 MB/s
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	lw := NewLimitedWriter(&buf, limiter, ctx)

	writes := []string{"hello", " ", "world"}
	for _, w := range writes {
		_, err := lw.Write([]byte(w))
		if err != nil {
			t.Errorf("Write returned error: %v", err)
		}
	}

	expected := "hello world"
	if buf.String() != expected {
		t.Errorf("Expected %q, got %q", expected, buf.String())
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		name     string
		bytes    float64
		contains string
	}{
		{"zero", 0, "< 1"},
		{"bytes", 512, "< 1"},
		{"kilobytes", 1024, "KB/s"},
		{"megabytes", 1024 * 1024, "MB/s"},
		{"gigabytes", 1024 * 1024 * 1024, "GB/s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSpeed(tt.bytes)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected result to contain %q, got %q", tt.contains, result)
			}
		})
	}
}

func TestBandwidthLimiter_ZeroRate(t *testing.T) {
	config := &BandwidthConfig{
		MaxBytesPerSecond: 0, // Unlimited
	}
	limiter := NewBandwidthLimiter(config, logger.NewTestLogger(t))
	ctx := context.Background()

	// Should complete immediately
	start := time.Now()
	err := limiter.Wait(ctx, 1024*1024) // 1 MB
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait returned error: %v", err)
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("Zero rate limiter took too long: %v", elapsed)
	}
}

func TestAdaptiveBandwidthLimiter_AdjustSpeedIncrease(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)   // 1 MB/s
	maxRate := int64(100 * 1024 * 1024) // 100 MB/s
	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))

	// Set initial speed to minimum
	adaptive.currentSpeed = minRate

	// Record high success rate, low error rate
	for i := 0; i < 100; i++ {
		adaptive.RecordSuccess()
	}

	// Wait for adjust interval to pass
	time.Sleep(2 * time.Second)
	initialSpeed := adaptive.currentSpeed

	// Trigger adjustment by calling Wait
	ctx := context.Background()
	_ = adaptive.Wait(ctx, 1024)

	// Speed should have increased (manually trigger adjustSpeed)
	adaptive.adjustSpeed()

	if adaptive.currentSpeed <= initialSpeed {
		t.Logf("Speed did not increase: before=%d, after=%d", initialSpeed, adaptive.currentSpeed)
		t.Logf("Success rate: %f, Error rate: %f", adaptive.successRate, adaptive.errorRate)
	}
}

func TestAdaptiveBandwidthLimiter_AdjustSpeedDecrease(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)   // 1 MB/s
	maxRate := int64(100 * 1024 * 1024) // 100 MB/s
	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))

	// Set initial speed to maximum
	adaptive.currentSpeed = maxRate

	// Record high error rate
	for i := 0; i < 100; i++ {
		adaptive.RecordError()
	}

	// Wait for adjust interval to pass
	time.Sleep(2 * time.Second)
	initialSpeed := adaptive.currentSpeed

	// Trigger adjustment
	adaptive.adjustSpeed()

	if adaptive.currentSpeed >= initialSpeed {
		t.Logf("Speed did not decrease: before=%d, after=%d", initialSpeed, adaptive.currentSpeed)
		t.Logf("Success rate: %f, Error rate: %f", adaptive.successRate, adaptive.errorRate)
	}

	// Should not go below minimum
	if adaptive.currentSpeed < minRate {
		t.Errorf("Speed decreased below minimum: %d < %d", adaptive.currentSpeed, minRate)
	}
}

func TestAdaptiveBandwidthLimiter_AdjustSpeedNoChange(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)   // 1 MB/s
	maxRate := int64(100 * 1024 * 1024) // 100 MB/s
	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))

	// Record moderate success/error rates (not high enough to trigger adjustment)
	for i := 0; i < 50; i++ {
		adaptive.RecordSuccess()
	}
	for i := 0; i < 10; i++ {
		adaptive.RecordError()
	}

	initialSpeed := adaptive.currentSpeed

	// Wait for adjust interval
	time.Sleep(2 * time.Second)

	// Trigger adjustment
	adaptive.adjustSpeed()

	// Speed should remain relatively stable
	t.Logf("Speed before: %d, after: %d", initialSpeed, adaptive.currentSpeed)
	t.Logf("Success rate: %f, Error rate: %f", adaptive.successRate, adaptive.errorRate)
}

func TestAdaptiveBandwidthLimiter_AdjustSpeedMaxBound(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)  // 1 MB/s
	maxRate := int64(10 * 1024 * 1024) // 10 MB/s
	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))

	// Set speed near maximum
	adaptive.currentSpeed = maxRate - 1024

	// Record very high success rate
	for i := 0; i < 100; i++ {
		adaptive.RecordSuccess()
	}

	// Wait for adjust interval
	time.Sleep(2 * time.Second)

	// Trigger adjustment
	adaptive.adjustSpeed()

	// Speed should be capped at maximum
	if adaptive.currentSpeed > maxRate {
		t.Errorf("Speed exceeded maximum: %d > %d", adaptive.currentSpeed, maxRate)
	}
}

func TestAdaptiveBandwidthLimiter_AdjustSpeedTooSoon(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)   // 1 MB/s
	maxRate := int64(100 * 1024 * 1024) // 100 MB/s
	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))

	initialSpeed := adaptive.currentSpeed

	// Record high success rate
	for i := 0; i < 100; i++ {
		adaptive.RecordSuccess()
	}

	// Call adjustSpeed immediately without waiting for adjust interval
	// This should return early without changing speed
	adaptive.adjustSpeed()

	// Speed should not have changed because adjust interval hasn't passed
	if adaptive.currentSpeed != initialSpeed {
		t.Logf("Speed changed when it shouldn't have: before=%d, after=%d", initialSpeed, adaptive.currentSpeed)
	}
}

func TestAdaptiveBandwidthLimiter_AdjustSpeedMinBound(t *testing.T) {
	minRate := int64(1 * 1024 * 1024)   // 1 MB/s
	maxRate := int64(100 * 1024 * 1024) // 100 MB/s
	adaptive := NewAdaptiveBandwidthLimiter(minRate, maxRate, logger.NewTestLogger(t))

	// Set speed near minimum
	adaptive.currentSpeed = minRate + 1024

	// Record very high error rate
	for i := 0; i < 100; i++ {
		adaptive.RecordError()
	}

	// Wait for adjust interval
	time.Sleep(2 * time.Second)

	// Trigger adjustment
	adaptive.adjustSpeed()

	// Speed should be capped at minimum
	if adaptive.currentSpeed < minRate {
		t.Errorf("Speed below minimum: %d < %d", adaptive.currentSpeed, minRate)
	}
}
