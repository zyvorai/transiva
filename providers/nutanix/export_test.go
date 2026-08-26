// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"testing"
	"time"

	"github.com/zyvorai/transiva/providers"
)

func TestMergeExportOptions(t *testing.T) {
	p := &Provider{
		cfg: providers.ProviderConfig{
			Metadata: map[string]interface{}{
				"output_dir":     "/data/exports",
				"export_format":  "raw",
				"mounts":         map[string]string{"ctr-a": "/mnt/a"},
				"enable_pipeline": true,
			},
		},
	}

	opts, err := mergeExportOptions(p, providers.ExportOptions{
		OutputPath: "/override/out",
		Format:     "qcow2",
		Metadata: map[string]interface{}{
			"mounts": []string{"ctr-b:/mnt/b"},
		},
	})
	if err != nil {
		t.Fatalf("mergeExportOptions: %v", err)
	}
	if opts.OutputDir != "/override/out" {
		t.Fatalf("OutputDir = %q", opts.OutputDir)
	}
	if opts.Format != "qcow2" {
		t.Fatalf("Format = %q", opts.Format)
	}
	if opts.Mounts["ctr-b"] != "/mnt/b" {
		t.Fatalf("Mounts = %v", opts.Mounts)
	}
	if !opts.EnablePipeline {
		t.Fatal("expected enable_pipeline from provider metadata")
	}
}

func TestMergeExportOptionsRequiresMounts(t *testing.T) {
	p := &Provider{
		cfg: providers.ProviderConfig{
			Metadata: map[string]interface{}{
				"output_dir": "/data/exports",
			},
		},
	}
	_, err := mergeExportOptions(p, providers.ExportOptions{})
	if err == nil {
		t.Fatal("expected error for missing mounts")
	}
}

func TestMountsFromMetadata(t *testing.T) {
	m, err := mountsFromMetadata("default:/mnt/default,uuid-1:/mnt/other")
	if err != nil {
		t.Fatalf("mountsFromMetadata: %v", err)
	}
	if m["default"] != "/mnt/default" || m["uuid-1"] != "/mnt/other" {
		t.Fatalf("unexpected mounts: %v", m)
	}

	m, err = mountsFromMetadata(map[string]interface{}{
		"ctr": "/mnt/ctr",
	})
	if err != nil {
		t.Fatalf("map mounts: %v", err)
	}
	if m["ctr"] != "/mnt/ctr" {
		t.Fatalf("unexpected map mounts: %v", m)
	}
}

func TestMetaDuration(t *testing.T) {
	meta := map[string]interface{}{
		"pipeline_timeout": "5m",
	}
	d, ok := metaDuration(meta, "pipeline_timeout")
	if !ok || d != 5*time.Minute {
		t.Fatalf("metaDuration = %v, ok=%v", d, ok)
	}
}

func TestInventoryToPickupVM(t *testing.T) {
	inv := VMInventory{
		Name: "web-01",
		UUID: "vm-uuid",
		Disks: []DiskInfo{{
			UUID:          "disk-uuid",
			ContainerUUID: "container-uuid",
		}},
	}
	pvm := inventoryToPickupVM(inv, map[string]string{"container-uuid": "default-container"}, "pc.example.com")
	if pvm.Name != "web-01" {
		t.Fatalf("name = %q", pvm.Name)
	}
	if len(pvm.Disks) != 1 {
		t.Fatalf("disks = %d", len(pvm.Disks))
	}
	if pvm.Disks[0].NFSRelativePath != DiskNFSRelativePath("disk-uuid") {
		t.Fatalf("nfs path = %q", pvm.Disks[0].NFSRelativePath)
	}
	if pvm.Disks[0].ContainerName != "default-container" {
		t.Fatalf("container name = %q", pvm.Disks[0].ContainerName)
	}
}
