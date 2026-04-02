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

import configcache "github.com/opencenter-cloud/opencenter-cli/internal/config/cache"

// globalConfigCache is the singleton cache instance.
var globalConfigCache = configcache.NewDefaultsCache(
	func(clusterName string) Config {
		return defaultConfig(clusterName)
	},
	GenerateDefaultFromSchema,
)

// CacheStats provides statistics about the config cache.
type CacheStats = configcache.Stats

// GetCachedDefaultConfig retrieves a cached default config or generates a new one.
// This is the public API for accessing cached default configs.
//
// Performance: Reduces allocations by ~98KB per call on cache hits.
func GetCachedDefaultConfig(clusterName string) Config {
	return globalConfigCache.GetDefaultConfig(clusterName)
}

// GetCachedSchemaDefaults retrieves cached schema defaults or generates new ones.
// This is the public API for accessing cached schema defaults.
//
// Performance: Reduces allocations by ~1.1MB per call on cache hits.
func GetCachedSchemaDefaults(clusterName string) ([]byte, error) {
	return globalConfigCache.GetSchemaDefaults(clusterName)
}

// InvalidateConfigCache removes a cached config for a specific cluster.
// Call this when a cluster's configuration changes.
func InvalidateConfigCache(clusterName string) {
	globalConfigCache.Invalidate(clusterName)
}

// InvalidateAllConfigCaches clears all cached configs.
// Call this when global configuration changes or for testing.
func InvalidateAllConfigCaches() {
	globalConfigCache.InvalidateAll()
}

// GetConfigCacheStats returns cache statistics for monitoring.
func GetConfigCacheStats() CacheStats {
	return globalConfigCache.Stats()
}
