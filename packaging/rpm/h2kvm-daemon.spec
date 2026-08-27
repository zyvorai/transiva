Name:           h2kvm-daemon
Version:        1.0.0
Release:        1%{?dist}
Summary:        Systemd daemon for h2kvm VM conversion

License:        Apache-2.0
URL:            https://github.com/zyvorai/h2kvm
Source0:        %{name}-%{version}.tar.gz

# This package is the renamed successor to hyper2kvm-daemon; Provides/Obsoletes
# let existing installs upgrade in place and keep any package/capability
# lookups against the old name working.
Provides:       hyper2kvm-daemon = %{version}-%{release}
Obsoletes:      hyper2kvm-daemon < %{version}-%{release}

BuildArch:      noarch
BuildRequires:  systemd-rpm-macros
Requires:       systemd
Requires:       qemu-img
Requires:       libvirt-daemon
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description
Systemd service units and configuration for deploying h2kvm as a system
daemon. Provides queue-based VM conversion with support for multiple instances,
resource management, and security hardening.

Features:
- Systemd service units (default and templated instances)
- Configuration templates for different cloud providers
- Security hardening (non-root user, resource limits, system call filtering)
- Auto-restart on failure
- Libvirt integration

%prep
%setup -q

%build
# Nothing to build - systemd units and configs only

%install
# Create directories
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_sysconfdir}/h2kvm
install -d %{buildroot}%{_sharedstatedir}/h2kvm/{queue,output}
install -d %{buildroot}%{_localstatedir}/log/h2kvm
install -d %{buildroot}%{_localstatedir}/cache/h2kvm
install -d %{buildroot}%{_docdir}/%{name}

# Install systemd unit files
install -m 0644 systemd/h2kvm.service %{buildroot}%{_unitdir}/
install -m 0644 systemd/h2kvm@.service %{buildroot}%{_unitdir}/
install -m 0644 systemd/h2kvm.target %{buildroot}%{_unitdir}/

# Install configuration examples
install -m 0640 systemd/h2kvm.conf.example %{buildroot}%{_sysconfdir}/h2kvm/
install -m 0640 systemd/h2kvm-vsphere.conf.example %{buildroot}%{_sysconfdir}/h2kvm/
install -m 0640 systemd/h2kvm-aws.conf.example %{buildroot}%{_sysconfdir}/h2kvm/

# Install documentation
install -m 0644 systemd/README.md %{buildroot}%{_docdir}/%{name}/

%pre
# Create h2kvm system user and group
getent group h2kvm >/dev/null || groupadd -r h2kvm
getent passwd h2kvm >/dev/null || \
    useradd -r -g h2kvm -d /var/lib/h2kvm -s /sbin/nologin \
    -c "h2kvm daemon user" h2kvm

# Add to kvm and libvirt groups if they exist
if getent group kvm >/dev/null; then
    usermod -aG kvm h2kvm 2>/dev/null || true
fi
if getent group libvirt >/dev/null; then
    usermod -aG libvirt h2kvm 2>/dev/null || true
fi

exit 0

%post
# %systemd_post also creates the hyper2kvm.{service,target} / hyper2kvm@.service
# alias symlinks declared via Alias= in each unit's [Install] section.
%systemd_post h2kvm.service h2kvm@.service h2kvm.target

# Set ownership on directories
chown -R h2kvm:h2kvm %{_sharedstatedir}/h2kvm
chown -R h2kvm:h2kvm %{_localstatedir}/log/h2kvm
chown -R h2kvm:h2kvm %{_localstatedir}/cache/h2kvm

# Set permissions
chmod 755 %{_sharedstatedir}/h2kvm
chmod 755 %{_sharedstatedir}/h2kvm/queue
chmod 755 %{_sharedstatedir}/h2kvm/output
chmod 755 %{_localstatedir}/log/h2kvm
chmod 755 %{_localstatedir}/cache/h2kvm

cat <<EOF

h2kvm-daemon has been installed successfully!

Next steps:
  1. Copy configuration:
     sudo cp /etc/h2kvm/h2kvm.conf.example /etc/h2kvm/h2kvm.conf
     sudo vi /etc/h2kvm/h2kvm.conf

  2. Enable and start the service:
     sudo systemctl enable --now h2kvm.service

  3. Check status:
     sudo systemctl status h2kvm.service

  4. View logs:
     sudo journalctl -u h2kvm.service -f

Documentation: /usr/share/doc/h2kvm-daemon/

EOF

%preun
%systemd_preun h2kvm.service h2kvm@*.service h2kvm.target

%postun
%systemd_postun_with_restart h2kvm.service

# Only remove directories on complete uninstall (not upgrade)
if [ $1 -eq 0 ]; then
    # Remove user and group
    userdel h2kvm 2>/dev/null || true
    groupdel h2kvm 2>/dev/null || true
fi

%files
%license LICENSE
%doc %{_docdir}/%{name}/README.md

# Systemd units
%{_unitdir}/h2kvm.service
%{_unitdir}/h2kvm@.service
%{_unitdir}/h2kvm.target

# Configuration
%dir %{_sysconfdir}/h2kvm
%config(noreplace) %attr(640,root,h2kvm) %{_sysconfdir}/h2kvm/h2kvm.conf.example
%config(noreplace) %attr(640,root,h2kvm) %{_sysconfdir}/h2kvm/h2kvm-vsphere.conf.example
%config(noreplace) %attr(640,root,h2kvm) %{_sysconfdir}/h2kvm/h2kvm-aws.conf.example

# Runtime directories
%dir %attr(755,h2kvm,h2kvm) %{_sharedstatedir}/h2kvm
%dir %attr(755,h2kvm,h2kvm) %{_sharedstatedir}/h2kvm/queue
%dir %attr(755,h2kvm,h2kvm) %{_sharedstatedir}/h2kvm/output
%dir %attr(755,h2kvm,h2kvm) %{_localstatedir}/log/h2kvm
%dir %attr(755,h2kvm,h2kvm) %{_localstatedir}/cache/h2kvm

%changelog
* Thu Aug 27 2026 HyperSDK Team <noreply@anthropic.com> - 1.0.0-1
- Renamed from hyper2kvm-daemon to h2kvm-daemon, following the upstream
  hyper2kvm -> h2kvm project rename (github.com/zyvorai/h2kvm). Provides/
  Obsoletes hyper2kvm-daemon for in-place upgrades; unit files keep their
  pre-rename names available via systemd Alias=.
* Sat Jan 24 2026 HyperSDK Team <noreply@anthropic.com> - 1.0.0-1
- Initial RPM release
- Systemd service units for hyper2kvm daemon
- Configuration templates for vSphere and AWS
- Security hardening with non-root user
- Resource limits and auto-restart
- Multi-instance support
- Libvirt integration
