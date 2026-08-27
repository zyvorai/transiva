# HyperSDK Deployment Guide

This directory contains all deployment configurations for HyperSDK across Docker, Podman, and Kubernetes environments.

## Quick Start

### Using Pre-built Images from GitHub Container Registry

Pull and run pre-built images (no build required):

```bash
# Pull images from GitHub Container Registry
docker pull ghcr.io/ssahani/transiva-hypervisord:latest
docker pull ghcr.io/ssahani/transiva-hyperexport:latest

# Run hypervisord
docker run -d \
  --name transiva \
  -p 8080:8080 \
  -p 8081:8081 \
  -v transiva-data:/data \
  -v transiva-exports:/exports \
  ghcr.io/ssahani/transiva-hypervisord:latest

# Access dashboard
open http://localhost:8080/web/dashboard/
```

### Docker (Build Locally)

```bash
# Start the full stack
cd deployments/docker
docker-compose up -d

# Access services
# API: http://localhost:8080
# Metrics: http://localhost:8081/metrics
# Grafana: http://localhost:3000 (admin/admin)
# Prometheus: http://localhost:9090
```

### Podman

```bash
# Start with podman-compose
cd deployments/podman
podman-compose up -d

# Or use Quadlet (systemd integration)
cp deployments/podman/quadlet/transiva.container ~/.config/containers/systemd/
systemctl --user daemon-reload
systemctl --user start transiva
```

### Kubernetes

```bash
# Deploy to development
./deployments/scripts/deploy-k8s.sh development

# Check deployment status
kubectl get pods -n transiva

# Port-forward to access API
kubectl port-forward -n transiva svc/hypervisord 8080:8080
```

## Directory Structure

```
deployments/
├── docker/                    # Docker and Docker Compose configurations
│   ├── dockerfiles/           # Multi-stage Dockerfiles for each component
│   │   ├── Dockerfile.hypervisord
│   │   └── Dockerfile.hyperexport
│   ├── docker-compose.yml     # Complete stack with monitoring
│   └── .dockerignore
│
├── kubernetes/                # Kubernetes manifests
│   ├── base/                  # Base Kustomize resources
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secrets.yaml.example
│   │   ├── pvc.yaml
│   │   └── ...
│   ├── overlays/              # Environment-specific configurations
│   │   ├── development/
│   │   ├── staging/
│   │   └── production/
│   └── monitoring/            # Prometheus Operator integration
│       ├── servicemonitor.yaml
│       └── prometheusrule.yaml
│
├── podman/                    # Podman configurations
│   ├── podman-compose.yml
│   └── quadlet/               # Systemd Quadlet files
│       └── transiva.container
│
├── scripts/                   # Automation scripts
│   ├── build-images.sh        # Build container images
│   ├── deploy-k8s.sh          # Deploy to Kubernetes
│   └── health-check.sh        # Health check across environments
│
└── README.md                  # This file
```

## Components

HyperSDK consists of two main components:

1. **hypervisord** - Main daemon with REST API (port 8080) and metrics (port 8081)
2. **hyperexport** - Standalone CLI tool for VM exports

## Prerequisites

### For Docker/Podman
- Docker 20.10+ or Podman 4.0+
- Docker Compose 2.0+ or podman-compose 1.0.6+
- 4GB RAM minimum, 8GB recommended
- 50GB+ disk space for VM exports

### For Kubernetes
- Kubernetes 1.24+
- kubectl with kustomize support
- 2 CPU cores, 4GB RAM minimum per node
- Storage class for PersistentVolumes
- Optional: Prometheus Operator for monitoring

## Configuration

### Environment Variables

All deployments support these environment variables:

**Daemon Configuration:**
- `DAEMON_ADDR` - API listen address (default: 0.0.0.0:8080)
- `LOG_LEVEL` - Logging level: debug, info, warn, error (default: info)
- `DATABASE_PATH` - SQLite database path (default: /data/transiva.db)
- `DOWNLOAD_WORKERS` - Concurrent download workers (default: 3)

**Cloud Provider Credentials:**
- vSphere: `GOVC_URL`, `GOVC_USERNAME`, `GOVC_PASSWORD`
- Nutanix: `NUTANIX_HOST`, `NUTANIX_USERNAME`, `NUTANIX_PASSWORD`, `NUTANIX_CLUSTER`, `NUTANIX_VERIFY_SSL`, `NUTANIX_ENABLED`
- AWS: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`
- Azure: `AZURE_SUBSCRIPTION_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`
- GCP: `GOOGLE_APPLICATION_CREDENTIALS`, `GCP_PROJECT_ID`

See `kubernetes/base/secrets.yaml.example` for complete list of supported providers. Nutanix migration guide: [docs/nutanix.md](../../docs/nutanix.md).

### Storage Requirements

- **Data Volume** (10-20Gi): SQLite database, job queue, cache
- **Exports Volume** (500Gi-2Ti): VM export storage (size depends on your VMs)

## Deployment Methods

### 1. Docker / Docker Compose

**Pros:**
- Simple setup for development and testing
- Includes full monitoring stack (Prometheus + Grafana)
- Easy to run on developer workstations

**Cons:**
- Not suitable for production HA
- Manual updates and scaling

See [docker/README.md](docker/README.md) for detailed instructions.

### 2. Podman

**Pros:**
- Rootless containers for better security
- Systemd integration via Quadlet
- Drop-in replacement for Docker

**Cons:**
- Smaller ecosystem than Docker
- Some compose features may differ

Compatible with both podman-compose and systemd Quadlet.

### 3. Kubernetes

**Pros:**
- Production-ready with HA capabilities
- Automatic scaling and self-healing
- Built-in secrets management
- Easy integration with monitoring stacks

**Cons:**
- More complex setup
- Requires Kubernetes knowledge
- SQLite limits horizontal scaling (use PostgreSQL for HA)

See [kubernetes/README.md](kubernetes/README.md) for detailed instructions.

## Building Images

Build all container images:

```bash
./deployments/scripts/build-images.sh
```

Build with specific version:

```bash
./deployments/scripts/build-images.sh --version v0.2.0
```

Build and push to registry:

```bash
./deployments/scripts/build-images.sh --version v0.2.0 --push --registry your-registry.com/transiva
```

Build with Podman:

```bash
./deployments/scripts/build-images.sh --builder podman
```

## Health Checks

Check deployment health:

```bash
# Docker deployment
./deployments/scripts/health-check.sh docker

