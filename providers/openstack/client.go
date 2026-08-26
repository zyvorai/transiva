// SPDX-License-Identifier: Apache-2.0

package openstack

import (
	"context"
	"fmt"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/startstop"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// Client wraps OpenStack Nova API for instance management
type Client struct {
	provider      *gophercloud.ProviderClient
	computeClient *gophercloud.ServiceClient
	imageClient   *gophercloud.ServiceClient
	config        *config.OpenStackConfig
	logger        logger.Logger
	retryer       *retry.Retryer
}

// NewClient creates a new OpenStack client
func NewClient(cfg *config.OpenStackConfig, log logger.Logger) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("OpenStack config is required")
	}

	// Create authentication options
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: cfg.AuthURL,
		Username:         cfg.Username,
		Password:         cfg.Password,
		TenantName:       cfg.TenantName,
		TenantID:         cfg.TenantID,
		DomainName:       cfg.DomainName,
	}

	// Create provider client
	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		return nil, fmt.Errorf("authenticate to OpenStack: %w", err)
	}

	// Create compute client
	computeClient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create compute client: %w", err)
	}

	// Create image client
	imageClient, err := openstack.NewImageServiceV2(provider, gophercloud.EndpointOpts{
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create image client: %w", err)
	}

	// Initialize retryer with defaults
	retryConfig := &retry.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
	retryer := retry.NewRetryer(retryConfig, log)

	return &Client{
		provider:      provider,
		computeClient: computeClient,
		imageClient:   imageClient,
		config:        cfg,
		logger:        log,
		retryer:       retryer,
	}, nil
}

// SetNetworkMonitor sets the network monitor for retry operations
func (c *Client) SetNetworkMonitor(monitor retry.NetworkMonitor) {
	c.retryer.SetNetworkMonitor(monitor)
}

// ListInstances lists compute instances
func (c *Client) ListInstances(ctx context.Context) ([]InstanceInfo, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying list instances", "attempt", attempt)
		}

		allPages, err := servers.List(c.computeClient, nil).AllPages()
		if err != nil {
			return nil, fmt.Errorf("list servers: %w", err)
		}

		serverList, err := servers.ExtractServers(allPages)
		if err != nil {
			return nil, fmt.Errorf("extract servers: %w", err)
		}

		var instances []InstanceInfo
		for _, server := range serverList {
			instances = append(instances, InstanceInfo{
				ID:       server.ID,
				Name:     server.Name,
				Status:   server.Status,
				TenantID: server.TenantID,
				Flavor:   server.Flavor["id"].(string),
				Created:  server.Created,
			})
		}

		return instances, nil
	}, "list OpenStack instances")

	if err != nil {
		return nil, err
	}

	return result.([]InstanceInfo), nil
}

// GetInstance gets details of a specific instance
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*InstanceInfo, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying get instance", "instance_id", instanceID, "attempt", attempt)
		}

		server, err := servers.Get(c.computeClient, instanceID).Extract()
		if err != nil {
			// Not found errors are not retryable
			if _, ok := err.(gophercloud.ErrDefault404); ok {
				return nil, retry.IsNonRetryable(fmt.Errorf("instance not found: %w", err))
			}
			return nil, fmt.Errorf("get server: %w", err)
		}

		info := &InstanceInfo{
			ID:       server.ID,
			Name:     server.Name,
			Status:   server.Status,
			TenantID: server.TenantID,
			Flavor:   server.Flavor["id"].(string),
			Created:  server.Created,
		}

		return info, nil
	}, fmt.Sprintf("get OpenStack instance %s", instanceID))

	if err != nil {
		return nil, err
	}

	return result.(*InstanceInfo), nil
}

// StopInstance stops a running instance
func (c *Client) StopInstance(ctx context.Context, instanceID string) error {
	return c.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			c.logger.Info("retrying stop instance", "instance_id", instanceID, "attempt", attempt)
		}

		err := startstop.Stop(c.computeClient, instanceID).ExtractErr()
		if err != nil {
			return fmt.Errorf("stop server: %w", err)
		}

		c.logger.Info("instance stopped", "instance_id", instanceID)
		return nil
	}, fmt.Sprintf("stop OpenStack instance %s", instanceID))
}

// StartInstance starts a stopped instance
func (c *Client) StartInstance(ctx context.Context, instanceID string) error {
	return c.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			c.logger.Info("retrying start instance", "instance_id", instanceID, "attempt", attempt)
		}

		err := startstop.Start(c.computeClient, instanceID).ExtractErr()
		if err != nil {
			return fmt.Errorf("start server: %w", err)
		}

		c.logger.Info("instance started", "instance_id", instanceID)
		return nil
	}, fmt.Sprintf("start OpenStack instance %s", instanceID))
}

// CreateSnapshot creates a snapshot/image of an instance
func (c *Client) CreateSnapshot(ctx context.Context, instanceID, snapshotName string) (string, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying create snapshot", "instance_id", instanceID, "attempt", attempt)
		}

		createOpts := servers.CreateImageOpts{
			Name: snapshotName,
		}

		imageID, err := servers.CreateImage(c.computeClient, instanceID, createOpts).ExtractImageID()
		if err != nil {
			return nil, fmt.Errorf("create image: %w", err)
		}

		c.logger.Info("snapshot created", "instance_id", instanceID, "image_id", imageID)
		return imageID, nil
	}, fmt.Sprintf("create OpenStack snapshot %s", snapshotName))

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

// WaitForImageStatus waits for an image to reach a specific status
func (c *Client) WaitForImageStatus(ctx context.Context, imageID, desiredStatus string, timeout time.Duration) error {
	c.logger.Info("waiting for image status",
		"image_id", imageID,
		"desired_status", desiredStatus,
		"timeout", timeout)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for image status %s", desiredStatus)
			}

			image, err := images.Get(c.imageClient, imageID).Extract()
			if err != nil {
				return fmt.Errorf("get image: %w", err)
			}

			currentStatus := string(image.Status)
			c.logger.Debug("image status check",
				"image_id", imageID,
				"current_status", currentStatus,
				"desired_status", desiredStatus)

			if currentStatus == desiredStatus {
				c.logger.Info("image reached desired status",
					"image_id", imageID,
					"status", desiredStatus)
				return nil
			}

			// Check for failed states
			if currentStatus == "killed" || currentStatus == "deleted" {
				return fmt.Errorf("image is in failed state: %s", currentStatus)
			}
		}
	}
}

// DeleteImage deletes an image
func (c *Client) DeleteImage(ctx context.Context, imageID string) error {
	return c.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			c.logger.Info("retrying delete image", "image_id", imageID, "attempt", attempt)
		}

		err := images.Delete(c.imageClient, imageID).ExtractErr()
		if err != nil {
			// Ignore not found errors
			if _, ok := err.(gophercloud.ErrDefault404); ok {
				return nil
			}
			return fmt.Errorf("delete image: %w", err)
		}

		c.logger.Info("image deleted", "image_id", imageID)
		return nil
	}, fmt.Sprintf("delete OpenStack image %s", imageID))
}

// InstanceInfo contains information about an OpenStack instance
type InstanceInfo struct {
	ID       string
	Name     string
	Status   string
	TenantID string
	Flavor   string
	Created  time.Time
}
