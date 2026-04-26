package cas

import (
	"context"
	"io"
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	bytesPer int64
	last     time.Time
}

func newRateLimiter(bytesPerSec int64) *RateLimiter {
	return &RateLimiter{bytesPer: bytesPerSec}
}

func NewSharedRateLimiter(bytesPerSec int64) *RateLimiter {
	return newRateLimiter(bytesPerSec)
}

func (r *RateLimiter) SetBytesPerSec(bytesPerSec int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bytesPer = bytesPerSec
	r.last = time.Time{}
}

func (r *RateLimiter) chunkSize(max int) int {
	if r == nil || r.bytesPer <= 0 {
		return max
	}
	r.mu.Lock()
	bytesPer := r.bytesPer
	r.mu.Unlock()
	chunk := int(bytesPer / 8)
	if chunk < 1024 {
		chunk = 1024
	}
	if chunk > 64*1024 {
		chunk = 64 * 1024
	}
	if chunk > max {
		chunk = max
	}
	if chunk <= 0 {
		chunk = max
	}
	return chunk
}

func (r *RateLimiter) wait(ctx context.Context, n int) error {
	if r == nil || n <= 0 {
		return nil
	}
	r.mu.Lock()
	bytesPer := r.bytesPer
	if bytesPer <= 0 {
		r.mu.Unlock()
		return nil
	}
	now := time.Now()
	if r.last.Before(now) {
		r.last = now
	}
	d := time.Duration(int64(time.Second) * int64(n) / bytesPer)
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
	limiter *RateLimiter
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.ctx != nil {
		select {
		case <-c.ctx.Done():
			return 0, c.ctx.Err()
		default:
		}
	}
	readBuf := p
	if c.limiter != nil {
		chunk := c.limiter.chunkSize(len(p))
		if err := c.limiter.wait(c.ctx, chunk); err != nil {
			return 0, err
		}
		readBuf = p[:chunk]
	}
	n, err := c.reader.Read(readBuf)
	if n > 0 {
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
