// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"time"

	"github.com/zyvorai/transiva/providers"
)

type providerCapabilitiesResponse struct {
	Provider     string                      `json:"provider"`
	Capabilities providers.ExportCapabilities `json:"capabilities"`
	Registered   bool                        `json:"registered"`
	Timestamp    time.Time                   `json:"timestamp"`
}

func (s *Server) handleProviderCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		s.errorResponse(w, http.StatusBadRequest, "provider query parameter is required")
		return
	}

	pType := providers.ProviderType(providerName)
	if s.providerRegistry == nil || !s.providerRegistry.IsRegistered(pType) {
		s.errorResponse(w, http.StatusBadRequest, "provider %q is not registered", providerName)
		return
	}

	var caps providers.ExportCapabilities
	if pType == providers.ProviderNutanix {
		providerCfg := s.nutanixProviderConfig("", "", "", false, "", true)
		if provider, err := s.providerRegistry.Create(pType, providerCfg); err == nil {
			caps = provider.GetExportCapabilities()
			_ = provider.Disconnect()
		} else {
			caps = providers.ExportCapabilities{
				SupportedFormats: []string{"qcow2", "raw"},
				SupportedTargets: []string{"local"},
			}
		}
	} else if provider, err := s.providerRegistry.Create(pType, providers.ProviderConfig{Type: pType}); err == nil {
		caps = provider.GetExportCapabilities()
		_ = provider.Disconnect()
	} else {
		s.errorResponse(w, http.StatusInternalServerError, "failed to query provider capabilities: %v", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, providerCapabilitiesResponse{
		Provider:     providerName,
		Capabilities: caps,
		Registered:   true,
		Timestamp:    time.Now(),
	})
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.providerRegistry == nil {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"providers": []string{},
			"timestamp": time.Now(),
		})
		return
	}

	types := s.providerRegistry.ListProviders()
	names := make([]string, 0, len(types))
	for _, p := range types {
		names = append(names, string(p))
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"providers": names,
		"timestamp": time.Now(),
	})
}
