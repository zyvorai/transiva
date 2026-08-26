// SPDX-License-Identifier: Apache-2.0

package formats

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// DiskFormat represents a virtual disk format
type DiskFormat string

const (
	FormatRAW   DiskFormat = "raw"
	FormatQCOW2 DiskFormat = "qcow2"
	FormatVMDK  DiskFormat = "vmdk"
	FormatVHD   DiskFormat = "vhd"
	FormatVHDX  DiskFormat = "vhdx"
	FormatUnknown DiskFormat = "unknown"
)

// Magic bytes for format detection
var (
	MagicQCOW2 = []byte{'Q', 'F', 'I', 0xfb} // QCOW2 magic: QFI\xfb
	MagicVMDK  = []byte{0x4b, 0x44, 0x4d}    // VMDK magic: KDM
	MagicVHD   = []byte("conectix")           // VHD footer signature
	MagicVHDX  = []byte("vhdxfile")           // VHDX header signature
)

// FormatInfo contains information about a detected format
type FormatInfo struct {
	Format      DiskFormat
	Size        int64
	VirtualSize int64
	Compressed  bool
	Metadata    map[string]interface{}
}

// DetectFormat detects the disk format from a file
func DetectFormat(path string) (DiskFormat, error) {
	// #nosec G304 -- path is a disk image path supplied by the local CLI
	// operator (see cmd/hyperconvert), not derived from an HTTP request.
	file, err := os.Open(path)
	if err != nil {
		return FormatUnknown, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Try extension first
	if format := detectFromExtension(path); format != FormatUnknown {
		// Verify with magic bytes
		if verified, err := verifyFormat(file, format); err == nil && verified {
			return format, nil
		}
	}

	// Detect from magic bytes
	return detectFromMagic(file)
}

// detectFromExtension guesses format from file extension
func detectFromExtension(path string) DiskFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".qcow2", ".qcow":
		return FormatQCOW2
	case ".vmdk":
		return FormatVMDK
	case ".vhd":
		return FormatVHD
	case ".vhdx":
		return FormatVHDX
	case ".raw", ".img":
		return FormatRAW
	default:
		return FormatUnknown
	}
}

// detectFromMagic detects format from magic bytes
func detectFromMagic(file *os.File) (DiskFormat, error) {
	// Read first 512 bytes for magic detection
	header := make([]byte, 512)
	if _, err := file.ReadAt(header, 0); err != nil && err != io.EOF {
		return FormatUnknown, fmt.Errorf("failed to read header: %w", err)
	}

	// Check QCOW2 (magic at offset 0)
	if bytes.Equal(header[0:4], MagicQCOW2) {
		return FormatQCOW2, nil
	}

	// Check VMDK (magic can be at offset 0 or in descriptor)
	if bytes.Contains(header, MagicVMDK) {
		return FormatVMDK, nil
	}

	// Check VHDX (magic at offset 0)
	if bytes.Equal(header[0:8], MagicVHDX) {
		return FormatVHDX, nil
	}

	// Check VHD (magic in footer, last 512 bytes of file)
	stat, err := file.Stat()
	if err == nil && stat.Size() >= 512 {
		footer := make([]byte, 512)
		if _, err := file.ReadAt(footer, stat.Size()-512); err == nil {
			if bytes.Contains(footer, MagicVHD) {
				return FormatVHD, nil
			}
		}
	}

	// If no magic found, assume RAW
	return FormatRAW, nil
}

// verifyFormat verifies a guessed format matches the actual format
func verifyFormat(file *os.File, expected DiskFormat) (bool, error) {
	detected, err := detectFromMagic(file)
	if err != nil {
		return false, err
	}

	// RAW format has no magic, so extension match is enough
	if expected == FormatRAW {
		return detected == FormatRAW || detected == FormatUnknown, nil
	}

	return detected == expected, nil
}

