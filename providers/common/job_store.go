// SPDX-License-Identifier: Apache-2.0

package common

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// JobStatus represents the status of a migration job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a migration job
type Job struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	VMName      string                 `json:"vm_name"`
	VMPath      string                 `json:"vm_path"`
	Provider    string                 `json:"provider"`
	OutputDir   string                 `json:"output_dir"`
	Status      JobStatus              `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Progress    float64                `json:"progress"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	User        string                 `json:"user"`
	TotalSize   int64                  `json:"total_size"`
	FilesCount  int                    `json:"files_count"`
}

// JobFilter represents filtering options for listing jobs
type JobFilter struct {
	Status   JobStatus
	Provider string
	User     string
	Since    time.Time
	Until    time.Time
	Limit    int
	Offset   int
}

// JobStore defines the interface for job persistence
type JobStore interface {
	// SaveJob saves or updates a job
	SaveJob(job *Job) error

	// LoadJob loads a job by ID
	LoadJob(id string) (*Job, error)

	// ListJobs lists jobs with optional filtering
	ListJobs(filter JobFilter) ([]*Job, error)

	// UpdateJobStatus updates just the status of a job
	UpdateJobStatus(id string, status JobStatus) error

	// UpdateJobProgress updates the progress of a running job
	UpdateJobProgress(id string, progress float64) error

	// DeleteJob deletes a job by ID
	DeleteJob(id string) error

	// GetJobStats returns statistics about jobs
	GetJobStats() (*JobStats, error)

	// Close closes the store
	Close() error
}

// JobStats contains statistics about jobs
type JobStats struct {
	Total       int64
	Pending     int64
	Running     int64
	Completed   int64
	Failed      int64
	Cancelled   int64
	SuccessRate float64
}

// SQLiteJobStore implements JobStore using SQLite
type SQLiteJobStore struct {
	db *sql.DB
}

// NewSQLiteJobStore creates a new SQLite job store
func NewSQLiteJobStore(dbPath string) (*SQLiteJobStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("enable WAL mode: %w (additionally failed to close database: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	store := &SQLiteJobStore{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("initialize schema: %w (additionally failed to close database: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database schema
func (s *SQLiteJobStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		vm_name TEXT NOT NULL,
		vm_path TEXT NOT NULL,
		provider TEXT NOT NULL,
		output_dir TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		progress REAL DEFAULT 0,
		error TEXT,
		metadata TEXT,
		user TEXT,
		total_size INTEGER DEFAULT 0,
		files_count INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_created_at ON jobs(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_provider ON jobs(provider);
	CREATE INDEX IF NOT EXISTS idx_user ON jobs(user);
	CREATE INDEX IF NOT EXISTS idx_vm_name ON jobs(vm_name);

	-- Migration metadata table
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	INSERT OR IGNORE INTO schema_version (version) VALUES (1);
	`

	_, err := s.db.Exec(schema)
	return err
}

// SaveJob saves or updates a job
func (s *SQLiteJobStore) SaveJob(job *Job) error {
	metadataJSON, err := json.Marshal(job.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO jobs (
			id, name, vm_name, vm_path, provider, output_dir, status,
			created_at, started_at, completed_at, progress, error,
			metadata, user, total_size, files_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		job.ID,
		job.Name,
		job.VMName,
		job.VMPath,
		job.Provider,
		job.OutputDir,
		job.Status,
		job.CreatedAt,
		job.StartedAt,
		job.CompletedAt,
		job.Progress,
		job.Error,
		string(metadataJSON),
		job.User,
		job.TotalSize,
		job.FilesCount,
	)

	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	return nil
}

// LoadJob loads a job by ID
func (s *SQLiteJobStore) LoadJob(id string) (*Job, error) {
	query := `
		SELECT id, name, vm_name, vm_path, provider, output_dir, status,
		       created_at, started_at, completed_at, progress, error,
		       metadata, user, total_size, files_count
		FROM jobs
		WHERE id = ?
	`

	var job Job
	var metadataJSON string
	var startedAt, completedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&job.ID,
		&job.Name,
		&job.VMName,
		&job.VMPath,
		&job.Provider,
		&job.OutputDir,
		&job.Status,
		&job.CreatedAt,
		&startedAt,
		&completedAt,
		&job.Progress,
		&job.Error,
		&metadataJSON,
		&job.User,
		&job.TotalSize,
		&job.FilesCount,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query job: %w", err)
	}

	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &job.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	return &job, nil
}

// ListJobs lists jobs with optional filtering
func (s *SQLiteJobStore) ListJobs(filter JobFilter) ([]*Job, error) {
	query := `
		SELECT id, name, vm_name, vm_path, provider, output_dir, status,
		       created_at, started_at, completed_at, progress, error,
		       metadata, user, total_size, files_count
		FROM jobs
		WHERE 1=1
	`
	args := []interface{}{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	if filter.Provider != "" {
		query += " AND provider = ?"
		args = append(args, filter.Provider)
	}

	if filter.User != "" {
		query += " AND user = ?"
		args = append(args, filter.User)
	}

	if !filter.Since.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.Since)
	}

	if !filter.Until.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, filter.Until)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var job Job
		var metadataJSON string
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Name,
			&job.VMName,
			&job.VMPath,
			&job.Provider,
			&job.OutputDir,
			&job.Status,
			&job.CreatedAt,
			&startedAt,
			&completedAt,
			&job.Progress,
			&job.Error,
			&metadataJSON,
			&job.User,
			&job.TotalSize,
			&job.FilesCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}

		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &job.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}

		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// UpdateJobStatus updates just the status of a job
func (s *SQLiteJobStore) UpdateJobStatus(id string, status JobStatus) error {
	query := `UPDATE jobs SET status = ? WHERE id = ?`
	result, err := s.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// UpdateJobProgress updates the progress of a running job
func (s *SQLiteJobStore) UpdateJobProgress(id string, progress float64) error {
	query := `UPDATE jobs SET progress = ? WHERE id = ?`
	result, err := s.db.Exec(query, progress, id)
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// DeleteJob deletes a job by ID
func (s *SQLiteJobStore) DeleteJob(id string) error {
	query := `DELETE FROM jobs WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// GetJobStats returns statistics about jobs
func (s *SQLiteJobStore) GetJobStats() (*JobStats, error) {
	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) as cancelled
		FROM jobs
	`

	var stats JobStats
	err := s.db.QueryRow(query).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Running,
		&stats.Completed,
		&stats.Failed,
		&stats.Cancelled,
	)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}

	// Calculate success rate
	if stats.Total > 0 {
		stats.SuccessRate = float64(stats.Completed) / float64(stats.Total) * 100
	}

	return &stats, nil
}

// Close closes the database connection
func (s *SQLiteJobStore) Close() error {
	return s.db.Close()
}

// Prune removes old completed jobs to prevent database growth
func (s *SQLiteJobStore) Prune(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	query := `DELETE FROM jobs WHERE status IN ('completed', 'failed', 'cancelled') AND completed_at < ?`
	result, err := s.db.Exec(query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune jobs: %w", err)
	}

	return result.RowsAffected()
}
