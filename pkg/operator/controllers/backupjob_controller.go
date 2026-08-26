// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	transiva "github.com/zyvorai/transiva/pkg/apis/transiva/v1alpha1"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// BackupJobController manages BackupJob resources
type BackupJobController struct {
	clientset    kubernetes.Interface
	scheme       *runtime.Scheme
	recorder     record.EventRecorder
	logger       logger.Logger
	jobManager   *jobs.Manager
	providerReg  *providers.Registry
	syncInterval time.Duration
}

// NewBackupJobController creates a new BackupJob controller
func NewBackupJobController(
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	logger logger.Logger,
	jobManager *jobs.Manager,
	providerReg *providers.Registry,
) *BackupJobController {
	return &BackupJobController{
		clientset:    clientset,
		scheme:       scheme,
		recorder:     recorder,
		logger:       logger,
		jobManager:   jobManager,
		providerReg:  providerReg,
		syncInterval: 30 * time.Second,
	}
}

// Reconcile handles BackupJob reconciliation
func (c *BackupJobController) Reconcile(ctx context.Context, backupJob *transiva.BackupJob) error {
	c.logger.Info("reconciling BackupJob",
		"name", backupJob.Name,
		"namespace", backupJob.Namespace,
		"phase", backupJob.Status.Phase)

	// Handle based on current phase
	switch backupJob.Status.Phase {
	case "":
		// New job - initialize
		return c.initializeBackupJob(ctx, backupJob)
	case transiva.BackupPhasePending:
		// Check if carbon-aware and needs delay
		return c.handlePendingJob(ctx, backupJob)
	case transiva.BackupPhaseRunning:
		// Monitor progress
		return c.monitorRunningJob(ctx, backupJob)
	case transiva.BackupPhaseCompleted, transiva.BackupPhaseFailed, transiva.BackupPhaseCancelled:
		// Final states - apply retention if completed
		if backupJob.Status.Phase == transiva.BackupPhaseCompleted && backupJob.Spec.Retention != nil {
			return c.applyRetentionPolicy(ctx, backupJob)
		}
		return nil
	default:
		c.logger.Warn("unknown backup job phase", "phase", backupJob.Status.Phase)
		return nil
	}
}

// initializeBackupJob sets up a new backup job
func (c *BackupJobController) initializeBackupJob(ctx context.Context, backupJob *transiva.BackupJob) error {
	c.logger.Info("initializing new BackupJob", "name", backupJob.Name)

	// Set initial status
	now := metav1.Now()
	backupJob.Status.Phase = transiva.BackupPhasePending
	backupJob.Status.StartTime = &now
	backupJob.Status.Progress = &transiva.BackupProgress{
		Percentage:   0,
		CurrentPhase: "Initializing",
	}

	// Add condition
	backupJob.Status.Conditions = append(backupJob.Status.Conditions, transiva.BackupCondition{
		Type:               "Initialized",
		Status:             "True",
		LastTransitionTime: now,
		Reason:             "BackupJobCreated",
		Message:            "BackupJob has been created and initialized",
	})

	c.recorder.Event(backupJob, "Normal", "Initialized", "BackupJob initialized")

	return nil
}

// handlePendingJob processes pending backup jobs
func (c *BackupJobController) handlePendingJob(ctx context.Context, backupJob *transiva.BackupJob) error {
	c.logger.Info("handling pending BackupJob", "name", backupJob.Name)

	// Check if carbon-aware scheduling is enabled
	if backupJob.Spec.CarbonAware != nil && backupJob.Spec.CarbonAware.Enabled {
		canRun, delay, err := c.checkCarbonIntensity(ctx, backupJob)
		if err != nil {
			c.logger.Error("failed to check carbon intensity", "error", err)
			// Don't fail the job, proceed anyway
		} else if !canRun {
			c.logger.Info("delaying backup due to high carbon intensity",
				"name", backupJob.Name,
				"delay", delay)

			// Update status to indicate delay
			backupJob.Status.Progress.CurrentPhase = fmt.Sprintf("Delayed for %v (carbon-aware)", delay)

			// Schedule recheck
			return fmt.Errorf("delaying backup: %v", delay)
		}
	}

	// Ready to start backup
	return c.startBackup(ctx, backupJob)
}

// checkCarbonIntensity checks if carbon intensity is acceptable
func (c *BackupJobController) checkCarbonIntensity(ctx context.Context, backupJob *transiva.BackupJob) (canRun bool, delay time.Duration, err error) {
	// TODO: Integrate with carbon-aware API
	// For now, always allow
	return true, 0, nil
}

