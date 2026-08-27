// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyvorai/transiva/logger"
)

func TestNewProfileManager(t *testing.T) {
	pm, err := NewProfileManager(logger.NewTestLogger(t))
	if err != nil {
		t.Fatalf("NewProfileManager failed: %v", err)
	}

	if pm == nil {
		t.Fatal("NewProfileManager returned nil")
	}
	if pm.profilesDir == "" {
		t.Error("Profiles directory should be set")
	}

	// Verify directory was created
	if _, err := os.Stat(pm.profilesDir); os.IsNotExist(err) {
		t.Error("Profiles directory should have been created")
	}
}

func TestProfileManager_SaveProfile(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log:         logger.NewTestLogger(t),
	}

	profile := &ExportProfile{
		Name:        "test-profile",
		Description: "Test profile description",
		Format:      "ova",
		Compress:    true,
		Verify:      true,
		PowerOff:    false,
		Parallel:    4,
	}

	err := pm.SaveProfile(profile)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Verify file was created
	profilePath := filepath.Join(tmpDir, "test-profile.json")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Error("Profile file should have been created")
	}

	// Verify timestamps were set
	if profile.Created.IsZero() {
		t.Error("Created timestamp should be set")
	}
	if profile.Modified.IsZero() {
		t.Error("Modified timestamp should be set")
	}
}

func TestProfileManager_SaveProfile_NoName(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	profile := &ExportProfile{
		Description: "Profile without name",
		Format:      "ova",
	}

	err := pm.SaveProfile(profile)
	if err == nil {
		t.Error("SaveProfile should fail when name is empty")
	}
}

func TestProfileManager_LoadProfile(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	// Save a profile
	original := &ExportProfile{
		Name:        "test-profile",
		Description: "Test description",
		Format:      "ova",
		Compress:    true,
		Verify:      false,
		Parallel:    4,
		Tags:        map[string]string{"env": "test"},
	}
	_ = pm.SaveProfile(original)

	// Load it back
	loaded, err := pm.LoadProfile("test-profile")
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	if loaded.Name != original.Name {
		t.Error("Name mismatch after load")
	}
	if loaded.Description != original.Description {
		t.Error("Description mismatch after load")
	}
	if loaded.Format != original.Format {
		t.Error("Format mismatch after load")
	}
	if loaded.Compress != original.Compress {
		t.Error("Compress mismatch after load")
	}
	if loaded.Tags["env"] != "test" {
		t.Error("Tags mismatch after load")
	}
}

func TestProfileManager_LoadProfile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	_, err := pm.LoadProfile("nonexistent")
	if err == nil {
		t.Error("LoadProfile should fail for nonexistent profile")
	}
}

func TestProfileManager_ListProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	// Save multiple profiles
	profiles := []*ExportProfile{
		{Name: "profile1", Description: "First", Format: "ova"},
		{Name: "profile2", Description: "Second", Format: "ovf"},
		{Name: "profile3", Description: "Third", Format: "vmdk"},
	}

	for _, profile := range profiles {
		if err := pm.SaveProfile(profile); err != nil {
			t.Fatalf("SaveProfile failed: %v", err)
		}
	}

	// List all profiles
	listed, err := pm.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(listed))
	}
}

func TestProfileManager_ListProfiles_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	profiles, err := pm.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}

	if len(profiles) != 0 {
		t.Errorf("Expected 0 profiles in empty directory, got %d", len(profiles))
	}
}

func TestProfileManager_DeleteProfile(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	// Save a profile
	profile := &ExportProfile{
		Name:   "to-delete",
		Format: "ova",
	}
	_ = pm.SaveProfile(profile)

	// Verify it exists
	if !pm.ProfileExists("to-delete") {
		t.Error("Profile should exist before deletion")
	}

	// Delete it
	err := pm.DeleteProfile("to-delete")
	if err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	// Verify it's gone
	if pm.ProfileExists("to-delete") {
		t.Error("Profile should not exist after deletion")
	}
}

