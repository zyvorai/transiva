// SPDX-License-Identifier: Apache-2.0

package hyperv

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

func TestHyperVProvider_Name(t *testing.T) {
	log := logger.New("info")
	p := &HyperVProvider{logger: log}

	if name := p.Name(); name != "Microsoft Hyper-V" {
		t.Errorf("Name() = %q, want %q", name, "Microsoft Hyper-V")
	}
}

func TestHyperVProvider_Type(t *testing.T) {
	log := logger.New("info")
	p := &HyperVProvider{logger: log}

	if typ := p.Type(); typ != providers.ProviderHyperV {
		t.Errorf("Type() = %v, want %v", typ, providers.ProviderHyperV)
	}
}

func TestHyperVProvider_NewProvider(t *testing.T) {
	log := logger.New("info")
	cfg := providers.ProviderConfig{
		Type:     providers.ProviderHyperV,
		Username: "admin",
		Password: "password",
		Metadata: map[string]interface{}{
			"host": "hyperv.example.com",
		},
	}

	provider, err := NewProvider(cfg, log)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider() returned nil provider")
	}

	hypervProvider, ok := provider.(*HyperVProvider)
	if !ok {
		t.Fatal("NewProvider() did not return *HyperVProvider")
	}

	if hypervProvider.logger == nil {
		t.Error("Expected logger to be set")
	}

	if hypervProvider.Name() != "Microsoft Hyper-V" {
		t.Error("Expected provider name to be 'Microsoft Hyper-V'")
	}

	if hypervProvider.Type() != providers.ProviderHyperV {
		t.Error("Expected provider type to be ProviderHyperV")
	}
}

func TestHyperVProvider_Disconnect(t *testing.T) {
	log := logger.New("info")

	tests := []struct {
		name         string
		provider     *HyperVProvider
		expectError  bool
	}{
		{
			name: "disconnect with nil client",
			provider: &HyperVProvider{
				logger: log,
				client: nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.Disconnect()

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestHyperVProvider_GetExportCapabilities(t *testing.T) {
	log := logger.New("info")
	p := &HyperVProvider{logger: log}

	caps := p.GetExportCapabilities()

	// Check supported formats
	if len(caps.SupportedFormats) != 3 {
		t.Errorf("Expected 3 supported formats, got %d", len(caps.SupportedFormats))
	}

	expectedFormats := map[string]bool{
		"vhdx":   true,
		"vhd":    true,
		"hyperv": true,
	}

	for _, format := range caps.SupportedFormats {
		if !expectedFormats[format] {
			t.Errorf("Unexpected format: %s", format)
		}
	}

	// Check compression support
	if caps.SupportsCompression {
		t.Error("Expected SupportsCompression to be false")
	}

	// Check streaming support
	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false")
	}

	// Check snapshots support
	if !caps.SupportsSnapshots {
		t.Error("Expected SupportsSnapshots to be true")
	}

	// Check max VM size
	expectedMaxSize := int64(64 * 1024) // 64TB
	if caps.MaxVMSizeGB != expectedMaxSize {
		t.Errorf("Expected MaxVMSizeGB to be %d, got %d", expectedMaxSize, caps.MaxVMSizeGB)
	}

	// Check supported targets
	if len(caps.SupportedTargets) != 2 {
		t.Errorf("Expected 2 supported targets, got %d", len(caps.SupportedTargets))
	}

	expectedTargets := map[string]bool{
		"local": true,
		"smb":   true,
	}

	for _, target := range caps.SupportedTargets {
		if !expectedTargets[target] {
			t.Errorf("Unexpected target: %s", target)
		}
	}
}
