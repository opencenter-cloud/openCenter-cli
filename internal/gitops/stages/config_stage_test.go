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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	tmpl "github.com/opencenter-cloud/opencenter-cli/internal/template"
)

// mockTemplateEngine is a simple mock for testing
type mockTemplateEngine struct{}

func (m *mockTemplateEngine) Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error) {
	return []byte("mock rendered content"), nil
}

func (m *mockTemplateEngine) RenderString(ctx context.Context, templateName, templateContent string, data interface{}) ([]byte, error) {
	return []byte("mock rendered content"), nil
}

func (m *mockTemplateEngine) RenderToWriter(ctx context.Context, templatePath string, data interface{}, w io.Writer) error {
	return nil
}

func (m *mockTemplateEngine) ValidateTemplate(templatePath string) error {
	return nil
}

func (m *mockTemplateEngine) RegisterFunction(name string, fn interface{}) {}

func (m *mockTemplateEngine) RegisterFunctions(funcs template.FuncMap) {}

func (m *mockTemplateEngine) SetCacheEnabled(enabled bool) {}

func (m *mockTemplateEngine) ClearCache() {}

func (m *mockTemplateEngine) LoadFromFS(fsys fs.FS, pattern string) error {
	return nil
}

func (m *mockTemplateEngine) LoadFromFile(path string) error {
	return nil
}

func (m *mockTemplateEngine) ExecuteTemplate(name string, data interface{}) ([]byte, error) {
	return []byte("mock rendered content"), nil
}

func (m *mockTemplateEngine) ExecuteTemplateToWriter(name string, data interface{}, w io.Writer) error {
	return nil
}

func (m *mockTemplateEngine) GetTemplate(name string) (*template.Template, error) {
	return nil, nil
}

// mockTemplateRegistry is a simple mock for testing
type mockTemplateRegistry struct {
	templates map[string]tmpl.TemplateDefinition
}

func newMockTemplateRegistry() *mockTemplateRegistry {
	return &mockTemplateRegistry{
		templates: make(map[string]tmpl.TemplateDefinition),
	}
}

func (m *mockTemplateRegistry) RegisterTemplate(t tmpl.TemplateDefinition) error {
	m.templates[t.Name] = t
	return nil
}

func (m *mockTemplateRegistry) GetTemplate(name string) (tmpl.TemplateDefinition, error) {
	if t, ok := m.templates[name]; ok {
		return t, nil
	}
	return tmpl.TemplateDefinition{}, nil
}

func (m *mockTemplateRegistry) GetTemplatesForProvider(provider string) []tmpl.TemplateDefinition {
	return []tmpl.TemplateDefinition{}
}

func (m *mockTemplateRegistry) GetTemplatesForService(service string) []tmpl.TemplateDefinition {
	return []tmpl.TemplateDefinition{}
}

func (m *mockTemplateRegistry) GetTemplatesForEnabledServices(enabledServices []string) []tmpl.TemplateDefinition {
	return []tmpl.TemplateDefinition{}
}

func (m *mockTemplateRegistry) GetTemplatesForType(templateType tmpl.TemplateType) []tmpl.TemplateDefinition {
	result := []tmpl.TemplateDefinition{}
	for _, t := range m.templates {
		if t.Type == templateType {
			result = append(result, t)
		}
	}
	return result
}

