// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
)

// PoolConfig holds connection pool configuration
type PoolConfig struct {
	MaxConnections      int
	IdleTimeout         time.Duration
	HealthCheckInterval time.Duration
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConnections:      5,
		IdleTimeout:         5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
	}
}

// pooledConnection wraps a vSphere client with metadata
type pooledConnection struct {
	client    *govmomi.Client
	finder    *find.Finder
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
}

// ConnectionPool manages a pool of vSphere connections
type ConnectionPool struct {
	config     *config.Config
	poolConfig *PoolConfig
	logger     logger.Logger

	// Connection pool
	connections []*pooledConnection
	available   chan *pooledConnection
	mu          sync.Mutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Stats
	totalCreated int
	totalReused  int
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(cfg *config.Config, poolCfg *PoolConfig, log logger.Logger) *ConnectionPool {
	if poolCfg == nil {
		poolCfg = DefaultPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPool{
		config:      cfg,
		poolConfig:  poolCfg,
		logger:      log,
		connections: make([]*pooledConnection, 0, poolCfg.MaxConnections),
		available:   make(chan *pooledConnection, poolCfg.MaxConnections),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start background health checker
	pool.wg.Add(1)
	go pool.healthCheckLoop()

	pool.logger.Info("connection pool initialized",
		"maxConnections", poolCfg.MaxConnections,
		"idleTimeout", poolCfg.IdleTimeout,
		"healthCheckInterval", poolCfg.HealthCheckInterval)

	return pool
}

// Get acquires a connection from the pool or creates a new one
func (p *ConnectionPool) Get(ctx context.Context) (*VSphereClient, error) {
	// Try to get an available connection
	select {
	case conn := <-p.available:
		// Check if connection is still valid
		if time.Since(conn.lastUsed) > p.poolConfig.IdleTimeout {
			p.logger.Debug("connection expired, creating new one")
			p.closeConnection(conn)
			return p.createNewConnection(ctx)
		}

		// Mark as in use
		p.mu.Lock()
		conn.inUse = true
		conn.lastUsed = time.Now()
		p.totalReused++
		p.mu.Unlock()

		p.logger.Debug("reusing pooled connection",
			"age", time.Since(conn.createdAt),
			"totalReused", p.totalReused)

		return &VSphereClient{
			client: conn.client,
			finder: conn.finder,
			config: p.config,
			logger: p.logger,
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	default:
		// No available connections, check if we can create a new one
		p.mu.Lock()
		canCreate := len(p.connections) < p.poolConfig.MaxConnections
		p.mu.Unlock()

		if canCreate {
			return p.createNewConnection(ctx)
		}

		// Wait for an available connection
		p.logger.Debug("waiting for available connection")
		select {
		case conn := <-p.available:
			p.mu.Lock()
			conn.inUse = true
			conn.lastUsed = time.Now()
			p.totalReused++
			p.mu.Unlock()

			return &VSphereClient{
				client: conn.client,
				finder: conn.finder,
				config: p.config,
				logger: p.logger,
			}, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(client *VSphereClient) {
	if client == nil || client.client == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Find the connection in our pool
	for _, conn := range p.connections {
		if conn.client == client.client {
			conn.inUse = false
			conn.lastUsed = time.Now()

			// Return to available channel (non-blocking)
			select {
			case p.available <- conn:
				p.logger.Debug("connection returned to pool")
			default:
				// Channel full, close the connection
				p.logger.Warn("pool full, closing connection")
				p.closeConnection(conn)
			}
			return
		}
	}

	// Connection not from pool, close it
	p.logger.Debug("closing non-pooled connection")
	if err := client.Close(); err != nil {
		p.logger.Warn("failed to close non-pooled connection", "error", err)
	}
}

// createNewConnection creates a new vSphere connection
func (p *ConnectionPool) createNewConnection(ctx context.Context) (*VSphereClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check we haven't exceeded max connections
	if len(p.connections) >= p.poolConfig.MaxConnections {
		return nil, fmt.Errorf("connection pool exhausted (max: %d)", p.poolConfig.MaxConnections)
	}

	p.logger.Debug("creating new vSphere connection",
		"current", len(p.connections),
		"max", p.poolConfig.MaxConnections)

	// Create new client
	client, err := NewVSphereClient(ctx, p.config, p.logger)
	if err != nil {
		return nil, fmt.Errorf("create vSphere client: %w", err)
	}

	// Wrap in pooled connection
	conn := &pooledConnection{
		client:    client.client,
		finder:    client.finder,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		inUse:     true,
	}

	p.connections = append(p.connections, conn)
	p.totalCreated++

	p.logger.Info("new connection created",
		"total", len(p.connections),
		"totalCreated", p.totalCreated)

	return client, nil
}

// healthCheckLoop periodically checks and cleans up idle connections
func (p *ConnectionPool) healthCheckLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.poolConfig.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return

		case <-ticker.C:
			p.performHealthCheck()
		}
	}
}

// performHealthCheck checks all connections and removes expired ones
func (p *ConnectionPool) performHealthCheck() {
	p.mu.Lock()
	defer p.mu.Unlock()

	var active int
	now := time.Now()

	for i := len(p.connections) - 1; i >= 0; i-- {
		conn := p.connections[i]

		// Skip connections in use
		if conn.inUse {
			active++
			continue
		}

		// Check if connection is idle for too long
		if now.Sub(conn.lastUsed) > p.poolConfig.IdleTimeout {
			p.logger.Debug("removing idle connection",
				"age", now.Sub(conn.createdAt),
				"idle", now.Sub(conn.lastUsed))

			p.closeConnection(conn)

			// Remove from slice
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
		} else {
			active++
		}
	}

	if active < len(p.connections) {
		p.logger.Debug("health check completed",
			"active", active,
			"removed", len(p.connections)-active)
	}
}

// closeConnection closes a pooled connection
func (p *ConnectionPool) closeConnection(conn *pooledConnection) {
	if conn.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := conn.client.Logout(ctx); err != nil {
			p.logger.Warn("failed to logout connection", "error", err)
		}
	}
}

// Close shuts down the connection pool
func (p *ConnectionPool) Close() error {
	p.logger.Info("closing connection pool")

	// Stop health check loop
	p.cancel()
	p.wg.Wait()

	// Close all connections
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		p.closeConnection(conn)
	}

	p.connections = nil
	close(p.available)

	p.logger.Info("connection pool closed",
		"totalCreated", p.totalCreated,
		"totalReused", p.totalReused)

	return nil
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	var inUse, idle int
	for _, conn := range p.connections {
		if conn.inUse {
			inUse++
		} else {
			idle++
		}
	}

	return map[string]interface{}{
		"total_connections": len(p.connections),
		"in_use":            inUse,
		"idle":              idle,
		"max_connections":   p.poolConfig.MaxConnections,
		"total_created":     p.totalCreated,
		"total_reused":      p.totalReused,
		"reuse_ratio":       float64(p.totalReused) / float64(max(p.totalCreated, 1)),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
