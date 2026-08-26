// SPDX-License-Identifier: Apache-2.0

// +build full

package kubevirt

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

// StartVM starts a virtual machine
func (p *KubeVirtProvider) StartVM(ctx context.Context, identifier string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("starting VM", "name", name, "namespace", namespace)

	// Get current VM
	vm, err := p.virtClient.VirtualMachine(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get VM: %w", err)
	}

	// Check if already running
	if vm.Spec.Running != nil && *vm.Spec.Running {
		p.logger.Info("VM already running", "name", name)
		return nil
	}

	// Start VM by setting spec.running = true
	running := true
	vm.Spec.Running = &running

	_, err = p.virtClient.VirtualMachine(namespace).Update(ctx, vm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	p.logger.Info("VM start initiated", "name", name)

	// Wait for VM to start (optional - with timeout)
	return p.waitForVMState(ctx, namespace, name, "Running", 2*time.Minute)
}

// StopVM stops a virtual machine gracefully
func (p *KubeVirtProvider) StopVM(ctx context.Context, identifier string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("stopping VM", "name", name, "namespace", namespace)

	// Get current VM
	vm, err := p.virtClient.VirtualMachine(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get VM: %w", err)
	}

	// Check if already stopped
	if vm.Spec.Running == nil || !*vm.Spec.Running {
		p.logger.Info("VM already stopped", "name", name)
		return nil
	}

	// Stop VM by setting spec.running = false
	running := false
	vm.Spec.Running = &running

	_, err = p.virtClient.VirtualMachine(namespace).Update(ctx, vm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	p.logger.Info("VM stop initiated", "name", name)

	return nil
}

// RestartVM restarts a virtual machine
func (p *KubeVirtProvider) RestartVM(ctx context.Context, identifier string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("restarting VM", "name", name, "namespace", namespace)

	// Stop VM first
	if err := p.StopVM(ctx, identifier); err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	// Wait for VM to stop
	if err := p.waitForVMState(ctx, namespace, name, "Stopped", 2*time.Minute); err != nil {
		p.logger.Warn("VM did not stop cleanly, attempting force restart", "error", err)
	}

	// Start VM
	if err := p.StartVM(ctx, identifier); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	p.logger.Info("VM restarted successfully", "name", name)

	return nil
}

// PauseVM pauses a virtual machine
func (p *KubeVirtProvider) PauseVM(ctx context.Context, identifier string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("pausing VM", "name", name, "namespace", namespace)

	// Pause the VMI (VirtualMachineInstance)
	err := p.virtClient.VirtualMachineInstance(namespace).Pause(ctx, name, &kubevirtv1.PauseOptions{})
	if err != nil {
		return fmt.Errorf("failed to pause VM: %w", err)
	}

	p.logger.Info("VM paused successfully", "name", name)

	return nil
}

// UnpauseVM resumes a paused virtual machine
func (p *KubeVirtProvider) UnpauseVM(ctx context.Context, identifier string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("unpausing VM", "name", name, "namespace", namespace)

	// Unpause the VMI
	err := p.virtClient.VirtualMachineInstance(namespace).Unpause(ctx, name, &kubevirtv1.UnpauseOptions{})
	if err != nil {
		return fmt.Errorf("failed to unpause VM: %w", err)
	}

	p.logger.Info("VM unpaused successfully", "name", name)

	return nil
}

// MigrateVM migrates a virtual machine to another node
func (p *KubeVirtProvider) MigrateVM(ctx context.Context, identifier string, targetNode string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("migrating VM", "name", name, "namespace", namespace, "target", targetNode)

	// Create VirtualMachineInstanceMigration
	migration := &kubevirtv1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-migration-", name),
			Namespace:    namespace,
		},
		Spec: kubevirtv1.VirtualMachineInstanceMigrationSpec{
			VMIName: name,
		},
	}

	_, err := p.virtClient.VirtualMachineInstanceMigration(namespace).Create(ctx, migration, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create migration: %w", err)
	}

	p.logger.Info("VM migration initiated", "name", name)

	return nil
}

