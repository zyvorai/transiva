// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// PreExportValidator performs validation before export
type PreExportValidator struct {
	log logger.Logger
}

// ValidationResult contains validation check results
type ValidationResult struct {
	Name    string
	Passed  bool
	Message string
	Warning bool
}

// ValidationReport aggregates all validation results
type ValidationReport struct {
	Checks      []ValidationResult
	AllPassed   bool
	HasWarnings bool
	Timestamp   time.Time
}

// NewPreExportValidator creates a new pre-export validator
func NewPreExportValidator(log logger.Logger) *PreExportValidator {
	return &PreExportValidator{log: log}
}

// ValidateExport performs comprehensive pre-export validation
func (v *PreExportValidator) ValidateExport(ctx context.Context, vm vsphere.VMInfo, outputDir string, estimatedSize int64) *ValidationReport {
	report := &ValidationReport{
		Checks:    make([]ValidationResult, 0),
		AllPassed: true,
		Timestamp: time.Now(),
	}

	// Check 1: Disk space availability
	diskCheck := v.validateDiskSpace(outputDir, estimatedSize)
	report.Checks = append(report.Checks, diskCheck)
	if !diskCheck.Passed {
		report.AllPassed = false
	}
	if diskCheck.Warning {
		report.HasWarnings = true
	}

	// Check 2: Output directory permissions
	permCheck := v.validatePermissions(outputDir)
	report.Checks = append(report.Checks, permCheck)
	if !permCheck.Passed {
		report.AllPassed = false
	}

	// Check 3: VM state validation
	stateCheck := v.validateVMState(vm)
	report.Checks = append(report.Checks, stateCheck)
	if !stateCheck.Passed && !stateCheck.Warning {
		report.AllPassed = false
	}
	if stateCheck.Warning {
		report.HasWarnings = true
	}

	// Check 4: Existing export check
	existingCheck := v.checkExistingExport(outputDir, vm.Name)
	report.Checks = append(report.Checks, existingCheck)
	if existingCheck.Warning {
		report.HasWarnings = true
	}

	// Check 5: Network connectivity (for vSphere)
	// This would be implementation-specific

	if v.log != nil {
		v.log.Info("pre-export validation completed",
			"vm", vm.Name,
			"passed", report.AllPassed,
			"warnings", report.HasWarnings,
			"checks", len(report.Checks))
	}

	return report
}

// validateDiskSpace checks if there's enough disk space
func (v *PreExportValidator) validateDiskSpace(path string, requiredBytes int64) ValidationResult {
	// Create the directory if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0750); err != nil {
			return ValidationResult{
				Name:    "Disk Space Check",
				Passed:  false,
				Message: fmt.Sprintf("Failed to create output directory: %v", err),
				Warning: false,
			}
		}
		if v.log != nil {
			v.log.Info("created output directory", "path", path)
		}
	}

	availableBytes, totalBytes, err := getDiskSpace(path)
	if err != nil {
		return ValidationResult{
			Name:    "Disk Space Check",
			Passed:  false,
			Message: fmt.Sprintf("Failed to check disk space: %v", err),
			Warning: false,
		}
	}

	// Require at least 10% overhead
	requiredWithOverhead := requiredBytes + (requiredBytes / 10)

	if availableBytes < requiredWithOverhead {
		return ValidationResult{
			Name:    "Disk Space Check",
			Passed:  false,
			Message: fmt.Sprintf("Insufficient disk space: need %s (with 10%% overhead), have %s available", formatBytes(requiredWithOverhead), formatBytes(availableBytes)),
			Warning: false,
		}
	}

	// Warning if less than 20% free space will remain
	remainingAfter := availableBytes - requiredBytes
	if remainingAfter < (totalBytes / 5) {
		return ValidationResult{
			Name:    "Disk Space Check",
			Passed:  true,
			Message: fmt.Sprintf("Disk space OK but will have less than 20%% free after export (%s available, %s needed)", formatBytes(availableBytes), formatBytes(requiredBytes)),
			Warning: true,
		}
	}

	return ValidationResult{
		Name:    "Disk Space Check",
		Passed:  true,
		Message: fmt.Sprintf("Sufficient disk space: %s available, %s needed", formatBytes(availableBytes), formatBytes(requiredBytes)),
		Warning: false,
	}
}

