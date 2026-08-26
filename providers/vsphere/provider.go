// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// VSphereProvider implements the Provider interface for VMware vSphere
type VSphereProvider struct {
	client *VSphereClient
	config providers.ProviderConfig
	logger logger.Logger
}

// NewProvider creates a new vSphere provider instance (factory function)
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	return &VSphereProvider{
		config: cfg,
		logger: log,
	}, nil
}

// Name returns the provider name
func (p *VSphereProvider) Name() string {
	return "VMware vSphere"
}

// Type returns the provider type
func (p *VSphereProvider) Type() providers.ProviderType {
	return providers.ProviderVSphere
}

// Connect establishes a connection to vSphere
func (p *VSphereProvider) Connect(ctx context.Context, providerCfg providers.ProviderConfig) error {
	// Convert generic config to vSphere-specific config
	cfg := &config.Config{
		VCenterURL:    providerCfg.Endpoint,
		Username:      providerCfg.Username,
		Password:      providerCfg.Password,
		Insecure:      providerCfg.Insecure,
		Timeout:       providerCfg.Timeout,
		RetryAttempts: 3,
	}

	client, err := NewVSphereClient(ctx, cfg, p.logger)
	if err != nil {
		return fmt.Errorf("failed to connect to vSphere: %w", err)
	}

	p.client = client
	p.config = providerCfg
	return nil
}

// Disconnect closes the vSphere connection
func (p *VSphereProvider) Disconnect() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// ValidateCredentials validates the connection and credentials
func (p *VSphereProvider) ValidateCredentials(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("not connected")
	}
	// Connection is already validated in Connect()
	return nil
}

// ListVMs lists virtual machines matching the filter
func (p *VSphereProvider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Use finder to list all VMs
	vms, err := p.client.finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list VMs: %w", err)
	}

	result := make([]*providers.VMInfo, 0, len(vms))

	// Get properties for each VM and convert to VMInfo
	for _, vm := range vms {
		var moVM mo.VirtualMachine
		if err := vm.Properties(ctx, vm.Reference(), []string{"config", "runtime", "guest", "storage", "summary"}, &moVM); err != nil {
			p.logger.Warn("failed to get VM properties, skipping", "vm", vm.Name(), "error", err)
			continue
		}

		// Calculate total storage
		var totalStorage int64
		if moVM.Config != nil && moVM.Config.Hardware.Device != nil {
			for _, device := range moVM.Config.Hardware.Device {
				if disk, ok := device.(*types.VirtualDisk); ok {
					totalStorage += disk.CapacityInBytes
				}
			}
		}

		// Get IP addresses from guest info
		var ipAddresses []string
		if moVM.Guest != nil && moVM.Guest.Net != nil {
			for _, nic := range moVM.Guest.Net {
				if nic.IpAddress != nil {
					ipAddresses = append(ipAddresses, nic.IpAddress...)
				}
			}
		}

		// Get tags if available
		tags := make(map[string]string)
		if moVM.Config != nil && moVM.Config.ExtraConfig != nil {
			for _, extra := range moVM.Config.ExtraConfig {
				if optValue, ok := extra.(*types.OptionValue); ok {
					if strVal, ok := optValue.Value.(string); ok {
						tags[optValue.Key] = strVal
					}
				}
			}
		}

		vmInfo := &providers.VMInfo{
			Provider:    providers.ProviderVSphere,
			ID:          vm.Reference().Value,
			Name:        vm.Name(),
			Location:    vm.InventoryPath,
			PowerState:  string(moVM.Runtime.PowerState),
			State:       string(moVM.Runtime.PowerState),
			IPAddresses: ipAddresses,
			Tags:        tags,
		}

		if moVM.Config != nil {
			vmInfo.GuestOS = moVM.Config.GuestFullName
			vmInfo.MemoryMB = int64(moVM.Config.Hardware.MemoryMB)
			vmInfo.NumCPUs = int(moVM.Config.Hardware.NumCPU)
		}

		vmInfo.StorageGB = totalStorage / (1024 * 1024 * 1024)

		// Apply filters
		if !p.matchesFilter(vmInfo, filter) {
			continue
		}

		result = append(result, vmInfo)
	}

	p.logger.Info("listed VMs", "total", len(result))
	return result, nil
}

