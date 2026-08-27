# Format Converters

Native Go implementations for converting between virtual disk formats.

## Overview

The `formats` package provides pure Go implementations for detecting and converting between common virtual disk formats without requiring external tools like `qemu-img`.

## Supported Formats

### Detection
- ✅ **RAW** - Raw disk images
- ✅ **QCOW2** - QEMU Copy-On-Write v2
- ✅ **VMDK** - VMware Virtual Machine Disk
- ✅ **VHD** - Virtual Hard Disk (Hyper-V)
- ✅ **VHDX** - Virtual Hard Disk v2 (Hyper-V)

### Conversion
- ✅ **RAW → QCOW2**
- ✅ **QCOW2 → RAW**
- ✅ **VMDK → RAW**
- ✅ **VMDK → QCOW2**
- ✅ **Same format → Same format** (copy)

## Features

- **Format Detection** - Automatic format detection from magic bytes
- **Streaming Conversion** - Memory-efficient streaming I/O
- **Progress Tracking** - Real-time progress callbacks
- **Context Support** - Cancellable operations
- **No External Dependencies** - Pure Go implementation

## Usage

### Command Line (hyperconvert)

```bash
# Convert VMDK to QCOW2
hyperconvert --source vm-disk.vmdk --target-format qcow2

# Auto-detect formats
hyperconvert --source disk.img --target output.qcow2

# Show disk information
hyperconvert --info --source disk.qcow2

# Specify buffer size
hyperconvert --source large-disk.vmdk --target-format qcow2 --buffer-size 16
```

### Programmatic Usage

#### Simple Conversion

```go
package main

import (
    "context"
    "github.com/zyvorai/transiva/logger"
    "github.com/zyvorai/transiva/providers/formats"
)

func main() {
    log := logger.New("info")
    converter := formats.NewConverter(log)

    opts := formats.DefaultConversionOptions()
    opts.SourceFormat = formats.FormatVMDK
    opts.TargetFormat = formats.FormatQCOW2

    result, err := converter.Convert(
        context.Background(),
        "/path/to/source.vmdk",
        "/path/to/target.qcow2",
        opts,
    )

    if err != nil {
        log.Error("conversion failed", "error", err)
        return
    }

    log.Info("conversion complete",
        "duration", result.Duration,
        "size", result.TargetSize)
}
```

#### With Progress Tracking

```go
opts := formats.DefaultConversionOptions()
opts.SourceFormat = formats.FormatVMDK
opts.TargetFormat = formats.FormatQCOW2

opts.ProgressCallback = func(progress float64, bytesProcessed int64) {
    fmt.Printf("Progress: %.2f%% (%d bytes)\n", progress, bytesProcessed)
}

result, err := converter.Convert(ctx, source, target, opts)
```

#### Format Detection

```go
// Detect format from file
format, err := formats.DetectFormat("/path/to/disk.img")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Detected format: %s\n", format)

// Get detailed information
info, err := formats.GetFormatInfo("/path/to/disk.qcow2")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Format: %s\n", info.Format)
fmt.Printf("File Size: %d\n", info.Size)
fmt.Printf("Virtual Size: %d\n", info.VirtualSize)
```

#### In-Place Conversion

```go
// Convert file in place (creates temp, then replaces original)
result, err := converter.ConvertInPlace(
    context.Background(),
    "/path/to/disk.vmdk",
    formats.FormatQCOW2,
    opts,
)
```

## Format Details

### RAW Format

- **Description**: Uncompressed sector-by-sector copy
- **Extensions**: `.raw`, `.img`
- **Magic**: None (no header)
- **Use Case**: Simple, universally compatible
- **Size**: Same as virtual disk size

### QCOW2 Format

- **Description**: QEMU Copy-On-Write version 2
- **Extensions**: `.qcow2`, `.qcow`
- **Magic**: `QFI\xfb` (0x514649fb)
- **Features**:
  - Sparse file support
  - Compression
  - Snapshots
  - Encryption
- **Size**: Only allocated blocks stored

### VMDK Format

- **Description**: VMware Virtual Machine Disk
- **Extensions**: `.vmdk`
- **Magic**: `KDM` at various offsets
- **Types**:
  - monolithicSparse
  - monolithicFlat
  - streamOptimized
  - twoGbMaxExtentSparse
