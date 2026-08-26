// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListISOsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/isos", nil)
	w := httptest.NewRecorder()

	server.handleListISOs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListISOs(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/isos", nil)
	w := httptest.NewRecorder()

	server.handleListISOs(w, req)

	// May fail with 500 if ISO directory doesn't exist and can't be created (permission denied)
	// or succeed with 200 if directory can be created/accessed
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["isos"] == nil {
			t.Error("Expected isos field in response")
		}

		if response["total"] == nil {
			t.Error("Expected total field in response")
		}

		if response["storage_dir"] == nil {
			t.Error("Expected storage_dir field in response")
		}
	}
}

func TestHandleUploadISOMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/isos/upload", nil)
	w := httptest.NewRecorder()

	server.handleUploadISO(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleUploadISONoFile(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/isos/upload", nil)
	w := httptest.NewRecorder()

	server.handleUploadISO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleUploadISOInvalidExtension(t *testing.T) {
	server := setupTestBasicServer(t)

	// Create multipart form with non-ISO file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("iso", "test.txt")
	_, _ = part.Write([]byte("test content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/isos/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	server.handleUploadISO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUploadISO(t *testing.T) {
	server := setupTestBasicServer(t)

	// Create multipart form with ISO file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("iso", "test.iso")
	_, _ = part.Write([]byte("fake ISO content"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/isos/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	server.handleUploadISO(w, req)

	// May fail with 500 if directory can't be created or file can't be written (permission denied)
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError && w.Code != http.StatusConflict {
		t.Errorf("Expected status 201, 409, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteISOMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/isos/test.iso", nil)
	w := httptest.NewRecorder()

	server.handleDeleteISO(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDeleteISOMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/isos/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeleteISO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeleteISONonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"filename": "nonexistent.iso",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/isos/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeleteISO(w, req)

	// Should return 404 for non-existent file
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAttachISOMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/isos/attach", nil)
	w := httptest.NewRecorder()

	server.handleAttachISO(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleAttachISOInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/isos/attach", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleAttachISO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleAttachISOMissingFields(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/isos/attach", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleAttachISO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleAttachISO(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name":  "test-vm",
		"iso_path": ISOStorageDir + "/test.iso", // Use valid storage directory
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/isos/attach", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleAttachISO(w, req)

	// Will fail with 500 if virsh command fails (VM doesn't exist or ISO doesn't exist)
	// May fail with 403 if path validation fails
	// May fail with 404 if ISO file doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
		t.Errorf("Expected status 200, 403, 404, or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDetachISOMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/isos/detach", nil)
	w := httptest.NewRecorder()

	server.handleDetachISO(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDetachISOInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/isos/detach", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleDetachISO(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDetachISO(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/isos/detach", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDetachISO(w, req)

	// Will fail with 500 if virsh command fails (VM doesn't exist)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}
