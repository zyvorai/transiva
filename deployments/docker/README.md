# HyperSDK Docker Deployment

Complete Docker and Docker Compose deployment for HyperSDK with monitoring stack.

## Quick Start

```bash
cd deployments/docker

# Copy example config (optional)
mkdir -p config
cp ../../config.example.yaml config/config.yaml

# Create .env file with your credentials
cat > .env << EOF
# vSphere Configuration
GOVC_URL=https://vcenter.example.com/sdk
GOVC_USERNAME=administrator@vsphere.local
GOVC_PASSWORD=your-password

# Optional: AWS credentials
# AWS_ACCESS_KEY_ID=
# AWS_SECRET_ACCESS_KEY=
# AWS_REGION=us-east-1

# Nutanix Configuration (optional)
# NUTANIX_HOST=prism-central.example.com
# NUTANIX_USERNAME=admin
# NUTANIX_PASSWORD=changeme
# NUTANIX_CLUSTER=
# NUTANIX_VERIFY_SSL=0
# NUTANIX_ENABLED=1

LOG_LEVEL=info
DOWNLOAD_WORKERS=3
EOF

# Start the stack
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f hypervisord
```

## Access Services

Once deployed, services are available at:

- **HyperSDK API**: http://localhost:8080
- **Metrics**: http://localhost:8081/metrics
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (username: admin, password: admin)

## Architecture

The Docker Compose stack includes:

1. **hypervisord** - Main daemon service
   - REST API on port 8080
   - Prometheus metrics on port 8081
   - Persistent data in `transiva-data` volume
   - VM exports in `transiva-exports` volume

2. **redis** - Caching and queue management
   - Port 6379
   - Persistent storage with AOF
   - Max memory: 256MB (configurable)

3. **prometheus** - Metrics collection
   - Port 9090
   - Scrapes hypervisord metrics every 30s
   - 15-day retention (configurable)

4. **grafana** - Dashboards and visualization
   - Port 3000
   - Pre-configured with HyperSDK dashboards
   - Connected to Prometheus data source

## Configuration

### Environment Variables

Create a `.env` file in the `deployments/docker` directory:

```bash
# Daemon settings
DAEMON_PORT=8080
METRICS_PORT=8081
LOG_LEVEL=info
DOWNLOAD_WORKERS=3

# vSphere
GOVC_URL=https://vcenter.example.com/sdk
GOVC_USERNAME=administrator@vsphere.local
GOVC_PASSWORD=changeme
GOVC_INSECURE=1
GOVC_DATACENTER=Datacenter1
GOVC_DATASTORE=datastore1

# Nutanix (optional)
# NUTANIX_HOST=prism-central.example.com
# NUTANIX_USERNAME=admin
# NUTANIX_PASSWORD=changeme
# NUTANIX_CLUSTER=
# NUTANIX_VERIFY_SSL=0
# NUTANIX_ENABLED=1

# AWS (optional)
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
AWS_REGION=us-east-1

# Azure (optional)
AZURE_SUBSCRIPTION_ID=00000000-0000-0000-0000-000000000000
AZURE_TENANT_ID=00000000-0000-0000-0000-000000000000
AZURE_CLIENT_ID=00000000-0000-0000-0000-000000000000
AZURE_CLIENT_SECRET=changeme

# Redis
REDIS_ENABLED=true
REDIS_PASSWORD=
REDIS_MAX_MEMORY=256mb

# Grafana
GRAFANA_USER=admin
GRAFANA_PASSWORD=admin

# Prometheus
PROMETHEUS_RETENTION=15d
```

### Custom Configuration File

To use a custom configuration file:

```bash
# Create config directory
mkdir -p deployments/docker/config

# Copy and edit config
cp config.example.yaml deployments/docker/config/config.yaml
vim deployments/docker/config/config.yaml

# The file is automatically mounted to /config/config.yaml in the container
```

## Volume Management

### Data Persistence

Docker Compose creates several named volumes:

- `transiva-data` - SQLite database, job state
- `transiva-exports` - VM export files
- `transiva-logs` - Application logs
- `redis-data` - Redis persistence
- `prometheus-data` - Prometheus TSDB
- `grafana-data` - Grafana dashboards and settings

### Inspect Volumes

```bash
# List volumes
docker volume ls | grep transiva

# Inspect a volume
docker volume inspect transiva-data

# View exports
docker run --rm -v transiva-exports:/exports alpine ls -lh /exports
```

### Backup Volumes

```bash
# Backup data volume
docker run --rm \
  -v transiva-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/transiva-data-$(date +%Y%m%d).tar.gz /data

# Backup exports volume
docker run --rm \
  -v transiva-exports:/exports \
  -v $(pwd):/backup \
  alpine tar czf /backup/transiva-exports-$(date +%Y%m%d).tar.gz /exports
```

### Restore Volumes

