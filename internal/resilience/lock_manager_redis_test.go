package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/testenv"
)

func startMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	testenv.RequireLoopbackBind(t)

	redisServer, err := miniredis.Run()
	if err != nil {
		t.Skipf("loopback redis unavailable in this environment: %v", err)
	}
	return redisServer
}

func TestRedisLockBackendAcquireRelease(t *testing.T) {
	redisServer := startMiniredis(t)
	defer redisServer.Close()

	manager, err := NewLockManager(LockConfig{
		Backend:        "redis",
		RedisAddr:      redisServer.Addr(),
		DefaultTTL:     time.Minute,
		AcquireTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create redis lock manager: %v", err)
	}

	lock, err := manager.AcquireWithMetadata(context.Background(), "cluster/test", time.Minute, map[string]string{"reason": "test"})
	if err != nil {
		t.Fatalf("failed to acquire redis lock: %v", err)
	}

	info, err := manager.GetLockInfo("cluster/test")
	if err != nil {
		t.Fatalf("failed to read redis lock info: %v", err)
	}
	if info == nil || info.Owner == "" {
		t.Fatalf("expected redis lock info to be present")
	}
	if info.Metadata["reason"] != "test" {
		t.Fatalf("expected metadata reason=test, got %q", info.Metadata["reason"])
	}

	if err := manager.Release(lock); err != nil {
		t.Fatalf("failed to release redis lock: %v", err)
	}

	info, err = manager.GetLockInfo("cluster/test")
	if err != nil {
		t.Fatalf("failed to read redis lock info after release: %v", err)
	}
	if info != nil {
		t.Fatalf("expected redis lock to be gone after release")
	}
}

func TestRedisLockBackendRefreshAndForceBreak(t *testing.T) {
	redisServer := startMiniredis(t)
	defer redisServer.Close()

	manager, err := NewLockManager(LockConfig{
		Backend:        "redis",
		RedisAddr:      redisServer.Addr(),
		DefaultTTL:     time.Second,
		AcquireTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create redis lock manager: %v", err)
	}

	lock, err := manager.Acquire(context.Background(), "cluster/test", time.Second)
	if err != nil {
		t.Fatalf("failed to acquire redis lock: %v", err)
	}

	redisServer.FastForward(500 * time.Millisecond)
	if err := lock.Refresh(3 * time.Second); err != nil {
		t.Fatalf("failed to refresh redis lock: %v", err)
	}

	info, err := manager.GetLockInfo("cluster/test")
	if err != nil {
		t.Fatalf("failed to read redis lock info: %v", err)
	}
	if info == nil || info.TTL < 2*time.Second {
		t.Fatalf("expected refreshed TTL to be extended, got %v", info.TTL)
	}

	if err := manager.ForceBreak("cluster/test"); err != nil {
		t.Fatalf("failed to force-break redis lock: %v", err)
	}

	info, err = manager.GetLockInfo("cluster/test")
	if err != nil {
		t.Fatalf("failed to read redis lock info after force-break: %v", err)
	}
	if info != nil {
		t.Fatalf("expected redis lock to be gone after force-break")
	}
}

func TestRedisLockBackendAcquireTimeout(t *testing.T) {
	redisServer := startMiniredis(t)
	defer redisServer.Close()

	manager, err := NewLockManager(LockConfig{
		Backend:        "redis",
		RedisAddr:      redisServer.Addr(),
		DefaultTTL:     time.Minute,
		AcquireTimeout: 150 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create redis lock manager: %v", err)
	}

	lock, err := manager.Acquire(context.Background(), "cluster/test", time.Minute)
	if err != nil {
		t.Fatalf("failed to acquire initial redis lock: %v", err)
	}
	defer manager.Release(lock)

	start := time.Now()
	if _, err := manager.Acquire(context.Background(), "cluster/test", time.Minute); err == nil {
		t.Fatalf("expected second acquire to time out")
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("expected second acquire to wait before timing out")
	}
}
