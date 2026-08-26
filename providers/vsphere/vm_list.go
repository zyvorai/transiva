// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// ListVMs returns a list of all VMs
func (c *VSphereClient) ListVMs(ctx context.Context) ([]VMInfo, error) {
	// Find all VMs using the finder
	vmList, err := c.finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, err
	}

	// Collect VM references and paths
	var refs []types.ManagedObjectReference
	vmPaths := make(map[types.ManagedObjectReference]string)

	for _, vmObj := range vmList {
		ref := vmObj.Reference()
		refs = append(refs, ref)

		// Get inventory path (it's a field, not a method)
		vmPaths[ref] = vmObj.InventoryPath
	}

	// Retrieve properties
	var vms []mo.VirtualMachine
	pc := property.DefaultCollector(c.client.Client)
	err = pc.Retrieve(ctx, refs, []string{
		"name",
		"runtime.powerState",
		"config.guestFullName",
		"config.hardware.memoryMB",
		"config.hardware.numCPU",
		"config.hardware.device",
	}, &vms)
	if err != nil {
		return nil, err
	}

	// Build response
	var result []VMInfo
	for i, vm := range vms {
		// Get path from map
		path := vmPaths[refs[i]]

		// Skip VMs without config (templates, inaccessible VMs, etc.)
		if vm.Config == nil {
			continue
		}

		// Calculate storage
		var totalStorage int64
		if vm.Config.Hardware.Device != nil {
			for _, device := range vm.Config.Hardware.Device {
				if disk, ok := device.(*types.VirtualDisk); ok {
					totalStorage += disk.CapacityInBytes
				}
			}
		}

		info := VMInfo{
			Name:       vm.Name,
			Path:       path,
			PowerState: string(vm.Runtime.PowerState),
			GuestOS:    vm.Config.GuestFullName,
			MemoryMB:   vm.Config.Hardware.MemoryMB,
			NumCPU:     vm.Config.Hardware.NumCPU,
			Storage:    totalStorage,
		}

		result = append(result, info)
	}

	return result, nil
}
