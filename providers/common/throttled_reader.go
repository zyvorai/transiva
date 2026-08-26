// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

// ThrottledReader wraps an io.Reader to limit read throughput
type ThrottledReader struct {
	reader  io.Reader
	limiter *rate.Limiter
	ctx     context.Context
}

// NewThrottledReader creates a new bandwidth-throttled reader
// bytesPerSecond: Maximum bytes per second (0 = unlimited)
// burstSize: Maximum burst size in bytes (0 = same as rate)
func NewThrottledReader(reader io.Reader, bytesPerSecond int64, burstSize int) io.Reader {
	if bytesPerSecond <= 0 {
		// No throttling
		return reader
	}

	if burstSize <= 0 {
		// Default burst is 10% of rate or minimum 64KB
		burstSize = int(bytesPerSecond / 10)
		if burstSize < 65536 {
			burstSize = 65536
		}
	}

	return &ThrottledReader{
		reader:  reader,
		limiter: rate.NewLimiter(rate.Limit(bytesPerSecond), burstSize),
		ctx:     context.Background(),
	}
}

// NewThrottledReaderWithContext creates a throttled reader with context support
func NewThrottledReaderWithContext(ctx context.Context, reader io.Reader, bytesPerSecond int64, burstSize int) io.Reader {
	if bytesPerSecond <= 0 {
		return reader
	}

	if burstSize <= 0 {
		burstSize = int(bytesPerSecond / 10)
		if burstSize < 65536 {
			burstSize = 65536
		}
	}

	return &ThrottledReader{
		reader:  reader,
		limiter: rate.NewLimiter(rate.Limit(bytesPerSecond), burstSize),
		ctx:     ctx,
	}
}

// Read implements io.Reader with rate limiting
func (tr *ThrottledReader) Read(p []byte) (int, error) {
	// Wait for permission to read
	if err := tr.limiter.WaitN(tr.ctx, len(p)); err != nil {
		return 0, err
	}

	// Perform the actual read
	return tr.reader.Read(p)
}

// SetBytesPerSecond dynamically changes the rate limit
func (tr *ThrottledReader) SetBytesPerSecond(bytesPerSecond int64) {
	tr.limiter.SetLimit(rate.Limit(bytesPerSecond))
}

// SetBurst dynamically changes the burst size
func (tr *ThrottledReader) SetBurst(burstSize int) {
	tr.limiter.SetBurst(burstSize)
}
