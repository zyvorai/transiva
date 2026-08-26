// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/daemon/store"
	"github.com/zyvorai/transiva/logger"
)

// Mock executor for testing
type mockExecutor struct {
	submittedJobs []models.JobDefinition
}

func (m *mockExecutor) SubmitJob(def models.JobDefinition) (string, error) {
	m.submittedJobs = append(m.submittedJobs, def)
	return def.ID, nil
}

func TestScheduler_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database store
	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = db.Close() }()

	log := logger.New("debug")
	executor := &mockExecutor{}

	// Create scheduler with persistence
	sched := NewScheduler(executor, log)
	sched.SetStore(db)
	sched.Start()
	defer sched.Stop()

	// Add scheduled job
	sj := &models.ScheduledJob{
		ID:          "test-schedule",
		Name:        "Test Schedule",
		Description: "Test Description",
		Schedule:    "*/5 * * * *",
		Enabled:     true,
		JobTemplate: models.JobDefinition{
			Name:       "test-job",
			VMPath:     "/test/vm",
			OutputPath: "/test/output",
		},
		Tags: []string{"test", "backup"},
	}

	err = sched.AddScheduledJob(sj)
	if err != nil {
		t.Fatalf("Failed to add scheduled job: %v", err)
	}

	// Verify it was persisted
	retrieved, err := db.GetSchedule("test-schedule")
	if err != nil {
		t.Fatalf("Failed to retrieve schedule from DB: %v", err)
	}

	if retrieved.Name != sj.Name {
		t.Errorf("Name mismatch: expected %s, got %s", sj.Name, retrieved.Name)
	}

	if retrieved.Schedule != sj.Schedule {
		t.Errorf("Schedule mismatch: expected %s, got %s", sj.Schedule, retrieved.Schedule)
	}
}

func TestScheduler_LoadSchedules(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database store
	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	log := logger.New("debug")
	executor := &mockExecutor{}

	// Create first scheduler and add schedules
	sched1 := NewScheduler(executor, log)
	sched1.SetStore(db)
	sched1.Start()

	sj := &models.ScheduledJob{
		ID:       "persistent-schedule",
		Name:     "Persistent Schedule",
		Schedule: "0 * * * *",
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			Name: "hourly-backup",
		},
	}

	_ = sched1.AddScheduledJob(sj)
	sched1.Stop()

	// Create new scheduler and load schedules
	sched2 := NewScheduler(executor, log)
	sched2.SetStore(db)

	err = sched2.LoadSchedules()
	if err != nil {
		t.Fatalf("Failed to load schedules: %v", err)
	}

	// Verify schedule was loaded
	loaded := sched2.ListScheduledJobs()
	if len(loaded) != 1 {
		t.Fatalf("Expected 1 loaded schedule, got %d", len(loaded))
	}

	if loaded[0].ID != "persistent-schedule" {
		t.Errorf("Loaded wrong schedule ID: %s", loaded[0].ID)
	}

	_ = db.Close()
}

func TestScheduler_UpdatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = db.Close() }()

	log := logger.New("debug")
	executor := &mockExecutor{}

	sched := NewScheduler(executor, log)
	sched.SetStore(db)
	sched.Start()
	defer sched.Stop()

	// Add schedule
	sj := &models.ScheduledJob{
		ID:          "update-test",
		Name:        "Original Name",
		Schedule:    "0 * * * *",
		Enabled:     true,
		JobTemplate: models.JobDefinition{},
	}

	_ = sched.AddScheduledJob(sj)

	// Update schedule
	updates := &models.ScheduledJob{
		Name:     "Updated Name",
		Schedule: "0 2 * * *",
		Enabled:  false,
	}

	err = sched.UpdateScheduledJob("update-test", updates)
	if err != nil {
		t.Fatalf("Failed to update schedule: %v", err)
	}

	// Verify update was persisted
	retrieved, err := db.GetSchedule("update-test")
	if err != nil {
		t.Fatalf("Failed to retrieve updated schedule: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Name not updated: got %s", retrieved.Name)
	}

	if retrieved.Schedule != "0 2 * * *" {
		t.Errorf("Schedule not updated: got %s", retrieved.Schedule)
	}

	if retrieved.Enabled {
		t.Error("Schedule should be disabled")
	}
}

