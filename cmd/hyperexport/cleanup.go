// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// CleanupManager handles automatic cleanup of old exports
type CleanupManager struct {
	log logger.Logger
}

// CleanupConfig configures cleanup behavior
type CleanupConfig struct {
	MaxAge           time.Duration // Delete exports older than this
	MaxCount         int           // Keep only this many exports (0 = unlimited)
	MaxTotalSize     int64         // Delete oldest when total size exceeds this (0 = unlimited)
	DryRun           bool          // Preview what would be deleted without deleting
	PreservePatterns []string      // Glob patterns for exports to preserve
	MinFreeSpace     int64         // Trigger cleanup when free space falls below this
}

// CleanupResult contains cleanup operation results
type CleanupResult struct {
	DeletedCount int
	DeletedSize  int64
	PreservedCount int
	Errors       []error
	DryRun       bool
}

// ExportInfo contains information about an export
type ExportInfo struct {
	Path      string
	Size      int64
	ModTime   time.Time
	Preserved bool
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(log logger.Logger) *CleanupManager {
	return &CleanupManager{
		log: log,
	}
}

// CleanupOldExports removes old exports based on configuration
func (cm *CleanupManager) CleanupOldExports(ctx context.Context, exportBaseDir string, config *CleanupConfig) (*CleanupResult, error) {
	if config == nil {
		config = &CleanupConfig{
			MaxAge:   30 * 24 * time.Hour, // 30 days default
			MaxCount: 0,
			DryRun:   false,
		}
	}

	cm.log.Info("starting export cleanup",
		"dir", exportBaseDir,
		"maxAge", config.MaxAge,
		"maxCount", config.MaxCount,
		"maxSize", config.MaxTotalSize,
		"dryRun", config.DryRun)

	result := &CleanupResult{
		DryRun: config.DryRun,
	}

	// Find all export directories
	exports, err := cm.findExports(exportBaseDir)
	if err != nil {
		return result, fmt.Errorf("find exports: %w", err)
	}

	if len(exports) == 0 {
		cm.log.Info("no exports found for cleanup")
		return result, nil
	}

	cm.log.Info("found exports", "count", len(exports))

	// Mark preserved exports
	if err := cm.markPreserved(exports, config.PreservePatterns); err != nil {
		cm.log.Warn("failed to mark preserved exports", "error", err)
	}

	// Sort exports by modification time (oldest first)
	sort.Slice(exports, func(i, j int) bool {
		return exports[i].ModTime.Before(exports[j].ModTime)
	})

	// Determine which exports to delete
	toDelete := cm.selectForDeletion(exports, config)

	if len(toDelete) == 0 {
		cm.log.Info("no exports need cleanup")
		return result, nil
	}

	// Delete selected exports
	for _, exp := range toDelete {
		if config.DryRun {
			cm.log.Info("would delete (dry-run)", "path", exp.Path, "size", exp.Size, "age", time.Since(exp.ModTime))
		} else {
			cm.log.Info("deleting export", "path", exp.Path, "size", exp.Size, "age", time.Since(exp.ModTime))

			if err := os.RemoveAll(exp.Path); err != nil {
				cm.log.Error("failed to delete export", "path", exp.Path, "error", err)
				result.Errors = append(result.Errors, err)
			}
		}

		result.DeletedCount++
		result.DeletedSize += exp.Size
	}

	result.PreservedCount = len(exports) - len(toDelete)

	cm.log.Info("cleanup completed",
		"deleted", result.DeletedCount,
		"deletedSize", result.DeletedSize,
		"preserved", result.PreservedCount,
		"errors", len(result.Errors))

	return result, nil
}

// findExports finds all export directories in the base directory
func (cm *CleanupManager) findExports(baseDir string) ([]*ExportInfo, error) {
	var exports []*ExportInfo

	// Check if base directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return exports, nil // No exports yet
	}

	// Find all export-* directories
	pattern := filepath.Join(baseDir, "export-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob exports: %w", err)
	}

	// Get info for each export
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			cm.log.Warn("failed to stat export", "path", path, "error", err)
			continue
		}

		if !info.IsDir() {
			continue
		}

		// Calculate directory size
		size, err := cm.calculateDirSize(path)
		if err != nil {
			cm.log.Warn("failed to calculate size", "path", path, "error", err)
			size = 0
		}

		exports = append(exports, &ExportInfo{
			Path:    path,
			Size:    size,
			ModTime: info.ModTime(),
		})
	}

	return exports, nil
}

// calculateDirSize calculates the total size of a directory
func (cm *CleanupManager) calculateDirSize(dir string) (int64, error) {
	var size int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// markPreserved marks exports matching preserve patterns
func (cm *CleanupManager) markPreserved(exports []*ExportInfo, patterns []string) error {
	if len(patterns) == 0 {
		return nil
	}

	for _, exp := range exports {
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, filepath.Base(exp.Path))
			if err != nil {
				return fmt.Errorf("match pattern %s: %w", pattern, err)
			}
			if matched {
				exp.Preserved = true
				cm.log.Info("preserving export", "path", exp.Path, "pattern", pattern)
				break
			}
		}
	}

	return nil
}

