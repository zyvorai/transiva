// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/zyvorai/transiva/logger"
)

func TestNewEncryptor(t *testing.T) {
	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test-passphrase",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))
	if encryptor == nil {
		t.Fatal("NewEncryptor returned nil")
	}
	if encryptor.config != config {
		t.Error("Config not set correctly")
	}
}

func TestEncryptionConfig_AES256(t *testing.T) {
	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "strong-passphrase-123",
	}

	if config.Method != EncryptionAES256 {
		t.Error("Method mismatch")
	}
	if config.Passphrase == "" {
		t.Error("Passphrase should not be empty")
	}
}

func TestEncryptionConfig_GPG(t *testing.T) {
	config := &EncryptionConfig{
		Method:       EncryptionGPG,
		GPGRecipient: "user@example.com",
	}

	if config.Method != EncryptionGPG {
		t.Error("Method mismatch")
	}
	if config.GPGRecipient == "" {
		t.Error("GPG recipient should not be empty")
	}
}

func TestEncryptFile_AES256_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	inputFile := filepath.Join(tmpDir, "test.txt")
	originalData := []byte("This is secret test data for encryption!")
	if err := os.WriteFile(inputFile, originalData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test-passphrase-12345",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	// Encrypt
	encryptedFile := filepath.Join(tmpDir, "test.txt.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Verify encrypted file exists
	if _, err := os.Stat(encryptedFile); os.IsNotExist(err) {
		t.Fatal("Encrypted file was not created")
	}

	// Verify encrypted data is different from original
	encryptedData, err := os.ReadFile(encryptedFile)
	if err != nil {
		t.Fatalf("Failed to read encrypted file: %v", err)
	}
	if bytes.Equal(encryptedData, originalData) {
		t.Error("Encrypted data should be different from original")
	}

	// Decrypt
	decryptedFile := filepath.Join(tmpDir, "test.txt.dec")
	err = encryptor.DecryptFile(encryptedFile, decryptedFile)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify decrypted data matches original
	decryptedData, err := os.ReadFile(decryptedFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if !bytes.Equal(decryptedData, originalData) {
		t.Errorf("Decrypted data doesn't match original.\nOriginal: %s\nDecrypted: %s",
			string(originalData), string(decryptedData))
	}
}

func TestEncryptFile_AES256_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create large test file (1 MB)
	inputFile := filepath.Join(tmpDir, "large.bin")
	largeData := make([]byte, 1*1024*1024) // 1 MB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	if err := os.WriteFile(inputFile, largeData, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test-passphrase",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	// Encrypt
	encryptedFile := filepath.Join(tmpDir, "large.bin.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Decrypt
	decryptedFile := filepath.Join(tmpDir, "large.bin.dec")
	err = encryptor.DecryptFile(encryptedFile, decryptedFile)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify
	decryptedData, err := os.ReadFile(decryptedFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if !bytes.Equal(decryptedData, largeData) {
		t.Error("Decrypted large file doesn't match original")
	}
}

func TestEncryptFile_AES256_WithKeyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create key file
	keyFile := filepath.Join(tmpDir, "key.txt")
	keyData := []byte("this-is-a-secret-key-12345")
	if err := os.WriteFile(keyFile, keyData, 0600); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	// Create test file
	inputFile := filepath.Join(tmpDir, "test.txt")
	originalData := []byte("Secret data with key file")
	if err := os.WriteFile(inputFile, originalData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &EncryptionConfig{
		Method:  EncryptionAES256,
		KeyFile: keyFile,
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	// Encrypt
	encryptedFile := filepath.Join(tmpDir, "test.txt.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Decrypt
	decryptedFile := filepath.Join(tmpDir, "test.txt.dec")
	err = encryptor.DecryptFile(encryptedFile, decryptedFile)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify
	decryptedData, err := os.ReadFile(decryptedFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if !bytes.Equal(decryptedData, originalData) {
		t.Error("Decrypted data doesn't match original (key file)")
	}
}

func TestEncryptFile_AES256_NoPassphrase(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(inputFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &EncryptionConfig{
		Method: EncryptionAES256,
		// No passphrase or key file
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	encryptedFile := filepath.Join(tmpDir, "test.txt.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err == nil {
		t.Error("Expected error when no passphrase or key file provided")
	}
}

func TestEncryptFile_AES256_WrongPassphrase(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and encrypt file
	inputFile := filepath.Join(tmpDir, "test.txt")
	originalData := []byte("Secret data")
	if err := os.WriteFile(inputFile, originalData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "correct-passphrase",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	encryptedFile := filepath.Join(tmpDir, "test.txt.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Try to decrypt with wrong passphrase
	wrongConfig := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "wrong-passphrase",
	}

	wrongEncryptor := NewEncryptor(wrongConfig, logger.NewTestLogger(t))

	decryptedFile := filepath.Join(tmpDir, "test.txt.dec")
	err = wrongEncryptor.DecryptFile(encryptedFile, decryptedFile)
	if err == nil {
		t.Error("Expected error when decrypting with wrong passphrase")
	}
}

func TestEncryptFile_NonExistentInput(t *testing.T) {
	tmpDir := t.TempDir()

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	err := encryptor.EncryptFile("/nonexistent/file.txt", filepath.Join(tmpDir, "out.enc"))
	if err == nil {
		t.Error("Expected error for nonexistent input file")
	}
}

func TestEncryptFile_UnsupportedMethod(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(inputFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionMethod("unsupported"),
		Passphrase: "test",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	err := encryptor.EncryptFile(inputFile, filepath.Join(tmpDir, "out.enc"))
	if err == nil {
		t.Error("Expected error for unsupported encryption method")
	}
}

func TestEncryptDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	inputDir := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(filepath.Join(inputDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create input dir: %v", err)
	}

	// Create test files
	files := map[string]string{
		"file1.txt":        "Content 1",
		"file2.txt":        "Content 2",
		"subdir/file3.txt": "Content 3",
	}

	for path, content := range files {
		fullPath := filepath.Join(inputDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test-passphrase",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	// Encrypt directory
	outputDir := filepath.Join(tmpDir, "encrypted")
	err := encryptor.EncryptDirectory(inputDir, outputDir)
	if err != nil {
		t.Fatalf("EncryptDirectory failed: %v", err)
	}

	// Verify encrypted files exist
	for path := range files {
		encryptedPath := filepath.Join(outputDir, path+".enc")
		if _, err := os.Stat(encryptedPath); os.IsNotExist(err) {
			t.Errorf("Encrypted file not found: %s", encryptedPath)
		}
	}

	// Decrypt one file to verify
	decryptedFile := filepath.Join(tmpDir, "file1.txt.dec")
	err = encryptor.DecryptFile(filepath.Join(outputDir, "file1.txt.enc"), decryptedFile)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	decryptedData, err := os.ReadFile(decryptedFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if string(decryptedData) != "Content 1" {
		t.Errorf("Expected 'Content 1', got %q", string(decryptedData))
	}
}

func TestDeriveKey(t *testing.T) {
	passphrase := "test-passphrase"
	salt := []byte("test-salt")

	key1 := deriveKey(passphrase, salt)
	key2 := deriveKey(passphrase, salt)

	// Same passphrase and salt should produce same key
	if !bytes.Equal(key1, key2) {
		t.Error("Same passphrase+salt should produce same key")
	}

	// Key should be 32 bytes (256 bits)
	if len(key1) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key1))
	}

	// Different passphrase should produce different key
	key3 := deriveKey("different-passphrase", salt)
	if bytes.Equal(key1, key3) {
		t.Error("Different passphrase should produce different key")
	}

	// Different salt should produce different key
	key4 := deriveKey(passphrase, []byte("different-salt"))
	if bytes.Equal(key1, key4) {
		t.Error("Different salt should produce different key")
	}
}

func TestIncrementNonce(t *testing.T) {
	nonce := make([]byte, 12)

	// Initial nonce should be all zeros
	for i := range nonce {
		if nonce[i] != 0 {
			t.Error("Initial nonce should be all zeros")
		}
	}

	// Increment once
	incrementNonce(nonce)
	if nonce[11] != 1 {
		t.Errorf("After one increment, last byte should be 1, got %d", nonce[11])
	}

	// Increment 255 more times to test overflow
	for i := 0; i < 255; i++ {
		incrementNonce(nonce)
	}
	// Should have overflowed: [0,0,0,0,0,0,0,0,0,0,1,0]
	if nonce[11] != 0 {
		t.Errorf("After 256 increments, last byte should be 0, got %d", nonce[11])
	}
	if nonce[10] != 1 {
		t.Errorf("After 256 increments, second-to-last byte should be 1, got %d", nonce[10])
	}
}

func TestGetEncryptedFilename(t *testing.T) {
	tests := []struct {
		name     string
		original string
		method   EncryptionMethod
		expected string
	}{
		{"AES256", "file.txt", EncryptionAES256, "file.txt.enc"},
		{"GPG", "file.txt", EncryptionGPG, "file.txt.gpg"},
		{"Unknown", "file.txt", EncryptionMethod("unknown"), "file.txt.encrypted"},
		{"Path with dir", "/path/to/file.ova", EncryptionAES256, "/path/to/file.ova.enc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEncryptedFilename(tt.original, tt.method)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsEncryptedFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"Enc file", "file.txt.enc", true},
		{"GPG file", "file.txt.gpg", true},
		{"Encrypted file", "file.txt.encrypted", true},
		{"Plain file", "file.txt", false},
		{"Ova file", "vm.ova", false},
		{"Partial match", "encrypted.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEncryptedFile(tt.filename)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for %q", tt.expected, result, tt.filename)
			}
		})
	}
}

func TestEncryptFile_AES256_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty file
	inputFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(inputFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	// Encrypt
	encryptedFile := filepath.Join(tmpDir, "empty.txt.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Decrypt
	decryptedFile := filepath.Join(tmpDir, "empty.txt.dec")
	err = encryptor.DecryptFile(encryptedFile, decryptedFile)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify
	decryptedData, err := os.ReadFile(decryptedFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if len(decryptedData) != 0 {
		t.Errorf("Expected empty file, got %d bytes", len(decryptedData))
	}
}

func TestEncryptFile_AES256_BinaryData(t *testing.T) {
	tmpDir := t.TempDir()

	// Create binary file with all byte values
	inputFile := filepath.Join(tmpDir, "binary.bin")
	binaryData := make([]byte, 256)
	for i := range binaryData {
		binaryData[i] = byte(i)
	}
	if err := os.WriteFile(inputFile, binaryData, 0644); err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	// Encrypt
	encryptedFile := filepath.Join(tmpDir, "binary.bin.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Decrypt
	decryptedFile := filepath.Join(tmpDir, "binary.bin.dec")
	err = encryptor.DecryptFile(encryptedFile, decryptedFile)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify
	decryptedData, err := os.ReadFile(decryptedFile)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if !bytes.Equal(decryptedData, binaryData) {
		t.Error("Decrypted binary data doesn't match original")
	}
}

func TestEncryptDirectory_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	inputDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("Failed to create empty dir: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionAES256,
		Passphrase: "test",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	outputDir := filepath.Join(tmpDir, "encrypted")
	err := encryptor.EncryptDirectory(inputDir, outputDir)
	if err != nil {
		t.Fatalf("EncryptDirectory failed: %v", err)
	}

	// Output dir should exist but be empty (except for subdirs)
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("Output directory should exist")
	}
}

func TestEncryptionMethods_Constants(t *testing.T) {
	if EncryptionAES256 != "aes256" {
		t.Errorf("EncryptionAES256 constant mismatch: %s", EncryptionAES256)
	}
	if EncryptionGPG != "gpg" {
		t.Errorf("EncryptionGPG constant mismatch: %s", EncryptionGPG)
	}
}

func TestDecryptFile_UnsupportedMethod(t *testing.T) {
	tmpDir := t.TempDir()

	encryptedFile := filepath.Join(tmpDir, "test.enc")
	if err := os.WriteFile(encryptedFile, []byte("fake encrypted data"), 0644); err != nil {
		t.Fatalf("Failed to create encrypted file: %v", err)
	}

	config := &EncryptionConfig{
		Method:     EncryptionMethod("unsupported"),
		Passphrase: "test",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	decryptedFile := filepath.Join(tmpDir, "test.dec")
	err := encryptor.DecryptFile(encryptedFile, decryptedFile)
	if err == nil {
		t.Error("Expected error for unsupported decryption method")
	}
}

func TestEncryptFile_AES256_NonExistentKeyFile(t *testing.T) {
	tmpDir := t.TempDir()

	inputFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(inputFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &EncryptionConfig{
		Method:  EncryptionAES256,
		KeyFile: "/nonexistent/key.txt",
	}

	encryptor := NewEncryptor(config, logger.NewTestLogger(t))

	encryptedFile := filepath.Join(tmpDir, "test.txt.enc")
	err := encryptor.EncryptFile(inputFile, encryptedFile)
	if err == nil {
		t.Error("Expected error for nonexistent key file")
	}
}
