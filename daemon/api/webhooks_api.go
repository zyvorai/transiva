// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/transiva/daemon/webhooks"
)

// Handle GET /webhooks - List all configured webhooks
func (es *EnhancedServer) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.webhookMgr == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "webhooks not enabled")
		return
	}

	webhooks := es.config.Webhooks
	es.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"webhooks":  webhooks,
		"total":     len(webhooks),
		"timestamp": time.Now(),
	})
}

// Handle POST /webhooks - Add new webhook
func (es *EnhancedServer) handleAddWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.webhookMgr == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "webhooks not enabled")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		es.errorResponse(w, http.StatusBadRequest, "failed to read request body: %v", err)
		return
	}

	var webhook webhooks.Webhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}

	// Validate required fields
	if webhook.URL == "" {
		es.errorResponse(w, http.StatusBadRequest, "webhook URL is required")
		return
	}

	// Validate webhook URL for security (SSRF prevention)
	if err := ValidateWebhookURL(webhook.URL, es.config.Security.BlockPrivateIPs); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid webhook URL: %v", err)
		es.logger.Warn("webhook URL validation failed", "url", webhook.URL, "error", err)
		return
	}

	// Add to config and reinitialize webhook manager
	es.config.Webhooks = append(es.config.Webhooks, webhook)
	es.webhookMgr = webhooks.NewManager(es.config.Webhooks, es.logger)

	es.logger.Info("webhook added", "url", webhook.URL)
	es.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"message": "webhook added successfully",
		"webhook": webhook,
	})
}

// Handle DELETE /webhooks/{index} - Delete webhook by index
func (es *EnhancedServer) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.webhookMgr == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "webhooks not enabled")
		return
	}

	// Extract index from path
	indexStr := strings.TrimPrefix(r.URL.Path, "/webhooks/")
	if indexStr == "" {
		es.errorResponse(w, http.StatusBadRequest, "webhook index is required")
		return
	}

	var index int
	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid index: %v", err)
		return
	}

	if index < 0 || index >= len(es.config.Webhooks) {
		es.errorResponse(w, http.StatusNotFound, "webhook index out of range")
		return
	}

	// Remove webhook
	removed := es.config.Webhooks[index]
	es.config.Webhooks = append(es.config.Webhooks[:index], es.config.Webhooks[index+1:]...)
	es.webhookMgr = webhooks.NewManager(es.config.Webhooks, es.logger)

	es.logger.Info("webhook deleted", "url", removed.URL)
	es.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "webhook deleted successfully",
	})
}

// Handle POST /webhooks/test - Test webhook delivery
func (es *EnhancedServer) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.webhookMgr == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "webhooks not enabled")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		es.errorResponse(w, http.StatusBadRequest, "failed to read request body: %v", err)
		return
	}

	var req struct {
		URL   string `json:"url"`
		Event string `json:"event"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}

	if req.URL == "" {
		es.errorResponse(w, http.StatusBadRequest, "URL is required")
		return
	}

	// Validate webhook URL for security (SSRF prevention)
	if err := ValidateWebhookURL(req.URL, es.config.Security.BlockPrivateIPs); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid webhook URL: %v", err)
		es.logger.Warn("test webhook URL validation failed", "url", req.URL, "error", err)
		return
	}

	if req.Event == "" {
		req.Event = "test"
	}

	// Create temporary webhook for testing
	testWebhook := webhooks.Webhook{
		URL:     req.URL,
		Events:  []string{req.Event},
		Enabled: true,
	}

	testMgr := webhooks.NewManager([]webhooks.Webhook{testWebhook}, es.logger)
	testMgr.Send(req.Event, map[string]interface{}{
		"message": "This is a test webhook",
		"test":    true,
	})

	es.logger.Info("test webhook sent", "url", req.URL, "event", req.Event)
	es.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "test webhook sent",
	})
}