# Podman deployment
./deployments/scripts/health-check.sh podman

# Kubernetes deployment
./deployments/scripts/health-check.sh kubernetes --namespace transiva
```

## Monitoring

All deployments include Prometheus metrics on port 8081. Metrics include:

- HTTP request rates and latencies
- Export job statistics (queued, running, completed, failed)
- Provider connection status
- Database query performance
- Resource usage (CPU, memory, disk)

### Docker/Podman Monitoring

The stack includes:
- **Prometheus** on port 9090 - Metrics collection
- **Grafana** on port 3000 - Dashboards (default: admin/admin)
- **Alertmanager** on port 9093 - Alert routing (optional)

Pre-configured dashboards:
- HyperSDK Overview
- Export Jobs Performance
- System Resources

### Kubernetes Monitoring

For Prometheus Operator:

```bash
# Apply ServiceMonitor and PrometheusRule
kubectl apply -f deployments/kubernetes/monitoring/
```

Metrics will be automatically scraped by Prometheus Operator.

## Security

### Non-Root Containers

All containers run as non-root user (UID/GID 1000) for security.

### Secrets Management

**Docker/Podman:** Use environment files (`.env`) - never commit to git

**Kubernetes:** Use native Secrets or:
- Sealed Secrets
- External Secrets Operator
- HashiCorp Vault
- Cloud provider secrets managers (AWS Secrets Manager, Azure Key Vault, GCP Secret Manager)

### Network Security

**Kubernetes:** Network Policies are included in production overlay to restrict traffic.

### TLS/HTTPS

Enable TLS in production:

**Docker:** Mount certificates to `/config/tls/`

**Kubernetes:** Use cert-manager for automatic TLS certificate management

## Troubleshooting

### Check logs

**Docker:**
```bash
docker-compose logs -f hypervisord
```

**Podman:**
```bash
podman logs -f transiva-hypervisord
# Or with systemd:
journalctl --user -u transiva -f
```

**Kubernetes:**
```bash
kubectl logs -n transiva -l app=hypervisord -f
```

### Common Issues

**Problem:** Containers fail to start
- Check logs for errors
- Verify environment variables are set
- Ensure volumes have correct permissions

**Problem:** Cannot connect to cloud providers
- Verify credentials in secrets
- Check network connectivity
- Review provider-specific logs

**Problem:** Database locked errors
- SQLite is single-writer - ensure only one instance is running
- For HA, migrate to PostgreSQL backend

**Problem:** Out of disk space
- Monitor exports volume usage
- Implement cleanup policies for old exports
- Consider using object storage (S3, Azure Blob) for exports

## Scaling Considerations

### Current Limitations

- **SQLite** is single-writer, limiting horizontal scaling to 1 replica
- Use PostgreSQL for multi-replica deployments
- Exports storage requires ReadWriteOnce access

### Future Enhancements

- PostgreSQL backend for HA
- S3-compatible object storage for exports
- Redis-based distributed job queue
- Multi-instance coordinator

## Upgrade Strategy

### Docker/Podman

```bash
# Pull new images
docker-compose pull

# Restart with new images
docker-compose up -d
```

### Kubernetes

```bash
# Update image tag in kustomization.yaml
# Then apply
./deployments/scripts/deploy-k8s.sh production
```

## Backup and Recovery

### Database Backup

**Docker/Podman:**
```bash
docker exec transiva-hypervisord sqlite3 /data/transiva.db ".backup '/data/backup.db'"
docker cp transiva-hypervisord:/data/backup.db ./backup-$(date +%Y%m%d).db
```

**Kubernetes:**
```bash
kubectl exec -n transiva deployment/hypervisord -- \
  sqlite3 /data/transiva.db ".backup '/data/backup.db'"
kubectl cp transiva/hypervisord-pod:/data/backup.db ./backup-$(date +%Y%m%d).db
```

### Volume Backup

Use volume snapshots or backup tools like Velero for Kubernetes.

## Support

- GitHub Issues: https://github.com/zyvorai/transiva/issues
- Documentation: https://github.com/zyvorai/transiva/docs

## License

See the main repository LICENSE file.
