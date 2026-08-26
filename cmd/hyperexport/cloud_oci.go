// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// OCIStorage implements CloudStorage for Oracle Cloud Infrastructure Object Storage
type OCIStorage struct {
	client    objectstorage.ObjectStorageClient
	namespace string
	bucket    string
	prefix    string
	log       logger.Logger
	retryer   *retry.Retryer
}

// NewOCIStorage creates a new OCI Object Storage client
func NewOCIStorage(cfg *CloudStorageConfig, log logger.Logger) (*OCIStorage, error) {
	// Validate required config
	if cfg.OCINamespace == "" {
		return nil, fmt.Errorf("OCI namespace is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	// Create config provider
	var configProvider common.ConfigurationProvider
	var err error

	if cfg.OCIConfigPath != "" {
		// Load from config file
		configProvider, err = common.ConfigurationProviderFromFile(cfg.OCIConfigPath, cfg.OCIProfile)
		if err != nil {
			return nil, fmt.Errorf("load OCI config from file: %w", err)
		}
	} else {
		// Use manual configuration
		configProvider = common.NewRawConfigurationProvider(
			cfg.OCITenancyOCID,
			cfg.OCIUserOCID,
			cfg.Region,
			cfg.OCIFingerprint,
			cfg.OCIPrivateKey,
			nil,
		)
	}

	// Create Object Storage client
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("create OCI client: %w", err)
	}

	// Set custom endpoint if provided
	if cfg.Endpoint != "" {
		client.SetRegion(cfg.Endpoint)
	} else if cfg.Region != "" {
		client.SetRegion(cfg.Region)
	}

	// Initialize retryer with config or defaults
	retryer := retry.NewRetryer(cfg.RetryConfig, log)

	return &OCIStorage{
		client:    client,
		namespace: cfg.OCINamespace,
		bucket:    cfg.Bucket,
		prefix:    cfg.Prefix,
		log:       log,
		retryer:   retryer,
	}, nil
}

// Upload uploads a file to OCI Object Storage
func (o *OCIStorage) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	objectName := o.buildObjectName(remotePath)

	return o.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		// #nosec G304 -- localPath is supplied by the local hyperexport CLI operator, not a remote caller
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
			o.log.Info("uploading to OCI Object Storage",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"object", objectName,
				"size", fileInfo.Size())
		} else {
			o.log.Info("retrying OCI upload",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"object", objectName,
				"attempt", attempt)
		}

		// Wrap reader with progress tracking
		var body io.ReadCloser
		if progress != nil {
			body = &progressReadCloser{
				ReadCloser: file,
				size:       fileInfo.Size(),
				callback:   progress,
			}
		} else {
			body = file
		}

		// Upload object
		request := objectstorage.PutObjectRequest{
			NamespaceName: common.String(o.namespace),
			BucketName:    common.String(o.bucket),
			ObjectName:    common.String(objectName),
			PutObjectBody: body,
			ContentLength: common.Int64(fileInfo.Size()),
		}

		_, err = o.client.PutObject(ctx, request)
		if err != nil {
			return fmt.Errorf("upload to OCI: %w", err)
		}

		return nil
	}, fmt.Sprintf("OCI upload %s", objectName))
}

// UploadStream uploads data from a reader to OCI Object Storage
func (o *OCIStorage) UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error {
	objectName := o.buildObjectName(remotePath)

	o.log.Info("uploading stream to OCI Object Storage",
		"namespace", o.namespace,
		"bucket", o.bucket,
		"object", objectName,
		"size", size)

	// Convert to ReadCloser if needed
	var body io.ReadCloser
	if rc, ok := reader.(io.ReadCloser); ok {
		if progress != nil {
			body = &progressReadCloser{
				ReadCloser: rc,
				size:       size,
				callback:   progress,
			}
		} else {
			body = rc
		}
	} else {
		// Wrap plain reader
		if progress != nil {
			body = &progressReadCloser{
				ReadCloser: io.NopCloser(reader),
				size:       size,
				callback:   progress,
			}
		} else {
			body = io.NopCloser(reader)
		}
	}

	request := objectstorage.PutObjectRequest{
		NamespaceName: common.String(o.namespace),
		BucketName:    common.String(o.bucket),
		ObjectName:    common.String(objectName),
		PutObjectBody: body,
		ContentLength: common.Int64(size),
	}

	_, err := o.client.PutObject(ctx, request)
	if err != nil {
		return fmt.Errorf("upload stream to OCI: %w", err)
	}

	return nil
}

