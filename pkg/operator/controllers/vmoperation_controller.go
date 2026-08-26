// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"encoding/json"
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
	vmOpFinalizer = "transiva.zyvor.ai/vmoperation-finalizer"
)

// VMOperationReconciler reconciles a VMOperation object
type VMOperationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmoperations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmoperations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmoperations/finalizers,verbs=update

// Reconcile handles VMOperation reconciliation
func (r *VMOperationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the VMOperation instance
	vmOp := &transiva.VMOperation{}
	if err := r.Get(ctx, req.NamespacedName, vmOp); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !vmOp.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, vmOp)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(vmOp, vmOpFinalizer) {
		controllerutil.AddFinalizer(vmOp, vmOpFinalizer)
		if err := r.Update(ctx, vmOp); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize status if needed
	if vmOp.Status.Phase == "" {
		vmOp.Status.Phase = transiva.VMOpPhasePending
		vmOp.Status.StartTime = &metav1.Time{Time: time.Now()}
		vmOp.Status.Progress = 0
		if err := r.Status().Update(ctx, vmOp); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile based on current phase
	switch vmOp.Status.Phase {
	case transiva.VMOpPhasePending:
		return r.reconcilePending(ctx, vmOp)
	case transiva.VMOpPhaseRunning:
		return r.reconcileRunning(ctx, vmOp)
	case transiva.VMOpPhaseSucceeded, transiva.VMOpPhaseFailed, transiva.VMOpPhaseCancelled:
		// Terminal states - do nothing
		return ctrl.Result{}, nil
	default:
		logger.Info("Unknown VMOperation phase", "phase", vmOp.Status.Phase)
		return ctrl.Result{}, nil
	}
}

// reconcilePending validates and starts the operation
func (r *VMOperationReconciler) reconcilePending(ctx context.Context, vmOp *transiva.VMOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate operation
	if err := r.validateOperation(ctx, vmOp); err != nil {
		vmOp.Status.Phase = transiva.VMOpPhaseFailed
		vmOp.Status.Message = fmt.Sprintf("Validation failed: %v", err)
		vmOp.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, vmOp); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Transition to Running
	vmOp.Status.Phase = transiva.VMOpPhaseRunning
	vmOp.Status.Progress = 10
	vmOp.Status.Message = fmt.Sprintf("Starting %s operation", vmOp.Spec.Operation)
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Started VMOperation", "operation", vmOp.Spec.Operation, "vm", vmOp.Spec.VMRef.Name)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// reconcileRunning executes the operation
func (r *VMOperationReconciler) reconcileRunning(ctx context.Context, vmOp *transiva.VMOperation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Execute operation based on type
	var err error
	switch vmOp.Spec.Operation {
	case transiva.VMOpStart:
		err = r.executeStart(ctx, vmOp)
	case transiva.VMOpStop:
		err = r.executeStop(ctx, vmOp)
	case transiva.VMOpRestart:
		err = r.executeRestart(ctx, vmOp)
	case transiva.VMOpClone:
		err = r.executeClone(ctx, vmOp)
	case transiva.VMOpMigrate:
		err = r.executeMigrate(ctx, vmOp)
	case transiva.VMOpResize:
		err = r.executeResize(ctx, vmOp)
	case transiva.VMOpSnapshot:
		err = r.executeSnapshot(ctx, vmOp)
	case transiva.VMOpDelete:
		err = r.executeDelete(ctx, vmOp)
	default:
		err = fmt.Errorf("unknown operation: %s", vmOp.Spec.Operation)
	}

	if err != nil {
		logger.Error(err, "Operation failed", "operation", vmOp.Spec.Operation)
		vmOp.Status.Phase = transiva.VMOpPhaseFailed
		vmOp.Status.Message = fmt.Sprintf("Operation failed: %v", err)
		vmOp.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, vmOp); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Operation succeeded
	vmOp.Status.Phase = transiva.VMOpPhaseSucceeded
	vmOp.Status.Progress = 100
	vmOp.Status.Message = fmt.Sprintf("%s operation completed successfully", vmOp.Spec.Operation)
	vmOp.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("VMOperation completed", "operation", vmOp.Spec.Operation, "vm", vmOp.Spec.VMRef.Name)
	return ctrl.Result{}, nil
}

// Operation executors

func (r *VMOperationReconciler) executeStart(ctx context.Context, vmOp *transiva.VMOperation) error {
	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, vmOp, vm); err != nil {
		return err
	}

	// Set running to true
	vm.Spec.Running = true
	if err := r.Update(ctx, vm); err != nil {
		return err
	}

	vmOp.Status.Progress = 100
	return nil
}

func (r *VMOperationReconciler) executeStop(ctx context.Context, vmOp *transiva.VMOperation) error {
	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, vmOp, vm); err != nil {
		return err
	}

	// Set running to false
	vm.Spec.Running = false
	if err := r.Update(ctx, vm); err != nil {
		return err
	}

	vmOp.Status.Progress = 100
	return nil
}

