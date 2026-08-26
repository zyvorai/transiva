# HyperSDK Kubernetes Deployment

Production-ready Kubernetes deployment for HyperSDK using Kustomize with environment-specific overlays.

## Quick Start

```bash
# 1. Create secrets (REQUIRED)
cd deployments/kubernetes
cp base/secrets.yaml.example overlays/development/secrets.yaml
vim overlays/development/secrets.yaml  # Add your credentials

# 2. Apply secrets
kubectl create namespace transiva
kubectl apply -f overlays/development/secrets.yaml

# 3. Deploy using script
cd ../..
./deployments/scripts/deploy-k8s.sh development

# 4. Check status
kubectl get all -n transiva

# 5. Access API (port-forward)
kubectl port-forward -n transiva svc/hypervisord 8080:8080
```

## Architecture

### Deployment Components

1. **Namespace**: `transiva` - Isolated namespace for all resources
2. **Deployment**: `hypervisord` - Main daemon (1 replica due to SQLite)
3. **Service**: `hypervisord` - ClusterIP service for internal access
4. **Service**: `hypervisord-external` - LoadBalancer for external access
5. **ConfigMap**: `hypervisord-config` - Configuration data
6. **Secrets**: Cloud provider credentials
7. **PVC**: `hypervisord-data-pvc` - Database storage (10-20Gi)
8. **PVC**: `hypervisord-exports-pvc` - VM exports storage (500Gi-2Ti)
9. **ServiceAccount**: RBAC permissions
10. **ServiceMonitor**: Prometheus Operator integration (optional)

### Directory Structure

```
kubernetes/
├── base/                           # Base Kustomize resources
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── secrets.yaml.example
│   ├── pvc.yaml
│   ├── serviceaccount.yaml
│   └── rbac.yaml
│
├── overlays/                       # Environment-specific configs
│   ├── development/
│   │   ├── kustomization.yaml
│   │   └── resources-patch.yaml   # Lower resources
│   ├── staging/
│   │   ├── kustomization.yaml
│   │   ├── resources-patch.yaml
│   │   ├── ingress.yaml           # TLS ingress
│   │   └── hpa.yaml               # Autoscaling
│   └── production/
│       ├── kustomization.yaml
│       ├── resources-patch.yaml   # Higher resources
│       ├── ingress.yaml           # Production ingress
│       ├── hpa.yaml               # Production HPA
│       └── networkpolicy.yaml     # Network restrictions
│
└── monitoring/                     # Prometheus Operator integration
    ├── servicemonitor.yaml
    └── prometheusrule.yaml
```

## Prerequisites

### Required

- Kubernetes 1.24+
- kubectl with kustomize support (built-in since 1.14)
- Storage class for dynamic PVC provisioning
- At least 2 CPU cores and 4GB RAM available

### Optional

- Ingress controller (nginx, Traefik, etc.) for external access
- cert-manager for automatic TLS certificates
- Prometheus Operator for monitoring
- Metrics Server for HPA

### Verify Prerequisites

```bash
# Check Kubernetes version
kubectl version

# Check storage classes
kubectl get storageclass

# Check if you have cluster-admin access
kubectl auth can-i create namespace
```

## Configuration

### 1. Secrets Configuration

**IMPORTANT**: You must configure secrets before deployment.

```bash
# Copy example to your environment
cp base/secrets.yaml.example overlays/development/secrets.yaml

# Edit with your credentials
vim overlays/development/secrets.yaml
```

The example includes templates for cloud and virtualization providers:
- vSphere / VMware vCenter
- Nutanix AHV (Prism Central)
- AWS
- Azure
- Google Cloud Platform
- Microsoft Hyper-V
- Oracle Cloud Infrastructure
- OpenStack
- Alibaba Cloud
- Proxmox VE

**Security Best Practice**: Never commit secrets.yaml to git. It's included in .gitignore.

### 2. ConfigMap Customization

Edit `base/configmap.yaml` to customize:

```yaml
data:
  log_level: "info"              # debug, info, warn, error
  download_workers: "3"          # Concurrent download workers
  chunk_size: "33554432"         # Download chunk size (32MB)
  retry_attempts: "3"            # Retry failed operations
```

### 3. Storage Configuration

Edit `base/pvc.yaml` to adjust storage:

```yaml
# Data PVC (database)
resources:
  requests:
    storage: 10Gi               # Adjust based on needs
storageClassName: fast-ssd      # Use fast storage for DB

# Exports PVC (VM files)
resources:
  requests:
    storage: 500Gi              # Adjust based on VM sizes
storageClassName: standard-hdd  # Cheaper storage for exports
```

### 4. Resource Limits

Resources are defined per environment in overlays:

**Development** (overlays/development/resources-patch.yaml):
```yaml
requests:
  memory: "256Mi"
  cpu: "100m"
limits:
  memory: "1Gi"
  cpu: "500m"
```

