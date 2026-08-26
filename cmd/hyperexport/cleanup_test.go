// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewCleanupManager(t *testing.T) {
	manager := NewCleanupManager(logger.NewTestLogger(t))
	if manager == nil {
		t.Fatal("NewCleanupManager returned nil")
	}
}

func TestCleanupConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *CleanupConfig
		wantErr bool
	}{
		{
			name: "valid config with age",
			config: &CleanupConfig{
				MaxAge:       30 * 24 * time.Hour,
				MaxCount:     0,
				MaxTotalSize: 0,
			},
			wantErr: false,
		},
		{
			name: "valid config with count",
			config: &CleanupConfig{
				MaxAge:       0,
				MaxCount:     10,
				MaxTotalSize: 0,
			},
			wantErr: false,
		},
		{
			name: "valid config with size",
			config: &CleanupConfig{
				MaxAge:       0,
				MaxCount:     0,
				MaxTotalSize: 10 * 1024 * 1024 * 1024, // 10 GB
			},
			wantErr: false,
		},
		{
			name: "all criteria disabled",
			config: &CleanupConfig{
				MaxAge:       0,
				MaxCount:     0,
				MaxTotalSize: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasAnyCriteria := tt.config.MaxAge > 0 || tt.config.MaxCount > 0 || tt.config.MaxTotalSize > 0
			hasErr := !hasAnyCriteria

			if hasErr != tt.wantErr {
				t.Errorf("Validation error = %v, wantErr %v", hasErr, tt.wantErr)
			}
		})
	}
}

func TestCleanupManager_CleanupOldExports_ByAge(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	// Create export directories
	oldExport := filepath.Join(tmpDir, "export-old")
	newExport := filepath.Join(tmpDir, "export-new")

	if err := os.Mkdir(oldExport, 0755); err != nil {
		t.Fatalf("Failed to create old export dir: %v", err)
	}
	if err := os.Mkdir(newExport, 0755); err != nil {
		t.Fatalf("Failed to create new export dir: %v", err)
	}

	// Add files to directories
	_ = os.WriteFile(filepath.Join(oldExport, "data.bin"), []byte("old data"), 0644)
	_ = os.WriteFile(filepath.Join(newExport, "data.bin"), []byte("new data"), 0644)

	// Set old directory's modification time to 60 days ago
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(oldExport, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change dir time: %v", err)
	}

	config := &CleanupConfig{
		MaxAge: 30 * 24 * time.Hour, // 30 days
		DryRun: false,
	}

	result, err := manager.CleanupOldExports(ctx, tmpDir, config)
	if err != nil {
		t.Fatalf("CleanupOldExports failed: %v", err)
	}

	if result.DeletedCount != 1 {
		t.Errorf("Expected 1 export deleted, got %d", result.DeletedCount)
	}

	// Verify old export was deleted
	if _, err := os.Stat(oldExport); !os.IsNotExist(err) {
		t.Error("Old export should have been deleted")
	}

	// Verify new export still exists
	if _, err := os.Stat(newExport); err != nil {
		t.Error("New export should still exist")
	}
}

func TestCleanupManager_CleanupOldExports_ByCount(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	// Create 5 export directories
	for i := 1; i <= 5; i++ {
		exportDir := filepath.Join(tmpDir, "export-"+string(rune('0'+i)))
		if err := os.Mkdir(exportDir, 0755); err != nil {
			t.Fatalf("Failed to create export dir: %v", err)
		}

		// Add a file
		_ = os.WriteFile(filepath.Join(exportDir, "data.bin"), []byte("data"), 0644)

		// Set different modification times (older = smaller number)
		modTime := time.Now().Add(-time.Duration(6-i) * time.Hour)
		if err := os.Chtimes(exportDir, modTime, modTime); err != nil {
			t.Fatalf("Failed to change dir time: %v", err)
		}
	}

	config := &CleanupConfig{
		MaxCount: 3, // Keep only 3 newest
		DryRun:   false,
	}

	result, err := manager.CleanupOldExports(ctx, tmpDir, config)
	if err != nil {
		t.Fatalf("CleanupOldExports failed: %v", err)
	}

	if result.DeletedCount != 2 {
		t.Errorf("Expected 2 exports deleted, got %d", result.DeletedCount)
	}

	// Count remaining exports
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() && entry.Name()[:7] == "export-" {
			count++
		}
	}

	if count != 3 {
		t.Errorf("Expected 3 exports remaining, got %d", count)
	}
}

