# Priority Job Queue

A production-ready priority-based job queue with concurrency control, automatic retries, and comprehensive metrics.

## Features

- **Priority-Based Scheduling**: Four priority levels (Critical, High, Normal, Low)
- **FIFO Within Priority**: Jobs with same priority execute in submission order
- **Concurrent Workers**: Configurable worker pool for parallel job execution
- **Automatic Retries**: Configurable retry mechanism with exponential backoff
- **Timeout Handling**: Per-job timeout with graceful cancellation
- **Metrics Collection**: Detailed metrics for monitoring and debugging
- **Thread-Safe**: Concurrent enqueue/dequeue operations
- **Graceful Shutdown**: Wait for in-progress jobs before shutdown

## Quick Start

### Basic Usage

```go
import "github.com/zyvorai/transiva/daemon/queue"

// Define job handler
handler := func(ctx context.Context, job *queue.Job) error {
    // Process job
    fmt.Printf("Processing job %s with payload: %v\n", job.ID, job.Payload)
    return nil
}

// Create queue
q, err := queue.NewQueue(queue.DefaultConfig(), handler)
if err != nil {
    log.Fatal(err)
}
defer q.Shutdown(context.Background())

// Enqueue jobs
job := &queue.Job{
    ID:       "job-1",
    Priority: queue.PriorityHigh,
    Payload:  "some data",
}

err = q.Enqueue(job)
if err != nil {
    log.Printf("Failed to enqueue: %v", err)
}
```

## Configuration

```go
type Config struct {
    MaxWorkers     int           // Number of concurrent workers
    MaxQueueSize   int           // Maximum jobs in queue
    DefaultTimeout time.Duration // Default job timeout
    EnableMetrics  bool          // Enable metrics collection
}
```

### Example Configurations

**Small Queue** (development/testing):
```go
config := &queue.Config{
    MaxWorkers:     2,
    MaxQueueSize:   100,
    DefaultTimeout: 5 * time.Minute,
    EnableMetrics:  true,
}
```

**Medium Queue** (production):
```go
config := &queue.Config{
    MaxWorkers:     10,
    MaxQueueSize:   1000,
    DefaultTimeout: 30 * time.Minute,
    EnableMetrics:  true,
}
```

**Large Queue** (high-throughput):
```go
config := &queue.Config{
    MaxWorkers:     50,
    MaxQueueSize:   10000,
    DefaultTimeout: 1 * time.Hour,
    EnableMetrics:  true,
}
```

## Priority Levels

Four priority levels available:

```go
queue.PriorityCritical  // Highest priority
queue.PriorityHigh      // High priority
queue.PriorityNormal    // Normal priority (default)
queue.PriorityLow       // Lowest priority
```

### Priority Behavior

- Higher priority jobs execute before lower priority jobs
- Within same priority: FIFO (First In, First Out)
- New high-priority jobs can jump ahead of queued low-priority jobs

### Example: Priority-Based Execution

```go
// These will execute in order: critical -> high -> normal -> low
jobs := []*queue.Job{
    {ID: "backup", Priority: queue.PriorityLow},
    {ID: "security-patch", Priority: queue.PriorityCritical},
    {ID: "data-sync", Priority: queue.PriorityNormal},
    {ID: "user-export", Priority: queue.PriorityHigh},
}

for _, job := range jobs {
    q.Enqueue(job)
}
```

## Job Structure

```go
type Job struct {
    ID          string        // Unique job identifier
    Priority    Priority      // Job priority level
    SubmittedAt time.Time     // Submission timestamp (auto-set)
    Payload     interface{}   // Job data/parameters
    RetryCount  int           // Current retry attempt
    MaxRetries  int           // Maximum retry attempts
    Timeout     time.Duration // Job execution timeout
}
```

### Creating Jobs

**Simple Job**:
```go
job := &queue.Job{
    ID:       "export-vm-001",
    Priority: queue.PriorityNormal,
    Payload:  vmConfig,
}
```

**Job with Retry**:
```go
job := &queue.Job{
    ID:         "fetch-data",
    Priority:   queue.PriorityHigh,
    Payload:    apiEndpoint,
    MaxRetries: 3,
}
```

**Job with Custom Timeout**:
```go
job := &queue.Job{
    ID:       "long-process",
    Priority: queue.PriorityNormal,
    Payload:  largeDataset,
    Timeout:  2 * time.Hour,
}
```

## Job Handler

The job handler is a function that processes each job:

```go
type JobHandler func(ctx context.Context, job *Job) error
```

