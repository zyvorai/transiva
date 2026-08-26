// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ProgressStatus represents the current status of a migration task
type ProgressStatus string

const (
	StatusPending    ProgressStatus = "pending"
	StatusExporting  ProgressStatus = "exporting"
	StatusConverting ProgressStatus = "converting"
	StatusUploading  ProgressStatus = "uploading"
	StatusCompleted  ProgressStatus = "completed"
	StatusFailed     ProgressStatus = "failed"
)

// ProgressInfo holds real-time progress information
type ProgressInfo struct {
	// Task identification
	TaskID   string         `json:"task_id"`
	VMName   string         `json:"vm_name"`
	Provider string         `json:"provider"`
	Status   ProgressStatus `json:"status"`

	// Timestamps
	StartTime   time.Time  `json:"start_time"`
	UpdatedTime time.Time  `json:"updated_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`

	// Progress tracking
	CurrentStage string  `json:"current_stage"`
	TotalStages  int     `json:"total_stages"`
	StageIndex   int     `json:"stage_index"`
	Percentage   float64 `json:"percentage"`

	// Export progress
	ExportProgress *StageProgress `json:"export_progress,omitempty"`

	// Conversion progress
	ConversionProgress *StageProgress `json:"conversion_progress,omitempty"`

	// Upload progress
	UploadProgress *StageProgress `json:"upload_progress,omitempty"`

	// Error information
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// StageProgress represents progress for a specific stage
type StageProgress struct {
	Stage      string    `json:"stage"`
	Status     string    `json:"status"`
	StartTime  time.Time `json:"start_time"`
	Percentage float64   `json:"percentage"`
	BytesTotal int64     `json:"bytes_total"`
	BytesDone  int64     `json:"bytes_done"`
	Rate       int64     `json:"rate"` // Bytes per second
	ETA        int64     `json:"eta"`  // Seconds remaining
	Message    string    `json:"message,omitempty"`
}

// ProgressTracker tracks progress for multiple tasks
type ProgressTracker struct {
	mu        sync.RWMutex
	tasks     map[string]*ProgressInfo
	listeners map[string][]chan *ProgressInfo
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		tasks:     make(map[string]*ProgressInfo),
		listeners: make(map[string][]chan *ProgressInfo),
	}
}

// StartTask starts tracking a new task
func (pt *ProgressTracker) StartTask(taskID, vmName, provider string) *ProgressInfo {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	info := &ProgressInfo{
		TaskID:      taskID,
		VMName:      vmName,
		Provider:    provider,
		Status:      StatusPending,
		StartTime:   time.Now(),
		UpdatedTime: time.Now(),
		TotalStages: 3, // export, convert, upload
		Metadata:    make(map[string]interface{}),
	}

	pt.tasks[taskID] = info
	pt.notifyListeners(taskID, info)

	return info
}

// UpdateProgress updates task progress
func (pt *ProgressTracker) UpdateProgress(taskID string, updates func(*ProgressInfo)) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	info, ok := pt.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	updates(info)
	info.UpdatedTime = time.Now()

	pt.notifyListeners(taskID, info)

	return nil
}

// SetStatus sets task status
func (pt *ProgressTracker) SetStatus(taskID string, status ProgressStatus) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		info.Status = status
	})
}

// SetStage sets current stage
func (pt *ProgressTracker) SetStage(taskID, stage string, stageIndex int) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		info.CurrentStage = stage
		info.StageIndex = stageIndex
	})
}

// SetPercentage sets overall percentage
func (pt *ProgressTracker) SetPercentage(taskID string, percentage float64) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		info.Percentage = percentage
	})
}

// SetExportProgress sets export stage progress
func (pt *ProgressTracker) SetExportProgress(taskID string, progress *StageProgress) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		info.ExportProgress = progress
		info.Status = StatusExporting
	})
}

// SetConversionProgress sets conversion stage progress
func (pt *ProgressTracker) SetConversionProgress(taskID string, progress *StageProgress) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		info.ConversionProgress = progress
		info.Status = StatusConverting
	})
}

// SetUploadProgress sets upload stage progress
func (pt *ProgressTracker) SetUploadProgress(taskID string, progress *StageProgress) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		info.UploadProgress = progress
		info.Status = StatusUploading
	})
}

// CompleteTask marks task as completed
func (pt *ProgressTracker) CompleteTask(taskID string) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		now := time.Now()
		info.Status = StatusCompleted
		info.EndTime = &now
		info.Percentage = 100.0
	})
}

