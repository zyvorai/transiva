// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
	vmFinalizer = "transiva.zyvor.ai/vm-finalizer"
)

// VirtualMachineReconciler reconciles a VirtualMachine object
type VirtualMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=virtualmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=virtualmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=virtualmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles VirtualMachine reconciliation
func (r *VirtualMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the VirtualMachine instance
	vm := &transiva.VirtualMachine{}
	if err := r.Get(ctx, req.NamespacedName, vm); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !vm.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, vm)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(vm, vmFinalizer) {
		controllerutil.AddFinalizer(vm, vmFinalizer)
		if err := r.Update(ctx, vm); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize status if needed
	if vm.Status.Phase == "" {
		vm.Status.Phase = transiva.VMPhasePending
		vm.Status.CreationTimestamp = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(ctx, vm); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile based on current phase
	switch vm.Status.Phase {
	case transiva.VMPhasePending:
		return r.reconcilePending(ctx, vm)
	case transiva.VMPhaseCreating:
		return r.reconcileCreating(ctx, vm)
	case transiva.VMPhaseRunning:
		return r.reconcileRunning(ctx, vm)
	case transiva.VMPhaseStopped:
		return r.reconcileStopped(ctx, vm)
	case transiva.VMPhaseFailed:
		return r.reconcileFailed(ctx, vm)
	default:
		logger.Info("Unknown VM phase", "phase", vm.Status.Phase)
		return ctrl.Result{}, nil
	}
}

// reconcilePending handles pending VMs
func (r *VirtualMachineReconciler) reconcilePending(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate VM spec
	if err := r.validateVMSpec(vm); err != nil {
		vm.Status.Phase = transiva.VMPhaseFailed
		r.updateCondition(vm, "ValidationFailed", corev1.ConditionFalse, "ValidationError", err.Error())
		if err := r.Status().Update(ctx, vm); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Check carbon-aware scheduling
	if vm.Spec.CarbonAware != nil && vm.Spec.CarbonAware.Enabled {
		ready, err := r.checkCarbonIntensity(ctx, vm)
		if err != nil {
			logger.Error(err, "Failed to check carbon intensity")
		} else if !ready {
			logger.Info("Waiting for lower carbon intensity", "maxIntensity", vm.Spec.CarbonAware.MaxIntensity)
			r.updateCondition(vm, "CarbonAware", corev1.ConditionFalse, "WaitingForGreenEnergy", "Waiting for carbon intensity to drop")
			if err := r.Status().Update(ctx, vm); err != nil {
				return ctrl.Result{}, err
			}
			// Requeue after 5 minutes
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
	}

	// Select node if not already assigned
	if vm.Status.NodeName == "" {
		nodeName, err := r.selectNode(ctx, vm)
		if err != nil {
			logger.Error(err, "Failed to select node")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		vm.Status.NodeName = nodeName
	}

	// Transition to Creating
	vm.Status.Phase = transiva.VMPhaseCreating
	r.updateCondition(vm, "Creating", corev1.ConditionTrue, "VMCreating", "VM resources are being created")
	if err := r.Status().Update(ctx, vm); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// reconcileCreating handles VM creation
func (r *VirtualMachineReconciler) reconcileCreating(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Create PVCs for disks
	for _, disk := range vm.Spec.Disks {
		pvcName := fmt.Sprintf("%s-%s", vm.Name, disk.Name)
		pvc := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: vm.Namespace}, pvc)

		if errors.IsNotFound(err) {
			// Create PVC
			pvc = r.buildPVCForDisk(vm, &disk)
			if err := controllerutil.SetControllerReference(vm, pvc, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, pvc); err != nil {
				logger.Error(err, "Failed to create PVC", "pvc", pvcName)
				return ctrl.Result{}, err
			}
			logger.Info("Created PVC", "pvc", pvcName)
		} else if err != nil {
			return ctrl.Result{}, err
		}

		// Check if PVC is bound
		if pvc.Status.Phase != corev1.ClaimBound {
			logger.Info("Waiting for PVC to be bound", "pvc", pvcName)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// Create VM pod
	pod := &corev1.Pod{}
	podName := vm.Name
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: vm.Namespace}, pod)

	if errors.IsNotFound(err) {
		// Build and create pod
		pod = r.buildPodForVM(vm)
		if err := controllerutil.SetControllerReference(vm, pod, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, pod); err != nil {
			logger.Error(err, "Failed to create VM pod")
			return ctrl.Result{}, err
		}
		logger.Info("Created VM pod", "pod", podName)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Check pod status
	if pod.Status.Phase == corev1.PodRunning {
		// VM is now running
		vm.Status.Phase = transiva.VMPhaseRunning
		vm.Status.StartTime = &metav1.Time{Time: time.Now()}

		// Extract IP addresses
		if len(pod.Status.PodIPs) > 0 {
			vm.Status.IPAddresses = make([]string, len(pod.Status.PodIPs))
			for i, ip := range pod.Status.PodIPs {
				vm.Status.IPAddresses[i] = ip.IP
			}
		}

		r.updateCondition(vm, "Ready", corev1.ConditionTrue, "VMReady", "VM is running and ready")
		if err := r.Status().Update(ctx, vm); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("VM is now running", "vm", vm.Name)
		return ctrl.Result{}, nil
	}

	// Still creating
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// reconcileRunning handles running VMs
func (r *VirtualMachineReconciler) reconcileRunning(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if VM should be stopped
	if !vm.Spec.Running {
		return r.stopVM(ctx, vm)
	}

	// Get VM pod
	pod := &corev1.Pod{}
	podName := vm.Name
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: vm.Namespace}, pod)

	if errors.IsNotFound(err) {
		// Pod is gone, VM failed
		vm.Status.Phase = transiva.VMPhaseFailed
		r.updateCondition(vm, "Ready", corev1.ConditionFalse, "PodLost", "VM pod was deleted unexpectedly")
		if err := r.Status().Update(ctx, vm); err != nil {
			return ctrl.Result{}, err
		}
		return r.handleVMFailure(ctx, vm)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Check pod health
	if pod.Status.Phase != corev1.PodRunning {
		logger.Info("VM pod not running", "phase", pod.Status.Phase)
		vm.Status.Phase = transiva.VMPhaseFailed
		r.updateCondition(vm, "Ready", corev1.ConditionFalse, "PodNotRunning", fmt.Sprintf("VM pod phase: %s", pod.Status.Phase))
		if err := r.Status().Update(ctx, vm); err != nil {
			return ctrl.Result{}, err
		}
		return r.handleVMFailure(ctx, vm)
	}

	// Update resource usage (mock for now - would query QEMU/guest agent)
	vm.Status.Resources = &transiva.VMResourceStatus{
		CPU: &transiva.ResourceMetrics{
			Usage:    "45%",
			Requests: fmt.Sprintf("%d", vm.Spec.CPUs),
		},
		Memory: &transiva.ResourceMetrics{
			Usage:    "6.2Gi",
			Requests: vm.Spec.Memory,
		},
	}

	if err := r.Status().Update(ctx, vm); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue for periodic health check
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileStopped handles stopped VMs
func (r *VirtualMachineReconciler) reconcileStopped(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	// Check if VM should be started
	if vm.Spec.Running {
		return r.startVM(ctx, vm)
	}

	// Ensure pod is deleted
	pod := &corev1.Pod{}
	podName := vm.Name
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: vm.Namespace}, pod)

	if err == nil {
		// Pod still exists, delete it
		if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	} else if !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// VM is properly stopped
	return ctrl.Result{}, nil
}

// reconcileFailed handles failed VMs
func (r *VirtualMachineReconciler) reconcileFailed(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	// Check if HA is enabled
	if vm.Spec.HighAvailability != nil && vm.Spec.HighAvailability.Enabled {
		return r.handleVMFailure(ctx, vm)
	}

	// Manual intervention required
	return ctrl.Result{}, nil
}

// handleDeletion handles VM deletion
func (r *VirtualMachineReconciler) handleDeletion(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(vm, vmFinalizer) {
		// Delete VM pod
		pod := &corev1.Pod{}
		podName := vm.Name
		err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: vm.Namespace}, pod)
		if err == nil {
			if err := r.Delete(ctx, pod); err != nil {
				return ctrl.Result{}, err
			}
			logger.Info("Deleted VM pod", "pod", podName)
		} else if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		// Delete PVCs
		for _, disk := range vm.Spec.Disks {
			pvcName := fmt.Sprintf("%s-%s", vm.Name, disk.Name)
			pvc := &corev1.PersistentVolumeClaim{}
			err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: vm.Namespace}, pvc)
			if err == nil {
				if err := r.Delete(ctx, pvc); err != nil {
					return ctrl.Result{}, err
				}
				logger.Info("Deleted PVC", "pvc", pvcName)
			} else if !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(vm, vmFinalizer)
		if err := r.Update(ctx, vm); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// stopVM stops a running VM
func (r *VirtualMachineReconciler) stopVM(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Delete the pod
	pod := &corev1.Pod{}
	podName := vm.Name
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: vm.Namespace}, pod)

	if err == nil {
		if err := r.Delete(ctx, pod); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Stopped VM", "vm", vm.Name)
	} else if !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Update status
	vm.Status.Phase = transiva.VMPhaseStopped
	vm.Status.IPAddresses = nil
	r.updateCondition(vm, "Ready", corev1.ConditionFalse, "VMStopped", "VM has been stopped")
	if err := r.Status().Update(ctx, vm); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// startVM starts a stopped VM
func (r *VirtualMachineReconciler) startVM(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Transition to Creating phase
	vm.Status.Phase = transiva.VMPhaseCreating
	r.updateCondition(vm, "Starting", corev1.ConditionTrue, "VMStarting", "VM is starting")
	if err := r.Status().Update(ctx, vm); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Starting VM", "vm", vm.Name)
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// handleVMFailure handles VM failures with HA
func (r *VirtualMachineReconciler) handleVMFailure(ctx context.Context, vm *transiva.VirtualMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if vm.Spec.HighAvailability == nil || !vm.Spec.HighAvailability.Enabled {
		return ctrl.Result{}, nil
	}

	// Check restart policy
	if vm.Spec.HighAvailability.RestartPolicy == "Never" {
		return ctrl.Result{}, nil
	}

	// TODO: Implement restart count tracking and max restarts
	// For now, always attempt restart

	logger.Info("Attempting to restart failed VM", "vm", vm.Name)

	// Transition back to Pending to restart
	vm.Status.Phase = transiva.VMPhasePending
	r.updateCondition(vm, "Restarting", corev1.ConditionTrue, "VMRestarting", "VM is being restarted due to failure")
	if err := r.Status().Update(ctx, vm); err != nil {
		return ctrl.Result{}, err
	}

	// TODO: Respect RestartDelay
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// Helper functions

func (r *VirtualMachineReconciler) validateVMSpec(vm *transiva.VirtualMachine) error {
	if vm.Spec.CPUs < 1 {
		return fmt.Errorf("CPUs must be at least 1")
	}
	if vm.Spec.Memory == "" {
		return fmt.Errorf("memory must be specified")
	}
	// Add more validation as needed
	return nil
}

func (r *VirtualMachineReconciler) checkCarbonIntensity(ctx context.Context, vm *transiva.VirtualMachine) (bool, error) {
	// TODO: Integrate with carbon intensity API
	// For now, always return true
	return true, nil
}

func (r *VirtualMachineReconciler) selectNode(ctx context.Context, vm *transiva.VirtualMachine) (string, error) {
	// TODO: Implement proper node selection based on:
	// - nodeSelector
	// - affinity
	// - carbon intensity
	// - resource availability

	// For now, return first available node or use nodeSelector
	if len(vm.Spec.NodeSelector) > 0 {
		// Simple implementation - would need proper scheduler integration
		return "node-1", nil
	}

	return "node-1", nil
}

func (r *VirtualMachineReconciler) updateCondition(vm *transiva.VirtualMachine, condType string, status corev1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := transiva.VMCondition{
		Type:               condType,
		Status:             string(status),
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Update or append condition
	found := false
	for i, c := range vm.Status.Conditions {
		if c.Type == condType {
			vm.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		vm.Status.Conditions = append(vm.Status.Conditions, condition)
	}
}

func (r *VirtualMachineReconciler) buildPVCForDisk(vm *transiva.VirtualMachine, disk *transiva.VMDisk) *corev1.PersistentVolumeClaim {
	storageClass := disk.StorageClass
	if storageClass == "" {
		storageClass = "standard"
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", vm.Name, disk.Name),
			Namespace: vm.Namespace,
			Labels: map[string]string{
				"app":  "transiva-vm",
				"vm":   vm.Name,
				"disk": disk.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(disk.Size),
				},
			},
		},
	}
}

func (r *VirtualMachineReconciler) buildPodForVM(vm *transiva.VirtualMachine) *corev1.Pod {
	// This is a simplified pod spec
	// In production, would use KubeVirt VirtualMachineInstance or custom VM runtime

	privileged := true

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vm.Name,
			Namespace: vm.Namespace,
			Labels: map[string]string{
				"app": "transiva-vm",
				"vm":  vm.Name,
			},
		},
		Spec: corev1.PodSpec{
			NodeName: vm.Status.NodeName,
			Containers: []corev1.Container{
				{
					Name:  "vm",
					Image: "github.com/zyvorai/transiva/vm-runtime:latest", // Custom VM runtime image
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
					Env: []corev1.EnvVar{
						{Name: "VM_NAME", Value: vm.Name},
						{Name: "VM_CPUS", Value: fmt.Sprintf("%d", vm.Spec.CPUs)},
						{Name: "VM_MEMORY", Value: vm.Spec.Memory},
					},
					// Add volume mounts for disks
					VolumeMounts: r.buildVolumeMounts(vm),
				},
			},
			Volumes: r.buildVolumes(vm),
		},
	}

	// Apply node selector
	if len(vm.Spec.NodeSelector) > 0 {
		pod.Spec.NodeSelector = vm.Spec.NodeSelector
	}

	// Apply affinity
	if vm.Spec.Affinity != nil {
		pod.Spec.Affinity = vm.Spec.Affinity
	}

	// Apply tolerations
	if len(vm.Spec.Tolerations) > 0 {
		pod.Spec.Tolerations = vm.Spec.Tolerations
	}

	return pod
}

func (r *VirtualMachineReconciler) buildVolumeMounts(vm *transiva.VirtualMachine) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{}
	for _, disk := range vm.Spec.Disks {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      disk.Name,
			MountPath: fmt.Sprintf("/vm/disks/%s", disk.Name),
		})
	}
	return mounts
}

func (r *VirtualMachineReconciler) buildVolumes(vm *transiva.VirtualMachine) []corev1.Volume {
	volumes := []corev1.Volume{}
	for _, disk := range vm.Spec.Disks {
		volumes = append(volumes, corev1.Volume{
			Name: disk.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-%s", vm.Name, disk.Name),
				},
			},
		})
	}
	return volumes
}

func parseQuantity(size string) resource.Quantity {
	// Parse size string (e.g., "50Gi") to Quantity
	// Simplified - would use apimachinery/pkg/api/resource
	return resource.MustParse(size)
}

// SetupWithManager sets up the controller with the Manager
func (r *VirtualMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&transiva.VirtualMachine{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
