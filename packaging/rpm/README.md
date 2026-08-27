# RPM Packaging for h2kvm-daemon

This directory contains RPM packaging files for the h2kvm systemd daemon.

## Files

- **h2kvm-daemon.spec** - RPM spec file
- **build.sh** - Automated build script
- **README.md** - This file

## Prerequisites

### Red Hat / CentOS / Fedora

```bash
# Install build tools
sudo yum install rpm-build rpmdevtools

# Or on Fedora
sudo dnf install rpm-build rpmdevtools
```

### Rocky Linux / AlmaLinux

```bash
sudo dnf install rpm-build rpmdevtools
```

## Building the RPM

### Quick Build

```bash
# Build with default version (1.0.0)
cd packaging/rpm
./build.sh

# Built RPMs will be in: ~/rpmbuild/RPMS/noarch/
```

### Custom Version

```bash
# Build specific version
./build.sh --version 1.1.0 --release 2
```

### Clean Build

```bash
# Remove previous build artifacts first
./build.sh --clean
```

## Manual Build Process

If you prefer to build manually:

### 1. Setup RPM Build Tree

```bash
rpmdev-setuptree
```

This creates:
```
~/rpmbuild/
├── BUILD/
├── RPMS/
├── SOURCES/
├── SPECS/
└── SRPMS/
```

### 2. Create Source Tarball

```bash
# From project root
cd /path/to/transiva

# Create tarball with systemd files
tar czf ~/rpmbuild/SOURCES/h2kvm-daemon-1.0.0.tar.gz \
    --transform 's,^,h2kvm-daemon-1.0.0/,' \
    systemd/*.service \
    systemd/*.target \
    systemd/*.example \
    systemd/README.md \
    SYSTEMD_DAEMON_INTEGRATION.md \
    LICENSE
```

### 3. Copy Spec File

```bash
cp packaging/rpm/h2kvm-daemon.spec ~/rpmbuild/SPECS/
```

### 4. Build RPM

```bash
# Build both binary and source RPMs
rpmbuild -ba ~/rpmbuild/SPECS/h2kvm-daemon.spec

# Or build only binary RPM
rpmbuild -bb ~/rpmbuild/SPECS/h2kvm-daemon.spec

# Or build only source RPM
rpmbuild -bs ~/rpmbuild/SPECS/h2kvm-daemon.spec
```

## Installation

### Install RPM

```bash
# Install
sudo rpm -ivh ~/rpmbuild/RPMS/noarch/h2kvm-daemon-1.0.0-1.el*.noarch.rpm

# Or upgrade
sudo rpm -Uvh ~/rpmbuild/RPMS/noarch/h2kvm-daemon-1.0.0-1.el*.noarch.rpm
```

### Using DNF/YUM

```bash
# Install with dependencies
sudo dnf install ~/rpmbuild/RPMS/noarch/h2kvm-daemon-1.0.0-1.el*.noarch.rpm

# Or with yum
sudo yum localinstall ~/rpmbuild/RPMS/noarch/h2kvm-daemon-1.0.0-1.el*.noarch.rpm
```

## Post-Installation

After installing the RPM:

### 1. Configure the Daemon

```bash
# Copy example configuration
sudo cp /etc/h2kvm/h2kvm.conf.example /etc/h2kvm/h2kvm.conf

# Edit configuration
sudo vi /etc/h2kvm/h2kvm.conf
```

### 2. Start the Service

```bash
# Enable and start
sudo systemctl enable --now h2kvm.service

# Check status
sudo systemctl status h2kvm.service

# View logs
sudo journalctl -u h2kvm.service -f
```

### 3. Verify Installation

```bash
# Check if h2kvm user was created
id h2kvm

# Check directories
ls -ld /var/lib/h2kvm
ls -ld /var/lib/h2kvm/queue
ls -ld /var/lib/h2kvm/output

# Check service files
systemctl list-unit-files | grep h2kvm
```

## Package Information

### Query Package Details

```bash
# Show package info
rpm -qi h2kvm-daemon

# List all files
rpm -ql h2kvm-daemon

# Show documentation files
rpm -qd h2kvm-daemon

# Show configuration files
rpm -qc h2kvm-daemon

# Show dependencies
rpm -qR h2kvm-daemon
```

### Verify Package

```bash
# Verify all files in package
rpm -V h2kvm-daemon

# Show pre/post install scripts
rpm -q --scripts h2kvm-daemon
```

## Uninstallation

### Remove Package

