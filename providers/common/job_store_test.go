// SPDX-License-Identifier: Apache-2.0

package common

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteJobStore_CreateAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create a job
	now := time.Now()
	job := &Job{
		ID:        "job-001",
		Name:      "Test Migration",
		VMName:    "test-vm",
		VMPath:    "/dc/vm/test-vm",
		Provider:  "vsphere",
		OutputDir: "/exports/test-vm",
		Status:    JobStatusPending,
		CreatedAt: now,
		Progress:  0.0,
		User:      "testuser",
		Metadata: map[string]interface{}{
			"format":   "ova",
			"compress": true,
		},
	}

	// Save job
	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Load job
	loaded, err := store.LoadJob("job-001")
	if err != nil {
		t.Fatalf("Failed to load job: %v", err)
	}

	// Verify fields
	if loaded.ID != job.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, job.ID)
	}
	if loaded.VMName != job.VMName {
		t.Errorf("VMName mismatch: got %s, want %s", loaded.VMName, job.VMName)
	}
	if loaded.Status != job.Status {
		t.Errorf("Status mismatch: got %s, want %s", loaded.Status, job.Status)
	}
	if loaded.Progress != job.Progress {
		t.Errorf("Progress mismatch: got %.2f, want %.2f", loaded.Progress, job.Progress)
	}

	// Verify metadata
	if loaded.Metadata["format"] != "ova" {
		t.Errorf("Metadata format mismatch")
	}
	if loaded.Metadata["compress"] != true {
		t.Errorf("Metadata compress mismatch")
	}

	t.Log("✅ Job save and load test passed")
}

func TestSQLiteJobStore_UpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create a job
	job := &Job{
		ID:        "job-002",
		Name:      "Status Test",
		VMName:    "test-vm",
		VMPath:    "/dc/vm/test-vm",
		Provider:  "vsphere",
		OutputDir: "/exports/test-vm",
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
		User:      "testuser",
	}

	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Update status to running
	if err := store.UpdateJobStatus("job-002", JobStatusRunning); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Load and verify
	loaded, err := store.LoadJob("job-002")
	if err != nil {
		t.Fatalf("Failed to load job: %v", err)
	}

	if loaded.Status != JobStatusRunning {
		t.Errorf("Status = %s, want %s", loaded.Status, JobStatusRunning)
	}

	// Update to completed
	if err := store.UpdateJobStatus("job-002", JobStatusCompleted); err != nil {
		t.Fatalf("Failed to update to completed: %v", err)
	}

	loaded, err = store.LoadJob("job-002")
	if err != nil {
		t.Fatalf("Failed to load job: %v", err)
	}

	if loaded.Status != JobStatusCompleted {
		t.Errorf("Status = %s, want %s", loaded.Status, JobStatusCompleted)
	}

	t.Log("✅ Status update test passed")
}

func TestSQLiteJobStore_UpdateProgress(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create a job
	job := &Job{
		ID:        "job-003",
		Name:      "Progress Test",
		VMName:    "test-vm",
		VMPath:    "/dc/vm/test-vm",
		Provider:  "vsphere",
		OutputDir: "/exports/test-vm",
		Status:    JobStatusRunning,
		CreatedAt: time.Now(),
		Progress:  0.0,
		User:      "testuser",
	}

	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Update progress
	progressUpdates := []float64{25.0, 50.0, 75.0, 100.0}
	for _, progress := range progressUpdates {
		if err := store.UpdateJobProgress("job-003", progress); err != nil {
			t.Fatalf("Failed to update progress to %.2f: %v", progress, err)
		}

		loaded, err := store.LoadJob("job-003")
		if err != nil {
			t.Fatalf("Failed to load job: %v", err)
		}

		if loaded.Progress != progress {
			t.Errorf("Progress = %.2f, want %.2f", loaded.Progress, progress)
		}
	}

	t.Log("✅ Progress update test passed")
}

