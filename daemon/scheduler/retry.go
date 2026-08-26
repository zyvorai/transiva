// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// RetryManager manages job retries with configurable policies
type RetryManager struct {
	mu            sync.RWMutex
	retryAttempts map[string]*RetryAttempt // jobID -> retry attempt info
	log           logger.Logger
	notifier      RetryNotifier
}

// RetryAttempt tracks retry information for a job
type RetryAttempt struct {
	JobID         string
	JobName       string
	Attempt       int
	MaxAttempts   int
	LastError     string
	NextRetry     time.Time
	RetryHistory  []RetryRecord
}

// RetryRecord represents a single retry attempt
type RetryRecord struct {
	Attempt   int
	Timestamp time.Time
	Error     string
	Delay     time.Duration
}

// RetryNotifier sends notifications about retry attempts
type RetryNotifier interface {
	NotifyRetry(jobID, jobName string, attempt, maxAttempts int, nextRetry time.Time)
}

// NewRetryManager creates a new retry manager
func NewRetryManager(log logger.Logger) *RetryManager {
	return &RetryManager{
		retryAttempts: make(map[string]*RetryAttempt),
		log:           log,
	}
}

// SetNotifier sets the retry notifier
func (rm *RetryManager) SetNotifier(notifier RetryNotifier) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.notifier = notifier
}

// ShouldRetry determines if a job should be retried
func (rm *RetryManager) ShouldRetry(job *models.ScheduledJob, err error) bool {
	if job.AdvancedConfig == nil || job.AdvancedConfig.RetryPolicy == nil {
		return false
	}

	policy := job.AdvancedConfig.RetryPolicy

	// Check if we've exceeded max attempts
	rm.mu.RLock()
	attempt, exists := rm.retryAttempts[job.ID]
	rm.mu.RUnlock()

	currentAttempt := 0
	if exists {
		currentAttempt = attempt.Attempt
	}

	if currentAttempt >= policy.MaxAttempts {
		rm.log.Info("max retry attempts reached",
			"job", job.Name,
			"attempts", currentAttempt,
			"max", policy.MaxAttempts)
		return false
	}

	// Check if error matches retry conditions
	if len(policy.RetryOnErrors) > 0 {
		errorStr := err.Error()
		matchFound := false
		for _, pattern := range policy.RetryOnErrors {
			if strings.Contains(errorStr, pattern) {
				matchFound = true
				break
			}
		}
		if !matchFound {
			rm.log.Info("error does not match retry conditions",
				"job", job.Name,
				"error", errorStr)
			return false
		}
	}

	return true
}

// ScheduleRetry schedules a retry for a failed job
func (rm *RetryManager) ScheduleRetry(ctx context.Context, job *models.ScheduledJob, err error, executor JobExecutor) error {
	if !rm.ShouldRetry(job, err) {
		return fmt.Errorf("retry not allowed for job %s", job.Name)
	}

	policy := job.AdvancedConfig.RetryPolicy

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Get or create retry attempt
	attempt, exists := rm.retryAttempts[job.ID]
	if !exists {
		attempt = &RetryAttempt{
			JobID:        job.ID,
			JobName:      job.Name,
			Attempt:      0,
			MaxAttempts:  policy.MaxAttempts,
			RetryHistory: make([]RetryRecord, 0),
		}
		rm.retryAttempts[job.ID] = attempt
	}

	// Increment attempt counter
	attempt.Attempt++
	attempt.LastError = err.Error()

	// Calculate backoff delay
	delay := rm.calculateBackoff(policy, attempt.Attempt)
	nextRetry := time.Now().Add(delay)
	attempt.NextRetry = nextRetry

	// Record this retry
	record := RetryRecord{
		Attempt:   attempt.Attempt,
		Timestamp: time.Now(),
		Error:     err.Error(),
		Delay:     delay,
	}
	attempt.RetryHistory = append(attempt.RetryHistory, record)

	rm.log.Info("scheduling retry",
		"job", job.Name,
		"attempt", attempt.Attempt,
		"max_attempts", attempt.MaxAttempts,
		"delay", delay,
		"next_retry", nextRetry.Format(time.RFC3339))

	// Notify about retry
	if rm.notifier != nil {
		go rm.notifier.NotifyRetry(job.ID, job.Name, attempt.Attempt, attempt.MaxAttempts, nextRetry)
	}

	// Schedule the retry
	go rm.executeRetry(ctx, job, executor, delay)

	return nil
}

