// SPDX-License-Identifier: Apache-2.0

package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCheckpoint(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")

	if checkpoint.VMName != "test-vm" {
		t.Errorf("Expected VMName 'test-vm', got '%s'", checkpoint.VMName)
	}

	if checkpoint.Provider != "vsphere" {
		t.Errorf("Expected Provider 'vsphere', got '%s'", checkpoint.Provider)
	}

	if checkpoint.ExportFormat != "ova" {
		t.Errorf("Expected ExportFormat 'ova', got '%s'", checkpoint.ExportFormat)
	}

	if checkpoint.OutputPath != "/tmp/output" {
		t.Errorf("Expected OutputPath '/tmp/output', got '%s'", checkpoint.OutputPath)
	}

	if checkpoint.Version != CheckpointVersion {
		t.Errorf("Expected Version '%s', got '%s'", CheckpointVersion, checkpoint.Version)
	}

	if checkpoint.Files == nil {
		t.Error("Expected Files slice to be initialized")
	}

	if checkpoint.Metadata == nil {
		t.Error("Expected Metadata map to be initialized")
	}

	if checkpoint.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if checkpoint.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestCheckpointAddFile(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")

	checkpoint.AddFile("disk1.vmdk", "https://example.com/disk1.vmdk", 1024*1024*100)

	if len(checkpoint.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(checkpoint.Files))
	}

	file := checkpoint.Files[0]
	if file.Path != "disk1.vmdk" {
		t.Errorf("Expected Path 'disk1.vmdk', got '%s'", file.Path)
	}

	if file.URL != "https://example.com/disk1.vmdk" {
		t.Errorf("Expected URL 'https://example.com/disk1.vmdk', got '%s'", file.URL)
	}

	if file.TotalSize != 1024*1024*100 {
		t.Errorf("Expected TotalSize %d, got %d", 1024*1024*100, file.TotalSize)
	}

	if file.DownloadedSize != 0 {
		t.Errorf("Expected DownloadedSize 0, got %d", file.DownloadedSize)
	}

	if file.Status != "pending" {
		t.Errorf("Expected Status 'pending', got '%s'", file.Status)
	}

	if file.RetryCount != 0 {
		t.Errorf("Expected RetryCount 0, got %d", file.RetryCount)
	}
}

func TestCheckpointUpdateFileProgress(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")
	checkpoint.AddFile("disk1.vmdk", "https://example.com/disk1.vmdk", 1024*1024*100)

	// Update progress
	checkpoint.UpdateFileProgress("disk1.vmdk", 1024*1024*50, "downloading")

	file := checkpoint.Files[0]
	if file.DownloadedSize != 1024*1024*50 {
		t.Errorf("Expected DownloadedSize %d, got %d", 1024*1024*50, file.DownloadedSize)
	}

	if file.Status != "downloading" {
		t.Errorf("Expected Status 'downloading', got '%s'", file.Status)
	}

	// Update to completed
	checkpoint.UpdateFileProgress("disk1.vmdk", 1024*1024*100, "completed")

	file = checkpoint.Files[0]
	if file.Status != "completed" {
		t.Errorf("Expected Status 'completed', got '%s'", file.Status)
	}
}

func TestCheckpointUpdateFileProgress_NonexistentFile(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")
	checkpoint.AddFile("disk1.vmdk", "https://example.com/disk1.vmdk", 1024*1024*100)

	// Try to update a file that doesn't exist - should not panic
	checkpoint.UpdateFileProgress("nonexistent.vmdk", 1024, "downloading")

	// Original file should be unchanged
	file := checkpoint.Files[0]
	if file.DownloadedSize != 0 {
		t.Error("Expected original file to be unchanged")
	}
}

func TestCheckpointGetFileProgress(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")
	checkpoint.AddFile("disk1.vmdk", "https://example.com/disk1.vmdk", 1024*1024*100)

	file := checkpoint.GetFileProgress("disk1.vmdk")
	if file == nil {
		t.Fatal("Expected to find file")
	}

	if file.Path != "disk1.vmdk" {
		t.Errorf("Expected Path 'disk1.vmdk', got '%s'", file.Path)
	}

	// Test nonexistent file
	file = checkpoint.GetFileProgress("nonexistent.vmdk")
	if file != nil {
		t.Error("Expected nil for nonexistent file")
	}
}

