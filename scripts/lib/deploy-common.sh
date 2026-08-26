# SPDX-License-Identifier: Apache-2.0
# shellcheck shell=bash
# HyperSDK deploy library (self-contained under scripts/lib/).

_DEPLOY_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

DEPLOY_UI_PROJECT="HyperSDK"
DEPLOY_UI_ICON="🚀"
DEPLOY_UI_ICON_UNINSTALL="🗑️"
DEPLOY_UI_ICON_MAGIC="✨"
DEPLOY_UI_PORT="5080"
DEPLOY_UI_SCHEME="https"
DEPLOY_UI_DASH_PATH="/web/dashboard/"
DEPLOY_UI_HEALTH_PATH="/api/v1/health"

# shellcheck source=deploy-ui.sh
source "$_DEPLOY_LIB_DIR/deploy-ui.sh"

hypersdk_info()  { deploy_ui_info "$@"; }
hypersdk_warn()  { deploy_ui_warn "$@"; }
hypersdk_error() { deploy_ui_error "$@"; }
hypersdk_print_banner() { deploy_ui_banner "$@"; }
hypersdk_step_start() { deploy_ui_step_start "$@"; }
hypersdk_step_finish() { deploy_ui_step_finish; }
hypersdk_spinner_start() { deploy_ui_spinner_start "$@"; }
hypersdk_spinner_stop() { deploy_ui_spinner_stop; }
hypersdk_print_success_art() {
    deploy_ui_success "$1" "$2" "./scripts/deploy remote --quick"
}

# ── Build metadata ────────────────────────────────────────────────────────────
hypersdk_build_metadata() {
    local repo_dir="$1"
    HYPERSDK_VERSION=$(git -C "$repo_dir" describe --tags --always --dirty 2>/dev/null || echo 'dev')
    HYPERSDK_COMMIT=$(git -C "$repo_dir" rev-parse --short HEAD 2>/dev/null || echo 'unknown')
    HYPERSDK_BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
    HYPERSDK_GO_VERSION=$(go version 2>/dev/null | awk '{print $3}' || echo 'unknown (local)')
    export HYPERSDK_VERSION HYPERSDK_COMMIT HYPERSDK_BUILD_TIME HYPERSDK_GO_VERSION
    HYPERSDK_LDFLAGS="-s -w \
        -X main.version=${HYPERSDK_VERSION} \
        -X main.commit=${HYPERSDK_COMMIT} \
        -X main.buildTime=${HYPERSDK_BUILD_TIME} \
        -X main.goVersion=${HYPERSDK_GO_VERSION}"
    export HYPERSDK_LDFLAGS
}

# All Go binaries we ship
hypersdk_binary_cmds() {
    cat <<'EOF'
hypervisord:cmd/hypervisord
hyperctl:cmd/hyperctl
hyperexport:cmd/hyperexport
hyperconvert:cmd/hyperconvert
transiva-dashboard:cmd/transiva-dashboard
transiva-agent:cmd/transiva-agent
transiva-control:cmd/transiva-control
transiva-operator:cmd/transiva-operator
EOF
}

# ── Preflight ─────────────────────────────────────────────────────────────────
hypersdk_preflight_local() {
    local repo_dir="$1"
    command -v go &>/dev/null || hypersdk_error "Go is required. Install: https://go.dev/dl/"
    hypersdk_preflight_deploy_sources "$repo_dir"
}

# Remote deploy: laptop only needs rsync + ssh; Go/npm build on the server.
hypersdk_preflight_deploy_sources() {
    local repo_dir="$1"
    [ -f "$repo_dir/go.mod" ] || hypersdk_error "Not in transiva repo: $repo_dir"
}

hypersdk_preflight_remote_tools() {
    local need_pass="$1"
    if [ "$need_pass" = "true" ] && ! command -v sshpass &>/dev/null; then
        hypersdk_error "sshpass required for password auth (dnf install sshpass / brew install sshpass)"
    fi
    command -v rsync &>/dev/null || hypersdk_error "rsync is required"
    command -v ssh &>/dev/null || hypersdk_error "ssh is required"
}

