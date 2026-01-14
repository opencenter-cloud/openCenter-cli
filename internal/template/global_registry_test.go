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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGlobalRegistry(t *testing.T) {
	// Reset the global registry before test
	ResetGlobalRegistry()

	// Get the global registry
	registry, err := GetGlobalRegistry()
	require.NoError(t, err)
	require.NotNil(t, registry)

	// Verify templates were registered
	templates := registry.ListTemplates()
	assert.Greater(t, len(templates), 0, "global registry should have templates")

	t.Logf("Global registry has %d templates", len(templates))
}

func TestGetGlobalRegistry_Singleton(t *testing.T) {
	// Reset the global registry before test
	ResetGlobalRegistry()

	// Get the registry multiple times
	registry1, err1 := GetGlobalRegistry()
	require.NoError(t, err1)

	registry2, err2 := GetGlobalRegistry()
	require.NoError(t, err2)

	// Verify they are the same instance
	assert.Equal(t, registry1, registry2, "should return the same registry instance")

	// Verify templates are the same
	templates1 := registry1.ListTemplates()
	templates2 := registry2.ListTemplates()
	assert.Equal(t, len(templates1), len(templates2), "should have same number of templates")
}

func TestGetGlobalRegistry_ThreadSafe(t *testing.T) {
	// Reset the global registry before test
	ResetGlobalRegistry()

	// Call GetGlobalRegistry concurrently from multiple goroutines
	const numGoroutines = 10
	var wg sync.WaitGroup
	registries := make([]TemplateRegistry, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			registries[index], errors[index] = GetGlobalRegistry()
		}(i)
	}

	wg.Wait()

	// Verify all calls succeeded
	for i, err := range errors {
		assert.NoError(t, err, "goroutine %d should not have error", i)
	}

	// Verify all returned the same instance
	firstRegistry := registries[0]
	for i, registry := range registries {
		assert.Equal(t, firstRegistry, registry, "goroutine %d should return same instance", i)
	}
}

func TestGlobalRegistry_TemplateAccess(t *testing.T) {
	// Reset the global registry before test
	ResetGlobalRegistry()

	registry, err := GetGlobalRegistry()
	require.NoError(t, err)

	// Test various registry operations
	t.Run("list all templates", func(t *testing.T) {
		templates := registry.ListTemplates()
		assert.Greater(t, len(templates), 0)
	})

	t.Run("get template by name", func(t *testing.T) {
		// Get a known template
		template, err := registry.GetTemplate("main.tf")
		assert.NoError(t, err)
		assert.Equal(t, "main.tf", template.Name)
	})

	t.Run("filter by provider", func(t *testing.T) {
		baremetalTemplates := registry.GetTemplatesForProvider("baremetal")
		t.Logf("Found %d baremetal templates", len(baremetalTemplates))

		// Verify all returned templates are for baremetal or universal
		for _, tmpl := range baremetalTemplates {
			assert.True(t, tmpl.Provider == "baremetal" || tmpl.Provider == "",
				"template %s should be baremetal or universal", tmpl.Name)
		}
	})

	t.Run("filter by service", func(t *testing.T) {
		lokiTemplates := registry.GetTemplatesForService("loki")
		t.Logf("Found %d loki templates", len(lokiTemplates))

		// Verify all returned templates are associated with loki
		for _, tmpl := range lokiTemplates {
			assert.Contains(t, tmpl.Services, "loki",
				"template %s should be associated with loki", tmpl.Name)
		}
	})

	t.Run("filter by enabled services", func(t *testing.T) {
		enabledServices := []string{"loki", "prometheus"}
		templates := registry.GetTemplatesForEnabledServices(enabledServices)
		t.Logf("Found %d templates for enabled services %v", len(templates), enabledServices)

		// Verify returned templates are either universal or have enabled services
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

func TestResetGlobalRegistry(t *testing.T) {
	// Get the registry
	registry1, err := GetGlobalRegistry()
	require.NoError(t, err)
	require.NotNil(t, registry1)

	// Reset it
	ResetGlobalRegistry()

	// Get it again
	registry2, err := GetGlobalRegistry()
	require.NoError(t, err)
	require.NotNil(t, registry2)

	// They should be different instances after reset
	// Note: We can't directly compare pointers, but we can verify they work independently
	templates1 := registry1.ListTemplates()
	templates2 := registry2.ListTemplates()

	// Both should have templates
	assert.Greater(t, len(templates1), 0)
	assert.Greater(t, len(templates2), 0)
}
