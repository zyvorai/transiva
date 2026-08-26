// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// Schedule represents a scheduled job
type Schedule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CronExpr    string    `json:"cron_expr"`
	NextRun     time.Time `json:"next_run"`
	Status      string    `json:"status"`
	JobTemplate string    `json:"job_template"`
	CreatedAt   time.Time `json:"created_at"`
}

// BackupPolicy represents an automated backup policy
type BackupPolicy struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Frequency  string   `json:"frequency"` // daily, weekly, monthly
	Retention  int      `json:"retention"` // days
	TargetTags []string `json:"target_tags"`
	Enabled    bool     `json:"enabled"`
}

// Workflow represents an automation workflow
type Workflow struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Steps []string `json:"steps"`
}

// handleListSchedules lists all scheduled jobs
func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Demo data - in production, load from database
	schedules := []Schedule{
		{
			ID:       "sched-1",
			Name:     "Daily Backup",
			CronExpr: "0 2 * * *",
			NextRun:  time.Now().Add(24 * time.Hour),
			Status:   "active",
		},
		{
			ID:       "sched-2",
			Name:     "Weekly Full Export",
			CronExpr: "0 0 * * 0",
			NextRun:  time.Now().Add(7 * 24 * time.Hour),
			Status:   "active",
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"schedules": schedules,
		"total":     len(schedules),
	})
}

// handleCreateSchedule creates a new scheduled job
func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Schedule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate cron expression
	if req.CronExpr == "" {
		http.Error(w, "cron expression required", http.StatusBadRequest)
		return
	}

	req.ID = "sched-" + time.Now().Format("20060102150405")
	req.CreatedAt = time.Now()
	req.Status = "active"

	s.jsonResponse(w, http.StatusCreated, req)
}

// handleListBackupPolicies lists all backup policies
func (s *Server) handleListBackupPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	policies := []BackupPolicy{
		{
			ID:         "policy-1",
			Name:       "Production Backup",
			Frequency:  "daily",
			Retention:  30,
			TargetTags: []string{"production", "critical"},
			Enabled:    true,
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"policies": policies,
		"total":    len(policies),
	})
}

// handleCreateBackupPolicy creates a new backup policy
func (s *Server) handleCreateBackupPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var policy BackupPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	policy.ID = "policy-" + time.Now().Format("20060102150405")
	policy.Enabled = true

	s.jsonResponse(w, http.StatusCreated, policy)
}

// handleListWorkflows lists all workflows
func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflows := []Workflow{
		{
			ID:    "workflow-1",
			Name:  "Export → Convert → Import",
			Steps: []string{"export", "convert", "import"},
		},
		{
			ID:    "workflow-2",
			Name:  "Snapshot → Export → Verify",
			Steps: []string{"snapshot", "export", "verify"},
		},
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"workflows": workflows,
		"total":     len(workflows),
	})
}
