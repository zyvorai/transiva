// SPDX-License-Identifier: Apache-2.0

// Package dashboard provides a real-time web dashboard for HyperSDK
package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

// Config holds dashboard configuration
type Config struct {
	// Enabled determines if dashboard is enabled
	Enabled bool

	// Port is the HTTP port to listen on
	Port int

	// UpdateInterval is how often to push updates
	UpdateInterval time.Duration

	// MaxClients is the maximum number of concurrent WebSocket clients
	MaxClients int
}

// DefaultConfig returns default dashboard configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:        true,
		Port:           8080,
		UpdateInterval: 1 * time.Second,
		MaxClients:     100,
	}
}

// Dashboard provides real-time monitoring
type Dashboard struct {
	config         *Config
	templates      *template.Template
	upgrader websocket.Upgrader
	// clients maps each connection to a mutex serializing writes to it --
	// gorilla/websocket requires a single writer per connection at a time,
	// and both handleWebSocket (initial message) and handleBroadcast can
	// write to the same connection concurrently.
	clients        map[*websocket.Conn]*sync.Mutex
	clientsMu      sync.RWMutex
	broadcast      chan []byte
	metrics        *Metrics
	metricsMu      sync.RWMutex
	k8sDash        *K8sDashboard
	customDashMgr  *CustomDashboardManager
}

// Metrics holds dashboard metrics
type Metrics struct {
	Timestamp         time.Time      `json:"timestamp"`
	JobsActive        int            `json:"jobs_active"`
	JobsCompleted     int            `json:"jobs_completed"`
	JobsFailed        int            `json:"jobs_failed"`
	JobsPending       int            `json:"jobs_pending"`
	QueueLength       int            `json:"queue_length"`
	HTTPRequests      int64          `json:"http_requests"`
	HTTPErrors        int64          `json:"http_errors"`
	AvgResponseTime   float64        `json:"avg_response_time"`
	MemoryUsage       int64          `json:"memory_usage"`
	CPUUsage          float64        `json:"cpu_usage"`
	Goroutines        int            `json:"goroutines"`
	ActiveConnections int            `json:"active_connections"`
	ProviderStats     map[string]int `json:"provider_stats"`
	RecentJobs        []JobInfo      `json:"recent_jobs"`
	SystemHealth      string         `json:"system_health"`
	Alerts            []Alert        `json:"alerts"`
}

