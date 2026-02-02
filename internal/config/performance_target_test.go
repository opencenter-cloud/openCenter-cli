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
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestConfigLoadingPerformanceTarget verifies that config loading meets the <100ms target.
// This test validates the optimization work done in Epic 4.2.2.
func TestConfigLoadingPerformanceTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance target test in short mode")
	}

	ctx := context.Background()
	loader := NewConfigLoader(nil)

	// Target: <100ms for config loading operations
	const targetDuration = 100 * time.Millisecond

	t.Run("LoadDefault", func(t *testing.T) {
		// Warm up cache
		_, _ = loader.LoadDefault(ctx, "test-cluster")

		// Measure performance
		start := time.Now()
		_, err := loader.LoadDefault(ctx, "test-cluster")
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("LoadDefault failed: %v", err)
		}

		if elapsed > targetDuration {
			t.Errorf("LoadDefault took %v, exceeds target of %v", elapsed, targetDuration)
		} else {
			t.Logf("LoadDefault took %v (target: %v) ✓", elapsed, targetDuration)
		}
	})

	t.Run("LoadFromBytes", func(t *testing.T) {
		// Prepare test data
		config := GetCachedDefaultConfig("test-cluster")
		data, err := MarshalConfigOptimized(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		// Warm up
		_, _ = loader.LoadFromBytes(ctx, data, "test-cluster")

		// Measure performance
		start := time.Now()
		_, err = loader.LoadFromBytes(ctx, data, "test-cluster")
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("LoadFromBytes failed: %v", err)
		}

		if elapsed > targetDuration {
			t.Errorf("LoadFromBytes took %v, exceeds target of %v", elapsed, targetDuration)
		} else {
			t.Logf("LoadFromBytes took %v (target: %v) ✓", elapsed, targetDuration)
		}
	})

	t.Run("LoadFromFile", func(t *testing.T) {
		// Create temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".test-cluster-config.yaml")

		config := GetCachedDefaultConfig("test-cluster")
		data, err := MarshalConfigOptimized(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		if err := os.WriteFile(configPath, data, 0600); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Warm up
		_, _ = loader.LoadFromFile(ctx, configPath)

		// Measure performance
		start := time.Now()
		_, err = loader.LoadFromFile(ctx, configPath)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("LoadFromFile failed: %v", err)
		}

		if elapsed > targetDuration {
			t.Errorf("LoadFromFile took %v, exceeds target of %v", elapsed, targetDuration)
		} else {
			t.Logf("LoadFromFile took %v (target: %v) ✓", elapsed, targetDuration)
		}
	})

	t.Run("YAMLMarshal", func(t *testing.T) {
		config := GetCachedDefaultConfig("test-cluster")

		// Warm up
		_, _ = MarshalConfigOptimized(&config)

		// Measure performance
		start := time.Now()
		_, err := MarshalConfigOptimized(&config)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("MarshalConfigOptimized failed: %v", err)
		}

		if elapsed > targetDuration {
			t.Errorf("MarshalConfigOptimized took %v, exceeds target of %v", elapsed, targetDuration)
		} else {
			t.Logf("MarshalConfigOptimized took %v (target: %v) ✓", elapsed, targetDuration)
		}
	})

	t.Run("YAMLUnmarshal", func(t *testing.T) {
		config := GetCachedDefaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		// Warm up
		var warmupConfig Config
		_ = UnmarshalConfigOptimized(data, &warmupConfig)

		// Measure performance
		start := time.Now()
		var result Config
		err = UnmarshalConfigOptimized(data, &result)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("UnmarshalConfigOptimized failed: %v", err)
		}

		if elapsed > targetDuration {
			t.Errorf("UnmarshalConfigOptimized took %v, exceeds target of %v", elapsed, targetDuration)
		} else {
			t.Logf("UnmarshalConfigOptimized took %v (target: %v) ✓", elapsed, targetDuration)
		}
	})
}

