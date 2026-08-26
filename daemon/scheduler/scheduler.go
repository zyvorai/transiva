// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// Scheduler manages scheduled jobs
type Scheduler struct {
	cron        *cron.Cron
	jobs        map[string]*models.ScheduledJob
	cronEntries map[string]cron.EntryID // Maps schedule ID to cron entry ID
	mu          sync.RWMutex
	log         logger.Logger
	executor    JobExecutor
	store       ScheduleStore // Persistent storage (optional)
}

// ScheduleStore interface for persistent storage
type ScheduleStore interface {
	SaveSchedule(sj *models.ScheduledJob) error
	UpdateSchedule(sj *models.ScheduledJob) error
	GetSchedule(id string) (*models.ScheduledJob, error)
	ListSchedules(enabled *bool) ([]*models.ScheduledJob, error)
	DeleteSchedule(id string) error
}

// JobExecutor interface for executing jobs
type JobExecutor interface {
	SubmitJob(definition models.JobDefinition) (string, error)
}

// NewScheduler creates a new job scheduler
func NewScheduler(executor JobExecutor, log logger.Logger) *Scheduler {
	return &Scheduler{
		cron:        cron.New(),
		jobs:        make(map[string]*models.ScheduledJob),
		cronEntries: make(map[string]cron.EntryID),
		log:         log,
		executor:    executor,
		store:       nil, // Set later via SetStore if persistence is needed
	}
}

// SetStore sets the persistent storage for scheduled jobs
func (s *Scheduler) SetStore(store ScheduleStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = store
	if store != nil {
		s.log.Info("schedule persistence enabled")
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.log.Info("Starting job scheduler")
	s.cron.Start()
}

// LoadSchedules restores scheduled jobs from persistent storage
func (s *Scheduler) LoadSchedules() error {
	if s.store == nil {
		s.log.Debug("No schedule store configured, skipping load")
		return nil
	}

	s.log.Info("Loading schedules from persistent storage")

	schedules, err := s.store.ListSchedules(nil) // Load all schedules
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var restored, failed int
	for _, sj := range schedules {
		s.jobs[sj.ID] = sj

		// Re-add to cron if enabled
		if sj.Enabled {
			entryID, err := s.cron.AddFunc(sj.Schedule, s.createJobFunc(sj))
			if err != nil {
				s.log.Error("Failed to restore schedule",
					"id", sj.ID,
					"name", sj.Name,
					"error", err)
				failed++
				continue
			}
			s.cronEntries[sj.ID] = entryID

			// Update next run time
			entry := s.cron.Entry(entryID)
			sj.NextRun = entry.Next

			restored++
		}
	}

	s.log.Info("Schedules loaded from storage",
		"total", len(schedules),
		"restored", restored,
		"failed", failed)

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.log.Info("Stopping job scheduler")
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// AddScheduledJob adds a new scheduled job
func (s *Scheduler) AddScheduledJob(sj *models.ScheduledJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate cron schedule
	if _, err := s.cron.AddFunc(sj.Schedule, func() {}); err != nil {
		return fmt.Errorf("invalid cron schedule: %w", err)
	}

	// Set timestamps
	sj.CreatedAt = time.Now()
	sj.UpdatedAt = time.Now()

	// Add to cron if enabled
	if sj.Enabled {
		entryID, err := s.cron.AddFunc(sj.Schedule, s.createJobFunc(sj))
		if err != nil {
			return fmt.Errorf("failed to schedule job: %w", err)
		}
		s.cronEntries[sj.ID] = entryID

		// Calculate next run
		entry := s.cron.Entry(entryID)
		sj.NextRun = entry.Next
	}

	s.jobs[sj.ID] = sj
	s.log.Info("Scheduled job added",
		"id", sj.ID,
		"name", sj.Name,
		"schedule", sj.Schedule,
		"enabled", sj.Enabled)

	// Persist to store if available
	if s.store != nil {
		if err := s.store.SaveSchedule(sj); err != nil {
			s.log.Error("Failed to persist scheduled job", "id", sj.ID, "error", err)
			// Continue anyway - in-memory schedule is still valid
		}
	}

	return nil
}

// RemoveScheduledJob removes a scheduled job
func (s *Scheduler) RemoveScheduledJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sj, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("scheduled job not found: %s", id)
	}

	// Remove from cron
	if sj.Enabled {
		s.cron.Remove(s.cronEntries[sj.ID])
	}

	delete(s.jobs, id)
	s.log.Info("Scheduled job removed", "id", id, "name", sj.Name)

	// Delete from store if available
	if s.store != nil {
		if err := s.store.DeleteSchedule(id); err != nil {
			s.log.Error("Failed to delete scheduled job from store", "id", id, "error", err)
			// Continue anyway - already removed from memory
		}
	}

	return nil
}

// UpdateScheduledJob updates an existing scheduled job
func (s *Scheduler) UpdateScheduledJob(id string, updates *models.ScheduledJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sj, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("scheduled job not found: %s", id)
	}

	// Remove old cron entry
	if sj.Enabled {
		s.cron.Remove(s.cronEntries[sj.ID])
	}

	// Update fields
	if updates.Name != "" {
		sj.Name = updates.Name
	}
	if updates.Description != "" {
		sj.Description = updates.Description
	}
	if updates.Schedule != "" {
		sj.Schedule = updates.Schedule
	}
	if updates.JobTemplate.VMPath != "" {
		sj.JobTemplate = updates.JobTemplate
	}
	if updates.Tags != nil {
		sj.Tags = updates.Tags
	}

	sj.Enabled = updates.Enabled
	sj.UpdatedAt = time.Now()

	// Re-add to cron if enabled
	if sj.Enabled {
		entryID, err := s.cron.AddFunc(sj.Schedule, s.createJobFunc(sj))
		if err != nil {
			return fmt.Errorf("failed to reschedule job: %w", err)
		}
		s.cronEntries[sj.ID] = entryID

		// Update next run
		entry := s.cron.Entry(entryID)
		sj.NextRun = entry.Next
	}

	s.log.Info("Scheduled job updated", "id", id, "name", sj.Name)

	// Persist to store if available
	if s.store != nil {
		if err := s.store.UpdateSchedule(sj); err != nil {
			s.log.Error("Failed to persist scheduled job update", "id", id, "error", err)
			// Continue anyway - in-memory schedule is updated
		}
	}

	return nil
}

