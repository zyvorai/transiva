# h2kvm-daemon Packaging

This directory contains packaging files for distributing h2kvm systemd daemon integration.

## Available Packages

### RPM (Red Hat, CentOS, Fedora, Rocky, AlmaLinux)

RPM packages for Red Hat-based distributions.

**Directory**: `rpm/`

**Quick Start**:
```bash
cd rpm
./build.sh
sudo rpm -ivh ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm
```

**Documentation**: [rpm/README.md](rpm/README.md)

**Supported Distributions**:
- Red Hat Enterprise Linux (RHEL) 8, 9
- CentOS Stream 8, 9
- Fedora 38, 39, 40
- Rocky Linux 8, 9
- AlmaLinux 8, 9
- Oracle Linux 8, 9

## Package Contents

All packages install:

### Systemd Service Units
- `h2kvm.service` - Default daemon instance
- `h2kvm@.service` - Template for named instances (vsphere, aws, etc.)
- `h2kvm.target` - Target to manage all instances

### Configuration Templates
- `h2kvm.conf.example` - Default configuration
- `h2kvm-vsphere.conf.example` - vSphere-optimized settings
- `h2kvm-aws.conf.example` - AWS-optimized settings

### Runtime Directories
- `/var/lib/h2kvm/queue` - Watch directory for manifest files
- `/var/lib/h2kvm/output` - Output directory for converted VMs
- `/var/log/h2kvm` - Log directory
- `/var/cache/h2kvm` - Cache directory

### System User
- User: `h2kvm` (system account)
- Group: `h2kvm`
- Additional groups: `kvm`, `libvirt` (if available)

### Documentation
- Installation guide
- Configuration reference
- Troubleshooting guide

## Dependencies

All packages require:
- **systemd** - Service management
- **qemu-img** / **qemu-utils** - VM image conversion
- **libvirt-daemon** - KVM/libvirt integration (optional but recommended)

## Installation Flow

### 1. Install Package

**RPM**:
```bash
sudo rpm -ivh h2kvm-daemon-1.0.0-1.el9.noarch.rpm
```

### 2. Configure

```bash
# Copy example configuration
sudo cp /etc/h2kvm/h2kvm.conf.example /etc/h2kvm/h2kvm.conf

# Edit as needed
sudo vi /etc/h2kvm/h2kvm.conf
```

### 3. Start Service

```bash
# Enable and start
sudo systemctl enable --now h2kvm.service

# Check status
sudo systemctl status h2kvm.service
```

### 4. Verify

```bash
# Check directories
ls -ld /var/lib/h2kvm/{queue,output}

# Check user
id h2kvm

# View logs
sudo journalctl -u h2kvm.service -f
```

## Usage with HyperSDK

After installing the package, use with HyperSDK:

### CLI

```bash
hyperexport --vm "Ubuntu-Server" \
  --output /tmp/export \
  --manifest \
  --pipeline \
  --h2kvm-daemon
```

### Interactive TUI

```bash
hyperexport -i
# Select "Enable daemon mode" in configuration
```

### Web Dashboard

Submit jobs via the web dashboard with daemon mode enabled.

### Monitor with hyperctl

```bash
# Check daemon status
hyperctl daemon -op status

# List all instances
hyperctl daemon -op list
```

## Multi-Instance Deployment

Deploy multiple daemon instances for different cloud providers:

### vSphere Instance

```bash
# Install package (if not already installed)
sudo rpm -ivh h2kvm-daemon-*.rpm

# Configure vSphere instance
sudo cp /etc/h2kvm/h2kvm-vsphere.conf.example /etc/h2kvm/h2kvm-vsphere.conf
sudo vi /etc/h2kvm/h2kvm-vsphere.conf

# Start instance
sudo systemctl enable --now h2kvm@vsphere.service

# Check status
sudo systemctl status h2kvm@vsphere.service
```

### AWS Instance

