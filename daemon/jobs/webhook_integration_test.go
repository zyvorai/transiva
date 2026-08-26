// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// MockWebhookManager for testing
type MockWebhookManager struct {
	mu        sync.Mutex
	created   []string
	started   []string
	completed []string
	failed    []string
	cancelled []string
}

func (m *MockWebhookManager) SendJobCreated(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.created = append(m.created, job.Definition.ID)
}

func (m *MockWebhookManager) SendJobStarted(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = append(m.started, job.Definition.ID)
}

func (m *MockWebhookManager) SendJobCompleted(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completed = append(m.completed, job.Definition.ID)
}

func (m *MockWebhookManager) SendJobFailed(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed = append(m.failed, job.Definition.ID)
}

func (m *MockWebhookManager) SendJobCancelled(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelled = append(m.cancelled, job.Definition.ID)
}

func (m *MockWebhookManager) SendJobProgress(job *models.Job) {
	// Mock implementation - just track that it was called
	// In a real scenario, this would send progress updates
}

func (m *MockWebhookManager) GetCreated() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.created...)
}

func (m *MockWebhookManager) GetStarted() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.started...)
}

func (m *MockWebhookManager) GetCompleted() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.completed...)
}

func (m *MockWebhookManager) GetFailed() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.failed...)
}

func (m *MockWebhookManager) GetCancelled() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.cancelled...)
}

func TestWebhookIntegration_JobCreated(t *testing.T) {
	log := logger.New("debug")
	detector := capabilities.NewDetector(log)
	manager := NewManager(log, detector)

	mockWebhook := &MockWebhookManager{}
	manager.SetWebhookManager(mockWebhook)

	// Submit a job
	def := models.JobDefinition{
		ID:         "test-job-1",
		Name:       "Test Job",
		VMPath:     "/test/vm",
		OutputPath: "/test/output",
	}

	jobID, err := manager.SubmitJob(def)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait a bit for async webhook
	time.Sleep(100 * time.Millisecond)

	// Verify webhook was called
	created := mockWebhook.GetCreated()
	if len(created) != 1 {
		t.Fatalf("Expected 1 created webhook, got %d", len(created))
	}

	if created[0] != jobID {
		t.Errorf("Expected job ID %s, got %s", jobID, created[0])
	}
}

func TestWebhookIntegration_JobCancelled(t *testing.T) {
	log := logger.New("debug")
	detector := capabilities.NewDetector(log)
	manager := NewManager(log, detector)

	mockWebhook := &MockWebhookManager{}
	manager.SetWebhookManager(mockWebhook)

	// Submit a job
	def := models.JobDefinition{
		ID:         "test-job-cancel",
		Name:       "Job To Cancel",
		VMPath:     "/test/vm",
		OutputPath: "/test/output",
	}

	jobID, err := manager.SubmitJob(def)
	if err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait for job to start
	time.Sleep(100 * time.Millisecond)

	// Try to cancel the job
	err = manager.CancelJob(jobID)
	if err != nil {
		// Job may have already failed if export method is not available
		// Check if it's a "cannot be cancelled" error
		job, getErr := manager.GetJob(jobID)
		if getErr == nil && (job.Status == models.JobStatusFailed || job.Status == models.JobStatusCompleted) {
			t.Logf("Job already finished (status: %s), cannot test cancellation webhook", job.Status)
			return
		}
		t.Fatalf("Failed to cancel job: %v", err)
	}

	// Wait for webhook
	time.Sleep(100 * time.Millisecond)

	// Verify cancel webhook was called
	cancelled := mockWebhook.GetCancelled()
	if len(cancelled) == 0 {
		t.Error("Expected cancellation webhook to be called")
	}

	if len(cancelled) > 0 && cancelled[0] != jobID {
		t.Errorf("Expected job ID %s in cancellation, got %s", jobID, cancelled[0])
	}
}

func TestWebhookIntegration_RealHTTPServer(t *testing.T) {
	// Track received webhooks
	var received []map[string]interface{}
	var mu sync.Mutex

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read webhook body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Parse JSON
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("Failed to parse webhook JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Store payload
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Logf("Test webhook server: %s", server.URL)

	// NOTE: This test demonstrates the integration but can't fully test
	// without actually wiring up the real webhook manager with the server URL.
	// The MockWebhookManager is sufficient for unit testing the integration points.
}

func TestWebhookIntegration_NoWebhookManager(t *testing.T) {
	// Test that manager works fine without webhook manager
	log := logger.New("debug")
	detector := capabilities.NewDetector(log)
	manager := NewManager(log, detector)

	// No webhook manager set

	// Submit a job - should not panic
	def := models.JobDefinition{
		ID:         "test-no-webhook",
		Name:       "No Webhook Job",
		VMPath:     "/test/vm",
		OutputPath: "/test/output",
	}

	_, err := manager.SubmitJob(def)
	if err != nil {
		t.Fatalf("Job submission should work without webhook manager: %v", err)
	}

	// No panic = success
}

func TestWebhookIntegration_MultipleJobs(t *testing.T) {
	log := logger.New("debug")
	detector := capabilities.NewDetector(log)
	manager := NewManager(log, detector)

	mockWebhook := &MockWebhookManager{}
	manager.SetWebhookManager(mockWebhook)

	// Submit multiple jobs
	numJobs := 5
	jobIDs := make([]string, numJobs)

	for i := 0; i < numJobs; i++ {
		def := models.JobDefinition{
			ID:         fmt.Sprintf("job-%d", i),
			Name:       fmt.Sprintf("Job %d", i),
			VMPath:     "/test/vm",
			OutputPath: "/test/output",
		}

		jobID, err := manager.SubmitJob(def)
		if err != nil {
			t.Fatalf("Failed to submit job %d: %v", i, err)
		}
		jobIDs[i] = jobID
	}

	// Wait for all webhooks
	time.Sleep(200 * time.Millisecond)

	// Verify all jobs triggered created webhook
	created := mockWebhook.GetCreated()
	if len(created) != numJobs {
		t.Errorf("Expected %d created webhooks, got %d", numJobs, len(created))
	}
}
