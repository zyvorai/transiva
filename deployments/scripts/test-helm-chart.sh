#!/bin/bash
# SPDX-License-Identifier: Apache-2.0


# HyperSDK Helm Chart Testing Script
# Tests the Helm chart for syntax, rendering, and deployment validation

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CHART_PATH="${CHART_PATH:-deployments/helm/transiva}"
NAMESPACE="${NAMESPACE:-transiva-helm-test}"
RELEASE_NAME="${RELEASE_NAME:-transiva-test}"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Usage
usage() {
    cat <<EOF
Usage: $0 [options]

Test the HyperSDK Helm chart for correctness and deployability.

Options:
    -h, --help              Show this help message
    -c, --chart PATH        Path to Helm chart (default: ${CHART_PATH})
    -n, --namespace NAME    Kubernetes namespace (default: ${NAMESPACE})
    -r, --release NAME      Helm release name (default: ${RELEASE_NAME})
    -d, --deploy            Actually deploy to cluster (requires active K8s context)
    -v, --verbose           Enable verbose output
    --skip-lint             Skip Helm lint test
    --skip-template         Skip template rendering tests
    --skip-values           Skip values file tests

Examples:
    $0                      # Run all tests
    $0 --deploy             # Run tests and deploy to cluster
    $0 -v                   # Verbose output
EOF
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

# Test execution wrapper
run_test() {
    local test_name="$1"
    local test_command="$2"

    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Test ${TESTS_RUN}: ${test_name}"

    if eval "$test_command"; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        log_success "${test_name}"
        return 0
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        log_error "${test_name}"
        return 1
    fi
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v helm &> /dev/null; then
        log_error "helm is not installed. Please install Helm 3.x"
        exit 1
    fi

    local helm_version=$(helm version --short)
    log_info "Using ${helm_version}"

    if ! command -v kubectl &> /dev/null; then
        log_warning "kubectl is not installed. Deployment tests will be skipped."
        CAN_DEPLOY=false
    else
        CAN_DEPLOY=true
    fi
}

# Test 1: Chart directory structure
test_chart_structure() {
    [ -f "${CHART_PATH}/Chart.yaml" ] && \
    [ -f "${CHART_PATH}/values.yaml" ] && \
    [ -d "${CHART_PATH}/templates" ]
}

# Test 2: Chart.yaml validation
test_chart_yaml() {
    helm show chart "${CHART_PATH}" > /dev/null 2>&1
}

# Test 3: values.yaml validation
test_values_yaml() {
    helm show values "${CHART_PATH}" > /dev/null 2>&1
}

# Test 4: Helm lint
test_helm_lint() {
    helm lint "${CHART_PATH}" > /dev/null 2>&1
}

# Test 5: Template rendering with default values
test_template_default() {
    helm template test-release "${CHART_PATH}" > /dev/null 2>&1
}

# Test 6: Template rendering with minikube values
test_template_minikube() {
    helm template test-release "${CHART_PATH}" \
        --values "${CHART_PATH}/examples/minikube-values.yaml" > /dev/null 2>&1
}

# Test 7: Template rendering with GKE values
test_template_gke() {
    helm template test-release "${CHART_PATH}" \
        --values "${CHART_PATH}/examples/gke-values.yaml" > /dev/null 2>&1
}

# Test 8: Template rendering with EKS values
test_template_eks() {
    helm template test-release "${CHART_PATH}" \
        --values "${CHART_PATH}/examples/eks-values.yaml" > /dev/null 2>&1
}

# Test 9: Template rendering with AKS values
test_template_aks() {
    helm template test-release "${CHART_PATH}" \
        --values "${CHART_PATH}/examples/aks-values.yaml" > /dev/null 2>&1
}

# Test 10: Validate generated manifests are valid YAML
test_yaml_validity() {
    # Check if kubectl can connect to a cluster
    if ! kubectl cluster-info &> /dev/null; then
        log_warning "No active Kubernetes cluster, skipping kubectl validation"
        # Just validate that template renders valid YAML
        helm template test-release "${CHART_PATH}" > /dev/null 2>&1
        return $?
    fi
    helm template test-release "${CHART_PATH}" | kubectl apply --dry-run=client -f - > /dev/null 2>&1
}

# Test 11: Check required templates exist
test_required_templates() {
    local templates_dir="${CHART_PATH}/templates"
    [ -f "${templates_dir}/deployment.yaml" ] && \
    [ -f "${templates_dir}/service.yaml" ] && \
    [ -f "${templates_dir}/configmap.yaml" ] && \
    [ -f "${templates_dir}/serviceaccount.yaml" ]
}

# Test 12: Validate all example values files
test_all_examples() {
    local examples_dir="${CHART_PATH}/examples"
    if [ -d "${examples_dir}" ]; then
        for values_file in "${examples_dir}"/*.yaml; do
            if [ -f "${values_file}" ]; then
                helm template test-release "${CHART_PATH}" \
                    --values "${values_file}" > /dev/null 2>&1 || return 1
            fi
        done
    fi
    return 0
}

# Test 13: Check chart version format
test_version_format() {
    local version=$(helm show chart "${CHART_PATH}" | grep '^version:' | awk '{print $2}')
    [[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

# Test 14: Validate NOTES.txt exists and has content
test_notes_template() {
    # Check if NOTES.txt exists and has content
    [ -f "${CHART_PATH}/templates/NOTES.txt" ] && \
    [ -s "${CHART_PATH}/templates/NOTES.txt" ]
}

# Deployment test (optional)
test_deploy_to_cluster() {
    if [ "${DO_DEPLOY}" != "true" ]; then
        log_info "Skipping deployment test (use --deploy to enable)"
        return 0
    fi

    if [ "${CAN_DEPLOY}" != "true" ]; then
        log_warning "kubectl not available, skipping deployment test"
        return 0
    fi

    # Check cluster connection
    if ! kubectl cluster-info &> /dev/null; then
        log_warning "No active Kubernetes cluster, skipping deployment test"
        return 0
    fi

    log_info "Deploying chart to cluster..."

    # Create namespace
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - > /dev/null

    # Install chart
    helm install "${RELEASE_NAME}" "${CHART_PATH}" \
        --namespace "${NAMESPACE}" \
        --values "${CHART_PATH}/examples/minikube-values.yaml" \
        --wait --timeout 5m

    # Check deployment
    kubectl wait --for=condition=available --timeout=300s \
        deployment/transiva -n "${NAMESPACE}"

    # Cleanup
    helm uninstall "${RELEASE_NAME}" --namespace "${NAMESPACE}"
    kubectl delete namespace "${NAMESPACE}"
}

# Main execution
main() {
    local do_deploy=false
    local skip_lint=false
    local skip_template=false
    local skip_values=false
    local verbose=false

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
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -r|--release)
                RELEASE_NAME="$2"
                shift 2
                ;;
            -d|--deploy)
                DO_DEPLOY=true
                shift
                ;;
            -v|--verbose)
                verbose=true
                set -x
                shift
                ;;
            --skip-lint)
                skip_lint=true
                shift
                ;;
            --skip-template)
                skip_template=true
                shift
                ;;
            --skip-values)
                skip_values=true
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    echo ""
    echo "========================================"
    echo "  HyperSDK Helm Chart Test Suite"
    echo "========================================"
    echo ""

    check_prerequisites

    echo ""
    log_info "Testing chart at: ${CHART_PATH}"
    echo ""

    # Run tests
    run_test "Chart directory structure" test_chart_structure
    run_test "Chart.yaml validation" test_chart_yaml
    run_test "values.yaml validation" test_values_yaml

    if [ "${skip_lint}" != "true" ]; then
        run_test "Helm lint" test_helm_lint
    fi

    if [ "${skip_template}" != "true" ]; then
        run_test "Template rendering (default)" test_template_default
        run_test "Template rendering (minikube)" test_template_minikube
        run_test "Template rendering (GKE)" test_template_gke
        run_test "Template rendering (EKS)" test_template_eks
        run_test "Template rendering (AKS)" test_template_aks
    fi

    run_test "Required templates exist" test_required_templates

    if [ "${skip_values}" != "true" ]; then
        run_test "All example values files" test_all_examples
    fi

    run_test "Chart version format" test_version_format
    run_test "YAML validity" test_yaml_validity
    run_test "NOTES.txt template" test_notes_template

    # Optional deployment test
    if [ "${DO_DEPLOY}" = "true" ]; then
        run_test "Deploy to cluster" test_deploy_to_cluster
    fi

    # Print summary
    echo ""
    echo "========================================"
    echo "  Test Summary"
    echo "========================================"
    echo -e "Total tests:  ${TESTS_RUN}"
    echo -e "${GREEN}Passed:       ${TESTS_PASSED}${NC}"
    if [ ${TESTS_FAILED} -gt 0 ]; then
        echo -e "${RED}Failed:       ${TESTS_FAILED}${NC}"
    else
        echo -e "Failed:       ${TESTS_FAILED}"
    fi
    echo ""

    # Calculate success rate
    local success_rate=$((TESTS_PASSED * 100 / TESTS_RUN))
    if [ ${success_rate} -eq 100 ]; then
        echo -e "${GREEN}✓ All tests passed! (100%)${NC}"
        echo ""
        exit 0
    elif [ ${success_rate} -ge 80 ]; then
        echo -e "${YELLOW}⚠ Some tests failed (${success_rate}%)${NC}"
        echo ""
        exit 1
    else
        echo -e "${RED}✗ Many tests failed (${success_rate}%)${NC}"
        echo ""
        exit 1
    fi
}

main "$@"
