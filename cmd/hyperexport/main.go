// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pterm/pterm"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/common"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// Command-line flags
var (
	vmName         = flag.String("vm", "", "VM name to export (skips interactive selection)")
	providerType   = flag.String("provider", "vsphere", "Provider type (vsphere, nutanix, aws, azure, gcp, hyperv)")
	outputDir      = flag.String("output", "", "Output directory (default: ./export-<vmname>)")
	format         = flag.String("format", "ovf", "Export format: ovf or ova")
	compress       = flag.Bool("compress", false, "Enable compression for OVA exports")
	verify         = flag.Bool("verify", false, "Verify export with checksum validation")
	dryRun         = flag.Bool("dry-run", false, "Preview export without actually exporting")
	batchFile      = flag.String("batch", "", "File containing list of VMs to export (one per line)")
	filter         = flag.String("filter", "", "Filter VMs by tag (format: key=value)")
	folder         = flag.String("folder", "", "Filter VMs by folder path")
	powerOff       = flag.Bool("power-off", false, "Automatically power off VM before export")
	parallel       = flag.Int("parallel", 4, "Number of parallel downloads")
	quiet          = flag.Bool("quiet", false, "Minimal output (for scripting)")
	showVersion    = flag.Bool("version", false, "Show version and exit")
	interactive    = flag.Bool("interactive", false, "Launch advanced interactive TUI mode")
	tui            = flag.Bool("tui", false, "Launch advanced interactive TUI mode (alias for -interactive)")
	validateOnly   = flag.Bool("validate-only", false, "Only run pre-export validation checks")
	vmInfo         = flag.Bool("vm-info", false, "Display VM information (power state, CPU, memory, storage)")
	showHistory    = flag.Bool("history", false, "Show export history")
	historyLimit   = flag.Int("history-limit", 10, "Number of recent exports to show in history")
	generateReport = flag.Bool("report", false, "Generate export statistics report")
	reportFile     = flag.String("report-file", "", "Save report to file instead of stdout")
	clearHistory   = flag.Bool("clear-history", false, "Clear export history")
	uploadTo       = flag.String("upload", "", "Upload export to cloud storage (s3://bucket/path, azure://container/path, gs://bucket/path, sftp://host/path)")
	keepLocal      = flag.Bool("keep-local", true, "Keep local copy after cloud upload")
	encrypt        = flag.Bool("encrypt", false, "Encrypt export files")
	encryptMethod  = flag.String("encrypt-method", "aes256", "Encryption method: aes256 or gpg")
	passphrase     = flag.String("passphrase", "", "Encryption passphrase")
	keyFile        = flag.String("keyfile", "", "Encryption key file")
	gpgRecipient   = flag.String("gpg-recipient", "", "GPG recipient email for encryption")
	profile        = flag.String("profile", "", "Use saved export profile")
	listProfiles   = flag.Bool("list-profiles", false, "List available profiles")
	deleteProfile  = flag.String("delete-profile", "", "Delete a saved profile")
	createDefaults = flag.Bool("create-default-profiles", false, "Create default profiles")

	// Artifact Manifest v1.0 options
	generateManifest     = flag.Bool("manifest", false, "Generate Artifact Manifest v1.0 for h2kvm")
	verifyManifestFlag   = flag.Bool("verify-manifest", false, "Verify manifest after generation")
	manifestChecksum     = flag.Bool("manifest-checksum", true, "Compute SHA-256 checksums for disks in manifest")
	manifestTargetFormat = flag.String("manifest-target", "qcow2", "Target disk format for conversion (qcow2, raw, vdi)")

	// Pipeline integration options
	enablePipeline  = flag.Bool("pipeline", false, "Enable h2kvm pipeline after export")
	h2kvmPath       = flag.String("h2kvm-path", "/home/tt/h2kvm/h2kvmctl", "Path to h2kvm executable")
	pipelineTimeout = flag.Duration("pipeline-timeout", 30*time.Minute, "Timeout for pipeline execution")
	streamPipeline  = flag.Bool("stream-pipeline", true, "Stream h2kvm output to console")
	pipelineDryRun  = flag.Bool("pipeline-dry-run", false, "Run pipeline in dry-run mode (no modifications)")

	// Pipeline stage configuration
	pipelineInspect  = flag.Bool("pipeline-inspect", true, "Enable INSPECT stage (collect guest info)")
	pipelineFix      = flag.Bool("pipeline-fix", true, "Enable FIX stage (fix fstab, grub, initramfs)")
	pipelineConvert  = flag.Bool("pipeline-convert", true, "Enable CONVERT stage (convert to qcow2)")
	pipelineValidate = flag.Bool("pipeline-validate", true, "Enable VALIDATE stage (check image integrity)")
	pipelineCompress = flag.Bool("pipeline-compress", true, "Enable qcow2 compression")
	compressLevel    = flag.Int("compress-level", 6, "qcow2 compression level 1-9")

	// Libvirt integration options
	libvirtIntegration = flag.Bool("libvirt", false, "Define VM in libvirt after conversion")
	libvirtURI         = flag.String("libvirt-uri", "qemu:///system", "Libvirt connection URI")
	libvirtAutoStart   = flag.Bool("libvirt-autostart", false, "Enable VM auto-start in libvirt")
	libvirtBridge      = flag.String("libvirt-bridge", "virbr0", "Network bridge for VM")
	libvirtPool        = flag.String("libvirt-pool", "default", "Storage pool for disks")

	// h2kvm daemon options
	h2kvmDaemon        = flag.Bool("h2kvm-daemon", false, "Use systemd daemon instead of direct execution")
	h2kvmInstance      = flag.String("h2kvm-instance", "", "Systemd instance name (e.g., 'vsphere-prod' for h2kvm@vsphere-prod.service)")
	h2kvmWatchDir      = flag.String("h2kvm-watch-dir", "/var/lib/h2kvm/queue", "Daemon watch directory")
	h2kvmOutputDir     = flag.String("h2kvm-output-dir", "/var/lib/h2kvm/output", "Daemon output directory")
	h2kvmPollInterval  = flag.Int("h2kvm-poll-interval", 5, "Poll interval in seconds (daemon mode)")
	h2kvmDaemonTimeout = flag.Int("h2kvm-daemon-timeout", 60, "Daemon processing timeout in minutes")

	// Phase 6: Orchestration & Monitoring options
	enableOrchestration = flag.Bool("orchestrate", false, "Enable Phase 6 migration orchestration")
	progressAPIPort     = flag.String("progress-api", "", "Enable progress API server (e.g., ':8080')")
	metricsAPIPort      = flag.String("metrics-api", "", "Enable Prometheus metrics API (e.g., ':9090')")
	auditLogPath        = flag.String("audit-log", "", "Enable audit logging to file (e.g., '/var/log/transiva/audit.log')")
	webhookURL          = flag.String("webhook-url", "", "Send webhook notifications to URL")
	webhookType         = flag.String("webhook-type", "generic", "Webhook type: slack, discord, or generic")
	webhookOnStart      = flag.Bool("webhook-on-start", true, "Send webhook on migration start")
	webhookOnComplete   = flag.Bool("webhook-on-complete", true, "Send webhook on migration complete")
	webhookOnError      = flag.Bool("webhook-on-error", true, "Send webhook on migration error")

	// Daemon integration options
	useDaemon      = flag.Bool("use-daemon", false, "Submit export job to daemon instead of running directly")
	daemonURL      = flag.String("daemon-url", defaultDaemonURL, "Daemon URL (default: http://localhost:8080)")
	daemonWatch    = flag.String("daemon-watch", "", "Watch a daemon job by ID (e.g., --daemon-watch job-123)")
	daemonList     = flag.String("daemon-list", "", "List daemon jobs (all, running, completed, failed)")
	daemonSchedule = flag.String("daemon-schedule", "", "Create scheduled export (format: 'name:cron', e.g., 'daily-backup:0 2 * * *')")
	daemonStatus   = flag.Bool("daemon-status", false, "Show daemon status information")
	watchProgress  = flag.Bool("watch", false, "Watch job progress in real-time (with --use-daemon)")

	// Incremental export options
	incrementalInfoFlag = flag.Bool("incremental-info", false, "Show incremental export analysis without exporting")

	// Shell completion
	completionShell = flag.String("completion", "", "Generate shell completion script (bash, zsh, fish)")
)

