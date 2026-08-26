// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/transiva/providers"
)

// VMInfoResponse returns VM details from a provider.
type VMInfoResponse struct {
	Provider  string              `json:"provider"`
	VM        *providers.VMInfo   `json:"vm"`
	Timestamp time.Time           `json:"timestamp"`
}

func (s *Server) handleVMInfo(w http.ResponseWriter, r *http.Request) {
	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		providerName = string(providers.ProviderVSphere)
	}

	if providerName == string(providers.ProviderNutanix) {
		s.handleNutanixVMInfo(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VMOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}

	if req.VMPath == "" {
		s.errorResponse(w, http.StatusBadRequest, "vm_path is required")
		return
	}

	client, err := s.manager.GetVSphereClient()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to connect to vSphere: %v", err)
		return
	}
	defer client.Close()

	ctx := context.Background()
	info, err := client.GetVMInfo(ctx, req.VMPath)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to get VM info: %v", err)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"vm_info":   info,
		"timestamp": time.Now(),
	})
}

func (s *Server) handleNutanixVMInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.providerRegistry == nil || !s.providerRegistry.IsRegistered(providers.ProviderNutanix) {
		s.errorResponse(w, http.StatusBadRequest, "nutanix provider is not registered")
		return
	}

	vmID := strings.TrimSpace(r.URL.Query().Get("vm"))
	cluster := r.URL.Query().Get("cluster")
	server := r.URL.Query().Get("server")
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")
	insecure := r.URL.Query().Get("insecure") == "true"

	if r.Method == http.MethodPost && vmID == "" {
		var req struct {
			VM       string `json:"vm"`
			Cluster  string `json:"cluster,omitempty"`
			Server   string `json:"server,omitempty"`
			Username string `json:"username,omitempty"`
			Password string `json:"password,omitempty"`
			Insecure bool   `json:"insecure,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
			return
		}
		if vmID == "" {
			vmID = strings.TrimSpace(req.VM)
		}
		if cluster == "" {
			cluster = req.Cluster
		}
		if server == "" {
			server = req.Server
		}
		if username == "" {
			username = req.Username
		}
		if password == "" {
			password = req.Password
		}
		if !insecure {
			insecure = req.Insecure
		}
	}

	if vmID == "" {
		s.errorResponse(w, http.StatusBadRequest, "vm is required")
		return
	}

	providerCfg := s.nutanixProviderConfig(server, username, password, insecure, cluster, true)
	provider, err := s.providerRegistry.Create(providers.ProviderNutanix, providerCfg)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to create nutanix provider: %v", err)
		return
	}
	defer provider.Disconnect()

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	vm, err := provider.GetVM(ctx, vmID)
	if err != nil {
		// Fall back to search by name
		matches, searchErr := provider.SearchVMs(ctx, vmID)
		if searchErr != nil || len(matches) == 0 {
			s.errorResponse(w, http.StatusNotFound, "VM %q not found: %v", vmID, err)
			return
		}
		for _, match := range matches {
			if strings.EqualFold(match.Name, vmID) || match.ID == vmID {
				vm, err = provider.GetVM(ctx, match.ID)
				break
			}
		}
		if vm == nil && len(matches) == 1 {
			vm, err = provider.GetVM(ctx, matches[0].ID)
		}
		if err != nil || vm == nil {
			s.errorResponse(w, http.StatusNotFound, "VM %q not found", vmID)
			return
		}
	}

	s.jsonResponse(w, http.StatusOK, VMInfoResponse{
		Provider:  string(providers.ProviderNutanix),
		VM:        vm,
		Timestamp: time.Now(),
	})
}
