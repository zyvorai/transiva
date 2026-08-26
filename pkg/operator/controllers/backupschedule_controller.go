// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	transiva "github.com/zyvorai/transiva/pkg/apis/transiva/v1alpha1"
	"github.com/zyvorai/transiva/logger"
)

// BackupScheduleController manages BackupSchedule resources
type BackupScheduleController struct {
	clientset    kubernetes.Interface
	scheme       *runtime.Scheme
	recorder     record.EventRecorder
	logger       logger.Logger
	cronParser   cron.Parser
	syncInterval time.Duration
}

// NewBackupScheduleController creates a new BackupSchedule controller
func NewBackupScheduleController(
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	logger logger.Logger,
) *BackupScheduleController {
	return &BackupScheduleController{
		clientset:    clientset,
		scheme:       scheme,
		recorder:     recorder,
		logger:       logger,
		cronParser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		syncInterval: 60 * time.Second,
	}
}

// Reconcile handles BackupSchedule reconciliation
func (c *BackupScheduleController) Reconcile(ctx context.Context, schedule *transiva.BackupSchedule) error {
	c.logger.Info("reconciling BackupSchedule",
		"name", schedule.Name,
		"namespace", schedule.Namespace,
		"schedule", schedule.Spec.Schedule)

	// Check if schedule is suspended
	if schedule.Spec.Suspend {
		c.logger.Debug("schedule is suspended", "name", schedule.Name)
		return nil
	}

	// Parse cron schedule
	cronSchedule, err := c.cronParser.Parse(schedule.Spec.Schedule)
	if err != nil {
		c.logger.Error("invalid cron schedule", "error", err, "schedule", schedule.Spec.Schedule)
		c.recorder.Event(schedule, "Warning", "InvalidSchedule", fmt.Sprintf("Invalid cron schedule: %v", err))
		return err
	}

	// Determine timezone
	timezone := "UTC"
	if schedule.Spec.Timezone != "" {
		timezone = schedule.Spec.Timezone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		c.logger.Error("invalid timezone", "error", err, "timezone", timezone)
		loc = time.UTC
	}

	// Get current time in specified timezone
	now := time.Now().In(loc)

	// Calculate next run time
	var lastScheduleTime time.Time
	if schedule.Status.LastScheduleTime != nil {
		lastScheduleTime = schedule.Status.LastScheduleTime.Time
	}

	nextRunTime := cronSchedule.Next(lastScheduleTime)

	// Check if it's time to run
	if now.After(nextRunTime) || now.Equal(nextRunTime) {
		// Check starting deadline
		if schedule.Spec.StartingDeadlineSeconds != nil {
			deadline := nextRunTime.Add(time.Duration(*schedule.Spec.StartingDeadlineSeconds) * time.Second)
			if now.After(deadline) {
				c.logger.Warn("missed schedule deadline",
					"name", schedule.Name,
					"scheduled", nextRunTime,
					"deadline", deadline,
					"now", now)
				c.recorder.Event(schedule, "Warning", "MissedSchedule", "Missed scheduling deadline")

				// Update last schedule time to avoid repeatedly trying missed schedules
				schedule.Status.LastScheduleTime = &metav1.Time{Time: nextRunTime}
				return nil
			}
		}

		// Check concurrency policy
		canRun, err := c.checkConcurrencyPolicy(ctx, schedule)
		if err != nil {
			return err
		}

		if !canRun {
			c.logger.Info("skipping scheduled run due to concurrency policy",
				"name", schedule.Name,
				"policy", schedule.Spec.ConcurrencyPolicy)
			return nil
		}

		// Create BackupJob
		if err := c.createBackupJob(ctx, schedule); err != nil {
			c.logger.Error("failed to create backup job", "error", err)
			c.recorder.Event(schedule, "Warning", "JobCreationFailed", err.Error())
			return err
		}

		// Update last schedule time
		now := metav1.Now()
		schedule.Status.LastScheduleTime = &now

		c.logger.Info("scheduled backup job created",
			"name", schedule.Name,
			"next", cronSchedule.Next(now.Time))
	}

	// Clean up old jobs
	if err := c.cleanupOldJobs(ctx, schedule); err != nil {
		c.logger.Warn("failed to cleanup old jobs", "error", err)
	}

	return nil
}

