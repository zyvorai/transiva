// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupJob is a specification for a backup job resource
type BackupJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupJobSpec   `json:"spec"`
	Status BackupJobStatus `json:"status,omitempty"`
}

// BackupJobSpec is the spec for a BackupJob resource
type BackupJobSpec struct {
	Source       BackupSource       `json:"source"`
	Destination  BackupDestination  `json:"destination"`
	Format       *BackupFormat      `json:"format,omitempty"`
	Incremental  *Incremental       `json:"incremental,omitempty"`
	CarbonAware  *CarbonAware       `json:"carbonAware,omitempty"`
	Retention    *RetentionPolicy   `json:"retention,omitempty"`
	Metadata     map[string]string  `json:"metadata,omitempty"`
}

// BackupSource defines the source VM for backup
type BackupSource struct {
	Provider  string            `json:"provider"`
	Namespace string            `json:"namespace,omitempty"`
	VMName    string            `json:"vmName,omitempty"`
	VMPath    string            `json:"vmPath,omitempty"`
	VMID      string            `json:"vmID,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// BackupDestination defines where to store the backup
type BackupDestination struct {
	Type         string            `json:"type"`
	Bucket       string            `json:"bucket,omitempty"`
	Prefix       string            `json:"prefix,omitempty"`
	Region       string            `json:"region,omitempty"`
	StorageClass string            `json:"storageClass,omitempty"`
	Endpoint     string            `json:"endpoint,omitempty"`
	Credentials  *CredentialsRef   `json:"credentials,omitempty"`
}

// CredentialsRef references a Kubernetes secret for credentials
type CredentialsRef struct {
	SecretRef SecretReference `json:"secretRef"`
}

// SecretReference references a secret by name and namespace
type SecretReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// BackupFormat defines the format options for backup
type BackupFormat struct {
	Type             string `json:"type,omitempty"`
	Compression      bool   `json:"compression,omitempty"`
	CompressionLevel int    `json:"compressionLevel,omitempty"`
}

// Incremental defines incremental backup options
type Incremental struct {
	Enabled       bool   `json:"enabled,omitempty"`
	BaseBackupRef string `json:"baseBackupRef,omitempty"`
}

// CarbonAware defines carbon-aware scheduling options
type CarbonAware struct {
	Enabled       bool    `json:"enabled,omitempty"`
	Zone          string  `json:"zone,omitempty"`
	MaxIntensity  float64 `json:"maxIntensity,omitempty"`
	MaxDelayHours float64 `json:"maxDelayHours,omitempty"`
}

// RetentionPolicy defines backup retention rules
type RetentionPolicy struct {
	KeepDaily   int `json:"keepDaily,omitempty"`
	KeepWeekly  int `json:"keepWeekly,omitempty"`
	KeepMonthly int `json:"keepMonthly,omitempty"`
	KeepYearly  int `json:"keepYearly,omitempty"`
}

// BackupJobStatus is the status for a BackupJob resource
type BackupJobStatus struct {
	Phase            BackupPhase           `json:"phase,omitempty"`
	StartTime        *metav1.Time          `json:"startTime,omitempty"`
	CompletionTime   *metav1.Time          `json:"completionTime,omitempty"`
	Progress         *BackupProgress       `json:"progress,omitempty"`
	Conditions       []BackupCondition     `json:"conditions,omitempty"`
	OutputPath       string                `json:"outputPath,omitempty"`
	Size             int64                 `json:"size,omitempty"`
	CarbonIntensity  float64               `json:"carbonIntensity,omitempty"`
	CarbonSavings    float64               `json:"carbonSavings,omitempty"`
	Error            string                `json:"error,omitempty"`
}

// BackupPhase represents the current phase of a backup job
type BackupPhase string

const (
	BackupPhasePending   BackupPhase = "Pending"
	BackupPhaseRunning   BackupPhase = "Running"
	BackupPhaseCompleted BackupPhase = "Completed"
	BackupPhaseFailed    BackupPhase = "Failed"
	BackupPhaseCancelled BackupPhase = "Cancelled"
)

// BackupProgress represents the progress of a backup job
type BackupProgress struct {
	Percentage          float64      `json:"percentage,omitempty"`
	BytesTransferred    int64        `json:"bytesTransferred,omitempty"`
	TotalBytes          int64        `json:"totalBytes,omitempty"`
	CurrentPhase        string       `json:"currentPhase,omitempty"`
	EstimatedCompletion *metav1.Time `json:"estimatedCompletion,omitempty"`
}

// BackupCondition represents a condition of a backup job
type BackupCondition struct {
	Type               string       `json:"type"`
	Status             string       `json:"status"`
	LastTransitionTime metav1.Time  `json:"lastTransitionTime"`
	Reason             string       `json:"reason,omitempty"`
	Message            string       `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupJobList is a list of BackupJob resources
type BackupJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []BackupJob `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupSchedule is a specification for a backup schedule resource
type BackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupScheduleSpec   `json:"spec"`
	Status BackupScheduleStatus `json:"status,omitempty"`
}

// BackupScheduleSpec is the spec for a BackupSchedule resource
type BackupScheduleSpec struct {
	Schedule                     string                  `json:"schedule"`
	Timezone                     string                  `json:"timezone,omitempty"`
	Suspend                      bool                    `json:"suspend,omitempty"`
	ConcurrencyPolicy            string                  `json:"concurrencyPolicy,omitempty"`
	StartingDeadlineSeconds      *int64                  `json:"startingDeadlineSeconds,omitempty"`
	SuccessfulJobsHistoryLimit   *int32                  `json:"successfulJobsHistoryLimit,omitempty"`
	FailedJobsHistoryLimit       *int32                  `json:"failedJobsHistoryLimit,omitempty"`
	JobTemplate                  BackupJobTemplateSpec   `json:"jobTemplate"`
}

// BackupJobTemplateSpec defines the template for backup jobs
type BackupJobTemplateSpec struct {
	ObjectMeta metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec       BackupJobSpec     `json:"spec"`
}

// BackupScheduleStatus is the status for a BackupSchedule resource
type BackupScheduleStatus struct {
	Active             []ActiveJob             `json:"active,omitempty"`
	LastScheduleTime   *metav1.Time            `json:"lastScheduleTime,omitempty"`
	LastSuccessfulTime *metav1.Time            `json:"lastSuccessfulTime,omitempty"`
	Conditions         []BackupCondition       `json:"conditions,omitempty"`
}

// ActiveJob references an active backup job
type ActiveJob struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupScheduleList is a list of BackupSchedule resources
type BackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []BackupSchedule `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RestoreJob is a specification for a restore job resource
type RestoreJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreJobSpec   `json:"spec"`
	Status RestoreJobStatus `json:"status,omitempty"`
}

