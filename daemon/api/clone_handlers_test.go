// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Clone Domain Handler Tests

func TestHandleCloneDomainMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/clone", nil)
	w := httptest.NewRecorder()

	server.handleCloneDomain(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCloneDomainInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/clone", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCloneDomain(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCloneDomainBasic(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"source":  "source-vm",
		"target":  "cloned-vm",
		"new_mac": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/clone", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCloneDomain(w, req)

	// Will fail with 500 if virt-clone fails (VM doesn't exist), which is expected in test
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCloneDomainWithFiles(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"source": "source-vm",
		"target": "cloned-vm",
		"files": []string{
			"/var/lib/libvirt/images/clone1.qcow2",
			"/var/lib/libvirt/images/clone2.qcow2",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/clone", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCloneDomain(w, req)

	// Will fail with 500 if virt-clone fails
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

func TestHandleCloneDomainWithPreserve(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"source":   "source-vm",
		"target":   "cloned-vm",
		"preserve": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/clone", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCloneDomain(w, req)

	// Will fail with 500 if virt-clone fails
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

// Clone Multiple Domains Handler Tests

func TestHandleCloneMultipleDomainsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/libvirt/clone/multiple", nil)
	w := httptest.NewRecorder()

	server.handleCloneMultipleDomains(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCloneMultipleDomainsInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/libvirt/clone/multiple", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCloneMultipleDomains(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCloneMultipleDomainsInvalidCount(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name  string
		count int
	}{
		{"zero count", 0},
		{"negative count", -1},
		{"count too large", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"source":      "source-vm",
				"name_prefix": "clone",
				"count":       tt.count,
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/libvirt/clone/multiple", bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleCloneMultipleDomains(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

func TestHandleCloneMultipleDomainsValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"source":      "source-vm",
		"name_prefix": "web",
		"count":       3,
		"start_index": 1,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/clone/multiple", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCloneMultipleDomains(w, req)

	// Should return 200 even if clones fail (results will show failures)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "completed" {
		t.Errorf("Expected status=completed, got %v", response["status"])
	}
	if response["total"] != float64(3) {
		t.Errorf("Expected total=3, got %v", response["total"])
	}
	if response["results"] == nil {
		t.Error("Expected results field in response")
	}
}

func TestHandleCloneMultipleDomainsDefaultStartIndex(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"source":      "source-vm",
		"name_prefix": "clone",
		"count":       2,
		// start_index omitted - should default to 1
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/libvirt/clone/multiple", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCloneMultipleDomains(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Create Template Handler Tests

func TestHandleCreateTemplateMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/templates/create", nil)
	w := httptest.NewRecorder()

	server.handleCreateTemplate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateTemplateInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/templates/create", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateTemplate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateTemplateBasic(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domain":      "source-vm",
		"name":        "my-template",
		"description": "Test template",
		"seal":        false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/templates/create", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateTemplate(w, req)

	// Will fail with 500 if virt-clone fails
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateTemplateWithSeal(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domain":      "source-vm",
		"name":        "sealed-template",
		"description": "Sealed template",
		"seal":        true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/templates/create", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateTemplate(w, req)

	// Will fail with 500 if virt-clone/virt-sysprep fails
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

func TestHandleCreateTemplateDefaultName(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domain": "source-vm",
		// name omitted - should default to "source-vm-template"
		"description": "Auto-named template",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/templates/create", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateTemplate(w, req)

	// Will fail with 500 if virt-clone fails
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
}

// List Templates Handler Tests

func TestHandleListTemplatesMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/templates", nil)
	w := httptest.NewRecorder()

	server.handleListTemplates(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListTemplates(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/templates", nil)
	w := httptest.NewRecorder()

	server.handleListTemplates(w, req)

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

		if _, ok := response["templates"]; !ok {
			t.Error("Expected templates field in response")
		}
		if _, ok := response["total"]; !ok {
			t.Error("Expected total field in response")
		}
	}
}

// Deploy From Template Handler Tests

func TestHandleDeployFromTemplateMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/templates/deploy", nil)
	w := httptest.NewRecorder()

	server.handleDeployFromTemplate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleDeployFromTemplateInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/templates/deploy", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleDeployFromTemplate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDeployFromTemplate(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"template":  "ubuntu-template",
		"name":      "new-vm-from-template",
		"memory":    2048,
		"vcpus":     2,
		"autostart": false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/templates/deploy", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleDeployFromTemplate(w, req)

	// Will fail with 500 if virt-clone fails (template doesn't exist), which is expected in test
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleExportTemplateMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/templates/export", nil)
	w := httptest.NewRecorder()

	server.handleExportTemplate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleExportTemplateInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/templates/export", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleExportTemplate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleExportTemplate(t *testing.T) {
	server := setupTestBasicServer(t)

	// handleExportTemplate's request struct uses `template`/`export_path`/
	// `compress` json keys. "test" is the domain libvirt's built-in test://
	// driver defines by default, so dumpxml/domblklist succeed against it.
	reqBody := map[string]interface{}{
		"template":    "test",
		"export_path": t.TempDir(),
		"compress":    false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/templates/export", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleExportTemplate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleExportTemplateMissingVMName(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"export_path": t.TempDir(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/templates/export", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleExportTemplate(w, req)

	// Handler validates template upfront and rejects the empty value with 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
