// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		// In production, check against allowed origins
		return true
	},
}

// handleMetricsStream streams real-time metrics via WebSocket
func (es *EnhancedServer) handleMetricsStream(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	entityName := r.URL.Query().Get("entity")
	entityType := r.URL.Query().Get("type")
	intervalStr := r.URL.Query().Get("interval")

	if entityName == "" {
		http.Error(w, "entity name required", http.StatusBadRequest)
		return
	}
	if entityType == "" {
		entityType = "vm" // Default to VM
	}

	// Parse interval (default 20 seconds)
	interval := 20 * time.Second
	if intervalStr != "" {
		parsedInterval, err := time.ParseDuration(intervalStr)
		if err != nil {
			http.Error(w, "invalid interval", http.StatusBadRequest)
			return
		}
		interval = parsedInterval
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		es.logger.Error("failed to upgrade to WebSocket", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	es.logger.Info("metrics stream started",
		"entity", entityName,
		"type", entityType,
		"interval", interval)

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.sendWSError(conn, fmt.Sprintf("failed to create vSphere client: %v", err))
		return
	}
	defer func() { _ = client.Close() }()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start metrics stream
	metricsChan, err := client.StreamMetrics(ctx, entityName, entityType, interval)
	if err != nil {
		es.sendWSError(conn, fmt.Sprintf("failed to start metrics stream: %v", err))
		return
	}

	// Set up ping/pong for connection health
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Stream metrics to WebSocket
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			es.logger.Info("metrics stream stopped", "entity", entityName)
			return

		case <-done:
			es.logger.Info("WebSocket client disconnected", "entity", entityName)
			return

		case metrics, ok := <-metricsChan:
			if !ok {
				es.logger.Info("metrics channel closed", "entity", entityName)
				return
			}

			// Send metrics to client
			msg := WSMessage{
				Type:      "metrics",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"entity_name":     metrics.EntityName,
					"entity_type":     metrics.EntityType,
					"timestamp":       metrics.Timestamp,
					"cpu_percent":     metrics.CPUPercent,
					"cpu_usage_mhz":   metrics.CPUUsageMhz,
					"memory_percent":  metrics.MemoryPercent,
					"memory_usage_mb": metrics.MemoryUsageMB,
					"disk_read_mbps":  metrics.DiskReadMBps,
					"disk_write_mbps": metrics.DiskWriteMBps,
					"net_rx_mbps":     metrics.NetRxMBps,
					"net_tx_mbps":     metrics.NetTxMBps,
				},
			}

			if err := conn.WriteJSON(msg); err != nil {
				es.logger.Error("failed to send metrics", "error", err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				es.logger.Error("failed to send ping", "error", err)
				return
			}
		}
	}
}

// handleEventsStream streams vCenter events via WebSocket
func (es *EnhancedServer) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	eventTypesStr := r.URL.Query().Get("types")
	entityTypesStr := r.URL.Query().Get("entity_types")

	var eventTypes []string
	if eventTypesStr != "" {
		eventTypes = strings.Split(eventTypesStr, ",")
	}

	var entityTypes []string
	if entityTypesStr != "" {
		entityTypes = strings.Split(entityTypesStr, ",")
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		es.logger.Error("failed to upgrade to WebSocket", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	es.logger.Info("event stream started",
		"event_types", eventTypes,
		"entity_types", entityTypes)

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.sendWSError(conn, fmt.Sprintf("failed to create vSphere client: %v", err))
		return
	}
	defer func() { _ = client.Close() }()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start event stream
	eventsChan, err := client.StreamEvents(ctx, eventTypes, entityTypes)
	if err != nil {
		es.sendWSError(conn, fmt.Sprintf("failed to start event stream: %v", err))
		return
	}

	// Set up ping/pong for connection health
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Stream events to WebSocket
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			es.logger.Info("event stream stopped")
			return

		case <-done:
			es.logger.Info("WebSocket client disconnected")
			return

		case event, ok := <-eventsChan:
			if !ok {
				es.logger.Info("event channel closed")
				return
			}

			// Send event to client
			msg := WSMessage{
				Type:      "event",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"event_id":     event.EventID,
					"event_type":   event.EventType,
					"message":      event.Message,
					"created_time": event.CreatedTime,
					"user_name":    event.UserName,
					"entity_name":  event.EntityName,
					"entity_type":  event.EntityType,
					"datacenter":   event.Datacenter,
					"severity":     event.Severity,
					"chain_id":     event.ChainID,
				},
			}

			if err := conn.WriteJSON(msg); err != nil {
				es.logger.Error("failed to send event", "error", err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				es.logger.Error("failed to send ping", "error", err)
				return
			}
		}
	}
}

// handleTasksStream streams task status updates via WebSocket
func (es *EnhancedServer) handleTasksStream(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		es.logger.Error("failed to upgrade to WebSocket", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	es.logger.Info("task stream started")

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.sendWSError(conn, fmt.Sprintf("failed to create vSphere client: %v", err))
		return
	}
	defer func() { _ = client.Close() }()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Set up ping/pong for connection health
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Poll tasks every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastTaskTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			es.logger.Info("task stream stopped")
			return

		case <-done:
			es.logger.Info("WebSocket client disconnected")
			return

		case <-ticker.C:
			// Get recent tasks
			taskCtx, taskCancel := context.WithTimeout(ctx, 30*time.Second)
			tasks, err := client.GetRecentTasks(taskCtx, lastTaskTime)
			taskCancel()

			if err != nil {
				es.logger.Error("failed to get tasks", "error", err)
				continue
			}

			// Send each new task to client
			for _, task := range tasks {
				if task.StartTime.After(lastTaskTime) {
					lastTaskTime = task.StartTime
				}

				msg := WSMessage{
					Type:      "task",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"task_id":       task.TaskID,
						"name":          task.Name,
						"description":   task.Description,
						"state":         task.State,
						"entity_name":   task.EntityName,
						"start_time":    task.StartTime,
						"complete_time": task.CompleteTime,
						"progress":      task.Progress,
						"error":         task.Error,
					},
				}

				if err := conn.WriteJSON(msg); err != nil {
					es.logger.Error("failed to send task", "error", err)
					return
				}
			}

			// Send ping
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				es.logger.Error("failed to send ping", "error", err)
				return
			}
		}
	}
}

// Helper function to send error message via WebSocket
func (es *EnhancedServer) sendWSError(conn *websocket.Conn, message string) {
	msg := WSMessage{
		Type:      "error",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message": message,
		},
	}
	if err := conn.WriteJSON(msg); err != nil {
		es.logger.Error("failed to send error message", "error", err)
	}
}
