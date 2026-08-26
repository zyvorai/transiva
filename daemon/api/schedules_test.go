// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func setupTestServer(t *testing.T) *EnhancedServer {
	log := logger.New("error") // Use error level to reduce test output
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)
	config := &Config{}
	config.Metrics.Enabled = false

	server, err := NewEnhancedServer(manager, detector, log, ":8080", config, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return server
}

func TestHandleListSchedules(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add test schedule
	testSchedule := &models.ScheduledJob{
		ID:       "test-schedule-1",
		Name:     "Test Schedule",
		Schedule: "0 2 * * *",
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			VMPath: "test/vm",
		},
	}
	server.scheduler.AddScheduledJob(testSchedule)

	// Test GET request
	req := httptest.NewRequest(http.MethodGet, "/schedules", nil)
	w := httptest.NewRecorder()

	server.handleListSchedules(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if schedules, ok := response["schedules"].([]interface{}); ok {
		if len(schedules) != 1 {
			t.Errorf("Expected 1 schedule, got %d", len(schedules))
		}
	} else {
		t.Error("Response missing schedules array")
	}
}

func TestHandleListSchedulesMethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodPost, "/schedules", nil)
	w := httptest.NewRecorder()

	server.handleListSchedules(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleCreateSchedule(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	schedule := models.ScheduledJob{
		ID:       "new-schedule",
		Name:     "New Schedule",
		Schedule: "*/15 * * * *",
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			VMPath:     "test/vm",
			OutputPath: "/tmp/output",
		},
	}

	body, _ := json.Marshal(schedule)
	req := httptest.NewRequest(http.MethodPost, "/schedules", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateSchedule(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify schedule was added
	retrievedSchedule, err := server.scheduler.GetScheduledJob("new-schedule")
	if err != nil {
		t.Errorf("Failed to retrieve created schedule: %v", err)
	}
	if retrievedSchedule.Name != "New Schedule" {
		t.Errorf("Schedule name mismatch")
	}
}

func TestHandleCreateScheduleInvalidJSON(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodPost, "/schedules", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleCreateSchedule(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleCreateScheduleMissingID(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	schedule := models.ScheduledJob{
		// Missing ID
		Name:     "Test",
		Schedule: "0 0 * * *",
	}

	body, _ := json.Marshal(schedule)
	req := httptest.NewRequest(http.MethodPost, "/schedules", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleCreateSchedule(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleGetSchedule(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add test schedule
	testSchedule := &models.ScheduledJob{
		ID:       "get-test",
		Name:     "Get Test",
		Schedule: "0 2 * * *",
		Enabled:  true,
	}
	server.scheduler.AddScheduledJob(testSchedule)

	req := httptest.NewRequest(http.MethodGet, "/schedules/get-test", nil)
	w := httptest.NewRecorder()

	server.handleGetSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var schedule models.ScheduledJob
	if err := json.Unmarshal(w.Body.Bytes(), &schedule); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if schedule.ID != "get-test" {
		t.Errorf("ID mismatch: expected 'get-test', got '%s'", schedule.ID)
	}
}

func TestHandleGetScheduleNotFound(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	req := httptest.NewRequest(http.MethodGet, "/schedules/nonexistent", nil)
	w := httptest.NewRecorder()

	server.handleGetSchedule(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleUpdateSchedule(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add initial schedule
	testSchedule := &models.ScheduledJob{
		ID:       "update-test",
		Name:     "Original Name",
		Schedule: "0 2 * * *",
		Enabled:  true,
	}
	server.scheduler.AddScheduledJob(testSchedule)

	// Update schedule
	updates := models.ScheduledJob{
		Name:     "Updated Name",
		Schedule: "0 3 * * *",
		Enabled:  false,
	}

	body, _ := json.Marshal(updates)
	req := httptest.NewRequest(http.MethodPut, "/schedules/update-test", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleUpdateSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify update
	updated, err := server.scheduler.GetScheduledJob("update-test")
	if err != nil {
		t.Fatalf("Failed to get updated schedule: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("Name not updated: got '%s'", updated.Name)
	}
}

func TestHandleDeleteSchedule(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add schedule to delete
	testSchedule := &models.ScheduledJob{
		ID:       "delete-test",
		Name:     "To Delete",
		Schedule: "0 2 * * *",
		Enabled:  true,
	}
	server.scheduler.AddScheduledJob(testSchedule)

	req := httptest.NewRequest(http.MethodDelete, "/schedules/delete-test", nil)
	w := httptest.NewRecorder()

	server.handleDeleteSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deletion
	_, err := server.scheduler.GetScheduledJob("delete-test")
	if err == nil {
		t.Error("Schedule should have been deleted")
	}
}

func TestHandleEnableSchedule(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add disabled schedule
	testSchedule := &models.ScheduledJob{
		ID:       "enable-test",
		Name:     "Enable Test",
		Schedule: "0 2 * * *",
		Enabled:  false,
	}
	server.scheduler.AddScheduledJob(testSchedule)

	req := httptest.NewRequest(http.MethodPost, "/schedules/enable-test/enable", nil)
	w := httptest.NewRecorder()

	server.handleEnableSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify enabled
	schedule, _ := server.scheduler.GetScheduledJob("enable-test")
	if !schedule.Enabled {
		t.Error("Schedule should be enabled")
	}
}

func TestHandleDisableSchedule(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add enabled schedule
	testSchedule := &models.ScheduledJob{
		ID:       "disable-test",
		Name:     "Disable Test",
		Schedule: "0 2 * * *",
		Enabled:  true,
	}
	server.scheduler.AddScheduledJob(testSchedule)

	req := httptest.NewRequest(http.MethodPost, "/schedules/disable-test/disable", nil)
	w := httptest.NewRecorder()

	server.handleDisableSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify disabled
	schedule, _ := server.scheduler.GetScheduledJob("disable-test")
	if schedule.Enabled {
		t.Error("Schedule should be disabled")
	}
}

func TestHandleTriggerSchedule(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add schedule
	testSchedule := &models.ScheduledJob{
		ID:       "trigger-test",
		Name:     "Trigger Test",
		Schedule: "0 2 * * *",
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			VMPath:     "test/vm",
			OutputPath: "/tmp/test",
		},
	}
	server.scheduler.AddScheduledJob(testSchedule)

	req := httptest.NewRequest(http.MethodPost, "/schedules/trigger-test/trigger", nil)
	w := httptest.NewRecorder()

	server.handleTriggerSchedule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleScheduleStats(t *testing.T) {
	server := setupTestServer(t)
	defer server.scheduler.Stop()

	// Add some schedules
	for i := 0; i < 3; i++ {
		schedule := &models.ScheduledJob{
			ID:       "stats-test-" + string(rune('a'+i)),
			Name:     "Stats Test",
			Schedule: "0 2 * * *",
			Enabled:  i < 2, // 2 enabled, 1 disabled
		}
		server.scheduler.AddScheduledJob(schedule)
	}

	req := httptest.NewRequest(http.MethodGet, "/schedules/stats", nil)
	w := httptest.NewRecorder()

	server.handleScheduleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to parse stats: %v", err)
	}

	// Check that stats contain expected fields
	if _, ok := stats["total_schedules"]; !ok {
		t.Error("Stats missing total_schedules")
	}
}

func TestHandleScheduleWithNoScheduler(t *testing.T) {
	log := logger.New("error")
	detector := capabilities.NewDetector(log)
	ctx := context.Background()
	detector.Detect(ctx)
	manager := jobs.NewManager(log, detector)

	// Create server without scheduler
	server := &EnhancedServer{
		Server:    NewServer(manager, detector, log, ":8080", nil, nil),
		scheduler: nil, // No scheduler
	}

	req := httptest.NewRequest(http.MethodGet, "/schedules", nil)
	w := httptest.NewRecorder()

	server.handleListSchedules(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}
