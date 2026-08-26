// SPDX-License-Identifier: Apache-2.0

package models

import (
	"time"
)

// ScheduledJob represents a job scheduled for recurring execution
type ScheduledJob struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Schedule    string        `json:"schedule"` // Cron format
	JobTemplate JobDefinition `json:"job_template"`
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	NextRun     time.Time     `json:"next_run"`
	LastRun     *time.Time    `json:"last_run,omitempty"`
	RunCount    int           `json:"run_count"`
	Tags        []string      `json:"tags,omitempty"`

	// Advanced Scheduling (optional)
	AdvancedConfig *AdvancedScheduleConfig `json:"advanced_config,omitempty"`
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

// RetryPolicy defines how a job should be retried on failure
type RetryPolicy struct {
	MaxAttempts     int      `json:"max_attempts"`      // Maximum retry attempts (0 = no retry)
	InitialDelay    int      `json:"initial_delay"`     // Initial delay in seconds
	MaxDelay        int      `json:"max_delay"`         // Maximum delay in seconds
	BackoffStrategy string   `json:"backoff_strategy"`  // linear, exponential, fibonacci
	RetryOnErrors   []string `json:"retry_on_errors"`   // Only retry on specific errors
}

// TimeWindow defines when a job is allowed to run
type TimeWindow struct {
	StartTime string   `json:"start_time"` // HH:MM format
	EndTime   string   `json:"end_time"`   // HH:MM format
	Days      []string `json:"days"`       // Mon, Tue, Wed, Thu, Fri, Sat, Sun
	Timezone  string   `json:"timezone"`   // IANA timezone
}

// JobCondition defines a condition that must be met for job execution
type JobCondition struct {
	Type     string                 `json:"type"`     // disk_space, time_of_day, custom
	Operator string                 `json:"operator"` // >, <, ==, !=
	Value    interface{}            `json:"value"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// JobDependency represents a dependency on another job
type JobDependency struct {
	JobID         string `json:"job_id"`          // ID of the job this depends on
	RequiredState string `json:"required_state"`  // completed, failed, any
	Timeout       int    `json:"timeout"`         // Max wait time in seconds
}
