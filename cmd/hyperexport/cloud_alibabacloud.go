// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// AlibabaCloudOSSStorage implements CloudStorage for Alibaba Cloud OSS
type AlibabaCloudOSSStorage struct {
	client  *oss.Client
	bucket  *oss.Bucket
	prefix  string
	log     logger.Logger
	retryer *retry.Retryer
}

// NewAlibabaCloudOSSStorage creates a new Alibaba Cloud OSS storage client
func NewAlibabaCloudOSSStorage(cfg *CloudStorageConfig, log logger.Logger) (*AlibabaCloudOSSStorage, error) {
	if cfg.AlibabaAccessKeyID == "" {
		return nil, fmt.Errorf("alibaba Cloud AccessKey ID is required")
	}
	if cfg.AlibabaAccessKeySecret == "" {
		return nil, fmt.Errorf("alibaba Cloud AccessKey Secret is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	// Determine endpoint
	endpoint := cfg.Endpoint
	if endpoint == "" {
		// Default endpoint format: oss-<region>.aliyuncs.com
		if cfg.AlibabaRegionID != "" {
			endpoint = fmt.Sprintf("https://oss-%s.aliyuncs.com", cfg.AlibabaRegionID)
		} else {
			endpoint = "https://oss-cn-hangzhou.aliyuncs.com"
		}
	}

	// Create OSS client
	client, err := oss.New(endpoint, cfg.AlibabaAccessKeyID, cfg.AlibabaAccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create OSS client: %w", err)
	}

	// Get bucket
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("get bucket: %w", err)
	}

	// Initialize retryer with config or defaults
	retryer := retry.NewRetryer(cfg.RetryConfig, log)

	return &AlibabaCloudOSSStorage{
		client:  client,
		bucket:  bucket,
		prefix:  cfg.Prefix,
		log:     log,
		retryer: retryer,
	}, nil
}

// Upload uploads a file to OSS
func (a *AlibabaCloudOSSStorage) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	objectKey := a.buildObjectKey(remotePath)

	return a.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		// #nosec G304 -- localPath is supplied by the local operator invoking hyperexport, not remote input
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
			a.log.Info("uploading to Alibaba Cloud OSS",
				"bucket", a.bucket.BucketName,
				"object", objectKey,
				"size", fileInfo.Size())
		} else {
			a.log.Info("retrying OSS upload",
				"bucket", a.bucket.BucketName,
				"object", objectKey,
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
		err = a.bucket.PutObject(objectKey, reader)
		if err != nil {
			return fmt.Errorf("upload to OSS: %w", err)
		}

		return nil
	}, fmt.Sprintf("OSS upload %s", objectKey))
}

// UploadStream uploads data from a reader to OSS
func (a *AlibabaCloudOSSStorage) UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error {
	objectKey := a.buildObjectKey(remotePath)

	a.log.Info("uploading stream to Alibaba Cloud OSS",
		"bucket", a.bucket.BucketName,
		"object", objectKey,
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

	err := a.bucket.PutObject(objectKey, uploadReader)
	if err != nil {
		return fmt.Errorf("upload stream to OSS: %w", err)
	}

	return nil
}

// Download downloads a file from OSS
func (a *AlibabaCloudOSSStorage) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	objectKey := a.buildObjectKey(remotePath)

	return a.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			a.log.Info("downloading from Alibaba Cloud OSS",
				"bucket", a.bucket.BucketName,
				"object", objectKey)
		} else {
			a.log.Info("retrying OSS download",
				"bucket", a.bucket.BucketName,
				"object", objectKey,
				"attempt", attempt)
		}

		// Get object
		body, err := a.bucket.GetObject(objectKey)
		if err != nil {
			// Not found errors are not retryable
			if isOSSNotFoundError(err) {
				return retry.IsNonRetryable(fmt.Errorf("object not found: %w", err))
			}
			return fmt.Errorf("get object from OSS: %w", err)
		}
		defer func() {
			if cerr := body.Close(); cerr != nil {
				a.log.Warn("failed to close response body", "error", cerr)
			}
		}()

		// Create local file
		// #nosec G304 -- localPath is supplied by the local operator invoking hyperexport, not remote input
		file, err := os.Create(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("create file: %w", err))
		}
		defer func() {
			if cerr := file.Close(); cerr != nil {
				a.log.Warn("failed to close file", "error", cerr)
			}
		}()

		// Get object metadata for size
		meta, err := a.bucket.GetObjectMeta(objectKey)
		size := int64(0)
		if err == nil {
			if contentLength := meta.Get("Content-Length"); contentLength != "" {
				if _, err := fmt.Sscanf(contentLength, "%d", &size); err != nil {
					a.log.Warn("failed to parse content length", "value", contentLength, "error", err)
				}
			}
		}

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := body.Read(buf)
			if nr > 0 {
				nw, ew := file.Write(buf[0:nr])
				if nw > 0 {
					written += int64(nw)
					if progress != nil && size > 0 {
						progress(written, size)
					}
				}
				if ew != nil {
					return retry.IsNonRetryable(fmt.Errorf("write file: %w", ew))
				}
			}
			if er != nil {
				if er != io.EOF {
					return fmt.Errorf("read from OSS: %w", er)
				}
				break
			}
		}

		return nil
	}, fmt.Sprintf("OSS download %s", objectKey))
}

