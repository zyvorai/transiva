// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// getValidConfig returns a valid configuration for testing
func getValidConfig() *Config {
	return &Config{
		Daemon: DaemonConfig{
			Port:        8080,
			Host:        "localhost",
			LogLevel:    "info",
			Timeout:     30 * time.Second,
			MaxRequests: 100,
		},
		Dashboard: DashboardConfig{
			Enabled:        false,
			Port:           8081,
			UpdateInterval: 1 * time.Second,
			MaxClients:     100,
		},
		Queue: QueueConfig{
			MaxWorkers:     10,
			MaxQueueSize:   1000,
			DefaultTimeout: 5 * time.Minute,
			EnableMetrics:  true,
		},
		Cache: CacheConfig{
			Backend:       "memory",
			MaxMemorySize: 100 * 1024 * 1024,
			DefaultTTL:    1 * time.Hour,
		},
		Backup: BackupConfig{
			Enabled:       false,
			BackupDir:     "/tmp/backups",
			MaxBackups:    10,
			RetentionDays: 30,
			Interval:      24 * time.Hour,
		},
		Monitoring: MonitoringConfig{
			Enabled:        false,
			PrometheusPort: 9090,
			TracingEnabled: false,
		},
		Auth: AuthConfig{
			Enabled:       false,
			JWTSecret:     "this-is-a-very-long-secret-key-for-jwt",
			SessionExpiry: 3600,
			AllowedUsers:  []string{"user1", "user2"},
		},
		RateLimit: RateLimitConfig{
			Enabled:        false,
			RequestsPerMin: 100,
			BurstSize:      150,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     30,
		},
	}
}

func TestValidatorValidConfig(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:        8080,
			Host:        "localhost",
			LogLevel:    "info",
			Timeout:     30 * time.Second,
			MaxRequests: 100,
		},
		Dashboard: DashboardConfig{
			Enabled:        true,
			Port:           8081,
			UpdateInterval: 1 * time.Second,
			MaxClients:     100,
		},
		Queue: QueueConfig{
			MaxWorkers:     10,
			MaxQueueSize:   1000,
			DefaultTimeout: 5 * time.Minute,
			EnableMetrics:  true,
		},
		Cache: CacheConfig{
			Backend:       "memory",
			MaxMemorySize: 100 * 1024 * 1024,
			DefaultTTL:    1 * time.Hour,
		},
		Backup: BackupConfig{
			Enabled:       true,
			BackupDir:     "/tmp/backups",
			MaxBackups:    10,
			RetentionDays: 30,
			Interval:      24 * time.Hour,
		},
		Monitoring: MonitoringConfig{
			Enabled:        true,
			PrometheusPort: 9090,
			TracingEnabled: false,
		},
		Auth: AuthConfig{
			Enabled:       true,
			JWTSecret:     "this-is-a-very-long-secret-key-for-jwt",
			SessionExpiry: 3600,
			AllowedUsers:  []string{"user1", "user2"},
		},
		RateLimit: RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 100,
			BurstSize:      150,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     30,
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Errorf("expected valid config, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("  - %s", err.Error())
		}
	}
}

func TestValidateDaemonInvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port too low", 0},
		{"port too high", 65536},
		{"port negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Daemon: DaemonConfig{
					Port:     tt.port,
					LogLevel: "info",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			validator := NewValidator(config)
			result := validator.Validate()

			if result.Valid {
				t.Error("expected invalid config")
			}

			found := false
			for _, err := range result.Errors {
				if err.Field == "daemon.port" {
					found = true
					break
				}
			}

			if !found {
				t.Error("expected daemon.port error")
			}
		})
	}
}

func TestValidateDaemonInvalidHost(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			Host:     "invalid host@#$",
			LogLevel: "info",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "daemon.host" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected daemon.host error")
	}
}

func TestValidateDaemonInvalidLogLevel(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "invalid",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "daemon.log_level" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected daemon.log_level error")
	}
}

func TestValidateDaemonNegativeTimeout(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
			Timeout:  -1 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "daemon.timeout" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected daemon.timeout error")
	}
}

func TestValidateDaemonShortTimeoutWarning(t *testing.T) {
	config := getValidConfig()
	config.Daemon.Timeout = 1 * time.Second

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Errorf("expected valid config with warnings, got errors: %v", result.Errors)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning about short timeout")
	}
}

