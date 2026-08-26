// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// Host/Cluster Infrastructure Handlers

func (es *EnhancedServer) handleListHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "*"
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// List hosts
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	hosts, err := client.ListHosts(ctx, pattern)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to list hosts: %v", err)
		return
	}

	resp := models.HostInfoResponse{
		Hosts:     hosts,
		Total:     len(hosts),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleListClusters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "*"
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// List clusters
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	clusters, err := client.ListClusters(ctx, pattern)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to list clusters: %v", err)
		return
	}

	resp := models.ClusterInfoResponse{
		Clusters:  clusters,
		Total:     len(clusters),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleListDatacenters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// List datacenters
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	datacenters, err := client.ListDatacenters(ctx)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to list datacenters: %v", err)
		return
	}

	resp := models.DatacenterInfoResponse{
		Datacenters: datacenters,
		Total:       len(datacenters),
		Timestamp:   time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleGetVCenterInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Get vCenter info
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	info, err := client.GetVCenterInfo(ctx)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to get vCenter info: %v", err)
		return
	}

	resp := models.VCenterInfoResponse{
		Info:      info,
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

// Performance Metrics Handlers

func (es *EnhancedServer) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	entityName := r.URL.Query().Get("entity")
	entityType := r.URL.Query().Get("type")
	realtime := r.URL.Query().Get("realtime") == "true"
	interval := r.URL.Query().Get("interval")

	if entityName == "" {
		es.errorResponse(w, http.StatusBadRequest, "entity name required")
		return
	}
	if entityType == "" {
		entityType = "vm" // Default to VM
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if realtime {
		// Get realtime metrics
		metrics, err := client.GetRealtimeMetrics(ctx, entityName, entityType)
		if err != nil {
			es.errorResponse(w, http.StatusInternalServerError, "failed to get metrics: %v", err)
			return
		}

		resp := models.MetricsResponse{
			Realtime: true,
			Current:  metrics,
		}
		es.jsonResponse(w, http.StatusOK, resp)
	} else {
		// Get historical metrics
		startStr := r.URL.Query().Get("start")
		endStr := r.URL.Query().Get("end")

		var start, end time.Time
		if startStr != "" {
			start, err = time.Parse(time.RFC3339, startStr)
			if err != nil {
				es.errorResponse(w, http.StatusBadRequest, "invalid start time: %v", err)
				return
			}
		} else {
			start = time.Now().Add(-1 * time.Hour) // Default: last hour
		}

		if endStr != "" {
			end, err = time.Parse(time.RFC3339, endStr)
			if err != nil {
				es.errorResponse(w, http.StatusBadRequest, "invalid end time: %v", err)
				return
			}
		} else {
			end = time.Now()
		}

		if interval == "" {
			interval = "5min" // Default interval
		}

		history, err := client.GetMetricsHistory(ctx, entityName, entityType, start, end, interval)
		if err != nil {
			es.errorResponse(w, http.StatusInternalServerError, "failed to get historical metrics: %v", err)
			return
		}

		resp := models.MetricsResponse{
			Realtime: false,
			History:  history,
		}
		es.jsonResponse(w, http.StatusOK, resp)
	}
}

// Resource Pool Handlers

func (es *EnhancedServer) handleListResourcePools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "*"
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// List resource pools
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	pools, err := client.ListResourcePools(ctx, pattern)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to list resource pools: %v", err)
		return
	}

	resp := models.ResourcePoolResponse{
		Pools:     pools,
		Total:     len(pools),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleCreateResourcePool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req models.CreateResourcePoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Create resource pool
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	err = client.CreateResourcePool(ctx, req.Config)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create resource pool: %v", err)
		return
	}

	resp := models.ResourcePoolOperationResponse{
		Success:   true,
		Message:   fmt.Sprintf("Resource pool %s created successfully", req.Config.Name),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleUpdateResourcePool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pool name from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		es.errorResponse(w, http.StatusBadRequest, "pool name required in URL")
		return
	}
	poolName := parts[len(parts)-1]

	// Parse request
	var req models.UpdateResourcePoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}
	req.PoolName = poolName

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Update resource pool
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	err = client.UpdateResourcePool(ctx, poolName, req.Config)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to update resource pool: %v", err)
		return
	}

	resp := models.ResourcePoolOperationResponse{
		Success:   true,
		Message:   fmt.Sprintf("Resource pool %s updated successfully", poolName),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleDeleteResourcePool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract pool name from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		es.errorResponse(w, http.StatusBadRequest, "pool name required in URL")
		return
	}
	poolName := parts[len(parts)-1]

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Delete resource pool
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	err = client.DeleteResourcePool(ctx, poolName)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to delete resource pool: %v", err)
		return
	}

	resp := models.ResourcePoolOperationResponse{
		Success:   true,
		Message:   fmt.Sprintf("Resource pool %s deleted successfully", poolName),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

// Event & Task Handlers

func (es *EnhancedServer) handleGetRecentEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	sinceStr := r.URL.Query().Get("since")
	eventTypesStr := r.URL.Query().Get("types")
	entityTypesStr := r.URL.Query().Get("entity_types")

	var since time.Time
	var err error
	if sinceStr != "" {
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			es.errorResponse(w, http.StatusBadRequest, "invalid since time: %v", err)
			return
		}
	} else {
		since = time.Now().Add(-1 * time.Hour) // Default: last hour
	}

	var eventTypes []string
	if eventTypesStr != "" {
		eventTypes = strings.Split(eventTypesStr, ",")
	}

	var entityTypes []string
	if entityTypesStr != "" {
		entityTypes = strings.Split(entityTypesStr, ",")
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Get events
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	events, err := client.GetRecentEvents(ctx, since, eventTypes, entityTypes)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to get events: %v", err)
		return
	}

	resp := models.EventResponse{
		Events:    events,
		Total:     len(events),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleGetRecentTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	sinceStr := r.URL.Query().Get("since")

	var since time.Time
	var err error
	if sinceStr != "" {
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			es.errorResponse(w, http.StatusBadRequest, "invalid since time: %v", err)
			return
		}
	} else {
		since = time.Now().Add(-1 * time.Hour) // Default: last hour
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Get tasks
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	tasks, err := client.GetRecentTasks(ctx, since)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to get tasks: %v", err)
		return
	}

	resp := models.TaskResponse{
		Tasks:     tasks,
		Total:     len(tasks),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

// VM Cloning Handlers

func (es *EnhancedServer) handleCloneVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req models.CloneVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Clone VM
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	result, err := client.CloneVM(ctx, req.Spec)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to clone VM: %v", err)
		return
	}

	resp := models.CloneVMResponse{
		Result:    *result,
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleBulkClone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req models.BulkCloneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}

	maxConcurrent := req.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5 // Default
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Bulk clone VMs
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	startTime := time.Now()
	results, err := client.BulkCloneVMs(ctx, req.Specs, maxConcurrent)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "bulk clone failed: %v", err)
		return
	}

	// Count successes and failures
	var successCount, failCount int
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	resp := models.BulkCloneResponse{
		Results:   results,
		Success:   successCount,
		Failed:    failCount,
		Duration:  time.Since(startTime),
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req models.TemplateOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Create template
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	err = client.CreateTemplate(ctx, req.VMName)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create template: %v", err)
		return
	}

	resp := models.TemplateOperationResponse{
		Success:   true,
		Message:   fmt.Sprintf("VM %s converted to template successfully", req.VMName),
		VMName:    req.VMName,
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

func (es *EnhancedServer) handleDeployFromTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req models.CloneVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		es.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}

	// Get vSphere client
	client, err := es.getVSphereClient(r)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to create vSphere client: %v", err)
		return
	}
	defer client.Close()

	// Deploy from template
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	result, err := client.DeployFromTemplate(ctx, req.Spec)
	if err != nil {
		es.errorResponse(w, http.StatusInternalServerError, "failed to deploy from template: %v", err)
		return
	}

	resp := models.CloneVMResponse{
		Result:    *result,
		Timestamp: time.Now(),
	}

	es.jsonResponse(w, http.StatusOK, resp)
}

// Helper function to get vSphere client from request
func (es *EnhancedServer) getVSphereClient(r *http.Request) (*vsphere.VSphereClient, error) {
	// Check if credentials are provided in query parameters (for web UI)
	server := r.URL.Query().Get("server")
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")
	insecure := r.URL.Query().Get("insecure") == "true"

	var cfg *config.Config

	if server != "" && username != "" && password != "" {
		// Use provided credentials (web UI mode)
		cfg = &config.Config{
			VCenterURL: fmt.Sprintf("https://%s/sdk", server),
			Username:   username,
			Password:   password,
			Insecure:   insecure,
			Timeout:    120 * time.Second,
		}
	} else {
		// Use configured credentials (CLI mode)
		cfg = config.FromEnvironment()
	}

	ctx := context.Background()
	return vsphere.NewVSphereClient(ctx, cfg, es.logger)
}
