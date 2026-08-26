#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# HyperSDK Pre-Commit Quality Checks
# Run this before committing to ensure code quality

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SKIP_TESTS="${SKIP_TESTS:-false}"
SKIP_LINT="${SKIP_LINT:-false}"
FIX_FORMAT="${FIX_FORMAT:-true}"

# Print header
print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

# Print step
print_step() {
    echo -e "\n${YELLOW}➜ $1${NC}"
}

# Print success
print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

# Print error
print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Print warning
print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Track failures
CHECKS_FAILED=0

# Main
print_header "HyperSDK Pre-Commit Checks"
echo ""

# 1. Format check and fix
print_step "Checking code formatting..."
if [[ "$FIX_FORMAT" == "true" ]]; then
    if gofmt -w .; then
        print_success "Code formatted"
    else
        print_error "Failed to format code"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
else
    UNFORMATTED=$(gofmt -l .)
    if [[ -z "$UNFORMATTED" ]]; then
        print_success "Code is properly formatted"
    else
        print_error "Code is not formatted. Run 'go fmt ./...' or 'make fmt'"
        echo "$UNFORMATTED"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
fi

# 2. Go vet
print_step "Running go vet..."
if go vet ./...; then
    print_success "No issues found by go vet"
else
    print_error "go vet found issues"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
fi

# 3. Go mod tidy
print_step "Checking go.mod and go.sum..."
cp go.mod go.mod.bak
cp go.sum go.sum.bak
if go mod tidy; then
    if diff -q go.mod go.mod.bak > /dev/null && diff -q go.sum go.sum.bak > /dev/null; then
        print_success "go.mod and go.sum are tidy"
        rm go.mod.bak go.sum.bak
    else
        print_warning "go.mod or go.sum needed tidying (fixed)"
        rm go.mod.bak go.sum.bak
    fi
else
    print_error "go mod tidy failed"
    mv go.mod.bak go.mod
    mv go.sum.bak go.sum
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
fi

# 4. Linter (if available and not skipped)
if [[ "$SKIP_LINT" == "false" ]]; then
    print_step "Running golangci-lint..."
    if command -v golangci-lint &> /dev/null; then
        if golangci-lint run ./... --timeout=3m; then
            print_success "No issues found by linter"
        else
            print_warning "Linter found issues (non-blocking)"
        fi
    else
        print_warning "golangci-lint not installed (skipping)"
    fi
else
    print_warning "Linting skipped (SKIP_LINT=true)"
fi

# 5. Tests (if not skipped)
if [[ "$SKIP_TESTS" == "false" ]]; then
    print_step "Running tests..."

    # Run fast tests for quick feedback
    if go test -short -timeout=30s ./... > /dev/null 2>&1; then
        print_success "All short tests passed"
    else
        print_error "Some tests failed"
        echo ""
        echo "Run 'go test ./...' to see details"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
else
    print_warning "Tests skipped (SKIP_TESTS=true)"
fi

# 6. Check for common issues
print_step "Checking for common issues..."

# Check for TODO/FIXME in staged files
if git diff --cached --name-only | grep -E '\.go$' > /dev/null; then
    STAGED_FILES=$(git diff --cached --name-only | grep -E '\.go$')
    TODOS=$(echo "$STAGED_FILES" | xargs grep -n "TODO\|FIXME" || true)
    if [[ -n "$TODOS" ]]; then
        print_warning "Found TODO/FIXME comments in staged files:"
        echo "$TODOS"
    fi
fi

# Check for debug statements
if git diff --cached --name-only | grep -E '\.go$' > /dev/null; then
    STAGED_FILES=$(git diff --cached --name-only | grep -E '\.go$')
    DEBUG=$(echo "$STAGED_FILES" | xargs grep -n "fmt.Print\|log.Print" | grep -v "logger\." || true)
    if [[ -n "$DEBUG" ]]; then
        print_warning "Found potential debug statements (fmt.Print/log.Print):"
        echo "$DEBUG"
        echo "Consider using structured logging instead"
    fi
fi

# Check for large files
if git diff --cached --name-only > /dev/null; then
    LARGE_FILES=$(git diff --cached --name-only | xargs ls -l 2>/dev/null | awk '$5 > 1000000 {print $9 " (" $5 " bytes)"}' || true)
    if [[ -n "$LARGE_FILES" ]]; then
        print_warning "Found large files being committed:"
        echo "$LARGE_FILES"
    fi
fi

print_success "Common issues check complete"

# 7. Security check (if gosec is available)
print_step "Running security checks..."
if command -v gosec &> /dev/null; then
    if gosec -quiet ./... 2>/dev/null; then
        print_success "No security issues found"
    else
        print_warning "Security scanner found potential issues (non-blocking)"
    fi
else
    print_warning "gosec not installed (skipping security scan)"
    echo "Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"
fi

# Summary
echo ""
print_header "Summary"

if [[ $CHECKS_FAILED -eq 0 ]]; then
    print_success "All critical checks passed! ✨"
    echo ""
    echo "You can now commit your changes:"
    echo "  git commit -m 'your commit message'"
    echo ""
    exit 0
else
    print_error "Some checks failed ($CHECKS_FAILED critical issues)"
    echo ""
    echo "Please fix the issues above before committing."
    echo ""
    echo "To skip specific checks, use:"
    echo "  SKIP_TESTS=true $0     # Skip tests"
    echo "  SKIP_LINT=true $0      # Skip linter"
    echo ""
    exit 1
fi
