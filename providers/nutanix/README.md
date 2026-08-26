# Nutanix AHV Provider

Prism Central v4 provider for VM discovery and offline NFS pickup export.

## Capabilities

- **Discovery**: Paginated `ListVms` + concurrent `GetVmById` for disk/container metadata
- **Export formats**: `qcow2`, `raw` (local NFS pickup only)
- **Pipeline**: Optional hyper2kvm integration after manifest generation

## Quick start

```bash
hyperctl list --provider nutanix --server $PC --user admin --pass 'xxx'
hyperctl export --provider nutanix --vm my-vm --output /exports \
  --mounts 'container-name:/mnt/nutanix/container'
```

Full guide: [docs/nutanix.md](../../docs/nutanix.md)

## Package layout

| File | Purpose |
|------|---------|
| `client.go` | Prism v4 VMM API client |
| `containers.go` | Storage container name resolution (clustermgmt v4) |
| `pickup.go` | Pickup plan JSON generation |
| `executor.go` | NFS path resolution + qemu-img convert |
| `export.go` | `Provider.ExportVM` implementation |
| `manifest.go` | hyper2kvm Artifact Manifest v1.0 builder |
| `progress.go` | qemu-img progress parsing |

## Tests

```bash
go test ./providers/nutanix/... -count=1
```
