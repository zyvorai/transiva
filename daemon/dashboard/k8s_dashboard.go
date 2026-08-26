// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// safeInt32 converts an int64 field pulled from a Kubernetes object (spec/status)
// to int32, clamping to the int32 range instead of silently overflowing/wrapping.
func safeInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

// K8sMetrics holds Kubernetes-specific metrics
type K8sMetrics struct {
	Timestamp         time.Time            `json:"timestamp"`
	OperatorStatus    string               `json:"operator_status"`
	OperatorReplicas  int                  `json:"operator_replicas"`
	BackupJobs        K8sResourceMetrics   `json:"backup_jobs"`
	BackupSchedules   K8sResourceMetrics   `json:"backup_schedules"`
	RestoreJobs       K8sResourceMetrics   `json:"restore_jobs"`
	VirtualMachines   K8sVMMetrics         `json:"virtual_machines"`
	VMTemplates       K8sResourceMetrics   `json:"vm_templates"`
	VMSnapshots       K8sResourceMetrics   `json:"vm_snapshots"`
	RecentBackups     []K8sBackupJobInfo   `json:"recent_backups"`
	RecentRestores    []K8sRestoreJobInfo  `json:"recent_restores"`
	ActiveSchedules   []K8sScheduleInfo    `json:"active_schedules"`
	RunningVMs        []K8sVMInfo          `json:"running_vms"`
	StoppedVMs        []K8sVMInfo          `json:"stopped_vms"`
	Templates         []K8sTemplateInfo    `json:"templates"`
	RecentSnapshots   []K8sSnapshotInfo    `json:"recent_snapshots"`
	CarbonStats       K8sCarbonStats       `json:"carbon_stats"`
	ClusterInfo       K8sClusterInfo       `json:"cluster_info"`
	StorageStats      K8sStorageStats      `json:"storage_stats"`
	VMResourceStats   K8sVMResourceStats   `json:"vm_resource_stats"`
}

// K8sResourceMetrics holds metrics for a specific resource type
type K8sResourceMetrics struct {
	Total      int            `json:"total"`
	Pending    int            `json:"pending"`
	Running    int            `json:"running"`
	Completed  int            `json:"completed"`
	Failed     int            `json:"failed"`
	ByProvider map[string]int `json:"by_provider"`
}

// K8sBackupJobInfo represents a backup job
type K8sBackupJobInfo struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	VMName            string    `json:"vm_name"`
	Provider          string    `json:"provider"`
	Destination       string    `json:"destination"`
	Phase             string    `json:"phase"`
	Progress          int       `json:"progress"`
	Size              int64     `json:"size"`
	CarbonAware       bool      `json:"carbon_aware"`
	CarbonIntensity   float64   `json:"carbon_intensity,omitempty"`
	CreationTimestamp time.Time `json:"creation_timestamp"`
	CompletionTime    time.Time `json:"completion_time,omitempty"`
	Duration          float64   `json:"duration"`
	ErrorMessage      string    `json:"error_message,omitempty"`
}

// K8sRestoreJobInfo represents a restore job
type K8sRestoreJobInfo struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	VMName            string    `json:"vm_name"`
	Provider          string    `json:"provider"`
	SourceBackup      string    `json:"source_backup"`
	Phase             string    `json:"phase"`
	Progress          int       `json:"progress"`
	PowerOn           bool      `json:"power_on"`
	CreationTimestamp time.Time `json:"creation_timestamp"`
	CompletionTime    time.Time `json:"completion_time,omitempty"`
	Duration          float64   `json:"duration"`
	ErrorMessage      string    `json:"error_message,omitempty"`
}

// K8sScheduleInfo represents a backup schedule
type K8sScheduleInfo struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	Schedule          string    `json:"schedule"`
	Timezone          string    `json:"timezone"`
	Suspended         bool      `json:"suspended"`
	VMName            string    `json:"vm_name"`
	Provider          string    `json:"provider"`
	Destination       string    `json:"destination"`
	LastScheduleTime  time.Time `json:"last_schedule_time,omitempty"`
	NextScheduleTime  time.Time `json:"next_schedule_time,omitempty"`
	ActiveJobs        int       `json:"active_jobs"`
	SuccessfulJobs    int       `json:"successful_jobs"`
	FailedJobs        int       `json:"failed_jobs"`
}

// K8sCarbonStats holds carbon-aware statistics
type K8sCarbonStats struct {
	Enabled             bool    `json:"enabled"`
	TotalBackups        int     `json:"total_backups"`
	CarbonAwareBackups  int     `json:"carbon_aware_backups"`
	AvgIntensity        float64 `json:"avg_intensity"`
	EstimatedSavingsKg  float64 `json:"estimated_savings_kg"`
	DelayedBackups      int     `json:"delayed_backups"`
	AvgDelayHours       float64 `json:"avg_delay_hours"`
}

// K8sClusterInfo holds Kubernetes cluster information
type K8sClusterInfo struct {
	Connected       bool   `json:"connected"`
	ClusterName     string `json:"cluster_name"`
	Version         string `json:"version"`
	NodeCount       int    `json:"node_count"`
	NamespaceCount  int    `json:"namespace_count"`
	KubeVirtEnabled bool   `json:"kubevirt_enabled"`
	Endpoint        string `json:"endpoint"`
}

// K8sStorageStats holds storage statistics
type K8sStorageStats struct {
	TotalBackupSize  int64              `json:"total_backup_size"`
	BackupsByDest    map[string]int     `json:"backups_by_dest"`
	SizeByDest       map[string]int64   `json:"size_by_dest"`
	AvgBackupSize    int64              `json:"avg_backup_size"`
	LargestBackup    string             `json:"largest_backup"`
	LargestBackupSize int64             `json:"largest_backup_size"`
}

// K8sVMMetrics holds VM-specific metrics
type K8sVMMetrics struct {
	Total      int            `json:"total"`
	Running    int            `json:"running"`
	Stopped    int            `json:"stopped"`
	Creating   int            `json:"creating"`
	Migrating  int            `json:"migrating"`
	Failed     int            `json:"failed"`
	ByNode     map[string]int `json:"by_node"`
}

// K8sVMInfo represents a virtual machine
type K8sVMInfo struct {
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace"`
	Phase             string             `json:"phase"`
	CPUs              int32              `json:"cpus"`
	Memory            string             `json:"memory"`
	NodeName          string             `json:"node_name"`
	IPAddresses       []string           `json:"ip_addresses"`
	DiskCount         int                `json:"disk_count"`
	NetworkCount      int                `json:"network_count"`
	CreationTimestamp time.Time          `json:"creation_timestamp"`
	StartTime         time.Time          `json:"start_time,omitempty"`
	CarbonIntensity   float64            `json:"carbon_intensity,omitempty"`
	GuestAgentConnected bool             `json:"guest_agent_connected"`
	CPUUsage          string             `json:"cpu_usage,omitempty"`
	MemoryUsage       string             `json:"memory_usage,omitempty"`
	Conditions        []VMConditionInfo  `json:"conditions"`
}

// VMConditionInfo represents a VM condition
type VMConditionInfo struct {
	Type    string    `json:"type"`
	Status  string    `json:"status"`
	Reason  string    `json:"reason,omitempty"`
	Message string    `json:"message,omitempty"`
	Time    time.Time `json:"time,omitempty"`
}

// K8sTemplateInfo represents a VM template
type K8sTemplateInfo struct {
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	OSType      string    `json:"os_type"`
	OSVersion   string    `json:"os_version"`
	Version     string    `json:"version"`
	DefaultCPUs int32     `json:"default_cpus"`
	DefaultMemory string  `json:"default_memory"`
	Tags        []string  `json:"tags"`
	Ready       bool      `json:"ready"`
	UsageCount  int32     `json:"usage_count"`
}

