// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

func TestAWSProvider_Name(t *testing.T) {
	log := logger.New("info")
	p := &AWSProvider{
		logger: log,
	}

	name := p.Name()
	expected := "Amazon Web Services EC2"
	if name != expected {
		t.Errorf("Name() = %s, want %s", name, expected)
	}
}

func TestAWSProvider_Type(t *testing.T) {
	log := logger.New("info")
	p := &AWSProvider{
		logger: log,
	}

	providerType := p.Type()
	if providerType != providers.ProviderAWS {
		t.Errorf("Type() = %v, want %v", providerType, providers.ProviderAWS)
	}
}

func TestAWSProvider_GetExportCapabilities(t *testing.T) {
	log := logger.New("info")
	p := &AWSProvider{
		logger: log,
	}

	caps := p.GetExportCapabilities()

	// Check supported formats
	if len(caps.SupportedFormats) != 3 {
		t.Errorf("Expected 3 supported formats, got %d", len(caps.SupportedFormats))
	}

	expectedFormats := map[string]bool{"vmdk": true, "vhd": true, "raw": true}
	for _, format := range caps.SupportedFormats {
		if !expectedFormats[format] {
			t.Errorf("Unexpected format: %s", format)
		}
	}

	// Check compression support
	if caps.SupportsCompression {
		t.Error("Expected SupportsCompression to be false for AWS (S3 handles compression)")
	}

	// Check streaming support
	if !caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be true for AWS")
	}

	// Check snapshot support
	if !caps.SupportsSnapshots {
		t.Error("Expected SupportsSnapshots to be true for AWS")
	}

	// Check max VM size
	if caps.MaxVMSizeGB != 1000 {
		t.Errorf("Expected MaxVMSizeGB 1000, got %d", caps.MaxVMSizeGB)
	}

	// Check supported targets
	if len(caps.SupportedTargets) != 2 {
		t.Errorf("Expected 2 supported targets, got %d", len(caps.SupportedTargets))
	}

	expectedTargets := map[string]bool{"s3": true, "local": true}
	for _, target := range caps.SupportedTargets {
		if !expectedTargets[target] {
			t.Errorf("Unexpected target: %s", target)
		}
	}
}

func TestNewProvider(t *testing.T) {
	log := logger.New("info")
	cfg := providers.ProviderConfig{
		Host:     "us-east-1.amazonaws.com",
		Username: "test-access-key",
		Password: "test-secret-key",
		Metadata: map[string]interface{}{
			"region": "us-east-1",
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
	if provider.Type() != providers.ProviderAWS {
		t.Errorf("Expected provider type AWS, got %v", provider.Type())
	}

	// Verify provider name
	expectedName := "Amazon Web Services EC2"
	if provider.Name() != expectedName {
		t.Errorf("Expected provider name '%s', got '%s'", expectedName, provider.Name())
	}
}

func TestAWSProvider_Disconnect(t *testing.T) {
	log := logger.New("info")
	p := &AWSProvider{
		logger: log,
	}

	// Disconnect should not error (AWS SDK handles cleanup automatically)
	err := p.Disconnect()
	if err != nil {
		t.Errorf("Disconnect() error = %v, want nil", err)
	}
}
