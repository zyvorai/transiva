// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/transiva/config"
	"github.com/zyvorai/transiva/logger"
)

func TestConnectionPool_NewPool(t *testing.T) {
	cfg := &config.Config{
		VCenterURL: "https://test.example.com",
		Username:   "test",
		Password:   "test",
	}

	poolCfg := &PoolConfig{
		MaxConnections:      3,
		IdleTimeout:         1 * time.Minute,
		HealthCheckInterval: 10 * time.Second,
	}

	log := logger.New("debug")
	pool := NewConnectionPool(cfg, poolCfg, log)

	if pool == nil {
		t.Fatal("NewConnectionPool returned nil")
	}

	if len(pool.connections) != 0 {
		t.Errorf("Expected 0 initial connections, got %d", len(pool.connections))
	}

	pool.Close()
}

func TestConnectionPool_Stats(t *testing.T) {
	cfg := &config.Config{
		VCenterURL: "https://test.example.com",
		Username:   "test",
		Password:   "test",
	}

	poolCfg := DefaultPoolConfig()
	log := logger.New("debug")
	pool := NewConnectionPool(cfg, poolCfg, log)
	defer pool.Close()

	stats := pool.Stats()

	if stats["total_connections"].(int) != 0 {
		t.Errorf("Expected 0 total connections, got %d", stats["total_connections"])
	}

	if stats["in_use"].(int) != 0 {
		t.Errorf("Expected 0 in use connections, got %d", stats["in_use"])
	}

	if stats["max_connections"].(int) != poolCfg.MaxConnections {
		t.Errorf("Expected max connections %d, got %d", poolCfg.MaxConnections, stats["max_connections"])
	}
}

func TestConnectionPool_Close(t *testing.T) {
	cfg := &config.Config{
		VCenterURL: "https://test.example.com",
		Username:   "test",
		Password:   "test",
	}

	poolCfg := DefaultPoolConfig()
	log := logger.New("debug")
	pool := NewConnectionPool(cfg, poolCfg, log)

	// Close the pool
	err := pool.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Verify pool is closed
	stats := pool.Stats()
	if stats["total_connections"].(int) != 0 {
		t.Errorf("Expected 0 connections after close, got %d", stats["total_connections"])
	}
}

func TestConnectionPool_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		VCenterURL: "https://test.example.com",
		Username:   "test",
		Password:   "test",
	}

	poolCfg := DefaultPoolConfig()
	log := logger.New("debug")
	pool := NewConnectionPool(cfg, poolCfg, log)
	defer pool.Close()

	// Create a context that's immediately cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to get connection with cancelled context
	_, err := pool.Get(ctx)
	if err == nil {
		t.Error("Expected error with cancelled context, got nil")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestDefaultPoolConfig(t *testing.T) {
	cfg := DefaultPoolConfig()

	if cfg.MaxConnections != 5 {
		t.Errorf("Expected default MaxConnections 5, got %d", cfg.MaxConnections)
	}

	if cfg.IdleTimeout != 5*time.Minute {
		t.Errorf("Expected default IdleTimeout 5m, got %v", cfg.IdleTimeout)
	}

	if cfg.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected default HealthCheckInterval 30s, got %v", cfg.HealthCheckInterval)
	}
}
