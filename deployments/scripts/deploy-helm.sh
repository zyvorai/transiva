#!/bin/bash
# SPDX-License-Identifier: Apache-2.0


# HyperSDK Helm Deployment Script
# Quick deployment to various Kubernetes environments

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
RELEASE_NAME="${RELEASE_NAME:-transiva}"
NAMESPACE="${NAMESPACE:-transiva}"
CHART_PATH="${CHART_PATH:-deployments/helm/transiva}"
REPO_URL="https://ssahani.github.io/transiva/helm-charts"
USE_REPO="${USE_REPO:-false}"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $*"
}

# Usage
usage() {
    cat <<EOF
Usage: $0 [environment] [options]

Deploy HyperSDK to Kubernetes using Helm.

Environments:
    k3d         - k3d local development cluster
    kind        - KIND local testing cluster
    minikube    - Minikube local cluster
    gke         - Google Kubernetes Engine
    eks         - Amazon Elastic Kubernetes Service
    aks         - Azure Kubernetes Service
    production  - Generic production deployment
    custom      - Custom values file

Options:
    -h, --help                  Show this help message
    -n, --namespace NAME        Kubernetes namespace (default: ${NAMESPACE})
    -r, --release NAME          Helm release name (default: ${RELEASE_NAME})
    -f, --values FILE           Custom values file
    --from-repo                 Install from Helm repository (not local chart)
    --version VERSION           Chart version (when using --from-repo)
    --create-namespace          Create namespace if it doesn't exist
    --wait                      Wait for deployment to be ready
    --timeout DURATION          Timeout for wait (default: 5m)
    --dry-run                   Show what would be installed
    --upgrade                   Upgrade existing release (install if not exists)
    --set KEY=VALUE             Set Helm values (can be used multiple times)

Examples:
    # Deploy to k3d cluster
    $0 k3d

    # Deploy to GKE with custom namespace
    $0 gke --namespace production --create-namespace

    # Install from Helm repository
    $0 minikube --from-repo

    # Deploy with custom values
    $0 custom --values my-values.yaml

    # Upgrade existing deployment
    $0 production --upgrade --wait

    # Dry run to see generated manifests
    $0 kind --dry-run

    # Set specific values
    $0 k3d --set replicaCount=2 --set config.logLevel=debug

    # Install specific version from repository
    $0 gke --from-repo --version 0.2.0
EOF
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v helm &> /dev/null; then
        log_error "helm is not installed. Please install Helm 3.x"
        exit 1
    fi

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed. Please install kubectl"
        exit 1
    fi

    # Check cluster connection
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi

    local current_context=$(kubectl config current-context)
    log_info "Current Kubernetes context: ${current_context}"
}

# Add Helm repository
add_helm_repo() {
    if helm repo list | grep -q "^transiva"; then
        log_info "Helm repository already added, updating..."
        helm repo update transiva
    else
        log_info "Adding HyperSDK Helm repository..."
        helm repo add transiva "${REPO_URL}"
        helm repo update transiva
    fi
}

# Get values file for environment
get_values_file() {
    local env="$1"

    case "${env}" in
        k3d)
            echo "${CHART_PATH}/examples/k3d-values.yaml"
            ;;
        kind)
            echo "${CHART_PATH}/examples/kind-values.yaml"
            ;;
        minikube)
            echo "${CHART_PATH}/examples/minikube-values.yaml"
            ;;
        gke)
            echo "${CHART_PATH}/examples/gke-values.yaml"
            ;;
        eks)
            echo "${CHART_PATH}/examples/eks-values.yaml"
            ;;
        aks)
            echo "${CHART_PATH}/examples/aks-values.yaml"
            ;;
        production)
            echo "${CHART_PATH}/values.yaml"
            ;;
        custom)
            echo ""  # Will be provided via --values
            ;;
        *)
            log_error "Unknown environment: ${env}"
            log_info "Valid environments: k3d, kind, minikube, gke, eks, aks, production, custom"
            exit 1
            ;;
    esac
}

# Build Helm command
build_helm_command() {
    local env="$1"
    local cmd=""

    # Base command
    if [ "${DO_UPGRADE}" = "true" ]; then
        cmd="helm upgrade --install"
    else
        cmd="helm install"
    fi

    cmd="${cmd} ${RELEASE_NAME}"

    # Chart source
    if [ "${USE_REPO}" = "true" ]; then
        cmd="${cmd} transiva/transiva"
        if [ -n "${CHART_VERSION}" ]; then
            cmd="${cmd} --version ${CHART_VERSION}"
        fi
    else
        cmd="${cmd} ${CHART_PATH}"
    fi

    # Namespace
    cmd="${cmd} --namespace ${NAMESPACE}"

    # Create namespace
    if [ "${CREATE_NAMESPACE}" = "true" ]; then
        cmd="${cmd} --create-namespace"
    fi

    # Values file
    if [ -n "${CUSTOM_VALUES}" ]; then
        cmd="${cmd} --values ${CUSTOM_VALUES}"
    elif [ "${env}" != "custom" ]; then
        local values_file=$(get_values_file "${env}")
        if [ -f "${values_file}" ]; then
            cmd="${cmd} --values ${values_file}"
        fi
    fi

    # Additional set values
    for set_value in "${SET_VALUES[@]}"; do
        cmd="${cmd} --set ${set_value}"
    done

    # Wait
    if [ "${DO_WAIT}" = "true" ]; then
        cmd="${cmd} --wait --timeout ${WAIT_TIMEOUT}"
    fi

    # Dry run
    if [ "${DRY_RUN}" = "true" ]; then
        cmd="${cmd} --dry-run --debug"
    fi

    echo "${cmd}"
}

