package api

import (
	"sync"
	"time"
)

// tokenBucket is a simple per-key token-bucket rate limiter. It is used to cap
// request rates per client IP without pulling in an external dependency.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64 // tokens refilled per second
	last     time.Time
}

func newBucket(rate, capacity float64) *tokenBucket {
	return &tokenBucket{tokens: capacity, capacity: capacity, rate: rate, last: time.Now()}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// rateLimiter holds a bucket per client key and evicts idle keys periodically.
type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64
	capacity float64
}

func newRateLimiter(rps, burst int) *rateLimiter {
	rl := &rateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     float64(rps),
		capacity: float64(burst),
	}
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = newBucket(rl.rate, rl.capacity)
		rl.buckets[key] = b
	}
	rl.mu.Unlock()
	return b.allow()
}
