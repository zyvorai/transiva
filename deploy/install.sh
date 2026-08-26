#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==========================================="
echo "HyperSDK Kubernetes Operator Installation"
echo "==========================================="
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check cluster connectivity
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot connect to Kubernetes cluster"
    echo "Please configure kubectl to connect to your cluster"
    exit 1
fi

echo "✓ kubectl is configured and cluster is reachable"
echo ""

# Step 1: Install CRDs
echo "Step 1/3: Installing Custom Resource Definitions..."
kubectl apply -f "${SCRIPT_DIR}/crds/"

# Wait for CRDs to be established
echo "Waiting for CRDs to be established..."
kubectl wait --for=condition=established --timeout=60s \
    crd/backupjobs.transiva.zyvor.ai \
    crd/backupschedules.transiva.zyvor.ai \
    crd/restorejobs.transiva.zyvor.ai

echo "✓ CRDs installed successfully"
echo ""

# Step 2: Create namespace and RBAC
echo "Step 2/3: Creating namespace and RBAC..."
kubectl apply -f "${SCRIPT_DIR}/operator/rbac.yaml"

echo "✓ RBAC configured successfully"
echo ""

# Step 3: Deploy operator
echo "Step 3/3: Deploying HyperSDK operator..."
kubectl apply -f "${SCRIPT_DIR}/operator/deployment.yaml"

# Wait for operator to be ready
echo "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=300s \
    deployment/transiva-operator -n transiva-system

echo "✓ Operator deployed successfully"
echo ""

echo "==========================================="
echo "Installation Complete!"
echo "==========================================="
echo ""
echo "Verify installation:"
echo "  kubectl get deployment -n transiva-system"
echo "  kubectl get pods -n transiva-system"
echo ""
echo "Check operator logs:"
echo "  kubectl logs -f -n transiva-system -l app=transiva-operator"
echo ""
echo "Create a sample BackupJob:"
echo "  kubectl apply -f examples/backupjob-sample.yaml"
echo ""
