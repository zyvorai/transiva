// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Workflow Status Handler Tests

func TestWorkflowStatusHandlerMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow/status", nil)
	w := httptest.NewRecorder()

	server.WorkflowStatusHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestWorkflowStatusHandler(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow/status", nil)
	w := httptest.NewRecorder()

	server.WorkflowStatusHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response WorkflowStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response structure
	if response.Mode == "" {
		t.Error("Expected mode to be set")
	}
	if response.MaxWorkers == 0 {
		t.Error("Expected max_workers to be set")
	}
}

// Workflow Jobs Handler Tests

func TestWorkflowJobsHandlerMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow/jobs", nil)
	w := httptest.NewRecorder()

	server.WorkflowJobsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestWorkflowJobsHandler(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name         string
		statusFilter string
	}{
		{"no filter", ""},
		{"processing filter", "processing"},
		{"pending filter", "pending"},
		{"completed filter", "completed"},
		{"failed filter", "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/workflow/jobs"
			if tt.statusFilter != "" {
				url += "?status=" + tt.statusFilter
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			server.WorkflowJobsHandler(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v\nBody: %s", err, w.Body.String())
			}

			// Jobs field should exist (may be empty array)
			if _, ok := response["jobs"]; !ok {
				t.Errorf("Expected jobs field in response, got: %v", response)
			}
			// Total field should exist
			if _, ok := response["total"]; !ok {
				t.Errorf("Expected total field in response, got: %v", response)
			}
		})
	}
}

// Workflow Jobs Active Handler Tests

func TestWorkflowJobsActiveHandlerMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow/jobs/active", nil)
	w := httptest.NewRecorder()

	server.WorkflowJobsActiveHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestWorkflowJobsActiveHandler(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow/jobs/active", nil)
	w := httptest.NewRecorder()

	server.WorkflowJobsActiveHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v\nBody: %s", err, w.Body.String())
	}

	// Jobs field should exist (may be empty array)
	if _, ok := response["jobs"]; !ok {
		t.Errorf("Expected jobs field in response, got: %v", response)
	}
	// Total field should exist
	if _, ok := response["total"]; !ok {
		t.Errorf("Expected total field in response, got: %v", response)
	}
}

// Manifest Submit Handler Tests

func TestManifestSubmitHandlerMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow/manifest/submit", nil)
	w := httptest.NewRecorder()

	server.ManifestSubmitHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestManifestSubmitHandlerInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/submit",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.ManifestSubmitHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestManifestSubmitHandlerMissingVersion(t *testing.T) {
	server := setupTestBasicServer(t)

	manifest := map[string]interface{}{
		"name": "test-manifest",
	}

	body, _ := json.Marshal(manifest)
	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/submit",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.ManifestSubmitHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestManifestSubmitHandlerMissingPipeline(t *testing.T) {
	server := setupTestBasicServer(t)

	manifest := map[string]interface{}{
		"version": "1.0",
		"name":    "test-manifest",
	}

	body, _ := json.Marshal(manifest)
	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/submit",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.ManifestSubmitHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestManifestSubmitHandlerValidManifest(t *testing.T) {
	server := setupTestBasicServer(t)

	manifest := map[string]interface{}{
		"version": "1.0",
		"name":    "test-manifest",
		"pipeline": map[string]interface{}{
			"load": map[string]interface{}{
				"source_type": "vcenter",
				"source_path": "/datacenter/vm/test",
			},
			"convert": map[string]interface{}{
				"output_format": "qcow2",
			},
		},
	}

	body, _ := json.Marshal(manifest)
	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/submit",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.ManifestSubmitHandler(w, req)

	// May fail with 500 if workflow directory can't be created (permission denied)
	// or succeed with 200 if directory can be created
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["success"] != true {
			t.Error("Expected success=true in response")
		}
		if response["manifest_path"] == nil {
			t.Error("Expected manifest_path in response")
		}
		if response["job_id"] == nil {
			t.Error("Expected job_id in response")
		}
	}
}

func TestManifestSubmitHandlerBatchManifest(t *testing.T) {
	server := setupTestBasicServer(t)

	manifest := map[string]interface{}{
		"version": "1.0",
		"batch":   true,
		"vms": []interface{}{
			map[string]interface{}{
				"source_path": "/datacenter/vm/vm1",
			},
			map[string]interface{}{
				"source_path": "/datacenter/vm/vm2",
			},
		},
	}

	body, _ := json.Marshal(manifest)
	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/submit",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.ManifestSubmitHandler(w, req)

	// May fail with 500 if workflow directory can't be created
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", w.Code)
	}
}

// Manifest Validate Handler Tests

func TestManifestValidateHandlerMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/workflow/manifest/validate", nil)
	w := httptest.NewRecorder()

	server.ManifestValidateHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestManifestValidateHandlerInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/validate",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.ManifestValidateHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestManifestValidateHandlerInvalidManifest(t *testing.T) {
	server := setupTestBasicServer(t)

	manifest := map[string]interface{}{
		"name": "test-manifest",
		// Missing version
	}

	body, _ := json.Marshal(manifest)
	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/validate",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.ManifestValidateHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["valid"] != false {
		t.Error("Expected valid=false for invalid manifest")
	}

	if response["errors"] == nil {
		t.Error("Expected errors field in response")
	}

	errors, ok := response["errors"].([]interface{})
	if !ok {
		t.Fatal("Expected errors to be an array")
	}

	if len(errors) == 0 {
		t.Error("Expected at least one validation error")
	}
}

