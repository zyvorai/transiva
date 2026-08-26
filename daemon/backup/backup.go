// SPDX-License-Identifier: Apache-2.0

// Package backup provides backup and disaster recovery functionality
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	// ErrBackupNotFound is returned when a backup does not exist
	ErrBackupNotFound = errors.New("backup not found")

	// ErrInvalidBackup is returned when a backup is corrupted
	ErrInvalidBackup = errors.New("invalid backup")

	// ErrRestoreInProgress is returned when a restore is already running
	ErrRestoreInProgress = errors.New("restore in progress")
)

// BackupType represents the type of backup
type BackupType string

const (
	// BackupTypeFull represents a full backup
	BackupTypeFull BackupType = "full"

	// BackupTypeIncremental represents an incremental backup
	BackupTypeIncremental BackupType = "incremental"
)

// BackupStatus represents the status of a backup
type BackupStatus string

const (
	// BackupStatusRunning indicates backup is in progress
	BackupStatusRunning BackupStatus = "running"

	// BackupStatusCompleted indicates backup completed successfully
	BackupStatusCompleted BackupStatus = "completed"

	// BackupStatusFailed indicates backup failed
	BackupStatusFailed BackupStatus = "failed"
)

// BackupMetadata contains metadata about a backup
type BackupMetadata struct {
	ID         string       `json:"id"`
	Type       BackupType   `json:"type"`
	Status     BackupStatus `json:"status"`
	StartTime  time.Time    `json:"start_time"`
	EndTime    time.Time    `json:"end_time"`
	Size       int64        `json:"size"`
	Checksum   string       `json:"checksum"`
	SourcePath string       `json:"source_path"`
	BackupPath string       `json:"backup_path"`
	Files      []string     `json:"files"`
	BaseBackup string       `json:"base_backup,omitempty"` // For incremental backups
	Encrypted  bool         `json:"encrypted"`
	Compressed bool         `json:"compressed"`
	Version    string       `json:"version"`
	Error      string       `json:"error,omitempty"`
}

// Config holds backup configuration
type Config struct {
	// Storage location
	BackupDir string

	// Backup settings
	EnableCompression bool
	EnableEncryption  bool
	EncryptionKey     []byte
	MaxBackups        int // Maximum number of backups to retain
	RetentionDays     int // Number of days to retain backups

	// Performance settings
	BufferSize   int
	MaxWorkers   int
	ChecksumType string // "sha256", "md5"

	// Scheduling
	EnableAutoBackup     bool
	BackupInterval       time.Duration
	AutoBackupSourcePath string // Path to backup automatically
	AutoBackupType       BackupType
}

// DefaultConfig returns default backup configuration
func DefaultConfig() *Config {
	return &Config{
		BackupDir:            "./backups",
		EnableCompression:    true,
		EnableEncryption:     false,
		MaxBackups:           10,
		RetentionDays:        30,
		BufferSize:           32 * 1024,
		MaxWorkers:           4,
		ChecksumType:         "sha256",
		EnableAutoBackup:     false,
		BackupInterval:       24 * time.Hour,
		AutoBackupSourcePath: "",
		AutoBackupType:       BackupTypeFull,
	}
}

// Logger interface for backup manager
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// noopLogger is a logger that does nothing
type noopLogger struct{}

