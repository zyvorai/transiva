#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"
export PKG_INSTALL_ROOT="${ROOT}"
if [[ -f "${ROOT}/.package-lib/zyvor-company-accept.sh" ]]; then
  # shellcheck source=/dev/null
  source "${ROOT}/.package-lib/zyvor-company-accept.sh"
  require_zyvor_company_accept "${ROOT}"
fi

# shellcheck source=/dev/null
[[ -f "${ROOT}/.package-lib/package-ui.sh" ]] && source "${ROOT}/.package-lib/package-ui.sh"

pkg_parse_install_args "$@"

_PKG_SESSION_START=${SECONDS}
pkg_install_welcome "HyperSDK"
pkg_banner "HyperSDK" "Multi-cloud VM platform · client bundle"
pkg_step_init 4

pkg_step "System dependencies"
[[ -x ./install-client-deps.sh ]] && { ./install-client-deps.sh || pkg_warn "deps issues"; pkg_step_done; } || { pkg_fail "install-client-deps.sh missing"; exit 1; }

pkg_step "Configuration"
pkg_env_bootstrap transiva.env.example transiva.env 2>/dev/null || pkg_env_bootstrap config.yaml.example config.yaml 2>/dev/null || pkg_ok "use ~/.config/transiva/config.yaml"
pkg_step_done

pkg_step "Verify binaries"
[[ -x ./bin/hypervisord ]] && pkg_ok "bin/hypervisord" || { pkg_fail "bin/hypervisord missing"; exit 1; }
[[ -d ./dashboard ]] && pkg_ok "dashboard/" || pkg_warn "dashboard/ missing"
pkg_step_done

pkg_step "Smoke test"
[[ -x ./test-package.sh ]] && ./test-package.sh || pkg_warn "test-package.sh"
pkg_step_done

pkg_install_finish "HyperSDK" https 5080 "/web/dashboard/" \
  "Start: ./bin/hypervisord" \
  "CLI: ./bin/hyperctl --help" \
  "Help: cat HELP.txt · ./install.sh --help" \
  "Config: ~/.config/transiva/config.yaml"
