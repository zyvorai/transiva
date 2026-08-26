// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"testing"
	"time"
)

// setEnvOrFatal sets an environment variable, failing the test if it errors.
func setEnvOrFatal(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
}

// unsetEnvOrFatal unsets an environment variable, failing the test if it errors.
func unsetEnvOrFatal(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
}

// removeOrFatal removes a file, failing the test if it errors.
func removeOrFatal(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("failed to remove %s: %v", path, err)
	}
}

// writeStringOrFatal writes a string to a file, failing the test if it errors.
func writeStringOrFatal(t *testing.T, f *os.File, s string) {
	t.Helper()
	if _, err := f.WriteString(s); err != nil {
		t.Fatalf("failed to write to %s: %v", f.Name(), err)
	}
}

// closeOrFatal closes a file, failing the test if it errors.
func closeOrFatal(t *testing.T, f *os.File) {
	t.Helper()
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close %s: %v", f.Name(), err)
	}
}

func TestFromEnvironment(t *testing.T) {
	// Set test environment variables
	setEnvOrFatal(t, "GOVC_URL", "https://test.vcenter.com/sdk")
	setEnvOrFatal(t, "GOVC_USERNAME", "testuser")
	setEnvOrFatal(t, "GOVC_PASSWORD", "testpass")
	setEnvOrFatal(t, "GOVC_INSECURE", "1")
	setEnvOrFatal(t, "DAEMON_ADDR", "localhost:9090")
	setEnvOrFatal(t, "LOG_LEVEL", "debug")
	defer func() {
		unsetEnvOrFatal(t, "GOVC_URL")
		unsetEnvOrFatal(t, "GOVC_USERNAME")
		unsetEnvOrFatal(t, "GOVC_PASSWORD")
		unsetEnvOrFatal(t, "GOVC_INSECURE")
		unsetEnvOrFatal(t, "DAEMON_ADDR")
		unsetEnvOrFatal(t, "LOG_LEVEL")
	}()

	cfg := FromEnvironment()

	if cfg.VCenterURL != "https://test.vcenter.com/sdk" {
		t.Errorf("Expected VCenterURL 'https://test.vcenter.com/sdk', got '%s'", cfg.VCenterURL)
	}
	if cfg.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got '%s'", cfg.Username)
	}
	if cfg.Password != "testpass" {
		t.Errorf("Expected Password 'testpass', got '%s'", cfg.Password)
	}
	if !cfg.Insecure {
		t.Error("Expected Insecure to be true")
	}
	if cfg.DaemonAddr != "localhost:9090" {
		t.Errorf("Expected DaemonAddr 'localhost:9090', got '%s'", cfg.DaemonAddr)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got '%s'", cfg.LogLevel)
	}
}

