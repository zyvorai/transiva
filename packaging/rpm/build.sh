#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# RPM Build Script for h2kvm-daemon
#
# This script builds RPM packages for the h2kvm systemd daemon.
#
# Usage:
#   ./build.sh [options]
#
# Options:
#   --version VERSION   Package version (default: 1.0.0)
#   --release RELEASE   Package release (default: 1)
#   --clean             Clean build artifacts before building
#   --help              Show this help message

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
VERSION="1.0.0"
RELEASE="1"
CLEAN=false

# Function to print colored messages
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Function to show usage
show_usage() {
    cat << EOF
RPM Build Script for h2kvm-daemon

Usage:
  $0 [options]

Options:
  --version VERSION   Package version (default: 1.0.0)
  --release RELEASE   Package release (default: 1)
  --clean             Clean build artifacts before building
  --help              Show this help message

Examples:
  # Build with default version
  $0

  # Build specific version
  $0 --version 1.1.0 --release 2

  # Clean build
  $0 --clean

Requirements:
  - rpmbuild command (install: yum install rpm-build or dnf install rpm-build)
  - rpmdevtools (install: yum install rpmdevtools)

Output:
  RPM packages will be in: ~/rpmbuild/RPMS/noarch/

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --release)
            RELEASE="$2"
            shift 2
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        --help)
            show_usage
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Check for required tools
step "Checking build requirements..."

if ! command -v rpmbuild &> /dev/null; then
    error "rpmbuild not found. Install with: sudo yum install rpm-build"
    exit 1
fi

if ! command -v rpmdev-setuptree &> /dev/null; then
    warn "rpmdevtools not found. Install with: sudo yum install rpmdevtools"
    warn "Continuing without rpmdev-setuptree..."
fi

info "Build tools are available"

# Setup RPM build tree
step "Setting up RPM build tree..."

if command -v rpmdev-setuptree &> /dev/null; then
    rpmdev-setuptree
else
    # Manually create directories
    mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
fi

info "RPM build tree ready"

# Clean if requested
if $CLEAN; then
    step "Cleaning previous build artifacts..."
    rm -rf ~/rpmbuild/BUILD/h2kvm-daemon-*
    rm -rf ~/rpmbuild/BUILDROOT/h2kvm-daemon-*
    rm -f ~/rpmbuild/SOURCES/h2kvm-daemon-*.tar.gz
    rm -f ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm
    rm -f ~/rpmbuild/SRPMS/h2kvm-daemon-*.rpm
    info "Cleaned build artifacts"
fi

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

step "Creating source tarball..."

# Create temporary directory for tarball
TEMP_DIR=$(mktemp -d)
PACKAGE_DIR="$TEMP_DIR/h2kvm-daemon-$VERSION"

mkdir -p "$PACKAGE_DIR"

# Copy systemd files
mkdir -p "$PACKAGE_DIR/systemd"
cp "$PROJECT_ROOT"/systemd/*.service "$PACKAGE_DIR/systemd/"
cp "$PROJECT_ROOT"/systemd/*.target "$PACKAGE_DIR/systemd/"
cp "$PROJECT_ROOT"/systemd/*.example "$PACKAGE_DIR/systemd/"
cp "$PROJECT_ROOT/systemd/README.md" "$PACKAGE_DIR/systemd/"

# Copy license
if [ -f "$PROJECT_ROOT/LICENSE" ]; then
    cp "$PROJECT_ROOT/LICENSE" "$PACKAGE_DIR/"
else
    # Create a basic LICENSE file if it doesn't exist
    cat > "$PACKAGE_DIR/LICENSE" << 'EOF'
Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

Copyright 2026 Zyvor AI Labs Private Limited

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
EOF
fi

# Create tarball
cd "$TEMP_DIR"
tar czf "h2kvm-daemon-${VERSION}.tar.gz" "h2kvm-daemon-${VERSION}"

# Move tarball to SOURCES
mv "h2kvm-daemon-${VERSION}.tar.gz" ~/rpmbuild/SOURCES/

# Cleanup temp dir
rm -rf "$TEMP_DIR"

info "Source tarball created: ~/rpmbuild/SOURCES/h2kvm-daemon-${VERSION}.tar.gz"

# Copy spec file
step "Copying spec file..."

cp "$SCRIPT_DIR/h2kvm-daemon.spec" ~/rpmbuild/SPECS/

# Update version and release in spec file
sed -i "s/^Version:.*/Version:        $VERSION/" ~/rpmbuild/SPECS/h2kvm-daemon.spec
sed -i "s/^Release:.*/Release:        $RELEASE%{?dist}/" ~/rpmbuild/SPECS/h2kvm-daemon.spec

info "Spec file ready: ~/rpmbuild/SPECS/h2kvm-daemon.spec"

# Build RPM
step "Building RPM package..."

rpmbuild -ba ~/rpmbuild/SPECS/h2kvm-daemon.spec

# Check build results
if [ $? -eq 0 ]; then
    echo
    info "✅ RPM build completed successfully!"
    echo
    info "Built packages:"
    ls -lh ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm 2>/dev/null || true
    ls -lh ~/rpmbuild/SRPMS/h2kvm-daemon-*.rpm 2>/dev/null || true
    echo
    info "To install:"
    echo "  sudo rpm -ivh ~/rpmbuild/RPMS/noarch/h2kvm-daemon-${VERSION}-${RELEASE}.*.rpm"
    echo
    info "To upgrade:"
    echo "  sudo rpm -Uvh ~/rpmbuild/RPMS/noarch/h2kvm-daemon-${VERSION}-${RELEASE}.*.rpm"
    echo
    info "To query package info:"
    echo "  rpm -qpi ~/rpmbuild/RPMS/noarch/h2kvm-daemon-${VERSION}-${RELEASE}.*.rpm"
    echo
    info "To list package contents:"
    echo "  rpm -qpl ~/rpmbuild/RPMS/noarch/h2kvm-daemon-${VERSION}-${RELEASE}.*.rpm"
else
    error "RPM build failed"
    exit 1
fi
