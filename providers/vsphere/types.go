// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"time"

	"github.com/zyvorai/transiva/providers/common"
)

const (
	defaultDirPerm   = 0755
	leaseWaitTimeout = 5 * time.Minute
	downloadTimeout  = 2 * time.Hour
)

type VMInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	PowerState string `json:"power_state"`
	GuestOS    string `json:"guest_os"`
	MemoryMB   int32  `json:"memory_mb"`
	NumCPU     int32  `json:"num_cpu"`
	Storage    int64  `json:"storage_bytes"`
}

type ExportResult struct {
	OutputDir    string
	OVFPath      string
	OVAPath      string // Path to OVA file (if Format is "ova")
	Format       string // "ovf" or "ova"
	Files        []string
	TotalSize    int64
	Duration     time.Duration
	ManifestPath string // Path to Artifact Manifest v1.0 JSON file

	// Conversion result (Phase 2)
	ConversionResult *common.ConversionResult

	// Metadata stores additional export metadata (pipeline results, etc.)
	Metadata map[string]interface{}
}

// Host/Cluster Infrastructure Types

type HostInfo struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	Datacenter      string `json:"datacenter"`
	Cluster         string `json:"cluster"`
	ConnectionState string `json:"connection_state"`
	PowerState      string `json:"power_state"`
	CPUModel        string `json:"cpu_model"`
	CPUCores        int32  `json:"cpu_cores"`
	CPUThreads      int32  `json:"cpu_threads"`
	CPUMhz          int32  `json:"cpu_mhz"`
	MemoryMB        int64  `json:"memory_mb"`
	NumNics         int    `json:"num_nics"`
	NumVMs          int    `json:"num_vms"`
	Version         string `json:"version"`
	Build           string `json:"build"`
}

type ClusterInfo struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	TotalCPU        int64  `json:"total_cpu_mhz"`
	TotalMemory     int64  `json:"total_memory_mb"`
	NumHosts        int    `json:"num_hosts"`
	NumCPUCores     int32  `json:"num_cpu_cores"`
	NumCPUThreads   int32  `json:"num_cpu_threads"`
	DRSEnabled      bool   `json:"drs_enabled"`
	DRSBehavior     string `json:"drs_behavior"`
	HAEnabled       bool   `json:"ha_enabled"`
	EffectiveCPU    int64  `json:"effective_cpu_mhz"`
	EffectiveMemory int64  `json:"effective_memory_mb"`
}

type DatacenterInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	NumClusters   int    `json:"num_clusters"`
	NumHosts      int    `json:"num_hosts"`
	NumVMs        int    `json:"num_vms"`
	NumDatastores int    `json:"num_datastores"`
}

type VCenterInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Build      string `json:"build"`
	OSType     string `json:"os_type"`
	APIType    string `json:"api_type"`
	APIVersion string `json:"api_version"`
	InstanceID string `json:"instance_id"`
}

// Performance Metrics Types

type PerformanceMetrics struct {
	EntityName    string                 `json:"entity_name"`
	EntityType    string                 `json:"entity_type"` // "vm", "host", "cluster"
	Timestamp     time.Time              `json:"timestamp"`
	CPUUsageMhz   int64                  `json:"cpu_usage_mhz"`
	CPUPercent    float64                `json:"cpu_percent"`
	MemoryUsageMB int64                  `json:"memory_usage_mb"`
	MemoryPercent float64                `json:"memory_percent"`
	DiskReadMBps  float64                `json:"disk_read_mbps"`
	DiskWriteMBps float64                `json:"disk_write_mbps"`
	NetRxMBps     float64                `json:"net_rx_mbps"`
	NetTxMBps     float64                `json:"net_tx_mbps"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type MetricsHistory struct {
	EntityName string               `json:"entity_name"`
	Interval   string               `json:"interval"` // "realtime", "5min", "30min", "2hour"
	StartTime  time.Time            `json:"start_time"`
	EndTime    time.Time            `json:"end_time"`
	Samples    []PerformanceMetrics `json:"samples"`
}

// Resource Pool Types

type ResourcePoolInfo struct {
	Name                string `json:"name"`
	Path                string `json:"path"`
	CPUAllocationMhz    int64  `json:"cpu_allocation_mhz"`
	CPUReservationMhz   int64  `json:"cpu_reservation_mhz"`
	CPULimitMhz         int64  `json:"cpu_limit_mhz"`
	CPUExpandable       bool   `json:"cpu_expandable"`
	MemoryAllocationMB  int64  `json:"memory_allocation_mb"`
	MemoryReservationMB int64  `json:"memory_reservation_mb"`
	MemoryLimitMB       int64  `json:"memory_limit_mb"`
	MemoryExpandable    bool   `json:"memory_expandable"`
	NumVMs              int    `json:"num_vms"`
	NumSubPools         int    `json:"num_sub_pools"`
}

type ResourcePoolConfig struct {
	Name                string `json:"name"`
	ParentPath          string `json:"parent_path"`
	CPUReservationMhz   int64  `json:"cpu_reservation_mhz"`
	CPULimitMhz         int64  `json:"cpu_limit_mhz"`
	CPUExpandable       bool   `json:"cpu_expandable"`
	CPUShares           string `json:"cpu_shares"` // "low", "normal", "high", "custom"
	CPUSharesLevel      int32  `json:"cpu_shares_level"`
	MemoryReservationMB int64  `json:"memory_reservation_mb"`
	MemoryLimitMB       int64  `json:"memory_limit_mb"`
	MemoryExpandable    bool   `json:"memory_expandable"`
	MemoryShares        string `json:"memory_shares"`
	MemorySharesLevel   int32  `json:"memory_shares_level"`
}

// Event & Monitoring Types

type VCenterEvent struct {
	EventID     int32                  `json:"event_id"`
	EventType   string                 `json:"event_type"`
	Message     string                 `json:"message"`
	CreatedTime time.Time              `json:"created_time"`
	UserName    string                 `json:"user_name"`
	EntityName  string                 `json:"entity_name"`
	EntityType  string                 `json:"entity_type"`
	Datacenter  string                 `json:"datacenter"`
	Severity    string                 `json:"severity"` // "info", "warning", "error"
	ChainID     int32                  `json:"chain_id"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type TaskInfo struct {
	TaskID       string    `json:"task_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	State        string    `json:"state"` // "queued", "running", "success", "error"
	EntityName   string    `json:"entity_name"`
	StartTime    time.Time `json:"start_time"`
	CompleteTime time.Time `json:"complete_time,omitempty"`
	Progress     int32     `json:"progress"`
	Error        string    `json:"error,omitempty"`
}

// VM Cloning Types

type CloneSpec struct {
	SourceVM       string            `json:"source_vm"`
	TargetName     string            `json:"target_name"`
	TargetFolder   string            `json:"target_folder,omitempty"`
	ResourcePool   string            `json:"resource_pool,omitempty"`
	Datastore      string            `json:"datastore,omitempty"`
	PowerOn        bool              `json:"power_on"`
	LinkedClone    bool              `json:"linked_clone"`
	Snapshot       string            `json:"snapshot,omitempty"`
	Template       bool              `json:"template"`
	CustomizeGuest bool              `json:"customize_guest"`
	Customization  map[string]string `json:"customization,omitempty"`
}

type CloneResult struct {
	SourceVM   string        `json:"source_vm"`
	TargetName string        `json:"target_name"`
	TargetPath string        `json:"target_path"`
	TaskID     string        `json:"task_id"`
	Success    bool          `json:"success"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
}
