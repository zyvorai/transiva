// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

func TestGCPProvider_Name(t *testing.T) {
	log := logger.New("info")
	p := &GCPProvider{
		logger: log,
	}

	name := p.Name()
	expected := "Google Cloud Platform"
	if name != expected {
		t.Errorf("Name() = %s, want %s", name, expected)
	}
}

func TestGCPProvider_Type(t *testing.T) {
	log := logger.New("info")
	p := &GCPProvider{
		logger: log,
	}

	providerType := p.Type()
	if providerType != providers.ProviderGCP {
		t.Errorf("Type() = %v, want %v", providerType, providers.ProviderGCP)
	}
}

func TestGCPProvider_GetExportCapabilities(t *testing.T) {
	log := logger.New("info")
	p := &GCPProvider{
		logger: log,
	}

	caps := p.GetExportCapabilities()

	// Check supported formats
	if len(caps.SupportedFormats) != 2 {
		t.Errorf("Expected 2 supported formats, got %d", len(caps.SupportedFormats))
	}

	expectedFormats := map[string]bool{"vmdk": true, "image": true}
	for _, format := range caps.SupportedFormats {
		if !expectedFormats[format] {
			t.Errorf("Unexpected format: %s", format)
		}
	}

	// Check compression support
	if caps.SupportsCompression {
		t.Error("Expected SupportsCompression to be false for GCP (GCS handles compression)")
	}

	// Check streaming support
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true for GCP")
	}

	// Check snapshot support
	if !caps.SupportsSnapshots {
		t.Error("Expected SupportsSnapshots to be true for GCP")
	}

	// Check max VM size
	if caps.MaxVMSizeGB != 65536 {
		t.Errorf("Expected MaxVMSizeGB 65536, got %d", caps.MaxVMSizeGB)
	}

	// Check supported targets
	if len(caps.SupportedTargets) != 2 {
		t.Errorf("Expected 2 supported targets, got %d", len(caps.SupportedTargets))
	}

	expectedTargets := map[string]bool{"gcs": true, "local": true}
	for _, target := range caps.SupportedTargets {
		if !expectedTargets[target] {
			t.Errorf("Unexpected target: %s", target)
		}
	}
}

func TestNewProvider(t *testing.T) {
	log := logger.New("info")
	cfg := providers.ProviderConfig{
		Metadata: map[string]interface{}{
			"project_id": "test-project",
			"zone":       "us-central1-a",
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
	if provider.Type() != providers.ProviderGCP {
		t.Errorf("Expected provider type GCP, got %v", provider.Type())
	}

	// Verify provider name
	expectedName := "Google Cloud Platform"
	if provider.Name() != expectedName {
		t.Errorf("Expected provider name '%s', got '%s'", expectedName, provider.Name())
	}
}

func TestGCPProvider_Disconnect(t *testing.T) {
	log := logger.New("info")
	p := &GCPProvider{
		logger: log,
	}

	// Disconnect should not error (GCP SDK handles cleanup automatically)
	err := p.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() error = %v, want nil", err)
	}
}
