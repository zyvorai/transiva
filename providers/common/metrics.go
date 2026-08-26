// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// MetricsCollector collects and exports Prometheus metrics
type MetricsCollector struct {
	mu sync.RWMutex

	// Migration counters
	migrationsTotal     int64
	migrationsSucceeded int64
	migrationsFailed    int64

	// Migration durations (in seconds)
	exportDurationTotal     float64
	conversionDurationTotal float64
	uploadDurationTotal     float64
	migrationDurationTotal  float64

	// Bytes transferred
	bytesExported  int64
	bytesConverted int64
	bytesUploaded  int64

	// Current active migrations
	activeMigrations int64

	// Provider breakdown
	providerCounts map[string]int64

	// Start time
	startTime time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		providerCounts: make(map[string]int64),
		startTime:      time.Now(),
	}
}

// RecordMigrationStart records the start of a migration
func (mc *MetricsCollector) RecordMigrationStart(provider string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.migrationsTotal++
	mc.activeMigrations++
	mc.providerCounts[provider]++
}

// RecordMigrationSuccess records a successful migration
func (mc *MetricsCollector) RecordMigrationSuccess(provider string, exportDuration, conversionDuration, uploadDuration time.Duration, bytesExported, bytesConverted, bytesUploaded int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.migrationsSucceeded++
	mc.activeMigrations--

	mc.exportDurationTotal += exportDuration.Seconds()
	mc.conversionDurationTotal += conversionDuration.Seconds()
	mc.uploadDurationTotal += uploadDuration.Seconds()
	mc.migrationDurationTotal += (exportDuration + conversionDuration + uploadDuration).Seconds()

	mc.bytesExported += bytesExported
	mc.bytesConverted += bytesConverted
	mc.bytesUploaded += bytesUploaded
}

// RecordMigrationFailure records a failed migration
func (mc *MetricsCollector) RecordMigrationFailure(provider string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.migrationsFailed++
	mc.activeMigrations--
}

