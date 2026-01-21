package template

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateComposition_BasicOverlay(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	// Create base template
	baseTemplate := filepath.Join(tmpDir, "base.tmpl")
	baseContent := `name: {{ .Name }}
version: {{ .Version }}`
	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	// Create overlay template
	overlayTemplate := filepath.Join(tmpDir, "overlay.tmpl")
	overlayContent := `description: {{ .Description }}
author: {{ .Author }}`
	err = os.WriteFile(overlayTemplate, []byte(overlayContent), 0644)
	require.NoError(t, err)

	// Setup engine and registry
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register templates
	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "base",
		Path: baseTemplate,
		Type: TemplateTypeBase,
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "overlay",
		Path: overlayTemplate,
		Type: TemplateTypeOverlay,
	})
	require.NoError(t, err)

	// Create composition
	composition := TemplateComposition{
		BaseTemplate: baseTemplate,
		Overlays: []TemplateOverlay{
			{
				Name:     "overlay",
				Path:     overlayTemplate,
				Priority: 1,
			},
		},
	}

	// Test data
	data := map[string]interface{}{
		"Name":        "test-app",
		"Version":     "1.0.0",
		"Description": "A test application",
		"Author":      "Test Author",
	}

	// Compose
	composer := NewDefaultTemplateComposer(engine, registry)
	result, err := composer.Compose(context.Background(), composition, data)
	require.NoError(t, err)

	// Verify result contains both base and overlay content
	resultStr := string(result)
	assert.Contains(t, resultStr, "name: test-app")
	assert.Contains(t, resultStr, "version: 1.0.0")
	assert.Contains(t, resultStr, "description: A test application")
	assert.Contains(t, resultStr, "author: Test Author")
}

func TestTemplateComposition_MultipleOverlays(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	// Create base template
	baseTemplate := filepath.Join(tmpDir, "base.tmpl")
	baseContent := `base: true`
	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	// Create overlay templates with different priorities
	overlay1Template := filepath.Join(tmpDir, "overlay1.tmpl")
	overlay1Content := `overlay1: priority-1`
	err = os.WriteFile(overlay1Template, []byte(overlay1Content), 0644)
	require.NoError(t, err)

	overlay2Template := filepath.Join(tmpDir, "overlay2.tmpl")
	overlay2Content := `overlay2: priority-2`
	err = os.WriteFile(overlay2Template, []byte(overlay2Content), 0644)
	require.NoError(t, err)

	overlay3Template := filepath.Join(tmpDir, "overlay3.tmpl")
	overlay3Content := `overlay3: priority-3`
	err = os.WriteFile(overlay3Template, []byte(overlay3Content), 0644)
	require.NoError(t, err)

	// Setup engine and registry
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register templates
	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "base",
		Path: baseTemplate,
		Type: TemplateTypeBase,
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "overlay1",
		Path: overlay1Template,
		Type: TemplateTypeOverlay,
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "overlay2",
		Path: overlay2Template,
		Type: TemplateTypeOverlay,
	})
	require.NoError(t, err)

	regErr := registry.RegisterTemplate(TemplateDefinition{
		Name: "overlay3",
		Path: overlay3Template,
		Type: TemplateTypeOverlay,
	})
	require.NoError(t, regErr)

	// Create composition with overlays in non-priority order
	composition := TemplateComposition{
		BaseTemplate: baseTemplate,
		Overlays: []TemplateOverlay{
			{
				Name:     "overlay1",
				Path:     overlay1Template,
				Priority: 1,
			},
			{
				Name:     "overlay3",
				Path:     overlay3Template,
				Priority: 3,
			},
			{
				Name:     "overlay2",
				Path:     overlay2Template,
				Priority: 2,
			},
		},
	}

	// Compose
	composer := NewDefaultTemplateComposer(engine, registry)
	result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
	require.NoError(t, err)

	// Verify overlays are applied in priority order (highest first)
	resultStr := string(result)
	assert.Contains(t, resultStr, "base: true")
	assert.Contains(t, resultStr, "overlay1: priority-1")
	assert.Contains(t, resultStr, "overlay2: priority-2")
	assert.Contains(t, resultStr, "overlay3: priority-3")

	// Verify order: base, then overlay3 (priority 3), then overlay2 (priority 2), then overlay1 (priority 1)
	baseIdx := indexOfSubstring(resultStr, "base: true")
	overlay3Idx := indexOfSubstring(resultStr, "overlay3: priority-3")
	overlay2Idx := indexOfSubstring(resultStr, "overlay2: priority-2")
	overlay1Idx := indexOfSubstring(resultStr, "overlay1: priority-1")

	assert.True(t, baseIdx < overlay3Idx, "base should come before overlay3")
	assert.True(t, overlay3Idx < overlay2Idx, "overlay3 should come before overlay2")
	assert.True(t, overlay2Idx < overlay1Idx, "overlay2 should come before overlay1")
}