func TestScheduler_DeletePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = db.Close() }()

	log := logger.New("debug")
	executor := &mockExecutor{}

	sched := NewScheduler(executor, log)
	sched.SetStore(db)
	sched.Start()
	defer sched.Stop()

	// Add schedule
	sj := &models.ScheduledJob{
		ID:          "delete-test",
		Name:        "To Be Deleted",
		Schedule:    "0 * * * *",
		Enabled:     true,
		JobTemplate: models.JobDefinition{},
	}

	_ = sched.AddScheduledJob(sj)

	// Verify it exists
	_, err = db.GetSchedule("delete-test")
	if err != nil {
		t.Fatal("Schedule should exist before deletion")
	}

	// Delete schedule
	err = sched.RemoveScheduledJob("delete-test")
	if err != nil {
		t.Fatalf("Failed to remove schedule: %v", err)
	}

	// Verify it's deleted from DB
	_, err = db.GetSchedule("delete-test")
	if err == nil {
		t.Error("Schedule should not exist after deletion")
	}
}

func TestScheduler_ExecutionHistory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Record some executions
	execution1 := &store.ScheduleExecution{
		ScheduleID:      "test-schedule",
		JobID:           "job-1",
		ExecutedAt:      time.Now(),
		Status:          "completed",
		DurationSeconds: 123.45,
	}

	execution2 := &store.ScheduleExecution{
		ScheduleID:      "test-schedule",
		JobID:           "job-2",
		ExecutedAt:      time.Now().Add(1 * time.Hour),
		Status:          "failed",
		DurationSeconds: 45.67,
		Error:           "Test error",
	}

	err = db.RecordExecution(execution1)
	if err != nil {
		t.Fatalf("Failed to record execution 1: %v", err)
	}

	err = db.RecordExecution(execution2)
	if err != nil {
		t.Fatalf("Failed to record execution 2: %v", err)
	}

	// Retrieve execution history
	history, err := db.GetExecutionHistory("test-schedule", 10)
	if err != nil {
		t.Fatalf("Failed to get execution history: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("Expected 2 executions, got %d", len(history))
	}

	// Verify most recent is first (DESC order)
	if history[0].JobID != "job-2" {
		t.Errorf("Expected job-2 first (most recent), got %s", history[0].JobID)
	}

	if history[0].Status != "failed" {
		t.Errorf("Expected failed status, got %s", history[0].Status)
	}

	if history[0].Error != "Test error" {
		t.Errorf("Expected error message, got %s", history[0].Error)
	}
}

func TestScheduler_MultipleSchedules(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = db.Close() }()

	log := logger.New("debug")
	executor := &mockExecutor{}

	sched := NewScheduler(executor, log)
	sched.SetStore(db)
	sched.Start()
	defer sched.Stop()

	// Add multiple schedules
	schedules := []*models.ScheduledJob{
		{
			ID:          "schedule-1",
			Name:        "Hourly Backup",
			Schedule:    "0 * * * *",
			Enabled:     true,
			JobTemplate: models.JobDefinition{Name: "hourly"},
		},
		{
			ID:          "schedule-2",
			Name:        "Daily Backup",
			Schedule:    "0 2 * * *",
			Enabled:     true,
			JobTemplate: models.JobDefinition{Name: "daily"},
		},
		{
			ID:          "schedule-3",
			Name:        "Weekly Backup",
			Schedule:    "0 2 * * 0",
			Enabled:     false,
			JobTemplate: models.JobDefinition{Name: "weekly"},
		},
	}

	for _, sj := range schedules {
		if err := sched.AddScheduledJob(sj); err != nil {
			t.Fatalf("Failed to add schedule %s: %v", sj.ID, err)
		}
	}

	// Load into new scheduler
	sched2 := NewScheduler(executor, log)
	sched2.SetStore(db)

	err = sched2.LoadSchedules()
	if err != nil {
		t.Fatalf("Failed to load schedules: %v", err)
	}

	loaded := sched2.ListScheduledJobs()
	if len(loaded) != 3 {
		t.Fatalf("Expected 3 schedules, got %d", len(loaded))
	}

	// Count enabled schedules
	enabled := 0
	for _, sj := range loaded {
		if sj.Enabled {
			enabled++
		}
	}

	if enabled != 2 {
		t.Errorf("Expected 2 enabled schedules, got %d", enabled)
	}
}
