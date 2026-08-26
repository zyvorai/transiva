// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestJobsTotal(t *testing.T) {
	// Reset metrics before test
	JobsTotal.Reset()

	// Test incrementing counters
	JobsTotal.WithLabelValues("completed", "vsphere").Inc()
	JobsTotal.WithLabelValues("failed", "vsphere").Inc()
	JobsTotal.WithLabelValues("completed", "vsphere").Inc()

	// Verify values
	if got := testutil.ToFloat64(JobsTotal.WithLabelValues("completed", "vsphere")); got != 2 {
		t.Errorf("JobsTotal completed/vsphere = %v, want 2", got)
	}

	if got := testutil.ToFloat64(JobsTotal.WithLabelValues("failed", "vsphere")); got != 1 {
		t.Errorf("JobsTotal failed/vsphere = %v, want 1", got)
	}
}

func TestJobDuration(t *testing.T) {
	// Reset metrics before test
	JobDuration.Reset()

	// Record some durations
	JobDuration.WithLabelValues("completed", "vsphere").Observe(45.5)
	JobDuration.WithLabelValues("completed", "vsphere").Observe(120.3)
	JobDuration.WithLabelValues("failed", "vsphere").Observe(5.2)

	// Verify metric collection doesn't panic and contains expected labels
	// For histograms, we just verify the observations were recorded without error
	count := testutil.CollectAndCount(JobDuration)
	if count == 0 {
		t.Error("JobDuration did not collect any metrics")
	}
}

func TestExportedVMs(t *testing.T) {
	// Reset metrics before test
	ExportedVMs.Reset()

	// Test different OS types
	ExportedVMs.WithLabelValues("vsphere", "linux").Inc()
	ExportedVMs.WithLabelValues("vsphere", "linux").Inc()
	ExportedVMs.WithLabelValues("vsphere", "windows").Inc()
	ExportedVMs.WithLabelValues("aws", "linux").Inc()

	// Verify values
	if got := testutil.ToFloat64(ExportedVMs.WithLabelValues("vsphere", "linux")); got != 2 {
		t.Errorf("ExportedVMs vsphere/linux = %v, want 2", got)
	}

	if got := testutil.ToFloat64(ExportedVMs.WithLabelValues("vsphere", "windows")); got != 1 {
		t.Errorf("ExportedVMs vsphere/windows = %v, want 1", got)
	}
}

func TestExportedBytes(t *testing.T) {
	// Reset metrics before test
	ExportedBytes.Reset()

	// Test adding bytes
	ExportedBytes.WithLabelValues("vsphere").Add(1024 * 1024 * 100) // 100 MB
	ExportedBytes.WithLabelValues("vsphere").Add(1024 * 1024 * 50)  // 50 MB

	// Verify total
	expected := float64(1024 * 1024 * 150) // 150 MB
	if got := testutil.ToFloat64(ExportedBytes.WithLabelValues("vsphere")); got != expected {
		t.Errorf("ExportedBytes = %v, want %v", got, expected)
	}
}

func TestExportSpeed(t *testing.T) {
	// Reset metrics before test
	ExportSpeed.Reset()

	// Record some speeds (bytes per second)
	ExportSpeed.WithLabelValues("vsphere").Observe(10 * 1024 * 1024) // 10 MB/s
	ExportSpeed.WithLabelValues("vsphere").Observe(50 * 1024 * 1024) // 50 MB/s
	ExportSpeed.WithLabelValues("vsphere").Observe(25 * 1024 * 1024) // 25 MB/s

	// Verify metric collection doesn't panic
	count := testutil.CollectAndCount(ExportSpeed)
	if count == 0 {
		t.Error("ExportSpeed did not collect any metrics")
	}
}

func TestAPIRequests(t *testing.T) {
	// Reset metrics before test
	APIRequests.Reset()

	// Simulate API requests
	APIRequests.WithLabelValues("GET", "/api/jobs", "200").Inc()
	APIRequests.WithLabelValues("GET", "/api/jobs", "200").Inc()
	APIRequests.WithLabelValues("POST", "/api/jobs", "201").Inc()
	APIRequests.WithLabelValues("GET", "/api/jobs", "404").Inc()

	// Verify counts
	if got := testutil.ToFloat64(APIRequests.WithLabelValues("GET", "/api/jobs", "200")); got != 2 {
		t.Errorf("APIRequests GET/200 = %v, want 2", got)
	}

	if got := testutil.ToFloat64(APIRequests.WithLabelValues("POST", "/api/jobs", "201")); got != 1 {
		t.Errorf("APIRequests POST/201 = %v, want 1", got)
	}
}

