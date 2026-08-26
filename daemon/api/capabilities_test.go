// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/logger"
)

func TestHandleCapabilities_Success(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	// Run detection
	ctx := context.Background()
	if err := detector.Detect(ctx); err != nil {
		t.Fatalf("Failed to detect capabilities: %v", err)
	}

	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, "localhost:0", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	w := httptest.NewRecorder()

	server.handleCapabilities(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check response structure
	if _, ok := response["capabilities"]; !ok {
		t.Error("Response missing 'capabilities' field")
	}

	if _, ok := response["default_method"]; !ok {
		t.Error("Response missing 'default_method' field")
	}

	if _, ok := response["timestamp"]; !ok {
		t.Error("Response missing 'timestamp' field")
	}

	// Verify capabilities structure
	caps, ok := response["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("capabilities is not a map")
	}

	// Should have all 4 methods
	expectedMethods := []string{"ctl", "govc", "ovftool", "web"}
	for _, method := range expectedMethods {
		if _, ok := caps[method]; !ok {
			t.Errorf("Missing capability for method: %s", method)
		}
	}

	// Verify default_method
	defaultMethod, ok := response["default_method"].(string)
	if !ok || defaultMethod == "" {
		t.Error("default_method is not a valid string")
	}
}

func TestHandleCapabilities_MethodNotAllowed(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)
	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, "localhost:0", nil, nil)

	// Test POST method (should be rejected)
	req := httptest.NewRequest(http.MethodPost, "/capabilities", nil)
	w := httptest.NewRecorder()

	server.handleCapabilities(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCapabilities_ContentType(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)
	_ = detector.Detect(context.Background())

	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, "localhost:0", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	w := httptest.NewRecorder()

	server.handleCapabilities(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestHandleCapabilities_VerifyCapabilityFields(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)
	_ = detector.Detect(context.Background())

	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, "localhost:0", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	w := httptest.NewRecorder()

	server.handleCapabilities(w, req)

	var response struct {
		Capabilities map[string]struct {
			Method      string `json:"method"`
			Available   bool   `json:"available"`
			Version     string `json:"version"`
			Path        string `json:"path"`
			Priority    int    `json:"priority"`
			LastChecked string `json:"last_checked"`
		} `json:"capabilities"`
		DefaultMethod string `json:"default_method"`
		Timestamp     string `json:"timestamp"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify web capability (always available)
	webCap, ok := response.Capabilities["web"]
	if !ok {
		t.Fatal("Web capability not found")
	}

	if webCap.Method != "web" {
		t.Errorf("Expected method 'web', got %s", webCap.Method)
	}

	if !webCap.Available {
		t.Error("Web method should always be available")
	}

	if webCap.Priority != 4 {
		t.Errorf("Expected priority 4 for web, got %d", webCap.Priority)
	}

	if webCap.Path != "internal" {
		t.Errorf("Expected path 'internal', got %s", webCap.Path)
	}

	if webCap.Version != "built-in" {
		t.Errorf("Expected version 'built-in', got %s", webCap.Version)
	}
}

func TestHandleCapabilities_DefaultMethodPriority(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)
	_ = detector.Detect(context.Background())

	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, "localhost:0", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	w := httptest.NewRecorder()

	server.handleCapabilities(w, req)

	var response struct {
		Capabilities map[string]struct {
			Available bool `json:"available"`
			Priority  int  `json:"priority"`
		} `json:"capabilities"`
		DefaultMethod string `json:"default_method"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Find the capability with lowest priority number (highest priority)
	lowestPriority := 999
	var expectedDefault string

	for method, cap := range response.Capabilities {
		if cap.Available && cap.Priority < lowestPriority {
			lowestPriority = cap.Priority
			expectedDefault = method
		}
	}

	if response.DefaultMethod != expectedDefault {
		t.Errorf("Default method should be %s (priority %d), got %s",
			expectedDefault, lowestPriority, response.DefaultMethod)
	}
}

func TestHandleCapabilities_MultipleRequests(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)
	_ = detector.Detect(context.Background())

	manager := jobs.NewManager(log, detector)
	server := NewServer(manager, detector, log, "localhost:0", nil, nil)

	// Make multiple requests to ensure consistent results
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
		w := httptest.NewRecorder()

		server.handleCapabilities(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i, w.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Request %d: Failed to decode response: %v", i, err)
		}

		if _, ok := response["capabilities"]; !ok {
			t.Errorf("Request %d: Missing capabilities field", i)
		}
	}
}
