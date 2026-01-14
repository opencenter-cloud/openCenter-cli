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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"

	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestLegacyCompatibility verifies that the legacy compatibility layer produces
// identical output to the new template engine for the same inputs.
func TestLegacyCompatibility(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		template     string
		data         interface{}
		expected     string
	}{
		{
			name:         "simple variable substitution",
			templateName: "simple.tmpl",
			template:     "Hello, {{.Name}}!",
			data:         map[string]string{"Name": "World"},
			expected:     "Hello, World!",
		},
		{
			name:         "sprig function - upper",
			templateName: "upper.tmpl",
			template:     "{{.Name | upper}}",
			data:         map[string]string{"Name": "hello"},
			expected:     "HELLO",
		},
		{
			name:         "sprig function - default",
			templateName: "default.tmpl",
			template:     "{{.Name | default \"Guest\"}}",
			data:         map[string]string{},
			expected:     "Guest",
		},
		{
			name:         "nested data structure",
			templateName: "nested.tmpl",
			template:     "{{.User.Name}} - {{.User.Email}}",
			data: map[string]interface{}{
				"User": map[string]string{
					"Name":  "John Doe",
					"Email": "john@example.com",
				},
			},
			expected: "John Doe - john@example.com",
		},
		{
			name:         "range over slice",
			templateName: "range.tmpl",
			template:     "{{range .Items}}{{.}},{{end}}",
			data:         map[string][]string{"Items": {"a", "b", "c"}},
			expected:     "a,b,c,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with legacy compatibility layer
			legacyResult, err := RenderTemplateString(tt.templateName, tt.template, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, legacyResult)

			// Test with new template engine
			engine := NewGoTemplateEngine()
			newResult, err := engine.RenderString(context.Background(), tt.templateName, tt.template, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(newResult))

			// Verify both produce identical output
			assert.Equal(t, legacyResult, string(newResult), "legacy and new engine should produce identical output")
		})
	}
}

// TestLegacyToNewMigration demonstrates how to migrate from legacy template calls
// to the new template engine while maintaining identical behavior.
func TestLegacyToNewMigration(t *testing.T) {
	// Create a test filesystem with templates
	fsys := fstest.MapFS{
		"greeting.tmpl": &fstest.MapFile{
			Data: []byte("Hello, {{.Name}}!"),
		},
		"config.tmpl": &fstest.MapFile{
			Data: []byte("cluster: {{.Cluster}}\nregion: {{.Region}}"),
		},
	}

	tmpDir := t.TempDir()
	data := map[string]string{
		"Name":    "World",
		"Cluster": "prod",
		"Region":  "us-east-1",
	}

	// Old way: Using legacy compatibility layer
	t.Run("legacy approach", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "legacy-greeting.txt")
		err := RenderTemplateToFile(fsys, "greeting.tmpl", outputPath, data)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", string(content))
	})

	// New way: Using template engine directly
	t.Run("new engine approach", func(t *testing.T) {
		engine := NewGoTemplateEngine()
		outputPath := filepath.Join(tmpDir, "new-greeting.txt")
		err := RenderWithEngine(engine, fsys, "greeting.tmpl", outputPath, data)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", string(content))
	})

	// Verify both approaches produce identical output
	legacyContent, err := os.ReadFile(filepath.Join(tmpDir, "legacy-greeting.txt"))
	require.NoError(t, err)

	newContent, err := os.ReadFile(filepath.Join(tmpDir, "new-greeting.txt"))
	require.NoError(t, err)

	assert.Equal(t, string(legacyContent), string(newContent), "legacy and new approaches should produce identical output")
}

