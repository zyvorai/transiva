// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CloneVM clones a virtual machine
func (c *VSphereClient) CloneVM(ctx context.Context, spec CloneSpec) (*CloneResult, error) {
	startTime := time.Now()

	// Find source VM
	sourceVM, err := c.finder.VirtualMachine(ctx, spec.SourceVM)
	if err != nil {
		return nil, fmt.Errorf("find source VM: %w", err)
	}

	// Find or use default folder
	var targetFolder *object.Folder
	if spec.TargetFolder != "" {
		targetFolder, err = c.finder.Folder(ctx, spec.TargetFolder)
		if err != nil {
			return nil, fmt.Errorf("find target folder: %w", err)
		}
	} else {
		// Use parent folder of source VM - get default VM folder
		dc, err := c.finder.DefaultDatacenter(ctx)
		if err != nil {
			return nil, fmt.Errorf("find default datacenter: %w", err)
		}
		folders, err := dc.Folders(ctx)
		if err != nil {
			return nil, fmt.Errorf("get datacenter folders: %w", err)
		}
		targetFolder = folders.VmFolder
	}

	// Build clone spec
	cloneSpec := types.VirtualMachineCloneSpec{
		PowerOn:  spec.PowerOn,
		Template: spec.Template,
	}

	// Configure relocation spec
	relocateSpec := types.VirtualMachineRelocateSpec{}

	// Set resource pool
	if spec.ResourcePool != "" {
		pool, err := c.finder.ResourcePool(ctx, spec.ResourcePool)
		if err != nil {
			return nil, fmt.Errorf("find resource pool: %w", err)
		}
		poolRef := pool.Reference()
		relocateSpec.Pool = &poolRef
	}

	// Set datastore
	if spec.Datastore != "" {
		ds, err := c.finder.Datastore(ctx, spec.Datastore)
		if err != nil {
			return nil, fmt.Errorf("find datastore: %w", err)
		}
		dsRef := ds.Reference()
		relocateSpec.Datastore = &dsRef
	}

	// Handle linked clone
	if spec.LinkedClone {
		if spec.Snapshot == "" {
			return nil, fmt.Errorf("snapshot name required for linked clone")
		}

		// Find snapshot
		snapshot, err := sourceVM.FindSnapshot(ctx, spec.Snapshot)
		if err != nil {
			return nil, fmt.Errorf("find snapshot: %w", err)
		}

		snapshotRef := snapshot.Reference()
		relocateSpec.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsCreateNewChildDiskBacking)
		cloneSpec.Snapshot = &snapshotRef
	}

	cloneSpec.Location = relocateSpec

	// Execute clone operation
	c.logger.Info("cloning VM",
		"source", spec.SourceVM,
		"target", spec.TargetName,
		"linked", spec.LinkedClone,
		"power_on", spec.PowerOn)

	task, err := sourceVM.Clone(ctx, targetFolder, spec.TargetName, cloneSpec)
	if err != nil {
		return &CloneResult{
			SourceVM:   spec.SourceVM,
			TargetName: spec.TargetName,
			Success:    false,
			Duration:   time.Since(startTime),
			Error:      err.Error(),
		}, fmt.Errorf("clone VM: %w", err)
	}

	// Wait for clone task to complete
	err = task.Wait(ctx)
	duration := time.Since(startTime)

	if err != nil {
		return &CloneResult{
			SourceVM:   spec.SourceVM,
			TargetName: spec.TargetName,
			Success:    false,
			Duration:   duration,
			Error:      err.Error(),
		}, fmt.Errorf("wait for clone task: %w", err)
	}

	// Get result (new VM reference)
	info, err := task.WaitForResult(ctx)
	if err != nil {
		return &CloneResult{
			SourceVM:   spec.SourceVM,
			TargetName: spec.TargetName,
			Success:    false,
			Duration:   duration,
			Error:      err.Error(),
		}, fmt.Errorf("get clone result: %w", err)
	}

	result := &CloneResult{
		SourceVM:   spec.SourceVM,
		TargetName: spec.TargetName,
		Success:    true,
		Duration:   duration,
		TaskID:     task.Reference().Value,
	}

	// Get new VM path
	if info.Result != nil {
		if vmRef, ok := info.Result.(types.ManagedObjectReference); ok {
			newVM := object.NewVirtualMachine(c.client.Client, vmRef)

			// Get VM properties to retrieve inventory path
			var vmMO mo.VirtualMachine
			err := newVM.Properties(ctx, vmRef, []string{"name", "parent"}, &vmMO)
			if err == nil {
				// Construct inventory path from target folder and VM name
				result.TargetPath = targetFolder.InventoryPath + "/" + spec.TargetName
			}
		}
	}

	c.logger.Info("clone completed",
		"source", spec.SourceVM,
		"target", spec.TargetName,
		"duration", duration)

	return result, nil
}

