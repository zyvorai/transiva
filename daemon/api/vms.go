// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/providers"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// VMListResponse represents the response for VM listing (vSphere legacy format).
type VMListResponse struct {
	Provider  string           `json:"provider,omitempty"`
	VMs       []vsphere.VMInfo `json:"vms,omitempty"`
	Unified   []*providers.VMInfo `json:"unified_vms,omitempty"`
	Total     int              `json:"total"`
	Timestamp time.Time        `json:"timestamp"`
}

// UnifiedVMListResponse is the provider-neutral VM list response.
type UnifiedVMListResponse struct {
	Provider  string              `json:"provider"`
	VMs       []*providers.VMInfo `json:"vms"`
	Total     int                 `json:"total"`
	Timestamp time.Time           `json:"timestamp"`
}

// Handle VM listing
func (s *Server) handleListVMs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		providerName = "vsphere"
	}

	if providerName == string(providers.ProviderNutanix) {
		s.handleListNutanixVMs(w, r)
		return
	}

	// Default: vSphere (backward compatible)
	server := r.URL.Query().Get("server")
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")
	insecure := r.URL.Query().Get("insecure") == "true"

	var vms []vsphere.VMInfo
	var err error

	if server != "" && username != "" && password != "" {
		vms, err = s.listVMsWithCredentials(server, username, password, insecure)
	} else {
		vms, err = s.listVMsFromVSphere()
	}

	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list VMs: %v", err)
		return
	}

	resp := VMListResponse{
		Provider:  "vsphere",
		VMs:       vms,
		Total:     len(vms),
		Timestamp: time.Now(),
	}

	s.jsonResponse(w, http.StatusOK, resp)
}

func (s *Server) handleListNutanixVMs(w http.ResponseWriter, r *http.Request) {
	if s.providerRegistry == nil || !s.providerRegistry.IsRegistered(providers.ProviderNutanix) {
		s.errorResponse(w, http.StatusBadRequest, "nutanix provider is not registered")
		return
	}

	detailed := r.URL.Query().Get("detailed") != "false"
	cluster := r.URL.Query().Get("cluster")

	server := r.URL.Query().Get("server")
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")
	insecure := r.URL.Query().Get("insecure") == "true"

	providerCfg := s.nutanixProviderConfig(server, username, password, insecure, cluster, detailed)

	provider, err := s.providerRegistry.Create(providers.ProviderNutanix, providerCfg)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create nutanix provider: %v", err)
		return
	}
	defer provider.Disconnect()

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	filter := providers.VMFilter{}
	if cluster != "" {
		filter.Location = cluster
	}
	if nameFilter := r.URL.Query().Get("filter"); nameFilter != "" {
		filter.NamePattern = nameFilter
	}

	vms, err := provider.ListVMs(ctx, filter)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to list Nutanix VMs: %v", err)
		return
	}

	resp := UnifiedVMListResponse{
		Provider:  string(providers.ProviderNutanix),
		VMs:       vms,
		Total:     len(vms),
		Timestamp: time.Now(),
	}

	s.logger.Info("discovered Nutanix VMs", "count", len(vms))
	s.jsonResponse(w, http.StatusOK, resp)
}

func (s *Server) nutanixProviderConfig(server, username, password string, insecure bool, cluster string, detailed bool) providers.ProviderConfig {
	cfg := providers.ProviderConfig{
		Type:     providers.ProviderNutanix,
		Insecure: insecure,
		Metadata: map[string]interface{}{
			"detailed": detailed,
		},
	}

	if server != "" && username != "" && password != "" {
		cfg.Host = server
		cfg.Username = username
		cfg.Password = password
	} else {
		appCfg := s.appConfig
		if appCfg == nil {
			appCfg = config.FromEnvironment()
		}
		if appCfg.Nutanix != nil {
			nx := appCfg.Nutanix
			cfg.Host = nx.Host
			cfg.Port = nx.Port
			cfg.Username = nx.Username
			cfg.Password = nx.Password
			cfg.Insecure = !nx.VerifySSL
			if cluster == "" {
				cluster = nx.Cluster
			}
			if nx.PageSize > 0 {
				cfg.Metadata["page_size"] = nx.PageSize
			}
			if nx.DetailWorkers > 0 {
				cfg.Metadata["detail_workers"] = nx.DetailWorkers
			}
		}
	}

	s.applyNutanixExportMetadata(&cfg)
	if cluster != "" {
		cfg.Metadata["cluster"] = cluster
	}
	if nxDetailed, ok := cfg.Metadata["detailed"].(bool); !ok || nxDetailed {
		cfg.Metadata["detailed"] = detailed
	}

	return cfg
}

func (s *Server) applyNutanixExportMetadata(cfg *providers.ProviderConfig) {
	appCfg := s.appConfig
	if appCfg == nil {
		appCfg = config.FromEnvironment()
	}
	if appCfg.Nutanix == nil {
		return
	}
	if cfg.Metadata == nil {
		cfg.Metadata = map[string]interface{}{}
	}

	nx := appCfg.Nutanix
	if _, ok := cfg.Metadata["output_dir"]; !ok && nx.OutputDir != "" {
		cfg.Metadata["output_dir"] = nx.OutputDir
	}
	if _, ok := cfg.Metadata["export_format"]; !ok && nx.ExportFormat != "" {
		cfg.Metadata["export_format"] = nx.ExportFormat
	}
	if _, ok := cfg.Metadata["mounts"]; !ok && len(nx.Mounts) > 0 {
		cfg.Metadata["mounts"] = nx.Mounts
	}
	if _, ok := cfg.Metadata["resolve_containers"]; !ok {
		cfg.Metadata["resolve_containers"] = nx.ResolveContainers
	}
	if _, ok := cfg.Metadata["enable_pipeline"]; !ok {
		cfg.Metadata["enable_pipeline"] = nx.EnablePipeline
	}
	if _, ok := cfg.Metadata["pipeline_timeout"]; !ok && nx.PipelineTimeout > 0 {
		cfg.Metadata["pipeline_timeout"] = nx.PipelineTimeout
	}
}

// List VMs with provided credentials (for web UI)
func (s *Server) listVMsWithCredentials(server, username, password string, insecure bool) ([]vsphere.VMInfo, error) {
	ctx := context.Background()

	cfg := &config.Config{
		VCenterURL: fmt.Sprintf("https://%s/sdk", server),
		Username:   username,
		Password:   password,
		Insecure:   insecure,
		Timeout:    30 * time.Second,
	}

	client, err := vsphere.NewVSphereClient(ctx, cfg, s.logger)
	if err != nil {
		s.logger.Error("failed to create vSphere client", "error", err, "server", server)
		return nil, err
	}
	defer client.Close()

	vms, err := client.ListVMs(ctx)
	if err != nil {
		s.logger.Error("failed to list VMs from vSphere", "error", err)
		return nil, err
	}

	s.logger.Info("discovered VMs from vSphere", "count", len(vms), "server", server)
	return vms, nil
}

// List all VMs from vSphere using configured credentials
func (s *Server) listVMsFromVSphere() ([]vsphere.VMInfo, error) {
	ctx := context.Background()

	client, err := s.manager.GetVSphereClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	vms, err := client.ListVMs(ctx)
	if err != nil {
		return nil, err
	}

	return vms, nil
}