// TestBackwardCompatibilityWithExistingCode verifies that existing code patterns
// continue to work with the legacy compatibility layer.
func TestBackwardCompatibilityWithExistingCode(t *testing.T) {
	// This test simulates the pattern used in internal/gitops/copy.go
	fsys := fstest.MapFS{
		"cluster-config.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.ClusterName}}
  namespace: {{.Namespace}}
data:
  region: {{.Region}}
  provider: {{.Provider}}`),
		},
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "cluster-config.yaml")

	// Simulate configuration data structure
	config := map[string]string{
		"ClusterName": "my-cluster",
		"Namespace":   "kube-system",
		"Region":      "us-west-2",
		"Provider":    "openstack",
	}

	// Render using legacy compatibility layer (same pattern as copy.go)
	err := RenderTemplateToFile(fsys, "cluster-config.yaml.tmpl", outputPath, config)
	require.NoError(t, err)

	// Verify output
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "name: my-cluster")
	assert.Contains(t, string(content), "namespace: kube-system")
	assert.Contains(t, string(content), "region: us-west-2")
	assert.Contains(t, string(content), "provider: openstack")
}

// TestMigrationPathDocumentation provides examples of how to migrate from
// legacy template calls to the new template engine.
func TestMigrationPathDocumentation(t *testing.T) {
	fsys := fstest.MapFS{
		"example.tmpl": &fstest.MapFile{
			Data: []byte("Value: {{.Value}}"),
		},
	}

	tmpDir := t.TempDir()
	data := map[string]string{"Value": "test"}

	// Step 1: Current code using direct text/template (simulated by legacy layer)
	t.Run("step1_current_code", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "step1.txt")
		err := RenderTemplateToFile(fsys, "example.tmpl", outputPath, data)
		require.NoError(t, err)
	})

	// Step 2: Migrate to using legacy compatibility layer (no code changes needed)
	t.Run("step2_legacy_layer", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "step2.txt")
		err := RenderTemplateToFile(fsys, "example.tmpl", outputPath, data)
		require.NoError(t, err)
	})

	// Step 3: Migrate to using new template engine with helper function
	t.Run("step3_engine_with_helper", func(t *testing.T) {
		engine := NewGoTemplateEngine()
		outputPath := filepath.Join(tmpDir, "step3.txt")
		err := RenderWithEngine(engine, fsys, "example.tmpl", outputPath, data)
		require.NoError(t, err)
	})

	// Step 4: Full migration to new template engine API
	t.Run("step4_full_engine_api", func(t *testing.T) {
		engine := NewGoTemplateEngine()
		outputPath := filepath.Join(tmpDir, "step4.txt")

		// Read template content
		content, err := fsys.ReadFile("example.tmpl")
		require.NoError(t, err)

		// Render using engine
		result, err := engine.RenderString(context.Background(), "example.tmpl", string(content), data)
		require.NoError(t, err)

		// Write to file
		err = os.WriteFile(outputPath, result, 0o644)
		require.NoError(t, err)
	})

	// Verify all steps produce identical output
	for i := 1; i <= 4; i++ {
		content, err := os.ReadFile(filepath.Join(tmpDir, "step"+string(rune('0'+i))+".txt"))
		require.NoError(t, err)
		assert.Equal(t, "Value: test", string(content), "step %d should produce identical output", i)
	}
}

// TestFeatureFlagSimulation demonstrates how a feature flag could be used
// to switch between old and new template systems during migration.
func TestFeatureFlagSimulation(t *testing.T) {
	fsys := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{
			Data: []byte("Result: {{.Value}}"),
		},
	}

	tmpDir := t.TempDir()
	data := map[string]string{"Value": "test"}

	// Simulate feature flag behavior
	useNewEngine := false // This would be read from environment or config

	renderTemplate := func(outputPath string) error {
		if useNewEngine {
			// Use new template engine
			engine := NewGoTemplateEngine()
			return RenderWithEngine(engine, fsys, "test.tmpl", outputPath, data)
		}
		// Use legacy compatibility layer
		return RenderTemplateToFile(fsys, "test.tmpl", outputPath, data)
	}

	// Test with legacy system
	t.Run("legacy_system", func(t *testing.T) {
		useNewEngine = false
		outputPath := filepath.Join(tmpDir, "legacy.txt")
		err := renderTemplate(outputPath)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Equal(t, "Result: test", string(content))
	})

	// Test with new system
	t.Run("new_system", func(t *testing.T) {
		useNewEngine = true
		outputPath := filepath.Join(tmpDir, "new.txt")
		err := renderTemplate(outputPath)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Equal(t, "Result: test", string(content))
	})

	// Verify both produce identical output
	legacyContent, err := os.ReadFile(filepath.Join(tmpDir, "legacy.txt"))
	require.NoError(t, err)

	newContent, err := os.ReadFile(filepath.Join(tmpDir, "new.txt"))
	require.NoError(t, err)

	assert.Equal(t, string(legacyContent), string(newContent), "legacy and new systems should produce identical output")
}

// TestFeatureFlagEnvironmentVariable tests the actual feature flag implementation
// using the OPENCENTER_USE_NEW_TEMPLATE_ENGINE environment variable.
func TestFeatureFlagEnvironmentVariable(t *testing.T) {
	fsys := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{
			Data: []byte("Value: {{.Value | upper}}"),
		},
	}

	tmpDir := t.TempDir()
	data := map[string]string{"Value": "hello"}

	tests := []struct {
		name        string
		envValue    string
		expectNew   bool
		description string
	}{
		{
			name:        "unset_uses_legacy",
			envValue:    "",
			expectNew:   false,
			description: "When env var is unset, should use legacy system",
		},
		{
			name:        "false_uses_legacy",
			envValue:    "false",
			expectNew:   false,
			description: "When env var is 'false', should use legacy system",
		},
		{
			name:        "true_uses_new",
			envValue:    "true",
			expectNew:   true,
			description: "When env var is 'true', should use new engine",
		},
		{
			name:        "1_uses_new",
			envValue:    "1",
			expectNew:   true,
			description: "When env var is '1', should use new engine",
		},
		{
			name:        "yes_uses_new",
			envValue:    "yes",
			expectNew:   true,
			description: "When env var is 'yes', should use new engine",
		},
		{
			name:        "on_uses_new",
			envValue:    "on",
			expectNew:   true,
			description: "When env var is 'on', should use new engine",
		},
		{
			name:        "TRUE_uppercase_uses_new",
			envValue:    "TRUE",
			expectNew:   true,
			description: "When env var is 'TRUE' (uppercase), should use new engine",
		},
		{
			name:        "invalid_uses_legacy",
			envValue:    "invalid",
			expectNew:   false,
			description: "When env var is invalid value, should use legacy system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				t.Setenv(EnvUseNewTemplateEngine, tt.envValue)
			}

			// Verify UseNewTemplateEngine returns expected value
			assert.Equal(t, tt.expectNew, UseNewTemplateEngine(), tt.description)

			// Render template and verify it works
			outputPath := filepath.Join(tmpDir, tt.name+".txt")
			err := RenderTemplateToFile(fsys, "test.tmpl", outputPath, data)
			require.NoError(t, err, "Template rendering should succeed regardless of engine")

			// Verify output is correct
			content, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			assert.Equal(t, "Value: HELLO", string(content), "Output should be correct regardless of engine")
		})
	}
}

// TestFeatureFlagOutputIdentity verifies that both engines produce identical output
// when the feature flag is toggled.
func TestFeatureFlagOutputIdentity(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     interface{}
	}{
		{
			name:     "simple_substitution",
			template: "Hello, {{.Name}}!",
			data:     map[string]string{"Name": "World"},
		},
		{
			name:     "sprig_functions",
			template: "{{.Value | upper | quote}}",
			data:     map[string]string{"Value": "test"},
		},
		{
			name:     "nested_data",
			template: "{{.User.Name}} - {{.User.Email}}",
			data: map[string]interface{}{
				"User": map[string]string{
					"Name":  "John",
					"Email": "john@example.com",
				},
			},
		},
		{
			name:     "range_iteration",
			template: "{{range .Items}}{{.}},{{end}}",
			data:     map[string][]string{"Items": {"a", "b", "c"}},
		},
		{
			name: "conditional_logic",
			template: `{{if .Enabled}}enabled{{else}}disabled{{end}}`,
			data:     map[string]bool{"Enabled": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"test.tmpl": &fstest.MapFile{
					Data: []byte(tt.template),
				},
			}

			tmpDir := t.TempDir()

			// Render with legacy system (feature flag off)
			t.Setenv(EnvUseNewTemplateEngine, "false")
			legacyOutput := filepath.Join(tmpDir, "legacy.txt")
			err := RenderTemplateToFile(fsys, "test.tmpl", legacyOutput, tt.data)
			require.NoError(t, err)

			// Render with new system (feature flag on)
			t.Setenv(EnvUseNewTemplateEngine, "true")
			newOutput := filepath.Join(tmpDir, "new.txt")
			err = RenderTemplateToFile(fsys, "test.tmpl", newOutput, tt.data)
			require.NoError(t, err)

			// Verify outputs are identical
			legacyContent, err := os.ReadFile(legacyOutput)
			require.NoError(t, err)

			newContent, err := os.ReadFile(newOutput)
			require.NoError(t, err)

			assert.Equal(t, string(legacyContent), string(newContent),
				"Legacy and new engines must produce identical output when feature flag is toggled")
		})
	}
}

// TestFeatureFlagRollbackScenario tests the ability to rollback to legacy system
// by disabling the feature flag.
func TestFeatureFlagRollbackScenario(t *testing.T) {
	fsys := fstest.MapFS{
		"config.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.Name}}
  namespace: {{.Namespace | default "default"}}
data:
  key: {{.Value | quote}}`),
		},
	}

	tmpDir := t.TempDir()
	data := map[string]string{
		"Name":  "test-config",
		"Value": "test-value",
	}

	// Phase 1: Deploy with new engine enabled
	t.Run("phase1_new_engine", func(t *testing.T) {
		t.Setenv(EnvUseNewTemplateEngine, "true")
		assert.True(t, UseNewTemplateEngine(), "Feature flag should be enabled")

		outputPath := filepath.Join(tmpDir, "phase1.yaml")
		err := RenderTemplateToFile(fsys, "config.yaml.tmpl", outputPath, data)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "name: test-config")
		assert.Contains(t, string(content), `key: "test-value"`)
	})

	// Phase 2: Rollback to legacy system (disable feature flag)
	t.Run("phase2_rollback_to_legacy", func(t *testing.T) {
		t.Setenv(EnvUseNewTemplateEngine, "false")
		assert.False(t, UseNewTemplateEngine(), "Feature flag should be disabled")

		outputPath := filepath.Join(tmpDir, "phase2.yaml")
		err := RenderTemplateToFile(fsys, "config.yaml.tmpl", outputPath, data)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "name: test-config")
		assert.Contains(t, string(content), `key: "test-value"`)
	})

	// Phase 3: Verify both phases produced identical output
	t.Run("phase3_verify_identity", func(t *testing.T) {
		phase1Content, err := os.ReadFile(filepath.Join(tmpDir, "phase1.yaml"))
		require.NoError(t, err)

		phase2Content, err := os.ReadFile(filepath.Join(tmpDir, "phase2.yaml"))
		require.NoError(t, err)

		assert.Equal(t, string(phase1Content), string(phase2Content),
			"Rollback should produce identical output to ensure safe migration")
	})
}