// FailTask marks task as failed
func (pt *ProgressTracker) FailTask(taskID string, err error) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		now := time.Now()
		info.Status = StatusFailed
		info.EndTime = &now
		info.Error = err.Error()
	})
}

// AddWarning adds a warning to task
func (pt *ProgressTracker) AddWarning(taskID, warning string) error {
	return pt.UpdateProgress(taskID, func(info *ProgressInfo) {
		info.Warnings = append(info.Warnings, warning)
	})
}

// GetProgress returns progress for a task
func (pt *ProgressTracker) GetProgress(taskID string) (*ProgressInfo, error) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	info, ok := pt.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	// Return a copy to avoid data races
	infoCopy := *info
	return &infoCopy, nil
}

// GetAllProgress returns progress for all tasks
func (pt *ProgressTracker) GetAllProgress() []*ProgressInfo {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var allProgress []*ProgressInfo
	for _, info := range pt.tasks {
		infoCopy := *info
		allProgress = append(allProgress, &infoCopy)
	}

	return allProgress
}

// Subscribe subscribes to progress updates for a task
func (pt *ProgressTracker) Subscribe(taskID string) chan *ProgressInfo {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	ch := make(chan *ProgressInfo, 10)
	pt.listeners[taskID] = append(pt.listeners[taskID], ch)

	return ch
}

// Unsubscribe unsubscribes from progress updates
func (pt *ProgressTracker) Unsubscribe(taskID string, ch chan *ProgressInfo) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	listeners := pt.listeners[taskID]
	for i, listener := range listeners {
		if listener == ch {
			pt.listeners[taskID] = append(listeners[:i], listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// notifyListeners notifies all listeners of progress update
func (pt *ProgressTracker) notifyListeners(taskID string, info *ProgressInfo) {
	listeners := pt.listeners[taskID]
	infoCopy := *info

	for _, ch := range listeners {
		select {
		case ch <- &infoCopy:
		default:
			// Channel full, skip
		}
	}
}

// RemoveTask removes a task from tracking
func (pt *ProgressTracker) RemoveTask(taskID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	delete(pt.tasks, taskID)

	// Close and remove all listeners
	for _, ch := range pt.listeners[taskID] {
		close(ch)
	}
	delete(pt.listeners, taskID)
}

// ProgressAPIServer serves progress information via HTTP
type ProgressAPIServer struct {
	tracker *ProgressTracker
	addr    string
	server  *http.Server
}

// NewProgressAPIServer creates a new progress API server
func NewProgressAPIServer(tracker *ProgressTracker, addr string) *ProgressAPIServer {
	return &ProgressAPIServer{
		tracker: tracker,
		addr:    addr,
	}
}

// Start starts the API server
func (s *ProgressAPIServer) Start() error {
	mux := http.NewServeMux()

	// GET /api/v1/progress - Get all tasks
	mux.HandleFunc("/api/v1/progress", s.handleGetAllProgress)

	// GET /api/v1/progress/{taskID} - Get specific task
	mux.HandleFunc("/api/v1/progress/", s.handleGetProgress)

	// GET /api/v1/stream/{taskID} - Stream progress (SSE)
	mux.HandleFunc("/api/v1/stream/", s.handleStreamProgress)

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Stop stops the API server
func (s *ProgressAPIServer) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleGetAllProgress handles GET /api/v1/progress
func (s *ProgressAPIServer) handleGetAllProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allProgress := s.tracker.GetAllProgress()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": allProgress,
		"count": len(allProgress),
	}); err != nil {
		log.Printf("progress_api: failed to encode progress list response: %v", err)
	}
}

// handleGetProgress handles GET /api/v1/progress/{taskID}
func (s *ProgressAPIServer) handleGetProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from path
	taskID := r.URL.Path[len("/api/v1/progress/"):]
	if taskID == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	progress, err := s.tracker.GetProgress(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(progress); err != nil {
		log.Printf("progress_api: failed to encode progress response: %v", err)
	}
}

// handleStreamProgress handles GET /api/v1/stream/{taskID} (Server-Sent Events)
func (s *ProgressAPIServer) handleStreamProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from path
	taskID := r.URL.Path[len("/api/v1/stream/"):]
	if taskID == "" {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe to progress updates
	ch := s.tracker.Subscribe(taskID)
	defer s.tracker.Unsubscribe(taskID, ch)

	// Send initial state
	if progress, err := s.tracker.GetProgress(taskID); err == nil {
		data, _ := json.Marshal(progress)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	// Stream updates
	for {
		select {
		case <-r.Context().Done():
			return
		case progress, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(progress)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}
