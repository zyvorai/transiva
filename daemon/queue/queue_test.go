// SPDX-License-Identifier: Apache-2.0

package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPriorityString(t *testing.T) {
	tests := []struct {
		priority Priority
		expected string
	}{
		{PriorityLow, "low"},
		{PriorityNormal, "normal"},
		{PriorityHigh, "high"},
		{PriorityCritical, "critical"},
		{Priority(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.priority.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.priority.String())
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxWorkers != 10 {
		t.Errorf("expected max workers 10, got %d", config.MaxWorkers)
	}

	if config.MaxQueueSize != 1000 {
		t.Errorf("expected max queue size 1000, got %d", config.MaxQueueSize)
	}

	if config.DefaultTimeout != 30*time.Minute {
		t.Errorf("expected default timeout 30m, got %v", config.DefaultTimeout)
	}

	if !config.EnableMetrics {
		t.Error("expected metrics to be enabled")
	}
}

func TestNewQueue(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		return nil
	}

	config := DefaultConfig()
	queue, err := NewQueue(config, handler)

	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}

	if queue == nil {
		t.Fatal("expected queue to be created")
	}

	if queue.config != config {
		t.Error("expected config to be set")
	}

	if len(queue.workers) != config.MaxWorkers {
		t.Errorf("expected %d workers, got %d", config.MaxWorkers, len(queue.workers))
	}

	// Cleanup
	_ = queue.Shutdown(context.Background())
}

func TestNewQueueNilConfig(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		return nil
	}

	queue, err := NewQueue(nil, handler)

	if err != nil {
		t.Fatalf("failed to create queue with nil config: %v", err)
	}

	if queue.config == nil {
		t.Error("expected default config to be set")
	}

	_ = queue.Shutdown(context.Background())
}

func TestNewQueueNilHandler(t *testing.T) {
	_, err := NewQueue(DefaultConfig(), nil)

	if err == nil {
		t.Error("expected error with nil handler")
	}
}

func TestNewQueueInvalidMaxWorkers(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 0

	handler := func(ctx context.Context, job *Job) error {
		return nil
	}

	_, err := NewQueue(config, handler)

	if err == nil {
		t.Error("expected error with invalid max workers")
	}
}

func TestEnqueue(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		<-ctx.Done()
		return ctx.Err()
	}

	config := DefaultConfig()
	config.MaxWorkers = 2
	queue, err := NewQueue(config, handler)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = queue.Shutdown(ctx)
	}()

	// Make all workers busy
	for i := 0; i < config.MaxWorkers; i++ {
		_ = queue.Enqueue(&Job{ID: fmt.Sprintf("blocker-%d", i), Priority: PriorityLow})
	}
	time.Sleep(100 * time.Millisecond) // Let workers pick up blocker jobs

	job := &Job{
		ID:       "test-1",
		Priority: PriorityNormal,
		Payload:  "test payload",
	}

	err = queue.Enqueue(job)
	if err != nil {
		t.Errorf("failed to enqueue job: %v", err)
	}

	if queue.Size() != 1 {
		t.Errorf("expected queue size 1, got %d", queue.Size())
	}

	metrics := queue.GetMetrics()
	if metrics.JobsEnqueued != 3 { // 2 blockers + 1 test job
		t.Errorf("expected 3 enqueued jobs, got %d", metrics.JobsEnqueued)
	}
}

func TestEnqueueDefaults(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		return nil
	}

	queue, _ := NewQueue(DefaultConfig(), handler)
	defer func() { _ = queue.Shutdown(context.Background()) }()

	job := &Job{
		ID:       "test-1",
		Priority: PriorityNormal,
	}

	_ = queue.Enqueue(job)

	// Check defaults were set
	if job.SubmittedAt.IsZero() {
		t.Error("expected SubmittedAt to be set")
	}

	if job.Timeout == 0 {
		t.Error("expected Timeout to be set to default")
	}
}

