// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// Server handles HTTP API requests
type Server struct {
	manager          *jobs.Manager
	detector         *capabilities.Detector
	logger           logger.Logger
	httpServer       *http.Server
	providerRegistry *providers.Registry
	appConfig        *config.Config
}

// NewServer creates a new API server
func NewServer(manager *jobs.Manager, detector *capabilities.Detector, log logger.Logger, addr string, providerRegistry *providers.Registry, appConfig *config.Config) *Server {
	s := &Server{
		manager:          manager,
		detector:         detector,
		logger:           log,
		providerRegistry: providerRegistry,
		appConfig:        appConfig,
	}

	mux := http.NewServeMux()

	// Core routes
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/capabilities", s.handleCapabilities)

	// Job management
	mux.HandleFunc("/jobs/submit", s.handleSubmitJob)
	mux.HandleFunc("/jobs/query", s.handleQueryJobs)
	mux.HandleFunc("/jobs/cancel", s.handleCancelJobs)
	mux.HandleFunc("/jobs/", s.handleGetJob) // /jobs/{id}

	// Job Progress Tracking
	mux.HandleFunc("/jobs/progress/", s.handleGetJobProgress) // /jobs/progress/{id}
	mux.HandleFunc("/jobs/logs/", s.handleGetJobLogs)         // /jobs/logs/{id}
	mux.HandleFunc("/jobs/eta/", s.handleGetJobETA)           // /jobs/eta/{id}

	// VM discovery and management
	mux.HandleFunc("/vms/list", s.handleListVMs)
	mux.HandleFunc("/vms/export", s.handleExportVM)
	mux.HandleFunc("/vms/info", s.handleVMInfo)
	mux.HandleFunc("/providers/list", s.handleListProviders)
	mux.HandleFunc("/providers/capabilities", s.handleProviderCapabilities)
	mux.HandleFunc("/vms/shutdown", s.handleVMShutdown)
	mux.HandleFunc("/vms/poweroff", s.handleVMPowerOff)
	mux.HandleFunc("/vms/remove-cdrom", s.handleVMRemoveCDROM)

	// Scheduler & Automation
	mux.HandleFunc("/schedules/list", s.handleListSchedules)
	mux.HandleFunc("/schedules/create", s.handleCreateSchedule)
	mux.HandleFunc("/backup-policies/list", s.handleListBackupPolicies)
	mux.HandleFunc("/backup-policies/create", s.handleCreateBackupPolicy)
	mux.HandleFunc("/workflows/list", s.handleListWorkflows)

	// User Management & RBAC
	mux.HandleFunc("/users/list", s.handleListUsers)
	mux.HandleFunc("/users/create", s.handleCreateUser)
	mux.HandleFunc("/roles/list", s.handleListRoles)
	mux.HandleFunc("/api-keys/list", s.handleListAPIKeys)
	mux.HandleFunc("/api-keys/generate", s.handleGenerateAPIKey)
	mux.HandleFunc("/sessions/list", s.handleListSessions)

	// Notifications & Alerts
	mux.HandleFunc("/notifications/config", s.handleGetNotificationConfig)
	mux.HandleFunc("/notifications/update", s.handleUpdateNotificationConfig)
	mux.HandleFunc("/alert-rules/list", s.handleListAlertRules)
	mux.HandleFunc("/alert-rules/create", s.handleCreateAlertRule)
	mux.HandleFunc("/webhooks/test", s.handleTestWebhook)

	// Hyper2KVM Integration
	mux.HandleFunc("/convert/vm", s.handleConvertVM)
	mux.HandleFunc("/convert/list", s.handleListConversions)
	mux.HandleFunc("/convert/status", s.handleConversionStatus)
	mux.HandleFunc("/import/kvm", s.handleImportToKVM)
	mux.HandleFunc("/vmdk/parse", s.handleVMDKParser)

	// Cost Management
	mux.HandleFunc("/cost/summary", s.handleGetCostSummary)
	mux.HandleFunc("/cost/history", s.handleGetCostHistory)
	mux.HandleFunc("/budget/config", s.handleGetBudgetConfig)
	mux.HandleFunc("/budget/update", s.handleUpdateBudgetConfig)

	// Organization
	mux.HandleFunc("/tags/list", s.handleListTags)
	mux.HandleFunc("/tags/create", s.handleCreateTag)
	mux.HandleFunc("/collections/list", s.handleListCollections)
	mux.HandleFunc("/collections/create", s.handleCreateCollection)
	mux.HandleFunc("/searches/list", s.handleListSavedSearches)
	mux.HandleFunc("/searches/create", s.handleCreateSavedSearch)

	// Cloud & Integration
	mux.HandleFunc("/cloud/providers/list", s.handleListCloudProviders)
	mux.HandleFunc("/cloud/providers/configure", s.handleConfigureCloudProvider)
	mux.HandleFunc("/vcenter/servers/list", s.handleListVCenterServers)
	mux.HandleFunc("/vcenter/servers/add", s.handleAddVCenterServer)
	mux.HandleFunc("/integrations/list", s.handleListIntegrations)
	mux.HandleFunc("/integrations/configure", s.handleConfigureIntegration)

	// Security & Compliance
	mux.HandleFunc("/encryption/config", s.handleGetEncryptionConfig)
	mux.HandleFunc("/encryption/update", s.handleUpdateEncryptionConfig)
	mux.HandleFunc("/compliance/frameworks", s.handleListComplianceFrameworks)
	mux.HandleFunc("/audit/logs", s.handleGetAuditLogs)
	mux.HandleFunc("/audit/export", s.handleExportAuditLogs)

	// Migration
	mux.HandleFunc("/migration/wizard", s.handleMigrationWizard)
	mux.HandleFunc("/migration/compatibility", s.handleCompatibilityCheck)
	mux.HandleFunc("/migration/rollback", s.handleRollback)

	// Migration Validation & Testing
	mux.HandleFunc("/migration/validate", s.handleValidateMigration)
	mux.HandleFunc("/migration/verify", s.handleVerifyMigration)
	mux.HandleFunc("/migration/check-compatibility", s.handleCheckCompatibility)
	mux.HandleFunc("/migration/test", s.handleTestMigration)

	// Config Generation
	mux.HandleFunc("/config/generate", s.handleGenerateConfig)
	mux.HandleFunc("/config/templates", s.handleListConfigTemplates)

	// Libvirt Management
	mux.HandleFunc("/libvirt/domains", s.handleListLibvirtDomains)
	mux.HandleFunc("/libvirt/domain", s.handleGetLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/start", s.handleStartLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/shutdown", s.handleShutdownLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/destroy", s.handleDestroyLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/reboot", s.handleRebootLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/pause", s.handlePauseLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/resume", s.handleResumeLibvirtDomain)
	mux.HandleFunc("/libvirt/snapshots", s.handleListLibvirtSnapshots)
	mux.HandleFunc("/libvirt/snapshot/create", s.handleCreateLibvirtSnapshot)
	mux.HandleFunc("/libvirt/snapshot/revert", s.handleRevertLibvirtSnapshot)
	mux.HandleFunc("/libvirt/snapshot/delete", s.handleDeleteLibvirtSnapshot)
	mux.HandleFunc("/libvirt/pools", s.handleListLibvirtPools)
	mux.HandleFunc("/libvirt/volumes", s.handleListLibvirtVolumes)
	mux.HandleFunc("/libvirt/console", s.handleGetLibvirtConsole)

	// Network Management
	mux.HandleFunc("/libvirt/networks", s.handleListNetworks)
	mux.HandleFunc("/libvirt/network", s.handleGetNetwork)
	mux.HandleFunc("/libvirt/network/create", s.handleCreateNetwork)
	mux.HandleFunc("/libvirt/network/delete", s.handleDeleteNetwork)
	mux.HandleFunc("/libvirt/network/start", s.handleStartNetwork)
	mux.HandleFunc("/libvirt/network/stop", s.handleStopNetwork)
	mux.HandleFunc("/libvirt/interface/attach", s.handleAttachInterface)
	mux.HandleFunc("/libvirt/interface/detach", s.handleDetachInterface)

	// Volume Operations
	mux.HandleFunc("/libvirt/volume/info", s.handleGetVolumeInfo)
	mux.HandleFunc("/libvirt/volume/create", s.handleCreateVolume)
	mux.HandleFunc("/libvirt/volume/clone", s.handleCloneVolume)
	mux.HandleFunc("/libvirt/volume/resize", s.handleResizeVolume)
	mux.HandleFunc("/libvirt/volume/delete", s.handleDeleteVolume)
	mux.HandleFunc("/libvirt/volume/upload", s.handleUploadVolume)
	mux.HandleFunc("/libvirt/volume/wipe", s.handleWipeVolume)

	// Resource Monitoring
	mux.HandleFunc("/libvirt/stats", s.handleGetDomainStats)
	mux.HandleFunc("/libvirt/stats/all", s.handleGetAllDomainStats)
	mux.HandleFunc("/libvirt/stats/cpu", s.handleGetCPUStats)
	mux.HandleFunc("/libvirt/stats/memory", s.handleGetMemoryStats)
	mux.HandleFunc("/libvirt/stats/disk", s.handleGetDiskIOStats)
	mux.HandleFunc("/libvirt/stats/network", s.handleGetNetworkIOStats)

	// Batch Operations
	mux.HandleFunc("/libvirt/batch/start", s.handleBatchStart)
	mux.HandleFunc("/libvirt/batch/stop", s.handleBatchStop)
	mux.HandleFunc("/libvirt/batch/reboot", s.handleBatchReboot)
	mux.HandleFunc("/libvirt/batch/snapshot", s.handleBatchSnapshot)
	mux.HandleFunc("/libvirt/batch/delete", s.handleBatchDelete)
	mux.HandleFunc("/libvirt/batch/pause", s.handleBatchPause)
	mux.HandleFunc("/libvirt/batch/resume", s.handleBatchResume)

	// VM Cloning & Templates
	mux.HandleFunc("/libvirt/clone", s.handleCloneDomain)
	mux.HandleFunc("/libvirt/clone/multiple", s.handleCloneMultipleDomains)
	mux.HandleFunc("/libvirt/template/create", s.handleCreateTemplate)
	mux.HandleFunc("/libvirt/template/deploy", s.handleDeployFromTemplate)
	mux.HandleFunc("/libvirt/template/list", s.handleListTemplates)
	mux.HandleFunc("/libvirt/template/export", s.handleExportTemplate)

	// ISO Management
	mux.HandleFunc("/libvirt/isos/list", s.handleListISOs)
	mux.HandleFunc("/libvirt/isos/upload", s.handleUploadISO)
	mux.HandleFunc("/libvirt/isos/delete", s.handleDeleteISO)
	mux.HandleFunc("/libvirt/domain/attach-iso", s.handleAttachISO)
	mux.HandleFunc("/libvirt/domain/detach-iso", s.handleDetachISO)

	// Backup & Restore
	mux.HandleFunc("/libvirt/backup/create", s.handleCreateBackup)
	mux.HandleFunc("/libvirt/backup/list", s.handleListBackups)
	mux.HandleFunc("/libvirt/backup/restore", s.handleRestoreBackup)
	mux.HandleFunc("/libvirt/backup/verify", s.handleVerifyBackup)
	mux.HandleFunc("/libvirt/backup/delete", s.handleDeleteBackup)

	// Workflow daemon integration
	mux.HandleFunc("/api/workflow/status", s.WorkflowStatusHandler)
	mux.HandleFunc("/api/workflow/jobs", s.WorkflowJobsHandler)
	mux.HandleFunc("/api/workflow/jobs/active", s.WorkflowJobsActiveHandler)
	mux.HandleFunc("/api/workflow/manifest/submit", s.ManifestSubmitHandler)
	mux.HandleFunc("/api/workflow/manifest/validate", s.ManifestValidateHandler)

	// Incremental Export & Changed Block Tracking
	mux.HandleFunc("/cbt/enable", s.handleEnableCBT)
	mux.HandleFunc("/cbt/disable", s.handleDisableCBT)
	mux.HandleFunc("/cbt/status", s.handleCBTStatus)
	mux.HandleFunc("/incremental/analyze", s.handleIncrementalAnalysis)

	// Advanced Scheduling
	mux.HandleFunc("/schedules/advanced/create", s.handleCreateAdvancedSchedule)
	mux.HandleFunc("/schedules/dependencies", s.handleGetDependencyStatus)
	mux.HandleFunc("/schedules/retry", s.handleGetRetryStatus)
	mux.HandleFunc("/schedules/timewindow", s.handleGetTimeWindowStatus)
	mux.HandleFunc("/schedules/queue", s.handleGetJobQueueStatus)
	mux.HandleFunc("/schedules/validate", s.handleValidateSchedule)

	// Cost Estimation
	mux.HandleFunc("/cost/estimate", s.handleEstimateCost)
	mux.HandleFunc("/cost/compare", s.handleCompareProviders)
	mux.HandleFunc("/cost/project", s.handleProjectYearlyCost)
	mux.HandleFunc("/cost/estimate-size", s.handleEstimateExportSize)

	// Console & Display
	mux.HandleFunc("/console/info", s.handleGetConsoleInfo)
	mux.HandleFunc("/console/vnc", s.handleVNCProxy)
	mux.HandleFunc("/console/serial", s.handleSerialConsole)
	mux.HandleFunc("/console/serial-device", s.handleGetSerialDevice)
	mux.HandleFunc("/console/screenshot", s.handleScreenshot)
	mux.HandleFunc("/libvirt/domain/send-key", s.handleSendKeys)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("starting API server", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down API server")
	return s.httpServer.Shutdown(ctx)
}

// Middleware for logging
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start))
	})
}

