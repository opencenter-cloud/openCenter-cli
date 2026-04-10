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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServiceStageBasicExecution tests basic service stage execution
func TestServiceStageBasicExecution(t *testing.T) {
	// Create test configuration with enabled services
	cfg := createTestConfigWithServices([]string{"prometheus", "loki"})

	// Create temporary workspace
	tempDir := t.TempDir()
	workspace := createTestWorkspace(t, tempDir, cfg)

	// Create template engine and registries
	engine := template.NewGoTemplateEngine()
	templateRegistry := template.NewInMemoryTemplateRegistry()
	serviceRegistry := services.NewServiceRegistry()

	// Register test services
	err := serviceRegistry.RegisterService(services.ServiceDefinition{
		Name: "prometheus",
		Type: services.ServiceTypeMonitoring,
	})
	require.NoError(t, err)

	err = serviceRegistry.RegisterService(services.ServiceDefinition{
		Name: "loki",
		Type: services.ServiceTypeLogging,
	})
	require.NoError(t, err)

	// Create test template files
	prometheusTemplatePath := filepath.Join(tempDir, "prometheus-template.yaml")
	prometheusContent := `# Prometheus for {{ .OpenCenter.Cluster.ClusterName }}
enabled: true
namespace: monitoring
`
	err = os.WriteFile(prometheusTemplatePath, []byte(prometheusContent), 0o644)
	require.NoError(t, err)

	lokiTemplatePath := filepath.Join(tempDir, "loki-template.yaml")
	lokiContent := `# Loki for {{ .OpenCenter.Cluster.ClusterName }}
enabled: true
namespace: logging
`
	err = os.WriteFile(lokiTemplatePath, []byte(lokiContent), 0o644)
	require.NoError(t, err)

	// Register service templates
	err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
		Name:     "prometheus-config",
		Path:     prometheusTemplatePath,
		Type:     template.TemplateTypeService,
		Services: []string{"prometheus"},
		Metadata: template.TemplateMetadata{
			Description: "Prometheus service configuration",
		},
	})
	require.NoError(t, err)

	err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
		Name:     "loki-config",
		Path:     lokiTemplatePath,
		Type:     template.TemplateTypeService,
		Services: []string{"loki"},
		Metadata: template.TemplateMetadata{
			Description: "Loki service configuration",
		},
	})
	require.NoError(t, err)

	// Create init stage directories first
	initStage := NewInitStage()
	err = initStage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Create and execute service stage
	stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
	err = stage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Verify output files were created
	prometheusOutput := filepath.Join(tempDir, "applications", "overlays", "test-cluster", "prometheus", "prometheus-config.yaml")
	assert.FileExists(t, prometheusOutput)

	lokiOutput := filepath.Join(tempDir, "applications", "overlays", "test-cluster", "loki", "loki-config.yaml")
	assert.FileExists(t, lokiOutput)

	// Verify content
	prometheusData, err := os.ReadFile(prometheusOutput)
	require.NoError(t, err)
	assert.Contains(t, string(prometheusData), "namespace: monitoring")

	lokiData, err := os.ReadFile(lokiOutput)
	require.NoError(t, err)
	assert.Contains(t, string(lokiData), "namespace: logging")
}

// TestServiceStageNoServicesEnabled tests behavior when no services are enabled
func TestServiceStageNoServicesEnabled(t *testing.T) {
	// Create test configuration with no enabled services
	cfg := createTestConfigWithServices([]string{})

	// Create temporary workspace
	tempDir := t.TempDir()
	workspace := createTestWorkspace(t, tempDir, cfg)

	// Create template engine and registries
	engine := template.NewGoTemplateEngine()
	templateRegistry := template.NewInMemoryTemplateRegistry()
	serviceRegistry := services.NewServiceRegistry()

	// Create init stage directories
	initStage := NewInitStage()
	err := initStage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Create and execute service stage
	stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
	err = stage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Verify no service files were created
	applicationsDir := filepath.Join(tempDir, "applications", "overlays", "test-cluster")
	if _, err := os.Stat(applicationsDir); err == nil {
		// Directory exists, check it's empty or only has base files
		entries, err := os.ReadDir(applicationsDir)
		require.NoError(t, err)
		assert.Empty(t, entries, "No service files should be created when no services are enabled")
	}
}