// Deprecated flag aliases: pre-rename "hyper2kvm-*" flag names kept pointing
// at the same variables as their "h2kvm-*" replacements, so existing scripts
// keep working for one release.
func init() {
	flag.StringVar(h2kvmPath, "hyper2kvm-path", *h2kvmPath, "Deprecated: use --h2kvm-path")
	flag.BoolVar(h2kvmDaemon, "hyper2kvm-daemon", *h2kvmDaemon, "Deprecated: use --h2kvm-daemon")
	flag.StringVar(h2kvmInstance, "hyper2kvm-instance", *h2kvmInstance, "Deprecated: use --h2kvm-instance")
	flag.StringVar(h2kvmWatchDir, "hyper2kvm-watch-dir", *h2kvmWatchDir, "Deprecated: use --h2kvm-watch-dir")
	flag.StringVar(h2kvmOutputDir, "hyper2kvm-output-dir", *h2kvmOutputDir, "Deprecated: use --h2kvm-output-dir")
	flag.IntVar(h2kvmPollInterval, "hyper2kvm-poll-interval", *h2kvmPollInterval, "Deprecated: use --h2kvm-poll-interval")
	flag.IntVar(h2kvmDaemonTimeout, "hyper2kvm-daemon-timeout", *h2kvmDaemonTimeout, "Deprecated: use --h2kvm-daemon-timeout")
}

// Vision-optimized color styles (based on human eye sensitivity: green > yellow > red).
// Sweet spot dark orange #D35400 - #C75B12: high contrast, no glare, no vibration.
var (
	// Primary accent color - dark orange (#D35400).
	colorOrange = pterm.NewRGB(211, 84, 0)         // Primary actions, highlights
	styleOrange = pterm.NewStyle(pterm.FgLightRed) // Closest pterm color to dark orange

	// Secondary colors optimized for readability.
	styleTeal   = pterm.NewStyle(pterm.FgLightCyan) // Info, directories
	colorGreen  = pterm.NewRGB(163, 190, 140)       // Success (eye-sensitive)
	styleGreen  = pterm.NewStyle(pterm.FgLightGreen)
	colorYellow = pterm.NewRGB(243, 156, 18) // Warnings (readable yellow)
	styleYellow = pterm.NewStyle(pterm.FgYellow)
	colorRed    = pterm.NewRGB(231, 76, 60) // Errors (bright for visibility)
	styleRed    = pterm.NewStyle(pterm.FgLightRed)
)

// newOrangeSpinner creates a modern orange-themed spinner with vision-optimized colors.
func newOrangeSpinner(message string) *pterm.SpinnerPrinter {
	s, _ := pterm.DefaultSpinner.
		WithStyle(styleOrange).
		WithSequence("⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏").
		Start(colorOrange.Sprintf("🔄 %s", message))
	return s
}

