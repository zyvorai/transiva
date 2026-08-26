// SPDX-License-Identifier: Apache-2.0

package store

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
)

func TestSQLiteStore_SaveAndGetJob(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Create test job
	now := time.Now()
	job := &models.Job{
		Definition: models.JobDefinition{
			ID:         "test-job-1",
			Name:       "Test Job",
			VMPath:     "/data/vm/test",
			OutputPath: "/tmp/test",
			CreatedAt:  now,
		},
		Status: models.JobStatusPending,
	}

	// Save job
	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Retrieve job
	retrieved, err := store.GetJob("test-job-1")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	// Verify
	if retrieved.Definition.ID != job.Definition.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.Definition.ID, job.Definition.ID)
	}
	if retrieved.Definition.Name != job.Definition.Name {
		t.Errorf("Name mismatch: got %s, want %s", retrieved.Definition.Name, job.Definition.Name)
	}
	if retrieved.Status != job.Status {
		t.Errorf("Status mismatch: got %s, want %s", retrieved.Status, job.Status)
	}
}

func TestSQLiteStore_UpdateJob(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Create and save job
	now := time.Now()
	job := &models.Job{
		Definition: models.JobDefinition{
			ID:         "test-job-2",
			Name:       "Test Job 2",
			VMPath:     "/data/vm/test2",
			OutputPath: "/tmp/test2",
			CreatedAt:  now,
		},
		Status: models.JobStatusPending,
	}

	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Update job
	startTime := time.Now()
	job.Status = models.JobStatusRunning
	job.StartedAt = &startTime
	job.Progress = &models.JobProgress{
		Phase:           "exporting",
		PercentComplete: 50.0,
	}

	if err := store.UpdateJob(job); err != nil {
		t.Fatalf("Failed to update job: %v", err)
	}

	// Retrieve updated job
	retrieved, err := store.GetJob("test-job-2")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	// Verify updates
	if retrieved.Status != models.JobStatusRunning {
		t.Errorf("Status not updated: got %s, want %s", retrieved.Status, models.JobStatusRunning)
	}
	if retrieved.Progress == nil {
		t.Error("Progress not saved")
	} else if retrieved.Progress.PercentComplete != 50.0 {
		t.Errorf("Progress not updated: got %.1f, want 50.0", retrieved.Progress.PercentComplete)
	}
}

func TestSQLiteStore_ListJobs(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Create multiple jobs
	jobs := []*models.Job{
		{
			Definition: models.JobDefinition{
				ID:         "job-1",
				Name:       "Job 1",
				VMPath:     "/vm1",
				OutputPath: "/out1",
				CreatedAt:  time.Now().Add(-2 * time.Hour),
			},
			Status: models.JobStatusCompleted,
		},
		{
			Definition: models.JobDefinition{
				ID:         "job-2",
				Name:       "Job 2",
				VMPath:     "/vm2",
				OutputPath: "/out2",
				CreatedAt:  time.Now().Add(-1 * time.Hour),
			},
			Status: models.JobStatusRunning,
		},
		{
			Definition: models.JobDefinition{
				ID:         "job-3",
				Name:       "Job 3",
				VMPath:     "/vm3",
				OutputPath: "/out3",
				CreatedAt:  time.Now(),
			},
			Status: models.JobStatusFailed,
		},
	}

	for _, job := range jobs {
		if err := store.SaveJob(job); err != nil {
			t.Fatalf("Failed to save job: %v", err)
		}
	}

	// Test: List all jobs
	allJobs, err := store.ListJobs(JobFilter{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	if len(allJobs) != 3 {
		t.Errorf("Expected 3 jobs, got %d", len(allJobs))
	}

	// Test: Filter by status
	runningJobs, err := store.ListJobs(JobFilter{
		Status: []models.JobStatus{models.JobStatusRunning},
	})
	if err != nil {
		t.Fatalf("Failed to list running jobs: %v", err)
	}
	if len(runningJobs) != 1 {
		t.Errorf("Expected 1 running job, got %d", len(runningJobs))
	}
	if runningJobs[0].Definition.ID != "job-2" {
		t.Errorf("Expected job-2, got %s", runningJobs[0].Definition.ID)
	}

	// Test: Limit
	limitedJobs, err := store.ListJobs(JobFilter{Limit: 2})
	if err != nil {
		t.Fatalf("Failed to list with limit: %v", err)
	}
	if len(limitedJobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(limitedJobs))
	}
}

func TestSQLiteStore_DeleteJob(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Create and save job
	job := &models.Job{
		Definition: models.JobDefinition{
			ID:         "delete-test",
			Name:       "Delete Test",
			VMPath:     "/vm",
			OutputPath: "/out",
			CreatedAt:  time.Now(),
		},
		Status: models.JobStatusCompleted,
	}

	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Delete job
	if err := store.DeleteJob("delete-test"); err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}

	// Verify deletion
	_, err = store.GetJob("delete-test")
	if err == nil {
		t.Error("Expected error when getting deleted job, got nil")
	}
}

