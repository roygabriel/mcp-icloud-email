package main

import (
	"fmt"
	"sync"
	"time"
)

// BreakerState represents the current state of the circuit breaker.
type BreakerState int

const (
	// StateClosed indicates normal operation — all requests pass through.
	StateClosed BreakerState = iota
	// StateOpen indicates the breaker is rejecting requests (failing fast).
	StateOpen
	// StateHalfOpen indicates the breaker is allowing one probe request.
	StateHalfOpen
)

// String returns the human-readable name of the breaker state.
func (s BreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements a three-state circuit breaker pattern.
//
// Closed: all requests pass through. After failThreshold consecutive failures
// the breaker transitions to Open.
//
// Open: all requests are rejected immediately. After resetTimeout elapses the
// breaker transitions to HalfOpen.
//
// HalfOpen: one probe request is allowed. On success the breaker returns to
// Closed; on failure it returns to Open.
type CircuitBreaker struct {
	mu             sync.Mutex
	state          BreakerState
	failures       int
	failThreshold  int
	resetTimeout   time.Duration
	lastFailure    time.Time
	halfOpenActive bool
	now            func() time.Time
}

// NewCircuitBreaker creates a circuit breaker that opens after failThreshold
// consecutive failures and resets after resetTimeout.
func NewCircuitBreaker(failThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failThreshold: failThreshold,
		resetTimeout:  resetTimeout,
		now:           time.Now,
	}
}

// Allow checks whether a request is permitted. If allowed, it returns a done
// callback that must be called with the outcome of the request. Only Go errors
// (err != nil) should be reported as failures; application-level result.IsError
// should not trip the breaker.
//
// Returns an error if the circuit is open.
func (cb *CircuitBreaker) Allow() (done func(success bool), err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return cb.recordOutcome, nil

	case StateOpen:
		if cb.now().Sub(cb.lastFailure) >= cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenActive = true
			return cb.recordOutcome, nil
		}
		return nil, fmt.Errorf("circuit breaker is open (upstream failures exceeded %d, retry after %s)",
			cb.failThreshold, cb.resetTimeout)

	case StateHalfOpen:
		if cb.halfOpenActive {
			return nil, fmt.Errorf("circuit breaker is half-open (probe in progress)")
		}
		cb.halfOpenActive = true
		return cb.recordOutcome, nil

	default:
		return nil, fmt.Errorf("circuit breaker in unknown state")
	}
}

// recordOutcome is called after a request completes. Must NOT be called under
// the mutex — it acquires it internally.
func (cb *CircuitBreaker) recordOutcome(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		if success {
			cb.failures = 0
		} else {
			cb.failures++
			cb.lastFailure = cb.now()
			if cb.failures >= cb.failThreshold {
				cb.state = StateOpen
			}
		}

	case StateHalfOpen:
		cb.halfOpenActive = false
		if success {
			cb.state = StateClosed
			cb.failures = 0
		} else {
			cb.state = StateOpen
			cb.lastFailure = cb.now()
		}
	}
}

// State returns the current breaker state (for logging/metrics).
func (cb *CircuitBreaker) State() BreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
