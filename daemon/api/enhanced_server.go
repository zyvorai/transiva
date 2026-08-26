// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bufio"
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/zyvorai/transiva/daemon/auth"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/metrics"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/daemon/scheduler"
	"github.com/zyvorai/transiva/daemon/store"
	"github.com/zyvorai/transiva/daemon/webhooks"
	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// Config holds the configuration for the enhanced server
type Config struct {
	Webhooks []webhooks.Webhook `yaml:"webhooks" json:"webhooks"`
	Database struct {
		Path string `yaml:"path" json:"path"`
	} `yaml:"database" json:"database"`
	Metrics struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
		Port    int  `yaml:"port" json:"port"`
	} `yaml:"metrics" json:"metrics"`
	Web struct {
		Disabled bool `yaml:"disabled" json:"disabled"` // Disable web dashboard (default: false, web enabled)
	} `yaml:"web" json:"web"`
	Security struct {
		APIKey           string   `yaml:"api_key" json:"-"`                       // API key for authentication
		AllowedOrigins   []string `yaml:"allowed_origins" json:"allowed_origins"` // Allowed WebSocket origins
		MaxRequestSizeMB int      `yaml:"max_request_size_mb" json:"max_request_size_mb"`
		RateLimitPerMin  int      `yaml:"rate_limit_per_min" json:"rate_limit_per_min"`
		EnableAuth       bool     `yaml:"enable_auth" json:"enable_auth"`
		TrustedProxies   []string `yaml:"trusted_proxies" json:"trusted_proxies"`
		BlockPrivateIPs  bool     `yaml:"block_private_ips" json:"block_private_ips"` // Block private IPs in webhooks
	} `yaml:"security" json:"security"`
}

// EnhancedServer extends the base server with new features
type EnhancedServer struct {
	*Server
	scheduler      *scheduler.Scheduler
	webhookMgr     *webhooks.Manager
	store          store.JobStore
	config         *Config
	wsHub          *WSHub
	statusTicker   *time.Ticker
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	authMgr        *auth.AuthManager
	systemMetrics  *metrics.SystemMetrics
}

// jobExecutorAdapter adapts jobs.Manager to scheduler.JobExecutor interface
type jobExecutorAdapter struct {
	manager *jobs.Manager
}

func (a *jobExecutorAdapter) SubmitJob(definition models.JobDefinition) (string, error) {
	return a.manager.SubmitJob(definition)
}

// NewEnhancedServer creates a new enhanced API server with all Phase 1 features
func NewEnhancedServer(manager *jobs.Manager, detector *capabilities.Detector, log logger.Logger, addr string, config *Config, providerRegistry *providers.Registry, appConfig *config.Config) (*EnhancedServer, error) {
	// Apply default security settings if not configured
	if config.Security.MaxRequestSizeMB == 0 {
		config.Security.MaxRequestSizeMB = 10 // 10MB default
	}
	if len(config.Security.AllowedOrigins) == 0 {
		// Default to localhost for development
		config.Security.AllowedOrigins = []string{
			"http://localhost:8080",
			"https://localhost:8080",
		}
	}
	if config.Security.BlockPrivateIPs {
		log.Info("webhook private IP blocking enabled")
	}

	// Create base server
	baseServer := NewServer(manager, detector, log, addr, providerRegistry, appConfig)
	manager.SetProviderContext(providerRegistry, appConfig)

	// Create shutdown context
	ctx, cancel := context.WithCancel(context.Background())

	es := &EnhancedServer{
		Server:         baseServer,
		config:         config,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
		systemMetrics:  metrics.NewSystemMetrics(),
	}

	// Initialize job store if database path is provided
	if config.Database.Path != "" {
		jobStore, err := store.NewSQLiteStore(config.Database.Path)
		if err != nil {
			log.Error("failed to initialize job store", "error", err)
		} else {
			es.store = jobStore
			log.Info("job persistence enabled", "dbPath", config.Database.Path)
		}
	}

	// Initialize webhook manager if webhooks array exists (even if empty)
	if config.Webhooks != nil {
		es.webhookMgr = webhooks.NewManager(config.Webhooks, log)
		if len(config.Webhooks) > 0 {
			log.Info("webhooks enabled", "count", len(config.Webhooks))
		}
	}

	// Initialize scheduler
	executor := &jobExecutorAdapter{manager: manager}
	es.scheduler = scheduler.NewScheduler(executor, log)
	es.scheduler.Start()
	log.Info("job scheduler enabled")

	// Set build info for metrics
	metrics.SetBuildInfo("0.0.1", "go1.24")

	// Initialize WebSocket hub with shutdown context
	es.wsHub = NewWSHub()
	es.wsHub.SetLogger(log)
	es.wsHub.SetOnDisconnect(func() {
		es.systemMetrics.RecordWSDisconnection()
	})
	go es.wsHub.Run(es.shutdownCtx)
	es.statusTicker = es.StartStatusBroadcaster(es.shutdownCtx)
	log.Info("websocket support enabled")

	// Initialize authentication manager
	es.authMgr = auth.NewAuthManager()
	log.Info("authentication enabled")

	// Register enhanced routes
	es.registerEnhancedRoutes()

	return es, nil
}

