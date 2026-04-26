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

package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInfrastructureStageBasicExecution tests basic infrastructure stage execution
func TestInfrastructureStageBasicExecution(t *testing.T) {
	// Create test configuration
	cfg := createTestConfigInfra("openstack")

	// Create temporary workspace
	tempDir := t.TempDir()
	workspace := createTestWorkspace(t, tempDir, cfg)

	// Create template engine and registry
	engine := template.NewGoTemplateEngine()
	registry := template.NewInMemoryTemplateRegistry()

	// Create test template file first
	templatePath := filepath.Join(tempDir, "test-template.yaml")
	templateContent := `# Infrastructure for {{ .OpenCenter.Cluster.ClusterName }}
provider: {{ .OpenCenter.Infrastructure.Provider }}
organization: {{ .OpenCenter.Meta.Organization }}
`
	err := os.WriteFile(templatePath, []byte(templateContent), 0o644)
	require.NoError(t, err)

	// Register a simple infrastructure template
	err = registry.RegisterTemplate(template.TemplateDefinition{
		Name:     "cluster-config",
		Path:     templatePath, // Use absolute path
		Type:     template.TemplateTypeInfrastructure,
		Provider: "openstack",
		Metadata: template.TemplateMetadata{
			Description: "Test infrastructure template",
		},
	})
	require.NoError(t, err)

	// Create init stage directories first
	initStage := NewInitStage()
	err = initStage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Create and execute infrastructure stage
	stage := NewInfrastructureStage(engine, registry)
	err = stage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Verify output file was created
	outputPath := filepath.Join(tempDir, "infrastructure", "clusters", "test-cluster", "cluster-config.yaml")
	assert.FileExists(t, outputPath)

	// Verify content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "provider: openstack")
	assert.Contains(t, string(content), "organization: test-org")
}

// TestInfrastructureStageProviderFiltering tests that only provider-specific templates are rendered
func TestInfrastructureStageProviderFiltering(t *testing.T) {
	tests := []struct {
		name                string
		provider            string
		expectedTemplates   []string
		unexpectedTemplates []string
	}{
		{
			name:                "OpenStack provider",
			provider:            "openstack",
			expectedTemplates:   []string{"openstack-config"},
			unexpectedTemplates: []string{"aws-config"},
		},
		{
			name:                "AWS provider",
			provider:            "aws",
			expectedTemplates:   []string{"aws-config"},
			unexpectedTemplates: []string{"openstack-config"},
		},
		{
			name:                "Baremetal provider",
			provider:            "baremetal",
			expectedTemplates:   []string{"baremetal-config"},
			unexpectedTemplates: []string{"openstack-config", "aws-config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := createTestConfigInfra(tt.provider)

			// Create template engine and registry
			engine := template.NewGoTemplateEngine()
			registry := template.NewInMemoryTemplateRegistry()

			// Create temporary workspace first
			tempDir := t.TempDir()
			workspace := createTestWorkspace(t, tempDir, cfg)

			// Create template files for all providers first
			providers := []string{"openstack", "aws", "baremetal"}
			for _, provider := range providers {
				templatePath := filepath.Join(tempDir, provider+"-template.yaml")
				templateContent := `provider: ` + provider + "\n"
				err := os.WriteFile(templatePath, []byte(templateContent), 0o644)
				require.NoError(t, err)
			}

			// Register templates for different providers with absolute paths
			for _, provider := range providers {
				templatePath := filepath.Join(tempDir, provider+"-template.yaml")
				err := registry.RegisterTemplate(template.TemplateDefinition{
					Name:     provider + "-config",
					Path:     templatePath, // Use absolute path
					Type:     template.TemplateTypeInfrastructure,
					Provider: provider,
				})
				require.NoError(t, err)
			}

			// Create init stage directories
			initStage := NewInitStage()
			err := initStage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Execute infrastructure stage
			stage := NewInfrastructureStage(engine, registry)
			err = stage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Verify expected templates were created
			for _, tmplName := range tt.expectedTemplates {
				outputPath := filepath.Join(tempDir, "infrastructure", "clusters", "test-cluster", tmplName+".yaml")
				assert.FileExists(t, outputPath, "Expected template %s should exist", tmplName)
			}

			// Verify unexpected templates were NOT created
			for _, tmplName := range tt.unexpectedTemplates {
				outputPath := filepath.Join(tempDir, "infrastructure", "clusters", "test-cluster", tmplName+".yaml")
				assert.NoFileExists(t, outputPath, "Unexpected template %s should not exist", tmplName)
			}
		})
	}
}

