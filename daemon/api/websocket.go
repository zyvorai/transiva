// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/zyvorai/transiva/daemon/models"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// WSClient represents a connected WebSocket client
type WSClient struct {
	conn      *websocket.Conn
	send      chan WSMessage
	hub       *WSHub
	closed    bool
	closeOnce sync.Once
}

// WSHub manages WebSocket clients and broadcasts
type WSHub struct {
	clients          map[*WSClient]bool
	broadcast        chan WSMessage
	register         chan *WSClient
	unregister       chan *WSClient
	mu               sync.RWMutex
	onDisconnect     func() // Callback for client disconnect
	logger           interface {
		Debug(msg string, keysAndValues ...interface{})
		Info(msg string, keysAndValues ...interface{})
		Warn(msg string, keysAndValues ...interface{})
		Error(msg string, keysAndValues ...interface{})
	}
}

// NewWSHub creates a new WebSocket hub
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan WSMessage, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
}

// SetLogger sets the logger for the WebSocket hub
func (h *WSHub) SetLogger(logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}) {
	h.logger = logger
}

// SetOnDisconnect sets the callback for client disconnects
func (h *WSHub) SetOnDisconnect(callback func()) {
	h.onDisconnect = callback
}

// Run starts the WebSocket hub with context for graceful shutdown
func (h *WSHub) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			if h.logger != nil {
				h.logger.Error("WebSocket hub panic recovered", "panic", r)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			if h.logger != nil {
				h.logger.Info("WebSocket hub shutting down")
			}
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			if h.logger != nil {
				h.logger.Debug("WebSocket client registered", "total_clients", len(h.clients))
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.closeOnce.Do(func() {
					close(client.send)
				})
				if h.logger != nil {
					h.logger.Debug("WebSocket client unregistered", "total_clients", len(h.clients))
				}
				// Call disconnect callback if set
				if h.onDisconnect != nil {
					h.onDisconnect()
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Fix race condition: collect clients to unregister first
			h.mu.RLock()
			clientsToUnregister := make([]*WSClient, 0)
			for client := range h.clients {
				select {
				case client.send <- message:
					// Message sent successfully
				default:
					// Client buffer full, mark for unregistration
					clientsToUnregister = append(clientsToUnregister, client)
				}
			}
			h.mu.RUnlock()

			// Unregister clients with full buffers
			for _, client := range clientsToUnregister {
				if h.logger != nil {
					h.logger.Warn("WebSocket client buffer full, closing connection")
				}
				h.unregister <- client
			}
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WSHub) Broadcast(msgType string, data map[string]interface{}) {
	message := WSMessage{
		Type:      msgType,
		Timestamp: time.Now(),
		Data:      data,
	}
	select {
	case h.broadcast <- message:
	default:
		// Drop message if channel is full
		if h.logger != nil {
			h.logger.Warn("WebSocket broadcast channel full, dropping message", "type", msgType)
		}
	}
}

// BroadcastRaw broadcasts raw data directly to all clients (for React dashboard metrics)
func (h *WSHub) BroadcastRaw(data interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- WSMessage{
			Type:      "metrics",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"raw": data},
		}:
		default:
			// Client buffer is full, skip this client
			if h.logger != nil {
				h.logger.Debug("Client buffer full, skipping broadcast")
			}
		}
	}
}

// GetClientCount returns the number of connected clients
func (h *WSHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Shutdown gracefully closes all WebSocket connections
func (h *WSHub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.logger != nil {
		h.logger.Info("Shutting down WebSocket hub", "client_count", len(h.clients))
	}

	for client := range h.clients {
		client.closeOnce.Do(func() {
			close(client.send)
		})
		if err := client.conn.Close(); err != nil && h.logger != nil {
			h.logger.Warn("failed to close WebSocket connection", "error", err)
		}
	}
	h.clients = make(map[*WSClient]bool)
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *WSClient) readPump(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			if c.hub.logger != nil {
				c.hub.logger.Error("WebSocket readPump panic recovered", "panic", r)
			}
		}
		c.hub.unregister <- c
		if err := c.conn.Close(); err != nil && c.hub.logger != nil {
			c.hub.logger.Debug("failed to close WebSocket connection", "error", err)
		}
	}()

	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil && c.hub.logger != nil {
		c.hub.logger.Warn("failed to set WebSocket read deadline", "error", err)
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			return err
		}
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, _, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					if c.hub.logger != nil {
						c.hub.logger.Warn("WebSocket unexpected close", "error", err)
					}
				}
				return
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *WSClient) writePump(ctx context.Context) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		if r := recover(); r != nil {
			if c.hub.logger != nil {
				c.hub.logger.Error("WebSocket writePump panic recovered", "panic", r)
			}
		}
		ticker.Stop()
		if err := c.conn.Close(); err != nil && c.hub.logger != nil {
			c.hub.logger.Debug("failed to close WebSocket connection", "error", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				if c.hub.logger != nil {
					c.hub.logger.Error("WebSocket write deadline error", "error", err)
				}
				return
			}
			if !ok {
				// Hub closed the channel
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil && c.hub.logger != nil {
					c.hub.logger.Debug("failed to send WebSocket close message", "error", err)
				}
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				if c.hub.logger != nil {
					c.hub.logger.Error("WebSocket write error", "error", err)
				}
				return
			}

			if err := json.NewEncoder(w).Encode(message); err != nil {
				if c.hub.logger != nil {
					c.hub.logger.Error("WebSocket JSON encode error", "error", err)
				}
				return
			}

			if err := w.Close(); err != nil {
				if c.hub.logger != nil {
					c.hub.logger.Error("WebSocket writer close error", "error", err)
				}
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				if c.hub.logger != nil {
					c.hub.logger.Debug("WebSocket write deadline error", "error", err)
				}
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				if c.hub.logger != nil {
					c.hub.logger.Debug("WebSocket ping error", "error", err)
				}
				return
			}
		}
	}
}

