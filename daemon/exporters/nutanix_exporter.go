// SPDX-License-Identifier: Apache-2.0

package exporters

import (
	"context"
	"fmt"
	"strings"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
	"github.com/zyvorai/transiva/providers/nutanix"
)

// NutanixExporter exports VMs via NFS-mounted storage containers.
type NutanixExporter struct {
	registry  *providers.Registry
	appConfig *config.Config
	logger    logger.Logger
}

// NewNutanixExporter creates a Nutanix NFS pickup exporter.
func NewNutanixExporter(registry *providers.Registry, appConfig *config.Config, log logger.Logger) *NutanixExporter {
	return &NutanixExporter{
		registry:  registry,
		appConfig: appConfig,
		logger:    log,
	}
}

func (e *NutanixExporter) Export(ctx context.Context, job *models.JobDefinition, progressCallback func(*models.JobProgress)) (*models.JobResult, error) {
	if err := e.Validate(job); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if e.registry == nil || !e.registry.IsRegistered(providers.ProviderNutanix) {
		return nil, fmt.Errorf("nutanix provider is not registered")
	}

	if progressCallback != nil {
		progressCallback(&models.JobProgress{
			Phase:            "connecting",
			PercentComplete:  5,
			ExportMethod:     string(capabilities.ExportMethodNutanix),
		})
	}

	providerCfg := buildNutanixProviderConfig(job, e.appConfig)
	provider, err := e.registry.Create(providers.ProviderNutanix, providerCfg)
	if err != nil {
		return nil, fmt.Errorf("create nutanix provider: %w", err)
	}
	defer func() { _ = provider.Disconnect() }()

	exportOpts := buildNutanixExportOptions(job, e.appConfig)
	if progressCallback != nil {
		exportOpts.Progress = &exportProgressReporter{
			callback: progressCallback,
			method:   string(capabilities.ExportMethodNutanix),
		}
	}

	result, err := provider.ExportVM(ctx, job.VMPath, exportOpts)
	if err != nil {
		return nil, fmt.Errorf("nutanix export: %w", err)
	}

	if progressCallback != nil {
		progressCallback(&models.JobProgress{
			Phase:           "completed",
			PercentComplete: 100,
			ExportMethod:    string(capabilities.ExportMethodNutanix),
			CurrentStep:     "Export completed",
		})
	}

	jobResult := &models.JobResult{
		Success:      true,
		VMName:       result.VMName,
		OutputDir:    result.OutputPath,
		Files:        result.Files,
		OutputFiles:  result.Files,
		TotalSize:    result.Size,
		Duration:     result.Duration,
		ExportMethod: string(capabilities.ExportMethodNutanix),
	}

	e.logger.Info("nutanix export completed",
		"vm", result.VMName,
		"output", result.OutputPath,
		"size", result.Size)

	return jobResult, nil
}

func (e *NutanixExporter) Method() capabilities.ExportMethod {
	return capabilities.ExportMethodNutanix
}

func (e *NutanixExporter) Validate(job *models.JobDefinition) error {
	if job.VMPath == "" {
		return fmt.Errorf("vm_path (VM name or UUID) is required")
	}
	outputDir := job.OutputDir
	if outputDir == "" {
		outputDir = job.OutputPath
	}
	if outputDir == "" {
		return fmt.Errorf("output_dir is required")
	}
	if _, err := resolveJobMounts(job, e.appConfig); err != nil {
		return err
	}
	return nil
}

// IsNutanixJob reports whether a job targets Nutanix export.
func IsNutanixJob(job *models.JobDefinition) bool {
	if job == nil {
		return false
	}
	if strings.EqualFold(job.Provider, string(providers.ProviderNutanix)) {
		return true
	}
	if strings.EqualFold(job.ExportMethod, string(capabilities.ExportMethodNutanix)) ||
		strings.EqualFold(job.Method, string(capabilities.ExportMethodNutanix)) {
		return true
	}
	if job.Metadata != nil {
		if p, ok := job.Metadata["provider"].(string); ok && strings.EqualFold(p, string(providers.ProviderNutanix)) {
			return true
		}
	}
	return false
}

