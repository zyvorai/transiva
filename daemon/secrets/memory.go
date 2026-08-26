// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryManager is an in-memory secret store for testing/development
// WARNING: Secrets are not encrypted and will be lost on restart
type MemoryManager struct {
	secrets map[string]*Secret
	mu      sync.RWMutex
}

// NewMemoryManager creates a new in-memory secret manager
func NewMemoryManager() *MemoryManager {
	return &MemoryManager{
		secrets: make(map[string]*Secret),
	}
}

// Get retrieves a secret by name
func (m *MemoryManager) Get(ctx context.Context, name string) (*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secret, exists := m.secrets[name]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", name)
	}

	// Return a copy to prevent external modification
	return &Secret{
		Name:      secret.Name,
		Type:      secret.Type,
		Value:     copyMap(secret.Value),
		Version:   secret.Version,
		CreatedAt: secret.CreatedAt,
		UpdatedAt: secret.UpdatedAt,
		Metadata:  copyMap(secret.Metadata),
	}, nil
}

// Set stores or updates a secret
func (m *MemoryManager) Set(ctx context.Context, secret *Secret) error {
	if secret.Name == "" {
		return fmt.Errorf("secret name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Check if updating existing secret
	existing, exists := m.secrets[secret.Name]
	if exists {
		secret.CreatedAt = existing.CreatedAt
		secret.UpdatedAt = now
		// Increment version
		secret.Version = incrementVersion(existing.Version)
	} else {
		secret.CreatedAt = now
		secret.UpdatedAt = now
		secret.Version = "1"
	}

	// Store a copy to prevent external modification
	m.secrets[secret.Name] = &Secret{
		Name:      secret.Name,
		Type:      secret.Type,
		Value:     copyMap(secret.Value),
		Version:   secret.Version,
		CreatedAt: secret.CreatedAt,
		UpdatedAt: secret.UpdatedAt,
		Metadata:  copyMap(secret.Metadata),
	}

	return nil
}

// Delete removes a secret
func (m *MemoryManager) Delete(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.secrets[name]; !exists {
		return fmt.Errorf("secret not found: %s", name)
	}

	delete(m.secrets, name)
	return nil
}

// List returns all secret names with optional type filter
func (m *MemoryManager) List(ctx context.Context, secretType SecretType) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for name, secret := range m.secrets {
		if secretType == "" || secret.Type == secretType {
			names = append(names, name)
		}
	}

	return names, nil
}

// Rotate rotates a secret
func (m *MemoryManager) Rotate(ctx context.Context, name string, newValue map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret, exists := m.secrets[name]
	if !exists {
		return fmt.Errorf("secret not found: %s", name)
	}

	now := time.Now()

	// Update with new value
	secret.Value = copyMap(newValue)
	secret.UpdatedAt = now
	secret.Version = incrementVersion(secret.Version)

	return nil
}

// Close is a no-op for memory manager
func (m *MemoryManager) Close() error {
	return nil
}

// Health always returns nil for memory manager
func (m *MemoryManager) Health(ctx context.Context) error {
	return nil
}

// Helper functions

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	copy := make(map[string]string, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

func incrementVersion(version string) string {
	// Simple version increment (could be enhanced)
	var v int
	if _, err := fmt.Sscanf(version, "%d", &v); err != nil {
		// version is not a valid integer (e.g. empty or corrupt); start from 0
		v = 0
	}
	return fmt.Sprintf("%d", v+1)
}
