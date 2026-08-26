// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// dashboardReadHeaderTimeout bounds how long the dashboard server waits to
// read request headers, mitigating Slowloris-style attacks.
const dashboardReadHeaderTimeout = 10 * time.Second

// DashboardData holds aggregated data for monitoring dashboard
type DashboardData struct {
	// System status
	Status    string    `json:"status"`
	Uptime    float64   `json:"uptime"`
	Timestamp time.Time `json:"timestamp"`

	// Metrics summary
	Metrics *MetricsSummary `json:"metrics"`

	// Active tasks
	ActiveTasks []*ProgressInfo `json:"active_tasks"`

	// Recent completions
	RecentCompletions []*TaskSummary `json:"recent_completions"`

	// Recent failures
	RecentFailures []*TaskSummary `json:"recent_failures"`

	// Provider breakdown
	ProviderStats map[string]*ProviderStats `json:"provider_stats"`

	// Performance trends
	Trends *PerformanceTrends `json:"trends"`
}

// MetricsSummary summarizes key metrics
type MetricsSummary struct {
	TotalMigrations      int64   `json:"total_migrations"`
	SuccessfulMigrations int64   `json:"successful_migrations"`
	FailedMigrations     int64   `json:"failed_migrations"`
	ActiveMigrations     int64   `json:"active_migrations"`
	SuccessRate          float64 `json:"success_rate"`
	AvgMigrationTime     float64 `json:"avg_migration_time"`
	TotalBytesProcessed  int64   `json:"total_bytes_processed"`
}

