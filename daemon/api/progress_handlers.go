// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/zyvorai/transiva/daemon/models"
)

// handleGetJobProgress gets real-time progress for a job
func (s *Server) handleGetJobProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path
	jobID := filepath.Base(r.URL.Path)
	if jobID == "progress" || jobID == "" {
		s.errorResponse(w, http.StatusBadRequest, "job ID required")
		return
	}

	job, err := s.manager.GetJob(jobID)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "job not found: %s", jobID)
		return
	}

	// Build progress response
	response := map[string]interface{}{
		"job_id":    jobID,
		"status":    job.Status,
		"timestamp": time.Now(),
	}

	if job.Progress != nil {
		response["progress"] = map[string]interface{}{
			"phase":               job.Progress.Phase,
			"current_file":        job.Progress.CurrentFile,
			"current_step":        job.Progress.CurrentStep,
			"files_downloaded":    job.Progress.FilesDownloaded,
			"total_files":         job.Progress.TotalFiles,
			"bytes_downloaded":    job.Progress.BytesDownloaded,
			"total_bytes":         job.Progress.TotalBytes,
			"percent_complete":    job.Progress.PercentComplete,
			"estimated_remaining": job.Progress.EstimatedRemaining,
			"export_method":       job.Progress.ExportMethod,
		}

		// Calculate transfer rate if job is running
		if job.Status == "running" && job.StartedAt != nil {
			elapsed := time.Since(*job.StartedAt).Seconds()
			if elapsed > 0 {
				bytesPerSecond := float64(job.Progress.BytesDownloaded) / elapsed
				response["transfer_rate_mbps"] = bytesPerSecond / (1024 * 1024)
			}
		}
	}

	// Add timing information
	if job.StartedAt != nil {
		response["started_at"] = *job.StartedAt
		response["elapsed_seconds"] = time.Since(*job.StartedAt).Seconds()
	}
	if job.CompletedAt != nil {
		response["completed_at"] = *job.CompletedAt
	}

	// Add error if present
	if job.Error != "" {
		response["error"] = job.Error
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// handleGetJobLogs gets detailed logs for a job
func (s *Server) handleGetJobLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path
	jobID := filepath.Base(r.URL.Path)
	if jobID == "logs" || jobID == "" {
		s.errorResponse(w, http.StatusBadRequest, "job ID required")
		return
	}

	job, err := s.manager.GetJob(jobID)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "job not found: %s", jobID)
		return
	}

	// Build log entries
	logs := []map[string]interface{}{}

	// Add job lifecycle events
	logs = append(logs, map[string]interface{}{
		"timestamp": job.Definition.CreatedAt,
		"level":     "info",
		"message":   "Job created",
		"details": map[string]interface{}{
			"vm_path":    job.Definition.VMPath,
			"output_dir": getOutputDir(job.Definition),
			"format":     job.Definition.Format,
			"method":     getExportMethod(job.Definition),
		},
	})

	if job.StartedAt != nil {
		logs = append(logs, map[string]interface{}{
			"timestamp": *job.StartedAt,
			"level":     "info",
			"message":   "Job started",
		})
	}

	// Add progress log entries if available
	if job.Progress != nil {
		if job.Progress.Phase != "" {
			logs = append(logs, map[string]interface{}{
				"timestamp": job.UpdatedAt,
				"level":     "info",
				"message":   "Progress update",
				"details": map[string]interface{}{
					"phase":            job.Progress.Phase,
					"current_step":     job.Progress.CurrentStep,
					"percent_complete": job.Progress.PercentComplete,
				},
			})
		}
	}

	// Add completion/failure log
	if job.CompletedAt != nil {
		level := "info"
		message := "Job completed successfully"
		switch job.Status {
		case "failed":
			level = "error"
			message = "Job failed"
		case "cancelled":
			level = "warn"
			message = "Job cancelled"
		}

		logEntry := map[string]interface{}{
			"timestamp": *job.CompletedAt,
			"level":     level,
			"message":   message,
		}

		if job.Error != "" {
			logEntry["error"] = job.Error
		}

		if job.Result != nil {
			logEntry["details"] = map[string]interface{}{
				"total_size":  job.Result.TotalSize,
				"duration":    job.Result.Duration.String(),
				"files_count": len(job.Result.Files),
			}
		}

		logs = append(logs, logEntry)
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"job_id":    jobID,
		"logs":      logs,
		"log_count": len(logs),
		"timestamp": time.Now(),
	})
}

// handleGetJobETA gets estimated time remaining for a job
func (s *Server) handleGetJobETA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path
	jobID := filepath.Base(r.URL.Path)
	if jobID == "eta" || jobID == "" {
		s.errorResponse(w, http.StatusBadRequest, "job ID required")
		return
	}

	job, err := s.manager.GetJob(jobID)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "job not found: %s", jobID)
		return
	}

	response := map[string]interface{}{
		"job_id":    jobID,
		"status":    job.Status,
		"timestamp": time.Now(),
	}

	// Calculate ETA for running jobs
	if job.Status == "running" && job.StartedAt != nil && job.Progress != nil {
		elapsed := time.Since(*job.StartedAt)

		if job.Progress.PercentComplete > 0 && job.Progress.PercentComplete < 100 {
			// Calculate ETA based on progress percentage
			totalEstimated := elapsed / time.Duration(job.Progress.PercentComplete/100.0)
			remaining := totalEstimated - elapsed

			etaMap := map[string]interface{}{
				"estimated_completion":   time.Now().Add(remaining),
				"time_remaining":         remaining.String(),
				"time_remaining_seconds": remaining.Seconds(),
				"elapsed":                elapsed.String(),
				"elapsed_seconds":        elapsed.Seconds(),
				"percent_complete":       job.Progress.PercentComplete,
			}

			// Add transfer rate based calculation if we have bytes info
			if job.Progress.TotalBytes > 0 && job.Progress.BytesDownloaded > 0 {
				bytesRemaining := job.Progress.TotalBytes - job.Progress.BytesDownloaded
				bytesPerSecond := float64(job.Progress.BytesDownloaded) / elapsed.Seconds()

				if bytesPerSecond > 0 {
					secondsRemaining := float64(bytesRemaining) / bytesPerSecond
					etaByBytes := time.Now().Add(time.Duration(secondsRemaining) * time.Second)

					etaMap["estimated_completion_by_bytes"] = etaByBytes
					etaMap["transfer_rate_mbps"] = bytesPerSecond / (1024 * 1024)
					etaMap["bytes_remaining"] = bytesRemaining
				}
			}

			response["eta"] = etaMap
		} else {
			response["eta"] = map[string]interface{}{
				"message": "ETA calculation not available yet",
				"elapsed": elapsed.String(),
			}
		}
	} else if job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled" {
		etaMap := map[string]interface{}{
			"message": "Job already " + string(job.Status),
		}
		if job.Result != nil {
			etaMap["total_duration"] = job.Result.Duration.String()
		}
		response["eta"] = etaMap
	} else {
		response["eta"] = map[string]interface{}{
			"message": "Job not started yet",
			"status":  job.Status,
		}
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// Helper functions
func getOutputDir(def models.JobDefinition) string {
	if def.OutputDir != "" {
		return def.OutputDir
	}
	return def.OutputPath
}

func getExportMethod(def models.JobDefinition) string {
	if def.ExportMethod != "" {
		return def.ExportMethod
	}
	if def.Method != "" {
		return def.Method
	}
	return "auto"
}
