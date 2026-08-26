// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// ListHosts returns information about ESXi hosts
func (c *VSphereClient) ListHosts(ctx context.Context, pattern string) ([]HostInfo, error) {
	if pattern == "" {
		pattern = "*"
	}

	// Use finder to get hosts
	hostList, err := c.finder.HostSystemList(ctx, pattern)
	if err != nil {
		// Check if it's a "not found" error - return empty list instead of error
		if isNotFoundError(err) {
			return []HostInfo{}, nil
		}
		return nil, fmt.Errorf("find hosts: %w", err)
	}

	if len(hostList) == 0 {
		return []HostInfo{}, nil
	}

	// Collect host references for batch property retrieval
	var refs []types.ManagedObjectReference
	for _, host := range hostList {
		refs = append(refs, host.Reference())
	}

	// Define properties to retrieve
	var hosts []mo.HostSystem
	pc := property.DefaultCollector(c.client.Client)
	err = pc.Retrieve(ctx, refs, []string{
		"name",
		"parent",
		"runtime.connectionState",
		"runtime.powerState",
		"hardware.cpuInfo",
		"hardware.systemInfo",
		"summary.hardware",
		"summary.config.product",
		"vm",
	}, &hosts)

	if err != nil {
		return nil, fmt.Errorf("retrieve host properties: %w", err)
	}

	// Build response
	result := make([]HostInfo, 0, len(hosts))
	for _, host := range hosts {
		hostInfo := HostInfo{
			Name:            host.Name,
			ConnectionState: string(host.Runtime.ConnectionState),
			PowerState:      string(host.Runtime.PowerState),
			NumVMs:          len(host.Vm),
		}

		// Get datacenter and cluster from parent hierarchy
		if host.Parent != nil {
			hostInfo.Path = hostList[0].InventoryPath
			// Parse path to extract datacenter and cluster
			// Path format: /Datacenter/host/ClusterName/hostname
			path := hostList[0].InventoryPath
			pathParts := splitPath(path)
			if len(pathParts) > 0 {
				hostInfo.Datacenter = pathParts[0]
			}
			if len(pathParts) > 2 {
				hostInfo.Cluster = pathParts[2]
			}
		}

		// CPU information
		if host.Hardware != nil {
			hostInfo.CPUCores = int32(host.Hardware.CpuInfo.NumCpuCores)
			hostInfo.CPUThreads = int32(host.Hardware.CpuInfo.NumCpuThreads)

			// Convert Hz to MHz. A real CPU frequency reported by vSphere is at most a
			// few GHz (well under int32's ~2.1 billion MHz range), but validate
			// defensively since Hz comes from the vCenter API response.
			mhz := host.Hardware.CpuInfo.Hz / 1000000
			if mhz > math.MaxInt32 || mhz < math.MinInt32 {
				return nil, fmt.Errorf("host %s reports out-of-range CPU frequency: %d MHz", host.Name, mhz)
			}
			hostInfo.CPUMhz = int32(mhz)
		}

		if host.Summary.Hardware != nil {
			hostInfo.MemoryMB = host.Summary.Hardware.MemorySize / (1024 * 1024) // Convert bytes to MB
			hostInfo.NumNics = int(host.Summary.Hardware.NumNics)
		}

		// System information
		if host.Hardware != nil {
			hostInfo.CPUModel = host.Hardware.SystemInfo.Model
		}

		// Product version
		if host.Summary.Config.Product != nil {
			hostInfo.Version = host.Summary.Config.Product.Version
			hostInfo.Build = host.Summary.Config.Product.Build
		}

		result = append(result, hostInfo)
	}

	c.logger.Info("listed hosts", "count", len(result), "pattern", pattern)
	return result, nil
}

// GetHostInfo returns information about a specific ESXi host by name
func (c *VSphereClient) GetHostInfo(ctx context.Context, hostName string) (*HostInfo, error) {
	if hostName == "" {
		return nil, fmt.Errorf("host name cannot be empty")
	}

	// Use ListHosts to get host information
	hosts, err := c.ListHosts(ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("host not found: %s", hostName)
	}

	// Return the first matching host
	return &hosts[0], nil
}

