// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewExportHistory(t *testing.T) {
	historyFile := "/tmp/history.json"
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	if history == nil {
		t.Fatal("NewExportHistory returned nil")
	}
	if history.historyFile != historyFile {
		t.Error("History file path mismatch")
	}
}

func TestGetDefaultHistoryFile(t *testing.T) {
	historyFile, err := GetDefaultHistoryFile()
	if err != nil {
		t.Fatalf("GetDefaultHistoryFile failed: %v", err)
	}

	if historyFile == "" {
		t.Error("History file path should not be empty")
	}

	if !strings.HasSuffix(historyFile, "history.json") {
		t.Errorf("History file should end with 'history.json', got %s", historyFile)
	}

	// Verify directory was created
	dir := filepath.Dir(historyFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Config directory should have been created")
	}
}

func TestExportHistory_RecordExport(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	entry := ExportHistoryEntry{
		Timestamp:  time.Now(),
		VMName:     "test-vm",
		VMPath:     "/datacenter/vm/test-vm",
		Provider:   "vsphere",
		Format:     "ova",
		OutputDir:  "/exports",
		TotalSize:  1024 * 1024,
		Duration:   5 * time.Minute,
		FilesCount: 3,
		Success:    true,
		Compressed: false,
		Verified:   true,
	}

	err := history.RecordExport(entry)
	if err != nil {
		t.Fatalf("RecordExport failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("History file should have been created")
	}
}

func TestExportHistory_GetHistory(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record multiple entries
	for i := 0; i < 5; i++ {
		entry := ExportHistoryEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			VMName:    "vm-" + string(rune('0'+i)),
			Provider:  "vsphere",
			Format:    "ova",
			Success:   true,
		}
		if err := history.RecordExport(entry); err != nil {
			t.Fatalf("RecordExport failed: %v", err)
		}
	}

	// Get all history
	entries, err := history.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(entries))
	}
}

func TestExportHistory_GetHistory_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nonexistent.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	entries, err := history.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for nonexistent file, got %d", len(entries))
	}
}

func TestExportHistory_GetRecentExports(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record 10 entries with different timestamps
	for i := 0; i < 10; i++ {
		entry := ExportHistoryEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Hour),
			VMName:    "vm-" + string(rune('0'+i)),
			Provider:  "vsphere",
			Format:    "ova",
			Success:   true,
		}
		if err := history.RecordExport(entry); err != nil {
			t.Fatalf("RecordExport failed: %v", err)
		}
	}

	// Get 5 most recent
	recent, err := history.GetRecentExports(5)
	if err != nil {
		t.Fatalf("GetRecentExports failed: %v", err)
	}

	if len(recent) != 5 {
		t.Errorf("Expected 5 recent entries, got %d", len(recent))
	}

	// Verify they're sorted by timestamp (newest first)
	for i := 0; i < len(recent)-1; i++ {
		if recent[i].Timestamp.Before(recent[i+1].Timestamp) {
			t.Error("Recent exports should be sorted newest first")
		}
	}
}

func TestExportHistory_GetExportsByVM(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record entries for different VMs
	vms := []string{"vm-a", "vm-b", "vm-a", "vm-c", "vm-a"}
	for _, vmName := range vms {
		entry := ExportHistoryEntry{
			Timestamp: time.Now(),
			VMName:    vmName,
			Provider:  "vsphere",
			Format:    "ova",
			Success:   true,
		}
		if err := history.RecordExport(entry); err != nil {
			t.Fatalf("RecordExport failed: %v", err)
		}
	}

	// Get exports for vm-a
	vmExports, err := history.GetExportsByVM("vm-a")
	if err != nil {
		t.Fatalf("GetExportsByVM failed: %v", err)
	}

	if len(vmExports) != 3 {
		t.Errorf("Expected 3 exports for vm-a, got %d", len(vmExports))
	}

	// Verify all are for correct VM
	for _, entry := range vmExports {
		if entry.VMName != "vm-a" {
			t.Errorf("Expected VM name 'vm-a', got %s", entry.VMName)
		}
	}
}

