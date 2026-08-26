// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/objects"
	"github.com/gophercloud/gophercloud/pagination"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// OpenStackSwiftStorage implements CloudStorage for OpenStack Swift
type OpenStackSwiftStorage struct {
	client    *gophercloud.ServiceClient
	container string
	prefix    string
	log       logger.Logger
	retryer   *retry.Retryer
}

// NewOpenStackSwiftStorage creates a new OpenStack Swift storage client
func NewOpenStackSwiftStorage(cfg *CloudStorageConfig, log logger.Logger) (*OpenStackSwiftStorage, error) {
	// Validate required config
	if cfg.SwiftAuthURL == "" {
		return nil, fmt.Errorf("Swift auth URL is required")
	}
	if cfg.SwiftUsername == "" {
		return nil, fmt.Errorf("Swift username is required")
	}
	if cfg.SwiftPassword == "" {
		return nil, fmt.Errorf("Swift password is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("container name is required")
	}

	// Create authentication options
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: cfg.SwiftAuthURL,
		Username:         cfg.SwiftUsername,
		Password:         cfg.SwiftPassword,
		TenantName:       cfg.SwiftTenantName,
		DomainName:       cfg.SwiftDomainName,
	}

	if cfg.SwiftDomainName == "" {
		authOpts.DomainName = "Default"
	}

	// Create provider client
	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		return nil, fmt.Errorf("authenticate to Swift: %w", err)
	}

	// Create object storage client
	client, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create Swift client: %w", err)
	}

	// Initialize retryer with config or defaults
	retryer := retry.NewRetryer(cfg.RetryConfig, log)

	return &OpenStackSwiftStorage{
		client:    client,
		container: cfg.Bucket,
		prefix:    cfg.Prefix,
		log:       log,
		retryer:   retryer,
	}, nil
}

// Upload uploads a file to Swift
func (s *OpenStackSwiftStorage) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	objectName := s.buildObjectName(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		// #nosec G304 -- localPath comes from the local operator-supplied export output directory (CLI flag), not remote input
		file, err := os.Open(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("open file: %w", err))
		}
		defer file.Close()

		fileInfo, err := file.Stat()
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("stat file: %w", err))
		}

		if attempt == 1 {
			s.log.Info("uploading to Swift",
				"container", s.container,
				"object", objectName,
				"size", fileInfo.Size())
		} else {
			s.log.Info("retrying Swift upload",
				"container", s.container,
				"object", objectName,
				"attempt", attempt)
		}

		// Wrap reader with progress tracking
		var reader io.Reader
		if progress != nil {
			reader = &progressReader{
				reader:   file,
				size:     fileInfo.Size(),
				read:     0,
				callback: progress,
			}
		} else {
			reader = file
		}

		// Upload object
		createOpts := objects.CreateOpts{
			Content: reader,
		}

		result := objects.Create(s.client, s.container, objectName, createOpts)
		if result.Err != nil {
			return fmt.Errorf("upload to Swift: %w", result.Err)
		}

		return nil
	}, fmt.Sprintf("Swift upload %s", objectName))
}

// UploadStream uploads data from a reader to Swift
func (s *OpenStackSwiftStorage) UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error {
	objectName := s.buildObjectName(remotePath)

	s.log.Info("uploading stream to Swift",
		"container", s.container,
		"object", objectName,
		"size", size)

	// Wrap reader with progress tracking
	var uploadReader io.Reader
	if progress != nil {
		uploadReader = &progressReader{
			reader:   reader,
			size:     size,
			read:     0,
			callback: progress,
		}
	} else {
		uploadReader = reader
	}

	createOpts := objects.CreateOpts{
		Content: uploadReader,
	}

	result := objects.Create(s.client, s.container, objectName, createOpts)
	if result.Err != nil {
		return fmt.Errorf("upload stream to Swift: %w", result.Err)
	}

	return nil
}

