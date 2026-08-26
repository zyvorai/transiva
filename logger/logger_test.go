// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug_level", "debug"},
		{"info_level", "info"},
		{"warn_level", "warn"},
		{"error_level", "error"},
		{"empty_level", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := New(tt.level)
			if log == nil {
				t.Fatal("New() returned nil logger")
			}
		})
	}
}

func TestLoggerMethods(t *testing.T) {
	// Test that all log methods can be called without panic
	log := New("debug")

	// These should not panic
	log.Debug("Debug message")
	log.Info("Info message")
	log.Warn("Warn message")
	log.Error("Error message")
}

func TestLoggerWithKeyValues(t *testing.T) {
	log := New("debug")

	// Test logging with key-value pairs
	log.Info("Test message", "key1", "value1", "key2", "value2")
	log.Debug("Debug with context", "vm_path", "/datacenter/vm/test", "status", "running")
	log.Warn("Warning with context", "error_count", 5)
	log.Error("Error with context", "failed_vm", "test-vm", "reason", "timeout")
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"warning", "warning"}, // alternative spelling
		{"error", "error"},
		{"invalid", "invalid"}, // should default to INFO
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := New(tt.level)
			if log == nil {
				t.Fatalf("New(%s) returned nil", tt.level)
			}

			// Should not panic
			log.Debug("test")
			log.Info("test")
			log.Warn("test")
			log.Error("test")
		})
	}
}

func TestStandardLogger(t *testing.T) {
	// Create a debug level logger
	log := New("debug")

	// Cast to concrete type for testing
	stdLog, ok := log.(*StandardLogger)
	if !ok {
		t.Fatal("Expected *StandardLogger type")
	}

	if stdLog.logger == nil {
		t.Error("StandardLogger.logger should not be nil")
	}

	// Verify level was set correctly
	if stdLog.level != DEBUG {
		t.Errorf("Expected DEBUG level, got %v", stdLog.level)
	}
}

func TestLoggerConcurrency(t *testing.T) {
	log := New("info")
	done := make(chan bool, 100)

	// Log from multiple goroutines
	for i := 0; i < 100; i++ {
		go func(index int) {
			log.Info("Concurrent log", "index", index)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// If we get here without panic, concurrency is safe
}

func TestNewTestLogger(t *testing.T) {
	testLog := NewTestLogger(t)
	if testLog == nil {
		t.Fatal("NewTestLogger() returned nil")
	}

	// Verify it implements the Logger interface
	var _ = testLog
}

func TestTestLogger_AllLevels(t *testing.T) {
	testLog := NewTestLogger(t)

	// Test all log levels without panic
	testLog.Debug("Debug message")
	testLog.Info("Info message")
	testLog.Warn("Warn message")
	testLog.Error("Error message")
}

func TestTestLogger_WithKeyValues(t *testing.T) {
	testLog := NewTestLogger(t)

	// Test logging with key-value pairs
	testLog.Debug("Debug with context", "key1", "value1")
	testLog.Info("Info with multiple pairs", "vm_name", "test-vm", "status", "running", "progress", 50)
	testLog.Warn("Warning with one pair", "error_count", 3)
	testLog.Error("Error with context", "vm_path", "/datacenter/vm/test", "error", "timeout")
}

func TestTestLogger_WithOddKeyValues(t *testing.T) {
	testLog := NewTestLogger(t)

	// Test with odd number of key-value pairs (should handle gracefully)
	testLog.Info("Message with odd pairs", "key1", "value1", "key2")
	testLog.Debug("Debug with single value", "lonely_key")
}

func TestTestLogger_EmptyKeyValues(t *testing.T) {
	testLog := NewTestLogger(t)

	// Test with no key-value pairs
	testLog.Debug("Just a message")
	testLog.Info("Another message")
	testLog.Warn("Warning message")
	testLog.Error("Error message")
}

func TestTestLogger_Format(t *testing.T) {
	testLog := NewTestLogger(t).(*TestLogger)

	tests := []struct {
		name          string
		level         string
		msg           string
		keysAndValues []interface{}
	}{
		{
			name:          "no pairs",
			level:         "INFO",
			msg:           "test message",
			keysAndValues: nil,
		},
		{
			name:          "one pair",
			level:         "DEBUG",
			msg:           "debug message",
			keysAndValues: []interface{}{"key1", "value1"},
		},
		{
			name:          "multiple pairs",
			level:         "WARN",
			msg:           "warning",
			keysAndValues: []interface{}{"key1", "value1", "key2", "value2"},
		},
		{
			name:          "odd number",
			level:         "ERROR",
			msg:           "error",
			keysAndValues: []interface{}{"key1", "value1", "key2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testLog.format(tt.level, tt.msg, tt.keysAndValues...)
			if result == "" {
				t.Error("format() returned empty string")
			}
			// Should contain level and message
			if len(result) < len(tt.level)+len(tt.msg) {
				t.Errorf("format() result too short: %s", result)
			}
		})
	}
}

// JSON Logger Tests

func TestNewWithConfig_JSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewWithConfig(Config{
		Level:  "info",
		Format: "json",
		Output: buf,
	})

	if log == nil {
		t.Fatal("NewWithConfig() returned nil logger")
	}

	stdLog, ok := log.(*StandardLogger)
	if !ok {
		t.Fatal("Expected *StandardLogger type")
	}

	if stdLog.format != FormatJSON {
		t.Errorf("Expected FormatJSON, got %v", stdLog.format)
	}
}

func TestJSONLogger_BasicMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewWithConfig(Config{
		Level:  "debug",
		Format: "json",
		Output: buf,
	})

	log.Info("test message")

	output := buf.String()
	if output == "" {
		t.Fatal("Expected output, got empty string")
	}

	// Parse JSON
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify fields
	if entry["level"] != "INFO" {
		t.Errorf("Expected level=INFO, got %v", entry["level"])
	}
	if entry["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got %v", entry["msg"])
	}
	if entry["timestamp"] == nil {
		t.Error("Expected timestamp field")
	}
}