// executeRetry executes a retry after the specified delay
func (rm *RetryManager) executeRetry(ctx context.Context, job *models.ScheduledJob, executor JobExecutor, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		rm.log.Info("retry cancelled", "job", job.Name)
		return
	case <-timer.C:
		rm.log.Info("executing retry", "job", job.Name)

		// Execute the job
		_, err := executor.SubmitJob(job.JobTemplate)
		if err != nil {
			rm.log.Error("retry failed",
				"job", job.Name,
				"error", err)

			// Check if we should retry again
			if rm.ShouldRetry(job, err) {
				if scheduleErr := rm.ScheduleRetry(ctx, job, err, executor); scheduleErr != nil {
					rm.log.Error("failed to schedule retry", "job", job.Name, "error", scheduleErr)
				}
			} else {
				rm.log.Info("no more retries for job", "job", job.Name)
				rm.ClearRetry(job.ID)
			}
		} else {
			rm.log.Info("retry succeeded", "job", job.Name)
			rm.ClearRetry(job.ID)
		}
	}
}

// calculateBackoff calculates the backoff delay for a retry attempt
func (rm *RetryManager) calculateBackoff(policy *models.RetryPolicy, attempt int) time.Duration {
	initialDelay := time.Duration(policy.InitialDelay) * time.Second
	maxDelay := time.Duration(policy.MaxDelay) * time.Second

	var delay time.Duration
	switch policy.BackoffStrategy {
	case "linear":
		delay = initialDelay * time.Duration(attempt)
	case "exponential":
		delay = initialDelay * time.Duration(1<<uint(attempt))
	case "fibonacci":
		delay = initialDelay * time.Duration(fibonacci(attempt+1))
	default:
		delay = initialDelay
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// fibonacci is defined in advanced.go to avoid duplication

// GetRetryAttempt returns the retry attempt for a job
func (rm *RetryManager) GetRetryAttempt(jobID string) (*RetryAttempt, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	attempt, exists := rm.retryAttempts[jobID]
	return attempt, exists
}

// ClearRetry removes retry information for a job
func (rm *RetryManager) ClearRetry(jobID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.retryAttempts, jobID)
	rm.log.Debug("cleared retry info", "job", jobID)
}

// GetAllRetries returns all active retry attempts
func (rm *RetryManager) GetAllRetries() map[string]*RetryAttempt {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[string]*RetryAttempt)
	for k, v := range rm.retryAttempts {
		result[k] = v
	}
	return result
}

// ClearOldRetries removes retry information older than the specified duration
func (rm *RetryManager) ClearOldRetries(maxAge time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for jobID, attempt := range rm.retryAttempts {
		if len(attempt.RetryHistory) > 0 {
			lastRetry := attempt.RetryHistory[len(attempt.RetryHistory)-1]
			if lastRetry.Timestamp.Before(cutoff) {
				delete(rm.retryAttempts, jobID)
				rm.log.Debug("cleared old retry info", "job", jobID)
			}
		}
	}
}

// GetRetryStats returns statistics about retry attempts
func (rm *RetryManager) GetRetryStats() RetryStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := RetryStats{
		TotalRetries:  0,
		SuccessRetries: 0,
		FailedRetries:  0,
		AvgAttempts:    0,
	}

	totalAttempts := 0
	for _, attempt := range rm.retryAttempts {
		stats.TotalRetries++
		totalAttempts += attempt.Attempt

		if attempt.Attempt >= attempt.MaxAttempts {
			stats.FailedRetries++
		}
	}

	if stats.TotalRetries > 0 {
		stats.AvgAttempts = float64(totalAttempts) / float64(stats.TotalRetries)
	}

	stats.SuccessRetries = stats.TotalRetries - stats.FailedRetries

	return stats
}

// RetryStats contains statistics about retry attempts
type RetryStats struct {
	TotalRetries   int
	SuccessRetries int
	FailedRetries  int
	AvgAttempts    float64
}
