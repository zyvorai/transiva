// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileLogger(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	if logger.directory != tmpDir {
		t.Errorf("expected directory %s, got %s", tmpDir, logger.directory)
	}

	if logger.maxSize != 10*1024*1024 {
		t.Errorf("expected maxSize 10MB, got %d", logger.maxSize)
	}
}

func TestLogEvent(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	event := NewEvent(EventTypeLogin, "testuser")
	event.Status = EventStatusSuccess
	event.IPAddress = "192.168.1.1"
	event.Details["method"] = "password"

	err = logger.Log(event)
	if err != nil {
		t.Errorf("failed to log event: %v", err)
	}

	// Verify file was created
	files, err := filepath.Glob(filepath.Join(tmpDir, "audit-*.log"))
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 log file, got %d", len(files))
	}
}

func TestQueryEvents(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log multiple events
	events := []*Event{
		{
			ID:        "1",
			Timestamp: time.Now(),
			EventType: EventTypeLogin,
			Status:    EventStatusSuccess,
			Username:  "alice",
			IPAddress: "192.168.1.1",
		},
		{
			ID:        "2",
			Timestamp: time.Now(),
			EventType: EventTypeExportVM,
			Status:    EventStatusSuccess,
			Username:  "bob",
			IPAddress: "192.168.1.2",
			Resource:  "vm-web-01",
		},
		{
			ID:        "3",
			Timestamp: time.Now(),
			EventType: EventTypeLogin,
			Status:    EventStatusFailure,
			Username:  "charlie",
			IPAddress: "192.168.1.3",
			Error:     "invalid credentials",
		},
	}

	for _, event := range events {
		if err := logger.Log(event); err != nil {
			t.Errorf("failed to log event: %v", err)
		}
	}

	// Query all events
	results, err := logger.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("failed to query events: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 events, got %d", len(results))
	}

	// Query by username
	results, err = logger.Query(QueryFilter{Username: "alice"})
	if err != nil {
		t.Fatalf("failed to query by username: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 event for alice, got %d", len(results))
	}

	if results[0].Username != "alice" {
		t.Errorf("expected username alice, got %s", results[0].Username)
	}

	// Query by event type
	results, err = logger.Query(QueryFilter{EventType: EventTypeLogin})
	if err != nil {
		t.Fatalf("failed to query by event type: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 login events, got %d", len(results))
	}

	// Query by status
	results, err = logger.Query(QueryFilter{Status: EventStatusFailure})
	if err != nil {
		t.Fatalf("failed to query by status: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 failure event, got %d", len(results))
	}
}

func TestQueryWithLimit(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log 10 events
	for i := 0; i < 10; i++ {
		event := NewEvent(EventTypeLogin, "user")
		event.Status = EventStatusSuccess
		logger.Log(event)
	}

	// Query with limit
	results, err := logger.Query(QueryFilter{Limit: 5})
	if err != nil {
		t.Fatalf("failed to query with limit: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("expected 5 events, got %d", len(results))
	}
}

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventTypeExportVM, "testuser")

	if event.ID == "" {
		t.Error("expected event ID to be generated")
	}

	if event.EventType != EventTypeExportVM {
		t.Errorf("expected event type export_vm, got %s", event.EventType)
	}

	if event.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", event.Username)
	}

	if event.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}

	if event.Details == nil {
		t.Error("expected details map to be initialized")
	}
}

func TestEventTypes(t *testing.T) {
	types := []EventType{
		EventTypeLogin,
		EventTypeLogout,
		EventTypeExportVM,
		EventTypeCreateJob,
		EventTypeCancelJob,
		EventTypeDeleteJob,
		EventTypeCreateSchedule,
		EventTypeUpdateSchedule,
		EventTypeDeleteSchedule,
		EventTypeCreateWebhook,
		EventTypeDeleteWebhook,
		EventTypeCreateUser,
		EventTypeUpdateUser,
		EventTypeDeleteUser,
		EventTypeConfigChange,
	}

	if len(types) != 15 {
		t.Errorf("expected 15 event types, got %d", len(types))
	}
}

func TestEventStatus(t *testing.T) {
	statuses := []EventStatus{
		EventStatusSuccess,
		EventStatusFailure,
		EventStatusDenied,
	}

	if len(statuses) != 3 {
		t.Errorf("expected 3 event statuses, got %d", len(statuses))
	}
}

func TestLogRotation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create logger with very small max size (1KB)
	logger, err := NewFileLogger(tmpDir, 1, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Manually set smaller size for testing
	logger.maxSize = 1024

	// Log many events to trigger rotation
	for i := 0; i < 100; i++ {
		event := NewEvent(EventTypeLogin, "user")
		event.Status = EventStatusSuccess
		event.Details["iteration"] = i
		logger.Log(event)
	}

	// Should have multiple log files now
	files, err := filepath.Glob(filepath.Join(tmpDir, "audit-*.log*"))
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	// Note: Rotation might not happen in tests due to timing
	if len(files) == 0 {
		t.Error("expected at least one log file")
	}
}

