// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"time"

	"github.com/zyvorai/transiva/providers"
)

// Metadata describes a provider plugin
type Metadata struct {
	// Basic information
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	License     string `json:"license"`

	// Provider information
	ProviderType providers.ProviderType      `json:"provider_type"`
	Capabilities providers.ExportCapabilities `json:"capabilities"`

	// Plugin requirements
	MinSDKVersion string   `json:"min_sdk_version"`
	Dependencies  []string `json:"dependencies"`

	// Build information
	BuildTime time.Time `json:"build_time"`
	GoVersion string    `json:"go_version"`
}

// Info contains runtime information about a loaded plugin
type Info struct {
	Metadata Metadata  `json:"metadata"`
	Path     string    `json:"path"`
	Status   Status    `json:"status"`
	LoadedAt time.Time `json:"loaded_at"`
	Error    string    `json:"error,omitempty"`
}

// Status represents the plugin status
type Status string

const (
	StatusLoaded   Status = "loaded"
	StatusFailed   Status = "failed"
	StatusDisabled Status = "disabled"
	StatusUnloaded Status = "unloaded"
)

// ValidateMetadata checks if plugin metadata is valid
func (m *Metadata) ValidateMetadata() error {
	if m.Name == "" {
		return ErrInvalidMetadata{Field: "name", Reason: "cannot be empty"}
	}
	if m.Version == "" {
		return ErrInvalidMetadata{Field: "version", Reason: "cannot be empty"}
	}
	if m.ProviderType == "" {
		return ErrInvalidMetadata{Field: "provider_type", Reason: "cannot be empty"}
	}
	return nil
}

// ErrInvalidMetadata indicates invalid plugin metadata
type ErrInvalidMetadata struct {
	Field  string
	Reason string
}

func (e ErrInvalidMetadata) Error() string {
	return "invalid plugin metadata: " + e.Field + " " + e.Reason
}