// K8sSnapshotInfo represents a VM snapshot
type K8sSnapshotInfo struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	VMName            string    `json:"vm_name"`
	Phase             string    `json:"phase"`
	IncludeMemory     bool      `json:"include_memory"`
	Size              string    `json:"size,omitempty"`
	SizeBytes         int64     `json:"size_bytes,omitempty"`
	CreationTime      time.Time `json:"creation_time,omitempty"`
	ReadyToRestore    bool      `json:"ready_to_restore"`
	Description       string    `json:"description,omitempty"`
}

// K8sVMResourceStats holds VM resource usage statistics
type K8sVMResourceStats struct {
	TotalCPUs          int32   `json:"total_cpus"`
	TotalMemoryGi      float64 `json:"total_memory_gi"`
	AvgCPUsPerVM       float64 `json:"avg_cpus_per_vm"`
	AvgMemoryPerVMGi   float64 `json:"avg_memory_per_vm_gi"`
	TotalDiskSize      int64   `json:"total_disk_size"`
	VMsBySize          map[string]int `json:"vms_by_size"` // small, medium, large
	CarbonAwareVMs     int     `json:"carbon_aware_vms"`
	AvgCarbonIntensity float64 `json:"avg_carbon_intensity"`
}

// K8sDashboard extends the main dashboard with Kubernetes-specific features
type K8sDashboard struct {
	dashboard        *Dashboard
	k8sClient        *kubernetes.Clientset
	dynamicClient    *DynamicK8sClient
	k8sConfig        *rest.Config
	k8sMetrics       *K8sMetrics
	k8sMetricsMu     sync.RWMutex
	namespace        string
	wsHub            *K8sWebSocketHub
	metricsHistory   *MetricsHistory
	multiCluster     *MultiClusterManager
	multiClusterMode bool
	vncProxy         *VNCProxy
}

// NewK8sDashboard creates a new Kubernetes dashboard extension
func NewK8sDashboard(dashboard *Dashboard, kubeconfig string, namespace string) (*K8sDashboard, error) {
	k8sDash := &K8sDashboard{
		dashboard: dashboard,
		namespace: namespace,
		k8sMetrics: &K8sMetrics{
			Timestamp: time.Now(),
			BackupJobs: K8sResourceMetrics{
				ByProvider: make(map[string]int),
			},
			BackupSchedules: K8sResourceMetrics{
				ByProvider: make(map[string]int),
			},
			RestoreJobs: K8sResourceMetrics{
				ByProvider: make(map[string]int),
			},
			VirtualMachines: K8sVMMetrics{
				ByNode: make(map[string]int),
			},
			VMTemplates: K8sResourceMetrics{
				ByProvider: make(map[string]int),
			},
			VMSnapshots: K8sResourceMetrics{
				ByProvider: make(map[string]int),
			},
			RecentBackups:   make([]K8sBackupJobInfo, 0),
			RecentRestores:  make([]K8sRestoreJobInfo, 0),
			ActiveSchedules: make([]K8sScheduleInfo, 0),
			RunningVMs:      make([]K8sVMInfo, 0),
			StoppedVMs:      make([]K8sVMInfo, 0),
			Templates:       make([]K8sTemplateInfo, 0),
			RecentSnapshots: make([]K8sSnapshotInfo, 0),
			CarbonStats: K8sCarbonStats{
				Enabled: false,
			},
			ClusterInfo: K8sClusterInfo{
				Connected: false,
			},
			StorageStats: K8sStorageStats{
				BackupsByDest: make(map[string]int),
				SizeByDest:    make(map[string]int64),
			},
			VMResourceStats: K8sVMResourceStats{
				VMsBySize: make(map[string]int),
			},
		},
	}

	// Initialize WebSocket hub
	k8sDash.wsHub = NewK8sWebSocketHub(k8sDash, 100)

	// Initialize metrics history (30 days retention)
	// Use ./data/metrics_history.db for storage
	metricsHistory, err := NewMetricsHistory("./data/metrics_history.db", 30)
	if err != nil {
		// Non-fatal - just log and continue without history
		fmt.Printf("Warning: Failed to initialize metrics history: %v\n", err)
		metricsHistory, _ = NewMetricsHistory("", 30) // disabled
	}
	k8sDash.metricsHistory = metricsHistory

	// Initialize multi-cluster manager
	k8sDash.multiCluster = NewMultiClusterManager()
	k8sDash.multiClusterMode = false // Disabled by default

	// Try to connect to Kubernetes
	if err := k8sDash.connectToK8s(kubeconfig); err != nil {
		// Non-fatal - dashboard can still show UI without live data
		k8sDash.k8sMetrics.ClusterInfo.Connected = false
	}

	// Initialize VNC proxy if connected
	if k8sDash.k8sClient != nil && k8sDash.k8sConfig != nil {
		k8sDash.vncProxy = NewVNCProxy(k8sDash.k8sClient, k8sDash.k8sConfig)
	}

	return k8sDash, nil
}

// CSV Export Helper Functions

// writeVMsCSV writes VM list as CSV
func writeVMsCSV(w http.ResponseWriter, vms []K8sVMInfo) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{
		"Name", "Namespace", "Phase", "CPUs", "Memory", "Node",
		"IP Addresses", "Disks", "Networks", "Created",
		"Carbon Intensity", "Guest Agent", "CPU Usage", "Memory Usage",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data rows
	for _, vm := range vms {
		row := []string{
			vm.Name,
			vm.Namespace,
			vm.Phase,
			fmt.Sprintf("%d", vm.CPUs),
			vm.Memory,
			vm.NodeName,
			strings.Join(vm.IPAddresses, ";"),
			fmt.Sprintf("%d", vm.DiskCount),
			fmt.Sprintf("%d", vm.NetworkCount),
			vm.CreationTimestamp.Format(time.RFC3339),
			fmt.Sprintf("%.2f", vm.CarbonIntensity),
			fmt.Sprintf("%t", vm.GuestAgentConnected),
			vm.CPUUsage,
			vm.MemoryUsage,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// writeMetricsCSV writes metrics summary as CSV
func writeMetricsCSV(w http.ResponseWriter, metrics *K8sMetrics) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{"Metric", "Value"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write key metrics
	rows := [][]string{
		{"Timestamp", metrics.Timestamp.Format(time.RFC3339)},
		{"Operator Status", metrics.OperatorStatus},
		{"Operator Replicas", fmt.Sprintf("%d", metrics.OperatorReplicas)},
		{"Total VMs", fmt.Sprintf("%d", metrics.VirtualMachines.Total)},
		{"Running VMs", fmt.Sprintf("%d", metrics.VirtualMachines.Running)},
		{"Stopped VMs", fmt.Sprintf("%d", metrics.VirtualMachines.Stopped)},
		{"Failed VMs", fmt.Sprintf("%d", metrics.VirtualMachines.Failed)},
		{"Total Backups", fmt.Sprintf("%d", metrics.BackupJobs.Total)},
		{"Completed Backups", fmt.Sprintf("%d", metrics.BackupJobs.Completed)},
		{"Failed Backups", fmt.Sprintf("%d", metrics.BackupJobs.Failed)},
		{"Total Restores", fmt.Sprintf("%d", metrics.RestoreJobs.Total)},
		{"Completed Restores", fmt.Sprintf("%d", metrics.RestoreJobs.Completed)},
		{"Total Templates", fmt.Sprintf("%d", metrics.VMTemplates.Total)},
		{"Total Snapshots", fmt.Sprintf("%d", metrics.VMSnapshots.Total)},
		{"Total CPU Cores (VMs)", fmt.Sprintf("%d", metrics.VMResourceStats.TotalCPUs)},
		{"Total Memory (VMs)", fmt.Sprintf("%.2f Gi", metrics.VMResourceStats.TotalMemoryGi)},
		{"Avg Carbon Intensity (VMs)", fmt.Sprintf("%.2f gCO2/kWh", metrics.VMResourceStats.AvgCarbonIntensity)},
		{"Carbon-Aware VMs", fmt.Sprintf("%d", metrics.VMResourceStats.CarbonAwareVMs)},
		{"Carbon-Aware Backups", fmt.Sprintf("%d", metrics.CarbonStats.CarbonAwareBackups)},
		{"Avg Carbon Intensity (Backups)", fmt.Sprintf("%.2f gCO2/kWh", metrics.CarbonStats.AvgIntensity)},
	}

	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// connectToK8s establishes Kubernetes client connection
func (kd *K8sDashboard) connectToK8s(kubeconfig string) error {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		// Try in-cluster config first
		config, err = rest.InClusterConfig()
		if err != nil {
			// Fall back to default kubeconfig
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
			config, err = clientConfig.ClientConfig()
		}
	}

	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	kd.k8sConfig = config
	kd.k8sClient = clientset

	// Initialize dynamic client for CRD access
	dynamicClient, err := NewDynamicK8sClient(config, kd.namespace)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}
	kd.dynamicClient = dynamicClient

	// Update cluster info
	if err := kd.updateClusterInfo(context.Background()); err != nil {
		return err
	}

	kd.k8sMetrics.ClusterInfo.Connected = true
	return nil
}

// updateClusterInfo fetches and updates cluster information
func (kd *K8sDashboard) updateClusterInfo(ctx context.Context) error {
	if kd.k8sClient == nil {
		return nil
	}

	kd.k8sMetricsMu.Lock()
	defer kd.k8sMetricsMu.Unlock()

	// Get server version
	version, err := kd.k8sClient.Discovery().ServerVersion()
	if err == nil {
		kd.k8sMetrics.ClusterInfo.Version = version.GitVersion
	}

	// Count nodes
	nodes, err := kd.k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		kd.k8sMetrics.ClusterInfo.NodeCount = len(nodes.Items)
	}

	// Count namespaces
	namespaces, err := kd.k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err == nil {
		kd.k8sMetrics.ClusterInfo.NamespaceCount = len(namespaces.Items)
	}

	// Check operator status
	pods, err := kd.k8sClient.CoreV1().Pods("transiva-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app=transiva-operator",
	})
	if err == nil && len(pods.Items) > 0 {
		kd.k8sMetrics.OperatorReplicas = len(pods.Items)
		allRunning := true
		for _, pod := range pods.Items {
			if pod.Status.Phase != "Running" {
				allRunning = false
				break
			}
		}
		if allRunning {
			kd.k8sMetrics.OperatorStatus = "Running"
		} else {
			kd.k8sMetrics.OperatorStatus = "Degraded"
		}
	} else {
		kd.k8sMetrics.OperatorStatus = "Not Running"
		kd.k8sMetrics.OperatorReplicas = 0
	}

	return nil
}

