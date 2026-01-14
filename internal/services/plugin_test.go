package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		validate    func(*testing.T, *ServicePluginManifest)
	}{
		{
			name: "valid manifest",
			content: `name: test-service
version: 1.0.0
type: monitoring
description: Test service
dependencies:
  - core-service
templates:
  - name: deployment
    path: templates/deployment.yaml
config:
  schema:
    enabled:
      type: boolean
  defaults:
    enabled: true
  required:
    - enabled
metadata:
  author: Test Author
`,
			expectError: false,
			validate: func(t *testing.T, m *ServicePluginManifest) {
				assert.Equal(t, "test-service", m.Name)
				assert.Equal(t, "1.0.0", m.Version)
				assert.Equal(t, ServiceTypeMonitoring, m.Type)
				assert.Equal(t, "Test service", m.Description)
				assert.Len(t, m.Dependencies, 1)
				assert.Equal(t, "core-service", m.Dependencies[0])
				assert.Len(t, m.Templates, 1)
				assert.Equal(t, "deployment", m.Templates[0].Name)
			},
		},
		{
			name: "minimal manifest",
			content: `name: minimal-service
version: 1.0.0
`,
			expectError: false,
			validate: func(t *testing.T, m *ServicePluginManifest) {
				assert.Equal(t, "minimal-service", m.Name)
				assert.Equal(t, "1.0.0", m.Version)
				assert.Equal(t, ServiceTypeCustom, m.Type) // Default type
			},
		},
		{
			name: "missing name",
			content: `version: 1.0.0
type: monitoring
`,
			expectError: true,
		},
		{
			name: "missing version",
			content: `name: test-service
type: monitoring
`,
			expectError: true,
		},
		{
			name:        "invalid yaml",
			content:     `invalid: yaml: content:`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			manifestPath := filepath.Join(tmpDir, "manifest.yaml")
			err := os.WriteFile(manifestPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			// Load manifest
			manifest, err := LoadManifest(manifestPath)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, manifest)

			if tt.validate != nil {
				tt.validate(t, manifest)
			}
		})
	}
}

func TestLoadManifestsFromDirectory(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		expectError bool
		expectCount int
	}{
		{
			name: "multiple valid manifests",
			files: map[string]string{
				"service1.yaml": `name: service1
version: 1.0.0
type: monitoring
`,
				"service2.yaml": `name: service2
version: 2.0.0
type: logging
`,
			},
			expectError: false,
			expectCount: 2,
		},
		{
			name: "mixed valid and non-yaml files",
			files: map[string]string{
				"service1.yaml": `name: service1
version: 1.0.0
`,
				"readme.txt": "This is not a manifest",
				"service2.yml": `name: service2
version: 2.0.0
`,
			},
			expectError: false,
			expectCount: 2,
		},
		{
			name: "invalid manifest in directory",
			files: map[string]string{
				"service1.yaml": `name: service1
version: 1.0.0
`,
				"invalid.yaml": `name: invalid
# missing version
`,
			},
			expectError: true,
		},
		{
			name:        "empty directory",
			files:       map[string]string{},
			expectError: false,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory with files
			tmpDir := t.TempDir()
			for filename, content := range tt.files {
				path := filepath.Join(tmpDir, filename)
				err := os.WriteFile(path, []byte(content), 0644)
				require.NoError(t, err)
			}

			// Load manifests
			manifests, err := LoadManifestsFromDirectory(tmpDir)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, manifests, tt.expectCount)
		})
	}
}

func TestLoadManifestsFromDirectory_NonExistentDirectory(t *testing.T) {
	manifests, err := LoadManifestsFromDirectory("/nonexistent/directory")
	require.NoError(t, err)
	assert.Nil(t, manifests)
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name        string
		manifest    *ServicePluginManifest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid manifest",
			manifest: &ServicePluginManifest{
				Name:    "test-service",
				Version: "1.0.0",
				Type:    ServiceTypeMonitoring,
			},
			expectError: false,
		},
		{
			name:        "nil manifest",
			manifest:    nil,
			expectError: true,
			errorMsg:    "manifest is nil",
		},
		{
			name: "missing name",
			manifest: &ServicePluginManifest{
				Version: "1.0.0",
				Type:    ServiceTypeMonitoring,
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "missing version",
			manifest: &ServicePluginManifest{
				Name: "test-service",
				Type: ServiceTypeMonitoring,
			},
			expectError: true,
			errorMsg:    "version is required",
		},
		{
			name: "invalid service type",
			manifest: &ServicePluginManifest{
				Name:    "test-service",
				Version: "1.0.0",
				Type:    ServiceType("invalid"),
			},
			expectError: true,
			errorMsg:    "invalid service type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateManifest(tt.manifest)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServiceTypes(t *testing.T) {
	validTypes := []ServiceType{
		ServiceTypeCore,
		ServiceTypeMonitoring,
		ServiceTypeLogging,
		ServiceTypeStorage,
		ServiceTypeNetworking,
		ServiceTypeSecurity,
		ServiceTypeGitOps,
		ServiceTypeCustom,
	}

	for _, serviceType := range validTypes {
		t.Run(string(serviceType), func(t *testing.T) {
			manifest := &ServicePluginManifest{
				Name:    "test-service",
				Version: "1.0.0",
				Type:    serviceType,
			}

			err := ValidateManifest(manifest)
			assert.NoError(t, err)
		})
	}
}
