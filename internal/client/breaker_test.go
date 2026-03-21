package client

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errSimulated = errors.New("simulated upstream failure")

// AC-001: Circuit starts Closed — requests pass through.
func TestCircuitBreaker_AC001_StartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(10, 30*time.Second)

	called := false
	err := cb.Execute(func() error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("function should be called when circuit is closed")
	}
}

// AC-002: Circuit opens after 10 consecutive failures.
func TestCircuitBreaker_AC002_OpensAfter10Failures(t *testing.T) {
	cb := NewCircuitBreaker(10, 30*time.Second)

	for i := 0; i < 10; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}

	// 11th call should be rejected by circuit
	err := cb.Execute(func() error {
		t.Error("function should not be called when circuit is open")
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

// AC-003: Open circuit rejects with ErrCircuitOpen.
func TestCircuitBreaker_AC003_OpenRejects(t *testing.T) {
	cb := NewCircuitBreaker(10, 30*time.Second)

	// Trip the circuit
	for i := 0; i < 10; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}

	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

// AC-004: Circuit transitions to Half-Open after cooldown.
func TestCircuitBreaker_AC004_HalfOpenAfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker(10, 50*time.Millisecond) // short cooldown for test

	// Trip the circuit
	for i := 0; i < 10; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)

	// Should allow one probe request (half-open)
	called := false
	err := cb.Execute(func() error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("expected half-open to allow probe, got: %v", err)
	}
	if !called {
		t.Error("probe function should be called in half-open state")
	}
}

// AC-005: Half-Open success → Closed.
func TestCircuitBreaker_AC005_HalfOpenSuccessCloses(t *testing.T) {
	cb := NewCircuitBreaker(10, 50*time.Millisecond)

	// Trip then wait for half-open
	for i := 0; i < 10; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}
	time.Sleep(60 * time.Millisecond)

	// Probe succeeds
	_ = cb.Execute(func() error { return nil })

	// Circuit should be closed — next call should pass
	called := false
	err := cb.Execute(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("circuit should be closed after half-open success: %v", err)
	}
	if !called {
		t.Error("function should be called after circuit closes")
	}
}

// AC-006: Half-Open failure → re-Open.
func TestCircuitBreaker_AC006_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(10, 50*time.Millisecond)

	// Trip then wait for half-open
	for i := 0; i < 10; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}
	time.Sleep(60 * time.Millisecond)

	// Probe fails
	_ = cb.Execute(func() error { return errSimulated })

	// Circuit should be open again — next call rejected
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("circuit should be open after half-open failure, got: %v", err)
	}
}

// AC-007: Success resets failure counter.
func TestCircuitBreaker_AC007_SuccessResetsCounter(t *testing.T) {
	cb := NewCircuitBreaker(10, 30*time.Second)

	// 9 failures
	for i := 0; i < 9; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}

	// 1 success — resets counter
	_ = cb.Execute(func() error { return nil })

	// 9 more failures — should NOT trip (counter was reset)
	for i := 0; i < 9; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}

	// Should still be closed
	called := false
	err := cb.Execute(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("circuit should still be closed: %v", err)
	}
	if !called {
		t.Error("function should be called — circuit not tripped")
	}
}

// AC-008: Thread-safe concurrent access.
func TestCircuitBreaker_AC008_ConcurrentSafe(t *testing.T) {
	cb := NewCircuitBreaker(10, 50*time.Millisecond)

	var wg sync.WaitGroup
	var successCount atomic.Int64

	// 50 goroutines hitting the breaker concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Execute(func() error { return nil })
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if successCount.Load() != 50 {
		t.Errorf("expected 50 successes, got %d", successCount.Load())
	}
}

// Test: IsOpen reports circuit state.
func TestCircuitBreaker_IsOpen(t *testing.T) {
	cb := NewCircuitBreaker(10, 30*time.Second)

	if cb.IsOpen() {
		t.Error("circuit should start closed")
	}

	for i := 0; i < 10; i++ {
		_ = cb.Execute(func() error { return errSimulated })
	}

	if !cb.IsOpen() {
		t.Error("circuit should be open after 10 failures")
	}
}
