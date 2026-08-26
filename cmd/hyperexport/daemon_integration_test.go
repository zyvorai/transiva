// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func TestNewDaemonClient(t *testing.T) {
	url := "http://localhost:8080"
	client := NewDaemonClient(url, logger.NewTestLogger(t))

	if client == nil {
		t.Fatal("NewDaemonClient returned nil")
	}
	if client.baseURL != url {
		t.Errorf("Expected baseURL %s, got %s", url, client.baseURL)
	}
	if client.httpClient == nil {
		t.Error("HTTP client should be initialized")
	}
	if client.httpClient.Timeout != 30*time.Second {
		t.Error("HTTP client timeout should be 30 seconds")
	}
}

func TestDaemonClient_SubmitExportJob(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/jobs/submit" {
			t.Errorf("Expected path /jobs/submit, got %s", r.URL.Path)
		}

		// Verify request body
		var jobDef models.JobDefinition
		if err := json.NewDecoder(r.Body).Decode(&jobDef); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Send response
		response := map[string]string{"job_id": "job-123"}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	jobID, err := client.SubmitExportJob(ctx, "/datacenter/vm/test", "/exports", "ova", true)
	if err != nil {
		t.Fatalf("SubmitExportJob failed: %v", err)
	}

	if jobID != "job-123" {
		t.Errorf("Expected job ID 'job-123', got %s", jobID)
	}
}

func TestDaemonClient_SubmitExportJob_Error(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	_, err := client.SubmitExportJob(ctx, "/datacenter/vm/test", "/exports", "ova", true)
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestDaemonClient_GetJobStatus(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/jobs/job-123" {
			t.Errorf("Expected path /jobs/job-123, got %s", r.URL.Path)
		}

		job := models.Job{
			Status: models.JobStatusRunning,
			Definition: models.JobDefinition{
				ID:     "job-123",
				VMPath: "/datacenter/vm/test",
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(job)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	job, err := client.GetJobStatus(ctx, "job-123")
	if err != nil {
		t.Fatalf("GetJobStatus failed: %v", err)
	}

	if job == nil {
		t.Fatal("Job should not be nil")
	}
	if job.Definition.ID != "job-123" {
		t.Error("Job ID mismatch")
	}
	if job.Status != models.JobStatusRunning {
		t.Error("Job status should be running")
	}
}

func TestDaemonClient_GetJobStatus_NotFound(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("job not found"))
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	_, err := client.GetJobStatus(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for not found job")
	}
}

func TestDaemonClient_ListJobs(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/jobs/query" {
			t.Errorf("Expected path /jobs/query, got %s", r.URL.Path)
		}

		jobs := []models.Job{
			{
				Status: models.JobStatusCompleted,
				Definition: models.JobDefinition{
					ID:     "job-1",
					VMPath: "/datacenter/vm/vm1",
				},
			},
			{
				Status: models.JobStatusRunning,
				Definition: models.JobDefinition{
					ID:     "job-2",
					VMPath: "/datacenter/vm/vm2",
				},
			},
		}

		response := map[string]interface{}{
			"jobs":  jobs,
			"total": 2,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	jobs, err := client.ListJobs(ctx, "")
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}

func TestDaemonClient_ListJobs_WithStatusFilter(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify status query parameter
		status := r.URL.Query().Get("status")
		if status != "running" {
			t.Errorf("Expected status=running, got %s", status)
		}

		jobs := []models.Job{
			{
				Status: models.JobStatusRunning,
				Definition: models.JobDefinition{
					ID: "job-1",
				},
			},
		}

		response := map[string]interface{}{
			"jobs":  jobs,
			"total": 1,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	jobs, err := client.ListJobs(ctx, "running")
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}
}

func TestDaemonClient_CreateSchedule(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/schedules" {
			t.Errorf("Expected path /schedules, got %s", r.URL.Path)
		}

		// Verify request body
		var schedJob models.ScheduledJob
		if err := json.NewDecoder(r.Body).Decode(&schedJob); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if schedJob.Name != "daily-backup" {
			t.Errorf("Expected schedule name 'daily-backup', got %s", schedJob.Name)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	err := client.CreateSchedule(ctx, "daily-backup", "0 2 * * *", "/datacenter/vm/test", "/exports")
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
}

func TestDaemonClient_CreateSchedule_Error(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid schedule"))
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	err := client.CreateSchedule(ctx, "daily-backup", "invalid", "/vm", "/exports")
	if err == nil {
		t.Error("Expected error for bad request")
	}
}

func TestDaemonClient_GetDaemonHealth(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Expected path /health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	healthy, err := client.GetDaemonHealth(ctx)
	if err != nil {
		t.Fatalf("GetDaemonHealth failed: %v", err)
	}

	if !healthy {
		t.Error("Daemon should be healthy")
	}
}

func TestDaemonClient_GetDaemonHealth_Unhealthy(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	healthy, err := client.GetDaemonHealth(ctx)
	if err != nil {
		t.Fatalf("GetDaemonHealth failed: %v", err)
	}

	if healthy {
		t.Error("Daemon should not be healthy with 503 response")
	}
}

func TestDaemonClient_GetDaemonStatus(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("Expected path /status, got %s", r.URL.Path)
		}

		status := map[string]interface{}{
			"version":     "1.0.0",
			"uptime":      "24h",
			"active_jobs": 5,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	status, err := client.GetDaemonStatus(ctx)
	if err != nil {
		t.Fatalf("GetDaemonStatus failed: %v", err)
	}

	if status == nil {
		t.Fatal("Status should not be nil")
	}
	if status["version"] != "1.0.0" {
		t.Error("Version mismatch")
	}
	if status["uptime"] != "24h" {
		t.Error("Uptime mismatch")
	}
}

func TestDaemonClient_GetDaemonStatus_Error(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	_, err := client.GetDaemonStatus(ctx)
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestDaemonClient_ContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.GetJobStatus(ctx, "job-123")
	if err == nil {
		t.Error("Expected error for context cancellation")
	}
}

func TestDaemonClient_InvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	_, err := client.GetJobStatus(ctx, "job-123")
	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestDaemonClient_EmptyJobsList(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jobs":  []models.Job{},
			"total": 0,
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewDaemonClient(server.URL, logger.NewTestLogger(t))
	ctx := context.Background()

	jobs, err := client.ListJobs(ctx, "")
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs, got %d", len(jobs))
	}
}

func TestDefaultDaemonURL(t *testing.T) {
	if defaultDaemonURL != "http://localhost:8080" {
		t.Errorf("Expected default daemon URL 'http://localhost:8080', got %s", defaultDaemonURL)
	}
}

func TestDaemonClient_BaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantURL string
	}{
		{"default", "http://localhost:8080", "http://localhost:8080"},
		{"custom port", "http://localhost:9090", "http://localhost:9090"},
		{"remote host", "http://daemon.example.com", "http://daemon.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewDaemonClient(tt.url, logger.NewTestLogger(t))
			if client.baseURL != tt.wantURL {
				t.Errorf("Expected baseURL %s, got %s", tt.wantURL, client.baseURL)
			}
		})
	}
}

func TestDaemonClient_HTTPClientTimeout(t *testing.T) {
	client := NewDaemonClient("http://localhost:8080", logger.NewTestLogger(t))

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}