// Start starts the Kubernetes dashboard features
func (kd *K8sDashboard) Start(ctx context.Context) error {
	// Start metrics collection
	go kd.updateMetrics(ctx)

	// Start WebSocket hub
	kd.wsHub.Start(ctx)

	return nil
}

// updateMetrics periodically updates Kubernetes metrics
func (kd *K8sDashboard) updateMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Snapshot ticker for historical data (every 5 minutes)
	snapshotTicker := time.NewTicker(5 * time.Minute)
	defer snapshotTicker.Stop()

	// Cleanup ticker for old data (daily)
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update multi-cluster metrics if enabled
			if kd.multiClusterMode && kd.multiCluster.IsEnabled() {
				kd.multiCluster.UpdateAllClusters(ctx)
			}

			// Update single cluster metrics
			if kd.k8sClient == nil {
				continue
			}

			kd.collectK8sMetrics(ctx)

		case <-snapshotTicker.C:
			// Record metrics snapshot for historical trends
			if kd.metricsHistory != nil && kd.metricsHistory.IsEnabled() {
				kd.k8sMetricsMu.RLock()
				if err := kd.metricsHistory.RecordSnapshot(kd.k8sMetrics); err != nil {
					fmt.Printf("Error recording metrics snapshot: %v\n", err)
				}
				kd.k8sMetricsMu.RUnlock()
			}

		case <-cleanupTicker.C:
			// Cleanup old historical data
			if kd.metricsHistory != nil && kd.metricsHistory.IsEnabled() {
				if err := kd.metricsHistory.CleanupOldData(); err != nil {
					fmt.Printf("Error cleaning up old metrics: %v\n", err)
				}
			}
		}
	}
}

// collectK8sMetrics collects Kubernetes resource metrics
func (kd *K8sDashboard) collectK8sMetrics(ctx context.Context) {
	kd.k8sMetricsMu.Lock()
	defer kd.k8sMetricsMu.Unlock()

	kd.k8sMetrics.Timestamp = time.Now()

	// Update cluster info
	if err := kd.updateClusterInfo(ctx); err != nil {
		fmt.Printf("Error updating cluster info: %v\n", err)
	}

	// Return early if dynamic client not available
	if kd.dynamicClient == nil {
		return
	}

	// Collect BackupJob metrics
	kd.collectBackupJobMetrics(ctx)

	// Collect BackupSchedule metrics
	kd.collectBackupScheduleMetrics(ctx)

	// Collect RestoreJob metrics
	kd.collectRestoreJobMetrics(ctx)

	// Collect VM metrics
	if err := kd.collectVMMetrics(ctx); err != nil {
		fmt.Printf("Error collecting VM metrics: %v\n", err)
	}

	// Collect VMTemplate metrics
	if err := kd.collectVMTemplateMetrics(ctx); err != nil {
		fmt.Printf("Error collecting VM template metrics: %v\n", err)
	}

	// Collect VMSnapshot metrics
	if err := kd.collectVMSnapshotMetrics(ctx); err != nil {
		fmt.Printf("Error collecting VM snapshot metrics: %v\n", err)
	}

	// Update carbon statistics
	kd.updateCarbonStatistics()

	// Update storage statistics
	kd.updateStorageStatistics()
}

// collectBackupJobMetrics collects BackupJob metrics from Kubernetes
func (kd *K8sDashboard) collectBackupJobMetrics(ctx context.Context) {
	backupJobs, err := kd.dynamicClient.ListBackupJobs(ctx)
	if err != nil {
		// Silently fail - CRDs might not be installed yet
		return
	}

	// Reset metrics
	kd.k8sMetrics.BackupJobs = K8sResourceMetrics{
		ByProvider: make(map[string]int),
	}
	kd.k8sMetrics.RecentBackups = make([]K8sBackupJobInfo, 0)

	for _, item := range backupJobs.Items {
		obj := item.Object

		// Extract spec
		spec := GetMapField(obj, "spec")
		if spec == nil {
			continue
		}

		// Extract status
		status := GetMapField(obj, "status")
		phase := GetStringField(obj, "status", "phase")

		// Count by phase
		kd.k8sMetrics.BackupJobs.Total++
		switch phase {
		case "Pending":
			kd.k8sMetrics.BackupJobs.Pending++
		case "Running":
			kd.k8sMetrics.BackupJobs.Running++
		case "Completed":
			kd.k8sMetrics.BackupJobs.Completed++
		case "Failed":
			kd.k8sMetrics.BackupJobs.Failed++
		}

		// Count by provider
		provider := GetStringField(obj, "spec", "source", "provider")
		if provider != "" {
			kd.k8sMetrics.BackupJobs.ByProvider[provider]++
		}

		// Extract backup job info
		backupInfo := K8sBackupJobInfo{
			Name:      GetStringField(obj, "metadata", "name"),
			Namespace: GetStringField(obj, "metadata", "namespace"),
			VMName:    GetStringField(obj, "spec", "source", "vmName"),
			Provider:  provider,
			Destination: GetStringField(obj, "spec", "destination", "type") + "://" +
				GetStringField(obj, "spec", "destination", "bucket"),
			Phase:    phase,
			Progress: int(GetInt64Field(obj, "status", "progress", "percentComplete")),
			Size:     GetInt64Field(obj, "status", "backupSize"),
		}

		// Check if carbon-aware
		carbonAware := GetMapField(obj, "spec", "carbonAware")
		if carbonAware != nil {
			backupInfo.CarbonAware = GetBoolField(obj, "spec", "carbonAware", "enabled")
			backupInfo.CarbonIntensity = GetFloat64Field(obj, "status", "carbonIntensity")
		}

		// Parse timestamps
		if creationTime := GetStringField(obj, "metadata", "creationTimestamp"); creationTime != "" {
			if t, err := time.Parse(time.RFC3339, creationTime); err == nil {
				backupInfo.CreationTimestamp = t
			}
		}

		if completionTime := GetStringField(obj, "status", "completionTime"); completionTime != "" {
			if t, err := time.Parse(time.RFC3339, completionTime); err == nil {
				backupInfo.CompletionTime = t
				backupInfo.Duration = t.Sub(backupInfo.CreationTimestamp).Seconds()
			}
		} else if phase == "Running" {
			backupInfo.Duration = time.Since(backupInfo.CreationTimestamp).Seconds()
		}

		// Error message if failed
		if phase == "Failed" {
			if conditions, ok := status["conditions"].([]interface{}); ok && len(conditions) > 0 {
				if lastCondition, ok := conditions[len(conditions)-1].(map[string]interface{}); ok {
					backupInfo.ErrorMessage = GetStringField(lastCondition, "message")
				}
			}
		}

		kd.k8sMetrics.RecentBackups = append(kd.k8sMetrics.RecentBackups, backupInfo)
	}

	// Keep only last 50 backups
	if len(kd.k8sMetrics.RecentBackups) > 50 {
		kd.k8sMetrics.RecentBackups = kd.k8sMetrics.RecentBackups[:50]
	}
}