func TestJSONLogger_WithKeyValues(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewWithConfig(Config{
		Level:  "debug",
		Format: "json",
		Output: buf,
	})

	log.Info("vm export started", "vm_path", "/datacenter/vm/test", "job_id", "abc123", "status", "running")

	output := buf.String()
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry["level"] != "INFO" {
		t.Errorf("Expected level=INFO, got %v", entry["level"])
	}
	if entry["msg"] != "vm export started" {
		t.Errorf("Expected msg='vm export started', got %v", entry["msg"])
	}
	if entry["vm_path"] != "/datacenter/vm/test" {
		t.Errorf("Expected vm_path='/datacenter/vm/test', got %v", entry["vm_path"])
	}
	if entry["job_id"] != "abc123" {
		t.Errorf("Expected job_id='abc123', got %v", entry["job_id"])
	}
	if entry["status"] != "running" {
		t.Errorf("Expected status='running', got %v", entry["status"])
	}
}

func TestJSONLogger_AllLevels(t *testing.T) {
	tests := []struct {
		name          string
		logFunc       func(Logger)
		expectedLevel string
	}{
		{
			name:          "debug level",
			logFunc:       func(l Logger) { l.Debug("debug message") },
			expectedLevel: "DEBUG",
		},
		{
			name:          "info level",
			logFunc:       func(l Logger) { l.Info("info message") },
			expectedLevel: "INFO",
		},
		{
			name:          "warn level",
			logFunc:       func(l Logger) { l.Warn("warn message") },
			expectedLevel: "WARN",
		},
		{
			name:          "error level",
			logFunc:       func(l Logger) { l.Error("error message") },
			expectedLevel: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			log := NewWithConfig(Config{
				Level:  "debug",
				Format: "json",
				Output: buf,
			})

			tt.logFunc(log)

			var entry map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			if entry["level"] != tt.expectedLevel {
				t.Errorf("Expected level=%s, got %v", tt.expectedLevel, entry["level"])
			}
		})
	}
}

func TestJSONLogger_LevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewWithConfig(Config{
		Level:  "warn", // Only WARN and ERROR should be logged
		Format: "json",
		Output: buf,
	})

	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should only have 2 lines (WARN and ERROR)
	if len(lines) != 2 {
		t.Errorf("Expected 2 log lines, got %d: %v", len(lines), lines)
	}

	// Verify first line is WARN
	var warn map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &warn); err != nil {
		t.Fatalf("Failed to parse WARN JSON: %v", err)
	}
	if warn["level"] != "WARN" {
		t.Errorf("Expected first line to be WARN, got %v", warn["level"])
	}

	// Verify second line is ERROR
	var errEntry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &errEntry); err != nil {
		t.Fatalf("Failed to parse ERROR JSON: %v", err)
	}
	if errEntry["level"] != "ERROR" {
		t.Errorf("Expected second line to be ERROR, got %v", errEntry["level"])
	}
}

func TestJSONLogger_NumericValues(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewWithConfig(Config{
		Level:  "info",
		Format: "json",
		Output: buf,
	})

	log.Info("metrics", "cpu_percent", 75.5, "memory_mb", 2048, "goroutines", 42)

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if entry["cpu_percent"] != 75.5 {
		t.Errorf("Expected cpu_percent=75.5, got %v", entry["cpu_percent"])
	}
	if entry["memory_mb"] != float64(2048) {
		t.Errorf("Expected memory_mb=2048, got %v", entry["memory_mb"])
	}
	if entry["goroutines"] != float64(42) {
		t.Errorf("Expected goroutines=42, got %v", entry["goroutines"])
	}
}

func TestTextLogger_StillWorks(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewWithConfig(Config{
		Level:  "info",
		Format: "text",
		Output: buf,
	})

	log.Info("test message", "key1", "value1")

	output := buf.String()
	if output == "" {
		t.Fatal("Expected output, got empty string")
	}

	// Should not be JSON
	if strings.HasPrefix(output, "{") {
		t.Error("Text format output should not be JSON")
	}

	// Should contain the message
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %s", output)
	}

	// Should contain the key-value pair
	if !strings.Contains(output, "key1=value1") {
		t.Errorf("Expected output to contain 'key1=value1', got: %s", output)
	}
}

func TestNewWithConfig_NilOutput(t *testing.T) {
	log := NewWithConfig(Config{
		Level:  "info",
		Format: "json",
		Output: nil, // Should default to os.Stderr
	})

	if log == nil {
		t.Fatal("NewWithConfig() with nil output returned nil logger")
	}

	stdLog, ok := log.(*StandardLogger)
	if !ok {
		t.Fatal("Expected *StandardLogger type")
	}

	if stdLog.logger == nil {
		t.Error("StandardLogger.logger should not be nil")
	}
}
