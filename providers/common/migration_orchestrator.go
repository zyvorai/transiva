// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// MigrationOrchestrator coordinates the complete end-to-end migration workflow
// integrating all Phase 0-5 components
type MigrationOrchestrator struct {
	// Phase 3: Conversion
	conversionManager *ConversionManager

	// Phase 4: Advanced features
	parallelConverter *ParallelConverter
	cloudStorage      *CloudStorageManager
	batchOrchestrator *BatchOrchestrator

	// Phase 5: Monitoring & Reporting
	progressTracker  *ProgressTracker
	metricsCollector *MetricsCollector
	auditLogger      *AuditLogger
	webhookManager   *WebhookManager

	logger logger.Logger
}

// MigrationConfig holds configuration for the complete migration
type MigrationConfig struct {
	// Basic config
	VMName       string
	Provider     string
	OutputDir    string
	TargetFormat string // qcow2, raw, vdi

	// Export options
	ExportManifest bool
	VerifyExport   bool

	// Phase 3: Conversion options
	EnableConversion bool
	ConvertOptions   ConvertOptions

	// Phase 4: Advanced features
	ParallelDisks    bool
	MaxParallelDisks int
	UploadToCloud    bool
	CloudDestination string

	// Phase 5: Monitoring
	EnableProgress     bool
	EnableMetrics      bool
	EnableAuditLogging bool
	EnableWebhooks     bool
	WebhookConfigs     []*WebhookConfig
	ProgressAPIPort    int
	MetricsAPIPort     int

	// User context
	User      string
	IPAddress string
}

// MigrationResult holds the results of a complete migration
type MigrationResult struct {
	TaskID   string
	VMName   string
	Provider string
	Success  bool
	Error    string

	// Export results
	ExportDuration time.Duration
	ExportedFiles  []string
	ExportSize     int64
	ManifestPath   string

	// Conversion results
	ConversionDuration time.Duration
	ConvertedFiles     []string
	ConversionSize     int64
	ConversionReport   string

	// Upload results
	UploadDuration   time.Duration
	UploadedFiles    []string
	CloudDestination string

	// Overall results
	TotalDuration time.Duration
	TotalSize     int64
	StartTime     time.Time
	EndTime       time.Time
}

// NewMigrationOrchestrator creates a new migration orchestrator
func NewMigrationOrchestrator(config *OrchestratorConfig, log logger.Logger) (*MigrationOrchestrator, error) {
	mo := &MigrationOrchestrator{
		logger: log,
	}

	// Initialize Phase 3: Conversion Manager
	if config.EnableConversion {
		convMgr, err := NewConversionManager(&ConverterConfig{}, log)
		if err != nil {
			return nil, fmt.Errorf("create conversion manager: %w", err)
		}
		mo.conversionManager = convMgr
	}

	// Initialize Phase 4: Advanced features
	if config.EnableParallelConversion {
		// ParallelConverter requires a base converter, use nil for now
		mo.parallelConverter = NewParallelConverter(nil, 4, log)
	}

	if config.EnableBatchOrchestration {
		batchOrch, err := NewBatchOrchestrator(&BatchMigrationConfig{}, log)
		if err != nil {
			return nil, fmt.Errorf("create batch orchestrator: %w", err)
		}
		mo.batchOrchestrator = batchOrch
	}

	// Initialize Phase 5: Monitoring & Reporting
	if config.EnableProgress {
		mo.progressTracker = NewProgressTracker()
	}

	if config.EnableMetrics {
		mo.metricsCollector = NewMetricsCollector()
	}

	if config.EnableAuditLogging && config.AuditLogPath != "" {
		auditLogger, err := NewAuditLogger(config.AuditLogPath)
		if err != nil {
			return nil, fmt.Errorf("create audit logger: %w", err)
		}
		mo.auditLogger = auditLogger
	}

	if config.EnableWebhooks && len(config.WebhookConfigs) > 0 {
		mo.webhookManager = NewWebhookManager(config.WebhookConfigs, log)
	}

	return mo, nil
}

// OrchestratorConfig holds configuration for the orchestrator
type OrchestratorConfig struct {
	// Phase 3
	EnableConversion bool

	// Phase 4
	EnableParallelConversion bool
	EnableCloudStorage       bool
	EnableBatchOrchestration bool

	// Phase 5
	EnableProgress     bool
	EnableMetrics      bool
	EnableAuditLogging bool
	EnableWebhooks     bool
	AuditLogPath       string
	WebhookConfigs     []*WebhookConfig
}