hypersdk_parse_target() { deploy_ui_parse_target "$@"; }

# ── Spinner / step timer ──────────────────────────────────────────────────────

hypersdk_step_start() {
    HYPERSDK_CURRENT_STEP="$1"
    deploy_ui_step_start "$1"
}

hypersdk_step_finish() { deploy_ui_step_finish; }

# ── Parallel Go build ─────────────────────────────────────────────────────────
hypersdk_build_go_binaries() {
    local repo_dir="$1" build_dir="$2" goos="${3:-}" goarch="${4:-}"
    local parallel="${5:-true}"
    mkdir -p "$build_dir"
    cd "$repo_dir" || return 1
    hypersdk_build_metadata "$repo_dir"

    local -a env_prefix=()
    [ -n "$goos" ] && env_prefix+=(GOOS="$goos")
    [ -n "$goarch" ] && env_prefix+=(GOARCH="$goarch")

    local failed=0
    local -a pids=()
    local -a names=()

    _build_one() {
        local name="$1" pkg="$2"
        if env "${env_prefix[@]}" go build -ldflags "$HYPERSDK_LDFLAGS" -o "$build_dir/$name" "./$pkg/" 2>/dev/null; then
            printf '  ✅ %s\n' "$name"
            return 0
        fi
        printf '  ❌ %s (build failed)\n' "$name"
        return 1
    }

    while IFS=: read -r name pkg; do
        [ -z "$name" ] && continue
        if [ "$parallel" = "true" ]; then
            (_build_one "$name" "$pkg") &
            pids+=($!)
            names+=("$name")
        else
            _build_one "$name" "$pkg" || failed=$((failed + 1))
        fi
    done < <(hypersdk_binary_cmds)

    if [ "$parallel" = "true" ]; then
        local i=0
        for pid in "${pids[@]}"; do
            if ! wait "$pid"; then
                failed=$((failed + 1))
                hypersdk_warn "${names[$i]} failed"
            fi
            i=$((i + 1))
        done
    fi

    cd - >/dev/null || true
    [ "$failed" -eq 0 ]
}

# ── Dashboard build ───────────────────────────────────────────────────────────
hypersdk_build_dashboard() {
    local repo_dir="$1"
    local dashboard_dir="$repo_dir/web/dashboard-react"
    local static_dir="$repo_dir/daemon/dashboard/static-react"

    if [ ! -f "$dashboard_dir/package.json" ]; then
        hypersdk_warn "Dashboard package.json not found — skipping"
        return 0
    fi

  if ! command -v npm &>/dev/null; then
        hypersdk_warn "npm not found — skipping dashboard build"
        return 0
    fi

    cd "$dashboard_dir" || return 1
    if [ -f package-lock.json ]; then
        npm ci --legacy-peer-deps --silent 2>&1 | tail -2 || npm install --silent 2>&1 | tail -1
    else
        npm install --silent 2>&1 | tail -1
    fi
    if npm run build 2>&1 | tail -5; then
        hypersdk_info "Dashboard → $static_dir"
        cd - >/dev/null || true
        return 0
    fi
    cd - >/dev/null || true
    return 1
}

# Top-level trees required for `go list -deps ./cmd/...` (keep rsync small — do not push whole repo).
hypersdk_deploy_source_dirs() {
    printf '%s\n' cmd internal pkg daemon providers config network logger manifest progress retry
}

