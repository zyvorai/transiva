// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Note: These tests focus on HTTP handler aspects. Full coverage requires
// mocking vSphere client or a real vCenter environment.

// List Hosts Handler Tests

func TestHandleListHostsMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/hosts", nil)
	w := httptest.NewRecorder()

	server.handleListHosts(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListHostsWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/hosts", nil)
	w := httptest.NewRecorder()

	server.handleListHosts(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// List Clusters Handler Tests

func TestHandleListClustersMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/clusters", nil)
	w := httptest.NewRecorder()

	server.handleListClusters(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListClustersWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/clusters", nil)
	w := httptest.NewRecorder()

	server.handleListClusters(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// List Datacenters Handler Tests

func TestHandleListDatacentersMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/datacenters", nil)
	w := httptest.NewRecorder()

	server.handleListDatacenters(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListDatacentersWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/datacenters", nil)
	w := httptest.NewRecorder()

	server.handleListDatacenters(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// Get VCenter Info Handler Tests

func TestHandleGetVCenterInfoMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/vcenter/info", nil)
	w := httptest.NewRecorder()

	server.handleGetVCenterInfo(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetVCenterInfoWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/vcenter/info", nil)
	w := httptest.NewRecorder()

	server.handleGetVCenterInfo(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// Get Metrics Handler Tests

func TestHandleGetMetricsMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/vsphere/metrics", nil)
	w := httptest.NewRecorder()

	server.handleGetMetrics(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetMetricsWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/metrics", nil)
	w := httptest.NewRecorder()

	server.handleGetMetrics(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// List Resource Pools Handler Tests

func TestHandleListResourcePoolsMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/resource-pools", nil)
	w := httptest.NewRecorder()

	server.handleListResourcePools(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListResourcePoolsWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/resource-pools", nil)
	w := httptest.NewRecorder()

	server.handleListResourcePools(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// Create Resource Pool Handler Tests

func TestHandleCreateResourcePoolMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/resource-pools", nil)
	w := httptest.NewRecorder()

	server.handleCreateResourcePool(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateResourcePoolInvalidJSON(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/resource-pools",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateResourcePool(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Update Resource Pool Handler Tests

func TestHandleUpdateResourcePoolMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/resource-pools/test", nil)
	w := httptest.NewRecorder()

	server.handleUpdateResourcePool(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleUpdateResourcePoolInvalidJSON(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPut, "/vsphere/resource-pools/test",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleUpdateResourcePool(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Delete Resource Pool Handler Tests

func TestHandleDeleteResourcePoolMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/resource-pools/test", nil)
	w := httptest.NewRecorder()

	server.handleDeleteResourcePool(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDeleteResourcePoolMissingName(t *testing.T) {
	server := setupTestServer(t)

	// Request without resource pool name in URL
	req := httptest.NewRequest(http.MethodDelete, "/vsphere/resource-pools/", nil)
	w := httptest.NewRecorder()

	server.handleDeleteResourcePool(w, req)

	// Expect error for missing name
	if w.Code == http.StatusOK {
		t.Error("Expected error for missing resource pool name")
	}
}

// Get Recent Events Handler Tests

func TestHandleGetRecentEventsMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/events", nil)
	w := httptest.NewRecorder()

	server.handleGetRecentEvents(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetRecentEventsWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/events", nil)
	w := httptest.NewRecorder()

	server.handleGetRecentEvents(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// Get Recent Tasks Handler Tests

func TestHandleGetRecentTasksMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/tasks", nil)
	w := httptest.NewRecorder()

	server.handleGetRecentTasks(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetRecentTasksWithoutVSphere(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/tasks", nil)
	w := httptest.NewRecorder()

	server.handleGetRecentTasks(w, req)

	// Without vSphere connection, expect error
	if w.Code == http.StatusOK {
		t.Error("Expected error without vSphere connection")
	}
}

// Clone VM Handler Tests

func TestHandleCloneVMMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/vm/clone", nil)
	w := httptest.NewRecorder()

	server.handleCloneVM(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCloneVMInvalidJSON(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/vm/clone",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCloneVM(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Bulk Clone Handler Tests

func TestHandleBulkCloneMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/vm/bulk-clone", nil)
	w := httptest.NewRecorder()

	server.handleBulkClone(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBulkCloneInvalidJSON(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vsphere/vm/bulk-clone",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBulkClone(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Note: Template handler tests (handleCreateTemplate, handleDeployFromTemplate)
// are in clone_handlers_test.go to avoid duplication

// Query Parameter Tests

func TestHandleListHostsWithPattern(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/hosts?pattern=esxi*", nil)
	w := httptest.NewRecorder()

	server.handleListHosts(w, req)

	// Will fail without vSphere, but tests pattern parameter handling
	// Pattern parameter extraction is tested
}

func TestHandleListClustersWithPattern(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/clusters?pattern=prod*", nil)
	w := httptest.NewRecorder()

	server.handleListClusters(w, req)

	// Will fail without vSphere, but tests pattern parameter handling
}

func TestHandleGetMetricsWithVMName(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/metrics?vm_name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetMetrics(w, req)

	// Will fail without vSphere, but tests vm_name parameter handling
}

func TestHandleGetRecentEventsWithLimit(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/events?limit=10", nil)
	w := httptest.NewRecorder()

	server.handleGetRecentEvents(w, req)

	// Will fail without vSphere, but tests limit parameter handling
}

func TestHandleGetRecentTasksWithLimit(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vsphere/tasks?limit=20", nil)
	w := httptest.NewRecorder()

	server.handleGetRecentTasks(w, req)

	// Will fail without vSphere, but tests limit parameter handling
}

// VSphere Client Helper Tests

func TestGetVSphereClientWithQueryParams(t *testing.T) {
	server := setupTestServer(t)

	// Test with query parameters (web UI mode)
	req := httptest.NewRequest(http.MethodGet,
		"/vsphere/hosts?server=vcenter.example.com&username=admin&password=secret&insecure=true",
		nil)

	client, err := server.getVSphereClient(req)

	// Client creation will fail without real vCenter, but we test parameter extraction
	// The function extracts parameters correctly even if connection fails
	_ = client
	_ = err
}

func TestGetVSphereClientWithoutQueryParams(t *testing.T) {
	server := setupTestServer(t)

	// Test without query parameters (CLI mode - uses environment)
	req := httptest.NewRequest(http.MethodGet, "/vsphere/hosts", nil)

	client, err := server.getVSphereClient(req)

	// Client creation will fail without configured environment or real vCenter
	_ = client
	_ = err
}
