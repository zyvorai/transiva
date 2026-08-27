# h2kvm Systemd Integration

This directory contains systemd unit files for deploying h2kvm as a system daemon.

## Overview

The h2kvm daemon watches a directory for VM conversion jobs (Artifact Manifest files) and automatically processes them. It supports:

- **Default instance**: Single daemon for general use
- **Named instances**: Multiple daemons with different configurations (e.g., vsphere, aws, azure)
- **Resource limits**: Memory and CPU constraints via systemd
- **Security hardening**: Restricted permissions and capabilities
- **Auto-restart**: Automatic recovery from failures
- **Libvirt integration**: Automatic VM registration after conversion

## Files

- `h2kvm.service` - Default daemon instance
- `h2kvm@.service` - Template for named instances
- `h2kvm.target` - Target to manage all instances
- `h2kvm.conf.example` - Default configuration
- `h2kvm-vsphere.conf.example` - vSphere-specific configuration
- `h2kvm-aws.conf.example` - AWS-specific configuration

## Installation

### Prerequisites

1. **h2kvm binary**: Install to `/usr/local/bin/h2kvm`
2. **System user**: Create dedicated user for the daemon
3. **Directories**: Create required directories with correct permissions

### Step 1: Create System User

```bash
# Create h2kvm user and group
sudo useradd --system --no-create-home --shell /usr/sbin/nologin h2kvm

# Add h2kvm to kvm and libvirt groups (if using libvirt)
sudo usermod -aG kvm,libvirt h2kvm
```

### Step 2: Install h2kvm Binary

```bash
# Copy binary (adjust path as needed)
sudo cp /path/to/h2kvm /usr/local/bin/
sudo chmod 755 /usr/local/bin/h2kvm
sudo chown root:root /usr/local/bin/h2kvm

# Verify installation
h2kvm --version
```

### Step 3: Create Directories

```bash
# Create base directories
sudo mkdir -p /var/lib/h2kvm/{queue,output}
sudo mkdir -p /var/log/h2kvm
sudo mkdir -p /var/cache/h2kvm
sudo mkdir -p /etc/h2kvm

# Set ownership
sudo chown -R h2kvm:h2kvm /var/lib/h2kvm
sudo chown -R h2kvm:h2kvm /var/log/h2kvm
sudo chown -R h2kvm:h2kvm /var/cache/h2kvm

# Set permissions
sudo chmod 755 /var/lib/h2kvm
sudo chmod 755 /var/lib/h2kvm/queue
sudo chmod 755 /var/lib/h2kvm/output
sudo chmod 755 /var/log/h2kvm
sudo chmod 755 /var/cache/h2kvm
```

### Step 4: Install Systemd Units

```bash
# Copy unit files
sudo cp systemd/h2kvm.service /etc/systemd/system/
sudo cp systemd/h2kvm@.service /etc/systemd/system/
sudo cp systemd/h2kvm.target /etc/systemd/system/

# Set permissions
sudo chmod 644 /etc/systemd/system/h2kvm.service
sudo chmod 644 /etc/systemd/system/h2kvm@.service
sudo chmod 644 /etc/systemd/system/h2kvm.target

# Reload systemd
sudo systemctl daemon-reload
```

### Step 5: Configure Daemon (Optional)

```bash
# Copy example configuration
sudo cp systemd/h2kvm.conf.example /etc/h2kvm/h2kvm.conf

# Edit configuration
sudo vi /etc/h2kvm/h2kvm.conf

# Set permissions
sudo chmod 640 /etc/h2kvm/h2kvm.conf
sudo chown root:h2kvm /etc/h2kvm/h2kvm.conf
```

### Step 6: Start Daemon

```bash
# Enable and start default instance
sudo systemctl enable h2kvm.service
sudo systemctl start h2kvm.service

# Check status
sudo systemctl status h2kvm.service

# View logs
sudo journalctl -u h2kvm.service -f
```

## Named Instances

Deploy multiple daemons with different configurations for different cloud providers.

### Example: vSphere Instance

```bash
# Create instance directories
sudo mkdir -p /var/lib/h2kvm/vsphere/{queue,output}
sudo chown -R h2kvm:h2kvm /var/lib/h2kvm/vsphere

# Copy instance configuration
sudo cp systemd/h2kvm-vsphere.conf.example /etc/h2kvm/h2kvm-vsphere.conf
sudo vi /etc/h2kvm/h2kvm-vsphere.conf

# Start instance
sudo systemctl enable h2kvm@vsphere.service
sudo systemctl start h2kvm@vsphere.service

# Check status
sudo systemctl status h2kvm@vsphere.service
```