func (r *VMOperationReconciler) executeRestart(ctx context.Context, vmOp *transiva.VMOperation) error {
	// Stop then start
	vmOp.Status.Progress = 50
	vmOp.Status.Message = "Stopping VM..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	if err := r.executeStop(ctx, vmOp); err != nil {
		return err
	}

	// Wait a moment
	time.Sleep(5 * time.Second)

	vmOp.Status.Progress = 75
	vmOp.Status.Message = "Starting VM..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	return r.executeStart(ctx, vmOp)
}

func (r *VMOperationReconciler) executeClone(ctx context.Context, vmOp *transiva.VMOperation) error {
	logger := log.FromContext(ctx)

	if vmOp.Spec.CloneSpec == nil {
		return fmt.Errorf("clone spec is required for clone operation")
	}

	var sourceSpec transiva.VirtualMachineSpec
	var sourceLabels map[string]string
	var sourceName string

	// Check if cloning from snapshot or from VM
	if vmOp.Spec.CloneSpec.SnapshotRef != "" {
		// Clone from snapshot
		vmOp.Status.Message = "Loading snapshot configuration..."
		if err := r.Status().Update(ctx, vmOp); err != nil {
			return err
		}

		snapshot := &transiva.VMSnapshot{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      vmOp.Spec.CloneSpec.SnapshotRef,
			Namespace: vmOp.Namespace,
		}, snapshot); err != nil {
			return fmt.Errorf("failed to get snapshot: %w", err)
		}

		// Get source VM spec from snapshot
		if snapshot.Spec.VMRef.Name == "" {
			return fmt.Errorf("snapshot does not reference a VM")
		}

		sourceVM := &transiva.VirtualMachine{}
		if err := r.Get(ctx, client.ObjectKey{
			Name:      snapshot.Spec.VMRef.Name,
			Namespace: snapshot.Spec.VMRef.Namespace,
		}, sourceVM); err != nil {
			return fmt.Errorf("failed to get source VM from snapshot: %w", err)
		}

		sourceSpec = sourceVM.Spec
		sourceLabels = sourceVM.Labels
		sourceName = snapshot.Name
		logger.Info("Cloning from snapshot", "snapshot", snapshot.Name, "sourceVM", sourceVM.Name)
	} else {
		// Clone from existing VM
		sourceVM := &transiva.VirtualMachine{}
		if err := r.getVM(ctx, vmOp, sourceVM); err != nil {
			return err
		}
		sourceSpec = sourceVM.Spec
		sourceLabels = sourceVM.Labels
		sourceName = sourceVM.Name
		logger.Info("Cloning from VM", "source", sourceVM.Name)
	}

	vmOp.Status.Progress = 30
	vmOp.Status.Message = "Cloning VM configuration..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// Create new VM from source
	targetNamespace := vmOp.Spec.CloneSpec.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = vmOp.Namespace
	}

	targetVM := &transiva.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmOp.Spec.CloneSpec.TargetName,
			Namespace: targetNamespace,
			Labels:    sourceLabels,
		},
		Spec: sourceSpec,
	}

	// Set initial running state
	if vmOp.Spec.CloneSpec.SnapshotRef != "" {
		targetVM.Spec.Running = vmOp.Spec.CloneSpec.PowerOnAfter
	} else {
		targetVM.Spec.Running = vmOp.Spec.CloneSpec.StartAfterClone
	}

	vmOp.Status.Progress = 60
	vmOp.Status.Message = "Creating cloned VM..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// Create the cloned VM
	if err := r.Create(ctx, targetVM); err != nil {
		return fmt.Errorf("failed to create cloned VM: %w", err)
	}

	logger.Info("VM cloned successfully", "source", sourceName, "target", targetVM.Name)

	vmOp.Status.Progress = 100
	resultData, _ := json.Marshal(map[string]interface{}{
		"targetVM":        targetVM.Name,
		"targetNamespace": targetVM.Namespace,
		"sourceSnapshot":  vmOp.Spec.CloneSpec.SnapshotRef,
	})
	vmOp.Status.Result = string(resultData)

	return nil
}

