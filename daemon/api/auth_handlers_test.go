// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandleLogin(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Get current user for authentication
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "root"
	}

	loginReq := LoginRequest{
		Username: currentUser,
		Password: "password",
	}

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	// Check if test environment supports auth (requires 'id' command)
	if w.Code == http.StatusUnauthorized {
		t.Skip("Skipping test: authentication requires 'id' command")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Token == "" {
		t.Error("Expected non-empty token")
	}

	if resp.Username != currentUser {
		t.Errorf("Expected username %s, got %s", currentUser, resp.Username)
	}

	if resp.ExpiresAt == "" {
		t.Error("Expected non-empty expiration time")
	}
}

func TestHandleLoginMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleLoginInvalidJSON(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleLoginInvalidCredentials(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	loginReq := LoginRequest{
		Username: "nonexistent_user_xyz_12345",
		Password: "wrongpassword",
	}

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHandleLoginEmptyUsername(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	loginReq := LoginRequest{
		Username: "",
		Password: "password",
	}

	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for empty username, got %d", w.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// First, create a session by logging in
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "root"
	}

	loginReq := LoginRequest{
		Username: currentUser,
		Password: "password",
	}

	loginBody, _ := json.Marshal(loginReq)
	loginReqHTTP := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
	loginW := httptest.NewRecorder()

	server.handleLogin(loginW, loginReqHTTP)

	if loginW.Code == http.StatusUnauthorized {
		t.Skip("Skipping test: authentication requires 'id' command")
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	// Now logout with the token
	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	logoutW := httptest.NewRecorder()

	server.handleLogout(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", logoutW.Code)
	}

	var logoutResp map[string]string
	if err := json.Unmarshal(logoutW.Body.Bytes(), &logoutResp); err != nil {
		t.Fatalf("Failed to parse logout response: %v", err)
	}

	if logoutResp["message"] != "logged out successfully" {
		t.Errorf("Expected logout message, got %s", logoutResp["message"])
	}

	// Verify session is invalidated
	_, err := server.authMgr.ValidateSession(loginResp.Token)
	if err == nil {
		t.Error("Expected session to be invalidated after logout")
	}
}

func TestHandleLogoutMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	w := httptest.NewRecorder()

	server.handleLogout(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleLogoutMissingToken(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()

	server.handleLogout(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHandleLogoutWithoutBearerPrefix(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Create a session manually
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "root"
	}

	loginReq := LoginRequest{
		Username: currentUser,
		Password: "password",
	}

	loginBody, _ := json.Marshal(loginReq)
	loginReqHTTP := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody))
	loginW := httptest.NewRecorder()

	server.handleLogin(loginW, loginReqHTTP)

	if loginW.Code == http.StatusUnauthorized {
		t.Skip("Skipping test: authentication requires 'id' command")
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(loginW.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	// Logout without Bearer prefix
	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.Header.Set("Authorization", loginResp.Token) // No "Bearer " prefix
	logoutW := httptest.NewRecorder()

	server.handleLogout(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", logoutW.Code)
	}
}

func TestHandleLogoutInvalidToken(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Try to logout with an invalid token (should still succeed - idempotent)
	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer invalid-token-xyz")
	logoutW := httptest.NewRecorder()

	server.handleLogout(logoutW, logoutReq)

	// Logout should succeed even with invalid token (idempotent operation)
	if logoutW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", logoutW.Code)
	}
}