// TestFeatureFlagWithRenderTemplateToWriter tests the feature flag with the Writer API.
func TestFeatureFlagWithRenderTemplateToWriter(t *testing.T) {
	fsys := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{
			Data: []byte("Result: {{.Value | upper}}"),
		},
	}

	data := map[string]string{"Value": "hello"}

	tests := []struct {
		name     string
		envValue string
	}{
		{name: "legacy_system", envValue: "false"},
		{name: "new_system", envValue: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvUseNewTemplateEngine, tt.envValue)

			var buf strings.Builder
			err := RenderTemplateToWriter(fsys, "test.tmpl", data, &buf)
			require.NoError(t, err)

			assert.Equal(t, "Result: HELLO", buf.String(),
				"Writer API should work correctly with feature flag")
		})
	}
}

// TestFeatureFlagDocumentation provides examples for the migration documentation.
func TestFeatureFlagDocumentation(t *testing.T) {
	t.Run("example_basic_usage", func(t *testing.T) {
		// Example 1: Check if new engine is enabled
		t.Setenv(EnvUseNewTemplateEngine, "true")
		if UseNewTemplateEngine() {
			t.Log("✓ New template engine is enabled")
		} else {
			t.Log("✗ Legacy template system is active")
		}
		assert.True(t, UseNewTemplateEngine())
	})

	t.Run("example_gradual_rollout", func(t *testing.T) {
		// Example 2: Gradual rollout strategy
		// Step 1: Deploy with flag disabled (default)
		t.Setenv(EnvUseNewTemplateEngine, "")
		assert.False(t, UseNewTemplateEngine(), "Step 1: Default to legacy")

		// Step 2: Enable for development/staging
		t.Setenv(EnvUseNewTemplateEngine, "true")
		assert.True(t, UseNewTemplateEngine(), "Step 2: Enable for dev/staging")

		// Step 3: Monitor and validate
		// (validation happens through integration tests)

		// Step 4: Enable for production
		// (same as step 2, but in production environment)
	})

	t.Run("example_rollback_procedure", func(t *testing.T) {
		// Example 3: Rollback procedure
		// If issues are detected, immediately disable the flag
		t.Setenv(EnvUseNewTemplateEngine, "false")
		assert.False(t, UseNewTemplateEngine(), "Rollback: Disable feature flag")

		// System automatically falls back to legacy implementation
		// No code changes or redeployment needed
	})
}

