// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	transiva "github.com/zyvorai/transiva/pkg/apis/transiva/v1alpha1"
)

const (
	backupScheduleFinalizer = "transiva.zyvor.ai/backupschedule-finalizer"
)

// VMBackupScheduleReconciler reconciles a VMBackupSchedule object
type VMBackupScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	cron   *cron.Cron
}

// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmbackupschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmbackupschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmbackupschedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmsnapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=virtualmachines,verbs=get;list;watch

// Reconcile handles VMBackupSchedule reconciliation
func (r *VMBackupScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the VMBackupSchedule instance
	schedule := &transiva.VMBackupSchedule{}
	if err := r.Get(ctx, req.NamespacedName, schedule); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !schedule.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, schedule)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(schedule, backupScheduleFinalizer) {
		controllerutil.AddFinalizer(schedule, backupScheduleFinalizer)
		if err := r.Update(ctx, schedule); err != nil {
			return ctrl.Result{}, err
		}
	}

	// If schedule is paused, skip processing
	if schedule.Spec.Paused {
		logger.Info("Backup schedule is paused", "schedule", schedule.Name)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Find matching VMs
	matchedVMs, err := r.findMatchingVMs(ctx, schedule)
	if err != nil {
		logger.Error(err, "Failed to find matching VMs")
		return ctrl.Result{}, err
	}

	// Update status with matched VMs
	schedule.Status.MatchedVMs = make([]string, len(matchedVMs))
	for i, vm := range matchedVMs {
		schedule.Status.MatchedVMs[i] = vm.Name
	}

	// Parse cron schedule
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronSchedule, err := parser.Parse(schedule.Spec.Schedule)
	if err != nil {
		logger.Error(err, "Failed to parse cron schedule", "schedule", schedule.Spec.Schedule)
		return ctrl.Result{}, err
	}

	// Calculate next backup time
	now := time.Now()
	nextBackup := cronSchedule.Next(now)
	schedule.Status.NextBackupTime = &metav1.Time{Time: nextBackup}

	// Check if it's time to create a backup
	if schedule.Status.LastBackupTime == nil || now.After(nextBackup) {
		logger.Info("Creating scheduled backups", "schedule", schedule.Name, "vmCount", len(matchedVMs))

		for _, vm := range matchedVMs {
			if err := r.createBackupSnapshot(ctx, schedule, &vm); err != nil {
				logger.Error(err, "Failed to create backup snapshot", "vm", vm.Name)
				schedule.Status.FailedBackups++
				schedule.Status.LastBackupStatus = "Failed"
				schedule.Status.LastBackupMessage = err.Error()
			} else {
				schedule.Status.TotalBackups++
				schedule.Status.LastBackupStatus = "Success"
			}
		}

		schedule.Status.LastBackupTime = &metav1.Time{Time: now}
	}

	// Clean up old backups based on retention policy
	if err := r.cleanupOldBackups(ctx, schedule); err != nil {
		logger.Error(err, "Failed to cleanup old backups")
	}

	// Update status
	if err := r.Status().Update(ctx, schedule); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue at next backup time
	requeueAfter := time.Until(nextBackup)
	if requeueAfter < 0 {
		requeueAfter = 1 * time.Minute
	}

	logger.Info("Next backup scheduled", "schedule", schedule.Name, "nextBackup", nextBackup, "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// findMatchingVMs finds VMs that match the selector
func (r *VMBackupScheduleReconciler) findMatchingVMs(ctx context.Context, schedule *transiva.VMBackupSchedule) ([]transiva.VirtualMachine, error) {
	vmList := &transiva.VirtualMachineList{}

	// Build list options based on selector
	opts := []client.ListOption{}

	// Match namespaces
	if len(schedule.Spec.VMSelector.MatchNamespaces) > 0 {
		// For multiple namespaces, we need to list from each namespace
		// For simplicity, we'll list from the schedule's namespace
		opts = append(opts, client.InNamespace(schedule.Namespace))
	} else {
		opts = append(opts, client.InNamespace(schedule.Namespace))
	}

	// Match labels
	if len(schedule.Spec.VMSelector.MatchLabels) > 0 {
		labelSelector := labels.SelectorFromSet(schedule.Spec.VMSelector.MatchLabels)
		opts = append(opts, client.MatchingLabelsSelector{Selector: labelSelector})
	}

	if err := r.List(ctx, vmList, opts...); err != nil {
		return nil, err
	}

	// Filter by name matching and exclusions
	var matchedVMs []transiva.VirtualMachine
	for _, vm := range vmList.Items {
		// Check if in matchNames (if specified)
		if len(schedule.Spec.VMSelector.MatchNames) > 0 {
			found := false
			for _, name := range schedule.Spec.VMSelector.MatchNames {
				if vm.Name == name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check if in excludeNames
		excluded := false
		for _, name := range schedule.Spec.VMSelector.ExcludeNames {
			if vm.Name == name {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		matchedVMs = append(matchedVMs, vm)
	}

	return matchedVMs, nil
}

// createBackupSnapshot creates a snapshot for a VM
func (r *VMBackupScheduleReconciler) createBackupSnapshot(ctx context.Context, schedule *transiva.VMBackupSchedule, vm *transiva.VirtualMachine) error {
	snapshotName := fmt.Sprintf("%s-%s-%d", schedule.Name, vm.Name, time.Now().Unix())

	snapshot := &transiva.VMSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: vm.Namespace,
			Labels: map[string]string{
				"backup-schedule": schedule.Name,
				"vm":              vm.Name,
				"backup-type":     "scheduled",
			},
		},
		Spec: transiva.VMSnapshotSpec{
			VMRef: transiva.VMReference{
				Name:      vm.Name,
				Namespace: vm.Namespace,
			},
			IncludeMemory: schedule.Spec.SnapshotTemplate.IncludeMemory,
			Quiesce:       schedule.Spec.SnapshotTemplate.Quiesce,
			Description:   fmt.Sprintf("Automated backup from schedule: %s", schedule.Name),
			Destination:   schedule.Spec.SnapshotTemplate.Destination,
		},
	}

	// Add template labels and annotations
	if snapshot.Labels == nil {
		snapshot.Labels = make(map[string]string)
	}
	for k, v := range schedule.Spec.SnapshotTemplate.Labels {
		snapshot.Labels[k] = v
	}

	if schedule.Spec.SnapshotTemplate.Annotations != nil {
		snapshot.Annotations = schedule.Spec.SnapshotTemplate.Annotations
	}

	if err := r.Create(ctx, snapshot); err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	schedule.Status.LastBackupSnapshot = snapshotName
	schedule.Status.ActiveBackups++

	return nil
}

// cleanupOldBackups removes old backups based on retention policy
func (r *VMBackupScheduleReconciler) cleanupOldBackups(ctx context.Context, schedule *transiva.VMBackupSchedule) error {
	logger := log.FromContext(ctx)

	// List all snapshots created by this schedule
	snapshotList := &transiva.VMSnapshotList{}
	opts := []client.ListOption{
		client.InNamespace(schedule.Namespace),
		client.MatchingLabels{
			"backup-schedule": schedule.Name,
		},
	}

	if err := r.List(ctx, snapshotList, opts...); err != nil {
		return err
	}

	// Sort snapshots by creation time (newest first)
	snapshots := snapshotList.Items
	// Simple bubble sort by creation timestamp
	for i := 0; i < len(snapshots); i++ {
		for j := i + 1; j < len(snapshots); j++ {
			if snapshots[i].CreationTimestamp.Before(&snapshots[j].CreationTimestamp) {
				snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
			}
		}
	}

	// Apply retention policy
	policy := schedule.Spec.RetentionPolicy
	toDelete := []transiva.VMSnapshot{}

	// Keep last N backups
	if policy.KeepLast > 0 {
		if len(snapshots) > int(policy.KeepLast) {
			toDelete = append(toDelete, snapshots[policy.KeepLast:]...)
		}
	}

	// Delete expired backups
	for _, snapshot := range toDelete {
		logger.Info("Deleting expired backup", "snapshot", snapshot.Name)
		if err := r.Delete(ctx, &snapshot); err != nil {
			logger.Error(err, "Failed to delete snapshot", "snapshot", snapshot.Name)
		} else {
			schedule.Status.ActiveBackups--
		}
	}

	return nil
}

// handleDeletion handles schedule deletion
func (r *VMBackupScheduleReconciler) handleDeletion(ctx context.Context, schedule *transiva.VMBackupSchedule) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(schedule, backupScheduleFinalizer) {
		// Cleanup if needed
		controllerutil.RemoveFinalizer(schedule, backupScheduleFinalizer)
		if err := r.Update(ctx, schedule); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *VMBackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize cron
	r.cron = cron.New()

	return ctrl.NewControllerManagedBy(mgr).
		For(&transiva.VMBackupSchedule{}).
		Complete(r)
}
