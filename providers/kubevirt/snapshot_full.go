// SPDX-License-Identifier: Apache-2.0

// +build full

package kubevirt

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
)

// SnapshotInfo represents information about a VM snapshot
type SnapshotInfo struct {
	Name        string
	Namespace   string
	VMName      string
	CreatedAt   time.Time
	ReadyToUse  bool
	Phase       string
	Indications []string
	Error       string
}

// CreateSnapshot creates a snapshot of a virtual machine
func (p *KubeVirtProvider) CreateSnapshot(ctx context.Context, identifier, snapshotName string) (*SnapshotInfo, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	p.logger.Info("creating snapshot", "vm", vmName, "snapshot", snapshotName, "namespace", namespace)

	// Create VirtualMachineSnapshot
	snapshot := &snapshotv1.VirtualMachineSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: namespace,
		},
		Spec: snapshotv1.VirtualMachineSnapshotSpec{
			Source: corev1.TypedLocalObjectReference{
				APIGroup: &snapshotv1.SchemeGroupVersion.Group,
				Kind:     "VirtualMachine",
				Name:     vmName,
			},
		},
	}

	createdSnapshot, err := p.virtClient.VirtualMachineSnapshot(namespace).Create(ctx, snapshot, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	p.logger.Info("snapshot created", "name", snapshotName, "vm", vmName)

	return &SnapshotInfo{
		Name:       createdSnapshot.Name,
		Namespace:  createdSnapshot.Namespace,
		VMName:     vmName,
		CreatedAt:  createdSnapshot.CreationTimestamp.Time,
		ReadyToUse: createdSnapshot.Status != nil && createdSnapshot.Status.ReadyToUse != nil && *createdSnapshot.Status.ReadyToUse,
	}, nil
}

// ListSnapshots lists all snapshots for a VM
func (p *KubeVirtProvider) ListSnapshots(ctx context.Context, identifier string) ([]*SnapshotInfo, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	p.logger.Info("listing snapshots", "vm", vmName, "namespace", namespace)

	// List all snapshots in namespace
	snapshotList, err := p.virtClient.VirtualMachineSnapshot(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	result := make([]*SnapshotInfo, 0)

	for _, snapshot := range snapshotList.Items {
		// Filter by VM name
		if snapshot.Spec.Source.Name != vmName {
			continue
		}

		info := &SnapshotInfo{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
			VMName:    vmName,
			CreatedAt: snapshot.CreationTimestamp.Time,
		}

		if snapshot.Status != nil {
			if snapshot.Status.ReadyToUse != nil {
				info.ReadyToUse = *snapshot.Status.ReadyToUse
			}
			if snapshot.Status.Phase != "" {
				info.Phase = string(snapshot.Status.Phase)
			}
			if len(snapshot.Status.Indications) > 0 {
				for _, indication := range snapshot.Status.Indications {
					info.Indications = append(info.Indications, string(indication))
				}
			}
			if snapshot.Status.Error != nil {
				info.Error = snapshot.Status.Error.Message
			}
		}

		result = append(result, info)
	}

	p.logger.Info("listed snapshots", "count", len(result), "vm", vmName)

	return result, nil
}

// GetSnapshot retrieves information about a specific snapshot
func (p *KubeVirtProvider) GetSnapshot(ctx context.Context, identifier, snapshotName string) (*SnapshotInfo, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	snapshot, err := p.virtClient.VirtualMachineSnapshot(namespace).Get(ctx, snapshotName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	info := &SnapshotInfo{
		Name:      snapshot.Name,
		Namespace: snapshot.Namespace,
		VMName:    vmName,
		CreatedAt: snapshot.CreationTimestamp.Time,
	}

	if snapshot.Status != nil {
		if snapshot.Status.ReadyToUse != nil {
			info.ReadyToUse = *snapshot.Status.ReadyToUse
		}
		if snapshot.Status.Phase != "" {
			info.Phase = string(snapshot.Status.Phase)
		}
		if len(snapshot.Status.Indications) > 0 {
			for _, indication := range snapshot.Status.Indications {
				info.Indications = append(info.Indications, string(indication))
			}
		}
		if snapshot.Status.Error != nil {
			info.Error = snapshot.Status.Error.Message
		}
	}

	return info, nil
}

// RestoreSnapshot restores a VM from a snapshot
func (p *KubeVirtProvider) RestoreSnapshot(ctx context.Context, identifier, snapshotName string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	p.logger.Info("restoring snapshot", "vm", vmName, "snapshot", snapshotName, "namespace", namespace)

	// Check if snapshot exists and is ready
	snapshot, err := p.virtClient.VirtualMachineSnapshot(namespace).Get(ctx, snapshotName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	if snapshot.Status == nil || snapshot.Status.ReadyToUse == nil || !*snapshot.Status.ReadyToUse {
		return fmt.Errorf("snapshot is not ready to use")
	}

	// Create VirtualMachineRestore
	restore := &snapshotv1.VirtualMachineRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-restore-%d", vmName, time.Now().Unix()),
			Namespace: namespace,
		},
		Spec: snapshotv1.VirtualMachineRestoreSpec{
			Target: corev1.TypedLocalObjectReference{
				APIGroup: &snapshotv1.SchemeGroupVersion.Group,
				Kind:     "VirtualMachine",
				Name:     vmName,
			},
			VirtualMachineSnapshotName: snapshotName,
		},
	}

	_, err = p.virtClient.VirtualMachineRestore(namespace).Create(ctx, restore, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create restore: %w", err)
	}

	p.logger.Info("snapshot restore initiated", "vm", vmName, "snapshot", snapshotName)

	return nil
}

// DeleteSnapshot deletes a snapshot
func (p *KubeVirtProvider) DeleteSnapshot(ctx context.Context, identifier, snapshotName string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	p.logger.Info("deleting snapshot", "vm", vmName, "snapshot", snapshotName, "namespace", namespace)

	err := p.virtClient.VirtualMachineSnapshot(namespace).Delete(ctx, snapshotName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	p.logger.Info("snapshot deleted", "name", snapshotName, "vm", vmName)

	return nil
}

// WaitForSnapshotReady waits for a snapshot to become ready
func (p *KubeVirtProvider) WaitForSnapshotReady(ctx context.Context, identifier, snapshotName string, timeout time.Duration) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	p.logger.Info("waiting for snapshot to be ready", "snapshot", snapshotName, "timeout", timeout)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for snapshot to be ready")
		case <-ticker.C:
			info, err := p.GetSnapshot(ctx, fmt.Sprintf("%s/%s", namespace, vmName), snapshotName)
			if err != nil {
				p.logger.Warn("failed to get snapshot status", "error", err)
				continue
			}

			if info.ReadyToUse {
				p.logger.Info("snapshot is ready", "snapshot", snapshotName)
				return nil
			}

			if info.Error != "" {
				return fmt.Errorf("snapshot error: %s", info.Error)
			}

			p.logger.Debug("snapshot not ready yet", "phase", info.Phase)
		}
	}
}

