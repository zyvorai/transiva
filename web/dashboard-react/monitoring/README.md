# HyperSDK Monitoring Stack

Complete monitoring and observability solution for HyperSDK using Prometheus, Grafana, and AlertManager.

## Quick Start

```bash
# Start the monitoring stack
cd monitoring
docker-compose up -d

# Access dashboards
# Grafana: http://localhost:3000 (admin/admin)
# Prometheus: http://localhost:9090
# AlertManager: http://localhost:9093
```

The Grafana dashboards will be automatically provisioned and available immediately.

## Architecture

```
┌─────────────┐
│  HyperSDK   │ ──metrics──> ┌────────────┐
│   Daemon    │              │ Prometheus │
│  :8080      │              │   :9090    │
└─────────────┘              └──────┬─────┘
                                    │
                                    │ scrapes
                                    ▼
┌─────────────┐              ┌────────────┐
│ AlertManager│ <──alerts──  │  Grafana   │
│   :9093     │              │   :3000    │
└─────────────┘              └────────────┘
       │
       │ notifications
       ▼
   Email/Slack/PagerDuty
```

## Dashboards

### 1. HyperSDK - Overview
**Purpose**: High-level system health at a glance

**Panels**:
- **Active Jobs** - Current number of running jobs
- **Completed Jobs (24h)** - Total successful jobs in last 24 hours
- **Failed Jobs (24h)** - Total failed jobs in last 24 hours
- **Job Completion Rate** - Jobs completed/failed per hour (time series)
- **Queue Length & Active Jobs** - Job queue backlog and concurrent execution
- **Job Success Rate** - Gauge showing success percentage
- **Jobs by Provider** - Pie chart showing provider distribution
- **Memory Usage** - Memory consumption over time with thresholds

**Use Case**: Daily monitoring, quick health checks

---

### 2. HyperSDK - Job Performance
**Purpose**: Deep dive into job execution metrics

**Panels**:
- **Job Throughput** - Jobs completed per minute
- **Job Duration Percentiles** - p50/p95/p99 latency tracking
- **Average Duration by Provider** - Bar chart comparing provider performance
- **Job Status Distribution** - Pie chart of completed/failed/active jobs
- **Export Throughput** - Data transfer rate (bytes/second)
- **Provider Performance Summary** - Table with avg duration and success rate per provider

**Use Case**: Performance troubleshooting, capacity planning, SLA monitoring

**Key Queries**:
```promql
# p95 job duration
histogram_quantile(0.95, rate(hypersdk_job_duration_seconds_bucket[5m]))

# Average duration by provider
avg by (provider) (rate(hypersdk_job_duration_seconds_sum[5m]) / rate(hypersdk_job_duration_seconds_count[5m]))
```

---

### 3. HyperSDK - System Resources
**Purpose**: Infrastructure and resource monitoring

**Panels**:
- **Memory Usage** - With color-coded thresholds (green < 1.5GB, yellow < 2GB, red > 2GB)
- **CPU Usage** - Percentage of CPU utilization
- **Goroutines** - Go runtime goroutine count (detect leaks)
- **Active Connections** - WebSocket and HTTP connections
- **HTTP Request Rate** - Requests per second
- **HTTP Response Time** - p50/p95/p99 API latency

**Use Case**: Resource planning, leak detection, performance optimization

**Thresholds**:
- Memory Warning: 1.5 GB
- Memory Critical: 2 GB
- Goroutine Warning: 1000
- Goroutine Critical: 2000

---

## Alert Rules

### Job Performance Alerts

#### HighJobFailureRate
- **Severity**: warning
- **Threshold**: >10% failure rate
- **Duration**: 5 minutes
- **Description**: Job failure rate exceeds acceptable threshold

#### CriticalJobFailureRate
- **Severity**: critical
- **Threshold**: >25% failure rate
- **Duration**: 2 minutes
- **Description**: Critical failure rate requiring immediate attention

---

### Resource Alerts