### Handler Implementation

**Basic Handler**:
```go
handler := func(ctx context.Context, job *queue.Job) error {
    data := job.Payload.(MyData)

    // Process job
    result, err := processData(data)
    if err != nil {
        return err
    }

    // Save result
    return saveResult(result)
}
```

**Handler with Context**:
```go
handler := func(ctx context.Context, job *queue.Job) error {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Long-running operation
    for i := 0; i < 1000; i++ {
        // Check periodically
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        processChunk(i)
    }

    return nil
}
```

**Handler with Error Types**:
```go
handler := func(ctx context.Context, job *queue.Job) error {
    err := doWork(job.Payload)

    if err != nil {
        // Permanent error - don't retry
        if isPermanentError(err) {
            job.MaxRetries = 0
            return err
        }

        // Temporary error - will retry
        return err
    }

    return nil
}
```

## Retries

Jobs can automatically retry on failure:

```go
job := &queue.Job{
    ID:         "api-call",
    Priority:   queue.PriorityHigh,
    MaxRetries: 5,  // Retry up to 5 times
}
```

### Retry Behavior

- Retry occurs after handler returns error
- Exponential backoff: wait time = 1s × retry_count
- Job re-enqueued with updated retry count
- After max retries, job fails permanently

### Retry Example

```go
attempts := 0

handler := func(ctx context.Context, job *queue.Job) error {
    attempts++

    if attempts < 3 {
        return errors.New("temporary failure")
    }

    return nil  // Success on 3rd attempt
}

job := &queue.Job{
    ID:         "retry-test",
    MaxRetries: 5,
}

q.Enqueue(job)
// Will succeed on 3rd attempt
```

## Timeouts

Each job can have a timeout:

```go
job := &queue.Job{
    ID:      "slow-job",
    Timeout: 10 * time.Minute,
}
```

### Timeout Behavior

- Context passed to handler has timeout
- Handler should respect context cancellation
- After timeout, job fails with timeout error
- Timeout jobs counted in metrics

### Timeout Example

```go
handler := func(ctx context.Context, job *queue.Job) error {
    // Respect context timeout
    select {
    case <-ctx.Done():
        return ctx.Err()  // Timeout or cancellation
    case <-doWork():
        return nil
    }
}

job := &queue.Job{
    ID:      "time-limited",
    Timeout: 30 * time.Second,
}
```

## Metrics

Queue provides comprehensive metrics:

```go
type Metrics struct {
    JobsEnqueued      int64         // Total jobs added
    JobsDequeued      int64         // Total jobs removed
    JobsCompleted     int64         // Successfully completed
    JobsFailed        int64         // Failed jobs
    JobsTimeout       int64         // Timed out jobs
    JobsRetried       int64         // Retry attempts
    CurrentQueueSize  int           // Current queue size
    ActiveWorkers     int           // Busy workers
    AverageWaitTime   time.Duration // Avg time in queue
    AverageProcessing time.Duration // Avg execution time
}
```

### Getting Metrics

```go
metrics := q.GetMetrics()

fmt.Printf("Queue size: %d\n", metrics.CurrentQueueSize)
fmt.Printf("Active workers: %d\n", metrics.ActiveWorkers)
fmt.Printf("Completed: %d\n", metrics.JobsCompleted)
fmt.Printf("Failed: %d\n", metrics.JobsFailed)
fmt.Printf("Avg processing: %v\n", metrics.AverageProcessing)
```

### Metrics Dashboard Integration

```go
// Update dashboard periodically
ticker := time.NewTicker(5 * time.Second)
go func() {
    for range ticker.C {
        metrics := q.GetMetrics()
        dashboard.UpdateQueueMetrics(
            metrics.CurrentQueueSize,
            metrics.ActiveWorkers,
            metrics.JobsCompleted,
            metrics.JobsFailed,
        )
    }
}()
```

## Queue Operations

### Check Queue Status

```go
// Is queue empty?
if q.IsEmpty() {
    fmt.Println("No jobs pending")
}

// Is queue full?
if q.IsFull() {
    fmt.Println("Queue at capacity")
}

// Current size
size := q.Size()
fmt.Printf("Queue has %d jobs\n", size)
```

### Graceful Shutdown

```go
// Shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
defer cancel()

err := q.Shutdown(ctx)
if err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

## Advanced Examples

### VM Export Queue

```go
type VMExportJob struct {
    VMName   string
    Provider string
    Options  map[string]string
}

