// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDiskSource(t *testing.T) {
	root := t.TempDir()
	diskUUID := "disk-abc"
	base := filepath.Join(root, ".acropolis", "vmdisk", diskUUID)
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(base, "meta.json")
	if err := os.WriteFile(meta, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	flat := filepath.Join(base, "disk-flat.img")
	if err := os.WriteFile(flat, make([]byte, 2*1024*1024), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := FindDiskSource(root, PickupDisk{
		DiskInfo:        DiskInfo{UUID: diskUUID},
		NFSRelativePath: DiskNFSRelativePath(diskUUID),
	})
	if err != nil {
		t.Fatalf("FindDiskSource: %v", err)
	}
	if got != flat {
		t.Fatalf("got %q want %q", got, flat)
	}
}

func TestParseMountMap(t *testing.T) {
	m, err := ParseMountMap([]string{"default-container:/mnt/default", "ctr-uuid:/mnt/other"})
	if err != nil {
		t.Fatal(err)
	}
	if m["default-container"] != "/mnt/default" {
		t.Fatalf("unexpected map: %v", m)
	}
}

func TestMountMapResolveMount(t *testing.T) {
	m := MountMap{"default-container": "/mnt/default"}
	disk := PickupDisk{
		ContainerName: "default-container",
		DiskInfo:      DiskInfo{ContainerUUID: "uuid-1"},
	}
	path, err := m.ResolveMount(disk)
	if err != nil || path != "/mnt/default" {
		t.Fatalf("ResolveMount = %q err=%v", path, err)
	}
}

func TestBuildArtifactManifest(t *testing.T) {
	out := t.TempDir()
	diskPath := filepath.Join(out, "disk-0.qcow2")
	if err := os.WriteFile(diskPath, []byte("fake qcow2"), 0644); err != nil {
		t.Fatal(err)
	}

	vm := PickupVM{
		VMInventory: VMInventory{
			Name:      "web-01",
			UUID:      "vm-1",
			VCPUs:     2,
			MemoryGiB: 4,
		},
		Disks: []PickupDisk{{
			DiskInfo: DiskInfo{UUID: "disk-1", DiskAddress: "SCSI:0", SizeGiB: 40},
		}},
	}
	converted := []ConvertedDisk{{
		PickupDisk: vm.Disks[0],
		OutputPath: diskPath,
		Bytes:      10,
	}}

	artifact, err := BuildArtifactManifest(vm, converted, "qcow2")
	if err != nil {
		t.Fatalf("BuildArtifactManifest: %v", err)
	}
	if artifact.Source.Provider != "nutanix" {
		t.Fatalf("provider = %q", artifact.Source.Provider)
	}
	if len(artifact.Disks) != 1 {
		t.Fatalf("disks = %d", len(artifact.Disks))
	}
}
