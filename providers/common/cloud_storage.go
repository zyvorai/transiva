// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// CloudStorageProvider represents a cloud storage provider type
type CloudStorageProvider string

const (
	ProviderS3    CloudStorageProvider = "s3"    // AWS S3
	ProviderAzure CloudStorageProvider = "azure" // Azure Blob Storage
	ProviderGCS   CloudStorageProvider = "gcs"   // Google Cloud Storage
	ProviderLocal CloudStorageProvider = "local" // Local filesystem
)

// CloudStorageConfig holds cloud storage configuration
type CloudStorageConfig struct {
	// Provider type
	Provider CloudStorageProvider `json:"provider"`

	// S3 configuration
	S3Config *S3Config `json:"s3_config,omitempty"`

	// Azure configuration
	AzureConfig *AzureStorageConfig `json:"azure_config,omitempty"`

	// GCS configuration
	GCSConfig *GCSConfig `json:"gcs_config,omitempty"`

	// Upload options
	UploadParallel int  `json:"upload_parallel,omitempty"` // Parallel upload threads
	DeleteLocal    bool `json:"delete_local,omitempty"`    // Delete local files after upload
	Encrypt        bool `json:"encrypt,omitempty"`         // Encrypt files before upload
	Compress       bool `json:"compress,omitempty"`        // Compress files before upload
}

// S3Config holds AWS S3 configuration
type S3Config struct {
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`      // Custom endpoint (for S3-compatible storage)
	Prefix          string `json:"prefix,omitempty"`        // Object key prefix
	StorageClass    string `json:"storage_class,omitempty"` // Storage class (STANDARD, GLACIER, etc.)
}

// AzureStorageConfig holds Azure Blob Storage configuration
type AzureStorageConfig struct {
	AccountName      string `json:"account_name"`
	AccountKey       string `json:"account_key,omitempty"`
	ConnectionString string `json:"connection_string,omitempty"`
	Container        string `json:"container"`
	Prefix           string `json:"prefix,omitempty"`
	BlobType         string `json:"blob_type,omitempty"` // BlockBlob, PageBlob, AppendBlob
}

// GCSConfig holds Google Cloud Storage configuration
type GCSConfig struct {
	Bucket          string `json:"bucket"`
	ProjectID       string `json:"project_id"`
	CredentialsFile string `json:"credentials_file,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	StorageClass    string `json:"storage_class,omitempty"` // STANDARD, NEARLINE, COLDLINE, ARCHIVE
}

// CloudStorageUploader defines interface for cloud storage uploaders
type CloudStorageUploader interface {
	// Upload uploads a file to cloud storage
	Upload(ctx context.Context, localPath, remotePath string, opts *UploadOptions) error

	// UploadMultiple uploads multiple files in parallel
	UploadMultiple(ctx context.Context, files map[string]string, opts *UploadOptions) error

	// Download downloads a file from cloud storage
	Download(ctx context.Context, remotePath, localPath string) error

	// Delete deletes a file from cloud storage
	Delete(ctx context.Context, remotePath string) error

	// List lists files in cloud storage
	List(ctx context.Context, prefix string) ([]string, error)

	// GetURL returns a pre-signed URL for download
	GetURL(ctx context.Context, remotePath string, expiration time.Duration) (string, error)
}

// UploadOptions holds options for file uploads
type UploadOptions struct {
	// Content type
	ContentType string

	// Metadata
	Metadata map[string]string

	// Progress callback
	ProgressCallback func(uploaded, total int64)

	// Encryption key (for client-side encryption)
	EncryptionKey []byte

	// Compression level (0-9)
	CompressionLevel int

	// Checksum verification
	VerifyChecksum bool

	// Storage class override
	StorageClass string

	// ACL (access control list)
	ACL string
}

// UploadResult holds the result of a cloud storage upload
type UploadResult struct {
	Provider     CloudStorageProvider
	RemotePath   string
	URL          string
	Size         int64
	Checksum     string
	Duration     time.Duration
	StorageClass string
}

// CloudStorageManager manages cloud storage operations
type CloudStorageManager struct {
	config   *CloudStorageConfig
	uploader CloudStorageUploader
}

