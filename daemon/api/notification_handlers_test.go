// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Get Notification Config Handler Tests

func TestHandleGetNotificationConfigMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/notifications/config", nil)
	w := httptest.NewRecorder()

	server.handleGetNotificationConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetNotificationConfig(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/notifications/config", nil)
	w := httptest.NewRecorder()

	server.handleGetNotificationConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response NotificationConfig
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify email config
	if !response.Email.Enabled {
		t.Error("Expected email to be enabled")
	}
	if response.Email.SMTPServer != "smtp.gmail.com" {
		t.Errorf("Expected SMTP server=smtp.gmail.com, got %s", response.Email.SMTPServer)
	}
	if response.Email.Port != 587 {
		t.Errorf("Expected port=587, got %d", response.Email.Port)
	}
	if !response.Email.OnCompletion {
		t.Error("Expected on_completion to be enabled")
	}
	if !response.Email.OnFailure {
		t.Error("Expected on_failure to be enabled")
	}

	// Verify other providers are disabled by default
	if response.Slack.Enabled {
		t.Error("Expected Slack to be disabled by default")
	}
	if response.Webhook.Enabled {
		t.Error("Expected webhook to be disabled by default")
	}
}

// Update Notification Config Handler Tests

func TestHandleUpdateNotificationConfigMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/notifications/config", nil)
	w := httptest.NewRecorder()

	server.handleUpdateNotificationConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleUpdateNotificationConfigInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPut, "/notifications/config",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleUpdateNotificationConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleUpdateNotificationConfigValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := NotificationConfig{
		Email: EmailConfig{
			Enabled:      true,
			SMTPServer:   "smtp.example.com",
			Port:         587,
			From:         "noreply@example.com",
			To:           []string{"admin@example.com"},
			OnCompletion: true,
			OnFailure:    true,
		},
		Slack: SlackConfig{
			Enabled:    true,
			WebhookURL: "https://hooks.slack.com/services/XXX",
			Channel:    "#alerts",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/notifications/config",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleUpdateNotificationConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status=success, got %s", response["status"])
	}
	if !strings.Contains(response["message"], "Notification configuration updated") {
		t.Errorf("Expected success message, got %s", response["message"])
	}
}

