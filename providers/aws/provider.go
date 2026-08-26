// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// AWSProvider implements the Provider interface for Amazon EC2
type AWSProvider struct {
	client *Client // Assumes Client exists in providers/aws/client.go
	config providers.ProviderConfig
	logger logger.Logger
}

// NewProvider creates a new AWS provider instance (factory function)
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	return &AWSProvider{
		config: cfg,
		logger: log,
	}, nil
}

// Name returns the provider name
func (p *AWSProvider) Name() string {
	return "Amazon Web Services EC2"
}

// Type returns the provider type
func (p *AWSProvider) Type() providers.ProviderType {
	return providers.ProviderAWS
}

// Connect establishes a connection to AWS
func (p *AWSProvider) Connect(ctx context.Context, config providers.ProviderConfig) error {
	// Extract AWS-specific config from metadata
	region, _ := config.Metadata["region"].(string)
	if region == "" {
		region = "us-east-1" // Default region
	}

	accessKey, _ := config.Metadata["access_key"].(string)
	secretKey, _ := config.Metadata["secret_key"].(string)

	if accessKey == "" {
		accessKey = config.Username
	}
	if secretKey == "" {
		secretKey = config.Password
	}

	// Create AWS config
	awsConfig := &Config{
		Region:          region,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
	}

	// Create AWS client
	client, err := NewClient(awsConfig, p.logger)
	if err != nil {
		return fmt.Errorf("failed to connect to AWS: %w", err)
	}

	p.client = client
	p.config = config
	return nil
}

// Disconnect closes the AWS connection
func (p *AWSProvider) Disconnect() error {
	// AWS SDK handles connection cleanup automatically
	return nil
}

// ValidateCredentials validates AWS credentials
func (p *AWSProvider) ValidateCredentials(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("not connected")
	}

	// Validate credentials by calling DescribeRegions API
	if err := p.client.ValidateCredentials(ctx); err != nil {
		return fmt.Errorf("AWS credential validation failed: %w", err)
	}

	p.logger.Info("AWS credentials validated successfully")
	return nil
}

// ListVMs lists EC2 instances matching the filter
func (p *AWSProvider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
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
			Provider: providers.ProviderAWS,
			ID:       inst.InstanceID,
			Name:     inst.Name,
			State:    inst.State,
			Location: inst.Region,
			Tags:     inst.Tags,
			Metadata: map[string]interface{}{
				"instance_type": inst.InstanceType,
				"image_id":      inst.ImageID,
			},
		}
		result = append(result, vmInfo)
	}

	return result, nil
}

// GetVM retrieves information about a specific EC2 instance
func (p *AWSProvider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	inst, err := p.client.GetInstance(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}

	return &providers.VMInfo{
		Provider: providers.ProviderAWS,
		ID:       inst.InstanceID,
		Name:     inst.Name,
		State:    inst.State,
		Location: inst.Region,
		Tags:     inst.Tags,
		Metadata: map[string]interface{}{
			"instance_type": inst.InstanceType,
			"image_id":      inst.ImageID,
		},
	}, nil
}

// SearchVMs searches for EC2 instances by query string
func (p *AWSProvider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Search instances by name tag, instance ID, state, type, IPs, and tags
	instances, err := p.client.SearchInstances(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search instances: %w", err)
	}

	// Convert to VMInfo format
	result := make([]*providers.VMInfo, 0, len(instances))
	for _, inst := range instances {
		vmInfo := &providers.VMInfo{
			Provider: providers.ProviderAWS,
			ID:       inst.InstanceID,
			Name:     inst.Name,
			State:    inst.State,
			Location: inst.Region,
			Tags:     inst.Tags,
			Metadata: map[string]interface{}{
				"instance_type": inst.InstanceType,
				"image_id":      inst.ImageID,
			},
		}
		result = append(result, vmInfo)
	}

	p.logger.Info("search completed", "query", query, "results", len(result))
	return result, nil
}

// ExportVM exports an EC2 instance to VMDK format
func (p *AWSProvider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	if p.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Update client's S3 bucket if provided in options
	bucket, _ := opts.Metadata["s3_bucket"].(string)
	if bucket != "" {
		p.client.config.S3Bucket = bucket
	}

	// Export instance to S3 if S3 bucket is configured
	if p.client.config.S3Bucket != "" {
		result, err := p.client.ExportInstanceToS3(ctx, identifier, opts.OutputPath, nil)
		if err != nil {
			return nil, fmt.Errorf("export instance to S3: %w", err)
		}

		return &providers.ExportResult{
			Provider:   providers.ProviderAWS,
			VMID:       identifier,
			Format:     result.Format,
			OutputPath: result.LocalPath,
			Size:       result.Size,
			Metadata: map[string]interface{}{
				"ami_id":      result.ImageID,
				"s3_bucket":   result.S3Bucket,
				"s3_key":      result.S3Key,
				"instance_id": result.InstanceID,
			},
		}, nil
	}

	// Fallback: Create AMI only (no S3 export)
	err := p.client.ExportInstance(ctx, identifier, opts.OutputPath, nil)
	if err != nil {
		return nil, fmt.Errorf("export instance: %w", err)
	}

	return &providers.ExportResult{
		Provider:   providers.ProviderAWS,
		VMID:       identifier,
		Format:     "ami",
		OutputPath: opts.OutputPath,
		Metadata: map[string]interface{}{
			"note": "S3 bucket not configured - AMI created but not exported to VMDK",
		},
	}, nil
}

// GetExportCapabilities returns the export capabilities of AWS
func (p *AWSProvider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{"vmdk", "vhd", "raw"},
		SupportsCompression: false, // S3 handles compression
		SupportsStreaming:   true,  // Can stream to S3
		SupportsSnapshots:   true,  // EBS snapshots
		MaxVMSizeGB:         1000,  // AWS export task limit
		SupportedTargets:    []string{"s3", "local"},
	}
}
