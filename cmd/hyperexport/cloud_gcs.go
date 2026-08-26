// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// GCSStorage implements CloudStorage for Google Cloud Storage
type GCSStorage struct {
	client  *storage.Client
	bucket  string
	prefix  string
	log     logger.Logger
	retryer *retry.Retryer
}

// NewGCSStorage creates a new GCS storage client
func NewGCSStorage(cfg *CloudStorageConfig, log logger.Logger) (*GCSStorage, error) {
	ctx := context.Background()

	var client *storage.Client
	var err error

	// Check for service account credentials
	credsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credsFile != "" {
		client, err = storage.NewClient(ctx, option.WithAuthCredentialsFile(option.ServiceAccount, credsFile))
	} else {
		// Use default credentials
		client, err = storage.NewClient(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("create GCS client: %w", err)
	}

	// Initialize retryer with config or defaults
	retryer := retry.NewRetryer(cfg.RetryConfig, log)

	return &GCSStorage{
		client:  client,
		bucket:  cfg.Bucket,
		prefix:  cfg.Prefix,
		log:     log,
		retryer: retryer,
	}, nil
}

// Upload uploads a file to GCS
func (g *GCSStorage) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	objectName := g.buildObjectName(remotePath)

	return g.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		// #nosec G304 -- localPath is supplied by the local operator via CLI/config, not remote input
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
			g.log.Info("uploading to GCS", "bucket", g.bucket, "object", objectName, "size", fileInfo.Size())
		} else {
			g.log.Info("retrying GCS upload", "bucket", g.bucket, "object", objectName, "attempt", attempt)
		}

		// Get object handle
		obj := g.client.Bucket(g.bucket).Object(objectName)
		writer := obj.NewWriter(ctx)

		// Wrap reader with progress tracking
		reader := &progressReader{
			reader:   file,
			size:     fileInfo.Size(),
			callback: progress,
		}

		// Copy data
		if _, err := io.Copy(writer, reader); err != nil {
			if closeErr := writer.Close(); closeErr != nil {
				g.log.Warn("failed to close GCS writer after copy error", "error", closeErr)
			}
			return fmt.Errorf("copy to GCS: %w", err)
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("close writer: %w", err)
		}

		return nil
	}, fmt.Sprintf("GCS upload %s", objectName))
}

// UploadStream uploads data from a reader to GCS
func (g *GCSStorage) UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error {
	objectName := g.buildObjectName(remotePath)

	g.log.Info("uploading stream to GCS", "bucket", g.bucket, "object", objectName, "size", size)

	// Get object handle
	obj := g.client.Bucket(g.bucket).Object(objectName)
	writer := obj.NewWriter(ctx)

	// Wrap reader with progress tracking
	progressReader := &progressReader{
		reader:   reader,
		size:     size,
		callback: progress,
	}

	// Copy data
	if _, err := io.Copy(writer, progressReader); err != nil {
		if closeErr := writer.Close(); closeErr != nil {
			g.log.Warn("failed to close GCS writer after copy error", "error", closeErr)
		}
		return fmt.Errorf("copy to GCS: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}

	return nil
}

// Download downloads a file from GCS
func (g *GCSStorage) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	objectName := g.buildObjectName(remotePath)

	return g.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			g.log.Info("downloading from GCS", "bucket", g.bucket, "object", objectName)
		} else {
			g.log.Info("retrying GCS download", "bucket", g.bucket, "object", objectName, "attempt", attempt)
		}

		// Get object handle
		obj := g.client.Bucket(g.bucket).Object(objectName)
		reader, err := obj.NewReader(ctx)
		if err != nil {
			// Object not found is not retryable
			if err == storage.ErrObjectNotExist {
				return retry.IsNonRetryable(fmt.Errorf("object not found: %w", err))
			}
			return fmt.Errorf("create reader: %w", err)
		}
		defer func() {
			if cerr := reader.Close(); cerr != nil {
				g.log.Warn("failed to close GCS reader", "error", cerr)
			}
		}()

		// Create local file
		// #nosec G304 -- localPath is supplied by the local operator via CLI/config, not remote input
		file, err := os.Create(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("create file: %w", err))
		}
		defer file.Close()

		// Get size
		attrs, err := obj.Attrs(ctx)
		size := int64(0)
		if err == nil {
			size = attrs.Size
		}

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := reader.Read(buf)
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
					return fmt.Errorf("read from GCS: %w", er)
				}
				break
			}
		}

		return nil
	}, fmt.Sprintf("GCS download %s", objectName))
}

// List lists objects in GCS with a prefix
func (g *GCSStorage) List(ctx context.Context, prefix string) ([]CloudFile, error) {
	objectPrefix := g.buildObjectName(prefix)

	result, err := g.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt == 1 {
			g.log.Debug("listing GCS objects", "bucket", g.bucket, "prefix", objectPrefix)
		} else {
			g.log.Debug("retrying GCS list", "bucket", g.bucket, "prefix", objectPrefix, "attempt", attempt)
		}

		var files []CloudFile
		query := &storage.Query{Prefix: objectPrefix}
		it := g.client.Bucket(g.bucket).Objects(ctx, query)

		for {
			attrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("iterate objects: %w", err)
			}

			files = append(files, CloudFile{
				Path:         attrs.Name,
				Size:         attrs.Size,
				LastModified: attrs.Updated,
				ETag:         attrs.Etag,
			})
		}

		return files, nil
	}, fmt.Sprintf("GCS list %s", objectPrefix))

	if err != nil {
		return nil, err
	}

	return result.([]CloudFile), nil
}

// Delete deletes an object from GCS
func (g *GCSStorage) Delete(ctx context.Context, remotePath string) error {
	objectName := g.buildObjectName(remotePath)

	return g.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			g.log.Info("deleting from GCS", "bucket", g.bucket, "object", objectName)
		} else {
			g.log.Info("retrying GCS delete", "bucket", g.bucket, "object", objectName, "attempt", attempt)
		}

		obj := g.client.Bucket(g.bucket).Object(objectName)
		if err := obj.Delete(ctx); err != nil {
			return fmt.Errorf("delete object: %w", err)
		}

		return nil
	}, fmt.Sprintf("GCS delete %s", objectName))
}

// Exists checks if an object exists in GCS
func (g *GCSStorage) Exists(ctx context.Context, remotePath string) (bool, error) {
	objectName := g.buildObjectName(remotePath)

	result, err := g.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		obj := g.client.Bucket(g.bucket).Object(objectName)
		_, err := obj.Attrs(ctx)
		if err != nil {
			if err == storage.ErrObjectNotExist {
				return false, nil
			}
			return false, fmt.Errorf("get object attrs: %w", err)
		}

		return true, nil
	}, fmt.Sprintf("GCS exists %s", objectName))

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// GetURL returns the GCS URL
func (g *GCSStorage) GetURL() string {
	return fmt.Sprintf("gs://%s/%s", g.bucket, g.prefix)
}

// Close closes the GCS client
func (g *GCSStorage) Close() error {
	return g.client.Close()
}

// buildObjectName builds the full object name with prefix
func (g *GCSStorage) buildObjectName(remotePath string) string {
	if g.prefix == "" {
		return remotePath
	}
	return filepath.ToSlash(filepath.Join(g.prefix, remotePath))
}