// collectBackupScheduleMetrics collects BackupSchedule metrics from Kubernetes
func (kd *K8sDashboard) collectBackupScheduleMetrics(ctx context.Context) {
	schedules, err := kd.dynamicClient.ListBackupSchedules(ctx)
	if err != nil {
		return
	}

	// Reset metrics
	kd.k8sMetrics.BackupSchedules = K8sResourceMetrics{
		ByProvider: make(map[string]int),
	}
	kd.k8sMetrics.ActiveSchedules = make([]K8sScheduleInfo, 0)

	for _, item := range schedules.Items {
		obj := item.Object

		// Extract spec
		suspended := GetBoolField(obj, "spec", "suspend")

		kd.k8sMetrics.BackupSchedules.Total++
		if !suspended {
			kd.k8sMetrics.BackupSchedules.Running++
		}

		// Count by provider
		provider := GetStringField(obj, "spec", "jobTemplate", "spec", "source", "provider")
		if provider != "" {
			kd.k8sMetrics.BackupSchedules.ByProvider[provider]++
		}

		// Extract schedule info
		scheduleInfo := K8sScheduleInfo{
			Name:      GetStringField(obj, "metadata", "name"),
			Namespace: GetStringField(obj, "metadata", "namespace"),
			Schedule:  GetStringField(obj, "spec", "schedule"),
			Timezone:  GetStringField(obj, "spec", "timezone"),
			Suspended: suspended,
			VMName:    GetStringField(obj, "spec", "jobTemplate", "spec", "source", "vmName"),
			Provider:  provider,
			Destination: GetStringField(obj, "spec", "jobTemplate", "spec", "destination", "type") + "://" +
				GetStringField(obj, "spec", "jobTemplate", "spec", "destination", "bucket"),
			ActiveJobs:     int(GetInt64Field(obj, "status", "active")),
			SuccessfulJobs: int(GetInt64Field(obj, "status", "successfulJobs")),
			FailedJobs:     int(GetInt64Field(obj, "status", "failedJobs")),
		}

		// Parse timestamps
		if lastScheduleTime := GetStringField(obj, "status", "lastScheduleTime"); lastScheduleTime != "" {
			if t, err := time.Parse(time.RFC3339, lastScheduleTime); err == nil {
				scheduleInfo.LastScheduleTime = t
			}
		}

		if nextScheduleTime := GetStringField(obj, "status", "nextScheduleTime"); nextScheduleTime != "" {
			if t, err := time.Parse(time.RFC3339, nextScheduleTime); err == nil {
				scheduleInfo.NextScheduleTime = t
			}
		}

		kd.k8sMetrics.ActiveSchedules = append(kd.k8sMetrics.ActiveSchedules, scheduleInfo)
	}
}

// collectRestoreJobMetrics collects RestoreJob metrics from Kubernetes
func (kd *K8sDashboard) collectRestoreJobMetrics(ctx context.Context) {
	restoreJobs, err := kd.dynamicClient.ListRestoreJobs(ctx)
	if err != nil {
		return
	}

	// Reset metrics
	kd.k8sMetrics.RestoreJobs = K8sResourceMetrics{
		ByProvider: make(map[string]int),
	}
	kd.k8sMetrics.RecentRestores = make([]K8sRestoreJobInfo, 0)

	for _, item := range restoreJobs.Items {
		obj := item.Object

		phase := GetStringField(obj, "status", "phase")

		kd.k8sMetrics.RestoreJobs.Total++
		switch phase {
		case "Pending":
			kd.k8sMetrics.RestoreJobs.Pending++
		case "Running":
			kd.k8sMetrics.RestoreJobs.Running++
		case "Completed":
			kd.k8sMetrics.RestoreJobs.Completed++
		case "Failed":
			kd.k8sMetrics.RestoreJobs.Failed++
		}

		// Count by provider
		provider := GetStringField(obj, "spec", "destination", "provider")
		if provider != "" {
			kd.k8sMetrics.RestoreJobs.ByProvider[provider]++
		}

		// Extract restore job info
		restoreInfo := K8sRestoreJobInfo{
			Name:         GetStringField(obj, "metadata", "name"),
			Namespace:    GetStringField(obj, "metadata", "namespace"),
			VMName:       GetStringField(obj, "spec", "destination", "vmName"),
			Provider:     provider,
			SourceBackup: GetStringField(obj, "spec", "source", "backupJobRef", "name"),
			Phase:        phase,
			Progress:     int(GetInt64Field(obj, "status", "progress", "percentComplete")),
			PowerOn:      GetBoolField(obj, "spec", "options", "powerOnAfterRestore"),
		}

		// Parse timestamps
		if creationTime := GetStringField(obj, "metadata", "creationTimestamp"); creationTime != "" {
			if t, err := time.Parse(time.RFC3339, creationTime); err == nil {
				restoreInfo.CreationTimestamp = t
			}
		}

		if completionTime := GetStringField(obj, "status", "completionTime"); completionTime != "" {
			if t, err := time.Parse(time.RFC3339, completionTime); err == nil {
				restoreInfo.CompletionTime = t
				restoreInfo.Duration = t.Sub(restoreInfo.CreationTimestamp).Seconds()
			}
		} else if phase == "Running" {
			restoreInfo.Duration = time.Since(restoreInfo.CreationTimestamp).Seconds()
		}

		// Error message if failed
		if phase == "Failed" {
			status := GetMapField(obj, "status")
			if conditions, ok := status["conditions"].([]interface{}); ok && len(conditions) > 0 {
				if lastCondition, ok := conditions[len(conditions)-1].(map[string]interface{}); ok {
					restoreInfo.ErrorMessage = GetStringField(lastCondition, "message")
				}
			}
		}

		kd.k8sMetrics.RecentRestores = append(kd.k8sMetrics.RecentRestores, restoreInfo)
	}

	// Keep only last 50 restores
	if len(kd.k8sMetrics.RecentRestores) > 50 {
		kd.k8sMetrics.RecentRestores = kd.k8sMetrics.RecentRestores[:50]
	}
}