func main() {
	// Parse flags
	flag.Parse()

	// Set global pterm theme to orange (vision-optimized colors)
	pterm.ThemeDefault.PrimaryStyle = *styleOrange
	pterm.ThemeDefault.SecondaryStyle = *styleTeal
	pterm.ThemeDefault.SuccessMessageStyle = *styleGreen
	pterm.ThemeDefault.WarningMessageStyle = *styleYellow
	pterm.ThemeDefault.ErrorMessageStyle = *styleRed

	// Generate shell completion if requested
	if *completionShell != "" {
		switch *completionShell {
		case "bash":
			generateBashCompletion()
		case "zsh":
			generateZshCompletion()
		case "fish":
			generateFishCompletion()
		default:
			fmt.Fprintf(os.Stderr, "Unknown shell: %s (supported: bash, zsh, fish)\n", *completionShell)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Show version if requested
	if *showVersion {
		fmt.Println("HyperExport v0.2.0")
		fmt.Println("Multi-cloud VM export tool")
		os.Exit(0)
	}

	// Setup logging (needed for history and profile operations)
	cfg := config.FromEnvironment()
	log := logger.New(cfg.LogLevel)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle daemon-only operations (don't require VM connection)
	if *daemonWatch != "" {
		client := NewDaemonClient(*daemonURL, log)
		if err := client.WatchJobProgress(ctx, *daemonWatch, *quiet); err != nil {
			if !*quiet {
				pterm.Error.Printfln("Watch failed: %v", err)
			}
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *daemonList != "" {
		client := NewDaemonClient(*daemonURL, log)
		jobs, err := client.ListJobs(ctx, *daemonList)
		if err != nil {
			if !*quiet {
				pterm.Error.Printfln("Failed to list jobs: %v", err)
			} else {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			os.Exit(1)
		}

		if *quiet {
			for _, job := range jobs {
				fmt.Printf("%s\t%s\t%s\n", job.Definition.ID, job.Status, job.Definition.VMPath)
			}
		} else {
			displayDaemonJobs(jobs)
		}
		os.Exit(0)
	}

	if *daemonStatus {
		client := NewDaemonClient(*daemonURL, log)

		var spinner *pterm.SpinnerPrinter
		if !*quiet {
			spinner, _ = pterm.DefaultSpinner.Start("Fetching daemon status...")
		}

		status, err := client.GetDaemonStatus(ctx)
		if err != nil {
			if !*quiet {
				pterm.Error.Printfln("Failed to get daemon status: %v", err)
			}
			os.Exit(1)
		}

		if !*quiet {
			spinner.Success("Daemon status retrieved")
			pterm.DefaultSection.Println("📊 Daemon Status")

			// Display status as pretty JSON
			data, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(data))
		} else {
			data, _ := json.Marshal(status)
			fmt.Println(string(data))
		}
		os.Exit(0)
	}

	if *daemonSchedule != "" {
		// Parse schedule format: name:cron
		parts := strings.SplitN(*daemonSchedule, ":", 2)
		if len(parts) != 2 {
			if !*quiet {
				pterm.Error.Println("Invalid schedule format. Use: 'name:cron'")
				pterm.Info.Println("Example: --daemon-schedule 'daily-backup:0 2 * * *'")
			} else {
				fmt.Fprintf(os.Stderr, "error: invalid schedule format\n")
			}
			os.Exit(1)
		}

		name := parts[0]
		cronSchedule := parts[1]

		// Validate required flags for schedule
		if *vmName == "" {
			if !*quiet {
				pterm.Error.Println("VM name required for scheduled exports (-vm flag)")
			} else {
				fmt.Fprintf(os.Stderr, "error: -vm flag required\n")
			}
			os.Exit(1)
		}

		outputPath := getOutputDir(*vmName)

		client := NewDaemonClient(*daemonURL, log)

		var spinner *pterm.SpinnerPrinter
		if !*quiet {
			spinner, _ = pterm.DefaultSpinner.Start("Creating scheduled export...")
		}

		if err := client.CreateSchedule(ctx, name, cronSchedule, *vmName, outputPath); err != nil {
			if !*quiet {
				spinner.Fail("Failed to create schedule")
				pterm.Error.Printfln("Error: %v", err)
			}
			os.Exit(1)
		}

		if !*quiet {
			spinner.Success("Schedule created successfully!")
			pterm.Success.Printfln("✅ Created schedule: %s", name)
			pterm.Info.Printfln("   Cron: %s", cronSchedule)
			pterm.Info.Printfln("   VM: %s", *vmName)
			pterm.Info.Printfln("   Output: %s", outputPath)
		} else {
			fmt.Printf("schedule-created: %s\n", name)
		}
		os.Exit(0)
	}

	// Handle profile operations (don't require provider connection)
	profileManager, err := NewProfileManager(log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create profile manager: %v\n", err)
		os.Exit(1)
	}

	if *createDefaults {
		if err := profileManager.CreateDefaultProfiles(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create default profiles: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Default profiles created successfully")
		fmt.Println("Profiles created:")
		fmt.Println("  - quick-export: Fast export without compression")
		fmt.Println("  - production-backup: OVA with compression and verification")
		fmt.Println("  - encrypted-backup: Encrypted backup for sensitive data")
		fmt.Println("  - cloud-backup: Backup and upload to cloud storage")
		fmt.Println("  - development: Quick export for development/testing")
		os.Exit(0)
	}

	if *listProfiles {
		profiles, err := profileManager.ListProfiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to list profiles: %v\n", err)
			os.Exit(1)
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles found. Create some with --create-default-profiles or --save-profile")
			os.Exit(0)
		}

		fmt.Println("\n=== Available Export Profiles ===")
		for _, p := range profiles {
			fmt.Printf("Profile: %s\n", p.Name)
			fmt.Printf("  Description: %s\n", p.Description)
			fmt.Printf("  Format: %s", p.Format)
			if p.Compress {
				fmt.Printf(" (compressed)")
			}
			fmt.Println()
			if p.Encrypt {
				fmt.Printf("  Encryption: %s\n", p.EncryptMethod)
			}
			if p.UploadTo != "" {
				fmt.Printf("  Upload to: %s\n", p.UploadTo)
			}
			fmt.Printf("  Created: %s\n", p.Created.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
		os.Exit(0)
	}

	if *deleteProfile != "" {
		if err := profileManager.DeleteProfile(*deleteProfile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to delete profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Profile '%s' deleted successfully\n", *deleteProfile)
		os.Exit(0)
	}

	// Load profile if specified
	if *profile != "" {
		loadedProfile, err := profileManager.LoadProfile(*profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load profile: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Loaded profile: %s\n", loadedProfile.Name)
		fmt.Printf("Description: %s\n", loadedProfile.Description)

		// Apply profile settings to flags (override current values)
		*format = loadedProfile.Format
		*compress = loadedProfile.Compress
		*verify = loadedProfile.Verify
		*powerOff = loadedProfile.PowerOff
		*parallel = loadedProfile.Parallel
		if loadedProfile.UploadTo != "" {
			*uploadTo = loadedProfile.UploadTo
		}
		*keepLocal = loadedProfile.KeepLocal
		*encrypt = loadedProfile.Encrypt
		*encryptMethod = loadedProfile.EncryptMethod
		if loadedProfile.GPGRecipient != "" {
			*gpgRecipient = loadedProfile.GPGRecipient
		}
		*validateOnly = loadedProfile.ValidateOnly

		// Apply manifest settings
		*generateManifest = loadedProfile.GenerateManifest
		*verifyManifestFlag = loadedProfile.VerifyManifest
		*manifestChecksum = loadedProfile.ManifestChecksum
		if loadedProfile.ManifestTargetFormat != "" {
			*manifestTargetFormat = loadedProfile.ManifestTargetFormat
		}

		// Apply pipeline settings from profile
		*enablePipeline = loadedProfile.AutoConvert
		if loadedProfile.H2KVMBinary != "" {
			*h2kvmPath = loadedProfile.H2KVMBinary
		}
		*streamPipeline = loadedProfile.StreamConversion
	}

	// Handle history operations (don't require provider connection)
	if *showHistory || *generateReport || *clearHistory {
		historyFile, err := GetDefaultHistoryFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get history file: %v\n", err)
			os.Exit(1)
		}

		history := NewExportHistory(historyFile, log)

		if *clearHistory {
			if err := history.ClearHistory(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to clear history: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Export history cleared successfully")
			os.Exit(0)
		}

		if *showHistory {
			entries, err := history.GetRecentExports(*historyLimit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to get history: %v\n", err)
				os.Exit(1)
			}

			if len(entries) == 0 {
				fmt.Println("No export history found")
				os.Exit(0)
			}

			fmt.Printf("\n=== Export History (Last %d) ===\n\n", len(entries))
			for i, entry := range entries {
				status := "✓ SUCCESS"
				if !entry.Success {
					status = "✗ FAILED"
				}

				fmt.Printf("%d. %s [%s]\n", i+1, entry.VMName, status)
				fmt.Printf("   Time: %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))
				fmt.Printf("   Format: %s | Size: %s | Duration: %s\n",
					entry.Format,
					formatBytes(entry.TotalSize),
					entry.Duration.Round(time.Second))
				fmt.Printf("   Output: %s\n", entry.OutputDir)

				if !entry.Success && entry.ErrorMessage != "" {
					fmt.Printf("   Error: %s\n", entry.ErrorMessage)
				}
				fmt.Println()
			}
			os.Exit(0)
		}

		if *generateReport {
			report := NewExportReport(history)

			if *reportFile != "" {
				if err := report.SaveReportToFile(*reportFile, true, 20); err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to save report: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Report saved to: %s\n", *reportFile)
			} else {
				reportText, err := report.GenerateReport(true, 20)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to generate report: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(reportText)
			}
			os.Exit(0)
		}
	}

	// Create intro animation (skip if quiet mode)
	if !*quiet {
		showIntro()
	}

	// Override config with flags
	if *parallel > 0 {
		cfg.DownloadWorkers = *parallel
	}

	// Validate required environment variables
	if cfg.VCenterURL == "" || cfg.Username == "" || cfg.Password == "" {
		log.Error("missing required environment variables",
			"required", "GOVC_URL, GOVC_USERNAME, GOVC_PASSWORD")

		pterm.Error.Println("Missing required environment variables")
		pterm.DefaultBox.WithTitle("Required Environment Variables").
			WithTitleTopCenter().
			WithBoxStyle(pterm.NewStyle(pterm.FgRed)).
			Println("export GOVC_URL=https://vcenter.example.com/sdk\n" +
				"export GOVC_USERNAME=administrator@vsphere.local\n" +
				"export GOVC_PASSWORD=your-password\n" +
				"export GOVC_INSECURE=1  # (optional, for self-signed certs)")
		os.Exit(1)
	}

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Info("received signal, shutting down", "signal", sig)
		pterm.Warning.Printfln("Received signal %v, shutting down gracefully...", sig)
		cancel()
		time.Sleep(2 * time.Second)
		os.Exit(1)
	}()

	// Run the application
	if err := run(ctx, cfg, log); err != nil {
		log.Error("application failed", "error", err)
		pterm.Error.Printfln("Application failed: %v", err)
		os.Exit(1)
	}

	pterm.Success.Println("Application completed successfully!")
}

func showIntro() {
	// Clear screen
	pterm.DefaultCenter.Println()

	// Vision-optimized dark orange color scheme (sweet spot #D35400).
	// High contrast on black, no glare, no vibration.

	// Show large styled title in deep orange (#D35400) with proper capitalization
	pterm.Println()
	pterm.Println()
	pterm.DefaultCenter.WithCenterEachLineSeparately().Println(
		pterm.DefaultHeader.WithFullWidth(false).
			WithBackgroundStyle(styleOrange).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			WithMargin(4).
			Sprint("    HyperSDK    "),
	)
	pterm.Println()

	// Show subtitle
	subtitle := pterm.DefaultCenter.Sprint(colorYellow.Sprint("Interactive VM export tool"))
	version := pterm.DefaultCenter.Sprint(colorOrange.Sprint("Version 1.0.0"))

	pterm.Println(subtitle)
	pterm.Println(version)
	pterm.Println()
}

func run(ctx context.Context, cfg *config.Config, log logger.Logger) error {
	if strings.EqualFold(*providerType, "nutanix") {
		return runNutanixExport(ctx, cfg, log)
	}

	// Handle daemon mode - submit job to daemon instead of direct export
	if *useDaemon {
		if *vmName == "" {
			return fmt.Errorf("VM name required for daemon mode (-vm flag)")
		}

		outputPath := getOutputDir(*vmName)
		return runDaemonMode(ctx, *vmName, outputPath, *format, *compress, *watchProgress, *quiet, *daemonURL, log)
	}

	// Handle batch mode
	if *batchFile != "" {
		if strings.EqualFold(*providerType, "nutanix") {
			return runNutanixBatchExport(ctx, cfg, log)
		}
		return runBatchExport(ctx, cfg, log)
	}

	// Connection spinner with orange theme
	var spinner *pterm.SpinnerPrinter
	if !*quiet && !*interactive && !*tui {
		spinner = newOrangeSpinner("Connecting to " + *providerType + "...")
	}

	log.Info("connecting to provider", "type", *providerType)

	client, err := vsphere.NewVSphereClient(ctx, cfg, log)
	if err != nil {
		if spinner != nil {
			spinner.Fail("Failed to connect to " + *providerType)
		}
		return fmt.Errorf("connect to provider: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Error("failed to close client", "error", err)
		}
	}()

	if spinner != nil {
		spinner.Success("Connected to " + *providerType + " successfully!")
	}

	// Launch interactive TUI mode if requested
	if *interactive || *tui {
		return runInteractiveHuh(ctx, client, cfg, log)
	}

	// Display VM information if requested
	if *vmInfo {
		if *vmName == "" {
			return fmt.Errorf("VM name required for --vm-info (use -vm flag)")
		}
		return showVMInfoCmd(ctx, client, *vmName, *quiet, log)
	}

	// Display incremental export analysis if requested
	if *incrementalInfoFlag {
		if *vmName == "" {
			return fmt.Errorf("VM name required for --incremental-info (use -vm flag)")
		}
		return showIncrementalInfo(ctx, client, *vmName, log)
	}

	// Show connection info panel (skip in quiet mode)
	if !*quiet {
		pterm.DefaultBox.WithTitle("Connection Info").
			WithTitleTopLeft().
			WithBoxStyle(styleOrange).
			Printfln("vCenter: %s\nUser: %s",
				cfg.VCenterURL,
				cfg.Username)
	}

	// Determine selected VM
	var selectedVM string
	if *vmName != "" {
		// VM specified via command line
		selectedVM = *vmName
		log.Info("using VM from command line", "vm", selectedVM)
	} else {
		// Discover VMs with modern orange-themed progress indicator
		if !*quiet {
			spinner = newOrangeSpinner("Discovering virtual machines...")
		}

		log.Info("discovering VMs")
		vms, err := client.FindAllVMs(ctx)
		if err != nil {
			if spinner != nil {
				spinner.Fail(colorRed.Sprintf("✗ Failed to discover VMs"))
			}
			return fmt.Errorf("find VMs: %w", err)
		}

		// Apply filters
		if *folder != "" {
			vms = filterByFolder(vms, *folder)
		}
		if *filter != "" {
			vms, err = filterByTag(ctx, client, vms, *filter, log)
			if err != nil {
				if spinner != nil {
					spinner.Fail(colorRed.Sprintf("✗ Failed to filter VMs by tag"))
				}
				return fmt.Errorf("filter by tag: %w", err)
			}
		}

		if spinner != nil {
			spinner.Success(colorGreen.Sprintf("✓ Found %d virtual machine(s)", len(vms)))
		}
		log.Info("found VMs", "count", len(vms))

		if len(vms) == 0 {
			log.Warn("no VMs found")
			if !*quiet {
				pterm.Warning.Println("No VMs found")
			}
			return nil
		}

		// Interactive VM selection
		selectedVM, err = selectVMInteractive(vms, log)
		if err != nil {
			return fmt.Errorf("select VM: %w", err)
		}
	}

	// Get VM info with spinner
	if !*quiet {
		spinner, _ = pterm.DefaultSpinner.Start("Retrieving VM information...")
	}

	info, err := client.GetVMInfo(ctx, selectedVM)
	if err != nil {
		if spinner != nil {
			spinner.Fail("Failed to get VM info")
		}
		return fmt.Errorf("get VM info: %w", err)
	}

	if spinner != nil {
		spinner.Success("Retrieved VM information")
	}

	// Display VM info in a beautiful panel (skip in quiet mode)
	if !*quiet {
		displayVMInfo(info)
	}

	log.Info("selected VM",
		"name", info.Name,
		"powerState", info.PowerState,
		"memoryMB", info.MemoryMB,
		"cpus", info.NumCPU,
		"os", info.GuestOS)

	// Dry-run mode
	if *dryRun {
		if !*quiet {
			pterm.Info.Println("Dry-run mode: Export preview")
			pterm.DefaultSection.Println("Export Plan")
			fmt.Printf("  VM: %s\n", info.Name)
			fmt.Printf("  Format: %s\n", *format)
			fmt.Printf("  Compression: %v\n", *compress)
			fmt.Printf("  Output: %s\n", getOutputDir(info.Name))
			fmt.Printf("  Estimated Size: %s\n", formatBytes(info.Storage))
		} else {
			fmt.Printf("dry-run: %s -> %s (%s, %s)\n", info.Name, getOutputDir(info.Name), *format, formatBytes(info.Storage))
		}
		return nil
	}

	// Shutdown if needed
	if info.PowerState == "poweredOn" {
		shouldPowerOff := *powerOff

		// Interactive confirmation if not specified via flag
		if !shouldPowerOff && !*quiet {
			pterm.Info.Println("VM is currently powered on")
			result, _ := pterm.DefaultInteractiveConfirm.
				WithDefaultText("Do you want to shutdown the VM before export?").
				WithDefaultValue(false).
				Show()
			shouldPowerOff = result
		}

		if shouldPowerOff {
			log.Warn("VM is powered on, attempting graceful shutdown")

			if !*quiet {
				spinner, _ = pterm.DefaultSpinner.Start("Shutting down VM gracefully...")
			}

			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Minute)
			defer shutdownCancel()

			if err := client.ShutdownVM(shutdownCtx, selectedVM, 5*time.Minute); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					if spinner != nil {
						spinner.Warning("Graceful shutdown timeout, forcing power off...")
					}
					log.Warn("graceful shutdown timeout, forcing power off")

					if err := client.PowerOffVM(ctx, selectedVM); err != nil {
						if spinner != nil {
							spinner.Fail("Failed to power off VM")
						}
						return fmt.Errorf("force power off: %w", err)
					}
				} else {
					if spinner != nil {
						spinner.Fail("Failed to shutdown VM")
					}
					return fmt.Errorf("shutdown VM: %w", err)
				}
			}
			if spinner != nil {
				spinner.Success("VM powered off successfully")
			}
		} else if !*quiet {
			pterm.Warning.Println("Continuing with VM powered on (not recommended)")
		}
	}

	// Export VM
	exportDir := getOutputDir(info.Name)

	// Pre-export validation
	if !*quiet {
		spinner = newOrangeSpinner("Running pre-export validation...")
	}

	preValidator := NewPreExportValidator(log)
	preReport := preValidator.ValidateExport(ctx, *info, exportDir, info.Storage)

	if spinner != nil {
		if preReport.AllPassed {
			spinner.Success("Pre-export validation passed")
		} else {
			spinner.Warning("Pre-export validation completed with issues")
		}
	}

	// Display validation results
	if !*quiet {
		displayValidationReport("Pre-Export Validation", preReport)
	}

	// Stop if validation-only mode
	if *validateOnly {
		if preReport.AllPassed {
			pterm.Success.Println("Validation completed - VM is ready for export")
			return nil
		} else {
			pterm.Error.Println("Validation failed - fix issues before exporting")
			return fmt.Errorf("validation failed")
		}
	}

	// Stop if validation failed (unless warnings only)
	if !preReport.AllPassed {
		if !*quiet {
			pterm.Error.Println("Pre-export validation failed - cannot proceed")
		}
		return fmt.Errorf("pre-export validation failed")
	}

	// Warn about validation warnings
	if preReport.HasWarnings && !*quiet {
		pterm.Warning.Println("Pre-export validation has warnings - proceeding anyway")
		if !*quiet && !*powerOff {
			result, _ := pterm.DefaultInteractiveConfirm.
				WithDefaultText("Continue with export despite warnings?").
				WithDefaultValue(true).
				Show()
			if !result {
				return fmt.Errorf("export cancelled by user")
			}
		}
	}

	// Phase 6: Create migration orchestrator if requested
	var orchestrator *common.MigrationOrchestrator
	var progressServer *common.ProgressAPIServer
	var metricsServer *common.MetricsServer

	if *enableOrchestration || *progressAPIPort != "" || *metricsAPIPort != "" || *auditLogPath != "" || *webhookURL != "" {
		if !*quiet {
			pterm.Info.Println("Phase 6 orchestration enabled")
		}

		// Configure webhooks
		var webhookConfigs []*common.WebhookConfig
		if *webhookURL != "" {
			hookType := common.WebhookGeneric
			switch *webhookType {
			case "slack":
				hookType = common.WebhookSlack
			case "discord":
				hookType = common.WebhookDiscord
			}

			webhookConfigs = []*common.WebhookConfig{
				{
					Type:       hookType,
					URL:        *webhookURL,
					Enabled:    true,
					OnStart:    *webhookOnStart,
					OnComplete: *webhookOnComplete,
					OnError:    *webhookOnError,
				},
			}
		}

		// Create orchestrator configuration
		orchConfig := &common.OrchestratorConfig{
			EnableConversion:   *enablePipeline,
			EnableProgress:     *progressAPIPort != "",
			EnableMetrics:      *metricsAPIPort != "",
			EnableAuditLogging: *auditLogPath != "",
			EnableWebhooks:     *webhookURL != "",
			AuditLogPath:       *auditLogPath,
			WebhookConfigs:     webhookConfigs,
		}

		var err error
		orchestrator, err = common.NewMigrationOrchestrator(orchConfig, log)
		if err != nil {
			return fmt.Errorf("create migration orchestrator: %w", err)
		}
		defer func() {
			if cerr := orchestrator.Close(); cerr != nil {
				log.Warn("failed to close migration orchestrator", "error", cerr)
			}
		}()

		if !*quiet {
			pterm.Success.Println("Migration orchestrator created")
		}

		// Start progress API server if requested
		if *progressAPIPort != "" {
			tracker := orchestrator.GetProgressTracker()
			progressServer = common.NewProgressAPIServer(tracker, *progressAPIPort)
			go func() {
				if err := progressServer.Start(); err != nil {
					log.Error("progress API server failed", "error", err)
				}
			}()
			if !*quiet {
				pterm.Success.Printfln("Progress API started on %s", *progressAPIPort)
				pterm.Info.Printfln("  • Query progress: http://localhost%s/api/v1/progress", *progressAPIPort)
			}
		}

		// Start metrics API server if requested
		if *metricsAPIPort != "" {
			collector := orchestrator.GetMetricsCollector()
			metricsServer = common.NewMetricsServer(collector, *metricsAPIPort)
			go func() {
				if err := metricsServer.Start(); err != nil {
					log.Error("metrics API server failed", "error", err)
				}
			}()
			if !*quiet {
				pterm.Success.Printfln("Metrics API started on %s", *metricsAPIPort)
				pterm.Info.Printfln("  • Prometheus metrics: http://localhost%s/metrics", *metricsAPIPort)
				pterm.Info.Printfln("  • JSON stats: http://localhost%s/stats", *metricsAPIPort)
			}
		}

		// Show audit log location if enabled
		if *auditLogPath != "" && !*quiet {
			pterm.Info.Printfln("Audit logging enabled: %s", *auditLogPath)
		}
	}

	opts := vsphere.DefaultExportOptions()
	opts.OutputPath = exportDir
	opts.ParallelDownloads = cfg.DownloadWorkers
	opts.ShowIndividualProgress = cfg.LogLevel == "debug"
	opts.Format = *format
	opts.Compress = *compress

	// Artifact Manifest v1.0 options
	opts.GenerateManifest = *generateManifest
	opts.VerifyManifest = *verifyManifestFlag
	opts.ManifestComputeChecksum = *manifestChecksum
	opts.ManifestTargetFormat = *manifestTargetFormat

	// Pipeline integration options
	opts.EnablePipeline = *enablePipeline
	opts.H2KVMPath = *h2kvmPath
	opts.PipelineTimeout = *pipelineTimeout
	opts.StreamPipelineOutput = *streamPipeline
	opts.PipelineDryRun = *pipelineDryRun
	opts.PipelineInspect = *pipelineInspect
	opts.PipelineFix = *pipelineFix
	opts.PipelineConvert = *pipelineConvert
	opts.PipelineValidate = *pipelineValidate
	opts.PipelineCompress = *pipelineCompress
	opts.PipelineCompressLevel = *compressLevel

	// Libvirt integration options
	opts.LibvirtIntegration = *libvirtIntegration
	opts.LibvirtURI = *libvirtURI
	opts.LibvirtAutoStart = *libvirtAutoStart
	opts.LibvirtNetworkBridge = *libvirtBridge
	opts.LibvirtStoragePool = *libvirtPool

	// h2kvm daemon options
	opts.H2KVMDaemon = *h2kvmDaemon
	opts.H2KVMInstance = *h2kvmInstance
	opts.H2KVMWatchDir = *h2kvmWatchDir
	opts.H2KVMOutputDir = *h2kvmOutputDir
	opts.H2KVMPollInterval = *h2kvmPollInterval
	opts.H2KVMDaemonTimeout = *h2kvmDaemonTimeout

	// If pipeline is enabled, force manifest generation
	if opts.EnablePipeline {
		opts.GenerateManifest = true
		if !*quiet {
			pterm.Info.Println("Pipeline enabled: manifest generation forced")
		}
	}

	if !*quiet {
		pterm.Info.Printfln("Starting %s export to: %s", strings.ToUpper(*format), exportDir)
	}
	log.Info("starting export", "vm", info.Name, "output", exportDir, "format", *format, "compress", *compress)

	// Phase 6: Track migration start
	var taskID string
	if orchestrator != nil {
		taskID = fmt.Sprintf("export-%s-%d", sanitizeForPath(info.Name), time.Now().Unix())
		tracker := orchestrator.GetProgressTracker()
		if tracker != nil {
			tracker.StartTask(taskID, info.Name, *providerType)
			if err := tracker.SetStatus(taskID, common.StatusExporting); err != nil {
				log.Warn("failed to set task status", "task_id", taskID, "error", err)
			}
		}
		collector := orchestrator.GetMetricsCollector()
		if collector != nil {
			collector.RecordMigrationStart(*providerType)
		}
		auditLogger := orchestrator.GetAuditLogger()
		if auditLogger != nil {
			username := cfg.Username
			if username == "" {
				username = os.Getenv("USER")
			}
			if err := auditLogger.LogMigrationStart(taskID, info.Name, *providerType, username); err != nil {
				log.Warn("failed to log migration start", "task_id", taskID, "error", err)
			}
			if err := auditLogger.LogExportStart(taskID, info.Name, *providerType); err != nil {
				log.Warn("failed to log export start", "task_id", taskID, "error", err)
			}
		}
	}

	exportStartTime := time.Now()
	result, err := client.ExportOVF(ctx, selectedVM, opts)
	exportDuration := time.Since(exportStartTime)

	if err != nil {
		// Phase 6: Record failure
		if orchestrator != nil {
			tracker := orchestrator.GetProgressTracker()
			if tracker != nil {
				if failErr := tracker.FailTask(taskID, err); failErr != nil {
					log.Warn("failed to mark task failed", "task_id", taskID, "error", failErr)
				}
			}
			collector := orchestrator.GetMetricsCollector()
			if collector != nil {
				collector.RecordMigrationFailure(*providerType)
			}
			auditLogger := orchestrator.GetAuditLogger()
			if auditLogger != nil {
				username := cfg.Username
				if username == "" {
					username = os.Getenv("USER")
				}
				if logErr := auditLogger.LogMigrationFailed(taskID, info.Name, *providerType, username, err); logErr != nil {
					log.Warn("failed to log migration failure", "task_id", taskID, "error", logErr)
				}
			}
		}

		if !*quiet {
			pterm.Error.Printfln("Export failed: %v", err)
		}
		return fmt.Errorf("export %s: %w", *format, err)
	}

	// Phase 6: Record export completion
	if orchestrator != nil {
		auditLogger := orchestrator.GetAuditLogger()
		if auditLogger != nil {
			if err := auditLogger.LogExportComplete(taskID, info.Name, *providerType, exportDuration, result.TotalSize); err != nil {
				log.Warn("failed to log export completion", "task_id", taskID, "error", err)
			}
		}
	}

	// Create OVA if requested
	if *format == "ova" {
		if !*quiet {
			spinner, _ = pterm.DefaultSpinner.Start("Packaging as OVA...")
		}
		log.Info("creating OVA archive")

		ovaPath := filepath.Join(exportDir, sanitizeForPath(info.Name)+".ova")
		compressionLevel := 6 // Default gzip compression level
		if err := vsphere.CreateOVA(exportDir, ovaPath, *compress, compressionLevel, log); err != nil {
			if spinner != nil {
				spinner.Fail("Failed to create OVA")
			}
			return fmt.Errorf("create OVA: %w", err)
		}

		if spinner != nil {
			spinner.Success("OVA created successfully")
		}
		result.OVAPath = ovaPath
		result.Format = "ova"
	}

	// Post-export validation
	if !*quiet {
		spinner, _ = pterm.DefaultSpinner.Start("Running post-export validation...")
	}

	postValidator := NewPostExportValidator(log)
	postReport := postValidator.ValidateExportedFiles(exportDir)

	if spinner != nil {
		if postReport.AllPassed {
			spinner.Success("Post-export validation passed")
		} else {
			spinner.Warning("Post-export validation completed with issues")
		}
	}

	// Display validation results
	if !*quiet {
		displayValidationReport("Post-Export Validation", postReport)
	}

	// Warn if post-validation failed
	if !postReport.AllPassed {
		log.Warn("post-export validation failed", "checks", len(postReport.Checks))
		if !*quiet {
			pterm.Warning.Println("Post-export validation detected issues with exported files")
		}
	}

	// Verify export if requested
	if *verify {
		if !*quiet {
			spinner, _ = pterm.DefaultSpinner.Start("Verifying export with checksums...")
		}
		log.Info("verifying export")

		checksums, err := verifyExport(result, log)
		if err != nil {
			if spinner != nil {
				spinner.Fail("Verification failed")
			}
			return fmt.Errorf("verify export: %w", err)
		}

		if spinner != nil {
			spinner.Success("Export verified successfully")
		}

		// Save checksums to file
		checksumFile := filepath.Join(exportDir, "checksums.txt")
		if err := saveChecksums(checksumFile, checksums); err != nil {
			log.Error("failed to save checksums", "error", err)
		}
	}

	// Show export summary in a fancy table (skip in quiet mode)
	if !*quiet {
		showExportSummary(info, result)
	} else {
		fmt.Printf("success: %s exported to %s (%s)\n", info.Name, exportDir, formatBytes(result.TotalSize))
	}

	log.Info("export completed successfully",
		"duration", result.Duration.Round(time.Second),
		"totalSize", formatBytes(result.TotalSize),
		"files", len(result.Files),
		"output", result.OutputDir)

	// Encrypt export if requested
	if *encrypt {
		if !*quiet {
			spinner, _ = pterm.DefaultSpinner.Start(fmt.Sprintf("Encrypting export with %s...", *encryptMethod))
		}
		log.Info("encrypting export", "method", *encryptMethod)

		encConfig := &EncryptionConfig{
			Method:       EncryptionMethod(*encryptMethod),
			Passphrase:   *passphrase,
			KeyFile:      *keyFile,
			GPGRecipient: *gpgRecipient,
		}

		// Check if passphrase/key is provided
		if encConfig.Passphrase == "" && encConfig.KeyFile == "" && encConfig.GPGRecipient == "" {
			if spinner != nil {
				spinner.Fail("Encryption failed: no passphrase, key file, or GPG recipient provided")
			}
			return fmt.Errorf("encryption requires passphrase, key file, or GPG recipient")
		}

		encryptor := NewEncryptor(encConfig, log)

		// Encrypt all files in export directory
		encryptedDir := exportDir + "-encrypted"
		if err := encryptor.EncryptDirectory(exportDir, encryptedDir); err != nil {
			if spinner != nil {
				spinner.Fail("Encryption failed")
			}
			return fmt.Errorf("encrypt export: %w", err)
		}

		if spinner != nil {
			spinner.Success("Export encrypted successfully")
		}

		// Replace export directory with encrypted directory
		if err := os.RemoveAll(exportDir); err != nil {
			log.Warn("failed to remove unencrypted export", "error", err)
		}
		if err := os.Rename(encryptedDir, exportDir); err != nil {
			return fmt.Errorf("rename encrypted directory: %w", err)
		}

		log.Info("export encrypted successfully", "method", *encryptMethod)
	}

	// Upload to cloud storage if requested
	if *uploadTo != "" {
		if !*quiet {
			spinner, _ = pterm.DefaultSpinner.Start(fmt.Sprintf("Uploading to %s...", *uploadTo))
		}
		log.Info("uploading export to cloud", "destination", *uploadTo)

		cloudStorage, err := NewCloudStorage(*uploadTo, log)
		if err != nil {
			if spinner != nil {
				spinner.Fail("Failed to initialize cloud storage")
			}
			log.Error("failed to create cloud storage client", "error", err)
			if !*quiet {
				pterm.Error.Printfln("Cloud upload failed: %v", err)
			}
		} else {
			defer func() {
				if cerr := cloudStorage.Close(); cerr != nil {
					log.Warn("failed to close cloud storage client", "error", cerr)
				}
			}()

			// Upload the export directory
			remotePath := sanitizeForPath(info.Name)
			if err := UploadDirectory(ctx, cloudStorage, exportDir, remotePath, log); err != nil {
				if spinner != nil {
					spinner.Fail("Upload failed")
				}
				log.Error("failed to upload export", "error", err)
				if !*quiet {
					pterm.Error.Printfln("Cloud upload failed: %v", err)
				}
			} else {
				if spinner != nil {
					spinner.Success(fmt.Sprintf("Uploaded to %s successfully", *uploadTo))
				}
				log.Info("export uploaded to cloud successfully", "destination", *uploadTo)

				// Delete local copy if requested
				if !*keepLocal {
					if !*quiet {
						spinner, _ = pterm.DefaultSpinner.Start("Removing local copy...")
					}
					log.Info("removing local export", "path", exportDir)

					if err := os.RemoveAll(exportDir); err != nil {
						if spinner != nil {
							spinner.Warning("Failed to remove local copy")
						}
						log.Warn("failed to remove local export", "error", err)
					} else {
						if spinner != nil {
							spinner.Success("Local copy removed")
						}
					}
				}
			}
		}
	}

	// Record export in history
	historyFile, err := GetDefaultHistoryFile()
	if err == nil {
		history := NewExportHistory(historyFile, log)
		historyEntry := ExportHistoryEntry{
			Timestamp:  time.Now(),
			VMName:     info.Name,
			VMPath:     selectedVM,
			Provider:   *providerType,
			Format:     result.Format,
			OutputDir:  result.OutputDir,
			TotalSize:  result.TotalSize,
			Duration:   result.Duration,
			FilesCount: len(result.Files),
			Success:    true,
			Compressed: *compress,
			Verified:   *verify,
			Metadata: map[string]string{
				"uploaded_to": *uploadTo,
			},
		}
		if err := history.RecordExport(historyEntry); err != nil {
			log.Warn("failed to record export in history", "error", err)
		}
	}

	// Record incremental export metadata
	if err := recordIncrementalExport(ctx, selectedVM, result, log); err != nil {
		log.Warn("failed to record incremental export metadata", "error", err)
	}

	// Phase 6: Record migration completion
	if orchestrator != nil {
		tracker := orchestrator.GetProgressTracker()
		if tracker != nil {
			if err := tracker.CompleteTask(taskID); err != nil {
				log.Warn("failed to mark task complete", "task_id", taskID, "error", err)
			}
		}
		collector := orchestrator.GetMetricsCollector()
		if collector != nil {
			conversionDuration := time.Duration(0)
			if result.ConversionResult != nil {
				conversionDuration = result.ConversionResult.Duration
			}
			collector.RecordMigrationSuccess(
				*providerType,
				exportDuration,
				conversionDuration,
				time.Duration(0), // upload duration (not tracked yet)
				result.TotalSize,
				result.TotalSize,
				result.TotalSize,
			)
		}
		auditLogger := orchestrator.GetAuditLogger()
		if auditLogger != nil {
			username := cfg.Username
			if username == "" {
				username = os.Getenv("USER")
			}
			totalDuration := time.Since(exportStartTime)
			metadata := map[string]interface{}{
				"format":     result.Format,
				"files":      len(result.Files),
				"compressed": *compress,
				"verified":   *verify,
				"manifest":   result.ManifestPath != "",
				"converted":  result.ConversionResult != nil && result.ConversionResult.Success,
			}
			if *uploadTo != "" {
				metadata["uploaded_to"] = *uploadTo
			}
			if err := auditLogger.LogMigrationComplete(taskID, info.Name, *providerType, username, totalDuration, metadata); err != nil {
				log.Warn("failed to log migration completion", "task_id", taskID, "error", err)
			}
		}

		// Stop servers if running
		if progressServer != nil {
			if err := progressServer.Stop(ctx); err != nil {
				log.Warn("failed to stop progress server", "error", err)
			}
		}
		if metricsServer != nil {
			if err := metricsServer.Stop(); err != nil {
				log.Warn("failed to stop metrics server", "error", err)
			}
		}
	}

	// Celebration (skip in quiet mode)
	if !*quiet {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgLightGreen)).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			Println("Export Completed Successfully!")
	}

	return nil
}

func selectVMInteractive(vms []string, log logger.Logger) (string, error) {
	// Clear screen for better UX
	pterm.Print("\033[H\033[2J")

	if len(vms) == 1 {
		log.Info("auto-selecting only VM", "vm", vms[0])
		pterm.Success.Println("Auto-selecting the only available VM")
		return vms[0], nil
	}

	// Show header with branding in deep orange (proper capitalization)
	pterm.Println()
	pterm.DefaultCenter.WithCenterEachLineSeparately().Println(
		pterm.DefaultHeader.WithFullWidth(false).
			WithBackgroundStyle(styleOrange).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			WithMargin(2).
			Sprint("  HyperExport  "),
	)
	pterm.Println()

	pterm.Println()

	// Show VM count and instructions
	pterm.DefaultBox.
		WithTitle("VM Selection").
		WithTitleTopCenter().
		WithBoxStyle(styleOrange).
		Printfln("Found %d virtual machines\n\n"+
			"💡 Use ↑/↓ arrows to navigate\n"+
			"💡 Press / to search and filter\n"+
			"💡 Press Enter to select\n"+
			"💡 Press Ctrl+C to cancel", len(vms))

	pterm.Println()

	// Extract VM names for display
	vmNames := make([]string, len(vms))
	for i, vm := range vms {
		parts := strings.Split(vm, "/")
		vmNames[i] = parts[len(parts)-1]
	}

	// Sort VMs alphabetically for easier navigation
	sorted := make([]string, len(vmNames))
	copy(sorted, vmNames)
	sortIndices := make([]int, len(vmNames))
	for i := range sortIndices {
		sortIndices[i] = i
	}

	// Simple bubble sort with index tracking
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
				sortIndices[j], sortIndices[j+1] = sortIndices[j+1], sortIndices[j]
			}
		}
	}

	// Show selection prompt with orange theme
	pterm.DefaultHeader.WithFullWidth().
		WithBackgroundStyle(styleOrange).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println("Select VM to Export")

	// Interactive select with orange theme and search
	// Customize pterm's DefaultInteractiveSelect to use orange theme
	pterm.DefaultInteractiveSelect.TextStyle = styleOrange

	selectedName, err := pterm.DefaultInteractiveSelect.
		WithDefaultText(colorOrange.Sprint("Select a VM to export [type to search]")).
		WithOptions(sorted).
		WithFilter(true).
		WithMaxHeight(15).
		Show()

	if err != nil {
		return "", err
	}

	// Find the original index
	var originalIndex int
	for i, name := range sorted {
		if name == selectedName {
			originalIndex = sortIndices[i]
			break
		}
	}

	pterm.Println()
	pterm.Success.WithPrefix(pterm.Prefix{
		Text:  "SELECTED",
		Style: pterm.NewStyle(pterm.BgGreen, pterm.FgBlack),
	}).Printfln("%s", selectedName)

	return vms[originalIndex], nil
}

func displayVMInfo(info *vsphere.VMInfo) {
	pterm.Println()

	// Show VM details in a beautiful panel (using vision-optimized orange).
	pterm.DefaultHeader.WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgLightRed)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println("📋 Virtual Machine Details")

	pterm.Println()

	// Create enhanced table data with icons
	data := pterm.TableData{
		{"Property", "Value"},
		{"🖥️  VM Name", pterm.Bold.Sprint(info.Name)},
		{"⚡ Power State", getPowerStateIcon(info.PowerState) + " " + pterm.Bold.Sprint(info.PowerState)},
		{"💿 Guest OS", info.GuestOS},
		{"🧠 Memory", colorOrange.Sprint(fmt.Sprintf("%d MB (%.1f GB)", info.MemoryMB, float64(info.MemoryMB)/1024))},
		{"⚙️  vCPUs", colorOrange.Sprint(fmt.Sprintf("%d", info.NumCPU))},
		{"💾 Storage", colorOrange.Sprint(formatBytes(info.Storage))},
		{"📁 Path", pterm.FgGray.Sprint(info.Path)},
	}

	// Render enhanced table
	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderRowSeparator("─").
		WithBoxed().
		WithLeftAlignment().
		WithData(data).
		Render(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to render VM info table: %v\n", err)
	}

	pterm.Println()
}

