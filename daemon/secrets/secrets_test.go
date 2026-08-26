// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewSecretManager(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "memory backend",
			config: &Config{
				Backend: "memory",
			},
			wantErr: false,
		},
		{
			name: "vault backend without config",
			config: &Config{
				Backend: "vault",
			},
			wantErr: true,
		},
		{
			name: "aws backend without config",
			config: &Config{
				Backend: "aws",
			},
			wantErr: true,
		},
		{
			name: "azure backend without config",
			config: &Config{
				Backend: "azure",
			},
			wantErr: true,
		},
		{
			name: "unsupported backend",
			config: &Config{
				Backend: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSecretManager(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSecretManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryManager_GetSet(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	secret := &Secret{
		Name: "test-secret",
		Type: SecretTypeVCenter,
		Value: map[string]string{
			"username": "admin",
			"password": "secret",
		},
		Metadata: map[string]string{
			"environment": "production",
		},
	}

	// Test Set
	err := mgr.Set(ctx, secret)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get
	retrieved, err := mgr.Get(ctx, "test-secret")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Name != secret.Name {
		t.Errorf("expected name %s, got %s", secret.Name, retrieved.Name)
	}

	if retrieved.Type != secret.Type {
		t.Errorf("expected type %s, got %s", secret.Type, retrieved.Type)
	}

	if retrieved.Value["username"] != "admin" {
		t.Errorf("expected username admin, got %s", retrieved.Value["username"])
	}

	if retrieved.Version != "1" {
		t.Errorf("expected version 1, got %s", retrieved.Version)
	}
}

func TestMemoryManager_GetNonExistent(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	_, err := mgr.Get(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent secret")
	}
}

func TestMemoryManager_Delete(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	secret := &Secret{
		Name: "to-delete",
		Type: SecretTypeAWS,
		Value: map[string]string{
			"key": "value",
		},
	}

	// Create secret
	err := mgr.Set(ctx, secret)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Delete secret
	err = mgr.Delete(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = mgr.Get(ctx, "to-delete")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestMemoryManager_List(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	// Create multiple secrets
	secrets := []*Secret{
		{
			Name:  "vcenter-1",
			Type:  SecretTypeVCenter,
			Value: map[string]string{"host": "vcenter1.example.com"},
		},
		{
			Name:  "vcenter-2",
			Type:  SecretTypeVCenter,
			Value: map[string]string{"host": "vcenter2.example.com"},
		},
		{
			Name:  "aws-1",
			Type:  SecretTypeAWS,
			Value: map[string]string{"region": "us-east-1"},
		},
	}

	for _, secret := range secrets {
		err := mgr.Set(ctx, secret)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// List all secrets
	all, err := mgr.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("expected 3 secrets, got %d", len(all))
	}

	// List by type
	vcenterSecrets, err := mgr.List(ctx, SecretTypeVCenter)
	if err != nil {
		t.Fatalf("List by type failed: %v", err)
	}

	if len(vcenterSecrets) != 2 {
		t.Errorf("expected 2 vcenter secrets, got %d", len(vcenterSecrets))
	}
}

func TestMemoryManager_Rotate(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	// Create initial secret
	secret := &Secret{
		Name: "rotatable",
		Type: SecretTypeAPIKey,
		Value: map[string]string{
			"key": "old-key",
		},
	}

	err := mgr.Set(ctx, secret)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get initial version
	initial, err := mgr.Get(ctx, "rotatable")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	initialVersion := initial.Version

	// Rotate secret
	newValue := map[string]string{
		"key": "new-key",
	}

	err = mgr.Rotate(ctx, "rotatable", newValue)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// Verify rotation
	rotated, err := mgr.Get(ctx, "rotatable")
	if err != nil {
		t.Fatalf("Get after rotate failed: %v", err)
	}

	if rotated.Value["key"] != "new-key" {
		t.Errorf("expected new-key, got %s", rotated.Value["key"])
	}

	if rotated.Version == initialVersion {
		t.Error("expected version to change after rotation")
	}
}

func TestMemoryManager_Update(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	// Create secret
	secret := &Secret{
		Name:  "updateable",
		Type:  SecretTypeDatabase,
		Value: map[string]string{"password": "old"},
	}

	err := mgr.Set(ctx, secret)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	firstCreated, _ := mgr.Get(ctx, "updateable")

	// Update secret
	time.Sleep(10 * time.Millisecond)
	secret.Value["password"] = "new"
	err = mgr.Set(ctx, secret)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	updated, err := mgr.Get(ctx, "updateable")
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}

	if updated.Value["password"] != "new" {
		t.Errorf("expected new password")
	}

	// CreatedAt should remain same
	if !updated.CreatedAt.Equal(firstCreated.CreatedAt) {
		t.Error("CreatedAt should not change on update")
	}

	// UpdatedAt should be newer
	if !updated.UpdatedAt.After(firstCreated.UpdatedAt) {
		t.Error("UpdatedAt should be newer after update")
	}

	// Version should increment
	if updated.Version != "2" {
		t.Errorf("expected version 2, got %s", updated.Version)
	}
}

func TestMemoryManager_Health(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	err := mgr.Health(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestMemoryManager_Close(t *testing.T) {
	mgr := NewMemoryManager()

	err := mgr.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestCachedSecretManager(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 100*time.Millisecond)

	secret := &Secret{
		Name:  "cached",
		Type:  SecretTypeVCenter,
		Value: map[string]string{"key": "value"},
	}

	// Set secret
	err := cached.Set(ctx, secret)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// First get - from backend
	retrieved1, err := cached.Get(ctx, "cached")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved1.Value["key"] != "value" {
		t.Error("unexpected value")
	}

	// Second get - from cache
	retrieved2, err := cached.Get(ctx, "cached")
	if err != nil {
		t.Fatalf("Cached get failed: %v", err)
	}

	if retrieved2.Value["key"] != "value" {
		t.Error("unexpected cached value")
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Get after expiry - from backend again
	retrieved3, err := cached.Get(ctx, "cached")
	if err != nil {
		t.Fatalf("Get after expiry failed: %v", err)
	}

	if retrieved3.Value["key"] != "value" {
		t.Error("unexpected value after expiry")
	}
}

func TestCachedSecretManager_InvalidateCache(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 1*time.Hour)

	secret := &Secret{
		Name:  "invalidatable",
		Type:  SecretTypeAWS,
		Value: map[string]string{"key": "old"},
	}

	// Set and cache
	if err := cached.Set(ctx, secret); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if _, err := cached.Get(ctx, "invalidatable"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Update backend directly
	secret.Value["key"] = "new"
	if err := backend.Set(ctx, secret); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Invalidate cache
	cached.InvalidateSecret("invalidatable")

	// Get should fetch new value
	retrieved, err := cached.Get(ctx, "invalidatable")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Value["key"] != "new" {
		t.Error("expected new value after cache invalidation")
	}
}

func TestSecretTypes(t *testing.T) {
	types := []SecretType{
		SecretTypeVCenter,
		SecretTypeAWS,
		SecretTypeAzure,
		SecretTypeGCP,
		SecretTypeDatabase,
		SecretTypeAPIKey,
		SecretTypeSSHKey,
		SecretTypeTLS,
	}

	if len(types) != 8 {
		t.Errorf("expected 8 secret types, got %d", len(types))
	}
}

func TestMemoryManager_SetEmptyName(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	secret := &Secret{
		Name:  "",
		Type:  SecretTypeVCenter,
		Value: map[string]string{"key": "value"},
	}

	err := mgr.Set(ctx, secret)
	if err == nil {
		t.Error("expected error for empty secret name")
	}
}

func TestMemoryManager_DeleteNonExistent(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	err := mgr.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("expected error when deleting non-existent secret")
	}
}

func TestMemoryManager_RotateNonExistent(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	err := mgr.Rotate(ctx, "non-existent", map[string]string{"key": "value"})
	if err == nil {
		t.Error("expected error when rotating non-existent secret")
	}
}

func TestMemoryManager_ListEmpty(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	names, err := mgr.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(names) != 0 {
		t.Errorf("expected 0 secrets, got %d", len(names))
	}
}

func TestMemoryManager_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	done := make(chan bool)
	errors := make(chan error, 100)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			secret := &Secret{
				Name:  fmt.Sprintf("secret-%d", id),
				Type:  SecretTypeVCenter,
				Value: map[string]string{"id": fmt.Sprintf("%d", id)},
			}
			if err := mgr.Set(ctx, secret); err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			time.Sleep(5 * time.Millisecond)
			_, err := mgr.Get(ctx, fmt.Sprintf("secret-%d", id))
			if err != nil && err.Error() != "secret not found: secret-"+fmt.Sprintf("%d", id) {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestMemoryManager_IsolationAfterCopy(t *testing.T) {
	ctx := context.Background()
	mgr := NewMemoryManager()

	original := &Secret{
		Name: "test",
		Type: SecretTypeVCenter,
		Value: map[string]string{
			"username": "admin",
			"password": "secret",
		},
		Metadata: map[string]string{
			"env": "prod",
		},
	}

	// Set the secret
	err := mgr.Set(ctx, original)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Modify original after setting
	original.Value["password"] = "modified"
	original.Metadata["env"] = "dev"

	// Get the secret - should have original values
	retrieved, err := mgr.Get(ctx, "test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Value["password"] != "secret" {
		t.Error("external modification affected stored secret")
	}

	if retrieved.Metadata["env"] != "prod" {
		t.Error("external metadata modification affected stored secret")
	}

	// Modify retrieved value
	retrieved.Value["password"] = "another-modification"

	// Get again - should still have original values
	retrieved2, err := mgr.Get(ctx, "test")
	if err != nil {
		t.Fatalf("Second get failed: %v", err)
	}

	if retrieved2.Value["password"] != "secret" {
		t.Error("modification of retrieved secret affected stored secret")
	}
}

func TestCopyMap(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  map[string]string
	}{
		{
			name:  "nil map",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty map",
			input: map[string]string{},
			want:  map[string]string{},
		},
		{
			name: "non-empty map",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copyMap(tt.input)

			if tt.input == nil && got != nil {
				t.Error("nil input should return nil")
			}

			if tt.input != nil && got == nil {
				t.Error("non-nil input should return non-nil")
			}

			if tt.input != nil && got != nil {
				if len(got) != len(tt.want) {
					t.Errorf("length mismatch: got %d, want %d", len(got), len(tt.want))
				}

				for k, v := range tt.want {
					if got[k] != v {
						t.Errorf("value mismatch for key %s: got %s, want %s", k, got[k], v)
					}
				}

				// Verify it's a copy, not the same map
				if len(tt.input) > 0 {
					for k := range got {
						got[k] = "modified"
					}
					for _, v := range tt.input {
						if v == "modified" {
							t.Error("modifying copy affected original")
						}
					}
				}
			}
		})
	}
}

func TestIncrementVersion(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"1", "2"},
		{"5", "6"},
		{"99", "100"},
		{"0", "1"},
		{"invalid", "1"}, // Falls back to 0, then increments to 1
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := incrementVersion(tt.version)
			if got != tt.want {
				t.Errorf("incrementVersion(%s) = %s, want %s", tt.version, got, tt.want)
			}
		})
	}
}

func TestCachedSecretManager_Delete(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 1*time.Hour)

	secret := &Secret{
		Name:  "to-delete",
		Type:  SecretTypeAWS,
		Value: map[string]string{"key": "value"},
	}

	// Set and cache
	if err := cached.Set(ctx, secret); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if _, err := cached.Get(ctx, "to-delete"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Delete should invalidate cache
	err := cached.Delete(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get should fail
	_, err = cached.Get(ctx, "to-delete")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestCachedSecretManager_Rotate(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 1*time.Hour)

	secret := &Secret{
		Name:  "to-rotate",
		Type:  SecretTypeAPIKey,
		Value: map[string]string{"key": "old"},
	}

	// Set and cache
	if err := cached.Set(ctx, secret); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if _, err := cached.Get(ctx, "to-rotate"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Rotate should invalidate cache
	err := cached.Rotate(ctx, "to-rotate", map[string]string{"key": "new"})
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// Get should return new value
	rotated, err := cached.Get(ctx, "to-rotate")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if rotated.Value["key"] != "new" {
		t.Error("expected new value after rotation")
	}
}

func TestCachedSecretManager_List(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 1*time.Hour)

	// Create multiple secrets
	secrets := []*Secret{
		{
			Name:  "secret-1",
			Type:  SecretTypeVCenter,
			Value: map[string]string{"key": "value1"},
		},
		{
			Name:  "secret-2",
			Type:  SecretTypeVCenter,
			Value: map[string]string{"key": "value2"},
		},
		{
			Name:  "secret-3",
			Type:  SecretTypeAWS,
			Value: map[string]string{"key": "value3"},
		},
	}

	for _, secret := range secrets {
		err := cached.Set(ctx, secret)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// List all secrets
	all, err := cached.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("expected 3 secrets, got %d", len(all))
	}

	// List by type
	vcenterSecrets, err := cached.List(ctx, SecretTypeVCenter)
	if err != nil {
		t.Fatalf("List by type failed: %v", err)
	}

	if len(vcenterSecrets) != 2 {
		t.Errorf("expected 2 vcenter secrets, got %d", len(vcenterSecrets))
	}
}

func TestCachedSecretManager_Health(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 1*time.Hour)

	err := cached.Health(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestCachedSecretManager_Close(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 1*time.Hour)

	// Add some secrets to cache
	secret := &Secret{
		Name:  "test",
		Type:  SecretTypeVCenter,
		Value: map[string]string{"key": "value"},
	}

	if err := cached.Set(ctx, secret); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if _, err := cached.Get(ctx, "test"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Close should clear cache and close backend
	err := cached.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestCachedSecretManager_InvalidateCacheAll(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryManager()
	cached := NewCachedSecretManager(backend, 1*time.Hour)

	// Create and cache multiple secrets
	secret1 := &Secret{
		Name:  "secret-1",
		Type:  SecretTypeVCenter,
		Value: map[string]string{"key": "value1"},
	}

	secret2 := &Secret{
		Name:  "secret-2",
		Type:  SecretTypeAWS,
		Value: map[string]string{"key": "value2"},
	}

	if err := cached.Set(ctx, secret1); err != nil {
		t.Fatalf("Set secret-1 failed: %v", err)
	}
	if err := cached.Set(ctx, secret2); err != nil {
		t.Fatalf("Set secret-2 failed: %v", err)
	}
	if _, err := cached.Get(ctx, "secret-1"); err != nil {
		t.Fatalf("Get secret-1 failed: %v", err)
	}
	if _, err := cached.Get(ctx, "secret-2"); err != nil {
		t.Fatalf("Get secret-2 failed: %v", err)
	}

	// Update backend directly
	secret1.Value["key"] = "new-value1"
	secret2.Value["key"] = "new-value2"
	if err := backend.Set(ctx, secret1); err != nil {
		t.Fatalf("Set secret-1 failed: %v", err)
	}
	if err := backend.Set(ctx, secret2); err != nil {
		t.Fatalf("Set secret-2 failed: %v", err)
	}

	// Invalidate entire cache
	cached.InvalidateCache()

	// Get should fetch new values from backend
	retrieved1, err := cached.Get(ctx, "secret-1")
	if err != nil {
		t.Fatalf("Get secret-1 failed: %v", err)
	}

	if retrieved1.Value["key"] != "new-value1" {
		t.Errorf("expected new-value1, got %s", retrieved1.Value["key"])
	}

	retrieved2, err := cached.Get(ctx, "secret-2")
	if err != nil {
		t.Fatalf("Get secret-2 failed: %v", err)
	}

	if retrieved2.Value["key"] != "new-value2" {
		t.Errorf("expected new-value2, got %s", retrieved2.Value["key"])
	}
}
