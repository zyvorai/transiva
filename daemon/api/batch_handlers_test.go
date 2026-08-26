// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleBatchDeleteMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/batch/delete", nil)
	w := httptest.NewRecorder()

	server.handleBatchDelete(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBatchDeleteInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/batch/delete", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBatchDelete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleBatchDeleteValidRequest(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains":        []string{"test-vm-1", "test-vm-2"},
		"remove_storage": false,
		"snapshots_only": false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchDelete(w, req)

	// Should return 200 even if virsh commands fail (VMs don't exist in test environment)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "undefine" {
		t.Errorf("Expected operation 'undefine', got '%s'", result.Operation)
	}

	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
}

func TestHandleBatchDeleteSnapshotsOnly(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains":        []string{"test-vm"},
		"snapshots_only": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchDelete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleBatchDeleteRemoveStorage(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains":        []string{"test-vm"},
		"remove_storage": true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchDelete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleBatchPauseMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/batch/pause", nil)
	w := httptest.NewRecorder()

	server.handleBatchPause(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBatchPauseInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/batch/pause", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBatchPause(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleBatchPauseValidRequest(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm-1", "test-vm-2"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/pause", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchPause(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "suspend" {
		t.Errorf("Expected operation 'suspend', got '%s'", result.Operation)
	}

	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
}

func TestHandleBatchPauseEmptyDomains(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/pause", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchPause(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("Expected total 0, got %d", result.Total)
	}
}

func TestHandleBatchResumeMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/batch/resume", nil)
	w := httptest.NewRecorder()

	server.handleBatchResume(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBatchResumeInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/batch/resume", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBatchResume(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleBatchResumeValidRequest(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm-1", "test-vm-2", "test-vm-3"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/resume", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchResume(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "resume" {
		t.Errorf("Expected operation 'resume', got '%s'", result.Operation)
	}

	if result.Total != 3 {
		t.Errorf("Expected total 3, got %d", result.Total)
	}
}

func TestHandleBatchResumeSingleDomain(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"single-vm"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/resume", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchResume(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected total 1, got %d", result.Total)
	}
}

func TestParseSnapshotList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty output",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single snapshot",
			input:    "snapshot1",
			expected: []string{"snapshot1"},
		},
		{
			name:     "multiple snapshots",
			input:    "snapshot1\nsnapshot2\nsnapshot3",
			expected: []string{"snapshot1", "snapshot2", "snapshot3"},
		},
		{
			name:     "snapshots with whitespace",
			input:    "  snapshot1  \n  snapshot2  \n  snapshot3  ",
			expected: []string{"snapshot1", "snapshot2", "snapshot3"},
		},
		{
			name:     "snapshots with empty lines",
			input:    "snapshot1\n\nsnapshot2\n\n\nsnapshot3",
			expected: []string{"snapshot1", "snapshot2", "snapshot3"},
		},
		{
			name:     "mixed spacing",
			input:    "snapshot1\n  \nsnapshot2  \n\t\nsnapshot3\n",
			expected: []string{"snapshot1", "snapshot2", "snapshot3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSnapshotList(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d snapshots, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				if i >= len(result) {
					t.Errorf("Missing snapshot at index %d: expected '%s'", i, expected)
					continue
				}
				if result[i] != expected {
					t.Errorf("Snapshot at index %d: expected '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestParseSnapshotListWithComplexNames(t *testing.T) {
	input := "pre-update-snapshot\nbackup-2024-01-26\ntest_snapshot_v1"
	expected := []string{"pre-update-snapshot", "backup-2024-01-26", "test_snapshot_v1"}

	result := parseSnapshotList(input)

	if len(result) != len(expected) {
		t.Errorf("Expected %d snapshots, got %d", len(expected), len(result))
	}

	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("Expected snapshot '%s', got '%s'", exp, result[i])
		}
	}
}

// Batch Start Handler Tests

func TestHandleBatchStartMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/batch/start", nil)
	w := httptest.NewRecorder()

	server.handleBatchStart(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBatchStartInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/batch/start", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBatchStart(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleBatchStartValidRequest(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm-1", "test-vm-2"},
		"paused":  false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/start", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchStart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "start" {
		t.Errorf("Expected operation 'start', got '%s'", result.Operation)
	}

	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
}

func TestHandleBatchStartPaused(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm"},
		"paused":  true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/start", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchStart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected total 1, got %d", result.Total)
	}
}

func TestHandleBatchStartEmptyDomains(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/start", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchStart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("Expected total 0, got %d", result.Total)
	}
}

// Batch Stop Handler Tests

func TestHandleBatchStopMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/batch/stop", nil)
	w := httptest.NewRecorder()

	server.handleBatchStop(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBatchStopInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/batch/stop", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBatchStop(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleBatchStopGraceful(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm-1", "test-vm-2"},
		"force":   false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/stop", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchStop(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "shutdown" {
		t.Errorf("Expected operation 'shutdown', got '%s'", result.Operation)
	}

	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
}

func TestHandleBatchStopForce(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm-1", "test-vm-2", "test-vm-3"},
		"force":   true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/stop", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchStop(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "destroy" {
		t.Errorf("Expected operation 'destroy', got '%s'", result.Operation)
	}

	if result.Total != 3 {
		t.Errorf("Expected total 3, got %d", result.Total)
	}
}

func TestHandleBatchStopEmptyDomains(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/stop", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchStop(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("Expected total 0, got %d", result.Total)
	}
}

// Batch Reboot Handler Tests

func TestHandleBatchRebootMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/batch/reboot", nil)
	w := httptest.NewRecorder()

	server.handleBatchReboot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBatchRebootInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/batch/reboot", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBatchReboot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleBatchRebootGraceful(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm-1", "test-vm-2"},
		"force":   false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/reboot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchReboot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "reboot" {
		t.Errorf("Expected operation 'reboot', got '%s'", result.Operation)
	}

	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
}

func TestHandleBatchRebootForce(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm"},
		"force":   true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/reboot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchReboot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "reset" {
		t.Errorf("Expected operation 'reset', got '%s'", result.Operation)
	}

	if result.Total != 1 {
		t.Errorf("Expected total 1, got %d", result.Total)
	}
}

func TestHandleBatchRebootMultipleDomains(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"vm1", "vm2", "vm3", "vm4", "vm5"},
		"force":   false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/reboot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchReboot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("Expected total 5, got %d", result.Total)
	}
}

// Batch Snapshot Handler Tests

func TestHandleBatchSnapshotMethodNotAllowed(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodGet, "/batch/snapshot", nil)
	w := httptest.NewRecorder()

	server.handleBatchSnapshot(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleBatchSnapshotInvalidJSON(t *testing.T) {
	server := setupTestBasicServer(t)

	req := httptest.NewRequest(http.MethodPost, "/batch/snapshot", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.handleBatchSnapshot(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleBatchSnapshotBasic(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains":     []string{"test-vm-1", "test-vm-2"},
		"name_prefix": "backup",
		"description": "Pre-update backup",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/snapshot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchSnapshot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Operation != "snapshot" {
		t.Errorf("Expected operation 'snapshot', got '%s'", result.Operation)
	}

	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
}

func TestHandleBatchSnapshotWithOptions(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains":     []string{"test-vm"},
		"name_prefix": "test",
		"description": "Test snapshot",
		"atomic":      true,
		"disk_only":   true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/snapshot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchSnapshot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected total 1, got %d", result.Total)
	}
}

func TestHandleBatchSnapshotDefaultPrefix(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains": []string{"test-vm"},
		// name_prefix omitted - should default to "snapshot"
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/snapshot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchSnapshot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected total 1, got %d", result.Total)
	}
}

func TestHandleBatchSnapshotEmptyDomains(t *testing.T) {
	server := setupTestBasicServer(t)

	reqBody := map[string]interface{}{
		"domains":     []string{},
		"name_prefix": "backup",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/batch/snapshot", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.handleBatchSnapshot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchOperationResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("Expected total 0, got %d", result.Total)
	}
}