func TestValidateDashboardPortConflict(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Dashboard: DashboardConfig{
			Enabled:        true,
			Port:           8080, // Same as daemon
			UpdateInterval: 1 * time.Second,
			MaxClients:     10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config due to port conflict")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "dashboard.port" && err.Message == "port conflicts with daemon port" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected dashboard.port conflict error")
	}
}

func TestValidateDashboardInvalidUpdateInterval(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Dashboard: DashboardConfig{
			Enabled:        true,
			Port:           8081,
			UpdateInterval: -1 * time.Second,
			MaxClients:     10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "dashboard.update_interval" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected dashboard.update_interval error")
	}
}

func TestValidateQueueInvalidWorkers(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Queue: QueueConfig{
			MaxWorkers:     0,
			MaxQueueSize:   1000,
			DefaultTimeout: 5 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "queue.max_workers" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected queue.max_workers error")
	}
}

func TestValidateQueueInvalidQueueSize(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Queue: QueueConfig{
			MaxWorkers:     10,
			MaxQueueSize:   0,
			DefaultTimeout: 5 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "queue.max_queue_size" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected queue.max_queue_size error")
	}
}

func TestValidateCacheInvalidBackend(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Cache: CacheConfig{
			Backend: "invalid",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "cache.backend" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected cache.backend error")
	}
}

func TestValidateCacheRedisNoAddress(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Cache: CacheConfig{
			Backend:   "redis",
			RedisAddr: "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "cache.redis_addr" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected cache.redis_addr error")
	}
}

func TestValidateCacheRedisInvalidAddress(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Cache: CacheConfig{
			Backend:   "redis",
			RedisAddr: "invalid-address",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "cache.redis_addr" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected cache.redis_addr error")
	}
}

func TestValidateCacheRedisInvalidDB(t *testing.T) {
	tests := []struct {
		name string
		db   int
	}{
		{"db too low", -1},
		{"db too high", 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Daemon: DaemonConfig{
					Port:     8080,
					LogLevel: "info",
				},
				Cache: CacheConfig{
					Backend:   "redis",
					RedisAddr: "localhost:6379",
					RedisDB:   tt.db,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			validator := NewValidator(config)
			result := validator.Validate()

			if result.Valid {
				t.Error("expected invalid config")
			}

			found := false
			for _, err := range result.Errors {
				if err.Field == "cache.redis_db" {
					found = true
					break
				}
			}

			if !found {
				t.Error("expected cache.redis_db error")
			}
		})
	}
}

func TestValidateBackupNoDirectory(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Backup: BackupConfig{
			Enabled:   true,
			BackupDir: "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "backup.backup_dir" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected backup.backup_dir error")
	}
}

func TestValidateBackupInvalidMaxBackups(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Backup: BackupConfig{
			Enabled:    true,
			BackupDir:  tmpDir,
			MaxBackups: 0,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "backup.max_backups" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected backup.max_backups error")
	}
}

func TestValidateMonitoringPortConflicts(t *testing.T) {
	tests := []struct {
		name           string
		daemonPort     int
		dashboardPort  int
		prometheusPort int
		expectError    bool
	}{
		{"conflict with daemon", 9090, 8081, 9090, true},
		{"conflict with dashboard", 8080, 9090, 9090, true},
		{"no conflict", 8080, 8081, 9090, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Daemon: DaemonConfig{
					Port:     tt.daemonPort,
					LogLevel: "info",
				},
				Dashboard: DashboardConfig{
					Enabled: true,
					Port:    tt.dashboardPort,
				},
				Monitoring: MonitoringConfig{
					Enabled:        true,
					PrometheusPort: tt.prometheusPort,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			validator := NewValidator(config)
			result := validator.Validate()

			if tt.expectError && result.Valid {
				t.Error("expected invalid config due to port conflict")
			}

			if tt.expectError {
				found := false
				for _, err := range result.Errors {
					if err.Field == "monitoring.prometheus_port" {
						found = true
						break
					}
				}

				if !found {
					t.Error("expected monitoring.prometheus_port error")
				}
			}
		})
	}
}

func TestValidateMonitoringTracingNoURL(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Monitoring: MonitoringConfig{
			Enabled:        true,
			PrometheusPort: 9090,
			TracingEnabled: true,
			TracingURL:     "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "monitoring.tracing_url" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected monitoring.tracing_url error")
	}
}

