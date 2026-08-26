// SPDX-License-Identifier: Apache-2.0

package hyperv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/progress"
)

// Config holds Hyper-V provider configuration
type Config struct {
	Host      string // Hyper-V host (empty for local)
	Username  string // Windows username for WinRM
	Password  string // Windows password for WinRM
	UseWinRM  bool   // Use WinRM for remote connections
	WinRMPort int    // WinRM port (default 5985 for HTTP, 5986 for HTTPS)
	UseHTTPS  bool   // Use HTTPS for WinRM
	Timeout   time.Duration
}

// Client represents a Hyper-V client for VM operations
type Client struct {
	config *Config
	logger logger.Logger
}

// VMInfo represents Hyper-V VM information
type VMInfo struct {
	Name              string   `json:"Name"`
	ID                string   `json:"Id"`
	State             string   `json:"State"`
	CPUUsage          int      `json:"CPUUsage"`
	MemoryAssigned    int64    `json:"MemoryAssigned"`
	MemoryDemand      int64    `json:"MemoryDemand"`
	Uptime            string   `json:"Uptime"`
	Status            string   `json:"Status"`
	Generation        int      `json:"Generation"`
	Version           string   `json:"Version"`
	Path              string   `json:"Path"`
	ConfigurationPath string   `json:"ConfigurationLocation"`
	VHDPath           []string `json:"-"` // Populated separately
	Notes             string   `json:"Notes"`
	CreationTime      string   `json:"CreationTime"`
}

// NewClient creates a new Hyper-V client
func NewClient(cfg *Config, log logger.Logger) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 1 * time.Hour
	}

	if cfg.WinRMPort == 0 {
		if cfg.UseHTTPS {
			cfg.WinRMPort = 5986
		} else {
			cfg.WinRMPort = 5985
		}
	}

	client := &Client{
		config: cfg,
		logger: log,
	}

	// Validate connection if remote
	if cfg.UseWinRM {
		if err := client.validateConnection(); err != nil {
			return nil, fmt.Errorf("failed to validate Hyper-V connection: %w", err)
		}
	}

	return client, nil
}

// ListVMs returns a list of all VMs on the Hyper-V host
func (c *Client) ListVMs(ctx context.Context) ([]*VMInfo, error) {
	c.logger.Info("listing Hyper-V VMs")

	// PowerShell script to get VM information
	script := `Get-VM | Select-Object Name, Id, State, CPUUsage, MemoryAssigned, MemoryDemand, Uptime, Status, Generation, Version, Path, ConfigurationLocation, Notes, CreationTime | ConvertTo-Json -Depth 3`

	output, err := c.executePowerShell(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	// Parse JSON output
	var vms []*VMInfo

	// Handle both single VM (object) and multiple VMs (array)
	output = strings.TrimSpace(output)
	if strings.HasPrefix(output, "[") {
		// Multiple VMs
		if err := json.Unmarshal([]byte(output), &vms); err != nil {
			return nil, fmt.Errorf("failed to parse VM list: %w", err)
		}
	} else if strings.HasPrefix(output, "{") {
		// Single VM
		var vm VMInfo
		if err := json.Unmarshal([]byte(output), &vm); err != nil {
			return nil, fmt.Errorf("failed to parse VM: %w", err)
		}
		vms = []*VMInfo{&vm}
	} else {
		// No VMs or empty output
		return []*VMInfo{}, nil
	}

	// Get VHD paths for each VM
	for _, vm := range vms {
		vhdPaths, err := c.getVMVHDPaths(ctx, vm.Name)
		if err != nil {
			c.logger.Warn("failed to get VHD paths", "vm", vm.Name, "error", err)
		} else {
			vm.VHDPath = vhdPaths
		}
	}

	c.logger.Info("discovered Hyper-V VMs", "count", len(vms))
	return vms, nil
}

// GetVM retrieves information about a specific VM
func (c *Client) GetVM(ctx context.Context, vmName string) (*VMInfo, error) {
	c.logger.Info("getting Hyper-V VM", "vm", vmName)

	script := fmt.Sprintf(`Get-VM -Name '%s' | Select-Object Name, Id, State, CPUUsage, MemoryAssigned, MemoryDemand, Uptime, Status, Generation, Version, Path, ConfigurationLocation, Notes, CreationTime | ConvertTo-Json -Depth 3`, vmName)

	output, err := c.executePowerShell(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM: %w", err)
	}

	var vm VMInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &vm); err != nil {
		return nil, fmt.Errorf("failed to parse VM info: %w", err)
	}

	// Get VHD paths
	vhdPaths, err := c.getVMVHDPaths(ctx, vmName)
	if err != nil {
		c.logger.Warn("failed to get VHD paths", "vm", vmName, "error", err)
	} else {
		vm.VHDPath = vhdPaths
	}

	return &vm, nil
}

