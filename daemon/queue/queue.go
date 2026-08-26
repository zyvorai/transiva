// SPDX-License-Identifier: Apache-2.0

// Package queue provides a priority-based job queue with concurrency control
package queue

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Priority levels for jobs
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// String returns the string representation of the priority
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Job represents a job in the queue
type Job struct {
	ID          string
	Priority    Priority
	SubmittedAt time.Time
	Payload     interface{}
	RetryCount  int
	MaxRetries  int
	Timeout     time.Duration
}

// JobResult represents the result of job execution
type JobResult struct {
	JobID     string
	Success   bool
	Error     error
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
}

// JobHandler is a function that processes a job
type JobHandler func(ctx context.Context, job *Job) error

// Config holds queue configuration
type Config struct {
	// MaxWorkers is the maximum number of concurrent workers
	MaxWorkers int

	// MaxQueueSize is the maximum number of jobs in the queue
	MaxQueueSize int

	// DefaultTimeout is the default timeout for job execution
	DefaultTimeout time.Duration

	// EnableMetrics enables metrics collection
	EnableMetrics bool
}

// DefaultConfig returns default queue configuration
func DefaultConfig() *Config {
	return &Config{
		MaxWorkers:     10,
		MaxQueueSize:   1000,
		DefaultTimeout: 30 * time.Minute,
		EnableMetrics:  true,
	}
}

// Queue is a priority-based job queue
type Queue struct {
	config  *Config
	pq      *priorityQueue
	mu      sync.RWMutex
	handler JobHandler
	workers []*worker
	results chan *JobResult
	metrics *Metrics
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// Metrics tracks queue metrics
type Metrics struct {
	mu                sync.RWMutex
	JobsEnqueued      int64
	JobsDequeued      int64
	JobsCompleted     int64
	JobsFailed        int64
	JobsTimeout       int64
	JobsRetried       int64
	CurrentQueueSize  int
	ActiveWorkers     int
	AverageWaitTime   time.Duration
	AverageProcessing time.Duration
}

// NewQueue creates a new priority queue
func NewQueue(config *Config, handler JobHandler) (*Queue, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if handler == nil {
		return nil, errors.New("job handler is required")
	}

	if config.MaxWorkers <= 0 {
		return nil, errors.New("max workers must be positive")
	}

	ctx, cancel := context.WithCancel(context.Background())

	q := &Queue{
		config:  config,
		pq:      &priorityQueue{},
		handler: handler,
		workers: make([]*worker, config.MaxWorkers),
		results: make(chan *JobResult, 100),
		metrics: &Metrics{},
		ctx:     ctx,
		cancel:  cancel,
	}

	heap.Init(q.pq)

	// Start workers
	for i := 0; i < config.MaxWorkers; i++ {
		w := newWorker(i, q)
		q.workers[i] = w
		q.wg.Add(1)
		go w.run()
	}

	// Start result collector
	q.wg.Add(1)
	go q.collectResults()

	return q, nil
}

// Enqueue adds a job to the queue
func (q *Queue) Enqueue(job *Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check queue size
	if q.pq.Len() >= q.config.MaxQueueSize {
		return fmt.Errorf("queue is full (max %d)", q.config.MaxQueueSize)
	}

	// Set defaults
	if job.SubmittedAt.IsZero() {
		job.SubmittedAt = time.Now()
	}

	if job.Timeout == 0 {
		job.Timeout = q.config.DefaultTimeout
	}

	// Add to queue
	heap.Push(q.pq, job)

	// Update metrics
	if q.config.EnableMetrics {
		q.metrics.mu.Lock()
		q.metrics.JobsEnqueued++
		q.metrics.CurrentQueueSize = q.pq.Len()
		q.metrics.mu.Unlock()
	}

	return nil
}

// Dequeue removes and returns the highest priority job
func (q *Queue) Dequeue() (*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pq.Len() == 0 {
		return nil, errors.New("queue is empty")
	}

	job := heap.Pop(q.pq).(*Job)

	// Update metrics
	if q.config.EnableMetrics {
		q.metrics.mu.Lock()
		q.metrics.JobsDequeued++
		q.metrics.CurrentQueueSize = q.pq.Len()
		q.metrics.mu.Unlock()
	}

	return job, nil
}

// Size returns the current queue size
func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.pq.Len()
}

// IsEmpty returns true if the queue is empty
func (q *Queue) IsEmpty() bool {
	return q.Size() == 0
}

// IsFull returns true if the queue is full
func (q *Queue) IsFull() bool {
	return q.Size() >= q.config.MaxQueueSize
}