func (n *noopLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (n *noopLogger) Info(msg string, keysAndValues ...interface{})  {}
func (n *noopLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (n *noopLogger) Error(msg string, keysAndValues ...interface{}) {}

// Manager manages backup and restore operations
type Manager struct {
	config    *Config
	backups   map[string]*BackupMetadata
	mu        sync.RWMutex
	restoring bool
	restoreMu sync.Mutex
	logger    Logger
	stopCh    chan struct{}
}

// NewManager creates a new backup manager
func NewManager(config *Config, logger Logger) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if logger == nil {
		logger = &noopLogger{}
	}

	// Create backup directory
	if err := os.MkdirAll(config.BackupDir, 0750); err != nil {
		return nil, err
	}

	manager := &Manager{
		config:  config,
		backups: make(map[string]*BackupMetadata),
		logger:  logger,
		stopCh:  make(chan struct{}),
	}

	// Load existing backups
	if err := manager.loadBackups(); err != nil {
		return nil, err
	}

	// Start auto-backup if enabled
	if config.EnableAutoBackup {
		if config.AutoBackupSourcePath == "" {
			logger.Warn("auto backup enabled but no source path configured")
		} else {
			go manager.autoBackupLoop()
		}
	}

	return manager, nil
}

// CreateBackup creates a new backup
func (m *Manager) CreateBackup(ctx context.Context, sourcePath string, backupType BackupType) (*BackupMetadata, error) {
	backupID := generateBackupID()

	metadata := &BackupMetadata{
		ID:         backupID,
		Type:       backupType,
		Status:     BackupStatusRunning,
		StartTime:  time.Now(),
		SourcePath: sourcePath,
		BackupPath: filepath.Join(m.config.BackupDir, backupID+".tar.gz"),
		Compressed: m.config.EnableCompression,
		Encrypted:  m.config.EnableEncryption,
		Version:    "1.0",
	}

	m.mu.Lock()
	m.backups[backupID] = metadata
	m.mu.Unlock()

	// Perform backup
	err := m.performBackup(ctx, metadata)
	if err != nil {
		metadata.Status = BackupStatusFailed
		metadata.Error = err.Error()
		metadata.EndTime = time.Now()
		return metadata, err
	}

	metadata.Status = BackupStatusCompleted
	metadata.EndTime = time.Now()

	// Save metadata
	if err := m.saveMetadata(metadata); err != nil {
		return metadata, err
	}

	// Apply retention policy
	go m.applyRetention()

	return metadata, nil
}

// performBackup performs the actual backup operation
func (m *Manager) performBackup(ctx context.Context, metadata *BackupMetadata) error {
	// Create backup file
	file, err := os.Create(metadata.BackupPath)
	if err != nil {
		return err
	}

	// Create writer chain
	var writer io.Writer = file
	var gzipWriter *gzip.Writer

	// Add gzip compression
	if m.config.EnableCompression {
		gzipWriter = gzip.NewWriter(writer)
		writer = gzipWriter
	}

	// Create tar writer
	tarWriter := tar.NewWriter(writer)

	// Open the source directory as a scoped root so all filesystem operations
	// during the walk stay confined to it, avoiding symlink TOCTOU traversal.
	root, err := os.OpenRoot(metadata.SourcePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			m.logger.Warn("failed to close scoped root", "error", closeErr)
		}
	}()

	// Walk source directory using the root-scoped filesystem view
	err = fs.WalkDir(root.FS(), ".", func(relPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// Update header with relative path
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file content for regular files
		if info.Mode().IsRegular() {
			f, err := root.Open(relPath)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := f.Close(); closeErr != nil {
					m.logger.Warn("failed to close source file", "error", closeErr)
				}
			}()

			if _, err := io.Copy(tarWriter, f); err != nil {
				return err
			}

			metadata.Files = append(metadata.Files, relPath)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Close writers in correct order to flush all data
	if err := tarWriter.Close(); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			m.logger.Warn("failed to close backup file after tar close error", "error", closeErr)
		}
		return err
	}

	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				m.logger.Warn("failed to close backup file after gzip close error", "error", closeErr)
			}
			return err
		}
	}

	if err := file.Close(); err != nil {
		return err
	}

	// Get file info for size
	fileInfo, err := os.Stat(metadata.BackupPath)
	if err != nil {
		return err
	}
	metadata.Size = fileInfo.Size()

	// Calculate checksum
	f, err := os.Open(metadata.BackupPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			m.logger.Warn("failed to close backup file after checksum", "error", closeErr)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	metadata.Checksum = hex.EncodeToString(hash.Sum(nil))

	return nil
}

