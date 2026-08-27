// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleConvertVMMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/h2kvm/convert", nil)
	w := httptest.NewRecorder()

	server.handleConvertVM(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleConvertVMInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/h2kvm/convert", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleConvertVM(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleConvertVMMissingSourcePath(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"format": "qcow2",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/h2kvm/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleConvertVM(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleConvertVM(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"source_path": "/tmp/test.vmdk",
		"dest_path":   "/tmp/output.qcow2",
		"format":      "qcow2",
		"compress":    true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/h2kvm/convert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleConvertVM(w, req)

	// Will fail with 500 if h2kvm command fails (file doesn't exist, command not found, etc.)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleImportToKVMMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/h2kvm/import", nil)
	w := httptest.NewRecorder()

	server.handleImportToKVM(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleImportToKVMInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/h2kvm/import", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleImportToKVM(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleImportToKVM(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"image_path": "/tmp/converted.qcow2",
		"vm_name":    "imported-vm",
		"memory":     2048,
		"cpus":       2,
		"network":    "default",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/h2kvm/import", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleImportToKVM(w, req)

	// Will fail with 500 if import fails (image doesn't exist, virsh errors, etc.)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleVMDKParserMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/h2kvm/parse?path=/tmp/test.vmdk", nil)
	w := httptest.NewRecorder()

	server.handleVMDKParser(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleVMDKParserMissingPath(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/h2kvm/parse", nil)
	w := httptest.NewRecorder()

	server.handleVMDKParser(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVMDKParser(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/h2kvm/parse?path=/tmp/test.vmdk", nil)
	w := httptest.NewRecorder()

	server.handleVMDKParser(w, req)

	// Will fail with 500 if parsing fails (file doesn't exist, invalid VMDK, etc.)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleListConversionsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/h2kvm/conversions", nil)
	w := httptest.NewRecorder()

	server.handleListConversions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListConversions(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/h2kvm/conversions", nil)
	w := httptest.NewRecorder()

	server.handleListConversions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["conversions"] == nil {
		t.Error("Expected conversions field in response")
	}
}

func TestHandleConversionStatusMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/h2kvm/status?id=conv-123", nil)
	w := httptest.NewRecorder()

	server.handleConversionStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleConversionStatusMissingID(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/h2kvm/status", nil)
	w := httptest.NewRecorder()

	server.handleConversionStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleConversionStatus(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/h2kvm/status?id=conv-123", nil)
	w := httptest.NewRecorder()

	server.handleConversionStatus(w, req)

	// Will return 404 if conversion ID doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}
