#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# Build HyperSDK and assemble customer tarball (local / GitHub Actions).
# Usage: ./scripts/package-binary-release.sh [--build] [--skip-dashboard] [--out-dir DIR]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

DO_BUILD=false
SKIP_DASHBOARD=false
OUT_DIR="${HYPERSDK_PACKAGE_DIR:-${REPO_DIR}/dist}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --build) DO_BUILD=true; shift ;;
        --skip-dashboard) SKIP_DASHBOARD=true; shift ;;
        --out-dir) OUT_DIR="${2:?}"; shift 2 ;;
        -h|--help)
            sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

# shellcheck source=lib/deploy-common.sh
source "${SCRIPT_DIR}/lib/deploy-common.sh"
hypersdk_build_metadata "${REPO_DIR}"
VERSION="${HYPERSDK_PACKAGE_VERSION:-${HYPERSDK_VERSION:-dev}}"
ARTIFACT="transiva-${VERSION}-linux-amd64"

if $DO_BUILD; then
    cd "${REPO_DIR}"
    export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"
    if $SKIP_DASHBOARD; then
        mkdir -p daemon/dashboard/static-react
        printf '%s\n' '<!DOCTYPE html><html><body>Dashboard disabled</body></html>' > daemon/dashboard/static-react/index.html
    else
        hypersdk_build_dashboard "${REPO_DIR}"
    fi
    make build-daemon build-export
    make build-ctl || echo "WARN: hyperctl build skipped"
fi

# shellcheck source=lib/package-transiva-client-bundle.sh
source "${SCRIPT_DIR}/lib/package-transiva-client-bundle.sh"

STAGE="${OUT_DIR}/${ARTIFACT}"
echo "Assemble customer bundle → ${OUT_DIR}/${ARTIFACT}.tar.gz"
package_hypersdk_client_bundle "${STAGE}" "${REPO_DIR}" "${VERSION}"
package_hypersdk_client_tarball "${OUT_DIR}" "${ARTIFACT}" "${STAGE}"
ls -lh "${OUT_DIR}/${ARTIFACT}.tar.gz"
ls -1 "${STAGE}/bin" | head -10
echo "Done: ${OUT_DIR}/${ARTIFACT}.tar.gz"
