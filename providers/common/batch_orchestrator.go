// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// BatchMigrationConfig holds configuration for batch VM migration
type BatchMigrationConfig struct {
	// List of VMs to migrate
	VMs []*VMMigrationTask `json:"vms"`

	// Parallel migrations
	MaxParallel int `json:"max_parallel,omitempty"`

	// Output directory
	OutputDir string `json:"output_dir"`

	// Conversion options
	ConvertOptions ConvertOptions `json:"convert_options,omitempty"`

	// Pipeline configuration
	PipelineConfig *PipelineConfig `json:"pipeline_config,omitempty"`

	// Guest configuration
	GuestConfig *GuestConfig `json:"guest_config,omitempty"`

	// Cloud storage upload
	UploadToCloud bool                `json:"upload_to_cloud,omitempty"`
	CloudStorage  *CloudStorageConfig `json:"cloud_storage,omitempty"`

	// Retry configuration
	MaxRetries int           `json:"max_retries,omitempty"`
	RetryDelay time.Duration `json:"retry_delay,omitempty"`

	// Continue on error
	ContinueOnError bool `json:"continue_on_error,omitempty"`
}

// VMMigrationTask represents a single VM migration task
type VMMigrationTask struct {
	// VM identifier
	ID string `json:"id"`

	// Provider-specific VM name/ID
	Name string `json:"name"`

	// Provider (vsphere, aws, azure, gcp)
	Provider string `json:"provider"`

	// Priority (higher priority VMs migrated first)
	Priority int `json:"priority,omitempty"`

	// Custom output directory (overrides global output_dir)
	OutputDir string `json:"output_dir,omitempty"`

	// Custom pipeline config (overrides global pipeline_config)
	PipelineConfig *PipelineConfig `json:"pipeline_config,omitempty"`

	// Custom guest config (overrides global guest_config)
	GuestConfig *GuestConfig `json:"guest_config,omitempty"`

	// Custom metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// VMMigrationResult represents the result of a VM migration
type VMMigrationResult struct {
	Task               *VMMigrationTask
	Success            bool
	Error              string
	ExportDuration     time.Duration
	ConversionDuration time.Duration
	TotalDuration      time.Duration
	ConvertedFiles     []string
	UploadResults      []*UploadResult
	Metadata           map[string]interface{}
}

// VMExporter defines interface for exporting VMs from different providers
type VMExporter interface {
	// ExportVM exports a VM and returns the manifest path
	ExportVM(ctx context.Context, vmName, provider, outputDir string) (manifestPath string, exportDuration time.Duration, err error)
}

// BatchOrchestrator orchestrates batch VM migrations
type BatchOrchestrator struct {
	config    *BatchMigrationConfig
	logger    logger.Logger
	converter Converter
	uploader  CloudStorageUploader
	exporter  VMExporter
}

// NewBatchOrchestrator creates a new batch orchestrator
func NewBatchOrchestrator(config *BatchMigrationConfig, log logger.Logger) (*BatchOrchestrator, error) {
	if config == nil {
		return nil, fmt.Errorf("batch migration config is required")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Set defaults
	if config.MaxParallel <= 0 {
		config.MaxParallel = 1
	}

	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}

	if config.RetryDelay == 0 {
		config.RetryDelay = 30 * time.Second
	}

	return &BatchOrchestrator{
		config: config,
		logger: log,
	}, nil
}

// SetConverter sets the converter for the orchestrator
func (bo *BatchOrchestrator) SetConverter(converter Converter) {
	bo.converter = converter
}

// SetUploader sets the cloud storage uploader
func (bo *BatchOrchestrator) SetUploader(uploader CloudStorageUploader) {
	bo.uploader = uploader
}

// SetExporter sets the VM exporter
func (bo *BatchOrchestrator) SetExporter(exporter VMExporter) {
	bo.exporter = exporter
}

// Execute executes batch VM migration
func (bo *BatchOrchestrator) Execute(ctx context.Context) ([]*VMMigrationResult, error) {
	bo.logger.Info("starting batch VM migration",
		"total_vms", len(bo.config.VMs),
		"max_parallel", bo.config.MaxParallel)

	// Sort VMs by priority (higher priority first)
	sortedVMs := bo.sortVMsByPriority()

	// Create semaphore for limiting parallelism
	sem := make(chan struct{}, bo.config.MaxParallel)

	// Results channel
	results := make(chan *VMMigrationResult, len(sortedVMs))

	// Wait group
	var wg sync.WaitGroup

	// Launch migration tasks
	for _, task := range sortedVMs {
		wg.Add(1)
		go func(t *VMMigrationTask) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Migrate VM
			result := bo.migrateVM(ctx, t)
			results <- result
		}(task)
	}

	// Wait for all tasks to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []*VMMigrationResult
	for result := range results {
		allResults = append(allResults, result)

		if result.Success {
			bo.logger.Info("VM migration completed",
				"vm_id", result.Task.ID,
				"duration", result.TotalDuration,
				"files", len(result.ConvertedFiles))
		} else {
			bo.logger.Error("VM migration failed",
				"vm_id", result.Task.ID,
				"error", result.Error)

			// Stop if continue_on_error is false
			if !bo.config.ContinueOnError {
				return allResults, fmt.Errorf("VM migration failed: %s", result.Error)
			}
		}
	}

	// Generate summary
	bo.logSummary(allResults)

	return allResults, nil
}

