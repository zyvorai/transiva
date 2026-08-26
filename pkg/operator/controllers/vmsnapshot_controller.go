// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	transiva "github.com/zyvorai/transiva/pkg/apis/transiva/v1alpha1"
)

const (
	vmSnapshotFinalizer = "transiva.zyvor.ai/vmsnapshot-finalizer"
)

// VMSnapshotReconciler reconciles a VMSnapshot object
type VMSnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmsnapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmsnapshots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmsnapshots/finalizers,verbs=update

// Reconcile handles VMSnapshot reconciliation
func (r *VMSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the VMSnapshot instance
	snapshot := &transiva.VMSnapshot{}
	if err := r.Get(ctx, req.NamespacedName, snapshot); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !snapshot.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, snapshot)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(snapshot, vmSnapshotFinalizer) {
		controllerutil.AddFinalizer(snapshot, vmSnapshotFinalizer)
		if err := r.Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize status if needed
	if snapshot.Status.Phase == "" {
		snapshot.Status.Phase = transiva.VMSnapPhasePending
		snapshot.Status.CreationTime = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check retention expiration
	if r.isExpired(snapshot) {
		return r.handleExpiration(ctx, snapshot)
	}

	// Reconcile based on current phase
	switch snapshot.Status.Phase {
	case transiva.VMSnapPhasePending:
		return r.reconcilePending(ctx, snapshot)
	case transiva.VMSnapPhaseCreating:
		return r.reconcileCreating(ctx, snapshot)
	case transiva.VMSnapPhaseReady:
		return r.reconcileReady(ctx, snapshot)
	case transiva.VMSnapPhaseFailed:
		// Terminal state
		return ctrl.Result{}, nil
	case transiva.VMSnapPhaseExpired:
		// Auto-delete if enabled
		if snapshot.Spec.Retention != nil && snapshot.Spec.Retention.AutoDelete {
			if err := r.Delete(ctx, snapshot); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	default:
		logger.Info("Unknown VMSnapshot phase", "phase", snapshot.Status.Phase)
		return ctrl.Result{}, nil
	}
}

// reconcilePending validates and starts snapshot creation
func (r *VMSnapshotReconciler) reconcilePending(ctx context.Context, snapshot *transiva.VMSnapshot) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the VM
	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, snapshot, vm); err != nil {
		snapshot.Status.Phase = transiva.VMSnapPhaseFailed
		r.updateCondition(snapshot, "VMFound", "False", "VMNotFound", fmt.Sprintf("VM not found: %v", err))
		if err := r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Capture VM state
	snapshot.Status.VMState = &transiva.VMStateSnapshot{
		CPUs:      vm.Spec.CPUs,
		Memory:    vm.Spec.Memory,
		DiskCount: clampInt32(len(vm.Spec.Disks)),
		Running:   vm.Spec.Running,
	}

	// Transition to Creating
	snapshot.Status.Phase = transiva.VMSnapPhaseCreating
	r.updateCondition(snapshot, "Creating", "True", "SnapshotCreating", "Snapshot is being created")
	if err := r.Status().Update(ctx, snapshot); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Started snapshot creation", "snapshot", snapshot.Name, "vm", vm.Name)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// reconcileCreating handles snapshot creation
func (r *VMSnapshotReconciler) reconcileCreating(ctx context.Context, snapshot *transiva.VMSnapshot) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the VM
	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, snapshot, vm); err != nil {
		snapshot.Status.Phase = transiva.VMSnapPhaseFailed
		r.updateCondition(snapshot, "VMFound", "False", "VMNotFound", fmt.Sprintf("VM not found: %v", err))
		if err := r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// TODO: Implement actual snapshot creation logic
	// For now, simulate snapshot creation
	if err := r.createSnapshot(ctx, snapshot, vm); err != nil {
		logger.Error(err, "Failed to create snapshot")
		snapshot.Status.Phase = transiva.VMSnapPhaseFailed
		r.updateCondition(snapshot, "Ready", "False", "SnapshotFailed", fmt.Sprintf("Snapshot creation failed: %v", err))
		if err := r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Snapshot created successfully
	snapshot.Status.Phase = transiva.VMSnapPhaseReady
	snapshot.Status.ReadyToRestore = true

	// Calculate snapshot size
	snapshot.Status.Size = r.calculateSnapshotSize(vm, snapshot.Spec.IncludeMemory)
	snapshot.Status.SizeBytes = r.parseSizeToBytes(snapshot.Status.Size)

	// Set backing files
	snapshot.Status.BackingFiles = r.generateBackingFiles(snapshot, vm)

	r.updateCondition(snapshot, "Ready", "True", "SnapshotReady", "Snapshot is ready for restore")
	if err := r.Status().Update(ctx, snapshot); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Snapshot created successfully", "snapshot", snapshot.Name, "size", snapshot.Status.Size)
	return ctrl.Result{}, nil
}

// reconcileReady handles ready snapshots
func (r *VMSnapshotReconciler) reconcileReady(ctx context.Context, snapshot *transiva.VMSnapshot) (ctrl.Result, error) {
	// Check if snapshot has expired
	if r.isExpired(snapshot) {
		return r.handleExpiration(ctx, snapshot)
	}

	// Recheck expiration periodically
	return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil
}

// createSnapshot performs the actual snapshot creation
func (r *VMSnapshotReconciler) createSnapshot(ctx context.Context, snapshot *transiva.VMSnapshot, vm *transiva.VirtualMachine) error {
	// TODO: Implement actual snapshot logic
	// This would involve:
	// 1. Quiesce filesystem if requested and guest agent is available
	// 2. Create disk snapshots for each VM disk
	// 3. Capture memory state if requested
	// 4. Store snapshot data to destination

	// For now, simulate a delay
	time.Sleep(2 * time.Second)

	return nil
}

// calculateSnapshotSize calculates the snapshot size
func (r *VMSnapshotReconciler) calculateSnapshotSize(vm *transiva.VirtualMachine, includeMemory bool) string {
	// TODO: Calculate actual snapshot size
	// For now, return a mock value
	if includeMemory {
		return "10.5Gi"
	}
	return "8.2Gi"
}

// parseSizeToBytes converts size string to bytes
func (r *VMSnapshotReconciler) parseSizeToBytes(size string) int64 {
	// TODO: Implement proper parsing
	// For now, return a mock value
	return 10737418240 // 10Gi in bytes
}

// generateBackingFiles generates the list of backing files
func (r *VMSnapshotReconciler) generateBackingFiles(snapshot *transiva.VMSnapshot, vm *transiva.VirtualMachine) []string {
	files := []string{}

	// Add disk snapshot files
	for i, disk := range vm.Spec.Disks {
		file := fmt.Sprintf("/var/lib/transiva/snapshots/%s-%s-disk%d.qcow2",
			snapshot.Name, disk.Name, i)
		files = append(files, file)
	}

	// Add memory file if included
	if snapshot.Spec.IncludeMemory {
		file := fmt.Sprintf("/var/lib/transiva/snapshots/%s-memory.mem", snapshot.Name)
		files = append(files, file)
	}

	return files
}

// isExpired checks if snapshot has expired
func (r *VMSnapshotReconciler) isExpired(snapshot *transiva.VMSnapshot) bool {
	if snapshot.Spec.Retention == nil {
		return false
	}

	// Check explicit expiration time
	if snapshot.Spec.Retention.ExpiresAt != nil {
		return time.Now().After(snapshot.Spec.Retention.ExpiresAt.Time)
	}

	// Check keep days
	if snapshot.Spec.Retention.KeepDays > 0 && snapshot.Status.CreationTime != nil {
		expirationTime := snapshot.Status.CreationTime.Add(time.Duration(snapshot.Spec.Retention.KeepDays) * 24 * time.Hour)
		return time.Now().After(expirationTime)
	}

	return false
}

// handleExpiration handles expired snapshots
func (r *VMSnapshotReconciler) handleExpiration(ctx context.Context, snapshot *transiva.VMSnapshot) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	snapshot.Status.Phase = transiva.VMSnapPhaseExpired
	r.updateCondition(snapshot, "Expired", "True", "RetentionExpired", "Snapshot has expired")
	if err := r.Status().Update(ctx, snapshot); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Snapshot expired", "snapshot", snapshot.Name)

	// Auto-delete if enabled
	if snapshot.Spec.Retention != nil && snapshot.Spec.Retention.AutoDelete {
		if err := r.Delete(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Snapshot auto-deleted", "snapshot", snapshot.Name)
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles snapshot deletion
func (r *VMSnapshotReconciler) handleDeletion(ctx context.Context, snapshot *transiva.VMSnapshot) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(snapshot, vmSnapshotFinalizer) {
		// Cleanup snapshot files
		// TODO: Implement actual file deletion
		logger.Info("Cleaning up snapshot files", "snapshot", snapshot.Name)

		// Remove finalizer
		controllerutil.RemoveFinalizer(snapshot, vmSnapshotFinalizer)
		if err := r.Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("VMSnapshot deleted", "snapshot", snapshot.Name)
	}

	return ctrl.Result{}, nil
}

// Helper functions

func (r *VMSnapshotReconciler) getVM(ctx context.Context, snapshot *transiva.VMSnapshot, vm *transiva.VirtualMachine) error {
	namespace := snapshot.Spec.VMRef.Namespace
	if namespace == "" {
		namespace = snapshot.Namespace
	}

	return r.Get(ctx, types.NamespacedName{
		Name:      snapshot.Spec.VMRef.Name,
		Namespace: namespace,
	}, vm)
}

func (r *VMSnapshotReconciler) updateCondition(snapshot *transiva.VMSnapshot, condType, status, reason, message string) {
	now := metav1.Now()
	condition := transiva.VMCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Update or append condition
	found := false
	for i, c := range snapshot.Status.Conditions {
		if c.Type == condType {
			snapshot.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		snapshot.Status.Conditions = append(snapshot.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *VMSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&transiva.VMSnapshot{}).
		Complete(r)
}
