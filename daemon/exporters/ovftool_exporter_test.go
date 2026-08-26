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

func TestNewOvftoolExporter(t *testing.T) {
	log := logger.New("info")
	exporter := NewOvftoolExporter("/usr/bin/ovftool", log)

	if exporter == nil {
		t.Fatal("NewOvftoolExporter returned nil")
	}

	if exporter.ovftoolPath != "/usr/bin/ovftool" {
		t.Errorf("Expected path /usr/bin/ovftool, got %s", exporter.ovftoolPath)
	}
}

func TestOvftoolExporter_Method(t *testing.T) {
	log := logger.New("info")
	exporter := NewOvftoolExporter("/usr/bin/ovftool", log)

	if exporter.Method() != capabilities.ExportMethodOvftool {
		t.Errorf("Expected method ovftool, got %s", exporter.Method())
	}
}

func TestOvftoolExporter_Validate(t *testing.T) {
	log := logger.New("info")
	exporter := NewOvftoolExporter("/usr/bin/ovftool", log)

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
					Server:   "vcenter.example.com",
					Username: "admin",
					Password: "password",
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

func TestOvftoolExporter_Export_InvalidJob(t *testing.T) {
	log := logger.New("info")
	exporter := NewOvftoolExporter("/usr/bin/ovftool", log)

	invalidJob := &models.JobDefinition{}

	ctx := context.Background()
	_, err := exporter.Export(ctx, invalidJob, nil)

	if err == nil {
		t.Error("Expected error for invalid job, got nil")
	}
}

func TestOvftoolExporter_monitorProgress(t *testing.T) {
	log := logger.New("info")
	exporter := NewOvftoolExporter("/usr/bin/ovftool", log)

	tests := []struct {
		name                 string
		input                string
		expectCallback       bool
		expectedPercent      float64
		expectedStep         string
		checkPercentComplete bool
	}{
		{
			name:                 "valid progress line",
			input:                "Progress: 45%\n",
			expectCallback:       true,
			expectedPercent:      45.0,
			expectedStep:         "Progress: 45%",
			checkPercentComplete: true,
		},
		{
			name:                 "disk transfer message",
			input:                "Transfer Disk 1 of 2\n",
			expectCallback:       true,
			expectedStep:         "Transfer Disk 1 of 2",
			checkPercentComplete: false,
		},
		{
			name:                 "disk message",
			input:                "Disk progress: 50MB of 100MB\n",
			expectCallback:       true,
			expectedStep:         "Disk progress: 50MB of 100MB",
			checkPercentComplete: false,
		},
		{
			name:           "no progress lines",
			input:          "Starting export\nDone\n",
			expectCallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a reader from the test input
			reader := bufio.NewReader(strings.NewReader(tt.input))

			var lastProgress *models.JobProgress
			callback := func(progress *models.JobProgress) {
				lastProgress = progress
			}

			// Run the monitor
			exporter.monitorProgress(reader, callback)

			if tt.expectCallback {
				if lastProgress == nil {
					t.Fatal("Expected progress callback to be called, but it wasn't")
				}

				if tt.checkPercentComplete && lastProgress.PercentComplete != tt.expectedPercent {
					t.Errorf("Expected percent %f, got %f", tt.expectedPercent, lastProgress.PercentComplete)
				}

				if lastProgress.CurrentStep != tt.expectedStep {
					t.Errorf("Expected step '%s', got '%s'", tt.expectedStep, lastProgress.CurrentStep)
				}

				if lastProgress.Phase != "exporting" {
					t.Errorf("Expected phase 'exporting', got '%s'", lastProgress.Phase)
				}

				if string(lastProgress.ExportMethod) != "ovftool" {
					t.Errorf("Expected export method 'ovftool', got '%s'", lastProgress.ExportMethod)
				}
			} else {
				if lastProgress != nil {
					t.Errorf("Expected no callback, but got progress: %+v", lastProgress)
				}
			}
		})
	}
}

func TestOvftoolExporter_monitorErrors(t *testing.T) {
	log := logger.New("info")
	exporter := NewOvftoolExporter("/usr/bin/ovftool", log)

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