// BulkCloneVMs clones multiple VMs concurrently
func (c *VSphereClient) BulkCloneVMs(ctx context.Context, specs []CloneSpec, maxConcurrent int) ([]CloneResult, error) {
	if maxConcurrent <= 0 {
		maxConcurrent = 5 // Default concurrency
	}

	results := make([]CloneResult, len(specs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	c.logger.Info("starting bulk clone",
		"count", len(specs),
		"max_concurrent", maxConcurrent)

	for i, spec := range specs {
		wg.Add(1)
		go func(index int, cloneSpec CloneSpec) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Clone VM
			result, err := c.CloneVM(ctx, cloneSpec)
			if err != nil {
				c.logger.Error("clone failed",
					"source", cloneSpec.SourceVM,
					"target", cloneSpec.TargetName,
					"error", err)
				if result != nil {
					results[index] = *result
				} else {
					results[index] = CloneResult{
						SourceVM:   cloneSpec.SourceVM,
						TargetName: cloneSpec.TargetName,
						Success:    false,
						Error:      err.Error(),
					}
				}
				return
			}

			results[index] = *result
		}(i, spec)
	}

	// Wait for all clones to complete
	wg.Wait()

	// Count successes and failures
	var successCount, failCount int
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	c.logger.Info("bulk clone completed",
		"total", len(specs),
		"success", successCount,
		"failed", failCount)

	return results, nil
}

// CreateTemplate converts a VM to a template
func (c *VSphereClient) CreateTemplate(ctx context.Context, vmName string) error {
	// Find VM
	vm, err := c.finder.VirtualMachine(ctx, vmName)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	// Mark as template
	err = vm.MarkAsTemplate(ctx)
	if err != nil {
		return fmt.Errorf("mark as template: %w", err)
	}

	c.logger.Info("converted VM to template", "vm", vmName)
	return nil
}

// DeployFromTemplate deploys a VM from a template
func (c *VSphereClient) DeployFromTemplate(ctx context.Context, spec CloneSpec) (*CloneResult, error) {
	startTime := time.Now()

	// Find template
	template, err := c.finder.VirtualMachine(ctx, spec.SourceVM)
	if err != nil {
		return nil, fmt.Errorf("find template: %w", err)
	}

	// Verify it's a template
	var vmConfig mo.VirtualMachine
	if err := template.Properties(ctx, template.Reference(), []string{"config.template"}, &vmConfig); err != nil {
		return nil, fmt.Errorf("get template properties: %w", err)
	}

	if vmConfig.Config != nil && !vmConfig.Config.Template {
		return nil, fmt.Errorf("%s is not a template", spec.SourceVM)
	}

	// Find target folder
	var targetFolder *object.Folder
	if spec.TargetFolder != "" {
		targetFolder, err = c.finder.Folder(ctx, spec.TargetFolder)
		if err != nil {
			return nil, fmt.Errorf("find target folder: %w", err)
		}
	} else {
		// Use default VM folder
		dc, err := c.finder.DefaultDatacenter(ctx)
		if err != nil {
			return nil, fmt.Errorf("find default datacenter: %w", err)
		}
		folders, err := dc.Folders(ctx)
		if err != nil {
			return nil, fmt.Errorf("get datacenter folders: %w", err)
		}
		targetFolder = folders.VmFolder
	}

	// Build clone spec
	cloneSpec := types.VirtualMachineCloneSpec{
		PowerOn:  spec.PowerOn,
		Template: false, // Deploy as VM, not template
	}

	// Configure relocation
	relocateSpec := types.VirtualMachineRelocateSpec{}

	// Set resource pool (required for template deployment)
	if spec.ResourcePool != "" {
		pool, err := c.finder.ResourcePool(ctx, spec.ResourcePool)
		if err != nil {
			return nil, fmt.Errorf("find resource pool: %w", err)
		}
		poolRef := pool.Reference()
		relocateSpec.Pool = &poolRef
	} else {
		return nil, fmt.Errorf("resource pool required for template deployment")
	}

	// Set datastore
	if spec.Datastore != "" {
		ds, err := c.finder.Datastore(ctx, spec.Datastore)
		if err != nil {
			return nil, fmt.Errorf("find datastore: %w", err)
		}
		dsRef := ds.Reference()
		relocateSpec.Datastore = &dsRef
	}

	cloneSpec.Location = relocateSpec

	// Guest customization (if specified)
	if spec.CustomizeGuest && len(spec.Customization) > 0 {
		// Build customization spec
		customSpec := types.CustomizationSpec{}
		// Note: Full guest customization implementation would require
		// building identity, globalIPSettings, nicSettingMap, etc.
		// This is a placeholder for the structure
		cloneSpec.Customization = &customSpec
	}

	// Execute deployment
	c.logger.Info("deploying from template",
		"template", spec.SourceVM,
		"target", spec.TargetName,
		"power_on", spec.PowerOn)

	task, err := template.Clone(ctx, targetFolder, spec.TargetName, cloneSpec)
	if err != nil {
		return &CloneResult{
			SourceVM:   spec.SourceVM,
			TargetName: spec.TargetName,
			Success:    false,
			Duration:   time.Since(startTime),
			Error:      err.Error(),
		}, fmt.Errorf("deploy from template: %w", err)
	}

	// Wait for completion
	err = task.Wait(ctx)
	duration := time.Since(startTime)

	if err != nil {
		return &CloneResult{
			SourceVM:   spec.SourceVM,
			TargetName: spec.TargetName,
			Success:    false,
			Duration:   duration,
			Error:      err.Error(),
		}, fmt.Errorf("wait for deployment: %w", err)
	}

	result := &CloneResult{
		SourceVM:   spec.SourceVM,
		TargetName: spec.TargetName,
		Success:    true,
		Duration:   duration,
		TaskID:     task.Reference().Value,
	}

	c.logger.Info("deployment completed",
		"template", spec.SourceVM,
		"target", spec.TargetName,
		"duration", duration)

	return result, nil
}
