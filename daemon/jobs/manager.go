// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/daemon/capabilities"
	"github.com/zyvorai/transiva/daemon/exporters"
	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

const (
	// Shutdown timeout for graceful termination of running jobs
	shutdownTimeout = 30 * time.Second
)

// Manager handles job lifecycle and execution
type Manager struct {
	jobs            map[string]*models.Job
	mu              sync.RWMutex
	logger          logger.Logger
	startTime       time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup // Track running goroutines
	exporterFactory *exporters.ExporterFactory
	webhookManager  WebhookManager // Interface for webhooks (can be nil)
}

// WebhookManager defines the interface for webhook notifications
type WebhookManager interface {
	SendJobCreated(job *models.Job)
	SendJobStarted(job *models.Job)
	SendJobCompleted(job *models.Job)
	SendJobFailed(job *models.Job)
	SendJobCancelled(job *models.Job)
	SendJobProgress(job *models.Job)
}

// NewManager creates a new job manager
func NewManager(log logger.Logger, detector *capabilities.Detector) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	factory := exporters.NewExporterFactory(detector, log)
	return &Manager{
		jobs:            make(map[string]*models.Job),
		logger:          log,
		startTime:       time.Now(),
		ctx:             ctx,
		cancel:          cancel,
		exporterFactory: factory,
		webhookManager:  nil, // Set later via SetWebhookManager if needed
	}
}

// SetWebhookManager sets the webhook manager for job event notifications
func (m *Manager) SetWebhookManager(webhookMgr WebhookManager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhookManager = webhookMgr
	if webhookMgr != nil {
		m.logger.Info("webhook manager configured")
	}
}

// SetProviderContext wires provider registry for Nutanix async exports.
func (m *Manager) SetProviderContext(registry *providers.Registry, appConfig *config.Config) {
	if m.exporterFactory != nil {
		m.exporterFactory.SetProviderContext(registry, appConfig)
	}
}

// SubmitJob submits a new job for execution
func (m *Manager) SubmitJob(def models.JobDefinition) (string, error) {
	m.mu.Lock()

	// Generate ID if not provided
	if def.ID == "" {
		def.ID = uuid.New().String()
	}

	// Check if job already exists
	if _, exists := m.jobs[def.ID]; exists {
		m.mu.Unlock()
		return "", fmt.Errorf("job with ID %s already exists", def.ID)
	}

	// Create job
	now := time.Now()
	def.CreatedAt = now

	job := &models.Job{
		Definition: def,
		Status:     models.JobStatusPending,
		UpdatedAt:  now,
	}

	m.jobs[def.ID] = job
	m.logger.Info("job submitted", "id", def.ID, "name", def.Name, "vm", def.VMPath)

	// Send webhook notification for job creation
	if m.webhookManager != nil {
		m.webhookManager.SendJobCreated(job)
	}

	// Unlock before starting goroutine to avoid holding lock during execution
	m.mu.Unlock()

	// Start job in goroutine with WaitGroup tracking
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.executeJob(def.ID)
	}()

	return def.ID, nil
}

