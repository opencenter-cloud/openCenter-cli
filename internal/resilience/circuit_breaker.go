package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed means normal operation - requests pass through
	StateClosed State = iota
	// StateOpen means failing fast - requests are rejected immediately
	StateOpen
	// StateHalfOpen means testing recovery - limited requests allowed
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
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

// CircuitBreaker prevents cascading failures by failing fast when a service is down
type CircuitBreaker interface {
	Call(ctx context.Context, operation func() error) error
	GetState() State
	Reset()
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold  int           // Open after N consecutive failures (default: 5)
	SuccessThreshold  int           // Close after N consecutive successes in half-open (default: 2)
	Timeout           time.Duration // Duration to stay open before half-open (default: 60s)
	HalfOpenRequests  int           // Max concurrent requests in half-open state (default: 1)
}

// DefaultCircuitBreakerConfig provides sensible defaults
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
	FailureThreshold:  5,
	SuccessThreshold:  2,
	Timeout:           60 * time.Second,
	HalfOpenRequests:  1,
}

// CircuitBreakerState tracks the internal state of the circuit breaker
type CircuitBreakerState struct {
	State            State
	FailureCount     int
	SuccessCount     int
	LastFailureAt    time.Time
	LastSuccessAt    time.Time
	OpenedAt         time.Time
	HalfOpenRequests int
}

// circuitBreaker implements CircuitBreaker interface
type circuitBreaker struct {
	config CircuitBreakerConfig
	state  CircuitBreakerState
	mu     sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) CircuitBreaker {
	// Set defaults if not provided
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.HalfOpenRequests == 0 {
		config.HalfOpenRequests = 1
	}

	return &circuitBreaker{
		config: config,
		state: CircuitBreakerState{
			State: StateClosed,
		},
	}
}

// Call executes the operation through the circuit breaker
func (cb *circuitBreaker) Call(ctx context.Context, operation func() error) error {
	// Check if we can proceed
	if err := cb.beforeCall(); err != nil {
		return err
	}

	// Execute the operation
	err := operation()

	// Record the result
	cb.afterCall(err)

	return err
}

// GetState returns the current circuit breaker state
func (cb *circuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state.State
}

// Reset manually resets the circuit breaker to closed state
func (cb *circuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitBreakerState{
		State: StateClosed,
	}
}

// beforeCall checks if the operation can proceed based on current state
func (cb *circuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state.State {
	case StateClosed:
		// Normal operation - allow the call
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.state.OpenedAt) >= cb.config.Timeout {
			// Transition to half-open
			cb.state.State = StateHalfOpen
			cb.state.SuccessCount = 0
			cb.state.FailureCount = 0
			cb.state.HalfOpenRequests = 1 // Count this request
			return nil
		}
		// Still open - reject the call
		return fmt.Errorf("circuit breaker is open (opened at %v, timeout %v)",
			cb.state.OpenedAt, cb.config.Timeout)

	case StateHalfOpen:
		// Check if we can allow more requests
		if cb.state.HalfOpenRequests >= cb.config.HalfOpenRequests {
			return fmt.Errorf("circuit breaker is half-open with max concurrent requests reached")
		}
		cb.state.HalfOpenRequests++
		return nil

	default:
		return fmt.Errorf("circuit breaker in unknown state: %v", cb.state.State)
	}
}

// afterCall records the result and updates state accordingly
func (cb *circuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onSuccess handles a successful operation
func (cb *circuitBreaker) onSuccess() {
	cb.state.LastSuccessAt = time.Now()

	switch cb.state.State {
	case StateClosed:
		// Reset failure count on success
		cb.state.FailureCount = 0

	case StateHalfOpen:
		cb.state.HalfOpenRequests--
		cb.state.SuccessCount++
		cb.state.FailureCount = 0

		// Check if we've had enough successes to close
		if cb.state.SuccessCount >= cb.config.SuccessThreshold {
			cb.state.State = StateClosed
			cb.state.SuccessCount = 0
			cb.state.FailureCount = 0
			cb.state.HalfOpenRequests = 0
		}

	case StateOpen:
		// This shouldn't happen, but reset if it does
		cb.state.FailureCount = 0
	}
}

// onFailure handles a failed operation
func (cb *circuitBreaker) onFailure() {
	cb.state.LastFailureAt = time.Now()

	switch cb.state.State {
	case StateClosed:
		cb.state.FailureCount++
		cb.state.SuccessCount = 0

		// Check if we've hit the failure threshold
		if cb.state.FailureCount >= cb.config.FailureThreshold {
			cb.state.State = StateOpen
			cb.state.OpenedAt = time.Now()
		}

	case StateHalfOpen:
		// Any failure in half-open immediately opens the circuit
		cb.state.State = StateOpen
		cb.state.OpenedAt = time.Now()
		cb.state.FailureCount = cb.config.FailureThreshold
		cb.state.SuccessCount = 0
		cb.state.HalfOpenRequests = 0

	case StateOpen:
		// Already open, just increment counter
		cb.state.FailureCount++
	}
}
