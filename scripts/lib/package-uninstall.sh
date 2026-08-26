#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
[[ -f "${ROOT}/.package-lib/package-ui.sh" ]] && source "${ROOT}/.package-lib/package-ui.sh"
if [[ -f "${ROOT}/.package-lib/package-uninstall-lib.sh" ]]; then
  # shellcheck source=/dev/null
  source "${ROOT}/.package-lib/package-uninstall-lib.sh"
else
  # shellcheck source=package-uninstall-lib.sh
  source "$(dirname "$0")/package-uninstall-lib.sh"
fi

PRODUCT="HyperSDK"
BINARIES=(hypervisord)
BINARIES_SUBPATH=(bin/hypervisord)
PORTS=(5080)
LOCAL_CONFIGS=(config.yaml.local)
SYSTEM_PATHS=("${HOME}/.config/transiva")

package_uninstall_main "${PRODUCT}" "${ROOT}" "$@"
