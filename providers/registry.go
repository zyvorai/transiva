// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"fmt"
	"sync"
)

// ProviderFactory is a function that creates a new provider instance
type ProviderFactory func(config ProviderConfig) (Provider, error)

// Registry manages provider factories and creates provider instances
type Registry struct {
	factories map[ProviderType]ProviderFactory
	mu        sync.RWMutex
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[ProviderType]ProviderFactory),
	}
}

// Register registers a provider factory for a given provider type
func (r *Registry) Register(pType ProviderType, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[pType] = factory
}

// Unregister removes a provider factory
func (r *Registry) Unregister(pType ProviderType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factories, pType)
}

// Create creates a new provider instance
func (r *Registry) Create(pType ProviderType, config ProviderConfig) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.factories[pType]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", pType)
	}

	// Ensure config has the correct type
	config.Type = pType

	return factory(config)
}

// IsRegistered checks if a provider type is registered
func (r *Registry) IsRegistered(pType ProviderType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[pType]
	return ok
}

// ListProviders returns a list of all registered provider types
func (r *Registry) ListProviders() []ProviderType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]ProviderType, 0, len(r.factories))
	for pType := range r.factories {
		types = append(types, pType)
	}
	return types
}

// GetCapabilities returns the capabilities of a provider type
// Creates a temporary instance to query capabilities
func (r *Registry) GetCapabilities(pType ProviderType) (*ExportCapabilities, error) {
	// Create a dummy provider instance with minimal config
	config := ProviderConfig{
		Type: pType,
	}

	provider, err := r.Create(pType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider instance: %w", err)
	}

	caps := provider.GetExportCapabilities()
	return &caps, nil
}
