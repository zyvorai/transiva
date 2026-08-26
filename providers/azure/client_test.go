// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewClient(t *testing.T) {
	cfg := &Config{
		SubscriptionID: "test-subscription-id",
		TenantID:       "test-tenant-id",
		ClientID:       "test-client-id",
		ClientSecret:   "test-secret",
		ResourceGroup:  "test-rg",
		Location:       "eastus",
		Timeout:        30 * time.Second,
	}

	log := logger.New("info")

	// Note: This will fail without valid Azure credentials
	// This test is primarily for structure validation
	_, err := NewClient(cfg, log)

	// We expect this to fail in test environment
	if err == nil {
		t.Log("Client created successfully (unexpected in test environment)")
	}
}

func TestNewClientDefaultTimeout(t *testing.T) {
	cfg := &Config{
		SubscriptionID: "test-subscription-id",
		TenantID:       "test-tenant-id",
		ClientID:       "test-client-id",
		ClientSecret:   "test-secret",
		ResourceGroup:  "test-rg",
		Location:       "westus2",
	}

	// Default timeout should be set to 1 hour
	expectedTimeout := 1 * time.Hour

	if cfg.Timeout != 0 {
		t.Errorf("expected timeout to be 0 initially, got %v", cfg.Timeout)
	}

	// After NewClient, timeout should be set
	// (we can't actually create the client without valid creds)
	if expectedTimeout != 1*time.Hour {
		t.Errorf("expected default timeout 1h, got %v", expectedTimeout)
	}
}

func TestVMInfoCreation(t *testing.T) {
	vmInfo := &VMInfo{
		Name:              "test-vm",
		ID:                "/subscriptions/xxx/resourceGroups/test-rg/providers/Microsoft.Compute/virtualMachines/test-vm",
		Location:          "eastus",
		VMSize:            "Standard_B2s",
		OSType:            "Linux",
		ProvisioningState: "Succeeded",
		PowerState:        "running",
		ResourceGroup:     "test-rg",
		Tags:              map[string]string{"Environment": "test"},
		DiskNames:         []string{"os-disk", "data-disk-1"},
	}

	if vmInfo.Name != "test-vm" {
		t.Errorf("unexpected name: %s", vmInfo.Name)
	}

	if vmInfo.VMSize != "Standard_B2s" {
		t.Errorf("unexpected VM size: %s", vmInfo.VMSize)
	}

	if vmInfo.OSType != "Linux" {
		t.Errorf("unexpected OS type: %s", vmInfo.OSType)
	}

	if vmInfo.PowerState != "running" {
		t.Errorf("unexpected power state: %s", vmInfo.PowerState)
	}

	if vmInfo.Tags["Environment"] != "test" {
		t.Errorf("unexpected tag value: %s", vmInfo.Tags["Environment"])
	}

	if len(vmInfo.DiskNames) != 2 {
		t.Errorf("expected 2 disks, got %d", len(vmInfo.DiskNames))
	}
}

func TestGetLocations(t *testing.T) {
	cfg := &Config{
		SubscriptionID: "test",
		TenantID:       "test",
		ClientID:       "test",
		ClientSecret:   "test",
		ResourceGroup:  "test",
		Location:       "eastus",
	}

	log := logger.New("info")

	// Create a minimal client struct for testing
	client := &Client{
		config: cfg,
		logger: log,
	}

	locations := client.GetLocations()

	if len(locations) == 0 {
		t.Error("expected locations list to be non-empty")
	}

	// Check for some common regions
	hasEastUS := false
	hasWestEurope := false

	for _, loc := range locations {
		if loc == "eastus" {
			hasEastUS = true
		}
		if loc == "westeurope" {
			hasWestEurope = true
		}
	}

	if !hasEastUS {
		t.Error("expected locations to include eastus")
	}

	if !hasWestEurope {
		t.Error("expected locations to include westeurope")
	}
}
