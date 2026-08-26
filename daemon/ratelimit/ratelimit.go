// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter interface for rate limiting
type Limiter interface {
	Allow() bool
	Wait() error
}

// PerUserLimiter manages rate limits per user
type PerUserLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewPerUserLimiter creates a new per-user rate limiter
// rate: requests per second per user
// burst: maximum burst size
func NewPerUserLimiter(r float64, burst int) *PerUserLimiter {
	return &PerUserLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(r),
		burst:    burst,
	}
}

// GetLimiter returns the rate limiter for a specific user
func (pul *PerUserLimiter) GetLimiter(userID string) *rate.Limiter {
	pul.mu.Lock()
	defer pul.mu.Unlock()

	limiter, exists := pul.limiters[userID]
	if !exists {
		limiter = rate.NewLimiter(pul.rate, pul.burst)
		pul.limiters[userID] = limiter
	}

	return limiter
}

// Allow checks if a request is allowed for a user
func (pul *PerUserLimiter) Allow(userID string) bool {
	limiter := pul.GetLimiter(userID)
	return limiter.Allow()
}

// Wait blocks until a request can proceed for a user
func (pul *PerUserLimiter) Wait(userID string) error {
	limiter := pul.GetLimiter(userID)
	return limiter.Wait(context.Background())
}

// Cleanup removes inactive limiters (call periodically)
func (pul *PerUserLimiter) Cleanup() {
	pul.mu.Lock()
	defer pul.mu.Unlock()

	// Remove limiters that haven't been used recently
	// This prevents memory leaks for one-time users
	now := time.Now()
	for userID, limiter := range pul.limiters {
		// If limiter has full burst capacity, it hasn't been used recently
		if limiter.Tokens() >= float64(pul.burst) {
			delete(pul.limiters, userID)
		}
		_ = now // Use now to avoid unused warning
	}
}

// GlobalLimiter manages global rate limits
type GlobalLimiter struct {
	limiter *rate.Limiter
}

// NewGlobalLimiter creates a new global rate limiter
func NewGlobalLimiter(r float64, burst int) *GlobalLimiter {
	return &GlobalLimiter{
		limiter: rate.NewLimiter(rate.Limit(r), burst),
	}
}

// Allow checks if a request is allowed globally
func (gl *GlobalLimiter) Allow() bool {
	return gl.limiter.Allow()
}

// Wait blocks until a request can proceed globally
func (gl *GlobalLimiter) Wait() error {
	return gl.limiter.Wait(context.Background())
}

// Middleware creates HTTP middleware for rate limiting
type Middleware struct {
	perUser *PerUserLimiter
	global  *GlobalLimiter
}

// NewMiddleware creates rate limiting middleware
func NewMiddleware(perUserRate, globalRate float64, perUserBurst, globalBurst int) *Middleware {
	return &Middleware{
		perUser: NewPerUserLimiter(perUserRate, perUserBurst),
		global:  NewGlobalLimiter(globalRate, globalBurst),
	}
}

// Handler returns HTTP middleware handler
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check global rate limit first
		if !m.global.Allow() {
			http.Error(w, "global rate limit exceeded", http.StatusTooManyRequests)
			w.Header().Set("Retry-After", "60")
			return
		}

		// Get user ID from context or header
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			// Fallback to IP address
			userID = getIP(r)
		}

		// Check per-user rate limit
		if !m.perUser.Allow(userID) {
			http.Error(w, "user rate limit exceeded", http.StatusTooManyRequests)
			w.Header().Set("Retry-After", "60")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getIP extracts the IP address from request
func getIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// Config holds rate limiter configuration
type Config struct {
	// Per-user limits
	PerUserRequestsPerMinute int
	PerUserBurst             int

	// Global limits
	GlobalRequestsPerMinute int
	GlobalBurst             int

	// Provider-specific limits
	ProviderLimits map[string]ProviderLimit
}

// ProviderLimit holds rate limits for a cloud provider
type ProviderLimit struct {
	RequestsPerMinute int
	Burst             int
	ConcurrentOps     int // Max concurrent operations
}

// DefaultConfig returns default rate limit configuration
func DefaultConfig() *Config {
	return &Config{
		PerUserRequestsPerMinute: 100,
		PerUserBurst:             20,
		GlobalRequestsPerMinute:  1000,
		GlobalBurst:              200,
		ProviderLimits: map[string]ProviderLimit{
			"aws": {
				RequestsPerMinute: 100,
				Burst:             20,
				ConcurrentOps:     10,
			},
			"azure": {
				RequestsPerMinute: 100,
				Burst:             20,
				ConcurrentOps:     10,
			},
			"gcp": {
				RequestsPerMinute: 100,
				Burst:             20,
				ConcurrentOps:     10,
			},
			"vsphere": {
				RequestsPerMinute: 50,
				Burst:             10,
				ConcurrentOps:     5,
			},
		},
	}
}

// ProviderLimiter manages rate limits for cloud providers
type ProviderLimiter struct {
	limiters    map[string]*rate.Limiter
	concurrency map[string]chan struct{} // Semaphore for concurrent ops
	mu          sync.RWMutex
	config      *Config
}

// NewProviderLimiter creates a new provider rate limiter
func NewProviderLimiter(config *Config) *ProviderLimiter {
	pl := &ProviderLimiter{
		limiters:    make(map[string]*rate.Limiter),
		concurrency: make(map[string]chan struct{}),
		config:      config,
	}

	// Initialize limiters and semaphores for each provider
	for provider, limits := range config.ProviderLimits {
		rps := float64(limits.RequestsPerMinute) / 60.0
		pl.limiters[provider] = rate.NewLimiter(rate.Limit(rps), limits.Burst)
		pl.concurrency[provider] = make(chan struct{}, limits.ConcurrentOps)
	}

	return pl
}

// Allow checks if a request to a provider is allowed
func (pl *ProviderLimiter) Allow(provider string) bool {
	pl.mu.RLock()
	limiter, exists := pl.limiters[provider]
	pl.mu.RUnlock()

	if !exists {
		return true // No limit for unknown providers
	}

	return limiter.Allow()
}

// AcquireConcurrency acquires a concurrency slot for a provider
func (pl *ProviderLimiter) AcquireConcurrency(provider string) error {
	pl.mu.RLock()
	sem, exists := pl.concurrency[provider]
	pl.mu.RUnlock()

	if !exists {
		return nil // No limit for unknown providers
	}

	select {
	case sem <- struct{}{}:
		return nil
	default:
		return fmt.Errorf("provider %s concurrency limit reached", provider)
	}
}

// ReleaseConcurrency releases a concurrency slot for a provider
func (pl *ProviderLimiter) ReleaseConcurrency(provider string) {
	pl.mu.RLock()
	sem, exists := pl.concurrency[provider]
	pl.mu.RUnlock()

	if !exists {
		return
	}

	select {
	case <-sem:
	default:
	}
}
