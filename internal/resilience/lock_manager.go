package resilience

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	internalConfig "github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

// LockManager manages distributed locks to prevent concurrent operations
type LockManager interface {
	Acquire(ctx context.Context, resource string, ttl time.Duration) (Lock, error)
	AcquireWithMetadata(ctx context.Context, resource string, ttl time.Duration, metadata map[string]string) (Lock, error)
	Release(lock Lock) error
	ForceBreak(resource string) error
	GetLockInfo(resource string) (*LockState, error)
}

// Lock represents an acquired lock
type Lock interface {
	Resource() string
	Owner() string
	AcquiredAt() time.Time
	Refresh(ttl time.Duration) error
}

// LockConfig configures lock manager behavior
type LockConfig struct {
	Backend        string        // "file" or "redis"
	RedisAddr      string        // Redis connection string (for redis backend)
	LockDir        string        // Directory for lock files (for file backend)
	DefaultTTL     time.Duration // Default lock TTL (default: 1 hour)
	AcquireTimeout time.Duration // Timeout for acquiring lock (default: 30 seconds)
}

// DefaultLockConfig provides sensible defaults
var DefaultLockConfig = LockConfig{
	Backend:        "file",
	LockDir:        "", // Will be set to ~/.config/opencenter/locks
	DefaultTTL:     1 * time.Hour,
	AcquireTimeout: 30 * time.Second,
}

// LockState tracks the state of a lock
type LockState struct {
	Resource   string
	Owner      string
	AcquiredAt time.Time
	ExpiresAt  time.Time
	TTL        time.Duration
	Metadata   map[string]string
}

// lockManager implements LockManager interface
type lockManager struct {
	config  LockConfig
	backend lockBackend
	mu      sync.Mutex
}

// lockBackend is an internal interface for different lock implementations
type lockBackend interface {
	acquire(ctx context.Context, resource string, owner string, ttl time.Duration, metadata map[string]string) (Lock, error)
	release(lock Lock) error
	forceBreak(resource string) error
	getLockInfo(resource string) (*LockState, error)
}

// NewLockManager creates a new lock manager with the given configuration
func NewLockManager(config LockConfig) (LockManager, error) {
	// Set defaults if not provided
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 1 * time.Hour
	}
	if config.AcquireTimeout == 0 {
		config.AcquireTimeout = 30 * time.Second
	}
	if config.Backend == "" {
		config.Backend = "file"
	}

	// Set default lock directory for file backend
	if config.Backend == "file" && config.LockDir == "" {
		configDir := internalConfig.GetConfigDir()
		config.LockDir = filepath.Join(configDir, "locks")
	}

	// Create the backend
	var backend lockBackend
	var err error

	switch config.Backend {
	case "file":
		backend, err = newFileLockBackend(config.LockDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create file lock backend: %w", err)
		}
	case "redis":
		return nil, fmt.Errorf("redis backend not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported lock backend: %s", config.Backend)
	}

	return &lockManager{
		config:  config,
		backend: backend,
	}, nil
}

// Acquire attempts to acquire a lock on the given resource
func (lm *lockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (Lock, error) {
	return lm.AcquireWithMetadata(ctx, resource, ttl, nil)
}

// AcquireWithMetadata attempts to acquire a lock with additional metadata
func (lm *lockManager) AcquireWithMetadata(ctx context.Context, resource string, ttl time.Duration, metadata map[string]string) (Lock, error) {
	if ttl == 0 {
		ttl = lm.config.DefaultTTL
	}

	// Create a context with acquire timeout
	acquireCtx, cancel := context.WithTimeout(ctx, lm.config.AcquireTimeout)
	defer cancel()

	// Generate owner identifier (hostname + PID)
	owner, err := generateOwnerID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate owner ID: %w", err)
	}

	// Try to acquire the lock with retries
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-acquireCtx.Done():
			return nil, fmt.Errorf("failed to acquire lock on %s within %v: %w",
				resource, lm.config.AcquireTimeout, acquireCtx.Err())
		case <-ticker.C:
			lock, err := lm.backend.acquire(acquireCtx, resource, owner, ttl, metadata)
			if err == nil {
				return lock, nil
			}
			// Continue retrying on error
		}
	}
}