func TestEnqueueFullQueue(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		// Block until cancelled
		<-ctx.Done()
		return ctx.Err()
	}

	config := DefaultConfig()
	config.MaxQueueSize = 5
	config.MaxWorkers = 1

	queue, _ := NewQueue(config, handler)

	// Fill the queue - worker will dequeue 1, leaving 4 in queue
	for i := 0; i < 5; i++ {
		job := &Job{
			ID:       string(rune('a' + i)),
			Priority: PriorityNormal,
		}
		_ = queue.Enqueue(job)
	}

	// Wait for worker to dequeue first job
	time.Sleep(100 * time.Millisecond)

	// Now add one more to fill the queue (5th in queue)
	_ = queue.Enqueue(&Job{ID: "fill", Priority: PriorityNormal})

	// Now try to overflow
	job := &Job{
		ID:       "overflow",
		Priority: PriorityNormal,
	}

	err := queue.Enqueue(job)
	if err == nil {
		t.Error("expected error when queue is full")
	}

	// Shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = queue.Shutdown(ctx)
}

func TestDequeue(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		<-ctx.Done()
		return ctx.Err()
	}

	config := DefaultConfig()
	config.MaxWorkers = 2
	queue, _ := NewQueue(config, handler)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = queue.Shutdown(ctx)
	}()

	// Make all workers busy
	for i := 0; i < config.MaxWorkers; i++ {
		_ = queue.Enqueue(&Job{ID: fmt.Sprintf("blocker-%d", i), Priority: PriorityLow})
	}
	time.Sleep(100 * time.Millisecond) // Let workers pick up blocker jobs

	job := &Job{
		ID:       "test-1",
		Priority: PriorityNormal,
	}

	_ = queue.Enqueue(job)

	dequeued, err := queue.Dequeue()
	if err != nil {
		t.Errorf("failed to dequeue: %v", err)
	}

	if dequeued.ID != "test-1" {
		t.Errorf("expected job ID 'test-1', got %s", dequeued.ID)
	}

	if queue.Size() != 0 {
		t.Errorf("expected queue size 0, got %d", queue.Size())
	}
}

func TestDequeueEmpty(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		return nil
	}

	queue, _ := NewQueue(DefaultConfig(), handler)
	defer func() { _ = queue.Shutdown(context.Background()) }()

	_, err := queue.Dequeue()
	if err == nil {
		t.Error("expected error when dequeuing from empty queue")
	}
}

func TestPriorityOrdering(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		<-ctx.Done()
		return ctx.Err()
	}

	config := DefaultConfig()
	config.MaxWorkers = 2
	queue, _ := NewQueue(config, handler)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = queue.Shutdown(ctx)
	}()

	// First, make all workers busy
	for i := 0; i < config.MaxWorkers; i++ {
		_ = queue.Enqueue(&Job{ID: fmt.Sprintf("blocker-%d", i), Priority: PriorityLow})
	}
	time.Sleep(200 * time.Millisecond) // Let workers pick up blocker jobs

	// Now enqueue jobs with different priorities
	jobs := []*Job{
		{ID: "low", Priority: PriorityLow},
		{ID: "critical", Priority: PriorityCritical},
		{ID: "normal", Priority: PriorityNormal},
		{ID: "high", Priority: PriorityHigh},
	}

	for _, job := range jobs {
		_ = queue.Enqueue(job)
	}

	// Dequeue should return in priority order
	expectedOrder := []string{"critical", "high", "normal", "low"}

	for i, expected := range expectedOrder {
		job, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("failed to dequeue job %d: %v", i, err)
		}

		if job.ID != expected {
			t.Errorf("job %d: expected %s, got %s", i, expected, job.ID)
		}
	}
}

