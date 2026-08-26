// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/zyvorai/transiva/logger"
)

const (
	// maxOVAFileSize bounds the size of any single file extracted from an OVA
	// archive, guarding against decompression-bomb style attacks (G110).
	maxOVAFileSize = 2 << 40 // 2 TiB, generous for VM disk images

	// maxOVATotalSize bounds the cumulative size of all files extracted from
	// a single OVA archive.
	maxOVATotalSize = 8 << 40 // 8 TiB
)

// CreateOVA packages an OVF export into a single OVA file
// Set compress to true for gzip compression, compressionLevel 0-9 (0=no compression, 6=default, 9=best)
func CreateOVA(ovfDir string, ovaPath string, compress bool, compressionLevel int, log logger.Logger) error {
	log.Info("Creating OVA package", "ovfDir", ovfDir, "ovaPath", ovaPath, "compress", compress)

	// Find all files to package
	files, err := findExportFiles(ovfDir)
	if err != nil {
		return fmt.Errorf("failed to find export files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no export files found in %s", ovfDir)
	}

	log.Info("Packaging files into OVA", "fileCount", len(files))

	// Create OVA file (TAR archive). ovaPath is supplied by the local operator/caller
	// exporting the VM, not remote input.
	// #nosec G304 -- ovaPath is caller-supplied by the local export workflow, not remote input
	ovaFile, err := os.Create(ovaPath)
	if err != nil {
		return fmt.Errorf("failed to create OVA file: %w", err)
	}
	defer func() { _ = ovaFile.Close() }()

	var tw *tar.Writer
	if compress {
		// Set compression level (default to 6 if invalid)
		if compressionLevel < gzip.NoCompression || compressionLevel > gzip.BestCompression {
			compressionLevel = gzip.DefaultCompression
		}

		log.Info("Enabling gzip compression", "level", compressionLevel)
		gzw, err := gzip.NewWriterLevel(ovaFile, compressionLevel)
		if err != nil {
			return fmt.Errorf("failed to create gzip writer: %w", err)
		}
		gzw.Name = filepath.Base(ovaPath)
		defer func() { _ = gzw.Close() }()
		tw = tar.NewWriter(gzw)
	} else {
		tw = tar.NewWriter(ovaFile)
	}
	defer func() { _ = tw.Close() }()

	// OVF must be first file in OVA (per OVF spec)
	var ovfFile string
	var otherFiles []string

	for _, file := range files {
		if strings.HasSuffix(file, ".ovf") {
			ovfFile = file
		} else {
			otherFiles = append(otherFiles, file)
		}
	}

	if ovfFile == "" {
		return fmt.Errorf("no OVF file found in %s", ovfDir)
	}

	// Add OVF first
	if err := addFileToTar(tw, ovfFile, log); err != nil {
		return fmt.Errorf("failed to add OVF to archive: %w", err)
	}

	// Add other files (manifest, disks, etc.)
	for _, file := range otherFiles {
		if err := addFileToTar(tw, file, log); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", filepath.Base(file), err)
		}
	}

	log.Info("OVA package created successfully", "path", ovaPath)
	return nil
}

