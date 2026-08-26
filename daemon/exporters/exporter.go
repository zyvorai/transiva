// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"context"
	"fmt"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// Exporter defines the interface for VM export implementations
type Exporter interface {
	// Export performs VM export using the specific method
	Export(ctx context.Context, job *models.JobDefinition, progressCallback func(*models.JobProgress)) (*models.JobResult, error)

	// Method returns the export method name
	Method() capabilities.ExportMethod

	// Validate checks if this exporter can handle the job
	Validate(job *models.JobDefinition) error
}

// ExporterFactory creates exporters based on capabilities
type ExporterFactory struct {
	detector         *capabilities.Detector
	logger           logger.Logger
	providerRegistry *providers.Registry
	appConfig        *config.Config
}

// NewExporterFactory creates a new exporter factory
func NewExporterFactory(detector *capabilities.Detector, log logger.Logger) *ExporterFactory {
	return &ExporterFactory{
		detector: detector,
		logger:   log,
	}
}

// SetProviderContext configures provider-aware exporters (Nutanix).
func (f *ExporterFactory) SetProviderContext(registry *providers.Registry, appConfig *config.Config) {
	f.providerRegistry = registry
	f.appConfig = appConfig
}

// CreateNutanixExporter creates the Nutanix NFS pickup exporter.
func (f *ExporterFactory) CreateNutanixExporter() (Exporter, error) {
	if f.providerRegistry == nil {
		return nil, fmt.Errorf("provider registry is not configured")
	}
	return NewNutanixExporter(f.providerRegistry, f.appConfig, f.logger), nil
}

// CreateExporter creates an exporter for the specified method
func (f *ExporterFactory) CreateExporter(method capabilities.ExportMethod) (Exporter, error) {
	// Check if method is available
	if !f.detector.IsAvailable(method) {
		return nil, fmt.Errorf("export method %s is not available", method)
	}

	caps := f.detector.GetCapabilities()
	cap := caps[method]

	switch method {
	case capabilities.ExportMethodCTL:
		return NewCTLExporter(cap.Path, f.logger), nil

	case capabilities.ExportMethodGovc:
		return NewGovcExporter(cap.Path, f.logger), nil

	case capabilities.ExportMethodOvftool:
		return NewOvftoolExporter(cap.Path, f.logger), nil

	case capabilities.ExportMethodWeb:
		return NewWebExporter(f.logger), nil

	default:
		return nil, fmt.Errorf("unknown export method: %s", method)
	}
}

// GetOrCreateDefault creates an exporter using the default (best available) method
func (f *ExporterFactory) GetOrCreateDefault() (Exporter, error) {
	defaultMethod := f.detector.GetDefaultMethod()
	return f.CreateExporter(defaultMethod)
}

// GetAvailableMethods returns a list of available export methods in priority order
func (f *ExporterFactory) GetAvailableMethods() []capabilities.ExportMethod {
	methods := []capabilities.ExportMethod{
		capabilities.ExportMethodCTL,
		capabilities.ExportMethodGovc,
		capabilities.ExportMethodOvftool,
		capabilities.ExportMethodWeb,
	}

	var available []capabilities.ExportMethod
	for _, method := range methods {
		if f.detector.IsAvailable(method) {
			available = append(available, method)
		}
	}

	return available
}

// IsAvailable checks if a specific export method is available
func (f *ExporterFactory) IsAvailable(method capabilities.ExportMethod) bool {
	return f.detector.IsAvailable(method)
}

// GetDefaultMethod returns the default (highest priority) export method
func (f *ExporterFactory) GetDefaultMethod() capabilities.ExportMethod {
	return f.detector.GetDefaultMethod()
}