// Migrate performs a complete end-to-end migration
func (mo *MigrationOrchestrator) Migrate(ctx context.Context, config *MigrationConfig) (*MigrationResult, error) {
	startTime := time.Now()
	taskID := generateTaskID()

	result := &MigrationResult{
		TaskID:    taskID,
		VMName:    config.VMName,
		Provider:  config.Provider,
		StartTime: startTime,
	}

	mo.logger.Info("starting migration orchestration",
		"task_id", taskID,
		"vm", config.VMName,
		"provider", config.Provider)

	// Phase 5: Start progress tracking
	if mo.progressTracker != nil {
		mo.progressTracker.StartTask(taskID, config.VMName, config.Provider)
	}

	// Phase 5: Record migration start
	if mo.metricsCollector != nil {
		mo.metricsCollector.RecordMigrationStart(config.Provider)
	}

	if mo.auditLogger != nil {
		if err := mo.auditLogger.LogMigrationStart(taskID, config.VMName, config.Provider, config.User); err != nil {
			mo.logger.Warn("failed to log migration start", "task_id", taskID, "error", err)
		}
	}

	if mo.webhookManager != nil {
		mo.webhookManager.NotifyStart(taskID, config.VMName, config.Provider)
	}

	// Phase 1: Export with manifest (handled by caller)
	// This returns ExportedFiles, ManifestPath, etc.

	// Phase 3: Conversion (if enabled)
	if config.EnableConversion && mo.conversionManager != nil {
		mo.logger.Info("starting conversion phase", "task_id", taskID)

		if mo.progressTracker != nil {
			if err := mo.progressTracker.SetStatus(taskID, StatusConverting); err != nil {
				mo.logger.Warn("failed to update progress status", "task_id", taskID, "error", err)
			}
		}

		if mo.auditLogger != nil {
			if err := mo.auditLogger.LogConversionStart(taskID, config.VMName); err != nil {
				mo.logger.Warn("failed to log conversion start", "task_id", taskID, "error", err)
			}
		}

		convStart := time.Now()

		// Load manifest
		manifestPath := config.OutputDir + "/manifest.json"

		// Phase 3: Sequential conversion using conversion manager
		convResult, err := mo.conversionManager.Convert(ctx, manifestPath, config.ConvertOptions)
		if err != nil {
			return mo.handleFailure(taskID, config, result, fmt.Errorf("conversion: %w", err))
		}

		convertedFiles := convResult.ConvertedFiles
		// Calculate size from converted files
		var conversionSize int64
		for _, file := range convertedFiles {
			if fi, err := os.Stat(file); err == nil {
				conversionSize += fi.Size()
			}
		}

		convDuration := time.Since(convStart)
		result.ConversionDuration = convDuration
		result.ConvertedFiles = convertedFiles
		result.ConversionSize = conversionSize

		if mo.auditLogger != nil {
			if err := mo.auditLogger.LogConversionComplete(taskID, config.VMName, convDuration, convertedFiles); err != nil {
				mo.logger.Warn("failed to log conversion complete", "task_id", taskID, "error", err)
			}
		}

		mo.logger.Info("conversion completed",
			"task_id", taskID,
			"duration", convDuration,
			"files", len(convertedFiles))
	}

	// Phase 4: Cloud upload (if enabled)
	if config.UploadToCloud && config.CloudDestination != "" && mo.cloudStorage != nil {
		mo.logger.Info("starting cloud upload phase",
			"task_id", taskID,
			"destination", config.CloudDestination)

		if mo.progressTracker != nil {
			if err := mo.progressTracker.SetStatus(taskID, StatusUploading); err != nil {
				mo.logger.Warn("failed to update progress status", "task_id", taskID, "error", err)
			}
		}

		if mo.auditLogger != nil {
			if err := mo.auditLogger.LogUploadStart(taskID, config.VMName, config.CloudDestination); err != nil {
				mo.logger.Warn("failed to log upload start", "task_id", taskID, "error", err)
			}
		}

		uploadStart := time.Now()

		// Determine which files to upload:
		// If conversion was performed, upload converted files
		// Otherwise, upload exported files
		var filesToUpload []string
		if len(result.ConvertedFiles) > 0 {
			filesToUpload = result.ConvertedFiles
		} else if len(result.ExportedFiles) > 0 {
			filesToUpload = result.ExportedFiles
		}

		if len(filesToUpload) == 0 {
			mo.logger.Warn("no files to upload", "task_id", taskID)
		} else {
			// Create conversion result for upload
			convResult := &ConversionResult{
				ConvertedFiles: filesToUpload,
			}

			// Upload files
			uploadResults, err := mo.cloudStorage.UploadConvertedImages(ctx, convResult, config.CloudDestination)
			if err != nil {
				return mo.handleFailure(taskID, config, result, fmt.Errorf("cloud upload: %w", err))
			}

			// Track upload results
			uploadDuration := time.Since(uploadStart)
			result.UploadDuration = uploadDuration
			result.CloudDestination = config.CloudDestination

			// Extract uploaded file paths
			uploadedFiles := make([]string, len(uploadResults))
			for i, ur := range uploadResults {
				uploadedFiles[i] = ur.RemotePath
			}
			result.UploadedFiles = uploadedFiles

			// Calculate total bytes uploaded
			var bytesUploaded int64
			for _, ur := range uploadResults {
				bytesUploaded += ur.Size
			}

			if mo.auditLogger != nil {
				if err := mo.auditLogger.LogUploadComplete(taskID, config.VMName, config.CloudDestination, uploadDuration, bytesUploaded); err != nil {
					mo.logger.Warn("failed to log upload complete", "task_id", taskID, "error", err)
				}
			}

			mo.logger.Info("cloud upload completed",
				"task_id", taskID,
				"duration", uploadDuration,
				"files", len(uploadedFiles))
		}
	}

	// Complete migration
	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)

	result.Success = true
	result.EndTime = endTime
	result.TotalDuration = totalDuration
	result.TotalSize = result.ExportSize + result.ConversionSize

	// Phase 5: Complete tracking
	if mo.progressTracker != nil {
		if err := mo.progressTracker.CompleteTask(taskID); err != nil {
			mo.logger.Warn("failed to complete progress task", "task_id", taskID, "error", err)
		}
	}

	if mo.metricsCollector != nil {
		mo.metricsCollector.RecordMigrationSuccess(
			config.Provider,
			result.ExportDuration,
			result.ConversionDuration,
			result.UploadDuration,
			result.ExportSize,
			result.ConversionSize,
			result.ConversionSize,
		)
	}

	if mo.auditLogger != nil {
		if err := mo.auditLogger.LogMigrationComplete(
			taskID,
			config.VMName,
			config.Provider,
			config.User,
			totalDuration,
			map[string]interface{}{
				"exported_files":  len(result.ExportedFiles),
				"converted_files": len(result.ConvertedFiles),
				"uploaded_files":  len(result.UploadedFiles),
				"total_size":      result.TotalSize,
			},
		); err != nil {
			mo.logger.Warn("failed to log migration complete", "task_id", taskID, "error", err)
		}
	}

	if mo.webhookManager != nil {
		mo.webhookManager.NotifyComplete(taskID, config.VMName, config.Provider, totalDuration)
	}

	mo.logger.Info("migration orchestration completed",
		"task_id", taskID,
		"total_duration", totalDuration,
		"success", true)

	return result, nil
}

