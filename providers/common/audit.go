// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	EventMigrationStart     AuditEventType = "migration_start"
	EventMigrationComplete  AuditEventType = "migration_complete"
	EventMigrationFailed    AuditEventType = "migration_failed"
	EventExportStart        AuditEventType = "export_start"
	EventExportComplete     AuditEventType = "export_complete"
	EventConversionStart    AuditEventType = "conversion_start"
	EventConversionComplete AuditEventType = "conversion_complete"
	EventUploadStart        AuditEventType = "upload_start"
	EventUploadComplete     AuditEventType = "upload_complete"
	EventConfigChange       AuditEventType = "config_change"
	EventAPIAccess          AuditEventType = "api_access"
	EventWarning            AuditEventType = "warning"
	EventError              AuditEventType = "error"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	// Event metadata
	EventID   string         `json:"event_id"`
	EventType AuditEventType `json:"event_type"`
	Timestamp time.Time      `json:"timestamp"`

	// Task information
	TaskID   string `json:"task_id,omitempty"`
	VMName   string `json:"vm_name,omitempty"`
	Provider string `json:"provider,omitempty"`

	// User/source information
	User      string `json:"user,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`

	// Action details
	Action      string                 `json:"action"`
	Description string                 `json:"description,omitempty"`
	Status      string                 `json:"status"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`

	// Result
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`

	// Resource information
	Resources []string `json:"resources,omitempty"`

	// Changes (for config changes)
	Changes map[string]interface{} `json:"changes,omitempty"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	mu          sync.Mutex
	logFile     *os.File
	logPath     string
	rotateSize  int64 // Rotate when log exceeds this size
	maxFiles    int   // Keep this many rotated files
	currentSize int64
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logPath string) (*AuditLogger, error) {
	// Create log directory
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	// Open log file
	// #nosec G304 -- logPath is supplied by the local operator/service configuration, not remote/network input
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	// Get current file size
	stat, _ := file.Stat()
	currentSize := int64(0)
	if stat != nil {
		currentSize = stat.Size()
	}

	return &AuditLogger{
		logFile:     file,
		logPath:     logPath,
		rotateSize:  100 * 1024 * 1024, // 100 MB default
		maxFiles:    10,                // Keep 10 rotated files
		currentSize: currentSize,
	}, nil
}

// Log logs an audit event
func (al *AuditLogger) Log(event *AuditEvent) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	// Set event ID and timestamp if not already set
	if event.EventID == "" {
		event.EventID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	// Write to log file
	line := append(data, '\n')
	n, err := al.logFile.Write(line)
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	al.currentSize += int64(n)

	// Check if rotation is needed
	if al.currentSize >= al.rotateSize {
		if err := al.rotate(); err != nil {
			return fmt.Errorf("rotate log: %w", err)
		}
	}

	return nil
}

// rotate rotates the log file
func (al *AuditLogger) rotate() error {
	// Close current file
	if err := al.logFile.Close(); err != nil {
		return err
	}

	// Rotate existing files
	for i := al.maxFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", al.logPath, i)
		newPath := fmt.Sprintf("%s.%d", al.logPath, i+1)
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate %s to %s: %w", oldPath, newPath, err)
		}
	}

	// Rename current file to .1
	if err := os.Rename(al.logPath, al.logPath+".1"); err != nil {
		return err
	}

	// Remove oldest file if it exists
	oldestFile := fmt.Sprintf("%s.%d", al.logPath, al.maxFiles+1)
	if err := os.Remove(oldestFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove oldest audit log %s: %w", oldestFile, err)
	}

	// Open new log file
	// #nosec G304 -- al.logPath is supplied by the local operator/service configuration, not remote/network input
	file, err := os.OpenFile(al.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	al.logFile = file
	al.currentSize = 0

	return nil
}

// Close closes the audit logger
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.logFile != nil {
		err := al.logFile.Close()
		al.logFile = nil
		return err
	}
	return nil
}

