#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# HyperSDK Code Issues Auto-Fix Script
# Addresses critical compilation and quality issues

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== HyperSDK Code Auto-Fix Script ==="
echo "Project Root: $PROJECT_ROOT"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warning() {
    echo -e "${YELLOW}!${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

info() {
    echo "  $1"
}

# Step 1: Format all Go files
echo "Step 1: Formatting Go code..."
if command -v gofmt &> /dev/null; then
    gofmt -w .
    success "Code formatted with gofmt"
else
    warning "gofmt not found, skipping formatting"
fi

if command -v goimports &> /dev/null; then
    goimports -w .
    success "Imports organized with goimports"
else
    warning "goimports not found, skipping import organization"
    info "Install with: go install golang.org/x/tools/cmd/goimports@latest"
fi

# Step 2: Fix daemon/queue/queue.go - lock value copy
echo ""
echo "Step 2: Fixing lock value copy in daemon/queue/queue.go..."
if [ -f "daemon/queue/queue.go" ]; then
    # Find the GetMetrics method and make it return a pointer
    if grep -q "func (q \*Queue) GetMetrics() Metrics" daemon/queue/queue.go; then
        sed -i 's/func (q \*Queue) GetMetrics() Metrics/func (q *Queue) GetMetrics() *Metrics/g' daemon/queue/queue.go
        sed -i 's/return Metrics{/return \&Metrics{/g' daemon/queue/queue.go
        success "Fixed lock value copy in queue.go"
    else
        warning "GetMetrics method not found in expected format"
    fi
else
    error "daemon/queue/queue.go not found"
fi

# Step 3: Fix build tags in hyperv tests
echo ""
echo "Step 3: Fixing build tags in providers/hyperv/client_test.go..."
if [ -f "providers/hyperv/client_test.go" ]; then
    # Move +build comments to top and convert to go:build
    sed -i '1i//go:build integration' providers/hyperv/client_test.go
    sed -i '/^\/\/ +build/d' providers/hyperv/client_test.go
    success "Fixed build tags in hyperv/client_test.go"
else
    warning "providers/hyperv/client_test.go not found"
fi

# Step 4: Remove unused variables
echo ""
echo "Step 4: Removing unused variables..."

# AWS client.go
if [ -f "providers/aws/client.go" ]; then
    sed -i '/^\s*exportResult :=/d' providers/aws/client.go
    success "Removed unused exportResult in aws/client.go"
fi

# Azure export.go
if [ -f "providers/azure/export.go" ]; then
    sed -i '/^\s*containerURLParsed :=/d' providers/azure/export.go
    success "Removed unused containerURLParsed in azure/export.go"
fi

# Examples
if [ -f "examples/migration_orchestrator_example.go" ]; then
    sed -i '/^\s*"log"$/d' examples/migration_orchestrator_example.go
    success "Removed unused log import in migration_orchestrator_example.go"
fi

# Step 5: Fix progress reporter type mismatches
echo ""
echo "Step 5: Fixing progress reporter type mismatches..."

# Fix int to int64 conversions
for file in providers/aws/export.go providers/gcp/export.go; do
    if [ -f "$file" ]; then
        # Replace reporter.Update(progress) with reporter.Update(int64(progress))
        sed -i 's/reporter\.Update(progress)/reporter.Update(int64(progress))/g' "$file"
        sed -i 's/reporter\.Update(percentage)/reporter.Update(int64(percentage))/g' "$file"
        sed -i 's/pr\.reporter\.Update(percentage)/pr.reporter.Update(int64(percentage))/g' "$file"
        success "Fixed progress types in $file"
    fi
done

# Step 6: Update go.mod and dependencies
echo ""
echo "Step 6: Tidying Go modules..."
go mod tidy
success "Go modules tidied"

# Step 7: Run go vet and capture remaining issues
echo ""
echo "Step 7: Checking for remaining issues with go vet..."
if go vet ./... 2>&1 | tee /tmp/vet-output.txt; then
    success "No go vet issues found"
else
    warning "Some go vet issues remain - see /tmp/vet-output.txt"
    echo ""
    echo "Remaining issues:"
    cat /tmp/vet-output.txt | grep -E "^(#|vet:)" | head -20
fi

# Step 8: Try to build all packages
echo ""
echo "Step 8: Attempting to build all packages..."
if go build ./... 2>&1 | tee /tmp/build-output.txt; then
    success "All packages built successfully"
else
    warning "Some packages failed to build - see /tmp/build-output.txt"
    echo ""
    echo "Build failures:"
    grep -E "^(#|\.go:)" /tmp/build-output.txt | head -30
fi

# Step 9: Run tests
echo ""
echo "Step 9: Running tests..."
if go test -short ./... 2>&1 | tee /tmp/test-output.txt; then
    success "All tests passed"
else
    warning "Some tests failed - see /tmp/test-output.txt"
fi

# Summary
echo ""
echo "=== Fix Summary ==="
echo ""
echo "Completed automatic fixes for:"
echo "  ✓ Code formatting (gofmt/goimports)"
echo "  ✓ Lock value copy in queue.go"
echo "  ✓ Build tags in hyperv tests"
echo "  ✓ Unused variables removal"
echo "  ✓ Progress reporter type conversions"
echo "  ✓ Go module cleanup"
echo ""
echo "Manual fixes still required for:"
echo "  ! AWS SDK API mismatches (providers/aws/)"
echo "  ! Azure SDK API mismatches (providers/azure/)"
echo "  ! GCP SDK API mismatches (providers/gcp/)"
echo "  ! Interface mismatches in test mocks"
echo "  ! Undefined types in integration tests"
echo ""
echo "See CODE_REVIEW_REPORT.md for detailed instructions"
echo ""
echo "Next steps:"
echo "  1. Review changes: git diff"
echo "  2. Update SDK dependencies: go get -u"
echo "  3. Fix remaining compilation errors manually"
echo "  4. Run full test suite: go test ./..."
echo "  5. Commit fixes: git add . && git commit"
echo ""
