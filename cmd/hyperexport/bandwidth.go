// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/zyvorai/transiva/logger"
)

// BandwidthLimiter controls download/upload bandwidth usage
type BandwidthLimiter struct {
	limit         int64 // bytes per second
	bucket        int64 // current available tokens
	bucketSize    int64 // maximum bucket size
	lastRefill    time.Time
	mu            sync.Mutex
	log           logger.Logger
	bytesTransferred int64
	startTime     time.Time
}

// BandwidthConfig configures bandwidth limiting
type BandwidthConfig struct {
	MaxBytesPerSecond int64         // Maximum bytes per second (0 = unlimited)
	BurstSize         int64         // Burst allowance in bytes
	UpdateInterval    time.Duration // How often to refill bucket
}

// NewBandwidthLimiter creates a new bandwidth limiter
func NewBandwidthLimiter(config *BandwidthConfig, log logger.Logger) *BandwidthLimiter {
	if config == nil || config.MaxBytesPerSecond <= 0 {
		// No limiting
		return &BandwidthLimiter{
			limit: 0,
			log:   log,
		}
	}

	burstSize := config.BurstSize
	if burstSize <= 0 {
		// Default burst: 2x the per-second limit
		burstSize = config.MaxBytesPerSecond * 2
	}

	limiter := &BandwidthLimiter{
		limit:      config.MaxBytesPerSecond,
		bucket:     burstSize, // Start with full bucket
		bucketSize: burstSize,
		lastRefill: time.Now(),
		log:        log,
		startTime:  time.Now(),
	}

	log.Info("bandwidth limiter created",
		"limit_mbps", float64(config.MaxBytesPerSecond)/1024/1024,
		"burst_mb", float64(burstSize)/1024/1024)

	return limiter
}

// Wait blocks until n bytes can be transferred
func (bl *BandwidthLimiter) Wait(ctx context.Context, n int64) error {
	if bl.limit == 0 {
		// No limiting
		return nil
	}

	bl.mu.Lock()
	defer bl.mu.Unlock()

	for {
		// Refill bucket based on time passed
		now := time.Now()
		elapsed := now.Sub(bl.lastRefill)
		refillAmount := int64(elapsed.Seconds() * float64(bl.limit))

		if refillAmount > 0 {
			bl.bucket += refillAmount
			if bl.bucket > bl.bucketSize {
				bl.bucket = bl.bucketSize
			}
			bl.lastRefill = now
		}

		// Check if we have enough tokens
		if bl.bucket >= n {
			bl.bucket -= n
			bl.bytesTransferred += n
			return nil
		}

		// Not enough tokens, wait a bit
		tokensNeeded := n - bl.bucket
		waitTime := time.Duration(float64(tokensNeeded) / float64(bl.limit) * float64(time.Second))

		// Cap wait time to prevent excessive blocking
		if waitTime > time.Second {
			waitTime = time.Second
		}

		bl.mu.Unlock()
		select {
		case <-ctx.Done():
			bl.mu.Lock()
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue loop to refill and retry
		}
		bl.mu.Lock()
	}
}

// LimitedReader wraps an io.Reader with bandwidth limiting
type LimitedReader struct {
	reader  io.Reader
	limiter *BandwidthLimiter
	ctx     context.Context
}

// NewLimitedReader creates a bandwidth-limited reader
func NewLimitedReader(reader io.Reader, limiter *BandwidthLimiter, ctx context.Context) *LimitedReader {
	return &LimitedReader{
		reader:  reader,
		limiter: limiter,
		ctx:     ctx,
	}
}

// Read implements io.Reader with bandwidth limiting
func (lr *LimitedReader) Read(p []byte) (int, error) {
	n, err := lr.reader.Read(p)
	if n > 0 && lr.limiter != nil {
		if waitErr := lr.limiter.Wait(lr.ctx, int64(n)); waitErr != nil {
			return n, waitErr
		}
	}
	return n, err
}

// LimitedWriter wraps an io.Writer with bandwidth limiting
type LimitedWriter struct {
	writer  io.Writer
	limiter *BandwidthLimiter
	ctx     context.Context
}

// NewLimitedWriter creates a bandwidth-limited writer
func NewLimitedWriter(writer io.Writer, limiter *BandwidthLimiter, ctx context.Context) *LimitedWriter {
	return &LimitedWriter{
		writer:  writer,
		limiter: limiter,
		ctx:     ctx,
	}
}

// Write implements io.Writer with bandwidth limiting
func (lw *LimitedWriter) Write(p []byte) (int, error) {
	if lw.limiter != nil {
		if err := lw.limiter.Wait(lw.ctx, int64(len(p))); err != nil {
			return 0, err
		}
	}
	return lw.writer.Write(p)
}

// GetStats returns bandwidth usage statistics
func (bl *BandwidthLimiter) GetStats() BandwidthStats {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	duration := time.Since(bl.startTime)
	avgSpeed := float64(0)
	if duration.Seconds() > 0 {
		avgSpeed = float64(bl.bytesTransferred) / duration.Seconds()
	}

	return BandwidthStats{
		BytesTransferred: bl.bytesTransferred,
		Duration:         duration,
		AverageSpeed:     avgSpeed,
		LimitSpeed:       float64(bl.limit),
	}
}

