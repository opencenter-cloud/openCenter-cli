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

package paths

import (
	"fmt"
	"sync"
	"time"
)

// CacheEntry represents a cached path resolution result.
type CacheEntry struct {
	// Paths contains the resolved cluster paths
	Paths *ClusterPaths

	// Timestamp is when the entry was cached
	Timestamp time.Time

	// Organization is the organization used for resolution
	Organization string

	// Strategy is the strategy that resolved the paths
	Strategy string
}

// PathCache provides thread-safe caching for path resolution results.
type PathCache struct {
	// entries stores cached path resolutions
	entries map[string]*CacheEntry

	// mu protects concurrent access to the cache
	mu sync.RWMutex

	// ttl is the time-to-live for cache entries
	ttl time.Duration

	// maxSize is the maximum number of entries to cache
	maxSize int

	// hits tracks cache hit count
	hits uint64

	// misses tracks cache miss count
	misses uint64
}

// NewPathCache creates a new path cache with the given TTL and max size.
func NewPathCache(ttl time.Duration, maxSize int) *PathCache {
	return &PathCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// DefaultPathCache creates a path cache with default settings.
// Default TTL: 5 minutes
// Default max size: 100 entries
func DefaultPathCache() *PathCache {
	return NewPathCache(5*time.Minute, 100)
}

// Get retrieves a cached path resolution result.
// Returns nil if the entry is not found or has expired.
func (c *PathCache) Get(clusterName, organization string) *ClusterPaths {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.makeKey(clusterName, organization)
	entry, exists := c.entries[key]

	if !exists {
		c.misses++
		return nil
	}

	// Fast path: check expiration without allocating time.Now()
	// Only check expiration if TTL is set (non-zero)
	if c.ttl > 0 && time.Since(entry.Timestamp) > c.ttl {
		c.misses++
		// Don't delete here to avoid write lock upgrade
		// Cleanup will happen on next Set or explicit Clear
		return nil
	}

	c.hits++
	return entry.Paths
}

// Set stores a path resolution result in the cache.
func (c *PathCache) Set(clusterName, organization, strategy string, paths *ClusterPaths) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce max size by removing oldest entries
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	key := c.makeKey(clusterName, organization)
	c.entries[key] = &CacheEntry{
		Paths:        paths,
		Timestamp:    time.Now(),
		Organization: organization,
		Strategy:     strategy,
	}
}

// Invalidate removes a specific cache entry.
func (c *PathCache) Invalidate(clusterName, organization string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.makeKey(clusterName, organization)
	delete(c.entries, key)
}

// InvalidateCluster removes all cache entries for a cluster (across all organizations).
func (c *PathCache) InvalidateCluster(clusterName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find and remove all entries for this cluster
	for key := range c.entries {
		if c.matchesCluster(key, clusterName) {
			delete(c.entries, key)
		}
	}
}

// Clear removes all cache entries.
func (c *PathCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.hits = 0
	c.misses = 0
}

// CleanupExpired removes all expired cache entries.
func (c *PathCache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	now := time.Now()

	for key, entry := range c.entries {
		if now.Sub(entry.Timestamp) > c.ttl {
			delete(c.entries, key)
			removed++
		}
	}

	return removed
}

// Stats returns cache statistics.
func (c *PathCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Entries: len(c.entries),
		Hits:    c.hits,
		Misses:  c.misses,
		HitRate: hitRate,
		MaxSize: c.maxSize,
		TTL:     c.ttl,
	}
}

// Size returns the current number of cache entries.
func (c *PathCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// makeKey creates a cache key from cluster name and organization.
// Optimized to avoid fmt.Sprintf allocation for common case.
func (c *PathCache) makeKey(clusterName, organization string) string {
	if organization == "" {
		organization = "opencenter"
	}
	// Avoid fmt.Sprintf allocation by using string concatenation
	// This is faster for simple string joining
	return organization + ":" + clusterName
}

// matchesCluster checks if a cache key matches the given cluster name.
func (c *PathCache) matchesCluster(key, clusterName string) bool {
	// Key format is "organization:clusterName"
	// Extract cluster name from key
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			return key[i+1:] == clusterName
		}
	}
	return false
}

// evictOldest removes the oldest cache entry.
// Must be called with write lock held.
func (c *PathCache) evictOldest() {
	if len(c.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time

	// Find the oldest entry
	for key, entry := range c.entries {
		if oldestKey == "" || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
		}
	}

	// Remove the oldest entry
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// CacheStats contains cache statistics.
type CacheStats struct {
	// Entries is the current number of cached entries
	Entries int

	// Hits is the total number of cache hits
	Hits uint64

	// Misses is the total number of cache misses
	Misses uint64

	// HitRate is the cache hit rate (0.0 to 1.0)
	HitRate float64

	// MaxSize is the maximum number of entries
	MaxSize int

	// TTL is the time-to-live for cache entries
	TTL time.Duration
}

// String returns a string representation of cache statistics.
func (s CacheStats) String() string {
	return fmt.Sprintf(
		"Cache Stats: %d entries, %d hits, %d misses, %.2f%% hit rate, max size: %d, TTL: %s",
		s.Entries,
		s.Hits,
		s.Misses,
		s.HitRate*100,
		s.MaxSize,
		s.TTL,
	)
}