func showExportSummary(info *vsphere.VMInfo, result *vsphere.ExportResult) {
	pterm.Println()
	pterm.Println()

	// Show celebration header with proper capitalization
	pterm.DefaultCenter.WithCenterEachLineSeparately().Println(
		pterm.DefaultHeader.WithFullWidth(false).
			WithBackgroundStyle(styleGreen).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			WithMargin(4).
			Sprint("   ✓ Export Successful   "),
	)

	pterm.Println()

	// Calculate speed
	speedMBps := float64(result.TotalSize) / result.Duration.Seconds() / 1024 / 1024

	// Create enhanced summary table with icons
	data := pterm.TableData{
		{"Metric", "Value"},
		{"🖥️  VM Name", pterm.Bold.Sprint(info.Name)},
		{"⏱️  Duration", colorOrange.Sprint(result.Duration.Round(time.Second).String())},
		{"💾 Total Size", colorOrange.Sprint(formatBytes(result.TotalSize))},
		{"⚡ Avg Speed", colorOrange.Sprint(fmt.Sprintf("%.1f MB/s", speedMBps))},
		{"📦 Files Exported", colorOrange.Sprint(fmt.Sprintf("%d", len(result.Files)))},
		{"📁 Output Directory", pterm.FgGray.Sprint(result.OutputDir)},
	}

	// Add manifest path if generated
	if result.ManifestPath != "" {
		data = append(data, []string{"📋 Artifact Manifest", pterm.Green(result.ManifestPath)})
	}

	// Add conversion results if present (Phase 2)
	if result.ConversionResult != nil {
		convStatus := pterm.Red("❌ FAILED")
		if result.ConversionResult.Success {
			convStatus = pterm.Green("✅ SUCCESS")
		}
		data = append(data, []string{"🔄 Conversion Status", convStatus})

		if result.ConversionResult.Success {
			data = append(data, []string{"📦 Converted Files", colorOrange.Sprint(fmt.Sprintf("%d", len(result.ConversionResult.ConvertedFiles)))})
			data = append(data, []string{"⏱️  Conversion Time", colorOrange.Sprint(result.ConversionResult.Duration.Round(time.Second).String())})
			if result.ConversionResult.ReportPath != "" {
				data = append(data, []string{"📊 Conversion Report", pterm.Green(result.ConversionResult.ReportPath)})
			}
		} else if result.ConversionResult.Error != "" {
			data = append(data, []string{"❌ Conversion Error", pterm.Red(result.ConversionResult.Error)})
		}
	}

	pterm.DefaultHeader.WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgGreen)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
		Println("✅ Export Summary")

	pterm.Println()

	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderRowSeparator("─").
		WithBoxed().
		WithLeftAlignment().
		WithData(data).
		Render(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to render export summary table: %v\n", err)
	}

	// Show file list in a panel if there are files
	if len(result.Files) > 0 && len(result.Files) <= 10 {
		pterm.DefaultSection.Println("Exported Files")
		fileList := pterm.DefaultBulletList
		items := make([]pterm.BulletListItem, 0, len(result.Files))
		for _, file := range result.Files {
			items = append(items, pterm.BulletListItem{
				Level: 0,
				Text:  file,
			})
		}
		if err := fileList.WithItems(items).Render(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to render exported files list: %v\n", err)
		}
	}

	// Show converted files if present (Phase 2)
	if result.ConversionResult != nil && result.ConversionResult.Success && len(result.ConversionResult.ConvertedFiles) > 0 {
		pterm.DefaultSection.Println("Converted Files (KVM-Ready)")
		fileList := pterm.DefaultBulletList
		items := make([]pterm.BulletListItem, 0, len(result.ConversionResult.ConvertedFiles))
		for _, file := range result.ConversionResult.ConvertedFiles {
			items = append(items, pterm.BulletListItem{
				Level: 0,
				Text:  pterm.Green(file),
			})
		}
		if err := fileList.WithItems(items).Render(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to render converted files list: %v\n", err)
		}
	}
}

