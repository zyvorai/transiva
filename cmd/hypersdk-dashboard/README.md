# HyperSDK Dashboard

Web-based dashboard for managing VirtualMachines on Kubernetes.

## Features

- **VM Management**: Create, start, stop, and delete VMs
- **Operations**: Track VM operations (clone, migrate, resize, snapshot)
- **Snapshots**: View and manage VM snapshots
- **Templates**: Browse and use VM templates
- **Real-time Updates**: WebSocket-based live status updates
- **Responsive UI**: Works on desktop and mobile

## Quick Start

### Build and Run

```bash
# Build the dashboard
go build -o transiva-dashboard ./cmd/transiva-dashboard

# Run with default settings
./transiva-dashboard

# Run with custom settings
./transiva-dashboard -port 8090 -namespace default -kubeconfig ~/.kube/config
```

### Access the Dashboard

Open your browser and navigate to:
```
http://localhost:8090
```

## Command-Line Options

- `-port` - HTTP port (default: 8090)
- `-namespace` - Default Kubernetes namespace (default: "default")
- `-kubeconfig` - Path to kubeconfig file (default: auto-detect)

## API Endpoints

### VMs
- `GET /api/vms` - List all VMs
- `POST /api/vms` - Create a new VM
- `GET /api/vms/{name}` - Get VM details
- `DELETE /api/vms/{name}` - Delete a VM

### Operations
- `GET /api/operations` - List VM operations
- `POST /api/operations` - Create a new operation

### Snapshots
- `GET /api/snapshots` - List VM snapshots

### Templates
- `GET /api/templates` - List VM templates

### WebSocket
- `WS /ws` - Real-time updates

## Dashboard Views

### Virtual Machines
- List all VMs with status, resources, and age
- Filter by name and status
- Quick actions: Start, Stop, More options
- Detailed VM view with disks and networks

### Operations
- View all VM operations (start, stop, clone, migrate, etc.)
- Progress tracking with percentages
- Operation status and timestamps

### Snapshots
- List all VM snapshots
- View snapshot details (size, creation time)
- Restore snapshots (coming soon)

### Templates
- Browse available VM templates
- Click to create VM from template
- View template details (OS, version, size)

## Usage Examples

### Creating a VM

1. Click "Create VM" button
2. Fill in the form:
   - VM Name: `web-server-1`
   - CPUs: `4`
   - Memory: `8Gi`
   - Source: `ubuntu:22.04` (image) or template name
3. Click "Create"

### Managing VMs

**Start/Stop VM:**
- Click the Start/Stop button in the VM list

**VM Operations:**
1. Click the "⋮" menu button
2. Choose an operation:
   - Clone VM
   - Create Snapshot
   - Migrate VM
   - Resize VM
   - Delete VM

### Viewing VM Details

- Click on any VM row to see detailed information
- View resources, disks, networks, and IP addresses

## Development

### Project Structure

```
cmd/transiva-dashboard/
├── main.go              # Dashboard server
├── static/
│   ├── dashboard.css    # Styles
│   └── dashboard.js     # Frontend JavaScript
└── README.md            # This file
```

### Mock Data

Currently, the dashboard uses mock data for demonstration. To connect to a real Kubernetes cluster:

1. Implement Kubernetes client in `main.go`
2. Replace mock data with actual API calls
3. Use the HyperSDK CRD client

### Adding Features

1. **New API Endpoint**: Add handler in `main.go`
2. **New UI Component**: Add HTML in `handleIndex()` or JavaScript in `dashboard.js`
3. **New Styles**: Add CSS in `dashboard.css`

## Deployment

### Docker

```bash
# Build Docker image
docker build -t transiva-dashboard -f cmd/transiva-dashboard/Dockerfile .

# Run container
docker run -p 8090:8090 transiva-dashboard
```

### Kubernetes

```bash
# Deploy as a service in the cluster
kubectl apply -f deploy/dashboard/deployment.yaml
kubectl apply -f deploy/dashboard/service.yaml

# Access via port-forward
kubectl port-forward svc/transiva-dashboard 8090:8090
```

## Security

**Important**: This is a development version with minimal security:

- No authentication/authorization
- WebSocket accepts all origins
- No HTTPS/TLS

For production use:
1. Add authentication (OAuth, OIDC, etc.)
2. Enable RBAC for Kubernetes access
3. Use HTTPS with valid certificates
4. Restrict WebSocket origins
5. Add rate limiting

## Browser Support

- Chrome/Edge (recommended)
- Firefox
- Safari

## Troubleshooting

### Dashboard won't start
- Check if port 8090 is already in use
- Verify kubeconfig path is correct
- Ensure Go dependencies are installed

### Can't connect to Kubernetes
- Verify kubeconfig is valid: `kubectl cluster-info`
- Check namespace exists: `kubectl get namespaces`
- Ensure CRDs are installed: `kubectl get crd virtualmachines.transiva.zyvor.ai`

### WebSocket not connecting
- Check browser console for errors
- Verify firewall allows WebSocket connections
- Try disabling browser extensions

## Contributing

1. Add new features in separate files
2. Follow Go and JavaScript best practices
3. Update this README with new features
4. Test on multiple browsers

## License

SPDX-License-Identifier: Apache-2.0
