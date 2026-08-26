#!/bin/bash
# SPDX-License-Identifier: Apache-2.0


# HyperSDK Helm Chart Packaging Script
# Packages the Helm chart and optionally publishes to a repository

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CHART_PATH="${CHART_PATH:-deployments/helm/transiva}"
OUTPUT_DIR="${OUTPUT_DIR:-deployments/helm/packages}"
REPO_URL="${REPO_URL:-https://ssahani.github.io/transiva}"
SIGN_CHART="${SIGN_CHART:-false}"

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

Package the HyperSDK Helm chart for distribution.

Options:
    -h, --help              Show this help message
    -c, --chart PATH        Path to Helm chart (default: ${CHART_PATH})
    -o, --output DIR        Output directory for packages (default: ${OUTPUT_DIR})
    -r, --repo-url URL      Helm repository URL (default: ${REPO_URL})
    -s, --sign              Sign the chart package with GPG
    -u, --update-index      Update Helm repository index
    -p, --publish           Publish to GitHub Pages
    -v, --version VERSION   Override chart version
    --skip-lint             Skip Helm lint before packaging
    --skip-test             Skip chart tests before packaging

Examples:
    $0                                  # Package chart
    $0 --sign                          # Package and sign chart
    $0 --update-index                  # Package and update index
    $0 --publish                       # Package, update index, and publish
    $0 --version 0.3.0                 # Override chart version
EOF
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

    if [ "${SIGN_CHART}" = "true" ]; then
        if ! command -v gpg &> /dev/null; then
            log_error "gpg is not installed. Required for signing charts."
            exit 1
        fi
        log_info "GPG available for chart signing"
    fi
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

# Package chart
package_chart() {
    local chart_name=$(get_chart_name)
    local chart_version=$(get_chart_version)

    log_info "Packaging chart ${chart_name} version ${chart_version}..."

    # Create output directory
    mkdir -p "${OUTPUT_DIR}"

    # Package command
    local package_cmd="helm package ${CHART_PATH} --destination ${OUTPUT_DIR}"

    if [ "${SIGN_CHART}" = "true" ]; then
        package_cmd="${package_cmd} --sign --key '${GPG_KEY}' --keyring '${GPG_KEYRING}'"
    fi

    if eval "${package_cmd}"; then
        local package_file="${OUTPUT_DIR}/${chart_name}-${chart_version}.tgz"
        log_success "Chart packaged: ${package_file}"

        # Show package info
        log_info "Package information:"
        ls -lh "${package_file}"

        # Show package contents
        log_info "Package contents:"
        tar -tzf "${package_file}" | head -20

        echo "${package_file}"
    else
        log_error "Failed to package chart"
        exit 1
    fi
}

# Update Helm repository index
update_repo_index() {
    log_info "Updating Helm repository index..."

    local index_file="${OUTPUT_DIR}/index.yaml"

    if [ -f "${index_file}" ]; then
        log_info "Merging with existing index..."
        helm repo index "${OUTPUT_DIR}" --url "${REPO_URL}" --merge "${index_file}"
    else
        log_info "Creating new index..."
        helm repo index "${OUTPUT_DIR}" --url "${REPO_URL}"
    fi

    if [ -f "${index_file}" ]; then
        log_success "Repository index updated: ${index_file}"

        # Show index info
        log_info "Index entries:"
        grep -A 5 'entries:' "${index_file}" | head -20
    else
        log_error "Failed to update repository index"
        exit 1
    fi
}

