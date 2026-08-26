// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// SFTPStorage implements CloudStorage for SFTP
type SFTPStorage struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	host       string
	prefix     string
	log        logger.Logger
	retryer    *retry.Retryer
}

// NewSFTPStorage creates a new SFTP storage client
func NewSFTPStorage(cfg *CloudStorageConfig, log logger.Logger) (*SFTPStorage, error) {
	var authMethods []ssh.AuthMethod

	// Password authentication
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	// Private key authentication
	if cfg.PrivateKey != "" {
		key, err := os.ReadFile(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided (password or private key required)")
	}

	// Get host key callback with proper verification
	hostKeyCallback, err := getHostKeyCallback(cfg.HostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("setup host key verification: %w", err)
	}

	// SSH client configuration
	config := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		if closeErr := sshClient.Close(); closeErr != nil {
			log.Warn("failed to close SSH client after SFTP client creation failure", "error", closeErr)
		}
		return nil, fmt.Errorf("SFTP client creation failed: %w", err)
	}

	// Initialize retryer with config or defaults
	retryer := retry.NewRetryer(cfg.RetryConfig, log)

	return &SFTPStorage{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		host:       cfg.Host,
		prefix:     cfg.Prefix,
		log:        log,
		retryer:    retryer,
	}, nil
}

// Upload uploads a file via SFTP
func (s *SFTPStorage) Upload(ctx context.Context, localPath, remotePath string, progress ProgressCallback) error {
	remoteFullPath := s.buildPath(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		// #nosec G304 -- localPath comes from the local operator's export job configuration, not remote/network input
		file, err := os.Open(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("open file: %w", err))
		}
		defer func() { _ = file.Close() }()

		fileInfo, err := file.Stat()
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("stat file: %w", err))
		}

		if attempt == 1 {
			s.log.Info("uploading via SFTP", "host", s.host, "path", remoteFullPath, "size", fileInfo.Size())
		} else {
			s.log.Info("retrying SFTP upload", "host", s.host, "path", remoteFullPath, "attempt", attempt)
		}

		// Create remote directories
		remoteDir := filepath.Dir(remoteFullPath)
		if err := s.sftpClient.MkdirAll(remoteDir); err != nil {
			return fmt.Errorf("create remote directory: %w", err)
		}

		// Create remote file
		remoteFile, err := s.sftpClient.Create(remoteFullPath)
		if err != nil {
			return fmt.Errorf("create remote file: %w", err)
		}
		defer func() { _ = remoteFile.Close() }()

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := file.Read(buf)
			if nr > 0 {
				nw, ew := remoteFile.Write(buf[0:nr])
				if nw > 0 {
					written += int64(nw)
					if progress != nil {
						progress(written, fileInfo.Size())
					}
				}
				if ew != nil {
					return fmt.Errorf("write to remote: %w", ew)
				}
			}
			if er != nil {
				if er != io.EOF {
					return retry.IsNonRetryable(fmt.Errorf("read local file: %w", er))
				}
				break
			}
		}

		return nil
	}, fmt.Sprintf("SFTP upload %s", remoteFullPath))
}

// UploadStream uploads data from a reader via SFTP
func (s *SFTPStorage) UploadStream(ctx context.Context, reader io.Reader, remotePath string, size int64, progress ProgressCallback) error {
	remoteFullPath := s.buildPath(remotePath)

	s.log.Info("uploading stream via SFTP", "host", s.host, "path", remoteFullPath, "size", size)

	// Create remote directories
	remoteDir := filepath.Dir(remoteFullPath)
	if err := s.sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("create remote directory: %w", err)
	}

	// Create remote file
	remoteFile, err := s.sftpClient.Create(remoteFullPath)
	if err != nil {
		return fmt.Errorf("create remote file: %w", err)
	}
	defer func() { _ = remoteFile.Close() }()

	// Copy with progress
	written := int64(0)
	buf := make([]byte, 32*1024) // 32 KB buffer

	for {
		nr, er := reader.Read(buf)
		if nr > 0 {
			nw, ew := remoteFile.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				if progress != nil {
					progress(written, size)
				}
			}
			if ew != nil {
				return fmt.Errorf("write to remote: %w", ew)
			}
		}
		if er != nil {
			if er != io.EOF {
				return fmt.Errorf("read stream: %w", er)
			}
			break
		}
	}

	return nil
}

