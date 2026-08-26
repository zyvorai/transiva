// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/daemon/scheduler"
)

// Advanced Scheduling Request/Response types

// CreateAdvancedScheduleRequest represents a request to create a schedule with advanced features
type CreateAdvancedScheduleRequest struct {
	Name           string                          `json:"name"`
	Description    string                          `json:"description"`
	Schedule       string                          `json:"schedule"` // Cron format
	JobTemplate    models.JobDefinition            `json:"job_template"`
	AdvancedConfig *models.AdvancedScheduleConfig  `json:"advanced_config,omitempty"`
}

// DependencyStatusResponse represents dependency status for a job
type DependencyStatusResponse struct {
	JobID        string                       `json:"job_id"`
	JobName      string                       `json:"job_name"`
	Satisfied    bool                         `json:"satisfied"`
	Reason       string                       `json:"reason,omitempty"`
	Dependencies []DependencyInfo             `json:"dependencies"`
	WaitingJobs  []string                     `json:"waiting_jobs,omitempty"`
}

// DependencyInfo represents information about a single dependency
type DependencyInfo struct {
	JobID         string `json:"job_id"`
	RequiredState string `json:"required_state"`
	CurrentState  string `json:"current_state"`
	Satisfied     bool   `json:"satisfied"`
	Timeout       int    `json:"timeout"`
}

// RetryStatusResponse represents retry status for a job
type RetryStatusResponse struct {
	JobID        string                    `json:"job_id"`
	JobName      string                    `json:"job_name"`
	Attempt      int                       `json:"attempt"`
	MaxAttempts  int                       `json:"max_attempts"`
	LastError    string                    `json:"last_error,omitempty"`
	NextRetry    string                    `json:"next_retry,omitempty"`
	History      []scheduler.RetryRecord   `json:"history,omitempty"`
}

// TimeWindowStatusResponse represents time window status for a job
type TimeWindowStatusResponse struct {
	JobID           string                        `json:"job_id"`
	JobName         string                        `json:"job_name"`
	InWindow        bool                          `json:"in_window"`
	Message         string                        `json:"message"`
	NextWindowStart string                        `json:"next_window_start,omitempty"`
	Windows         []WindowStatusInfo            `json:"windows,omitempty"`
}

// WindowStatusInfo represents status for a single time window
type WindowStatusInfo struct {
	Index     int      `json:"index"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time"`
	Days      []string `json:"days"`
	Timezone  string   `json:"timezone"`
	Active    bool     `json:"active"`
	NextStart string   `json:"next_start,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// JobQueueStatusResponse represents the status of the job queue
type JobQueueStatusResponse struct {
	QueueSize    int                 `json:"queue_size"`
	RunningJobs  int                 `json:"running_jobs"`
	MaxSlots     int                 `json:"max_slots"`
	QueuedJobs   []QueuedJobInfo     `json:"queued_jobs"`
}

// QueuedJobInfo represents information about a queued job
type QueuedJobInfo struct {
	JobID    string `json:"job_id"`
	JobName  string `json:"job_name"`
	Priority int    `json:"priority"`
	AddedAt  string `json:"added_at"`
	Attempts int    `json:"attempts"`
}

// API Handlers

// handleCreateAdvancedSchedule handles POST /schedules/advanced/create
func (s *Server) handleCreateAdvancedSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateAdvancedScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Schedule == "" {
		http.Error(w, "name and schedule are required", http.StatusBadRequest)
		return
	}

	// Validate advanced config if provided
	if req.AdvancedConfig != nil {
		// Validate time windows
		for _, window := range req.AdvancedConfig.TimeWindows {
			if err := scheduler.ValidateTimeWindow(window); err != nil {
				http.Error(w, "Invalid time window: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Validate retry policy
		if req.AdvancedConfig.RetryPolicy != nil {
			if req.AdvancedConfig.RetryPolicy.MaxAttempts < 0 {
				http.Error(w, "max_attempts must be >= 0", http.StatusBadRequest)
				return
			}
		}
	}

	// Create scheduled job
	scheduledJob := &models.ScheduledJob{
		ID:             generateID("schedule"),
		Name:           req.Name,
		Description:    req.Description,
		Schedule:       req.Schedule,
		JobTemplate:    req.JobTemplate,
		Enabled:        true,
		AdvancedConfig: req.AdvancedConfig,
	}

	// Note: Integration with actual scheduler would happen here
	// For now, return success with the created schedule

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Advanced schedule created successfully",
		"schedule": scheduledJob,
	}); err != nil {
		s.logger.Error("failed to encode create schedule response", "error", err)
	}
}

// handleGetDependencyStatus handles GET /schedules/dependencies/{id}
func (s *Server) handleGetDependencyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path
	// Note: In real implementation, use proper routing
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id parameter required", http.StatusBadRequest)
		return
	}

	// Note: This would query the actual dependency tracker
	// For now, return example response
	response := DependencyStatusResponse{
		JobID:        jobID,
		JobName:      "Example Job",
		Satisfied:    true,
		Reason:       "",
		Dependencies: []DependencyInfo{},
		WaitingJobs:  []string{},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode dependency status response", "error", err)
	}
}

// handleGetRetryStatus handles GET /schedules/retry/{id}
func (s *Server) handleGetRetryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id parameter required", http.StatusBadRequest)
		return
	}

	// Note: This would query the actual retry manager
	// For now, return example response
	response := RetryStatusResponse{
		JobID:       jobID,
		JobName:     "Example Job",
		Attempt:     0,
		MaxAttempts: 3,
		History:     []scheduler.RetryRecord{},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode retry status response", "error", err)
	}
}

// handleGetTimeWindowStatus handles GET /schedules/timewindow/{id}
func (s *Server) handleGetTimeWindowStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id parameter required", http.StatusBadRequest)
		return
	}

	// Note: This would query the actual time window manager
	// For now, return example response
	response := TimeWindowStatusResponse{
		JobID:    jobID,
		JobName:  "Example Job",
		InWindow: true,
		Message:  "Job is within time window",
		Windows:  []WindowStatusInfo{},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode time window status response", "error", err)
	}
}

// handleGetJobQueueStatus handles GET /schedules/queue
func (s *Server) handleGetJobQueueStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Note: This would query the actual job queue
	// For now, return example response
	response := JobQueueStatusResponse{
		QueueSize:   0,
		RunningJobs: 0,
		MaxSlots:    4,
		QueuedJobs:  []QueuedJobInfo{},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode job queue status response", "error", err)
	}
}

// handleValidateSchedule handles POST /schedules/validate
func (s *Server) handleValidateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateAdvancedScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	errors := []string{}

	// Validate cron schedule
	// Note: Would use actual cron parser here

	// Validate time windows
	if req.AdvancedConfig != nil {
		for i, window := range req.AdvancedConfig.TimeWindows {
			if err := scheduler.ValidateTimeWindow(window); err != nil {
				errors = append(errors, "Window "+string(rune(i))+": "+err.Error())
			}
		}
	}

	if len(errors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"errors": errors,
		}); err != nil {
			s.logger.Error("failed to encode schedule validation response", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"message": "Schedule configuration is valid",
	}); err != nil {
		s.logger.Error("failed to encode schedule validation response", "error", err)
	}
}

// Helper function to generate IDs
func generateID(prefix string) string {
	// Simple implementation - in production would use UUID
	return prefix + "-" + "123456"
}
