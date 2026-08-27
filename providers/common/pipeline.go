// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zyvorai/transiva/manifest"
)

// H2KVMConfig contains configuration for the h2kvm pipeline
type H2KVMConfig struct {
	// Enabled controls whether the pipeline runs after export
	Enabled bool

	// H2KVMPath is the path to the h2kvm executable
	// Default: searches PATH for "h2kvmctl" (falls back to legacy "hyper2kvm")
	H2KVMPath string

	// ManifestPath is the path to the manifest file
	// This is set automatically by the export process
	ManifestPath string

	// LibvirtIntegration enables libvirt VM definition after conversion
	LibvirtIntegration bool

	// LibvirtURI is the libvirt connection URI
	// Default: "qemu:///system"
	LibvirtURI string

	// AutoStart enables VM auto-start in libvirt
	AutoStart bool

	// Verbose enables verbose output from h2kvm
	Verbose bool

	// DryRun runs h2kvm in dry-run mode (no modifications)
	DryRun bool

	// UseDaemon uses systemd daemon instead of direct execution
	UseDaemon bool

	// DaemonInstance is the systemd instance name (e.g., "vsphere-prod")
	// Used with h2kvm@{instance}.service
	DaemonInstance string

	// DaemonWatchDir is the watch directory for daemon mode
	// Default: /var/lib/h2kvm/queue
	DaemonWatchDir string

	// DaemonOutputDir is the output directory for daemon mode
	// Default: /var/lib/h2kvm/output
	DaemonOutputDir string

	// DaemonPollInterval is the poll interval in seconds
	// Default: 5
	DaemonPollInterval int

	// DaemonTimeout is the timeout in minutes
	// Default: 60
	DaemonTimeout int
}

// PipelineResult contains the result of pipeline execution
type PipelineResult struct {
	// Success indicates whether the pipeline completed successfully
	Success bool

	// Duration is the total pipeline execution time
	Duration time.Duration

	// OutputPath is the path to the converted disk image
	OutputPath string

	// LibvirtDomain is the libvirt domain name (if libvirt integration enabled)
	LibvirtDomain string

	// Error contains any error that occurred
	Error error

	// Output contains the pipeline output (stdout + stderr)
	Output []string
}

// PipelineExecutor executes the h2kvm pipeline
type PipelineExecutor struct {
	config *H2KVMConfig
	logger Logger
}

// Logger interface for pipeline logging
type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewPipelineExecutor creates a new pipeline executor
func NewPipelineExecutor(config *H2KVMConfig, logger Logger) *PipelineExecutor {
	// Set defaults
	if config.H2KVMPath == "" {
		// Try to find h2kvm in PATH or common locations
		config.H2KVMPath = findH2KVM()
	}
	if config.LibvirtURI == "" {
		config.LibvirtURI = "qemu:///system"
	}

	return &PipelineExecutor{
		config: config,
		logger: logger,
	}
}

// detectDaemonMode checks if the h2kvm daemon is available
func (e *PipelineExecutor) detectDaemonMode() (bool, string) {
	// If daemon mode explicitly disabled, return false
	if !e.config.UseDaemon {
		return false, "direct"
	}

	// Check if specific instance requested
	serviceName := "h2kvm.service"
	if e.config.DaemonInstance != "" {
		serviceName = fmt.Sprintf("h2kvm@%s.service", e.config.DaemonInstance)
	}

	// Check if the current-name systemd service is running
	if cmd := exec.Command("systemctl", "is-active", serviceName); cmd.Run() == nil {
		return true, serviceName
	}

	// Fall back to the legacy (pre-rename) service name for older installs
	legacyServiceName := "hyper2kvm.service"
	if e.config.DaemonInstance != "" {
		legacyServiceName = fmt.Sprintf("hyper2kvm@%s.service", e.config.DaemonInstance)
	}
	if cmd := exec.Command("systemctl", "is-active", legacyServiceName); cmd.Run() == nil {
		return true, legacyServiceName
	}

	// Daemon not available, fall back to direct
	e.logger.Warn("h2kvm daemon not running, using direct execution", "service", serviceName)
	return false, "direct"
}

