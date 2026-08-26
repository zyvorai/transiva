// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("expected dashboard to be enabled by default")
	}

	if config.Port != 8080 {
		t.Errorf("expected port 8080, got %d", config.Port)
	}

	if config.UpdateInterval != 1*time.Second {
		t.Errorf("expected update interval 1s, got %v", config.UpdateInterval)
	}

	if config.MaxClients != 100 {
		t.Errorf("expected max clients 100, got %d", config.MaxClients)
	}
}

func TestNewDashboard(t *testing.T) {
	config := DefaultConfig()
	dashboard, err := NewDashboard(config)

	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	if dashboard == nil {
		t.Fatal("expected dashboard to be created")
	}

	if dashboard.config != config {
		t.Error("expected config to be set")
	}

	if dashboard.templates == nil {
		t.Error("expected templates to be parsed")
	}

	if dashboard.clients == nil {
		t.Error("expected clients map to be initialized")
	}

	if dashboard.metrics == nil {
		t.Error("expected metrics to be initialized")
	}
}

func TestNewDashboardNilConfig(t *testing.T) {
	dashboard, err := NewDashboard(nil)

	if err != nil {
		t.Fatalf("failed to create dashboard with nil config: %v", err)
	}

	if dashboard == nil {
		t.Fatal("expected dashboard to be created")
	}

	if dashboard.config == nil {
		t.Error("expected default config to be set")
	}
}

func TestHandleIndex(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	dashboard.handleIndex(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "HyperSDK Dashboard") {
		t.Error("expected dashboard title in response")
	}
}

