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

// Validate Migration Handler Tests

func TestHandleValidateMigrationMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/validate/migration", nil)
	w := httptest.NewRecorder()

	server.handleValidateMigration(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleValidateMigrationInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/validate/migration",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleValidateMigration(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleValidateMigrationFileNotFound(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]string{
		"path": "/nonexistent/file.vmdk",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/validate/migration",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleValidateMigration(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "complete" {
		t.Errorf("Expected status=complete, got %v", response["status"])
	}

	validation := response["validation"].(map[string]interface{})
	if validation["valid"].(bool) {
		t.Error("Expected valid=false for nonexistent file")
	}

	errors := validation["errors"].([]interface{})
	if len(errors) == 0 {
		t.Error("Expected errors for nonexistent file")
	}
}

func TestHandleValidateMigrationEmptyPath(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]string{
		"path": "",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/validate/migration",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleValidateMigration(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	validation := response["validation"].(map[string]interface{})
	// Empty path should still return valid structure
	if _, ok := validation["valid"]; !ok {
		t.Error("Expected valid field in validation")
	}
}

// Verify Migration Handler Tests

func TestHandleVerifyMigrationMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/verify/migration", nil)
	w := httptest.NewRecorder()

	server.handleVerifyMigration(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleVerifyMigrationInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/verify/migration",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleVerifyMigration(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVerifyMigrationBasicRequest(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name":   "test-vm",
		"boot_test": false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/verify/migration",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleVerifyMigration(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "complete" {
		t.Errorf("Expected status=complete, got %v", response["status"])
	}

	if _, ok := response["verification"]; !ok {
		t.Error("Expected verification field in response")
	}
}

func TestHandleVerifyMigrationWithTests(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name":       "test-vm",
		"boot_test":     true,
		"checksum_test": true,
		"source_path":   "/tmp/source.vmdk",
		"converted_path": "/tmp/converted.qcow2",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/verify/migration",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleVerifyMigration(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	verification := response["verification"].(map[string]interface{})

	// Should have details field
	if _, ok := verification["details"]; !ok {
		t.Error("Expected details field in verification")
	}
}

// Check Compatibility Handler Tests

func TestHandleCheckCompatibilityMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/check/compatibility", nil)
	w := httptest.NewRecorder()

	server.handleCheckCompatibility(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCheckCompatibilityInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/check/compatibility",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCheckCompatibility(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCheckCompatibilityBasicRequest(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
		"os_type": "linux",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/check/compatibility",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCheckCompatibility(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "complete" {
		t.Errorf("Expected status=complete, got %v", response["status"])
	}

	if _, ok := response["compatibility"]; !ok {
		t.Error("Expected compatibility field in response")
	}
}

func TestHandleCheckCompatibilityWithFirmware(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name     string
		firmware string
	}{
		{"BIOS", "bios"},
		{"UEFI", "uefi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"vm_name":  "test-vm",
				"firmware": tt.firmware,
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/check/compatibility",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleCheckCompatibility(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestHandleCheckCompatibilityWithFeatures(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
		"features": []string{"nested-virtualization", "tpm", "secureboot"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/check/compatibility",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCheckCompatibility(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	compatibility := response["compatibility"].(map[string]interface{})
	details := compatibility["details"].(map[string]interface{})

	// Should have requested features in details
	if _, ok := details["requested_features"]; !ok {
		t.Error("Expected requested_features in details")
	}
}

// Test Migration Handler Tests

func TestHandleTestMigrationMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/test/migration", nil)
	w := httptest.NewRecorder()

	server.handleTestMigration(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleTestMigrationInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/test/migration",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleTestMigration(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleTestMigrationDefaultTests(t *testing.T) {
	server := setupTestBasicServer(t)

	// When no tests specified, should use defaults (boot, disk)
	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/test/migration",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleTestMigration(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "complete" {
		t.Errorf("Expected status=complete, got %v", response["status"])
	}

	if _, ok := response["test"]; !ok {
		t.Error("Expected test field in response")
	}
}

func TestHandleTestMigrationAllTests(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
		"tests":   []string{"boot", "network", "disk", "shutdown"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/test/migration",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleTestMigration(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	test := response["test"].(map[string]interface{})
	details := test["details"].(map[string]interface{})

	// Should have tests field with results
	if _, ok := details["tests"]; !ok {
		t.Error("Expected tests field in details")
	}
}

func TestHandleTestMigrationWithAutoShutdown(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name":       "test-vm",
		"tests":         []string{"boot"},
		"auto_shutdown": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/test/migration",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleTestMigration(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify response structure
	if _, ok := response["test"]; !ok {
		t.Error("Expected test field in response")
	}
}

func TestHandleTestMigrationIndividualTests(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name     string
		testType string
	}{
		{"BootTest", "boot"},
		{"NetworkTest", "network"},
		{"DiskTest", "disk"},
		{"ShutdownTest", "shutdown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"vm_name": "test-vm",
				"tests":   []string{tt.testType},
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/test/migration",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleTestMigration(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

// Helper Function Tests

func TestGetDiskInfo(t *testing.T) {
	// Test with nonexistent file
	info := getDiskInfo("/nonexistent/file.qcow2")
	if info != nil {
		t.Error("Expected nil for nonexistent file")
	}
}

func TestGetDiskInfoWithTempFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(tmpFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// getDiskInfo will fail on non-image file, but should handle gracefully
	info := getDiskInfo(tmpFile)
	// Info might be nil if qemu-img fails on non-image file
	// This is expected behavior
	_ = info
}

// Integration-style test for validation result structure
func TestValidationResultStructure(t *testing.T) {
	result := ValidationResult{
		Valid:    true,
		Errors:   []string{"error1", "error2"},
		Warnings: []string{"warning1"},
		Details:  map[string]interface{}{"key": "value"},
	}

	// Marshal and unmarshal to verify JSON structure
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationResult: %v", err)
	}

	var decoded ValidationResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ValidationResult: %v", err)
	}

	if decoded.Valid != result.Valid {
		t.Errorf("Expected valid=%v, got %v", result.Valid, decoded.Valid)
	}
	if len(decoded.Errors) != len(result.Errors) {
		t.Errorf("Expected %d errors, got %d", len(result.Errors), len(decoded.Errors))
	}
	if len(decoded.Warnings) != len(result.Warnings) {
		t.Errorf("Expected %d warnings, got %d", len(result.Warnings), len(decoded.Warnings))
	}
}
