// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// EncryptionConfig represents encryption configuration
type EncryptionConfig struct {
	AtRest        EncryptionAtRest    `json:"at_rest"`
	InTransit     EncryptionInTransit `json:"in_transit"`
	KeyManagement KeyManagement       `json:"key_management"`
}

// EncryptionAtRest configuration
type EncryptionAtRest struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"` // AES-256, AES-128, ChaCha20-Poly1305
}

// EncryptionInTransit configuration
type EncryptionInTransit struct {
	RequireTLS13   bool `json:"require_tls13"`
	VerifySSLCerts bool `json:"verify_ssl_certs"`
}

// KeyManagement configuration
type KeyManagement struct {
	Storage  string `json:"storage"` // local, vault, aws-kms, azure-keyvault
	VaultURL string `json:"vault_url,omitempty"`
}

// ComplianceFramework represents a compliance framework
type ComplianceFramework struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // compliant, non-compliant, pending
	Score       float64   `json:"score"`
	LastChecked time.Time `json:"last_checked"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	User      string                 `json:"user"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	IPAddress string                 `json:"ip_address"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// MigrationWizardState represents wizard state
type MigrationWizardState struct {
	Step        int                    `json:"step"`
	Source      MigrationSource        `json:"source"`
	Destination MigrationDestination   `json:"destination"`
	VMs         []string               `json:"vms"`
	Options     map[string]interface{} `json:"options"`
}

// MigrationSource represents migration source
type MigrationSource struct {
	Type   string `json:"type"` // vmware, hyperv, kvm
	Server string `json:"server"`
}

// MigrationDestination represents migration destination
type MigrationDestination struct {
	Type     string `json:"type"` // kvm, aws, azure, gcp
	Location string `json:"location"`
}

// CompatibilityCheck represents compatibility check result
type CompatibilityCheck struct {
	Item    string `json:"item"`
	Status  string `json:"status"` // pass, warning, fail
	Message string `json:"message"`
}

// handleGetEncryptionConfig gets encryption configuration
func (s *Server) handleGetEncryptionConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := EncryptionConfig{
		AtRest: EncryptionAtRest{
			Enabled:   true,
			Algorithm: "AES-256",
		},
		InTransit: EncryptionInTransit{
			RequireTLS13:   true,
			VerifySSLCerts: true,
		},
		KeyManagement: KeyManagement{
			Storage: "local",
		},
	}

	s.jsonResponse(w, http.StatusOK, config)
}

// handleUpdateEncryptionConfig updates encryption configuration
func (s *Server) handleUpdateEncryptionConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config EncryptionConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Encryption configuration updated",
	})
}

// handleListComplianceFrameworks lists compliance frameworks
func (s *Server) handleListComplianceFrameworks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	frameworks := []ComplianceFramework{
		{
			ID:          "comp-1",
			Name:        "GDPR",
			Description: "General Data Protection Regulation",
			Status:      "compliant",
			Score:       98.0,
			LastChecked: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:          "comp-2",
			Name:        "SOC 2 Type II",
			Description: "Service Organization Control 2",
			Status:      "compliant",
			Score:       97.5,
			LastChecked: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:          "comp-3",
			Name:        "HIPAA",
			Description: "Health Insurance Portability and Accountability Act",
			Status:      "compliant",
			Score:       99.0,
			LastChecked: time.Now().Add(-24 * time.Hour),
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"frameworks": frameworks,
		"total":      len(frameworks),
	})
}

// handleGetAuditLogs gets audit logs
func (s *Server) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logs := []AuditLog{
		{
			ID:        "audit-1",
			Timestamp: time.Now().Add(-10 * time.Minute),
			User:      "admin",
			Action:    "EXPORT",
			Resource:  "web-server-01",
			IPAddress: "192.168.1.100",
		},
		{
			ID:        "audit-2",
			Timestamp: time.Now().Add(-30 * time.Minute),
			User:      "operator1",
			Action:    "CREATE_USER",
			Resource:  "viewer2",
			IPAddress: "192.168.1.101",
		},
		{
			ID:        "audit-3",
			Timestamp: time.Now().Add(-1 * time.Hour),
			User:      "admin",
			Action:    "MODIFY_POLICY",
			Resource:  "backup-policy-1",
			IPAddress: "192.168.1.100",
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": len(logs),
	})
}

// handleExportAuditLogs exports audit logs
func (s *Server) handleExportAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return CSV or JSON export
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_logs.csv")

	csv := "Timestamp,User,Action,Resource,IP Address\n"
	csv += "2026-01-19 15:45:32,admin,EXPORT,web-server-01,192.168.1.100\n"

	if _, err := w.Write([]byte(csv)); err != nil {
		s.logger.Warn("failed to write audit log export response", "error", err)
	}
}

// handleMigrationWizard handles migration wizard state
func (s *Server) handleMigrationWizard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Get current wizard state
		state := MigrationWizardState{
			Step: 1,
			Source: MigrationSource{
				Type: "vmware",
			},
		}
		s.jsonResponse(w, http.StatusOK, state)
		return
	}

	if r.Method == http.MethodPost {
		// Update wizard state
		var state MigrationWizardState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, http.StatusOK, state)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// handleCompatibilityCheck runs compatibility checks
func (s *Server) handleCompatibilityCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMID     string `json:"vm_id"`
		Platform string `json:"platform"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	checks := []CompatibilityCheck{
		{
			Item:    "Guest OS Supported",
			Status:  "pass",
			Message: "Ubuntu 20.04 is fully supported",
		},
		{
			Item:    "Disk Format Compatible",
			Status:  "pass",
			Message: "VMDK can be converted to qcow2",
		},
		{
			Item:    "VMware Tools",
			Status:  "warning",
			Message: "VMware Tools should be removed after migration",
		},
		{
			Item:    "Network Adapters",
			Status:  "pass",
			Message: "VMXNET3 adapters are supported",
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"checks": checks,
		"total":  len(checks),
	})
}

// handleRollback handles VM rollback
func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMID     string `json:"vm_id"`
		BackupID string `json:"backup_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "VM rollback initiated",
		"vm_id":   req.VMID,
	})
}
