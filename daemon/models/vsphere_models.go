// SPDX-License-Identifier: Apache-2.0

package models

import (
	"time"

	"github.com/zyvorai/transiva/providers/vsphere"
)

// Host/Cluster Request/Response Types

type HostInfoRequest struct {
	HostPattern string `json:"host_pattern,omitempty"` // Empty = all hosts
}

type HostInfoResponse struct {
	Hosts     []vsphere.HostInfo `json:"hosts"`
	Total     int                `json:"total"`
	Timestamp time.Time          `json:"timestamp"`
}

type ClusterInfoRequest struct {
	ClusterPattern string `json:"cluster_pattern,omitempty"` // Empty = all clusters
}

type ClusterInfoResponse struct {
	Clusters  []vsphere.ClusterInfo `json:"clusters"`
	Total     int                   `json:"total"`
	Timestamp time.Time             `json:"timestamp"`
}

type DatacenterInfoRequest struct {
	// Currently no filtering, returns all datacenters
}

type DatacenterInfoResponse struct {
	Datacenters []vsphere.DatacenterInfo `json:"datacenters"`
	Total       int                      `json:"total"`
	Timestamp   time.Time                `json:"timestamp"`
}

type VCenterInfoResponse struct {
	Info      *vsphere.VCenterInfo `json:"info"`
	Timestamp time.Time            `json:"timestamp"`
}

// Performance Metrics Request/Response Types

type MetricsRequest struct {
	EntityName string    `json:"entity_name"`
	EntityType string    `json:"entity_type"` // "vm", "host", "cluster"
	Realtime   bool      `json:"realtime"`
	StartTime  time.Time `json:"start_time,omitempty"`
	EndTime    time.Time `json:"end_time,omitempty"`
	Interval   string    `json:"interval,omitempty"` // "realtime", "5min", "30min", "2hour"
}

type MetricsResponse struct {
	Realtime bool                         `json:"realtime"`
	Current  *vsphere.PerformanceMetrics  `json:"current,omitempty"`
	History  *vsphere.MetricsHistory      `json:"history,omitempty"`
}

// Resource Pool Request/Response Types

type ResourcePoolRequest struct {
	PoolPattern string `json:"pool_pattern,omitempty"` // Empty = all pools
}

type ResourcePoolResponse struct {
	Pools     []vsphere.ResourcePoolInfo `json:"pools"`
	Total     int                        `json:"total"`
	Timestamp time.Time                  `json:"timestamp"`
}

type CreateResourcePoolRequest struct {
	Config vsphere.ResourcePoolConfig `json:"config"`
}

type UpdateResourcePoolRequest struct {
	PoolName string                     `json:"pool_name"`
	Config   vsphere.ResourcePoolConfig `json:"config"`
}

type ResourcePoolOperationResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Event & Monitoring Request/Response Types

type EventRequest struct {
	Since       time.Time `json:"since,omitempty"`
	EventTypes  []string  `json:"event_types,omitempty"`  // Filter by event type
	EntityTypes []string  `json:"entity_types,omitempty"` // Filter by entity type
	Limit       int       `json:"limit,omitempty"`        // Max events to return
}

type EventResponse struct {
	Events    []vsphere.VCenterEvent `json:"events"`
	Total     int                    `json:"total"`
	Timestamp time.Time              `json:"timestamp"`
}

type EventStreamRequest struct {
	EventTypes  []string  `json:"event_types,omitempty"`  // Filter by event type
	EntityTypes []string  `json:"entity_types,omitempty"` // Filter by entity
	Since       time.Time `json:"since,omitempty"`        // Start time
	Follow      bool      `json:"follow"`                 // Stream continuously
}

type TaskRequest struct {
	Since time.Time `json:"since,omitempty"`
	Limit int       `json:"limit,omitempty"` // Max tasks to return
}

type TaskResponse struct {
	Tasks     []vsphere.TaskInfo `json:"tasks"`
	Total     int                `json:"total"`
	Timestamp time.Time          `json:"timestamp"`
}

// VM Cloning Request/Response Types

type CloneVMRequest struct {
	Spec vsphere.CloneSpec `json:"spec"`
}

type CloneVMResponse struct {
	Result    vsphere.CloneResult `json:"result"`
	Timestamp time.Time           `json:"timestamp"`
}

type BulkCloneRequest struct {
	Specs          []vsphere.CloneSpec `json:"specs"`
	MaxConcurrent  int                 `json:"max_concurrent,omitempty"`  // Default: 5
	StopOnError    bool                `json:"stop_on_error,omitempty"`   // Stop if any clone fails
}

type BulkCloneResponse struct {
	Results   []vsphere.CloneResult `json:"results"`
	Success   int                   `json:"success"`
	Failed    int                   `json:"failed"`
	Duration  time.Duration         `json:"duration"`
	Timestamp time.Time             `json:"timestamp"`
}

type TemplateOperationRequest struct {
	VMName string `json:"vm_name"`
}

type TemplateOperationResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	VMName    string    `json:"vm_name"`
	Timestamp time.Time `json:"timestamp"`
}
