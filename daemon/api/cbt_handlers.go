// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/providers/incremental"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// CBTEnableRequest represents a request to enable CBT on a VM
type CBTEnableRequest struct {
	VMPath string `json:"vm_path"`
}

// CBTEnableResponse represents the response after enabling CBT
type CBTEnableResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// CBTStatusRequest represents a request to check CBT status
type CBTStatusRequest struct {
	VMPath string `json:"vm_path"`
}

// CBTStatusResponse represents the CBT status for a VM
type CBTStatusResponse struct {
	VMPath         string                       `json:"vm_path"`
	CBTEnabled     bool                         `json:"cbt_enabled"`
	Disks          []incremental.DiskMetadata   `json:"disks"`
	LastExport     *incremental.ExportMetadata  `json:"last_export,omitempty"`
	CanIncremental bool                         `json:"can_incremental"`
	Reason         string                       `json:"reason,omitempty"`
}

// IncrementalAnalysisRequest represents a request to analyze incremental export potential
type IncrementalAnalysisRequest struct {
	VMPath string `json:"vm_path"`
}

// IncrementalAnalysisResponse represents the analysis of incremental export potential
type IncrementalAnalysisResponse struct {
	VMPath            string                      `json:"vm_path"`
	CanIncremental    bool                        `json:"can_incremental"`
	Reason            string                      `json:"reason"`
	LastExport        *incremental.ExportMetadata `json:"last_export,omitempty"`
	CurrentDisks      []incremental.DiskMetadata  `json:"current_disks"`
	EstimatedSavings  int64                       `json:"estimated_savings_bytes"`
	EstimatedDuration string                      `json:"estimated_duration"`
}

// handleEnableCBT handles POST /cbt/enable
func (s *Server) handleEnableCBT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CBTEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.VMPath == "" {
		http.Error(w, "vm_path is required", http.StatusBadRequest)
		return
	}

	// Create vSphere client
	cfg := config.FromEnvironment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := vsphere.NewVSphereClient(ctx, cfg, s.logger)
	if err != nil {
		s.logger.Error("failed to create vSphere client", "error", err)
		respondJSON(w, CBTEnableResponse{
			Success: false,
			Message: "Failed to connect to vSphere",
			Error:   err.Error(),
		})
		return
	}
	defer func() { _ = client.Close() }()

	// Enable CBT
	if err := client.EnableCBT(ctx, req.VMPath); err != nil {
		s.logger.Error("failed to enable CBT", "vm", req.VMPath, "error", err)
		respondJSON(w, CBTEnableResponse{
			Success: false,
			Message: "Failed to enable CBT",
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, CBTEnableResponse{
		Success: true,
		Message: fmt.Sprintf("CBT enabled successfully for %s", req.VMPath),
	})
}

// handleDisableCBT handles POST /cbt/disable
func (s *Server) handleDisableCBT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CBTEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.VMPath == "" {
		http.Error(w, "vm_path is required", http.StatusBadRequest)
		return
	}

	// Create vSphere client
	cfg := config.FromEnvironment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := vsphere.NewVSphereClient(ctx, cfg, s.logger)
	if err != nil {
		s.logger.Error("failed to create vSphere client", "error", err)
		respondJSON(w, CBTEnableResponse{
			Success: false,
			Message: "Failed to connect to vSphere",
			Error:   err.Error(),
		})
		return
	}
	defer func() { _ = client.Close() }()

	// Disable CBT
	if err := client.DisableCBT(ctx, req.VMPath); err != nil {
		s.logger.Error("failed to disable CBT", "vm", req.VMPath, "error", err)
		respondJSON(w, CBTEnableResponse{
			Success: false,
			Message: "Failed to disable CBT",
			Error:   err.Error(),
		})
		return
	}

	respondJSON(w, CBTEnableResponse{
		Success: true,
		Message: fmt.Sprintf("CBT disabled successfully for %s", req.VMPath),
	})
}

