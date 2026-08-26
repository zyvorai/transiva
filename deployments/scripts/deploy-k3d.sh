#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# =============================================================================
# HyperSDK k3d Deployment Script
# =============================================================================
# Automatically creates a k3d cluster and deploys HyperSDK
#
# Usage:
#   ./deploy-k3d.sh [OPTIONS]
#
# Options:
#   -n, --name NAME          Cluster name (default: transiva)
#   -a, --agents NUM         Number of agent nodes (default: 2)
#   -e, --env ENV            Environment: dev, staging, prod (default: dev)
#   -m, --monitoring         Deploy monitoring stack
#   -r, --registry           Create local registry
#   -p, --port PORT          LoadBalancer port (default: 8080)
#   -v, --volume PATH        Mount host volume for exports
#   -d, --delete             Delete existing cluster first
#   -h, --help               Show this help message
#
# Examples:
#   ./deploy-k3d.sh
#   ./deploy-k3d.sh --name test --agents 3
#   ./deploy-k3d.sh --monitoring --registry
#   ./deploy-k3d.sh --delete --env staging
#
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
CLUSTER_NAME="transiva"
AGENTS=2
ENVIRONMENT="development"
MONITORING=false
REGISTRY=false
LB_PORT=8080
DELETE_EXISTING=false
VOLUME_PATH=""
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

    if ! command -v k3d &> /dev/null; then
        missing_deps+=("k3d")
    fi

    if ! command -v kubectl &> /dev/null; then
        missing_deps+=("kubectl")
    fi

    if ! command -v docker &> /dev/null && ! command -v podman &> /dev/null; then
        missing_deps+=("docker or podman")
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing dependencies: ${missing_deps[*]}"
        echo ""
        echo "Installation instructions:"
        echo "  k3d:     https://k3d.io/#installation"
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
        -a|--agents)
            AGENTS="$2"
            shift 2
            ;;
        -e|--env)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -m|--monitoring)
            MONITORING=true
            shift
            ;;
        -r|--registry)
            REGISTRY=true
            shift
            ;;
        -p|--port)
            LB_PORT="$2"
            shift 2
            ;;
        -v|--volume)
            VOLUME_PATH="$2"
            shift 2
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
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Validate environment
if [[ ! "$ENVIRONMENT" =~ ^(development|staging|production)$ ]]; then
    log_error "Invalid environment: $ENVIRONMENT"
    echo "Valid environments: development, staging, production"
    exit 1
fi

# =============================================================================
# Main Script
# =============================================================================

log_info "HyperSDK k3d Deployment Script"
echo ""
echo "Configuration:"
echo "  Cluster Name:  $CLUSTER_NAME"
echo "  Agent Nodes:   $AGENTS"
echo "  Environment:   $ENVIRONMENT"
echo "  Monitoring:    $MONITORING"
echo "  Registry:      $REGISTRY"
echo "  LB Port:       $LB_PORT"
echo "  Volume Mount:  ${VOLUME_PATH:-none}"
echo ""

# Check dependencies
log_info "Checking dependencies..."
check_dependencies
log_success "All dependencies found"

# Delete existing cluster if requested
if [ "$DELETE_EXISTING" = true ]; then
    if k3d cluster list | grep -q "^$CLUSTER_NAME "; then
        log_info "Deleting existing cluster: $CLUSTER_NAME"
        k3d cluster delete "$CLUSTER_NAME"
        log_success "Cluster deleted"
    fi
fi

# Check if cluster already exists
if k3d cluster list | grep -q "^$CLUSTER_NAME "; then
    log_warning "Cluster '$CLUSTER_NAME' already exists"
    read -p "Delete and recreate? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        k3d cluster delete "$CLUSTER_NAME"
        log_success "Cluster deleted"
    else
        log_info "Using existing cluster"
        export KUBECONFIG="$(k3d kubeconfig write "$CLUSTER_NAME")"
        kubectl cluster-info
        exit 0
    fi
fi

# Build k3d cluster creation command
log_info "Creating k3d cluster..."

CREATE_CMD="k3d cluster create $CLUSTER_NAME"
CREATE_CMD+=" --agents $AGENTS"
CREATE_CMD+=" --port \"$LB_PORT:80@loadbalancer\""
CREATE_CMD+=" --port \"9090:9090@server:0\""
CREATE_CMD+=" --k3s-arg \"--disable=traefik@server:0\""

