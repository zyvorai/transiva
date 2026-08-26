// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"bufio"
	"context"
	"strings"
	"testing"

	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

func TestNewGovcExporter(t *testing.T) {
	log := logger.New("info")
	exporter := NewGovcExporter("/usr/bin/govc", log)

	if exporter == nil {
		t.Fatal("NewGovcExporter returned nil")
	}

	if exporter.govcPath != "/usr/bin/govc" {
		t.Errorf("Expected path /usr/bin/govc, got %s", exporter.govcPath)
	}
}

func TestGovcExporter_Method(t *testing.T) {
	log := logger.New("info")
	exporter := NewGovcExporter("/usr/bin/govc", log)

	if exporter.Method() != capabilities.ExportMethodGovc {
		t.Errorf("Expected method govc, got %s", exporter.Method())
	}
}

func TestGovcExporter_Validate(t *testing.T) {
	log := logger.New("info")
	exporter := NewGovcExporter("/usr/bin/govc", log)

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
					Server: "vcenter.example.com",
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
					Server: "",
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

func TestGovcExporter_Export_InvalidJob(t *testing.T) {
	log := logger.New("info")
	exporter := NewGovcExporter("/usr/bin/govc", log)

	invalidJob := &models.JobDefinition{}

	ctx := context.Background()
	_, err := exporter.Export(ctx, invalidJob, nil)

	if err == nil {
		t.Error("Expected error for invalid job, got nil")
	}
}

func TestGovcExporter_monitorErrors(t *testing.T) {
	log := logger.New("info")
	exporter := NewGovcExporter("/usr/bin/govc", log)

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single error line",
			input: "Error: connection failed\n",
		},
		{
			name:  "multiple error lines",
			input: "Warning: deprecated option\nError: file not found\n",
		},
		{
			name:  "no errors",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a reader from the test input
			reader := bufio.NewReader(strings.NewReader(tt.input))

			// Run the monitor - it should not panic
			exporter.monitorErrors(reader)

			// If we get here without panic, the test passes
		})
	}
}
