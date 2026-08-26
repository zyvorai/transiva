// SPDX-License-Identifier: Apache-2.0

package proxmox

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

func TestProvider_Name(t *testing.T) {
	log := logger.New("info")
	p := &Provider{
		logger: log,
	}

	name := p.Name()
	expected := "Proxmox VE"
	if name != expected {
		t.Errorf("Name() = %s, want %s", name, expected)
	}
}

func TestProvider_Type(t *testing.T) {
	log := logger.New("info")
	p := &Provider{
		logger: log,
	}

	providerType := p.Type()
	if providerType != providers.ProviderProxmox {
		t.Errorf("Type() = %v, want %v", providerType, providers.ProviderProxmox)
	}
}

func TestProvider_GetExportCapabilities(t *testing.T) {
	log := logger.New("info")
	p := &Provider{
		logger: log,
	}

	caps := p.GetExportCapabilities()

	// Check supported formats
	if len(caps.SupportedFormats) != 2 {
		t.Errorf("Expected 2 supported formats, got %d", len(caps.SupportedFormats))
	}

	expectedFormats := map[string]bool{"vzdump": true, "vma": true}
	for _, format := range caps.SupportedFormats {
		if !expectedFormats[format] {
			t.Errorf("Unexpected format: %s", format)
		}
	}

	// Check compression support
	if !caps.SupportsCompression {
		t.Error("Expected SupportsCompression to be true")
	}

	// Check streaming support
	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false for Proxmox")
	}
}

func TestParseVMIdentifier_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		wantNode   string
		wantVMID   int
		wantErr    bool
	}{
		{
			name:       "large VMID",
			identifier: "999999",
			wantNode:   "",
			wantVMID:   999999,
			wantErr:    false,
		},
		{
			name:       "node with hyphen",
			identifier: "pve-node-1:100",
			wantNode:   "pve-node-1",
			wantVMID:   100,
			wantErr:    false,
		},
		{
			name:       "empty string",
			identifier: "",
			wantErr:    true,
		},
		{
			name:       "only colon",
			identifier: ":",
			wantErr:    true,
		},
		{
			name:       "node without VMID",
			identifier: "pve-node1:",
			wantErr:    true,
		},
		{
			name:       "VMID with leading zeros",
			identifier: "0100",
			wantNode:   "",
			wantVMID:   100,
			wantErr:    false,
		},
		{
			name:       "node with spaces in VMID",
			identifier: "pve-node1: 100",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, vmid, err := parseVMIdentifier(tt.identifier)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseVMIdentifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if node != tt.wantNode {
					t.Errorf("parseVMIdentifier() node = %v, want %v", node, tt.wantNode)
				}
				if vmid != tt.wantVMID {
					t.Errorf("parseVMIdentifier() vmid = %v, want %v", vmid, tt.wantVMID)
				}
			}
		})
	}
}