#### HighMemoryUsage
- **Severity**: warning
- **Threshold**: >2 GB
- **Duration**: 5 minutes
- **Query**: `hypersdk_memory_bytes > 2147483648`

#### CriticalMemoryUsage
- **Severity**: critical
- **Threshold**: >4 GB
- **Duration**: 2 minutes
- **Query**: `hypersdk_memory_bytes > 4294967296`

#### GoroutineLeakSuspected
- **Severity**: warning
- **Threshold**: >1000 goroutines
- **Duration**: 15 minutes
- **Description**: Potential goroutine leak detected

#### GoroutineLeak
- **Severity**: critical
- **Threshold**: >2000 goroutines
- **Duration**: 5 minutes
- **Description**: Confirmed goroutine leak

---

### Queue Alerts

#### LargeJobQueue
- **Severity**: warning
- **Threshold**: >50 pending jobs
- **Duration**: 10 minutes
- **Description**: Job queue backlog building up

#### MassiveJobQueue
- **Severity**: critical
- **Threshold**: >100 pending jobs
- **Duration**: 5 minutes
- **Description**: Critical queue backlog

---

### API Performance Alerts

#### HighHTTPErrorRate
- **Severity**: warning
- **Threshold**: >5% error rate
- **Duration**: 5 minutes
- **Query**: `rate(hypersdk_http_errors_total[5m]) / rate(hypersdk_http_requests_total[5m]) > 0.05`

#### SlowAPIResponses
- **Severity**: warning
- **Threshold**: p95 >1 second
- **Duration**: 5 minutes
- **Query**: `histogram_quantile(0.95, rate(hypersdk_http_response_time_seconds_bucket[5m])) > 1`

---

### Provider Alerts

#### ProviderJobFailures
- **Severity**: warning
- **Threshold**: >20% failure rate per provider
- **Duration**: 5 minutes
- **Description**: High failure rate for specific provider

---

### Health Alerts

#### DaemonDown
- **Severity**: critical
- **Threshold**: unreachable
- **Duration**: 1 minute
- **Query**: `up{job="transiva"} == 0`
- **Description**: HyperSDK daemon is not responding

---

## Configuration

### Prometheus Configuration

Edit `prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s      # How often to scrape metrics
  evaluation_interval: 15s  # How often to evaluate rules

scrape_configs:
  - job_name: 'transiva'
    static_configs:
      - targets:
          - 'hypervisord:8080'  # Change to your daemon host:port
```

**For remote daemon**:
```yaml
scrape_configs:
  - job_name: 'transiva'
    static_configs:
      - targets:
          - '192.168.1.100:8080'
```

---

### Alert Thresholds

Edit `prometheus/alerts.yml` to customize thresholds:

```yaml
- alert: HighMemoryUsage
  expr: hypersdk_memory_bytes > 2147483648  # Change to your threshold
  for: 5m                                    # Change alert duration
```

**Common threshold adjustments**:
- Memory: Adjust based on available RAM
- Goroutines: Adjust based on typical workload
- Queue length: Adjust based on processing capacity
- Job failure rate: Adjust based on SLA requirements

---

### Notification Channels

Edit `alertmanager/config.yml` to configure notifications:

#### Email Notifications

Uncomment and configure:
```yaml
global:
  smtp_smarthost: 'smtp.gmail.com:587'
  smtp_from: 'alerts@transiva.example.com'
  smtp_auth_username: 'your-email@gmail.com'
  smtp_auth_password: 'your-app-password'

receivers:
  - name: 'critical'
    email_configs:
      - to: 'oncall@example.com'
        headers:
          Subject: '[CRITICAL] HyperSDK Alert: {{ .GroupLabels.alertname }}'
```

#### Slack Notifications

```yaml
receivers:
  - name: 'critical'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
        channel: '#transiva-alerts'
        title: '[CRITICAL] {{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
        send_resolved: true
```

**Get Slack webhook URL**:
1. Go to https://api.slack.com/apps
2. Create new app → Incoming Webhooks
3. Add webhook to workspace
4. Copy webhook URL

