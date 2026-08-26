#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# ============================================================================
# package-binary-remote.sh — Build HyperSDK on a remote Linux host and tarball it
# ============================================================================
# Minimal rsync, remote `npm` dashboard + `make build` (same as deploy-remote),
# tarball Go binaries from build/ + embedded static-react for client handoff.
#
# Usage:
#   ./scripts/package-binary-remote.sh <host> [user] [--fetch] [--reuse-build] [--skip-dashboard]
#
# See: docs/PACKAGE_BINARY_REMOTE.md
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=lib/zyvor-company-accept.sh
source "${SCRIPT_DIR}/lib/zyvor-company-accept.sh"
require_zyvor_company_accept "${REPO_DIR}"
# shellcheck source=lib/deploy-common.sh
source "${SCRIPT_DIR}/lib/deploy-common.sh"

FETCH=false
REUSE_BUILD=false
SKIP_DASHBOARD=false
SKIP_DEPS=false
POSITIONAL=()

for arg in "$@"; do
    case "$arg" in
        --fetch) FETCH=true ;;
        --reuse-build) REUSE_BUILD=true ;;
        --skip-dashboard) SKIP_DASHBOARD=true ;;
        --skip-deps) SKIP_DEPS=true ;;
        -h|--help)
            sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) POSITIONAL+=("$arg") ;;
    esac
done

HOST="${POSITIONAL[0]:-${DEPLOY_HOST:-}}"
USER="${POSITIONAL[1]:-${DEPLOY_USER:-sus}}"
SSH_TIMEOUT="${DEPLOY_SSH_TIMEOUT:-20}"

if [[ -z "${HOST}" ]]; then
    echo "Usage: $0 <host> [user] [--fetch] [--reuse-build]" >&2
    exit 1
fi

hypersdk_build_metadata "${REPO_DIR}"
VERSION="${HYPERSDK_PACKAGE_VERSION:-${HYPERSDK_VERSION:-dev}}"
ARCH="linux-amd64"
REMOTE="${USER}@${HOST}"
REMOTE_HOME=$(ssh -o BatchMode=yes -o ConnectTimeout="${SSH_TIMEOUT}" "${REMOTE}" 'echo "$HOME"')
BUILD_DIR="${REMOTE_HOME}/.deployment/transiva-package"
OUT_DIR="${HYPERSDK_PACKAGE_DIR:-${REMOTE_HOME}/transiva-dist}"
ARTIFACT="transiva-${VERSION}-${ARCH}"
LOCAL_DIST="${REPO_DIR}/dist"

RSYNC_EXCLUDES=(
    --exclude='build/'
    --exclude='.git/'
    --exclude='web/dashboard-react/node_modules/'
    --exclude='ui/node_modules/'
)

# shellcheck source=lib/package-remote-ui.sh
source "${SCRIPT_DIR}/lib/package-remote-ui.sh"

pkg_remote_banner "HyperSDK" "${VERSION}" "${REMOTE}" "${ARCH}"

if [[ "${HYPERSDK_REMOTE_SKIP_SSH_CHECK:-}" != "1" ]]; then
    pkg_remote_phase "Preflight"
    ssh -o BatchMode=yes -o ConnectTimeout="${SSH_TIMEOUT}" -o StrictHostKeyChecking=accept-new \
        "${REMOTE}" "true"
    pkg_ok "SSH ${REMOTE}"
fi

pkg_remote_phase "Sync source"
pkg_remote_kv "Build dir" "${BUILD_DIR}"
ssh "${REMOTE}" "mkdir -p '${BUILD_DIR}'"
rsync -az --delete "${RSYNC_EXCLUDES[@]}" \
    -e "ssh -o StrictHostKeyChecking=no -o ServerAliveInterval=15 -o ServerAliveCountMax=120" \
    "${REPO_DIR}/" "${REMOTE}:${BUILD_DIR}/"

SKIP_FLAG=0
$SKIP_DASHBOARD && SKIP_FLAG=1

if ! $SKIP_DEPS; then
    pkg_remote_phase "Build dependencies"
    DEPS_SCRIPT=$(hypersdk_package_install_build_deps)
    ssh "${REMOTE}" bash -s <<< "${DEPS_SCRIPT}" 2>&1 | sed 's/^/  [deps] /'
    pkg_ok "go + npm deps"
fi

if $REUSE_BUILD; then
    if ssh "${REMOTE}" "test -d '${BUILD_DIR}/build' && [ -n \"\$(ls -A '${BUILD_DIR}/build' 2>/dev/null)\" ]"; then
        pkg_ok "Reusing build/ (--reuse-build)"
    else
        pkg_remote_phase "Compile (go + npm)"
        REMOTE_BUILD_SCRIPT=$(hypersdk_remote_build_go_body "${BUILD_DIR}" "${SKIP_FLAG}")
        pkg_info "First build often 10–20 minutes…"
        ssh "${REMOTE}" "${REMOTE_BUILD_SCRIPT}" 2>&1 | sed 's/^/  [build] /'
        pkg_ok "build/ ready"
    fi
else
    pkg_remote_phase "Compile (go + npm)"
    REMOTE_BUILD_SCRIPT=$(hypersdk_remote_build_go_body "${BUILD_DIR}" "${SKIP_FLAG}")
    pkg_info "First build often 10–20 minutes…"
    ssh "${REMOTE}" "${REMOTE_BUILD_SCRIPT}" 2>&1 | sed 's/^/  [build] /'
    pkg_ok "build/ ready"
fi

pkg_remote_phase "Assemble customer bundle"
pkg_remote_kv "Output" "${OUT_DIR}/${ARTIFACT}"
ssh "${REMOTE}" bash -s <<REMOTE_PACK
set -euo pipefail
OUT_DIR='${OUT_DIR}'
BUILD_DIR='${BUILD_DIR}'
ARTIFACT='${ARTIFACT}'
VERSION='${VERSION}'

# shellcheck source=scripts/lib/package-transiva-client-bundle.sh
source "\${BUILD_DIR}/scripts/lib/package-transiva-client-bundle.sh"
STAGE="\${OUT_DIR}/\${ARTIFACT}"
package_hypersdk_client_bundle "\${STAGE}" "\${BUILD_DIR}" "\${VERSION}"
package_hypersdk_client_tarball "\${OUT_DIR}" "\${ARTIFACT}" "\${STAGE}"
ls -lh "\${OUT_DIR}/\${ARTIFACT}.tar.gz"
ls -1 "\${STAGE}/bin" | head -10
REMOTE_PACK

TARBALL="${ARTIFACT}.tar.gz"
REMOTE_TARBALL="${OUT_DIR}/${TARBALL}"

if $FETCH; then
    pkg_remote_phase "Fetch to laptop"
    mkdir -p "${LOCAL_DIST}"
    scp -o StrictHostKeyChecking=no \
        "${REMOTE}:${REMOTE_TARBALL}" \
        "${REMOTE}:${OUT_DIR}/${TARBALL}.sha256" \
        "${LOCAL_DIST}/"
    (cd "${LOCAL_DIST}" && shasum -a 256 -c "${TARBALL}.sha256" 2>/dev/null || sha256sum -c "${TARBALL}.sha256") && pkg_ok "Checksum verified"
fi

pkg_remote_done "HyperSDK" "${REMOTE}:${REMOTE_TARBALL}" "${REMOTE}:${OUT_DIR}/${TARBALL}.sha256"