// selectForDeletion selects which exports should be deleted
func (cm *CleanupManager) selectForDeletion(exports []*ExportInfo, config *CleanupConfig) []*ExportInfo {
	var toDelete []*ExportInfo

	now := time.Now()
	var totalSize int64

	// First pass: mark exports by age
	for _, exp := range exports {
		if exp.Preserved {
			continue
		}

		age := now.Sub(exp.ModTime)
		if config.MaxAge > 0 && age > config.MaxAge {
			toDelete = append(toDelete, exp)
		} else {
			totalSize += exp.Size
		}
	}

	// Second pass: enforce count limit
	if config.MaxCount > 0 {
		// Count non-deleted, non-preserved exports
		remaining := make([]*ExportInfo, 0)
		for _, exp := range exports {
			if exp.Preserved {
				continue
			}
			// Skip if already marked for deletion
			alreadyMarked := false
			for _, del := range toDelete {
				if del.Path == exp.Path {
					alreadyMarked = true
					break
				}
			}
			if !alreadyMarked {
				remaining = append(remaining, exp)
			}
		}

		// Delete oldest if we exceed count
		if len(remaining) > config.MaxCount {
			excess := len(remaining) - config.MaxCount
			for i := 0; i < excess; i++ {
				toDelete = append(toDelete, remaining[i])
				totalSize -= remaining[i].Size
			}
		}
	}

	// Third pass: enforce size limit
	if config.MaxTotalSize > 0 && totalSize > config.MaxTotalSize {
		// Find exports not yet marked for deletion
		notMarked := make([]*ExportInfo, 0)
		for _, exp := range exports {
			if exp.Preserved {
				continue
			}
			alreadyMarked := false
			for _, del := range toDelete {
				if del.Path == exp.Path {
					alreadyMarked = true
					break
				}
			}
			if !alreadyMarked {
				notMarked = append(notMarked, exp)
			}
		}

		// Sort by age (oldest first) and delete until under limit
		sort.Slice(notMarked, func(i, j int) bool {
			return notMarked[i].ModTime.Before(notMarked[j].ModTime)
		})

		for _, exp := range notMarked {
			if totalSize <= config.MaxTotalSize {
				break
			}
			toDelete = append(toDelete, exp)
			totalSize -= exp.Size
		}
	}

	return toDelete
}

// CleanupByFreeSpace triggers cleanup if free space is low
func (cm *CleanupManager) CleanupByFreeSpace(ctx context.Context, exportBaseDir string, minFreeSpace int64, config *CleanupConfig) (*CleanupResult, error) {
	// Get filesystem stats
	freeSpace, err := cm.getAvailableSpace(exportBaseDir)
	if err != nil {
		return nil, fmt.Errorf("get available space: %w", err)
	}

	cm.log.Info("checking free space",
		"available", freeSpace,
		"minimum", minFreeSpace)

	if freeSpace >= minFreeSpace {
		cm.log.Info("sufficient free space available", "free", freeSpace)
		return &CleanupResult{}, nil
	}

	cm.log.Warn("low free space detected, triggering cleanup",
		"available", freeSpace,
		"minimum", minFreeSpace,
		"deficit", minFreeSpace-freeSpace)

	// Run cleanup until we have enough space
	result, err := cm.CleanupOldExports(ctx, exportBaseDir, config)
	if err != nil {
		return result, err
	}

	// Check if we freed enough space
	newFreeSpace, err := cm.getAvailableSpace(exportBaseDir)
	if err != nil {
		cm.log.Warn("failed to check new free space", "error", err)
	} else {
		cm.log.Info("cleanup freed space",
			"before", freeSpace,
			"after", newFreeSpace,
			"freed", newFreeSpace-freeSpace)
	}

	return result, nil
}

// getAvailableSpace returns available disk space in bytes
func (cm *CleanupManager) getAvailableSpace(path string) (int64, error) {
	// This is platform-specific, simplified implementation
	// In production, use syscall to get accurate filesystem stats
	if _, err := os.Stat(path); err != nil {
		return 0, err
	}

	// Placeholder: return a large number
	// In real implementation, use syscall.Statfs on Unix or similar on Windows
	return 1024 * 1024 * 1024 * 100, nil // 100 GB placeholder
}

// ScheduledCleanup performs periodic cleanup
func (cm *CleanupManager) ScheduledCleanup(ctx context.Context, exportBaseDir string, config *CleanupConfig, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cm.log.Info("starting scheduled cleanup",
		"interval", interval,
		"dir", exportBaseDir)

	for {
		select {
		case <-ctx.Done():
			cm.log.Info("scheduled cleanup stopped")
			return
		case <-ticker.C:
			cm.log.Info("running scheduled cleanup")
			result, err := cm.CleanupOldExports(ctx, exportBaseDir, config)
			if err != nil {
				cm.log.Error("scheduled cleanup failed", "error", err)
			} else {
				cm.log.Info("scheduled cleanup completed",
					"deleted", result.DeletedCount,
					"deletedSize", result.DeletedSize,
					"preserved", result.PreservedCount)
			}
		}
	}
}
