// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// OvftoolExporter implements export using VMware ovftool
type OvftoolExporter struct {
	ovftoolPath string
	logger      logger.Logger
}

// NewOvftoolExporter creates a new ovftool exporter
func NewOvftoolExporter(ovftoolPath string, log logger.Logger) *OvftoolExporter {
	return &OvftoolExporter{
		ovftoolPath: ovftoolPath,
		logger:      log,
	}
}

// Export performs VM export using VMware ovftool
func (e *OvftoolExporter) Export(ctx context.Context, job *models.JobDefinition, progressCallback func(*models.JobProgress)) (*models.JobResult, error) {
	// Validate job first
	if err := e.Validate(job); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	e.logger.Info("starting ovftool export", "vm", job.VMPath, "ovftool_path", e.ovftoolPath)

	// Build VI URL: vi://user:pass@server/path/to/vm
	viURL := fmt.Sprintf("vi://%s:%s@%s%s",
		job.VCenter.Username,
		job.VCenter.Password,
		job.VCenter.Server,
		job.VMPath,
	)

	// Build output path
	outputPath := filepath.Join(job.OutputDir, filepath.Base(job.VMPath))

	// Build ovftool command
	args := []string{
		"--acceptAllEulas",
		"--noSSLVerify",
	}

	if job.Compress {
		args = append(args, "--compress=9")
	}

	// Add source and destination
	args = append(args, viURL, outputPath)

	// Create command
	// #nosec G204 -- e.ovftoolPath is operator-configured locally; viURL/outputPath args are
	// validated by Validate() (rejects leading '-' and control characters) before use here
	cmd := exec.CommandContext(ctx, e.ovftoolPath, args...)

	// Get stdout/stderr pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	// Update progress
	if progressCallback != nil {
		progressCallback(&models.JobProgress{
			Phase:        "connecting",
			ExportMethod: string(capabilities.ExportMethodOvftool),
		})
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ovftool: %w", err)
	}

	// Wrap pipes in bufio.Reader for monitoring
	stdout := bufio.NewReader(stdoutPipe)
	stderr := bufio.NewReader(stderrPipe)

	// Monitor progress (ovftool outputs progress to stdout)
	go e.monitorProgress(stdout, progressCallback)
	go e.monitorErrors(stderr)

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ovftool export failed: %w", err)
	}

	// Build result
	result := &models.JobResult{
		Success:      true,
		ExportMethod: string(capabilities.ExportMethodOvftool),
	}

	e.logger.Info("ovftool export completed successfully", "vm", job.VMPath)

	return result, nil
}

// monitorProgress parses ovftool progress output
func (e *OvftoolExporter) monitorProgress(stdout *bufio.Reader, progressCallback func(*models.JobProgress)) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		e.logger.Debug("ovftool output", "line", line)

		// ovftool outputs progress like: "Progress: 45%"
		if strings.Contains(line, "Progress:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				percentStr := strings.TrimSpace(strings.TrimSuffix(parts[1], "%"))
				if percent, err := strconv.ParseFloat(percentStr, 64); err == nil {
					if progressCallback != nil {
						progressCallback(&models.JobProgress{
							Phase:           "exporting",
							PercentComplete: percent,
							CurrentStep:     line,
							ExportMethod:    string(capabilities.ExportMethodOvftool),
						})
					}
				}
			}
		}

		// Also update on other important messages
		if progressCallback != nil && (strings.Contains(line, "Disk") || strings.Contains(line, "Transfer")) {
			progressCallback(&models.JobProgress{
				Phase:        "exporting",
				CurrentStep:  line,
				ExportMethod: string(capabilities.ExportMethodOvftool),
			})
		}
	}
}

// monitorErrors logs stderr
func (e *OvftoolExporter) monitorErrors(stderr *bufio.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		e.logger.Warn("ovftool stderr", "line", line)
	}
}

// Method returns the export method name
func (e *OvftoolExporter) Method() capabilities.ExportMethod {
	return capabilities.ExportMethodOvftool
}

// Validate checks if this exporter can handle the job
func (e *OvftoolExporter) Validate(job *models.JobDefinition) error {
	if job.VMPath == "" {
		return fmt.Errorf("vm_path is required")
	}

	if job.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}

	if job.VCenter == nil || job.VCenter.Server == "" {
		return fmt.Errorf("vcenter server is required")
	}

	if job.VCenter.Username == "" {
		return fmt.Errorf("vcenter username is required")
	}

	if job.VCenter.Password == "" {
		return fmt.Errorf("vcenter password is required")
	}

	// These fields are interpolated into the ovftool command line (either as part of
	// the vi:// URL or as standalone positional arguments). Since job definitions can
	// be submitted through the daemon API, reject values that could be interpreted as
	// extra ovftool flags (leading "-") or that contain control characters.
	fields := map[string]string{
		"vm_path":        job.VMPath,
		"output_dir":     job.OutputDir,
		"vcenter.server": job.VCenter.Server,
		"vcenter.user":   job.VCenter.Username,
	}
	for name, val := range fields {
		if err := validateOvftoolArg(name, val); err != nil {
			return err
		}
	}

	return nil
}

// validateOvftoolArg rejects values that could be interpreted as command-line flags
// or that contain control/newline characters when passed as ovftool arguments.
func validateOvftoolArg(name, val string) error {
	if strings.HasPrefix(val, "-") {
		return fmt.Errorf("%s must not start with '-': %q", name, val)
	}
	for _, r := range val {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains an invalid control character", name)
		}
	}
	return nil
}
