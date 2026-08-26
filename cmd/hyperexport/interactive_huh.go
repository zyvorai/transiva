// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/pterm/pterm"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// Export templates
type exportTemplate struct {
	name        string
	description string
	format      string
	compress    bool
	verify      bool
}

var templates = []exportTemplate{
	{
		name:        "Quick Export",
		description: "Fast export without compression (OVF)",
		format:      "ovf",
		compress:    false,
		verify:      false,
	},
	{
		name:        "Production Backup",
		description: "OVA with compression and verification",
		format:      "ova",
		compress:    true,
		verify:      true,
	},
	{
		name:        "Development",
		description: "OVF for fast development cycles",
		format:      "ovf",
		compress:    false,
		verify:      false,
	},
	{
		name:        "Archive",
		description: "Compressed OVA for long-term storage",
		format:      "ova",
		compress:    true,
		verify:      true,
	},
}

// Orange theme colors
var (
	orangePrimary = lipgloss.Color("#FF9E64") // Vibrant peach/orange
)

// Sentinel value for back navigation
const backSentinel = "<BACK>"

// runInteractiveHuh runs the new huh-based interactive TUI
func runInteractiveHuh(ctx context.Context, client *vsphere.VSphereClient, cfg *config.Config, log logger.Logger) error {
	// Set orange theme for huh
	theme := huh.ThemeBase()
	theme.Focused.Base = theme.Focused.Base.BorderForeground(orangePrimary)
	theme.Focused.Title = theme.Focused.Title.Foreground(orangePrimary).Bold(true)
	theme.Focused.SelectSelector = theme.Focused.SelectSelector.Foreground(orangePrimary)
	theme.Focused.MultiSelectSelector = theme.Focused.MultiSelectSelector.Foreground(orangePrimary)
	theme.Focused.SelectedOption = theme.Focused.SelectedOption.Foreground(orangePrimary)

	// Print banner
	printBanner()

	// Get output directory
	outputDirPath := *outputDir
	if outputDirPath == "" {
		outputDirPath = "./exports"
	}

	// Step 1: Load VMs (only once at start)
	spinner := newOrangeSpinner("Loading VMs from vSphere...")
	vms, err := client.ListVMs(ctx)
	if stopErr := spinner.Stop(); stopErr != nil {
		log.Warn("failed to stop spinner", "error", stopErr)
	}
	if err != nil {
		return fmt.Errorf("failed to load VMs: %w", err)
	}

	if len(vms) == 0 {
		pterm.Warning.Println("No VMs found")
		return nil
	}

	pterm.Success.Printf("Found %d VMs\n\n", len(vms))

	// Loop-based navigation with step counter
	currentStep := 1
	totalSteps := 3
	var selectedVMs []vsphere.VMInfo
	var exportConfig *exportConfiguration

	for {
		switch currentStep {
		case 1:
			// Step 1: VM Selection
			pterm.DefaultSection.Printf("Step %d/%d: VM Selection\n\n", currentStep, totalSteps)

			selected, err := selectVMs(vms, theme, currentStep > 1)
			if err != nil {
				return err
			}
			if len(selected) == 0 && selected != nil {
				// Back button pressed (empty slice is sentinel for back)
				if currentStep > 1 {
					currentStep--
				}
				continue
			}
			selectedVMs = selected
			currentStep++

		case 2:
			// Step 2: Export Configuration
			pterm.DefaultSection.Printf("Step %d/%d: Export Configuration\n\n", currentStep, totalSteps)

			config, err := configureExport(outputDirPath, theme, true)
			if err != nil {
				return err
			}
			if config == nil {
				// Back button pressed
				currentStep--
				continue
			}
			exportConfig = config
			currentStep++

		case 3:
			// Step 3: Confirm and Execute
			pterm.DefaultSection.Printf("Step %d/%d: Confirmation\n\n", currentStep, totalSteps)

			shouldExecute, err := confirmAndExecute(ctx, client, selectedVMs, exportConfig, log, theme, true)
			if err != nil {
				return err
			}
			if shouldExecute == -1 {
				// Back button pressed
				currentStep--
				continue
			}
			if shouldExecute == 0 {
				// Cancelled
				pterm.Info.Println("Export cancelled")
				return nil
			}
			// Success - exit
			return nil

		default:
			return nil
		}
	}
}

