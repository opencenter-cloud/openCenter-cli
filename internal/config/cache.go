// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a cached configuration with metadata.
type CacheEntry struct {
	Config    *Config
	Timestamp time.Time
	TTL       time.Duration
}

// IsExpired checks if the cache entry has expired.
func (ce *CacheEntry) IsExpired() bool {
	if ce.TTL <= 0 {
		return false // No expiration
	}
	return time.Since(ce.Timestamp) > ce.TTL
}

// InMemoryConfigCache implements the ConfigCacheInterface using in-memory storage.
type InMemoryConfigCache struct {
	cache map[string]*CacheEntry
	mu    sync.RWMutex

	// Configuration
	defaultTTL time.Duration
	maxSize    int
}

// NewInMemoryConfigCache creates a new in-memory configuration cache.
func NewInMemoryConfigCache(defaultTTL time.Duration, maxSize int) *InMemoryConfigCache {
	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}
	if maxSize <= 0 {
		maxSize = 100
	}

	return &InMemoryConfigCache{
		cache:      make(map[string]*CacheEntry),
		defaultTTL: defaultTTL,
		maxSize:    maxSize,
	}
}

// Get retrieves a cached configuration.
func (c *InMemoryConfigCache) Get(ctx context.Context, key string) (*Config, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if entry.IsExpired() {
		// Remove expired entry (will be cleaned up later)
		go c.removeExpired(key)
		return nil, false
	}

	// Return a copy of the configuration to prevent modification
	configCopy := *entry.Config
	return &configCopy, true
}

// Set stores a configuration in cache.
func (c *InMemoryConfigCache) Set(ctx context.Context, key string, config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries to make room
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	// Store a copy of the configuration to prevent external modification
	configCopy := *config
	c.cache[key] = &CacheEntry{
		Config:    &configCopy,
		Timestamp: time.Now(),
		TTL:       c.defaultTTL,
	}

	return nil
}

// Delete removes a configuration from cache.
func (c *InMemoryConfigCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
	return nil
}

// Clear clears all cached configurations.
func (c *InMemoryConfigCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry)
	return nil
}

// InvalidateCluster invalidates all cache entries for a cluster.
func (c *InMemoryConfigCache) InvalidateCluster(ctx context.Context, clusterName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find and remove all entries related to the cluster
	keysToDelete := make([]string, 0)
	for key := range c.cache {
		if c.isClusterRelated(key, clusterName) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(c.cache, key)
	}

	return nil
}

// SetTTL sets a custom TTL for a specific cache entry.
func (c *InMemoryConfigCache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if !exists {
		return fmt.Errorf("cache entry not found: %s", key)
	}

	entry.TTL = ttl
	return nil
}

// GetStats returns cache statistics.
func (c *InMemoryConfigCache) GetStats(ctx context.Context) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	expired := 0
	for _, entry := range c.cache {
		if entry.IsExpired() {
			expired++
		}
	}

	return map[string]interface{}{
		"total_entries":   len(c.cache),
		"expired_entries": expired,
		"max_size":        c.maxSize,
		"default_ttl":     c.defaultTTL.String(),
	}
}

// CleanupExpired removes all expired entries from the cache.
func (c *InMemoryConfigCache) CleanupExpired(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	keysToDelete := make([]string, 0)
	for key, entry := range c.cache {
		if entry.IsExpired() {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(c.cache, key)
	}

	return nil
}

// StartCleanupRoutine starts a background routine to clean up expired entries.
func (c *InMemoryConfigCache) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.CleanupExpired(ctx)
			}
		}
	}()
}

// removeExpired removes an expired entry (called asynchronously).
func (c *InMemoryConfigCache) removeExpired(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if exists && entry.IsExpired() {
		delete(c.cache, key)
	}
}

// evictOldest removes the oldest entry from the cache.
func (c *InMemoryConfigCache) evictOldest() {
	if len(c.cache) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range c.cache {
		if first || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
			first = false
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// isClusterRelated checks if a cache key is related to a specific cluster.
func (c *InMemoryConfigCache) isClusterRelated(key, clusterName string) bool {
	// Simple heuristic: check if the key contains the cluster name
	return strings.Contains(key, clusterName)
}

// GetKeys returns all cache keys (for debugging).
func (c *InMemoryConfigCache) GetKeys(ctx context.Context) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.cache))
	for key := range c.cache {
		keys = append(keys, key)
	}

	return keys
}

// Contains checks if a key exists in the cache.
func (c *InMemoryConfigCache) Contains(ctx context.Context, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		return false
	}

	return !entry.IsExpired()
}

// Size returns the current number of entries in the cache.
func (c *InMemoryConfigCache) Size(ctx context.Context) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}

// SetMaxSize sets the maximum cache size.
func (c *InMemoryConfigCache) SetMaxSize(maxSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxSize = maxSize

	// Evict entries if current size exceeds new max size
	for len(c.cache) > c.maxSize {
		c.evictOldest()
	}
}

// SetDefaultTTL sets the default TTL for new cache entries.
func (c *InMemoryConfigCache) SetDefaultTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.defaultTTL = ttl
}
