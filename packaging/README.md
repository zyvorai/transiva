# hyper2kvm-daemon Packaging

This directory contains packaging files for distributing hyper2kvm systemd daemon integration.

## Available Packages

### RPM (Red Hat, CentOS, Fedora, Rocky, AlmaLinux)

RPM packages for Red Hat-based distributions.

**Directory**: `rpm/`

**Quick Start**:
```bash
cd rpm
./build.sh
sudo rpm -ivh ~/rpmbuild/RPMS/noarch/hyper2kvm-daemon-*.rpm
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
- `hyper2kvm.service` - Default daemon instance
- `hyper2kvm@.service` - Template for named instances (vsphere, aws, etc.)
- `hyper2kvm.target` - Target to manage all instances

### Configuration Templates
- `hyper2kvm.conf.example` - Default configuration
- `hyper2kvm-vsphere.conf.example` - vSphere-optimized settings
- `hyper2kvm-aws.conf.example` - AWS-optimized settings

### Runtime Directories
- `/var/lib/hyper2kvm/queue` - Watch directory for manifest files
- `/var/lib/hyper2kvm/output` - Output directory for converted VMs
- `/var/log/hyper2kvm` - Log directory
- `/var/cache/hyper2kvm` - Cache directory

### System User
- User: `hyper2kvm` (system account)
- Group: `hyper2kvm`
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
sudo rpm -ivh hyper2kvm-daemon-1.0.0-1.el9.noarch.rpm
```

### 2. Configure

```bash
# Copy example configuration
sudo cp /etc/hyper2kvm/hyper2kvm.conf.example /etc/hyper2kvm/hyper2kvm.conf

# Edit as needed
sudo vi /etc/hyper2kvm/hyper2kvm.conf
```

### 3. Start Service

```bash
# Enable and start
sudo systemctl enable --now hyper2kvm.service

# Check status
sudo systemctl status hyper2kvm.service
```

### 4. Verify

```bash
# Check directories
ls -ld /var/lib/hyper2kvm/{queue,output}

# Check user
id hyper2kvm

# View logs
sudo journalctl -u hyper2kvm.service -f
```

## Usage with HyperSDK

After installing the package, use with HyperSDK:

### CLI

```bash
hyperexport --vm "Ubuntu-Server" \
  --output /tmp/export \
  --manifest \
  --pipeline \
  --hyper2kvm-daemon
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
sudo rpm -ivh hyper2kvm-daemon-*.rpm

# Configure vSphere instance
sudo cp /etc/hyper2kvm/hyper2kvm-vsphere.conf.example /etc/hyper2kvm/hyper2kvm-vsphere.conf
sudo vi /etc/hyper2kvm/hyper2kvm-vsphere.conf

# Start instance
sudo systemctl enable --now hyper2kvm@vsphere.service

# Check status
sudo systemctl status hyper2kvm@vsphere.service
```

### AWS Instance

```bash
# Configure AWS instance
sudo cp /etc/hyper2kvm/hyper2kvm-aws.conf.example /etc/hyper2kvm/hyper2kvm-aws.conf
sudo vi /etc/hyper2kvm/hyper2kvm-aws.conf

# Start instance
sudo systemctl enable --now hyper2kvm@aws.service
```

### Manage All Instances

```bash
# Start all instances
sudo systemctl start hyper2kvm.target

# Check all instances
sudo systemctl status 'hyper2kvm@*'
```

## Uninstallation

### RPM

```bash
# Stop services
sudo systemctl stop hyper2kvm.service 'hyper2kvm@*'

# Remove package
sudo rpm -e hyper2kvm-daemon

# Optionally remove data (careful!)
sudo rm -rf /var/lib/hyper2kvm
sudo rm -rf /var/log/hyper2kvm
sudo rm -rf /etc/hyper2kvm
sudo userdel hyper2kvm
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
mkdir -p ~/hyper2kvm-repo/el9/x86_64

# Copy RPM
cp ~/rpmbuild/RPMS/noarch/hyper2kvm-daemon-*.rpm ~/hyper2kvm-repo/el9/x86_64/

# Create metadata
createrepo ~/hyper2kvm-repo/el9/x86_64

# Serve via HTTP
cd ~/hyper2kvm-repo
python3 -m http.server 8080
```

Configure clients:

```bash
# Create repo file
sudo tee /etc/yum.repos.d/hyper2kvm.repo << EOF
[hyper2kvm]
name=hyper2kvm Repository
baseurl=http://your-server:8080/el9/x86_64
enabled=1
gpgcheck=0
EOF

# Install
sudo dnf install hyper2kvm-daemon
```

## Security

### Package Verification

**RPM**:
```bash
# Verify package integrity
rpm -V hyper2kvm-daemon

# Check signatures (if GPG signed)
rpm -K ~/rpmbuild/RPMS/noarch/hyper2kvm-daemon-*.rpm
```

### Service Security

The daemon runs with security hardening:
- **Non-root user**: Runs as `hyper2kvm` system user
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
rpm -qpR hyper2kvm-daemon-*.rpm

# Force install (not recommended)
sudo rpm -ivh --nodeps hyper2kvm-daemon-*.rpm
```

### Service Won't Start

```bash
# Check if hyper2kvm binary exists
which hyper2kvm

# Note: This package only installs systemd units.
# The hyper2kvm binary must be installed separately.

# Check logs
sudo journalctl -u hyper2kvm.service -n 50
```

### Permission Issues

```bash
# Fix directory ownership
sudo chown -R hyper2kvm:hyper2kvm /var/lib/hyper2kvm
sudo chown -R hyper2kvm:hyper2kvm /var/log/hyper2kvm

# Check user groups
groups hyper2kvm

# Add to required groups
sudo usermod -aG kvm,libvirt hyper2kvm
```

## Support

For issues or questions:
- GitHub Issues: https://github.com/ssahani/transiva/issues
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