// updateCarbonStatistics calculates carbon-aware statistics
func (kd *K8sDashboard) updateCarbonStatistics() {
	stats := &kd.k8sMetrics.CarbonStats

	// Reset stats
	stats.TotalBackups = len(kd.k8sMetrics.RecentBackups)
	stats.CarbonAwareBackups = 0
	var totalIntensity float64
	var totalDelay float64
	stats.DelayedBackups = 0

	for _, backup := range kd.k8sMetrics.RecentBackups {
		if backup.CarbonAware {
			stats.CarbonAwareBackups++
			if backup.CarbonIntensity > 0 {
				totalIntensity += backup.CarbonIntensity
			}
		}

		// Check if backup was delayed (creation time vs scheduled time)
		// This would need additional fields in the CRD status
	}

	if stats.CarbonAwareBackups > 0 {
		stats.Enabled = true
		stats.AvgIntensity = totalIntensity / float64(stats.CarbonAwareBackups)

		// Estimate CO2 savings (rough calculation)
		// Assume average backup uses 10 kWh, baseline intensity 400 gCO2/kWh
		baselineIntensity := 400.0
		if stats.AvgIntensity < baselineIntensity {
			savingsPerBackup := (baselineIntensity - stats.AvgIntensity) * 10.0 / 1000.0 // kg
			stats.EstimatedSavingsKg = savingsPerBackup * float64(stats.CarbonAwareBackups)
		}
	}

	if stats.DelayedBackups > 0 {
		stats.AvgDelayHours = totalDelay / float64(stats.DelayedBackups)
	}
}

// updateStorageStatistics calculates storage statistics
func (kd *K8sDashboard) updateStorageStatistics() {
	stats := &kd.k8sMetrics.StorageStats

	// Reset stats
	stats.TotalBackupSize = 0
	stats.BackupsByDest = make(map[string]int)
	stats.SizeByDest = make(map[string]int64)
	stats.LargestBackupSize = 0

	for _, backup := range kd.k8sMetrics.RecentBackups {
		if backup.Size > 0 {
			stats.TotalBackupSize += backup.Size

			// Track by destination
			dest := backup.Destination
			stats.BackupsByDest[dest]++
			stats.SizeByDest[dest] += backup.Size

			// Track largest
			if backup.Size > stats.LargestBackupSize {
				stats.LargestBackupSize = backup.Size
				stats.LargestBackup = backup.Name
			}
		}
	}

	if len(kd.k8sMetrics.RecentBackups) > 0 {
		stats.AvgBackupSize = stats.TotalBackupSize / int64(len(kd.k8sMetrics.RecentBackups))
	}
}

// RegisterHandlers registers Kubernetes-specific HTTP handlers
func (kd *K8sDashboard) RegisterHandlers(mux *http.ServeMux) {
	// API endpoints for Kubernetes resources
	mux.HandleFunc("/api/k8s/metrics", kd.handleK8sMetrics)
	mux.HandleFunc("/api/k8s/backupjobs", kd.handleBackupJobs)
	mux.HandleFunc("/api/k8s/backupjobs/", kd.handleBackupJobDetail)
	mux.HandleFunc("/api/k8s/backupschedules", kd.handleBackupSchedules)
	mux.HandleFunc("/api/k8s/restorejobs", kd.handleRestoreJobs)
	mux.HandleFunc("/api/k8s/carbon", kd.handleCarbonStats)
	mux.HandleFunc("/api/k8s/cluster", kd.handleClusterInfo)
	mux.HandleFunc("/api/k8s/storage", kd.handleStorageStats)

	// VM endpoints
	mux.HandleFunc("/api/k8s/vms", kd.handleVMs)
	mux.HandleFunc("/api/k8s/vms/", kd.handleVMDetail)
	mux.HandleFunc("/api/k8s/vm-metrics", kd.handleVMMetrics)
	mux.HandleFunc("/api/k8s/templates", kd.handleTemplates)
	mux.HandleFunc("/api/k8s/snapshots", kd.handleSnapshots)

	// Historical metrics endpoints
	mux.HandleFunc("/api/k8s/history", kd.handleMetricsHistory)
	mux.HandleFunc("/api/k8s/trends", kd.handleMetricsTrends)

	// Multi-cluster endpoints
	mux.HandleFunc("/api/k8s/clusters", kd.handleClusters)
	mux.HandleFunc("/api/k8s/clusters/", kd.handleClusterDetail)
	mux.HandleFunc("/api/k8s/clusters/switch", kd.handleClusterSwitch)
	mux.HandleFunc("/api/k8s/aggregated-metrics", kd.handleAggregatedMetrics)

	// Console endpoints
	mux.HandleFunc("/api/k8s/console-sessions", kd.handleConsoleSessions)
	mux.HandleFunc("/ws/console", kd.handleConsoleWebSocket)

	// WebSocket endpoint for real-time updates
	mux.HandleFunc("/ws/k8s", kd.wsHub.HandleWebSocket)
}

// handleK8sMetrics serves Kubernetes metrics
func (kd *K8sDashboard) handleK8sMetrics(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	metrics := *kd.k8sMetrics
	kd.k8sMetricsMu.RUnlock()

	// Check for export format
	format := r.URL.Query().Get("format")
	download := r.URL.Query().Get("download")

	switch format {
	case "csv":
		if download == "true" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=k8s-metrics-%s.csv", time.Now().Format("2006-01-02")))
		}
		w.Header().Set("Content-Type", "text/csv")
		if err := writeMetricsCSV(w, &metrics); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		// Default to JSON
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			fmt.Printf("Error encoding metrics response: %v\n", err)
		}
	}
}

// handleBackupJobs serves BackupJobs list
func (kd *K8sDashboard) handleBackupJobs(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	backups := kd.k8sMetrics.RecentBackups
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(backups); err != nil {
		fmt.Printf("Error encoding backups response: %v\n", err)
	}
}

// handleBackupJobDetail serves BackupJob details
func (kd *K8sDashboard) handleBackupJobDetail(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/api/k8s/backupjobs/"):]

	kd.k8sMetricsMu.RLock()
	defer kd.k8sMetricsMu.RUnlock()

	for _, backup := range kd.k8sMetrics.RecentBackups {
		if backup.Name == name {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(backup); err != nil {
				fmt.Printf("Error encoding backup response: %v\n", err)
			}
			return
		}
	}

	http.NotFound(w, r)
}

// handleBackupSchedules serves BackupSchedules list
func (kd *K8sDashboard) handleBackupSchedules(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	schedules := kd.k8sMetrics.ActiveSchedules
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(schedules); err != nil {
		fmt.Printf("Error encoding schedules response: %v\n", err)
	}
}

// handleRestoreJobs serves RestoreJobs list
func (kd *K8sDashboard) handleRestoreJobs(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	restores := kd.k8sMetrics.RecentRestores
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(restores); err != nil {
		fmt.Printf("Error encoding restores response: %v\n", err)
	}
}

// handleCarbonStats serves carbon statistics
func (kd *K8sDashboard) handleCarbonStats(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	stats := kd.k8sMetrics.CarbonStats
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		fmt.Printf("Error encoding carbon stats response: %v\n", err)
	}
}

// handleClusterInfo serves cluster information
func (kd *K8sDashboard) handleClusterInfo(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	info := kd.k8sMetrics.ClusterInfo
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		fmt.Printf("Error encoding cluster info response: %v\n", err)
	}
}

// handleStorageStats serves storage statistics
func (kd *K8sDashboard) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	stats := kd.k8sMetrics.StorageStats
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		fmt.Printf("Error encoding storage stats response: %v\n", err)
	}
}

// handleVMs serves VirtualMachines list
func (kd *K8sDashboard) handleVMs(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	vms := append(kd.k8sMetrics.RunningVMs, kd.k8sMetrics.StoppedVMs...)
	kd.k8sMetricsMu.RUnlock()

	// Check for export format
	format := r.URL.Query().Get("format")
	download := r.URL.Query().Get("download")

	switch format {
	case "csv":
		if download == "true" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=vms-%s.csv", time.Now().Format("2006-01-02")))
		}
		w.Header().Set("Content-Type", "text/csv")
		if err := writeVMsCSV(w, vms); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		// Default to JSON
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(vms); err != nil {
			fmt.Printf("Error encoding VMs response: %v\n", err)
		}
	}
}

