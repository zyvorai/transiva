# HyperSDK Grafana Dashboards

Pre-built Grafana dashboards for monitoring HyperSDK deployments.

## Available Dashboards

### 1. HyperSDK Overview (`transiva-overview.json`)

**Purpose**: Comprehensive operational dashboard for monitoring HyperSDK application health and performance.

**Panels**:
- **Running Instances**: Gauge showing number of healthy instances
- **Request Rate**: Time series of HTTP requests per second
- **HTTP Status Distribution**: Breakdown of 2xx/4xx/5xx responses
- **Request Duration**: P50, P95, P99 latency percentiles
- **Active Export Jobs**: Current number of running export jobs
- **Job Completion Rate**: Hourly success/failure statistics
- **Export Duration**: P50, P95, P99 export job duration
- **Memory Usage**: Per-pod memory consumption
- **CPU Usage**: Per-pod CPU utilization

**Use Cases**:
- Daily operations monitoring
- Performance troubleshooting
- Capacity planning
- SLA tracking

### 2. Cost Tracking (`transiva-cost-tracking.json`)

**Purpose**: FinOps dashboard for tracking and optimizing Kubernetes resource costs.

**Panels**:
- **Estimated Monthly CPU Cost**: Projected monthly spend on CPU resources
- **Estimated Monthly Memory Cost**: Projected monthly spend on memory resources
- **Estimated Monthly Storage Cost**: Projected monthly spend on persistent storage
- **Total Estimated Monthly Cost**: Combined infrastructure cost projection
- **Resource Utilization vs Requests**: Identify over-provisioned resources
- **CPU Requests vs Usage**: Compare allocated vs actual CPU usage
- **Resource Waste by Pod**: Table showing waste percentage per pod

**Use Cases**:
- Cost optimization
- Right-sizing recommendations
- Budget tracking
- Resource waste identification

## Installation

### Option 1: Import via Grafana UI

1. Log into your Grafana instance
2. Navigate to **Dashboards** → **Import**
3. Upload the JSON file or paste its contents
4. Select your Prometheus datasource
5. Click **Import**

### Option 2: Deploy via ConfigMap

```bash
# Create ConfigMap with dashboards
kubectl create configmap transiva-dashboards \
  --from-file=deployments/helm/dashboards/ \
  -n monitoring

# Add label for Grafana sidecar auto-discovery
kubectl label configmap transiva-dashboards \
  grafana_dashboard=1 \
  -n monitoring
```

### Option 3: Automated Deployment with Helm

If using the Prometheus Operator's Grafana deployment:

```yaml
# values.yaml
grafana:
  dashboardProviders:
    dashboardproviders.yaml:
      apiVersion: 1
      providers:
        - name: 'transiva'
          orgId: 1
          folder: 'HyperSDK'
          type: file
          disableDeletion: false
          editable: true
          options:
            path: /var/lib/grafana/dashboards/transiva

  dashboards:
    transiva:
      transiva-overview:
        file: dashboards/transiva-overview.json
      transiva-cost:
        file: dashboards/transiva-cost-tracking.json
```

Then install/upgrade:

```bash
helm upgrade --install prometheus-stack prometheus-community/kube-prometheus-stack \
  -f values.yaml \
  -n monitoring
```

## Prerequisites

### Required Data Sources

- **Prometheus**: For metrics collection
  - ServiceMonitor should be enabled (`monitoring.serviceMonitor.enabled=true`)
  - Metrics endpoint accessible at `:8081/metrics`

### Required Metrics

The dashboards expect the following Prometheus metrics:

**Application Metrics** (from HyperSDK):
```
hypersdk_http_requests_total
hypersdk_http_request_duration_seconds_bucket
hypersdk_active_jobs
hypersdk_jobs_completed_total{status="success|failed"}
hypersdk_export_duration_seconds_bucket
```

**System Metrics** (from kube-state-metrics and node-exporter):
```
up
process_resident_memory_bytes
process_cpu_seconds_total
kube_pod_container_resource_requests
kube_persistentvolumeclaim_resource_requests_storage_bytes
container_cpu_usage_seconds_total
container_memory_working_set_bytes
```

## Cost Calculation Details

The cost dashboard uses the following pricing assumptions (update for your cloud provider):

| Resource | Formula | Default Rate |
|----------|---------|--------------|
| **CPU** | cores × 730 hours × $0.04/core-hour | $0.04/core-hour |
| **Memory** | GB × 730 hours × $0.004/GB-hour | $0.004/GB-hour |
| **Storage** | GB × $0.10/GB-month | $0.10/GB-month |

**Update rates for your provider**:

- **AWS EKS**: $0.04/vCPU-hour, $0.004/GB-hour
- **GCP GKE**: $0.033/vCPU-hour, $0.004/GB-hour
- **Azure AKS**: $0.036/vCPU-hour, $0.005/GB-hour

Edit the dashboard queries to adjust pricing.

## Customization

### Modify Time Range

Default: Last 6 hours
To change: Dashboard settings → Time options → Default time range

### Add Alerts

Convert any panel to an alert:

1. Click panel title → **Edit**
2. Navigate to **Alert** tab
3. Configure alert conditions
4. Add notification channel
5. **Save**

### Add Custom Panels

Common additions:

**Network I/O**:
```promql
sum(rate(container_network_receive_bytes_total{namespace="transiva"}[5m]))
sum(rate(container_network_transmit_bytes_total{namespace="transiva"}[5m]))
```

**Disk I/O**:
```promql
sum(rate(container_fs_reads_bytes_total{namespace="transiva"}[5m]))
sum(rate(container_fs_writes_bytes_total{namespace="transiva"}[5m]))
```

**Database Connections** (if exposing custom metrics):
```promql
hypersdk_database_connections_active
hypersdk_database_connections_idle
```

## Troubleshooting

### No Data Showing

**Check Prometheus**:
```bash
# Verify metrics are being scraped
kubectl port-forward -n transiva svc/transiva 8081:8081
curl http://localhost:8081/metrics

# Check ServiceMonitor
kubectl get servicemonitor -n transiva
kubectl describe servicemonitor transiva -n transiva
```

**Check Grafana datasource**:
1. Grafana → Configuration → Data Sources
2. Select Prometheus
3. Click **Test** button
4. Verify connection successful

### Wrong Namespace

Update the `$namespace` variable in dashboard settings to match your deployment namespace.

### Cost Calculations Inaccurate

The cost dashboard provides estimates based on resource **requests**, not actual usage. For precise costs:

1. Install [Kubecost](https://www.kubecost.com/) or [OpenCost](https://www.opencost.io/)
2. Use their dedicated dashboards for accurate cloud billing integration

## Best Practices

1. **Create snapshots** before making changes: Dashboard → Share → Snapshot
2. **Set up alerts** for critical metrics (high error rate, low success rate)
3. **Review cost dashboard weekly** to identify optimization opportunities
4. **Combine with logs** using Loki for complete observability
5. **Enable dashboard versioning** in Grafana settings

## Related Documentation

- [OBSERVABILITY.md](../OBSERVABILITY.md) - Complete observability setup guide
- [COST-OPTIMIZATION.md](../COST-OPTIMIZATION.md) - Cost reduction strategies
- [OPERATIONS-RUNBOOK.md](../OPERATIONS-RUNBOOK.md) - Daily operations procedures

## Support

- **Issues**: https://github.com/ssahani/transiva/issues
- **Discussions**: https://github.com/ssahani/transiva/discussions
- **Grafana Docs**: https://grafana.com/docs/grafana/latest/

---

**💡 Tip**: Pin these dashboards to your Grafana home for quick access during incidents!
