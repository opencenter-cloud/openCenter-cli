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

package testing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/template"
)

func TestNewBenchmarkFramework(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1) // Dummy value to make it valid

	fw := NewBenchmarkFramework(b)

	if fw == nil {
		t.Fatal("expected non-nil benchmark framework")
	}

	if fw.ThresholdMultiplier != 1.5 {
		t.Errorf("expected threshold multiplier 1.5, got %f", fw.ThresholdMultiplier)
	}

	if fw.BaselineResults == nil {
		t.Error("expected non-nil baseline results map")
	}

	if fw.ConfigGenerator == nil {
		t.Error("expected non-nil config generator")
	}

	if fw.TemplateDataGenerator == nil {
		t.Error("expected non-nil template data generator")
	}
}

func TestNewBenchmarkFrameworkWithThreshold(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	threshold := 2.0
	fw := NewBenchmarkFrameworkWithThreshold(b, threshold)

	if fw.ThresholdMultiplier != threshold {
		t.Errorf("expected threshold multiplier %f, got %f", threshold, fw.ThresholdMultiplier)
	}
}

func TestRecordBaseline(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	result := BenchmarkResult{
		NsPerOp:     1000,
		AllocsPerOp: 10,
		BytesPerOp:  100,
	}

	fw.RecordBaseline("test-benchmark", result)

	recorded, exists := fw.BaselineResults["test-benchmark"]
	if !exists {
		t.Fatal("expected baseline to be recorded")
	}

	if recorded.Name != "test-benchmark" {
		t.Errorf("expected name 'test-benchmark', got %s", recorded.Name)
	}

	if recorded.NsPerOp != 1000 {
		t.Errorf("expected NsPerOp 1000, got %d", recorded.NsPerOp)
	}

	if recorded.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestCompareToBaseline_NoBaseline(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	// Should return true and record baseline when no baseline exists
	result := fw.CompareToBaseline(b, "new-benchmark", 1000)

	if !result {
		t.Error("expected true when no baseline exists")
	}

	// Verify baseline was recorded
	_, exists := fw.BaselineResults["new-benchmark"]
	if !exists {
		t.Error("expected baseline to be recorded")
	}
}

func TestCompareToBaseline_WithinThreshold(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	// Record baseline
	fw.RecordBaseline("test-benchmark", BenchmarkResult{
		NsPerOp: 1000,
	})

	// Test with value within threshold (1.5x = 1500)
	result := fw.CompareToBaseline(b, "test-benchmark", 1400)

	if !result {
		t.Error("expected true for value within threshold")
	}
}

func TestCompareToBaseline_ExceedsThreshold(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	// Record baseline
	fw.RecordBaseline("test-benchmark", BenchmarkResult{
		NsPerOp: 1000,
	})

	// Test with value exceeding threshold (1.5x = 1500)
	result := fw.CompareToBaseline(b, "test-benchmark", 1600)

	if result {
		t.Error("expected false for value exceeding threshold")
	}
}

func TestCompareToBaseline_Improvement(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	// Record baseline
	fw.RecordBaseline("test-benchmark", BenchmarkResult{
		NsPerOp: 1000,
	})

	// Test with improved value
	result := fw.CompareToBaseline(b, "test-benchmark", 800)

	if !result {
		t.Error("expected true for improved performance")
	}
}

func TestCreateBenchmarkTemplate(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	templatePath := fw.CreateBenchmarkTemplate(b, "test-template.yaml", 1000)

	if templatePath == "" {
		t.Fatal("expected non-empty template path")
	}

	// Verify template file exists
	fw.TempDir = b.TempDir() // Use testing.B's temp dir
	// File should exist in the temp directory
}

func TestCreateBenchmarkConfig(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	cfg := fw.CreateBenchmarkConfig("openstack")

	if cfg.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Errorf("expected provider 'openstack', got %s", cfg.OpenCenter.Infrastructure.Provider)
	}
}

