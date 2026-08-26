// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// profileNamePattern restricts profile names to safe filesystem identifiers,
// preventing path traversal when the name is joined into a file path under
// the profiles directory.
var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func validProfileName(name string) bool {
	return profileNamePattern.MatchString(name)
}

// ExportProfile defines an export configuration profile
type ExportProfile struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`

	// Export settings
	Format     string `json:"format"`      // ovf, ova
	Compress   bool   `json:"compress"`    // Enable compression
	Verify     bool   `json:"verify"`      // Verify with checksums
	PowerOff   bool   `json:"power_off"`   // Power off VM before export
	Parallel   int    `json:"parallel"`    // Parallel downloads
	CleanupOVF bool   `json:"cleanup_ovf"` // Cleanup OVF files after OVA creation

	// Cloud storage
	UploadTo  string `json:"upload_to"`  // Cloud storage URL
	KeepLocal bool   `json:"keep_local"` // Keep local copy after upload

	// Encryption
	Encrypt       bool   `json:"encrypt"`        // Enable encryption
	EncryptMethod string `json:"encrypt_method"` // aes256, gpg
	GPGRecipient  string `json:"gpg_recipient"`  // GPG recipient (don't store passphrase!)

	// Validation
	ValidateOnly bool `json:"validate_only"` // Only run validation

	// Artifact Manifest v1.0
	GenerateManifest     bool   `json:"generate_manifest"`      // Generate Artifact Manifest v1.0
	VerifyManifest       bool   `json:"verify_manifest"`        // Verify manifest after generation
	ManifestChecksum     bool   `json:"manifest_checksum"`      // Compute checksums in manifest
	ManifestTargetFormat string `json:"manifest_target_format"` // Target format for conversion

	// Automatic conversion (Phase 2)
	AutoConvert      bool   `json:"auto_convert"`      // Automatically convert with hyper2kvm
	Hyper2KVMBinary  string `json:"hyper2kvm_binary"`  // Path to hyper2kvm binary
	StreamConversion bool   `json:"stream_conversion"` // Stream conversion output

	// Retention
	RetentionDays  int `json:"retention_days"`  // Keep exports for N days
	RetentionCount int `json:"retention_count"` // Keep last N exports

	// Notifications
	NotifyEmail   string `json:"notify_email"`   // Email notification
	NotifySlack   string `json:"notify_slack"`   // Slack webhook
	NotifyDiscord string `json:"notify_discord"` // Discord webhook

	// Metadata
	Tags map[string]string `json:"tags"` // Custom tags
}

// ProfileManager manages export profiles
type ProfileManager struct {
	profilesDir string
	log         logger.Logger
}

// NewProfileManager creates a new profile manager
func NewProfileManager(log logger.Logger) (*ProfileManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	profilesDir := filepath.Join(homeDir, ".hyperexport", "profiles")
	if err := os.MkdirAll(profilesDir, 0750); err != nil {
		return nil, fmt.Errorf("create profiles directory: %w", err)
	}

	return &ProfileManager{
		profilesDir: profilesDir,
		log:         log,
	}, nil
}

// SaveProfile saves a profile
func (pm *ProfileManager) SaveProfile(profile *ExportProfile) error {
	// Set modification time
	if profile.Created.IsZero() {
		profile.Created = time.Now()
	}
	profile.Modified = time.Now()

	// Validate profile
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if !validProfileName(profile.Name) {
		return fmt.Errorf("invalid profile name %q", profile.Name)
	}

	// Save to file
	profilePath := filepath.Join(pm.profilesDir, profile.Name+".json")
	// #nosec G304 -- profilePath is built from a fixed profilesDir plus a name validated by validProfileName
	file, err := os.Create(profilePath)
	if err != nil {
		return fmt.Errorf("create profile file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(profile); err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}

	pm.log.Info("profile saved", "name", profile.Name, "path", profilePath)
	return nil
}

// LoadProfile loads a profile by name
func (pm *ProfileManager) LoadProfile(name string) (*ExportProfile, error) {
	if !validProfileName(name) {
		return nil, fmt.Errorf("invalid profile name %q", name)
	}
	profilePath := filepath.Join(pm.profilesDir, name+".json")

	// #nosec G304 -- profilePath is built from a fixed profilesDir plus a name validated by validProfileName
	file, err := os.Open(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile '%s' not found", name)
		}
		return nil, fmt.Errorf("open profile file: %w", err)
	}
	defer file.Close()

	var profile ExportProfile
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}

	pm.log.Debug("profile loaded", "name", name)
	return &profile, nil
}

// ListProfiles lists all available profiles
func (pm *ProfileManager) ListProfiles() ([]*ExportProfile, error) {
	entries, err := os.ReadDir(pm.profilesDir)
	if err != nil {
		return nil, fmt.Errorf("read profiles directory: %w", err)
	}

	var profiles []*ExportProfile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		name := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		profile, err := pm.LoadProfile(name)
		if err != nil {
			pm.log.Warn("failed to load profile", "name", name, "error", err)
			continue
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// DeleteProfile deletes a profile
func (pm *ProfileManager) DeleteProfile(name string) error {
	profilePath := filepath.Join(pm.profilesDir, name+".json")

	if err := os.Remove(profilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile '%s' not found", name)
		}
		return fmt.Errorf("delete profile: %w", err)
	}

	pm.log.Info("profile deleted", "name", name)
	return nil
}

// ProfileExists checks if a profile exists
func (pm *ProfileManager) ProfileExists(name string) bool {
	profilePath := filepath.Join(pm.profilesDir, name+".json")
	_, err := os.Stat(profilePath)
	return err == nil
}

// CreateDefaultProfiles creates default built-in profiles
func (pm *ProfileManager) CreateDefaultProfiles() error {
	// Quick Export profile
	quickExport := &ExportProfile{
		Name:        "quick-export",
		Description: "Fast export without compression",
		Format:      "ovf",
		Compress:    false,
		Verify:      false,
		PowerOff:    false,
		Parallel:    4,
	}
	if err := pm.SaveProfile(quickExport); err != nil {
		return err
	}

	// Production Backup profile
	prodBackup := &ExportProfile{
		Name:        "production-backup",
		Description: "Production backup with compression and verification",
		Format:      "ova",
		Compress:    true,
		Verify:      true,
		PowerOff:    true,
		Parallel:    6,
		CleanupOVF:  true,
		Encrypt:     false, // User should enable and configure
	}
	if err := pm.SaveProfile(prodBackup); err != nil {
		return err
	}

	// Encrypted Backup profile
	encryptedBackup := &ExportProfile{
		Name:          "encrypted-backup",
		Description:   "Encrypted backup for sensitive data",
		Format:        "ova",
		Compress:      true,
		Verify:        true,
		PowerOff:      true,
		Parallel:      4,
		Encrypt:       true,
		EncryptMethod: "aes256",
	}
	if err := pm.SaveProfile(encryptedBackup); err != nil {
		return err
	}

	// Cloud Backup profile
	cloudBackup := &ExportProfile{
		Name:        "cloud-backup",
		Description: "Backup and upload to cloud storage",
		Format:      "ova",
		Compress:    true,
		Verify:      true,
		PowerOff:    true,
		Parallel:    6,
		KeepLocal:   false,
	}
	if err := pm.SaveProfile(cloudBackup); err != nil {
		return err
	}

	// Development profile
	devExport := &ExportProfile{
		Name:        "development",
		Description: "Quick export for development/testing",
		Format:      "ovf",
		Compress:    false,
		Verify:      false,
		PowerOff:    false,
		Parallel:    8,
	}
	if err := pm.SaveProfile(devExport); err != nil {
		return err
	}

	pm.log.Info("default profiles created")
	return nil
}

// ApplyProfileToFlags applies a profile to command-line flags
// This returns a map of flag values that should be set
func ApplyProfileToFlags(profile *ExportProfile) map[string]interface{} {
	flags := make(map[string]interface{})

	flags["format"] = profile.Format
	flags["compress"] = profile.Compress
	flags["verify"] = profile.Verify
	flags["power-off"] = profile.PowerOff
	flags["parallel"] = profile.Parallel

	if profile.UploadTo != "" {
		flags["upload"] = profile.UploadTo
	}
	flags["keep-local"] = profile.KeepLocal

	if profile.Encrypt {
		flags["encrypt"] = true
		flags["encrypt-method"] = profile.EncryptMethod
		if profile.GPGRecipient != "" {
			flags["gpg-recipient"] = profile.GPGRecipient
		}
	}

	flags["validate-only"] = profile.ValidateOnly

	return flags
}

// CreateProfileFromFlags creates a profile from current flag values
func CreateProfileFromFlags(name, description string) *ExportProfile {
	profile := &ExportProfile{
		Name:          name,
		Description:   description,
		Format:        *format,
		Compress:      *compress,
		Verify:        *verify,
		PowerOff:      *powerOff,
		Parallel:      *parallel,
		UploadTo:      *uploadTo,
		KeepLocal:     *keepLocal,
		Encrypt:       *encrypt,
		EncryptMethod: *encryptMethod,
		GPGRecipient:  *gpgRecipient,
		ValidateOnly:  *validateOnly,
	}

	return profile
}
