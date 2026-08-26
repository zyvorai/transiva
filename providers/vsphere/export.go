// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/zyvorai/transiva/manifest"
	"github.com/zyvorai/transiva/progress"
	"github.com/zyvorai/transiva/providers/common"
	"github.com/zyvorai/transiva/retry"
)

const (
	// maxFilenameLength is the maximum allowed filename length on most filesystems
	maxFilenameLength = 255
)

// sanitizeVMName sanitizes a VM name for safe use in file paths
func sanitizeVMName(name string) string {
	// Replace directory separators and path traversal attempts
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "..", "-")

	// Remove null bytes and other invalid characters
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "*", "-")
	name = strings.ReplaceAll(name, "?", "-")
	name = strings.ReplaceAll(name, "\"", "-")
	name = strings.ReplaceAll(name, "<", "-")
	name = strings.ReplaceAll(name, ">", "-")
	name = strings.ReplaceAll(name, "|", "-")

	// Trim dangerous prefixes/suffixes
	name = strings.Trim(name, ".-")

	// Default if empty
	if name == "" {
		name = "unnamed-vm"
	}

	// Limit length
	if len(name) > maxFilenameLength {
		name = name[:maxFilenameLength]
	}

	return name
}

// progressWriter implements io.Writer to update progress bar
type progressWriter struct {
	progressBar progress.ProgressReporter
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.progressBar.Add(int64(n))
	return n, nil
}

// callbackProgressReporter wraps a progress reporter and calls a callback function
type callbackProgressReporter struct {
	inner      progress.ProgressReporter
	callback   func(current, total int64, fileName string, fileIndex, totalFiles int)
	totalBytes *int64 // pointer to shared atomic counter
	totalSize  int64
	fileName   string
	fileIndex  int
	totalFiles int
}

func (cpr *callbackProgressReporter) Start(total int64, description string) {
	if cpr.inner != nil {
		cpr.inner.Start(total, description)
	}
}

func (cpr *callbackProgressReporter) Add(delta int64) {
	if cpr.inner != nil {
		cpr.inner.Add(delta)
	}
	// Atomically update total bytes and call callback
	current := atomic.AddInt64(cpr.totalBytes, delta)
	if cpr.callback != nil {
		cpr.callback(current, cpr.totalSize, cpr.fileName, cpr.fileIndex, cpr.totalFiles)
	}
}

func (cpr *callbackProgressReporter) Update(current int64) {
	if cpr.inner != nil {
		cpr.inner.Update(current)
	}
}

func (cpr *callbackProgressReporter) SetTotal(total int64) {
	if cpr.inner != nil {
		cpr.inner.SetTotal(total)
	}
}

func (cpr *callbackProgressReporter) Finish() {
	if cpr.inner != nil {
		cpr.inner.Finish()
	}
}

func (cpr *callbackProgressReporter) Close() error {
	if cpr.inner != nil {
		return cpr.inner.Close()
	}
	return nil
}

func (cpr *callbackProgressReporter) Describe(description string) {
	if cpr.inner != nil {
		cpr.inner.Describe(description)
	}
}

