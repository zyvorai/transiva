# Caching Layer

A flexible caching layer with Redis and in-memory backends, automatic failover, and comprehensive metrics.

## Features

- **Multiple Backends**: Redis and in-memory storage
- **Automatic Fallback**: Falls back to memory cache if Redis is unavailable
- **TTL Support**: Per-key expiration with automatic cleanup
- **Memory Management**: Automatic eviction when memory limits are reached
- **Thread-Safe**: Concurrent read/write operations
- **Statistics**: Detailed cache metrics (hits, misses, evictions)
- **Type-Safe**: Support for any serializable Go type

## Quick Start

### Basic Usage

```go
import "github.com/zyvorai/transiva/daemon/cache"

// Create memory cache
config := cache.DefaultConfig()
c, err := cache.NewCache(config)
if err != nil {
    log.Fatal(err)
}
defer c.Close()

ctx := context.Background()

// Set value
c.Set(ctx, "user:123", userData, 1*time.Hour)

// Get value
value, err := c.Get(ctx, "user:123")
if err == cache.ErrCacheMiss {
    // Cache miss, fetch from database
}
```

### Redis Cache

```go
config := &cache.Config{
    Backend:        "redis",
    RedisAddr:      "localhost:6379",
    RedisPassword:  "",
    RedisDB:        0,
    EnableFallback: true, // Fall back to memory on Redis failure
}

c, err := cache.NewCache(config)
if err != nil {
    log.Fatal(err)
}
defer c.Close()
```

## Configuration

```go
type Config struct {
    // Backend type: "redis" or "memory"
    Backend string

    // Redis configuration
    RedisAddr     string
    RedisPassword string
    RedisDB       int

    // Memory cache configuration
    MaxMemorySize int64         // Maximum memory in bytes
    DefaultTTL    time.Duration // Default TTL

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
    EnableFallback bool // Fallback to memory if Redis fails
}
```

### Example Configurations

**Development** (memory only):
```go
config := &cache.Config{
    Backend:       "memory",
    MaxMemorySize: 50 * 1024 * 1024, // 50 MB
    DefaultTTL:    30 * time.Minute,
}
```

**Production** (Redis with fallback):
```go
config := &cache.Config{
    Backend:         "redis",
    RedisAddr:       "redis.example.com:6379",
    RedisPassword:   os.Getenv("REDIS_PASSWORD"),
    RedisDB:         0,
    MaxRetries:      3,
    ConnectTimeout:  5 * time.Second,
    ReadTimeout:     3 * time.Second,
    WriteTimeout:    3 * time.Second,
    PoolSize:        20,
    MinIdleConns:    10,
    MaxIdleConns:    20,
    ConnMaxLifetime: 5 * time.Minute,
    EnableFallback:  true,
    MaxMemorySize:   100 * 1024 * 1024, // For fallback
}
```

**High-Performance** (tuned Redis):
```go
config := &cache.Config{
    Backend:         "redis",
    RedisAddr:       "redis-cluster:6379",
    PoolSize:        50,
    MinIdleConns:    20,
    MaxIdleConns:    50,
    ReadTimeout:     1 * time.Second,
    WriteTimeout:    1 * time.Second,
    EnableFallback:  false, // No fallback for max performance
}
```

## Cache Operations

### Set

Store a value with TTL:

```go
// String
c.Set(ctx, "username", "alice", 1*time.Hour)

// Number
c.Set(ctx, "count", 42, 5*time.Minute)

// Struct
type User struct {
    ID   int
    Name string
}
user := User{ID: 123, Name: "Alice"}
c.Set(ctx, "user:123", user, 24*time.Hour)

// Map
data := map[string]interface{}{
    "name":  "Alice",
    "email": "alice@example.com",
}
c.Set(ctx, "profile:123", data, 1*time.Hour)

// Slice
tags := []string{"golang", "redis", "cache"}
c.Set(ctx, "tags:post:456", tags, 30*time.Minute)
```

### Get

Retrieve a value:

```go
value, err := c.Get(ctx, "user:123")
if err == cache.ErrCacheMiss {
    // Not in cache, fetch from database
    user, err := db.GetUser(123)
    if err != nil {
        return err
    }

    // Store in cache
    c.Set(ctx, "user:123", user, 1*time.Hour)
    value = user
} else if err != nil {
    return err
}

// Type assertion
user := value.(User)
```

### Delete

Remove a key:

```go
err := c.Delete(ctx, "user:123")
if err != nil {
    log.Printf("Failed to delete: %v", err)
}
```

### Exists

Check if key exists:

```go
exists, err := c.Exists(ctx, "user:123")
if err != nil {
    return err
}

if exists {
    // Key is in cache
}
```

### Clear

Remove all keys:

```go
err := c.Clear(ctx)
if err != nil {
    log.Printf("Failed to clear cache: %v", err)
}
```

## Statistics

Get cache statistics:

```go
stats := c.Stats()

fmt.Printf("Hits:      %d\n", stats.Hits)
fmt.Printf("Misses:    %d\n", stats.Misses)
fmt.Printf("Sets:      %d\n", stats.Sets)
fmt.Printf("Deletes:   %d\n", stats.Deletes)
fmt.Printf("Evictions: %d\n", stats.Evictions)
fmt.Printf("Size:      %d bytes\n", stats.Size)

// Calculate hit rate
hitRate := float64(stats.Hits) / float64(stats.Hits+stats.Misses) * 100
fmt.Printf("Hit rate:  %.2f%%\n", hitRate)
```

## Advanced Patterns

### Cache-Aside Pattern

```go
func GetUser(c cache.Cache, db *sql.DB, userID int) (*User, error) {
    ctx := context.Background()
    key := fmt.Sprintf("user:%d", userID)

    // Try cache first
    value, err := c.Get(ctx, key)
    if err == nil {
        return value.(*User), nil
    }

    // Cache miss, fetch from database
    user, err := fetchUserFromDB(db, userID)
    if err != nil {
        return nil, err
    }

    // Store in cache
    c.Set(ctx, key, user, 1*time.Hour)

    return user, nil
}
```

### Write-Through Pattern

```go
func UpdateUser(c cache.Cache, db *sql.DB, user *User) error {
    ctx := context.Background()

    // Update database
    if err := updateUserInDB(db, user); err != nil {
        return err
    }

    // Update cache
    key := fmt.Sprintf("user:%d", user.ID)
    if err := c.Set(ctx, key, user, 1*time.Hour); err != nil {
        log.Printf("Failed to update cache: %v", err)
    }

    return nil
}
```

### Lazy Loading with Mutex

```go
var mu sync.Mutex

func GetConfig(c cache.Cache) (*Config, error) {
    ctx := context.Background()

    value, err := c.Get(ctx, "config")
    if err == nil {
        return value.(*Config), nil
    }

    // Prevent thundering herd
    mu.Lock()
    defer mu.Unlock()

    // Double-check after acquiring lock
    value, err = c.Get(ctx, "config")
    if err == nil {
        return value.(*Config), nil
    }

    // Load from source
    config, err := loadConfig()
    if err != nil {
        return nil, err
    }

    // Cache it
    c.Set(ctx, "config", config, 5*time.Minute)

    return config, nil
}
```

### Batch Operations

```go
func GetUsers(c cache.Cache, db *sql.DB, userIDs []int) ([]*User, error) {
    ctx := context.Background()
    users := make([]*User, 0, len(userIDs))
    missedIDs := make([]int, 0)

    // Try to get from cache
    for _, id := range userIDs {
        key := fmt.Sprintf("user:%d", id)
        value, err := c.Get(ctx, key)
        if err == cache.ErrCacheMiss {
            missedIDs = append(missedIDs, id)
            continue
        }
        users = append(users, value.(*User))
    }

    // Fetch missed users from database
    if len(missedIDs) > 0 {
        dbUsers, err := fetchUsersFromDB(db, missedIDs)
        if err != nil {
            return nil, err
        }

        // Cache them
        for _, user := range dbUsers {
            key := fmt.Sprintf("user:%d", user.ID)
            c.Set(ctx, key, user, 1*time.Hour)
            users = append(users, user)
        }
    }

    return users, nil
}
```

### Cache Invalidation

```go
// Invalidate specific key
func InvalidateUser(c cache.Cache, userID int) error {
    ctx := context.Background()
    key := fmt.Sprintf("user:%d", userID)
    return c.Delete(ctx, key)
}

// Invalidate multiple related keys
func InvalidateUserRelated(c cache.Cache, userID int) error {
    ctx := context.Background()
    keys := []string{
        fmt.Sprintf("user:%d", userID),
        fmt.Sprintf("user:%d:profile", userID),
        fmt.Sprintf("user:%d:settings", userID),
    }

    for _, key := range keys {
        if err := c.Delete(ctx, key); err != nil {
            log.Printf("Failed to delete %s: %v", key, err)
        }
    }

    return nil
}
```

### Cache Warming

