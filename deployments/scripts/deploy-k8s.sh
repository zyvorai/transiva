#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# ==============================================================================
# HyperSDK Kubernetes Deployment Script
# Deploys HyperSDK to Kubernetes using Kustomize
# ==============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="$(cd "${SCRIPT_DIR}/../kubernetes" && pwd)"

# Default values
ENVIRONMENT="${1:-development}"
NAMESPACE="${NAMESPACE:-transiva}"
ACTION="${ACTION:-apply}"
DRY_RUN="${DRY_RUN:-false}"

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

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

usage() {
    cat << EOF
Usage: $0 [ENVIRONMENT] [OPTIONS]

Deploy HyperSDK to Kubernetes using Kustomize

ENVIRONMENTS:
    development    Deploy to development environment (default)
    staging        Deploy to staging environment
    production     Deploy to production environment

OPTIONS:
    --namespace NAME       Kubernetes namespace (default: transiva)
    --action ACTION        Action to perform: apply, delete, diff (default: apply)
    --dry-run              Perform dry-run without making changes
    -h, --help             Show this help message

EXAMPLES:
    # Deploy to development
    $0 development

    # Deploy to production
    $0 production

    # Dry-run deployment
    $0 staging --dry-run

    # Show diff without applying
    $0 production --action diff

    # Delete deployment
    $0 development --action delete

PREREQUISITES:
    - kubectl must be installed and configured
    - kustomize must be installed (or use kubectl with kustomize support)
    - Secrets must be configured before deployment

EOF
}

check_prerequisites() {
    log_step "Checking prerequisites..."

    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi

    # Check kubectl context
    local current_context
    current_context=$(kubectl config current-context 2>/dev/null || echo "none")
    log_info "Current kubectl context: ${current_context}"

    # Check kustomize
    if ! kubectl version --client | grep -q "kustomize"; then
        log_warn "kubectl does not have kustomize support, checking for standalone kustomize..."
        if ! command -v kustomize &> /dev/null; then
            log_error "kustomize is not available"
            exit 1
        fi
    fi

    log_info "Prerequisites check passed"
}

check_secrets() {
    log_step "Checking secrets configuration..."

    local secrets_file="${K8S_DIR}/overlays/${ENVIRONMENT}/secrets.yaml"
    local example_file="${K8S_DIR}/base/secrets.yaml.example"

    if [ ! -f "${secrets_file}" ]; then
        log_warn "Secrets file not found: ${secrets_file}"
        log_warn "Please create secrets.yaml from the example:"
        log_warn "  cp ${example_file} ${secrets_file}"
        log_warn "  # Edit ${secrets_file} with your credentials"
        log_warn "  kubectl apply -f ${secrets_file} -n ${NAMESPACE}"

        read -p "Continue without secrets? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    else
        log_info "Secrets file found: ${secrets_file}"
    fi
}

create_namespace() {
    log_step "Creating namespace ${NAMESPACE}..."

    if kubectl get namespace "${NAMESPACE}" &> /dev/null; then
        log_info "Namespace ${NAMESPACE} already exists"
    else
        if [ "${DRY_RUN}" = "false" ]; then
            kubectl create namespace "${NAMESPACE}"
            log_info "Namespace ${NAMESPACE} created"
        else
            log_info "[DRY-RUN] Would create namespace ${NAMESPACE}"
        fi
    fi
}

apply_secrets() {
    log_step "Applying secrets..."

    local secrets_file="${K8S_DIR}/overlays/${ENVIRONMENT}/secrets.yaml"

    if [ -f "${secrets_file}" ]; then
        if [ "${DRY_RUN}" = "false" ]; then
            kubectl apply -f "${secrets_file}" -n "${NAMESPACE}"
            log_info "Secrets applied"
        else
            log_info "[DRY-RUN] Would apply secrets from ${secrets_file}"
        fi
    else
        log_warn "Skipping secrets (file not found)"
    fi
}

deploy_app() {
    local overlay_dir="${K8S_DIR}/overlays/${ENVIRONMENT}"

    if [ ! -d "${overlay_dir}" ]; then
        log_error "Environment overlay not found: ${overlay_dir}"
        exit 1
    fi

    log_step "Deploying HyperSDK to ${ENVIRONMENT} environment..."

    local kubectl_args=()

    if [ "${DRY_RUN}" = "true" ]; then
        kubectl_args+=("--dry-run=client")
    fi

    case "${ACTION}" in
        apply)
            kubectl apply -k "${overlay_dir}" -n "${NAMESPACE}" "${kubectl_args[@]}"
            ;;
        delete)
            kubectl delete -k "${overlay_dir}" -n "${NAMESPACE}" "${kubectl_args[@]}"
            ;;
        diff)
            kubectl diff -k "${overlay_dir}" -n "${NAMESPACE}" || true
            return
            ;;
        *)
            log_error "Unknown action: ${ACTION}"
            exit 1
            ;;
    esac

    if [ "${ACTION}" = "apply" ] && [ "${DRY_RUN}" = "false" ]; then
        log_info "Deployment initiated"
    fi
}

wait_for_deployment() {
    if [ "${ACTION}" != "apply" ] || [ "${DRY_RUN}" = "true" ]; then
        return
    fi

    log_step "Waiting for deployment to be ready..."

    if kubectl rollout status deployment/hypervisord -n "${NAMESPACE}" --timeout=300s; then
        log_info "Deployment is ready!"
    else
        log_error "Deployment failed or timed out"
        exit 1
    fi
}

show_status() {
    if [ "${DRY_RUN}" = "true" ]; then
        return
    fi

    log_step "Deployment status:"

    echo
    echo "Pods:"
    kubectl get pods -n "${NAMESPACE}" -l app=hypervisord

    echo
    echo "Services:"
    kubectl get services -n "${NAMESPACE}" -l app=hypervisord

    echo
    echo "PVCs:"
    kubectl get pvc -n "${NAMESPACE}"

    echo
    echo "Ingress (if configured):"
    kubectl get ingress -n "${NAMESPACE}" 2>/dev/null || echo "No ingress configured"

    echo
    log_info "To view logs:"
    echo "  kubectl logs -n ${NAMESPACE} -l app=hypervisord -f"

    echo
    log_info "To port-forward API:"
    echo "  kubectl port-forward -n ${NAMESPACE} svc/hypervisord 8080:8080"
}

# ==============================================================================
# Main
# ==============================================================================

# Parse arguments
while [[ $# -gt 1 ]]; do
    case $2 in
        --namespace)
            NAMESPACE="$3"
            shift 2
            ;;
        --action)
            ACTION="$3"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $2"
            usage
            exit 1
            ;;
    esac
done

# Validate environment
case "${ENVIRONMENT}" in
    development|staging|production)
        log_info "Deploying to ${ENVIRONMENT} environment"
        ;;
    *)
        log_error "Invalid environment: ${ENVIRONMENT}"
        log_error "Valid environments: development, staging, production"
        exit 1
        ;;
esac

# Execute deployment
check_prerequisites
check_secrets
create_namespace

if [ "${ACTION}" = "apply" ]; then
    apply_secrets
fi

deploy_app
wait_for_deployment
show_status

if [ "${ACTION}" = "apply" ] && [ "${DRY_RUN}" = "false" ]; then
    log_info "Deployment complete!"
fi
