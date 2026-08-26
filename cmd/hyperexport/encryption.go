// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"

	"github.com/zyvorai/transiva/logger"
)

// EncryptionMethod defines the encryption method
type EncryptionMethod string

const (
	EncryptionAES256 EncryptionMethod = "aes256"
	EncryptionGPG    EncryptionMethod = "gpg"
)

// EncryptionConfig contains encryption configuration
type EncryptionConfig struct {
	Method       EncryptionMethod
	Passphrase   string
	KeyFile      string
	GPGRecipient string
}

// Encryptor handles file encryption
type Encryptor struct {
	config *EncryptionConfig
	log    logger.Logger
}

// NewEncryptor creates a new encryptor
func NewEncryptor(config *EncryptionConfig, log logger.Logger) *Encryptor {
	return &Encryptor{
		config: config,
		log:    log,
	}
}

// EncryptFile encrypts a single file
func (e *Encryptor) EncryptFile(inputPath, outputPath string) error {
	switch e.config.Method {
	case EncryptionAES256:
		return e.encryptFileAES256(inputPath, outputPath)
	case EncryptionGPG:
		return e.encryptFileGPG(inputPath, outputPath)
	default:
		return fmt.Errorf("unsupported encryption method: %s", e.config.Method)
	}
}

// encryptFileAES256 encrypts a file using AES-256-GCM
func (e *Encryptor) encryptFileAES256(inputPath, outputPath string) error {
	e.log.Info("encrypting file with AES-256", "input", inputPath)

	// Derive encryption key from passphrase
	var key []byte
	if e.config.KeyFile != "" {
		// Read key from file
		keyData, err := os.ReadFile(e.config.KeyFile)
		if err != nil {
			return fmt.Errorf("read key file: %w", err)
		}
		key = deriveKey(string(keyData), []byte("hyperexport-salt"))
	} else if e.config.Passphrase != "" {
		key = deriveKey(e.config.Passphrase, []byte("hyperexport-salt"))
	} else {
		return fmt.Errorf("no passphrase or key file provided")
	}

	// Open input file
	// #nosec G304 -- inputPath comes from the local operator's export CLI arguments, not remote/network input
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input file: %w", err)
	}
	defer func() {
		if closeErr := inputFile.Close(); closeErr != nil {
			e.log.Warn("failed to close input file", "path", inputPath, "error", closeErr)
		}
	}()

	// Create output file
	// #nosec G304 -- outputPath comes from the local operator's export CLI arguments, not remote/network input
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() {
		if closeErr := outputFile.Close(); closeErr != nil {
			e.log.Warn("failed to close output file", "path", outputPath, "error", closeErr)
		}
	}()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	// Write nonce to output file
	if _, err := outputFile.Write(nonce); err != nil {
		return fmt.Errorf("write nonce: %w", err)
	}

	// Encrypt and write data in chunks
	buf := make([]byte, 64*1024) // 64 KB chunks
	for {
		n, err := inputFile.Read(buf)
		if n > 0 {
			// Encrypt chunk. The nonce is freshly randomized for this file and
			// deterministically incremented after every chunk (see incrementNonce),
			// so it is never reused with the same key -- this is not a hardcoded nonce.
			// #nosec G407 -- nonce is randomly generated per file and incremented per chunk, never reused
			ciphertext := gcm.Seal(nil, nonce, buf[:n], nil)

			// Write encrypted chunk size (4 bytes). Chunk length is bounded by the
			// 64KB read buffer plus GCM overhead, well within a 32-bit range.
			sizeBytes := []byte{
				byte(len(ciphertext) >> 24), // #nosec G115 -- chunk length bounded by 64KB read buffer, safe truncation
				byte(len(ciphertext) >> 16), // #nosec G115 -- chunk length bounded by 64KB read buffer, safe truncation
				byte(len(ciphertext) >> 8),  // #nosec G115 -- chunk length bounded by 64KB read buffer, safe truncation
				byte(len(ciphertext)),       // #nosec G115 -- chunk length bounded by 64KB read buffer, safe truncation
			}
			if _, err := outputFile.Write(sizeBytes); err != nil {
				return fmt.Errorf("write chunk size: %w", err)
			}

			// Write encrypted chunk
			if _, err := outputFile.Write(ciphertext); err != nil {
				return fmt.Errorf("write encrypted chunk: %w", err)
			}

			// Update nonce for next chunk (increment)
			incrementNonce(nonce)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read input: %w", err)
		}
	}

	e.log.Info("file encrypted successfully", "output", outputPath)
	return nil
}

// encryptFileGPG encrypts a file using GPG
func (e *Encryptor) encryptFileGPG(inputPath, outputPath string) error {
	e.log.Info("encrypting file with GPG", "input", inputPath, "recipient", e.config.GPGRecipient)

	// Check if gpg is available
	if _, err := exec.LookPath("gpg"); err != nil {
		return fmt.Errorf("gpg not found in PATH: %w", err)
	}

	// Build GPG command
	args := []string{
		"--encrypt",
		"--output", outputPath,
	}

	if e.config.GPGRecipient != "" {
		args = append(args, "--recipient", e.config.GPGRecipient)
	} else if e.config.Passphrase != "" {
		// Symmetric encryption
		args = append(args,
			"--symmetric",
			"--batch",
			"--passphrase", e.config.Passphrase,
		)
	} else {
		return fmt.Errorf("no recipient or passphrase provided for GPG")
	}

	args = append(args, inputPath)

	// Execute GPG
	// #nosec G204 -- args are built from local operator-supplied export config (recipient/passphrase/paths), not remote input
	cmd := exec.Command("gpg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg encryption failed: %w\nOutput: %s", err, string(output))
	}

	e.log.Info("file encrypted with GPG successfully", "output", outputPath)
	return nil
}

