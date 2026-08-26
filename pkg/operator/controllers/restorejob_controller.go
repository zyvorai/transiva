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

// RestoreJobController manages RestoreJob resources
type RestoreJobController struct {
	clientset    kubernetes.Interface
	scheme       *runtime.Scheme
	recorder     record.EventRecorder
	logger       logger.Logger
	jobManager   *jobs.Manager
	providerReg  *providers.Registry
	syncInterval time.Duration
}

// NewRestoreJobController creates a new RestoreJob controller
func NewRestoreJobController(
	clientset kubernetes.Interface,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	logger logger.Logger,
	jobManager *jobs.Manager,
	providerReg *providers.Registry,
) *RestoreJobController {
	return &RestoreJobController{
		clientset:    clientset,
		scheme:       scheme,
		recorder:     recorder,
		logger:       logger,
		jobManager:   jobManager,
		providerReg:  providerReg,
		syncInterval: 30 * time.Second,
	}
}

// Reconcile handles RestoreJob reconciliation
func (c *RestoreJobController) Reconcile(ctx context.Context, restoreJob *transiva.RestoreJob) error {
	c.logger.Info("reconciling RestoreJob",
		"name", restoreJob.Name,
		"namespace", restoreJob.Namespace,
		"phase", restoreJob.Status.Phase)

	// Handle based on current phase
	switch restoreJob.Status.Phase {
	case "":
		// New job - initialize
		return c.initializeRestoreJob(ctx, restoreJob)
	case transiva.RestorePhasePending:
		// Ready to start restore
		return c.startRestore(ctx, restoreJob)
	case transiva.RestorePhaseRunning:
		// Monitor progress
		return c.monitorRunningJob(ctx, restoreJob)
	case transiva.RestorePhaseCompleted, transiva.RestorePhaseFailed, transiva.RestorePhaseCancelled:
		// Final states - handle post-restore actions
		if restoreJob.Status.Phase == transiva.RestorePhaseCompleted {
			return c.handleCompletedRestore(ctx, restoreJob)
		}
		return nil
	default:
		c.logger.Warn("unknown restore job phase", "phase", restoreJob.Status.Phase)
		return nil
	}
}

// initializeRestoreJob sets up a new restore job
func (c *RestoreJobController) initializeRestoreJob(ctx context.Context, restoreJob *transiva.RestoreJob) error {
	c.logger.Info("initializing new RestoreJob", "name", restoreJob.Name)

	// Validate restore source
	if err := c.validateRestoreSource(ctx, restoreJob); err != nil {
		c.logger.Error("invalid restore source", "error", err)
		restoreJob.Status.Phase = transiva.RestorePhaseFailed
		restoreJob.Status.Error = err.Error()
		c.recorder.Event(restoreJob, "Warning", "ValidationFailed", err.Error())
		return err
	}

	// Set initial status
	now := metav1.Now()
	restoreJob.Status.Phase = transiva.RestorePhasePending
	restoreJob.Status.StartTime = &now
	restoreJob.Status.Progress = &transiva.RestoreProgress{
		Percentage:   0,
		CurrentPhase: "Initializing",
	}

	// Add condition
	restoreJob.Status.Conditions = append(restoreJob.Status.Conditions, transiva.BackupCondition{
		Type:               "Initialized",
		Status:             "True",
		LastTransitionTime: now,
		Reason:             "RestoreJobCreated",
		Message:            "RestoreJob has been created and initialized",
	})

	c.recorder.Event(restoreJob, "Normal", "Initialized", "RestoreJob initialized")

	return nil
}