func TestSQLiteStore_GetStatistics(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Create jobs with different statuses
	statuses := []models.JobStatus{
		models.JobStatusCompleted,
		models.JobStatusCompleted,
		models.JobStatusFailed,
		models.JobStatusRunning,
		models.JobStatusPending,
	}

	for i, status := range statuses {
		job := &models.Job{
			Definition: models.JobDefinition{
				ID:         fmt.Sprintf("stats-job-%d", i),
				Name:       fmt.Sprintf("Stats Job %d", i),
				VMPath:     "/vm",
				OutputPath: "/out",
				CreatedAt:  time.Now(),
			},
			Status: status,
		}
		if err := store.SaveJob(job); err != nil {
			t.Fatalf("Failed to save job: %v", err)
		}
	}

	// Get statistics
	stats, err := store.GetStatistics()
	if err != nil {
		t.Fatalf("Failed to get statistics: %v", err)
	}

	// Verify
	if stats.Total != 5 {
		t.Errorf("Expected 5 total jobs, got %d", stats.Total)
	}
	if stats.Completed != 2 {
		t.Errorf("Expected 2 completed jobs, got %d", stats.Completed)
	}
	if stats.Failed != 1 {
		t.Errorf("Expected 1 failed job, got %d", stats.Failed)
	}
	if stats.Running != 1 {
		t.Errorf("Expected 1 running job, got %d", stats.Running)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected 1 pending job, got %d", stats.Pending)
	}
}
func TestSQLiteStore_SaveAndGetSchedule(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "test-schedule-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Create test scheduled job
	now := time.Now()
	nextRun := now.Add(1 * time.Hour)
	schedule := &models.ScheduledJob{
		ID:          "sched-1",
		Name:        "Daily Backup",
		Description: "Daily backup of VMs",
		Schedule:    "0 2 * * *",
		JobTemplate: models.JobDefinition{
			Name:   "backup-job",
			VMPath: "/vm/production",
		},
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
		NextRun:   nextRun,
		RunCount:  0,
		Tags:      []string{"backup", "daily"},
	}

	// Save schedule
	if err := store.SaveSchedule(schedule); err != nil {
		t.Fatalf("Failed to save schedule: %v", err)
	}

	// Retrieve schedule
	retrieved, err := store.GetSchedule("sched-1")
	if err != nil {
		t.Fatalf("Failed to get schedule: %v", err)
	}

	// Verify
	if retrieved.ID != schedule.ID {
		t.Errorf("Expected ID %s, got %s", schedule.ID, retrieved.ID)
	}
	if retrieved.Name != schedule.Name {
		t.Errorf("Expected name %s, got %s", schedule.Name, retrieved.Name)
	}
	if retrieved.Schedule != schedule.Schedule {
		t.Errorf("Expected schedule %s, got %s", schedule.Schedule, retrieved.Schedule)
	}
	if !retrieved.Enabled {
		t.Error("Expected enabled to be true")
	}
	if len(retrieved.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(retrieved.Tags))
	}
}

