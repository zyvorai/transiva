// SPDX-License-Identifier: Apache-2.0

package models

import (
	"time"

	"github.com/zyvorai/transiva/providers/vsphere"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// VCenterConfig represents vCenter connection details
type VCenterConfig struct {
	Server   string `json:"server" yaml:"server"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Insecure bool   `json:"insecure" yaml:"insecure"`
}

// JobDefinition represents a VM export job from YAML/JSON
type JobDefinition struct {
	ID           string         `json:"id" yaml:"id"`
	Name         string         `json:"name" yaml:"name"`
	VMPath       string         `json:"vm_path" yaml:"vm_path"`
	OutputPath   string         `json:"output_path,omitempty" yaml:"output_path,omitempty"` // For CLI/YAML
	OutputDir    string         `json:"output_dir,omitempty" yaml:"output_dir,omitempty"`   // For web API
	Options      *ExportOptions `json:"options,omitempty" yaml:"options,omitempty"`
	VCenterURL   string         `json:"vcenter_url,omitempty" yaml:"vcenter_url,omitempty"` // For CLI/YAML
	VCenter      *VCenterConfig `json:"vcenter,omitempty" yaml:"vcenter,omitempty"`         // For web API
	Username     string         `json:"username,omitempty" yaml:"username,omitempty"`
	Password     string         `json:"-" yaml:"password,omitempty"` // Excluded from JSON to prevent exposure
	Insecure     bool           `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	Datacenter   string         `json:"datacenter,omitempty" yaml:"datacenter,omitempty"`
	Format       string         `json:"format,omitempty" yaml:"format,omitempty"`               // Export format: qcow2, raw, vmdk, ova
	Provider     string         `json:"provider,omitempty" yaml:"provider,omitempty"`             // vsphere, nutanix, etc.
	ExportMethod string         `json:"export_method,omitempty" yaml:"export_method,omitempty"` // ctl, govc, ovftool, web, nutanix, or "" for auto
	Method       string         `json:"method,omitempty" yaml:"method,omitempty"`               // Alias for ExportMethod (web API compatibility)
	Compress       bool   `json:"compress,omitempty" yaml:"compress,omitempty"`
	Thin           bool   `json:"thin,omitempty" yaml:"thin,omitempty"`
	EnablePipeline bool   `json:"enable_pipeline,omitempty" yaml:"enable_pipeline,omitempty"`
	EnableRDP      *bool  `json:"enable_rdp,omitempty" yaml:"enable_rdp,omitempty"`
	CreatedAt      time.Time              `json:"created_at" yaml:"created_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"` // Additional metadata (e.g., carbon-aware settings)
}

// Redacted returns a copy of JobDefinition with sensitive fields redacted for logging
func (jd JobDefinition) Redacted() JobDefinition {
	redacted := jd
	if redacted.Password != "" {
		redacted.Password = "***REDACTED***"
	}
	return redacted
}

// ExportOptions represents export configuration
type ExportOptions struct {
	ParallelDownloads      int  `json:"parallel_downloads,omitempty" yaml:"parallel_downloads,omitempty"`
	RemoveCDROM            bool `json:"remove_cdrom,omitempty" yaml:"remove_cdrom,omitempty"`
	ShowIndividualProgress bool `json:"show_individual_progress,omitempty" yaml:"show_individual_progress,omitempty"`

	// Pipeline integration options
	EnablePipeline      bool   `json:"enable_pipeline,omitempty" yaml:"enable_pipeline,omitempty"`
	H2KVMPath           string `json:"h2kvm_path,omitempty" yaml:"h2kvm_path,omitempty"`
	// Deprecated: use H2KVMPath. Kept for configs written before the h2kvm rename.
	Hyper2KVMPath       string `json:"hyper2kvm_path,omitempty" yaml:"hyper2kvm_path,omitempty"`
	PipelineInspect     bool   `json:"pipeline_inspect,omitempty" yaml:"pipeline_inspect,omitempty"`
	PipelineFix         bool   `json:"pipeline_fix,omitempty" yaml:"pipeline_fix,omitempty"`
	PipelineConvert     bool   `json:"pipeline_convert,omitempty" yaml:"pipeline_convert,omitempty"`
	PipelineValidate    bool   `json:"pipeline_validate,omitempty" yaml:"pipeline_validate,omitempty"`
	PipelineCompress    bool   `json:"pipeline_compress,omitempty" yaml:"pipeline_compress,omitempty"`
	CompressLevel       int    `json:"compress_level,omitempty" yaml:"compress_level,omitempty"`
	EnableRDP           *bool  `json:"enable_rdp,omitempty" yaml:"enable_rdp,omitempty"`

	// Libvirt integration options
	LibvirtIntegration bool   `json:"libvirt_integration,omitempty" yaml:"libvirt_integration,omitempty"`
	LibvirtURI         string `json:"libvirt_uri,omitempty" yaml:"libvirt_uri,omitempty"`
	LibvirtAutoStart   bool   `json:"libvirt_autostart,omitempty" yaml:"libvirt_autostart,omitempty"`
	LibvirtBridge      string `json:"libvirt_bridge,omitempty" yaml:"libvirt_bridge,omitempty"`
	LibvirtPool        string `json:"libvirt_pool,omitempty" yaml:"libvirt_pool,omitempty"`

	// h2kvm daemon options
	H2KVMDaemon        bool   `json:"h2kvm_daemon,omitempty" yaml:"h2kvm_daemon,omitempty"`
	H2KVMInstance      string `json:"h2kvm_instance,omitempty" yaml:"h2kvm_instance,omitempty"`
	H2KVMWatchDir      string `json:"h2kvm_watch_dir,omitempty" yaml:"h2kvm_watch_dir,omitempty"`
	H2KVMOutputDir     string `json:"h2kvm_output_dir,omitempty" yaml:"h2kvm_output_dir,omitempty"`
	H2KVMPollInterval  int    `json:"h2kvm_poll_interval,omitempty" yaml:"h2kvm_poll_interval,omitempty"`
	H2KVMDaemonTimeout int    `json:"h2kvm_daemon_timeout,omitempty" yaml:"h2kvm_daemon_timeout,omitempty"`

	// Deprecated: use the H2KVM* fields above. Kept so configs/API requests
	// written before the h2kvm rename keep working for one release.
	Hyper2KVMDaemon        bool   `json:"hyper2kvm_daemon,omitempty" yaml:"hyper2kvm_daemon,omitempty"`
	Hyper2KVMInstance      string `json:"hyper2kvm_instance,omitempty" yaml:"hyper2kvm_instance,omitempty"`
	Hyper2KVMWatchDir      string `json:"hyper2kvm_watch_dir,omitempty" yaml:"hyper2kvm_watch_dir,omitempty"`
	Hyper2KVMOutputDir     string `json:"hyper2kvm_output_dir,omitempty" yaml:"hyper2kvm_output_dir,omitempty"`
	Hyper2KVMPollInterval  int    `json:"hyper2kvm_poll_interval,omitempty" yaml:"hyper2kvm_poll_interval,omitempty"`
	Hyper2KVMDaemonTimeout int    `json:"hyper2kvm_daemon_timeout,omitempty" yaml:"hyper2kvm_daemon_timeout,omitempty"`
}

// ApplyH2KVMCompat copies any legacy hyper2kvm_* values into their h2kvm_*
// replacements when the new field was left unset. Call this after decoding
// an ExportOptions from JSON/YAML written before the h2kvm rename.
func (o *ExportOptions) ApplyH2KVMCompat() {
	if o.H2KVMPath == "" {
		o.H2KVMPath = o.Hyper2KVMPath
	}
	if !o.H2KVMDaemon {
		o.H2KVMDaemon = o.Hyper2KVMDaemon
	}
	if o.H2KVMInstance == "" {
		o.H2KVMInstance = o.Hyper2KVMInstance
	}
	if o.H2KVMWatchDir == "" {
		o.H2KVMWatchDir = o.Hyper2KVMWatchDir
	}
	if o.H2KVMOutputDir == "" {
		o.H2KVMOutputDir = o.Hyper2KVMOutputDir
	}
	if o.H2KVMPollInterval == 0 {
		o.H2KVMPollInterval = o.Hyper2KVMPollInterval
	}
	if o.H2KVMDaemonTimeout == 0 {
		o.H2KVMDaemonTimeout = o.Hyper2KVMDaemonTimeout
	}
}

// Job represents an active or completed export job
type Job struct {
	Definition  JobDefinition `json:"definition"`
	Status      JobStatus     `json:"status"`
	Progress    *JobProgress  `json:"progress,omitempty"`
	Result      *JobResult    `json:"result,omitempty"`
	Error       string        `json:"error,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// JobProgress tracks the progress of an export
type JobProgress struct {
	Phase              string  `json:"phase"` // "connecting", "discovering", "exporting"
	CurrentFile        string  `json:"current_file,omitempty"`
	CurrentStep        string  `json:"current_step,omitempty"` // Current step description
	FilesDownloaded    int     `json:"files_downloaded"`
	TotalFiles         int     `json:"total_files"`
	BytesDownloaded    int64   `json:"bytes_downloaded"`
	BytesTransferred   int64   `json:"bytes_transferred"` // Alias for BytesDownloaded
	TotalBytes         int64   `json:"total_bytes"`
	PercentComplete    float64 `json:"percent_complete"`
	EstimatedRemaining string  `json:"estimated_remaining,omitempty"`
	ExportMethod       string  `json:"export_method,omitempty"` // Which export method is being used
}

// JobResult represents the result of a completed job
type JobResult struct {
	VMName       string        `json:"vm_name"`
	OutputDir    string        `json:"output_dir"`
	OVFPath      string        `json:"ovf_path"`
	Files        []string      `json:"files"`
	OutputFiles  []string      `json:"output_files,omitempty"` // Alias for Files
	TotalSize    int64         `json:"total_size"`
	Duration     time.Duration `json:"duration"`
	Success      bool          `json:"success"`
	ExportMethod string        `json:"export_method,omitempty"` // Which method was used
	Error        string        `json:"error,omitempty"`         // Error message if failed
}

// ToVSphereOptions converts ExportOptions to vsphere.ExportOptions
func (eo *ExportOptions) ToVSphereOptions(outputPath string) vsphere.ExportOptions {
	opts := vsphere.DefaultExportOptions()
	opts.OutputPath = outputPath

	if eo != nil {
		if eo.ParallelDownloads > 0 {
			opts.ParallelDownloads = eo.ParallelDownloads
		}
		opts.RemoveCDROM = eo.RemoveCDROM
		opts.ShowIndividualProgress = eo.ShowIndividualProgress
		opts.EnablePipeline = eo.EnablePipeline
		opts.PipelineInspect = eo.PipelineInspect
		opts.PipelineFix = eo.PipelineFix
		opts.PipelineConvert = eo.PipelineConvert
		opts.PipelineValidate = eo.PipelineValidate
		opts.PipelineCompress = eo.PipelineCompress
		opts.EnableRDP = eo.EnableRDP
	}

	return opts
}

// BatchJobDefinition represents multiple jobs in a single file
type BatchJobDefinition struct {
	Jobs []JobDefinition `json:"jobs" yaml:"jobs"`
}

// QueryRequest represents a query from hyperctl
type QueryRequest struct {
	JobIDs []string    `json:"job_ids,omitempty"` // Specific job IDs to query
	Status []JobStatus `json:"status,omitempty"`  // Filter by status
	All    bool        `json:"all"`               // Return all jobs
	Limit  int         `json:"limit,omitempty"`   // Limit results
}

// QueryResponse represents the response to a query
type QueryResponse struct {
	Jobs      []*Job    `json:"jobs"`
	Total     int       `json:"total"`
	Timestamp time.Time `json:"timestamp"`
}

// SubmitResponse represents the response to job submission
type SubmitResponse struct {
	JobIDs    []string  `json:"job_ids"`
	Accepted  int       `json:"accepted"`
	Rejected  int       `json:"rejected"`
	Errors    []string  `json:"errors,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// CancelRequest represents a request to cancel jobs
type CancelRequest struct {
	JobIDs []string `json:"job_ids"`
}

// CancelResponse represents the response to a cancel request
type CancelResponse struct {
	Cancelled []string          `json:"cancelled"`
	Failed    []string          `json:"failed"`
	Errors    map[string]string `json:"errors,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// DaemonStatus represents the overall status of the daemon
type DaemonStatus struct {
	Version        string    `json:"version"`
	Uptime         string    `json:"uptime"`
	TotalJobs      int       `json:"total_jobs"`
	RunningJobs    int       `json:"running_jobs"`
	CompletedJobs  int       `json:"completed_jobs"`
	FailedJobs     int       `json:"failed_jobs"`
	CancelledJobs  int       `json:"cancelled_jobs"`
	Timestamp      time.Time `json:"timestamp"`
}