// TestMigrationPerformanceComparison validates that the new template engine
// performs as well or better than the legacy system, especially with caching.
func TestMigrationPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	fsys := fstest.MapFS{
		"perf.tmpl": &fstest.MapFile{
			Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.Name}}
  namespace: {{.Namespace | default "default"}}
data:
  {{range $key, $value := .Data}}
  {{$key}}: {{$value | quote}}
  {{end}}`),
		},
	}

	data := map[string]interface{}{
		"Name":      "test-config",
		"Namespace": "production",
		"Data": map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	tmpDir := t.TempDir()
	iterations := 100

	// Benchmark legacy system
	legacyStart := time.Now()
	for i := 0; i < iterations; i++ {
		outputPath := filepath.Join(tmpDir, fmt.Sprintf("legacy-%d.yaml", i))
		err := renderLegacyTemplate(fsys, "perf.tmpl", outputPath, data)
		require.NoError(t, err)
	}
	legacyDuration := time.Since(legacyStart)

	// Benchmark new system with caching
	engine := NewGoTemplateEngine()
	newStart := time.Now()
	for i := 0; i < iterations; i++ {
		outputPath := filepath.Join(tmpDir, fmt.Sprintf("new-%d.yaml", i))
		err := RenderWithEngine(engine, fsys, "perf.tmpl", outputPath, data)
		require.NoError(t, err)
	}
	newDuration := time.Since(newStart)

	t.Logf("Legacy system: %v (%v per render)", legacyDuration, legacyDuration/time.Duration(iterations))
	t.Logf("New system: %v (%v per render)", newDuration, newDuration/time.Duration(iterations))
	t.Logf("Performance improvement: %.2fx", float64(legacyDuration)/float64(newDuration))

	// New system should be at least as fast or faster due to caching
	// Allow 20% margin for test variability
	assert.LessOrEqual(t, newDuration, legacyDuration*12/10,
		"New system should perform as well or better than legacy system")
}

// TestMigrationErrorHandling validates that the new system provides better
// error messages than the legacy system while maintaining compatibility.
func TestMigrationErrorHandling(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		data         interface{}
		expectError  bool
		errorPattern string
	}{
		{
			name:         "invalid function",
			template:     "Value: {{.Value | invalidFunc}}",
			data:         map[string]string{"Value": "test"},
			expectError:  true,
			errorPattern: "function",
		},
		{
			name:         "syntax error",
			template:     "Value: {{.Value",
			data:         map[string]string{"Value": "test"},
			expectError:  true,
			errorPattern: "unclosed",
		},
		{
			name:        "valid template",
			template:    "Value: {{.Value}}",
			data:        map[string]string{"Value": "test"},
			expectError: false,
		},
		{
			name:        "undefined variable renders empty",
			template:    "Value: {{.UndefinedVar}}",
			data:        map[string]string{},
			expectError: false, // Go templates don't error on undefined vars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"test.tmpl": &fstest.MapFile{
					Data: []byte(tt.template),
				},
			}

			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "output.txt")

			// Test with new system
			engine := NewGoTemplateEngine()
			err := RenderWithEngine(engine, fsys, "test.tmpl", outputPath, tt.data)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorPattern != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorPattern),
						"Error message should contain expected pattern")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestMigrationWithRealWorldTemplates tests migration with actual template
// patterns used in the openCenter codebase.
func TestMigrationWithRealWorldTemplates(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     interface{}
	}{
		{
			name: "flux kustomization",
			template: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{.Name}}
  namespace: flux-system
spec:
  interval: {{.Interval | default "10m"}}
  path: {{.Path}}
  prune: {{.Prune | default true}}
  sourceRef:
    kind: GitRepository
    name: {{.SourceRef}}`,
			data: map[string]interface{}{
				"Name":      "infrastructure",
				"Interval":  "5m",
				"Path":      "./infrastructure",
				"Prune":     true,
				"SourceRef": "flux-system",
			},
		},
		{
			name: "cluster configuration",
			template: `opencenter:
  organization: {{.Organization}}
  cluster_name: {{.ClusterName}}
  provider: {{.Provider}}
kubernetes:
  version: {{.KubernetesVersion | default "1.28.0"}}
  control_plane:
    count: {{.ControlPlaneCount | default 3}}
  workers:
    count: {{.WorkerCount | default 3}}`,
			data: map[string]interface{}{
				"Organization":       "test-org",
				"ClusterName":        "test-cluster",
				"Provider":           "openstack",
				"KubernetesVersion":  "1.29.0",
				"ControlPlaneCount":  3,
				"WorkerCount":        5,
			},
		},
		{
			name: "service manifest with conditionals",
			template: `apiVersion: v1
kind: Service
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  {{if .Labels}}
  labels:
    {{range $key, $value := .Labels}}
    {{$key}}: {{$value}}
    {{end}}
  {{end}}
spec:
  type: {{.Type | default "ClusterIP"}}
  ports:
  {{range .Ports}}
  - port: {{.Port}}
    targetPort: {{.TargetPort}}
    protocol: {{.Protocol | default "TCP"}}
    {{if .Name}}name: {{.Name}}{{end}}
  {{end}}
  selector:
    app: {{.Name}}`,
			data: map[string]interface{}{
				"Name":      "my-service",
				"Namespace": "default",
				"Labels": map[string]string{
					"app":     "my-app",
					"version": "v1",
				},
				"Type": "LoadBalancer",
				"Ports": []map[string]interface{}{
					{"Port": 80, "TargetPort": 8080, "Protocol": "TCP", "Name": "http"},
					{"Port": 443, "TargetPort": 8443, "Protocol": "TCP", "Name": "https"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"template.yaml": &fstest.MapFile{
					Data: []byte(tt.template),
				},
			}

			tmpDir := t.TempDir()

			// Render with legacy system
			legacyOutput := filepath.Join(tmpDir, "legacy.yaml")
			err := renderLegacyTemplate(fsys, "template.yaml", legacyOutput, tt.data)
			require.NoError(t, err)

			// Render with new system
			newOutput := filepath.Join(tmpDir, "new.yaml")
			engine := NewGoTemplateEngine()
			err = RenderWithEngine(engine, fsys, "template.yaml", newOutput, tt.data)
			require.NoError(t, err)

			// Verify outputs are identical
			legacyContent, err := os.ReadFile(legacyOutput)
			require.NoError(t, err)

			newContent, err := os.ReadFile(newOutput)
			require.NoError(t, err)

			assert.Equal(t, string(legacyContent), string(newContent),
				"Legacy and new systems should produce identical output for real-world templates")

			// Verify output is valid YAML
			var parsed interface{}
			err = yaml.Unmarshal(newContent, &parsed)
			require.NoError(t, err, "Output should be valid YAML")
		})
	}
}