// ExportOVF exports a VM as OVF format with progress tracking
func (c *VSphereClient) ExportOVF(ctx context.Context, vmPath string, opts ExportOptions) (*ExportResult, error) {
	startTime := time.Now()
	c.logger.Info("starting OVF export", "vm", vmPath, "output", opts.OutputPath)

	// Validate and prepare output directory
	outputDir, err := c.prepareOutputDirectory(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("prepare output directory: %w", err)
	}

	// Get VM with retry
	var vm *object.VirtualMachine
	vmResult, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying VM lookup", "vm", vmPath, "attempt", attempt)
		}

		foundVM, err := c.finder.VirtualMachine(ctx, vmPath)
		if err != nil {
			// VM not found is not retryable
			if strings.Contains(err.Error(), "not found") {
				return nil, retry.IsNonRetryable(err)
			}
			return nil, fmt.Errorf("find VM: %w", err)
		}
		return foundVM, nil
	}, fmt.Sprintf("find VM %s", vmPath))

	if err != nil {
		return nil, err
	}
	vm = vmResult.(*object.VirtualMachine)

	// Initialize or load checkpoint if enabled
	var checkpoint *common.Checkpoint
	var checkpointPath string
	if opts.EnableCheckpoints {
		// Determine checkpoint path
		if opts.CheckpointPath != "" {
			checkpointPath = opts.CheckpointPath
		} else {
			checkpointPath = common.GetCheckpointPath(outputDir, sanitizeVMName(vm.Name()))
		}

		// Try to load existing checkpoint if resumption is enabled
		if opts.ResumeFromCheckpoint && common.CheckpointExists(checkpointPath) {
			loadedCheckpoint, err := common.LoadCheckpoint(checkpointPath)
			if err != nil {
				c.logger.Warn("failed to load checkpoint, starting fresh", "error", err)
				checkpoint = common.NewCheckpoint(vm.Name(), "vsphere", opts.Format, outputDir)
			} else {
				c.logger.Info("resuming from checkpoint", "progress", loadedCheckpoint.GetProgress())
				checkpoint = loadedCheckpoint
			}
		} else {
			// Create new checkpoint
			checkpoint = common.NewCheckpoint(vm.Name(), "vsphere", opts.Format, outputDir)
		}
	}

	// Remove CD/DVD devices if requested
	if opts.RemoveCDROM {
		if err := c.RemoveCDROMDevices(ctx, vmPath); err != nil {
			c.logger.Warn("failed to remove CD/DVD devices", "error", err)
		}
	}

	// Create OVF descriptor
	ovfPath, err := c.createOVFDescriptor(ctx, vm, outputDir)
	if err != nil {
		return nil, fmt.Errorf("create OVF descriptor: %w", err)
	}

	// Start export lease with retry and proper cleanup
	var lease *nfc.Lease
	leaseResult, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying export lease creation", "vm", vmPath, "attempt", attempt)
		}

		exportLease, err := vm.Export(ctx)
		if err != nil {
			return nil, fmt.Errorf("start export lease: %w", err)
		}
		return exportLease, nil
	}, fmt.Sprintf("create export lease for %s", vmPath))

	if err != nil {
		return nil, err
	}
	lease = leaseResult.(*nfc.Lease)

	// Defer lease cleanup
	defer func() {
		if err != nil {
			if abortErr := lease.Abort(ctx, nil); abortErr != nil {
				c.logger.Warn("failed to abort lease", "error", abortErr)
			}
		}
	}()

	// Wait for lease to be ready with retry and timeout
	leaseCtx, leaseCancel := context.WithTimeout(ctx, leaseWaitTimeout)
	defer leaseCancel()

	var info *nfc.LeaseInfo
	infoResult, err := c.retryer.DoWithResult(leaseCtx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying lease wait", "vm", vmPath, "attempt", attempt)
		}

		leaseInfo, err := lease.Wait(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("wait for lease ready: %w", err)
		}
		return leaseInfo, nil
	}, fmt.Sprintf("wait for lease ready %s", vmPath))

	if err != nil {
		return nil, err
	}
	info = infoResult.(*nfc.LeaseInfo)

	// Calculate total size
	totalSize := int64(0)
	for _, item := range info.Items {
		totalSize += item.Size
	}

	c.logger.Info("starting download", "files", len(info.Items), "totalSize", totalSize)

	// Create multi-progress manager for parallel downloads (only if progress bars enabled)
	var multiProgress *progress.MultiProgress
	var overallBar *progress.BarProgress
	if opts.ShowOverallProgress || opts.ShowIndividualProgress {
		multiProgress = progress.NewMultiProgress()
		defer multiProgress.Close()

		// Create overall progress bar
		if opts.ShowOverallProgress {
			overallBar = progress.NewOverallProgress(os.Stderr, vm.Name(), len(info.Items))
			overallBar.Start(int64(len(info.Items)), "Files")
			multiProgress.AddBar(overallBar)
		}
	}

	// Download files
	downloadCtx, downloadCancel := context.WithTimeout(ctx, downloadTimeout)
	defer downloadCancel()

	var fileBars []*progress.BarProgress
	if opts.ShowIndividualProgress && multiProgress != nil {
		// Create individual progress bars for each file
		for _, item := range info.Items {
			bar := progress.NewDownloadProgress(os.Stderr, filepath.Base(item.Path), item.Size)
			bar.Start(item.Size, "Downloading")
			fileBars = append(fileBars, bar)
			multiProgress.AddBar(bar)
		}
	}

	downloadedFiles, err := c.downloadFilesParallel(
		downloadCtx,
		info.Items,
		outputDir,
		opts.ParallelDownloads,
		overallBar,
		fileBars,
		opts.ProgressCallback,
		totalSize,
		opts.BandwidthLimit,
		opts.BandwidthBurst,
		checkpoint,
		checkpointPath,
		opts.CheckpointInterval,
	)
	if err != nil {
		return nil, fmt.Errorf("download files: %w", err)
	}

	// Finish progress bars
	if overallBar != nil {
		overallBar.Finish()
	}
	for _, bar := range fileBars {
		bar.Finish()
	}

	// Complete lease with retry
	err = c.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt > 1 {
			c.logger.Info("retrying lease completion", "vm", vmPath, "attempt", attempt)
		}

		if err := lease.Complete(ctx); err != nil {
			return fmt.Errorf("complete lease: %w", err)
		}
		return nil
	}, fmt.Sprintf("complete lease for %s", vmPath))

	if err != nil {
		return nil, err
	}

	duration := time.Since(startTime)
	result := &ExportResult{
		OutputDir: outputDir,
		OVFPath:   ovfPath,
		Format:    opts.Format,
		Files:     downloadedFiles,
		TotalSize: totalSize,
		Duration:  duration,
	}

	// Create OVA if requested
	if opts.Format == "ova" {
		c.logger.Info("packaging OVF export as OVA", "compress", opts.Compress)
		sanitizedName := sanitizeVMName(vm.Name())

		// Determine file extension based on compression
		var ovaPath string
		if opts.Compress {
			ovaPath = filepath.Join(outputDir, sanitizedName+".ova.gz")
		} else {
			ovaPath = filepath.Join(outputDir, sanitizedName+".ova")
		}

		// Set default compression level if not specified
		compressionLevel := opts.CompressionLevel
		if compressionLevel == 0 && opts.Compress {
			compressionLevel = 6 // gzip.DefaultCompression
		}

		if err := CreateOVA(outputDir, ovaPath, opts.Compress, compressionLevel, c.logger); err != nil {
			return nil, fmt.Errorf("create OVA: %w", err)
		}

		result.OVAPath = ovaPath
		result.Format = "ova"

		// Optionally cleanup OVF files
		if opts.CleanupOVF {
			c.logger.Info("cleaning up intermediate OVF files")
			for _, file := range downloadedFiles {
				if err := os.Remove(file); err != nil {
					c.logger.Warn("failed to remove OVF file", "file", file, "error", err)
				}
			}
			// Also remove the OVF descriptor
			if err := os.Remove(ovfPath); err != nil {
				c.logger.Warn("failed to remove OVF descriptor", "error", err)
			}
		}

		// Get file size for reporting
		if fi, err := os.Stat(ovaPath); err == nil {
			c.logger.Info("OVA package created successfully",
				"path", ovaPath,
				"size", fi.Size(),
				"compressed", opts.Compress)
		}
	}

	// Generate Artifact Manifest v1.0 if requested
	if opts.GenerateManifest {
		c.logger.Info("generating Artifact Manifest v1.0")

		manifestPath, err := c.generateArtifactManifest(ctx, vm, result, opts)
		if err != nil {
			c.logger.Warn("failed to generate manifest", "error", err)
			// Don't fail the export if manifest generation fails
		} else {
			result.ManifestPath = manifestPath
			c.logger.Info("Artifact Manifest v1.0 created", "path", manifestPath)

			// Verify manifest if requested
			if opts.VerifyManifest {
				c.logger.Info("verifying manifest")
				if err := c.verifyManifest(manifestPath); err != nil {
					c.logger.Warn("manifest verification failed", "error", err)
				} else {
					c.logger.Info("manifest verification successful")
				}
			}

			// Note: Old AutoConvert code removed - replaced with pipeline integration below
		}
	}

	c.logger.Info("export completed successfully",
		"vm", vmPath,
		"format", result.Format,
		"duration", duration,
		"totalSize", totalSize,
		"files", len(downloadedFiles))

	// Delete checkpoint file on successful completion
	if checkpoint != nil && checkpointPath != "" {
		if err := common.DeleteCheckpoint(checkpointPath); err != nil {
			c.logger.Warn("failed to delete checkpoint", "error", err)
		} else {
			c.logger.Info("checkpoint deleted after successful export")
		}
	}

	// Run hyper2kvm pipeline if enabled
	if opts.EnablePipeline && result.ManifestPath != "" {
		c.logger.Info("starting hyper2kvm pipeline", "manifest", result.ManifestPath)

		pipelineConfig := &common.Hyper2KVMConfig{
			Enabled:            true,
			Hyper2KVMPath:      opts.Hyper2KVMPath,
			ManifestPath:       result.ManifestPath,
			LibvirtIntegration: opts.LibvirtIntegration,
			LibvirtURI:         opts.LibvirtURI,
			AutoStart:          opts.LibvirtAutoStart,
			Verbose:            opts.StreamPipelineOutput,
			DryRun:             opts.PipelineDryRun,

			// Daemon options
			UseDaemon:          opts.Hyper2KVMDaemon,
			DaemonInstance:     opts.Hyper2KVMInstance,
			DaemonWatchDir:     opts.Hyper2KVMWatchDir,
			DaemonOutputDir:    opts.Hyper2KVMOutputDir,
			DaemonPollInterval: opts.Hyper2KVMPollInterval,
			DaemonTimeout:      opts.Hyper2KVMDaemonTimeout,
		}

		executor := common.NewPipelineExecutor(pipelineConfig, c.logger)

		pipelineCtx := ctx
		if opts.PipelineTimeout > 0 {
			var cancel context.CancelFunc
			pipelineCtx, cancel = context.WithTimeout(ctx, opts.PipelineTimeout)
			defer cancel()
		}

		pipelineResult, err := executor.Execute(pipelineCtx)
		if err != nil {
			c.logger.Error("pipeline failed (non-fatal)", "error", err)
			// Store pipeline error in result metadata but don't fail the export
			if result.Metadata == nil {
				result.Metadata = make(map[string]interface{})
			}
			result.Metadata["pipeline_error"] = err.Error()
			result.Metadata["pipeline_success"] = false
		} else {
			c.logger.Info("pipeline completed successfully",
				"duration", pipelineResult.Duration,
				"output_path", pipelineResult.OutputPath,
				"libvirt_domain", pipelineResult.LibvirtDomain)

			// Store pipeline result in metadata
			if result.Metadata == nil {
				result.Metadata = make(map[string]interface{})
			}
			result.Metadata["pipeline_success"] = pipelineResult.Success
			result.Metadata["pipeline_duration"] = pipelineResult.Duration.String()
			result.Metadata["converted_path"] = pipelineResult.OutputPath
			if pipelineResult.LibvirtDomain != "" {
				result.Metadata["libvirt_domain"] = pipelineResult.LibvirtDomain
				result.Metadata["libvirt_uri"] = opts.LibvirtURI
			}
		}
	}

	return result, nil
}

