// SPDX-License-Identifier: Apache-2.0

// +build !full

// Stub implementation of KubeVirt provider
// This is used by default until KubeVirt dependencies are fully resolved
// To use the full implementation, build with: go build -tags full

package kubevirt

import (
	"context"
	"fmt"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// KubeVirtProviderStub is a stub implementation of the KubeVirt provider
// This will be replaced with the full implementation once dependencies are resolved
type KubeVirtProviderStub struct {
	config    providers.ProviderConfig
	logger    logger.Logger
	namespace string
}

// NewProvider creates a new KubeVirt provider stub instance
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	namespace := "default"
	if ns, ok := cfg.Metadata["namespace"].(string); ok && ns != "" {
		namespace = ns
	}

	return &KubeVirtProviderStub{
		config:    cfg,
		logger:    log,
		namespace: namespace,
	}, nil
}

// Name returns the provider name
func (p *KubeVirtProviderStub) Name() string {
	return "KubeVirt (Stub - Full implementation pending)"
}

// Type returns the provider type
func (p *KubeVirtProviderStub) Type() providers.ProviderType {
	return providers.ProviderKubeVirt
}

// Connect establishes a connection to Kubernetes cluster with KubeVirt
func (p *KubeVirtProviderStub) Connect(ctx context.Context, providerCfg providers.ProviderConfig) error {
	p.logger.Info("KubeVirt provider stub - connection not yet implemented")
	return fmt.Errorf("KubeVirt provider: full implementation pending - dependency resolution required")
}

// Disconnect closes the KubeVirt connection
func (p *KubeVirtProviderStub) Disconnect() error {
	return nil
}

// ValidateCredentials validates the connection and credentials
func (p *KubeVirtProviderStub) ValidateCredentials(ctx context.Context) error {
	return fmt.Errorf("KubeVirt provider: full implementation pending")
}

// ListVMs lists virtual machines matching the filter
func (p *KubeVirtProviderStub) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
	return nil, fmt.Errorf("KubeVirt provider: full implementation pending")
}

// GetVM retrieves information about a specific VM
func (p *KubeVirtProviderStub) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	return nil, fmt.Errorf("KubeVirt provider: full implementation pending")
}

// SearchVMs searches for VMs by query string
func (p *KubeVirtProviderStub) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	return nil, fmt.Errorf("KubeVirt provider: full implementation pending")
}

// ExportVM exports a virtual machine
func (p *KubeVirtProviderStub) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	return nil, fmt.Errorf("KubeVirt provider: full implementation pending")
}

// GetExportCapabilities returns the export capabilities of KubeVirt
func (p *KubeVirtProviderStub) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{},
		SupportsCompression: false,
		SupportsStreaming:   false,
		SupportsSnapshots:   false,
		MaxVMSizeGB:         0,
		SupportedTargets:    []string{},
	}
}