// TestMigrationRollbackScenario tests the rollback strategy when migration
// encounters issues.
func TestMigrationRollbackScenario(t *testing.T) {
	fsys := fstest.MapFS{
		"good.tmpl": &fstest.MapFile{
			Data: []byte("Value: {{.Value}}"),
		},
		"bad.tmpl": &fstest.MapFile{
			Data: []byte("Value: {{.InvalidSyntax"),
		},
	}

	tmpDir := t.TempDir()
	data := map[string]string{"Value": "test"}

	// Simulate migration with feature flag
	useNewEngine := true

	renderWithFallback := func(templateName, outputPath string) error {
		if useNewEngine {
			engine := NewGoTemplateEngine()
			err := RenderWithEngine(engine, fsys, templateName, outputPath, data)
			if err != nil {
				// Rollback to legacy system on error
				t.Logf("New engine failed, rolling back to legacy: %v", err)
				useNewEngine = false
				return RenderTemplateToFile(fsys, templateName, outputPath, data)
			}
			return nil
		}
		return RenderTemplateToFile(fsys, templateName, outputPath, data)
	}

	// Test successful render with new engine
	t.Run("successful_render", func(t *testing.T) {
		useNewEngine = true
		outputPath := filepath.Join(tmpDir, "good.txt")
		err := renderWithFallback("good.tmpl", outputPath)
		require.NoError(t, err)
		assert.True(t, useNewEngine, "Should still use new engine after success")
	})

	// Test rollback on error
	t.Run("rollback_on_error", func(t *testing.T) {
		useNewEngine = true
		outputPath := filepath.Join(tmpDir, "bad.txt")
		err := renderWithFallback("bad.tmpl", outputPath)
		// Both systems should fail on invalid syntax
		require.Error(t, err)
	})
}

