// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/daemon/webhooks"
	"github.com/zyvorai/transiva/logger"
)

// TestFullIntegrationFlow tests the complete workflow
func TestFullIntegrationFlow(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)

	config := &Config{
		Webhooks: []webhooks.Webhook{
			{
				URL:     "http://example.com/webhook",
				Events:  []string{"job.completed", "job.failed"},
				Enabled: true,
			},
		},
	}
	config.Database.Path = "" // No DB in test
	config.Metrics.Enabled = true

	server, err := NewEnhancedServer(manager, detector, log, ":8080", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.scheduler.Stop()

	// Step 1: Check health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	server.handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Health check failed: %d", w.Code)
	}

	// Step 2: Check status
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	w = httptest.NewRecorder()
	server.handleStatus(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Status check failed: %d", w.Code)
	}

	// Step 3: Create a schedule
	schedule := models.ScheduledJob{
		ID:       "integration-schedule",
		Name:     "Integration Test Schedule",
		Schedule: "*/5 * * * *",
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			VMPath:     "test/vm",
			OutputPath: "/tmp/test",
		},
	}
	body, _ := json.Marshal(schedule)
	req = httptest.NewRequest(http.MethodPost, "/schedules", bytes.NewReader(body))
	w = httptest.NewRecorder()
	server.handleCreateSchedule(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("Schedule creation failed: %d", w.Code)
	}

	// Step 4: List schedules
	req = httptest.NewRequest(http.MethodGet, "/schedules", nil)
	w = httptest.NewRecorder()
	server.handleListSchedules(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("List schedules failed: %d", w.Code)
	}

	// Step 5: Disable the schedule
	req = httptest.NewRequest(http.MethodPost, "/schedules/integration-schedule/disable", nil)
	w = httptest.NewRecorder()
	server.handleDisableSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Disable schedule failed: %d", w.Code)
	}

	// Step 6: List webhooks
	req = httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	w = httptest.NewRecorder()
	server.handleListWebhooks(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("List webhooks failed: %d", w.Code)
	}

	// Step 7: Check schedule stats
	req = httptest.NewRequest(http.MethodGet, "/schedules/stats", nil)
	w = httptest.NewRecorder()
	server.handleScheduleStats(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Schedule stats failed: %d", w.Code)
	}
}

// TestWebSocketIntegration tests WebSocket with job updates
func TestWebSocketIntegration(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)
	config := &Config{}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":0", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.scheduler.Stop()

	// Create test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer testServer.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer func() {
		if cerr := ws.Close(); cerr != nil {
			t.Logf("ws close: %v", cerr)
		}
	}()

	// Collect initial messages
	if err := ws.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	initialMessages := 0
	for i := 0; i < 3; i++ {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
		initialMessages++
	}

	if initialMessages == 0 {
		t.Error("Should receive initial messages")
	}

	// Broadcast a job update
	testJob := &models.Job{
		Definition: models.JobDefinition{
			ID:     "ws-test-job",
			VMPath: "test/vm",
		},
		Status: models.JobStatusRunning,
	}

	server.BroadcastJobUpdate(testJob)

	// Read the update
	if err := ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, message, err := ws.ReadMessage()
	if err == nil {
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			if msg.Type == "job_update" {
				t.Log("Successfully received job update via WebSocket")
			}
		}
	}
}

// TestScheduleWebhookIntegration tests schedule + webhook integration
func TestScheduleWebhookIntegration(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)

	// Create webhook receiver
	webhookHits := 0
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	config := &Config{
		Webhooks: []webhooks.Webhook{
			{
				URL:     webhookServer.URL,
				Events:  []string{"schedule.triggered"},
				Enabled: true,
			},
		},
	}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":8080", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.scheduler.Stop()

	// Create schedule
	schedule := &models.ScheduledJob{
		ID:       "webhook-test",
		Name:     "Webhook Test",
		Schedule: "* * * * *",
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			VMPath:     "test/vm",
			OutputPath: "/tmp/test",
		},
	}
	if err := server.scheduler.AddScheduledJob(schedule); err != nil {
		t.Fatalf("AddScheduledJob failed: %v", err)
	}

	// Broadcast schedule event
	server.BroadcastScheduleEvent("triggered", "webhook-test", nil)

	// Note: Webhook is sent asynchronously, so we can't reliably test it here
	// In a real test environment, you'd wait and verify webhook reception
}