func TestExportHistory_GetExportsByVM_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record entry for different VM
	entry := ExportHistoryEntry{
		Timestamp: time.Now(),
		VMName:    "vm-a",
		Provider:  "vsphere",
		Format:    "ova",
		Success:   true,
	}
	history.RecordExport(entry)

	// Get exports for non-existent VM
	vmExports, err := history.GetExportsByVM("nonexistent-vm")
	if err != nil {
		t.Fatalf("GetExportsByVM failed: %v", err)
	}

	if len(vmExports) != 0 {
		t.Errorf("Expected 0 exports for nonexistent VM, got %d", len(vmExports))
	}
}

func TestExportHistory_GetStatistics(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record mix of successful and failed exports
	entries := []ExportHistoryEntry{
		{Timestamp: time.Now(), VMName: "vm-1", Provider: "vsphere", Format: "ova", TotalSize: 1000, Duration: 1 * time.Minute, Success: true},
		{Timestamp: time.Now(), VMName: "vm-2", Provider: "vsphere", Format: "vmdk", TotalSize: 2000, Duration: 2 * time.Minute, Success: true},
		{Timestamp: time.Now(), VMName: "vm-3", Provider: "aws", Format: "ova", TotalSize: 1500, Duration: 3 * time.Minute, Success: false},
		{Timestamp: time.Now(), VMName: "vm-4", Provider: "azure", Format: "vhd", TotalSize: 3000, Duration: 4 * time.Minute, Success: true},
	}

	for _, entry := range entries {
		if err := history.RecordExport(entry); err != nil {
			t.Fatalf("RecordExport failed: %v", err)
		}
	}

	stats, err := history.GetStatistics()
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if stats.TotalExports != 4 {
		t.Errorf("Expected 4 total exports, got %d", stats.TotalExports)
	}
	if stats.SuccessfulCount != 3 {
		t.Errorf("Expected 3 successful, got %d", stats.SuccessfulCount)
	}
	if stats.FailedCount != 1 {
		t.Errorf("Expected 1 failed, got %d", stats.FailedCount)
	}
	if stats.TotalDataExported != 7500 {
		t.Errorf("Expected total data 7500, got %d", stats.TotalDataExported)
	}
	if stats.SuccessRate != 75.0 {
		t.Errorf("Expected success rate 75%%, got %.1f%%", stats.SuccessRate)
	}

	// Verify format counts
	if stats.FormatCounts["ova"] != 2 {
		t.Error("Expected 2 ova exports")
	}
	if stats.FormatCounts["vmdk"] != 1 {
		t.Error("Expected 1 vmdk export")
	}
	if stats.FormatCounts["vhd"] != 1 {
		t.Error("Expected 1 vhd export")
	}

	// Verify provider counts
	if stats.ProviderCounts["vsphere"] != 2 {
		t.Error("Expected 2 vsphere exports")
	}
	if stats.ProviderCounts["aws"] != 1 {
		t.Error("Expected 1 aws export")
	}
	if stats.ProviderCounts["azure"] != 1 {
		t.Error("Expected 1 azure export")
	}
}

func TestExportHistory_GetStatistics_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	stats, err := history.GetStatistics()
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if stats.TotalExports != 0 {
		t.Errorf("Expected 0 total exports, got %d", stats.TotalExports)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("Expected 0%% success rate, got %.1f%%", stats.SuccessRate)
	}
}

func TestExportHistory_ClearHistory(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record some entries
	entry := ExportHistoryEntry{
		Timestamp: time.Now(),
		VMName:    "test-vm",
		Provider:  "vsphere",
		Format:    "ova",
		Success:   true,
	}
	history.RecordExport(entry)

	// Clear history
	err := history.ClearHistory()
	if err != nil {
		t.Fatalf("ClearHistory failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(historyFile); !os.IsNotExist(err) {
		t.Error("History file should have been deleted")
	}

	// Get history should return empty
	entries, err := history.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(entries))
	}
}

