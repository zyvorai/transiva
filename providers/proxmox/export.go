// SPDX-License-Identifier: Apache-2.0

package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportOptions holds VM export configuration
type ExportOptions struct {
	Node           string
	VMID           int
	OutputPath     string
	BackupMode     string // snapshot, suspend, stop
	Compress       string // zstd, gzip, lzo, or empty for no compression
	RemoveExisting bool   // Remove existing backup before creating new one
	Notes          string // Optional backup notes
}

// ExportResult holds export operation results
type ExportResult struct {
	BackupFile string
	BackupID   string
	Size       int64
	Duration   time.Duration
	Format     string
}

// CreateBackup creates a VM backup using vzdump
func (c *Client) CreateBackup(ctx context.Context, opts ExportOptions) (*ExportResult, error) {
	startTime := time.Now()

	c.logger.Info("creating VM backup",
		"node", opts.Node,
		"vmid", opts.VMID,
		"mode", opts.BackupMode,
		"compress", opts.Compress)

	// Prepare backup parameters
	params := url.Values{}
	params.Set("vmid", fmt.Sprintf("%d", opts.VMID))

	// Set backup mode (default: snapshot)
	backupMode := opts.BackupMode
	if backupMode == "" {
		backupMode = "snapshot"
	}
	params.Set("mode", backupMode)

	// Set compression
	if opts.Compress != "" {
		params.Set("compress", opts.Compress)
	} else {
		params.Set("compress", "zstd") // Default to zstd compression
	}

	// Set storage target (local by default)
	params.Set("storage", "local")

	// Set notes if provided
	if opts.Notes != "" {
		params.Set("notes", opts.Notes)
	}

	// Remove existing backup if requested
	if opts.RemoveExisting {
		params.Set("remove", "1")
	}

	// Create backup task
	path := fmt.Sprintf("/nodes/%s/vzdump", opts.Node)
	resp, err := c.apiRequest(ctx, "POST", path, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create backup task: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backup creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse task ID from response
	var taskResp struct {
		Data string `json:"data"` // UPID (task ID)
	}

	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		return nil, fmt.Errorf("decode task response: %w", err)
	}

	upid := taskResp.Data
	c.logger.Info("backup task created", "upid", upid)

	// Wait for backup to complete
	if err := c.WaitForTask(ctx, opts.Node, upid); err != nil {
		return nil, fmt.Errorf("wait for backup: %w", err)
	}

	duration := time.Since(startTime)

	// Find the created backup file
	backupFile, err := c.findLatestBackup(ctx, opts.Node, opts.VMID)
	if err != nil {
		return nil, fmt.Errorf("find backup file: %w", err)
	}

	c.logger.Info("backup created successfully",
		"file", backupFile,
		"duration", duration)

	result := &ExportResult{
		BackupFile: backupFile,
		BackupID:   upid,
		Duration:   duration,
		Format:     "vzdump",
	}

	return result, nil
}