func (c *VSphereClient) prepareOutputDirectory(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %w", err)
	}

	if err := os.MkdirAll(absPath, defaultDirPerm); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	return absPath, nil
}

func (c *VSphereClient) createOVFDescriptor(ctx context.Context, vm *object.VirtualMachine, outputDir string) (string, error) {
	// Sanitize VM name to prevent path traversal
	sanitizedName := sanitizeVMName(vm.Name())
	ovfPath := filepath.Join(outputDir, sanitizedName+".ovf")

	// Create OVF manager
	manager := ovf.NewManager(c.client.Client)

	// Create descriptor with default parameters
	cdp := types.OvfCreateDescriptorParams{}
	desc, err := manager.CreateDescriptor(ctx, vm, cdp)
	if err != nil {
		return "", fmt.Errorf("create OVF descriptor: %w", err)
	}

	// Write descriptor to file
	// #nosec G304 -- ovfPath is built from outputDir (operator/config-supplied) and a sanitized VM name
	file, err := os.Create(ovfPath)
	if err != nil {
		return "", fmt.Errorf("create OVF file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteString(desc.OvfDescriptor); err != nil {
		return "", fmt.Errorf("write OVF descriptor: %w", err)
	}

	c.logger.Debug("created OVF descriptor", "path", ovfPath)
	return ovfPath, nil
}

func (c *VSphereClient) downloadFilesParallel(
	ctx context.Context,
	items []nfc.FileItem,
	outputDir string,
	concurrency int,
	overallBar progress.ProgressReporter,
	fileBars []*progress.BarProgress,
	progressCallback func(current, total int64, fileName string, fileIndex, totalFiles int),
	totalSize int64,
	bandwidthLimit int64,
	bandwidthBurst int,
	checkpoint *common.Checkpoint,
	checkpointPath string,
	checkpointInterval time.Duration,
) ([]string, error) {
	if concurrency < 1 {
		concurrency = 1
	}

	var (
		wg                   sync.WaitGroup
		sem                  = make(chan struct{}, concurrency)
		errCh                = make(chan error, len(items))
		results              = make([]string, len(items))
		resultsMux           sync.Mutex
		totalBytesDownloaded int64      // Track cumulative progress
		checkpointMux        sync.Mutex // Protect checkpoint updates
		lastCheckpointSave   time.Time
	)

	// Initialize checkpoint with all files if enabled
	if checkpoint != nil {
		for _, item := range items {
			// Check if file already exists in checkpoint
			if checkpoint.GetFileProgress(item.Path) == nil {
				checkpoint.AddFile(item.Path, item.URL.String(), item.Size)
			}
		}
		// Save initial checkpoint
		if err := checkpoint.Save(checkpointPath); err != nil {
			c.logger.Warn("failed to save initial checkpoint", "error", err)
		}
		lastCheckpointSave = time.Now()
	}

	// Download files
	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, item nfc.FileItem) {
			defer wg.Done()
			defer func() { <-sem }()

			filePath := filepath.Join(outputDir, item.Path)

			// Check if file is already completed in checkpoint
			if checkpoint != nil {
				checkpointMux.Lock()
				fileProgress := checkpoint.GetFileProgress(item.Path)
				checkpointMux.Unlock()

				if fileProgress != nil && fileProgress.Status == "completed" {
					// Verify file exists and has correct size
					fileInfo, err := os.Stat(filePath)
					if err == nil && fileInfo.Size() == item.Size {
						c.logger.Info("skipping already completed file", "file", item.Path)

						// Store result
						resultsMux.Lock()
						results[idx] = filePath
						resultsMux.Unlock()

						// Update overall progress
						if overallBar != nil {
							overallBar.Add(1)
						}

						// Update total bytes downloaded for accurate progress
						atomic.AddInt64(&totalBytesDownloaded, item.Size)

						return
					}
					// File missing or size mismatch, re-download
					c.logger.Warn("file incomplete or missing, re-downloading", "file", item.Path)
				}
			}

			// Create directory if needed
			if err := os.MkdirAll(filepath.Dir(filePath), defaultDirPerm); err != nil {
				errCh <- fmt.Errorf("create directory for %s: %w", item.Path, err)
				return
			}

			// Get progress bar for this file (if individual bars are enabled)
			var fileBar progress.ProgressReporter
			if fileBars != nil && idx < len(fileBars) {
				fileBar = fileBars[idx]
			}

			// Wrap progress bar with callback if provided
			if progressCallback != nil {
				fileBar = &callbackProgressReporter{
					inner:      fileBar,
					callback:   progressCallback,
					totalBytes: &totalBytesDownloaded,
					totalSize:  totalSize,
					fileName:   filepath.Base(item.Path),
					fileIndex:  idx,
					totalFiles: len(items),
				}
			}

			// Download with retry
			bytes, err := c.downloadFileWithRetry(ctx, item.URL.String(), filePath, c.config.RetryAttempts, fileBar, bandwidthLimit, bandwidthBurst)
			if err != nil {
				// Update checkpoint with failed status
				if checkpoint != nil {
					checkpointMux.Lock()
					checkpoint.UpdateFileProgress(item.Path, 0, "failed")
					checkpointMux.Unlock()
				}
				errCh <- fmt.Errorf("download %s: %w", item.Path, err)
				return
			}

			// Store result
			resultsMux.Lock()
			results[idx] = filePath
			resultsMux.Unlock()

			// Update checkpoint with completed status
			if checkpoint != nil {
				checkpointMux.Lock()
				checkpoint.UpdateFileProgress(item.Path, bytes, "completed")

				// Save checkpoint periodically or after each file
				shouldSave := false
				if checkpointInterval == 0 {
					// Save after each file
					shouldSave = true
				} else if time.Since(lastCheckpointSave) >= checkpointInterval {
					// Save based on interval
					shouldSave = true
					lastCheckpointSave = time.Now()
				}

				if shouldSave {
					if err := checkpoint.Save(checkpointPath); err != nil {
						c.logger.Warn("failed to save checkpoint", "error", err)
					} else {
						c.logger.Debug("checkpoint saved", "progress", checkpoint.GetProgress())
					}
				}
				checkpointMux.Unlock()
			}

			// Update overall progress
			if overallBar != nil {
				overallBar.Add(1)
			}

			c.logger.Debug("file downloaded", "file", item.Path, "size", bytes)
		}(i, item)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("download errors: %s", strings.Join(errs, "; "))
	}

	// Filter out empty results
	finalResults := make([]string, 0, len(results))
	for _, result := range results {
		if result != "" {
			finalResults = append(finalResults, result)
		}
	}

	return finalResults, nil
}

