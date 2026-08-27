Name:           transiva
Version:        2.2.1
Release:        1%{?dist}
Summary:        Zyvor Transiva - Enterprise workload mobility & migration control plane

License:        Apache-2.0
URL:            https://github.com/zyvorai/transiva
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.21
BuildRequires:  systemd-rpm-macros
BuildRequires:  git

Requires:       systemd

%description
Transiva is a Zyvor AI Labs workload mobility and migration control plane. It provides discovery, inventory, export, and orchestration for VM migrations with an interactive CLI and REST daemon. Key features:
- Interactive CLI (transivaexport, compat: hyperexport) for manual exports with terminal UI
- Background daemon (transivad, compat: hypervisord) with comprehensive REST API
- Web dashboard for browser-based VM management and console access
- Integration with h2kvm for offline conversion to QCOW2
- Job scheduling, webhooks, and monitoring (Prometheus metrics)
- API-only mode (--disable-web) for security-conscious deployments
- YAML/JSON configuration support for batch operations

%prep
%setup -q

%build
# Build all binaries (produce transiva* and hyper* compatibility names)
go build -v -o transivaexport ./cmd/hyperexport
go build -v -o hyperexport ./cmd/hyperexport
go build -v -o transivad ./cmd/hypervisord
go build -v -o hypervisord ./cmd/hypervisord

%install
# Install binaries (both new and compat)
install -Dm755 transivaexport %{buildroot}%{_bindir}/transivaexport
install -Dm755 hyperexport %{buildroot}%{_bindir}/hyperexport
install -Dm755 transivad %{buildroot}%{_bindir}/transivad
install -Dm755 hypervisord %{buildroot}%{_bindir}/hypervisord

# Install systemd services (new and compat)
install -Dm644 systemd/transivad.service %{buildroot}%{_unitdir}/transivad.service || true
install -Dm644 systemd/hypervisord.service %{buildroot}%{_unitdir}/hypervisord.service || true

# Install configuration
install -Dm644 config.example.yaml %{buildroot}%{_sysconfdir}/transiva/config.yaml

# Install web dashboard (to working directory for daemon access)
install -dm755 %{buildroot}%{_sharedstatedir}/transiva/web/dashboard
install -Dm644 web/dashboard/index.html %{buildroot}%{_sharedstatedir}/transiva/web/dashboard/index.html
install -Dm644 web/dashboard/vm-console.html %{buildroot}%{_sharedstatedir}/transiva/web/dashboard/vm-console.html

# Create data and log directories
install -dm755 %{buildroot}%{_sharedstatedir}/transiva
install -dm755 %{buildroot}%{_localstatedir}/log/transiva

# Install documentation
install -Dm644 README.md %{buildroot}%{_docdir}/%{name}/README.md
install -Dm644 SECURITY.md %{buildroot}%{_docdir}/%{name}/SECURITY.md

%pre
# Create system user for the daemon if it doesn't exist
getent group transiva >/dev/null || groupadd -r transiva
getent passwd transiva >/dev/null || \
    useradd -r -g transiva -d %{_sharedstatedir}/transiva \
    -s /sbin/nologin -c "transiva daemon user" transiva
exit 0

%post
%systemd_post transivad.service || true
%systemd_post hypervisord.service || true

# Set ownership
chown -R transiva:transiva %{_sharedstatedir}/transiva
chown -R transiva:transiva %{_localstatedir}/log/transiva

if [ $1 -eq 1 ]; then
    # First install
    echo "Transiva installed successfully!"
    echo "Edit /etc/transiva/config.yaml with your vCenter credentials (compat path)"
    echo "Start the daemon: systemctl start transivad"
    echo "Enable auto-start: systemctl enable transivad"
fi

%preun
%systemd_preun transivad.service || true
%systemd_preun hypervisord.service || true

%postun
%systemd_postun_with_restart transivad.service || true
%systemd_postun_with_restart hypervisord.service || true

if [ $1 -eq 0 ]; then
    # Uninstall
    userdel transiva 2>/dev/null || true
    groupdel transiva 2>/dev/null || true
fi

%files
%license LICENSE
%doc README.md
%doc %{_docdir}/%{name}/SECURITY.md
# New and compatibility binaries
%{_bindir}/transivaexport
%{_bindir}/hyperexport
%{_bindir}/transivad
%{_bindir}/hypervisord
%{_unitdir}/transivad.service
%{_unitdir}/hypervisord.service
%dir %{_sysconfdir}/transiva
%config(noreplace) %{_sysconfdir}/transiva/config.yaml
%attr(0755,transiva,transiva) %dir %{_sharedstatedir}/transiva
%attr(0755,transiva,transiva) %dir %{_sharedstatedir}/transiva/web
%attr(0755,transiva,transiva) %dir %{_sharedstatedir}/transiva/web/dashboard
%attr(0644,transiva,transiva) %{_sharedstatedir}/transiva/web/dashboard/index.html
%attr(0644,transiva,transiva) %{_sharedstatedir}/transiva/web/dashboard/vm-console.html
%attr(0755,transiva,transiva) %dir %{_localstatedir}/log/transiva

%changelog
* Tue Jan 20 2026 ZyvorAI Labs Private Limited <ssahani@zyvor.dev> - 0.2.0-1
- Phase 2 release - Production ready
- Added 51+ REST API endpoints for complete VM management
- Added web dashboard (index.html, vm-console.html)
- Added libvirt/KVM integration (domains, snapshots, networks, volumes)
- Added console access features (VNC, serial, screenshots)
- Added job scheduling and webhook support
- Added Prometheus metrics integration
- Added --disable-web flag for API-only deployments
- Security enhancements: TLS validation, path traversal protection, timing-safe auth
- Updated systemd service to run as transiva user (not root)
- Added comprehensive documentation (CHANGELOG.md, SECURITY.md, API_ENDPOINTS.md)
- Fixed systemd service paths and permissions
- Improved error handling and logging throughout

* Sat Jan 17 2026 ZyvorAI Labs Private Limited <ssahani@zyvor.dev> - 0.1.0-1
- Initial package release
- Multi-cloud provider architecture (vSphere production-ready)
- Beautiful terminal UI with pterm
- REST JSON API daemon (6 core endpoints)
- Configuration file support (YAML)
- Systemd integration
- Parallel downloads with worker pools
- Resumable downloads with retry logic
- Interactive VM selection
- Batch job processing
- Comprehensive logging
