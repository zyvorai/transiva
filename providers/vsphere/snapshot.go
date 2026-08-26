// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// SnapshotInfo contains information about a VM snapshot
type SnapshotInfo struct {
	Name            string
	Description     string
	ID              int32
	CreateTime      time.Time
	State           string
	Quiesced        bool
	ReplaySupported bool
}

// CreateSnapshot creates a snapshot of a virtual machine
func (c *VSphereClient) CreateSnapshot(ctx context.Context, vmPath, snapshotName, description string, memory, quiesce bool) (string, error) {
	// Find the VM
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return "", fmt.Errorf("find VM: %w", err)
	}

	// Create snapshot task
	task, err := vm.CreateSnapshot(ctx, snapshotName, description, memory, quiesce)
	if err != nil {
		return "", fmt.Errorf("create snapshot task: %w", err)
	}

	// Wait for task to complete
	info, err := task.WaitForResult(ctx)
	if err != nil {
		return "", fmt.Errorf("snapshot creation failed: %w", err)
	}

	// Get snapshot reference from result
	if info.Result == nil {
		return "", fmt.Errorf("snapshot creation returned no result")
	}

	snapshotRef, ok := info.Result.(types.ManagedObjectReference)
	if !ok {
		return "", fmt.Errorf("unexpected result type: %T", info.Result)
	}

	return snapshotRef.Value, nil
}

// DeleteSnapshot removes a snapshot by name
func (c *VSphereClient) DeleteSnapshot(ctx context.Context, vmPath, snapshotName string, consolidateDisks bool) error {
	// Find the VM
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	// Remove snapshot using VM's built-in method
	task, err := vm.RemoveSnapshot(ctx, snapshotName, false, &consolidateDisks)
	if err != nil {
		return fmt.Errorf("remove snapshot task: %w", err)
	}

	// Wait for completion
	err = task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("snapshot removal failed: %w", err)
	}

	return nil
}

// ListSnapshots lists all snapshots for a VM
func (c *VSphereClient) ListSnapshots(ctx context.Context, vmPath string) ([]SnapshotInfo, error) {
	// Find the VM
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, fmt.Errorf("find VM: %w", err)
	}

	// Get snapshot information
	var moVM mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &moVM)
	if err != nil {
		return nil, fmt.Errorf("get VM properties: %w", err)
	}

	// Extract snapshot tree
	var snapshots []SnapshotInfo
	if moVM.Snapshot != nil && moVM.Snapshot.RootSnapshotList != nil {
		snapshots = c.extractSnapshotInfo(moVM.Snapshot.RootSnapshotList)
	}

	return snapshots, nil
}

// RevertToSnapshot reverts a VM to a specific snapshot
func (c *VSphereClient) RevertToSnapshot(ctx context.Context, vmPath, snapshotName string, suppressPowerOn bool) error {
	// Find the VM
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	// Revert to snapshot using VM's built-in method
	task, err := vm.RevertToSnapshot(ctx, snapshotName, suppressPowerOn)
	if err != nil {
		return fmt.Errorf("revert snapshot task: %w", err)
	}

	// Wait for completion
	err = task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("snapshot revert failed: %w", err)
	}

	return nil
}

// Helper function to extract snapshot information from tree
func (c *VSphereClient) extractSnapshotInfo(tree []types.VirtualMachineSnapshotTree) []SnapshotInfo {
	var snapshots []SnapshotInfo

	for _, snapshot := range tree {
		replaySupported := false
		if snapshot.ReplaySupported != nil {
			replaySupported = *snapshot.ReplaySupported
		}

		info := SnapshotInfo{
			Name:            snapshot.Name,
			Description:     snapshot.Description,
			ID:              snapshot.Id,
			CreateTime:      snapshot.CreateTime,
			State:           string(snapshot.State),
			Quiesced:        snapshot.Quiesced,
			ReplaySupported: replaySupported,
		}
		snapshots = append(snapshots, info)

		// Add children recursively
		if len(snapshot.ChildSnapshotList) > 0 {
			childSnapshots := c.extractSnapshotInfo(snapshot.ChildSnapshotList)
			snapshots = append(snapshots, childSnapshots...)
		}
	}

	return snapshots
}