```bash
# Configure AWS instance
sudo cp /etc/h2kvm/h2kvm-aws.conf.example /etc/h2kvm/h2kvm-aws.conf
sudo vi /etc/h2kvm/h2kvm-aws.conf

# Start instance
sudo systemctl enable --now h2kvm@aws.service
```

### Manage All Instances

```bash
# Start all instances
sudo systemctl start h2kvm.target

# Check all instances
sudo systemctl status 'h2kvm@*'
```

## Uninstallation

### RPM

```bash
# Stop services
sudo systemctl stop h2kvm.service 'h2kvm@*'

# Remove package
sudo rpm -e h2kvm-daemon

# Optionally remove data (careful!)
sudo rm -rf /var/lib/h2kvm
sudo rm -rf /var/log/h2kvm
sudo rm -rf /etc/h2kvm
sudo userdel h2kvm
```

## Building from Source

### Prerequisites

Install build tools for your distribution:

**Red Hat / CentOS / Fedora**:
```bash
sudo dnf install rpm-build rpmdevtools
```

### Build RPM

```bash
cd packaging/rpm
./build.sh

# Custom version
./build.sh --version 1.1.0 --release 2

# Clean build
./build.sh --clean
```

Built packages will be in:
- RPM: `~/rpmbuild/RPMS/noarch/`

## Distribution

### YUM/DNF Repository

Create a local repository:

```bash
# Create repo directory
mkdir -p ~/h2kvm-repo/el9/x86_64

# Copy RPM
cp ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm ~/h2kvm-repo/el9/x86_64/

# Create metadata
createrepo ~/h2kvm-repo/el9/x86_64

# Serve via HTTP
cd ~/h2kvm-repo
python3 -m http.server 8080
```

Configure clients:

```bash
# Create repo file
sudo tee /etc/yum.repos.d/h2kvm.repo << EOF
[h2kvm]
name=h2kvm Repository
baseurl=http://your-server:8080/el9/x86_64
enabled=1
gpgcheck=0
EOF

# Install
sudo dnf install h2kvm-daemon
```

## Security

### Package Verification

**RPM**:
```bash
# Verify package integrity
rpm -V h2kvm-daemon

# Check signatures (if GPG signed)
rpm -K ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm
```

### Service Security

The daemon runs with security hardening:
- **Non-root user**: Runs as `h2kvm` system user
- **Resource limits**: Memory (4GB), CPU (200%)
- **Filesystem restrictions**: Read-only system, specific write paths
- **System call filtering**: Only allowed syscalls
- **No new privileges**: Cannot escalate privileges
- **Private /tmp**: Isolated temporary directory

## Troubleshooting

### Package Won't Install

**RPM**:
```bash
# Check dependencies
rpm -qpR h2kvm-daemon-*.rpm

# Force install (not recommended)
sudo rpm -ivh --nodeps h2kvm-daemon-*.rpm
```

### Service Won't Start

```bash
# Check if h2kvm binary exists
which h2kvm

# Note: This package only installs systemd units.
# The h2kvm binary must be installed separately.

# Check logs
sudo journalctl -u h2kvm.service -n 50
```

### Permission Issues

```bash
# Fix directory ownership
sudo chown -R h2kvm:h2kvm /var/lib/h2kvm
sudo chown -R h2kvm:h2kvm /var/log/h2kvm

# Check user groups
groups h2kvm

# Add to required groups
sudo usermod -aG kvm,libvirt h2kvm
```

## Support

For issues or questions:
- GitHub Issues: https://github.com/zyvorai/transiva/issues
- Documentation: See [systemd/README.md](../systemd/README.md)

## License

Apache-2.0

## Contributing

Contributions welcome! Please:
1. Test packages on target distributions
2. Update changelog in spec file
3. Follow distribution packaging guidelines
4. Submit pull request

## See Also

- [systemd/README.md](../systemd/README.md) - Deployment guide
- [SYSTEMD_DAEMON_INTEGRATION.md](../SYSTEMD_DAEMON_INTEGRATION.md) - Architecture
- [rpm/README.md](rpm/README.md) - RPM-specific documentation
