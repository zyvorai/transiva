// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name   string
		path   string
		exists bool
	}{
		{
			name:   "existing file",
			path:   testFile,
			exists: true,
		},
		{
			name:   "non-existent file",
			path:   filepath.Join(tmpDir, "nonexistent.txt"),
			exists: false,
		},
		{
			name:   "existing directory",
			path:   tmpDir,
			exists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileExists(tt.path)
			if result != tt.exists {
				t.Errorf("fileExists(%s) = %v, want %v", tt.path, result, tt.exists)
			}
		})
	}
}

func TestGetAvailableDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()

	space, err := getAvailableDiskSpace(tmpDir)
	if err != nil {
		t.Fatalf("getAvailableDiskSpace() error = %v", err)
	}

	if space <= 0 {
		t.Errorf("Expected positive disk space, got %d", space)
	}
}

func TestGetAvailableDiskSpace_InvalidPath(t *testing.T) {
	_, err := getAvailableDiskSpace("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent path, got nil")
	}
}

func TestCheckDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		requiredBytes int64
		expectError   bool
	}{
		{
			name:          "sufficient space (1KB)",
			requiredBytes: 1024,
			expectError:   false,
		},
		{
			name:          "insufficient space (1PB)",
			requiredBytes: 1024 * 1024 * 1024 * 1024 * 1024, // 1 PB
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDiskSpace(tmpDir, tt.requiredBytes)
			if (err != nil) != tt.expectError {
				t.Errorf("checkDiskSpace() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestCalculateDirectorySize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("12345"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	if err := os.WriteFile(file2, []byte("1234567890"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Calculate size
	size, err := calculateDirectorySize(tmpDir)
	if err != nil {
		t.Fatalf("calculateDirectorySize() error = %v", err)
	}

	expectedSize := int64(15) // 5 + 10 bytes
	if size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}
}

func TestCalculateDirectorySize_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	size, err := calculateDirectorySize(tmpDir)
	if err != nil {
		t.Fatalf("calculateDirectorySize() error = %v", err)
	}

	if size != 0 {
		t.Errorf("Expected size 0 for empty directory, got %d", size)
	}
}

func TestCalculateDirectorySize_WithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectory with files
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create files in both directories
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(subDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("12345"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	if err := os.WriteFile(file2, []byte("1234567890"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Calculate size - should include both files
	size, err := calculateDirectorySize(tmpDir)
	if err != nil {
		t.Fatalf("calculateDirectorySize() error = %v", err)
	}

	expectedSize := int64(15) // 5 + 10 bytes
	if size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}
}

func TestIsValidVMName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "valid simple name",
			input: "my-vm",
			valid: true,
		},
		{
			name:  "valid with underscores",
			input: "my_vm_123",
			valid: true,
		},
		{
			name:  "valid with dots",
			input: "vm.test.local",
			valid: true,
		},
		{
			name:  "valid mixed",
			input: "VM-123_test.v2",
			valid: true,
		},
		{
			name:  "empty name",
			input: "",
			valid: false,
		},
		{
			name:  "name with spaces",
			input: "my vm",
			valid: false,
		},
		{
			name:  "name with special chars",
			input: "vm@test",
			valid: false,
		},
		{
			name:  "name with slash",
			input: "vm/test",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidVMName(tt.input)
			if result != tt.valid {
				t.Errorf("isValidVMName(%q) = %v, want %v", tt.input, result, tt.valid)
			}
		})
	}
}

func TestHandleListBackupsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/backups", nil)
	w := httptest.NewRecorder()

	server.handleListBackups(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListBackupsBasic(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/backups", nil)
	w := httptest.NewRecorder()

	server.handleListBackups(w, req)

	// May fail with 500 if backup directory doesn't exist and can't be created (permission denied)
	// or succeed with 200 if directory can be created/accessed
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["backups"] == nil {
			t.Error("Expected backups field in response")
		}
	}
}

func TestHandleCreateBackupMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/backups/create", nil)
	w := httptest.NewRecorder()

	server.handleCreateBackup(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateBackupInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/backups/create", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateBackup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateBackupMissingVMName(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"description": "test backup",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/backups/create", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateBackup(w, req)

	// Handler validates vm_name upfront and rejects the empty value with 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRestoreBackupMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/backups/restore", nil)
	w := httptest.NewRecorder()

	server.handleRestoreBackup(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleRestoreBackupInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/backups/restore", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleRestoreBackup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRestoreBackupMissingBackupID(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"new_vm_name": "restored-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/backups/restore", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleRestoreBackup(w, req)

	// Handler sanitizes/validates backup_name upfront and rejects the empty value with 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVerifyBackupMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPut, "/backups/verify", nil)
	w := httptest.NewRecorder()

	server.handleVerifyBackup(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleVerifyBackupInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/backups/verify", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleVerifyBackup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVerifyBackupMissingBackupID(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/backups/verify", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleVerifyBackup(w, req)

	// Handler sanitizes/validates backup_name upfront and rejects the empty value with 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeleteBackupMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/backups/delete", nil)
	w := httptest.NewRecorder()

	server.handleDeleteBackup(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDeleteBackupMissingBackupID(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/backups/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeleteBackup(w, req)

	// Handler checks if backup exists, returns 404 for empty backup_name
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleDeleteBackupNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"backup_name": "nonexistent-backup-id",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/backups/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeleteBackup(w, req)

	// Should return 404 for non-existent backup
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestParseDiskList(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name: "standard output with --details",
			output: `Type       Device     Target     Source
------------------------------------------------
file       disk       vda        /var/lib/libvirt/images/test.qcow2
file       disk       sda        /var/lib/libvirt/images/disk2.qcow2`,
			expected: []string{
				"/var/lib/libvirt/images/test.qcow2",
				"/var/lib/libvirt/images/disk2.qcow2",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: []string{},
		},
		{
			name: "header only",
			output: `Type       Device     Target     Source
------------------------------------------------`,
			expected: []string{},
		},
		{
			name: "single disk",
			output: `Type       Device     Target     Source
------------------------------------------------
file       disk       vda        /path/to/disk.qcow2`,
			expected: []string{"/path/to/disk.qcow2"},
		},
		{
			name: "with blank lines",
			output: `Type       Device     Target     Source
------------------------------------------------
file       disk       vda        /path/to/disk1.qcow2

file       disk       sda        /path/to/disk2.qcow2`,
			expected: []string{
				"/path/to/disk1.qcow2",
				"/path/to/disk2.qcow2",
			},
		},
		{
			name: "mixed types",
			output: `Type       Device     Target     Source
------------------------------------------------
file       disk       vda        /var/lib/libvirt/images/test.qcow2
file       cdrom      hdc        /var/lib/libvirt/images/cdrom.iso`,
			expected: []string{
				"/var/lib/libvirt/images/test.qcow2",
				"/var/lib/libvirt/images/cdrom.iso",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDiskList(tt.output)
			if len(result) != len(tt.expected) {
				t.Errorf("parseDiskList() returned %d disks, want %d", len(result), len(tt.expected))
				return
			}
			for i, disk := range result {
				if disk != tt.expected[i] {
					t.Errorf("parseDiskList() disk[%d] = %v, want %v", i, disk, tt.expected[i])
				}
			}
		})
	}
}

func TestReplaceVMNameInXML(t *testing.T) {
	tests := []struct {
		name        string
		xmlInput    string
		newName     string
		expectError bool
		checkName   bool
	}{
		{
			name: "valid simple replacement",
			xmlInput: `<domain type='kvm'>
  <name>old-vm-name</name>
  <memory>1048576</memory>
</domain>`,
			newName:     "new-vm-name",
			expectError: false,
			checkName:   true,
		},
		{
			name: "valid with underscores",
			xmlInput: `<domain type='kvm'>
  <name>old_vm</name>
  <memory>1048576</memory>
</domain>`,
			newName:     "new_vm_123",
			expectError: false,
			checkName:   true,
		},
		{
			name: "valid with dots",
			xmlInput: `<domain type='kvm'>
  <name>test</name>
</domain>`,
			newName:     "test.v2.local",
			expectError: false,
			checkName:   true,
		},
		{
			name: "invalid name with spaces",
			xmlInput: `<domain type='kvm'>
  <name>test</name>
</domain>`,
			newName:     "my vm",
			expectError: true,
			checkName:   false,
		},
		{
			name: "invalid name with special chars",
			xmlInput: `<domain type='kvm'>
  <name>test</name>
</domain>`,
			newName:     "vm@test",
			expectError: true,
			checkName:   false,
		},
		{
			name: "invalid name empty",
			xmlInput: `<domain type='kvm'>
  <name>test</name>
</domain>`,
			newName:     "",
			expectError: true,
			checkName:   false,
		},
		{
			name:        "invalid XML",
			xmlInput:    `<domain type='kvm'><name>test`,
			newName:     "new-name",
			expectError: true,
			checkName:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := replaceVMNameInXML(tt.xmlInput, tt.newName)
			if (err != nil) != tt.expectError {
				t.Errorf("replaceVMNameInXML() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && tt.checkName {
				// Verify the new name is in the result
				if !containsHelper(result, "<name>"+tt.newName+"</name>") {
					t.Errorf("replaceVMNameInXML() result doesn't contain new name %q", tt.newName)
				}
			}
		})
	}
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
