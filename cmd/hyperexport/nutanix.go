// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/api"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
	"github.com/zyvorai/transiva/providers/nutanix"
)

var (
	nutanixMounts           = flag.String("mounts", "", "Nutanix container NFS mounts (name:/path,name2:/path2)")
	nutanixResolveContainers = flag.Bool("resolve-containers", false, "Resolve Nutanix storage container names via Prism API")
	nutanixCluster          = flag.String("cluster", "", "Filter Nutanix VMs by cluster name or UUID")
	nutanixInsecure         = flag.Bool("insecure", true, "Skip TLS verification for Nutanix Prism")
)

func runNutanixExport(ctx context.Context, cfg *config.Config, log logger.Logger) error {
	if *vmName == "" {
		return fmt.Errorf("VM name or UUID required (-vm flag)")
	}
	return exportSingleNutanixVM(ctx, cfg, log, *vmName)
}

func exportSingleNutanixVM(ctx context.Context, cfg *config.Config, log logger.Logger, vm string) error {
	if *useDaemon {
		outputPath := getNutanixOutputDirForVM(vm)
		return runNutanixDaemonExport(ctx, vm, outputPath, *format, *dryRun, *quiet, *daemonURL, log)
	}

	nxCfg := cfg.Nutanix
	if nxCfg == nil {
		nxCfg = &config.NutanixConfig{}
	}

	host := nxCfg.Host
	user := nxCfg.Username
	pass := nxCfg.Password
	if host == "" || pass == "" {
		return fmt.Errorf("nutanix credentials required (set NUTANIX_HOST/NUTANIX_USERNAME/NUTANIX_PASSWORD or config.yaml nutanix section)")
	}

	outputPath := *outputDir
	if outputPath == "" {
		outputPath = nxCfg.OutputDir
	}
	if outputPath == "" {
		outputPath = getNutanixOutputDirForVM(vm)
	}

	formatVal := *format
	if formatVal == "" || formatVal == "ovf" || formatVal == "ova" {
		if nxCfg.ExportFormat != "" {
			formatVal = nxCfg.ExportFormat
		} else {
			formatVal = "qcow2"
		}
	}

	mounts, err := resolveNutanixMounts(nxCfg.Mounts, *nutanixMounts)
	if err != nil {
		return err
	}

	resolveContainers := *nutanixResolveContainers || nxCfg.ResolveContainers
	enablePipeline := *enablePipeline || nxCfg.EnablePipeline

	var spinner *pterm.SpinnerPrinter
	if !*quiet {
		spinner, _ = pterm.DefaultSpinner.Start("Connecting to Nutanix Prism...")
	}

	providerCfg := providers.ProviderConfig{
		Type:     providers.ProviderNutanix,
		Host:     host,
		Port:     nxCfg.Port,
		Username: user,
		Password: pass,
		Insecure: *nutanixInsecure || !nxCfg.VerifySSL,
		Metadata: map[string]interface{}{
			"detailed":            true,
			"cluster":             firstNonEmpty(*nutanixCluster, nxCfg.Cluster),
			"output_dir":          outputPath,
			"export_format":       formatVal,
			"mounts":              mounts,
			"resolve_containers":  resolveContainers,
			"enable_pipeline":     enablePipeline,
		},
	}
	if nxCfg.PipelineTimeout > 0 {
		providerCfg.Metadata["pipeline_timeout"] = nxCfg.PipelineTimeout
	}

	provider, err := nutanix.NewProvider(providerCfg, log)
	if err != nil {
		if spinner != nil {
			spinner.Fail("Failed to create Nutanix provider")
		}
		return fmt.Errorf("create nutanix provider: %w", err)
	}
	defer func() { _ = provider.Disconnect() }()

	if spinner != nil {
		spinner.Success("Connected to Nutanix Prism")
	}

	if *dryRun && !*quiet {
		pterm.Info.Println("Dry-run mode: export preview")
		pterm.DefaultSection.Println("Export Plan")
		fmt.Printf("  VM: %s\n", vm)
		fmt.Printf("  Format: %s\n", formatVal)
		fmt.Printf("  Output: %s\n", outputPath)
		fmt.Printf("  Mounts: %d container(s)\n", len(mounts))
		fmt.Printf("  Resolve containers: %v\n", resolveContainers)
		fmt.Printf("  Pipeline: %v\n", enablePipeline)
	}

	exportOpts := providers.ExportOptions{
		OutputPath: outputPath,
		Format:     formatVal,
		Metadata: map[string]interface{}{
			"mounts":              mounts,
			"dry_run":             *dryRun,
			"resolve_containers":  resolveContainers,
			"enable_pipeline":     enablePipeline,
		},
	}
	if nxCfg.PipelineTimeout > 0 {
		exportOpts.Metadata["pipeline_timeout"] = nxCfg.PipelineTimeout
	}
	applyHyperexportPipelineMetadata(exportOpts.Metadata)

	if !*quiet && !*dryRun {
		spinner, _ = pterm.DefaultSpinner.Start(fmt.Sprintf("Exporting VM %s...", vm))
	}

	result, err := provider.ExportVM(ctx, vm, exportOpts)
	if err != nil {
		if spinner != nil {
			spinner.Fail("Export failed")
		}
		return fmt.Errorf("export VM: %w", err)
	}

	if spinner != nil {
		spinner.Success(fmt.Sprintf("Exported %s", result.VMName))
	}

	if *quiet {
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
		return nil
	}

	pterm.DefaultSection.Println("Export Result")
	fmt.Printf("  VM:       %s (%s)\n", result.VMName, result.VMID)
	fmt.Printf("  Format:   %s\n", result.Format)
	fmt.Printf("  Output:   %s\n", result.OutputPath)
	fmt.Printf("  Size:     %s\n", formatBytes(result.Size))
	fmt.Printf("  Duration: %s\n", result.Duration.Round(time.Millisecond))
	if manifestPath, ok := result.Metadata["manifest_path"].(string); ok && manifestPath != "" {
		fmt.Printf("  Manifest: %s\n", manifestPath)
	}
	if pipelineOK, ok := result.Metadata["pipeline_success"].(bool); ok {
		fmt.Printf("  Pipeline: %v\n", pipelineOK)
	}

	return nil
}

