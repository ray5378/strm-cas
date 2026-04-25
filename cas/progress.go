package cas

import (
	"context"
	"io"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	bytesPer int64
	last     time.Time
}

func newRateLimiter(bytesPerSec int64) *rateLimiter {
	if bytesPerSec <= 0 {
		return nil
	}
	return &rateLimiter{bytesPer: bytesPerSec}
}

func NewSharedRateLimiter(bytesPerSec int64) *rateLimiter {
	return newRateLimiter(bytesPerSec)
}

func (r *rateLimiter) wait(ctx context.Context, n int) error {
	if r == nil || r.bytesPer <= 0 || n <= 0 {
		return nil
	}
	r.mu.Lock()
	now := time.Now()
	if r.last.Before(now) {
		r.last = now
	}
	d := time.Duration(int64(time.Second) * int64(n) / r.bytesPer)
	target := r.last.Add(d)
	wait := time.Until(target)
	if wait < 0 {
		wait = 0
	}
	r.last = target
	r.mu.Unlock()
	if wait == 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type countingReader struct {
	ctx     context.Context
	reader  io.Reader
	total   int64
	onRead  func(total int64)
	limiter *rateLimiter
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.ctx != nil {
		select {
		case <-c.ctx.Done():
			return 0, c.ctx.Err()
		default:
		}
	}
	n, err := c.reader.Read(p)
	if n > 0 {
		if c.limiter != nil {
			if waitErr := c.limiter.wait(c.ctx, n); waitErr != nil {
				return 0, waitErr
			}
		}
		c.total += int64(n)
		if c.onRead != nil {
			c.onRead(c.total)
		}
	}
	return n, err
}

func contentTotal(contentLength, partialSize int64) int64 {
	if contentLength <= 0 {
		return 0
	}
	return partialSize + contentLength
}