func (m *mockTemplateRegistry) ResolveTemplateDependencies(templates []string) ([]tmpl.TemplateDefinition, error) {
	result := []tmpl.TemplateDefinition{}
	for _, name := range templates {
		if t, ok := m.templates[name]; ok {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTemplateRegistry) ListTemplates() []tmpl.TemplateDefinition {
	result := []tmpl.TemplateDefinition{}
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result
}

func (m *mockTemplateRegistry) UnregisterTemplate(name string) error {
	delete(m.templates, name)
	return nil
}

func TestConfigStage_Execute(t *testing.T) {
	tests := []struct {
		name          string
		config        v2.Config
		templates     []tmpl.TemplateDefinition
		wantErr       bool
		expectedFiles []string
	}{
		{
			name: "creates default configuration files",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
					Cluster: config.ClusterConfig{
						ClusterName: "test-cluster",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
			},
			templates: []tmpl.TemplateDefinition{},
			wantErr:   false,
			expectedFiles: []string{
				"infrastructure/kustomization.yaml",
				"infrastructure/clusters/test-cluster/kustomization.yaml",
				"applications/kustomization.yaml",
				"applications/overlays/test-cluster/kustomization.yaml",
				".flux-system/kustomization.yaml",
			},
		},
		{
			name: "renders configuration templates",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
					Cluster: config.ClusterConfig{
						ClusterName: "test-cluster",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
			},
			templates: []tmpl.TemplateDefinition{
				{
					Name: "cluster-config.yaml",
					Path: "testdata/cluster-config.yaml.tmpl",
					Type: tmpl.TemplateTypeConfig,
					Metadata: tmpl.TemplateMetadata{
						Tags: []string{"infrastructure/clusters/test-cluster/cluster-config.yaml"},
					},
				},
			},
			wantErr: false,
			expectedFiles: []string{
				"infrastructure/kustomization.yaml",
				"infrastructure/clusters/test-cluster/kustomization.yaml",
				"infrastructure/clusters/test-cluster/cluster-config.yaml",
				"applications/kustomization.yaml",
				"applications/overlays/test-cluster/kustomization.yaml",
				".flux-system/kustomization.yaml",
			},
		},
		{
			name: "fails when cluster name is missing",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
			},
			templates: []tmpl.TemplateDefinition{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary workspace
			tempDir := t.TempDir()
			workspace := &gitops.GitOpsWorkspace{
				ID:          "test-workspace",
				RootDir:     tempDir,
				TempDir:     filepath.Join(tempDir, ".tmp"),
				Config:      tt.config,
				Metadata:    make(map[string]interface{}),
				Checkpoints: make(map[string]gitops.WorkspaceCheckpoint),
			}

			// Create temp directory
			if err := os.MkdirAll(workspace.TempDir, 0o755); err != nil {
				t.Fatalf("failed to create temp directory: %v", err)
			}

			// Create mock template engine and registry
			engine := &mockTemplateEngine{}
			registry := newMockTemplateRegistry()

			// Register templates
			for _, tmpl := range tt.templates {
				registry.RegisterTemplate(tmpl)
			}

			// Create config stage
			stage := NewConfigStage(engine, registry)

			// Execute stage
			err := stage.Execute(context.Background(), workspace)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigStage.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify expected files were created
			for _, expectedFile := range tt.expectedFiles {
				fullPath := filepath.Join(tempDir, expectedFile)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					t.Errorf("expected file not created: %s", expectedFile)
				}
			}
		})
	}
}

func TestConfigStage_Rollback(t *testing.T) {
	// Create temporary workspace
	tempDir := t.TempDir()
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Organization: "test-org",
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
		},
	}

	workspace := &gitops.GitOpsWorkspace{
		ID:          "test-workspace",
		RootDir:     tempDir,
		TempDir:     filepath.Join(tempDir, ".tmp"),
		Config:      cfg,
		Metadata:    make(map[string]interface{}),
		Checkpoints: make(map[string]gitops.WorkspaceCheckpoint),
	}

	// Create temp directory
	if err := os.MkdirAll(workspace.TempDir, 0o755); err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	// Create mock template engine and registry
	engine := &mockTemplateEngine{}
	registry := newMockTemplateRegistry()

	// Create config stage
	stage := NewConfigStage(engine, registry)

	// Execute stage to create files
	if err := stage.Execute(context.Background(), workspace); err != nil {
		t.Fatalf("failed to execute stage: %v", err)
	}

	// Verify files exist
	testFile := filepath.Join(tempDir, "infrastructure", "kustomization.yaml")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("test file was not created")
	}

	// Rollback stage
	if err := stage.Rollback(context.Background(), workspace); err != nil {
		t.Fatalf("failed to rollback stage: %v", err)
	}

	// Verify files were removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("test file was not removed during rollback")
	}
}