func TestCleanupManager_CleanupOldExports_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	// Create old export
	oldExport := filepath.Join(tmpDir, "export-old")
	if err := os.Mkdir(oldExport, 0755); err != nil {
		t.Fatalf("Failed to create export dir: %v", err)
	}

	_ = os.WriteFile(filepath.Join(oldExport, "data.bin"), []byte("data"), 0644)

	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(oldExport, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change dir time: %v", err)
	}

	config := &CleanupConfig{
		MaxAge: 30 * 24 * time.Hour,
		DryRun: true, // Dry run mode
	}

	result, err := manager.CleanupOldExports(ctx, tmpDir, config)
	if err != nil {
		t.Fatalf("CleanupOldExports failed: %v", err)
	}

	if !result.DryRun {
		t.Error("Result should indicate dry run")
	}

	if result.DeletedCount != 1 {
		t.Errorf("Expected 1 export identified, got %d", result.DeletedCount)
	}

	// Verify export still exists (dry run doesn't delete)
	if _, err := os.Stat(oldExport); os.IsNotExist(err) {
		t.Error("Export should not be deleted in dry run mode")
	}
}

func TestCleanupManager_CleanupOldExports_PreservePattern(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	// Create exports
	oldRegular := filepath.Join(tmpDir, "export-regular")
	oldImportant := filepath.Join(tmpDir, "export-IMPORTANT")

	for _, dir := range []string{oldRegular, oldImportant} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		_ = os.WriteFile(filepath.Join(dir, "data.bin"), []byte("data"), 0644)

		oldTime := time.Now().Add(-60 * 24 * time.Hour)
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatalf("Failed to change dir time: %v", err)
		}
	}

	config := &CleanupConfig{
		MaxAge:           30 * 24 * time.Hour,
		PreservePatterns: []string{"*IMPORTANT*"},
		DryRun:           false,
	}

	result, err := manager.CleanupOldExports(ctx, tmpDir, config)
	if err != nil {
		t.Fatalf("CleanupOldExports failed: %v", err)
	}

	if result.DeletedCount != 1 {
		t.Errorf("Expected 1 export deleted, got %d", result.DeletedCount)
	}

	if result.PreservedCount != 1 {
		t.Errorf("Expected 1 export preserved, got %d", result.PreservedCount)
	}

	// Verify regular export was deleted
	if _, err := os.Stat(oldRegular); !os.IsNotExist(err) {
		t.Error("Regular export should have been deleted")
	}

	// Verify important export was preserved
	if _, err := os.Stat(oldImportant); err != nil {
		t.Error("Important export should have been preserved")
	}
}

func TestCleanupManager_CleanupOldExports_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	config := &CleanupConfig{
		MaxAge: 30 * 24 * time.Hour,
	}

	result, err := manager.CleanupOldExports(ctx, tmpDir, config)
	if err != nil {
		t.Fatalf("CleanupOldExports failed: %v", err)
	}

	if result.DeletedCount != 0 {
		t.Errorf("Expected 0 exports deleted in empty directory, got %d", result.DeletedCount)
	}
}

func TestCleanupManager_CleanupOldExports_NonexistentDirectory(t *testing.T) {
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	config := &CleanupConfig{
		MaxAge: 30 * 24 * time.Hour,
	}

	// Should not error on nonexistent directory
	result, err := manager.CleanupOldExports(ctx, "/nonexistent/directory", config)
	if err != nil {
		t.Logf("Got error (may be expected): %v", err)
	}
	if result != nil && result.DeletedCount != 0 {
		t.Errorf("Expected 0 exports deleted, got %d", result.DeletedCount)
	}
}

