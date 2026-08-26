// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
)

func TestRetryPolicyCalculateBackoff(t *testing.T) {
	tests := []struct {
		name     string
		policy   *models.RetryPolicy
		attempt  int
		expected time.Duration
	}{
		{
			name: "Linear backoff",
			policy: &models.RetryPolicy{
				InitialDelay:    60,
				MaxDelay:        300,
				BackoffStrategy: "linear",
			},
			attempt:  2,
			expected: 120 * time.Second,
		},
		{
			name: "Exponential backoff",
			policy: &models.RetryPolicy{
				InitialDelay:    60,
				MaxDelay:        600,
				BackoffStrategy: "exponential",
			},
			attempt:  3,
			expected: 480 * time.Second,
		},
		{
			name: "Fibonacci backoff",
			policy: &models.RetryPolicy{
				InitialDelay:    60,
				MaxDelay:        1000,
				BackoffStrategy: "fibonacci",
			},
			attempt:  4,
			expected: 300 * time.Second, // fib(4) = 5 * 60
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewRetryManager(nil)
			result := rm.calculateBackoff(tt.policy, tt.attempt)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTimeWindowIsInWindow(t *testing.T) {
	window := models.TimeWindow{
		StartTime: "09:00",
		EndTime:   "17:00",
		Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
		Timezone:  "UTC",
	}

	// Test during business hours (Monday 12:00 UTC)
	testTime := time.Date(2024, 2, 5, 12, 0, 0, 0, time.UTC) // Monday
	twm := NewTimeWindowManager(nil)
	inWindow, err := twm.checkWindow(window, testTime)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !inWindow {
		t.Error("Expected to be in window during business hours")
	}

	// Test outside business hours (Monday 20:00 UTC)
	testTime = time.Date(2024, 2, 5, 20, 0, 0, 0, time.UTC)
	inWindow, err = twm.checkWindow(window, testTime)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if inWindow {
		t.Error("Expected to be outside window after hours")
	}

	// Test weekend (Saturday 12:00 UTC)
	testTime = time.Date(2024, 2, 10, 12, 0, 0, 0, time.UTC) // Saturday
	inWindow, err = twm.checkWindow(window, testTime)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if inWindow {
		t.Error("Expected to be outside window on weekend")
	}
}

func TestValidateTimeWindow(t *testing.T) {
	tests := []struct {
		name    string
		window  models.TimeWindow
		wantErr bool
	}{
		{
			name: "Valid window",
			window: models.TimeWindow{
				StartTime: "09:00",
				EndTime:   "17:00",
				Days:      []string{"Mon", "Tue"},
				Timezone:  "UTC",
			},
			wantErr: false,
		},
		{
			name: "Invalid timezone",
			window: models.TimeWindow{
				StartTime: "09:00",
				EndTime:   "17:00",
				Days:      []string{"Mon"},
				Timezone:  "Invalid/Timezone",
			},
			wantErr: true,
		},
		{
			name: "Invalid start time",
			window: models.TimeWindow{
				StartTime: "25:00",
				EndTime:   "17:00",
				Days:      []string{"Mon"},
				Timezone:  "UTC",
			},
			wantErr: true,
		},
		{
			name: "No days specified",
			window: models.TimeWindow{
				StartTime: "09:00",
				EndTime:   "17:00",
				Days:      []string{},
				Timezone:  "UTC",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeWindow(tt.window)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTimeWindow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJobQueue(t *testing.T) {
	queue := NewJobQueue(2)

	// Create test jobs with different priorities
	job1 := &models.ScheduledJob{ID: "job1", Name: "Low Priority"}
	config1 := &AdvancedScheduleConfig{Priority: 10}

	job2 := &models.ScheduledJob{ID: "job2", Name: "High Priority"}
	config2 := &AdvancedScheduleConfig{Priority: 90}

	job3 := &models.ScheduledJob{ID: "job3", Name: "Medium Priority"}
	config3 := &AdvancedScheduleConfig{Priority: 50}

	// Add jobs
	queue.Add(job1, config1)
	queue.Add(job2, config2)
	queue.Add(job3, config3)

	// Check queue size
	if queue.Size() != 3 {
		t.Errorf("Expected queue size 3, got %d", queue.Size())
	}

	// Get next job (should be highest priority)
	next := queue.GetNext()
	if next.Job.ID != "job2" {
		t.Errorf("Expected job2 (high priority), got %s", next.Job.ID)
	}

	// Check running status
	if !queue.IsRunning("job2") {
		t.Error("job2 should be marked as running")
	}

	// Complete job
	queue.Complete("job2")
	if queue.IsRunning("job2") {
		t.Error("job2 should not be running after completion")
	}
}
