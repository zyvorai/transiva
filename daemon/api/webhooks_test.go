// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/webhooks"
	"github.com/zyvorai/transiva/logger"
)

func setupWebhookTestServer(t *testing.T, initialWebhooks []webhooks.Webhook) *EnhancedServer {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)

	config := &Config{
		Webhooks: initialWebhooks,
	}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":8080", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return server
}

func TestHandleListWebhooks(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{
		{
			URL:     "http://example.com/webhook1",
			Events:  []string{"job.completed"},
			Enabled: true,
		},
		{
			URL:     "http://example.com/webhook2",
			Events:  []string{"job.failed"},
			Enabled: false,
		},
	})
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	w := httptest.NewRecorder()

	server.handleListWebhooks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if hooks, ok := response["webhooks"].([]interface{}); ok {
		if len(hooks) != 2 {
			t.Errorf("Expected 2 webhooks, got %d", len(hooks))
		}
	} else {
		t.Error("Response missing webhooks array")
	}

	if total, ok := response["total"].(float64); ok {
		if int(total) != 2 {
			t.Errorf("Expected total=2, got %d", int(total))
		}
	}
}

func TestHandleListWebhooksMethodNotAllowed(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodDelete, "/webhooks", nil)
	w := httptest.NewRecorder()

	server.handleListWebhooks(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleAddWebhook(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	webhook := webhooks.Webhook{
		URL:     "http://example.com/new-webhook",
		Events:  []string{"job.started", "job.completed"},
		Enabled: true,
		Timeout: 30,
	}

	body, _ := json.Marshal(webhook)
	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleAddWebhook(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify webhook was added
	if len(server.config.Webhooks) != 1 {
		t.Errorf("Expected 1 webhook, got %d", len(server.config.Webhooks))
	}

	if server.config.Webhooks[0].URL != "http://example.com/new-webhook" {
		t.Error("Webhook URL mismatch")
	}
}

func TestHandleAddWebhookInvalidJSON(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	server.handleAddWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleAddWebhookMissingURL(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	webhook := webhooks.Webhook{
		// Missing URL
		Events:  []string{"job.completed"},
		Enabled: true,
	}

	body, _ := json.Marshal(webhook)
	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleAddWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeleteWebhook(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{
		{
			URL:     "http://example.com/webhook1",
			Events:  []string{"job.completed"},
			Enabled: true,
		},
		{
			URL:     "http://example.com/webhook2",
			Events:  []string{"job.failed"},
			Enabled: true,
		},
	})
	defer server.scheduler.Stop()

	// Delete webhook at index 0
	req := httptest.NewRequest(http.MethodDelete, "/webhooks/0", nil)
	w := httptest.NewRecorder()

	server.handleDeleteWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deletion
	if len(server.config.Webhooks) != 1 {
		t.Errorf("Expected 1 webhook remaining, got %d", len(server.config.Webhooks))
	}

	if server.config.Webhooks[0].URL != "http://example.com/webhook2" {
		t.Error("Wrong webhook deleted")
	}
}

func TestHandleDeleteWebhookInvalidIndex(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodDelete, "/webhooks/invalid", nil)
	w := httptest.NewRecorder()

	server.handleDeleteWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeleteWebhookOutOfRange(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{
		{URL: "http://example.com/webhook", Enabled: true},
	})
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodDelete, "/webhooks/5", nil)
	w := httptest.NewRecorder()

	server.handleDeleteWebhook(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleTestWebhook(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	// Create test webhook server
	testWebhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testWebhookServer.Close()

	testReq := map[string]string{
		"url":   testWebhookServer.URL,
		"event": "test",
	}

	body, _ := json.Marshal(testReq)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/test", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleTestWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleTestWebhookInvalidJSON(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodPost, "/webhooks/test", bytes.NewReader([]byte("bad json")))
	w := httptest.NewRecorder()

	server.handleTestWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleTestWebhookMissingURL(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	testReq := map[string]string{
		// Missing URL
		"event": "test",
	}

	body, _ := json.Marshal(testReq)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/test", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleTestWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleWebhooksNoManager(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	_ = detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)

	// Create server without webhook manager
	server := &EnhancedServer{
		Server:     NewServer(manager, detector, log, ":8080", nil, nil),
		webhookMgr: nil, // No webhook manager
		config:     &Config{},
	}

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	w := httptest.NewRecorder()

	server.handleListWebhooks(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestWebhookManagerReinitialization(t *testing.T) {
	server := setupWebhookTestServer(t, []webhooks.Webhook{})
	defer server.scheduler.Stop()

	// Add first webhook
	webhook1 := webhooks.Webhook{
		URL:     "http://example.com/webhook1",
		Events:  []string{"job.completed"},
		Enabled: true,
	}
	body1, _ := json.Marshal(webhook1)
	req1 := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body1))
	w1 := httptest.NewRecorder()
	server.handleAddWebhook(w1, req1)

	// Add second webhook
	webhook2 := webhooks.Webhook{
		URL:     "http://example.com/webhook2",
		Events:  []string{"job.failed"},
		Enabled: true,
	}
	body2, _ := json.Marshal(webhook2)
	req2 := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body2))
	w2 := httptest.NewRecorder()
	server.handleAddWebhook(w2, req2)

	// Verify both webhooks exist
	if len(server.config.Webhooks) != 2 {
		t.Errorf("Expected 2 webhooks, got %d", len(server.config.Webhooks))
	}

	// Webhook manager should be reinitialized after each add
	if server.webhookMgr == nil {
		t.Error("Webhook manager should not be nil")
	}
}