func TestExportHistory_ClearHistory_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nonexistent.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Should not error when clearing non-existent file
	err := history.ClearHistory()
	if err != nil {
		t.Errorf("ClearHistory should not fail on nonexistent file: %v", err)
	}
}

func TestExportHistory_MaxEntriesLimit(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record 1100 entries (more than 1000 limit)
	for i := 0; i < 1100; i++ {
		entry := ExportHistoryEntry{
			Timestamp: time.Now(),
			VMName:    "vm-test",
			Provider:  "vsphere",
			Format:    "ova",
			Success:   true,
		}
		if err := history.RecordExport(entry); err != nil {
			t.Fatalf("RecordExport failed: %v", err)
		}
	}

	// Get history - should be capped at 1000
	entries, err := history.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(entries) != 1000 {
		t.Errorf("Expected 1000 entries (max limit), got %d", len(entries))
	}
}

func TestExportHistoryEntry_Fields(t *testing.T) {
	now := time.Now()
	metadata := map[string]string{"key": "value"}

	entry := ExportHistoryEntry{
		Timestamp:    now,
		VMName:       "test-vm",
		VMPath:       "/datacenter/vm/test",
		Provider:     "vsphere",
		Format:       "ova",
		OutputDir:    "/exports",
		TotalSize:    1024,
		Duration:     5 * time.Minute,
		FilesCount:   3,
		Success:      true,
		ErrorMessage: "",
		Compressed:   true,
		Verified:     true,
		Metadata:     metadata,
	}

	if !entry.Timestamp.Equal(now) {
		t.Error("Timestamp mismatch")
	}
	if entry.VMName != "test-vm" {
		t.Error("VMName mismatch")
	}
	if entry.Provider != "vsphere" {
		t.Error("Provider mismatch")
	}
	if entry.TotalSize != 1024 {
		t.Error("TotalSize mismatch")
	}
	if !entry.Success {
		t.Error("Success should be true")
	}
	if !entry.Compressed {
		t.Error("Compressed should be true")
	}
	if !entry.Verified {
		t.Error("Verified should be true")
	}
	if entry.Metadata["key"] != "value" {
		t.Error("Metadata mismatch")
	}
}

func TestNewExportReport(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))
	report := NewExportReport(history)

	if report == nil {
		t.Fatal("NewExportReport returned nil")
	}
	if report.history != history {
		t.Error("Report should reference history")
	}
}

func TestExportReport_GenerateReport(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record some entries
	entries := []ExportHistoryEntry{
		{Timestamp: time.Now(), VMName: "vm-1", Provider: "vsphere", Format: "ova", TotalSize: 1000, Duration: 1 * time.Minute, Success: true},
		{Timestamp: time.Now(), VMName: "vm-2", Provider: "vsphere", Format: "vmdk", TotalSize: 2000, Duration: 2 * time.Minute, Success: true},
		{Timestamp: time.Now(), VMName: "vm-3", Provider: "aws", Format: "ova", TotalSize: 1500, Duration: 3 * time.Minute, Success: false, ErrorMessage: "test error"},
	}

	for _, entry := range entries {
		if err := history.RecordExport(entry); err != nil {
			t.Fatalf("RecordExport failed: %v", err)
		}
	}

	report := NewExportReport(history)

	// Generate report without history
	reportText, err := report.GenerateReport(false, 10)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if reportText == "" {
		t.Error("Report should not be empty")
	}
	if !strings.Contains(reportText, "HyperExport Report") {
		t.Error("Report should contain title")
	}
	if !strings.Contains(reportText, "Total Exports: 3") {
		t.Error("Report should contain total exports")
	}
	if !strings.Contains(reportText, "Successful: 2") {
		t.Error("Report should contain successful count")
	}
}