# Deploy HyperSDK
deploy() {
    local env="$1"

    log_step "Deploying HyperSDK to ${env} environment..."

    # Add repository if using repo
    if [ "${USE_REPO}" = "true" ]; then
        add_helm_repo
    fi

    # Build and execute Helm command
    local helm_cmd=$(build_helm_command "${env}")

    log_info "Executing: ${helm_cmd}"
    echo ""

    if eval "${helm_cmd}"; then
        log_success "Deployment successful!"

        if [ "${DRY_RUN}" != "true" ]; then
            echo ""
            show_access_info
        fi
    else
        log_error "Deployment failed"
        exit 1
    fi
}

# Show access information
show_access_info() {
    log_step "Access Information"
    echo ""

    # Get service info
    local service_type=$(kubectl get svc -n "${NAMESPACE}" "${RELEASE_NAME}" -o jsonpath='{.spec.type}' 2>/dev/null || echo "Unknown")

    log_info "Release: ${RELEASE_NAME}"
    log_info "Namespace: ${NAMESPACE}"
    log_info "Service Type: ${service_type}"
    echo ""

    case "${service_type}" in
        LoadBalancer)
            log_info "Waiting for LoadBalancer IP..."
            kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' \
                svc/"${RELEASE_NAME}" -n "${NAMESPACE}" --timeout=60s 2>/dev/null || true

            local external_ip=$(kubectl get svc -n "${NAMESPACE}" "${RELEASE_NAME}" \
                -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)

            if [ -n "${external_ip}" ]; then
                log_success "Access URL: http://${external_ip}:8080"
                echo ""
                log_info "Health Check: curl http://${external_ip}:8080/health"
                log_info "Dashboard: http://${external_ip}:8080/web/dashboard/"
                log_info "Metrics: curl http://${external_ip}:8081/metrics"
            else
                log_warning "LoadBalancer IP not yet assigned. Check with:"
                echo "  kubectl get svc -n ${NAMESPACE} ${RELEASE_NAME}"
            fi
            ;;
        NodePort)
            local node_port=$(kubectl get svc -n "${NAMESPACE}" "${RELEASE_NAME}" \
                -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null)
            local node_ip=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}' 2>/dev/null)

            if [ -n "${node_port}" ] && [ -n "${node_ip}" ]; then
                log_success "Access URL: http://${node_ip}:${node_port}"
                echo ""
                log_info "Health Check: curl http://${node_ip}:${node_port}/health"
                log_info "Dashboard: http://${node_ip}:${node_port}/web/dashboard/"
            fi
            ;;
        ClusterIP)
            log_info "Use port-forward to access the service:"
            echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME} 8080:8080"
            echo ""
            log_info "Then access at: http://localhost:8080"
            ;;
    esac

    echo ""
    log_step "Useful Commands"
    echo ""
    echo "  # Check deployment status"
    echo "  kubectl get all -n ${NAMESPACE}"
    echo ""
    echo "  # View logs"
    echo "  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=transiva -f"
    echo ""
    echo "  # Get Helm values"
    echo "  helm get values ${RELEASE_NAME} -n ${NAMESPACE}"
    echo ""
    echo "  # Upgrade deployment"
    echo "  helm upgrade ${RELEASE_NAME} transiva/transiva -n ${NAMESPACE}"
    echo ""
    echo "  # Uninstall"
    echo "  helm uninstall ${RELEASE_NAME} -n ${NAMESPACE}"
    echo ""
}

# Main execution
main() {
    local environment=""
    local create_namespace=false
    local do_wait=false
    local do_upgrade=false
    local dry_run=false
    local custom_values=""
    local chart_version=""
    local wait_timeout="5m"
    declare -a set_values=()

    # Parse arguments
    if [ $# -eq 0 ]; then
        usage
        exit 1
    fi

    environment="$1"
    shift

    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                usage
                exit 0
                ;;
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -r|--release)
                RELEASE_NAME="$2"
                shift 2
                ;;
            -f|--values)
                CUSTOM_VALUES="$2"
                shift 2
                ;;
            --from-repo)
                USE_REPO=true
                shift
                ;;
            --version)
                CHART_VERSION="$2"
                shift 2
                ;;
            --create-namespace)
                CREATE_NAMESPACE=true
                shift
                ;;
            --wait)
                DO_WAIT=true
                shift
                ;;
            --timeout)
                WAIT_TIMEOUT="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --upgrade)
                DO_UPGRADE=true
                shift
                ;;
            --set)
                set_values+=("$2")
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    # Export variables
    export CREATE_NAMESPACE DO_WAIT DO_UPGRADE DRY_RUN CUSTOM_VALUES CHART_VERSION WAIT_TIMEOUT
    export SET_VALUES=("${set_values[@]}")

    echo ""
    echo "========================================"
    echo "  HyperSDK Helm Deployment"
    echo "========================================"
    echo ""

    check_prerequisites
    deploy "${environment}"
}

main "$@"
