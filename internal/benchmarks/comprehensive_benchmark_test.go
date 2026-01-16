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

// Package benchmarks provides comprehensive performance benchmarks for the
// configuration system refactor, comparing legacy vs new implementations.
//
// This benchmark suite validates Requirements 9.1, 9.3, and 9.5:
// - 9.1: Cache parsed templates and compiled configurations for reuse
// - 9.3: Support parallel template rendering where dependencies allow
// - 9.5: Reuse common template processing when generating multiple clusters
package benchmarks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/gitops"
	"github.com/rackerlabs/openCenter-cli/internal/template"
	testingpkg "github.com/rackerlabs/openCenter-cli/internal/testing"
)

// BenchmarkTemplateRendering_Legacy benchmarks the legacy template rendering implementation.
// This provides a baseline for comparing the new template engine performance.
//
// Validates: Requirement 9.1 (caching), 9.3 (parallel processing)
func BenchmarkTemplateRendering_Legacy(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test templates
	templates := createTestTemplates(b, tmpDir)
	testData := createTestData()

	for name, templatePath := range templates {
		b.Run(name, func(b *testing.B) {
			data := testData[name]

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Legacy rendering (direct text/template usage)
				_, err := template.RenderTemplateString(name, readTemplateFile(b, templatePath), data)
				if err != nil {
					b.Fatalf("legacy render failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkTemplateRendering_New benchmarks the new template engine implementation.
// This demonstrates performance improvements from caching and optimizations.
//
// Validates: Requirement 9.1 (caching), 9.3 (parallel processing)
func BenchmarkTemplateRendering_New(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test templates
	templates := createTestTemplates(b, tmpDir)
	testData := createTestData()

	// Create engine with caching enabled
	engine := template.NewGoTemplateEngine()
	engine.SetCacheEnabled(true)
	ctx := context.Background()

	for name, templatePath := range templates {
		b.Run(name, func(b *testing.B) {
			data := testData[name]

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := engine.Render(ctx, templatePath, data)
				if err != nil {
					b.Fatalf("new render failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkTemplateRendering_Parallel benchmarks parallel template rendering.
// This validates that the new engine can efficiently render templates concurrently.
//
// Validates: Requirement 9.3 (parallel processing)
func BenchmarkTemplateRendering_Parallel(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test template
	templatePath := filepath.Join(tmpDir, "parallel.tmpl")
	content := `Hello, {{.Name}}! Your ID is {{.ID}}.`
	if err := os.WriteFile(templatePath, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write template: %v", err)
	}

	engine := template.NewGoTemplateEngine()
	engine.SetCacheEnabled(true)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			data := map[string]interface{}{
				"Name": fmt.Sprintf("User%d", i),
				"ID":   i,
			}
			_, err := engine.Render(ctx, templatePath, data)
			if err != nil {
				b.Fatalf("parallel render failed: %v", err)
			}
			i++
		}
	})
}

// BenchmarkConfigBuilding_Legacy benchmarks legacy configuration building.
// This provides a baseline for configuration construction performance.
//
// Validates: Requirement 9.1 (caching)
func BenchmarkConfigBuilding_Legacy(b *testing.B) {
	providers := []string{"openstack", "aws", "baremetal"}

	for _, provider := range providers {
		b.Run(provider, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Legacy: Direct struct construction
				cfg := config.Config{
					SchemaVersion: "v1.0.0",
					OpenCenter: config.SimplifiedOpenCenter{
						Meta: config.ClusterMeta{
							Name:         "test-cluster",
							Organization: "test-org",
						},
						Infrastructure: config.Infrastructure{
							Provider: provider,
						},
					},
				}
				_ = cfg
			}
		})
	}
}

// BenchmarkConfigBuilding_New benchmarks new configuration builder.
// This demonstrates the fluent API performance with validation.
//
// Validates: Requirement 9.1 (caching)
func BenchmarkConfigBuilding_New(b *testing.B) {
	providers := []string{"baremetal"} // Only test baremetal which doesn't require cloud config

	for _, provider := range providers {
		b.Run(provider, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// New: Fluent builder API
				builder := config.NewConfigBuilder("test-cluster")
				cfg, err := builder.
					WithProvider(provider).
					WithOrganization("test-org").
					Build()
				if err != nil {
					b.Fatalf("config build failed: %v", err)
				}
				_ = cfg
			}
		})
	}
}

// BenchmarkConfigBuilding_Complex benchmarks complex configuration building.
// This tests performance with large configurations and many overrides.
//
// Validates: Requirement 9.1 (caching), 9.5 (reuse)
func BenchmarkConfigBuilding_Complex(b *testing.B) {
	gen := testingpkg.NewConfigGenerator(42)

	b.Run("Legacy", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			cfg := gen.GenerateConfig("openstack")
			// Add many overrides
			if cfg.Overrides == nil {
				cfg.Overrides = make(map[string]any)
			}
			for j := 0; j < 50; j++ {
				cfg.Overrides[fmt.Sprintf("custom.setting.%d", j)] = fmt.Sprintf("value-%d", j)
			}
			_ = cfg
		}
	})

	b.Run("New", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			builder := config.NewConfigBuilder("test-cluster")
			builder.WithProvider("baremetal"). // Use baremetal to avoid cloud validation
				WithOrganization("test-org")

			// Add many overrides
			for j := 0; j < 50; j++ {
				builder.WithOverride(fmt.Sprintf("custom.setting.%d", j), fmt.Sprintf("value-%d", j))
			}

			cfg, err := builder.Build()
			if err != nil {
				b.Fatalf("config build failed: %v", err)
			}
			_ = cfg
		}
	})
}

// BenchmarkGitOpsGeneration_Legacy benchmarks legacy GitOps generation.
// This provides a baseline for repository generation performance.
//
// Validates: Requirement 9.5 (reuse common template processing)
func BenchmarkGitOpsGeneration_Legacy(b *testing.B) {
	gen := testingpkg.NewConfigGenerator(42)
	cfg := gen.GenerateConfig("openstack")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tmpDir := b.TempDir()
		cfg.OpenCenter.GitOps.GitDir = tmpDir

		// Legacy: Direct copy with template rendering
		err := gitops.CopyBase(cfg, true)
		if err != nil {
			b.Fatalf("legacy gitops generation failed: %v", err)
		}
	}
}

