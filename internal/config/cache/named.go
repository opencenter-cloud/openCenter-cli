package cache

import (
	"context"
	"sync"
	"time"
)

type Entry[T any] struct {
	Value     T
	LoadedAt  time.Time
	ExpiresAt time.Time
}

// NamedCache is a generic in-memory cache keyed by configuration name.
type NamedCache[T any] struct {
	entries map[string]*Entry[T]
	mu      sync.RWMutex
}

func NewNamedCache[T any]() *NamedCache[T] {
	return &NamedCache[T]{
		entries: make(map[string]*Entry[T]),
	}
}

func (c *NamedCache[T]) Get(ctx context.Context, name string) (T, bool) {
	var zero T

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[name]
	if !exists {
		return zero, false
	}

	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		return zero, false
	}

	return entry.Value, true
}

func (c *NamedCache[T]) Set(ctx context.Context, name string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[name] = &Entry[T]{
		Value:     value,
		LoadedAt:  time.Now(),
		ExpiresAt: time.Time{},
	}
}

func (c *NamedCache[T]) SetWithExpiration(ctx context.Context, name string, value T, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[name] = &Entry[T]{
		Value:     value,
		LoadedAt:  time.Now(),
		ExpiresAt: expiresAt,
	}
}

func (c *NamedCache[T]) Invalidate(ctx context.Context, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, name)
}

func (c *NamedCache[T]) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*Entry[T])
}

func (c *NamedCache[T]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
