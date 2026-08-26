// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"

	"github.com/zyvorai/transiva/providers/cost"
)

// Cost Estimation API Handlers

// handleEstimateCost handles POST /cost/estimate
func (s *Server) handleEstimateCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req cost.CostEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.StorageGB <= 0 {
		http.Error(w, "storage_gb must be greater than 0", http.StatusBadRequest)
		return
	}

	calculator := cost.NewCalculator(s.logger)
	estimate, err := calculator.EstimateCost(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(estimate); err != nil {
		s.logger.Error("failed to encode cost estimate response", "error", err)
	}
}

// handleCompareProviders handles POST /cost/compare
func (s *Server) handleCompareProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req cost.CostEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	calculator := cost.NewCalculator(s.logger)
	comparison, err := calculator.CompareProviders(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(comparison); err != nil {
		s.logger.Error("failed to encode cost comparison response", "error", err)
	}
}

// handleProjectYearlyCost handles POST /cost/project
func (s *Server) handleProjectYearlyCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req cost.CostEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	calculator := cost.NewCalculator(s.logger)
	projection, err := calculator.ProjectYearlyCost(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(projection); err != nil {
		s.logger.Error("failed to encode cost projection response", "error", err)
	}
}

// handleEstimateExportSize handles POST /cost/estimate-size
func (s *Server) handleEstimateExportSize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DiskSizeGB       float64 `json:"disk_size_gb"`
		Format           string  `json:"format"`
		IncludeSnapshots bool    `json:"include_snapshots"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	calculator := cost.NewCalculator(s.logger)
	estimate := calculator.EstimateExportSize(req.DiskSizeGB, req.Format, req.IncludeSnapshots)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(estimate); err != nil {
		s.logger.Error("failed to encode export size estimate response", "error", err)
	}
}
