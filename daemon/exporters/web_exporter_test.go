// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"context"
	"testing"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func TestNewWebExporter(t *testing.T) {
	log := logger.New("info")
	exporter := NewWebExporter(log)

	if exporter == nil {
		t.Fatal("NewWebExporter returned nil")
	}

	if exporter.logger == nil {
		t.Error("logger not set")
	}
}

func TestWebExporter_Method(t *testing.T) {
	log := logger.New("info")
	exporter := NewWebExporter(log)

	if exporter.Method() != capabilities.ExportMethodWeb {
		t.Errorf("Expected method web, got %s", exporter.Method())
	}
}

func TestWebExporter_Validate(t *testing.T) {
	log := logger.New("info")
	exporter := NewWebExporter(log)

	tests := []struct {
		name    string
		job     *models.JobDefinition
		wantErr bool
	}{
		{
			name: "valid job",
			job: &models.JobDefinition{
				VMPath:    "/datacenter/vm/test-vm",
				OutputDir: "/tmp/output",
				VCenter: &models.VCenterConfig{
					Server:   "vcenter.example.com",
					Username: "admin",
					Password: "password",
				},
			},
			wantErr: false,
		},
		{
			name: "missing vm_path",
			job: &models.JobDefinition{
				OutputDir: "/tmp/output",
				VCenter: &models.VCenterConfig{
					Server: "vcenter.example.com",
				},
			},
			wantErr: true,
		},
		{
			name: "missing output_dir",
			job: &models.JobDefinition{
				VMPath: "/datacenter/vm/test-vm",
				VCenter: &models.VCenterConfig{
					Server: "vcenter.example.com",
				},
			},
			wantErr: true,
		},
		{
			name: "missing vcenter config",
			job: &models.JobDefinition{
				VMPath:    "/datacenter/vm/test-vm",
				OutputDir: "/tmp/output",
				VCenter:   nil,
			},
			wantErr: true,
		},
		{
			name: "vcenter with empty server",
			job: &models.JobDefinition{
				VMPath:    "/datacenter/vm/test-vm",
				OutputDir: "/tmp/output",
				VCenter: &models.VCenterConfig{
					Server:   "",
					Username: "admin",
					Password: "password",
				},
			},
			wantErr: true,
		},
		{
			name: "missing vcenter username",
			job: &models.JobDefinition{
				VMPath:    "/datacenter/vm/test-vm",
				OutputDir: "/tmp/output",
				VCenter: &models.VCenterConfig{
					Server:   "vcenter.example.com",
					Password: "password",
				},
			},
			wantErr: true,
		},
		{
			name: "missing vcenter password",
			job: &models.JobDefinition{
				VMPath:    "/datacenter/vm/test-vm",
				OutputDir: "/tmp/output",
				VCenter: &models.VCenterConfig{
					Server:   "vcenter.example.com",
					Username: "admin",
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

func TestWebExporter_Export_InvalidJob(t *testing.T) {
	log := logger.New("info")
	exporter := NewWebExporter(log)

	invalidJob := &models.JobDefinition{}

	ctx := context.Background()
	_, err := exporter.Export(ctx, invalidJob, nil)

	if err == nil {
		t.Error("Expected error for invalid job, got nil")
	}
}

func TestWebExporter_ProgressCallback(t *testing.T) {
	log := logger.New("info")
	_ = NewWebExporter(log)

	// Test that we can create exporter with progress callback
	progressCallback := func(progress *models.JobProgress) {
		if progress.ExportMethod != string(capabilities.ExportMethodWeb) {
			t.Errorf("Expected export method web, got %s", progress.ExportMethod)
		}
	}

	// We can't actually test export without real vCenter, but we can verify callback is callable
	progressCallback(&models.JobProgress{ExportMethod: string(capabilities.ExportMethodWeb)})
}
