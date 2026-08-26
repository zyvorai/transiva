#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# Client-side runtime dependencies for HyperSDK (libvirt/KVM host).
set -euo pipefail
SUDO=""
[ "$(id -u)" -ne 0 ] && command -v sudo &>/dev/null && SUDO=sudo
echo "== HyperSDK client dependencies =="
if command -v dnf &>/dev/null; then
  $SUDO dnf install -y qemu-kvm qemu-img libvirt virt-install openssh-clients rsync curl tar 2>&1 | tail -8
  $SUDO dnf install -y guestfs-tools libguestfs-tools edk2-ovmf 2>&1 | tail -3 || true
elif command -v apt-get &>/dev/null; then
  $SUDO apt-get update -qq
  $SUDO apt-get install -y qemu-kvm qemu-utils libvirt-daemon-system virtinst guestfs-tools ovmf 2>&1 | tail -8
else
  echo "Install libvirt/qemu manually for your distro" >&2
  exit 1
fi
$SUDO systemctl enable --now libvirtd 2>/dev/null || true
echo "Done. Configure vCenter in config.yaml then ./bin/hypervisord"
