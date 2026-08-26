// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRealtimeMetrics(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	// Get a VM to test metrics against
	vms, err := client.ListVMs(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vms)

	vmName := vms[0].Name

	tests := []struct {
		name       string
		entityName string
		entityType string
		wantErr    bool
	}{
		{
			name:       "get VM metrics",
			entityName: vmName,
			entityType: "vm",
			wantErr:    false,
		},
		{
			name:       "invalid entity type",
			entityName: vmName,
			entityType: "invalid",
			wantErr:    true,
		},
		{
			name:       "non-existent entity",
			entityName: "non-existent-vm",
			entityType: "vm",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			metrics, err := client.GetRealtimeMetrics(ctx, tt.entityName, tt.entityType)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, metrics)

			assert.Equal(t, tt.entityName, metrics.EntityName)
			assert.Equal(t, tt.entityType, metrics.EntityType)
			assert.NotZero(t, metrics.Timestamp)

			// Metrics should be >= 0
			assert.GreaterOrEqual(t, metrics.CPUPercent, 0.0)
			assert.GreaterOrEqual(t, metrics.MemoryPercent, 0.0)
			assert.GreaterOrEqual(t, metrics.DiskReadMBps, 0.0)
			assert.GreaterOrEqual(t, metrics.DiskWriteMBps, 0.0)
			assert.GreaterOrEqual(t, metrics.NetRxMBps, 0.0)
			assert.GreaterOrEqual(t, metrics.NetTxMBps, 0.0)
		})
	}
}

func TestGetHistoricalMetrics(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	// Get a VM to test metrics against
	vms, err := client.ListVMs(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vms)

	vmName := vms[0].Name

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	tests := []struct {
		name       string
		entityName string
		entityType string
		startTime  time.Time
		endTime    time.Time
		interval   string
		wantErr    bool
	}{
		{
			name:       "get VM historical metrics",
			entityName: vmName,
			entityType: "vm",
			startTime:  oneHourAgo,
			endTime:    now,
			interval:   "5min",
			wantErr:    false,
		},
		{
			name:       "invalid interval",
			entityName: vmName,
			entityType: "vm",
			startTime:  oneHourAgo,
			endTime:    now,
			interval:   "invalid",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			metricsHistory, err := client.GetMetricsHistory(
				ctx,
				tt.entityName,
				tt.entityType,
				tt.startTime,
				tt.endTime,
				tt.interval,
			)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, metricsHistory)
			assert.Equal(t, tt.entityName, metricsHistory.EntityName)

			// Historical metrics may be empty if no data exists
			for _, metrics := range metricsHistory.Samples {
				assert.Equal(t, tt.entityName, metrics.EntityName)
				assert.Equal(t, tt.entityType, metrics.EntityType)
				assert.NotZero(t, metrics.Timestamp)
			}
		})
	}
}

func TestStreamMetrics(t *testing.T) {
	t.Skip("Skipping streaming test - requires running vCenter")

	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	// Get a VM to test metrics against
	vms, err := client.ListVMs(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vms)

	vmName := vms[0].Name

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	metricsChan, err := client.StreamMetrics(ctx, vmName, "vm", 20*time.Second)
	require.NoError(t, err)
	require.NotNil(t, metricsChan)

	// Collect metrics for a short period
	timeout := time.After(45 * time.Second)
	metricsCount := 0

	for {
		select {
		case metrics, ok := <-metricsChan:
			if !ok {
				// Channel closed
				assert.Greater(t, metricsCount, 0, "should have received at least one metric")
				return
			}

			assert.Equal(t, vmName, metrics.EntityName)
			assert.Equal(t, "vm", metrics.EntityType)
			assert.NotZero(t, metrics.Timestamp)
			metricsCount++

		case <-timeout:
			assert.Greater(t, metricsCount, 0, "should have received at least one metric")
			return
		}
	}
}

func TestPerformanceMetricsValidation(t *testing.T) {
	metrics := PerformanceMetrics{
		EntityName:    "test-vm",
		EntityType:    "vm",
		Timestamp:     time.Now(),
		CPUPercent:    45.5,
		CPUUsageMhz:   2000,
		MemoryPercent: 60.2,
		MemoryUsageMB: 4096,
		DiskReadMBps:  10.5,
		DiskWriteMBps: 5.2,
		NetRxMBps:     2.1,
		NetTxMBps:     1.8,
	}

	assert.Equal(t, "test-vm", metrics.EntityName)
	assert.Equal(t, "vm", metrics.EntityType)
	assert.NotZero(t, metrics.Timestamp)
	assert.InDelta(t, 45.5, metrics.CPUPercent, 0.1)
	assert.Equal(t, int64(2000), metrics.CPUUsageMhz)
	assert.InDelta(t, 60.2, metrics.MemoryPercent, 0.1)
	assert.Equal(t, int64(4096), metrics.MemoryUsageMB)
}

func TestMetricsEntityTypeValidation(t *testing.T) {
	validTypes := []string{"vm", "host", "cluster"}

	for _, entityType := range validTypes {
		metrics := PerformanceMetrics{
			EntityName: "test-entity",
			EntityType: entityType,
			Timestamp:  time.Now(),
		}

		assert.Contains(t, validTypes, metrics.EntityType)
	}
}

func TestMetricsTimestampValidation(t *testing.T) {
	now := time.Now()
	metrics := PerformanceMetrics{
		EntityName: "test-vm",
		EntityType: "vm",
		Timestamp:  now,
	}

	assert.WithinDuration(t, now, metrics.Timestamp, 1*time.Second)
}

func TestMetricsContextCancellation(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())

	// Get a VM
	vms, err := client.ListVMs(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vms)

	vmName := vms[0].Name

	// Start streaming
	metricsChan, err := client.StreamMetrics(ctx, vmName, "vm", 20*time.Second)
	require.NoError(t, err)

	// Cancel context immediately
	cancel()

	// Channel should close
	timeout := time.After(5 * time.Second)
	select {
	case _, ok := <-metricsChan:
		assert.False(t, ok, "channel should be closed")
	case <-timeout:
		t.Fatal("timeout waiting for channel to close")
	}
}

func TestMetricsDataRangeValidation(t *testing.T) {
	tests := []struct {
		name    string
		metrics PerformanceMetrics
		valid   bool
	}{
		{
			name: "valid metrics",
			metrics: PerformanceMetrics{
				CPUPercent:    50.0,
				MemoryPercent: 70.0,
				DiskReadMBps:  100.0,
				NetRxMBps:     50.0,
			},
			valid: true,
		},
		{
			name: "cpu percent over 100",
			metrics: PerformanceMetrics{
				CPUPercent: 150.0,
			},
			valid: false,
		},
		{
			name: "memory percent over 100",
			metrics: PerformanceMetrics{
				MemoryPercent: 120.0,
			},
			valid: false,
		},
		{
			name: "negative disk read",
			metrics: PerformanceMetrics{
				DiskReadMBps: -10.0,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.LessOrEqual(t, tt.metrics.CPUPercent, 100.0)
				assert.LessOrEqual(t, tt.metrics.MemoryPercent, 100.0)
				assert.GreaterOrEqual(t, tt.metrics.DiskReadMBps, 0.0)
				assert.GreaterOrEqual(t, tt.metrics.NetRxMBps, 0.0)
			} else {
				// Check at least one metric is out of valid range
				invalid := tt.metrics.CPUPercent > 100.0 ||
					tt.metrics.MemoryPercent > 100.0 ||
					tt.metrics.DiskReadMBps < 0.0 ||
					tt.metrics.NetRxMBps < 0.0
				assert.True(t, invalid)
			}
		})
	}
}