#### PagerDuty Integration

```yaml
receivers:
  - name: 'critical'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_SERVICE_KEY'
        description: '{{ .GroupLabels.alertname }}: {{ .CommonAnnotations.summary }}'
```

**Get PagerDuty service key**:
1. Go to PagerDuty → Services
2. Create or select service
3. Integrations → Add integration → Events API v2
4. Copy Integration Key

---

### Alert Routing

Configure which alerts go to which receivers:

```yaml
route:
  receiver: 'default'
  routes:
    # Critical alerts to PagerDuty
    - receiver: 'pagerduty'
      match:
        severity: critical
      continue: false  # Stop processing after match

    # Warnings to Slack
    - receiver: 'slack-warnings'
      match:
        severity: warning
```

---

### Alert Inhibition

Suppress lower-severity alerts when higher-severity is firing:

```yaml
inhibit_rules:
  # Suppress warning if critical is firing for same alert
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'cluster', 'service']
```

This prevents alert fatigue - you won't get both a warning AND critical alert for the same issue.

---

## Grafana Configuration

### Accessing Grafana

1. Navigate to http://localhost:3000
2. Login: `admin` / `admin`
3. Change password on first login
4. Dashboards are pre-loaded in the "HyperSDK" folder

### Creating Custom Dashboards

1. Click "+" → "Dashboard"
2. Add panel
3. Select "Prometheus" as data source
4. Enter PromQL query
5. Choose visualization type

**Example PromQL queries**:

```promql
# Current active jobs
hypersdk_jobs_active

# Job completion rate (per hour)
rate(hypersdk_jobs_completed_total[5m]) * 3600

# Success rate percentage
(
  rate(hypersdk_jobs_completed_total[5m])
  /
  (rate(hypersdk_jobs_completed_total[5m]) + rate(hypersdk_jobs_failed_total[5m]))
) * 100

# Average job duration
rate(hypersdk_job_duration_seconds_sum[5m]) / rate(hypersdk_job_duration_seconds_count[5m])

# Jobs by provider
sum by (provider) (hypersdk_provider_jobs_total)

# p99 response time in milliseconds
histogram_quantile(0.99, rate(hypersdk_http_response_time_seconds_bucket[5m])) * 1000
```

### Exporting Dashboards

```bash
# Export dashboard JSON
curl -H "Authorization: Bearer <api-key>" \
  http://localhost:3000/api/dashboards/uid/transiva-overview | jq .dashboard > dashboard.json

# Import dashboard
curl -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer <api-key>" \
  -d @dashboard.json \
  http://localhost:3000/api/dashboards/db
```

---

## Production Deployment

### Security Hardening

1. **Change default passwords**:
```yaml
grafana:
  environment:
    - GF_SECURITY_ADMIN_PASSWORD=<strong-password>
```

2. **Enable HTTPS**:
```yaml
grafana:
  environment:
    - GF_SERVER_PROTOCOL=https
    - GF_SERVER_CERT_FILE=/etc/grafana/ssl/cert.pem
    - GF_SERVER_CERT_KEY=/etc/grafana/ssl/key.pem
  volumes:
    - ./ssl:/etc/grafana/ssl:ro
```

3. **Restrict Prometheus access** (add to `prometheus.yml`):
```yaml
global:
  external_labels:
    cluster: 'production'
    datacenter: 'us-east-1'

# Enable authentication
basic_auth_users:
  admin: <bcrypt-hashed-password>
```

4. **Network isolation**:
```yaml
networks:
  transiva-monitoring:
    driver: bridge
    internal: true  # No external access
```

---

### Data Retention

Configure retention based on storage capacity:

```yaml
prometheus:
  command:
    - '--storage.tsdb.retention.time=30d'  # Default: 30 days
    - '--storage.tsdb.retention.size=50GB' # Or size-based
```

**Retention recommendations**:
- Development: 7 days
- Staging: 15 days
- Production: 30-90 days

