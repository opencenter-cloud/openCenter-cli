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

package validation

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheEntry represents a cached validation result with expiration.
type CacheEntry struct {
	Result    *ValidationResult
	ExpiresAt time.Time
}

// IsExpired returns true if the cache entry has expired.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// ValidationCache provides caching for validation results.
//
// The cache stores validation results keyed by validator name and data hash.
// Entries expire after a configurable TTL (default 5 minutes).
//
// Features:
//   - Thread-safe concurrent access
//   - Automatic expiration of stale entries
//   - Data hash-based invalidation
//   - Configurable TTL per entry
//
// Example usage:
//
//	cache := validation.NewValidationCache(5 * time.Minute)
//	
//	// Try to get cached result
//	if result := cache.Get("cluster-name", data); result != nil {
//	    return result, nil
//	}
//	
//	// Cache miss - perform validation
//	result := validator.Validate(ctx, data)
//	cache.Set("cluster-name", data, result, 5 * time.Minute)
type ValidationCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

// NewValidationCache creates a new validation cache with the specified TTL.
//
// Parameters:
//   - ttl: Time-to-live for cache entries (0 disables caching)
//
// Returns:
//   - *ValidationCache: New cache instance
//
// Example:
//
//	cache := validation.NewValidationCache(5 * time.Minute)
func NewValidationCache(ttl time.Duration) *ValidationCache {
	return &ValidationCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a cached validation result.
//
// Returns nil if:
//   - No cached entry exists
//   - The cached entry has expired
//   - The data hash doesn't match (data changed)
//
// Parameters:
//   - validatorName: Name of the validator
//   - data: Data to validate (used for hash calculation)
//
// Returns:
//   - *ValidationResult: Cached result or nil if not found/expired
//
// Example:
//
//	if result := cache.Get("cluster-name", clusterName); result != nil {
//	    // Cache hit - use cached result
//	    return result, nil
//	}
func (c *ValidationCache) Get(validatorName string, data interface{}) *ValidationResult {
	if c.ttl == 0 {
		return nil // Caching disabled
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Generate cache key
	key := c.generateKey(validatorName, data)

	// Check if entry exists
	entry, exists := c.entries[key]
	if !exists {
		return nil
	}

	// Check if entry has expired
	if entry.IsExpired() {
		return nil
	}

	return entry.Result
}

// Set stores a validation result in the cache.
//
// The result is stored with the specified TTL. If ttl is 0, the cache's
// default TTL is used.
//
// Parameters:
//   - validatorName: Name of the validator
//   - data: Data that was validated
//   - result: Validation result to cache
//   - ttl: Time-to-live for this entry (0 uses default)
//
// Example:
//
//	result := validator.Validate(ctx, data)
//	cache.Set("cluster-name", data, result, 5 * time.Minute)
func (c *ValidationCache) Set(validatorName string, data interface{}, result *ValidationResult, ttl time.Duration) {
	if c.ttl == 0 {
		return // Caching disabled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.ttl
	}

	// Generate cache key
	key := c.generateKey(validatorName, data)

	// Store entry
	c.entries[key] = &CacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Invalidate removes a cached entry.
//
// Parameters:
//   - validatorName: Name of the validator
//   - data: Data to invalidate
//
// Example:
//
//	// Data changed - invalidate cache
//	cache.Invalidate("cluster-name", oldData)
func (c *ValidationCache) Invalidate(validatorName string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.generateKey(validatorName, data)
	delete(c.entries, key)
}

// InvalidateAll removes all cached entries for a validator.
//
// Parameters:
//   - validatorName: Name of the validator
//
// Example:
//
//	// Validator logic changed - invalidate all entries
//	cache.InvalidateAll("cluster-name")
func (c *ValidationCache) InvalidateAll(validatorName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find and delete all entries for this validator
	for key := range c.entries {
		if c.keyMatchesValidator(key, validatorName) {
			delete(c.entries, key)
		}
	}
}

// Clear removes all cached entries.
//
// Example:
//
//	cache.Clear() // Clear all cached validation results
func (c *ValidationCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// CleanExpired removes all expired entries from the cache.
//
// This method should be called periodically to prevent memory leaks from
// expired entries. Consider running it in a background goroutine.
//
// Returns:
//   - int: Number of entries removed
//
// Example:
//
//	// Run cleanup every minute
//	go func() {
//	    ticker := time.NewTicker(1 * time.Minute)
//	    defer ticker.Stop()
//	    for range ticker.C {
//	        removed := cache.CleanExpired()
//	        log.Printf("Cleaned %d expired cache entries", removed)
//	    }
//	}()
func (c *ValidationCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	now := time.Now()

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			removed++
		}
	}

	return removed
}

// Size returns the number of cached entries.
//
// Returns:
//   - int: Number of entries in cache
func (c *ValidationCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// Stats returns cache statistics.
//
// Returns:
//   - total: Total number of entries
//   - expired: Number of expired entries
type CacheStats struct {
	Total   int
	Expired int
}

// Stats returns cache statistics.
//
// Returns:
//   - CacheStats: Cache statistics
func (c *ValidationCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Total: len(c.entries),
	}

	now := time.Now()
	for _, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			stats.Expired++
		}
	}

	return stats
}

// generateKey generates a cache key from validator name and data.
//
// The key is generated by:
//   1. JSON-encoding the data
//   2. Computing SHA-256 hash of the JSON
//   3. Combining validator name with hash
//
// This ensures that:
//   - Different data produces different keys
//   - Same data produces same key (cache hit)
//   - Keys are unique per validator
func (c *ValidationCache) generateKey(validatorName string, data interface{}) string {
	// Compute data hash
	hash := c.hashData(data)
	
	// Combine validator name with hash
	return fmt.Sprintf("%s:%s", validatorName, hash)
}

// hashData computes a SHA-256 hash of the data.
//
// The data is JSON-encoded before hashing to ensure consistent
// representation across different data types.
func (c *ValidationCache) hashData(data interface{}) string {
	// JSON-encode the data
	jsonData, err := json.Marshal(data)
	if err != nil {
		// If marshaling fails, use string representation
		jsonData = []byte(fmt.Sprintf("%v", data))
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256(jsonData)
	
	// Return hex-encoded hash
	return fmt.Sprintf("%x", hash)
}

// keyMatchesValidator checks if a cache key belongs to a validator.
func (c *ValidationCache) keyMatchesValidator(key, validatorName string) bool {
	prefix := validatorName + ":"
	return len(key) >= len(prefix) && key[:len(prefix)] == prefix
}
