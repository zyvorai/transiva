# HyperSDK Logger

Production-grade structured logging for HyperSDK with support for both text and JSON formats.

## Features

- **Structured logging** with key-value pairs
- **Log levels**: DEBUG, INFO, WARN, ERROR with filtering
- **Multiple output formats**: Text (human-readable) and JSON (machine-parseable)
- **Configurable output** destination (stdout, stderr, file)
- **Thread-safe** for concurrent goroutines
- **96.7% test coverage**

## Quick Start

### Basic Text Logging

```go
import "github.com/zyvorai/transiva/logger"

// Create a logger with INFO level (default format is text)
log := logger.New("info")

// Log messages with structured fields
log.Info("VM export started", "vm_path", "/datacenter/vm/web01", "job_id", "abc123")
log.Warn("Export slow", "progress", 45.5, "eta_seconds", 120)
log.Error("Export failed", "vm_path", "/datacenter/vm/db01", "error", "timeout")
```

**Output (Text Format):**
```
[2026-01-27 03:41:47] INFO: VM export started | vm_path=/datacenter/vm/web01, job_id=abc123
[2026-01-27 03:41:47] WARN: Export slow | progress=45.5, eta_seconds=120
[2026-01-27 03:41:47] ERROR: Export failed | vm_path=/datacenter/vm/db01, error=timeout
```

### JSON Logging for Production

```go
import (
    "os"
    "github.com/zyvorai/transiva/logger"
)

// Create a JSON logger for log aggregation (ELK, Splunk, Datadog)
log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",
    Output: os.Stdout,
})

log.Info("VM export completed",
    "vm_path", "/datacenter/vm/web01",
    "job_id", "abc123",
    "duration_seconds", 245.7,
    "bytes_transferred", 10737418240)
```

**Output (JSON Format):**
```json
{"timestamp":"2026-01-27T03:41:47Z","level":"INFO","msg":"VM export completed","vm_path":"/datacenter/vm/web01","job_id":"abc123","duration_seconds":245.7,"bytes_transferred":10737418240}
```

## Configuration

### Log Levels

```go
// DEBUG - Most verbose, includes all messages
log := logger.New("debug")

// INFO - General information (default)
log := logger.New("info")

// WARN - Warning messages only
log := logger.New("warn")

// ERROR - Error messages only
log := logger.New("error")
```

### Output Formats

#### Text Format (Human-Readable)
```go
log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "text",  // Default
    Output: os.Stderr,
})
```

#### JSON Format (Machine-Parseable)
```go
log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",  // For log aggregators
    Output: os.Stdout,
})
```

### Output Destinations

#### Standard Error (Default)
```go
log := logger.New("info")  // Outputs to stderr
```

#### Standard Output
```go
log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",
    Output: os.Stdout,
})
```

#### File Output
```go
file, err := os.OpenFile("/var/log/transiva/app.log",
    os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
if err != nil {
    panic(err)
}
defer file.Close()

log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",
    Output: file,
})
```

## Usage Examples

### API Server Logging

```go
type Server struct {
    logger logger.Logger
}

func NewServer() *Server {
    return &Server{
        logger: logger.NewWithConfig(logger.Config{
            Level:  "info",
            Format: "json",
            Output: os.Stdout,
        }),
    }
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
    s.logger.Info("export request received",
        "method", r.Method,
        "path", r.URL.Path,
        "remote_addr", r.RemoteAddr)

    // ... handle request ...

    s.logger.Info("export request completed",
        "status", 200,
        "duration_ms", 1234)
}
```

### Job Management Logging

```go
func (jm *JobManager) StartJob(job *Job) {
    jm.logger.Info("job started",
        "job_id", job.ID,
        "job_type", job.Type,
        "vm_path", job.VMPath)

    go func() {
        err := jm.executeJob(job)
        if err != nil {
            jm.logger.Error("job failed",
                "job_id", job.ID,
                "error", err.Error(),
                "duration_seconds", time.Since(job.StartTime).Seconds())
        } else {
            jm.logger.Info("job completed",
                "job_id", job.ID,
                "duration_seconds", time.Since(job.StartTime).Seconds(),
                "bytes_transferred", job.BytesTransferred)
        }
    }()
}
```

