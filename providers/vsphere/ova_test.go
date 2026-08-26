// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zyvorai/transiva/logger"
)

func TestCreateOVA_Uncompressed(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create test OVF files
	ovfFile := filepath.Join(tmpDir, "test.ovf")
	vmdkFile := filepath.Join(tmpDir, "test.vmdk")
	mfFile := filepath.Join(tmpDir, "test.mf")

	if err := os.WriteFile(ovfFile, []byte("OVF content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vmdkFile, []byte("VMDK content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mfFile, []byte("Manifest content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create OVA
	ovaPath := filepath.Join(tmpDir, "test.ova")
	log := logger.New("debug")

	err := CreateOVA(tmpDir, ovaPath, false, 0, log)
	if err != nil {
		t.Fatalf("CreateOVA failed: %v", err)
	}

	// Verify OVA was created
	if _, err := os.Stat(ovaPath); os.IsNotExist(err) {
		t.Fatal("OVA file was not created")
	}

	// Verify OVA structure
	if err := verifyOVAStructure(ovaPath, false); err != nil {
		t.Errorf("OVA structure verification failed: %v", err)
	}
}

func TestCreateOVA_Compressed(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create test OVF files
	ovfFile := filepath.Join(tmpDir, "test.ovf")
	vmdkFile := filepath.Join(tmpDir, "test.vmdk")

	content := strings.Repeat("A", 10000) // Compressible content
	if err := os.WriteFile(ovfFile, []byte("OVF:"+content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vmdkFile, []byte("VMDK:"+content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create compressed OVA
	ovaPath := filepath.Join(tmpDir, "test.ova.gz")
	log := logger.New("debug")

	err := CreateOVA(tmpDir, ovaPath, true, 6, log)
	if err != nil {
		t.Fatalf("CreateOVA with compression failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(ovaPath); os.IsNotExist(err) {
		t.Fatal("Compressed OVA file was not created")
	}

	// Check that compressed file is smaller than uncompressed
	compressedInfo, _ := os.Stat(ovaPath)

	// Create uncompressed version for comparison
	uncompressedPath := filepath.Join(tmpDir, "test-uncompressed.ova")
	_ = CreateOVA(tmpDir, uncompressedPath, false, 0, log)
	uncompressedInfo, _ := os.Stat(uncompressedPath)

	if compressedInfo.Size() >= uncompressedInfo.Size() {
		t.Errorf("Compressed OVA (%d bytes) should be smaller than uncompressed (%d bytes)",
			compressedInfo.Size(), uncompressedInfo.Size())
	}

	// Verify compressed OVA structure
	if err := verifyOVAStructure(ovaPath, true); err != nil {
		t.Errorf("Compressed OVA structure verification failed: %v", err)
	}
}

func TestCreateOVA_OVFFirst(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create files in "wrong" alphabetical order
	_ = os.WriteFile(filepath.Join(tmpDir, "zzz.vmdk"), []byte("VMDK"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "aaa.mf"), []byte("MF"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "middle.ovf"), []byte("OVF"), 0644)

	// Create OVA
	ovaPath := filepath.Join(tmpDir, "test.ova")
	log := logger.New("debug")

	err := CreateOVA(tmpDir, ovaPath, false, 0, log)
	if err != nil {
		t.Fatalf("CreateOVA failed: %v", err)
	}

	// Verify OVF is first file in TAR
	file, err := os.Open(ovaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	tr := tar.NewReader(file)
	header, err := tr.Next()
	if err != nil {
		t.Fatalf("Failed to read first TAR entry: %v", err)
	}

	if !strings.HasSuffix(header.Name, ".ovf") {
		t.Errorf("First file should be OVF, got: %s", header.Name)
	}
}

func TestExtractOVA_Uncompressed(t *testing.T) {
	// Create a test OVA
	tmpDir := t.TempDir()
	ovfContent := "Test OVF content"
	_ = os.WriteFile(filepath.Join(tmpDir, "test.ovf"), []byte(ovfContent), 0644)

	ovaPath := filepath.Join(tmpDir, "test.ova")
	log := logger.New("debug")
	_ = CreateOVA(tmpDir, ovaPath, false, 0, log)

	// Extract to new directory
	extractDir := filepath.Join(tmpDir, "extracted")
	err := ExtractOVA(ovaPath, extractDir, log)
	if err != nil {
		t.Fatalf("ExtractOVA failed: %v", err)
	}

	// Verify extracted file
	extractedOVF := filepath.Join(extractDir, "test.ovf")
	content, err := os.ReadFile(extractedOVF)
	if err != nil {
		t.Fatalf("Failed to read extracted OVF: %v", err)
	}

	if string(content) != ovfContent {
		t.Errorf("Extracted content mismatch. Expected %q, got %q", ovfContent, string(content))
	}
}

func TestExtractOVA_Compressed(t *testing.T) {
	// Create a test compressed OVA
	tmpDir := t.TempDir()
	ovfContent := "Test OVF content for compression"
	_ = os.WriteFile(filepath.Join(tmpDir, "test.ovf"), []byte(ovfContent), 0644)

	ovaPath := filepath.Join(tmpDir, "test.ova.gz")
	log := logger.New("debug")
	_ = CreateOVA(tmpDir, ovaPath, true, 9, log)

	// Extract to new directory
	extractDir := filepath.Join(tmpDir, "extracted")
	err := ExtractOVA(ovaPath, extractDir, log)
	if err != nil {
		t.Fatalf("ExtractOVA (compressed) failed: %v", err)
	}

	// Verify extracted file
	extractedOVF := filepath.Join(extractDir, "test.ovf")
	content, err := os.ReadFile(extractedOVF)
	if err != nil {
		t.Fatalf("Failed to read extracted OVF: %v", err)
	}

	if string(content) != ovfContent {
		t.Errorf("Extracted content mismatch. Expected %q, got %q", ovfContent, string(content))
	}
}

func TestValidateOVA_Valid(t *testing.T) {
	// Create valid OVA
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.ovf"), []byte("OVF"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "test.vmdk"), []byte("VMDK"), 0644)

	ovaPath := filepath.Join(tmpDir, "test.ova")
	log := logger.New("debug")
	_ = CreateOVA(tmpDir, ovaPath, false, 0, log)

	// Validate
	err := ValidateOVA(ovaPath)
	if err != nil {
		t.Errorf("ValidateOVA failed for valid OVA: %v", err)
	}
}

func TestValidateOVA_NoOVF(t *testing.T) {
	// Create OVA without OVF file
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.vmdk"), []byte("VMDK"), 0644)

	ovaPath := filepath.Join(tmpDir, "test.ova")
	log := logger.New("debug")
	_ = CreateOVA(tmpDir, ovaPath, false, 0, log)

	// Should fail validation
	err := ValidateOVA(ovaPath)
	if err == nil {
		t.Error("ValidateOVA should fail for OVA without OVF file")
	}
}

func TestValidateOVA_NoVMDK(t *testing.T) {
	// Create OVA without VMDK file
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "test.ovf"), []byte("OVF"), 0644)

	ovaPath := filepath.Join(tmpDir, "test.ova")
	log := logger.New("debug")
	_ = CreateOVA(tmpDir, ovaPath, false, 0, log)

	// Should fail validation
	err := ValidateOVA(ovaPath)
	if err == nil {
		t.Error("ValidateOVA should fail for OVA without VMDK file")
	}
}

// Helper function to verify OVA structure
func verifyOVAStructure(ovaPath string, compressed bool) error {
	file, err := os.Open(ovaPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	var tr *tar.Reader

	if compressed {
		// Decompress first
		gzr, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer func() { _ = gzr.Close() }()
		tr = tar.NewReader(gzr)
	} else {
		tr = tar.NewReader(file)
	}

	// Verify first file is OVF
	header, err := tr.Next()
	if err != nil {
		return err
	}

	if !strings.HasSuffix(header.Name, ".ovf") {
		return fmt.Errorf("first file should be .ovf, got %s", header.Name)
	}

	return nil
}

func TestCompressionLevels(t *testing.T) {
	tmpDir := t.TempDir()

	// Create large compressible file
	content := strings.Repeat("ABCD", 10000)
	_ = os.WriteFile(filepath.Join(tmpDir, "test.ovf"), []byte(content), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "test.vmdk"), []byte(content), 0644)

	log := logger.New("debug")

	levels := []int{1, 6, 9}
	sizes := make([]int64, len(levels))

	for i, level := range levels {
		ovaPath := filepath.Join(tmpDir, fmt.Sprintf("test-level%d.ova.gz", level))
		err := CreateOVA(tmpDir, ovaPath, true, level, log)
		if err != nil {
			t.Fatalf("CreateOVA with level %d failed: %v", level, err)
		}

		info, _ := os.Stat(ovaPath)
		sizes[i] = info.Size()
	}

	// Verify that higher compression levels produce smaller files (generally)
	// Level 9 should be <= level 1
	if sizes[2] > sizes[0] {
		t.Logf("Warning: Level 9 (%d bytes) larger than level 1 (%d bytes) - may vary with content",
			sizes[2], sizes[0])
	}

	t.Logf("Compression sizes - Level 1: %d, Level 6: %d, Level 9: %d",
		sizes[0], sizes[1], sizes[2])
}
