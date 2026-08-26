// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/api"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/jobs"
	"github.com/zyvorai/transiva/daemon/scheduler"
	"github.com/zyvorai/transiva/daemon/store"
	"github.com/zyvorai/transiva/daemon/webhooks"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
	"github.com/zyvorai/transiva/providers/kubevirt"
	"github.com/zyvorai/transiva/providers/nutanix"
	"github.com/zyvorai/transiva/providers/proxmox"
	"github.com/zyvorai/transiva/providers/vsphere"
)

const (
	defaultAddr = "localhost:8080"
	version     = "0.0.1"
)

func main() {
	// Parse flags
	configFile := flag.String("config", "", "Path to config file (YAML)")
	addr := flag.String("addr", "", "API server address (overrides config file)")
	logLevel := flag.String("log-level", "", "Log level (debug, info, warn, error)")
	disableWeb := flag.Bool("disable-web", false, "Disable web dashboard (API-only mode)")
	versionFlag := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("hypervisord version %s\n", version)
		os.Exit(0)
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if *configFile != "" {
		// Load from file
		cfg, err = config.FromFile(*configFile)
		if err != nil {
			pterm.Error.Printfln("Failed to load config file: %v", err)
			os.Exit(1)
		}
		// Merge with environment variables (env takes precedence)
		cfg = cfg.MergeWithEnv()
		pterm.Info.Printfln("Loaded configuration from: %s", *configFile)
	} else {
		// Load from environment
		cfg = config.FromEnvironment()
	}

	// Override with flags if specified
	if *addr != "" {
		cfg.DaemonAddr = *addr
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}

	// Use defaults if still empty
	if cfg.DaemonAddr == "" {
		cfg.DaemonAddr = defaultAddr
	}

	// Show banner
	showBanner()

	// Setup logging
	log := logger.New(cfg.LogLevel)

	pterm.Info.Printfln("Starting hypervisord daemon v%s", version)
	pterm.Info.Printfln("API server will listen on: %s", cfg.DaemonAddr)

	// Create capability detector
	pterm.Info.Println("Detecting available export capabilities...")
	detector := capabilities.NewDetector(log)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := detector.Detect(ctx); err != nil {
		pterm.Warning.Printfln("Failed to detect capabilities: %v", err)
	}

	// Display detected capabilities
	showCapabilities(detector)

	// Create job manager with capability detector
	manager := jobs.NewManager(log, detector)

	// ===== PHASE 1-3 FEATURE INTEGRATION =====

	// Phase 1.1: Setup Connection Pooling for vSphere
	var connectionPool *vsphere.ConnectionPool
	if cfg.ConnectionPool != nil && cfg.ConnectionPool.Enabled {
		pterm.Info.Println("Initializing vSphere connection pool...")
		poolConfig := &vsphere.PoolConfig{
			MaxConnections:      cfg.ConnectionPool.MaxConnections,
			IdleTimeout:         cfg.ConnectionPool.IdleTimeout,
			HealthCheckInterval: cfg.ConnectionPool.HealthCheckInterval,
		}
		connectionPool = vsphere.NewConnectionPool(cfg, poolConfig, log)
		pterm.Success.Printfln("Connection pool enabled (max: %d connections)", poolConfig.MaxConnections)
	}

	// Phase 1.2: Setup Webhook Integration
	var webhookMgr *webhooks.Manager
	if len(cfg.Webhooks) > 0 {
		pterm.Info.Printfln("Configuring webhooks (%d endpoints)...", len(cfg.Webhooks))

		// Convert config webhooks to webhook manager format
		wh := make([]webhooks.Webhook, len(cfg.Webhooks))
		for i, w := range cfg.Webhooks {
			wh[i] = webhooks.Webhook{
				URL:     w.URL,
				Events:  w.Events,
				Headers: w.Headers,
				Timeout: w.Timeout,
				Retry:   w.Retry,
				Enabled: w.Enabled,
			}
		}

		webhookMgr = webhooks.NewManager(wh, log)
		manager.SetWebhookManager(webhookMgr)
		pterm.Success.Printfln("Webhooks enabled for job notifications")
	}

	// Phase 2.3: Setup Job Scheduling with Persistence
	var jobScheduler *scheduler.Scheduler
	var dbStore *store.SQLiteStore
	if cfg.DatabasePath != "" {
		pterm.Info.Printfln("Opening database: %s", cfg.DatabasePath)
		dbStore, err = store.NewSQLiteStore(cfg.DatabasePath)
		if err != nil {
			pterm.Error.Printfln("Failed to open database: %v", err)
			os.Exit(1)
		}
		pterm.Success.Println("Database initialized")

		// Create scheduler with persistence
		pterm.Info.Println("Initializing job scheduler...")
		jobScheduler = scheduler.NewScheduler(manager, log)
		jobScheduler.SetStore(dbStore)

		// Load existing schedules from database
		if err := jobScheduler.LoadSchedules(); err != nil {
			pterm.Warning.Printfln("Failed to load schedules: %v", err)
		} else {
			schedules := jobScheduler.ListScheduledJobs()
			pterm.Success.Printfln("Loaded %d scheduled jobs from database", len(schedules))
		}

		jobScheduler.Start()
	}

	// Phase 3: Setup Provider Registry
	pterm.Info.Println("Initializing provider registry...")
	providerRegistry := providers.NewRegistry()

	// Register all available providers
	providerRegistry.Register(providers.ProviderVSphere, func(cfg providers.ProviderConfig) (providers.Provider, error) {
		return vsphere.NewProvider(cfg, log)
	})

	// Register Proxmox provider (Phase 4.5 completion)
	providerRegistry.Register(providers.ProviderProxmox, func(cfg providers.ProviderConfig) (providers.Provider, error) {
		return proxmox.NewProvider(cfg, log)
	})

	// Register KubeVirt provider (Kubernetes integration)
	providerRegistry.Register(providers.ProviderKubeVirt, func(cfg providers.ProviderConfig) (providers.Provider, error) {
		return kubevirt.NewProvider(cfg, log)
	})

	// Register Nutanix provider (Prism Central v4 discovery)
	providerRegistry.Register(providers.ProviderNutanix, func(cfg providers.ProviderConfig) (providers.Provider, error) {
		return nutanix.NewProvider(cfg, log)
	})

	pterm.Success.Printfln("Provider registry initialized (%d providers)", len(providerRegistry.ListProviders()))

	// Create API config
	apiConfig := &api.Config{}
	apiConfig.Metrics.Enabled = false
	apiConfig.Security.EnableAuth = false // Disable auth for local development
	apiConfig.Web.Disabled = *disableWeb  // Set web dashboard state from CLI flag

	// Create Enhanced API server with Phase 2 features
	server, err := api.NewEnhancedServer(manager, detector, log, cfg.DaemonAddr, apiConfig, providerRegistry, cfg)
	if err != nil {
		pterm.Error.Printfln("Failed to create server: %v", err)
		os.Exit(1)
	}

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errCh <- err
		}
	}()

	pterm.Success.Printfln("Daemon started successfully")
	pterm.Info.Println("Waiting for jobs... (Press Ctrl+C to stop)")

	// Show API endpoints
	showEndpoints(cfg.DaemonAddr)

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		pterm.Warning.Printfln("Received signal: %v", sig)
		pterm.Info.Println("Shutting down gracefully...")

		// Graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown API server
		if err := server.Shutdown(ctx); err != nil {
			pterm.Error.Printfln("Server shutdown error: %v", err)
		}

		// Shutdown scheduler
		if jobScheduler != nil {
			pterm.Info.Println("Stopping job scheduler...")
			jobScheduler.Stop()
		}

		// Shutdown job manager
		pterm.Info.Println("Stopping job manager...")
		manager.Shutdown()

		// Close connection pool
		if connectionPool != nil {
			pterm.Info.Println("Closing connection pool...")
			if err := connectionPool.Close(); err != nil {
				pterm.Error.Printfln("Connection pool close error: %v", err)
			}
			stats := connectionPool.Stats()
			pterm.Info.Printfln("Pool stats - Created: %d, Reused: %d, Ratio: %.2f%%",
				stats["total_created"],
				stats["total_reused"],
				stats["reuse_ratio"].(float64)*100)
		}

		// Close database
		if dbStore != nil {
			pterm.Info.Println("Closing database...")
			if err := dbStore.Close(); err != nil {
				pterm.Error.Printfln("Database close error: %v", err)
			}
		}

		pterm.Success.Println("Daemon stopped gracefully")

	case err := <-errCh:
		pterm.Error.Printfln("Server error: %v", err)
		// Cleanup on error
		if connectionPool != nil {
			if closeErr := connectionPool.Close(); closeErr != nil {
				pterm.Error.Printfln("Connection pool close error: %v", closeErr)
			}
		}
		if dbStore != nil {
			if closeErr := dbStore.Close(); closeErr != nil {
				pterm.Error.Printfln("Database close error: %v", closeErr)
			}
		}
		os.Exit(1)
	}
}