// SubmitBatch submits multiple jobs
func (m *Manager) SubmitBatch(defs []models.JobDefinition) ([]string, []error) {
	ids := make([]string, 0, len(defs))
	errs := make([]error, 0)

	for _, def := range defs {
		id, err := m.SubmitJob(def)
		if err != nil {
			errs = append(errs, err)
		} else {
			ids = append(ids, id)
		}
	}

	return ids, errs
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(id string) (*models.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	// Deep copy to avoid race conditions with pointer fields
	jobCopy := *job

	// Deep copy Progress if present
	if job.Progress != nil {
		progressCopy := *job.Progress
		jobCopy.Progress = &progressCopy
	}

	// Deep copy Result if present
	if job.Result != nil {
		resultCopy := *job.Result
		// Copy slice fields
		if len(job.Result.Files) > 0 {
			resultCopy.Files = make([]string, len(job.Result.Files))
			copy(resultCopy.Files, job.Result.Files)
		}
		jobCopy.Result = &resultCopy
	}

	return &jobCopy, nil
}

// ListJobs returns jobs matching the criteria
func (m *Manager) ListJobs(statuses []models.JobStatus, limit int) []*models.Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.Job

	for _, job := range m.jobs {
		// Filter by status if specified
		if len(statuses) > 0 {
			matched := false
			for _, status := range statuses {
				if job.Status == status {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Deep copy job to avoid race conditions
		jobCopy := *job

		// Deep copy Progress if present
		if job.Progress != nil {
			progressCopy := *job.Progress
			jobCopy.Progress = &progressCopy
		}

		// Deep copy Result if present
		if job.Result != nil {
			resultCopy := *job.Result
			if len(job.Result.Files) > 0 {
				resultCopy.Files = make([]string, len(job.Result.Files))
				copy(resultCopy.Files, job.Result.Files)
			}
			jobCopy.Result = &resultCopy
		}

		result = append(result, &jobCopy)

		// Apply limit
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// GetAllJobs returns all jobs
func (m *Manager) GetAllJobs() []*models.Job {
	return m.ListJobs(nil, 0)
}

// CancelJob cancels a running job
func (m *Manager) CancelJob(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	if job.Status != models.JobStatusRunning && job.Status != models.JobStatusPending {
		return fmt.Errorf("job %s cannot be cancelled (status: %s)", id, job.Status)
	}

	job.Status = models.JobStatusCancelled
	now := time.Now()
	job.CompletedAt = &now
	job.UpdatedAt = now

	m.logger.Info("job cancelled", "id", id)

	// Send webhook notification for job cancellation
	if m.webhookManager != nil {
		m.webhookManager.SendJobCancelled(job)
	}

	return nil
}

// GetStatus returns daemon status
func (m *Manager) GetStatus() *models.DaemonStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var running, completed, failed, cancelled int
	for _, job := range m.jobs {
		switch job.Status {
		case models.JobStatusRunning:
			running++
		case models.JobStatusCompleted:
			completed++
		case models.JobStatusFailed:
			failed++
		case models.JobStatusCancelled:
			cancelled++
		}
	}

	return &models.DaemonStatus{
		Version:        "0.0.1",
		Uptime:         time.Since(m.startTime).String(),
		TotalJobs:      len(m.jobs),
		RunningJobs:    running,
		CompletedJobs:  completed,
		FailedJobs:     failed,
		CancelledJobs:  cancelled,
		Timestamp:      time.Now(),
	}
}

// Shutdown gracefully shuts down the manager
func (m *Manager) Shutdown() {
	m.logger.Info("shutting down job manager")

	// Cancel context to signal running jobs to stop
	m.cancel()

	// Wait for all goroutines to complete (with timeout)
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("all jobs completed gracefully")
	case <-time.After(shutdownTimeout):
		m.logger.Warn("shutdown timeout reached, some jobs may still be running")
	}
}

// executeJob runs a job (called in goroutine)
func (m *Manager) executeJob(jobID string) {
	// Get job
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return
	}

	// Update status to running
	job.Status = models.JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	job.UpdatedAt = now
	job.Progress = &models.JobProgress{
		Phase: "connecting",
	}

	// Send webhook notification for job start
	webhookMgr := m.webhookManager // Capture webhook manager before unlock
	m.mu.Unlock()

	m.logger.Info("job started", "id", jobID, "vm", job.Definition.VMPath)

	if webhookMgr != nil {
		webhookMgr.SendJobStarted(job)
	}

	// Execute the export. runExport (via normalizeJobDefinition) mutates the
	// definition it's given, so operate on a private copy rather than
	// job.Definition directly -- job.Definition is read concurrently by
	// GetJob under m.mu, and this goroutine doesn't hold that lock while
	// the export runs.
	defCopy := job.Definition
	err := m.runExport(jobID, &defCopy)

	// Update final status
	m.mu.Lock()

	job.Definition = defCopy
	endTime := time.Now()
	job.CompletedAt = &endTime
	job.UpdatedAt = endTime

	if err != nil {
		job.Status = models.JobStatusFailed
		job.Error = err.Error()
		m.logger.Error("job failed", "id", jobID, "error", err)

		// Send webhook notification for job failure
		if m.webhookManager != nil {
			m.webhookManager.SendJobFailed(job)
		}
	} else {
		job.Status = models.JobStatusCompleted
		m.logger.Info("job completed", "id", jobID)

		// Send webhook notification for job completion
		if m.webhookManager != nil {
			m.webhookManager.SendJobCompleted(job)
		}
	}

	m.mu.Unlock()
}

// runExport performs the actual VM export using capability-aware exporters
func (m *Manager) runExport(jobID string, def *models.JobDefinition) error {
	// Normalize job definition fields
	m.normalizeJobDefinition(def)

	if exporters.IsNutanixJob(def) {
		return m.runNutanixExport(jobID, def)
	}

	// Determine which export method to use
	method := m.selectExportMethod(def)
	m.logger.Info("using export method", "method", method, "job", jobID)

	// Create exporter
	exporter, err := m.exporterFactory.CreateExporter(method)
	if err != nil {
		return fmt.Errorf("create exporter: %w", err)
	}

	// Validate job definition against exporter requirements
	if err := exporter.Validate(def); err != nil {
		return fmt.Errorf("validate job definition: %w", err)
	}

	// Progress callback
	progressCallback := func(progress *models.JobProgress) {
		m.updateProgress(jobID, progress)
	}

	// Execute export
	ctx := m.ctx
	result, err := exporter.Export(ctx, def, progressCallback)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Store result
	m.mu.Lock()
	if job, exists := m.jobs[jobID]; exists {
		job.Result = result
	}
	m.mu.Unlock()

	return nil
}

func (m *Manager) runNutanixExport(jobID string, def *models.JobDefinition) error {
	exporter, err := m.exporterFactory.CreateNutanixExporter()
	if err != nil {
		return fmt.Errorf("create nutanix exporter: %w", err)
	}

	if err := exporter.Validate(def); err != nil {
		return fmt.Errorf("validate job definition: %w", err)
	}

	progressCallback := func(progress *models.JobProgress) {
		m.updateProgress(jobID, progress)
	}

	result, err := exporter.Export(m.ctx, def, progressCallback)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	m.mu.Lock()
	if job, exists := m.jobs[jobID]; exists {
		job.Result = result
	}
	m.mu.Unlock()

	return nil
}

// normalizeJobDefinition normalizes fields for backward compatibility
func (m *Manager) normalizeJobDefinition(def *models.JobDefinition) {
	// Normalize VCenter config
	if def.VCenter == nil && (def.VCenterURL != "" || def.Username != "" || def.Password != "") {
		// Convert old-style fields to new VCenter struct
		server := def.VCenterURL
		if server == "" {
			server = "vcenter.example.com" // fallback
		}
		def.VCenter = &models.VCenterConfig{
			Server:   server,
			Username: def.Username,
			Password: def.Password,
			Insecure: def.Insecure,
		}
	}

	// Normalize output paths
	if def.OutputDir == "" && def.OutputPath != "" {
		def.OutputDir = def.OutputPath
	}

	// Normalize export method
	if def.ExportMethod == "" && def.Method != "" {
		def.ExportMethod = def.Method
	}

	// Merge top-level pipeline / RDP flags into Options (web API compatibility)
	if def.EnablePipeline || def.EnableRDP != nil {
		if def.Options == nil {
			def.Options = &models.ExportOptions{}
		}
		if def.EnablePipeline {
			def.Options.EnablePipeline = true
		}
		if def.EnableRDP != nil {
			def.Options.EnableRDP = def.EnableRDP
		}
	}

	// Set defaults
	if def.Format == "" {
		if exporters.IsNutanixJob(def) {
			def.Format = "qcow2"
		} else {
			def.Format = "ovf"
		}
	}

	if def.Provider == "" && def.Metadata != nil {
		if p, ok := def.Metadata["provider"].(string); ok {
			def.Provider = p
		}
	}
}

// selectExportMethod determines which export method to use
func (m *Manager) selectExportMethod(def *models.JobDefinition) capabilities.ExportMethod {
	// If explicitly specified in job, use that
	if def.ExportMethod != "" {
		method := capabilities.ExportMethod(def.ExportMethod)
		if m.exporterFactory.IsAvailable(method) {
			return method
		}
		m.logger.Warn("requested export method not available, using default",
			"requested", def.ExportMethod)
	}

	// Use default (highest priority available)
	return m.exporterFactory.GetDefaultMethod()
}

// updateProgress updates job progress (thread-safe)
func (m *Manager) updateProgress(jobID string, progress *models.JobProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, exists := m.jobs[jobID]; exists {
		job.Progress = progress
		job.UpdatedAt = time.Now()
	}
}