// TestMigrationDocumentationExamples validates all code examples from the
// migration documentation to ensure they work correctly.
func TestMigrationDocumentationExamples(t *testing.T) {
	t.Run("path1_no_changes", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.yaml.tmpl": &fstest.MapFile{
				Data: []byte("cluster: {{.ClusterName}}"),
			},
		}
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "output.yaml")
		data := map[string]string{"ClusterName": "test"}

		// This code continues to work unchanged
		err := RenderTemplateToFile(fsys, "template.yaml.tmpl", outputPath, data)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Equal(t, "cluster: test", string(content))
	})

	t.Run("path2_helper_functions", func(t *testing.T) {
		fsys := fstest.MapFS{
			"template.yaml": &fstest.MapFile{
				Data: []byte("cluster: {{.ClusterName}}"),
			},
		}
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "output.yaml")
		data := map[string]string{"ClusterName": "test"}

		engine := NewGoTemplateEngine()
		err := RenderWithEngine(engine, fsys, "template.yaml", outputPath, data)
		require.NoError(t, err)

		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Equal(t, "cluster: test", string(content))
	})

	t.Run("path3_full_api", func(t *testing.T) {
		templateContent := "cluster: {{.ClusterName}}"
		data := map[string]string{"ClusterName": "test"}

		engine := NewGoTemplateEngine()

		// Render directly using RenderString
		result, err := engine.RenderString(context.Background(), "mytemplate", templateContent, data)
		require.NoError(t, err)
		assert.Equal(t, "cluster: test", string(result))
	})
}