func TestAPIRequestDuration(t *testing.T) {
	// Reset metrics before test
	APIRequestDuration.Reset()

	// Record request durations
	APIRequestDuration.WithLabelValues("GET", "/api/jobs").Observe(0.050)  // 50ms
	APIRequestDuration.WithLabelValues("GET", "/api/jobs").Observe(0.100)  // 100ms
	APIRequestDuration.WithLabelValues("POST", "/api/jobs").Observe(0.250) // 250ms

	// Verify metric collection doesn't panic
	count := testutil.CollectAndCount(APIRequestDuration)
	if count == 0 {
		t.Error("APIRequestDuration did not collect any metrics")
	}
}

func TestVMsDiscovered(t *testing.T) {
	// Test gauge operations
	VMsDiscovered.WithLabelValues("vsphere", "poweredOn").Set(42)
	VMsDiscovered.WithLabelValues("vsphere", "poweredOff").Set(18)

	// Verify values
	if got := testutil.ToFloat64(VMsDiscovered.WithLabelValues("vsphere", "poweredOn")); got != 42 {
		t.Errorf("VMsDiscovered poweredOn = %v, want 42", got)
	}

	if got := testutil.ToFloat64(VMsDiscovered.WithLabelValues("vsphere", "poweredOff")); got != 18 {
		t.Errorf("VMsDiscovered poweredOff = %v, want 18", got)
	}

	// Test incrementing/decrementing
	VMsDiscovered.WithLabelValues("vsphere", "poweredOn").Inc()
	if got := testutil.ToFloat64(VMsDiscovered.WithLabelValues("vsphere", "poweredOn")); got != 43 {
		t.Errorf("VMsDiscovered after Inc = %v, want 43", got)
	}
}

func TestActiveJobs(t *testing.T) {
	// Test gauge operations
	ActiveJobs.Set(5)

	if got := testutil.ToFloat64(ActiveJobs); got != 5 {
		t.Errorf("ActiveJobs = %v, want 5", got)
	}

	// Increment
	ActiveJobs.Inc()
	if got := testutil.ToFloat64(ActiveJobs); got != 6 {
		t.Errorf("ActiveJobs after Inc = %v, want 6", got)
	}

	// Decrement
	ActiveJobs.Dec()
	if got := testutil.ToFloat64(ActiveJobs); got != 5 {
		t.Errorf("ActiveJobs after Dec = %v, want 5", got)
	}
}

func TestQueuedJobs(t *testing.T) {
	// Test gauge operations
	QueuedJobs.Set(10)

	if got := testutil.ToFloat64(QueuedJobs); got != 10 {
		t.Errorf("QueuedJobs = %v, want 10", got)
	}

	// Add to queue
	QueuedJobs.Add(3)
	if got := testutil.ToFloat64(QueuedJobs); got != 13 {
		t.Errorf("QueuedJobs after Add = %v, want 13", got)
	}

	// Remove from queue
	QueuedJobs.Sub(2)
	if got := testutil.ToFloat64(QueuedJobs); got != 11 {
		t.Errorf("QueuedJobs after Sub = %v, want 11", got)
	}
}

func TestMetricsCollection(t *testing.T) {
	// Ensure all metrics are registered with Prometheus
	metrics := []prometheus.Collector{
		JobsTotal,
		JobDuration,
		ExportedVMs,
		ExportedBytes,
		ExportSpeed,
		APIRequests,
		APIRequestDuration,
		VMsDiscovered,
		ActiveJobs,
		QueuedJobs,
	}

	for i, metric := range metrics {
		if metric == nil {
			t.Errorf("Metric at index %d is nil", i)
		}
	}
}

func TestMetricsLabels(t *testing.T) {
	tests := []struct {
		name   string
		metric interface{}
		labels []string
	}{
		{"JobsTotal", JobsTotal, []string{"completed", "vsphere"}},
		{"JobDuration", JobDuration, []string{"failed", "aws"}},
		{"ExportedVMs", ExportedVMs, []string{"vsphere", "linux"}},
		{"ExportedBytes", ExportedBytes, []string{"aws"}},
		{"ExportSpeed", ExportSpeed, []string{"vsphere"}},
		{"APIRequests", APIRequests, []string{"GET", "/api/jobs", "200"}},
		{"APIRequestDuration", APIRequestDuration, []string{"POST", "/api/vms"}},
		{"VMsDiscovered", VMsDiscovered, []string{"vsphere", "poweredOn"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify we can create metrics with labels without panicking
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked with labels %v: %v", tt.name, tt.labels, r)
				}
			}()

			switch m := tt.metric.(type) {
			case *prometheus.CounterVec:
				m.WithLabelValues(tt.labels...).Add(1)
			case *prometheus.HistogramVec:
				m.WithLabelValues(tt.labels...).Observe(1)
			case *prometheus.GaugeVec:
				m.WithLabelValues(tt.labels...).Set(1)
			}
		})
	}
}

