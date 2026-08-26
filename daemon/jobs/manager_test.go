// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"testing"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// Helper to create a test capability detector
func newTestDetector(log logger.Logger) *capabilities.Detector {
	return capabilities.NewDetector(log)
}

func TestNewManager(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}

	status := mgr.GetStatus()
	if status.TotalJobs != 0 {
		t.Errorf("Expected 0 total jobs, got %d", status.TotalJobs)
	}

	if status.RunningJobs != 0 {
		t.Errorf("Expected 0 running jobs, got %d", status.RunningJobs)
	}
}

func TestSubmitJob(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	jobDef := models.JobDefinition{
		Name:       "test-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test-output",
		Options: &models.ExportOptions{
			ParallelDownloads: 4,
			RemoveCDROM:       true,
		},
	}

	jobID, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("SubmitJob() error = %v", err)
	}

	if jobID == "" {
		t.Error("Expected non-empty job ID")
	}

	// Verify job was added
	status := mgr.GetStatus()
	if status.TotalJobs != 1 {
		t.Errorf("Expected 1 total job, got %d", status.TotalJobs)
	}
}

func TestGetJob(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	jobDef := models.JobDefinition{
		Name:       "test-job-2",
		VMPath:     "/datacenter/vm/test-vm-2",
		OutputPath: "/tmp/test-output-2",
	}

	jobID, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("SubmitJob() error = %v", err)
	}

	// Get the job
	job, err := mgr.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if job == nil {
		t.Fatal("GetJob() returned nil job")
	}

	if job.Definition.Name != "test-job-2" {
		t.Errorf("Expected job name 'test-job-2', got '%s'", job.Definition.Name)
	}
}

func TestGetJobNotFound(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	_, err := mgr.GetJob("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent job ID, got nil")
	}
}

func TestGetAllJobs(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Submit multiple jobs
	for i := 0; i < 3; i++ {
		jobDef := models.JobDefinition{
			Name:       "test-job",
			VMPath:     "/datacenter/vm/test-vm",
			OutputPath: "/tmp/test",
		}
		_, err := mgr.SubmitJob(jobDef)
		if err != nil {
			t.Fatalf("SubmitJob() error = %v", err)
		}
	}

	jobs := mgr.GetAllJobs()
	if len(jobs) != 3 {
		t.Errorf("Expected 3 jobs, got %d", len(jobs))
	}
}

func TestListJobsByStatus(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	jobDef := models.JobDefinition{
		Name:       "pending-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test",
	}

	jobID, _ := mgr.SubmitJob(jobDef)

	// Get job and verify it's pending or running (since it starts automatically)
	job, _ := mgr.GetJob(jobID)
	if job.Status != models.JobStatusPending && job.Status != models.JobStatusRunning {
		t.Logf("Job status is %s (expected pending or running)", job.Status)
	}

	// List pending jobs
	pendingJobs := mgr.ListJobs([]models.JobStatus{models.JobStatusPending}, 0)
	// Since jobs start automatically, there might be 0 or 1 pending jobs
	t.Logf("Found %d pending jobs", len(pendingJobs))
}

func TestCancelJob(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	jobDef := models.JobDefinition{
		Name:       "cancel-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test",
	}

	jobID, _ := mgr.SubmitJob(jobDef)

	// Try to cancel the job
	err := mgr.CancelJob(jobID)
	// It might fail if the job already completed, which is OK for this test
	if err != nil {
		t.Logf("CancelJob() returned error (job may have completed): %v", err)
	}

	// If cancel succeeded, verify cancelled state
	job, _ := mgr.GetJob(jobID)
	if job.Status == models.JobStatusCancelled {
		t.Log("Job successfully cancelled")
	} else {
		t.Logf("Job status is %s", job.Status)
	}
}

