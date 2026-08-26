// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ToJSON serializes the manifest to JSON
func ToJSON(m *ArtifactManifest) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	return data, nil
}

// ToYAML serializes the manifest to YAML
func ToYAML(m *ArtifactManifest) ([]byte, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	return data, nil
}

// FromJSON deserializes the manifest from JSON
func FromJSON(data []byte) (*ArtifactManifest, error) {
	var m ArtifactManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal JSON: %w", err)
	}
	return &m, nil
}

// FromYAML deserializes the manifest from YAML
func FromYAML(data []byte) (*ArtifactManifest, error) {
	var m ArtifactManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal YAML: %w", err)
	}
	return &m, nil
}

// WriteToFile writes the manifest to a file (JSON or YAML based on extension)
func WriteToFile(m *ArtifactManifest, filePath string) error {
	var data []byte
	var err error

	// Determine format based on file extension
	if len(filePath) >= 5 && (filePath[len(filePath)-5:] == ".yaml" || filePath[len(filePath)-4:] == ".yml") {
		data, err = ToYAML(m)
	} else {
		data, err = ToJSON(m)
	}

	if err != nil {
		return err
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ReadFromFile reads a manifest from a file (JSON or YAML based on extension)
func ReadFromFile(filePath string) (*ArtifactManifest, error) {
	// #nosec G304 -- filePath is a manifest path supplied by the local CLI
	// operator or pipeline config (see providers/vsphere, providers/nutanix,
	// providers/common/pipeline.go), not derived from an HTTP request.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Determine format based on file extension
	var m *ArtifactManifest
	if len(filePath) >= 5 && (filePath[len(filePath)-5:] == ".yaml" || filePath[len(filePath)-4:] == ".yml") {
		m, err = FromYAML(data)
	} else {
		m, err = FromJSON(data)
	}

	if err != nil {
		return nil, err
	}

	// Validate after loading
	if err := Validate(m); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return m, nil
}
