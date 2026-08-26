// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// DownloadTask represents a single file download task
type DownloadTask struct {
	URL         string
	Destination string
	Size        int64
	Name        string
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	Task            DownloadTask
	Success         bool
	Error           error
	Duration        time.Duration
	BytesDownloaded int64
}

// DownloadWorkerPool manages parallel download workers
type DownloadWorkerPool struct {
	workerCount int
	tasks       chan DownloadTask
	results     chan DownloadResult
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	log         logger.Logger

	// Progress tracking
	totalBytes      int64
	downloadedBytes int64
	startTime       time.Time
}

// Note: ProgressCallback is defined in cloud_storage.go

// NewDownloadWorkerPool creates a new worker pool for parallel downloads
func NewDownloadWorkerPool(ctx context.Context, workerCount int, log logger.Logger) *DownloadWorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)

	return &DownloadWorkerPool{
		workerCount: workerCount,
		tasks:       make(chan DownloadTask, workerCount*2),
		results:     make(chan DownloadResult, workerCount*2),
		ctx:         poolCtx,
		cancel:      cancel,
		log:         log,
		startTime:   time.Now(),
	}
}

// Start initializes and starts all worker goroutines
func (p *DownloadWorkerPool) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	p.log.Info("download worker pool started", "workers", p.workerCount)
}

