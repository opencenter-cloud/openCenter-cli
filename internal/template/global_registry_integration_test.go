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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllEmbeddedTemplatesRegistered verifies that all embedded templates
// from all sources are registered in the global registry
func TestAllEmbeddedTemplatesRegistered(t *testing.T) {
	// Reset the global registry to ensure clean state
	ResetGlobalRegistry()

	// Get the global registry
	registry, err := GetGlobalRegistry()
	require.NoError(t, err, "should initialize global registry without error")
	require.NotNil(t, registry, "global registry should not be nil")

	// List all registered templates
	templates := registry.ListTemplates()
	require.Greater(t, len(templates), 0, "should have registered templates")

	t.Logf("Total templates registered: %d", len(templates))

	// Verify we have templates from different sources
	infraTemplates := registry.GetTemplatesForType(TemplateTypeInfrastructure)
	serviceTemplates := registry.GetTemplatesForType(TemplateTypeService)
	baseTemplates := registry.GetTemplatesForType(TemplateTypeBase)

	t.Logf("Infrastructure templates: %d", len(infraTemplates))
	t.Logf("Service templates: %d", len(serviceTemplates))
	t.Logf("Base templates: %d", len(baseTemplates))

	// We should have templates from gitops (infrastructure and services)
	assert.Greater(t, len(infraTemplates), 0, "should have infrastructure templates from gitops")
	assert.Greater(t, len(serviceTemplates), 0, "should have service templates from gitops")

	// Verify we have templates from provision package
	provisionTemplates := 0
	for _, tmpl := range templates {
		// Provision templates are in the "templates" directory and are infrastructure type
		if tmpl.Type == TemplateTypeInfrastructure && 
		   (tmpl.Name == "main.tf" || tmpl.Name == "variables.tf" || tmpl.Name == "inventory") {
			provisionTemplates++
		}
	}
	assert.Greater(t, provisionTemplates, 0, "should have templates from provision package")

	// Verify provider-specific templates
	openstackTemplates := registry.GetTemplatesForProvider("openstack")
	baremetalTemplates := registry.GetTemplatesForProvider("baremetal")
	vsphereTemplates := registry.GetTemplatesForProvider("vsphere")

	t.Logf("OpenStack templates: %d", len(openstackTemplates))
	t.Logf("Baremetal templates: %d", len(baremetalTemplates))
	t.Logf("vSphere templates: %d", len(vsphereTemplates))

	assert.Greater(t, len(openstackTemplates), 0, "should have openstack-specific templates")
	assert.Greater(t, len(baremetalTemplates), 0, "should have baremetal-specific templates")
	assert.Greater(t, len(vsphereTemplates), 0, "should have vsphere-specific templates")

	// Verify service-specific templates
	lokiTemplates := registry.GetTemplatesForService("loki")
	prometheusTemplates := registry.GetTemplatesForService("prometheus")
	certManagerTemplates := registry.GetTemplatesForService("cert-manager")

	t.Logf("Loki templates: %d", len(lokiTemplates))
	t.Logf("Prometheus templates: %d", len(prometheusTemplates))
	t.Logf("Cert-manager templates: %d", len(certManagerTemplates))

	assert.Greater(t, len(lokiTemplates), 0, "should have loki service templates")
	assert.Greater(t, len(prometheusTemplates), 0, "should have prometheus service templates")
	assert.Greater(t, len(certManagerTemplates), 0, "should have cert-manager service templates")
}

// TestGlobalRegistryInitializationIdempotent verifies that calling GetGlobalRegistry
// multiple times returns the same registry instance
func TestGlobalRegistryInitializationIdempotent(t *testing.T) {
	// Reset to ensure clean state
	ResetGlobalRegistry()

	// Get registry multiple times
	registry1, err1 := GetGlobalRegistry()
	require.NoError(t, err1)

	registry2, err2 := GetGlobalRegistry()
	require.NoError(t, err2)

	registry3, err3 := GetGlobalRegistry()
	require.NoError(t, err3)

	// All should return the same instance
	assert.Equal(t, registry1, registry2, "should return same registry instance")
	assert.Equal(t, registry2, registry3, "should return same registry instance")

	// Template counts should be identical
	templates1 := registry1.ListTemplates()
	templates2 := registry2.ListTemplates()
	templates3 := registry3.ListTemplates()

	assert.Equal(t, len(templates1), len(templates2), "template counts should match")
	assert.Equal(t, len(templates2), len(templates3), "template counts should match")
}