```bash
# Remove package (keeps config files)
sudo rpm -e h2kvm-daemon

# Note: User, group, and data directories are NOT removed on uninstall
# Remove manually if needed:
sudo userdel h2kvm
sudo groupdel h2kvm
sudo rm -rf /var/lib/h2kvm
sudo rm -rf /var/log/h2kvm
sudo rm -rf /var/cache/h2kvm
sudo rm -rf /etc/h2kvm
```

## Package Contents

The RPM installs:

### Systemd Units
- `/usr/lib/systemd/system/h2kvm.service` - Default service
- `/usr/lib/systemd/system/h2kvm@.service` - Template service
- `/usr/lib/systemd/system/h2kvm.target` - Service target

### Configuration
- `/etc/h2kvm/h2kvm.conf.example` - Default config
- `/etc/h2kvm/h2kvm-vsphere.conf.example` - vSphere config
- `/etc/h2kvm/h2kvm-aws.conf.example` - AWS config

### Directories
- `/var/lib/h2kvm/queue` - Watch directory
- `/var/lib/h2kvm/output` - Output directory
- `/var/log/h2kvm` - Log directory
- `/var/cache/h2kvm` - Cache directory

### Documentation
- `/usr/share/doc/h2kvm-daemon/README.md`
- `/usr/share/doc/h2kvm-daemon/SYSTEMD_DAEMON_INTEGRATION.md`

### System User
- User: `h2kvm`
- Group: `h2kvm`
- Additional groups: `kvm`, `libvirt` (if they exist)

## Building for Different Distributions

### Fedora

```bash
# Build for Fedora 39
./build.sh --version 1.0.0 --release 1.fc39
```

### RHEL / CentOS

```bash
# Build for RHEL 9
./build.sh --version 1.0.0 --release 1.el9

# Build for RHEL 8
./build.sh --version 1.0.0 --release 1.el8
```

### Rocky Linux / AlmaLinux

```bash
# Build for Rocky 9
./build.sh --version 1.0.0 --release 1.el9
```

## Creating a YUM Repository

To distribute the RPM via a YUM repository:

```bash
# 1. Create repository directory
mkdir -p ~/yum-repo/el9/x86_64

# 2. Copy RPM to repository
cp ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm ~/yum-repo/el9/x86_64/

# 3. Create repository metadata
createrepo ~/yum-repo/el9/x86_64

# 4. Serve via HTTP (example with Python)
cd ~/yum-repo
python3 -m http.server 8080

# 5. Configure client to use repository
sudo tee /etc/yum.repos.d/h2kvm.repo << EOF
[h2kvm]
name=h2kvm Repository
baseurl=http://your-server:8080/el9/x86_64
enabled=1
gpgcheck=0
EOF

# 6. Install from repository
sudo dnf install h2kvm-daemon
```

## Troubleshooting

### Build Fails - Missing Dependencies

```bash
# Install missing build dependencies
sudo yum-builddep ~/rpmbuild/SPECS/h2kvm-daemon.spec
```

### RPM Build Error - Bad Source

```bash
# Clean and rebuild tarball
./build.sh --clean
```

### Installation Fails - Conflicts

```bash
# Check for conflicting packages
rpm -qa | grep h2kvm

# Force reinstall
sudo rpm -e h2kvm-daemon --nodeps
sudo rpm -ivh --force ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm
```

### Service Won't Start After Install

```bash
# Check if h2kvm binary exists
which h2kvm

# Note: This package only installs systemd units
# You must install the h2kvm binary separately
```

## Development

### Testing the Spec File

```bash
# Check spec file syntax
rpmlint ~/rpmbuild/SPECS/h2kvm-daemon.spec

# Check built RPM
rpmlint ~/rpmbuild/RPMS/noarch/h2kvm-daemon-*.rpm

# Test installation in mock (clean room)
mock -r fedora-39-x86_64 ~/rpmbuild/SRPMS/h2kvm-daemon-*.src.rpm
```

### Modifying the Spec File

After modifying `h2kvm-daemon.spec`:

1. Update the `%changelog` section
2. Increment `Release` or `Version` as appropriate
3. Rebuild: `./build.sh --clean`

## See Also

- [systemd/README.md](../../systemd/README.md) - Systemd deployment guide
- [SYSTEMD_DAEMON_INTEGRATION.md](../../SYSTEMD_DAEMON_INTEGRATION.md) - Integration architecture
- [RPM Packaging Guide](https://rpm-packaging-guide.github.io/) - Official RPM docs
- [Fedora Packaging Guidelines](https://docs.fedoraproject.org/en-US/packaging-guidelines/)
