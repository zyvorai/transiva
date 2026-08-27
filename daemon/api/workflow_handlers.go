// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WorkflowStatus represents workflow daemon status
type WorkflowStatus struct {
	Mode           string `json:"mode"`
	Running        bool   `json:"running"`
	QueueDepth     int    `json:"queue_depth"`
	ActiveJobs     int    `json:"active_jobs"`
	ProcessedToday int    `json:"processed_today"`
	FailedToday    int    `json:"failed_today"`
	MaxWorkers     int    `json:"max_workers"`
	UptimeSeconds  int    `json:"uptime_seconds"`
}

// WorkflowJob represents a workflow job
type WorkflowJob struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Stage          string    `json:"stage"`
	Progress       int       `json:"progress"`
	StartedAt      time.Time `json:"started_at"`
	ElapsedSeconds int       `json:"elapsed_seconds"`
	Status         string    `json:"status"`
}

// WorkflowStatusHandler handles GET /api/workflow/status
func (s *Server) WorkflowStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.getWorkflowStatus()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		s.logger.Warn("failed to encode workflow status response", "error", err)
	}
}

// WorkflowJobsHandler handles GET /api/workflow/jobs
func (s *Server) WorkflowJobsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	statusFilter := r.URL.Query().Get("status")

	jobs := s.getWorkflowJobs(statusFilter)

	response := map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Warn("failed to encode response", "error", err)
	}
}

// WorkflowJobsActiveHandler handles GET /api/workflow/jobs/active
func (s *Server) WorkflowJobsActiveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobs := s.getWorkflowJobs("processing")

	response := map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Warn("failed to encode response", "error", err)
	}
}

// ManifestSubmitHandler handles POST /api/workflow/manifest/submit
func (s *Server) ManifestSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request: %v", err), http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var manifest map[string]interface{}
	if err := json.Unmarshal(body, &manifest); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := validateManifest(manifest); err != nil {
		http.Error(w, fmt.Sprintf("Invalid manifest: %v", err), http.StatusBadRequest)
		return
	}

	manifestPath, err := s.submitManifestToWorkflow(manifest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit manifest: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success":       true,
		"manifest_path": manifestPath,
		"job_id":        filepath.Base(manifestPath),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Warn("failed to encode response", "error", err)
	}
}

// ManifestValidateHandler handles POST /api/workflow/manifest/validate
func (s *Server) ManifestValidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request: %v", err), http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var manifest map[string]interface{}
	if err := json.Unmarshal(body, &manifest); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	errors := validateManifestDetailed(manifest)

	response := map[string]interface{}{
		"valid":  len(errors) == 0,
		"errors": errors,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Warn("failed to encode response", "error", err)
	}
}

// Helper functions

func (s *Server) getWorkflowStatus() WorkflowStatus {
	status := WorkflowStatus{
		Mode:       "manifest",
		Running:    false,
		MaxWorkers: 3,
	}

	workflowDirs := []string{
		"/var/lib/h2kvm/workflow",
		"/var/lib/h2kvm/manifest-workflow",
		// Legacy (pre-rename) paths, kept for hosts with an older h2kvm install
		"/var/lib/hyper2kvm/workflow",
		"/var/lib/hyper2kvm/manifest-workflow",
	}

	for _, dir := range workflowDirs {
		if _, err := os.Stat(dir); err == nil {
			status.Running = true

			if strings.Contains(dir, "manifest") {
				status.Mode = "manifest"
			} else {
				status.Mode = "disk"
			}

			status.QueueDepth = s.countFilesInDir(filepath.Join(dir, "to_be_processed"))
			status.ActiveJobs = s.countFilesInDir(filepath.Join(dir, "processing"))

			today := time.Now().Format("2006-01-02")
			status.ProcessedToday = s.countFilesInDir(filepath.Join(dir, "processed", today))
			status.FailedToday = s.countFilesInDir(filepath.Join(dir, "failed", today))

			break
		}
	}

	return status
}

