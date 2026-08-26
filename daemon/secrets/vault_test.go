// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"testing"
)

func TestVaultManager_secretPath(t *testing.T) {
	tests := []struct {
		name     string
		mount    string
		secret   string
		expected string
	}{
		{
			name:     "secret without mount prefix",
			mount:    "secret",
			secret:   "mysecret",
			expected: "mysecret",
		},
		{
			name:     "secret with mount prefix",
			mount:    "secret",
			secret:   "secret/mysecret",
			expected: "mysecret",
		},
		{
			name:     "secret with nested path",
			mount:    "secret",
			secret:   "secret/app/database/password",
			expected: "app/database/password",
		},
		{
			name:     "secret without mount prefix but nested",
			mount:    "secret",
			secret:   "app/database/password",
			expected: "app/database/password",
		},
		{
			name:     "empty secret name",
			mount:    "secret",
			secret:   "",
			expected: "",
		},
		{
			name:     "only mount prefix",
			mount:    "secret",
			secret:   "secret/",
			expected: "",
		},
		{
			name:     "different mount",
			mount:    "kv",
			secret:   "kv/credentials",
			expected: "credentials",
		},
		{
			name:     "secret with partial mount match",
			mount:    "secret",
			secret:   "secrets/mysecret",
			expected: "secrets/mysecret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &VaultManager{
				mount: tt.mount,
			}

			result := v.secretPath(tt.secret)
			if result != tt.expected {
				t.Errorf("secretPath(%q) = %q, want %q", tt.secret, result, tt.expected)
			}
		})
	}
}
