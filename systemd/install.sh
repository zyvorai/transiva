#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# hyper2kvm Systemd Service Installer
#
# This script installs and configures the hyper2kvm systemd daemon.
#
# Usage:
#   sudo ./install.sh [options]
#
# Options:
#   --binary PATH       Path to hyper2kvm binary (default: detect in PATH)
#   --user USER         Service user (default: hyper2kvm)
#   --instance NAME     Install named instance (default: install default service)
#   --uninstall         Uninstall hyper2kvm systemd services
#   --help              Show this help message

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
HYPER2KVM_USER="hyper2kvm"
HYPER2KVM_BINARY=""
INSTANCE_NAME=""
UNINSTALL=false

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

# Function to check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root"
        exit 1
    fi
}

# Function to find hyper2kvm binary
find_binary() {
    if [[ -n "$HYPER2KVM_BINARY" ]]; then
        if [[ ! -x "$HYPER2KVM_BINARY" ]]; then
            error "Binary not found or not executable: $HYPER2KVM_BINARY"
            exit 1
        fi
        return
    fi

    # Check common locations
    for path in /usr/local/bin/hyper2kvm /usr/bin/hyper2kvm ./hyper2kvm; do
        if [[ -x "$path" ]]; then
            HYPER2KVM_BINARY="$path"
            info "Found hyper2kvm binary: $HYPER2KVM_BINARY"
            return
        fi
    done

    # Check PATH
    if command -v hyper2kvm &> /dev/null; then
        HYPER2KVM_BINARY=$(command -v hyper2kvm)
        info "Found hyper2kvm binary in PATH: $HYPER2KVM_BINARY"
        return
    fi

    error "hyper2kvm binary not found. Use --binary to specify location."
    exit 1
}

# Function to create system user
create_user() {
    if id "$HYPER2KVM_USER" &>/dev/null; then
        info "User '$HYPER2KVM_USER' already exists"
    else
        info "Creating system user '$HYPER2KVM_USER'..."
        useradd --system --no-create-home --shell /usr/sbin/nologin "$HYPER2KVM_USER"
    fi

    # Add to kvm and libvirt groups if they exist
    if getent group kvm &>/dev/null; then
        usermod -aG kvm "$HYPER2KVM_USER" 2>/dev/null || true
        info "Added '$HYPER2KVM_USER' to 'kvm' group"
    fi

    if getent group libvirt &>/dev/null; then
        usermod -aG libvirt "$HYPER2KVM_USER" 2>/dev/null || true
        info "Added '$HYPER2KVM_USER' to 'libvirt' group"
    fi
}

# Function to create directories
create_directories() {
    local base_dir="/var/lib/hyper2kvm"

    if [[ -n "$INSTANCE_NAME" ]]; then
        base_dir="/var/lib/hyper2kvm/$INSTANCE_NAME"
    fi

    info "Creating directories..."

    mkdir -p "$base_dir"/{queue,output}
    mkdir -p /var/log/hyper2kvm
    mkdir -p /var/cache/hyper2kvm
    mkdir -p /etc/hyper2kvm

    chown -R "$HYPER2KVM_USER:$HYPER2KVM_USER" /var/lib/hyper2kvm
    chown -R "$HYPER2KVM_USER:$HYPER2KVM_USER" /var/log/hyper2kvm
    chown -R "$HYPER2KVM_USER:$HYPER2KVM_USER" /var/cache/hyper2kvm

    chmod 755 /var/lib/hyper2kvm
    chmod 755 "$base_dir"
    chmod 755 "$base_dir"/queue
    chmod 755 "$base_dir"/output
    chmod 755 /var/log/hyper2kvm
    chmod 755 /var/cache/hyper2kvm

    info "Directories created successfully"
}

# Function to install binary
install_binary() {
    local target="/usr/local/bin/hyper2kvm"

    if [[ "$HYPER2KVM_BINARY" == "$target" ]]; then
        info "Binary already installed at $target"
        return
    fi

    info "Installing hyper2kvm binary to $target..."
    cp "$HYPER2KVM_BINARY" "$target"
    chmod 755 "$target"
    chown root:root "$target"

    info "Binary installed successfully"
}

# Function to install systemd units
install_systemd_units() {
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    info "Installing systemd unit files..."

    if [[ -n "$INSTANCE_NAME" ]]; then
        # Install template service
        cp "$script_dir/hyper2kvm@.service" /etc/systemd/system/
        chmod 644 /etc/systemd/system/hyper2kvm@.service
        info "Installed hyper2kvm@.service"
    else
        # Install default service
        cp "$script_dir/hyper2kvm.service" /etc/systemd/system/
        chmod 644 /etc/systemd/system/hyper2kvm.service
        info "Installed hyper2kvm.service"
    fi

    # Install target
    if [[ ! -f /etc/systemd/system/hyper2kvm.target ]]; then
        cp "$script_dir/hyper2kvm.target" /etc/systemd/system/
        chmod 644 /etc/systemd/system/hyper2kvm.target
        info "Installed hyper2kvm.target"
    fi

    systemctl daemon-reload
    info "Systemd units installed successfully"
}