func (s *Server) getWorkflowJobs(statusFilter string) []WorkflowJob {
	var jobs []WorkflowJob

	baseDir := "/var/lib/h2kvm/manifest-workflow"
	switch {
	case fileExists("/var/lib/h2kvm/workflow/processing"):
		baseDir = "/var/lib/h2kvm/workflow"
	case fileExists("/var/lib/h2kvm/manifest-workflow"):
		// use default above
	case fileExists("/var/lib/hyper2kvm/workflow/processing"):
		// Legacy (pre-rename) path, kept for hosts with an older h2kvm install
		baseDir = "/var/lib/hyper2kvm/workflow"
	case fileExists("/var/lib/hyper2kvm/manifest-workflow"):
		baseDir = "/var/lib/hyper2kvm/manifest-workflow"
	}

	var dirPath string
	switch statusFilter {
	case "processing", "active":
		dirPath = filepath.Join(baseDir, "processing")
	case "pending":
		dirPath = filepath.Join(baseDir, "to_be_processed")
	case "completed":
		today := time.Now().Format("2006-01-02")
		dirPath = filepath.Join(baseDir, "processed", today)
	case "failed":
		today := time.Now().Format("2006-01-02")
		dirPath = filepath.Join(baseDir, "failed", today)
	default:
		dirPath = filepath.Join(baseDir, "processing")
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return jobs
	}

	for _, entry := range entries {
		if entry.IsDir() || isMetadataFile(entry.Name()) {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		job := WorkflowJob{
			ID:             entry.Name(),
			Name:           strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Status:         statusFilter,
			StartedAt:      fileInfo.ModTime(),
			ElapsedSeconds: int(time.Since(fileInfo.ModTime()).Seconds()),
			Stage:          "PROCESSING",
			Progress:       0,
		}

		jobs = append(jobs, job)
	}

	return jobs
}

func (s *Server) countFilesInDir(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && !isMetadataFile(entry.Name()) {
			count++
		}
	}
	return count
}

func (s *Server) submitManifestToWorkflow(manifest map[string]interface{}) (string, error) {
	// Submit into whichever tree already exists on this host (new name preferred),
	// falling back to the legacy (pre-rename) path for older installs, and
	// defaulting to the new name if neither exists yet.
	workflowDir := "/var/lib/h2kvm/manifest-workflow/to_be_processed"
	if !fileExists("/var/lib/h2kvm/manifest-workflow") && fileExists("/var/lib/hyper2kvm/manifest-workflow") {
		workflowDir = "/var/lib/hyper2kvm/manifest-workflow/to_be_processed"
	}

	if err := os.MkdirAll(workflowDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create workflow directory: %v", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("manifest-%s.json", timestamp)

	if name, ok := manifest["name"].(string); ok {
		filename = fmt.Sprintf("%s-%s.json", name, timestamp)
	}

	manifestPath := filepath.Join(workflowDir, filename)

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest: %v", err)
	}

	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write manifest: %v", err)
	}

	return manifestPath, nil
}

func isMetadataFile(name string) bool {
	return strings.HasSuffix(name, ".meta.json") ||
		strings.HasSuffix(name, ".report.json") ||
		strings.HasSuffix(name, ".error.json")
}

func validateManifest(manifest map[string]interface{}) error {
	version, ok := manifest["version"].(string)
	if !ok || version == "" {
		return fmt.Errorf("version is required")
	}

	batch, _ := manifest["batch"].(bool)

	if !batch {
		pipeline, ok := manifest["pipeline"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("pipeline is required for single VM manifest")
		}

		load, ok := pipeline["load"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("pipeline.load is required")
		}

		if sourceType, _ := load["source_type"].(string); sourceType == "" {
			return fmt.Errorf("pipeline.load.source_type is required")
		}
		if sourcePath, _ := load["source_path"].(string); sourcePath == "" {
			return fmt.Errorf("pipeline.load.source_path is required")
		}

		convert, ok := pipeline["convert"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("pipeline.convert is required")
		}

		if outputFormat, _ := convert["output_format"].(string); outputFormat == "" {
			return fmt.Errorf("pipeline.convert.output_format is required")
		}
	} else {
		vms, ok := manifest["vms"].([]interface{})
		if !ok || len(vms) == 0 {
			return fmt.Errorf("batch manifest requires at least one VM in 'vms' array")
		}
	}

	return nil
}

func validateManifestDetailed(manifest map[string]interface{}) []string {
	var errors []string

	version, ok := manifest["version"].(string)
	if !ok || version == "" {
		errors = append(errors, "version is required")
	}

	batch, _ := manifest["batch"].(bool)

	if !batch {
		pipeline, ok := manifest["pipeline"].(map[string]interface{})
		if !ok {
			errors = append(errors, "pipeline is required")
			return errors
		}

		load, ok := pipeline["load"].(map[string]interface{})
		if !ok {
			errors = append(errors, "pipeline.load is required")
		} else {
			if sourceType, _ := load["source_type"].(string); sourceType == "" {
				errors = append(errors, "pipeline.load.source_type is required")
			}
			if sourcePath, _ := load["source_path"].(string); sourcePath == "" {
				errors = append(errors, "pipeline.load.source_path is required")
			}
		}

		convert, ok := pipeline["convert"].(map[string]interface{})
		if !ok {
			errors = append(errors, "pipeline.convert is required")
		} else {
			if outputFormat, _ := convert["output_format"].(string); outputFormat == "" {
				errors = append(errors, "pipeline.convert.output_format is required")
			}
		}
	} else {
		vms, ok := manifest["vms"].([]interface{})
		if !ok {
			errors = append(errors, "vms array is required for batch manifests")
			return errors
		}

		if len(vms) == 0 {
			errors = append(errors, "batch manifest requires at least one VM")
		}
	}

	return errors
}
