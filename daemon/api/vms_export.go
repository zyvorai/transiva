// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/transiva/providers"
)

// VMExportRequest describes a VM export operation.
type VMExportRequest struct {
	VM                string            `json:"vm"`
	OutputPath        string            `json:"output_path"`
	Format            string            `json:"format,omitempty"`
	DryRun            bool              `json:"dry_run,omitempty"`
	Mounts            map[string]string `json:"mounts,omitempty"`
	ResolveContainers bool              `json:"resolve_containers,omitempty"`
	EnablePipeline    bool              `json:"enable_pipeline,omitempty"`
	PipelineTimeout   string            `json:"pipeline_timeout,omitempty"`
	Server            string            `json:"server,omitempty"`
	Username          string            `json:"username,omitempty"`
	Password          string            `json:"password,omitempty"`
	Insecure          bool              `json:"insecure,omitempty"`
	Cluster           string            `json:"cluster,omitempty"`
}

// VMExportResponse wraps a provider export result.
type VMExportResponse struct {
	Provider  string                  `json:"provider"`
	Result    *providers.ExportResult `json:"result"`
	Timestamp time.Time               `json:"timestamp"`
}

func (s *Server) handleExportVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		providerName = string(providers.ProviderVSphere)
	}

	switch providerName {
	case string(providers.ProviderNutanix):
		s.handleExportNutanixVM(w, r)
	default:
		s.errorResponse(w, http.StatusBadRequest, "export not supported for provider %q (supported: nutanix)", providerName)
	}
}

func (s *Server) handleExportNutanixVM(w http.ResponseWriter, r *http.Request) {
	if s.providerRegistry == nil || !s.providerRegistry.IsRegistered(providers.ProviderNutanix) {
		s.errorResponse(w, http.StatusBadRequest, "nutanix provider is not registered")
		return
	}

	req, err := parseVMExportRequest(r)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid export request: %v", err)
		return
	}
	if strings.TrimSpace(req.VM) == "" {
		s.errorResponse(w, http.StatusBadRequest, "vm is required")
		return
	}
	if strings.TrimSpace(req.OutputPath) == "" {
		s.errorResponse(w, http.StatusBadRequest, "output_path is required")
		return
	}

	server := firstNonEmpty(req.Server, r.URL.Query().Get("server"))
	username := firstNonEmpty(req.Username, r.URL.Query().Get("username"))
	password := firstNonEmpty(req.Password, r.URL.Query().Get("password"))
	insecure := req.Insecure || r.URL.Query().Get("insecure") == "true"
	cluster := firstNonEmpty(req.Cluster, r.URL.Query().Get("cluster"))

	providerCfg := s.nutanixProviderConfig(server, username, password, insecure, cluster, true)
	mergeExportRequestIntoProviderConfig(&providerCfg, req)

	provider, err := s.providerRegistry.Create(providers.ProviderNutanix, providerCfg)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create nutanix provider: %v", err)
		return
	}
	defer provider.Disconnect()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	exportOpts := providers.ExportOptions{
		OutputPath: req.OutputPath,
		Format:     req.Format,
		Metadata:   exportMetadataFromRequest(req),
	}

	result, err := provider.ExportVM(ctx, req.VM, exportOpts)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "export failed: %v", err)
		return
	}

	s.logger.Info("exported Nutanix VM",
		"vm", result.VMName,
		"uuid", result.VMID,
		"output", result.OutputPath,
		"format", result.Format)

	s.jsonResponse(w, http.StatusOK, VMExportResponse{
		Provider:  string(providers.ProviderNutanix),
		Result:    result,
		Timestamp: time.Now(),
	})
}

func parseVMExportRequest(r *http.Request) (VMExportRequest, error) {
	req := VMExportRequest{}

	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return req, err
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return req, err
			}
		}
	}

	q := r.URL.Query()
	if req.VM == "" {
		req.VM = q.Get("vm")
	}
	if req.OutputPath == "" {
		req.OutputPath = firstNonEmpty(q.Get("output_path"), q.Get("output"))
	}
	if req.Format == "" {
		req.Format = q.Get("format")
	}
	if !req.DryRun {
		req.DryRun = q.Get("dry_run") == "true" || q.Get("dry-run") == "true"
	}
	if !req.ResolveContainers {
		req.ResolveContainers = q.Get("resolve_containers") == "true"
	}
	if !req.EnablePipeline {
		req.EnablePipeline = q.Get("enable_pipeline") == "true" || q.Get("pipeline") == "true"
	}
	if req.PipelineTimeout == "" {
		req.PipelineTimeout = q.Get("pipeline_timeout")
	}
	if req.Cluster == "" {
		req.Cluster = q.Get("cluster")
	}
	if req.Server == "" {
		req.Server = q.Get("server")
	}
	if req.Username == "" {
		req.Username = q.Get("username")
	}
	if req.Password == "" {
		req.Password = q.Get("password")
	}
	if !req.Insecure {
		req.Insecure = q.Get("insecure") == "true"
	}
	if len(req.Mounts) == 0 {
		if mountsSpec := q.Get("mounts"); mountsSpec != "" {
			req.Mounts = parseMountSpecMap(mountsSpec)
		}
	}

	return req, nil
}

func mergeExportRequestIntoProviderConfig(cfg *providers.ProviderConfig, req VMExportRequest) {
	if cfg.Metadata == nil {
		cfg.Metadata = map[string]interface{}{}
	}
	if req.OutputPath != "" {
		if _, ok := cfg.Metadata["output_dir"]; !ok {
			cfg.Metadata["output_dir"] = req.OutputPath
		}
	}
	if req.Format != "" {
		if _, ok := cfg.Metadata["export_format"]; !ok {
			cfg.Metadata["export_format"] = req.Format
		}
	}
	if len(req.Mounts) > 0 {
		cfg.Metadata["mounts"] = req.Mounts
	}
	if req.ResolveContainers {
		cfg.Metadata["resolve_containers"] = true
	}
	if req.EnablePipeline {
		cfg.Metadata["enable_pipeline"] = true
	}
	if req.PipelineTimeout != "" {
		cfg.Metadata["pipeline_timeout"] = req.PipelineTimeout
	}
}

func exportMetadataFromRequest(req VMExportRequest) map[string]interface{} {
	meta := map[string]interface{}{
		"dry_run": req.DryRun,
	}
	if len(req.Mounts) > 0 {
		meta["mounts"] = req.Mounts
	}
	if req.ResolveContainers {
		meta["resolve_containers"] = true
	}
	if req.EnablePipeline {
		meta["enable_pipeline"] = true
	}
	if req.PipelineTimeout != "" {
		meta["pipeline_timeout"] = req.PipelineTimeout
	}
	return meta
}

func parseMountSpecMap(spec string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			continue
		}
		out[kv[0]] = kv[1]
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