// Download downloads a file via SFTP
func (s *SFTPStorage) Download(ctx context.Context, remotePath, localPath string, progress ProgressCallback) error {
	remoteFullPath := s.buildPath(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			s.log.Info("downloading via SFTP", "host", s.host, "path", remoteFullPath)
		} else {
			s.log.Info("retrying SFTP download", "host", s.host, "path", remoteFullPath, "attempt", attempt)
		}

		// Open remote file
		remoteFile, err := s.sftpClient.Open(remoteFullPath)
		if err != nil {
			// File not found is not retryable
			if os.IsNotExist(err) {
				return retry.IsNonRetryable(fmt.Errorf("remote file not found: %w", err))
			}
			return fmt.Errorf("open remote file: %w", err)
		}
		defer func() { _ = remoteFile.Close() }()

		// Get file info for size
		remoteInfo, err := remoteFile.Stat()
		if err != nil {
			return fmt.Errorf("stat remote file: %w", err)
		}

		// Create local file
		// #nosec G304 -- localPath comes from the local operator's export job configuration, not remote/network input
		localFile, err := os.Create(localPath)
		if err != nil {
			return retry.IsNonRetryable(fmt.Errorf("create local file: %w", err))
		}
		defer func() { _ = localFile.Close() }()

		// Copy with progress
		written := int64(0)
		buf := make([]byte, 32*1024) // 32 KB buffer

		for {
			nr, er := remoteFile.Read(buf)
			if nr > 0 {
				nw, ew := localFile.Write(buf[0:nr])
				if nw > 0 {
					written += int64(nw)
					if progress != nil {
						progress(written, remoteInfo.Size())
					}
				}
				if ew != nil {
					return retry.IsNonRetryable(fmt.Errorf("write local file: %w", ew))
				}
			}
			if er != nil {
				if er != io.EOF {
					return fmt.Errorf("read remote file: %w", er)
				}
				break
			}
		}

		return nil
	}, fmt.Sprintf("SFTP download %s", remoteFullPath))
}

// List lists files via SFTP with a prefix
func (s *SFTPStorage) List(ctx context.Context, prefix string) ([]CloudFile, error) {
	remotePath := s.buildPath(prefix)

	result, err := s.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		if attempt == 1 {
			s.log.Debug("listing SFTP files", "host", s.host, "path", remotePath)
		} else {
			s.log.Debug("retrying SFTP list", "host", s.host, "path", remotePath, "attempt", attempt)
		}

		var files []CloudFile

		// Walk directory
		walker := s.sftpClient.Walk(remotePath)
		for walker.Step() {
			if err := walker.Err(); err != nil {
				return nil, fmt.Errorf("walk error: %w", err)
			}

			info := walker.Stat()
			if info.IsDir() {
				continue
			}

			files = append(files, CloudFile{
				Path:         walker.Path(),
				Size:         info.Size(),
				LastModified: info.ModTime(),
			})
		}

		return files, nil
	}, fmt.Sprintf("SFTP list %s", remotePath))

	if err != nil {
		return nil, err
	}

	return result.([]CloudFile), nil
}

// Delete deletes a file via SFTP
func (s *SFTPStorage) Delete(ctx context.Context, remotePath string) error {
	remoteFullPath := s.buildPath(remotePath)

	return s.retryer.Do(ctx, func(ctx context.Context, attempt int) error {
		if attempt == 1 {
			s.log.Info("deleting via SFTP", "host", s.host, "path", remoteFullPath)
		} else {
			s.log.Info("retrying SFTP delete", "host", s.host, "path", remoteFullPath, "attempt", attempt)
		}

		if err := s.sftpClient.Remove(remoteFullPath); err != nil {
			return fmt.Errorf("delete file: %w", err)
		}

		return nil
	}, fmt.Sprintf("SFTP delete %s", remoteFullPath))
}

// Exists checks if a file exists via SFTP
func (s *SFTPStorage) Exists(ctx context.Context, remotePath string) (bool, error) {
	remoteFullPath := s.buildPath(remotePath)

	result, err := s.retryer.DoWithResult(ctx, func(ctx context.Context, attempt int) (interface{}, error) {
		_, err := s.sftpClient.Stat(remoteFullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("stat file: %w", err)
		}

		return true, nil
	}, fmt.Sprintf("SFTP exists %s", remoteFullPath))

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// GetURL returns the SFTP URL
func (s *SFTPStorage) GetURL() string {
	return fmt.Sprintf("sftp://%s/%s", s.host, s.prefix)
}

// Close closes the SFTP connection
func (s *SFTPStorage) Close() error {
	if s.sftpClient != nil {
		if err := s.sftpClient.Close(); err != nil {
			s.log.Warn("failed to close SFTP client", "error", err)
		}
	}
	if s.sshClient != nil {
		if err := s.sshClient.Close(); err != nil {
			s.log.Warn("failed to close SSH client", "error", err)
		}
	}
	return nil
}

// buildPath builds the full path with prefix
func (s *SFTPStorage) buildPath(remotePath string) string {
	if s.prefix == "" {
		return remotePath
	}
	return filepath.Join(s.prefix, remotePath)
}

// getHostKeyCallback returns a secure host key callback for SSH
func getHostKeyCallback(hostKeyPath string) (ssh.HostKeyCallback, error) {
	// Determine known_hosts path
	knownHostsPath := hostKeyPath
	if knownHostsPath == "" {
		// Default to ~/.ssh/known_hosts
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		knownHostsPath = filepath.Join(homeDir, ".ssh", "known_hosts")
	}

	// Check if known_hosts file exists
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("known_hosts file not found: %s\n"+
			"Create it first by connecting via ssh or specify a different path with HostKeyPath", knownHostsPath)
	}

	// Load known hosts
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts from %s: %w", knownHostsPath, err)
	}

	return hostKeyCallback, nil
}
