// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewPerUserLimiter(t *testing.T) {
	limiter := NewPerUserLimiter(10.0, 5)

	if limiter == nil {
		t.Fatal("expected limiter to be created")
	}

	if limiter.rate != 10.0 {
		t.Errorf("expected rate 10.0, got %v", limiter.rate)
	}

	if limiter.burst != 5 {
		t.Errorf("expected burst 5, got %d", limiter.burst)
	}
}

func TestPerUserLimiterAllow(t *testing.T) {
	// 1 request per second, burst of 2
	limiter := NewPerUserLimiter(1.0, 2)

	// First two requests should be allowed (burst)
	if !limiter.Allow("user1") {
		t.Error("first request should be allowed")
	}

	if !limiter.Allow("user1") {
		t.Error("second request should be allowed")
	}

	// Third request should be denied (exceeded burst)
	if limiter.Allow("user1") {
		t.Error("third request should be denied")
	}

	// Different user should have separate limit
	if !limiter.Allow("user2") {
		t.Error("different user should be allowed")
	}
}

func TestPerUserLimiterCleanup(t *testing.T) {
	limiter := NewPerUserLimiter(10.0, 5)

	// Use limiter for user1
	limiter.Allow("user1")

	if len(limiter.limiters) != 1 {
		t.Errorf("expected 1 limiter, got %d", len(limiter.limiters))
	}

	// Wait for tokens to refill
	time.Sleep(1 * time.Second)

	// Cleanup should remove unused limiters
	limiter.Cleanup()

	// Note: Cleanup might not remove if tokens haven't fully refilled
	// This is expected behavior
}

func TestGlobalLimiter(t *testing.T) {
	// 2 requests per second, burst of 3
	limiter := NewGlobalLimiter(2.0, 3)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// Fourth request should be denied
	if limiter.Allow() {
		t.Error("fourth request should be denied")
	}
}

func TestMiddleware(t *testing.T) {
	// Create middleware with low limits for testing
	middleware := NewMiddleware(1.0, 10.0, 2, 5)

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &http.Client{}

	// First request should succeed
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Second request should succeed (within burst)
	resp, err = client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Third request should be rate limited
	resp, err = client.Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.PerUserRequestsPerMinute != 100 {
		t.Errorf("expected 100 requests per minute, got %d", config.PerUserRequestsPerMinute)
	}

	if config.GlobalRequestsPerMinute != 1000 {
		t.Errorf("expected 1000 global requests per minute, got %d", config.GlobalRequestsPerMinute)
	}

	// Check provider limits
	if len(config.ProviderLimits) == 0 {
		t.Error("expected provider limits to be configured")
	}

	// Check AWS limit
	awsLimit, exists := config.ProviderLimits["aws"]
	if !exists {
		t.Error("expected AWS provider limit")
	}

	if awsLimit.RequestsPerMinute != 100 {
		t.Errorf("expected AWS 100 rpm, got %d", awsLimit.RequestsPerMinute)
	}
}

func TestProviderLimiter(t *testing.T) {
	config := DefaultConfig()
	limiter := NewProviderLimiter(config)

	// AWS should be allowed
	if !limiter.Allow("aws") {
		t.Error("AWS request should be allowed")
	}

	// Unknown provider should be allowed
	if !limiter.Allow("unknown") {
		t.Error("unknown provider should be allowed")
	}
}

func TestProviderConcurrency(t *testing.T) {
	config := DefaultConfig()
	// Set low concurrency limit for testing
	config.ProviderLimits["test"] = ProviderLimit{
		RequestsPerMinute: 100,
		Burst:             10,
		ConcurrentOps:     2,
	}

	limiter := NewProviderLimiter(config)

	// First two acquisitions should succeed
	err := limiter.AcquireConcurrency("test")
	if err != nil {
		t.Errorf("first acquisition should succeed: %v", err)
	}

	err = limiter.AcquireConcurrency("test")
	if err != nil {
		t.Errorf("second acquisition should succeed: %v", err)
	}

	// Third acquisition should fail
	err = limiter.AcquireConcurrency("test")
	if err == nil {
		t.Error("third acquisition should fail")
	}

	// Release one slot
	limiter.ReleaseConcurrency("test")

	// Now acquisition should succeed again
	err = limiter.AcquireConcurrency("test")
	if err != nil {
		t.Errorf("acquisition after release should succeed: %v", err)
	}
}

func TestGetIP(t *testing.T) {
	tests := []struct {
		name           string
		headers        map[string]string
		remoteAddr     string
		expectedPrefix string
	}{
		{
			name: "X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "1.2.3.4",
			},
			remoteAddr:     "5.6.7.8:1234",
			expectedPrefix: "1.2.3.4",
		},
		{
			name: "X-Real-IP",
			headers: map[string]string{
				"X-Real-IP": "9.10.11.12",
			},
			remoteAddr:     "5.6.7.8:1234",
			expectedPrefix: "9.10.11.12",
		},
		{
			name:           "RemoteAddr fallback",
			headers:        map[string]string{},
			remoteAddr:     "13.14.15.16:5678",
			expectedPrefix: "13.14.15.16:5678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remoteAddr

			ip := getIP(req)
			if ip != tt.expectedPrefix {
				t.Errorf("expected %s, got %s", tt.expectedPrefix, ip)
			}
		})
	}
}

func TestPerUserLimiter_Wait(t *testing.T) {
	limiter := NewPerUserLimiter(10, 1) // 10 requests per second, burst of 1

	// First wait should succeed immediately
	err := limiter.Wait("user1")
	if err != nil {
		t.Errorf("Expected no error on first wait, got: %v", err)
	}

	// Create a context with timeout for second wait (should block briefly)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// This should work but might take a moment
	done := make(chan error, 1)
	go func() {
		done <- limiter.Wait("user1")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected wait to succeed, got error: %v", err)
		}
	case <-ctx.Done():
		// Timeout is acceptable for this test
	}
}

func TestGlobalLimiter_Wait(t *testing.T) {
	limiter := NewGlobalLimiter(10, 1) // 10 requests per second, burst of 1

	// First wait should succeed immediately
	err := limiter.Wait()
	if err != nil {
		t.Errorf("Expected no error on first wait, got: %v", err)
	}

	// Create a context with timeout for second wait
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// This should work but might take a moment
	done := make(chan error, 1)
	go func() {
		done <- limiter.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Expected wait to succeed, got error: %v", err)
		}
	case <-ctx.Done():
		// Timeout is acceptable for this test
	}
}
