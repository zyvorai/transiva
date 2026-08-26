// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// Client wraps OCI Compute SDK for instance management
type Client struct {
	computeClient core.ComputeClient
	config        *config.OCIConfig
	logger        logger.Logger
	retryer       *retry.Retryer
}

// NewClient creates a new OCI client
func NewClient(cfg *config.OCIConfig, log logger.Logger) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("OCI config is required")
	}

	// Create config provider
	configProvider := common.NewRawConfigurationProvider(
		cfg.TenancyOCID,
		cfg.UserOCID,
		cfg.Region,
		cfg.Fingerprint,
		cfg.PrivateKeyPath,
		nil,
	)

	// Create Compute client
	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("create compute client: %w", err)
	}

	// Set region
	computeClient.SetRegion(cfg.Region)

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
		computeClient: computeClient,
		config:        cfg,
		logger:        log,
		retryer:       retryer,
	}, nil
}

// SetNetworkMonitor sets the network monitor for retry operations
func (c *Client) SetNetworkMonitor(monitor retry.NetworkMonitor) {
	c.retryer.SetNetworkMonitor(monitor)
}

// ListInstances lists compute instances in the compartment
func (c *Client) ListInstances(ctx context.Context) ([]InstanceInfo, error) {
	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying list instances", "attempt", attempt)
		}

		request := core.ListInstancesRequest{
			CompartmentId: common.String(c.config.CompartmentOCID),
		}

		response, err := c.computeClient.ListInstances(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("list instances: %w", err)
		}

		var instances []InstanceInfo
		for _, instance := range response.Items {
			instances = append(instances, InstanceInfo{
				ID:                 *instance.Id,
				Name:               *instance.DisplayName,
				State:              string(instance.LifecycleState),
				CompartmentOCID:    *instance.CompartmentId,
				AvailabilityDomain: *instance.AvailabilityDomain,
				Shape:              *instance.Shape,
				TimeCreated:        instance.TimeCreated.Time,
			})
		}

		return instances, nil
	}, "list OCI instances")

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

		request := core.GetInstanceRequest{
			InstanceId: common.String(instanceID),
		}

		response, err := c.computeClient.GetInstance(ctx, request)
		if err != nil {
			// Not found errors are not retryable
			if IsNotFoundError(err) {
				return nil, retry.IsNonRetryable(fmt.Errorf("instance not found: %w", err))
			}
			return nil, fmt.Errorf("get instance: %w", err)
		}

		instance := response.Instance
		info := &InstanceInfo{
			ID:                 *instance.Id,
			Name:               *instance.DisplayName,
			State:              string(instance.LifecycleState),
			CompartmentOCID:    *instance.CompartmentId,
			AvailabilityDomain: *instance.AvailabilityDomain,
			Shape:              *instance.Shape,
			TimeCreated:        instance.TimeCreated.Time,
		}

		return info, nil
	}, fmt.Sprintf("get OCI instance %s", instanceID))

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

		request := core.InstanceActionRequest{
			InstanceId: common.String(instanceID),
			Action:     core.InstanceActionActionStop,
		}

		_, err := c.computeClient.InstanceAction(ctx, request)
		if err != nil {
			return fmt.Errorf("stop instance: %w", err)
		}

		c.logger.Info("instance stopped", "instance_id", instanceID)
		return nil
	}, fmt.Sprintf("stop OCI instance %s", instanceID))
}

// StartInstance starts a stopped instance
func (c *Client) StartInstance(ctx context.Context, instanceID string) error {
	return c.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			c.logger.Info("retrying start instance", "instance_id", instanceID, "attempt", attempt)
		}

		request := core.InstanceActionRequest{
			InstanceId: common.String(instanceID),
			Action:     core.InstanceActionActionStart,
		}

		_, err := c.computeClient.InstanceAction(ctx, request)
		if err != nil {
			return fmt.Errorf("start instance: %w", err)
		}

		c.logger.Info("instance started", "instance_id", instanceID)
		return nil
	}, fmt.Sprintf("start OCI instance %s", instanceID))
}

// WaitForInstanceState waits for an instance to reach a specific state
func (c *Client) WaitForInstanceState(ctx context.Context, instanceID string, desiredState core.InstanceLifecycleStateEnum, timeout time.Duration) error {
	c.logger.Info("waiting for instance state",
		"instance_id", instanceID,
		"desired_state", desiredState,
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
				return fmt.Errorf("timeout waiting for instance state %s", desiredState)
			}

			instance, err := c.GetInstance(ctx, instanceID)
			if err != nil {
				return fmt.Errorf("get instance state: %w", err)
			}

			currentState := core.InstanceLifecycleStateEnum(instance.State)
			c.logger.Debug("instance state check",
				"instance_id", instanceID,
				"current_state", currentState,
				"desired_state", desiredState)

			if currentState == desiredState {
				c.logger.Info("instance reached desired state",
					"instance_id", instanceID,
					"state", desiredState)
				return nil
			}

			// Check for failed states
			if currentState == core.InstanceLifecycleStateTerminated ||
				currentState == core.InstanceLifecycleStateTerminating {
				return fmt.Errorf("instance is in failed state: %s", currentState)
			}
		}
	}
}

// InstanceInfo contains information about an OCI instance
type InstanceInfo struct {
	ID                 string
	Name               string
	State              string
	CompartmentOCID    string
	AvailabilityDomain string
	Shape              string
	TimeCreated        time.Time
}

// IsNotFoundError checks if an error is a not found error
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// OCI SDK returns specific error for not found
	// Check error message or type
	errMsg := err.Error()
	return contains(errMsg, "NotAuthorizedOrNotFound") ||
		contains(errMsg, "404")
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
