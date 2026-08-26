// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Backup storage directory (configurable via environment variable)
var BackupStorageDir = getBackupStorageDir()

func getBackupStorageDir() string {
	if dir := os.Getenv("HYPERSDK_BACKUP_DIR"); dir != "" {
		return dir
	}
	return "/var/lib/libvirt/backups"
}

// BackupInfo represents a backup
type BackupInfo struct {
	Name       string    `json:"name"`
	VMName     string    `json:"vm_name"`
	Path       string    `json:"path"`
	Type       string    `json:"type"` // full, incremental
	Size       int64     `json:"size"`
	CreatedAt  time.Time `json:"created_at"`
	Compressed bool      `json:"compressed"`
	Verified   bool      `json:"verified"`
}

// handleCreateBackup creates a full VM backup
func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMName      string `json:"vm_name"`
		BackupName  string `json:"backup_name,omitempty"` // Optional custom name
		Compress    bool   `json:"compress"`
		Description string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the VM name before it is ever used to build paths or shell out
	if !isValidVMName(req.VMName) {
		http.Error(w, "invalid vm_name: must contain only alphanumeric characters, hyphens, underscores, and dots", http.StatusBadRequest)
		return
	}

	// Generate backup name if not provided
	if req.BackupName == "" {
		req.BackupName = fmt.Sprintf("%s-%s", req.VMName, time.Now().Format("20060102-150405"))
	}

	// Sanitize the backup name to prevent path traversal
	sanitizedBackupName, err := sanitizeBackupName(req.BackupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.BackupName = sanitizedBackupName

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(BackupStorageDir, 0750); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create backup directory: %v", err)
		return
	}

	backupDir := filepath.Join(BackupStorageDir, req.BackupName)
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create backup subdirectory: %v", err)
		return
	}

	// Dump VM XML definition
	xmlPath := filepath.Join(backupDir, "domain.xml")
	// #nosec G204 -- req.VMName is validated by isValidVMName above before being passed to virsh
	dumpCmd := exec.Command("virsh", "dumpxml", req.VMName)
	xmlOutput, err := dumpCmd.Output()
	if err != nil {
		if rmErr := os.RemoveAll(backupDir); rmErr != nil { // Cleanup
			s.logger.Warn("failed to clean up backup directory", "dir", backupDir, "error", rmErr)
		}
		s.errorResponse(w, http.StatusInternalServerError, "failed to dump VM XML: %v", err)
		return
	}

	// domain.xml may contain sensitive data (e.g. VNC/SPICE passwords), so keep it private
	if err := os.WriteFile(xmlPath, xmlOutput, 0600); err != nil {
		if rmErr := os.RemoveAll(backupDir); rmErr != nil {
			s.logger.Warn("failed to clean up backup directory", "dir", backupDir, "error", rmErr)
		}
		s.errorResponse(w, http.StatusInternalServerError, "failed to write XML: %v", err)
		return
	}

	// Get disk paths
	// #nosec G204 -- req.VMName is validated by isValidVMName above before being passed to virsh
	domblklistCmd := exec.Command("virsh", "domblklist", req.VMName, "--details")
	domblklistOutput, err := domblklistCmd.Output()
	if err != nil {
		if rmErr := os.RemoveAll(backupDir); rmErr != nil {
			s.logger.Warn("failed to clean up backup directory", "dir", backupDir, "error", rmErr)
		}
		s.errorResponse(w, http.StatusInternalServerError, "failed to list disks: %v", err)
		return
	}

	// Parse disk list and copy disks
	disks := parseDiskList(string(domblklistOutput))
	backedUpDisks := []string{}

	for _, disk := range disks {
		if disk == "" || !fileExists(disk) {
			continue
		}

		diskName := filepath.Base(disk)
		backupDiskPath := filepath.Join(backupDir, diskName)

		// Copy disk using qemu-img convert. disk comes from `virsh domblklist` output for
		// the already-validated VM, and backupDiskPath is derived from the sanitized backup
		// directory, so neither is directly attacker-controlled.
		var convertCmd *exec.Cmd
		if req.Compress {
			// #nosec G204 -- disk/backupDiskPath derived from virsh output and sanitized backup dir, not raw request input
			convertCmd = exec.Command("qemu-img", "convert", "-O", "qcow2", "-c", disk, backupDiskPath)
		} else {
			// #nosec G204 -- disk/backupDiskPath derived from virsh output and sanitized backup dir, not raw request input
			convertCmd = exec.Command("qemu-img", "convert", "-O", "qcow2", disk, backupDiskPath)
		}

		if output, err := convertCmd.CombinedOutput(); err != nil {
			s.logger.Warn("failed to backup disk", "disk", disk, "error", string(output))
			continue
		}

		backedUpDisks = append(backedUpDisks, backupDiskPath)
	}

	// Write metadata
	metadata := map[string]interface{}{
		"vm_name":     req.VMName,
		"backup_name": req.BackupName,
		"created_at":  time.Now(),
		"type":        "full",
		"compressed":  req.Compress,
		"description": req.Description,
		"disks":       backedUpDisks,
	}

	metadataPath := filepath.Join(backupDir, "metadata.json")
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		s.logger.Warn("failed to marshal backup metadata", "path", metadataPath, "error", err)
	} else if err := os.WriteFile(metadataPath, metadataBytes, 0600); err != nil {
		s.logger.Warn("failed to write backup metadata", "path", metadataPath, "error", err)
	}

	// Calculate total backup size
	totalSize, _ := calculateDirectorySize(backupDir)

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":          "success",
		"message":         fmt.Sprintf("Backup created for %s", req.VMName),
		"vm_name":         req.VMName,
		"backup_name":     req.BackupName,
		"backup_path":     backupDir,
		"disks_backed_up": len(backedUpDisks),
		"total_size":      totalSize,
		"compressed":      req.Compress,
	})
}

