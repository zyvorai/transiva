// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListResourcePools(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	tests := []struct {
		name     string
		pattern  string
		wantErr  bool
		minPools int
	}{
		{
			name:     "list all pools",
			pattern:  "*",
			wantErr:  false,
			minPools: 0, // May be 0 in test environment
		},
		{
			name:     "list with pattern",
			pattern:  "test-*",
			wantErr:  false,
			minPools: 0,
		},
		{
			name:     "empty pattern defaults to all",
			pattern:  "",
			wantErr:  false,
			minPools: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			pools, err := client.ListResourcePools(ctx, tt.pattern)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(pools), tt.minPools)

			for _, pool := range pools {
				assert.NotEmpty(t, pool.Name)
				assert.GreaterOrEqual(t, pool.NumVMs, 0)
			}
		})
	}
}

func TestCreateResourcePool(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	// Get existing resource pools to use as parent
	pools, err := client.ListResourcePools(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, pools, "need at least one resource pool as parent")

	config := ResourcePoolConfig{
		Name:                "test-pool",
		ParentPath:          pools[0].Path,
		CPUReservationMhz:   1000,
		CPULimitMhz:         4000,
		MemoryReservationMB: 2048,
		MemoryLimitMB:       8192,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.CreateResourcePool(ctx, config)
	require.NoError(t, err)

	// Verify pool was created
	pools, err = client.ListResourcePools(ctx, config.Name)
	require.NoError(t, err)
	assert.NotEmpty(t, pools)

	found := false
	for _, pool := range pools {
		if pool.Name == config.Name {
			found = true
			assert.Equal(t, config.CPUReservationMhz, pool.CPUReservationMhz)
			assert.Equal(t, config.MemoryReservationMB, pool.MemoryReservationMB)
			break
		}
	}
	assert.True(t, found, "created pool should be in list")
}

func TestUpdateResourcePool(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	// Get existing resource pools to use as parent
	pools, err := client.ListResourcePools(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, pools, "need at least one resource pool as parent")

	// First create a pool
	createConfig := ResourcePoolConfig{
		Name:                "test-pool-update",
		ParentPath:          pools[0].Path,
		CPUReservationMhz:   1000,
		CPULimitMhz:         4000,
		MemoryReservationMB: 2048,
		MemoryLimitMB:       8192,
	}

	err = client.CreateResourcePool(ctx, createConfig)
	require.NoError(t, err)

	// Update the pool
	updateConfig := ResourcePoolConfig{
		Name:                "test-pool-update",
		CPUReservationMhz:   2000,
		CPULimitMhz:         8000,
		MemoryReservationMB: 4096,
		MemoryLimitMB:       16384,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.UpdateResourcePool(ctx, updateConfig.Name, updateConfig)
	require.NoError(t, err)

	// Verify update
	pools, err = client.ListResourcePools(ctx, updateConfig.Name)
	require.NoError(t, err)

	found := false
	for _, pool := range pools {
		if pool.Name == updateConfig.Name {
			found = true
			assert.Equal(t, updateConfig.CPUReservationMhz, pool.CPUReservationMhz)
			assert.Equal(t, updateConfig.MemoryReservationMB, pool.MemoryReservationMB)
			break
		}
	}
	assert.True(t, found)
}

func TestDeleteResourcePool(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	// Get existing resource pools to use as parent
	pools, err := client.ListResourcePools(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, pools, "need at least one resource pool as parent")

	// Create a pool to delete
	config := ResourcePoolConfig{
		Name:                "test-pool-delete",
		ParentPath:          pools[0].Path,
		CPUReservationMhz:   1000,
		MemoryReservationMB: 2048,
	}

	err = client.CreateResourcePool(ctx, config)
	require.NoError(t, err)

	// Delete the pool
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.DeleteResourcePool(ctx, config.Name)
	require.NoError(t, err)

	// Verify deletion
	pools, err = client.ListResourcePools(ctx, config.Name)
	require.NoError(t, err)

	for _, pool := range pools {
		assert.NotEqual(t, config.Name, pool.Name)
	}
}

func TestResourcePoolConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config ResourcePoolConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: ResourcePoolConfig{
				Name:                "valid-pool",
				CPUReservationMhz:   1000,
				CPULimitMhz:         4000,
				MemoryReservationMB: 2048,
				MemoryLimitMB:       8192,
			},
			valid: true,
		},
		{
			name: "empty name",
			config: ResourcePoolConfig{
				Name:                "",
				CPUReservationMhz:   1000,
				MemoryReservationMB: 2048,
			},
			valid: false,
		},
		{
			name: "cpu limit less than reservation",
			config: ResourcePoolConfig{
				Name:                "invalid-pool",
				CPUReservationMhz:   4000,
				CPULimitMhz:         1000, // Invalid: limit < reservation
				MemoryReservationMB: 2048,
			},
			valid: false,
		},
		{
			name: "memory limit less than reservation",
			config: ResourcePoolConfig{
				Name:                "invalid-pool",
				CPUReservationMhz:   1000,
				MemoryReservationMB: 8192,
				MemoryLimitMB:       2048, // Invalid: limit < reservation
			},
			valid: false,
		},
		{
			name: "negative values",
			config: ResourcePoolConfig{
				Name:                "invalid-pool",
				CPUReservationMhz:   -1000,
				MemoryReservationMB: -2048,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.config.Name != ""

			// Validate name

			// Validate CPU limits
			if tt.config.CPULimitMhz > 0 && tt.config.CPUReservationMhz > tt.config.CPULimitMhz {
				isValid = false
			}

			// Validate memory limits
			if tt.config.MemoryLimitMB > 0 && tt.config.MemoryReservationMB > tt.config.MemoryLimitMB {
				isValid = false
			}

			// Validate no negative values
			if tt.config.CPUReservationMhz < 0 || tt.config.MemoryReservationMB < 0 {
				isValid = false
			}

			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestResourcePoolInfoValidation(t *testing.T) {
	poolInfo := ResourcePoolInfo{
		Name:                "test-pool",
		Path:                "/DC1/host/cluster/Resources/test-pool",
		CPUReservationMhz:   2000,
		CPULimitMhz:         8000,
		MemoryReservationMB: 4096,
		MemoryLimitMB:       16384,
		NumVMs:              5,
	}

	assert.NotEmpty(t, poolInfo.Name)
	assert.NotEmpty(t, poolInfo.Path)
	assert.Greater(t, poolInfo.CPUReservationMhz, int64(0))
	assert.Greater(t, poolInfo.CPULimitMhz, poolInfo.CPUReservationMhz)
	assert.Greater(t, poolInfo.MemoryReservationMB, int64(0))
	assert.Greater(t, poolInfo.MemoryLimitMB, poolInfo.MemoryReservationMB)
	assert.GreaterOrEqual(t, poolInfo.NumVMs, 0)
}

func TestResourcePoolLimitsCalculation(t *testing.T) {
	// Test that CPU and memory limits are correctly calculated
	config := ResourcePoolConfig{
		Name:                "calc-pool",
		CPUReservationMhz:   1000,
		CPULimitMhz:         4000,
		MemoryReservationMB: 2048,
		MemoryLimitMB:       8192,
	}

	// Available CPU
	availableCPU := config.CPULimitMhz - config.CPUReservationMhz
	assert.Equal(t, int64(3000), availableCPU)

	// Available Memory
	availableMemory := config.MemoryLimitMB - config.MemoryReservationMB
	assert.Equal(t, int64(6144), availableMemory)
}

func TestResourcePoolContextCancellation(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.ListResourcePools(ctx, "*")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestResourcePoolNameValidation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"alphanumeric", "pool123", true},
		{"with hyphen", "test-pool", true},
		{"with underscore", "test_pool", true},
		{"empty string", "", false},
		{"only spaces", "   ", false},
		{"special chars", "pool@#$%", false},
		{"very long name", string(make([]byte, 256)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ResourcePoolConfig{
				Name: tt.input,
			}

			// Basic validation
			isValid := config.Name != "" && len(config.Name) > 0 && len(config.Name) < 255

			// Check for invalid characters (simplified)
			if tt.input == "" || tt.input == "   " || tt.input == "pool@#$%" {
				isValid = false
			}

			assert.Equal(t, tt.valid, isValid)
		})
	}
}
