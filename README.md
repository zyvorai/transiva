<div align="center">

# Transiva — Community Edition

### Enterprise Workload Mobility & Migration Control Plane

Transiva is a Go control plane that discovers, inventories, and orchestrates workload exports from VMware vSphere and Nutanix AHV, handing artifacts to [hyper2kvm](https://github.com/transiva/hyper2kvm) for conversion and running fleet jobs over REST.

**Apache-2.0** · a Zyvor AI Labs project · day-2 on **[Zeus OS](https://zyvor.dev/zeus-os)**

<br/>

[![CI](https://github.com/transiva/transiva/actions/workflows/ci.yml/badge.svg)](https://github.com/transiva/transiva/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/tag/transiva/transiva?label=release&sort=semver&color=F97316)](https://github.com/transiva/transiva/tags)
[![Go 1.24+](https://img.shields.io/badge/go-1.24+-00ADD8.svg)](https://go.dev/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Stars](https://img.shields.io/github/stars/transiva/transiva?style=social)](https://github.com/transiva/transiva/stargazers)

<br/>

[![Talk to an engineer](https://img.shields.io/badge/Talk_to_an_engineer-F97316?style=for-the-badge)](https://zyvor.dev/schedule?utm_source=github&utm_medium=transiva&utm_campaign=readme_hero)
[![30-day PoC](https://img.shields.io/badge/30--day_PoC-111827?style=for-the-badge)](https://zyvor.dev/poc?utm_source=github&utm_medium=transiva&utm_campaign=readme_hero)
[![Book a Platform demo](https://img.shields.io/badge/Book_a_Platform_demo-22C55E?style=for-the-badge)](https://zyvor.dev/contact?intent=demo&utm_source=github&utm_medium=transiva&utm_campaign=readme_hero)

**[Quick start](#60-second-quick-start)** ·
**[Demos](#see-it-in-action)** ·
**[CE vs Platform](#community-edition-vs-transiva-platform)** ·
**[Nutanix](docs/nutanix.md)** ·
**[OpenAPI](openapi.yaml)** ·
**[Docs](https://zyvor.dev/docs/transiva-platform?utm_source=github&utm_medium=transiva)**

</div>

---

## The renewal trap — escaped with an API

Every hypervisor renewal buries your VMs deeper in someone else’s proprietary API. Getting out usually means a spreadsheet, a maintenance window, and a pile of one-off `ovftool` invocations.

**HyperSDK turns that into a job.**

```text
  vSphere · Nutanix AHV
           │
           ▼
  ┌────────────────────────────────────────┐
  │  Transiva (Community)                  │──►  discover · inventory · export
  │  transivactl  (compat: hyperctl)       │──►  resumable jobs · REST
  │  transivad    (compat: hypervisord)    │──►  artifacts for hyper2kvm
  └────────────────────────────────────────┘
           │
           ▼
  hyper2kvm → GuestKit → KVM / KubeVirt → Zeus OS
```

| | | |
|:---:|:---:|:---:|
| **2** source hypervisors (CE) | **1** workflow for both | **0** guest agents |
| Apache-2.0 | CLI + REST | Offline export — source VM untouched |

**Export with HyperSDK → convert with [hyper2kvm](https://github.com/transiva/hyper2kvm) → assure with [GuestKit](https://github.com/transiva/guestkit) → operate on [Zeus OS](https://zyvor.dev/zeus-os).**

---

## See it in action

<table>
<tr>
<td width="50%" align="center">
<a href="https://www.youtube.com/watch?v=SFNeNn-yvb4">
<img src="https://img.youtube.com/vi/SFNeNn-yvb4/hqdefault.jpg" alt="HyperSDK Platform console tour" width="100%">
<br><b>▶ Platform console tour</b>
</a>
<br><sub>Migrate hub · providers · noVNC — live cluster</sub>
</td>
<td width="50%" align="center">
<a href="https://www.youtube.com/watch?v=9sAl6uhHFQI">
<img src="https://img.youtube.com/vi/9sAl6uhHFQI/hqdefault.jpg" alt="HyperSDK tutorial" width="100%">
<br><b>▶ Full tutorial</b>
</a>
<br><sub>Export → convert → deploy walkthrough</sub>
</td>
</tr>
</table>

<p align="center">
  <a href="https://zyvor.dev/demo?utm_source=github&utm_medium=transiva&utm_campaign=readme_demos"><b>More demos →</b></a>
  &nbsp;·&nbsp;
  Recorded against real deployments — not staged slides
</p>

---

## Why teams start here

| Before HyperSDK | With HyperSDK |
|-----------------|----------------|
| `ovftool` one-offs and tribal scripts | One CLI + job model for vSphere **and** Nutanix |
| Spreadsheet of VMs nobody trusts | `hyperctl list` / `export` against the provider API |
| Export fails mid-transfer — start over | Resumable jobs via `hypervisord` + REST |
| Conversion mutates the live guest | Offline artifacts — source VM untouched until cutover |
| No path to KubeVirt / Zeus OS | Clean handoff into the Zyvor suite |

- **Two sources, one workflow.** vSphere and Nutanix AHV behave identically — same CLI, same jobs, same output layout.
- **Scriptable end to end.** Interactive CLI for one-offs; daemon + REST for fleets.
- **Open by default.** Apache-2.0, no phone-home, no agent in the guest. Source, Docker, RPMs.

---

## 60-second quick start

<details open>
<summary><b>VMware vSphere</b></summary>

```bash
git clone https://github.com/transiva/transiva.git
cd transiva
./scripts/build.sh

cp config.example.yaml config.yaml
# edit config.yaml for your vSphere endpoint
./bin/hyperexport --config config.yaml
```

</details>

<details>
<summary><b>Nutanix AHV</b></summary>

```bash
cp config.example.yaml config.yaml
# configure nutanix.host, credentials, and nutanix.mounts (NFS container paths)

./bin/hypervisord --config config.yaml

hyperctl list --provider nutanix
hyperctl export --provider nutanix --vm web-prod-01 \
  --output /var/lib/transiva/nutanix \
  --mounts 'default-container:/mnt/nutanix/default'
```

Full walkthrough: **[docs/nutanix.md](docs/nutanix.md)**

</details>

<details>
<summary><b>Daemon + REST</b></summary>

```bash
./bin/hypervisord --config config.yaml
./bin/hyperctl status
```

REST surface: [openapi.yaml](openapi.yaml) · containers: [deployments/docker/](deployments/docker/)

</details>

### Binaries

| Binary | Role |
|--------|------|
| `transivaexport` (compat: `hyperexport`) | Interactive CLI exports (vSphere, Nutanix, …) |
| `transivad` (compat: `hypervisord`) | REST API daemon |
| `transivactl` (compat: `hyperctl`) | Job control — `list`, `export`, `info`, `submit` |
| `nutanix-pickup` | Standalone Nutanix discovery/pickup |

> Note: Legacy `hyper*` binaries will continue to be provided as compatibility aliases for at least one major release.

---

## Where this fits: the Zyvor suite

```mermaid
flowchart LR
    A["vSphere"] --> H
    B["Nutanix AHV"] --> H
    H["<b>transiva</b><br/>discover · export"] --> K["hyper2kvm<br/>convert to QCOW2"]
    K --> G["GuestKit<br/>inspect · repair"]
    G --> T["KVM · libvirt · KubeVirt"]
    T --> Z["Zeus OS<br/>day-2"]

    classDef accent fill:#F97316,stroke:#EA580C,color:#fff;
    classDef muted fill:#F3F4F6,stroke:#D1D5DB,color:#111827;
    class H accent;
    class Z muted;
```

| Stage | Tool | What it does |
|---|---|---|
| **Export** | **transiva** *(this repo)* | Discover via provider API, export disks + metadata |
| Convert | [hyper2kvm](https://github.com/transiva/hyper2kvm) | Rewrite to QCOW2 offline, fix drivers before first boot |
| Assure | [GuestKit](https://github.com/transiva/guestkit) | Offline doctor score + Passport before power-on |
| Host | [Machina](https://zyvor.dev/machina) | Bare-metal KVM / libvirt on the hypervisor host |
| Operate | [Zeus OS](https://zyvor.dev/zeus-os) | VMs + containers — KubeVirt lifecycle, GPU, multi-cluster |

Methodology: **Discover → Assess → Convert → Deploy → Operate → Optimize.** This repo covers the first two. [Full hypervisor-exit route →](https://zyvor.dev/hypervisor-exit?utm_source=github&utm_medium=transiva&utm_campaign=readme_suite)

---

## Community Edition vs HyperSDK Platform

**CE proves export. Platform runs the hypervisor-exit program.**

CE is free forever for labs — two sources, CLI, GitHub Issues. If you are moving an estate: no CBT, no waves, no SSO, no named owner on cutover night. **Buy Platform.**

| | **Community Edition** *(this repo)* | **[HyperSDK Platform](https://zyvor.dev/transiva?utm_source=github&utm_medium=transiva&utm_campaign=readme_table)** |
|---|---|---|
| **Who it is for** | Labs · PoC · single-host | Platform / SRE leads · **50–10,000+ VMs** |
| **Sources** | vSphere, Nutanix AHV | **10–11 providers** — Hyper-V, AWS, Azure, GCP, OCI, OpenStack, Proxmox, KubeVirt, … |
| **Interface** | CLI + REST daemon | **Dashboard** · Spotlight · noVNC · SDKs · 200–267+ routes |
| **Incremental** | Full exports only | **CBT** + smart fallback — windows that fit change freeze |
| **Pipeline** | Hand off to hyper2kvm yourself | Wizard · **17 readiness checks** · waves · approvals · blackouts |
| **Deploy targets** | Local output → hyper2kvm | Glance/Nova · libvirt · **KubeVirt** · live libvirt→KubeVirt |
| **Security** | Config-file credentials | Vault · **SSO/OIDC/SAML** · RBAC · audit · air-gap packs |
| **Ops** | Single-node eval | Multi-tenant CP · HA · agents · carbon/chargeback · compliance |
| **Day-2** | Hand off to Zeus OS | Licensed **Zeus OS** suite path |
| **Support** | [GitHub Issues](https://github.com/transiva/transiva/issues) | **SLA** · workshops · hypervisor-exit PS |

### Why teams upgrade

1. Full-export weekends do not survive a multi-wave estate  
2. Security needs SSO, vaulted secrets, and an audit stream  
3. Executives need wave plans — not CLI screenshots  
4. CBT shrinks the cutover window into change freeze  
5. You need Zyvor on the bridge — not Issues at 2 a.m.  

**[Full feature matrix →](docs/ce-vs-enterprise.md)** · [enterprise.md](docs/enterprise.md)

<div align="center">
<br/>

**Bring us your worst estate.** 30-day PoC on your hardware, your workloads.

[![Start a proof of concept](https://img.shields.io/badge/Start_a_proof_of_concept-F97316?style=for-the-badge)](https://zyvor.dev/poc?utm_source=github&utm_medium=transiva&utm_campaign=readme_footer)
[![Book a Platform demo](https://img.shields.io/badge/Book_a_Platform_demo-111827?style=for-the-badge)](https://zyvor.dev/contact?intent=demo&utm_source=github&utm_medium=transiva&utm_campaign=readme_footer)
[![Pricing](https://img.shields.io/badge/Pricing-22C55E?style=for-the-badge)](https://zyvor.dev/pricing?utm_source=github&utm_medium=transiva&utm_campaign=readme_footer)

</div>

---

## Development

```bash
make test
make lint
```

PRs welcome. Security reports → [SECURITY.md](SECURITY.md).

## Support the project

HyperSDK Community Edition is free and open source, maintained by **Susant Sahani** at [Zyvor AI Labs](https://zyvor.dev?utm_source=github&utm_medium=transiva&utm_campaign=readme_support).

If it saved you a licence renewal, a ⭐ helps more people find it.

| | |
|---|---|
| **Production / Platform** | [Talk to an engineer](https://zyvor.dev/schedule?utm_source=github&utm_medium=transiva) · [sales@zyvor.dev](mailto:sales@zyvor.dev) |
| **Community** | [GitHub Issues](https://github.com/transiva/transiva/issues) |
| **General** | [info@zyvor.dev](mailto:info@zyvor.dev) |

## Related open-source repos

| Repo | Role |
|---|---|
| [hyper2kvm](https://github.com/transiva/hyper2kvm) | Convert exported disks → KVM |
| [guestkit](https://github.com/transiva/guestkit) | Offline disk doctor + Passport |
| [netevd](https://github.com/transiva/netevd) | Real-time network event tracking |
| [netctl](https://github.com/transiva/netctl) | Networking from the CLI |

[Browse all open-source →](https://zyvor.dev/about#support-open-source)

## License

Apache License, Version 2.0. Copyright © 2026 Zyvor AI Labs Private Limited.

This repository is **HyperSDK Community Edition** only. Platform and other Zyvor offerings may use different terms. [Licensing →](https://zyvor.dev/docs/licensing?utm_source=github&utm_medium=transiva)

<div align="center">
<sub>Built by <a href="https://zyvor.dev?utm_source=github&utm_medium=transiva&utm_campaign=readme_colophon">Zyvor AI Labs</a> · Open infrastructure for the long term</sub>
</div>