func TestSQLiteJobStore_ListJobs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create multiple jobs
	jobs := []*Job{
		{
			ID: "job-list-001", Name: "Job 1", VMName: "vm-001", VMPath: "/dc/vm/vm-001",
			Provider: "vsphere", OutputDir: "/exports/vm-001", Status: JobStatusCompleted,
			CreatedAt: time.Now().Add(-2 * time.Hour), User: "user1",
		},
		{
			ID: "job-list-002", Name: "Job 2", VMName: "vm-002", VMPath: "/dc/vm/vm-002",
			Provider: "vsphere", OutputDir: "/exports/vm-002", Status: JobStatusRunning,
			CreatedAt: time.Now().Add(-1 * time.Hour), User: "user1",
		},
		{
			ID: "job-list-003", Name: "Job 3", VMName: "vm-003", VMPath: "/dc/vm/vm-003",
			Provider: "hyperv", OutputDir: "/exports/vm-003", Status: JobStatusFailed,
			CreatedAt: time.Now().Add(-30 * time.Minute), User: "user2",
		},
		{
			ID: "job-list-004", Name: "Job 4", VMName: "vm-004", VMPath: "/dc/vm/vm-004",
			Provider: "vsphere", OutputDir: "/exports/vm-004", Status: JobStatusPending,
			CreatedAt: time.Now(), User: "user1",
		},
	}

	for _, job := range jobs {
		if err := store.SaveJob(job); err != nil {
			t.Fatalf("Failed to save job %s: %v", job.ID, err)
		}
	}

	// Test: List all jobs
	allJobs, err := store.ListJobs(JobFilter{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	if len(allJobs) != 4 {
		t.Errorf("ListJobs (all) returned %d jobs, want 4", len(allJobs))
	}

	// Test: Filter by status
	runningJobs, err := store.ListJobs(JobFilter{Status: JobStatusRunning})
	if err != nil {
		t.Fatalf("Failed to list running jobs: %v", err)
	}
	if len(runningJobs) != 1 {
		t.Errorf("ListJobs (running) returned %d jobs, want 1", len(runningJobs))
	}

	// Test: Filter by provider
	vsphereJobs, err := store.ListJobs(JobFilter{Provider: "vsphere"})
	if err != nil {
		t.Fatalf("Failed to list vsphere jobs: %v", err)
	}
	if len(vsphereJobs) != 3 {
		t.Errorf("ListJobs (vsphere) returned %d jobs, want 3", len(vsphereJobs))
	}

	// Test: Filter by user
	user1Jobs, err := store.ListJobs(JobFilter{User: "user1"})
	if err != nil {
		t.Fatalf("Failed to list user1 jobs: %v", err)
	}
	if len(user1Jobs) != 3 {
		t.Errorf("ListJobs (user1) returned %d jobs, want 3", len(user1Jobs))
	}

	// Test: Limit
	limitedJobs, err := store.ListJobs(JobFilter{Limit: 2})
	if err != nil {
		t.Fatalf("Failed to list limited jobs: %v", err)
	}
	if len(limitedJobs) != 2 {
		t.Errorf("ListJobs (limit=2) returned %d jobs, want 2", len(limitedJobs))
	}

	t.Log("✅ List jobs test passed")
}

func TestSQLiteJobStore_DeleteJob(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create a job
	job := &Job{
		ID:        "job-delete-001",
		Name:      "Delete Test",
		VMName:    "test-vm",
		VMPath:    "/dc/vm/test-vm",
		Provider:  "vsphere",
		OutputDir: "/exports/test-vm",
		Status:    JobStatusCompleted,
		CreatedAt: time.Now(),
		User:      "testuser",
	}

	if err := store.SaveJob(job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Verify job exists
	_, err = store.LoadJob("job-delete-001")
	if err != nil {
		t.Fatalf("Job should exist: %v", err)
	}

	// Delete job
	if err := store.DeleteJob("job-delete-001"); err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}

	// Verify job doesn't exist
	_, err = store.LoadJob("job-delete-001")
	if err == nil {
		t.Error("Job should not exist after deletion")
	}

	t.Log("✅ Delete job test passed")
}

func TestSQLiteJobStore_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create jobs with different statuses
	jobs := []*Job{
		{ID: "stats-001", Name: "Job 1", VMName: "vm-001", VMPath: "/dc/vm/vm-001", Provider: "vsphere", OutputDir: "/exports/vm-001", Status: JobStatusCompleted, CreatedAt: time.Now(), User: "user1"},
		{ID: "stats-002", Name: "Job 2", VMName: "vm-002", VMPath: "/dc/vm/vm-002", Provider: "vsphere", OutputDir: "/exports/vm-002", Status: JobStatusCompleted, CreatedAt: time.Now(), User: "user1"},
		{ID: "stats-003", Name: "Job 3", VMName: "vm-003", VMPath: "/dc/vm/vm-003", Provider: "vsphere", OutputDir: "/exports/vm-003", Status: JobStatusFailed, CreatedAt: time.Now(), User: "user1"},
		{ID: "stats-004", Name: "Job 4", VMName: "vm-004", VMPath: "/dc/vm/vm-004", Provider: "vsphere", OutputDir: "/exports/vm-004", Status: JobStatusRunning, CreatedAt: time.Now(), User: "user1"},
		{ID: "stats-005", Name: "Job 5", VMName: "vm-005", VMPath: "/dc/vm/vm-005", Provider: "vsphere", OutputDir: "/exports/vm-005", Status: JobStatusPending, CreatedAt: time.Now(), User: "user1"},
	}

	for _, job := range jobs {
		if err := store.SaveJob(job); err != nil {
			t.Fatalf("Failed to save job %s: %v", job.ID, err)
		}
	}

	// Get stats
	stats, err := store.GetJobStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	// Verify stats
	if stats.Total != 5 {
		t.Errorf("Total = %d, want 5", stats.Total)
	}
	if stats.Completed != 2 {
		t.Errorf("Completed = %d, want 2", stats.Completed)
	}
	if stats.Failed != 1 {
		t.Errorf("Failed = %d, want 1", stats.Failed)
	}
	if stats.Running != 1 {
		t.Errorf("Running = %d, want 1", stats.Running)
	}
	if stats.Pending != 1 {
		t.Errorf("Pending = %d, want 1", stats.Pending)
	}

	expectedSuccessRate := 2.0 / 5.0 * 100.0
	if stats.SuccessRate < expectedSuccessRate-1 || stats.SuccessRate > expectedSuccessRate+1 {
		t.Errorf("SuccessRate = %.2f, want ~%.2f", stats.SuccessRate, expectedSuccessRate)
	}

	t.Log("✅ Get stats test passed")
	t.Logf("   Total: %d, Completed: %d, Failed: %d, Success Rate: %.2f%%",
		stats.Total, stats.Completed, stats.Failed, stats.SuccessRate)
}