### Example: Multiple Instances

```bash
# Start multiple instances
sudo systemctl start h2kvm@vsphere.service
sudo systemctl start h2kvm@aws.service
sudo systemctl start h2kvm@azure.service

# Check all instances
sudo systemctl status h2kvm@*.service

# Manage all instances via target
sudo systemctl start h2kvm.target
sudo systemctl stop h2kvm.target
```

## Usage with HyperSDK

### CLI (hyperexport)

```bash
# Export with daemon mode (auto-detect)
hyperexport --vm "Ubuntu-Server" \
  --output /tmp/export \
  --manifest \
  --pipeline \
  --h2kvm-daemon

# Export with specific instance
hyperexport --vm "Ubuntu-Server" \
  --output /tmp/export \
  --manifest \
  --pipeline \
  --h2kvm-daemon \
  --h2kvm-instance vsphere \
  --h2kvm-watch-dir /var/lib/h2kvm/vsphere/queue \
  --h2kvm-output-dir /var/lib/h2kvm/vsphere/output
```

### Interactive TUI

```bash
# Launch interactive mode
hyperexport -i

# Select daemon mode in configuration step
# Choose instance if multiple are available
```

### Web Dashboard

Submit jobs via the web dashboard with daemon mode enabled:

```json
{
  "name": "Ubuntu Server Migration",
  "vm_path": "/DC1/vm/ubuntu-server",
  "output_dir": "/tmp/export",
  "format": "ova",
  "options": {
    "enable_pipeline": true,
    "h2kvm_daemon": true,
    "h2kvm_instance": "vsphere",
    "libvirt_integration": true
  }
}
```

### Monitoring with hyperctl

```bash
# Check daemon status
hyperctl daemon -op status

# Check specific instance
hyperctl daemon -op status -instance vsphere

# List all instances
hyperctl daemon -op list
```

## Management

### Start/Stop Services

```bash
# Default instance
sudo systemctl start h2kvm.service
sudo systemctl stop h2kvm.service
sudo systemctl restart h2kvm.service

# Named instance
sudo systemctl start h2kvm@vsphere.service
sudo systemctl stop h2kvm@vsphere.service
sudo systemctl restart h2kvm@vsphere.service

# All instances
sudo systemctl start h2kvm.target
sudo systemctl stop h2kvm.target
```

### Enable/Disable Auto-Start

```bash
# Enable on boot
sudo systemctl enable h2kvm.service
sudo systemctl enable h2kvm@vsphere.service

# Disable on boot
sudo systemctl disable h2kvm.service
sudo systemctl disable h2kvm@vsphere.service
```

### View Logs

```bash
# Default instance
sudo journalctl -u h2kvm.service -f

# Named instance
sudo journalctl -u h2kvm@vsphere.service -f

# All instances
sudo journalctl -u 'h2kvm*' -f

# Last 100 lines
sudo journalctl -u h2kvm.service -n 100

# Since yesterday
sudo journalctl -u h2kvm.service --since yesterday

# With priority (errors only)
sudo journalctl -u h2kvm.service -p err
```

### Check Status

```bash
# Detailed status
sudo systemctl status h2kvm.service

# Check if active
systemctl is-active h2kvm.service

# Check if enabled
systemctl is-enabled h2kvm.service

# Show properties
systemctl show h2kvm.service
```

## Resource Management

### Memory Limits

The service files include memory limits to prevent runaway processes:

- `MemoryMax=4G` - Hard limit (daemon will be killed if exceeded)
- `MemoryHigh=3G` - Soft limit (daemon will be throttled)

Adjust in `/etc/systemd/system/h2kvm.service`:

```ini
[Service]
MemoryMax=8G
MemoryHigh=6G
```

Then reload:

```bash
sudo systemctl daemon-reload
sudo systemctl restart h2kvm.service
```

### CPU Limits

The service files include CPU quota:

- `CPUQuota=200%` - Use up to 2 CPU cores

Adjust as needed:

```ini
[Service]
CPUQuota=400%  # 4 cores
```

### Check Resource Usage

```bash
# Current resource usage
systemctl status h2kvm.service | grep -E 'Memory|CPU'

# Detailed cgroup stats
systemd-cgtop -m
```

## Security

The service files include security hardening:

