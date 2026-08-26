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

// macAddressPattern validates a colon-separated MAC address (e.g. 52:54:00:aa:bb:cc)
var macAddressPattern = regexp.MustCompile(`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)

// isValidMACAddress validates that mac is a well-formed MAC address
func isValidMACAddress(mac string) bool {
	return macAddressPattern.MatchString(mac)
}

// isValidNICModel validates that model is one of the supported virtio NIC models
func isValidNICModel(model string) bool {
	switch model {
	case "virtio", "e1000", "rtl8139":
		return true
	default:
		return false
	}
}

// LibvirtNetwork represents a libvirt network
type LibvirtNetwork struct {
	Name       string `json:"name"`
	UUID       string `json:"uuid"`
	Active     bool   `json:"active"`
	Persistent bool   `json:"persistent"`
	Autostart  bool   `json:"autostart"`
	Bridge     string `json:"bridge"`
}

// handleListNetworks lists all libvirt networks
func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := exec.Command("virsh", "net-list", "--all")
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list networks: %v", err)
		return
	}

	networks := s.parseNetworkList(string(output))

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"networks": networks,
		"total":    len(networks),
	})
}

// handleGetNetwork gets network details
func (s *Server) handleGetNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}
	if !isValidVMName(name) {
		http.Error(w, "invalid name parameter", http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd := exec.Command("virsh", "net-info", name)
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "network not found: %v", err)
		return
	}

	network := s.parseNetworkInfo(name, string(output))

	s.jsonResponse(w, http.StatusOK, network)
}

// handleCreateNetwork creates a new network
func (s *Server) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Bridge  string `json:"bridge"`
		Forward string `json:"forward"` // nat, route, bridge, none
		IPStart string `json:"ip_start"`
		IPEnd   string `json:"ip_end"`
		Subnet  string `json:"subnet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidVMName(req.Name) {
		http.Error(w, "invalid name parameter", http.StatusBadRequest)
		return
	}

	// Generate network XML
	xml := fmt.Sprintf(`<network>
  <name>%s</name>
  <forward mode='%s'/>
  <bridge name='%s' stp='on' delay='0'/>
  <ip address='%s' netmask='255.255.255.0'>
    <dhcp>
      <range start='%s' end='%s'/>
    </dhcp>
  </ip>
</network>`, req.Name, req.Forward, req.Bridge, req.Subnet, req.IPStart, req.IPEnd)

	// Define network
	cmd := exec.Command("virsh", "net-define", "/dev/stdin")
	cmd.Stdin = strings.NewReader(xml)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create network: %s", string(output))
		return
	}

	// Start network
	// #nosec G204,G702 -- req.Name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd = exec.Command("virsh", "net-start", req.Name)
	if err := cmd.Run(); err != nil {
		s.logger.Warn("failed to start network after create", "name", req.Name, "error", err)
	}

	// Autostart
	// #nosec G204,G702 -- req.Name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd = exec.Command("virsh", "net-autostart", req.Name)
	if err := cmd.Run(); err != nil {
		s.logger.Warn("failed to set network autostart after create", "name", req.Name, "error", err)
	}

	s.jsonResponse(w, http.StatusCreated, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Network %s created", req.Name),
		"name":    req.Name,
	})
}

// handleDeleteNetwork deletes a network
func (s *Server) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
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
	if !isValidVMName(req.Name) {
		http.Error(w, "invalid name parameter", http.StatusBadRequest)
		return
	}

	// Destroy (stop) network
	// #nosec G204,G702 -- req.Name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd := exec.Command("virsh", "net-destroy", req.Name)
	if err := cmd.Run(); err != nil {
		// Ignore error if already stopped, but log for observability
		s.logger.Debug("net-destroy before delete failed (may already be stopped)", "name", req.Name, "error", err)
	}

	// Undefine network
	// #nosec G204,G702 -- req.Name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd = exec.Command("virsh", "net-undefine", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to delete network: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Network %s deleted", req.Name),
	})
}

// handleStartNetwork starts a network
func (s *Server) handleStartNetwork(w http.ResponseWriter, r *http.Request) {
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
	if !isValidVMName(req.Name) {
		http.Error(w, "invalid name parameter", http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- req.Name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd := exec.Command("virsh", "net-start", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to start network: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Network %s started", req.Name),
	})
}

// handleStopNetwork stops a network
func (s *Server) handleStopNetwork(w http.ResponseWriter, r *http.Request) {
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
	if !isValidVMName(req.Name) {
		http.Error(w, "invalid name parameter", http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- req.Name validated by isValidVMName() to contain only [a-zA-Z0-9._-]
	cmd := exec.Command("virsh", "net-destroy", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to stop network: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Network %s stopped", req.Name),
	})
}

// handleAttachInterface attaches network interface to VM
func (s *Server) handleAttachInterface(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMName  string `json:"vm_name"`
		Network string `json:"network"`
		Model   string `json:"model"` // virtio, e1000, rtl8139
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		req.Model = "virtio"
	}

	if !isValidVMName(req.VMName) {
		http.Error(w, "invalid vm_name parameter", http.StatusBadRequest)
		return
	}
	if !isValidVMName(req.Network) {
		http.Error(w, "invalid network parameter", http.StatusBadRequest)
		return
	}
	if !isValidNICModel(req.Model) {
		http.Error(w, "invalid model parameter", http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- vm_name/network validated by isValidVMName(), model validated by isValidNICModel()
	cmd := exec.Command("virsh", "attach-interface", req.VMName,
		"network", req.Network,
		"--model", req.Model,
		"--config", "--live")
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to attach interface: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Interface attached to %s", req.VMName),
	})
}

// handleDetachInterface detaches network interface from VM
func (s *Server) handleDetachInterface(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMName string `json:"vm_name"`
		MAC    string `json:"mac"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !isValidVMName(req.VMName) {
		http.Error(w, "invalid vm_name parameter", http.StatusBadRequest)
		return
	}
	if !isValidMACAddress(req.MAC) {
		http.Error(w, "invalid mac parameter", http.StatusBadRequest)
		return
	}

	// #nosec G204,G702 -- vm_name validated by isValidVMName(), mac validated by isValidMACAddress()
	cmd := exec.Command("virsh", "detach-interface", req.VMName,
		"network", "--mac", req.MAC,
		"--config", "--live")
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to detach interface: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Interface detached from %s", req.VMName),
	})
}

// parseNetworkList parses virsh net-list output
func (s *Server) parseNetworkList(output string) []LibvirtNetwork {
	lines := strings.Split(output, "\n")
	networks := []LibvirtNetwork{}

	for i, line := range lines {
		if i < 2 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		network := LibvirtNetwork{
			Name:   fields[0],
			Active: fields[1] == "active",
		}
		networks = append(networks, network)
	}

	return networks
}

// parseNetworkInfo parses virsh net-info output
func (s *Server) parseNetworkInfo(name, output string) LibvirtNetwork {
	network := LibvirtNetwork{Name: name}
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
			network.UUID = value
		case "Active":
			network.Active = value == "yes"
		case "Persistent":
			network.Persistent = value == "yes"
		case "Autostart":
			network.Autostart = value == "yes"
		case "Bridge":
			network.Bridge = value
		}
	}

	return network
}
