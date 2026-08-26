// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"time"
)

// Converter defines the interface for VM conversion tools (e.g., hyper2kvm)
// This interface is provider-agnostic and can be used with vSphere, AWS, Azure, GCP, etc.
type Converter interface {
	// Convert runs the conversion process on the given manifest
	Convert(ctx context.Context, manifestPath string, opts ConvertOptions) (*ConversionResult, error)

	// GetVersion returns the converter tool version
	GetVersion() (string, error)

	// Validate checks if the converter is properly configured
	Validate() error
}

// ConvertOptions holds options for VM conversion
type ConvertOptions struct {
	// StreamOutput enables real-time output streaming to console
	StreamOutput bool

	// Verbose enables verbose conversion logging
	Verbose bool

	// DryRun performs a dry run without actual conversion
	DryRun bool

	// TargetFormat specifies the target disk format (qcow2, raw, vmdk)
	TargetFormat string

	// CompressionLevel specifies compression level (0-9, 0=none)
	CompressionLevel int

	// PreserveMAC preserves source MAC addresses
	PreserveMAC bool

	// CustomArgs allows passing custom arguments to the converter
	CustomArgs []string
}

// ConversionResult holds the result of a VM conversion
type ConversionResult struct {
	// Success indicates if the conversion succeeded
	Success bool

	// ConvertedFiles lists all converted disk files (qcow2, raw, etc.)
	ConvertedFiles []string

	// ReportPath points to the conversion report JSON file
	ReportPath string

	// Duration is the total conversion time
	Duration time.Duration

	// Error contains the error message if conversion failed
	Error string

	// Metadata contains provider-specific metadata
	Metadata map[string]interface{}

	// Warnings contains non-fatal warnings from conversion
	Warnings []string
}

// ConverterConfig holds configuration for converter initialization
type ConverterConfig struct {
	// BinaryPath is the path to the converter binary
	// If empty, auto-detection will be attempted
	BinaryPath string

	// Timeout is the maximum duration for conversion
	Timeout time.Duration

	// Environment variables for the converter process
	Environment map[string]string

	// WorkingDirectory for the converter process
	WorkingDirectory string
}

// ConverterCapabilities describes what a converter can do
type ConverterCapabilities struct {
	// SupportedSourceFormats lists supported source formats
	SupportedSourceFormats []string

	// SupportedTargetFormats lists supported target formats
	SupportedTargetFormats []string

	// SupportsDriverInjection indicates if virtio driver injection is supported
	SupportsDriverInjection bool

	// SupportsOSDetection indicates if OS detection is supported
	SupportsOSDetection bool

	// SupportsValidation indicates if image validation is supported
	SupportsValidation bool

	// SupportedProviders lists supported cloud providers
	SupportedProviders []string
}
