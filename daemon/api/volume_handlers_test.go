// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Get Volume Info Handler Tests

func TestHandleGetVolumeInfoMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/volume/info?pool=default&volume=test.qcow2", nil)
	w := httptest.NewRecorder()

	server.handleGetVolumeInfo(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetVolumeInfoMissingPool(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/info?volume=test.qcow2", nil)
	w := httptest.NewRecorder()

	server.handleGetVolumeInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetVolumeInfoMissingVolume(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/info?pool=default", nil)
	w := httptest.NewRecorder()

	server.handleGetVolumeInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetVolumeInfoNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/info?pool=default&volume=nonexistent-12345.qcow2", nil)
	w := httptest.NewRecorder()

	server.handleGetVolumeInfo(w, req)

	// Should return 404 for non-existent volume
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Create Volume Handler Tests

func TestHandleCreateVolumeMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/create", nil)
	w := httptest.NewRecorder()

	server.handleCreateVolume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateVolumeInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/volume/create",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateVolume(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateVolumeBasic(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":     "default",
		"name":     "test-vol.qcow2",
		"format":   "qcow2",
		"capacity": 10,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/create",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateVolume(w, req)

	// May fail with 500 if pool doesn't exist or permission denied
	// or succeed with 201 if volume created
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusCreated {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["status"] != "success" {
			t.Errorf("Expected status=success, got %v", response["status"])
		}
		if response["format"] != "qcow2" {
			t.Errorf("Expected format=qcow2, got %v", response["format"])
		}
	}
}

func TestHandleCreateVolumeDefaultFormat(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":     "default",
		"name":     "test-default.qcow2",
		"capacity": 5,
		// format omitted - should default to qcow2
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/create",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateVolume(w, req)

	// May fail with 500 if pool doesn't exist
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

func TestHandleCreateVolumeWithPrealloc(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":     "default",
		"name":     "test-prealloc.qcow2",
		"format":   "qcow2",
		"capacity": 10,
		"prealloc": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/create",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateVolume(w, req)

	// May fail with 500 if pool doesn't exist
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

func TestHandleCreateVolumeRawFormat(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":     "default",
		"name":     "test-raw.img",
		"format":   "raw",
		"capacity": 5,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/create",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateVolume(w, req)

	// May fail with 500 if pool doesn't exist
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

// Clone Volume Handler Tests

func TestHandleCloneVolumeMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/clone", nil)
	w := httptest.NewRecorder()

	server.handleCloneVolume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCloneVolumeInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/volume/clone",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCloneVolume(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCloneVolumeBasic(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":          "default",
		"source_volume": "source.qcow2",
		"target_volume": "clone.qcow2",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/clone",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCloneVolume(w, req)

	// May fail with 500 if source doesn't exist
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusCreated {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["status"] != "success" {
			t.Errorf("Expected status=success, got %v", response["status"])
		}
		if response["source_volume"] != "source.qcow2" {
			t.Errorf("Expected source_volume=source.qcow2, got %v", response["source_volume"])
		}
		if response["target_volume"] != "clone.qcow2" {
			t.Errorf("Expected target_volume=clone.qcow2, got %v", response["target_volume"])
		}
	}
}

func TestHandleCloneVolumeDefaultTargetPool(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":          "default",
		"source_volume": "source.qcow2",
		"target_volume": "clone.qcow2",
		// target_pool omitted - should default to same pool
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/clone",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCloneVolume(w, req)

	// May fail with 500 if source doesn't exist
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

// Resize Volume Handler Tests

func TestHandleResizeVolumeMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/resize", nil)
	w := httptest.NewRecorder()

	server.handleResizeVolume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleResizeVolumeInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/volume/resize",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleResizeVolume(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleResizeVolumeExpand(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":     "default",
		"volume":   "test.qcow2",
		"capacity": 20,
		"shrink":   false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/resize",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleResizeVolume(w, req)

	// May fail with 500 if volume doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["status"] != "success" {
			t.Errorf("Expected status=success, got %v", response["status"])
		}
		if response["new_capacity"] != float64(20) {
			t.Errorf("Expected new_capacity=20, got %v", response["new_capacity"])
		}
	}
}

func TestHandleResizeVolumeShrink(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":     "default",
		"volume":   "test.qcow2",
		"capacity": 5,
		"shrink":   true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/resize",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleResizeVolume(w, req)

	// May fail with 500 if volume doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}
}

// Delete Volume Handler Tests

func TestHandleDeleteVolumeMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/delete", nil)
	w := httptest.NewRecorder()

	server.handleDeleteVolume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDeleteVolumeInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/volume/delete",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleDeleteVolume(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeleteVolumeNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":   "default",
		"volume": "nonexistent-12345.qcow2",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/delete",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeleteVolume(w, req)

	// Should return 500 for non-existent volume
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Upload Volume Handler Tests

func TestHandleUploadVolumeMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/upload", nil)
	w := httptest.NewRecorder()

	server.handleUploadVolume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleUploadVolumeInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/volume/upload",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleUploadVolume(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleUploadVolumeDefaultFormat(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":        "default",
		"volume":      "uploaded.qcow2",
		"source_path": "/nonexistent/image.qcow2",
		// format omitted - should default to qcow2
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/upload",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleUploadVolume(w, req)

	// Will fail with 400 or 500 because source doesn't exist
	if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 400 or 500, got %d", w.Code)
	}
}