// RestoreJobSpec is the spec for a RestoreJob resource
type RestoreJobSpec struct {
	Source      RestoreSource      `json:"source"`
	Destination RestoreDestination `json:"destination"`
	Options     *RestoreOptions    `json:"options,omitempty"`
	Metadata    map[string]string  `json:"metadata,omitempty"`
}

// RestoreSource defines the source backup for restore
type RestoreSource struct {
	Type          string            `json:"type"`
	Bucket        string            `json:"bucket,omitempty"`
	Path          string            `json:"path,omitempty"`
	Region        string            `json:"region,omitempty"`
	Endpoint      string            `json:"endpoint,omitempty"`
	BackupJobRef  *BackupJobRef     `json:"backupJobRef,omitempty"`
	Credentials   *CredentialsRef   `json:"credentials,omitempty"`
}

// BackupJobRef references a BackupJob resource
type BackupJobRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// RestoreDestination defines where to restore the VM
type RestoreDestination struct {
	Provider     string `json:"provider"`
	Namespace    string `json:"namespace,omitempty"`
	VMName       string `json:"vmName,omitempty"`
	Datacenter   string `json:"datacenter,omitempty"`
	ResourcePool string `json:"resourcePool,omitempty"`
	Datastore    string `json:"datastore,omitempty"`
	Network      string `json:"network,omitempty"`
	Folder       string `json:"folder,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
}

// RestoreOptions defines restore operation options
type RestoreOptions struct {
	PowerOnAfterRestore bool                   `json:"powerOnAfterRestore,omitempty"`
	Overwrite           bool                   `json:"overwrite,omitempty"`
	RenameVM            string                 `json:"renameVM,omitempty"`
	ConvertFormat       string                 `json:"convertFormat,omitempty"`
	Customization       *VMCustomization       `json:"customization,omitempty"`
}

// VMCustomization defines VM customization options
type VMCustomization struct {
	Memory   string            `json:"memory,omitempty"`
	CPU      int               `json:"cpu,omitempty"`
	Networks []NetworkConfig   `json:"networks,omitempty"`
}

// NetworkConfig defines network configuration
type NetworkConfig struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// RestoreJobStatus is the status for a RestoreJob resource
type RestoreJobStatus struct {
	Phase           RestorePhase      `json:"phase,omitempty"`
	StartTime       *metav1.Time      `json:"startTime,omitempty"`
	CompletionTime  *metav1.Time      `json:"completionTime,omitempty"`
	Progress        *RestoreProgress  `json:"progress,omitempty"`
	Conditions      []BackupCondition `json:"conditions,omitempty"`
	RestoredVMName  string            `json:"restoredVMName,omitempty"`
	RestoredVMID    string            `json:"restoredVMID,omitempty"`
	Error           string            `json:"error,omitempty"`
}

// RestorePhase represents the current phase of a restore job
type RestorePhase string

const (
	RestorePhasePending   RestorePhase = "Pending"
	RestorePhaseRunning   RestorePhase = "Running"
	RestorePhaseCompleted RestorePhase = "Completed"
	RestorePhaseFailed    RestorePhase = "Failed"
	RestorePhaseCancelled RestorePhase = "Cancelled"
)

// RestoreProgress represents the progress of a restore job
type RestoreProgress struct {
	Percentage          float64      `json:"percentage,omitempty"`
	BytesTransferred    int64        `json:"bytesTransferred,omitempty"`
	TotalBytes          int64        `json:"totalBytes,omitempty"`
	CurrentPhase        string       `json:"currentPhase,omitempty"`
	EstimatedCompletion *metav1.Time `json:"estimatedCompletion,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RestoreJobList is a list of RestoreJob resources
type RestoreJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RestoreJob `json:"items"`
}