### Error Handling

```go
func exportVM(vmPath string, log logger.Logger) error {
    log.Debug("connecting to vSphere", "vm_path", vmPath)

    client, err := vsphere.NewClient(vcenterURL)
    if err != nil {
        log.Error("vSphere connection failed",
            "vm_path", vmPath,
            "vcenter_url", vcenterURL,
            "error", err.Error())
        return err
    }
    defer client.Logout()

    log.Info("vSphere connection successful", "vm_path", vmPath)

    // ... export logic ...

    return nil
}
```

## Integration with Log Aggregators

### ELK Stack (Elasticsearch, Logstash, Kibana)

Configure JSON logging and pipe to Logstash:

```go
log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",
    Output: os.Stdout,
})
```

Run application:
```bash
./hypervisord 2>&1 | logstash -f /etc/logstash/conf.d/transiva.conf
```

Logstash config (`/etc/logstash/conf.d/transiva.conf`):
```
input {
  stdin {
    codec => json
  }
}

output {
  elasticsearch {
    hosts => ["localhost:9200"]
    index => "transiva-%{+YYYY.MM.dd}"
  }
}
```

### Splunk

```go
log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",
    Output: os.Stdout,
})
```

Configure Splunk Universal Forwarder to monitor the log file or stdout.

### Datadog

```go
import "os"

// Log to file for Datadog Agent to collect
file, _ := os.OpenFile("/var/log/transiva/app.log",
    os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",
    Output: file,
})
```

Configure Datadog Agent (`/etc/datadog-agent/conf.d/transiva.d/conf.yaml`):
```yaml
logs:
  - type: file
    path: /var/log/transiva/app.log
    service: transiva
    source: go
```

### CloudWatch Logs

Use AWS CloudWatch Logs agent with JSON format for structured queries.

## Testing

The logger package includes comprehensive tests with 96.7% coverage:

```bash
# Run all tests
go test ./logger

# Run with coverage
go test -coverprofile=coverage.out ./logger
go tool cover -html=coverage.out

# Run JSON logger tests only
go test -v ./logger -run TestJSON
```

## Best Practices

### 1. Use Structured Fields

**Good:**
```go
log.Info("VM export started", "vm_path", "/datacenter/vm/web01", "size_gb", 50)
```

**Bad:**
```go
log.Info(fmt.Sprintf("VM export started: vm_path=%s, size_gb=%d", vmPath, sizeGB))
```

### 2. Choose Appropriate Log Levels

- **DEBUG**: Detailed diagnostic information for debugging
- **INFO**: General informational messages (state changes, progress)
- **WARN**: Warning messages (recoverable errors, performance issues)
- **ERROR**: Error messages (failures that need attention)

### 3. Use JSON Format in Production

For production deployments, use JSON format for better integration with log aggregation tools:

```go
log := logger.NewWithConfig(logger.Config{
    Level:  "info",
    Format: "json",
    Output: os.Stdout,
})
```

### 4. Include Context in Logs

Always include relevant context (job ID, VM path, user ID, etc.):

```go
log.Error("export failed",
    "job_id", jobID,
    "vm_path", vmPath,
    "user_id", userID,
    "error", err.Error())
```

### 5. Log at Appropriate Times

- Log at state transitions (job started, job completed)
- Log errors with full context
- Avoid logging in tight loops (use sampling)

## Performance

The logger is designed for production use:

- **Thread-safe**: Safe to use from multiple goroutines
- **Efficient**: Minimal allocations for common cases
- **Non-blocking**: Logging doesn't block application logic
- **Configurable**: Only log what you need (level filtering)

## Migration Guide

### From Standard log Package

**Before:**
```go
import "log"

log.Printf("VM export started: %s", vmPath)
```

**After:**
```go
import "github.com/zyvorai/transiva/logger"

log := logger.New("info")
log.Info("VM export started", "vm_path", vmPath)
```

### From fmt.Println Debugging

**Before:**
```go
fmt.Printf("DEBUG: job_id=%s, progress=%f\n", jobID, progress)
```

**After:**
```go
log.Debug("job progress", "job_id", jobID, "progress", progress)
```

## License

Apache-2.0
