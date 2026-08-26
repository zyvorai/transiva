// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// vmNamePattern restricts VM/domain/template names accepted from API requests
// to safe libvirt-style identifiers. This prevents flag/argument injection
// into the virt-clone/virsh subprocess commands that are built from them.
var vmNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// validateVMName ensures name is a safe identifier before it is used to build
// subprocess arguments.
func validateVMName(name, field string) error {
	if !vmNamePattern.MatchString(name) {
		return fmt.Errorf("%s %q is invalid: must match %s", field, name, vmNamePattern.String())
	}
	return nil
}

// validateFilePath ensures path is a safe, absolute filesystem path before it
// is used to build subprocess arguments, and returns the cleaned path.
func validateFilePath(path, field string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s must not be empty", field)
	}
	if strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("%s contains invalid characters", field)
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("%s must be an absolute path", field)
	}
	return clean, nil
}

// validateArgValue ensures a string used as a subprocess argument value does
// not look like a command-line flag and contains no control characters.
func validateArgValue(value, field string) error {
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("%s contains invalid characters", field)
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("%s must not start with '-'", field)
	}
	return nil
}

// handleCloneDomain clones a VM
func (s *Server) handleCloneDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Source    string   `json:"source"`          // Source domain name
		Target    string   `json:"target"`          // Target domain name
		Files     []string `json:"files,omitempty"` // New disk paths (optional)
		NewMAC    bool     `json:"new_mac"`         // Generate new MAC addresses
		AutoClone bool     `json:"auto_clone"`      // Auto-generate clone names
		Preserve  bool     `json:"preserve"`        // Preserve original domain
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateVMName(req.Source, "source"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}
	if err := validateVMName(req.Target, "target"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}

	// Build virt-clone command
	args := []string{
		"--original", req.Source,
		"--name", req.Target,
	}

	// Add custom disk paths if specified
	if len(req.Files) > 0 {
		for i, file := range req.Files {
			clean, err := validateFilePath(file, fmt.Sprintf("files[%d]", i))
			if err != nil {
				s.errorResponse(w, http.StatusBadRequest, "%v", err)
				return
			}
			req.Files[i] = clean
			args = append(args, "--file", clean)
		}
	} else {
		args = append(args, "--auto-clone")
	}

	// MAC address handling: virt-clone auto-generates new MACs by default,
	// regardless of req.NewMAC, so no flag is needed here.

	// Preserve original domain (don't undefine after cloning)
	if req.Preserve {
		args = append(args, "--preserve-data")
	}

	// #nosec G204 -- req.Source/req.Target validated by validateVMName and req.Files by validateFilePath above; virt-clone is a fixed binary
	cmd := exec.Command("virt-clone", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to clone domain: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Domain %s cloned to %s", req.Source, req.Target),
		"source":  req.Source,
		"target":  req.Target,
		"output":  string(output),
	})
}

