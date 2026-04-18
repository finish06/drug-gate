package client

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open and
// the request is rejected without calling the upstream.
var ErrCircuitOpen = errors.New("circuit breaker is open")

type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for upstream calls.
// It tracks consecutive failures and opens the circuit when the threshold
// is exceeded, preventing cascading failures.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            circuitState
	consecutiveFails int
	maxFailures      int
	cooldown         time.Duration
	openedAt         time.Time
}

// NewCircuitBreaker creates a circuit breaker with the given thresholds.
// maxFailures: consecutive failures before opening the circuit.
// cooldown: duration to wait before transitioning from Open to Half-Open.
func NewCircuitBreaker(maxFailures int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       stateClosed,
		maxFailures: maxFailures,
		cooldown:    cooldown,
	}
}

// Execute wraps a function call with circuit breaker protection.
// Returns ErrCircuitOpen if the circuit is open and the cooldown hasn't elapsed.
// In Half-Open state, allows one probe call — success closes the circuit,
// failure re-opens it.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()

	switch cb.state {
	case stateOpen:
		if time.Since(cb.openedAt) < cb.cooldown {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
		// Cooldown elapsed — transition to half-open, allow probe
		cb.state = stateHalfOpen
		cb.mu.Unlock()

	case stateHalfOpen:
		// Only one probe at a time — reject concurrent probes
		cb.mu.Unlock()
		return ErrCircuitOpen

	default: // stateClosed
		cb.mu.Unlock()
	}

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.consecutiveFails++
		if cb.state == stateHalfOpen {
			// Probe failed — re-open
			cb.state = stateOpen
			cb.openedAt = time.Now()
		} else if cb.consecutiveFails >= cb.maxFailures {
			// Threshold exceeded — open circuit
			cb.state = stateOpen
			cb.openedAt = time.Now()
		}
		return err
	}

	// Success — reset
	cb.consecutiveFails = 0
	cb.state = stateClosed
	return nil
}

// IsOpen returns true if the circuit breaker is in the Open state
// and the cooldown has not elapsed.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == stateOpen && time.Since(cb.openedAt) < cb.cooldown
}