func TestProfileManager_DeleteProfile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	err := pm.DeleteProfile("nonexistent")
	if err == nil {
		t.Error("DeleteProfile should fail for nonexistent profile")
	}
}

func TestProfileManager_ProfileExists(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	// Should not exist initially
	if pm.ProfileExists("test-profile") {
		t.Error("Profile should not exist yet")
	}

	// Save a profile
	profile := &ExportProfile{
		Name:   "test-profile",
		Format: "ova",
	}
	_ = pm.SaveProfile(profile)

	// Should exist now
	if !pm.ProfileExists("test-profile") {
		t.Error("Profile should exist after saving")
	}
}

func TestProfileManager_CreateDefaultProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	err := pm.CreateDefaultProfiles()
	if err != nil {
		t.Fatalf("CreateDefaultProfiles failed: %v", err)
	}

	// Verify default profiles were created
	defaultProfiles := []string{
		"quick-export",
		"production-backup",
		"encrypted-backup",
		"cloud-backup",
		"development",
	}

	for _, name := range defaultProfiles {
		if !pm.ProfileExists(name) {
			t.Errorf("Default profile %s should exist", name)
		}
	}
}

func TestApplyProfileToFlags(t *testing.T) {
	profile := &ExportProfile{
		Name:          "test-profile",
		Format:        "ova",
		Compress:      true,
		Verify:        true,
		PowerOff:      true,
		Parallel:      8,
		UploadTo:      "s3://bucket/path",
		KeepLocal:     false,
		Encrypt:       true,
		EncryptMethod: "aes256",
		GPGRecipient:  "user@example.com",
		ValidateOnly:  false,
	}

	flags := ApplyProfileToFlags(profile)

	if flags["format"] != "ova" {
		t.Error("Format flag mismatch")
	}
	if flags["compress"] != true {
		t.Error("Compress flag mismatch")
	}
	if flags["verify"] != true {
		t.Error("Verify flag mismatch")
	}
	if flags["power-off"] != true {
		t.Error("PowerOff flag mismatch")
	}
	if flags["parallel"] != 8 {
		t.Error("Parallel flag mismatch")
	}
	if flags["upload"] != "s3://bucket/path" {
		t.Error("Upload flag mismatch")
	}
	if flags["keep-local"] != false {
		t.Error("KeepLocal flag mismatch")
	}
	if flags["encrypt"] != true {
		t.Error("Encrypt flag mismatch")
	}
	if flags["encrypt-method"] != "aes256" {
		t.Error("EncryptMethod flag mismatch")
	}
	if flags["gpg-recipient"] != "user@example.com" {
		t.Error("GPGRecipient flag mismatch")
	}
}

func TestApplyProfileToFlags_NoEncryption(t *testing.T) {
	profile := &ExportProfile{
		Name:     "no-encrypt",
		Format:   "ovf",
		Encrypt:  false,
		Parallel: 4,
	}

	flags := ApplyProfileToFlags(profile)

	if _, hasEncrypt := flags["encrypt"]; hasEncrypt {
		if flags["encrypt"] == true {
			t.Error("Encrypt should not be true when disabled in profile")
		}
	}
}

func TestExportProfile_Fields(t *testing.T) {
	now := time.Now()
	tags := map[string]string{"env": "prod"}

	profile := ExportProfile{
		Name:                 "test-profile",
		Description:          "Test description",
		Created:              now,
		Modified:             now,
		Format:               "ova",
		Compress:             true,
		Verify:               true,
		PowerOff:             false,
		Parallel:             4,
		CleanupOVF:           true,
		UploadTo:             "s3://bucket",
		KeepLocal:            false,
		Encrypt:              true,
		EncryptMethod:        "aes256",
		GPGRecipient:         "user@example.com",
		ValidateOnly:         false,
		GenerateManifest:     true,
		VerifyManifest:       true,
		ManifestChecksum:     true,
		ManifestTargetFormat: "qcow2",
		AutoConvert:          false,
		H2KVMBinary:          "/usr/bin/h2kvm",
		StreamConversion:     false,
		RetentionDays:        30,
		RetentionCount:       10,
		NotifyEmail:          "admin@example.com",
		NotifySlack:          "https://hooks.slack.com/...",
		NotifyDiscord:        "https://discord.com/...",
		Tags:                 tags,
	}

	if profile.Name != "test-profile" {
		t.Error("Name mismatch")
	}
	if !profile.Compress {
		t.Error("Compress should be true")
	}
	if !profile.Encrypt {
		t.Error("Encrypt should be true")
	}
	if profile.Tags["env"] != "prod" {
		t.Error("Tags mismatch")
	}
}

