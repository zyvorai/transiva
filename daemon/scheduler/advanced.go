// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"fmt"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
)

// RetryPolicy defines how a job should be retried on failure
type RetryPolicy struct {
	MaxAttempts     int           `json:"max_attempts"`      // Maximum retry attempts (0 = no retry)
	InitialDelay    time.Duration `json:"initial_delay"`     // Initial delay before first retry
	MaxDelay        time.Duration `json:"max_delay"`         // Maximum delay between retries
	BackoffStrategy string        `json:"backoff_strategy"`  // linear, exponential, fibonacci
	RetryOnErrors   []string      `json:"retry_on_errors"`   // Only retry on specific errors (empty = all)
}

// TimeWindow defines when a job is allowed to run
type TimeWindow struct {
	StartTime string   `json:"start_time"` // HH:MM format (e.g., "09:00")
	EndTime   string   `json:"end_time"`   // HH:MM format (e.g., "17:00")
	Days      []string `json:"days"`       // Mon, Tue, Wed, Thu, Fri, Sat, Sun
	Timezone  string   `json:"timezone"`   // IANA timezone (e.g., "America/New_York")
}

// JobCondition defines a condition that must be met for job execution
type JobCondition struct {
	Type     string                 `json:"type"`     // disk_space, time_of_day, custom
	Operator string                 `json:"operator"` // >, <, ==, !=, contains
	Value    interface{}            `json:"value"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// JobDependency represents a dependency on another job
type JobDependency struct {
	JobID         string   `json:"job_id"`          // ID of the job this depends on
	RequiredState string   `json:"required_state"`  // completed, failed, any
	Timeout       int      `json:"timeout"`         // Max wait time in seconds (0 = no timeout)
}

// AdvancedScheduleConfig extends the basic schedule with advanced features
type AdvancedScheduleConfig struct {
	// Job Dependencies
	DependsOn []JobDependency `json:"depends_on,omitempty"`

	// Retry Configuration
	RetryPolicy *RetryPolicy `json:"retry_policy,omitempty"`

	// Time Windows
	TimeWindows []TimeWindow `json:"time_windows,omitempty"`

	// Job Priority (0 = lowest, 100 = highest)
	Priority int `json:"priority"`

	// Execution Conditions
	Conditions []JobCondition `json:"conditions,omitempty"`

	// Concurrency Control
	MaxConcurrent int  `json:"max_concurrent"` // Max concurrent runs (0 = unlimited)
	SkipIfRunning bool `json:"skip_if_running"` // Skip if already running

	// Notifications
	NotifyOnStart   bool `json:"notify_on_start"`
	NotifyOnSuccess bool `json:"notify_on_success"`
	NotifyOnFailure bool `json:"notify_on_failure"`
	NotifyOnRetry   bool `json:"notify_on_retry"`
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Minute,
		MaxDelay:        30 * time.Minute,
		BackoffStrategy: "exponential",
		RetryOnErrors:   []string{},
	}
}

// DefaultTimeWindow returns a business hours time window
func DefaultTimeWindow() TimeWindow {
	return TimeWindow{
		StartTime: "09:00",
		EndTime:   "17:00",
		Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
		Timezone:  "UTC",
	}
}

// CalculateBackoff calculates the delay for a retry attempt
func (rp *RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	if attempt >= rp.MaxAttempts {
		return 0
	}

	var delay time.Duration
	switch rp.BackoffStrategy {
	case "linear":
		delay = rp.InitialDelay * time.Duration(attempt+1)
	case "exponential":
		delay = rp.InitialDelay * time.Duration(1<<uint(attempt))
	case "fibonacci":
		delay = rp.InitialDelay * time.Duration(fibonacci(attempt+1))
	default:
		delay = rp.InitialDelay
	}

	if delay > rp.MaxDelay {
		delay = rp.MaxDelay
	}

	return delay
}

// IsInTimeWindow checks if the current time is within the time window
func (tw *TimeWindow) IsInTimeWindow(t time.Time) (bool, error) {
	// Load timezone
	loc, err := time.LoadLocation(tw.Timezone)
	if err != nil {
		return false, fmt.Errorf("invalid timezone: %w", err)
	}

	// Convert to window timezone
	localTime := t.In(loc)

	// Check day of week
	dayMatch := false
	currentDay := localTime.Weekday().String()[:3] // Mon, Tue, etc.
	for _, day := range tw.Days {
		if day == currentDay {
			dayMatch = true
			break
		}
	}
	if !dayMatch {
		return false, nil
	}

	// Parse start and end times
	startTime, err := time.Parse("15:04", tw.StartTime)
	if err != nil {
		return false, fmt.Errorf("invalid start_time format: %w", err)
	}

	endTime, err := time.Parse("15:04", tw.EndTime)
	if err != nil {
		return false, fmt.Errorf("invalid end_time format: %w", err)
	}

	// Create time bounds for today
	year, month, day := localTime.Date()
	start := time.Date(year, month, day, startTime.Hour(), startTime.Minute(), 0, 0, loc)
	end := time.Date(year, month, day, endTime.Hour(), endTime.Minute(), 0, 0, loc)

	// Check if current time is within window
	return localTime.After(start) && localTime.Before(end), nil
}

// EvaluateCondition evaluates a job condition
func (jc *JobCondition) EvaluateCondition() (bool, error) {
	switch jc.Type {
	case "time_of_day":
		return evaluateTimeOfDay(jc)
	case "disk_space":
		return evaluateDiskSpace(jc)
	case "custom":
		return evaluateCustomCondition(jc)
	default:
		return false, fmt.Errorf("unknown condition type: %s", jc.Type)
	}
}

// Helper functions

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func evaluateTimeOfDay(jc *JobCondition) (bool, error) {
	now := time.Now()
	hour := now.Hour()

	switch jc.Operator {
	case ">":
		targetHour, ok := jc.Value.(float64)
		if !ok {
			return false, fmt.Errorf("invalid value type for time_of_day")
		}
		return hour > int(targetHour), nil
	case "<":
		targetHour, ok := jc.Value.(float64)
		if !ok {
			return false, fmt.Errorf("invalid value type for time_of_day")
		}
		return hour < int(targetHour), nil
	default:
		return false, fmt.Errorf("unsupported operator for time_of_day: %s", jc.Operator)
	}
}

func evaluateDiskSpace(jc *JobCondition) (bool, error) {
	// TODO: Implement disk space checking
	// For now, always return true
	return true, nil
}

func evaluateCustomCondition(jc *JobCondition) (bool, error) {
	// Custom conditions can be implemented by users
	// For now, always return true
	return true, nil
}

// JobQueue manages job execution with priorities
type JobQueue struct {
	jobs     []*QueuedJob
	running  map[string]bool
	maxSlots int
}

// QueuedJob represents a job in the execution queue
type QueuedJob struct {
	Job      *models.ScheduledJob
	Config   *AdvancedScheduleConfig
	AddedAt  time.Time
	Attempts int
}

// NewJobQueue creates a new job queue
func NewJobQueue(maxSlots int) *JobQueue {
	return &JobQueue{
		jobs:     make([]*QueuedJob, 0),
		running:  make(map[string]bool),
		maxSlots: maxSlots,
	}
}

// Add adds a job to the queue with priority
func (jq *JobQueue) Add(job *models.ScheduledJob, config *AdvancedScheduleConfig) {
	qj := &QueuedJob{
		Job:     job,
		Config:  config,
		AddedAt: time.Now(),
	}

	// Insert in priority order
	inserted := false
	for i, existing := range jq.jobs {
		if config.Priority > existing.Config.Priority {
			// Insert before this job
			jq.jobs = append(jq.jobs[:i], append([]*QueuedJob{qj}, jq.jobs[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		jq.jobs = append(jq.jobs, qj)
	}
}

// GetNext returns the next job to execute (highest priority)
func (jq *JobQueue) GetNext() *QueuedJob {
	if len(jq.jobs) == 0 {
		return nil
	}

	// Check if we have available slots
	if len(jq.running) >= jq.maxSlots {
		return nil
	}

	// Get highest priority job
	job := jq.jobs[0]
	jq.jobs = jq.jobs[1:]

	// Mark as running
	jq.running[job.Job.ID] = true

	return job
}

// Complete marks a job as completed
func (jq *JobQueue) Complete(jobID string) {
	delete(jq.running, jobID)
}

// Size returns the number of jobs in the queue
func (jq *JobQueue) Size() int {
	return len(jq.jobs)
}

// IsRunning checks if a job is currently running
func (jq *JobQueue) IsRunning(jobID string) bool {
	return jq.running[jobID]
}