// handleCBTStatus handles GET /cbt/status
func (s *Server) handleCBTStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CBTStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.VMPath == "" {
		http.Error(w, "vm_path is required", http.StatusBadRequest)
		return
	}

	// Create vSphere client
	cfg := config.FromEnvironment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := vsphere.NewVSphereClient(ctx, cfg, s.logger)
	if err != nil {
		s.logger.Error("failed to create vSphere client", "error", err)
		http.Error(w, fmt.Sprintf("Failed to connect to vSphere: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() { _ = client.Close() }()

	// Check CBT status
	cbtEnabled, err := client.IsCBTEnabled(ctx, req.VMPath)
	if err != nil {
		s.logger.Error("failed to check CBT status", "vm", req.VMPath, "error", err)
		http.Error(w, fmt.Sprintf("Failed to check CBT status: %v", err), http.StatusInternalServerError)
		return
	}

	// Get disk metadata
	disks, err := client.GetDiskMetadata(ctx, req.VMPath)
	if err != nil {
		s.logger.Error("failed to get disk metadata", "vm", req.VMPath, "error", err)
		http.Error(w, fmt.Sprintf("Failed to get disk metadata: %v", err), http.StatusInternalServerError)
		return
	}

	// Create change tracker to check for previous exports
	tracker, err := incremental.NewChangeTracker("~/.transiva/incremental", s.logger)
	if err != nil {
		s.logger.Warn("failed to create change tracker", "error", err)
		respondJSON(w, CBTStatusResponse{
			VMPath:     req.VMPath,
			CBTEnabled: cbtEnabled,
			Disks:      disks,
		})
		return
	}

	// Get last export metadata
	vmID := sanitizeVMPath(req.VMPath)
	lastExport, err := tracker.GetLastExport(ctx, vmID)
	if err != nil {
		s.logger.Warn("failed to get last export", "error", err)
	}

	// Check if incremental is possible
	canIncremental, reason := tracker.IsIncrementalPossible(ctx, vmID, disks)

	respondJSON(w, CBTStatusResponse{
		VMPath:         req.VMPath,
		CBTEnabled:     cbtEnabled,
		Disks:          disks,
		LastExport:     lastExport,
		CanIncremental: canIncremental,
		Reason:         reason,
	})
}

// handleIncrementalAnalysis handles POST /incremental/analyze
func (s *Server) handleIncrementalAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IncrementalAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.VMPath == "" {
		http.Error(w, "vm_path is required", http.StatusBadRequest)
		return
	}

	// Create vSphere client
	cfg := config.FromEnvironment()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := vsphere.NewVSphereClient(ctx, cfg, s.logger)
	if err != nil {
		s.logger.Error("failed to create vSphere client", "error", err)
		http.Error(w, fmt.Sprintf("Failed to connect to vSphere: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() { _ = client.Close() }()

	// Get disk metadata
	disks, err := client.GetDiskMetadata(ctx, req.VMPath)
	if err != nil {
		s.logger.Error("failed to get disk metadata", "vm", req.VMPath, "error", err)
		http.Error(w, fmt.Sprintf("Failed to get disk metadata: %v", err), http.StatusInternalServerError)
		return
	}

	// Create change tracker
	tracker, err := incremental.NewChangeTracker("~/.transiva/incremental", s.logger)
	if err != nil {
		s.logger.Error("failed to create change tracker", "error", err)
		http.Error(w, fmt.Sprintf("Failed to create change tracker: %v", err), http.StatusInternalServerError)
		return
	}

	// Get last export metadata
	vmID := sanitizeVMPath(req.VMPath)
	lastExport, err := tracker.GetLastExport(ctx, vmID)
	if err != nil {
		s.logger.Warn("failed to get last export", "error", err)
	}

	// Check if incremental is possible
	canIncremental, reason := tracker.IsIncrementalPossible(ctx, vmID, disks)

	// Estimate savings
	estimatedSavings, err := tracker.EstimateChangedSize(ctx, vmID, disks)
	if err != nil {
		s.logger.Warn("failed to estimate savings", "error", err)
		estimatedSavings = 0
	}

	// Estimate duration based on changed size (rough estimate: 100 MB/s)
	estimatedDuration := "unknown"
	if estimatedSavings > 0 {
		seconds := estimatedSavings / (100 * 1024 * 1024) // 100 MB/s
		if seconds < 60 {
			estimatedDuration = fmt.Sprintf("%d seconds", seconds)
		} else if seconds < 3600 {
			estimatedDuration = fmt.Sprintf("%d minutes", seconds/60)
		} else {
			estimatedDuration = fmt.Sprintf("%.1f hours", float64(seconds)/3600)
		}
	}

	respondJSON(w, IncrementalAnalysisResponse{
		VMPath:            req.VMPath,
		CanIncremental:    canIncremental,
		Reason:            reason,
		LastExport:        lastExport,
		CurrentDisks:      disks,
		EstimatedSavings:  estimatedSavings,
		EstimatedDuration: estimatedDuration,
	})
}

// sanitizeVMPath converts a VM path to a safe VM ID
func sanitizeVMPath(vmPath string) string {
	// Simple implementation - could be enhanced
	return vmPath
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("failed to encode JSON response: %v\n", err)
	}
}