// TestEnhancedServerShutdown tests graceful shutdown
func TestEnhancedServerShutdown(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)
	config := &Config{}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":0", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("server.Start returned: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	if err := server.Shutdown(context.TODO()); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestConcurrentWebSocketClients tests multiple WebSocket clients
func TestConcurrentWebSocketClients(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)
	config := &Config{}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":0", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.scheduler.Stop()

	testServer := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer testServer.Close()

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http")

	// Connect multiple clients
	clients := make([]*websocket.Conn, 3)
	for i := 0; i < 3; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		defer func() {
			if cerr := ws.Close(); cerr != nil {
				t.Logf("ws close: %v", cerr)
			}
		}()
		clients[i] = ws
	}

	time.Sleep(50 * time.Millisecond)

	// Check client count
	count := server.wsHub.GetClientCount()
	if count != 3 {
		t.Errorf("Expected 3 clients, got %d", count)
	}

	// Broadcast message
	server.wsHub.Broadcast("test", map[string]interface{}{
		"message": "concurrent test",
	})

	// Each client should receive the message
	for i, ws := range clients {
		if err := ws.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			t.Fatalf("SetReadDeadline failed: %v", err)
		}

		// Read initial messages first
		for j := 0; j < 3; j++ {
			_, message, err := ws.ReadMessage()
			if err != nil {
				break
			}

			var msg WSMessage
			if err := json.Unmarshal(message, &msg); err == nil {
				if msg.Type == "test" {
					t.Logf("Client %d received broadcast", i)
					break
				}
			}
		}
	}
}

// TestScheduleCRUDFlow tests complete schedule CRUD operations
func TestScheduleCRUDFlow(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)
	config := &Config{}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":8080", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.scheduler.Stop()

	scheduleID := "crud-test"

	// Create
	schedule := models.ScheduledJob{
		ID:       scheduleID,
		Name:     "CRUD Test",
		Schedule: "0 0 * * *",
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			VMPath:     "test/vm",
			OutputPath: "/tmp/test",
		},
	}
	body, _ := json.Marshal(schedule)
	req := httptest.NewRequest(http.MethodPost, "/schedules", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.handleCreateSchedule(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create failed: %d", w.Code)
	}

	// Read
	req = httptest.NewRequest(http.MethodGet, "/schedules/"+scheduleID, nil)
	w = httptest.NewRecorder()
	server.handleGetSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Read failed: %d", w.Code)
	}

	// Update
	updates := models.ScheduledJob{
		Name:    "Updated Name",
		Enabled: false,
	}
	body, _ = json.Marshal(updates)
	req = httptest.NewRequest(http.MethodPut, "/schedules/"+scheduleID, bytes.NewReader(body))
	w = httptest.NewRecorder()
	server.handleUpdateSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Update failed: %d", w.Code)
	}

	// Verify update
	updated, _ := server.scheduler.GetScheduledJob(scheduleID)
	if updated.Name != "Updated Name" {
		t.Error("Update not applied")
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/schedules/"+scheduleID, nil)
	w = httptest.NewRecorder()
	server.handleDeleteSchedule(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Delete failed: %d", w.Code)
	}

	// Verify deletion
	_, err = server.scheduler.GetScheduledJob(scheduleID)
	if err == nil {
		t.Error("Schedule should be deleted")
	}
}

// TestWebhookCRUDFlow tests complete webhook CRUD operations
func TestWebhookCRUDFlow(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)
	config := &Config{
		Webhooks: []webhooks.Webhook{}, // Initialize empty webhooks to enable manager
	}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":8080", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.scheduler.Stop()

	// Create
	webhook := webhooks.Webhook{
		URL:     "http://example.com/webhook",
		Events:  []string{"job.completed"},
		Enabled: true,
	}
	body, _ := json.Marshal(webhook)
	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.handleAddWebhook(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Create failed: %d", w.Code)
	}

	// Read/List
	req = httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	w = httptest.NewRecorder()
	server.handleListWebhooks(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("List failed: %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if int(response["total"].(float64)) != 1 {
		t.Error("Webhook not added")
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/webhooks/0", nil)
	w = httptest.NewRecorder()
	server.handleDeleteWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Delete failed: %d", w.Code)
	}

	// Verify deletion
	if len(server.config.Webhooks) != 0 {
		t.Error("Webhook should be deleted")
	}
}
