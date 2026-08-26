// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGenerateConfigMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/config/generate", nil)
	w := httptest.NewRecorder()

	server.handleGenerateConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGenerateConfigInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/config/generate", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleGenerateConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGenerateConfigMissingFields(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/config/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleGenerateConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGenerateConfig(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"os_type":    "linux",
		"os_flavor":  "ubuntu",
		"vmdk_path":  "/tmp/test.vmdk",
		"output_dir": "/tmp/output",
		"vm_name":    "test-vm",
		"memory":     4096,
		"vcpus":      4,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/config/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleGenerateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["config_yaml"] == nil {
		t.Error("Expected config_yaml field in response")
	}

	if response["config_path"] == nil {
		t.Error("Expected config_path field in response")
	}
}

func TestHandleGenerateConfigWithDefaults(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"os_type":    "linux",
		"os_flavor":  "debian",
		"vmdk_path":  "/tmp/test.vmdk",
		"output_dir": "/tmp/output",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/config/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleGenerateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGenerateConfigWithService(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"os_type":          "linux",
		"os_flavor":        "ubuntu",
		"vmdk_path":        "/tmp/test.vmdk",
		"output_dir":       "/tmp/output",
		"vm_name":          "test-vm",
		"generate_service": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/config/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleGenerateConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["service_file"] == nil {
		t.Error("Expected service_file field in response when generate_service is true")
	}

	if response["service_path"] == nil {
		t.Error("Expected service_path field in response when generate_service is true")
	}
}

func TestHandleListConfigTemplatesMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/config/templates", nil)
	w := httptest.NewRecorder()

	server.handleListConfigTemplates(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListConfigTemplates(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/config/templates", nil)
	w := httptest.NewRecorder()

	server.handleListConfigTemplates(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	templates, ok := response["templates"].([]interface{})
	if !ok {
		t.Fatal("Expected templates array in response")
	}

	if len(templates) == 0 {
		t.Error("Expected at least one template")
	}
}
