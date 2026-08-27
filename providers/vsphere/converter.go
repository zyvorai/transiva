// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/common"
)

// H2KVMConverter handles automatic conversion of exported VMs using h2kvm
type H2KVMConverter struct {
	binaryPath string
	logger     logger.Logger
}

// NewH2KVMConverter creates a new converter instance
func NewH2KVMConverter(binaryPath string, log logger.Logger) (*H2KVMConverter, error) {
	// Auto-detect h2kvm binary if not provided
	if binaryPath == "" {
		detected, err := detectH2KVMBinary()
		if err != nil {
			return nil, fmt.Errorf("h2kvm not found: %w (install with: pip install h2kvm)", err)
		}
		binaryPath = detected
	}

	// Validate binary exists and is executable
	if err := validateBinary(binaryPath); err != nil {
		return nil, fmt.Errorf("invalid h2kvm binary: %w", err)
	}

	log.Info("h2kvm binary detected", "path", binaryPath)

	return &H2KVMConverter{
		binaryPath: binaryPath,
		logger:     log,
	}, nil
}

// Convert runs h2kvm conversion on the manifest
func (c *H2KVMConverter) Convert(ctx context.Context, manifestPath string, opts common.ConvertOptions) (*common.ConversionResult, error) {
	startTime := time.Now()

	c.logger.Info("starting h2kvm conversion", "manifest", manifestPath)

	// Build command
	args := []string{"--manifest", manifestPath}

	if opts.Verbose {
		args = append(args, "--verbose")
	}

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	// Create command
	// #nosec G204 -- binaryPath is validated (validateBinary) and set via local operator config/auto-detection;
	// args are built internally from typed opts fields and the manifest path, not raw remote input.
	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	cmd.Dir = filepath.Dir(manifestPath)

	// Setup output streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start h2kvm: %w", err)
	}

	// Stream output
	outputChan := make(chan string, 100)
	errorChan := make(chan string, 100)

	go streamOutput(stdout, outputChan)
	go streamOutput(stderr, errorChan)

	// Process output
	done := make(chan bool)
	go func() {
		for {
			select {
			case line, ok := <-outputChan:
				if !ok {
					done <- true
					return
				}
				if opts.StreamOutput {
					fmt.Println(line)
				}
				c.logger.Debug("h2kvm output", "line", line)

			case line, ok := <-errorChan:
				if !ok {
					continue
				}
				if opts.StreamOutput {
					fmt.Fprintf(os.Stderr, "%s\n", line)
				}
				c.logger.Warn("h2kvm error", "line", line)
			}
		}
	}()

	// Wait for completion with timeout
	waitChan := make(chan error, 1)
	go func() {
		waitChan <- cmd.Wait()
	}()

	var cmdErr error
	select {
	case <-ctx.Done():
		if killErr := cmd.Process.Kill(); killErr != nil {
			c.logger.Warn("failed to kill h2kvm process after timeout", "error", killErr)
		}
		return nil, fmt.Errorf("conversion timeout: %w", ctx.Err())
	case cmdErr = <-waitChan:
		<-done
	}

	duration := time.Since(startTime)

	// Check for errors
	if cmdErr != nil {
		return &common.ConversionResult{
			Success:  false,
			Duration: duration,
			Error:    cmdErr.Error(),
		}, fmt.Errorf("h2kvm failed: %w", cmdErr)
	}

	// Parse conversion results
	result, err := c.parseConversionResults(filepath.Dir(manifestPath))
	if err != nil {
		c.logger.Warn("failed to parse conversion results", "error", err)
		// Don't fail if we can't parse results - conversion may have succeeded
		result = &common.ConversionResult{
			Success:  true,
			Duration: duration,
		}
	} else {
		result.Duration = duration
	}

	c.logger.Info("h2kvm conversion completed",
		"duration", duration,
		"success", result.Success,
		"converted_files", len(result.ConvertedFiles))

	return result, nil
}

// detectH2KVMBinary attempts to find the h2kvm binary in PATH, falling back
// to the legacy "hyper2kvm" name for hosts that have not upgraded yet.
func detectH2KVMBinary() (string, error) {
	// Try common locations, new name first
	candidates := []string{
		"h2kvmctl",                // In PATH
		"h2kvm",                   // In PATH
		"/usr/local/bin/h2kvmctl", // System install
		"/usr/local/bin/h2kvm",    // System install
		"/usr/bin/h2kvmctl",       // Package manager
		"/usr/bin/h2kvm",          // Package manager
		filepath.Join(os.Getenv("HOME"), ".local/bin/h2kvmctl"), // User install
		filepath.Join(os.Getenv("HOME"), ".local/bin/h2kvm"),    // User install
		// Legacy (pre-rename) locations, kept for backward compatibility
		"hyper2kvm",
		"/usr/local/bin/hyper2kvm",
		"/usr/bin/hyper2kvm",
		filepath.Join(os.Getenv("HOME"), ".local/bin/hyper2kvm"),
	}

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("h2kvm binary not found in PATH or common locations")
}

// validateBinary checks if the binary exists and is executable
func validateBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("binary not found: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a binary")
	}

	// Check if executable (Unix permission bits)
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable")
	}

	return nil
}

// streamOutput reads from pipe and sends to channel
func streamOutput(pipe io.ReadCloser, output chan<- string) {
	defer close(output)

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		output <- scanner.Text()
	}
}

// parseConversionResults reads the conversion report and extracts results
func (c *H2KVMConverter) parseConversionResults(outputDir string) (*common.ConversionResult, error) {
	reportPath := filepath.Join(outputDir, "report.json")

	// Check if report exists
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("report.json not found: %w", err)
	}

	// Read report
	// #nosec G304 -- reportPath is filepath.Join(outputDir, "report.json") where outputDir is the
	// local manifest directory used for this conversion, not attacker-controlled input.
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}

	// Parse report
	var report struct {
		Success  bool `json:"success"`
		Pipeline struct {
			Stages map[string]struct {
				Success bool `json:"success"`
			} `json:"stages"`
		} `json:"pipeline"`
		Artifacts struct {
			ConvertedDisks []struct {
				Path string `json:"path"`
			} `json:"converted_disks"`
		} `json:"artifacts"`
	}

	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}

	// Extract converted files
	var convertedFiles []string
	for _, disk := range report.Artifacts.ConvertedDisks {
		convertedFiles = append(convertedFiles, disk.Path)
	}

	result := &common.ConversionResult{
		Success:        report.Success,
		ConvertedFiles: convertedFiles,
		ReportPath:     reportPath,
	}

	return result, nil
}

// Validate checks if the converter is properly configured
func (c *H2KVMConverter) Validate() error {
	return validateBinary(c.binaryPath)
}

// GetVersion returns the h2kvm version
func (c *H2KVMConverter) GetVersion() (string, error) {
	// #nosec G204 -- binaryPath is validated (validateBinary) and set via local operator config/auto-detection, not remote input.
	cmd := exec.Command(c.binaryPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}