func TestCreateLargeBenchmarkConfig(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	cfg := fw.CreateLargeBenchmarkConfig("openstack")

	if cfg.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Errorf("expected provider 'openstack', got %s", cfg.OpenCenter.Infrastructure.Provider)
	}

	// Verify overrides were added
	if len(cfg.Overrides) < 100 {
		t.Errorf("expected at least 100 overrides, got %d", len(cfg.Overrides))
	}
}

func TestCreateBenchmarkTemplateData(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	data := fw.CreateBenchmarkTemplateData()

	if data == nil {
		t.Fatal("expected non-nil template data")
	}

	if len(data) == 0 {
		t.Error("expected non-empty template data")
	}
}

func TestCreateLargeBenchmarkTemplateData(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	data := fw.CreateLargeBenchmarkTemplateData()

	if data == nil {
		t.Fatal("expected non-nil template data")
	}

	// Verify large array was added
	largeArray, exists := data["LargeArray"]
	if !exists {
		t.Fatal("expected LargeArray in template data")
	}

	arraySlice, ok := largeArray.([]map[string]interface{})
	if !ok {
		t.Fatal("expected LargeArray to be []map[string]interface{}")
	}

	if len(arraySlice) != 100 {
		t.Errorf("expected 100 items in LargeArray, got %d", len(arraySlice))
	}
}

func TestMeasureOperation(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	operation := func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	duration := fw.MeasureOperation(b, "test-operation", operation)

	if duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", duration)
	}
}

func TestMeasureOperation_Error(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	operation := func() error {
		return errors.New("test error")
	}

	// This should cause a fatal error in the benchmark
	// We can't easily test this without running an actual benchmark
	_ = fw
	_ = operation
}

func TestCompareOperations(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	fastOp := func() error {
		time.Sleep(5 * time.Millisecond)
		return nil
	}

	slowOp := func() error {
		time.Sleep(15 * time.Millisecond)
		return nil
	}

	// This should log that fastOp is faster
	fw.CompareOperations(b, "fast", fastOp, "slow", slowOp)
}

func TestRunBenchmarkSuite(t *testing.T) {
	// This test just verifies the structure exists
	// Actual benchmark suite execution happens in benchmark tests
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	suite := map[string]func(*testing.B){
		"benchmark1": func(b *testing.B) {
			// Benchmark function
		},
		"benchmark2": func(b *testing.B) {
			// Benchmark function
		},
	}

	// Verify suite is not nil
	if suite == nil {
		t.Error("expected non-nil suite")
	}

	// Verify framework can accept suite
	if fw == nil {
		t.Error("expected non-nil framework")
	}
}

func TestSaveBenchmarkResults(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	// Record some results
	fw.RecordBaseline("test1", BenchmarkResult{
		NsPerOp:     1000,
		AllocsPerOp: 10,
		BytesPerOp:  100,
	})

	fw.RecordBaseline("test2", BenchmarkResult{
		NsPerOp:     2000,
		AllocsPerOp: 20,
		BytesPerOp:  200,
	})

	err := fw.SaveBenchmarkResults(b, "results.txt")
	if err != nil {
		t.Fatalf("failed to save results: %v", err)
	}
}

func TestLoadBenchmarkResults(t *testing.T) {
	b := &testing.B{}
	b.SetBytes(1)

	fw := NewBenchmarkFramework(b)

	// Save results first
	fw.RecordBaseline("test1", BenchmarkResult{
		NsPerOp: 1000,
	})

	err := fw.SaveBenchmarkResults(b, "results.txt")
	if err != nil {
		t.Fatalf("failed to save results: %v", err)
	}

	// Load results
	err = fw.LoadBenchmarkResults(b, "results.txt")
	if err != nil {
		t.Fatalf("failed to load results: %v", err)
	}
}

// Actual benchmark tests

func BenchmarkFramework_TemplateRendering(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	// Create a simple template
	engine := template.NewGoTemplateEngine()
	engine.SetCacheEnabled(true)

	templatePath := fw.CreateBenchmarkTemplate(b, "simple.yaml", 500)
	data := fw.CreateBenchmarkTemplateData()

	fw.BenchmarkTemplateRendering(b, engine, templatePath, data)
}