// TestGlobalRegistryTemplateMetadata verifies that registered templates
// have proper metadata
func TestGlobalRegistryTemplateMetadata(t *testing.T) {
	// Reset to ensure clean state
	ResetGlobalRegistry()

	registry, err := GetGlobalRegistry()
	require.NoError(t, err)

	templates := registry.ListTemplates()
	require.Greater(t, len(templates), 0, "should have templates")

	// Verify all templates have required fields
	for _, tmpl := range templates {
		assert.NotEmpty(t, tmpl.Name, "template should have a name")
		assert.NotEmpty(t, tmpl.Path, "template should have a path")
		assert.NotEmpty(t, tmpl.Type, "template should have a type")
		// Provider can be empty (universal templates)
		// Services can be empty (non-service templates)
	}

	// Verify templates have proper priorities
	infraTemplates := registry.GetTemplatesForType(TemplateTypeInfrastructure)
	for _, tmpl := range infraTemplates {
		assert.GreaterOrEqual(t, tmpl.Metadata.Priority, 0, "priority should be non-negative")
	}
}

// TestGlobalRegistryTemplateResolution verifies that template dependencies
// can be resolved correctly
func TestGlobalRegistryTemplateResolution(t *testing.T) {
	// Reset to ensure clean state
	ResetGlobalRegistry()

	registry, err := GetGlobalRegistry()
	require.NoError(t, err)

	// Get a few templates to test resolution
	templates := registry.ListTemplates()
	require.Greater(t, len(templates), 0, "should have templates")

	// Try to resolve dependencies for a subset of templates
	templateNames := []string{}
	for i := 0; i < 5 && i < len(templates); i++ {
		templateNames = append(templateNames, templates[i].Name)
	}

	resolved, err := registry.ResolveTemplateDependencies(templateNames)
	require.NoError(t, err, "should resolve template dependencies")
	assert.GreaterOrEqual(t, len(resolved), len(templateNames), "resolved should include at least requested templates")
}

// TestGlobalRegistryEnabledServicesFiltering verifies that templates
// are correctly filtered based on enabled services
func TestGlobalRegistryEnabledServicesFiltering(t *testing.T) {
	// Reset to ensure clean state
	ResetGlobalRegistry()

	registry, err := GetGlobalRegistry()
	require.NoError(t, err)

	// Test with no enabled services - should only return universal templates
	noServicesTemplates := registry.GetTemplatesForEnabledServices([]string{})
	t.Logf("Templates with no services enabled: %d", len(noServicesTemplates))

	// Test with specific services enabled
	enabledServices := []string{"loki", "prometheus", "cert-manager"}
	filteredTemplates := registry.GetTemplatesForEnabledServices(enabledServices)
	t.Logf("Templates with services %v enabled: %d", enabledServices, len(filteredTemplates))

	// Filtered templates should include universal templates plus service-specific ones
	assert.GreaterOrEqual(t, len(filteredTemplates), len(noServicesTemplates), 
		"enabling services should not reduce template count")

	// Verify that all returned templates either have no services or have at least one enabled service
	for _, tmpl := range filteredTemplates {
		if len(tmpl.Services) > 0 {
			hasEnabledService := false
			for _, svc := range tmpl.Services {
				for _, enabled := range enabledServices {
					if svc == enabled {
						hasEnabledService = true
						break
					}
				}
				if hasEnabledService {
					break
				}
			}
			assert.True(t, hasEnabledService, 
				"template %s with services %v should have at least one enabled service", 
				tmpl.Name, tmpl.Services)
		}
	}
}
