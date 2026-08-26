// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

func TestAzureProvider_Name(t *testing.T) {
	log := logger.New("info")
	p := &AzureProvider{
		logger: log,
	}

	name := p.Name()
	expected := "Microsoft Azure"
	if name != expected {
		t.Errorf("Name() = %s, want %s", name, expected)
	}
}

func TestAzureProvider_Type(t *testing.T) {
	log := logger.New("info")
	p := &AzureProvider{
		logger: log,
	}

	providerType := p.Type()
	if providerType != providers.ProviderAzure {
		t.Errorf("Type() = %v, want %v", providerType, providers.ProviderAzure)
	}
}

func TestAzureProvider_GetExportCapabilities(t *testing.T) {
	log := logger.New("info")
	p := &AzureProvider{
		logger: log,
	}

	caps := p.GetExportCapabilities()

	// Check supported formats
	if len(caps.SupportedFormats) != 2 {
		t.Errorf("Expected 2 supported formats, got %d", len(caps.SupportedFormats))
	}

	expectedFormats := map[string]bool{"vhd": true, "image": true}
	for _, format := range caps.SupportedFormats {
		if !expectedFormats[format] {
			t.Errorf("Unexpected format: %s", format)
		}
	}

	// Check compression support
	if caps.SupportsCompression {
		t.Error("Expected SupportsCompression to be false for Azure (VHD format doesn't support compression)")
	}

	// Check streaming support
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true for Azure")
	}

	// Check snapshot support
	if !caps.SupportsSnapshots {
		t.Error("Expected SupportsSnapshots to be true for Azure")
	}

	// Check max VM size
	if caps.MaxVMSizeGB != 4096 {
		t.Errorf("Expected MaxVMSizeGB 4096, got %d", caps.MaxVMSizeGB)
	}

	// Check supported targets
	if len(caps.SupportedTargets) != 2 {
		t.Errorf("Expected 2 supported targets, got %d", len(caps.SupportedTargets))
	}

	expectedTargets := map[string]bool{"blob": true, "local": true}
	for _, target := range caps.SupportedTargets {
		if !expectedTargets[target] {
			t.Errorf("Unexpected target: %s", target)
		}
	}
}

func TestNewProvider(t *testing.T) {
	log := logger.New("info")
	cfg := providers.ProviderConfig{
		Username: "test-client-id",
		Password: "test-client-secret",
		Metadata: map[string]interface{}{
			"subscription_id": "test-subscription",
			"tenant_id":       "test-tenant",
			"location":        "eastus",
		},
	}

	provider, err := NewProvider(cfg, log)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider() returned nil provider")
	}

	// Verify provider type
	if provider.Type() != providers.ProviderAzure {
		t.Errorf("Expected provider type Azure, got %v", provider.Type())
	}

	// Verify provider name
	expectedName := "Microsoft Azure"
	if provider.Name() != expectedName {
		t.Errorf("Expected provider name '%s', got '%s'", expectedName, provider.Name())
	}
}

func TestAzureProvider_Disconnect(t *testing.T) {
	log := logger.New("info")
	p := &AzureProvider{
		logger: log,
	}

	// Disconnect should not error (Azure SDK handles cleanup automatically)
	err := p.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() error = %v, want nil", err)
	}
}