// Health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Get daemon status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.manager.GetStatus()
	s.jsonResponse(w, http.StatusOK, status)
}

// Get available export capabilities
func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	caps := s.detector.GetCapabilities()
	defaultMethod := s.detector.GetDefaultMethod()

	response := map[string]interface{}{
		"capabilities":   caps,
		"default_method": defaultMethod,
		"timestamp":      time.Now(),
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// Submit job(s) from JSON, YAML, or file
func (s *Server) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")

	var defs []models.JobDefinition
	var err error

	// Parse based on content type
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "failed to read request body: %v", err)
		return
	}

	switch contentType {
	case "application/json":
		err = s.parseJSONJobs(body, &defs)
	case "application/x-yaml", "text/yaml":
		err = s.parseYAMLJobs(body, &defs)
	default:
		// Try JSON first, then YAML
		if err = s.parseJSONJobs(body, &defs); err != nil {
			err = s.parseYAMLJobs(body, &defs)
		}
	}

	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "failed to parse jobs: %v", err)
		return
	}

	// Submit jobs
	ids, errs := s.manager.SubmitBatch(defs)

	// Build response
	resp := models.SubmitResponse{
		JobIDs:    ids,
		Accepted:  len(ids),
		Rejected:  len(errs),
		Timestamp: time.Now(),
	}

	if len(errs) > 0 {
		resp.Errors = make([]string, len(errs))
		for i, err := range errs {
			resp.Errors[i] = err.Error()
		}
	}

	s.jsonResponse(w, http.StatusOK, resp)
}

