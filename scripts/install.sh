#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

# Installation script for hypervisord

set -e

echo "Installing hypervisord..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (use sudo)"
  exit 1
fi

# Create directories
echo "Creating directories..."
mkdir -p /usr/local/bin
mkdir -p /etc/hypervisord
mkdir -p /var/lib/hypervisord
mkdir -p /var/log/hypervisord

# Copy binaries
echo "Installing binaries..."
cp build/hypervisord /usr/local/bin/
cp build/hyperctl /usr/local/bin/
chmod +x /usr/local/bin/hypervisord
chmod +x /usr/local/bin/hyperctl

# Copy config if it doesn't exist
if [ ! -f /etc/hypervisord/config.yaml ]; then
  echo "Installing default config..."
  cp config.yaml.example /etc/hypervisord/config.yaml
  echo "IMPORTANT: Edit /etc/hypervisord/config.yaml with your vCenter credentials"
fi

# Install systemd service
echo "Installing systemd service..."
cp systemd/hypervisord.service /etc/systemd/system/
systemctl daemon-reload

echo "Installation complete!"
echo ""
echo "Next steps:"
echo "  1. Edit /etc/hypervisord/config.yaml with your vCenter credentials"
echo "  2. Start the daemon: sudo systemctl start hypervisord"
echo "  3. Enable auto-start: sudo systemctl enable hypervisord"
echo "  4. Check status: sudo systemctl status hypervisord"
echo "  5. View logs: sudo journalctl -u hypervisord -f"
echo ""
echo "Query the daemon with: hyperctl status"
