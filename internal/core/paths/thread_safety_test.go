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
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPathCache_ConcurrentAccess tests concurrent cache operations
func TestPathCache_ConcurrentAccess(t *testing.T) {
	cache := DefaultPathCache()

	// Create test paths
	testPaths := &ClusterPaths{
		OrganizationDir: "/test/org",
		ClusterDir:      "/test/org/infrastructure/clusters/test",
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 100

	// Concurrent Set operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Set("test-cluster", "test-org", "org-based", testPaths)
			}
		}(i)
	}

	// Concurrent Get operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = cache.Get("test-cluster", "test-org")
			}
		}(i)
	}

	// Concurrent Invalidate operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Invalidate("test-cluster", "test-org")
			}
		}(i)
	}

	// Concurrent Stats operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = cache.Stats()
			}
		}(i)
	}

	wg.Wait()

	// If we get here without deadlock or race conditions, test passes
	t.Log("Concurrent cache access test completed successfully")
}

// TestPathCache_ConcurrentCleanup tests concurrent cleanup operations
func TestPathCache_ConcurrentCleanup(t *testing.T) {
	cache := NewPathCache(100*time.Millisecond, 100)

	testPaths := &ClusterPaths{
		OrganizationDir: "/test/org",
		ClusterDir:      "/test/org/infrastructure/clusters/test",
	}

	var wg sync.WaitGroup
	numGoroutines := 20

	// Add entries
	for i := 0; i < 50; i++ {
		cache.Set("cluster-"+string(rune(i)), "test-org", "org-based", testPaths)
	}

	// Concurrent cleanup operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cache.CleanupExpired()
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Concurrent access during cleanup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cache.Get("cluster-1", "test-org")
			}
		}()
	}

	wg.Wait()
	t.Log("Concurrent cleanup test completed successfully")
}

// TestPathResolver_ConcurrentResolve tests concurrent path resolution
func TestPathResolver_ConcurrentResolve(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup multiple clusters
	clusters := []string{"cluster1", "cluster2", "cluster3", "cluster4", "cluster5"}
	orgs := []string{"org1", "org2"}

	for _, org := range orgs {
		for _, cluster := range clusters {
			createSecureClusterForTest(t, tmpDir, org, cluster)
		}
	}

	resolver := NewPathResolver(tmpDir)

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var errorCount atomic.Int64
	numGoroutines := 50
	numOperations := 100

	// Concurrent Resolve operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cluster := clusters[j%len(clusters)]
				org := orgs[j%len(orgs)]
				_, err := resolver.Resolve(ctx, cluster, org)
				if err != nil {
					errorCount.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations succeeded
	if errorCount.Load() > 0 {
		t.Errorf("expected 0 errors, got %d", errorCount.Load())
	}

	expectedSuccess := int64(numGoroutines * numOperations)
	if successCount.Load() != expectedSuccess {
		t.Errorf("expected %d successful operations, got %d", expectedSuccess, successCount.Load())
	}

	t.Logf("Concurrent resolve test completed: %d successful operations", successCount.Load())
}

// TestPathResolver_ConcurrentResolveWithFallback tests concurrent fallback resolution
func TestPathResolver_ConcurrentResolveWithFallback(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup clusters in different organizations
	clusters := []string{"cluster1", "cluster2", "cluster3"}
	orgs := []string{"org-alpha", "org-beta", "org-gamma"}

	for i, cluster := range clusters {
		org := orgs[i%len(orgs)]
		createSecureClusterForTest(t, tmpDir, org, cluster)
	}

	resolver := NewPathResolver(tmpDir)

	var wg sync.WaitGroup
	numGoroutines := 30
	numOperations := 50

	// Concurrent ResolveWithFallback operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cluster := clusters[j%len(clusters)]
				_, _ = resolver.ResolveWithFallback(ctx, cluster)
			}
		}(i)
	}

	wg.Wait()
	t.Log("Concurrent resolve with fallback test completed successfully")
}

// TestPathResolver_ConcurrentCacheOperations tests concurrent cache operations
func TestPathResolver_ConcurrentCacheOperations(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(t, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	var wg sync.WaitGroup
	numGoroutines := 40

	// Concurrent Resolve (populates cache)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = resolver.Resolve(ctx, "test-cluster", "test-org")
			}
		}()
	}

	// Concurrent InvalidateCache
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				resolver.InvalidateCache("test-cluster")
			}
		}()
	}

	// Concurrent ClearCache
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				resolver.ClearCache()
			}
		}()
	}

	// Concurrent GetCacheStats
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = resolver.GetCacheStats()
			}
		}()
	}

	wg.Wait()
	t.Log("Concurrent cache operations test completed successfully")
}

