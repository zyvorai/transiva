# Artifact Manifest v1.0 Package

This package implements the **Artifact Manifest v1.0** integration contract between transiva and hyper2kvm.

## Overview

The Artifact Manifest v1.0 is a versioned JSON/YAML specification that describes VM disk artifacts and conversion pipeline configuration. It serves as the integration contract between:

- **transiva** (export/fetch daemon) - handles provider-specific operations
- **hyper2kvm** (fix/convert engine) - performs deterministic offline transformations

## Key Features

- ✅ **Type-safe Go structs** for Artifact Manifest v1.0
- ✅ **Fluent builder API** for easy manifest creation
- ✅ **Automatic validation** against JSON schema
- ✅ **SHA-256 checksum** computation and verification
- ✅ **JSON and YAML** serialization support
- ✅ **Multi-disk support** with boot order hints
- ✅ **Comprehensive testing** (24 tests, 100% pass rate)

## Quick Start

### Creating a Manifest

```go
package main

import (
    "fmt"
    "log"
    "github.com/zyvorai/transiva/manifest"
)

func main() {
    // Create manifest using fluent builder
    m, err := manifest.NewBuilder().
        WithSource("vsphere", "vm-1234", "production-webserver", "DC1", "govc-export").
        WithVM(4, 16, "uefi", "linux", "Ubuntu 22.04", false).
        AddDisk("boot-disk", "vmdk", "/work/boot-disk.vmdk", 107374182400, 0, "boot").
        WithPipeline(true, true, true, true).
        Build()

    if err != nil {
        log.Fatal(err)
    }

    // Write to file
    if err := manifest.WriteToFile(m, "/work/artifact-manifest.json"); err != nil {
        log.Fatal(err)
    }

    fmt.Println("✅ Manifest created successfully")
}
```

### Loading and Validating a Manifest

```go
// Load manifest from file (automatically validated)
m, err := manifest.ReadFromFile("/work/artifact-manifest.json")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Loaded manifest v%s with %d disk(s)\n", m.ManifestVersion, len(m.Disks))
```

### Checksum Verification

```go
// Create manifest with checksums
m, err := manifest.NewBuilder().
    AddDiskWithChecksum("disk-0", "vmdk", "/work/disk.vmdk", 10737418240, 0, "boot", true).
    Build()

// Later, verify checksums
results, err := manifest.VerifyChecksums(m)
if err != nil {
    log.Fatal(err)
}

for diskID, valid := range results {
    if valid {
        fmt.Printf("✅ Disk %s: checksum valid\n", diskID)
    }
}
```

## Manifest Structure

### Required Fields

```json
{
  "manifest_version": "1.0",
  "disks": [
    {
      "id": "disk-0",
      "source_format": "vmdk",
      "bytes": 10737418240,
      "local_path": "/path/to/disk.vmdk"
    }
  ]
}
```

### Recommended Fields

```json
{
  "manifest_version": "1.0",
  "source": {
    "provider": "vsphere",
    "vm_id": "vm-1234",
    "vm_name": "production-server",
    "datacenter": "DC1",
    "export_timestamp": "2026-01-21T18:30:00Z",
    "export_method": "govc-export"
  },
  "vm": {
    "cpu": 4,
    "mem_gb": 16,
    "firmware": "uefi",
    "os_hint": "linux",
    "os_version": "Ubuntu 22.04"
  },
  "disks": [
    {
      "id": "boot-disk",
      "source_format": "vmdk",
      "bytes": 107374182400,
      "local_path": "/work/boot-disk.vmdk",
      "checksum": "sha256:a1b2c3d4...",
      "boot_order_hint": 0,
      "disk_type": "boot"
    }
  ],
  "pipeline": {
    "inspect": {"enabled": true, "collect_guest_info": true},
    "fix": {"enabled": true, "backup": true, "regen_initramfs": true},
    "convert": {"enabled": true, "compress": true},
    "validate": {"enabled": true, "check_image_integrity": true}
  }
}
```

## API Reference

### Builder Methods

| Method | Description |
|--------|-------------|
| `NewBuilder()` | Creates a new manifest builder |
| `WithSource()` | Sets source metadata (provider, VM ID, etc.) |
| `WithVM()` | Sets VM hardware metadata (CPU, memory, firmware) |
| `AddDisk()` | Adds a disk artifact |
| `AddDiskWithChecksum()` | Adds a disk with automatic checksum computation |
| `AddNIC()` | Adds a network interface |
| `AddNote()` | Adds an informational note |
| `AddWarning()` | Adds a warning message |
| `WithMetadata()` | Sets transiva metadata (version, job ID, tags) |
| `WithPipeline()` | Configures hyper2kvm pipeline stages |
| `WithOutput()` | Sets output configuration |
| `WithOptions()` | Sets runtime options |
| `Build()` | Returns the constructed manifest or an error |

