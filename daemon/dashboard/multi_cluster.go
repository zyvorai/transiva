// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterConnection represents a connection to a Kubernetes cluster
type ClusterConnection struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Context       string                `json:"context"`
	Server        string                `json:"server"`
	Namespace     string                `json:"namespace"`
	Client        *kubernetes.Clientset `json:"-"`
	DynamicClient *DynamicK8sClient     `json:"-"`
	Config        *rest.Config          `json:"-"`
	Metrics       *K8sMetrics           `json:"metrics"`
	Connected     bool                  `json:"connected"`
	LastUpdated   time.Time             `json:"last_updated"`
	Error         string                `json:"error,omitempty"`
}

// MultiClusterManager manages connections to multiple Kubernetes clusters
type MultiClusterManager struct {
	clusters   map[string]*ClusterConnection
	clustersMu sync.RWMutex
	primary    string // Primary cluster ID
	enabled    bool
}

// NewMultiClusterManager creates a new multi-cluster manager
func NewMultiClusterManager() *MultiClusterManager {
	return &MultiClusterManager{
		clusters: make(map[string]*ClusterConnection),
		enabled:  false,
	}
}

// AddCluster adds a cluster connection from kubeconfig context
func (mcm *MultiClusterManager) AddCluster(id, name, context, namespace string) error {
	mcm.clustersMu.Lock()
	defer mcm.clustersMu.Unlock()

	// Load kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig for context %s: %w", context, err)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create client for context %s: %w", context, err)
	}

	// Create dynamic client for CRDs
	dynamicClient, err := NewDynamicK8sClient(config, namespace)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create cluster connection
	cluster := &ClusterConnection{
		ID:            id,
		Name:          name,
		Context:       context,
		Server:        config.Host,
		Namespace:     namespace,
		Client:        client,
		DynamicClient: dynamicClient,
		Config:        config,
		Connected:     true,
		LastUpdated:   time.Now(),
		Metrics: &K8sMetrics{
			Timestamp:       time.Now(),
			BackupJobs:      K8sResourceMetrics{ByProvider: make(map[string]int)},
			BackupSchedules: K8sResourceMetrics{ByProvider: make(map[string]int)},
			RestoreJobs:     K8sResourceMetrics{ByProvider: make(map[string]int)},
			VirtualMachines: K8sVMMetrics{ByNode: make(map[string]int)},
			VMTemplates:     K8sResourceMetrics{ByProvider: make(map[string]int)},
			VMSnapshots:     K8sResourceMetrics{ByProvider: make(map[string]int)},
			RecentBackups:   make([]K8sBackupJobInfo, 0),
			RecentRestores:  make([]K8sRestoreJobInfo, 0),
			ActiveSchedules: make([]K8sScheduleInfo, 0),
			RunningVMs:      make([]K8sVMInfo, 0),
			StoppedVMs:      make([]K8sVMInfo, 0),
			Templates:       make([]K8sTemplateInfo, 0),
			RecentSnapshots: make([]K8sSnapshotInfo, 0),
			CarbonStats:     K8sCarbonStats{Enabled: false},
			ClusterInfo:     K8sClusterInfo{Connected: true},
			StorageStats:    K8sStorageStats{BackupsByDest: make(map[string]int), SizeByDest: make(map[string]int64)},
			VMResourceStats: K8sVMResourceStats{VMsBySize: make(map[string]int)},
		},
	}

	mcm.clusters[id] = cluster
	mcm.enabled = true

	// Set as primary if it's the first cluster
	if mcm.primary == "" {
		mcm.primary = id
	}

	return nil
}

// RemoveCluster removes a cluster connection
func (mcm *MultiClusterManager) RemoveCluster(id string) error {
	mcm.clustersMu.Lock()
	defer mcm.clustersMu.Unlock()

	if _, exists := mcm.clusters[id]; !exists {
		return fmt.Errorf("cluster %s not found", id)
	}

	delete(mcm.clusters, id)

	// Update primary if removed
	if mcm.primary == id {
		mcm.primary = ""
		// Set first available cluster as primary
		for clusterID := range mcm.clusters {
			mcm.primary = clusterID
			break
		}
	}

	// Disable multi-cluster if no clusters left
	if len(mcm.clusters) == 0 {
		mcm.enabled = false
	}

	return nil
}

