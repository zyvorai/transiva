// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// AzureStorage implements CloudStorage for Azure Blob Storage
type AzureStorage struct {
	client    *azblob.Client
	container string
	prefix    string
	log       logger.Logger
	retryer   *retry.Retryer
}

// NewAzureStorage creates a new Azure storage client
func NewAzureStorage(cfg *CloudStorageConfig, log logger.Logger) (*AzureStorage, error) {
	// Build storage URL
	accountURL := fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccessKey)

	var client *azblob.Client
	var err error

	// Use shared key authentication if secret key is provided
	if cfg.SecretKey != "" {
		cred, credErr := azblob.NewSharedKeyCredential(cfg.AccessKey, cfg.SecretKey)
		if credErr != nil {
			return nil, fmt.Errorf("create shared key credential: %w", credErr)
		}
		client, err = azblob.NewClientWithSharedKeyCredential(accountURL, cred, nil)
	} else {
		// Use default Azure credential (managed identity, Azure CLI, etc.)
		cred, credErr := azidentity.NewDefaultAzureCredential(nil)
		if credErr != nil {
			return nil, fmt.Errorf("create default credential: %w", credErr)
		}
		client, err = azblob.NewClient(accountURL, cred, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("create Azure client: %w", err)
	}

	// Initialize retryer with config or defaults
	retryer := retry.NewRetryer(cfg.RetryConfig, log)

	return &AzureStorage{
		client:    client,
		container: cfg.Bucket, // container name
		prefix:    cfg.Prefix,
		log:       log,
		retryer:   retryer,
	}, nil
}

// Upload uploads a file to Azure Blob Storage
func (a *AzureStorage) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	blobName := a.buildBlobName(remotePath)

	return a.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		// #nosec G304 -- localPath is supplied by the local hyperexport CLI/config, not remote/API input
		file, err := os.Open(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("open file: %w", err))
		}
		defer func() {
			if cerr := file.Close(); cerr != nil {
				a.log.Warn("failed to close file", "error", cerr)
			}
		}()

		fileInfo, err := file.Stat()
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("stat file: %w", err))
		}

		if attempt == 1 {
			a.log.Info("uploading to Azure", "container", a.container, "blob", blobName, "size", fileInfo.Size())
		} else {
			a.log.Info("retrying Azure upload", "container", a.container, "blob", blobName, "attempt", attempt)
		}

		// Wrap reader with progress tracking
		reader := &progressReader{
			reader:   file,
			size:     fileInfo.Size(),
			callback: progress,
		}

		_, err = a.client.UploadStream(ctx, a.container, blobName, reader, nil)
		if err != nil {
			return fmt.Errorf("upload to Azure: %w", err)
		}

		return nil
	}, fmt.Sprintf("Azure upload %s", blobName))
}

// UploadStream uploads data from a reader to Azure
func (a *AzureStorage) UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error {
	blobName := a.buildBlobName(remotePath)

	a.log.Info("uploading stream to Azure", "container", a.container, "blob", blobName, "size", size)

	// Wrap reader with progress tracking
	progressReader := &progressReader{
		reader:   reader,
		size:     size,
		callback: progress,
	}

	_, err := a.client.UploadStream(ctx, a.container, blobName, progressReader, nil)
	if err != nil {
		return fmt.Errorf("upload stream to Azure: %w", err)
	}

	return nil
}