# Remote deploy: npm build (embed FS) + make build on the SSH host.
hypersdk_remote_build_go_body() {
    local remote_dir="$1"
    local skip_dashboard="${2:-0}"
    cat <<EOF
set -euo pipefail
cd "$remote_dir"
export PATH="\${HOME}/go/bin:/usr/local/go/bin:/usr/bin:\${PATH}"
SUDO=""
[ "\$(id -u)" -ne 0 ] && command -v sudo &>/dev/null && SUDO=sudo

if ! command -v go &>/dev/null; then
    echo 'ERROR: go not on PATH — run full deploy once (installs golang) or: sudo dnf install -y golang' >&2
    exit 1
fi

# hypervisord embeds daemon/dashboard/static-react — must exist before go build.
mkdir -p daemon/dashboard/static-react
if [ "$skip_dashboard" = "1" ]; then
    echo '🌐 Dashboard skipped — minimal placeholder for go:embed'
    printf '%s\n' '<!DOCTYPE html><html><body>Dashboard disabled</body></html>' > daemon/dashboard/static-react/index.html
else
    echo '🌐 Building dashboard (npm) → daemon/dashboard/static-react ...'
    if command -v dnf &>/dev/null; then
        nmaj=0
        command -v node &>/dev/null && nmaj=\$(node -p 'parseInt(process.versions.node,10)' 2>/dev/null || echo 0)
        if [ "\${nmaj:-0}" -lt 18 ]; then
            \$SUDO dnf module reset -y nodejs 2>/dev/null || true
            \$SUDO dnf module install -y nodejs:20/common 2>/dev/null || \$SUDO dnf install -y nodejs npm 2>/dev/null || true
        fi
    fi
    if ! command -v npm &>/dev/null || [ ! -f web/dashboard-react/package.json ]; then
        echo 'ERROR: npm or web/dashboard-react missing — cannot build embedded dashboard' >&2
        exit 1
    fi
    cd web/dashboard-react
    if [ -f package-lock.json ]; then
        npm ci --legacy-peer-deps
    else
        npm install
    fi
    npm run build
    cd ../..
fi
if [ ! -f daemon/dashboard/static-react/index.html ]; then
    echo 'ERROR: daemon/dashboard/static-react/index.html missing after npm build' >&2
    exit 1
fi
echo '✅ static-react ready for go:embed'

export VERSION='${HYPERSDK_VERSION}'
export COMMIT='${HYPERSDK_COMMIT}'
export BUILD_TIME='${HYPERSDK_BUILD_TIME}'
export GO_VERSION=\$(go version 2>/dev/null | awk '{print \$3}' || echo unknown)
mkdir -p build
if [ -f Makefile ]; then
    echo "🔨 make build (Go \${GO_VERSION})..."
    make build
else
    echo 'ERROR: Makefile missing in deploy tree' >&2
    exit 1
fi
count=\$(find build -maxdepth 1 -type f 2>/dev/null | wc -l | tr -d ' ')
if [ "\${count:-0}" -lt 1 ]; then
    echo 'ERROR: make build produced no files in build/' >&2
    ls -la build/ 2>/dev/null || true
    exit 1
fi
echo "✅ Go binaries in \$(pwd)/build (\${count} files):"
ls -1 build/ 2>/dev/null | sed 's/^/  /' || true
EOF
}

# System + toolchain deps for remote package builds (same family as deploy-remote full install).
hypersdk_package_install_build_deps() {
    cat <<'EOF'
set -euo pipefail
SUDO=""
[ "$(id -u)" -ne 0 ] && command -v sudo &>/dev/null && SUDO=sudo
if command -v dnf &>/dev/null; then
  if ! dnf repolist --enabled 2>/dev/null | grep -qiE '^(crb|powertools)[[:space:]]'; then
    $SUDO dnf install -y dnf-command\(config-manager\) 2>/dev/null || true
    $SUDO dnf config-manager --set-enabled crb 2>/dev/null || $SUDO dnf config-manager --set-enabled powertools 2>/dev/null || true
  fi
  PKG="$SUDO dnf -y install"
elif command -v apt-get &>/dev/null; then
  $SUDO apt-get update -qq
  PKG="$SUDO apt-get -y install"
else
  echo "No dnf/apt package manager" >&2
  exit 1
fi
$PKG golang gcc git make curl tar rsync 2>&1 | tail -6
command -v go &>/dev/null || { echo "go missing after install" >&2; exit 1; }
if ! command -v npm &>/dev/null; then
  command -v dnf &>/dev/null && $PKG nodejs npm 2>&1 | tail -3 || $PKG nodejs 2>&1 | tail -3 || true
  command -v apt-get &>/dev/null && $PKG nodejs npm 2>&1 | tail -3 || true
fi
echo "build deps: go npm OK"
EOF
}

