// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/option"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/progress"
)

// Config holds GCP provider configuration
type Config struct {
	ProjectID         string
	Zone              string
	Region            string
	CredentialsJSON   string // Path to service account JSON or JSON content
	DestinationBucket string // GCS bucket for exports
	Timeout           time.Duration
}

// Client represents a GCP Compute Engine client for VM operations
type Client struct {
	instancesClient *compute.InstancesClient
	imagesClient    *compute.ImagesClient
	disksClient     *compute.DisksClient
	config          *Config
	logger          logger.Logger
	ctx             context.Context
}

// VMInfo represents GCP VM instance information
type VMInfo struct {
	Name              string
	ID                uint64
	Zone              string
	MachineType       string
	Status            string
	InternalIP        string
	ExternalIP        string
	DiskNames         []string
	Labels            map[string]string
	CreationTimestamp string
	LastStartTime     string
}

// NewClient creates a new GCP Compute Engine client
func NewClient(cfg *Config, log logger.Logger) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 1 * time.Hour
	}

	ctx := context.Background()

	var opts []option.ClientOption
	if cfg.CredentialsJSON != "" {
		// Check if it's a file path or JSON content
		if _, err := os.Stat(cfg.CredentialsJSON); err == nil {
			opts = append(opts, option.WithCredentialsFile(cfg.CredentialsJSON))
		} else {
			opts = append(opts, option.WithCredentialsJSON([]byte(cfg.CredentialsJSON)))
		}
	}

	// Create instances client
	instancesClient, err := compute.NewInstancesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create instances client: %w", err)
	}

	// Create images client
	imagesClient, err := compute.NewImagesRESTClient(ctx, opts...)
	if err != nil {
		if closeErr := instancesClient.Close(); closeErr != nil {
			log.Warn("failed to close instances client during cleanup", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to create images client: %w", err)
	}

	// Create disks client
	disksClient, err := compute.NewDisksRESTClient(ctx, opts...)
	if err != nil {
		if closeErr := instancesClient.Close(); closeErr != nil {
			log.Warn("failed to close instances client during cleanup", "error", closeErr)
		}
		if closeErr := imagesClient.Close(); closeErr != nil {
			log.Warn("failed to close images client during cleanup", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to create disks client: %w", err)
	}

	return &Client{
		instancesClient: instancesClient,
		imagesClient:    imagesClient,
		disksClient:     disksClient,
		config:          cfg,
		logger:          log,
		ctx:             ctx,
	}, nil
}

// ListInstances returns a list of all instances in the zone
func (c *Client) ListInstances(ctx context.Context) ([]*VMInfo, error) {
	req := &computepb.ListInstancesRequest{
		Project: c.config.ProjectID,
		Zone:    c.config.Zone,
	}

	it := c.instancesClient.List(ctx, req)

	var instances []*VMInfo
	for {
		instance, err := it.Next()
		if err != nil {
			if err.Error() == "no more items in iterator" {
				break
			}
			return nil, fmt.Errorf("failed to list instances: %w", err)
		}

		vmInfo := c.instanceToVMInfo(instance)
		instances = append(instances, vmInfo)
	}

	c.logger.Info("discovered GCP instances", "count", len(instances))
	return instances, nil
}

// GetInstance retrieves information about a specific instance
func (c *Client) GetInstance(ctx context.Context, instanceName string) (*VMInfo, error) {
	req := &computepb.GetInstanceRequest{
		Project:  c.config.ProjectID,
		Zone:     c.config.Zone,
		Instance: instanceName,
	}

	instance, err := c.instancesClient.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance %s: %w", instanceName, err)
	}

	return c.instanceToVMInfo(instance), nil
}

// ExportInstance creates a machine image from a GCP instance
func (c *Client) ExportInstance(ctx context.Context, instanceName, outputPath string, reporter progress.ProgressReporter) error {
	c.logger.Info("starting GCP instance export", "instance", instanceName)

	// Get instance info
	vmInfo, err := c.GetInstance(ctx, instanceName)
	if err != nil {
		return err
	}

	if reporter != nil {
		reporter.Describe("Stopping instance")
	}

	// Stop instance
	stopReq := &computepb.StopInstanceRequest{
		Project:  c.config.ProjectID,
		Zone:     c.config.Zone,
		Instance: instanceName,
	}

	stopOp, err := c.instancesClient.Stop(ctx, stopReq)
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	// Wait for stop operation
	err = stopOp.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for instance to stop: %w", err)
	}

	c.logger.Info("instance stopped", "instance", instanceName)

	if reporter != nil {
		reporter.Describe("Creating machine image")
	}

	// Export to GCS if configured
	var exportResults []*ExportResult
	if c.config.DestinationBucket != "" {
		if reporter != nil {
			reporter.Describe("Exporting instance to GCS as VMDK")
		}

		results, err := c.ExportInstanceToGCS(ctx, instanceName, c.config.DestinationBucket, outputPath, reporter)
		if err != nil {
			c.logger.Error("GCS export failed", "error", err)
		} else {
			exportResults = results
			c.logger.Info("VMDK exported to GCS", "disks_exported", len(results))

			// Create manifest for multi-disk exports
			if len(results) > 1 {
				if err := CreateExportManifest(instanceName, results, outputPath); err != nil {
					c.logger.Warn("failed to create export manifest", "error", err)
				}
			}
		}
	} else {
		// Fallback: Create machine image only
		imageName := fmt.Sprintf("%s-image-%d", instanceName, time.Now().Unix())
		c.logger.Info("machine image creation initiated (no GCS export)", "imageName", imageName)
	}

	// Save metadata
	metadataPath := filepath.Join(outputPath, fmt.Sprintf("%s-metadata.json", instanceName))
	imageName := "see-manifest"
	if len(exportResults) > 0 {
		imageName = exportResults[0].ImageName
	}
	if err := c.saveMetadata(vmInfo, imageName, metadataPath); err != nil {
		c.logger.Warn("failed to save metadata", "error", err)
	}

	if reporter != nil {
		reporter.Describe("Export complete")
		reporter.Update(100)
	}

	c.logger.Info("GCP instance export complete", "instance", instanceName)
	return nil
}

// StopInstance stops a GCP instance
func (c *Client) StopInstance(ctx context.Context, instanceName string) error {
	req := &computepb.StopInstanceRequest{
		Project:  c.config.ProjectID,
		Zone:     c.config.Zone,
		Instance: instanceName,
	}

	op, err := c.instancesClient.Stop(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for stop: %w", err)
	}

	c.logger.Info("instance stopped", "instance", instanceName)
	return nil
}

// StartInstance starts a GCP instance
func (c *Client) StartInstance(ctx context.Context, instanceName string) error {
	req := &computepb.StartInstanceRequest{
		Project:  c.config.ProjectID,
		Zone:     c.config.Zone,
		Instance: instanceName,
	}

	op, err := c.instancesClient.Start(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for start: %w", err)
	}

	c.logger.Info("instance started", "instance", instanceName)
	return nil
}

// DeleteInstance deletes a GCP instance
func (c *Client) DeleteInstance(ctx context.Context, instanceName string) error {
	req := &computepb.DeleteInstanceRequest{
		Project:  c.config.ProjectID,
		Zone:     c.config.Zone,
		Instance: instanceName,
	}

	op, err := c.instancesClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for delete: %w", err)
	}

	c.logger.Info("instance deleted", "instance", instanceName)
	return nil
}