// getVMVHDPaths retrieves all VHD paths for a VM
func (c *Client) getVMVHDPaths(ctx context.Context, vmName string) ([]string, error) {
	script := fmt.Sprintf(`Get-VM -Name '%s' | Get-VMHardDiskDrive | Select-Object -ExpandProperty Path`, vmName)

	output, err := c.executePowerShell(ctx, script)
	if err != nil {
		return nil, err
	}

	paths := strings.Split(strings.TrimSpace(output), "\n")
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			result = append(result, path)
		}
	}

	return result, nil
}

// ExportVM exports a Hyper-V VM
func (c *Client) ExportVM(ctx context.Context, vmName, outputPath string, reporter progress.ProgressReporter) error {
	c.logger.Info("starting Hyper-V VM export", "vm", vmName, "output", outputPath)

	if reporter != nil {
		reporter.Describe("Exporting Hyper-V VM")
	}

	// Get VM info
	vmInfo, err := c.GetVM(ctx, vmName)
	if err != nil {
		return fmt.Errorf("get VM info: %w", err)
	}

	// Create output directory
	exportDir := filepath.Join(outputPath, vmName)

	// Export VM using PowerShell
	script := fmt.Sprintf(`Export-VM -Name '%s' -Path '%s' -ErrorAction Stop`, vmName, outputPath)

	c.logger.Info("executing VM export", "vm", vmName)

	output, err := c.executePowerShell(ctx, script)
	if err != nil {
		return fmt.Errorf("export VM failed: %w (output: %s)", err, output)
	}

	c.logger.Info("VM export completed", "vm", vmName, "output", exportDir)

	// Save metadata
	metadataPath := filepath.Join(outputPath, fmt.Sprintf("%s-metadata.json", vmName))
	if err := c.saveMetadata(vmInfo, metadataPath); err != nil {
		c.logger.Warn("failed to save metadata", "error", err)
	}

	if reporter != nil {
		reporter.Describe("Export complete")
		reporter.Update(100)
	}

	return nil
}

// ExportVHD exports VM VHD files to a specified directory
func (c *Client) ExportVHD(ctx context.Context, vmName, outputPath string, reporter progress.ProgressReporter) ([]string, error) {
	c.logger.Info("exporting Hyper-V VM VHDs", "vm", vmName)

	// Get VM VHD paths
	vhdPaths, err := c.getVMVHDPaths(ctx, vmName)
	if err != nil {
		return nil, fmt.Errorf("get VHD paths: %w", err)
	}

	if len(vhdPaths) == 0 {
		return nil, fmt.Errorf("VM has no VHD files")
	}

	exportedPaths := make([]string, 0, len(vhdPaths))

	for i, vhdPath := range vhdPaths {
		if reporter != nil {
			reporter.Describe(fmt.Sprintf("Copying VHD %d/%d", i+1, len(vhdPaths)))
		}

		vhdName := filepath.Base(vhdPath)
		destPath := filepath.Join(outputPath, vhdName)

		// Copy VHD file
		script := fmt.Sprintf(`Copy-Item -Path '%s' -Destination '%s' -Force`, vhdPath, destPath)

		c.logger.Info("copying VHD", "source", vhdPath, "dest", destPath)

		_, err := c.executePowerShell(ctx, script)
		if err != nil {
			c.logger.Error("failed to copy VHD", "vhd", vhdPath, "error", err)
			continue
		}

		exportedPaths = append(exportedPaths, destPath)
		c.logger.Info("VHD copied", "path", destPath)
	}

	if len(exportedPaths) == 0 {
		return nil, fmt.Errorf("failed to export any VHDs")
	}

	if reporter != nil {
		reporter.Update(100)
	}

	return exportedPaths, nil
}