// NewCloudStorageManager creates a new cloud storage manager
func NewCloudStorageManager(config *CloudStorageConfig) (*CloudStorageManager, error) {
	if config == nil {
		return nil, fmt.Errorf("cloud storage config is required")
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &CloudStorageManager{
		config: config,
	}, nil
}

// SetUploader sets the cloud storage uploader implementation
func (csm *CloudStorageManager) SetUploader(uploader CloudStorageUploader) {
	csm.uploader = uploader
}

// UploadConvertedImages uploads converted VM images to cloud storage
func (csm *CloudStorageManager) UploadConvertedImages(ctx context.Context, result *ConversionResult, prefix string) ([]*UploadResult, error) {
	if csm.uploader == nil {
		return nil, fmt.Errorf("no uploader configured")
	}

	if len(result.ConvertedFiles) == 0 {
		return nil, fmt.Errorf("no converted files to upload")
	}

	// Prepare upload map
	files := make(map[string]string)
	for _, localPath := range result.ConvertedFiles {
		fileName := filepath.Base(localPath)
		remotePath := filepath.Join(prefix, fileName)
		files[localPath] = remotePath
	}

	// Upload options
	opts := &UploadOptions{
		ContentType:      "application/octet-stream",
		VerifyChecksum:   true,
		CompressionLevel: 0, // Already compressed by converter
		Metadata: map[string]string{
			"source":          "transiva",
			"conversion-tool": "hyper2kvm",
		},
	}

	// Upload files
	startTime := time.Now()
	if err := csm.uploader.UploadMultiple(ctx, files, opts); err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	// Create upload results
	var results []*UploadResult
	for localPath, remotePath := range files {
		stat, _ := os.Stat(localPath)
		var size int64
		if stat != nil {
			size = stat.Size()
		}

		results = append(results, &UploadResult{
			Provider:   csm.config.Provider,
			RemotePath: remotePath,
			Size:       size,
			Duration:   time.Since(startTime),
		})
	}

	// Delete local files if requested
	if csm.config.DeleteLocal {
		for localPath := range files {
			if err := os.Remove(localPath); err != nil {
				// Log error but don't fail
				fmt.Printf("Warning: failed to delete local file %s: %v\n", localPath, err)
			}
		}
	}

	return results, nil
}

// Validate validates the cloud storage configuration
func (cfg *CloudStorageConfig) Validate() error {
	switch cfg.Provider {
	case ProviderS3:
		if cfg.S3Config == nil {
			return fmt.Errorf("S3 config is required for S3 provider")
		}
		if cfg.S3Config.Bucket == "" {
			return fmt.Errorf("S3 bucket is required")
		}
		if cfg.S3Config.Region == "" {
			return fmt.Errorf("S3 region is required")
		}

	case ProviderAzure:
		if cfg.AzureConfig == nil {
			return fmt.Errorf("azure config is required for Azure provider")
		}
		if cfg.AzureConfig.AccountName == "" {
			return fmt.Errorf("azure account name is required")
		}
		if cfg.AzureConfig.Container == "" {
			return fmt.Errorf("azure container is required")
		}

	case ProviderGCS:
		if cfg.GCSConfig == nil {
			return fmt.Errorf("GCS config is required for GCS provider")
		}
		if cfg.GCSConfig.Bucket == "" {
			return fmt.Errorf("GCS bucket is required")
		}
		if cfg.GCSConfig.ProjectID == "" {
			return fmt.Errorf("GCS project ID is required")
		}

	case ProviderLocal:
		// No validation needed for local storage

	default:
		return fmt.Errorf("unsupported cloud storage provider: %s", cfg.Provider)
	}

	return nil
}

// ProgressReader wraps an io.Reader to report progress
type ProgressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	callback func(uploaded, total int64)
}

// NewProgressReader creates a new progress reader
func NewProgressReader(reader io.Reader, total int64, callback func(uploaded, total int64)) *ProgressReader {
	return &ProgressReader{
		reader:   reader,
		total:    total,
		callback: callback,
	}
}

// Read implements io.Reader
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	if pr.callback != nil {
		pr.callback(pr.current, pr.total)
	}

	return n, err
}