**Production** (overlays/production/resources-patch.yaml):
```yaml
requests:
  memory: "1Gi"
  cpu: "500m"
limits:
  memory: "4Gi"
  cpu: "2000m"
```

## Deployment

### Using the Deployment Script (Recommended)

```bash
# Deploy to development
./deployments/scripts/deploy-k8s.sh development

# Deploy to staging
./deployments/scripts/deploy-k8s.sh staging

# Deploy to production
./deployments/scripts/deploy-k8s.sh production

# Dry-run before applying
./deployments/scripts/deploy-k8s.sh production --dry-run

# Show diff
./deployments/scripts/deploy-k8s.sh production --action diff
```

### Manual Deployment

```bash
# 1. Create namespace
kubectl create namespace transiva

# 2. Apply secrets
kubectl apply -f overlays/development/secrets.yaml -n transiva

# 3. Deploy using Kustomize
kubectl apply -k overlays/development

# 4. Wait for deployment
kubectl rollout status deployment/hypervisord -n transiva

# 5. Verify
kubectl get all -n transiva
```

## Accessing the Service

### Port Forward (Development)

```bash
# Forward API port
kubectl port-forward -n transiva svc/hypervisord 8080:8080

# Forward metrics port
kubectl port-forward -n transiva svc/hypervisord 8081:8081

# Access in another terminal
curl http://localhost:8080/health
curl http://localhost:8081/metrics
```

### LoadBalancer (Cloud)

If using `hypervisord-external` service with LoadBalancer:

```bash
# Get external IP
kubectl get svc hypervisord-external -n transiva

# Access via external IP
curl http://<EXTERNAL-IP>/health
```

### Ingress (Staging/Production)

For staging and production, Ingress is configured in overlays:

```bash
# Check Ingress
kubectl get ingress -n transiva

# Access via hostname (after DNS configuration)
curl https://transiva.example.com/health
```

## Monitoring

### Prometheus Operator Integration

If you have Prometheus Operator installed:

```bash
# Apply monitoring resources
kubectl apply -f monitoring/servicemonitor.yaml
kubectl apply -f monitoring/prometheusrule.yaml

# Verify ServiceMonitor
kubectl get servicemonitor -n transiva

# Verify PrometheusRule
kubectl get prometheusrule -n transiva
```

### Metrics Endpoint

Access metrics directly:

```bash
# Port-forward metrics
kubectl port-forward -n transiva svc/hypervisord 8081:8081

# View metrics
curl http://localhost:8081/metrics
```

### Prometheus Alerts

The PrometheusRule includes alerts for:
- Service health (down, high error rate)
- Job failures and backlog
- Resource usage (CPU, memory, disk)
- Provider connection issues
- API performance degradation
- Database performance

## Scaling

### Current Limitations

HyperSDK uses SQLite which is single-writer, limiting scaling:

```yaml
spec:
  replicas: 1  # Cannot increase due to SQLite
```

### Future HA Support

For horizontal scaling:
1. Migrate to PostgreSQL backend
2. Use ReadWriteMany PVC or object storage for exports
3. Implement distributed job queue (Redis Streams/NATS)
4. Update HPA maxReplicas

### Vertical Scaling

Adjust resources in environment overlays:

```yaml
resources:
  requests:
    memory: "2Gi"
    cpu: "1000m"
  limits:
    memory: "8Gi"
    cpu: "4000m"
```

## Updating

### Update Image Version

Edit overlay kustomization.yaml:

```yaml
images:
- name: transiva/hypervisord
  newName: transiva/hypervisord
  newTag: v0.3.0  # Update version
```

Apply update:

```bash
kubectl apply -k overlays/production
kubectl rollout status deployment/hypervisord -n transiva
```

### Rolling Update

Kubernetes automatically performs rolling updates:

```bash
# Trigger update
kubectl set image deployment/hypervisord \
  hypervisord=transiva/hypervisord:v0.3.0 \
  -n transiva

# Watch rollout
kubectl rollout status deployment/hypervisord -n transiva

# Check history
kubectl rollout history deployment/hypervisord -n transiva
```

### Rollback

```bash
# Rollback to previous version
kubectl rollout undo deployment/hypervisord -n transiva

# Rollback to specific revision
kubectl rollout undo deployment/hypervisord -n transiva --to-revision=2
```

## Troubleshooting

### Check Pod Status

```bash
# List pods
kubectl get pods -n transiva

# Describe pod
kubectl describe pod -n transiva -l app=hypervisord

# Check events
kubectl get events -n transiva --sort-by='.lastTimestamp'
```

### View Logs

```bash
# Current logs
kubectl logs -n transiva -l app=hypervisord

# Follow logs
kubectl logs -n transiva -l app=hypervisord -f

# Previous container logs
kubectl logs -n transiva -l app=hypervisord --previous

# Logs from specific container
kubectl logs -n transiva deployment/hypervisord -c hypervisord
```