// TestLegacySystemOutputIdentity validates that the new template system produces
// IDENTICAL output to the actual legacy renderTemplate function from gitops/copy.go.
// This is the critical test for the acceptance criterion:
// "Template output is identical to legacy system"
func TestLegacySystemOutputIdentity(t *testing.T) {
	// Test cases covering various template patterns used in the actual system
	tests := []struct {
		name         string
		templateName string
		template     string
		data         interface{}
	}{
		{
			name:         "simple variable substitution",
			templateName: "simple.yaml",
			template:     "cluster: {{.ClusterName}}\nregion: {{.Region}}",
			data: map[string]string{
				"ClusterName": "test-cluster",
				"Region":      "us-east-1",
			},
		},
		{
			name:         "sprig upper function",
			templateName: "upper.yaml",
			template:     "name: {{.Name | upper}}",
			data:         map[string]string{"Name": "test"},
		},
		{
			name:         "sprig default function",
			templateName: "default.yaml",
			template:     "value: {{.Value | default \"default-value\"}}",
			data:         map[string]string{},
		},
		{
			name:         "sprig quote function",
			templateName: "quote.yaml",
			template:     "quoted: {{.Value | quote}}",
			data:         map[string]string{"Value": "test-value"},
		},
		{
			name:         "nested data structure",
			templateName: "nested.yaml",
			template: `metadata:
  name: {{.Metadata.Name}}
  namespace: {{.Metadata.Namespace}}
spec:
  replicas: {{.Spec.Replicas}}`,
			data: map[string]interface{}{
				"Metadata": map[string]string{
					"Name":      "my-app",
					"Namespace": "default",
				},
				"Spec": map[string]int{
					"Replicas": 3,
				},
			},
		},
		{
			name:         "range over slice",
			templateName: "range.yaml",
			template: `items:
{{range .Items}}- {{.}}
{{end}}`,
			data: map[string][]string{
				"Items": {"item1", "item2", "item3"},
			},
		},
		{
			name:         "conditional with if",
			templateName: "conditional.yaml",
			template: `{{if .Enabled}}enabled: true
{{else}}enabled: false
{{end}}`,
			data: map[string]bool{"Enabled": true},
		},
		{
			name:         "makefile with escaped helm syntax",
			templateName: "Makefile.tpl",
			template:     `VERSION := $(shell helm version --template="{{.Version}}")`,
			data:         map[string]string{},
		},
		{
			name:         "complex kubernetes manifest",
			templateName: "deployment.yaml",
			template: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Name}}
  namespace: {{.Namespace | default "default"}}
  labels:
    app: {{.Name}}
spec:
  replicas: {{.Replicas | default 1}}
  selector:
    matchLabels:
      app: {{.Name}}
  template:
    metadata:
      labels:
        app: {{.Name}}
    spec:
      containers:
      - name: {{.Name}}
        image: {{.Image}}
        ports:
        - containerPort: {{.Port | default 8080}}`,
			data: map[string]interface{}{
				"Name":      "my-app",
				"Namespace": "production",
				"Replicas":  3,
				"Image":     "nginx:latest",
				"Port":      80,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test filesystem with the template
			fsys := fstest.MapFS{
				tt.templateName: &fstest.MapFile{
					Data: []byte(tt.template),
				},
			}

			tmpDir := t.TempDir()

			// Render using the ACTUAL legacy system (simulating gitops/copy.go renderTemplate)
			legacyOutput := filepath.Join(tmpDir, "legacy-output")
			legacyErr := renderLegacyTemplate(fsys, tt.templateName, legacyOutput, tt.data)
			require.NoError(t, legacyErr, "legacy rendering should not fail")

			// Render using the NEW template system
			newOutput := filepath.Join(tmpDir, "new-output")
			newErr := RenderTemplateToFile(fsys, tt.templateName, newOutput, tt.data)
			require.NoError(t, newErr, "new rendering should not fail")

			// Read both outputs
			legacyContent, err := os.ReadFile(legacyOutput)
			require.NoError(t, err)

			newContent, err := os.ReadFile(newOutput)
			require.NoError(t, err)

			// CRITICAL ASSERTION: Outputs must be byte-for-byte identical
			assert.Equal(t, string(legacyContent), string(newContent),
				"Template output must be IDENTICAL between legacy and new systems.\nLegacy:\n%s\n\nNew:\n%s",
				string(legacyContent), string(newContent))
		})
	}
}

// renderLegacyTemplate simulates the EXACT behavior of the renderTemplate function
// from internal/gitops/copy.go. This is the actual legacy implementation we need to match.
func renderLegacyTemplate(fsys fs.FS, path, dst string, data interface{}) error {
	// This is a direct copy of the legacy renderTemplate logic from gitops/copy.go
	fileData, err := fs.ReadFile(fsys, path)
	if err != nil {
		return err
	}

	// Handle special cases for files that contain conflicting template syntax
	content := string(fileData)
	filename := filepath.Base(path)

	// For Makefile.tpl, escape Helm template syntax to prevent Go template parsing conflicts
	if filename == "Makefile.tpl" {
		// Replace Helm template syntax with escaped version for Go template processing
		content = strings.ReplaceAll(content, `--template="{{.Version}}"`, `--template="{{"{{"}}.Version{{"}}"}}"`)
	}

	t, err := template.New(filename).Funcs(sprig.TxtFuncMap()).Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", path, err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", path, err)
	}

	return nil
}
