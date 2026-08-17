package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeClock is an injectable time source for deterministic tests.
type fakeClock struct {
	t time.Time
}

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestBreaker(cfg Config, clock *fakeClock) *Breaker {
	b := New(cfg)
	b.now = clock.Now
	return b
}

var errBoom = errors.New("boom")

func success[T any](v T) func() (T, error) {
	return func() (T, error) { return v, nil }
}

func fail[T any]() func() (T, error) {
	return func() (T, error) {
		var zero T
		return zero, errBoom
	}
}

func TestClosedToOpen(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{
		MaxFailures:      3,
		Timeout:          time.Minute,
		HalfOpenMaxCalls: 1,
	}, clock)

	if got := b.State(); got != StateClosed {
		t.Fatalf("initial state = %v, want closed", got)
	}

	// Successes do not affect the breaker.
	if _, err := Execute(context.Background(), b, success(42)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := b.State(); got != StateClosed {
		t.Fatalf("state after success = %v, want closed", got)
	}

	// Two failures keep it closed.
	for i := 0; i < 2; i++ {
		if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
			t.Fatalf("call %d expected errBoom, got %v", i, err)
		}
	}
	if got := b.State(); got != StateClosed {
		t.Fatalf("state after 2 failures = %v, want closed", got)
	}

	// Third failure trips it open.
	if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
		t.Fatalf("expected errBoom, got %v", err)
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("state after max failures = %v, want open", got)
	}
}

func TestOpenRejectsBeforeTimeout(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{MaxFailures: 1, Timeout: time.Minute, HalfOpenMaxCalls: 1}, clock)

	if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
		t.Fatal("expected first failure to trip open")
	}

	// Before timeout elapses, calls are rejected without running fn.
	ran := false
	_, err := Execute(context.Background(), b, func() (int, error) {
		ran = true
		return 1, nil
	})
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
	if ran {
		t.Fatal("fn must not be executed while breaker is open")
	}
}

func TestOpenToHalfOpenAfterTimeout(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{MaxFailures: 1, Timeout: time.Second, HalfOpenMaxCalls: 1}, clock)

	if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
		t.Fatal("expected first failure to trip open")
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("state = %v, want open", got)
	}

	// Before timeout: still open.
	clock.Advance(900 * time.Millisecond)
	if got := b.State(); got != StateOpen {
		t.Fatalf("state before timeout = %v, want open", got)
	}

	// After timeout: half-open.
	clock.Advance(200 * time.Millisecond)
	if got := b.State(); got != StateHalfOpen {
		t.Fatalf("state after timeout = %v, want half-open", got)
	}
}

func TestHalfOpenToClosedOnSuccess(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{MaxFailures: 1, Timeout: time.Second, HalfOpenMaxCalls: 1}, clock)

	// Trip open.
	if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Second) // → half-open

	if _, err := Execute(context.Background(), b, success(7)); err != nil {
		t.Fatalf("probe should succeed, got %v", err)
	}
	if got := b.State(); got != StateClosed {
		t.Fatalf("state after successful probe = %v, want closed", got)
	}
}

func TestHalfOpenCapacity(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{MaxFailures: 1, Timeout: time.Second, HalfOpenMaxCalls: 1}, clock)

	// Trip open.
	if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Second) // → half-open

	// Start a probe that blocks in-flight, holding the single half-open slot.
	release := make(chan struct{})
	ran := make(chan struct{}, 1)
	var once sync.Once
	go func() {
		Execute[int](context.Background(), b, func() (int, error) {
			once.Do(func() { ran <- struct{}{} })
			<-release
			return 1, nil
		})
	}()
	<-ran // the probe is now in-flight

	// While half-open capacity (1) is exhausted, another call must fail fast.
	if _, err := Execute(context.Background(), b, success(2)); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen while half-open capacity exhausted, got %v", err)
	}
	close(release)
}

func TestHalfOpenFailureReopens(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{MaxFailures: 1, Timeout: time.Second, HalfOpenMaxCalls: 1}, clock)

	if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
		t.Fatal(err)
	}
	clock.Advance(2 * time.Second) // → half-open

	if _, err := Execute(context.Background(), b, fail[int]()); err != errBoom {
		t.Fatalf("probe failure should propagate, got %v", err)
	}
	if got := b.State(); got != StateOpen {
		t.Fatalf("state after failed probe = %v, want open", got)
	}
}

func TestContextCancellationPropagates(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{MaxFailures: 3, Timeout: time.Second, HalfOpenMaxCalls: 1}, clock)

	// fn blocks until ctx is done; Execute must return ctx.Err() promptly.
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		_, err := Execute[int](ctx, b, func() (int, error) {
			<-ctx.Done() // block, mimicking a long-running provider call
			return 0, ctx.Err()
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		close(done)
	}()

	// Give the goroutine a chance to start fn, then cancel.
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Execute did not return after context cancellation")
	}

	// A cancelled call must not count as a provider failure.
	if got := b.State(); got != StateClosed {
		t.Fatalf("state after cancelled call = %v, want closed", got)
	}
}

func TestSingleFailureDoesNotTripBreaker(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	b := newTestBreaker(Config{MaxFailures: 3, Timeout: time.Second, HalfOpenMaxCalls: 1}, clock)

	// A single failure (with an active context) propagates the error and keeps
	// the breaker closed.
	ctx := context.Background()
	var someErr = errors.New("boom")
	if _, err := Execute(ctx, b, func() (int, error) { return 0, someErr }); !errors.Is(err, someErr) {
		t.Fatalf("expected fn error to propagate, got %v", err)
	}
	if got := b.State(); got != StateClosed {
		t.Fatalf("state should remain closed on single failure, got %v", got)
	}
}
