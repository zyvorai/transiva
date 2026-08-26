# Community Edition vs HyperSDK Platform (Enterprise)

**Community Edition (this repo) proves export in a lab.**  
**HyperSDK Platform is what you buy to run a hypervisor-exit program.**

CE is free forever for PoC. The moment you need CBT cutover windows, waves, SSO, the full provider matrix, or a named owner when the bridge is live — that is Platform. Day-2 lands on **Zeus OS**. One slipped estate wave usually costs more than the license.

Canonical Enterprise tree: commercial HyperSDK Platform builds (private). Product: [zyvor.dev/transiva](https://zyvor.dev/transiva?utm_source=github&utm_medium=transiva) · [Book a Platform demo](https://zyvor.dev/contact?intent=demo) · [30-day PoC](https://zyvor.dev/poc) · [sales@zyvor.dev](mailto:sales@zyvor.dev)

Pipeline: **HyperSDK → [hyper2kvm](https://github.com/transiva/hyper2kvm) → [GuestKit](https://github.com/transiva/guestkit) → [Zeus OS](https://zyvor.dev/zeus-os)**

---

## Full capability matrix

### Positioning

| Capability | Community Edition (this repo) | HyperSDK Platform (Enterprise) |
| --- | --- | --- |
| What you get | Export / discover control plane for eval | Full migration **operating system** for fleets |
| Who it is for | Labs, PoC, single-host exports | Platform / SRE leads · **50–10,000+ VMs** |
| Success metric | Disks land as artifacts | Wave completion % · CBT windows · attributable exit |
| Cost of staying on CE | Full exports · 2 providers · Issues | Avoided: slipped waves, security blockers, unowned cutovers |
| License | Apache-2.0 | Commercial + SLA |
| Support | GitHub Issues | SLA · workshops · hypervisor-exit programs |

### Sources & export

| Capability | Community | Enterprise |
| --- | --- | --- |
| VMware vSphere / vCenter | ✅ | ✅ |
| Nutanix AHV | ✅ | ✅ |
| Hyper-V · AWS · Azure · GCP · OCI · OpenStack · Alibaba · Proxmox · KubeVirt | Limited / eval | ✅ Full provider matrix (10–11) |
| Custom / air-gap adapters | — | ✅ |
| Parallel + resumable export | ✅ Basic | ✅ Fleet-scale + verify |
| Changed Block Tracking (CBT) | — | ✅ Smart full-export fallback |
| Batch export · compression · verify | ✅ CLI | ✅ Dashboard + API + audit |
| Native format converters (OVF/OVA/VMDK/QCOW2/VHD/VHDX/RAW) | ✅ Core | ✅ Full matrix |

### Pipeline & planning

| Capability | Community | Enterprise |
| --- | --- | --- |
| Hand off to hyper2kvm | ✅ | ✅ In-pipeline orchestration |
| Migration wizard | — | ✅ |
| 17-point readiness checks | — | ✅ |
| Wave planning · approvals · blackout calendars | — | ✅ |
| One-click Export → Convert → Fix → Deploy | Manual glue | ✅ Orchestrated jobs |
| Carbon-aware scheduling · cost estimator | — / limited | ✅ Org-wide budgets · chargeback |
| Cron · deps · retries · priority windows | Basic | ✅ |
| Webhooks · email digests | — | ✅ |

### Deploy targets

| Capability | Community | Enterprise |
| --- | --- | --- |
| Local artifacts → hyper2kvm | ✅ | ✅ |
| libvirt / KVM deploy | Via hyper2kvm | ✅ Integrated |
| KubeVirt / OpenShift | — / DIY | ✅ Operator · PVC · VM CR |
| OpenStack Glance (+ Nova boot) | — | ✅ |
| Live libvirt → KubeVirt path | — | ✅ |
| Windows auto-detect · BitLocker offline · RDP | — | ✅ |

### Interfaces

| Capability | Community | Enterprise |
| --- | --- | --- |
| CLI (`hyperctl` / `hyperexport`) + REST daemon | ✅ | ✅ |
| React dashboard (40–47+ views) | — | ✅ |
| Spotlight / command palette | — | ✅ |
| noVNC consoles | — | ✅ |
| hyperctl TUI | Limited | ✅ |
| Python / TypeScript SDKs | — | ✅ |
| 200–267+ versioned REST routes | Core OpenAPI | ✅ Full contract |

### Identity · security · ops

| Capability | Community | Enterprise |
| --- | --- | --- |
| Config-file credentials | ✅ | ✅ + Vault / AWS SM / Azure KV |
| API keys · session auth | Basic | ✅ |
| RBAC (admin / operator / viewer+) | Limited | ✅ Fine-grained |
| SSO / OIDC / SAML · SCIM | — | ✅ |
| Audit logging · rate limits · HTTPS | Limited | ✅ |
| AES / GPG encryption at rest | — | ✅ |
| Air-gapped bundles · compliance packs | — | ✅ |
| FIPS guidance · pen-test attestation | — | ✅ |
| Multi-tenant control plane · PostgreSQL HA | Single-node / SQLite | ✅ |
| transiva-agent / control plane scale-out | — | ✅ |
| Prometheus · OpenTelemetry · SIEM | Basic | ✅ Fleet SLO reporting |
| Helm operator · CRDs | Eval | ✅ Production |

### Suite landing

| Capability | Community | Enterprise |
| --- | --- | --- |
| Day-2 on **Zeus OS** | Hand off | ✅ Licensed suite path |
| GuestKit assurance gate | Pair yourself | ✅ Integrated playbooks |
| Contractual hypervisor exit PS | — | ✅ |

---

## Why buy Platform (Enterprise)

1. **Two providers and a CLI do not run a multi-wave estate exit** — Platform is the operating system for the program  
2. **Full-export weekends fail change freeze** — CBT and smart fallbacks shrink the window  
3. **Security needs SSO, vaulted secrets, and an audit stream** — not a weekend OIDC experiment under freeze  
4. **Executives need wave plans, readiness checks, and exportable reports** — CLI screenshots do not survive steering  
5. **You want Zyvor on the critical path** — GitHub Issues is not a cutover war-room  

**CE is free forever for labs. Buy Platform when the estate must move.**

**→ [Book a Platform demo](https://zyvor.dev/contact?intent=demo)** · **[30-day PoC](https://zyvor.dev/poc)** · **[Pricing](https://zyvor.dev/pricing)** · **[transiva product](https://zyvor.dev/transiva)**
