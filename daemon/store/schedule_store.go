// SPDX-License-Identifier: Apache-2.0

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
)

// ScheduleStore provides persistent storage for scheduled jobs
type ScheduleStore interface {
	SaveSchedule(sj *models.ScheduledJob) error
	UpdateSchedule(sj *models.ScheduledJob) error
	GetSchedule(id string) (*models.ScheduledJob, error)
	ListSchedules(enabled *bool) ([]*models.ScheduledJob, error)
	DeleteSchedule(id string) error
	RecordExecution(execution *ScheduleExecution) error
	GetExecutionHistory(scheduleID string, limit int) ([]*ScheduleExecution, error)
}

// ScheduleExecution represents a single execution of a scheduled job
type ScheduleExecution struct {
	ID              int
	ScheduleID      string
	JobID           string
	ExecutedAt      time.Time
	Status          string
	DurationSeconds float64
	Error           string
}

// SaveSchedule persists a new scheduled job
func (s *SQLiteStore) SaveSchedule(sj *models.ScheduledJob) error {
	jobTemplateJSON, err := json.Marshal(sj.JobTemplate)
	if err != nil {
		return fmt.Errorf("failed to marshal job template: %w", err)
	}

	tagsJSON, err := json.Marshal(sj.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		INSERT INTO scheduled_jobs (
			id, name, description, schedule, job_template_json,
			enabled, created_at, updated_at, last_run, next_run,
			run_count, tags_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		sj.ID,
		sj.Name,
		sj.Description,
		sj.Schedule,
		string(jobTemplateJSON),
		sj.Enabled,
		sj.CreatedAt,
		sj.UpdatedAt,
		sj.LastRun,
		sj.NextRun,
		sj.RunCount,
		string(tagsJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to insert scheduled job: %w", err)
	}

	return nil
}

// UpdateSchedule updates an existing scheduled job
func (s *SQLiteStore) UpdateSchedule(sj *models.ScheduledJob) error {
	jobTemplateJSON, err := json.Marshal(sj.JobTemplate)
	if err != nil {
		return fmt.Errorf("failed to marshal job template: %w", err)
	}

	tagsJSON, err := json.Marshal(sj.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		UPDATE scheduled_jobs SET
			name = ?,
			description = ?,
			schedule = ?,
			job_template_json = ?,
			enabled = ?,
			updated_at = ?,
			last_run = ?,
			next_run = ?,
			run_count = ?,
			tags_json = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query,
		sj.Name,
		sj.Description,
		sj.Schedule,
		string(jobTemplateJSON),
		sj.Enabled,
		sj.UpdatedAt,
		sj.LastRun,
		sj.NextRun,
		sj.RunCount,
		string(tagsJSON),
		sj.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update scheduled job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("scheduled job not found: %s", sj.ID)
	}

	return nil
}

// GetSchedule retrieves a scheduled job by ID
func (s *SQLiteStore) GetSchedule(id string) (*models.ScheduledJob, error) {
	query := `
		SELECT
			id, name, description, schedule, job_template_json,
			enabled, created_at, updated_at, last_run, next_run,
			run_count, tags_json
		FROM scheduled_jobs
		WHERE id = ?
	`

	row := s.db.QueryRow(query, id)

	var sj models.ScheduledJob
	var jobTemplateJSON, tagsJSON string
	var lastRun, nextRun sql.NullTime

	err := row.Scan(
		&sj.ID,
		&sj.Name,
		&sj.Description,
		&sj.Schedule,
		&jobTemplateJSON,
		&sj.Enabled,
		&sj.CreatedAt,
		&sj.UpdatedAt,
		&lastRun,
		&nextRun,
		&sj.RunCount,
		&tagsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("scheduled job not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query scheduled job: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal([]byte(jobTemplateJSON), &sj.JobTemplate); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job template: %w", err)
	}

	if err := json.Unmarshal([]byte(tagsJSON), &sj.Tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}

	if lastRun.Valid {
		sj.LastRun = &lastRun.Time
	}
	if nextRun.Valid {
		sj.NextRun = nextRun.Time
	}

	return &sj, nil
}

// ListSchedules retrieves all scheduled jobs (optionally filtered by enabled status)
func (s *SQLiteStore) ListSchedules(enabled *bool) ([]*models.ScheduledJob, error) {
	query := `
		SELECT
			id, name, description, schedule, job_template_json,
			enabled, created_at, updated_at, last_run, next_run,
			run_count, tags_json
		FROM scheduled_jobs
	`
	args := []interface{}{}

	if enabled != nil {
		query += " WHERE enabled = ?"
		args = append(args, *enabled)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query scheduled jobs: %w", err)
	}
	defer rows.Close()

	var schedules []*models.ScheduledJob
	for rows.Next() {
		var sj models.ScheduledJob
		var jobTemplateJSON, tagsJSON string
		var lastRun, nextRun sql.NullTime

		err := rows.Scan(
			&sj.ID,
			&sj.Name,
			&sj.Description,
			&sj.Schedule,
			&jobTemplateJSON,
			&sj.Enabled,
			&sj.CreatedAt,
			&sj.UpdatedAt,
			&lastRun,
			&nextRun,
			&sj.RunCount,
			&tagsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan scheduled job: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal([]byte(jobTemplateJSON), &sj.JobTemplate); err != nil {
			return nil, fmt.Errorf("failed to unmarshal job template: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &sj.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}

		if lastRun.Valid {
			sj.LastRun = &lastRun.Time
		}
		if nextRun.Valid {
			sj.NextRun = nextRun.Time
		}

		schedules = append(schedules, &sj)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return schedules, nil
}

// DeleteSchedule removes a scheduled job
func (s *SQLiteStore) DeleteSchedule(id string) error {
	result, err := s.db.Exec("DELETE FROM scheduled_jobs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete scheduled job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("scheduled job not found: %s", id)
	}

	return nil
}

// RecordExecution records the execution of a scheduled job
func (s *SQLiteStore) RecordExecution(execution *ScheduleExecution) error {
	query := `
		INSERT INTO schedule_executions (
			schedule_id, job_id, executed_at, status, duration_seconds, error
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		execution.ScheduleID,
		execution.JobID,
		execution.ExecutedAt,
		execution.Status,
		execution.DurationSeconds,
		execution.Error,
	)

	if err != nil {
		return fmt.Errorf("failed to record execution: %w", err)
	}

	return nil
}

// GetExecutionHistory retrieves execution history for a scheduled job
func (s *SQLiteStore) GetExecutionHistory(scheduleID string, limit int) ([]*ScheduleExecution, error) {
	query := `
		SELECT id, schedule_id, job_id, executed_at, status, duration_seconds, error
		FROM schedule_executions
		WHERE schedule_id = ?
		ORDER BY executed_at DESC
	`
	args := []interface{}{scheduleID}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query execution history: %w", err)
	}
	defer rows.Close()

	var executions []*ScheduleExecution
	for rows.Next() {
		var exec ScheduleExecution
		var errorMsg sql.NullString

		err := rows.Scan(
			&exec.ID,
			&exec.ScheduleID,
			&exec.JobID,
			&exec.ExecutedAt,
			&exec.Status,
			&exec.DurationSeconds,
			&errorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		if errorMsg.Valid {
			exec.Error = errorMsg.String
		}

		executions = append(executions, &exec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return executions, nil
}
