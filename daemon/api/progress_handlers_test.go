// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/transiva/daemon/models"
)

func TestHandleGetJobProgress(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a test job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	req := httptest.NewRequest(http.MethodGet, "/jobs/progress/"+jobID, nil)
	w := httptest.NewRecorder()

	server.handleGetJobProgress(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["job_id"] != jobID {
		t.Errorf("Expected job_id %s, got %v", jobID, response["job_id"])
	}

	if response["status"] == nil {
		t.Error("Expected status field in response")
	}
}

func TestHandleGetJobProgressMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/jobs/progress/some-id", nil)
	w := httptest.NewRecorder()

	server.handleGetJobProgress(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetJobProgressMissingID(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/progress", nil)
	w := httptest.NewRecorder()

	server.handleGetJobProgress(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetJobProgressNotFound(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/progress/nonexistent-job", nil)
	w := httptest.NewRecorder()

	server.handleGetJobProgress(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleGetJobProgressWithProgress(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	// Note: We can't easily manipulate job progress without running an actual export,
	// but we can verify the endpoint works with a basic job
	req := httptest.NewRequest(http.MethodGet, "/jobs/progress/"+jobID, nil)
	w := httptest.NewRecorder()

	server.handleGetJobProgress(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify basic fields are present
	if response["job_id"] != jobID {
		t.Errorf("Expected job_id %s, got %v", jobID, response["job_id"])
	}

	if response["status"] == nil {
		t.Error("Expected status field in response")
	}

	if response["timestamp"] == nil {
		t.Error("Expected timestamp field in response")
	}
}

func TestHandleGetJobLogs(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a test job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	req := httptest.NewRequest(http.MethodGet, "/jobs/logs/"+jobID, nil)
	w := httptest.NewRecorder()

	server.handleGetJobLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["job_id"] != jobID {
		t.Errorf("Expected job_id %s, got %v", jobID, response["job_id"])
	}

	logs, ok := response["logs"].([]interface{})
	if !ok {
		t.Fatal("Expected logs array in response")
	}

	if len(logs) == 0 {
		t.Error("Expected at least one log entry")
	}
}

func TestHandleGetJobLogsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/jobs/logs/some-id", nil)
	w := httptest.NewRecorder()

	server.handleGetJobLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetJobLogsMissingID(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/logs", nil)
	w := httptest.NewRecorder()

	server.handleGetJobLogs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetJobLogsNotFound(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/logs/nonexistent-job", nil)
	w := httptest.NewRecorder()

	server.handleGetJobLogs(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleGetJobLogsWithJobData(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	req := httptest.NewRequest(http.MethodGet, "/jobs/logs/"+jobID, nil)
	w := httptest.NewRecorder()

	server.handleGetJobLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	logs, ok := response["logs"].([]interface{})
	if !ok {
		t.Fatal("Expected logs array in response")
	}

	// Should have at least the creation log
	if len(logs) < 1 {
		t.Errorf("Expected at least 1 log entry, got %d", len(logs))
	}

	// Verify log structure
	if len(logs) > 0 {
		firstLog, ok := logs[0].(map[string]interface{})
		if !ok {
			t.Fatal("Expected log entry to be a map")
		}

		if firstLog["message"] == nil {
			t.Error("Expected log entry to have a message")
		}

		if firstLog["level"] == nil {
			t.Error("Expected log entry to have a level")
		}
	}
}

func TestHandleGetJobETA(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a test job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	req := httptest.NewRequest(http.MethodGet, "/jobs/eta/"+jobID, nil)
	w := httptest.NewRecorder()

	server.handleGetJobETA(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["job_id"] != jobID {
		t.Errorf("Expected job_id %s, got %v", jobID, response["job_id"])
	}
}

func TestHandleGetJobETAMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/jobs/eta/some-id", nil)
	w := httptest.NewRecorder()

	server.handleGetJobETA(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetJobETAMissingID(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/eta", nil)
	w := httptest.NewRecorder()

	server.handleGetJobETA(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetJobETANotFound(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/jobs/eta/nonexistent-job", nil)
	w := httptest.NewRecorder()

	server.handleGetJobETA(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleGetJobETAFields(t *testing.T) {
	server := setupTestBasicServer(t)

	// Submit a job
	jobDef := models.JobDefinition{
		VMPath:     "/datacenter/vm/test",
		OutputPath: "/tmp/output",
	}
	jobID, _ := server.manager.SubmitJob(jobDef)

	req := httptest.NewRequest(http.MethodGet, "/jobs/eta/"+jobID, nil)
	w := httptest.NewRecorder()

	server.handleGetJobETA(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response has expected fields
	if response["job_id"] != jobID {
		t.Errorf("Expected job_id %s, got %v", jobID, response["job_id"])
	}

	// The endpoint should return a response even without progress data
	// Check that at least basic fields are present
	if response["timestamp"] == nil {
		t.Error("Expected timestamp field in response")
	}
}
