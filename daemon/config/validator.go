// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration validation for HyperSDK
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult contains the result of configuration validation
type ValidationResult struct {
	Valid    bool
	Errors   []*ValidationError
	Warnings []string
}

// AddError adds a validation error
func (r *ValidationResult) AddError(field string, value interface{}, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	})
}

// AddWarning adds a validation warning
func (r *ValidationResult) AddWarning(message string) {
	r.Warnings = append(r.Warnings, message)
}

// Config represents the application configuration
type Config struct {
	Daemon     DaemonConfig     `yaml:"daemon" json:"daemon"`
	Dashboard  DashboardConfig  `yaml:"dashboard" json:"dashboard"`
	Queue      QueueConfig      `yaml:"queue" json:"queue"`
	Cache      CacheConfig      `yaml:"cache" json:"cache"`
	Backup     BackupConfig     `yaml:"backup" json:"backup"`
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring"`
	Auth       AuthConfig       `yaml:"auth" json:"auth"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit" json:"rate_limit"`
	Logging    LoggingConfig    `yaml:"logging" json:"logging"`
}

type DaemonConfig struct {
	Port        int           `yaml:"port" json:"port"`
	Host        string        `yaml:"host" json:"host"`
	LogLevel    string        `yaml:"log_level" json:"log_level"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout"`
	MaxRequests int           `yaml:"max_requests" json:"max_requests"`
}

type DashboardConfig struct {
	Enabled        bool          `yaml:"enabled" json:"enabled"`
	Port           int           `yaml:"port" json:"port"`
	UpdateInterval time.Duration `yaml:"update_interval" json:"update_interval"`
	MaxClients     int           `yaml:"max_clients" json:"max_clients"`
}

type QueueConfig struct {
	MaxWorkers     int           `yaml:"max_workers" json:"max_workers"`
	MaxQueueSize   int           `yaml:"max_queue_size" json:"max_queue_size"`
	DefaultTimeout time.Duration `yaml:"default_timeout" json:"default_timeout"`
	EnableMetrics  bool          `yaml:"enable_metrics" json:"enable_metrics"`
}

type CacheConfig struct {
	Backend       string        `yaml:"backend" json:"backend"`
	RedisAddr     string        `yaml:"redis_addr" json:"redis_addr"`
	RedisPassword string        `yaml:"redis_password" json:"redis_password"`
	RedisDB       int           `yaml:"redis_db" json:"redis_db"`
	MaxMemorySize int64         `yaml:"max_memory_size" json:"max_memory_size"`
	DefaultTTL    time.Duration `yaml:"default_ttl" json:"default_ttl"`
}

type BackupConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	BackupDir     string        `yaml:"backup_dir" json:"backup_dir"`
	MaxBackups    int           `yaml:"max_backups" json:"max_backups"`
	RetentionDays int           `yaml:"retention_days" json:"retention_days"`
	Interval      time.Duration `yaml:"interval" json:"interval"`
}

type MonitoringConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	PrometheusPort int    `yaml:"prometheus_port" json:"prometheus_port"`
	TracingEnabled bool   `yaml:"tracing_enabled" json:"tracing_enabled"`
	TracingURL     string `yaml:"tracing_url" json:"tracing_url"`
}

type AuthConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	JWTSecret     string   `yaml:"jwt_secret" json:"jwt_secret"`
	SessionExpiry int      `yaml:"session_expiry" json:"session_expiry"`
	AllowedUsers  []string `yaml:"allowed_users" json:"allowed_users"`
}

type RateLimitConfig struct {
	Enabled        bool `yaml:"enabled" json:"enabled"`
	RequestsPerMin int  `yaml:"requests_per_min" json:"requests_per_min"`
	BurstSize      int  `yaml:"burst_size" json:"burst_size"`
}

