// SPDX-License-Identifier: Apache-2.0

// Package secrets provides secure secrets management with support for
// multiple backends including HashiCorp Vault, AWS Secrets Manager,
// and Azure Key Vault
package secrets

import (
	"context"
	"fmt"
	"time"
)

// SecretType represents the type of secret
type SecretType string

const (
	// SecretTypeVCenter represents vCenter credentials
	SecretTypeVCenter SecretType = "vcenter"
	// SecretTypeAWS represents AWS credentials
	SecretTypeAWS SecretType = "aws"
	// SecretTypeAzure represents Azure credentials
	SecretTypeAzure SecretType = "azure"
	// SecretTypeGCP represents GCP credentials
	SecretTypeGCP SecretType = "gcp"
	// SecretTypeDatabase represents database credentials
	SecretTypeDatabase SecretType = "database"
	// SecretTypeAPIKey represents API keys
	SecretTypeAPIKey SecretType = "api_key"
	// SecretTypeSSHKey represents SSH keys
	SecretTypeSSHKey SecretType = "ssh_key"
	// SecretTypeTLS represents TLS certificates and keys
	SecretTypeTLS SecretType = "tls"
)

// Secret represents a secret with metadata
type Secret struct {
	// Name is the unique identifier for this secret
	Name string
	// Type is the category of this secret
	Type SecretType
	// Value is the actual secret data
	Value map[string]string
	// Version is the secret version
	Version string
	// CreatedAt is when the secret was created
	CreatedAt time.Time
	// UpdatedAt is when the secret was last updated
	UpdatedAt time.Time
	// Metadata contains additional key-value pairs
	Metadata map[string]string
}

// SecretManager interface defines the contract for secret management backends
type SecretManager interface {
	// Get retrieves a secret by name
	Get(ctx context.Context, name string) (*Secret, error)

	// Set stores or updates a secret
	Set(ctx context.Context, secret *Secret) error

	// Delete removes a secret
	Delete(ctx context.Context, name string) error

	// List returns all secret names (not values) with optional type filter
	List(ctx context.Context, secretType SecretType) ([]string, error)

	// Rotate rotates a secret (creates new version, keeps old for grace period)
	Rotate(ctx context.Context, name string, newValue map[string]string) error

	// Close cleans up any resources
	Close() error

	// Health checks if the backend is accessible
	Health(ctx context.Context) error
}

// Config holds configuration for secret managers
type Config struct {
	// Backend specifies which backend to use (vault, aws, azure, memory)
	Backend string

	// Vault specific configuration
	Vault *VaultConfig

	// AWS specific configuration
	AWS *AWSSecretsManagerConfig

	// Azure specific configuration
	Azure *AzureKeyVaultConfig

	// CacheDuration specifies how long to cache secrets (0 = no cache)
	CacheDuration time.Duration

	// RefreshInterval specifies how often to refresh cached secrets
	RefreshInterval time.Duration
}

// VaultConfig holds HashiCorp Vault configuration
type VaultConfig struct {
	// Address is the Vault server address
	Address string
	// Token is the authentication token
	Token string
	// Mount is the KV mount path (default: "secret")
	Mount string
	// Namespace is the Vault namespace (Enterprise feature)
	Namespace string
	// TLSConfig for secure connections
	TLSSkipVerify bool
	CACert        string
}

// AWSSecretsManagerConfig holds AWS Secrets Manager configuration
type AWSSecretsManagerConfig struct {
	// Region is the AWS region
	Region string
	// AccessKeyID for authentication
	AccessKeyID string
	// SecretAccessKey for authentication
	SecretAccessKey string
	// SessionToken for temporary credentials
	SessionToken string
	// KMSKeyID for encryption (optional)
	KMSKeyID string
}