// GetMetrics returns current metrics as Prometheus format
func (mc *MetricsCollector) GetMetrics() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var metrics string

	// Help and type annotations
	metrics += "# HELP hypersdk_migrations_total Total number of migrations attempted\n"
	metrics += "# TYPE hypersdk_migrations_total counter\n"
	metrics += fmt.Sprintf("hypersdk_migrations_total %d\n", mc.migrationsTotal)

	metrics += "# HELP hypersdk_migrations_succeeded Total number of successful migrations\n"
	metrics += "# TYPE hypersdk_migrations_succeeded counter\n"
	metrics += fmt.Sprintf("hypersdk_migrations_succeeded %d\n", mc.migrationsSucceeded)

	metrics += "# HELP hypersdk_migrations_failed Total number of failed migrations\n"
	metrics += "# TYPE hypersdk_migrations_failed counter\n"
	metrics += fmt.Sprintf("hypersdk_migrations_failed %d\n", mc.migrationsFailed)

	metrics += "# HELP hypersdk_migrations_active Current number of active migrations\n"
	metrics += "# TYPE hypersdk_migrations_active gauge\n"
	metrics += fmt.Sprintf("hypersdk_migrations_active %d\n", mc.activeMigrations)

	metrics += "# HELP hypersdk_export_duration_seconds Total export duration in seconds\n"
	metrics += "# TYPE hypersdk_export_duration_seconds counter\n"
	metrics += fmt.Sprintf("hypersdk_export_duration_seconds %.2f\n", mc.exportDurationTotal)

	metrics += "# HELP hypersdk_conversion_duration_seconds Total conversion duration in seconds\n"
	metrics += "# TYPE hypersdk_conversion_duration_seconds counter\n"
	metrics += fmt.Sprintf("hypersdk_conversion_duration_seconds %.2f\n", mc.conversionDurationTotal)

	metrics += "# HELP hypersdk_upload_duration_seconds Total upload duration in seconds\n"
	metrics += "# TYPE hypersdk_upload_duration_seconds counter\n"
	metrics += fmt.Sprintf("hypersdk_upload_duration_seconds %.2f\n", mc.uploadDurationTotal)

	metrics += "# HELP hypersdk_migration_duration_seconds Total migration duration in seconds\n"
	metrics += "# TYPE hypersdk_migration_duration_seconds counter\n"
	metrics += fmt.Sprintf("hypersdk_migration_duration_seconds %.2f\n", mc.migrationDurationTotal)

	metrics += "# HELP hypersdk_bytes_exported_total Total bytes exported\n"
	metrics += "# TYPE hypersdk_bytes_exported_total counter\n"
	metrics += fmt.Sprintf("hypersdk_bytes_exported_total %d\n", mc.bytesExported)

	metrics += "# HELP hypersdk_bytes_converted_total Total bytes converted\n"
	metrics += "# TYPE hypersdk_bytes_converted_total counter\n"
	metrics += fmt.Sprintf("hypersdk_bytes_converted_total %d\n", mc.bytesConverted)

	metrics += "# HELP hypersdk_bytes_uploaded_total Total bytes uploaded\n"
	metrics += "# TYPE hypersdk_bytes_uploaded_total counter\n"
	metrics += fmt.Sprintf("hypersdk_bytes_uploaded_total %d\n", mc.bytesUploaded)

	// Provider breakdown
	metrics += "# HELP hypersdk_migrations_by_provider Migrations by provider\n"
	metrics += "# TYPE hypersdk_migrations_by_provider counter\n"
	for provider, count := range mc.providerCounts {
		metrics += fmt.Sprintf("hypersdk_migrations_by_provider{provider=\"%s\"} %d\n", provider, count)
	}

	// Uptime
	uptime := time.Since(mc.startTime).Seconds()
	metrics += "# HELP hypersdk_uptime_seconds Uptime in seconds\n"
	metrics += "# TYPE hypersdk_uptime_seconds counter\n"
	metrics += fmt.Sprintf("hypersdk_uptime_seconds %.2f\n", uptime)

	// Success rate
	var successRate float64
	if mc.migrationsTotal > 0 {
		successRate = float64(mc.migrationsSucceeded) / float64(mc.migrationsTotal) * 100
	}
	metrics += "# HELP hypersdk_success_rate_percent Migration success rate percentage\n"
	metrics += "# TYPE hypersdk_success_rate_percent gauge\n"
	metrics += fmt.Sprintf("hypersdk_success_rate_percent %.2f\n", successRate)

	// Average durations
	var avgExportDuration, avgConversionDuration, avgUploadDuration, avgMigrationDuration float64
	if mc.migrationsSucceeded > 0 {
		avgExportDuration = mc.exportDurationTotal / float64(mc.migrationsSucceeded)
		avgConversionDuration = mc.conversionDurationTotal / float64(mc.migrationsSucceeded)
		avgUploadDuration = mc.uploadDurationTotal / float64(mc.migrationsSucceeded)
		avgMigrationDuration = mc.migrationDurationTotal / float64(mc.migrationsSucceeded)
	}

	metrics += "# HELP hypersdk_avg_export_duration_seconds Average export duration in seconds\n"
	metrics += "# TYPE hypersdk_avg_export_duration_seconds gauge\n"
	metrics += fmt.Sprintf("hypersdk_avg_export_duration_seconds %.2f\n", avgExportDuration)

	metrics += "# HELP hypersdk_avg_conversion_duration_seconds Average conversion duration in seconds\n"
	metrics += "# TYPE hypersdk_avg_conversion_duration_seconds gauge\n"
	metrics += fmt.Sprintf("hypersdk_avg_conversion_duration_seconds %.2f\n", avgConversionDuration)

	metrics += "# HELP hypersdk_avg_upload_duration_seconds Average upload duration in seconds\n"
	metrics += "# TYPE hypersdk_avg_upload_duration_seconds gauge\n"
	metrics += fmt.Sprintf("hypersdk_avg_upload_duration_seconds %.2f\n", avgUploadDuration)

	metrics += "# HELP hypersdk_avg_migration_duration_seconds Average total migration duration in seconds\n"
	metrics += "# TYPE hypersdk_avg_migration_duration_seconds gauge\n"
	metrics += fmt.Sprintf("hypersdk_avg_migration_duration_seconds %.2f\n", avgMigrationDuration)

	return metrics
}