// findLatestBackup finds the most recent backup for a VM
func (c *Client) findLatestBackup(ctx context.Context, node string, vmid int) (string, error) {
	// List backups in local storage
	path := fmt.Sprintf("/nodes/%s/storage/local/content", node)

	resp, err := c.apiRequest(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("list storage content failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []StorageContent `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// Find backups for this VM, sorted by creation time
	var latestBackup *StorageContent
	vmidStr := fmt.Sprintf("-%d-", vmid)

	for i := range apiResp.Data {
		content := &apiResp.Data[i]
		if content.Content == "backup" && strings.Contains(content.VolID, vmidStr) {
			if latestBackup == nil || content.CreationTime > latestBackup.CreationTime {
				latestBackup = content
			}
		}
	}

	if latestBackup == nil {
		return "", fmt.Errorf("no backup found for VMID %d", vmid)
	}

	return latestBackup.VolID, nil
}

// StorageContent represents storage content item
type StorageContent struct {
	VolID        string `json:"volid"`
	Content      string `json:"content"`
	Format       string `json:"format"`
	Size         int64  `json:"size"`
	CreationTime int64  `json:"ctime"`
	Notes        string `json:"notes"`
}

// DownloadBackup downloads a backup file to local path
func (c *Client) DownloadBackup(ctx context.Context, node, backupVolID, outputPath string) error {
	c.logger.Info("downloading backup",
		"volid", backupVolID,
		"output", outputPath)

	// Parse volume ID (format: local:backup/vzdump-qemu-100-2024_01_21-12_00_00.vma.zst)
	parts := strings.SplitN(backupVolID, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid volume ID format: %s", backupVolID)
	}

	storage := parts[0]
	volumePath := parts[1]

	// Construct download URL
	downloadPath := fmt.Sprintf("/nodes/%s/storage/%s/download", node, storage)
	downloadURL := c.baseURL + downloadPath + "?volume=" + url.QueryEscape(volumePath)

	c.logger.Debug("download URL", "url", downloadURL)

	// Create download request
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}

	// Set authentication
	req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.ticket))

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send download request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Create output file
	// #nosec G304 -- outputPath is derived from the caller-supplied ExportOptions.OutputPath (local/CLI config) joined with a backup filename sanitized in ExportVM
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	// Copy content with progress tracking
	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("download backup: %w", err)
	}

	c.logger.Info("backup downloaded successfully",
		"file", outputPath,
		"size", written)

	return nil
}

// ExportVM performs complete export operation
func (c *Client) ExportVM(ctx context.Context, opts ExportOptions) (*ExportResult, error) {
	c.logger.Info("exporting VM",
		"node", opts.Node,
		"vmid", opts.VMID,
		"output", opts.OutputPath)

	// Create backup
	result, err := c.CreateBackup(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}

	// Determine output filename
	backupFilename := filepath.Base(result.BackupFile)
	// Remove storage prefix (e.g., "local:backup/")
	if strings.Contains(backupFilename, "/") {
		parts := strings.Split(backupFilename, "/")
		backupFilename = parts[len(parts)-1]
	}
	// Guard against path traversal: filepath.Base can still return "." or ".."
	// for degenerate inputs (e.g. result.BackupFile == "..").
	if backupFilename == "" || backupFilename == "." || backupFilename == ".." {
		return nil, fmt.Errorf("invalid backup filename derived from %q", result.BackupFile)
	}

	outputPath := filepath.Join(opts.OutputPath, backupFilename)

	// Download backup to local path
	if err := c.DownloadBackup(ctx, opts.Node, result.BackupFile, outputPath); err != nil {
		return nil, fmt.Errorf("download backup: %w", err)
	}

	// Get file size
	if fi, err := os.Stat(outputPath); err == nil {
		result.Size = fi.Size()
	}

	result.BackupFile = outputPath

	c.logger.Info("VM export completed successfully",
		"output", outputPath,
		"size", result.Size,
		"duration", result.Duration)

	return result, nil
}

// DeleteBackup deletes a backup from storage
func (c *Client) DeleteBackup(ctx context.Context, node, backupVolID string) error {
	c.logger.Info("deleting backup", "volid", backupVolID)

	// Parse volume ID
	parts := strings.SplitN(backupVolID, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid volume ID format: %s", backupVolID)
	}

	storage := parts[0]
	volumePath := parts[1]

	// Delete volume
	path := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", node, storage, url.PathEscape(volumePath))

	resp, err := c.apiRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete backup failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Info("backup deleted successfully", "volid", backupVolID)
	return nil
}

// ListBackups lists all backups for a VM
func (c *Client) ListBackups(ctx context.Context, node string, vmid int) ([]StorageContent, error) {
	path := fmt.Sprintf("/nodes/%s/storage/local/content", node)

	resp, err := c.apiRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list storage content failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []StorageContent `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Filter backups for this VM
	var backups []StorageContent
	vmidStr := fmt.Sprintf("-%d-", vmid)

	for _, content := range apiResp.Data {
		if content.Content == "backup" && strings.Contains(content.VolID, vmidStr) {
			backups = append(backups, content)
		}
	}

	c.logger.Info("listed backups", "node", node, "vmid", vmid, "count", len(backups))

	return backups, nil
}
