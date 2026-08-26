// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func setupTestBasicServer(t *testing.T) *Server {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)

	return NewServer(manager, detector, log, ":8080", nil, nil)
}

func TestHandleSubmitJob(t *testing.T) {
	server := setupTestBasicServer(t)

	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test-vm",
		OutputPath: "/tmp/output",
	}

	body, _ := json.Marshal(jobDef)
	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleSubmitJob(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Accepted != 1 {
		t.Errorf("Expected 1 accepted job, got %d", resp.Accepted)
	}

	if len(resp.JobIDs) != 1 {
		t.Errorf("Expected 1 job ID, got %d", len(resp.JobIDs))
	}
}

func TestHandleSubmitJobMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	w := httptest.NewRecorder()

	server.handleSubmitJob(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleSubmitJobInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleSubmitJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSubmitJobBatch(t *testing.T) {
	server := setupTestBasicServer(t)

	jobs := []models.JobDefinition{
		{
			VMPath:     "/datacenter/vm/vm1",
			OutputPath: "/tmp/output1",
		},
		{
			VMPath:     "/datacenter/vm/vm2",
			OutputPath: "/tmp/output2",
		},
	}

	body, _ := json.Marshal(jobs)
	req := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleSubmitJob(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Accepted != 2 {
		t.Errorf("Expected 2 accepted jobs, got %d", resp.Accepted)
	}
}

func TestHandleQueryJobs(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a test job first
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	_, _ = server.manager.SubmitJob(jobDef)

	// Query all jobs with GET
	req := httptest.NewRequest(http.MethodGet, "/jobs?all=true", nil)
	w := httptest.NewRecorder()

	server.handleQueryJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.QueryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 job, got %d", resp.Total)
	}
}

func TestHandleQueryJobsWithPOST(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit test jobs
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	_, _ = server.manager.SubmitJob(jobDef)

	// Query with POST
	queryReq := models.QueryRequest{
		All: true,
	}
	body, _ := json.Marshal(queryReq)
	req := httptest.NewRequest(http.MethodPost, "/jobs/query", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleQueryJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.QueryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total < 1 {
		t.Errorf("Expected at least 1 job, got %d", resp.Total)
	}
}

func TestHandleQueryJobsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/jobs/query", nil)
	w := httptest.NewRecorder()

	server.handleQueryJobs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleQueryJobsInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/jobs/query", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	server.handleQueryJobs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleQueryJobsByIDs(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit test job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	// Query by ID
	queryReq := models.QueryRequest{
		JobIDs: []string{jobID},
	}
	body, _ := json.Marshal(queryReq)
	req := httptest.NewRequest(http.MethodPost, "/jobs/query", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleQueryJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.QueryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 job, got %d", resp.Total)
	}

	if len(resp.Jobs) > 0 && resp.Jobs[0].Definition.ID != jobID {
		t.Errorf("Expected job ID %s, got %s", jobID, resp.Jobs[0].Definition.ID)
	}
}

func TestHandleQueryJobsByStatus(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit test job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	_, _ = server.manager.SubmitJob(jobDef)

	// Wait for job to process
	time.Sleep(500 * time.Millisecond)

	// Query by status (jobs fail validation immediately, so query for failed status)
	queryReq := models.QueryRequest{
		Status: []models.JobStatus{models.JobStatusFailed},
		Limit:  10,
	}
	body, _ := json.Marshal(queryReq)
	req := httptest.NewRequest(http.MethodPost, "/jobs/query", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleQueryJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.QueryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have at least 1 failed job
	if resp.Total < 1 {
		t.Errorf("Expected at least 1 failed job, got %d", resp.Total)
	}
}

func TestHandleGetJob(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a test job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	// Get the job
	req := httptest.NewRequest(http.MethodGet, "/jobs/"+jobID, nil)
	w := httptest.NewRecorder()

	server.handleGetJob(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var job models.Job
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatalf("Failed to parse job: %v", err)
	}

	if job.Definition.ID != jobID {
		t.Errorf("Expected job ID %s, got %s", jobID, job.Definition.ID)
	}
}

func TestHandleGetJobNotFound(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/nonexistent-job-id", nil)
	w := httptest.NewRecorder()

	server.handleGetJob(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleGetJobMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/jobs/some-id", nil)
	w := httptest.NewRecorder()

	server.handleGetJob(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetJobMissingID(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/", nil)
	w := httptest.NewRecorder()

	server.handleGetJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCancelJobs(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a test job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	// Wait a moment for job to be registered
	time.Sleep(10 * time.Millisecond)

	// Cancel the job
	cancelReq := models.CancelRequest{
		JobIDs: []string{jobID},
	}
	body, _ := json.Marshal(cancelReq)
	req := httptest.NewRequest(http.MethodPost, "/jobs/cancel", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCancelJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.CancelResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Job should be cancelled (unless it already completed)
	totalProcessed := len(resp.Cancelled) + len(resp.Failed)
	if totalProcessed != 1 {
		t.Errorf("Expected 1 job processed, got %d", totalProcessed)
	}
}

func TestHandleCancelJobsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/cancel", nil)
	w := httptest.NewRecorder()

	server.handleCancelJobs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCancelJobsInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/jobs/cancel", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	server.handleCancelJobs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCancelJobsNonexistent(t *testing.T) {
	server := setupTestBasicServer(t)

	cancelReq := models.CancelRequest{
		JobIDs: []string{"nonexistent-job"},
	}
	body, _ := json.Marshal(cancelReq)
	req := httptest.NewRequest(http.MethodPost, "/jobs/cancel", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCancelJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.CancelResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Nonexistent job should fail
	if len(resp.Failed) != 1 {
		t.Errorf("Expected 1 failed cancel, got %d", len(resp.Failed))
	}
}

func TestHandleHealth(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", resp["status"])
	}
}

func TestHandleHealthMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleStatus(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a test job to have some stats
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	_, _ = server.manager.SubmitJob(jobDef)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()

	server.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.DaemonStatus
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.TotalJobs < 1 {
		t.Errorf("Expected at least 1 job, got %d", resp.TotalJobs)
	}
}

func TestHandleStatusMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/status", nil)
	w := httptest.NewRecorder()

	server.handleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestParseJSONJobsSingleJob(t *testing.T) {
	server := setupTestBasicServer(t)

	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}

	data, _ := json.Marshal(jobDef)
	var defs []models.JobDefinition

	err := server.parseJSONJobs(data, &defs)
	if err != nil {
		t.Fatalf("parseJSONJobs failed: %v", err)
	}

	if len(defs) != 1 {
		t.Errorf("Expected 1 job definition, got %d", len(defs))
	}

	if defs[0].VMPath != "/datacenter/vm/test" {
		t.Errorf("VMPath mismatch: got %s", defs[0].VMPath)
	}
}

func TestParseJSONJobsArray(t *testing.T) {
	server := setupTestBasicServer(t)

	jobs := []models.JobDefinition{
		{VMPath: "/datacenter/vm/vm1", OutputPath: "/tmp/out1"},
		{VMPath: "/datacenter/vm/vm2", OutputPath: "/tmp/out2"},
	}

	data, _ := json.Marshal(jobs)
	var defs []models.JobDefinition

	err := server.parseJSONJobs(data, &defs)
	if err != nil {
		t.Fatalf("parseJSONJobs failed: %v", err)
	}

	if len(defs) != 2 {
		t.Errorf("Expected 2 job definitions, got %d", len(defs))
	}
}

func TestParseJSONJobsBatch(t *testing.T) {
	server := setupTestBasicServer(t)

	batch := models.BatchJobDefinition{
		Jobs: []models.JobDefinition{
			{VMPath: "/datacenter/vm/vm1", OutputPath: "/tmp/out1"},
			{VMPath: "/datacenter/vm/vm2", OutputPath: "/tmp/out2"},
		},
	}

	data, _ := json.Marshal(batch)
	var defs []models.JobDefinition

	err := server.parseJSONJobs(data, &defs)
	if err != nil {
		t.Fatalf("parseJSONJobs failed: %v", err)
	}

	if len(defs) != 2 {
		t.Errorf("Expected 2 job definitions, got %d", len(defs))
	}
}

func TestParseJSONJobsInvalid(t *testing.T) {
	server := setupTestBasicServer(t)

	var defs []models.JobDefinition
	err := server.parseJSONJobs([]byte("{}"), &defs)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestParseYAMLJobsSingleJob(t *testing.T) {
	server := setupTestBasicServer(t)

	yamlData := []byte(`
vm_path: /datacenter/vm/test
output_path: /tmp/output
`)

	var defs []models.JobDefinition
	err := server.parseYAMLJobs(yamlData, &defs)
	if err != nil {
		t.Fatalf("parseYAMLJobs failed: %v", err)
	}

	if len(defs) != 1 {
		t.Errorf("Expected 1 job definition, got %d", len(defs))
	}

	if defs[0].VMPath != "/datacenter/vm/test" {
		t.Errorf("VMPath mismatch: got %s", defs[0].VMPath)
	}
}

func TestParseYAMLJobsInvalid(t *testing.T) {
	server := setupTestBasicServer(t)

	var defs []models.JobDefinition
	err := server.parseYAMLJobs([]byte("{}"), &defs)

	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}
