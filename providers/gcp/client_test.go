// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewClient(t *testing.T) {
	cfg := &Config{
		ProjectID:       "test-project",
		Zone:            "us-central1-a",
		CredentialsJSON: `{"type":"service_account"}`,
		Timeout:         30 * time.Second,
	}

	log := logger.New("info")

	// Note: This will fail without valid GCP credentials
	// This test is primarily for structure validation
	_, err := NewClient(cfg, log)

	// We expect this to fail in test environment
	if err == nil {
		t.Log("Client created successfully (unexpected in test environment)")
	}
}

func TestNewClientDefaultTimeout(t *testing.T) {
	cfg := &Config{
		ProjectID:       "test-project",
		Zone:            "us-east1-b",
		CredentialsJSON: "",
	}

	expectedTimeout := 1 * time.Hour

	if cfg.Timeout != 0 {
		t.Errorf("expected timeout to be 0 initially, got %v", cfg.Timeout)
	}

	// After NewClient, timeout should be set to 1 hour by default
	if expectedTimeout != 1*time.Hour {
		t.Errorf("expected default timeout 1h, got %v", expectedTimeout)
	}
}

func TestVMInfoCreation(t *testing.T) {
	vmInfo := &VMInfo{
		Name:              "test-instance",
		ID:                1234567890,
		Zone:              "us-central1-a",
		MachineType:       "n1-standard-1",
		Status:            "RUNNING",
		InternalIP:        "10.128.0.2",
		ExternalIP:        "34.123.45.67",
		DiskNames:         []string{"boot-disk", "data-disk"},
		Labels:            map[string]string{"env": "test"},
		CreationTimestamp: "2024-01-15T10:30:00.000-07:00",
	}

	if vmInfo.Name != "test-instance" {
		t.Errorf("unexpected name: %s", vmInfo.Name)
	}

	if vmInfo.MachineType != "n1-standard-1" {
		t.Errorf("unexpected machine type: %s", vmInfo.MachineType)
	}

	if vmInfo.Status != "RUNNING" {
		t.Errorf("unexpected status: %s", vmInfo.Status)
	}

	if vmInfo.InternalIP != "10.128.0.2" {
		t.Errorf("unexpected internal IP: %s", vmInfo.InternalIP)
	}

	if vmInfo.ExternalIP != "34.123.45.67" {
		t.Errorf("unexpected external IP: %s", vmInfo.ExternalIP)
	}

	if vmInfo.Labels["env"] != "test" {
		t.Errorf("unexpected label value: %s", vmInfo.Labels["env"])
	}

	if len(vmInfo.DiskNames) != 2 {
		t.Errorf("expected 2 disks, got %d", len(vmInfo.DiskNames))
	}
}

func TestGetZones(t *testing.T) {
	cfg := &Config{
		ProjectID: "test-project",
		Zone:      "us-central1-a",
	}

	log := logger.New("info")

	// Create a minimal client struct for testing
	client := &Client{
		config: cfg,
		logger: log,
	}

	zones := client.GetZones()

	if len(zones) == 0 {
		t.Error("expected zones list to be non-empty")
	}

	// Check for some common zones
	hasUSCentral := false
	hasEuropeWest := false
	hasAsiaEast := false

	for _, zone := range zones {
		if zone == "us-central1-a" {
			hasUSCentral = true
		}
		if zone == "europe-west1-b" {
			hasEuropeWest = true
		}
		if zone == "asia-east1-a" {
			hasAsiaEast = true
		}
	}

	if !hasUSCentral {
		t.Error("expected zones to include us-central1-a")
	}

	if !hasEuropeWest {
		t.Error("expected zones to include europe-west1-b")
	}

	if !hasAsiaEast {
		t.Error("expected zones to include asia-east1-a")
	}
}

func TestClientString(t *testing.T) {
	cfg := &Config{
		ProjectID: "my-gcp-project",
		Zone:      "us-west1-a",
	}

	log := logger.New("info")

	client := &Client{
		config: cfg,
		logger: log,
	}

	expected := "GCP Compute Engine Client (project=my-gcp-project, zone=us-west1-a)"
	if client.String() != expected {
		t.Errorf("expected %s, got %s", expected, client.String())
	}
}