# Publish to GitHub Pages
publish_to_github_pages() {
    log_info "Publishing to GitHub Pages..."

    local gh_pages_dir="docs/helm-charts"

    # Create GitHub Pages directory
    mkdir -p "${gh_pages_dir}"

    # Copy packages and index to GitHub Pages directory
    cp -r "${OUTPUT_DIR}"/* "${gh_pages_dir}/"

    # Create index.html for repository
    cat > "${gh_pages_dir}/index.html" <<'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HyperSDK Helm Charts</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            max-width: 900px;
            margin: 40px auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 { color: #2c3e50; }
        h2 { color: #34495e; margin-top: 30px; }
        pre {
            background: #f4f4f4;
            border: 1px solid #ddd;
            border-radius: 4px;
            padding: 15px;
            overflow-x: auto;
        }
        code {
            background: #f4f4f4;
            padding: 2px 5px;
            border-radius: 3px;
        }
        .warning {
            background: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 10px 15px;
            margin: 20px 0;
        }
        .info {
            background: #d1ecf1;
            border-left: 4px solid #17a2b8;
            padding: 10px 15px;
            margin: 20px 0;
        }
        a { color: #3498db; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>HyperSDK Helm Charts Repository</h1>

    <p>Official Helm charts for deploying HyperSDK to Kubernetes clusters.</p>

    <h2>Quick Start</h2>

    <p>Add the HyperSDK Helm repository:</p>
    <pre>helm repo add transiva https://ssahani.github.io/transiva/helm-charts
helm repo update</pre>

    <p>Install HyperSDK:</p>
    <pre>helm install transiva transiva/transiva \
  --namespace transiva \
  --create-namespace</pre>

    <h2>Available Charts</h2>

    <h3>transiva</h3>
    <p>Multi-cloud VM export and migration toolkit with support for vSphere, AWS, Azure, GCP, and more.</p>

    <p><strong>Installation:</strong></p>
    <pre>helm install transiva transiva/transiva</pre>

    <p><strong>Features:</strong></p>
    <ul>
        <li>Multi-cloud support (9 providers)</li>
        <li>Production-ready with HA support</li>
        <li>Auto-scaling with HPA</li>
        <li>Prometheus metrics integration</li>
        <li>Network policies and security hardening</li>
        <li>Cloud-specific optimizations (GKE, EKS, AKS)</li>
    </ul>

    <h2>Cloud Provider Examples</h2>

    <h3>Google Kubernetes Engine (GKE)</h3>
    <pre>helm install transiva transiva/transiva \
  --set replicaCount=3 \
  --set serviceAccount.annotations."iam\.gke\.io/gcp-service-account"=transiva@PROJECT_ID.iam.gserviceaccount.com \
  --set persistence.data.storageClass=standard-rwo \
  --namespace transiva \
  --create-namespace</pre>

    <h3>Amazon Elastic Kubernetes Service (EKS)</h3>
    <pre>helm install transiva transiva/transiva \
  --set replicaCount=3 \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=arn:aws:iam::ACCOUNT_ID:role/transiva-role \
  --set persistence.data.storageClass=gp3 \
  --namespace transiva \
  --create-namespace</pre>

    <h3>Azure Kubernetes Service (AKS)</h3>
    <pre>helm install transiva transiva/transiva \
  --set replicaCount=3 \
  --set serviceAccount.labels.aadpodidbinding=transiva-identity \
  --set persistence.data.storageClass=managed-premium \
  --namespace transiva \
  --create-namespace</pre>

    <h2>Configuration</h2>

    <p>See the <a href="https://github.com/ssahani/transiva/tree/main/deployments/helm/transiva">chart README</a> for complete configuration options.</p>

    <p>View all available parameters:</p>
    <pre>helm show values transiva/transiva</pre>

    <h2>Documentation</h2>

    <ul>
        <li><a href="https://github.com/ssahani/transiva">GitHub Repository</a></li>
        <li><a href="https://github.com/ssahani/transiva/tree/main/docs">Documentation</a></li>
        <li><a href="https://github.com/ssahani/transiva/tree/main/deployments/helm">Helm Charts</a></li>
        <li><a href="https://github.com/ssahani/transiva/issues">Issue Tracker</a></li>
    </ul>

    <h2>Support</h2>

    <p>For questions and support:</p>
    <ul>
        <li>Report issues on <a href="https://github.com/ssahani/transiva/issues">GitHub Issues</a></li>
        <li>View the <a href="https://github.com/ssahani/transiva/tree/main/docs/reference/troubleshooting-guide.md">Troubleshooting Guide</a></li>
    </ul>

    <hr>
    <p style="color: #7f8c8d; font-size: 0.9em;">
        Last updated: <span id="updated"></span> |
        <a href="https://github.com/ssahani/transiva">HyperSDK Project</a>
    </p>

    <script>
        document.getElementById('updated').textContent = new Date().toLocaleDateString();
    </script>
</body>
</html>
EOF

    log_success "GitHub Pages files created in ${gh_pages_dir}"

    # Create README for helm-charts directory
    cat > "${gh_pages_dir}/README.md" <<'EOF'
# HyperSDK Helm Charts Repository

This directory contains packaged Helm charts for HyperSDK, published via GitHub Pages.

## Repository URL

```
https://ssahani.github.io/transiva/helm-charts
```

## Usage

Add the repository:
```bash
helm repo add transiva https://ssahani.github.io/transiva/helm-charts
helm repo update
```

Install a chart:
```bash
helm install transiva transiva/transiva
```

## Available Charts

- **transiva** - Multi-cloud VM export and migration toolkit

## Files

- `index.yaml` - Helm repository index
- `index.html` - Web interface for the repository
- `*.tgz` - Packaged Helm charts

## Automation

This directory is automatically updated by the chart packaging and release workflows.

Do not manually edit files in this directory.
EOF

    log_info "To publish, commit and push the ${gh_pages_dir} directory"
    log_info "Ensure GitHub Pages is enabled for the repository"
}

# Main execution
main() {
    local do_update_index=false
    local do_publish=false
    local skip_lint=false
    local skip_test=false

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
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -r|--repo-url)
                REPO_URL="$2"
                shift 2
                ;;
            -s|--sign)
                SIGN_CHART=true
                shift
                ;;
            -u|--update-index)
                do_update_index=true
                shift
                ;;
            -p|--publish)
                do_publish=true
                do_update_index=true
                shift
                ;;
            -v|--version)
                OVERRIDE_VERSION="$2"
                shift 2
                ;;
            --skip-lint)
                SKIP_LINT=true
                shift
                ;;
            --skip-test)
                SKIP_TEST=true
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
    echo "  HyperSDK Helm Chart Packaging"
    echo "========================================"
    echo ""

    check_prerequisites
    validate_chart
    update_chart_version

    local package_file=$(package_chart)

    if [ "${do_update_index}" = "true" ]; then
        update_repo_index
    fi

    if [ "${do_publish}" = "true" ]; then
        publish_to_github_pages
    fi

    echo ""
    echo "========================================"
    echo "  Packaging Complete"
    echo "========================================"
    echo ""
    log_success "Chart package: ${package_file}"

    if [ "${do_update_index}" = "true" ]; then
        log_success "Repository index updated"
    fi

    if [ "${do_publish}" = "true" ]; then
        log_success "GitHub Pages files created"
        echo ""
        log_info "Next steps:"
        echo "  1. git add docs/helm-charts"
        echo "  2. git commit -m 'helm: Publish chart version $(get_chart_version)'"
        echo "  3. git push origin main"
        echo "  4. Enable GitHub Pages in repository settings (source: main branch, /docs folder)"
    else
        echo ""
        log_info "To publish to Helm repository:"
        echo "  $0 --publish"
    fi

    echo ""
}

main "$@"
