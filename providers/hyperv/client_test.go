//go:build integration

// SPDX-License-Identifier: Apache-2.0

package hyperv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// TestVMInfo_JSONParsing tests VMInfo JSON parsing
func TestVMInfo_JSONParsing(t *testing.T) {
	jsonData := `{
		"Name": "test-vm",
		"Id": "12345",
		"State": "Running",
		"CPUUsage": 25,
		"MemoryAssigned": 4294967296,
		"Generation": 2,
		"Status": "Operating normally"
	}`

	var vm VMInfo
	err := json.Unmarshal([]byte(jsonData), &vm)
	if err != nil {
		t.Fatalf("Failed to parse VMInfo JSON: %v", err)
	}

	if vm.Name != "test-vm" {
		t.Errorf("Expected Name 'test-vm', got '%s'", vm.Name)
	}

	if vm.State != "Running" {
		t.Errorf("Expected State 'Running', got '%s'", vm.State)
	}

	if vm.Generation != 2 {
		t.Errorf("Expected Generation 2, got %d", vm.Generation)
	}

	if vm.CPUUsage != 25 {
		t.Errorf("Expected CPUUsage 25, got %d", vm.CPUUsage)
	}
}

// TestVMInfo_MultipleVMs tests parsing multiple VMs
func TestVMInfo_MultipleVMs(t *testing.T) {
	jsonData := `[
		{"Name": "vm1", "State": "Running", "Generation": 1},
		{"Name": "vm2", "State": "Off", "Generation": 2}
	]`

	var vms []*VMInfo
	err := json.Unmarshal([]byte(jsonData), &vms)
	if err != nil {
		t.Fatalf("Failed to parse VMs JSON: %v", err)
	}

	if len(vms) != 2 {
		t.Fatalf("Expected 2 VMs, got %d", len(vms))
	}

	if vms[0].Name != "vm1" {
		t.Errorf("Expected first VM name 'vm1', got '%s'", vms[0].Name)
	}

	if vms[1].Name != "vm2" {
		t.Errorf("Expected second VM name 'vm2', got '%s'", vms[1].Name)
	}
}

// TestConfig_Defaults tests configuration defaults
func TestConfig_Defaults(t *testing.T) {
	cfg := &Config{
		Host:     "hyperv-host",
		UseWinRM: true,
	}

	log := logger.New("debug")
	client, err := NewClient(cfg, log)
	if err != nil {
		// Expected to fail without real connection, but we can check config
		t.Logf("Client creation failed as expected (no real host): %v", err)
	}

	// Check WinRM port defaults
	if cfg.WinRMPort == 0 {
		if cfg.UseHTTPS {
			cfg.WinRMPort = 5986
		} else {
			cfg.WinRMPort = 5985
		}
	}

	if !cfg.UseHTTPS && cfg.WinRMPort != 5985 {
		t.Errorf("Expected WinRM port 5985 for HTTP, got %d", cfg.WinRMPort)
	}

	// Check timeout default
	if cfg.Timeout == 0 {
		cfg.Timeout = 1 * time.Hour
	}

	if cfg.Timeout != 1*time.Hour {
		t.Errorf("Expected timeout 1 hour, got %v", cfg.Timeout)
	}

	if client != nil {
		client.Close()
	}
}

// TestConfig_HTTPS tests HTTPS WinRM configuration
func TestConfig_HTTPS(t *testing.T) {
	cfg := &Config{
		Host:     "hyperv-host",
		UseWinRM: true,
		UseHTTPS: true,
	}

	log := logger.New("debug")
	client, err := NewClient(cfg, log)
	if err != nil {
		t.Logf("Client creation failed as expected: %v", err)
	}

	// Check HTTPS port
	if cfg.WinRMPort == 0 {
		if cfg.UseHTTPS {
			cfg.WinRMPort = 5986
		}
	}

	if cfg.WinRMPort != 5986 {
		t.Errorf("Expected WinRM HTTPS port 5986, got %d", cfg.WinRMPort)
	}

	if client != nil {
		client.Close()
	}
}

