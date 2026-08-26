// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"github.com/zyvorai/transiva/providers"
)

// InventoryFromVMInfo converts unified VMInfo to Nutanix inventory.
func InventoryFromVMInfo(vm *providers.VMInfo) VMInventory {
	if vm == nil {
		return VMInventory{}
	}

	inv := VMInventory{
		Name:       vm.Name,
		UUID:       vm.ID,
		PowerState: vm.PowerState,
		VCPUs:      vm.NumCPUs,
		MemoryGiB:  float64(vm.MemoryMB) / 1024,
	}
	if vm.Tags != nil {
		inv.ClusterUUID = vm.Tags["cluster_uuid"]
		inv.ClusterName = vm.Tags["cluster_name"]
	}
	if vm.Metadata != nil {
		if dc, ok := vm.Metadata["disk_count"].(int); ok {
			inv.DiskCount = dc
		}
		if td, ok := vm.Metadata["total_disk_gib"].(float64); ok {
			inv.TotalDiskGiB = td
		}
		if nc, ok := vm.Metadata["nic_count"].(int); ok {
			inv.NICCount = nc
		}
		inv.Disks = parseDiskInfoSlice(vm.Metadata["disks"])
	}
	if inv.ClusterUUID == "" {
		inv.ClusterUUID = vm.Location
	}
	if inv.DiskCount == 0 {
		inv.DiskCount = len(inv.Disks)
	}
	return inv
}