// Query jobs
func (s *Server) handleQueryJobs(w http.ResponseWriter, r *http.Request) {
	var req models.QueryRequest

	// Support both GET (for dashboard) and POST methods
	switch r.Method {
	case http.MethodGet:
		// For GET requests, check for 'all' query parameter
		if r.URL.Query().Get("all") == "true" {
			// Return all jobs
			req = models.QueryRequest{All: true}
		} else {
			// Could support other query parameters here if needed
			req = models.QueryRequest{}
		}
	case http.MethodPost:
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var jobList []*models.Job

	if len(req.JobIDs) > 0 {
		// Get specific jobs
		for _, id := range req.JobIDs {
			job, err := s.manager.GetJob(id)
			if err == nil {
				jobList = append(jobList, job)
			}
		}
	} else if req.All {
		// Get all jobs
		jobList = s.manager.GetAllJobs()
	} else {
		// Filter by status
		jobList = s.manager.ListJobs(req.Status, req.Limit)
	}

	resp := models.QueryResponse{
		Jobs:      jobList,
		Total:     len(jobList),
		Timestamp: time.Now(),
	}

	s.jsonResponse(w, http.StatusOK, resp)
}

// Get specific job by ID
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path
	jobID := filepath.Base(r.URL.Path)
	if jobID == "jobs" || jobID == "" {
		s.errorResponse(w, http.StatusBadRequest, "job ID required")
		return
	}

	job, err := s.manager.GetJob(jobID)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "job not found: %s", jobID)
		return
	}

	s.jsonResponse(w, http.StatusOK, job)
}