// handleListBackups lists all available backups
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vmName := r.URL.Query().Get("vm_name") // Optional filter by VM

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(BackupStorageDir, 0750); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to access backup directory: %v", err)
		return
	}

	entries, err := os.ReadDir(BackupStorageDir)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to read backup directory: %v", err)
		return
	}

	backups := []BackupInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupDir := filepath.Join(BackupStorageDir, entry.Name())
		metadataPath := filepath.Join(backupDir, "metadata.json")

		// Read metadata. metadataPath is derived from BackupStorageDir joined with a name
		// returned by os.ReadDir, not from external input.
		var metadata map[string]interface{}
		// #nosec G304 -- metadataPath is built from a local directory listing, not user input
		if data, err := os.ReadFile(metadataPath); err == nil {
			if err := json.Unmarshal(data, &metadata); err != nil {
				s.logger.Warn("failed to parse backup metadata", "path", metadataPath, "error", err)
			}
		}

		// Get backup info
		info, _ := entry.Info()
		backupVMName := ""
		backupType := "full"
		compressed := false
		verified := false

		if metadata != nil {
			if vm, ok := metadata["vm_name"].(string); ok {
				backupVMName = vm
			}
			if t, ok := metadata["type"].(string); ok {
				backupType = t
			}
			if c, ok := metadata["compressed"].(bool); ok {
				compressed = c
			}
			if v, ok := metadata["verified"].(bool); ok {
				verified = v
			}
		}

		// Filter by VM name if specified
		if vmName != "" && backupVMName != vmName {
			continue
		}

		// Calculate total size
		totalSize, _ := calculateDirectorySize(backupDir)

		backups = append(backups, BackupInfo{
			Name:       entry.Name(),
			VMName:     backupVMName,
			Path:       backupDir,
			Type:       backupType,
			Size:       totalSize,
			CreatedAt:  info.ModTime(),
			Compressed: compressed,
			Verified:   verified,
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"backups":     backups,
		"total":       len(backups),
		"storage_dir": BackupStorageDir,
	})
}