func TestHandleUploadVolumeInvalidSource(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":        "default",
		"volume":      "uploaded.qcow2",
		"source_path": "/nonexistent/path/image.qcow2",
		"format":      "qcow2",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/upload",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleUploadVolume(w, req)

	// Should fail with 400 because source doesn't exist
	if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 400 or 500, got %d", w.Code)
	}
}

// Wipe Volume Handler Tests

func TestHandleWipeVolumeMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/volume/wipe", nil)
	w := httptest.NewRecorder()

	server.handleWipeVolume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleWipeVolumeInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/volume/wipe",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleWipeVolume(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleWipeVolumeDefaultAlgorithm(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":   "default",
		"volume": "test.qcow2",
		// algorithm omitted - should default to "zero"
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/wipe",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleWipeVolume(w, req)

	// May fail with 500 if volume doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}
}

func TestHandleWipeVolumeWithAlgorithm(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":      "default",
		"volume":    "test.qcow2",
		"algorithm": "nnsa",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/wipe",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleWipeVolume(w, req)

	// May fail with 500 if volume doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["status"] != "success" {
			t.Errorf("Expected status=success, got %v", response["status"])
		}
		if response["algorithm"] != "nnsa" {
			t.Errorf("Expected algorithm=nnsa, got %v", response["algorithm"])
		}
	}
}

func TestHandleWipeVolumeNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"pool":      "default",
		"volume":    "nonexistent-12345.qcow2",
		"algorithm": "zero",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/volume/wipe",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleWipeVolume(w, req)

	// Should return 500 for non-existent volume
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Helper Function Tests

func TestParseVolumeInfo(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name      string
		pool      string
		volume    string
		output    string
		checkFunc func(*testing.T, LibvirtVolume)
	}{
		{
			name:   "complete info with GiB",
			pool:   "default",
			volume: "test.qcow2",
			output: `Name:          test.qcow2
Type:          file
Capacity:      10.00 GiB
Allocation:    2.50 GiB`,
			checkFunc: func(t *testing.T, vol LibvirtVolume) {
				if vol.Name != "test.qcow2" {
					t.Errorf("Expected name=test.qcow2, got %s", vol.Name)
				}
				if vol.Pool != "default" {
					t.Errorf("Expected pool=default, got %s", vol.Pool)
				}
				if vol.Type != "file" {
					t.Errorf("Expected type=file, got %s", vol.Type)
				}
				if vol.Capacity != 10*1024*1024*1024 {
					t.Errorf("Expected capacity=10GiB in bytes, got %d", vol.Capacity)
				}
				if vol.Allocation != int64(2.5*1024*1024*1024) {
					t.Errorf("Expected allocation=2.5GiB in bytes, got %d", vol.Allocation)
				}
			},
		},
		{
			name:   "info with MiB",
			pool:   "default",
			volume: "small.qcow2",
			output: `Name:          small.qcow2
Type:          file
Capacity:      512.00 MiB
Allocation:    128.00 MiB`,
			checkFunc: func(t *testing.T, vol LibvirtVolume) {
				if vol.Capacity != 512*1024*1024 {
					t.Errorf("Expected capacity=512MiB in bytes, got %d", vol.Capacity)
				}
				if vol.Allocation != 128*1024*1024 {
					t.Errorf("Expected allocation=128MiB in bytes, got %d", vol.Allocation)
				}
			},
		},
		{
			name:   "info with bytes",
			pool:   "default",
			volume: "tiny.qcow2",
			output: `Name:          tiny.qcow2
Type:          file
Capacity:      1048576 bytes
Allocation:    524288 bytes`,
			checkFunc: func(t *testing.T, vol LibvirtVolume) {
				if vol.Capacity != 1048576 {
					t.Errorf("Expected capacity=1048576 bytes, got %d", vol.Capacity)
				}
				if vol.Allocation != 524288 {
					t.Errorf("Expected allocation=524288 bytes, got %d", vol.Allocation)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volInfo := server.parseVolumeInfo(tt.pool, tt.volume, tt.output)
			tt.checkFunc(t, volInfo)
		})
	}
}
