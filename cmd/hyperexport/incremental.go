// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pterm/pterm"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers/incremental"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// showIncrementalInfo displays information about incremental export potential
func showIncrementalInfo(ctx context.Context, client *vsphere.VSphereClient, vmPath string, log logger.Logger) error {
	// Get metadata directory
	metadataDir := getIncrementalMetadataDir()

	// Create change tracker
	tracker, err := incremental.NewChangeTracker(metadataDir, log)
	if err != nil {
		return fmt.Errorf("create change tracker: %w", err)
	}

	// Check CBT status
	cbtEnabled, err := client.IsCBTEnabled(ctx, vmPath)
	if err != nil {
		pterm.Warning.Printfln("Failed to check CBT status: %v", err)
		cbtEnabled = false
	}

	// Get current disk metadata
	disks, err := client.GetDiskMetadata(ctx, vmPath)
	if err != nil {
		return fmt.Errorf("get disk metadata: %w", err)
	}

	// Get last export
	vmID := sanitizeForPath(vmPath)
	lastExport, err := tracker.GetLastExport(ctx, vmID)
	if err != nil {
		log.Warn("failed to get last export", "error", err)
	}

	// Check if incremental is possible
	canIncremental, reason := tracker.IsIncrementalPossible(ctx, vmID, disks)

	// Display information
	pterm.DefaultSection.Println("Incremental Export Analysis")

	// CBT Status
	cbtStatus := pterm.Red("Disabled")
	if cbtEnabled {
		cbtStatus = pterm.Green("Enabled")
	}
	pterm.Info.Printfln("Changed Block Tracking (CBT): %s", cbtStatus)

	// Disk information
	pterm.Info.Printfln("Current Disks: %d", len(disks))
	for _, disk := range disks {
		fmt.Printf("  • %s (%s) - %s\n",
			disk.Key,
			formatBytes(disk.CapacityBytes),
			disk.BackingInfo)
		if disk.ChangeID != "" {
			fmt.Printf("    ChangeID: %s\n", disk.ChangeID)
		}
	}

	// Last export information
	if lastExport != nil {
		pterm.DefaultSection.Println("Last Export")
		pterm.Info.Printfln("Time: %s", lastExport.ExportTime.Format("2006-01-02 15:04:05"))
		pterm.Info.Printfln("Size: %s", formatBytes(lastExport.TotalSize))
		pterm.Info.Printfln("Disks: %d", len(lastExport.DiskInfo))
	} else {
		pterm.Info.Println("No previous export found")
	}

	// Incremental potential
	pterm.DefaultSection.Println("Incremental Export Status")
	if canIncremental {
		pterm.Success.Println("✓ Incremental export is possible")

		// Estimate savings
		estimatedChanged, err := tracker.EstimateChangedSize(ctx, vmID, disks)
		if err != nil {
			log.Warn("failed to estimate changed size", "error", err)
		} else {
			totalSize := int64(0)
			for _, disk := range disks {
				totalSize += disk.CapacityBytes
			}
			savings := totalSize - estimatedChanged
			savingsPercent := float64(savings) / float64(totalSize) * 100

			pterm.Info.Printfln("Estimated changed data: %s", formatBytes(estimatedChanged))
			pterm.Info.Printfln("Potential savings: %s (%.1f%%)", formatBytes(savings), savingsPercent)
		}
	} else {
		pterm.Warning.Println("✗ Full export required")
		pterm.Info.Printfln("Reason: %s", reason)
	}

	return nil
}

// getIncrementalMetadataDir returns the directory for incremental metadata
func getIncrementalMetadataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/var/lib/transiva/incremental"
	}
	return filepath.Join(homeDir, ".transiva", "incremental")
}

// recordIncrementalExport records incremental export metadata
func recordIncrementalExport(ctx context.Context, vmPath string, exportResult *vsphere.ExportResult, log logger.Logger) error {
	// Get metadata directory
	metadataDir := getIncrementalMetadataDir()

	// Create change tracker
	tracker, err := incremental.NewChangeTracker(metadataDir, log)
	if err != nil {
		return fmt.Errorf("create change tracker: %w", err)
	}

	// Get VM ID
	vmID := sanitizeForPath(vmPath)

	// Get current snapshot ID (if any)
	snapshotID := ""
	// TODO: Extract from export result if available

	// Create metadata
	metadata := &incremental.ExportMetadata{
		VMID:         vmID,
		VMName:       filepath.Base(vmPath),
		ExportTime:   time.Now(),
		SnapshotID:   snapshotID,
		ExportPath:   exportResult.OutputDir,
		TotalSize:    exportResult.TotalSize,
		DiskInfo:     []incremental.DiskMetadata{},
	}

	// Record the export
	if err := tracker.RecordExport(ctx, metadata); err != nil {
		return fmt.Errorf("record export: %w", err)
	}

	log.Info("recorded incremental export metadata", "vm", vmID)
	return nil
}
