// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"encoding/json"
	"fmt"
	"time"
)

const pickupPlanVersion = "1.0"

// PickupPlan describes Nutanix VM disks ready for NFS mount and qemu-img conversion.
type PickupPlan struct {
	Version     string     `json:"version"`
	GeneratedAt time.Time  `json:"generated_at"`
	PrismHost   string     `json:"prism_host"`
	VMCount     int        `json:"vm_count"`
	VMs         []PickupVM `json:"vms"`
	Notes       []string   `json:"notes,omitempty"`
}

// PickupVM extends inventory with migration pickup paths per disk.
type PickupVM struct {
	VMInventory
	Disks []PickupDisk `json:"disks"`
}

// PickupDisk adds NFS-relative paths and optional container name for offline pickup.
type PickupDisk struct {
	DiskInfo
	ContainerName   string `json:"container_name,omitempty"`
	NFSRelativePath string `json:"nfs_relative_path"`
}

// BuildPickupPlan creates a migration pickup plan from VM inventory.
func BuildPickupPlan(host string, inventory []VMInventory, containerNames map[string]string) PickupPlan {
	plan := PickupPlan{
		Version:     pickupPlanVersion,
		GeneratedAt: time.Now().UTC(),
		PrismHost:   host,
		VMCount:     len(inventory),
		Notes: []string{
			"Mount storage containers via NFS on the migration host, then locate disks under nfs_relative_path.",
			"Typical mount layout: <container_mount>/.acropolis/vmdisk/<disk_uuid>/",
			"Convert with: qemu-img convert -O qcow2 <source> <dest.qcow2>",
		},
	}

	for _, inv := range inventory {
		pvm := PickupVM{VMInventory: inv}
		for _, disk := range inv.Disks {
			pd := PickupDisk{
				DiskInfo:        disk,
				NFSRelativePath: DiskNFSRelativePath(disk.UUID),
			}
			if containerNames != nil && disk.ContainerUUID != "" {
				pd.ContainerName = containerNames[disk.ContainerUUID]
			}
			pvm.Disks = append(pvm.Disks, pd)
		}
		plan.VMs = append(plan.VMs, pvm)
	}

	return plan
}

// DiskNFSRelativePath returns the path under a storage container NFS mount.
func DiskNFSRelativePath(diskUUID string) string {
	return fmt.Sprintf(".acropolis/vmdisk/%s/", diskUUID)
}

// DisksFromVMInfo extracts disk metadata from provider VMInfo metadata.
func DisksFromVMInfo(meta map[string]interface{}) []DiskInfo {
	if meta == nil {
		return nil
	}
	raw, ok := meta["disks"]
	if !ok || raw == nil {
		return nil
	}
	return parseDiskInfoSlice(raw)
}

// parseDiskInfoSlice handles in-memory and JSON-decoded disk metadata.
func parseDiskInfoSlice(raw interface{}) []DiskInfo {
	switch v := raw.(type) {
	case []DiskInfo:
		return v
	case []interface{}:
		out := make([]DiskInfo, 0, len(v))
		for _, item := range v {
			switch d := item.(type) {
			case DiskInfo:
				out = append(out, d)
			case map[string]interface{}:
				if disk, err := diskInfoFromMap(d); err == nil {
					out = append(out, disk)
				}
			}
		}
		return out
	default:
		data, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		var disks []DiskInfo
		if err := json.Unmarshal(data, &disks); err != nil {
			return nil
		}
		return disks
	}
}

func diskInfoFromMap(m map[string]interface{}) (DiskInfo, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return DiskInfo{}, err
	}
	var disk DiskInfo
	if err := json.Unmarshal(data, &disk); err != nil {
		return DiskInfo{}, err
	}
	return disk, nil
}