// AzureKeyVaultConfig holds Azure Key Vault configuration
type AzureKeyVaultConfig struct {
	// VaultURL is the Key Vault URL (e.g., https://myvault.vault.azure.net/)
	VaultURL string
	// TenantID is the Azure AD tenant ID
	TenantID string
	// ClientID is the service principal client ID
	ClientID string
	// ClientSecret is the service principal secret
	ClientSecret string
}

// NewSecretManager creates a new secret manager based on configuration
func NewSecretManager(config *Config) (SecretManager, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	switch config.Backend {
	case "vault":
		if config.Vault == nil {
			return nil, fmt.Errorf("vault config is required for vault backend")
		}
		return NewVaultManager(config.Vault)

	case "aws":
		if config.AWS == nil {
			return nil, fmt.Errorf("aws config is required for aws backend")
		}
		return NewAWSSecretsManager(config.AWS)

	case "azure":
		if config.Azure == nil {
			return nil, fmt.Errorf("azure config is required for azure backend")
		}
		return NewAzureKeyVaultManager(config.Azure)

	case "memory":
		// In-memory store for testing/development
		return NewMemoryManager(), nil

	default:
		return nil, fmt.Errorf("unsupported backend: %s (supported: vault, aws, azure, memory)", config.Backend)
	}
}

// CachedSecretManager wraps any SecretManager with caching
type CachedSecretManager struct {
	backend       SecretManager
	cache         map[string]*cachedSecret
	cacheDuration time.Duration
}

type cachedSecret struct {
	secret    *Secret
	expiresAt time.Time
}

// NewCachedSecretManager creates a caching wrapper around a secret manager
func NewCachedSecretManager(backend SecretManager, cacheDuration time.Duration) *CachedSecretManager {
	return &CachedSecretManager{
		backend:       backend,
		cache:         make(map[string]*cachedSecret),
		cacheDuration: cacheDuration,
	}
}

// Get retrieves a secret, using cache if available
func (c *CachedSecretManager) Get(ctx context.Context, name string) (*Secret, error) {
	// Check cache
	if cached, ok := c.cache[name]; ok {
		if time.Now().Before(cached.expiresAt) {
			return cached.secret, nil
		}
		// Expired, remove from cache
		delete(c.cache, name)
	}

	// Fetch from backend
	secret, err := c.backend.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.cache[name] = &cachedSecret{
		secret:    secret,
		expiresAt: time.Now().Add(c.cacheDuration),
	}

	return secret, nil
}

// Set stores a secret and invalidates cache
func (c *CachedSecretManager) Set(ctx context.Context, secret *Secret) error {
	// Invalidate cache
	delete(c.cache, secret.Name)

	return c.backend.Set(ctx, secret)
}

// Delete removes a secret and invalidates cache
func (c *CachedSecretManager) Delete(ctx context.Context, name string) error {
	// Invalidate cache
	delete(c.cache, name)

	return c.backend.Delete(ctx, name)
}

// List delegates to backend (cache not applicable)
func (c *CachedSecretManager) List(ctx context.Context, secretType SecretType) ([]string, error) {
	return c.backend.List(ctx, secretType)
}

// Rotate delegates to backend and invalidates cache
func (c *CachedSecretManager) Rotate(ctx context.Context, name string, newValue map[string]string) error {
	// Invalidate cache
	delete(c.cache, name)

	return c.backend.Rotate(ctx, name, newValue)
}

// Close clears cache and closes backend
func (c *CachedSecretManager) Close() error {
	c.cache = make(map[string]*cachedSecret)
	return c.backend.Close()
}

// Health delegates to backend
func (c *CachedSecretManager) Health(ctx context.Context) error {
	return c.backend.Health(ctx)
}

// InvalidateCache clears all cached secrets
func (c *CachedSecretManager) InvalidateCache() {
	c.cache = make(map[string]*cachedSecret)
}

// InvalidateSecret removes a specific secret from cache
func (c *CachedSecretManager) InvalidateSecret(name string) {
	delete(c.cache, name)
}
