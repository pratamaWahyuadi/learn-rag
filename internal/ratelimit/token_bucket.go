// Package ratelimit provides a simple in-memory token bucket rate limiter
// suitable for a single-VPS, demo-scale deployment. No external dependency or
// Redis is required.
package ratelimit

import (
	"sync"
	"time"
)

// bucket holds the token state for a single key.
type bucket struct {
	// tokens is the current number of available tokens.
	tokens float64
	// last is the last time tokens were refilled.
	last time.Time
}

// TokenBucket rate-limits arbitrary keys (an API key id, a tenant id, or a
// client IP) with a fixed capacity and a per-second refill rate.
type TokenBucket struct {
	mu       sync.Mutex
	capacity float64
	refill   float64 // tokens added per second
	buckets  map[string]*bucket
	now      func() time.Time
}

// NewTokenBucket creates a TokenBucket with the given capacity and refill rate.
// capacity is the maximum burst allowed, and perSecond is how many tokens are
// added each second. Both must be > 0.
func NewTokenBucket(capacity, perSecond float64) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		refill:   perSecond,
		buckets:  make(map[string]*bucket),
		now:      time.Now,
	}
}

// Allow reports whether a token is available for key, consuming one if so. The
// bucket refills lazily based on elapsed time since the last access.
func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.now()

	b, ok := tb.buckets[key]
	if !ok {
		// Start a fresh, full bucket for an unknown key.
		b = &bucket{tokens: tb.capacity, last: now}
		tb.buckets[key] = b
	}

	tb.refillBucket(b, now)

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// refillBucket adds tokens proportional to elapsed time, capped at capacity,
// and updates the last refill time when tokens were gained.
func (tb *TokenBucket) refillBucket(b *bucket, now time.Time) {
	elapsed := now.Sub(b.last).Seconds()
	if elapsed <= 0 {
		return
	}
	b.tokens += elapsed * tb.refill
	if b.tokens > tb.capacity {
		b.tokens = tb.capacity
	}
	b.last = now
}

// Reset drops all tracked buckets. Useful for tests and shutdown.
func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.buckets = make(map[string]*bucket)
}