// TestPathResolver_ConcurrentCreateDirectories tests concurrent directory creation
func TestPathResolver_ConcurrentCreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	resolver := NewPathResolver(tmpDir)

	var wg sync.WaitGroup
	numGoroutines := 20
	clusters := []string{"cluster1", "cluster2", "cluster3", "cluster4", "cluster5"}

	// Concurrent CreateClusterDirectories operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cluster := clusters[id%len(clusters)]
			org := "test-org"
			_ = resolver.CreateClusterDirectories(ctx, cluster, org)
		}(i)
	}

	wg.Wait()

	// Verify all directories were created
	for _, cluster := range clusters {
		clusterDir := filepath.Join(tmpDir, "gitops", "test-org", "infrastructure", "clusters", cluster)
		if stat, err := os.Stat(clusterDir); err != nil {
			t.Errorf("cluster directory %s was not created: %v", clusterDir, err)
		} else if !stat.IsDir() {
			t.Errorf("path %s exists but is not a directory", clusterDir)
		}
	}

	t.Log("Concurrent create directories test completed successfully")
}

// TestPathResolver_ConcurrentGetters tests concurrent getter operations
func TestPathResolver_ConcurrentGetters(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(t, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 100

	// Concurrent GetBaseDir
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = resolver.GetBaseDir()
			}
		}()
	}

	// Concurrent GetStrategies
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = resolver.GetStrategies()
			}
		}()
	}

	// Concurrent GetOptions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = resolver.GetOptions()
			}
		}()
	}

	// Concurrent GetOrganization
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, _ = resolver.GetOrganization(ctx, "test-cluster")
			}
		}()
	}

	// Concurrent DetectStructureType
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, _ = resolver.DetectStructureType(ctx, "test-cluster")
			}
		}()
	}

	wg.Wait()
	t.Log("Concurrent getters test completed successfully")
}

// TestPathCache_ConcurrentEviction tests concurrent cache eviction
func TestPathCache_ConcurrentEviction(t *testing.T) {
	// Create cache with small max size to trigger eviction
	cache := NewPathCache(5*time.Minute, 10)

	testPaths := &ClusterPaths{
		OrganizationDir: "/test/org",
		ClusterDir:      "/test/org/infrastructure/clusters/test",
	}

	var wg sync.WaitGroup
	numGoroutines := 20

	// Concurrent Set operations that will trigger eviction
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				clusterName := "cluster-" + string(rune('a'+id)) + string(rune('0'+j%10))
				cache.Set(clusterName, "test-org", "org-based", testPaths)
			}
		}(i)
	}

	// Concurrent Get operations during eviction
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				clusterName := "cluster-" + string(rune('a'+id)) + string(rune('0'+j%10))
				_ = cache.Get(clusterName, "test-org")
			}
		}(i)
	}

	wg.Wait()

	// Verify cache size is within max size
	stats := cache.Stats()
	if stats.Entries > 10 {
		t.Errorf("cache size %d exceeds max size 10", stats.Entries)
	}

	t.Log("Concurrent eviction test completed successfully")
}

// TestPathResolver_RaceConditions tests for race conditions using Go's race detector
func TestPathResolver_RaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race condition test in short mode")
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup test clusters
	clusters := []string{"cluster0", "cluster1", "cluster2", "cluster3", "cluster4"}
	for _, cluster := range clusters {
		clusterDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", cluster)
		if err := os.MkdirAll(clusterDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	resolver := NewPathResolver(tmpDir)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Mix of all operations to detect race conditions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cluster := clusters[id%len(clusters)]

			// Resolve
			_, _ = resolver.Resolve(ctx, cluster, "test-org")

			// ResolveWithFallback
			_, _ = resolver.ResolveWithFallback(ctx, cluster)

			// GetOrganization
			_, _ = resolver.GetOrganization(ctx, cluster)

			// DetectStructureType
			_, _ = resolver.DetectStructureType(ctx, cluster)

			// InvalidateCache
			resolver.InvalidateCache(cluster)

			// GetCacheStats
			_ = resolver.GetCacheStats()

			// GetBaseDir
			_ = resolver.GetBaseDir()

			// GetStrategies
			_ = resolver.GetStrategies()

			// GetOptions
			_ = resolver.GetOptions()
		}(i)
	}

	wg.Wait()
	t.Log("Race condition test completed successfully")
}
