// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zyvorai/transiva/providers"
	"github.com/zyvorai/transiva/providers/common"
)

// ExportVM converts Nutanix VM disks from mounted storage containers and writes
// a h2kvm artifact manifest. Requires NFS mounts of storage containers.
func (p *Provider) ExportVM(ctx context.Context, identifier string, opts providers.ExportOptions) (*providers.ExportResult, error) {
	nxOpts, err := mergeExportOptions(p, opts)
	if err != nil {
		return nil, err
	}

	vmInfo, err := p.resolveVMForExport(ctx, identifier)
	if err != nil {
		return nil, err
	}

	containerNames := map[string]string{}
	if nxOpts.ResolveContainers {
		containerNames, err = p.ResolveContainerNames(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve container names: %w", err)
		}
	}

	prismHost := firstNonEmpty(p.cfg.Host, p.cfg.Endpoint)
	pickupVM := inventoryToPickupVM(InventoryFromVMInfo(vmInfo), containerNames, prismHost)
	if len(pickupVM.Disks) == 0 {
		return nil, fmt.Errorf("VM %q has no exportable disks", vmInfo.Name)
	}

	pickupOpts := PickupExecuteOptions{
		OutputDir: nxOpts.OutputDir,
		Format:    nxOpts.Format,
		Mounts:    nxOpts.Mounts,
		DryRun:    nxOpts.DryRun,
	}
	if opts.Metadata != nil {
		if v, ok := metaString(opts.Metadata, "qemu_img_path"); ok {
			pickupOpts.QemuImgPath = v
		}
	}
	if opts.Progress != nil {
		pickupOpts.Progress = func(p PickupProgress) {
			opts.Progress.Update(p.Phase, p.PercentComplete, p.Message)
		}
	}

	if p.logger != nil {
		p.logger.Info("exporting Nutanix VM via NFS pickup",
			"vm", vmInfo.Name,
			"uuid", vmInfo.ID,
			"output", nxOpts.OutputDir,
			"format", nxOpts.Format,
			"dry_run", nxOpts.DryRun)
	}

	pickupResult, err := ExecutePickupVM(ctx, pickupVM, pickupOpts)
	if err != nil {
		return nil, fmt.Errorf("nutanix pickup export: %w", err)
	}

	result := pickupResultToExportResult(pickupResult, nxOpts.Format)

	if nxOpts.EnablePipeline && pickupResult.ManifestPath != "" && !nxOpts.DryRun {
		p.runExportPipeline(ctx, nxOpts, result)
	}

	return result, nil
}

// GetExportCapabilities reports Nutanix offline NFS pickup export support.
func (p *Provider) GetExportCapabilities() providers.ExportCapabilities {
	return providers.ExportCapabilities{
		SupportedFormats: []string{"qcow2", "raw"},
		SupportedTargets: []string{"local"},
	}
}

func (p *Provider) resolveVMForExport(ctx context.Context, identifier string) (*providers.VMInfo, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("VM identifier is required")
	}

	vm, err := p.GetVM(ctx, identifier)
	if err == nil {
		return vm, nil
	}

	matches, searchErr := p.SearchVMs(ctx, identifier)
	if searchErr != nil {
		return nil, fmt.Errorf("lookup VM %q: %w", identifier, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("VM %q not found", identifier)
	}

	for _, match := range matches {
		if strings.EqualFold(match.Name, identifier) || match.ID == identifier {
			return p.GetVM(ctx, match.ID)
		}
	}
	if len(matches) == 1 {
		return p.GetVM(ctx, matches[0].ID)
	}

	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match.Name)
	}
	return nil, fmt.Errorf("VM %q matched %d VMs (%s); specify UUID",
		identifier, len(matches), strings.Join(names, ", "))
}

func inventoryToPickupVM(inv VMInventory, containerNames map[string]string, host string) PickupVM {
	plan := BuildPickupPlan(host, []VMInventory{inv}, containerNames)
	if len(plan.VMs) == 0 {
		return PickupVM{VMInventory: inv}
	}
	return plan.VMs[0]
}

func pickupResultToExportResult(pickup *PickupResult, format string) *providers.ExportResult {
	files := make([]string, 0, len(pickup.Disks)+1)
	var totalSize int64
	for _, disk := range pickup.Disks {
		if disk.OutputPath != "" {
			files = append(files, disk.OutputPath)
		}
		totalSize += disk.Bytes
	}
	if pickup.ManifestPath != "" {
		files = append(files, pickup.ManifestPath)
	}

	outputPath := filepath.Join(pickupOptsOutputDir(pickup), sanitizeFilename(pickup.VMName))
	if pickup.ManifestPath != "" {
		outputPath = pickup.ManifestPath
	}

	metadata := map[string]interface{}{
		"vm_uuid":       pickup.VMUUID,
		"disk_count":    len(pickup.Disks),
		"pickup_method": "nutanix-nfs",
	}
	if pickup.ManifestPath != "" {
		metadata["manifest_path"] = pickup.ManifestPath
	}

	return &providers.ExportResult{
		Provider:   providers.ProviderNutanix,
		VMName:     pickup.VMName,
		VMID:       pickup.VMUUID,
		Format:     format,
		OutputPath: outputPath,
		Files:      files,
		Size:       totalSize,
		Duration:   pickup.Duration,
		Metadata:   metadata,
	}
}

func pickupOptsOutputDir(pickup *PickupResult) string {
	if len(pickup.Disks) == 0 {
		return ""
	}
	return filepath.Dir(filepath.Dir(pickup.Disks[0].OutputPath))
}

func (p *Provider) runExportPipeline(ctx context.Context, nxOpts nutanixExportOptions, result *providers.ExportResult) {
	manifestPath, _ := result.Metadata["manifest_path"].(string)
	if manifestPath == "" {
		return
	}

	if p.logger != nil {
		p.logger.Info("starting h2kvm pipeline", "manifest", manifestPath)
	}

	pipelineConfig := nxOpts.Pipeline
	pipelineConfig.Enabled = true
	pipelineConfig.ManifestPath = manifestPath

	executor := common.NewPipelineExecutor(&pipelineConfig, p.logger)

	pipelineCtx := ctx
	if nxOpts.PipelineTimeout > 0 {
		var cancel context.CancelFunc
		pipelineCtx, cancel = context.WithTimeout(ctx, nxOpts.PipelineTimeout)
		defer cancel()
	}

	pipelineResult, err := executor.Execute(pipelineCtx)
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	if err != nil {
		if p.logger != nil {
			p.logger.Error("pipeline failed (non-fatal)", "error", err)
		}
		result.Metadata["pipeline_error"] = err.Error()
		result.Metadata["pipeline_success"] = false
		return
	}

	result.Metadata["pipeline_success"] = pipelineResult.Success
	result.Metadata["pipeline_duration"] = pipelineResult.Duration.String()
	if pipelineResult.OutputPath != "" {
		result.Metadata["converted_path"] = pipelineResult.OutputPath
	}
	if pipelineResult.LibvirtDomain != "" {
		result.Metadata["libvirt_domain"] = pipelineResult.LibvirtDomain
	}
}