handler := func(ctx context.Context, job *queue.Job) error {
    export := job.Payload.(VMExportJob)

    // Export VM
    exporter := providers.Get(export.Provider)
    return exporter.Export(ctx, export.VMName, export.Options)
}

config := &queue.Config{
    MaxWorkers:     5,
    MaxQueueSize:   100,
    DefaultTimeout: 2 * time.Hour,
}

q, _ := queue.NewQueue(config, handler)

// Enqueue critical export
q.Enqueue(&queue.Job{
    ID:       "export-prod-db",
    Priority: queue.PriorityCritical,
    Payload: VMExportJob{
        VMName:   "prod-db-01",
        Provider: "aws",
    },
    MaxRetries: 3,
})
```

### Batch Processing

```go
type BatchJob struct {
    Items []string
    ProcessFunc func(string) error
}

handler := func(ctx context.Context, job *queue.Job) error {
    batch := job.Payload.(BatchJob)

    for _, item := range batch.Items {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if err := batch.ProcessFunc(item); err != nil {
            return err
        }
    }

    return nil
}

// Enqueue batch
q.Enqueue(&queue.Job{
    ID:       "batch-001",
    Priority: queue.PriorityNormal,
    Payload: BatchJob{
        Items: []string{"a", "b", "c"},
        ProcessFunc: processItem,
    },
})
```

### Priority-Based Rate Limiting

```go
// Different priorities for different users
func submitUserJob(q *queue.Queue, userTier string, jobData interface{}) {
    var priority queue.Priority

    switch userTier {
    case "enterprise":
        priority = queue.PriorityCritical
    case "pro":
        priority = queue.PriorityHigh
    case "free":
        priority = queue.PriorityLow
    default:
        priority = queue.PriorityNormal
    }

    job := &queue.Job{
        ID:       generateID(),
        Priority: priority,
        Payload:  jobData,
    }

    q.Enqueue(job)
}
```

## Best Practices

1. **Use Appropriate Priorities**
   - Critical: System-critical operations, urgent fixes
   - High: Important user-facing operations
   - Normal: Regular background tasks
   - Low: Maintenance, cleanup, non-urgent tasks

2. **Set Reasonable Timeouts**
   - Consider job complexity
   - Account for network latency
   - Add buffer for retries
   - Monitor timeout metrics

3. **Handle Context Cancellation**
   ```go
   handler := func(ctx context.Context, job *queue.Job) error {
       for {
           select {
           case <-ctx.Done():
               return ctx.Err()
           default:
               // Do work
           }
       }
   }
   ```

4. **Configure Worker Count**
   - Too few: Jobs queue up
   - Too many: Resource contention
   - Rule of thumb: 1-2x CPU cores for CPU-bound
   - Higher for I/O-bound tasks

5. **Monitor Metrics**
   - Track queue size growth
   - Watch failure rates
   - Monitor timeout rates
   - Alert on anomalies

6. **Graceful Degradation**
   ```go
   if q.IsFull() {
       // Return error to user
       return errors.New("system busy, try again later")
   }
   ```

7. **Idempotent Handlers**
   - Jobs may retry
   - Handlers should be idempotent
   - Check if work already done

## Performance Tuning

### CPU-Bound Jobs

```go
config := &queue.Config{
    MaxWorkers:   runtime.NumCPU(),  // 1 worker per CPU
    MaxQueueSize: 500,
}
```

### I/O-Bound Jobs

```go
config := &queue.Config{
    MaxWorkers:   runtime.NumCPU() * 4,  // More workers
    MaxQueueSize: 2000,
}
```

### Memory Constraints

```go
config := &queue.Config{
    MaxWorkers:   5,   // Limit concurrent memory usage
    MaxQueueSize: 100, // Limit queue memory
}
```

## Troubleshooting

### Jobs Not Processing

1. Check worker count: `metrics.ActiveWorkers`
2. Verify handler not blocking
3. Check for context cancellation
4. Review timeout settings

### Queue Growing

1. Increase worker count
2. Optimize handler performance
3. Check for failed jobs retrying
4. Review job priorities

### High Failure Rate

1. Check metrics.JobsFailed
2. Review handler error handling
3. Verify resource availability
4. Check timeout settings

### Memory Usage

1. Limit queue size
2. Clear completed jobs
3. Reduce worker count
4. Optimize job payload size

## Testing

```bash
# Run tests
go test ./daemon/queue/...

# Run with race detection
go test ./daemon/queue/... -race

# Benchmarks
go test ./daemon/queue/... -bench=.
```

## License

SPDX-License-Identifier: Apache-2.0
