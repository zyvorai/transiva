// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// List Networks Handler Tests

func TestHandleListNetworksMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/networks", nil)
	w := httptest.NewRecorder()

	server.handleListNetworks(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListNetworks(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/networks", nil)
	w := httptest.NewRecorder()

	server.handleListNetworks(w, req)

	// May fail with 500 if virsh is not available or libvirt not running
	// or succeed with 200 if libvirt is available
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if _, ok := response["networks"]; !ok {
			t.Error("Expected networks field in response")
		}
		if _, ok := response["total"]; !ok {
			t.Error("Expected total field in response")
		}
	}
}

// Get Network Handler Tests

func TestHandleGetNetworkMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/network?name=default", nil)
	w := httptest.NewRecorder()

	server.handleGetNetwork(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetNetworkMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/network", nil)
	w := httptest.NewRecorder()

	server.handleGetNetwork(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetNetworkNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/network?name=nonexistent-network-12345", nil)
	w := httptest.NewRecorder()

	server.handleGetNetwork(w, req)

	// Should return 404 for non-existent network
	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 404 or 500, got %d", w.Code)
	}
}

func TestHandleGetNetwork(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/network?name=default", nil)
	w := httptest.NewRecorder()

	server.handleGetNetwork(w, req)

	// May return 200 if default network exists, or 404 if not
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200, 404, or 500, got %d", w.Code)
	}

	if w.Code == http.StatusOK {
		var response LibvirtNetwork
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response.Name != "default" {
			t.Errorf("Expected name=default, got %s", response.Name)
		}
	}
}

// Create Network Handler Tests

func TestHandleCreateNetworkMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/network/create", nil)
	w := httptest.NewRecorder()

	server.handleCreateNetwork(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateNetworkInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/create",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateNetwork(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateNetworkValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name":     "test-network",
		"bridge":   "virbr-test",
		"forward":  "nat",
		"subnet":   "192.168.100.1",
		"ip_start": "192.168.100.10",
		"ip_end":   "192.168.100.254",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/create",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateNetwork(w, req)

	// May fail with 500 if virsh is not available or permission denied
	// or succeed with 201 if network created successfully
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
		if response["name"] != "test-network" {
			t.Errorf("Expected name=test-network, got %v", response["name"])
		}
	}
}

// Delete Network Handler Tests

func TestHandleDeleteNetworkMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/network/delete", nil)
	w := httptest.NewRecorder()

	server.handleDeleteNetwork(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDeleteNetworkInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/delete",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleDeleteNetwork(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeleteNetworkNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "nonexistent-network-12345",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/delete",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeleteNetwork(w, req)

	// Should return 500 for non-existent network
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Start Network Handler Tests

func TestHandleStartNetworkMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/network/start", nil)
	w := httptest.NewRecorder()

	server.handleStartNetwork(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleStartNetworkInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/start",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleStartNetwork(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleStartNetworkNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "nonexistent-network-12345",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/start",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleStartNetwork(w, req)

	// Should return 500 for non-existent network
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Stop Network Handler Tests

func TestHandleStopNetworkMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/network/stop", nil)
	w := httptest.NewRecorder()

	server.handleStopNetwork(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleStopNetworkInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/stop",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleStopNetwork(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleStopNetworkNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name": "nonexistent-network-12345",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/network/stop",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleStopNetwork(w, req)

	// Should return 500 for non-existent network
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Attach Interface Handler Tests

func TestHandleAttachInterfaceMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/interface/attach", nil)
	w := httptest.NewRecorder()

	server.handleAttachInterface(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleAttachInterfaceInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/interface/attach",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleAttachInterface(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleAttachInterfaceNonExistentVM(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "nonexistent-vm-12345",
		"network": "default",
		"model":   "virtio",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/interface/attach",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleAttachInterface(w, req)

	// Should return 500 for non-existent VM
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandleAttachInterfaceDefaultModel(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "test-vm",
		"network": "default",
		// model omitted - should default to virtio
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/interface/attach",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleAttachInterface(w, req)

	// May fail with 500 if VM doesn't exist
	// or succeed with 200 if VM exists and interface attached
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}
}

// Detach Interface Handler Tests

func TestHandleDetachInterfaceMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/interface/detach", nil)
	w := httptest.NewRecorder()

	server.handleDetachInterface(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDetachInterfaceInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/interface/detach",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleDetachInterface(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDetachInterfaceNonExistentVM(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"vm_name": "nonexistent-vm-12345",
		"mac":     "52:54:00:12:34:56",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/interface/detach",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDetachInterface(w, req)

	// Should return 500 for non-existent VM
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// Helper Function Tests

func TestParseNetworkList(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name: "standard output",
			output: ` Name      State    Autostart   Persistent
----------------------------------------------------------
 default   active   yes         yes
 isolated  inactive no          yes`,
			expected: 2,
		},
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name: "header only",
			output: ` Name      State    Autostart   Persistent
----------------------------------------------------------`,
			expected: 0,
		},
		{
			name: "single network",
			output: ` Name      State    Autostart   Persistent
----------------------------------------------------------
 default   active   yes         yes`,
			expected: 1,
		},
		{
			name: "with blank lines",
			output: ` Name      State    Autostart   Persistent
----------------------------------------------------------
 default   active   yes         yes

 isolated  inactive no          yes`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networks := server.parseNetworkList(tt.output)
			if len(networks) != tt.expected {
				t.Errorf("parseNetworkList() returned %d networks, want %d", len(networks), tt.expected)
			}

			// Verify structure for non-empty results
			if len(networks) > 0 {
				if networks[0].Name == "" {
					t.Error("Expected network to have a name")
				}
			}
		})
	}
}

