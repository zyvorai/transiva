// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"testing"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

func TestIsNutanixJob(t *testing.T) {
	tests := []struct {
		name string
		job  *models.JobDefinition
		want bool
	}{
		{
			name: "provider field",
			job:  &models.JobDefinition{Provider: "nutanix"},
			want: true,
		},
		{
			name: "export method",
			job:  &models.JobDefinition{ExportMethod: "nutanix"},
			want: true,
		},
		{
			name: "metadata provider",
			job:  &models.JobDefinition{Metadata: map[string]interface{}{"provider": "nutanix"}},
			want: true,
		},
		{
			name: "vsphere default",
			job:  &models.JobDefinition{VMPath: "/dc/vm/test"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNutanixJob(tt.job); got != tt.want {
				t.Fatalf("IsNutanixJob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNutanixExporterValidate(t *testing.T) {
	exporter := NewNutanixExporter(providers.NewRegistry(), &config.Config{
		Nutanix: &config.NutanixConfig{
			Mounts: map[string]string{"default-container": "/mnt/default"},
		},
	}, logger.New("error"))

	err := exporter.Validate(&models.JobDefinition{
		VMPath:     "web-01",
		OutputPath: "/tmp/out",
	})
	if err != nil {
		t.Fatalf("Validate() = %v", err)
	}

	err = exporter.Validate(&models.JobDefinition{
		VMPath: "web-01",
	})
	if err == nil {
		t.Fatal("expected error for missing output_dir")
	}
}

func TestResolveJobMounts(t *testing.T) {
	job := &models.JobDefinition{
		Metadata: map[string]interface{}{
			"mounts": map[string]string{"ctr-a": "/mnt/a"},
		},
	}
	mounts, err := resolveJobMounts(job, nil)
	if err != nil {
		t.Fatalf("resolveJobMounts: %v", err)
	}
	if mounts["ctr-a"] != "/mnt/a" {
		t.Fatalf("mounts = %v", mounts)
	}
}
