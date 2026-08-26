// SPDX-License-Identifier: Apache-2.0

package proxmox

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// Provider implements the unified Provider interface for Proxmox VE
type Provider struct {
	client *Client
	logger logger.Logger
	config providers.ProviderConfig
}

// NewProvider creates a new Proxmox provider instance
func NewProvider(cfg providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
	// Convert generic config to Proxmox-specific config
	proxmoxCfg := &config.ProxmoxConfig{
		Host:      cfg.Host,
		Port:      cfg.Port,
		Username:  cfg.Username,
		Password:  cfg.Password,
		Node:      cfg.Region, // Reuse Region field for default node
		VerifySSL: !cfg.Insecure,
	}

	// Set defaults
	if proxmoxCfg.Port == 0 {
		proxmoxCfg.Port = 8006
	}

	// Create client
	client, err := NewClient(proxmoxCfg, log)
	if err != nil {
		return nil, fmt.Errorf("create Proxmox client: %w", err)
	}

	return &Provider{
		client: client,
		logger: log,
		config: cfg,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "Proxmox VE"
}

// Type returns the provider type
func (p *Provider) Type() providers.ProviderType {
	return providers.ProviderProxmox
}

// Connect establishes connection to Proxmox (already done in NewProvider)
func (p *Provider) Connect(ctx context.Context, config providers.ProviderConfig) error {
	// Connection is established in NewProvider/NewClient
	// This is a no-op for Proxmox since authentication happens during client creation
	return nil
}

// Disconnect closes the connection
func (p *Provider) Disconnect() error {
	return p.client.Close()
}

// ValidateCredentials verifies that credentials are valid
func (p *Provider) ValidateCredentials(ctx context.Context) error {
	// Try to list nodes as a credentials validation
	_, err := p.client.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}
	return nil
}

// ListVMs lists all VMs across all nodes
func (p *Provider) ListVMs(ctx context.Context, filter providers.VMFilter) ([]*providers.VMInfo, error) {
	// Get list of nodes
	nodes, err := p.client.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var allVMs []*providers.VMInfo

	// List VMs on each node
	for _, node := range nodes {
		vms, err := p.client.ListVMs(ctx, node.Node)
		if err != nil {
			p.logger.Warn("failed to list VMs on node", "node", node.Node, "error", err)
			continue
		}

		for _, vm := range vms {
			vmInfo := &providers.VMInfo{
				Provider: providers.ProviderProxmox,
				ID:       fmt.Sprintf("%s:%d", node.Node, vm.VMID),
				Name:     vm.Name,
				State:    vm.Status,
				Location: node.Node,
				Tags: map[string]string{
					"node": node.Node,
					"vmid": strconv.Itoa(vm.VMID),
				},
				Metadata: map[string]interface{}{
					"vmid":      vm.VMID,
					"node":      node.Node,
					"cpus":      vm.CPUs,
					"memory":    vm.Memory,
					"maxmemory": vm.MaxMemory,
					"disk":      vm.Disk,
					"maxdisk":   vm.MaxDisk,
					"uptime":    vm.Uptime,
				},
			}

			// Apply filters
			if filter.State != "" && vmInfo.State != filter.State {
				continue
			}

			if filter.Location != "" && vmInfo.Location != filter.Location {
				continue
			}

			if len(filter.Tags) > 0 {
				match := true
				for key, value := range filter.Tags {
					if vmInfo.Tags[key] != value {
						match = false
						break
					}
				}
				if !match {
					continue
				}
			}

			allVMs = append(allVMs, vmInfo)
		}
	}

	p.logger.Info("listed VMs across all nodes", "total", len(allVMs))

	return allVMs, nil
}

// GetVM retrieves information about a specific VM
func (p *Provider) GetVM(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	// Parse identifier (format: "node:vmid" or just "vmid")
	node, vmid, err := parseVMIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	// If node not specified, search all nodes
	if node == "" {
		return p.searchVM(ctx, vmid)
	}

	// Get VM from specific node
	vm, err := p.client.GetVM(ctx, node, vmid)
	if err != nil {
		return nil, fmt.Errorf("get VM: %w", err)
	}

	vmInfo := &providers.VMInfo{
		Provider: providers.ProviderProxmox,
		ID:       fmt.Sprintf("%s:%d", node, vm.VMID),
		Name:     vm.Name,
		State:    vm.Status,
		Location: node,
		Tags: map[string]string{
			"node": node,
			"vmid": strconv.Itoa(vm.VMID),
		},
		Metadata: map[string]interface{}{
			"vmid":      vm.VMID,
			"node":      node,
			"cpus":      vm.CPUs,
			"memory":    vm.Memory,
			"maxmemory": vm.MaxMemory,
			"disk":      vm.Disk,
			"maxdisk":   vm.MaxDisk,
			"uptime":    vm.Uptime,
		},
	}

	return vmInfo, nil
}

