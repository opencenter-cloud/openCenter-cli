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
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackwardCompatibility_ExistingCallsWorkWithoutModification validates
// the acceptance criterion: "Existing template calls work without modification"
//
// This test demonstrates that all existing template function calls continue
// to work exactly as they did before the refactor, producing identical output.
func TestBackwardCompatibility_ExistingCallsWorkWithoutModification(t *testing.T) {
	// Create test filesystem with various template types
	fsys := fstest.MapFS{
		"simple.yaml.tmpl": &fstest.MapFile{
			Data: []byte("name: {{.Name}}\nvalue: {{.Value}}"),
		},
		"with-sprig.yaml.tmpl": &fstest.MapFile{
			Data: []byte("name: {{.Name | upper}}\nvalue: {{.Value | quote}}"),
		},
		"complex.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: {{.Name}}
  namespace: {{.Namespace | default "default"}}
data:
  key: {{.Value}}`),
		},
	}

	tmpDir := t.TempDir()
	data := map[string]string{
		"Name":      "test-config",
		"Value":     "test-value",
		"Namespace": "production",
	}

	tests := []struct {
		name         string
		templatePath string
		outputFile   string
		expectOutput string
	}{
		{
			name:         "simple template",
			templatePath: "simple.yaml.tmpl",
			outputFile:   "simple.yaml",
			expectOutput: "name: test-config\nvalue: test-value",
		},
		{
			name:         "template with sprig functions",
			templatePath: "with-sprig.yaml.tmpl",
			outputFile:   "with-sprig.yaml",
			expectOutput: "name: TEST-CONFIG\nvalue: \"test-value\"",
		},
		{
			name:         "complex kubernetes manifest",
			templatePath: "complex.yaml.tmpl",
			outputFile:   "complex.yaml",
			expectOutput: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test-config\n  namespace: production\ndata:\n  key: test-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := filepath.Join(tmpDir, tt.outputFile)

			// Call the existing RenderTemplateToFile function WITHOUT any modifications
			// This is the same function signature that existing code uses
			err := RenderTemplateToFile(fsys, tt.templatePath, outputPath, data)
			require.NoError(t, err, "existing template call should work without modification")

			// Verify output is correct
			content, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expectOutput, string(content))
		})
	}
}

// TestBackwardCompatibility_RenderTemplateToWriter validates that the
// RenderTemplateToWriter function continues to work without modification.
func TestBackwardCompatibility_RenderTemplateToWriter(t *testing.T) {
	fsys := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{
			Data: []byte("Hello, {{.Name}}!"),
		},
	}

	data := map[string]string{"Name": "World"}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.txt")

	// Create file and use it as writer
	f, err := os.Create(outputPath)
	require.NoError(t, err)
	defer f.Close()

	// Call existing function WITHOUT modification
	err = RenderTemplateToWriter(fsys, "test.tmpl", data, f)
	require.NoError(t, err, "existing RenderTemplateToWriter call should work without modification")

	// Close file to flush
	f.Close()

	// Verify output
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(content))
}

// TestBackwardCompatibility_CopyFileFromFS validates that the
// CopyFileFromFS function continues to work without modification.
func TestBackwardCompatibility_CopyFileFromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"test.txt": &fstest.MapFile{
			Data: []byte("test content"),
		},
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.txt")

	// Call existing function WITHOUT modification
	err := CopyFileFromFS(fsys, "test.txt", outputPath)
	require.NoError(t, err, "existing CopyFileFromFS call should work without modification")

	// Verify output
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

// TestBackwardCompatibility_RenderTemplateString validates that the
// RenderTemplateString function continues to work without modification.
func TestBackwardCompatibility_RenderTemplateString(t *testing.T) {
	templateContent := "Value: {{.Value | upper}}"
	data := map[string]string{"Value": "hello"}

	// Call existing function WITHOUT modification
	result, err := RenderTemplateString("test", templateContent, data)
	require.NoError(t, err, "existing RenderTemplateString call should work without modification")

	assert.Equal(t, "Value: HELLO", result)
}