type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"`
	Output     string `yaml:"output" json:"output"`
	MaxSize    int    `yaml:"max_size" json:"max_size"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
	MaxAge     int    `yaml:"max_age" json:"max_age"`
}

// Validator validates configuration
type Validator struct {
	config *Config
	result *ValidationResult
}

// NewValidator creates a new configuration validator
func NewValidator(config *Config) *Validator {
	return &Validator{
		config: config,
		result: &ValidationResult{Valid: true},
	}
}

// Validate performs comprehensive validation
func (v *Validator) Validate() *ValidationResult {
	v.validateDaemon()
	v.validateDashboard()
	v.validateQueue()
	v.validateCache()
	v.validateBackup()
	v.validateMonitoring()
	v.validateAuth()
	v.validateRateLimit()
	v.validateLogging()

	return v.result
}

// validateDaemon validates daemon configuration
func (v *Validator) validateDaemon() {
	d := v.config.Daemon

	// Validate port
	if d.Port < 1 || d.Port > 65535 {
		v.result.AddError("daemon.port", d.Port, "port must be between 1 and 65535")
	}

	// Validate host
	if d.Host != "" {
		if net.ParseIP(d.Host) == nil && d.Host != "localhost" {
			v.result.AddError("daemon.host", d.Host, "invalid host address")
		}
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true,
	}
	if !validLogLevels[strings.ToLower(d.LogLevel)] {
		v.result.AddError("daemon.log_level", d.LogLevel, "invalid log level, must be one of: debug, info, warn, error, fatal")
	}

	// Validate timeout
	if d.Timeout < 0 {
		v.result.AddError("daemon.timeout", d.Timeout, "timeout cannot be negative")
	} else if d.Timeout < 5*time.Second {
		v.result.AddWarning("daemon.timeout is very short (< 5s), may cause premature timeouts")
	}

	// Validate max requests
	if d.MaxRequests < 0 {
		v.result.AddError("daemon.max_requests", d.MaxRequests, "max_requests cannot be negative")
	} else if d.MaxRequests == 0 {
		v.result.AddWarning("daemon.max_requests is 0, unlimited requests may cause resource exhaustion")
	}
}

// validateDashboard validates dashboard configuration
func (v *Validator) validateDashboard() {
	d := v.config.Dashboard

	if !d.Enabled {
		return
	}

	// Validate port
	if d.Port < 1 || d.Port > 65535 {
		v.result.AddError("dashboard.port", d.Port, "port must be between 1 and 65535")
	}

	// Check port conflict
	if d.Port == v.config.Daemon.Port {
		v.result.AddError("dashboard.port", d.Port, "port conflicts with daemon port")
	}

	// Validate update interval
	if d.UpdateInterval < 0 {
		v.result.AddError("dashboard.update_interval", d.UpdateInterval, "update_interval cannot be negative")
	} else if d.UpdateInterval < 100*time.Millisecond {
		v.result.AddWarning("dashboard.update_interval is very short (< 100ms), may cause high CPU usage")
	}

	// Validate max clients
	if d.MaxClients < 1 {
		v.result.AddError("dashboard.max_clients", d.MaxClients, "max_clients must be at least 1")
	} else if d.MaxClients > 10000 {
		v.result.AddWarning("dashboard.max_clients is very high (> 10000), may cause memory issues")
	}
}

// validateQueue validates queue configuration
func (v *Validator) validateQueue() {
	q := v.config.Queue

	// Validate max workers
	if q.MaxWorkers < 1 {
		v.result.AddError("queue.max_workers", q.MaxWorkers, "max_workers must be at least 1")
	} else if q.MaxWorkers > 1000 {
		v.result.AddWarning("queue.max_workers is very high (> 1000), may cause resource exhaustion")
	}

	// Validate max queue size
	if q.MaxQueueSize < 1 {
		v.result.AddError("queue.max_queue_size", q.MaxQueueSize, "max_queue_size must be at least 1")
	} else if q.MaxQueueSize > 1000000 {
		v.result.AddWarning("queue.max_queue_size is very large (> 1M), may cause memory issues")
	}

	// Validate default timeout
	if q.DefaultTimeout < 0 {
		v.result.AddError("queue.default_timeout", q.DefaultTimeout, "default_timeout cannot be negative")
	} else if q.DefaultTimeout < 1*time.Minute {
		v.result.AddWarning("queue.default_timeout is very short (< 1m), jobs may timeout prematurely")
	}
}

// validateCache validates cache configuration
func (v *Validator) validateCache() {
	c := v.config.Cache

	// Validate backend
	validBackends := map[string]bool{"redis": true, "memory": true}
	if !validBackends[c.Backend] {
		v.result.AddError("cache.backend", c.Backend, "backend must be 'redis' or 'memory'")
	}

	// Validate Redis configuration
	if c.Backend == "redis" {
		if c.RedisAddr == "" {
			v.result.AddError("cache.redis_addr", c.RedisAddr, "redis_addr is required when backend is 'redis'")
		} else {
			if err := validateAddress(c.RedisAddr); err != nil {
				v.result.AddError("cache.redis_addr", c.RedisAddr, err.Error())
			}
		}

		if c.RedisDB < 0 || c.RedisDB > 15 {
			v.result.AddError("cache.redis_db", c.RedisDB, "redis_db must be between 0 and 15")
		}
	}

	// Validate memory configuration
	if c.MaxMemorySize < 0 {
		v.result.AddError("cache.max_memory_size", c.MaxMemorySize, "max_memory_size cannot be negative")
	} else if c.MaxMemorySize < 1024*1024 {
		v.result.AddWarning("cache.max_memory_size is very small (< 1MB), may limit cache effectiveness")
	}

	// Validate default TTL
	if c.DefaultTTL < 0 {
		v.result.AddError("cache.default_ttl", c.DefaultTTL, "default_ttl cannot be negative")
	} else if c.DefaultTTL > 24*time.Hour {
		v.result.AddWarning("cache.default_ttl is very long (> 24h), may retain stale data")
	}
}

// validateBackup validates backup configuration
func (v *Validator) validateBackup() {
	b := v.config.Backup

	if !b.Enabled {
		return
	}

	// Validate backup directory
	if b.BackupDir == "" {
		v.result.AddError("backup.backup_dir", b.BackupDir, "backup_dir is required when backup is enabled")
	} else {
		if !filepath.IsAbs(b.BackupDir) {
			v.result.AddWarning("backup.backup_dir is not an absolute path, may cause issues")
		}

		// Check if directory exists or can be created
		if _, err := os.Stat(b.BackupDir); os.IsNotExist(err) {
			if err := os.MkdirAll(b.BackupDir, 0750); err != nil {
				v.result.AddError("backup.backup_dir", b.BackupDir, fmt.Sprintf("cannot create directory: %v", err))
			}
		}
	}

	// Validate max backups
	if b.MaxBackups < 1 {
		v.result.AddError("backup.max_backups", b.MaxBackups, "max_backups must be at least 1")
	}

	// Validate retention days
	if b.RetentionDays < 0 {
		v.result.AddError("backup.retention_days", b.RetentionDays, "retention_days cannot be negative")
	} else if b.RetentionDays == 0 {
		v.result.AddWarning("backup.retention_days is 0, backups will be retained indefinitely")
	}

	// Validate interval
	if b.Interval < 0 {
		v.result.AddError("backup.interval", b.Interval, "interval cannot be negative")
	} else if b.Interval < 1*time.Hour {
		v.result.AddWarning("backup.interval is very short (< 1h), may cause frequent backups")
	}
}

// validateMonitoring validates monitoring configuration
func (v *Validator) validateMonitoring() {
	m := v.config.Monitoring

	if !m.Enabled {
		return
	}

	// Validate Prometheus port
	if m.PrometheusPort < 1 || m.PrometheusPort > 65535 {
		v.result.AddError("monitoring.prometheus_port", m.PrometheusPort, "port must be between 1 and 65535")
	}

	// Check port conflicts
	if m.PrometheusPort == v.config.Daemon.Port {
		v.result.AddError("monitoring.prometheus_port", m.PrometheusPort, "port conflicts with daemon port")
	}
	if m.PrometheusPort == v.config.Dashboard.Port {
		v.result.AddError("monitoring.prometheus_port", m.PrometheusPort, "port conflicts with dashboard port")
	}

	// Validate tracing URL
	if m.TracingEnabled {
		if m.TracingURL == "" {
			v.result.AddError("monitoring.tracing_url", m.TracingURL, "tracing_url is required when tracing is enabled")
		} else {
			if _, err := url.Parse(m.TracingURL); err != nil {
				v.result.AddError("monitoring.tracing_url", m.TracingURL, fmt.Sprintf("invalid URL: %v", err))
			}
		}
	}
}

// validateAuth validates authentication configuration
func (v *Validator) validateAuth() {
	a := v.config.Auth

	if !a.Enabled {
		return
	}

	// Validate JWT secret
	if a.JWTSecret == "" {
		v.result.AddError("auth.jwt_secret", a.JWTSecret, "jwt_secret is required when auth is enabled")
	} else if len(a.JWTSecret) < 32 {
		v.result.AddWarning("auth.jwt_secret is short (< 32 chars), consider using a longer secret")
	}

	// Validate session expiry
	if a.SessionExpiry < 0 {
		v.result.AddError("auth.session_expiry", a.SessionExpiry, "session_expiry cannot be negative")
	} else if a.SessionExpiry < 300 {
		v.result.AddWarning("auth.session_expiry is very short (< 5 minutes), may cause frequent re-authentication")
	}

	// Validate allowed users
	if len(a.AllowedUsers) == 0 {
		v.result.AddWarning("auth.allowed_users is empty, no users will be able to authenticate")
	}
}

// validateRateLimit validates rate limiting configuration
func (v *Validator) validateRateLimit() {
	r := v.config.RateLimit

	if !r.Enabled {
		return
	}

	// Validate requests per minute
	if r.RequestsPerMin < 1 {
		v.result.AddError("rate_limit.requests_per_min", r.RequestsPerMin, "requests_per_min must be at least 1")
	}

	// Validate burst size
	if r.BurstSize < 1 {
		v.result.AddError("rate_limit.burst_size", r.BurstSize, "burst_size must be at least 1")
	} else if r.BurstSize < r.RequestsPerMin {
		v.result.AddWarning("rate_limit.burst_size is less than requests_per_min, may cause rate limiting issues")
	}
}

// validateLogging validates logging configuration
func (v *Validator) validateLogging() {
	l := v.config.Logging

	// Validate level
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true,
	}
	if !validLevels[strings.ToLower(l.Level)] {
		v.result.AddError("logging.level", l.Level, "invalid level, must be one of: debug, info, warn, error, fatal")
	}

	// Validate format
	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[strings.ToLower(l.Format)] {
		v.result.AddError("logging.format", l.Format, "invalid format, must be 'json' or 'text'")
	}

	// Validate output
	if l.Output != "" && l.Output != "stdout" && l.Output != "stderr" {
		// Check if it's a file path
		dir := filepath.Dir(l.Output)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			v.result.AddWarning(fmt.Sprintf("logging.output directory does not exist: %s", dir))
		}
	}

	// Validate max size
	if l.MaxSize < 0 {
		v.result.AddError("logging.max_size", l.MaxSize, "max_size cannot be negative")
	}

	// Validate max backups
	if l.MaxBackups < 0 {
		v.result.AddError("logging.max_backups", l.MaxBackups, "max_backups cannot be negative")
	}

	// Validate max age
	if l.MaxAge < 0 {
		v.result.AddError("logging.max_age", l.MaxAge, "max_age cannot be negative")
	}
}

// validateAddress validates a network address (host:port)
func validateAddress(addr string) error {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("invalid address format, expected host:port")
	}

	// Validate host
	if host != "localhost" && net.ParseIP(host) == nil {
		// Try as hostname
		if !isValidHostname(host) {
			return errors.New("invalid host")
		}
	}

	// Validate port
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return errors.New("invalid port number")
	}
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	return nil
}

// isValidHostname validates a hostname
func isValidHostname(hostname string) bool {
	// Hostname regex pattern
	pattern := `^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`
	matched, _ := regexp.MatchString(pattern, hostname)
	return matched
}
