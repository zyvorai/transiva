// SPDX-License-Identifier: Apache-2.0

package models

import (
	"testing"
	"time"
)

func TestJobStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
		want   string
	}{
		{"Pending", JobStatusPending, "pending"},
		{"Running", JobStatusRunning, "running"},
		{"Completed", JobStatusCompleted, "completed"},
		{"Failed", JobStatusFailed, "failed"},
		{"Cancelled", JobStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("JobStatus = %v, want %v", tt.status, tt.want)
			}
		})
	}
}

func TestExportOptionsToVSphereOptions(t *testing.T) {
	opts := &ExportOptions{
		ParallelDownloads:      8,
		RemoveCDROM:            true,
		ShowIndividualProgress: true,
	}

	vsOpts := opts.ToVSphereOptions("/tmp/export")

	if vsOpts.OutputPath != "/tmp/export" {
		t.Errorf("Expected OutputPath '/tmp/export', got '%s'", vsOpts.OutputPath)
	}
	if vsOpts.ParallelDownloads != 8 {
		t.Errorf("Expected ParallelDownloads 8, got %d", vsOpts.ParallelDownloads)
	}
	if !vsOpts.RemoveCDROM {
		t.Error("Expected RemoveCDROM to be true")
	}
	if !vsOpts.ShowIndividualProgress {
		t.Error("Expected ShowIndividualProgress to be true")
	}
}

func TestExportOptionsDefaults(t *testing.T) {
	// Nil options should use defaults
	var opts *ExportOptions
	vsOpts := opts.ToVSphereOptions("/tmp/test")

	if vsOpts.OutputPath != "/tmp/test" {
		t.Errorf("Expected OutputPath '/tmp/test', got '%s'", vsOpts.OutputPath)
	}
	// Check default values are set by DefaultExportOptions
	if vsOpts.ParallelDownloads == 0 {
		t.Error("Expected default ParallelDownloads to be set")
	}
}

func TestJobCreation(t *testing.T) {
	def := JobDefinition{
		ID:         "test-job-1",
		Name:       "Test Job",
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/export",
		CreatedAt:  time.Now(),
	}

	job := &Job{
		Definition: def,
		Status:     JobStatusPending,
		UpdatedAt:  time.Now(),
	}

	if job.Status != JobStatusPending {
		t.Errorf("Expected status pending, got %s", job.Status)
	}
	if job.Definition.ID != "test-job-1" {
		t.Errorf("Expected job ID 'test-job-1', got '%s'", job.Definition.ID)
	}
}

func TestQueryRequest(t *testing.T) {
	req := &QueryRequest{
		JobIDs: []string{"job1", "job2"},
		Status: []JobStatus{JobStatusRunning, JobStatusCompleted},
		All:    false,
		Limit:  10,
	}

	if len(req.JobIDs) != 2 {
		t.Errorf("Expected 2 job IDs, got %d", len(req.JobIDs))
	}
	if len(req.Status) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(req.Status))
	}
	if req.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", req.Limit)
	}
}

func TestJobDefinition_Redacted(t *testing.T) {
	tests := []struct {
		name     string
		jobDef   JobDefinition
		wantPass string
	}{
		{
			name: "password is redacted",
			jobDef: JobDefinition{
				ID:         "job-1",
				Name:       "Test Job",
				VCenterURL: "vcenter.example.com",
				Username:   "admin",
				Password:   "SuperSecret123!",
				VMPath:     "/datacenter/vm/test",
				Datacenter: "dc1",
			},
			wantPass: "***REDACTED***",
		},
		{
			name: "empty password remains empty",
			jobDef: JobDefinition{
				ID:         "job-2",
				Name:       "Test Job 2",
				VCenterURL: "vcenter.example.com",
				Username:   "admin",
				Password:   "",
				VMPath:     "/datacenter/vm/test2",
			},
			wantPass: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redacted := tt.jobDef.Redacted()

			// Verify password is redacted correctly
			if redacted.Password != tt.wantPass {
				t.Errorf("Expected password '%s', got '%s'", tt.wantPass, redacted.Password)
			}

			// Verify other fields are unchanged
			if redacted.ID != tt.jobDef.ID {
				t.Errorf("Expected ID '%s', got '%s'", tt.jobDef.ID, redacted.ID)
			}
			if redacted.Name != tt.jobDef.Name {
				t.Errorf("Expected Name '%s', got '%s'", tt.jobDef.Name, redacted.Name)
			}
			if redacted.VCenterURL != tt.jobDef.VCenterURL {
				t.Errorf("Expected VCenterURL '%s', got '%s'", tt.jobDef.VCenterURL, redacted.VCenterURL)
			}
			if redacted.Username != tt.jobDef.Username {
				t.Errorf("Expected Username '%s', got '%s'", tt.jobDef.Username, redacted.Username)
			}
			if redacted.VMPath != tt.jobDef.VMPath {
				t.Errorf("Expected VMPath '%s', got '%s'", tt.jobDef.VMPath, redacted.VMPath)
			}
			if redacted.Datacenter != tt.jobDef.Datacenter {
				t.Errorf("Expected Datacenter '%s', got '%s'", tt.jobDef.Datacenter, redacted.Datacenter)
			}

			// Verify original is not modified
			if tt.jobDef.Password == "SuperSecret123!" && tt.jobDef.Password != "SuperSecret123!" {
				t.Error("Original JobDefinition was modified")
			}
		})
	}
}

func TestJobDefinition_RedactedDoesNotModifyOriginal(t *testing.T) {
	original := JobDefinition{
		ID:       "job-original",
		Password: "MyPassword123",
		VMPath:   "/vm/test",
	}

	originalPassword := original.Password

	// Call Redacted
	redacted := original.Redacted()

	// Original should be unchanged
	if original.Password != originalPassword {
		t.Errorf("Original password was modified: expected '%s', got '%s'", originalPassword, original.Password)
	}

	// Redacted should have password masked
	if redacted.Password != "***REDACTED***" {
		t.Errorf("Redacted password incorrect: expected '***REDACTED***', got '%s'", redacted.Password)
	}
}