func TestSQLiteStore_UpdateSchedule(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-schedule-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	now := time.Now()
	schedule := &models.ScheduledJob{
		ID:       "sched-update",
		Name:     "Original Name",
		Schedule: "0 1 * * *",
		JobTemplate: models.JobDefinition{
			Name:   "test-job",
			VMPath: "/vm/test",
		},
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
		RunCount:  0,
	}

	// Save initial schedule
	if err := store.SaveSchedule(schedule); err != nil {
		t.Fatalf("Failed to save schedule: %v", err)
	}

	// Update schedule
	schedule.Name = "Updated Name"
	schedule.Schedule = "0 2 * * *"
	schedule.Enabled = false
	schedule.RunCount = 5
	schedule.UpdatedAt = time.Now()

	if err := store.UpdateSchedule(schedule); err != nil {
		t.Fatalf("Failed to update schedule: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetSchedule("sched-update")
	if err != nil {
		t.Fatalf("Failed to get schedule: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Expected updated name, got %s", retrieved.Name)
	}
	if retrieved.Schedule != "0 2 * * *" {
		t.Errorf("Expected updated schedule, got %s", retrieved.Schedule)
	}
	if retrieved.Enabled {
		t.Error("Expected enabled to be false")
	}
	if retrieved.RunCount != 5 {
		t.Errorf("Expected run count 5, got %d", retrieved.RunCount)
	}
}

func TestSQLiteStore_ListSchedules(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-schedule-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	now := time.Now()

	// Create multiple schedules
	schedules := []*models.ScheduledJob{
		{
			ID:       "sched-1",
			Name:     "Schedule 1",
			Schedule: "0 1 * * *",
			JobTemplate: models.JobDefinition{
				Name:   "job-1",
				VMPath: "/vm/1",
			},
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:       "sched-2",
			Name:     "Schedule 2",
			Schedule: "0 2 * * *",
			JobTemplate: models.JobDefinition{
				Name:   "job-2",
				VMPath: "/vm/2",
			},
			Enabled:   false,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:       "sched-3",
			Name:     "Schedule 3",
			Schedule: "0 3 * * *",
			JobTemplate: models.JobDefinition{
				Name:   "job-3",
				VMPath: "/vm/3",
			},
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	for _, sched := range schedules {
		if err := store.SaveSchedule(sched); err != nil {
			t.Fatalf("Failed to save schedule: %v", err)
		}
	}

	// List all schedules
	all, err := store.ListSchedules(nil)
	if err != nil {
		t.Fatalf("Failed to list schedules: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 schedules, got %d", len(all))
	}

	// List enabled schedules
	enabled := true
	enabledList, err := store.ListSchedules(&enabled)
	if err != nil {
		t.Fatalf("Failed to list enabled schedules: %v", err)
	}
	if len(enabledList) != 2 {
		t.Errorf("Expected 2 enabled schedules, got %d", len(enabledList))
	}

	// List disabled schedules
	disabled := false
	disabledList, err := store.ListSchedules(&disabled)
	if err != nil {
		t.Fatalf("Failed to list disabled schedules: %v", err)
	}
	if len(disabledList) != 1 {
		t.Errorf("Expected 1 disabled schedule, got %d", len(disabledList))
	}
}

func TestSQLiteStore_DeleteSchedule(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-schedule-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	now := time.Now()
	schedule := &models.ScheduledJob{
		ID:       "sched-delete",
		Name:     "To Delete",
		Schedule: "0 1 * * *",
		JobTemplate: models.JobDefinition{
			Name:   "job",
			VMPath: "/vm",
		},
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Save schedule
	if err := store.SaveSchedule(schedule); err != nil {
		t.Fatalf("Failed to save schedule: %v", err)
	}

	// Delete schedule
	if err := store.DeleteSchedule("sched-delete"); err != nil {
		t.Fatalf("Failed to delete schedule: %v", err)
	}

	// Verify deletion
	_, err = store.GetSchedule("sched-delete")
	if err == nil {
		t.Error("Expected error when getting deleted schedule")
	}
}

func TestSQLiteStore_RecordAndGetExecutionHistory(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-schedule-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	scheduleID := "sched-exec"

	// Record executions
	executions := []*ScheduleExecution{
		{
			ScheduleID:      scheduleID,
			JobID:           "job-1",
			ExecutedAt:      time.Now(),
			Status:          "completed",
			DurationSeconds: 120.5,
		},
		{
			ScheduleID:      scheduleID,
			JobID:           "job-2",
			ExecutedAt:      time.Now().Add(-1 * time.Hour),
			Status:          "failed",
			DurationSeconds: 30.2,
			Error:           "disk space error",
		},
		{
			ScheduleID:      scheduleID,
			JobID:           "job-3",
			ExecutedAt:      time.Now().Add(-2 * time.Hour),
			Status:          "completed",
			DurationSeconds: 90.0,
		},
	}

	for _, exec := range executions {
		if err := store.RecordExecution(exec); err != nil {
			t.Fatalf("Failed to record execution: %v", err)
		}
	}

	// Get execution history
	history, err := store.GetExecutionHistory(scheduleID, 10)
	if err != nil {
		t.Fatalf("Failed to get execution history: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 executions, got %d", len(history))
	}

	// Verify first execution (should be most recent)
	if history[0].Status != "completed" {
		t.Errorf("Expected status completed, got %s", history[0].Status)
	}

	// Verify failed execution
	found := false
	for _, h := range history {
		if h.Status == "failed" {
			found = true
			if h.Error != "disk space error" {
				t.Errorf("Expected error message, got %s", h.Error)
			}
		}
	}
	if !found {
		t.Error("Failed execution not found in history")
	}

	// Test limit
	limited, err := store.GetExecutionHistory(scheduleID, 2)
	if err != nil {
		t.Fatalf("Failed to get limited history: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 executions with limit, got %d", len(limited))
	}
}

func TestSQLiteStore_GetJobHistory(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-history-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	now := time.Now()
	job := &models.Job{
		Definition: models.JobDefinition{
			ID:        "job-history-test",
			Name:      "History Test Job",
			VMPath:    "/vm/test",
			CreatedAt: now,
		},
		Status: models.JobStatusPending,
	}

	// Save job
	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Update job status multiple times to create history
	statuses := []models.JobStatus{
		models.JobStatusRunning,
		models.JobStatusCompleted,
	}

	for _, status := range statuses {
		job.Status = status
		if err := store.UpdateJob(job); err != nil {
			t.Fatalf("Failed to update job: %v", err)
		}
	}

	// Get job history
	history, err := store.GetJobHistory("job-history-test")
	if err != nil {
		t.Fatalf("Failed to get job history: %v", err)
	}

	// Verify we have history entries
	if len(history) == 0 {
		t.Error("Expected history entries, got none")
	}

	// Verify history is ordered by timestamp
	for i := 1; i < len(history); i++ {
		if history[i].Timestamp.Before(history[i-1].Timestamp) {
			t.Error("History entries not ordered by timestamp")
		}
	}

	// Verify status transitions are recorded
	statusFound := false
	for _, entry := range history {
		if entry.Status == models.JobStatusRunning ||
			entry.Status == models.JobStatusCompleted {
			statusFound = true
			break
		}
	}
	if !statusFound {
		t.Error("Expected status transitions in history")
	}
}

func TestNewSQLiteStore_InvalidPath(t *testing.T) {
	// Try to create a store at an invalid path (directory that doesn't exist)
	invalidPath := "/nonexistent/directory/that/does/not/exist/test.db"
	_, err := NewSQLiteStore(invalidPath)
	
	if err == nil {
		t.Error("Expected error when creating store with invalid path, got nil")
	}
}

func TestSQLiteStore_CloseNilDB(t *testing.T) {
	// Create a store with nil db (simulate closed or invalid state)
	store := &SQLiteStore{db: nil}
	
	// Should not panic or error when db is nil
	err := store.Close()
	if err != nil {
		t.Errorf("Expected no error when closing with nil db, got: %v", err)
	}
}

func TestSQLiteStore_DoubleClose(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Close once
	err = store.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}

	// Close again - should handle gracefully
	// SQLite might return an error on double close, which is acceptable;
	// we just verify it doesn't panic, so the error is intentionally discarded.
	_ = store.Close()
}

func TestSQLiteStore_UpdateJobNotFound(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Try to update non-existent job
	job := &models.Job{
		Definition: models.JobDefinition{
			ID:     "nonexistent-job",
			VMPath: "/datacenter/vm/test",
		},
		Status: models.JobStatusPending,
	}

	err = store.UpdateJob(job)
	if err == nil {
		t.Error("Expected error when updating non-existent job")
	}
}

func TestSQLiteStore_GetJobNotFound(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Try to get non-existent job
	_, err = store.GetJob("nonexistent-job")
	if err == nil {
		t.Error("Expected error when getting non-existent job")
	}
}

func TestSQLiteStore_DeleteJobNotFound(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Try to delete non-existent job
	err = store.DeleteJob("nonexistent-job")
	if err == nil {
		t.Error("Expected error when deleting non-existent job")
	}
}

func TestSQLiteStore_UpdateScheduleNotFound(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Try to update non-existent schedule
	schedule := &models.ScheduledJob{
		ID:          "nonexistent-schedule",
		Schedule:    "0 0 * * *",
		JobTemplate: models.JobDefinition{VMPath: "/vm/test"},
	}

	err = store.UpdateSchedule(schedule)
	if err == nil {
		t.Error("Expected error when updating non-existent schedule")
	}
}

func TestSQLiteStore_GetScheduleNotFound(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Try to get non-existent schedule
	_, err = store.GetSchedule("nonexistent-schedule")
	if err == nil {
		t.Error("Expected error when getting non-existent schedule")
	}
}

func TestSQLiteStore_DeleteScheduleNotFound(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Try to delete non-existent schedule
	err = store.DeleteSchedule("nonexistent-schedule")
	if err == nil {
		t.Error("Expected error when deleting non-existent schedule")
	}
}

func TestSQLiteStore_ListJobsWithFilter(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Save multiple jobs with different statuses
	jobs := []*models.Job{
		{
			Definition: models.JobDefinition{
				ID:     "job-completed",
				VMPath: "/datacenter/vm/vm1",
			},
			Status: models.JobStatusCompleted,
		},
		{
			Definition: models.JobDefinition{
				ID:     "job-running",
				VMPath: "/datacenter/vm/vm2",
			},
			Status: models.JobStatusRunning,
		},
		{
			Definition: models.JobDefinition{
				ID:     "job-failed",
				VMPath: "/datacenter/vm/vm3",
			},
			Status: models.JobStatusFailed,
		},
	}

	for _, job := range jobs {
		if err := store.SaveJob(job); err != nil {
			t.Fatalf("Failed to save job %s: %v", job.Definition.ID, err)
		}
	}

	// List jobs with status filter
	filter := JobFilter{Status: []models.JobStatus{models.JobStatusCompleted}}
	listed, err := store.ListJobs(filter)
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(listed) != 1 {
		t.Errorf("Expected 1 completed job, got %d", len(listed))
	}

	if len(listed) > 0 && listed[0].Definition.ID != "job-completed" {
		t.Errorf("Expected job-completed, got %s", listed[0].Definition.ID)
	}
}

func TestSQLiteStore_ListSchedulesInactive(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "test-db-*.sqlite")
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp db file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("failed to remove temp db file: %v", err)
		}
	}()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close store: %v", err)
		}
	}()

	// Save active and inactive schedules
	now := time.Now()
	activeSchedule := &models.ScheduledJob{
		ID:          "active-schedule",
		Name:        "Active Job",
		Schedule:    "0 0 * * *",
		JobTemplate: models.JobDefinition{VMPath: "/vm/1"},
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
		NextRun:     now.Add(1 * time.Hour),
	}

	inactiveSchedule := &models.ScheduledJob{
		ID:          "inactive-schedule",
		Name:        "Inactive Job",
		Schedule:    "0 0 * * *",
		JobTemplate: models.JobDefinition{VMPath: "/vm/2"},
		Enabled:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
		NextRun:     now.Add(1 * time.Hour),
	}

	if err := store.SaveSchedule(activeSchedule); err != nil {
		t.Fatalf("Failed to save active schedule: %v", err)
	}

	if err := store.SaveSchedule(inactiveSchedule); err != nil {
		t.Fatalf("Failed to save inactive schedule: %v", err)
	}

	// List enabled schedules
	enabled := true
	schedules, err := store.ListSchedules(&enabled)
	if err != nil {
		t.Fatalf("Failed to list schedules: %v", err)
	}

	if len(schedules) != 1 {
		t.Errorf("Expected 1 active schedule, got %d", len(schedules))
	}

	if len(schedules) > 0 && schedules[0].ID != "active-schedule" {
		t.Errorf("Expected active-schedule, got %s", schedules[0].ID)
	}
}

