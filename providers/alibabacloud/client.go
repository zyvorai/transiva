// SPDX-License-Identifier: Apache-2.0

package alibabacloud

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// Client wraps Alibaba Cloud ECS SDK for instance management
type Client struct {
	ecsClient *ecs.Client
	config    *config.AlibabaCloudConfig
	logger    logger.Logger
	retryer   *retry.Retryer
}

// NewClient creates a new Alibaba Cloud ECS client
func NewClient(cfg *config.AlibabaCloudConfig, log logger.Logger) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Alibaba Cloud config is required")
	}

	// Create ECS client
	ecsClient, err := ecs.NewClientWithAccessKey(cfg.RegionID, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create ECS client: %w", err)
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
		ecsClient: ecsClient,
		config:    cfg,
		logger:    log,
		retryer:   retryer,
	}, nil
}

// SetNetworkMonitor sets the network monitor for retry operations
func (c *Client) SetNetworkMonitor(monitor retry.NetworkMonitor) {
	c.retryer.SetNetworkMonitor(monitor)
}

// ListInstances lists ECS instances
func (c *Client) ListInstances(ctx context.Context) ([]InstanceInfo, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying list instances", "attempt", attempt)
		}

		request := ecs.CreateDescribeInstancesRequest()
		request.Scheme = "https"
		request.RegionId = c.config.RegionID

		response, err := c.ecsClient.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("describe instances: %w", err)
		}

		var instances []InstanceInfo
		for _, inst := range response.Instances.Instance {
			instances = append(instances, InstanceInfo{
				ID:           inst.InstanceId,
				Name:         inst.InstanceName,
				Status:       inst.Status,
				RegionID:     inst.RegionId,
				ZoneID:       inst.ZoneId,
				InstanceType: inst.InstanceType,
				CreatedTime:  inst.CreationTime,
			})
		}

		return instances, nil
	}, "list Alibaba Cloud instances")

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

		request := ecs.CreateDescribeInstancesRequest()
		request.Scheme = "https"
		request.InstanceIds = fmt.Sprintf("[\"%s\"]", instanceID)

		response, err := c.ecsClient.DescribeInstances(request)
		if err != nil {
			return nil, fmt.Errorf("describe instance: %w", err)
		}

		if len(response.Instances.Instance) == 0 {
			return nil, retry.IsNonRetryable(fmt.Errorf("instance not found: %s", instanceID))
		}

		inst := response.Instances.Instance[0]
		info := &InstanceInfo{
			ID:           inst.InstanceId,
			Name:         inst.InstanceName,
			Status:       inst.Status,
			RegionID:     inst.RegionId,
			ZoneID:       inst.ZoneId,
			InstanceType: inst.InstanceType,
			CreatedTime:  inst.CreationTime,
		}

		return info, nil
	}, fmt.Sprintf("get Alibaba Cloud instance %s", instanceID))

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

		request := ecs.CreateStopInstanceRequest()
		request.Scheme = "https"
		request.InstanceId = instanceID

		_, err := c.ecsClient.StopInstance(request)
		if err != nil {
			return fmt.Errorf("stop instance: %w", err)
		}

		c.logger.Info("instance stopped", "instance_id", instanceID)
		return nil
	}, fmt.Sprintf("stop Alibaba Cloud instance %s", instanceID))
}

// StartInstance starts a stopped instance
func (c *Client) StartInstance(ctx context.Context, instanceID string) error {
	return c.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			c.logger.Info("retrying start instance", "instance_id", instanceID, "attempt", attempt)
		}

		request := ecs.CreateStartInstanceRequest()
		request.Scheme = "https"
		request.InstanceId = instanceID

		_, err := c.ecsClient.StartInstance(request)
		if err != nil {
			return fmt.Errorf("start instance: %w", err)
		}

		c.logger.Info("instance started", "instance_id", instanceID)
		return nil
	}, fmt.Sprintf("start Alibaba Cloud instance %s", instanceID))
}

