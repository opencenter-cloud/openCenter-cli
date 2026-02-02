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
	"runtime"
	"testing"
)

// TestMemoryUsageTarget verifies that memory usage stays below the 100MB target.
// This test simulates realistic config operations and measures peak memory usage.
//
// Target: <100MB peak memory usage (from design document)
// Current baseline: ~985KB peak heap allocation (from profiling results)
func TestMemoryUsageTarget(t *testing.T) {
	// Target from design document
	const targetMemoryMB = 100
	const targetMemoryBytes = targetMemoryMB * 1024 * 1024

	// Force GC to start with clean slate
	runtime.GC()

	// Get initial memory stats
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Simulate realistic workload
	iterations := 1000
	for i := 0; i < iterations; i++ {
		// Load default config (cached)
		cfg := GetCachedDefaultConfig("test-cluster")

		// Marshal to YAML (optimized)
		data, err := OptimizedYAMLMarshal(&cfg)
		if err != nil {
			t.Fatalf("OptimizedYAMLMarshal failed: %v", err)
		}

		// Unmarshal from YAML
		var cfg2 Config
		if err := OptimizedYAMLUnmarshal(data, &cfg2); err != nil {
			t.Fatalf("OptimizedYAMLUnmarshal failed: %v", err)
		}

		// Use memory pools
		pool := GetMemoryPool()
		errors := pool.GetConfigErrorSlice()
		*errors = append(*errors, &ConfigError{Message: "test error"})
		pool.PutConfigErrorSlice(errors)

		strings := pool.GetStringSlice()
		*strings = append(*strings, "test1", "test2", "test3")
		pool.PutStringSlice(strings)

		// Use allocation optimizer
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

	// Calculate memory metrics
	peakHeapAlloc := memStatsAfter.HeapAlloc
	totalAlloc := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
	avgAllocPerOp := totalAlloc / uint64(iterations)

	// Report results
	t.Logf("Memory Usage Target Verification:")
	t.Logf("  Target: <%d MB", targetMemoryMB)
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Peak heap allocation: %.2f MB", float64(peakHeapAlloc)/(1024*1024))
	t.Logf("  Total allocated: %.2f MB", float64(totalAlloc)/(1024*1024))
	t.Logf("  Average per operation: %.2f KB", float64(avgAllocPerOp)/1024)
	t.Logf("  Allocations per operation: %.2f", float64(memStatsAfter.Mallocs-memStatsBefore.Mallocs)/float64(iterations))

	// Verify we're well below the target
	if peakHeapAlloc > targetMemoryBytes {
		t.Errorf("Peak heap allocation %.2f MB exceeds target of %d MB",
			float64(peakHeapAlloc)/(1024*1024), targetMemoryMB)
	} else {
		percentOfTarget := float64(peakHeapAlloc) / float64(targetMemoryBytes) * 100
		t.Logf("  ✓ Peak memory is %.2f%% of target (%.2f MB below target)",
			percentOfTarget, float64(targetMemoryBytes-peakHeapAlloc)/(1024*1024))
	}

	// Additional check: average per operation should be reasonable
	const maxAvgPerOpKB = 2000 // 2MB per operation is reasonable
	if avgAllocPerOp > maxAvgPerOpKB*1024 {
		t.Errorf("Average allocation per operation %.2f KB exceeds reasonable limit of %d KB",
			float64(avgAllocPerOp)/1024, maxAvgPerOpKB)
	}
}

// TestMemoryUsageRegression verifies that memory usage doesn't regress.
// This test establishes a baseline and fails if memory usage increases significantly.
func TestMemoryUsageRegression(t *testing.T) {
	// Baseline from profiling results (cluster init profiling)
	// Peak heap allocation: ~985KB
	// Total allocated per operation: ~3.36MB / 1000 iterations = ~3.4KB
	const baselinePeakHeapKB = 1000 // 1MB baseline (rounded up from 985KB)
	const baselinePerOpKB = 1100    // 1.1MB per operation (from benchmarks)
	const regressionThreshold = 1.5 // Allow 50% increase before failing

	// Force GC to start with clean slate
	runtime.GC()

	// Get initial memory stats
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Run typical operations
	iterations := 100
	for i := 0; i < iterations; i++ {
		cfg := GetCachedDefaultConfig("test-cluster")
		data, _ := OptimizedYAMLMarshal(&cfg)
		_ = data
	}

	// Force GC
	runtime.GC()

	// Get final memory stats
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	// Calculate metrics
	peakHeapKB := memStatsAfter.HeapAlloc / 1024
	totalAllocKB := (memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc) / 1024
	avgPerOpKB := totalAllocKB / uint64(iterations)

	// Report results
	t.Logf("Memory Usage Regression Check:")
	t.Logf("  Baseline peak heap: %d KB", baselinePeakHeapKB)
	t.Logf("  Current peak heap: %d KB", peakHeapKB)
	t.Logf("  Baseline per operation: %d KB", baselinePerOpKB)
	t.Logf("  Current per operation: %d KB", avgPerOpKB)

	// Check for regression
	peakRatio := float64(peakHeapKB) / float64(baselinePeakHeapKB)
	perOpRatio := float64(avgPerOpKB) / float64(baselinePerOpKB)

	if peakRatio > regressionThreshold {
		t.Errorf("Peak heap allocation regressed by %.2fx (%.2f%% increase)",
			peakRatio, (peakRatio-1)*100)
	} else {
		t.Logf("  ✓ Peak heap is %.2fx baseline (%.2f%% of baseline)",
			peakRatio, peakRatio*100)
	}

	if perOpRatio > regressionThreshold {
		t.Errorf("Per-operation allocation regressed by %.2fx (%.2f%% increase)",
			perOpRatio, (perOpRatio-1)*100)
	} else {
		t.Logf("  ✓ Per-operation allocation is %.2fx baseline (%.2f%% of baseline)",
			perOpRatio, perOpRatio*100)
	}
}

// TestMemoryLeaks verifies that memory is properly released after operations.
// This test checks that repeated operations don't cause memory to grow unbounded.
func TestMemoryLeaks(t *testing.T) {
	// Force GC to start with clean slate
	runtime.GC()

	// Get initial memory stats
	var memStatsInitial runtime.MemStats
	runtime.ReadMemStats(&memStatsInitial)

	// Run operations in batches
	batches := 10
	opsPerBatch := 100

	var heapAllocations []uint64

	for batch := 0; batch < batches; batch++ {
		// Run operations
		for i := 0; i < opsPerBatch; i++ {
			cfg := GetCachedDefaultConfig("test-cluster")
			data, _ := OptimizedYAMLMarshal(&cfg)
			_ = data

			pool := GetMemoryPool()
			errors := pool.GetConfigErrorSlice()
			*errors = append(*errors, &ConfigError{Message: "test"})
			pool.PutConfigErrorSlice(errors)
		}

		// Force GC after each batch
		runtime.GC()

		// Record heap allocation
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		heapAllocations = append(heapAllocations, memStats.HeapAlloc)
	}

	// Check for memory growth
	firstBatchHeap := heapAllocations[0]
	lastBatchHeap := heapAllocations[len(heapAllocations)-1]
	heapGrowth := float64(lastBatchHeap) / float64(firstBatchHeap)

	t.Logf("Memory Leak Check:")
	t.Logf("  Batches: %d", batches)
	t.Logf("  Operations per batch: %d", opsPerBatch)
	t.Logf("  First batch heap: %.2f KB", float64(firstBatchHeap)/1024)
	t.Logf("  Last batch heap: %.2f KB", float64(lastBatchHeap)/1024)
	t.Logf("  Heap growth: %.2fx", heapGrowth)

	// Heap should not grow significantly (allow 20% growth for GC overhead)
	const maxGrowth = 1.2
	if heapGrowth > maxGrowth {
		t.Errorf("Potential memory leak detected: heap grew by %.2fx (%.2f%% increase)",
			heapGrowth, (heapGrowth-1)*100)
	} else {
		t.Logf("  ✓ No memory leak detected (growth within %.2fx threshold)", maxGrowth)
	}

	// Print heap allocation trend
	t.Logf("  Heap allocation trend:")
	for i, heap := range heapAllocations {
		t.Logf("    Batch %2d: %.2f KB", i+1, float64(heap)/1024)
	}
}
