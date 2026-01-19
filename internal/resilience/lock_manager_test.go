package resilience

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLockManager(t *testing.T) {
	tests := []struct {
		name    string
		config  LockConfig
		wantErr bool
	}{
		{
			name: "default config",
			config: LockConfig{
				Backend: "file",
			},
			wantErr: false,
		},
		{
			name: "custom lock dir",
			config: LockConfig{
				Backend: "file",
				LockDir: t.TempDir(),
			},
			wantErr: false,
		},
		{
			name: "unsupported backend",
			config: LockConfig{
				Backend: "unsupported",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lm, err := NewLockManager(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLockManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && lm == nil {
				t.Error("NewLockManager() returned nil without error")
			}
		})
	}
}

func TestLockManager_AcquireAndRelease(t *testing.T) {
	lockDir := t.TempDir()
	config := LockConfig{
		Backend:        "file",
		LockDir:        lockDir,
		DefaultTTL:     1 * time.Hour,
		AcquireTimeout: 5 * time.Second,
	}

	lm, err := NewLockManager(config)
	if err != nil {
		t.Fatalf("NewLockManager() error = %v", err)
	}

	ctx := context.Background()
	resource := "test-cluster"

	// Acquire lock
	lock, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if lock == nil {
		t.Fatal("Acquire() returned nil lock")
	}

	// Verify lock properties
	if lock.Resource() != resource {
		t.Errorf("Lock.Resource() = %v, want %v", lock.Resource(), resource)
	}

	if lock.Owner() == "" {
		t.Error("Lock.Owner() returned empty string")
	}

	if lock.AcquiredAt().IsZero() {
		t.Error("Lock.AcquiredAt() returned zero time")
	}

	// Verify lock file exists
	lockPath := filepath.Join(lockDir, resource+".lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Errorf("Lock file does not exist at %s", lockPath)
	}

	// Release lock
	if err := lm.Release(lock); err != nil {
		t.Errorf("Release() error = %v", err)
	}
}

func TestLockManager_ConcurrentAcquire(t *testing.T) {
	lockDir := t.TempDir()
	config := LockConfig{
		Backend:        "file",
		LockDir:        lockDir,
		DefaultTTL:     1 * time.Hour,
		AcquireTimeout: 1 * time.Second,
	}

	lm, err := NewLockManager(config)
	if err != nil {
		t.Fatalf("NewLockManager() error = %v", err)
	}

	ctx := context.Background()
	resource := "test-cluster"

	// Acquire first lock
	lock1, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("First Acquire() error = %v", err)
	}
	defer lm.Release(lock1)

	// Try to acquire second lock on same resource (should fail)
	lock2, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err == nil {
		lm.Release(lock2)
		t.Fatal("Second Acquire() should have failed but succeeded")
	}
}

func TestLockManager_AcquireAfterRelease(t *testing.T) {
	lockDir := t.TempDir()
	config := LockConfig{
		Backend:        "file",
		LockDir:        lockDir,
		DefaultTTL:     1 * time.Hour,
		AcquireTimeout: 5 * time.Second,
	}

	lm, err := NewLockManager(config)
	if err != nil {
		t.Fatalf("NewLockManager() error = %v", err)
	}

	ctx := context.Background()
	resource := "test-cluster"

	// Acquire first lock
	lock1, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("First Acquire() error = %v", err)
	}

	// Release first lock
	if err := lm.Release(lock1); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	// Acquire second lock on same resource (should succeed)
	lock2, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("Second Acquire() error = %v", err)
	}
	defer lm.Release(lock2)

	if lock2 == nil {
		t.Fatal("Second Acquire() returned nil lock")
	}
}