func TestFromEnvironmentDefaults(t *testing.T) {
	// Clear all env vars
	os.Clearenv()

	cfg := FromEnvironment()

	if cfg.DownloadWorkers != 3 {
		t.Errorf("Expected default DownloadWorkers 3, got %d", cfg.DownloadWorkers)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("Expected default RetryAttempts 3, got %d", cfg.RetryAttempts)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if cfg.DaemonAddr != "localhost:8080" {
		t.Errorf("Expected default DaemonAddr 'localhost:8080', got '%s'", cfg.DaemonAddr)
	}
}

func TestFromFile(t *testing.T) {
	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer removeOrFatal(t, tmpFile.Name())

	configContent := `vcenterurl: "https://file.vcenter.com/sdk"
username: "fileuser"
password: "filepass"
insecure: true
daemonaddr: "0.0.0.0:8888"
loglevel: "warn"
downloadworkers: 8
`
	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	closeOrFatal(t, tmpFile)

	cfg, err := FromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	if cfg.VCenterURL != "https://file.vcenter.com/sdk" {
		t.Errorf("Expected VCenterURL from file, got '%s'", cfg.VCenterURL)
	}
	if cfg.Username != "fileuser" {
		t.Errorf("Expected Username 'fileuser', got '%s'", cfg.Username)
	}
	if cfg.DaemonAddr != "0.0.0.0:8888" {
		t.Errorf("Expected DaemonAddr '0.0.0.0:8888', got '%s'", cfg.DaemonAddr)
	}
	if cfg.DownloadWorkers != 8 {
		t.Errorf("Expected DownloadWorkers 8, got %d", cfg.DownloadWorkers)
	}
}

func TestMergeWithEnv(t *testing.T) {
	// Set environment variables
	setEnvOrFatal(t, "GOVC_URL", "https://env.vcenter.com/sdk")
	setEnvOrFatal(t, "LOG_LEVEL", "error")
	defer func() {
		unsetEnvOrFatal(t, "GOVC_URL")
		unsetEnvOrFatal(t, "LOG_LEVEL")
	}()

	// Create base config
	cfg := &Config{
		VCenterURL: "https://file.vcenter.com/sdk",
		LogLevel:   "info",
		DaemonAddr: "localhost:8080",
	}

	// Merge with env (env should take precedence)
	merged := cfg.MergeWithEnv()

	if merged.VCenterURL != "https://env.vcenter.com/sdk" {
		t.Errorf("Expected env to override VCenterURL, got '%s'", merged.VCenterURL)
	}
	if merged.LogLevel != "error" {
		t.Errorf("Expected env to override LogLevel, got '%s'", merged.LogLevel)
	}
	if merged.DaemonAddr != "localhost:8080" {
		t.Errorf("Expected DaemonAddr to remain from file, got '%s'", merged.DaemonAddr)
	}
}

func TestFromFile_NonexistentFile(t *testing.T) {
	_, err := FromFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFromFile_InvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "invalid-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer removeOrFatal(t, tmpFile.Name())

	// Write invalid YAML
	writeStringOrFatal(t, tmpFile, "invalid: yaml: content: :\n")
	closeOrFatal(t, tmpFile)

	_, err = FromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestFromFile_WithDefaults(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "minimal-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer removeOrFatal(t, tmpFile.Name())

	// Minimal config - should get defaults
	configContent := `vcenterurl: "https://minimal.vcenter.com/sdk"
`
	writeStringOrFatal(t, tmpFile, configContent)
	closeOrFatal(t, tmpFile)

	cfg, err := FromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	// Verify defaults are applied
	if cfg.DownloadWorkers != 3 {
		t.Errorf("Expected default DownloadWorkers 3, got %d", cfg.DownloadWorkers)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("Expected default RetryAttempts 3, got %d", cfg.RetryAttempts)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if cfg.DaemonAddr != "localhost:8080" {
		t.Errorf("Expected default DaemonAddr 'localhost:8080', got '%s'", cfg.DaemonAddr)
	}
	if cfg.ProgressStyle != "bar" {
		t.Errorf("Expected default ProgressStyle 'bar', got '%s'", cfg.ProgressStyle)
	}
}

func TestFromFile_WithConnectionPool(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "pool-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer removeOrFatal(t, tmpFile.Name())

	configContent := `vcenterurl: "https://pool.vcenter.com/sdk"
connectionpool:
  enabled: true
  max_connections: 10
`
	writeStringOrFatal(t, tmpFile, configContent)
	closeOrFatal(t, tmpFile)

	cfg, err := FromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	// Verify connection pool config
	if cfg.ConnectionPool == nil {
		t.Fatal("Expected ConnectionPool to be set")
	}
	if !cfg.ConnectionPool.Enabled {
		t.Error("Expected ConnectionPool.Enabled to be true")
	}
	if cfg.ConnectionPool.MaxConnections != 10 {
		t.Errorf("Expected MaxConnections 10, got %d", cfg.ConnectionPool.MaxConnections)
	}
	// Verify defaults are applied for missing fields
	if cfg.ConnectionPool.IdleTimeout == 0 {
		t.Error("Expected default IdleTimeout to be set")
	}
	if cfg.ConnectionPool.HealthCheckInterval == 0 {
		t.Error("Expected default HealthCheckInterval to be set")
	}
}

func TestFromFile_AllDefaults(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer removeOrFatal(t, tmpFile.Name())

	// Empty config - should get all defaults
	configContent := `{}`
	writeStringOrFatal(t, tmpFile, configContent)
	closeOrFatal(t, tmpFile)

	cfg, err := FromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	// Verify all defaults
	if cfg.Timeout != 3600*time.Second {
		t.Errorf("Expected default Timeout 3600s, got %v", cfg.Timeout)
	}
	if cfg.DownloadWorkers != 3 {
		t.Errorf("Expected default DownloadWorkers 3, got %d", cfg.DownloadWorkers)
	}
	if cfg.ChunkSize != 33554432 {
		t.Errorf("Expected default ChunkSize 33554432, got %d", cfg.ChunkSize)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("Expected default RetryAttempts 3, got %d", cfg.RetryAttempts)
	}
	if cfg.RetryDelay != 5*time.Second {
		t.Errorf("Expected default RetryDelay 5s, got %v", cfg.RetryDelay)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel 'info', got '%s'", cfg.LogLevel)
	}
	if cfg.ProgressStyle != "bar" {
		t.Errorf("Expected default ProgressStyle 'bar', got '%s'", cfg.ProgressStyle)
	}
	if cfg.RefreshRate != 100*time.Millisecond {
		t.Errorf("Expected default RefreshRate 100ms, got %v", cfg.RefreshRate)
	}
	if cfg.DaemonAddr != "localhost:8080" {
		t.Errorf("Expected default DaemonAddr 'localhost:8080', got '%s'", cfg.DaemonAddr)
	}

	// Verify ConnectionPool defaults
	if cfg.ConnectionPool == nil {
		t.Fatal("Expected ConnectionPool to be initialized")
	}
	if cfg.ConnectionPool.Enabled {
		t.Error("Expected ConnectionPool.Enabled to be false by default")
	}
	if cfg.ConnectionPool.MaxConnections != 5 {
		t.Errorf("Expected default MaxConnections 5, got %d", cfg.ConnectionPool.MaxConnections)
	}
	if cfg.ConnectionPool.IdleTimeout != 5*time.Minute {
		t.Errorf("Expected default IdleTimeout 5m, got %v", cfg.ConnectionPool.IdleTimeout)
	}
	if cfg.ConnectionPool.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected default HealthCheckInterval 30s, got %v", cfg.ConnectionPool.HealthCheckInterval)
	}
}

func TestFromFile_PartialConnectionPool(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "partial-pool-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer removeOrFatal(t, tmpFile.Name())

	// Partial connection pool config - should get defaults for missing fields
	configContent := `vcenterurl: "https://test.vcenter.com/sdk"
connectionpool:
  enabled: true
`
	writeStringOrFatal(t, tmpFile, configContent)
	closeOrFatal(t, tmpFile)

	cfg, err := FromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	// Verify ConnectionPool was parsed
	if cfg.ConnectionPool == nil {
		t.Fatal("Expected ConnectionPool to be set")
	}

	// Verify defaults were applied for missing fields
	if cfg.ConnectionPool.MaxConnections != 5 {
		t.Errorf("Expected default MaxConnections 5, got %d", cfg.ConnectionPool.MaxConnections)
	}
	if cfg.ConnectionPool.IdleTimeout != 5*time.Minute {
		t.Errorf("Expected default IdleTimeout 5m, got %v", cfg.ConnectionPool.IdleTimeout)
	}
	if cfg.ConnectionPool.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected default HealthCheckInterval 30s, got %v", cfg.ConnectionPool.HealthCheckInterval)
	}
}

func TestFromFile_PartialProviderConfigs(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "partial-providers-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer removeOrFatal(t, tmpFile.Name())

	// Partial provider configs - should get defaults for missing fields
	configContent := `vcenterurl: "https://test.vcenter.com/sdk"
aws:
  enabled: true
azure:
  enabled: true
gcp:
  enabled: true
hyperv:
  enabled: true
  use_https: true
oci:
  enabled: true
openstack:
  enabled: true
alibabacloud:
  enabled: true
proxmox:
  enabled: true
`
	writeStringOrFatal(t, tmpFile, configContent)
	closeOrFatal(t, tmpFile)

	cfg, err := FromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}

	// Verify AWS defaults
	if cfg.AWS.Region != "us-east-1" {
		t.Errorf("Expected default AWS Region, got %s", cfg.AWS.Region)
	}
	if cfg.AWS.ExportFormat != "vmdk" {
		t.Errorf("Expected default AWS ExportFormat, got %s", cfg.AWS.ExportFormat)
	}

	// Verify Azure defaults
	if cfg.Azure.Location != "eastus" {
		t.Errorf("Expected default Azure Location, got %s", cfg.Azure.Location)
	}
	if cfg.Azure.ExportFormat != "image" {
		t.Errorf("Expected default Azure ExportFormat, got %s", cfg.Azure.ExportFormat)
	}

	// Verify GCP defaults
	if cfg.GCP.Zone != "us-central1-a" {
		t.Errorf("Expected default GCP Zone, got %s", cfg.GCP.Zone)
	}
	if cfg.GCP.Region != "us-central1" {
		t.Errorf("Expected default GCP Region, got %s", cfg.GCP.Region)
	}
	if cfg.GCP.ExportFormat != "vmdk" {
		t.Errorf("Expected default GCP ExportFormat, got %s", cfg.GCP.ExportFormat)
	}

	// Verify Hyper-V defaults (should use HTTPS port since use_https is true)
	if cfg.HyperV.WinRMPort != 5986 {
		t.Errorf("Expected Hyper-V WinRMPort 5986 for HTTPS, got %d", cfg.HyperV.WinRMPort)
	}
	if cfg.HyperV.ExportFormat != "vhdx" {
		t.Errorf("Expected default Hyper-V ExportFormat, got %s", cfg.HyperV.ExportFormat)
	}

	// Verify OCI defaults
	if cfg.OCI.Region != "us-phoenix-1" {
		t.Errorf("Expected default OCI Region, got %s", cfg.OCI.Region)
	}

	// Verify OpenStack defaults
	if cfg.OpenStack.DomainName != "Default" {
		t.Errorf("Expected default OpenStack DomainName, got %s", cfg.OpenStack.DomainName)
	}

	// Verify Alibaba Cloud defaults
	if cfg.AlibabaCloud.RegionID != "cn-hangzhou" {
		t.Errorf("Expected default Alibaba Cloud RegionID, got %s", cfg.AlibabaCloud.RegionID)
	}

	// Verify Proxmox defaults
	if cfg.Proxmox.Port != 8006 {
		t.Errorf("Expected default Proxmox Port, got %d", cfg.Proxmox.Port)
	}
}

