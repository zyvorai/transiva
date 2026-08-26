// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
	"github.com/zyvorai/transiva/retry"
)

// mockLogger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) With(keysAndValues ...interface{}) logger.Logger { return m }

// setupTestClient creates a test vSphere client using vcsim simulator
func setupTestClient(t *testing.T) (*VSphereClient, func()) {
	model := simulator.VPX()
	model.Host = 2 // 2 ESXi hosts

	err := model.Create()
	require.NoError(t, err)

	s := model.Service.NewServer()

	ctx := context.Background()

	// Create SOAP client
	soapClient := soap.NewClient(s.URL, true)

	// Create vim25 client
	vimClient, err := vim25.NewClient(ctx, soapClient)
	require.NoError(t, err)

	// Create govmomi client
	govmomiClient := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	// Login to simulator (required for authentication)
	err = govmomiClient.Login(ctx, s.URL.User)
	require.NoError(t, err)

	// Create finder
	finder := find.NewFinder(vimClient, true)

	// Get default datacenter
	dc, err := finder.DefaultDatacenter(ctx)
	require.NoError(t, err)

	finder.SetDatacenter(dc)

	// Create test config
	cfg := &config.Config{
		VCenterURL:    s.URL.String(),
		Insecure:      true,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
	}

	// Create mock logger
	log := &mockLogger{}

	// Create retry config
	retryConfig := &retry.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     8 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	client := &VSphereClient{
		client:  govmomiClient,
		finder:  finder,
		config:  cfg,
		logger:  log,
		retryer: retry.NewRetryer(retryConfig, log),
	}

	cleanup := func() {
		govmomiClient.Logout(ctx)
		s.Close()
		model.Remove()
	}

	return client, cleanup
}

func TestListHosts(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
		minHosts int
	}{
		{
			name:     "list all hosts",
			pattern:  "*",
			wantErr:  false,
			minHosts: 2,
		},
		{
			name:     "list with specific pattern",
			pattern:  "host-*",
			wantErr:  false,
			minHosts: 0,
		},
		{
			name:     "empty pattern defaults to all",
			pattern:  "",
			wantErr:  false,
			minHosts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			hosts, err := client.ListHosts(ctx, tt.pattern)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(hosts), tt.minHosts)

			for _, host := range hosts {
				assert.NotEmpty(t, host.Name)
				assert.NotEmpty(t, host.ConnectionState)
				assert.NotEmpty(t, host.PowerState)
				assert.Greater(t, host.CPUCores, int32(0))
				assert.Greater(t, host.MemoryMB, int64(0))
			}
		})
	}
}

func TestGetHostInfo(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	// List hosts first to get a valid host name
	hosts, err := client.ListHosts(ctx, "*")
	require.NoError(t, err)
	require.NotEmpty(t, hosts)

	hostName := hosts[0].Name

	tests := []struct {
		name     string
		hostName string
		wantErr  bool
	}{
		{
			name:     "get valid host info",
			hostName: hostName,
			wantErr:  false,
		},
		{
			name:     "get non-existent host",
			hostName: "non-existent-host",
			wantErr:  true,
		},
		{
			name:     "empty host name",
			hostName: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			host, err := client.GetHostInfo(ctx, tt.hostName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, host)
			assert.Equal(t, tt.hostName, host.Name)
			assert.NotEmpty(t, host.Datacenter)
			assert.Greater(t, host.CPUCores, int32(0))
			assert.Greater(t, host.MemoryMB, int64(0))
		})
	}
}

func TestListClusters(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	tests := []struct {
		name        string
		pattern     string
		wantErr     bool
		minClusters int
	}{
		{
			name:        "list all clusters",
			pattern:     "*",
			wantErr:     false,
			minClusters: 1,
		},
		{
			name:        "list with pattern",
			pattern:     "DC0_*",
			wantErr:     false,
			minClusters: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			clusters, err := client.ListClusters(ctx, tt.pattern)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(clusters), tt.minClusters)

			for _, cluster := range clusters {
				assert.NotEmpty(t, cluster.Name)
				assert.GreaterOrEqual(t, cluster.NumHosts, 0)
			}
		})
	}
}

func TestGetVCenterInfo(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := client.GetVCenterInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.Build)
	assert.NotEmpty(t, info.OSType)
}

func TestListDatacenters(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	datacenters, err := client.ListDatacenters(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, datacenters)

	for _, dc := range datacenters {
		assert.NotEmpty(t, dc.Name)
		assert.GreaterOrEqual(t, dc.NumHosts, 0)
		assert.GreaterOrEqual(t, dc.NumVMs, 0)
	}
}

func TestContextCancellation(t *testing.T) {
	client, cleanup := setupTestClient(t)
	defer cleanup()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.ListHosts(ctx, "*")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestHostInfoValidation(t *testing.T) {
	hostInfo := HostInfo{
		Name:            "test-host",
		ConnectionState: "connected",
		PowerState:      "poweredOn",
		CPUCores:        16,
		CPUThreads:      32,
		CPUMhz:          2400,
		MemoryMB:        65536,
		NumVMs:          10,
	}

	assert.NotEmpty(t, hostInfo.Name)
	assert.Equal(t, "connected", hostInfo.ConnectionState)
	assert.Equal(t, "poweredOn", hostInfo.PowerState)
	assert.Greater(t, hostInfo.CPUCores, int32(0))
	assert.Greater(t, hostInfo.MemoryMB, int64(0))
}

func TestClusterInfoValidation(t *testing.T) {
	clusterInfo := ClusterInfo{
		Name:        "test-cluster",
		TotalCPU:    48000,
		TotalMemory: 196608,
		NumHosts:    3,
		NumCPUCores: 48,
		DRSEnabled:  true,
		HAEnabled:   true,
	}

	assert.NotEmpty(t, clusterInfo.Name)
	assert.Greater(t, clusterInfo.TotalCPU, int64(0))
	assert.Greater(t, clusterInfo.TotalMemory, int64(0))
	assert.Greater(t, clusterInfo.NumHosts, 0)
	assert.True(t, clusterInfo.DRSEnabled)
	assert.True(t, clusterInfo.HAEnabled)
}
