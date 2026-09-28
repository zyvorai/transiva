# The renewal trap and why teams start here

The problem Transiva Community Edition addresses, the export flow, and how it compares with one-off scripts.

[Back to the README](../README.md)

---

## The renewal trap — escaped with an API

Every hypervisor renewal buries your VMs deeper in someone else’s proprietary API. Getting out usually means a spreadsheet, a maintenance window, and a pile of one-off `ovftool` invocations.

**Transiva turns that into a job.**

```text
  vSphere · Nutanix AHV
           │
           ▼
  ┌────────────────────────────────────────┐
  │  Transiva (Community)                  │──►  discover · inventory · export
  │  transivactl  (compat: hyperctl)       │──►  resumable jobs · REST
  │  transivad    (compat: hypervisord)    │──►  artifacts for h2kvm
  └────────────────────────────────────────┘
           │
           ▼
  h2kvm → GuestKit → KVM / KubeVirt → Zeus OS
```

| | | |
|:---:|:---:|:---:|
| **2** source hypervisors (CE) | **1** workflow for both | **0** guest agents |
| Apache-2.0 | CLI + REST | Offline export — source VM untouched |

**Export with Transiva → convert with [hyper2kvm](https://github.com/zyvorai/h2kvm) → assure with [GuestKit](https://github.com/zyvorai/guestkit) → operate on [Zeus OS](https://zyvor.dev/zeus-os).**

> **Maturity (honest):** CE covers **two sources** and **full exports** (no CBT, no multi-provider dashboard). Ten-plus providers, waves, SSO, and cutover-night support are **Transiva Platform** — [feature matrix](ce-vs-enterprise.md).

## Why teams start here

| Before one-off scripts | With Transiva CE |
|-----------------|----------------|
| `ovftool` one-offs and tribal scripts | One CLI + job model for vSphere **and** Nutanix |
| Spreadsheet of VMs nobody trusts | `transivactl list` / `export` (compat: `hyperctl`) |
| Export fails mid-transfer — start over | Resumable jobs via `transivad` + REST |
| Conversion mutates the live guest | Offline artifacts — source VM untouched until cutover |
| No path to KubeVirt / Zeus OS | Clean handoff into the Zyvor suite |

- **Two sources, one workflow.** vSphere and Nutanix AHV behave identically — same CLI, same jobs, same output layout.
- **Scriptable end to end.** Interactive CLI for one-offs; daemon + REST for fleets.
- **Open by default.** Apache-2.0, no phone-home, no agent in the guest. Source, Docker, RPMs.

---
