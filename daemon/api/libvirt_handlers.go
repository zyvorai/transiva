// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
)

// libvirtNameRe restricts domain/pool/snapshot names accepted from API
// requests to a safe character set, and requires the name to start with an
// alphanumeric character so it cannot be interpreted as a virsh
// command-line flag (argument injection) when passed as a subprocess
// argument.
var libvirtNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$`)

// validateLibvirtName validates a libvirt resource name (domain, pool,
// snapshot, etc.) received from an API request before it is used as an
// argument to the virsh subprocess.
func validateLibvirtName(name string) error {
	if !libvirtNameRe.MatchString(name) {
		return fmt.Errorf("invalid name %q: must start with a letter or digit and contain only letters, digits, '.', '_', ':' or '-'", name)
	}
	return nil
}

// LibvirtDomain represents a libvirt domain/VM
type LibvirtDomain struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	UUID       string `json:"uuid"`
	State      string `json:"state"` // running, shut off, paused
	CPUs       int    `json:"cpus"`
	Memory     int    `json:"memory"` // MB
	Autostart  bool   `json:"autostart"`
	Persistent bool   `json:"persistent"`
}

// LibvirtSnapshot represents a VM snapshot
type LibvirtSnapshot struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	CreationTime string `json:"creation_time"`
	State        string `json:"state"`
	Current      bool   `json:"current"`
}

// LibvirtStoragePool represents a storage pool
type LibvirtStoragePool struct {
	Name      string `json:"name"`
	UUID      string `json:"uuid"`
	State     string `json:"state"`
	Capacity  int64  `json:"capacity"`
	Available int64  `json:"available"`
	Path      string `json:"path"`
}

// LibvirtVolume represents a storage volume
type LibvirtVolume struct {
	Name       string `json:"name"`
	Pool       string `json:"pool,omitempty"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Capacity   int64  `json:"capacity"`
	Allocation int64  `json:"allocation,omitempty"`
	Format     string `json:"format"`
}

// handleListLibvirtDomains lists all libvirt domains
func (s *Server) handleListLibvirtDomains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// List all domains (running and shut off)
	cmd := exec.Command("virsh", "list", "--all")
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list domains: %v", err)
		return
	}

	domains := s.parseVirshListOutput(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"domains": domains,
		"total":   len(domains),
	})
}

// handleGetLibvirtDomain gets details of a specific domain
func (s *Server) handleGetLibvirtDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get domain info
	// #nosec G204,G702 -- name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "dominfo", name)
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "domain not found: %v", err)
		return
	}

	domain := s.parseVirshDominfo(string(output))
	domain.Name = name

	s.jsonResponse(w, http.StatusOK, domain)
}

// handleStartLibvirtDomain starts a domain
func (s *Server) handleStartLibvirtDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "start", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to start domain: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Domain %s started", req.Name),
	})
}

// handleShutdownLibvirtDomain gracefully shuts down a domain
func (s *Server) handleShutdownLibvirtDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "shutdown", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to shutdown domain: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Domain %s shutdown initiated", req.Name),
	})
}

// handleDestroyLibvirtDomain forcefully stops a domain
func (s *Server) handleDestroyLibvirtDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "destroy", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to destroy domain: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Domain %s destroyed", req.Name),
	})
}

// handleRebootLibvirtDomain reboots a domain
func (s *Server) handleRebootLibvirtDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "reboot", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to reboot domain: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Domain %s reboot initiated", req.Name),
	})
}

// handlePauseLibvirtDomain pauses a domain
func (s *Server) handlePauseLibvirtDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "suspend", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to pause domain: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Domain %s paused", req.Name),
	})
}

// handleResumeLibvirtDomain resumes a paused domain
func (s *Server) handleResumeLibvirtDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "resume", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to resume domain: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Domain %s resumed", req.Name),
	})
}

// handleListLibvirtSnapshots lists snapshots for a domain
func (s *Server) handleListLibvirtSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "snapshot-list", name, "--tree")
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list snapshots: %v", err)
		return
	}

	snapshots := s.parseSnapshotList(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"snapshots": snapshots,
		"total":     len(snapshots),
		"domain":    name,
	})
}

// handleCreateLibvirtSnapshot creates a snapshot
func (s *Server) handleCreateLibvirtSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName   string `json:"domain_name"`
		SnapshotName string `json:"snapshot_name"`
		Description  string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.DomainName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.SnapshotName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	args := []string{"snapshot-create-as", req.DomainName, req.SnapshotName}
	if req.Description != "" {
		args = append(args, "--description", req.Description)
	}

	// #nosec G204 -- DomainName/SnapshotName are validated by validateLibvirtName above; Description is passed as a value to the fixed --description flag, not interpreted as a command
	cmd := exec.Command("virsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create snapshot: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Snapshot %s created for domain %s", req.SnapshotName, req.DomainName),
	})
}

