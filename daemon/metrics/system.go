// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"runtime"
	"sync/atomic"
	"time"
)

// SystemMetrics tracks system-level daemon metrics
type SystemMetrics struct {
	startTime time.Time

	// HTTP metrics
	httpRequests  atomic.Int64
	httpErrors    atomic.Int64
	totalRespTime atomic.Int64 // cumulative response time in microseconds

	// WebSocket metrics
	wsConnections atomic.Int64
}

// NewSystemMetrics creates a new system metrics tracker
func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{
		startTime: time.Now(),
	}
}

// RecordHTTPRequest increments HTTP request counter
func (sm *SystemMetrics) RecordHTTPRequest() {
	sm.httpRequests.Add(1)
}

// RecordHTTPError increments HTTP error counter
func (sm *SystemMetrics) RecordHTTPError() {
	sm.httpErrors.Add(1)
}

// RecordResponseTime adds response time to cumulative total
func (sm *SystemMetrics) RecordResponseTime(duration time.Duration) {
	sm.totalRespTime.Add(int64(duration.Microseconds()))
}

// RecordWSConnection increments WebSocket connection counter
func (sm *SystemMetrics) RecordWSConnection() {
	sm.wsConnections.Add(1)
}

// RecordWSDisconnection decrements WebSocket connection counter
func (sm *SystemMetrics) RecordWSDisconnection() {
	sm.wsConnections.Add(-1)
}

// GetHTTPRequests returns total HTTP requests
func (sm *SystemMetrics) GetHTTPRequests() int64 {
	return sm.httpRequests.Load()
}

// GetHTTPErrors returns total HTTP errors
func (sm *SystemMetrics) GetHTTPErrors() int64 {
	return sm.httpErrors.Load()
}

// GetAvgResponseTime returns average response time in milliseconds
func (sm *SystemMetrics) GetAvgResponseTime() float64 {
	requests := sm.httpRequests.Load()
	if requests == 0 {
		return 0.0
	}
	totalMicros := sm.totalRespTime.Load()
	return float64(totalMicros) / float64(requests) / 1000.0 // convert to ms
}

// GetUptime returns daemon uptime in seconds
func (sm *SystemMetrics) GetUptime() float64 {
	return time.Since(sm.startTime).Seconds()
}

// GetMemoryUsage returns current memory usage in bytes
func (sm *SystemMetrics) GetMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// GetCPUUsage returns approximate CPU usage percentage
// Note: This is goroutine count as a proxy; proper CPU usage requires sampling
func (sm *SystemMetrics) GetCPUUsage() float64 {
	// Return goroutine count as indicator of activity
	return float64(runtime.NumGoroutine())
}

// GetGoroutineCount returns current number of goroutines
func (sm *SystemMetrics) GetGoroutineCount() int {
	return runtime.NumGoroutine()
}

// GetWSConnections returns current WebSocket connections
func (sm *SystemMetrics) GetWSConnections() int64 {
	return sm.wsConnections.Load()
}

// GetSnapshot returns a snapshot of all metrics
func (sm *SystemMetrics) GetSnapshot() Snapshot {
	return Snapshot{
		HTTPRequests:     sm.GetHTTPRequests(),
		HTTPErrors:       sm.GetHTTPErrors(),
		AvgResponseTime:  sm.GetAvgResponseTime(),
		UptimeSeconds:    sm.GetUptime(),
		MemoryUsage:      sm.GetMemoryUsage(),
		GoroutineCount:   sm.GetGoroutineCount(),
		WSConnections:    sm.GetWSConnections(),
	}
}

// Snapshot represents a point-in-time snapshot of metrics
type Snapshot struct {
	HTTPRequests    int64   `json:"http_requests"`
	HTTPErrors      int64   `json:"http_errors"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
	UptimeSeconds   float64 `json:"uptime_seconds"`
	MemoryUsage     uint64  `json:"memory_usage_bytes"`
	GoroutineCount  int     `json:"goroutine_count"`
	WSConnections   int64   `json:"websocket_connections"`
}
