/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCachePerformanceImprovement validates that template caching provides
// measurable performance improvements over non-cached rendering.
// This test ensures that the cache implementation meets the acceptance criteria
// for Task 1.2: "Template caching improves performance measurably".
func TestCachePerformanceImprovement(t *testing.T) {
	// Create a temporary directory for test templates
	tmpDir := t.TempDir()

	// Create a test template
	templatePath := filepath.Join(tmpDir, "test.tmpl")
	templateContent := `
{{- range .Items }}
Item: {{.Name}} - {{.Value}}
{{- end }}
Total: {{.Total}}
`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Test data
	data := map[string]interface{}{
		"Items": []map[string]interface{}{
			{"Name": "Item1", "Value": 100},
			{"Name": "Item2", "Value": 200},
			{"Name": "Item3", "Value": 300},
		},
		"Total": 600,
	}

	ctx := context.Background()
	iterations := 100

	// Measure performance with cache enabled
	engineWithCache := NewGoTemplateEngine()
	engineWithCache.SetCacheEnabled(true)

	startWithCache := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := engineWithCache.Render(ctx, templatePath, data)
		if err != nil {
			t.Fatalf("render with cache failed: %v", err)
		}
	}
	durationWithCache := time.Since(startWithCache)

	// Measure performance with cache disabled
	engineWithoutCache := NewGoTemplateEngine()
	engineWithoutCache.SetCacheEnabled(false)

	startWithoutCache := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := engineWithoutCache.Render(ctx, templatePath, data)
		if err != nil {
			t.Fatalf("render without cache failed: %v", err)
		}
	}
	durationWithoutCache := time.Since(startWithoutCache)

	// Calculate performance improvement
	improvement := float64(durationWithoutCache-durationWithCache) / float64(durationWithoutCache) * 100

	// Log performance metrics
	t.Logf("Performance Metrics (n=%d):", iterations)
	t.Logf("  With Cache:    %v (avg: %v per render)", durationWithCache, durationWithCache/time.Duration(iterations))
	t.Logf("  Without Cache: %v (avg: %v per render)", durationWithoutCache, durationWithoutCache/time.Duration(iterations))
	t.Logf("  Improvement:   %.2f%%", improvement)
	t.Logf("  Speedup:       %.2fx", float64(durationWithoutCache)/float64(durationWithCache))

	// Validate that caching provides measurable improvement
	// We expect at least 50% improvement (2x speedup) from caching
	minImprovementPercent := 50.0
	if improvement < minImprovementPercent {
		t.Errorf("cache performance improvement (%.2f%%) is below minimum threshold (%.2f%%)",
			improvement, minImprovementPercent)
	}

	// Validate that cached rendering is significantly faster
	// We expect cached rendering to be at least 2x faster
	minSpeedup := 2.0
	speedup := float64(durationWithoutCache) / float64(durationWithCache)
	if speedup < minSpeedup {
		t.Errorf("cache speedup (%.2fx) is below minimum threshold (%.2fx)",
			speedup, minSpeedup)
	}
}

// TestCacheConsistency validates that cached templates produce identical output
// to non-cached templates, ensuring correctness is maintained.
func TestCacheConsistency(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test template
	templatePath := filepath.Join(tmpDir, "consistency.tmpl")
	templateContent := `Hello, {{.Name}}! Your ID is {{.ID}}.`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	ctx := context.Background()
	data := map[string]interface{}{
		"Name": "TestUser",
		"ID":   12345,
	}

	// Render with cache enabled
	engineWithCache := NewGoTemplateEngine()
	engineWithCache.SetCacheEnabled(true)
	outputWithCache, err := engineWithCache.Render(ctx, templatePath, data)
	if err != nil {
		t.Fatalf("render with cache failed: %v", err)
	}

	// Render with cache disabled
	engineWithoutCache := NewGoTemplateEngine()
	engineWithoutCache.SetCacheEnabled(false)
	outputWithoutCache, err := engineWithoutCache.Render(ctx, templatePath, data)
	if err != nil {
		t.Fatalf("render without cache failed: %v", err)
	}

	// Validate outputs are identical
	if string(outputWithCache) != string(outputWithoutCache) {
		t.Errorf("cached and non-cached outputs differ:\n  Cached:    %s\n  Non-cached: %s",
			outputWithCache, outputWithoutCache)
	}
}