func runNutanixDaemonExport(ctx context.Context, vmName, outputPath, format string, dryRun, quiet bool, daemonURL string, log logger.Logger) error {
	client := NewDaemonClient(daemonURL, log)
	var spinner *pterm.SpinnerPrinter

	if !quiet {
		spinner, _ = pterm.DefaultSpinner.Start("Checking daemon connectivity...")
		healthy, err := client.GetDaemonHealth(ctx)
		if err != nil {
			spinner.Fail(fmt.Sprintf("Failed to connect to daemon at %s", daemonURL))
			return err
		}
		if healthy {
			spinner.Success(fmt.Sprintf("Connected to daemon at %s", daemonURL))
		} else {
			spinner.Warning("Daemon is not healthy")
		}
	}

	req := api.VMExportRequest{
		VM:         vmName,
		OutputPath: outputPath,
		Format:     format,
		DryRun:     dryRun,
	}
	if *nutanixMounts != "" {
		req.Mounts = parseMountSpecToMap(*nutanixMounts)
	}
	req.ResolveContainers = *nutanixResolveContainers
	req.EnablePipeline = *enablePipeline
	if *nutanixCluster != "" {
		req.Cluster = *nutanixCluster
	}

	if !quiet {
		spinner, _ = pterm.DefaultSpinner.Start("Submitting Nutanix export to daemon...")
	}

	result, err := client.ExportNutanixVM(ctx, req)
	if err != nil {
		if !quiet {
			pterm.Error.Printfln("Export failed: %v", err)
		}
		return err
	}

	if !quiet {
		spinner.Success(fmt.Sprintf("Exported %s", result.VMName))
		pterm.DefaultSection.Println("Export Result")
		fmt.Printf("  VM:     %s (%s)\n", result.VMName, result.VMID)
		fmt.Printf("  Output: %s\n", result.OutputPath)
		fmt.Printf("  Format: %s\n", result.Format)
	} else {
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
	}

	return nil
}

