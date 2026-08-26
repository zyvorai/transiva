// SPDX-License-Identifier: Apache-2.0

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/zyvorai/transiva/daemon/models"
)

// JobStore provides persistent storage for jobs
type JobStore interface {
	SaveJob(job *models.Job) error
	UpdateJob(job *models.Job) error
	GetJob(id string) (*models.Job, error)
	ListJobs(filter JobFilter) ([]*models.Job, error)
	DeleteJob(id string) error
	Close() error
}

// JobFilter defines query filters for jobs
type JobFilter struct {
	Status []models.JobStatus
	Since  *time.Time
	Until  *time.Time
	Limit  int
	Offset int
}

// SQLiteStore implements JobStore using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-backed job store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("failed to close database after WAL mode error: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("failed to close database after schema init error: %v", closeErr)
		}
		return nil, err
	}

	return store, nil
}

// initSchema creates the database schema
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		vm_path TEXT NOT NULL,
		output_path TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		progress_json TEXT,
		result_json TEXT,
		error TEXT,
		definition_json TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_created_at ON jobs(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_vm_path ON jobs(vm_path);

	CREATE TABLE IF NOT EXISTS job_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id TEXT NOT NULL,
		status TEXT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		details TEXT,
		FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_job_history_job_id ON job_history(job_id);
	CREATE INDEX IF NOT EXISTS idx_job_history_timestamp ON job_history(timestamp DESC);

	CREATE TABLE IF NOT EXISTS scheduled_jobs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		schedule TEXT NOT NULL,
		job_template_json TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		last_run TIMESTAMP,
		next_run TIMESTAMP,
		run_count INTEGER DEFAULT 0,
		tags_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_scheduled_enabled ON scheduled_jobs(enabled);
	CREATE INDEX IF NOT EXISTS idx_scheduled_next_run ON scheduled_jobs(next_run);

	CREATE TABLE IF NOT EXISTS schedule_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		schedule_id TEXT NOT NULL,
		job_id TEXT NOT NULL,
		executed_at TIMESTAMP NOT NULL,
		status TEXT NOT NULL,
		duration_seconds REAL,
		error TEXT,
		FOREIGN KEY (schedule_id) REFERENCES scheduled_jobs(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_execution_schedule ON schedule_executions(schedule_id);
	CREATE INDEX IF NOT EXISTS idx_execution_time ON schedule_executions(executed_at DESC);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// SaveJob persists a new job to the database
func (s *SQLiteStore) SaveJob(job *models.Job) error {
	progressJSON, err := json.Marshal(job.Progress)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	resultJSON, err := json.Marshal(job.Result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	definitionJSON, err := json.Marshal(job.Definition)
	if err != nil {
		return fmt.Errorf("failed to marshal definition: %w", err)
	}

	query := `
		INSERT INTO jobs (
			id, name, vm_path, output_path, status, created_at,
			started_at, completed_at, progress_json, result_json,
			error, definition_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		job.Definition.ID,
		job.Definition.Name,
		job.Definition.VMPath,
		job.Definition.OutputPath,
		job.Status,
		job.Definition.CreatedAt,
		job.StartedAt,
		job.CompletedAt,
		string(progressJSON),
		string(resultJSON),
		job.Error,
		string(definitionJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}

	// Record initial history
	return s.addHistory(job.Definition.ID, job.Status, "Job created")
}

// UpdateJob updates an existing job in the database
func (s *SQLiteStore) UpdateJob(job *models.Job) error {
	progressJSON, err := json.Marshal(job.Progress)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	resultJSON, err := json.Marshal(job.Result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	query := `
		UPDATE jobs SET
			status = ?,
			started_at = ?,
			completed_at = ?,
			progress_json = ?,
			result_json = ?,
			error = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query,
		job.Status,
		job.StartedAt,
		job.CompletedAt,
		string(progressJSON),
		string(resultJSON),
		job.Error,
		job.Definition.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", job.Definition.ID)
	}

	// Record status change in history
	return s.addHistory(job.Definition.ID, job.Status, "Status updated")
}

// GetJob retrieves a job by ID
func (s *SQLiteStore) GetJob(id string) (*models.Job, error) {
	query := `
		SELECT
			id, name, vm_path, output_path, status, created_at,
			started_at, completed_at, progress_json, result_json,
			error, definition_json
		FROM jobs
		WHERE id = ?
	`

	row := s.db.QueryRow(query, id)

	var job models.Job
	var progressJSON, resultJSON, definitionJSON string
	var startedAt, completedAt sql.NullTime
	var createdAt time.Time

	err := row.Scan(
		&job.Definition.ID,
		&job.Definition.Name,
		&job.Definition.VMPath,
		&job.Definition.OutputPath,
		&job.Status,
		&createdAt,
		&startedAt,
		&completedAt,
		&progressJSON,
		&resultJSON,
		&job.Error,
		&definitionJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query job: %w", err)
	}

	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal([]byte(progressJSON), &job.Progress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal progress: %w", err)
	}

	if err := json.Unmarshal([]byte(resultJSON), &job.Result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	var definition models.JobDefinition
	if err := json.Unmarshal([]byte(definitionJSON), &definition); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition: %w", err)
	}
	definition.CreatedAt = createdAt
	job.Definition = definition

	return &job, nil
}

// ListJobs retrieves jobs matching the filter
func (s *SQLiteStore) ListJobs(filter JobFilter) ([]*models.Job, error) {
	query := `
		SELECT
			id, name, vm_path, output_path, status, created_at,
			started_at, completed_at, progress_json, result_json,
			error, definition_json
		FROM jobs
		WHERE 1=1
	`
	args := []interface{}{}

	// Apply status filter
	if len(filter.Status) > 0 {
		placeholders := ""
		for i, status := range filter.Status {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, status)
		}
		// #nosec G202 -- placeholders contains only "?" and "," characters generated from len(filter.Status); the actual status values are bound as parameterized args below, never concatenated into the query text
		query += fmt.Sprintf(" AND status IN (%s)", placeholders)
	}

	// Apply time filters
	if filter.Since != nil {
		query += " AND created_at >= ?"
		args = append(args, filter.Since)
	}
	if filter.Until != nil {
		query += " AND created_at <= ?"
		args = append(args, filter.Until)
	}

	// Order by creation time descending
	query += " ORDER BY created_at DESC"

	// Apply limit and offset
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
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var progressJSON, resultJSON, definitionJSON string
		var startedAt, completedAt sql.NullTime
		var createdAt time.Time

		err := rows.Scan(
			&job.Definition.ID,
			&job.Definition.Name,
			&job.Definition.VMPath,
			&job.Definition.OutputPath,
			&job.Status,
			&createdAt,
			&startedAt,
			&completedAt,
			&progressJSON,
			&resultJSON,
			&job.Error,
			&definitionJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal([]byte(progressJSON), &job.Progress); err != nil {
			return nil, fmt.Errorf("failed to unmarshal progress: %w", err)
		}

		if err := json.Unmarshal([]byte(resultJSON), &job.Result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}

		var definition models.JobDefinition
		if err := json.Unmarshal([]byte(definitionJSON), &definition); err != nil {
			return nil, fmt.Errorf("failed to unmarshal definition: %w", err)
		}
		definition.CreatedAt = createdAt
		job.Definition = definition

		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return jobs, nil
}

// DeleteJob removes a job from the database
func (s *SQLiteStore) DeleteJob(id string) error {
	result, err := s.db.Exec("DELETE FROM jobs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// addHistory adds an entry to the job history table
func (s *SQLiteStore) addHistory(jobID string, status models.JobStatus, details string) error {
	query := `
		INSERT INTO job_history (job_id, status, timestamp, details)
		VALUES (?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, jobID, status, time.Now(), details)
	if err != nil {
		return fmt.Errorf("failed to add history: %w", err)
	}

	return nil
}

// GetJobHistory retrieves the history of a job
func (s *SQLiteStore) GetJobHistory(jobID string) ([]HistoryEntry, error) {
	query := `
		SELECT status, timestamp, details
		FROM job_history
		WHERE job_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := s.db.Query(query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var history []HistoryEntry
	for rows.Next() {
		var entry HistoryEntry
		if err := rows.Scan(&entry.Status, &entry.Timestamp, &entry.Details); err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}
		history = append(history, entry)
	}

	return history, nil
}

// HistoryEntry represents a single history record
type HistoryEntry struct {
	Status    models.JobStatus
	Timestamp time.Time
	Details   string
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetStatistics returns job statistics
func (s *SQLiteStore) GetStatistics() (*Statistics, error) {
	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END) as cancelled
		FROM jobs
	`

	var stats Statistics
	err := s.db.QueryRow(query).Scan(
		&stats.Total,
		&stats.Completed,
		&stats.Failed,
		&stats.Running,
		&stats.Pending,
		&stats.Cancelled,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	return &stats, nil
}

// Statistics holds job statistics
type Statistics struct {
	Total     int
	Completed int
	Failed    int
	Running   int
	Pending   int
	Cancelled int
}