// handleVMDetail serves VirtualMachine details
func (kd *K8sDashboard) handleVMDetail(w http.ResponseWriter, r *http.Request) {
	// Extract namespace/name from path: /api/k8s/vms/{namespace}/{name}
	path := r.URL.Path[len("/api/k8s/vms/"):]
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	namespace := parts[0]
	name := parts[1]

	kd.k8sMetricsMu.RLock()
	defer kd.k8sMetricsMu.RUnlock()

	// Search in running VMs
	for _, vm := range kd.k8sMetrics.RunningVMs {
		if vm.Namespace == namespace && vm.Name == name {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(vm); err != nil {
				fmt.Printf("Error encoding VM response: %v\n", err)
			}
			return
		}
	}

	// Search in stopped VMs
	for _, vm := range kd.k8sMetrics.StoppedVMs {
		if vm.Namespace == namespace && vm.Name == name {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(vm); err != nil {
				fmt.Printf("Error encoding VM response: %v\n", err)
			}
			return
		}
	}

	http.NotFound(w, r)
}

// handleVMMetrics serves VM resource metrics
func (kd *K8sDashboard) handleVMMetrics(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	metrics := struct {
		VMs K8sVMMetrics       `json:"vms"`
		Resources K8sVMResourceStats `json:"resources"`
	}{
		VMs:       kd.k8sMetrics.VirtualMachines,
		Resources: kd.k8sMetrics.VMResourceStats,
	}
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		fmt.Printf("Error encoding VM metrics response: %v\n", err)
	}
}

// handleTemplates serves VMTemplates list
func (kd *K8sDashboard) handleTemplates(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	templates := kd.k8sMetrics.Templates
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(templates); err != nil {
		fmt.Printf("Error encoding templates response: %v\n", err)
	}
}

// handleSnapshots serves VMSnapshots list
func (kd *K8sDashboard) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	kd.k8sMetricsMu.RLock()
	snapshots := kd.k8sMetrics.RecentSnapshots
	kd.k8sMetricsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snapshots); err != nil {
		fmt.Printf("Error encoding snapshots response: %v\n", err)
	}
}

// collectVMMetrics collects VirtualMachine metrics
func (kd *K8sDashboard) collectVMMetrics(ctx context.Context) error {
	if kd.dynamicClient == nil {
		return nil
	}

	vms, err := kd.dynamicClient.GetVirtualMachines(ctx, kd.namespace)
	if err != nil {
		return err
	}

	// Reset metrics
	kd.k8sMetrics.VirtualMachines = K8sVMMetrics{
		ByNode: make(map[string]int),
	}
	kd.k8sMetrics.RunningVMs = make([]K8sVMInfo, 0)
	kd.k8sMetrics.StoppedVMs = make([]K8sVMInfo, 0)

	for _, obj := range vms {
		phase := GetStringField(obj, "status", "phase")
		nodeName := GetStringField(obj, "status", "nodeName")

		// Count by phase
		kd.k8sMetrics.VirtualMachines.Total++
		switch phase {
		case "Running":
			kd.k8sMetrics.VirtualMachines.Running++
		case "Stopped":
			kd.k8sMetrics.VirtualMachines.Stopped++
		case "Creating":
			kd.k8sMetrics.VirtualMachines.Creating++
		case "Migrating":
			kd.k8sMetrics.VirtualMachines.Migrating++
		case "Failed":
			kd.k8sMetrics.VirtualMachines.Failed++
		}

		// Count by node
		if nodeName != "" {
			kd.k8sMetrics.VirtualMachines.ByNode[nodeName]++
		}

		// Build VM info
		vmInfo := K8sVMInfo{
			Name:      GetStringField(obj, "metadata", "name"),
			Namespace: GetStringField(obj, "metadata", "namespace"),
			Phase:     phase,
			CPUs:      safeInt32(GetInt64Field(obj, "spec", "cpus")),
			Memory:    GetStringField(obj, "spec", "memory"),
			NodeName:  nodeName,
			CarbonIntensity: GetFloat64Field(obj, "status", "carbonIntensity"),
		}

		// Parse IP addresses
		if ipAddrs := GetArrayField(obj, "status", "ipAddresses"); len(ipAddrs) > 0 {
			vmInfo.IPAddresses = make([]string, 0, len(ipAddrs))
			for _, ip := range ipAddrs {
				if ipStr, ok := ip.(string); ok {
					vmInfo.IPAddresses = append(vmInfo.IPAddresses, ipStr)
				}
			}
		}

		// Parse disks and networks
		if disks := GetArrayField(obj, "spec", "disks"); len(disks) > 0 {
			vmInfo.DiskCount = len(disks)
		}
		if networks := GetArrayField(obj, "spec", "networks"); len(networks) > 0 {
			vmInfo.NetworkCount = len(networks)
		}

		// Parse guest agent status
		if guestAgent := GetMapField(obj, "status", "guestAgent"); guestAgent != nil {
			vmInfo.GuestAgentConnected = GetBoolField(guestAgent, "connected")
		}

		// Parse resource usage
		if resources := GetMapField(obj, "status", "resources"); resources != nil {
			if cpu := GetMapField(resources, "cpu"); cpu != nil {
				vmInfo.CPUUsage = GetStringField(cpu, "usage")
			}
			if memory := GetMapField(resources, "memory"); memory != nil {
				vmInfo.MemoryUsage = GetStringField(memory, "usage")
			}
		}

		// Parse timestamps
		if creationTime := GetStringField(obj, "metadata", "creationTimestamp"); creationTime != "" {
			if t, err := time.Parse(time.RFC3339, creationTime); err == nil {
				vmInfo.CreationTimestamp = t
			}
		}
		if startTime := GetStringField(obj, "status", "startTime"); startTime != "" {
			if t, err := time.Parse(time.RFC3339, startTime); err == nil {
				vmInfo.StartTime = t
			}
		}

		// Parse conditions
		if conditions := GetArrayField(obj, "status", "conditions"); len(conditions) > 0 {
			vmInfo.Conditions = make([]VMConditionInfo, 0, len(conditions))
			for _, cond := range conditions {
				if condMap, ok := cond.(map[string]interface{}); ok {
					condInfo := VMConditionInfo{
						Type:    GetStringField(condMap, "type"),
						Status:  GetStringField(condMap, "status"),
						Reason:  GetStringField(condMap, "reason"),
						Message: GetStringField(condMap, "message"),
					}
					if timeStr := GetStringField(condMap, "lastTransitionTime"); timeStr != "" {
						if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
							condInfo.Time = t
						}
					}
					vmInfo.Conditions = append(vmInfo.Conditions, condInfo)
				}
			}
		}

		// Add to appropriate list
		if phase == "Running" {
			kd.k8sMetrics.RunningVMs = append(kd.k8sMetrics.RunningVMs, vmInfo)
		} else {
			kd.k8sMetrics.StoppedVMs = append(kd.k8sMetrics.StoppedVMs, vmInfo)
		}
	}

	// Calculate resource statistics
	kd.updateVMResourceStats()

	return nil
}

// collectVMTemplateMetrics collects VMTemplate metrics
func (kd *K8sDashboard) collectVMTemplateMetrics(ctx context.Context) error {
	if kd.dynamicClient == nil {
		return nil
	}

	templates, err := kd.dynamicClient.GetVMTemplates(ctx, kd.namespace)
	if err != nil {
		return err
	}

	kd.k8sMetrics.VMTemplates = K8sResourceMetrics{
		Total:      len(templates),
		ByProvider: make(map[string]int),
	}
	kd.k8sMetrics.Templates = make([]K8sTemplateInfo, 0, len(templates))

	for _, obj := range templates {
		templateInfo := K8sTemplateInfo{
			Name:        GetStringField(obj, "metadata", "name"),
			Namespace:   GetStringField(obj, "metadata", "namespace"),
			DisplayName: GetStringField(obj, "spec", "displayName"),
			Description: GetStringField(obj, "spec", "description"),
			Version:     GetStringField(obj, "spec", "version"),
			Ready:       GetBoolField(obj, "status", "ready"),
			UsageCount:  safeInt32(GetInt64Field(obj, "status", "usageCount")),
		}

		// Parse OS info
		if osInfo := GetMapField(obj, "spec", "osInfo"); osInfo != nil {
			templateInfo.OSType = GetStringField(osInfo, "type")
			templateInfo.OSVersion = GetStringField(osInfo, "version")
		}

		// Parse default resources
		if defaultSpec := GetMapField(obj, "spec", "defaultSpec"); defaultSpec != nil {
			templateInfo.DefaultCPUs = safeInt32(GetInt64Field(defaultSpec, "cpus"))
			templateInfo.DefaultMemory = GetStringField(defaultSpec, "memory")
		}

		// Parse tags
		if tags := GetArrayField(obj, "spec", "tags"); len(tags) > 0 {
			templateInfo.Tags = make([]string, 0, len(tags))
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					templateInfo.Tags = append(templateInfo.Tags, tagStr)
				}
			}
		}

		kd.k8sMetrics.Templates = append(kd.k8sMetrics.Templates, templateInfo)
	}

	return nil
}