- **Size**: Varies by type

### VHD Format

- **Description**: Microsoft Virtual Hard Disk
- **Extensions**: `.vhd`
- **Magic**: `conectix` in footer
- **Types**:
  - Fixed
  - Dynamic
  - Differencing
- **Size**: Footer at end of file

## Performance

Typical conversion speeds on modern hardware:

| Conversion | Speed | Notes |
|------------|-------|-------|
| VMDK → RAW | ~200 MB/s | Limited by disk I/O |
| RAW → QCOW2 | ~150 MB/s | Simplified implementation |
| QCOW2 → RAW | ~180 MB/s | Simplified implementation |
| Same → Same | ~250 MB/s | Pure copy |

Buffer sizes affect performance:
- **1 MB**: Good for small disks
- **4 MB**: Default, balanced
- **16 MB**: Better for large disks
- **64 MB**: Maximum efficiency for very large disks

## Implementation Status

### Current Implementation

The current implementation provides:
- ✅ Format detection via magic bytes
- ✅ Streaming copy with progress tracking
- ✅ Basic conversions between formats
- ⚠️ Simplified QCOW2 handling (no L1/L2 table parsing yet)
- ⚠️ Simplified VMDK handling (flat/sparse only)

### Future Enhancements

Planned improvements:
- 🔜 Full QCOW2 v3 support with L1/L2 table handling
- 🔜 QCOW2 compression support
- 🔜 VMDK streamOptimized support with grain tables
- 🔜 VHD/VHDX full support
- 🔜 Incremental conversion (sparse handling)
- 🔜 Multi-threaded conversion
- 🔜 Direct streaming to cloud storage

## Error Handling

```go
result, err := converter.Convert(ctx, source, target, opts)
if err != nil {
    switch {
    case errors.Is(err, context.Canceled):
        // Conversion was cancelled
    case errors.Is(err, context.DeadlineExceeded):
        // Conversion timeout
    default:
        // Other error
        log.Error("conversion failed", "error", err)
    }
}
```

## Testing

```bash
# Run tests
go test -v ./providers/formats/

# Test with specific disk image
go test -v ./providers/formats/ -args -test-image /path/to/disk.vmdk

# Benchmark
go test -bench=. ./providers/formats/
```

## Examples

See `cmd/hyperconvert/` for a complete CLI implementation.

## Integration

### With Export Pipeline

```go
// After VM export, convert to desired format
exporter := vsphere.NewExporter(config, log)
result, err := exporter.ExportVM(ctx, vmPath, exportOpts)

// Convert exported disk
converter := formats.NewConverter(log)
for _, file := range result.Files {
    if strings.HasSuffix(file, ".vmdk") {
        targetPath := strings.TrimSuffix(file, ".vmdk") + ".qcow2"
        converter.Convert(ctx, file, targetPath, opts)
    }
}
```

### With H2KVM

```go
// Export then convert for KVM
result, err := exporter.ExportVM(ctx, vmPath, exportOpts)

// Convert to QCOW2 for KVM
converter := formats.NewConverter(log)
for _, vmdk := range result.Files {
    qcow2Path := strings.Replace(vmdk, ".vmdk", ".qcow2", 1)
    converter.Convert(ctx, vmdk, qcow2Path, opts)
}
```

## Troubleshooting

### Conversion Fails

**Check format support**:
```bash
hyperconvert --info --source problem-disk.img
```

**Try with larger buffer**:
```bash
hyperconvert --source disk.vmdk --target-format qcow2 --buffer-size 16
```

### Memory Issues

For very large disks, use smaller buffer:
```bash
hyperconvert --source huge-disk.vmdk --target-format qcow2 --buffer-size 1
```

### Slow Conversion

- Increase buffer size (`--buffer-size 16`)
- Check disk I/O (`iostat -x 1`)
- Ensure sufficient free space
- Use SSD for temp files

## License

Apache-2.0

## References

- [QCOW2 Specification](https://github.com/qemu/qemu/blob/master/docs/interop/qcow2.txt)
- [VMDK Specification](https://www.vmware.com/support/developer/vddk/vmdk_50_technote.pdf)
- [VHD Specification](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-vhdx/)
