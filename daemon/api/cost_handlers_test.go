// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetCostSummaryMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/costs/summary", nil)
	w := httptest.NewRecorder()

	server.handleGetCostSummary(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetCostSummary(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/costs/summary", nil)
	w := httptest.NewRecorder()

	server.handleGetCostSummary(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var summary CostSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if summary.CurrentMonth <= 0 {
		t.Error("Expected positive current month cost")
	}

	if summary.Breakdown == nil {
		t.Error("Expected breakdown field in response")
	}

	if len(summary.Breakdown) == 0 {
		t.Error("Expected at least one breakdown entry")
	}
}

func TestHandleGetBudgetConfigMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/costs/budget", nil)
	w := httptest.NewRecorder()

	server.handleGetBudgetConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetBudgetConfig(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/costs/budget", nil)
	w := httptest.NewRecorder()

	server.handleGetBudgetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var config BudgetConfig
	if err := json.Unmarshal(w.Body.Bytes(), &config); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if config.ID == "" {
		t.Error("Expected non-empty budget ID")
	}

	if config.MonthlyBudget <= 0 {
		t.Error("Expected positive monthly budget")
	}

	if config.AlertThreshold <= 0 || config.AlertThreshold > 100 {
		t.Errorf("Expected alert threshold between 0 and 100, got %f", config.AlertThreshold)
	}
}

func TestHandleUpdateBudgetConfigMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/costs/budget", nil)
	w := httptest.NewRecorder()

	server.handleUpdateBudgetConfig(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleUpdateBudgetConfigInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPut, "/costs/budget", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleUpdateBudgetConfig(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleUpdateBudgetConfig(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"monthly_budget":  5000.00,
		"alert_threshold": 85.0,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/costs/budget", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleUpdateBudgetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", response["status"])
	}
}

func TestHandleGetCostHistoryMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/costs/history", nil)
	w := httptest.NewRecorder()

	server.handleGetCostHistory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetCostHistory(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/costs/history", nil)
	w := httptest.NewRecorder()

	server.handleGetCostHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	historyData, ok := response["history"].([]interface{})
	if !ok {
		t.Fatal("Expected history array in response")
	}

	if len(historyData) == 0 {
		t.Error("Expected at least one history entry")
	}

	for i, entry := range historyData {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			t.Errorf("Entry %d: expected map", i)
			continue
		}

		if entryMap["month"] == nil || entryMap["month"] == "" {
			t.Errorf("Entry %d: expected non-empty month", i)
		}

		if budget, ok := entryMap["budget"].(float64); !ok || budget <= 0 {
			t.Errorf("Entry %d: expected positive budget", i)
		}
	}
}

func TestHandleGetCostHistoryFields(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/costs/history", nil)
	w := httptest.NewRecorder()

	server.handleGetCostHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	historyData, ok := response["history"].([]interface{})
	if !ok {
		t.Fatal("Expected history array in response")
	}

	if len(historyData) > 0 {
		firstEntry := historyData[0].(map[string]interface{})

		if firstEntry["month"] == nil || firstEntry["month"] == "" {
			t.Error("Expected month field to be non-empty")
		}

		// Check variance field exists and is a number
		if firstEntry["variance"] == nil {
			t.Error("Expected variance field to be present")
		}

		budget := firstEntry["budget"].(float64)
		actual := firstEntry["actual"].(float64)
		variance := firstEntry["variance"].(float64)

		// Variance represents actual - budget (negative means under budget)
		if budget > 0 && actual > 0 {
			// Just verify the variance is consistent with budget and actual
			if (variance > 0 && actual < budget) || (variance < 0 && actual > budget) {
				t.Errorf("Variance %f inconsistent with budget %f and actual %f", variance, budget, actual)
			}
		}
	}
}