func TestHandleUpdateNotificationConfigDifferentProviders(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name   string
		config NotificationConfig
	}{
		{
			"EmailOnly",
			NotificationConfig{
				Email: EmailConfig{
					Enabled:    true,
					SMTPServer: "smtp.example.com",
					Port:       587,
				},
			},
		},
		{
			"SlackOnly",
			NotificationConfig{
				Slack: SlackConfig{
					Enabled:    true,
					WebhookURL: "https://hooks.slack.com/services/XXX",
					Channel:    "#alerts",
				},
			},
		},
		{
			"WebhookOnly",
			NotificationConfig{
				Webhook: WebhookConfig{
					Enabled: true,
					URL:     "https://example.com/webhook",
					Events:  []string{"migration_complete", "migration_failed"},
				},
			},
		},
		{
			"SMSOnly",
			NotificationConfig{
				SMS: SMSConfig{
					Enabled:  true,
					Provider: "twilio",
					Numbers:  []string{"+1234567890"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.config)
			req := httptest.NewRequest(http.MethodPut, "/notifications/config",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleUpdateNotificationConfig(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

// List Alert Rules Handler Tests

func TestHandleListAlertRulesMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/notifications/alerts", nil)
	w := httptest.NewRecorder()

	server.handleListAlertRules(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListAlertRules(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/notifications/alerts", nil)
	w := httptest.NewRecorder()

	server.handleListAlertRules(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["rules"]; !ok {
		t.Error("Expected rules field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 2 {
		t.Errorf("Expected total=2, got %v", total)
	}

	// Verify rule structure
	rules := response["rules"].([]interface{})
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}

	// Check first rule
	rule1 := rules[0].(map[string]interface{})
	if rule1["id"] != "rule-1" {
		t.Errorf("Expected id=rule-1, got %v", rule1["id"])
	}
	if rule1["name"] != "Storage Almost Full" {
		t.Errorf("Expected name='Storage Almost Full', got %v", rule1["name"])
	}
	if rule1["condition"] != "storage_usage" {
		t.Errorf("Expected condition=storage_usage, got %v", rule1["condition"])
	}
	if rule1["threshold"].(float64) != 90 {
		t.Errorf("Expected threshold=90, got %v", rule1["threshold"])
	}
	if !rule1["enabled"].(bool) {
		t.Error("Expected rule to be enabled")
	}
	actions := rule1["actions"].([]interface{})
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(actions))
	}

	// Check second rule with complex threshold
	rule2 := rules[1].(map[string]interface{})
	if rule2["name"] != "Job Failure Rate High" {
		t.Errorf("Expected name='Job Failure Rate High', got %v", rule2["name"])
	}
	threshold := rule2["threshold"].(map[string]interface{})
	if threshold["count"].(float64) != 5 {
		t.Errorf("Expected threshold count=5, got %v", threshold["count"])
	}
	if threshold["period"] != "1h" {
		t.Errorf("Expected threshold period=1h, got %v", threshold["period"])
	}
}

// Create Alert Rule Handler Tests

func TestHandleCreateAlertRuleMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/notifications/alerts", nil)
	w := httptest.NewRecorder()

	server.handleCreateAlertRule(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateAlertRuleInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/notifications/alerts",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateAlertRule(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateAlertRuleValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := AlertRule{
		Name:      "High CPU Usage",
		Condition: "cpu_usage",
		Threshold: 80,
		Actions:   []string{"email", "slack"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/notifications/alerts",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateAlertRule(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response AlertRule
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Name != "High CPU Usage" {
		t.Errorf("Expected name='High CPU Usage', got %s", response.Name)
	}
	if response.Condition != "cpu_usage" {
		t.Errorf("Expected condition=cpu_usage, got %s", response.Condition)
	}
	if response.Threshold.(float64) != 80 {
		t.Errorf("Expected threshold=80, got %v", response.Threshold)
	}
	if !response.Enabled {
		t.Error("Expected rule to be enabled by default")
	}
	if response.ID == "" {
		t.Error("Expected ID to be auto-generated")
	}
	if !strings.HasPrefix(response.ID, "rule-") {
		t.Errorf("Expected ID to start with 'rule-', got %s", response.ID)
	}
}

func TestHandleCreateAlertRuleDifferentConditions(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name      string
		rule      AlertRule
		threshold interface{}
	}{
		{
			"CPUUsage",
			AlertRule{
				Name:      "CPU Alert",
				Condition: "cpu_usage",
				Threshold: 90,
				Actions:   []string{"email"},
			},
			float64(90),
		},
		{
			"MemoryUsage",
			AlertRule{
				Name:      "Memory Alert",
				Condition: "memory_usage",
				Threshold: 85,
				Actions:   []string{"slack"},
			},
			float64(85),
		},
		{
			"ComplexThreshold",
			AlertRule{
				Name:      "Failure Rate Alert",
				Condition: "failure_rate",
				Threshold: map[string]interface{}{"rate": 10, "window": "5m"},
				Actions:   []string{"email", "webhook"},
			},
			nil, // Will check map separately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.rule)
			req := httptest.NewRequest(http.MethodPost, "/notifications/alerts",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleCreateAlertRule(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", w.Code)
			}

			var response AlertRule
			_ = json.Unmarshal(w.Body.Bytes(), &response)

			if response.Name != tt.rule.Name {
				t.Errorf("Expected name=%s, got %s", tt.rule.Name, response.Name)
			}
			if !response.Enabled {
				t.Error("Expected rule to be enabled")
			}
		})
	}
}

// Notification Test Webhook Handler Tests (Server.handleTestWebhook in notification_handlers.go)
// Note: This is different from EnhancedServer.handleTestWebhook tested in webhooks_test.go

func TestHandleNotificationTestWebhookMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/notifications/webhook/test", nil)
	w := httptest.NewRecorder()

	server.handleTestWebhook(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleNotificationTestWebhookInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/notifications/webhook/test",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleTestWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleNotificationTestWebhookValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"url": "https://example.com/webhook",
		"headers": map[string]string{
			"Authorization": "Bearer token123",
			"Content-Type":  "application/json",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/notifications/webhook/test",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleTestWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status=success, got %v", response["status"])
	}
	if !strings.Contains(response["message"].(string), "Test webhook sent successfully") {
		t.Errorf("Expected success message, got %v", response["message"])
	}
	if _, ok := response["payload"]; !ok {
		t.Error("Expected payload field in response")
	}

	// Verify test payload structure
	payload := response["payload"].(map[string]interface{})
	if payload["event"] != "test" {
		t.Errorf("Expected event=test, got %v", payload["event"])
	}
	if payload["message"] != "This is a test webhook from HyperSDK" {
		t.Errorf("Expected test message, got %v", payload["message"])
	}
	if _, ok := payload["timestamp"]; !ok {
		t.Error("Expected timestamp in payload")
	}
}