// JobInfo represents job information
type JobInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Duration  float64   `json:"duration"`
	Provider  string    `json:"provider"`
	VMName    string    `json:"vm_name"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

// Alert represents a system alert
type Alert struct {
	ID       string    `json:"id"`
	Severity string    `json:"severity"`
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
}

// NewDashboard creates a new dashboard
func NewDashboard(config *Config) (*Dashboard, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Parse embedded templates
	tmpl, err := template.ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Initialize custom dashboard manager
	customDashMgr, err := NewCustomDashboardManager("./data/dashboards")
	if err != nil {
		// Non-fatal error - continue without custom dashboards
		fmt.Printf("Warning: Failed to initialize custom dashboards: %v\n", err)
	}

	return &Dashboard{
		config:    config,
		templates: tmpl,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
		clients:   make(map[*websocket.Conn]*sync.Mutex),
		broadcast: make(chan []byte, 256),
		metrics: &Metrics{
			Timestamp:     time.Now(),
			ProviderStats: make(map[string]int),
			RecentJobs:    make([]JobInfo, 0),
			Alerts:        make([]Alert, 0),
			SystemHealth:  "healthy",
		},
		customDashMgr: customDashMgr,
	}, nil
}

// Start starts the dashboard server
func (d *Dashboard) Start(ctx context.Context) error {
	if !d.config.Enabled {
		return nil
	}

	// Start broadcast goroutine
	go d.handleBroadcast()

	// Start metrics update goroutine
	go d.updateMetrics(ctx)

	// Try to initialize Kubernetes dashboard (optional)
	k8sDash, err := NewK8sDashboard(d, "", "")
	if err == nil {
		d.k8sDash = k8sDash
		// Start K8s metrics collection
		go func() { _ = d.k8sDash.Start(ctx) }()
	}

	// Setup chi router
	r := chi.NewRouter()

	// Add middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	// ClientIPFromRemoteAddr (not the deprecated RealIP) since this server may
	// not sit behind a trusted reverse proxy; it never trusts spoofable
	// headers like X-Forwarded-For.
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.Compress(5))

	// Serve embedded static files
	r.Handle("/static/*", http.FileServer(http.FS(embeddedFS)))

	// Dashboard pages
	r.Get("/", d.handleIndex)
	r.Get("/k8s", d.handleK8s)
	r.Get("/k8s/charts", d.handleK8sCharts)
	r.Get("/k8s/vms", d.handleK8sVMs)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Main API endpoints
		r.Get("/metrics", d.handleMetrics)
		r.Get("/jobs", d.handleJobs)
		r.Get("/jobs/{id}", d.handleJobDetail)

		// Kubernetes dashboard handlers if available
		if d.k8sDash != nil {
			d.k8sDash.RegisterChiHandlers(r)
		}

		// Custom dashboard handlers if available
		if d.customDashMgr != nil {
			RegisterCustomDashboardChiRoutes(r, d.customDashMgr)
		}
	})

	// WebSocket endpoint
	r.Get("/ws", d.handleWebSocket)

	addr := fmt.Sprintf(":%d", d.config.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Dashboard server error: %v\n", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return server.Shutdown(context.Background())
}

// handleIndex serves the main dashboard page
func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if err := d.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleK8s serves the Kubernetes dashboard page
func (d *Dashboard) handleK8s(w http.ResponseWriter, r *http.Request) {
	if err := d.templates.ExecuteTemplate(w, "k8s.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleK8sCharts serves the Kubernetes charts and analytics page
func (d *Dashboard) handleK8sCharts(w http.ResponseWriter, r *http.Request) {
	if err := d.templates.ExecuteTemplate(w, "k8s-charts.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleK8sVMs serves the Kubernetes VM management page
func (d *Dashboard) handleK8sVMs(w http.ResponseWriter, r *http.Request) {
	if err := d.templates.ExecuteTemplate(w, "k8s-vms.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleMetrics serves current metrics as JSON
func (d *Dashboard) handleMetrics(w http.ResponseWriter, r *http.Request) {
	d.metricsMu.RLock()
	metrics := *d.metrics
	d.metricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		fmt.Printf("failed to encode metrics response: %v\n", err)
	}
}

// handleJobs serves jobs list
func (d *Dashboard) handleJobs(w http.ResponseWriter, r *http.Request) {
	d.metricsMu.RLock()
	jobs := d.metrics.RecentJobs
	d.metricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(jobs); err != nil {
		fmt.Printf("failed to encode jobs response: %v\n", err)
	}
}

// handleJobDetail serves job details
func (d *Dashboard) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from chi URL params
	jobID := chi.URLParam(r, "id")

	d.metricsMu.RLock()
	defer d.metricsMu.RUnlock()

	for _, job := range d.metrics.RecentJobs {
		if job.ID == jobID {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(job); err != nil {
				fmt.Printf("failed to encode job response: %v\n", err)
			}
			return
		}
	}

	http.NotFound(w, r)
}

// handleWebSocket handles WebSocket connections
func (d *Dashboard) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check client limit
	d.clientsMu.RLock()
	clientCount := len(d.clients)
	d.clientsMu.RUnlock()

	if clientCount >= d.config.MaxClients {
		http.Error(w, "Too many clients", http.StatusServiceUnavailable)
		return
	}

	// Upgrade connection
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade error: %v\n", err)
		return
	}

	// Register client
	writeMu := &sync.Mutex{}
	d.clientsMu.Lock()
	d.clients[conn] = writeMu
	d.clientsMu.Unlock()

	// Send initial metrics
	d.metricsMu.RLock()
	data, _ := json.Marshal(d.metrics)
	d.metricsMu.RUnlock()
	writeMu.Lock()
	writeErr := conn.WriteMessage(websocket.TextMessage, data)
	writeMu.Unlock()
	if writeErr != nil {
		fmt.Printf("WebSocket write error: %v\n", writeErr)
		d.clientsMu.Lock()
		delete(d.clients, conn)
		d.clientsMu.Unlock()
		if cerr := conn.Close(); cerr != nil {
			fmt.Printf("WebSocket close error: %v\n", cerr)
		}
		return
	}

	// Handle client messages
	go d.handleClient(conn)
}

// handleClient handles individual client connections
func (d *Dashboard) handleClient(conn *websocket.Conn) {
	defer func() {
		d.clientsMu.Lock()
		delete(d.clients, conn)
		d.clientsMu.Unlock()
		if err := conn.Close(); err != nil {
			fmt.Printf("WebSocket close error: %v\n", err)
		}
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// handleBroadcast broadcasts metrics to all connected clients
func (d *Dashboard) handleBroadcast() {
	for data := range d.broadcast {
		d.clientsMu.RLock()
		clients := make(map[*websocket.Conn]*sync.Mutex, len(d.clients))
		for client, mu := range d.clients {
			clients[client] = mu
		}
		d.clientsMu.RUnlock()

		var dead []*websocket.Conn
		for client, mu := range clients {
			mu.Lock()
			err := client.WriteMessage(websocket.TextMessage, data)
			mu.Unlock()
			if err != nil {
				if cerr := client.Close(); cerr != nil {
					fmt.Printf("WebSocket close error: %v\n", cerr)
				}
				dead = append(dead, client)
			}
		}

		if len(dead) > 0 {
			d.clientsMu.Lock()
			for _, client := range dead {
				delete(d.clients, client)
			}
			d.clientsMu.Unlock()
		}
	}
}

// updateMetrics periodically updates and broadcasts metrics
func (d *Dashboard) updateMetrics(ctx context.Context) {
	ticker := time.NewTicker(d.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect metrics (in real implementation, this would fetch from actual sources)
			d.collectMetrics()

			// Broadcast to clients
			d.metricsMu.RLock()
			data, err := json.Marshal(d.metrics)
			d.metricsMu.RUnlock()

			if err == nil {
				select {
				case d.broadcast <- data:
				default:
					// Channel full, skip this update
				}
			}
		}
	}
}

// collectMetrics collects current metrics
func (d *Dashboard) collectMetrics() {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()

	// Update timestamp
	d.metrics.Timestamp = time.Now()

	// In a real implementation, these would be fetched from actual sources
	// For now, we'll use placeholder logic

	// Update active connections
	d.clientsMu.RLock()
	d.metrics.ActiveConnections = len(d.clients)
	d.clientsMu.RUnlock()
}

// UpdateJobMetrics updates job-related metrics
func (d *Dashboard) UpdateJobMetrics(active, completed, failed, pending, queueLen int) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()

	d.metrics.JobsActive = active
	d.metrics.JobsCompleted = completed
	d.metrics.JobsFailed = failed
	d.metrics.JobsPending = pending
	d.metrics.QueueLength = queueLen
}

// AddJob adds a job to the recent jobs list
func (d *Dashboard) AddJob(job JobInfo) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()

	// Add to beginning of list
	d.metrics.RecentJobs = append([]JobInfo{job}, d.metrics.RecentJobs...)

	// Keep only last 50 jobs
	if len(d.metrics.RecentJobs) > 50 {
		d.metrics.RecentJobs = d.metrics.RecentJobs[:50]
	}

	// Update provider stats
	if d.metrics.ProviderStats == nil {
		d.metrics.ProviderStats = make(map[string]int)
	}
	d.metrics.ProviderStats[job.Provider]++
}

// UpdateSystemMetrics updates system resource metrics
func (d *Dashboard) UpdateSystemMetrics(memoryMB int64, cpuPercent float64, goroutines int) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()

	d.metrics.MemoryUsage = memoryMB
	d.metrics.CPUUsage = cpuPercent
	d.metrics.Goroutines = goroutines
}

// UpdateHTTPMetrics updates HTTP metrics
func (d *Dashboard) UpdateHTTPMetrics(requests, errors int64, avgResponseTime float64) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()

	d.metrics.HTTPRequests = requests
	d.metrics.HTTPErrors = errors
	d.metrics.AvgResponseTime = avgResponseTime
}

// AddAlert adds a new alert
func (d *Dashboard) AddAlert(severity, message string) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()

	alert := Alert{
		ID:       fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Severity: severity,
		Message:  message,
		Time:     time.Now(),
	}

	d.metrics.Alerts = append([]Alert{alert}, d.metrics.Alerts...)

	// Keep only last 20 alerts
	if len(d.metrics.Alerts) > 20 {
		d.metrics.Alerts = d.metrics.Alerts[:20]
	}
}

// SetSystemHealth sets the overall system health status
func (d *Dashboard) SetSystemHealth(health string) {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()

	d.metrics.SystemHealth = health
}

// GetClientCount returns the number of connected WebSocket clients
func (d *Dashboard) GetClientCount() int {
	d.clientsMu.RLock()
	defer d.clientsMu.RUnlock()
	return len(d.clients)
}