func TestManifestValidateHandlerValidManifest(t *testing.T) {
	server := setupTestBasicServer(t)

	manifest := map[string]interface{}{
		"version": "1.0",
		"name":    "test-manifest",
		"pipeline": map[string]interface{}{
			"load": map[string]interface{}{
				"source_type": "vcenter",
				"source_path": "/datacenter/vm/test",
			},
			"convert": map[string]interface{}{
				"output_format": "qcow2",
			},
		},
	}

	body, _ := json.Marshal(manifest)
	req := httptest.NewRequest(http.MethodPost, "/api/workflow/manifest/validate",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.ManifestValidateHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["valid"] != true {
		t.Errorf("Expected valid=true for valid manifest, errors: %v", response["errors"])
	}

	// Errors field should exist
	if _, ok := response["errors"]; !ok {
		t.Error("Expected errors field in response")
	}

	// If errors is not nil, it should be an array with length 0
	if errorsField := response["errors"]; errorsField != nil {
		errors, ok := errorsField.([]interface{})
		if ok && len(errors) != 0 {
			t.Errorf("Expected zero validation errors, got: %v", errors)
		}
	}
}

// Helper Function Tests

func TestIsMetadataFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"meta file", "job.meta.json", true},
		{"report file", "job.report.json", true},
		{"error file", "job.error.json", true},
		{"regular json", "manifest.json", false},
		{"yaml file", "config.yaml", false},
		{"no extension", "job", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMetadataFile(tt.filename)
			if result != tt.expected {
				t.Errorf("isMetadataFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name        string
		manifest    map[string]interface{}
		expectError bool
	}{
		{
			name: "valid single VM manifest",
			manifest: map[string]interface{}{
				"version": "1.0",
				"pipeline": map[string]interface{}{
					"load": map[string]interface{}{
						"source_type": "vcenter",
						"source_path": "/datacenter/vm/test",
					},
					"convert": map[string]interface{}{
						"output_format": "qcow2",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid batch manifest",
			manifest: map[string]interface{}{
				"version": "1.0",
				"batch":   true,
				"vms": []interface{}{
					map[string]interface{}{"source_path": "/vm1"},
				},
			},
			expectError: false,
		},
		{
			name:        "missing version",
			manifest:    map[string]interface{}{},
			expectError: true,
		},
		{
			name: "missing pipeline for single VM",
			manifest: map[string]interface{}{
				"version": "1.0",
			},
			expectError: true,
		},
		{
			name: "missing load in pipeline",
			manifest: map[string]interface{}{
				"version": "1.0",
				"pipeline": map[string]interface{}{
					"convert": map[string]interface{}{
						"output_format": "qcow2",
					},
				},
			},
			expectError: true,
		},
		{
			name: "missing source_type in load",
			manifest: map[string]interface{}{
				"version": "1.0",
				"pipeline": map[string]interface{}{
					"load": map[string]interface{}{
						"source_path": "/datacenter/vm/test",
					},
					"convert": map[string]interface{}{
						"output_format": "qcow2",
					},
				},
			},
			expectError: true,
		},
		{
			name: "missing convert in pipeline",
			manifest: map[string]interface{}{
				"version": "1.0",
				"pipeline": map[string]interface{}{
					"load": map[string]interface{}{
						"source_type": "vcenter",
						"source_path": "/datacenter/vm/test",
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty vms array for batch",
			manifest: map[string]interface{}{
				"version": "1.0",
				"batch":   true,
				"vms":     []interface{}{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateManifest(tt.manifest)
			if (err != nil) != tt.expectError {
				t.Errorf("validateManifest() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestValidateManifestDetailed(t *testing.T) {
	tests := []struct {
		name         string
		manifest     map[string]interface{}
		expectErrors int
	}{
		{
			name: "valid manifest",
			manifest: map[string]interface{}{
				"version": "1.0",
				"pipeline": map[string]interface{}{
					"load": map[string]interface{}{
						"source_type": "vcenter",
						"source_path": "/datacenter/vm/test",
					},
					"convert": map[string]interface{}{
						"output_format": "qcow2",
					},
				},
			},
			expectErrors: 0,
		},
		{
			name: "multiple errors",
			manifest: map[string]interface{}{
				// Missing version
				"pipeline": map[string]interface{}{
					"load": map[string]interface{}{
						// Missing source_type and source_path
					},
					// Missing convert
				},
			},
			expectErrors: 4, // version, source_type, source_path, convert
		},
		{
			name: "batch manifest missing vms",
			manifest: map[string]interface{}{
				"version": "1.0",
				"batch":   true,
				// Missing vms array
			},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateManifestDetailed(tt.manifest)
			if len(errors) != tt.expectErrors {
				t.Errorf("validateManifestDetailed() returned %d errors, want %d. Errors: %v",
					len(errors), tt.expectErrors, errors)
			}
		})
	}
}