func (r *VMOperationReconciler) executeMigrate(ctx context.Context, vmOp *transiva.VMOperation) error {
	logger := log.FromContext(ctx)

	if vmOp.Spec.MigrateSpec == nil {
		return fmt.Errorf("migrate spec is required for migrate operation")
	}

	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, vmOp, vm); err != nil {
		return err
	}

	vmOp.Status.Progress = 30
	vmOp.Status.Message = "Preparing for migration..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// Update VM status to Migrating
	vm.Status.Phase = transiva.VMPhaseMigrating
	if err := r.Status().Update(ctx, vm); err != nil {
		return err
	}

	vmOp.Status.Progress = 50
	vmOp.Status.Message = "Migrating VM..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// TODO: Implement actual migration logic
	// For now, simulate migration by updating node assignment
	vm.Status.NodeName = vmOp.Spec.MigrateSpec.TargetNode

	vmOp.Status.Progress = 90
	vmOp.Status.Message = "Finalizing migration..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// Update VM status back to Running
	vm.Status.Phase = transiva.VMPhaseRunning
	if err := r.Status().Update(ctx, vm); err != nil {
		return err
	}

	logger.Info("VM migrated successfully", "vm", vm.Name, "targetNode", vmOp.Spec.MigrateSpec.TargetNode)

	vmOp.Status.Progress = 100
	resultData, _ := json.Marshal(map[string]interface{}{
		"targetNode": vmOp.Spec.MigrateSpec.TargetNode,
		"live":       vmOp.Spec.MigrateSpec.Live,
	})
	vmOp.Status.Result = string(resultData)

	return nil
}

func (r *VMOperationReconciler) executeResize(ctx context.Context, vmOp *transiva.VMOperation) error {
	logger := log.FromContext(ctx)

	if vmOp.Spec.ResizeSpec == nil {
		return fmt.Errorf("resize spec is required for resize operation")
	}

	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, vmOp, vm); err != nil {
		return err
	}

	vmOp.Status.Progress = 50
	vmOp.Status.Message = "Resizing VM..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// Update VM spec
	oldCPUs := vm.Spec.CPUs
	oldMemory := vm.Spec.Memory

	if vmOp.Spec.ResizeSpec.CPUs > 0 {
		vm.Spec.CPUs = vmOp.Spec.ResizeSpec.CPUs
	}
	if vmOp.Spec.ResizeSpec.Memory != "" {
		vm.Spec.Memory = vmOp.Spec.ResizeSpec.Memory
	}

	if err := r.Update(ctx, vm); err != nil {
		return err
	}

	logger.Info("VM resized successfully", "vm", vm.Name,
		"oldCPUs", oldCPUs, "newCPUs", vm.Spec.CPUs,
		"oldMemory", oldMemory, "newMemory", vm.Spec.Memory)

	vmOp.Status.Progress = 100
	resultData, _ := json.Marshal(map[string]interface{}{
		"oldCPUs":   oldCPUs,
		"newCPUs":   vm.Spec.CPUs,
		"oldMemory": oldMemory,
		"newMemory": vm.Spec.Memory,
		"hotplug":   vmOp.Spec.ResizeSpec.Hotplug,
	})
	vmOp.Status.Result = string(resultData)

	return nil
}