- Runs as non-root `h2kvm` user
- Private `/tmp` directory
- Read-only filesystem (except specified paths)
- Restricted system calls
- No new privileges
- Minimal capabilities

### AppArmor/SELinux

If using AppArmor or SELinux, you may need to create profiles:

```bash
# Check for denials
sudo ausearch -m avc -ts recent

# Generate policy (SELinux)
sudo ausearch -m avc -ts recent | audit2allow -M h2kvm
sudo semodule -i h2kvm.pp
```

## Troubleshooting

### Daemon Won't Start

```bash
# Check service status
sudo systemctl status h2kvm.service

# Check logs for errors
sudo journalctl -u h2kvm.service -n 50

# Verify binary exists and is executable
ls -l /usr/local/bin/h2kvm

# Check permissions on directories
ls -ld /var/lib/h2kvm
```

### Permission Denied

```bash
# Verify h2kvm user ownership
sudo chown -R h2kvm:h2kvm /var/lib/h2kvm
sudo chown -R h2kvm:h2kvm /var/log/h2kvm

# Check group membership (for libvirt)
groups h2kvm
# Should show: h2kvm kvm libvirt

# Add to groups if missing
sudo usermod -aG kvm,libvirt h2kvm
```

### Jobs Not Processing

```bash
# Check watch directory
ls -la /var/lib/h2kvm/queue/

# Verify daemon is watching correct directory
systemctl show h2kvm.service | grep WATCH_DIR

# Check for errors in logs
sudo journalctl -u h2kvm.service -p err

# Test manually
echo '{"test": true}' | sudo -u h2kvm tee /var/lib/h2kvm/queue/test.json
```

### High Resource Usage

```bash
# Check current usage
systemd-cgtop -m | grep h2kvm

# Reduce concurrent conversions in config
sudo vi /etc/h2kvm/h2kvm.conf
# Set: MAX_CONCURRENT=1

# Restart daemon
sudo systemctl restart h2kvm.service
```

## Uninstallation

```bash
# Stop and disable services
sudo systemctl stop h2kvm.service
sudo systemctl disable h2kvm.service
sudo systemctl stop h2kvm@*.service

# Remove unit files
sudo rm /etc/systemd/system/h2kvm.service
sudo rm /etc/systemd/system/h2kvm@.service
sudo rm /etc/systemd/system/h2kvm.target

# Reload systemd
sudo systemctl daemon-reload

# Remove binary
sudo rm /usr/local/bin/h2kvm

# Remove directories (CAUTION: This deletes all data)
sudo rm -rf /var/lib/h2kvm
sudo rm -rf /var/log/h2kvm
sudo rm -rf /var/cache/h2kvm
sudo rm -rf /etc/h2kvm

# Remove user
sudo userdel h2kvm
```

## Examples

### Example 1: Development Setup

```bash
# Single instance for development
sudo systemctl start h2kvm.service

# Submit test job
hyperexport --vm "test-vm" \
  --output /tmp/test \
  --manifest \
  --pipeline \
  --h2kvm-daemon
```

### Example 2: Production Multi-Cloud

```bash
# Start instances for each cloud provider
sudo systemctl start h2kvm@vsphere.service
sudo systemctl start h2kvm@aws.service
sudo systemctl start h2kvm@azure.service

# Route jobs to appropriate instance
hyperexport --vm "/DC1/vm/web01" \
  --output /exports/vsphere \
  --manifest \
  --pipeline \
  --h2kvm-daemon \
  --h2kvm-instance vsphere
```

### Example 3: Batch Processing

```bash
# Start daemon
sudo systemctl start h2kvm.service

# Batch export multiple VMs
for vm in $(hyperctl list -filter prod | grep -v '^#' | awk '{print $2}'); do
  hyperexport --vm "$vm" \
    --output /batch-exports \
    --manifest \
    --pipeline \
    --h2kvm-daemon &
done

# Monitor progress
watch -n 5 'ls -lh /var/lib/h2kvm/queue/ /var/lib/h2kvm/output/'
```

## See Also

- [SYSTEMD_DAEMON_INTEGRATION.md](../SYSTEMD_DAEMON_INTEGRATION.md) - Integration architecture
- [PIPELINE_INTEGRATION.md](../PIPELINE_INTEGRATION.md) - Pipeline details
- h2kvm documentation: https://github.com/zyvorai/h2kvm
- systemd documentation: https://www.freedesktop.org/software/systemd/man/
