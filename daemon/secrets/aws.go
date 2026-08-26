// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// AWSSecretsManager manages secrets using AWS Secrets Manager
type AWSSecretsManager struct {
	client *secretsmanager.Client
	config *AWSSecretsManagerConfig
}

// NewAWSSecretsManager creates a new AWS Secrets Manager client
func NewAWSSecretsManager(cfg *AWSSecretsManagerConfig) (*AWSSecretsManager, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("aws region is required")
	}

	ctx := context.Background()

	// Load AWS config
	var awsCfg aws.Config
	var err error

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use provided credentials
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				cfg.SessionToken,
			)),
		)
	} else {
		// Use default credential chain
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := secretsmanager.NewFromConfig(awsCfg)

	return &AWSSecretsManager{
		client: client,
		config: cfg,
	}, nil
}

// Get retrieves a secret from AWS Secrets Manager
func (a *AWSSecretsManager) Get(ctx context.Context, name string) (*Secret, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	result, err := a.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	// Parse secret string (expected to be JSON)
	var value map[string]string
	if result.SecretString != nil {
		if err := json.Unmarshal([]byte(*result.SecretString), &value); err != nil {
			// If not JSON, store as single "value" key
			value = map[string]string{"value": *result.SecretString}
		}
	} else if result.SecretBinary != nil {
		value = map[string]string{"value": string(result.SecretBinary)}
	} else {
		return nil, fmt.Errorf("secret has no value")
	}

	// Extract metadata from tags
	metadata := make(map[string]string)
	secretType := SecretType("")

	// Get secret metadata including tags
	describeInput := &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(name),
	}

	describeResult, err := a.client.DescribeSecret(ctx, describeInput)
	if err == nil && describeResult.Tags != nil {
		for _, tag := range describeResult.Tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == "type" {
					secretType = SecretType(*tag.Value)
				}
				metadata[*tag.Key] = *tag.Value
			}
		}
	}

	createdAt := time.Now()
	updatedAt := time.Now()
	if describeResult.CreatedDate != nil {
		createdAt = *describeResult.CreatedDate
	}
	if describeResult.LastChangedDate != nil {
		updatedAt = *describeResult.LastChangedDate
	}

	version := ""
	if result.VersionId != nil {
		version = *result.VersionId
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

// Set stores or updates a secret in AWS Secrets Manager
func (a *AWSSecretsManager) Set(ctx context.Context, secret *Secret) error {
	if secret.Name == "" {
		return fmt.Errorf("secret name is required")
	}

	// Convert value to JSON string
	secretString, err := json.Marshal(secret.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal secret value: %w", err)
	}

	// Prepare tags from metadata
	var tags []types.Tag
	if secret.Metadata != nil {
		for k, v := range secret.Metadata {
			tags = append(tags, types.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
	}
	if secret.Type != "" {
		tags = append(tags, types.Tag{
			Key:   aws.String("type"),
			Value: aws.String(string(secret.Type)),
		})
	}

	// Check if secret exists
	_, err = a.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secret.Name),
	})

	if err != nil {
		// Secret doesn't exist, create it
		createInput := &secretsmanager.CreateSecretInput{
			Name:         aws.String(secret.Name),
			SecretString: aws.String(string(secretString)),
			Tags:         tags,
		}

		if a.config.KMSKeyID != "" {
			createInput.KmsKeyId = aws.String(a.config.KMSKeyID)
		}

		_, err = a.client.CreateSecret(ctx, createInput)
		if err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
	} else {
		// Secret exists, update it
		updateInput := &secretsmanager.PutSecretValueInput{
			SecretId:     aws.String(secret.Name),
			SecretString: aws.String(string(secretString)),
		}

		_, err = a.client.PutSecretValue(ctx, updateInput)
		if err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}

		// Update tags
		if len(tags) > 0 {
			_, err = a.client.TagResource(ctx, &secretsmanager.TagResourceInput{
				SecretId: aws.String(secret.Name),
				Tags:     tags,
			})
			if err != nil {
				// Don't fail on tag update error
				fmt.Printf("warning: failed to update tags: %v\n", err)
			}
		}
	}

	return nil
}

// Delete removes a secret from AWS Secrets Manager
func (a *AWSSecretsManager) Delete(ctx context.Context, name string) error {
	input := &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(name),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	}

	_, err := a.client.DeleteSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// List returns all secret names from AWS Secrets Manager
func (a *AWSSecretsManager) List(ctx context.Context, secretType SecretType) ([]string, error) {
	input := &secretsmanager.ListSecretsInput{}

	// Add type filter if specified
	if secretType != "" {
		input.Filters = []types.Filter{
			{
				Key:    types.FilterNameStringTypeTagKey,
				Values: []string{"type"},
			},
			{
				Key:    types.FilterNameStringTypeTagValue,
				Values: []string{string(secretType)},
			},
		}
	}

	var names []string
	paginator := secretsmanager.NewListSecretsPaginator(a.client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, secret := range output.SecretList {
			if secret.Name != nil {
				names = append(names, *secret.Name)
			}
		}
	}

	return names, nil
}

// Rotate rotates a secret in AWS Secrets Manager
func (a *AWSSecretsManager) Rotate(ctx context.Context, name string, newValue map[string]string) error {
	// Get existing secret to preserve metadata
	existing, err := a.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get existing secret: %w", err)
	}

	// Update with new value
	existing.Value = newValue

	// Convert to JSON
	secretString, err := json.Marshal(newValue)
	if err != nil {
		return fmt.Errorf("failed to marshal secret value: %w", err)
	}

	// Put new secret version
	input := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretString: aws.String(string(secretString)),
	}

	_, err = a.client.PutSecretValue(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to rotate secret: %w", err)
	}

	return nil
}

// Close cleans up AWS Secrets Manager client
func (a *AWSSecretsManager) Close() error {
	// AWS SDK clients don't need explicit cleanup
	return nil
}

// Health checks AWS Secrets Manager connectivity
func (a *AWSSecretsManager) Health(ctx context.Context) error {
	// Try to list secrets (with limit 1) to verify connectivity
	input := &secretsmanager.ListSecretsInput{
		MaxResults: aws.Int32(1),
	}

	_, err := a.client.ListSecrets(ctx, input)
	if err != nil {
		return fmt.Errorf("aws secrets manager health check failed: %w", err)
	}

	return nil
}