func (c *VSphereClient) downloadFileWithRetry(
	ctx context.Context,
	urlStr, filePath string,
	maxRetries int,
	progressBar progress.ProgressReporter,
	bandwidthLimit int64,
	bandwidthBurst int,
) (int64, error) {
	fileName := filepath.Base(filePath)

	result, err := c.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt > 1 {
			c.logger.Info("retrying file download",
				"file", fileName,
				"attempt", attempt)

			if progressBar != nil {
				progressBar.SetTotal(0) // Reset progress bar for retry
			}
		}

		bytes, err := c.downloadFileResumable(ctx, urlStr, filePath, progressBar, bandwidthLimit, bandwidthBurst)
		if err != nil {
			// Check for non-retryable errors (file not found, permission denied, etc.)
			if strings.Contains(err.Error(), "404") ||
				strings.Contains(err.Error(), "403") ||
				strings.Contains(err.Error(), "not found") {
				return nil, retry.IsNonRetryable(err)
			}
			return nil, fmt.Errorf("download file %s: %w", fileName, err)
		}

		if progressBar != nil {
			progressBar.Finish()
		}

		return bytes, nil
	}, fmt.Sprintf("download %s", fileName))

	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

func (c *VSphereClient) downloadFileResumable(
	ctx context.Context,
	urlStr, filePath string,
	progressBar progress.ProgressReporter,
	bandwidthLimit int64,
	bandwidthBurst int,
) (int64, error) {
	// Parse URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return 0, fmt.Errorf("parse URL: %w", err)
	}

	// Check if file already exists for resume
	var startPos int64 = 0
	if fi, err := os.Stat(filePath); err == nil {
		startPos = fi.Size()
		c.logger.Debug("resuming download",
			"file", filepath.Base(filePath),
			"resumeFrom", startPos)

		if progressBar != nil {
			progressBar.Update(startPos)
		}
	}

	// Create HTTP request with range header for resume
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	if startPos > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startPos))
	}

	// Open file for writing (append if resuming)
	fileFlag := os.O_CREATE | os.O_WRONLY
	if startPos > 0 {
		fileFlag |= os.O_APPEND
	} else {
		fileFlag |= os.O_TRUNC
	}

	// #nosec G304 -- filePath is built from outputDir (operator/config-supplied) and a sanitized VM/disk name
	file, err := os.OpenFile(filePath, fileFlag, 0600)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Use SOAP client for download (handles auth automatically)
	// Must read the response body inside the callback
	var totalWritten int64
	var downloadErr error

	err = c.client.Do(ctx, req, func(res *http.Response) error {
		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
			return fmt.Errorf("HTTP error: %s", res.Status)
		}

		// Get total size from Content-Length or Content-Range
		var totalSize int64 = -1
		if cl := res.Header.Get("Content-Length"); cl != "" {
			if size, err := strconv.ParseInt(cl, 10, 64); err == nil {
				totalSize = size + startPos
			}
		} else if cr := res.Header.Get("Content-Range"); cr != "" {
			// Parse "bytes start-end/total" format
			parts := strings.Split(cr, "/")
			if len(parts) == 2 {
				if total, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					totalSize = total
				}
			}
		}

		// Set progress bar total if not set
		if progressBar != nil && totalSize > 0 {
			progressBar.SetTotal(totalSize)
		}

		// Create a proxy reader that updates progress
		var reader io.Reader = res.Body
		if progressBar != nil {
			reader = io.TeeReader(res.Body, &progressWriter{progressBar: progressBar})
		}

		// Apply bandwidth throttling if enabled
		if bandwidthLimit > 0 {
			reader = common.NewThrottledReaderWithContext(ctx, reader, bandwidthLimit, bandwidthBurst)
		}

		// Copy data
		written, err := io.Copy(file, reader)
		if err != nil {
			downloadErr = fmt.Errorf("copy data: %w", err)
			return downloadErr
		}

		totalWritten = startPos + written
		return nil
	})

	if err != nil {
		if downloadErr != nil {
			return totalWritten, downloadErr
		}
		return 0, fmt.Errorf("HTTP request: %w", err)
	}

	c.logger.Debug("download completed",
		"file", filepath.Base(filePath),
		"size", totalWritten)

	return totalWritten, nil
}

