# OpenTelemetry Distributed Tracing

This package provides comprehensive distributed tracing support using OpenTelemetry for the transiva daemon.

## Features

- **Multiple Exporters**: Support for Jaeger, OTLP, and stdout exporters
- **HTTP Middleware**: Automatic tracing of HTTP requests with context propagation
- **Configurable Sampling**: Control trace sampling rates (0.0 to 1.0)
- **Trace Context Propagation**: W3C Trace Context and Baggage propagation
- **Helper Functions**: Convenient helpers for common tracing operations
- **Performance Tuning**: Configurable batch sizes and queue sizes

## Supported Exporters

### Jaeger
```go
config := &tracing.Config{
    Enabled:        true,
    ServiceName:    "transiva",
    Exporter:       "jaeger",
    JaegerEndpoint: "http://localhost:14268/api/traces",
}
```

### OTLP (OpenTelemetry Protocol)
```go
config := &tracing.Config{
    Enabled:      true,
    ServiceName:  "transiva",
    Exporter:     "otlp",
    OTLPEndpoint: "localhost:4317",
}
```

### Stdout (Development/Testing)
```go
config := &tracing.Config{
    Enabled:     true,
    ServiceName: "transiva",
    Exporter:    "stdout",
}
```

## Usage

### Basic Setup

```go
import "github.com/zyvorai/transiva/daemon/tracing"

// Create configuration
config := tracing.DefaultConfig("transiva")
config.Enabled = true
config.Exporter = "jaeger"
config.JaegerEndpoint = "http://localhost:14268/api/traces"

// Create provider
provider, err := tracing.NewProvider(config)
if err != nil {
    log.Fatal(err)
}
defer provider.Shutdown(context.Background())

// Get tracer
tracer := provider.Tracer("my-component")
```

### HTTP Middleware

```go
// Create middleware
middleware := tracing.NewHTTPMiddleware(tracer)

// Wrap your HTTP handler
handler := middleware.Handler(http.HandlerFunc(myHandler))

http.Handle("/api/jobs", handler)
```

### Manual Span Creation

```go
func processJob(ctx context.Context, tracer trace.Tracer, jobID string) error {
    // Start span
    ctx, span := tracer.Start(ctx, "process-job",
        trace.WithAttributes(
            tracing.AttrJobID.String(jobID),
        ),
    )
    defer span.End()

    // Add events
    tracing.AddEvent(ctx, "job-started")

    // Do work...
    err := doWork(ctx)
    if err != nil {
        // Record error
        tracing.RecordError(ctx, err)
        tracing.SetStatus(ctx, codes.Error, "job failed")
        return err
    }

    tracing.AddEvent(ctx, "job-completed")
    tracing.SetStatus(ctx, codes.Ok, "")
    return nil
}
```

### Helper Functions

```go
// Trace VM export job
ctx, span := tracing.TraceJobExport(ctx, tracer, "job123", "export-vm", "my-vm")
defer span.End()

// Trace provider operation
ctx, span := tracing.TraceProviderOperation(ctx, tracer, "aws", "export-instance")
defer span.End()

// Trace HTTP request
ctx, span := tracing.TraceHTTPRequest(ctx, tracer, "GET", "/api/jobs")
defer span.End()

// Trace database operation
ctx, span := tracing.TraceDBOperation(ctx, tracer, "SELECT", "jobs")
defer span.End()
```

### Trace Context Propagation

```go
// Client side - inject context into HTTP request
req, _ := http.NewRequest("GET", "http://api.example.com/data", nil)
tracing.InjectTraceContext(ctx, req)
resp, _ := http.DefaultClient.Do(req)

// Server side - extract context from HTTP request
ctx := tracing.ExtractTraceContext(context.Background(), req)
ctx, span := tracer.Start(ctx, "handle-request")
defer span.End()
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| Enabled | bool | false | Enable/disable tracing |
| ServiceName | string | - | Name of the service |
| ServiceVersion | string | "1.0.0" | Version of the service |
| Environment | string | "development" | Deployment environment |
| Exporter | string | "stdout" | Exporter type (jaeger, otlp, stdout) |
| JaegerEndpoint | string | "http://localhost:14268/api/traces" | Jaeger collector endpoint |
| OTLPEndpoint | string | "localhost:4317" | OTLP collector endpoint |
| SamplingRate | float64 | 1.0 | Trace sampling rate (0.0-1.0) |
| MaxExportBatchSize | int | 512 | Maximum batch size for export |
| MaxQueueSize | int | 2048 | Maximum queue size for spans |
| ExportTimeout | time.Duration | 30s | Timeout for exporting spans |

## Sampling

Control which traces are exported:

```go
config.SamplingRate = 1.0  // Always sample (100%)
config.SamplingRate = 0.5  // Sample 50% of traces
config.SamplingRate = 0.0  // Never sample (0%)
```

## Common Attributes

The package provides predefined attribute keys for common operations:

- `AttrJobID` - Job identifier
- `AttrJobName` - Job name
- `AttrJobStatus` - Job status
- `AttrVMName` - Virtual machine name
- `AttrVMPath` - Virtual machine path
- `AttrProvider` - Cloud provider name
- `AttrOperation` - Operation name
- `AttrUserID` - User identifier
- `AttrUserName` - User name
- `AttrHTTPMethod` - HTTP method
- `AttrHTTPPath` - HTTP path
- `AttrHTTPStatus` - HTTP status code
- `AttrErrorType` - Error type
- `AttrErrorMessage` - Error message

## Integration with Observability Stack

### Jaeger

1. Run Jaeger locally:
```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:latest
```

2. View traces at http://localhost:16686

### OpenTelemetry Collector

1. Configure collector with OTLP receiver
2. Point `OTLPEndpoint` to collector
3. Export to multiple backends (Jaeger, Zipkin, Prometheus, etc.)

## Best Practices

1. **Use meaningful span names**: Choose descriptive names that identify the operation
2. **Add relevant attributes**: Include context that helps understand the trace
3. **Record errors**: Always record errors on spans for debugging
4. **Use span kinds appropriately**:
   - `SpanKindServer` for HTTP handlers
   - `SpanKindClient` for outbound calls
   - `SpanKindInternal` for internal operations
5. **Propagate context**: Always pass context through function calls
6. **Close spans**: Use `defer span.End()` to ensure spans are closed
7. **Configure sampling in production**: Use sampling to reduce overhead

## Performance Considerations

- Tracing adds overhead to each operation
- Use sampling to reduce data volume in production
- Tune batch sizes and queue sizes based on load
- Monitor exporter performance
- Consider using asynchronous exporters

## Testing

```bash
# Run tests
go test ./daemon/tracing/...

# Run tests with coverage
go test ./daemon/tracing/... -cover

# Run tests with race detection
go test ./daemon/tracing/... -race
```

## Troubleshooting

### Traces not appearing

1. Verify `Enabled` is set to `true`
2. Check exporter endpoint is reachable
3. Verify sampling rate is > 0.0
4. Check exporter logs for errors

### High memory usage

1. Reduce `MaxQueueSize`
2. Reduce `MaxExportBatchSize`
3. Increase sampling (reduce `SamplingRate`)
4. Check for span leaks (spans not closed)

### Missing trace context

1. Ensure context is propagated through all function calls
2. Verify HTTP headers are forwarded
3. Check middleware is installed correctly

## License

SPDX-License-Identifier: Apache-2.0
