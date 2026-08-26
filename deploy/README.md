# HyperSDK Kubernetes Operator Deployment

This directory contains deployment manifests and installation scripts for the HyperSDK Kubernetes Operator.

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.25+)
- kubectl configured to access your cluster
- Cluster admin permissions

### Installation

```bash
# Install operator and CRDs
./install.sh
```

This will:
1. Install Custom Resource Definitions (CRDs)
2. Create namespace and RBAC
3. Deploy the HyperSDK operator

### Verify Installation

```bash
# Check operator pod
kubectl get pods -n transiva-system

# Check CRDs
kubectl get crds | grep transiva

# View operator logs
kubectl logs -f -n transiva-system -l app=transiva-operator
```

### Uninstallation

```bash
# Remove operator and CRDs
./uninstall.sh
```

**Warning**: This will delete all BackupJobs, BackupSchedules, and RestoreJobs.

---

## Directory Structure

```
deploy/
├── crds/                          # Custom Resource Definitions
│   ├── transiva.zyvor.ai_backupjobs.yaml
│   ├── transiva.zyvor.ai_backupschedules.yaml
│   └── transiva.zyvor.ai_restorejobs.yaml
├── operator/                      # Operator deployment manifests
│   ├── deployment.yaml
│   └── rbac.yaml
├── examples/                      # Example resource manifests
│   ├── backupjob-kubevirt.yaml
│   ├── backupschedule-nightly.yaml
│   └── restorejob-example.yaml
├── install.sh                     # Installation script
├── uninstall.sh                   # Uninstallation script
└── README.md                      # This file
```

---

## Usage Examples

### Create a BackupJob

Backup a KubeVirt VM to S3:

```bash
kubectl apply -f examples/backupjob-kubevirt.yaml
```

Check status:

```bash
kubectl get backupjobs
kubectl describe backupjob ubuntu-vm-backup
```

### Create a BackupSchedule

Schedule nightly backups:

```bash
kubectl apply -f examples/backupschedule-nightly.yaml
```

Check schedule status:

```bash
kubectl get backupschedules
kubectl describe backupschedule nightly-vm-backup
```

### Restore a VM

Restore from a previous backup:

```bash
kubectl apply -f examples/restorejob-example.yaml
```

Monitor restore progress:

```bash
kubectl get restorejobs
kubectl describe restorejob restore-ubuntu-vm
```

---

## Custom Resource Definitions

### BackupJob

Declarative VM backup specification.

**Key Features**:
- Multi-provider support (KubeVirt, vSphere, AWS, Azure, GCP, etc.)
- Multiple destination types (S3, Azure Blob, GCS, local, NFS)
- Format options (OVF, OVA, RAW, QCOW2, VMDK, VHD, VHDX)
- Incremental backup support
- Carbon-aware scheduling
- Retention policies

**Example**:
```yaml
apiVersion: transiva.zyvor.ai/v1alpha1
kind: BackupJob
metadata:
  name: my-vm-backup
spec:
  source:
    provider: kubevirt
    namespace: default
    vmName: my-vm
  destination:
    type: s3
    bucket: my-backups
    region: us-west-2
  carbonAware:
    enabled: true
```

### BackupSchedule

Cron-based backup scheduling.

**Key Features**:
- Cron schedule with timezone support
- Concurrency policies (Allow, Forbid, Replace)
- Job history management
- Suspend/resume capability

**Example**:
```yaml
apiVersion: transiva.zyvor.ai/v1alpha1
kind: BackupSchedule
metadata:
  name: nightly-backup
spec:
  schedule: "0 2 * * *"
  timezone: America/Los_Angeles
  jobTemplate:
    spec:
      source:
        provider: kubevirt
        namespace: default
        vmName: my-vm
      destination:
        type: s3
        bucket: my-backups
```

### RestoreJob

VM restore operation.

**Key Features**:
- Multiple source types (S3, Azure, GCS, BackupJob reference)
- Power-on after restore
- VM customization (memory, CPU, networks)
- Format conversion during restore

**Example**:
```yaml
apiVersion: transiva.zyvor.ai/v1alpha1
kind: RestoreJob
metadata:
  name: restore-my-vm
spec:
  source:
    backupJobRef:
      name: my-vm-backup
  destination:
    provider: kubevirt
    namespace: default
    vmName: my-vm-restored
  options:
    powerOnAfterRestore: true
```

---

## Configuration

### AWS Credentials

Create a secret for AWS access:

