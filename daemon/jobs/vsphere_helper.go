// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"time"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/providers/vsphere"
)

// GetVSphereClient returns a vSphere client using environment configuration
func (m *Manager) GetVSphereClient() (*vsphere.VSphereClient, error) {
	// Get config from environment
	cfg := config.FromEnvironment()

	// Set defaults
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}
	if cfg.DownloadWorkers == 0 {
		cfg.DownloadWorkers = 4
	}
	if cfg.RetryAttempts == 0 {
		cfg.RetryAttempts = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 5 * time.Second
	}

	// Create client
	ctx := m.ctx
	client, err := vsphere.NewVSphereClient(ctx, cfg, m.logger)
	if err != nil {
		return nil, err
	}

	return client, nil
}