---

### High Availability

#### Prometheus HA

```yaml
services:
  prometheus-1:
    image: prom/prometheus:latest
    command:
      - '--storage.tsdb.path=/prometheus-1'
      - '--web.listen-address=:9090'

  prometheus-2:
    image: prom/prometheus:latest
    command:
      - '--storage.tsdb.path=/prometheus-2'
      - '--web.listen-address=:9091'
```

Both instances scrape the same targets independently.

#### Grafana HA

Use shared database:
```yaml
grafana:
  environment:
    - GF_DATABASE_TYPE=postgres
    - GF_DATABASE_HOST=postgres:5432
    - GF_DATABASE_NAME=grafana
    - GF_DATABASE_USER=grafana
    - GF_DATABASE_PASSWORD=<password>
```

---

### Backup and Restore

#### Backup Prometheus Data

```bash
# Create backup
docker run --rm \
  -v prometheus-data:/data \
  -v $(pwd)/backup:/backup \
  alpine tar czf /backup/prometheus-$(date +%Y%m%d).tar.gz /data

# Restore backup
docker run --rm \
  -v prometheus-data:/data \
  -v $(pwd)/backup:/backup \
  alpine tar xzf /backup/prometheus-20260121.tar.gz -C /
```

#### Backup Grafana Dashboards

```bash
# Backup all dashboards
mkdir -p grafana-backup
for dash in $(curl -H "Authorization: Bearer <api-key>" \
  http://localhost:3000/api/search | jq -r '.[].uid'); do
  curl -H "Authorization: Bearer <api-key>" \
    http://localhost:3000/api/dashboards/uid/$dash | \
    jq .dashboard > grafana-backup/$dash.json
done
```

#### Automated Backups

Add to `docker-compose.yml`:
```yaml
services:
  backup:
    image: alpine
    volumes:
      - prometheus-data:/prometheus:ro
      - grafana-data:/grafana:ro
      - ./backups:/backups
    command: |
      sh -c "
        while true; do
          tar czf /backups/prometheus-$(date +%Y%m%d-%H%M).tar.gz /prometheus
          tar czf /backups/grafana-$(date +%Y%m%d-%H%M).tar.gz /grafana
          find /backups -mtime +7 -delete
          sleep 86400
        done
      "
```

---

## Troubleshooting

### Prometheus Not Scraping Metrics

**Symptom**: No data in Grafana, "No data" on panels

**Check**:
```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | jq

# Check if daemon is exposing metrics
curl http://localhost:8080/metrics
```

**Fix**:
1. Verify HyperSDK daemon is running
2. Check `hypervisord:8080` is reachable from Prometheus container
3. Update target in `prometheus.yml` if using different host/port

---

### Grafana Dashboard Not Loading

**Symptom**: Empty dashboard folder, "Dashboard not found"

**Check**:
```bash
# Check provisioning logs
docker logs transiva-grafana | grep provisioning

# Verify dashboard files mounted
docker exec transiva-grafana ls -la /var/lib/grafana/dashboards
```

**Fix**:
1. Verify dashboard JSON files exist in `grafana/dashboards/`
2. Check file permissions (should be readable)
3. Restart Grafana: `docker-compose restart grafana`

---

### Alerts Not Firing

**Symptom**: No alerts in AlertManager despite threshold breach

**Check**:
```bash
# Check alert rules loaded
curl http://localhost:9090/api/v1/rules | jq

# Check active alerts
curl http://localhost:9090/api/v1/alerts | jq
```

**Fix**:
1. Verify `alerts.yml` is mounted: `docker exec transiva-prometheus cat /etc/prometheus/alerts.yml`
2. Check for syntax errors: `promtool check rules prometheus/alerts.yml`
3. Restart Prometheus: `docker-compose restart prometheus`

---

### Alerts Not Sending Notifications

**Symptom**: Alerts fire in Prometheus but no email/Slack/PagerDuty

