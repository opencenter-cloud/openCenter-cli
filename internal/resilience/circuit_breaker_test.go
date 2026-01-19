package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	tests := []struct {
		name   string
		config CircuitBreakerConfig
		want   CircuitBreakerConfig
	}{
		{
			name:   "default config",
			config: CircuitBreakerConfig{},
			want:   DefaultCircuitBreakerConfig,
		},
		{
			name: "custom config",
			config: CircuitBreakerConfig{
				FailureThreshold:  3,
				SuccessThreshold:  1,
				Timeout:           30 * time.Second,
				HalfOpenRequests:  2,
			},
			want: CircuitBreakerConfig{
				FailureThreshold:  3,
				SuccessThreshold:  1,
				Timeout:           30 * time.Second,
				HalfOpenRequests:  2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker(tt.config).(*circuitBreaker)
			if cb.config.FailureThreshold != tt.want.FailureThreshold {
				t.Errorf("FailureThreshold = %v, want %v", cb.config.FailureThreshold, tt.want.FailureThreshold)
			}
			if cb.config.SuccessThreshold != tt.want.SuccessThreshold {
				t.Errorf("SuccessThreshold = %v, want %v", cb.config.SuccessThreshold, tt.want.SuccessThreshold)
			}
			if cb.config.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %v, want %v", cb.config.Timeout, tt.want.Timeout)
			}
			if cb.config.HalfOpenRequests != tt.want.HalfOpenRequests {
				t.Errorf("HalfOpenRequests = %v, want %v", cb.config.HalfOpenRequests, tt.want.HalfOpenRequests)
			}
		})
	}
}

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig)
	
	if state := cb.GetState(); state != StateClosed {
		t.Errorf("Initial state = %v, want %v", state, StateClosed)
	}
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:  3,
		SuccessThreshold:  2,
		Timeout:           1 * time.Second,
		HalfOpenRequests:  1,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Verify initial state
	if state := cb.GetState(); state != StateClosed {
		t.Fatalf("Initial state = %v, want %v", state, StateClosed)
	}

	// Fail 3 times to open the circuit
	failingOp := func() error {
		return errors.New("operation failed")
	}

	for i := 0; i < 3; i++ {
		err := cb.Call(ctx, failingOp)
		if err == nil {
			t.Errorf("Call %d: expected error, got nil", i+1)
		}
	}

	// Circuit should now be open
	if state := cb.GetState(); state != StateOpen {
		t.Errorf("State after %d failures = %v, want %v", 3, state, StateOpen)
	}

	// Next call should fail immediately without executing operation
	executed := false
	err := cb.Call(ctx, func() error {
		executed = true
		return nil
	})
	if err == nil {
		t.Error("Call on open circuit: expected error, got nil")
	}
	if executed {
		t.Error("Operation was executed on open circuit")
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  2,
		Timeout:           100 * time.Millisecond,
		HalfOpenRequests:  1,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	failingOp := func() error {
		return errors.New("operation failed")
	}
	for i := 0; i < 2; i++ {
		cb.Call(ctx, failingOp)
	}

	if state := cb.GetState(); state != StateOpen {
		t.Fatalf("State after failures = %v, want %v", state, StateOpen)
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next call should transition to half-open
	successOp := func() error {
		return nil
	}
	err := cb.Call(ctx, successOp)
	if err != nil {
		t.Errorf("Call after timeout: unexpected error: %v", err)
	}

	// State should be half-open (or closed if success threshold is 1)
	state := cb.GetState()
	if state != StateHalfOpen && state != StateClosed {
		t.Errorf("State after timeout = %v, want %v or %v", state, StateHalfOpen, StateClosed)
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  2,
		Timeout:           100 * time.Millisecond,
		HalfOpenRequests:  1,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	failingOp := func() error {
		return errors.New("operation failed")
	}
	for i := 0; i < 2; i++ {
		cb.Call(ctx, failingOp)
	}

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Succeed enough times to close the circuit
	successOp := func() error {
		return nil
	}
	for i := 0; i < 2; i++ {
		err := cb.Call(ctx, successOp)
		if err != nil {
			t.Errorf("Call %d: unexpected error: %v", i+1, err)
		}
	}

	// Circuit should now be closed
	if state := cb.GetState(); state != StateClosed {
		t.Errorf("State after successes = %v, want %v", state, StateClosed)
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  2,
		Timeout:           100 * time.Millisecond,
		HalfOpenRequests:  1,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	failingOp := func() error {
		return errors.New("operation failed")
	}
	for i := 0; i < 2; i++ {
		cb.Call(ctx, failingOp)
	}

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Fail once in half-open state
	err := cb.Call(ctx, failingOp)
	if err == nil {
		t.Error("Expected error from failing operation, got nil")
	}

	// Circuit should be open again
	if state := cb.GetState(); state != StateOpen {
		t.Errorf("State after failure in half-open = %v, want %v", state, StateOpen)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  2,
		Timeout:           1 * time.Second,
		HalfOpenRequests:  1,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	failingOp := func() error {
		return errors.New("operation failed")
	}
	for i := 0; i < 2; i++ {
		cb.Call(ctx, failingOp)
	}

	if state := cb.GetState(); state != StateOpen {
		t.Fatalf("State after failures = %v, want %v", state, StateOpen)
	}

	// Reset the circuit
	cb.Reset()

	// Circuit should be closed
	if state := cb.GetState(); state != StateClosed {
		t.Errorf("State after reset = %v, want %v", state, StateClosed)
	}

	// Should be able to execute operations
	successOp := func() error {
		return nil
	}
	err := cb.Call(ctx, successOp)
	if err != nil {
		t.Errorf("Call after reset: unexpected error: %v", err)
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:  3,
		SuccessThreshold:  2,
		Timeout:           1 * time.Second,
		HalfOpenRequests:  1,
	}
	cb := NewCircuitBreaker(config).(*circuitBreaker)
	ctx := context.Background()

	// Fail twice (not enough to open)
	failingOp := func() error {
		return errors.New("operation failed")
	}
	for i := 0; i < 2; i++ {
		cb.Call(ctx, failingOp)
	}

	// Succeed once
	successOp := func() error {
		return nil
	}
	cb.Call(ctx, successOp)

	// Failure count should be reset
	cb.mu.RLock()
	failureCount := cb.state.FailureCount
	cb.mu.RUnlock()

	if failureCount != 0 {
		t.Errorf("Failure count after success = %d, want 0", failureCount)
	}

	// Circuit should still be closed
	if state := cb.GetState(); state != StateClosed {
		t.Errorf("State = %v, want %v", state, StateClosed)
	}
}

func TestCircuitBreaker_HalfOpenMaxRequests(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold:  2,
		SuccessThreshold:  3, // Need 3 successes to close, so we stay in half-open
		Timeout:           100 * time.Millisecond,
		HalfOpenRequests:  1,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	failingOp := func() error {
		return errors.New("operation failed")
	}
	for i := 0; i < 2; i++ {
		cb.Call(ctx, failingOp)
	}

	if state := cb.GetState(); state != StateOpen {
		t.Fatalf("State after failures = %v, want %v", state, StateOpen)
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Start a slow operation in half-open state
	started := make(chan struct{})
	slowOp := func() error {
		close(started)
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Start first operation (should succeed and transition to half-open)
	done := make(chan error, 1)
	go func() {
		done <- cb.Call(ctx, slowOp)
	}()

	// Wait for operation to actually start
	<-started

	// Verify we're in half-open state
	state := cb.GetState()
	if state != StateHalfOpen {
		t.Logf("State during concurrent test = %v, expected %v", state, StateHalfOpen)
	}

	// Try second operation (should be rejected due to max concurrent requests)
	err := cb.Call(ctx, func() error {
		t.Log("Second operation executed")
		return nil
	})
	if err == nil {
		t.Error("Second concurrent call in half-open: expected error, got nil")
	} else {
		t.Logf("Second call rejected with error: %v", err)
	}

	// Wait for first operation to complete
	firstErr := <-done
	if firstErr != nil {
		t.Errorf("First operation failed: %v", firstErr)
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
