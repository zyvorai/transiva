// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	transiva "github.com/zyvorai/transiva/pkg/apis/transiva/v1alpha1"
)

const (
	vmTemplateFinalizer = "transiva.zyvor.ai/vmtemplate-finalizer"
)

// VMTemplateReconciler reconciles a VMTemplate object
type VMTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmtemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmtemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmtemplates/finalizers,verbs=update

// Reconcile handles VMTemplate reconciliation
func (r *VMTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the VMTemplate instance
	template := &transiva.VMTemplate{}
	if err := r.Get(ctx, req.NamespacedName, template); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !template.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, template)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(template, vmTemplateFinalizer) {
		controllerutil.AddFinalizer(template, vmTemplateFinalizer)
		if err := r.Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Validate template
	if err := r.validateTemplate(template); err != nil {
		logger.Error(err, "Template validation failed")
		template.Status.Ready = false
		r.updateCondition(template, "Valid", "False", "ValidationFailed", err.Error())
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Check if image is accessible
	accessible, size, err := r.checkImageAccessibility(ctx, template)
	if err != nil {
		logger.Error(err, "Failed to check image accessibility")
		template.Status.Ready = false
		r.updateCondition(template, "ImageAccessible", "False", "ImageCheckFailed", err.Error())
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		// Retry after 5 minutes
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	if !accessible {
		logger.Info("Template image not accessible yet", "source", template.Spec.Image.Source)
		template.Status.Ready = false
		r.updateCondition(template, "ImageAccessible", "False", "ImageNotAccessible", "Template image is not accessible")
		if err := r.Status().Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}
		// Retry after 1 minute
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Template is ready
	now := metav1.Now()
	template.Status.Ready = true
	template.Status.Size = size
	template.Status.LastUpdated = &now
	r.updateCondition(template, "Ready", "True", "TemplateReady", "Template is ready for use")

	if err := r.Status().Update(ctx, template); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("VMTemplate is ready", "template", template.Name)
	return ctrl.Result{}, nil
}

// validateTemplate validates the template specification
func (r *VMTemplateReconciler) validateTemplate(template *transiva.VMTemplate) error {
	// Check display name
	if template.Spec.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}

	// Check image source
	if template.Spec.Image.Source == "" {
		return fmt.Errorf("image source is required")
	}

	// Validate default spec if provided
	if template.Spec.DefaultSpec != nil {
		if template.Spec.DefaultSpec.CPUs < 1 {
			return fmt.Errorf("default CPUs must be at least 1")
		}
		if template.Spec.DefaultSpec.Memory == "" {
			return fmt.Errorf("default memory must be specified")
		}
	}

	return nil
}

// checkImageAccessibility checks if the template image is accessible
func (r *VMTemplateReconciler) checkImageAccessibility(ctx context.Context, template *transiva.VMTemplate) (bool, string, error) {
	// TODO: Implement actual image accessibility check
	// For now, simulate by checking if source is set
	if template.Spec.Image.Source == "" {
		return false, "", fmt.Errorf("image source is empty")
	}

	// Simulate image size
	size := "2.3Gi"
	if template.Spec.Image.Size != "" {
		size = template.Spec.Image.Size
	}

	return true, size, nil
}

// handleDeletion handles template deletion
func (r *VMTemplateReconciler) handleDeletion(ctx context.Context, template *transiva.VMTemplate) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(template, vmTemplateFinalizer) {
		// Check if any VMs are using this template
		vmList := &transiva.VirtualMachineList{}
		if err := r.List(ctx, vmList, client.InNamespace(template.Namespace)); err != nil {
			return ctrl.Result{}, err
		}

		usingVMs := []string{}
		for _, vm := range vmList.Items {
			if vm.Spec.Image != nil && vm.Spec.Image.TemplateRef != nil {
				if vm.Spec.Image.TemplateRef.Name == template.Name {
					usingVMs = append(usingVMs, vm.Name)
				}
			}
		}

		if len(usingVMs) > 0 {
			logger.Info("Template is in use, cannot delete", "template", template.Name, "vms", usingVMs)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("template is in use by %d VM(s)", len(usingVMs))
		}

		// Cleanup - delete cached image if any
		// TODO: Implement image cleanup

		// Remove finalizer
		controllerutil.RemoveFinalizer(template, vmTemplateFinalizer)
		if err := r.Update(ctx, template); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("VMTemplate deleted", "template", template.Name)
	}

	return ctrl.Result{}, nil
}

// updateCondition updates template condition
func (r *VMTemplateReconciler) updateCondition(template *transiva.VMTemplate, condType, status, reason, message string) {
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
	for i, c := range template.Status.Conditions {
		if c.Type == condType {
			template.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		template.Status.Conditions = append(template.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *VMTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&transiva.VMTemplate{}).
		Complete(r)
}
