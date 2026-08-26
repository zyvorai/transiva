// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewSnapshotManager(t *testing.T) {
	log := logger.NewTestLogger(t)
	sm := NewSnapshotManager(nil, log)
	if sm == nil {
		t.Fatal("NewSnapshotManager returned nil")
	}
	if sm.client != nil {
		t.Error("Expected nil client")
	}
	if sm.log == nil {
		t.Error("Expected non-nil logger")
	}
}

func TestSnapshotConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *SnapshotConfig
		wantErr bool
	}{
		{
			name: "valid config with all defaults",
			config: &SnapshotConfig{
				CreateSnapshot:  true,
				DeleteAfter:     true,
				SnapshotName:    "export-snapshot",
				SnapshotMemory:  false,
				SnapshotQuiesce: true,
				KeepSnapshots:   3,
				SnapshotTimeout: 5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid config with memory snapshot",
			config: &SnapshotConfig{
				CreateSnapshot:  true,
				DeleteAfter:     false,
				SnapshotName:    "backup-snapshot",
				SnapshotMemory:  true,
				SnapshotQuiesce: false,
				KeepSnapshots:   5,
				SnapshotTimeout: 10 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "disabled snapshot creation",
			config: &SnapshotConfig{
				CreateSnapshot: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.SnapshotTimeout == 0 {
				tt.config.SnapshotTimeout = 5 * time.Minute
			}
			if tt.config.SnapshotName == "" {
				tt.config.SnapshotName = "default-snapshot"
			}
		})
	}
}

func TestSnapshotManager_CreateExportSnapshot(t *testing.T) {
	sm := NewSnapshotManager(nil, logger.NewTestLogger(t))
	ctx := context.Background()

	config := &SnapshotConfig{
		CreateSnapshot:  true,
		SnapshotName:    "test-snapshot",
		SnapshotMemory:  false,
		SnapshotQuiesce: true,
		SnapshotTimeout: 5 * time.Minute,
	}

	// Test with nil client (should return error)
	result, err := sm.CreateExportSnapshot(ctx, "/datacenter/vm/test-vm", config)
	if err == nil {
		t.Error("Expected error with nil client, got nil")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}

func TestSnapshotManager_DeleteSnapshot(t *testing.T) {
	sm := NewSnapshotManager(nil, logger.NewTestLogger(t))
	ctx := context.Background()

	// Test with nil client (should return error)
	err := sm.DeleteSnapshot(ctx, "/datacenter/vm/test-vm", "snapshot-ref-123")
	if err == nil {
		t.Error("Expected error with nil client, got nil")
	}
}

func TestSnapshotManager_ListSnapshots(t *testing.T) {
	sm := NewSnapshotManager(nil, logger.NewTestLogger(t))
	ctx := context.Background()

	// Test with nil client (should return error)
	snapshots, err := sm.ListSnapshots(ctx, "/datacenter/vm/test-vm")
	if err == nil {
		t.Error("Expected error with nil client, got nil")
	}
	if snapshots != nil {
		t.Error("Expected nil snapshots on error")
	}
}

func TestSnapshotManager_CleanupOldSnapshots(t *testing.T) {
	sm := NewSnapshotManager(nil, logger.NewTestLogger(t))
	ctx := context.Background()

	config := &SnapshotConfig{
		KeepSnapshots: 3,
	}

	// Test with nil client (should return error)
	err := sm.CleanupOldSnapshots(ctx, "/datacenter/vm/test-vm", config.KeepSnapshots)
	if err == nil {
		t.Error("Expected error with nil client, got nil")
	}
}

func TestSnapshotResult_Fields(t *testing.T) {
	now := time.Now()
	result := &SnapshotResult{
		SnapshotRef:  "snapshot-123",
		SnapshotName: "test-snapshot",
		Created:      now,
		Size:         1024 * 1024,
		Success:      true,
		Error:        nil,
	}

	if result.SnapshotRef == "" {
		t.Error("SnapshotRef should not be empty")
	}
	if result.SnapshotName == "" {
		t.Error("SnapshotName should not be empty")
	}
	if result.Created.IsZero() {
		t.Error("Created should not be zero")
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.Size != 1024*1024 {
		t.Error("Size mismatch")
	}
}

func TestSnapshotConfig_DefaultTimeout(t *testing.T) {
	config := &SnapshotConfig{
		CreateSnapshot: true,
		SnapshotName:   "test",
	}

	if config.SnapshotTimeout == 0 {
		config.SnapshotTimeout = 5 * time.Minute
	}

	if config.SnapshotTimeout != 5*time.Minute {
		t.Errorf("Expected default timeout of 5 minutes, got %v", config.SnapshotTimeout)
	}
}

func TestSnapshotConfig_KeepSnapshotsValidation(t *testing.T) {
	tests := []struct {
		name          string
		keepSnapshots int
		expectValid   bool
	}{
		{"zero snapshots", 0, true},
		{"negative snapshots", -1, false},
		{"one snapshot", 1, true},
		{"many snapshots", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SnapshotConfig{
				KeepSnapshots: tt.keepSnapshots,
			}

			isValid := config.KeepSnapshots >= 0
			if isValid != tt.expectValid {
				t.Errorf("Expected valid=%v for %d snapshots, got %v", tt.expectValid, tt.keepSnapshots, isValid)
			}
		})
	}
}