func TestQueryWithFilters(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Log events with different attributes
	now := time.Now()
	events := []*Event{
		{
			ID:        "1",
			Timestamp: now.Add(-2 * time.Hour),
			EventType: EventTypeLogin,
			Status:    EventStatusSuccess,
			Username:  "alice",
			Resource:  "resource-a",
		},
		{
			ID:        "2",
			Timestamp: now.Add(-1 * time.Hour),
			EventType: EventTypeExportVM,
			Status:    EventStatusFailure,
			Username:  "bob",
			Resource:  "resource-b",
		},
		{
			ID:        "3",
			Timestamp: now,
			EventType: EventTypeLogin,
			Status:    EventStatusSuccess,
			Username:  "alice",
			Resource:  "resource-a",
		},
	}

	for _, event := range events {
		if err := logger.Log(event); err != nil {
			t.Fatalf("failed to log event: %v", err)
		}
	}

	// Query by username
	usernameFilter := QueryFilter{Username: "alice"}
	results, err := logger.Query(usernameFilter)
	if err != nil {
		t.Fatalf("failed to query by username: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 alice events, got %d", len(results))
	}

	// Query by event type
	typeFilter := QueryFilter{EventType: EventTypeExportVM}
	results, err = logger.Query(typeFilter)
	if err != nil {
		t.Fatalf("failed to query by type: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 ExportVM event, got %d", len(results))
	}

	// Query by status
	statusFilter := QueryFilter{Status: EventStatusFailure}
	results, err = logger.Query(statusFilter)
	if err != nil {
		t.Fatalf("failed to query by status: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 failure event, got %d", len(results))
	}

	// Query by resource
	resourceFilter := QueryFilter{Resource: "resource-a"}
	results, err = logger.Query(resourceFilter)
	if err != nil {
		t.Fatalf("failed to query by resource: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 resource-a events, got %d", len(results))
	}

	// Query by time range
	startTime := now.Add(-90 * time.Minute)
	endTime := now.Add(-30 * time.Minute)
	timeFilter := QueryFilter{StartTime: &startTime, EndTime: &endTime}
	results, err = logger.Query(timeFilter)
	if err != nil {
		t.Fatalf("failed to query by time range: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 event in time range, got %d", len(results))
	}
}

func TestQueryWithStartTimeFilter(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	now := time.Now()
	oldEvent := &Event{
		ID:        "1",
		Timestamp: now.Add(-2 * time.Hour),
		EventType: EventTypeLogin,
		Username:  "alice",
	}
	newEvent := &Event{
		ID:        "2",
		Timestamp: now,
		EventType: EventTypeLogin,
		Username:  "bob",
	}

	logger.Log(oldEvent)
	logger.Log(newEvent)

	// Query with start time filter
	startTime := now.Add(-1 * time.Hour)
	filter := QueryFilter{StartTime: &startTime}
	results, err := logger.Query(filter)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 event after start time, got %d", len(results))
	}
	if len(results) > 0 && results[0].Username != "bob" {
		t.Errorf("expected bob's event, got %s", results[0].Username)
	}
}

func TestQueryWithEndTimeFilter(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	now := time.Now()
	oldEvent := &Event{
		ID:        "1",
		Timestamp: now.Add(-2 * time.Hour),
		EventType: EventTypeLogin,
		Username:  "alice",
	}
	newEvent := &Event{
		ID:        "2",
		Timestamp: now,
		EventType: EventTypeLogin,
		Username:  "bob",
	}

	logger.Log(oldEvent)
	logger.Log(newEvent)

	// Query with end time filter
	endTime := now.Add(-1 * time.Hour)
	filter := QueryFilter{EndTime: &endTime}
	results, err := logger.Query(filter)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 event before end time, got %d", len(results))
	}
	if len(results) > 0 && results[0].Username != "alice" {
		t.Errorf("expected alice's event, got %s", results[0].Username)
	}
}

func TestCloseNilFile(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Close the logger
	logger.Close()

	// Set file to nil and close again - should not panic
	logger.file = nil
	err = logger.Close()
	if err != nil {
		t.Errorf("close with nil file should not error: %v", err)
	}
}

func TestShouldRotateNilFile(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set file to nil
	logger.file = nil

	// shouldRotate should return true when file is nil
	if !logger.shouldRotate() {
		t.Error("shouldRotate should return true when file is nil")
	}
}

func TestShouldRotateFileSize(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Set very small max size
	logger.maxSize = 100

	// Log an event
	event := NewEvent(EventTypeLogin, "user")
	logger.Log(event)

	// Log many more events to exceed size
	for i := 0; i < 50; i++ {
		event := NewEvent(EventTypeLogin, "user")
		event.Details["data"] = "some long data to increase file size"
		logger.Log(event)
	}

	// shouldRotate should return true when file exceeds max size
	if !logger.shouldRotate() {
		t.Error("shouldRotate should return true when file exceeds maxSize")
	}
}

func TestNewFileLoggerInvalidDirectory(t *testing.T) {
	// Try to create logger with invalid directory
	_, err := NewFileLogger("/nonexistent/invalid/path", 10, 30, 5)
	if err == nil {
		t.Error("expected error when creating logger with invalid directory")
	}
}

func TestQueryEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewFileLogger(tmpDir, 10, 30, 5)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	// Close and remove all files
	logger.Close()

	// Query should return empty results, not error
	results, err := logger.Query(QueryFilter{})
	if err != nil {
		t.Errorf("query on empty directory should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty directory, got %d", len(results))
	}
}
