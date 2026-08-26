// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// RetryConfig contains retry configuration
type RetryConfig struct {
	MaxAttempts     int           // Maximum number of retry attempts (default: 3)
	InitialDelay    time.Duration // Initial delay between retries (default: 1s)
	MaxDelay        time.Duration // Maximum delay between retries (default: 30s)
	Multiplier      float64       // Exponential backoff multiplier (default: 2.0)
	Jitter          bool          // Add random jitter to delays (default: true)
	RetryableErrors []error       // Specific errors that should trigger retry
	WaitForNetwork  bool          // Wait for network to come back up before retrying (default: false)
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryOperation represents an operation that can be retried
type RetryOperation func(ctx context.Context, attempt int) error

// NetworkMonitor represents a network state monitor
type NetworkMonitor interface {
	IsUp() bool
	WaitForNetwork(ctx context.Context) error
}

// Retryer handles retry logic with exponential backoff
type Retryer struct {
	config         *RetryConfig
	log            logger.Logger
	networkMonitor NetworkMonitor // Optional network monitor
}

// NewRetryer creates a new retryer with the given configuration
func NewRetryer(config *RetryConfig, log logger.Logger) *Retryer {
	if config == nil {
		config = DefaultRetryConfig()
	}

	// Validate and apply defaults
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 1 * time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}

	return &Retryer{
		config: config,
		log:    log,
	}
}

// SetNetworkMonitor sets the network monitor for the retryer
func (r *Retryer) SetNetworkMonitor(monitor NetworkMonitor) {
	r.networkMonitor = monitor
}

// Do executes the operation with retry logic
func (r *Retryer) Do(ctx context.Context, operation RetryOperation, operationName string) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check if context is already cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", operationName, ctx.Err())
		default:
		}

		// Execute operation
		err := operation(ctx, attempt)
		if err == nil {
			// Success
			if attempt > 1 {
				r.log.Info("operation succeeded after retry",
					"operation", operationName,
					"attempt", attempt,
					"total_attempts", r.config.MaxAttempts)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			r.log.Warn("operation failed with non-retryable error",
				"operation", operationName,
				"attempt", attempt,
				"error", err)
			return fmt.Errorf("%s (attempt %d/%d): %w", operationName, attempt, r.config.MaxAttempts, err)
		}

		// Don't sleep after last attempt
		if attempt >= r.config.MaxAttempts {
			r.log.Error("operation failed after max attempts",
				"operation", operationName,
				"attempts", r.config.MaxAttempts,
				"error", err)
			return fmt.Errorf("%s failed after %d attempts: %w", operationName, r.config.MaxAttempts, err)
		}

		// Check network state if network monitor is available
		if r.networkMonitor != nil && r.config.WaitForNetwork {
			if !r.networkMonitor.IsUp() {
				r.log.Warn("network is down, waiting for network to recover",
					"operation", operationName,
					"attempt", attempt)

				// Wait for network to come back up
				if err := r.networkMonitor.WaitForNetwork(ctx); err != nil {
					// Context cancelled while waiting for network
					return fmt.Errorf("%s: network wait cancelled: %w", operationName, err)
				}

				r.log.Info("network recovered, retrying operation",
					"operation", operationName,
					"attempt", attempt)

				// Network is back up, continue with retry
				// Don't sleep since we already waited for network
				continue
			}
		}

		// Calculate delay with exponential backoff
		delay := r.calculateDelay(attempt)

		r.log.Warn("operation failed, retrying",
			"operation", operationName,
			"attempt", attempt,
			"max_attempts", r.config.MaxAttempts,
			"delay", delay,
			"error", err)

		// Wait before retry
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", operationName, ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operationName, r.config.MaxAttempts, lastErr)
}