func buildNutanixProviderConfig(job *models.JobDefinition, appConfig *config.Config) providers.ProviderConfig {
	cfg := providers.ProviderConfig{
		Type: providers.ProviderNutanix,
		Metadata: map[string]interface{}{
			"detailed": true,
		},
	}

	if appConfig == nil {
		appConfig = config.FromEnvironment()
	}
	if appConfig.Nutanix != nil {
		nx := appConfig.Nutanix
		cfg.Host = nx.Host
		cfg.Port = nx.Port
		cfg.Username = nx.Username
		cfg.Password = nx.Password
		cfg.Insecure = !nx.VerifySSL
		if nx.PageSize > 0 {
			cfg.Metadata["page_size"] = nx.PageSize
		}
		if nx.DetailWorkers > 0 {
			cfg.Metadata["detail_workers"] = nx.DetailWorkers
		}
		if nx.OutputDir != "" {
			cfg.Metadata["output_dir"] = nx.OutputDir
		}
		if nx.ExportFormat != "" {
			cfg.Metadata["export_format"] = nx.ExportFormat
		}
		if len(nx.Mounts) > 0 {
			cfg.Metadata["mounts"] = nx.Mounts
		}
		cfg.Metadata["resolve_containers"] = nx.ResolveContainers
		cfg.Metadata["enable_pipeline"] = nx.EnablePipeline
		if nx.PipelineTimeout > 0 {
			cfg.Metadata["pipeline_timeout"] = nx.PipelineTimeout
		}
		if nx.Cluster != "" {
			cfg.Metadata["cluster"] = nx.Cluster
		}
	}

	applyJobOverridesToProviderConfig(&cfg, job)
	return cfg
}

func applyJobOverridesToProviderConfig(cfg *providers.ProviderConfig, job *models.JobDefinition) {
	if cfg.Metadata == nil {
		cfg.Metadata = map[string]interface{}{}
	}

	outputDir := job.OutputDir
	if outputDir == "" {
		outputDir = job.OutputPath
	}
	if outputDir != "" {
		cfg.Metadata["output_dir"] = outputDir
	}
	if job.Format != "" {
		cfg.Metadata["export_format"] = job.Format
	}
	if job.EnablePipeline {
		cfg.Metadata["enable_pipeline"] = true
	}
	if job.Options != nil && job.Options.EnablePipeline {
		cfg.Metadata["enable_pipeline"] = true
	}

	if job.Metadata == nil {
		return
	}
	for _, key := range []string{"cluster", "resolve_containers", "enable_pipeline", "pipeline_timeout"} {
		if v, ok := job.Metadata[key]; ok {
			cfg.Metadata[key] = v
		}
	}
	if mounts, ok := job.Metadata["mounts"]; ok {
		cfg.Metadata["mounts"] = mounts
	}
	if server, ok := job.Metadata["server"].(string); ok && server != "" {
		cfg.Host = server
	}
	if user, ok := job.Metadata["username"].(string); ok && user != "" {
		cfg.Username = user
	}
	if pass, ok := job.Metadata["password"].(string); ok && pass != "" {
		cfg.Password = pass
	}
	if insecure, ok := job.Metadata["insecure"].(bool); ok {
		cfg.Insecure = insecure
	}
}

func buildNutanixExportOptions(job *models.JobDefinition, appConfig *config.Config) providers.ExportOptions {
	outputDir := job.OutputDir
	if outputDir == "" {
		outputDir = job.OutputPath
	}

	format := job.Format
	if format == "" && appConfig != nil && appConfig.Nutanix != nil && appConfig.Nutanix.ExportFormat != "" {
		format = appConfig.Nutanix.ExportFormat
	}
	if format == "" {
		format = "qcow2"
	}

	meta := map[string]interface{}{}
	mounts, _ := resolveJobMounts(job, appConfig)
	if len(mounts) > 0 {
		meta["mounts"] = mounts
	}
	if resolve, ok := boolFromMeta(job.Metadata, "resolve_containers"); ok {
		meta["resolve_containers"] = resolve
	} else if appConfig != nil && appConfig.Nutanix != nil {
		meta["resolve_containers"] = appConfig.Nutanix.ResolveContainers
	}
	enablePipeline := job.EnablePipeline
	if job.Options != nil && job.Options.EnablePipeline {
		enablePipeline = true
	}
	if enablePipeline {
		meta["enable_pipeline"] = true
	}
	if job.Metadata != nil {
		if v, ok := job.Metadata["dry_run"]; ok {
			meta["dry_run"] = v
		}
		if v, ok := job.Metadata["pipeline_timeout"]; ok {
			meta["pipeline_timeout"] = v
		}
		applyPipelineOptionsFromJob(meta, job)
	}

	return providers.ExportOptions{
		OutputPath: outputDir,
		Format:     format,
		Metadata:   meta,
	}
}

