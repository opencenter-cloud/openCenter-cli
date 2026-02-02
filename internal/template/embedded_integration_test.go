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

	"github.com/rackerlabs/opencenter-cli/internal/gitops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterRealGitOpsTemplates tests registration of actual embedded gitops templates
func TestRegisterRealGitOpsTemplates(t *testing.T) {
	registry := NewInMemoryTemplateRegistry()

	// Register templates from the actual gitops embedded filesystem
	err := RegisterGitOpsTemplates(registry, gitops.Files)
	require.NoError(t, err)

	// Verify templates were registered
	templates := registry.ListTemplates()
	assert.Greater(t, len(templates), 0, "should have registered templates from gitops.Files")

	// Log registered templates for debugging
	t.Logf("Registered %d templates from gitops.Files", len(templates))
	for _, tmpl := range templates {
		t.Logf("  - %s (type: %s, provider: %s, services: %v)",
			tmpl.Name, tmpl.Type, tmpl.Provider, tmpl.Services)
	}

	// Verify we have infrastructure templates
	infraTemplates := make([]TemplateDefinition, 0)
	for _, tmpl := range templates {
		if tmpl.Type == TemplateTypeInfrastructure {
			infraTemplates = append(infraTemplates, tmpl)
		}
	}
	assert.Greater(t, len(infraTemplates), 0, "should have infrastructure templates")

	// Verify we have service templates
	serviceTemplates := make([]TemplateDefinition, 0)
	for _, tmpl := range templates {
		if tmpl.Type == TemplateTypeService {
			serviceTemplates = append(serviceTemplates, tmpl)
		}
	}
	assert.Greater(t, len(serviceTemplates), 0, "should have service templates")

	// Verify specific expected templates exist
	expectedTemplates := []string{
		"main.tf",
		"variables.tf",
		"Makefile",
	}

	for _, expectedName := range expectedTemplates {
		_, err := registry.GetTemplate(expectedName)
		assert.NoError(t, err, "expected template %s should be registered", expectedName)
	}
}

// TestRegisterRealProvisionTemplates tests registration of actual embedded provision templates
func TestRegisterRealProvisionTemplates(t *testing.T) {
	t.Skip("Skipping: provision.templatesFS is not exported, tested via mock in other tests")
}

// TestGlobalTemplateRegistry tests the global registry initialization
func TestGlobalTemplateRegistry(t *testing.T) {
	// Create a new global registry
	globalRegistry := NewInMemoryTemplateRegistry()

	// Register all embedded templates
	err := RegisterGitOpsTemplates(globalRegistry, gitops.Files)
	require.NoError(t, err)

	// Note: provision templates are not exported, so we only test gitops
	// In production, we would also register provision templates

	// Verify we have templates from gitops
	templates := globalRegistry.ListTemplates()
	assert.Greater(t, len(templates), 5, "should have many templates registered")

	t.Logf("Total templates registered: %d", len(templates))

	// Verify we can filter by provider
	baremetalTemplates := globalRegistry.GetTemplatesForProvider("baremetal")
	t.Logf("Baremetal templates: %d", len(baremetalTemplates))

	// Verify we can filter by service
	lokiTemplates := globalRegistry.GetTemplatesForService("loki")
	t.Logf("Loki templates: %d", len(lokiTemplates))

	// Verify template metadata is populated
	for _, tmpl := range templates {
		assert.NotEmpty(t, tmpl.Name, "template should have a name")
		assert.NotEmpty(t, tmpl.Path, "template should have a path")
		assert.NotEmpty(t, tmpl.Type, "template should have a type")
	}
}

// TestTemplateRegistryFiltering tests various filtering capabilities
func TestTemplateRegistryFiltering(t *testing.T) {
	registry := NewInMemoryTemplateRegistry()

	// Register all templates
	err := RegisterGitOpsTemplates(registry, gitops.Files)
	require.NoError(t, err)

	// Note: provision templates are not exported, so we only test gitops

	t.Run("filter by provider", func(t *testing.T) {
		// Test filtering by different providers
		providers := []string{"baremetal", "openstack", "aws", ""}

		for _, provider := range providers {
			templates := registry.GetTemplatesForProvider(provider)
			t.Logf("Provider '%s': %d templates", provider, len(templates))

			// Verify all returned templates match the provider or are universal
			for _, tmpl := range templates {
				if tmpl.Provider != "" {
					assert.Equal(t, provider, tmpl.Provider,
						"template %s should match provider filter", tmpl.Name)
				}
			}
		}
	})

	t.Run("filter by service", func(t *testing.T) {
		// Test filtering by different services
		services := []string{"loki", "alert-proxy", "prometheus", "cert-manager"}

		for _, service := range services {
			templates := registry.GetTemplatesForService(service)
			t.Logf("Service '%s': %d templates", service, len(templates))

			// Verify all returned templates are associated with the service
			for _, tmpl := range templates {
				assert.Contains(t, tmpl.Services, service,
					"template %s should be associated with service %s", tmpl.Name, service)
			}
		}
	})

	t.Run("filter by enabled services", func(t *testing.T) {
		enabledServices := []string{"loki", "prometheus"}
		templates := registry.GetTemplatesForEnabledServices(enabledServices)

		t.Logf("Enabled services %v: %d templates", enabledServices, len(templates))

		// Verify returned templates are either universal or associated with enabled services
		for _, tmpl := range templates {
			if len(tmpl.Services) > 0 {
				hasEnabledService := false
				for _, svc := range tmpl.Services {
					for _, enabled := range enabledServices {
						if svc == enabled {
							hasEnabledService = true
							break
						}
					}
				}
				assert.True(t, hasEnabledService,
					"template %s should have at least one enabled service", tmpl.Name)
			}
		}
	})
}

// TestTemplateMetadata verifies that registered templates have proper metadata
func TestTemplateMetadata(t *testing.T) {
	registry := NewInMemoryTemplateRegistry()

	err := RegisterGitOpsTemplates(registry, gitops.Files)
	require.NoError(t, err)

	templates := registry.ListTemplates()
	require.Greater(t, len(templates), 0)

	for _, tmpl := range templates {
		t.Run(tmpl.Name, func(t *testing.T) {
			// Verify basic fields
			assert.NotEmpty(t, tmpl.Name, "template should have a name")
			assert.NotEmpty(t, tmpl.Path, "template should have a path")
			assert.NotEmpty(t, tmpl.Type, "template should have a type")

			// Verify metadata
			assert.NotEmpty(t, tmpl.Metadata.Version, "template should have a version")
			assert.GreaterOrEqual(t, tmpl.Metadata.Priority, 0, "template should have a priority")

			// Log template details
			t.Logf("Template: %s", tmpl.Name)
			t.Logf("  Path: %s", tmpl.Path)
			t.Logf("  Type: %s", tmpl.Type)
			t.Logf("  Provider: %s", tmpl.Provider)
			t.Logf("  Services: %v", tmpl.Services)
			t.Logf("  Priority: %d", tmpl.Metadata.Priority)
			t.Logf("  Version: %s", tmpl.Metadata.Version)
		})
	}
}
