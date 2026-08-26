// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// JobsTotal tracks the total number of jobs by status and provider
	JobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hypersdk_jobs_total",
			Help: "Total number of export jobs",
		},
		[]string{"status", "provider"},
	)

	// JobDuration tracks the duration of completed jobs
	JobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hypersdk_job_duration_seconds",
			Help:    "Job duration in seconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1s to ~1 hour
		},
		[]string{"status", "provider"},
	)

	// ExportedVMs tracks the total number of VMs exported
	ExportedVMs = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hypersdk_vms_exported_total",
			Help: "Total number of VMs exported",
		},
		[]string{"provider", "os_type"},
	)

	// ExportedBytes tracks the total bytes exported
	ExportedBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hypersdk_bytes_exported_total",
			Help: "Total bytes exported",
		},
		[]string{"provider"},
	)

	// ExportSpeed tracks export speed in bytes per second
	ExportSpeed = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hypersdk_export_speed_bytes_per_second",
			Help:    "Export speed in bytes per second",
			Buckets: prometheus.ExponentialBuckets(1024*1024, 2, 10), // 1MB/s to 1GB/s
		},
		[]string{"provider"},
	)

	// APIRequests tracks HTTP API requests
	APIRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hypersdk_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	// APIRequestDuration tracks API request duration
	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hypersdk_api_request_duration_seconds",
			Help:    "API request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// VMsDiscovered tracks the number of VMs discovered
	VMsDiscovered = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hypersdk_vms_discovered",
			Help: "Number of VMs discovered in vCenter",
		},
		[]string{"provider", "power_state"},
	)

	// ActiveJobs tracks currently running jobs
	ActiveJobs = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hypersdk_active_jobs",
			Help: "Number of currently active jobs",
		},
	)

	// QueuedJobs tracks jobs waiting to execute
	QueuedJobs = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hypersdk_queued_jobs",
			Help: "Number of jobs in queue",
		},
	)

	// ConnectionPoolSize tracks connection pool metrics
	ConnectionPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hypersdk_connection_pool_size",
			Help: "Connection pool size",
		},
		[]string{"provider", "state"}, // state: active, idle
	)

	// ErrorsTotal tracks errors by type
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hypersdk_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type", "provider"},
	)

	// RetryAttempts tracks retry attempts
	RetryAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hypersdk_retry_attempts_total",
			Help: "Total number of retry attempts",
		},
		[]string{"operation", "provider"},
	)

	// DiskDownloads tracks individual disk downloads
	DiskDownloads = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hypersdk_disk_download_duration_seconds",
			Help:    "Disk download duration in seconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 12), // 10s to ~11 hours
		},
		[]string{"provider", "disk_size_gb"},
	)

	// OVFGenerationDuration tracks OVF generation time
	OVFGenerationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "hypersdk_ovf_generation_duration_seconds",
			Help:    "OVF generation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// BuildInfo provides build information
	BuildInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hypersdk_build_info",
			Help: "Build information",
		},
		[]string{"version", "go_version"},
	)
)

// RecordJobStart records the start of a job
func RecordJobStart(provider string) {
	ActiveJobs.Inc()
	JobsTotal.WithLabelValues("started", provider).Inc()
}

// RecordJobCompletion records the completion of a job
func RecordJobCompletion(provider, status string, durationSeconds float64) {
	ActiveJobs.Dec()
	JobsTotal.WithLabelValues(status, provider).Inc()
	JobDuration.WithLabelValues(status, provider).Observe(durationSeconds)
}

// RecordVMExport records a successful VM export
func RecordVMExport(provider, osType string, bytesExported int64) {
	ExportedVMs.WithLabelValues(provider, osType).Inc()
	ExportedBytes.WithLabelValues(provider).Add(float64(bytesExported))
}

// RecordExportSpeed records export speed
func RecordExportSpeed(provider string, bytesPerSecond float64) {
	ExportSpeed.WithLabelValues(provider).Observe(bytesPerSecond)
}

// RecordAPIRequest records an API request
func RecordAPIRequest(method, endpoint, statusCode string, durationSeconds float64) {
	APIRequests.WithLabelValues(method, endpoint, statusCode).Inc()
	APIRequestDuration.WithLabelValues(method, endpoint).Observe(durationSeconds)
}

// RecordError records an error
func RecordError(errorType, provider string) {
	ErrorsTotal.WithLabelValues(errorType, provider).Inc()
}

// RecordRetry records a retry attempt
func RecordRetry(operation, provider string) {
	RetryAttempts.WithLabelValues(operation, provider).Inc()
}

// UpdateVMsDiscovered updates the number of discovered VMs
func UpdateVMsDiscovered(provider, powerState string, count float64) {
	VMsDiscovered.WithLabelValues(provider, powerState).Set(count)
}

// UpdateQueuedJobs updates the number of queued jobs
func UpdateQueuedJobs(count float64) {
	QueuedJobs.Set(count)
}

// RecordDiskDownload records a disk download
func RecordDiskDownload(provider string, diskSizeGB int, durationSeconds float64) {
	DiskDownloads.WithLabelValues(provider, diskSizeToLabel(diskSizeGB)).Observe(durationSeconds)
}

// RecordOVFGeneration records OVF generation time
func RecordOVFGeneration(durationSeconds float64) {
	OVFGenerationDuration.Observe(durationSeconds)
}

// SetBuildInfo sets build information
func SetBuildInfo(version, goVersion string) {
	BuildInfo.WithLabelValues(version, goVersion).Set(1)
}

// diskSizeToLabel converts disk size to a label bucket
func diskSizeToLabel(sizeGB int) string {
	switch {
	case sizeGB < 10:
		return "small"
	case sizeGB < 50:
		return "medium"
	case sizeGB < 100:
		return "large"
	case sizeGB < 500:
		return "xlarge"
	default:
		return "xxlarge"
	}
}
