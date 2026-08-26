// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListCloudProvidersMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/cloud/providers", nil)
	w := httptest.NewRecorder()

	server.handleListCloudProviders(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListCloudProviders(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/cloud/providers", nil)
	w := httptest.NewRecorder()

	server.handleListCloudProviders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	providers, ok := response["providers"].([]interface{})
	if !ok {
		t.Fatal("Expected providers array in response")
	}

	if len(providers) == 0 {
		t.Error("Expected at least one cloud provider")
	}
}

func TestHandleConfigureCloudProviderMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/cloud/providers/configure", nil)
	w := httptest.NewRecorder()

	server.handleConfigureCloudProvider(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleConfigureCloudProviderInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/cloud/providers/configure", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleConfigureCloudProvider(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleConfigureCloudProvider(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"provider": "aws",
		"enabled":  true,
		"config": map[string]interface{}{
			"region":    "us-west-2",
			"s3_bucket": "test-bucket",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/cloud/providers/configure", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleConfigureCloudProvider(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleListVCenterServersMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/cloud/vcenter", nil)
	w := httptest.NewRecorder()

	server.handleListVCenterServers(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListVCenterServers(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/cloud/vcenter", nil)
	w := httptest.NewRecorder()

	server.handleListVCenterServers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	servers, ok := response["servers"].([]interface{})
	if !ok {
		t.Fatal("Expected servers array in response")
	}

	// May be empty if no vCenter servers configured
	if servers == nil {
		t.Error("Expected servers array, got nil")
	}
}

func TestHandleAddVCenterServerMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/cloud/vcenter/add", nil)
	w := httptest.NewRecorder()

	server.handleAddVCenterServer(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleAddVCenterServerInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/cloud/vcenter/add", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleAddVCenterServer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleAddVCenterServer(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name":     "test-vcenter",
		"hostname": "vcenter.example.com",
		"username": "admin",
		"password": "password",
		"insecure": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/cloud/vcenter/add", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleAddVCenterServer(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleListIntegrationsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/integrations", nil)
	w := httptest.NewRecorder()

	server.handleListIntegrations(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListIntegrations(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/integrations", nil)
	w := httptest.NewRecorder()

	server.handleListIntegrations(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	integrations, ok := response["integrations"].([]interface{})
	if !ok {
		t.Fatal("Expected integrations array in response")
	}

	if len(integrations) == 0 {
		t.Error("Expected at least one integration")
	}
}

func TestHandleConfigureIntegrationMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/integrations/configure", nil)
	w := httptest.NewRecorder()

	server.handleConfigureIntegration(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleConfigureIntegrationInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/integrations/configure", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleConfigureIntegration(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleConfigureIntegration(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"name":    "jenkins",
		"enabled": true,
		"config": map[string]interface{}{
			"url":   "https://jenkins.example.com",
			"token": "secret-token",
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/integrations/configure", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleConfigureIntegration(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}
