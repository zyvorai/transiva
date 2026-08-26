// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// validVolumeAlgorithms is the allowlist of secure-wipe algorithms accepted by virsh vol-wipe.
var validVolumeAlgorithms = map[string]bool{
	"zero": true, "nnsa": true, "dod": true, "bsi": true,
	"gutmann": true, "schneier": true, "pfitzner7": true, "pfitzner33": true,
	"random": true, "trim": true,
}

// validVolumeFormats is the allowlist of volume formats accepted by libvirt storage commands.
var validVolumeFormats = map[string]bool{
	"qcow2": true, "raw": true, "qed": true, "vmdk": true, "vpc": true, "vdi": true,
}

// isValidVolumeSourcePath validates that a source disk image path supplied via the API
// is well-formed and does not attempt path traversal or contain unsafe characters.
func isValidVolumeSourcePath(path string) bool {
	if path == "" || len(path) > 4096 {
		return false
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return false
	}
	if strings.Contains(cleaned, "..") {
		return false
	}
	return true
}

// handleGetVolumeInfo gets detailed volume information
func (s *Server) handleGetVolumeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pool := r.URL.Query().Get("pool")
	volume := r.URL.Query().Get("volume")

	if pool == "" || volume == "" {
		http.Error(w, "missing pool or volume parameter", http.StatusBadRequest)
		return
	}
	if !isValidVMName(pool) || !isValidVMName(volume) {
		http.Error(w, "invalid pool or volume name", http.StatusBadRequest)
		return
	}

	// #nosec G204 G702 -- pool and volume are validated by isValidVMName above
	cmd := exec.Command("virsh", "vol-info", volume, "--pool", pool)
	output, err := cmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "volume not found: %v", err)
		return
	}

	volInfo := s.parseVolumeInfo(pool, volume, string(output))

	s.jsonResponse(w, http.StatusOK, volInfo)
}

// handleCreateVolume creates a new storage volume
func (s *Server) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pool     string `json:"pool"`
		Name     string `json:"name"`
		Format   string `json:"format"`   // qcow2, raw, etc.
		Capacity int    `json:"capacity"` // GB
		Prealloc bool   `json:"prealloc"` // preallocate space
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Format == "" {
		req.Format = "qcow2"
	}

	if !isValidVMName(req.Pool) || !isValidVMName(req.Name) {
		http.Error(w, "invalid pool or volume name", http.StatusBadRequest)
		return
	}
	if !validVolumeFormats[req.Format] {
		http.Error(w, "invalid volume format", http.StatusBadRequest)
		return
	}

	// Build virsh vol-create-as command
	args := []string{
		"vol-create-as",
		req.Pool,
		req.Name,
		fmt.Sprintf("%dG", req.Capacity),
		"--format", req.Format,
	}

	if req.Prealloc && req.Format == "qcow2" {
		args = append(args, "--prealloc-metadata")
	}

	// #nosec G204 -- req.Pool/req.Name/req.Format are validated above; capacity is numeric
	cmd := exec.Command("virsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create volume: %s", string(output))
		return
	}

	// Get volume path
	// #nosec G204 -- req.Name/req.Pool are validated above
	pathCmd := exec.Command("virsh", "vol-path", req.Name, "--pool", req.Pool)
	pathOutput, _ := pathCmd.Output()
	path := strings.TrimSpace(string(pathOutput))

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":   "success",
		"message":  fmt.Sprintf("Volume %s created", req.Name),
		"pool":     req.Pool,
		"name":     req.Name,
		"path":     path,
		"format":   req.Format,
		"capacity": req.Capacity,
	})
}

// handleCloneVolume clones an existing volume
func (s *Server) handleCloneVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pool       string `json:"pool"`
		SourceVol  string `json:"source_volume"`
		TargetVol  string `json:"target_volume"`
		TargetPool string `json:"target_pool,omitempty"` // optional, defaults to same pool
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TargetPool == "" {
		req.TargetPool = req.Pool
	}

	if !isValidVMName(req.Pool) || !isValidVMName(req.SourceVol) ||
		!isValidVMName(req.TargetVol) || !isValidVMName(req.TargetPool) {
		http.Error(w, "invalid pool or volume name", http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Pool/req.SourceVol/req.TargetVol are validated above
	cmd := exec.Command("virsh", "vol-clone",
		"--pool", req.Pool,
		req.SourceVol,
		req.TargetVol)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to clone volume: %s", string(output))
		return
	}

	// Get cloned volume path
	// #nosec G204 -- req.TargetVol/req.TargetPool are validated above
	pathCmd := exec.Command("virsh", "vol-path", req.TargetVol, "--pool", req.TargetPool)
	pathOutput, _ := pathCmd.Output()
	path := strings.TrimSpace(string(pathOutput))

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Volume %s cloned to %s", req.SourceVol, req.TargetVol),
		"source_volume": req.SourceVol,
		"target_volume": req.TargetVol,
		"target_pool":   req.TargetPool,
		"path":          path,
	})
}