# Build Go + dashboard in parallel (local install only)
hypersdk_build_all_parallel() {
    local repo_dir="$1" build_dir="$2" goos="${3:-}" goarch="${4:-}"
    local go_ok=1 dash_ok=1

    hypersdk_spinner_start "Building Go binaries + dashboard in parallel"
    (
        hypersdk_build_go_binaries "$repo_dir" "$build_dir" "$goos" "$goarch" true
    ) &
    local pid_go=$!
  (
        hypersdk_build_dashboard "$repo_dir"
    ) &
    local pid_dash=$!

    wait "$pid_go" || go_ok=0
    wait "$pid_dash" || dash_ok=0
    hypersdk_spinner_stop 0

    [ "$go_ok" -eq 1 ] || hypersdk_warn "Some Go binaries failed to build"
    [ "$dash_ok" -eq 1 ] || hypersdk_warn "Dashboard build failed or skipped"
    [ "$go_ok" -eq 1 ]
}

# ── Last deploy target (no secrets) ───────────────────────────────────────────
hypersdk_deploy_state_file() { deploy_ui_deploy_state_file "$1"; }
hypersdk_save_deploy_last() {
    deploy_ui_save_deploy_last "$1" "$2" "$3" "$4" "${HYPERSDK_VERSION:-}" "${HYPERSDK_COMMIT:-}"
}
hypersdk_load_deploy_last() { deploy_ui_load_deploy_last "$1"; }

# Open dashboard/API port on firewalld, ufw, or raw iptables (EL sysconfig).
hypersdk_open_dashboard_port() {
    local port="${DEPLOY_UI_PORT:-5080}" opened=false iptables_cmd=""

    if command -v firewall-cmd &>/dev/null && ${SUDO:-} firewall-cmd --state &>/dev/null 2>&1; then
        ${SUDO:-} firewall-cmd --permanent --add-port="${port}/tcp" 2>/dev/null || true
        ${SUDO:-} firewall-cmd --reload 2>/dev/null || true
        echo "  ✅ firewalld: ${port}/tcp (dashboard/API)"
        opened=true
    fi

    if command -v ufw &>/dev/null && ${SUDO:-} ufw status 2>/dev/null | grep -q 'Status: active'; then
        ${SUDO:-} ufw allow "${port}/tcp" 2>/dev/null || true
        echo "  ✅ ufw: ${port}/tcp (dashboard/API)"
        opened=true
    fi

    if command -v iptables &>/dev/null; then
        iptables_cmd=iptables
    elif command -v iptables-nft &>/dev/null; then
        iptables_cmd=iptables-nft
    fi
    if [ -n "$iptables_cmd" ]; then
        ${SUDO:-} "$iptables_cmd" -C INPUT -p tcp --dport "$port" -j ACCEPT 2>/dev/null || \
            ${SUDO:-} "$iptables_cmd" -I INPUT -p tcp --dport "$port" -j ACCEPT 2>/dev/null || true
        if [ -f /etc/sysconfig/iptables ] && ! grep -q "dport ${port}" /etc/sysconfig/iptables 2>/dev/null; then
            ${SUDO:-} sed -i "/^-A INPUT -j REJECT/i -A INPUT -p tcp -m tcp --dport ${port} -j ACCEPT" \
                /etc/sysconfig/iptables 2>/dev/null || true
        fi
        if ${SUDO:-} "$iptables_cmd" -C INPUT -p tcp --dport "$port" -j ACCEPT 2>/dev/null; then
            echo "  ✅ iptables: ${port}/tcp (dashboard/API)"
            opened=true
        fi
    fi

    if ! $opened; then
        echo "  ⚠️  No active firewall manager detected — ensure ${port}/tcp is reachable"
    fi
}

