// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// Provider implements providers.Provider for Nutanix AHV via Prism Central v4 API.
type Provider struct {
	client   *Client
	logger   logger.Logger
	cfg      providers.ProviderConfig
	detailed bool
}

// NewProvider creates a Nutanix provider instance.
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	nutanixCfg := &config.NutanixConfig{
		Host:          firstNonEmpty(cfg.Host, cfg.Endpoint),
		Port:          cfg.Port,
		Username:      cfg.Username,
		Password:      cfg.Password,
		VerifySSL:     !cfg.Insecure,
		Cluster:       cfg.Region,
		PageSize:      500,
		DetailWorkers: 10,
		Detailed:      true,
	}

	if cfg.Metadata != nil {
		if v, ok := cfg.Metadata["cluster"].(string); ok && v != "" {
			nutanixCfg.Cluster = v
		}
		if v, ok := cfg.Metadata["detailed"].(bool); ok {
			nutanixCfg.Detailed = v
		}
		if v, ok := cfg.Metadata["page_size"].(int); ok && v > 0 {
			nutanixCfg.PageSize = v
		}
		if v, ok := cfg.Metadata["detail_workers"].(int); ok && v > 0 {
			nutanixCfg.DetailWorkers = v
		}
	}

	if nutanixCfg.Port == 0 {
		nutanixCfg.Port = 9440
	}

	clientCfg := ClientConfigFromHypersdk(nutanixCfg)
	client, err := NewClient(clientCfg, log)
	if err != nil {
		return nil, err
	}

	return &Provider{
		client:   client,
		logger:   log,
		cfg:      cfg,
		detailed: nutanixCfg.Detailed,
	}, nil
}

// Name returns the provider display name.
func (p *Provider) Name() string {
	return "Nutanix AHV"
}

// Type returns the provider type identifier.
func (p *Provider) Type() providers.ProviderType {
	return providers.ProviderNutanix
}

// Connect validates that the client is configured.
func (p *Provider) Connect(ctx context.Context, providerCfg providers.ProviderConfig) error {
	return p.ValidateCredentials(ctx)
}

// Disconnect is a no-op for the HTTP API client.
func (p *Provider) Disconnect() error {
	return nil
}

// ValidateCredentials verifies Prism credentials by listing one VM.
func (p *Provider) ValidateCredentials(ctx context.Context) error {
	page := 0
	limit := 1
	_, err := p.client.vmAPI.ListVms(&page, &limit, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("invalid Nutanix credentials: %w", err)
	}
	return nil
}

// ListVMs lists VMs from Prism Central.
func (p *Provider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
	detailed := p.detailed
	if filter.Location != "" {
		p.client.cfg.ClusterFilter = filter.Location
	}

	inventory, err := p.client.ListVMs(ctx, detailed)
	if err != nil {
		return nil, err
	}

	out := make([]*providers.VMInfo, 0, len(inventory))
	for _, inv := range inventory {
		vmInfo := inventoryToVMInfo(inv)
		if !matchesFilter(vmInfo, filter) {
			continue
		}
		out = append(out, vmInfo)
	}

	if p.logger != nil {
		p.logger.Info("listed Nutanix VMs", "total", len(out))
	}
	return out, nil
}

// GetVM retrieves a single VM by extId.
func (p *Provider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	inv, err := p.client.GetVM(ctx, identifier)
	if err != nil {
		return nil, err
	}
	vmInfo := inventoryToVMInfo(*inv)
	return vmInfo, nil
}

// SearchVMs searches VMs by name or UUID substring.
func (p *Provider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	all, err := p.ListVMs(ctx, providers.VMFilter{})
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	var results []*providers.VMInfo
	for _, vm := range all {
		if strings.Contains(strings.ToLower(vm.Name), queryLower) ||
			strings.Contains(strings.ToLower(vm.ID), queryLower) {
			results = append(results, vm)
		}
	}
	return results, nil
}

// GetClient exposes the underlying Nutanix API client for advanced operations.
func (p *Provider) GetClient() *Client {
	return p.client
}

// ResolveContainerNames returns storage container UUID to name mapping.
func (p *Provider) ResolveContainerNames(ctx context.Context) (map[string]string, error) {
	return p.client.ResolveContainerNames(ctx)
}

func inventoryToVMInfo(inv VMInventory) *providers.VMInfo {
	state := strings.ToLower(inv.PowerState)
	switch inv.PowerState {
	case "ON":
		state = "running"
	case "OFF":
		state = "stopped"
	case "PAUSED":
		state = "suspended"
	}

	storageGB := int64(inv.TotalDiskGiB)
	if inv.TotalDiskGiB > 0 {
		storageGB = int64(inv.TotalDiskGiB + 0.5)
	}

	return &providers.VMInfo{
		Provider:   providers.ProviderNutanix,
		ID:         inv.UUID,
		Name:       inv.Name,
		State:      state,
		PowerState: inv.PowerState,
		Location:   inv.ClusterUUID,
		MemoryMB:   int64(inv.MemoryGiB * 1024),
		NumCPUs:    inv.VCPUs,
		StorageGB:  storageGB,
		Tags: map[string]string{
			"cluster_uuid": inv.ClusterUUID,
			"cluster_name": inv.ClusterName,
		},
		Metadata: map[string]interface{}{
			"cluster_uuid":   inv.ClusterUUID,
			"cluster_name":   inv.ClusterName,
			"disk_count":     inv.DiskCount,
			"total_disk_gib": inv.TotalDiskGiB,
			"nic_count":      inv.NICCount,
			"disks":          inv.Disks,
		},
	}
}

func matchesFilter(vm *providers.VMInfo, filter providers.VMFilter) bool {
	if filter.NamePattern != "" {
		matched, _ := filepath.Match(filter.NamePattern, vm.Name)
		if !matched && !strings.Contains(strings.ToLower(vm.Name), strings.ToLower(filter.NamePattern)) {
			return false
		}
	}

	state := filter.State
	if state == "" {
		state = filter.PowerState
	}
	if state != "" && !strings.EqualFold(vm.State, state) && !strings.EqualFold(vm.PowerState, state) {
		return false
	}

	if filter.MinMemoryMB > 0 && vm.MemoryMB < filter.MinMemoryMB {
		return false
	}
	if filter.MinCPUs > 0 && vm.NumCPUs < filter.MinCPUs {
		return false
	}

	return true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