func getPowerStateIcon(state string) string {
	switch state {
	case "poweredOn":
		return pterm.Green("●")
	case "poweredOff":
		return pterm.Red("●")
	case "suspended":
		return pterm.Yellow("●")
	default:
		return pterm.Gray("●")
	}
}

func sanitizeForPath(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '|', '?', '*', '/', '\\':
			return '_'
		default:
			return r
		}
	}, name)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getOutputDir(vmName string) string {
	if *outputDir != "" {
		return *outputDir
	}
	return "./export-" + sanitizeForPath(vmName)
}

func filterByFolder(vms []string, folder string) []string {
	filtered := make([]string, 0)
	for _, vm := range vms {
		if strings.Contains(vm, folder) {
			filtered = append(filtered, vm)
		}
	}
	return filtered
}

// filterByTag filters VMs by tag (format: key=value)
func filterByTag(ctx context.Context, client *vsphere.VSphereClient, vms []string, tagFilter string, log logger.Logger) ([]string, error) {
	// Parse filter (key=value)
	parts := strings.SplitN(tagFilter, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid tag filter format, expected key=value")
	}
	tagKey := strings.TrimSpace(parts[0])
	tagValue := strings.TrimSpace(parts[1])

	// Use client method to filter VMs by tag
	return client.FilterVMsByTag(ctx, vms, tagKey, tagValue)
}

