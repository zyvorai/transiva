# Nutanix AHV Migration Guide

HyperSDK supports **Nutanix AHV** VM discovery and **offline NFS pickup export** for migration to KVM via [hyper2kvm](https://github.com/transiva/hyper2kvm).

## Overview

| Phase | What HyperSDK does |
|-------|-------------------|
| Discovery | List VMs from Prism Central (v4 API), fetch disk UUIDs and storage container IDs |
| Pickup plan | Generate JSON with NFS-relative paths (`.acropolis/vmdisk/<uuid>/`) |
| Export | `qemu-img convert` from mounted containers → qcow2/raw + `artifact-manifest.json` |
| Pipeline | Optional hyper2kvm queue after export |

Nutanix export is **not** a live API pull of disk bytes. You mount storage containers via NFS on the migration host, then HyperSDK locates and converts disk images offline.

## Prerequisites

1. **Prism Central** credentials (read access for VM/container discovery)
2. **NFS mounts** of Nutanix storage containers on the export host
3. **`qemu-img`** on the export host PATH
4. Container mount map, e.g. `default-container-12345:/mnt/nutanix/default`

## Configuration

Copy `config.example.yaml` → `config.yaml` and configure the `nutanix` section, or use environment variables:

| Variable | Description |
|----------|-------------|
| `NUTANIX_HOST` | Prism Central FQDN or IP |
| `NUTANIX_USERNAME` | Prism username |
| `NUTANIX_PASSWORD` | Prism password |
| `NUTANIX_CLUSTER` | Optional cluster name/UUID filter |
| `NUTANIX_VERIFY_SSL` | `1` to verify TLS (default off in labs) |
| `NUTANIX_ENABLED` | `1` to enable provider in daemon |

Example `config.yaml`:

```yaml
nutanix:
  host: "prism-central.example.com"
  port: 9440
  username: "admin"
  password: "your-password"
  verify_ssl: false
  cluster: ""
  output_dir: "/var/lib/transiva/nutanix"
  export_format: "qcow2"
  mounts:
    default-container-12345: "/mnt/nutanix/default"
  resolve_containers: true
  enable_pipeline: false
  enabled: true
```

## CLI Workflows

### Discovery

```bash
# Via hyperctl (daemon must be running)
hyperctl list --provider nutanix --server prism.example.com --user admin --pass 'xxx'

# VM details
hyperctl info --provider nutanix --vm web-prod-01 --server prism.example.com --user admin --pass 'xxx'

# Standalone pickup plan
go run ./cmd/nutanix-pickup \
  --prism prism.example.com --user admin --pass 'xxx' \
  --format pickup-plan --resolve-containers --out plan.json
```

### Export (sync)

```bash
hyperctl export --provider nutanix \
  --vm web-prod-01 \
  --output /var/lib/transiva/nutanix \
  --mounts 'default-container-12345:/mnt/nutanix/default' \
  --resolve-containers

hyperexport --provider nutanix \
  -vm web-prod-01 \
  -output /var/lib/transiva/nutanix \
  -mounts 'default-container-12345:/mnt/nutanix/default'
```

### Export (async job queue)

```bash
hyperctl submit --provider nutanix \
  --vm web-prod-01 \
  --output /var/lib/transiva/nutanix \
  --mounts 'default-container-12345:/mnt/nutanix/default' \
  --resolve-containers --pipeline

hyperctl query -status running
```

### Batch export

```bash
# One VM name/UUID per line in vms.txt
hyperexport --provider nutanix --batch vms.txt \
  -mounts 'default-container-12345:/mnt/nutanix/default' \
  -output /var/lib/transiva/nutanix
```

## REST API

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/vms/list?provider=nutanix` | GET | List VMs (query: `server`, `username`, `password`, `cluster`, `detailed`) |
| `/vms/info?provider=nutanix&vm={name\|uuid}` | GET/POST | VM details + disk metadata |
| `/vms/export?provider=nutanix` | POST | Synchronous NFS pickup export |
| `/providers/list` | GET | Registered providers |
| `/providers/capabilities?provider=nutanix` | GET | Supported formats (qcow2, raw) |
| `/jobs/submit` | POST | Async export (`provider: nutanix`, `export_method: nutanix`) |

Example export request:

```bash
curl -X POST 'http://localhost:8080/vms/export?provider=nutanix' \
  -H 'Content-Type: application/json' \
  -d '{
    "vm": "web-prod-01",
    "output_path": "/var/lib/transiva/nutanix",
    "format": "qcow2",
    "mounts": {"default-container-12345": "/mnt/nutanix/default"},
    "resolve_containers": true
  }'
```

Example job YAML (`jobs/nutanix-export.yaml`):

```yaml
name: nutanix-web-prod-01
provider: nutanix
export_method: nutanix
vm_path: web-prod-01
output_path: /var/lib/transiva/nutanix
format: qcow2
enable_pipeline: true
metadata:
  mounts:
    default-container-12345: /mnt/nutanix/default
  resolve_containers: true
```

## Dashboard

Open **Export Workflow** in the web dashboard (`http://localhost:8080/web/dashboard/`), select the **Nutanix AHV** tab, connect to Prism Central, pick a VM, configure container mounts, and submit an export job.

## Progress

Nutanix exports report live progress during `qemu-img convert` (parsed from `-p` stderr). Job progress is visible via:

```bash
hyperctl watch <job-id>
```

Phases: `connecting` → `locating` → `converting` → `manifest` → `completed`.

## Architecture

```
Prism Central (v4 API)  →  VM + disk metadata
        ↓
NFS-mounted containers  →  .acropolis/vmdisk/<uuid>/
        ↓
qemu-img convert        →  qcow2/raw per disk
        ↓
artifact-manifest.json →  hyper2kvm pipeline
```

See also: [`providers/nutanix/`](../providers/nutanix/) source, [`config.example.yaml`](../config.example.yaml).