// Execute runs the h2kvm pipeline
func (e *PipelineExecutor) Execute(ctx context.Context) (*PipelineResult, error) {
	if !e.config.Enabled {
		return &PipelineResult{
			Success: true,
			Output:  []string{"Pipeline disabled, skipping"},
		}, nil
	}

	// Auto-detect daemon mode if configured
	if e.config.UseDaemon {
		isDaemon, serviceName := e.detectDaemonMode()
		if isDaemon {
			e.logger.Info("using h2kvm daemon", "service", serviceName)
			return e.ExecuteViaDaemon(ctx)
		}
	}

	// Use direct execution
	e.logger.Info("using direct h2kvm execution")
	return e.ExecuteDirect(ctx)
}

// ExecuteDirect runs h2kvm directly (original implementation)
func (e *PipelineExecutor) ExecuteDirect(ctx context.Context) (*PipelineResult, error) {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	startTime := time.Now()
	result := &PipelineResult{
		Success: false,
		Output:  []string{},
	}

	e.logger.Info("starting h2kvm pipeline", "manifest", e.config.ManifestPath)

	// Verify h2kvm exists
	if _, err := os.Stat(e.config.H2KVMPath); err != nil {
		result.Error = fmt.Errorf("h2kvm not found at %s: %w", e.config.H2KVMPath, err)
		return result, result.Error
	}

	// Verify manifest exists
	if _, err := os.Stat(e.config.ManifestPath); err != nil {
		result.Error = fmt.Errorf("manifest not found at %s: %w", e.config.ManifestPath, err)
		return result, result.Error
	}

	// Build h2kvm command
	args := []string{e.config.ManifestPath}
	if e.config.Verbose {
		args = append(args, "-v")
	}
	if e.config.DryRun {
		args = append(args, "--dry-run")
	}

	// #nosec G204 -- H2KVMPath and ManifestPath are supplied via local CLI/config (not remote/API input); args are built from validated flags above
	cmd := exec.CommandContext(ctx, e.config.H2KVMPath, args...)

	// Set up output capturing
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = fmt.Errorf("create stdout pipe: %w", err)
		return result, result.Error
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		result.Error = fmt.Errorf("create stderr pipe: %w", err)
		return result, result.Error
	}

	// Start the command
	e.logger.Info("executing h2kvm", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Errorf("start h2kvm: %w", err)
		return result, result.Error
	}

	// Stream output
	outputChan := make(chan string, 100)
	go streamOutput(stdoutPipe, outputChan)
	go streamOutput(stderrPipe, outputChan)

	// Collect output and log it
	go func() {
		for line := range outputChan {
			result.Output = append(result.Output, line)
			e.logger.Info("h2kvm", "output", line)
		}
	}()

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		result.Duration = time.Since(startTime)
		result.Error = fmt.Errorf("h2kvm failed: %w", err)
		e.logger.Error("h2kvm failed", "duration", result.Duration, "error", err)
		return result, result.Error
	}

	result.Duration = time.Since(startTime)
	e.logger.Info("h2kvm completed successfully", "duration", result.Duration)

	// Parse output to find converted disk path
	result.OutputPath = e.findOutputPath(result.Output)
	result.Success = true

	// Run libvirt integration if enabled
	if e.config.LibvirtIntegration && result.OutputPath != "" {
		e.logger.Info("running libvirt integration")
		if err := e.runLibvirtIntegration(ctx, result); err != nil {
			e.logger.Warn("libvirt integration failed (non-fatal)", "error", err)
			// Don't fail the overall pipeline if libvirt integration fails
		}
	}

	return result, nil
}

