// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// MetricsHistory handles historical metrics storage
type MetricsHistory struct {
	db             *sql.DB
	retentionDays  int
	enabled        bool
}

// HistoricalMetrics represents a snapshot of metrics at a point in time
type HistoricalMetrics struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalVMs        int       `json:"total_vms"`
	RunningVMs      int       `json:"running_vms"`
	StoppedVMs      int       `json:"stopped_vms"`
	FailedVMs       int       `json:"failed_vms"`
	TotalBackups    int       `json:"total_backups"`
	CompletedBackups int      `json:"completed_backups"`
	FailedBackups   int       `json:"failed_backups"`
	TotalRestores   int       `json:"total_restores"`
	TotalCPUs       int32     `json:"total_cpus"`
	TotalMemoryGi   float64   `json:"total_memory_gi"`
	AvgCarbonIntensity float64 `json:"avg_carbon_intensity"`
	CarbonAwareVMs  int       `json:"carbon_aware_vms"`
}

// MetricsTrend represents aggregated metrics over a time period
type MetricsTrend struct {
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	AvgVMs         float64   `json:"avg_vms"`
	MaxVMs         int       `json:"max_vms"`
	MinVMs         int       `json:"min_vms"`
	AvgRunningVMs  float64   `json:"avg_running_vms"`
	AvgCPUs        float64   `json:"avg_cpus"`
	AvgMemoryGi    float64   `json:"avg_memory_gi"`
	AvgCarbon      float64   `json:"avg_carbon"`
	TotalBackups   int       `json:"total_backups"`
	TotalRestores  int       `json:"total_restores"`
	DataPoints     int       `json:"data_points"`
}

// NewMetricsHistory creates a new metrics history storage
func NewMetricsHistory(dbPath string, retentionDays int) (*MetricsHistory, error) {
	if dbPath == "" {
		// History disabled
		return &MetricsHistory{enabled: false}, nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	mh := &MetricsHistory{
		db:            db,
		retentionDays: retentionDays,
		enabled:       true,
	}

	if err := mh.createTables(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("failed to close metrics database after createTables error: %v", closeErr)
		}
		return nil, err
	}

	return mh, nil
}

// createTables creates the necessary database tables
func (mh *MetricsHistory) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS metrics_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		total_vms INTEGER,
		running_vms INTEGER,
		stopped_vms INTEGER,
		failed_vms INTEGER,
		total_backups INTEGER,
		completed_backups INTEGER,
		failed_backups INTEGER,
		total_restores INTEGER,
		total_cpus INTEGER,
		total_memory_gi REAL,
		avg_carbon_intensity REAL,
		carbon_aware_vms INTEGER,
		raw_metrics TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_snapshots_timestamp ON metrics_snapshots(timestamp);
	CREATE INDEX IF NOT EXISTS idx_snapshots_created_at ON metrics_snapshots(created_at);
	`

	_, err := mh.db.Exec(schema)
	return err
}

// RecordSnapshot stores a metrics snapshot
func (mh *MetricsHistory) RecordSnapshot(metrics *K8sMetrics) error {
	if !mh.enabled {
		return nil
	}

	// Serialize full metrics as JSON for detailed queries
	rawMetrics, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to serialize metrics: %w", err)
	}

	query := `
		INSERT INTO metrics_snapshots (
			timestamp, total_vms, running_vms, stopped_vms, failed_vms,
			total_backups, completed_backups, failed_backups, total_restores,
			total_cpus, total_memory_gi, avg_carbon_intensity, carbon_aware_vms,
			raw_metrics
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = mh.db.Exec(query,
		metrics.Timestamp,
		metrics.VirtualMachines.Total,
		metrics.VirtualMachines.Running,
		metrics.VirtualMachines.Stopped,
		metrics.VirtualMachines.Failed,
		metrics.BackupJobs.Total,
		metrics.BackupJobs.Completed,
		metrics.BackupJobs.Failed,
		metrics.RestoreJobs.Total,
		metrics.VMResourceStats.TotalCPUs,
		metrics.VMResourceStats.TotalMemoryGi,
		metrics.VMResourceStats.AvgCarbonIntensity,
		metrics.VMResourceStats.CarbonAwareVMs,
		string(rawMetrics),
	)

	return err
}

