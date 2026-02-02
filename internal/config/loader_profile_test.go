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
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestConfigLoadingCPUProfile profiles CPU usage during config loading operations
func TestConfigLoadingCPUProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CPU profiling test in short mode")
	}

	// Create CPU profile file in current directory
	cpuProfilePath := "config_loading_cpu.prof"
	f, err := os.Create(cpuProfilePath)
	if err != nil {
		t.Fatalf("Failed to create CPU profile: %v", err)
	}
	defer f.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(f); err != nil {
		t.Fatalf("Failed to start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	// Run config loading operations
	ctx := context.Background()
	loader := NewConfigLoader(nil)

	// Test 1: Load default config (baseline)
	t.Run("LoadDefault", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			_, err := loader.LoadDefault(ctx, "test-cluster")
			if err != nil {
				t.Errorf("LoadDefault failed: %v", err)
			}
		}
	})

	// Test 2: Generate complete config (with defaults)
	t.Run("GenerateCompleteConfig", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			_, err := loader.GenerateCompleteConfig(ctx, "test-cluster")
			if err != nil {
				t.Errorf("GenerateCompleteConfig failed: %v", err)
			}
		}
	})

	// Test 3: Load from bytes (YAML parsing)
	t.Run("LoadFromBytes", func(t *testing.T) {
		config := defaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		for i := 0; i < 100; i++ {
			_, err := loader.LoadFromBytes(ctx, data, "test-cluster")
			if err != nil {
				t.Errorf("LoadFromBytes failed: %v", err)
			}
		}
	})

	// Test 4: Load from file (I/O + parsing)
	t.Run("LoadFromFile", func(t *testing.T) {
		// Create temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".test-cluster-config.yaml")

		config := defaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		if err := os.WriteFile(configPath, data, 0600); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		for i := 0; i < 100; i++ {
			_, err := loader.LoadFromFile(ctx, configPath)
			if err != nil {
				t.Errorf("LoadFromFile failed: %v", err)
			}
		}
	})

	t.Logf("CPU profile written to: %s", cpuProfilePath)
	t.Logf("Analyze with: go tool pprof %s", cpuProfilePath)
}

// TestConfigLoadingMemoryProfile profiles memory usage during config loading operations
func TestConfigLoadingMemoryProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profiling test in short mode")
	}

	ctx := context.Background()
	loader := NewConfigLoader(nil)

	// Force GC before starting
	runtime.GC()

	// Capture initial memory stats
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Run config loading operations
	const iterations = 1000

	// Test 1: Load default config
	t.Run("LoadDefault", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			_, err := loader.LoadDefault(ctx, "test-cluster")
			if err != nil {
				t.Errorf("LoadDefault failed: %v", err)
			}
		}
	})

	// Test 2: Generate complete config
	t.Run("GenerateCompleteConfig", func(t *testing.T) {
		for i := 0; i < iterations; i++ {
			_, err := loader.GenerateCompleteConfig(ctx, "test-cluster")
			if err != nil {
				t.Errorf("GenerateCompleteConfig failed: %v", err)
			}
		}
	})

	// Test 3: Load from bytes
	t.Run("LoadFromBytes", func(t *testing.T) {
		config := defaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		for i := 0; i < iterations; i++ {
			_, err := loader.LoadFromBytes(ctx, data, "test-cluster")
			if err != nil {
				t.Errorf("LoadFromBytes failed: %v", err)
			}
		}
	})

	// Force GC and capture final memory stats
	runtime.GC()
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	// Write memory profile
	memProfilePath := "config_loading_mem.prof"
	f, err := os.Create(memProfilePath)
	if err != nil {
		t.Fatalf("Failed to create memory profile: %v", err)
	}
	defer f.Close()

	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatalf("Failed to write memory profile: %v", err)
	}

	// Calculate memory usage
	allocDiff := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
	allocPerOp := allocDiff / (iterations * 3) // 3 operations

	t.Logf("Memory profile written to: %s", memProfilePath)
	t.Logf("Analyze with: go tool pprof %s", memProfilePath)
	t.Logf("\nMemory Statistics:")
	t.Logf("  Total allocations: %d bytes", allocDiff)
	t.Logf("  Allocations per operation: %d bytes", allocPerOp)
	t.Logf("  Heap objects before: %d", memStatsBefore.HeapObjects)
	t.Logf("  Heap objects after: %d", memStatsAfter.HeapObjects)
	t.Logf("  Heap alloc before: %d bytes", memStatsBefore.HeapAlloc)
	t.Logf("  Heap alloc after: %d bytes", memStatsAfter.HeapAlloc)
}