// TestInfrastructureStageTemplateDependencies tests template dependency resolution
func TestInfrastructureStageTemplateDependencies(t *testing.T) {
	// Create test configuration
	cfg := createTestConfigInfra("openstack")

	// Create template engine and registry
	engine := template.NewGoTemplateEngine()
	registry := template.NewInMemoryTemplateRegistry()

	// Create temporary workspace first
	tempDir := t.TempDir()
	workspace := createTestWorkspace(t, tempDir, cfg)

	// Create template files first
	baseTemplatePath := filepath.Join(tempDir, "base-template.yaml")
	err := os.WriteFile(baseTemplatePath, []byte("base: config\n"), 0o644)
	require.NoError(t, err)

	networkTemplatePath := filepath.Join(tempDir, "network-template.yaml")
	err = os.WriteFile(networkTemplatePath, []byte("network: config\n"), 0o644)
	require.NoError(t, err)

	// Register templates with dependencies using absolute paths
	err = registry.RegisterTemplate(template.TemplateDefinition{
		Name:         "base-config",
		Path:         baseTemplatePath, // Use absolute path
		Type:         template.TemplateTypeInfrastructure,
		Provider:     "openstack",
		Dependencies: []string{},
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(template.TemplateDefinition{
		Name:         "network-config",
		Path:         networkTemplatePath, // Use absolute path
		Type:         template.TemplateTypeInfrastructure,
		Provider:     "openstack",
		Dependencies: []string{"base-config"},
	})
	require.NoError(t, err)

	// Create init stage directories
	initStage := NewInitStage()
	err = initStage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Execute infrastructure stage
	stage := NewInfrastructureStage(engine, registry)
	err = stage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Verify both templates were created
	baseOutput := filepath.Join(tempDir, "infrastructure", "clusters", "test-cluster", "base-config.yaml")
	assert.FileExists(t, baseOutput)

	networkOutput := filepath.Join(tempDir, "infrastructure", "clusters", "test-cluster", "network-config.yaml")
	assert.FileExists(t, networkOutput)
}

// TestInfrastructureStageRollback tests rollback functionality
func TestInfrastructureStageRollback(t *testing.T) {
	// Create test configuration
	cfg := createTestConfigInfra("openstack")

	// Create template engine and registry
	engine := template.NewGoTemplateEngine()
	registry := template.NewInMemoryTemplateRegistry()

	// Create temporary workspace first
	tempDir := t.TempDir()
	workspace := createTestWorkspace(t, tempDir, cfg)

	// Create template file first
	templatePath := filepath.Join(tempDir, "test-template.yaml")
	err := os.WriteFile(templatePath, []byte("test: config\n"), 0o644)
	require.NoError(t, err)

	// Register template with absolute path
	err = registry.RegisterTemplate(template.TemplateDefinition{
		Name:     "test-config",
		Path:     templatePath, // Use absolute path
		Type:     template.TemplateTypeInfrastructure,
		Provider: "openstack",
	})
	require.NoError(t, err)

	// Create init stage directories
	initStage := NewInitStage()
	err = initStage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Execute infrastructure stage
	stage := NewInfrastructureStage(engine, registry)
	err = stage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Verify file was created
	outputPath := filepath.Join(tempDir, "infrastructure", "clusters", "test-cluster", "test-config.yaml")
	assert.FileExists(t, outputPath)

	// Rollback
	err = stage.Rollback(context.Background(), workspace)
	require.NoError(t, err)

	// Verify file was removed
	assert.NoFileExists(t, outputPath)
}

// TestInfrastructureStageValidation tests validation functionality
func TestInfrastructureStageValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupStage  bool
		expectError bool
	}{
		{
			name:        "Valid infrastructure",
			setupStage:  true,
			expectError: false,
		},
		{
			name:        "Missing infrastructure files",
			setupStage:  false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := createTestConfigInfra("openstack")

			// Create template engine and registry
			engine := template.NewGoTemplateEngine()
			registry := template.NewInMemoryTemplateRegistry()

			// Create temporary workspace first
			tempDir := t.TempDir()
			workspace := createTestWorkspace(t, tempDir, cfg)

			// Create template file first
			templatePath := filepath.Join(tempDir, "test-template.yaml")
			err := os.WriteFile(templatePath, []byte("test: config\n"), 0o644)
			require.NoError(t, err)

			// Register template with absolute path
			err = registry.RegisterTemplate(template.TemplateDefinition{
				Name:     "test-config",
				Path:     templatePath, // Use absolute path
				Type:     template.TemplateTypeInfrastructure,
				Provider: "openstack",
			})
			require.NoError(t, err)

			// Create init stage directories
			initStage := NewInitStage()
			err = initStage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			if tt.setupStage {
				// Execute infrastructure stage
				stage := NewInfrastructureStage(engine, registry)
				err = stage.Execute(context.Background(), workspace)
				require.NoError(t, err)
			}

			// Validate
			stage := NewInfrastructureStage(engine, registry)
			err = stage.Validate(context.Background(), workspace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestInfrastructureStageDryRun tests dry-run functionality
func TestInfrastructureStageDryRun(t *testing.T) {
	// Create test configuration
	cfg := createTestConfigInfra("openstack")

	// Create template engine and registry
	engine := template.NewGoTemplateEngine()
	registry := template.NewInMemoryTemplateRegistry()

	// Register templates
	err := registry.RegisterTemplate(template.TemplateDefinition{
		Name:     "config1",
		Path:     "template1.yaml",
		Type:     template.TemplateTypeInfrastructure,
		Provider: "openstack",
	})
	require.NoError(t, err)

	err = registry.RegisterTemplate(template.TemplateDefinition{
		Name:     "config2",
		Path:     "template2.yaml",
		Type:     template.TemplateTypeInfrastructure,
		Provider: "openstack",
	})
	require.NoError(t, err)

	// Create infrastructure stage
	stage := NewInfrastructureStage(engine, registry)

	// Execute dry-run
	plan, err := stage.DryRun(context.Background(), cfg)
	require.NoError(t, err)

	// Verify plan
	assert.Equal(t, "infrastructure", plan.Name)
	assert.Contains(t, plan.Description, "openstack")
	assert.Len(t, plan.Files, 2)
	assert.Contains(t, plan.Files, "infrastructure/clusters/test-cluster/config1.yaml")
	assert.Contains(t, plan.Files, "infrastructure/clusters/test-cluster/config2.yaml")
}

// TestInfrastructureStageConditions tests template condition evaluation
func TestInfrastructureStageConditions(t *testing.T) {
	tests := []struct {
		name         string
		condition    template.RenderCondition
		provider     string
		shouldRender bool
	}{
		{
			name: "Equals condition matches",
			condition: template.RenderCondition{
				Type:  template.ConditionTypeEquals,
				Field: "provider",
				Value: "openstack",
			},
			provider:     "openstack",
			shouldRender: true,
		},
		{
			name: "Equals condition doesn't match",
			condition: template.RenderCondition{
				Type:  template.ConditionTypeEquals,
				Field: "provider",
				Value: "aws",
			},
			provider:     "openstack",
			shouldRender: false,
		},
		{
			name: "NotEquals condition matches",
			condition: template.RenderCondition{
				Type:  template.ConditionTypeNotEquals,
				Field: "provider",
				Value: "aws",
			},
			provider:     "openstack",
			shouldRender: true,
		},
		{
			name: "Exists condition",
			condition: template.RenderCondition{
				Type:  template.ConditionTypeExists,
				Field: "provider",
			},
			provider:     "openstack",
			shouldRender: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := createTestConfigInfra(tt.provider)

			// Create template engine and registry
			engine := template.NewGoTemplateEngine()
			registry := template.NewInMemoryTemplateRegistry()

			// Create temporary workspace first
			tempDir := t.TempDir()
			workspace := createTestWorkspace(t, tempDir, cfg)

			// Create template file first
			templatePath := filepath.Join(tempDir, "conditional-template.yaml")
			err := os.WriteFile(templatePath, []byte("conditional: config\n"), 0o644)
			require.NoError(t, err)

			// Register template with condition using absolute path
			err = registry.RegisterTemplate(template.TemplateDefinition{
				Name:       "conditional-config",
				Path:       templatePath, // Use absolute path
				Type:       template.TemplateTypeInfrastructure,
				Provider:   tt.provider,
				Conditions: []template.RenderCondition{tt.condition},
			})
			require.NoError(t, err)

			// Create init stage directories
			initStage := NewInitStage()
			err = initStage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Execute infrastructure stage
			stage := NewInfrastructureStage(engine, registry)
			err = stage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Check if file was created based on condition
			outputPath := filepath.Join(tempDir, "infrastructure", "clusters", "test-cluster", "conditional-config.yaml")
			if tt.shouldRender {
				assert.FileExists(t, outputPath)
			} else {
				assert.NoFileExists(t, outputPath)
			}
		})
	}
}

// Helper functions

func createTestConfigInfra(provider string) v2.Config {
	return v2.Config{
		SchemaVersion: "2.0",
		OpenCenter: v2.OpenCenterConfig{
			Meta: v2.MetaConfig{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: v2.InfrastructureConfig{
				Provider: provider,
			},
			Cluster: v2.ClusterConfig{
				ClusterName: "test-cluster",
			},
		},
	}
}

func createTestWorkspace(t *testing.T, rootDir string, cfg v2.Config) *gitops.GitOpsWorkspace {
	workspace := &gitops.GitOpsWorkspace{
		RootDir: rootDir,
		TempDir: filepath.Join(rootDir, ".tmp"),
		Config:  cfg,
	}

	// Create temp directory
	err := os.MkdirAll(workspace.TempDir, 0o755)
	require.NoError(t, err)

	return workspace
}
