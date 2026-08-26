// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
)

// Handle GET /schedules - List all scheduled jobs
func (es *EnhancedServer) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	schedules := es.scheduler.ListScheduledJobs()
	es.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"schedules": schedules,
		"total":     len(schedules),
		"timestamp": time.Now(),
	})
}

// Handle POST /schedules - Create new scheduled job
func (es *EnhancedServer) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		es.errorResponse(w, http.StatusBadRequest, "failed to read request body: %v", err)
		return
	}

	var schedule models.ScheduledJob
	if err := json.Unmarshal(body, &schedule); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}

	// Validate required fields
	if schedule.ID == "" {
		es.errorResponse(w, http.StatusBadRequest, "schedule ID is required")
		return
	}
	if schedule.Schedule == "" {
		es.errorResponse(w, http.StatusBadRequest, "schedule cron expression is required")
		return
	}
	if schedule.JobTemplate.VMPath == "" {
		es.errorResponse(w, http.StatusBadRequest, "job template vm_path is required")
		return
	}

	// Add schedule
	if err := es.scheduler.AddScheduledJob(&schedule); err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to add schedule: %v", err)
		return
	}

	es.logger.Info("schedule created", "id", schedule.ID, "name", schedule.Name)
	es.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"message":  "schedule created successfully",
		"schedule": schedule,
	})
}

// Handle GET /schedules/{id} - Get specific scheduled job
func (es *EnhancedServer) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/schedules/")
	if id == "" {
		es.errorResponse(w, http.StatusBadRequest, "schedule ID is required")
		return
	}

	schedule, err := es.scheduler.GetScheduledJob(id)
	if err != nil {
		es.errorResponse(w, http.StatusNotFound, "schedule not found: %v", err)
		return
	}

	es.jsonResponse(w, http.StatusOK, schedule)
}

// Handle PUT /schedules/{id} - Update scheduled job
func (es *EnhancedServer) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/schedules/")
	if id == "" {
		es.errorResponse(w, http.StatusBadRequest, "schedule ID is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		es.errorResponse(w, http.StatusBadRequest, "failed to read request body: %v", err)
		return
	}

	var updates models.ScheduledJob
	if err := json.Unmarshal(body, &updates); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid JSON: %v", err)
		return
	}

	if err := es.scheduler.UpdateScheduledJob(id, &updates); err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to update schedule: %v", err)
		return
	}

	es.logger.Info("schedule updated", "id", id)
	es.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "schedule updated successfully",
	})
}

// Handle DELETE /schedules/{id} - Delete scheduled job
func (es *EnhancedServer) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/schedules/")
	if id == "" {
		es.errorResponse(w, http.StatusBadRequest, "schedule ID is required")
		return
	}

	if err := es.scheduler.RemoveScheduledJob(id); err != nil {
		es.errorResponse(w, http.StatusNotFound, "failed to delete schedule: %v", err)
		return
	}

	es.logger.Info("schedule deleted", "id", id)
	es.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "schedule deleted successfully",
	})
}

// Handle POST /schedules/{id}/enable - Enable scheduled job
func (es *EnhancedServer) handleEnableSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	// Extract ID from path
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		es.errorResponse(w, http.StatusBadRequest, "invalid path")
		return
	}
	id := parts[1]

	if err := es.scheduler.EnableScheduledJob(id); err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to enable schedule: %v", err)
		return
	}

	es.logger.Info("schedule enabled", "id", id)
	es.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "schedule enabled successfully",
	})
}

// Handle POST /schedules/{id}/disable - Disable scheduled job
func (es *EnhancedServer) handleDisableSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	// Extract ID from path
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		es.errorResponse(w, http.StatusBadRequest, "invalid path")
		return
	}
	id := parts[1]

	if err := es.scheduler.DisableScheduledJob(id); err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to disable schedule: %v", err)
		return
	}

	es.logger.Info("schedule disabled", "id", id)
	es.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "schedule disabled successfully",
	})
}

// Handle POST /schedules/{id}/trigger - Manually trigger scheduled job
func (es *EnhancedServer) handleTriggerSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	// Extract ID from path
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		es.errorResponse(w, http.StatusBadRequest, "invalid path")
		return
	}
	id := parts[1]

	if err := es.scheduler.TriggerNow(id); err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to trigger schedule: %v", err)
		return
	}

	es.logger.Info("schedule triggered manually", "id", id)
	es.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "schedule triggered successfully",
	})
}

// Handle GET /schedules/stats - Get schedule statistics
func (es *EnhancedServer) handleScheduleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if es.scheduler == nil {
		es.errorResponse(w, http.StatusServiceUnavailable, "scheduler not enabled")
		return
	}

	stats := es.scheduler.GetScheduleStats()
	es.jsonResponse(w, http.StatusOK, stats)
}
