# KubeVirt Provider

## Overview

The KubeVirt provider enables HyperSDK to manage virtual machines running on Kubernetes with KubeVirt.

## Current Status

**Status**: Stub Implementation (MVP Phase)

The provider currently uses a stub implementation while KubeVirt dependency versions are being finalized.

### Files

- **provider_stub.go**: Current stub implementation (default build)
- **provider_full.go**: Full implementation (requires `full` build tag)
- **operations_full.go**: VM operations (start, stop, restart, etc.)
- **snapshot_full.go**: Snapshot management

### Building

**Default (Stub):**
```bash
go build
```

**Full Implementation:**
```bash
go build -tags full
```

Note: The full implementation requires KubeVirt dependencies to be resolved.

## Planned Features

### Phase 1: Core Provider (Current)
- [x] Provider interface implementation
- [x] Registration in provider registry
- [ ] Dependency resolution for KubeVirt libraries
- [ ] Basic VM listing and discovery

### Phase 2: VM Operations
- [ ] Start/Stop/Restart VMs
- [ ] Pause/Unpause VMs
- [ ] VM migration
- [ ] VM cloning
- [ ] VM deletion

### Phase 3: Snapshot Management
- [ ] Create snapshots
- [ ] List snapshots
- [ ] Restore from snapshot
- [ ] Delete snapshots
- [ ] Export snapshots
- [ ] Clone from snapshot

### Phase 4: VM Export
- [ ] Export to OVF/OVA
- [ ] Export to RAW/QCOW2/VMDK
- [ ] Streaming export
- [ ] Direct cloud storage export

### Phase 5: Advanced Features
- [ ] Live migration
- [ ] Resource monitoring
- [ ] Network management
- [ ] Volume management
- [ ] Template support

## Dependencies

The full implementation requires:

```go
k8s.io/api v0.30.0+
k8s.io/apimachinery v0.30.0+
k8s.io/client-go v0.30.0+
kubevirt.io/api v1.1.0+
kubevirt.io/client-go v1.1.0+
kubevirt.io/containerized-data-importer-api v1.58.0+
```

## Configuration

### Provider Config

```yaml
provider:
  type: kubevirt
  metadata:
    namespace: default              # Kubernetes namespace
    kubeconfig: ~/.kube/config      # Path to kubeconfig (optional)
    storageClass: standard          # Default storage class for volumes
```

### Job Definition with KubeVirt

```json
{
  "vm_path": "default/ubuntu-vm-1",
  "output_dir": "/backups",
  "provider": "kubevirt",
  "metadata": {
    "namespace": "default"
  }
}
```

## Usage Examples

### List VMs (Stub)

```bash
hyperctl provider list --type kubevirt --namespace default
```

### Export VM (Stub)

```bash
hyperctl export --provider kubevirt --vm default/my-vm --output /backups
```

## Implementation Notes

### VM Identifiers

VMs are identified using the format: `namespace/vm-name`

Example: `default/ubuntu-vm-1`

### Namespace

- Default namespace: `default`
- Can be overridden in provider config or per-operation
- Use `--all-namespaces` flag for cluster-wide operations

### Authentication

The provider uses standard Kubernetes authentication:

1. **In-Cluster**: Uses service account when running in a pod
2. **Kubeconfig**: Uses `~/.kube/config` or path specified in config
3. **Context**: Respects current context in kubeconfig

## API Endpoints

Once fully implemented, the following endpoints will be available:

### KubeVirt-Specific Endpoints

- `POST /kubevirt/vms/list` - List VMs in namespace
- `GET /kubevirt/vms/:namespace/:name` - Get VM details
- `POST /kubevirt/vms/:namespace/:name/start` - Start VM
- `POST /kubevirt/vms/:namespace/:name/stop` - Stop VM
- `POST /kubevirt/vms/:namespace/:name/restart` - Restart VM
- `POST /kubevirt/vms/:namespace/:name/migrate` - Migrate VM
- `POST /kubevirt/vms/:namespace/:name/snapshot` - Create snapshot
- `GET /kubevirt/vms/:namespace/:name/snapshots` - List snapshots
- `POST /kubevirt/vms/:namespace/:name/restore` - Restore from snapshot

## Next Steps

1. **Resolve Dependencies**: Finalize compatible versions of k8s.io and kubevirt.io packages
2. **Enable Full Build**: Remove build tags once dependencies are stable
3. **Integration Testing**: Test with live KubeVirt cluster
4. **Documentation**: Complete API documentation and examples
5. **CLI Integration**: Add kubevirt-specific CLI commands

## Development Timeline

Estimated implementation timeline once dependencies are resolved:

- **Week 1**: Core provider implementation and VM operations
- **Week 2**: Snapshot management and export functionality
- **Week 3**: Integration testing and documentation
- **Week 4**: Advanced features and optimization

## Contributing

The KubeVirt provider is part of HyperSDK v2.1.0 roadmap. For questions or contributions, please see the main project documentation.

## References

- [KubeVirt Documentation](https://kubevirt.io/user-guide/)
- [KubeVirt API Reference](https://kubevirt.io/api-reference/)
- [Kubernetes Client-Go](https://github.com/kubernetes/client-go)
- [CDI (Containerized Data Importer)](https://github.com/kubevirt/containerized-data-importer)

---

*KubeVirt Provider - HyperSDK v2.1.0*
*Status: Development - Stub Implementation*
*Last Updated: 2026-02-04*