// registerEnhancedRoutes adds new API endpoints for Phase 1 features
func (es *EnhancedServer) registerEnhancedRoutes() {
	mux := http.NewServeMux()

	// Authentication endpoints (no auth required)
	mux.HandleFunc("/api/login", es.handleLogin)
	mux.HandleFunc("/api/logout", es.handleLogout)

	// Copy existing routes from base server
	mux.HandleFunc("/health", es.handleHealth)
	mux.HandleFunc("/status", es.handleStatus)
	mux.HandleFunc("/capabilities", es.handleCapabilities)
	mux.HandleFunc("/jobs/submit", es.handleSubmitJob)
	mux.HandleFunc("/jobs/query", es.handleQueryJobs)
	mux.HandleFunc("/jobs/cancel", es.handleCancelJobs)
	mux.HandleFunc("/jobs/", es.handleGetJob)

	// VM discovery and management
	mux.HandleFunc("/vms/list", es.handleListVMs)
	mux.HandleFunc("/vms/export", es.handleExportVM)
	mux.HandleFunc("/vms/info", es.handleVMInfo)
	mux.HandleFunc("/providers/list", es.handleListProviders)
	mux.HandleFunc("/providers/capabilities", es.handleProviderCapabilities)
	mux.HandleFunc("/vms/shutdown", es.handleVMShutdown)
	mux.HandleFunc("/vms/poweroff", es.handleVMPowerOff)
	mux.HandleFunc("/vms/remove-cdrom", es.handleVMRemoveCDROM)

	// Libvirt management (copied from base server)
	mux.HandleFunc("/libvirt/domains", es.handleListLibvirtDomains)
	mux.HandleFunc("/libvirt/domain", es.handleGetLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/start", es.handleStartLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/shutdown", es.handleShutdownLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/destroy", es.handleDestroyLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/reboot", es.handleRebootLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/pause", es.handlePauseLibvirtDomain)
	mux.HandleFunc("/libvirt/domain/resume", es.handleResumeLibvirtDomain)
	mux.HandleFunc("/libvirt/snapshots", es.handleListLibvirtSnapshots)
	mux.HandleFunc("/libvirt/snapshot/create", es.handleCreateLibvirtSnapshot)
	mux.HandleFunc("/libvirt/snapshot/revert", es.handleRevertLibvirtSnapshot)
	mux.HandleFunc("/libvirt/snapshot/delete", es.handleDeleteLibvirtSnapshot)
	mux.HandleFunc("/libvirt/pools", es.handleListLibvirtPools)
	mux.HandleFunc("/libvirt/volumes", es.handleListLibvirtVolumes)
	mux.HandleFunc("/libvirt/console", es.handleGetLibvirtConsole)

	// Network management
	mux.HandleFunc("/libvirt/networks", es.handleListNetworks)
	mux.HandleFunc("/libvirt/network", es.handleGetNetwork)
	mux.HandleFunc("/libvirt/network/create", es.handleCreateNetwork)
	mux.HandleFunc("/libvirt/network/delete", es.handleDeleteNetwork)
	mux.HandleFunc("/libvirt/network/start", es.handleStartNetwork)
	mux.HandleFunc("/libvirt/network/stop", es.handleStopNetwork)
	mux.HandleFunc("/libvirt/interface/attach", es.handleAttachInterface)
	mux.HandleFunc("/libvirt/interface/detach", es.handleDetachInterface)

	// Volume operations
	mux.HandleFunc("/libvirt/volume/info", es.handleGetVolumeInfo)
	mux.HandleFunc("/libvirt/volume/create", es.handleCreateVolume)
	mux.HandleFunc("/libvirt/volume/clone", es.handleCloneVolume)
	mux.HandleFunc("/libvirt/volume/resize", es.handleResizeVolume)
	mux.HandleFunc("/libvirt/volume/delete", es.handleDeleteVolume)
	mux.HandleFunc("/libvirt/volume/upload", es.handleUploadVolume)
	mux.HandleFunc("/libvirt/volume/wipe", es.handleWipeVolume)

	// Resource monitoring
	mux.HandleFunc("/libvirt/stats", es.handleGetDomainStats)
	mux.HandleFunc("/libvirt/stats/all", es.handleGetAllDomainStats)
	mux.HandleFunc("/libvirt/stats/cpu", es.handleGetCPUStats)
	mux.HandleFunc("/libvirt/stats/memory", es.handleGetMemoryStats)
	mux.HandleFunc("/libvirt/stats/disk", es.handleGetDiskIOStats)
	mux.HandleFunc("/libvirt/stats/network", es.handleGetNetworkIOStats)

	// Batch operations
	mux.HandleFunc("/libvirt/batch/start", es.handleBatchStart)
	mux.HandleFunc("/libvirt/batch/stop", es.handleBatchStop)
	mux.HandleFunc("/libvirt/batch/reboot", es.handleBatchReboot)
	mux.HandleFunc("/libvirt/batch/snapshot", es.handleBatchSnapshot)
	mux.HandleFunc("/libvirt/batch/delete", es.handleBatchDelete)
	mux.HandleFunc("/libvirt/batch/pause", es.handleBatchPause)
	mux.HandleFunc("/libvirt/batch/resume", es.handleBatchResume)

	// VM cloning & templates
	mux.HandleFunc("/libvirt/clone", es.handleCloneDomain)
	mux.HandleFunc("/libvirt/clone/multiple", es.handleCloneMultipleDomains)
	mux.HandleFunc("/libvirt/template/create", es.handleCreateTemplate)
	mux.HandleFunc("/libvirt/template/deploy", es.handleDeployFromTemplate)
	mux.HandleFunc("/libvirt/template/list", es.handleListTemplates)
	mux.HandleFunc("/libvirt/template/export", es.handleExportTemplate)

	// ISO management
	mux.HandleFunc("/libvirt/isos/list", es.handleListISOs)
	mux.HandleFunc("/libvirt/isos/upload", es.handleUploadISO)
	mux.HandleFunc("/libvirt/isos/delete", es.handleDeleteISO)
	mux.HandleFunc("/libvirt/domain/attach-iso", es.handleAttachISO)
	mux.HandleFunc("/libvirt/domain/detach-iso", es.handleDetachISO)

	// Backup & restore
	mux.HandleFunc("/libvirt/backup/create", es.handleCreateBackup)
	mux.HandleFunc("/libvirt/backup/list", es.handleListBackups)
	mux.HandleFunc("/libvirt/backup/restore", es.handleRestoreBackup)
	mux.HandleFunc("/libvirt/backup/verify", es.handleVerifyBackup)
	mux.HandleFunc("/libvirt/backup/delete", es.handleDeleteBackup)

	// Workflow daemon integration (manifest & disk workflows)
	mux.HandleFunc("/api/workflow/status", es.WorkflowStatusHandler)
	mux.HandleFunc("/api/workflow/jobs", es.WorkflowJobsHandler)
	mux.HandleFunc("/api/workflow/jobs/active", es.WorkflowJobsActiveHandler)
	mux.HandleFunc("/api/workflow/manifest/submit", es.ManifestSubmitHandler)
	mux.HandleFunc("/api/workflow/manifest/validate", es.ManifestValidateHandler)

	// Console & display
	mux.HandleFunc("/console/info", es.handleGetConsoleInfo)
	mux.HandleFunc("/console/vnc", es.handleVNCProxy)
	mux.HandleFunc("/console/serial", es.handleSerialConsole)
	mux.HandleFunc("/console/serial-device", es.handleGetSerialDevice)
	mux.HandleFunc("/console/screenshot", es.handleScreenshot)
	mux.HandleFunc("/libvirt/domain/send-key", es.handleSendKeys)

	// Job progress tracking
	mux.HandleFunc("/jobs/progress/", es.handleGetJobProgress)
	mux.HandleFunc("/jobs/logs/", es.handleGetJobLogs)
	mux.HandleFunc("/jobs/eta/", es.handleGetJobETA)

	// Prometheus metrics endpoint
	if es.config.Metrics.Enabled {
		mux.Handle("/metrics", promhttp.Handler())
	}

	// WebSocket endpoint for real-time updates
	mux.HandleFunc("/ws", es.handleWebSocket)

	// Serve React dashboard (static files) - can be disabled via config or --disable-web flag
	// By default (web.disabled=false), React dashboard is enabled
	if !es.config.Web.Disabled {
		// React dashboard (production build)
		reactDir := filepath.Join(".", "daemon", "dashboard", "static-react")
		reactFileServer := http.FileServer(http.Dir(reactDir))
		mux.Handle("/web/dashboard/", http.StripPrefix("/web/dashboard/", reactFileServer))

		// Redirect root to React dashboard
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/web/dashboard/", http.StatusFound)
				return
			}
			http.NotFound(w, r)
		})
		es.logger.Info("React web dashboard enabled", "url", "/web/dashboard/")
	} else {
		// Web dashboard disabled (API-only mode)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
		es.logger.Info("web dashboard disabled - API-only mode")
	}

	// Schedule management endpoints
	mux.HandleFunc("/schedules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			es.handleListSchedules(w, r)
		case http.MethodPost:
			es.handleCreateSchedule(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/schedules/stats", es.handleScheduleStats)
	mux.HandleFunc("/schedules/", func(w http.ResponseWriter, r *http.Request) {
		// Handle different operations on specific schedules
		if r.URL.Path == "/schedules/" {
			http.Error(w, "schedule ID required", http.StatusBadRequest)
			return
		}

		// Check for sub-actions
		if strings.HasSuffix(r.URL.Path, "/enable") {
			es.handleEnableSchedule(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/disable") {
			es.handleDisableSchedule(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/trigger") {
			es.handleTriggerSchedule(w, r)
		} else {
			switch r.Method {
			case http.MethodGet:
				es.handleGetSchedule(w, r)
			case http.MethodPut:
				es.handleUpdateSchedule(w, r)
			case http.MethodDelete:
				es.handleDeleteSchedule(w, r)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})

	// Webhook management endpoints
	mux.HandleFunc("/webhooks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			es.handleListWebhooks(w, r)
		case http.MethodPost:
			es.handleAddWebhook(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/webhooks/test", es.handleTestWebhook)
	mux.HandleFunc("/webhooks/", es.handleDeleteWebhook)

	// vSphere Infrastructure Management
	mux.HandleFunc("/vsphere/hosts", es.handleListHosts)
	mux.HandleFunc("/vsphere/clusters", es.handleListClusters)
	mux.HandleFunc("/vsphere/datacenters", es.handleListDatacenters)
	mux.HandleFunc("/vsphere/vcenter/info", es.handleGetVCenterInfo)

	// vSphere Performance Metrics
	mux.HandleFunc("/vsphere/metrics", es.handleGetMetrics)
	mux.HandleFunc("/vsphere/metrics/stream", es.handleMetricsStream) // WebSocket

	// vSphere Resource Pools
	mux.HandleFunc("/vsphere/pools", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			es.handleListResourcePools(w, r)
		case http.MethodPost:
			es.handleCreateResourcePool(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/vsphere/pools/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			es.handleUpdateResourcePool(w, r)
		case http.MethodDelete:
			es.handleDeleteResourcePool(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// vSphere Events & Monitoring
	mux.HandleFunc("/vsphere/events", es.handleGetRecentEvents)
	mux.HandleFunc("/vsphere/events/stream", es.handleEventsStream) // WebSocket
	mux.HandleFunc("/vsphere/tasks", es.handleGetRecentTasks)
	mux.HandleFunc("/vsphere/tasks/stream", es.handleTasksStream) // WebSocket

	// vSphere VM Cloning & Templates
	mux.HandleFunc("/vsphere/clone", es.handleCloneVM)
	mux.HandleFunc("/vsphere/clone/bulk", es.handleBulkClone)
	mux.HandleFunc("/vsphere/template/create", es.handleCreateTemplate)
	mux.HandleFunc("/vsphere/template/deploy", es.handleDeployFromTemplate)

	// Create a wrapper handler that bypasses middleware for WebSocket endpoint
	wsHandler := http.HandlerFunc(es.handleWebSocket)

	// Update the HTTP server with middleware chain
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass all middleware for WebSocket connections
		if r.URL.Path == "/ws" ||
		   r.URL.Path == "/vsphere/metrics/stream" ||
		   r.URL.Path == "/vsphere/events/stream" ||
		   r.URL.Path == "/vsphere/tasks/stream" {
			wsHandler.ServeHTTP(w, r)
			return
		}

		// Apply middleware chain for all other requests
		h := es.loggingMiddleware(mux)
		h = es.requestSizeLimitMiddleware(h)
		h = es.authMiddleware(h)
		h.ServeHTTP(w, r)
	})

	es.httpServer.Handler = handler
}

// Shutdown gracefully shuts down the enhanced server
func (es *EnhancedServer) Shutdown(ctx context.Context) error {
	es.logger.Info("shutting down enhanced API server")

	// Cancel shutdown context to stop background goroutines
	if es.shutdownCancel != nil {
		es.shutdownCancel()
	}

	// Stop status broadcaster ticker
	if es.statusTicker != nil {
		es.statusTicker.Stop()
		es.logger.Debug("status broadcaster stopped")
	}

	// Stop scheduler
	if es.scheduler != nil {
		es.scheduler.Stop()
		es.logger.Debug("scheduler stopped")
	}

	// Close WebSocket hub (close all client connections)
	if es.wsHub != nil {
		es.wsHub.Shutdown()
		es.logger.Debug("websocket hub shutdown")
	}

	// Close store
	if es.store != nil {
		if err := es.store.Close(); err != nil {
			es.logger.Error("failed to close job store", "error", err)
		} else {
			es.logger.Debug("job store closed")
		}
	}

	// Shutdown HTTP server with timeout
	es.logger.Info("shutting down HTTP server")
	if err := es.httpServer.Shutdown(ctx); err != nil {
		es.logger.Error("HTTP server shutdown error", "error", err)
		return err
	}

	es.logger.Info("enhanced API server shutdown complete")
	return nil
}

// authMiddleware checks API authentication
func (es *EnhancedServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login/logout endpoints and static files
		if r.URL.Path == "/api/login" || r.URL.Path == "/health" ||
			r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/web/dashboard/") {
			next.ServeHTTP(w, r)
			return
		}

		// Try session token authentication first
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			_, err := es.authMgr.ValidateSession(token)
			if err == nil {
				// Valid session token
				next.ServeHTTP(w, r)
				return
			}
		}

		// Fall back to API key authentication if configured
		if es.config.Security.EnableAuth && es.config.Security.APIKey != "" {
			// Get API key from header or query parameter
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apiKey = r.Header.Get("Authorization")
				apiKey = strings.TrimPrefix(apiKey, "Bearer ")
			}
			if apiKey == "" {
				apiKey = r.URL.Query().Get("api_key")
			}

			// Validate API key using constant-time comparison to prevent timing attacks
			if apiKey != "" && subtle.ConstantTimeCompare([]byte(apiKey), []byte(es.config.Security.APIKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
		}

		// If auth is not enabled and no valid session, allow through
		if !es.config.Security.EnableAuth {
			next.ServeHTTP(w, r)
			return
		}

		// Unauthorized
		es.logger.Warn("unauthorized API access attempt",
			"remote", r.RemoteAddr,
			"path", r.URL.Path,
			"method", r.Method)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// requestSizeLimitMiddleware limits request body size
func (es *EnhancedServer) requestSizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			maxBytes := int64(es.config.Security.MaxRequestSizeMB) * 1024 * 1024
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// Enhanced middleware with metrics
func (es *EnhancedServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		durationSeconds := duration.Seconds()

		// Log request
		es.logger.Debug("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", durationSeconds)

		// Record metrics
		metrics.RecordAPIRequest(r.Method, r.URL.Path, http.StatusText(rw.statusCode), durationSeconds)

		// Record system metrics
		es.systemMetrics.RecordHTTPRequest()
		es.systemMetrics.RecordResponseTime(duration)
		if rw.statusCode >= 400 {
			es.systemMetrics.RecordHTTPError()
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support hijacking")
}
