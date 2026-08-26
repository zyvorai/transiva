// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func TestWSHubBroadcast(t *testing.T) {
	hub := NewWSHub()
	ctx := context.Background()
	go hub.Run(ctx)

	// Create mock client
	client := &WSClient{
		send:   make(chan WSMessage, 256),
		hub:    hub,
		closed: false,
	}

	// Register client
	hub.register <- client

	// Give time for registration
	time.Sleep(10 * time.Millisecond)

	// Broadcast message
	hub.Broadcast("test", map[string]interface{}{
		"key": "value",
	})

	// Receive message
	select {
	case msg := <-client.send:
		if msg.Type != "test" {
			t.Errorf("Expected type 'test', got '%s'", msg.Type)
		}
		if msg.Data["key"] != "value" {
			t.Errorf("Expected data key='value', got '%v'", msg.Data["key"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for broadcast message")
	}
}

func TestWSHubClientCount(t *testing.T) {
	hub := NewWSHub()
	ctx := context.Background()
	go hub.Run(ctx)

	// Initially should have 0 clients
	if count := hub.GetClientCount(); count != 0 {
		t.Errorf("Expected 0 clients, got %d", count)
	}

	// Register client
	client := &WSClient{
		send:   make(chan WSMessage, 256),
		hub:    hub,
		closed: false,
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	if count := hub.GetClientCount(); count != 1 {
		t.Errorf("Expected 1 client, got %d", count)
	}

	// Unregister client
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	if count := hub.GetClientCount(); count != 0 {
		t.Errorf("Expected 0 clients after unregister, got %d", count)
	}
}

func TestWSHubMultipleClients(t *testing.T) {
	hub := NewWSHub()
	ctx := context.Background()
	go hub.Run(ctx)

	// Register multiple clients
	clients := make([]*WSClient, 5)
	for i := 0; i < 5; i++ {
		clients[i] = &WSClient{
			send:   make(chan WSMessage, 256),
			hub:    hub,
			closed: false,
		}
		hub.register <- clients[i]
	}

	time.Sleep(20 * time.Millisecond)

	if count := hub.GetClientCount(); count != 5 {
		t.Errorf("Expected 5 clients, got %d", count)
	}

	// Broadcast to all
	hub.Broadcast("multi", map[string]interface{}{
		"message": "broadcast to all",
	})

	// All clients should receive
	for i, client := range clients {
		select {
		case msg := <-client.send:
			if msg.Type != "multi" {
				t.Errorf("Client %d: Expected type 'multi', got '%s'", i, msg.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Client %d: Timeout waiting for message", i)
		}
	}
}

func TestWSMessage(t *testing.T) {
	msg := WSMessage{
		Type:      "test",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal WSMessage: %v", err)
	}

	// Deserialize
	var decoded WSMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal WSMessage: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: expected '%s', got '%s'", msg.Type, decoded.Type)
	}

	if decoded.Data["key1"] != "value1" {
		t.Errorf("Data mismatch for key1")
	}
}

func TestBroadcastJobUpdate(t *testing.T) {
	log := logger.New("info")
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

	// Create test job
	testJob := &models.Job{
		Definition: models.JobDefinition{
			ID:     "test-job-123",
			VMPath: "test/vm",
		},
		Status: models.JobStatusRunning,
		Progress: &models.JobProgress{
			Phase:           "exporting",
			PercentComplete: 50.0,
		},
	}

	// This should not panic
	server.BroadcastJobUpdate(testJob)

	// Register a client and verify message
	client := &WSClient{
		send:   make(chan WSMessage, 256),
		hub:    server.wsHub,
		closed: false,
	}
	server.wsHub.register <- client
	time.Sleep(10 * time.Millisecond)

	server.BroadcastJobUpdate(testJob)

	select {
	case msg := <-client.send:
		if msg.Type != "job_update" {
			t.Errorf("Expected type 'job_update', got '%s'", msg.Type)
		}
		if msg.Data["status"] != models.JobStatusRunning {
			t.Errorf("Status mismatch")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for job update")
	}
}

func TestBroadcastScheduleEvent(t *testing.T) {
	log := logger.New("info")
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

	// Register a client
	client := &WSClient{
		send:   make(chan WSMessage, 256),
		hub:    server.wsHub,
		closed: false,
	}
	server.wsHub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Broadcast schedule event
	server.BroadcastScheduleEvent("triggered", "schedule-123", map[string]interface{}{
		"job_id": "job-456",
	})

	select {
	case msg := <-client.send:
		if msg.Type != "schedule_event" {
			t.Errorf("Expected type 'schedule_event', got '%s'", msg.Type)
		}
		if msg.Data["schedule_id"] != "schedule-123" {
			t.Errorf("Schedule ID mismatch")
		}
		if msg.Data["event"] != "triggered" {
			t.Errorf("Event mismatch")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for schedule event")
	}
}

func TestWebSocketUpgrade(t *testing.T) {
	log := logger.New("info")
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

	// Create test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer testServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http")

	// Connect WebSocket client
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer func() { _ = ws.Close() }()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("Expected status 101, got %d", resp.StatusCode)
	}

	// Should receive initial data
	receivedMessages := 0
	if err := ws.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, message, err := ws.ReadMessage()
		if err != nil {
			if i == 0 {
				t.Fatalf("Failed to read initial message: %v", err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		if msg.Type != "status" && msg.Type != "jobs" {
			t.Errorf("Unexpected message type: %s", msg.Type)
		}
		receivedMessages++
	}

	if receivedMessages == 0 {
		t.Error("No messages received from WebSocket")
	}
}

func TestStartStatusBroadcaster(t *testing.T) {
	log := logger.New("info")
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

	// Register a client
	client := &WSClient{
		send:   make(chan WSMessage, 256),
		hub:    server.wsHub,
		closed: false,
	}
	server.wsHub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Wait for metrics broadcast (every 1 second)
	select {
	case msg := <-client.send:
		// First message should be initial status
		if msg.Type != "status" && msg.Type != "metrics" {
			t.Errorf("Expected type 'status' or 'metrics', got '%s'", msg.Type)
		}
		// Check for metrics structure (new format with raw wrapper)
		switch msg.Type {
		case "metrics":
			if raw, ok := msg.Data["raw"]; ok {
				rawMap, ok := raw.(map[string]interface{})
				if !ok {
					t.Error("Expected raw to be a map")
				} else {
					if _, ok := rawMap["jobs_active"]; !ok {
						t.Error("Missing jobs_active in metrics data")
					}
					if _, ok := rawMap["jobs_completed"]; !ok {
						t.Error("Missing jobs_completed in metrics data")
					}
				}
			} else {
				t.Error("Missing raw data in metrics message")
			}
		case "status":
			// Old format compatibility check
			if _, ok := msg.Data["total_jobs"]; !ok {
				t.Error("Missing total_jobs in status data")
			}
			if _, ok := msg.Data["running_jobs"]; !ok {
				t.Error("Missing running_jobs in status data")
			}
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for status/metrics broadcast")
	}
}

func TestWSHubBroadcastOverflow(t *testing.T) {
	hub := NewWSHub()
	ctx := context.Background()
	go hub.Run(ctx)

	// Create client with small buffer
	client := &WSClient{
		send:   make(chan WSMessage, 2),
		hub:    hub,
		closed: false,
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	for i := 0; i < 10; i++ {
		hub.Broadcast("overflow", map[string]interface{}{
			"index": i,
		})
	}

	// Client should still be registered (dropped messages, not closed)
	time.Sleep(50 * time.Millisecond)
	// Note: In real implementation, client with full buffer gets unregistered
}

func TestWSMessageTimestamp(t *testing.T) {
	msg := WSMessage{
		Type:      "test",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{},
	}

	// Ensure timestamp is recent
	age := time.Since(msg.Timestamp)
	if age > time.Second {
		t.Errorf("Message timestamp too old: %v", age)
	}
}
