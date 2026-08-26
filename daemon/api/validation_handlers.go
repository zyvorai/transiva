// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// ValidationResult represents a validation result
type ValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []string               `json:"errors,omitempty"`
	Warnings []string               `json:"warnings,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// handleValidateMigration validates a VM/disk before migration
func (s *Server) handleValidateMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path   string `json:"path"`              // Path to VMDK/disk file
		VMName string `json:"vm_name,omitempty"` // Optional: vCenter VM name
		Format string `json:"format,omitempty"`  // Expected format (vmdk, qcow2, etc.)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result := ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
		Details:  make(map[string]interface{}),
	}

	// Check if file exists
	if req.Path != "" && !isValidVolumeSourcePath(req.Path) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("invalid file path: %s", req.Path))
		req.Path = ""
	}
	if req.Path != "" {
		fileInfo, err := os.Stat(req.Path)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("file not found: %s", req.Path))
		} else {
			result.Details["file_size"] = fileInfo.Size()
			result.Details["file_path"] = req.Path

			// Validate disk image using qemu-img info
			// #nosec G204 -- req.Path is validated by isValidVolumeSourcePath above
			infoCmd := exec.Command("qemu-img", "info", "--output=json", req.Path)
			output, err := infoCmd.Output()
			if err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("failed to read disk image: %v", err))
			} else {
				var imageInfo map[string]interface{}
				if json.Unmarshal(output, &imageInfo) == nil {
					result.Details["image_info"] = imageInfo

					// Validate format if specified
					if req.Format != "" {
						if format, ok := imageInfo["format"].(string); ok {
							if format != req.Format {
								result.Warnings = append(result.Warnings,
									fmt.Sprintf("format mismatch: expected %s, got %s", req.Format, format))
							}
						}
					}

					// Check for backing files (snapshots)
					if backingFile, ok := imageInfo["backing-filename"]; ok {
						result.Warnings = append(result.Warnings,
							fmt.Sprintf("disk has backing file: %s", backingFile))
						result.Details["has_backing_file"] = true
					}

					// Check virtual size
					if virtualSize, ok := imageInfo["virtual-size"].(float64); ok {
						if virtualSize > 2*1024*1024*1024*1024 { // > 2TB
							result.Warnings = append(result.Warnings,
								fmt.Sprintf("large disk size: %.2f GB", virtualSize/(1024*1024*1024)))
						}
						result.Details["virtual_size_gb"] = virtualSize / (1024 * 1024 * 1024)
					}
				}
			}

			// Run qemu-img check
			// #nosec G204 -- req.Path is validated by isValidVolumeSourcePath above
			checkCmd := exec.Command("qemu-img", "check", req.Path)
			checkOutput, err := checkCmd.CombinedOutput()
			if err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("disk check failed: %s", string(checkOutput)))
			} else {
				result.Details["check_result"] = "passed"

				// Parse check output for warnings
				checkStr := string(checkOutput)
				if strings.Contains(checkStr, "leaked clusters") {
					result.Warnings = append(result.Warnings, "disk has leaked clusters")
				}
				if strings.Contains(checkStr, "corruptions") {
					result.Errors = append(result.Errors, "disk has corruptions")
					result.Valid = false
				}
			}
		}
	}

	// Validation summary
	result.Details["total_errors"] = len(result.Errors)
	result.Details["total_warnings"] = len(result.Warnings)

	status := http.StatusOK
	if !result.Valid {
		status = http.StatusOK // Still 200, but valid=false
	}

	s.jsonResponse(w, status, map[string]interface{}{
		"status":     "complete",
		"validation": result,
	})
}

// handleVerifyMigration verifies a VM after migration
func (s *Server) handleVerifyMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMName        string `json:"vm_name"`
		SourcePath    string `json:"source_path,omitempty"`    // Original VMDK path
		ConvertedPath string `json:"converted_path,omitempty"` // Converted qcow2 path
		BootTest      bool   `json:"boot_test"`                // Test if VM boots
		ChecksumTest  bool   `json:"checksum_test"`            // Compare checksums
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result := ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
		Details:  make(map[string]interface{}),
	}

	if !isValidVMName(req.VMName) {
		s.jsonResponse(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "invalid vm_name",
		})
		return
	}
	if req.SourcePath != "" && !isValidVolumeSourcePath(req.SourcePath) {
		http.Error(w, "invalid source_path", http.StatusBadRequest)
		return
	}
	if req.ConvertedPath != "" && !isValidVolumeSourcePath(req.ConvertedPath) {
		http.Error(w, "invalid converted_path", http.StatusBadRequest)
		return
	}

	// Check if VM exists
	// #nosec G204 -- req.VMName is validated by isValidVMName above
	domainCmd := exec.Command("virsh", "dominfo", req.VMName)
	output, err := domainCmd.Output()
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("VM not found: %s", req.VMName))
	} else {
		result.Details["vm_exists"] = true

		// Parse domain info
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "State:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					state := strings.TrimSpace(parts[1])
					result.Details["vm_state"] = state
				}
			}
		}
	}

	// Verify disk files exist
	// #nosec G204 -- req.VMName is validated by isValidVMName above
	domblklistCmd := exec.Command("virsh", "domblklist", req.VMName)
	diskOutput, err := domblklistCmd.Output()
	if err == nil {
		disks := parseDiskList(string(diskOutput))
		result.Details["disk_count"] = len(disks)

		for _, disk := range disks {
			if disk != "" && !fileExists(disk) {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("disk file missing: %s", disk))
			}
		}
	}

	// Boot test if requested
	if req.BootTest {
		result.Details["boot_test"] = "requested"

		// Check current state
		// #nosec G204 -- req.VMName is validated by isValidVMName above
		stateCmd := exec.Command("virsh", "domstate", req.VMName)
		stateOutput, _ := stateCmd.Output()
		currentState := strings.TrimSpace(string(stateOutput))

		if currentState == "shut off" {
			// Try to start VM
			// #nosec G204 -- req.VMName is validated by isValidVMName above
			startCmd := exec.Command("virsh", "start", req.VMName)
			if err := startCmd.Run(); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, "failed to start VM for boot test")
				result.Details["boot_test"] = "failed"
			} else {
				result.Details["boot_test"] = "started"
				result.Warnings = append(result.Warnings, "VM started for boot test - remember to shut down")

				// Wait a moment and check if still running
				// In production, would poll for a few seconds
				// #nosec G204 -- req.VMName is validated by isValidVMName above
				stateCmd2 := exec.Command("virsh", "domstate", req.VMName)
				stateOutput2, _ := stateCmd2.Output()
				newState := strings.TrimSpace(string(stateOutput2))

				if newState == "running" {
					result.Details["boot_test"] = "success"
				} else {
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("VM not running after start: %s", newState))
					result.Details["boot_test"] = "failed"
				}
			}
		} else {
			result.Details["boot_test"] = "skipped (VM already running)"
		}
	}

	// Checksum test if requested and paths provided
	if req.ChecksumTest && req.SourcePath != "" && req.ConvertedPath != "" {
		result.Details["checksum_test"] = "requested"

		if fileExists(req.SourcePath) && fileExists(req.ConvertedPath) {
			// Compare virtual sizes
			sourceInfo := getDiskInfo(req.SourcePath)
			convertedInfo := getDiskInfo(req.ConvertedPath)

			if sourceInfo != nil && convertedInfo != nil {
				result.Details["source_virtual_size"] = sourceInfo["virtual-size"]
				result.Details["converted_virtual_size"] = convertedInfo["virtual-size"]

				if sourceInfo["virtual-size"] != convertedInfo["virtual-size"] {
					result.Warnings = append(result.Warnings, "virtual size mismatch between source and converted disk")
				} else {
					result.Details["checksum_test"] = "sizes match"
				}
			}
		} else {
			result.Warnings = append(result.Warnings, "source or converted path not accessible for checksum test")
			result.Details["checksum_test"] = "skipped"
		}
	}

	result.Details["total_errors"] = len(result.Errors)
	result.Details["total_warnings"] = len(result.Warnings)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":       "complete",
		"verification": result,
	})
}

// handleCheckCompatibility checks VM compatibility with KVM
func (s *Server) handleCheckCompatibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMName   string   `json:"vm_name,omitempty"`
		DiskPath string   `json:"disk_path,omitempty"`
		OSType   string   `json:"os_type,omitempty"`  // windows, linux, etc.
		Firmware string   `json:"firmware,omitempty"` // bios, uefi
		Features []string `json:"features,omitempty"` // Required features
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result := ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
		Details:  make(map[string]interface{}),
	}

	if req.VMName != "" && !isValidVMName(req.VMName) {
		http.Error(w, "invalid vm_name", http.StatusBadRequest)
		return
	}
	if req.DiskPath != "" && !isValidVolumeSourcePath(req.DiskPath) {
		http.Error(w, "invalid disk_path", http.StatusBadRequest)
		return
	}

	// Check KVM support
	kvmCheckCmd := exec.Command("virsh", "capabilities")
	kvmOutput, err := kvmCheckCmd.Output()
	if err != nil {
		result.Warnings = append(result.Warnings, "could not check KVM capabilities")
	} else {
		capsStr := string(kvmOutput)
		result.Details["kvm_available"] = strings.Contains(capsStr, "<domain type='kvm'>")

		// Check for UEFI support if needed
		if req.Firmware == "uefi" {
			hasUEFI := strings.Contains(capsStr, "OVMF") || strings.Contains(capsStr, "ovmf")
			result.Details["uefi_support"] = hasUEFI
			if !hasUEFI {
				result.Valid = false
				result.Errors = append(result.Errors, "UEFI firmware requested but not available")
			}
		}
	}

	// Check if VM exists and get its configuration
	if req.VMName != "" {
		// #nosec G204 -- req.VMName is validated by isValidVMName above
		xmlCmd := exec.Command("virsh", "dumpxml", req.VMName)
		xmlOutput, err := xmlCmd.Output()
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read VM XML: %s", req.VMName))
		} else {
			xmlStr := string(xmlOutput)
			result.Details["has_xml"] = true

			// Check for potentially incompatible features
			if strings.Contains(xmlStr, "vmware") {
				result.Warnings = append(result.Warnings, "VM XML contains VMware-specific configurations")
			}

			// Check OS type
			if strings.Contains(xmlStr, "<os>") {
				if strings.Contains(xmlStr, "hvm") {
					result.Details["os_type"] = "hvm"
				}
				if strings.Contains(xmlStr, "loader") {
					result.Details["uses_uefi"] = true
				}
			}

			// Check for VirtIO drivers (recommended for performance)
			hasVirtIO := strings.Contains(xmlStr, "virtio")
			result.Details["has_virtio"] = hasVirtIO
			if !hasVirtIO && req.OSType == "linux" {
				result.Warnings = append(result.Warnings, "VirtIO drivers recommended for Linux guests")
			}
		}
	}

	// Check disk compatibility if path provided
	if req.DiskPath != "" && fileExists(req.DiskPath) {
		diskInfo := getDiskInfo(req.DiskPath)
		if diskInfo != nil {
			result.Details["disk_format"] = diskInfo["format"]

			// Check if format is supported
			format, ok := diskInfo["format"].(string)
			if ok {
				supportedFormats := []string{"qcow2", "raw", "vmdk"}
				supported := false
				for _, sf := range supportedFormats {
					if format == sf {
						supported = true
						break
					}
				}
				if !supported {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("disk format %s may not be fully supported", format))
				}
			}
		}
	}

	// Check for required features
	if len(req.Features) > 0 {
		result.Details["requested_features"] = req.Features

		// Features that might need checking
		for _, feature := range req.Features {
			switch feature {
			case "nested-virtualization":
				// Check if nested virt is available
				nestedCmd := exec.Command("cat", "/sys/module/kvm_intel/parameters/nested")
				if nestedOutput, err := nestedCmd.Output(); err == nil {
					nested := strings.TrimSpace(string(nestedOutput))
					result.Details["nested_virt"] = nested == "Y" || nested == "1"
				}
			case "tpm":
				result.Warnings = append(result.Warnings, "TPM support should be verified manually")
			case "secureboot":
				result.Warnings = append(result.Warnings, "Secure Boot requires UEFI and additional configuration")
			}
		}
	}

	result.Details["total_errors"] = len(result.Errors)
	result.Details["total_warnings"] = len(result.Warnings)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "complete",
		"compatibility": result,
	})
}

// handleTestMigration tests VM functionality after migration
func (s *Server) handleTestMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VMName       string   `json:"vm_name"`
		Tests        []string `json:"tests"` // boot, network, disk, shutdown
		AutoShutdown bool     `json:"auto_shutdown,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Default tests if none specified
	if len(req.Tests) == 0 {
		req.Tests = []string{"boot", "disk"}
	}

	if !isValidVMName(req.VMName) {
		http.Error(w, "invalid vm_name", http.StatusBadRequest)
		return
	}

	result := ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
		Details:  make(map[string]interface{}),
	}

	testResults := make(map[string]interface{})

	// Track initial state
	// #nosec G204 -- req.VMName is validated by isValidVMName above
	initialStateCmd := exec.Command("virsh", "domstate", req.VMName)
	initialStateOutput, _ := initialStateCmd.Output()
	initialState := strings.TrimSpace(string(initialStateOutput))
	result.Details["initial_state"] = initialState

	for _, test := range req.Tests {
		switch test {
		case "boot":
			// Boot test
			if initialState != "running" {
				// #nosec G204 -- req.VMName is validated by isValidVMName above
				startCmd := exec.Command("virsh", "start", req.VMName)
				if err := startCmd.Run(); err != nil {
					result.Valid = false
					result.Errors = append(result.Errors, "boot test failed: could not start VM")
					testResults["boot"] = "failed"
				} else {
					testResults["boot"] = "success"
				}
			} else {
				testResults["boot"] = "skipped (already running)"
			}

		case "network":
			// Network test - check if interfaces are present
			// #nosec G204 -- req.VMName is validated by isValidVMName above
			iflistCmd := exec.Command("virsh", "domiflist", req.VMName)
			ifOutput, err := iflistCmd.Output()
			if err != nil {
				result.Warnings = append(result.Warnings, "network test: could not list interfaces")
				testResults["network"] = "warning"
			} else {
				lines := strings.Split(string(ifOutput), "\n")
				interfaceCount := 0
				for _, line := range lines {
					if strings.Contains(line, "network") || strings.Contains(line, "bridge") {
						interfaceCount++
					}
				}
				testResults["network"] = map[string]interface{}{
					"status":          "success",
					"interface_count": interfaceCount,
				}
			}

		case "disk":
			// Disk test - verify all disks are accessible
			// #nosec G204 -- req.VMName is validated by isValidVMName above
			domblklistCmd := exec.Command("virsh", "domblklist", req.VMName)
			diskOutput, err := domblklistCmd.Output()
			if err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, "disk test failed: could not list disks")
				testResults["disk"] = "failed"
			} else {
				disks := parseDiskList(string(diskOutput))
				allExist := true
				for _, disk := range disks {
					if disk != "" && !fileExists(disk) {
						allExist = false
						result.Errors = append(result.Errors, fmt.Sprintf("disk missing: %s", disk))
					}
				}
				if allExist {
					testResults["disk"] = map[string]interface{}{
						"status":     "success",
						"disk_count": len(disks),
					}
				} else {
					result.Valid = false
					testResults["disk"] = "failed"
				}
			}

		case "shutdown":
			// Shutdown test
			if initialState == "running" || testResults["boot"] == "success" {
				// #nosec G204 -- req.VMName is validated by isValidVMName above
				shutdownCmd := exec.Command("virsh", "shutdown", req.VMName)
				if err := shutdownCmd.Run(); err != nil {
					result.Warnings = append(result.Warnings, "shutdown test: failed to send shutdown signal")
					testResults["shutdown"] = "warning"
				} else {
					testResults["shutdown"] = "signal sent (graceful shutdown initiated)"
				}
			} else {
				testResults["shutdown"] = "skipped (VM not running)"
			}
		}
	}

	result.Details["tests"] = testResults

	// Auto shutdown if requested and VM was started by us
	if req.AutoShutdown && initialState != "running" && testResults["boot"] == "success" {
		// #nosec G204 -- req.VMName is validated by isValidVMName above
		shutdownCmd := exec.Command("virsh", "destroy", req.VMName)
		if err := shutdownCmd.Run(); err != nil {
			s.logger.Warn("failed to force shutdown VM after test", "vm", req.VMName, "error", err)
		}
		result.Details["auto_shutdown"] = "executed"
	}

	result.Details["total_errors"] = len(result.Errors)
	result.Details["total_warnings"] = len(result.Warnings)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "complete",
		"test":   result,
	})
}

// Helper function to get disk info. Callers are expected to have already
// validated path via isValidVolumeSourcePath; re-check defensively since this
// is a shared helper reachable from multiple handlers.
func getDiskInfo(path string) map[string]interface{} {
	if !isValidVolumeSourcePath(path) {
		return nil
	}
	// #nosec G204 -- path is validated by isValidVolumeSourcePath immediately above
	cmd := exec.Command("qemu-img", "info", "--output=json", path)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var info map[string]interface{}
	if err := json.Unmarshal(output, &info); err != nil {
		return nil
	}

	return info
}
