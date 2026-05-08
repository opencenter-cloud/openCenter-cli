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
	"testing"
	"time"
)

// BenchmarkPathResolver_Resolve benchmarks path resolution
func BenchmarkPathResolver_Resolve(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(b, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathResolver_ResolveCached benchmarks cached path resolution
func BenchmarkPathResolver_ResolveCached(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(b, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	// Warm up cache
	_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathResolver_ResolveWithFallback benchmarks fallback resolution
func BenchmarkPathResolver_ResolveWithFallback(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(b, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveWithFallback(ctx, "test-cluster")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathResolver_ResolveWithFallbackCached benchmarks cached fallback resolution
func BenchmarkPathResolver_ResolveWithFallbackCached(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(b, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	// Warm up cache
	_, err := resolver.ResolveWithFallback(ctx, "test-cluster")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveWithFallback(ctx, "test-cluster")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathResolver_GetOrganization benchmarks organization detection
func BenchmarkPathResolver_GetOrganization(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(b, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.GetOrganization(ctx, "test-cluster")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathResolver_DetectStructureType benchmarks structure type detection
func BenchmarkPathResolver_DetectStructureType(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(b, tmpDir, "opencenter", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.DetectStructureType(ctx, "test-cluster")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathResolver_CreateClusterDirectories benchmarks directory creation
func BenchmarkPathResolver_CreateClusterDirectories(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tmpDir := b.TempDir()
		resolver := NewPathResolver(tmpDir)
		b.StartTimer()

		err := resolver.CreateClusterDirectories(ctx, "test-cluster", "test-org")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathCache_Get benchmarks cache get operations
func BenchmarkPathCache_Get(b *testing.B) {
	cache := DefaultPathCache()

	testPaths := &ClusterPaths{
		OrganizationDir: "/test/org",
		ClusterDir:      "/test/org/infrastructure/clusters/test",
	}

	// Populate cache
	cache.Set("test-cluster", "test-org", "org-based", testPaths)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Get("test-cluster", "test-org")
	}
}

// BenchmarkPathCache_Set benchmarks cache set operations
func BenchmarkPathCache_Set(b *testing.B) {
	cache := DefaultPathCache()

	testPaths := &ClusterPaths{
		OrganizationDir: "/test/org",
		ClusterDir:      "/test/org/infrastructure/clusters/test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("test-cluster", "test-org", "org-based", testPaths)
	}
}

// BenchmarkPathCache_Invalidate benchmarks cache invalidation
func BenchmarkPathCache_Invalidate(b *testing.B) {
	cache := DefaultPathCache()

	testPaths := &ClusterPaths{
		OrganizationDir: "/test/org",
		ClusterDir:      "/test/org/infrastructure/clusters/test",
	}

	// Populate cache
	cache.Set("test-cluster", "test-org", "org-based", testPaths)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Invalidate("test-cluster", "test-org")
	}
}

// BenchmarkPathCache_Stats benchmarks cache statistics
func BenchmarkPathCache_Stats(b *testing.B) {
	cache := DefaultPathCache()

	testPaths := &ClusterPaths{
		OrganizationDir: "/test/org",
		ClusterDir:      "/test/org/infrastructure/clusters/test",
	}

	// Populate cache
	cache.Set("test-cluster", "test-org", "org-based", testPaths)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Stats()
	}
}

// BenchmarkOrgBasedStrategy_CanResolve benchmarks strategy resolution check
func BenchmarkOrgBasedStrategy_CanResolve(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(b, tmpDir, "test-org", "test-cluster")

	strategy := NewOrgBasedStrategy(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := strategy.CanResolve(ctx, "test-cluster", "test-org")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOrgBasedStrategy_Resolve benchmarks strategy path resolution
func BenchmarkOrgBasedStrategy_Resolve(b *testing.B) {
	tmpDir := b.TempDir()
	ctx := context.Background()

	strategy := NewOrgBasedStrategy(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := strategy.Resolve(ctx, "test-cluster", "test-org")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPathResolver_ValidateClusterName benchmarks cluster name validation
func BenchmarkPathResolver_ValidateClusterName(b *testing.B) {
	resolver := NewPathResolver("/tmp/test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resolver.validateClusterName("test-cluster")
	}
}

// BenchmarkPathResolver_ValidatePath benchmarks path validation
func BenchmarkPathResolver_ValidatePath(b *testing.B) {
	resolver := NewPathResolver("/tmp/test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resolver.ValidatePath("/tmp/test/cluster")
	}
}

// TestBenchmarkPerformance verifies that benchmarks meet performance targets
func TestBenchmarkPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	createSecureClusterForTest(t, tmpDir, "test-org", "test-cluster")

	resolver := NewPathResolver(tmpDir)

	// Test uncached resolution (should be < 5ms)
	start := time.Now()
	_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatal(err)
	}
	uncachedDuration := time.Since(start)

	if uncachedDuration > 5*time.Millisecond {
		t.Errorf("uncached resolution took %v, want < 5ms", uncachedDuration)
	} else {
		t.Logf("uncached resolution: %v (target: < 5ms)", uncachedDuration)
	}

	// Test cached resolution (should be < 100μs)
	start = time.Now()
	_, err = resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatal(err)
	}
	cachedDuration := time.Since(start)

	if cachedDuration > 100*time.Microsecond {
		t.Errorf("cached resolution took %v, want < 100μs", cachedDuration)
	} else {
		t.Logf("cached resolution: %v (target: < 100μs)", cachedDuration)
	}

	// Test average over multiple iterations
	iterations := 1000
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
		if err != nil {
			t.Fatal(err)
		}
	}
	avgDuration := time.Since(start) / time.Duration(iterations)

	if avgDuration > 100*time.Microsecond {
		t.Errorf("average cached resolution took %v, want < 100μs", avgDuration)
	} else {
		t.Logf("average cached resolution over %d iterations: %v (target: < 100μs)", iterations, avgDuration)
	}
}