// instanceToVMInfo converts GCP Instance to VMInfo
func (c *Client) instanceToVMInfo(instance *computepb.Instance) *VMInfo {
	vmInfo := &VMInfo{
		Name:              instance.GetName(),
		ID:                instance.GetId(),
		Zone:              c.config.Zone,
		MachineType:       instance.GetMachineType(),
		Status:            instance.GetStatus(),
		Labels:            make(map[string]string),
		DiskNames:         make([]string, 0),
		CreationTimestamp: instance.GetCreationTimestamp(),
		LastStartTime:     instance.GetLastStartTimestamp(),
	}

	// Extract machine type (last part of URL)
	if parts := strings.Split(vmInfo.MachineType, "/"); len(parts) > 0 {
		vmInfo.MachineType = parts[len(parts)-1]
	}

	// Get network interfaces
	for _, ni := range instance.NetworkInterfaces {
		if ni.NetworkIP != nil {
			vmInfo.InternalIP = *ni.NetworkIP
		}

		// Get external IP from access configs
		for _, ac := range ni.AccessConfigs {
			if ac.NatIP != nil {
				vmInfo.ExternalIP = *ac.NatIP
				break
			}
		}

		if vmInfo.ExternalIP != "" {
			break
		}
	}

	// Get disk names
	for _, disk := range instance.Disks {
		if disk.Source != nil {
			// Extract disk name from source URL
			parts := strings.Split(*disk.Source, "/")
			if len(parts) > 0 {
				vmInfo.DiskNames = append(vmInfo.DiskNames, parts[len(parts)-1])
			}
		}
	}

	// Get labels
	if instance.Labels != nil {
		vmInfo.Labels = instance.Labels
	}

	return vmInfo
}

