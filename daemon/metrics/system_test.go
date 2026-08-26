// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"
	"time"
)

func TestNewSystemMetrics(t *testing.T) {
	sm := NewSystemMetrics()
	if sm == nil {
		t.Fatal("NewSystemMetrics returned nil")
	}

	// Verify start time is set
	if sm.startTime.IsZero() {
		t.Error("start time should be set")
	}

	// Verify initial counters are zero
	if sm.GetHTTPRequests() != 0 {
		t.Errorf("expected 0 HTTP requests, got %d", sm.GetHTTPRequests())
	}
	if sm.GetHTTPErrors() != 0 {
		t.Errorf("expected 0 HTTP errors, got %d", sm.GetHTTPErrors())
	}
	if sm.GetWSConnections() != 0 {
		t.Errorf("expected 0 WS connections, got %d", sm.GetWSConnections())
	}
}

func TestSystemMetrics_HTTPRequests(t *testing.T) {
	sm := NewSystemMetrics()

	// Record some requests
	sm.RecordHTTPRequest()
	sm.RecordHTTPRequest()
	sm.RecordHTTPRequest()

	requests := sm.GetHTTPRequests()
	if requests != 3 {
		t.Errorf("expected 3 HTTP requests, got %d", requests)
	}
}

func TestSystemMetrics_HTTPErrors(t *testing.T) {
	sm := NewSystemMetrics()

	// Record some errors
	sm.RecordHTTPError()
	sm.RecordHTTPError()

	errors := sm.GetHTTPErrors()
	if errors != 2 {
		t.Errorf("expected 2 HTTP errors, got %d", errors)
	}
}

func TestSystemMetrics_ResponseTime(t *testing.T) {
	sm := NewSystemMetrics()

	// Record response times
	sm.RecordHTTPRequest()
	sm.RecordResponseTime(100 * time.Millisecond)

	sm.RecordHTTPRequest()
	sm.RecordResponseTime(200 * time.Millisecond)

	sm.RecordHTTPRequest()
	sm.RecordResponseTime(300 * time.Millisecond)

	// Average should be 200ms
	avgTime := sm.GetAvgResponseTime()
	expected := 200.0 // milliseconds
	if avgTime != expected {
		t.Errorf("expected avg response time %f ms, got %f ms", expected, avgTime)
	}
}

func TestSystemMetrics_ResponseTimeNoRequests(t *testing.T) {
	sm := NewSystemMetrics()

	// Without any requests, average should be 0
	avgTime := sm.GetAvgResponseTime()
	if avgTime != 0.0 {
		t.Errorf("expected avg response time 0 with no requests, got %f", avgTime)
	}
}

func TestSystemMetrics_WebSocketConnections(t *testing.T) {
	sm := NewSystemMetrics()

	// Add connections
	sm.RecordWSConnection()
	sm.RecordWSConnection()
	sm.RecordWSConnection()

	if sm.GetWSConnections() != 3 {
		t.Errorf("expected 3 WS connections, got %d", sm.GetWSConnections())
	}

	// Remove a connection
	sm.RecordWSDisconnection()

	if sm.GetWSConnections() != 2 {
		t.Errorf("expected 2 WS connections after disconnect, got %d", sm.GetWSConnections())
	}

	// Remove remaining connections
	sm.RecordWSDisconnection()
	sm.RecordWSDisconnection()

	if sm.GetWSConnections() != 0 {
		t.Errorf("expected 0 WS connections, got %d", sm.GetWSConnections())
	}
}

func TestSystemMetrics_Uptime(t *testing.T) {
	sm := NewSystemMetrics()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	uptime := sm.GetUptime()
	if uptime < 0.1 {
		t.Errorf("expected uptime >= 0.1 seconds, got %f", uptime)
	}
}

func TestSystemMetrics_MemoryUsage(t *testing.T) {
	sm := NewSystemMetrics()

	memUsage := sm.GetMemoryUsage()
	if memUsage == 0 {
		t.Error("expected memory usage > 0")
	}
}

