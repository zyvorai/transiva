// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"context"
	"fmt"
	"testing"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func TestNewExporterFactory(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	factory := NewExporterFactory(detector, log)

	if factory == nil {
		t.Fatal("NewExporterFactory returned nil")
	}

	if factory.detector == nil {
		t.Error("detector not set")
	}

	if factory.logger == nil {
		t.Error("logger not set")
	}
}

func TestCreateExporter_WebMethod(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	// Mark web as available
	detector.Detect(context.Background())

	factory := NewExporterFactory(detector, log)

	exporter, err := factory.CreateExporter(capabilities.ExportMethodWeb)
	if err != nil {
		t.Fatalf("Failed to create web exporter: %v", err)
	}

	if exporter == nil {
		t.Fatal("CreateExporter returned nil")
	}

	if exporter.Method() != capabilities.ExportMethodWeb {
		t.Errorf("Expected web method, got %s", exporter.Method())
	}
}

func TestCreateExporter_UnavailableMethod(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	// Don't detect anything, so methods are unavailable
	factory := NewExporterFactory(detector, log)

	// Try to create CTL exporter when it's not available
	_, err := factory.CreateExporter(capabilities.ExportMethodCTL)
	if err == nil {
		t.Error("Expected error when creating unavailable exporter")
	}
}

func TestCreateExporter_AllMethods(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	// Run detection
	ctx := context.Background()
	detector.Detect(ctx)

	factory := NewExporterFactory(detector, log)

	// Test all methods that are available
	methods := []capabilities.ExportMethod{
		capabilities.ExportMethodCTL,
		capabilities.ExportMethodGovc,
		capabilities.ExportMethodOvftool,
		capabilities.ExportMethodWeb,
	}

	for _, method := range methods {
		t.Run(string(method), func(t *testing.T) {
			if !detector.IsAvailable(method) {
				t.Skipf("Method %s not available, skipping", method)
			}

			exporter, err := factory.CreateExporter(method)
			if err != nil {
				t.Fatalf("Failed to create %s exporter: %v", method, err)
			}

			if exporter == nil {
				t.Fatal("CreateExporter returned nil")
			}

			if exporter.Method() != method {
				t.Errorf("Expected method %s, got %s", method, exporter.Method())
			}
		})
	}
}

func TestCreateExporter_UnknownMethod(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	// Manually add a fake method as available
	detector.Detect(context.Background())

	factory := NewExporterFactory(detector, log)

	// Try to create exporter with unknown method
	_, err := factory.CreateExporter(capabilities.ExportMethod("unknown"))
	if err == nil {
		t.Error("Expected error for unknown export method")
	}
}

func TestGetOrCreateDefault(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	ctx := context.Background()
	detector.Detect(ctx)

	factory := NewExporterFactory(detector, log)

	exporter, err := factory.GetOrCreateDefault()
	if err != nil {
		t.Fatalf("Failed to create default exporter: %v", err)
	}

	if exporter == nil {
		t.Fatal("GetOrCreateDefault returned nil")
	}

	// Should match the best available method
	defaultMethod := detector.GetDefaultMethod()
	if exporter.Method() != defaultMethod {
		t.Errorf("Expected default method %s, got %s", defaultMethod, exporter.Method())
	}
}

// Mock exporter for testing interface
type mockExporter struct {
	method capabilities.ExportMethod
}

func (m *mockExporter) Export(ctx context.Context, job *models.JobDefinition, progressCallback func(*models.JobProgress)) (*models.JobResult, error) {
	return &models.JobResult{
		Success:      true,
		ExportMethod: string(m.method),
	}, nil
}

func (m *mockExporter) Method() capabilities.ExportMethod {
	return m.method
}

func (m *mockExporter) Validate(job *models.JobDefinition) error {
	if job.VMPath == "" {
		return fmt.Errorf("vm_path is required")
	}
	return nil
}

func TestExporterInterface(t *testing.T) {
	var _ Exporter = (*mockExporter)(nil)

	mock := &mockExporter{method: capabilities.ExportMethodWeb}

	// Test Method()
	if mock.Method() != capabilities.ExportMethodWeb {
		t.Errorf("Expected web method, got %s", mock.Method())
	}

	// Test Validate()
	job := &models.JobDefinition{
		VMPath: "test-vm",
	}

	err := mock.Validate(job)
	if err != nil {
		t.Errorf("Unexpected validation error: %v", err)
	}

	// Test validation failure
	emptyJob := &models.JobDefinition{}
	err = mock.Validate(emptyJob)
	if err == nil {
		t.Error("Expected validation error for empty job")
	}

	// Test Export()
	ctx := context.Background()
	result, err := mock.Export(ctx, job, nil)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful export")
	}

	if result.ExportMethod != string(capabilities.ExportMethodWeb) {
		t.Errorf("Expected web export method, got %s", result.ExportMethod)
	}
}