hypersdk_open_dashboard_port_script() {
    cat <<'FWSCRIPT'
port=5080
opened=false
if command -v firewall-cmd &>/dev/null && $SUDO firewall-cmd --state &>/dev/null 2>&1; then
    $SUDO firewall-cmd --permanent --add-port=${port}/tcp 2>/dev/null || true
    $SUDO firewall-cmd --reload 2>/dev/null || true
    echo "  ✅ firewalld: ${port}/tcp (dashboard/API)"
    opened=true
fi
if command -v ufw &>/dev/null && $SUDO ufw status 2>/dev/null | grep -q 'Status: active'; then
    $SUDO ufw allow ${port}/tcp 2>/dev/null || true
    echo "  ✅ ufw: ${port}/tcp (dashboard/API)"
    opened=true
fi
iptables_cmd=""
command -v iptables &>/dev/null && iptables_cmd=iptables
command -v iptables-nft &>/dev/null && iptables_cmd=iptables-nft
if [ -n "$iptables_cmd" ]; then
    $SUDO $iptables_cmd -C INPUT -p tcp --dport $port -j ACCEPT 2>/dev/null || \
        $SUDO $iptables_cmd -I INPUT -p tcp --dport $port -j ACCEPT 2>/dev/null || true
    if [ -f /etc/sysconfig/iptables ] && ! grep -q "dport ${port}" /etc/sysconfig/iptables 2>/dev/null; then
        $SUDO sed -i "/^-A INPUT -j REJECT/i -A INPUT -p tcp -m tcp --dport ${port} -j ACCEPT" /etc/sysconfig/iptables 2>/dev/null || true
    fi
    if $SUDO $iptables_cmd -C INPUT -p tcp --dport $port -j ACCEPT 2>/dev/null; then
        echo "  ✅ iptables: ${port}/tcp (dashboard/API)"
        opened=true
    fi
fi
if [ "$opened" = false ]; then
    echo "  ⚠️  No active firewall manager detected — ensure ${port}/tcp is reachable"
fi
FWSCRIPT
}

# ── Smoke tests (run on target machine via shell, not SSH) ──────────────────────
hypersdk_smoke_tests_script() {
    cat <<'SMOKE'
pass=0
fail=0
test_endpoint() {
    local label="$1" url="$2" expect="$3"
    body=$(mktemp)
    code=$(curl -sLk -o "$body" -w '%{http_code}' --max-time 5 "$url" 2>/dev/null)
    if [ "$code" = "$expect" ]; then
        printf '  ✅ %-28s %s -> %s\n' "$label" "$url" "$code"
        pass=$((pass + 1))
    else
        printf '  ❌ %-28s %s -> %s (expected %s)\n' "$label" "$url" "$code" "$expect"
        fail=$((fail + 1))
    fi
    rm -f "$body"
}
echo ''
echo '  🩺 HyperSDK endpoints ──'
test_endpoint 'Health'       'https://localhost:5080/api/v1/health'          200
test_endpoint 'Status'       'https://localhost:5080/api/v1/status'          200
test_endpoint 'Dashboard'    'https://localhost:5080/web/dashboard/'         200
test_endpoint 'Providers'    'https://localhost:5080/api/providers/list'     200
test_endpoint 'Libvirt VMs'  'https://localhost:5080/api/v1/libvirt/domains' 200
echo ''
echo "  Results: ${pass} passed, ${fail} failed"
SMOKE
}
