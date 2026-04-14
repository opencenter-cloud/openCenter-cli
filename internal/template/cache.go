/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"sync"
	"text/template"
	"time"
)

// TemplateCache provides thread-safe caching of parsed templates.
// It supports TTL-based expiration and size limits to prevent unbounded growth.
type TemplateCache interface {
	// Get retrieves a template from the cache
	Get(key string) (*template.Template, bool)

	// Set stores a template in the cache
	Set(key string, tmpl *template.Template)

	// Delete removes a template from the cache
	Delete(key string)

	// Clear removes all templates from the cache
	Clear()

	// Size returns the number of cached templates
	Size() int
}

// CacheEntry represents a cached template with metadata.
type CacheEntry struct {
	Template    *template.Template
	CreatedAt   time.Time
	AccessedAt  time.Time
	AccessCount int64
}

// InMemoryTemplateCache implements TemplateCache with in-memory storage.
// It provides thread-safe access and optional TTL-based expiration.
type InMemoryTemplateCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	maxSize int
}

// NewInMemoryTemplateCache creates a new in-memory template cache.
// If ttl is 0, entries never expire. If maxSize is 0, cache size is unlimited.
func NewInMemoryTemplateCache(ttl time.Duration, maxSize int) *InMemoryTemplateCache {
	return &InMemoryTemplateCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get retrieves a template from the cache.
// Returns the template and true if found and not expired, nil and false otherwise.
func (c *InMemoryTemplateCache) Get(key string) (*template.Template, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	// Check TTL expiration
	if c.ttl > 0 && time.Since(entry.CreatedAt) > c.ttl {
		// Entry expired, will be cleaned up on next Set
		return nil, false
	}

	// Update access metadata
	entry.AccessedAt = time.Now()
	entry.AccessCount++

	return entry.Template, true
}

// Set stores a template in the cache.
// If the cache is full, it evicts the least recently used entry.
func (c *InMemoryTemplateCache) Set(key string, tmpl *template.Template) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries
	if c.maxSize > 0 && len(c.entries) >= c.maxSize {
		// Evict least recently used entry
		c.evictLRU()
	}

	// Clean up expired entries
	if c.ttl > 0 {
		c.cleanupExpired()
	}

	// Store new entry
	c.entries[key] = &CacheEntry{
		Template:    tmpl,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
	}
}

// Delete removes a template from the cache.
func (c *InMemoryTemplateCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// Clear removes all templates from the cache.
func (c *InMemoryTemplateCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// Size returns the number of cached templates.
func (c *InMemoryTemplateCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// evictLRU evicts the least recently used entry from the cache.
// Must be called with write lock held.
func (c *InMemoryTemplateCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cleanupExpired removes expired entries from the cache.
// Must be called with write lock held.
func (c *InMemoryTemplateCache) cleanupExpired() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.Sub(entry.CreatedAt) > c.ttl {
			delete(c.entries, key)
		}
	}
}

// CacheStats provides statistics about cache usage.
type CacheStats struct {
	Size        int
	Hits        int64
	Misses      int64
	Evictions   int64
	TotalAccess int64
}

// GetStats returns cache statistics.
// This is useful for monitoring and optimization.
func (c *InMemoryTemplateCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalAccess int64
	for _, entry := range c.entries {
		totalAccess += entry.AccessCount
	}

	return CacheStats{
		Size:        len(c.entries),
		TotalAccess: totalAccess,
	}
}
