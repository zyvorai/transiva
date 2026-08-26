// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"testing"

	"github.com/zyvorai/transiva/providers"
)

func TestBuildPickupPlan(t *testing.T) {
	inv := []VMInventory{{
		Name: "web-01",
		UUID: "vm-1",
		Disks: []DiskInfo{{
			UUID:          "disk-1",
			ContainerUUID: "ctr-1",
			SizeGiB:       100,
			DeviceType:    "DISK",
			DiskAddress:   "SCSI:0",
		}},
	}}

	plan := BuildPickupPlan("prism.example.com", inv, map[string]string{"ctr-1": "default-container"})
	if plan.VMCount != 1 {
		t.Fatalf("vm_count = %d", plan.VMCount)
	}
	if len(plan.VMs[0].Disks) != 1 {
		t.Fatalf("expected 1 disk")
	}
	disk := plan.VMs[0].Disks[0]
	if disk.ContainerName != "default-container" {
		t.Fatalf("container_name = %q", disk.ContainerName)
	}
	if disk.NFSRelativePath != ".acropolis/vmdisk/disk-1/" {
		t.Fatalf("nfs path = %q", disk.NFSRelativePath)
	}
}

func TestParseDiskInfoSlice(t *testing.T) {
	raw := []interface{}{
		map[string]interface{}{
			"uuid":           "disk-1",
			"size_gib":       50.0,
			"device_type":    "DISK",
			"container_uuid": "ctr-1",
		},
	}
	disks := parseDiskInfoSlice(raw)
	if len(disks) != 1 || disks[0].UUID != "disk-1" {
		t.Fatalf("unexpected disks: %+v", disks)
	}
}

func TestInventoryFromVMInfo(t *testing.T) {
	vm := &providers.VMInfo{
		ID:         "vm-1",
		Name:       "app",
		PowerState: "ON",
		NumCPUs:    2,
		MemoryMB:   4096,
		Location:   "cluster-1",
		Tags:       map[string]string{"cluster_uuid": "cluster-1"},
		Metadata: map[string]interface{}{
			"disks": []DiskInfo{{UUID: "d1", SizeGiB: 40}},
		},
	}
	inv := InventoryFromVMInfo(vm)
	if inv.UUID != "vm-1" || len(inv.Disks) != 1 {
		t.Fatalf("unexpected inventory: %+v", inv)
	}
}

func TestDiskNFSRelativePath(t *testing.T) {
	got := DiskNFSRelativePath("abc-123")
	want := ".acropolis/vmdisk/abc-123/"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
