# HyperSDK Helm Chart Examples

This directory contains example values files for different deployment scenarios.

## Available Examples

### `openshift-values.yaml`

**Use this for**: Red Hat OpenShift Container Platform deployments

**Features**:
- OpenShift Routes (instead of Kubernetes Ingress)
- Custom SecurityContextConstraints (SCC)
- OpenShift monitoring integration
- Network policies for OpenShift
- Random UID assignment support
- Disabled init containers (not compatible with OpenShift security model)

**Installation**:

```bash
# Add Helm repository
helm repo add transiva https://ssahani.github.io/transiva/helm-charts
helm repo update

# Install with OpenShift values
helm install transiva transiva/transiva \
  -f https://raw.githubusercontent.com/ssahani/transiva/main/deployments/helm/transiva/examples/openshift-values.yaml \
  -n transiva --create-namespace

# Or if you have the file locally
helm install transiva transiva/transiva \
  -f examples/openshift-values.yaml \
  -n transiva --create-namespace
```

**Customization**:

You can override specific values:

```bash
helm install transiva transiva/transiva \
  -f examples/openshift-values.yaml \
  --set route.host=transiva.apps.my-cluster.example.com \
  --set persistence.data.storageClass=ocs-storagecluster-ceph-rbd \
  -n transiva
```

**Documentation**: See [../../../OPENSHIFT.md](../../../OPENSHIFT.md) for complete OpenShift deployment guide.

**Accessing the Application**:

After installation, access the HyperSDK web server (port 8080):

```bash
# Get the Route URL
ROUTE_URL=$(oc get route transiva -n transiva -o jsonpath='{.spec.host}')

# Access in browser
firefox https://$ROUTE_URL

# Or use port forwarding for development
oc port-forward svc/transiva 8080:8080 -n transiva
```

See [Accessing the Web Server](../../../OPENSHIFT.md#accessing-the-web-server) for all access methods.

## Creating Custom Values

You can create your own values file by starting with one of these examples:

```bash
# Download example
curl -O https://raw.githubusercontent.com/ssahani/transiva/main/deployments/helm/transiva/examples/openshift-values.yaml

# Edit for your environment
vi openshift-values.yaml

# Install with your custom values
helm install transiva transiva/transiva -f openshift-values.yaml -n transiva
```

## Testing Values

Before installing, you can test your values file:

```bash
# Dry run
helm install transiva transiva/transiva \
  -f examples/openshift-values.yaml \
  --dry-run --debug \
  -n transiva

# Template rendering
helm template transiva transiva/transiva \
  -f examples/openshift-values.yaml \
  -n transiva > rendered.yaml

# Review rendered manifests
less rendered.yaml
```

## Common Overrides

### Change Route Hostname (OpenShift)

```bash
--set route.host=transiva.apps.my-cluster.example.com
```

### Use Custom Storage Class

```bash
--set persistence.data.storageClass=fast-ssd \
--set persistence.exports.storageClass=slow-hdd
```

### Adjust Resources

```bash
--set resources.requests.cpu=500m \
--set resources.requests.memory=1Gi \
--set resources.limits.cpu=2000m \
--set resources.limits.memory=4Gi
```

### Enable Autoscaling

```bash
--set autoscaling.enabled=true \
--set autoscaling.minReplicas=2 \
--set autoscaling.maxReplicas=10
```

### Configure Cloud Credentials

```bash
# Create secret first
oc create secret generic vsphere-credentials \
  --from-literal=url=https://vcenter.example.com/sdk \
  --from-literal=username=admin@vsphere.local \
  --from-literal=password='changeme' \
  --from-literal=insecure=1 \
  -n transiva

# Reference in Helm
--set credentials.vsphere.enabled=true \
--set credentials.vsphere.existingSecret=vsphere-credentials
```

## Support

For issues or questions:
- **GitHub Issues**: https://github.com/ssahani/transiva/issues
- **Documentation**: https://github.com/ssahani/transiva