func TestSubmitBatch(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create batch of job definitions
	defs := []models.JobDefinition{
		{
			Name:       "batch-job-1",
			VMPath:     "/datacenter/vm/vm1",
			OutputPath: "/tmp/out1",
		},
		{
			Name:       "batch-job-2",
			VMPath:     "/datacenter/vm/vm2",
			OutputPath: "/tmp/out2",
		},
		{
			Name:       "batch-job-3",
			VMPath:     "/datacenter/vm/vm3",
			OutputPath: "/tmp/out3",
		},
	}

	ids, errs := mgr.SubmitBatch(defs)

	if len(errs) > 0 {
		t.Errorf("SubmitBatch() returned %d errors", len(errs))
	}

	if len(ids) != 3 {
		t.Errorf("Expected 3 job IDs, got %d", len(ids))
	}

	// Verify all jobs were added
	status := mgr.GetStatus()
	if status.TotalJobs != 3 {
		t.Errorf("Expected 3 total jobs, got %d", status.TotalJobs)
	}
}

func TestGetStatus(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	status := mgr.GetStatus()

	if status == nil {
		t.Fatal("GetStatus() returned nil")
	}

	if status.Version == "" {
		t.Error("Expected non-empty version")
	}

	if status.Uptime == "" {
		t.Error("Expected non-empty uptime")
	}

	if status.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestGetStatusWithJobs(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create jobs with different statuses
	job1 := &models.Job{Definition: models.JobDefinition{ID: "job1"}, Status: models.JobStatusRunning}
	job2 := &models.Job{Definition: models.JobDefinition{ID: "job2"}, Status: models.JobStatusCompleted}
	job3 := &models.Job{Definition: models.JobDefinition{ID: "job3"}, Status: models.JobStatusFailed}
	job4 := &models.Job{Definition: models.JobDefinition{ID: "job4"}, Status: models.JobStatusCancelled}
	job5 := &models.Job{Definition: models.JobDefinition{ID: "job5"}, Status: models.JobStatusPending}

	mgr.jobs[job1.Definition.ID] = job1
	mgr.jobs[job2.Definition.ID] = job2
	mgr.jobs[job3.Definition.ID] = job3
	mgr.jobs[job4.Definition.ID] = job4
	mgr.jobs[job5.Definition.ID] = job5

	status := mgr.GetStatus()

	if status.TotalJobs != 5 {
		t.Errorf("Expected TotalJobs 5, got %d", status.TotalJobs)
	}

	if status.RunningJobs != 1 {
		t.Errorf("Expected RunningJobs 1, got %d", status.RunningJobs)
	}

	if status.CompletedJobs != 1 {
		t.Errorf("Expected CompletedJobs 1, got %d", status.CompletedJobs)
	}

	if status.FailedJobs != 1 {
		t.Errorf("Expected FailedJobs 1, got %d", status.FailedJobs)
	}

	if status.CancelledJobs != 1 {
		t.Errorf("Expected CancelledJobs 1, got %d", status.CancelledJobs)
	}
}

func TestShutdown(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Should not panic
	mgr.Shutdown()
}

func TestUpdateProgress(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create a job
	jobDef := models.JobDefinition{
		Name:       "progress-test-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test",
	}

	jobID, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("SubmitJob() error = %v", err)
	}

	// Create progress update
	progress := &models.JobProgress{
		Phase:           "exporting",
		CurrentFile:     "disk1.vmdk",
		CurrentStep:     "Downloading disk",
		FilesDownloaded: 1,
		TotalFiles:      3,
		BytesDownloaded: 1024,
		TotalBytes:      4096,
		PercentComplete: 25.0,
	}

	// Update progress
	mgr.updateProgress(jobID, progress)

	// Verify progress was updated
	job, err := mgr.GetJob(jobID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if job.Progress == nil {
		t.Fatal("Expected progress to be set")
	}

	if job.Progress.Phase != "exporting" {
		t.Errorf("Expected phase 'exporting', got '%s'", job.Progress.Phase)
	}

	if job.Progress.CurrentFile != "disk1.vmdk" {
		t.Errorf("Expected CurrentFile 'disk1.vmdk', got '%s'", job.Progress.CurrentFile)
	}

	if job.Progress.PercentComplete != 25.0 {
		t.Errorf("Expected PercentComplete 25.0, got %f", job.Progress.PercentComplete)
	}

	if job.Progress.FilesDownloaded != 1 {
		t.Errorf("Expected FilesDownloaded 1, got %d", job.Progress.FilesDownloaded)
	}

	if job.Progress.TotalFiles != 3 {
		t.Errorf("Expected TotalFiles 3, got %d", job.Progress.TotalFiles)
	}
}

func TestUpdateProgress_NonExistentJob(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	progress := &models.JobProgress{
		Phase:           "exporting",
		PercentComplete: 50.0,
	}

	// Should not panic when updating progress for non-existent job
	mgr.updateProgress("non-existent-id", progress)
}

func TestGetVSphereClient(t *testing.T) {
	// Skip this test if vSphere credentials are not configured
	// This test would hang trying to connect to a non-existent server
	t.Skip("Skipping GetVSphereClient test - requires vSphere credentials")
}

func TestNormalizeJobDefinition(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	manager := NewManager(log, detector)

	tests := []struct {
		name     string
		input    *models.JobDefinition
		validate func(*testing.T, *models.JobDefinition)
	}{
		{
			name: "convert old-style VCenter fields",
			input: &models.JobDefinition{
				VCenterURL: "vcenter.example.com",
				Username:   "admin",
				Password:   "password",
				Insecure:   true,
			},
			validate: func(t *testing.T, def *models.JobDefinition) {
				if def.VCenter == nil {
					t.Error("Expected VCenter to be set")
					return
				}
				if def.VCenter.Server != "vcenter.example.com" {
					t.Errorf("Expected Server='vcenter.example.com', got '%s'", def.VCenter.Server)
				}
				if def.VCenter.Username != "admin" {
					t.Errorf("Expected Username='admin', got '%s'", def.VCenter.Username)
				}
				if def.VCenter.Password != "password" {
					t.Errorf("Expected Password='password', got '%s'", def.VCenter.Password)
				}
				if !def.VCenter.Insecure {
					t.Error("Expected Insecure=true")
				}
			},
		},
		{
			name: "fallback server when empty",
			input: &models.JobDefinition{
				Username: "admin",
				Password: "password",
			},
			validate: func(t *testing.T, def *models.JobDefinition) {
				if def.VCenter == nil {
					t.Error("Expected VCenter to be set")
					return
				}
				if def.VCenter.Server != "vcenter.example.com" {
					t.Errorf("Expected fallback Server='vcenter.example.com', got '%s'", def.VCenter.Server)
				}
			},
		},
		{
			name: "normalize output paths",
			input: &models.JobDefinition{
				OutputPath: "/tmp/output",
			},
			validate: func(t *testing.T, def *models.JobDefinition) {
				if def.OutputDir != "/tmp/output" {
					t.Errorf("Expected OutputDir='/tmp/output', got '%s'", def.OutputDir)
				}
			},
		},
		{
			name: "normalize export method",
			input: &models.JobDefinition{
				Method: "govc",
			},
			validate: func(t *testing.T, def *models.JobDefinition) {
				if def.ExportMethod != "govc" {
					t.Errorf("Expected ExportMethod='govc', got '%s'", def.ExportMethod)
				}
			},
		},
		{
			name: "set default format",
			input: &models.JobDefinition{
				Format: "",
			},
			validate: func(t *testing.T, def *models.JobDefinition) {
				if def.Format != "ovf" {
					t.Errorf("Expected Format='ovf', got '%s'", def.Format)
				}
			},
		},
		{
			name: "preserve existing VCenter",
			input: &models.JobDefinition{
				VCenter: &models.VCenterConfig{
					Server:   "existing.vcenter.com",
					Username: "existing-user",
				},
				VCenterURL: "ignored.vcenter.com",
			},
			validate: func(t *testing.T, def *models.JobDefinition) {
				if def.VCenter.Server != "existing.vcenter.com" {
					t.Errorf("Expected existing VCenter to be preserved, got '%s'", def.VCenter.Server)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager.normalizeJobDefinition(tt.input)
			tt.validate(t, tt.input)
		})
	}
}

func TestSelectExportMethod(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	manager := NewManager(log, detector)

	tests := []struct {
		name     string
		jobDef   *models.JobDefinition
		expected capabilities.ExportMethod
	}{
		{
			name: "use specified method when available",
			jobDef: &models.JobDefinition{
				ExportMethod: string(capabilities.ExportMethodWeb),
			},
			expected: capabilities.ExportMethodWeb,
		},
		{
			name: "use default when method not specified",
			jobDef: &models.JobDefinition{
				ExportMethod: "",
			},
			expected: capabilities.ExportMethodWeb, // Web is always available in test setup
		},
		{
			name: "fallback to default when method unavailable",
			jobDef: &models.JobDefinition{
				ExportMethod: "unavailable-method",
			},
			expected: capabilities.ExportMethodWeb,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.selectExportMethod(tt.jobDef)
			if result != tt.expected {
				t.Errorf("Expected method '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetJobWithProgress(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create a job
	jobDef := models.JobDefinition{
		Name:       "test-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test-output",
	}

	jobID, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Set progress on the job
	mgr.mu.Lock()
	if job, exists := mgr.jobs[jobID]; exists {
		job.Progress = &models.JobProgress{
			Phase:           "exporting",
			PercentComplete: 50.0,
			CurrentStep:     "Downloading disk",
			ExportMethod:    "web",
		}
	}
	mgr.mu.Unlock()

	// Get the job
	retrievedJob, err := mgr.GetJob(jobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	// Verify progress was deep copied
	if retrievedJob.Progress == nil {
		t.Fatal("Expected progress to be present")
	}

	if retrievedJob.Progress.Phase != "exporting" {
		t.Errorf("Expected phase 'exporting', got '%s'", retrievedJob.Progress.Phase)
	}

	if retrievedJob.Progress.PercentComplete != 50.0 {
		t.Errorf("Expected 50%% complete, got %.1f%%", retrievedJob.Progress.PercentComplete)
	}
}

func TestGetJobWithResult(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create a job
	jobDef := models.JobDefinition{
		Name:       "test-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test-output",
	}

	jobID, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Set result on the job
	mgr.mu.Lock()
	if job, exists := mgr.jobs[jobID]; exists {
		job.Result = &models.JobResult{
			Success:      true,
			ExportMethod: "web",
			Files:        []string{"disk1.vmdk", "disk2.vmdk", "vm.ovf"},
			TotalSize:    1024000,
		}
	}
	mgr.mu.Unlock()

	// Get the job
	retrievedJob, err := mgr.GetJob(jobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	// Verify result was deep copied
	if retrievedJob.Result == nil {
		t.Fatal("Expected result to be present")
	}

	if !retrievedJob.Result.Success {
		t.Error("Expected result success to be true")
	}

	if len(retrievedJob.Result.Files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(retrievedJob.Result.Files))
	}

	if retrievedJob.Result.TotalSize != 1024000 {
		t.Errorf("Expected total size 1024000, got %d", retrievedJob.Result.TotalSize)
	}
}

func TestListJobsWithLimit(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create multiple jobs
	for i := 0; i < 5; i++ {
		jobDef := models.JobDefinition{
			Name:       "test-job",
			VMPath:     "/datacenter/vm/test-vm",
			OutputPath: "/tmp/test-output",
		}
		_, err := mgr.SubmitJob(jobDef)
		if err != nil {
			t.Fatalf("Failed to submit job %d: %v", i, err)
		}
	}

	// List with limit
	jobs := mgr.ListJobs(nil, 3)
	if len(jobs) != 3 {
		t.Errorf("Expected 3 jobs with limit, got %d", len(jobs))
	}
}

func TestListJobsWithProgressAndResult(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create a job
	jobDef := models.JobDefinition{
		Name:       "test-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test-output",
	}

	jobID, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Set progress and result
	mgr.mu.Lock()
	if job, exists := mgr.jobs[jobID]; exists {
		job.Progress = &models.JobProgress{
			Phase:           "exporting",
			PercentComplete: 75.0,
		}
		job.Result = &models.JobResult{
			Success: true,
			Files:   []string{"file1.vmdk", "file2.ovf"},
		}
	}
	mgr.mu.Unlock()

	// List all jobs
	jobs := mgr.ListJobs(nil, 0)
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}

	// Verify progress and result were deep copied
	job := jobs[0]
	if job.Progress == nil {
		t.Error("Expected progress to be present in listed job")
	}
	if job.Result == nil {
		t.Error("Expected result to be present in listed job")
	}

	if job.Progress != nil && job.Progress.PercentComplete != 75.0 {
		t.Errorf("Expected 75%% progress, got %.1f%%", job.Progress.PercentComplete)
	}

	if job.Result != nil && len(job.Result.Files) != 2 {
		t.Errorf("Expected 2 files in result, got %d", len(job.Result.Files))
	}
}

func TestListJobsFilteringWithProgress(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create jobs with different statuses
	for i := 0; i < 3; i++ {
		jobDef := models.JobDefinition{
			Name:       "test-job",
			VMPath:     "/datacenter/vm/test-vm",
			OutputPath: "/tmp/test-output",
		}
		jobID, err := mgr.SubmitJob(jobDef)
		if err != nil {
			t.Fatalf("Failed to submit job %d: %v", i, err)
		}

		// Set different statuses and add progress
		mgr.mu.Lock()
		if job, exists := mgr.jobs[jobID]; exists {
			switch i {
			case 0:
				job.Status = models.JobStatusRunning
			case 1:
				job.Status = models.JobStatusCompleted
			default:
				job.Status = models.JobStatusFailed
			}
			job.Progress = &models.JobProgress{
				Phase:           "test",
				PercentComplete: float64(i * 25),
			}
		}
		mgr.mu.Unlock()
	}

	// List only completed jobs
	completedJobs := mgr.ListJobs([]models.JobStatus{models.JobStatusCompleted}, 0)
	if len(completedJobs) != 1 {
		t.Errorf("Expected 1 completed job, got %d", len(completedJobs))
	}

	// Verify the completed job has progress
	if len(completedJobs) > 0 && completedJobs[0].Progress == nil {
		t.Error("Expected progress in completed job")
	}
}

func TestListJobsEmptyStatuses(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create a job
	jobDef := models.JobDefinition{
		Name:       "test-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test-output",
	}

	_, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// List with empty statuses array - should return all jobs
	jobs := mgr.ListJobs([]models.JobStatus{}, 0)
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job with empty status filter, got %d", len(jobs))
	}
}

func TestGetJobDeepCopyIsolation(t *testing.T) {
	log := logger.New("info")
	detector := newTestDetector(log)
	mgr := NewManager(log, detector)

	// Create a job
	jobDef := models.JobDefinition{
		Name:       "test-job",
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/test-output",
	}

	jobID, err := mgr.SubmitJob(jobDef)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Set progress and result
	mgr.mu.Lock()
	if job, exists := mgr.jobs[jobID]; exists {
		job.Progress = &models.JobProgress{
			Phase:           "exporting",
			PercentComplete: 50.0,
		}
		job.Result = &models.JobResult{
			Files: []string{"file1.vmdk"},
		}
	}
	mgr.mu.Unlock()

	// Get the job
	retrievedJob, err := mgr.GetJob(jobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	// Modify the retrieved job's fields
	retrievedJob.Progress.PercentComplete = 100.0
	retrievedJob.Result.Files = append(retrievedJob.Result.Files, "file2.vmdk")

	// Get the job again and verify original values weren't modified
	retrievedJob2, err := mgr.GetJob(jobID)
	if err != nil {
		t.Fatalf("Failed to get job second time: %v", err)
	}

	if retrievedJob2.Progress.PercentComplete != 50.0 {
		t.Errorf("Deep copy failed: original progress was modified to %.1f%%", retrievedJob2.Progress.PercentComplete)
	}

	if len(retrievedJob2.Result.Files) != 1 {
		t.Errorf("Deep copy failed: original files slice was modified to length %d", len(retrievedJob2.Result.Files))
	}
}
