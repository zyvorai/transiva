// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/progress"
)

// Config holds Azure provider configuration
type Config struct {
	SubscriptionID string
	TenantID       string
	ClientID       string
	ClientSecret   string
	ResourceGroup  string
	Location       string
	StorageAccount string // Storage account name for blob storage
	Container      string // Container name for VHD exports
	ContainerURL   string // Full container URL
	ExportFormat   string // "image" or "vhd"
	Timeout        time.Duration
}

// Client represents an Azure Compute client for VM operations
type Client struct {
	vmClient       *armcompute.VirtualMachinesClient
	imageClient    *armcompute.ImagesClient
	diskClient     *armcompute.DisksClient
	snapshotClient *armcompute.SnapshotsClient
	nicClient      *armnetwork.InterfacesClient
	config         *Config
	logger         logger.Logger
}

// VMInfo represents Azure VM information
type VMInfo struct {
	Name              string
	ID                string
	Location          string
	VMSize            string
	OSType            string
	ProvisioningState string
	PowerState        string
	PrivateIP         string
	PublicIP          string
	ResourceGroup     string
	Tags              map[string]string
	DiskNames         []string
	Created           time.Time
}

// NewClient creates a new Azure Compute client
func NewClient(cfg *Config, log logger.Logger) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 1 * time.Hour
	}

	// Create credential
	cred, err := azidentity.NewClientSecretCredential(
		cfg.TenantID,
		cfg.ClientID,
		cfg.ClientSecret,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	// Create VM client
	vmClient, err := armcompute.NewVirtualMachinesClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM client: %w", err)
	}

	// Create image client
	imageClient, err := armcompute.NewImagesClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image client: %w", err)
	}

	// Create disk client
	diskClient, err := armcompute.NewDisksClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create disk client: %w", err)
	}

	// Create snapshot client
	snapshotClient, err := armcompute.NewSnapshotsClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot client: %w", err)
	}

	// Create network interface client
	nicClient, err := armnetwork.NewInterfacesClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface client: %w", err)
	}

	return &Client{
		vmClient:       vmClient,
		imageClient:    imageClient,
		diskClient:     diskClient,
		snapshotClient: snapshotClient,
		nicClient:      nicClient,
		config:         cfg,
		logger:         log,
	}, nil
}

// ListVMs returns a list of all VMs in the resource group
func (c *Client) ListVMs(ctx context.Context) ([]*VMInfo, error) {
	pager := c.vmClient.NewListPager(c.config.ResourceGroup, nil)

	var vms []*VMInfo
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list VMs: %w", err)
		}

		for _, vm := range page.Value {
			vmInfo := c.vmToVMInfo(vm)
			vms = append(vms, vmInfo)
		}
	}

	c.logger.Info("discovered Azure VMs", "count", len(vms))
	return vms, nil
}

// GetVM retrieves information about a specific VM
func (c *Client) GetVM(ctx context.Context, vmName string) (*VMInfo, error) {
	result, err := c.vmClient.Get(ctx, c.config.ResourceGroup, vmName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM %s: %w", vmName, err)
	}

	return c.vmToVMInfo(&result.VirtualMachine), nil
}