// generateArtifactManifest creates an Artifact Manifest v1.0 for the exported VM
func (c *VSphereClient) generateArtifactManifest(
	ctx context.Context,
	vm *object.VirtualMachine,
	result *ExportResult,
	opts ExportOptions,
) (string, error) {
	// Get VM properties for metadata
	moVM := vm.Reference()

	// Get VM runtime info
	vmProps, err := c.getVMProperties(ctx, vm)
	if err != nil {
		return "", fmt.Errorf("get VM properties: %w", err)
	}

	// Create manifest builder
	builder := manifest.NewBuilder()

	// Set source metadata
	// Get datacenter name from vCenter URL
	datacenter := "vsphere"
	if c.config.VCenterURL != "" {
		datacenter = c.config.VCenterURL
	}

	builder.WithSource(
		"vsphere",       // provider
		moVM.Value,      // VM ID (MoRef)
		vm.Name(),       // VM name
		datacenter,      // datacenter
		"transiva-govc", // export method
	)

	// Set VM hardware metadata
	firmware := "bios"
	if vmProps.Firmware != "" {
		firmware = vmProps.Firmware
	}

	osHint := "unknown"
	osVersion := vmProps.GuestOS
	if strings.Contains(strings.ToLower(vmProps.GuestOS), "linux") {
		osHint = "linux"
	} else if strings.Contains(strings.ToLower(vmProps.GuestOS), "windows") {
		osHint = "windows"
	}

	builder.WithVM(
		int(vmProps.NumCPU),   // CPUs
		int(vmProps.MemoryGB), // memory GB
		firmware,              // firmware
		osHint,                // OS hint
		osVersion,             // OS version
		false,                 // secure boot (unknown)
	)

	// Add disk artifacts
	for i, diskFile := range result.Files {
		// Only add VMDK files (skip OVF descriptor and manifest files)
		if !strings.HasSuffix(strings.ToLower(diskFile), ".vmdk") {
			continue
		}

		diskID := fmt.Sprintf("disk-%d", i)
		diskType := "data"
		if i == 0 {
			diskType = "boot"
		}

		// Get file size
		fileInfo, err := os.Stat(diskFile)
		if err != nil {
			c.logger.Warn("failed to stat disk file", "file", diskFile, "error", err)
			continue
		}

		// Add disk with optional checksum
		if opts.ManifestComputeChecksum {
			c.logger.Debug("computing checksum for disk", "disk", diskFile)
			builder.AddDiskWithChecksum(
				diskID,
				"vmdk",
				diskFile,
				fileInfo.Size(),
				i, // boot_order_hint
				diskType,
				true, // compute checksum
			)
		} else {
			builder.AddDisk(
				diskID,
				"vmdk",
				diskFile,
				fileInfo.Size(),
				i, // boot_order_hint
				diskType,
			)
		}
	}

	// Add notes
	builder.AddNote(fmt.Sprintf("Exported from vSphere by transiva v%s", "0.1.0"))
	builder.AddNote(fmt.Sprintf("Export method: %s", opts.Format))
	if opts.Compress {
		builder.AddNote("Export compressed with gzip")
	}

	// Configure transiva metadata
	builder.WithMetadata(
		"0.1.0",    // transiva version
		moVM.Value, // job ID (use VM ID)
		map[string]string{
			"provider":      "vsphere",
			"export_format": opts.Format,
			"vcenter_url":   c.config.VCenterURL,
		},
	)

	// Configure hyper2kvm pipeline with user settings
	builder.WithPipeline(
		opts.PipelineInspect,  // inspect
		opts.PipelineFix,      // fix
		opts.PipelineConvert,  // convert
		opts.PipelineValidate, // validate
	)

	enableRDP := strings.EqualFold(osHint, "windows")
	if opts.EnableRDP != nil {
		enableRDP = *opts.EnableRDP
	}
	builder.WithFixEnableRDP(enableRDP)
	c.logger.Info("manifest: enable_rdp for hyper2kvm firstboot", "enable_rdp", enableRDP, "os_hint", osHint)

	// Set pipeline options if enabled
	if opts.EnablePipeline {
		builder.WithOptions(
			opts.PipelineDryRun, // dry run
			1,                   // verbose level (normal)
		)
	}

	// Set target format for conversion
	targetFormat := opts.ManifestTargetFormat
	if targetFormat == "" {
		targetFormat = "qcow2" // default
	}

	builder.WithOutput(
		result.OutputDir, // output directory
		targetFormat,     // target format
		"",               // filename (auto-generated)
	)

	// Build manifest
	m, err := builder.Build()
	if err != nil {
		return "", fmt.Errorf("build manifest: %w", err)
	}

	// Write manifest to file
	manifestPath := filepath.Join(result.OutputDir, "artifact-manifest.json")
	if err := manifest.WriteToFile(m, manifestPath); err != nil {
		return "", fmt.Errorf("write manifest: %w", err)
	}

	return manifestPath, nil
}

