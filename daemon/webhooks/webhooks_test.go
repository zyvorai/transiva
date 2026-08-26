// SPDX-License-Identifier: Apache-2.0

package webhooks

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func TestNewManager(t *testing.T) {
	log := logger.New("info")
	webhooks := []Webhook{
		{
			URL:     "http://example.com/webhook",
			Events:  []string{EventJobCompleted},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	if manager.client == nil {
		t.Fatal("Manager.client is nil")
	}

	if len(manager.webhooks) != 1 {
		t.Errorf("Expected 1 webhook, got %d", len(manager.webhooks))
	}
}

func TestWebhookIsSubscribed(t *testing.T) {
	tests := []struct {
		name     string
		webhook  Webhook
		event    string
		expected bool
	}{
		{
			name: "Empty events subscribes to all",
			webhook: Webhook{
				Events: []string{},
			},
			event:    EventJobCompleted,
			expected: true,
		},
		{
			name: "Specific event match",
			webhook: Webhook{
				Events: []string{EventJobCompleted, EventJobFailed},
			},
			event:    EventJobCompleted,
			expected: true,
		},
		{
			name: "Specific event no match",
			webhook: Webhook{
				Events: []string{EventJobCompleted},
			},
			event:    EventJobStarted,
			expected: false,
		},
		{
			name: "Wildcard subscribes to all",
			webhook: Webhook{
				Events: []string{"*"},
			},
			event:    EventJobFailed,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.webhook.isSubscribed(tt.event)
			if result != tt.expected {
				t.Errorf("isSubscribed() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSendWebhookBasic(t *testing.T) {
	log := logger.New("info")

	// Create a test server
	var mu sync.Mutex
	var receivedPayload Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Payload
		json.Unmarshal(body, &payload)
		mu.Lock()
		receivedPayload = payload
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCompleted},
			Enabled: true,
			Timeout: 5 * time.Second,
			Retry:   0,
		},
	}

	manager := NewManager(webhooks, log)

	// Send a webhook
	data := map[string]interface{}{
		"job_id":   "test-123",
		"job_name": "Test Job",
	}
	manager.Send(EventJobCompleted, data)

	// Wait for async delivery
	time.Sleep(100 * time.Millisecond)

	// Verify payload
	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventJobCompleted {
		t.Errorf("Expected event %s, got %s", EventJobCompleted, receivedPayload.Event)
	}

	if receivedPayload.Data["job_id"] != "test-123" {
		t.Errorf("Expected job_id 'test-123', got %v", receivedPayload.Data["job_id"])
	}
}

func TestSendWebhookDisabled(t *testing.T) {
	log := logger.New("info")

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCompleted},
			Enabled: false, // Disabled
		},
	}

	manager := NewManager(webhooks, log)
	manager.Send(EventJobCompleted, map[string]interface{}{"test": "data"})

	// Wait to ensure no delivery
	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Errorf("Expected no webhook calls for disabled webhook, got %d", callCount.Load())
	}
}

func TestSendWebhookNotSubscribed(t *testing.T) {
	log := logger.New("info")

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCompleted}, // Only subscribed to completed
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)
	manager.Send(EventJobStarted, map[string]interface{}{"test": "data"})

	// Wait to ensure no delivery
	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Errorf("Expected no webhook calls for unsubscribed event, got %d", callCount.Load())
	}
}

func TestSendWebhookRetry(t *testing.T) {
	log := logger.New("info")

	var attemptCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attemptCount.Add(1)
		if count < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			// Succeed on 3rd attempt
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCompleted},
			Enabled: true,
			Timeout: 5 * time.Second,
			Retry:   3,
		},
	}

	manager := NewManager(webhooks, log)
	manager.Send(EventJobCompleted, map[string]interface{}{"test": "data"})

	// Wait for retries
	time.Sleep(5 * time.Second)

	count := attemptCount.Load()
	if count < 3 {
		t.Errorf("Expected at least 3 attempts, got %d", count)
	}
}