// List lists objects in OSS with a prefix
func (a *AlibabaCloudOSSStorage) List(ctx context.Context, prefix string) ([]CloudFile, error) {
	objectPrefix := a.buildObjectKey(prefix)

	result, err := a.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt == 1 {
			a.log.Debug("listing OSS objects",
				"bucket", a.bucket.BucketName,
				"prefix", objectPrefix)
		} else {
			a.log.Debug("retrying OSS list",
				"bucket", a.bucket.BucketName,
				"prefix", objectPrefix,
				"attempt", attempt)
		}

		var files []CloudFile
		marker := ""

		for {
			listResult, err := a.bucket.ListObjects(
				oss.Prefix(objectPrefix),
				oss.Marker(marker),
				oss.MaxKeys(1000),
			)
			if err != nil {
				return nil, fmt.Errorf("list objects: %w", err)
			}

			for _, obj := range listResult.Objects {
				files = append(files, CloudFile{
					Path:         obj.Key,
					Size:         obj.Size,
					LastModified: obj.LastModified,
					ETag:         strings.Trim(obj.ETag, "\""),
				})
			}

			if !listResult.IsTruncated {
				break
			}
			marker = listResult.NextMarker
		}

		return files, nil
	}, fmt.Sprintf("OSS list %s", objectPrefix))

	if err != nil {
		return nil, err
	}

	return result.([]CloudFile), nil
}

// Delete deletes an object from OSS
func (a *AlibabaCloudOSSStorage) Delete(ctx context.Context, remotePath string) error {
	objectKey := a.buildObjectKey(remotePath)

	return a.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			a.log.Info("deleting from Alibaba Cloud OSS",
				"bucket", a.bucket.BucketName,
				"object", objectKey)
		} else {
			a.log.Info("retrying OSS delete",
				"bucket", a.bucket.BucketName,
				"object", objectKey,
				"attempt", attempt)
		}

		err := a.bucket.DeleteObject(objectKey)
		if err != nil {
			// Ignore not found errors on delete
			if isOSSNotFoundError(err) {
				return nil
			}
			return fmt.Errorf("delete object: %w", err)
		}

		return nil
	}, fmt.Sprintf("OSS delete %s", objectKey))
}

// Exists checks if an object exists in OSS
func (a *AlibabaCloudOSSStorage) Exists(ctx context.Context, remotePath string) (bool, error) {
	objectKey := a.buildObjectKey(remotePath)

	result, err := a.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		exists, err := a.bucket.IsObjectExist(objectKey)
		if err != nil {
			return false, fmt.Errorf("check object existence: %w", err)
		}
		return exists, nil
	}, fmt.Sprintf("OSS exists %s", objectKey))

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// GetURL returns the OSS storage URL
func (a *AlibabaCloudOSSStorage) GetURL() string {
	return fmt.Sprintf("oss://%s/%s", a.bucket.BucketName, a.prefix)
}

// Close closes the OSS client (no-op for OSS)
func (a *AlibabaCloudOSSStorage) Close() error {
	return nil
}

// buildObjectKey builds the full object key with prefix
func (a *AlibabaCloudOSSStorage) buildObjectKey(remotePath string) string {
	if a.prefix == "" {
		return remotePath
	}
	return filepath.ToSlash(filepath.Join(a.prefix, remotePath))
}

// isOSSNotFoundError checks if an error is a not found error
func isOSSNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "NoSuchKey") ||
		strings.Contains(errMsg, "404") ||
		strings.Contains(errMsg, "Not Found")
}
