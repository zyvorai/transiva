# Where Transiva fits in the Zyvor suite

How exports flow through h2kvm and GuestKit to KVM, KubeVirt and Zeus OS.

[Back to the README](../README.md)

---

## Where this fits: the Zyvor suite

```mermaid
flowchart LR
    A["vSphere"] --> H
    B["Nutanix AHV"] --> H
    H["<b>transiva</b><br/>discover · export"] --> K["h2kvm<br/>convert to QCOW2"]
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
| Convert | [h2kvm](https://github.com/zyvorai/h2kvm) | Rewrite to QCOW2 offline, fix drivers before first boot |
| Assure | [GuestKit](https://github.com/zyvorai/guestkit) | Offline doctor score + Passport before power-on |
| Host | [Machina](https://zyvor.dev/machina) | Bare-metal KVM / libvirt on the hypervisor host |
| Operate | [Zeus OS](https://zyvor.dev/zeus-os) | VMs + containers — KubeVirt lifecycle, GPU, multi-cluster |

Methodology: **Discover → Assess → Convert → Deploy → Operate → Optimize.** This repo covers discover + export in CE. [Full hypervisor-exit route →](https://zyvor.dev/hypervisor-exit?utm_source=github&utm_medium=transiva&utm_campaign=readme_suite)

---