// TaskSummary summarizes a completed task
type TaskSummary struct {
	TaskID    string        `json:"task_id"`
	VMName    string        `json:"vm_name"`
	Provider  string        `json:"provider"`
	Status    string        `json:"status"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Error     string        `json:"error,omitempty"`
}

// ProviderStats holds statistics for a specific provider
type ProviderStats struct {
	TotalMigrations      int64   `json:"total_migrations"`
	SuccessfulMigrations int64   `json:"successful_migrations"`
	FailedMigrations     int64   `json:"failed_migrations"`
	SuccessRate          float64 `json:"success_rate"`
	AvgDuration          float64 `json:"avg_duration"`
}

// PerformanceTrends holds performance trend data
type PerformanceTrends struct {
	MigrationsPerHour []float64 `json:"migrations_per_hour"`
	AvgDurationTrend  []float64 `json:"avg_duration_trend"`
	SuccessRateTrend  []float64 `json:"success_rate_trend"`
	Labels            []string  `json:"labels"`
}

// DashboardProvider provides dashboard data
type DashboardProvider struct {
	progressTracker  *ProgressTracker
	metricsCollector *MetricsCollector
	auditLogger      *AuditLogger
}

// NewDashboardProvider creates a new dashboard provider
func NewDashboardProvider(tracker *ProgressTracker, collector *MetricsCollector, audit *AuditLogger) *DashboardProvider {
	return &DashboardProvider{
		progressTracker:  tracker,
		metricsCollector: collector,
		auditLogger:      audit,
	}
}

// GetDashboardData returns current dashboard data
func (dp *DashboardProvider) GetDashboardData() *DashboardData {
	stats := dp.metricsCollector.GetStats()
	allProgress := dp.progressTracker.GetAllProgress()

	// Filter active tasks
	var activeTasks []*ProgressInfo
	var recentCompletions []*TaskSummary
	var recentFailures []*TaskSummary

	for _, progress := range allProgress {
		switch progress.Status {
		case StatusCompleted:
			recentCompletions = append(recentCompletions, &TaskSummary{
				TaskID:    progress.TaskID,
				VMName:    progress.VMName,
				Provider:  progress.Provider,
				Status:    string(progress.Status),
				Duration:  progress.EndTime.Sub(progress.StartTime),
				Timestamp: progress.UpdatedTime,
			})
		case StatusFailed:
			recentFailures = append(recentFailures, &TaskSummary{
				TaskID:    progress.TaskID,
				VMName:    progress.VMName,
				Provider:  progress.Provider,
				Status:    string(progress.Status),
				Duration:  progress.EndTime.Sub(progress.StartTime),
				Timestamp: progress.UpdatedTime,
				Error:     progress.Error,
			})
		default:
			activeTasks = append(activeTasks, progress)
		}
	}

	// Limit recent items
	if len(recentCompletions) > 10 {
		recentCompletions = recentCompletions[:10]
	}
	if len(recentFailures) > 10 {
		recentFailures = recentFailures[:10]
	}

	// Extract metrics
	migrations := stats["migrations"].(map[string]interface{})
	durations := stats["durations"].(map[string]interface{})
	bytes := stats["bytes"].(map[string]interface{})
	providers := stats["providers"].(map[string]int64)

	metricsSummary := &MetricsSummary{
		TotalMigrations:      migrations["total"].(int64),
		SuccessfulMigrations: migrations["succeeded"].(int64),
		FailedMigrations:     migrations["failed"].(int64),
		ActiveMigrations:     migrations["active"].(int64),
		SuccessRate:          stats["success_rate"].(float64),
		AvgMigrationTime:     durations["migration_avg"].(float64),
		TotalBytesProcessed:  bytes["exported"].(int64) + bytes["converted"].(int64) + bytes["uploaded"].(int64),
	}

	// Provider stats
	providerStats := make(map[string]*ProviderStats)
	for provider, count := range providers {
		providerStats[provider] = &ProviderStats{
			TotalMigrations:      count,
			SuccessfulMigrations: count, // Simplified - would need detailed tracking
			FailedMigrations:     0,
			SuccessRate:          100.0,
			AvgDuration:          durations["migration_avg"].(float64),
		}
	}

	// Determine status
	status := "healthy"
	if migrations["active"].(int64) > 10 {
		status = "busy"
	}
	if stats["success_rate"].(float64) < 90 {
		status = "degraded"
	}

	return &DashboardData{
		Status:            status,
		Uptime:            stats["uptime"].(float64),
		Timestamp:         time.Now(),
		Metrics:           metricsSummary,
		ActiveTasks:       activeTasks,
		RecentCompletions: recentCompletions,
		RecentFailures:    recentFailures,
		ProviderStats:     providerStats,
		Trends:            &PerformanceTrends{}, // Would be populated from historical data
	}
}

// DashboardServer serves dashboard data via HTTP
type DashboardServer struct {
	provider *DashboardProvider
	addr     string
	server   *http.Server
}

// NewDashboardServer creates a new dashboard server
func NewDashboardServer(provider *DashboardProvider, addr string) *DashboardServer {
	return &DashboardServer{
		provider: provider,
		addr:     addr,
	}
}

// Start starts the dashboard server
func (ds *DashboardServer) Start() error {
	mux := http.NewServeMux()

	// Dashboard data endpoint
	mux.HandleFunc("/api/v1/dashboard", ds.handleDashboard)

	// System status endpoint
	mux.HandleFunc("/api/v1/status", ds.handleStatus)

	// Active tasks endpoint
	mux.HandleFunc("/api/v1/active", ds.handleActiveTasks)

	// Recent completions endpoint
	mux.HandleFunc("/api/v1/completions", ds.handleRecentCompletions)

	// Recent failures endpoint
	mux.HandleFunc("/api/v1/failures", ds.handleRecentFailures)

	ds.server = &http.Server{
		Addr:              ds.addr,
		Handler:           mux,
		ReadHeaderTimeout: dashboardReadHeaderTimeout,
	}

	return ds.server.ListenAndServe()
}

// Stop stops the dashboard server
func (ds *DashboardServer) Stop() error {
	if ds.server != nil {
		return ds.server.Close()
	}
	return nil
}

// handleDashboard serves complete dashboard data
func (ds *DashboardServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := ds.provider.GetDashboardData()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("dashboard: failed to encode dashboard response: %v", err)
	}
}

// handleStatus serves system status
func (ds *DashboardServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := ds.provider.GetDashboardData()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    data.Status,
		"uptime":    data.Uptime,
		"timestamp": data.Timestamp,
	}); err != nil {
		log.Printf("dashboard: failed to encode status response: %v", err)
	}
}

// handleActiveTasks serves active tasks
func (ds *DashboardServer) handleActiveTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := ds.provider.GetDashboardData()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"active_tasks": data.ActiveTasks,
		"count":        len(data.ActiveTasks),
	}); err != nil {
		log.Printf("dashboard: failed to encode active tasks response: %v", err)
	}
}

// handleRecentCompletions serves recent completions
func (ds *DashboardServer) handleRecentCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := ds.provider.GetDashboardData()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"completions": data.RecentCompletions,
		"count":       len(data.RecentCompletions),
	}); err != nil {
		log.Printf("dashboard: failed to encode completions response: %v", err)
	}
}

// handleRecentFailures serves recent failures
func (ds *DashboardServer) handleRecentFailures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := ds.provider.GetDashboardData()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"failures": data.RecentFailures,
		"count":    len(data.RecentFailures),
	}); err != nil {
		log.Printf("dashboard: failed to encode failures response: %v", err)
	}
}

// HealthStatus represents the health status of the system
type HealthStatus struct {
	Healthy   bool            `json:"healthy"`
	Status    string          `json:"status"`
	Checks    map[string]bool `json:"checks"`
	Timestamp time.Time       `json:"timestamp"`
	Uptime    float64         `json:"uptime"`
	Version   string          `json:"version"`
}

// GetHealthStatus returns the system health status
func (dp *DashboardProvider) GetHealthStatus() *HealthStatus {
	stats := dp.metricsCollector.GetStats()

	checks := map[string]bool{
		"metrics_collector": true,
		"progress_tracker":  true,
		"audit_logger":      dp.auditLogger != nil,
	}

	healthy := true
	for _, check := range checks {
		if !check {
			healthy = false
			break
		}
	}

	status := "healthy"
	if !healthy {
		status = "unhealthy"
	} else if stats["migrations"].(map[string]interface{})["active"].(int64) > 10 {
		status = "busy"
	}

	return &HealthStatus{
		Healthy:   healthy,
		Status:    status,
		Checks:    checks,
		Timestamp: time.Now(),
		Uptime:    stats["uptime"].(float64),
		Version:   "1.0.0",
	}
}
