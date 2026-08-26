# HyperSDK Monitoring

Comprehensive monitoring and alerting setup for HyperSDK using Prometheus and Grafana.

## Overview

This monitoring stack provides:

- **Prometheus**: Metrics collection and storage
- **Grafana**: Visualization dashboards
- **Alertmanager**: Alert routing and notification
- **Alert Rules**: Pre-configured alerts for critical conditions

## Quick Start

### Docker Compose

```bash
cd monitoring
docker-compose up -d
```

Access the services:
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Alertmanager**: http://localhost:9093

### Kubernetes

```bash
kubectl apply -f monitoring/k8s/
```

## Dashboards

### HyperSDK Overview

**File**: `grafana/dashboards/transiva-overview.json`

Provides a high-level view of the HyperSDK service:

- **Service Status**: Uptime and health
- **Active Jobs**: Current running jobs
- **Completed/Failed Jobs**: Success metrics
- **HTTP Request Rate**: API traffic
- **HTTP Latency**: p50, p95 percentiles
- **Resource Usage**: CPU, memory, goroutines
- **Rate Limiting**: Rejection rates

**Key Metrics**:
- `hypersdk_jobs_total{status}` - Total jobs by status
- `hypersdk_http_requests_total` - HTTP request counter
- `hypersdk_http_request_duration_seconds` - Request latency histogram
- `process_resident_memory_bytes` - Memory usage
- `process_cpu_seconds_total` - CPU usage
- `go_goroutines` - Goroutine count

### HyperSDK Jobs

**File**: `grafana/dashboards/transiva-jobs.json`

Detailed job execution metrics:

- **Job Duration**: p50, p95, p99 percentiles
- **Job Completion Rate**: Success/failure rates
- **Job Progress**: Average progress for running jobs
- **Queue Length**: Pending jobs backlog
- **VMDK Transfer Rate**: Disk transfer speeds
- **Jobs by Provider**: AWS, Azure, GCP breakdown

**Key Metrics**:
- `hypersdk_job_duration_seconds` - Job execution time histogram
- `hypersdk_job_completions_total{status}` - Job completion counter
- `hypersdk_job_progress{status}` - Current job progress (0-100)
- `hypersdk_job_queue_length` - Queue backlog
- `hypersdk_vmdk_bytes_transferred_total` - VMDK transfer counter

## Alert Rules

### Service Availability

**HyperSDKDown**
- **Severity**: Critical
- **Condition**: Service down for >1 minute
- **Action**: Immediate investigation required

### Job Monitoring

**HighJobFailureRate**
- **Severity**: Warning
- **Condition**: >10% job failure rate over 5 minutes
- **Action**: Check logs for error patterns

**JobStuckInRunning**
- **Severity**: Warning
- **Condition**: Job running for >1 hour
- **Action**: Check job status, consider manual intervention

**QueueBacklog**
- **Severity**: Warning
- **Condition**: >100 jobs in queue for >10 minutes
- **Action**: Scale workers or investigate slow jobs

### HTTP Performance

**HighHTTPLatency**
- **Severity**: Warning
- **Condition**: p95 latency >5 seconds for 5 minutes
- **Action**: Check database, network, or application performance

**HighHTTPErrorRate**
- **Severity**: Warning
- **Condition**: >5% 5xx errors over 5 minutes
- **Action**: Check application logs and health

### Resource Usage

**HighMemoryUsage**
- **Severity**: Warning
- **Condition**: Memory >2GB for 5 minutes
- **Action**: Check for memory leaks

**HighCPUUsage**
- **Severity**: Warning
- **Condition**: CPU >80% for 10 minutes
- **Action**: Scale horizontally or optimize code

**TooManyGoroutines**
- **Severity**: Warning
- **Condition**: >10,000 goroutines for 10 minutes
- **Action**: Check for goroutine leaks

### Provider Integration

**ProviderAPIErrors**
- **Severity**: Warning
- **Condition**: >1 API error/s over 5 minutes
- **Action**: Check provider credentials and quotas

**SlowVMDKTransfer**
- **Severity**: Warning
- **Condition**: Transfer rate <10MB/s for 15 minutes
- **Action**: Check network bandwidth and storage I/O

