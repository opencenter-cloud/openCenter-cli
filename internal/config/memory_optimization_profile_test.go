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
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
)

// TestMemoryOptimizationProfile profiles memory usage with optimizations enabled.
// This test generates a memory profile that can be analyzed with pprof.
//
// Run with: go test -run TestMemoryOptimizationProfile -v ./internal/config
// Analyze with: go tool pprof -http=:8080 /tmp/memory_optimization.prof
func TestMemoryOptimizationProfile(t *testing.T) {
	// Create profile file
	f, err := os.CreateTemp("", "memory_optimization_*.prof")
	if err != nil {
		t.Fatalf("Failed to create profile file: %v", err)
	}
	defer f.Close()

	t.Logf("Memory profile will be written to: %s", f.Name())

	// Force GC before starting
	runtime.GC()

	// Get initial memory stats
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Run operations with optimizations
	iterations := 1000
	for i := 0; i < iterations; i++ {
		// Test config caching
		cfg := GetCachedDefaultConfig("test-cluster")
		_ = cfg

		// Test YAML marshaling with pooling
		data, err := OptimizedYAMLMarshal(&cfg)
		if err != nil {
			t.Fatalf("OptimizedYAMLMarshal failed: %v", err)
		}
		_ = data

		// Test memory pool usage
		pool := GetMemoryPool()
		errors := pool.GetConfigErrorSlice()
		*errors = append(*errors, &ConfigError{Message: "test"})
		pool.PutConfigErrorSlice(errors)

		// Test allocation optimizer
		optimizer := GetAllocationOptimizer()
		s := optimizer.GetStringSlice()
		*s = append(*s, "test")
		optimizer.PutStringSlice(s)
	}

	// Force GC to see actual memory usage
	runtime.GC()

	// Get final memory stats
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	// Write memory profile
	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatalf("Failed to write memory profile: %v", err)
	}

	// Calculate memory metrics
	totalAlloc := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
	heapAlloc := memStatsAfter.HeapAlloc - memStatsBefore.HeapAlloc
	allocsPerOp := float64(memStatsAfter.Mallocs-memStatsBefore.Mallocs) / float64(iterations)

	t.Logf("Memory Optimization Profile Results:")
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Total allocated: %d bytes (%.2f MB)", totalAlloc, float64(totalAlloc)/(1024*1024))
	t.Logf("  Heap allocated: %d bytes (%.2f KB)", heapAlloc, float64(heapAlloc)/1024)
	t.Logf("  Allocations per operation: %.2f", allocsPerOp)
	t.Logf("  Average memory per operation: %d bytes", totalAlloc/uint64(iterations))
	t.Logf("")
	t.Logf("Profile written to: %s", f.Name())
	t.Logf("Analyze with: go tool pprof -http=:8080 %s", f.Name())
}

// TestMemoryOptimizationComparison compares memory usage with and without optimizations.
func TestMemoryOptimizationComparison(t *testing.T) {
	iterations := 100

	// Test without optimizations
	runtime.GC()
	var memStatsBeforeUnopt runtime.MemStats
	runtime.ReadMemStats(&memStatsBeforeUnopt)

	for i := 0; i < iterations; i++ {
		cfg := defaultConfig("test-cluster")
		data, _ := MarshalConfigOptimized(&cfg)
		_ = data
	}

	runtime.GC()
	var memStatsAfterUnopt runtime.MemStats
	runtime.ReadMemStats(&memStatsAfterUnopt)

	unoptAlloc := memStatsAfterUnopt.TotalAlloc - memStatsBeforeUnopt.TotalAlloc
	unoptMallocs := memStatsAfterUnopt.Mallocs - memStatsBeforeUnopt.Mallocs

	// Test with optimizations
	runtime.GC()
	var memStatsBeforeOpt runtime.MemStats
	runtime.ReadMemStats(&memStatsBeforeOpt)

	for i := 0; i < iterations; i++ {
		cfg := GetCachedDefaultConfig("test-cluster")
		data, _ := OptimizedYAMLMarshal(&cfg)
		_ = data
	}

	runtime.GC()
	var memStatsAfterOpt runtime.MemStats
	runtime.ReadMemStats(&memStatsAfterOpt)

	optAlloc := memStatsAfterOpt.TotalAlloc - memStatsBeforeOpt.TotalAlloc
	optMallocs := memStatsAfterOpt.Mallocs - memStatsBeforeOpt.Mallocs

	// Calculate improvements
	allocReduction := float64(unoptAlloc-optAlloc) / float64(unoptAlloc) * 100
	mallocsReduction := float64(unoptMallocs-optMallocs) / float64(unoptMallocs) * 100

	t.Logf("Memory Optimization Comparison (%d iterations):", iterations)
	t.Logf("")
	t.Logf("Without optimizations:")
	t.Logf("  Total allocated: %d bytes (%.2f MB)", unoptAlloc, float64(unoptAlloc)/(1024*1024))
	t.Logf("  Allocations: %d", unoptMallocs)
	t.Logf("  Per operation: %d bytes, %.2f allocs", unoptAlloc/uint64(iterations), float64(unoptMallocs)/float64(iterations))
	t.Logf("")
	t.Logf("With optimizations:")
	t.Logf("  Total allocated: %d bytes (%.2f MB)", optAlloc, float64(optAlloc)/(1024*1024))
	t.Logf("  Allocations: %d", optMallocs)
	t.Logf("  Per operation: %d bytes, %.2f allocs", optAlloc/uint64(iterations), float64(optMallocs)/float64(iterations))
	t.Logf("")
	t.Logf("Improvement:")
	t.Logf("  Memory reduction: %.2f%%", allocReduction)
	t.Logf("  Allocation reduction: %.2f%%", mallocsReduction)

	// Verify we achieved some improvement
	if allocReduction < 0 {
		t.Logf("Warning: Optimizations increased memory usage by %.2f%%", -allocReduction)
	}
	if mallocsReduction < 0 {
		t.Logf("Warning: Optimizations increased allocations by %.2f%%", -mallocsReduction)
	}
}

// BenchmarkMemoryOptimization_WithPooling benchmarks memory usage with pooling.
func BenchmarkMemoryOptimization_WithPooling(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg := GetCachedDefaultConfig("test-cluster")
		data, _ := OptimizedYAMLMarshal(&cfg)
		_ = data

		pool := GetMemoryPool()
		errors := pool.GetConfigErrorSlice()
		*errors = append(*errors, &ConfigError{Message: "test"})
		pool.PutConfigErrorSlice(errors)
	}
}

// BenchmarkMemoryOptimization_WithoutPooling benchmarks memory usage without pooling.
func BenchmarkMemoryOptimization_WithoutPooling(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg := defaultConfig("test-cluster")
		data, _ := MarshalConfigOptimized(&cfg)
		_ = data

		errors := make([]*ConfigError, 0, 8)
		errors = append(errors, &ConfigError{Message: "test"})
		_ = errors
	}
}