// BenchmarkConfigLoading benchmarks various config loading operations
func BenchmarkConfigLoading(b *testing.B) {
	ctx := context.Background()
	loader := NewConfigLoader(nil)

	b.Run("LoadDefault", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := loader.LoadDefault(ctx, "test-cluster")
			if err != nil {
				b.Fatalf("LoadDefault failed: %v", err)
			}
		}
	})

	b.Run("GenerateCompleteConfig", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := loader.GenerateCompleteConfig(ctx, "test-cluster")
			if err != nil {
				b.Fatalf("GenerateCompleteConfig failed: %v", err)
			}
		}
	})

	b.Run("LoadFromBytes", func(b *testing.B) {
		config := defaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			b.Fatalf("Failed to marshal config: %v", err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := loader.LoadFromBytes(ctx, data, "test-cluster")
			if err != nil {
				b.Fatalf("LoadFromBytes failed: %v", err)
			}
		}
	})

	b.Run("YAMLMarshal", func(b *testing.B) {
		config := defaultConfig("test-cluster")

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := yaml.Marshal(&config)
			if err != nil {
				b.Fatalf("Marshal failed: %v", err)
			}
		}
	})

	b.Run("YAMLUnmarshal", func(b *testing.B) {
		config := defaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			b.Fatalf("Failed to marshal config: %v", err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var cfg Config
			err := yaml.Unmarshal(data, &cfg)
			if err != nil {
				b.Fatalf("Unmarshal failed: %v", err)
			}
		}
	})

	b.Run("LoadFromFile", func(b *testing.B) {
		// Create temporary config file
		tmpDir := b.TempDir()
		configPath := filepath.Join(tmpDir, ".test-cluster-config.yaml")

		config := defaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			b.Fatalf("Failed to marshal config: %v", err)
		}

		if err := os.WriteFile(configPath, data, 0600); err != nil {
			b.Fatalf("Failed to write config file: %v", err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := loader.LoadFromFile(ctx, configPath)
			if err != nil {
				b.Fatalf("LoadFromFile failed: %v", err)
			}
		}
	})
}

// TestConfigLoadingHotPaths identifies hot paths in config loading
func TestConfigLoadingHotPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping hot path analysis in short mode")
	}

	ctx := context.Background()
	loader := NewConfigLoader(nil)

	// Measure time for each operation
	operations := []struct {
		name string
		fn   func() error
	}{
		{
			name: "LoadDefault",
			fn: func() error {
				_, err := loader.LoadDefault(ctx, "test-cluster")
				return err
			},
		},
		{
			name: "GenerateCompleteConfig",
			fn: func() error {
				_, err := loader.GenerateCompleteConfig(ctx, "test-cluster")
				return err
			},
		},
		{
			name: "defaultConfig",
			fn: func() error {
				_ = defaultConfig("test-cluster")
				return nil
			},
		},
		{
			name: "YAMLMarshal",
			fn: func() error {
				config := defaultConfig("test-cluster")
				_, err := yaml.Marshal(&config)
				return err
			},
		},
		{
			name: "YAMLUnmarshal",
			fn: func() error {
				config := defaultConfig("test-cluster")
				data, err := yaml.Marshal(&config)
				if err != nil {
					return err
				}
				var cfg Config
				return yaml.Unmarshal(data, &cfg)
			},
		},
		{
			name: "ReferenceResolver",
			fn: func() error {
				config := defaultConfig("test-cluster")
				resolver := NewReferenceResolver()
				return resolver.Resolve(&config)
			},
		},
	}

	t.Log("\nHot Path Analysis:")
	t.Log("==================")

	for _, op := range operations {
		// Warm up
		for i := 0; i < 10; i++ {
			_ = op.fn()
		}

		// Measure
		const iterations = 100
		start := time.Now()
		for i := 0; i < iterations; i++ {
			if err := op.fn(); err != nil {
				t.Errorf("%s failed: %v", op.name, err)
			}
		}
		elapsed := time.Since(start)

		avgTime := elapsed / iterations
		t.Logf("  %-25s: %v (avg per op)", op.name, avgTime)
	}
}