// saveMetadata saves instance metadata to a JSON file
func (c *Client) saveMetadata(vmInfo *VMInfo, imageName, path string) error {
	metadata := fmt.Sprintf(`{
  "provider": "gcp",
  "instance_name": "%s",
  "instance_id": %d,
  "zone": "%s",
  "machine_type": "%s",
  "status": "%s",
  "internal_ip": "%s",
  "external_ip": "%s",
  "image_name": "%s",
  "creation_timestamp": "%s",
  "export_time": "%s"
}`,
		vmInfo.Name,
		vmInfo.ID,
		vmInfo.Zone,
		vmInfo.MachineType,
		vmInfo.Status,
		vmInfo.InternalIP,
		vmInfo.ExternalIP,
		imageName,
		vmInfo.CreationTimestamp,
		time.Now().Format(time.RFC3339),
	)

	return os.WriteFile(path, []byte(metadata), 0600)
}

// Close cleans up resources
func (c *Client) Close() error {
	var errs []error
	if c.instancesClient != nil {
		if err := c.instancesClient.Close(); err != nil {
			c.logger.Warn("failed to close instances client", "error", err)
			errs = append(errs, err)
		}
	}
	if c.imagesClient != nil {
		if err := c.imagesClient.Close(); err != nil {
			c.logger.Warn("failed to close images client", "error", err)
			errs = append(errs, err)
		}
	}
	if c.disksClient != nil {
		if err := c.disksClient.Close(); err != nil {
			c.logger.Warn("failed to close disks client", "error", err)
			errs = append(errs, err)
		}
	}
	c.logger.Info("GCP client closed")
	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d gcp client(s): %w", len(errs), errs[0])
	}
	return nil
}

// String returns a string representation of the client
func (c *Client) String() string {
	return fmt.Sprintf("GCP Compute Engine Client (project=%s, zone=%s)",
		c.config.ProjectID, c.config.Zone)
}

// ValidateCredentials validates GCP credentials by making a simple API call
func (c *Client) ValidateCredentials(ctx context.Context) error {
	req := &computepb.ListInstancesRequest{
		Project:    c.config.ProjectID,
		Zone:       c.config.Zone,
		MaxResults: makeUint32Ptr(1),
	}

	it := c.instancesClient.List(ctx, req)
	_, err := it.Next()
	if err != nil && err.Error() != "no more items in iterator" {
		return fmt.Errorf("invalid GCP credentials or permissions: %w", err)
	}
	return nil
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
		// Search in name, machine type, status, IPs
		if strings.Contains(strings.ToLower(vm.Name), query) ||
			strings.Contains(strings.ToLower(vm.MachineType), query) ||
			strings.Contains(strings.ToLower(vm.Status), query) ||
			strings.Contains(vm.InternalIP, query) ||
			strings.Contains(vm.ExternalIP, query) {
			matches = append(matches, vm)
			continue
		}

		// Search in labels
		for key, value := range vm.Labels {
			if strings.Contains(strings.ToLower(key), query) ||
				strings.Contains(strings.ToLower(value), query) {
				matches = append(matches, vm)
				break
			}
		}
	}

	return matches, nil
}

// GetZones returns available GCP zones (placeholder)
func (c *Client) GetZones() []string {
	return []string{
		"us-central1-a", "us-central1-b", "us-central1-c", "us-central1-f",
		"us-east1-b", "us-east1-c", "us-east1-d",
		"us-east4-a", "us-east4-b", "us-east4-c",
		"us-west1-a", "us-west1-b", "us-west1-c",
		"us-west2-a", "us-west2-b", "us-west2-c",
		"us-west3-a", "us-west3-b", "us-west3-c",
		"us-west4-a", "us-west4-b", "us-west4-c",
		"europe-west1-b", "europe-west1-c", "europe-west1-d",
		"europe-west2-a", "europe-west2-b", "europe-west2-c",
		"europe-west3-a", "europe-west3-b", "europe-west3-c",
		"europe-west4-a", "europe-west4-b", "europe-west4-c",
		"europe-north1-a", "europe-north1-b", "europe-north1-c",
		"asia-east1-a", "asia-east1-b", "asia-east1-c",
		"asia-east2-a", "asia-east2-b", "asia-east2-c",
		"asia-northeast1-a", "asia-northeast1-b", "asia-northeast1-c",
		"asia-northeast2-a", "asia-northeast2-b", "asia-northeast2-c",
		"asia-northeast3-a", "asia-northeast3-b", "asia-northeast3-c",
		"asia-south1-a", "asia-south1-b", "asia-south1-c",
		"asia-southeast1-a", "asia-southeast1-b", "asia-southeast1-c",
		"australia-southeast1-a", "australia-southeast1-b", "australia-southeast1-c",
	}
}

// Helper function to create uint32 pointer
func makeUint32Ptr(v uint32) *uint32 {
	return &v
}