### Validation Functions

| Function | Description |
|----------|-------------|
| `Validate(m)` | Validates manifest against schema |
| `VerifyChecksums(m)` | Verifies SHA-256 checksums for all disks |

### Serialization Functions

| Function | Description |
|----------|-------------|
| `ToJSON(m)` | Serializes manifest to JSON |
| `ToYAML(m)` | Serializes manifest to YAML |
| `FromJSON(data)` | Deserializes manifest from JSON |
| `FromYAML(data)` | Deserializes manifest from YAML |
| `WriteToFile(m, path)` | Writes manifest to file (JSON or YAML) |
| `ReadFromFile(path)` | Reads and validates manifest from file |

## Multi-Disk Support

The manifest supports multiple disks with boot order hints:

```go
m, err := manifest.NewBuilder().
    AddDisk("boot-disk", "vmdk", "/work/boot.vmdk", 100*1024*1024*1024, 0, "boot").
    AddDisk("data-disk-1", "vmdk", "/work/data1.vmdk", 200*1024*1024*1024, 1, "data").
    AddDisk("data-disk-2", "vmdk", "/work/data2.vmdk", 300*1024*1024*1024, 2, "data").
    Build()
```

**Boot Disk Selection:**
- Lowest `boot_order_hint` = primary boot disk
- Only boot disk receives offline fixes (fstab, grub, initramfs)
- All disks are converted and validated

## Validation Rules

The package enforces these validation rules:

### Disk Artifacts
- ✅ **ID**: Must match pattern `^[a-zA-Z0-9_-]+$`
- ✅ **No duplicate IDs**: Each disk must have a unique ID
- ✅ **Source format**: Must be one of: `vmdk`, `qcow2`, `raw`, `vhd`, `vhdx`, `vdi`
- ✅ **File existence**: `local_path` must exist and be readable
- ✅ **Checksum format**: If present, must match `sha256:[a-f0-9]{64}`

### VM Metadata
- ✅ **Firmware**: Must be one of: `bios`, `uefi`, `unknown`
- ✅ **Non-negative values**: CPU and memory must be >= 0

## Integration with transiva Export

Example integration in hyperexport:

```go
func exportVM(ctx context.Context, vmPath string, opts ExportOptions) error {
    // 1. Export VM disks
    result, err := client.ExportOVF(ctx, vmPath, opts)
    if err != nil {
        return err
    }

    // 2. Create Artifact Manifest
    builder := manifest.NewBuilder().
        WithSource("vsphere", vmID, vmName, datacenter, "govc-export").
        WithVM(vm.CPU, vm.MemoryGB, vm.Firmware, vm.OSHint, vm.OSVersion, false)

    // 3. Add exported disks
    for i, file := range result.Files {
        builder.AddDiskWithChecksum(
            fmt.Sprintf("disk-%d", i),
            "vmdk",
            file,
            fileSize,
            i,  // boot_order_hint
            diskType,
            true, // compute checksum
        )
    }

    // 4. Configure pipeline
    builder.WithPipeline(true, true, true, true).
        WithMetadata(hypersdkVersion, jobID, tags)

    // 5. Build and save
    m, err := builder.Build()
    if err != nil {
        return err
    }

    manifestPath := filepath.Join(opts.OutputPath, "artifact-manifest.json")
    return manifest.WriteToFile(m, manifestPath)
}
```

## Testing

Run tests:

```bash
go test ./manifest/... -v
```

**Test Coverage:**
- 24 tests
- 100% pass rate
- Covers all validation rules
- Tests checksums, serialization, and builder API

## Compliance

This package implements the **Artifact Manifest v1.0** specification as documented in:
- `/home/ssahani/tt/hyper2kvm/docs/Integration-Contract.md`
- `/home/ssahani/tt/hyper2kvm/docs/artifact-manifest-v1.0.schema.json`

The manifest format is fully compatible with hyper2kvm's ManifestLoader.

## Examples

See `example_test.go` for complete examples:
- Creating manifests
- Multi-disk VMs
- Checksum verification
- Minimal manifests
- Loading existing manifests

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-01-21 | Initial release with full Artifact Manifest v1.0 support |

## License

Apache-2.0