// ExportVMWithOptions exports a Hyper-V VM using ExportOptions
func (c *Client) ExportVMWithOptions(ctx context.Context, vmName string, opts ExportOptions) error {
	c.logger.Info("starting Hyper-V VM export with options", "vm", vmName)

	// Validate options
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid export options: %w", err)
	}

	// Get VM info
	vmInfo, err := c.GetVM(ctx, vmName)
	if err != nil {
		return fmt.Errorf("get VM info: %w", err)
	}

	// Create output directory
	exportDir := filepath.Join(opts.OutputPath, vmName)

	// Export based on type
	switch opts.ExportType {
	case "vm":
		// Full VM export
		script := fmt.Sprintf(`Export-VM -Name '%s' -Path '%s' -ErrorAction Stop`, vmName, opts.OutputPath)
		c.logger.Info("executing VM export", "vm", vmName)

		output, err := c.executePowerShell(ctx, script)
		if err != nil {
			return fmt.Errorf("export VM failed: %w (output: %s)", err, output)
		}

		c.logger.Info("VM export completed", "vm", vmName, "output", exportDir)

		// Save metadata
		metadataPath := filepath.Join(opts.OutputPath, fmt.Sprintf("%s-metadata.json", vmName))
		if err := c.saveMetadata(vmInfo, metadataPath); err != nil {
			c.logger.Warn("failed to save metadata", "error", err)
		}

		// Notify completion via callback if provided
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(100, 100, vmName, 1, 1)
		}

	case "vhd-only":
		// VHD-only export
		vhdPaths, err := c.exportVHDWithOptions(ctx, vmName, opts)
		if err != nil {
			return fmt.Errorf("export VHD failed: %w", err)
		}

		c.logger.Info("VHD export completed", "vm", vmName, "vhd_count", len(vhdPaths))
	}

	return nil
}

// exportVHDWithOptions exports VHD files with progress callback support
func (c *Client) exportVHDWithOptions(ctx context.Context, vmName string, opts ExportOptions) ([]string, error) {
	c.logger.Info("exporting Hyper-V VM VHDs with options", "vm", vmName)

	// Get VM VHD paths
	vhdPaths, err := c.getVMVHDPaths(ctx, vmName)
	if err != nil {
		return nil, fmt.Errorf("get VHD paths: %w", err)
	}

	if len(vhdPaths) == 0 {
		return nil, fmt.Errorf("VM has no VHD files")
	}

	exportedPaths := make([]string, 0, len(vhdPaths))

	for i, vhdPath := range vhdPaths {
		vhdName := filepath.Base(vhdPath)
		destPath := filepath.Join(opts.OutputPath, vhdName)

		c.logger.Info("copying VHD", "source", vhdPath, "dest", destPath)

		// Copy VHD file with progress tracking
		if opts.ProgressCallback != nil {
			// Copy with progress callback
			err = c.copyFileWithProgress(ctx, vhdPath, destPath, opts.ProgressCallback, i+1, len(vhdPaths))
		} else {
			// Simple copy
			script := fmt.Sprintf(`Copy-Item -Path '%s' -Destination '%s' -Force`, vhdPath, destPath)
			_, err = c.executePowerShell(ctx, script)
		}

		if err != nil {
			c.logger.Error("failed to copy VHD", "vhd", vhdPath, "error", err)
			continue
		}

		exportedPaths = append(exportedPaths, destPath)
		c.logger.Info("VHD copied", "path", destPath)
	}

	if len(exportedPaths) == 0 {
		return nil, fmt.Errorf("failed to export any VHDs")
	}

	return exportedPaths, nil
}

