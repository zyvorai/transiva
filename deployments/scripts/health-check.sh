#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# ==============================================================================
# HyperSDK Health Check Script
# Validates deployment health across different environments
# ==============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
MODE="${1:-docker}"
NAMESPACE="${NAMESPACE:-transiva}"
TIMEOUT="${TIMEOUT:-30}"

# ==============================================================================
# Functions
# ==============================================================================

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_check() {
    echo -e "${BLUE}[CHECK]${NC} $1"
}

check_success() {
    echo -e "${GREEN}✓${NC} $1"
}

check_failure() {
    echo -e "${RED}✗${NC} $1"
}

usage() {
    cat << EOF
Usage: $0 [MODE] [OPTIONS]

Health check for HyperSDK deployment

MODES:
    docker         Check Docker/Docker Compose deployment (default)
    podman         Check Podman deployment
    kubernetes     Check Kubernetes deployment
    k8s            Alias for kubernetes

OPTIONS:
    --namespace NAME    Kubernetes namespace (default: transiva)
    --timeout SECONDS   Timeout for health checks (default: 30)
    -h, --help          Show this help message

EXAMPLES:
    # Check Docker deployment
    $0 docker

    # Check Kubernetes deployment
    $0 kubernetes --namespace transiva

    # Check with custom timeout
    $0 docker --timeout 60

EOF
}

# ==============================================================================
# Docker Health Checks
# ==============================================================================

check_docker() {
    log_info "Checking Docker deployment..."

    # Check Docker daemon
    log_check "Docker daemon status"
    if docker info &> /dev/null; then
        check_success "Docker is running"
    else
        check_failure "Docker is not running"
        return 1
    fi

    # Check containers
    log_check "Container status"
    local containers=("transiva-hypervisord" "transiva-redis" "transiva-prometheus" "transiva-grafana")
    local failed=0

    for container in "${containers[@]}"; do
        if docker ps --filter "name=${container}" --filter "status=running" | grep -q "${container}"; then
            check_success "${container} is running"
        else
            check_failure "${container} is not running"
            ((failed++))
        fi
    done

    if [ $failed -gt 0 ]; then
        log_error "${failed} container(s) not running"
        return 1
    fi

    # Check hypervisord health endpoint
    log_check "HyperSDK API health"
    if curl -sf http://localhost:8080/health > /dev/null; then
        check_success "API health endpoint responding"
    else
        check_failure "API health endpoint not responding"
        return 1
    fi

    # Check metrics endpoint
    log_check "Metrics endpoint"
    if curl -sf http://localhost:8081/metrics > /dev/null; then
        check_success "Metrics endpoint responding"
    else
        check_failure "Metrics endpoint not responding"
        return 1
    fi

    # Check Prometheus
    log_check "Prometheus status"
    if curl -sf http://localhost:9090/-/healthy > /dev/null; then
        check_success "Prometheus is healthy"
    else
        check_warn "Prometheus health check failed"
    fi

    # Check Grafana
    log_check "Grafana status"
    if curl -sf http://localhost:3000/api/health > /dev/null; then
        check_success "Grafana is healthy"
    else
        check_warn "Grafana health check failed"
    fi

    log_info "Docker health check completed successfully!"
    return 0
}

# ==============================================================================
# Podman Health Checks
# ==============================================================================

check_podman() {
    log_info "Checking Podman deployment..."

    # Check Podman daemon
    log_check "Podman status"
    if podman info &> /dev/null; then
        check_success "Podman is running"
    else
        check_failure "Podman is not running"
        return 1
    fi

    # Check containers
    log_check "Container status"
    local failed=0

    if podman ps --filter "name=hypervisord" --filter "status=running" | grep -q "hypervisord"; then
        check_success "hypervisord is running"
    else
        check_failure "hypervisord is not running"
        ((failed++))
    fi

    if [ $failed -gt 0 ]; then
        return 1
    fi

    # Check health endpoint
    log_check "HyperSDK API health"
    if curl -sf http://localhost:8080/health > /dev/null; then
        check_success "API health endpoint responding"
    else
        check_failure "API health endpoint not responding"
        return 1
    fi

    log_info "Podman health check completed successfully!"
    return 0
}

# ==============================================================================
# Kubernetes Health Checks
# ==============================================================================

check_kubernetes() {
    log_info "Checking Kubernetes deployment in namespace: ${NAMESPACE}..."

    # Check kubectl
    log_check "kubectl connection"
    if kubectl cluster-info &> /dev/null; then
        check_success "kubectl connected to cluster"
    else
        check_failure "kubectl cannot connect to cluster"
        return 1
    fi

    # Check namespace
    log_check "Namespace existence"
    if kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        check_success "Namespace ${NAMESPACE} exists"
    else
        check_failure "Namespace ${NAMESPACE} not found"
        return 1
    fi

    # Check deployment
    log_check "Deployment status"
    if kubectl get deployment hypervisord -n "${NAMESPACE}" &> /dev/null; then
        local ready
        ready=$(kubectl get deployment hypervisord -n "${NAMESPACE}" -o jsonpath='{.status.readyReplicas}')
        local desired
        desired=$(kubectl get deployment hypervisord -n "${NAMESPACE}" -o jsonpath='{.status.replicas}')

        if [ "${ready:-0}" -eq "${desired:-0}" ] && [ "${desired:-0}" -gt 0 ]; then
            check_success "Deployment is ready (${ready}/${desired} replicas)"
        else
            check_failure "Deployment not ready (${ready:-0}/${desired:-0} replicas)"
            return 1
        fi
    else
        check_failure "Deployment not found"
        return 1
    fi

    # Check pods
    log_check "Pod status"
    local pod_status
    pod_status=$(kubectl get pods -n "${NAMESPACE}" -l app=hypervisord -o jsonpath='{.items[*].status.phase}')

    if echo "${pod_status}" | grep -q "Running"; then
        check_success "Pods are running"
    else
        check_failure "Pods not running (status: ${pod_status})"
        return 1
    fi

    # Check service
    log_check "Service status"
    if kubectl get service hypervisord -n "${NAMESPACE}" &> /dev/null; then
        check_success "Service exists"
    else
        check_failure "Service not found"
        return 1
    fi

    # Check PVCs
    log_check "PersistentVolumeClaim status"
    local pvc_status
    pvc_status=$(kubectl get pvc -n "${NAMESPACE}" -o jsonpath='{.items[*].status.phase}')

    if echo "${pvc_status}" | grep -q "Bound"; then
        check_success "PVCs are bound"
    else
        check_warn "PVC status: ${pvc_status}"
    fi

    # Port-forward and check health endpoint
    log_check "API health endpoint"
    local pod_name
    pod_name=$(kubectl get pods -n "${NAMESPACE}" -l app=hypervisord -o jsonpath='{.items[0].metadata.name}')

    if [ -n "${pod_name}" ]; then
        if kubectl exec -n "${NAMESPACE}" "${pod_name}" -- curl -sf http://localhost:8080/health > /dev/null 2>&1; then
            check_success "API health endpoint responding"
        else
            check_failure "API health endpoint not responding"
            return 1
        fi
    fi

    log_info "Kubernetes health check completed successfully!"
    return 0
}

# ==============================================================================
# Main
# ==============================================================================

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        docker|podman|kubernetes|k8s)
            MODE="$1"
            shift
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Execute health check based on mode
case "${MODE}" in
    docker)
        check_docker
        ;;
    podman)
        check_podman
        ;;
    kubernetes|k8s)
        check_kubernetes
        ;;
    *)
        log_error "Invalid mode: ${MODE}"
        usage
        exit 1
        ;;
esac