// GetFormatInfo returns detailed information about a disk image
func GetFormatInfo(path string) (*FormatInfo, error) {
	format, err := DetectFormat(path)
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- path is a disk image path supplied by the local CLI
	// operator (see cmd/hyperconvert), not derived from an HTTP request.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	info := &FormatInfo{
		Format:   format,
		Size:     stat.Size(),
		Metadata: make(map[string]interface{}),
	}

	// Read format-specific metadata
	switch format {
	case FormatQCOW2:
		if err := readQCOW2Info(file, info); err != nil {
			return nil, err
		}
	case FormatVMDK:
		if err := readVMDKInfo(file, info); err != nil {
			return nil, err
		}
	case FormatVHD:
		if err := readVHDInfo(file, info); err != nil {
			return nil, err
		}
	case FormatRAW:
		info.VirtualSize = stat.Size()
		info.Compressed = false
	}

	return info, nil
}

// readQCOW2Info reads QCOW2 header information
func readQCOW2Info(file *os.File, info *FormatInfo) error {
	// QCOW2 header structure (simplified)
	var header struct {
		Magic         uint32
		Version       uint32
		BackingOffset uint64
		BackingSize   uint32
		ClusterBits   uint32
		Size          uint64
		CryptMethod   uint32
	}

	if err := binary.Read(file, binary.BigEndian, &header); err != nil {
		return fmt.Errorf("failed to read QCOW2 header: %w", err)
	}

	if header.Size > math.MaxInt64 {
		return fmt.Errorf("QCOW2 virtual size %d overflows int64", header.Size)
	}
	info.VirtualSize = int64(header.Size)
	info.Compressed = false // QCOW2 compression is per-cluster
	info.Metadata["version"] = header.Version
	info.Metadata["cluster_bits"] = header.ClusterBits
	info.Metadata["cluster_size"] = 1 << header.ClusterBits

	return nil
}

// readVMDKInfo reads VMDK descriptor information
func readVMDKInfo(file *os.File, info *FormatInfo) error {
	// Read descriptor (first 20KB typically)
	descriptor := make([]byte, 20480)
	n, err := file.Read(descriptor)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read VMDK descriptor: %w", err)
	}

	descriptor = descriptor[:n]

	// Parse descriptor for size info
	lines := strings.Split(string(descriptor), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for extent size
		if strings.HasPrefix(line, "RW ") || strings.HasPrefix(line, "RDONLY ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var size int64
				if _, err := fmt.Sscanf(fields[1], "%d", &size); err != nil {
					return fmt.Errorf("failed to parse VMDK extent size %q: %w", fields[1], err)
				}
				info.VirtualSize = size * 512 // VMDK size is in sectors
				break
			}
		}
	}

	info.Compressed = false
	return nil
}

// readVHDInfo reads VHD footer information
func readVHDInfo(file *os.File, info *FormatInfo) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// VHD footer is last 512 bytes
	footer := make([]byte, 512)
	if _, err := file.ReadAt(footer, stat.Size()-512); err != nil {
		return fmt.Errorf("failed to read VHD footer: %w", err)
	}

	// Current size at offset 48 (8 bytes, big-endian)
	vhdSize := binary.BigEndian.Uint64(footer[48:56])
	if vhdSize > math.MaxInt64 {
		return fmt.Errorf("VHD virtual size %d overflows int64", vhdSize)
	}
	info.VirtualSize = int64(vhdSize)
	info.Compressed = false

	return nil
}

// ParseFormatString parses a format string into DiskFormat
func ParseFormatString(s string) DiskFormat {
	switch strings.ToLower(s) {
	case "qcow2", "qcow":
		return FormatQCOW2
	case "vmdk":
		return FormatVMDK
	case "vhd":
		return FormatVHD
	case "vhdx":
		return FormatVHDX
	case "raw", "img":
		return FormatRAW
	default:
		return FormatUnknown
	}
}

// String returns the string representation of a format
func (f DiskFormat) String() string {
	return string(f)
}

// Extension returns the typical file extension for a format
func (f DiskFormat) Extension() string {
	switch f {
	case FormatQCOW2:
		return ".qcow2"
	case FormatVMDK:
		return ".vmdk"
	case FormatVHD:
		return ".vhd"
	case FormatVHDX:
		return ".vhdx"
	case FormatRAW:
		return ".raw"
	default:
		return ""
	}
}
