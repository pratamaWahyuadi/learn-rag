package ratelimit

import (
	"testing"
	"time"
)

func TestAllowUnderCapacity(t *testing.T) {
	tb := NewTokenBucket(3, 1)
	tb.now = func() time.Time { return time.Unix(0, 0) }

	for i := 0; i < 3; i++ {
		if !tb.Allow("key") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestAllowOverCapacity(t *testing.T) {
	tb := NewTokenBucket(3, 1)
	tb.now = func() time.Time { return time.Unix(0, 0) }

	for i := 0; i < 3; i++ {
		tb.Allow("key")
	}
	// 4th request within the same instant is denied.
	if tb.Allow("key") {
		t.Fatal("request over capacity should be denied")
	}
}

func TestAllowRefillAfterTime(t *testing.T) {
	tb := NewTokenBucket(1, 1) // capacity 1, refill 1 token/sec
	base := time.Unix(1000, 0)
	current := base
	tb.now = func() time.Time { return current }

	if !tb.Allow("key") {
		t.Fatal("first request should be allowed")
	}
	if tb.Allow("key") {
		t.Fatal("second request before refill should be denied")
	}

	// Wait 2 seconds: bucket should have refilled ~2 tokens, capped at 1.
	current = base.Add(2 * time.Second)
	if !tb.Allow("key") {
		t.Fatal("request after refill should be allowed")
	}
	// Only 1 token refilled (capped at capacity), so the next is denied.
	if tb.Allow("key") {
		t.Fatal("second request after single-token refill should be denied")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	tb := NewTokenBucket(1, 1)
	tb.now = func() time.Time { return time.Unix(0, 0) }

	if !tb.Allow("a") {
		t.Fatal("key a should be allowed")
	}
	if tb.Allow("a") {
		t.Fatal("key a second request should be denied")
	}
	if !tb.Allow("b") {
		t.Fatal("key b should be independent of key a")
	}
}

func TestResetClearsState(t *testing.T) {
	tb := NewTokenBucket(1, 1)
	tb.now = func() time.Time { return time.Unix(0, 0) }

	tb.Allow("a")
	tb.Allow("a") // consume the 1 token
	if tb.Allow("a") {
		t.Fatal("expected key a to be limited before reset")
	}

	tb.Reset()
	if !tb.Allow("a") {
		t.Fatal("expected key a to be allowed after reset")
	}
}
