// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// ExportHistoryEntry represents a single export in the history
type ExportHistoryEntry struct {
	Timestamp    time.Time         `json:"timestamp"`
	VMName       string            `json:"vm_name"`
	VMPath       string            `json:"vm_path"`
	Provider     string            `json:"provider"`
	Format       string            `json:"format"`
	OutputDir    string            `json:"output_dir"`
	TotalSize    int64             `json:"total_size"`
	Duration     time.Duration     `json:"duration"`
	FilesCount   int               `json:"files_count"`
	Success      bool              `json:"success"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Compressed   bool              `json:"compressed"`
	Verified     bool              `json:"verified"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ExportHistory manages export history
type ExportHistory struct {
	historyFile string
	log         logger.Logger
	mu          sync.Mutex
}

// NewExportHistory creates a new export history manager
func NewExportHistory(historyFile string, log logger.Logger) *ExportHistory {
	return &ExportHistory{
		historyFile: historyFile,
		log:         log,
	}
}

// GetDefaultHistoryFile returns the default history file path
func GetDefaultHistoryFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".hyperexport")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	return filepath.Join(configDir, "history.json"), nil
}

// RecordExport adds an export entry to the history
func (h *ExportHistory) RecordExport(entry ExportHistoryEntry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Load existing history
	entries, err := h.loadHistory()
	if err != nil {
		h.log.Warn("failed to load existing history, starting fresh", "error", err)
		entries = []ExportHistoryEntry{}
	}

	// Add new entry
	entries = append(entries, entry)

	// Keep only last 1000 entries to prevent unbounded growth
	if len(entries) > 1000 {
		entries = entries[len(entries)-1000:]
	}

	// Save updated history
	if err := h.saveHistory(entries); err != nil {
		return fmt.Errorf("save history: %w", err)
	}

	h.log.Debug("export recorded in history", "vm", entry.VMName, "success", entry.Success)
	return nil
}

// GetHistory returns all export history entries
func (h *ExportHistory) GetHistory() ([]ExportHistoryEntry, error) {
	return h.loadHistory()
}