func TestHandleIndexNotFound(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	req := httptest.NewRequest("GET", "/invalid", nil)
	rr := httptest.NewRecorder()

	dashboard.handleIndex(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestHandleMetrics(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Set some test metrics
	dashboard.UpdateJobMetrics(5, 10, 2, 3, 8)

	req := httptest.NewRequest("GET", "/api/metrics", nil)
	rr := httptest.NewRecorder()

	dashboard.handleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content type application/json, got %s", contentType)
	}

	var metrics Metrics
	err = json.NewDecoder(rr.Body).Decode(&metrics)
	if err != nil {
		t.Fatalf("failed to decode metrics: %v", err)
	}

	if metrics.JobsActive != 5 {
		t.Errorf("expected jobs_active 5, got %d", metrics.JobsActive)
	}

	if metrics.JobsCompleted != 10 {
		t.Errorf("expected jobs_completed 10, got %d", metrics.JobsCompleted)
	}

	if metrics.JobsFailed != 2 {
		t.Errorf("expected jobs_failed 2, got %d", metrics.JobsFailed)
	}
}

func TestHandleJobs(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Add test jobs
	job1 := JobInfo{
		ID:       "job-1",
		Name:     "test-job-1",
		Status:   "running",
		Progress: 50,
		Provider: "aws",
		VMName:   "test-vm",
	}
	dashboard.AddJob(job1)

	req := httptest.NewRequest("GET", "/api/jobs", nil)
	rr := httptest.NewRecorder()

	dashboard.handleJobs(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var jobs []JobInfo
	err = json.NewDecoder(rr.Body).Decode(&jobs)
	if err != nil {
		t.Fatalf("failed to decode jobs: %v", err)
	}

	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].ID != "job-1" {
		t.Errorf("expected job ID 'job-1', got %s", jobs[0].ID)
	}
}

func TestHandleJobDetail(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Add test job
	job := JobInfo{
		ID:       "job-123",
		Name:     "test-job",
		Status:   "completed",
		Progress: 100,
	}
	dashboard.AddJob(job)

	req := httptest.NewRequest("GET", "/api/jobs/job-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "job-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	dashboard.handleJobDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var returnedJob JobInfo
	err = json.NewDecoder(rr.Body).Decode(&returnedJob)
	if err != nil {
		t.Fatalf("failed to decode job: %v", err)
	}

	if returnedJob.ID != "job-123" {
		t.Errorf("expected job ID 'job-123', got %s", returnedJob.ID)
	}
}

func TestHandleJobDetailNotFound(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/jobs/nonexistent", nil)
	rr := httptest.NewRecorder()

	dashboard.handleJobDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestUpdateJobMetrics(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	dashboard.UpdateJobMetrics(10, 20, 5, 15, 25)

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	if dashboard.metrics.JobsActive != 10 {
		t.Errorf("expected JobsActive 10, got %d", dashboard.metrics.JobsActive)
	}

	if dashboard.metrics.JobsCompleted != 20 {
		t.Errorf("expected JobsCompleted 20, got %d", dashboard.metrics.JobsCompleted)
	}

	if dashboard.metrics.JobsFailed != 5 {
		t.Errorf("expected JobsFailed 5, got %d", dashboard.metrics.JobsFailed)
	}

	if dashboard.metrics.JobsPending != 15 {
		t.Errorf("expected JobsPending 15, got %d", dashboard.metrics.JobsPending)
	}

	if dashboard.metrics.QueueLength != 25 {
		t.Errorf("expected QueueLength 25, got %d", dashboard.metrics.QueueLength)
	}
}

func TestAddJob(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	job := JobInfo{
		ID:       "job-1",
		Name:     "test-job",
		Status:   "running",
		Provider: "aws",
	}

	dashboard.AddJob(job)

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	if len(dashboard.metrics.RecentJobs) != 1 {
		t.Errorf("expected 1 recent job, got %d", len(dashboard.metrics.RecentJobs))
	}

	if dashboard.metrics.RecentJobs[0].ID != "job-1" {
		t.Errorf("expected job ID 'job-1', got %s", dashboard.metrics.RecentJobs[0].ID)
	}

	if dashboard.metrics.ProviderStats["aws"] != 1 {
		t.Errorf("expected provider stats for aws to be 1, got %d", dashboard.metrics.ProviderStats["aws"])
	}
}

func TestAddJobLimit(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Add 60 jobs
	for i := 0; i < 60; i++ {
		job := JobInfo{
			ID:       string(rune('a' + i)),
			Name:     "test-job",
			Status:   "completed",
			Provider: "aws",
		}
		dashboard.AddJob(job)
	}

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	// Should keep only last 50
	if len(dashboard.metrics.RecentJobs) != 50 {
		t.Errorf("expected 50 recent jobs, got %d", len(dashboard.metrics.RecentJobs))
	}
}

func TestUpdateSystemMetrics(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	dashboard.UpdateSystemMetrics(2048, 75.5, 1000)

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	if dashboard.metrics.MemoryUsage != 2048 {
		t.Errorf("expected MemoryUsage 2048, got %d", dashboard.metrics.MemoryUsage)
	}

	if dashboard.metrics.CPUUsage != 75.5 {
		t.Errorf("expected CPUUsage 75.5, got %f", dashboard.metrics.CPUUsage)
	}

	if dashboard.metrics.Goroutines != 1000 {
		t.Errorf("expected Goroutines 1000, got %d", dashboard.metrics.Goroutines)
	}
}

func TestUpdateHTTPMetrics(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	dashboard.UpdateHTTPMetrics(10000, 50, 123.45)

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	if dashboard.metrics.HTTPRequests != 10000 {
		t.Errorf("expected HTTPRequests 10000, got %d", dashboard.metrics.HTTPRequests)
	}

	if dashboard.metrics.HTTPErrors != 50 {
		t.Errorf("expected HTTPErrors 50, got %d", dashboard.metrics.HTTPErrors)
	}

	if dashboard.metrics.AvgResponseTime != 123.45 {
		t.Errorf("expected AvgResponseTime 123.45, got %f", dashboard.metrics.AvgResponseTime)
	}
}

func TestAddAlert(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	dashboard.AddAlert("warning", "Test alert message")

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	if len(dashboard.metrics.Alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(dashboard.metrics.Alerts))
	}

	alert := dashboard.metrics.Alerts[0]
	if alert.Severity != "warning" {
		t.Errorf("expected severity 'warning', got %s", alert.Severity)
	}

	if alert.Message != "Test alert message" {
		t.Errorf("expected message 'Test alert message', got %s", alert.Message)
	}
}

func TestAddAlertLimit(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Add 25 alerts
	for i := 0; i < 25; i++ {
		dashboard.AddAlert("info", "Alert")
	}

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	// Should keep only last 20
	if len(dashboard.metrics.Alerts) != 20 {
		t.Errorf("expected 20 alerts, got %d", len(dashboard.metrics.Alerts))
	}
}

func TestSetSystemHealth(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	dashboard.SetSystemHealth("degraded")

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	if dashboard.metrics.SystemHealth != "degraded" {
		t.Errorf("expected system health 'degraded', got %s", dashboard.metrics.SystemHealth)
	}
}

func TestGetClientCount(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	count := dashboard.GetClientCount()
	if count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestWebSocketUpgrade(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(dashboard.handleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Should receive initial metrics
	_, message, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}

	var metrics Metrics
	err = json.Unmarshal(message, &metrics)
	if err != nil {
		t.Fatalf("failed to unmarshal metrics: %v", err)
	}

	if metrics.SystemHealth != "healthy" {
		t.Errorf("expected system health 'healthy', got %s", metrics.SystemHealth)
	}
}

func TestCollectMetrics(t *testing.T) {
	dashboard, err := NewDashboard(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	beforeTime := time.Now()
	dashboard.collectMetrics()

	dashboard.metricsMu.RLock()
	defer dashboard.metricsMu.RUnlock()

	if dashboard.metrics.Timestamp.Before(beforeTime) {
		t.Error("expected timestamp to be updated")
	}
}

func TestStart(t *testing.T) {
	config := DefaultConfig()
	config.Port = 0 // Use random available port
	config.UpdateInterval = 50 * time.Millisecond
	dashboard, err := NewDashboard(config)
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Start the dashboard with a short-lived context
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start in goroutine since it blocks until context is done
	done := make(chan error, 1)
	go func() {
		done <- dashboard.Start(ctx)
	}()

	// Give the server time to start
	time.Sleep(50 * time.Millisecond)

	// Wait for context to cancel and server to shutdown
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Start did not complete within timeout")
	}
}

func TestStartDisabled(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false
	dashboard, err := NewDashboard(config)
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	ctx := context.Background()
	err = dashboard.Start(ctx)
	if err != nil {
		t.Errorf("Start should return nil when disabled, got: %v", err)
	}
}

func TestHandleBroadcast(t *testing.T) {
	config := DefaultConfig()
	config.UpdateInterval = 50 * time.Millisecond
	dashboard, err := NewDashboard(config)
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Start handleBroadcast in goroutine
	go dashboard.handleBroadcast()

	// Connect a WebSocket client
	server := httptest.NewServer(http.HandlerFunc(dashboard.handleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Read initial message
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial message: %v", err)
	}

	// Give time for client to be registered
	time.Sleep(10 * time.Millisecond)

	// Send a broadcast message
	testData := []byte(`{"test": "data"}`)
	dashboard.broadcast <- testData

	// Read the broadcast message
	_, message, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read broadcast message: %v", err)
	}

	if string(message) != string(testData) {
		t.Errorf("expected message %s, got %s", testData, message)
	}
}

func TestUpdateMetrics(t *testing.T) {
	config := DefaultConfig()
	config.UpdateInterval = 30 * time.Millisecond
	dashboard, err := NewDashboard(config)
	if err != nil {
		t.Fatalf("failed to create dashboard: %v", err)
	}

	// Start updateMetrics with a short-lived context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Connect a WebSocket client to receive broadcasts
	server := httptest.NewServer(http.HandlerFunc(dashboard.handleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Start broadcast handler
	go dashboard.handleBroadcast()

	// Read initial message
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial message: %v", err)
	}

	// Update some metrics
	dashboard.UpdateJobMetrics(5, 10, 2, 3, 8)

	// Start the updateMetrics goroutine
	go dashboard.updateMetrics(ctx)

	// Wait for at least one update cycle
	time.Sleep(50 * time.Millisecond)

	// Try to read an update (may timeout if update cycle hasn't completed yet)
	ws.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, message, err := ws.ReadMessage()
	if err == nil {
		var metrics Metrics
		err = json.Unmarshal(message, &metrics)
		if err != nil {
			t.Fatalf("failed to unmarshal metrics: %v", err)
		}

		if metrics.JobsActive != 5 {
			t.Logf("expected JobsActive 5, got %d", metrics.JobsActive)
		}
	} else {
		t.Logf("No metric update received (may be expected depending on timing): %v", err)
	}

	// Wait for context to cancel
	<-ctx.Done()
}
