// SPDX-License-Identifier: Apache-2.0

package formats

import (
	"bytes"
	"os"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name       string
		magic      []byte
		extension  string
		wantFormat DiskFormat
	}{
		{
			name:       "QCOW2 format",
			magic:      MagicQCOW2,
			extension:  ".qcow2",
			wantFormat: FormatQCOW2,
		},
		{
			name:       "VMDK format",
			magic:      MagicVMDK,
			extension:  ".vmdk",
			wantFormat: FormatVMDK,
		},
		{
			name:       "RAW format",
			magic:      []byte{0x00, 0x00, 0x00, 0x00},
			extension:  ".raw",
			wantFormat: FormatRAW,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with magic bytes
			tmpfile, err := os.CreateTemp("", "test-*"+tt.extension)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(tmpfile.Name()) }()

			// Write magic bytes
			if _, err := tmpfile.Write(tt.magic); err != nil {
				t.Fatal(err)
			}
			_ = tmpfile.Close()

			// Detect format
			got, err := DetectFormat(tmpfile.Name())
			if err != nil {
				t.Fatalf("DetectFormat() error = %v", err)
			}

			if got != tt.wantFormat {
				t.Errorf("DetectFormat() = %v, want %v", got, tt.wantFormat)
			}
		})
	}
}

func TestDetectFromExtension(t *testing.T) {
	tests := []struct {
		path string
		want DiskFormat
	}{
		{"/path/to/disk.qcow2", FormatQCOW2},
		{"/path/to/disk.vmdk", FormatVMDK},
		{"/path/to/disk.vhd", FormatVHD},
		{"/path/to/disk.vhdx", FormatVHDX},
		{"/path/to/disk.raw", FormatRAW},
		{"/path/to/disk.img", FormatRAW},
		{"/path/to/disk.unknown", FormatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectFromExtension(tt.path)
			if got != tt.want {
				t.Errorf("detectFromExtension(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectFromMagic(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		want   DiskFormat
	}{
		{
			name: "QCOW2 magic",
			data: append(MagicQCOW2, make([]byte, 508)...),
			want: FormatQCOW2,
		},
		{
			name: "VMDK magic",
			data: append(MagicVMDK, make([]byte, 509)...),
			want: FormatVMDK,
		},
		{
			name: "No magic (RAW)",
			data: make([]byte, 512),
			want: FormatRAW,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test-magic-*")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(tmpfile.Name()) }()

			if _, err := tmpfile.Write(tt.data); err != nil {
				t.Fatal(err)
			}
			_, _ = tmpfile.Seek(0, 0)

			got, err := detectFromMagic(tmpfile)
			if err != nil {
				t.Fatalf("detectFromMagic() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("detectFromMagic() = %v, want %v", got, tt.want)
			}

			_ = tmpfile.Close()
		})
	}
}

func TestParseFormatString(t *testing.T) {
	tests := []struct {
		input string
		want  DiskFormat
	}{
		{"qcow2", FormatQCOW2},
		{"QCOW2", FormatQCOW2},
		{"qcow", FormatQCOW2},
		{"vmdk", FormatVMDK},
		{"VMDK", FormatVMDK},
		{"vhd", FormatVHD},
		{"vhdx", FormatVHDX},
		{"raw", FormatRAW},
		{"RAW", FormatRAW},
		{"img", FormatRAW},
		{"invalid", FormatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseFormatString(tt.input)
			if got != tt.want {
				t.Errorf("ParseFormatString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatExtension(t *testing.T) {
	tests := []struct {
		format DiskFormat
		want   string
	}{
		{FormatQCOW2, ".qcow2"},
		{FormatVMDK, ".vmdk"},
		{FormatVHD, ".vhd"},
		{FormatVHDX, ".vhdx"},
		{FormatRAW, ".raw"},
		{FormatUnknown, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			got := tt.format.Extension()
			if got != tt.want {
				t.Errorf("Extension() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetFormatInfo(t *testing.T) {
	// Create a test QCOW2 file with basic header
	tmpfile, err := os.CreateTemp("", "test-qcow2-*.qcow2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Write QCOW2 header
	header := make([]byte, 512)
	copy(header[0:4], MagicQCOW2)
	// Version 3
	header[4] = 0
	header[5] = 0
	header[6] = 0
	header[7] = 3
	// Size: 10 GB
	size := uint64(10 * 1024 * 1024 * 1024)
	for i := 0; i < 8; i++ {
		header[24+i] = byte(size >> (56 - uint(i)*8))
	}
	_, _ = tmpfile.Write(header)
	_ = tmpfile.Close()

	// Get format info
	info, err := GetFormatInfo(tmpfile.Name())
	if err != nil {
		t.Fatalf("GetFormatInfo() error = %v", err)
	}

	if info.Format != FormatQCOW2 {
		t.Errorf("Format = %v, want %v", info.Format, FormatQCOW2)
	}

	if info.Size != 512 {
		t.Errorf("Size = %d, want 512", info.Size)
	}
}

func TestMagicBytes(t *testing.T) {
	// Verify magic byte constants
	if !bytes.Equal(MagicQCOW2, []byte{'Q', 'F', 'I', 0xfb}) {
		t.Errorf("MagicQCOW2 is incorrect")
	}

	if !bytes.Equal(MagicVMDK, []byte{0x4b, 0x44, 0x4d}) {
		t.Errorf("MagicVMDK is incorrect")
	}

	if !bytes.Equal(MagicVHD, []byte("conectix")) {
		t.Errorf("MagicVHD is incorrect")
	}

	if !bytes.Equal(MagicVHDX, []byte("vhdxfile")) {
		t.Errorf("MagicVHDX is incorrect")
	}
}

func BenchmarkDetectFormat(b *testing.B) {
	tmpfile, err := os.CreateTemp("", "bench-*"+".qcow2")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Write QCOW2 magic
	_, _ = tmpfile.Write(MagicQCOW2)
	_ = tmpfile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DetectFormat(tmpfile.Name())
	}
}

func BenchmarkParseFormatString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseFormatString("qcow2")
	}
}
