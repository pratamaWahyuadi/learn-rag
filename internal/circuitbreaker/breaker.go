// Package circuitbreaker implements a lightweight in-memory circuit breaker for
// calls to external providers. It follows a closed → open → half-open → closed
// state machine, is safe for concurrent use, and depends on no external libs.
package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State is the circuit breaker state.
type State int

// Circuit breaker states.
const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// Config controls breaker behavior.
type Config struct {
	// MaxFailures is the number of consecutive failures that trip the breaker
	// from closed to open.
	MaxFailures int
	// Timeout is how long the breaker stays open before allowing probe calls.
	Timeout time.Duration
	// HalfOpenMaxCalls is the number of probe calls allowed while half-open.
	HalfOpenMaxCalls int
}

// ErrOpen is returned when the breaker is open and refuses to execute a call.
var ErrOpen = errors.New("circuit breaker open")

// Breaker is a concurrent-safe circuit breaker.
type Breaker struct {
	mu     sync.Mutex
	config Config

	state         State
	failures      int
	openedAt      time.Time
	halfOpenCalls int

	// now allows tests to control time; defaults to time.Now.
	now func() time.Time
}

// New returns a Breaker with the given configuration.
func New(cfg Config) *Breaker {
	return &Breaker{
		config: cfg,
		now:    time.Now,
	}
}

// State returns the current effective state, lazily transitioning open→half-open
// once the timeout has elapsed.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.evaluateTimeout()
	return b.state
}

// Execute runs fn guarded by the breaker. While open (before timeout) calls
// fail fast with ErrOpen without invoking fn. While half-open, up to
// HalfOpenMaxCalls probe calls are allowed; a success closes the breaker and a
// failure reopens it. While closed, failures are counted and trip the breaker
// once MaxFailures is reached.
//
// If ctx is cancellable and is done while fn is still running, Execute returns
// ctx.Err() without recording the call as a provider failure. fn continues to
// run in the background and its eventual result is ignored.
func Execute[T any](ctx context.Context, b *Breaker, fn func() (T, error)) (T, error) {
	var zero T

	if !b.allowAcquire() {
		return zero, ErrOpen
	}

	// Run fn in a goroutine only when the caller provides a cancellable context
	// so we can abort on cancellation. Otherwise run synchronously to avoid the
	// goroutine overhead in the common no-cancellation path.
	if ctx == nil || ctx.Done() == nil {
		value, err := fn()
		b.record(err)
		return value, err
	}

	type outcome struct {
		value T
		err   error
	}
	ch := make(chan outcome, 1)
	go func() {
		v, err := fn()
		ch <- outcome{value: v, err: err}
	}()

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case res := <-ch:
		b.record(res.err)
		return res.value, res.err
	}
}

// record updates the breaker state after fn completes: a success resets the
// failure counters (and closes a half-open breaker), while a failure increments
// failures or reopens the breaker as appropriate.
func (b *Breaker) record(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err == nil {
		// Success: reset failure state. If we were probing in half-open, close.
		b.failures = 0
		b.halfOpenCalls = 0
		if b.state == StateHalfOpen {
			b.state = StateClosed
		}
		return
	}

	// Failure.
	if b.state == StateHalfOpen {
		// Probe call failed -> reopen immediately.
		b.open()
	} else if b.state == StateClosed {
		b.failures++
		if b.config.MaxFailures > 0 && b.failures >= b.config.MaxFailures {
			b.open()
		}
	}
}

// allowAcquire decides whether fn may run, updating state as needed. It returns
// false when the breaker is open (before timeout) or half-open capacity is
// exhausted.
func (b *Breaker) allowAcquire() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.evaluateTimeout()

	switch b.state {
	case StateOpen:
		return false
	case StateHalfOpen:
		if b.config.HalfOpenMaxCalls > 0 && b.halfOpenCalls >= b.config.HalfOpenMaxCalls {
			return false
		}
		b.halfOpenCalls++
		return true
	default: // closed
		return true
	}
}

// evaluateTimeout transitions open→half-open once Timeout has elapsed.
func (b *Breaker) evaluateTimeout() {
	if b.state != StateOpen {
		return
	}
	if b.config.Timeout <= 0 {
		return
	}
	if b.now().Sub(b.openedAt) >= b.config.Timeout {
		b.state = StateHalfOpen
		b.halfOpenCalls = 0
	}
}

// open moves the breaker to the open state and resets counters.
func (b *Breaker) open() {
	b.state = StateOpen
	b.openedAt = b.now()
	b.halfOpenCalls = 0
}
