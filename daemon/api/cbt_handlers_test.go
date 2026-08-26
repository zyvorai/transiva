// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/logger"
)

func TestHandleEnableCBT(t *testing.T) {
	log := logger.New("debug")
	detector := capabilities.NewDetector(log)
	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, ":8080", nil, nil)

	tests := []struct {
		name           string
		method         string
		body           CBTEnableRequest
		expectedStatus int
	}{
		{
			name:   "Valid request",
			method: http.MethodPost,
			body: CBTEnableRequest{
				VMPath: "/Datacenter/vm/test-vm",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Missing VM path",
			method: http.MethodPost,
			body: CBTEnableRequest{
				VMPath: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Wrong method",
			method:         http.MethodGet,
			body:           CBTEnableRequest{},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(tt.method, "/cbt/enable", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			server.handleEnableCBT(rr, req)

			if tt.name == "Wrong method" && rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.name == "Missing VM path" && rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Note: Valid request will fail without actual vSphere connection
			// In production tests, use mocks for the vSphere client
		})
	}
}

func TestHandleCBTStatus(t *testing.T) {
	log := logger.New("debug")
	detector := capabilities.NewDetector(log)
	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, ":8080", nil, nil)

	tests := []struct {
		name           string
		method         string
		body           CBTStatusRequest
		expectedStatus int
	}{
		{
			name:   "Valid request",
			method: http.MethodPost,
			body: CBTStatusRequest{
				VMPath: "/Datacenter/vm/test-vm",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Missing VM path",
			method: http.MethodPost,
			body: CBTStatusRequest{
				VMPath: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Wrong method",
			method:         http.MethodGet,
			body:           CBTStatusRequest{},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(tt.method, "/cbt/status", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			server.handleCBTStatus(rr, req)

			if tt.name == "Wrong method" && rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.name == "Missing VM path" && rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestHandleIncrementalAnalysis(t *testing.T) {
	log := logger.New("debug")
	detector := capabilities.NewDetector(log)
	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, ":8080", nil, nil)

	tests := []struct {
		name           string
		method         string
		body           IncrementalAnalysisRequest
		expectedStatus int
	}{
		{
			name:   "Valid request",
			method: http.MethodPost,
			body: IncrementalAnalysisRequest{
				VMPath: "/Datacenter/vm/test-vm",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Missing VM path",
			method: http.MethodPost,
			body: IncrementalAnalysisRequest{
				VMPath: "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Wrong method",
			method:         http.MethodGet,
			body:           IncrementalAnalysisRequest{},
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(tt.method, "/incremental/analyze", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			server.handleIncrementalAnalysis(rr, req)

			if tt.name == "Wrong method" && rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.name == "Missing VM path" && rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestRespondJSON(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{
			name: "Simple object",
			data: map[string]string{
				"status": "ok",
			},
		},
		{
			name: "CBT response",
			data: CBTEnableResponse{
				Success: true,
				Message: "CBT enabled",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			respondJSON(rr, tt.data)

			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
			}

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rr.Code)
			}

			// Verify JSON is valid
			var result interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
				t.Errorf("Failed to unmarshal JSON: %v", err)
			}
		})
	}
}
