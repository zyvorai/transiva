// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

// DependencyTracker tracks job dependencies and their completion states
type DependencyTracker struct {
	mu            sync.RWMutex
	jobStates     map[string]JobState      // jobID -> current state
	waitingJobs   map[string][]*WaitingJob // dependencyID -> jobs waiting for it
	log           logger.Logger
	stateCallback StateChangeCallback
}

// JobState represents the current state of a job execution
type JobState struct {
	JobID       string
	State       string // running, completed, failed, cancelled
	CompletedAt time.Time
	Error       string
}

// WaitingJob represents a job waiting for dependencies
type WaitingJob struct {
	Job          *models.ScheduledJob
	Config       *models.AdvancedScheduleConfig
	WaitingSince time.Time
	Context      context.Context
	Cancel       context.CancelFunc
}

// StateChangeCallback is called when a job state changes
type StateChangeCallback func(jobID, state string, err error)

// NewDependencyTracker creates a new dependency tracker
func NewDependencyTracker(log logger.Logger) *DependencyTracker {
	return &DependencyTracker{
		jobStates:   make(map[string]JobState),
		waitingJobs: make(map[string][]*WaitingJob),
		log:         log,
	}
}

// SetStateCallback sets the callback for state changes
func (dt *DependencyTracker) SetStateCallback(callback StateChangeCallback) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.stateCallback = callback
}

// CheckDependencies checks if all dependencies for a job are satisfied
func (dt *DependencyTracker) CheckDependencies(job *models.ScheduledJob) (bool, string) {
	if job.AdvancedConfig == nil || len(job.AdvancedConfig.DependsOn) == 0 {
		return true, "" // No dependencies
	}

	dt.mu.RLock()
	defer dt.mu.RUnlock()

	for _, dep := range job.AdvancedConfig.DependsOn {
		state, exists := dt.jobStates[dep.JobID]

		if !exists {
			return false, fmt.Sprintf("dependency job %s has not run yet", dep.JobID)
		}

		// Check if dependency is in required state
		switch dep.RequiredState {
		case "completed":
			if state.State != "completed" {
				return false, fmt.Sprintf("dependency job %s is not completed (state: %s)", dep.JobID, state.State)
			}
		case "failed":
			if state.State != "failed" {
				return false, fmt.Sprintf("dependency job %s has not failed (state: %s)", dep.JobID, state.State)
			}
		case "any":
			if state.State != "completed" && state.State != "failed" {
				return false, fmt.Sprintf("dependency job %s is still running", dep.JobID)
			}
		default:
			return false, fmt.Sprintf("unknown required state: %s", dep.RequiredState)
		}

		// Check timeout if specified
		if dep.Timeout > 0 {
			elapsed := time.Since(state.CompletedAt)
			if elapsed > time.Duration(dep.Timeout)*time.Second {
				return false, fmt.Sprintf("dependency job %s completed too long ago (timeout: %ds)", dep.JobID, dep.Timeout)
			}
		}
	}

	return true, ""
}

// WaitForDependencies waits for all dependencies to be satisfied
func (dt *DependencyTracker) WaitForDependencies(ctx context.Context, job *models.ScheduledJob) error {
	if job.AdvancedConfig == nil || len(job.AdvancedConfig.DependsOn) == 0 {
		return nil // No dependencies
	}

	dt.log.Info("waiting for dependencies",
		"job", job.Name,
		"dependencies", len(job.AdvancedConfig.DependsOn))

	// Register this job as waiting for its dependencies
	dt.mu.Lock()
	waitCtx, cancel := context.WithCancel(ctx)
	waiting := &WaitingJob{
		Job:          job,
		Config:       job.AdvancedConfig,
		WaitingSince: time.Now(),
		Context:      waitCtx,
		Cancel:       cancel,
	}

	for _, dep := range job.AdvancedConfig.DependsOn {
		dt.waitingJobs[dep.JobID] = append(dt.waitingJobs[dep.JobID], waiting)
	}
	dt.mu.Unlock()

	// Wait for all dependencies
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			dt.removewaitingJob(job.ID)
			return ctx.Err()
		case <-waitCtx.Done():
			// Dependencies satisfied
			dt.removewaitingJob(job.ID)
			return nil
		case <-ticker.C:
			// Check dependencies periodically
			satisfied, reason := dt.CheckDependencies(job)
			if satisfied {
				dt.log.Info("dependencies satisfied", "job", job.Name)
				cancel()
				return nil
			}

			// Check for timeout
			for _, dep := range job.AdvancedConfig.DependsOn {
				if dep.Timeout > 0 {
					elapsed := time.Since(waiting.WaitingSince)
					if elapsed > time.Duration(dep.Timeout)*time.Second {
						dt.removewaitingJob(job.ID)
						return fmt.Errorf("dependency timeout: %s", reason)
					}
				}
			}
		}
	}
}