// GetStats returns metrics as a structured object
func (mc *MetricsCollector) GetStats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var successRate float64
	if mc.migrationsTotal > 0 {
		successRate = float64(mc.migrationsSucceeded) / float64(mc.migrationsTotal) * 100
	}

	var avgExportDuration, avgConversionDuration, avgUploadDuration, avgMigrationDuration float64
	if mc.migrationsSucceeded > 0 {
		avgExportDuration = mc.exportDurationTotal / float64(mc.migrationsSucceeded)
		avgConversionDuration = mc.conversionDurationTotal / float64(mc.migrationsSucceeded)
		avgUploadDuration = mc.uploadDurationTotal / float64(mc.migrationsSucceeded)
		avgMigrationDuration = mc.migrationDurationTotal / float64(mc.migrationsSucceeded)
	}

	return map[string]interface{}{
		"migrations": map[string]interface{}{
			"total":     mc.migrationsTotal,
			"succeeded": mc.migrationsSucceeded,
			"failed":    mc.migrationsFailed,
			"active":    mc.activeMigrations,
		},
		"success_rate": successRate,
		"durations": map[string]interface{}{
			"export_total":     mc.exportDurationTotal,
			"conversion_total": mc.conversionDurationTotal,
			"upload_total":     mc.uploadDurationTotal,
			"migration_total":  mc.migrationDurationTotal,
			"export_avg":       avgExportDuration,
			"conversion_avg":   avgConversionDuration,
			"upload_avg":       avgUploadDuration,
			"migration_avg":    avgMigrationDuration,
		},
		"bytes": map[string]interface{}{
			"exported":  mc.bytesExported,
			"converted": mc.bytesConverted,
			"uploaded":  mc.bytesUploaded,
		},
		"providers": mc.providerCounts,
		"uptime":    time.Since(mc.startTime).Seconds(),
	}
}

// Reset resets all metrics (useful for testing)
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.migrationsTotal = 0
	mc.migrationsSucceeded = 0
	mc.migrationsFailed = 0
	mc.activeMigrations = 0
	mc.exportDurationTotal = 0
	mc.conversionDurationTotal = 0
	mc.uploadDurationTotal = 0
	mc.migrationDurationTotal = 0
	mc.bytesExported = 0
	mc.bytesConverted = 0
	mc.bytesUploaded = 0
	mc.providerCounts = make(map[string]int64)
	mc.startTime = time.Now()
}

// MetricsServer serves Prometheus metrics via HTTP
type MetricsServer struct {
	collector *MetricsCollector
	addr      string
	server    *http.Server
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(collector *MetricsCollector, addr string) *MetricsServer {
	return &MetricsServer{
		collector: collector,
		addr:      addr,
	}
}

// Start starts the metrics server
func (ms *MetricsServer) Start() error {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.HandleFunc("/metrics", ms.handleMetrics)

	// JSON stats endpoint
	mux.HandleFunc("/stats", ms.handleStats)

	// Health check endpoint
	mux.HandleFunc("/health", ms.handleHealth)

	ms.server = &http.Server{
		Addr:              ms.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return ms.server.ListenAndServe()
}

// Stop stops the metrics server
func (ms *MetricsServer) Stop() error {
	if ms.server != nil {
		return ms.server.Close()
	}
	return nil
}

// handleMetrics serves Prometheus metrics
func (ms *MetricsServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	if _, err := w.Write([]byte(ms.collector.GetMetrics())); err != nil {
		log.Printf("failed to write metrics response: %v", err)
	}
}

// handleStats serves JSON stats
func (ms *MetricsServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	stats := ms.collector.GetStats()

	// Use json package to encode
	data, err := json.Marshal(stats)
	if err != nil {
		http.Error(w, "failed to marshal stats", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(data); err != nil {
		log.Printf("failed to write stats response: %v", err)
	}
}

// handleHealth serves health check
func (ms *MetricsServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("failed to write health response: %v", err)
	}
}