// handleFailure handles migration failures
func (mo *MigrationOrchestrator) handleFailure(
	taskID string,
	config *MigrationConfig,
	result *MigrationResult,
	err error,
) (*MigrationResult, error) {
	result.Success = false
	result.Error = err.Error()
	result.EndTime = time.Now()
	result.TotalDuration = result.EndTime.Sub(result.StartTime)

	mo.logger.Error("migration orchestration failed",
		"task_id", taskID,
		"error", err)

	// Phase 5: Record failure
	if mo.progressTracker != nil {
		if failErr := mo.progressTracker.FailTask(taskID, err); failErr != nil {
			mo.logger.Warn("failed to record failed progress task", "task_id", taskID, "error", failErr)
		}
	}

	if mo.metricsCollector != nil {
		mo.metricsCollector.RecordMigrationFailure(config.Provider)
	}

	if mo.auditLogger != nil {
		if logErr := mo.auditLogger.LogMigrationFailed(taskID, config.VMName, config.Provider, config.User, err); logErr != nil {
			mo.logger.Warn("failed to log migration failed", "task_id", taskID, "error", logErr)
		}
	}

	if mo.webhookManager != nil {
		mo.webhookManager.NotifyError(taskID, config.VMName, config.Provider, err)
	}

	return result, err
}

// MigrateBatch performs batch migration using Phase 4 BatchOrchestrator
func (mo *MigrationOrchestrator) MigrateBatch(
	ctx context.Context,
	configs []*MigrationConfig,
) ([]*MigrationResult, error) {
	if mo.batchOrchestrator == nil {
		return nil, fmt.Errorf("batch orchestration not enabled")
	}

	mo.logger.Info("starting batch migration", "count", len(configs))

	// This would coordinate with BatchOrchestrator from Phase 4
	// For now, we'll do sequential migrations
	results := make([]*MigrationResult, 0, len(configs))

	for i, config := range configs {
		mo.logger.Info("batch migration progress",
			"current", i+1,
			"total", len(configs),
			"vm", config.VMName)

		result, err := mo.Migrate(ctx, config)
		if err != nil {
			mo.logger.Error("batch migration item failed",
				"vm", config.VMName,
				"error", err)
		}

		results = append(results, result)
	}

	return results, nil
}

// GetProgressTracker returns the progress tracker
func (mo *MigrationOrchestrator) GetProgressTracker() *ProgressTracker {
	return mo.progressTracker
}

// GetMetricsCollector returns the metrics collector
func (mo *MigrationOrchestrator) GetMetricsCollector() *MetricsCollector {
	return mo.metricsCollector
}

// GetAuditLogger returns the audit logger
func (mo *MigrationOrchestrator) GetAuditLogger() *AuditLogger {
	return mo.auditLogger
}

// Close closes all resources
func (mo *MigrationOrchestrator) Close() error {
	if mo.auditLogger != nil {
		return mo.auditLogger.Close()
	}
	return nil
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}
