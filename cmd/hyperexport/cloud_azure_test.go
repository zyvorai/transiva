// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/zyvorai/transiva/logger"
)

func TestAzureStorage_buildBlobName(t *testing.T) {
	log := logger.New("info")

	tests := []struct {
		name       string
		prefix     string
		remotePath string
		expected   string
	}{
		{
			name:       "no prefix",
			prefix:     "",
			remotePath: "file.vmdk",
			expected:   "file.vmdk",
		},
		{
			name:       "with prefix",
			prefix:     "backups",
			remotePath: "file.vmdk",
			expected:   "backups/file.vmdk",
		},
		{
			name:       "with nested prefix",
			prefix:     "backups/2024/01",
			remotePath: "vm-export.vmdk",
			expected:   "backups/2024/01/vm-export.vmdk",
		},
		{
			name:       "remote path with subdirectory",
			prefix:     "exports",
			remotePath: "prod/vm1/disk.vmdk",
			expected:   "exports/prod/vm1/disk.vmdk",
		},
		{
			name:       "empty remote path",
			prefix:     "backups",
			remotePath: "",
			expected:   "backups",
		},
		{
			name:       "both empty",
			prefix:     "",
			remotePath: "",
			expected:   "",
		},
		{
			name:       "prefix with trailing slash",
			prefix:     "backups/",
			remotePath: "file.vmdk",
			expected:   "backups/file.vmdk",
		},
		{
			name:       "remote path with leading slash",
			prefix:     "backups",
			remotePath: "/file.vmdk",
			expected:   "backups/file.vmdk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &AzureStorage{
				prefix: tt.prefix,
				log:    log,
			}

			result := storage.buildBlobName(tt.remotePath)
			if result != tt.expected {
				t.Errorf("buildBlobName() = %q, want %q", result, tt.expected)
			}
		})
	}
}
