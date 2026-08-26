// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// ListResourcePools returns all resource pools
func (c *VSphereClient) ListResourcePools(ctx context.Context, pattern string) ([]ResourcePoolInfo, error) {
	if pattern == "" {
		pattern = "*"
	}

	// Use finder to get resource pools
	poolList, err := c.finder.ResourcePoolList(ctx, pattern)
	if err != nil {
		// Check if it's a "not found" error - return empty list instead of error
		if isNotFoundError(err) {
			return []ResourcePoolInfo{}, nil
		}
		return nil, fmt.Errorf("find resource pools: %w", err)
	}

	if len(poolList) == 0 {
		return []ResourcePoolInfo{}, nil
	}

	// Collect pool references for batch property retrieval
	var refs []types.ManagedObjectReference
	for _, pool := range poolList {
		refs = append(refs, pool.Reference())
	}

	// Define properties to retrieve
	var pools []mo.ResourcePool
	pc := property.DefaultCollector(c.client.Client)
	err = pc.Retrieve(ctx, refs, []string{
		"name",
		"summary",
		"config",
		"vm",
		"resourcePool", // Sub-pools
	}, &pools)

	if err != nil {
		return nil, fmt.Errorf("retrieve pool properties: %w", err)
	}

	// Build response
	result := make([]ResourcePoolInfo, 0, len(pools))
	for i, pool := range pools {
		poolInfo := ResourcePoolInfo{
			Name:        pool.Name,
			Path:        poolList[i].InventoryPath,
			NumVMs:      len(pool.Vm),
			NumSubPools: len(pool.ResourcePool),
		}

		// CPU/Memory configuration
		if pool.Config.CpuAllocation.Reservation != nil {
			poolInfo.CPUReservationMhz = *pool.Config.CpuAllocation.Reservation
		}
		if pool.Config.CpuAllocation.Limit != nil {
			poolInfo.CPULimitMhz = *pool.Config.CpuAllocation.Limit
		}
		if pool.Config.CpuAllocation.ExpandableReservation != nil {
			poolInfo.CPUExpandable = *pool.Config.CpuAllocation.ExpandableReservation
		}

		if pool.Config.MemoryAllocation.Reservation != nil {
			poolInfo.MemoryReservationMB = *pool.Config.MemoryAllocation.Reservation
		}
		if pool.Config.MemoryAllocation.Limit != nil {
			poolInfo.MemoryLimitMB = *pool.Config.MemoryAllocation.Limit
		}
		if pool.Config.MemoryAllocation.ExpandableReservation != nil {
			poolInfo.MemoryExpandable = *pool.Config.MemoryAllocation.ExpandableReservation
		}

		result = append(result, poolInfo)
	}

	c.logger.Info("listed resource pools", "count", len(result), "pattern", pattern)
	return result, nil
}

// CreateResourcePool creates a new resource pool
func (c *VSphereClient) CreateResourcePool(ctx context.Context, config ResourcePoolConfig) error {
	// Find parent (cluster or resource pool)
	var parent *object.ResourcePool
	var err error

	// Try to find as resource pool first, then as cluster
	parent, err = c.finder.ResourcePool(ctx, config.ParentPath)
	if err != nil {
		// Parent might be a cluster, get its root resource pool
		cluster, clusterErr := c.finder.ClusterComputeResource(ctx, config.ParentPath)
		if clusterErr != nil {
			return fmt.Errorf("find parent resource pool or cluster: %w", err)
		}
		parent, err = cluster.ResourcePool(ctx)
		if err != nil {
			return fmt.Errorf("get cluster resource pool: %w", err)
		}
	}

	// Build resource pool spec
	spec := types.ResourceConfigSpec{
		CpuAllocation:    c.buildAllocationInfo(config.CPUReservationMhz, config.CPULimitMhz, config.CPUExpandable, config.CPUShares, config.CPUSharesLevel),
		MemoryAllocation: c.buildAllocationInfo(config.MemoryReservationMB, config.MemoryLimitMB, config.MemoryExpandable, config.MemoryShares, config.MemorySharesLevel),
	}

	// Create resource pool
	_, err = parent.Create(ctx, config.Name, spec)
	if err != nil {
		return fmt.Errorf("create resource pool: %w", err)
	}

	c.logger.Info("created resource pool",
		"name", config.Name,
		"parent", config.ParentPath,
		"cpu_reserve", config.CPUReservationMhz,
		"mem_reserve", config.MemoryReservationMB)

	return nil
}

// UpdateResourcePool modifies resource pool settings
func (c *VSphereClient) UpdateResourcePool(ctx context.Context, poolName string, config ResourcePoolConfig) error {
	// Find resource pool
	pool, err := c.finder.ResourcePool(ctx, poolName)
	if err != nil {
		return fmt.Errorf("find resource pool: %w", err)
	}

	// Build resource config spec
	spec := types.ResourceConfigSpec{
		CpuAllocation:    c.buildAllocationInfo(config.CPUReservationMhz, config.CPULimitMhz, config.CPUExpandable, config.CPUShares, config.CPUSharesLevel),
		MemoryAllocation: c.buildAllocationInfo(config.MemoryReservationMB, config.MemoryLimitMB, config.MemoryExpandable, config.MemoryShares, config.MemorySharesLevel),
	}

	// Update configuration
	err = pool.UpdateConfig(ctx, config.Name, &spec)
	if err != nil {
		return fmt.Errorf("update resource pool: %w", err)
	}

	c.logger.Info("updated resource pool",
		"name", poolName,
		"cpu_reserve", config.CPUReservationMhz,
		"mem_reserve", config.MemoryReservationMB)

	return nil
}

// DeleteResourcePool removes a resource pool
func (c *VSphereClient) DeleteResourcePool(ctx context.Context, poolName string) error {
	// Find resource pool
	pool, err := c.finder.ResourcePool(ctx, poolName)
	if err != nil {
		return fmt.Errorf("find resource pool: %w", err)
	}

	// Destroy resource pool
	task, err := pool.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("destroy resource pool: %w", err)
	}

	// Wait for task completion
	err = task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait for destroy task: %w", err)
	}

	c.logger.Info("deleted resource pool", "name", poolName)
	return nil
}

// Helper function to build ResourceAllocationInfo
func (c *VSphereClient) buildAllocationInfo(reservation, limit int64, expandable bool, shares string, sharesLevel int32) types.ResourceAllocationInfo {
	info := types.ResourceAllocationInfo{
		Reservation:           &reservation,
		ExpandableReservation: &expandable,
	}

	// Set limit (-1 means unlimited)
	if limit >= 0 {
		info.Limit = &limit
	}

	// Set shares
	var sharesInfo types.SharesInfo
	switch shares {
	case "low":
		sharesInfo = types.SharesInfo{
			Level: types.SharesLevelLow,
		}
	case "normal":
		sharesInfo = types.SharesInfo{
			Level: types.SharesLevelNormal,
		}
	case "high":
		sharesInfo = types.SharesInfo{
			Level: types.SharesLevelHigh,
		}
	case "custom":
		sharesInfo = types.SharesInfo{
			Level:  types.SharesLevelCustom,
			Shares: sharesLevel,
		}
	default:
		// Default to normal
		sharesInfo = types.SharesInfo{
			Level: types.SharesLevelNormal,
		}
	}

	info.Shares = &sharesInfo
	return info
}