// TestCacheEffectivenessWithMultipleTemplates validates that caching provides
// performance benefits when rendering multiple different templates.
func TestCacheEffectivenessWithMultipleTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple templates
	numTemplates := 5
	templates := make([]string, numTemplates)
	for i := 0; i < numTemplates; i++ {
		templatePath := filepath.Join(tmpDir, filepath.Base(tmpDir)+"-template-"+string(rune('A'+i))+".tmpl")
		content := "Template " + string(rune('A'+i)) + ": {{.Value}}"
		if err := os.WriteFile(templatePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}
		templates[i] = templatePath
	}

	ctx := context.Background()
	data := map[string]int{"Value": 42}
	iterations := 50 // Render each template multiple times

	// Measure performance with cache enabled
	engineWithCache := NewGoTemplateEngine()
	engineWithCache.SetCacheEnabled(true)

	startWithCache := time.Now()
	for i := 0; i < iterations; i++ {
		for _, templatePath := range templates {
			_, err := engineWithCache.Render(ctx, templatePath, data)
			if err != nil {
				t.Fatalf("render with cache failed: %v", err)
			}
		}
	}
	durationWithCache := time.Since(startWithCache)

	// Measure performance with cache disabled
	engineWithoutCache := NewGoTemplateEngine()
	engineWithoutCache.SetCacheEnabled(false)

	startWithoutCache := time.Now()
	for i := 0; i < iterations; i++ {
		for _, templatePath := range templates {
			_, err := engineWithoutCache.Render(ctx, templatePath, data)
			if err != nil {
				t.Fatalf("render without cache failed: %v", err)
			}
		}
	}
	durationWithoutCache := time.Since(startWithoutCache)

	// Calculate performance improvement
	improvement := float64(durationWithoutCache-durationWithCache) / float64(durationWithoutCache) * 100
	totalRenders := iterations * numTemplates

	// Log performance metrics
	t.Logf("Multi-Template Performance Metrics (templates=%d, iterations=%d, total renders=%d):",
		numTemplates, iterations, totalRenders)
	t.Logf("  With Cache:    %v (avg: %v per render)", durationWithCache, durationWithCache/time.Duration(totalRenders))
	t.Logf("  Without Cache: %v (avg: %v per render)", durationWithoutCache, durationWithoutCache/time.Duration(totalRenders))
	t.Logf("  Improvement:   %.2f%%", improvement)
	t.Logf("  Speedup:       %.2fx", float64(durationWithoutCache)/float64(durationWithCache))

	// Validate that caching provides measurable improvement with multiple templates.
	// The exact gain varies substantially across CI hosts, so keep the threshold
	// low enough to avoid machine-specific flakes while still catching regressions.
	minImprovementPercent := 10.0
	if improvement < minImprovementPercent {
		t.Errorf("cache performance improvement (%.2f%%) is below minimum threshold (%.2f%%)",
			improvement, minImprovementPercent)
	}
}

// TestCacheMemoryEfficiency validates that template caching doesn't consume
// excessive memory while still providing performance benefits.
func TestCacheMemoryEfficiency(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a moderately complex template
	templatePath := filepath.Join(tmpDir, "memory.tmpl")
	templateContent := `
{{- range .Items }}
Item: {{.Name}} - {{.Description}}
Price: ${{.Price}}
{{- end }}
`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	ctx := context.Background()
	data := map[string]interface{}{
		"Items": []map[string]interface{}{
			{"Name": "A", "Description": "Item A", "Price": 10.99},
			{"Name": "B", "Description": "Item B", "Price": 20.99},
			{"Name": "C", "Description": "Item C", "Price": 30.99},
		},
	}

	engine := NewGoTemplateEngine()
	engine.SetCacheEnabled(true)

	// Render template multiple times to populate cache
	for i := 0; i < 10; i++ {
		_, err := engine.Render(ctx, templatePath, data)
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}
	}

	// Verify cache contains the template
	engine.mu.RLock()
	cacheSize := len(engine.cache)
	engine.mu.RUnlock()

	if cacheSize != 1 {
		t.Errorf("expected cache size of 1, got %d", cacheSize)
	}

	// Verify cache can be cleared to free memory
	engine.ClearCache()

	engine.mu.RLock()
	cacheSizeAfterClear := len(engine.cache)
	engine.mu.RUnlock()

	if cacheSizeAfterClear != 0 {
		t.Errorf("expected cache size of 0 after clear, got %d", cacheSizeAfterClear)
	}
}
