// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zyvorai/transiva/logger"
)

// ConversionManager manages VM conversions across multiple providers
type ConversionManager struct {
	config    *ConverterConfig
	logger    logger.Logger
	converter Converter
}

// NewConversionManager creates a new conversion manager
func NewConversionManager(config *ConverterConfig, log logger.Logger) (*ConversionManager, error) {
	if config == nil {
		config = &ConverterConfig{}
	}

	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 2 * 3600 * 1e9 // 2 hours in nanoseconds
	}

	return &ConversionManager{
		config: config,
		logger: log,
	}, nil
}

// SetConverter sets the converter implementation
func (cm *ConversionManager) SetConverter(converter Converter) {
	cm.converter = converter
}

// Convert performs VM conversion using the configured converter
func (cm *ConversionManager) Convert(ctx context.Context, manifestPath string, opts ConvertOptions) (*ConversionResult, error) {
	if cm.converter == nil {
		return nil, fmt.Errorf("no converter configured")
	}

	// Validate converter is ready
	if err := cm.converter.Validate(); err != nil {
		return nil, fmt.Errorf("converter validation failed: %w", err)
	}

	// Run conversion
	cm.logger.Info("starting VM conversion", "manifest", manifestPath)
	result, err := cm.converter.Convert(ctx, manifestPath, opts)
	if err != nil {
		cm.logger.Error("conversion failed", "error", err)
		return result, err
	}

	if result.Success {
		cm.logger.Info("conversion completed successfully",
			"duration", result.Duration,
			"files", len(result.ConvertedFiles))
	} else {
		cm.logger.Error("conversion failed", "error", result.Error)
	}

	return result, nil
}

// DetectProvider detects the cloud provider from the manifest
func (cm *ConversionManager) DetectProvider(manifestPath string) (string, error) {
	// Read manifest to detect provider. manifestPath is supplied by the local
	// conversion workflow (CLI/config), not derived from an HTTP request.
	// #nosec G304 -- manifestPath is a local conversion artifact path controlled by the calling code, not remote input
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}

	content := string(data)

	// Simple provider detection based on manifest content
	if strings.Contains(content, `"provider":"vsphere"`) || strings.Contains(content, `"provider":"vmware"`) {
		return "vsphere", nil
	}
	if strings.Contains(content, `"provider":"aws"`) || strings.Contains(content, "ec2") {
		return "aws", nil
	}
	if strings.Contains(content, `"provider":"azure"`) {
		return "azure", nil
	}
	if strings.Contains(content, `"provider":"gcp"`) || strings.Contains(content, "compute") {
		return "gcp", nil
	}

	return "unknown", fmt.Errorf("unable to detect provider from manifest")
}

// AutoDetectConverterBinary attempts to find the converter binary
func AutoDetectConverterBinary(binaryName string) (string, error) {
	// Try common locations
	candidates := []string{
		binaryName,                     // In PATH
		"/usr/local/bin/" + binaryName, // System install
		"/usr/bin/" + binaryName,       // Package manager
		filepath.Join(os.Getenv("HOME"), ".local/bin", binaryName), // User install
	}

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("%s binary not found in PATH or common locations", binaryName)
}

// ValidateBinary checks if a binary exists and is executable
func ValidateBinary(path string) error {
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

// GetCapabilities returns the converter's capabilities
func (cm *ConversionManager) GetCapabilities() (*ConverterCapabilities, error) {
	// This would be implemented by each converter
	// For now, return default capabilities
	return &ConverterCapabilities{
		SupportedSourceFormats:  []string{"vmdk", "vhd", "vhdx", "raw"},
		SupportedTargetFormats:  []string{"qcow2", "raw"},
		SupportsDriverInjection: true,
		SupportsOSDetection:     true,
		SupportsValidation:      true,
		SupportedProviders:      []string{"vsphere", "aws", "azure", "gcp"},
	}, nil
}