// ExecuteViaDaemon submits work to the h2kvm daemon and waits for completion
func (e *PipelineExecutor) ExecuteViaDaemon(ctx context.Context) (*PipelineResult, error) {
	startTime := time.Now()
	result := &PipelineResult{
		Success: false,
		Output:  []string{},
	}

	// Set defaults for daemon configuration
	watchDir := e.config.DaemonWatchDir
	if watchDir == "" {
		watchDir = "/var/lib/h2kvm/queue"
	}

	outputDir := e.config.DaemonOutputDir
	if outputDir == "" {
		outputDir = "/var/lib/h2kvm/output"
	}

	pollInterval := e.config.DaemonPollInterval
	if pollInterval == 0 {
		pollInterval = 5 // 5 seconds
	}

	timeout := e.config.DaemonTimeout
	if timeout == 0 {
		timeout = 60 // 60 minutes
	}

	e.logger.Info("submitting to h2kvm daemon",
		"watchDir", watchDir,
		"outputDir", outputDir,
		"pollInterval", pollInterval,
		"timeout", timeout)

	// Verify watch directory exists; fall back to the legacy (pre-rename) path
	// for hosts that still have the daemon installed under its old name.
	if _, err := os.Stat(watchDir); err != nil {
		if e.config.DaemonWatchDir == "" {
			if legacy := "/var/lib/hyper2kvm/queue"; func() bool { _, statErr := os.Stat(legacy); return statErr == nil }() {
				watchDir = legacy
			} else {
				result.Error = fmt.Errorf("daemon watch directory not found: %s: %w", watchDir, err)
				return result, result.Error
			}
		} else {
			result.Error = fmt.Errorf("daemon watch directory not found: %s: %w", watchDir, err)
			return result, result.Error
		}
	}

	// Verify output directory exists; same legacy fallback as above.
	if _, err := os.Stat(outputDir); err != nil {
		if e.config.DaemonOutputDir == "" {
			if legacy := "/var/lib/hyper2kvm/output"; func() bool { _, statErr := os.Stat(legacy); return statErr == nil }() {
				outputDir = legacy
			} else {
				result.Error = fmt.Errorf("daemon output directory not found: %s: %w", outputDir, err)
				return result, result.Error
			}
		} else {
			result.Error = fmt.Errorf("daemon output directory not found: %s: %w", outputDir, err)
			return result, result.Error
		}
	}

	// Load manifest to get VM name
	m, err := manifest.ReadFromFile(e.config.ManifestPath)
	if err != nil {
		result.Error = fmt.Errorf("read manifest: %w", err)
		return result, result.Error
	}

	vmName := m.Source.VMName
	if vmName == "" {
		vmName = "vm"
	}
	// Guard against path traversal via a maliciously crafted VM name in the manifest,
	// since vmName is used below to build filesystem paths under watchDir/outputDir.
	if strings.ContainsAny(vmName, "/\\") || strings.Contains(vmName, "..") {
		result.Error = fmt.Errorf("invalid VM name in manifest: %q", vmName)
		return result, result.Error
	}

	// Copy manifest to watch directory
	manifestDest := filepath.Join(watchDir, filepath.Base(e.config.ManifestPath))
	// Defense in depth: ensure the destination path actually resolves inside watchDir.
	if !strings.HasPrefix(manifestDest, filepath.Clean(watchDir)+string(os.PathSeparator)) {
		result.Error = fmt.Errorf("manifest destination path escapes watch directory: %s", manifestDest)
		return result, result.Error
	}
	e.logger.Info("copying manifest to daemon watch directory", "dest", manifestDest)

	data, err := os.ReadFile(e.config.ManifestPath)
	if err != nil {
		result.Error = fmt.Errorf("read manifest: %w", err)
		return result, result.Error
	}

	// #nosec G703 -- manifestDest is validated above to resolve inside watchDir, preventing path traversal despite the static taint warning
	if err := os.WriteFile(manifestDest, data, 0600); err != nil {
		result.Error = fmt.Errorf("write manifest to watch directory: %w", err)
		return result, result.Error
	}

	// Wait for daemon to process
	e.logger.Info("waiting for daemon to process VM", "vmName", vmName, "timeout", fmt.Sprintf("%dm", timeout))

	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(time.Duration(timeout) * time.Minute)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			result.Error = fmt.Errorf("context cancelled")
			return result, result.Error

		case <-timeoutTimer.C:
			result.Duration = time.Since(startTime)
			result.Error = fmt.Errorf("daemon processing timeout after %dm", timeout)
			e.logger.Error("daemon timeout", "duration", result.Duration)
			return result, result.Error

		case <-ticker.C:
			// Check if output file exists
			expectedOutput := filepath.Join(outputDir, vmName+".qcow2")
			if _, err := os.Stat(expectedOutput); err == nil {
				e.logger.Info("daemon processing complete", "output", expectedOutput)
				result.Success = true
				result.OutputPath = expectedOutput
				result.Duration = time.Since(startTime)
				result.Output = []string{fmt.Sprintf("Daemon processed VM in %s", result.Duration)}

				// Run libvirt integration if enabled
				if e.config.LibvirtIntegration && result.OutputPath != "" {
					e.logger.Info("running libvirt integration")
					if err := e.runLibvirtIntegration(ctx, result); err != nil {
						e.logger.Warn("libvirt integration failed (non-fatal)", "error", err)
					}
				}

				return result, nil
			}

			// Check for error files
			errorFile := filepath.Join(outputDir, vmName+".error")
			// #nosec G304 -- errorFile is under fixed outputDir with vmName sanitized against path traversal above
			if data, err := os.ReadFile(errorFile); err == nil {
				result.Duration = time.Since(startTime)
				result.Error = fmt.Errorf("daemon processing failed: %s", string(data))
				e.logger.Error("daemon failed", "error", result.Error)
				return result, result.Error
			}

			e.logger.Debug("waiting for daemon to complete processing")
		}
	}
}

