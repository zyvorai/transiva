// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigGeneratorRequest represents a request to generate h2kvm config
type ConfigGeneratorRequest struct {
	OSType          string `json:"os_type"`          // windows, linux
	OSFlavor        string `json:"os_flavor"`        // windows-10, windows-11, ubuntu-22, rhel-10, photon-os
	VMDKPath        string `json:"vmdk_path"`        // Source VMDK path
	OutputDir       string `json:"output_dir"`       // Output directory
	VMName          string `json:"vm_name"`          // VM name
	Flatten         bool   `json:"flatten"`          // Flatten snapshots
	Compress        bool   `json:"compress"`         // Compress output
	VirtIODrivers   string `json:"virtio_drivers"`   // VirtIO drivers dir (Windows only)
	LibvirtTest     bool   `json:"libvirt_test"`     // Run libvirt smoke test
	Memory          int    `json:"memory"`           // Memory in MB
	VCPUs           int    `json:"vcpus"`            // Number of vCPUs
	UEFI            bool   `json:"uefi"`             // Use UEFI
	GenerateService bool   `json:"generate_service"` // Generate systemd service
}

// H2KVMConfig represents the full h2kvm YAML configuration
type H2KVMConfig struct {
	Cmd              string `yaml:"cmd"`
	VMDK             string `yaml:"vmdk"`
	OutputDir        string `yaml:"output_dir"`
	Flatten          bool   `yaml:"flatten,omitempty"`
	FlattenFormat    string `yaml:"flatten_format,omitempty"`
	ToOutput         string `yaml:"to_output"`
	OutFormat        string `yaml:"out_format"`
	Compress         bool   `yaml:"compress"`
	Checksum         bool   `yaml:"checksum"`
	PrintFstab       bool   `yaml:"print_fstab"`
	FstabMode        string `yaml:"fstab_mode"`
	NoGrub           bool   `yaml:"no_grub"`
	RegenInitramfs   bool   `yaml:"regen_initramfs"`
	NoBackup         bool   `yaml:"no_backup"`
	DryRun           bool   `yaml:"dry_run"`
	Verbose          int    `yaml:"verbose"`
	LogFile          string `yaml:"log_file,omitempty"`
	Report           string `yaml:"report,omitempty"`
	VirtIODriversDir string `yaml:"virtio_drivers_dir,omitempty"`
	LibvirtTest      bool   `yaml:"libvirt_test,omitempty"`
	QemuTest         bool   `yaml:"qemu_test,omitempty"`
	VMName           string `yaml:"vm_name,omitempty"`
	Memory           int    `yaml:"memory,omitempty"`
	VCPUs            int    `yaml:"vcpus,omitempty"`
	UEFI             bool   `yaml:"uefi,omitempty"`
	Timeout          int    `yaml:"timeout,omitempty"`
	KeepDomain       bool   `yaml:"keep_domain,omitempty"`
	Headless         bool   `yaml:"headless,omitempty"`
}

// ConfigGeneratorResponse represents the generated configuration
type ConfigGeneratorResponse struct {
	ConfigYAML  string `json:"config_yaml"`
	ConfigPath  string `json:"config_path"`
	ServiceFile string `json:"service_file,omitempty"`
	ServicePath string `json:"service_path,omitempty"`
}