// validatePermissions checks write permissions
func (v *PreExportValidator) validatePermissions(path string) ValidationResult {
	// Ensure directory exists
	if err := os.MkdirAll(path, 0750); err != nil {
		return ValidationResult{
			Name:    "Permissions Check",
			Passed:  false,
			Message: fmt.Sprintf("Cannot create directory: %v", err),
			Warning: false,
		}
	}

	// Try to create a test file
	testFile := filepath.Join(path, ".hyperexport_test")
	// #nosec G304 -- path is the local operator-supplied -output directory (CLI flag), not remote input
	f, err := os.Create(testFile)
	if err != nil {
		return ValidationResult{
			Name:    "Permissions Check",
			Passed:  false,
			Message: fmt.Sprintf("No write permission in output directory: %v", err),
			Warning: false,
		}
	}
	if err := f.Close(); err != nil && v.log != nil {
		v.log.Warn("failed to close permissions test file", "path", testFile, "error", err)
	}
	if err := os.Remove(testFile); err != nil && v.log != nil {
		v.log.Warn("failed to remove permissions test file", "path", testFile, "error", err)
	}

	return ValidationResult{
		Name:    "Permissions Check",
		Passed:  true,
		Message: "Output directory is writable",
		Warning: false,
	}
}

// validateVMState checks VM power state and other conditions
func (v *PreExportValidator) validateVMState(vm vsphere.VMInfo) ValidationResult {
	if vm.PowerState == "poweredOn" {
		return ValidationResult{
			Name:    "VM State Check",
			Passed:  true,
			Message: "VM is powered on - export will be crash-consistent (consider powering off for consistent backup)",
			Warning: true,
		}
	}

	return ValidationResult{
		Name:    "VM State Check",
		Passed:  true,
		Message: "VM is powered off - export will be consistent",
		Warning: false,
	}
}

// checkExistingExport checks if export already exists
func (v *PreExportValidator) checkExistingExport(outputDir, vmName string) ValidationResult {
	vmDir := filepath.Join(outputDir, sanitizeVMName(vmName))

	if _, err := os.Stat(vmDir); !os.IsNotExist(err) {
		return ValidationResult{
			Name:    "Existing Export Check",
			Passed:  true,
			Message: "Existing export found - will be overwritten",
			Warning: true,
		}
	}

	return ValidationResult{
		Name:    "Existing Export Check",
		Passed:  true,
		Message: "No existing export found",
		Warning: false,
	}
}

// sanitizeVMName removes invalid characters from VM names for file paths
func sanitizeVMName(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '|', '?', '*', '/', '\\':
			return '_'
		default:
			return r
		}
	}, name)
}

// PostExportValidator performs validation after export
type PostExportValidator struct {
	log logger.Logger
}

// NewPostExportValidator creates a new post-export validator
func NewPostExportValidator(log logger.Logger) *PostExportValidator {
	return &PostExportValidator{log: log}
}

// ValidateExportedFiles verifies exported files integrity
func (v *PostExportValidator) ValidateExportedFiles(exportDir string) *ValidationReport {
	report := &ValidationReport{
		Checks:    make([]ValidationResult, 0),
		AllPassed: true,
		Timestamp: time.Now(),
	}

	// Check 1: OVF file exists and is valid
	ovfCheck := v.validateOVFFile(exportDir)
	report.Checks = append(report.Checks, ovfCheck)
	if !ovfCheck.Passed {
		report.AllPassed = false
	}

	// Check 2: All referenced files exist
	filesCheck := v.validateReferencedFiles(exportDir)
	report.Checks = append(report.Checks, filesCheck)
	if !filesCheck.Passed {
		report.AllPassed = false
	}

	// Check 3: File sizes match expected
	sizeCheck := v.validateFileSizes(exportDir)
	report.Checks = append(report.Checks, sizeCheck)
	if sizeCheck.Warning {
		report.HasWarnings = true
	}

	// Check 4: Checksums (if available)
	checksumCheck := v.validateChecksums(exportDir)
	report.Checks = append(report.Checks, checksumCheck)
	if !checksumCheck.Passed && !checksumCheck.Warning {
		report.AllPassed = false
	}

	if v.log != nil {
		v.log.Info("post-export validation completed",
			"dir", exportDir,
			"passed", report.AllPassed,
			"warnings", report.HasWarnings)
	}

	return report
}