func showBanner() {
	pterm.DefaultCenter.Println()

	// Orange/amber color scheme (Claude-inspired)
	orange := pterm.NewStyle(pterm.FgLightRed)
	amber := pterm.NewStyle(pterm.FgYellow)

	bigText, _ := pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("HYPER", orange),
		putils.LettersFromStringWithStyle("VISOR", amber),
		putils.LettersFromStringWithStyle("D", orange),
	).Srender()

	pterm.DefaultCenter.Println(bigText)

	subtitle := pterm.DefaultCenter.Sprint(pterm.LightYellow("Multi-cloud VM export daemon"))
	pterm.Println(subtitle)
	pterm.Println()
}

func showCapabilities(detector *capabilities.Detector) {
	caps := detector.GetCapabilities()
	defaultMethod := detector.GetDefaultMethod()

	capData := [][]string{
		{"Method", "Available", "Priority", "Path"},
	}

	// Priority order: ctl, govc, ovftool, web
	methods := []capabilities.ExportMethod{
		capabilities.ExportMethodCTL,
		capabilities.ExportMethodGovc,
		capabilities.ExportMethodOvftool,
		capabilities.ExportMethodWeb,
	}

	for _, method := range methods {
		if cap, ok := caps[method]; ok {
			available := "✗"
			if cap.Available {
				available = "✓"
			}
			isDefault := ""
			if method == defaultMethod {
				isDefault = " (default)"
			}
			capData = append(capData, []string{
				string(method) + isDefault,
				available,
				fmt.Sprintf("%d", cap.Priority),
				cap.Path,
			})
		}
	}

	pterm.DefaultSection.Println("Export Capabilities")
	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderRowSeparator("-").
		WithBoxed().
		WithData(capData).
		Render(); err != nil {
		pterm.Error.Printfln("failed to render capabilities table: %v", err)
	}

	pterm.Success.Printfln("Default export method: %s", defaultMethod)
	pterm.Println()
}