// findExportFiles finds all files related to the export
func findExportFiles(dir string) ([]string, error) {
	var files []string

	// Extensions to include in OVA
	validExts := map[string]bool{
		".ovf":  true,
		".vmdk": true,
		".mf":   true, // manifest file
		".cert": true, // certificate (if present)
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if validExts[ext] {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// addFileToTar adds a file to a TAR archive
func addFileToTar(tw *tar.Writer, filePath string, log logger.Logger) error {
	// filePath is enumerated by findExportFiles walking the local export directory, not user input.
	// #nosec G304 -- filePath comes from a local filesystem walk of the export directory, not remote input
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Create tar header
	header := &tar.Header{
		Name:    filepath.Base(filePath),
		Mode:    int64(info.Mode()),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	log.Debug("Adding file to OVA", "file", header.Name, "size", header.Size)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Copy file contents
	_, err = io.Copy(tw, file)
	if err != nil {
		return fmt.Errorf("failed to write file to tar: %w", err)
	}

	return nil
}

// ExtractOVA extracts an OVA file to a directory
func ExtractOVA(ovaPath string, destDir string, log logger.Logger) error {
	log.Info("Extracting OVA", "ovaPath", ovaPath, "destDir", destDir)

	// Create destination directory
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open OVA file. ovaPath is supplied by the local operator/caller, not remote input.
	// #nosec G304 -- ovaPath is caller-supplied by the local extract workflow, not remote input
	ovaFile, err := os.Open(ovaPath)
	if err != nil {
		return fmt.Errorf("failed to open OVA file: %w", err)
	}
	defer func() { _ = ovaFile.Close() }()

	// Detect gzip compression by reading magic bytes
	magic := make([]byte, 2)
	_, err = ovaFile.Read(magic)
	if err != nil {
		return fmt.Errorf("failed to read file header: %w", err)
	}

	// Reset file pointer to beginning
	if _, err := ovaFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	var tr *tar.Reader
	isGzipped := magic[0] == 0x1f && magic[1] == 0x8b

	if isGzipped {
		log.Info("Detected gzip compression")
		gzr, err := gzip.NewReader(ovaFile)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() { _ = gzr.Close() }()
		tr = tar.NewReader(gzr)
	} else {
		tr = tar.NewReader(ovaFile)
	}

	// Extract all files
	var totalExtracted int64
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Construct destination path and guard against path traversal ("zip slip")
		// by verifying the resolved path stays within destDir immediately below.
		// #nosec G305 -- destPath is validated against destDir with filepath.Rel below before any use
		destPath := filepath.Join(destDir, header.Name)
		rel, err := filepath.Rel(destDir, destPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("tar entry %q attempts to escape destination directory", header.Name)
		}

		// Guard against decompression-bomb style attacks by bounding both the
		// per-file and cumulative extracted size.
		if header.Size < 0 || header.Size > maxOVAFileSize {
			return fmt.Errorf("tar entry %q declares an invalid or excessive size (%d bytes)", header.Name, header.Size)
		}
		totalExtracted += header.Size
		if totalExtracted > maxOVATotalSize {
			return fmt.Errorf("OVA extraction exceeds maximum total size of %d bytes", int64(maxOVATotalSize))
		}

		log.Debug("Extracting file from OVA", "file", header.Name, "size", header.Size)

		// Create file. destPath has been validated above to stay within destDir.
		// #nosec G304 -- destPath is validated above to be confined to destDir
		outFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", header.Name, err)
		}

		// Copy contents, bounded by the validated header size
		written, err := io.CopyN(outFile, tr, header.Size)
		closeErr := outFile.Close()
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to extract file %s: %w", header.Name, err)
		}
		if written < header.Size {
			return fmt.Errorf("truncated file %s: wrote %d of %d bytes", header.Name, written, header.Size)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close file %s: %w", header.Name, closeErr)
		}

		// Set file mode. header.Mode is bounds-checked before the int64->uint32(FileMode) conversion.
		if header.Mode < 0 || header.Mode > math.MaxUint32 {
			log.Warn("Skipping invalid file mode", "file", header.Name, "mode", header.Mode)
		} else {
			// #nosec G115 -- header.Mode is bounds-checked above before conversion to os.FileMode
			if err := os.Chmod(destPath, os.FileMode(header.Mode)); err != nil {
				log.Warn("Failed to set file mode", "file", header.Name, "error", err)
			}
		}
	}

	log.Info("OVA extracted successfully", "destDir", destDir)
	return nil
}

// ValidateOVA validates an OVA file structure
func ValidateOVA(ovaPath string) error {
	// ovaPath is supplied by the local operator/caller, not remote input.
	// #nosec G304 -- ovaPath is caller-supplied by the local validation workflow, not remote input
	ovaFile, err := os.Open(ovaPath)
	if err != nil {
		return fmt.Errorf("failed to open OVA file: %w", err)
	}
	defer func() { _ = ovaFile.Close() }()

	tr := tar.NewReader(ovaFile)

	foundOVF := false
	foundVMDK := false
	fileCount := 0

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		fileCount++
		name := strings.ToLower(header.Name)

		if strings.HasSuffix(name, ".ovf") {
			if fileCount != 1 {
				return fmt.Errorf("OVF file must be first file in OVA (found at position %d)", fileCount)
			}
			foundOVF = true
		}

		if strings.HasSuffix(name, ".vmdk") {
			foundVMDK = true
		}
	}

	if !foundOVF {
		return fmt.Errorf("OVA does not contain an OVF file")
	}

	if !foundVMDK {
		return fmt.Errorf("OVA does not contain any VMDK files")
	}

	return nil
}