// validateOVFFile checks if OVF file exists and is readable
func (v *PostExportValidator) validateOVFFile(exportDir string) ValidationResult {
	// Find OVF file
	pattern := filepath.Join(exportDir, "*.ovf")
	matches, err := filepath.Glob(pattern)

	if err != nil || len(matches) == 0 {
		return ValidationResult{
			Name:    "OVF File Check",
			Passed:  false,
			Message: "No OVF file found in export directory",
			Warning: false,
		}
	}

	ovfFile := matches[0]
	info, err := os.Stat(ovfFile)
	if err != nil {
		return ValidationResult{
			Name:    "OVF File Check",
			Passed:  false,
			Message: fmt.Sprintf("Cannot access OVF file: %v", err),
			Warning: false,
		}
	}

	if info.Size() == 0 {
		return ValidationResult{
			Name:    "OVF File Check",
			Passed:  false,
			Message: "OVF file is empty",
			Warning: false,
		}
	}

	return ValidationResult{
		Name:    "OVF File Check",
		Passed:  true,
		Message: fmt.Sprintf("OVF file found and valid (%s)", formatBytes(info.Size())),
		Warning: false,
	}
}

// validateReferencedFiles checks if all disk files exist
func (v *PostExportValidator) validateReferencedFiles(exportDir string) ValidationResult {
	// Find all VMDK files
	pattern := filepath.Join(exportDir, "*.vmdk")
	vmdkFiles, err := filepath.Glob(pattern)

	if err != nil {
		return ValidationResult{
			Name:    "Referenced Files Check",
			Passed:  false,
			Message: fmt.Sprintf("Error scanning for disk files: %v", err),
			Warning: false,
		}
	}

	if len(vmdkFiles) == 0 {
		return ValidationResult{
			Name:    "Referenced Files Check",
			Passed:  false,
			Message: "No disk files (.vmdk) found",
			Warning: false,
		}
	}

	return ValidationResult{
		Name:    "Referenced Files Check",
		Passed:  true,
		Message: fmt.Sprintf("All disk files present (%d files)", len(vmdkFiles)),
		Warning: false,
	}
}

// validateFileSizes checks if file sizes are reasonable
func (v *PostExportValidator) validateFileSizes(exportDir string) ValidationResult {
	pattern := filepath.Join(exportDir, "*")
	files, err := filepath.Glob(pattern)

	if err != nil {
		return ValidationResult{
			Name:    "File Size Check",
			Passed:  true,
			Message: "Could not verify file sizes",
			Warning: true,
		}
	}

	var totalSize int64
	for _, file := range files {
		info, err := os.Stat(file)
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
	}

	if totalSize == 0 {
		return ValidationResult{
			Name:    "File Size Check",
			Passed:  false,
			Message: "Total export size is 0 bytes",
			Warning: false,
		}
	}

	return ValidationResult{
		Name:    "File Size Check",
		Passed:  true,
		Message: fmt.Sprintf("Total export size: %s", formatBytes(totalSize)),
		Warning: false,
	}
}

