# HyperSDK Helm Chart

Official Helm chart for deploying HyperSDK to Kubernetes.

## TL;DR

```bash
helm repo add transiva https://ssahani.github.io/transiva/helm-charts
helm repo update
helm install my-transiva transiva/transiva
```

Or install from local chart:

```bash
helm install my-transiva ./deployments/helm/transiva \
  --set credentials.vsphere.enabled=true \
  --set credentials.vsphere.url="https://vcenter.example.com/sdk" \
  --set credentials.vsphere.username="admin" \
  --set credentials.vsphere.password="password"
```

## Introduction

This chart bootstraps a HyperSDK deployment on a Kubernetes cluster using the Helm package manager.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- PV provisioner support in the underlying infrastructure
- Cloud provider credentials (vSphere, Nutanix, AWS, Azure, or GCP)

## Installing the Chart

### From Helm Repository (Recommended)

```bash
# Add Helm repository
helm repo add transiva https://ssahani.github.io/transiva/helm-charts
helm repo update

# Search available charts
helm search repo transiva

# Install latest version
helm install my-transiva transiva/transiva \
  --create-namespace \
  --namespace transiva \
  --set credentials.vsphere.enabled=true \
  --set credentials.vsphere.url="https://vcenter.example.com/sdk" \
  --set credentials.vsphere.username="administrator@vsphere.local" \
  --set credentials.vsphere.password="your-password"

# Install specific version
helm install my-transiva transiva/transiva \
  --version 0.2.0 \
  --create-namespace \
  --namespace transiva
```

### From Local Chart

```bash
# Clone repository
git clone https://github.com/ssahani/transiva.git
cd transiva

# Install from local path
helm install my-transiva ./deployments/helm/transiva \
  --create-namespace \
  --namespace transiva \
  --set credentials.vsphere.enabled=true \
  --set credentials.vsphere.url="https://vcenter.example.com/sdk" \
  --set credentials.vsphere.username="administrator@vsphere.local" \
  --set credentials.vsphere.password="your-password"
```

### Install with Custom Values

```bash
# Create custom values file
cat > my-values.yaml <<EOF
credentials:
  vsphere:
    enabled: true
    url: "https://vcenter.example.com/sdk"
    username: "administrator@vsphere.local"
    password: "change-me"

replicaCount: 1

resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "4Gi"
    cpu: "2000m"

persistence:
  exports:
    size: 1Ti

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: transiva.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: transiva-tls
      hosts:
        - transiva.example.com
EOF

# Install with custom values
helm install my-transiva transiva/transiva \
  -f my-values.yaml \
  --namespace transiva \
  --create-namespace
```

### Install from Local Chart

```bash
cd /path/to/transiva

helm install my-transiva ./deployments/helm/transiva \
  -f my-values.yaml \
  --namespace transiva \
  --create-namespace
```

## Accessing the Application

HyperSDK exposes a web server on port 8080 and metrics on port 8081.

### Kubernetes (Ingress)

**Get the Ingress URL**:

```bash
# Get ingress hostname
kubectl get ingress my-transiva -n transiva

# Access in browser
http://my-transiva.example.com
```

**Configure Ingress**:

```yaml
ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: transiva.example.com
      paths:
        - path: /
          pathType: Prefix
```

### OpenShift (Route)

**Get the Route URL**:

```bash
# Get route URL
ROUTE_URL=$(oc get route my-transiva -n transiva -o jsonpath='{.spec.host}')
echo "HyperSDK Web Server: https://${ROUTE_URL}"

# Access in browser
firefox https://${ROUTE_URL}
```