// UpdateJobState updates the state of a job
func (dt *DependencyTracker) UpdateJobState(jobID, state string, err error) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	jobState := JobState{
		JobID:       jobID,
		State:       state,
		CompletedAt: time.Now(),
	}
	if err != nil {
		jobState.Error = err.Error()
	}

	dt.jobStates[jobID] = jobState

	dt.log.Info("job state updated",
		"job", jobID,
		"state", state,
		"error", jobState.Error)

	// Notify state change callback
	if dt.stateCallback != nil {
		go dt.stateCallback(jobID, state, err)
	}

	// Check if any jobs are waiting for this dependency
	if waitingList, exists := dt.waitingJobs[jobID]; exists {
		dt.log.Info("notifying waiting jobs",
			"dependency", jobID,
			"waiting_count", len(waitingList))

		for _, waiting := range waitingList {
			// Check if all dependencies are now satisfied
			satisfied, _ := dt.checkDependenciesLocked(waiting.Job)
			if satisfied {
				dt.log.Info("dependencies satisfied for waiting job",
					"job", waiting.Job.Name)
				waiting.Cancel() // Signal that dependencies are satisfied
			}
		}
	}
}

// checkDependenciesLocked is like CheckDependencies but assumes lock is held
func (dt *DependencyTracker) checkDependenciesLocked(job *models.ScheduledJob) (bool, string) {
	if job.AdvancedConfig == nil || len(job.AdvancedConfig.DependsOn) == 0 {
		return true, ""
	}

	for _, dep := range job.AdvancedConfig.DependsOn {
		state, exists := dt.jobStates[dep.JobID]

		if !exists {
			return false, fmt.Sprintf("dependency job %s has not run yet", dep.JobID)
		}

		switch dep.RequiredState {
		case "completed":
			if state.State != "completed" {
				return false, fmt.Sprintf("dependency job %s is not completed", dep.JobID)
			}
		case "failed":
			if state.State != "failed" {
				return false, fmt.Sprintf("dependency job %s has not failed", dep.JobID)
			}
		case "any":
			if state.State != "completed" && state.State != "failed" {
				return false, fmt.Sprintf("dependency job %s is still running", dep.JobID)
			}
		}

		if dep.Timeout > 0 {
			elapsed := time.Since(state.CompletedAt)
			if elapsed > time.Duration(dep.Timeout)*time.Second {
				return false, "dependency timeout exceeded"
			}
		}
	}

	return true, ""
}

// removewaitingJob removes a job from the waiting list
func (dt *DependencyTracker) removewaitingJob(jobID string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	for depID, waitingList := range dt.waitingJobs {
		newList := make([]*WaitingJob, 0)
		for _, waiting := range waitingList {
			if waiting.Job.ID != jobID {
				newList = append(newList, waiting)
			}
		}
		if len(newList) > 0 {
			dt.waitingJobs[depID] = newList
		} else {
			delete(dt.waitingJobs, depID)
		}
	}
}

// GetJobState returns the current state of a job
func (dt *DependencyTracker) GetJobState(jobID string) (JobState, bool) {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	state, exists := dt.jobStates[jobID]
	return state, exists
}

// GetWaitingJobs returns all jobs waiting for a specific dependency
func (dt *DependencyTracker) GetWaitingJobs(depID string) []*WaitingJob {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	return dt.waitingJobs[depID]
}

// GetAllWaitingJobs returns all waiting jobs
func (dt *DependencyTracker) GetAllWaitingJobs() map[string][]*WaitingJob {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	result := make(map[string][]*WaitingJob)
	for k, v := range dt.waitingJobs {
		result[k] = v
	}
	return result
}

// ClearOldStates removes job states older than the specified duration
func (dt *DependencyTracker) ClearOldStates(maxAge time.Duration) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for jobID, state := range dt.jobStates {
		if state.CompletedAt.Before(cutoff) {
			delete(dt.jobStates, jobID)
			dt.log.Debug("cleared old job state", "job", jobID)
		}
	}
}
