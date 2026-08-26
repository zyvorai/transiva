# SPDX-License-Identifier: Apache-2.0

# shellcheck shell=bash
# Assemble HyperSDK customer tarball (shared by remote pack and GitHub release).
#
# Usage: package_hypersdk_client_bundle STAGE BUILD_DIR VERSION

package_hypersdk_client_bundle() {
    local stage="$1" build_dir="$2" version="$3"
    local lib="${build_dir}/scripts/lib"

    rm -rf "${stage}"
    mkdir -p "${stage}/bin" "${stage}/dashboard"

    if [[ ! -d "${build_dir}/build" ]] || [[ -z "$(ls -A "${build_dir}/build" 2>/dev/null)" ]]; then
        echo "package_hypersdk_client_bundle: build/ empty — run make build first" >&2
        return 1
    fi

    cp -a "${build_dir}/build/." "${stage}/bin/"
    if [[ -d "${build_dir}/daemon/dashboard/static-react" ]]; then
        cp -a "${build_dir}/daemon/dashboard/static-react/." "${stage}/dashboard/"
    fi
    cp "${build_dir}/config.example.yaml" "${stage}/config.example.yaml" 2>/dev/null || true

    cat > "${stage}/transiva.env.example" <<'ENV_EOF'
# Copy to transiva.env and adjust
HYPERSDK_CONFIG=/etc/transiva/config.yaml
# VCENTER_HOST=
# VCENTER_USER=
# VCENTER_PASSWORD=
ENV_EOF

    for f in package-install.sh package-client-install.sh package-client-test.sh; do
        if [[ ! -f "${lib}/${f}" ]]; then
            echo "package_hypersdk_client_bundle: missing ${lib}/${f}" >&2
            return 1
        fi
    done

    cp "${lib}/package-install.sh" "${stage}/install.sh"
    cp "${lib}/package-client-install.sh" "${stage}/install-client-deps.sh"
    cp "${lib}/package-client-test.sh" "${stage}/test-package.sh"
    mkdir -p "${stage}/.package-lib"
    cp "${lib}/package-ui.sh" "${stage}/.package-lib/"
    cp "${lib}/install-everything.sh" "${stage}/"
    cp "${lib}/package-uninstall-lib.sh" "${stage}/.package-lib/"
    cp "${lib}/package-uninstall.sh" "${stage}/uninstall.sh"
    chmod +x "${stage}/install.sh" "${stage}/install-client-deps.sh" "${stage}/test-package.sh" \
        "${stage}/install-everything.sh" "${stage}/uninstall.sh"
    chmod +x "${lib}/write-customer-help.sh"
    "${lib}/write-customer-help.sh" "${stage}" "HyperSDK" platform
    cp "${lib}/START_HERE.txt" "${stage}/"

    cat > "${stage}/.package-lib/product.meta" <<'META'
PRODUCT_NAME=HyperSDK
ACCESS_SCHEME=https
ACCESS_PORT=5080
ACCESS_PATH=/web/dashboard/
AUTO_FULL_INSTALL=0
FINISH_EXTRA_1='Start: ./bin/hypervisord'
FINISH_EXTRA_2='CLI: ./bin/hyperctl --help'
FINISH_EXTRA_3=
META

    cat > "${stage}/QUICKSTART.txt" <<'QEOF'
HyperSDK — 5-minute install
============================

1. tar xzf transiva-*-linux-amd64.tar.gz && cd transiva-*-linux-amd64
2. sha256sum -c ../transiva-*-linux-amd64.tar.gz.sha256   # optional
3. ./install.sh
4. Edit ~/.config/transiva/config.yaml (vCenter, credentials)
5. ./bin/hypervisord
   https://<server-ip>:5080/web/dashboard/
6. ./test-package.sh

More: README.txt

Packaged by Zyvor — zyvor.dev · HyperSDK · © 2026
QEOF

    if [[ -f "${build_dir}/scripts/zyvor-branding/ZYVOR_INSTALL.txt" ]]; then
        cp "${build_dir}/scripts/zyvor-branding/ZYVOR_INSTALL.txt" "${stage}/ZYVOR_INSTALL.txt"
    fi

    chmod +x "${lib}/copy-zyvor-legal-to-bundle.sh"
    "${lib}/copy-zyvor-legal-to-bundle.sh" "${stage}" "${build_dir}" --with-accept

    cat > "${stage}/README.txt" <<README_EOF
HyperSDK ${version} — Linux amd64 client bundle
================================================

START: cat START_HERE.txt  |  full help: cat HELP.txt

WHAT IS IN THIS ARCHIVE
  bin/           hypervisord, hyperctl, hyperexport, …
  dashboard/     UI files (also embedded in hypervisord)
  install.sh, uninstall.sh  ← install / remove on this machine
  install-client-deps.sh, test-package.sh
  config.example.yaml, QUICKSTART.txt

REQUIREMENTS: Linux x86_64 KVM host, vCenter or supported source

CUSTOMER INSTALL
  tar xzf transiva-*-linux-amd64.tar.gz
  cd transiva-*-linux-amd64
  ./install.sh
  ./bin/hypervisord

Dashboard: https://<your-server>:5080/web/dashboard/

TEST: ./test-package.sh
UNINSTALL: ./uninstall.sh --yes  (add --remove-dir to delete this folder)
README_EOF

    local req
    for req in HELP.txt START_HERE.txt install.sh uninstall.sh README.txt QUICKSTART.txt \
        install-client-deps.sh test-package.sh bin/hypervisord config.example.yaml \
        LEGAL-INDEX.txt ZYVOR-COMPANY-TERMS.md LICENSE; do
        if [[ ! -e "${stage}/${req}" ]]; then
            echo "package_hypersdk_client_bundle: bundle missing ${req}" >&2
            return 1
        fi
    done

    chmod +x "${lib}/verify-bundle-script-paths.sh"
    "${lib}/verify-bundle-script-paths.sh" "${stage}" || return 1

    chmod +x "${lib}/finalize-customer-bundle.sh"
    "${lib}/finalize-customer-bundle.sh" "${stage}" "${build_dir}" "HyperSDK" "${version}" || return 1

    echo "Customer bundle OK"
}

package_hypersdk_client_tarball() {
    local out_dir="$1" artifact="$2" stage="$3"
    mkdir -p "${out_dir}"
    (
        cd "${out_dir}"
        tar czf "${artifact}.tar.gz" "$(basename "${stage}")"
        sha256sum "${artifact}.tar.gz" | tee "${artifact}.tar.gz.sha256"
    )
}
