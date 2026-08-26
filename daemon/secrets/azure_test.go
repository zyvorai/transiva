// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"testing"
)

func TestExtractSecretName(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected string
	}{
		{
			name:     "valid Azure Key Vault ID with version",
			id:       "https://myvault.vault.azure.net/secrets/mysecret/abc123",
			expected: "mysecret",
		},
		{
			name:     "valid Azure Key Vault ID without version",
			id:       "https://myvault.vault.azure.net/secrets/mysecret",
			expected: "mysecret",
		},
		{
			name:     "secret with hyphens",
			id:       "https://myvault.vault.azure.net/secrets/my-secret-name/version123",
			expected: "my-secret-name",
		},
		{
			name:     "secret with underscores",
			id:       "https://myvault.vault.azure.net/secrets/my_secret_name/v1",
			expected: "my_secret_name",
		},
		{
			name:     "empty ID",
			id:       "",
			expected: "",
		},
		{
			name:     "ID without secrets segment",
			id:       "https://myvault.vault.azure.net/keys/mykey",
			expected: "",
		},
		{
			name:     "ID with only secrets segment",
			id:       "secrets",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSecretName(tt.id)
			if result != tt.expected {
				t.Errorf("extractSecretName(%q) = %q, want %q", tt.id, result, tt.expected)
			}
		})
	}
}

func TestExtractVersionFromID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected string
	}{
		{
			name:     "valid ID with version",
			id:       "https://myvault.vault.azure.net/secrets/mysecret/abc123",
			expected: "abc123",
		},
		{
			name:     "valid ID with numeric version",
			id:       "https://myvault.vault.azure.net/secrets/mysecret/12345",
			expected: "12345",
		},
		{
			name:     "ID without version",
			id:       "https://myvault.vault.azure.net/secrets/mysecret",
			expected: "mysecret",
		},
		{
			name:     "empty ID",
			id:       "",
			expected: "",
		},
		{
			name:     "single segment",
			id:       "version123",
			expected: "version123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromID(tt.id)
			if result != tt.expected {
				t.Errorf("extractVersionFromID(%q) = %q, want %q", tt.id, result, tt.expected)
			}
		})
	}
}

func TestSplitString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		separator string
		expected  []string
	}{
		{
			name:      "simple split by slash",
			input:     "a/b/c",
			separator: "/",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "split URL by slash",
			input:     "https://example.com/path/to/resource",
			separator: "/",
			expected:  []string{"https:", "example.com", "path", "to", "resource"},
		},
		{
			name:      "split by multi-character separator",
			input:     "a::b::c",
			separator: "::",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "no separator in string",
			input:     "noseparator",
			separator: "/",
			expected:  []string{"noseparator"},
		},
		{
			name:      "empty string",
			input:     "",
			separator: "/",
			expected:  []string{},
		},
		{
			name:      "only separator",
			input:     "/",
			separator: "/",
			expected:  []string{},
		},
		{
			name:      "multiple consecutive separators",
			input:     "a//b///c",
			separator: "/",
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "separator at start",
			input:     "/start",
			separator: "/",
			expected:  []string{"start"},
		},
		{
			name:      "separator at end",
			input:     "end/",
			separator: "/",
			expected:  []string{"end"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitString(tt.input, tt.separator)
			if len(result) != len(tt.expected) {
				t.Errorf("splitString(%q, %q) returned %d elements, want %d",
					tt.input, tt.separator, len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Want: %v", tt.expected)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitString(%q, %q)[%d] = %q, want %q",
						tt.input, tt.separator, i, result[i], tt.expected[i])
				}
			}
		})
	}
}