func TestExporterValidation(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	ctx := context.Background()
	detector.Detect(ctx)

	factory := NewExporterFactory(detector, log)

	// Get web exporter (always available)
	exporter, err := factory.CreateExporter(capabilities.ExportMethodWeb)
	if err != nil {
		t.Fatalf("Failed to create web exporter: %v", err)
	}

	tests := []struct {
		name    string
		job     *models.JobDefinition
		wantErr bool
	}{
		{
			name: "valid job",
			job: &models.JobDefinition{
				VMPath:    "/vm/path",
				OutputDir: "/output",
				VCenter: &models.VCenterConfig{
					Server:   "vcenter.example.com",
					Username: "user",
					Password: "pass",
				},
			},
			wantErr: false,
		},
		{
			name: "missing vm_path",
			job: &models.JobDefinition{
				OutputDir: "/output",
				VCenter: &models.VCenterConfig{
					Server: "vcenter.example.com",
				},
			},
			wantErr: true,
		},
		{
			name: "missing output_dir",
			job: &models.JobDefinition{
				VMPath: "/vm/path",
				VCenter: &models.VCenterConfig{
					Server: "vcenter.example.com",
				},
			},
			wantErr: true,
		},
		{
			name: "missing vcenter server",
			job: &models.JobDefinition{
				VMPath:    "/vm/path",
				OutputDir: "/output",
				VCenter: &models.VCenterConfig{
					Username: "user",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := exporter.Validate(tt.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetAvailableMethods(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	ctx := context.Background()
	detector.Detect(ctx)

	factory := NewExporterFactory(detector, log)

	// Get available methods
	methods := factory.GetAvailableMethods()

	// Should return at least web (always available)
	if len(methods) == 0 {
		t.Error("Expected at least one available method")
	}

	// Verify all returned methods are actually available
	for _, method := range methods {
		if !detector.IsAvailable(method) {
			t.Errorf("Method %s reported as available but detector says it's not", method)
		}
	}

	// Web should always be available
	foundWeb := false
	for _, method := range methods {
		if method == capabilities.ExportMethodWeb {
			foundWeb = true
			break
		}
	}
	if !foundWeb {
		t.Error("Expected web method to be in available methods")
	}
}

func TestIsAvailable(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	ctx := context.Background()
	detector.Detect(ctx)

	factory := NewExporterFactory(detector, log)

	tests := []struct {
		name   string
		method capabilities.ExportMethod
	}{
		{
			name:   "web method",
			method: capabilities.ExportMethodWeb,
		},
		{
			name:   "ctl method",
			method: capabilities.ExportMethodCTL,
		},
		{
			name:   "govc method",
			method: capabilities.ExportMethodGovc,
		},
		{
			name:   "ovftool method",
			method: capabilities.ExportMethodOvftool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should match detector's result
			factoryResult := factory.IsAvailable(tt.method)
			detectorResult := detector.IsAvailable(tt.method)

			if factoryResult != detectorResult {
				t.Errorf("Factory.IsAvailable(%s) = %v, but detector says %v",
					tt.method, factoryResult, detectorResult)
			}
		})
	}
}

func TestGetDefaultMethod(t *testing.T) {
	log := logger.New("info")
	detector := capabilities.NewDetector(log)

	ctx := context.Background()
	detector.Detect(ctx)

	factory := NewExporterFactory(detector, log)

	// Get default method from factory
	factoryDefault := factory.GetDefaultMethod()

	// Should match detector's default
	detectorDefault := detector.GetDefaultMethod()

	if factoryDefault != detectorDefault {
		t.Errorf("Factory.GetDefaultMethod() = %s, but detector says %s",
			factoryDefault, detectorDefault)
	}

	// Default method should be available
	if !factory.IsAvailable(factoryDefault) {
		t.Errorf("Default method %s is not available", factoryDefault)
	}

	// Default should be in the list of available methods
	methods := factory.GetAvailableMethods()
	found := false
	for _, method := range methods {
		if method == factoryDefault {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Default method %s not in available methods list", factoryDefault)
	}
}