// Cancel jobs
func (s *Server) handleCancelJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request: %v", err)
		return
	}

	resp := models.CancelResponse{
		Cancelled: []string{},
		Failed:    []string{},
		Errors:    make(map[string]string),
		Timestamp: time.Now(),
	}

	for _, jobID := range req.JobIDs {
		err := s.manager.CancelJob(jobID)
		if err != nil {
			resp.Failed = append(resp.Failed, jobID)
			resp.Errors[jobID] = err.Error()
		} else {
			resp.Cancelled = append(resp.Cancelled, jobID)
		}
	}

	s.jsonResponse(w, http.StatusOK, resp)
}

// Helper: parse JSON jobs
func (s *Server) parseJSONJobs(data []byte, defs *[]models.JobDefinition) error {
	// Try single job first
	var single models.JobDefinition
	if err := json.Unmarshal(data, &single); err == nil && single.VMPath != "" {
		*defs = append(*defs, single)
		return nil
	}

	// Try batch
	var batch models.BatchJobDefinition
	if err := json.Unmarshal(data, &batch); err == nil && len(batch.Jobs) > 0 {
		*defs = batch.Jobs
		return nil
	}

	// Try array
	var array []models.JobDefinition
	if err := json.Unmarshal(data, &array); err == nil && len(array) > 0 {
		*defs = array
		return nil
	}

	return fmt.Errorf("invalid JSON format")
}

