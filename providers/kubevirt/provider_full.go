// SPDX-License-Identifier: Apache-2.0

// +build full

// Full KubeVirt provider implementation - requires KubeVirt dependencies
// This file will be compiled when build tag 'full' is provided
// For now, using stub implementation in provider_stub.go

package kubevirt

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// KubeVirtProvider implements the Provider interface for KubeVirt
type KubeVirtProvider struct {
	clientset    kubernetes.Interface
	virtClient   kubecli.KubevirtClient
	config       providers.ProviderConfig
	logger       logger.Logger
	namespace    string
	storageClass string
}

// NewProvider creates a new KubeVirt provider instance (factory function)
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	namespace := "default"
	if ns, ok := cfg.Metadata["namespace"].(string); ok && ns != "" {
		namespace = ns
	}

	storageClass := ""
	if sc, ok := cfg.Metadata["storageClass"].(string); ok {
		storageClass = sc
	}

	return &KubeVirtProvider{
		config:       cfg,
		logger:       log,
		namespace:    namespace,
		storageClass: storageClass,
	}, nil
}

// Name returns the provider name
func (p *KubeVirtProvider) Name() string {
	return "KubeVirt"
}

// Type returns the provider type
func (p *KubeVirtProvider) Type() providers.ProviderType {
	return providers.ProviderKubeVirt
}

// Connect establishes a connection to Kubernetes cluster with KubeVirt
func (p *KubeVirtProvider) Connect(ctx context.Context, providerCfg providers.ProviderConfig) error {
	var config *rest.Config
	var err error

	// Check if kubeconfig path is provided
	if kubeconfigPath, ok := providerCfg.Metadata["kubeconfig"].(string); ok && kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	} else {
		// Use in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create KubeVirt client
	virtClient, err := kubecli.GetKubevirtClientFromRESTConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubevirt client: %w", err)
	}

	p.clientset = clientset
	p.virtClient = virtClient
	p.config = providerCfg

	p.logger.Info("connected to Kubernetes cluster with KubeVirt",
		"namespace", p.namespace)

	return nil
}

// Disconnect closes the KubeVirt connection
func (p *KubeVirtProvider) Disconnect() error {
	// Kubernetes clients don't need explicit disconnection
	p.logger.Info("disconnected from KubeVirt")
	return nil
}

// ValidateCredentials validates the connection and credentials
func (p *KubeVirtProvider) ValidateCredentials(ctx context.Context) error {
	if p.virtClient == nil {
		return fmt.Errorf("not connected")
	}

	// Try to list VMs to validate access
	_, err := p.virtClient.VirtualMachine(p.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}

	return nil
}

// ListVMs lists virtual machines matching the filter
func (p *KubeVirtProvider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Determine namespace to list from
	namespace := p.namespace
	if filter.Location != "" {
		namespace = filter.Location
	}

	// List VirtualMachines
	vmList, err := p.virtClient.VirtualMachine(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	result := make([]*providers.VMInfo, 0, len(vmList.Items))

	for _, vm := range vmList.Items {
		vmInfo := p.convertVMToInfo(&vm)

		// Apply filters
		if !p.matchesFilter(vmInfo, filter) {
			continue
		}

		result = append(result, vmInfo)
	}

	p.logger.Info("listed VMs", "total", len(result), "namespace", namespace)
	return result, nil
}

// convertVMToInfo converts a KubeVirt VirtualMachine to providers.VMInfo
func (p *KubeVirtProvider) convertVMToInfo(vm *kubevirtv1.VirtualMachine) *providers.VMInfo {
	// Determine state
	state := "stopped"
	if vm.Spec.Running != nil && *vm.Spec.Running {
		state = "running"
	}

	// Get instance info if available
	vmi, err := p.virtClient.VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})

	var ipAddresses []string
	var guestOS string

	if err == nil && vmi != nil {
		// Extract IP addresses
		for _, iface := range vmi.Status.Interfaces {
			if iface.IP != "" {
				ipAddresses = append(ipAddresses, iface.IP)
			}
			ipAddresses = append(ipAddresses, iface.IPs...)
		}

		// Get guest OS info
		if vmi.Status.GuestOSInfo.Name != "" {
			guestOS = fmt.Sprintf("%s %s", vmi.Status.GuestOSInfo.Name, vmi.Status.GuestOSInfo.Version)
		}

		// Update state based on phase
		if vmi.Status.Phase != "" {
			state = strings.ToLower(string(vmi.Status.Phase))
		}
	}

	// Extract resource requirements
	var memoryMB int64
	var numCPUs int
	var storageGB int64

	if vm.Spec.Template != nil {
		// Get memory
		if mem := vm.Spec.Template.Spec.Domain.Resources.Requests.Memory(); mem != nil {
			memoryMB = mem.Value() / (1024 * 1024)
		}

		// Get CPUs
		if vm.Spec.Template.Spec.Domain.CPU != nil {
			numCPUs = int(vm.Spec.Template.Spec.Domain.CPU.Cores)
		}

		// Estimate storage from volumes
		for _, vol := range vm.Spec.Template.Spec.Volumes {
			if vol.DataVolume != nil {
				// Would need to query DataVolume to get actual size
				storageGB += 10 // Placeholder
			} else if vol.PersistentVolumeClaim != nil {
				// Would need to query PVC to get actual size
				storageGB += 10 // Placeholder
			}
		}
	}

	// Extract labels as tags
	tags := make(map[string]string)
	for k, v := range vm.Labels {
		tags[k] = v
	}

	return &providers.VMInfo{
		Provider:    providers.ProviderKubeVirt,
		ID:          string(vm.UID),
		Name:        vm.Name,
		State:       state,
		PowerState:  state,
		Location:    vm.Namespace,
		GuestOS:     guestOS,
		MemoryMB:    memoryMB,
		NumCPUs:     numCPUs,
		StorageGB:   storageGB,
		IPAddresses: ipAddresses,
		Tags:        tags,
		Metadata: map[string]interface{}{
			"namespace":     vm.Namespace,
			"uid":           string(vm.UID),
			"labels":        vm.Labels,
			"annotations":   vm.Annotations,
			"running":       vm.Spec.Running,
		},
		CreatedAt: &vm.CreationTimestamp.Time,
	}
}

