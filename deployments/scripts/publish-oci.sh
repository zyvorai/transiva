#!/bin/bash
# SPDX-License-Identifier: Apache-2.0


# HyperSDK Helm Chart OCI Registry Publishing Script
# Publishes Helm charts to OCI-compliant registries (ghcr.io, Docker Hub, ECR, ACR, etc.)

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CHART_PATH="${CHART_PATH:-deployments/helm/transiva}"
OCI_REGISTRY="${OCI_REGISTRY:-ghcr.io}"
OCI_NAMESPACE="${OCI_NAMESPACE:-ssahani/charts}"
OUTPUT_DIR="${OUTPUT_DIR:-deployments/helm/packages}"

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

# Usage
usage() {
    cat <<EOF
Usage: $0 [options]

Publish HyperSDK Helm chart to OCI registry.

Options:
    -h, --help              Show this help message
    -c, --chart PATH        Path to Helm chart (default: ${CHART_PATH})
    -r, --registry URL      OCI registry URL (default: ${OCI_REGISTRY})
    -n, --namespace NAME    Registry namespace (default: ${OCI_NAMESPACE})
    -o, --output DIR        Output directory for packages (default: ${OUTPUT_DIR})
    -v, --version VERSION   Override chart version
    --username USER         Registry username (or use REGISTRY_USERNAME env)
    --password PASS         Registry password (or use REGISTRY_PASSWORD env)
    --skip-login            Skip registry login (already logged in)
    --skip-lint             Skip Helm lint before packaging
    --skip-test             Skip chart tests before packaging
    --tag-latest            Also tag as :latest

Examples:
    # Publish to GitHub Container Registry
    $0 --registry ghcr.io --namespace ssahani/charts

    # Publish to Docker Hub
    $0 --registry registry-1.docker.io --namespace myuser

    # Publish to AWS ECR
    $0 --registry 123456789.dkr.ecr.us-east-1.amazonaws.com --namespace charts

    # Publish to Azure ACR
    $0 --registry myregistry.azurecr.io --namespace charts

    # Publish with specific version
    $0 --version 0.3.0

    # Skip login (already authenticated)
    $0 --skip-login
EOF
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v helm &> /dev/null; then
        log_error "helm is not installed. Please install Helm 3.8+"
        exit 1
    fi

    local helm_version=$(helm version --short | grep -oP 'v\d+\.\d+' | sed 's/v//')
    local major_version=$(echo "$helm_version" | cut -d. -f1)
    local minor_version=$(echo "$helm_version" | cut -d. -f2)

    if [ "$major_version" -lt 3 ] || ([ "$major_version" -eq 3 ] && [ "$minor_version" -lt 8 ]); then
        log_error "Helm 3.8+ required for OCI support. Current: v${helm_version}"
        exit 1
    fi

    log_info "Using Helm v${helm_version}"
}

# Validate chart
validate_chart() {
    log_info "Validating chart at ${CHART_PATH}..."

    if [ ! -f "${CHART_PATH}/Chart.yaml" ]; then
        log_error "Chart.yaml not found at ${CHART_PATH}"
        exit 1
    fi

    if [ "${SKIP_LINT}" != "true" ]; then
        log_info "Running helm lint..."
        if ! helm lint "${CHART_PATH}"; then
            log_error "Helm lint failed"
            exit 1
        fi
        log_success "Helm lint passed"
    fi

    if [ "${SKIP_TEST}" != "true" ]; then
        if [ -x "deployments/scripts/test-helm-chart.sh" ]; then
            log_info "Running chart tests..."
            if ! ./deployments/scripts/test-helm-chart.sh --skip-lint; then
                log_error "Chart tests failed"
                exit 1
            fi
            log_success "Chart tests passed"
        fi
    fi
}

# Update chart version
update_chart_version() {
    if [ -n "${OVERRIDE_VERSION}" ]; then
        log_info "Updating chart version to ${OVERRIDE_VERSION}..."

        # Update Chart.yaml
        sed -i "s/^version:.*/version: ${OVERRIDE_VERSION}/" "${CHART_PATH}/Chart.yaml"
        sed -i "s/^appVersion:.*/appVersion: ${OVERRIDE_VERSION}/" "${CHART_PATH}/Chart.yaml"

        log_success "Chart version updated to ${OVERRIDE_VERSION}"
    fi
}

# Get chart version
get_chart_version() {
    grep '^version:' "${CHART_PATH}/Chart.yaml" | awk '{print $2}'
}

# Get chart name
get_chart_name() {
    grep '^name:' "${CHART_PATH}/Chart.yaml" | awk '{print $2}'
}