// Helper: parse YAML jobs
func (s *Server) parseYAMLJobs(data []byte, defs *[]models.JobDefinition) error {
	// Try single job first
	var single models.JobDefinition
	if err := yaml.Unmarshal(data, &single); err == nil && single.VMPath != "" {
		*defs = append(*defs, single)
		return nil
	}

	// Try batch
	var batch models.BatchJobDefinition
	if err := yaml.Unmarshal(data, &batch); err == nil && len(batch.Jobs) > 0 {
		*defs = batch.Jobs
		return nil
	}

	// Try array
	var array []models.JobDefinition
	if err := yaml.Unmarshal(data, &array); err == nil && len(array) > 0 {
		*defs = array
		return nil
	}

	return fmt.Errorf("invalid YAML format")
}

// Helper: send JSON response
func (s *Server) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Warn("failed to encode JSON response", "error", err)
	}
}

// Helper: send error response
func (s *Server) errorResponse(w http.ResponseWriter, statusCode int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.logger.Warn("api error", "status", statusCode, "message", msg)
	s.jsonResponse(w, statusCode, map[string]string{
		"error":     msg,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// SubmitJobFromFile submits jobs from a YAML or JSON file
func SubmitJobFromFile(apiURL, filePath string) error {
	// Determine content type from extension
	contentType := "application/json"
	ext := filepath.Ext(filePath)
	if ext == ".yaml" || ext == ".yml" {
		contentType = "application/x-yaml"
	}

	// Send request
	// #nosec G304 -- filePath is supplied by the local CLI caller submitting a job definition, not remote/network input
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	req, err := http.NewRequest("POST", apiURL+"/jobs/submit", file)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	return nil
}