// createUpgrader creates a WebSocket upgrader with origin validation
func (es *EnhancedServer) createUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// If authentication is disabled, allow all origins (development mode)
			if !es.config.Security.EnableAuth {
				return true
			}

			origin := r.Header.Get("Origin")
			if origin == "" {
				// No origin header, allow (non-browser client)
				return true
			}

			// Check against allowed origins
			for _, allowed := range es.config.Security.AllowedOrigins {
				if origin == allowed {
					return true
				}
				// Support wildcard subdomain matching (e.g., *.example.com)
				if strings.HasPrefix(allowed, "*.") {
					domain := strings.TrimPrefix(allowed, "*")
					// Ensure domain starts with "." (e.g., ".example.com")
					if !strings.HasPrefix(domain, ".") {
						domain = "." + domain
					}
					// Check if origin ends with domain and has a subdomain
					// This prevents "evilexample.com" from matching "*.example.com"
					if strings.HasSuffix(origin, domain) {
						// Ensure it's not just the bare domain without subdomain
						expectedBare := strings.TrimPrefix(domain, ".")
						if origin != "http://"+expectedBare && origin != "https://"+expectedBare {
							return true
						}
					}
				}
			}

			es.logger.Warn("WebSocket connection rejected - origin not allowed",
				"origin", origin,
				"allowed", es.config.Security.AllowedOrigins)
			return false
		},
	}
}