// selectVMs presents a multi-select interface for choosing VMs
func selectVMs(vms []vsphere.VMInfo, theme *huh.Theme, allowBack bool) ([]vsphere.VMInfo, error) {
	// Sort VMs by name
	sort.Slice(vms, func(i, j int) bool {
		return strings.ToLower(vms[i].Name) < strings.ToLower(vms[j].Name)
	})

	// Create options for multi-select
	options := make([]huh.Option[string], len(vms))
	vmMap := make(map[string]vsphere.VMInfo)

	for i, vm := range vms {
		// Format VM info
		powerIcon := "○"
		if vm.PowerState == "poweredOn" {
			powerIcon = "⚡"
		}

		label := fmt.Sprintf("%s %-30s │ %2d CPU │ %4.0f GB RAM │ %s",
			powerIcon,
			truncate(vm.Name, 30),
			vm.NumCPU,
			float64(vm.MemoryMB)/1024,
			formatBytes(vm.Storage),
		)

		options[i] = huh.NewOption(label, vm.Path)
		vmMap[vm.Path] = vm
	}

	// Add back button option if allowed
	if allowBack {
		options = append(options, huh.NewOption("← Go Back", backSentinel))
	}

	var selectedPaths []string

	// Loop until at least one VM is selected or back is chosen
	for {
		description := "Use arrow keys to navigate, space to select, enter to confirm"
		if allowBack {
			description += " | Select '← Go Back' to return to previous step"
		}

		// Create the form with multi-select and orange theme
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select VMs to Export").
					Description(description).
					Options(options...).
					Value(&selectedPaths).
					Height(15).
					Filterable(true),
			),
		).WithTheme(theme)

		if err := form.Run(); err != nil {
			return nil, err
		}

		// Check if back button was selected
		if allowBack {
			for _, path := range selectedPaths {
				if path == backSentinel {
					// Return empty slice as signal for back navigation
					return []vsphere.VMInfo{}, nil
				}
			}
		}

		// Check if at least one VM is selected
		if len(selectedPaths) == 0 {
			pterm.Warning.Println("Please select at least one VM to export")
			pterm.Println()
			continue
		}

		// Valid selection - break out of loop
		break
	}

	// Map selected paths back to VM objects
	selected := make([]vsphere.VMInfo, 0, len(selectedPaths))
	for _, path := range selectedPaths {
		if path == backSentinel {
			continue // Skip back button sentinel
		}
		if vm, ok := vmMap[path]; ok {
			selected = append(selected, vm)
		}
	}

	return selected, nil
}

// exportConfiguration holds export settings
type exportConfiguration struct {
	outputDir    string
	templateName string
	format       string
	compress     bool
	verify       bool
	parallel     int
	parallelStr  string // for form input

	// hyper2kvm daemon options
	useDaemon          bool
	daemonInstance     string
	daemonWatchDir     string
	daemonOutputDir    string
	daemonPollInterval int
	daemonTimeout      int
	pollIntervalStr    string // for form input
	daemonTimeoutStr   string // for form input
}

