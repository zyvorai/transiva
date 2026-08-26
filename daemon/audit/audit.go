// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventType represents the type of audit event
type EventType string

const (
	EventTypeLogin          EventType = "login"
	EventTypeLogout         EventType = "logout"
	EventTypeExportVM       EventType = "export_vm"
	EventTypeCreateJob      EventType = "create_job"
	EventTypeCancelJob      EventType = "cancel_job"
	EventTypeDeleteJob      EventType = "delete_job"
	EventTypeCreateSchedule EventType = "create_schedule"
	EventTypeUpdateSchedule EventType = "update_schedule"
	EventTypeDeleteSchedule EventType = "delete_schedule"
	EventTypeCreateWebhook  EventType = "create_webhook"
	EventTypeDeleteWebhook  EventType = "delete_webhook"
	EventTypeCreateUser     EventType = "create_user"
	EventTypeUpdateUser     EventType = "update_user"
	EventTypeDeleteUser     EventType = "delete_user"
	EventTypeConfigChange   EventType = "config_change"
)

// EventStatus represents the outcome of an audit event
type EventStatus string

const (
	EventStatusSuccess EventStatus = "success"
	EventStatusFailure EventStatus = "failure"
	EventStatusDenied  EventStatus = "denied"
)

// Event represents a single audit log entry
type Event struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	EventType EventType              `json:"event_type"`
	Status    EventStatus            `json:"status"`
	Username  string                 `json:"username"`
	UserID    string                 `json:"user_id,omitempty"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Resource  string                 `json:"resource,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  int64                  `json:"duration_ms,omitempty"` // Duration in milliseconds
}

// Logger interface for audit logging
type Logger interface {
	Log(event *Event) error
	Query(filter QueryFilter) ([]*Event, error)
	Close() error
}

// QueryFilter for filtering audit log queries
type QueryFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Username  string
	EventType EventType
	Status    EventStatus
	Resource  string
	Limit     int
	Offset    int
}

// FileLogger writes audit logs to rotating files
type FileLogger struct {
	directory  string
	file       *os.File
	mu         sync.Mutex
	maxSize    int64 // Maximum file size in bytes
	maxAge     int   // Maximum age in days
	maxBackups int   // Maximum number of backup files
}

// NewFileLogger creates a new file-based audit logger
func NewFileLogger(directory string, maxSizeMB int, maxAge, maxBackups int) (*FileLogger, error) {
	if err := os.MkdirAll(directory, 0750); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}

	fl := &FileLogger{
		directory:  directory,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxAge:     maxAge,
		maxBackups: maxBackups,
	}

	if err := fl.openNewFile(); err != nil {
		return nil, err
	}

	// Start cleanup goroutine
	go fl.cleanupOldFiles()

	return fl, nil
}

// Log writes an audit event to the log file
func (fl *FileLogger) Log(event *Event) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// Check if rotation is needed
	if fl.shouldRotate() {
		if err := fl.rotate(); err != nil {
			return fmt.Errorf("failed to rotate log: %w", err)
		}
	}

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write to file with newline
	data = append(data, '\n')
	if _, err := fl.file.Write(data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Sync to disk for important events
	if event.EventType == EventTypeLogin || event.EventType == EventTypeConfigChange {
		if err := fl.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync audit log: %w", err)
		}
	}

	return nil
}

// Query searches audit logs (simplified version - reads all files)
func (fl *FileLogger) Query(filter QueryFilter) ([]*Event, error) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	var events []*Event
	count := 0
	skipped := 0

	// Read current log file
	files, err := filepath.Glob(filepath.Join(fl.directory, "audit-*.log*"))
	if err != nil {
		return nil, fmt.Errorf("failed to list log files: %w", err)
	}

	// Read files in reverse chronological order
	for i := len(files) - 1; i >= 0; i-- {
		fileEvents, err := fl.readLogFile(files[i])
		if err != nil {
			continue // Skip corrupted files
		}

		// Filter and add events
		for _, event := range fileEvents {
			if fl.matchesFilter(event, filter) {
				if skipped < filter.Offset {
					skipped++
					continue
				}

				events = append(events, event)
				count++

				if filter.Limit > 0 && count >= filter.Limit {
					return events, nil
				}
			}
		}
	}

	return events, nil
}

// Close closes the audit logger
func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.file != nil {
		return fl.file.Close()
	}

	return nil
}

// openNewFile opens a new log file
func (fl *FileLogger) openNewFile() error {
	filename := filepath.Join(fl.directory, fmt.Sprintf("audit-%s.log",
		time.Now().Format("2006-01-02")))

	// #nosec G304 -- filename is derived from fl.directory, which is supplied by the local operator via config, not remote input
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	fl.file = file
	return nil
}

// shouldRotate checks if log rotation is needed
func (fl *FileLogger) shouldRotate() bool {
	if fl.file == nil {
		return true
	}

	info, err := fl.file.Stat()
	if err != nil {
		return true
	}

	// Rotate if file is too large
	if info.Size() >= fl.maxSize {
		return true
	}

	// Rotate if it's a new day
	fileDate := time.Now().Format("2006-01-02")
	if !contains(info.Name(), fileDate) {
		return true
	}

	return false
}

// rotate closes current file and opens a new one
func (fl *FileLogger) rotate() error {
	if fl.file != nil {
		if err := fl.file.Close(); err != nil {
			return fmt.Errorf("failed to close audit log for rotation: %w", err)
		}
	}

	return fl.openNewFile()
}

// cleanupOldFiles removes old log files
func (fl *FileLogger) cleanupOldFiles() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		fl.mu.Lock()

		files, err := filepath.Glob(filepath.Join(fl.directory, "audit-*.log*"))
		if err != nil {
			fl.mu.Unlock()
			continue
		}

		// Remove old files
		cutoff := time.Now().AddDate(0, 0, -fl.maxAge)
		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				if err := os.Remove(file); err != nil {
					fmt.Fprintf(os.Stderr, "audit: failed to remove old log file %s: %v\n", file, err)
				}
			}
		}

		fl.mu.Unlock()
	}
}

// readLogFile reads and parses a log file
func (fl *FileLogger) readLogFile(filename string) ([]*Event, error) {
	// #nosec G304 -- filename comes from filepath.Glob over fl.directory, which is supplied by the local operator via config, not remote input
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var events []*Event
	lines := splitLines(string(data))

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip malformed lines
		}

		events = append(events, &event)
	}

	return events, nil
}

// matchesFilter checks if an event matches the query filter
func (fl *FileLogger) matchesFilter(event *Event, filter QueryFilter) bool {
	if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
		return false
	}

	if filter.Username != "" && event.Username != filter.Username {
		return false
	}

	if filter.EventType != "" && event.EventType != filter.EventType {
		return false
	}

	if filter.Status != "" && event.Status != filter.Status {
		return false
	}

	if filter.Resource != "" && event.Resource != filter.Resource {
		return false
	}

	return true
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			(len(s) > len(substr) && s[len(s)-len(substr)-1:len(s)-1] == substr))
}

func splitLines(s string) []string {
	var lines []string
	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}

	if start < len(s) {
		lines = append(lines, s[start:])
	}

	return lines
}

// NewEvent creates a new audit event with auto-generated ID
func NewEvent(eventType EventType, username string) *Event {
	return &Event{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		EventType: eventType,
		Username:  username,
		Details:   make(map[string]interface{}),
	}
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}
