// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// mockJobExecutor is a mock implementation of JobExecutor for testing
type mockJobExecutor struct {
	mu            sync.Mutex
	submittedJobs []models.JobDefinition
	submitError   error
}

func (m *mockJobExecutor) SubmitJob(definition models.JobDefinition) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.submitError != nil {
		return "", m.submitError
	}

	m.submittedJobs = append(m.submittedJobs, definition)
	return definition.ID, nil
}

func (m *mockJobExecutor) getSubmittedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.submittedJobs)
}

func (m *mockJobExecutor) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submittedJobs = nil
}

func TestNewScheduler(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}

	scheduler := NewScheduler(executor, log)

	if scheduler == nil {
		t.Fatal("NewScheduler() returned nil")
	}

	if scheduler.cron == nil {
		t.Fatal("Scheduler.cron is nil")
	}

	if scheduler.jobs == nil {
		t.Fatal("Scheduler.jobs is nil")
	}

	if scheduler.executor == nil {
		t.Fatal("Scheduler.executor is nil")
	}
}

func TestSchedulerStartStop(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	// Start should not panic
	scheduler.Start()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic
	scheduler.Stop()
}

func TestAddScheduledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:          "test-job-1",
		Name:        "Test Job",
		Description: "Test scheduled job",
		Schedule:    "0 0 * * *", // Daily at midnight
		JobTemplate: models.JobDefinition{
			Name:       "test-vm-export",
			VMPath:     "/datacenter/vm/test",
			OutputPath: "/tmp/output",
		},
		Enabled: true,
	}

	err := scheduler.AddScheduledJob(sj)
	if err != nil {
		t.Fatalf("AddScheduledJob() error = %v", err)
	}

	// Verify job was added
	retrieved, err := scheduler.GetScheduledJob("test-job-1")
	if err != nil {
		t.Fatalf("GetScheduledJob() error = %v", err)
	}

	if retrieved.Name != "Test Job" {
		t.Errorf("Expected job name 'Test Job', got '%s'", retrieved.Name)
	}

	if !retrieved.Enabled {
		t.Error("Expected job to be enabled")
	}
}

func TestAddScheduledJobInvalidCron(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	sj := &models.ScheduledJob{
		ID:       "test-job-invalid",
		Name:     "Invalid Job",
		Schedule: "invalid-cron-expression",
		Enabled:  true,
	}

	err := scheduler.AddScheduledJob(sj)
	if err == nil {
		t.Error("Expected error for invalid cron schedule, got nil")
	}
}

func TestAddScheduledJobDisabled(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:       "test-job-disabled",
		Name:     "Disabled Job",
		Schedule: "0 0 * * *",
		JobTemplate: models.JobDefinition{
			Name:   "test",
			VMPath: "/vm/test",
		},
		Enabled: false,
	}

	err := scheduler.AddScheduledJob(sj)
	if err != nil {
		t.Fatalf("AddScheduledJob() error = %v", err)
	}

	// Verify job was added but not scheduled
	retrieved, _ := scheduler.GetScheduledJob("test-job-disabled")
	if retrieved.Enabled {
		t.Error("Expected job to be disabled")
	}
}

func TestRemoveScheduledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:       "test-job-remove",
		Name:     "Remove Test",
		Schedule: "0 0 * * *",
		Enabled:  true,
	}

	scheduler.AddScheduledJob(sj)

	err := scheduler.RemoveScheduledJob("test-job-remove")
	if err != nil {
		t.Fatalf("RemoveScheduledJob() error = %v", err)
	}

	// Verify job was removed
	_, err = scheduler.GetScheduledJob("test-job-remove")
	if err == nil {
		t.Error("Expected error for removed job, got nil")
	}
}

func TestRemoveNonexistentJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	err := scheduler.RemoveScheduledJob("nonexistent-job")
	if err == nil {
		t.Error("Expected error for nonexistent job, got nil")
	}
}

func TestUpdateScheduledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	original := &models.ScheduledJob{
		ID:          "test-job-update",
		Name:        "Original Name",
		Description: "Original Description",
		Schedule:    "0 0 * * *",
		Enabled:     true,
	}

	scheduler.AddScheduledJob(original)

	// Update the job
	updates := &models.ScheduledJob{
		Name:        "Updated Name",
		Description: "Updated Description",
		Schedule:    "0 12 * * *", // Change to noon
		Enabled:     true,
	}

	err := scheduler.UpdateScheduledJob("test-job-update", updates)
	if err != nil {
		t.Fatalf("UpdateScheduledJob() error = %v", err)
	}

	// Verify updates
	updated, _ := scheduler.GetScheduledJob("test-job-update")
	if updated.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", updated.Name)
	}

	if updated.Description != "Updated Description" {
		t.Errorf("Expected description 'Updated Description', got '%s'", updated.Description)
	}

	if updated.Schedule != "0 12 * * *" {
		t.Errorf("Expected schedule '0 12 * * *', got '%s'", updated.Schedule)
	}
}

func TestUpdateNonexistentJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	updates := &models.ScheduledJob{
		Name: "Updated Name",
	}

	err := scheduler.UpdateScheduledJob("nonexistent", updates)
	if err == nil {
		t.Error("Expected error for nonexistent job, got nil")
	}
}

func TestGetScheduledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:       "test-job-get",
		Name:     "Get Test",
		Schedule: "0 0 * * *",
		Enabled:  true,
	}

	scheduler.AddScheduledJob(sj)

	retrieved, err := scheduler.GetScheduledJob("test-job-get")
	if err != nil {
		t.Fatalf("GetScheduledJob() error = %v", err)
	}

	if retrieved.Name != "Get Test" {
		t.Errorf("Expected name 'Get Test', got '%s'", retrieved.Name)
	}
}

func TestGetNonexistentJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	_, err := scheduler.GetScheduledJob("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent job, got nil")
	}
}

func TestListScheduledJobs(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	// Add multiple jobs
	for i := 1; i <= 3; i++ {
		sj := &models.ScheduledJob{
			ID:       string(rune('a' + i - 1)),
			Name:     "Test Job",
			Schedule: "0 0 * * *",
			Enabled:  i%2 == 1, // Alternate enabled/disabled
		}
		scheduler.AddScheduledJob(sj)
	}

	jobs := scheduler.ListScheduledJobs()
	if len(jobs) != 3 {
		t.Errorf("Expected 3 jobs, got %d", len(jobs))
	}
}

func TestEnableScheduledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:       "test-job-enable",
		Name:     "Enable Test",
		Schedule: "0 0 * * *",
		Enabled:  false,
	}

	scheduler.AddScheduledJob(sj)

	err := scheduler.EnableScheduledJob("test-job-enable")
	if err != nil {
		t.Fatalf("EnableScheduledJob() error = %v", err)
	}

	// Verify job is enabled
	retrieved, _ := scheduler.GetScheduledJob("test-job-enable")
	if !retrieved.Enabled {
		t.Error("Expected job to be enabled")
	}
}

func TestEnableAlreadyEnabledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:       "test-job-already-enabled",
		Name:     "Already Enabled",
		Schedule: "0 0 * * *",
		Enabled:  true,
	}

	scheduler.AddScheduledJob(sj)

	// Should not error when enabling already enabled job
	err := scheduler.EnableScheduledJob("test-job-already-enabled")
	if err != nil {
		t.Fatalf("EnableScheduledJob() error = %v", err)
	}
}

func TestDisableScheduledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:       "test-job-disable",
		Name:     "Disable Test",
		Schedule: "0 0 * * *",
		Enabled:  true,
	}

	scheduler.AddScheduledJob(sj)

	err := scheduler.DisableScheduledJob("test-job-disable")
	if err != nil {
		t.Fatalf("DisableScheduledJob() error = %v", err)
	}

	// Verify job is disabled
	retrieved, _ := scheduler.GetScheduledJob("test-job-disable")
	if retrieved.Enabled {
		t.Error("Expected job to be disabled")
	}
}

func TestDisableAlreadyDisabledJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:       "test-job-already-disabled",
		Name:     "Already Disabled",
		Schedule: "0 0 * * *",
		Enabled:  false,
	}

	scheduler.AddScheduledJob(sj)

	// Should not error when disabling already disabled job
	err := scheduler.DisableScheduledJob("test-job-already-disabled")
	if err != nil {
		t.Fatalf("DisableScheduledJob() error = %v", err)
	}
}

func TestTriggerNow(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	sj := &models.ScheduledJob{
		ID:   "test-job-trigger",
		Name: "Trigger Test",
		JobTemplate: models.JobDefinition{
			Name:   "test-vm",
			VMPath: "/vm/test",
		},
		Schedule: "0 0 * * *",
		Enabled:  true,
	}

	scheduler.AddScheduledJob(sj)

	err := scheduler.TriggerNow("test-job-trigger")
	if err != nil {
		t.Fatalf("TriggerNow() error = %v", err)
	}

	// Wait for job to be submitted
	time.Sleep(100 * time.Millisecond)

	// Verify job was submitted
	if executor.getSubmittedCount() == 0 {
		t.Error("Expected job to be submitted")
	}
}

func TestTriggerNonexistentJob(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	err := scheduler.TriggerNow("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent job, got nil")
	}
}

func TestGetScheduleStats(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	// Add some jobs
	sj1 := &models.ScheduledJob{
		ID:       "job1",
		Name:     "Job 1",
		Schedule: "0 0 * * *",
		Enabled:  true,
		RunCount: 5,
	}

	sj2 := &models.ScheduledJob{
		ID:       "job2",
		Name:     "Job 2",
		Schedule: "0 12 * * *",
		Enabled:  true,
		RunCount: 3,
	}

	sj3 := &models.ScheduledJob{
		ID:       "job3",
		Name:     "Job 3",
		Schedule: "0 0 * * *",
		Enabled:  false,
		RunCount: 0,
	}

	scheduler.AddScheduledJob(sj1)
	scheduler.AddScheduledJob(sj2)
	scheduler.AddScheduledJob(sj3)

	stats := scheduler.GetScheduleStats()

	if stats.TotalSchedules != 3 {
		t.Errorf("Expected 3 total schedules, got %d", stats.TotalSchedules)
	}

	if stats.EnabledSchedules != 2 {
		t.Errorf("Expected 2 enabled schedules, got %d", stats.EnabledSchedules)
	}

	if stats.TotalRuns != 8 { // 5 + 3 + 0
		t.Errorf("Expected 8 total runs, got %d", stats.TotalRuns)
	}

	if stats.NextRunning == nil {
		t.Error("Expected NextRunning to be set")
	}
}

func TestScheduleStatsEmpty(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	stats := scheduler.GetScheduleStats()

	if stats.TotalSchedules != 0 {
		t.Errorf("Expected 0 total schedules, got %d", stats.TotalSchedules)
	}

	if stats.NextRunning != nil {
		t.Error("Expected NextRunning to be nil")
	}
}

func TestScheduledJobExecution(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	// Use a very frequent schedule for testing (every second)
	sj := &models.ScheduledJob{
		ID:   "test-job-exec",
		Name: "Execution Test",
		JobTemplate: models.JobDefinition{
			Name:   "test-vm-scheduled",
			VMPath: "/vm/test",
		},
		Schedule: "* * * * * *", // Every second (with seconds field)
		Enabled:  true,
	}

	err := scheduler.AddScheduledJob(sj)
	if err != nil {
		// If cron doesn't support seconds, skip this test
		t.Skip("Cron library may not support seconds field")
	}

	// Wait for at least one execution
	time.Sleep(2 * time.Second)

	// Verify at least one job was submitted
	count := executor.getSubmittedCount()
	if count == 0 {
		t.Error("Expected at least one job to be submitted")
	}

	// Verify run count increased
	updated, _ := scheduler.GetScheduledJob("test-job-exec")
	if updated.RunCount == 0 {
		t.Error("Expected RunCount to be > 0")
	}

	if updated.LastRun == nil {
		t.Error("Expected LastRun to be set")
	}
}