func runBatchExport(ctx context.Context, cfg *config.Config, log logger.Logger) error {
	// Read VM list from file
	data, err := os.ReadFile(*batchFile)
	if err != nil {
		return fmt.Errorf("read batch file: %w", err)
	}

	vmList := strings.Split(string(data), "\n")
	validVMs := make([]string, 0)
	for _, vm := range vmList {
		vm = strings.TrimSpace(vm)
		if vm != "" && !strings.HasPrefix(vm, "#") {
			validVMs = append(validVMs, vm)
		}
	}

	if len(validVMs) == 0 {
		return fmt.Errorf("no valid VMs in batch file")
	}

	log.Info("batch export", "count", len(validVMs))
	if !*quiet {
		pterm.Info.Printfln("Batch export: %d VMs", len(validVMs))
	}

	// Connect once for all exports
	client, err := vsphere.NewVSphereClient(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("connect to vSphere: %w", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			log.Warn("failed to close vSphere client", "error", cerr)
		}
	}()

	successCount := 0
	failureCount := 0

	for i, vmPath := range validVMs {
		if !*quiet {
			pterm.DefaultSection.Printfln("Exporting VM %d/%d: %s", i+1, len(validVMs), vmPath)
		}

		// Temporarily set vmName flag
		originalVMName := *vmName
		*vmName = vmPath

		// Run export
		if err := run(ctx, cfg, log); err != nil {
			log.Error("batch export failed", "vm", vmPath, "error", err)
			if !*quiet {
				pterm.Error.Printfln("Failed to export %s: %v", vmPath, err)
			}
			failureCount++
		} else {
			successCount++
		}

		// Restore original vmName
		*vmName = originalVMName
	}

	if !*quiet {
		pterm.DefaultSection.Println("Batch Export Summary")
		pterm.Info.Printfln("Total: %d | Success: %d | Failed: %d", len(validVMs), successCount, failureCount)
	} else {
		fmt.Printf("batch-summary: total=%d success=%d failed=%d\n", len(validVMs), successCount, failureCount)
	}

	if failureCount > 0 {
		return fmt.Errorf("%d exports failed", failureCount)
	}

	return nil
}

