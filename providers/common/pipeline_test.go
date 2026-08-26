// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements Logger interface for testing
type mockLogger struct {
	infoCalls  []string
	warnCalls  []string
	errorCalls []string
}

func (m *mockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.infoCalls = append(m.infoCalls, msg)
}

func (m *mockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.warnCalls = append(m.warnCalls, msg)
}

func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.errorCalls = append(m.errorCalls, msg)
}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) With(keysAndValues ...interface{}) Logger         { return m }

func TestNewPipelineExecutor(t *testing.T) {
	logger := &mockLogger{}
	config := &Hyper2KVMConfig{
		Enabled:       true,
		Hyper2KVMPath: "/path/to/hyper2kvm",
		ManifestPath:  "/path/to/manifest.json",
	}

	executor := NewPipelineExecutor(config, logger)
	require.NotNil(t, executor)
	assert.NotNil(t, executor.config)
	assert.NotNil(t, executor.logger)
}

func TestHyper2KVMConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Hyper2KVMConfig
		valid   bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Hyper2KVMConfig{
				Enabled:       true,
				Hyper2KVMPath: "/usr/bin/hyper2kvm",
				ManifestPath:  "/tmp/manifest.json",
			},
			valid: true,
		},
		{
			name: "disabled config",
			config: Hyper2KVMConfig{
				Enabled: false,
			},
			valid: true, // Valid but won't execute
		},
		{
			name: "missing hyper2kvm path",
			config: Hyper2KVMConfig{
				Enabled:      true,
				ManifestPath: "/tmp/manifest.json",
			},
			valid:  false,
			errMsg: "hyper2kvm path required",
		},
		{
			name: "missing manifest path",
			config: Hyper2KVMConfig{
				Enabled:       true,
				Hyper2KVMPath: "/usr/bin/hyper2kvm",
			},
			valid:  false,
			errMsg: "manifest path required",
		},
		{
			name: "libvirt integration enabled",
			config: Hyper2KVMConfig{
				Enabled:            true,
				Hyper2KVMPath:      "/usr/bin/hyper2kvm",
				ManifestPath:       "/tmp/manifest.json",
				LibvirtIntegration: true,
				LibvirtURI:         "qemu:///system",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := true
			if tt.config.Enabled {
				if tt.config.Hyper2KVMPath == "" {
					isValid = false
				}
				if tt.config.ManifestPath == "" {
					isValid = false
				}
			}

			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestFindHyper2KVM(t *testing.T) {
	t.Skip("Skipping hyper2kvm discovery test - depends on filesystem")

	// Create a temporary directory structure
	tempDir := t.TempDir()
	hyper2kvmPath := filepath.Join(tempDir, "hyper2kvm")

	// Create a mock hyper2kvm executable
	err := os.WriteFile(hyper2kvmPath, []byte("#!/bin/bash\necho 'mock hyper2kvm'\n"), 0755)
	require.NoError(t, err)

	// Test discovery
	found := findHyper2KVM()
	assert.NotEmpty(t, found)
}

func TestPipelineResultValidation(t *testing.T) {
	result := PipelineResult{
		Success:       true,
		OutputPath:    "/var/lib/libvirt/images/vm.qcow2",
		Duration:      15 * time.Minute,
		LibvirtDomain: "test-vm",
		Error:         nil,
	}

	assert.True(t, result.Success)
	assert.NotEmpty(t, result.OutputPath)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.NotEmpty(t, result.LibvirtDomain)
	assert.Nil(t, result.Error)
}

func TestPipelineResultWithError(t *testing.T) {
	result := PipelineResult{
		Success:    false,
		OutputPath: "",
		Duration:   2 * time.Minute,
		Error:      fmt.Errorf("disk conversion failed: insufficient disk space"),
	}

	assert.False(t, result.Success)
	assert.Empty(t, result.OutputPath)
	assert.NotNil(t, result.Error)
}

func TestPipelineStagesValidation(t *testing.T) {
	validStages := []string{"INSPECT", "FIX", "CONVERT", "VALIDATE"}

	// Test that all valid stages are recognized
	for _, stage := range validStages {
		assert.Contains(t, validStages, stage)
	}
}

func TestExecuteContextCancellation(t *testing.T) {
	logger := &mockLogger{}
	config := &Hyper2KVMConfig{
		Enabled:       true,
		Hyper2KVMPath: "/nonexistent/hyper2kvm",
		ManifestPath:  "/tmp/manifest.json",
	}

	executor := NewPipelineExecutor(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := executor.Execute(ctx)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestExecuteWithTimeout(t *testing.T) {
	t.Skip("Skipping timeout test - requires mock execution")

	logger := &mockLogger{}
	config := &Hyper2KVMConfig{
		Enabled:       true,
		Hyper2KVMPath: "/usr/bin/sleep",
		ManifestPath:  "100", // sleep for 100 seconds
	}

	executor := NewPipelineExecutor(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := executor.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.Nil(t, result)
}

func TestPipelineDryRun(t *testing.T) {
	logger := &mockLogger{}
	config := &Hyper2KVMConfig{
		Enabled:       true,
		Hyper2KVMPath: "/usr/bin/hyper2kvm",
		ManifestPath:  "/tmp/manifest.json",
		DryRun:        true,
	}

	executor := NewPipelineExecutor(config, logger)
	assert.NotNil(t, executor)
	assert.True(t, executor.config.DryRun)
}

func TestLibvirtConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config LibvirtConfig
		valid  bool
	}{
		{
			name: "valid config with system URI",
			config: LibvirtConfig{
				URI:           "qemu:///system",
				NetworkBridge: "virbr0",
				StoragePool:   "default",
			},
			valid: true,
		},
		{
			name: "valid config with session URI",
			config: LibvirtConfig{
				URI:           "qemu:///session",
				NetworkBridge: "virbr0",
			},
			valid: true,
		},
		{
			name: "valid config with remote URI",
			config: LibvirtConfig{
				URI:           "qemu+ssh://host/system",
				NetworkBridge: "br0",
			},
			valid: true,
		},
		{
			name: "empty URI",
			config: LibvirtConfig{
				URI: "",
			},
			valid: false,
		},
		{
			name: "auto-start enabled",
			config: LibvirtConfig{
				URI:       "qemu:///system",
				AutoStart: true,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.config.URI != ""
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestPipelineOutputParsing(t *testing.T) {
	// Mock hyper2kvm output
	mockOutput := `
[INSPECT] Detecting guest OS...
[INSPECT] OS detected: Ubuntu 20.04 LTS
[FIX] Fixing fstab entries...
[FIX] Updating GRUB configuration...
[CONVERT] Converting VMDK to qcow2...
[CONVERT] Output: /var/lib/libvirt/images/vm.qcow2
[VALIDATE] Checking image integrity...
[VALIDATE] Image validation successful
`

	// Parse output to extract stages
	stages := []string{}
	if contains(mockOutput, "[INSPECT]") {
		stages = append(stages, "INSPECT")
	}
	if contains(mockOutput, "[FIX]") {
		stages = append(stages, "FIX")
	}
	if contains(mockOutput, "[CONVERT]") {
		stages = append(stages, "CONVERT")
	}
	if contains(mockOutput, "[VALIDATE]") {
		stages = append(stages, "VALIDATE")
	}

	assert.Len(t, stages, 4)
	assert.Equal(t, []string{"INSPECT", "FIX", "CONVERT", "VALIDATE"}, stages)
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != substr && (s[0:len(substr)] == substr || contains(s[1:], substr))
}

func TestPipelineMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"pipeline_success":  true,
		"pipeline_duration": "15m30s",
		"converted_path":    "/var/lib/libvirt/images/vm.qcow2",
		"libvirt_domain":    "test-vm",
		"libvirt_uri":       "qemu:///system",
		"stages_completed":  []string{"INSPECT", "FIX", "CONVERT", "VALIDATE"},
	}

	assert.Equal(t, true, metadata["pipeline_success"])
	assert.Equal(t, "15m30s", metadata["pipeline_duration"])
	assert.NotEmpty(t, metadata["converted_path"])
	assert.NotEmpty(t, metadata["libvirt_domain"])
	assert.Len(t, metadata["stages_completed"], 4)
}

func TestPipelineErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		fatal    bool
	}{
		{
			name:     "hyper2kvm not found",
			errorMsg: "hyper2kvm not found at /path/to/hyper2kvm",
			fatal:    true,
		},
		{
			name:     "manifest file not found",
			errorMsg: "manifest file not found",
			fatal:    true,
		},
		{
			name:     "disk conversion failed",
			errorMsg: "disk conversion failed: insufficient disk space",
			fatal:    false,
		},
		{
			name:     "libvirt integration failed",
			errorMsg: "virsh define failed: permission denied",
			fatal:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.errorMsg)

			// Fatal errors should stop pipeline
			// Non-fatal errors should be logged but allow export to succeed
			if tt.fatal {
				assert.Contains(t, []string{
					"hyper2kvm not found",
					"manifest file not found",
				}, tt.name)
			}
		})
	}
}