**Check**:
```bash
# Check AlertManager config
curl http://localhost:9093/api/v2/status | jq

# Check active alerts in AlertManager
curl http://localhost:9093/api/v2/alerts | jq
```

**Fix**:
1. Verify AlertManager configuration: `amtool check-config alertmanager/config.yml`
2. Test notification channel:
   - Email: Check SMTP credentials
   - Slack: Test webhook URL with curl
   - PagerDuty: Verify service key
3. Check AlertManager logs: `docker logs transiva-alertmanager`

---

### High Prometheus Memory Usage

**Symptom**: Prometheus container using excessive memory

**Fix**:
1. Reduce retention: `--storage.tsdb.retention.time=15d`
2. Reduce scrape frequency: `scrape_interval: 30s`
3. Limit maximum samples: `--storage.tsdb.max-block-duration=2h`
4. Increase container memory limit in `docker-compose.yml`:
```yaml
prometheus:
  deploy:
    resources:
      limits:
        memory: 2G
```

---

### Missing Metrics

**Symptom**: Some metrics show "No data" but others work

**Check**:
```bash
# List all available metrics
curl http://localhost:8080/metrics | grep transiva

# Query specific metric in Prometheus
curl 'http://localhost:9090/api/v1/query?query=hypersdk_jobs_active'
```

**Fix**:
1. Verify metric is exposed by daemon
2. Check metric name spelling in dashboard query
3. Verify metric has recent data points
4. Check if metric is a histogram/summary (requires `_bucket`, `_sum`, `_count` suffixes)

---

## Metrics Reference

All metrics exposed by HyperSDK daemon:

### Job Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `hypersdk_jobs_active` | Gauge | Current number of running jobs |
| `hypersdk_jobs_completed_total` | Counter | Total completed jobs |
| `hypersdk_jobs_failed_total` | Counter | Total failed jobs |
| `hypersdk_jobs_pending` | Gauge | Jobs waiting in queue |
| `hypersdk_job_duration_seconds` | Histogram | Job execution duration |
| `hypersdk_queue_length` | Gauge | Current queue length |

### HTTP Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `hypersdk_http_requests_total` | Counter | Total HTTP requests |
| `hypersdk_http_errors_total` | Counter | Total HTTP errors |
| `hypersdk_http_response_time_seconds` | Histogram | HTTP response time |

### System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `hypersdk_memory_bytes` | Gauge | Memory usage in bytes |
| `hypersdk_cpu_usage` | Gauge | CPU usage percentage |
| `hypersdk_goroutines` | Gauge | Number of goroutines |
| `hypersdk_websocket_clients` | Gauge | Active WebSocket connections |

### Provider Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `hypersdk_provider_jobs_total` | Counter | Jobs per provider |
| `hypersdk_provider_jobs_failed` | Counter | Failed jobs per provider |

---

## Performance Tuning

### Optimize Scrape Interval

```yaml
# High-frequency monitoring (more load)
scrape_interval: 5s

# Balanced (recommended)
scrape_interval: 15s

# Low-frequency (less accurate)
scrape_interval: 60s
```

### Optimize Query Performance

Use recording rules for frequently queried metrics:

```yaml
groups:
  - name: hypersdk_recording_rules
    interval: 30s
    rules:
      # Pre-calculate success rate
      - record: transiva:job_success_rate
        expr: |
          rate(hypersdk_jobs_completed_total[5m])
          /
          (rate(hypersdk_jobs_completed_total[5m]) + rate(hypersdk_jobs_failed_total[5m]))

      # Pre-calculate average duration
      - record: transiva:job_duration_avg
        expr: |
          rate(hypersdk_job_duration_seconds_sum[5m])
          /
          rate(hypersdk_job_duration_seconds_count[5m])
```

Then use in dashboards:
```promql
# Instead of complex query
transiva:job_success_rate

# Instead of histogram calculation
transiva:job_duration_avg
```

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/zyvorai/transiva/issues
- Documentation: https://transiva.dev/docs/monitoring

---

## License

Same as HyperSDK main project.