// DoWithResult executes the operation with retry logic and returns a result
func (r *Retryer) DoWithResult(ctx context.Context, operation func(ctx context.Context, attempt int) (interface{}, error), operationName string) (interface{}, error) {
	var lastErr error
	var result interface{}

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%s: %w", operationName, ctx.Err())
		default:
		}

		res, err := operation(ctx, attempt)
		if err == nil {
			if attempt > 1 {
				r.log.Info("operation succeeded after retry",
					"operation", operationName,
					"attempt", attempt,
					"total_attempts", r.config.MaxAttempts)
			}
			return res, nil
		}

		lastErr = err

		if !r.isRetryable(err) {
			r.log.Warn("operation failed with non-retryable error",
				"operation", operationName,
				"attempt", attempt,
				"error", err)
			return nil, fmt.Errorf("%s (attempt %d/%d): %w", operationName, attempt, r.config.MaxAttempts, err)
		}

		if attempt >= r.config.MaxAttempts {
			r.log.Error("operation failed after max attempts",
				"operation", operationName,
				"attempts", r.config.MaxAttempts,
				"error", err)
			return nil, fmt.Errorf("%s failed after %d attempts: %w", operationName, r.config.MaxAttempts, err)
		}

		// Check network state if network monitor is available
		if r.networkMonitor != nil && r.config.WaitForNetwork {
			if !r.networkMonitor.IsUp() {
				r.log.Warn("network is down, waiting for network to recover",
					"operation", operationName,
					"attempt", attempt)

				// Wait for network to come back up
				if err := r.networkMonitor.WaitForNetwork(ctx); err != nil {
					// Context cancelled while waiting for network
					return nil, fmt.Errorf("%s: network wait cancelled: %w", operationName, err)
				}

				r.log.Info("network recovered, retrying operation",
					"operation", operationName,
					"attempt", attempt)

				// Network is back up, continue with retry
				// Don't sleep since we already waited for network
				continue
			}
		}

		delay := r.calculateDelay(attempt)

		r.log.Warn("operation failed, retrying",
			"operation", operationName,
			"attempt", attempt,
			"max_attempts", r.config.MaxAttempts,
			"delay", delay,
			"error", err)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%s: %w", operationName, ctx.Err())
		case <-time.After(delay):
		}
	}

	return result, fmt.Errorf("%s failed after %d attempts: %w", operationName, r.config.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay before next retry using exponential backoff
func (r *Retryer) calculateDelay(attempt int) time.Duration {
	// Calculate exponential delay: initialDelay * (multiplier ^ (attempt - 1))
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if r.config.Jitter {
		// Add random jitter up to 25% of delay
		// #nosec G404 -- non-cryptographic jitter for retry backoff timing, not a security-sensitive value
		jitter := delay * 0.25 * rand.Float64()
		delay += jitter
	}

	return time.Duration(delay)
}

// isRetryable checks if an error should trigger a retry
func (r *Retryer) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check against configured retryable errors
	for _, retryableErr := range r.config.RetryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	// Check for common retryable error patterns
	errMsg := err.Error()

	// Network errors
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network unreachable",
		"no such host",
		"temporary failure",
		"timeout",
		"TLS handshake timeout",
		"i/o timeout",
		"broken pipe",
		"EOF",
	}

	for _, pattern := range networkErrors {
		if containsIgnoreCase(errMsg, pattern) {
			return true
		}
	}

	// HTTP/Cloud service errors (5xx server errors, 429 rate limit)
	httpErrors := []string{
		"500 Internal Server Error",
		"502 Bad Gateway",
		"503 Service Unavailable",
		"504 Gateway Timeout",
		"429 Too Many Requests",
		"RequestTimeout",
		"ServiceUnavailable",
		"InternalError",
		"SlowDown",
		"ThrottlingException",
	}

	for _, pattern := range httpErrors {
		if containsIgnoreCase(errMsg, pattern) {
			return true
		}
	}

	// Cloud provider specific errors
	cloudErrors := []string{
		"RequestLimitExceeded",
		"ProvisionedThroughputExceededException",
		"TransactionInProgressException",
		"TooManyRequests",
	}

	for _, pattern := range cloudErrors {
		if containsIgnoreCase(errMsg, pattern) {
			return true
		}
	}

	return false
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return containsString(s, substr)
}

// containsString checks if string contains substring
func containsString(s, substr string) bool {
	// Simple case-insensitive contains
	sLower := toLower(s)
	substrLower := toLower(substr)
	return len(sLower) >= len(substrLower) && indexString(sLower, substrLower) >= 0
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// indexString finds the index of substr in s
func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// RetryableError wraps an error to mark it as retryable
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable marks an error as retryable
func IsRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// NonRetryableError wraps an error to mark it as non-retryable
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

// IsNonRetryable marks an error as non-retryable
func IsNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &NonRetryableError{Err: err}
}

// Quick retry helpers for common scenarios

// WithRetry executes an operation with default retry settings
func WithRetry(ctx context.Context, operation RetryOperation, operationName string, log logger.Logger) error {
	retryer := NewRetryer(DefaultRetryConfig(), log)
	return retryer.Do(ctx, operation, operationName)
}

// WithCustomRetry executes an operation with custom retry settings
func WithCustomRetry(ctx context.Context, operation RetryOperation, operationName string, config *RetryConfig, log logger.Logger) error {
	retryer := NewRetryer(config, log)
	return retryer.Do(ctx, operation, operationName)
}
