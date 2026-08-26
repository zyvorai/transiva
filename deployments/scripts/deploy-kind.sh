#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# =============================================================================
# HyperSDK kind Deployment Script
# =============================================================================
# Deploy HyperSDK to kind (Kubernetes in Docker) for upstream-like testing
#
# Usage:
#   ./deploy-kind.sh [OPTIONS]
#
# Options:
#   -n, --name NAME          Cluster name (default: transiva)
#   -w, --workers NUM        Number of worker nodes (default: 2)
#   -e, --env ENV            Environment: dev, staging, prod (default: dev)
#   -m, --metallb            Install MetalLB for LoadBalancer support
#   -i, --ingress            Install nginx ingress controller
#   -d, --delete             Delete existing cluster first
#   -h, --help               Show this help message
#
# Examples:
#   ./deploy-kind.sh
#   ./deploy-kind.sh --name test --workers 3 --metallb
#   ./deploy-kind.sh --delete --env production --ingress
#
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Defaults
CLUSTER_NAME="transiva"
WORKERS=2
ENVIRONMENT="development"
INSTALL_METALLB=false
INSTALL_INGRESS=false
DELETE_EXISTING=false
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# =============================================================================
# Helper Functions
# =============================================================================

log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

check_dependencies() {
    local missing_deps=()

    if ! command -v kind &> /dev/null; then
        missing_deps+=("kind")
    fi

    if ! command -v kubectl &> /dev/null; then
        missing_deps+=("kubectl")
    fi

    if ! command -v docker &> /dev/null; then
        missing_deps+=("docker")
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing dependencies: ${missing_deps[*]}"
        echo ""
        echo "Installation:"
        echo "  kind:    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/"
        echo "  kubectl: https://kubernetes.io/docs/tasks/tools/"
        echo "  docker:  https://docs.docker.com/get-docker/"
        exit 1
    fi
}

show_help() {
    grep '^#' "$0" | grep -v '#!/usr/bin/env' | sed 's/^# //g' | sed 's/^#//g'
}

# =============================================================================
# Parse Arguments
# =============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        -w|--workers)
            WORKERS="$2"
            shift 2
            ;;
        -e|--env)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -m|--metallb)
            INSTALL_METALLB=true
            shift
            ;;
        -i|--ingress)
            INSTALL_INGRESS=true
            shift
            ;;
        -d|--delete)
            DELETE_EXISTING=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# =============================================================================
# Main Script
# =============================================================================

log_info "HyperSDK kind Deployment Script"
echo ""
echo "Configuration:"
echo "  Cluster Name:  $CLUSTER_NAME"
echo "  Workers:       $WORKERS"
echo "  Environment:   $ENVIRONMENT"
echo "  MetalLB:       $INSTALL_METALLB"
echo "  Ingress:       $INSTALL_INGRESS"
echo ""

# Check dependencies
log_info "Checking dependencies..."
check_dependencies
log_success "All dependencies found"

# Delete existing cluster if requested
if [ "$DELETE_EXISTING" = true ]; then
    if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
        log_info "Deleting existing cluster: $CLUSTER_NAME"
        kind delete cluster --name "$CLUSTER_NAME"
        log_success "Cluster deleted"
    fi
fi

# Check if cluster exists
if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    log_warning "Cluster '$CLUSTER_NAME' already exists"
    read -p "Delete and recreate? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kind delete cluster --name "$CLUSTER_NAME"
        log_success "Cluster deleted"
    else
        log_info "Using existing cluster"
        kubectl cluster-info --context "kind-$CLUSTER_NAME"
        exit 0
    fi
fi

# Create kind config
log_info "Creating kind cluster configuration..."

KIND_CONFIG="/tmp/kind-${CLUSTER_NAME}-config.yaml"

cat > "$KIND_CONFIG" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: $CLUSTER_NAME
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF

# Add worker nodes
for ((i=0; i<WORKERS; i++)); do
    cat >> "$KIND_CONFIG" <<EOF
- role: worker
EOF
done

log_success "Configuration created"

# Create cluster
log_info "Creating kind cluster (this may take a few minutes)..."
kind create cluster --config "$KIND_CONFIG" --wait 5m

log_success "Cluster created: $CLUSTER_NAME"

# Set kubectl context
kubectl cluster-info --context "kind-$CLUSTER_NAME"
kubectl config use-context "kind-$CLUSTER_NAME"