// Download downloads a file from OCI Object Storage
func (o *OCIStorage) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	objectName := o.buildObjectName(remotePath)

	return o.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			o.log.Info("downloading from OCI Object Storage",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"object", objectName)
		} else {
			o.log.Info("retrying OCI download",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"object", objectName,
				"attempt", attempt)
		}

		// Get object
		request := objectstorage.GetObjectRequest{
			NamespaceName: common.String(o.namespace),
			BucketName:    common.String(o.bucket),
			ObjectName:    common.String(objectName),
		}

		response, err := o.client.GetObject(ctx, request)
		if err != nil {
			// Check for not found errors
			if strings.Contains(err.Error(), "ObjectNotFound") || strings.Contains(err.Error(), "404") {
				return retry.IsNonRetryable(fmt.Errorf("object not found: %w", err))
			}
			return fmt.Errorf("get object from OCI: %w", err)
		}
		defer response.Content.Close()

		// Create local file
		// #nosec G304 -- localPath is supplied by the local hyperexport CLI operator, not a remote caller
		file, err := os.Create(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("create file: %w", err))
		}
		defer file.Close()

		// Get size
		size := int64(0)
		if response.ContentLength != nil {
			size = *response.ContentLength
		}

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := response.Content.Read(buf)
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
					return fmt.Errorf("read from OCI: %w", er)
				}
				break
			}
		}

		return nil
	}, fmt.Sprintf("OCI download %s", objectName))
}

// List lists objects in OCI Object Storage with a prefix
func (o *OCIStorage) List(ctx context.Context, prefix string) ([]CloudFile, error) {
	objectPrefix := o.buildObjectName(prefix)

	result, err := o.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt == 1 {
			o.log.Debug("listing OCI objects",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"prefix", objectPrefix)
		} else {
			o.log.Debug("retrying OCI list",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"prefix", objectPrefix,
				"attempt", attempt)
		}

		var files []CloudFile
		var nextStartWith *string

		for {
			request := objectstorage.ListObjectsRequest{
				NamespaceName: common.String(o.namespace),
				BucketName:    common.String(o.bucket),
				Prefix:        common.String(objectPrefix),
				Start:         nextStartWith,
				Limit:         common.Int(1000),
			}

			response, err := o.client.ListObjects(ctx, request)
			if err != nil {
				return nil, fmt.Errorf("list objects: %w", err)
			}

			for _, obj := range response.Objects {
				files = append(files, CloudFile{
					Path:         *obj.Name,
					Size:         *obj.Size,
					LastModified: obj.TimeModified.Time,
					ETag:         strings.Trim(*obj.Etag, "\""),
				})
			}

			// Check if there are more results
			if response.NextStartWith == nil {
				break
			}
			nextStartWith = response.NextStartWith
		}

		return files, nil
	}, fmt.Sprintf("OCI list %s", objectPrefix))

	if err != nil {
		return nil, err
	}

	return result.([]CloudFile), nil
}

// Delete deletes an object from OCI Object Storage
func (o *OCIStorage) Delete(ctx context.Context, remotePath string) error {
	objectName := o.buildObjectName(remotePath)

	return o.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			o.log.Info("deleting from OCI Object Storage",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"object", objectName)
		} else {
			o.log.Info("retrying OCI delete",
				"namespace", o.namespace,
				"bucket", o.bucket,
				"object", objectName,
				"attempt", attempt)
		}

		request := objectstorage.DeleteObjectRequest{
			NamespaceName: common.String(o.namespace),
			BucketName:    common.String(o.bucket),
			ObjectName:    common.String(objectName),
		}

		_, err := o.client.DeleteObject(ctx, request)
		if err != nil {
			// Ignore not found errors on delete
			if strings.Contains(err.Error(), "ObjectNotFound") || strings.Contains(err.Error(), "404") {
				return nil
			}
			return fmt.Errorf("delete object: %w", err)
		}

		return nil
	}, fmt.Sprintf("OCI delete %s", objectName))
}

// Exists checks if an object exists in OCI Object Storage
func (o *OCIStorage) Exists(ctx context.Context, remotePath string) (bool, error) {
	objectName := o.buildObjectName(remotePath)

	result, err := o.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		request := objectstorage.HeadObjectRequest{
			NamespaceName: common.String(o.namespace),
			BucketName:    common.String(o.bucket),
			ObjectName:    common.String(objectName),
		}

		_, err := o.client.HeadObject(ctx, request)
		if err != nil {
			// Check if it's a not found error
			if strings.Contains(err.Error(), "ObjectNotFound") || strings.Contains(err.Error(), "404") {
				return false, nil
			}
			return false, fmt.Errorf("head object: %w", err)
		}

		return true, nil
	}, fmt.Sprintf("OCI exists %s", objectName))

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// GetURL returns the OCI Object Storage URL
func (o *OCIStorage) GetURL() string {
	return fmt.Sprintf("oci://%s/%s/%s", o.namespace, o.bucket, o.prefix)
}

// Close closes the OCI client (no-op for OCI)
func (o *OCIStorage) Close() error {
	return nil
}

// buildObjectName builds the full object name with prefix
func (o *OCIStorage) buildObjectName(remotePath string) string {
	if o.prefix == "" {
		return remotePath
	}
	return filepath.ToSlash(filepath.Join(o.prefix, remotePath))
}

// progressReadCloser wraps a ReadCloser to track progress
type progressReadCloser struct {
	io.ReadCloser
	size     int64
	read     int64
	callback ProgressCallback
}

func (pr *progressReadCloser) Read(p []byte) (int, error) {
	n, err := pr.ReadCloser.Read(p)
	pr.read += int64(n)

	if pr.callback != nil {
		pr.callback(pr.read, pr.size)
	}

	return n, err
}