// TestServiceStageServiceFiltering tests that only enabled services are rendered
func TestServiceStageServiceFiltering(t *testing.T) {
	tests := []struct {
		name            string
		enabledServices []string
		expectedFiles   []string
		unexpectedFiles []string
	}{
		{
			name:            "Only prometheus enabled",
			enabledServices: []string{"prometheus"},
			expectedFiles:   []string{"prometheus-config.yaml"},
			unexpectedFiles: []string{"loki-config.yaml", "grafana-config.yaml"},
		},
		{
			name:            "Multiple services enabled",
			enabledServices: []string{"prometheus", "loki"},
			expectedFiles:   []string{"prometheus-config.yaml", "loki-config.yaml"},
			unexpectedFiles: []string{"grafana-config.yaml"},
		},
		{
			name:            "All services enabled",
			enabledServices: []string{"prometheus", "loki", "grafana"},
			expectedFiles:   []string{"prometheus-config.yaml", "loki-config.yaml", "grafana-config.yaml"},
			unexpectedFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := createTestConfigWithServices(tt.enabledServices)

			// Create template engine and registries
			engine := template.NewGoTemplateEngine()
			templateRegistry := template.NewInMemoryTemplateRegistry()
			serviceRegistry := services.NewServiceRegistry()

			// Register all services
			allServices := []string{"prometheus", "loki", "grafana"}
			for _, svc := range allServices {
				err := serviceRegistry.RegisterService(services.ServiceDefinition{
					Name: svc,
					Type: services.ServiceTypeMonitoring,
				})
				require.NoError(t, err)
			}

			// Create temporary workspace
			tempDir := t.TempDir()
			workspace := createTestWorkspace(t, tempDir, cfg)

			// Create template files for all services
			for _, svc := range allServices {
				templatePath := filepath.Join(tempDir, svc+"-template.yaml")
				content := fmt.Sprintf("service: %s\n", svc)
				err := os.WriteFile(templatePath, []byte(content), 0o644)
				require.NoError(t, err)

				err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
					Name:     svc + "-config",
					Path:     templatePath,
					Type:     template.TemplateTypeService,
					Services: []string{svc},
				})
				require.NoError(t, err)
			}

			// Create init stage directories
			initStage := NewInitStage()
			err := initStage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Execute service stage
			stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
			err = stage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Verify expected files were created
			for _, filename := range tt.expectedFiles {
				// Extract service name from filename
				serviceName := filename[:len(filename)-len("-config.yaml")]
				outputPath := filepath.Join(tempDir, "applications", "overlays", "test-cluster", serviceName, filename)
				assert.FileExists(t, outputPath, "Expected file %s should exist", filename)
			}

			// Verify unexpected files were NOT created
			for _, filename := range tt.unexpectedFiles {
				serviceName := filename[:len(filename)-len("-config.yaml")]
				outputPath := filepath.Join(tempDir, "applications", "overlays", "test-cluster", serviceName, filename)
				assert.NoFileExists(t, outputPath, "Unexpected file %s should not exist", filename)
			}
		})
	}
}

// TestServiceStageDependencyResolution tests service dependency resolution
func TestServiceStageDependencyResolution(t *testing.T) {
	// Create test configuration with services that have dependencies
	cfg := createTestConfigWithServices([]string{"grafana", "prometheus"})

	// Create template engine and registries
	engine := template.NewGoTemplateEngine()
	templateRegistry := template.NewInMemoryTemplateRegistry()
	serviceRegistry := services.NewServiceRegistry()

	// Register services with dependencies (grafana depends on prometheus)
	err := serviceRegistry.RegisterService(services.ServiceDefinition{
		Name:         "prometheus",
		Type:         services.ServiceTypeMonitoring,
		Dependencies: []string{},
	})
	require.NoError(t, err)

	err = serviceRegistry.RegisterService(services.ServiceDefinition{
		Name:         "grafana",
		Type:         services.ServiceTypeMonitoring,
		Dependencies: []string{"prometheus"},
	})
	require.NoError(t, err)

	// Create temporary workspace
	tempDir := t.TempDir()
	workspace := createTestWorkspace(t, tempDir, cfg)

	// Create template files
	prometheusTemplate := filepath.Join(tempDir, "prometheus-template.yaml")
	err = os.WriteFile(prometheusTemplate, []byte("prometheus: config\n"), 0o644)
	require.NoError(t, err)

	grafanaTemplate := filepath.Join(tempDir, "grafana-template.yaml")
	err = os.WriteFile(grafanaTemplate, []byte("grafana: config\n"), 0o644)
	require.NoError(t, err)

	// Register templates
	err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
		Name:     "prometheus-config",
		Path:     prometheusTemplate,
		Type:     template.TemplateTypeService,
		Services: []string{"prometheus"},
	})
	require.NoError(t, err)

	err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
		Name:     "grafana-config",
		Path:     grafanaTemplate,
		Type:     template.TemplateTypeService,
		Services: []string{"grafana"},
	})
	require.NoError(t, err)

	// Create init stage directories
	initStage := NewInitStage()
	err = initStage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Execute service stage
	stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
	err = stage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Verify both services were rendered
	prometheusOutput := filepath.Join(tempDir, "applications", "overlays", "test-cluster", "prometheus", "prometheus-config.yaml")
	assert.FileExists(t, prometheusOutput)

	grafanaOutput := filepath.Join(tempDir, "applications", "overlays", "test-cluster", "grafana", "grafana-config.yaml")
	assert.FileExists(t, grafanaOutput)
}