func verifyExport(result *vsphere.ExportResult, log logger.Logger) (map[string]string, error) {
	checksums := make(map[string]string)

	files := result.Files
	if result.OVAPath != "" {
		// Verify OVA file instead of individual files
		files = []string{result.OVAPath}
	}

	for _, filePath := range files {
		hash, err := calculateSHA256(filePath)
		if err != nil {
			log.Error("failed to calculate checksum", "file", filePath, "error", err)
			return nil, fmt.Errorf("checksum %s: %w", filePath, err)
		}
		checksums[filepath.Base(filePath)] = hash
		log.Info("calculated checksum", "file", filepath.Base(filePath), "sha256", hash)
	}

	return checksums, nil
}

func calculateSHA256(filePath string) (string, error) {
	// #nosec G304 -- filePath comes from the export result's file list, derived from the operator-supplied output directory
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func saveChecksums(filename string, checksums map[string]string) error {
	// #nosec G304 -- filename is derived from the operator-supplied output directory (checksumFile), not remote input
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	for name, hash := range checksums {
		if _, err := fmt.Fprintf(file, "%s  %s\n", hash, name); err != nil {
			return fmt.Errorf("write checksum for %s: %w", name, err)
		}
	}

	return nil
}

// displayValidationReport displays validation results in a nice table
func displayValidationReport(title string, report *ValidationReport) {
	pterm.DefaultSection.Println(title)

	// Create table data
	data := pterm.TableData{
		{"Check", "Status", "Details"},
	}

	for _, check := range report.Checks {
		var status string
		var statusColor pterm.Color

		if !check.Passed {
			status = "✗ FAIL"
			statusColor = pterm.FgRed
		} else if check.Warning {
			status = "⚠ WARN"
			statusColor = pterm.FgYellow
		} else {
			status = "✓ PASS"
			statusColor = pterm.FgGreen
		}

		data = append(data, []string{
			check.Name,
			pterm.NewStyle(statusColor).Sprint(status),
			check.Message,
		})
	}

	// Render table
	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderRowSeparator("-").
		WithBoxed().
		WithData(data).
		Render(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to render validation report table: %v\n", err)
	}

	// Summary
	if report.AllPassed {
		if report.HasWarnings {
			pterm.Info.Printfln("Validation passed with %d warnings", countWarnings(report))
		} else {
			pterm.Success.Println("All validation checks passed")
		}
	} else {
		failedCount := countFailed(report)
		pterm.Error.Printfln("%d validation check(s) failed", failedCount)
	}

	fmt.Println()
}

// countWarnings counts the number of warnings in a validation report
func countWarnings(report *ValidationReport) int {
	count := 0
	for _, check := range report.Checks {
		if check.Warning {
			count++
		}
	}
	return count
}

// countFailed counts the number of failed checks in a validation report
func countFailed(report *ValidationReport) int {
	count := 0
	for _, check := range report.Checks {
		if !check.Passed {
			count++
		}
	}
	return count
}

/*
// Old implementation commented out for reference
func runInteractiveTUI_OLD_ORIGINAL(ctx context.Context, client *vsphere.VSphereClient, cfg *config.Config, log logger.Logger) error {
	// 200+ lines of complex bubbletea initialization code
	keys := tuiKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle [x]"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		Features: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "features"),
		),
		Cloud: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "cloud"),
		),
		SplitScreen: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "split view"),
		),
		SwitchPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		Queue: key.NewBinding(
			key.WithKeys("Q"),
			key.WithHelp("Q", "queue manager"),
		),
		MoveUp: key.NewBinding(
			key.WithKeys("K", "shift+up"),
			key.WithHelp("K", "move up in queue"),
		),
		MoveDown: key.NewBinding(
			key.WithKeys("J", "shift+down"),
			key.WithHelp("J", "move down in queue"),
		),
		Priority: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "change priority"),
		),
		History: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "export history"),
		),
		FilterHistory: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter history"),
		),
		Logs: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "view logs"),
		),
		FilterLogs: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "filter log level"),
		),
		ToggleAutoScroll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "toggle auto-scroll"),
		),
		Tree: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "folder tree view"),
		),
		ExpandFolder: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("enter/space", "expand/collapse folder"),
		),
		Preview: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "preview export"),
		),
		Actions: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "quick actions"),
		),
		BulkOps: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "bulk operations"),
		),
		Compare: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "compare VMs"),
		),
		Bookmarks: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "bookmarks & filters"),
		),
		Metrics: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "performance metrics"),
		),
		FilterBuilder: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "filter builder"),
		),
		Snapshots: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "snapshot manager"),
		),
		Resources: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "resource planner"),
		),
		Migration: key.NewBinding(
			key.WithKeys("W"),
			key.WithHelp("W", "migration wizard"),
		),
	}

	// Create initial model
	m := tuiModel{
		vms:             []tuiVMItem{},
		filteredVMs:     []tuiVMItem{},
		cursor:          0,
		phase:           "loading",
		sortMode:        "name",
		client:          client,
		outputDir:       outputDirPath,
		log:             log,
		ctx:             ctx,
		progressBar:     prog,
		helpModel:       h,
		spinner:         s,
		searchInput:     si,
		keys:            keys,
		splitScreenMode: false,
		focusedPane:     "list",
		logEntries:      []logEntry{},
		logLevelFilter:  "all",
		autoScrollLogs:  true,
		showLogsPanel:   false,
		maxLogEntries:   1000,
		viewMode:        "list",
		treeItems:       []interface{}{},
		treeCursor:      0,
		exportPreviews:  []exportPreview{},
		previewCursor:   0,
		showPreview:     false,
	}

	// Run the TUI program with panic recovery for debugging
	defer func() {
		if r := recover(); r != nil {
			log.Error("TUI panic recovered", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
*/

// showVMInfoCmd retrieves and displays VM information for the command-line --vm-info flag
func showVMInfoCmd(ctx context.Context, client *vsphere.VSphereClient, vmName string, quiet bool, log logger.Logger) error {
	var spinner *pterm.SpinnerPrinter
	if !quiet {
		spinner = newOrangeSpinner(fmt.Sprintf("Retrieving information for VM: %s", vmName))
	}

	// Find VM by name to get its path
	vmPath, err := client.FindVMByName(ctx, vmName)
	if err != nil {
		if spinner != nil {
			spinner.Fail("Failed to find VM")
		}
		return fmt.Errorf("find VM %s: %w", vmName, err)
	}

	// Get VM information
	vmInfo, err := client.GetVMInfo(ctx, vmPath)
	if err != nil {
		if spinner != nil {
			spinner.Fail("Failed to retrieve VM information")
		}
		return fmt.Errorf("get VM info: %w", err)
	}

	if spinner != nil {
		spinner.Success("VM information retrieved")
	}

	// Display VM information
	if quiet {
		// Simple key=value format for scripting
		fmt.Printf("name=%s\n", vmInfo.Name)
		fmt.Printf("path=%s\n", vmInfo.Path)
		fmt.Printf("power_state=%s\n", vmInfo.PowerState)
		fmt.Printf("guest_os=%s\n", vmInfo.GuestOS)
		fmt.Printf("cpu=%d\n", vmInfo.NumCPU)
		fmt.Printf("memory_mb=%d\n", vmInfo.MemoryMB)
		fmt.Printf("storage_bytes=%d\n", vmInfo.Storage)
	} else {
		// Use existing displayVMInfo function for pretty output
		displayVMInfo(vmInfo)
	}

	return nil
}
