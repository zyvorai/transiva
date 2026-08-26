// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"context"
	"fmt"
	"time"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// WebExporter implements export using HTTP/NFC (current implementation via govmomi)
type WebExporter struct {
	logger logger.Logger
}

// NewWebExporter creates a new web/HTTP exporter
func NewWebExporter(log logger.Logger) *WebExporter {
	return &WebExporter{
		logger: log,
	}
}

// Export performs VM export using HTTP/NFC protocol
func (e *WebExporter) Export(ctx context.Context, job *models.JobDefinition, progressCallback func(*models.JobProgress)) (*models.JobResult, error) {
	// Validate job first
	if err := e.Validate(job); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	e.logger.Info("starting web/HTTP export", "vm", job.VMPath)

	// Create vSphere config from job definition
	cfg := &config.Config{
		VCenterURL: fmt.Sprintf("https://%s/sdk", job.VCenter.Server),
		Username:   job.VCenter.Username,
		Password:   job.VCenter.Password,
		Insecure:   job.VCenter.Insecure,
		Timeout:    30 * time.Second,
	}

	// Create vSphere client
	client, err := vsphere.NewVSphereClient(ctx, cfg, e.logger)
	if err != nil {
		return nil, fmt.Errorf("create vsphere client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Update progress
	if progressCallback != nil {
		progressCallback(&models.JobProgress{
			Phase:        "connecting",
			ExportMethod: string(capabilities.ExportMethodWeb),
		})
	}

	// Prepare export options (merge job.Options for pipeline, enable_rdp, etc.)
	opts := vsphere.DefaultExportOptions()
	if job.Options != nil {
		opts = job.Options.ToVSphereOptions(job.OutputDir)
	} else {
		opts.OutputPath = job.OutputDir
	}
	if job.Format != "" {
		opts.Format = job.Format
	}
	opts.RemoveCDROM = true

	// Note: govmomi doesn't support compress/thin options in the same way
	// These would be handled post-export or via the export method choice

	// Perform export using existing vsphere package
	result, err := client.ExportOVF(ctx, job.VMPath, opts)
	if err != nil {
		return nil, fmt.Errorf("export ovf: %w", err)
	}

	// Convert to job result
	jobResult := &models.JobResult{
		Success:      true,
		OutputFiles:  result.Files,
		TotalSize:    result.TotalSize,
		ExportMethod: string(capabilities.ExportMethodWeb),
	}

	e.logger.Info("web/HTTP export completed successfully",
		"vm", job.VMPath,
		"files", len(result.Files),
		"size", result.TotalSize)

	return jobResult, nil
}

// Method returns the export method name
func (e *WebExporter) Method() capabilities.ExportMethod {
	return capabilities.ExportMethodWeb
}

// Validate checks if this exporter can handle the job
func (e *WebExporter) Validate(job *models.JobDefinition) error {
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

	return nil
}
