// SPDX-License-Identifier: Apache-2.0

package nutanix

import "github.com/zyvorai/transiva/config"

// DiskInfo describes a Nutanix VM disk for migration pickup.
type DiskInfo struct {
	UUID          string  `json:"uuid"`
	SizeGiB       float64 `json:"size_gib"`
	DeviceType    string  `json:"device_type"`
	DiskAddress   string  `json:"disk_address,omitempty"`
	ContainerUUID string  `json:"container_uuid,omitempty"`
}

// VMInventory is the JSON-friendly inventory record emitted by nutanix-pickup.
type VMInventory struct {
	Name         string     `json:"name"`
	UUID         string     `json:"uuid"`
	ClusterUUID  string     `json:"cluster_uuid"`
	ClusterName  string     `json:"cluster_name,omitempty"`
	Location     string     `json:"location,omitempty"`
	PowerState   string     `json:"power_state"`
	VCPUs        int        `json:"vcpus"`
	MemoryGiB    float64    `json:"memory_gib"`
	DiskCount    int        `json:"disk_count"`
	TotalDiskGiB float64    `json:"total_disk_gib"`
	NICCount     int        `json:"nic_count"`
	Disks        []DiskInfo `json:"disks,omitempty"`
}

// ClientConfig holds Nutanix Prism connection and discovery settings.
type ClientConfig struct {
	Host          string
	Port          int
	Username      string
	Password      string
	VerifySSL     bool
	PageSize      int
	DetailWorkers int
	Detailed      bool
	ClusterFilter string
}

// ClientConfigFromHypersdk converts transiva config to Nutanix client settings.
func ClientConfigFromHypersdk(cfg *config.NutanixConfig) *ClientConfig {
	if cfg == nil {
		return &ClientConfig{
			Port:          9440,
			PageSize:      500,
			DetailWorkers: 10,
			Detailed:      true,
			VerifySSL:     false,
		}
	}

	port := cfg.Port
	if port == 0 {
		port = 9440
	}
	pageSize := cfg.PageSize
	if pageSize == 0 {
		pageSize = 500
	}
	workers := cfg.DetailWorkers
	if workers == 0 {
		workers = 10
	}

	return &ClientConfig{
		Host:          cfg.Host,
		Port:          port,
		Username:      cfg.Username,
		Password:      cfg.Password,
		VerifySSL:     cfg.VerifySSL,
		PageSize:      pageSize,
		DetailWorkers: workers,
		Detailed:      cfg.Detailed,
		ClusterFilter: cfg.Cluster,
	}
}