func TestLockManager_AcquireTimeout(t *testing.T) {
	lockDir := t.TempDir()
	config := LockConfig{
		Backend:        "file",
		LockDir:        lockDir,
		DefaultTTL:     1 * time.Hour,
		AcquireTimeout: 500 * time.Millisecond,
	}

	lm, err := NewLockManager(config)
	if err != nil {
		t.Fatalf("NewLockManager() error = %v", err)
	}

	ctx := context.Background()
	resource := "test-cluster"

	// Acquire first lock
	lock1, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("First Acquire() error = %v", err)
	}
	defer lm.Release(lock1)

	// Try to acquire second lock with short timeout
	start := time.Now()
	lock2, err := lm.Acquire(ctx, resource, 1*time.Hour)
	duration := time.Since(start)

	if err == nil {
		lm.Release(lock2)
		t.Fatal("Second Acquire() should have timed out but succeeded")
	}

	// Verify timeout duration is approximately correct
	if duration < 400*time.Millisecond || duration > 700*time.Millisecond {
		t.Errorf("Acquire timeout duration = %v, want ~500ms", duration)
	}
}

func TestLockManager_ForceBreak(t *testing.T) {
	lockDir := t.TempDir()
	config := LockConfig{
		Backend:        "file",
		LockDir:        lockDir,
		DefaultTTL:     1 * time.Hour,
		AcquireTimeout: 5 * time.Second,
	}

	lm, err := NewLockManager(config)
	if err != nil {
		t.Fatalf("NewLockManager() error = %v", err)
	}

	ctx := context.Background()
	resource := "test-cluster"

	// Acquire lock
	lock, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	// Force break the lock
	if err := lm.ForceBreak(resource); err != nil {
		t.Errorf("ForceBreak() error = %v", err)
	}

	// Verify we can acquire a new lock
	lock2, err := lm.Acquire(ctx, resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("Acquire after ForceBreak() error = %v", err)
	}
	defer lm.Release(lock2)

	// Original lock should still be valid in memory but file lock is broken
	if lock.Resource() != resource {
		t.Errorf("Original lock.Resource() = %v, want %v", lock.Resource(), resource)
	}
}

func TestLock_Refresh(t *testing.T) {
	lockDir := t.TempDir()
	config := LockConfig{
		Backend:        "file",
		LockDir:        lockDir,
		DefaultTTL:     1 * time.Hour,
		AcquireTimeout: 5 * time.Second,
	}

	lm, err := NewLockManager(config)
	if err != nil {
		t.Fatalf("NewLockManager() error = %v", err)
	}

	ctx := context.Background()
	resource := "test-cluster"

	// Acquire lock with short TTL
	lock, err := lm.Acquire(ctx, resource, 1*time.Second)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer lm.Release(lock)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Refresh the lock
	if err := lock.Refresh(2 * time.Second); err != nil {
		t.Errorf("Refresh() error = %v", err)
	}
}

func TestLockManager_ContextCancellation(t *testing.T) {
	lockDir := t.TempDir()
	config := LockConfig{
		Backend:        "file",
		LockDir:        lockDir,
		DefaultTTL:     1 * time.Hour,
		AcquireTimeout: 10 * time.Second,
	}

	lm, err := NewLockManager(config)
	if err != nil {
		t.Fatalf("NewLockManager() error = %v", err)
	}

	resource := "test-cluster"

	// Acquire first lock
	lock1, err := lm.Acquire(context.Background(), resource, 1*time.Hour)
	if err != nil {
		t.Fatalf("First Acquire() error = %v", err)
	}
	defer lm.Release(lock1)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Try to acquire second lock with cancelled context
	start := time.Now()
	lock2, err := lm.Acquire(ctx, resource, 1*time.Hour)
	duration := time.Since(start)

	if err == nil {
		lm.Release(lock2)
		t.Fatal("Acquire() with cancelled context should have failed")
	}

	// Verify it failed quickly due to context cancellation
	if duration > 1*time.Second {
		t.Errorf("Acquire() took %v, expected to fail quickly due to context cancellation", duration)
	}
}

func TestGenerateOwnerID(t *testing.T) {
	owner, err := generateOwnerID()
	if err != nil {
		t.Fatalf("generateOwnerID() error = %v", err)
	}

	if owner == "" {
		t.Error("generateOwnerID() returned empty string")
	}

	// Should contain hostname and PID
	if len(owner) < 3 {
		t.Errorf("generateOwnerID() = %v, seems too short", owner)
	}
}
