// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// testLogger is a simple logger for tests that discards output
type testLogger struct{}

func (t *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (t *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (t *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (t *testLogger) Error(msg string, keysAndValues ...interface{}) {}

func newTestLogger() logger.Logger {
	return &testLogger{}
}

// Test successful operation (no retry needed)
func TestRetrySuccessFirstAttempt(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryer(config, log)

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return nil // Success on first attempt
	}

	err := retryer.Do(context.Background(), operation, "test-operation")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

// Test retry on transient error then success
func TestRetrySuccessAfterRetry(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryer(config, log)

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempt < 3 {
			return fmt.Errorf("connection timeout") // Retryable error
		}
		return nil // Success on 3rd attempt
	}

	start := time.Now()
	err := retryer.Do(context.Background(), operation, "test-operation")
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// Should have waited: 10ms + 20ms = 30ms (without jitter)
	expectedMinDelay := 30 * time.Millisecond
	if duration < expectedMinDelay {
		t.Errorf("Expected at least %v delay, got %v", expectedMinDelay, duration)
	}
}

// Test max attempts exceeded
func TestRetryMaxAttemptsExceeded(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryer(config, log)

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return fmt.Errorf("connection refused") // Always fail
	}

	err := retryer.Do(context.Background(), operation, "test-operation")
	if err == nil {
		t.Error("Expected error after max attempts")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("Expected 'failed after 3 attempts' in error, got: %v", err)
	}
}

// Test non-retryable error
func TestRetryNonRetryableError(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
	}

	retryer := NewRetryer(config, log)

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return fmt.Errorf("file not found") // Non-retryable error
	}

	err := retryer.Do(context.Background(), operation, "test-operation")
	if err == nil {
		t.Error("Expected error")
	}

	// Should fail immediately without retry
	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry for non-retryable error), got %d", attempts)
	}
}

// Test context cancellation
func TestRetryContextCancellation(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
	}

	retryer := NewRetryer(config, log)

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempt == 2 {
			cancel() // Cancel after 2nd attempt
		}
		return fmt.Errorf("timeout") // Retryable error
	}

	err := retryer.Do(ctx, operation, "test-operation")
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// Test exponential backoff
func TestRetryExponentialBackoff(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  4,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryer(config, log)

	var delays []time.Duration
	var lastTime time.Time

	operation := func(ctx context.Context, attempt int) error {
		now := time.Now()
		if attempt > 1 {
			delay := now.Sub(lastTime)
			delays = append(delays, delay)
		}
		lastTime = now
		return fmt.Errorf("timeout")
	}

	_ = retryer.Do(context.Background(), operation, "test-operation")

	// Expected delays: 10ms, 20ms, 40ms
	expectedDelays := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
	}

	if len(delays) != len(expectedDelays) {
		t.Fatalf("Expected %d delays, got %d", len(expectedDelays), len(delays))
	}

	for i, expected := range expectedDelays {
		// Allow 5ms tolerance
		tolerance := 5 * time.Millisecond
		if delays[i] < expected-tolerance || delays[i] > expected+tolerance {
			t.Errorf("Delay %d: expected ~%v, got %v", i, expected, delays[i])
		}
	}
}

// Test max delay cap
func TestRetryMaxDelayCap(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     200 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryer(config, log)

	// Calculate what delay should be for attempt 5 without cap
	// 100ms * 2^4 = 1600ms, but should be capped at 200ms
	delay := retryer.calculateDelay(5)

	if delay > config.MaxDelay {
		t.Errorf("Delay %v exceeds max delay %v", delay, config.MaxDelay)
	}

	// Should be exactly MaxDelay (no jitter)
	if delay != config.MaxDelay {
		t.Errorf("Expected delay %v, got %v", config.MaxDelay, delay)
	}
}

// Test retryable error detection
func TestIsRetryableError(t *testing.T) {
	log := newTestLogger()
	retryer := NewRetryer(DefaultRetryConfig(), log)

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"connection refused", fmt.Errorf("connection refused"), true},
		{"connection timeout", fmt.Errorf("connection timeout"), true},
		{"network unreachable", fmt.Errorf("network unreachable"), true},
		{"i/o timeout", fmt.Errorf("i/o timeout"), true},
		{"500 Internal Server Error", fmt.Errorf("500 Internal Server Error"), true},
		{"503 Service Unavailable", fmt.Errorf("503 Service Unavailable"), true},
		{"429 Too Many Requests", fmt.Errorf("429 Too Many Requests"), true},
		{"RequestTimeout", fmt.Errorf("RequestTimeout"), true},
		{"ThrottlingException", fmt.Errorf("ThrottlingException"), true},
		{"file not found", fmt.Errorf("file not found"), false},
		{"invalid argument", fmt.Errorf("invalid argument"), false},
		{"permission denied", fmt.Errorf("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryable := retryer.isRetryable(tt.err)
			if retryable != tt.retryable {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, retryable, tt.retryable)
			}
		})
	}
}

