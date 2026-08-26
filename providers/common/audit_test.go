// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewAuditLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("Expected logger to be created")
	}

	if logger.logPath != logPath {
		t.Errorf("Expected logPath %s, got %s", logPath, logger.logPath)
	}

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
	}
}

func TestAuditLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	event := &AuditEvent{
		EventType:   EventMigrationStart,
		Action:      "start_migration",
		Description: "Starting VM migration",
		VMName:      "test-vm",
		Provider:    "vsphere",
		Status:      "started",
		Success:     true,
	}

	err = logger.Log(event)
	if err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}

	// Verify event ID and timestamp were set
	if event.EventID == "" {
		t.Error("Expected EventID to be set")
	}

	if event.Timestamp.IsZero() {
		t.Error("Expected Timestamp to be set")
	}

	// Verify log file has content
	stat, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if stat.Size() == 0 {
		t.Error("Expected log file to have content")
	}
}

func TestAuditLogger_LogMigrationStart(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	err = logger.LogMigrationStart("task-123", "test-vm", "vsphere", "testuser")
	if err != nil {
		t.Fatalf("Failed to log migration start: %v", err)
	}

	// Verify log file has content
	stat, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if stat.Size() == 0 {
		t.Error("Expected log file to have content")
	}
}

func TestAuditLogger_LogMigrationComplete(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	duration := 5 * time.Minute
	details := map[string]interface{}{
		"source": "vsphere",
		"target": "kvm",
	}
	err = logger.LogMigrationComplete("task-123", "test-vm", "vsphere", "testuser", duration, details)
	if err != nil {
		t.Fatalf("Failed to log migration complete: %v", err)
	}

	stat, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if stat.Size() == 0 {
		t.Error("Expected log file to have content")
	}
}

func TestAuditLogger_LogMigrationFailed(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	migrationErr := fmt.Errorf("disk space error")
	err = logger.LogMigrationFailed("task-123", "test-vm", "vsphere", "testuser", migrationErr)
	if err != nil {
		t.Fatalf("Failed to log migration failed: %v", err)
	}

	stat, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if stat.Size() == 0 {
		t.Error("Expected log file to have content")
	}
}

func TestAuditLogger_LogExport(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Test export start
	err = logger.LogExportStart("task-123", "test-vm", "vsphere")
	if err != nil {
		t.Fatalf("Failed to log export start: %v", err)
	}

	// Test export complete
	duration := 3 * time.Minute
	bytesExported := int64(1024 * 1024 * 500)
	err = logger.LogExportComplete("task-123", "test-vm", "vsphere", duration, bytesExported)
	if err != nil {
		t.Fatalf("Failed to log export complete: %v", err)
	}
}

func TestAuditLogger_LogConversion(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Test conversion start
	err = logger.LogConversionStart("task-123", "test-vm")
	if err != nil {
		t.Fatalf("Failed to log conversion start: %v", err)
	}

	// Test conversion complete
	files := []string{"/path/to/output/disk1.qcow2", "/path/to/output/disk2.qcow2"}
	err = logger.LogConversionComplete("task-123", "test-vm", 3*time.Minute, files)
	if err != nil {
		t.Fatalf("Failed to log conversion complete: %v", err)
	}
}

func TestAuditLogger_LogUpload(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Test upload start
	err = logger.LogUploadStart("task-123", "test-vm", "s3://bucket/path")
	if err != nil {
		t.Fatalf("Failed to log upload start: %v", err)
	}

	// Test upload complete
	duration := 10 * time.Minute
	bytesUploaded := int64(1024 * 1024 * 1024)
	err = logger.LogUploadComplete("task-123", "test-vm", "s3://bucket/path", duration, bytesUploaded)
	if err != nil {
		t.Fatalf("Failed to log upload complete: %v", err)
	}
}

func TestAuditLogger_LogConfigChange(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	changes := map[string]interface{}{
		"max_workers": map[string]interface{}{
			"old": 5,
			"new": 10,
		},
	}

	err = logger.LogConfigChange("admin", "worker_config", changes)
	if err != nil {
		t.Fatalf("Failed to log config change: %v", err)
	}
}

func TestAuditLogger_LogAPIAccess(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	err = logger.LogAPIAccess("api-user", "192.168.1.100", "curl/7.68.0", "POST", "/api/v1/jobs", 200)
	if err != nil {
		t.Fatalf("Failed to log API access: %v", err)
	}
}

func TestAuditLogger_LogWarning(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	err = logger.LogWarning("task-123", "test-vm", "Disk space below 10%")
	if err != nil {
		t.Fatalf("Failed to log warning: %v", err)
	}
}

func TestAuditLogger_LogError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	testErr := fmt.Errorf("connection timeout")
	err = logger.LogError("task-123", "test-vm", testErr)
	if err != nil {
		t.Fatalf("Failed to log error: %v", err)
	}
}

func TestAuditLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}

	// Log an event
	event := &AuditEvent{
		EventType: EventMigrationStart,
		Action:    "test",
		Status:    "started",
		Success:   true,
	}

	err = logger.Log(event)
	if err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}

	// Close logger
	err = logger.Close()
	if err != nil {
		t.Fatalf("Failed to close logger: %v", err)
	}

	// Closing again should not error
	err = logger.Close()
	if err != nil {
		t.Errorf("Expected no error on second close, got: %v", err)
	}
}