// GetHistory retrieves historical metrics for a time range
func (mh *MetricsHistory) GetHistory(startTime, endTime time.Time) ([]HistoricalMetrics, error) {
	if !mh.enabled {
		return nil, fmt.Errorf("metrics history not enabled")
	}

	query := `
		SELECT timestamp, total_vms, running_vms, stopped_vms, failed_vms,
		       total_backups, completed_backups, failed_backups, total_restores,
		       total_cpus, total_memory_gi, avg_carbon_intensity, carbon_aware_vms
		FROM metrics_snapshots
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp ASC
	`

	rows, err := mh.db.Query(query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var history []HistoricalMetrics
	for rows.Next() {
		var m HistoricalMetrics
		err := rows.Scan(
			&m.Timestamp, &m.TotalVMs, &m.RunningVMs, &m.StoppedVMs, &m.FailedVMs,
			&m.TotalBackups, &m.CompletedBackups, &m.FailedBackups, &m.TotalRestores,
			&m.TotalCPUs, &m.TotalMemoryGi, &m.AvgCarbonIntensity, &m.CarbonAwareVMs,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, m)
	}

	return history, rows.Err()
}

// GetTrend calculates aggregated trends for a time range
func (mh *MetricsHistory) GetTrend(startTime, endTime time.Time) (*MetricsTrend, error) {
	if !mh.enabled {
		return nil, fmt.Errorf("metrics history not enabled")
	}

	var trend MetricsTrend
	trend.StartTime = startTime
	trend.EndTime = endTime

	// Simplified query without window functions for better compatibility
	simpleQuery := `
		SELECT
			COUNT(*) as data_points,
			AVG(total_vms) as avg_vms,
			MAX(total_vms) as max_vms,
			MIN(total_vms) as min_vms,
			AVG(running_vms) as avg_running_vms,
			AVG(total_cpus) as avg_cpus,
			AVG(total_memory_gi) as avg_memory_gi,
			AVG(avg_carbon_intensity) as avg_carbon
		FROM metrics_snapshots
		WHERE timestamp BETWEEN ? AND ?
	`

	err := mh.db.QueryRow(simpleQuery, startTime, endTime).Scan(
		&trend.DataPoints,
		&trend.AvgVMs,
		&trend.MaxVMs,
		&trend.MinVMs,
		&trend.AvgRunningVMs,
		&trend.AvgCPUs,
		&trend.AvgMemoryGi,
		&trend.AvgCarbon,
	)
	if err != nil {
		return nil, err
	}

	// Get backup/restore counts separately
	backupQuery := `
		SELECT
			MAX(completed_backups) - MIN(completed_backups) as new_backups,
			MAX(total_restores) - MIN(total_restores) as new_restores
		FROM metrics_snapshots
		WHERE timestamp BETWEEN ? AND ?
	`
	if err := mh.db.QueryRow(backupQuery, startTime, endTime).Scan(
		&trend.TotalBackups,
		&trend.TotalRestores,
	); err != nil {
		log.Printf("failed to query backup/restore trend: %v", err)
	}

	return &trend, nil
}

// CleanupOldData removes data older than retention period
func (mh *MetricsHistory) CleanupOldData() error {
	if !mh.enabled {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -mh.retentionDays)

	query := `DELETE FROM metrics_snapshots WHERE timestamp < ?`
	result, err := mh.db.Exec(query, cutoff)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		// Log cleanup
		fmt.Printf("Cleaned up %d old metrics snapshots (older than %v)\n", rows, cutoff)
	}

	return nil
}

// GetLatestSnapshot gets the most recent metrics snapshot
func (mh *MetricsHistory) GetLatestSnapshot() (*HistoricalMetrics, error) {
	if !mh.enabled {
		return nil, fmt.Errorf("metrics history not enabled")
	}

	query := `
		SELECT timestamp, total_vms, running_vms, stopped_vms, failed_vms,
		       total_backups, completed_backups, failed_backups, total_restores,
		       total_cpus, total_memory_gi, avg_carbon_intensity, carbon_aware_vms
		FROM metrics_snapshots
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var m HistoricalMetrics
	err := mh.db.QueryRow(query).Scan(
		&m.Timestamp, &m.TotalVMs, &m.RunningVMs, &m.StoppedVMs, &m.FailedVMs,
		&m.TotalBackups, &m.CompletedBackups, &m.FailedBackups, &m.TotalRestores,
		&m.TotalCPUs, &m.TotalMemoryGi, &m.AvgCarbonIntensity, &m.CarbonAwareVMs,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Close closes the database connection
func (mh *MetricsHistory) Close() error {
	if !mh.enabled || mh.db == nil {
		return nil
	}
	return mh.db.Close()
}

// IsEnabled returns whether metrics history is enabled
func (mh *MetricsHistory) IsEnabled() bool {
	return mh.enabled
}