func TestRecordJobStart(t *testing.T) {
	initialValue := testutil.ToFloat64(ActiveJobs)
	RecordJobStart("vsphere")
	newValue := testutil.ToFloat64(ActiveJobs)
	if newValue <= initialValue {
		t.Errorf("RecordJobStart did not increment ActiveJobs: before=%v, after=%v", initialValue, newValue)
	}
}

func TestRecordJobCompletion(t *testing.T) {
	initialActive := testutil.ToFloat64(ActiveJobs)
	RecordJobCompletion("vsphere", "completed", 123.45)
	newActive := testutil.ToFloat64(ActiveJobs)
	if newActive >= initialActive {
		t.Errorf("RecordJobCompletion did not decrement ActiveJobs: before=%v, after=%v", initialActive, newActive)
	}
}

func TestRecordVMExport(t *testing.T) {
	ExportedVMs.Reset()
	ExportedBytes.Reset()

	RecordVMExport("vsphere", "linux", 1024*1024*100) // 100 MB

	if got := testutil.ToFloat64(ExportedVMs.WithLabelValues("vsphere", "linux")); got != 1 {
		t.Errorf("RecordVMExport VM count = %v, want 1", got)
	}

	expectedBytes := float64(1024 * 1024 * 100)
	if got := testutil.ToFloat64(ExportedBytes.WithLabelValues("vsphere")); got != expectedBytes {
		t.Errorf("RecordVMExport bytes = %v, want %v", got, expectedBytes)
	}
}

func TestRecordExportSpeed(t *testing.T) {
	RecordExportSpeed("vsphere", 10*1024*1024) // 10 MB/s
	// Just verify it doesn't panic
	count := testutil.CollectAndCount(ExportSpeed)
	if count == 0 {
		t.Error("RecordExportSpeed did not collect metrics")
	}
}

func TestRecordAPIRequest(t *testing.T) {
	APIRequests.Reset()
	RecordAPIRequest("GET", "/api/test", "200", 0.123)

	if got := testutil.ToFloat64(APIRequests.WithLabelValues("GET", "/api/test", "200")); got != 1 {
		t.Errorf("RecordAPIRequest count = %v, want 1", got)
	}
}

func TestRecordError(t *testing.T) {
	ErrorsTotal.Reset()
	RecordError("connection_timeout", "vsphere")

	if got := testutil.ToFloat64(ErrorsTotal.WithLabelValues("connection_timeout", "vsphere")); got != 1 {
		t.Errorf("RecordError count = %v, want 1", got)
	}
}

func TestRecordRetry(t *testing.T) {
	RetryAttempts.Reset()
	RecordRetry("disk_download", "vsphere")

	if got := testutil.ToFloat64(RetryAttempts.WithLabelValues("disk_download", "vsphere")); got != 1 {
		t.Errorf("RecordRetry count = %v, want 1", got)
	}
}

func TestUpdateVMsDiscovered(t *testing.T) {
	UpdateVMsDiscovered("vsphere", "poweredOn", 42)

	if got := testutil.ToFloat64(VMsDiscovered.WithLabelValues("vsphere", "poweredOn")); got != 42 {
		t.Errorf("UpdateVMsDiscovered = %v, want 42", got)
	}
}

func TestUpdateQueuedJobs(t *testing.T) {
	UpdateQueuedJobs(15)

	if got := testutil.ToFloat64(QueuedJobs); got != 15 {
		t.Errorf("UpdateQueuedJobs = %v, want 15", got)
	}
}

func TestRecordDiskDownload(t *testing.T) {
	DiskDownloads.Reset()
	RecordDiskDownload("vsphere", 75, 120.5)

	count := testutil.CollectAndCount(DiskDownloads)
	if count == 0 {
		t.Error("RecordDiskDownload did not collect metrics")
	}
}

func TestRecordOVFGeneration(t *testing.T) {
	RecordOVFGeneration(5.5)

	count := testutil.CollectAndCount(OVFGenerationDuration)
	if count == 0 {
		t.Error("RecordOVFGeneration did not collect metrics")
	}
}

func TestSetBuildInfo(t *testing.T) {
	BuildInfo.Reset()
	SetBuildInfo("1.0.0", "go1.21")

	if got := testutil.ToFloat64(BuildInfo.WithLabelValues("1.0.0", "go1.21")); got != 1 {
		t.Errorf("SetBuildInfo = %v, want 1", got)
	}
}

func TestDiskSizeToLabel(t *testing.T) {
	tests := []struct {
		sizeGB int
		want   string
	}{
		{5, "small"},
		{9, "small"},
		{10, "medium"},
		{49, "medium"},
		{50, "large"},
		{99, "large"},
		{100, "xlarge"},
		{499, "xlarge"},
		{500, "xxlarge"},
		{1000, "xxlarge"},
	}

	for _, tt := range tests {
		got := diskSizeToLabel(tt.sizeGB)
		if got != tt.want {
			t.Errorf("diskSizeToLabel(%d) = %v, want %v", tt.sizeGB, got, tt.want)
		}
	}
}