// Release releases the given lock
func (lm *lockManager) Release(lock Lock) error {
	if lock == nil {
		return fmt.Errorf("cannot release nil lock")
	}
	return lm.backend.release(lock)
}

// ForceBreak forcefully breaks a lock (use with caution)
func (lm *lockManager) ForceBreak(resource string) error {
	return lm.backend.forceBreak(resource)
}

// GetLockInfo retrieves information about a lock without acquiring it
func (lm *lockManager) GetLockInfo(resource string) (*LockState, error) {
	return lm.backend.getLockInfo(resource)
}

// generateOwnerID creates a unique identifier for this lock holder
func generateOwnerID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	pid := os.Getpid()
	return fmt.Sprintf("%s:%d", hostname, pid), nil
}

// ============================================================================
// File-based Lock Backend
// ============================================================================

// fileLockBackend implements lockBackend using file-based locking with flock()
type fileLockBackend struct {
	lockDir    string
	fileSystem fs.FileSystem
	mu         sync.Mutex
	locks      map[string]*fileLock
}

// fileLock represents a file-based lock
type fileLock struct {
	resource   string
	owner      string
	acquiredAt time.Time
	expiresAt  time.Time
	ttl        time.Duration
	file       *os.File
	mu         sync.Mutex
}

// newFileLockBackend creates a new file-based lock backend
func newFileLockBackend(lockDir string) (*fileLockBackend, error) {
	// Create lock directory if it doesn't exist
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Create FileSystem with error handler
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	return &fileLockBackend{
		lockDir:    lockDir,
		fileSystem: fileSystem,
		locks:      make(map[string]*fileLock),
	}, nil
}

// acquire attempts to acquire a file-based lock
func (fb *fileLockBackend) acquire(ctx context.Context, resource string, owner string, ttl time.Duration, metadata map[string]string) (Lock, error) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	// Check if we already have this lock
	if existingLock, exists := fb.locks[resource]; exists {
		// Check if lock has expired
		if time.Now().Before(existingLock.expiresAt) {
			return nil, fmt.Errorf("lock already held by %s (acquired at %v, expires at %v)",
				existingLock.owner, existingLock.acquiredAt, existingLock.expiresAt)
		}
		// Lock has expired, clean it up inline to avoid deadlock
		existingLock.mu.Lock()
		if existingLock.file != nil {
			syscall.Flock(int(existingLock.file.Fd()), syscall.LOCK_UN)
			existingLock.file.Close()
			existingLock.file = nil
		}
		existingLock.mu.Unlock()
		delete(fb.locks, resource)
	}

	// Create lock file path
	lockPath := filepath.Join(fb.lockDir, resource+".lock")

	// Ensure parent directory exists (in case resource contains path separators)
	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create the lock file
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock with flock
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to acquire file lock: %w", err)
	}

	// Write lock metadata to file
	now := time.Now()
	expiresAt := now.Add(ttl)
	metadataStr := fmt.Sprintf("owner=%s\nacquired=%s\nexpires=%s\nttl=%s\n",
		owner, now.Format(time.RFC3339), expiresAt.Format(time.RFC3339), ttl)

	// Add custom metadata
	if metadata != nil {
		for key, value := range metadata {
			metadataStr += fmt.Sprintf("%s=%s\n", key, value)
		}
	}

	if _, err := file.WriteAt([]byte(metadataStr), 0); err != nil {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
		return nil, fmt.Errorf("failed to write lock metadata: %w", err)
	}

	// Create lock object
	lock := &fileLock{
		resource:   resource,
		owner:      owner,
		acquiredAt: now,
		expiresAt:  expiresAt,
		ttl:        ttl,
		file:       file,
	}

	// Store in our map
	fb.locks[resource] = lock

	return lock, nil
}