func TestCheckpointIsComplete(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")

	// Empty checkpoint is not complete
	if checkpoint.IsComplete() {
		t.Error("Expected empty checkpoint to not be complete")
	}

	// Add files
	checkpoint.AddFile("disk1.vmdk", "", 1024)
	checkpoint.AddFile("disk2.vmdk", "", 2048)

	// Not complete yet (status is pending)
	if checkpoint.IsComplete() {
		t.Error("Expected checkpoint with pending files to not be complete")
	}

	// Complete one file
	checkpoint.UpdateFileProgress("disk1.vmdk", 1024, "completed")

	// Still not complete
	if checkpoint.IsComplete() {
		t.Error("Expected checkpoint with one pending file to not be complete")
	}

	// Complete second file
	checkpoint.UpdateFileProgress("disk2.vmdk", 2048, "completed")

	// Now complete
	if !checkpoint.IsComplete() {
		t.Error("Expected checkpoint with all completed files to be complete")
	}
}

func TestCheckpointGetProgress(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")

	// Empty checkpoint has 0 progress
	if checkpoint.GetProgress() != 0.0 {
		t.Errorf("Expected progress 0.0, got %f", checkpoint.GetProgress())
	}

	// Add files
	checkpoint.AddFile("disk1.vmdk", "", 1000)
	checkpoint.AddFile("disk2.vmdk", "", 1000)

	// No downloads yet
	if checkpoint.GetProgress() != 0.0 {
		t.Errorf("Expected progress 0.0, got %f", checkpoint.GetProgress())
	}

	// Download 50% of first file
	checkpoint.UpdateFileProgress("disk1.vmdk", 500, "downloading")

	progress := checkpoint.GetProgress()
	expectedProgress := 0.25 // 500 / 2000 = 0.25
	if progress < expectedProgress-0.01 || progress > expectedProgress+0.01 {
		t.Errorf("Expected progress ~%f, got %f", expectedProgress, progress)
	}

	// Download 100% of both files
	checkpoint.UpdateFileProgress("disk1.vmdk", 1000, "completed")
	checkpoint.UpdateFileProgress("disk2.vmdk", 1000, "completed")

	if checkpoint.GetProgress() != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", checkpoint.GetProgress())
	}
}

func TestCheckpointGetProgress_ZeroSize(t *testing.T) {
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")

	// Add files with zero size
	checkpoint.AddFile("file1.txt", "", 0)
	checkpoint.AddFile("file2.txt", "", 0)

	// Should return 0 to avoid division by zero
	if checkpoint.GetProgress() != 0.0 {
		t.Errorf("Expected progress 0.0 for zero-sized files, got %f", checkpoint.GetProgress())
	}
}

func TestCheckpointSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointPath := filepath.Join(tmpDir, "test.checkpoint")

	// Create checkpoint
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")
	checkpoint.AddFile("disk1.vmdk", "https://example.com/disk1.vmdk", 1024*1024*100)
	checkpoint.UpdateFileProgress("disk1.vmdk", 1024*1024*50, "downloading")
	checkpoint.Metadata["key1"] = "value1"

	// Save
	if err := checkpoint.Save(checkpointPath); err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(checkpointPath); err != nil {
		t.Fatalf("Checkpoint file not found: %v", err)
	}

	// Load
	loaded, err := LoadCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("Failed to load checkpoint: %v", err)
	}

	// Verify fields
	if loaded.VMName != checkpoint.VMName {
		t.Errorf("VMName mismatch: got %s, want %s", loaded.VMName, checkpoint.VMName)
	}

	if loaded.Provider != checkpoint.Provider {
		t.Errorf("Provider mismatch: got %s, want %s", loaded.Provider, checkpoint.Provider)
	}

	if len(loaded.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(loaded.Files))
	}

	if loaded.Files[0].Path != "disk1.vmdk" {
		t.Errorf("File path mismatch: got %s, want disk1.vmdk", loaded.Files[0].Path)
	}

	if loaded.Files[0].DownloadedSize != 1024*1024*50 {
		t.Errorf("Downloaded size mismatch: got %d, want %d", loaded.Files[0].DownloadedSize, 1024*1024*50)
	}

	if loaded.Metadata["key1"] != "value1" {
		t.Error("Metadata not preserved")
	}
}

