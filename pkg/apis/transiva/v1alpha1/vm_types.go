// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualMachine represents a virtual machine instance
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec"`
	Status VirtualMachineStatus `json:"status,omitempty"`
}

// VirtualMachineSpec defines the desired state of VirtualMachine
type VirtualMachineSpec struct {
	// VM Resources
	CPUs   int32  `json:"cpus"`
	Memory string `json:"memory"`
	GPUs   []VMGPU `json:"gpus,omitempty"`

	// USB Devices
	USBDevices []VMUSBDevice `json:"usbDevices,omitempty"`

	// Disks
	Disks []VMDisk `json:"disks,omitempty"`

	// Networks
	Networks []VMNetwork `json:"networks,omitempty"`

	// Image Source
	Image *VMImage `json:"image,omitempty"`

	// Cloud-init configuration
	CloudInit *CloudInitConfig `json:"cloudInit,omitempty"`

	// Desired power state
	Running bool `json:"running"`

	// Scheduling
	NodeSelector map[string]string      `json:"nodeSelector,omitempty"`
	Affinity     *corev1.Affinity       `json:"affinity,omitempty"`
	Tolerations  []corev1.Toleration    `json:"tolerations,omitempty"`

	// Carbon-aware scheduling
	CarbonAware *CarbonAwareSpec `json:"carbonAware,omitempty"`

	// Machine type
	MachineType string `json:"machineType,omitempty"`

	// Firmware
	Firmware *VMFirmware `json:"firmware,omitempty"`

	// Guest agent
	GuestAgent *VMGuestAgent `json:"guestAgent,omitempty"`

	// High Availability
	HighAvailability *VMHighAvailability `json:"highAvailability,omitempty"`

	// Labels and annotations to apply to the VM pod
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// VMDisk represents a virtual disk
type VMDisk struct {
	Name         string         `json:"name"`
	Size         string         `json:"size"`
	StorageClass string         `json:"storageClass,omitempty"`
	BootOrder    *int32         `json:"bootOrder,omitempty"`
	Type         string         `json:"type,omitempty"` // disk, cdrom
	Source       *VMDiskSource  `json:"source,omitempty"`
}

// VMDiskSource defines disk source
type VMDiskSource struct {
	PVC           string `json:"pvc,omitempty"`
	HTTP          string `json:"http,omitempty"`
	S3            string `json:"s3,omitempty"`
	ContainerDisk string `json:"containerDisk,omitempty"`
}

// VMNetwork represents a network interface
type VMNetwork struct {
	Name              string `json:"name"`
	Type              string `json:"type,omitempty"` // pod-network, multus, bridge
	MultusNetworkName string `json:"multusNetworkName,omitempty"`
	MACAddress        string `json:"macAddress,omitempty"`
}

// VMGPU represents a GPU device assignment
type VMGPU struct {
	Name         string `json:"name"`
	DeviceName   string `json:"deviceName,omitempty"`   // GPU device name (e.g., "nvidia.com/gpu")
	Vendor       string `json:"vendor,omitempty"`       // nvidia, amd, intel
	Model        string `json:"model,omitempty"`        // GPU model (e.g., "Tesla V100")
	Count        int32  `json:"count,omitempty"`        // Number of GPUs (default: 1)
	VirtualGPU   bool   `json:"virtualGPU,omitempty"`   // Use vGPU instead of full passthrough
	VGPUID       string `json:"vgpuID,omitempty"`       // vGPU profile ID
	Passthrough  bool   `json:"passthrough,omitempty"`  // Full GPU passthrough (default: true)
	ResourceName string `json:"resourceName,omitempty"` // Kubernetes resource name for GPU
}

// VMUSBDevice represents a USB device assignment
type VMUSBDevice struct {
	Name       string `json:"name"`
	VendorID   string `json:"vendorID,omitempty"`   // USB vendor ID (e.g., "0x1234")
	ProductID  string `json:"productID,omitempty"`  // USB product ID (e.g., "0x5678")
	Bus        string `json:"bus,omitempty"`        // USB bus number
	Device     string `json:"device,omitempty"`     // USB device number
	Serial     string `json:"serial,omitempty"`     // USB device serial number
	Hotplug    bool   `json:"hotplug,omitempty"`    // Enable hot-plug support
	DevicePath string `json:"devicePath,omitempty"` // Host device path (e.g., /dev/bus/usb/001/002)
}

// VMImage defines the VM image source
type VMImage struct {
	Source      string        `json:"source,omitempty"`
	TemplateRef *TemplateRef  `json:"templateRef,omitempty"`
	Checksum    *ImageChecksum `json:"checksum,omitempty"`
}

// TemplateRef references a VMTemplate
type TemplateRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ImageChecksum for image verification
type ImageChecksum struct {
	Type  string `json:"type"`  // md5, sha256, sha512
	Value string `json:"value"`
}

// CloudInitConfig for cloud-init
type CloudInitConfig struct {
	UserData    string                  `json:"userData,omitempty"`
	NetworkData string                  `json:"networkData,omitempty"`
	SecretRef   *corev1.SecretReference `json:"secretRef,omitempty"`
}

// CarbonAwareSpec for carbon-aware scheduling
type CarbonAwareSpec struct {
	Enabled          bool   `json:"enabled"`
	MaxIntensity     int32  `json:"maxIntensity,omitempty"`
	PreferGreenEnergy bool   `json:"preferGreenEnergy,omitempty"`
	Zone             string `json:"zone,omitempty"`
}

// VMFirmware configuration
type VMFirmware struct {
	Bootloader string `json:"bootloader,omitempty"` // bios, uefi
	SecureBoot bool   `json:"secureBoot,omitempty"`
}

// VMGuestAgent configuration
type VMGuestAgent struct {
	Enabled bool `json:"enabled"`
}

// VMHighAvailability configuration
type VMHighAvailability struct {
	Enabled          bool   `json:"enabled"`
	RestartPolicy    string `json:"restartPolicy,omitempty"` // Always, OnFailure, Never
	RestartDelay     string `json:"restartDelay,omitempty"`
	MaxRestarts      int32  `json:"maxRestarts,omitempty"`
	EvictionStrategy string `json:"evictionStrategy,omitempty"` // LiveMigrate, Shutdown, None
}

// VirtualMachineStatus defines the observed state of VirtualMachine
type VirtualMachineStatus struct {
	Phase       VMPhase              `json:"phase,omitempty"`
	Conditions  []VMCondition        `json:"conditions,omitempty"`
	GuestAgent  *GuestAgentStatus    `json:"guestAgent,omitempty"`
	IPAddresses []string             `json:"ipAddresses,omitempty"`
	NodeName    string               `json:"nodeName,omitempty"`
	Resources   *VMResourceStatus    `json:"resources,omitempty"`
	QEMUPid     int32                `json:"qemuPid,omitempty"`
	VNC         *VNCStatus           `json:"vnc,omitempty"`
	CreationTimestamp *metav1.Time   `json:"creationTimestamp,omitempty"`
	StartTime   *metav1.Time         `json:"startTime,omitempty"`
	CarbonIntensity float64          `json:"carbonIntensity,omitempty"`
}

// VMPhase represents VM lifecycle phase
type VMPhase string

const (
	VMPhasePending   VMPhase = "Pending"
	VMPhaseCreating  VMPhase = "Creating"
	VMPhaseRunning   VMPhase = "Running"
	VMPhaseStopped   VMPhase = "Stopped"
	VMPhaseMigrating VMPhase = "Migrating"
	VMPhasePaused    VMPhase = "Paused"
	VMPhaseFailed    VMPhase = "Failed"
	VMPhaseUnknown   VMPhase = "Unknown"
)

// VMCondition represents a VM condition
type VMCondition struct {
	Type               string      `json:"type"`
	Status             string      `json:"status"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	Reason             string      `json:"reason,omitempty"`
	Message            string      `json:"message,omitempty"`
}

// GuestAgentStatus represents guest agent status
type GuestAgentStatus struct {
	Connected bool   `json:"connected"`
	Version   string `json:"version,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
}

// VMResourceStatus represents current resource usage
type VMResourceStatus struct {
	CPU    *ResourceMetrics `json:"cpu,omitempty"`
	Memory *ResourceMetrics `json:"memory,omitempty"`
	GPUs   []GPUStatus      `json:"gpus,omitempty"`
}

// GPUStatus represents GPU device status
type GPUStatus struct {
	Name        string  `json:"name"`
	Model       string  `json:"model,omitempty"`
	UUID        string  `json:"uuid,omitempty"`
	PCIAddress  string  `json:"pciAddress,omitempty"`
	Utilization float64 `json:"utilization,omitempty"` // GPU utilization percentage
	Memory      *GPUMemoryStatus `json:"memory,omitempty"`
	Temperature int32   `json:"temperature,omitempty"` // GPU temperature in Celsius
	PowerUsage  int32   `json:"powerUsage,omitempty"`  // Power usage in watts
}

// GPUMemoryStatus represents GPU memory status
type GPUMemoryStatus struct {
	Total string  `json:"total,omitempty"`
	Used  string  `json:"used,omitempty"`
	Free  string  `json:"free,omitempty"`
}

// ResourceMetrics for CPU/Memory
type ResourceMetrics struct {
	Usage    string `json:"usage,omitempty"`
	Requests string `json:"requests,omitempty"`
}

// VNCStatus represents VNC console status
type VNCStatus struct {
	Port     int32 `json:"port,omitempty"`
	NodePort int32 `json:"nodePort,omitempty"`
	Enabled  bool  `json:"enabled"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualMachineList contains a list of VirtualMachine
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMOperation represents an asynchronous VM operation
type VMOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMOperationSpec   `json:"spec"`
	Status VMOperationStatus `json:"status,omitempty"`
}

// VMOperationSpec defines the desired operation
type VMOperationSpec struct {
	VMRef     VMReference       `json:"vmRef"`
	Operation VMOperationType   `json:"operation"`
	Force     bool              `json:"force,omitempty"`

	// Operation-specific specs
	CloneSpec   *CloneSpec   `json:"cloneSpec,omitempty"`
	MigrateSpec *MigrateSpec `json:"migrateSpec,omitempty"`
	ResizeSpec  *ResizeSpec  `json:"resizeSpec,omitempty"`
	SnapshotSpec *SnapshotSpec `json:"snapshotSpec,omitempty"`

	Timeout string `json:"timeout,omitempty"`
}

// VMReference references a VM
type VMReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// VMOperationType represents operation type
type VMOperationType string

const (
	VMOpStart    VMOperationType = "start"
	VMOpStop     VMOperationType = "stop"
	VMOpRestart  VMOperationType = "restart"
	VMOpClone    VMOperationType = "clone"
	VMOpMigrate  VMOperationType = "migrate"
	VMOpSnapshot VMOperationType = "snapshot"
	VMOpResize   VMOperationType = "resize"
	VMOpDelete   VMOperationType = "delete"
)

// CloneSpec for clone operations
type CloneSpec struct {
	TargetName       string `json:"targetName"`
	TargetNamespace  string `json:"targetNamespace,omitempty"`
	LinkedClone      bool   `json:"linkedClone,omitempty"`
	StartAfterClone  bool   `json:"startAfterClone,omitempty"`
	SnapshotRef      string `json:"snapshotRef,omitempty"`      // Clone from snapshot instead of VM
	PowerOnAfter     bool   `json:"powerOnAfter,omitempty"`     // Power on after cloning from snapshot
}

// MigrateSpec for migrate operations
type MigrateSpec struct {
	TargetNode   string `json:"targetNode"`
	Live         bool   `json:"live,omitempty"`
	Bandwidth    string `json:"bandwidth,omitempty"`
	AutoConverge bool   `json:"autoConverge,omitempty"`
	PostCopy     bool   `json:"postCopy,omitempty"`
}

// ResizeSpec for resize operations
type ResizeSpec struct {
	CPUs    int32  `json:"cpus,omitempty"`
	Memory  string `json:"memory,omitempty"`
	Hotplug bool   `json:"hotplug,omitempty"`
}

// SnapshotSpec for snapshot operations
type SnapshotSpec struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	IncludeMemory bool   `json:"includeMemory,omitempty"`
	Quiesce       bool   `json:"quiesce,omitempty"`
}

// VMOperationStatus represents operation status
type VMOperationStatus struct {
	Phase          VMOpPhase     `json:"phase,omitempty"`
	Conditions     []VMCondition `json:"conditions,omitempty"`
	Progress       int32         `json:"progress,omitempty"`
	StartTime      *metav1.Time  `json:"startTime,omitempty"`
	CompletionTime *metav1.Time  `json:"completionTime,omitempty"`
	Message        string        `json:"message,omitempty"`
	Result         string        `json:"result,omitempty"` // JSON-encoded result data
}

// VMOpPhase represents operation phase
type VMOpPhase string

const (
	VMOpPhasePending   VMOpPhase = "Pending"
	VMOpPhaseRunning   VMOpPhase = "Running"
	VMOpPhaseSucceeded VMOpPhase = "Succeeded"
	VMOpPhaseFailed    VMOpPhase = "Failed"
	VMOpPhaseCancelled VMOpPhase = "Cancelled"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMOperationList contains a list of VMOperation
type VMOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMOperation `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMTemplate represents a VM template
type VMTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMTemplateSpec   `json:"spec"`
	Status VMTemplateStatus `json:"status,omitempty"`
}

// VMTemplateSpec defines the template specification
type VMTemplateSpec struct {
	DisplayName  string                  `json:"displayName"`
	Description  string                  `json:"description,omitempty"`
	Version      string                  `json:"version,omitempty"`
	Tags         []string                `json:"tags,omitempty"`
	Icon         string                  `json:"icon,omitempty"`
	OSInfo       *OSInfo                 `json:"osInfo,omitempty"`
	DefaultSpec  *VirtualMachineSpec     `json:"defaultSpec,omitempty"`
	Image        VMTemplateImage         `json:"image"`
	CloudInit    *CloudInitDefaults      `json:"cloudInit,omitempty"`
	RequiredFeatures []string            `json:"requiredFeatures,omitempty"`
	RecommendedResources *ResourceRecommendations `json:"recommendedResources,omitempty"`
}

// OSInfo represents OS information
type OSInfo struct {
	Type         string `json:"type,omitempty"` // linux, windows, other
	Distribution string `json:"distribution,omitempty"`
	Version      string `json:"version,omitempty"`
}

// VMTemplateImage defines the template image
type VMTemplateImage struct {
	Source   string         `json:"source"`
	Format   string         `json:"format,omitempty"`
	Size     string         `json:"size,omitempty"`
	Checksum *ImageChecksum `json:"checksum,omitempty"`
}

// CloudInitDefaults for templates
type CloudInitDefaults struct {
	DefaultUserData    string `json:"defaultUserData,omitempty"`
	DefaultNetworkData string `json:"defaultNetworkData,omitempty"`
}

// ResourceRecommendations for templates
type ResourceRecommendations struct {
	MinCPUs      int32  `json:"minCpus,omitempty"`
	MinMemory    string `json:"minMemory,omitempty"`
	MinDiskSize  string `json:"minDiskSize,omitempty"`
}

// VMTemplateStatus represents template status
type VMTemplateStatus struct {
	Ready       bool         `json:"ready"`
	Size        string       `json:"size,omitempty"`
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
	UsageCount  int32        `json:"usageCount,omitempty"`
	Conditions  []VMCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMTemplateList contains a list of VMTemplate
type VMTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMTemplate `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMSnapshot represents a VM snapshot
type VMSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMSnapshotSpec   `json:"spec"`
	Status VMSnapshotStatus `json:"status,omitempty"`
}

// VMSnapshotSpec defines the snapshot specification
type VMSnapshotSpec struct {
	VMRef         VMReference           `json:"vmRef"`
	IncludeMemory bool                  `json:"includeMemory,omitempty"`
	Quiesce       bool                  `json:"quiesce,omitempty"`
	Description   string                `json:"description,omitempty"`
	Retention     *SnapshotRetention    `json:"retention,omitempty"`
	Destination   *SnapshotDestination  `json:"destination,omitempty"`
}

// SnapshotRetention defines retention policy
type SnapshotRetention struct {
	KeepDays   int32        `json:"keepDays,omitempty"`
	KeepCount  int32        `json:"keepCount,omitempty"`
	AutoDelete bool         `json:"autoDelete,omitempty"`
	ExpiresAt  *metav1.Time `json:"expiresAt,omitempty"`
}

// SnapshotDestination defines snapshot location
type SnapshotDestination struct {
	StorageClass string `json:"storageClass,omitempty"`
	Location     string `json:"location,omitempty"`
}

// VMSnapshotStatus represents snapshot status
type VMSnapshotStatus struct {
	Phase          VMSnapPhase   `json:"phase,omitempty"`
	Conditions     []VMCondition `json:"conditions,omitempty"`
	Size           string        `json:"size,omitempty"`
	SizeBytes      int64         `json:"sizeBytes,omitempty"`
	CreationTime   *metav1.Time  `json:"creationTime,omitempty"`
	ReadyToRestore bool          `json:"readyToRestore"`
	BackingFiles   []string      `json:"backingFiles,omitempty"`
	VMState        *VMStateSnapshot `json:"vmState,omitempty"`
	ParentSnapshot string        `json:"parentSnapshot,omitempty"`
	SnapshotChain  []string      `json:"snapshotChain,omitempty"`
}

// VMSnapPhase represents snapshot phase
type VMSnapPhase string

const (
	VMSnapPhasePending  VMSnapPhase = "Pending"
	VMSnapPhaseCreating VMSnapPhase = "Creating"
	VMSnapPhaseReady    VMSnapPhase = "Ready"
	VMSnapPhaseFailed   VMSnapPhase = "Failed"
	VMSnapPhaseDeleting VMSnapPhase = "Deleting"
	VMSnapPhaseExpired  VMSnapPhase = "Expired"
)

// VMStateSnapshot captures VM state at snapshot time
type VMStateSnapshot struct {
	CPUs      int32  `json:"cpus,omitempty"`
	Memory    string `json:"memory,omitempty"`
	DiskCount int32  `json:"diskCount,omitempty"`
	Running   bool   `json:"running,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMSnapshotList contains a list of VMSnapshot
type VMSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMSnapshot `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMBackupSchedule represents automated backup scheduling
type VMBackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMBackupScheduleSpec   `json:"spec"`
	Status VMBackupScheduleStatus `json:"status,omitempty"`
}

// VMBackupScheduleSpec defines the backup schedule specification
type VMBackupScheduleSpec struct {
	VMSelector       VMSelector             `json:"vmSelector"`
	Schedule         string                 `json:"schedule"` // Cron expression
	SnapshotTemplate VMSnapshotTemplateSpec `json:"snapshotTemplate"`
	RetentionPolicy  BackupRetentionPolicy  `json:"retentionPolicy"`
	Paused           bool                   `json:"paused,omitempty"`
	Timezone         string                 `json:"timezone,omitempty"` // Timezone for cron schedule
}

// VMSelector selects VMs for backup
type VMSelector struct {
	MatchLabels      map[string]string `json:"matchLabels,omitempty"`
	MatchNames       []string          `json:"matchNames,omitempty"`
	MatchNamespaces  []string          `json:"matchNamespaces,omitempty"`
	ExcludeNames     []string          `json:"excludeNames,omitempty"`
}

// VMSnapshotTemplateSpec defines snapshot creation template
type VMSnapshotTemplateSpec struct {
	IncludeMemory bool                  `json:"includeMemory,omitempty"`
	Quiesce       bool                  `json:"quiesce,omitempty"`
	Destination   *SnapshotDestination  `json:"destination,omitempty"`
	Labels        map[string]string     `json:"labels,omitempty"`
	Annotations   map[string]string     `json:"annotations,omitempty"`
}

// BackupRetentionPolicy defines backup retention rules
type BackupRetentionPolicy struct {
	KeepLast     int32  `json:"keepLast,omitempty"`     // Keep last N backups
	KeepDaily    int32  `json:"keepDaily,omitempty"`    // Keep daily backups for N days
	KeepWeekly   int32  `json:"keepWeekly,omitempty"`   // Keep weekly backups for N weeks
	KeepMonthly  int32  `json:"keepMonthly,omitempty"`  // Keep monthly backups for N months
	KeepYearly   int32  `json:"keepYearly,omitempty"`   // Keep yearly backups for N years
	MaxAge       string `json:"maxAge,omitempty"`       // Max age (e.g., "30d", "6m", "1y")
	AutoDelete   bool   `json:"autoDelete,omitempty"`   // Auto-delete expired backups
}

// VMBackupScheduleStatus represents backup schedule status
type VMBackupScheduleStatus struct {
	LastBackupTime      *metav1.Time  `json:"lastBackupTime,omitempty"`
	NextBackupTime      *metav1.Time  `json:"nextBackupTime,omitempty"`
	LastBackupSnapshot  string        `json:"lastBackupSnapshot,omitempty"`
	TotalBackups        int32         `json:"totalBackups,omitempty"`
	ActiveBackups       int32         `json:"activeBackups,omitempty"`
	FailedBackups       int32         `json:"failedBackups,omitempty"`
	LastBackupStatus    string        `json:"lastBackupStatus,omitempty"`
	LastBackupMessage   string        `json:"lastBackupMessage,omitempty"`
	MatchedVMs          []string      `json:"matchedVMs,omitempty"`
	Conditions          []VMCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMBackupScheduleList contains a list of VMBackupSchedule
type VMBackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMBackupSchedule `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMMigrationPolicy represents automated VM migration policy
type VMMigrationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VMMigrationPolicySpec   `json:"spec"`
	Status VMMigrationPolicyStatus `json:"status,omitempty"`
}

// VMMigrationPolicySpec defines the migration policy specification
type VMMigrationPolicySpec struct {
	Enabled         bool                    `json:"enabled"`
	Triggers        []MigrationTrigger      `json:"triggers"`
	Constraints     MigrationConstraints    `json:"constraints,omitempty"`
	TargetSelection TargetSelectionStrategy `json:"targetSelection"`
	DryRun          bool                    `json:"dryRun,omitempty"`
	Schedule        string                  `json:"schedule,omitempty"` // Cron for scheduled checks
}

// MigrationTrigger defines conditions that trigger migration
type MigrationTrigger struct {
	Type      MigrationTriggerType `json:"type"`
	Threshold string               `json:"threshold,omitempty"` // e.g., "80%", "high", "200W"
	Duration  string               `json:"duration,omitempty"`  // e.g., "5m", "1h"
	Priority  int32                `json:"priority,omitempty"`  // Higher = more important
}

// MigrationTriggerType represents trigger types
type MigrationTriggerType string

const (
	TriggerNodePressure      MigrationTriggerType = "NodePressure"       // High CPU/memory on node
	TriggerCarbonIntensity   MigrationTriggerType = "CarbonIntensity"    // High carbon intensity
	TriggerCost              MigrationTriggerType = "Cost"               // Cost optimization
	TriggerMaintenance       MigrationTriggerType = "Maintenance"        // Planned maintenance
	TriggerLoadBalancing     MigrationTriggerType = "LoadBalancing"      // Distribute load
	TriggerNodeDrain         MigrationTriggerType = "NodeDrain"          // Node being drained
	TriggerPowerUsage        MigrationTriggerType = "PowerUsage"         // High power consumption
	TriggerTemperature       MigrationTriggerType = "Temperature"        // High temperature
)

// MigrationConstraints defines migration constraints
type MigrationConstraints struct {
	MaxConcurrentMigrations int32             `json:"maxConcurrentMigrations,omitempty"`
	AllowedTimeWindows      []TimeWindow      `json:"allowedTimeWindows,omitempty"`
	BlockedTimeWindows      []TimeWindow      `json:"blockedTimeWindows,omitempty"`
	RequireVMLive           bool              `json:"requireVMLive,omitempty"` // Only live migration
	MaxMigrationsPerNode    int32             `json:"maxMigrationsPerNode,omitempty"`
	CooldownPeriod          string            `json:"cooldownPeriod,omitempty"` // e.g., "1h"
	NodeSelector            map[string]string `json:"nodeSelector,omitempty"`   // Target node labels
}

// TimeWindow defines a time window for migrations
type TimeWindow struct {
	Start    string   `json:"start"`              // HH:MM format
	End      string   `json:"end"`                // HH:MM format
	Days     []string `json:"days,omitempty"`     // Mon, Tue, etc.
	Timezone string   `json:"timezone,omitempty"` // IANA timezone
}

// TargetSelectionStrategy defines how to select migration targets
type TargetSelectionStrategy struct {
	Strategy  TargetStrategy         `json:"strategy"`
	Weights   map[string]int32       `json:"weights,omitempty"`   // Factor weights for scoring
	Filters   []TargetFilter         `json:"filters,omitempty"`   // Filter criteria
	AntiAffinity *AntiAffinityRule   `json:"antiAffinity,omitempty"`
}

// TargetStrategy represents target selection strategies
type TargetStrategy string

const (
	StrategyLeastUtilized   TargetStrategy = "LeastUtilized"   // Node with lowest utilization
	StrategyMostAvailable   TargetStrategy = "MostAvailable"   // Node with most free resources
	StrategyLowestCarbon    TargetStrategy = "LowestCarbon"    // Node in lowest carbon zone
	StrategyLowestCost      TargetStrategy = "LowestCost"      // Cheapest node
	StrategyRoundRobin      TargetStrategy = "RoundRobin"      // Distribute evenly
	StrategyBinPacking      TargetStrategy = "BinPacking"      // Pack VMs tightly
	StrategyWeightedScore   TargetStrategy = "WeightedScore"   // Score based on weights
)

// TargetFilter represents filtering criteria for target nodes
type TargetFilter struct {
	Type     string `json:"type"`     // cpu, memory, carbon, cost, temperature
	Operator string `json:"operator"` // <, >, <=, >=, ==, !=
	Value    string `json:"value"`
}

// AntiAffinityRule defines anti-affinity rules
type AntiAffinityRule struct {
	VMLabels map[string]string `json:"vmLabels,omitempty"` // Don't co-locate VMs with these labels
	Strength string            `json:"strength,omitempty"` // required, preferred
}

// VMMigrationPolicyStatus represents migration policy status
type VMMigrationPolicyStatus struct {
	Active                   bool              `json:"active"`
	TotalMigrations          int32             `json:"totalMigrations,omitempty"`
	SuccessfulMigrations     int32             `json:"successfulMigrations,omitempty"`
	FailedMigrations         int32             `json:"failedMigrations,omitempty"`
	PendingMigrations        int32             `json:"pendingMigrations,omitempty"`
	LastMigrationTime        *metav1.Time      `json:"lastMigrationTime,omitempty"`
	LastEvaluationTime       *metav1.Time      `json:"lastEvaluationTime,omitempty"`
	TriggeredMigrations      []MigrationRecord `json:"triggeredMigrations,omitempty"`
	Conditions               []VMCondition     `json:"conditions,omitempty"`
	RecommendedMigrations    int32             `json:"recommendedMigrations,omitempty"`
}

// MigrationRecord represents a migration event
type MigrationRecord struct {
	VMName       string           `json:"vmName"`
	FromNode     string           `json:"fromNode"`
	ToNode       string           `json:"toNode"`
	TriggerType  MigrationTriggerType `json:"triggerType"`
	StartTime    metav1.Time      `json:"startTime"`
	CompletionTime *metav1.Time   `json:"completionTime,omitempty"`
	Status       string           `json:"status"` // pending, running, succeeded, failed
	Reason       string           `json:"reason,omitempty"`
	Duration     string           `json:"duration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VMMigrationPolicyList contains a list of VMMigrationPolicy
type VMMigrationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VMMigrationPolicy `json:"items"`
}
