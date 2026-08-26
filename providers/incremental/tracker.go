// SPDX-License-Identifier: Apache-2.0

package incremental

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// ChangeTracker tracks changes between exports for incremental backups
type ChangeTracker struct {
	store  *MetadataStore
	logger logger.Logger
}

// NewChangeTracker creates a new change tracker
func NewChangeTracker(metadataPath string, log logger.Logger) (*ChangeTracker, error) {
	store, err := NewMetadataStore(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata store: %w", err)
	}

	return &ChangeTracker{
		store:  store,
		logger: log,
	}, nil
}

// ExportMetadata contains metadata about a previous export
type ExportMetadata struct {
	VMID          string            `json:"vm_id"`
	VMName        string            `json:"vm_name"`
	ExportTime    time.Time         `json:"export_time"`
	SnapshotID    string            `json:"snapshot_id"`
	ChangeID      string            `json:"change_id"`
	DiskChangeIDs map[string]string `json:"disk_change_ids"` // disk key -> changeId
	ExportPath    string            `json:"export_path"`
	TotalSize     int64             `json:"total_size"`
	DiskInfo      []DiskMetadata    `json:"disks"`
}

// DiskMetadata contains metadata about a single disk
type DiskMetadata struct {
	Key           string `json:"key"`
	Path          string `json:"path"`
	CapacityBytes int64  `json:"capacity_bytes"`
	ChangeID      string `json:"change_id"`
	BackingInfo   string `json:"backing_info"`
}

// ChangedBlock represents a changed disk block
type ChangedBlock struct {
	Offset int64 `json:"offset"`
	Length int64 `json:"length"`
}

// DiskChanges contains information about changed blocks on a disk
type DiskChanges struct {
	DiskKey       string         `json:"disk_key"`
	ChangeID      string         `json:"change_id"`
	PrevChangeID  string         `json:"prev_change_id"`
	ChangedBlocks []ChangedBlock `json:"changed_blocks"`
	TotalChanged  int64          `json:"total_changed"`
}

// GetLastExport retrieves metadata about the last export for a VM
func (ct *ChangeTracker) GetLastExport(ctx context.Context, vmID string) (*ExportMetadata, error) {
	return ct.store.GetLatestExport(vmID)
}

// RecordExport records metadata about a completed export
func (ct *ChangeTracker) RecordExport(ctx context.Context, metadata *ExportMetadata) error {
	return ct.store.SaveExport(metadata)
}

// CalculateChanges calculates changed blocks between two changeIds
func (ct *ChangeTracker) CalculateChanges(ctx context.Context, diskKey, prevChangeID, currentChangeID string, capacityBytes int64) (*DiskChanges, error) {
	ct.logger.Debug("calculating disk changes",
		"disk", diskKey,
		"prev_change_id", prevChangeID,
		"current_change_id", currentChangeID)

	// This is a simplified implementation
	// In a real implementation, this would query the provider's CBT API
	// For vSphere, this would use QueryChangedDiskAreas
	// For AWS, this would use ListChangedBlocks (EBS snapshots)
	// For Azure, this would use GetPageRanges with previous snapshot

	changes := &DiskChanges{
		DiskKey:       diskKey,
		ChangeID:      currentChangeID,
		PrevChangeID:  prevChangeID,
		ChangedBlocks: []ChangedBlock{},
	}

	// Placeholder: In production, query the actual CBT data
	// For now, return empty changes list
	ct.logger.Warn("CBT query not yet implemented, returning full export hint")

	return changes, nil
}