func TestSystemMetrics_CPUUsage(t *testing.T) {
	sm := NewSystemMetrics()

	cpuUsage := sm.GetCPUUsage()
	// CPU usage is based on goroutine count, should be > 0
	if cpuUsage <= 0 {
		t.Error("expected CPU usage (goroutine count) > 0")
	}
}

func TestSystemMetrics_GoroutineCount(t *testing.T) {
	sm := NewSystemMetrics()

	count := sm.GetGoroutineCount()
	if count <= 0 {
		t.Error("expected goroutine count > 0")
	}
}

func TestSystemMetrics_Snapshot(t *testing.T) {
	sm := NewSystemMetrics()

	// Record some metrics
	sm.RecordHTTPRequest()
	sm.RecordHTTPRequest()
	sm.RecordHTTPError()
	sm.RecordResponseTime(150 * time.Millisecond)
	sm.RecordResponseTime(250 * time.Millisecond)
	sm.RecordWSConnection()
	sm.RecordWSConnection()

	// Wait a bit for uptime
	time.Sleep(50 * time.Millisecond)

	// Get snapshot
	snapshot := sm.GetSnapshot()

	// Verify all fields
	if snapshot.HTTPRequests != 2 {
		t.Errorf("snapshot: expected 2 HTTP requests, got %d", snapshot.HTTPRequests)
	}

	if snapshot.HTTPErrors != 1 {
		t.Errorf("snapshot: expected 1 HTTP error, got %d", snapshot.HTTPErrors)
	}

	if snapshot.AvgResponseTime != 200.0 {
		t.Errorf("snapshot: expected avg response time 200ms, got %f", snapshot.AvgResponseTime)
	}

	if snapshot.UptimeSeconds < 0.05 {
		t.Errorf("snapshot: expected uptime >= 0.05s, got %f", snapshot.UptimeSeconds)
	}

	if snapshot.MemoryUsage == 0 {
		t.Error("snapshot: expected memory usage > 0")
	}

	if snapshot.GoroutineCount <= 0 {
		t.Error("snapshot: expected goroutine count > 0")
	}

	if snapshot.WSConnections != 2 {
		t.Errorf("snapshot: expected 2 WS connections, got %d", snapshot.WSConnections)
	}
}

func TestSystemMetrics_ConcurrentAccess(t *testing.T) {
	sm := NewSystemMetrics()

	// Concurrent operations
	done := make(chan bool)

	// Multiple goroutines recording requests
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				sm.RecordHTTPRequest()
				sm.RecordResponseTime(time.Millisecond * time.Duration(j))
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counts
	requests := sm.GetHTTPRequests()
	if requests != 1000 {
		t.Errorf("expected 1000 requests from concurrent access, got %d", requests)
	}

	// Avg response time should be calculated correctly
	avgTime := sm.GetAvgResponseTime()
	if avgTime <= 0 {
		t.Error("expected avg response time > 0 from concurrent access")
	}
}

func TestSystemMetrics_MixedOperations(t *testing.T) {
	sm := NewSystemMetrics()

	// Mix of operations
	sm.RecordHTTPRequest()
	sm.RecordHTTPError()
	sm.RecordResponseTime(100 * time.Millisecond)

	sm.RecordHTTPRequest()
	sm.RecordResponseTime(200 * time.Millisecond)

	sm.RecordWSConnection()
	sm.RecordWSConnection()
	sm.RecordWSDisconnection()

	// Verify state
	if sm.GetHTTPRequests() != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", sm.GetHTTPRequests())
	}

	if sm.GetHTTPErrors() != 1 {
		t.Errorf("expected 1 HTTP error, got %d", sm.GetHTTPErrors())
	}

	if sm.GetAvgResponseTime() != 150.0 {
		t.Errorf("expected avg response time 150ms, got %f", sm.GetAvgResponseTime())
	}

	if sm.GetWSConnections() != 1 {
		t.Errorf("expected 1 WS connection, got %d", sm.GetWSConnections())
	}
}