// validateRestoreSource validates the restore source configuration
func (c *RestoreJobController) validateRestoreSource(ctx context.Context, restoreJob *transiva.RestoreJob) error {
	source := restoreJob.Spec.Source

	// Validate based on source type
	switch source.Type {
	case "s3", "azure-blob", "gcs":
		// Cloud storage - validate bucket
		if source.Bucket == "" {
			return fmt.Errorf("bucket is required for %s source", source.Type)
		}
		if source.Path == "" {
			return fmt.Errorf("path is required for %s source", source.Type)
		}

	case "local", "nfs":
		// Local storage - validate path
		if source.Path == "" {
			return fmt.Errorf("path is required for %s source", source.Type)
		}

	case "backup-ref":
		// BackupJob reference - validate reference
		if source.BackupJobRef == nil {
			return fmt.Errorf("backupJobRef is required for backup-ref source type")
		}
		if source.BackupJobRef.Name == "" {
			return fmt.Errorf("backupJobRef.name is required")
		}

		// TODO: Verify BackupJob exists and is completed
		// namespace := source.BackupJobRef.Namespace
		// if namespace == "" {
		// 	namespace = restoreJob.Namespace
		// }

	default:
		return fmt.Errorf("unknown source type: %s", source.Type)
	}

	return nil
}

// startRestore initiates the restore operation
func (c *RestoreJobController) startRestore(ctx context.Context, restoreJob *transiva.RestoreJob) error {
	c.logger.Info("starting restore", "name", restoreJob.Name)

	// Resolve source path
	sourcePath, err := c.resolveSourcePath(ctx, restoreJob)
	if err != nil {
		c.logger.Error("failed to resolve source path", "error", err)
		restoreJob.Status.Phase = transiva.RestorePhaseFailed
		restoreJob.Status.Error = err.Error()
		c.recorder.Event(restoreJob, "Warning", "ResolveFailed", err.Error())
		return err
	}

	// Build restore job definition
	jobDef, err := c.buildRestoreJobDefinition(restoreJob, sourcePath)
	if err != nil {
		c.logger.Error("failed to build restore job definition", "error", err)
		restoreJob.Status.Phase = transiva.RestorePhaseFailed
		restoreJob.Status.Error = err.Error()
		c.recorder.Event(restoreJob, "Warning", "BuildFailed", err.Error())
		return err
	}

	// Submit restore job to job manager
	// TODO: Add restore operation to job manager
	// For now, simulate
	jobID := fmt.Sprintf("restore-%s-%d", restoreJob.Name, time.Now().Unix())

	// Update status
	restoreJob.Status.Phase = transiva.RestorePhaseRunning
	restoreJob.Status.Progress.CurrentPhase = "Restore in progress"

	now := metav1.Now()
	restoreJob.Status.Conditions = append(restoreJob.Status.Conditions, transiva.BackupCondition{
		Type:               "Running",
		Status:             "True",
		LastTransitionTime: now,
		Reason:             "RestoreStarted",
		Message:            fmt.Sprintf("Restore job started with ID: %s", jobID),
	})

	c.recorder.Event(restoreJob, "Normal", "Started", fmt.Sprintf("Restore started: %s", jobID))

	c.logger.Info("restore job submitted",
		"name", restoreJob.Name,
		"jobID", jobID,
		"source", sourcePath,
		"destination", jobDef)

	return nil
}

// resolveSourcePath resolves the source path for the restore
func (c *RestoreJobController) resolveSourcePath(ctx context.Context, restoreJob *transiva.RestoreJob) (string, error) {
	source := restoreJob.Spec.Source

	switch source.Type {
	case "s3":
		return fmt.Sprintf("s3://%s/%s", source.Bucket, source.Path), nil

	case "azure-blob":
		return fmt.Sprintf("azure://%s/%s", source.Bucket, source.Path), nil

	case "gcs":
		return fmt.Sprintf("gs://%s/%s", source.Bucket, source.Path), nil

	case "local", "nfs":
		return source.Path, nil

	case "backup-ref":
		// TODO: Query BackupJob and get its output path
		namespace := source.BackupJobRef.Namespace
		if namespace == "" {
			namespace = restoreJob.Namespace
		}

		// For now, return a placeholder
		return fmt.Sprintf("/backups/%s/%s", namespace, source.BackupJobRef.Name), nil

	default:
		return "", fmt.Errorf("unknown source type: %s", source.Type)
	}
}