// copyFileWithProgress copies a file with progress callback
func (c *Client) copyFileWithProgress(ctx context.Context, src, dst string, callback func(current, total int64, fileName string, fileIndex, totalFiles int), fileIndex, totalFiles int) error {
	// Get source file size
	// #nosec G304 -- src is a VHD path discovered from the local Hyper-V host being exported, not remote/API input
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			c.logger.Warn("failed to close source file", "error", closeErr)
		}
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	totalSize := srcInfo.Size()

	// Create destination file
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	// #nosec G304 -- dst is derived from the local operator-supplied output directory (opts.OutputPath), not remote/API input
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil {
			c.logger.Warn("failed to close destination file", "error", closeErr)
		}
	}()

	// Copy with progress tracking
	var currentBytes int64
	var reader io.Reader = &callbackProgressReader{
		reader:       srcFile,
		total:        totalSize,
		currentBytes: &currentBytes,
		callback:     callback,
		fileName:     filepath.Base(src),
		fileIndex:    fileIndex,
		totalFiles:   totalFiles,
	}

	// Note: Bandwidth throttling not applicable for local file copy
	// Hyper-V exports are local operations

	_, err = io.Copy(dstFile, reader)
	if err != nil {
		return fmt.Errorf("copy file: %w", err)
	}

	return nil
}

// callbackProgressReader wraps an io.Reader to call progress callback
type callbackProgressReader struct {
	reader       io.Reader
	total        int64
	currentBytes *int64
	callback     func(current, total int64, fileName string, fileIndex, totalFiles int)
	fileName     string
	fileIndex    int
	totalFiles   int
}

func (cpr *callbackProgressReader) Read(p []byte) (int, error) {
	n, err := cpr.reader.Read(p)

	// Atomically update current bytes
	current := atomic.AddInt64(cpr.currentBytes, int64(n))

	// Call progress callback
	if cpr.callback != nil {
		cpr.callback(current, cpr.total, cpr.fileName, cpr.fileIndex, cpr.totalFiles)
	}

	return n, err
}

// StartVM starts a Hyper-V VM
func (c *Client) StartVM(ctx context.Context, vmName string) error {
	c.logger.Info("starting VM", "vm", vmName)

	script := fmt.Sprintf(`Start-VM -Name '%s'`, vmName)

	_, err := c.executePowerShell(ctx, script)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	c.logger.Info("VM started", "vm", vmName)
	return nil
}

// StopVM stops a Hyper-V VM
func (c *Client) StopVM(ctx context.Context, vmName string) error {
	c.logger.Info("stopping VM", "vm", vmName)

	script := fmt.Sprintf(`Stop-VM -Name '%s' -Force`, vmName)

	_, err := c.executePowerShell(ctx, script)
	if err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	c.logger.Info("VM stopped", "vm", vmName)
	return nil
}

// DeleteVM deletes a Hyper-V VM
func (c *Client) DeleteVM(ctx context.Context, vmName string) error {
	c.logger.Info("deleting VM", "vm", vmName)

	script := fmt.Sprintf(`Remove-VM -Name '%s' -Force`, vmName)

	_, err := c.executePowerShell(ctx, script)
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	c.logger.Info("VM deleted", "vm", vmName)
	return nil
}

// executePowerShell executes a PowerShell script
func (c *Client) executePowerShell(ctx context.Context, script string) (string, error) {
	if c.config.UseWinRM {
		return c.executePowerShellWinRM(ctx, script)
	}
	return c.executePowerShellLocal(ctx, script)
}