if [ "$REGISTRY" = true ]; then
    CREATE_CMD+=" --registry-create ${CLUSTER_NAME}-registry:5000"
fi

if [ -n "$VOLUME_PATH" ]; then
    # Create host directory if it doesn't exist
    mkdir -p "$VOLUME_PATH"
    CREATE_CMD+=" --volume \"$VOLUME_PATH:/exports@all\""
fi

# Create cluster
eval "$CREATE_CMD"

# Set kubeconfig
export KUBECONFIG="$(k3d kubeconfig write "$CLUSTER_NAME")"
log_success "Cluster created: $CLUSTER_NAME"

# Wait for cluster to be ready
log_info "Waiting for cluster to be ready..."
kubectl wait --for=condition=ready node --all --timeout=300s
log_success "Cluster is ready"

# Show cluster info
echo ""
kubectl cluster-info
echo ""
kubectl get nodes
echo ""

# Create namespace
log_info "Creating namespace: transiva"
kubectl create namespace transiva || true
log_success "Namespace created"

# Create secrets
log_info "Creating secrets..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: vsphere-credentials
  namespace: transiva
type: Opaque
stringData:
  url: "https://vcenter.example.com/sdk"
  username: "administrator@vsphere.local"
  password: "change-me"
  insecure: "1"
EOF
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
    log_error "Deployment failed to become ready"
    echo ""
    echo "Pod status:"
    kubectl get pods -n transiva
    echo ""
    echo "Pod logs:"
    kubectl logs -n transiva -l app=hypervisord --tail=50
    exit 1
}
log_success "HyperSDK is ready"

# Deploy monitoring if requested
if [ "$MONITORING" = true ]; then
    log_info "Deploying monitoring stack..."

    # Deploy Prometheus
    kubectl create namespace monitoring || true
    kubectl apply -f "$REPO_ROOT/monitoring/prometheus/" || {
        log_warning "Monitoring deployment failed (optional)"
    }

    log_success "Monitoring deployed"
fi

# Get service information
echo ""
log_info "Service Information:"
echo ""
kubectl get svc -n transiva

# Get LoadBalancer IP
LB_IP=$(kubectl get svc hypervisord-external -n transiva \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

echo ""
if [ -n "$LB_IP" ]; then
    log_success "Deployment successful!"
    echo ""
    echo "Access HyperSDK:"
    echo "  API:       http://$LB_IP/health"
    echo "  Status:    http://$LB_IP/status"
    echo "  Dashboard: http://$LB_IP/web/dashboard/"
    echo ""
    echo "Test the deployment:"
    echo "  curl http://$LB_IP/health"
    echo "  curl http://$LB_IP/status | jq"
else
    log_success "Deployment successful!"
    echo ""
    echo "LoadBalancer IP not yet assigned (may take 30-60 seconds)"
    echo ""
    echo "Get LoadBalancer IP:"
    echo "  kubectl get svc hypervisord-external -n transiva"
    echo ""
    echo "Or use port-forward:"
    echo "  kubectl port-forward -n transiva svc/hypervisord 8080:8080"
    echo "  curl http://localhost:8080/health"
fi

# Export kubeconfig info
echo ""
echo "Cluster kubeconfig:"
echo "  export KUBECONFIG=\"$(k3d kubeconfig write "$CLUSTER_NAME")\""

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
echo "Restart deployment:"
echo "  kubectl rollout restart deployment/hypervisord -n transiva"
echo ""
echo "Scale deployment:"
echo "  kubectl scale deployment/hypervisord --replicas=3 -n transiva"
echo ""
echo "Delete deployment:"
echo "  kubectl delete namespace transiva"
echo ""
echo "Delete cluster:"
echo "  k3d cluster delete $CLUSTER_NAME"
echo ""

# Create kubeconfig file in /tmp for easy access
KUBECONFIG_FILE="/tmp/k3d-${CLUSTER_NAME}-config.yaml"
k3d kubeconfig get "$CLUSTER_NAME" > "$KUBECONFIG_FILE"
log_success "Kubeconfig saved to: $KUBECONFIG_FILE"

log_success "Deployment complete! 🎉"
