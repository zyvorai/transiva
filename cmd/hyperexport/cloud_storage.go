// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// CloudStorage defines the interface for cloud storage providers
type CloudStorage interface {
	// Upload uploads a file to cloud storage
	Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error

	// UploadStream uploads data from a reader to cloud storage
	UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error

	// Download downloads a file from cloud storage
	Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error

	// List lists files in a directory
	List(ctx context.Context, prefix string) ([]CloudFile, error)

	// Delete deletes a file
	Delete(ctx context.Context, remotePath string) error

	// Exists checks if a file exists
	Exists(ctx context.Context, remotePath string) (bool, error)

	// GetURL returns the storage URL
	GetURL() string

	// Close closes the connection
	Close() error
}

// CloudFile represents a file in cloud storage
type CloudFile struct {
	Path         string
	Size         int64
	LastModified time.Time
	ETag         string
}

// ProgressCallback is called during upload/download
type ProgressCallback func(bytesTransferred, totalBytes int64)

// CloudStorageConfig contains cloud storage configuration
type CloudStorageConfig struct {
	Provider    string             // s3, azure, gcs, sftp, oci
	Bucket      string             // S3 bucket, Azure container, GCS bucket, OCI bucket
	Region      string             // AWS region, OCI region
	Endpoint    string             // Custom endpoint (for S3-compatible storage)
	AccessKey   string             // AWS access key, Azure account name
	SecretKey   string             // AWS secret key, Azure account key
	Host         string             // SFTP host
	Port         int                // SFTP port
	Username     string             // SFTP username
	Password     string             // SFTP password
	PrivateKey   string             // SFTP private key path
	HostKeyPath  string             // SFTP known_hosts file path (optional, defaults to ~/.ssh/known_hosts)
	Prefix       string             // Path prefix in bucket/container
	RetryConfig  *retry.RetryConfig // Retry configuration (nil = use defaults)

	// OCI-specific fields
	OCINamespace   string // OCI Object Storage namespace
	OCITenancyOCID string // OCI tenancy OCID
	OCIUserOCID    string // OCI user OCID
	OCIFingerprint string // API key fingerprint
	OCIPrivateKey  string // Private key content or path
	OCIConfigPath  string // Path to OCI config file (~/.oci/config)
	OCIProfile     string // Profile name in config file (default: DEFAULT)

	// OpenStack Swift specific fields
	SwiftAuthURL    string // Keystone auth URL
	SwiftUsername   string // OpenStack username
	SwiftPassword   string // OpenStack password
	SwiftTenantName string // Tenant/Project name
	SwiftDomainName string // Domain name (default: Default)

	// Alibaba Cloud OSS specific fields
	AlibabaAccessKeyID     string // Alibaba Cloud AccessKey ID
	AlibabaAccessKeySecret string // Alibaba Cloud AccessKey Secret
	AlibabaRegionID        string // Alibaba Cloud region ID
}

