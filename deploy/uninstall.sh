#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "============================================="
echo "HyperSDK Kubernetes Operator Uninstallation"
echo "============================================="
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

echo "⚠️  WARNING: This will delete all HyperSDK resources including:"
echo "  - BackupJobs, BackupSchedules, RestoreJobs"
echo "  - The HyperSDK operator"
echo "  - CRDs (this will delete ALL custom resources)"
echo ""
read -p "Are you sure you want to continue? (yes/no): " -r
echo ""

if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Uninstallation cancelled"
    exit 0
fi

# Step 1: Delete operator deployment
echo "Step 1/3: Deleting operator deployment..."
kubectl delete -f "${SCRIPT_DIR}/operator/deployment.yaml" --ignore-not-found=true

echo "✓ Operator deleted"
echo ""

# Step 2: Delete RBAC
echo "Step 2/3: Deleting RBAC resources..."
kubectl delete -f "${SCRIPT_DIR}/operator/rbac.yaml" --ignore-not-found=true

echo "✓ RBAC deleted"
echo ""

# Step 3: Delete CRDs (this deletes all custom resources)
echo "Step 3/3: Deleting CRDs..."
echo "⚠️  This will delete ALL BackupJobs, BackupSchedules, and RestoreJobs"
read -p "Continue? (yes/no): " -r
echo ""

if [[ $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    kubectl delete -f "${SCRIPT_DIR}/crds/" --ignore-not-found=true
    echo "✓ CRDs deleted"
else
    echo "⚠️  CRDs not deleted. Custom resources remain in the cluster."
fi

echo ""
echo "============================================="
echo "Uninstallation Complete!"
echo "============================================="
echo ""
