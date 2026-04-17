package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryHandler_Do_Success(t *testing.T) {
	handler := NewRetryHandler(DefaultConfig)
	ctx := context.Background()

	callCount := 0
	err := handler.Do(ctx, func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestRetryHandler_Do_SuccessAfterRetry(t *testing.T) {
	handler := NewRetryHandler(RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      0.1,
	})
	ctx := context.Background()

	callCount := 0
	err := handler.Do(ctx, func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestRetryHandler_Do_ExhaustedRetries(t *testing.T) {
	handler := NewRetryHandler(RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      0.1,
	})
	ctx := context.Background()

	callCount := 0
	testErr := errors.New("persistent failure")
	err := handler.Do(ctx, func() error {
		callCount++
		return testErr
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if !errors.Is(err, testErr) {
		t.Errorf("expected error to wrap testErr, got %v", err)
	}
}

func TestRetryHandler_Do_ContextCancellation(t *testing.T) {
	handler := NewRetryHandler(RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callCount := 0
	err := handler.Do(ctx, func() error {
		callCount++
		return errors.New("failure")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount > 2 {
		t.Errorf("expected at most 2 calls before cancellation, got %d", callCount)
	}
}

func TestRetryHandler_DoWithResult_Success(t *testing.T) {
	handler := NewRetryHandler(DefaultConfig)
	ctx := context.Background()

	expectedResult := "success"
	result, err := handler.DoWithResult(ctx, func() (interface{}, error) {
		return expectedResult, nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != expectedResult {
		t.Errorf("expected result %v, got %v", expectedResult, result)
	}
}

func TestRetryHandler_DoWithResult_SuccessAfterRetry(t *testing.T) {
	handler := NewRetryHandler(RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      0.1,
	})
	ctx := context.Background()

	callCount := 0
	expectedResult := 42
	result, err := handler.DoWithResult(ctx, func() (interface{}, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("temporary failure")
		}
		return expectedResult, nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != expectedResult {
		t.Errorf("expected result %v, got %v", expectedResult, result)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestRetryHandler_WithMaxAttempts(t *testing.T) {
	handler := NewRetryHandler(DefaultConfig)
	newHandler := handler.WithMaxAttempts(5)

	ctx := context.Background()
	callCount := 0
	err := newHandler.Do(ctx, func() error {
		callCount++
		return errors.New("failure")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount != 5 {
		t.Errorf("expected 5 calls, got %d", callCount)
	}
}

func TestRetryHandler_WithBackoff(t *testing.T) {
	handler := NewRetryHandler(DefaultConfig)
	newHandler := handler.WithBackoff(50*time.Millisecond, 200*time.Millisecond)

	ctx := context.Background()
	start := time.Now()
	callCount := 0
	_ = newHandler.Do(ctx, func() error {
		callCount++
		return errors.New("failure")
	})
	duration := time.Since(start)

	// With 3 attempts and base delay 50ms, we expect at least 50ms + 100ms = 150ms
	// (first retry after 50ms, second after 100ms)
	if duration < 100*time.Millisecond {
		t.Errorf("expected at least 100ms duration, got %v", duration)
	}
}

func TestRetryHandler_ExplicitZeroJitterDisablesJitter(t *testing.T) {
	handler := NewRetryHandler(RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      0.0,
	}).(*retryHandler)

	for range 5 {
		delay := handler.calculateDelay(1)
		if delay != 100*time.Millisecond {
			t.Fatalf("expected deterministic 100ms delay with jitter disabled, got %v", delay)
		}
	}
}

func TestRetryHandler_ExponentialBackoff(t *testing.T) {
	handler := NewRetryHandler(RetryConfig{
		MaxAttempts: 4,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.0, // No jitter for predictable timing
	})

	ctx := context.Background()
	start := time.Now()
	callCount := 0
	_ = handler.Do(ctx, func() error {
		callCount++
		return errors.New("failure")
	})
	duration := time.Since(start)

	// Expected delays: 100ms, 200ms, 400ms = 700ms total
	// Allow some tolerance for execution time
	if duration < 650*time.Millisecond || duration > 850*time.Millisecond {
		t.Errorf("expected duration around 700ms, got %v", duration)
	}
	if callCount != 4 {
		t.Errorf("expected 4 calls, got %d", callCount)
	}
}

func TestRetryHandler_MaxDelayEnforced(t *testing.T) {
	handler := NewRetryHandler(RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      0.0,
	})

	ctx := context.Background()
	start := time.Now()
	_ = handler.Do(ctx, func() error {
		return errors.New("failure")
	})
	duration := time.Since(start)

	// Expected delays: 100ms, 200ms, 200ms, 200ms = 700ms total
	// (third and fourth delays capped at maxDelay)
	if duration < 650*time.Millisecond || duration > 850*time.Millisecond {
		t.Errorf("expected duration around 700ms, got %v", duration)
	}
}

func TestDefaultConfig(t *testing.T) {
	if DefaultConfig.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", DefaultConfig.MaxAttempts)
	}
	if DefaultConfig.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay=1s, got %v", DefaultConfig.BaseDelay)
	}
	if DefaultConfig.MaxDelay != 60*time.Second {
		t.Errorf("expected MaxDelay=60s, got %v", DefaultConfig.MaxDelay)
	}
	if DefaultConfig.Multiplier != 2.0 {
		t.Errorf("expected Multiplier=2.0, got %f", DefaultConfig.Multiplier)
	}
	if DefaultConfig.Jitter != 0.1 {
		t.Errorf("expected Jitter=0.1, got %f", DefaultConfig.Jitter)
	}
}

func TestFileOperationConfig(t *testing.T) {
	if FileOperationConfig.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", FileOperationConfig.MaxAttempts)
	}
	if FileOperationConfig.BaseDelay != 500*time.Millisecond {
		t.Errorf("expected BaseDelay=500ms, got %v", FileOperationConfig.BaseDelay)
	}
	if FileOperationConfig.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay=30s, got %v", FileOperationConfig.MaxDelay)
	}
}