// GetRecentExports returns the N most recent exports
func (h *ExportHistory) GetRecentExports(limit int) ([]ExportHistoryEntry, error) {
	entries, err := h.loadHistory()
	if err != nil {
		return nil, err
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

// GetExportsByVM returns all exports for a specific VM
func (h *ExportHistory) GetExportsByVM(vmName string) ([]ExportHistoryEntry, error) {
	entries, err := h.loadHistory()
	if err != nil {
		return nil, err
	}

	var vmExports []ExportHistoryEntry
	for _, entry := range entries {
		if entry.VMName == vmName {
			vmExports = append(vmExports, entry)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(vmExports, func(i, j int) bool {
		return vmExports[i].Timestamp.After(vmExports[j].Timestamp)
	})

	return vmExports, nil
}

// GetStatistics calculates export statistics
func (h *ExportHistory) GetStatistics() (*ExportStatistics, error) {
	entries, err := h.loadHistory()
	if err != nil {
		return nil, err
	}

	stats := &ExportStatistics{
		TotalExports:      len(entries),
		SuccessfulCount:   0,
		FailedCount:       0,
		TotalDataExported: 0,
		AverageDuration:   0,
		FormatCounts:      make(map[string]int),
		ProviderCounts:    make(map[string]int),
	}

	var totalDuration time.Duration

	for _, entry := range entries {
		if entry.Success {
			stats.SuccessfulCount++
		} else {
			stats.FailedCount++
		}

		stats.TotalDataExported += entry.TotalSize
		totalDuration += entry.Duration

		stats.FormatCounts[entry.Format]++
		stats.ProviderCounts[entry.Provider]++
	}

	if len(entries) > 0 {
		stats.AverageDuration = totalDuration / time.Duration(len(entries))
		stats.SuccessRate = float64(stats.SuccessfulCount) / float64(len(entries)) * 100
	}

	return stats, nil
}

// ClearHistory removes all history entries
func (h *ExportHistory) ClearHistory() error {
	if err := os.Remove(h.historyFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove history file: %w", err)
	}
	h.log.Info("export history cleared")
	return nil
}

// loadHistory loads history from file
func (h *ExportHistory) loadHistory() ([]ExportHistoryEntry, error) {
	file, err := os.Open(h.historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []ExportHistoryEntry{}, nil
		}
		return nil, fmt.Errorf("open history file: %w", err)
	}
	defer file.Close()

	var entries []ExportHistoryEntry
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}

	return entries, nil
}

// saveHistory saves history to file
func (h *ExportHistory) saveHistory(entries []ExportHistoryEntry) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(h.historyFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	file, err := os.Create(h.historyFile)
	if err != nil {
		return fmt.Errorf("create history file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		return fmt.Errorf("encode history: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync history file: %w", err)
	}

	return nil
}

// ExportStatistics contains aggregated statistics
type ExportStatistics struct {
	TotalExports      int
	SuccessfulCount   int
	FailedCount       int
	SuccessRate       float64
	TotalDataExported int64
	AverageDuration   time.Duration
	FormatCounts      map[string]int
	ProviderCounts    map[string]int
}

// ExportReport generates a formatted report
type ExportReport struct {
	history *ExportHistory
}

// NewExportReport creates a new export report generator
func NewExportReport(history *ExportHistory) *ExportReport {
	return &ExportReport{history: history}
}

// GenerateReport generates a formatted text report
func (r *ExportReport) GenerateReport(includeHistory bool, historyLimit int) (string, error) {
	stats, err := r.history.GetStatistics()
	if err != nil {
		return "", fmt.Errorf("get statistics: %w", err)
	}

	report := "=== HyperExport Report ===\n\n"
	report += "## Summary Statistics\n\n"
	report += fmt.Sprintf("Total Exports: %d\n", stats.TotalExports)
	report += fmt.Sprintf("Successful: %d (%.1f%%)\n", stats.SuccessfulCount, stats.SuccessRate)
	report += fmt.Sprintf("Failed: %d\n", stats.FailedCount)
	report += fmt.Sprintf("Total Data Exported: %s\n", formatBytes(stats.TotalDataExported))
	report += fmt.Sprintf("Average Duration: %s\n", stats.AverageDuration.Round(time.Second))
	report += "\n"

	if len(stats.FormatCounts) > 0 {
		report += "## Export Formats\n\n"
		for format, count := range stats.FormatCounts {
			report += fmt.Sprintf("  %s: %d\n", format, count)
		}
		report += "\n"
	}

	if len(stats.ProviderCounts) > 0 {
		report += "## Providers\n\n"
		for provider, count := range stats.ProviderCounts {
			report += fmt.Sprintf("  %s: %d\n", provider, count)
		}
		report += "\n"
	}

	if includeHistory {
		entries, err := r.history.GetRecentExports(historyLimit)
		if err != nil {
			return report, fmt.Errorf("get recent exports: %w", err)
		}

		report += fmt.Sprintf("## Recent Exports (Last %d)\n\n", len(entries))
		for i, entry := range entries {
			status := "✓"
			if !entry.Success {
				status = "✗"
			}

			report += fmt.Sprintf("%d. [%s] %s\n", i+1, status, entry.VMName)
			report += fmt.Sprintf("   Time: %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))
			report += fmt.Sprintf("   Format: %s | Size: %s | Duration: %s\n",
				entry.Format,
				formatBytes(entry.TotalSize),
				entry.Duration.Round(time.Second))

			if !entry.Success && entry.ErrorMessage != "" {
				report += fmt.Sprintf("   Error: %s\n", entry.ErrorMessage)
			}
			report += "\n"
		}
	}

	return report, nil
}

// SaveReportToFile saves the report to a file
func (r *ExportReport) SaveReportToFile(filename string, includeHistory bool, historyLimit int) error {
	report, err := r.GenerateReport(includeHistory, historyLimit)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, []byte(report), 0600); err != nil {
		return fmt.Errorf("write report file: %w", err)
	}

	return nil
}
