#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# Smoke-test HyperSDK client bundle after install.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"
export PKG_INSTALL_ROOT="${ROOT}"
# shellcheck source=/dev/null
[[ -f "${ROOT}/.package-lib/package-ui.sh" ]] && source "${ROOT}/.package-lib/package-ui.sh"

[[ "${1:-}" == "-h" || "${1:-}" == "--help" ]] && {
  pkg_script_help "test-package.sh"
  exit 0
}

_PKG_SESSION_START=${SECONDS}
pkg_counters_reset
pkg_banner "HyperSDK package test" "hypervisord · hyperctl · optional API"

if [[ -x ./bin/hypervisord ]]; then
  pkg_ok "bin/hypervisord present"
else
  pkg_fail "bin/hypervisord missing"
fi

if [[ -x ./bin/hyperctl ]]; then
  ./bin/hyperctl --help >/dev/null 2>&1 && pkg_ok "hyperctl --help" || pkg_fail "hyperctl --help"
else
  pkg_warn "bin/hyperctl missing"
fi

if systemctl is-active libvirtd &>/dev/null 2>&1; then
  pkg_ok "libvirtd active"
else
  pkg_warn "libvirtd not active — run ./install-client-deps.sh"
fi

if curl -skf https://127.0.0.1:5080/api/v1/health >/dev/null 2>&1; then
  pkg_ok "API health https://127.0.0.1:5080"
else
  pkg_skip "API not listening — start: ./bin/hypervisord"
fi

pkg_summary "Package test"
[[ "${_PKG_COUNTERS_FAIL}" -eq 0 ]]