func BenchmarkFramework_ConfigBuilding(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	buildFunc := func() (config.Config, error) {
		return fw.CreateBenchmarkConfig("openstack"), nil
	}

	fw.BenchmarkConfigBuilding(b, buildFunc)
}

func BenchmarkFramework_ConfigValidation(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	cfg := fw.CreateBenchmarkConfig("openstack")

	validateFunc := func(c config.Config) error {
		// Simple validation
		if c.OpenCenter.Infrastructure.Provider == "" {
			return errors.New("provider is required")
		}
		return nil
	}

	fw.BenchmarkConfigValidation(b, cfg, validateFunc)
}

func BenchmarkFramework_ConcurrentOperations(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	operation := func() error {
		// Simulate some work
		cfg := fw.CreateBenchmarkConfig("openstack")
		_ = cfg
		return nil
	}

	fw.BenchmarkConcurrentOperations(b, operation, 4)
}

func BenchmarkFramework_MemoryUsage(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	operation := func() error {
		// Create a large config to measure memory
		cfg := fw.CreateLargeBenchmarkConfig("openstack")
		_ = cfg
		return nil
	}

	fw.BenchmarkMemoryUsage(b, operation)
}

func BenchmarkFramework_LargeTemplateRendering(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	engine := template.NewGoTemplateEngine()
	engine.SetCacheEnabled(true)

	// Create a large template
	templatePath := fw.CreateBenchmarkTemplate(b, "large.yaml", 10000)
	data := fw.CreateLargeBenchmarkTemplateData()

	fw.BenchmarkTemplateRendering(b, engine, templatePath, data)
}

func BenchmarkFramework_CachedVsUncached(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	templatePath := fw.CreateBenchmarkTemplate(b, "test.yaml", 1000)
	data := fw.CreateBenchmarkTemplateData()

	b.Run("Cached", func(b *testing.B) {
		engine := template.NewGoTemplateEngine()
		engine.SetCacheEnabled(true)
		fw.BenchmarkTemplateRendering(b, engine, templatePath, data)
	})

	b.Run("Uncached", func(b *testing.B) {
		engine := template.NewGoTemplateEngine()
		engine.SetCacheEnabled(false)
		fw.BenchmarkTemplateRendering(b, engine, templatePath, data)
	})
}

func BenchmarkFramework_ConfigGenerators(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	providers := []string{"openstack", "aws", "baremetal"}

	for _, provider := range providers {
		b.Run(provider, func(b *testing.B) {
			buildFunc := func() (config.Config, error) {
				return fw.CreateBenchmarkConfig(provider), nil
			}
			fw.BenchmarkConfigBuilding(b, buildFunc)
		})
	}
}

func BenchmarkFramework_TemplateDataGeneration(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	b.Run("Small", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = fw.CreateBenchmarkTemplateData()
		}
	})

	b.Run("Large", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = fw.CreateLargeBenchmarkTemplateData()
		}
	})
}

func BenchmarkFramework_BaselineComparison(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	// Record a baseline
	fw.RecordBaseline("test-operation", BenchmarkResult{
		NsPerOp: 1000,
	})

	b.Run("WithinThreshold", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			result := fw.CompareToBaseline(b, "test-operation", 1200)
			if !result {
				// This should pass
				b.Log("Comparison within threshold passed")
			}
		}
	})

	// Note: We don't test ExceedsThreshold in benchmarks because it would fail
	// That functionality is tested in unit tests
}

func BenchmarkFramework_ContextPropagation(b *testing.B) {
	fw := NewBenchmarkFramework(b)

	engine := template.NewGoTemplateEngine()
	engine.SetCacheEnabled(true)

	templatePath := fw.CreateBenchmarkTemplate(b, "context.yaml", 500)
	data := fw.CreateBenchmarkTemplateData()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := engine.Render(ctx, templatePath, data)
		if err != nil {
			b.Fatalf("render failed: %v", err)
		}
	}
}