func TestSendWebhookCustomHeaders(t *testing.T) {
	log := logger.New("info")

	var mu sync.Mutex
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedHeaders = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:    server.URL,
			Events: []string{EventJobCompleted},
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"Authorization":   "Bearer secret-token",
			},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)
	manager.Send(EventJobCompleted, map[string]interface{}{"test": "data"})

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Expected custom header 'custom-value', got '%s'", receivedHeaders.Get("X-Custom-Header"))
	}

	if receivedHeaders.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("Expected auth header, got '%s'", receivedHeaders.Get("Authorization"))
	}

	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", receivedHeaders.Get("Content-Type"))
	}

	if receivedHeaders.Get("User-Agent") != "HyperSDK-Webhook/1.0" {
		t.Errorf("Expected User-Agent 'HyperSDK-Webhook/1.0', got '%s'", receivedHeaders.Get("User-Agent"))
	}
}

func TestSendJobCreated(t *testing.T) {
	log := logger.New("info")

	var mu sync.Mutex
	var receivedPayload Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Payload
		json.Unmarshal(body, &payload)
		mu.Lock()
		receivedPayload = payload
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCreated},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	job := &models.Job{
		Definition: models.JobDefinition{
			ID:         "job-123",
			Name:       "Test VM Export",
			VMPath:     "/datacenter/vm/test",
			OutputPath: "/tmp/output",
		},
	}

	manager.SendJobCreated(job)

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventJobCreated {
		t.Errorf("Expected event %s, got %s", EventJobCreated, receivedPayload.Event)
	}

	if receivedPayload.Data["job_id"] != "job-123" {
		t.Errorf("Expected job_id 'job-123', got %v", receivedPayload.Data["job_id"])
	}
}

func TestSendJobStarted(t *testing.T) {
	log := logger.New("info")

	var mu sync.Mutex
	var receivedPayload Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Payload
		json.Unmarshal(body, &payload)
		mu.Lock()
		receivedPayload = payload
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobStarted},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	job := &models.Job{
		Definition: models.JobDefinition{
			ID:     "job-456",
			Name:   "Started Job",
			VMPath: "/vm/test",
		},
	}

	manager.SendJobStarted(job)

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventJobStarted {
		t.Errorf("Expected event %s, got %s", EventJobStarted, receivedPayload.Event)
	}
}

func TestSendJobCompleted(t *testing.T) {
	log := logger.New("info")

	var mu sync.Mutex
	var receivedPayload Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Payload
		json.Unmarshal(body, &payload)
		mu.Lock()
		receivedPayload = payload
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCompleted},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()

	job := &models.Job{
		Definition: models.JobDefinition{
			ID:     "job-789",
			Name:   "Completed Job",
			VMPath: "/vm/test",
		},
		StartedAt:   &startTime,
		CompletedAt: &endTime,
		Result: &models.JobResult{
			OVFPath: "/output/test.ovf",
			Files:   []string{"test.vmdk", "test.ovf"},
		},
	}

	manager.SendJobCompleted(job)

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventJobCompleted {
		t.Errorf("Expected event %s, got %s", EventJobCompleted, receivedPayload.Event)
	}

	if receivedPayload.Data["ovf_path"] != "/output/test.ovf" {
		t.Errorf("Expected ovf_path '/output/test.ovf', got %v", receivedPayload.Data["ovf_path"])
	}

	// Check duration is reasonable (should be ~300 seconds)
	duration := receivedPayload.Data["duration_seconds"].(float64)
	if duration < 290 || duration > 310 {
		t.Errorf("Expected duration around 300 seconds, got %v", duration)
	}
}

func TestSendJobFailed(t *testing.T) {
	log := logger.New("info")

	var mu sync.Mutex
	var receivedPayload Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Payload
		json.Unmarshal(body, &payload)
		mu.Lock()
		receivedPayload = payload
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobFailed},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	job := &models.Job{
		Definition: models.JobDefinition{
			ID:     "job-fail",
			Name:   "Failed Job",
			VMPath: "/vm/test",
		},
		Error: "Connection timeout",
	}

	manager.SendJobFailed(job)

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventJobFailed {
		t.Errorf("Expected event %s, got %s", EventJobFailed, receivedPayload.Event)
	}

	if receivedPayload.Data["error"] != "Connection timeout" {
		t.Errorf("Expected error 'Connection timeout', got %v", receivedPayload.Data["error"])
	}
}