// matchesFilter checks if a VM matches the given filter
func (p *VSphereProvider) matchesFilter(vm *providers.VMInfo, filter providers.VMFilter) bool {
	// Name pattern filter
	if filter.NamePattern != "" {
		matched, err := filepath.Match(filter.NamePattern, vm.Name)
		if err != nil || !matched {
			return false
		}
	}

	// State filter
	if filter.State != "" && vm.State != filter.State {
		return false
	}

	// PowerState filter (alternative to State)
	if filter.PowerState != "" && vm.PowerState != filter.PowerState {
		return false
	}

	// Location filter
	if filter.Location != "" && !strings.Contains(vm.Location, filter.Location) {
		return false
	}

	// Memory filter
	if filter.MinMemoryMB > 0 && vm.MemoryMB < filter.MinMemoryMB {
		return false
	}

	// CPU filter
	if filter.MinCPUs > 0 && vm.NumCPUs < filter.MinCPUs {
		return false
	}

	// Tags filter
	if len(filter.Tags) > 0 {
		for key, value := range filter.Tags {
			if vmValue, ok := vm.Tags[key]; !ok || vmValue != value {
				return false
			}
		}
	}

	return true
}

// GetVM retrieves information about a specific VM
func (p *VSphereProvider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Get VM using finder
	vm, err := p.client.finder.VirtualMachine(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to find VM: %w", err)
	}

	// Convert to VMInfo
	return &providers.VMInfo{
		Provider: providers.ProviderVSphere,
		ID:       vm.Reference().Value,
		Name:     vm.Name(),
		Location: "vSphere", // Could extract datacenter
		Metadata: map[string]interface{}{
			"path": vm.InventoryPath,
		},
	}, nil
}

// SearchVMs searches for VMs by query string
func (p *VSphereProvider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Use finder to search with wildcard
	searchPattern := "*" + query + "*"
	vms, err := p.client.finder.VirtualMachineList(ctx, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search VMs: %w", err)
	}

	results := make([]*providers.VMInfo, 0, len(vms))
	for _, vm := range vms {
		results = append(results, &providers.VMInfo{
			Provider: providers.ProviderVSphere,
			ID:       vm.Reference().Value,
			Name:     vm.Name(),
			Location: "vSphere",
			Metadata: map[string]interface{}{
				"path": vm.InventoryPath,
			},
		})
	}

	return results, nil
}

// ExportVM exports a virtual machine
func (p *VSphereProvider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Convert generic export options to vSphere-specific options
	exportOpts := ExportOptions{
		Format:              opts.Format,
		OutputPath:          opts.OutputPath,
		RemoveCDROM:         opts.RemoveCDROM,
		Compress:            opts.Compress,
		CompressionLevel:    opts.CompressionLevel,
		ParallelDownloads:   3,
		ShowOverallProgress: true,
	}

	// Set defaults
	if exportOpts.Format == "" {
		exportOpts.Format = "ovf"
	}

	// Perform export
	result, err := p.client.ExportOVF(ctx, identifier, exportOpts)
	if err != nil {
		return nil, fmt.Errorf("export failed: %w", err)
	}

	// Convert to generic export result
	outputPath := result.OVFPath
	if result.Format == "ova" && result.OVAPath != "" {
		outputPath = result.OVAPath
	}

	return &providers.ExportResult{
		Provider:   providers.ProviderVSphere,
		VMName:     filepath.Base(strings.TrimSuffix(result.OVFPath, ".ovf")),
		Format:     result.Format,
		OutputPath: outputPath,
		Files:      result.Files,
		Size:       result.TotalSize,
		Duration:   result.Duration,
		Metadata: map[string]interface{}{
			"ovf_path": result.OVFPath,
			"ova_path": result.OVAPath,
		},
	}, nil
}

// GetExportCapabilities returns the export capabilities of vSphere
func (p *VSphereProvider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{"ovf", "ova"},
		SupportsCompression: true,
		SupportsStreaming:   false,
		SupportsSnapshots:   false,
		MaxVMSizeGB:         0, // No limit
		SupportedTargets:    []string{"local"},
	}
}