// handleCloneMultipleDomains clones a VM multiple times
func (s *Server) handleCloneMultipleDomains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Source     string `json:"source"`      // Source domain name
		NamePrefix string `json:"name_prefix"` // Prefix for cloned VMs
		Count      int    `json:"count"`       // Number of clones to create
		StartIndex int    `json:"start_index"` // Starting index (default 1)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Count <= 0 || req.Count > 100 {
		http.Error(w, "count must be between 1 and 100", http.StatusBadRequest)
		return
	}

	if req.StartIndex == 0 {
		req.StartIndex = 1
	}

	if err := validateVMName(req.Source, "source"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}
	if err := validateVMName(req.NamePrefix, "name_prefix"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}

	type CloneResult struct {
		Name    string `json:"name"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	results := make([]CloneResult, 0, req.Count)

	for i := 0; i < req.Count; i++ {
		index := req.StartIndex + i
		targetName := fmt.Sprintf("%s-%d", req.NamePrefix, index)

		result := CloneResult{Name: targetName, Success: true}

		args := []string{
			"--original", req.Source,
			"--name", targetName,
			"--auto-clone",
		}

		// #nosec G204 -- req.Source validated by validateVMName and targetName is derived from a validated prefix plus a numeric index
		cmd := exec.Command("virt-clone", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			result.Success = false
			result.Error = string(output)
		}

		results = append(results, result)
	}

	successful := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			successful++
		} else {
			failed++
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":     "completed",
		"source":     req.Source,
		"total":      req.Count,
		"successful": successful,
		"failed":     failed,
		"results":    results,
	})
}

// handleCreateTemplate converts a VM into a template
func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain      string            `json:"domain"`      // Source domain
		Name        string            `json:"name"`        // Template name
		Description string            `json:"description"` // Template description
		Metadata    map[string]string `json:"metadata"`     // Additional metadata
		Seal        bool              `json:"seal"`         // Seal the template (virt-sysprep)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateVMName(req.Domain, "domain"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}
	if req.Name != "" {
		if err := validateVMName(req.Name, "name"); err != nil {
			s.errorResponse(w, http.StatusBadRequest, "%v", err)
			return
		}
	}
	if req.Description != "" {
		if err := validateArgValue(req.Description, "description"); err != nil {
			s.errorResponse(w, http.StatusBadRequest, "%v", err)
			return
		}
	}

	// Step 1: Ensure domain is shut down
	// #nosec G204 -- req.Domain validated by validateVMName above
	shutdownCmd := exec.Command("virsh", "shutdown", req.Domain)
	if err := shutdownCmd.Run(); err != nil {
		// Not fatal: the domain may already be shut down.
		s.logger.Debug("virsh shutdown failed (domain may already be shut down)", "domain", req.Domain, "error", err)
	}

	// Wait a few seconds for shutdown
	if err := exec.Command("sleep", "3").Run(); err != nil {
		s.logger.Warn("sleep failed while waiting for domain shutdown", "error", err)
	}

	// Step 2: Clone the domain to create template
	templateName := req.Name
	if templateName == "" {
		templateName = req.Domain + "-template"
	}

	cloneArgs := []string{
		"--original", req.Domain,
		"--name", templateName,
		"--auto-clone",
	}

	// #nosec G204 -- req.Domain and templateName are validated by validateVMName above
	cloneCmd := exec.Command("virt-clone", cloneArgs...)
	cloneOutput, err := cloneCmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to clone domain: %s", string(cloneOutput))
		return
	}

	// Step 3: Seal the template if requested (remove machine-specific config)
	if req.Seal {
		// #nosec G204 -- templateName validated by validateVMName above
		sealCmd := exec.Command("virt-sysprep", "-d", templateName)
		sealOutput, err := sealCmd.CombinedOutput()
		if err != nil {
			// If virt-sysprep fails, warn but continue
			s.logger.Warn("virt-sysprep failed", "error", string(sealOutput))
		}
	}

	// Step 4: Add metadata to template
	if req.Description != "" {
		// #nosec G204 -- templateName validated by validateVMName and req.Description validated by validateArgValue above
		descCmd := exec.Command("virsh", "desc", templateName, "--title", req.Description)
		if err := descCmd.Run(); err != nil {
			s.logger.Warn("failed to set template description", "template", templateName, "error", err)
		}
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Template %s created from %s", templateName, req.Domain),
		"template":    templateName,
		"source":      req.Domain,
		"sealed":      req.Seal,
		"description": req.Description,
	})
}

// handleDeployFromTemplate deploys a new VM from a template
func (s *Server) handleDeployFromTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Template  string            `json:"template"`  // Template name
		Name      string            `json:"name"`      // New VM name
		Memory    int               `json:"memory"`    // Memory in MB (optional)
		VCPUs     int               `json:"vcpus"`     // Number of CPUs (optional)
		Network   string            `json:"network"`   // Network name (optional)
		AutoStart bool              `json:"autostart"` // Start after creation
		Customize map[string]string `json:"customize"` // Customization options
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateVMName(req.Template, "template"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}
	if err := validateVMName(req.Name, "name"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}

	// Clone template to create new VM
	cloneArgs := []string{
		"--original", req.Template,
		"--name", req.Name,
		"--auto-clone",
	}

	// #nosec G204 -- req.Template and req.Name validated by validateVMName above
	cloneCmd := exec.Command("virt-clone", cloneArgs...)
	cloneOutput, err := cloneCmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to deploy from template: %s", string(cloneOutput))
		return
	}

	// Customize VM if requested
	if req.Memory > 0 {
		// #nosec G204 -- req.Name validated by validateVMName; memory value is an int formatted via fmt.Sprintf
		memCmd := exec.Command("virsh", "setmaxmem", req.Name, fmt.Sprintf("%dM", req.Memory), "--config")
		if err := memCmd.Run(); err != nil {
			s.logger.Warn("failed to set max memory", "vm", req.Name, "error", err)
		}
		// #nosec G204 -- req.Name validated by validateVMName; memory value is an int formatted via fmt.Sprintf
		memCmd = exec.Command("virsh", "setmem", req.Name, fmt.Sprintf("%dM", req.Memory), "--config")
		if err := memCmd.Run(); err != nil {
			s.logger.Warn("failed to set memory", "vm", req.Name, "error", err)
		}
	}

	if req.VCPUs > 0 {
		// #nosec G204 -- req.Name validated by validateVMName; vcpu value is an int formatted via fmt.Sprintf
		vcpuCmd := exec.Command("virsh", "setvcpus", req.Name, fmt.Sprintf("%d", req.VCPUs), "--config", "--maximum")
		if err := vcpuCmd.Run(); err != nil {
			s.logger.Warn("failed to set max vcpus", "vm", req.Name, "error", err)
		}
		// #nosec G204 -- req.Name validated by validateVMName; vcpu value is an int formatted via fmt.Sprintf
		vcpuCmd = exec.Command("virsh", "setvcpus", req.Name, fmt.Sprintf("%d", req.VCPUs), "--config")
		if err := vcpuCmd.Run(); err != nil {
			s.logger.Warn("failed to set vcpus", "vm", req.Name, "error", err)
		}
	}

	// Start VM if requested
	if req.AutoStart {
		// #nosec G204 -- req.Name validated by validateVMName above
		startCmd := exec.Command("virsh", "start", req.Name)
		if err := startCmd.Run(); err != nil {
			s.logger.Warn("failed to start VM", "vm", req.Name, "error", err)
		}
	}

	// Enable autostart if requested
	// #nosec G204 -- req.Name validated by validateVMName above
	autostartCmd := exec.Command("virsh", "autostart", req.Name)
	if err := autostartCmd.Run(); err != nil {
		s.logger.Warn("failed to enable autostart", "vm", req.Name, "error", err)
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("VM %s deployed from template %s", req.Name, req.Template),
		"vm":        req.Name,
		"template":  req.Template,
		"started":   req.AutoStart,
		"memory_mb": req.Memory,
		"vcpus":     req.VCPUs,
	})
}

// handleListTemplates lists all VM templates
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all domains
	cmd := exec.Command("virsh", "list", "--all", "--name")
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list domains: %v", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	var templates []string

	// Filter domains that are likely templates (contain "template" in name or are shut off)
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}

		// Check if domain name contains "template"
		if strings.Contains(strings.ToLower(name), "template") {
			templates = append(templates, name)
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

// handleExportTemplate exports a template for backup/sharing
func (s *Server) handleExportTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Template   string `json:"template"`    // Template name
		ExportPath string `json:"export_path"` // Export directory
		Compress   bool   `json:"compress"`    // Compress export
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateVMName(req.Template, "template"); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}
	cleanExportPath, err := validateFilePath(req.ExportPath, "export_path")
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "%v", err)
		return
	}
	req.ExportPath = cleanExportPath

	// Export domain XML
	xmlPath := filepath.Join(req.ExportPath, req.Template+".xml")
	// #nosec G204 -- req.Template validated by validateVMName above
	dumpCmd := exec.Command("virsh", "dumpxml", req.Template)
	xmlOutput, err := dumpCmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to dump XML: %v", err)
		return
	}

	// Write XML directly to file (avoids spawning a shell just to redirect output)
	if err := os.WriteFile(xmlPath, xmlOutput, 0600); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to write XML: %v", err)
		return
	}

	// Get disk paths
	// #nosec G204 -- req.Template validated by validateVMName above
	domblklistCmd := exec.Command("virsh", "domblklist", req.Template)
	domblklistOutput, err := domblklistCmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list disks: %v", err)
		return
	}

	var diskPaths []string
	lines := strings.Split(string(domblklistOutput), "\n")
	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			diskPaths = append(diskPaths, fields[1])
		}
	}

	// Copy disk files
	for _, diskPath := range diskPaths {
		// #nosec G204 -- diskPath comes from `virsh domblklist` output for the already-validated template; req.ExportPath is validated/cleaned above
		cpCmd := exec.Command("cp", diskPath, req.ExportPath)
		if err := cpCmd.Run(); err != nil {
			s.logger.Warn("failed to copy disk file", "disk", diskPath, "error", err)
		}
	}

	// Compress if requested
	if req.Compress {
		tarPath := filepath.Join(req.ExportPath, req.Template+".tar.gz")
		xmlName := req.Template + ".xml"
		// #nosec G204 -- req.ExportPath validated/cleaned and req.Template validated by validateVMName above
		tarCmd := exec.Command("tar", "-czf", tarPath, "-C", req.ExportPath, xmlName)
		if err := tarCmd.Run(); err != nil {
			s.logger.Warn("failed to compress template export", "template", req.Template, "error", err)
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Template %s exported to %s", req.Template, req.ExportPath),
		"template":    req.Template,
		"export_path": req.ExportPath,
		"xml_file":    xmlPath,
		"disk_files":  diskPaths,
		"compressed":  req.Compress,
	})
}
