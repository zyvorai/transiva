// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"sync"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/providers"
)

// Manager manages provider plugins
type Manager struct {
	logger   logger.Logger
	loader   *Loader
	registry *providers.Registry
	plugins  map[string]*Info
	mu       sync.RWMutex
	watcher  *Watcher
	config   *Config
}

// Config holds plugin manager configuration
type Config struct {
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	Directories []string `yaml:"directories" json:"directories"`
	EnabledList []string `yaml:"enabled" json:"enabled_list"`   // Specific plugins to enable
	DisabledList []string `yaml:"disabled" json:"disabled_list"` // Specific plugins to disable
	HotReload   bool     `yaml:"hot_reload" json:"hot_reload"`
}

// NewManager creates a new plugin manager
func NewManager(registry *providers.Registry, log logger.Logger, config *Config) *Manager {
	if config == nil {
		config = &Config{
			Enabled:     true,
			Directories: GetDefaultPluginDirs(),
			HotReload:   false,
		}
	}

	// Ensure we have plugin directories
	if len(config.Directories) == 0 {
		config.Directories = GetDefaultPluginDirs()
	}

	// Add directories from environment
	envDirs := ParsePluginPath()
	if len(envDirs) > 0 {
		config.Directories = append(config.Directories, envDirs...)
	}

	return &Manager{
		logger:   log,
		loader:   NewLoader(log),
		registry: registry,
		plugins:  make(map[string]*Info),
		config:   config,
	}
}

// LoadAll discovers and loads all plugins from configured directories
func (m *Manager) LoadAll() error {
	if !m.config.Enabled {
		m.logger.Info("plugin system disabled")
		return nil
	}

	m.logger.Info("discovering plugins", "directories", m.config.Directories)

	// Discover all plugin files
	pluginPaths, err := m.loader.DiscoverAll(m.config.Directories)
	if err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	m.logger.Info("found plugin files", "count", len(pluginPaths))

	// Load each plugin
	var loadedCount int
	for _, path := range pluginPaths {
		if err := m.LoadPlugin(path); err != nil {
			m.logger.Warn("failed to load plugin",
				"path", path,
				"error", err)
			continue
		}
		loadedCount++
	}

	m.logger.Info("plugins loaded", "total", loadedCount, "failed", len(pluginPaths)-loadedCount)

	return nil
}

// LoadPlugin loads a single plugin
func (m *Manager) LoadPlugin(path string) error {
	// Load plugin
	info, factory, err := m.loader.Load(path)
	if err != nil {
		return err
	}

	// Check if plugin is enabled
	if !m.isPluginEnabled(info.Metadata.Name) {
		m.logger.Info("plugin disabled by configuration",
			"name", info.Metadata.Name)
		info.Status = StatusDisabled
		m.addPluginInfo(info)
		return nil
	}

	// Register provider with registry
	m.registry.Register(info.Metadata.ProviderType, factory)

	// Update info
	info.LoadedAt = time.Now()
	info.Status = StatusLoaded

	// Store plugin info
	m.addPluginInfo(info)

	m.logger.Info("plugin registered",
		"name", info.Metadata.Name,
		"type", info.Metadata.ProviderType)

	return nil
}

// UnloadPlugin unloads a plugin
func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Unregister from provider registry
	m.registry.Unregister(info.Metadata.ProviderType)

	// Update status
	info.Status = StatusUnloaded

	m.logger.Info("plugin unloaded", "name", name)

	return nil
}

// ReloadPlugin reloads a plugin
func (m *Manager) ReloadPlugin(name string) error {
	m.mu.RLock()
	info, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Unload first
	if err := m.UnloadPlugin(name); err != nil {
		return fmt.Errorf("failed to unload plugin: %w", err)
	}

	// Reload
	if err := m.LoadPlugin(info.Path); err != nil {
		return fmt.Errorf("failed to reload plugin: %w", err)
	}

	m.logger.Info("plugin reloaded", "name", name)

	return nil
}

// ListPlugins returns information about all loaded plugins
func (m *Manager) ListPlugins() []*Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*Info, 0, len(m.plugins))
	for _, info := range m.plugins {
		plugins = append(plugins, info)
	}

	return plugins
}

// GetPlugin returns information about a specific plugin
func (m *Manager) GetPlugin(name string) (*Info, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.plugins[name]
	return info, exists
}

// StartWatcher starts the hot-reload file watcher
func (m *Manager) StartWatcher() error {
	if !m.config.HotReload {
		m.logger.Info("hot-reload disabled")
		return nil
	}

	watcher, err := NewWatcher(m, m.logger)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Watch all plugin directories
	for _, dir := range m.config.Directories {
		if err := watcher.Watch(dir); err != nil {
			m.logger.Warn("failed to watch directory",
				"dir", dir,
				"error", err)
		}
	}

	m.watcher = watcher

	m.logger.Info("hot-reload watcher started")

	return nil
}

// StopWatcher stops the file watcher
func (m *Manager) StopWatcher() error {
	if m.watcher == nil {
		return nil
	}

	if err := m.watcher.Close(); err != nil {
		return err
	}

	m.watcher = nil
	m.logger.Info("hot-reload watcher stopped")

	return nil
}

// Helper methods

func (m *Manager) addPluginInfo(info *Info) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[info.Metadata.Name] = info
}

func (m *Manager) isPluginEnabled(name string) bool {
	// If there's an enabled list, plugin must be in it
	if len(m.config.EnabledList) > 0 {
		for _, enabled := range m.config.EnabledList {
			if enabled == name {
				return true
			}
		}
		return false
	}

	// If there's a disabled list, plugin must not be in it
	if len(m.config.DisabledList) > 0 {
		for _, disabled := range m.config.DisabledList {
			if disabled == name {
				return false
			}
		}
	}

	// Default: enabled
	return true
}

// GetStats returns plugin statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total":    len(m.plugins),
		"loaded":   0,
		"failed":   0,
		"disabled": 0,
	}

	for _, info := range m.plugins {
		switch info.Status {
		case StatusLoaded:
			stats["loaded"] = stats["loaded"].(int) + 1
		case StatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		case StatusDisabled:
			stats["disabled"] = stats["disabled"].(int) + 1
		}
	}

	return stats
}