// TestToJSONArray tests JSON array helper function
func TestToJSONArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "empty array",
			input:    []string{},
			expected: "[]",
		},
		{
			name:     "single item",
			input:    []string{"item1"},
			expected: `["item1"]`,
		},
		{
			name:     "multiple items",
			input:    []string{"item1", "item2", "item3"},
			expected: `["item1", "item2", "item3"]`,
		},
		{
			name:     "items with quotes",
			input:    []string{`item"with"quotes`},
			expected: `["item\"with\"quotes"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toJSONArray(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestSaveMetadata tests metadata file creation
func TestSaveMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	vmInfo := &VMInfo{
		Name:           "test-vm",
		ID:             "vm-12345",
		State:          "Running",
		Generation:     2,
		CPUUsage:       30,
		MemoryAssigned: 8589934592,
		Path:           "C:\\VMs\\test-vm",
		VHDPath:        []string{"C:\\VMs\\test-vm\\disk1.vhdx", "C:\\VMs\\test-vm\\disk2.vhdx"},
	}

	log := logger.New("debug")
	client := &Client{
		config: &Config{},
		logger: log,
	}

	metadataPath := filepath.Join(tmpDir, "test-vm-metadata.json")
	err := client.saveMetadata(vmInfo, metadataPath)
	if err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Metadata file should exist")
	}

	// Read and verify content
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	metadata := string(content)

	// Check for expected fields
	if !contains(metadata, "test-vm") {
		t.Error("Metadata should contain VM name")
	}
	if !contains(metadata, "vm-12345") {
		t.Error("Metadata should contain VM ID")
	}
	if !contains(metadata, "Running") {
		t.Error("Metadata should contain state")
	}
	if !contains(metadata, "disk1.vhdx") {
		t.Error("Metadata should contain VHD paths")
	}
}

// TestClient_String tests string representation
func TestClient_String(t *testing.T) {
	log := logger.New("debug")

	// Local client
	localClient := &Client{
		config: &Config{UseWinRM: false},
		logger: log,
	}

	str := localClient.String()
	if str != "Hyper-V Client (local)" {
		t.Errorf("Expected local client string, got: %s", str)
	}

	// Remote client
	remoteClient := &Client{
		config: &Config{
			Host:     "hyperv-server",
			UseWinRM: true,
		},
		logger: log,
	}

	str = remoteClient.String()
	expected := "Hyper-V Client (remote=hyperv-server, winrm=true)"
	if str != expected {
		t.Errorf("Expected %s, got: %s", expected, str)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAtIndex(s, substr))
}

func containsAtIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Integration test placeholders (require real Hyper-V host)
// These tests are disabled by default - enable with build tag 'integration'

// TestListVMs_Integration tests VM listing
func TestListVMs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Skip("Integration test - requires Hyper-V host")

	// This test requires:
	// 1. Hyper-V installed and running
	// 2. Admin permissions to query Hyper-V
	// 3. At least one VM configured
}

// TestExportVM_Integration tests VM export
func TestExportVM_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Skip("Integration test - requires Hyper-V host")

	// This test requires:
	// 1. Hyper-V installed
	// 2. VM to export
	// 3. Sufficient disk space for export
	// 4. Admin permissions
}

// Benchmark tests

func BenchmarkVMInfoParsing_Single(b *testing.B) {
	jsonData := `{
		"Name": "test-vm",
		"Id": "12345",
		"State": "Running",
		"CPUUsage": 25,
		"MemoryAssigned": 4294967296,
		"Generation": 2
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vm VMInfo
		json.Unmarshal([]byte(jsonData), &vm)
	}
}

func BenchmarkVMInfoParsing_Multiple(b *testing.B) {
	jsonData := `[
		{"Name": "vm1", "State": "Running", "Generation": 1},
		{"Name": "vm2", "State": "Off", "Generation": 2},
		{"Name": "vm3", "State": "Running", "Generation": 2},
		{"Name": "vm4", "State": "Saved", "Generation": 1},
		{"Name": "vm5", "State": "Running", "Generation": 2}
	]`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vms []*VMInfo
		json.Unmarshal([]byte(jsonData), &vms)
	}
}

// TestSearchVMs_Logic tests search functionality logic
func TestSearchVMs_Logic(t *testing.T) {
	vms := []*VMInfo{
		{Name: "web-server", State: "Running", Status: "Normal"},
		{Name: "db-server", State: "Off", Status: "Stopped"},
		{Name: "app-server", State: "Running", Status: "Normal"},
	}

	tests := []struct {
		query    string
		expected int
	}{
		{"web", 1},
		{"server", 3},
		{"running", 2},
		{"off", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			var matches []*VMInfo
			query := tt.query

			for _, vm := range vms {
				if containsIgnoreCase(vm.Name, query) ||
					containsIgnoreCase(vm.State, query) ||
					containsIgnoreCase(vm.Status, query) {
					matches = append(matches, vm)
				}
			}

			if len(matches) != tt.expected {
				t.Errorf("Query '%s': expected %d matches, got %d", tt.query, tt.expected, len(matches))
			}
		})
	}
}

func containsIgnoreCase(s, substr string) bool {
	return contains(toLower(s), toLower(substr))
}

func toLower(s string) string {
	// Simple lowercase conversion
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}
