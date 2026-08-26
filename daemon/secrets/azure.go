// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
)

// AzureKeyVaultManager manages secrets using Azure Key Vault
type AzureKeyVaultManager struct {
	client *azsecrets.Client
	config *AzureKeyVaultConfig
}

// NewAzureKeyVaultManager creates a new Azure Key Vault manager
func NewAzureKeyVaultManager(config *AzureKeyVaultConfig) (*AzureKeyVaultManager, error) {
	if config.VaultURL == "" {
		return nil, fmt.Errorf("vault URL is required")
	}

	var cred azcore.TokenCredential
	var err error

	if config.ClientID != "" && config.ClientSecret != "" {
		// Use service principal authentication
		cred, err = azidentity.NewClientSecretCredential(
			config.TenantID,
			config.ClientID,
			config.ClientSecret,
			nil,
		)
	} else {
		// Use default Azure credentials (managed identity, Azure CLI, etc.)
		cred, err = azidentity.NewDefaultAzureCredential(nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create azure credentials: %w", err)
	}

	client, err := azsecrets.NewClient(config.VaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create key vault client: %w", err)
	}

	return &AzureKeyVaultManager{
		client: client,
		config: config,
	}, nil
}

// Get retrieves a secret from Azure Key Vault
func (a *AzureKeyVaultManager) Get(ctx context.Context, name string) (*Secret, error) {
	// Get secret value
	resp, err := a.client.GetSecret(ctx, name, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	if resp.Value == nil {
		return nil, fmt.Errorf("secret has no value")
	}

	// Parse secret value (expected to be JSON)
	var value map[string]string
	if err := json.Unmarshal([]byte(*resp.Value), &value); err != nil {
		// If not JSON, store as single "value" key
		value = map[string]string{"value": *resp.Value}
	}

	// Extract metadata from tags
	metadata := make(map[string]string)
	secretType := SecretType("")

	if resp.Tags != nil {
		for k, v := range resp.Tags {
			if v != nil {
				if k == "type" {
					secretType = SecretType(*v)
				}
				metadata[k] = *v
			}
		}
	}

	createdAt := time.Now()
	updatedAt := time.Now()
	if resp.Attributes != nil {
		if resp.Attributes.Created != nil {
			createdAt = *resp.Attributes.Created
		}
		if resp.Attributes.Updated != nil {
			updatedAt = *resp.Attributes.Updated
		}
	}

	version := ""
	if resp.ID != nil {
		// Extract version from ID (last segment of URL path)
		version = extractVersionFromID(string(*resp.ID))
	}

	return &Secret{
		Name:      name,
		Type:      secretType,
		Value:     value,
		Version:   version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Metadata:  metadata,
	}, nil
}

// Set stores or updates a secret in Azure Key Vault
func (a *AzureKeyVaultManager) Set(ctx context.Context, secret *Secret) error {
	if secret.Name == "" {
		return fmt.Errorf("secret name is required")
	}

	// Convert value to JSON string
	secretString, err := json.Marshal(secret.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal secret value: %w", err)
	}

	value := string(secretString)

	// Prepare tags from metadata
	tags := make(map[string]*string)
	if secret.Metadata != nil {
		for k, v := range secret.Metadata {
			vCopy := v
			tags[k] = &vCopy
		}
	}
	if secret.Type != "" {
		typeStr := string(secret.Type)
		tags["type"] = &typeStr
	}

	// Set secret
	params := azsecrets.SetSecretParameters{
		Value: &value,
		Tags:  tags,
	}

	_, err = a.client.SetSecret(ctx, secret.Name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to set secret: %w", err)
	}

	return nil
}

// Delete removes a secret from Azure Key Vault
func (a *AzureKeyVaultManager) Delete(ctx context.Context, name string) error {
	// Delete secret (soft delete by default)
	_, err := a.client.DeleteSecret(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Purge deleted secret (permanent delete)
	_, err = a.client.PurgeDeletedSecret(ctx, name, nil)
	if err != nil {
		// Purge might fail if soft-delete protection is enabled
		// Don't treat as error
		fmt.Printf("warning: failed to purge secret: %v\n", err)
	}

	return nil
}

// List returns all secret names from Azure Key Vault
func (a *AzureKeyVaultManager) List(ctx context.Context, secretType SecretType) ([]string, error) {
	var names []string

	pager := a.client.NewListSecretsPager(nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, item := range page.Value {
			if item.ID == nil {
				continue
			}

			// Extract secret name from ID
			name := extractSecretName(string(*item.ID))

			// Filter by type if specified
			if secretType != "" {
				if item.Tags == nil {
					continue
				}

				typeTag, ok := item.Tags["type"]
				if !ok || typeTag == nil || *typeTag != string(secretType) {
					continue
				}
			}

			names = append(names, name)
		}
	}

	return names, nil
}

// Rotate rotates a secret in Azure Key Vault
func (a *AzureKeyVaultManager) Rotate(ctx context.Context, name string, newValue map[string]string) error {
	// Get existing secret to preserve metadata
	existing, err := a.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get existing secret: %w", err)
	}

	// Update with new value
	existing.Value = newValue

	return a.Set(ctx, existing)
}

// Close cleans up Azure Key Vault client
func (a *AzureKeyVaultManager) Close() error {
	// Azure SDK clients don't need explicit cleanup
	return nil
}

// Health checks Azure Key Vault connectivity
func (a *AzureKeyVaultManager) Health(ctx context.Context) error {
	// Try to list secrets (with limit) to verify connectivity
	pager := a.client.NewListSecretsPager(nil)

	if !pager.More() {
		// If there are no secrets, that's OK for health check
		return nil
	}

	_, err := pager.NextPage(ctx)
	if err != nil {
		return fmt.Errorf("azure key vault health check failed: %w", err)
	}

	return nil
}

// Helper functions

// extractSecretName extracts the secret name from an Azure Key Vault ID
// Format: https://myvault.vault.azure.net/secrets/mysecret/version
func extractSecretName(id string) string {
	// Parse the ID to extract secret name
	// Simple implementation: split by "/" and get the second-to-last segment
	parts := splitString(id, "/")
	if len(parts) >= 2 {
		// Return the segment before version
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == "secrets" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return ""
}

// extractVersionFromID extracts the version from an Azure Key Vault secret ID
// Format: https://myvault.vault.azure.net/secrets/mysecret/version
func extractVersionFromID(id string) string {
	parts := splitString(id, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func splitString(s string, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			if start < i {
				result = append(result, s[start:i])
			}
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