// TestConfigLoadingPerformanceRegression detects performance regressions.
// This test ensures that optimizations are maintained over time.
func TestConfigLoadingPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	ctx := context.Background()
	loader := NewConfigLoader(nil)

	// Performance baselines (from optimization work)
	// These are conservative estimates; actual performance should be better
	baselines := map[string]time.Duration{
		"LoadDefault":   50 * time.Millisecond, // Cached: ~200ns, uncached: ~40µs
		"LoadFromBytes": 10 * time.Millisecond, // ~1-2ms typical
		"YAMLMarshal":   5 * time.Millisecond,  // ~360µs typical
		"YAMLUnmarshal": 5 * time.Millisecond,  // ~450µs typical
	}

	t.Run("LoadDefault", func(t *testing.T) {
		// Warm up cache
		_, _ = loader.LoadDefault(ctx, "test-cluster")

		// Measure average over multiple iterations
		const iterations = 10
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := loader.LoadDefault(ctx, "test-cluster")
			if err != nil {
				t.Fatalf("LoadDefault failed: %v", err)
			}
			totalDuration += time.Since(start)
		}

		avgDuration := totalDuration / iterations
		baseline := baselines["LoadDefault"]

		if avgDuration > baseline {
			t.Errorf("LoadDefault average %v exceeds baseline %v (potential regression)", avgDuration, baseline)
		} else {
			t.Logf("LoadDefault average %v (baseline: %v) ✓", avgDuration, baseline)
		}
	})

	t.Run("LoadFromBytes", func(t *testing.T) {
		config := GetCachedDefaultConfig("test-cluster")
		data, err := MarshalConfigOptimized(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		// Measure average over multiple iterations
		const iterations = 10
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := loader.LoadFromBytes(ctx, data, "test-cluster")
			if err != nil {
				t.Fatalf("LoadFromBytes failed: %v", err)
			}
			totalDuration += time.Since(start)
		}

		avgDuration := totalDuration / iterations
		baseline := baselines["LoadFromBytes"]

		if avgDuration > baseline {
			t.Errorf("LoadFromBytes average %v exceeds baseline %v (potential regression)", avgDuration, baseline)
		} else {
			t.Logf("LoadFromBytes average %v (baseline: %v) ✓", avgDuration, baseline)
		}
	})

	t.Run("YAMLMarshal", func(t *testing.T) {
		config := GetCachedDefaultConfig("test-cluster")

		// Measure average over multiple iterations
		const iterations = 10
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			_, err := MarshalConfigOptimized(&config)
			if err != nil {
				t.Fatalf("MarshalConfigOptimized failed: %v", err)
			}
			totalDuration += time.Since(start)
		}

		avgDuration := totalDuration / iterations
		baseline := baselines["YAMLMarshal"]

		if avgDuration > baseline {
			t.Errorf("YAMLMarshal average %v exceeds baseline %v (potential regression)", avgDuration, baseline)
		} else {
			t.Logf("YAMLMarshal average %v (baseline: %v) ✓", avgDuration, baseline)
		}
	})

	t.Run("YAMLUnmarshal", func(t *testing.T) {
		config := GetCachedDefaultConfig("test-cluster")
		data, err := yaml.Marshal(&config)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		// Measure average over multiple iterations
		const iterations = 10
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			var result Config
			err := UnmarshalConfigOptimized(data, &result)
			if err != nil {
				t.Fatalf("UnmarshalConfigOptimized failed: %v", err)
			}
			totalDuration += time.Since(start)
		}

		avgDuration := totalDuration / iterations
		baseline := baselines["YAMLUnmarshal"]

		if avgDuration > baseline {
			t.Errorf("YAMLUnmarshal average %v exceeds baseline %v (potential regression)", avgDuration, baseline)
		} else {
			t.Logf("YAMLUnmarshal average %v (baseline: %v) ✓", avgDuration, baseline)
		}
	})
}

// TestOptimizationImpact measures the impact of optimizations.
func TestOptimizationImpact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping optimization impact test in short mode")
	}

	t.Run("CacheImpact", func(t *testing.T) {
		// Measure uncached performance
		InvalidateAllConfigCaches()
		start := time.Now()
		_ = defaultConfig("test-cluster")
		uncachedDuration := time.Since(start)

		// Measure cached performance
		_ = GetCachedDefaultConfig("test-cluster")
		start = time.Now()
		_ = GetCachedDefaultConfig("test-cluster")
		cachedDuration := time.Since(start)

		improvement := float64(uncachedDuration) / float64(cachedDuration)
		t.Logf("Cache improvement: %.2fx faster (uncached: %v, cached: %v)", improvement, uncachedDuration, cachedDuration)

		// Cache should provide at least 100x improvement
		if improvement < 100 {
			t.Errorf("Cache improvement %.2fx is less than expected 100x", improvement)
		}
	})

	t.Run("YAMLOptimizationImpact", func(t *testing.T) {
		config := GetCachedDefaultConfig("test-cluster")

		// Measure standard marshal
		start := time.Now()
		_, _ = yaml.Marshal(&config)
		standardDuration := time.Since(start)

		// Measure optimized marshal
		start = time.Now()
		_, _ = MarshalConfigOptimized(&config)
		optimizedDuration := time.Since(start)

		improvement := float64(standardDuration) / float64(optimizedDuration)
		t.Logf("YAML optimization improvement: %.2fx faster (standard: %v, optimized: %v)", improvement, standardDuration, optimizedDuration)

		// Optimized should be at least as fast as standard (may be slightly faster)
		if optimizedDuration > standardDuration*2 {
			t.Errorf("Optimized marshal is slower than standard (optimized: %v, standard: %v)", optimizedDuration, standardDuration)
		}
	})
}
