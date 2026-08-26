// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/zyvorai/transiva/manifest"
)

// ExampleBuilder demonstrates how to create an Artifact Manifest v1.0
// using the fluent builder API.
func ExampleBuilder() {
	// Create output directory
	outputDir := "/work/export-vsphere-vm-1234"
	_ = os.MkdirAll(outputDir, 0755)

	// Create example disk files (in real usage, these would be exported disks)
	bootDiskPath := filepath.Join(outputDir, "boot-disk.vmdk")
	dataDiskPath := filepath.Join(outputDir, "data-disk.vmdk")

	// Create manifest using fluent builder
	m, err := manifest.NewBuilder().
		// Source metadata (where the VM came from)
		WithSource(
			"vsphere",                 // provider
			"vm-1234",                 // VM ID
			"production-webserver-01", // VM name
			"DC1",                     // datacenter
			"govc-export",             // export method
		).
		// VM hardware metadata
		WithVM(
			4,              // CPUs
			16,             // memory GB
			"uefi",         // firmware
			"linux",        // OS hint
			"Ubuntu 22.04", // OS version
			false,          // secure boot
		).
		// Add boot disk
		AddDisk(
			"boot-disk",  // disk ID
			"vmdk",       // source format
			bootDiskPath, // local path
			107374182400, // bytes (100 GB)
			0,            // boot order hint (0 = primary)
			"boot",       // disk type
		).
		// Add data disk
		AddDisk(
			"data-disk-1", // disk ID
			"vmdk",        // source format
			dataDiskPath,  // local path
			214748364800,  // bytes (200 GB)
			1,             // boot order hint (1 = secondary)
			"data",        // disk type
		).
		// Add network interface
		AddNIC(
			"eth0",              // ID
			"00:50:56:ab:cd:ef", // MAC address
			"VM Network",        // network name
		).
		// Add informational notes
		AddNote("Exported from vSphere 7.0").
		AddNote("VM was powered off during export").
		// Configure transiva metadata
		WithMetadata(
			"0.1.0",   // transiva version
			"job-123", // job ID
			map[string]string{
				"environment": "production",
				"team":        "ops",
				"application": "webserver",
			},
		).
		// Configure hyper2kvm pipeline
		WithPipeline(
			true, // inspect
			true, // fix
			true, // convert
			true, // validate
		).
		// Configure output
		WithOutput(
			outputDir, // output directory
			"qcow2",   // format
			"",        // filename (auto-generated)
		).
		// Build the manifest
		Build()

	if err != nil {
		log.Fatalf("Failed to build manifest: %v", err)
	}

	// Write manifest to file
	manifestPath := filepath.Join(outputDir, "artifact-manifest.json")
	if err := manifest.WriteToFile(m, manifestPath); err != nil {
		log.Fatalf("Failed to write manifest: %v", err)
	}

	fmt.Printf("✅ Artifact Manifest v1.0 created: %s\n", manifestPath)
	fmt.Printf("   Source: %s / %s\n", m.Source.Provider, m.Source.VMName)
	fmt.Printf("   Disks: %d\n", len(m.Disks))
	fmt.Printf("   Ready for hyper2kvm processing\n")
}

// Example_builderWithChecksums demonstrates creating a manifest with
// checksum verification enabled.
func Example_builderWithChecksums() {
	outputDir := "/work/export-vm-checksums"
	_ = os.MkdirAll(outputDir, 0755)

	diskPath := filepath.Join(outputDir, "disk.vmdk")

	// Create manifest with automatic checksum computation
	m, err := manifest.NewBuilder().
		WithSource("vsphere", "vm-5678", "test-vm", "DC1", "govc-export").
		// AddDiskWithChecksum automatically computes SHA-256
		AddDiskWithChecksum(
			"disk-0",
			"vmdk",
			diskPath,
			10737418240, // 10 GB
			0,
			"boot",
			true, // compute checksum
		).
		WithPipeline(true, true, true, true).
		Build()

	if err != nil {
		log.Fatalf("Failed to build manifest: %v", err)
	}

	// Verify the checksum was computed
	if m.Disks[0].Checksum != "" {
		fmt.Printf("✅ Checksum computed: %s\n", m.Disks[0].Checksum[:20]+"...")
	}

	// Later, verify checksums match
	results, err := manifest.VerifyChecksums(m)
	if err != nil {
		log.Fatalf("Checksum verification failed: %v", err)
	}

	for diskID, valid := range results {
		if valid {
			fmt.Printf("✅ Disk %s: checksum valid\n", diskID)
		} else {
			fmt.Printf("❌ Disk %s: checksum mismatch\n", diskID)
		}
	}
}

// Example_loadManifest demonstrates loading and validating an existing
// Artifact Manifest from a file.
func Example_loadManifest() {
	manifestPath := "/work/export-vm/artifact-manifest.json"

	// Load manifest from file
	m, err := manifest.ReadFromFile(manifestPath)
	if err != nil {
		log.Fatalf("Failed to load manifest: %v", err)
	}

	// The manifest is automatically validated during load
	fmt.Printf("✅ Loaded Artifact Manifest v%s\n", m.ManifestVersion)
	fmt.Printf("   Source: %s\n", m.Source.Provider)
	fmt.Printf("   VM: %s\n", m.Source.VMName)
	fmt.Printf("   Disks: %d\n", len(m.Disks))

	// Access disk information
	for i, disk := range m.Disks {
		fmt.Printf("   Disk %d: %s (%s, %d bytes)\n",
			i, disk.ID, disk.SourceFormat, disk.Bytes)
	}

	// Verify checksums if present
	if len(m.Disks) > 0 && m.Disks[0].Checksum != "" {
		results, err := manifest.VerifyChecksums(m)
		if err != nil {
			log.Printf("Warning: checksum verification failed: %v", err)
		} else {
			for diskID, valid := range results {
				if valid {
					fmt.Printf("✅ Disk %s verified\n", diskID)
				}
			}
		}
	}
}

// Example_minimalManifest demonstrates creating the absolute minimum
// required manifest (for testing or simple use cases).
func Example_minimalManifest() {
	diskPath := "/tmp/test-disk.vmdk"

	// Minimal manifest: just version and one disk
	m, err := manifest.NewBuilder().
		AddDisk("disk-0", "vmdk", diskPath, 1024, 0, "boot").
		Build()

	if err != nil {
		log.Fatalf("Failed to build minimal manifest: %v", err)
	}

	data, _ := manifest.ToJSON(m)
	fmt.Printf("Minimal manifest:\n%s\n", string(data))

	// Output (formatted):
	// {
	//   "manifest_version": "1.0",
	//   "disks": [
	//     {
	//       "id": "disk-0",
	//       "source_format": "vmdk",
	//       "bytes": 1024,
	//       "local_path": "/tmp/test-disk.vmdk",
	//       "boot_order_hint": 0,
	//       "label": "disk-0",
	//       "disk_type": "boot"
	//     }
	//   ]
	// }
}