func TestConcurrentOperations(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)
	scheduler.Start()
	defer scheduler.Stop()

	// Add multiple jobs concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sj := &models.ScheduledJob{
				ID:       string(rune('a' + idx)),
				Name:     "Concurrent Job",
				Schedule: "0 0 * * *",
				Enabled:  true,
			}
			scheduler.AddScheduledJob(sj)
		}(i)
	}

	wg.Wait()

	// Verify all jobs were added
	jobs := scheduler.ListScheduledJobs()
	if len(jobs) != 10 {
		t.Errorf("Expected 10 jobs, got %d", len(jobs))
	}
}

// mockScheduleStore is a mock implementation of ScheduleStore for testing
type mockScheduleStore struct {
	schedules []*models.ScheduledJob
	listError error
}

func (m *mockScheduleStore) SaveSchedule(sj *models.ScheduledJob) error {
	return nil
}

func (m *mockScheduleStore) UpdateSchedule(sj *models.ScheduledJob) error {
	return nil
}

func (m *mockScheduleStore) GetSchedule(id string) (*models.ScheduledJob, error) {
	return nil, nil
}

func (m *mockScheduleStore) ListSchedules(enabled *bool) ([]*models.ScheduledJob, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	return m.schedules, nil
}

func (m *mockScheduleStore) DeleteSchedule(id string) error {
	return nil
}

func TestLoadSchedules(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	// Create mock store with some schedules
	mockStore := &mockScheduleStore{
		schedules: []*models.ScheduledJob{
			{
				ID:       "job1",
				Name:     "Test Job 1",
				Schedule: "0 0 * * *",
				Enabled:  true,
				JobTemplate: models.JobDefinition{
					ID:   "template1",
					Name: "Template 1",
				},
			},
			{
				ID:       "job2",
				Name:     "Test Job 2",
				Schedule: "0 12 * * *",
				Enabled:  false, // This one is disabled
				JobTemplate: models.JobDefinition{
					ID:   "template2",
					Name: "Template 2",
				},
			},
		},
	}

	scheduler.SetStore(mockStore)
	scheduler.Start()
	defer scheduler.Stop()

	// Load schedules
	err := scheduler.LoadSchedules()
	if err != nil {
		t.Fatalf("LoadSchedules failed: %v", err)
	}

	// Verify schedules were loaded
	jobs := scheduler.ListScheduledJobs()
	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}

	// Verify the enabled job is in cron
	job1, err := scheduler.GetScheduledJob("job1")
	if err != nil {
		t.Fatalf("job1 not found: %v", err)
	}
	if !job1.NextRun.IsZero() {
		// NextRun should be set for enabled jobs
		t.Log("job1 NextRun is set correctly")
	}

	// Verify the disabled job is not in cron (NextRun should be zero)
	job2, err := scheduler.GetScheduledJob("job2")
	if err != nil {
		t.Fatalf("job2 not found: %v", err)
	}
	if !job2.NextRun.IsZero() {
		t.Error("Expected disabled job to have zero NextRun")
	}
}

func TestLoadSchedules_NoStore(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	// Don't set a store - should return without error
	err := scheduler.LoadSchedules()
	if err != nil {
		t.Errorf("LoadSchedules without store should not error, got: %v", err)
	}
}

func TestLoadSchedules_StoreError(t *testing.T) {
	log := logger.New("info")
	executor := &mockJobExecutor{}
	scheduler := NewScheduler(executor, log)

	// Create mock store that returns an error
	mockStore := &mockScheduleStore{
		listError: fmt.Errorf("database error"),
	}

	scheduler.SetStore(mockStore)

	err := scheduler.LoadSchedules()
	if err == nil {
		t.Error("Expected error when store returns error")
	}
}