func TestValidateAuthNoSecret(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Auth: AuthConfig{
			Enabled:   true,
			JWTSecret: "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "auth.jwt_secret" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected auth.jwt_secret error")
	}
}

func TestValidateAuthShortSecretWarning(t *testing.T) {
	config := getValidConfig()
	config.Auth.Enabled = true
	config.Auth.JWTSecret = "short"

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Errorf("expected valid config with warnings, got errors: %v", result.Errors)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning about short JWT secret")
	}
}

func TestValidateAuthInvalidSessionExpiry(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Auth: AuthConfig{
			Enabled:       true,
			JWTSecret:     "this-is-a-very-long-secret-key",
			SessionExpiry: -1,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "auth.session_expiry" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected auth.session_expiry error")
	}
}

func TestValidateRateLimitInvalidRequests(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		RateLimit: RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 0,
			BurstSize:      10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "rate_limit.requests_per_min" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected rate_limit.requests_per_min error")
	}
}

func TestValidateRateLimitInvalidBurstSize(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		RateLimit: RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 100,
			BurstSize:      0,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "rate_limit.burst_size" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected rate_limit.burst_size error")
	}
}

func TestValidateLoggingInvalidLevel(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Logging: LoggingConfig{
			Level:  "invalid",
			Format: "json",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "logging.level" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected logging.level error")
	}
}

func TestValidateLoggingInvalidFormat(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "invalid",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config")
	}

	found := false
	for _, err := range result.Errors {
		if err.Field == "logging.format" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected logging.format error")
	}
}

func TestValidateLoggingInvalidOutput(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     8080,
			LogLevel: "info",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "/nonexistent/directory/logfile.log",
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	// This should produce a warning, not an error
	if len(result.Warnings) == 0 {
		t.Error("expected warning about nonexistent directory")
	}
}

func TestValidateLoggingNegativeValues(t *testing.T) {
	tests := []struct {
		name        string
		maxSize     int
		maxBackups  int
		maxAge      int
		expectField string
	}{
		{"negative max_size", -1, 5, 30, "logging.max_size"},
		{"negative max_backups", 100, -1, 30, "logging.max_backups"},
		{"negative max_age", 100, 5, -1, "logging.max_age"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Daemon: DaemonConfig{
					Port:     8080,
					LogLevel: "info",
				},
				Logging: LoggingConfig{
					Level:      "info",
					Format:     "json",
					MaxSize:    tt.maxSize,
					MaxBackups: tt.maxBackups,
					MaxAge:     tt.maxAge,
				},
			}

			validator := NewValidator(config)
			result := validator.Validate()

			if result.Valid {
				t.Error("expected invalid config")
			}

			found := false
			for _, err := range result.Errors {
				if err.Field == tt.expectField {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected %s error", tt.expectField)
			}
		})
	}
}

func TestValidateAddressValid(t *testing.T) {
	tests := []string{
		"localhost:6379",
		"127.0.0.1:6379",
		"redis.example.com:6379",
		"10.0.0.1:1234",
	}

	for _, addr := range tests {
		t.Run(addr, func(t *testing.T) {
			err := validateAddress(addr)
			if err != nil {
				t.Errorf("expected valid address, got error: %v", err)
			}
		})
	}
}

func TestValidateAddressInvalid(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{"no port", "localhost"},
		{"invalid port", "localhost:abc"},
		{"port too high", "localhost:99999"},
		{"port too low", "localhost:0"},
		{"invalid host", "invalid@host:6379"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddress(tt.addr)
			if err == nil {
				t.Error("expected error for invalid address")
			}
		})
	}
}

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		hostname string
		valid    bool
	}{
		{"localhost", true},
		{"example.com", true},
		{"sub.example.com", true},
		{"my-server", true},
		{"my-server.local", true},
		{"192.168.1.1", true}, // IP addresses are not hostnames, but we test the pattern
		{"invalid@host", false},
		{"-invalid", false},
		{"invalid-", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := isValidHostname(tt.hostname)
			if result != tt.valid {
				t.Errorf("expected %v, got %v", tt.valid, result)
			}
		})
	}
}