// searchVM searches for a VM by VMID across all nodes
func (p *Provider) searchVM(ctx context.Context, vmid int) (*providers.VMInfo, error) {
	nodes, err := p.client.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	for _, node := range nodes {
		vm, err := p.client.GetVM(ctx, node.Node, vmid)
		if err == nil {
			return &providers.VMInfo{
				Provider: providers.ProviderProxmox,
				ID:       fmt.Sprintf("%s:%d", node.Node, vm.VMID),
				Name:     vm.Name,
				State:    vm.Status,
				Location: node.Node,
				Tags: map[string]string{
					"node": node.Node,
					"vmid": strconv.Itoa(vm.VMID),
				},
				Metadata: map[string]interface{}{
					"vmid":   vm.VMID,
					"node":   node.Node,
					"cpus":   vm.CPUs,
					"memory": vm.Memory,
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("VM with VMID %d not found on any node", vmid)
}

// SearchVMs searches for VMs by name or other criteria
func (p *Provider) SearchVMs(ctx context.Context, query string) ([]*providers.VMInfo, error) {
	// Get all VMs
	allVMs, err := p.ListVMs(ctx, providers.VMFilter{})
	if err != nil {
		return nil, err
	}

	// Filter by query (match name or ID)
	var results []*providers.VMInfo
	queryLower := strings.ToLower(query)

	for _, vm := range allVMs {
		if strings.Contains(strings.ToLower(vm.Name), queryLower) ||
			strings.Contains(strings.ToLower(vm.ID), queryLower) {
			results = append(results, vm)
		}
	}

	return results, nil
}

// ExportVM exports a VM to a local file
func (p *Provider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	// Parse identifier
	node, vmid, err := parseVMIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	// If node not specified, find it
	if node == "" {
		vmInfo, err := p.searchVM(ctx, vmid)
		if err != nil {
			return nil, err
		}
		node = vmInfo.Location
	}

	p.logger.Info("exporting Proxmox VM",
		"node", node,
		"vmid", vmid,
		"output", opts.OutputPath)

	// Prepare export options
	exportOpts := ExportOptions{
		Node:       node,
		VMID:       vmid,
		OutputPath: opts.OutputPath,
		BackupMode: "snapshot", // Default to snapshot mode
	}

	// Map compression settings
	if opts.Compress {
		compressionLevel := opts.CompressionLevel
		if compressionLevel == 0 {
			compressionLevel = 6
		}

		// Map compression level to Proxmox compression type
		switch {
		case compressionLevel >= 7:
			exportOpts.Compress = "zstd" // Best compression
		case compressionLevel >= 4:
			exportOpts.Compress = "gzip"
		default:
			exportOpts.Compress = "lzo" // Fastest
		}
	}

	// Perform export
	result, err := p.client.ExportVM(ctx, exportOpts)
	if err != nil {
		return nil, fmt.Errorf("export VM: %w", err)
	}

	// Convert to generic result
	return &providers.ExportResult{
		Provider:   providers.ProviderProxmox,
		VMName:     fmt.Sprintf("%d", vmid),
		Format:     result.Format,
		OutputPath: result.BackupFile,
		Size:       result.Size,
		Checksum:   "", // Proxmox doesn't provide checksum automatically
		Metadata: map[string]interface{}{
			"backup_id": result.BackupID,
			"duration":  result.Duration.String(),
			"node":      node,
			"vmid":      vmid,
		},
	}, nil
}

// GetExportCapabilities returns export capabilities for Proxmox
func (p *Provider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats:    []string{"vzdump", "vma"},
		SupportsCompression: true,
		SupportsStreaming:   false, // Proxmox requires backup creation first
	}
}

// parseVMIdentifier parses VM identifier in format "node:vmid" or just "vmid"
func parseVMIdentifier(identifier string) (node string, vmid int, err error) {
	parts := strings.Split(identifier, ":")

	if len(parts) == 1 {
		// Just VMID
		vmid, err = strconv.Atoi(parts[0])
		if err != nil {
			return "", 0, fmt.Errorf("invalid VMID: %s", parts[0])
		}
		return "", vmid, nil
	}

	if len(parts) == 2 {
		// Node:VMID format
		node = parts[0]
		vmid, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, fmt.Errorf("invalid VMID: %s", parts[1])
		}
		return node, vmid, nil
	}

	return "", 0, fmt.Errorf("invalid identifier format: %s (expected 'vmid' or 'node:vmid')", identifier)
}
