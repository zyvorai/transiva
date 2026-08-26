# HyperSDK Operator Helm Chart

Helm chart for deploying the HyperSDK Kubernetes Operator - declarative VM backup and restore for Kubernetes.

## Features

- ✅ Declarative VM backup with BackupJob CRD
- ✅ Automated scheduling with BackupSchedule CRD
- ✅ VM restore with RestoreJob CRD
- ✅ Carbon-aware scheduling (30-50% CO2 reduction)
- ✅ Multi-provider support (KubeVirt, vSphere, AWS, Azure, GCP)
- ✅ Incremental backups
- ✅ Retention policies
- ✅ Kubernetes-native management

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+
- Cluster admin permissions

## Installation

### Quick Install

```bash
# Add HyperSDK Helm repository (when published)
helm repo add transiva https://ssahani.github.io/transiva-helm-charts
helm repo update

# Install the chart
helm install transiva-operator transiva/transiva-operator \
  --namespace transiva-system \
  --create-namespace
```

### Install from Source

```bash
# Clone repository
git clone https://github.com/ssahani/transiva.git
cd transiva/deploy/helm

# Install chart
helm install transiva-operator ./transiva-operator \
  --namespace transiva-system \
  --create-namespace
```

### Custom Values

```bash
# Install with custom values
helm install transiva-operator ./transiva-operator \
  --namespace transiva-system \
  --create-namespace \
  --set operator.replicaCount=3 \
  --set carbonAware.defaultEnabled=true \
  --set operator.args.logLevel=debug
```

Or create a `custom-values.yaml`:

```yaml
operator:
  replicaCount: 3
  args:
    workers: 5
    logLevel: debug
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi

carbonAware:
  defaultEnabled: true
  defaultZone: DE  # Germany (very clean grid)
  defaultMaxIntensity: 150.0

backupDefaults:
  format: ova
  compression: true
  retention:
    keepDaily: 14
    keepWeekly: 8
    keepMonthly: 24
```

Install:

```bash
helm install transiva-operator ./transiva-operator \
  --namespace transiva-system \
  --create-namespace \
  -f custom-values.yaml
```

## Upgrade

```bash
# Upgrade to latest version
helm upgrade transiva-operator ./transiva-operator \
  --namespace transiva-system

# Upgrade with new values
helm upgrade transiva-operator ./transiva-operator \
  --namespace transiva-system \
  -f custom-values.yaml
```

## Uninstall

```bash
# Uninstall the chart
helm uninstall transiva-operator --namespace transiva-system

# Optionally delete CRDs (this will delete all BackupJobs, BackupSchedules, RestoreJobs)
kubectl delete crd backupjobs.transiva.zyvor.ai
kubectl delete crd backupschedules.transiva.zyvor.ai
kubectl delete crd restorejobs.transiva.zyvor.ai

# Optionally delete namespace
kubectl delete namespace transiva-system
```

## Configuration

### Operator Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.replicaCount` | Number of operator replicas | `1` |
| `operator.image.repository` | Operator image repository | `ghcr.io/ssahani/transiva-operator` |
| `operator.image.tag` | Operator image tag | `2.1.0` |
| `operator.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `operator.args.workers` | Worker threads per controller | `3` |
| `operator.args.logLevel` | Log level (debug, info, warn, error) | `info` |
| `operator.args.namespace` | Namespace to watch (empty = all) | `""` |
| `operator.resources.requests.cpu` | CPU request | `100m` |
| `operator.resources.requests.memory` | Memory request | `128Mi` |
| `operator.resources.limits.cpu` | CPU limit | `500m` |
| `operator.resources.limits.memory` | Memory limit | `512Mi` |

### Carbon-Aware Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `carbonAware.defaultEnabled` | Enable carbon-aware by default | `true` |
| `carbonAware.defaultZone` | Default carbon zone | `US-CAL-CISO` |
| `carbonAware.defaultMaxIntensity` | Default max carbon intensity (gCO2/kWh) | `200.0` |
| `carbonAware.defaultMaxDelayHours` | Default max delay in hours | `4.0` |

**Available Zones**:
- North America: `US-CAL-CISO`, `US-NEISO`, `US-PJM`, `US-MISO`
- Europe: `SE`, `FR`, `GB`, `DE`
- Asia-Pacific: `JP`, `SG`, `AU`, `IN`

### Backup Defaults

| Parameter | Description | Default |
|-----------|-------------|---------|
| `backupDefaults.format` | Default backup format | `ova` |
| `backupDefaults.compression` | Enable compression | `true` |
| `backupDefaults.compressionLevel` | Compression level (1-9) | `6` |
| `backupDefaults.retention.keepDaily` | Daily backups to keep | `7` |
| `backupDefaults.retention.keepWeekly` | Weekly backups to keep | `4` |
| `backupDefaults.retention.keepMonthly` | Monthly backups to keep | `12` |
| `backupDefaults.retention.keepYearly` | Yearly backups to keep | `3` |

### Schedule Defaults

| Parameter | Description | Default |
|-----------|-------------|---------|
| `scheduleDefaults.concurrencyPolicy` | Concurrency policy | `Forbid` |
| `scheduleDefaults.successfulJobsHistoryLimit` | Successful jobs to keep | `3` |
| `scheduleDefaults.failedJobsHistoryLimit` | Failed jobs to keep | `1` |
| `scheduleDefaults.timezone` | Default timezone | `UTC` |

### Provider Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `providers.vsphere.enabled` | Enable vSphere provider | `true` |
| `providers.kubevirt.enabled` | Enable KubeVirt provider | `true` |
| `providers.aws.enabled` | Enable AWS provider | `true` |
| `providers.azure.enabled` | Enable Azure provider | `true` |
| `providers.gcp.enabled` | Enable GCP provider | `true` |

### Security Context

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.podSecurityContext.runAsNonRoot` | Run as non-root | `true` |
| `operator.podSecurityContext.runAsUser` | User ID | `65532` |
| `operator.podSecurityContext.fsGroup` | Filesystem group | `65532` |
| `operator.securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `operator.securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |

### RBAC

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `""` (generated) |

### Service

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8081` |

### Feature Gates

| Parameter | Description | Default |
|-----------|-------------|---------|
| `featureGates.incrementalBackups` | Enable incremental backups | `true` |
| `featureGates.vmCloning` | Enable VM cloning | `true` |
| `featureGates.liveMigration` | Enable live migration | `false` |
| `featureGates.webhookValidation` | Enable webhook validation | `false` |

## Usage Examples

### Create a BackupJob

```yaml
apiVersion: transiva.zyvor.ai/v1alpha1
kind: BackupJob
metadata:
  name: ubuntu-vm-backup
  namespace: default
spec:
  source:
    provider: kubevirt
    namespace: default
    vmName: ubuntu-vm-1
  destination:
    type: s3
    bucket: my-vm-backups
    region: us-west-2
  carbonAware:
    enabled: true
    zone: US-CAL-CISO
```

### Create a BackupSchedule

```yaml
apiVersion: transiva.zyvor.ai/v1alpha1
kind: BackupSchedule
metadata:
  name: nightly-backup
  namespace: default
spec:
  schedule: "0 2 * * *"
  timezone: America/Los_Angeles
  jobTemplate:
    spec:
      source:
        provider: kubevirt
        namespace: production
        tags:
          backup: enabled
      destination:
        type: s3
        bucket: prod-backups
```

### Create a RestoreJob

```yaml
apiVersion: transiva.zyvor.ai/v1alpha1
kind: RestoreJob
metadata:
  name: restore-ubuntu-vm
  namespace: default
spec:
  source:
    backupJobRef:
      name: ubuntu-vm-backup
  destination:
    provider: kubevirt
    namespace: default
    vmName: ubuntu-vm-restored
  options:
    powerOnAfterRestore: true
```

## Monitoring

### kubectl Commands

```bash
# List all backup jobs
kubectl get backupjobs --all-namespaces

# Watch backup progress
kubectl get backupjobs -w

# Get detailed status
kubectl describe backupjob ubuntu-vm-backup

# View operator logs
kubectl logs -n transiva-system -l app.kubernetes.io/name=transiva-operator -f
```

### Metrics (Coming Soon)

When `metrics.enabled=true`, Prometheus metrics will be available at:
- Endpoint: `http://<service>:8080/metrics`
- ServiceMonitor: Automatically created if `metrics.serviceMonitor.enabled=true`

## Troubleshooting

### Operator Not Starting

```bash
# Check pod status
kubectl get pods -n transiva-system

# View logs
kubectl logs -n transiva-system -l app.kubernetes.io/name=transiva-operator

# Describe pod
kubectl describe pod -n transiva-system -l app.kubernetes.io/name=transiva-operator
```

### BackupJob Stuck in Pending

```bash
# Check job conditions
kubectl describe backupjob <name>

# View events
kubectl get events --sort-by=.metadata.creationTimestamp
```

Common causes:
- Invalid credentials secret
- Provider connection failure
- Carbon-aware delay (check carbon settings)

### Insufficient Permissions

```bash
# Verify RBAC
kubectl get clusterrole transiva-operator
kubectl get clusterrolebinding transiva-operator

# Check service account
kubectl get serviceaccount -n transiva-system
```

## Development

### Testing the Chart

```bash
# Lint the chart
helm lint ./transiva-operator

# Dry run install
helm install transiva-operator ./transiva-operator \
  --namespace transiva-system \
  --create-namespace \
  --dry-run --debug

# Template the chart
helm template transiva-operator ./transiva-operator \
  --namespace transiva-system
```

### Package the Chart

```bash
# Package
helm package ./transiva-operator

# Generate index
helm repo index .
```

## Support

- **Documentation**: [https://github.com/ssahani/transiva/tree/main/docs](https://github.com/ssahani/transiva/tree/main/docs)
- **Examples**: [https://github.com/ssahani/transiva/tree/main/deploy/examples](https://github.com/ssahani/transiva/tree/main/deploy/examples)
- **Issues**: [https://github.com/ssahani/transiva/issues](https://github.com/ssahani/transiva/issues)

## License

Apache-2.0

## Maintainers

- ZyvorAI Labs Private Limited (@ssahani)

---

*HyperSDK Operator Helm Chart - v2.1.0*
*Declarative VM Backup and Restore for Kubernetes*
