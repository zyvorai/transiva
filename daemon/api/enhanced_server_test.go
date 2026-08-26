// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"testing"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/webhooks"
	"github.com/zyvorai/transiva/logger"
)

func TestEnhancedServerCreation(t *testing.T) {
	// Create test logger
	log := logger.New("info")

	// Create capability detector
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)

	// Create job manager
	manager := jobs.NewManager(log, detector)

	// Create test config
	config := &Config{
		Webhooks: []webhooks.Webhook{
			{
				URL:     "http://example.com/webhook",
				Events:  []string{"job.completed"},
				Enabled: true,
			},
		},
	}
	config.Database.Path = "" // Don't use database in test
	config.Metrics.Enabled = true
	config.Metrics.Port = 8080

	// Create enhanced server
	server, err := NewEnhancedServer(manager, detector, log, ":8080", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create enhanced server: %v", err)
	}

	// Verify components are initialized
	if server.scheduler == nil {
		t.Error("Scheduler not initialized")
	}

	if server.webhookMgr == nil {
		t.Error("Webhook manager not initialized")
	}

	if server.wsHub == nil {
		t.Error("WebSocket hub not initialized")
	}

	if server.config == nil {
		t.Error("Config not set")
	}

	// Verify WebSocket hub is running
	clientCount := server.wsHub.GetClientCount()
	if clientCount != 0 {
		t.Errorf("Expected 0 clients, got %d", clientCount)
	}

	// Clean up
	server.scheduler.Stop()
}

func TestJobExecutorAdapter(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)

	adapter := &jobExecutorAdapter{manager: manager}

	// This should not panic
	_ = adapter
}

func TestWSHub(t *testing.T) {
	hub := NewWSHub()

	// Start hub in background
	ctx := context.Background()
	go hub.Run(ctx)

	// Test client count
	count := hub.GetClientCount()
	if count != 0 {
		t.Errorf("Expected 0 clients, got %d", count)
	}

	// Test broadcast (should not panic with no clients)
	hub.Broadcast("test", map[string]interface{}{
		"message": "test message",
	})
}
