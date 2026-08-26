// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"fmt"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// AzureProvider implements the Provider interface for Microsoft Azure
type AzureProvider struct {
	client *Client
	config providers.ProviderConfig
	logger logger.Logger
}

// NewProvider creates a new Azure provider instance (factory function)
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	return &AzureProvider{
		config: cfg,
		logger: log,
	}, nil
}

// Name returns the provider name
func (p *AzureProvider) Name() string {
	return "Microsoft Azure"
}

// Type returns the provider type
func (p *AzureProvider) Type() providers.ProviderType {
	return providers.ProviderAzure
}

// Connect establishes a connection to Azure
func (p *AzureProvider) Connect(ctx context.Context, config providers.ProviderConfig) error {
	// Extract Azure-specific config from metadata
	subscriptionID, _ := config.Metadata["subscription_id"].(string)
	tenantID, _ := config.Metadata["tenant_id"].(string)
	clientID, _ := config.Metadata["client_id"].(string)
	clientSecret, _ := config.Metadata["client_secret"].(string)
	resourceGroup, _ := config.Metadata["resource_group"].(string)
	location, _ := config.Metadata["location"].(string)
	storageAccount, _ := config.Metadata["storage_account"].(string)
	container, _ := config.Metadata["container"].(string)
	containerURL, _ := config.Metadata["container_url"].(string)
	exportFormat, _ := config.Metadata["export_format"].(string)

	// Use Username/Password fields if client_id/secret not in metadata
	if clientID == "" {
		clientID = config.Username
	}
	if clientSecret == "" {
		clientSecret = config.Password
	}

	// Defaults
	if location == "" {
		location = "eastus"
	}
	if exportFormat == "" {
		exportFormat = "image"
	}

	// Create Azure client config
	azureConfig := &Config{
		SubscriptionID: subscriptionID,
		TenantID:       tenantID,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		ResourceGroup:  resourceGroup,
		Location:       location,
		StorageAccount: storageAccount,
		Container:      container,
		ContainerURL:   containerURL,
		ExportFormat:   exportFormat,
	}

	// Create Azure client
	client, err := NewClient(azureConfig, p.logger)
	if err != nil {
		return fmt.Errorf("failed to connect to Azure: %w", err)
	}

	p.client = client
	p.config = config
	return nil
}

// Disconnect closes the Azure connection
func (p *AzureProvider) Disconnect() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// ValidateCredentials validates Azure credentials
func (p *AzureProvider) ValidateCredentials(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("not connected")
	}

	return p.client.ValidateCredentials(ctx)
}

// ListVMs lists Azure VMs matching the filter
func (p *AzureProvider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
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
			Provider: providers.ProviderAzure,
			ID:       vm.ID,
			Name:     vm.Name,
			State:    vm.PowerState,
			Location: vm.Location,
			Tags:     vm.Tags,
			Metadata: map[string]interface{}{
				"vm_size":            vm.VMSize,
				"os_type":            vm.OSType,
				"provisioning_state": vm.ProvisioningState,
				"resource_group":     vm.ResourceGroup,
				"disk_names":         vm.DiskNames,
			},
		}
		result = append(result, vmInfo)
	}

	return result, nil
}

// GetVM retrieves information about a specific Azure VM
func (p *AzureProvider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	vm, err := p.client.GetVM(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("get VM: %w", err)
	}

	return &providers.VMInfo{
		Provider: providers.ProviderAzure,
		ID:       vm.ID,
		Name:     vm.Name,
		State:    vm.PowerState,
		Location: vm.Location,
		Tags:     vm.Tags,
		Metadata: map[string]interface{}{
			"vm_size":            vm.VMSize,
			"os_type":            vm.OSType,
			"provisioning_state": vm.ProvisioningState,
			"resource_group":     vm.ResourceGroup,
			"disk_names":         vm.DiskNames,
		},
	}, nil
}

// SearchVMs searches for Azure VMs by query string
func (p *AzureProvider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
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
			Provider: providers.ProviderAzure,
			ID:       vm.ID,
			Name:     vm.Name,
			State:    vm.PowerState,
			Location: vm.Location,
			Tags:     vm.Tags,
			Metadata: map[string]interface{}{
				"vm_size":        vm.VMSize,
				"os_type":        vm.OSType,
				"resource_group": vm.ResourceGroup,
			},
		}
		result = append(result, vmInfo)
	}

	return result, nil
}

// ExportVM exports an Azure VM to VHD or managed image format
func (p *AzureProvider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Update client config with export options
	containerURL, _ := opts.Metadata["container_url"].(string)
	if containerURL != "" {
		p.client.config.ContainerURL = containerURL
	}

	exportFormat, _ := opts.Metadata["export_format"].(string)
	if exportFormat != "" {
		p.client.config.ExportFormat = exportFormat
	}

	// Export VM to VHD if format is "vhd"
	if p.client.config.ExportFormat == "vhd" && p.client.config.ContainerURL != "" {
		vhdResults, err := p.client.ExportVMToVHD(ctx, identifier, p.client.config.ContainerURL, opts.OutputPath, nil)
		if err != nil {
			return nil, fmt.Errorf("export VM to VHD: %w", err)
		}

		if len(vhdResults) == 0 {
			return nil, fmt.Errorf("no disks exported")
		}

		// Create manifest
		if err := CreateExportManifest(identifier, vhdResults, opts.OutputPath); err != nil {
			p.logger.Warn("failed to create manifest", "error", err)
		}

		// Return result for first disk (OS disk)
		firstDisk := vhdResults[0]
		return &providers.ExportResult{
			Provider:   providers.ProviderAzure,
			VMID:       identifier,
			Format:     "vhd",
			OutputPath: firstDisk.LocalPath,
			Size:       firstDisk.Size,
			Metadata: map[string]interface{}{
				"disk_count":   len(vhdResults),
				"blob_url":     firstDisk.BlobURL,
				"disk_name":    firstDisk.DiskName,
				"disk_type":    firstDisk.DiskType,
				"disk_size_gb": firstDisk.DiskSizeGB,
			},
		}, nil
	}

	// Fallback: Create managed image
	err := p.client.ExportVM(ctx, identifier, opts.OutputPath, nil)
	if err != nil {
		return nil, fmt.Errorf("export VM: %w", err)
	}

	return &providers.ExportResult{
		Provider:   providers.ProviderAzure,
		VMID:       identifier,
		Format:     "image",
		OutputPath: opts.OutputPath,
		Metadata: map[string]interface{}{
			"note": "Created Azure managed image - set export_format=vhd for VHD export",
		},
	}, nil
}

// GetExportCapabilities returns the export capabilities of Azure
func (p *AzureProvider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{"vhd", "image"},
		SupportsCompression: false, // VHD format doesn't support compression
		SupportsStreaming:   true,  // Can stream to blob storage
		SupportsSnapshots:   true,  // Azure snapshots
		MaxVMSizeGB:         4096,  // Azure max disk size
		SupportedTargets:    []string{"blob", "local"},
	}
}