// verifyManifest validates the generated manifest
func (c *VSphereClient) verifyManifest(manifestPath string) error {
	// Load manifest
	m, err := manifest.ReadFromFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	// Validate manifest
	if err := manifest.Validate(m); err != nil {
		return fmt.Errorf("validate manifest: %w", err)
	}

	// Verify checksums if present
	if len(m.Disks) > 0 && m.Disks[0].Checksum != "" {
		c.logger.Info("verifying disk checksums")
		results, err := manifest.VerifyChecksums(m)
		if err != nil {
			return fmt.Errorf("verify checksums: %w", err)
		}

		for diskID, valid := range results {
			if !valid {
				return fmt.Errorf("checksum verification failed for disk: %s", diskID)
			}
			c.logger.Debug("checksum verified", "disk", diskID)
		}
	}

	return nil
}

// vmProperties holds properties needed for manifest generation
type vmProperties struct {
	NumCPU   int32
	MemoryGB int32
	GuestOS  string
	Firmware string
}

// getVMProperties retrieves VM properties needed for manifest generation
func (c *VSphereClient) getVMProperties(ctx context.Context, vm *object.VirtualMachine) (*vmProperties, error) {
	moVM := vm.Reference()

	// Get VM configuration
	var obj struct {
		Config types.VirtualMachineConfigInfo `mo:"config"`
	}

	err := c.client.RetrieveOne(ctx, moVM, []string{"config"}, &obj)
	if err != nil {
		return nil, fmt.Errorf("retrieve VM config: %w", err)
	}

	props := &vmProperties{
		NumCPU:   obj.Config.Hardware.NumCPU,
		MemoryGB: int32(obj.Config.Hardware.MemoryMB / 1024),
		GuestOS:  obj.Config.GuestId,
		Firmware: "bios",
	}

	// Determine firmware type
	if obj.Config.Firmware != "" {
		props.Firmware = strings.ToLower(obj.Config.Firmware)
	}

	return props, nil
}