// BandwidthStats contains bandwidth usage statistics
type BandwidthStats struct {
	BytesTransferred int64
	Duration         time.Duration
	AverageSpeed     float64 // bytes per second
	LimitSpeed       float64 // bytes per second
}

// FormatSpeed formats bytes per second into human-readable form
func FormatSpeed(bytesPerSecond float64) string {
	const unit = 1024
	if bytesPerSecond < unit {
		return "< 1 KB/s"
	}

	div := float64(unit)
	exp := 0
	for n := bytesPerSecond / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB/s", "MB/s", "GB/s", "TB/s"}
	if exp >= len(units) {
		exp = len(units) - 1
	}

	return formatFloat(bytesPerSecond/div, 2) + " " + units[exp]
}

func formatFloat(f float64, decimals int) string {
	format := "%." + strconv.Itoa(decimals) + "f"
	return fmt.Sprintf(format, f)
}

// AdaptiveBandwidthLimiter adjusts bandwidth based on network conditions
type AdaptiveBandwidthLimiter struct {
	baseLimiter    *BandwidthLimiter
	minSpeed       int64
	maxSpeed       int64
	currentSpeed   int64
	errorRate      float64
	successRate    float64
	adjustInterval time.Duration
	lastAdjust     time.Time
	mu             sync.Mutex
	log            logger.Logger
}

// NewAdaptiveBandwidthLimiter creates an adaptive bandwidth limiter
func NewAdaptiveBandwidthLimiter(minSpeed, maxSpeed int64, log logger.Logger) *AdaptiveBandwidthLimiter {
	if minSpeed <= 0 {
		minSpeed = 1024 * 1024 // 1 MB/s minimum
	}
	if maxSpeed <= 0 {
		maxSpeed = 100 * 1024 * 1024 // 100 MB/s maximum
	}

	currentSpeed := (minSpeed + maxSpeed) / 2

	config := &BandwidthConfig{
		MaxBytesPerSecond: currentSpeed,
	}

	return &AdaptiveBandwidthLimiter{
		baseLimiter:    NewBandwidthLimiter(config, log),
		minSpeed:       minSpeed,
		maxSpeed:       maxSpeed,
		currentSpeed:   currentSpeed,
		adjustInterval: 10 * time.Second,
		lastAdjust:     time.Now(),
		log:            log,
	}
}

// RecordSuccess records a successful transfer
func (abl *AdaptiveBandwidthLimiter) RecordSuccess() {
	abl.mu.Lock()
	defer abl.mu.Unlock()

	abl.successRate = abl.successRate*0.9 + 0.1 // Exponential moving average
	abl.adjustSpeed()
}

// RecordError records a failed transfer
func (abl *AdaptiveBandwidthLimiter) RecordError() {
	abl.mu.Lock()
	defer abl.mu.Unlock()

	abl.errorRate = abl.errorRate*0.9 + 0.1 // Exponential moving average
	abl.adjustSpeed()
}

// adjustSpeed adjusts bandwidth based on error/success rate
func (abl *AdaptiveBandwidthLimiter) adjustSpeed() {
	now := time.Now()
	if now.Sub(abl.lastAdjust) < abl.adjustInterval {
		return
	}

	oldSpeed := abl.currentSpeed

	// Increase speed if success rate is high
	if abl.successRate > 0.9 && abl.errorRate < 0.05 {
		abl.currentSpeed = int64(float64(abl.currentSpeed) * 1.2)
		if abl.currentSpeed > abl.maxSpeed {
			abl.currentSpeed = abl.maxSpeed
		}
	} else if abl.errorRate > 0.1 {
		// Decrease speed if error rate is high
		abl.currentSpeed = int64(float64(abl.currentSpeed) * 0.8)
		if abl.currentSpeed < abl.minSpeed {
			abl.currentSpeed = abl.minSpeed
		}
	}

	if oldSpeed != abl.currentSpeed {
		abl.log.Info("bandwidth adjusted",
			"old_mbps", float64(oldSpeed)/1024/1024,
			"new_mbps", float64(abl.currentSpeed)/1024/1024,
			"success_rate", abl.successRate,
			"error_rate", abl.errorRate)

		// Update base limiter
		abl.baseLimiter.mu.Lock()
		abl.baseLimiter.limit = abl.currentSpeed
		abl.baseLimiter.mu.Unlock()
	}

	abl.lastAdjust = now
}

// Wait wraps the base limiter's Wait method
func (abl *AdaptiveBandwidthLimiter) Wait(ctx context.Context, n int64) error {
	return abl.baseLimiter.Wait(ctx, n)
}

// GetStats wraps the base limiter's GetStats method
func (abl *AdaptiveBandwidthLimiter) GetStats() BandwidthStats {
	return abl.baseLimiter.GetStats()
}
