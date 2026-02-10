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
	"sync"
	"time"
)

// cacheEntry represents a cached configuration with metadata.
type cacheEntry struct {
	config    *Config
	loadedAt  time.Time
	expiresAt time.Time
}

// ConfigCache provides thread-safe caching of configurations.
// It stores loaded configurations in memory to avoid repeated disk reads.
type ConfigCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
}

// NewConfigCache creates a new ConfigCache instance.
func NewConfigCache() *ConfigCache {
	return &ConfigCache{
		entries: make(map[string]*cacheEntry),
	}
}

// Get retrieves a configuration from cache.
// Returns the cached config and true if found and not expired, nil and false otherwise.
func (cc *ConfigCache) Get(ctx context.Context, name string) (*Config, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	entry, exists := cc.entries[name]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.config, true
}

// Set stores a configuration in cache with optional expiration.
// If expiration is zero, the entry never expires.
func (cc *ConfigCache) Set(ctx context.Context, name string, config *Config) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.entries[name] = &cacheEntry{
		config:    config,
		loadedAt:  time.Now(),
		expiresAt: time.Time{}, // No expiration by default
	}
}

// SetWithExpiration stores a configuration in cache with a specific expiration time.
func (cc *ConfigCache) SetWithExpiration(ctx context.Context, name string, config *Config, expiresAt time.Time) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.entries[name] = &cacheEntry{
		config:    config,
		loadedAt:  time.Now(),
		expiresAt: expiresAt,
	}
}

// Invalidate removes a specific entry from cache.
func (cc *ConfigCache) Invalidate(ctx context.Context, name string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	delete(cc.entries, name)
}

// Clear removes all entries from cache.
func (cc *ConfigCache) Clear(ctx context.Context) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.entries = make(map[string]*cacheEntry)
}

// Size returns the number of cached entries.
func (cc *ConfigCache) Size() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return len(cc.entries)
}
