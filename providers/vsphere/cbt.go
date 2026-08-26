// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"

	// "github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/zyvorai/transiva/providers/incremental"
)

// EnableCBT enables Changed Block Tracking on a VM
func (c *VSphereClient) EnableCBT(ctx context.Context, vmPath string) error {
	c.logger.Info("enabling CBT", "vm", vmPath)

	// Find the VM
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	// Get current config
	var moVM mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config"}, &moVM)
	if err != nil {
		return fmt.Errorf("get VM properties: %w", err)
	}

	// Check if already enabled
	if moVM.Config != nil && moVM.Config.ChangeTrackingEnabled != nil && *moVM.Config.ChangeTrackingEnabled {
		c.logger.Info("CBT already enabled", "vm", vmPath)
		return nil
	}

	// Enable CBT
	spec := types.VirtualMachineConfigSpec{
		ChangeTrackingEnabled: types.NewBool(true),
	}

	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("reconfigure VM: %w", err)
	}

	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("enable CBT failed: %w", err)
	}

	c.logger.Info("CBT enabled successfully", "vm", vmPath)

	// Create a snapshot to activate CBT
	_, err = c.CreateSnapshot(ctx, vmPath, "cbt-activation", "Snapshot to activate CBT", false, false)
	if err != nil {
		c.logger.Warn("failed to create activation snapshot", "error", err)
		// Non-fatal, CBT will be activated on next snapshot
	}

	return nil
}

// DisableCBT disables Changed Block Tracking on a VM
func (c *VSphereClient) DisableCBT(ctx context.Context, vmPath string) error {
	c.logger.Info("disabling CBT", "vm", vmPath)

	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	spec := types.VirtualMachineConfigSpec{
		ChangeTrackingEnabled: types.NewBool(false),
	}

	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("reconfigure VM: %w", err)
	}

	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("disable CBT failed: %w", err)
	}

	c.logger.Info("CBT disabled successfully", "vm", vmPath)
	return nil
}

// IsCBTEnabled checks if CBT is enabled on a VM
func (c *VSphereClient) IsCBTEnabled(ctx context.Context, vmPath string) (bool, error) {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return false, fmt.Errorf("find VM: %w", err)
	}

	var moVM mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config"}, &moVM)
	if err != nil {
		return false, fmt.Errorf("get VM properties: %w", err)
	}

	if moVM.Config == nil || moVM.Config.ChangeTrackingEnabled == nil {
		return false, nil
	}

	return *moVM.Config.ChangeTrackingEnabled, nil
}

// GetDiskChangeIDs retrieves change IDs for all disks in a VM
func (c *VSphereClient) GetDiskChangeIDs(ctx context.Context, vmPath string) (map[string]string, error) {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, fmt.Errorf("find VM: %w", err)
	}

	var moVM mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config", "layout.disk"}, &moVM)
	if err != nil {
		return nil, fmt.Errorf("get VM properties: %w", err)
	}

	changeIDs := make(map[string]string)

	// Extract changeId from each disk
	if moVM.Config != nil && moVM.Config.Hardware.Device != nil {
		for _, device := range moVM.Config.Hardware.Device {
			if disk, ok := device.(*types.VirtualDisk); ok {
				key := fmt.Sprintf("disk-%d", disk.Key)

				// Try to get changeId from backing info
				if backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
					if backing.ChangeId != "" {
						changeIDs[key] = backing.ChangeId
					}
				} else if backing, ok := disk.Backing.(*types.VirtualDiskSparseVer2BackingInfo); ok {
					if backing.ChangeId != "" {
						changeIDs[key] = backing.ChangeId
					}
				}
			}
		}
	}

	return changeIDs, nil
}

// QueryChangedDiskAreas queries changed areas of a disk between two changeIds
// TODO: Fix this function - QueryChangedDiskAreas needs proper VM object API call
func (c *VSphereClient) QueryChangedDiskAreas(ctx context.Context, vmPath string, snapshot *types.ManagedObjectReference, diskKey int32, startOffset int64, prevChangeID, currentChangeID string) ([]incremental.ChangedBlock, error) {
	c.logger.Debug("querying changed disk areas",
		"vm", vmPath,
		"disk_key", diskKey,
		"prev_change_id", prevChangeID,
		"current_change_id", currentChangeID)

	// TODO: This function needs to be implemented with proper govmomi API
	// For now, return empty list to avoid compilation errors
	c.logger.Warn("QueryChangedDiskAreas not yet implemented - returning empty list")
	return []incremental.ChangedBlock{}, nil

	/*
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, fmt.Errorf("find VM: %w", err)
	}

	// Create DiskChangeInfo request
	req := types.QueryChangedDiskAreas{
		This:      vm.Reference(),
		Snapshot:  snapshot,
		DeviceKey: diskKey,
		StartOffset: startOffset,
		ChangeId:  prevChangeID,
	}

	// Call QueryChangedDiskAreas API - needs proper VM method
	res, err := vm.QueryChangedDiskAreas(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("query changed disk areas: %w", err)
	}

	// Convert response to our format
	var changedBlocks []incremental.ChangedBlock

	for _, change := range res.Returnval.ChangedArea {
		changedBlocks = append(changedBlocks, incremental.ChangedBlock{
			Offset: change.Start,
			Length: change.Length,
		})
	}

	c.logger.Debug("found changed blocks",
		"count", len(changedBlocks),
		"total_changed_mb", calculateTotalSize(changedBlocks)/(1024*1024))

	return changedBlocks, nil
	*/
}

