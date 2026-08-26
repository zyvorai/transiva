// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGetConsoleInfoMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/console/info?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetConsoleInfo(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetConsoleInfoMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/info", nil)
	w := httptest.NewRecorder()

	server.handleGetConsoleInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetConsoleInfo(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/info?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetConsoleInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response ConsoleInfo
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.VMName != "test-vm" {
		t.Errorf("Expected vm_name 'test-vm', got '%s'", response.VMName)
	}
}

func TestHandleGetConsoleInfoEmptyName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/info?name=", nil)
	w := httptest.NewRecorder()

	server.handleGetConsoleInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetConsoleInfoNonExistentVM(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/info?name=nonexistent-vm-12345", nil)
	w := httptest.NewRecorder()

	server.handleGetConsoleInfo(w, req)

	// Should still return 200 with partial info even if VM doesn't exist
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response ConsoleInfo
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.VMName != "nonexistent-vm-12345" {
		t.Errorf("Expected vm_name 'nonexistent-vm-12345', got '%s'", response.VMName)
	}

	// VM doesn't exist, so VNC should not be available
	if response.HasVNC {
		t.Error("Expected HasVNC to be false for nonexistent VM")
	}
}

func TestHandleVNCProxyMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/vnc", nil)
	w := httptest.NewRecorder()

	server.handleVNCProxy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleVNCProxy(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/vnc?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleVNCProxy(w, req)

	// May fail with 500 if virsh vncdisplay fails (VM doesn't exist or VNC not configured)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	// If successful, should return HTML
	if w.Code == http.StatusOK {
		contentType := w.Header().Get("Content-Type")
		if contentType != "text/html" {
			t.Errorf("Expected Content-Type text/html, got %s", contentType)
		}

		body := w.Body.String()
		if !contains(body, "VNC Console") {
			t.Error("Expected HTML to contain 'VNC Console'")
		}
		if !contains(body, "test-vm") {
			t.Error("Expected HTML to contain VM name")
		}
	}
}

func TestHandleSerialConsoleMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/serial", nil)
	w := httptest.NewRecorder()

	server.handleSerialConsole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSerialConsole(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/serial?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleSerialConsole(w, req)

	// Always returns 200 with HTML
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Should return HTML
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html" {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	body := w.Body.String()
	if !contains(body, "Serial Console") {
		t.Error("Expected HTML to contain 'Serial Console'")
	}
	if !contains(body, "test-vm") {
		t.Error("Expected HTML to contain VM name")
	}
}

func TestHandleGetSerialDeviceMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/console/serial?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetSerialDevice(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetSerialDeviceMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/console/serial", nil)
	w := httptest.NewRecorder()

	server.handleGetSerialDevice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetSerialDevice(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/console/serial?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetSerialDevice(w, req)

	// May fail with 500 if virsh dumpxml fails (VM doesn't exist)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["vm_name"] != "test-vm" {
			t.Errorf("Expected vm_name 'test-vm', got %v", response["vm_name"])
		}

		if response["has_serial"] == nil {
			t.Error("Expected has_serial field in response")
		}
	}
}

func TestHandleScreenshotMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/console/screenshot?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleScreenshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleScreenshotMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/screenshot", nil)
	w := httptest.NewRecorder()

	server.handleScreenshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleScreenshot(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/screenshot?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleScreenshot(w, req)

	// Will fail with 500 if virsh screenshot fails (VM doesn't exist or not running)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSendKeysMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/console/sendkeys", nil)
	w := httptest.NewRecorder()

	server.handleSendKeys(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleSendKeysInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/console/sendkeys", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleSendKeys(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSendKeys(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := `{"name":"test-vm","keys":["KEY_LEFTCTRL","KEY_LEFTALT","KEY_DELETE"]}`
	req := httptest.NewRequest(http.MethodPost, "/console/sendkeys", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleSendKeys(w, req)

	// Will fail with 500 if virsh send-key fails (VM doesn't exist or not running)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