## Metrics Exposed

### Job Metrics

```prometheus
# Job counters
hypersdk_jobs_total{status, provider}
hypersdk_job_completions_total{status}

# Job duration
hypersdk_job_duration_seconds{bucket, le}

# Job progress
hypersdk_job_progress{job_id, status}

# Queue metrics
hypersdk_job_queue_length
hypersdk_job_start_time{job_id, status}
```

### HTTP Metrics

```prometheus
# Request counters
hypersdk_http_requests_total{method, path, status}

# Request duration
hypersdk_http_request_duration_seconds{method, path, bucket, le}

# Request size
hypersdk_http_request_size_bytes{method, path, bucket, le}
hypersdk_http_response_size_bytes{method, path, bucket, le}
```

### Resource Metrics

```prometheus
# Process metrics (from Prometheus client)
process_resident_memory_bytes
process_cpu_seconds_total
process_open_fds

# Go metrics (from Prometheus client)
go_goroutines
go_memstats_alloc_bytes
go_memstats_heap_objects
```

### Rate Limiting Metrics

```prometheus
# Rate limit rejections
hypersdk_ratelimit_requests_rejected_total{user}

# Per-user limits
hypersdk_ratelimit_user_limit{user}
```

### Provider Metrics

```prometheus
# Provider API calls
hypersdk_provider_api_calls_total{provider, operation}

# Provider API errors
hypersdk_provider_api_errors_total{provider, operation, error_type}

# Provider API duration
hypersdk_provider_api_duration_seconds{provider, operation}
```

### Transfer Metrics

```prometheus
# VMDK transfer
hypersdk_vmdk_bytes_transferred_total{vm_name}
hypersdk_vmdk_transfer_duration_seconds{vm_name}
```

### Authentication Metrics

```prometheus
# Auth attempts
hypersdk_auth_attempts_total{method}
hypersdk_auth_failures_total{method, reason}
hypersdk_auth_sessions_active
```

### Secrets Management Metrics

```prometheus
# Secrets operations
hypersdk_secrets_operations_total{backend, operation}
hypersdk_secrets_manager_health{backend}
```

### Audit Log Metrics

```prometheus
# Audit events
hypersdk_audit_events_total{event_type, status}
hypersdk_audit_log_write_failures_total
```

## Configuration

### Prometheus

Edit `prometheus/prometheus.yaml` to configure:

- **Scrape interval**: How often to collect metrics
- **Retention**: How long to keep data
- **Alert rules**: Custom alert conditions
- **Remote storage**: For long-term storage

### Grafana

Dashboard provisioning:

```yaml
# grafana/provisioning/dashboards.yaml
apiVersion: 1

providers:
  - name: 'HyperSDK'
    folder: 'HyperSDK'
    type: file
    options:
      path: /etc/grafana/provisioning/dashboards
```

Datasource provisioning:

```yaml
# grafana/provisioning/datasources.yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    url: http://prometheus:9090
    isDefault: true
```

### Alertmanager

Configure alert routing in `alertmanager/config.yaml`:

```yaml
route:
  receiver: 'default'
  group_by: ['alertname', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h

receivers:
  - name: 'default'
    email_configs:
      - to: 'team@example.com'
        from: 'alerts@example.com'
        smarthost: 'smtp.example.com:587'

  - name: 'slack'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/...'
        channel: '#alerts'
        title: 'HyperSDK Alert'

  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: 'your-service-key'
```

## Docker Compose Setup

Create `docker-compose.yaml`:

```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus/prometheus.yaml:/etc/prometheus/prometheus.yml
      - ./prometheus/alerts.yaml:/etc/prometheus/alerts.yaml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=30d'
      - '--web.enable-lifecycle'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - ./grafana/provisioning/:/etc/grafana/provisioning/
      - ./grafana/dashboards/:/etc/grafana/provisioning/dashboards/
      - grafana-data:/var/lib/grafana
    restart: unless-stopped
    depends_on:
      - prometheus

  alertmanager:
    image: prom/alertmanager:latest
    container_name: alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager/config.yaml:/etc/alertmanager/config.yml
      - alertmanager-data:/alertmanager
    command:
      - '--config.file=/etc/alertmanager/config.yml'
      - '--storage.path=/alertmanager'
    restart: unless-stopped

  node-exporter:
    image: prom/node-exporter:latest
    container_name: node-exporter
    ports:
      - "9100:9100"
    restart: unless-stopped

volumes:
  prometheus-data:
  grafana-data:
  alertmanager-data:
```