// GetDiskMetadata retrieves metadata for all disks in a VM
func (c *VSphereClient) GetDiskMetadata(ctx context.Context, vmPath string) ([]incremental.DiskMetadata, error) {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, fmt.Errorf("find VM: %w", err)
	}

	var moVM mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config"}, &moVM)
	if err != nil {
		return nil, fmt.Errorf("get VM properties: %w", err)
	}

	var disks []incremental.DiskMetadata

	if moVM.Config != nil && moVM.Config.Hardware.Device != nil {
		for _, device := range moVM.Config.Hardware.Device {
			if disk, ok := device.(*types.VirtualDisk); ok {
				key := fmt.Sprintf("disk-%d", disk.Key)
				var changeID string
				var backingInfo string
				var path string

				// Extract backing info
				if backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
					changeID = backing.ChangeId
					path = backing.FileName
					backingInfo = "FlatVer2"
				} else if backing, ok := disk.Backing.(*types.VirtualDiskSparseVer2BackingInfo); ok {
					changeID = backing.ChangeId
					path = backing.FileName
					backingInfo = "SparseVer2"
				}

				disks = append(disks, incremental.DiskMetadata{
					Key:           key,
					Path:          path,
					CapacityBytes: disk.CapacityInBytes,
					ChangeID:      changeID,
					BackingInfo:   backingInfo,
				})
			}
		}
	}

	return disks, nil
}

// ExportIncrementalVMDK exports only changed blocks of a VMDK
func (c *VSphereClient) ExportIncrementalVMDK(ctx context.Context, vmPath string, diskKey int32, prevChangeID string, outputPath string) error {
	c.logger.Info("starting incremental VMDK export",
		"vm", vmPath,
		"disk_key", diskKey,
		"prev_change_id", prevChangeID)

	// Create a snapshot for the export
	snapshotName := fmt.Sprintf("incremental-export-%d", diskKey)
	snapshotID, err := c.CreateSnapshot(ctx, vmPath, snapshotName, "Temporary snapshot for incremental export", false, true)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}

	defer func() {
		// Clean up snapshot
		if err := c.DeleteSnapshot(ctx, vmPath, snapshotName, true); err != nil {
			c.logger.Warn("failed to delete temporary snapshot",
				"snapshot", snapshotName,
				"error", err)
		}
	}()

	// Get snapshot reference
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	var moVM mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"snapshot"}, &moVM)
	if err != nil {
		return fmt.Errorf("get VM properties: %w", err)
	}

	if moVM.Snapshot == nil {
		return fmt.Errorf("snapshot not found")
	}

	// Find the snapshot we just created
	snapshot := findSnapshotByID(moVM.Snapshot.RootSnapshotList, snapshotID)
	if snapshot == nil {
		return fmt.Errorf("snapshot %s not found", snapshotID)
	}

	// Query changed areas
	// In a full implementation, this would:
	// 1. Query all changed areas in chunks
	// 2. Export only those blocks
	// 3. Create a delta file or patch file
	// For now, this is a placeholder

	c.logger.Warn("incremental VMDK export not fully implemented yet")
	c.logger.Info("would export changed blocks to", "output", outputPath)

	return nil
}

// Helper function to find snapshot by ID
func findSnapshotByID(tree []types.VirtualMachineSnapshotTree, id string) *types.ManagedObjectReference {
	for _, snapshot := range tree {
		if fmt.Sprintf("%d", snapshot.Id) == id || snapshot.Snapshot.Value == id {
			return &snapshot.Snapshot
		}

		if len(snapshot.ChildSnapshotList) > 0 {
			if found := findSnapshotByID(snapshot.ChildSnapshotList, id); found != nil {
				return found
			}
		}
	}
	return nil
}