// handleGenerateConfig generates h2kvm configuration files
func (s *Server) handleGenerateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConfigGeneratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.OSType == "" || req.OSFlavor == "" || req.VMDKPath == "" || req.OutputDir == "" {
		http.Error(w, "missing required fields: os_type, os_flavor, vmdk_path, output_dir", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Memory == 0 {
		req.Memory = 2048
	}
	if req.VCPUs == 0 {
		req.VCPUs = 2
	}

	// Generate config
	config := s.buildH2KVMConfig(req)

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to generate YAML: %v", err), http.StatusInternalServerError)
		return
	}

	// Add header comment
	yamlContent := s.generateConfigHeader(req) + string(yamlBytes)

	// Generate config path
	configFilename := fmt.Sprintf("h2kvm-%s.yaml", strings.ToLower(req.VMName))
	if req.VMName == "" {
		configFilename = fmt.Sprintf("h2kvm-%s-%s.yaml", req.OSType, req.OSFlavor)
	}
	configPath := filepath.Join(req.OutputDir, configFilename)

	response := ConfigGeneratorResponse{
		ConfigYAML: yamlContent,
		ConfigPath: configPath,
	}

	// Generate systemd service if requested
	if req.GenerateService {
		serviceContent := s.generateSystemdService(req, configPath)
		servicePath := fmt.Sprintf("/etc/systemd/system/h2kvm-%s.service", strings.ToLower(req.VMName))
		response.ServiceFile = serviceContent
		response.ServicePath = servicePath
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// buildH2KVMConfig builds the h2kvm config based on request
func (s *Server) buildH2KVMConfig(req ConfigGeneratorRequest) *H2KVMConfig {
	outputFilename := fmt.Sprintf("%s-fixed.qcow2", strings.ToLower(req.VMName))
	if req.VMName == "" {
		outputFilename = fmt.Sprintf("%s-%s-fixed.qcow2", req.OSType, req.OSFlavor)
	}

	config := &H2KVMConfig{
		Cmd:            "local",
		VMDK:           req.VMDKPath,
		OutputDir:      req.OutputDir,
		Flatten:        req.Flatten,
		ToOutput:       outputFilename,
		OutFormat:      "qcow2",
		Compress:       req.Compress,
		Checksum:       true,
		PrintFstab:     true,
		FstabMode:      "stabilize-all",
		NoGrub:         false,
		RegenInitramfs: true,
		NoBackup:       false,
		DryRun:         false,
		Verbose:        2,
		LogFile:        filepath.Join(req.OutputDir, "h2kvm.log"),
		Report:         filepath.Join(req.OutputDir, "h2kvm-report.md"),
	}

	if req.Flatten {
		config.FlattenFormat = "qcow2"
	}

	// Windows-specific configuration
	if req.OSType == "windows" {
		if req.VirtIODrivers != "" {
			config.VirtIODriversDir = req.VirtIODrivers
		}
		config.PrintFstab = false // Windows doesn't have fstab
		config.RegenInitramfs = false
		config.NoGrub = true
	}

	// Add libvirt test configuration if requested
	if req.LibvirtTest {
		config.LibvirtTest = true
		config.VMName = req.VMName
		config.Memory = req.Memory
		config.VCPUs = req.VCPUs
		config.UEFI = req.UEFI
		config.Timeout = 60
		config.KeepDomain = false
		config.Headless = true
	}

	return config
}

// generateConfigHeader generates a header comment for the config file
func (s *Server) generateConfigHeader(req ConfigGeneratorRequest) string {
	osEmoji := map[string]string{
		"windows": "🪟",
		"linux":   "🐧",
	}

	flavorName := map[string]string{
		"windows-10": "Windows 10",
		"windows-11": "Windows 11",
		"ubuntu-22":  "Ubuntu 22.04 LTS",
		"ubuntu-24":  "Ubuntu 24.04 LTS",
		"rhel-10":    "Red Hat Enterprise Linux 10",
		"rhel-9":     "Red Hat Enterprise Linux 9",
		"photon-os":  "Photon OS",
		"debian-12":  "Debian 12",
		"rocky-9":    "Rocky Linux 9",
		"alma-9":     "AlmaLinux 9",
	}

	emoji := osEmoji[req.OSType]
	if emoji == "" {
		emoji = "💻"
	}

	name := flavorName[req.OSFlavor]
	if name == "" {
		name = req.OSFlavor
	}

	header := fmt.Sprintf(`# ==============================================================================
# %s %s VM Conversion Configuration
# ==============================================================================
#
# Generated by HyperSDK Migration Wizard
# OS Type: %s
# Flavor: %s
# VM Name: %s
#
# Usage:
#   h2kvm --config %s local
#
# Features:
#   ✅ Automatic fstab stabilization (UUID-based)
#   ✅ GRUB root= fixing
#   ✅ Initramfs regeneration
#   ✅ QCOW2 format with compression
#   ✅ Checksum generation
#   ✅ Detailed reporting
#
# ==============================================================================

`, emoji, name, req.OSType, req.OSFlavor, req.VMName, filepath.Base(fmt.Sprintf("h2kvm-%s.yaml", req.VMName)))

	return header
}

// generateSystemdService generates a systemd service file
func (s *Server) generateSystemdService(req ConfigGeneratorRequest, configPath string) string {
	serviceName := strings.ToLower(req.VMName)
	if serviceName == "" {
		serviceName = fmt.Sprintf("%s-%s", req.OSType, req.OSFlavor)
	}

	service := fmt.Sprintf(`[Unit]
Description=HyperSDK VM Conversion - %s
Documentation=https://github.com/zyvorai/h2kvm
After=network.target

[Service]
Type=oneshot
# NOTE: invokes the pre-rename "hyper2kvm" Python module; the upstream
# project (github.com/zyvorai/h2kvm) now ships an "h2kvmctl" CLI with a
# different flag surface that this generated unit hasn't been re-verified against.
ExecStart=/usr/bin/python3 -m hyper2kvm --config %s local
WorkingDirectory=%s
StandardOutput=journal
StandardError=journal
User=root
RemainAfterExit=yes

# Security settings
PrivateTmp=yes
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=%s

# Resource limits
TimeoutStartSec=3600
MemoryLimit=4G

[Install]
WantedBy=multi-user.target
`, serviceName, configPath, req.OutputDir, req.OutputDir)

	return service
}

// handleListConfigTemplates lists available configuration templates
func (s *Server) handleListConfigTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	templates := []map[string]interface{}{
		{
			"id":              "windows-10",
			"name":            "Windows 10",
			"os_type":         "windows",
			"description":     "Windows 10 with VirtIO drivers",
			"requires_virtio": true,
		},
		{
			"id":              "windows-11",
			"name":            "Windows 11",
			"os_type":         "windows",
			"description":     "Windows 11 with VirtIO drivers and UEFI",
			"requires_virtio": true,
			"requires_uefi":   true,
		},
		{
			"id":          "ubuntu-22",
			"name":        "Ubuntu 22.04 LTS",
			"os_type":     "linux",
			"description": "Ubuntu 22.04 with systemd and netplan",
		},
		{
			"id":          "ubuntu-24",
			"name":        "Ubuntu 24.04 LTS",
			"os_type":     "linux",
			"description": "Ubuntu 24.04 with systemd and netplan",
		},
		{
			"id":          "rhel-10",
			"name":        "RHEL 10",
			"os_type":     "linux",
			"description": "Red Hat Enterprise Linux 10 with dracut",
		},
		{
			"id":          "rhel-9",
			"name":        "RHEL 9",
			"os_type":     "linux",
			"description": "Red Hat Enterprise Linux 9 with dracut",
		},
		{
			"id":          "photon-os",
			"name":        "Photon OS",
			"os_type":     "linux",
			"description": "VMware Photon OS",
		},
		{
			"id":          "debian-12",
			"name":        "Debian 12",
			"os_type":     "linux",
			"description": "Debian 12 (Bookworm)",
		},
		{
			"id":          "rocky-9",
			"name":        "Rocky Linux 9",
			"os_type":     "linux",
			"description": "Rocky Linux 9 (RHEL clone)",
		},
		{
			"id":          "alma-9",
			"name":        "AlmaLinux 9",
			"os_type":     "linux",
			"description": "AlmaLinux 9 (RHEL clone)",
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}
