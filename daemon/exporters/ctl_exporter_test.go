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

func TestNewCTLExporter(t *testing.T) {
	log := logger.New("info")
	exporter := NewCTLExporter("/usr/bin/hyperctl", log)

	if exporter == nil {
		t.Fatal("NewCTLExporter returned nil")
	}

	if exporter.ctlPath != "/usr/bin/hyperctl" {
		t.Errorf("Expected path /usr/bin/hyperctl, got %s", exporter.ctlPath)
	}

	if exporter.logger == nil {
		t.Error("logger not set")
	}
}

func TestCTLExporter_Method(t *testing.T) {
	log := logger.New("info")
	exporter := NewCTLExporter("/usr/bin/hyperctl", log)

	if exporter.Method() != capabilities.ExportMethodCTL {
		t.Errorf("Expected method ctl, got %s", exporter.Method())
	}
}

func TestCTLExporter_Validate(t *testing.T) {
	log := logger.New("info")
	exporter := NewCTLExporter("/usr/bin/hyperctl", log)

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
			name: "missing vcenter server",
			job: &models.JobDefinition{
				VMPath:    "/datacenter/vm/test-vm",
				OutputDir: "/tmp/output",
				VCenter: &models.VCenterConfig{
					Username: "admin",
				},
			},
			wantErr: true,
		},
		{
			name: "nil vcenter config",
			job: &models.JobDefinition{
				VMPath:    "/datacenter/vm/test-vm",
				OutputDir: "/tmp/output",
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

func TestCTLExporter_BuildArgs(t *testing.T) {
	log := logger.New("info")
	exporter := NewCTLExporter("/usr/bin/hyperctl", log)

	job := &models.JobDefinition{
		VMPath:    "/datacenter/vm/test-vm",
		OutputDir: "/tmp/output",
		Format:    "ova",
		VCenter: &models.VCenterConfig{
			Server:   "vcenter.example.com",
			Username: "admin",
			Password: "secret",
			Insecure: true,
		},
		Compress: true,
		Thin:     true,
	}

	// We can't easily test the actual command execution, but we can validate the job
	err := exporter.Validate(job)
	if err != nil {
		t.Errorf("Valid job failed validation: %v", err)
	}
}

func TestCTLExporter_Export_InvalidJob(t *testing.T) {
	log := logger.New("info")
	exporter := NewCTLExporter("/usr/bin/hyperctl", log)

	invalidJob := &models.JobDefinition{
		// Missing required fields
	}

	ctx := context.Background()
	_, err := exporter.Export(ctx, invalidJob, nil)

	// Should fail validation
	if err == nil {
		t.Error("Expected error for invalid job, got nil")
	}
}

func TestCTLExporter_monitorProgress(t *testing.T) {
	log := logger.New("info")
	exporter := NewCTLExporter("/usr/bin/hyperctl", log)

	tests := []struct {
		name             string
		input            string
		expectCallback   bool
		expectedPercent  float64
		expectedStep     string
	}{
		{
			name:            "valid progress line",
			input:           "Progress: 45.5%\n",
			expectCallback:  true,
			expectedPercent: 45.5,
			expectedStep:    "Progress: 45.5%",
		},
		{
			name:            "multiple progress lines",
			input:           "Starting export\nProgress: 25.0%\nProgress: 50.0%\nDone\n",
			expectCallback:  true,
			expectedPercent: 50.0,  // Last progress update
			expectedStep:    "Progress: 50.0%",
		},
		{
			name:           "no progress lines",
			input:          "Starting export\nDone\n",
			expectCallback: false,
		},
		{
			name:           "malformed progress line",
			input:          "Progress: abc%\n",
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

				if lastProgress.PercentComplete != tt.expectedPercent {
					t.Errorf("Expected percent %f, got %f", tt.expectedPercent, lastProgress.PercentComplete)
				}

				if lastProgress.CurrentStep != tt.expectedStep {
					t.Errorf("Expected step '%s', got '%s'", tt.expectedStep, lastProgress.CurrentStep)
				}

				if lastProgress.Phase != "exporting" {
					t.Errorf("Expected phase 'exporting', got '%s'", lastProgress.Phase)
				}

				if string(lastProgress.ExportMethod) != "ctl" {
					t.Errorf("Expected export method 'ctl', got '%s'", lastProgress.ExportMethod)
				}
			} else {
				if lastProgress != nil {
					t.Errorf("Expected no callback, but got progress: %+v", lastProgress)
				}
			}
		})
	}
}

func TestCTLExporter_monitorErrors(t *testing.T) {
	log := logger.New("info")
	exporter := NewCTLExporter("/usr/bin/hyperctl", log)

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
