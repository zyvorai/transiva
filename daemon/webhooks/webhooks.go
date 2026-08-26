// SPDX-License-Identifier: Apache-2.0

package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// Event types
const (
	EventJobCreated   = "job.created"
	EventJobStarted   = "job.started"
	EventJobCompleted = "job.completed"
	EventJobFailed    = "job.failed"
	EventJobCancelled = "job.cancelled"
	EventJobProgress  = "job.progress"
	EventVMDiscovered = "vm.discovered"
)

// Webhook represents a webhook endpoint configuration
type Webhook struct {
	URL     string            `yaml:"url" json:"url"`
	Events  []string          `yaml:"events" json:"events"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Timeout time.Duration     `yaml:"timeout" json:"timeout"`
	Retry   int               `yaml:"retry" json:"retry"`
	Enabled bool              `yaml:"enabled" json:"enabled"`
}

// Payload represents the webhook payload
type Payload struct {
	Event     string                 `json:"event"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Manager handles webhook delivery
type Manager struct {
	webhooks []Webhook
	client   *http.Client
	log      logger.Logger
}

// NewManager creates a new webhook manager
func NewManager(webhooks []Webhook, log logger.Logger) *Manager {
	return &Manager{
		webhooks: webhooks,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// Send sends a webhook notification
func (m *Manager) Send(event string, data map[string]interface{}) {
	payload := Payload{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	}

	for _, webhook := range m.webhooks {
		if !webhook.Enabled {
			continue
		}

		// Check if webhook is subscribed to this event
		if !webhook.isSubscribed(event) {
			continue
		}

		// Send webhook asynchronously
		go m.sendWebhook(webhook, payload)
	}
}

// sendWebhook sends a single webhook with retry logic
func (m *Manager) sendWebhook(webhook Webhook, payload Payload) {
	maxRetries := webhook.Retry
	if maxRetries == 0 {
		maxRetries = 3 // Default
	}

	timeout := webhook.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, etc.
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			m.log.Info("Retrying webhook delivery",
				"url", webhook.URL,
				"attempt", attempt,
				"backoff", backoff)
			time.Sleep(backoff)
		}

		err := m.deliverWebhook(ctx, webhook, payload)
		if err == nil {
			m.log.Info("Webhook delivered successfully",
				"url", webhook.URL,
				"event", payload.Event)
			return
		}

		lastErr = err
		m.log.Warn("Webhook delivery failed",
			"url", webhook.URL,
			"event", payload.Event,
			"attempt", attempt,
			"error", err)
	}

	m.log.Error("Webhook delivery failed after all retries",
		"url", webhook.URL,
		"event", payload.Event,
		"error", lastErr)
}

// deliverWebhook delivers a webhook to a single endpoint
func (m *Manager) deliverWebhook(ctx context.Context, webhook Webhook, payload Payload) error {
	// Marshal payload
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "HyperSDK-Webhook/1.0")
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// isSubscribed checks if a webhook is subscribed to an event
func (w *Webhook) isSubscribed(event string) bool {
	// Empty events list means subscribe to all
	if len(w.Events) == 0 {
		return true
	}

	for _, e := range w.Events {
		if e == event || e == "*" {
			return true
		}
	}

	return false
}

// SendJobCreated sends a job created event
func (m *Manager) SendJobCreated(job *models.Job) {
	m.Send(EventJobCreated, map[string]interface{}{
		"job_id":      job.Definition.ID,
		"job_name":    job.Definition.Name,
		"vm_path":     job.Definition.VMPath,
		"output_path": job.Definition.OutputPath,
	})
}

// SendJobStarted sends a job started event
func (m *Manager) SendJobStarted(job *models.Job) {
	m.Send(EventJobStarted, map[string]interface{}{
		"job_id":   job.Definition.ID,
		"job_name": job.Definition.Name,
		"vm_path":  job.Definition.VMPath,
	})
}

// SendJobCompleted sends a job completed event
func (m *Manager) SendJobCompleted(job *models.Job) {
	duration := 0.0
	if job.StartedAt != nil && job.CompletedAt != nil {
		duration = job.CompletedAt.Sub(*job.StartedAt).Seconds()
	}

	data := map[string]interface{}{
		"job_id":           job.Definition.ID,
		"job_name":         job.Definition.Name,
		"vm_path":          job.Definition.VMPath,
		"duration_seconds": duration,
	}

	if job.Result != nil {
		data["ovf_path"] = job.Result.OVFPath
		data["exported_files"] = job.Result.Files
	}

	m.Send(EventJobCompleted, data)
}

// SendJobFailed sends a job failed event
func (m *Manager) SendJobFailed(job *models.Job) {
	m.Send(EventJobFailed, map[string]interface{}{
		"job_id":   job.Definition.ID,
		"job_name": job.Definition.Name,
		"vm_path":  job.Definition.VMPath,
		"error":    job.Error,
	})
}

// SendJobCancelled sends a job cancelled event
func (m *Manager) SendJobCancelled(job *models.Job) {
	m.Send(EventJobCancelled, map[string]interface{}{
		"job_id":   job.Definition.ID,
		"job_name": job.Definition.Name,
		"vm_path":  job.Definition.VMPath,
	})
}

// SendJobProgress sends a job progress event
func (m *Manager) SendJobProgress(job *models.Job) {
	if job.Progress == nil {
		return
	}

	m.Send(EventJobProgress, map[string]interface{}{
		"job_id":           job.Definition.ID,
		"job_name":         job.Definition.Name,
		"phase":            job.Progress.Phase,
		"percent_complete": job.Progress.PercentComplete,
		"files_done":       job.Progress.FilesDownloaded,
		"files_total":      job.Progress.TotalFiles,
		"bytes_done":       job.Progress.BytesDownloaded,
		"bytes_total":      job.Progress.TotalBytes,
	})
}
