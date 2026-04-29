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
	"sync"
	"testing"
)

func TestGetCachedDefaultConfig(t *testing.T) {
	// Clear cache before test
	InvalidateAllConfigCaches()

	// First call should generate and cache
	config1 := GetCachedDefaultConfig("test-cluster")
	if config1.ClusterName() != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", config1.ClusterName())
	}

	// Second call should return cached version
	config2 := GetCachedDefaultConfig("test-cluster")
	if config2.ClusterName() != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", config2.ClusterName())
	}

	// Verify cache stats
	stats := GetConfigCacheStats()
	if stats.DefaultConfigCount != 1 {
		t.Errorf("Expected 1 cached config, got %d", stats.DefaultConfigCount)
	}
}

func TestInvalidateConfigCache(t *testing.T) {
	// Clear cache before test
	InvalidateAllConfigCaches()

	// Generate and cache
	_ = GetCachedDefaultConfig("test-cluster")

	// Verify cached
	stats := GetConfigCacheStats()
	if stats.DefaultConfigCount != 1 {
		t.Errorf("Expected 1 cached config, got %d", stats.DefaultConfigCount)
	}

	// Invalidate
	InvalidateConfigCache("test-cluster")

	// Verify cleared
	stats = GetConfigCacheStats()
	if stats.DefaultConfigCount != 0 {
		t.Errorf("Expected 0 cached configs after invalidation, got %d", stats.DefaultConfigCount)
	}
}

func TestInvalidateAllConfigCaches(t *testing.T) {
	// Clear cache before test
	InvalidateAllConfigCaches()

	// Generate and cache multiple clusters
	_ = GetCachedDefaultConfig("cluster1")
	_ = GetCachedDefaultConfig("cluster2")

	// Verify cached
	stats := GetConfigCacheStats()
	if stats.DefaultConfigCount != 2 {
		t.Errorf("Expected 2 cached configs, got %d", stats.DefaultConfigCount)
	}

	// Invalidate all
	InvalidateAllConfigCaches()

	// Verify cleared
	stats = GetConfigCacheStats()
	if stats.DefaultConfigCount != 0 {
		t.Errorf("Expected 0 cached configs after invalidation, got %d", stats.DefaultConfigCount)
	}
}

func TestCacheConcurrency(t *testing.T) {
	// Clear cache before test
	InvalidateAllConfigCaches()

	// Test concurrent access to cache
	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = GetCachedDefaultConfig("test-cluster")
			}
		}()
	}

	wg.Wait()

	// Verify cache has exactly one entry (not duplicated)
	stats := GetConfigCacheStats()
	if stats.DefaultConfigCount != 1 {
		t.Errorf("Expected 1 cached config after concurrent access, got %d", stats.DefaultConfigCount)
	}
}

func TestCacheIsolation(t *testing.T) {
	// Clear cache before test
	InvalidateAllConfigCaches()

	// Get cached config
	config1 := GetCachedDefaultConfig("test-cluster")

	// Modify the returned config
	config1.OpenCenter.Cluster.ClusterName = "modified-cluster"

	// Get cached config again
	config2 := GetCachedDefaultConfig("test-cluster")

	// Verify the cached version is not modified
	if config2.ClusterName() != "test-cluster" {
		t.Errorf("Cache was mutated: expected 'test-cluster', got '%s'", config2.ClusterName())
	}
}

func TestCacheMultipleClusters(t *testing.T) {
	// Clear cache before test
	InvalidateAllConfigCaches()

	// Cache multiple clusters
	config1 := GetCachedDefaultConfig("cluster1")
	config2 := GetCachedDefaultConfig("cluster2")
	config3 := GetCachedDefaultConfig("cluster3")

	// Verify each has correct name
	if config1.ClusterName() != "cluster1" {
		t.Errorf("Expected 'cluster1', got '%s'", config1.ClusterName())
	}
	if config2.ClusterName() != "cluster2" {
		t.Errorf("Expected 'cluster2', got '%s'", config2.ClusterName())
	}
	if config3.ClusterName() != "cluster3" {
		t.Errorf("Expected 'cluster3', got '%s'", config3.ClusterName())
	}

	// Verify cache stats
	stats := GetConfigCacheStats()
	if stats.DefaultConfigCount != 3 {
		t.Errorf("Expected 3 cached configs, got %d", stats.DefaultConfigCount)
	}
}

// BenchmarkCachePerformance compares cached vs uncached performance
func BenchmarkCachePerformance(b *testing.B) {
	b.Run("Uncached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = defaultConfig("test-cluster")
		}
	})

	b.Run("Cached", func(b *testing.B) {
		// Pre-populate cache
		_ = GetCachedDefaultConfig("test-cluster")

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = GetCachedDefaultConfig("test-cluster")
		}
	})

	b.Run("CachedColdStart", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			InvalidateAllConfigCaches()
			_ = GetCachedDefaultConfig("test-cluster")
		}
	})
}