// executePowerShellLocal executes PowerShell locally
func (c *Client) executePowerShellLocal(ctx context.Context, script string) (string, error) {
	// #nosec G204 -- script is built internally by this package's own VM-management methods
	// (StartVM/StopVM/DeleteVM/GetVM/ExportVM etc.) from operator-configured/local VM names, not
	// forwarded directly from an untrusted network request body.
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("PowerShell execution failed: %w", err)
	}

	return string(output), nil
}

// executePowerShellWinRM executes PowerShell via WinRM
func (c *Client) executePowerShellWinRM(ctx context.Context, script string) (string, error) {
	// For WinRM, we use the winrm command-line tool or a Go WinRM library
	// Here's a simple implementation using the winrm CLI tool

	// Escape script for WinRM
	escapedScript := strings.ReplaceAll(script, `"`, `\"`)
	escapedScript = strings.ReplaceAll(escapedScript, `'`, `''`)

	// Use winrm command-line tool
	// #nosec G204 -- host/username/password/port come from the operator-supplied Config
	// (not request input), and escapedScript is built internally from operator-configured VM names.
	cmd := exec.CommandContext(ctx, "winrm",
		"-hostname", c.config.Host,
		"-username", c.config.Username,
		"-password", c.config.Password,
		"-port", fmt.Sprintf("%d", c.config.WinRMPort),
		"-https="+fmt.Sprintf("%t", c.config.UseHTTPS),
		"powershell", escapedScript,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("WinRM PowerShell execution failed: %w", err)
	}

	return string(output), nil
}

// validateConnection validates the connection to Hyper-V host
func (c *Client) validateConnection() error {
	// Try a simple command to verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	script := "Get-Command Get-VM"
	_, err := c.executePowerShell(ctx, script)
	if err != nil {
		return fmt.Errorf("connection validation failed: %w", err)
	}

	c.logger.Info("Hyper-V connection validated")
	return nil
}

// saveMetadata saves VM metadata to a JSON file
func (c *Client) saveMetadata(vmInfo *VMInfo, path string) error {
	metadata := fmt.Sprintf(`{
  "provider": "hyperv",
  "vm_name": "%s",
  "vm_id": "%s",
  "state": "%s",
  "generation": %d,
  "cpu_usage": %d,
  "memory_assigned": %d,
  "path": "%s",
  "vhd_paths": %s,
  "export_time": "%s"
}`,
		vmInfo.Name,
		vmInfo.ID,
		vmInfo.State,
		vmInfo.Generation,
		vmInfo.CPUUsage,
		vmInfo.MemoryAssigned,
		vmInfo.Path,
		toJSONArray(vmInfo.VHDPath),
		time.Now().Format(time.RFC3339),
	)

	return os.WriteFile(path, []byte(metadata), 0600)
}

// toJSONArray converts string slice to JSON array string
func toJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}

	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(item, `"`, `\"`))
	}

	return "[" + strings.Join(quoted, ", ") + "]"
}

// Close cleans up resources
func (c *Client) Close() error {
	c.logger.Info("Hyper-V client closed")
	return nil
}

// String returns a string representation of the client
func (c *Client) String() string {
	if c.config.UseWinRM {
		return fmt.Sprintf("Hyper-V Client (remote=%s, winrm=true)", c.config.Host)
	}
	return "Hyper-V Client (local)"
}

// SearchVMs searches for VMs matching a query
func (c *Client) SearchVMs(ctx context.Context, query string) ([]*VMInfo, error) {
	vms, err := c.ListVMs(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var matches []*VMInfo

	for _, vm := range vms {
		if strings.Contains(strings.ToLower(vm.Name), query) ||
			strings.Contains(strings.ToLower(vm.State), query) ||
			strings.Contains(strings.ToLower(vm.Status), query) {
			matches = append(matches, vm)
		}
	}

	return matches, nil
}