// release releases a file-based lock
func (fb *fileLockBackend) release(lock Lock) error {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	fileLock, ok := lock.(*fileLock)
	if !ok {
		return fmt.Errorf("invalid lock type")
	}

	fileLock.mu.Lock()
	defer fileLock.mu.Unlock()

	// Release the flock
	if fileLock.file != nil {
		syscall.Flock(int(fileLock.file.Fd()), syscall.LOCK_UN)
		fileLock.file.Close()
		fileLock.file = nil
	}

	// Remove from our map
	delete(fb.locks, fileLock.resource)

	return nil
}

// forceBreak forcefully breaks a file-based lock
func (fb *fileLockBackend) forceBreak(resource string) error {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	// Check if we have this lock
	if lock, exists := fb.locks[resource]; exists {
		// Release the lock without calling fb.release to avoid deadlock
		lock.mu.Lock()
		if lock.file != nil {
			syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
			lock.file.Close()
			lock.file = nil
		}
		lock.mu.Unlock()
		delete(fb.locks, resource)
	}

	// Try to remove the lock file
	lockPath := filepath.Join(fb.lockDir, resource+".lock")
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	return nil
}

// getLockInfo retrieves information about a lock by reading the lock file
func (fb *fileLockBackend) getLockInfo(resource string) (*LockState, error) {
	lockPath := filepath.Join(fb.lockDir, resource+".lock")

	// Check if lock file exists
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		return nil, nil // No lock exists
	}

	// Read lock file content using FileSystem
	content, err := fb.fileSystem.ReadFile(lockPath)
	if err != nil {
		// Handle not found case
		if os.IsNotExist(stderrors.Unwrap(err)) {
			return nil, nil // No lock exists
		}
		return nil, fmt.Errorf("failed to read lock file: %w", err)
	}

	// Parse lock metadata (format: key=value\n)
	state := &LockState{
		Resource: resource,
		Metadata: make(map[string]string),
	}

	lines := string(content)
	for i := 0; i < len(lines); {
		// Find the next newline
		endIdx := i
		for endIdx < len(lines) && lines[endIdx] != '\n' {
			endIdx++
		}

		line := lines[i:endIdx]
		if line != "" {
			// Split on '='
			eqIdx := -1
			for j := 0; j < len(line); j++ {
				if line[j] == '=' {
					eqIdx = j
					break
				}
			}

			if eqIdx > 0 {
				key := line[:eqIdx]
				value := line[eqIdx+1:]

				switch key {
				case "owner":
					state.Owner = value
				case "acquired":
					if t, err := time.Parse(time.RFC3339, value); err == nil {
						state.AcquiredAt = t
					}
				case "expires":
					if t, err := time.Parse(time.RFC3339, value); err == nil {
						state.ExpiresAt = t
					}
				case "ttl":
					if d, err := time.ParseDuration(value); err == nil {
						state.TTL = d
					}
				default:
					state.Metadata[key] = value
				}
			}
		}

		i = endIdx + 1
	}

	return state, nil
}

// Lock interface implementation for fileLock

func (fl *fileLock) Resource() string {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.resource
}

func (fl *fileLock) Owner() string {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.owner
}

func (fl *fileLock) AcquiredAt() time.Time {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.acquiredAt
}

func (fl *fileLock) Refresh(ttl time.Duration) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.file == nil {
		return fmt.Errorf("lock has been released")
	}

	// Update expiration time
	fl.expiresAt = time.Now().Add(ttl)
	fl.ttl = ttl

	// Update metadata in file
	metadata := fmt.Sprintf("owner=%s\nacquired=%s\nexpires=%s\nttl=%s\n",
		fl.owner, fl.acquiredAt.Format(time.RFC3339),
		fl.expiresAt.Format(time.RFC3339), fl.ttl)

	if _, err := fl.file.WriteAt([]byte(metadata), 0); err != nil {
		return fmt.Errorf("failed to refresh lock metadata: %w", err)
	}

	return nil
}