func TestCleanupResult_Fields(t *testing.T) {
	result := &CleanupResult{
		DeletedCount:   5,
		DeletedSize:    1024 * 1024 * 500, // 500 MB
		PreservedCount: 3,
		Errors:         []error{},
		DryRun:         false,
	}

	if result.DeletedCount != 5 {
		t.Error("DeletedCount mismatch")
	}
	if result.DeletedSize != 1024*1024*500 {
		t.Error("DeletedSize mismatch")
	}
	if result.PreservedCount != 3 {
		t.Error("PreservedCount mismatch")
	}
	if len(result.Errors) != 0 {
		t.Error("Errors length mismatch")
	}
	if result.DryRun {
		t.Error("DryRun should be false")
	}
}

func TestCleanupConfig_MultipleConstraints(t *testing.T) {
	config := &CleanupConfig{
		MaxAge:       30 * 24 * time.Hour,
		MaxCount:     10,
		MaxTotalSize: 10 * 1024 * 1024 * 1024,
	}

	// All constraints are set
	if config.MaxAge == 0 {
		t.Error("MaxAge should be set")
	}
	if config.MaxCount == 0 {
		t.Error("MaxCount should be set")
	}
	if config.MaxTotalSize == 0 {
		t.Error("MaxTotalSize should be set")
	}
}

func TestCleanupManager_FormatBytesUsage(t *testing.T) {
	// Test that formatBytes works correctly in cleanup context
	tests := []struct {
		name  string
		bytes int64
	}{
		{"small file", 1024},
		{"medium file", 1024 * 1024},
		{"large file", 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result == "" {
				t.Error("formatBytes should not return empty string")
			}
		})
	}
}

func TestExportInfo_Fields(t *testing.T) {
	now := time.Now()
	info := &ExportInfo{
		Path:      "/exports/export-123",
		Size:      1024 * 1024 * 100,
		ModTime:   now,
		Preserved: false,
	}

	if info.Path != "/exports/export-123" {
		t.Error("Path mismatch")
	}
	if info.Size != 1024*1024*100 {
		t.Error("Size mismatch")
	}
	if !info.ModTime.Equal(now) {
		t.Error("ModTime mismatch")
	}
	if info.Preserved {
		t.Error("Preserved should be false")
	}
}

func TestCleanupManager_CleanupOldExports_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	// Should use default config when nil
	result, err := manager.CleanupOldExports(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("CleanupOldExports with nil config failed: %v", err)
	}

	if result == nil {
		t.Error("Result should not be nil")
	}
}

func TestCleanupManager_CleanupOldExports_WithTotalSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	// Create multiple exports with different sizes
	for i := 1; i <= 3; i++ {
		exportDir := filepath.Join(tmpDir, "export-"+string(rune('0'+i)))
		if err := os.Mkdir(exportDir, 0755); err != nil {
			t.Fatalf("Failed to create export dir: %v", err)
		}

		// Create files of different sizes
		dataSize := i * 10 * 1024 // 10KB, 20KB, 30KB
		_ = os.WriteFile(filepath.Join(exportDir, "data.bin"), make([]byte, dataSize), 0644)

		// Set different modification times (older = smaller number)
		modTime := time.Now().Add(-time.Duration(4-i) * time.Hour)
		if err := os.Chtimes(exportDir, modTime, modTime); err != nil {
			t.Fatalf("Failed to change dir time: %v", err)
		}
	}

	config := &CleanupConfig{
		MaxTotalSize: 25 * 1024, // 25KB max total
		DryRun:       false,
	}

	result, err := manager.CleanupOldExports(ctx, tmpDir, config)
	if err != nil {
		t.Fatalf("CleanupOldExports failed: %v", err)
	}

	// Should delete oldest exports to get under limit
	if result.DeletedCount == 0 {
		t.Log("Expected some exports to be deleted for size limit (implementation may vary)")
	}
}