// handleRevertLibvirtSnapshot reverts to a snapshot
func (s *Server) handleRevertLibvirtSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName   string `json:"domain_name"`
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.DomainName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.SnapshotName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- DomainName/SnapshotName are validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "snapshot-revert", req.DomainName, req.SnapshotName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to revert snapshot: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Reverted to snapshot %s", req.SnapshotName),
	})
}

// handleDeleteLibvirtSnapshot deletes a snapshot
func (s *Server) handleDeleteLibvirtSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DomainName   string `json:"domain_name"`
		SnapshotName string `json:"snapshot_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.DomainName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(req.SnapshotName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204 -- DomainName/SnapshotName are validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "snapshot-delete", req.DomainName, req.SnapshotName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to delete snapshot: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Snapshot %s deleted", req.SnapshotName),
	})
}

// handleListLibvirtPools lists storage pools
func (s *Server) handleListLibvirtPools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := exec.Command("virsh", "pool-list", "--all")
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list pools: %v", err)
		return
	}

	pools := s.parsePoolList(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"pools": pools,
		"total": len(pools),
	})
}

// handleListLibvirtVolumes lists volumes in a pool
func (s *Server) handleListLibvirtVolumes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	poolName := r.URL.Query().Get("pool")
	if poolName == "" {
		poolName = "default"
	}
	if err := validateLibvirtName(poolName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- poolName is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "vol-list", poolName)
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list volumes: %v", err)
		return
	}

	volumes := s.parseVolumeList(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"volumes": volumes,
		"total":   len(volumes),
		"pool":    poolName,
	})
}

// parseVirshListOutput parses virsh list output
func (s *Server) parseVirshListOutput(output string) []LibvirtDomain {
	lines := strings.Split(output, "\n")
	domains := []LibvirtDomain{}

	for i, line := range lines {
		// Skip header and separator lines
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		state := strings.Join(fields[2:], " ")
		domain := LibvirtDomain{
			ID:    fields[0],
			Name:  fields[1],
			State: state,
		}
		domains = append(domains, domain)
	}

	return domains
}

// parseVirshDominfo parses virsh dominfo output
func (s *Server) parseVirshDominfo(output string) LibvirtDomain {
	domain := LibvirtDomain{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "UUID":
			domain.UUID = value
		case "State":
			domain.State = value
		case "CPU(s)":
			if _, err := fmt.Sscanf(value, "%d", &domain.CPUs); err != nil {
				s.logger.Warn("failed to parse domain CPU count from virsh output", "value", value, "error", err)
			}
		case "Max memory":
			var mem int
			if _, err := fmt.Sscanf(value, "%d", &mem); err != nil {
				s.logger.Warn("failed to parse domain memory from virsh output", "value", value, "error", err)
			}
			domain.Memory = mem / 1024 // Convert KB to MB
		case "Autostart":
			domain.Autostart = value == "enable"
		case "Persistent":
			domain.Persistent = value == "yes"
		}
	}

	return domain
}

// parseSnapshotList parses snapshot list output
func (s *Server) parseSnapshotList(output string) []LibvirtSnapshot {
	snapshots := []LibvirtSnapshot{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		snapshot := LibvirtSnapshot{
			Name: line,
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots
}

// parsePoolList parses pool list output
func (s *Server) parsePoolList(output string) []LibvirtStoragePool {
	pools := []LibvirtStoragePool{}
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		pool := LibvirtStoragePool{
			Name:  fields[0],
			State: fields[1],
		}
		pools = append(pools, pool)
	}

	return pools
}

// parseVolumeList parses volume list output
func (s *Server) parseVolumeList(output string) []LibvirtVolume {
	volumes := []LibvirtVolume{}
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		volume := LibvirtVolume{
			Name: fields[0],
			Path: fields[1],
		}
		volumes = append(volumes, volume)
	}

	return volumes
}

// handleGetLibvirtConsole gets VNC console info
func (s *Server) handleGetLibvirtConsole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}
	if err := validateLibvirtName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get VNC port
	// #nosec G204,G702 -- name is validated by validateLibvirtName above against an allowlist regex
	cmd := exec.Command("virsh", "vncdisplay", name)
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to get VNC display: %v", err)
		return
	}

	vncDisplay := strings.TrimSpace(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"domain":      name,
		"vnc_display": vncDisplay,
		"vnc_url":     fmt.Sprintf("vnc://localhost%s", vncDisplay),
	})
}
