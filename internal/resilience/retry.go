package resilience

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryHandler provides automatic retry logic with exponential backoff
type RetryHandler interface {
	Do(ctx context.Context, operation func() error) error
	DoWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error)
	WithMaxAttempts(n int) RetryHandler
	WithBackoff(base, max time.Duration) RetryHandler
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts   int           // Maximum number of retry attempts
	BaseDelay     time.Duration // Base delay for exponential backoff
	MaxDelay      time.Duration // Maximum delay between retries
	Multiplier    float64       // Backoff multiplier (default: 2.0)
	Jitter        float64       // Jitter factor (default: 0.1 = 10%)
	RetryableErrs []error       // Specific errors that trigger retry
}

// DefaultConfig provides sensible defaults for API calls
var DefaultConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Second,
	MaxDelay:    60 * time.Second,
	Multiplier:  2.0,
	Jitter:      0.1,
}

// FileOperationConfig provides defaults for file operations
var FileOperationConfig = RetryConfig{
	MaxAttempts: 5,
	BaseDelay:   500 * time.Millisecond,
	MaxDelay:    30 * time.Second,
	Multiplier:  2.0,
	Jitter:      0.1,
}

// RetryState tracks the state of retry attempts
type RetryState struct {
	Attempt       int
	LastError     error
	LastAttemptAt time.Time
	NextAttemptAt time.Time
	TotalDuration time.Duration
}

// retryHandler implements RetryHandler interface
type retryHandler struct {
	config RetryConfig
}

// NewRetryHandler creates a new retry handler with the given configuration
func NewRetryHandler(config RetryConfig) RetryHandler {
	// Set defaults if not provided
	if config.Multiplier == 0 {
		config.Multiplier = 2.0
	}
	if config.Jitter == 0 {
		config.Jitter = 0.1
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}
	if config.BaseDelay == 0 {
		config.BaseDelay = 1 * time.Second
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 60 * time.Second
	}

	return &retryHandler{
		config: config,
	}
}

// Do executes the operation with retry logic
func (r *retryHandler) Do(ctx context.Context, operation func() error) error {
	_, err := r.DoWithResult(ctx, func() (interface{}, error) {
		return nil, operation()
	})
	return err
}

// DoWithResult executes the operation with retry logic and returns a result
func (r *retryHandler) DoWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	var lastErr error
	startTime := time.Now()

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		// Execute the operation
		result, err := operation()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't sleep after the last attempt
		if attempt < r.config.MaxAttempts-1 {
			delay := r.calculateDelay(attempt)

			// Wait for the delay or context cancellation
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	totalDuration := time.Since(startTime)
	return nil, fmt.Errorf("retry budget exhausted after %d attempts (took %v): %w",
		r.config.MaxAttempts, totalDuration, lastErr)
}

// WithMaxAttempts returns a new RetryHandler with the specified max attempts
func (r *retryHandler) WithMaxAttempts(n int) RetryHandler {
	newConfig := r.config
	newConfig.MaxAttempts = n
	return NewRetryHandler(newConfig)
}

// WithBackoff returns a new RetryHandler with the specified backoff parameters
func (r *retryHandler) WithBackoff(base, max time.Duration) RetryHandler {
	newConfig := r.config
	newConfig.BaseDelay = base
	newConfig.MaxDelay = max
	return NewRetryHandler(newConfig)
}

// calculateDelay computes the delay for the given attempt with exponential backoff and jitter
// Formula: delay = min(baseDelay * multiplier^attempt, maxDelay)
// With jitter: actualDelay = delay * (1 + random(-jitter, +jitter))
func (r *retryHandler) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt))

	// Apply max delay cap
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Apply jitter: random value between -jitter and +jitter
	jitterRange := delay * r.config.Jitter
	jitterValue := (rand.Float64()*2 - 1) * jitterRange // Random between -jitterRange and +jitterRange
	delay += jitterValue

	// Ensure delay is positive
	if delay < 0 {
		delay = float64(r.config.BaseDelay)
	}

	return time.Duration(delay)
}

// isRetryable determines if an error should trigger a retry
func (r *retryHandler) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check against configured retryable errors
	for _, retryableErr := range r.config.RetryableErrs {
		if err == retryableErr {
			return true
		}
	}

	// Default retryable error detection
	// This is a simple implementation - in production, you'd check for:
	// - Network timeouts
	// - Connection refused
	// - 429 Rate Limit
	// - 503 Service Unavailable
	// - Temporary DNS failures
	// For now, we'll consider all errors retryable unless explicitly marked non-retryable
	return true
}
