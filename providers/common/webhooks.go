// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// WebhookType represents the type of webhook provider
type WebhookType string

const (
	WebhookSlack   WebhookType = "slack"
	WebhookDiscord WebhookType = "discord"
	WebhookGeneric WebhookType = "generic"
	WebhookEmail   WebhookType = "email"
)

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
	Type    WebhookType `json:"type"`
	URL     string      `json:"url"`
	Enabled bool        `json:"enabled"`

	// Event filters
	OnStart    bool `json:"on_start"`
	OnComplete bool `json:"on_complete"`
	OnError    bool `json:"on_error"`
	OnWarning  bool `json:"on_warning"`

	// Slack-specific
	SlackChannel  string `json:"slack_channel,omitempty"`
	SlackUsername string `json:"slack_username,omitempty"`
	SlackIconURL  string `json:"slack_icon_url,omitempty"`

	// Discord-specific
	DiscordUsername  string `json:"discord_username,omitempty"`
	DiscordAvatarURL string `json:"discord_avatar_url,omitempty"`

	// Email-specific
	EmailFrom    string   `json:"email_from,omitempty"`
	EmailTo      []string `json:"email_to,omitempty"`
	EmailSubject string   `json:"email_subject,omitempty"`

	// Retry configuration
	MaxRetries int           `json:"max_retries"`
	RetryDelay time.Duration `json:"retry_delay"`
	Timeout    time.Duration `json:"timeout"`
}