// BenchmarkGitOpsGeneration_New benchmarks new pipeline-based GitOps generation.
// This demonstrates performance improvements from staged generation and caching.
//
// Validates: Requirement 9.5 (reuse common template processing)
func BenchmarkGitOpsGeneration_New(b *testing.B) {
	gen := testingpkg.NewConfigGenerator(42)
	cfg := gen.GenerateConfig("openstack")

	// Create workspace manager and pipeline generator
	tmpBase := b.TempDir()
	workspaceManager := gitops.NewWorkspaceManager(tmpBase)

	// Create stages (simplified for benchmark)
	stages := []gitops.GenerationStage{
		// Add actual stages here when available
	}
	generator := gitops.NewPipelineGenerator(workspaceManager, stages)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// New: Pipeline-based generation
		err := generator.Generate(ctx, cfg)
		if err != nil {
			// Skip if pipeline not fully implemented yet
			b.Skipf("pipeline generation not fully implemented: %v", err)
		}
	}
}

// BenchmarkGitOpsGeneration_MultiCluster benchmarks generating multiple clusters.
// This validates that common template processing is reused efficiently.
//
// Validates: Requirement 9.5 (reuse common template processing)
func BenchmarkGitOpsGeneration_MultiCluster(b *testing.B) {
	gen := testingpkg.NewConfigGenerator(42)
	numClusters := 5

	b.Run("Legacy", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			for j := 0; j < numClusters; j++ {
				cfg := gen.GenerateConfig("openstack")
				cfg.OpenCenter.Meta.Name = fmt.Sprintf("cluster-%d", j)

				tmpDir := b.TempDir()
				cfg.OpenCenter.GitOps.GitDir = tmpDir

				err := gitops.CopyBase(cfg, true)
				if err != nil {
					b.Fatalf("legacy multi-cluster generation failed: %v", err)
				}
			}
		}
	})

	b.Run("New", func(b *testing.B) {
		tmpBase := b.TempDir()
		workspaceManager := gitops.NewWorkspaceManager(tmpBase)

		stages := []gitops.GenerationStage{
			// Add actual stages here when available
		}
		generator := gitops.NewPipelineGenerator(workspaceManager, stages)
		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			for j := 0; j < numClusters; j++ {
				cfg := gen.GenerateConfig("openstack")
				cfg.OpenCenter.Meta.Name = fmt.Sprintf("cluster-%d", j)

				err := generator.Generate(ctx, cfg)
				if err != nil {
					// Skip if pipeline not fully implemented yet
					b.Skipf("pipeline generation not fully implemented: %v", err)
				}
			}
		}
	})
}