func TestTemplateComposition_ConditionalOverlay(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	// Create base template
	baseTemplate := filepath.Join(tmpDir, "base.tmpl")
	baseContent := `base: true`
	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	// Create conditional overlay
	overlayTemplate := filepath.Join(tmpDir, "overlay.tmpl")
	overlayContent := `overlay: applied`
	err = os.WriteFile(overlayTemplate, []byte(overlayContent), 0644)
	require.NoError(t, err)

	// Setup engine and registry
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register templates
	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "base",
		Path: baseTemplate,
		Type: TemplateTypeBase,
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "overlay",
		Path: overlayTemplate,
		Type: TemplateTypeOverlay,
	})
	require.NoError(t, err)

	// Test with condition met
	t.Run("condition met", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Overlays: []TemplateOverlay{
				{
					Name:     "overlay",
					Path:     overlayTemplate,
					Priority: 1,
					Conditions: []RenderCondition{
						{
							Type:  ConditionTypeEquals,
							Field: "enabled",
							Value: true,
						},
					},
				},
			},
		}

		data := map[string]interface{}{
			"enabled": true,
		}

		composer := NewDefaultTemplateComposer(engine, registry)
		result, err := composer.Compose(context.Background(), composition, data)
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "base: true")
		assert.Contains(t, resultStr, "overlay: applied")
	})

	// Test with condition not met
	t.Run("condition not met", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Overlays: []TemplateOverlay{
				{
					Name:     "overlay",
					Path:     overlayTemplate,
					Priority: 1,
					Conditions: []RenderCondition{
						{
							Type:  ConditionTypeEquals,
							Field: "enabled",
							Value: true,
						},
					},
				},
			},
		}

		data := map[string]interface{}{
			"enabled": false,
		}

		composer := NewDefaultTemplateComposer(engine, registry)
		result, err := composer.Compose(context.Background(), composition, data)
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "base: true")
		assert.NotContains(t, resultStr, "overlay: applied")
	})
}

func TestTemplateComposition_ValidateComposition(t *testing.T) {
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register a test template
	err := registry.RegisterTemplate(TemplateDefinition{
		Name: "test-template",
		Path: "/tmp/test.tmpl",
		Type: TemplateTypeBase,
	})
	require.NoError(t, err)

	composer := NewDefaultTemplateComposer(engine, registry)

	tests := []struct {
		name        string
		composition TemplateComposition
		wantErr     bool
		errContains string
	}{
		{
			name: "valid composition",
			composition: TemplateComposition{
				BaseTemplate: "test-template",
				Overlays: []TemplateOverlay{
					{
						Name:     "test-template",
						Path:     "/tmp/overlay.tmpl",
						Priority: 1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing base template",
			composition: TemplateComposition{
				BaseTemplate: "",
			},
			wantErr:     true,
			errContains: "base template is required",
		},
		{
			name: "overlay missing name",
			composition: TemplateComposition{
				BaseTemplate: "test-template",
				Overlays: []TemplateOverlay{
					{
						Name:     "",
						Path:     "/tmp/overlay.tmpl",
						Priority: 1,
					},
				},
			},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "overlay missing path",
			composition: TemplateComposition{
				BaseTemplate: "test-template",
				Overlays: []TemplateOverlay{
					{
						Name:     "overlay",
						Path:     "",
						Priority: 1,
					},
				},
			},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name: "patch missing operation",
			composition: TemplateComposition{
				BaseTemplate: "test-template",
				Patches: []TemplatePatch{
					{
						Operation: "",
						Path:      "$.field",
						Value:     "value",
					},
				},
			},
			wantErr:     true,
			errContains: "operation is required",
		},
		{
			name: "patch missing path",
			composition: TemplateComposition{
				BaseTemplate: "test-template",
				Patches: []TemplatePatch{
					{
						Operation: "add",
						Path:      "",
						Value:     "value",
					},
				},
			},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name: "patch invalid operation",
			composition: TemplateComposition{
				BaseTemplate: "test-template",
				Patches: []TemplatePatch{
					{
						Operation: "invalid",
						Path:      "$.field",
						Value:     "value",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := composer.ValidateComposition(tt.composition)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTemplateComposition_Patches(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	// Create base template
	baseTemplate := filepath.Join(tmpDir, "base.tmpl")
	baseContent := `line1: value1
line2: value2
line3: value3`
	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	// Setup engine and registry
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register template
	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "base",
		Path: baseTemplate,
		Type: TemplateTypeBase,
	})
	require.NoError(t, err)

	composer := NewDefaultTemplateComposer(engine, registry)

	t.Run("add patch - append to end", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      ".",
					Value:     "line4: value4",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line1: value1")
		assert.Contains(t, resultStr, "line4: value4")
	})

	t.Run("add patch - insert after line number", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      "line:1",
					Value:     "inserted: after-line-1",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		lines := strings.Split(resultStr, "\n")

		// Should have original 3 lines + 1 inserted = 4 lines (plus potential empty line)
		assert.GreaterOrEqual(t, len(lines), 4)
		assert.Equal(t, "line1: value1", lines[0])
		assert.Equal(t, "inserted: after-line-1", lines[1])
		assert.Equal(t, "line2: value2", lines[2])
	})

	t.Run("add patch - insert after key pattern", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      "line2",
					Value:     "inserted: after-line2",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line2: value2")
		assert.Contains(t, resultStr, "inserted: after-line2")

		// Verify insertion order
		line2Idx := strings.Index(resultStr, "line2: value2")
		insertedIdx := strings.Index(resultStr, "inserted: after-line2")
		assert.True(t, line2Idx < insertedIdx, "inserted content should come after line2")
	})

	t.Run("add patch - invalid line number", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      "line:999",
					Value:     "should-fail",
				},
			},
		}

		_, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("remove patch - by line number", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "remove",
					Path:      "line:1",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line1: value1")
		assert.NotContains(t, resultStr, "line2: value2")
		assert.Contains(t, resultStr, "line3: value3")
	})

	t.Run("remove patch - by line range", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "remove",
					Path:      "lines:1-2",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line1: value1")
		assert.NotContains(t, resultStr, "line2: value2")
		assert.NotContains(t, resultStr, "line3: value3")
	})

	t.Run("remove patch - by pattern", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "remove",
					Path:      "line2",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line1: value1")
		assert.NotContains(t, resultStr, "line2: value2")
		assert.Contains(t, resultStr, "line3: value3")
	})

	t.Run("remove patch - pattern not found", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "remove",
					Path:      "nonexistent",
				},
			},
		}

		_, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no lines found matching")
	})

	t.Run("replace patch - by line number", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "replace",
					Path:      "line:1",
					Value:     "line2: replaced-value",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line1: value1")
		assert.Contains(t, resultStr, "line2: replaced-value")
		assert.NotContains(t, resultStr, "line2: value2")
	})

	t.Run("replace patch - by pattern with YAML key", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "replace",
					Path:      "line2",
					Value:     "new-value",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line1: value1")
		assert.Contains(t, resultStr, "line2: new-value")
		assert.NotContains(t, resultStr, "line2: value2")
	})

	t.Run("replace patch - pattern not found", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "replace",
					Path:      "nonexistent",
					Value:     "should-fail",
				},
			},
		}

		_, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no lines found matching")
	})

	t.Run("multiple patches in sequence", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      ".",
					Value:     "line4: value4",
				},
				{
					Operation: "replace",
					Path:      "line2",
					Value:     "modified",
				},
				{
					Operation: "remove",
					Path:      "line3",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "line1: value1")
		assert.Contains(t, resultStr, "line2: modified")
		assert.NotContains(t, resultStr, "line3: value3")
		assert.Contains(t, resultStr, "line4: value4")
	})

	t.Run("patch with condition - met", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      ".",
					Value:     "conditional: added",
					Condition: RenderCondition{
						Type:  ConditionTypeEquals,
						Field: "enabled",
						Value: true,
					},
				},
			},
		}

		data := map[string]interface{}{
			"enabled": true,
		}

		result, err := composer.Compose(context.Background(), composition, data)
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "conditional: added")
	})

	t.Run("patch with condition - not met", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      ".",
					Value:     "conditional: added",
					Condition: RenderCondition{
						Type:  ConditionTypeEquals,
						Field: "enabled",
						Value: true,
					},
				},
			},
		}

		data := map[string]interface{}{
			"enabled": false,
		}

		result, err := composer.Compose(context.Background(), composition, data)
		require.NoError(t, err)

		resultStr := string(result)
		assert.NotContains(t, resultStr, "conditional: added")
	})
}

