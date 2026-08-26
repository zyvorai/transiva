Name:           hyper2kvm-daemon
Version:        1.0.0
Release:        1%{?dist}
Summary:        Systemd daemon for hyper2kvm VM conversion

License:        Apache-2.0
URL:            https://github.com/ssahani/hyper2kvm
Source0:        %{name}-%{version}.tar.gz

BuildArch:      noarch
BuildRequires:  systemd-rpm-macros
Requires:       systemd
Requires:       qemu-img
Requires:       libvirt-daemon
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description
Systemd service units and configuration for deploying hyper2kvm as a system
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
install -d %{buildroot}%{_sysconfdir}/hyper2kvm
install -d %{buildroot}%{_sharedstatedir}/hyper2kvm/{queue,output}
install -d %{buildroot}%{_localstatedir}/log/hyper2kvm
install -d %{buildroot}%{_localstatedir}/cache/hyper2kvm
install -d %{buildroot}%{_docdir}/%{name}

# Install systemd unit files
install -m 0644 systemd/hyper2kvm.service %{buildroot}%{_unitdir}/
install -m 0644 systemd/hyper2kvm@.service %{buildroot}%{_unitdir}/
install -m 0644 systemd/hyper2kvm.target %{buildroot}%{_unitdir}/

# Install configuration examples
install -m 0640 systemd/hyper2kvm.conf.example %{buildroot}%{_sysconfdir}/hyper2kvm/
install -m 0640 systemd/hyper2kvm-vsphere.conf.example %{buildroot}%{_sysconfdir}/hyper2kvm/
install -m 0640 systemd/hyper2kvm-aws.conf.example %{buildroot}%{_sysconfdir}/hyper2kvm/

# Install documentation
install -m 0644 systemd/README.md %{buildroot}%{_docdir}/%{name}/

%pre
# Create hyper2kvm system user and group
getent group hyper2kvm >/dev/null || groupadd -r hyper2kvm
getent passwd hyper2kvm >/dev/null || \
    useradd -r -g hyper2kvm -d /var/lib/hyper2kvm -s /sbin/nologin \
    -c "hyper2kvm daemon user" hyper2kvm

# Add to kvm and libvirt groups if they exist
if getent group kvm >/dev/null; then
    usermod -aG kvm hyper2kvm 2>/dev/null || true
fi
if getent group libvirt >/dev/null; then
    usermod -aG libvirt hyper2kvm 2>/dev/null || true
fi

exit 0

%post
%systemd_post hyper2kvm.service hyper2kvm@.service hyper2kvm.target

# Set ownership on directories
chown -R hyper2kvm:hyper2kvm %{_sharedstatedir}/hyper2kvm
chown -R hyper2kvm:hyper2kvm %{_localstatedir}/log/hyper2kvm
chown -R hyper2kvm:hyper2kvm %{_localstatedir}/cache/hyper2kvm

# Set permissions
chmod 755 %{_sharedstatedir}/hyper2kvm
chmod 755 %{_sharedstatedir}/hyper2kvm/queue
chmod 755 %{_sharedstatedir}/hyper2kvm/output
chmod 755 %{_localstatedir}/log/hyper2kvm
chmod 755 %{_localstatedir}/cache/hyper2kvm

cat <<EOF

hyper2kvm-daemon has been installed successfully!

Next steps:
  1. Copy configuration:
     sudo cp /etc/hyper2kvm/hyper2kvm.conf.example /etc/hyper2kvm/hyper2kvm.conf
     sudo vi /etc/hyper2kvm/hyper2kvm.conf

  2. Enable and start the service:
     sudo systemctl enable --now hyper2kvm.service

  3. Check status:
     sudo systemctl status hyper2kvm.service

  4. View logs:
     sudo journalctl -u hyper2kvm.service -f

Documentation: /usr/share/doc/hyper2kvm-daemon/

EOF

%preun
%systemd_preun hyper2kvm.service hyper2kvm@*.service hyper2kvm.target

%postun
%systemd_postun_with_restart hyper2kvm.service

# Only remove directories on complete uninstall (not upgrade)
if [ $1 -eq 0 ]; then
    # Remove user and group
    userdel hyper2kvm 2>/dev/null || true
    groupdel hyper2kvm 2>/dev/null || true
fi

%files
%license LICENSE
%doc %{_docdir}/%{name}/README.md

# Systemd units
%{_unitdir}/hyper2kvm.service
%{_unitdir}/hyper2kvm@.service
%{_unitdir}/hyper2kvm.target

# Configuration
%dir %{_sysconfdir}/hyper2kvm
%config(noreplace) %attr(640,root,hyper2kvm) %{_sysconfdir}/hyper2kvm/hyper2kvm.conf.example
%config(noreplace) %attr(640,root,hyper2kvm) %{_sysconfdir}/hyper2kvm/hyper2kvm-vsphere.conf.example
%config(noreplace) %attr(640,root,hyper2kvm) %{_sysconfdir}/hyper2kvm/hyper2kvm-aws.conf.example

# Runtime directories
%dir %attr(755,hyper2kvm,hyper2kvm) %{_sharedstatedir}/hyper2kvm
%dir %attr(755,hyper2kvm,hyper2kvm) %{_sharedstatedir}/hyper2kvm/queue
%dir %attr(755,hyper2kvm,hyper2kvm) %{_sharedstatedir}/hyper2kvm/output
%dir %attr(755,hyper2kvm,hyper2kvm) %{_localstatedir}/log/hyper2kvm
%dir %attr(755,hyper2kvm,hyper2kvm) %{_localstatedir}/cache/hyper2kvm

%changelog
* Sat Jan 24 2026 HyperSDK Team <noreply@anthropic.com> - 1.0.0-1
- Initial RPM release
- Systemd service units for hyper2kvm daemon
- Configuration templates for vSphere and AWS
- Security hardening with non-root user
- Resource limits and auto-restart
- Multi-instance support
- Libvirt integration