func TestSendJobCancelled(t *testing.T) {
	log := logger.New("info")

	var mu sync.Mutex
	var receivedPayload Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Payload
		json.Unmarshal(body, &payload)
		mu.Lock()
		receivedPayload = payload
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCancelled},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	job := &models.Job{
		Definition: models.JobDefinition{
			ID:     "job-cancel",
			Name:   "Cancelled Job",
			VMPath: "/vm/test",
		},
	}

	manager.SendJobCancelled(job)

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventJobCancelled {
		t.Errorf("Expected event %s, got %s", EventJobCancelled, receivedPayload.Event)
	}
}

func TestSendJobProgress(t *testing.T) {
	log := logger.New("info")

	var mu sync.Mutex
	var receivedPayload Payload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload Payload
		json.Unmarshal(body, &payload)
		mu.Lock()
		receivedPayload = payload
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobProgress},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	job := &models.Job{
		Definition: models.JobDefinition{
			ID:   "job-progress",
			Name: "Progress Job",
		},
		Progress: &models.JobProgress{
			Phase:           "downloading",
			PercentComplete: 50,
			FilesDownloaded: 5,
			TotalFiles:      10,
			BytesDownloaded: 1024 * 1024 * 500,  // 500 MB
			TotalBytes:      1024 * 1024 * 1000, // 1 GB
		},
	}

	manager.SendJobProgress(job)

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedPayload.Event != EventJobProgress {
		t.Errorf("Expected event %s, got %s", EventJobProgress, receivedPayload.Event)
	}

	if receivedPayload.Data["phase"] != "downloading" {
		t.Errorf("Expected phase 'downloading', got %v", receivedPayload.Data["phase"])
	}

	if receivedPayload.Data["percent_complete"] != float64(50) {
		t.Errorf("Expected percent_complete 50, got %v", receivedPayload.Data["percent_complete"])
	}
}

func TestSendJobProgressNil(t *testing.T) {
	log := logger.New("info")

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobProgress},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)

	job := &models.Job{
		Definition: models.JobDefinition{
			ID:   "job-no-progress",
			Name: "No Progress Job",
		},
		Progress: nil, // No progress data
	}

	manager.SendJobProgress(job)

	// Wait to ensure no delivery
	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Errorf("Expected no webhook calls when progress is nil, got %d", callCount.Load())
	}
}

func TestMultipleWebhooks(t *testing.T) {
	log := logger.New("info")

	var wg sync.WaitGroup
	wg.Add(3)

	var counts [3]atomic.Int32

	// Create 3 test servers
	servers := make([]*httptest.Server, 3)
	for i := 0; i < 3; i++ {
		idx := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counts[idx].Add(1)
			wg.Done()
			w.WriteHeader(http.StatusOK)
		}))
		defer servers[i].Close()
	}

	webhooks := []Webhook{
		{
			URL:     servers[0].URL,
			Events:  []string{EventJobCompleted},
			Enabled: true,
		},
		{
			URL:     servers[1].URL,
			Events:  []string{EventJobCompleted},
			Enabled: true,
		},
		{
			URL:     servers[2].URL,
			Events:  []string{EventJobCompleted},
			Enabled: true,
		},
	}

	manager := NewManager(webhooks, log)
	manager.Send(EventJobCompleted, map[string]interface{}{"test": "data"})

	// Wait for all webhooks
	wg.Wait()

	// Verify all received
	for i := 0; i < 3; i++ {
		if counts[i].Load() != 1 {
			t.Errorf("Webhook %d expected 1 call, got %d", i, counts[i].Load())
		}
	}
}

func TestWebhookTimeout(t *testing.T) {
	log := logger.New("info")

	// Server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // Longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []Webhook{
		{
			URL:     server.URL,
			Events:  []string{EventJobCompleted},
			Enabled: true,
			Timeout: 100 * time.Millisecond, // Short timeout
			Retry:   0,                      // No retries
		},
	}

	manager := NewManager(webhooks, log)

	start := time.Now()
	manager.Send(EventJobCompleted, map[string]interface{}{"test": "data"})

	// Wait for timeout
	time.Sleep(500 * time.Millisecond)

	elapsed := time.Since(start)
	if elapsed > 1*time.Second {
		t.Errorf("Expected webhook to timeout quickly, took %v", elapsed)
	}
}