// migrateVM migrates a single VM
func (bo *BatchOrchestrator) migrateVM(ctx context.Context, task *VMMigrationTask) *VMMigrationResult {
	startTime := time.Now()

	bo.logger.Info("starting VM migration", "vm_id", task.ID, "provider", task.Provider)

	result := &VMMigrationResult{
		Task:     task,
		Metadata: make(map[string]interface{}),
	}

	// Retry logic
	var err error
	for attempt := 0; attempt <= bo.config.MaxRetries; attempt++ {
		if attempt > 0 {
			bo.logger.Info("retrying VM migration",
				"vm_id", task.ID,
				"attempt", attempt+1,
				"max_retries", bo.config.MaxRetries)
			time.Sleep(bo.config.RetryDelay)
		}

		err = bo.executeMigration(ctx, task, result)
		if err == nil {
			result.Success = true
			result.TotalDuration = time.Since(startTime)
			return result
		}

		bo.logger.Warn("VM migration attempt failed",
			"vm_id", task.ID,
			"attempt", attempt+1,
			"error", err)
	}

	// All attempts failed
	result.Success = false
	result.Error = err.Error()
	result.TotalDuration = time.Since(startTime)
	return result
}

// executeMigration executes the actual migration
func (bo *BatchOrchestrator) executeMigration(ctx context.Context, task *VMMigrationTask, result *VMMigrationResult) error {
	// Determine output directory
	outputDir := task.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(bo.config.OutputDir, task.ID)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Export VM (provider-specific)
	var manifestPath string
	var exportDuration time.Duration

	if bo.exporter != nil {
		// Use configured exporter to export the VM
		bo.logger.Info("exporting VM",
			"vm_id", task.ID,
			"provider", task.Provider,
			"output_dir", outputDir)

		var err error
		manifestPath, exportDuration, err = bo.exporter.ExportVM(ctx, task.Name, task.Provider, outputDir)
		if err != nil {
			return fmt.Errorf("export VM: %w", err)
		}

		result.ExportDuration = exportDuration
		bo.logger.Info("VM export completed",
			"vm_id", task.ID,
			"duration", exportDuration,
			"manifest", manifestPath)
	} else {
		// No exporter configured - assume manifest already exists
		manifestPath = filepath.Join(outputDir, "artifact-manifest.json")
		bo.logger.Warn("no VM exporter configured, assuming manifest exists",
			"vm_id", task.ID,
			"manifest", manifestPath)

		// Verify manifest exists
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			return fmt.Errorf("no exporter configured and manifest not found at %s", manifestPath)
		}
	}

	// Convert VM
	if bo.converter != nil {
		convStartTime := time.Now()

		opts := bo.config.ConvertOptions
		if task.PipelineConfig != nil {
			// Apply custom pipeline config
			// This would be passed to the converter
		}

		convResult, err := bo.converter.Convert(ctx, manifestPath, opts)
		if err != nil {
			return fmt.Errorf("conversion failed: %w", err)
		}

		result.ConversionDuration = time.Since(convStartTime)
		result.ConvertedFiles = convResult.ConvertedFiles
	}

	// Upload to cloud storage
	if bo.config.UploadToCloud && bo.uploader != nil {
		csm := &CloudStorageManager{
			config:   bo.config.CloudStorage,
			uploader: bo.uploader,
		}

		convResult := &ConversionResult{
			ConvertedFiles: result.ConvertedFiles,
		}

		uploadResults, err := csm.UploadConvertedImages(ctx, convResult, task.ID)
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		result.UploadResults = uploadResults
	}

	return nil
}

// sortVMsByPriority sorts VMs by priority (higher priority first)
func (bo *BatchOrchestrator) sortVMsByPriority() []*VMMigrationTask {
	vms := make([]*VMMigrationTask, len(bo.config.VMs))
	copy(vms, bo.config.VMs)

	// Simple bubble sort (sufficient for small lists)
	for i := 0; i < len(vms); i++ {
		for j := i + 1; j < len(vms); j++ {
			if vms[j].Priority > vms[i].Priority {
				vms[i], vms[j] = vms[j], vms[i]
			}
		}
	}

	return vms
}

// logSummary logs migration summary
func (bo *BatchOrchestrator) logSummary(results []*VMMigrationResult) {
	var successful, failed int
	var totalDuration time.Duration

	for _, result := range results {
		if result.Success {
			successful++
		} else {
			failed++
		}
		totalDuration += result.TotalDuration
	}

	avgDuration := time.Duration(0)
	if len(results) > 0 {
		avgDuration = totalDuration / time.Duration(len(results))
	}

	bo.logger.Info("batch migration summary",
		"total", len(results),
		"successful", successful,
		"failed", failed,
		"total_duration", totalDuration,
		"avg_duration", avgDuration)
}

// Validate validates the batch migration configuration
func (cfg *BatchMigrationConfig) Validate() error {
	if len(cfg.VMs) == 0 {
		return fmt.Errorf("no VMs to migrate")
	}

	if cfg.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}

	// Validate each VM task
	for i, vm := range cfg.VMs {
		if vm.ID == "" {
			return fmt.Errorf("VM %d: ID is required", i)
		}
		if vm.Name == "" {
			return fmt.Errorf("VM %d: name is required", i)
		}
		if vm.Provider == "" {
			return fmt.Errorf("VM %d: provider is required", i)
		}
	}

	// Validate cloud storage config if upload is enabled
	if cfg.UploadToCloud && cfg.CloudStorage != nil {
		if err := cfg.CloudStorage.Validate(); err != nil {
			return fmt.Errorf("cloud storage config: %w", err)
		}
	}

	return nil
}

// SaveResults saves batch migration results to a file
func SaveResults(results []*VMMigrationResult, path string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write results file: %w", err)
	}

	return nil
}

// LoadBatchConfig loads batch migration configuration from a file
func LoadBatchConfig(path string) (*BatchMigrationConfig, error) {
	// #nosec G304 -- path is supplied by the local operator invoking batch migration (CLI/config), not remote input
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read batch config: %w", err)
	}

	var config BatchMigrationConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse batch config: %w", err)
	}

	return &config, nil
}
