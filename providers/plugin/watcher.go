// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/zyvorai/transiva/logger"
)

// Watcher watches for plugin file changes and triggers reloads
type Watcher struct {
	manager  *Manager
	logger   logger.Logger
	watcher  *fsnotify.Watcher
	stopChan chan struct{}
}

// NewWatcher creates a new plugin watcher
func NewWatcher(manager *Manager, log logger.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	w := &Watcher{
		manager:  manager,
		logger:   log,
		watcher:  fsWatcher,
		stopChan: make(chan struct{}),
	}

	// Start event loop
	go w.eventLoop()

	return w, nil
}

// Watch adds a directory to watch
func (w *Watcher) Watch(dir string) error {
	if err := w.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	w.logger.Debug("watching directory for plugins", "dir", dir)

	return nil
}

// Close stops the watcher
func (w *Watcher) Close() error {
	close(w.stopChan)
	return w.watcher.Close()
}

// eventLoop processes file system events
func (w *Watcher) eventLoop() {
	// Debounce map to avoid duplicate events
	debounce := make(map[string]time.Time)
	debounceDuration := 1 * time.Second

	for {
		select {
		case <-w.stopChan:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only process .so files
			if !strings.HasSuffix(event.Name, ".so") {
				continue
			}

			// Debounce events
			now := time.Now()
			if lastEvent, exists := debounce[event.Name]; exists {
				if now.Sub(lastEvent) < debounceDuration {
					continue
				}
			}
			debounce[event.Name] = now

			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn("file watcher error", "error", err)
		}
	}
}

// handleEvent handles a single file system event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	pluginName := w.getPluginName(event.Name)

	w.logger.Debug("plugin file event",
		"name", pluginName,
		"path", event.Name,
		"op", event.Op.String())

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		w.handleCreate(event.Name)

	case event.Op&fsnotify.Write == fsnotify.Write:
		w.handleUpdate(pluginName, event.Name)

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		w.handleRemove(pluginName)

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		w.handleRemove(pluginName)
	}
}

// handleCreate handles plugin file creation
func (w *Watcher) handleCreate(path string) {
	w.logger.Info("new plugin detected", "path", path)

	// Wait a moment for file to be fully written
	time.Sleep(500 * time.Millisecond)

	// Load the new plugin
	if err := w.manager.LoadPlugin(path); err != nil {
		w.logger.Error("failed to load new plugin",
			"path", path,
			"error", err)
		return
	}

	w.logger.Info("new plugin loaded successfully", "path", path)
}

// handleUpdate handles plugin file updates
func (w *Watcher) handleUpdate(name, path string) {
	w.logger.Info("plugin updated", "name", name, "path", path)

	// Wait a moment for file to be fully written
	time.Sleep(500 * time.Millisecond)

	// Reload the plugin
	if err := w.manager.ReloadPlugin(name); err != nil {
		w.logger.Error("failed to reload plugin",
			"name", name,
			"error", err)
		return
	}

	w.logger.Info("plugin reloaded successfully", "name", name)
}

// handleRemove handles plugin file removal
func (w *Watcher) handleRemove(name string) {
	w.logger.Info("plugin removed", "name", name)

	// Unload the plugin
	if err := w.manager.UnloadPlugin(name); err != nil {
		w.logger.Error("failed to unload plugin",
			"name", name,
			"error", err)
		return
	}

	w.logger.Info("plugin unloaded successfully", "name", name)
}

// getPluginName extracts plugin name from file path
func (w *Watcher) getPluginName(path string) string {
	base := filepath.Base(path)
	// Remove .so extension
	return strings.TrimSuffix(base, ".so")
}
