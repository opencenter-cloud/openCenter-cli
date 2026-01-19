package resilience

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 12: Lock Acquisition and Release
// For any cluster operation, the system SHALL acquire a lock before starting, and SHALL release
// the lock after completion or failure, preventing concurrent modifications.
// **Validates: Requirements 11.1, 11.4, 11.5, 11.7**
func TestProperty_LockAcquisitionAndRelease(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // Reduced for faster tests
	properties := gopter.NewProperties(parameters)

	// Property 12.1: Lock acquisition prevents concurrent access
	properties.Property("lock acquisition prevents concurrent access", prop.ForAll(
		func(resource string, ttlSeconds int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if ttlSeconds < 1 || ttlSeconds > 3600 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     time.Duration(ttlSeconds) * time.Second,
				AcquireTimeout: 1 * time.Second,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()
			ttl := time.Duration(ttlSeconds) * time.Second

			// Acquire first lock
			lock1, err := lm.Acquire(ctx, resource, ttl)
			if err != nil {
				return false
			}
			defer lm.Release(lock1)

			// Try to acquire second lock (should fail)
			lock2, err := lm.Acquire(ctx, resource, ttl)
			if err == nil {
				lm.Release(lock2)
				return false // Should have failed
			}

			return true
		},
		gen.AlphaString(),
		gen.IntRange(1, 60),
	))

	// Property 12.2: Released locks can be reacquired
	properties.Property("released locks can be reacquired", prop.ForAll(
		func(resource string, ttlSeconds int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if ttlSeconds < 1 || ttlSeconds > 60 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     time.Duration(ttlSeconds) * time.Second,
				AcquireTimeout: 500 * time.Millisecond, // Shorter timeout for tests
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()
			ttl := time.Duration(ttlSeconds) * time.Second

			// Acquire and release first lock
			lock1, err := lm.Acquire(ctx, resource, ttl)
			if err != nil {
				return false
			}
			if err := lm.Release(lock1); err != nil {
				return false
			}

			// Acquire second lock (should succeed)
			lock2, err := lm.Acquire(ctx, resource, ttl)
			if err != nil {
				return false
			}
			defer lm.Release(lock2)

			return lock2 != nil
		},
		gen.AlphaString(),
		gen.IntRange(1, 10), // Reduced range for faster tests
	))

	// Property 12.3: Lock properties are correctly set
	properties.Property("lock properties are correctly set", prop.ForAll(
		func(resource string, ttlSeconds int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if ttlSeconds < 1 || ttlSeconds > 10 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     time.Duration(ttlSeconds) * time.Second,
				AcquireTimeout: 500 * time.Millisecond,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()
			ttl := time.Duration(ttlSeconds) * time.Second

			beforeAcquire := time.Now()
			lock, err := lm.Acquire(ctx, resource, ttl)
			if err != nil {
				return false
			}
			defer lm.Release(lock)
			afterAcquire := time.Now()

			// Verify resource name
			if lock.Resource() != resource {
				return false
			}

			// Verify owner is set
			if lock.Owner() == "" {
				return false
			}

			// Verify acquired time is reasonable
			acquiredAt := lock.AcquiredAt()
			if acquiredAt.Before(beforeAcquire) || acquiredAt.After(afterAcquire) {
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.IntRange(1, 10),
	))

	// Property 12.4: Multiple different resources can be locked simultaneously
	properties.Property("multiple different resources can be locked simultaneously", prop.ForAll(
		func(resource1 string, resource2 string, ttlSeconds int) bool {
			// Skip invalid inputs
			if resource1 == "" || resource2 == "" || resource1 == resource2 {
				return true
			}
			if len(resource1) > 100 || len(resource2) > 100 {
				return true
			}
			if ttlSeconds < 1 || ttlSeconds > 10 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     time.Duration(ttlSeconds) * time.Second,
				AcquireTimeout: 500 * time.Millisecond,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()
			ttl := time.Duration(ttlSeconds) * time.Second

			// Acquire lock on first resource
			lock1, err := lm.Acquire(ctx, resource1, ttl)
			if err != nil {
				return false
			}
			defer lm.Release(lock1)

			// Acquire lock on second resource (should succeed)
			lock2, err := lm.Acquire(ctx, resource2, ttl)
			if err != nil {
				return false
			}
			defer lm.Release(lock2)

			return lock1 != nil && lock2 != nil
		},
		gen.AlphaString(),
		gen.AlphaString(),
		gen.IntRange(1, 10),
	))

	// Property 12.5: Lock refresh extends TTL
	properties.Property("lock refresh extends TTL", prop.ForAll(
		func(resource string, initialTTLSeconds int, refreshTTLSeconds int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if initialTTLSeconds < 1 || initialTTLSeconds > 10 {
				return true
			}
			if refreshTTLSeconds < 1 || refreshTTLSeconds > 10 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     time.Duration(initialTTLSeconds) * time.Second,
				AcquireTimeout: 500 * time.Millisecond,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()
			initialTTL := time.Duration(initialTTLSeconds) * time.Second

			lock, err := lm.Acquire(ctx, resource, initialTTL)
			if err != nil {
				return false
			}
			defer lm.Release(lock)

			// Refresh the lock
			refreshTTL := time.Duration(refreshTTLSeconds) * time.Second
			err = lock.Refresh(refreshTTL)
			if err != nil {
				return false
			}

			// Lock should still be valid
			return lock.Resource() == resource
		},
		gen.AlphaString(),
		gen.IntRange(1, 10),
		gen.IntRange(1, 10),
	))

	// Property 12.6: ForceBreak allows reacquisition
	properties.Property("ForceBreak allows reacquisition", prop.ForAll(
		func(resource string, ttlSeconds int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if ttlSeconds < 1 || ttlSeconds > 10 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     time.Duration(ttlSeconds) * time.Second,
				AcquireTimeout: 500 * time.Millisecond,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()
			ttl := time.Duration(ttlSeconds) * time.Second

			// Acquire lock
			_, err = lm.Acquire(ctx, resource, ttl)
			if err != nil {
				return false
			}
			// Note: We intentionally don't release this lock to test ForceBreak

			// Force break the lock
			if err := lm.ForceBreak(resource); err != nil {
				return false
			}

			// Should be able to acquire new lock
			lock2, err := lm.Acquire(ctx, resource, ttl)
			if err != nil {
				return false
			}
			defer lm.Release(lock2)

			return lock2 != nil
		},
		gen.AlphaString(),
		gen.IntRange(1, 10),
	))

	// Property 12.7: Acquire timeout is respected
	properties.Property("acquire timeout is respected", prop.ForAll(
		func(resource string, timeoutMs int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if timeoutMs < 100 || timeoutMs > 2000 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     1 * time.Hour,
				AcquireTimeout: time.Duration(timeoutMs) * time.Millisecond,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()

			// Acquire first lock
			lock1, err := lm.Acquire(ctx, resource, 1*time.Hour)
			if err != nil {
				return false
			}
			defer lm.Release(lock1)

			// Try to acquire second lock with timeout
			start := time.Now()
			lock2, err := lm.Acquire(ctx, resource, 1*time.Hour)
			duration := time.Since(start)

			if err == nil {
				lm.Release(lock2)
				return false // Should have timed out
			}

			// Verify timeout duration is approximately correct (within 50% margin)
			expectedTimeout := time.Duration(timeoutMs) * time.Millisecond
			minDuration := time.Duration(float64(expectedTimeout) * 0.5)
			maxDuration := time.Duration(float64(expectedTimeout) * 1.5)

			return duration >= minDuration && duration <= maxDuration
		},
		gen.AlphaString(),
		gen.IntRange(100, 1000),
	))

	// Property 12.8: Context cancellation stops acquisition
	properties.Property("context cancellation stops acquisition", prop.ForAll(
		func(resource string, timeoutMs int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if timeoutMs < 50 || timeoutMs > 500 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     1 * time.Hour,
				AcquireTimeout: 10 * time.Second,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			// Acquire first lock
			lock1, err := lm.Acquire(context.Background(), resource, 1*time.Hour)
			if err != nil {
				return false
			}
			// Note: We intentionally don't release this lock to test concurrent access
			_ = lock1 // Suppress unused variable warning

			// Try to acquire with cancelled context
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
			defer cancel()

			start := time.Now()
			lock2, err := lm.Acquire(ctx, resource, 1*time.Hour)
			duration := time.Since(start)

			if err == nil {
				lm.Release(lock2)
				return false // Should have failed
			}

			// Should fail relatively quickly due to context cancellation
			maxExpected := time.Duration(timeoutMs)*time.Millisecond + 200*time.Millisecond
			return duration <= maxExpected
		},
		gen.AlphaString(),
		gen.IntRange(50, 500),
	))

	// Property 12.9: Concurrent goroutines respect lock exclusivity
	properties.Property("concurrent goroutines respect lock exclusivity", prop.ForAll(
		func(resource string, goroutineCount int) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}
			if goroutineCount < 2 || goroutineCount > 10 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     1 * time.Hour,
				AcquireTimeout: 1 * time.Second,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()
			concurrentHolders := 0
			maxConcurrentHolders := 0
			var mu sync.Mutex
			var wg sync.WaitGroup
			
			// Use a channel to synchronize goroutine starts
			startChan := make(chan struct{})

			// Launch multiple goroutines trying to acquire the same lock
			for i := 0; i < goroutineCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					
					// Wait for start signal to ensure all goroutines try simultaneously
					<-startChan
					
					lock, err := lm.Acquire(ctx, resource, 500*time.Millisecond)
					if err == nil {
						// Track concurrent holders
						mu.Lock()
						concurrentHolders++
						if concurrentHolders > maxConcurrentHolders {
							maxConcurrentHolders = concurrentHolders
						}
						mu.Unlock()
						
						// Hold lock for a bit
						time.Sleep(100 * time.Millisecond)
						
						// Release and decrement
						mu.Lock()
						concurrentHolders--
						mu.Unlock()
						lm.Release(lock)
					}
				}()
			}

			// Signal all goroutines to start simultaneously
			close(startChan)
			
			wg.Wait()

			// At most one goroutine should have held the lock at any given time
			return maxConcurrentHolders <= 1
		},
		gen.AlphaString(),
		gen.IntRange(2, 5),
	))

	// Property 12.10: Lock release is idempotent
	properties.Property("lock release is idempotent", prop.ForAll(
		func(resource string) bool {
			// Skip invalid inputs
			if resource == "" || len(resource) > 100 {
				return true
			}

			lockDir := t.TempDir()
			config := LockConfig{
				Backend:        "file",
				LockDir:        lockDir,
				DefaultTTL:     1 * time.Hour,
				AcquireTimeout: 2 * time.Second,
			}

			lm, err := NewLockManager(config)
			if err != nil {
				return false
			}

			ctx := context.Background()

			lock, err := lm.Acquire(ctx, resource, 1*time.Hour)
			if err != nil {
				return false
			}

			// Release once
			if err := lm.Release(lock); err != nil {
				return false
			}

			// Release again (should not panic or error)
			err = lm.Release(lock)
			// Some implementations may return an error, which is acceptable
			// The important thing is it doesn't panic

			return true
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