func TestProfileManager_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	// Create a profile with all fields set
	original := &ExportProfile{
		Name:             "full-profile",
		Description:      "Profile with all fields",
		Format:           "ova",
		Compress:         true,
		Verify:           true,
		PowerOff:         true,
		Parallel:         8,
		CleanupOVF:       true,
		UploadTo:         "s3://bucket",
		KeepLocal:        false,
		Encrypt:          true,
		EncryptMethod:    "aes256",
		GPGRecipient:     "user@example.com",
		ValidateOnly:     false,
		RetentionDays:    30,
		RetentionCount:   10,
		NotifyEmail:      "admin@example.com",
		Tags:             map[string]string{"env": "prod", "team": "ops"},
	}

	// Save
	if err := pm.SaveProfile(original); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Load
	loaded, err := pm.LoadProfile("full-profile")
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	// Verify all fields
	if loaded.Name != original.Name {
		t.Error("Name mismatch after round-trip")
	}
	if loaded.Description != original.Description {
		t.Error("Description mismatch after round-trip")
	}
	if loaded.Format != original.Format {
		t.Error("Format mismatch after round-trip")
	}
	if loaded.Compress != original.Compress {
		t.Error("Compress mismatch after round-trip")
	}
	if loaded.Encrypt != original.Encrypt {
		t.Error("Encrypt mismatch after round-trip")
	}
	if loaded.RetentionDays != original.RetentionDays {
		t.Error("RetentionDays mismatch after round-trip")
	}
	if loaded.Tags["env"] != "prod" || loaded.Tags["team"] != "ops" {
		t.Error("Tags mismatch after round-trip")
	}
}

func TestProfileManager_UpdateProfile(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	// Save initial profile
	profile := &ExportProfile{
		Name:        "update-test",
		Description: "Original description",
		Format:      "ovf",
		Compress:    false,
	}
	_ = pm.SaveProfile(profile)
	originalModified := profile.Modified

	// Wait a bit to ensure Modified timestamp changes
	time.Sleep(10 * time.Millisecond)

	// Update the profile
	profile.Description = "Updated description"
	profile.Compress = true
	_ = pm.SaveProfile(profile)

	// Load and verify
	loaded, err := pm.LoadProfile("update-test")
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	if loaded.Description != "Updated description" {
		t.Error("Description should be updated")
	}
	if !loaded.Compress {
		t.Error("Compress should be updated")
	}
	if !loaded.Modified.After(originalModified) {
		t.Error("Modified timestamp should be updated")
	}
}

func TestProfileManager_ListProfiles_IgnoresNonJSON(t *testing.T) {
	tmpDir := t.TempDir()
	pm := &ProfileManager{
		profilesDir: tmpDir,
		log: logger.NewTestLogger(t),
	}

	// Create a valid profile
	profile := &ExportProfile{
		Name:   "valid-profile",
		Format: "ova",
	}
	_ = pm.SaveProfile(profile)

	// Create a non-JSON file
	nonJSONFile := filepath.Join(tmpDir, "not-a-profile.txt")
	_ = os.WriteFile(nonJSONFile, []byte("not json"), 0644)

	// Create a subdirectory
	subdir := filepath.Join(tmpDir, "subdir")
	_ = os.Mkdir(subdir, 0755)

	// List profiles
	profiles, err := pm.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}

	// Should only have the valid profile
	if len(profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "valid-profile" {
		t.Error("Wrong profile returned")
	}
}