// GetScheduledJob retrieves a scheduled job by ID
func (s *Scheduler) GetScheduledJob(id string) (*models.ScheduledJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sj, exists := s.jobs[id]
	if !exists {
		return nil, fmt.Errorf("scheduled job not found: %s", id)
	}

	return sj, nil
}

// ListScheduledJobs returns all scheduled jobs
func (s *Scheduler) ListScheduledJobs() []*models.ScheduledJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*models.ScheduledJob, 0, len(s.jobs))
	for _, sj := range s.jobs {
		// Update next run time
		if sj.Enabled {
			entry := s.cron.Entry(s.cronEntries[sj.ID])
			sj.NextRun = entry.Next
		}
		jobs = append(jobs, sj)
	}

	return jobs
}

// EnableScheduledJob enables a scheduled job
func (s *Scheduler) EnableScheduledJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sj, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("scheduled job not found: %s", id)
	}

	if sj.Enabled {
		return nil // Already enabled
	}

	// Add to cron
	entryID, err := s.cron.AddFunc(sj.Schedule, s.createJobFunc(sj))
	if err != nil {
		return fmt.Errorf("failed to enable job: %w", err)
	}

	s.cronEntries[sj.ID] = entryID
	sj.Enabled = true
	sj.UpdatedAt = time.Now()

	// Update next run
	entry := s.cron.Entry(entryID)
	sj.NextRun = entry.Next

	s.log.Info("Scheduled job enabled", "id", id, "name", sj.Name)

	return nil
}

// DisableScheduledJob disables a scheduled job
func (s *Scheduler) DisableScheduledJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sj, exists := s.jobs[id]
	if !exists {
		return fmt.Errorf("scheduled job not found: %s", id)
	}

	if !sj.Enabled {
		return nil // Already disabled
	}

	// Remove from cron
	s.cron.Remove(s.cronEntries[sj.ID])
	sj.Enabled = false
	sj.UpdatedAt = time.Now()

	s.log.Info("Scheduled job disabled", "id", id, "name", sj.Name)

	return nil
}

// TriggerNow manually triggers a scheduled job
func (s *Scheduler) TriggerNow(id string) error {
	s.mu.RLock()
	sj, exists := s.jobs[id]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scheduled job not found: %s", id)
	}

	s.log.Info("Manually triggering scheduled job", "id", id, "name", sj.Name)

	// Execute job
	go s.executeScheduledJob(sj)

	return nil
}

// createJobFunc creates a function that executes a scheduled job
func (s *Scheduler) createJobFunc(sj *models.ScheduledJob) func() {
	return func() {
		s.executeScheduledJob(sj)
	}
}

// executeScheduledJob executes a scheduled job
func (s *Scheduler) executeScheduledJob(sj *models.ScheduledJob) {
	s.log.Info("Executing scheduled job",
		"id", sj.ID,
		"name", sj.Name,
		"schedule", sj.Schedule)

	// Create job definition with timestamp in name
	jobDef := sj.JobTemplate
	jobDef.ID = fmt.Sprintf("%s-%d", sj.ID, time.Now().Unix())
	jobDef.Name = fmt.Sprintf("%s (scheduled)", sj.JobTemplate.Name)

	// Execute job
	jobID, err := s.executor.SubmitJob(jobDef)
	if err != nil {
		s.log.Error("Failed to execute scheduled job",
			"id", sj.ID,
			"name", sj.Name,
			"error", err)
		return
	}

	s.log.Info("Submitted scheduled job",
		"schedule_id", sj.ID,
		"job_id", jobID)

	// Update statistics
	s.mu.Lock()
	now := time.Now()
	sj.LastRun = &now
	sj.RunCount++
	sj.UpdatedAt = now

	// Persist updated statistics if store is available
	store := s.store // Capture before unlock
	s.mu.Unlock()

	if store != nil {
		if err := store.UpdateSchedule(sj); err != nil {
			s.log.Error("Failed to persist schedule statistics",
				"id", sj.ID,
				"error", err)
		}
	}

	s.log.Info("Scheduled job executed successfully",
		"id", sj.ID,
		"name", sj.Name,
		"runCount", sj.RunCount)
}

// GetScheduleStats returns statistics about scheduled jobs
func (s *Scheduler) GetScheduleStats() *ScheduleStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &ScheduleStats{
		TotalSchedules:   len(s.jobs),
		EnabledSchedules: 0,
		NextRunning:      nil,
	}

	for _, sj := range s.jobs {
		if sj.Enabled {
			stats.EnabledSchedules++
			stats.TotalRuns += sj.RunCount

			if stats.NextRunning == nil || sj.NextRun.Before(*stats.NextRunning) {
				stats.NextRunning = &sj.NextRun
			}
		}
	}

	return stats
}

// ScheduleStats holds statistics about scheduled jobs
type ScheduleStats struct {
	TotalSchedules   int        `json:"total_schedules"`
	EnabledSchedules int        `json:"enabled_schedules"`
	TotalRuns        int        `json:"total_runs"`
	NextRunning      *time.Time `json:"next_running,omitempty"`
}
