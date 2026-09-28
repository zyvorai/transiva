# Quick start

Build Transiva, export from VMware vSphere or Nutanix AHV, and run the daemon and REST API.

[Back to the README](../README.md)

---

## 60-second quick start

<details open>
<summary><b>VMware vSphere</b></summary>

```bash
git clone https://github.com/zyvorai/transiva.git
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

Full walkthrough: **[docs/nutanix.md](nutanix.md)**

</details>

<details>
<summary><b>Daemon + REST</b></summary>

```bash
./bin/hypervisord --config config.yaml
./bin/hyperctl status
```

REST surface: [openapi.yaml](../openapi.yaml) · containers: [deployments/docker/](../deployments/docker/)

</details>

### Binaries

| Binary | Role |
|--------|------|
| `transivaexport` (compat: `hyperexport`) | Interactive CLI exports (vSphere, Nutanix, …) |
| `transivad` (compat: `hypervisord`) | REST API daemon |
| `transivactl` (compat: `hyperctl`) | Job control — `list`, `export`, `info`, `submit` |
| `nutanix-pickup` | Standalone Nutanix discovery/pickup |

> Legacy `hyper*` binaries remain compatibility aliases for at least one major release.

---
