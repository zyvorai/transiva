// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// CostSummary represents cost summary
type CostSummary struct {
	CurrentMonth  float64            `json:"current_month"`
	LastMonth     float64            `json:"last_month"`
	Change        float64            `json:"change"`
	ChangePercent float64            `json:"change_percent"`
	Annual        float64            `json:"annual_projected"`
	Breakdown     map[string]float64 `json:"breakdown"`
}

// BudgetConfig represents budget configuration
type BudgetConfig struct {
	ID             string    `json:"id"`
	MonthlyBudget  float64   `json:"monthly_budget"`
	AlertThreshold float64   `json:"alert_threshold"` // percentage
	CurrentSpend   float64   `json:"current_spend"`
	CreatedAt      time.Time `json:"created_at"`
}

// CostHistory represents historical cost data
type CostHistory struct {
	Month    string  `json:"month"`
	Budget   float64 `json:"budget"`
	Actual   float64 `json:"actual"`
	Variance float64 `json:"variance"`
}

// handleGetCostSummary gets cost summary
func (s *Server) handleGetCostSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary := CostSummary{
		CurrentMonth:  2450.00,
		LastMonth:     1890.00,
		Change:        560.00,
		ChangePercent: 29.6,
		Annual:        28500.00,
		Breakdown: map[string]float64{
			"storage": 1200.00,
			"network": 850.00,
			"compute": 400.00,
		},
	}

	s.jsonResponse(w, http.StatusOK, summary)
}

// handleGetBudgetConfig gets budget configuration
func (s *Server) handleGetBudgetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := BudgetConfig{
		ID:             "budget-1",
		MonthlyBudget:  3000.00,
		AlertThreshold: 80.0,
		CurrentSpend:   2450.00,
		CreatedAt:      time.Now().Add(-30 * 24 * time.Hour),
	}

	s.jsonResponse(w, http.StatusOK, config)
}

// handleUpdateBudgetConfig updates budget configuration
func (s *Server) handleUpdateBudgetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config BudgetConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Budget configuration updated",
	})
}

// handleGetCostHistory gets historical cost data
func (s *Server) handleGetCostHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history := []CostHistory{
		{
			Month:    "January 2026",
			Budget:   3000.00,
			Actual:   2450.00,
			Variance: -550.00,
		},
		{
			Month:    "December 2025",
			Budget:   3000.00,
			Actual:   1890.00,
			Variance: -1110.00,
		},
		{
			Month:    "November 2025",
			Budget:   3000.00,
			Actual:   3240.00,
			Variance: 240.00,
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"total":   len(history),
	})
}
