// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestFormatBytesForTemplate(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "bytes less than 1KB",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "exactly 1KB",
			bytes:    1024,
			expected: "1.0 KiB",
		},
		{
			name:     "kilobytes",
			bytes:    2048,
			expected: "2.0 KiB",
		},
		{
			name:     "megabytes",
			bytes:    1024 * 1024 * 5,
			expected: "5.0 MiB",
		},
		{
			name:     "gigabytes",
			bytes:    1024 * 1024 * 1024 * 10,
			expected: "10.0 GiB",
		},
		{
			name:     "terabytes",
			bytes:    1024 * 1024 * 1024 * 1024 * 2,
			expected: "2.0 TiB",
		},
		{
			name:     "petabytes",
			bytes:    1024 * 1024 * 1024 * 1024 * 1024 * 3,
			expected: "3.0 PiB",
		},
		{
			name:     "fractional GB",
			bytes:    1024 * 1024 * 1024 * 5 / 2,
			expected: "2.5 GiB",
		},
		{
			name:     "small fractional MB",
			bytes:    1024 * 1024 * 3 / 2,
			expected: "1.5 MiB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytesForTemplate(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytesForTemplate(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestNewNotificationManager(t *testing.T) {
	log := logger.New("info")

	config := &EmailConfig{
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUsername: "user@example.com",
		SMTPPassword: "password",
		FromAddress:  "noreply@example.com",
		ToAddresses:  []string{"admin@example.com"},
		TLSEnabled:   true,
		AuthMethod:   "plain",
	}

	nm := NewNotificationManager(config, log)

	if nm == nil {
		t.Fatal("NewNotificationManager() returned nil")
	}

	if nm.config != config {
		t.Error("Expected config to be set")
	}

	if nm.log == nil {
		t.Error("Expected log to be set")
	}
}

func TestNewNotificationManager_NilConfig(t *testing.T) {
	log := logger.New("info")
	nm := NewNotificationManager(nil, log)

	if nm == nil {
		t.Fatal("NewNotificationManager() returned nil")
	}

	if nm.config != nil {
		t.Error("Expected config to be nil")
	}

	if nm.log == nil {
		t.Error("Expected log to be set")
	}
}

func TestBuildEmailMessage(t *testing.T) {
	log := logger.New("info")
	config := &EmailConfig{
		FromAddress: "noreply@example.com",
		ToAddresses: []string{"admin@example.com", "backup@example.com"},
	}

	nm := NewNotificationManager(config, log)

	subject := "Test Export Notification"
	body := "<html><body>Test Body</body></html>"

	message := nm.buildEmailMessage(subject, body)

	// Check message contains expected headers
	if !strings.Contains(message, "From: noreply@example.com\r\n") {
		t.Error("Expected From header in message")
	}

	if !strings.Contains(message, "To: admin@example.com\r\n") {
		t.Error("Expected To header with primary recipient")
	}

	if !strings.Contains(message, "Subject: Test Export Notification\r\n") {
		t.Error("Expected Subject header in message")
	}

	if !strings.Contains(message, "MIME-Version: 1.0\r\n") {
		t.Error("Expected MIME-Version header in message")
	}

	if !strings.Contains(message, "Content-Type: text/html; charset=UTF-8\r\n") {
		t.Error("Expected Content-Type header in message")
	}

	if !strings.Contains(message, "<html><body>Test Body</body></html>") {
		t.Error("Expected body in message")
	}
}

func TestRenderTemplate(t *testing.T) {
	log := logger.New("info")
	nm := NewNotificationManager(nil, log)

	notification := &ExportNotification{
		VMName:      "test-vm",
		Provider:    "vsphere",
		Format:      "vmdk",
		TotalSize:   1024 * 1024 * 500, // 500 MB
		FilesCount:  3,
		Compressed:  true,
		Verified:    true,
		StartTime:   time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
		Duration:    1 * time.Hour,
	}

	tmpl := `VM: {{.VMName}}, Provider: {{.Provider}}, Size: {{FormatBytes .TotalSize}}`

	result := nm.renderTemplate(tmpl, notification)

	if !strings.Contains(result, "VM: test-vm") {
		t.Errorf("Expected template to contain VM name, got: %s", result)
	}

	if !strings.Contains(result, "Provider: vsphere") {
		t.Errorf("Expected template to contain provider, got: %s", result)
	}

	if !strings.Contains(result, "500.0 MiB") {
		t.Errorf("Expected template to contain formatted size, got: %s", result)
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	log := logger.New("info")
	nm := NewNotificationManager(nil, log)

	notification := &ExportNotification{
		VMName: "test-vm",
	}

	// Invalid template syntax
	tmpl := `{{.VMName`

	result := nm.renderTemplate(tmpl, notification)

	if result != "Failed to render email template" {
		t.Errorf("Expected error message, got: %s", result)
	}
}

func TestRenderStartTemplate(t *testing.T) {
	log := logger.New("info")
	nm := NewNotificationManager(nil, log)

	notification := &ExportNotification{
		VMName:    "production-vm",
		Provider:  "vsphere",
		Format:    "vmdk",
		StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
	}

	result := nm.renderStartTemplate(notification)

	// Check for expected content
	if !strings.Contains(result, "production-vm") {
		t.Error("Expected VM name in rendered template")
	}

	if !strings.Contains(result, "vsphere") {
		t.Error("Expected provider in rendered template")
	}

	if !strings.Contains(result, "vmdk") {
		t.Error("Expected format in rendered template")
	}

	if !strings.Contains(result, "Export Started") {
		t.Error("Expected 'Export Started' in rendered template")
	}

	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("Expected HTML document in rendered template")
	}
}

