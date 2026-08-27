// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"fmt"
	"strings"
	"time"

	"github.com/zyvorai/transiva/providers"
	"github.com/zyvorai/transiva/providers/common"
)

type nutanixExportOptions struct {
	OutputDir         string
	Format            string
	Mounts            MountMap
	DryRun            bool
	ResolveContainers bool
	EnablePipeline    bool
	PipelineTimeout   time.Duration
	Pipeline          common.H2KVMConfig
}

func mergeExportOptions(p *Provider, opts providers.ExportOptions) (nutanixExportOptions, error) {
	out := nutanixExportOptions{
		Format: "qcow2",
	}

	if p != nil && p.cfg.Metadata != nil {
		applyProviderMetadata(&out, p.cfg.Metadata)
	}
	if opts.Metadata != nil {
		applyExportMetadata(&out, opts.Metadata)
	}

	if opts.OutputPath != "" {
		out.OutputDir = opts.OutputPath
	}
	if opts.Format != "" {
		out.Format = strings.ToLower(opts.Format)
	}

	if out.OutputDir == "" {
		return out, fmt.Errorf("output path is required (set ExportOptions.OutputPath or nutanix.output_dir)")
	}
	if len(out.Mounts) == 0 {
		return out, fmt.Errorf("container NFS mounts are required (set nutanix.mounts or ExportOptions.Metadata[\"mounts\"])")
	}
	if out.Format != "qcow2" && out.Format != "raw" {
		return out, fmt.Errorf("unsupported export format %q (use qcow2 or raw)", out.Format)
	}

	return out, nil
}

func applyProviderMetadata(out *nutanixExportOptions, meta map[string]interface{}) {
	if v, ok := metaString(meta, "output_dir"); ok {
		out.OutputDir = v
	}
	if v, ok := metaString(meta, "export_format"); ok {
		out.Format = strings.ToLower(v)
	}
	if mounts, err := mountsFromMetadata(meta["mounts"]); err == nil && len(mounts) > 0 {
		out.Mounts = mounts
	}
	if v, ok := metaBool(meta, "resolve_containers"); ok {
		out.ResolveContainers = v
	}
	if v, ok := metaBool(meta, "enable_pipeline"); ok {
		out.EnablePipeline = v
	}
	if d, ok := metaDuration(meta, "pipeline_timeout"); ok {
		out.PipelineTimeout = d
	}
	applyPipelineMetadata(&out.Pipeline, meta)
}

func applyExportMetadata(out *nutanixExportOptions, meta map[string]interface{}) {
	if mounts, err := mountsFromMetadata(meta["mounts"]); err == nil && len(mounts) > 0 {
		out.Mounts = mounts
	}
	if v, ok := metaString(meta, "output_dir"); ok {
		out.OutputDir = v
	}
	if v, ok := metaString(meta, "disk_format", "export_format"); ok {
		out.Format = strings.ToLower(v)
	}
	if v, ok := metaBool(meta, "dry_run"); ok {
		out.DryRun = v
	}
	if v, ok := metaBool(meta, "resolve_containers"); ok {
		out.ResolveContainers = v
	}
	if v, ok := metaBool(meta, "run_pipeline", "enable_pipeline"); ok {
		out.EnablePipeline = v
	}
	if d, ok := metaDuration(meta, "pipeline_timeout"); ok {
		out.PipelineTimeout = d
	}
	applyPipelineMetadata(&out.Pipeline, meta)
}

func applyPipelineMetadata(cfg *common.H2KVMConfig, meta map[string]interface{}) {
	// h2kvm_* keys are primary; hyper2kvm_* are accepted as deprecated aliases
	// for configs written before the h2kvm rename.
	if v, ok := metaString(meta, "h2kvm_path", "hyper2kvm_path"); ok {
		cfg.H2KVMPath = v
	}
	if v, ok := metaBool(meta, "libvirt_integration"); ok {
		cfg.LibvirtIntegration = v
	}
	if v, ok := metaString(meta, "libvirt_uri"); ok {
		cfg.LibvirtURI = v
	}
	if v, ok := metaBool(meta, "libvirt_auto_start"); ok {
		cfg.AutoStart = v
	}
	if v, ok := metaBool(meta, "pipeline_dry_run"); ok {
		cfg.DryRun = v
	}
	if v, ok := metaBool(meta, "h2kvm_daemon", "hyper2kvm_daemon", "use_daemon"); ok {
		cfg.UseDaemon = v
	}
	if v, ok := metaString(meta, "h2kvm_instance", "hyper2kvm_instance", "daemon_instance"); ok {
		cfg.DaemonInstance = v
	}
	if v, ok := metaString(meta, "h2kvm_watch_dir", "hyper2kvm_watch_dir", "daemon_watch_dir"); ok {
		cfg.DaemonWatchDir = v
	}
	if v, ok := metaString(meta, "h2kvm_output_dir", "hyper2kvm_output_dir", "daemon_output_dir"); ok {
		cfg.DaemonOutputDir = v
	}
	if v, ok := metaInt(meta, "h2kvm_poll_interval", "hyper2kvm_poll_interval", "daemon_poll_interval"); ok {
		cfg.DaemonPollInterval = v
	}
	if v, ok := metaInt(meta, "h2kvm_daemon_timeout", "hyper2kvm_daemon_timeout", "daemon_timeout"); ok {
		cfg.DaemonTimeout = v
	}
}

func mountsFromMetadata(raw interface{}) (MountMap, error) {
	if raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case MountMap:
		return v, nil
	case map[string]string:
		return MountMap(v), nil
	case map[string]interface{}:
		m := make(MountMap, len(v))
		for k, val := range v {
			path, ok := val.(string)
			if !ok || strings.TrimSpace(path) == "" {
				return nil, fmt.Errorf("invalid mount path for container %q", k)
			}
			m[k] = path
		}
		return m, nil
	case []string:
		return ParseMountMap(v)
	case []interface{}:
		specs := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("mount list entries must be strings")
			}
			specs = append(specs, s)
		}
		return ParseMountMap(specs)
	case string:
		parts := strings.Split(v, ",")
		return ParseMountMap(parts)
	default:
		return nil, fmt.Errorf("unsupported mounts type %T", raw)
	}
}

func metaString(meta map[string]interface{}, keys ...string) (string, bool) {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s), true
		}
	}
	return "", false
}

func metaBool(meta map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		switch b := v.(type) {
		case bool:
			return b, true
		case string:
			switch strings.ToLower(strings.TrimSpace(b)) {
			case "1", "true", "yes", "on":
				return true, true
			case "0", "false", "no", "off":
				return false, true
			}
		}
	}
	return false, false
}

func metaInt(meta map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case int:
			return n, true
		case int64:
			return int(n), true
		case float64:
			return int(n), true
		}
	}
	return 0, false
}

func metaDuration(meta map[string]interface{}, keys ...string) (time.Duration, bool) {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok {
			continue
		}
		switch d := v.(type) {
		case time.Duration:
			return d, true
		case string:
			if parsed, err := time.ParseDuration(d); err == nil {
				return parsed, true
			}
		case int:
			return time.Duration(d) * time.Second, true
		case int64:
			return time.Duration(d) * time.Second, true
		case float64:
			return time.Duration(d) * time.Second, true
		}
	}
	return 0, false
}