```bash
kubectl create secret generic aws-credentials \
  --from-literal=access-key-id=YOUR_ACCESS_KEY \
  --from-literal=secret-access-key=YOUR_SECRET_KEY \
  --namespace=default
```

Reference in BackupJob:

```yaml
spec:
  destination:
    type: s3
    bucket: my-backups
    credentials:
      secretRef:
        name: aws-credentials
        namespace: default
```

### Azure Credentials

```bash
kubectl create secret generic azure-credentials \
  --from-literal=storage-account=YOUR_ACCOUNT \
  --from-literal=storage-key=YOUR_KEY \
  --namespace=default
```

### GCS Credentials

```bash
kubectl create secret generic gcs-credentials \
  --from-file=service-account.json=path/to/service-account.json \
  --namespace=default
```

---

## Monitoring

### kubectl commands

```bash
# List all backup jobs
kubectl get backupjobs --all-namespaces

# Watch backup progress
kubectl get backupjobs -w

# Get detailed status
kubectl describe backupjob <name>

# View events
kubectl get events --sort-by=.metadata.creationTimestamp

# Check operator logs
kubectl logs -f -n transiva-system deployment/transiva-operator
```

### Status Fields

All resources include comprehensive status:

- **phase**: Current execution phase (Pending, Running, Completed, Failed)
- **progress**: Percentage, bytes transferred, current phase
- **conditions**: Detailed condition tracking
- **startTime**: Job start time
- **completionTime**: Job completion time

Example:

```bash
$ kubectl get backupjobs
NAME               PHASE       PROGRESS   SOURCE        DESTINATION   AGE
ubuntu-vm-backup   Running     45%        ubuntu-vm-1   s3            2m
```

---

## Troubleshooting

### Operator not starting

Check operator logs:
```bash
kubectl logs -n transiva-system deployment/transiva-operator
```

Check pod status:
```bash
kubectl describe pod -n transiva-system -l app=transiva-operator
```

### BackupJob stuck in Pending

Check job conditions:
```bash
kubectl describe backupjob <name>
```

Common issues:
- Invalid credentials secret
- Provider connection failure
- Carbon-aware delay (check `carbonAware` settings)

### Insufficient permissions

Check RBAC:
```bash
kubectl get clusterrole transiva-operator
kubectl get clusterrolebinding transiva-operator
```

Reinstall RBAC:
```bash
kubectl apply -f operator/rbac.yaml
```

---

## Advanced Configuration

### Operator Configuration

Edit deployment:
```bash
kubectl edit deployment transiva-operator -n transiva-system
```

Available args:
- `--workers=N`: Number of worker threads per controller (default: 3)
- `--log-level=LEVEL`: Log level: debug, info, warn, error (default: info)
- `--namespace=NS`: Watch specific namespace (default: all namespaces)

### Resource Limits

Adjust operator resources:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

### Leader Election

For high availability, run multiple operator replicas:

```yaml
spec:
  replicas: 3
```

Only one replica will be active (leader election).

---

## Integration

### With Carbon-Aware Scheduling

All backups can opt into carbon-aware scheduling:

```yaml
spec:
  carbonAware:
    enabled: true
    zone: US-CAL-CISO        # Carbon intensity zone
    maxIntensity: 200.0       # Max gCO2/kWh threshold
    maxDelayHours: 4.0        # Max delay window
```

See [Carbon-Aware Quick Start](../docs/CARBON_AWARE_QUICK_START.md) for details.

### With Incremental Backups

Enable incremental backups for faster operation:

```yaml
spec:
  incremental:
    enabled: true
    baseBackupRef: initial-backup  # Reference to full backup
```

### With Retention Policies

Automatic backup cleanup:

```yaml
spec:
  retention:
    keepDaily: 7      # Keep 7 daily backups
    keepWeekly: 4     # Keep 4 weekly backups
    keepMonthly: 12   # Keep 12 monthly backups
    keepYearly: 3     # Keep 3 yearly backups
```

---

## Migration from v2.0

HyperSDK v2.1 introduces Kubernetes operator. No changes to existing workflows.

**New capabilities**:
- Declarative backups via CRDs
- Automated scheduling
- Kubernetes-native management

**Backward compatibility**:
- REST API remains available
- CLI tools continue to work
- Python/TypeScript SDKs unchanged

---

## Support

- **Documentation**: [Complete guides](../docs/)
- **Examples**: [More examples](examples/)
- **Issues**: [GitHub Issues](https://github.com/ssahani/transiva/issues)

---

*HyperSDK Kubernetes Operator - v2.1.0*
*Declarative VM Backup and Restore for Kubernetes*
