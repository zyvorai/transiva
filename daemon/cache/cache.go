// SPDX-License-Identifier: Apache-2.0

// Package cache provides a caching layer with Redis and in-memory backends
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrCacheMiss is returned when a key is not found in the cache
	ErrCacheMiss = errors.New("cache miss")

	// ErrCacheUnavailable is returned when the cache backend is unavailable
	ErrCacheUnavailable = errors.New("cache unavailable")
)

// Cache interface defines the caching operations
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) (interface{}, error)

	// Set stores a value in the cache with TTL
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a key from the cache
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all keys from the cache
	Clear(ctx context.Context) error

	// Close closes the cache connection
	Close() error

	// Stats returns cache statistics
	Stats() Stats
}

// Stats represents cache statistics
type Stats struct {
	Hits      int64
	Misses    int64
	Sets      int64
	Deletes   int64
	Evictions int64
	Size      int64
}

// Config holds cache configuration
type Config struct {
	// Backend type: "redis" or "memory"
	Backend string

	// Redis configuration
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Memory cache configuration
	MaxMemorySize int64 // Maximum memory size in bytes
	DefaultTTL    time.Duration

	// Connection settings
	MaxRetries      int
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolSize        int
	MinIdleConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// Fallback settings
	EnableFallback bool // Fallback to memory cache if Redis fails
}

// DefaultConfig returns default cache configuration
func DefaultConfig() *Config {
	return &Config{
		Backend:         "memory",
		RedisAddr:       "localhost:6379",
		RedisDB:         0,
		MaxMemorySize:   100 * 1024 * 1024, // 100 MB
		DefaultTTL:      1 * time.Hour,
		MaxRetries:      3,
		ConnectTimeout:  5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        10,
		MinIdleConns:    5,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		EnableFallback:  true,
	}
}

// NewCache creates a new cache instance
func NewCache(config *Config) (Cache, error) {
	if config == nil {
		config = DefaultConfig()
	}

	switch config.Backend {
	case "redis":
		cache, err := newRedisCache(config)
		if err != nil {
			if config.EnableFallback {
				// Fallback to memory cache
				return newMemoryCache(config), nil
			}
			return nil, err
		}
		return cache, nil
	case "memory":
		return newMemoryCache(config), nil
	default:
		return nil, errors.New("unsupported cache backend")
	}
}

// RedisCache implements Cache using Redis
type RedisCache struct {
	client   *redis.Client
	config   *Config
	stats    Stats
	statsMu  sync.RWMutex
	fallback Cache
}

// newRedisCache creates a new Redis cache
func newRedisCache(config *Config) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            config.RedisAddr,
		Password:        config.RedisPassword,
		DB:              config.RedisDB,
		MaxRetries:      config.MaxRetries,
		DialTimeout:     config.ConnectTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		MaxIdleConns:    config.MaxIdleConns,
		ConnMaxLifetime: config.ConnMaxLifetime,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	cache := &RedisCache{
		client: client,
		config: config,
	}

	// Setup fallback if enabled
	if config.EnableFallback {
		cache.fallback = newMemoryCache(config)
	}

	return cache, nil
}

// Get retrieves a value from Redis
func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		r.statsMu.Lock()
		r.stats.Misses++
		r.statsMu.Unlock()

		if err == redis.Nil {
			return nil, ErrCacheMiss
		}

		// Try fallback
		if r.fallback != nil {
			return r.fallback.Get(ctx, key)
		}

		return nil, err
	}

	r.statsMu.Lock()
	r.stats.Hits++
	r.statsMu.Unlock()

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return value, nil
}

// Set stores a value in Redis
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		// Try fallback
		if r.fallback != nil {
			return r.fallback.Set(ctx, key, value, ttl)
		}
		return err
	}

	r.statsMu.Lock()
	r.stats.Sets++
	r.statsMu.Unlock()

	// Also set in fallback for consistency
	if r.fallback != nil {
		if err := r.fallback.Set(ctx, key, value, ttl); err != nil {
			log.Printf("cache: failed to sync set to fallback cache: %v", err)
		}
	}

	return nil
}

// Delete removes a key from Redis
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		// Try fallback
		if r.fallback != nil {
			return r.fallback.Delete(ctx, key)
		}
		return err
	}

	r.statsMu.Lock()
	r.stats.Deletes++
	r.statsMu.Unlock()

	// Also delete from fallback
	if r.fallback != nil {
		if err := r.fallback.Delete(ctx, key); err != nil {
			log.Printf("cache: failed to sync delete to fallback cache: %v", err)
		}
	}

	return nil
}

// Exists checks if a key exists in Redis
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		// Try fallback
		if r.fallback != nil {
			return r.fallback.Exists(ctx, key)
		}
		return false, err
	}

	return count > 0, nil
}