// Download downloads a file from Swift
func (s *OpenStackSwiftStorage) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	objectName := s.buildObjectName(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			s.log.Info("downloading from Swift",
				"container", s.container,
				"object", objectName)
		} else {
			s.log.Info("retrying Swift download",
				"container", s.container,
				"object", objectName,
				"attempt", attempt)
		}

		// Download object
		downloadResult := objects.Download(s.client, s.container, objectName, nil)
		if downloadResult.Err != nil {
			// Not found errors are not retryable
			if _, ok := downloadResult.Err.(gophercloud.ErrDefault404); ok {
				return retry.IsNonRetryable(fmt.Errorf("object not found: %w", downloadResult.Err))
			}
			return fmt.Errorf("download from Swift: %w", downloadResult.Err)
		}

		content, err := downloadResult.ExtractContent()
		if err != nil {
			return fmt.Errorf("extract content: %w", err)
		}

		// Create local file
		// #nosec G304 -- localPath comes from the local operator-supplied download destination (CLI flag), not remote input
		file, err := os.Create(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("create file: %w", err))
		}
		defer file.Close()

		// Get size from headers
		size := int64(len(content))

		// Write with progress
		if progress != nil {
			// For small objects, content is already in memory
			// So we can't track progress during write
			// Just report 100% at the end
			if _, err := file.Write(content); err != nil {
				return retry.IsNonRetryable(fmt.Errorf("write file: %w", err))
			}
			progress(size, size)
		} else {
			if _, err := file.Write(content); err != nil {
				return retry.IsNonRetryable(fmt.Errorf("write file: %w", err))
			}
		}

		return nil
	}, fmt.Sprintf("Swift download %s", objectName))
}

// List lists objects in Swift with a prefix
func (s *OpenStackSwiftStorage) List(ctx context.Context, prefix string) ([]CloudFile, error) {
	objectPrefix := s.buildObjectName(prefix)

	result, err := s.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt == 1 {
			s.log.Debug("listing Swift objects",
				"container", s.container,
				"prefix", objectPrefix)
		} else {
			s.log.Debug("retrying Swift list",
				"container", s.container,
				"prefix", objectPrefix,
				"attempt", attempt)
		}

		var files []CloudFile

		listOpts := objects.ListOpts{
			Prefix: objectPrefix,
		}

		err := objects.List(s.client, s.container, listOpts).EachPage(func(page pagination.Page) (bool, error) {
			objectList, err := objects.ExtractInfo(page)
			if err != nil {
				return false, err
			}

			for _, obj := range objectList {
				files = append(files, CloudFile{
					Path:         obj.Name,
					Size:         obj.Bytes,
					LastModified: obj.LastModified,
					ETag:         strings.Trim(obj.Hash, "\""),
				})
			}

			return true, nil
		})

		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		return files, nil
	}, fmt.Sprintf("Swift list %s", objectPrefix))

	if err != nil {
		return nil, err
	}

	return result.([]CloudFile), nil
}

// Delete deletes an object from Swift
func (s *OpenStackSwiftStorage) Delete(ctx context.Context, remotePath string) error {
	objectName := s.buildObjectName(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			s.log.Info("deleting from Swift",
				"container", s.container,
				"object", objectName)
		} else {
			s.log.Info("retrying Swift delete",
				"container", s.container,
				"object", objectName,
				"attempt", attempt)
		}

		result := objects.Delete(s.client, s.container, objectName, nil)
		if result.Err != nil {
			// Ignore not found errors on delete
			if _, ok := result.Err.(gophercloud.ErrDefault404); ok {
				return nil
			}
			return fmt.Errorf("delete object: %w", result.Err)
		}

		return nil
	}, fmt.Sprintf("Swift delete %s", objectName))
}

// Exists checks if an object exists in Swift
func (s *OpenStackSwiftStorage) Exists(ctx context.Context, remotePath string) (bool, error) {
	objectName := s.buildObjectName(remotePath)

	result, err := s.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		getResult := objects.Get(s.client, s.container, objectName, nil)
		if getResult.Err != nil {
			// Check if it's a not found error
			if _, ok := getResult.Err.(gophercloud.ErrDefault404); ok {
				return false, nil
			}
			return false, fmt.Errorf("get object: %w", getResult.Err)
		}

		return true, nil
	}, fmt.Sprintf("Swift exists %s", objectName))

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// GetURL returns the Swift storage URL
func (s *OpenStackSwiftStorage) GetURL() string {
	return fmt.Sprintf("swift://%s/%s", s.container, s.prefix)
}

// Close closes the Swift client (no-op for Swift)
func (s *OpenStackSwiftStorage) Close() error {
	return nil
}

// buildObjectName builds the full object name with prefix
func (s *OpenStackSwiftStorage) buildObjectName(remotePath string) string {
	if s.prefix == "" {
		return remotePath
	}
	return filepath.ToSlash(filepath.Join(s.prefix, remotePath))
}