// RestoreBackup restores from a backup
func (m *Manager) RestoreBackup(ctx context.Context, backupID string, targetPath string) error {
	m.restoreMu.Lock()
	if m.restoring {
		m.restoreMu.Unlock()
		return ErrRestoreInProgress
	}
	m.restoring = true
	m.restoreMu.Unlock()

	defer func() {
		m.restoreMu.Lock()
		m.restoring = false
		m.restoreMu.Unlock()
	}()

	// Get backup metadata
	m.mu.RLock()
	metadata, exists := m.backups[backupID]
	m.mu.RUnlock()

	if !exists {
		return ErrBackupNotFound
	}

	// Verify checksum
	if err := m.verifyBackup(metadata); err != nil {
		return err
	}

	// Perform restore
	return m.performRestore(ctx, metadata, targetPath)
}

// performRestore performs the actual restore operation
func (m *Manager) performRestore(ctx context.Context, metadata *BackupMetadata, targetPath string) error {
	// Open backup file
	file, err := os.Open(metadata.BackupPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			m.logger.Warn("failed to close backup file after restore", "error", closeErr)
		}
	}()

	// Create reader chain
	var reader io.Reader = file

	// Add gzip decompression
	if metadata.Compressed {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := gzipReader.Close(); closeErr != nil {
				m.logger.Warn("failed to close gzip reader", "error", closeErr)
			}
		}()
		reader = gzipReader
	}

	// Create tar reader
	tarReader := tar.NewReader(reader)

	// Extract files
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Construct target path and ensure it stays within targetPath to
		// prevent zip-slip / path traversal via a crafted archive entry name.
		cleanTargetPath := filepath.Clean(targetPath)
		// #nosec G305 -- result is validated immediately below to stay within cleanTargetPath before any use
		target := filepath.Join(cleanTargetPath, header.Name)
		if target != cleanTargetPath && !strings.HasPrefix(target, cleanTargetPath+string(os.PathSeparator)) {
			return fmt.Errorf("tar entry %q escapes target directory", header.Name)
		}

		// header.Mode is a tar-format int64; validate it fits the permission
		// bits expected by os.FileMode before converting to avoid overflow.
		if header.Mode < 0 || header.Mode > 0o7777 {
			return fmt.Errorf("tar entry %q has invalid file mode %d", header.Name, header.Mode)
		}
		fileMode := os.FileMode(header.Mode) // #nosec G115 -- bounds-checked above

		// Handle directories
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, fileMode); err != nil {
				return err
			}
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}

		// Create file
		// #nosec G304 -- target is validated above to stay within targetPath
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
		if err != nil {
			return err
		}

		// Copy content, bounded by the size declared in the tar header to
		// guard against decompression-bomb style archives.
		if header.Size < 0 {
			if closeErr := f.Close(); closeErr != nil {
				m.logger.Warn("failed to close restored file after invalid size", "error", closeErr)
			}
			return fmt.Errorf("tar entry %q has invalid size %d", header.Name, header.Size)
		}
		if _, err := io.CopyN(f, tarReader, header.Size); err != nil && err != io.EOF {
			if closeErr := f.Close(); closeErr != nil {
				m.logger.Warn("failed to close restored file after copy error", "error", closeErr)
			}
			return err
		}
		if err := f.Close(); err != nil {
			m.logger.Warn("failed to close restored file", "error", err)
		}
	}

	return nil
}

// verifyBackup verifies the integrity of a backup
func (m *Manager) verifyBackup(metadata *BackupMetadata) error {
	file, err := os.Open(metadata.BackupPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			m.logger.Warn("failed to close backup file after verify", "error", closeErr)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	if checksum != metadata.Checksum {
		return ErrInvalidBackup
	}

	return nil
}

// ListBackups returns all backups
func (m *Manager) ListBackups() []*BackupMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backups := make([]*BackupMetadata, 0, len(m.backups))
	for _, backup := range m.backups {
		backups = append(backups, backup)
	}

	return backups
}

