# Provider Plugin System

Dynamic provider loading for HyperSDK.

## Overview

The plugin system allows HyperSDK to load provider implementations at runtime without recompiling the daemon. This enables:

✅ **Community Contributions** - Add support for new cloud providers
✅ **Rapid Development** - Test and iterate without daemon restart
✅ **Modularity** - Keep provider code separate from core
✅ **Hot Reload** - Load new plugins without downtime

## Quick Start

### For Users

1. **Download a plugin**:
   ```bash
   wget https://github.com/user/provider-plugin/releases/download/v1.0.0/myprovider.so
   ```

2. **Install the plugin**:
   ```bash
   mkdir -p ~/.transiva/plugins
   cp myprovider.so ~/.transiva/plugins/
   ```

3. **Enable hot-reload** (optional):
   ```yaml
   # config.yaml
   plugins:
     enabled: true
     hot_reload: true
   ```

4. **Restart daemon**:
   ```bash
   systemctl restart hypervisord
   ```

### For Developers

1. **Create plugin project**:
   ```bash
   mkdir myprovider && cd myprovider
   go mod init transiva/plugins/myprovider
   ```

2. **Implement provider**:
   ```go
   // main.go
   package main

   import (
       "github.com/zyvorai/transiva/logger"
       "github.com/zyvorai/transiva/providers"
       "github.com/zyvorai/transiva/providers/plugin"
   )

   var PluginInfo = plugin.Metadata{
       Name: "myprovider",
       Version: "1.0.0",
       ProviderType: "myprovider",
   }

   func NewProvider(config providers.ProviderConfig, log logger.Logger) (providers.Provider, error) {
       return &MyProvider{}, nil
   }
   ```

3. **Build plugin**:
   ```bash
   go build -buildmode=plugin -o myprovider.so
   ```

4. **Test locally**:
   ```bash
   mkdir -p ~/.transiva/plugins
   cp myprovider.so ~/.transiva/plugins/
   ```

## Architecture

### Components

- **Loader** (`loader.go`) - Discovers and loads `.so` files
- **Manager** (`manager.go`) - Manages plugin lifecycle
- **Watcher** (`watcher.go`) - Monitors files for hot-reload
- **Metadata** (`metadata.go`) - Plugin information structures

### Plugin Discovery

Plugins are discovered in:
1. `/usr/local/lib/transiva/plugins/`
2. `/usr/lib/transiva/plugins/`
3. `~/.transiva/plugins/`
4. `./plugins/` (current directory)
5. `$HYPERSDK_PLUGIN_PATH` (environment variable)

### Required Exports

Every plugin must export:

1. **PluginInfo** - Metadata
   ```go
   var PluginInfo = plugin.Metadata{
       Name: "myprovider",
       Version: "1.0.0",
       ProviderType: "myprovider",
       Capabilities: providers.ExportCapabilities{...},
   }
   ```

2. **NewProvider** - Factory function
   ```go
   func NewProvider(config providers.ProviderConfig, log logger.Logger) (providers.Provider, error)
   ```

## Configuration

```yaml
# config.yaml
plugins:
  enabled: true
  hot_reload: true
  directories:
    - /usr/local/lib/transiva/plugins
    - ~/.transiva/plugins
  enabled:
    - digitalocean
    - linode
  disabled:
    - legacy-provider
```

## API Endpoints

### List Plugins

```bash
GET /api/plugins
```

Response:
```json
{
  "plugins": [
    {
      "name": "digitalocean",
      "version": "1.0.0",
      "type": "digitalocean",
      "status": "loaded",
      "loaded_at": "2026-02-03T10:30:00Z"
    }
  ]
}
```

### Reload Plugins

```bash
POST /api/plugins/reload
```

### Get Plugin Info

```bash
GET /api/plugins/{name}
```

## Documentation

- **Example Plugin** — implement `Provider` in a separate Go module (see below)

## Examples

- See the plugin interface section below for a minimal implementation outline

## Troubleshooting

### Plugin Not Loading

Check logs:
```bash
journalctl -u hypervisord | grep plugin
```

Validate exports:
```bash
go tool nm myprovider.so | grep -E '(PluginInfo|NewProvider)'
```

### Version Mismatch

Ensure Go versions match:
```bash
hypervisord --version  # Shows Go version
go version             # Your Go version
```

Must match exactly for plugins to load.

## Development

### Running Tests

```bash
go test -v ./...
```

### Building

```bash
go build
```

## Contributing

See the interface definitions in this package and `providers/` for integration patterns.

## License

Apache-2.0