### Debug Container

```bash
# Exec into container
kubectl exec -it -n transiva deployment/hypervisord -- sh

# Run commands inside
env | grep GOVC
curl http://localhost:8080/health
ls -la /data
```

### Common Issues

#### Pod in ImagePullBackOff

```bash
# Check image
kubectl describe pod -n transiva -l app=hypervisord | grep -A 5 Events

# Possible solutions:
# - Fix image name/tag in kustomization.yaml
# - Add imagePullSecrets for private registry
# - Ensure image exists in registry
```

#### Pod in CrashLoopBackOff

```bash
# Check logs
kubectl logs -n transiva -l app=hypervisord --previous

# Common causes:
# - Missing or invalid credentials in secrets
# - Database corruption
# - Insufficient resources
# - Configuration errors
```

#### PVC Pending

```bash
# Check PVC status
kubectl get pvc -n transiva
kubectl describe pvc hypervisord-data-pvc -n transiva

# Possible solutions:
# - Ensure storage class exists
# - Check storage class provisioner
# - Verify sufficient storage quota
# - Create PV manually if using static provisioning
```

#### Cannot Access Service

```bash
# Check service
kubectl get svc -n transiva
kubectl describe svc hypervisord -n transiva

# Check endpoints
kubectl get endpoints -n transiva

# Verify pod is ready
kubectl get pods -n transiva -l app=hypervisord
```

## Backup and Restore

### Backup Database

```bash
# Backup SQLite database
POD=$(kubectl get pod -n transiva -l app=hypervisord -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n transiva $POD -- \
  sqlite3 /data/transiva.db ".backup '/data/backup.db'"

# Copy backup locally
kubectl cp transiva/$POD:/data/backup.db ./transiva-backup-$(date +%Y%m%d).db
```

### Backup PVCs

```bash
# Using VolumeSnapshot (if supported)
cat <<EOF | kubectl apply -f -
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: hypervisord-data-snapshot
  namespace: transiva
spec:
  source:
    persistentVolumeClaimName: hypervisord-data-pvc
EOF
```

### Backup with Velero

```bash
# Backup entire namespace
velero backup create transiva-backup --include-namespaces transiva

# Restore backup
velero restore create --from-backup transiva-backup
```

## Security Hardening

### Network Policies

Production overlay includes NetworkPolicy:

```bash
# Verify NetworkPolicy
kubectl get networkpolicy -n transiva

# Test connectivity
kubectl run -it --rm debug --image=alpine --restart=Never -n transiva -- sh
# Inside debug pod:
wget -O- http://hypervisord:8080/health
```

### Pod Security Standards

Apply Pod Security Standards:

```bash
kubectl label namespace transiva \
  pod-security.kubernetes.io/enforce=restricted \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted
```

### RBAC

Minimal RBAC is configured in `base/rbac.yaml`. Review and adjust:

```bash
# View current permissions
kubectl describe role hypervisord-role -n transiva
kubectl describe rolebinding hypervisord-rolebinding -n transiva
```

### Secrets Encryption

Enable encryption at rest in Kubernetes:

```bash
# Check if encryption is enabled
kubectl get secrets -n transiva -o yaml | head -20
```

## Cost Optimization

### Storage Optimization

```yaml
# Use cheaper storage for exports
storageClassName: standard-hdd  # vs fast-ssd

# Implement lifecycle policies
# Delete old exports after N days
```

### Resource Rightsizing

Monitor actual usage:

```bash
# Install metrics-server
kubectl top pods -n transiva
kubectl top nodes

# Adjust requests/limits based on actual usage
```

### Spot/Preemptible Nodes

Use node affinity to schedule on spot instances:

```yaml
affinity:
  nodeAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      preference:
        matchExpressions:
        - key: cloud.google.com/gke-preemptible
          operator: In
          values:
          - "true"
```

## Best Practices

1. **Use specific image tags** in production (not `latest`)
2. **Test in development** before promoting to staging/production
3. **Monitor resource usage** and adjust limits
4. **Enable Prometheus monitoring** with alerting
5. **Configure backup** strategy for PVCs
6. **Use Secrets management** (Sealed Secrets, External Secrets, Vault)
7. **Enable NetworkPolicies** in production
8. **Set up log aggregation** (ELK, Loki, CloudWatch)
9. **Implement GitOps** (ArgoCD, Flux) for deployment automation
10. **Regular security audits** with tools like kube-bench, kube-hunter

## Additional Resources

- Main deployment guide: ../README.md
- Docker deployment: ../docker/README.md
- Kustomize documentation: https://kustomize.io
- Kubernetes best practices: https://kubernetes.io/docs/concepts/configuration/overview/