// ExportSnapshot exports a snapshot to a DataVolume
func (p *KubeVirtProvider) ExportSnapshot(ctx context.Context, identifier, snapshotName, exportName string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	p.logger.Info("exporting snapshot", "vm", vmName, "snapshot", snapshotName, "export", exportName)

	// Get snapshot to verify it exists
	snapshot, err := p.virtClient.VirtualMachineSnapshot(namespace).Get(ctx, snapshotName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	if snapshot.Status == nil || snapshot.Status.ReadyToUse == nil || !*snapshot.Status.ReadyToUse {
		return fmt.Errorf("snapshot is not ready")
	}

	// Create a VirtualMachineExport for the snapshot
	export := &snapshotv1.VirtualMachineExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      exportName,
			Namespace: namespace,
		},
		Spec: snapshotv1.VirtualMachineExportSpec{
			Source: corev1.TypedLocalObjectReference{
				APIGroup: &snapshotv1.SchemeGroupVersion.Group,
				Kind:     "VirtualMachineSnapshot",
				Name:     snapshotName,
			},
		},
	}

	_, err = p.virtClient.VirtualMachineExport(namespace).Create(ctx, export, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create export: %w", err)
	}

	p.logger.Info("snapshot export created", "export", exportName)

	return nil
}

// CloneFromSnapshot creates a new VM from a snapshot
func (p *KubeVirtProvider) CloneFromSnapshot(ctx context.Context, identifier, snapshotName, newVMName string) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	p.logger.Info("cloning VM from snapshot", "source", vmName, "snapshot", snapshotName, "new", newVMName)

	// Get snapshot
	snapshot, err := p.virtClient.VirtualMachineSnapshot(namespace).Get(ctx, snapshotName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	if snapshot.Status == nil || snapshot.Status.ReadyToUse == nil || !*snapshot.Status.ReadyToUse {
		return fmt.Errorf("snapshot is not ready")
	}

	// Get source VM to use as template
	sourceVM, err := p.virtClient.VirtualMachine(namespace).Get(ctx, vmName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get source VM: %w", err)
	}

	// Create new VM based on source
	newVM := sourceVM.DeepCopy()
	newVM.ObjectMeta = metav1.ObjectMeta{
		Name:      newVMName,
		Namespace: namespace,
		Labels:    sourceVM.Labels,
	}
	newVM.ResourceVersion = ""
	newVM.UID = ""
	newVM.Status = nil

	// Set to not running initially
	running := false
	newVM.Spec.Running = &running

	// TODO: Clone the DataVolumes from snapshot
	// This would require:
	// 1. Getting snapshot content
	// 2. Creating new DataVolumes from snapshot PVCs
	// 3. Updating volume references in new VM

	_, err = p.virtClient.VirtualMachine(namespace).Create(ctx, newVM, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create new VM: %w", err)
	}

	p.logger.Info("VM cloned from snapshot", "source", vmName, "new", newVMName)

	return nil
}

// GetSnapshotContent retrieves the content information of a snapshot
func (p *KubeVirtProvider) GetSnapshotContent(ctx context.Context, identifier, snapshotName string) (map[string]interface{}, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	namespace, vmName := p.parseIdentifier(identifier)

	snapshot, err := p.virtClient.VirtualMachineSnapshot(namespace).Get(ctx, snapshotName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	content := map[string]interface{}{
		"name":      snapshot.Name,
		"namespace": snapshot.Namespace,
		"vmName":    vmName,
		"created":   snapshot.CreationTimestamp.Time,
	}

	if snapshot.Status != nil {
		content["phase"] = snapshot.Status.Phase
		content["readyToUse"] = snapshot.Status.ReadyToUse
		content["creationTime"] = snapshot.Status.CreationTime

		if snapshot.Status.SnapshotVolumes != nil {
			volumes := make([]map[string]interface{}, 0)
			for _, vol := range snapshot.Status.SnapshotVolumes.IncludedVolumes {
				volumes = append(volumes, map[string]interface{}{
					"name": vol,
				})
			}
			content["volumes"] = volumes
		}

		if snapshot.Status.VirtualMachineSnapshotContentName != nil {
			content["contentName"] = *snapshot.Status.VirtualMachineSnapshotContentName
		}
	}

	return content, nil
}
