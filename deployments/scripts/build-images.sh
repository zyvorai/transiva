#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# ==============================================================================
# HyperSDK Container Image Build Script
# Builds Docker/Podman images for all HyperSDK components
# ==============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DOCKERFILES_DIR="${PROJECT_ROOT}/deployments/docker/dockerfiles"

# Default values
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
COMMIT_SHA=${COMMIT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
BUILD_DATE=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}
REGISTRY=${REGISTRY:-docker.io/transiva}
BUILDER=${BUILDER:-docker}  # docker or podman
PUSH=${PUSH:-false}
NO_CACHE=${NO_CACHE:-false}

# Components to build
COMPONENTS=("hypervisord" "hyperexport")

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

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Build HyperSDK container images

OPTIONS:
    -v, --version VERSION     Image version (default: git describe or 'dev')
    -r, --registry REGISTRY   Container registry (default: docker.io/transiva)
    -b, --builder BUILDER     Builder to use: docker or podman (default: docker)
    -p, --push                Push images to registry after build
    -n, --no-cache            Build without cache
    -c, --component NAME      Build specific component only
    -h, --help                Show this help message

EXAMPLES:
    # Build all images with default settings
    $0

    # Build with specific version
    $0 --version v0.2.0

    # Build and push to registry
    $0 --version v0.2.0 --push

    # Build with Podman
    $0 --builder podman

    # Build single component
    $0 --component hypervisord

EOF
}

check_builder() {
    if ! command -v "${BUILDER}" &> /dev/null; then
        log_error "${BUILDER} is not installed or not in PATH"
        exit 1
    fi
    log_info "Using builder: ${BUILDER}"
}

build_image() {
    local component=$1
    local dockerfile="${DOCKERFILES_DIR}/Dockerfile.${component}"

    if [ ! -f "${dockerfile}" ]; then
        log_error "Dockerfile not found: ${dockerfile}"
        return 1
    fi

    log_info "Building ${component} image..."
    log_info "  Version: ${VERSION}"
    log_info "  Commit:  ${COMMIT_SHA}"
    log_info "  Date:    ${BUILD_DATE}"

    local build_args=(
        "--build-arg" "VERSION=${VERSION}"
        "--build-arg" "COMMIT_SHA=${COMMIT_SHA}"
        "--build-arg" "BUILD_DATE=${BUILD_DATE}"
        "--file" "${dockerfile}"
        "--tag" "${REGISTRY}/${component}:${VERSION}"
        "--tag" "${REGISTRY}/${component}:latest"
    )

    if [ "${NO_CACHE}" = "true" ]; then
        build_args+=("--no-cache")
    fi

    if ! ${BUILDER} build "${build_args[@]}" "${PROJECT_ROOT}"; then
        log_error "Failed to build ${component}"
        return 1
    fi

    log_info "Successfully built ${component}"
    return 0
}

push_image() {
    local component=$1

    log_info "Pushing ${component} images to ${REGISTRY}..."

    if ! ${BUILDER} push "${REGISTRY}/${component}:${VERSION}"; then
        log_error "Failed to push ${component}:${VERSION}"
        return 1
    fi

    if ! ${BUILDER} push "${REGISTRY}/${component}:latest"; then
        log_error "Failed to push ${component}:latest"
        return 1
    fi

    log_info "Successfully pushed ${component}"
    return 0
}

# ==============================================================================
# Main
# ==============================================================================

# Parse arguments
SELECTED_COMPONENT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -b|--builder)
            BUILDER="$2"
            shift 2
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        -n|--no-cache)
            NO_CACHE=true
            shift
            ;;
        -c|--component)
            SELECTED_COMPONENT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Check builder
check_builder

# Determine components to build
if [ -n "${SELECTED_COMPONENT}" ]; then
    COMPONENTS=("${SELECTED_COMPONENT}")
fi

# Build images
log_info "Starting build process..."
log_info "Registry: ${REGISTRY}"
log_info "Version:  ${VERSION}"

FAILED_BUILDS=()

for component in "${COMPONENTS[@]}"; do
    if ! build_image "${component}"; then
        FAILED_BUILDS+=("${component}")
    fi
done

# Check for failures
if [ ${#FAILED_BUILDS[@]} -gt 0 ]; then
    log_error "Failed to build: ${FAILED_BUILDS[*]}"
    exit 1
fi

log_info "All images built successfully!"

# List images
log_info "Built images:"
${BUILDER} images | grep "${REGISTRY}" || true

# Push images if requested
if [ "${PUSH}" = "true" ]; then
    log_info "Pushing images to registry..."

    for component in "${COMPONENTS[@]}"; do
        if ! push_image "${component}"; then
            log_error "Failed to push ${component}"
            exit 1
        fi
    done

    log_info "All images pushed successfully!"
fi

log_info "Build complete!"