func TestLoadCheckpoint_NonexistentFile(t *testing.T) {
	_, err := LoadCheckpoint("/nonexistent/checkpoint.json")
	if err == nil {
		t.Error("Expected error for nonexistent checkpoint file")
	}
}

func TestLoadCheckpoint_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointPath := filepath.Join(tmpDir, "invalid.checkpoint")

	// Write invalid JSON
	if err := os.WriteFile(checkpointPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid checkpoint: %v", err)
	}

	_, err := LoadCheckpoint(checkpointPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoadCheckpoint_WrongVersion(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointPath := filepath.Join(tmpDir, "wrong-version.checkpoint")

	// Create checkpoint with wrong version
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")
	checkpoint.Version = "999.0"

	if err := checkpoint.Save(checkpointPath); err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	_, err := LoadCheckpoint(checkpointPath)
	if err == nil {
		t.Error("Expected error for incompatible checkpoint version")
	}
}

func TestDeleteCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointPath := filepath.Join(tmpDir, "test.checkpoint")

	// Create and save checkpoint
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")
	if err := checkpoint.Save(checkpointPath); err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Delete
	if err := DeleteCheckpoint(checkpointPath); err != nil {
		t.Fatalf("Failed to delete checkpoint: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(checkpointPath); !os.IsNotExist(err) {
		t.Error("Checkpoint file should not exist after deletion")
	}
}

func TestDeleteCheckpoint_NonexistentFile(t *testing.T) {
	// Should not error when deleting nonexistent file
	err := DeleteCheckpoint("/nonexistent/checkpoint.json")
	if err != nil {
		t.Errorf("DeleteCheckpoint should not error for nonexistent file: %v", err)
	}
}

func TestComputeChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-file.txt")

	content := []byte("test content for checksum")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	checksum, err := ComputeChecksum(filePath)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	// Verify checksum is 64 hex characters (SHA-256)
	if len(checksum) != 64 {
		t.Errorf("Expected checksum length 64, got %d", len(checksum))
	}

	// Compute again and verify it's the same
	checksum2, err := ComputeChecksum(filePath)
	if err != nil {
		t.Fatalf("Failed to compute checksum second time: %v", err)
	}

	if checksum != checksum2 {
		t.Error("Checksum should be deterministic")
	}
}

func TestComputeChecksum_NonexistentFile(t *testing.T) {
	_, err := ComputeChecksum("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestGetCheckpointPath(t *testing.T) {
	path := GetCheckpointPath("/tmp/output", "test-vm")

	expected := filepath.Join("/tmp/output", ".test-vm.checkpoint")
	if path != expected {
		t.Errorf("Expected path %s, got %s", expected, path)
	}
}

func TestCheckpointExists(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointPath := filepath.Join(tmpDir, "test.checkpoint")

	// Should not exist initially
	if CheckpointExists(checkpointPath) {
		t.Error("Checkpoint should not exist initially")
	}

	// Create checkpoint
	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")
	if err := checkpoint.Save(checkpointPath); err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Should exist now
	if !CheckpointExists(checkpointPath) {
		t.Error("Checkpoint should exist after saving")
	}
}

func TestCheckpointSave_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointPath := filepath.Join(tmpDir, "nested", "dir", "test.checkpoint")

	checkpoint := NewCheckpoint("test-vm", "vsphere", "ova", "/tmp/output")

	// Should create nested directories
	if err := checkpoint.Save(checkpointPath); err != nil {
		t.Fatalf("Failed to save checkpoint with nested directories: %v", err)
	}

	// Verify file exists
	if !CheckpointExists(checkpointPath) {
		t.Error("Checkpoint should exist after saving with nested directories")
	}
}