## Kubernetes Deployment

### ServiceMonitor (Prometheus Operator)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: transiva
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: transiva
  endpoints:
    - port: metrics
      interval: 15s
      path: /metrics
```

### PrometheusRule

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: transiva-alerts
  namespace: monitoring
spec:
  groups:
    - name: transiva
      interval: 30s
      rules:
        # Import from prometheus/alerts.yaml
```

## Instrumenting Your Code

### Add Prometheus Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    jobsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "hypersdk_jobs_total",
            Help: "Total number of jobs",
        },
        []string{"status", "provider"},
    )

    jobDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "hypersdk_job_duration_seconds",
            Help: "Job execution duration",
            Buckets: prometheus.ExponentialBuckets(1, 2, 10),
        },
        []string{"provider"},
    )
)

// Expose metrics endpoint
http.Handle("/metrics", promhttp.Handler())
```

### Record Metrics

```go
// Increment counter
jobsTotal.WithLabelValues("completed", "aws").Inc()

// Record duration
timer := prometheus.NewTimer(jobDuration.WithLabelValues("aws"))
defer timer.ObserveDuration()

// Set gauge
jobQueueLength.Set(float64(len(queue)))
```

## Best Practices

1. **Use Labels Wisely**: Don't create high-cardinality labels
2. **Set Appropriate Retention**: Balance storage and data needs
3. **Configure Alerts Carefully**: Avoid alert fatigue
4. **Monitor the Monitors**: Ensure Prometheus/Grafana are healthy
5. **Use Recording Rules**: Pre-compute expensive queries
6. **Secure Endpoints**: Use authentication for metrics endpoints
7. **Document Custom Metrics**: Maintain metric documentation
8. **Test Alerts**: Trigger test alerts to verify notification paths

## Troubleshooting

### Metrics Not Appearing

1. Check Prometheus targets: http://localhost:9090/targets
2. Verify service is exposing `/metrics` endpoint
3. Check Prometheus logs for scrape errors
4. Verify network connectivity

### Dashboards Not Loading

1. Check Grafana datasource connection
2. Verify dashboard JSON syntax
3. Check Grafana provisioning logs
4. Ensure Prometheus has data

### Alerts Not Firing

1. Verify alert rules syntax
2. Check Prometheus alerts page: http://localhost:9090/alerts
3. Verify Alertmanager configuration
4. Check notification receiver settings

### High Cardinality

1. Review label values
2. Use recording rules to pre-aggregate
3. Set appropriate retention policies
4. Consider downsampling with Thanos/Cortex

## Advanced Features

### Recording Rules

Pre-compute expensive queries:

```yaml
groups:
  - name: hypersdk_recording_rules
    interval: 1m
    rules:
      - record: job:hypersdk_job_duration_seconds:p95
        expr: histogram_quantile(0.95, rate(hypersdk_job_duration_seconds_bucket[5m]))

      - record: job:hypersdk_http_requests:rate5m
        expr: rate(hypersdk_http_requests_total[5m])
```

### Federated Prometheus

For multi-cluster setups:

```yaml
scrape_configs:
  - job_name: 'federate'
    scrape_interval: 15s
    honor_labels: true
    metrics_path: '/federate'
    params:
      'match[]':
        - '{job="transiva"}'
    static_configs:
      - targets:
          - 'prometheus-1:9090'
          - 'prometheus-2:9090'
```

### Thanos Integration

For long-term storage and global view:

```bash
# Run Thanos sidecar
thanos sidecar \
  --prometheus.url=http://localhost:9090 \
  --tsdb.path=/prometheus/data \
  --objstore.config-file=bucket.yaml
```

## Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Alert Rules Best Practices](https://prometheus.io/docs/practices/alerting/)
- [Metric Naming](https://prometheus.io/docs/practices/naming/)

## License

SPDX-License-Identifier: Apache-2.0
