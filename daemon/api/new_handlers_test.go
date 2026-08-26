// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/logger"
)

func newTestServer() *Server {
	log := logger.New("error") // Use error level to reduce test output
	detector := capabilities.NewDetector(log)
	mgr := jobs.NewManager(log, detector)
	return NewServer(mgr, detector, log, ":8080", nil, nil)
}

// TestNetworkHandlers tests network management endpoints
func TestNetworkHandlers(t *testing.T) {
	server := newTestServer()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "list networks - GET only",
			handler:        server.handleListNetworks,
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "list networks - POST not allowed",
			handler:        server.handleListNetworks,
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "get network - missing name",
			handler:        server.handleGetNetwork,
			method:         http.MethodGet,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "create network - invalid JSON",
			handler:        server.handleCreateNetwork,
			method:         http.MethodPost,
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "start network - valid request structure",
			handler: server.handleStartNetwork,
			method:  http.MethodPost,
			body: map[string]string{
				"name": "test-network",
			},
			expectedStatus: http.StatusInternalServerError, // Will fail without virsh but validates structure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					req = httptest.NewRequest(tt.method, "/test", bytes.NewBufferString(str))
				} else {
					jsonBody, _ := json.Marshal(tt.body)
					req = httptest.NewRequest(tt.method, "/test", bytes.NewBuffer(jsonBody))
				}
			} else {
				req = httptest.NewRequest(tt.method, "/test", nil)
			}

			w := httptest.NewRecorder()
			tt.handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestVolumeHandlers tests volume operation endpoints
func TestVolumeHandlers(t *testing.T) {
	server := newTestServer()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		method         string
		query          string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "get volume info - missing parameters",
			handler:        server.handleGetVolumeInfo,
			method:         http.MethodGet,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "create volume - invalid JSON",
			handler:        server.handleCreateVolume,
			method:         http.MethodPost,
			body:           "not json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "create volume - valid structure",
			handler: server.handleCreateVolume,
			method:  http.MethodPost,
			body: map[string]interface{}{
				"pool":     "default",
				"name":     "test-vol",
				"format":   "qcow2",
				"capacity": 10,
			},
			expectedStatus: http.StatusInternalServerError, // Fails without virsh
		},
		{
			name:    "clone volume - valid structure",
			handler: server.handleCloneVolume,
			method:  http.MethodPost,
			body: map[string]string{
				"pool":          "default",
				"source_volume": "source",
				"target_volume": "target",
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "delete volume - wrong method",
			handler:        server.handleDeleteVolume,
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					req = httptest.NewRequest(tt.method, "/test"+tt.query, bytes.NewBufferString(str))
				} else {
					jsonBody, _ := json.Marshal(tt.body)
					req = httptest.NewRequest(tt.method, "/test"+tt.query, bytes.NewBuffer(jsonBody))
				}
			} else {
				req = httptest.NewRequest(tt.method, "/test"+tt.query, nil)
			}

			w := httptest.NewRecorder()
			tt.handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestMonitoringHandlers tests resource monitoring endpoints