// checkConcurrencyPolicy checks if a new job can run based on concurrency policy
func (c *BackupScheduleController) checkConcurrencyPolicy(ctx context.Context, schedule *transiva.BackupSchedule) (bool, error) {
	policy := schedule.Spec.ConcurrencyPolicy
	if policy == "" {
		policy = "Forbid" // Default
	}

	activeJobs := len(schedule.Status.Active)

	switch policy {
	case "Allow":
		// Always allow
		return true, nil

	case "Forbid":
		// Don't allow if there are active jobs
		if activeJobs > 0 {
			c.logger.Debug("forbidding new job due to active jobs",
				"name", schedule.Name,
				"active", activeJobs)
			return false, nil
		}
		return true, nil

	case "Replace":
		// Cancel active jobs and run new one
		if activeJobs > 0 {
			c.logger.Info("replacing active jobs",
				"name", schedule.Name,
				"active", activeJobs)

			// TODO: Cancel active backup jobs
			// For now, just proceed
		}
		return true, nil

	default:
		c.logger.Warn("unknown concurrency policy", "policy", policy)
		return false, fmt.Errorf("unknown concurrency policy: %s", policy)
	}
}

// createBackupJob creates a BackupJob from the schedule template
func (c *BackupScheduleController) createBackupJob(ctx context.Context, schedule *transiva.BackupSchedule) error {
	c.logger.Info("creating backup job from schedule", "name", schedule.Name)

	// Generate job name
	jobName := fmt.Sprintf("%s-%d", schedule.Name, time.Now().Unix())

	// Create BackupJob from template
	backupJob := &transiva.BackupJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: schedule.Namespace,
			Labels: map[string]string{
				"app":                          "transiva",
				"transiva.zyvor.ai/schedule":         schedule.Name,
				"transiva.zyvor.ai/scheduled-backup": "true",
			},
			Annotations: map[string]string{
				"transiva.zyvor.ai/scheduled-at": time.Now().Format(time.RFC3339),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "transiva.zyvor.ai/v1alpha1",
					Kind:       "BackupSchedule",
					Name:       schedule.Name,
					UID:        schedule.UID,
				},
			},
		},
		Spec: schedule.Spec.JobTemplate.Spec,
	}

	// Add template labels and annotations
	if schedule.Spec.JobTemplate.ObjectMeta.Labels != nil {
		for k, v := range schedule.Spec.JobTemplate.ObjectMeta.Labels {
			backupJob.Labels[k] = v
		}
	}

	if schedule.Spec.JobTemplate.ObjectMeta.Annotations != nil {
		for k, v := range schedule.Spec.JobTemplate.ObjectMeta.Annotations {
			backupJob.Annotations[k] = v
		}
	}

	// TODO: Create BackupJob using Kubernetes client
	// For now, just log
	c.logger.Info("backup job created from schedule",
		"schedule", schedule.Name,
		"job", jobName)

	c.recorder.Event(schedule, "Normal", "JobCreated", fmt.Sprintf("Created backup job: %s", jobName))

	// Add to active jobs
	schedule.Status.Active = append(schedule.Status.Active, transiva.ActiveJob{
		Name:      jobName,
		Namespace: schedule.Namespace,
		UID:       string(backupJob.UID),
	})

	return nil
}

// cleanupOldJobs removes old completed/failed jobs based on history limits
func (c *BackupScheduleController) cleanupOldJobs(ctx context.Context, schedule *transiva.BackupSchedule) error {
	successLimit := int32(3) // Default
	if schedule.Spec.SuccessfulJobsHistoryLimit != nil {
		successLimit = *schedule.Spec.SuccessfulJobsHistoryLimit
	}

	failedLimit := int32(1) // Default
	if schedule.Spec.FailedJobsHistoryLimit != nil {
		failedLimit = *schedule.Spec.FailedJobsHistoryLimit
	}

	c.logger.Debug("cleaning up old jobs",
		"name", schedule.Name,
		"successLimit", successLimit,
		"failedLimit", failedLimit)

	// TODO: List BackupJobs owned by this schedule
	// TODO: Categorize by success/failure
	// TODO: Delete jobs exceeding limits (oldest first)

	return nil
}

// UpdateStatus updates the BackupSchedule status
func (c *BackupScheduleController) UpdateStatus(ctx context.Context, schedule *transiva.BackupSchedule) error {
	// TODO: Update status using Kubernetes client
	c.logger.Debug("updating BackupSchedule status", "name", schedule.Name)
	return nil
}

// UpdateActiveJobs updates the list of active jobs in the schedule status
func (c *BackupScheduleController) UpdateActiveJobs(ctx context.Context, schedule *transiva.BackupSchedule) error {
	c.logger.Debug("updating active jobs", "name", schedule.Name)

	// TODO: Query all BackupJobs owned by this schedule
	// TODO: Filter to only Running/Pending jobs
	// TODO: Update schedule.Status.Active

	// Remove completed jobs from active list
	// TODO: Check if job is still running; for now, keep all
	activeJobs := []transiva.ActiveJob{}
	activeJobs = append(activeJobs, schedule.Status.Active...)

	schedule.Status.Active = activeJobs

	return nil
}
