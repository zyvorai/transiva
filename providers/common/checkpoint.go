// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Checkpoint represents the state of an in-progress export
type Checkpoint struct {
	Version      string            `json:"version"`       // Checkpoint format version
	VMName       string            `json:"vm_name"`       // VM being exported
	Provider     string            `json:"provider"`      // Provider type (vsphere, aws, etc.)
	ExportFormat string            `json:"export_format"` // Export format (ovf, ova, vmdk, vhd)
	OutputPath   string            `json:"output_path"`   // Destination directory
	CreatedAt    time.Time         `json:"created_at"`    // When export started
	UpdatedAt    time.Time         `json:"updated_at"`    // Last checkpoint update
	Files        []FileCheckpoint  `json:"files"`         // Per-file progress
	Metadata     map[string]string `json:"metadata"`      // Additional metadata
}

// FileCheckpoint tracks progress for a single file
type FileCheckpoint struct {
	Path           string    `json:"path"`             // Relative path in export
	URL            string    `json:"url,omitempty"`    // Source URL (if applicable)
	TotalSize      int64     `json:"total_size"`       // Total file size in bytes
	DownloadedSize int64     `json:"downloaded_size"`  // Bytes downloaded so far
	Checksum       string    `json:"checksum"`         // SHA-256 checksum (partial or complete)
	Status         string    `json:"status"`           // pending, downloading, completed, failed
	LastModified   time.Time `json:"last_modified"`    // Last update time
	RetryCount     int       `json:"retry_count"`      // Number of retry attempts
}

const CheckpointVersion = "1.0"

// NewCheckpoint creates a new checkpoint for an export
func NewCheckpoint(vmName, provider, exportFormat, outputPath string) *Checkpoint {
	now := time.Now()
	return &Checkpoint{
		Version:      CheckpointVersion,
		VMName:       vmName,
		Provider:     provider,
		ExportFormat: exportFormat,
		OutputPath:   outputPath,
		CreatedAt:    now,
		UpdatedAt:    now,
		Files:        make([]FileCheckpoint, 0),
		Metadata:     make(map[string]string),
	}
}

// AddFile adds a file to the checkpoint
func (c *Checkpoint) AddFile(path, url string, totalSize int64) {
	c.Files = append(c.Files, FileCheckpoint{
		Path:           path,
		URL:            url,
		TotalSize:      totalSize,
		DownloadedSize: 0,
		Status:         "pending",
		LastModified:   time.Now(),
		RetryCount:     0,
	})
	c.UpdatedAt = time.Now()
}

// UpdateFileProgress updates download progress for a file
func (c *Checkpoint) UpdateFileProgress(path string, downloadedSize int64, status string) {
	for i := range c.Files {
		if c.Files[i].Path == path {
			c.Files[i].DownloadedSize = downloadedSize
			c.Files[i].Status = status
			c.Files[i].LastModified = time.Now()
			c.UpdatedAt = time.Now()
			return
		}
	}
}

// GetFileProgress returns the checkpoint for a specific file
func (c *Checkpoint) GetFileProgress(path string) *FileCheckpoint {
	for i := range c.Files {
		if c.Files[i].Path == path {
			return &c.Files[i]
		}
	}
	return nil
}

// IsComplete returns true if all files are completed
func (c *Checkpoint) IsComplete() bool {
	if len(c.Files) == 0 {
		return false
	}

	for _, file := range c.Files {
		if file.Status != "completed" {
			return false
		}
	}
	return true
}

// GetProgress returns overall progress (0.0 to 1.0)
func (c *Checkpoint) GetProgress() float64 {
	if len(c.Files) == 0 {
		return 0.0
	}

	var totalSize, downloadedSize int64
	for _, file := range c.Files {
		totalSize += file.TotalSize
		downloadedSize += file.DownloadedSize
	}

	if totalSize == 0 {
		return 0.0
	}

	return float64(downloadedSize) / float64(totalSize)
}

// Save writes the checkpoint to disk
func (c *Checkpoint) Save(checkpointPath string) error {
	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(checkpointPath), 0750); err != nil {
		return fmt.Errorf("create checkpoint directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	// Write to temp file first
	tempPath := checkpointPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, checkpointPath); err != nil {
		return fmt.Errorf("rename checkpoint: %w", err)
	}

	return nil
}

// LoadCheckpoint loads a checkpoint from disk
func LoadCheckpoint(checkpointPath string) (*Checkpoint, error) {
	// #nosec G304 -- checkpointPath is derived from the local operator-supplied export output directory (CLI flag/config), not remote input
	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}

	// Validate version
	if checkpoint.Version != CheckpointVersion {
		return nil, fmt.Errorf("incompatible checkpoint version: %s (expected %s)",
			checkpoint.Version, CheckpointVersion)
	}

	return &checkpoint, nil
}

// DeleteCheckpoint removes a checkpoint file
func DeleteCheckpoint(checkpointPath string) error {
	if err := os.Remove(checkpointPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}

// ComputeChecksum computes SHA-256 checksum of a file
func ComputeChecksum(filePath string) (string, error) {
	// #nosec G304 -- filePath is derived from the local operator-supplied export output directory (CLI flag/config), not remote input
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// GetCheckpointPath returns the standard checkpoint path for a VM export
func GetCheckpointPath(outputDir, vmName string) string {
	return filepath.Join(outputDir, fmt.Sprintf(".%s.checkpoint", vmName))
}

// CheckpointExists checks if a checkpoint file exists
func CheckpointExists(checkpointPath string) bool {
	_, err := os.Stat(checkpointPath)
	return err == nil
}