// handleRestoreBackup restores a VM from backup
func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BackupName string `json:"backup_name"`
		NewVMName  string `json:"new_vm_name,omitempty"` // Optional: restore with different name
		Start      bool   `json:"start"`                 // Start VM after restore
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Sanitize the backup name to prevent path traversal
	sanitizedBackupName, err := sanitizeBackupName(req.BackupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.BackupName = sanitizedBackupName

	backupDir := filepath.Join(BackupStorageDir, req.BackupName)
	xmlPath := filepath.Join(backupDir, "domain.xml")

	// Check if backup exists
	if !fileExists(xmlPath) {
		s.errorResponse(w, http.StatusNotFound, "backup not found: %s", req.BackupName)
		return
	}

	// Read XML. xmlPath is derived from the sanitized backup name joined onto
	// BackupStorageDir, so it cannot escape the backup directory.
	// #nosec G304 -- xmlPath is built from a sanitized backup name confined to BackupStorageDir
	xmlBytes, err := os.ReadFile(xmlPath)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to read backup XML: %v", err)
		return
	}

	xml := string(xmlBytes)

	// Replace VM name if requested
	if req.NewVMName != "" {
		var err error
		xml, err = replaceVMNameInXML(xml, req.NewVMName)
		if err != nil {
			s.errorResponse(w, http.StatusBadRequest, "failed to replace VM name: %v", err)
			return
		}
	}

	// Define the VM
	defineCmd := exec.Command("virsh", "define", "/dev/stdin")
	defineCmd.Stdin = strings.NewReader(xml)
	defineOutput, err := defineCmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to define VM: %s", string(defineOutput))
		return
	}

	// Read metadata to get VM name. metadataPath is derived from the sanitized backup name.
	var metadata map[string]interface{}
	metadataPath := filepath.Join(backupDir, "metadata.json")
	// #nosec G304 -- metadataPath is built from a sanitized backup name confined to BackupStorageDir
	if data, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(data, &metadata); err != nil {
			s.logger.Warn("failed to parse backup metadata", "path", metadataPath, "error", err)
		}
	}

	vmName := req.NewVMName
	if vmName == "" && metadata != nil {
		if vm, ok := metadata["vm_name"].(string); ok {
			vmName = vm
		}
	}

	// Start VM if requested
	if req.Start && vmName != "" {
		if !isValidVMName(vmName) {
			s.logger.Warn("skipping VM start: invalid vm name", "vm_name", vmName)
		} else {
			// #nosec G204 -- vmName is validated by isValidVMName above before being passed to virsh
			startCmd := exec.Command("virsh", "start", vmName)
			if output, err := startCmd.CombinedOutput(); err != nil {
				// Ignore error if already running, but log for visibility
				s.logger.Warn("failed to start VM after restore", "vm_name", vmName, "error", err, "output", string(output))
			}
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("VM restored from backup %s", req.BackupName),
		"backup_name": req.BackupName,
		"vm_name":     vmName,
		"started":     req.Start,
	})
}

// handleVerifyBackup verifies backup integrity
func (s *Server) handleVerifyBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BackupName string `json:"backup_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Sanitize the backup name to prevent path traversal
	sanitizedBackupName, err := sanitizeBackupName(req.BackupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.BackupName = sanitizedBackupName

	backupDir := filepath.Join(BackupStorageDir, req.BackupName)
	xmlPath := filepath.Join(backupDir, "domain.xml")

	if !fileExists(xmlPath) {
		s.errorResponse(w, http.StatusNotFound, "backup not found: %s", req.BackupName)
		return
	}

	// Verify XML is valid. xmlPath is derived from the sanitized backup name.
	// #nosec G204 -- xmlPath is built from a sanitized backup name confined to BackupStorageDir
	validateCmd := exec.Command("virsh", "define", "--validate", xmlPath)
	if output, err := validateCmd.CombinedOutput(); err != nil {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":      "failed",
			"backup_name": req.BackupName,
			"valid":       false,
			"error":       string(output),
		})
		return
	}

	// Check disk files with qemu-img
	diskErrors := []string{}
	disksChecked := 0

	walkErr := filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check qcow2 files
		if strings.HasSuffix(strings.ToLower(path), ".qcow2") {
			disksChecked++
			// #nosec G204 -- path is enumerated by filepath.Walk over the validated backup directory, not user input
			checkCmd := exec.Command("qemu-img", "check", path)
			if output, err := checkCmd.CombinedOutput(); err != nil {
				diskErrors = append(diskErrors, fmt.Sprintf("%s: %s", filepath.Base(path), string(output)))
			}
		}

		return nil
	})
	if walkErr != nil {
		s.logger.Warn("error walking backup directory", "dir", backupDir, "error", walkErr)
	}

	// Update metadata with verification result. metadataPath is derived from the sanitized backup name.
	metadataPath := filepath.Join(backupDir, "metadata.json")
	var metadata map[string]interface{}
	// #nosec G304 -- metadataPath is built from a sanitized backup name confined to BackupStorageDir
	if data, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(data, &metadata); err != nil {
			s.logger.Warn("failed to parse backup metadata", "path", metadataPath, "error", err)
			metadata = map[string]interface{}{}
		}
		metadata["verified"] = len(diskErrors) == 0
		metadata["verified_at"] = time.Now()
		if updatedBytes, err := json.MarshalIndent(metadata, "", "  "); err == nil {
			if err := os.WriteFile(metadataPath, updatedBytes, 0600); err != nil {
				s.logger.Warn("failed to update backup metadata", "path", metadataPath, "error", err)
			}
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "success",
		"backup_name":   req.BackupName,
		"valid":         len(diskErrors) == 0,
		"disks_checked": disksChecked,
		"errors":        diskErrors,
	})
}

// handleDeleteBackup deletes a backup
func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BackupName string `json:"backup_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Prevent path traversal
	backupName := filepath.Base(req.BackupName)
	backupDir := filepath.Join(BackupStorageDir, backupName)

	if !fileExists(backupDir) {
		s.errorResponse(w, http.StatusNotFound, "backup not found: %s", backupName)
		return
	}

	// Delete backup directory
	if err := os.RemoveAll(backupDir); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to delete backup: %v", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Backup %s deleted", backupName),
		"backup_name": backupName,
	})
}