// ExportVM creates a managed image from an Azure VM
func (c *Client) ExportVM(ctx context.Context, vmName, outputPath string, reporter progress.ProgressReporter) error {
	c.logger.Info("starting Azure VM export", "vmName", vmName)

	// Get VM info
	vmInfo, err := c.GetVM(ctx, vmName)
	if err != nil {
		return err
	}

	if reporter != nil {
		reporter.Describe("Deallocating VM")
	}

	// Deallocate VM (required for creating image)
	pollerResponse, err := c.vmClient.BeginDeallocate(ctx, c.config.ResourceGroup, vmName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin VM deallocation: %w", err)
	}

	_, err = pollerResponse.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to deallocate VM: %w", err)
	}

	c.logger.Info("VM deallocated", "vmName", vmName)

	if reporter != nil {
		reporter.Describe("Generalizing VM")
	}

	// Generalize VM
	_, err = c.vmClient.Generalize(ctx, c.config.ResourceGroup, vmName, nil)
	if err != nil {
		return fmt.Errorf("failed to generalize VM: %w", err)
	}

	c.logger.Info("VM generalized", "vmName", vmName)

	// Create image
	imageName := fmt.Sprintf("%s-image-%d", vmName, time.Now().Unix())

	if reporter != nil {
		reporter.Describe("Creating managed image")
	}

	// Get VM to use as source
	vmResult, err := c.vmClient.Get(ctx, c.config.ResourceGroup, vmName, nil)
	if err != nil {
		return fmt.Errorf("failed to get VM for image creation: %w", err)
	}

	imageParams := armcompute.Image{
		Location: to.Ptr(c.config.Location),
		Properties: &armcompute.ImageProperties{
			SourceVirtualMachine: &armcompute.SubResource{
				ID: vmResult.ID,
			},
		},
	}

	pollerCreateImage, err := c.imageClient.BeginCreateOrUpdate(
		ctx,
		c.config.ResourceGroup,
		imageName,
		imageParams,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to begin image creation: %w", err)
	}

	imageResult, err := pollerCreateImage.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	c.logger.Info("managed image created", "imageName", imageName, "imageID", *imageResult.ID)

	// Export to VHD if configured
	if c.config.ExportFormat == "vhd" && c.config.ContainerURL != "" {
		if reporter != nil {
			reporter.Describe("Exporting disks to VHD format")
		}

		vhdResults, err := c.ExportVMToVHD(ctx, vmName, c.config.ContainerURL, outputPath, reporter)
		if err != nil {
			c.logger.Error("VHD export failed", "error", err)
		} else {
			c.logger.Info("VHD export complete", "disks_exported", len(vhdResults))

			// Create manifest file for multi-disk exports
			if len(vhdResults) > 0 {
				if err := CreateExportManifest(vmName, vhdResults, outputPath); err != nil {
					c.logger.Warn("failed to create export manifest", "error", err)
				}
			}
		}
	}

	// Save metadata
	metadataPath := filepath.Join(outputPath, fmt.Sprintf("%s-metadata.json", vmName))
	if err := c.saveMetadata(vmInfo, *imageResult.ID, metadataPath); err != nil {
		c.logger.Warn("failed to save metadata", "error", err)
	}

	if reporter != nil {
		reporter.Describe("Export complete")
		reporter.Update(100)
	}

	c.logger.Info("Azure VM export complete", "vmName", vmName, "imageID", *imageResult.ID)
	return nil
}

// PowerOffVM powers off a VM
func (c *Client) PowerOffVM(ctx context.Context, vmName string) error {
	poller, err := c.vmClient.BeginPowerOff(ctx, c.config.ResourceGroup, vmName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin power off: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to power off VM: %w", err)
	}

	c.logger.Info("VM powered off", "vmName", vmName)
	return nil
}