// WebhookEvent represents an event to send via webhook
type WebhookEvent struct {
	EventType string                 `json:"event_type"`
	TaskID    string                 `json:"task_id"`
	VMName    string                 `json:"vm_name"`
	Provider  string                 `json:"provider"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WebhookNotifier sends webhook notifications
type WebhookNotifier struct {
	config *WebhookConfig
	client *http.Client
	logger logger.Logger
}

// NewWebhookNotifier creates a new webhook notifier
func NewWebhookNotifier(config *WebhookConfig, log logger.Logger) *WebhookNotifier {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 5 * time.Second
	}

	return &WebhookNotifier{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: log,
	}
}

// Notify sends a webhook notification
func (wn *WebhookNotifier) Notify(event *WebhookEvent) error {
	if !wn.config.Enabled {
		return nil
	}

	// Check event filters
	if !wn.shouldNotify(event) {
		return nil
	}

	wn.logger.Info("sending webhook notification",
		"type", wn.config.Type,
		"event", event.EventType,
		"task_id", event.TaskID)

	// Prepare payload based on webhook type
	var payload interface{}
	var err error

	switch wn.config.Type {
	case WebhookSlack:
		payload = wn.formatSlackPayload(event)
	case WebhookDiscord:
		payload = wn.formatDiscordPayload(event)
	case WebhookGeneric:
		payload = event
	default:
		return fmt.Errorf("unsupported webhook type: %s", wn.config.Type)
	}

	// Send with retries
	for attempt := 0; attempt <= wn.config.MaxRetries; attempt++ {
		if attempt > 0 {
			wn.logger.Info("retrying webhook",
				"attempt", attempt,
				"max_retries", wn.config.MaxRetries)
			time.Sleep(wn.config.RetryDelay)
		}

		err = wn.sendWebhook(payload)
		if err == nil {
			wn.logger.Info("webhook sent successfully",
				"type", wn.config.Type,
				"event", event.EventType)
			return nil
		}

		wn.logger.Warn("webhook failed",
			"attempt", attempt+1,
			"error", err)
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", wn.config.MaxRetries+1, err)
}

// shouldNotify checks if event should trigger notification
func (wn *WebhookNotifier) shouldNotify(event *WebhookEvent) bool {
	switch event.EventType {
	case "start":
		return wn.config.OnStart
	case "complete":
		return wn.config.OnComplete
	case "error":
		return wn.config.OnError
	case "warning":
		return wn.config.OnWarning
	default:
		return true
	}
}

// sendWebhook sends the actual HTTP request
func (wn *WebhookNotifier) sendWebhook(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, wn.config.URL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "transiva-webhook/1.0")

	resp, err := wn.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// formatSlackPayload formats event as Slack message
func (wn *WebhookNotifier) formatSlackPayload(event *WebhookEvent) map[string]interface{} {
	color := "good"
	if event.EventType == "error" {
		color = "danger"
	} else if event.EventType == "warning" {
		color = "warning"
	}

	username := wn.config.SlackUsername
	if username == "" {
		username = "HyperSDK Migration"
	}

	attachment := map[string]interface{}{
		"color": color,
		"title": fmt.Sprintf("VM Migration: %s", event.VMName),
		"fields": []map[string]interface{}{
			{"title": "Event", "value": event.EventType, "short": true},
			{"title": "Status", "value": event.Status, "short": true},
			{"title": "Task ID", "value": event.TaskID, "short": true},
			{"title": "Provider", "value": event.Provider, "short": true},
		},
		"footer": "HyperSDK",
		"ts":     event.Timestamp.Unix(),
	}

	if event.Message != "" {
		attachment["text"] = event.Message
	}

	if event.Error != "" {
		attachment["fields"] = append(attachment["fields"].([]map[string]interface{}),
			map[string]interface{}{
				"title": "Error",
				"value": event.Error,
				"short": false,
			})
	}

	if event.Duration > 0 {
		attachment["fields"] = append(attachment["fields"].([]map[string]interface{}),
			map[string]interface{}{
				"title": "Duration",
				"value": event.Duration.String(),
				"short": true,
			})
	}

	payload := map[string]interface{}{
		"username":    username,
		"attachments": []interface{}{attachment},
	}

	if wn.config.SlackChannel != "" {
		payload["channel"] = wn.config.SlackChannel
	}

	if wn.config.SlackIconURL != "" {
		payload["icon_url"] = wn.config.SlackIconURL
	}

	return payload
}

// formatDiscordPayload formats event as Discord message
func (wn *WebhookNotifier) formatDiscordPayload(event *WebhookEvent) map[string]interface{} {
	color := 0x00ff00 // Green
	if event.EventType == "error" {
		color = 0xff0000 // Red
	} else if event.EventType == "warning" {
		color = 0xffa500 // Orange
	}

	username := wn.config.DiscordUsername
	if username == "" {
		username = "HyperSDK Migration"
	}

	fields := []map[string]interface{}{
		{"name": "Event", "value": event.EventType, "inline": true},
		{"name": "Status", "value": event.Status, "inline": true},
		{"name": "Task ID", "value": event.TaskID, "inline": true},
		{"name": "Provider", "value": event.Provider, "inline": true},
	}

	if event.Error != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Error",
			"value":  event.Error,
			"inline": false,
		})
	}

	if event.Duration > 0 {
		fields = append(fields, map[string]interface{}{
			"name":   "Duration",
			"value":  event.Duration.String(),
			"inline": true,
		})
	}

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("VM Migration: %s", event.VMName),
		"description": event.Message,
		"color":       color,
		"fields":      fields,
		"footer": map[string]interface{}{
			"text": "HyperSDK",
		},
		"timestamp": event.Timestamp.Format(time.RFC3339),
	}

	payload := map[string]interface{}{
		"username": username,
		"embeds":   []interface{}{embed},
	}

	if wn.config.DiscordAvatarURL != "" {
		payload["avatar_url"] = wn.config.DiscordAvatarURL
	}

	return payload
}

// NotifyStart sends a start notification
func (wn *WebhookNotifier) NotifyStart(taskID, vmName, provider string) error {
	return wn.Notify(&WebhookEvent{
		EventType: "start",
		TaskID:    taskID,
		VMName:    vmName,
		Provider:  provider,
		Status:    "started",
		Message:   fmt.Sprintf("VM migration started for %s", vmName),
		Timestamp: time.Now(),
	})
}

// NotifyComplete sends a completion notification
func (wn *WebhookNotifier) NotifyComplete(taskID, vmName, provider string, duration time.Duration) error {
	return wn.Notify(&WebhookEvent{
		EventType: "complete",
		TaskID:    taskID,
		VMName:    vmName,
		Provider:  provider,
		Status:    "completed",
		Message:   fmt.Sprintf("VM migration completed successfully for %s", vmName),
		Timestamp: time.Now(),
		Duration:  duration,
	})
}

// NotifyError sends an error notification
func (wn *WebhookNotifier) NotifyError(taskID, vmName, provider string, err error) error {
	return wn.Notify(&WebhookEvent{
		EventType: "error",
		TaskID:    taskID,
		VMName:    vmName,
		Provider:  provider,
		Status:    "failed",
		Message:   fmt.Sprintf("VM migration failed for %s", vmName),
		Error:     err.Error(),
		Timestamp: time.Now(),
	})
}

// NotifyWarning sends a warning notification
func (wn *WebhookNotifier) NotifyWarning(taskID, vmName, provider, warning string) error {
	return wn.Notify(&WebhookEvent{
		EventType: "warning",
		TaskID:    taskID,
		VMName:    vmName,
		Provider:  provider,
		Status:    "warning",
		Message:   warning,
		Timestamp: time.Now(),
	})
}

// WebhookManager manages multiple webhook notifiers
type WebhookManager struct {
	notifiers []*WebhookNotifier
	logger    logger.Logger
}

// NewWebhookManager creates a new webhook manager
func NewWebhookManager(configs []*WebhookConfig, log logger.Logger) *WebhookManager {
	var notifiers []*WebhookNotifier
	for _, config := range configs {
		if config.Enabled {
			notifiers = append(notifiers, NewWebhookNotifier(config, log))
		}
	}

	return &WebhookManager{
		notifiers: notifiers,
		logger:    log,
	}
}

// NotifyAll sends notification to all configured webhooks
func (wm *WebhookManager) NotifyAll(event *WebhookEvent) {
	for _, notifier := range wm.notifiers {
		go func(n *WebhookNotifier) {
			if err := n.Notify(event); err != nil {
				wm.logger.Error("webhook notification failed",
					"type", n.config.Type,
					"error", err)
			}
		}(notifier)
	}
}

// NotifyStart sends start notification to all webhooks
func (wm *WebhookManager) NotifyStart(taskID, vmName, provider string) {
	wm.NotifyAll(&WebhookEvent{
		EventType: "start",
		TaskID:    taskID,
		VMName:    vmName,
		Provider:  provider,
		Status:    "started",
		Timestamp: time.Now(),
	})
}

// NotifyComplete sends completion notification to all webhooks
func (wm *WebhookManager) NotifyComplete(taskID, vmName, provider string, duration time.Duration) {
	wm.NotifyAll(&WebhookEvent{
		EventType: "complete",
		TaskID:    taskID,
		VMName:    vmName,
		Provider:  provider,
		Status:    "completed",
		Duration:  duration,
		Timestamp: time.Now(),
	})
}

// NotifyError sends error notification to all webhooks
func (wm *WebhookManager) NotifyError(taskID, vmName, provider string, err error) {
	wm.NotifyAll(&WebhookEvent{
		EventType: "error",
		TaskID:    taskID,
		VMName:    vmName,
		Provider:  provider,
		Status:    "failed",
		Error:     err.Error(),
		Timestamp: time.Now(),
	})
}