// findOutputPath parses h2kvm output to find the converted disk path
func (e *PipelineExecutor) findOutputPath(output []string) string {
	// Look for output path in h2kvm output
	// Expected format: "Output: /path/to/converted.qcow2"
	for _, line := range output {
		if strings.HasPrefix(line, "Output:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
		// Also check for "Wrote: /path/to/file"
		if strings.HasPrefix(line, "Wrote:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	// Fallback: Load manifest and construct expected output path
	m, err := manifest.ReadFromFile(e.config.ManifestPath)
	if err != nil {
		return ""
	}

	if m.Output != nil && m.Output.Directory != "" && m.Output.Filename != "" {
		return filepath.Join(m.Output.Directory, m.Output.Filename)
	}

	return ""
}

// runLibvirtIntegration integrates the converted VM with libvirt
func (e *PipelineExecutor) runLibvirtIntegration(ctx context.Context, result *PipelineResult) error {
	// Load manifest to get VM metadata
	m, err := manifest.ReadFromFile(e.config.ManifestPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	// Create libvirt integrator
	integrator := NewLibvirtIntegrator(&LibvirtConfig{
		URI:       e.config.LibvirtURI,
		AutoStart: e.config.AutoStart,
	}, e.logger)

	// Define VM in libvirt
	domainName, err := integrator.DefineVM(ctx, m, result.OutputPath)
	if err != nil {
		return fmt.Errorf("define VM in libvirt: %w", err)
	}

	result.LibvirtDomain = domainName
	e.logger.Info("VM defined in libvirt", "domain", domainName, "uri", e.config.LibvirtURI)

	return nil
}

// streamOutput reads from a pipe and sends lines to a channel
func streamOutput(reader io.Reader, output chan<- string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		output <- scanner.Text()
	}
}

// findH2KVM searches for the h2kvm executable, falling back to the legacy
// "hyper2kvm" name for hosts that have not upgraded the CLI yet.
func findH2KVM() string {
	// Common locations to check, new name first
	locations := []string{
		"/home/tt/h2kvm/h2kvmctl",
		"/home/tt/h2kvm",
		"/usr/local/bin/h2kvmctl",
		"/usr/local/bin/h2kvm",
		"/usr/bin/h2kvmctl",
		"/usr/bin/h2kvm",
		"./h2kvmctl",
		"./h2kvm",
		"../h2kvm/h2kvmctl",
		// Legacy (pre-rename) locations, kept for backward compatibility
		"/home/tt/hyper2kvm/hyper2kvm",
		"/home/tt/hyper2kvm",
		"/usr/local/bin/hyper2kvm",
		"/usr/bin/hyper2kvm",
		"./hyper2kvm",
		"../hyper2kvm/hyper2kvm",
	}

	// Check common locations
	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try PATH, new name first, then legacy name
	if path, err := exec.LookPath("h2kvmctl"); err == nil {
		return path
	}
	if path, err := exec.LookPath("h2kvm"); err == nil {
		return path
	}
	if path, err := exec.LookPath("hyper2kvm"); err == nil {
		return path
	}

	// Default fallback
	return "/home/tt/h2kvm/h2kvmctl"
}
