// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"github.com/zyvorai/transiva/daemon/models"
	"github.com/zyvorai/transiva/logger"
)

const (
	defaultDaemonURL = "http://localhost:8080"
)

// DaemonClient handles communication with hypervisord daemon
type DaemonClient struct {
	baseURL    string
	httpClient *http.Client
	log        logger.Logger
}

// NewDaemonClient creates a new daemon client
func NewDaemonClient(url string, log logger.Logger) *DaemonClient {
	return &DaemonClient{
		baseURL: url,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// SubmitExportJob submits an export job to the daemon
func (c *DaemonClient) SubmitExportJob(ctx context.Context, vmPath, outputDir, format string, compress bool) (string, error) {
	jobDef := models.JobDefinition{
		VMPath:     vmPath,
		OutputPath: outputDir,
		Format:     format,
		Compress:   compress,
	}

	data, err := json.Marshal(jobDef)
	if err != nil {
		return "", fmt.Errorf("marshal job definition: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/jobs/submit", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", "error", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.JobID, nil
}

// GetJobStatus retrieves the status of a job from the daemon
func (c *DaemonClient) GetJobStatus(ctx context.Context, jobID string) (*models.Job, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", "error", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var job models.Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &job, nil
}

// WatchJobProgress monitors job progress and displays it
func (c *DaemonClient) WatchJobProgress(ctx context.Context, jobID string, quiet bool) error {
	if !quiet {
		pterm.Info.Printfln("Watching job: %s", jobID)
		pterm.Info.Println("Press Ctrl+C to stop watching (job will continue)")
		pterm.Println()
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastProgress float64
	var lastStatus string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			job, err := c.GetJobStatus(ctx, jobID)
			if err != nil {
				c.log.Error("failed to get job status", "error", err)
				if !quiet {
					pterm.Warning.Printfln("Failed to get status: %v", err)
				}
				continue
			}

			currentStatus := string(job.Status)
			currentProgress := 0.0
			if job.Progress != nil {
				currentProgress = job.Progress.PercentComplete
			}

			// Only update display if status or progress changed
			if currentStatus != lastStatus || currentProgress != lastProgress {
				if !quiet {
					fmt.Printf("\r\033[K") // Clear line

					statusColor := pterm.FgGray
					switch currentStatus {
					case "running":
						statusColor = pterm.FgLightCyan
					case "completed":
						statusColor = pterm.FgGreen
					case "failed":
						statusColor = pterm.FgRed
					}

					if job.Progress != nil {
						fmt.Printf("[%s] %s %.1f%% - %s",
							time.Now().Format("15:04:05"),
							statusColor.Sprint(currentStatus),
							currentProgress,
							job.Progress.Phase)
					} else {
						fmt.Printf("[%s] %s",
							time.Now().Format("15:04:05"),
							statusColor.Sprint(currentStatus))
					}
				}

				lastStatus = currentStatus
				lastProgress = currentProgress
			}

			// Exit if job completed or failed
			if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
				fmt.Println()
				if currentStatus == "completed" {
					if !quiet {
						pterm.Success.Println("✅ Job completed successfully!")
					}
					return nil
				} else {
					if !quiet {
						pterm.Error.Printfln("❌ Job %s", currentStatus)
						if job.Error != "" {
							pterm.Error.Printfln("Error: %s", job.Error)
						}
					}
					return fmt.Errorf("job %s", currentStatus)
				}
			}
		}
	}
}

// ListJobs lists all jobs from the daemon
func (c *DaemonClient) ListJobs(ctx context.Context, status string) ([]models.Job, error) {
	url := c.baseURL + "/jobs/query"
	if status != "" {
		url += "?status=" + status
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", "error", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Jobs  []models.Job `json:"jobs"`
		Total int          `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Jobs, nil
}

// CreateSchedule creates a scheduled export job on the daemon
func (c *DaemonClient) CreateSchedule(ctx context.Context, name, schedule, vmPath, outputPath string) error {
	schedJob := models.ScheduledJob{
		ID:       fmt.Sprintf("schedule-%d", time.Now().Unix()),
		Name:     name,
		Schedule: schedule,
		Enabled:  true,
		JobTemplate: models.JobDefinition{
			VMPath:     vmPath,
			OutputPath: outputPath,
		},
	}

	data, err := json.Marshal(schedJob)
	if err != nil {
		return fmt.Errorf("marshal schedule: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/schedules", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetDaemonHealth checks if the daemon is running and healthy
func (c *DaemonClient) GetDaemonHealth(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetDaemonStatus retrieves daemon status information
func (c *DaemonClient) GetDaemonStatus(ctx context.Context) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/status", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			c.log.Warn("failed to close response body", "error", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("daemon returned status %d: %s", resp.StatusCode, string(body))
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return status, nil
}

// runDaemonMode handles export via daemon instead of direct export
func runDaemonMode(ctx context.Context, vmPath, outputPath, format string, compress, watch, quiet bool, daemonURL string, log logger.Logger) error {
	client := NewDaemonClient(daemonURL, log)

	// Check daemon health first
	var spinner *pterm.SpinnerPrinter
	if !quiet {
		spinner, _ = pterm.DefaultSpinner.Start("Checking daemon connectivity...")
		healthy, err := client.GetDaemonHealth(ctx)
		if err != nil {
			spinner.Fail(fmt.Sprintf("Failed to connect to daemon at %s", daemonURL))
			pterm.Error.Printfln("Error: %v", err)
			pterm.Info.Println("Make sure hypervisord is running:")
			pterm.Info.Println("  systemctl start hypervisord")
			pterm.Info.Println("  OR")
			pterm.Info.Println("  hypervisord -config /etc/transiva/config.yaml")
			return err
		}
		if !healthy {
			spinner.Warning("Daemon is not healthy")
		} else {
			spinner.Success(fmt.Sprintf("Connected to daemon at %s", daemonURL))
		}
	}

	// Submit job
	if !quiet {
		spinner, _ = pterm.DefaultSpinner.Start("Submitting export job to daemon...")
	}

	jobID, err := client.SubmitExportJob(ctx, vmPath, outputPath, format, compress)
	if err != nil {
		if !quiet {
			spinner.Fail(fmt.Sprintf("Failed to submit job: %v", err))
		}
		return err
	}

	if !quiet {
		spinner.Success(fmt.Sprintf("Job submitted successfully: %s", jobID))
		pterm.Info.Println("You can monitor the job with:")
		pterm.Info.Printfln("  hyperctl query -id %s", jobID)
		pterm.Info.Printfln("  hyperexport --daemon-watch %s", jobID)
	} else {
		fmt.Printf("job-id: %s\n", jobID)
	}

	// Watch job progress if requested
	if watch {
		if !quiet {
			pterm.Println()
		}
		return client.WatchJobProgress(ctx, jobID, quiet)
	}

	return nil
}

// displayDaemonJobs displays a list of jobs in a table
func displayDaemonJobs(jobs []models.Job) {
	if len(jobs) == 0 {
		pterm.Info.Println("No jobs found")
		return
	}

	pterm.DefaultSection.Println("📋 Daemon Jobs")

	data := [][]string{
		{"ID", "VM", "Status", "Progress", "Started", "Duration"},
	}

	for _, job := range jobs {
		id := job.Definition.ID
		if len(id) > 12 {
			id = id[:12] + "..."
		}

		vmName := job.Definition.VMPath
		if len(vmName) > 30 {
			parts := strings.Split(vmName, "/")
			vmName = parts[len(parts)-1]
			if len(vmName) > 30 {
				vmName = vmName[:27] + "..."
			}
		}

		progress := "-"
		if job.Progress != nil {
			progress = fmt.Sprintf("%.1f%%", job.Progress.PercentComplete)
		}

		started := "-"
		if job.StartedAt != nil && !job.StartedAt.IsZero() {
			started = job.StartedAt.Format("15:04 01/02")
		}

		duration := "-"
		if job.StartedAt != nil && !job.StartedAt.IsZero() {
			if job.CompletedAt != nil && !job.CompletedAt.IsZero() {
				duration = job.CompletedAt.Sub(*job.StartedAt).Round(time.Second).String()
			} else {
				duration = time.Since(*job.StartedAt).Round(time.Second).String()
			}
		}

		statusStr := string(job.Status)
		switch job.Status {
		case models.JobStatusRunning:
			statusStr = pterm.FgLightCyan.Sprint(statusStr)
		case models.JobStatusCompleted:
			statusStr = pterm.FgGreen.Sprint(statusStr)
		case models.JobStatusFailed:
			statusStr = pterm.FgRed.Sprint(statusStr)
		}

		data = append(data, []string{
			id,
			vmName,
			statusStr,
			progress,
			started,
			duration,
		})
	}

	if err := pterm.DefaultTable.
		WithHasHeader().
		WithHeaderRowSeparator("-").
		WithBoxed().
		WithData(data).
		Render(); err != nil {
		pterm.Error.Printfln("failed to render jobs table: %v", err)
	}
}