// NewCloudStorage creates a cloud storage client from URL
func NewCloudStorage(storageURL string, log logger.Logger) (CloudStorage, error) {
	u, err := url.Parse(storageURL)
	if err != nil {
		return nil, fmt.Errorf("parse storage URL: %w", err)
	}

	config := &CloudStorageConfig{
		Provider: u.Scheme,
	}

	// Parse URL components
	switch u.Scheme {
	case "s3":
		config.Bucket = u.Host
		config.Prefix = strings.TrimPrefix(u.Path, "/")
		// Get credentials from environment
		config.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
		config.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		config.Region = os.Getenv("AWS_REGION")
		if config.Region == "" {
			config.Region = "us-east-1"
		}
		return NewS3Storage(config, log)

	case "azure":
		config.Bucket = u.Host // container name
		config.Prefix = strings.TrimPrefix(u.Path, "/")
		config.AccessKey = os.Getenv("AZURE_STORAGE_ACCOUNT")
		config.SecretKey = os.Getenv("AZURE_STORAGE_KEY")
		return NewAzureStorage(config, log)

	case "gcs", "gs":
		config.Bucket = u.Host
		config.Prefix = strings.TrimPrefix(u.Path, "/")
		// GCS uses service account JSON
		return NewGCSStorage(config, log)

	case "sftp":
		config.Host = u.Host
		config.Prefix = strings.TrimPrefix(u.Path, "/")
		if u.User != nil {
			config.Username = u.User.Username()
			config.Password, _ = u.User.Password()
		}
		if config.Port == 0 {
			config.Port = 22
		}
		return NewSFTPStorage(config, log)

	case "oci":
		// Format: oci://namespace/bucket/prefix
		pathParts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
		if len(pathParts) < 1 {
			return nil, fmt.Errorf("OCI URL must include bucket: oci://namespace/bucket/prefix")
		}
		config.OCINamespace = u.Host
		config.Bucket = pathParts[0]
		if len(pathParts) > 1 {
			config.Prefix = pathParts[1]
		}
		// Get credentials from environment or config file
		config.OCIConfigPath = os.Getenv("OCI_CONFIG_PATH")
		if config.OCIConfigPath == "" {
			config.OCIConfigPath = filepath.Join(os.Getenv("HOME"), ".oci", "config")
		}
		config.OCIProfile = os.Getenv("OCI_PROFILE")
		if config.OCIProfile == "" {
			config.OCIProfile = "DEFAULT"
		}
		config.Region = os.Getenv("OCI_REGION")
		return NewOCIStorage(config, log)

	case "swift":
		// Format: swift://container/prefix
		config.Bucket = u.Host // container name
		config.Prefix = strings.TrimPrefix(u.Path, "/")
		// Get credentials from environment
		config.SwiftAuthURL = os.Getenv("OS_AUTH_URL")
		config.SwiftUsername = os.Getenv("OS_USERNAME")
		config.SwiftPassword = os.Getenv("OS_PASSWORD")
		config.SwiftTenantName = os.Getenv("OS_TENANT_NAME")
		config.SwiftDomainName = os.Getenv("OS_DOMAIN_NAME")
		config.Region = os.Getenv("OS_REGION_NAME")
		if config.SwiftDomainName == "" {
			config.SwiftDomainName = "Default"
		}
		return NewOpenStackSwiftStorage(config, log)

	case "oss":
		// Format: oss://bucket/prefix
		config.Bucket = u.Host // bucket name
		config.Prefix = strings.TrimPrefix(u.Path, "/")
		// Get credentials from environment
		config.AlibabaAccessKeyID = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
		config.AlibabaAccessKeySecret = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
		config.AlibabaRegionID = os.Getenv("ALIBABA_CLOUD_REGION_ID")
		if config.AlibabaRegionID == "" {
			config.AlibabaRegionID = "cn-hangzhou"
		}
		return NewAlibabaCloudOSSStorage(config, log)

	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", u.Scheme)
	}
}

// UploadDirectory uploads an entire directory to cloud storage
func UploadDirectory(ctx context.Context, storage CloudStorage, localDir, remotePrefix string, log logger.Logger) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		remotePath := filepath.Join(remotePrefix, relPath)
		remotePath = filepath.ToSlash(remotePath) // Convert to forward slashes for cloud

		log.Info("uploading file to cloud", "local", path, "remote", remotePath)

		if err := storage.Upload(ctx, path, remotePath, func(transferred, total int64) {
			if total > 0 {
				pct := float64(transferred) / float64(total) * 100
				log.Debug("upload progress", "file", relPath, "percent", fmt.Sprintf("%.1f%%", pct))
			}
		}); err != nil {
			return fmt.Errorf("upload %s: %w", path, err)
		}

		return nil
	})
}

// DownloadDirectory downloads an entire directory from cloud storage
func DownloadDirectory(ctx context.Context, storage CloudStorage, remotePrefix, localDir string, log logger.Logger) error {
	files, err := storage.List(ctx, remotePrefix)
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}

	for _, file := range files {
		// Calculate local path
		relPath := strings.TrimPrefix(file.Path, remotePrefix)
		relPath = strings.TrimPrefix(relPath, "/")
		localPath := filepath.Join(localDir, relPath)

		// Create directory
		if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		log.Info("downloading file from cloud", "remote", file.Path, "local", localPath)

		if err := storage.Download(ctx, file.Path, localPath, func(transferred, total int64) {
			if total > 0 {
				pct := float64(transferred) / float64(total) * 100
				log.Debug("download progress", "file", relPath, "percent", fmt.Sprintf("%.1f%%", pct))
			}
		}); err != nil {
			return fmt.Errorf("download %s: %w", file.Path, err)
		}
	}

	return nil
}

// DeleteDirectory deletes all files under a prefix
func DeleteDirectory(ctx context.Context, storage CloudStorage, remotePrefix string, log logger.Logger) error {
	files, err := storage.List(ctx, remotePrefix)
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}

	for _, file := range files {
		log.Info("deleting file from cloud", "path", file.Path)
		if err := storage.Delete(ctx, file.Path); err != nil {
			return fmt.Errorf("delete %s: %w", file.Path, err)
		}
	}

	return nil
}