// LogMigrationStart logs migration start
func (al *AuditLogger) LogMigrationStart(taskID, vmName, provider, user string) error {
	return al.Log(&AuditEvent{
		EventType:   EventMigrationStart,
		TaskID:      taskID,
		VMName:      vmName,
		Provider:    provider,
		User:        user,
		Action:      "start_migration",
		Description: fmt.Sprintf("Started migration for VM %s", vmName),
		Status:      "started",
		Success:     true,
	})
}

// LogMigrationComplete logs migration completion
func (al *AuditLogger) LogMigrationComplete(taskID, vmName, provider, user string, duration time.Duration, details map[string]interface{}) error {
	return al.Log(&AuditEvent{
		EventType:   EventMigrationComplete,
		TaskID:      taskID,
		VMName:      vmName,
		Provider:    provider,
		User:        user,
		Action:      "complete_migration",
		Description: fmt.Sprintf("Completed migration for VM %s", vmName),
		Status:      "completed",
		Duration:    duration,
		Details:     details,
		Success:     true,
	})
}

// LogMigrationFailed logs migration failure
func (al *AuditLogger) LogMigrationFailed(taskID, vmName, provider, user string, err error) error {
	return al.Log(&AuditEvent{
		EventType:   EventMigrationFailed,
		TaskID:      taskID,
		VMName:      vmName,
		Provider:    provider,
		User:        user,
		Action:      "fail_migration",
		Description: fmt.Sprintf("Failed migration for VM %s", vmName),
		Status:      "failed",
		Error:       err.Error(),
		Success:     false,
	})
}

// LogExportStart logs export start
func (al *AuditLogger) LogExportStart(taskID, vmName, provider string) error {
	return al.Log(&AuditEvent{
		EventType:   EventExportStart,
		TaskID:      taskID,
		VMName:      vmName,
		Provider:    provider,
		Action:      "start_export",
		Description: fmt.Sprintf("Started export for VM %s", vmName),
		Status:      "started",
		Success:     true,
	})
}

// LogExportComplete logs export completion
func (al *AuditLogger) LogExportComplete(taskID, vmName, provider string, duration time.Duration, bytesExported int64) error {
	return al.Log(&AuditEvent{
		EventType:   EventExportComplete,
		TaskID:      taskID,
		VMName:      vmName,
		Provider:    provider,
		Action:      "complete_export",
		Description: fmt.Sprintf("Completed export for VM %s", vmName),
		Status:      "completed",
		Duration:    duration,
		Details: map[string]interface{}{
			"bytes_exported": bytesExported,
		},
		Success: true,
	})
}

// LogConversionStart logs conversion start
func (al *AuditLogger) LogConversionStart(taskID, vmName string) error {
	return al.Log(&AuditEvent{
		EventType:   EventConversionStart,
		TaskID:      taskID,
		VMName:      vmName,
		Action:      "start_conversion",
		Description: fmt.Sprintf("Started conversion for VM %s", vmName),
		Status:      "started",
		Success:     true,
	})
}

// LogConversionComplete logs conversion completion
func (al *AuditLogger) LogConversionComplete(taskID, vmName string, duration time.Duration, files []string) error {
	return al.Log(&AuditEvent{
		EventType:   EventConversionComplete,
		TaskID:      taskID,
		VMName:      vmName,
		Action:      "complete_conversion",
		Description: fmt.Sprintf("Completed conversion for VM %s", vmName),
		Status:      "completed",
		Duration:    duration,
		Details: map[string]interface{}{
			"converted_files": files,
			"file_count":      len(files),
		},
		Success: true,
	})
}

// LogUploadStart logs upload start
func (al *AuditLogger) LogUploadStart(taskID, vmName, destination string) error {
	return al.Log(&AuditEvent{
		EventType:   EventUploadStart,
		TaskID:      taskID,
		VMName:      vmName,
		Action:      "start_upload",
		Description: fmt.Sprintf("Started upload for VM %s to %s", vmName, destination),
		Status:      "started",
		Details: map[string]interface{}{
			"destination": destination,
		},
		Success: true,
	})
}