// Download downloads a file from Azure Blob Storage
func (a *AzureStorage) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	blobName := a.buildBlobName(remotePath)

	return a.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			a.log.Info("downloading from Azure", "container", a.container, "blob", blobName)
		} else {
			a.log.Info("retrying Azure download", "container", a.container, "blob", blobName, "attempt", attempt)
		}

		// Get blob client
		blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlobClient(blobName)

		// Download blob
		downloadResponse, err := blobClient.DownloadStream(ctx, nil)
		if err != nil {
			// BlobNotFound is not retryable
			if strings.Contains(err.Error(), "BlobNotFound") {
				return retry.IsNonRetryable(fmt.Errorf("blob not found: %w", err))
			}
			return fmt.Errorf("download from Azure: %w", err)
		}
		defer func() {
			if cerr := downloadResponse.Body.Close(); cerr != nil {
				a.log.Warn("failed to close download response body", "error", cerr)
			}
		}()

		// Create local file
		// #nosec G304 -- localPath is supplied by the local hyperexport CLI/config, not remote/API input
		file, err := os.Create(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("create file: %w", err))
		}
		defer func() {
			if cerr := file.Close(); cerr != nil {
				a.log.Warn("failed to close file", "error", cerr)
			}
		}()

		// Get size
		size := int64(0)
		if downloadResponse.ContentLength != nil {
			size = *downloadResponse.ContentLength
		}

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := downloadResponse.Body.Read(buf)
			if nr > 0 {
				nw, ew := file.Write(buf[0:nr])
				if nw > 0 {
					written += int64(nw)
					if progress != nil {
						progress(written, size)
					}
				}
				if ew != nil {
					return retry.IsNonRetryable(fmt.Errorf("write file: %w", ew))
				}
			}
			if er != nil {
				if er != io.EOF {
					return fmt.Errorf("read from Azure: %w", er)
				}
				break
			}
		}

		return nil
	}, fmt.Sprintf("Azure download %s", blobName))
}

// List lists blobs in Azure with a prefix
func (a *AzureStorage) List(ctx context.Context, prefix string) ([]CloudFile, error) {
	blobPrefix := a.buildBlobName(prefix)

	result, err := a.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt == 1 {
			a.log.Debug("listing Azure blobs", "container", a.container, "prefix", blobPrefix)
		} else {
			a.log.Debug("retrying Azure list", "container", a.container, "prefix", blobPrefix, "attempt", attempt)
		}

		containerClient := a.client.ServiceClient().NewContainerClient(a.container)

		var files []CloudFile
		pager := containerClient.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{
			Prefix: &blobPrefix,
		})

		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("list blobs: %w", err)
			}

			for _, blobItem := range page.Segment.BlobItems {
				files = append(files, CloudFile{
					Path:         *blobItem.Name,
					Size:         *blobItem.Properties.ContentLength,
					LastModified: *blobItem.Properties.LastModified,
					ETag:         string(*blobItem.Properties.ETag),
				})
			}
		}

		return files, nil
	}, fmt.Sprintf("Azure list %s", blobPrefix))

	if err != nil {
		return nil, err
	}

	return result.([]CloudFile), nil
}

// Delete deletes a blob from Azure
func (a *AzureStorage) Delete(ctx context.Context, remotePath string) error {
	blobName := a.buildBlobName(remotePath)

	return a.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			a.log.Info("deleting from Azure", "container", a.container, "blob", blobName)
		} else {
			a.log.Info("retrying Azure delete", "container", a.container, "blob", blobName, "attempt", attempt)
		}

		blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlobClient(blobName)

		_, err := blobClient.Delete(ctx, nil)
		if err != nil {
			return fmt.Errorf("delete blob: %w", err)
		}

		return nil
	}, fmt.Sprintf("Azure delete %s", blobName))
}

// Exists checks if a blob exists in Azure
func (a *AzureStorage) Exists(ctx context.Context, remotePath string) (bool, error) {
	blobName := a.buildBlobName(remotePath)

	result, err := a.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlobClient(blobName)

		_, err := blobClient.GetProperties(ctx, nil)
		if err != nil {
			if strings.Contains(err.Error(), "BlobNotFound") {
				return false, nil
			}
			return false, fmt.Errorf("get blob properties: %w", err)
		}

		return true, nil
	}, fmt.Sprintf("Azure exists %s", blobName))

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// GetURL returns the Azure storage URL
func (a *AzureStorage) GetURL() string {
	return fmt.Sprintf("azure://%s/%s", a.container, a.prefix)
}

// Close closes the Azure client (no-op for Azure)
func (a *AzureStorage) Close() error {
	return nil
}

// buildBlobName builds the full blob name with prefix
func (a *AzureStorage) buildBlobName(remotePath string) string {
	if a.prefix == "" {
		return remotePath
	}
	return filepath.ToSlash(filepath.Join(a.prefix, remotePath))
}