// GetCluster retrieves a cluster connection
func (mcm *MultiClusterManager) GetCluster(id string) (*ClusterConnection, error) {
	mcm.clustersMu.RLock()
	defer mcm.clustersMu.RUnlock()

	cluster, exists := mcm.clusters[id]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", id)
	}

	return cluster, nil
}

// ListClusters returns all cluster connections
func (mcm *MultiClusterManager) ListClusters() []*ClusterConnection {
	mcm.clustersMu.RLock()
	defer mcm.clustersMu.RUnlock()

	clusters := make([]*ClusterConnection, 0, len(mcm.clusters))
	for _, cluster := range mcm.clusters {
		clusters = append(clusters, cluster)
	}

	return clusters
}

// GetPrimaryCluster returns the primary cluster
func (mcm *MultiClusterManager) GetPrimaryCluster() (*ClusterConnection, error) {
	mcm.clustersMu.RLock()
	defer mcm.clustersMu.RUnlock()

	if mcm.primary == "" {
		return nil, fmt.Errorf("no primary cluster set")
	}

	cluster, exists := mcm.clusters[mcm.primary]
	if !exists {
		return nil, fmt.Errorf("primary cluster %s not found", mcm.primary)
	}

	return cluster, nil
}

// SetPrimaryCluster sets the primary cluster
func (mcm *MultiClusterManager) SetPrimaryCluster(id string) error {
	mcm.clustersMu.Lock()
	defer mcm.clustersMu.Unlock()

	if _, exists := mcm.clusters[id]; !exists {
		return fmt.Errorf("cluster %s not found", id)
	}

	mcm.primary = id
	return nil
}

// UpdateClusterMetrics updates metrics for a specific cluster
func (mcm *MultiClusterManager) UpdateClusterMetrics(ctx context.Context, id string) error {
	cluster, err := mcm.GetCluster(id)
	if err != nil {
		return err
	}

	if cluster.Client == nil || cluster.DynamicClient == nil {
		return fmt.Errorf("cluster %s not connected", id)
	}

	// Create a temporary K8sDashboard instance to collect metrics
	tempDash := &K8sDashboard{
		k8sClient:     cluster.Client,
		dynamicClient: cluster.DynamicClient,
		k8sConfig:     cluster.Config,
		k8sMetrics:    cluster.Metrics,
		namespace:     cluster.Namespace,
	}

	// Collect metrics using existing collection methods
	tempDash.collectK8sMetrics(ctx)

	// Update cluster info
	cluster.LastUpdated = time.Now()
	cluster.Connected = true
	cluster.Error = ""

	return nil
}

// UpdateAllClusters updates metrics for all clusters
func (mcm *MultiClusterManager) UpdateAllClusters(ctx context.Context) {
	mcm.clustersMu.RLock()
	clusters := make([]string, 0, len(mcm.clusters))
	for id := range mcm.clusters {
		clusters = append(clusters, id)
	}
	mcm.clustersMu.RUnlock()

	// Update clusters in parallel
	var wg sync.WaitGroup
	for _, id := range clusters {
		wg.Add(1)
		go func(clusterID string) {
			defer wg.Done()
			if err := mcm.UpdateClusterMetrics(ctx, clusterID); err != nil {
				mcm.clustersMu.Lock()
				if cluster, exists := mcm.clusters[clusterID]; exists {
					cluster.Connected = false
					cluster.Error = err.Error()
				}
				mcm.clustersMu.Unlock()
			}
		}(id)
	}

	wg.Wait()
}

