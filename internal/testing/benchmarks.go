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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/template"
)

// BenchmarkFramework provides utilities for performance testing and regression detection.
// It helps measure and compare performance across different implementations and versions.
type BenchmarkFramework struct {
	// BaselineResults stores baseline performance metrics for comparison
	BaselineResults map[string]BenchmarkResult

	// ThresholdMultiplier is the acceptable performance degradation multiplier
	// For example, 1.5 means 50% slower is acceptable
	ThresholdMultiplier float64

	// TempDir is the temporary directory for benchmark artifacts
	TempDir string

	// ConfigGenerator generates test configurations for benchmarks
	ConfigGenerator *ConfigGenerator

	// TemplateDataGenerator generates template data for benchmarks
	TemplateDataGenerator *TemplateDataGenerator
}

// BenchmarkResult captures the results of a benchmark run.
type BenchmarkResult struct {
	Name            string
	NsPerOp         int64
	AllocsPerOp     int64
	BytesPerOp      int64
	MemAllocsPerOp  int64
	MemBytesPerOp   int64
	Timestamp       time.Time
	Version         string
	Description     string
}

// NewBenchmarkFramework creates a new benchmark framework with default settings.
func NewBenchmarkFramework(b *testing.B) *BenchmarkFramework {
	b.Helper()

	tempDir := b.TempDir()

	return &BenchmarkFramework{
		BaselineResults:       make(map[string]BenchmarkResult),
		ThresholdMultiplier:   1.5, // Allow 50% performance degradation
		TempDir:               tempDir,
		ConfigGenerator:       NewConfigGenerator(42),
		TemplateDataGenerator: NewTemplateDataGenerator(42),
	}
}

// NewBenchmarkFrameworkWithThreshold creates a benchmark framework with a custom threshold.
func NewBenchmarkFrameworkWithThreshold(b *testing.B, threshold float64) *BenchmarkFramework {
	b.Helper()

	fw := NewBenchmarkFramework(b)
	fw.ThresholdMultiplier = threshold
	return fw
}

// RecordBaseline records a baseline benchmark result for future comparison.
func (fw *BenchmarkFramework) RecordBaseline(name string, result BenchmarkResult) {
	result.Name = name
	result.Timestamp = time.Now()
	fw.BaselineResults[name] = result
}

// CompareToBaseline compares a benchmark result to the recorded baseline.
// Returns true if the performance is within acceptable threshold, false otherwise.
func (fw *BenchmarkFramework) CompareToBaseline(b *testing.B, name string, nsPerOp int64) bool {
	b.Helper()

	baseline, exists := fw.BaselineResults[name]
	if !exists {
		b.Logf("No baseline found for %s, recording current result", name)
		fw.RecordBaseline(name, BenchmarkResult{
			Name:      name,
			NsPerOp:   nsPerOp,
			Timestamp: time.Now(),
		})
		return true
	}

	threshold := float64(baseline.NsPerOp) * fw.ThresholdMultiplier
	if float64(nsPerOp) > threshold {
		b.Errorf("Performance regression detected for %s: %d ns/op (baseline: %d ns/op, threshold: %.0f ns/op)",
			name, nsPerOp, baseline.NsPerOp, threshold)
		return false
	}

	improvement := float64(baseline.NsPerOp-nsPerOp) / float64(baseline.NsPerOp) * 100
	if improvement > 0 {
		b.Logf("Performance improvement for %s: %.2f%% faster", name, improvement)
	}

	return true
}

// BenchmarkTemplateRendering benchmarks template rendering performance.
func (fw *BenchmarkFramework) BenchmarkTemplateRendering(b *testing.B, engine template.TemplateEngine, templatePath string, data interface{}) {
	b.Helper()

	ctx := context.Background()

	// Warmup
	for i := 0; i < 10; i++ {
		_, _ = engine.Render(ctx, templatePath, data)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := engine.Render(ctx, templatePath, data)
		if err != nil {
			b.Fatalf("template rendering failed: %v", err)
		}
	}
}

// BenchmarkConfigBuilding benchmarks configuration building performance.
func (fw *BenchmarkFramework) BenchmarkConfigBuilding(b *testing.B, buildFunc func() (config.Config, error)) {
	b.Helper()

	// Warmup
	for i := 0; i < 10; i++ {
		_, _ = buildFunc()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := buildFunc()
		if err != nil {
			b.Fatalf("config building failed: %v", err)
		}
	}
}

// BenchmarkConfigValidation benchmarks configuration validation performance.
func (fw *BenchmarkFramework) BenchmarkConfigValidation(b *testing.B, cfg config.Config, validateFunc func(config.Config) error) {
	b.Helper()

	// Warmup
	for i := 0; i < 10; i++ {
		_ = validateFunc(cfg)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := validateFunc(cfg)
		if err != nil {
			b.Fatalf("config validation failed: %v", err)
		}
	}
}

// BenchmarkConcurrentOperations benchmarks concurrent operations performance.
func (fw *BenchmarkFramework) BenchmarkConcurrentOperations(b *testing.B, operation func() error, concurrency int) {
	b.Helper()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := operation(); err != nil {
				b.Errorf("concurrent operation failed: %v", err)
			}
		}
	})
}

// BenchmarkMemoryUsage benchmarks memory usage for an operation.
func (fw *BenchmarkFramework) BenchmarkMemoryUsage(b *testing.B, operation func() error) {
	b.Helper()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := operation(); err != nil {
			b.Fatalf("operation failed: %v", err)
		}
	}
}