func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	id2 := generateEventID()

	if id1 == "" {
		t.Error("Expected non-empty event ID")
	}

	if id1 == id2 {
		t.Error("Expected unique event IDs")
	}

	// Should be in format: timestamp-random
	if len(id1) < 20 {
		t.Errorf("Event ID seems too short: %s", id1)
	}
}

func TestQueryAuditLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}

	// Log various events
	err = logger.LogMigrationStart("task-1", "vm-1", "vsphere", "user1")
	if err != nil {
		t.Fatalf("Failed to log migration start: %v", err)
	}

	err = logger.LogMigrationComplete("task-2", "vm-2", "vsphere", "user2", 5*time.Minute, nil)
	if err != nil {
		t.Fatalf("Failed to log migration complete: %v", err)
	}

	err = logger.LogError("task-3", "vm-3", fmt.Errorf("test error"))
	if err != nil {
		t.Fatalf("Failed to log error: %v", err)
	}

	logger.Close()

	// Query all events
	events, err := QueryAuditLogs(logPath, QueryOptions{})
	if err != nil {
		t.Fatalf("Failed to query audit logs: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	// Query by task ID
	events, err = QueryAuditLogs(logPath, QueryOptions{
		TaskID: "task-1",
	})
	if err != nil {
		t.Fatalf("Failed to query by task ID: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event for task-1, got %d", len(events))
	}

	// Query by VM name
	events, err = QueryAuditLogs(logPath, QueryOptions{
		VMName: "vm-2",
	})
	if err != nil {
		t.Fatalf("Failed to query by VM name: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event for vm-2, got %d", len(events))
	}

	// Query by event type
	events, err = QueryAuditLogs(logPath, QueryOptions{
		EventTypes: []AuditEventType{EventError},
	})
	if err != nil {
		t.Fatalf("Failed to query by event type: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 error event, got %d", len(events))
	}

	if events[0].EventType != EventError {
		t.Errorf("Expected EventError, got %s", events[0].EventType)
	}

	// Query by success
	successFilter := true
	events, err = QueryAuditLogs(logPath, QueryOptions{
		Success: &successFilter,
	})
	if err != nil {
		t.Fatalf("Failed to query by success: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 successful events, got %d", len(events))
	}

	// Query with limit
	events, err = QueryAuditLogs(logPath, QueryOptions{
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("Failed to query with limit: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events with limit, got %d", len(events))
	}
}

func TestAuditLogger_Rotate(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create logger with small max file size to trigger rotation
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Set small max file size and files to test rotation
	logger.rotateSize = 100 // 100 bytes
	logger.maxFiles = 3

	// Write initial data
	event1 := &AuditEvent{
		EventType:   EventMigrationStart,
		TaskID:      "task-1",
		Action:      "test",
		Description: "Test event for rotation",
		Timestamp:   time.Now(),
	}

	err = logger.Log(event1)
	if err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}

	// Write more data to exceed max file size
	for i := 0; i < 10; i++ {
		event := &AuditEvent{
			EventType:   EventMigrationStart,
			TaskID:      fmt.Sprintf("task-%d", i),
			Action:      "test",
			Description: "This is a longer description to increase file size and trigger rotation mechanism",
			Timestamp:   time.Now(),
		}
		logger.Log(event)
	}

	// Manually trigger rotation to test the function
	err = logger.rotate()
	if err != nil {
		t.Fatalf("Failed to rotate log: %v", err)
	}

	// Verify rotated file exists
	rotatedFile := logPath + ".1"
	if _, err := os.Stat(rotatedFile); os.IsNotExist(err) {
		t.Error("Expected rotated file to exist")
	}

	// Verify new log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Expected new log file to exist after rotation")
	}

	// Verify current size was reset
	if logger.currentSize != 0 {
		t.Errorf("Expected currentSize to be reset to 0, got %d", logger.currentSize)
	}

	// Write to new file to verify it works
	event2 := &AuditEvent{
		EventType:   EventMigrationComplete,
		TaskID:      "task-after-rotation",
		Action:      "test",
		Description: "Test after rotation",
		Timestamp:   time.Now(),
	}

	err = logger.Log(event2)
	if err != nil {
		t.Fatalf("Failed to log event after rotation: %v", err)
	}
}

func TestAuditLogger_MultipleRotations(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Set small max file size and max 2 rotated files
	logger.rotateSize = 50
	logger.maxFiles = 2

	// Rotate multiple times
	for i := 0; i < 3; i++ {
		// Write some data
		for j := 0; j < 5; j++ {
			event := &AuditEvent{
				EventType:   EventMigrationStart,
				TaskID:      fmt.Sprintf("task-%d-%d", i, j),
				Action:      "test",
				Description: "Test data for rotation",
				Timestamp:   time.Now(),
			}
			logger.Log(event)
		}

		// Rotate
		err = logger.rotate()
		if err != nil {
			t.Fatalf("Failed to rotate log (iteration %d): %v", i, err)
		}
	}

	// Verify .1 and .2 exist
	file1 := logPath + ".1"
	file2 := logPath + ".2"

	if _, err := os.Stat(file1); os.IsNotExist(err) {
		t.Error("Expected .1 rotated file to exist")
	}

	if _, err := os.Stat(file2); os.IsNotExist(err) {
		t.Error("Expected .2 rotated file to exist")
	}

	// .3 should not exist (maxFiles is 2)
	file3 := logPath + ".3"
	if _, err := os.Stat(file3); err == nil {
		t.Error("Expected .3 file to not exist (should be removed)")
	}
}