// handleWebSocket handles WebSocket connections
func (es *EnhancedServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authentication check
	if es.config.Security.EnableAuth {
		// Validate server configuration
		if es.config.Security.APIKey == "" {
			es.logger.Error("authentication enabled but API key not configured")
			http.Error(w, "server configuration error", http.StatusInternalServerError)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		// Use constant-time comparison to prevent timing attacks
		if apiKey == "" || subtle.ConstantTimeCompare([]byte(apiKey), []byte(es.config.Security.APIKey)) != 1 {
			es.logger.Warn("WebSocket connection rejected - invalid API key", "remote", r.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	upgrader := es.createUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		es.logger.Error("WebSocket upgrade failed", "error", err, "remote", r.RemoteAddr)
		return
	}

	client := &WSClient{
		conn: conn,
		send: make(chan WSMessage, 256),
		hub:  es.wsHub,
	}

	es.wsHub.register <- client
	es.systemMetrics.RecordWSConnection()

	// Send initial data
	go func() {
		defer func() {
			if r := recover(); r != nil {
				es.logger.Error("WebSocket initial data panic", "panic", r)
			}
		}()

		// Send current status
		status := es.manager.GetStatus()
		client.send <- WSMessage{
			Type:      "status",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"total_jobs":     status.TotalJobs,
				"running_jobs":   status.RunningJobs,
				"completed_jobs": status.CompletedJobs,
				"failed_jobs":    status.FailedJobs,
			},
		}

		// Send current jobs
		allJobs := es.manager.GetAllJobs()
		jobsData := make([]map[string]interface{}, 0, len(allJobs))
		for _, job := range allJobs {
			jobsData = append(jobsData, map[string]interface{}{
				"definition": job.Definition,
				"status":     job.Status,
				"progress":   job.Progress,
				"error":      job.Error,
			})
		}
		client.send <- WSMessage{
			Type:      "jobs",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"jobs": jobsData,
			},
		}
	}()

	// Run client pumps with context
	go client.writePump(es.shutdownCtx)
	go client.readPump(es.shutdownCtx)

	es.logger.Info("websocket client connected", "remote", r.RemoteAddr)
}

// StartStatusBroadcaster starts a goroutine that broadcasts status updates
func (es *EnhancedServer) StartStatusBroadcaster(ctx context.Context) *time.Ticker {
	ticker := time.NewTicker(1 * time.Second) // Changed to 1 second for better real-time updates
	go func() {
		defer func() {
			if r := recover(); r != nil {
				es.logger.Error("Status broadcaster panic recovered", "panic", r)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				es.logger.Debug("Status broadcaster stopped")
				return

			case <-ticker.C:
				if es.wsHub.GetClientCount() == 0 {
					continue
				}

				// Broadcast full dashboard metrics for React dashboard
				es.broadcastDashboardMetrics()
			}
		}
	}()
	return ticker
}

// broadcastDashboardMetrics sends comprehensive metrics in the format expected by the React dashboard
func (es *EnhancedServer) broadcastDashboardMetrics() {
	status := es.manager.GetStatus()
	allJobs := es.manager.GetAllJobs()

	// Convert jobs to the format expected by the React dashboard
	recentJobs := make([]map[string]interface{}, 0, len(allJobs))
	for _, job := range allJobs {
		var progressPercent int
		if job.Progress != nil {
			progressPercent = int(job.Progress.PercentComplete)
		}

		var startTime string
		if job.StartedAt != nil {
			startTime = job.StartedAt.Format(time.RFC3339)
		} else {
			startTime = job.Definition.CreatedAt.Format(time.RFC3339)
		}

		var durationSeconds float64
		if job.StartedAt != nil {
			if job.CompletedAt != nil {
				durationSeconds = job.CompletedAt.Sub(*job.StartedAt).Seconds()
			} else {
				durationSeconds = time.Since(*job.StartedAt).Seconds()
			}
		}

		jobInfo := map[string]interface{}{
			"id":               job.Definition.ID,
			"name":             job.Definition.Name,
			"status":           string(job.Status),
			"progress":         progressPercent,
			"start_time":       startTime,
			"duration_seconds": durationSeconds,
			"provider":         "vsphere",             // Default to vsphere for now
			"vm_name":          job.Definition.VMPath, // Using VMPath as vm_name
			"vm_path":          job.Definition.VMPath,
			"output_dir":       job.Definition.OutputDir,
			"format":           job.Definition.Format,
			"compress":         job.Definition.Compress,
			"created_at":       job.Definition.CreatedAt.Format(time.RFC3339),
			"updated_at":       job.UpdatedAt.Format(time.RFC3339),
		}

		if job.CompletedAt != nil {
			jobInfo["end_time"] = job.CompletedAt.Format(time.RFC3339)
		}
		if job.Error != "" {
			jobInfo["error_msg"] = job.Error
		}

		recentJobs = append(recentJobs, jobInfo)
	}

	// Get system metrics snapshot
	sysMetrics := es.systemMetrics.GetSnapshot()

	// Build complete metrics object matching the React dashboard's Metrics interface
	metrics := map[string]interface{}{
		"timestamp":          time.Now().Format(time.RFC3339),
		"jobs_active":        status.RunningJobs,
		"jobs_completed":     status.CompletedJobs,
		"jobs_failed":        status.FailedJobs,
		"jobs_pending":       status.TotalJobs - status.RunningJobs - status.CompletedJobs - status.FailedJobs - status.CancelledJobs,
		"jobs_cancelled":     status.CancelledJobs,
		"queue_length":       status.TotalJobs,
		"http_requests":      sysMetrics.HTTPRequests,
		"http_errors":        sysMetrics.HTTPErrors,
		"avg_response_time":  sysMetrics.AvgResponseTime,
		"memory_usage":       sysMetrics.MemoryUsage,
		"cpu_usage":          0.0, // Goroutine count as proxy
		"goroutines":         sysMetrics.GoroutineCount,
		"active_connections": sysMetrics.WSConnections,
		"websocket_clients":  es.wsHub.GetClientCount(),
		"provider_stats":     map[string]interface{}{},
		"recent_jobs":        recentJobs,
		"system_health":      "healthy",
		"alerts":             []interface{}{},
		"uptime_seconds":     sysMetrics.UptimeSeconds,
	}

	// Send metrics directly as JSON (not wrapped in WSMessage)
	es.wsHub.BroadcastRaw(metrics)
}

// BroadcastJobUpdate broadcasts a job update to all connected clients
func (es *EnhancedServer) BroadcastJobUpdate(job *models.Job) {
	if es.wsHub == nil {
		return
	}

	es.wsHub.Broadcast("job_update", map[string]interface{}{
		"definition": job.Definition,
		"status":     job.Status,
		"progress":   job.Progress,
		"error":      job.Error,
	})
}

// BroadcastScheduleEvent broadcasts a schedule event
func (es *EnhancedServer) BroadcastScheduleEvent(event string, scheduleID string, data map[string]interface{}) {
	if es.wsHub == nil {
		return
	}

	if data == nil {
		data = make(map[string]interface{})
	}
	data["schedule_id"] = scheduleID
	data["event"] = event

	es.wsHub.Broadcast("schedule_event", data)
}
