# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Currently supported versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | :white_check_mark: |
| 0.1.x   | :x:                |
| < 0.1   | :x:                |

## Reporting a Vulnerability

We take the security of transiva seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please Do NOT:

- Open a public GitHub issue for security vulnerabilities
- Disclose the vulnerability publicly before it has been addressed

### Please DO:

**Report security vulnerabilities to:** ssahani@zyvor.dev

Include the following information in your report:

- Type of vulnerability (e.g., authentication bypass, injection, XSS, etc.)
- Full paths of source file(s) related to the vulnerability
- The location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability, including how an attacker might exploit it

### What to Expect:

- **Initial Response:** Within 48 hours, you will receive an acknowledgment of your report
- **Status Updates:** We will keep you informed about our progress every 5-7 days
- **Disclosure Timeline:** We aim to resolve critical vulnerabilities within 90 days
- **Credit:** If you wish, we will acknowledge your contribution in the security advisory

## Security Best Practices for Users

### Deployment Security

1. **API-Only Mode for Production**
   ```bash
   # Disable web dashboard for reduced attack surface
   ./hypervisord --disable-web
   ```

2. **Enable Authentication**
   ```yaml
   # config.yaml
   security:
     enable_auth: true
     api_key: "your-strong-random-key-here"
   ```

3. **Configure TLS**
   ```yaml
   # Use proper TLS certificates, avoid insecure mode in production
   Insecure: false  # Verify TLS certificates
   ```

4. **Restrict Network Access**
   - Bind to localhost only for local-only access: `DaemonAddr: "localhost:8080"`
   - Use firewall rules to limit access to API port
   - Run behind reverse proxy with TLS termination

5. **Run with Minimal Privileges**
   ```bash
   # Create dedicated user
   sudo useradd -r -s /bin/false hypervisord

   # Run service as non-root user
   sudo systemctl edit hypervisord
   # Add: User=hypervisord
   ```

6. **Protect Configuration Files**
   ```bash
   # Secure config file permissions
   chmod 600 /etc/hypervisord/config.yaml
   chown hypervisord:hypervisord /etc/hypervisord/config.yaml
   ```

7. **Block Private IPs in Webhooks**
   ```yaml
   # config.yaml
   security:
     block_private_ips: true  # Prevent SSRF attacks
   ```

8. **Set Request Size Limits**
   ```yaml
   # config.yaml
   security:
     max_request_size_mb: 10  # Prevent DoS
   ```

9. **Configure Trusted Proxies**
   ```yaml
   # config.yaml (if behind reverse proxy)
   security:
     trusted_proxies:
       - "10.0.0.1"  # Your proxy IP
   ```

### vSphere Credentials

- **Never commit credentials** to version control
- Store credentials in config file with restricted permissions
- Use environment variables for temporary/testing use only
- Consider using HashiCorp Vault or similar for credential management
- Rotate credentials regularly
- Use dedicated service accounts with minimal required permissions

### Systemd Hardening

The included systemd service unit provides security hardening:

```ini
[Service]
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
```

Review and customize based on your environment.

## Known Security Considerations

### 1. VMware Credential Handling

- Credentials are stored in memory during daemon operation
- Consider using credential vaults for production deployments
- Credentials are not logged (marked as `json:"-"` in config)

### 2. Web Dashboard

- Default deployment serves static files without authentication
- Disable in production using `--disable-web` flag
- If needed, place behind authenticated reverse proxy

### 3. Libvirt Access

- Daemon requires access to libvirt socket
- Runs with privileges to manage VMs
- Isolate on dedicated management network

### 4. API Authentication

- Default: authentication disabled for development
- **Always enable authentication in production**
- API keys transmitted in headers (use HTTPS)
- Session tokens have limited lifetime

### 5. Download Operations

- Files downloaded to local filesystem
- Ensure sufficient disk space and proper permissions
- Downloaded files not automatically cleaned up

## Security Fixes in Recent Versions

### Version 0.2.0 (2026-01-20)

- **Fixed:** TLS certificate validation bypass vulnerability
- **Fixed:** Path traversal in file operations
- **Fixed:** Timing attack in API key comparison (now uses constant-time)
- **Added:** Request size limits to prevent DoS
- **Added:** Private IP blocking for webhooks (SSRF protection)
- **Added:** Input sanitization for VM names
- **Added:** Optional web dashboard disable for reduced attack surface

See [SECURITY_FIXES_APPLIED.md](SECURITY_FIXES_APPLIED.md) for detailed technical information.

## Vulnerability Disclosure Policy

When we receive a security vulnerability report, we will:

1. Confirm the problem and determine affected versions
2. Audit code to find similar problems
3. Prepare fixes for all supported versions
4. Release new versions with security patches
5. Publish security advisory with CVE if applicable
6. Credit the reporter (if desired)

## Security Audit

This project has not undergone independent security audit. We welcome security researchers to review the codebase and report any findings.

## Contact

For security-related questions: ssahani@zyvor.dev

For general questions: Open a GitHub issue

---

**Last updated:** 2026-01-20