// collectVMSnapshotMetrics collects VMSnapshot metrics
func (kd *K8sDashboard) collectVMSnapshotMetrics(ctx context.Context) error {
	if kd.dynamicClient == nil {
		return nil
	}

	snapshots, err := kd.dynamicClient.GetVMSnapshots(ctx, kd.namespace)
	if err != nil {
		return err
	}

	kd.k8sMetrics.VMSnapshots = K8sResourceMetrics{
		Total:      len(snapshots),
		ByProvider: make(map[string]int),
	}
	kd.k8sMetrics.RecentSnapshots = make([]K8sSnapshotInfo, 0, len(snapshots))

	for _, obj := range snapshots {
		phase := GetStringField(obj, "status", "phase")

		// Count by phase
		switch phase {
		case "Pending":
			kd.k8sMetrics.VMSnapshots.Pending++
		case "Creating":
			kd.k8sMetrics.VMSnapshots.Running++
		case "Ready":
			kd.k8sMetrics.VMSnapshots.Completed++
		case "Failed":
			kd.k8sMetrics.VMSnapshots.Failed++
		}

		snapshotInfo := K8sSnapshotInfo{
			Name:           GetStringField(obj, "metadata", "name"),
			Namespace:      GetStringField(obj, "metadata", "namespace"),
			VMName:         GetStringField(obj, "spec", "vmRef", "name"),
			Phase:          phase,
			IncludeMemory:  GetBoolField(obj, "spec", "includeMemory"),
			Size:           GetStringField(obj, "status", "size"),
			SizeBytes:      GetInt64Field(obj, "status", "sizeBytes"),
			ReadyToRestore: GetBoolField(obj, "status", "readyToRestore"),
			Description:    GetStringField(obj, "spec", "description"),
		}

		if creationTime := GetStringField(obj, "status", "creationTime"); creationTime != "" {
			if t, err := time.Parse(time.RFC3339, creationTime); err == nil {
				snapshotInfo.CreationTime = t
			}
		}

		kd.k8sMetrics.RecentSnapshots = append(kd.k8sMetrics.RecentSnapshots, snapshotInfo)
	}

	// Keep only last 50 snapshots
	if len(kd.k8sMetrics.RecentSnapshots) > 50 {
		kd.k8sMetrics.RecentSnapshots = kd.k8sMetrics.RecentSnapshots[:50]
	}

	return nil
}

// updateVMResourceStats calculates VM resource statistics
func (kd *K8sDashboard) updateVMResourceStats() {
	stats := &kd.k8sMetrics.VMResourceStats
	stats.VMsBySize = make(map[string]int)
	stats.TotalCPUs = 0
	stats.TotalMemoryGi = 0.0
	stats.CarbonAwareVMs = 0
	var totalCarbonIntensity float64

	allVMs := append(kd.k8sMetrics.RunningVMs, kd.k8sMetrics.StoppedVMs...)
	if len(allVMs) == 0 {
		return
	}

	for _, vm := range allVMs {
		stats.TotalCPUs += vm.CPUs

		// Parse memory (e.g., "8Gi" -> 8.0)
		if vm.Memory != "" {
			// Simple parsing - just extract number
			var memGi float64
			if _, err := fmt.Sscanf(vm.Memory, "%fGi", &memGi); err == nil {
				stats.TotalMemoryGi += memGi
			}
		}

		// Categorize by size
		if vm.CPUs <= 2 {
			stats.VMsBySize["small"]++
		} else if vm.CPUs <= 8 {
			stats.VMsBySize["medium"]++
		} else {
			stats.VMsBySize["large"]++
		}

		// Carbon stats
		if vm.CarbonIntensity > 0 {
			stats.CarbonAwareVMs++
			totalCarbonIntensity += vm.CarbonIntensity
		}
	}

	stats.AvgCPUsPerVM = float64(stats.TotalCPUs) / float64(len(allVMs))
	stats.AvgMemoryPerVMGi = stats.TotalMemoryGi / float64(len(allVMs))

	if stats.CarbonAwareVMs > 0 {
		stats.AvgCarbonIntensity = totalCarbonIntensity / float64(stats.CarbonAwareVMs)
	}
}

// GetMetrics returns a snapshot of Kubernetes metrics
func (kd *K8sDashboard) GetMetrics() K8sMetrics {
	kd.k8sMetricsMu.RLock()
	defer kd.k8sMetricsMu.RUnlock()
	return *kd.k8sMetrics
}

// handleMetricsHistory serves historical metrics data
func (kd *K8sDashboard) handleMetricsHistory(w http.ResponseWriter, r *http.Request) {
	if kd.metricsHistory == nil || !kd.metricsHistory.IsEnabled() {
		http.Error(w, "Metrics history not enabled", http.StatusServiceUnavailable)
		return
	}

	// Parse time range parameters
	timeRange := r.URL.Query().Get("timeRange")
	var startTime, endTime time.Time
	var err error

	switch timeRange {
	case "1h":
		endTime = time.Now()
		startTime = endTime.Add(-1 * time.Hour)
	case "6h":
		endTime = time.Now()
		startTime = endTime.Add(-6 * time.Hour)
	case "24h", "1d", "":
		endTime = time.Now()
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		endTime = time.Now()
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		endTime = time.Now()
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		// Parse custom start/end times
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")
		if startStr != "" && endStr != "" {
			startTime, err = time.Parse(time.RFC3339, startStr)
			if err != nil {
				http.Error(w, "Invalid start time format", http.StatusBadRequest)
				return
			}
			endTime, err = time.Parse(time.RFC3339, endStr)
			if err != nil {
				http.Error(w, "Invalid end time format", http.StatusBadRequest)
				return
			}
		} else {
			http.Error(w, "Invalid time range", http.StatusBadRequest)
			return
		}
	}

	// Get historical data
	history, err := kd.metricsHistory.GetHistory(startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for export format
	format := r.URL.Query().Get("format")
	download := r.URL.Query().Get("download")

	switch format {
	case "csv":
		if download == "true" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=metrics-history-%s.csv", time.Now().Format("2006-01-02")))
		}
		w.Header().Set("Content-Type", "text/csv")
		if err := writeHistoryCSV(w, history); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		// Default to JSON
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(history); err != nil {
			fmt.Printf("Error encoding history response: %v\n", err)
		}
	}
}

