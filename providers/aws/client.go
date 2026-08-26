// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/progress"
)

// Config holds AWS provider configuration
type Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	S3Bucket        string // Temporary bucket for exports
	Timeout         time.Duration
}

// Client represents an AWS EC2 client for VM operations
type Client struct {
	ec2Client *ec2.Client
	s3Client  *s3.Client
	config    *Config
	logger    logger.Logger
}

// VMInfo represents AWS EC2 instance information
type VMInfo struct {
	InstanceID       string
	ImageID          string
	Name             string
	State            string
	InstanceType     string
	Platform         string
	AvailabilityZone string
	Region           string
	PrivateIP        string
	PublicIP         string
	VolumeIDs        []string
	Tags             map[string]string
	LaunchTime       time.Time
}

// NewClient creates a new AWS EC2 client
func NewClient(cfg *Config, log logger.Logger) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 1 * time.Hour
	}

	// Create AWS config
	var awsCfg aws.Config
	var err error

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use static credentials
		awsCfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				cfg.SessionToken,
			)),
		)
	} else {
		// Use default credential chain (env vars, shared config, IAM role)
		awsCfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		ec2Client: ec2.NewFromConfig(awsCfg),
		s3Client:  s3.NewFromConfig(awsCfg),
		config:    cfg,
		logger:    log,
	}, nil
}

// ListInstances returns a list of all EC2 instances
func (c *Client) ListInstances(ctx context.Context) ([]*VMInfo, error) {
	input := &ec2.DescribeInstancesInput{}

	result, err := c.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []*VMInfo
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			vmInfo := c.instanceToVMInfo(instance)
			instances = append(instances, vmInfo)
		}
	}

	c.logger.Info("discovered EC2 instances", "count", len(instances))
	return instances, nil
}

// GetInstance retrieves information about a specific EC2 instance
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*VMInfo, error) {
	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := c.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return c.instanceToVMInfo(result.Reservations[0].Instances[0]), nil
}

// ExportInstance creates an AMI from an EC2 instance
func (c *Client) ExportInstance(ctx context.Context, instanceID, outputPath string, reporter progress.ProgressReporter) error {
	c.logger.Info("starting EC2 instance export", "instanceID", instanceID)

	// Get instance info
	vmInfo, err := c.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}

	// Create AMI
	amiName := fmt.Sprintf("%s-export-%d", vmInfo.Name, time.Now().Unix())
	createImageInput := &ec2.CreateImageInput{
		InstanceId: aws.String(instanceID),
		Name:       aws.String(amiName),
		NoReboot:   aws.Bool(true), // Don't reboot the instance
	}

	if reporter != nil {
		reporter.Describe("Creating AMI snapshot")
	}

	createImageOutput, err := c.ec2Client.CreateImage(ctx, createImageInput)
	if err != nil {
		return fmt.Errorf("failed to create AMI: %w", err)
	}

	imageID := *createImageOutput.ImageId
	c.logger.Info("AMI creation initiated", "imageID", imageID)

	// Wait for AMI to be available
	if reporter != nil {
		reporter.Describe("Waiting for AMI to be ready")
	}

	waiter := ec2.NewImageAvailableWaiter(c.ec2Client)
	err = waiter.Wait(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{imageID},
	}, c.config.Timeout)
	if err != nil {
		return fmt.Errorf("failed waiting for AMI: %w", err)
	}

	c.logger.Info("AMI is ready", "imageID", imageID)

	// Export instance to S3 as VMDK (requires VM Import/Export role)
	if c.config.S3Bucket != "" {
		if reporter != nil {
			reporter.Describe("Exporting instance to S3 as VMDK")
		}

		result, err := c.ExportInstanceToS3(ctx, instanceID, outputPath, reporter)
		if err != nil {
			c.logger.Error("S3 export failed, continuing with AMI only", "error", err)
		} else {
			c.logger.Info("VMDK exported to S3", "path", result.LocalPath, "s3_key", result.S3Key)
		}
	}

	// Create metadata file
	metadataPath := filepath.Join(outputPath, fmt.Sprintf("%s-metadata.json", instanceID))
	if err := c.saveMetadata(vmInfo, imageID, metadataPath); err != nil {
		c.logger.Warn("failed to save metadata", "error", err)
	}

	if reporter != nil {
		reporter.Describe("Export complete")
		reporter.Update(100)
	}

	c.logger.Info("EC2 instance export complete", "instanceID", instanceID, "imageID", imageID)
	return nil
}

// StopInstance stops an EC2 instance
func (c *Client) StopInstance(ctx context.Context, instanceID string) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2Client.StopInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceID, err)
	}

	c.logger.Info("instance stop initiated", "instanceID", instanceID)
	return nil
}

// TerminateInstance terminates an EC2 instance
func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2Client.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}

	c.logger.Info("instance termination initiated", "instanceID", instanceID)
	return nil
}