// IsIncrementalPossible checks if incremental export is possible
func (ct *ChangeTracker) IsIncrementalPossible(ctx context.Context, vmID string, currentDisks []DiskMetadata) (bool, string) {
	// Get last export metadata
	lastExport, err := ct.GetLastExport(ctx, vmID)
	if err != nil || lastExport == nil {
		return false, "no previous export found"
	}

	// Check if disk configuration has changed
	if len(lastExport.DiskInfo) != len(currentDisks) {
		return false, "disk configuration changed (different number of disks)"
	}

	// Verify each disk still exists and has CBT enabled
	for i, disk := range currentDisks {
		lastDisk := lastExport.DiskInfo[i]

		if disk.Key != lastDisk.Key {
			return false, fmt.Sprintf("disk configuration changed (disk %s)", disk.Key)
		}

		if disk.ChangeID == "" {
			return false, fmt.Sprintf("CBT not enabled on disk %s", disk.Key)
		}

		if lastDisk.ChangeID == "" {
			return false, "previous export did not have CBT enabled"
		}
	}

	// Check if export is too old (CBT data might be expired)
	maxAge := 7 * 24 * time.Hour // 7 days
	if time.Since(lastExport.ExportTime) > maxAge {
		return false, "previous export is too old (CBT data may have expired)"
	}

	return true, ""
}

// EstimateChangedSize estimates the size of changed data
func (ct *ChangeTracker) EstimateChangedSize(ctx context.Context, vmID string, currentDisks []DiskMetadata) (int64, error) {
	lastExport, err := ct.GetLastExport(ctx, vmID)
	if err != nil || lastExport == nil {
		// No previous export, return full size
		var totalSize int64
		for _, disk := range currentDisks {
			totalSize += disk.CapacityBytes
		}
		return totalSize, nil
	}

	// Calculate changed blocks for each disk
	var totalChanged int64
	for _, disk := range currentDisks {
		// Find matching disk in last export
		var lastDiskChangeID string
		for _, lastDisk := range lastExport.DiskInfo {
			if lastDisk.Key == disk.Key {
				lastDiskChangeID = lastDisk.ChangeID
				break
			}
		}

		if lastDiskChangeID == "" {
			// New disk, count full size
			totalChanged += disk.CapacityBytes
			continue
		}

		// Calculate changes
		changes, err := ct.CalculateChanges(ctx, disk.Key, lastDiskChangeID, disk.ChangeID, disk.CapacityBytes)
		if err != nil {
			ct.logger.Warn("failed to calculate changes, assuming full disk",
				"disk", disk.Key,
				"error", err)
			totalChanged += disk.CapacityBytes
		} else {
			totalChanged += changes.TotalChanged
		}
	}

	return totalChanged, nil
}

// MetadataStore persists export metadata to disk
type MetadataStore struct {
	basePath string
}

// NewMetadataStore creates a new metadata store
func NewMetadataStore(basePath string) (*MetadataStore, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	return &MetadataStore{
		basePath: basePath,
	}, nil
}

// safeVMDir resolves the VM-specific metadata directory for vmID and ensures the
// result stays within basePath, guarding against a VM ID containing path traversal
// sequences (VM IDs can ultimately originate from job definitions submitted via the
// daemon API).
func safeVMDir(basePath, vmID string) (string, error) {
	if vmID == "" {
		return "", fmt.Errorf("vm id is required")
	}

	// Clean the ID as if it were rooted, which collapses any ".." traversal
	// attempts before joining it onto basePath.
	cleaned := filepath.Clean(string(filepath.Separator) + vmID)
	vmDir := filepath.Join(basePath, cleaned)

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("resolve base path: %w", err)
	}
	absVMDir, err := filepath.Abs(vmDir)
	if err != nil {
		return "", fmt.Errorf("resolve vm directory: %w", err)
	}
	if absVMDir != absBase && !strings.HasPrefix(absVMDir, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("vm id %q resolves outside metadata directory", vmID)
	}

	return vmDir, nil
}

