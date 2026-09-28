# Community Edition vs Transiva Platform

What the Community Edition covers, what Transiva Platform adds, and why teams upgrade. The full feature matrix is in [ce-vs-enterprise.md](ce-vs-enterprise.md).

[Back to the README](../README.md)

---

## Community Edition vs Transiva Platform

**CE proves export. Platform runs the hypervisor-exit program.**

CE is free forever for labs — two sources, CLI, GitHub Issues. If you are moving an estate: no CBT, no waves, no SSO, no named owner on cutover night. **Buy Platform.**

| | **Community Edition** *(this repo)* | **[Transiva Platform](https://zyvor.dev/transiva?utm_source=github&utm_medium=transiva&utm_campaign=readme_table)** |
|---|---|---|
| **Who it is for** | Labs · PoC · single-host | Platform / SRE leads · **50–10,000+ VMs** |
| **Sources** | vSphere, Nutanix AHV | **10–11 providers** — Hyper-V, AWS, Azure, GCP, OCI, OpenStack, Proxmox, KubeVirt, … |
| **Interface** | CLI + REST daemon | **Dashboard** · Spotlight · noVNC · SDKs · 200–267+ routes |
| **Incremental** | Full exports only | **CBT** + smart fallback — windows that fit change freeze |
| **Pipeline** | Hand off to h2kvm yourself | Wizard · **17 readiness checks** · waves · approvals · blackouts |
| **Deploy targets** | Local output → h2kvm | Glance/Nova · libvirt · **KubeVirt** · live libvirt→KubeVirt |
| **Security** | Config-file credentials | Vault · **SSO/OIDC/SAML** · RBAC · audit · air-gap packs |
| **Ops** | Single-node eval | Multi-tenant CP · HA · agents · carbon/chargeback · compliance |
| **Day-2** | Hand off to Zeus OS | Licensed **Zeus OS** suite path |
| **Support** | [GitHub Issues](https://github.com/zyvorai/transiva/issues) | **SLA** · workshops · hypervisor-exit PS |

### Why teams upgrade

1. Full-export weekends do not survive a multi-wave estate  
2. Security needs SSO, vaulted secrets, and an audit stream  
3. Executives need wave plans — not CLI screenshots  
4. CBT shrinks the cutover window into change freeze  
5. You need Zyvor on the bridge — not Issues at 2 a.m.  

**[Full feature matrix →](ce-vs-enterprise.md)** · [enterprise.md](enterprise.md)

**Bring us your worst estate.** [30-day PoC](https://zyvor.dev/poc?utm_source=github&utm_medium=transiva&utm_campaign=readme_footer) · [Platform demo](https://zyvor.dev/contact?intent=demo&utm_source=github&utm_medium=transiva&utm_campaign=readme_footer) · [Pricing](https://zyvor.dev/pricing?utm_source=github&utm_medium=transiva&utm_campaign=readme_footer)

---
