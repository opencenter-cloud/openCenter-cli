package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 11: Circuit Breaker State Transitions
// For any circuit breaker, after N consecutive failures it SHALL transition to Open state,
// after timeout it SHALL transition to HalfOpen, and after M consecutive successes it SHALL
// transition to Closed.
// **Validates: Requirements 7.5, 7.6**
func TestProperty_CircuitBreakerStateTransitions(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property 11.1: Circuit opens after N consecutive failures
	properties.Property("circuit opens after N consecutive failures", prop.ForAll(
		func(failureThreshold int) bool {
			// Skip invalid inputs
			if failureThreshold < 1 || failureThreshold > 10 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  failureThreshold,
				SuccessThreshold:  2,
				Timeout:           100 * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Fail exactly failureThreshold times
			for i := 0; i < failureThreshold; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Circuit should be open
			return cb.GetState() == StateOpen
		},
		gen.IntRange(1, 10),
	))

	// Property 11.2: Circuit stays closed with fewer than N failures
	properties.Property("circuit stays closed with fewer than N failures", prop.ForAll(
		func(failureThreshold int, actualFailures int) bool {
			// Skip invalid inputs
			if failureThreshold < 2 || failureThreshold > 10 {
				return true
			}
			if actualFailures < 1 || actualFailures >= failureThreshold {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  failureThreshold,
				SuccessThreshold:  2,
				Timeout:           100 * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Fail fewer times than threshold
			for i := 0; i < actualFailures; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Circuit should still be closed
			return cb.GetState() == StateClosed
		},
		gen.IntRange(2, 10),
		gen.IntRange(1, 9),
	))

	// Property 11.3: Open circuit transitions to half-open after timeout
	properties.Property("open circuit transitions to half-open after timeout", prop.ForAll(
		func(timeoutMs int) bool {
			// Skip invalid inputs
			if timeoutMs < 10 || timeoutMs > 200 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  2,
				SuccessThreshold:  2,
				Timeout:           time.Duration(timeoutMs) * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Open the circuit
			for i := 0; i < 2; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			if cb.GetState() != StateOpen {
				return false
			}

			// Wait for timeout plus a small buffer
			time.Sleep(time.Duration(timeoutMs+50) * time.Millisecond)

			// Make a call to trigger state transition
			cb.Call(ctx, func() error {
				return nil
			})

			// Should be in half-open or closed state
			state := cb.GetState()
			return state == StateHalfOpen || state == StateClosed
		},
		gen.IntRange(10, 200),
	))

	// Property 11.4: Half-open circuit closes after M successes
	properties.Property("half-open circuit closes after M successes", prop.ForAll(
		func(successThreshold int) bool {
			// Skip invalid inputs
			if successThreshold < 1 || successThreshold > 5 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  2,
				SuccessThreshold:  successThreshold,
				Timeout:           50 * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Open the circuit
			for i := 0; i < 2; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Wait for timeout
			time.Sleep(100 * time.Millisecond)

			// Succeed exactly successThreshold times
			for i := 0; i < successThreshold; i++ {
				cb.Call(ctx, func() error {
					return nil
				})
			}

			// Circuit should be closed
			return cb.GetState() == StateClosed
		},
		gen.IntRange(1, 5),
	))

	// Property 11.5: Half-open circuit reopens on any failure
	properties.Property("half-open circuit reopens on any failure", prop.ForAll(
		func(successesBeforeFailure int) bool {
			// Skip invalid inputs
			if successesBeforeFailure < 0 || successesBeforeFailure > 3 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  2,
				SuccessThreshold:  5, // High threshold so we stay in half-open
				Timeout:           50 * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Open the circuit
			for i := 0; i < 2; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Wait for timeout
			time.Sleep(100 * time.Millisecond)

			// Succeed a few times
			for i := 0; i < successesBeforeFailure; i++ {
				cb.Call(ctx, func() error {
					return nil
				})
			}

			// Then fail once
			cb.Call(ctx, func() error {
				return errors.New("failure")
			})

			// Circuit should be open again
			return cb.GetState() == StateOpen
		},
		gen.IntRange(0, 3),
	))

	// Property 11.6: Success in closed state resets failure count
	properties.Property("success in closed state resets failure count", prop.ForAll(
		func(failuresBeforeSuccess int) bool {
			// Skip invalid inputs
			if failuresBeforeSuccess < 1 || failuresBeforeSuccess > 3 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  5,
				SuccessThreshold:  2,
				Timeout:           100 * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config).(*circuitBreaker)
			ctx := context.Background()

			// Fail a few times (not enough to open)
			for i := 0; i < failuresBeforeSuccess; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Succeed once
			cb.Call(ctx, func() error {
				return nil
			})

			// Check failure count is reset
			cb.mu.RLock()
			failureCount := cb.state.FailureCount
			cb.mu.RUnlock()

			return failureCount == 0 && cb.GetState() == StateClosed
		},
		gen.IntRange(1, 3),
	))

	// Property 11.7: Reset always returns circuit to closed state
	properties.Property("reset always returns circuit to closed state", prop.ForAll(
		func(failures int) bool {
			// Skip invalid inputs
			if failures < 1 || failures > 10 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  3,
				SuccessThreshold:  2,
				Timeout:           100 * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Fail enough times to potentially open the circuit
			for i := 0; i < failures; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Reset the circuit
			cb.Reset()

			// Should be closed regardless of previous state
			return cb.GetState() == StateClosed
		},
		gen.IntRange(1, 10),
	))

	// Property 11.8: Open circuit rejects calls without executing operation
	properties.Property("open circuit rejects calls without executing operation", prop.ForAll(
		func(failureThreshold int) bool {
			// Skip invalid inputs
			if failureThreshold < 1 || failureThreshold > 5 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  failureThreshold,
				SuccessThreshold:  2,
				Timeout:           1 * time.Second, // Long timeout
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Open the circuit
			for i := 0; i < failureThreshold; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Try to call with open circuit
			executed := false
			err := cb.Call(ctx, func() error {
				executed = true
				return nil
			})

			// Operation should not execute and should return error
			return !executed && err != nil
		},
		gen.IntRange(1, 5),
	))

	// Property 11.9: Half-open circuit limits concurrent requests
	properties.Property("half-open circuit limits concurrent requests", prop.ForAll(
		func(maxRequests int) bool {
			// Skip invalid inputs
			if maxRequests < 1 || maxRequests > 3 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  2,
				SuccessThreshold:  5, // High threshold to stay in half-open
				Timeout:           50 * time.Millisecond,
				HalfOpenRequests:  maxRequests,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Open the circuit
			for i := 0; i < 2; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Wait for timeout
			time.Sleep(100 * time.Millisecond)

			// Start maxRequests slow operations
			started := make(chan struct{}, maxRequests)
			done := make(chan error, maxRequests)
			for i := 0; i < maxRequests; i++ {
				go func() {
					err := cb.Call(ctx, func() error {
						started <- struct{}{}
						time.Sleep(100 * time.Millisecond)
						return nil
					})
					done <- err
				}()
			}

			// Wait for all to start
			for i := 0; i < maxRequests; i++ {
				<-started
			}

			// Try one more request (should be rejected)
			extraExecuted := false
			extraErr := cb.Call(ctx, func() error {
				extraExecuted = true
				return nil
			})

			// Wait for all operations to complete
			for i := 0; i < maxRequests; i++ {
				<-done
			}

			// Extra request should be rejected
			return !extraExecuted && extraErr != nil
		},
		gen.IntRange(1, 3),
	))

	// Property 11.10: State transitions are monotonic within a failure sequence
	properties.Property("state transitions are monotonic within a failure sequence", prop.ForAll(
		func(failureThreshold int, totalFailures int) bool {
			// Skip invalid inputs
			if failureThreshold < 2 || failureThreshold > 5 {
				return true
			}
			if totalFailures < failureThreshold || totalFailures > failureThreshold+3 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  failureThreshold,
				SuccessThreshold:  2,
				Timeout:           1 * time.Second,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			states := make([]State, 0, totalFailures+1)
			states = append(states, cb.GetState())

			// Fail multiple times
			for i := 0; i < totalFailures; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
				states = append(states, cb.GetState())
			}

			// Check state progression: Closed -> Open (never goes back)
			sawOpen := false
			for _, state := range states {
				if state == StateOpen {
					sawOpen = true
				}
				if sawOpen && state == StateClosed {
					return false // Invalid: went from Open back to Closed
				}
			}

			return true
		},
		gen.IntRange(2, 5),
		gen.IntRange(2, 8),
	))

	// Property 11.11: Timeout duration affects transition timing
	properties.Property("timeout duration affects transition timing", prop.ForAll(
		func(timeoutMs int) bool {
			// Skip invalid inputs
			if timeoutMs < 50 || timeoutMs > 200 {
				return true
			}

			config := CircuitBreakerConfig{
				FailureThreshold:  2,
				SuccessThreshold:  2,
				Timeout:           time.Duration(timeoutMs) * time.Millisecond,
				HalfOpenRequests:  1,
			}

			cb := NewCircuitBreaker(config)
			ctx := context.Background()

			// Open the circuit
			for i := 0; i < 2; i++ {
				cb.Call(ctx, func() error {
					return errors.New("failure")
				})
			}

			// Wait for less than timeout
			time.Sleep(time.Duration(timeoutMs/2) * time.Millisecond)

			// Try to call - should still be rejected
			executed := false
			err := cb.Call(ctx, func() error {
				executed = true
				return nil
			})

			// Should still be open
			return !executed && err != nil && cb.GetState() == StateOpen
		},
		gen.IntRange(50, 200),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