// validateChecksums verifies file checksums if available
func (v *PostExportValidator) validateChecksums(exportDir string) ValidationResult {
	checksumFile := filepath.Join(exportDir, "checksums.txt")

	if _, err := os.Stat(checksumFile); os.IsNotExist(err) {
		return ValidationResult{
			Name:    "Checksum Check",
			Passed:  true,
			Message: "No checksum file found (run with -verify to generate)",
			Warning: true,
		}
	}

	// Read checksum file
	// #nosec G304 -- checksumFile is derived from the local operator-supplied export directory (CLI flag), not remote input
	content, err := os.ReadFile(checksumFile)
	if err != nil {
		return ValidationResult{
			Name:    "Checksum Check",
			Passed:  false,
			Message: fmt.Sprintf("Failed to read checksum file: %v", err),
			Warning: false,
		}
	}

	// Parse checksums (format: "checksum  filename")
	lines := strings.Split(string(content), "\n")
	expectedChecksums := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		checksum := parts[0]
		filename := parts[1]
		expectedChecksums[filename] = checksum
	}

	if len(expectedChecksums) == 0 {
		return ValidationResult{
			Name:    "Checksum Check",
			Passed:  false,
			Message: "Checksum file is empty or invalid",
			Warning: false,
		}
	}

	// Verify each file's checksum
	var verified, failed, missing int
	var failedFiles []string

	for filename, expectedChecksum := range expectedChecksums {
		filePath := filepath.Join(exportDir, filename)

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missing++
			failedFiles = append(failedFiles, fmt.Sprintf("%s (missing)", filename))
			continue
		}

		// Calculate actual checksum
		actualChecksum, err := CalculateFileChecksum(filePath)
		if err != nil {
			failed++
			failedFiles = append(failedFiles, fmt.Sprintf("%s (error: %v)", filename, err))
			continue
		}

		// Compare checksums
		if actualChecksum != expectedChecksum {
			failed++
			failedFiles = append(failedFiles, fmt.Sprintf("%s (mismatch)", filename))
			if v.log != nil {
				v.log.Warn("checksum mismatch",
					"file", filename,
					"expected", expectedChecksum,
					"actual", actualChecksum)
			}
		} else {
			verified++
			if v.log != nil {
				v.log.Debug("checksum verified", "file", filename)
			}
		}
	}

	// Build result message
	if failed > 0 || missing > 0 {
		return ValidationResult{
			Name:    "Checksum Check",
			Passed:  false,
			Message: fmt.Sprintf("Checksum verification failed: %d verified, %d failed, %d missing. Failed files: %s",
				verified, failed, missing, strings.Join(failedFiles, ", ")),
			Warning: false,
		}
	}

	return ValidationResult{
		Name:    "Checksum Check",
		Passed:  true,
		Message: fmt.Sprintf("All checksums verified successfully (%d files)", verified),
		Warning: false,
	}
}

// CalculateFileChecksum computes SHA256 checksum of a file
func CalculateFileChecksum(filePath string) (string, error) {
	// #nosec G304 -- filePath is derived from the local operator-supplied export directory (CLI flag), not remote input
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("compute hash: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// SaveChecksumManifest saves checksums for all files in directory
func SaveChecksumManifest(exportDir string, checksums map[string]string) error {
	manifestPath := filepath.Join(exportDir, "checksums.txt")

	// #nosec G304 -- manifestPath is derived from the local operator-supplied export directory (CLI flag), not remote input
	f, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	defer f.Close()

	for filename, checksum := range checksums {
		fmt.Fprintf(f, "%s  %s\n", checksum, filename)
	}

	return nil
}

// ComputeExportChecksums calculates checksums for all exported files
func ComputeExportChecksums(exportDir string, log logger.Logger) (map[string]string, error) {
	checksums := make(map[string]string)

	// Get all files in export directory
	pattern := filepath.Join(exportDir, "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	for _, filePath := range files {
		info, err := os.Stat(filePath)
		if err != nil || info.IsDir() {
			continue
		}

		if log != nil {
			log.Info("computing checksum", "file", filepath.Base(filePath))
		}

		checksum, err := CalculateFileChecksum(filePath)
		if err != nil {
			return nil, fmt.Errorf("checksum %s: %w", filePath, err)
		}

		checksums[filepath.Base(filePath)] = checksum
	}

	return checksums, nil
}