func TestValidationErrorString(t *testing.T) {
	err := &ValidationError{
		Field:   "test.field",
		Value:   123,
		Message: "test message",
	}

	expected := "validation error for 'test.field': test message (value: 123)"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestBackupDirectoryCreation(t *testing.T) {
	// Create a temp directory for the test
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")

	config := getValidConfig()
	config.Backup.Enabled = true
	config.Backup.BackupDir = backupDir
	config.Backup.MaxBackups = 10
	config.Backup.RetentionDays = 30
	config.Backup.Interval = 24 * time.Hour

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Errorf("expected valid config, got errors: %v", result.Errors)
	}

	// Check that directory was created
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error("expected backup directory to be created")
	}
}

func TestMultipleErrors(t *testing.T) {
	config := &Config{
		Daemon: DaemonConfig{
			Port:     -1,        // Invalid
			LogLevel: "invalid", // Invalid
		},
		Dashboard: DashboardConfig{
			Enabled:        true,
			Port:           99999,            // Invalid
			UpdateInterval: -1 * time.Second, // Invalid
			MaxClients:     0,                // Invalid
		},
		Logging: LoggingConfig{
			Level:  "invalid", // Invalid
			Format: "invalid", // Invalid
		},
	}

	validator := NewValidator(config)
	result := validator.Validate()

	if result.Valid {
		t.Error("expected invalid config with multiple errors")
	}

	// Should have at least 7 errors
	if len(result.Errors) < 7 {
		t.Errorf("expected at least 7 errors, got %d", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("  - %s", err.Error())
		}
	}
}

func TestDisabledSectionsSkipValidation(t *testing.T) {
	config := getValidConfig()
	// Set invalid values for disabled sections
	config.Dashboard.Enabled = false
	config.Dashboard.Port = 99999 // Invalid, but should be ignored

	config.Backup.Enabled = false
	config.Backup.BackupDir = "" // Invalid, but should be ignored

	config.Monitoring.Enabled = false
	config.Monitoring.PrometheusPort = 99999 // Invalid, but should be ignored

	config.Auth.Enabled = false
	config.Auth.JWTSecret = "" // Invalid, but should be ignored

	config.RateLimit.Enabled = false
	config.RateLimit.RequestsPerMin = 0 // Invalid, but should be ignored

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Errorf("expected valid config (disabled sections should be skipped), got errors:")
		for _, err := range result.Errors {
			t.Logf("  - %s", err.Error())
		}
	}
}

func TestValidateQueueHighWorkersWarning(t *testing.T) {
	config := getValidConfig()
	config.Queue.MaxWorkers = 1500

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for very high max_workers")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "max_workers is very high") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about high max_workers")
	}
}

func TestValidateQueueLargeQueueSizeWarning(t *testing.T) {
	config := getValidConfig()
	config.Queue.MaxQueueSize = 2000000

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for very large max_queue_size")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "max_queue_size is very large") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about large max_queue_size")
	}
}

func TestValidateQueueShortTimeoutWarning(t *testing.T) {
	config := getValidConfig()
	config.Queue.DefaultTimeout = 30 * time.Second

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for very short default_timeout")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "default_timeout is very short") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about short default_timeout")
	}
}

func TestValidateCacheSmallMemoryWarning(t *testing.T) {
	config := getValidConfig()
	config.Cache.MaxMemorySize = 512 * 1024 // 512KB

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for very small max_memory_size")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "max_memory_size is very small") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about small max_memory_size")
	}
}

func TestValidateCacheLongTTLWarning(t *testing.T) {
	config := getValidConfig()
	config.Cache.DefaultTTL = 48 * time.Hour

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for very long default_ttl")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "default_ttl is very long") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about long default_ttl")
	}
}

func TestValidateBackupRelativePathWarning(t *testing.T) {
	config := getValidConfig()
	config.Backup.Enabled = true
	config.Backup.BackupDir = "relative/path"

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for non-absolute backup_dir")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "backup_dir is not an absolute path") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about non-absolute backup_dir")
	}
}

func TestValidateBackupZeroRetentionWarning(t *testing.T) {
	config := getValidConfig()
	config.Backup.Enabled = true
	config.Backup.RetentionDays = 0

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for zero retention_days")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "retention_days is 0") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about zero retention_days")
	}
}

func TestValidateBackupShortIntervalWarning(t *testing.T) {
	config := getValidConfig()
	config.Backup.Enabled = true
	config.Backup.Interval = 30 * time.Minute

	validator := NewValidator(config)
	result := validator.Validate()

	if !result.Valid {
		t.Error("expected valid config with warning")
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warning for very short backup interval")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "interval is very short") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("expected warning about short backup interval")
	}
}