// GetBackup returns a specific backup
func (m *Manager) GetBackup(backupID string) (*BackupMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backup, exists := m.backups[backupID]
	if !exists {
		return nil, ErrBackupNotFound
	}

	return backup, nil
}

// DeleteBackup deletes a backup
func (m *Manager) DeleteBackup(backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metadata, exists := m.backups[backupID]
	if !exists {
		return ErrBackupNotFound
	}

	// Delete backup file
	if err := os.Remove(metadata.BackupPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Delete metadata file
	metadataPath := filepath.Join(m.config.BackupDir, backupID+".json")
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	delete(m.backups, backupID)

	return nil
}

// saveMetadata saves backup metadata to disk
func (m *Manager) saveMetadata(metadata *BackupMetadata) error {
	metadataPath := filepath.Join(m.config.BackupDir, metadata.ID+".json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0600)
}

// loadBackups loads all backup metadata from disk
func (m *Manager) loadBackups() error {
	pattern := filepath.Join(m.config.BackupDir, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, path := range matches {
		// #nosec G304 -- path comes from filepath.Glob over m.config.BackupDir, which is supplied by the local operator via config
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var metadata BackupMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}

		m.backups[metadata.ID] = &metadata
	}

	return nil
}

// applyRetention applies backup retention policy
func (m *Manager) applyRetention() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Sort backups by time
	type backupTime struct {
		id   string
		time time.Time
	}

	var sorted []backupTime
	for id, backup := range m.backups {
		sorted = append(sorted, backupTime{id, backup.StartTime})
	}

	// Sort by time (oldest first)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].time.After(sorted[j].time) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Delete old backups
	now := time.Now()
	for i, bt := range sorted {
		backup := m.backups[bt.id]

		// Delete if exceeds count limit
		if i < len(sorted)-m.config.MaxBackups {
			if err := m.DeleteBackup(bt.id); err != nil {
				m.logger.Warn("failed to delete backup exceeding count limit", "backup_id", bt.id, "error", err)
			}
			continue
		}

		// Delete if exceeds retention days
		if m.config.RetentionDays > 0 {
			age := now.Sub(backup.StartTime)
			if age > time.Duration(m.config.RetentionDays)*24*time.Hour {
				if err := m.DeleteBackup(bt.id); err != nil {
					m.logger.Warn("failed to delete backup exceeding retention period", "backup_id", bt.id, "error", err)
				}
			}
		}
	}
}

// autoBackupLoop runs periodic backups
func (m *Manager) autoBackupLoop() {
	ticker := time.NewTicker(m.config.BackupInterval)
	defer ticker.Stop()

	m.logger.Info("starting auto backup loop",
		"interval", m.config.BackupInterval,
		"source_path", m.config.AutoBackupSourcePath,
		"backup_type", m.config.AutoBackupType)

	for {
		select {
		case <-ticker.C:
			m.logger.Debug("triggering automatic backup")

			// Create backup with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			metadata, err := m.CreateBackup(ctx, m.config.AutoBackupSourcePath, m.config.AutoBackupType)
			cancel()

			if err != nil {
				m.logger.Error("automatic backup failed",
					"error", err,
					"source_path", m.config.AutoBackupSourcePath)
			} else {
				m.logger.Info("automatic backup completed",
					"backup_id", metadata.ID,
					"size", metadata.Size,
					"duration", metadata.EndTime.Sub(metadata.StartTime))
			}

		case <-m.stopCh:
			m.logger.Info("stopping auto backup loop")
			return
		}
	}
}

// Stop gracefully stops the backup manager and terminates any running auto-backup loops
func (m *Manager) Stop() {
	if m.stopCh != nil {
		close(m.stopCh)
	}
}

// generateBackupID generates a unique backup ID
func generateBackupID() string {
	return fmt.Sprintf("backup-%d", time.Now().Unix())
}