func TestTemplateComposition_PatchEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Create base template with indented YAML
	baseTemplate := filepath.Join(tmpDir, "base.tmpl")
	baseContent := `metadata:
  name: test
  labels:
    app: myapp
spec:
  replicas: 3`
	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()
	composer := NewDefaultTemplateComposer(engine, registry)

	t.Run("replace preserves YAML structure", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "replace",
					Path:      "replicas",
					Value:     "5",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "replicas: 5")
		assert.NotContains(t, resultStr, "replicas: 3")
		// Verify indentation is preserved
		assert.Contains(t, resultStr, "  replicas: 5")
	})

	t.Run("add after specific key in nested structure", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "add",
					Path:      "labels",
					Value:     "    version: v1.0",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.Contains(t, resultStr, "labels:")
		assert.Contains(t, resultStr, "version: v1.0")
	})

	t.Run("remove from nested structure", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				{
					Operation: "remove",
					Path:      "app: myapp",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)
		assert.NotContains(t, resultStr, "app: myapp")
		assert.Contains(t, resultStr, "labels:")
	})
}

func TestTemplateComposition_ConditionEvaluation(t *testing.T) {
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()
	composer := NewDefaultTemplateComposer(engine, registry)

	tests := []struct {
		name      string
		condition RenderCondition
		data      map[string]interface{}
		want      bool
		wantErr   bool
	}{
		{
			name: "equals - true",
			condition: RenderCondition{
				Type:  ConditionTypeEquals,
				Field: "provider",
				Value: "openstack",
			},
			data: map[string]interface{}{
				"provider": "openstack",
			},
			want: true,
		},
		{
			name: "equals - false",
			condition: RenderCondition{
				Type:  ConditionTypeEquals,
				Field: "provider",
				Value: "aws",
			},
			data: map[string]interface{}{
				"provider": "openstack",
			},
			want: false,
		},
		{
			name: "not equals - true",
			condition: RenderCondition{
				Type:  ConditionTypeNotEquals,
				Field: "provider",
				Value: "aws",
			},
			data: map[string]interface{}{
				"provider": "openstack",
			},
			want: true,
		},
		{
			name: "contains - true",
			condition: RenderCondition{
				Type:  ConditionTypeContains,
				Field: "name",
				Value: "test",
			},
			data: map[string]interface{}{
				"name": "test-cluster",
			},
			want: true,
		},
		{
			name: "contains - false",
			condition: RenderCondition{
				Type:  ConditionTypeContains,
				Field: "name",
				Value: "prod",
			},
			data: map[string]interface{}{
				"name": "test-cluster",
			},
			want: false,
		},
		{
			name: "exists - true",
			condition: RenderCondition{
				Type:  ConditionTypeExists,
				Field: "provider",
			},
			data: map[string]interface{}{
				"provider": "openstack",
			},
			want: true,
		},
		{
			name: "exists - false",
			condition: RenderCondition{
				Type:  ConditionTypeExists,
				Field: "missing",
			},
			data: map[string]interface{}{
				"provider": "openstack",
			},
			want:    false,
			wantErr: true, // Field not found
		},
		{
			name: "greater than - true",
			condition: RenderCondition{
				Type:  ConditionTypeGreaterThan,
				Field: "count",
				Value: 5,
			},
			data: map[string]interface{}{
				"count": 10,
			},
			want: true,
		},
		{
			name: "greater than - false",
			condition: RenderCondition{
				Type:  ConditionTypeGreaterThan,
				Field: "count",
				Value: 10,
			},
			data: map[string]interface{}{
				"count": 5,
			},
			want: false,
		},
		{
			name: "less than - true",
			condition: RenderCondition{
				Type:  ConditionTypeLessThan,
				Field: "count",
				Value: 10,
			},
			data: map[string]interface{}{
				"count": 5,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := composer.evaluateCondition(tt.condition, tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// Helper function to find index of substring
func indexOfSubstring(s, substr string) int {
	return strings.Index(s, substr)
}

func TestTemplateComposition_ConfigurableOrdering(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	// Create base template
	baseTemplate := filepath.Join(tmpDir, "base.tmpl")
	baseContent := `base: true`
	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	// Create overlay templates
	overlayATemplate := filepath.Join(tmpDir, "overlayA.tmpl")
	overlayAContent := `overlayA: priority-2`
	err = os.WriteFile(overlayATemplate, []byte(overlayAContent), 0644)
	require.NoError(t, err)

	overlayBTemplate := filepath.Join(tmpDir, "overlayB.tmpl")
	overlayBContent := `overlayB: priority-1`
	err = os.WriteFile(overlayBTemplate, []byte(overlayBContent), 0644)
	require.NoError(t, err)

	overlayCTemplate := filepath.Join(tmpDir, "overlayC.tmpl")
	overlayCContent := `overlayC: priority-2`
	err = os.WriteFile(overlayCTemplate, []byte(overlayCContent), 0644)
	require.NoError(t, err)

	// Setup engine and registry
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register templates
	err = registry.RegisterTemplate(TemplateDefinition{
		Name: "base",
		Path: baseTemplate,
		Type: TemplateTypeBase,
	})
	require.NoError(t, err)

	// Create composition with overlays
	composition := TemplateComposition{
		BaseTemplate: baseTemplate,
		Overlays: []TemplateOverlay{
			{
				Name:     "overlayB",
				Path:     overlayBTemplate,
				Priority: 1,
			},
			{
				Name:     "overlayA",
				Path:     overlayATemplate,
				Priority: 2,
			},
			{
				Name:     "overlayC",
				Path:     overlayCTemplate,
				Priority: 2,
			},
		},
	}

	t.Run("default ordering - priority desc", func(t *testing.T) {
		composer := NewDefaultTemplateComposer(engine, registry)
		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)

		// With priority desc: overlayA (priority 2, name A), overlayC (priority 2, name C), overlayB (priority 1)
		baseIdx := indexOfSubstring(resultStr, "base: true")
		overlayAIdx := indexOfSubstring(resultStr, "overlayA: priority-2")
		overlayCIdx := indexOfSubstring(resultStr, "overlayC: priority-2")
		overlayBIdx := indexOfSubstring(resultStr, "overlayB: priority-1")

		assert.True(t, baseIdx < overlayAIdx, "base should come before overlayA")
		assert.True(t, overlayAIdx < overlayCIdx, "overlayA should come before overlayC (same priority, sorted by name)")
		assert.True(t, overlayCIdx < overlayBIdx, "overlayC should come before overlayB (higher priority)")
	})

	t.Run("priority ascending ordering", func(t *testing.T) {
		composer := NewDefaultTemplateComposer(engine, registry)
		composer.SetOrderingConfig(OverlayOrderingConfig{
			Strategy: OrderByPriorityAsc,
		})

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)

		// With priority asc: overlayB (priority 1), overlayA (priority 2, name A), overlayC (priority 2, name C)
		baseIdx := indexOfSubstring(resultStr, "base: true")
		overlayAIdx := indexOfSubstring(resultStr, "overlayA: priority-2")
		overlayCIdx := indexOfSubstring(resultStr, "overlayC: priority-2")
		overlayBIdx := indexOfSubstring(resultStr, "overlayB: priority-1")

		assert.True(t, baseIdx < overlayBIdx, "base should come before overlayB")
		assert.True(t, overlayBIdx < overlayAIdx, "overlayB should come before overlayA (lower priority first)")
		assert.True(t, overlayAIdx < overlayCIdx, "overlayA should come before overlayC (same priority, sorted by name)")
	})

	t.Run("name ordering", func(t *testing.T) {
		composer := NewDefaultTemplateComposer(engine, registry)
		composer.SetOrderingConfig(OverlayOrderingConfig{
			Strategy: OrderByName,
		})

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)

		// With name ordering: overlayA, overlayB, overlayC (alphabetical)
		baseIdx := indexOfSubstring(resultStr, "base: true")
		overlayAIdx := indexOfSubstring(resultStr, "overlayA: priority-2")
		overlayBIdx := indexOfSubstring(resultStr, "overlayB: priority-1")
		overlayCIdx := indexOfSubstring(resultStr, "overlayC: priority-2")

		assert.True(t, baseIdx < overlayAIdx, "base should come before overlayA")
		assert.True(t, overlayAIdx < overlayBIdx, "overlayA should come before overlayB (alphabetical)")
		assert.True(t, overlayBIdx < overlayCIdx, "overlayB should come before overlayC (alphabetical)")
	})

	t.Run("registration ordering", func(t *testing.T) {
		composer := NewDefaultTemplateComposer(engine, registry)
		composer.SetOrderingConfig(OverlayOrderingConfig{
			Strategy: OrderByRegistration,
		})

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)

		// With registration ordering: overlayB, overlayA, overlayC (as provided in composition)
		baseIdx := indexOfSubstring(resultStr, "base: true")
		overlayAIdx := indexOfSubstring(resultStr, "overlayA: priority-2")
		overlayBIdx := indexOfSubstring(resultStr, "overlayB: priority-1")
		overlayCIdx := indexOfSubstring(resultStr, "overlayC: priority-2")

		assert.True(t, baseIdx < overlayBIdx, "base should come before overlayB")
		assert.True(t, overlayBIdx < overlayAIdx, "overlayB should come before overlayA (registration order)")
		assert.True(t, overlayAIdx < overlayCIdx, "overlayA should come before overlayC (registration order)")
	})

	t.Run("custom ordering function", func(t *testing.T) {
		composer := NewDefaultTemplateComposer(engine, registry)

		// Custom sort: reverse alphabetical order
		composer.SetOrderingConfig(OverlayOrderingConfig{
			CustomSort: func(overlays []TemplateOverlay) []TemplateOverlay {
				sorted := make([]TemplateOverlay, len(overlays))
				copy(sorted, overlays)
				sort.Slice(sorted, func(i, j int) bool {
					return sorted[i].Name > sorted[j].Name // Reverse alphabetical
				})
				return sorted
			},
		})

		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		resultStr := string(result)

		// With custom reverse alphabetical: overlayC, overlayB, overlayA
		baseIdx := indexOfSubstring(resultStr, "base: true")
		overlayAIdx := indexOfSubstring(resultStr, "overlayA: priority-2")
		overlayBIdx := indexOfSubstring(resultStr, "overlayB: priority-1")
		overlayCIdx := indexOfSubstring(resultStr, "overlayC: priority-2")

		assert.True(t, baseIdx < overlayCIdx, "base should come before overlayC")
		assert.True(t, overlayCIdx < overlayBIdx, "overlayC should come before overlayB (reverse alphabetical)")
		assert.True(t, overlayBIdx < overlayAIdx, "overlayB should come before overlayA (reverse alphabetical)")
	})
}

func TestTemplateComposition_GetSetOrderingConfig(t *testing.T) {
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()
	composer := NewDefaultTemplateComposer(engine, registry)

	// Test default config
	defaultConfig := composer.GetOrderingConfig()
	assert.Equal(t, OrderByPriorityDesc, defaultConfig.Strategy)
	assert.Nil(t, defaultConfig.CustomSort)

	// Test setting new config
	newConfig := OverlayOrderingConfig{
		Strategy: OrderByName,
	}
	composer.SetOrderingConfig(newConfig)

	retrievedConfig := composer.GetOrderingConfig()
	assert.Equal(t, OrderByName, retrievedConfig.Strategy)
	assert.Nil(t, retrievedConfig.CustomSort)

	// Test setting config with custom sort
	customConfig := OverlayOrderingConfig{
		Strategy: OrderByPriorityDesc,
		CustomSort: func(overlays []TemplateOverlay) []TemplateOverlay {
			return overlays
		},
	}
	composer.SetOrderingConfig(customConfig)

	retrievedConfig = composer.GetOrderingConfig()
	assert.Equal(t, OrderByPriorityDesc, retrievedConfig.Strategy)
	assert.NotNil(t, retrievedConfig.CustomSort)
}

func TestTemplateComposition_DeterministicOrdering(t *testing.T) {
	// This test verifies that overlay ordering is deterministic across multiple runs
	tmpDir := t.TempDir()

	// Create base template
	baseTemplate := filepath.Join(tmpDir, "base.tmpl")
	baseContent := `base: true`
	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	// Create multiple overlays with same priority
	overlays := []struct {
		name    string
		content string
	}{
		{"overlay1", "overlay1: data"},
		{"overlay2", "overlay2: data"},
		{"overlay3", "overlay3: data"},
		{"overlay4", "overlay4: data"},
		{"overlay5", "overlay5: data"},
	}

	var templateOverlays []TemplateOverlay
	for _, o := range overlays {
		overlayPath := filepath.Join(tmpDir, o.name+".tmpl")
		err := os.WriteFile(overlayPath, []byte(o.content), 0644)
		require.NoError(t, err)

		templateOverlays = append(templateOverlays, TemplateOverlay{
			Name:     o.name,
			Path:     overlayPath,
			Priority: 1, // All same priority
		})
	}

	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()
	composer := NewDefaultTemplateComposer(engine, registry)

	composition := TemplateComposition{
		BaseTemplate: baseTemplate,
		Overlays:     templateOverlays,
	}

	// Run composition multiple times and verify results are identical
	var firstResult string
	for i := 0; i < 10; i++ {
		result, err := composer.Compose(context.Background(), composition, map[string]interface{}{})
		require.NoError(t, err)

		if i == 0 {
			firstResult = string(result)
		} else {
			assert.Equal(t, firstResult, string(result), "Results should be identical across runs")
		}
	}

	// Verify overlays are in alphabetical order (since they have same priority)
	resultStr := firstResult
	overlay1Idx := indexOfSubstring(resultStr, "overlay1: data")
	overlay2Idx := indexOfSubstring(resultStr, "overlay2: data")
	overlay3Idx := indexOfSubstring(resultStr, "overlay3: data")
	overlay4Idx := indexOfSubstring(resultStr, "overlay4: data")
	overlay5Idx := indexOfSubstring(resultStr, "overlay5: data")

	assert.True(t, overlay1Idx < overlay2Idx, "overlay1 should come before overlay2")
	assert.True(t, overlay2Idx < overlay3Idx, "overlay2 should come before overlay3")
	assert.True(t, overlay3Idx < overlay4Idx, "overlay3 should come before overlay4")
	assert.True(t, overlay4Idx < overlay5Idx, "overlay4 should come before overlay5")
}

func TestTemplateComposition_PatchSystemDocumentation(t *testing.T) {
	// This test serves as documentation for the patch system capabilities
	tmpDir := t.TempDir()

	// Create a realistic Kubernetes deployment template
	baseTemplate := filepath.Join(tmpDir, "deployment.tmpl")
	baseContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  replicas: 3
  selector:
    matchLabels:
      app: {{ .Name }}
  template:
    metadata:
      labels:
        app: {{ .Name }}
    spec:
      containers:
      - name: {{ .Name }}
        image: {{ .Image }}
        ports:
        - containerPort: 8080`

	err := os.WriteFile(baseTemplate, []byte(baseContent), 0644)
	require.NoError(t, err)

	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()
	composer := NewDefaultTemplateComposer(engine, registry)

	data := map[string]interface{}{
		"Name":        "myapp",
		"Namespace":   "production",
		"Image":       "myapp:1.0.0",
		"Environment": "production",
	}

	t.Run("comprehensive patch example", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				// Add a label after the existing labels
				{
					Operation: "add",
					Path:      "app: myapp",
					Value:     "        version: 1.0.0",
				},
				// Replace the replica count
				{
					Operation: "replace",
					Path:      "replicas",
					Value:     "5",
				},
				// Add environment variables (insert after image line)
				{
					Operation: "add",
					Path:      "image:",
					Value: `        env:
        - name: ENV
          value: production`,
				},
				// Add resource limits at the end
				{
					Operation: "add",
					Path:      ".",
					Value: `        resources:
          limits:
            cpu: "1"
            memory: "512Mi"`,
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, data)
		require.NoError(t, err)

		resultStr := string(result)

		// Verify all patches were applied
		assert.Contains(t, resultStr, "replicas: 5")
		assert.Contains(t, resultStr, "version: 1.0.0")
		assert.Contains(t, resultStr, "ENV")
		assert.Contains(t, resultStr, "resources:")
		assert.Contains(t, resultStr, "cpu: \"1\"")
	})

	t.Run("conditional patches based on environment", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				// Add debug annotations only in development
				{
					Operation: "add",
					Path:      "namespace:",
					Value:     "  annotations:\n    debug: \"true\"",
					Condition: RenderCondition{
						Type:  ConditionTypeEquals,
						Field: "Environment",
						Value: "development",
					},
				},
				// Increase replicas in production
				{
					Operation: "replace",
					Path:      "replicas",
					Value:     "10",
					Condition: RenderCondition{
						Type:  ConditionTypeEquals,
						Field: "Environment",
						Value: "production",
					},
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, data)
		require.NoError(t, err)

		resultStr := string(result)

		// Debug annotations should NOT be added (we're in production)
		assert.NotContains(t, resultStr, "debug: \"true\"")

		// Replicas should be increased (we're in production)
		assert.Contains(t, resultStr, "replicas: 10")
	})

	t.Run("line-based operations", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: baseTemplate,
			Patches: []TemplatePatch{
				// Insert a comment at line 2
				{
					Operation: "add",
					Path:      "line:2",
					Value:     "# This is a production deployment",
				},
				// Remove the selector section by pattern
				{
					Operation: "remove",
					Path:      "selector:",
				},
			},
		}

		result, err := composer.Compose(context.Background(), composition, data)
		require.NoError(t, err)

		resultStr := string(result)

		// Comment should be inserted
		assert.Contains(t, resultStr, "# This is a production deployment")

		// Selector line should be removed
		assert.NotContains(t, resultStr, "selector:")
	})
}

func TestTemplateComposition_CompatibilityValidation(t *testing.T) {
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register base templates with different providers
	err := registry.RegisterTemplate(TemplateDefinition{
		Name:     "openstack-base",
		Path:     "/tmp/openstack-base.tmpl",
		Type:     TemplateTypeBase,
		Provider: "openstack",
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(TemplateDefinition{
		Name:     "aws-base",
		Path:     "/tmp/aws-base.tmpl",
		Type:     TemplateTypeBase,
		Provider: "aws",
	})
	require.NoError(t, err)

	// Register overlays with different providers
	err = registry.RegisterTemplate(TemplateDefinition{
		Name:     "openstack-overlay",
		Path:     "/tmp/openstack-overlay.tmpl",
		Type:     TemplateTypeOverlay,
		Provider: "openstack",
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(TemplateDefinition{
		Name:     "aws-overlay",
		Path:     "/tmp/aws-overlay.tmpl",
		Type:     TemplateTypeOverlay,
		Provider: "aws",
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(TemplateDefinition{
		Name:     "universal-overlay",
		Path:     "/tmp/universal-overlay.tmpl",
		Type:     TemplateTypeOverlay,
		Provider: "", // Universal - no provider specified
	})
	require.NoError(t, err)

	// Register overlay with service type (incompatible)
	err = registry.RegisterTemplate(TemplateDefinition{
		Name:     "service-template",
		Path:     "/tmp/service.tmpl",
		Type:     TemplateTypeService,
		Provider: "openstack",
	})
	require.NoError(t, err)

	composer := NewDefaultTemplateComposer(engine, registry)

	t.Run("compatible provider overlay", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: "openstack-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "openstack-overlay",
					Path:     "/tmp/openstack-overlay.tmpl",
					Priority: 1,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		assert.NoError(t, err)
	})

	t.Run("incompatible provider overlay", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: "openstack-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "aws-overlay",
					Path:     "/tmp/aws-overlay.tmpl",
					Priority: 1,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider")
		assert.Contains(t, err.Error(), "incompatible")
	})

	t.Run("universal overlay is compatible with any provider", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: "openstack-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "universal-overlay",
					Path:     "/tmp/universal-overlay.tmpl",
					Priority: 1,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		assert.NoError(t, err)
	})

	t.Run("incompatible template type", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: "openstack-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "service-template",
					Path:     "/tmp/service.tmpl",
					Priority: 1,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "type")
		assert.Contains(t, err.Error(), "incompatible")
	})

	t.Run("duplicate overlay names", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: "openstack-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "openstack-overlay",
					Path:     "/tmp/openstack-overlay.tmpl",
					Priority: 1,
				},
				{
					Name:     "openstack-overlay",
					Path:     "/tmp/openstack-overlay-2.tmpl",
					Priority: 2,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate overlay name detected")
		assert.Contains(t, err.Error(), "openstack-overlay")
		assert.Contains(t, err.Error(), "positions 0 and 1")
		assert.Contains(t, err.Error(), "Resolution:")
		assert.Contains(t, err.Error(), "Rename one of the overlays")
	})

	t.Run("conflicting providers in overlays", func(t *testing.T) {
		// This test verifies that when multiple overlays have different providers,
		// the validation catches it. In this case, the first overlay (openstack) is compatible
		// with the base, but the second overlay (aws) is not compatible with the base.
		// The error will be caught at the individual overlay validation level.
		composition := TemplateComposition{
			BaseTemplate: "openstack-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "openstack-overlay",
					Path:     "/tmp/openstack-overlay.tmpl",
					Priority: 1,
				},
				{
					Name:     "aws-overlay",
					Path:     "/tmp/aws-overlay.tmpl",
					Priority: 2,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		assert.Error(t, err)
		// The error will mention provider incompatibility with enhanced details
		assert.Contains(t, err.Error(), "provider")
		assert.Contains(t, err.Error(), "Resolution")
	})

	t.Run("enhanced error message for type mismatch", func(t *testing.T) {
		// Register templates with incompatible types
		err := registry.RegisterTemplate(TemplateDefinition{
			Name:     "service-base",
			Path:     "/tmp/service-base.tmpl",
			Type:     TemplateTypeService,
			Provider: "openstack",
		})
		require.NoError(t, err)

		err = registry.RegisterTemplate(TemplateDefinition{
			Name:     "infra-overlay",
			Path:     "/tmp/infra-overlay.tmpl",
			Type:     TemplateTypeInfrastructure,
			Provider: "openstack",
		})
		require.NoError(t, err)

		composition := TemplateComposition{
			BaseTemplate: "service-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "infra-overlay",
					Path:     "/tmp/infra-overlay.tmpl",
					Priority: 1,
				},
			},
		}

		err = composer.ValidateComposition(composition)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "incompatible template types")
		assert.Contains(t, err.Error(), "Resolution Options:")
		assert.Contains(t, err.Error(), "Impact:")
	})

	t.Run("enhanced error message for circular dependency", func(t *testing.T) {
		// Test circular dependency detection during template registration
		// The registry will catch circular dependencies when templates are registered
		err := registry.RegisterTemplate(TemplateDefinition{
			Name:         "self-dependent-overlay",
			Path:         "/tmp/self-dependent.tmpl",
			Type:         TemplateTypeOverlay,
			Provider:     "openstack",
			Dependencies: []string{"self-dependent-overlay"}, // Depends on itself
		})

		// The registry should catch this circular dependency
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template cannot depend on itself")
	})

	t.Run("enhanced error message for provider conflict between overlays", func(t *testing.T) {
		// Register overlays with different providers
		err := registry.RegisterTemplate(TemplateDefinition{
			Name:     "openstack-specific",
			Path:     "/tmp/openstack-specific.tmpl",
			Type:     TemplateTypeOverlay,
			Provider: "openstack",
		})
		require.NoError(t, err)

		err = registry.RegisterTemplate(TemplateDefinition{
			Name:     "aws-specific",
			Path:     "/tmp/aws-specific.tmpl",
			Type:     TemplateTypeOverlay,
			Provider: "aws",
		})
		require.NoError(t, err)

		composition := TemplateComposition{
			BaseTemplate: "openstack-base",
			Overlays: []TemplateOverlay{
				{
					Name:     "openstack-specific",
					Path:     "/tmp/openstack-specific.tmpl",
					Priority: 1,
				},
				{
					Name:     "aws-specific",
					Path:     "/tmp/aws-specific.tmpl",
					Priority: 2,
				},
			},
		}

		err = composer.ValidateComposition(composition)
		assert.Error(t, err)
		// The error will be caught at overlay compatibility validation (aws-specific vs openstack-base)
		assert.Contains(t, err.Error(), "incompatible cloud providers")
		assert.Contains(t, err.Error(), "Resolution Options:")
		assert.Contains(t, err.Error(), "Impact:")
	})
}

func TestTemplateComposition_DependencyValidation(t *testing.T) {
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register base template
	err := registry.RegisterTemplate(TemplateDefinition{
		Name: "base",
		Path: "/tmp/base.tmpl",
		Type: TemplateTypeBase,
	})
	require.NoError(t, err)

	// Register overlay with dependencies
	err = registry.RegisterTemplate(TemplateDefinition{
		Name:         "overlay-with-deps",
		Path:         "/tmp/overlay-deps.tmpl",
		Type:         TemplateTypeOverlay,
		Dependencies: []string{"base", "other-template"},
	})
	require.NoError(t, err)

	composer := NewDefaultTemplateComposer(engine, registry)

	t.Run("valid dependencies", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: "base",
			Overlays: []TemplateOverlay{
				{
					Name:     "overlay-with-deps",
					Path:     "/tmp/overlay-deps.tmpl",
					Priority: 1,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		// Should not error - dependencies are just validated for format, not existence
		assert.NoError(t, err)
	})

	t.Run("circular dependency via file path", func(t *testing.T) {
		// Create a temporary directory for test files
		tmpDir := t.TempDir()

		// Create a base template file
		baseFile := filepath.Join(tmpDir, "base.tmpl")
		err := os.WriteFile(baseFile, []byte("base content"), 0644)
		require.NoError(t, err)

		// Create an overlay file (we'll simulate circular dependency in validation)
		overlayFile := filepath.Join(tmpDir, "circular.tmpl")
		err = os.WriteFile(overlayFile, []byte("overlay content"), 0644)
		require.NoError(t, err)

		// For file-based templates, we can't easily test circular dependencies
		// since the registry validates them at registration time.
		// This test documents that circular dependencies are caught at registration.
		composition := TemplateComposition{
			BaseTemplate: baseFile,
			Overlays: []TemplateOverlay{
				{
					Name:     "circular-overlay-file",
					Path:     overlayFile,
					Priority: 1,
				},
			},
		}

		// This should pass because file-based templates don't have dependency metadata
		err = composer.ValidateComposition(composition)
		assert.NoError(t, err)
	})

	t.Run("registry prevents circular dependency at registration", func(t *testing.T) {
		// Attempt to register a template with circular dependency
		err := registry.RegisterTemplate(TemplateDefinition{
			Name:         "circular-overlay",
			Path:         "/tmp/circular.tmpl",
			Type:         TemplateTypeOverlay,
			Dependencies: []string{"circular-overlay"},
		})

		// Registry should reject this at registration time
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot depend on itself")
	})

	t.Run("registry prevents empty dependency at registration", func(t *testing.T) {
		// Attempt to register a template with empty dependency
		err := registry.RegisterTemplate(TemplateDefinition{
			Name:         "empty-dep-overlay",
			Path:         "/tmp/empty-dep.tmpl",
			Type:         TemplateTypeOverlay,
			Dependencies: []string{""},
		})

		// Registry should reject this at registration time
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dependency name cannot be empty")
	})
}

func TestTemplateComposition_ServiceCompatibility(t *testing.T) {
	engine := NewGoTemplateEngine()
	registry := NewInMemoryTemplateRegistry()

	// Register base template with services
	err := registry.RegisterTemplate(TemplateDefinition{
		Name:     "base-with-services",
		Path:     "/tmp/base-services.tmpl",
		Type:     TemplateTypeBase,
		Services: []string{"prometheus", "grafana"},
	})
	require.NoError(t, err)

	// Register overlay with compatible services
	err = registry.RegisterTemplate(TemplateDefinition{
		Name:     "monitoring-overlay",
		Path:     "/tmp/monitoring-overlay.tmpl",
		Type:     TemplateTypeOverlay,
		Services: []string{"prometheus"},
	})
	require.NoError(t, err)

	composer := NewDefaultTemplateComposer(engine, registry)

	t.Run("compatible services", func(t *testing.T) {
		composition := TemplateComposition{
			BaseTemplate: "base-with-services",
			Overlays: []TemplateOverlay{
				{
					Name:     "monitoring-overlay",
					Path:     "/tmp/monitoring-overlay.tmpl",
					Priority: 1,
				},
			},
		}

		err := composer.ValidateComposition(composition)
		assert.NoError(t, err)
	})
}
