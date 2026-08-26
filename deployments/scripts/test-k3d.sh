#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# =============================================================================
# HyperSDK k3d Test Script
# =============================================================================
# Validates HyperSDK deployment on k3d cluster
#
# Usage:
#   ./test-k3d.sh [OPTIONS]
#
# Options:
#   -n, --name NAME          Cluster name (default: transiva)
#   -o, --output FILE        Save test results to file
#   -v, --verbose            Verbose output
#   -h, --help               Show this help message
#
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default values
CLUSTER_NAME="transiva"
OUTPUT_FILE=""
VERBOSE=false

# Test results
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# =============================================================================
# Helper Functions
# =============================================================================

log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

log_test() {
    ((TESTS_TOTAL++))
    if [ "$2" = "PASS" ]; then
        ((TESTS_PASSED++))
        log_success "Test $TESTS_TOTAL: $1"
    else
        ((TESTS_FAILED++))
        log_error "Test $TESTS_TOTAL: $1"
        if [ -n "$3" ]; then
            echo "  Error: $3"
        fi
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
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
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
# Test Functions
# =============================================================================

test_cluster_exists() {
    if k3d cluster list | grep -q "^$CLUSTER_NAME "; then
        log_test "Cluster '$CLUSTER_NAME' exists" "PASS"
        return 0
    else
        log_test "Cluster '$CLUSTER_NAME' exists" "FAIL" "Cluster not found"
        return 1
    fi
}

test_cluster_running() {
    export KUBECONFIG="$(k3d kubeconfig write "$CLUSTER_NAME" 2>/dev/null)"
    if kubectl cluster-info &>/dev/null; then
        log_test "Cluster is accessible" "PASS"
        return 0
    else
        log_test "Cluster is accessible" "FAIL" "Cannot connect to cluster"
        return 1
    fi
}

test_namespace_exists() {
    if kubectl get namespace transiva &>/dev/null; then
        log_test "Namespace 'transiva' exists" "PASS"
        return 0
    else
        log_test "Namespace 'transiva' exists" "FAIL" "Namespace not found"
        return 1
    fi
}

test_deployment_exists() {
    if kubectl get deployment hypervisord -n transiva &>/dev/null; then
        log_test "Deployment 'hypervisord' exists" "PASS"
        return 0
    else
        log_test "Deployment 'hypervisord' exists" "FAIL" "Deployment not found"
        return 1
    fi
}

test_pod_running() {
    POD_STATUS=$(kubectl get pods -n transiva -l app=hypervisord \
        -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")

    if [ "$POD_STATUS" = "Running" ]; then
        log_test "Pod is running" "PASS"
        return 0
    else
        log_test "Pod is running" "FAIL" "Pod status: $POD_STATUS"
        return 1
    fi
}

test_pod_ready() {
    POD_READY=$(kubectl get pods -n transiva -l app=hypervisord \
        -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")

    if [ "$POD_READY" = "True" ]; then
        log_test "Pod is ready" "PASS"
        return 0
    else
        log_test "Pod is ready" "FAIL" "Ready status: $POD_READY"
        return 1
    fi
}

test_services_exist() {
    if kubectl get svc hypervisord -n transiva &>/dev/null && \
       kubectl get svc hypervisord-external -n transiva &>/dev/null; then
        log_test "Services exist" "PASS"
        return 0
    else
        log_test "Services exist" "FAIL" "One or more services not found"
        return 1
    fi
}

test_pvcs_bound() {
    DATA_PVC_STATUS=$(kubectl get pvc hypervisord-data-pvc -n transiva \
        -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
    EXPORTS_PVC_STATUS=$(kubectl get pvc hypervisord-exports-pvc -n transiva \
        -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")

    if [ "$DATA_PVC_STATUS" = "Bound" ] && [ "$EXPORTS_PVC_STATUS" = "Bound" ]; then
        log_test "PVCs are bound" "PASS"
        return 0
    else
        log_test "PVCs are bound" "FAIL" "Data: $DATA_PVC_STATUS, Exports: $EXPORTS_PVC_STATUS"
        return 1
    fi
}

test_health_endpoint() {
    LB_IP=$(kubectl get svc hypervisord-external -n transiva \
        -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

    if [ -z "$LB_IP" ]; then
        log_test "Health endpoint responds" "FAIL" "LoadBalancer IP not assigned"
        return 1
    fi

    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "http://$LB_IP/health" || echo "000")

    if [ "$RESPONSE" = "200" ]; then
        log_test "Health endpoint responds" "PASS"
        return 0
    else
        log_test "Health endpoint responds" "FAIL" "HTTP status: $RESPONSE"
        return 1
    fi
}

test_status_endpoint() {
    LB_IP=$(kubectl get svc hypervisord-external -n transiva \
        -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

    if [ -z "$LB_IP" ]; then
        log_test "Status endpoint responds" "FAIL" "LoadBalancer IP not assigned"
        return 1
    fi

    RESPONSE=$(curl -s "http://$LB_IP/status" | jq -e '.version' &>/dev/null && echo "OK" || echo "FAIL")

    if [ "$RESPONSE" = "OK" ]; then
        log_test "Status endpoint responds with valid JSON" "PASS"
        return 0
    else
        log_test "Status endpoint responds with valid JSON" "FAIL"
        return 1
    fi
}

test_capabilities_endpoint() {
    LB_IP=$(kubectl get svc hypervisord-external -n transiva \
        -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

    if [ -z "$LB_IP" ]; then
        log_test "Capabilities endpoint responds" "FAIL" "LoadBalancer IP not assigned"
        return 1
    fi

    RESPONSE=$(curl -s "http://$LB_IP/capabilities" | jq -e '.capabilities.web.available' 2>/dev/null || echo "false")

    if [ "$RESPONSE" = "true" ]; then
        log_test "Capabilities endpoint shows web available" "PASS"
        return 0
    else
        log_test "Capabilities endpoint shows web available" "FAIL"
        return 1
    fi
}

test_configmap_exists() {
    if kubectl get configmap hypervisord-config -n transiva &>/dev/null; then
        log_test "ConfigMap exists" "PASS"
        return 0
    else
        log_test "ConfigMap exists" "FAIL"
        return 1
    fi
}

test_secrets_exist() {
    if kubectl get secret vsphere-credentials -n transiva &>/dev/null; then
        log_test "Secrets exist" "PASS"
        return 0
    else
        log_test "Secrets exist" "FAIL"
        return 1
    fi
}

test_rbac_configured() {
    if kubectl get serviceaccount hypervisord -n transiva &>/dev/null && \
       kubectl get role hypervisord-role -n transiva &>/dev/null && \
       kubectl get rolebinding hypervisord-rolebinding -n transiva &>/dev/null; then
        log_test "RBAC configured" "PASS"
        return 0
    else
        log_test "RBAC configured" "FAIL"
        return 1
    fi
}

# =============================================================================
# Main Test Execution
# =============================================================================

echo ""
log_info "HyperSDK k3d Test Suite"
echo ""
echo "Cluster: $CLUSTER_NAME"
echo "Date: $(date)"
echo ""

# Run tests
log_info "Running tests..."
echo ""

test_cluster_exists || exit 1
test_cluster_running || exit 1
test_namespace_exists
test_deployment_exists
test_pod_running
test_pod_ready
test_services_exist
test_pvcs_bound
test_configmap_exists
test_secrets_exist
test_rbac_configured
test_health_endpoint
test_status_endpoint
test_capabilities_endpoint

# Show results
echo ""
echo "========================================="
echo "Test Results Summary"
echo "========================================="
echo "Total Tests:  $TESTS_TOTAL"
echo "Passed:       $TESTS_PASSED"
echo "Failed:       $TESTS_FAILED"
echo "Success Rate: $(awk "BEGIN {printf \"%.1f\", ($TESTS_PASSED/$TESTS_TOTAL)*100}")%"
echo "========================================="
echo ""

# Show cluster info
if [ "$VERBOSE" = true ]; then
    echo ""
    log_info "Cluster Information:"
    echo ""
    kubectl get nodes
    echo ""
    kubectl get all -n transiva
    echo ""
    kubectl get pvc -n transiva
    echo ""
fi

# Show LoadBalancer info
LB_IP=$(kubectl get svc hypervisord-external -n transiva \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

if [ -n "$LB_IP" ]; then
    echo ""
    log_info "Access Information:"
    echo ""
    echo "  Health:      http://$LB_IP/health"
    echo "  Status:      http://$LB_IP/status"
    echo "  Dashboard:   http://$LB_IP/web/dashboard/"
    echo "  Capabilities: http://$LB_IP/capabilities"
    echo ""
fi

# Save results to file if requested
if [ -n "$OUTPUT_FILE" ]; then
    cat > "$OUTPUT_FILE" <<EOF
# HyperSDK k3d Test Results

**Date**: $(date)
**Cluster**: $CLUSTER_NAME

## Summary

- Total Tests: $TESTS_TOTAL
- Passed: $TESTS_PASSED
- Failed: $TESTS_FAILED
- Success Rate: $(awk "BEGIN {printf \"%.1f\", ($TESTS_PASSED/$TESTS_TOTAL)*100}")%

## Test Details

See console output for detailed test results.

## Cluster Information

\`\`\`
$(kubectl get nodes 2>/dev/null)
\`\`\`

## Deployment Status

\`\`\`
$(kubectl get all -n transiva 2>/dev/null)
\`\`\`

## Storage

\`\`\`
$(kubectl get pvc -n transiva 2>/dev/null)
\`\`\`
EOF
    log_success "Results saved to: $OUTPUT_FILE"
fi

# Exit code
if [ $TESTS_FAILED -eq 0 ]; then
    log_success "All tests passed! ✨"
    exit 0
else
    log_error "$TESTS_FAILED test(s) failed"
    exit 1
fi