func TestExportReport_GenerateReport_WithHistory(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record entries
	entry := ExportHistoryEntry{
		Timestamp: time.Now(),
		VMName:    "test-vm",
		Provider:  "vsphere",
		Format:    "ova",
		TotalSize: 1024,
		Duration:  5 * time.Minute,
		Success:   true,
	}
	history.RecordExport(entry)

	report := NewExportReport(history)

	// Generate report with history
	reportText, err := report.GenerateReport(true, 10)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if !strings.Contains(reportText, "Recent Exports") {
		t.Error("Report should contain recent exports section")
	}
	if !strings.Contains(reportText, "test-vm") {
		t.Error("Report should contain VM name")
	}
}

func TestExportReport_SaveReportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record entry
	entry := ExportHistoryEntry{
		Timestamp: time.Now(),
		VMName:    "test-vm",
		Provider:  "vsphere",
		Format:    "ova",
		Success:   true,
	}
	history.RecordExport(entry)

	report := NewExportReport(history)
	reportFile := filepath.Join(tmpDir, "report.txt")

	err := report.SaveReportToFile(reportFile, false, 10)
	if err != nil {
		t.Fatalf("SaveReportToFile failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(reportFile); os.IsNotExist(err) {
		t.Error("Report file should have been created")
	}

	// Read and verify content
	content, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("Failed to read report file: %v", err)
	}

	if !strings.Contains(string(content), "HyperExport Report") {
		t.Error("Report file should contain report content")
	}
}

func TestExportStatistics_Fields(t *testing.T) {
	stats := ExportStatistics{
		TotalExports:      10,
		SuccessfulCount:   8,
		FailedCount:       2,
		SuccessRate:       80.0,
		TotalDataExported: 1024 * 1024,
		AverageDuration:   5 * time.Minute,
		FormatCounts:      map[string]int{"ova": 5, "vmdk": 5},
		ProviderCounts:    map[string]int{"vsphere": 10},
	}

	if stats.TotalExports != 10 {
		t.Error("TotalExports mismatch")
	}
	if stats.SuccessfulCount != 8 {
		t.Error("SuccessfulCount mismatch")
	}
	if stats.FailedCount != 2 {
		t.Error("FailedCount mismatch")
	}
	if stats.SuccessRate != 80.0 {
		t.Error("SuccessRate mismatch")
	}
	if stats.TotalDataExported != 1024*1024 {
		t.Error("TotalDataExported mismatch")
	}
	if stats.AverageDuration != 5*time.Minute {
		t.Error("AverageDuration mismatch")
	}
	if len(stats.FormatCounts) != 2 {
		t.Error("FormatCounts length mismatch")
	}
	if len(stats.ProviderCounts) != 1 {
		t.Error("ProviderCounts length mismatch")
	}
}

func TestExportHistory_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")
	history := NewExportHistory(historyFile, logger.NewTestLogger(t))

	// Record entry
	originalEntry := ExportHistoryEntry{
		Timestamp:    time.Now().Round(time.Second), // Round to avoid precision issues
		VMName:       "test-vm",
		VMPath:       "/datacenter/vm/test",
		Provider:     "vsphere",
		Format:       "ova",
		OutputDir:    "/exports",
		TotalSize:    1024 * 1024,
		Duration:     5 * time.Minute,
		FilesCount:   3,
		Success:      true,
		ErrorMessage: "",
		Compressed:   true,
		Verified:     true,
		Metadata:     map[string]string{"key": "value"},
	}

	if err := history.RecordExport(originalEntry); err != nil {
		t.Fatalf("RecordExport failed: %v", err)
	}

	// Load back
	entries, err := history.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	loaded := entries[0]

	// Verify fields match
	if !loaded.Timestamp.Equal(originalEntry.Timestamp) {
		t.Error("Timestamp mismatch after round-trip")
	}
	if loaded.VMName != originalEntry.VMName {
		t.Error("VMName mismatch after round-trip")
	}
	if loaded.TotalSize != originalEntry.TotalSize {
		t.Error("TotalSize mismatch after round-trip")
	}
	if loaded.Success != originalEntry.Success {
		t.Error("Success mismatch after round-trip")
	}
	if loaded.Metadata["key"] != "value" {
		t.Error("Metadata mismatch after round-trip")
	}
}