// Helper functions
func parseDiskList(output string) []string {
	lines := strings.Split(output, "\n")
	disks := []string{}

	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			// Fourth field is the source path
			disks = append(disks, fields[3])
		}
	}

	return disks
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// checkDiskSpace verifies sufficient disk space is available
func checkDiskSpace(path string, requiredBytes int64) error {
	available, err := getAvailableDiskSpace(path)
	if err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	// Require at least 10% more than needed for safety
	requiredWithBuffer := int64(float64(requiredBytes) * 1.1)

	if available < requiredWithBuffer {
		return fmt.Errorf("insufficient disk space: need %d GB, have %d GB available",
			requiredWithBuffer/(1024*1024*1024),
			available/(1024*1024*1024))
	}
	return nil
}

// calculateDirectorySize calculates the total size of a directory
func calculateDirectorySize(dirPath string) (int64, error) {
	var totalSize int64
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return totalSize, err
}

// isValidVMName validates that a VM name contains only safe characters
func isValidVMName(name string) bool {
	if len(name) == 0 || len(name) > 255 {
		return false
	}
	// Only allow alphanumeric, hyphens, underscores, and dots
	match, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, name)
	return match
}

// sanitizeBackupName validates and sanitizes a backup name so it cannot be
// used to escape BackupStorageDir (e.g. via "..", path separators, or an
// empty/dot-only value).
func sanitizeBackupName(name string) (string, error) {
	clean := filepath.Base(filepath.Clean(name))
	if clean == "" || clean == "." || clean == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid backup name: %q", name)
	}
	if !isValidVMName(clean) {
		return "", fmt.Errorf("invalid backup name: must contain only alphanumeric characters, hyphens, underscores, and dots")
	}
	return clean, nil
}

// replaceVMNameInXML safely replaces the VM name in libvirt XML using proper XML parsing
func replaceVMNameInXML(xmlStr, newName string) (string, error) {
	// Validate the new name first
	if !isValidVMName(newName) {
		return "", fmt.Errorf("invalid VM name: must contain only alphanumeric characters, hyphens, underscores, and dots")
	}

	// Parse XML into generic structure
	type Domain struct {
		XMLName xml.Name `xml:"domain"`
		Name    string   `xml:"name"`
		Content []byte   `xml:",innerxml"`
	}

	var domain Domain
	if err := xml.Unmarshal([]byte(xmlStr), &domain); err != nil {
		return "", fmt.Errorf("failed to parse XML: %w", err)
	}

	// Replace the name
	domain.Name = newName

	// Marshal back to XML with proper formatting
	output, err := xml.MarshalIndent(domain, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to generate XML: %w", err)
	}

	// Add XML declaration
	result := xml.Header + string(output)
	return result, nil
}