// Clear removes all keys from Redis
func (r *RedisCache) Clear(ctx context.Context) error {
	if err := r.client.FlushDB(ctx).Err(); err != nil {
		// Try fallback
		if r.fallback != nil {
			return r.fallback.Clear(ctx)
		}
		return err
	}

	// Also clear fallback
	if r.fallback != nil {
		if err := r.fallback.Clear(ctx); err != nil {
			log.Printf("cache: failed to clear fallback cache: %v", err)
		}
	}

	return nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	if r.fallback != nil {
		if err := r.fallback.Close(); err != nil {
			log.Printf("cache: failed to close fallback cache: %v", err)
		}
	}
	return r.client.Close()
}

// Stats returns cache statistics
func (r *RedisCache) Stats() Stats {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()
	return r.stats
}

// MemoryCache implements Cache using in-memory storage
type MemoryCache struct {
	config  *Config
	data    map[string]*cacheEntry
	mu      sync.RWMutex
	stats   Stats
	statsMu sync.RWMutex
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
	size      int64
}

// newMemoryCache creates a new memory cache
func newMemoryCache(config *Config) *MemoryCache {
	cache := &MemoryCache{
		config: config,
		data:   make(map[string]*cacheEntry),
	}

	// Start eviction loop
	go cache.evictionLoop()

	return cache
}

// Get retrieves a value from memory
func (m *MemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	if !exists {
		m.statsMu.Lock()
		m.stats.Misses++
		m.statsMu.Unlock()
		return nil, ErrCacheMiss
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		m.statsMu.Lock()
		m.stats.Misses++
		m.statsMu.Unlock()
		return nil, ErrCacheMiss
	}

	m.statsMu.Lock()
	m.stats.Hits++
	m.statsMu.Unlock()

	return entry.value, nil
}

// Set stores a value in memory
func (m *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Calculate size
	data, _ := json.Marshal(value)
	size := int64(len(data))

	// Check memory limit
	currentSize := m.calculateSize()
	if currentSize+size > m.config.MaxMemorySize {
		// Evict oldest entries
		m.evictOldest(size)
	}

	m.data[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
		size:      size,
	}

	m.statsMu.Lock()
	m.stats.Sets++
	m.stats.Size = m.calculateSize()
	m.statsMu.Unlock()

	return nil
}

// Delete removes a key from memory
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)

	m.statsMu.Lock()
	m.stats.Deletes++
	m.stats.Size = m.calculateSize()
	m.statsMu.Unlock()

	return nil
}

// Exists checks if a key exists in memory
func (m *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	if !exists {
		return false, nil
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		return false, nil
	}

	return true, nil
}

// Clear removes all keys from memory
func (m *MemoryCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]*cacheEntry)

	m.statsMu.Lock()
	m.stats.Size = 0
	m.statsMu.Unlock()

	return nil
}

// Close is a no-op for memory cache
func (m *MemoryCache) Close() error {
	return nil
}

// Stats returns cache statistics
func (m *MemoryCache) Stats() Stats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()
	return m.stats
}

// calculateSize calculates total cache size
func (m *MemoryCache) calculateSize() int64 {
	var total int64
	for _, entry := range m.data {
		total += entry.size
	}
	return total
}

// evictOldest evicts oldest entries to free up space
func (m *MemoryCache) evictOldest(needed int64) {
	var freed int64

	// Find and remove expired entries first
	now := time.Now()
	for key, entry := range m.data {
		if now.After(entry.expiresAt) {
			freed += entry.size
			delete(m.data, key)
			m.statsMu.Lock()
			m.stats.Evictions++
			m.statsMu.Unlock()
		}
	}

	// If still need more space, remove oldest entries
	if freed < needed {
		// Convert to slice for sorting
		type kv struct {
			key   string
			entry *cacheEntry
		}
		var entries []kv
		for k, v := range m.data {
			entries = append(entries, kv{k, v})
		}

		// Sort by expiration time
		for i := 0; i < len(entries)-1; i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[i].entry.expiresAt.After(entries[j].entry.expiresAt) {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}

		// Remove until enough space
		for _, kv := range entries {
			if freed >= needed {
				break
			}
			freed += kv.entry.size
			delete(m.data, kv.key)
			m.statsMu.Lock()
			m.stats.Evictions++
			m.statsMu.Unlock()
		}
	}
}

// evictionLoop periodically removes expired entries
func (m *MemoryCache) evictionLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for key, entry := range m.data {
			if now.After(entry.expiresAt) {
				delete(m.data, key)
				m.statsMu.Lock()
				m.stats.Evictions++
				m.statsMu.Unlock()
			}
		}
		m.mu.Unlock()
	}
}