// ListClusters returns cluster information
func (c *VSphereClient) ListClusters(ctx context.Context, pattern string) ([]ClusterInfo, error) {
	if pattern == "" {
		pattern = "*"
	}

	// Use finder to get clusters
	clusterList, err := c.finder.ClusterComputeResourceList(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("find clusters: %w", err)
	}

	if len(clusterList) == 0 {
		return []ClusterInfo{}, nil
	}

	// Collect cluster references for batch property retrieval
	var refs []types.ManagedObjectReference
	for _, cluster := range clusterList {
		refs = append(refs, cluster.Reference())
	}

	// Define properties to retrieve
	var clusters []mo.ClusterComputeResource
	pc := property.DefaultCollector(c.client.Client)
	err = pc.Retrieve(ctx, refs, []string{
		"name",
		"summary",
		"configuration.drsConfig",
		"configuration.dasConfig",
		"host",
	}, &clusters)

	if err != nil {
		return nil, fmt.Errorf("retrieve cluster properties: %w", err)
	}

	// Build response
	result := make([]ClusterInfo, 0, len(clusters))
	for i, cluster := range clusters {
		clusterInfo := ClusterInfo{
			Name:     cluster.Name,
			Path:     clusterList[i].InventoryPath,
			NumHosts: len(cluster.Host),
		}

		// Summary information
		if cluster.Summary != nil {
			summary := cluster.Summary.GetComputeResourceSummary()
			if summary != nil {
				clusterInfo.TotalCPU = int64(summary.TotalCpu)
				clusterInfo.TotalMemory = summary.TotalMemory / (1024 * 1024) // Convert bytes to MB
				clusterInfo.EffectiveCPU = int64(summary.EffectiveCpu)
				clusterInfo.EffectiveMemory = summary.EffectiveMemory / (1024 * 1024)
				clusterInfo.NumCPUCores = int32(summary.NumCpuCores)
				clusterInfo.NumCPUThreads = int32(summary.NumCpuThreads)
			}
		}

		// DRS configuration
		if cluster.ConfigurationEx != nil {
			if config, ok := cluster.ConfigurationEx.(*types.ClusterConfigInfoEx); ok {
				if config.DrsConfig.Enabled != nil {
					clusterInfo.DRSEnabled = *config.DrsConfig.Enabled
					clusterInfo.DRSBehavior = string(config.DrsConfig.DefaultVmBehavior)
				}

				// HA configuration
				if config.DasConfig.Enabled != nil {
					clusterInfo.HAEnabled = *config.DasConfig.Enabled
				}
			}
		}

		result = append(result, clusterInfo)
	}

	c.logger.Info("listed clusters", "count", len(result), "pattern", pattern)
	return result, nil
}

// ListDatacenters returns datacenter information
func (c *VSphereClient) ListDatacenters(ctx context.Context) ([]DatacenterInfo, error) {
	// Use finder to get all datacenters
	dcList, err := c.finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("find datacenters: %w", err)
	}

	if len(dcList) == 0 {
		return []DatacenterInfo{}, nil
	}

	result := make([]DatacenterInfo, 0, len(dcList))

	for _, dc := range dcList {
		// Create a new finder for this datacenter
		dcFinder := find.NewFinder(c.client.Client)
		dcFinder.SetDatacenter(dc)

		dcInfo := DatacenterInfo{
			Name: dc.Name(),
			Path: dc.InventoryPath,
		}

		// Count clusters
		clusters, err := dcFinder.ClusterComputeResourceList(ctx, "*")
		if err == nil {
			dcInfo.NumClusters = len(clusters)
		}

		// Count hosts
		hosts, err := dcFinder.HostSystemList(ctx, "*")
		if err == nil {
			dcInfo.NumHosts = len(hosts)
		}

		// Count VMs
		vms, err := dcFinder.VirtualMachineList(ctx, "*")
		if err == nil {
			dcInfo.NumVMs = len(vms)
		}

		// Count datastores
		datastores, err := dcFinder.DatastoreList(ctx, "*")
		if err == nil {
			dcInfo.NumDatastores = len(datastores)
		}

		result = append(result, dcInfo)
	}

	c.logger.Info("listed datacenters", "count", len(result))
	return result, nil
}

// GetVCenterInfo returns vCenter server information
func (c *VSphereClient) GetVCenterInfo(ctx context.Context) (*VCenterInfo, error) {
	// Get About info from ServiceContent
	about := c.client.ServiceContent.About

	info := &VCenterInfo{
		Name:       about.Name,
		Version:    about.Version,
		Build:      about.Build,
		OSType:     about.OsType,
		APIType:    about.ApiType,
		APIVersion: about.ApiVersion,
		InstanceID: about.InstanceUuid,
	}

	c.logger.Info("retrieved vCenter info",
		"version", info.Version,
		"build", info.Build,
		"api_version", info.APIVersion)

	return info, nil
}

// Helper function to split inventory path
func splitPath(path string) []string {
	// Remove leading slash
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	var parts []string
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// isNotFoundError checks if an error is a "not found" error from govmomi finder
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}