// GetAggregatedMetrics returns aggregated metrics across all clusters
func (mcm *MultiClusterManager) GetAggregatedMetrics() *K8sMetrics {
	mcm.clustersMu.RLock()
	defer mcm.clustersMu.RUnlock()

	aggregated := &K8sMetrics{
		Timestamp:       time.Now(),
		BackupJobs:      K8sResourceMetrics{ByProvider: make(map[string]int)},
		BackupSchedules: K8sResourceMetrics{ByProvider: make(map[string]int)},
		RestoreJobs:     K8sResourceMetrics{ByProvider: make(map[string]int)},
		VirtualMachines: K8sVMMetrics{ByNode: make(map[string]int)},
		VMTemplates:     K8sResourceMetrics{ByProvider: make(map[string]int)},
		VMSnapshots:     K8sResourceMetrics{ByProvider: make(map[string]int)},
		RecentBackups:   make([]K8sBackupJobInfo, 0),
		RecentRestores:  make([]K8sRestoreJobInfo, 0),
		ActiveSchedules: make([]K8sScheduleInfo, 0),
		RunningVMs:      make([]K8sVMInfo, 0),
		StoppedVMs:      make([]K8sVMInfo, 0),
		Templates:       make([]K8sTemplateInfo, 0),
		RecentSnapshots: make([]K8sSnapshotInfo, 0),
		CarbonStats:     K8sCarbonStats{Enabled: false},
		ClusterInfo:     K8sClusterInfo{Connected: false},
		StorageStats:    K8sStorageStats{BackupsByDest: make(map[string]int), SizeByDest: make(map[string]int64)},
		VMResourceStats: K8sVMResourceStats{VMsBySize: make(map[string]int)},
	}

	connectedClusters := 0

	for _, cluster := range mcm.clusters {
		if !cluster.Connected || cluster.Metrics == nil {
			continue
		}

		connectedClusters++
		m := cluster.Metrics

		// Aggregate counts
		aggregated.BackupJobs.Total += m.BackupJobs.Total
		aggregated.BackupJobs.Pending += m.BackupJobs.Pending
		aggregated.BackupJobs.Running += m.BackupJobs.Running
		aggregated.BackupJobs.Completed += m.BackupJobs.Completed
		aggregated.BackupJobs.Failed += m.BackupJobs.Failed

		aggregated.VirtualMachines.Total += m.VirtualMachines.Total
		aggregated.VirtualMachines.Running += m.VirtualMachines.Running
		aggregated.VirtualMachines.Stopped += m.VirtualMachines.Stopped
		aggregated.VirtualMachines.Failed += m.VirtualMachines.Failed

		aggregated.RestoreJobs.Total += m.RestoreJobs.Total
		aggregated.RestoreJobs.Completed += m.RestoreJobs.Completed
		aggregated.RestoreJobs.Failed += m.RestoreJobs.Failed

		aggregated.VMTemplates.Total += m.VMTemplates.Total
		aggregated.VMSnapshots.Total += m.VMSnapshots.Total

		// Aggregate resource stats
		aggregated.VMResourceStats.TotalCPUs += m.VMResourceStats.TotalCPUs
		aggregated.VMResourceStats.TotalMemoryGi += m.VMResourceStats.TotalMemoryGi
		aggregated.VMResourceStats.TotalDiskSize += m.VMResourceStats.TotalDiskSize
		aggregated.VMResourceStats.CarbonAwareVMs += m.VMResourceStats.CarbonAwareVMs

		// Merge VMs lists
		aggregated.RunningVMs = append(aggregated.RunningVMs, m.RunningVMs...)
		aggregated.StoppedVMs = append(aggregated.StoppedVMs, m.StoppedVMs...)
		aggregated.Templates = append(aggregated.Templates, m.Templates...)
		aggregated.RecentSnapshots = append(aggregated.RecentSnapshots, m.RecentSnapshots...)
		aggregated.RecentBackups = append(aggregated.RecentBackups, m.RecentBackups...)
		aggregated.RecentRestores = append(aggregated.RecentRestores, m.RecentRestores...)
	}

	// Calculate averages
	if connectedClusters > 0 {
		aggregated.ClusterInfo.Connected = true
		if aggregated.VirtualMachines.Total > 0 {
			aggregated.VMResourceStats.AvgCPUsPerVM = float64(aggregated.VMResourceStats.TotalCPUs) / float64(aggregated.VirtualMachines.Total)
			aggregated.VMResourceStats.AvgMemoryPerVMGi = aggregated.VMResourceStats.TotalMemoryGi / float64(aggregated.VirtualMachines.Total)
		}
	}

	// Set cluster count
	aggregated.ClusterInfo.NodeCount = len(mcm.clusters)

	return aggregated
}

// IsEnabled returns whether multi-cluster mode is enabled
func (mcm *MultiClusterManager) IsEnabled() bool {
	mcm.clustersMu.RLock()
	defer mcm.clustersMu.RUnlock()
	return mcm.enabled
}

// GetClusterCount returns the number of configured clusters
func (mcm *MultiClusterManager) GetClusterCount() int {
	mcm.clustersMu.RLock()
	defer mcm.clustersMu.RUnlock()
	return len(mcm.clusters)
}