// instanceToVMInfo converts EC2 Instance to VMInfo
func (c *Client) instanceToVMInfo(instance types.Instance) *VMInfo {
	vmInfo := &VMInfo{
		InstanceID:       *instance.InstanceId,
		State:            string(instance.State.Name),
		InstanceType:     string(instance.InstanceType),
		AvailabilityZone: *instance.Placement.AvailabilityZone,
		Region:           c.config.Region,
		Tags:             make(map[string]string),
	}

	// Populate ImageID if available
	if instance.ImageId != nil {
		vmInfo.ImageID = *instance.ImageId
	}

	if instance.LaunchTime != nil {
		vmInfo.LaunchTime = *instance.LaunchTime
	}

	if instance.Platform != "" {
		vmInfo.Platform = string(instance.Platform)
	} else {
		vmInfo.Platform = "linux"
	}

	if instance.PrivateIpAddress != nil {
		vmInfo.PrivateIP = *instance.PrivateIpAddress
	}

	if instance.PublicIpAddress != nil {
		vmInfo.PublicIP = *instance.PublicIpAddress
	}

	// Extract name from tags
	for _, tag := range instance.Tags {
		if tag.Key != nil && tag.Value != nil {
			vmInfo.Tags[*tag.Key] = *tag.Value
			if *tag.Key == "Name" {
				vmInfo.Name = *tag.Value
			}
		}
	}

	// If no name tag, use instance ID
	if vmInfo.Name == "" {
		vmInfo.Name = vmInfo.InstanceID
	}

	// Get volume IDs
	for _, mapping := range instance.BlockDeviceMappings {
		if mapping.Ebs != nil && mapping.Ebs.VolumeId != nil {
			vmInfo.VolumeIDs = append(vmInfo.VolumeIDs, *mapping.Ebs.VolumeId)
		}
	}

	return vmInfo
}

// saveMetadata saves instance metadata to a JSON file
func (c *Client) saveMetadata(vmInfo *VMInfo, imageID, path string) error {
	metadata := fmt.Sprintf(`{
  "provider": "aws",
  "instance_id": "%s",
  "name": "%s",
  "state": "%s",
  "instance_type": "%s",
  "platform": "%s",
  "availability_zone": "%s",
  "private_ip": "%s",
  "public_ip": "%s",
  "image_id": "%s",
  "launch_time": "%s",
  "export_time": "%s"
}`,
		vmInfo.InstanceID,
		vmInfo.Name,
		vmInfo.State,
		vmInfo.InstanceType,
		vmInfo.Platform,
		vmInfo.AvailabilityZone,
		vmInfo.PrivateIP,
		vmInfo.PublicIP,
		imageID,
		vmInfo.LaunchTime.Format(time.RFC3339),
		time.Now().Format(time.RFC3339),
	)

	return os.WriteFile(path, []byte(metadata), 0600)
}

// DownloadSnapshot downloads an EBS snapshot as VMDK via S3
func (c *Client) DownloadSnapshot(ctx context.Context, snapshotID, outputPath string, reporter progress.ProgressReporter) error {
	c.logger.Info("downloading EBS snapshot", "snapshotID", snapshotID)

	if c.config.S3Bucket == "" {
		return fmt.Errorf("S3 bucket required for snapshot export - configure S3Bucket in client config")
	}

	// Export snapshot to S3 as VMDK
	result, err := c.ExportSnapshotToS3(ctx, snapshotID, outputPath, reporter)
	if err != nil {
		return fmt.Errorf("export snapshot to S3: %w", err)
	}

	c.logger.Info("snapshot downloaded successfully", "path", result.LocalPath, "size_bytes", result.Size)

	if reporter != nil {
		reporter.Describe("Snapshot export complete")
		reporter.Update(100)
	}

	return nil
}

// Close cleans up resources
func (c *Client) Close() error {
	c.logger.Info("AWS client closed")
	return nil
}

// String returns a string representation of the client
func (c *Client) String() string {
	return fmt.Sprintf("AWS EC2 Client (region=%s)", c.config.Region)
}

// ValidateCredentials validates AWS credentials by making a simple API call
func (c *Client) ValidateCredentials(ctx context.Context) error {
	_, err := c.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return fmt.Errorf("invalid AWS credentials: %w", err)
	}
	return nil
}

// GetRegions returns available AWS regions
func (c *Client) GetRegions(ctx context.Context) ([]string, error) {
	result, err := c.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe regions: %w", err)
	}

	var regions []string
	for _, region := range result.Regions {
		if region.RegionName != nil {
			regions = append(regions, *region.RegionName)
		}
	}

	return regions, nil
}

// SearchInstances searches for instances matching a query
func (c *Client) SearchInstances(ctx context.Context, query string) ([]*VMInfo, error) {
	instances, err := c.ListInstances(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var matches []*VMInfo

	for _, vm := range instances {
		// Search in name, instance ID, state, type, IPs
		if strings.Contains(strings.ToLower(vm.Name), query) ||
			strings.Contains(strings.ToLower(vm.InstanceID), query) ||
			strings.Contains(strings.ToLower(vm.State), query) ||
			strings.Contains(strings.ToLower(vm.InstanceType), query) ||
			strings.Contains(vm.PrivateIP, query) ||
			strings.Contains(vm.PublicIP, query) {
			matches = append(matches, vm)
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