func TestConfigStage_Validate(t *testing.T) {
	tests := []struct {
		name       string
		config     v2.Config
		setupFiles []string
		wantErr    bool
	}{
		{
			name: "validates successfully with all required files",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
					Cluster: config.ClusterConfig{
						ClusterName: "test-cluster",
					},
				},
			},
			setupFiles: []string{
				"infrastructure/kustomization.yaml",
				"infrastructure/clusters/test-cluster/kustomization.yaml",
				"applications/kustomization.yaml",
				"applications/overlays/test-cluster/kustomization.yaml",
			},
			wantErr: false,
		},
		{
			name: "fails when required files are missing",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
					Cluster: config.ClusterConfig{
						ClusterName: "test-cluster",
					},
				},
			},
			setupFiles: []string{
				"infrastructure/kustomization.yaml",
			},
			wantErr: true,
		},
		{
			name: "fails when cluster name is missing",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
				},
			},
			setupFiles: []string{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary workspace
			tempDir := t.TempDir()
			workspace := &gitops.GitOpsWorkspace{
				ID:          "test-workspace",
				RootDir:     tempDir,
				TempDir:     filepath.Join(tempDir, ".tmp"),
				Config:      tt.config,
				Metadata:    make(map[string]interface{}),
				Checkpoints: make(map[string]gitops.WorkspaceCheckpoint),
			}

			// Create setup files
			for _, file := range tt.setupFiles {
				fullPath := filepath.Join(tempDir, file)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
					t.Fatalf("failed to create directory: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte("test content"), 0o644); err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
			}

			// Create mock template engine and registry
			engine := &mockTemplateEngine{}
			registry := newMockTemplateRegistry()

			// Create config stage
			stage := NewConfigStage(engine, registry)

			// Validate stage
			err := stage.Validate(context.Background(), workspace)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigStage.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigStage_DryRun(t *testing.T) {
	tests := []struct {
		name              string
		config            v2.Config
		templates         []tmpl.TemplateDefinition
		wantErr           bool
		expectedFileCount int
	}{
		{
			name: "returns plan with default configuration files",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
					Cluster: config.ClusterConfig{
						ClusterName: "test-cluster",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
			},
			templates:         []tmpl.TemplateDefinition{},
			wantErr:           false,
			expectedFileCount: 5, // 5 default config files
		},
		{
			name: "includes template files in plan",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
					Cluster: config.ClusterConfig{
						ClusterName: "test-cluster",
					},
					Infrastructure: config.Infrastructure{
						Provider: "openstack",
					},
				},
			},
			templates: []tmpl.TemplateDefinition{
				{
					Name: "cluster-config.yaml",
					Path: "testdata/cluster-config.yaml.tmpl",
					Type: tmpl.TemplateTypeConfig,
				},
			},
			wantErr:           false,
			expectedFileCount: 6, // 5 default + 1 template
		},
		{
			name: "fails when cluster name is missing",
			config: v2.Config{
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Organization: "test-org",
					},
				},
			},
			templates: []tmpl.TemplateDefinition{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock template engine and registry
			engine := &mockTemplateEngine{}
			registry := newMockTemplateRegistry()

			// Register templates
			for _, tmpl := range tt.templates {
				registry.RegisterTemplate(tmpl)
			}

			// Create config stage
			stage := NewConfigStage(engine, registry)

			// Execute dry run
			plan, err := stage.DryRun(context.Background(), tt.config)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigStage.DryRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify plan
			if plan == nil {
				t.Fatal("expected plan to be non-nil")
			}

			if plan.Name != "config" {
				t.Errorf("expected plan name to be 'config', got %s", plan.Name)
			}

			if len(plan.Files) != tt.expectedFileCount {
				t.Errorf("expected %d files in plan, got %d", tt.expectedFileCount, len(plan.Files))
			}

			if len(plan.Dependencies) != 1 || plan.Dependencies[0] != "service" {
				t.Errorf("expected dependencies to be ['service'], got %v", plan.Dependencies)
			}
		})
	}
}

func TestConfigStage_CreateDefaultConfigs(t *testing.T) {
	// Create temporary workspace
	tempDir := t.TempDir()
	cfg := v2.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Organization: "test-org",
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
		},
	}

	workspace := &gitops.GitOpsWorkspace{
		ID:          "test-workspace",
		RootDir:     tempDir,
		TempDir:     filepath.Join(tempDir, ".tmp"),
		Config:      cfg,
		Metadata:    make(map[string]interface{}),
		Checkpoints: make(map[string]gitops.WorkspaceCheckpoint),
	}

	// Create temp directory
	if err := os.MkdirAll(workspace.TempDir, 0o755); err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	// Create mock template engine and registry
	engine := &mockTemplateEngine{}
	registry := newMockTemplateRegistry()

	// Create config stage
	stage := NewConfigStage(engine, registry)

	// Create default configs
	if err := stage.createDefaultConfigs(context.Background(), workspace); err != nil {
		t.Fatalf("failed to create default configs: %v", err)
	}

	// Verify all default files were created
	expectedFiles := []string{
		"infrastructure/kustomization.yaml",
		"infrastructure/clusters/test-cluster/kustomization.yaml",
		"applications/kustomization.yaml",
		"applications/overlays/test-cluster/kustomization.yaml",
		".flux-system/kustomization.yaml",
	}

	for _, file := range expectedFiles {
		fullPath := filepath.Join(tempDir, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected file not created: %s", file)
		}

		// Verify file content is not empty
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("failed to read file %s: %v", file, err)
		}
		if len(content) == 0 {
			t.Errorf("file %s is empty", file)
		}
	}
}