// CreateSnapshot creates a snapshot of a disk
func (c *Client) CreateSnapshot(ctx context.Context, diskID, snapshotName string) (string, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying create snapshot", "disk_id", diskID, "attempt", attempt)
		}

		request := ecs.CreateCreateSnapshotRequest()
		request.Scheme = "https"
		request.DiskId = diskID
		request.SnapshotName = snapshotName

		response, err := c.ecsClient.CreateSnapshot(request)
		if err != nil {
			return nil, fmt.Errorf("create snapshot: %w", err)
		}

		c.logger.Info("snapshot created", "disk_id", diskID, "snapshot_id", response.SnapshotId)
		return response.SnapshotId, nil
	}, fmt.Sprintf("create Alibaba Cloud snapshot %s", snapshotName))

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

// CreateImage creates a custom image from an instance
func (c *Client) CreateImage(ctx context.Context, instanceID, imageName string) (string, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying create image", "instance_id", instanceID, "attempt", attempt)
		}

		request := ecs.CreateCreateImageRequest()
		request.Scheme = "https"
		request.InstanceId = instanceID
		request.ImageName = imageName

		response, err := c.ecsClient.CreateImage(request)
		if err != nil {
			return nil, fmt.Errorf("create image: %w", err)
		}

		c.logger.Info("image created", "instance_id", instanceID, "image_id", response.ImageId)
		return response.ImageId, nil
	}, fmt.Sprintf("create Alibaba Cloud image %s", imageName))

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

// WaitForImageReady waits for an image to be ready
func (c *Client) WaitForImageReady(ctx context.Context, imageID string, timeout time.Duration) error {
	c.logger.Info("waiting for image to be ready",
		"image_id", imageID,
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
				return fmt.Errorf("timeout waiting for image to be ready")
			}

			request := ecs.CreateDescribeImagesRequest()
			request.Scheme = "https"
			request.ImageId = imageID

			response, err := c.ecsClient.DescribeImages(request)
			if err != nil {
				return fmt.Errorf("describe image: %w", err)
			}

			if len(response.Images.Image) == 0 {
				return fmt.Errorf("image not found: %s", imageID)
			}

			image := response.Images.Image[0]
			c.logger.Debug("image status check",
				"image_id", imageID,
				"status", image.Status)

			if image.Status == "Available" {
				c.logger.Info("image is ready", "image_id", imageID)
				return nil
			}

			if image.Status == "CreateFailed" {
				return fmt.Errorf("image creation failed")
			}
		}
	}
}

// DeleteImage deletes a custom image
func (c *Client) DeleteImage(ctx context.Context, imageID string) error {
	return c.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			c.logger.Info("retrying delete image", "image_id", imageID, "attempt", attempt)
		}

		request := ecs.CreateDeleteImageRequest()
		request.Scheme = "https"
		request.ImageId = imageID

		_, err := c.ecsClient.DeleteImage(request)
		if err != nil {
			// Ignore not found errors
			if isNotFoundError(err) {
				return nil
			}
			return fmt.Errorf("delete image: %w", err)
		}

		c.logger.Info("image deleted", "image_id", imageID)
		return nil
	}, fmt.Sprintf("delete Alibaba Cloud image %s", imageID))
}

// InstanceInfo contains information about an Alibaba Cloud ECS instance
type InstanceInfo struct {
	ID           string
	Name         string
	Status       string
	RegionID     string
	ZoneID       string
	InstanceType string
	CreatedTime  string
}

// isNotFoundError checks if an error is a not found error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Alibaba Cloud SDK errors contain specific codes
	errMsg := err.Error()
	return contains(errMsg, "InvalidImageId.NotFound") ||
		contains(errMsg, "InvalidInstanceId.NotFound") ||
		contains(errMsg, "InvalidSnapshotId.NotFound")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)+1 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