// StartVM starts a VM
func (c *Client) StartVM(ctx context.Context, vmName string) error {
	poller, err := c.vmClient.BeginStart(ctx, c.config.ResourceGroup, vmName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin start: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	c.logger.Info("VM started", "vmName", vmName)
	return nil
}

// DeleteVM deletes a VM
func (c *Client) DeleteVM(ctx context.Context, vmName string) error {
	poller, err := c.vmClient.BeginDelete(ctx, c.config.ResourceGroup, vmName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	c.logger.Info("VM deleted", "vmName", vmName)
	return nil
}

// vmToVMInfo converts Azure VM to VMInfo
func (c *Client) vmToVMInfo(vm *armcompute.VirtualMachine) *VMInfo {
	vmInfo := &VMInfo{
		Name:          *vm.Name,
		ID:            *vm.ID,
		Location:      *vm.Location,
		ResourceGroup: c.config.ResourceGroup,
		Tags:          make(map[string]string),
		DiskNames:     make([]string, 0),
	}

	if vm.Properties != nil {
		if vm.Properties.ProvisioningState != nil {
			vmInfo.ProvisioningState = *vm.Properties.ProvisioningState
		}

		if vm.Properties.HardwareProfile != nil && vm.Properties.HardwareProfile.VMSize != nil {
			vmInfo.VMSize = string(*vm.Properties.HardwareProfile.VMSize)
		}

		if vm.Properties.StorageProfile != nil {
			if vm.Properties.StorageProfile.OSDisk != nil && vm.Properties.StorageProfile.OSDisk.OSType != nil {
				vmInfo.OSType = string(*vm.Properties.StorageProfile.OSDisk.OSType)
			}

			// Get disk names
			if vm.Properties.StorageProfile.OSDisk != nil && vm.Properties.StorageProfile.OSDisk.Name != nil {
				vmInfo.DiskNames = append(vmInfo.DiskNames, *vm.Properties.StorageProfile.OSDisk.Name)
			}

			if vm.Properties.StorageProfile.DataDisks != nil {
				for _, disk := range vm.Properties.StorageProfile.DataDisks {
					if disk.Name != nil {
						vmInfo.DiskNames = append(vmInfo.DiskNames, *disk.Name)
					}
				}
			}
		}

		// Get instance view for power state
		if vm.Properties.InstanceView != nil && vm.Properties.InstanceView.Statuses != nil {
			for _, status := range vm.Properties.InstanceView.Statuses {
				if status.Code != nil && strings.HasPrefix(*status.Code, "PowerState/") {
					vmInfo.PowerState = strings.TrimPrefix(*status.Code, "PowerState/")
					break
				}
			}
		}
	}

	// Extract tags
	if vm.Tags != nil {
		for key, value := range vm.Tags {
			if value != nil {
				vmInfo.Tags[key] = *value
			}
		}
	}

	return vmInfo
}

// saveMetadata saves VM metadata to a JSON file
func (c *Client) saveMetadata(vmInfo *VMInfo, imageID, path string) error {
	metadata := fmt.Sprintf(`{
  "provider": "azure",
  "vm_name": "%s",
  "vm_id": "%s",
  "location": "%s",
  "vm_size": "%s",
  "os_type": "%s",
  "provisioning_state": "%s",
  "power_state": "%s",
  "resource_group": "%s",
  "image_id": "%s",
  "export_time": "%s"
}`,
		vmInfo.Name,
		vmInfo.ID,
		vmInfo.Location,
		vmInfo.VMSize,
		vmInfo.OSType,
		vmInfo.ProvisioningState,
		vmInfo.PowerState,
		vmInfo.ResourceGroup,
		imageID,
		time.Now().Format(time.RFC3339),
	)

	return os.WriteFile(path, []byte(metadata), 0600)
}

// Close cleans up resources
func (c *Client) Close() error {
	c.logger.Info("Azure client closed")
	return nil
}

// String returns a string representation of the client
func (c *Client) String() string {
	return fmt.Sprintf("Azure Compute Client (subscription=%s, rg=%s)",
		c.config.SubscriptionID, c.config.ResourceGroup)
}

// ValidateCredentials validates Azure credentials by making a simple API call
func (c *Client) ValidateCredentials(ctx context.Context) error {
	// Try to list VMs as a validation check
	pager := c.vmClient.NewListPager(c.config.ResourceGroup, nil)
	_, err := pager.NextPage(ctx)
	if err != nil {
		return fmt.Errorf("invalid Azure credentials or permissions: %w", err)
	}
	return nil
}

// SearchVMs searches for VMs matching a query
func (c *Client) SearchVMs(ctx context.Context, query string) ([]*VMInfo, error) {
	vms, err := c.ListVMs(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var matches []*VMInfo

	for _, vm := range vms {
		// Search in name, ID, location, size, OS type
		if strings.Contains(strings.ToLower(vm.Name), query) ||
			strings.Contains(strings.ToLower(vm.ID), query) ||
			strings.Contains(strings.ToLower(vm.Location), query) ||
			strings.Contains(strings.ToLower(vm.VMSize), query) ||
			strings.Contains(strings.ToLower(vm.OSType), query) ||
			strings.Contains(strings.ToLower(vm.PowerState), query) {
			matches = append(matches, vm)
			continue
		}

		// Search in tags
		for key, value := range vm.Tags {
			if strings.Contains(strings.ToLower(key), query) ||
				strings.Contains(strings.ToLower(value), query) {
				matches = append(matches, vm)
				break
			}
		}
	}

	return matches, nil
}

// GetLocations returns available Azure regions (placeholder)
func (c *Client) GetLocations() []string {
	return []string{
		"eastus", "eastus2", "westus", "westus2", "westus3",
		"centralus", "northcentralus", "southcentralus",
		"westcentralus", "canadacentral", "canadaeast",
		"brazilsouth", "northeurope", "westeurope",
		"uksouth", "ukwest", "francecentral", "francesouth",
		"germanywestcentral", "norwayeast", "switzerlandnorth",
		"swedencentral", "uaenorth", "southafricanorth",
		"australiaeast", "australiasoutheast", "centralindia",
		"southindia", "japaneast", "japanwest",
		"koreacentral", "koreasouth", "southeastasia", "eastasia",
	}
}