// handleMetricsTrends serves aggregated trend data
func (kd *K8sDashboard) handleMetricsTrends(w http.ResponseWriter, r *http.Request) {
	if kd.metricsHistory == nil || !kd.metricsHistory.IsEnabled() {
		http.Error(w, "Metrics history not enabled", http.StatusServiceUnavailable)
		return
	}

	// Parse time range (default: 7 days)
	timeRange := r.URL.Query().Get("timeRange")
	endTime := time.Now()
	var startTime time.Time

	switch timeRange {
	case "24h", "1d":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d", "":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		startTime = endTime.Add(-7 * 24 * time.Hour)
	}

	// Get trend data
	trend, err := kd.metricsHistory.GetTrend(startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(trend); err != nil {
		fmt.Printf("Error encoding trend response: %v\n", err)
	}
}

// writeHistoryCSV writes historical metrics as CSV
func writeHistoryCSV(w http.ResponseWriter, history []HistoricalMetrics) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{
		"Timestamp", "Total VMs", "Running VMs", "Stopped VMs", "Failed VMs",
		"Total Backups", "Completed Backups", "Failed Backups", "Total Restores",
		"Total CPUs", "Total Memory (Gi)", "Avg Carbon Intensity", "Carbon-Aware VMs",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data rows
	for _, m := range history {
		row := []string{
			m.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%d", m.TotalVMs),
			fmt.Sprintf("%d", m.RunningVMs),
			fmt.Sprintf("%d", m.StoppedVMs),
			fmt.Sprintf("%d", m.FailedVMs),
			fmt.Sprintf("%d", m.TotalBackups),
			fmt.Sprintf("%d", m.CompletedBackups),
			fmt.Sprintf("%d", m.FailedBackups),
			fmt.Sprintf("%d", m.TotalRestores),
			fmt.Sprintf("%d", m.TotalCPUs),
			fmt.Sprintf("%.2f", m.TotalMemoryGi),
			fmt.Sprintf("%.2f", m.AvgCarbonIntensity),
			fmt.Sprintf("%d", m.CarbonAwareVMs),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// Multi-cluster handler functions

// handleClusters handles cluster listing and adding
func (kd *K8sDashboard) handleClusters(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List all clusters
		clusters := kd.multiCluster.ListClusters()
		
		type ClusterInfo struct {
			ID          string    `json:"id"`
			Name        string    `json:"name"`
			Context     string    `json:"context"`
			Server      string    `json:"server"`
			Namespace   string    `json:"namespace"`
			Connected   bool      `json:"connected"`
			LastUpdated time.Time `json:"last_updated"`
			Error       string    `json:"error,omitempty"`
			IsPrimary   bool      `json:"is_primary"`
		}
		
		infos := make([]ClusterInfo, 0, len(clusters))
		for _, cluster := range clusters {
			infos = append(infos, ClusterInfo{
				ID:          cluster.ID,
				Name:        cluster.Name,
				Context:     cluster.Context,
				Server:      cluster.Server,
				Namespace:   cluster.Namespace,
				Connected:   cluster.Connected,
				LastUpdated: cluster.LastUpdated,
				Error:       cluster.Error,
				IsPrimary:   cluster.ID == kd.multiCluster.primary,
			})
		}
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(infos); err != nil {
			fmt.Printf("Error encoding clusters response: %v\n", err)
		}

	case http.MethodPost:
		// Add new cluster
		type AddClusterRequest struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Context   string `json:"context"`
			Namespace string `json:"namespace"`
		}
		
		var req AddClusterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		if err := kd.multiCluster.AddCluster(req.ID, req.Name, req.Context, req.Namespace); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		// Enable multi-cluster mode
		kd.multiClusterMode = true
		
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "cluster added"}); err != nil {
			fmt.Printf("Error encoding cluster-added response: %v\n", err)
		}


	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClusterDetail handles individual cluster operations
func (kd *K8sDashboard) handleClusterDetail(w http.ResponseWriter, r *http.Request) {
	// Extract cluster ID from path
	path := r.URL.Path[len("/api/k8s/clusters/"):]
	clusterID := strings.TrimSuffix(path, "/")
	
	if clusterID == "" {
		http.Error(w, "Cluster ID required", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		// Get cluster metrics
		cluster, err := kd.multiCluster.GetCluster(clusterID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(cluster.Metrics); err != nil {
			fmt.Printf("Error encoding cluster metrics response: %v\n", err)
		}


	case http.MethodDelete:
		// Remove cluster
		if err := kd.multiCluster.RemoveCluster(clusterID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		// Disable multi-cluster mode if no clusters left
		if kd.multiCluster.GetClusterCount() == 0 {
			kd.multiClusterMode = false
		}
		
		w.WriteHeader(http.StatusNoContent)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClusterSwitch switches the primary cluster
func (kd *K8sDashboard) handleClusterSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	type SwitchRequest struct {
		ClusterID string `json:"cluster_id"`
	}
	
	var req SwitchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := kd.multiCluster.SetPrimaryCluster(req.ClusterID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "primary cluster switched"}); err != nil {
		fmt.Printf("Error encoding cluster-switch response: %v\n", err)
	}
}

// handleAggregatedMetrics returns aggregated metrics across all clusters
func (kd *K8sDashboard) handleAggregatedMetrics(w http.ResponseWriter, r *http.Request) {
	if !kd.multiClusterMode || !kd.multiCluster.IsEnabled() {
		// Fall back to single cluster metrics
		kd.handleK8sMetrics(w, r)
		return
	}
	
	aggregated := kd.multiCluster.GetAggregatedMetrics()
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(aggregated); err != nil {
		fmt.Printf("Error encoding aggregated metrics response: %v\n", err)
	}
}

// Console handler functions

// handleConsoleSessions returns active console sessions
func (kd *K8sDashboard) handleConsoleSessions(w http.ResponseWriter, r *http.Request) {
	if kd.vncProxy == nil {
		http.Error(w, "Console proxy not available", http.StatusServiceUnavailable)
		return
	}

	kd.vncProxy.HandleConsoleInfo(w, r)
}

// handleConsoleWebSocket handles console WebSocket connections
func (kd *K8sDashboard) handleConsoleWebSocket(w http.ResponseWriter, r *http.Request) {
	if kd.vncProxy == nil {
		http.Error(w, "Console proxy not available", http.StatusServiceUnavailable)
		return
	}

	kd.vncProxy.HandleVNCWebSocket(w, r)
}

// RegisterChiHandlers registers Kubernetes-specific HTTP handlers with chi router
func (kd *K8sDashboard) RegisterChiHandlers(r chi.Router) {
	r.Route("/k8s", func(r chi.Router) {
		// Main metrics
		r.Get("/metrics", kd.handleK8sMetrics)
		r.Get("/carbon", kd.handleCarbonStats)
		r.Get("/cluster", kd.handleClusterInfo)
		r.Get("/storage", kd.handleStorageStats)
		r.Get("/vm-metrics", kd.handleVMMetrics)

		// BackupJobs
		r.Route("/backupjobs", func(r chi.Router) {
			r.Get("/", kd.handleBackupJobs)
			r.Get("/{id}", kd.handleBackupJobDetailChi)
		})

		// Backup schedules
		r.Get("/backupschedules", kd.handleBackupSchedules)

		// Restore jobs
		r.Get("/restorejobs", kd.handleRestoreJobs)

		// VMs
		r.Route("/vms", func(r chi.Router) {
			r.Get("/", kd.handleVMs)
			r.Get("/{id}", kd.handleVMDetailChi)
		})

		// Templates and snapshots
		r.Get("/templates", kd.handleTemplates)
		r.Get("/snapshots", kd.handleSnapshots)

		// Historical metrics
		r.Get("/history", kd.handleMetricsHistory)
		r.Get("/trends", kd.handleMetricsTrends)

		// Multi-cluster management
		r.Route("/clusters", func(r chi.Router) {
			r.Get("/", kd.handleClusters)
			r.Post("/", kd.handleClusterDetailChi) // For adding clusters
			r.Get("/{id}", kd.handleClusterDetailChi)
			r.Delete("/{id}", kd.handleClusterDetailChi)
			r.Post("/switch", kd.handleClusterSwitch)
		})

		r.Get("/aggregated-metrics", kd.handleAggregatedMetrics)

		// Console endpoints
		r.Get("/console-sessions", kd.handleConsoleSessions)
	})

	// WebSocket endpoints (separate from /api prefix)
	r.Get("/ws/k8s", kd.wsHub.HandleWebSocket)
	r.Get("/ws/console", kd.handleConsoleWebSocket)
}

// Chi-compatible handlers that use chi.URLParam

func (kd *K8sDashboard) handleBackupJobDetailChi(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.URL.Path = "/api/k8s/backupjobs/" + id
	kd.handleBackupJobDetail(w, r)
}

func (kd *K8sDashboard) handleVMDetailChi(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.URL.Path = "/api/k8s/vms/" + id
	kd.handleVMDetail(w, r)
}

func (kd *K8sDashboard) handleClusterDetailChi(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.URL.Path = "/api/k8s/clusters/" + id
	kd.handleClusterDetail(w, r)
}
