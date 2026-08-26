// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
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
	migrationPolicyFinalizer = "transiva.zyvor.ai/migrationpolicy-finalizer"
)

// clampInt32 safely converts a non-negative int (e.g. a slice length) to
// int32, clamping to math.MaxInt32 instead of silently wrapping around if the
// count is implausibly large.
func clampInt32(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < 0 {
		return 0
	}
	return int32(n) // #nosec G115 -- bounds-checked above
}

// VMMigrationPolicyReconciler reconciles a VMMigrationPolicy object
type VMMigrationPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	cron   *cron.Cron
}

// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmmigrationpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmmigrationpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmmigrationpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=vmoperations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=transiva.zyvor.ai,resources=virtualmachines,verbs=get;list;watch;update;patch

// Reconcile handles VMMigrationPolicy reconciliation
func (r *VMMigrationPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the VMMigrationPolicy instance
	policy := &transiva.VMMigrationPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !policy.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, policy)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(policy, migrationPolicyFinalizer) {
		controllerutil.AddFinalizer(policy, migrationPolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	// If policy is not enabled, skip processing
	if !policy.Spec.Enabled {
		logger.Info("Migration policy is disabled", "policy", policy.Name)
		policy.Status.Active = false
		if err := r.Status().Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	policy.Status.Active = true
	policy.Status.LastEvaluationTime = &metav1.Time{Time: time.Now()}

	// Evaluate migration triggers
	migrations, err := r.evaluateTriggers(ctx, policy)
	if err != nil {
		logger.Error(err, "Failed to evaluate migration triggers")
		return ctrl.Result{}, err
	}

	// Execute migrations if not in dry-run mode
	if !policy.Spec.DryRun && len(migrations) > 0 {
		logger.Info("Executing migrations", "policy", policy.Name, "count", len(migrations))

		for _, migration := range migrations {
			if err := r.executeMigration(ctx, policy, migration); err != nil {
				logger.Error(err, "Failed to execute migration", "vm", migration.VMName)
				policy.Status.FailedMigrations++
			} else {
				policy.Status.SuccessfulMigrations++
				policy.Status.LastMigrationTime = &metav1.Time{Time: time.Now()}
			}
		}
	} else if policy.Spec.DryRun {
		logger.Info("Dry-run mode: would execute migrations", "count", len(migrations))
		policy.Status.RecommendedMigrations = clampInt32(len(migrations))
	}

	policy.Status.TotalMigrations = policy.Status.SuccessfulMigrations + policy.Status.FailedMigrations

	// Update status
	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue based on schedule
	requeueAfter := 5 * time.Minute
	if policy.Spec.Schedule != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		cronSchedule, err := parser.Parse(policy.Spec.Schedule)
		if err == nil {
			nextCheck := cronSchedule.Next(time.Now())
			requeueAfter = time.Until(nextCheck)
			if requeueAfter < 0 {
				requeueAfter = 1 * time.Minute
			}
		}
	}

	logger.Info("Migration policy evaluation complete", "policy", policy.Name, "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// evaluateTriggers evaluates all triggers and returns recommended migrations
func (r *VMMigrationPolicyReconciler) evaluateTriggers(ctx context.Context, policy *transiva.VMMigrationPolicy) ([]transiva.MigrationRecord, error) {
	logger := log.FromContext(ctx)

	var migrations []transiva.MigrationRecord

	// Get all VMs in the namespace
	vmList := &transiva.VirtualMachineList{}
	if err := r.List(ctx, vmList, client.InNamespace(policy.Namespace)); err != nil {
		return nil, err
	}

	// Evaluate each trigger
	for _, trigger := range policy.Spec.Triggers {
		logger.Info("Evaluating trigger", "type", trigger.Type, "threshold", trigger.Threshold)

		switch trigger.Type {
		case transiva.TriggerNodePressure:
			// Check for node resource pressure
			nodeMigrations := r.evaluateNodePressure(ctx, vmList.Items, trigger, policy)
			migrations = append(migrations, nodeMigrations...)

		case transiva.TriggerCarbonIntensity:
			// Check for high carbon intensity
			carbonMigrations := r.evaluateCarbonIntensity(ctx, vmList.Items, trigger, policy)
			migrations = append(migrations, carbonMigrations...)

		case transiva.TriggerLoadBalancing:
			// Check for load imbalance
			balancingMigrations := r.evaluateLoadBalancing(ctx, vmList.Items, trigger, policy)
			migrations = append(migrations, balancingMigrations...)

		case transiva.TriggerMaintenance:
			// Check for maintenance windows
			maintenanceMigrations := r.evaluateMaintenance(ctx, vmList.Items, trigger, policy)
			migrations = append(migrations, maintenanceMigrations...)
		}
	}

	// Apply constraints
	migrations = r.applyConstraints(migrations, policy)

	return migrations, nil
}

// evaluateNodePressure checks for high node resource utilization
func (r *VMMigrationPolicyReconciler) evaluateNodePressure(ctx context.Context, vms []transiva.VirtualMachine, trigger transiva.MigrationTrigger, policy *transiva.VMMigrationPolicy) []transiva.MigrationRecord {
	var migrations []transiva.MigrationRecord

	// Group VMs by node
	nodeVMs := make(map[string][]transiva.VirtualMachine)
	for _, vm := range vms {
		if vm.Status.NodeName != "" && vm.Status.Phase == transiva.VMPhaseRunning {
			nodeVMs[vm.Status.NodeName] = append(nodeVMs[vm.Status.NodeName], vm)
		}
	}

	// Check each node for pressure (simplified - would need real metrics in production)
	for nodeName, vmsOnNode := range nodeVMs {
		if len(vmsOnNode) > 5 { // Simplified threshold
			// Select VMs to migrate
			for i := 0; i < len(vmsOnNode) && i < 2; i++ {
				vm := vmsOnNode[i]
				targetNode := r.selectTargetNode(ctx, vm, nodeName, policy)

				migrations = append(migrations, transiva.MigrationRecord{
					VMName:      vm.Name,
					FromNode:    nodeName,
					ToNode:      targetNode,
					TriggerType: trigger.Type,
					StartTime:   metav1.Now(),
					Status:      "pending",
					Reason:      fmt.Sprintf("Node %s under pressure", nodeName),
				})
			}
		}
	}

	return migrations
}

// evaluateCarbonIntensity checks for high carbon intensity zones
func (r *VMMigrationPolicyReconciler) evaluateCarbonIntensity(ctx context.Context, vms []transiva.VirtualMachine, trigger transiva.MigrationTrigger, policy *transiva.VMMigrationPolicy) []transiva.MigrationRecord {
	var migrations []transiva.MigrationRecord

	// Check VMs with carbon awareness enabled
	for _, vm := range vms {
		if vm.Spec.CarbonAware != nil && vm.Spec.CarbonAware.Enabled {
			// If carbon intensity is high, suggest migration
			if vm.Status.CarbonIntensity > 300 { // Simplified threshold in gCO2/kWh
				targetNode := r.selectTargetNode(ctx, vm, vm.Status.NodeName, policy)

				migrations = append(migrations, transiva.MigrationRecord{
					VMName:      vm.Name,
					FromNode:    vm.Status.NodeName,
					ToNode:      targetNode,
					TriggerType: trigger.Type,
					StartTime:   metav1.Now(),
					Status:      "pending",
					Reason:      fmt.Sprintf("High carbon intensity: %.1f gCO2/kWh", vm.Status.CarbonIntensity),
				})
			}
		}
	}

	return migrations
}

// evaluateLoadBalancing checks for load imbalance across nodes
func (r *VMMigrationPolicyReconciler) evaluateLoadBalancing(ctx context.Context, vms []transiva.VirtualMachine, trigger transiva.MigrationTrigger, policy *transiva.VMMigrationPolicy) []transiva.MigrationRecord {
	var migrations []transiva.MigrationRecord

	// Group VMs by node
	nodeVMs := make(map[string][]transiva.VirtualMachine)
	for _, vm := range vms {
		if vm.Status.NodeName != "" && vm.Status.Phase == transiva.VMPhaseRunning {
			nodeVMs[vm.Status.NodeName] = append(nodeVMs[vm.Status.NodeName], vm)
		}
	}

	// Find most and least loaded nodes
	var maxNode, minNode string
	maxCount, minCount := 0, 999999
	for nodeName, vmsOnNode := range nodeVMs {
		count := len(vmsOnNode)
		if count > maxCount {
			maxCount = count
			maxNode = nodeName
		}
		if count < minCount {
			minCount = count
			minNode = nodeName
		}
	}

	// If imbalance is significant, migrate VMs from max to min
	if maxCount-minCount > 3 && maxNode != "" && minNode != "" {
		vmsToMove := (maxCount - minCount) / 2
		for i := 0; i < vmsToMove && i < len(nodeVMs[maxNode]); i++ {
			vm := nodeVMs[maxNode][i]

			migrations = append(migrations, transiva.MigrationRecord{
				VMName:      vm.Name,
				FromNode:    maxNode,
				ToNode:      minNode,
				TriggerType: trigger.Type,
				StartTime:   metav1.Now(),
				Status:      "pending",
				Reason:      fmt.Sprintf("Load balancing: %s (%d VMs) -> %s (%d VMs)", maxNode, maxCount, minNode, minCount),
			})
		}
	}

	return migrations
}

// evaluateMaintenance checks for planned maintenance windows
func (r *VMMigrationPolicyReconciler) evaluateMaintenance(ctx context.Context, vms []transiva.VirtualMachine, trigger transiva.MigrationTrigger, policy *transiva.VMMigrationPolicy) []transiva.MigrationRecord {
	var migrations []transiva.MigrationRecord

	// Check VMs on nodes marked for maintenance (simplified)
	for _, vm := range vms {
		if vm.Status.NodeName != "" && strings.Contains(vm.Status.NodeName, "maint") {
			targetNode := r.selectTargetNode(ctx, vm, vm.Status.NodeName, policy)

			migrations = append(migrations, transiva.MigrationRecord{
				VMName:      vm.Name,
				FromNode:    vm.Status.NodeName,
				ToNode:      targetNode,
				TriggerType: trigger.Type,
				StartTime:   metav1.Now(),
				Status:      "pending",
				Reason:      "Node under maintenance",
			})
		}
	}

	return migrations
}

// selectTargetNode selects the best target node for migration
func (r *VMMigrationPolicyReconciler) selectTargetNode(ctx context.Context, vm transiva.VirtualMachine, currentNode string, policy *transiva.VMMigrationPolicy) string {
	strategy := policy.Spec.TargetSelection.Strategy

	// Simplified node selection based on strategy
	switch strategy {
	case transiva.StrategyLeastUtilized:
		// Would query metrics to find least utilized node
		return "node-low-util"
	case transiva.StrategyLowestCarbon:
		// Would check carbon intensity data
		return "node-low-carbon"
	case transiva.StrategyRoundRobin:
		// Simple round-robin
		nodeNum := 1
		if strings.Contains(currentNode, "1") {
			nodeNum = 2
		}
		return fmt.Sprintf("node-%d", nodeNum)
	default:
		return "node-1"
	}
}

// applyConstraints filters migrations based on constraints
func (r *VMMigrationPolicyReconciler) applyConstraints(migrations []transiva.MigrationRecord, policy *transiva.VMMigrationPolicy) []transiva.MigrationRecord {
	constraints := policy.Spec.Constraints

	// Apply max concurrent migrations limit
	if constraints.MaxConcurrentMigrations > 0 && clampInt32(len(migrations)) > constraints.MaxConcurrentMigrations {
		// Sort by priority and take top N
		sort.SliceStable(migrations, func(i, j int) bool {
			return migrations[i].TriggerType < migrations[j].TriggerType // Simple priority
		})
		migrations = migrations[:constraints.MaxConcurrentMigrations]
	}

	// Apply time window constraints
	now := time.Now()
	if len(constraints.AllowedTimeWindows) > 0 {
		inWindow := false
		for _, window := range constraints.AllowedTimeWindows {
			if r.isInTimeWindow(now, window) {
				inWindow = true
				break
			}
		}
		if !inWindow {
			return []transiva.MigrationRecord{} // Not in allowed window
		}
	}

	// Check blocked time windows
	for _, window := range constraints.BlockedTimeWindows {
		if r.isInTimeWindow(now, window) {
			return []transiva.MigrationRecord{} // In blocked window
		}
	}

	return migrations
}

// isInTimeWindow checks if current time is within a time window
func (r *VMMigrationPolicyReconciler) isInTimeWindow(now time.Time, window transiva.TimeWindow) bool {
	// Simplified time window check
	// In production, would parse start/end times and check against current time
	currentHour := now.Hour()

	// Parse start hour from "HH:MM" format
	startParts := strings.Split(window.Start, ":")
	if len(startParts) == 2 {
		startHour, _ := strconv.Atoi(startParts[0])
		endParts := strings.Split(window.End, ":")
		if len(endParts) == 2 {
			endHour, _ := strconv.Atoi(endParts[0])

			if currentHour >= startHour && currentHour < endHour {
				return true
			}
		}
	}

	return false
}

// executeMigration creates a VMOperation for migration
func (r *VMMigrationPolicyReconciler) executeMigration(ctx context.Context, policy *transiva.VMMigrationPolicy, migration transiva.MigrationRecord) error {
	logger := log.FromContext(ctx)

	opName := fmt.Sprintf("%s-migrate-%d", migration.VMName, time.Now().Unix())

	vmOp := &transiva.VMOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opName,
			Namespace: policy.Namespace,
			Labels: map[string]string{
				"migration-policy": policy.Name,
				"trigger-type":     string(migration.TriggerType),
			},
		},
		Spec: transiva.VMOperationSpec{
			VMRef: transiva.VMReference{
				Name:      migration.VMName,
				Namespace: policy.Namespace,
			},
			Operation: transiva.VMOpMigrate,
			MigrateSpec: &transiva.MigrateSpec{
				TargetNode: migration.ToNode,
				Live:       policy.Spec.Constraints.RequireVMLive,
			},
		},
	}

	if err := r.Create(ctx, vmOp); err != nil {
		return fmt.Errorf("failed to create migration operation: %w", err)
	}

	logger.Info("Created migration operation", "vm", migration.VMName, "from", migration.FromNode, "to", migration.ToNode)

	// Add to triggered migrations
	policy.Status.TriggeredMigrations = append(policy.Status.TriggeredMigrations, migration)
	policy.Status.PendingMigrations++

	return nil
}

// handleDeletion handles policy deletion
func (r *VMMigrationPolicyReconciler) handleDeletion(ctx context.Context, policy *transiva.VMMigrationPolicy) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(policy, migrationPolicyFinalizer) {
		// Cleanup if needed
		controllerutil.RemoveFinalizer(policy, migrationPolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *VMMigrationPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize cron
	r.cron = cron.New()

	return ctrl.NewControllerManagedBy(mgr).
		For(&transiva.VMMigrationPolicy{}).
		Complete(r)
}