// GetMetrics returns current metrics
func (q *Queue) GetMetrics() *Metrics {
	q.metrics.mu.RLock()
	defer q.metrics.mu.RUnlock()

	// Update active workers
	q.metrics.ActiveWorkers = q.activeWorkerCount()

	// Return a copy without the mutex
	return &Metrics{
		JobsEnqueued:      q.metrics.JobsEnqueued,
		JobsDequeued:      q.metrics.JobsDequeued,
		JobsCompleted:     q.metrics.JobsCompleted,
		JobsFailed:        q.metrics.JobsFailed,
		JobsTimeout:       q.metrics.JobsTimeout,
		JobsRetried:       q.metrics.JobsRetried,
		CurrentQueueSize:  q.metrics.CurrentQueueSize,
		ActiveWorkers:     q.metrics.ActiveWorkers,
		AverageWaitTime:   q.metrics.AverageWaitTime,
		AverageProcessing: q.metrics.AverageProcessing,
	}
}

// activeWorkerCount returns the number of active workers
func (q *Queue) activeWorkerCount() int {
	count := 0
	for _, w := range q.workers {
		if w.isBusy() {
			count++
		}
	}
	return count
}

// collectResults collects job results
func (q *Queue) collectResults() {
	defer q.wg.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		case result := <-q.results:
			q.handleResult(result)
		}
	}
}

// handleResult handles a job result
func (q *Queue) handleResult(result *JobResult) {
	if !q.config.EnableMetrics {
		return
	}

	q.metrics.mu.Lock()
	defer q.metrics.mu.Unlock()

	if result.Success {
		q.metrics.JobsCompleted++
	} else {
		q.metrics.JobsFailed++
	}

	// Update average processing time
	if q.metrics.JobsCompleted > 0 {
		total := time.Duration(q.metrics.JobsCompleted) * q.metrics.AverageProcessing
		total += result.Duration
		q.metrics.AverageProcessing = total / time.Duration(q.metrics.JobsCompleted+1)
	} else {
		q.metrics.AverageProcessing = result.Duration
	}
}

// Shutdown gracefully shuts down the queue
func (q *Queue) Shutdown(ctx context.Context) error {
	// Cancel context to stop workers
	q.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// worker processes jobs from the queue
type worker struct {
	id       int
	queue    *Queue
	busy     bool
	busyMu   sync.RWMutex
	jobCount int64
}

// newWorker creates a new worker
func newWorker(id int, queue *Queue) *worker {
	return &worker{
		id:    id,
		queue: queue,
	}
}

// run starts the worker
func (w *worker) run() {
	defer w.queue.wg.Done()

	for {
		select {
		case <-w.queue.ctx.Done():
			return
		default:
			job, err := w.queue.Dequeue()
			if err != nil {
				// Queue empty, wait a bit
				time.Sleep(100 * time.Millisecond)
				continue
			}

			w.processJob(job)
		}
	}
}

// processJob processes a single job
func (w *worker) processJob(job *Job) {
	w.setBusy(true)
	defer w.setBusy(false)

	w.jobCount++
	startedAt := time.Now()

	// Create job context with timeout
	ctx, cancel := context.WithTimeout(w.queue.ctx, job.Timeout)
	defer cancel()

	// Execute job
	err := w.queue.handler(ctx, job)

	endedAt := time.Now()
	duration := endedAt.Sub(startedAt)

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		w.queue.metrics.mu.Lock()
		w.queue.metrics.JobsTimeout++
		w.queue.metrics.mu.Unlock()

		err = fmt.Errorf("job timeout after %v: %w", duration, ctx.Err())
	}

	// Handle retry
	if err != nil && job.RetryCount < job.MaxRetries {
		job.RetryCount++
		w.queue.metrics.mu.Lock()
		w.queue.metrics.JobsRetried++
		w.queue.metrics.mu.Unlock()

		// Re-enqueue with delay
		time.Sleep(time.Second * time.Duration(job.RetryCount))
		if enqueueErr := w.queue.Enqueue(job); enqueueErr != nil {
			// Could not re-enqueue (e.g. queue full/closed); fall through and
			// report the original failure instead of silently dropping the job.
			err = fmt.Errorf("retry failed: %w (original error: %v)", enqueueErr, err)
		} else {
			return
		}
	}

	// Send result
	result := &JobResult{
		JobID:     job.ID,
		Success:   err == nil,
		Error:     err,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Duration:  duration,
	}

	select {
	case w.queue.results <- result:
	default:
		// Results channel full, drop result
	}
}

// isBusy returns true if the worker is processing a job
func (w *worker) isBusy() bool {
	w.busyMu.RLock()
	defer w.busyMu.RUnlock()
	return w.busy
}

// setBusy sets the worker busy status
func (w *worker) setBusy(busy bool) {
	w.busyMu.Lock()
	defer w.busyMu.Unlock()
	w.busy = busy
}

// priorityQueue implements heap.Interface
type priorityQueue []*Job

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Higher priority first
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// Earlier submission time first (FIFO within same priority)
	return pq[i].SubmittedAt.Before(pq[j].SubmittedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*Job))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	job := old[n-1]
	*pq = old[0 : n-1]
	return job
}