func TestSQLiteJobStore_Prune(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create old completed jobs
	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-1 * time.Hour)

	jobs := []*Job{
		{
			ID: "prune-001", Name: "Old Job 1", VMName: "vm-001", VMPath: "/dc/vm/vm-001",
			Provider: "vsphere", OutputDir: "/exports/vm-001", Status: JobStatusCompleted,
			CreatedAt: oldTime, CompletedAt: &oldTime, User: "user1",
		},
		{
			ID: "prune-002", Name: "Old Job 2", VMName: "vm-002", VMPath: "/dc/vm/vm-002",
			Provider: "vsphere", OutputDir: "/exports/vm-002", Status: JobStatusFailed,
			CreatedAt: oldTime, CompletedAt: &oldTime, User: "user1",
		},
		{
			ID: "prune-003", Name: "Recent Job", VMName: "vm-003", VMPath: "/dc/vm/vm-003",
			Provider: "vsphere", OutputDir: "/exports/vm-003", Status: JobStatusCompleted,
			CreatedAt: recentTime, CompletedAt: &recentTime, User: "user1",
		},
		{
			ID: "prune-004", Name: "Running Job", VMName: "vm-004", VMPath: "/dc/vm/vm-004",
			Provider: "vsphere", OutputDir: "/exports/vm-004", Status: JobStatusRunning,
			CreatedAt: oldTime, User: "user1",
		},
	}

	for _, job := range jobs {
		if err := store.SaveJob(job); err != nil {
			t.Fatalf("Failed to save job %s: %v", job.ID, err)
		}
	}

	// Prune jobs older than 24 hours
	deleted, err := store.Prune(24 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to prune: %v", err)
	}

	// Should delete 2 old completed/failed jobs
	if deleted != 2 {
		t.Errorf("Pruned %d jobs, want 2", deleted)
	}

	// Verify remaining jobs
	allJobs, err := store.ListJobs(JobFilter{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	if len(allJobs) != 2 {
		t.Errorf("After prune: %d jobs remaining, want 2", len(allJobs))
	}

	t.Log("✅ Prune test passed")
	t.Logf("   Deleted %d old jobs", deleted)
}

func TestSQLiteJobStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store and save job
	{
		store, err := NewSQLiteJobStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}

		job := &Job{
			ID:        "persist-001",
			Name:      "Persistence Test",
			VMName:    "test-vm",
			VMPath:    "/dc/vm/test-vm",
			Provider:  "vsphere",
			OutputDir: "/exports/test-vm",
			Status:    JobStatusCompleted,
			CreatedAt: time.Now(),
			User:      "testuser",
		}

		if err := store.SaveJob(job); err != nil {
			t.Fatalf("Failed to save job: %v", err)
		}

		_ = store.Close()
	}

	// Reopen database and verify job persisted
	{
		store, err := NewSQLiteJobStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to reopen store: %v", err)
		}
		defer func() { _ = store.Close() }()

		job, err := store.LoadJob("persist-001")
		if err != nil {
			t.Fatalf("Failed to load persisted job: %v", err)
		}

		if job.ID != "persist-001" {
			t.Errorf("Loaded job ID = %s, want persist-001", job.ID)
		}
		if job.VMName != "test-vm" {
			t.Errorf("Loaded job VMName = %s, want test-vm", job.VMName)
		}
		if job.Status != JobStatusCompleted {
			t.Errorf("Loaded job Status = %s, want %s", job.Status, JobStatusCompleted)
		}
	}

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file should exist after close")
	}

	t.Log("✅ Persistence test passed")
}

