// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// FindAllVMs returns paths of all VMs in the datacenter
func (c *VSphereClient) FindAllVMs(ctx context.Context) ([]string, error) {
	vms, err := c.finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list VMs: %w", err)
	}

	paths := make([]string, 0, len(vms))
	for _, vm := range vms {
		paths = append(paths, vm.InventoryPath)
	}

	return paths, nil
}

// FilterVMsByTag filters VMs by tag key-value pair from ExtraConfig
func (c *VSphereClient) FilterVMsByTag(ctx context.Context, vmPaths []string, tagKey, tagValue string) ([]string, error) {
	c.logger.Debug("filtering VMs by tag", "key", tagKey, "value", tagValue)

	filtered := make([]string, 0)

	for _, vmPath := range vmPaths {
		vm, err := c.finder.VirtualMachine(ctx, vmPath)
		if err != nil {
			c.logger.Warn("failed to find VM, skipping", "vm", vmPath, "error", err)
			continue
		}

		var moVM mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), []string{"config"}, &moVM); err != nil {
			c.logger.Warn("failed to get VM properties, skipping", "vm", vmPath, "error", err)
			continue
		}

		// Check ExtraConfig for matching tag
		if moVM.Config != nil && moVM.Config.ExtraConfig != nil {
			for _, extra := range moVM.Config.ExtraConfig {
				if optValue, ok := extra.(*types.OptionValue); ok {
					if optValue.Key == tagKey {
						if strVal, ok := optValue.Value.(string); ok && strVal == tagValue {
							filtered = append(filtered, vmPath)
							break
						}
					}
				}
			}
		}
	}

	c.logger.Info("tag filter applied", "matched", len(filtered), "total", len(vmPaths))
	return filtered, nil
}

// GetVMInfo retrieves detailed information about a VM
func (c *VSphereClient) GetVMInfo(ctx context.Context, vmPath string) (*VMInfo, error) {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, fmt.Errorf("find VM: %w", err)
	}

	var moVM mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config", "runtime", "guest", "storage"}, &moVM); err != nil {
		return nil, fmt.Errorf("get VM properties: %w", err)
	}

	// Calculate total storage
	var totalStorage int64
	if moVM.Config != nil && moVM.Config.Hardware.Device != nil {
		for _, device := range moVM.Config.Hardware.Device {
			if disk, ok := device.(*types.VirtualDisk); ok {
				totalStorage += disk.CapacityInBytes
			}
		}
	}

	info := &VMInfo{
		Name:       vm.Name(),
		Path:       vm.InventoryPath,
		PowerState: string(moVM.Runtime.PowerState),
		Storage:    totalStorage,
	}

	if moVM.Config != nil {
		info.GuestOS = moVM.Config.GuestFullName
		info.MemoryMB = int32(moVM.Config.Hardware.MemoryMB)
		info.NumCPU = moVM.Config.Hardware.NumCPU
	}

	return info, nil
}

// ShutdownVM performs graceful shutdown of a VM
func (c *VSphereClient) ShutdownVM(ctx context.Context, vmPath string, timeout time.Duration) error {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	c.logger.Info("initiating graceful shutdown", "vm", vmPath)

	// Initiate shutdown
	if err := vm.ShutdownGuest(ctx); err != nil {
		return fmt.Errorf("shutdown guest: %w", err)
	}

	// Wait for VM to power off
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-shutdownCtx.Done():
			return fmt.Errorf("shutdown timeout after %v", timeout)
		case <-ticker.C:
			powerState, err := vm.PowerState(ctx)
			if err != nil {
				c.logger.Warn("failed to check power state", "error", err)
				continue
			}

			if powerState == types.VirtualMachinePowerStatePoweredOff {
				c.logger.Info("VM shutdown complete", "vm", vmPath)
				return nil
			}

			c.logger.Debug("waiting for shutdown", "vm", vmPath, "powerState", powerState)
		}
	}
}

// PowerOffVM forcefully powers off a VM
func (c *VSphereClient) PowerOffVM(ctx context.Context, vmPath string) error {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	c.logger.Info("powering off VM", "vm", vmPath)

	task, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("power off: %w", err)
	}

	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("wait for power off: %w", err)
	}

	c.logger.Info("VM powered off", "vm", vmPath)
	return nil
}

// RemoveCDROMDevices removes all CD/DVD devices from a VM
func (c *VSphereClient) RemoveCDROMDevices(ctx context.Context, vmPath string) error {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("find VM: %w", err)
	}

	c.logger.Info("removing CD/DVD devices", "vm", vmPath)

	// Get VM devices
	devices, err := vm.Device(ctx)
	if err != nil {
		return fmt.Errorf("get devices: %w", err)
	}

	// Find CD/DVD devices
	var cdroms object.VirtualDeviceList
	for _, device := range devices {
		if _, ok := device.(*types.VirtualCdrom); ok {
			cdroms = append(cdroms, device)
		}
	}

	if len(cdroms) == 0 {
		c.logger.Debug("no CD/DVD devices found")
		return nil
	}

	c.logger.Info("found CD/DVD devices", "count", len(cdroms))

	// Remove devices
	if err := vm.RemoveDevice(ctx, true, cdroms...); err != nil {
		return fmt.Errorf("remove CD/DVD devices: %w", err)
	}

	c.logger.Info("removed CD/DVD devices", "count", len(cdroms))
	return nil
}

// GetVMProperty retrieves a specific property from a VM
func (c *VSphereClient) GetVMProperty(ctx context.Context, vmPath, property string) (interface{}, error) {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, fmt.Errorf("find VM: %w", err)
	}

	var moVM mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{property}, &moVM); err != nil {
		return nil, fmt.Errorf("get property: %w", err)
	}

	// Use reflection to get property value
	// This is simplified - in production you'd use proper reflection
	return moVM, nil
}

// ListVMDisks returns information about all disks attached to a VM
func (c *VSphereClient) ListVMDisks(ctx context.Context, vmPath string) ([]types.VirtualDisk, error) {
	vm, err := c.finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		return nil, fmt.Errorf("find VM: %w", err)
	}

	devices, err := vm.Device(ctx)
	if err != nil {
		return nil, fmt.Errorf("get devices: %w", err)
	}

	var disks []types.VirtualDisk
	for _, device := range devices {
		if disk, ok := device.(*types.VirtualDisk); ok {
			disks = append(disks, *disk)
		}
	}

	return disks, nil
}

// FindVMByName finds a VM by its name (searches inventory)
func (c *VSphereClient) FindVMByName(ctx context.Context, name string) (string, error) {
	vms, err := c.FindAllVMs(ctx)
	if err != nil {
		return "", err
	}

	// Try exact match first
	for _, vmPath := range vms {
		if strings.HasSuffix(vmPath, "/"+name) {
			return vmPath, nil
		}
	}

	// Try fuzzy match
	for _, vmPath := range vms {
		if strings.Contains(strings.ToLower(vmPath), strings.ToLower(name)) {
			return vmPath, nil
		}
	}

	return "", fmt.Errorf("VM not found: %s", name)
}
