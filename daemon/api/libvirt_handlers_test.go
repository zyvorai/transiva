// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListLibvirtDomainsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domains", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtDomains(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListLibvirtDomains(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domains", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtDomains(w, req)

	// May fail with 500 if virsh command fails
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["domains"] == nil {
			t.Error("Expected domains field in response")
		}

		if response["total"] == nil {
			t.Error("Expected total field in response")
		}
	}
}

func TestHandleGetLibvirtDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain?name=test", nil)
	w := httptest.NewRecorder()

	server.handleGetLibvirtDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetLibvirtDomainMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain", nil)
	w := httptest.NewRecorder()

	server.handleGetLibvirtDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetLibvirtDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetLibvirtDomain(w, req)

	// Will fail with 404 if domain doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStartLibvirtDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain/start", nil)
	w := httptest.NewRecorder()

	server.handleStartLibvirtDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleStartLibvirtDomainInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/start", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleStartLibvirtDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleStartLibvirtDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/start", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleStartLibvirtDomain(w, req)

	// Will fail with 500 if domain doesn't exist or is already running
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleShutdownLibvirtDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain/shutdown", nil)
	w := httptest.NewRecorder()

	server.handleShutdownLibvirtDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleShutdownLibvirtDomainInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/shutdown", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleShutdownLibvirtDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleShutdownLibvirtDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/shutdown", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleShutdownLibvirtDomain(w, req)

	// Will fail with 500 if domain doesn't exist or is not running
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDestroyLibvirtDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain/destroy", nil)
	w := httptest.NewRecorder()

	server.handleDestroyLibvirtDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDestroyLibvirtDomainInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/destroy", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleDestroyLibvirtDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDestroyLibvirtDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/destroy", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDestroyLibvirtDomain(w, req)

	// Will fail with 500 if domain doesn't exist or is not running
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRebootLibvirtDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain/reboot", nil)
	w := httptest.NewRecorder()

	server.handleRebootLibvirtDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleRebootLibvirtDomainInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/reboot", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleRebootLibvirtDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRebootLibvirtDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/reboot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleRebootLibvirtDomain(w, req)

	// Will fail with 500 if domain doesn't exist or is not running
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlePauseLibvirtDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain/pause", nil)
	w := httptest.NewRecorder()

	server.handlePauseLibvirtDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandlePauseLibvirtDomainInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/pause", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handlePauseLibvirtDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlePauseLibvirtDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/pause", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handlePauseLibvirtDomain(w, req)

	// Will fail with 500 if domain doesn't exist or is not running
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleResumeLibvirtDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/domain/resume", nil)
	w := httptest.NewRecorder()

	server.handleResumeLibvirtDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleResumeLibvirtDomainInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/resume", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleResumeLibvirtDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleResumeLibvirtDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "test-vm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/domain/resume", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleResumeLibvirtDomain(w, req)

	// Will fail with 500 if domain doesn't exist or is not paused
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleListLibvirtSnapshotsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/snapshots?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtSnapshots(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListLibvirtSnapshotsMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/snapshots", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtSnapshots(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleListLibvirtSnapshots(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/snapshots?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtSnapshots(w, req)

	// Will fail with 500 if domain doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["snapshots"] == nil {
			t.Error("Expected snapshots field in response")
		}

		if response["domain"] != "test-vm" {
			t.Errorf("Expected domain 'test-vm', got %v", response["domain"])
		}
	}
}

func TestHandleCreateLibvirtSnapshotMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/snapshot/create", nil)
	w := httptest.NewRecorder()

	server.handleCreateLibvirtSnapshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateLibvirtSnapshotInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/snapshot/create", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateLibvirtSnapshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateLibvirtSnapshot(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domain_name":   "test-vm",
		"snapshot_name": "test-snapshot",
		"description":   "Test snapshot",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/snapshot/create", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateLibvirtSnapshot(w, req)

	// Will fail with 500 if domain doesn't exist
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRevertLibvirtSnapshotMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/snapshot/revert", nil)
	w := httptest.NewRecorder()

	server.handleRevertLibvirtSnapshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleRevertLibvirtSnapshotInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/snapshot/revert", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleRevertLibvirtSnapshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRevertLibvirtSnapshot(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domain_name":   "test-vm",
		"snapshot_name": "test-snapshot",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/snapshot/revert", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleRevertLibvirtSnapshot(w, req)

	// Will fail with 500 if domain or snapshot doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteLibvirtSnapshotMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/snapshot/delete", nil)
	w := httptest.NewRecorder()

	server.handleDeleteLibvirtSnapshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDeleteLibvirtSnapshotInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/snapshot/delete", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleDeleteLibvirtSnapshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeleteLibvirtSnapshot(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domain_name":   "test-vm",
		"snapshot_name": "test-snapshot",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/snapshot/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeleteLibvirtSnapshot(w, req)

	// Will fail with 500 if domain or snapshot doesn't exist
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleListLibvirtPoolsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/pools", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtPools(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListLibvirtPools(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/pools", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtPools(w, req)

	// May fail with 500 if virsh command fails
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["pools"] == nil {
			t.Error("Expected pools field in response")
		}

		if response["total"] == nil {
			t.Error("Expected total field in response")
		}
	}
}

func TestHandleListLibvirtVolumesMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/volumes?pool=default", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtVolumes(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListLibvirtVolumes(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/volumes?pool=default", nil)
	w := httptest.NewRecorder()

	server.handleListLibvirtVolumes(w, req)

	// May fail with 500 if pool doesn't exist or virsh command fails
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200, 400, or 500, got %d: %s", w.Code, w.Body.String())
	}
}