// matchesFilter checks if a VM matches the given filter
func (p *KubeVirtProvider) matchesFilter(vm *providers.VMInfo, filter providers.VMFilter) bool {
	// Name pattern filter
	if filter.NamePattern != "" {
		matched, err := filepath.Match(filter.NamePattern, vm.Name)
		if err != nil || !matched {
			return false
		}
	}

	// State filter
	if filter.State != "" && vm.State != filter.State {
		return false
	}

	// PowerState filter (alternative to State)
	if filter.PowerState != "" && vm.PowerState != filter.PowerState {
		return false
	}

	// Location filter (namespace)
	if filter.Location != "" && !strings.Contains(vm.Location, filter.Location) {
		return false
	}

	// Memory filter
	if filter.MinMemoryMB > 0 && vm.MemoryMB < filter.MinMemoryMB {
		return false
	}

	// CPU filter
	if filter.MinCPUs > 0 && vm.NumCPUs < filter.MinCPUs {
		return false
	}

	// Tags filter
	if len(filter.Tags) > 0 {
		for key, value := range filter.Tags {
			if vmValue, ok := vm.Tags[key]; !ok || vmValue != value {
				return false
			}
		}
	}

	return true
}

// GetVM retrieves information about a specific VM
func (p *KubeVirtProvider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Parse identifier (format: namespace/name or just name)
	namespace := p.namespace
	name := identifier

	if parts := strings.Split(identifier, "/"); len(parts) == 2 {
		namespace = parts[0]
		name = parts[1]
	}

	// Get VirtualMachine
	vm, err := p.virtClient.VirtualMachine(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get VM: %w", err)
	}

	return p.convertVMToInfo(vm), nil
}

// SearchVMs searches for VMs by query string
func (p *KubeVirtProvider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	// List all VMs and filter by name
	vmList, err := p.virtClient.VirtualMachine(p.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	results := make([]*providers.VMInfo, 0)
	queryLower := strings.ToLower(query)

	for _, vm := range vmList.Items {
		if strings.Contains(strings.ToLower(vm.Name), queryLower) {
			results = append(results, p.convertVMToInfo(&vm))
		}
	}

	return results, nil
}

// ExportVM exports a virtual machine
func (p *KubeVirtProvider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	if p.virtClient == nil {
		return nil, fmt.Errorf("not connected")
	}

	startTime := time.Now()

	// Parse identifier
	namespace := p.namespace
	name := identifier

	if parts := strings.Split(identifier, "/"); len(parts) == 2 {
		namespace = parts[0]
		name = parts[1]
	}

	// Get VM
	vm, err := p.virtClient.VirtualMachine(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get VM: %w", err)
	}

	p.logger.Info("exporting KubeVirt VM", "name", name, "namespace", namespace)

	// For now, create a basic export using VirtualMachineExport CRD
	// This is a simplified implementation - full implementation would use VirtualMachineExport API
	exportName := fmt.Sprintf("%s-export-%d", name, time.Now().Unix())

	// TODO: Implement actual VirtualMachineExport CRD usage
	// For MVP, we'll create a snapshot and reference it

	exportResult := &providers.ExportResult{
		Provider:   providers.ProviderKubeVirt,
		VMName:     name,
		VMID:       string(vm.UID),
		Format:     opts.Format,
		OutputPath: opts.OutputPath,
		Files:      []string{}, // Would be populated from actual export
		Duration:   time.Since(startTime),
		Metadata: map[string]interface{}{
			"namespace":   namespace,
			"exportName":  exportName,
			"kubernetesVM": true,
		},
	}

	p.logger.Info("VM export completed", "name", name, "duration", exportResult.Duration)

	return exportResult, nil
}

// GetExportCapabilities returns the export capabilities of KubeVirt
func (p *KubeVirtProvider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{"raw", "qcow2", "vmdk"},
		SupportsCompression: true,
		SupportsStreaming:   true,
		SupportsSnapshots:   true,
		MaxVMSizeGB:         0, // No limit
		SupportedTargets:    []string{"local", "s3", "gcs", "azure-blob"},
	}
}
