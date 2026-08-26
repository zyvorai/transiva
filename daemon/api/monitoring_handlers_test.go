// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Get Domain Stats Handler Tests

func TestHandleGetDomainStatsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/stats/domain?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetDomainStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetDomainStatsMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/domain", nil)
	w := httptest.NewRecorder()

	server.handleGetDomainStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetDomainStatsNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/domain?name=nonexistent-vm-12345", nil)
	w := httptest.NewRecorder()

	server.handleGetDomainStats(w, req)

	// getDomainStatistics returns stats even for non-existent VMs (with partial data)
	// so we may get 200 with minimal stats or 500 if virsh fails
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}
}

func TestHandleGetDomainStatsValid(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/domain?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetDomainStats(w, req)

	// May fail with 500 if VM doesn't exist or succeed with 200
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response DomainStats
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response.Name != "test-vm" {
			t.Errorf("Expected name=test-vm, got %s", response.Name)
		}
		if response.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}
	}
}

// Get All Domain Stats Handler Tests

func TestHandleGetAllDomainStatsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/stats/all", nil)
	w := httptest.NewRecorder()

	server.handleGetAllDomainStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetAllDomainStats(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/all", nil)
	w := httptest.NewRecorder()

	server.handleGetAllDomainStats(w, req)

	// May fail with 500 if virsh is not available
	// or succeed with 200 if virsh is available
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if _, ok := response["domains"]; !ok {
			t.Error("Expected domains field in response")
		}
		if _, ok := response["total"]; !ok {
			t.Error("Expected total field in response")
		}
		if _, ok := response["timestamp"]; !ok {
			t.Error("Expected timestamp field in response")
		}
	}
}

// Get CPU Stats Handler Tests

func TestHandleGetCPUStatsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/stats/cpu?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetCPUStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetCPUStatsMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/cpu", nil)
	w := httptest.NewRecorder()

	server.handleGetCPUStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetCPUStatsNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/cpu?name=nonexistent-vm-12345", nil)
	w := httptest.NewRecorder()

	server.handleGetCPUStats(w, req)

	// Should return 500 for non-existent domain
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandleGetCPUStatsValid(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/cpu?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetCPUStats(w, req)

	// May fail with 500 if VM doesn't exist or succeed with 200
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["name"] != "test-vm" {
			t.Errorf("Expected name=test-vm, got %v", response["name"])
		}
		if _, ok := response["cpu_stats"]; !ok {
			t.Error("Expected cpu_stats field in response")
		}
		if _, ok := response["timestamp"]; !ok {
			t.Error("Expected timestamp field in response")
		}
	}
}

// Get Memory Stats Handler Tests

func TestHandleGetMemoryStatsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/stats/memory?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetMemoryStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetMemoryStatsMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/memory", nil)
	w := httptest.NewRecorder()

	server.handleGetMemoryStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetMemoryStatsNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/memory?name=nonexistent-vm-12345", nil)
	w := httptest.NewRecorder()

	server.handleGetMemoryStats(w, req)

	// Should return 500 for non-existent domain
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandleGetMemoryStatsValid(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/memory?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetMemoryStats(w, req)

	// May fail with 500 if VM doesn't exist or succeed with 200
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["name"] != "test-vm" {
			t.Errorf("Expected name=test-vm, got %v", response["name"])
		}
		if _, ok := response["memory"]; !ok {
			t.Error("Expected memory field in response")
		}
		if _, ok := response["timestamp"]; !ok {
			t.Error("Expected timestamp field in response")
		}
	}
}

// Get Disk I/O Stats Handler Tests

func TestHandleGetDiskIOStatsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/stats/disk?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetDiskIOStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetDiskIOStatsMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/disk", nil)
	w := httptest.NewRecorder()

	server.handleGetDiskIOStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetDiskIOStatsNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/disk?name=nonexistent-vm-12345", nil)
	w := httptest.NewRecorder()

	server.handleGetDiskIOStats(w, req)

	// Should return 500 for non-existent domain
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandleGetDiskIOStatsValid(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/disk?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetDiskIOStats(w, req)

	// May fail with 500 if VM doesn't exist or succeed with 200
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["name"] != "test-vm" {
			t.Errorf("Expected name=test-vm, got %v", response["name"])
		}
		if _, ok := response["disks"]; !ok {
			t.Error("Expected disks field in response")
		}
		if _, ok := response["timestamp"]; !ok {
			t.Error("Expected timestamp field in response")
		}
	}
}

// Get Network I/O Stats Handler Tests

func TestHandleGetNetworkIOStatsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/stats/network?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetNetworkIOStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetNetworkIOStatsMissingName(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/network", nil)
	w := httptest.NewRecorder()

	server.handleGetNetworkIOStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetNetworkIOStatsNonExistent(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/network?name=nonexistent-vm-12345", nil)
	w := httptest.NewRecorder()

	server.handleGetNetworkIOStats(w, req)

	// Should return 500 for non-existent domain
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandleGetNetworkIOStatsValid(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stats/network?name=test-vm", nil)
	w := httptest.NewRecorder()

	server.handleGetNetworkIOStats(w, req)

	// May fail with 500 if VM doesn't exist or succeed with 200
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["name"] != "test-vm" {
			t.Errorf("Expected name=test-vm, got %v", response["name"])
		}
		if _, ok := response["networks"]; !ok {
			t.Error("Expected networks field in response")
		}
		if _, ok := response["timestamp"]; !ok {
			t.Error("Expected timestamp field in response")
		}
	}
}

// Helper Function Tests

func TestParseCPUStats(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name      string
		output    string
		checkFunc func(*testing.T, CPUStats)
	}{
		{
			name: "complete CPU stats",
			output: `cpu_time 123.456
user_time 78.9
system_time 44.556
vcpu 4`,
			checkFunc: func(t *testing.T, stats CPUStats) {
				if stats.VCPUs != 4 {
					t.Errorf("Expected VCPUs=4, got %d", stats.VCPUs)
				}
				if stats.CPUTime == 0 {
					t.Error("Expected CPUTime to be set")
				}
				if stats.UserTime == 0 {
					t.Error("Expected UserTime to be set")
				}
				if stats.SystemTime == 0 {
					t.Error("Expected SystemTime to be set")
				}
			},
		},
		{
			name:   "empty output",
			output: "",
			checkFunc: func(t *testing.T, stats CPUStats) {
				if stats.VCPUs != 0 {
					t.Errorf("Expected VCPUs=0, got %d", stats.VCPUs)
				}
			},
		},
		{
			name: "partial stats",
			output: `cpu_time 100.0
vcpu 2`,
			checkFunc: func(t *testing.T, stats CPUStats) {
				if stats.VCPUs != 2 {
					t.Errorf("Expected VCPUs=2, got %d", stats.VCPUs)
				}
				if stats.CPUTime == 0 {
					t.Error("Expected CPUTime to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := server.parseCPUStats(tt.output)
			tt.checkFunc(t, stats)
		})
	}
}

func TestParseMemoryStats(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name      string
		output    string
		checkFunc func(*testing.T, MemStats)
	}{
		{
			name: "complete memory stats",
			output: `actual 4194304
unused 2097152
swap_in 1024
swap_out 2048`,
			checkFunc: func(t *testing.T, stats MemStats) {
				if stats.Total != 4194304 {
					t.Errorf("Expected Total=4194304, got %d", stats.Total)
				}
				if stats.Available != 2097152 {
					t.Errorf("Expected Available=2097152, got %d", stats.Available)
				}
				if stats.SwapIn != 1024 {
					t.Errorf("Expected SwapIn=1024, got %d", stats.SwapIn)
				}
				if stats.SwapOut != 2048 {
					t.Errorf("Expected SwapOut=2048, got %d", stats.SwapOut)
				}
				if stats.Used == 0 {
					t.Error("Expected Used to be calculated")
				}
				if stats.Usage == 0.0 {
					t.Error("Expected Usage percentage to be calculated")
				}
			},
		},
		{
			name:   "empty output",
			output: "",
			checkFunc: func(t *testing.T, stats MemStats) {
				if stats.Total != 0 {
					t.Errorf("Expected Total=0, got %d", stats.Total)
				}
			},
		},
		{
			name: "usable instead of unused",
			output: `available 8388608
usable 4194304`,
			checkFunc: func(t *testing.T, stats MemStats) {
				if stats.Total != 8388608 {
					t.Errorf("Expected Total=8388608, got %d", stats.Total)
				}
				if stats.Available != 4194304 {
					t.Errorf("Expected Available=4194304, got %d", stats.Available)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := server.parseMemoryStats(tt.output)
			tt.checkFunc(t, stats)
		})
	}
}

func TestGetDiskIOStats(t *testing.T) {
	server := setupTestBasicServer(t)

	// Test with non-existent VM
	_, err := server.getDiskIOStats("nonexistent-vm-12345")
	if err == nil {
		t.Error("Expected error for non-existent VM")
	}
}

func TestGetNetworkIOStats(t *testing.T) {
	server := setupTestBasicServer(t)

	// Test with non-existent VM
	_, err := server.getNetworkIOStats("nonexistent-vm-12345")
	if err == nil {
		t.Error("Expected error for non-existent VM")
	}
}

func TestGetDomainStatistics(t *testing.T) {
	server := setupTestBasicServer(t)

	// Test with non-existent VM - should still return stats object, just with minimal data
	stats, err := server.getDomainStatistics("nonexistent-vm-12345")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if stats == nil {
		t.Fatal("Expected stats object, got nil")
	}
	if stats.Name != "nonexistent-vm-12345" {
		t.Errorf("Expected name=nonexistent-vm-12345, got %s", stats.Name)
	}
	if stats.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}