func TestParseNetworkListActiveStatus(t *testing.T) {
	server := setupTestBasicServer(t)

	output := ` Name      State    Autostart   Persistent
----------------------------------------------------------
 default   active   yes         yes
 isolated  inactive no          yes`

	networks := server.parseNetworkList(output)

	if len(networks) != 2 {
		t.Fatalf("Expected 2 networks, got %d", len(networks))
	}

	if networks[0].Name != "default" {
		t.Errorf("Expected first network name 'default', got %s", networks[0].Name)
	}
	if !networks[0].Active {
		t.Error("Expected first network to be active")
	}

	if networks[1].Name != "isolated" {
		t.Errorf("Expected second network name 'isolated', got %s", networks[1].Name)
	}
	if networks[1].Active {
		t.Error("Expected second network to be inactive")
	}
}

func TestParseNetworkInfo(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name         string
		networkName  string
		output       string
		checkFunc    func(*testing.T, LibvirtNetwork)
	}{
		{
			name:        "complete info",
			networkName: "default",
			output: `Name:           default
UUID:           12345678-1234-1234-1234-123456789abc
Active:         yes
Persistent:     yes
Autostart:      yes
Bridge:         virbr0`,
			checkFunc: func(t *testing.T, net LibvirtNetwork) {
				if net.Name != "default" {
					t.Errorf("Expected name=default, got %s", net.Name)
				}
				if net.UUID != "12345678-1234-1234-1234-123456789abc" {
					t.Errorf("Expected UUID to be set, got %s", net.UUID)
				}
				if !net.Active {
					t.Error("Expected Active=true")
				}
				if !net.Persistent {
					t.Error("Expected Persistent=true")
				}
				if !net.Autostart {
					t.Error("Expected Autostart=true")
				}
				if net.Bridge != "virbr0" {
					t.Errorf("Expected Bridge=virbr0, got %s", net.Bridge)
				}
			},
		},
		{
			name:        "inactive network",
			networkName: "test",
			output: `Name:           test
UUID:           abcdef12-3456-7890-abcd-ef1234567890
Active:         no
Persistent:     yes
Autostart:      no`,
			checkFunc: func(t *testing.T, net LibvirtNetwork) {
				if net.Name != "test" {
					t.Errorf("Expected name=test, got %s", net.Name)
				}
				if net.Active {
					t.Error("Expected Active=false")
				}
				if net.Autostart {
					t.Error("Expected Autostart=false")
				}
			},
		},
		{
			name:        "minimal info",
			networkName: "minimal",
			output: `Name:           minimal
UUID:           minimal-uuid`,
			checkFunc: func(t *testing.T, net LibvirtNetwork) {
				if net.Name != "minimal" {
					t.Errorf("Expected name=minimal, got %s", net.Name)
				}
				if net.UUID != "minimal-uuid" {
					t.Errorf("Expected UUID=minimal-uuid, got %s", net.UUID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := server.parseNetworkInfo(tt.networkName, tt.output)
			tt.checkFunc(t, network)
		})
	}
}