# Login to OCI registry
login_registry() {
    if [ "${SKIP_LOGIN}" = "true" ]; then
        log_info "Skipping registry login (--skip-login specified)"
        return 0
    fi

    local username="${REGISTRY_USERNAME}"
    local password="${REGISTRY_PASSWORD}"

    if [ -z "${username}" ] || [ -z "${password}" ]; then
        log_error "Registry credentials not provided"
        log_info "Set REGISTRY_USERNAME and REGISTRY_PASSWORD environment variables"
        log_info "Or use --username and --password options"
        exit 1
    fi

    log_info "Logging in to ${OCI_REGISTRY}..."

    if echo "${password}" | helm registry login "${OCI_REGISTRY}" --username "${username}" --password-stdin; then
        log_success "Logged in to ${OCI_REGISTRY}"
    else
        log_error "Failed to login to ${OCI_REGISTRY}"
        exit 1
    fi
}

# Package chart
package_chart() {
    local chart_name=$(get_chart_name)
    local chart_version=$(get_chart_version)

    log_info "Packaging chart ${chart_name} version ${chart_version}..."

    # Create output directory
    mkdir -p "${OUTPUT_DIR}"

    # Package chart
    if helm package "${CHART_PATH}" --destination "${OUTPUT_DIR}"; then
        local package_file="${OUTPUT_DIR}/${chart_name}-${chart_version}.tgz"
        log_success "Chart packaged: ${package_file}"

        # Show package info
        log_info "Package information:"
        ls -lh "${package_file}"

        echo "${package_file}"
    else
        log_error "Failed to package chart"
        exit 1
    fi
}

# Push chart to OCI registry
push_chart() {
    local chart_name=$(get_chart_name)
    local chart_version=$(get_chart_version)
    local package_file="${OUTPUT_DIR}/${chart_name}-${chart_version}.tgz"
    local oci_url="oci://${OCI_REGISTRY}/${OCI_NAMESPACE}"

    log_info "Pushing chart to ${oci_url}..."

    if helm push "${package_file}" "${oci_url}"; then
        log_success "Chart pushed to ${oci_url}/${chart_name}:${chart_version}"

        # Tag as latest if requested
        if [ "${TAG_LATEST}" = "true" ]; then
            log_info "Tagging as latest..."
            # Note: Helm OCI doesn't support re-tagging yet
            # This would require using crane or docker to re-tag
            log_warning "Latest tagging not yet supported by Helm OCI"
            log_info "Chart is available at: ${oci_url}/${chart_name}:${chart_version}"
        fi
    else
        log_error "Failed to push chart"
        exit 1
    fi

    # Show installation instructions
    echo ""
    log_success "Chart published successfully!"
    echo ""
    log_info "Install with:"
    echo "  helm install my-${chart_name} oci://${OCI_REGISTRY}/${OCI_NAMESPACE}/${chart_name} --version ${chart_version}"
    echo ""
}

# Main execution
main() {
    local skip_login=false
    local skip_lint=false
    local skip_test=false
    local tag_latest=false

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                usage
                exit 0
                ;;
            -c|--chart)
                CHART_PATH="$2"
                shift 2
                ;;
            -r|--registry)
                OCI_REGISTRY="$2"
                shift 2
                ;;
            -n|--namespace)
                OCI_NAMESPACE="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -v|--version)
                OVERRIDE_VERSION="$2"
                shift 2
                ;;
            --username)
                REGISTRY_USERNAME="$2"
                shift 2
                ;;
            --password)
                REGISTRY_PASSWORD="$2"
                shift 2
                ;;
            --skip-login)
                SKIP_LOGIN=true
                shift
                ;;
            --skip-lint)
                SKIP_LINT=true
                shift
                ;;
            --skip-test)
                SKIP_TEST=true
                shift
                ;;
            --tag-latest)
                TAG_LATEST=true
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    # Export variables for sub-functions
    export SKIP_LOGIN SKIP_LINT SKIP_TEST TAG_LATEST

    echo ""
    echo "========================================"
    echo "  HyperSDK OCI Registry Publishing"
    echo "========================================"
    echo ""
    echo "Registry:  ${OCI_REGISTRY}"
    echo "Namespace: ${OCI_NAMESPACE}"
    echo "Chart:     ${CHART_PATH}"
    echo ""

    check_prerequisites
    validate_chart
    update_chart_version
    login_registry

    local package_file=$(package_chart)
    push_chart

    echo ""
    echo "========================================"
    echo "  Publishing Complete"
    echo "========================================"
    echo ""
}

main "$@"