type exportProgressReporter struct {
	callback func(*models.JobProgress)
	method   string
}

func (r *exportProgressReporter) Update(phase string, percentComplete float64, message string) {
	if r.callback == nil {
		return
	}
	r.callback(&models.JobProgress{
		Phase:           phase,
		PercentComplete: percentComplete,
		CurrentStep:     message,
		ExportMethod:    r.method,
	})
}

func resolveJobMounts(job *models.JobDefinition, appConfig *config.Config) (nutanix.MountMap, error) {
	if job.Metadata != nil {
		if raw, ok := job.Metadata["mounts"]; ok {
			return mountsFromJobValue(raw)
		}
	}
	if appConfig != nil && appConfig.Nutanix != nil && len(appConfig.Nutanix.Mounts) > 0 {
		return nutanix.MountMap(appConfig.Nutanix.Mounts), nil
	}
	return nil, fmt.Errorf("container NFS mounts required (job.metadata.mounts or nutanix.mounts in config)")
}

func mountsFromJobValue(raw interface{}) (nutanix.MountMap, error) {
	switch v := raw.(type) {
	case map[string]string:
		if len(v) == 0 {
			return nil, fmt.Errorf("mounts map is empty")
		}
		return nutanix.MountMap(v), nil
	case map[string]interface{}:
		out := make(nutanix.MountMap, len(v))
		for k, val := range v {
			path, ok := val.(string)
			if !ok || path == "" {
				return nil, fmt.Errorf("invalid mount path for container %q", k)
			}
			out[k] = path
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("mounts map is empty")
		}
		return out, nil
	case []interface{}:
		specs := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("mount list entries must be strings")
			}
			specs = append(specs, s)
		}
		return nutanix.ParseMountMap(specs)
	case []string:
		return nutanix.ParseMountMap(v)
	case string:
		return nutanix.ParseMountMap(strings.Split(v, ","))
	default:
		return nil, fmt.Errorf("unsupported mounts type %T", raw)
	}
}

func boolFromMeta(meta map[string]interface{}, key string) (bool, bool) {
	if meta == nil {
		return false, false
	}
	v, ok := meta[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func applyPipelineOptionsFromJob(meta map[string]interface{}, job *models.JobDefinition) {
	if job.Options == nil {
		return
	}
	opts := job.Options
	if opts.Hyper2KVMPath != "" {
		meta["hyper2kvm_path"] = opts.Hyper2KVMPath
	}
	if opts.LibvirtIntegration {
		meta["libvirt_integration"] = true
	}
	if opts.LibvirtURI != "" {
		meta["libvirt_uri"] = opts.LibvirtURI
	}
	if opts.LibvirtAutoStart {
		meta["libvirt_auto_start"] = true
	}
	if opts.Hyper2KVMDaemon {
		meta["hyper2kvm_daemon"] = true
	}
	if opts.Hyper2KVMInstance != "" {
		meta["hyper2kvm_instance"] = opts.Hyper2KVMInstance
	}
	if opts.Hyper2KVMWatchDir != "" {
		meta["hyper2kvm_watch_dir"] = opts.Hyper2KVMWatchDir
	}
	if opts.Hyper2KVMOutputDir != "" {
		meta["hyper2kvm_output_dir"] = opts.Hyper2KVMOutputDir
	}
	if opts.Hyper2KVMPollInterval > 0 {
		meta["hyper2kvm_poll_interval"] = opts.Hyper2KVMPollInterval
	}
	if opts.Hyper2KVMDaemonTimeout > 0 {
		meta["hyper2kvm_daemon_timeout"] = opts.Hyper2KVMDaemonTimeout
	}
}