func TestMergeWithEnv_AllFields(t *testing.T) {
	// Set comprehensive environment variables
	setEnvOrFatal(t, "GOVC_URL", "https://env.vcenter.com/sdk")
	setEnvOrFatal(t, "GOVC_USERNAME", "envuser")
	setEnvOrFatal(t, "GOVC_PASSWORD", "envpass")
	setEnvOrFatal(t, "GOVC_INSECURE", "1")
	setEnvOrFatal(t, "DOWNLOAD_WORKERS", "10")
	setEnvOrFatal(t, "LOG_LEVEL", "debug")
	setEnvOrFatal(t, "DAEMON_ADDR", "0.0.0.0:9999")
	defer func() {
		unsetEnvOrFatal(t, "GOVC_URL")
		unsetEnvOrFatal(t, "GOVC_USERNAME")
		unsetEnvOrFatal(t, "GOVC_PASSWORD")
		unsetEnvOrFatal(t, "GOVC_INSECURE")
		unsetEnvOrFatal(t, "DOWNLOAD_WORKERS")
		unsetEnvOrFatal(t, "LOG_LEVEL")
		unsetEnvOrFatal(t, "DAEMON_ADDR")
	}()

	// Create base config with different values
	cfg := &Config{
		VCenterURL:      "https://file.vcenter.com/sdk",
		Username:        "fileuser",
		Password:        "filepass",
		Insecure:        false,
		DownloadWorkers: 3,
		RetryAttempts:   3,
		LogLevel:        "info",
		DaemonAddr:      "localhost:8080",
	}

	merged := cfg.MergeWithEnv()

	// Verify env values override config values where supported
	if merged.VCenterURL != "https://env.vcenter.com/sdk" {
		t.Errorf("Expected env VCenterURL, got '%s'", merged.VCenterURL)
	}
	if merged.Username != "envuser" {
		t.Errorf("Expected env Username, got '%s'", merged.Username)
	}
	if merged.Password != "envpass" {
		t.Errorf("Expected env Password, got '%s'", merged.Password)
	}
	if !merged.Insecure {
		t.Error("Expected env Insecure to be true")
	}
	if merged.DownloadWorkers != 10 {
		t.Errorf("Expected env DownloadWorkers 10, got %d", merged.DownloadWorkers)
	}
	// RetryAttempts is not merged from env, should remain from config
	if merged.RetryAttempts != 3 {
		t.Errorf("Expected config RetryAttempts 3, got %d", merged.RetryAttempts)
	}
	if merged.LogLevel != "debug" {
		t.Errorf("Expected env LogLevel, got '%s'", merged.LogLevel)
	}
	if merged.DaemonAddr != "0.0.0.0:9999" {
		t.Errorf("Expected env DaemonAddr, got '%s'", merged.DaemonAddr)
	}
}