```go
func WarmCache(c cache.Cache, db *sql.DB) error {
    ctx := context.Background()

    // Fetch frequently accessed data
    users, err := fetchPopularUsers(db)
    if err != nil {
        return err
    }

    // Pre-populate cache
    for _, user := range users {
        key := fmt.Sprintf("user:%d", user.ID)
        if err := c.Set(ctx, key, user, 1*time.Hour); err != nil {
            log.Printf("Failed to warm cache for user %d: %v", user.ID, err)
        }
    }

    return nil
}
```

## Memory Cache Details

### Eviction Policy

The memory cache uses a combination of:
1. **TTL-based eviction**: Expired entries are automatically removed
2. **Size-based eviction**: When memory limit is reached, oldest entries are evicted
3. **Background cleanup**: Expired entries are removed every minute

### Memory Calculation

Cache size is calculated based on JSON-serialized value size:
```go
data, _ := json.Marshal(value)
size := int64(len(data))
```

### Eviction Process

1. Check if new entry exceeds memory limit
2. Remove all expired entries first
3. If still over limit, remove oldest entries by expiration time
4. Continue until enough space is freed

## Redis Cache Details

### Connection Pooling

Redis cache uses connection pooling for efficiency:
- `PoolSize`: Maximum number of connections
- `MinIdleConns`: Minimum idle connections maintained
- `MaxIdleConns`: Maximum idle connections
- `ConnMaxLifetime`: Maximum connection lifetime

### Failover Behavior

When `EnableFallback` is true:
1. Redis operations are attempted first
2. On failure, falls back to in-memory cache
3. Both caches are kept in sync for writes
4. Provides high availability even when Redis is down

### Serialization

All values are JSON-serialized before storage:
```go
data, err := json.Marshal(value)
// Store data in Redis
```

## Best Practices

1. **Choose Appropriate TTL**
   - Short TTL (minutes): Frequently changing data
   - Medium TTL (hours): Stable user data
   - Long TTL (days): Static configuration

2. **Use Specific Keys**
   ```go
   // Good
   "user:123:profile"
   "product:456:details"

   // Bad
   "data"
   "cache1"
   ```

3. **Handle Cache Misses**
   ```go
   value, err := c.Get(ctx, key)
   if err == cache.ErrCacheMiss {
       // Always have fallback logic
   }
   ```

4. **Monitor Statistics**
   ```go
   ticker := time.NewTicker(1 * time.Minute)
   for range ticker.C {
       stats := c.Stats()
       logCacheStats(stats)
   }
   ```

5. **Use Context Timeouts**
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
   defer cancel()

   c.Get(ctx, key)
   ```

6. **Set Memory Limits**
   ```go
   config.MaxMemorySize = calculateMemoryLimit()
   // Leave headroom for other processes
   ```

## Performance Tips

1. **Batch Operations**: Group multiple operations when possible
2. **Connection Pooling**: Tune pool size based on load
3. **TTL Strategy**: Use longer TTL for static data
4. **Serialization**: Keep cached values small
5. **Monitoring**: Watch hit rates and adjust strategy

## Error Handling

```go
value, err := c.Get(ctx, key)
switch err {
case nil:
    // Success
case cache.ErrCacheMiss:
    // Key not found
case cache.ErrCacheUnavailable:
    // Cache backend unavailable
default:
    // Other errors
}
```

## Testing

```bash
# Run all tests
go test ./daemon/cache/...

# Run with race detection
go test ./daemon/cache/... -race

# Run with coverage
go test ./daemon/cache/... -cover

# Benchmark
go test ./daemon/cache/... -bench=.
```

## Redis Setup

### Docker

```bash
docker run -d -p 6379:6379 --name redis redis:7-alpine
```

### Docker Compose

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes

volumes:
  redis-data:
```

### Production Configuration

```bash
# redis.conf
maxmemory 1gb
maxmemory-policy allkeys-lru
save 900 1
save 300 10
save 60 10000
```

## Troubleshooting

### High Memory Usage

1. Check cache size: `stats.Size`
2. Reduce `MaxMemorySize`
3. Decrease TTL values
4. Monitor eviction rate

### Low Hit Rate

1. Check `stats.Hits` and `stats.Misses`
2. Increase TTL for stable data
3. Implement cache warming
4. Review cache key strategy

### Redis Connection Errors

1. Verify Redis is running: `redis-cli ping`
2. Check network connectivity
3. Verify credentials
4. Enable fallback mode

### Slow Performance

1. Increase pool size
2. Reduce timeout values
3. Use shorter keys
4. Optimize value serialization

## License

SPDX-License-Identifier: Apache-2.0
