// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// TestSanitizeVMName tests VM name sanitization
func TestSanitizeVMName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean name", "MyVM", "MyVM"},
		{"name with spaces", "My VM Name", "My VM Name"},
		{"name with colons", "VM:Test", "VM_Test"},
		{"name with slashes", "folder/vm\\name", "folder_vm_name"},
		{"multiple invalid chars", "VM<>:\"|?*", "VM_______"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeVMName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeVMName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestValidateVMState tests VM power state validation
func TestValidateVMState(t *testing.T) {
	log := logger.NewTestLogger(t)
	validator := NewPreExportValidator(log)

	tests := []struct {
		name        string
		powerState  string
		wantPassed  bool
		wantWarning bool
	}{
		{"powered off", "poweredOff", true, false},
		{"powered on", "poweredOn", true, true},
		{"suspended", "suspended", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := vsphere.VMInfo{
				Name:       "TestVM",
				PowerState: tt.powerState,
			}

			result := validator.validateVMState(vm)

			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", result.Passed, tt.wantPassed)
			}
			if result.Warning != tt.wantWarning {
				t.Errorf("Warning = %v, want %v", result.Warning, tt.wantWarning)
			}
		})
	}
}

// TestCalculateFileChecksum tests checksum calculation
func TestCalculateFileChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content []byte
	}{
		{"small file", []byte("hello world")},
		{"empty file", []byte{}},
		{"binary", []byte{0x00, 0xFF, 0xAB}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(path, tt.content, 0644); err != nil {
				t.Fatal(err)
			}

			checksum, err := CalculateFileChecksum(path)
			if err != nil {
				t.Fatal(err)
			}

			if len(checksum) != 64 {
				t.Errorf("checksum length = %d, want 64", len(checksum))
			}
		})
	}
}

// TestValidateExport tests comprehensive validation
func TestValidateExport(t *testing.T) {
	log := logger.NewTestLogger(t)
	validator := NewPreExportValidator(log)
	ctx := context.Background()
	tmpDir := t.TempDir()

	vm := vsphere.VMInfo{
		Name:       "TestVM",
		PowerState: "poweredOff",
		Storage:    1024 * 1024,
	}

	report := validator.ValidateExport(ctx, vm, tmpDir, vm.Storage)

	if report == nil {
		t.Fatal("got nil report")
	}
	if len(report.Checks) < 4 {
		t.Errorf("got %d checks, want at least 4", len(report.Checks))
	}
}

// TestSaveChecksumManifest tests manifest creation
func TestSaveChecksumManifest(t *testing.T) {
	tmpDir := t.TempDir()

	checksums := map[string]string{
		"file1.vmdk": "abc123",
		"file2.vmdk": "def456",
	}

	if err := SaveChecksumManifest(tmpDir, checksums); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(tmpDir, "checksums.txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("checksums.txt not created")
	}
}