// CreateBenchmarkTemplate creates a template file for benchmarking.
func (fw *BenchmarkFramework) CreateBenchmarkTemplate(b *testing.B, name string, size int) string {
	b.Helper()

	templateDir := filepath.Join(fw.TempDir, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		b.Fatalf("failed to create template directory: %v", err)
	}

	templatePath := filepath.Join(templateDir, name)

	// Generate template content of specified size
	content := fw.generateTemplateContent(size)

	if err := os.WriteFile(templatePath, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write template: %v", err)
	}

	return templatePath
}

// generateTemplateContent generates template content of approximately the specified size.
func (fw *BenchmarkFramework) generateTemplateContent(size int) string {
	// Generate a template with loops and conditionals to reach target size
	template := `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
data:
`

	// Add data entries to reach target size
	entrySize := 50 // Approximate size per entry
	numEntries := size / entrySize

	for i := 0; i < numEntries; i++ {
		template += fmt.Sprintf("  key-%d: {{ .Data.Value%d }}\n", i, i)
	}

	return template
}

// CreateBenchmarkConfig creates a configuration for benchmarking.
func (fw *BenchmarkFramework) CreateBenchmarkConfig(provider string) config.Config {
	return fw.ConfigGenerator.GenerateConfig(provider)
}

// CreateLargeBenchmarkConfig creates a large configuration for stress testing.
func (fw *BenchmarkFramework) CreateLargeBenchmarkConfig(provider string) config.Config {
	cfg := fw.ConfigGenerator.GenerateConfig(provider)

	// Simulate large configuration by adding overrides
	if cfg.Overrides == nil {
		cfg.Overrides = make(map[string]any)
	}

	for i := 0; i < 100; i++ {
		cfg.Overrides[fmt.Sprintf("custom.setting.%d", i)] = fmt.Sprintf("value-%d", i)
	}

	return cfg
}

// CreateBenchmarkTemplateData creates template data for benchmarking.
func (fw *BenchmarkFramework) CreateBenchmarkTemplateData() map[string]interface{} {
	return fw.TemplateDataGenerator.GenerateTemplateData()
}

// CreateLargeBenchmarkTemplateData creates large template data for stress testing.
func (fw *BenchmarkFramework) CreateLargeBenchmarkTemplateData() map[string]interface{} {
	data := fw.TemplateDataGenerator.GenerateTemplateData()

	// Add large nested structures
	data["LargeArray"] = make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		data["LargeArray"].([]map[string]interface{})[i] = map[string]interface{}{
			"Index":       i,
			"Name":        fmt.Sprintf("item-%d", i),
			"Description": fmt.Sprintf("Description for item %d", i),
			"Metadata": map[string]string{
				"key1": fmt.Sprintf("value1-%d", i),
				"key2": fmt.Sprintf("value2-%d", i),
				"key3": fmt.Sprintf("value3-%d", i),
			},
		}
	}

	return data
}

// MeasureOperation measures the time taken by an operation.
func (fw *BenchmarkFramework) MeasureOperation(b *testing.B, name string, operation func() error) time.Duration {
	b.Helper()

	start := time.Now()
	if err := operation(); err != nil {
		b.Fatalf("operation %s failed: %v", name, err)
	}
	duration := time.Since(start)

	b.Logf("Operation %s took %v", name, duration)
	return duration
}

// CompareOperations compares the performance of two operations.
func (fw *BenchmarkFramework) CompareOperations(b *testing.B, name1 string, op1 func() error, name2 string, op2 func() error) {
	b.Helper()

	// Measure first operation
	duration1 := fw.MeasureOperation(b, name1, op1)

	// Measure second operation
	duration2 := fw.MeasureOperation(b, name2, op2)

	// Compare results
	if duration1 < duration2 {
		improvement := float64(duration2-duration1) / float64(duration2) * 100
		b.Logf("%s is %.2f%% faster than %s", name1, improvement, name2)
	} else {
		degradation := float64(duration1-duration2) / float64(duration1) * 100
		b.Logf("%s is %.2f%% slower than %s", name1, degradation, name2)
	}
}

// RunBenchmarkSuite runs a suite of related benchmarks.
func (fw *BenchmarkFramework) RunBenchmarkSuite(b *testing.B, suite map[string]func(*testing.B)) {
	b.Helper()

	for name, benchFunc := range suite {
		b.Run(name, benchFunc)
	}
}

// SaveBenchmarkResults saves benchmark results to a file for historical comparison.
func (fw *BenchmarkFramework) SaveBenchmarkResults(b *testing.B, filename string) error {
	b.Helper()

	resultsPath := filepath.Join(fw.TempDir, filename)

	// Create results directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(resultsPath), 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	// Format results as text
	var content string
	for name, result := range fw.BaselineResults {
		content += fmt.Sprintf("%s: %d ns/op, %d allocs/op, %d bytes/op\n",
			name, result.NsPerOp, result.AllocsPerOp, result.BytesPerOp)
	}

	if err := os.WriteFile(resultsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}

	b.Logf("Benchmark results saved to %s", resultsPath)
	return nil
}

// LoadBenchmarkResults loads benchmark results from a file for comparison.
func (fw *BenchmarkFramework) LoadBenchmarkResults(b *testing.B, filename string) error {
	b.Helper()

	resultsPath := filepath.Join(fw.TempDir, filename)

	data, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("failed to read results: %w", err)
	}

	// Parse results (simplified parsing)
	// In a real implementation, you'd use a proper format like JSON
	b.Logf("Loaded benchmark results from %s: %d bytes", resultsPath, len(data))

	return nil
}