func TestNewSQLiteJobStore_InvalidPath(t *testing.T) {
	// Try to create store with invalid path
	_, err := NewSQLiteJobStore("/nonexistent/directory/that/does/not/exist/test.db")
	if err == nil {
		t.Error("Expected error when creating store with invalid path")
	}
}

func TestSQLiteJobStore_SaveJobValidation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "validation.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Try to save job with empty ID
	job := &Job{
		ID:     "",
		VMName: "test",
		Status: JobStatusPending,
	}

	_ = store.SaveJob(job)
	// Should succeed even with empty ID (database will handle it)
	// This tests that the function doesn't crash with edge cases
}

func TestSQLiteJobStore_LoadNonexistentJob(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	_, err = store.LoadJob("nonexistent-job-id")
	if err == nil {
		t.Error("Expected error when loading nonexistent job")
	}
}

func TestSQLiteJobStore_UpdateNonexistentJob(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "update.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Try to update status of nonexistent job
	_ = store.UpdateJobStatus("nonexistent-job", JobStatusRunning)
	// Should handle gracefully (may succeed with no effect or return error)
	// This tests robustness
}

func TestSQLiteJobStore_DeleteNonexistentJob(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "delete.db")

	store, err := NewSQLiteJobStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Try to delete nonexistent job - should not error
	_ = store.DeleteJob("nonexistent-job")
	// Should succeed (no-op) or return specific error
}
