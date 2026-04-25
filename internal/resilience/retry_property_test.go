package resilience

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 10: Exponential Backoff Retry
// For any retryable operation failure, the system SHALL retry with exponential backoff
// (delay = min(baseDelay * 2^attempt, maxDelay)) plus jitter, up to the configured retry budget.
// **Validates: Requirements 7.1, 7.2, 7.3, 7.4, 7.8**
func TestProperty_ExponentialBackoffRetry(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 10.1: Retry delays follow exponential backoff formula
	properties.Property("retry delays follow exponential backoff formula", prop.ForAll(
		func(attempt int, baseDelayMs int, maxDelayMs int, multiplier float64) bool {
			// Skip invalid inputs
			if attempt < 0 || attempt > 10 {
				return true
			}
			if baseDelayMs < 1 || baseDelayMs > 10000 {
				return true
			}
			if maxDelayMs < baseDelayMs || maxDelayMs > 60000 {
				return true
			}
			if multiplier < 1.0 || multiplier > 5.0 {
				return true
			}

			config := RetryConfig{
				BaseDelay:  time.Duration(baseDelayMs) * time.Millisecond,
				MaxDelay:   time.Duration(maxDelayMs) * time.Millisecond,
				Multiplier: multiplier,
				Jitter:     0.1,
			}

			handler := NewRetryHandler(config).(*retryHandler)
			delay := handler.calculateDelay(attempt)

			// Calculate expected delay without jitter
			expectedDelay := float64(config.BaseDelay) * math.Pow(multiplier, float64(attempt))
			if expectedDelay > float64(config.MaxDelay) {
				expectedDelay = float64(config.MaxDelay)
			}

			// With 10% jitter, delay should be within [0.9 * expected, 1.1 * expected]
			minDelay := time.Duration(expectedDelay * 0.9)
			maxDelay := time.Duration(expectedDelay * 1.1)

			return delay >= minDelay && delay <= maxDelay
		},
		gen.IntRange(0, 10),
		gen.IntRange(10, 1000),
		gen.IntRange(100, 10000),
		gen.Float64Range(1.5, 3.0),
	))

	// Property 10.2: Max delay is enforced
	properties.Property("max delay is enforced", prop.ForAll(
		func(attempt int, baseDelayMs int, maxDelayMs int) bool {
			// Skip invalid inputs
			if attempt < 0 || attempt > 20 {
				return true
			}
			if baseDelayMs < 1 || baseDelayMs > 1000 {
				return true
			}
			if maxDelayMs < baseDelayMs || maxDelayMs > 10000 {
				return true
			}

			config := RetryConfig{
				BaseDelay:  time.Duration(baseDelayMs) * time.Millisecond,
				MaxDelay:   time.Duration(maxDelayMs) * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     0.1,
			}

			handler := NewRetryHandler(config).(*retryHandler)
			delay := handler.calculateDelay(attempt)

			// Delay should never exceed maxDelay * 1.1 (accounting for jitter)
			maxAllowed := time.Duration(float64(config.MaxDelay) * 1.1)
			return delay <= maxAllowed
		},
		gen.IntRange(0, 20),
		gen.IntRange(10, 1000),
		gen.IntRange(100, 10000),
	))

	// Property 10.3: Retry budget is respected
	properties.Property("retry budget is respected", prop.ForAll(
		func(maxAttempts int) bool {
			// Skip invalid inputs
			if maxAttempts < 1 || maxAttempts > 10 {
				return true
			}

			config := RetryConfig{
				MaxAttempts: maxAttempts,
				BaseDelay:   1 * time.Millisecond,
				MaxDelay:    10 * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.1,
			}

			handler := NewRetryHandler(config)
			ctx := context.Background()

			callCount := 0
			err := handler.Do(ctx, func() error {
				callCount++
				return errors.New("persistent failure")
			})

			// Should fail after exactly maxAttempts
			return err != nil && callCount == maxAttempts
		},
		gen.IntRange(1, 10),
	))

	// Property 10.4: Successful operations don't retry
	properties.Property("successful operations don't retry", prop.ForAll(
		func(maxAttempts int) bool {
			// Skip invalid inputs
			if maxAttempts < 1 || maxAttempts > 10 {
				return true
			}

			config := RetryConfig{
				MaxAttempts: maxAttempts,
				BaseDelay:   1 * time.Millisecond,
				MaxDelay:    10 * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.1,
			}

			handler := NewRetryHandler(config)
			ctx := context.Background()

			callCount := 0
			err := handler.Do(ctx, func() error {
				callCount++
				return nil // Success on first attempt
			})

			// Should succeed on first attempt without retries
			return err == nil && callCount == 1
		},
		gen.IntRange(1, 10),
	))

	// Property 10.5: Context cancellation stops retries
	properties.Property("context cancellation stops retries", prop.ForAll(
		func(timeoutMs int) bool {
			// Skip invalid inputs
			if timeoutMs < 1 || timeoutMs > 100 {
				return true
			}

			config := RetryConfig{
				MaxAttempts: 10,
				BaseDelay:   50 * time.Millisecond,
				MaxDelay:    500 * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.1,
			}

			handler := NewRetryHandler(config)
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
			defer cancel()

			callCount := 0
			err := handler.Do(ctx, func() error {
				callCount++
				return errors.New("failure")
			})

			// Should fail due to context cancellation before exhausting retries
			return err != nil && callCount < config.MaxAttempts
		},
		gen.IntRange(10, 100),
	))

	// Property 10.6: DoWithResult returns correct result on success
	properties.Property("DoWithResult returns correct result on success", prop.ForAll(
		func(result int) bool {
			config := RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   1 * time.Millisecond,
				MaxDelay:    10 * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.1,
			}

			handler := NewRetryHandler(config)
			ctx := context.Background()

			actualResult, err := handler.DoWithResult(ctx, func() (interface{}, error) {
				return result, nil
			})

			return err == nil && actualResult == result
		},
		gen.Int(),
	))

	// Property 10.7: DoWithResult retries on failure then succeeds
	properties.Property("DoWithResult retries on failure then succeeds", prop.ForAll(
		func(result int, failuresBeforeSuccess int) bool {
			// Skip invalid inputs
			if failuresBeforeSuccess < 1 || failuresBeforeSuccess > 5 {
				return true
			}

			config := RetryConfig{
				MaxAttempts: failuresBeforeSuccess + 1,
				BaseDelay:   1 * time.Millisecond,
				MaxDelay:    10 * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.1,
			}

			handler := NewRetryHandler(config)
			ctx := context.Background()

			callCount := 0
			actualResult, err := handler.DoWithResult(ctx, func() (interface{}, error) {
				callCount++
				if callCount <= failuresBeforeSuccess {
					return nil, errors.New("temporary failure")
				}
				return result, nil
			})

			return err == nil && actualResult == result && callCount == failuresBeforeSuccess+1
		},
		gen.Int(),
		gen.IntRange(1, 5),
	))

	// Property 10.8: WithMaxAttempts creates new handler with updated config
	properties.Property("WithMaxAttempts creates new handler with updated config", prop.ForAll(
		func(initialAttempts int, newAttempts int) bool {
			// Skip invalid inputs
			if initialAttempts < 1 || initialAttempts > 10 {
				return true
			}
			if newAttempts < 1 || newAttempts > 10 {
				return true
			}

			config := RetryConfig{
				MaxAttempts: initialAttempts,
				BaseDelay:   1 * time.Millisecond,
				MaxDelay:    10 * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.1,
			}

			handler := NewRetryHandler(config)
			newHandler := handler.WithMaxAttempts(newAttempts)

			ctx := context.Background()
			callCount := 0
			_ = newHandler.Do(ctx, func() error {
				callCount++
				return errors.New("failure")
			})

			// New handler should use new max attempts
			return callCount == newAttempts
		},
		gen.IntRange(1, 10),
		gen.IntRange(1, 10),
	))

	// Property 10.9: WithBackoff creates new handler with updated config
	properties.Property("WithBackoff creates new handler with updated config", prop.ForAll(
		func(baseDelayMs int, maxDelayMs int) bool {
			// Skip invalid inputs - ensure meaningful difference between base and max
			if baseDelayMs < 10 || baseDelayMs > 50 {
				return true
			}
			if maxDelayMs <= baseDelayMs*2 || maxDelayMs > 500 {
				return true
			}

			config := RetryConfig{
				MaxAttempts: 3,
				BaseDelay:   10 * time.Millisecond,
				MaxDelay:    100 * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.0, // Remove jitter for predictable timing
			}

			handler := NewRetryHandler(config)
			newHandler := handler.WithBackoff(
				time.Duration(baseDelayMs)*time.Millisecond,
				time.Duration(maxDelayMs)*time.Millisecond,
			)

			updated, ok := newHandler.(*retryHandler)
			if !ok {
				return false
			}
			original, ok := handler.(*retryHandler)
			if !ok {
				return false
			}

			return updated.config.BaseDelay == time.Duration(baseDelayMs)*time.Millisecond &&
				updated.config.MaxDelay == time.Duration(maxDelayMs)*time.Millisecond &&
				original.config.BaseDelay == config.BaseDelay &&
				original.config.MaxDelay == config.MaxDelay
		},
		gen.IntRange(10, 50),
		gen.IntRange(100, 500),
	))

	// Property 10.10: Jitter adds randomness to delays
	properties.Property("jitter adds randomness to delays", prop.ForAll(
		func(attempt int) bool {
			// Skip invalid inputs
			if attempt < 0 || attempt > 5 {
				return true
			}

			config := RetryConfig{
				BaseDelay:  100 * time.Millisecond,
				MaxDelay:   1 * time.Second,
				Multiplier: 2.0,
				Jitter:     0.1,
			}

			handler := NewRetryHandler(config).(*retryHandler)

			// Calculate multiple delays for the same attempt
			delays := make([]time.Duration, 10)
			for i := 0; i < 10; i++ {
				delays[i] = handler.calculateDelay(attempt)
			}

			// Check that not all delays are identical (jitter adds randomness)
			allSame := true
			for i := 1; i < len(delays); i++ {
				if delays[i] != delays[0] {
					allSame = false
					break
				}
			}

			// With jitter, delays should vary
			return !allSame
		},
		gen.IntRange(0, 5),
	))

	// Property 10.11: Total retry duration is bounded
	properties.Property("total retry duration is bounded", prop.ForAll(
		func(maxAttempts int, baseDelayMs int, maxDelayMs int) bool {
			// Skip invalid inputs - use reasonable ranges
			if maxAttempts < 2 || maxAttempts > 4 {
				return true
			}
			if baseDelayMs < 10 || baseDelayMs > 50 {
				return true
			}
			if maxDelayMs <= baseDelayMs*2 || maxDelayMs > 500 {
				return true
			}

			config := RetryConfig{
				MaxAttempts: maxAttempts,
				BaseDelay:   time.Duration(baseDelayMs) * time.Millisecond,
				MaxDelay:    time.Duration(maxDelayMs) * time.Millisecond,
				Multiplier:  2.0,
				Jitter:      0.1,
			}

			handler := NewRetryHandler(config)
			ctx := context.Background()

			start := time.Now()
			_ = handler.Do(ctx, func() error {
				return errors.New("failure")
			})
			duration := time.Since(start)

			// Calculate maximum possible duration
			// Sum of geometric series: baseDelay * (1 + 2 + 4 + ... + 2^(n-2))
			// With jitter and max delay cap, multiply by 1.5 for safety margin
			maxPossibleDuration := time.Duration(0)
			for i := 0; i < maxAttempts-1; i++ {
				delay := float64(config.BaseDelay) * math.Pow(2.0, float64(i))
				if delay > float64(config.MaxDelay) {
					delay = float64(config.MaxDelay)
				}
				maxPossibleDuration += time.Duration(delay * 1.5)
			}

			// Add extra buffer for test execution overhead
			maxPossibleDuration += 100 * time.Millisecond

			return duration <= maxPossibleDuration
		},
		gen.IntRange(2, 4),
		gen.IntRange(10, 50),
		gen.IntRange(100, 500),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