// TestServiceStageRollback tests rollback functionality
func TestServiceStageRollback(t *testing.T) {
	// Create test configuration
	cfg := createTestConfigWithServices([]string{"prometheus"})

	// Create template engine and registries
	engine := template.NewGoTemplateEngine()
	templateRegistry := template.NewInMemoryTemplateRegistry()
	serviceRegistry := services.NewServiceRegistry()

	// Register service
	err := serviceRegistry.RegisterService(services.ServiceDefinition{
		Name: "prometheus",
		Type: services.ServiceTypeMonitoring,
	})
	require.NoError(t, err)

	// Create temporary workspace
	tempDir := t.TempDir()
	workspace := createTestWorkspace(t, tempDir, cfg)

	// Create template file
	templatePath := filepath.Join(tempDir, "prometheus-template.yaml")
	err = os.WriteFile(templatePath, []byte("prometheus: config\n"), 0o644)
	require.NoError(t, err)

	// Register template
	err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
		Name:     "prometheus-config",
		Path:     templatePath,
		Type:     template.TemplateTypeService,
		Services: []string{"prometheus"},
	})
	require.NoError(t, err)

	// Create init stage directories
	initStage := NewInitStage()
	err = initStage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Execute service stage
	stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
	err = stage.Execute(context.Background(), workspace)
	require.NoError(t, err)

	// Verify file was created
	outputPath := filepath.Join(tempDir, "applications", "overlays", "test-cluster", "prometheus", "prometheus-config.yaml")
	assert.FileExists(t, outputPath)

	// Rollback
	err = stage.Rollback(context.Background(), workspace)
	require.NoError(t, err)

	// Verify file was removed
	assert.NoFileExists(t, outputPath)
}

// TestServiceStageValidation tests validation functionality
func TestServiceStageValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupStage  bool
		expectError bool
	}{
		{
			name:        "Valid service configuration",
			setupStage:  true,
			expectError: false,
		},
		{
			name:        "Missing service files",
			setupStage:  false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := createTestConfigWithServices([]string{"prometheus"})

			// Create template engine and registries
			engine := template.NewGoTemplateEngine()
			templateRegistry := template.NewInMemoryTemplateRegistry()
			serviceRegistry := services.NewServiceRegistry()

			// Register service
			err := serviceRegistry.RegisterService(services.ServiceDefinition{
				Name: "prometheus",
				Type: services.ServiceTypeMonitoring,
			})
			require.NoError(t, err)

			// Create temporary workspace
			tempDir := t.TempDir()
			workspace := createTestWorkspace(t, tempDir, cfg)

			// Register template
			templatePath := filepath.Join(tempDir, "prometheus-template.yaml")
			err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
				Name:     "prometheus-config",
				Path:     templatePath,
				Type:     template.TemplateTypeService,
				Services: []string{"prometheus"},
			})
			require.NoError(t, err)

			// Create init stage directories
			initStage := NewInitStage()
			err = initStage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			if tt.setupStage {
				// Create template file
				err = os.WriteFile(templatePath, []byte("prometheus: config\n"), 0o644)
				require.NoError(t, err)

				// Execute service stage
				stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
				err = stage.Execute(context.Background(), workspace)
				require.NoError(t, err)
			}

			// Validate
			stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
			err = stage.Validate(context.Background(), workspace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestServiceStageDryRun tests dry-run functionality
func TestServiceStageDryRun(t *testing.T) {
	// Create test configuration
	cfg := createTestConfigWithServices([]string{"prometheus", "loki"})

	// Create template engine and registries
	engine := template.NewGoTemplateEngine()
	templateRegistry := template.NewInMemoryTemplateRegistry()
	serviceRegistry := services.NewServiceRegistry()

	// Register services
	err := serviceRegistry.RegisterService(services.ServiceDefinition{
		Name: "prometheus",
		Type: services.ServiceTypeMonitoring,
	})
	require.NoError(t, err)

	err = serviceRegistry.RegisterService(services.ServiceDefinition{
		Name: "loki",
		Type: services.ServiceTypeLogging,
	})
	require.NoError(t, err)

	// Register templates
	err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
		Name:     "prometheus-config",
		Path:     "prometheus-template.yaml",
		Type:     template.TemplateTypeService,
		Services: []string{"prometheus"},
	})
	require.NoError(t, err)

	err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
		Name:     "loki-config",
		Path:     "loki-template.yaml",
		Type:     template.TemplateTypeService,
		Services: []string{"loki"},
	})
	require.NoError(t, err)

	// Create service stage
	stage := NewServiceStage(engine, templateRegistry, serviceRegistry)

	// Execute dry-run
	plan, err := stage.DryRun(context.Background(), cfg)
	require.NoError(t, err)

	// Verify plan
	assert.Equal(t, "service", plan.Name)
	assert.Contains(t, plan.Description, "2 services enabled")
	assert.Len(t, plan.Files, 2)
	assert.Contains(t, plan.Files, "applications/overlays/test-cluster/prometheus/prometheus-config.yaml")
	assert.Contains(t, plan.Files, "applications/overlays/test-cluster/loki/loki-config.yaml")
}

