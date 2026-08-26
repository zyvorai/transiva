// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	vault "github.com/hashicorp/vault/api"
)

// VaultManager manages secrets using HashiCorp Vault
type VaultManager struct {
	client *vault.Client
	mount  string
}

// NewVaultManager creates a new Vault secret manager
func NewVaultManager(config *VaultConfig) (*VaultManager, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("vault address is required")
	}

	if config.Token == "" {
		return nil, fmt.Errorf("vault token is required")
	}

	// Create Vault config
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = config.Address

	// Configure TLS
	if config.TLSSkipVerify {
		// #nosec G402 -- TLS verification is only disabled when the operator explicitly
		// opts in via config.TLSSkipVerify (e.g. local dev with self-signed certs); default is verified TLS.
		vaultConfig.HttpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	// Create client
	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	// Set token
	client.SetToken(config.Token)

	// Set namespace if provided (Vault Enterprise)
	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	}

	// Default mount path
	mount := config.Mount
	if mount == "" {
		mount = "secret"
	}

	return &VaultManager{
		client: client,
		mount:  mount,
	}, nil
}

// Get retrieves a secret from Vault
func (v *VaultManager) Get(ctx context.Context, name string) (*Secret, error) {
	path := v.secretPath(name)

	// Read from KV v2
	secret, err := v.client.KVv2(v.mount).Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret not found: %s", name)
	}

	// Convert vault secret to our Secret type
	value := make(map[string]string)
	for k, v := range secret.Data {
		// Try to convert to string
		var str string
		switch val := v.(type) {
		case string:
			str = val
		case interface{}:
			// Convert other types to string
			str = fmt.Sprintf("%v", val)
		}
		if str != "" {
			value[k] = str
		}
	}

	metadata := make(map[string]string)
	if secret.CustomMetadata != nil {
		for k, v := range secret.CustomMetadata {
			if str, ok := v.(string); ok {
				metadata[k] = str
			}
		}
	}

	secretType := SecretType("")
	if typeStr, ok := metadata["type"]; ok {
		secretType = SecretType(typeStr)
	}

	return &Secret{
		Name:      name,
		Type:      secretType,
		Value:     value,
		Version:   fmt.Sprintf("%d", secret.VersionMetadata.Version),
		CreatedAt: secret.VersionMetadata.CreatedTime,
		UpdatedAt: secret.VersionMetadata.CreatedTime,
		Metadata:  metadata,
	}, nil
}

// Set stores or updates a secret in Vault
func (v *VaultManager) Set(ctx context.Context, secret *Secret) error {
	if secret.Name == "" {
		return fmt.Errorf("secret name is required")
	}

	path := v.secretPath(secret.Name)

	// Convert to vault format
	data := make(map[string]interface{})
	for k, v := range secret.Value {
		data[k] = v
	}

	// Add metadata
	metadata := make(map[string]string)
	if secret.Metadata != nil {
		for k, v := range secret.Metadata {
			metadata[k] = v
		}
	}
	if secret.Type != "" {
		metadata["type"] = string(secret.Type)
	}

	// Write to KV v2
	_, err := v.client.KVv2(v.mount).Put(ctx, path, data)
	if err != nil {
		return fmt.Errorf("failed to write secret: %w", err)
	}

	// Update custom metadata if provided
	if len(metadata) > 0 {
		// Convert metadata to interface{} map
		customMeta := make(map[string]interface{})
		for k, v := range metadata {
			customMeta[k] = v
		}

		patchInput := vault.KVMetadataPatchInput{
			CustomMetadata: customMeta,
		}

		err = v.client.KVv2(v.mount).PatchMetadata(ctx, path, patchInput)
		if err != nil {
			// Don't fail on metadata error
			fmt.Printf("warning: failed to update metadata: %v\n", err)
		}
	}

	return nil
}

// Delete removes a secret from Vault
func (v *VaultManager) Delete(ctx context.Context, name string) error {
	path := v.secretPath(name)

	// Delete all versions (KV v2)
	err := v.client.KVv2(v.mount).DeleteMetadata(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// List returns all secret names from Vault
func (v *VaultManager) List(ctx context.Context, secretType SecretType) ([]string, error) {
	// List secrets using metadata list
	metadataPath := v.mount + "/metadata/"
	secret, err := v.client.Logical().List(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return []string{}, nil
	}

	// Extract keys from response
	keysInterface, ok := secret.Data["keys"]
	if !ok {
		return []string{}, nil
	}

	keys, ok := keysInterface.([]interface{})
	if !ok {
		return []string{}, nil
	}

	var names []string
	for _, keyInterface := range keys {
		if key, ok := keyInterface.(string); ok {
			// If type filter specified, we need to fetch each secret to check type
			if secretType != "" {
				secretData, err := v.Get(ctx, key)
				if err != nil {
					continue // Skip inaccessible secrets
				}
				if secretData.Type == secretType {
					names = append(names, key)
				}
			} else {
				names = append(names, key)
			}
		}
	}

	return names, nil
}

// Rotate rotates a secret in Vault
func (v *VaultManager) Rotate(ctx context.Context, name string, newValue map[string]string) error {
	// Get existing secret to preserve metadata
	existing, err := v.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get existing secret: %w", err)
	}

	// Update with new value
	existing.Value = newValue
	existing.UpdatedAt = time.Now()

	return v.Set(ctx, existing)
}

// Close cleans up Vault client
func (v *VaultManager) Close() error {
	// Vault client doesn't need explicit cleanup
	return nil
}

// Health checks Vault connectivity
func (v *VaultManager) Health(ctx context.Context) error {
	health, err := v.client.Sys().Health()
	if err != nil {
		return fmt.Errorf("vault health check failed: %w", err)
	}

	if !health.Initialized {
		return fmt.Errorf("vault is not initialized")
	}

	if health.Sealed {
		return fmt.Errorf("vault is sealed")
	}

	return nil
}

// secretPath builds the full path for a secret
func (v *VaultManager) secretPath(name string) string {
	// Remove mount prefix if present in name
	name = strings.TrimPrefix(name, v.mount+"/")
	return name
}
