// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

func TestProvider_Name(t *testing.T) {
	p := &Provider{logger: logger.New("info")}
	if got := p.Name(); got != "Nutanix AHV" {
		t.Errorf("Name() = %q, want Nutanix AHV", got)
	}
}

func TestProvider_Type(t *testing.T) {
	p := &Provider{logger: logger.New("info")}
	if got := p.Type(); got != providers.ProviderNutanix {
		t.Errorf("Type() = %v, want %v", got, providers.ProviderNutanix)
	}
}

func TestProvider_GetExportCapabilities(t *testing.T) {
	p := &Provider{logger: logger.New("info")}
	caps := p.GetExportCapabilities()
	if len(caps.SupportedFormats) != 2 {
		t.Fatalf("expected 2 supported formats, got %v", caps.SupportedFormats)
	}
	if len(caps.SupportedTargets) != 1 || caps.SupportedTargets[0] != "local" {
		t.Fatalf("expected local target, got %v", caps.SupportedTargets)
	}
}

func TestInventoryToVMInfo(t *testing.T) {
	inv := VMInventory{
		Name:         "web-prod-01",
		UUID:         "vm-uuid",
		ClusterUUID:  "cluster-uuid",
		PowerState:   "ON",
		VCPUs:        4,
		MemoryGiB:    16,
		DiskCount:    1,
		TotalDiskGiB: 100,
		NICCount:     2,
		Disks: []DiskInfo{{
			UUID:          "disk-uuid",
			SizeGiB:       100,
			DeviceType:    "DISK",
			DiskAddress:   "SCSI:0",
			ContainerUUID: "container-uuid",
		}},
	}

	vm := inventoryToVMInfo(inv)
	if vm.Provider != providers.ProviderNutanix {
		t.Fatalf("provider = %v", vm.Provider)
	}
	if vm.State != "running" {
		t.Fatalf("state = %q", vm.State)
	}
	if vm.Metadata["disk_count"] != 1 {
		t.Fatalf("disk_count = %v", vm.Metadata["disk_count"])
	}
}

func TestClusterMatches(t *testing.T) {
	inv := VMInventory{ClusterUUID: "abc-123", ClusterName: "Prod-Cluster"}
	if !clusterMatches(inv, "Prod-Cluster") {
		t.Fatal("expected cluster name match")
	}
	if !clusterMatches(inv, "abc-123") {
		t.Fatal("expected cluster UUID match")
	}
	if clusterMatches(inv, "other") {
		t.Fatal("expected no match")
	}
}

func TestBytesToGiB(t *testing.T) {
	size := int64(1073741824)
	got := bytesToGiB(&size)
	if got != 1 {
		t.Fatalf("bytesToGiB = %v, want 1", got)
	}
}

func TestMatchesFilter(t *testing.T) {
	vm := inventoryToVMInfo(VMInventory{Name: "web-prod", PowerState: "ON", VCPUs: 4, MemoryGiB: 8})
	if !matchesFilter(vm, providers.VMFilter{NamePattern: "web"}) {
		t.Fatal("expected name filter match")
	}
	if matchesFilter(vm, providers.VMFilter{NamePattern: "db"}) {
		t.Fatal("expected name filter miss")
	}
}