```bash
# Restore data volume
docker run --rm \
  -v transiva-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/transiva-data-20240130.tar.gz -C /
```

## Building Images

### Build All Images

```bash
cd deployments/docker
docker-compose build
```

### Build Single Service

```bash
docker-compose build hypervisord
```

### Build with Script

```bash
# From project root
./deployments/scripts/build-images.sh

# With custom version
./deployments/scripts/build-images.sh --version v0.2.0

# Build and push
./deployments/scripts/build-images.sh --version v0.2.0 --push
```

## Managing the Stack

### Start Services

```bash
# Start all services
docker-compose up -d

# Start specific service
docker-compose up -d hypervisord

# Start with logs
docker-compose up
```

### Stop Services

```bash
# Stop all services
docker-compose stop

# Stop specific service
docker-compose stop hypervisord
```

### Restart Services

```bash
# Restart all
docker-compose restart

# Restart specific service
docker-compose restart hypervisord
```

### Remove Everything

```bash
# Stop and remove containers
docker-compose down

# Remove containers and volumes (WARNING: data loss!)
docker-compose down -v
```

## Monitoring

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f hypervisord

# Last 100 lines
docker-compose logs --tail=100 hypervisord
```

### Health Checks

```bash
# Run health check script
../../scripts/health-check.sh docker

# Manual health check
curl http://localhost:8080/health

# Check metrics
curl http://localhost:8081/metrics
```

### Prometheus Queries

Access Prometheus at http://localhost:9090 and try these queries:

```promql
# Request rate
rate(hypersdk_http_requests_total[5m])

# Export jobs
hypersdk_export_jobs_queued
hypersdk_export_jobs_running
hypersdk_export_jobs_completed_total
hypersdk_export_jobs_failed_total

# Resource usage
process_resident_memory_bytes
process_cpu_seconds_total
```

### Grafana Dashboards

1. Open Grafana: http://localhost:3000
2. Login (admin/admin)
3. Navigate to Dashboards
4. Open "HyperSDK Overview" or "HyperSDK Jobs"

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs hypervisord

# Inspect container
docker inspect transiva-hypervisord

# Check resources
docker stats
```

### Permission Issues

```bash
# Fix volume permissions
docker-compose down
docker volume rm transiva-data transiva-exports
docker-compose up -d
```

### Database Locked

SQLite is single-writer. Ensure only one hypervisord instance is running:

```bash
docker-compose ps
# Should show only one hypervisord container
```

### Network Issues

```bash
# Check network
docker network ls | grep transiva
docker network inspect transiva-network

# Recreate network
docker-compose down
docker network rm transiva-network
docker-compose up -d
```

### Can't Connect to vCenter

```bash
# Check environment variables
docker-compose config | grep GOVC

# Test from container
docker-compose exec hypervisord sh
# Inside container:
env | grep GOVC
curl -k $GOVC_URL
```

## Performance Tuning

### Adjust Worker Threads

```bash
# In .env file
DOWNLOAD_WORKERS=5  # Increase for faster parallel downloads
```

### Redis Memory

```bash
# In .env file
REDIS_MAX_MEMORY=512mb  # Increase for larger cache
```

### Prometheus Retention

```bash
# In .env file
PROMETHEUS_RETENTION=30d  # Increase retention period
```

## Updating

### Update to Latest Images

```bash
# Pull latest images
docker-compose pull

# Restart with new images
docker-compose up -d

# Verify update
docker-compose exec hypervisord hypervisord --version
```

### Update Configuration

```bash
# Edit .env or config.yaml
vim .env

# Restart to apply changes
docker-compose restart hypervisord
```

## Development

### Run in Development Mode

```bash
# Use override file
cat > docker-compose.override.yml << EOF
version: '3.8'
services:
  hypervisord:
    environment:
      - LOG_LEVEL=debug
    volumes:
      - ../../:/app:ro
EOF

docker-compose up -d
```

### Access Container Shell

```bash
# Shell into hypervisord
docker-compose exec hypervisord sh

# Shell into Prometheus
docker-compose exec prometheus sh
```

### Test API

```bash
# Health check
curl http://localhost:8080/health

# List jobs
curl http://localhost:8080/api/v1/jobs

# Get API version
curl http://localhost:8080/api/v1/version
```

## Production Deployment

For production use, consider:

1. **Use specific image tags** instead of `latest`
2. **Set strong passwords** in `.env` file
3. **Enable TLS** for API endpoints
4. **Configure backups** for volumes
5. **Set resource limits** in docker-compose.yml
6. **Use secrets** instead of environment variables
7. **Monitor disk space** for exports volume
8. **Set up log rotation**
9. **Use external Redis** for better reliability
10. **Consider Kubernetes** for HA deployments

## Additional Resources

- Main documentation: ../../README.md
- Kubernetes deployment: ../kubernetes/README.md
- API documentation: ../../docs/API.md
- Configuration reference: ../../docs/CONFIGURATION.md