func resolveNutanixMounts(configMounts map[string]string, flagSpec string) (nutanix.MountMap, error) {
	if flagSpec != "" {
		return nutanix.ParseMountMap(strings.Split(flagSpec, ","))
	}
	if len(configMounts) > 0 {
		return nutanix.MountMap(configMounts), nil
	}
	return nil, fmt.Errorf("container NFS mounts required (-mounts or nutanix.mounts in config)")
}

func getNutanixOutputDirForVM(vmName string) string {
	if *outputDir != "" {
		return *outputDir
	}
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_").Replace(strings.TrimSpace(vmName))
	if safe == "" {
		safe = "vm"
	}
	return filepath.Join(".", "export-"+safe)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseMountSpecToMap(spec string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 && kv[0] != "" && kv[1] != "" {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func applyHyperexportPipelineMetadata(meta map[string]interface{}) {
	if *enablePipeline {
		meta["enable_pipeline"] = true
	}
	if *pipelineTimeout > 0 {
		meta["pipeline_timeout"] = *pipelineTimeout
	}
	if *h2kvmPath != "" {
		meta["h2kvm_path"] = *h2kvmPath
	}
	if *libvirtIntegration {
		meta["libvirt_integration"] = true
	}
	if *libvirtURI != "" {
		meta["libvirt_uri"] = *libvirtURI
	}
	if *libvirtAutoStart {
		meta["libvirt_auto_start"] = true
	}
	if *pipelineDryRun {
		meta["pipeline_dry_run"] = true
	}
	if *h2kvmDaemon {
		meta["h2kvm_daemon"] = true
	}
	if *h2kvmInstance != "" {
		meta["h2kvm_instance"] = *h2kvmInstance
	}
	if *h2kvmWatchDir != "" {
		meta["h2kvm_watch_dir"] = *h2kvmWatchDir
	}
	if *h2kvmOutputDir != "" {
		meta["h2kvm_output_dir"] = *h2kvmOutputDir
	}
	if *h2kvmPollInterval > 0 {
		meta["h2kvm_poll_interval"] = *h2kvmPollInterval
	}
	if *h2kvmDaemonTimeout > 0 {
		meta["h2kvm_daemon_timeout"] = *h2kvmDaemonTimeout
	}
}

// ExportNutanixVM submits a Nutanix export request to the daemon.
func (c *DaemonClient) ExportNutanixVM(ctx context.Context, req api.VMExportRequest) (*providers.ExportResult, error) {
	// #nosec G117 -- req.Password is the credential payload for the daemon's
	// export API request body (sent, not logged); it must be marshaled as-is.
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal export request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/vms/export?provider=nutanix", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var exportResp struct {
		Result *providers.ExportResult `json:"result"`
	}
	if err := json.Unmarshal(body, &exportResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if exportResp.Result == nil {
		return nil, fmt.Errorf("empty export result from daemon")
	}
	return exportResp.Result, nil
}

func runNutanixBatchExport(ctx context.Context, cfg *config.Config, log logger.Logger) error {
	data, err := os.ReadFile(*batchFile)
	if err != nil {
		return fmt.Errorf("read batch file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var failures []string
	exported := 0

	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}

		if err := exportSingleNutanixVM(ctx, cfg, log, name); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			log.Error("batch export failed", "vm", name, "error", err)
			continue
		}
		exported++
	}

	if !*quiet {
		pterm.Success.Printfln("Batch export complete: %d succeeded, %d failed", exported, len(failures))
		for _, f := range failures {
			pterm.Error.Println(f)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("%d batch export(s) failed", len(failures))
	}
	return nil
}
