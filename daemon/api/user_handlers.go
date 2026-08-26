// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// User represents a system user
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"` // admin, operator, viewer, auditor
	Status    string    `json:"status"`
	LastLogin time.Time `json:"last_login"`
	CreatedAt time.Time `json:"created_at"`
}

// Role represents an RBAC role
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// APIKey represents an API key
type APIKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"` // masked in responses
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`
}

// Session represents an active user session
type Session struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	IPAddress string    `json:"ip_address"`
	Location  string    `json:"location"`
	LoginTime time.Time `json:"login_time"`
}

// handleListUsers lists all users
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users := []User{
		{
			ID:        "user-1",
			Username:  "admin",
			Email:     "admin@example.com",
			Role:      "administrator",
			Status:    "active",
			LastLogin: time.Now().Add(-15 * time.Minute),
			CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
		},
		{
			ID:        "user-2",
			Username:  "operator1",
			Email:     "operator@example.com",
			Role:      "operator",
			Status:    "active",
			LastLogin: time.Now().Add(-1 * time.Hour),
			CreatedAt: time.Now().Add(-15 * 24 * time.Hour),
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

// handleCreateUser creates a new user
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user.ID = "user-" + time.Now().Format("20060102150405")
	user.Status = "active"
	user.CreatedAt = time.Now()

	s.jsonResponse(w, http.StatusCreated, user)
}

// handleListRoles lists all roles
func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	roles := []Role{
		{
			ID:          "role-1",
			Name:        "Administrator",
			Description: "Full system access",
			Permissions: []string{"*"},
		},
		{
			ID:          "role-2",
			Name:        "Operator",
			Description: "Export/import VMs, manage jobs",
			Permissions: []string{"vm.export", "vm.import", "job.*"},
		},
		{
			ID:          "role-3",
			Name:        "Viewer",
			Description: "Read-only access",
			Permissions: []string{"vm.view", "job.view"},
		},
		{
			ID:          "role-4",
			Name:        "Auditor",
			Description: "Access to logs and compliance",
			Permissions: []string{"audit.*", "logs.*", "compliance.*"},
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"roles": roles,
		"total": len(roles),
	})
}

// handleListAPIKeys lists all API keys
func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keys := []APIKey{
		{
			ID:        "key-1",
			Name:      "Production API",
			Key:       "hsd_abc123***",
			CreatedAt: time.Now().Add(-5 * 24 * time.Hour),
			LastUsed:  time.Now().Add(-2 * time.Hour),
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"keys":  keys,
		"total": len(keys),
	})
}

// handleGenerateAPIKey generates a new API key
func (s *Server) handleGenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	key := APIKey{
		ID:        "key-" + time.Now().Format("20060102150405"),
		Name:      req.Name,
		Key:       "hsd_" + generateRandomString(32),
		CreatedAt: time.Now(),
	}

	s.jsonResponse(w, http.StatusCreated, key)
}

// handleListSessions lists active sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions := []Session{
		{
			ID:        "sess-1",
			Username:  "admin",
			IPAddress: "192.168.1.100",
			Location:  "New York, US",
			LoginTime: time.Now().Add(-15 * time.Minute),
		},
		{
			ID:        "sess-2",
			Username:  "operator1",
			IPAddress: "192.168.1.101",
			Location:  "San Francisco, US",
			LoginTime: time.Now().Add(-1 * time.Hour),
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// generateRandomString generates a random string of given length
func generateRandomString(length int) string {
	// Simple implementation for demo
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}