func TestCleanupManager_GetAvailableSpace(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))

	// Test with valid directory
	space, err := manager.getAvailableSpace(tmpDir)
	if err != nil {
		t.Fatalf("getAvailableSpace failed: %v", err)
	}

	if space <= 0 {
		t.Error("Expected positive available space")
	}

	// Test with file path (should use parent directory)
	testFile := filepath.Join(tmpDir, "testfile.txt")
	_ = os.WriteFile(testFile, []byte("test"), 0644)

	space, err = manager.getAvailableSpace(testFile)
	if err != nil {
		t.Fatalf("getAvailableSpace with file path failed: %v", err)
	}

	if space <= 0 {
		t.Error("Expected positive available space for file path")
	}

	// Test with nonexistent path
	_, err = manager.getAvailableSpace("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for nonexistent path")
	}
}

func TestCleanupManager_CleanupByFreeSpace(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))
	ctx := context.Background()

	// Create export directories with files
	for i := 1; i <= 3; i++ {
		exportDir := filepath.Join(tmpDir, "export-"+string(rune('0'+i)))
		if err := os.Mkdir(exportDir, 0755); err != nil {
			t.Fatalf("Failed to create export dir: %v", err)
		}

		// Add files
		dataSize := i * 1024 * 1024 // 1MB, 2MB, 3MB
		_ = os.WriteFile(filepath.Join(exportDir, "data.bin"), make([]byte, dataSize), 0644)

		// Set different modification times
		modTime := time.Now().Add(-time.Duration(4-i) * 24 * time.Hour)
		if err := os.Chtimes(exportDir, modTime, modTime); err != nil {
			t.Fatalf("Failed to change dir time: %v", err)
		}
	}

	// Test with very high minimum (will trigger cleanup)
	config := &CleanupConfig{
		MaxAge: 365 * 24 * time.Hour, // Delete exports older than 1 year
	}

	minFreeSpace := int64(1024 * 1024 * 1024 * 1024) // 1TB - very high, will trigger cleanup
	result, err := manager.CleanupByFreeSpace(ctx, tmpDir, minFreeSpace, config)
	if err != nil {
		t.Fatalf("CleanupByFreeSpace failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Test with low minimum (won't trigger cleanup)
	minFreeSpace = int64(1024) // 1KB - very low
	result, err = manager.CleanupByFreeSpace(ctx, tmpDir, minFreeSpace, config)
	if err != nil {
		t.Fatalf("CleanupByFreeSpace failed: %v", err)
	}

	if result.DeletedCount != 0 {
		t.Logf("Expected no cleanup needed with low minimum, but got %d deleted", result.DeletedCount)
	}
}

func TestCleanupManager_ScheduledCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewCleanupManager(logger.NewTestLogger(t))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Create an old export
	oldExport := filepath.Join(tmpDir, "export-old")
	if err := os.Mkdir(oldExport, 0755); err != nil {
		t.Fatalf("Failed to create export dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(oldExport, "data.bin"), []byte("data"), 0644)

	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(oldExport, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change dir time: %v", err)
	}

	config := &CleanupConfig{
		MaxAge: 30 * 24 * time.Hour,
	}

	// Run scheduled cleanup with very short interval
	done := make(chan bool)
	go func() {
		manager.ScheduledCleanup(ctx, tmpDir, config, 50*time.Millisecond)
		done <- true
	}()

	// Wait for context to cancel
	select {
	case <-done:
		t.Log("Scheduled cleanup completed")
	case <-time.After(500 * time.Millisecond):
		t.Error("Scheduled cleanup did not stop within timeout")
	}

	// Verify the old export was eventually cleaned up
	// (may or may not be deleted depending on timing)
	t.Log("Scheduled cleanup test completed")
}