// Test DoWithResult
func TestRetryDoWithResult(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Jitter:       false,
	}

	retryer := NewRetryer(config, log)

	attempts := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attempts++
		if attempt < 2 {
			return nil, fmt.Errorf("timeout")
		}
		return "success", nil
	}

	result, err := retryer.DoWithResult(context.Background(), operation, "test-operation")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got: %v", result)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// Test WithRetry helper
func TestWithRetry(t *testing.T) {
	log := newTestLogger()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempt < 2 {
			return fmt.Errorf("connection reset")
		}
		return nil
	}

	err := WithRetry(context.Background(), operation, "test-operation", log)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// Test custom retryable errors
func TestCustomRetryableErrors(t *testing.T) {
	log := newTestLogger()

	customErr := errors.New("custom retryable error")

	config := &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    10 * time.Millisecond,
		RetryableErrors: []error{customErr},
	}

	retryer := NewRetryer(config, log)

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempt < 2 {
			return customErr
		}
		return nil
	}

	err := retryer.Do(context.Background(), operation, "test-operation")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// Test default config validation
func TestDefaultConfigValidation(t *testing.T) {
	log := newTestLogger()

	// Test with nil config
	retryer := NewRetryer(nil, log)
	if retryer.config.MaxAttempts != 3 {
		t.Errorf("Expected default MaxAttempts 3, got %d", retryer.config.MaxAttempts)
	}

	// Test with invalid values
	invalidConfig := &RetryConfig{
		MaxAttempts:  0,
		InitialDelay: 0,
		MaxDelay:     0,
		Multiplier:   0,
	}

	retryer = NewRetryer(invalidConfig, log)

	if retryer.config.MaxAttempts != 3 {
		t.Errorf("Expected default MaxAttempts 3, got %d", retryer.config.MaxAttempts)
	}
	if retryer.config.InitialDelay != 1*time.Second {
		t.Errorf("Expected default InitialDelay 1s, got %v", retryer.config.InitialDelay)
	}
	if retryer.config.MaxDelay != 30*time.Second {
		t.Errorf("Expected default MaxDelay 30s, got %v", retryer.config.MaxDelay)
	}
	if retryer.config.Multiplier != 2.0 {
		t.Errorf("Expected default Multiplier 2.0, got %v", retryer.config.Multiplier)
	}
}

// Test jitter
func TestRetryWithJitter(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	retryer := NewRetryer(config, log)

	// Calculate delays multiple times - they should vary due to jitter
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = retryer.calculateDelay(2)
	}

	// Check that not all delays are identical (jitter working)
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected delays to vary with jitter, but all were identical")
	}

	// All delays should be within base delay ± 25%
	baseDelay := 200 * time.Millisecond // 100ms * 2^1
	minDelay := baseDelay
	maxDelay := time.Duration(float64(baseDelay) * 1.25)

	for i, delay := range delays {
		if delay < minDelay || delay > maxDelay {
			t.Errorf("Delay %d (%v) outside expected range [%v, %v]", i, delay, minDelay, maxDelay)
		}
	}
}

// Benchmark retry overhead
func BenchmarkRetrySuccess(b *testing.B) {
	log := newTestLogger()
	config := DefaultRetryConfig()
	retryer := NewRetryer(config, log)

	operation := func(ctx context.Context, attempt int) error {
		return nil // Always succeed
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retryer.Do(context.Background(), operation, "benchmark")
	}
}

// Benchmark retry with failures
func BenchmarkRetryWithFailures(b *testing.B) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}
	retryer := NewRetryer(config, log)

	operation := func(ctx context.Context, attempt int) error {
		if attempt < 2 {
			return fmt.Errorf("timeout")
		}
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retryer.Do(context.Background(), operation, "benchmark")
	}
}

func TestRetryableError(t *testing.T) {
	baseErr := errors.New("test error")
	retryableErr := IsRetryable(baseErr)

	if retryableErr == nil {
		t.Fatal("Expected retryable error, got nil")
	}

	// Test Error() method
	if retryableErr.Error() != "test error" {
		t.Errorf("Expected error message 'test error', got '%s'", retryableErr.Error())
	}

	// Test Unwrap() method
	unwrapped := errors.Unwrap(retryableErr)
	if unwrapped != baseErr {
		t.Error("Expected unwrapped error to match base error")
	}

	// Test with nil error
	nilRetryable := IsRetryable(nil)
	if nilRetryable != nil {
		t.Error("Expected nil for retryable wrapper of nil error")
	}
}

func TestNonRetryableError(t *testing.T) {
	baseErr := errors.New("fatal error")
	nonRetryableErr := IsNonRetryable(baseErr)

	if nonRetryableErr == nil {
		t.Fatal("Expected non-retryable error, got nil")
	}

	// Test Error() method
	if nonRetryableErr.Error() != "fatal error" {
		t.Errorf("Expected error message 'fatal error', got '%s'", nonRetryableErr.Error())
	}

	// Test Unwrap() method
	unwrapped := errors.Unwrap(nonRetryableErr)
	if unwrapped != baseErr {
		t.Error("Expected unwrapped error to match base error")
	}

	// Test with nil error
	nilNonRetryable := IsNonRetryable(nil)
	if nilNonRetryable != nil {
		t.Error("Expected nil for non-retryable wrapper of nil error")
	}
}