// LogUploadComplete logs upload completion
func (al *AuditLogger) LogUploadComplete(taskID, vmName, destination string, duration time.Duration, bytesUploaded int64) error {
	return al.Log(&AuditEvent{
		EventType:   EventUploadComplete,
		TaskID:      taskID,
		VMName:      vmName,
		Action:      "complete_upload",
		Description: fmt.Sprintf("Completed upload for VM %s to %s", vmName, destination),
		Status:      "completed",
		Duration:    duration,
		Details: map[string]interface{}{
			"destination":    destination,
			"bytes_uploaded": bytesUploaded,
		},
		Success: true,
	})
}

// LogConfigChange logs configuration change
func (al *AuditLogger) LogConfigChange(user, configType string, changes map[string]interface{}) error {
	return al.Log(&AuditEvent{
		EventType:   EventConfigChange,
		User:        user,
		Action:      "change_config",
		Description: fmt.Sprintf("Changed %s configuration", configType),
		Status:      "completed",
		Changes:     changes,
		Success:     true,
	})
}

// LogAPIAccess logs API access
func (al *AuditLogger) LogAPIAccess(user, ipAddress, userAgent, method, path string, statusCode int) error {
	return al.Log(&AuditEvent{
		EventType:   EventAPIAccess,
		User:        user,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Action:      "api_access",
		Description: fmt.Sprintf("%s %s", method, path),
		Status:      fmt.Sprintf("status_%d", statusCode),
		Details: map[string]interface{}{
			"method":      method,
			"path":        path,
			"status_code": statusCode,
		},
		Success: statusCode < 400,
	})
}

// LogWarning logs a warning event
func (al *AuditLogger) LogWarning(taskID, vmName, warning string) error {
	return al.Log(&AuditEvent{
		EventType:   EventWarning,
		TaskID:      taskID,
		VMName:      vmName,
		Action:      "warning",
		Description: warning,
		Status:      "warning",
		Success:     true,
	})
}

// LogError logs an error event
func (al *AuditLogger) LogError(taskID, vmName string, err error) error {
	return al.Log(&AuditEvent{
		EventType:   EventError,
		TaskID:      taskID,
		VMName:      vmName,
		Action:      "error",
		Description: err.Error(),
		Status:      "error",
		Error:       err.Error(),
		Success:     false,
	})
}

var eventIDCounter uint64

// generateEventID generates a unique event ID. The counter guards against
// collisions when called twice within the same clock tick.
func generateEventID() string {
	seq := atomic.AddUint64(&eventIDCounter, 1)
	return fmt.Sprintf("event_%d_%d", time.Now().UnixNano(), seq)
}

// QueryOptions holds options for querying audit logs
type QueryOptions struct {
	StartTime  time.Time
	EndTime    time.Time
	EventTypes []AuditEventType
	TaskID     string
	VMName     string
	Provider   string
	User       string
	Success    *bool // nil = all, true = success only, false = failures only
	Limit      int
}

// QueryAuditLogs queries audit logs (reads from file)
func QueryAuditLogs(logPath string, options QueryOptions) ([]*AuditEvent, error) {
	// #nosec G304 -- logPath is supplied by the local operator/service configuration, not remote/network input
	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	var events []*AuditEvent
	decoder := json.NewDecoder(file)

	for {
		var event AuditEvent
		if err := decoder.Decode(&event); err != nil {
			break // End of file
		}

		// Apply filters
		if !options.StartTime.IsZero() && event.Timestamp.Before(options.StartTime) {
			continue
		}
		if !options.EndTime.IsZero() && event.Timestamp.After(options.EndTime) {
			continue
		}
		if len(options.EventTypes) > 0 {
			found := false
			for _, et := range options.EventTypes {
				if event.EventType == et {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if options.TaskID != "" && event.TaskID != options.TaskID {
			continue
		}
		if options.VMName != "" && event.VMName != options.VMName {
			continue
		}
		if options.Provider != "" && event.Provider != options.Provider {
			continue
		}
		if options.User != "" && event.User != options.User {
			continue
		}
		if options.Success != nil && event.Success != *options.Success {
			continue
		}

		events = append(events, &event)

		// Apply limit
		if options.Limit > 0 && len(events) >= options.Limit {
			break
		}
	}

	return events, nil
}
