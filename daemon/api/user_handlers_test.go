// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// List Users Handler Tests

func TestHandleListUsersMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	w := httptest.NewRecorder()

	server.handleListUsers(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListUsers(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	server.handleListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["users"]; !ok {
		t.Error("Expected users field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 2 {
		t.Errorf("Expected total=2, got %v", total)
	}

	// Verify user structure
	users := response["users"].([]interface{})
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}

	// Check first user (admin)
	admin := users[0].(map[string]interface{})
	if admin["username"] != "admin" {
		t.Errorf("Expected username='admin', got %v", admin["username"])
	}
	if admin["email"] != "admin@example.com" {
		t.Errorf("Expected email='admin@example.com', got %v", admin["email"])
	}
	if admin["role"] != "administrator" {
		t.Errorf("Expected role='administrator', got %v", admin["role"])
	}
	if admin["status"] != "active" {
		t.Errorf("Expected status='active', got %v", admin["status"])
	}

	// Check second user (operator)
	operator := users[1].(map[string]interface{})
	if operator["username"] != "operator1" {
		t.Errorf("Expected username='operator1', got %v", operator["username"])
	}
	if operator["role"] != "operator" {
		t.Errorf("Expected role='operator', got %v", operator["role"])
	}
}

// Create User Handler Tests

func TestHandleCreateUserMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	server.handleCreateUser(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateUserInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/users",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateUser(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateUserValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := User{
		Username: "newuser",
		Email:    "newuser@example.com",
		Role:     "viewer",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/users",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateUser(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response User
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Username != "newuser" {
		t.Errorf("Expected username='newuser', got %s", response.Username)
	}
	if response.Email != "newuser@example.com" {
		t.Errorf("Expected email='newuser@example.com', got %s", response.Email)
	}
	if response.Role != "viewer" {
		t.Errorf("Expected role='viewer', got %s", response.Role)
	}
	if response.Status != "active" {
		t.Errorf("Expected status='active', got %s", response.Status)
	}
	if response.ID == "" {
		t.Error("Expected ID to be set")
	}
	if response.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestHandleCreateUserDifferentRoles(t *testing.T) {
	server := setupTestBasicServer(t)

	tests := []struct {
		name string
		role string
	}{
		{"Administrator", "admin"},
		{"Operator", "operator"},
		{"Viewer", "viewer"},
		{"Auditor", "auditor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := User{
				Username: strings.ToLower(tt.name),
				Email:    strings.ToLower(tt.name) + "@example.com",
				Role:     tt.role,
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/users",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleCreateUser(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", w.Code)
			}

			var response User
			json.Unmarshal(w.Body.Bytes(), &response)

			if response.Role != tt.role {
				t.Errorf("Expected role='%s', got %s", tt.role, response.Role)
			}
		})
	}
}

// List Roles Handler Tests

func TestHandleListRolesMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/roles", nil)
	w := httptest.NewRecorder()

	server.handleListRoles(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListRoles(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	w := httptest.NewRecorder()

	server.handleListRoles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["roles"]; !ok {
		t.Error("Expected roles field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 4 {
		t.Errorf("Expected total=4, got %v", total)
	}

	// Verify roles
	roles := response["roles"].([]interface{})
	if len(roles) != 4 {
		t.Errorf("Expected 4 roles, got %d", len(roles))
	}

	// Check Administrator role
	admin := roles[0].(map[string]interface{})
	if admin["name"] != "Administrator" {
		t.Errorf("Expected name='Administrator', got %v", admin["name"])
	}
	if admin["description"] != "Full system access" {
		t.Errorf("Expected description='Full system access', got %v", admin["description"])
	}
	adminPerms := admin["permissions"].([]interface{})
	if len(adminPerms) != 1 || adminPerms[0] != "*" {
		t.Error("Expected Administrator to have wildcard permission")
	}

	// Check Operator role
	operator := roles[1].(map[string]interface{})
	if operator["name"] != "Operator" {
		t.Errorf("Expected name='Operator', got %v", operator["name"])
	}
	operatorPerms := operator["permissions"].([]interface{})
	if len(operatorPerms) != 3 {
		t.Errorf("Expected 3 permissions for Operator, got %d", len(operatorPerms))
	}

	// Check Viewer role
	viewer := roles[2].(map[string]interface{})
	if viewer["name"] != "Viewer" {
		t.Errorf("Expected name='Viewer', got %v", viewer["name"])
	}
	viewerPerms := viewer["permissions"].([]interface{})
	if len(viewerPerms) != 2 {
		t.Errorf("Expected 2 permissions for Viewer, got %d", len(viewerPerms))
	}

	// Check Auditor role
	auditor := roles[3].(map[string]interface{})
	if auditor["name"] != "Auditor" {
		t.Errorf("Expected name='Auditor', got %v", auditor["name"])
	}
	if auditor["description"] != "Access to logs and compliance" {
		t.Errorf("Expected description='Access to logs and compliance', got %v", auditor["description"])
	}
	auditorPerms := auditor["permissions"].([]interface{})
	if len(auditorPerms) != 3 {
		t.Errorf("Expected 3 permissions for Auditor, got %d", len(auditorPerms))
	}
}

// List API Keys Handler Tests

func TestHandleListAPIKeysMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api-keys", nil)
	w := httptest.NewRecorder()

	server.handleListAPIKeys(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListAPIKeys(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
	w := httptest.NewRecorder()

	server.handleListAPIKeys(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["keys"]; !ok {
		t.Error("Expected keys field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 1 {
		t.Errorf("Expected total=1, got %v", total)
	}

	// Verify API key structure
	keys := response["keys"].([]interface{})
	if len(keys) != 1 {
		t.Errorf("Expected 1 key, got %d", len(keys))
	}

	// Check key details
	key := keys[0].(map[string]interface{})
	if key["name"] != "Production API" {
		t.Errorf("Expected name='Production API', got %v", key["name"])
	}
	keyValue := key["key"].(string)
	if !strings.Contains(keyValue, "***") {
		t.Error("Expected key to be masked")
	}
	if !strings.HasPrefix(keyValue, "hsd_") {
		t.Error("Expected key to start with hsd_ prefix")
	}
}

// Generate API Key Handler Tests

func TestHandleGenerateAPIKeyMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
	w := httptest.NewRecorder()

	server.handleGenerateAPIKey(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGenerateAPIKeyInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api-keys",
		bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleGenerateAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGenerateAPIKeyValid(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]string{
		"name": "Test API Key",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api-keys",
		bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleGenerateAPIKey(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response APIKey
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Name != "Test API Key" {
		t.Errorf("Expected name='Test API Key', got %s", response.Name)
	}
	if response.ID == "" {
		t.Error("Expected ID to be set")
	}
	if !strings.HasPrefix(response.Key, "hsd_") {
		t.Error("Expected key to start with hsd_ prefix")
	}
	if len(response.Key) < 10 {
		t.Error("Expected key to have sufficient length")
	}
	if response.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestHandleGenerateAPIKeyMultiple(t *testing.T) {
	server := setupTestBasicServer(t)

	keyNames := []string{"Dev API", "Prod API", "Test API"}
	generatedKeys := make(map[string]bool)

	for _, name := range keyNames {
		t.Run(name, func(t *testing.T) {
			reqBody := map[string]string{
				"name": name,
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/api-keys",
				bytes.NewReader(body))
			w := httptest.NewRecorder()

			server.handleGenerateAPIKey(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", w.Code)
			}

			var response APIKey
			json.Unmarshal(w.Body.Bytes(), &response)

			// Verify each key is unique
			if generatedKeys[response.Key] {
				t.Error("Generated duplicate API key")
			}
			generatedKeys[response.Key] = true
		})
	}
}

// List Sessions Handler Tests

func TestHandleListSessionsMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/sessions", nil)
	w := httptest.NewRecorder()

	server.handleListSessions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListSessions(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	w := httptest.NewRecorder()

	server.handleListSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := response["sessions"]; !ok {
		t.Error("Expected sessions field in response")
	}
	if _, ok := response["total"]; !ok {
		t.Error("Expected total field in response")
	}

	total := response["total"].(float64)
	if total != 2 {
		t.Errorf("Expected total=2, got %v", total)
	}

	// Verify session structure
	sessions := response["sessions"].([]interface{})
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	// Check first session (admin)
	adminSession := sessions[0].(map[string]interface{})
	if adminSession["username"] != "admin" {
		t.Errorf("Expected username='admin', got %v", adminSession["username"])
	}
	if adminSession["ip_address"] != "192.168.1.100" {
		t.Errorf("Expected ip_address='192.168.1.100', got %v", adminSession["ip_address"])
	}
	if adminSession["location"] != "New York, US" {
		t.Errorf("Expected location='New York, US', got %v", adminSession["location"])
	}

	// Check second session (operator)
	operatorSession := sessions[1].(map[string]interface{})
	if operatorSession["username"] != "operator1" {
		t.Errorf("Expected username='operator1', got %v", operatorSession["username"])
	}
	if operatorSession["ip_address"] != "192.168.1.101" {
		t.Errorf("Expected ip_address='192.168.1.101', got %v", operatorSession["ip_address"])
	}
	if operatorSession["location"] != "San Francisco, US" {
		t.Errorf("Expected location='San Francisco, US', got %v", operatorSession["location"])
	}
}

// Helper Function Tests

func TestGenerateRandomString(t *testing.T) {
	tests := []int{8, 16, 32, 64}

	for _, length := range tests {
		t.Run(string(rune(length)), func(t *testing.T) {
			result := generateRandomString(length)

			if len(result) != length {
				t.Errorf("Expected length=%d, got %d", length, len(result))
			}

			// Verify only contains valid characters
			const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			for _, char := range result {
				if !strings.ContainsRune(charset, char) {
					t.Errorf("Invalid character in generated string: %c", char)
				}
			}
		})
	}
}

func TestGenerateRandomStringUniqueness(t *testing.T) {
	// Generate multiple strings and verify they're different
	generated := make(map[string]bool)
	length := 32

	for i := 0; i < 10; i++ {
		result := generateRandomString(length)
		if generated[result] {
			t.Error("Generated duplicate random string")
		}
		generated[result] = true
	}
}
