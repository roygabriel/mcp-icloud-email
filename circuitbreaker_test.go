package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestBreaker() (*CircuitBreaker, *time.Time) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cb := NewCircuitBreaker(5, 30*time.Second)
	cb.now = func() time.Time { return now }
	return cb, &now
}

func TestCircuitBreaker_ClosedAllowsRequests(t *testing.T) {
	cb, _ := newTestBreaker()

	done, err := cb.Allow()
	if err != nil {
		t.Fatalf("expected request to be allowed: %v", err)
	}
	done(true)

	if cb.State() != StateClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb, _ := newTestBreaker()

	for i := 0; i < 5; i++ {
		done, err := cb.Allow()
		if err != nil {
			t.Fatalf("request %d should be allowed: %v", i, err)
		}
		done(false)
	}

	if cb.State() != StateOpen {
		t.Errorf("expected open after 5 failures, got %s", cb.State())
	}

	_, err := cb.Allow()
	if err == nil {
		t.Fatal("expected error when circuit is open")
	}
}

func TestCircuitBreaker_SuccessResetFailures(t *testing.T) {
	cb, _ := newTestBreaker()

	// 4 failures then a success — should stay closed.
	for i := 0; i < 4; i++ {
		done, _ := cb.Allow()
		done(false)
	}
	done, _ := cb.Allow()
	done(true)

	if cb.State() != StateClosed {
		t.Errorf("expected closed after success resets counter, got %s", cb.State())
	}

	// Need full 5 failures again to open.
	for i := 0; i < 5; i++ {
		d, _ := cb.Allow()
		d(false)
	}
	if cb.State() != StateOpen {
		t.Error("expected open after 5 consecutive failures")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb, now := newTestBreaker()

	// Open the breaker.
	for i := 0; i < 5; i++ {
		done, _ := cb.Allow()
		done(false)
	}
	if cb.State() != StateOpen {
		t.Fatal("expected open")
	}

	// Before timeout — still open.
	*now = now.Add(29 * time.Second)
	_, err := cb.Allow()
	if err == nil {
		t.Fatal("expected rejection before timeout")
	}

	// After timeout — should transition to half-open.
	*now = now.Add(2 * time.Second) // now at 31s
	done, err := cb.Allow()
	if err != nil {
		t.Fatalf("expected half-open to allow probe: %v", err)
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("expected half-open, got %s", cb.State())
	}

	// Second concurrent request should be rejected (probe in progress).
	_, err = cb.Allow()
	if err == nil {
		t.Fatal("expected rejection while probe is in progress")
	}

	done(true)
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	cb, now := newTestBreaker()

	for i := 0; i < 5; i++ {
		done, _ := cb.Allow()
		done(false)
	}
	*now = now.Add(31 * time.Second)

	done, _ := cb.Allow()
	done(true)

	if cb.State() != StateClosed {
		t.Errorf("expected closed after half-open success, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb, now := newTestBreaker()

	for i := 0; i < 5; i++ {
		done, _ := cb.Allow()
		done(false)
	}
	*now = now.Add(31 * time.Second)

	done, _ := cb.Allow()
	done(false)

	if cb.State() != StateOpen {
		t.Errorf("expected open after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(100, 30*time.Second)
	var allowed atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done, err := cb.Allow()
			if err != nil {
				return
			}
			allowed.Add(1)
			done(true)
		}()
	}
	wg.Wait()

	if allowed.Load() != 200 {
		t.Errorf("expected all 200 allowed when closed, got %d", allowed.Load())
	}
}

func TestBreakerState_String(t *testing.T) {
	tests := []struct {
		state BreakerState
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{BreakerState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