func showEndpoints(addr string) {
	baseURL := fmt.Sprintf("http://%s", addr)

	endpoints := [][]string{
		{"Endpoint", "Method", "Description"},
		{baseURL + "/", "GET", "Web Dashboard (redirect)"},
		{baseURL + "/web/dashboard/", "GET", "Web Dashboard UI"},
		{baseURL + "/ws", "WS", "WebSocket (real-time updates)"},
		{baseURL + "/health", "GET", "Health check"},
		{baseURL + "/status", "GET", "Daemon status"},
		{baseURL + "/capabilities", "GET", "Export capabilities"},
		{baseURL + "/jobs/submit", "POST", "Submit job(s) (JSON/YAML)"},
		{baseURL + "/jobs/query", "POST", "Query jobs"},
		{baseURL + "/jobs/{id}", "GET", "Get specific job"},
		{baseURL + "/jobs/cancel", "POST", "Cancel job(s)"},
		{baseURL + "/schedules", "GET/POST", "Manage schedules"},
		{baseURL + "/webhooks", "GET/POST", "Manage webhooks"},
		{baseURL + "/vms/list", "GET", "List discovered VMs"},
		{baseURL + "/vms/export?provider=nutanix", "POST", "Export Nutanix VM via NFS pickup"},
		{baseURL + "/vms/info?provider=nutanix&vm={name|uuid}", "GET", "Get Nutanix VM details"},
		{baseURL + "/providers/list", "GET", "List registered providers"},
		{baseURL + "/providers/capabilities?provider=nutanix", "GET", "Provider export capabilities"},
	}

	pterm.DefaultSection.Println("Available API Endpoints")
	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderRowSeparator("-").
		WithBoxed().
		WithData(endpoints).
		Render(); err != nil {
		pterm.Error.Printfln("failed to render endpoints table: %v", err)
	}

	pterm.Info.Printfln("\n📊 Open dashboard in browser: %s/web/dashboard/", baseURL)
}
