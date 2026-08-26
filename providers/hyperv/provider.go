// SPDX-License-Identifier: Apache-2.0

package hyperv

import (
	"context"
	"fmt"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// HyperVProvider implements the Provider interface for Microsoft Hyper-V
type HyperVProvider struct {
	client *Client
	config providers.ProviderConfig
	logger logger.Logger
}

// NewProvider creates a new Hyper-V provider instance (factory function)
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	return &HyperVProvider{
		config: cfg,
		logger: log,
	}, nil
}

// Name returns the provider name
func (p *HyperVProvider) Name() string {
	return "Microsoft Hyper-V"
}

// Type returns the provider type
func (p *HyperVProvider) Type() providers.ProviderType {
	return providers.ProviderHyperV
}

// Connect establishes a connection to Hyper-V
func (p *HyperVProvider) Connect(ctx context.Context, config providers.ProviderConfig) error {
	// Extract Hyper-V-specific config from metadata
	host, _ := config.Metadata["host"].(string)
	username, _ := config.Metadata["username"].(string)
	password, _ := config.Metadata["password"].(string)
	useWinRM, _ := config.Metadata["use_winrm"].(bool)
	useHTTPS, _ := config.Metadata["use_https"].(bool)

	winrmPort := 5985
	if portVal, ok := config.Metadata["winrm_port"].(float64); ok {
		winrmPort = int(portVal)
	} else if portVal, ok := config.Metadata["winrm_port"].(int); ok {
		winrmPort = portVal
	}

	// Use Username/Password fields if not in metadata
	if username == "" {
		username = config.Username
	}
	if password == "" {
		password = config.Password
	}

	// Auto-enable WinRM if host is specified
	if host != "" && !useWinRM {
		useWinRM = true
	}

	// Create Hyper-V client config
	hypervConfig := &Config{
		Host:      host,
		Username:  username,
		Password:  password,
		UseWinRM:  useWinRM,
		WinRMPort: winrmPort,
		UseHTTPS:  useHTTPS,
	}

	// Create Hyper-V client
	client, err := NewClient(hypervConfig, p.logger)
	if err != nil {
		return fmt.Errorf("failed to connect to Hyper-V: %w", err)
	}

	p.client = client
	p.config = config
	return nil
}

// Disconnect closes the Hyper-V connection
func (p *HyperVProvider) Disconnect() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// ValidateCredentials validates Hyper-V credentials
func (p *HyperVProvider) ValidateCredentials(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("not connected")
	}

	// Try to list VMs as validation
	_, err := p.client.ListVMs(ctx)
	return err
}

// ListVMs lists Hyper-V VMs matching the filter
func (p *HyperVProvider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	vms, err := p.client.ListVMs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list VMs: %w", err)
	}

	result := make([]*providers.VMInfo, 0, len(vms))
	for _, vm := range vms {
		vmInfo := &providers.VMInfo{
			Provider: providers.ProviderHyperV,
			ID:       vm.ID,
			Name:     vm.Name,
			State:    vm.State,
			Location: p.client.config.Host,
			Tags:     make(map[string]string),
			Metadata: map[string]interface{}{
				"generation":      vm.Generation,
				"cpu_usage":       vm.CPUUsage,
				"memory_assigned": vm.MemoryAssigned,
				"memory_demand":   vm.MemoryDemand,
				"status":          vm.Status,
				"uptime":          vm.Uptime,
				"vhd_paths":       vm.VHDPath,
				"path":            vm.Path,
			},
		}

		// Add notes as tags if present
		if vm.Notes != "" {
			vmInfo.Tags["notes"] = vm.Notes
		}

		result = append(result, vmInfo)
	}

	return result, nil
}

// GetVM retrieves information about a specific Hyper-V VM
func (p *HyperVProvider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	vm, err := p.client.GetVM(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("get VM: %w", err)
	}

	return &providers.VMInfo{
		Provider: providers.ProviderHyperV,
		ID:       vm.ID,
		Name:     vm.Name,
		State:    vm.State,
		Location: p.client.config.Host,
		Tags:     make(map[string]string),
		Metadata: map[string]interface{}{
			"generation":      vm.Generation,
			"cpu_usage":       vm.CPUUsage,
			"memory_assigned": vm.MemoryAssigned,
			"status":          vm.Status,
			"vhd_paths":       vm.VHDPath,
			"path":            vm.Path,
		},
	}, nil
}

// SearchVMs searches for Hyper-V VMs by query string
func (p *HyperVProvider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	vms, err := p.client.SearchVMs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search VMs: %w", err)
	}

	result := make([]*providers.VMInfo, 0, len(vms))
	for _, vm := range vms {
		vmInfo := &providers.VMInfo{
			Provider: providers.ProviderHyperV,
			ID:       vm.ID,
			Name:     vm.Name,
			State:    vm.State,
			Location: p.client.config.Host,
			Metadata: map[string]interface{}{
				"generation": vm.Generation,
				"status":     vm.Status,
			},
		}
		result = append(result, vmInfo)
	}

	return result, nil
}

// ExportVM exports a Hyper-V VM to VHDX format
func (p *HyperVProvider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Check export format preference
	exportFormat, _ := opts.Metadata["export_format"].(string)

	if exportFormat == "vhdx" || exportFormat == "vhd" {
		// Export VHD files only
		vhdPaths, err := p.client.ExportVHD(ctx, identifier, opts.OutputPath, nil)
		if err != nil {
			return nil, fmt.Errorf("export VHDs: %w", err)
		}

		// Get total size
		var totalSize int64
		for range vhdPaths {
			// Would need to get file size here
			totalSize += 0 // Placeholder
		}

		return &providers.ExportResult{
			Provider:   providers.ProviderHyperV,
			VMID:       identifier,
			Format:     "vhdx",
			OutputPath: vhdPaths[0], // First VHD
			Size:       totalSize,
			Metadata: map[string]interface{}{
				"vhd_count": len(vhdPaths),
				"vhd_paths": vhdPaths,
			},
		}, nil
	}

	// Full VM export (default)
	err := p.client.ExportVM(ctx, identifier, opts.OutputPath, nil)
	if err != nil {
		return nil, fmt.Errorf("export VM: %w", err)
	}

	return &providers.ExportResult{
		Provider:   providers.ProviderHyperV,
		VMID:       identifier,
		Format:     "hyperv",
		OutputPath: opts.OutputPath,
		Metadata: map[string]interface{}{
			"note": "Full Hyper-V VM export - includes configuration and VHDs",
		},
	}, nil
}

// GetExportCapabilities returns the export capabilities of Hyper-V
func (p *HyperVProvider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{"vhdx", "vhd", "hyperv"},
		SupportsCompression: false,     // VHDX supports internal compression
		SupportsStreaming:   false,     // File-based export
		SupportsSnapshots:   true,      // Hyper-V checkpoints
		MaxVMSizeGB:         64 * 1024, // 64TB max VHDX size
		SupportedTargets:    []string{"local", "smb"},
	}
}
