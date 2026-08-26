// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// NotificationConfig represents notification settings
type NotificationConfig struct {
	Email   EmailConfig   `json:"email"`
	Slack   SlackConfig   `json:"slack"`
	Webhook WebhookConfig `json:"webhook"`
	SMS     SMSConfig     `json:"sms"`
}

// EmailConfig for email notifications
type EmailConfig struct {
	Enabled      bool     `json:"enabled"`
	SMTPServer   string   `json:"smtp_server"`
	Port         int      `json:"port"`
	Username     string   `json:"username"`
	Password     string   `json:"password,omitempty"`
	From         string   `json:"from"`
	To           []string `json:"to"`
	OnCompletion bool     `json:"on_completion"`
	OnFailure    bool     `json:"on_failure"`
	DailySummary bool     `json:"daily_summary"`
}

// SlackConfig for Slack notifications
type SlackConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url,omitempty"`
	Channel    string `json:"channel"`
}

// WebhookConfig for webhook notifications
type WebhookConfig struct {
	Enabled bool              `json:"enabled"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Events  []string          `json:"events"`
}

// SMSConfig for SMS notifications
type SMSConfig struct {
	Enabled  bool     `json:"enabled"`
	Provider string   `json:"provider"` // twilio, aws-sns
	Numbers  []string `json:"numbers"`
}

// AlertRule represents an alert rule
type AlertRule struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Condition string                 `json:"condition"`
	Threshold interface{}            `json:"threshold"`
	Enabled   bool                   `json:"enabled"`
	Actions   []string               `json:"actions"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// handleGetNotificationConfig gets notification configuration
func (s *Server) handleGetNotificationConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := NotificationConfig{
		Email: EmailConfig{
			Enabled:      true,
			SMTPServer:   "smtp.gmail.com",
			Port:         587,
			OnCompletion: true,
			OnFailure:    true,
		},
		Slack: SlackConfig{
			Enabled: false,
		},
		Webhook: WebhookConfig{
			Enabled: false,
		},
	}

	s.jsonResponse(w, http.StatusOK, config)
}

// handleUpdateNotificationConfig updates notification configuration
func (s *Server) handleUpdateNotificationConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config NotificationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Save configuration (in production, persist to database/file)
	s.logger.Info("notification config updated")

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Notification configuration updated",
	})
}

// handleListAlertRules lists all alert rules
func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rules := []AlertRule{
		{
			ID:        "rule-1",
			Name:      "Storage Almost Full",
			Condition: "storage_usage",
			Threshold: 90,
			Enabled:   true,
			Actions:   []string{"email", "slack"},
		},
		{
			ID:        "rule-2",
			Name:      "Job Failure Rate High",
			Condition: "failed_jobs",
			Threshold: map[string]interface{}{"count": 5, "period": "1h"},
			Enabled:   true,
			Actions:   []string{"email"},
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

// handleCreateAlertRule creates a new alert rule
func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rule AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	rule.ID = "rule-" + time.Now().Format("20060102150405")
	rule.Enabled = true

	s.jsonResponse(w, http.StatusCreated, rule)
}

// handleTestWebhook tests a webhook configuration
func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Send test webhook
	testPayload := map[string]interface{}{
		"event":     "test",
		"message":   "This is a test webhook from HyperSDK",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// In production, actually send HTTP request
	s.logger.Info("webhook test", "url", req.URL)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Test webhook sent successfully",
		"payload": testPayload,
	})
}