// SaveExport saves export metadata
func (ms *MetadataStore) SaveExport(metadata *ExportMetadata) error {
	// Create VM-specific directory
	vmDir, err := safeVMDir(ms.basePath, metadata.VMID)
	if err != nil {
		return fmt.Errorf("invalid vm id: %w", err)
	}
	if err := os.MkdirAll(vmDir, 0750); err != nil {
		return fmt.Errorf("failed to create VM directory: %w", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("%s.json", metadata.ExportTime.Format("2006-01-02T15-04-05"))
	filePath := filepath.Join(vmDir, filename)

	// Write JSON
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Update "latest" symlink
	latestLink := filepath.Join(vmDir, "latest.json")
	if err := os.Remove(latestLink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old latest symlink: %w", err)
	}
	if err := os.Symlink(filename, latestLink); err != nil {
		// Non-critical error: symlinks might not be supported on all filesystems.
		// GetLatestExport falls back to scanning the directory when the symlink is missing.
		return nil
	}

	return nil
}

// GetLatestExport retrieves the most recent export metadata for a VM
func (ms *MetadataStore) GetLatestExport(vmID string) (*ExportMetadata, error) {
	vmDir, err := safeVMDir(ms.basePath, vmID)
	if err != nil {
		return nil, fmt.Errorf("invalid vm id: %w", err)
	}

	// Try latest symlink first
	latestLink := filepath.Join(vmDir, "latest.json")
	if data, err := os.ReadFile(latestLink); err == nil { // #nosec G304 -- latestLink is a fixed filename under the validated vmDir (see safeVMDir), not external input
		var metadata ExportMetadata
		if err := json.Unmarshal(data, &metadata); err == nil {
			return &metadata, nil
		}
	}

	// Fall back to finding the most recent file
	entries, err := os.ReadDir(vmDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No previous exports
		}
		return nil, fmt.Errorf("failed to read VM directory: %w", err)
	}

	// Find most recent JSON file
	var latestFile string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		if entry.Name() == "latest.json" {
			continue // Skip symlink
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = entry.Name()
		}
	}

	if latestFile == "" {
		return nil, nil // No exports found
	}

	// Read the latest file. filePath is built from a directory entry name returned
	// by os.ReadDir(vmDir) above, not from external input, and vmDir was already
	// validated by safeVMDir.
	filePath := filepath.Join(vmDir, latestFile)
	data, err := os.ReadFile(filePath) // #nosec G304 -- filePath comes from a directory listing of the validated vmDir, not external input
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata ExportMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// ListExports lists all exports for a VM
func (ms *MetadataStore) ListExports(vmID string) ([]*ExportMetadata, error) {
	vmDir, err := safeVMDir(ms.basePath, vmID)
	if err != nil {
		return nil, fmt.Errorf("invalid vm id: %w", err)
	}

	entries, err := os.ReadDir(vmDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ExportMetadata{}, nil
		}
		return nil, fmt.Errorf("failed to read VM directory: %w", err)
	}

	var exports []*ExportMetadata

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		if entry.Name() == "latest.json" {
			continue // Skip symlink
		}

		// filePath is built from a directory entry name returned by os.ReadDir(vmDir)
		// above, not from external input, and vmDir was already validated by safeVMDir.
		filePath := filepath.Join(vmDir, entry.Name())
		data, err := os.ReadFile(filePath) // #nosec G304 -- filePath comes from a directory listing of the validated vmDir, not external input
		if err != nil {
			continue
		}

		var metadata ExportMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}

		exports = append(exports, &metadata)
	}

	return exports, nil
}

// DeleteOldExports removes exports older than the specified age
func (ms *MetadataStore) DeleteOldExports(vmID string, maxAge time.Duration) error {
	exports, err := ms.ListExports(vmID)
	if err != nil {
		return err
	}

	vmDir, err := safeVMDir(ms.basePath, vmID)
	if err != nil {
		return fmt.Errorf("invalid vm id: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)

	var deleteErrs []error
	for _, export := range exports {
		if export.ExportTime.Before(cutoff) {
			filename := fmt.Sprintf("%s.json", export.ExportTime.Format("2006-01-02T15-04-05"))
			filePath := filepath.Join(vmDir, filename)
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				deleteErrs = append(deleteErrs, fmt.Errorf("remove %s: %w", filePath, err))
			}
		}
	}

	if len(deleteErrs) > 0 {
		return fmt.Errorf("failed to delete %d old export(s): %w", len(deleteErrs), errors.Join(deleteErrs...))
	}

	return nil
}