// configureExport presents export configuration options
func configureExport(defaultOutputDir string, theme *huh.Theme, allowBack bool) (*exportConfiguration, error) {
	config := &exportConfiguration{
		outputDir:          defaultOutputDir,
		parallel:           4,
		parallelStr:        "4",
		daemonWatchDir:     "/var/lib/hyper2kvm/queue",
		daemonOutputDir:    "/var/lib/hyper2kvm/output",
		daemonPollInterval: 5,
		daemonTimeout:      60,
		pollIntervalStr:    "5",
		daemonTimeoutStr:   "60",
	}

	// Template selection
	templateOptions := make([]huh.Option[string], len(templates))
	for i, t := range templates {
		templateOptions[i] = huh.NewOption(
			fmt.Sprintf("%s - %s", t.name, t.description),
			t.name,
		)
	}

	// Add back button option to template selection if allowed
	if allowBack {
		templateOptions = append(templateOptions, huh.NewOption("← Go Back", backSentinel))
	}

	var templateName string
	var configureDaemonStr string
	var useDaemonStr string

	// Build daemon mode options with back button
	daemonOptions := []huh.Option[string]{
		huh.NewOption("Yes, use daemon", "yes"),
		huh.NewOption("No, direct execution", "no"),
	}
	if allowBack {
		daemonOptions = append(daemonOptions, huh.NewOption("← Go Back", backSentinel))
	}

	// Build configure daemon options with back button
	configureDaemonOptions := []huh.Option[string]{
		huh.NewOption("Yes, customize", "yes"),
		huh.NewOption("No, use defaults", "no"),
	}
	if allowBack {
		configureDaemonOptions = append(configureDaemonOptions, huh.NewOption("← Go Back", backSentinel))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Export Template").
				Description("Choose a predefined export configuration").
				Options(templateOptions...).
				Value(&templateName),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Output Directory").
				Description("Where to save exported VMs (press Esc to go back)").
				Value(&config.outputDir).
				Placeholder(defaultOutputDir),

			huh.NewInput().
				Title("Parallel Downloads").
				Description("Number of concurrent file downloads (1-8, press Esc to go back)").
				Value(&config.parallelStr).
				Placeholder("4").
				Validate(func(s string) error {
					var num int
					if _, err := fmt.Sscanf(s, "%d", &num); err != nil {
						return fmt.Errorf("must be a number")
					}
					if num < 1 || num > 8 {
						return fmt.Errorf("must be between 1 and 8")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("hyper2kvm Daemon Mode").
				Description("Use systemd daemon for VM conversion?").
				Options(daemonOptions...).
				Value(&useDaemonStr),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Configure Daemon").
				Description("Customize daemon settings (instance, directories, timeouts)?").
				Options(configureDaemonOptions...).
				Value(&configureDaemonStr),
		).WithHideFunc(func() bool {
			return useDaemonStr != "yes"
		}),
		huh.NewGroup(
			huh.NewInput().
				Title("Daemon Instance").
				Description("Systemd instance name (empty for default, press Esc to go back)").
				Value(&config.daemonInstance).
				Placeholder("(default)"),

			huh.NewInput().
				Title("Watch Directory").
				Description("Directory where daemon watches for jobs (press Esc to go back)").
				Value(&config.daemonWatchDir).
				Placeholder("/var/lib/hyper2kvm/queue"),

			huh.NewInput().
				Title("Output Directory").
				Description("Directory where daemon outputs converted VMs (press Esc to go back)").
				Value(&config.daemonOutputDir).
				Placeholder("/var/lib/hyper2kvm/output"),

			huh.NewInput().
				Title("Poll Interval (seconds)").
				Description("How often to check for completion (1-60, press Esc to go back)").
				Value(&config.pollIntervalStr).
				Placeholder("5").
				Validate(func(s string) error {
					var num int
					if _, err := fmt.Sscanf(s, "%d", &num); err != nil {
						return fmt.Errorf("must be a number")
					}
					if num < 1 || num > 60 {
						return fmt.Errorf("must be between 1 and 60")
					}
					return nil
				}),

			huh.NewInput().
				Title("Daemon Timeout (minutes)").
				Description("Maximum time to wait for daemon (1-240, press Esc to go back)").
				Value(&config.daemonTimeoutStr).
				Placeholder("60").
				Validate(func(s string) error {
					var num int
					if _, err := fmt.Sscanf(s, "%d", &num); err != nil {
						return fmt.Errorf("must be a number")
					}
					if num < 1 || num > 240 {
						return fmt.Errorf("must be between 1 and 240")
					}
					return nil
				}),
		).WithHideFunc(func() bool {
			return useDaemonStr != "yes" || configureDaemonStr != "yes"
		}),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Check if back button was selected in any field
	if allowBack && (templateName == backSentinel || useDaemonStr == backSentinel || configureDaemonStr == backSentinel) {
		return nil, nil // Return nil as signal for back navigation
	}

	// Convert string selections to booleans
	config.useDaemon = (useDaemonStr == "yes")

	// Convert parallel string to int
	if config.parallelStr != "" {
		if _, err := fmt.Sscanf(config.parallelStr, "%d", &config.parallel); err != nil {
			return nil, fmt.Errorf("invalid parallel downloads value: %w", err)
		}
	}

	// Convert daemon numeric strings to int
	if config.useDaemon {
		if config.pollIntervalStr != "" {
			if _, err := fmt.Sscanf(config.pollIntervalStr, "%d", &config.daemonPollInterval); err != nil {
				return nil, fmt.Errorf("invalid poll interval value: %w", err)
			}
		}
		if config.daemonTimeoutStr != "" {
			if _, err := fmt.Sscanf(config.daemonTimeoutStr, "%d", &config.daemonTimeout); err != nil {
				return nil, fmt.Errorf("invalid daemon timeout value: %w", err)
			}
		}
	}

	// Apply template settings
	for _, t := range templates {
		if t.name == templateName {
			config.format = t.format
			config.compress = t.compress
			config.verify = t.verify
			config.templateName = t.name
			break
		}
	}

	return config, nil
}

// confirmAndExecute shows summary and executes the export
// Returns: 1 = executed successfully, 0 = cancelled, -1 = go back
func confirmAndExecute(ctx context.Context, client *vsphere.VSphereClient, vms []vsphere.VMInfo, cfg *exportConfiguration, log logger.Logger, theme *huh.Theme, allowBack bool) (int, error) {
	// Calculate totals
	var totalCPU int32
	var totalMemoryMB int32
	var totalStorage int64

	for _, vm := range vms {
		totalCPU += vm.NumCPU
		totalMemoryMB += vm.MemoryMB
		totalStorage += vm.Storage
	}

	// Build summary with pterm for better terminal compatibility
	colorOrange := pterm.NewRGB(211, 84, 0)

	pterm.DefaultSection.Println("Export Summary")
	pterm.Println()

	// VM summary
	pterm.Printf("  %s  %d\n", colorOrange.Sprint("VMs Selected:     "), len(vms))
	pterm.Printf("  %s  %d\n", colorOrange.Sprint("Total CPUs:       "), totalCPU)
	pterm.Printf("  %s  %.1f GB\n", colorOrange.Sprint("Total Memory:     "), float64(totalMemoryMB)/1024)
	pterm.Printf("  %s  %s\n", colorOrange.Sprint("Total Storage:    "), formatBytes(totalStorage))
	pterm.Println()

	// Configuration
	pterm.Printf("  %s  %s\n", colorOrange.Sprint("Template:         "), cfg.templateName)
	pterm.Printf("  %s  %s\n", colorOrange.Sprint("Format:           "), strings.ToUpper(cfg.format))
	pterm.Printf("  %s  %v\n", colorOrange.Sprint("Compression:      "), cfg.compress)
	pterm.Printf("  %s  %v\n", colorOrange.Sprint("Verification:     "), cfg.verify)
	pterm.Printf("  %s  %d\n", colorOrange.Sprint("Parallel:         "), cfg.parallel)
	pterm.Printf("  %s  %s\n", colorOrange.Sprint("Output Directory: "), cfg.outputDir)

	// Daemon configuration (if enabled)
	if cfg.useDaemon {
		pterm.Println()
		pterm.Printf("  %s\n", colorOrange.Sprint("Daemon Configuration:"))
		daemonMode := "Default"
		if cfg.daemonInstance != "" {
			daemonMode = fmt.Sprintf("Instance: %s", cfg.daemonInstance)
		}
		pterm.Printf("  %s  %s\n", colorOrange.Sprint("  Mode:           "), daemonMode)
		pterm.Printf("  %s  %s\n", colorOrange.Sprint("  Watch Dir:      "), cfg.daemonWatchDir)
		pterm.Printf("  %s  %s\n", colorOrange.Sprint("  Output Dir:     "), cfg.daemonOutputDir)
		pterm.Printf("  %s  %ds\n", colorOrange.Sprint("  Poll Interval:  "), cfg.daemonPollInterval)
		pterm.Printf("  %s  %dm\n", colorOrange.Sprint("  Timeout:        "), cfg.daemonTimeout)
	}
	pterm.Println()

	// Confirmation with back button option
	confirmOptions := []huh.Option[string]{
		huh.NewOption("Yes, export!", "export"),
		huh.NewOption("Cancel", "cancel"),
	}
	if allowBack {
		confirmOptions = append(confirmOptions, huh.NewOption("← Go Back", backSentinel))
	}

	var action string
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Proceed with Export?").
				Description("This will export the selected VMs").
				Options(confirmOptions...).
				Value(&action),
		),
	).WithTheme(theme)

	if err := confirmForm.Run(); err != nil {
		return 0, err
	}

	// Check action
	if action == backSentinel {
		return -1, nil // Go back
	}
	if action == "cancel" {
		return 0, nil // Cancelled
	}

	// Execute exports
	pterm.Info.Println("\nStarting export...")
	fmt.Println()

	for i, vm := range vms {
		pterm.DefaultSection.Printf("Exporting VM %d/%d: %s", i+1, len(vms), vm.Name)

		sanitized := sanitizeFilename(vm.Name)
		vmOutputDir := filepath.Join(cfg.outputDir, sanitized)

		// Validate path to prevent directory traversal
		absOutputDir, err := filepath.Abs(cfg.outputDir)
		if err != nil {
			return 0, fmt.Errorf("resolve output directory path: %w", err)
		}
		absVMDir, err := filepath.Abs(vmOutputDir)
		if err != nil {
			return 0, fmt.Errorf("resolve VM output directory path: %w", err)
		}
		if !strings.HasPrefix(absVMDir, absOutputDir+string(filepath.Separator)) {
			return 0, fmt.Errorf("security: invalid VM name would escape output directory")
		}

		if err := os.MkdirAll(vmOutputDir, 0750); err != nil {
			return 0, fmt.Errorf("create output directory: %w", err)
		}

		// Create export options
		opts := vsphere.ExportOptions{
			Format:            cfg.format,
			OutputPath:        vmOutputDir,
			ParallelDownloads: cfg.parallel,
			ValidateChecksum:  cfg.verify,
			Compress:          cfg.compress,
			CleanupOVF:        cfg.format == "ova", // Clean up OVF files after creating OVA

			// hyper2kvm daemon options
			Hyper2KVMDaemon:        cfg.useDaemon,
			Hyper2KVMInstance:      cfg.daemonInstance,
			Hyper2KVMWatchDir:      cfg.daemonWatchDir,
			Hyper2KVMOutputDir:     cfg.daemonOutputDir,
			Hyper2KVMPollInterval:  cfg.daemonPollInterval,
			Hyper2KVMDaemonTimeout: cfg.daemonTimeout,
		}

		// Export the VM
		spinner := newOrangeSpinner(fmt.Sprintf("Exporting %s...", vm.Name))

		startTime := time.Now()
		result, err := client.ExportOVF(ctx, vm.Path, opts)
		duration := time.Since(startTime)

		if stopErr := spinner.Stop(); stopErr != nil {
			log.Warn("failed to stop spinner", "error", stopErr)
		}

		if err != nil {
			pterm.Error.Printf("Failed to export %s: %v\n", vm.Name, err)

			// Ask if user wants to continue
			var continueExport bool
			continueForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Continue with remaining VMs?").
						Value(&continueExport),
				),
			).WithTheme(theme)
			if err := continueForm.Run(); err != nil || !continueExport {
				return 0, fmt.Errorf("export aborted")
			}
			continue
		}

		exportPath := result.OVFPath
		if result.Format == "ova" {
			exportPath = result.OVAPath
		}

		pterm.Success.Printf("Exported %s to %s (took %s)\n", vm.Name, exportPath, duration.Round(time.Second))
		log.Info("VM exported successfully",
			"vm", vm.Name,
			"path", exportPath,
			"format", result.Format,
			"size", formatBytes(result.TotalSize),
			"duration", duration.String(),
		)
		fmt.Println()
	}

	// Final summary
	pterm.Success.Printf("\n✓ Successfully exported %d VMs to %s\n", len(vms), cfg.outputDir)

	return 1, nil // Successfully executed
}

// Helper functions
func printBanner() {
	// Orange colors
	colorOrange := pterm.NewRGB(211, 84, 0)  // Deep orange #D35400
	styleOrange := pterm.NewStyle(pterm.FgLightRed) // Background style

	pterm.Println()
	pterm.DefaultCenter.WithCenterEachLineSeparately().Println(
		pterm.DefaultHeader.WithFullWidth(false).
			WithBackgroundStyle(styleOrange).
			WithTextStyle(pterm.NewStyle(pterm.FgBlack)).
			WithMargin(4).
			Sprint("    HyperSDK    "),
	)
	pterm.Println()

	// Subtitle and version with orange color
	subtitle := pterm.DefaultCenter.Sprint(
		pterm.NewStyle(pterm.FgYellow).Sprint("Interactive VM export tool"))
	version := pterm.DefaultCenter.Sprint(
		colorOrange.Sprint("Version 1.0.0"))

	pterm.Println(subtitle)
	pterm.Println(version)
	pterm.Println()

	pterm.Info.Println("Use arrow keys to navigate, space to select, enter to confirm")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func sanitizeFilename(name string) string {
	// Replace invalid filename characters
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}