func TestRenderSuccessTemplate(t *testing.T) {
	log := logger.New("info")
	nm := NewNotificationManager(nil, log)

	notification := &ExportNotification{
		VMName:           "production-vm",
		Provider:         "vsphere",
		Format:           "vmdk",
		StartTime:        time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2024, 1, 1, 11, 30, 0, 0, time.UTC),
		Duration:         90 * time.Minute,
		TotalSize:        1024 * 1024 * 1024 * 10, // 10 GB
		FilesCount:       5,
		OutputDir:        "/exports/production-vm",
		Compressed:       true,
		Verified:         true,
		CloudDestination: "s3://backups/production-vm",
	}

	result := nm.renderSuccessTemplate(notification)

	// Check for expected content
	if !strings.Contains(result, "production-vm") {
		t.Error("Expected VM name in rendered template")
	}

	if !strings.Contains(result, "vsphere") {
		t.Error("Expected provider in rendered template")
	}

	if !strings.Contains(result, "vmdk") {
		t.Error("Expected format in rendered template")
	}

	if !strings.Contains(result, "compressed") {
		t.Error("Expected 'compressed' flag in rendered template")
	}

	if !strings.Contains(result, "/exports/production-vm") {
		t.Error("Expected output directory in rendered template")
	}

	if !strings.Contains(result, "Export verified with checksums") {
		t.Error("Expected verification message in rendered template")
	}

	if !strings.Contains(result, "s3://backups/production-vm") {
		t.Error("Expected cloud destination in rendered template")
	}

	if !strings.Contains(result, "Export Completed Successfully") {
		t.Error("Expected success message in rendered template")
	}
}

func TestRenderFailureTemplate(t *testing.T) {
	log := logger.New("info")
	nm := NewNotificationManager(nil, log)

	notification := &ExportNotification{
		VMName:       "production-vm",
		Provider:     "vsphere",
		StartTime:    time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:      time.Date(2024, 1, 1, 10, 15, 0, 0, time.UTC),
		ErrorMessage: "Failed to connect to vCenter: connection timeout",
	}

	result := nm.renderFailureTemplate(notification)

	// Check for expected content
	if !strings.Contains(result, "production-vm") {
		t.Error("Expected VM name in rendered template")
	}

	if !strings.Contains(result, "vsphere") {
		t.Error("Expected provider in rendered template")
	}

	if !strings.Contains(result, "Failed to connect to vCenter: connection timeout") {
		t.Error("Expected error message in rendered template")
	}

	if !strings.Contains(result, "Export Failed") {
		t.Error("Expected failure message in rendered template")
	}

	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("Expected HTML document in rendered template")
	}
}

func TestLoginAuth_Start(t *testing.T) {
	auth := &loginAuth{
		username: "testuser",
		password: "testpass",
	}

	proto, toServer, err := auth.Start(nil)

	if err != nil {
		t.Errorf("Start() returned error: %v", err)
	}

	if proto != "LOGIN" {
		t.Errorf("Expected protocol 'LOGIN', got %q", proto)
	}

	if len(toServer) != 0 {
		t.Errorf("Expected empty initial response, got %v", toServer)
	}
}

func TestLoginAuth_Next(t *testing.T) {
	auth := &loginAuth{
		username: "testuser",
		password: "testpass",
	}

	tests := []struct {
		name       string
		fromServer []byte
		more       bool
		expected   []byte
		expectErr  bool
	}{
		{
			name:       "username challenge",
			fromServer: []byte("Username:"),
			more:       true,
			expected:   []byte("testuser"),
			expectErr:  false,
		},
		{
			name:       "password challenge",
			fromServer: []byte("Password:"),
			more:       true,
			expected:   []byte("testpass"),
			expectErr:  false,
		},
		{
			name:       "unknown challenge",
			fromServer: []byte("Unknown:"),
			more:       true,
			expected:   nil,
			expectErr:  true,
		},
		{
			name:       "no more challenges",
			fromServer: []byte{},
			more:       false,
			expected:   nil,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := auth.Next(tt.fromServer, tt.more)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if string(result) != string(tt.expected) {
					t.Errorf("Next() = %q, want %q", result, tt.expected)
				}
			}
		})
	}
}