func (r *VMOperationReconciler) executeSnapshot(ctx context.Context, vmOp *transiva.VMOperation) error {
	logger := log.FromContext(ctx)

	if vmOp.Spec.SnapshotSpec == nil {
		return fmt.Errorf("snapshot spec is required for snapshot operation")
	}

	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, vmOp, vm); err != nil {
		return err
	}

	vmOp.Status.Progress = 30
	vmOp.Status.Message = "Creating snapshot..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// Create VMSnapshot resource
	snapshot := &transiva.VMSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmOp.Spec.SnapshotSpec.Name,
			Namespace: vm.Namespace,
			Labels: map[string]string{
				"vm": vm.Name,
			},
		},
		Spec: transiva.VMSnapshotSpec{
			VMRef: transiva.VMReference{
				Name:      vm.Name,
				Namespace: vm.Namespace,
			},
			IncludeMemory: vmOp.Spec.SnapshotSpec.IncludeMemory,
			Quiesce:       vmOp.Spec.SnapshotSpec.Quiesce,
			Description:   vmOp.Spec.SnapshotSpec.Description,
		},
	}

	if err := r.Create(ctx, snapshot); err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	logger.Info("VM snapshot created", "vm", vm.Name, "snapshot", snapshot.Name)

	vmOp.Status.Progress = 100
	resultData, _ := json.Marshal(map[string]interface{}{
		"snapshotName":  snapshot.Name,
		"includeMemory": vmOp.Spec.SnapshotSpec.IncludeMemory,
	})
	vmOp.Status.Result = string(resultData)

	return nil
}

func (r *VMOperationReconciler) executeDelete(ctx context.Context, vmOp *transiva.VMOperation) error {
	logger := log.FromContext(ctx)

	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, vmOp, vm); err != nil {
		if errors.IsNotFound(err) {
			// VM already deleted
			return nil
		}
		return err
	}

	vmOp.Status.Progress = 50
	vmOp.Status.Message = "Deleting VM..."
	if err := r.Status().Update(ctx, vmOp); err != nil {
		return err
	}

	// Delete the VM
	if err := r.Delete(ctx, vm); err != nil {
		return err
	}

	logger.Info("VM deleted", "vm", vm.Name)

	vmOp.Status.Progress = 100
	return nil
}

// Helper functions

func (r *VMOperationReconciler) validateOperation(ctx context.Context, vmOp *transiva.VMOperation) error {
	// Get the target VM
	vm := &transiva.VirtualMachine{}
	if err := r.getVM(ctx, vmOp, vm); err != nil {
		return fmt.Errorf("failed to get VM: %w", err)
	}

	// Validate operation-specific requirements
	switch vmOp.Spec.Operation {
	case transiva.VMOpClone:
		if vmOp.Spec.CloneSpec == nil {
			return fmt.Errorf("clone spec is required")
		}
		if vmOp.Spec.CloneSpec.TargetName == "" {
			return fmt.Errorf("target name is required for clone operation")
		}
	case transiva.VMOpMigrate:
		if vmOp.Spec.MigrateSpec == nil {
			return fmt.Errorf("migrate spec is required")
		}
		if vmOp.Spec.MigrateSpec.TargetNode == "" {
			return fmt.Errorf("target node is required for migrate operation")
		}
	case transiva.VMOpResize:
		if vmOp.Spec.ResizeSpec == nil {
			return fmt.Errorf("resize spec is required")
		}
		if vmOp.Spec.ResizeSpec.CPUs == 0 && vmOp.Spec.ResizeSpec.Memory == "" {
			return fmt.Errorf("at least one of CPUs or Memory must be specified for resize operation")
		}
	case transiva.VMOpSnapshot:
		if vmOp.Spec.SnapshotSpec == nil {
			return fmt.Errorf("snapshot spec is required")
		}
		if vmOp.Spec.SnapshotSpec.Name == "" {
			return fmt.Errorf("snapshot name is required")
		}
	}

	return nil
}

func (r *VMOperationReconciler) getVM(ctx context.Context, vmOp *transiva.VMOperation, vm *transiva.VirtualMachine) error {
	namespace := vmOp.Spec.VMRef.Namespace
	if namespace == "" {
		namespace = vmOp.Namespace
	}

	return r.Get(ctx, types.NamespacedName{
		Name:      vmOp.Spec.VMRef.Name,
		Namespace: namespace,
	}, vm)
}

func (r *VMOperationReconciler) handleDeletion(ctx context.Context, vmOp *transiva.VMOperation) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(vmOp, vmOpFinalizer) {
		// Cleanup if needed
		controllerutil.RemoveFinalizer(vmOp, vmOpFinalizer)
		if err := r.Update(ctx, vmOp); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *VMOperationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&transiva.VMOperation{}).
		Complete(r)
}