func TestFIFOWithinPriority(t *testing.T) {
	// Use a blocking handler to prevent workers from consuming jobs
	startChan := make(chan struct{})
	handler := func(ctx context.Context, job *Job) error {
		<-startChan
		return nil
	}

	config := DefaultConfig()
	config.MaxWorkers = 2
	queue, _ := NewQueue(config, handler)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = queue.Shutdown(ctx)
	}()

	// Make all workers busy
	for i := 0; i < config.MaxWorkers; i++ {
		_ = queue.Enqueue(&Job{ID: fmt.Sprintf("blocker-%d", i), Priority: PriorityLow})
	}
	time.Sleep(200 * time.Millisecond) // Let workers pick up blocker jobs

	// Enqueue jobs with same priority
	for i := 0; i < 5; i++ {
		job := &Job{
			ID:       string(rune('a' + i)),
			Priority: PriorityNormal,
		}
		_ = queue.Enqueue(job)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	// Dequeue and verify FIFO order
	expectedOrder := []string{"a", "b", "c", "d", "e"}
	for i, expected := range expectedOrder {
		job, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("failed to dequeue job %d: %v", i, err)
		}
		if job.ID != expected {
			t.Errorf("job %d: expected %s, got %s", i, expected, job.ID)
		}
	}

	// Unblock workers
	close(startChan)
}

func TestJobExecution(t *testing.T) {
	executed := make(chan string, 10)

	handler := func(ctx context.Context, job *Job) error {
		executed <- job.ID
		return nil
	}

	config := DefaultConfig()
	config.MaxWorkers = 2

	queue, _ := NewQueue(config, handler)
	defer func() { _ = queue.Shutdown(context.Background()) }()

	// Enqueue jobs
	for i := 0; i < 5; i++ {
		job := &Job{
			ID:       string(rune('a' + i)),
			Priority: PriorityNormal,
		}
		_ = queue.Enqueue(job)
	}

	// Wait for jobs to execute
	timeout := time.After(5 * time.Second)
	count := 0

	for count < 5 {
		select {
		case <-executed:
			count++
		case <-timeout:
			t.Fatalf("timeout waiting for jobs, executed %d/5", count)
		}
	}

	// Wait a bit for metrics to be collected
	time.Sleep(100 * time.Millisecond)

	metrics := queue.GetMetrics()
	if metrics.JobsCompleted != 5 {
		t.Errorf("expected 5 completed jobs, got %d", metrics.JobsCompleted)
	}
}

func TestJobTimeout(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	}

	config := DefaultConfig()
	config.DefaultTimeout = 100 * time.Millisecond

	queue, _ := NewQueue(config, handler)
	defer func() { _ = queue.Shutdown(context.Background()) }()

	job := &Job{
		ID:       "timeout-test",
		Priority: PriorityNormal,
	}

	_ = queue.Enqueue(job)

	// Wait for timeout
	time.Sleep(500 * time.Millisecond)

	metrics := queue.GetMetrics()
	if metrics.JobsTimeout == 0 {
		t.Error("expected timeout to be recorded")
	}
}

func TestJobRetry(t *testing.T) {
	attempts := 0
	var mu sync.Mutex

	handler := func(ctx context.Context, job *Job) error {
		mu.Lock()
		attempts++
		currentAttempt := attempts
		mu.Unlock()

		if currentAttempt < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	queue, _ := NewQueue(DefaultConfig(), handler)
	defer func() { _ = queue.Shutdown(context.Background()) }()

	job := &Job{
		ID:         "retry-test",
		Priority:   PriorityNormal,
		MaxRetries: 3,
	}

	_ = queue.Enqueue(job)

	// Wait for retries
	time.Sleep(5 * time.Second)

	mu.Lock()
	finalAttempts := attempts
	mu.Unlock()

	if finalAttempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", finalAttempts)
	}

	metrics := queue.GetMetrics()
	if metrics.JobsRetried == 0 {
		t.Error("expected retries to be recorded")
	}
}