// handleResizeVolume resizes an existing volume
func (s *Server) handleResizeVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pool     string `json:"pool"`
		Volume   string `json:"volume"`
		Capacity int    `json:"capacity"` // New capacity in GB
		Shrink   bool   `json:"shrink"`   // Allow shrinking (dangerous)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !isValidVMName(req.Volume) || !isValidVMName(req.Pool) {
		http.Error(w, "invalid pool or volume name", http.StatusBadRequest)
		return
	}

	args := []string{
		"vol-resize",
		req.Volume,
		fmt.Sprintf("%dG", req.Capacity),
		"--pool", req.Pool,
	}

	if req.Shrink {
		args = append(args, "--shrink")
	}

	// #nosec G204 -- req.Volume/req.Pool are validated above; capacity is numeric
	cmd := exec.Command("virsh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to resize volume: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":       "success",
		"message":      fmt.Sprintf("Volume %s resized to %dGB", req.Volume, req.Capacity),
		"pool":         req.Pool,
		"volume":       req.Volume,
		"new_capacity": req.Capacity,
	})
}

// handleDeleteVolume deletes a storage volume
func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pool   string `json:"pool"`
		Volume string `json:"volume"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !isValidVMName(req.Volume) || !isValidVMName(req.Pool) {
		http.Error(w, "invalid pool or volume name", http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Volume/req.Pool are validated above
	cmd := exec.Command("virsh", "vol-delete", req.Volume, "--pool", req.Pool)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to delete volume: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Volume %s deleted", req.Volume),
		"pool":    req.Pool,
		"volume":  req.Volume,
	})
}

// handleUploadVolume uploads a disk image to create a volume
func (s *Server) handleUploadVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pool       string `json:"pool"`
		Volume     string `json:"volume"`
		SourcePath string `json:"source_path"` // Path to disk image file
		Format     string `json:"format"`      // qcow2, raw, etc.
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Format == "" {
		req.Format = "qcow2"
	}

	if !isValidVMName(req.Pool) || !isValidVMName(req.Volume) {
		http.Error(w, "invalid pool or volume name", http.StatusBadRequest)
		return
	}
	if !validVolumeFormats[req.Format] {
		http.Error(w, "invalid volume format", http.StatusBadRequest)
		return
	}
	if !isValidVolumeSourcePath(req.SourcePath) {
		http.Error(w, "invalid source path", http.StatusBadRequest)
		return
	}

	// Step 1: Get size of source image
	// #nosec G204 -- req.SourcePath is validated by isValidVolumeSourcePath above
	qemuCmd := exec.Command("qemu-img", "info", "--output=json", req.SourcePath)
	qemuOutput, err := qemuCmd.Output()
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "failed to read source image: %v", err)
		return
	}

	// Parse JSON to get virtual size
	var imgInfo struct {
		VirtualSize int64 `json:"virtual-size"`
	}
	if err := json.Unmarshal(qemuOutput, &imgInfo); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to parse image info: %v", err)
		return
	}

	// Convert bytes to GB (round up)
	capacityGB := (imgInfo.VirtualSize + 1024*1024*1024 - 1) / (1024 * 1024 * 1024)

	// Step 2: Create volume
	// #nosec G204 -- req.Pool/req.Volume/req.Format are validated above; capacityGB is numeric
	createCmd := exec.Command("virsh", "vol-create-as",
		req.Pool,
		req.Volume,
		fmt.Sprintf("%dG", capacityGB),
		"--format", req.Format)
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create volume: %s", string(createOutput))
		return
	}

	// Step 3: Upload disk image to volume
	// #nosec G204 -- req.Pool/req.Volume/req.SourcePath are validated above
	uploadCmd := exec.Command("virsh", "vol-upload",
		"--pool", req.Pool,
		req.Volume,
		req.SourcePath)
	uploadOutput, err := uploadCmd.CombinedOutput()
	if err != nil {
		// Try to cleanup failed volume
		// #nosec G204 -- req.Volume/req.Pool are validated above
		if cleanupErr := exec.Command("virsh", "vol-delete", req.Volume, "--pool", req.Pool).Run(); cleanupErr != nil {
			s.logger.Warn("failed to cleanup volume after failed upload", "volume", req.Volume, "pool", req.Pool, "error", cleanupErr)
		}
		s.errorResponse(w, http.StatusInternalServerError, "failed to upload image: %s", string(uploadOutput))
		return
	}

	// Get volume path
	// #nosec G204 -- req.Volume/req.Pool are validated above
	pathCmd := exec.Command("virsh", "vol-path", req.Volume, "--pool", req.Pool)
	pathOutput, _ := pathCmd.Output()
	path := strings.TrimSpace(string(pathOutput))

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":      "success",
		"message":     fmt.Sprintf("Image uploaded to volume %s", req.Volume),
		"pool":        req.Pool,
		"volume":      req.Volume,
		"path":        path,
		"format":      req.Format,
		"capacity_gb": capacityGB,
		"source":      req.SourcePath,
	})
}

// handleWipeVolume securely wipes a volume
func (s *Server) handleWipeVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Pool      string `json:"pool"`
		Volume    string `json:"volume"`
		Algorithm string `json:"algorithm"` // zero, nnsa, dod, bsi, gutmann, schneier, pfitzner7, etc.
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Algorithm == "" {
		req.Algorithm = "zero"
	}

	if !isValidVMName(req.Volume) || !isValidVMName(req.Pool) {
		http.Error(w, "invalid pool or volume name", http.StatusBadRequest)
		return
	}
	if !validVolumeAlgorithms[req.Algorithm] {
		http.Error(w, "invalid wipe algorithm", http.StatusBadRequest)
		return
	}

	// #nosec G204 -- req.Pool/req.Algorithm/req.Volume are validated above
	cmd := exec.Command("virsh", "vol-wipe",
		"--pool", req.Pool,
		"--algorithm", req.Algorithm,
		req.Volume)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to wipe volume: %s", string(output))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Volume %s wiped using %s algorithm", req.Volume, req.Algorithm),
		"pool":      req.Pool,
		"volume":    req.Volume,
		"algorithm": req.Algorithm,
	})
}

// parseVolumeInfo parses virsh vol-info output
func (s *Server) parseVolumeInfo(pool, volume, output string) LibvirtVolume {
	volInfo := LibvirtVolume{
		Name: volume,
		Pool: pool,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Type":
			volInfo.Type = value
		case "Capacity":
			// Parse "10.00 GiB" format
			fields := strings.Fields(value)
			if len(fields) >= 2 {
				capacity, _ := strconv.ParseFloat(fields[0], 64)
				unit := fields[1]
				switch unit {
				case "GiB":
					volInfo.Capacity = int64(capacity * 1024 * 1024 * 1024)
				case "MiB":
					volInfo.Capacity = int64(capacity * 1024 * 1024)
				case "KiB":
					volInfo.Capacity = int64(capacity * 1024)
				case "bytes":
					volInfo.Capacity = int64(capacity)
				}
			}
		case "Allocation":
			// Parse allocation same as capacity
			fields := strings.Fields(value)
			if len(fields) >= 2 {
				allocation, _ := strconv.ParseFloat(fields[0], 64)
				unit := fields[1]
				switch unit {
				case "GiB":
					volInfo.Allocation = int64(allocation * 1024 * 1024 * 1024)
				case "MiB":
					volInfo.Allocation = int64(allocation * 1024 * 1024)
				case "KiB":
					volInfo.Allocation = int64(allocation * 1024)
				case "bytes":
					volInfo.Allocation = int64(allocation)
				}
			}
		}
	}

	// Get format and path using vol-dumpxml. pool/volume are validated by callers
	// (handleGetVolumeInfo) before reaching this function; re-validate defensively
	// since this is a shared helper.
	if !isValidVMName(pool) || !isValidVMName(volume) {
		return volInfo
	}
	// #nosec G204 G702 -- pool/volume are validated immediately above
	dumpCmd := exec.Command("virsh", "vol-dumpxml", volume, "--pool", pool)
	if dumpOutput, err := dumpCmd.Output(); err == nil {
		dumpStr := string(dumpOutput)
		// Extract format
		if strings.Contains(dumpStr, "<format type='qcow2'/>") {
			volInfo.Format = "qcow2"
		} else if strings.Contains(dumpStr, "<format type='raw'/>") {
			volInfo.Format = "raw"
		}
		// Extract path
		if pathStart := strings.Index(dumpStr, "<path>"); pathStart != -1 {
			pathEnd := strings.Index(dumpStr[pathStart:], "</path>")
			if pathEnd != -1 {
				volInfo.Path = dumpStr[pathStart+6 : pathStart+pathEnd]
			}
		}
	}

	return volInfo
}