# Function to install configuration
install_config() {
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local config_file="/etc/hyper2kvm/hyper2kvm.conf"

    if [[ -n "$INSTANCE_NAME" ]]; then
        config_file="/etc/hyper2kvm/hyper2kvm-$INSTANCE_NAME.conf"
    fi

    if [[ -f "$config_file" ]]; then
        warn "Configuration file already exists: $config_file"
        warn "Skipping configuration installation. Remove manually if you want to reinstall."
        return
    fi

    info "Installing configuration..."

    if [[ -n "$INSTANCE_NAME" ]]; then
        # Try instance-specific example first
        if [[ -f "$script_dir/hyper2kvm-$INSTANCE_NAME.conf.example" ]]; then
            cp "$script_dir/hyper2kvm-$INSTANCE_NAME.conf.example" "$config_file"
        else
            cp "$script_dir/hyper2kvm.conf.example" "$config_file"
        fi
    else
        cp "$script_dir/hyper2kvm.conf.example" "$config_file"
    fi

    chmod 640 "$config_file"
    chown root:$HYPER2KVM_USER "$config_file"

    info "Configuration installed: $config_file"
    warn "Please review and customize $config_file"
}

# Function to uninstall
uninstall() {
    info "Uninstalling hyper2kvm systemd services..."

    # Stop all services
    systemctl stop hyper2kvm.service 2>/dev/null || true
    systemctl stop 'hyper2kvm@*.service' 2>/dev/null || true
    systemctl stop hyper2kvm.target 2>/dev/null || true

    # Disable all services
    systemctl disable hyper2kvm.service 2>/dev/null || true
    systemctl disable hyper2kvm.target 2>/dev/null || true

    # Remove unit files
    rm -f /etc/systemd/system/hyper2kvm.service
    rm -f /etc/systemd/system/hyper2kvm@.service
    rm -f /etc/systemd/system/hyper2kvm.target

    systemctl daemon-reload

    info "Systemd services uninstalled"
    warn "Binary, configuration, and data directories were NOT removed"
    warn "To remove completely:"
    warn "  sudo rm /usr/local/bin/hyper2kvm"
    warn "  sudo rm -rf /var/lib/hyper2kvm"
    warn "  sudo rm -rf /var/log/hyper2kvm"
    warn "  sudo rm -rf /var/cache/hyper2kvm"
    warn "  sudo rm -rf /etc/hyper2kvm"
    warn "  sudo userdel $HYPER2KVM_USER"
}

# Function to show usage
show_usage() {
    cat << EOF
hyper2kvm Systemd Service Installer

Usage:
  sudo $0 [options]

Options:
  --binary PATH       Path to hyper2kvm binary (default: detect in PATH)
  --user USER         Service user (default: hyper2kvm)
  --instance NAME     Install named instance (e.g., vsphere, aws)
  --uninstall         Uninstall hyper2kvm systemd services
  --help              Show this help message

Examples:
  # Install default service
  sudo $0

  # Install with custom binary path
  sudo $0 --binary /opt/hyper2kvm/hyper2kvm

  # Install named instance
  sudo $0 --instance vsphere

  # Uninstall
  sudo $0 --uninstall

After installation:
  # Enable and start service
  sudo systemctl enable hyper2kvm.service
  sudo systemctl start hyper2kvm.service

  # For named instances
  sudo systemctl enable hyper2kvm@vsphere.service
  sudo systemctl start hyper2kvm@vsphere.service

  # Check status
  sudo systemctl status hyper2kvm.service

  # View logs
  sudo journalctl -u hyper2kvm.service -f

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --binary)
            HYPER2KVM_BINARY="$2"
            shift 2
            ;;
        --user)
            HYPER2KVM_USER="$2"
            shift 2
            ;;
        --instance)
            INSTANCE_NAME="$2"
            shift 2
            ;;
        --uninstall)
            UNINSTALL=true
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

# Main installation flow
main() {
    check_root

    if $UNINSTALL; then
        uninstall
        exit 0
    fi

    info "Installing hyper2kvm systemd service..."
    echo

    find_binary
    create_user
    create_directories
    install_binary
    install_systemd_units
    install_config

    echo
    info "Installation complete!"
    echo
    info "Next steps:"

    if [[ -n "$INSTANCE_NAME" ]]; then
        echo "  1. Review configuration: /etc/hyper2kvm/hyper2kvm-$INSTANCE_NAME.conf"
        echo "  2. Enable service:      sudo systemctl enable hyper2kvm@$INSTANCE_NAME.service"
        echo "  3. Start service:       sudo systemctl start hyper2kvm@$INSTANCE_NAME.service"
        echo "  4. Check status:        sudo systemctl status hyper2kvm@$INSTANCE_NAME.service"
        echo "  5. View logs:           sudo journalctl -u hyper2kvm@$INSTANCE_NAME.service -f"
    else
        echo "  1. Review configuration: /etc/hyper2kvm/hyper2kvm.conf"
        echo "  2. Enable service:      sudo systemctl enable hyper2kvm.service"
        echo "  3. Start service:       sudo systemctl start hyper2kvm.service"
        echo "  4. Check status:        sudo systemctl status hyper2kvm.service"
        echo "  5. View logs:           sudo journalctl -u hyper2kvm.service -f"
    fi

    echo
    info "Test with HyperSDK:"
    echo "  hyperexport --vm test-vm --output /tmp/test --manifest --pipeline --hyper2kvm-daemon"
}

main