func TestWithCustomRetry(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempt < 2 {
			// Return an error that matches retryable patterns
			return errors.New("connection timeout")
		}
		return nil
	}

	err := WithCustomRetry(context.Background(), operation, "custom-retry-test", config, log)
	if err != nil {
		t.Errorf("Expected success after retry, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestSetNetworkMonitor(t *testing.T) {
	log := newTestLogger()
	config := DefaultRetryConfig()
	retryer := NewRetryer(config, log)

	// Create a mock network monitor
	monitor := &mockNetworkMonitor{
		isUp: true,
	}

	// Test SetNetworkMonitor
	retryer.SetNetworkMonitor(monitor)

	// Verify the monitor was set by checking if it's used during retry
	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return nil
	}

	err := retryer.Do(context.Background(), operation, "test-with-monitor")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestNetworkMonitorRecovery(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:    3,
		InitialDelay:   10 * time.Millisecond,
		WaitForNetwork: true,
	}
	retryer := NewRetryer(config, log)

	// Create a mock network monitor that simulates network recovery
	monitor := &mockNetworkMonitor{
		isUp:              false,
		recoverAfterCalls: 1,
	}
	retryer.SetNetworkMonitor(monitor)

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		if attempt < 2 {
			return errors.New("connection timeout")
		}
		return nil
	}

	err := retryer.Do(context.Background(), operation, "test-network-recovery")
	if err != nil {
		t.Errorf("Expected success after network recovery, got: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}

	if monitor.waitCalled < 1 {
		t.Error("Expected WaitForNetwork to be called")
	}
}

func TestNetworkMonitorContextCancellation(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:    5,
		InitialDelay:   10 * time.Millisecond,
		WaitForNetwork: true,
	}
	retryer := NewRetryer(config, log)

	// Create a monitor that never recovers
	monitor := &mockNetworkMonitor{
		isUp:          false,
		cancelContext: true,
	}
	retryer.SetNetworkMonitor(monitor)

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return errors.New("connection timeout")
	}

	err := retryer.Do(context.Background(), operation, "test-network-cancel")
	if err == nil {
		t.Error("Expected error due to context cancellation during network wait")
	}

	if !strings.Contains(err.Error(), "network wait cancelled") {
		t.Errorf("Expected 'network wait cancelled' in error, got: %v", err)
	}
}

func TestContextAlreadyCancelled(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
	}
	retryer := NewRetryer(config, log)

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	operation := func(ctx context.Context, attempt int) error {
		attempts++
		return errors.New("should not reach here")
	}

	err := retryer.Do(ctx, operation, "test-cancelled-context")
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	// Should not execute operation at all
	if attempts != 0 {
		t.Errorf("Expected 0 attempts (context already cancelled), got %d", attempts)
	}
}

func TestDoWithResultNetworkRecovery(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:    3,
		InitialDelay:   10 * time.Millisecond,
		WaitForNetwork: true,
	}
	retryer := NewRetryer(config, log)

	monitor := &mockNetworkMonitor{
		isUp:              false,
		recoverAfterCalls: 1,
	}
	retryer.SetNetworkMonitor(monitor)

	attempts := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attempts++
		if attempt < 2 {
			return nil, errors.New("network timeout")
		}
		return "recovered", nil
	}

	result, err := retryer.DoWithResult(context.Background(), operation, "test-result-network")
	if err != nil {
		t.Errorf("Expected success after network recovery, got: %v", err)
	}

	if result != "recovered" {
		t.Errorf("Expected 'recovered', got: %v", result)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestDoWithResultContextCancelled(t *testing.T) {
	log := newTestLogger()
	config := &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
	}
	retryer := NewRetryer(config, log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	operation := func(ctx context.Context, attempt int) (interface{}, error) {
		attempts++
		return nil, errors.New("should not execute")
	}

	result, err := retryer.DoWithResult(ctx, operation, "test-result-cancelled")
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}

	if result != nil {
		t.Errorf("Expected nil result, got: %v", result)
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	if attempts != 0 {
		t.Errorf("Expected 0 attempts, got %d", attempts)
	}
}

// mockNetworkMonitor is a test implementation of NetworkMonitor
type mockNetworkMonitor struct {
	isUp              bool
	waitCalled        int
	recoverAfterCalls int
	cancelContext     bool
}

func (m *mockNetworkMonitor) IsUp() bool {
	return m.isUp
}

func (m *mockNetworkMonitor) WaitForNetwork(ctx context.Context) error {
	m.waitCalled++

	if m.cancelContext {
		return context.Canceled
	}

	if m.recoverAfterCalls > 0 && m.waitCalled >= m.recoverAfterCalls {
		m.isUp = true
		return nil
	}

	if m.isUp {
		return nil
	}

	return errors.New("network offline")
}