// CloneVM creates a clone of a virtual machine
func (p *KubeVirtProvider) CloneVM(ctx context.Context, identifier string, cloneName string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("cloning VM", "source", name, "clone", cloneName, "namespace", namespace)

	// Get source VM
	sourceVM, err := p.virtClient.VirtualMachine(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get source VM: %w", err)
	}

	// Create a deep copy for the clone
	cloneVM := sourceVM.DeepCopy()

	// Update metadata
	cloneVM.ObjectMeta = metav1.ObjectMeta{
		Name:      cloneName,
		Namespace: namespace,
		Labels:    sourceVM.Labels,
	}

	// Clear status and resource version
	cloneVM.Status = kubevirtv1.VirtualMachineStatus{}
	cloneVM.ResourceVersion = ""
	cloneVM.UID = ""

	// Set running to false for clone
	running := false
	cloneVM.Spec.Running = &running

	// TODO: Clone DataVolumes/PVCs - this is simplified
	// In production, you'd need to:
	// 1. Clone all DataVolumes
	// 2. Update volume references
	// 3. Wait for clones to complete

	_, err = p.virtClient.VirtualMachine(namespace).Create(ctx, cloneVM, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create clone: %w", err)
	}

	p.logger.Info("VM clone created", "source", name, "clone", cloneName)

	return nil
}

// DeleteVM deletes a virtual machine
func (p *KubeVirtProvider) DeleteVM(ctx context.Context, identifier string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("deleting VM", "name", name, "namespace", namespace)

	// Delete the VirtualMachine
	err := p.virtClient.VirtualMachine(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete VM: %w", err)
	}

	p.logger.Info("VM deleted successfully", "name", name)

	return nil
}

// GetVMStatus returns the current status of a VM
func (p *KubeVirtProvider) GetVMStatus(ctx context.Context, identifier string) (string, error) {
	if p.virtClient == nil {
		return "", fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	// Get VirtualMachine
	vm, err := p.virtClient.VirtualMachine(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get VM: %w", err)
	}

	// Check if running
	if vm.Spec.Running == nil || !*vm.Spec.Running {
		return "Stopped", nil
	}

	// Get VirtualMachineInstance for detailed status
	vmi, err := p.virtClient.VirtualMachineInstance(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// VMI might not exist yet if VM is starting
		return "Starting", nil
	}

	return string(vmi.Status.Phase), nil
}

// waitForVMState waits for VM to reach desired state with timeout
func (p *KubeVirtProvider) waitForVMState(ctx context.Context, namespace, name, desiredState string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for VM to reach state %s", desiredState)
		case <-ticker.C:
			status, err := p.GetVMStatus(ctx, fmt.Sprintf("%s/%s", namespace, name))
			if err != nil {
				p.logger.Warn("failed to get VM status", "error", err)
				continue
			}

			if status == desiredState {
				return nil
			}

			p.logger.Debug("waiting for VM state", "current", status, "desired", desiredState)
		}
	}
}

// parseIdentifier parses VM identifier into namespace and name
func (p *KubeVirtProvider) parseIdentifier(identifier string) (namespace, name string) {
	if parts := strings.Split(identifier, "/"); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return p.namespace, identifier
}

// GetVMMetrics returns resource usage metrics for a VM
func (p *KubeVirtProvider) GetVMMetrics(ctx context.Context, identifier string) (map[string]interface{}, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	// Get VirtualMachineInstance
	vmi, err := p.virtClient.VirtualMachineInstance(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get VMI: %w", err)
	}

	metrics := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
		"phase":     vmi.Status.Phase,
		"nodeName":  vmi.Status.NodeName,
	}

	// Add resource usage if available
	if vmi.Status.GuestOSInfo.Name != "" {
		metrics["guestOS"] = map[string]string{
			"name":    vmi.Status.GuestOSInfo.Name,
			"version": vmi.Status.GuestOSInfo.Version,
		}
	}

	// Add interface info
	if len(vmi.Status.Interfaces) > 0 {
		interfaces := make([]map[string]interface{}, 0, len(vmi.Status.Interfaces))
		for _, iface := range vmi.Status.Interfaces {
			interfaces = append(interfaces, map[string]interface{}{
				"name":      iface.Name,
				"ip":        iface.IP,
				"ips":       iface.IPs,
				"mac":       iface.MAC,
				"interface": iface.InterfaceName,
			})
		}
		metrics["interfaces"] = interfaces
	}

	return metrics, nil
}

// PatchVM applies a JSON patch to a VM
func (p *KubeVirtProvider) PatchVM(ctx context.Context, identifier string, patchData []byte) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, name := p.parseIdentifier(identifier)

	p.logger.Info("patching VM", "name", name, "namespace", namespace)

	_, err := p.virtClient.VirtualMachine(namespace).Patch(ctx, name, types.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch VM: %w", err)
	}

	p.logger.Info("VM patched successfully", "name", name)

	return nil
}