**Configure Route** (see [OpenShift documentation](../../OPENSHIFT.md#accessing-the-web-server)):

```yaml
route:
  enabled: true
  host: transiva.apps.cluster.example.com
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect

# Disable Ingress when using Route
ingress:
  enabled: false
```

### Port Forwarding (Development)

**Forward Web Server**:

```bash
# Forward port 8080 to localhost
kubectl port-forward svc/my-transiva 8080:8080 -n transiva

# Access at
http://localhost:8080
```

**Forward Both Web and Metrics**:

```bash
# Forward both ports
kubectl port-forward svc/my-transiva 8080:8080 8081:8081 -n transiva

# Access web server
http://localhost:8080

# Access metrics
http://localhost:8081/metrics
```

### Quick Access Commands

```bash
# Check deployment status
kubectl get pods,svc,ingress -n transiva

# View logs
kubectl logs -f -l app.kubernetes.io/name=transiva -n transiva

# Get service URL (LoadBalancer)
kubectl get svc my-transiva -n transiva -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'

# Test health endpoint (if available)
curl http://localhost:8080/health
```

For more detailed access methods, see:
- [OpenShift Access Guide](../../OPENSHIFT.md#accessing-the-web-server)
- [Examples](examples/README.md)

## Uninstalling the Chart

```bash
helm uninstall my-transiva --namespace transiva
```

This removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

### Common Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas (use >1 only with PostgreSQL) | `1` |
| `image.repository` | Image repository | `ghcr.io/ssahani/transiva-hypervisord` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Service Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.metricsPort` | Metrics port | `8081` |
| `externalService.enabled` | Enable LoadBalancer service | `true` |
| `externalService.type` | External service type | `LoadBalancer` |

### Ingress Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.className` | Ingress class name | `nginx` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.hosts` | Ingress hosts | `[]` |
| `ingress.tls` | Ingress TLS configuration | `[]` |

### Storage Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `persistence.data.enabled` | Enable data persistence | `true` |
| `persistence.data.storageClass` | Storage class | `""` (cluster default) |
| `persistence.data.size` | Data volume size | `10Gi` |
| `persistence.exports.enabled` | Enable exports persistence | `true` |
| `persistence.exports.size` | Exports volume size | `500Gi` |
| `persistence.exports.accessMode` | Access mode | `ReadWriteOnce` |

### Credentials Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `credentials.vsphere.enabled` | Enable vSphere credentials | `false` |
| `credentials.vsphere.url` | vCenter URL | `""` |
| `credentials.vsphere.username` | vSphere username | `""` |
| `credentials.vsphere.password` | vSphere password | `""` |
| `credentials.vsphere.existingSecret` | Use existing secret | `""` |
| `credentials.aws.enabled` | Enable AWS credentials | `false` |
| `credentials.aws.accessKeyId` | AWS access key | `""` |
| `credentials.aws.secretAccessKey` | AWS secret key | `""` |
| `credentials.aws.region` | AWS region | `us-east-1` |
| `credentials.azure.enabled` | Enable Azure credentials | `false` |
| `credentials.gcp.enabled` | Enable GCP credentials | `false` |

### Autoscaling Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `autoscaling.enabled` | Enable HPA | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `2` |
| `autoscaling.maxReplicas` | Maximum replicas | `10` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU | `70` |
| `autoscaling.targetMemoryUtilizationPercentage` | Target memory | `80` |

### Monitoring Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `monitoring.serviceMonitor.enabled` | Enable ServiceMonitor | `false` |
| `monitoring.serviceMonitor.interval` | Scrape interval | `30s` |
| `monitoring.prometheusRule.enabled` | Enable PrometheusRule | `false` |

## Examples

### Example 1: Development Environment

```yaml
# dev-values.yaml
replicaCount: 1

credentials:
  vsphere:
    enabled: true
    url: "https://vcenter.dev.example.com/sdk"
    username: "admin"
    password: "dev-password"

resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "1Gi"
    cpu: "500m"

persistence:
  data:
    size: 5Gi
  exports:
    size: 50Gi

config:
  logLevel: debug
```

Install:
```bash
helm install dev transiva/transiva \
  -f dev-values.yaml \
  --namespace transiva-dev \
  --create-namespace
```

### Example 2: Production with HA (requires PostgreSQL)

```yaml
# prod-values.yaml
replicaCount: 3

credentials:
  vsphere:
    enabled: true
    existingSecret: "vsphere-prod-credentials"

resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "4Gi"
    cpu: "2000m"

persistence:
  data:
    storageClass: "ssd"
    size: 20Gi
  exports:
    storageClass: "standard"
    size: 2Ti
    # For RWX storage:
    # accessMode: ReadWriteMany

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

podDisruptionBudget:
  enabled: true
  minAvailable: 2

monitoring:
  serviceMonitor:
    enabled: true
    labels:
      release: prometheus

networkPolicy:
  enabled: true

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
  hosts:
    - host: transiva.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: transiva-tls
      hosts:
        - transiva.example.com

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - transiva
          topologyKey: topology.kubernetes.io/zone
```

Install:
```bash
# Create secret first
kubectl create secret generic vsphere-prod-credentials \
  --from-literal=url="https://vcenter.example.com/sdk" \
  --from-literal=username="prod-user" \
  --from-literal=password="prod-password" \
  --from-literal=insecure="0" \
  -n transiva

# Install chart
helm install prod transiva/transiva \
  -f prod-values.yaml \
  --namespace transiva \
  --create-namespace
```

### Example 3: Multi-Cloud Setup

```yaml
# multi-cloud-values.yaml
credentials:
  vsphere:
    enabled: true
    url: "https://vcenter.example.com/sdk"
    username: "admin"
    password: "vsphere-pass"

  aws:
    enabled: true
    accessKeyId: "AKIA..."
    secretAccessKey: "secret..."
    region: "us-east-1"

  azure:
    enabled: true
    subscriptionId: "00000000-0000-0000-0000-000000000000"
    tenantId: "00000000-0000-0000-0000-000000000000"
    clientId: "00000000-0000-0000-0000-000000000000"
    clientSecret: "azure-secret"

  gcp:
    enabled: true
    projectId: "my-project"
    serviceAccountJSON: |
      {
        "type": "service_account",
        ...
      }

persistence:
  exports:
    size: 2Ti
```

### Example 4: GKE with Workload Identity

```yaml
# gke-values.yaml
serviceAccount:
  create: true
  annotations:
    iam.gke.io/gcp-service-account: transiva@PROJECT_ID.iam.gserviceaccount.com

persistence:
  data:
    storageClass: "standard-rwo"
  exports:
    storageClass: "standard-rwo"
    size: 1Ti

externalService:
  type: LoadBalancer
  annotations:
    cloud.google.com/load-balancer-type: "Internal"

credentials:
  vsphere:
    enabled: true
    existingSecret: "vsphere-credentials"
```

### Example 5: EKS with IRSA

```yaml
# eks-values.yaml
serviceAccount:
  create: true
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT_ID:role/transiva-role

persistence:
  data:
    storageClass: "gp3"
  exports:
    storageClass: "gp3"
    size: 1Ti

credentials:
  vsphere:
    enabled: true
    existingSecret: "vsphere-credentials"
```

### Example 6: Red Hat OpenShift

OpenShift requires specific configurations for Routes and SecurityContextConstraints.

**Quick Start**:

```bash
# Install with OpenShift-specific values
helm install transiva transiva/transiva \
  -f https://raw.githubusercontent.com/ssahani/transiva/main/deployments/helm/transiva/examples/openshift-values.yaml \
  -n transiva --create-namespace

# Or with custom route hostname
helm install transiva transiva/transiva \
  -f examples/openshift-values.yaml \
  --set route.host=transiva.apps.$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}') \
  -n transiva
```

**Features**:
- OpenShift Routes (instead of Ingress)
- Custom SecurityContextConstraints (SCC)
- OpenShift cluster monitoring integration
- Network policies for OpenShift
- Compatible with random UID assignment

**See**: [../../OPENSHIFT.md](../../OPENSHIFT.md) for comprehensive OpenShift deployment guide including GitOps, Pipelines, and production best practices.

## Upgrading

### To 0.2.0

Version 0.2.0 introduces breaking changes:

- `config.webhooks` changed from map to array
- New required init container for permissions

Upgrade:
```bash
helm upgrade my-transiva transiva/transiva \
  -f my-values.yaml \
  --namespace transiva
```

## Troubleshooting

### Pods stuck in Pending

```bash
# Check PVC status
kubectl get pvc -n transiva

# Check events
kubectl describe pod -n transiva -l app=hypervisord
```

### Cannot access LoadBalancer

```bash
# Check service
kubectl get svc -n transiva

# Check LoadBalancer status
kubectl describe svc my-transiva-external -n transiva
```

### Credentials not working

```bash
# Check secret
kubectl get secret -n transiva

# Verify secret data
kubectl get secret my-transiva-vsphere -n transiva -o yaml
```

## Further Information

- [Kubernetes Deployment Guide](../../docs/guides/upstream-kubernetes.md)
- [OpenShift Deployment Guide](../../OPENSHIFT.md)
- [Configuration Reference](../../docs/tutorials/configuration.md)
- [API Documentation](../../docs/api/README.md)

## License

Apache-2.0