// TestServiceStageConditions tests template condition evaluation
func TestServiceStageConditions(t *testing.T) {
	tests := []struct {
		name         string
		condition    template.RenderCondition
		services     []string
		shouldRender bool
	}{
		{
			name: "Service enabled condition matches",
			condition: template.RenderCondition{
				Type:  template.ConditionTypeEquals,
				Field: "service.prometheus",
				Value: true,
			},
			services:     []string{"prometheus"},
			shouldRender: true,
		},
		{
			name: "Service enabled condition doesn't match",
			condition: template.RenderCondition{
				Type:  template.ConditionTypeEquals,
				Field: "service.prometheus",
				Value: true,
			},
			services:     []string{}, // No services enabled
			shouldRender: false,
		},
		{
			name: "Service exists condition",
			condition: template.RenderCondition{
				Type:  template.ConditionTypeExists,
				Field: "service.prometheus",
			},
			services:     []string{"prometheus"},
			shouldRender: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration
			cfg := createTestConfigWithServices(tt.services)

			// Create template engine and registries
			engine := template.NewGoTemplateEngine()
			templateRegistry := template.NewInMemoryTemplateRegistry()
			serviceRegistry := services.NewServiceRegistry()

			// Register service
			err := serviceRegistry.RegisterService(services.ServiceDefinition{
				Name: "prometheus",
				Type: services.ServiceTypeMonitoring,
			})
			require.NoError(t, err)

			// Create temporary workspace
			tempDir := t.TempDir()
			workspace := createTestWorkspace(t, tempDir, cfg)

			// Create template file
			templatePath := filepath.Join(tempDir, "conditional-template.yaml")
			err = os.WriteFile(templatePath, []byte("conditional: config\n"), 0o644)
			require.NoError(t, err)

			// Register template with condition
			err = templateRegistry.RegisterTemplate(template.TemplateDefinition{
				Name:       "conditional-config",
				Path:       templatePath,
				Type:       template.TemplateTypeService,
				Services:   []string{"prometheus"},
				Conditions: []template.RenderCondition{tt.condition},
			})
			require.NoError(t, err)

			// Create init stage directories
			initStage := NewInitStage()
			err = initStage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Execute service stage
			stage := NewServiceStage(engine, templateRegistry, serviceRegistry)
			err = stage.Execute(context.Background(), workspace)
			require.NoError(t, err)

			// Check if file was created based on condition
			outputPath := filepath.Join(tempDir, "applications", "overlays", "test-cluster", "prometheus", "conditional-config.yaml")
			if tt.shouldRender {
				assert.FileExists(t, outputPath)
			} else {
				assert.NoFileExists(t, outputPath)
			}
		})
	}
}

// Helper functions

func createTestConfigWithServices(enabledServices []string) v2.Config {
	services := make(config.ServiceMap)
	for _, svc := range enabledServices {
		services[svc] = config.ServiceCfg{
			Enabled: true,
		}
	}

	return v2.Config{
		SchemaVersion: "1.0.0",
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "test-org",
			},
			Infrastructure: config.Infrastructure{
				Provider: "openstack",
			},
			Cluster: config.ClusterConfig{
				ClusterName: "test-cluster",
			},
			Services: services,
		},
	}
}
