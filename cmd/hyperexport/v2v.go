// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zyvorai/transiva/logger"
)

// V2VConverter handles VM conversion from various formats to KVM
type V2VConverter struct {
	log        logger.Logger
	virtV2VBin string // Path to virt-v2v binary
}

// V2VConfig contains conversion configuration
type V2VConfig struct {
	InputFormat  string // "ova", "ovf", "vmx", "libvirt"
	OutputFormat string // "libvirt", "local", "rhev", "openstack"
	OutputPath   string // Destination for converted VM
	InputPath    string // Source VM/OVF/OVA path

	// KVM-specific options
	Network      string // Network mapping (e.g., "vmnet0:bridge:br0")
	Bridge       string // Default bridge name
	Storage      string // Storage pool name

	// Conversion options
	VMName       string // New VM name
	VirtIODrivers bool  // Install virtio drivers
	RemoveVMwareTools bool // Remove VMware Tools
	InstallGuestAgent bool // Install qemu-guest-agent

	// Advanced options
	Debug        bool
	Verbose      bool
	NoTrim       bool   // Don't trim/zero free space
	InPlace      bool   // Convert in-place (dangerous)
}

// ConversionResult contains conversion output
type ConversionResult struct {
	Success      bool
	VMName       string
	OutputPath   string
	LibvirtXML   string
	DiskImages   []string
	Warnings     []string
	Errors       []string
	Duration     string
}

// NewV2VConverter creates a new V2V converter
func NewV2VConverter(log logger.Logger) (*V2VConverter, error) {
	// Check if virt-v2v is available
	virtV2VPath, err := exec.LookPath("virt-v2v")
	if err != nil {
		return nil, fmt.Errorf("virt-v2v not found in PATH (install libguestfs-tools): %w", err)
	}

	return &V2VConverter{
		log:        log,
		virtV2VBin: virtV2VPath,
	}, nil
}

// Convert performs the VM conversion
func (c *V2VConverter) Convert(ctx context.Context, config *V2VConfig) (*ConversionResult, error) {
	c.log.Info("starting V2V conversion",
		"input", config.InputPath,
		"output", config.OutputPath,
		"format", config.InputFormat)

	// Build virt-v2v command
	args := c.buildVirtV2VArgs(config)

	// Execute virt-v2v
	// #nosec G204 -- virtV2VBin resolved via exec.LookPath and args are built from local CLI-supplied V2VConfig, not remote input
	cmd := exec.CommandContext(ctx, c.virtV2VBin, args...)

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result := &ConversionResult{
		Success: err == nil,
		VMName:  config.VMName,
		OutputPath: config.OutputPath,
	}

	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.Errors = append(result.Errors, outputStr)
		c.log.Error("conversion failed", "error", err, "output", outputStr)
		return result, fmt.Errorf("virt-v2v failed: %w", err)
	}

	// Parse output for warnings
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "warning") {
			result.Warnings = append(result.Warnings, line)
		}
	}

	// Find converted disk images
	result.DiskImages = c.findConvertedDisks(config.OutputPath)

	// Find libvirt XML if output to libvirt
	if config.OutputFormat == "libvirt" {
		xmlPath := filepath.Join(config.OutputPath, config.VMName+".xml")
		if _, err := os.Stat(xmlPath); err == nil {
			result.LibvirtXML = xmlPath
		}
	}

	c.log.Info("conversion completed successfully",
		"vm", config.VMName,
		"disks", len(result.DiskImages),
		"warnings", len(result.Warnings))

	return result, nil
}

// buildVirtV2VArgs constructs virt-v2v command arguments
func (c *V2VConverter) buildVirtV2VArgs(config *V2VConfig) []string {
	args := []string{}

	// Input specification
	switch config.InputFormat {
	case "ova":
		args = append(args, "-i", "ova", config.InputPath)
	case "ovf":
		args = append(args, "-i", "ova", config.InputPath)
	case "vmx":
		args = append(args, "-i", "vmx", config.InputPath)
	case "libvirt":
		args = append(args, "-i", "libvirt", "-ic", "qemu:///system", config.VMName)
	}

	// Output specification
	switch config.OutputFormat {
	case "libvirt":
		args = append(args, "-o", "libvirt", "-os", config.OutputPath)
	case "local":
		args = append(args, "-o", "local", "-os", config.OutputPath)
	case "rhev":
		args = append(args, "-o", "rhev")
	case "openstack":
		args = append(args, "-o", "openstack")
	}

	// Network mapping
	if config.Network != "" {
		args = append(args, "--bridge", config.Network)
	} else if config.Bridge != "" {
		args = append(args, "--bridge", config.Bridge)
	}

	// VM name
	if config.VMName != "" {
		args = append(args, "-n", config.VMName)
	}

	// Debug/verbose
	if config.Debug {
		args = append(args, "-x")
	}
	if config.Verbose {
		args = append(args, "-v", "-x")
	}

	// Additional options
	if config.NoTrim {
		args = append(args, "--no-trim")
	}

	return args
}

// findConvertedDisks finds disk images in output directory
func (c *V2VConverter) findConvertedDisks(outputPath string) []string {
	disks := []string{}

	// Common disk extensions
	extensions := []string{".qcow2", ".raw", ".img", ".vmdk"}

	for _, ext := range extensions {
		pattern := filepath.Join(outputPath, "*"+ext)
		matches, err := filepath.Glob(pattern)
		if err == nil {
			disks = append(disks, matches...)
		}
	}

	return disks
}

// CheckDependencies verifies required tools are installed
func (c *V2VConverter) CheckDependencies() error {
	required := []string{
		"virt-v2v",
		"qemu-img",
		"virt-inspector",
	}

	missing := []string{}
	for _, tool := range required {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s (install libguestfs-tools)", strings.Join(missing, ", "))
	}

	return nil
}

// GetVirtV2VVersion returns the virt-v2v version
func (c *V2VConverter) GetVirtV2VVersion() (string, error) {
	// #nosec G204 -- virtV2VBin resolved via exec.LookPath in NewV2VConverter; fixed argument
	cmd := exec.Command(c.virtV2VBin, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ConvertOVAToKVM is a high-level wrapper for OVA→KVM conversion
func ConvertOVAToKVM(ctx context.Context, ovaPath, outputDir, vmName string, log logger.Logger) error {
	converter, err := NewV2VConverter(log)
	if err != nil {
		return fmt.Errorf("create converter: %w", err)
	}

	// Check dependencies
	if err := converter.CheckDependencies(); err != nil {
		return err
	}

	config := &V2VConfig{
		InputFormat:  "ova",
		OutputFormat: "local",
		InputPath:    ovaPath,
		OutputPath:   outputDir,
		VMName:       vmName,
		Bridge:       "virbr0", // Default libvirt bridge
		VirtIODrivers: true,
		RemoveVMwareTools: true,
		InstallGuestAgent: true,
		Verbose:      true,
	}

	result, err := converter.Convert(ctx, config)
	if err != nil {
		return err
	}

	if len(result.Warnings) > 0 {
		log.Warn("conversion completed with warnings",
			"warnings", len(result.Warnings))
		for _, warning := range result.Warnings {
			log.Warn("conversion warning", "message", warning)
		}
	}

	log.Info("KVM conversion successful",
		"disks", len(result.DiskImages),
		"output", result.OutputPath)

	return nil
}
