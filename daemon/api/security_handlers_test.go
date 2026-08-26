// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Get Encryption Config Handler Tests

func TestHandleGetEncryptionConfigMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/security/encryption", nil)
	w := httptest.NewRecorder()

	server.handleGetEncryptionConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetEncryptionConfig(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/security/encryption", nil)
	w := httptest.NewRecorder()

	server.handleGetEncryptionConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response EncryptionConfig
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.AtRest.Algorithm != "AES-256" {
		t.Errorf("Expected algorithm=AES-256, got %s", response.AtRest.Algorithm)
	}
	if !response.AtRest.Enabled {
		t.Error("Expected at-rest encryption to be enabled")
	}
	if !response.InTransit.RequireTLS13 {
		t.Error("Expected TLS 1.3 to be required")
	}
	if !response.InTransit.VerifySSLCerts {
		t.Error("Expected SSL cert verification to be enabled")
	}
	if response.KeyManagement.Storage != "local" {
		t.Errorf("Expected storage=local, got %s", response.KeyManagement.Storage)
	}
}

// Update Encryption Config Handler Tests

func TestHandleUpdateEncryptionConfigMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/security/encryption", nil)
	w := httptest.NewRecorder()

	server.handleUpdateEncryptionConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleUpdateEncryptionConfigInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPut, "/security/encryption",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleUpdateEncryptionConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleUpdateEncryptionConfigValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := EncryptionConfig{
		AtRest: EncryptionAtRest{
			Enabled:   true,
			Algorithm: "AES-256",
		},
		InTransit: EncryptionInTransit{
			RequireTLS13:   true,
			VerifySSLCerts: true,
		},
		KeyManagement: KeyManagement{
			Storage:  "vault",
			VaultURL: "https://vault.example.com",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/security/encryption",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleUpdateEncryptionConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status=success, got %s", response["status"])
	}
}