// buildRestoreJobDefinition builds a job definition for restore
func (c *RestoreJobController) buildRestoreJobDefinition(restoreJob *transiva.RestoreJob, sourcePath string) (*models.JobDefinition, error) {
	dest := restoreJob.Spec.Destination

	// Build VM name
	vmName := dest.VMName
	if restoreJob.Spec.Options != nil && restoreJob.Spec.Options.RenameVM != "" {
		vmName = restoreJob.Spec.Options.RenameVM
	}

	jobDef := &models.JobDefinition{
		VMPath:    sourcePath,
		OutputDir: "/tmp/restores", // Temporary
		Format:    "ovf",
		Metadata:  make(map[string]interface{}),
	}

	// Add metadata
	jobDef.Metadata["restoreJob"] = fmt.Sprintf("%s/%s", restoreJob.Namespace, restoreJob.Name)
	jobDef.Metadata["provider"] = dest.Provider
	jobDef.Metadata["vmName"] = vmName
	jobDef.Metadata["operation"] = "restore"

	// Add destination details
	if dest.Namespace != "" {
		jobDef.Metadata["namespace"] = dest.Namespace
	}
	if dest.Datacenter != "" {
		jobDef.Metadata["datacenter"] = dest.Datacenter
	}
	if dest.ResourcePool != "" {
		jobDef.Metadata["resourcePool"] = dest.ResourcePool
	}

	// Add options
	if restoreJob.Spec.Options != nil {
		opts := restoreJob.Spec.Options

		if opts.PowerOnAfterRestore {
			jobDef.Metadata["powerOn"] = true
		}
		if opts.Overwrite {
			jobDef.Metadata["overwrite"] = true
		}
		if opts.ConvertFormat != "" {
			jobDef.Format = opts.ConvertFormat
		}

		// Add customization
		if opts.Customization != nil {
			custom := opts.Customization
			if custom.Memory != "" {
				jobDef.Metadata["customMemory"] = custom.Memory
			}
			if custom.CPU > 0 {
				jobDef.Metadata["customCPU"] = custom.CPU
			}
		}
	}

	return jobDef, nil
}

// monitorRunningJob monitors the progress of a running restore
func (c *RestoreJobController) monitorRunningJob(ctx context.Context, restoreJob *transiva.RestoreJob) error {
	c.logger.Debug("monitoring running restore", "name", restoreJob.Name)

	// TODO: Query job manager for job status
	// TODO: Update progress from job manager
	// TODO: If job completed, update status accordingly

	return nil
}

// handleCompletedRestore handles post-restore actions
func (c *RestoreJobController) handleCompletedRestore(ctx context.Context, restoreJob *transiva.RestoreJob) error {
	c.logger.Info("handling completed restore", "name", restoreJob.Name)

	// Check if we need to power on the VM
	if restoreJob.Spec.Options != nil && restoreJob.Spec.Options.PowerOnAfterRestore {
		c.logger.Info("powering on restored VM", "name", restoreJob.Name, "vm", restoreJob.Status.RestoredVMName)

		// TODO: Power on VM using provider
		// provider := c.providerReg.GetProvider(restoreJob.Spec.Destination.Provider)
		// provider.StartVM(ctx, restoreJob.Status.RestoredVMID)

		c.recorder.Event(restoreJob, "Normal", "PoweredOn", fmt.Sprintf("VM powered on: %s", restoreJob.Status.RestoredVMName))
	}

	return nil
}

// UpdateStatus updates the RestoreJob status
func (c *RestoreJobController) UpdateStatus(ctx context.Context, restoreJob *transiva.RestoreJob) error {
	// TODO: Update status using Kubernetes client
	c.logger.Debug("updating RestoreJob status", "name", restoreJob.Name, "phase", restoreJob.Status.Phase)
	return nil
}