func TestConcurrentEnqueue(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	queue, _ := NewQueue(DefaultConfig(), handler)
	defer func() { _ = queue.Shutdown(context.Background()) }()

	var wg sync.WaitGroup
	jobCount := 100

	// Enqueue jobs concurrently
	for i := 0; i < jobCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			job := &Job{
				ID:       string(rune('a' + id%26)),
				Priority: Priority(id % 4),
			}
			_ = queue.Enqueue(job)
		}(i)
	}

	wg.Wait()

	metrics := queue.GetMetrics()
	if metrics.JobsEnqueued != int64(jobCount) {
		t.Errorf("expected %d enqueued jobs, got %d", jobCount, metrics.JobsEnqueued)
	}
}

func TestIsEmpty(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		<-ctx.Done()
		return ctx.Err()
	}

	config := DefaultConfig()
	config.MaxWorkers = 2
	queue, _ := NewQueue(config, handler)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = queue.Shutdown(ctx)
	}()

	if !queue.IsEmpty() {
		t.Error("expected queue to be empty")
	}

	// Make all workers busy
	for i := 0; i < config.MaxWorkers; i++ {
		_ = queue.Enqueue(&Job{ID: fmt.Sprintf("blocker-%d", i), Priority: PriorityLow})
	}
	time.Sleep(100 * time.Millisecond) // Let workers pick up blocker jobs

	_ = queue.Enqueue(&Job{ID: "test", Priority: PriorityNormal})

	if queue.IsEmpty() {
		t.Error("expected queue to not be empty")
	}
}

func TestIsFull(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		<-ctx.Done()
		return ctx.Err()
	}

	config := DefaultConfig()
	config.MaxQueueSize = 5
	config.MaxWorkers = 2

	queue, _ := NewQueue(config, handler)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = queue.Shutdown(ctx)
	}()

	if queue.IsFull() {
		t.Error("expected queue to not be full")
	}

	// First, make all workers busy
	for i := 0; i < config.MaxWorkers; i++ {
		_ = queue.Enqueue(&Job{ID: fmt.Sprintf("blocker-%d", i), Priority: PriorityLow})
	}
	time.Sleep(100 * time.Millisecond) // Let workers pick up blocker jobs

	// Now fill the queue
	for i := 0; i < 5; i++ {
		_ = queue.Enqueue(&Job{ID: string(rune('a' + i)), Priority: PriorityNormal})
	}

	if !queue.IsFull() {
		t.Error("expected queue to be full")
	}
}

func TestGetMetrics(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		return nil
	}

	queue, _ := NewQueue(DefaultConfig(), handler)
	defer func() { _ = queue.Shutdown(context.Background()) }()

	_ = queue.Enqueue(&Job{ID: "test", Priority: PriorityNormal})

	metrics := queue.GetMetrics()

	if metrics.JobsEnqueued != 1 {
		t.Errorf("expected 1 enqueued job, got %d", metrics.JobsEnqueued)
	}

	if metrics.CurrentQueueSize <= 0 {
		t.Errorf("expected positive queue size, got %d", metrics.CurrentQueueSize)
	}
}

func TestShutdown(t *testing.T) {
	handler := func(ctx context.Context, job *Job) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	queue, _ := NewQueue(DefaultConfig(), handler)

	// Enqueue some jobs
	for i := 0; i < 5; i++ {
		_ = queue.Enqueue(&Job{ID: string(rune('a' + i)), Priority: PriorityNormal})
	}

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := queue.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

func TestShutdownTimeout(t *testing.T) {
	started := make(chan bool, 1)

	handler := func(ctx context.Context, job *Job) error {
		started <- true
		time.Sleep(10 * time.Second) // Long running
		return nil
	}

	config := DefaultConfig()
	config.MaxWorkers = 1

	queue, _ := NewQueue(config, handler)

	// Enqueue job
	_ = queue.Enqueue(&Job{ID: "long-job", Priority: PriorityNormal})

	// Wait for job to start
	<-started

	// Shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := queue.Shutdown(ctx)
	if err == nil {
		t.Error("expected timeout error")
	}
}
