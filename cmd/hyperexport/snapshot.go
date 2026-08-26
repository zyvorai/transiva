// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// SnapshotManager handles VM snapshot operations for safe exports
type SnapshotManager struct {
	client *vsphere.VSphereClient
	log    logger.Logger
}

// SnapshotConfig configures snapshot behavior
type SnapshotConfig struct {
	CreateSnapshot   bool   // Create snapshot before export
	DeleteAfter      bool   // Delete snapshot after export
	SnapshotName     string // Custom snapshot name
	SnapshotMemory   bool   // Include memory in snapshot
	SnapshotQuiesce  bool   // Quiesce filesystem before snapshot
	KeepSnapshots    int    // Number of snapshots to keep (0 = unlimited)
	SnapshotTimeout  time.Duration
}

// SnapshotResult contains snapshot operation results
type SnapshotResult struct {
	SnapshotName string
	SnapshotRef  string
	Created      time.Time
	Size         int64
	Success      bool
	Error        error
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(client *vsphere.VSphereClient, log logger.Logger) *SnapshotManager {
	return &SnapshotManager{
		client: client,
		log:    log,
	}
}

// CreateExportSnapshot creates a snapshot for export
func (sm *SnapshotManager) CreateExportSnapshot(ctx context.Context, vmPath string, config *SnapshotConfig) (*SnapshotResult, error) {
	if sm.client == nil {
		return nil, fmt.Errorf("vsphere client is nil")
	}

	if config == nil {
		config = &SnapshotConfig{
			CreateSnapshot:  true,
			SnapshotName:    fmt.Sprintf("export-%d", time.Now().Unix()),
			SnapshotMemory:  false,
			SnapshotQuiesce: true,
			SnapshotTimeout: 10 * time.Minute,
		}
	}

	sm.log.Info("creating export snapshot",
		"vm", vmPath,
		"name", config.SnapshotName,
		"memory", config.SnapshotMemory,
		"quiesce", config.SnapshotQuiesce)

	result := &SnapshotResult{
		SnapshotName: config.SnapshotName,
		Created:      time.Now(),
	}

	// Create snapshot with timeout
	snapshotCtx, cancel := context.WithTimeout(ctx, config.SnapshotTimeout)
	defer cancel()

	description := fmt.Sprintf("Export snapshot created at %s", time.Now().Format(time.RFC3339))
	snapshotRef, err := sm.client.CreateSnapshot(snapshotCtx, vmPath, config.SnapshotName, description, config.SnapshotMemory, config.SnapshotQuiesce)
	if err != nil {
		result.Error = err
		result.Success = false
		sm.log.Error("snapshot creation failed", "error", err)
		return result, fmt.Errorf("create snapshot: %w", err)
	}

	result.SnapshotRef = snapshotRef
	result.Success = true

	sm.log.Info("snapshot created successfully",
		"vm", vmPath,
		"snapshot", config.SnapshotName,
		"ref", snapshotRef)

	return result, nil
}

// DeleteSnapshot removes a snapshot
func (sm *SnapshotManager) DeleteSnapshot(ctx context.Context, vmPath, snapshotRef string) error {
	if sm.client == nil {
		return fmt.Errorf("vsphere client is nil")
	}

	sm.log.Info("deleting snapshot", "vm", vmPath, "ref", snapshotRef)

	// Delete snapshot (consolidate disks by default)
	err := sm.client.DeleteSnapshot(ctx, vmPath, snapshotRef, true)
	if err != nil {
		sm.log.Error("snapshot deletion failed", "error", err)
		return fmt.Errorf("delete snapshot: %w", err)
	}

	sm.log.Info("snapshot deleted successfully", "vm", vmPath, "ref", snapshotRef)
	return nil
}

// ListSnapshots returns all snapshots for a VM
func (sm *SnapshotManager) ListSnapshots(ctx context.Context, vmPath string) ([]string, error) {
	if sm.client == nil {
		return nil, fmt.Errorf("vsphere client is nil")
	}

	sm.log.Info("listing snapshots", "vm", vmPath)

	// Get snapshot information from vSphere
	snapshotInfos, err := sm.client.ListSnapshots(ctx, vmPath)
	if err != nil {
		sm.log.Error("failed to list snapshots", "error", err)
		return nil, fmt.Errorf("list snapshots: %w", err)
	}

	// Extract snapshot names
	snapshots := make([]string, len(snapshotInfos))
	for i, info := range snapshotInfos {
		snapshots[i] = info.Name
	}

	sm.log.Info("snapshots listed", "vm", vmPath, "count", len(snapshots))
	return snapshots, nil
}

// CleanupOldSnapshots removes old snapshots keeping only the newest N
func (sm *SnapshotManager) CleanupOldSnapshots(ctx context.Context, vmPath string, keepCount int) error {
	if keepCount <= 0 {
		sm.log.Info("snapshot cleanup disabled (keepCount <= 0)")
		return nil
	}

	snapshots, err := sm.ListSnapshots(ctx, vmPath)
	if err != nil {
		return err
	}

	if len(snapshots) <= keepCount {
		sm.log.Info("no snapshots to cleanup",
			"current", len(snapshots),
			"keep", keepCount)
		return nil
	}

	// Delete oldest snapshots (assumes snapshots are ordered by creation time)
	deleteCount := len(snapshots) - keepCount
	sm.log.Info("cleaning up old snapshots",
		"total", len(snapshots),
		"delete", deleteCount,
		"keep", keepCount)

	for i := 0; i < deleteCount; i++ {
		if err := sm.DeleteSnapshot(ctx, vmPath, snapshots[i]); err != nil {
			sm.log.Warn("failed to delete snapshot", "snapshot", snapshots[i], "error", err)
			// Continue with other snapshots even if one fails
		}
	}

	return nil
}

// RevertToSnapshot reverts VM to a snapshot
func (sm *SnapshotManager) RevertToSnapshot(ctx context.Context, vmPath, snapshotRef string) error {
	sm.log.Info("reverting to snapshot", "vm", vmPath, "ref", snapshotRef)

	// Revert to snapshot (suppress power on - let caller decide)
	err := sm.client.RevertToSnapshot(ctx, vmPath, snapshotRef, true)
	if err != nil {
		sm.log.Error("snapshot revert failed", "error", err)
		return fmt.Errorf("revert to snapshot: %w", err)
	}

	sm.log.Info("reverted to snapshot successfully", "vm", vmPath, "ref", snapshotRef)
	return nil
}

// ConsolidateSnapshots consolidates all snapshots (commits them to base disk)
func (sm *SnapshotManager) ConsolidateSnapshots(ctx context.Context, vmPath string) error {
	sm.log.Info("consolidating snapshots", "vm", vmPath)

	snapshots, err := sm.ListSnapshots(ctx, vmPath)
	if err != nil {
		return err
	}

	if len(snapshots) == 0 {
		sm.log.Info("no snapshots to consolidate")
		return nil
	}

	// Delete all snapshots (consolidates them)
	for _, snapshot := range snapshots {
		if err := sm.DeleteSnapshot(ctx, vmPath, snapshot); err != nil {
			return err
		}
	}

	sm.log.Info("snapshots consolidated successfully", "count", len(snapshots))
	return nil
}
