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
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedTemplateRegistrar_RegisterFromFS(t *testing.T) {
	tests := []struct {
		name        string
		fsys        fstest.MapFS
		basePath    string
		opts        RegistrationOptions
		expectCount int
		expectError bool
	}{
		{
			name: "register simple templates",
			fsys: fstest.MapFS{
				"templates/test1.tpl": &fstest.MapFile{
					Data: []byte("template content"),
				},
				"templates/test2.tmpl": &fstest.MapFile{
					Data: []byte("template content"),
				},
			},
			basePath: "templates",
			opts: RegistrationOptions{
				Type:     TemplateTypeBase,
				Provider: "openstack",
			},
			expectCount: 2,
			expectError: false,
		},
		{
			name: "skip non-template files",
			fsys: fstest.MapFS{
				"templates/test.tpl": &fstest.MapFile{
					Data: []byte("template content"),
				},
				"templates/readme.md": &fstest.MapFile{
					Data: []byte("documentation"),
				},
				"templates/script.sh": &fstest.MapFile{
					Data: []byte("#!/bin/bash"),
				},
			},
			basePath:    "templates",
			opts:        RegistrationOptions{},
			expectCount: 1,
			expectError: false,
		},
		{
			name: "register nested templates",
			fsys: fstest.MapFS{
				"templates/base/config.yaml": &fstest.MapFile{
					Data: []byte("config"),
				},
				"templates/services/loki/values.yaml": &fstest.MapFile{
					Data: []byte("values"),
				},
			},
			basePath:    "templates",
			opts:        RegistrationOptions{},
			expectCount: 2,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewInMemoryTemplateRegistry()
			registrar := NewEmbeddedTemplateRegistrar(registry)

			err := registrar.RegisterFromFS(tt.fsys, tt.basePath, tt.opts)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				templates := registry.ListTemplates()
				assert.Len(t, templates, tt.expectCount)
			}
		})
	}
}

func TestIsTemplateFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"template.tpl", true},
		{"template.tmpl", true},
		{"config.yaml", true},
		{"config.yml", true},
		{"readme.md", false},
		{"script.sh", false},
		{"main.go", false},
		{"Makefile", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isTemplateFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateTemplateName(t *testing.T) {
	tests := []struct {
		path     string
		basePath string
		expected string
	}{
		{
			path:     "templates/config.yaml",
			basePath: "templates",
			expected: "config",
		},
		{
			path:     "templates/services/loki/values.yaml",
			basePath: "templates",
			expected: "services.loki.values",
		},
		{
			path:     "infrastructure/main.tf.tpl",
			basePath: "infrastructure",
			expected: "main.tf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := generateTemplateName(tt.path, tt.basePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferTemplateType(t *testing.T) {
	tests := []struct {
		path     string
		expected TemplateType
	}{
		{"templates/infrastructure/main.tf", TemplateTypeInfrastructure},
		{"templates/services/loki/values.yaml", TemplateTypeService},
		{"templates/managed-services/prometheus/config.yaml", TemplateTypeService},
		{"templates/overlay/patch.yaml", TemplateTypeOverlay},
		{"templates/base/config.yaml", TemplateTypeBase},
		{"templates/unknown/file.yaml", TemplateTypeBase},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := inferTemplateType(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferProvider(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"templates/main-baremetal.tf.tpl", "baremetal"},
		{"templates/openstack-config.yaml", "openstack"},
		{"templates/aws/main.tf", "aws"},
		{"templates/vsphere/config.yaml", "vsphere"},
		{"templates/kind/cluster.yaml", "kind"},
		{"templates/generic/config.yaml", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := inferProvider(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferServices(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{
			path:     "templates/services/loki/values.yaml",
			expected: []string{"loki"},
		},
		{
			path:     "templates/managed-services/alert-proxy/config.yaml",
			expected: []string{"alert-proxy"},
		},
		{
			path:     "templates/services/prometheus-stack/values.yaml",
			expected: []string{"prometheus"},
		},
		{
			path:     "templates/infrastructure/main.tf",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := inferServices(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegisterGitOpsTemplates(t *testing.T) {
	// Create a mock filesystem that simulates the gitops embedded structure
	fsys := fstest.MapFS{
		"templates/infrastructure-cluster-template/main.tf.tpl": &fstest.MapFile{
			Data: []byte("terraform config"),
		},
		"templates/infrastructure-cluster-template/variables.tf.tpl": &fstest.MapFile{
			Data: []byte("terraform variables"),
		},
		"templates/cluster-apps-base/services/loki/values.yaml": &fstest.MapFile{
			Data: []byte("loki values"),
		},
		"templates/cluster-apps-base/managed-services/alert-proxy/config.yaml": &fstest.MapFile{
			Data: []byte("alert-proxy config"),
		},
	}

	registry := NewInMemoryTemplateRegistry()
	err := RegisterGitOpsTemplates(registry, fsys)
	require.NoError(t, err)

	// Verify templates were registered
	templates := registry.ListTemplates()
	assert.Greater(t, len(templates), 0, "should have registered templates")

	// Verify infrastructure templates
	infraTemplates := registry.GetTemplatesForProvider("")
	assert.Greater(t, len(infraTemplates), 0, "should have infrastructure templates")

	// Verify service templates
	lokiTemplates := registry.GetTemplatesForService("loki")
	assert.Greater(t, len(lokiTemplates), 0, "should have loki templates")
}

func TestRegisterProvisionTemplates(t *testing.T) {
	// Create a mock filesystem that simulates the provision embedded structure
	fsys := fstest.MapFS{
		"templates/main.tf.tmpl": &fstest.MapFile{
			Data: []byte("terraform main"),
		},
		"templates/variables.tf.tmpl": &fstest.MapFile{
			Data: []byte("terraform variables"),
		},
		"templates/inventory.tmpl": &fstest.MapFile{
			Data: []byte("ansible inventory"),
		},
	}

	registry := NewInMemoryTemplateRegistry()
	err := RegisterProvisionTemplates(registry, fsys)
	require.NoError(t, err)

	// Verify templates were registered
	templates := registry.ListTemplates()
	assert.Len(t, templates, 3, "should have registered 3 templates")

	// Verify all are infrastructure type
	for _, tmpl := range templates {
		assert.Equal(t, TemplateTypeInfrastructure, tmpl.Type)
	}
}

func TestCreateTemplateDefinition(t *testing.T) {
	registry := NewInMemoryTemplateRegistry()
	registrar := NewEmbeddedTemplateRegistrar(registry)

	opts := RegistrationOptions{
		Provider:    "openstack",
		Services:    []string{"loki"},
		Type:        TemplateTypeService,
		Priority:    50,
		Description: "Test template",
		Version:     "1.0.0",
		Tags:        []string{"test"},
	}

	def := registrar.createTemplateDefinition(
		"templates/services/loki/values.yaml",
		"templates",
		opts,
	)

	assert.Equal(t, "services.loki.values", def.Name)
	assert.Equal(t, "templates/services/loki/values.yaml", def.Path)
	assert.Equal(t, TemplateTypeService, def.Type)
	assert.Equal(t, "openstack", def.Provider)
	assert.Equal(t, []string{"loki"}, def.Services)
	assert.Equal(t, 50, def.Metadata.Priority)
	assert.Equal(t, "Test template", def.Metadata.Description)
	assert.Equal(t, "1.0.0", def.Metadata.Version)
	assert.Equal(t, []string{"test"}, def.Metadata.Tags)
}

func TestRegistrationWithInference(t *testing.T) {
	// Test that when options are not provided, inference works correctly
	registry := NewInMemoryTemplateRegistry()
	registrar := NewEmbeddedTemplateRegistrar(registry)

	fsys := fstest.MapFS{
		"templates/infrastructure/main-baremetal.tf.tpl": &fstest.MapFile{
			Data: []byte("baremetal terraform"),
		},
		"templates/services/loki/values.yaml": &fstest.MapFile{
			Data: []byte("loki values"),
		},
	}

	// Register with minimal options - let inference do the work
	opts := RegistrationOptions{}
	err := registrar.RegisterFromFS(fsys, "templates", opts)
	require.NoError(t, err)

	templates := registry.ListTemplates()
	require.Len(t, templates, 2)

	// Find the baremetal template
	var baremetalTemplate TemplateDefinition
	for _, tmpl := range templates {
		if tmpl.Provider == "baremetal" {
			baremetalTemplate = tmpl
			break
		}
	}

	assert.NotEmpty(t, baremetalTemplate.Name)
	assert.Equal(t, "baremetal", baremetalTemplate.Provider)
	assert.Equal(t, TemplateTypeInfrastructure, baremetalTemplate.Type)

	// Find the loki template
	var lokiTemplate TemplateDefinition
	for _, tmpl := range templates {
		if len(tmpl.Services) > 0 && tmpl.Services[0] == "loki" {
			lokiTemplate = tmpl
			break
		}
	}

	assert.NotEmpty(t, lokiTemplate.Name)
	assert.Equal(t, TemplateTypeService, lokiTemplate.Type)
	assert.Contains(t, lokiTemplate.Services, "loki")
}

func TestRegisterGitOpsBaseTemplates(t *testing.T) {
	t.Run("with structure files only", func(t *testing.T) {
		// Create a mock filesystem that simulates the gitops-base-dir structure
		// Note: gitops-base-dir contains only structure files (.gitignore, .gitkeep)
		// which are not considered templates
		fsys := fstest.MapFS{
			"gitops-base-dir/.gitignore": &fstest.MapFile{
				Data: []byte("# gitignore content"),
			},
			"gitops-base-dir/applications/overlays/.gitkeep": &fstest.MapFile{
				Data: []byte(""),
			},
			"gitops-base-dir/infrastructure/clusters/.gitkeep": &fstest.MapFile{
				Data: []byte(""),
			},
		}

		registry := NewInMemoryTemplateRegistry()
		err := RegisterGitOpsBaseTemplates(registry, fsys)
		require.NoError(t, err)

		// Since gitops-base-dir only contains structure files (not templates),
		// no templates should be registered
		templates := registry.ListTemplates()
		assert.Equal(t, 0, len(templates), "structure files should not be registered as templates")
	})

	t.Run("with actual template files", func(t *testing.T) {
		// Create a mock filesystem with actual template files
		fsys := fstest.MapFS{
			"gitops-base-dir/config.yaml": &fstest.MapFile{
				Data: []byte("# config template"),
			},
			"gitops-base-dir/base/kustomization.yaml": &fstest.MapFile{
				Data: []byte("# kustomization"),
			},
		}

		registry := NewInMemoryTemplateRegistry()
		err := RegisterGitOpsBaseTemplates(registry, fsys)
		require.NoError(t, err)

		// Verify templates were registered
		templates := registry.ListTemplates()
		assert.Equal(t, 2, len(templates), "should have registered template files")

		// Verify all are base type
		baseTemplates := registry.GetTemplatesForType(TemplateTypeBase)
		assert.Equal(t, 2, len(baseTemplates), "should have base type templates")

		// Verify high priority
		for _, tmpl := range baseTemplates {
			assert.Equal(t, 200, tmpl.Metadata.Priority, "base templates should have priority 200")
		}
	})
}