// worker is the worker goroutine that processes download tasks
func (p *DownloadWorkerPool) worker(id int) {
	defer p.wg.Done()

	p.log.Debug("worker started", "id", id)

	for {
		select {
		case <-p.ctx.Done():
			p.log.Debug("worker stopped", "id", id)
			return

		case task, ok := <-p.tasks:
			if !ok {
				p.log.Debug("worker finished (channel closed)", "id", id)
				return
			}

			p.log.Info("worker processing task", "id", id, "file", task.Name, "size", formatBytes(task.Size))

			result := p.downloadFile(id, task)

			select {
			case p.results <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// downloadFile performs the actual file download
func (p *DownloadWorkerPool) downloadFile(workerID int, task DownloadTask) DownloadResult {
	startTime := time.Now()
	result := DownloadResult{
		Task:    task,
		Success: false,
	}

	// Create destination directory
	destDir := filepath.Dir(task.Destination)
	if err := os.MkdirAll(destDir, 0750); err != nil {
		result.Error = fmt.Errorf("create directory: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Open destination file
	destFile, err := os.Create(task.Destination)
	if err != nil {
		result.Error = fmt.Errorf("create file: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			p.log.Warn("failed to close destination file", "error", closeErr)
		}
	}()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(p.ctx, "GET", task.URL, nil)
	if err != nil {
		result.Error = fmt.Errorf("create request: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Perform HTTP request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("http request: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		result.Duration = time.Since(startTime)
		return result
	}

	// Download with progress tracking
	var bytesWritten int64
	buf := make([]byte, 32*1024) // 32KB buffer
	lastProgress := time.Now()

	for {
		select {
		case <-p.ctx.Done():
			result.Error = fmt.Errorf("download cancelled")
			result.Duration = time.Since(startTime)
			return result
		default:
		}

		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := destFile.Write(buf[0:nr])
			if nw > 0 {
				bytesWritten += int64(nw)
				atomic.AddInt64(&p.downloadedBytes, int64(nw))

				// Log progress every second to avoid spam
				if time.Since(lastProgress) > time.Second {
					p.log.Debug("download progress",
						"worker", workerID,
						"file", task.Name,
						"progress", fmt.Sprintf("%.1f%%", float64(bytesWritten)/float64(task.Size)*100),
						"bytes", formatBytes(bytesWritten))
					lastProgress = time.Now()
				}
			}
			if ew != nil {
				result.Error = fmt.Errorf("write to file: %w", ew)
				result.Duration = time.Since(startTime)
				return result
			}
			if nr != nw {
				result.Error = fmt.Errorf("short write: wrote %d bytes, read %d bytes", nw, nr)
				result.Duration = time.Since(startTime)
				return result
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			result.Error = fmt.Errorf("read from response: %w", er)
			result.Duration = time.Since(startTime)
			return result
		}
	}

	result.Success = true
	result.BytesDownloaded = bytesWritten
	result.Duration = time.Since(startTime)

	p.log.Info("download completed",
		"worker", workerID,
		"file", task.Name,
		"size", formatBytes(bytesWritten),
		"duration", result.Duration)

	return result
}

// Submit adds a download task to the queue
func (p *DownloadWorkerPool) Submit(task DownloadTask) error {
	// Check context cancellation first to avoid race condition
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("pool is shutting down")
	default:
	}

	// Try to submit task
	select {
	case p.tasks <- task:
		atomic.AddInt64(&p.totalBytes, task.Size)
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// Results returns the results channel
func (p *DownloadWorkerPool) Results() <-chan DownloadResult {
	return p.results
}

// GetProgress returns current download progress
func (p *DownloadWorkerPool) GetProgress() (downloaded, total int64, speed float64) {
	downloaded = atomic.LoadInt64(&p.downloadedBytes)
	total = atomic.LoadInt64(&p.totalBytes)

	elapsed := time.Since(p.startTime).Seconds()
	if elapsed > 0 {
		speed = float64(downloaded) / elapsed / (1024 * 1024) // MB/s
	}

	return
}

// Close shuts down the worker pool gracefully
func (p *DownloadWorkerPool) Close() error {
	p.log.Info("shutting down download worker pool")

	// Close tasks channel to signal workers
	close(p.tasks)

	// Wait for all workers to finish
	p.wg.Wait()

	// Close results channel
	close(p.results)

	// Cancel context
	p.cancel()

	p.log.Info("download worker pool shutdown complete")
	return nil
}

// Wait blocks until all submitted tasks are completed
func (p *DownloadWorkerPool) Wait() {
	p.wg.Wait()
}

// DownloadBatch downloads multiple files in parallel
func (p *DownloadWorkerPool) DownloadBatch(tasks []DownloadTask, progressCallback ProgressCallback) ([]DownloadResult, error) {
	results := make([]DownloadResult, 0, len(tasks))
	var resultsMu sync.Mutex

	// Start progress reporter
	progressTicker := time.NewTicker(500 * time.Millisecond)
	defer progressTicker.Stop()

	progressDone := make(chan struct{})
	go func() {
		for {
			select {
			case <-progressTicker.C:
				if progressCallback != nil {
					downloaded, total, _ := p.GetProgress()
					progressCallback(downloaded, total)
				}
			case <-progressDone:
				return
			case <-p.ctx.Done():
				return
			}
		}
	}()

	// Submit all tasks
	for _, task := range tasks {
		if err := p.Submit(task); err != nil {
			return results, fmt.Errorf("submit task %s: %w", task.Name, err)
		}
	}

	// Collect results
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < len(tasks); i++ {
			select {
			case result := <-p.results:
				resultsMu.Lock()
				results = append(results, result)
				resultsMu.Unlock()

				if !result.Success {
					p.log.Error("download failed",
						"file", result.Task.Name,
						"error", result.Error)
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	close(progressDone)

	return results, nil
}

// ResumeableDownloader handles downloads with resume capability
type ResumeableDownloader struct {
	checkpointFile string
	log            logger.Logger
}

// Checkpoint stores download state
type Checkpoint struct {
	FilePath        string
	BytesDownloaded int64
	TotalBytes      int64
	Timestamp       time.Time
	Completed       bool
}

// NewResumeableDownloader creates a downloader with resume support
func NewResumeableDownloader(checkpointFile string, log logger.Logger) *ResumeableDownloader {
	return &ResumeableDownloader{
		checkpointFile: checkpointFile,
		log:            log,
	}
}

// SaveCheckpoint saves current download state
func (r *ResumeableDownloader) SaveCheckpoint(cp Checkpoint) error {
	// Create checkpoint directory if it doesn't exist
	checkpointDir := filepath.Dir(r.checkpointFile)
	if err := os.MkdirAll(checkpointDir, 0750); err != nil {
		return fmt.Errorf("create checkpoint directory: %w", err)
	}

	// Encode checkpoint to JSON
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	// Write to temporary file first for atomic operation
	tempFile := r.checkpointFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, r.checkpointFile); err != nil {
		if removeErr := os.Remove(tempFile); removeErr != nil {
			r.log.Warn("failed to remove temp checkpoint file", "file", tempFile, "error", removeErr)
		}
		return fmt.Errorf("rename checkpoint: %w", err)
	}

	r.log.Debug("checkpoint saved",
		"file", cp.FilePath,
		"bytes", formatBytes(cp.BytesDownloaded),
		"progress", fmt.Sprintf("%.1f%%", float64(cp.BytesDownloaded)/float64(cp.TotalBytes)*100))

	return nil
}

// LoadCheckpoint loads saved download state
func (r *ResumeableDownloader) LoadCheckpoint() (*Checkpoint, error) {
	// Check if checkpoint file exists
	if _, err := os.Stat(r.checkpointFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("no checkpoint found")
	}

	// Read checkpoint file
	data, err := os.ReadFile(r.checkpointFile)
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}

	// Decode JSON
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}

	r.log.Info("checkpoint loaded",
		"file", filepath.Base(cp.FilePath),
		"bytes", formatBytes(cp.BytesDownloaded),
		"progress", fmt.Sprintf("%.1f%%", float64(cp.BytesDownloaded)/float64(cp.TotalBytes)*100),
		"completed", cp.Completed)

	return &cp, nil
}

// DownloadWithResume performs a resumeable download
func (r *ResumeableDownloader) DownloadWithResume(ctx context.Context, task DownloadTask, progressCallback func(int64, int64)) error {
	// Check for existing checkpoint
	cp, err := r.LoadCheckpoint()
	startOffset := int64(0)
	var destFile *os.File

	if err == nil && cp != nil && cp.FilePath == task.Destination && !cp.Completed {
		startOffset = cp.BytesDownloaded
		r.log.Info("resuming download", "file", task.Name, "offset", formatBytes(startOffset))

		// Open file in append mode for resuming
		destFile, err = os.OpenFile(task.Destination, os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			// If we can't open for append, start over
			r.log.Warn("cannot resume download, starting over", "error", err)
			startOffset = 0
			destFile = nil
		}
	}

	// If not resuming, create new file
	if destFile == nil {
		// Create destination directory
		destDir := filepath.Dir(task.Destination)
		if err := os.MkdirAll(destDir, 0750); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		destFile, err = os.Create(task.Destination)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			r.log.Warn("failed to close destination file", "error", closeErr)
		}
	}()

	// Create HTTP request with Range header for resumeable download
	req, err := http.NewRequestWithContext(ctx, "GET", task.URL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set Range header if resuming
	if startOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startOffset))
		r.log.Info("requesting range", "start", formatBytes(startOffset))
	}

	// Perform HTTP request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.log.Warn("failed to close HTTP response body", "error", closeErr)
		}
	}()

	// Check response status
	if startOffset > 0 {
		// Expect 206 Partial Content if resuming
		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d (expected 206 or 200)", resp.StatusCode)
		}
		if resp.StatusCode == http.StatusOK {
			// Server doesn't support Range, start over
			r.log.Warn("server doesn't support Range header, starting over")
			if closeErr := destFile.Close(); closeErr != nil {
				r.log.Warn("failed to close destination file before recreate", "error", closeErr)
			}
			destFile, err = os.Create(task.Destination)
			if err != nil {
				return fmt.Errorf("recreate file: %w", err)
			}
			defer func() {
				if closeErr := destFile.Close(); closeErr != nil {
					r.log.Warn("failed to close recreated destination file", "error", closeErr)
				}
			}()
			startOffset = 0
		}
	} else {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
	}

	// Track progress
	var bytesDownloaded = startOffset
	var mu sync.Mutex

	// Periodic checkpoint saving
	checkpointTicker := time.NewTicker(5 * time.Second)
	defer checkpointTicker.Stop()

	checkpointDone := make(chan struct{})
	defer close(checkpointDone)

	go func() {
		for {
			select {
			case <-checkpointTicker.C:
				mu.Lock()
				currentBytes := bytesDownloaded
				mu.Unlock()

				if err := r.SaveCheckpoint(Checkpoint{
					FilePath:        task.Destination,
					BytesDownloaded: currentBytes,
					TotalBytes:      task.Size,
					Timestamp:       time.Now(),
					Completed:       false,
				}); err != nil {
					r.log.Warn("failed to save periodic checkpoint", "error", err)
				}
			case <-checkpointDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Download with progress tracking
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("download cancelled")
		default:
		}

		nr, err := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := destFile.Write(buf[0:nr])
			if nw > 0 {
				mu.Lock()
				bytesDownloaded += int64(nw)
				current := bytesDownloaded
				mu.Unlock()

				if progressCallback != nil {
					progressCallback(current, task.Size)
				}
			}
			if ew != nil {
				return fmt.Errorf("write to file: %w", ew)
			}
			if nr != nw {
				return fmt.Errorf("short write: wrote %d bytes, read %d bytes", nw, nr)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read from response: %w", err)
		}
	}

	// Save final checkpoint
	err = r.SaveCheckpoint(Checkpoint{
		FilePath:        task.Destination,
		BytesDownloaded: bytesDownloaded,
		TotalBytes:      task.Size,
		Timestamp:       time.Now(),
		Completed:       true,
	})
	if err != nil {
		r.log.Warn("failed to save final checkpoint", "error", err)
	}

	r.log.Info("download completed",
		"file", task.Name,
		"size", formatBytes(bytesDownloaded),
		"resumed_from", formatBytes(startOffset))

	return nil
}
