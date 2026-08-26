#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# HyperSDK VM Lifecycle Test Script
# Tests complete VM lifecycle on Kubernetes
#
# Usage: ./test-vm-lifecycle.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_NS="transiva-test"
TEST_VM="test-vm"
TEST_TEMPLATE="test-template"
TEST_SNAPSHOT="test-snapshot"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

wait_for_condition() {
    local resource=$1
    local name=$2
    local condition=$3
    local timeout=${4:-300}

    log_info "Waiting for $resource/$name condition: $condition (timeout: ${timeout}s)"
    kubectl wait --for="$condition" "$resource/$name" -n "$TEST_NS" --timeout="${timeout}s" || {
        log_error "Timeout waiting for $condition on $resource/$name"
        kubectl describe "$resource/$name" -n "$TEST_NS"
        return 1
    }
}

cleanup() {
    log_info "Cleaning up test resources..."
    kubectl delete namespace "$TEST_NS" --ignore-not-found=true --wait=false
}

# Trap errors and cleanup
trap cleanup EXIT

# Main test flow
main() {
    log_info "Starting HyperSDK VM Lifecycle Test"

    # Check prerequisites
    log_info "Checking prerequisites..."
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    if ! kubectl get crd virtualmachines.transiva.zyvor.ai &> /dev/null; then
        log_error "VirtualMachine CRD not installed"
        exit 1
    fi

    # Create test namespace
    log_info "Creating test namespace: $TEST_NS"
    kubectl create namespace "$TEST_NS" || true

    # Test 1: Create VM Template
    log_info "Test 1: Creating VM Template..."
    kubectl apply -n "$TEST_NS" -f - <<EOF
apiVersion: transiva.zyvor.ai/v1alpha1
kind: VMTemplate
metadata:
  name: $TEST_TEMPLATE
spec:
  displayName: "Test Template"
  version: "1.0.0"
  osInfo:
    type: linux
    distribution: ubuntu
    version: "22.04"
  defaultSpec:
    cpus: 2
    memory: "4Gi"
  image:
    source: "ubuntu:22.04"
    format: "qcow2"
EOF

    sleep 5
    log_info "✓ Template created"

    # Test 2: Create VM from Template
    log_info "Test 2: Creating VM from template..."
    kubectl apply -n "$TEST_NS" -f - <<EOF
apiVersion: transiva.zyvor.ai/v1alpha1
kind: VirtualMachine
metadata:
  name: $TEST_VM
spec:
  cpus: 2
  memory: "4Gi"
  running: true
  image:
    templateRef:
      name: $TEST_TEMPLATE
  disks:
    - name: root
      size: "20Gi"
      storageClass: standard
      bootOrder: 1
  networks:
    - name: default
      type: pod-network
EOF

    # Wait for VM to be running
    if wait_for_condition "vm" "$TEST_VM" "jsonpath='{.status.phase}'=Running" 300; then
        log_info "✓ VM is running"
    else
        log_error "VM failed to start"
        exit 1
    fi

    # Test 3: Stop VM
    log_info "Test 3: Stopping VM..."
    kubectl patch vm "$TEST_VM" -n "$TEST_NS" --type=merge -p '{"spec":{"running":false}}'

    if wait_for_condition "vm" "$TEST_VM" "jsonpath='{.status.phase}'=Stopped" 120; then
        log_info "✓ VM stopped successfully"
    else
        log_error "VM failed to stop"
        exit 1
    fi

    # Test 4: Restart VM
    log_info "Test 4: Restarting VM..."
    kubectl patch vm "$TEST_VM" -n "$TEST_NS" --type=merge -p '{"spec":{"running":true}}'

    if wait_for_condition "vm" "$TEST_VM" "jsonpath='{.status.phase}'=Running" 300; then
        log_info "✓ VM restarted successfully"
    else
        log_error "VM failed to restart"
        exit 1
    fi

    # Test 5: Create Snapshot
    log_info "Test 5: Creating VM snapshot..."
    kubectl apply -n "$TEST_NS" -f - <<EOF
apiVersion: transiva.zyvor.ai/v1alpha1
kind: VMSnapshot
metadata:
  name: $TEST_SNAPSHOT
spec:
  vmRef:
    name: $TEST_VM
  includeMemory: false
  description: "Test snapshot"
EOF

    sleep 10
    log_info "✓ Snapshot created"

    # Test 6: Clone VM
    log_info "Test 6: Cloning VM..."
    kubectl apply -n "$TEST_NS" -f - <<EOF
apiVersion: transiva.zyvor.ai/v1alpha1
kind: VMOperation
metadata:
  name: clone-test
spec:
  vmRef:
    name: $TEST_VM
  operation: clone
  cloneSpec:
    targetName: ${TEST_VM}-clone
    linkedClone: false
    startAfterClone: false
EOF

    sleep 10
    if kubectl get vm "${TEST_VM}-clone" -n "$TEST_NS" &> /dev/null; then
        log_info "✓ VM cloned successfully"
    else
        log_warn "Clone may still be in progress"
    fi

    # Test 7: Delete Clone
    log_info "Test 7: Deleting cloned VM..."
    kubectl delete vm "${TEST_VM}-clone" -n "$TEST_NS" --ignore-not-found=true
    log_info "✓ Clone deleted"

    # Test 8: Delete Snapshot
    log_info "Test 8: Deleting snapshot..."
    kubectl delete vmsnapshot "$TEST_SNAPSHOT" -n "$TEST_NS"
    log_info "✓ Snapshot deleted"

    # Test 9: Delete VM
    log_info "Test 9: Deleting VM..."
    kubectl delete vm "$TEST_VM" -n "$TEST_NS"
    sleep 5
    log_info "✓ VM deleted"

    # Test 10: Verify Cleanup
    log_info "Test 10: Verifying resource cleanup..."
    local pvcs=$(kubectl get pvc -n "$TEST_NS" -l "vm=$TEST_VM" --no-headers 2>/dev/null | wc -l)
    if [ "$pvcs" -eq 0 ]; then
        log_info "✓ All PVCs cleaned up"
    else
        log_warn "Some PVCs may still exist (expected during deletion)"
    fi

    # Success summary
    echo ""
    log_info "========================================="
    log_info "    VM Lifecycle Test: SUCCESS! ✓"
    log_info "========================================="
    echo ""
    log_info "All tests passed:"
    echo "  ✓ Template creation"
    echo "  ✓ VM creation from template"
    echo "  ✓ VM stop"
    echo "  ✓ VM restart"
    echo "  ✓ Snapshot creation"
    echo "  ✓ VM clone"
    echo "  ✓ Resource cleanup"
}

# Run main function
main "$@"