func TestHandleUpdateEncryptionConfigDifferentAlgorithms(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []string{"AES-256", "AES-128", "ChaCha20-Poly1305"}

	for _, algorithm := range tests {
		t.Run(algorithm, func(t *testing.T) {
			reqBody := EncryptionConfig{
				AtRest: EncryptionAtRest{
					Enabled:   true,
					Algorithm: algorithm,
				},
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPut, "/security/encryption",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleUpdateEncryptionConfig(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

// List Compliance Frameworks Handler Tests

func TestHandleListComplianceFrameworksMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/security/compliance", nil)
	w := httptest.NewRecorder()

	server.handleListComplianceFrameworks(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListComplianceFrameworks(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/security/compliance", nil)
	w := httptest.NewRecorder()

	server.handleListComplianceFrameworks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["frameworks"]; !ok {
		t.Error("Expected frameworks field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 3 {
		t.Errorf("Expected total=3, got %v", total)
	}

	// Verify framework structure
	frameworks := response["frameworks"].([]interface{})
	if len(frameworks) != 3 {
		t.Errorf("Expected 3 frameworks, got %d", len(frameworks))
	}

	// Check first framework
	framework := frameworks[0].(map[string]interface{})
	if framework["name"] != "GDPR" {
		t.Errorf("Expected first framework name=GDPR, got %v", framework["name"])
	}
	if framework["status"] != "compliant" {
		t.Errorf("Expected status=compliant, got %v", framework["status"])
	}
}

// Get Audit Logs Handler Tests

func TestHandleGetAuditLogsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/security/audit", nil)
	w := httptest.NewRecorder()

	server.handleGetAuditLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetAuditLogs(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/security/audit", nil)
	w := httptest.NewRecorder()

	server.handleGetAuditLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["logs"]; !ok {
		t.Error("Expected logs field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 3 {
		t.Errorf("Expected total=3, got %v", total)
	}

	// Verify log entry structure
	logs := response["logs"].([]interface{})
	if len(logs) != 3 {
		t.Errorf("Expected 3 log entries, got %d", len(logs))
	}

	// Check first log entry
	log := logs[0].(map[string]interface{})
	if log["user"] != "admin" {
		t.Errorf("Expected user=admin, got %v", log["user"])
	}
	if log["action"] != "EXPORT" {
		t.Errorf("Expected action=EXPORT, got %v", log["action"])
	}
	if log["resource"] != "web-server-01" {
		t.Errorf("Expected resource=web-server-01, got %v", log["resource"])
	}
}

// Export Audit Logs Handler Tests

func TestHandleExportAuditLogsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/security/audit/export", nil)
	w := httptest.NewRecorder()

	server.handleExportAuditLogs(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleExportAuditLogs(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/security/audit/export", nil)
	w := httptest.NewRecorder()

	server.handleExportAuditLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("Expected Content-Type=text/csv, got %s", contentType)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "audit_logs.csv") {
		t.Errorf("Expected Content-Disposition to contain audit_logs.csv, got %s", contentDisposition)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Timestamp,User,Action,Resource,IP Address") {
		t.Error("Expected CSV header in response")
	}
}

// Migration Wizard Handler Tests

func TestHandleMigrationWizardMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/migration/wizard", nil)
	w := httptest.NewRecorder()

	server.handleMigrationWizard(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleMigrationWizardGet(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/migration/wizard", nil)
	w := httptest.NewRecorder()

	server.handleMigrationWizard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response MigrationWizardState
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Step != 1 {
		t.Errorf("Expected step=1, got %d", response.Step)
	}
	if response.Source.Type != "vmware" {
		t.Errorf("Expected source type=vmware, got %s", response.Source.Type)
	}
}

func TestHandleMigrationWizardPostInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/migration/wizard",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleMigrationWizard(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleMigrationWizardPostValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := MigrationWizardState{
		Step: 2,
		Source: MigrationSource{
			Type:   "vmware",
			Server: "vcenter.example.com",
		},
		Destination: MigrationDestination{
			Type:     "kvm",
			Location: "hypervisor01",
		},
		VMs: []string{"web-vm-01", "db-vm-01"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/migration/wizard",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleMigrationWizard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response MigrationWizardState
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Step != 2 {
		t.Errorf("Expected step=2, got %d", response.Step)
	}
	if len(response.VMs) != 2 {
		t.Errorf("Expected 2 VMs, got %d", len(response.VMs))
	}
}

func TestHandleMigrationWizardDifferentSources(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name       string
		sourceType string
	}{
		{"VMware", "vmware"},
		{"Hyper-V", "hyperv"},
		{"KVM", "kvm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := MigrationWizardState{
				Step: 1,
				Source: MigrationSource{
					Type: tt.sourceType,
				},
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/migration/wizard",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleMigrationWizard(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

// Compatibility Check Handler Tests

func TestHandleCompatibilityCheckMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/migration/compatibility", nil)
	w := httptest.NewRecorder()

	server.handleCompatibilityCheck(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCompatibilityCheckInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/migration/compatibility",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCompatibilityCheck(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCompatibilityCheckValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]string{
		"vm_id":    "vm-123",
		"platform": "kvm",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/migration/compatibility",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCompatibilityCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["checks"]; !ok {
		t.Error("Expected checks field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 4 {
		t.Errorf("Expected total=4, got %v", total)
	}

	// Verify check structure
	checks := response["checks"].([]interface{})
	if len(checks) != 4 {
		t.Errorf("Expected 4 checks, got %d", len(checks))
	}

	// Verify check statuses
	statusCounts := make(map[string]int)
	for _, check := range checks {
		c := check.(map[string]interface{})
		status := c["status"].(string)
		statusCounts[status]++
	}

	if statusCounts["pass"] != 3 {
		t.Errorf("Expected 3 'pass' checks, got %d", statusCounts["pass"])
	}
	if statusCounts["warning"] != 1 {
		t.Errorf("Expected 1 'warning' check, got %d", statusCounts["warning"])
	}
}

// Rollback Handler Tests

func TestHandleRollbackMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/vm/rollback", nil)
	w := httptest.NewRecorder()

	server.handleRollback(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleRollbackInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/vm/rollback",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleRollback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRollbackValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]string{
		"vm_id":     "vm-123",
		"backup_id": "backup-456",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/vm/rollback",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleRollback(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status=success, got %v", response["status"])
	}
	if response["vm_id"] != "vm-123" {
		t.Errorf("Expected vm_id=vm-123, got %v", response["vm_id"])
	}
}