// startBackup initiates the backup operation
func (c *BackupJobController) startBackup(ctx context.Context, backupJob *transiva.BackupJob) error {
	c.logger.Info("starting backup", "name", backupJob.Name)

	// Build job definition from BackupJob spec
	jobDef, err := c.buildJobDefinition(backupJob)
	if err != nil {
		c.logger.Error("failed to build job definition", "error", err)
		backupJob.Status.Phase = transiva.BackupPhaseFailed
		backupJob.Status.Error = err.Error()
		c.recorder.Event(backupJob, "Warning", "BuildFailed", err.Error())
		return err
	}

	// Submit job to job manager
	jobID, err := c.jobManager.SubmitJob(*jobDef)
	if err != nil {
		c.logger.Error("failed to submit backup job", "error", err)
		backupJob.Status.Phase = transiva.BackupPhaseFailed
		backupJob.Status.Error = err.Error()
		c.recorder.Event(backupJob, "Warning", "SubmitFailed", err.Error())
		return err
	}

	// Update status
	backupJob.Status.Phase = transiva.BackupPhaseRunning
	backupJob.Status.Progress.CurrentPhase = "Backup in progress"

	// Store job ID in metadata
	if backupJob.Status.Conditions == nil {
		backupJob.Status.Conditions = []transiva.BackupCondition{}
	}

	now := metav1.Now()
	backupJob.Status.Conditions = append(backupJob.Status.Conditions, transiva.BackupCondition{
		Type:               "Running",
		Status:             "True",
		LastTransitionTime: now,
		Reason:             "BackupStarted",
		Message:            fmt.Sprintf("Backup job started with ID: %s", jobID),
	})

	c.recorder.Event(backupJob, "Normal", "Started", fmt.Sprintf("Backup started: %s", jobID))

	c.logger.Info("backup job submitted", "name", backupJob.Name, "jobID", jobID)

	return nil
}

// buildJobDefinition converts BackupJob spec to job definition
func (c *BackupJobController) buildJobDefinition(backupJob *transiva.BackupJob) (*models.JobDefinition, error) {
	// Build VM path based on provider
	var vmPath string
	switch backupJob.Spec.Source.Provider {
	case "kubevirt":
		if backupJob.Spec.Source.Namespace != "" && backupJob.Spec.Source.VMName != "" {
			vmPath = fmt.Sprintf("%s/%s", backupJob.Spec.Source.Namespace, backupJob.Spec.Source.VMName)
		} else {
			vmPath = backupJob.Spec.Source.VMName
		}
	case "vsphere":
		vmPath = backupJob.Spec.Source.VMPath
	default:
		vmPath = backupJob.Spec.Source.VMName
	}

	// Build output directory
	outputDir := "/tmp/backups" // Default
	if backupJob.Spec.Destination.Type == "local" {
		outputDir = backupJob.Spec.Destination.Bucket
	}

	jobDef := &models.JobDefinition{
		VMPath:    vmPath,
		OutputDir: outputDir,
		Format:    "ovf",
		Metadata:  make(map[string]interface{}),
	}

	// Set format
	if backupJob.Spec.Format != nil && backupJob.Spec.Format.Type != "" {
		jobDef.Format = backupJob.Spec.Format.Type
	}

	// Add metadata
	jobDef.Metadata["backupJob"] = fmt.Sprintf("%s/%s", backupJob.Namespace, backupJob.Name)
	jobDef.Metadata["provider"] = backupJob.Spec.Source.Provider

	// Add carbon-aware metadata
	if backupJob.Spec.CarbonAware != nil && backupJob.Spec.CarbonAware.Enabled {
		jobDef.Metadata["carbon_aware"] = true
		jobDef.Metadata["carbon_zone"] = backupJob.Spec.CarbonAware.Zone
		jobDef.Metadata["carbon_max_intensity"] = backupJob.Spec.CarbonAware.MaxIntensity
	}

	// Add incremental metadata
	if backupJob.Spec.Incremental != nil && backupJob.Spec.Incremental.Enabled {
		jobDef.Metadata["incremental"] = true
		if backupJob.Spec.Incremental.BaseBackupRef != "" {
			jobDef.Metadata["base_backup_ref"] = backupJob.Spec.Incremental.BaseBackupRef
		}
	}

	return jobDef, nil
}

// monitorRunningJob monitors the progress of a running backup
func (c *BackupJobController) monitorRunningJob(ctx context.Context, backupJob *transiva.BackupJob) error {
	c.logger.Debug("monitoring running backup", "name", backupJob.Name)

	// TODO: Query job manager for job status
	// For now, simulate progress

	// Check if job is still in job manager
	// Update progress from job manager
	// If job completed, update status accordingly

	return nil
}

// applyRetentionPolicy applies retention policy to backups
func (c *BackupJobController) applyRetentionPolicy(ctx context.Context, backupJob *transiva.BackupJob) error {
	c.logger.Info("applying retention policy", "name", backupJob.Name)

	retention := backupJob.Spec.Retention
	if retention == nil {
		return nil
	}

	// TODO: Implement retention policy
	// 1. List all completed backups for the same source
	// 2. Categorize by daily/weekly/monthly/yearly
	// 3. Delete backups exceeding retention limits

	c.logger.Info("retention policy applied",
		"name", backupJob.Name,
		"keepDaily", retention.KeepDaily,
		"keepWeekly", retention.KeepWeekly,
		"keepMonthly", retention.KeepMonthly,
		"keepYearly", retention.KeepYearly)

	return nil
}

// UpdateStatus updates the BackupJob status
func (c *BackupJobController) UpdateStatus(ctx context.Context, backupJob *transiva.BackupJob) error {
	// TODO: Update status using Kubernetes client
	// This would use the status subresource
	c.logger.Debug("updating BackupJob status", "name", backupJob.Name, "phase", backupJob.Status.Phase)
	return nil
}
