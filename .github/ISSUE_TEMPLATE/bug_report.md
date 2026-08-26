---
name: Bug Report
about: Report a Community Edition bug (not for sales or enterprise quotes)
title: '[BUG] '
labels: bug
assignees: ''
---

> **Enterprise / sales / SLAs / VMware exit programs** → [zyvor.dev/contact](https://zyvor.dev/contact?utm_source=github&utm_medium=transiva) or [sales@zyvor.dev](mailto:sales@zyvor.dev) — do not use this form.

## 🐛 Bug Description

A clear and concise description of what the bug is.

## 🔄 Steps to Reproduce

1. Go to '...'
2. Run command '...'
3. See error

## ✅ Expected Behavior

What you expected to happen.

## ❌ Actual Behavior

What actually happened.

## 📊 Environment

**HyperSDK Version:**
```bash
$ hypervisord --version
v2.0.0
```

**Operating System:**
- OS: [e.g., Fedora 40, Ubuntu 22.04]
- Architecture: [e.g., x86_64, arm64]

**Deployment Method:**
- [ ] Docker/Podman
- [ ] Kubernetes/Helm
- [ ] Native binary
- [ ] From source

**Provider:**
- [ ] vSphere
- [ ] AWS
- [ ] Azure
- [ ] GCP
- [ ] Other: ___________

## 📝 Logs

<details>
<summary>Daemon Logs</summary>

```
Paste relevant logs here
```
</details>

<details>
<summary>Job Details</summary>

```bash
$ hyperctl query --id <job-id>
# Paste output here
```
</details>

## 🖼️ Screenshots

If applicable, add screenshots to help explain your problem.

## 📎 Additional Context

Add any other context about the problem here.

## ✔️ Possible Solution

If you have ideas on how to fix this, please share.

## 🔗 Related Issues

- Related to #___