# Wait for nodes
log_info "Waiting for nodes to be ready..."
kubectl wait --for=condition=ready node --all --timeout=300s
log_success "All nodes ready"

# Show cluster info
echo ""
kubectl get nodes
echo ""

# Install MetalLB if requested
if [ "$INSTALL_METALLB" = true ]; then
    log_info "Installing MetalLB..."

    kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml

    log_info "Waiting for MetalLB to be ready..."
    kubectl wait --namespace metallb-system \
        --for=condition=ready pod \
        --selector=app=metallb \
        --timeout=90s

    # Get kind network subnet
    KIND_NETWORK="kind"
    SUBNET=$(docker network inspect "$KIND_NETWORK" | jq -r '.[0].IPAM.Config[0].Subnet')
    # Calculate IP range (last 50 IPs of subnet)
    BASE_IP=$(echo "$SUBNET" | cut -d'.' -f1-3)
    START_IP="${BASE_IP}.200"
    END_IP="${BASE_IP}.250"

    log_info "Configuring MetalLB IP pool: $START_IP-$END_IP"

    kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - $START_IP-$END_IP
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
  - default
EOF

    log_success "MetalLB installed and configured"
fi

# Install nginx ingress if requested
if [ "$INSTALL_INGRESS" = true ]; then
    log_info "Installing nginx ingress controller..."

    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

    log_info "Waiting for ingress controller to be ready..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=90s

    log_success "Nginx ingress controller installed"
fi

# Create namespace
log_info "Creating namespace: transiva"
kubectl create namespace transiva || true
log_success "Namespace created"

# Create secrets
log_info "Creating secrets..."
kubectl create secret generic vsphere-credentials \
    --from-literal=url="https://vcenter.example.com/sdk" \
    --from-literal=username="administrator@vsphere.local" \
    --from-literal=password="change-me" \
    --from-literal=insecure="1" \
    -n transiva --dry-run=client -o yaml | kubectl apply -f -
log_success "Secrets created (update with real credentials)"

# Deploy HyperSDK
log_info "Deploying HyperSDK ($ENVIRONMENT environment)..."
kubectl apply -k "$REPO_ROOT/deployments/kubernetes/overlays/$ENVIRONMENT"
log_success "HyperSDK deployed"

# Wait for deployment
log_info "Waiting for HyperSDK to be ready..."
kubectl wait --for=condition=ready pod \
    -l app=hypervisord \
    -n transiva \
    --timeout=300s || {
    log_error "Deployment failed"
    echo ""
    kubectl get pods -n transiva
    echo ""
    kubectl logs -n transiva -l app=hypervisord --tail=50
    exit 1
}
log_success "HyperSDK is ready"

# Get service info
echo ""
log_info "Service Information:"
kubectl get svc -n transiva

# Get LoadBalancer IP if MetalLB installed
if [ "$INSTALL_METALLB" = true ]; then
    echo ""
    log_info "Waiting for LoadBalancer IP..."
    sleep 5
    LB_IP=$(kubectl get svc hypervisord-external -n transiva \
        -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

    if [ -n "$LB_IP" ]; then
        log_success "LoadBalancer IP assigned: $LB_IP"
        echo ""
        echo "Access HyperSDK:"
        echo "  Health:    curl http://$LB_IP/health"
        echo "  Status:    curl http://$LB_IP/status | jq"
        echo "  Dashboard: open http://$LB_IP/web/dashboard/"
    else
        log_warning "LoadBalancer IP not yet assigned"
        echo "Run: kubectl get svc hypervisord-external -n transiva -w"
    fi
else
    echo ""
    echo "Access via port-forward:"
    echo "  kubectl port-forward -n transiva svc/hypervisord 8080:8080"
    echo "  curl http://localhost:8080/health"
fi

# Show pod status
echo ""
log_info "Pod Status:"
kubectl get pods -n transiva

# Show quick reference
echo ""
log_info "Quick Reference:"
echo ""
echo "View logs:"
echo "  kubectl logs -n transiva -l app=hypervisord -f"
echo ""
echo "Delete deployment:"
echo "  kubectl delete namespace transiva"
echo ""
echo "Delete cluster:"
echo "  kind delete cluster --name $CLUSTER_NAME"
echo ""
echo "Load image to cluster:"
echo "  kind load docker-image <image> --name $CLUSTER_NAME"
echo ""

log_success "Deployment complete! 🎉"
