// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// S3Storage implements CloudStorage for AWS S3
type S3Storage struct {
	client *s3.Client
	// uploader uses the deprecated feature/s3/manager package. Migrating to
	// feature/s3/transfermanager requires adding a new go.mod dependency,
	// which is out of scope for this change; tracked as follow-up work.
	//nolint:staticcheck // SA1019: see above
	uploader *manager.Uploader
	bucket   string
	prefix   string
	log      logger.Logger
	retryer  *retry.Retryer
}

// NewS3Storage creates a new S3 storage client
func NewS3Storage(cfg *CloudStorageConfig, log logger.Logger) (*S3Storage, error) {
	var awsCfg aws.Config
	var err error

	ctx := context.Background()

	// Use custom credentials if provided
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(creds),
		)
	} else {
		// Use default credential chain
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	}

	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Custom endpoint for S3-compatible storage
	var s3Client *s3.Client
	if cfg.Endpoint != "" {
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	} else {
		s3Client = s3.NewFromConfig(awsCfg)
	}

	// feature/s3/manager.NewUploader is deprecated in favor of
	// feature/s3/transfermanager, but adopting it requires a new go.mod
	// dependency, which is out of scope for this change.
	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) { //nolint:staticcheck // SA1019: see comment above
		u.PartSize = 10 * 1024 * 1024 // 10 MB parts
		u.Concurrency = 5
	})

	// Initialize retryer with config or defaults
	retryer := retry.NewRetryer(cfg.RetryConfig, log)

	return &S3Storage{
		client:   s3Client,
		uploader: uploader,
		bucket:   cfg.Bucket,
		prefix:   cfg.Prefix,
		log:      log,
		retryer:  retryer,
	}, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	key := s.buildKey(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
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
			s.log.Info("uploading to S3", "bucket", s.bucket, "key", key, "size", fileInfo.Size())
		} else {
			s.log.Info("retrying S3 upload", "bucket", s.bucket, "key", key, "attempt", attempt)
		}

		// Wrap reader with progress tracking
		reader := &progressReader{
			reader:   file,
			size:     fileInfo.Size(),
			callback: progress,
		}

		_, err = s.uploader.Upload(ctx, &s3.PutObjectInput{ //nolint:staticcheck // SA1019: manager.Uploader is deprecated; migrating to transfermanager needs a new go.mod dep, out of scope
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
			Body:   reader,
		})

		if err != nil {
			return fmt.Errorf("upload to S3: %w", err)
		}

		return nil
	}, fmt.Sprintf("S3 upload %s", key))
}

// UploadStream uploads data from a reader to S3
func (s *S3Storage) UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error {
	key := s.buildKey(remotePath)

	s.log.Info("uploading stream to S3", "bucket", s.bucket, "key", key, "size", size)

	// Wrap reader with progress tracking
	progressReader := &progressReader{
		reader:   reader,
		size:     size,
		callback: progress,
	}

	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{ //nolint:staticcheck // SA1019: manager.Uploader is deprecated; migrating to transfermanager needs a new go.mod dep, out of scope
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   progressReader,
	})

	if err != nil {
		return fmt.Errorf("upload stream to S3: %w", err)
	}

	return nil
}

// Download downloads a file from S3
func (s *S3Storage) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	key := s.buildKey(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			s.log.Info("downloading from S3", "bucket", s.bucket, "key", key)
		} else {
			s.log.Info("retrying S3 download", "bucket", s.bucket, "key", key, "attempt", attempt)
		}

		// Get object
		result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			// Not found errors are not retryable
			var notFound *types.NoSuchKey
			if errors.As(err, &notFound) {
				return retry.IsNonRetryable(fmt.Errorf("object not found: %w", err))
			}
			return fmt.Errorf("get object from S3: %w", err)
		}
		defer func() {
			if closeErr := result.Body.Close(); closeErr != nil {
				s.log.Warn("failed to close S3 response body", "error", closeErr)
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
		size := int64(0)
		if result.ContentLength != nil {
			size = *result.ContentLength
		}

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := result.Body.Read(buf)
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
					return fmt.Errorf("read from S3: %w", er)
				}
				break
			}
		}

		return nil
	}, fmt.Sprintf("S3 download %s", key))
}

// List lists files in S3 with a prefix
func (s *S3Storage) List(ctx context.Context, prefix string) ([]CloudFile, error) {
	key := s.buildKey(prefix)

	result, err := s.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt == 1 {
			s.log.Debug("listing S3 objects", "bucket", s.bucket, "prefix", key)
		} else {
			s.log.Debug("retrying S3 list", "bucket", s.bucket, "prefix", key, "attempt", attempt)
		}

		var files []CloudFile
		paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String(key),
		})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("list objects: %w", err)
			}

			for _, obj := range page.Contents {
				files = append(files, CloudFile{
					Path:         *obj.Key,
					Size:         *obj.Size,
					LastModified: *obj.LastModified,
					ETag:         strings.Trim(*obj.ETag, "\""),
				})
			}
		}

		return files, nil
	}, fmt.Sprintf("S3 list %s", key))

	if err != nil {
		return nil, err
	}

	return result.([]CloudFile), nil
}

// Delete deletes a file from S3
func (s *S3Storage) Delete(ctx context.Context, remotePath string) error {
	key := s.buildKey(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			s.log.Info("deleting from S3", "bucket", s.bucket, "key", key)
		} else {
			s.log.Info("retrying S3 delete", "bucket", s.bucket, "key", key, "attempt", attempt)
		}

		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})

		if err != nil {
			return fmt.Errorf("delete object: %w", err)
		}

		return nil
	}, fmt.Sprintf("S3 delete %s", key))
}

// Exists checks if a file exists in S3
func (s *S3Storage) Exists(ctx context.Context, remotePath string) (bool, error) {
	key := s.buildKey(remotePath)

	result, err := s.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})

		if err != nil {
			// Check if it's a not found error
			var notFound *types.NotFound
			if errors.As(err, &notFound) {
				return false, nil
			}
			return false, fmt.Errorf("head object: %w", err)
		}

		return true, nil
	}, fmt.Sprintf("S3 exists %s", key))

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// GetURL returns the S3 URL
func (s *S3Storage) GetURL() string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.prefix)
}

// Close closes the S3 client (no-op for S3)
func (s *S3Storage) Close() error {
	return nil
}

// buildKey builds the full S3 key with prefix
func (s *S3Storage) buildKey(remotePath string) string {
	if s.prefix == "" {
		return remotePath
	}
	return filepath.ToSlash(filepath.Join(s.prefix, remotePath))
}

// progressReader wraps a reader to track progress
type progressReader struct {
	reader   io.Reader
	size     int64
	read     int64
	callback ProgressCallback
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.read += int64(n)

	if pr.callback != nil {
		pr.callback(pr.read, pr.size)
	}

	return n, err
}