func TestMonitoringHandlers(t *testing.T) {
	server := newTestServer()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		method         string
		query          string
		expectedStatus int
	}{
		{
			name:           "get domain stats - missing name",
			handler:        server.handleGetDomainStats,
			method:         http.MethodGet,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "get domain stats - with name",
			handler:        server.handleGetDomainStats,
			method:         http.MethodGet,
			query:          "?name=test-vm",
			expectedStatus: http.StatusOK, // Returns empty stats
		},
		{
			name:           "get all domain stats",
			handler:        server.handleGetAllDomainStats,
			method:         http.MethodGet,
			expectedStatus: http.StatusOK, // Returns empty list
		},
		{
			name:           "get CPU stats - missing name",
			handler:        server.handleGetCPUStats,
			method:         http.MethodGet,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "get memory stats - wrong method",
			handler:        server.handleGetMemoryStats,
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test"+tt.query, nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestBatchHandlers tests batch operation endpoints
func TestBatchHandlers(t *testing.T) {
	server := newTestServer()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "batch start - invalid JSON",
			handler:        server.handleBatchStart,
			method:         http.MethodPost,
			body:           "not json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "batch start - valid structure",
			handler: server.handleBatchStart,
			method:  http.MethodPost,
			body: map[string]interface{}{
				"domains": []string{"vm1", "vm2"},
				"paused":  false,
			},
			expectedStatus: http.StatusOK, // Will return with failed results but correct structure
		},
		{
			name:    "batch stop - valid structure",
			handler: server.handleBatchStop,
			method:  http.MethodPost,
			body: map[string]interface{}{
				"domains": []string{"vm1"},
				"force":   false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "batch reboot - wrong method",
			handler:        server.handleBatchReboot,
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:    "batch snapshot - valid structure",
			handler: server.handleBatchSnapshot,
			method:  http.MethodPost,
			body: map[string]interface{}{
				"domains":     []string{"vm1"},
				"name_prefix": "backup",
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					req = httptest.NewRequest(tt.method, "/test", bytes.NewBufferString(str))
				} else {
					jsonBody, _ := json.Marshal(tt.body)
					req = httptest.NewRequest(tt.method, "/test", bytes.NewBuffer(jsonBody))
				}
			} else {
				req = httptest.NewRequest(tt.method, "/test", nil)
			}

			w := httptest.NewRecorder()
			tt.handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestCloneHandlers tests VM cloning and template endpoints
func TestCloneHandlers(t *testing.T) {
	server := newTestServer()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		method         string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "clone domain - invalid JSON",
			handler:        server.handleCloneDomain,
			method:         http.MethodPost,
			body:           "{invalid}",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "clone domain - valid structure",
			handler: server.handleCloneDomain,
			method:  http.MethodPost,
			body: map[string]interface{}{
				"source":  "source-vm",
				"target":  "target-vm",
				"new_mac": true,
			},
			expectedStatus: http.StatusInternalServerError, // Fails without virt-clone
		},
		{
			name:    "clone multiple - valid structure",
			handler: server.handleCloneMultipleDomains,
			method:  http.MethodPost,
			body: map[string]interface{}{
				"source":      "template",
				"name_prefix": "vm",
				"count":       3,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "clone multiple - count too high",
			handler: server.handleCloneMultipleDomains,
			method:  http.MethodPost,
			body: map[string]interface{}{
				"source":      "template",
				"name_prefix": "vm",
				"count":       200,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "list templates - GET only",
			handler:        server.handleListTemplates,
			method:         http.MethodGet,
			expectedStatus: http.StatusOK, // Returns empty list
		},
		{
			name:           "create template - wrong method",
			handler:        server.handleCreateTemplate,
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					req = httptest.NewRequest(tt.method, "/test", bytes.NewBufferString(str))
				} else {
					jsonBody, _ := json.Marshal(tt.body)
					req = httptest.NewRequest(tt.method, "/test", bytes.NewBuffer(jsonBody))
				}
			} else {
				req = httptest.NewRequest(tt.method, "/test", nil)
			}

			w := httptest.NewRecorder()
			tt.handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestParsingFunctions tests the parsing helper functions
func TestNetworkParsing(t *testing.T) {
	server := newTestServer()

	t.Run("parse network list", func(t *testing.T) {
		output := ` Name                 State      Autostart     Persistent
----------------------------------------------------------
 default              active     yes           yes
 test-network         inactive   no            yes
`
		networks := server.parseNetworkList(output)
		if len(networks) != 2 {
			t.Errorf("expected 2 networks, got %d", len(networks))
		}
		if networks[0].Name != "default" {
			t.Errorf("expected first network name 'default', got '%s'", networks[0].Name)
		}
		if !networks[0].Active {
			t.Error("expected first network to be active")
		}
		if networks[1].Active {
			t.Error("expected second network to be inactive")
		}
	})

	t.Run("parse network info", func(t *testing.T) {
		output := `Name:           default
UUID:           12345678-1234-1234-1234-123456789abc
Active:         yes
Persistent:     yes
Autostart:      yes
Bridge:         virbr0
`
		network := server.parseNetworkInfo("default", output)
		if network.Name != "default" {
			t.Errorf("expected name 'default', got '%s'", network.Name)
		}
		if network.UUID != "12345678-1234-1234-1234-123456789abc" {
			t.Errorf("expected UUID, got '%s'", network.UUID)
		}
		if !network.Active {
			t.Error("expected network to be active")
		}
		if !network.Persistent {
			t.Error("expected network to be persistent")
		}
		if !network.Autostart {
			t.Error("expected network to autostart")
		}
		if network.Bridge != "virbr0" {
			t.Errorf("expected bridge 'virbr0', got '%s'", network.Bridge)
		}
	})
}

// TestBatchOperationResult tests batch result building
func TestBatchOperationResult(t *testing.T) {
	server := newTestServer()

	results := []DomainOpResult{
		{Domain: "vm1", Success: true},
		{Domain: "vm2", Success: true},
		{Domain: "vm3", Success: false, Error: "failed"},
	}

	result := server.buildBatchResult("test", []string{"vm1", "vm2", "vm3"}, results,
		testTime, testTime.Add(testDuration))

	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}
	if result.Successful != 2 {
		t.Errorf("expected 2 successful, got %d", result.Successful)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
	if result.Operation != "test" {
		t.Errorf("expected operation 'test', got '%s'", result.Operation)
	}
}

// Test time variables for batch result testing
var (
	testTime     = mustParseTime("2026-01-19T12:00:00Z")
	testDuration = mustParseDuration("15s")
)

func mustParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func mustParseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}