// DecryptFile decrypts a single file
func (e *Encryptor) DecryptFile(inputPath, outputPath string) error {
	switch e.config.Method {
	case EncryptionAES256:
		return e.decryptFileAES256(inputPath, outputPath)
	case EncryptionGPG:
		return e.decryptFileGPG(inputPath, outputPath)
	default:
		return fmt.Errorf("unsupported encryption method: %s", e.config.Method)
	}
}

// decryptFileAES256 decrypts a file using AES-256-GCM
func (e *Encryptor) decryptFileAES256(inputPath, outputPath string) error {
	e.log.Info("decrypting file with AES-256", "input", inputPath)

	// Derive encryption key
	var key []byte
	if e.config.KeyFile != "" {
		keyData, err := os.ReadFile(e.config.KeyFile)
		if err != nil {
			return fmt.Errorf("read key file: %w", err)
		}
		key = deriveKey(string(keyData), []byte("hyperexport-salt"))
	} else if e.config.Passphrase != "" {
		key = deriveKey(e.config.Passphrase, []byte("hyperexport-salt"))
	} else {
		return fmt.Errorf("no passphrase or key file provided")
	}

	// Open input file
	// #nosec G304 -- inputPath comes from the local operator's export CLI arguments, not remote/network input
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input file: %w", err)
	}
	defer func() {
		if closeErr := inputFile.Close(); closeErr != nil {
			e.log.Warn("failed to close input file", "path", inputPath, "error", closeErr)
		}
	}()

	// Create output file
	// #nosec G304 -- outputPath comes from the local operator's export CLI arguments, not remote/network input
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer func() {
		if closeErr := outputFile.Close(); closeErr != nil {
			e.log.Warn("failed to close output file", "path", outputPath, "error", closeErr)
		}
	}()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	// Read nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(inputFile, nonce); err != nil {
		return fmt.Errorf("read nonce: %w", err)
	}

	// Decrypt data in chunks
	for {
		// Read chunk size
		sizeBytes := make([]byte, 4)
		if _, err := io.ReadFull(inputFile, sizeBytes); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read chunk size: %w", err)
		}

		size := int(sizeBytes[0])<<24 | int(sizeBytes[1])<<16 | int(sizeBytes[2])<<8 | int(sizeBytes[3])

		// Read encrypted chunk
		ciphertext := make([]byte, size)
		if _, err := io.ReadFull(inputFile, ciphertext); err != nil {
			return fmt.Errorf("read encrypted chunk: %w", err)
		}

		// Decrypt chunk
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return fmt.Errorf("decrypt chunk: %w", err)
		}

		// Write decrypted data
		if _, err := outputFile.Write(plaintext); err != nil {
			return fmt.Errorf("write decrypted data: %w", err)
		}

		// Update nonce for next chunk
		incrementNonce(nonce)
	}

	e.log.Info("file decrypted successfully", "output", outputPath)
	return nil
}

// decryptFileGPG decrypts a file using GPG
func (e *Encryptor) decryptFileGPG(inputPath, outputPath string) error {
	e.log.Info("decrypting file with GPG", "input", inputPath)

	// Check if gpg is available
	if _, err := exec.LookPath("gpg"); err != nil {
		return fmt.Errorf("gpg not found in PATH: %w", err)
	}

	// Build GPG command
	args := []string{
		"--decrypt",
		"--output", outputPath,
	}

	if e.config.Passphrase != "" {
		args = append(args,
			"--batch",
			"--passphrase", e.config.Passphrase,
		)
	}

	args = append(args, inputPath)

	// Execute GPG
	// #nosec G204 -- args are built from local operator-supplied export config (passphrase/paths), not remote input
	cmd := exec.Command("gpg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg decryption failed: %w\nOutput: %s", err, string(output))
	}

	e.log.Info("file decrypted with GPG successfully", "output", outputPath)
	return nil
}

// EncryptDirectory encrypts all files in a directory
func (e *Encryptor) EncryptDirectory(inputDir, outputDir string) error {
	e.log.Info("encrypting directory", "input", inputDir, "output", outputDir)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Walk input directory
	return filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(inputDir, path)
		if err != nil {
			return err
		}

		// Build output path with .enc extension
		outputPath := filepath.Join(outputDir, relPath+".enc")

		// Create output directory for this file
		if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		// Encrypt file
		if err := e.EncryptFile(path, outputPath); err != nil {
			return fmt.Errorf("encrypt %s: %w", path, err)
		}

		return nil
	})
}

// deriveKey derives an encryption key from a passphrase using PBKDF2
func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)
}

// incrementNonce increments a nonce for the next encryption
func incrementNonce(nonce []byte) {
	for i := len(nonce) - 1; i >= 0; i-- {
		nonce[i]++
		if nonce[i] != 0 {
			break
		}
	}
}

// GetEncryptedFilename returns the encrypted filename
func GetEncryptedFilename(original string, method EncryptionMethod) string {
	switch method {
	case EncryptionGPG:
		return original + ".gpg"
	case EncryptionAES256:
		return original + ".enc"
	default:
		return original + ".encrypted"
	}
}

// IsEncryptedFile checks if a file is encrypted based on extension
func IsEncryptedFile(filename string) bool {
	return strings.HasSuffix(filename, ".enc") ||
		strings.HasSuffix(filename, ".gpg") ||
		strings.HasSuffix(filename, ".encrypted")
}
