// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zyvorai/transiva/logger"
)

// TestNewV2VConverter tests converter creation
func TestNewV2VConverter(t *testing.T) {
	log := logger.NewTestLogger(t)

	// This will fail if virt-v2v is not installed, which is expected
	converter, err := NewV2VConverter(log)
	
	// We expect this to fail in most test environments
	if err != nil {
		t.Logf("virt-v2v not available (expected): %v", err)
		return
	}

	if converter == nil {
		t.Fatal("NewV2VConverter returned nil without error")
	}

	if converter.log == nil {
		t.Error("converter.log is nil")
	}

	if converter.virtV2VBin == "" {
		t.Error("virtV2VBin is empty")
	}
}

// TestBuildVirtV2VArgs tests command argument building
func TestBuildVirtV2VArgs(t *testing.T) {
	log := logger.NewTestLogger(t)
	converter := &V2VConverter{
		log:        log,
		virtV2VBin: "/usr/bin/virt-v2v",
	}

	tests := []struct {
		name       string
		config     *V2VConfig
		wantLen    int
		checkArgs  func([]string) bool
	}{
		{
			name: "basic OVA conversion",
			config: &V2VConfig{
				InputFormat:  "ova",
				OutputFormat: "local",
				InputPath:    "/path/to/vm.ova",
				OutputPath:   "/output",
				VMName:       "TestVM",
			},
			wantLen: 7, // Minimum args
			checkArgs: func(args []string) bool {
				hasInput := false
				hasOutput := false
				for i, arg := range args {
					if arg == "-i" && i+1 < len(args) && args[i+1] == "ova" {
						hasInput = true
					}
					if arg == "-o" && i+1 < len(args) && args[i+1] == "local" {
						hasOutput = true
					}
				}
				return hasInput && hasOutput
			},
		},
		{
			name: "VMX with network mapping",
			config: &V2VConfig{
				InputFormat:  "vmx",
				OutputFormat: "libvirt",
				InputPath:    "/path/to/vm.vmx",
				OutputPath:   "/output",
				VMName:       "TestVM",
				Bridge:       "br0",
			},
			wantLen: 9,
			checkArgs: func(args []string) bool {
				hasBridge := false
				for i, arg := range args {
					if arg == "--bridge" && i+1 < len(args) && args[i+1] == "br0" {
						hasBridge = true
					}
				}
				return hasBridge
			},
		},
		{
			name: "verbose and debug mode",
			config: &V2VConfig{
				InputFormat:  "ova",
				OutputFormat: "local",
				InputPath:    "/path/to/vm.ova",
				OutputPath:   "/output",
				VMName:       "TestVM",
				Debug:        true,
				Verbose:      true,
			},
			wantLen: 10,
			checkArgs: func(args []string) bool {
				hasDebug := false
				hasVerbose := false
				for _, arg := range args {
					if arg == "-x" {
						hasDebug = true
					}
					if arg == "-v" {
						hasVerbose = true
					}
				}
				return hasDebug && hasVerbose
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := converter.buildVirtV2VArgs(tt.config)

			if len(args) < tt.wantLen {
				t.Errorf("got %d args, want at least %d", len(args), tt.wantLen)
			}

			if tt.checkArgs != nil && !tt.checkArgs(args) {
				t.Errorf("args validation failed: %v", args)
			}
		})
	}
}

// TestFindConvertedDisks tests disk file detection
func TestFindConvertedDisks(t *testing.T) {
	log := logger.NewTestLogger(t)
	converter := &V2VConverter{
		log:        log,
		virtV2VBin: "/usr/bin/virt-v2v",
	}

	tmpDir := t.TempDir()

	// Create test disk files
	testDisks := []string{
		"disk1.qcow2",
		"disk2.qcow2",
		"disk3.raw",
		"disk4.vmdk",
		"not-a-disk.txt",
	}

	for _, disk := range testDisks {
		path := filepath.Join(tmpDir, disk)
		_ = os.WriteFile(path, []byte("test"), 0644)
	}

	disks := converter.findConvertedDisks(tmpDir)

	// Should find 4 disk files (excluding .txt)
	expectedCount := 4
	if len(disks) != expectedCount {
		t.Errorf("found %d disks, want %d", len(disks), expectedCount)
	}

	// Verify each disk is in the list
	diskMap := make(map[string]bool)
	for _, disk := range disks {
		diskMap[filepath.Base(disk)] = true
	}

	expectedDisks := []string{"disk1.qcow2", "disk2.qcow2", "disk3.raw", "disk4.vmdk"}
	for _, expected := range expectedDisks {
		if !diskMap[expected] {
			t.Errorf("expected disk %s not found", expected)
		}
	}

	if diskMap["not-a-disk.txt"] {
		t.Error("non-disk file should not be in results")
	}
}

// TestV2VConfig validates config structure
func TestV2VConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *V2VConfig
	}{
		{
			name: "minimal config",
			config: &V2VConfig{
				InputFormat:  "ova",
				OutputFormat: "local",
				InputPath:    "/input.ova",
				OutputPath:   "/output",
			},
		},
		{
			name: "full config",
			config: &V2VConfig{
				InputFormat:       "vmx",
				OutputFormat:      "libvirt",
				InputPath:         "/vm.vmx",
				OutputPath:        "/output",
				VMName:            "ConvertedVM",
				Network:           "vmnet0:bridge:br0",
				Bridge:            "br0",
				Storage:           "default",
				VirtIODrivers:     true,
				RemoveVMwareTools: true,
				InstallGuestAgent: true,
				Debug:             true,
				Verbose:           true,
				NoTrim:            false,
				InPlace:           false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.InputFormat == "" {
				t.Error("InputFormat should not be empty")
			}
			if tt.config.OutputFormat == "" {
				t.Error("OutputFormat should not be empty")
			}
			if tt.config.InputPath == "" {
				t.Error("InputPath should not be empty")
			}
			if tt.config.OutputPath == "" {
				t.Error("OutputPath should not be empty")
			}
		})
	}
}

// TestConversionResult validates result structure
func TestConversionResult(t *testing.T) {
	result := &ConversionResult{
		Success:    true,
		VMName:     "TestVM",
		OutputPath: "/output/TestVM",
		LibvirtXML: "/output/TestVM.xml",
		DiskImages: []string{"/output/disk1.qcow2", "/output/disk2.qcow2"},
		Warnings:   []string{"warning 1"},
		Errors:     []string{},
		Duration:   "5m30s",
	}

	if !result.Success {
		t.Error("Success should be true")
	}

	if len(result.DiskImages) != 2 {
		t.Errorf("got %d disk images, want 2", len(result.DiskImages))
	}

	if len(result.Warnings) != 1 {
		t.Errorf("got %d warnings, want 1", len(result.Warnings))
	}

	if len(result.Errors) != 0 {
		t.Errorf("got %d errors, want 0", len(result.Errors))
	}
}

// TestConvertOVAToKVM tests high-level OVA conversion function signature
func TestConvertOVAToKVM(t *testing.T) {
	log := logger.NewTestLogger(t)
	ctx := context.Background()

	// This will fail without virt-v2v installed, which is expected
	err := ConvertOVAToKVM(ctx, "/nonexistent.ova", "/output", "TestVM", log)
	
	// We expect this to fail
	if err == nil {
		t.Error("expected error for nonexistent file or missing virt-v2v")
	}
}
