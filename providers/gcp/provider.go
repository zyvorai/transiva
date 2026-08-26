// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// GCPProvider implements the Provider interface for Google Cloud Platform
type GCPProvider struct {
	client *Client
	config providers.ProviderConfig
	logger logger.Logger
}

// NewProvider creates a new GCP provider instance (factory function)
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	return &GCPProvider{
		config: cfg,
		logger: log,
	}, nil
}

// Name returns the provider name
func (p *GCPProvider) Name() string {
	return "Google Cloud Platform"
}

// Type returns the provider type
func (p *GCPProvider) Type() providers.ProviderType {
	return providers.ProviderGCP
}

// Connect establishes a connection to GCP
func (p *GCPProvider) Connect(ctx context.Context, config providers.ProviderConfig) error {
	// Extract GCP-specific config from metadata
	projectID, _ := config.Metadata["project_id"].(string)
	zone, _ := config.Metadata["zone"].(string)
	region, _ := config.Metadata["region"].(string)
	credentialsJSON, _ := config.Metadata["credentials_json"].(string)
	bucket, _ := config.Metadata["gcs_bucket"].(string)

	// Use Password field for credentials if not in metadata
	if credentialsJSON == "" {
		credentialsJSON = config.Password
	}

	// Defaults
	if zone == "" {
		zone = "us-central1-a"
	}
	if region == "" {
		region = "us-central1"
	}

	// Create GCP client config
	gcpConfig := &Config{
		ProjectID:         projectID,
		Zone:              zone,
		Region:            region,
		CredentialsJSON:   credentialsJSON,
		DestinationBucket: bucket,
	}

	// Create GCP client
	client, err := NewClient(gcpConfig, p.logger)
	if err != nil {
		return fmt.Errorf("failed to connect to GCP: %w", err)
	}

	p.client = client
	p.config = config
	return nil
}

// Disconnect closes the GCP connection
func (p *GCPProvider) Disconnect() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// ValidateCredentials validates GCP credentials
func (p *GCPProvider) ValidateCredentials(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("not connected")
	}

	return p.client.ValidateCredentials(ctx)
}

// ListVMs lists GCP instances matching the filter
func (p *GCPProvider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	instances, err := p.client.ListInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}

	result := make([]*providers.VMInfo, 0, len(instances))
	for _, inst := range instances {
		vmInfo := &providers.VMInfo{
			Provider: providers.ProviderGCP,
			ID:       fmt.Sprintf("%d", inst.ID),
			Name:     inst.Name,
			State:    inst.Status,
			Location: inst.Zone,
			Tags:     inst.Labels,
			Metadata: map[string]interface{}{
				"machine_type":       inst.MachineType,
				"internal_ip":        inst.InternalIP,
				"external_ip":        inst.ExternalIP,
				"disk_names":         inst.DiskNames,
				"creation_timestamp": inst.CreationTimestamp,
			},
		}
		result = append(result, vmInfo)
	}

	return result, nil
}

// GetVM retrieves information about a specific GCP instance
func (p *GCPProvider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	inst, err := p.client.GetInstance(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}

	return &providers.VMInfo{
		Provider: providers.ProviderGCP,
		ID:       fmt.Sprintf("%d", inst.ID),
		Name:     inst.Name,
		State:    inst.Status,
		Location: inst.Zone,
		Tags:     inst.Labels,
		Metadata: map[string]interface{}{
			"machine_type": inst.MachineType,
			"internal_ip":  inst.InternalIP,
			"external_ip":  inst.ExternalIP,
			"disk_names":   inst.DiskNames,
		},
	}, nil
}

// SearchVMs searches for GCP instances by query string
func (p *GCPProvider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	instances, err := p.client.SearchInstances(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search instances: %w", err)
	}

	result := make([]*providers.VMInfo, 0, len(instances))
	for _, inst := range instances {
		vmInfo := &providers.VMInfo{
			Provider: providers.ProviderGCP,
			ID:       fmt.Sprintf("%d", inst.ID),
			Name:     inst.Name,
			State:    inst.Status,
			Location: inst.Zone,
			Tags:     inst.Labels,
			Metadata: map[string]interface{}{
				"machine_type": inst.MachineType,
				"internal_ip":  inst.InternalIP,
				"external_ip":  inst.ExternalIP,
			},
		}
		result = append(result, vmInfo)
	}

	return result, nil
}

// ExportVM exports a GCP instance to VMDK format via GCS
func (p *GCPProvider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Update client config with export options
	gcsBucket, _ := opts.Metadata["gcs_bucket"].(string)
	if gcsBucket != "" {
		p.client.config.DestinationBucket = gcsBucket
	}

	// Export instance to GCS if bucket is configured
	if p.client.config.DestinationBucket != "" {
		results, err := p.client.ExportInstanceToGCS(ctx, identifier, p.client.config.DestinationBucket, opts.OutputPath, nil)
		if err != nil {
			return nil, fmt.Errorf("export instance to GCS: %w", err)
		}

		if len(results) == 0 {
			return nil, fmt.Errorf("no disks exported")
		}

		// Create manifest for multi-disk exports
		if len(results) > 1 {
			if err := CreateExportManifest(identifier, results, opts.OutputPath); err != nil {
				p.logger.Warn("failed to create manifest", "error", err)
			}
		}

		// Return result for first disk (boot disk)
		firstDisk := results[0]
		return &providers.ExportResult{
			Provider:   providers.ProviderGCP,
			VMID:       identifier,
			Format:     "vmdk",
			OutputPath: firstDisk.LocalPath,
			Size:       firstDisk.Size,
			Metadata: map[string]interface{}{
				"disk_count": len(results),
				"gcs_bucket": firstDisk.GCSBucket,
				"gcs_object": firstDisk.GCSObject,
				"gcs_uri":    firstDisk.GCSURI,
				"image_name": firstDisk.ImageName,
				"disk_name":  firstDisk.DiskName,
				"disk_type":  firstDisk.DiskType,
			},
		}, nil
	}

	// Fallback: Create machine image only (no GCS export)
	err := p.client.ExportInstance(ctx, identifier, opts.OutputPath, nil)
	if err != nil {
		return nil, fmt.Errorf("export instance: %w", err)
	}

	return &providers.ExportResult{
		Provider:   providers.ProviderGCP,
		VMID:       identifier,
		Format:     "image",
		OutputPath: opts.OutputPath,
		Metadata: map[string]interface{}{
			"note": "Created GCP machine image - set gcs_bucket for VMDK export",
		},
	}, nil
}

// GetExportCapabilities returns the export capabilities of GCP
func (p *GCPProvider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{"vmdk", "image"},
		SupportsCompression: false, // GCS handles compression
		SupportsStreaming:   true,  // Can stream to GCS
		SupportsSnapshots:   true,  // Persistent disk snapshots
		MaxVMSizeGB:         65536, // GCP max persistent disk size
		SupportedTargets:    []string{"gcs", "local"},
	}
}