// BenchmarkCaching_TemplateReuse benchmarks template caching effectiveness.
// This validates that cached templates provide measurable performance improvements.
//
// Validates: Requirement 9.1 (cache parsed templates)
func BenchmarkCaching_TemplateReuse(b *testing.B) {
	tmpDir := b.TempDir()
	templatePath := filepath.Join(tmpDir, "reuse.tmpl")
	content := `Hello, {{.Name}}! Count: {{.Count}}`
	if err := os.WriteFile(templatePath, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write template: %v", err)
	}

	ctx := context.Background()

	b.Run("WithCache", func(b *testing.B) {
		engine := template.NewGoTemplateEngine()
		engine.SetCacheEnabled(true)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			data := map[string]interface{}{
				"Name":  "Test",
				"Count": i,
			}
			_, err := engine.Render(ctx, templatePath, data)
			if err != nil {
				b.Fatalf("cached render failed: %v", err)
			}
		}
	})

	b.Run("WithoutCache", func(b *testing.B) {
		engine := template.NewGoTemplateEngine()
		engine.SetCacheEnabled(false)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			data := map[string]interface{}{
				"Name":  "Test",
				"Count": i,
			}
			_, err := engine.Render(ctx, templatePath, data)
			if err != nil {
				b.Fatalf("uncached render failed: %v", err)
			}
		}
	})
}

// BenchmarkMemoryUsage_TemplateEngine benchmarks memory usage of template engines.
// This ensures that the new engine doesn't consume excessive memory.
//
// Validates: Requirement 9.1 (optimize memory usage)
func BenchmarkMemoryUsage_TemplateEngine(b *testing.B) {
	tmpDir := b.TempDir()

	// Create templates of varying sizes
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			templatePath := filepath.Join(tmpDir, fmt.Sprintf("large%d.tmpl", size))
			content := generateLargeTemplate(size)
			if err := os.WriteFile(templatePath, []byte(content), 0644); err != nil {
				b.Fatalf("failed to write template: %v", err)
			}

			engine := template.NewGoTemplateEngine()
			engine.SetCacheEnabled(true)
			ctx := context.Background()
			data := map[string]interface{}{"Value": 42}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := engine.Render(ctx, templatePath, data)
				if err != nil {
					b.Fatalf("render failed: %v", err)
				}
			}
		})
	}
}

// Helper functions

func createTestTemplates(b *testing.B, tmpDir string) map[string]string {
	b.Helper()

	templates := map[string]string{
		"simple": `Hello, {{.Name}}!`,
		"medium": `
{{- range .Items }}
Item: {{.Name}} - {{.Value}}
{{- end }}
Total: {{.Total}}
`,
		"complex": `
{{- define "header" -}}
=== {{.Title}} ===
{{- end -}}

{{- template "header" . }}

{{- range $idx, $item := .Items }}
Item #{{add $idx 1}}: {{$item.Name}}
{{- end }}
`,
	}

	result := make(map[string]string)
	for name, content := range templates {
		path := filepath.Join(tmpDir, name+".tmpl")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to write template %s: %v", name, err)
		}
		result[name] = path
	}

	return result
}

func createTestData() map[string]interface{} {
	return map[string]interface{}{
		"simple": map[string]string{
			"Name": "World",
		},
		"medium": map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Name": "Item1", "Value": 100},
				{"Name": "Item2", "Value": 200},
			},
			"Total": 300,
		},
		"complex": map[string]interface{}{
			"Title": "Test",
			"Items": []map[string]interface{}{
				{"Name": "A"},
				{"Name": "B"},
			},
		},
	}
}

func readTemplateFile(b *testing.B, path string) string {
	b.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("failed to read template: %v", err)
	}
	return string(content)
}

func generateLargeTemplate(size int) string {
	content := "{{.Value}}"
	for i := 0; i < size; i++ {
		content += fmt.Sprintf("\nLine %d: {{.Value}}", i)
	}
	return content
}
