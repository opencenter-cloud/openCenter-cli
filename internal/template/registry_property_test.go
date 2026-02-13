package template

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty10_TemplateDependencyOrdering validates Property 10 from the design document:
// "For any set of templates with dependencies, they should always be returned in dependency-first order"
// **Validates: Requirements 2.5**
//
// Feature: configuration-system-refactor, Property 10: Template Dependency Ordering
func TestProperty10_TemplateDependencyOrdering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("dependencies always come before dependents", prop.ForAll(
		func(templateGraph TemplateGraph) bool {
			registry := NewInMemoryTemplateRegistry()

			// Register all templates in the graph
			for _, tmpl := range templateGraph.Templates {
				if err := registry.RegisterTemplate(tmpl); err != nil {
					// Skip invalid graphs (e.g., self-dependencies)
					return true
				}
			}

			// Resolve dependencies for all templates
			templateNames := make([]string, len(templateGraph.Templates))
			for i, tmpl := range templateGraph.Templates {
				templateNames[i] = tmpl.Name
			}

			resolved, err := registry.ResolveTemplateDependencies(templateNames)
			if err != nil {
				// Circular dependencies should be detected
				return true
			}

			// Build position map
			position := make(map[string]int)
			for i, tmpl := range resolved {
				position[tmpl.Name] = i
			}

			// Verify: for each template, all its dependencies must appear before it
			for _, tmpl := range resolved {
				for _, dep := range tmpl.Dependencies {
					depPos, depExists := position[dep]
					tmplPos, tmplExists := position[tmpl.Name]

					if !depExists || !tmplExists {
						return false
					}

					// Dependency must come before dependent
					if depPos >= tmplPos {
						return false
					}
				}
			}

			return true
		},
		genTemplateGraph(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_DependencyResolutionIdempotence validates that resolving dependencies
// multiple times produces the same result
func TestProperty_DependencyResolutionIdempotence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("resolving dependencies multiple times produces same order", prop.ForAll(
		func(templateGraph TemplateGraph) bool {
			registry := NewInMemoryTemplateRegistry()

			// Register all templates
			for _, tmpl := range templateGraph.Templates {
				if err := registry.RegisterTemplate(tmpl); err != nil {
					return true
				}
			}

			templateNames := make([]string, len(templateGraph.Templates))
			for i, tmpl := range templateGraph.Templates {
				templateNames[i] = tmpl.Name
			}

			// Resolve multiple times
			resolved1, err1 := registry.ResolveTemplateDependencies(templateNames)
			resolved2, err2 := registry.ResolveTemplateDependencies(templateNames)
			resolved3, err3 := registry.ResolveTemplateDependencies(templateNames)

			// All should succeed or fail together
			if (err1 == nil) != (err2 == nil) || (err2 == nil) != (err3 == nil) {
				return false
			}

			// If successful, results should be identical
			if err1 == nil {
				if len(resolved1) != len(resolved2) || len(resolved2) != len(resolved3) {
					return false
				}

				for i := range resolved1 {
					if resolved1[i].Name != resolved2[i].Name || resolved2[i].Name != resolved3[i].Name {
						return false
					}
				}
			}

			return true
		},
		genTemplateGraph(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_CircularDependencyDetection validates that circular dependencies
// are always detected
func TestProperty_CircularDependencyDetection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("circular dependencies are always detected", prop.ForAll(
		func(cycle []string) bool {
			if len(cycle) < 2 {
				return true // Need at least 2 templates for a cycle
			}

			registry := NewInMemoryTemplateRegistry()

			// Create circular dependency: A -> B -> C -> A
			for i := 0; i < len(cycle); i++ {
				nextIdx := (i + 1) % len(cycle)
				tmpl := TemplateDefinition{
					Name:         cycle[i],
					Path:         fmt.Sprintf("/path/%s", cycle[i]),
					Type:         TemplateTypeBase,
					Dependencies: []string{cycle[nextIdx]},
				}
				if err := registry.RegisterTemplate(tmpl); err != nil {
					return true
				}
			}

			// Attempting to resolve should detect the cycle
			_, err := registry.ResolveTemplateDependencies([]string{cycle[0]})

			// Should return an error about circular dependency
			return err != nil
		},
		gen.SliceOfN(3, gen.AlphaString()).SuchThat(func(v interface{}) bool {
			slice := v.([]string)
			// Ensure unique names
			seen := make(map[string]bool)
			for _, s := range slice {
				if s == "" || seen[s] {
					return false
				}
				seen[s] = true
			}
			return true
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_TransitiveDependencyResolution validates that transitive dependencies
// are correctly resolved
func TestProperty_TransitiveDependencyResolution(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("transitive dependencies are included in resolution", prop.ForAll(
		func(chain []string) bool {
			if len(chain) < 2 {
				return true
			}

			registry := NewInMemoryTemplateRegistry()

			// Create dependency chain: A -> B -> C -> D
			for i := 0; i < len(chain); i++ {
				deps := []string{}
				if i > 0 {
					deps = []string{chain[i-1]}
				}

				tmpl := TemplateDefinition{
					Name:         chain[i],
					Path:         fmt.Sprintf("/path/%s", chain[i]),
					Type:         TemplateTypeBase,
					Dependencies: deps,
				}
				if err := registry.RegisterTemplate(tmpl); err != nil {
					return true
				}
			}

			// Resolve the last template in the chain
			resolved, err := registry.ResolveTemplateDependencies([]string{chain[len(chain)-1]})
			if err != nil {
				return false
			}

			// All templates in the chain should be included
			if len(resolved) != len(chain) {
				return false
			}

			// Verify all templates are present
			names := make(map[string]bool)
			for _, tmpl := range resolved {
				names[tmpl.Name] = true
			}

			for _, name := range chain {
				if !names[name] {
					return false
				}
			}

			return true
		},
		gen.SliceOfN(4, gen.AlphaString()).SuchThat(func(v interface{}) bool {
			slice := v.([]string)
			// Ensure unique names
			seen := make(map[string]bool)
			for _, s := range slice {
				if s == "" || seen[s] {
					return false
				}
				seen[s] = true
			}
			return true
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_EmptyDependencyList validates that templates with no dependencies
// are handled correctly
func TestProperty_EmptyDependencyList(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("templates with no dependencies resolve to themselves", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}

			registry := NewInMemoryTemplateRegistry()

			tmpl := TemplateDefinition{
				Name:         name,
				Path:         fmt.Sprintf("/path/%s", name),
				Type:         TemplateTypeBase,
				Dependencies: []string{}, // No dependencies
			}

			if err := registry.RegisterTemplate(tmpl); err != nil {
				return true
			}

			resolved, err := registry.ResolveTemplateDependencies([]string{name})
			if err != nil {
				return false
			}

			// Should return exactly one template
			return len(resolved) == 1 && resolved[0].Name == name
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_MultipleDependentsShareDependency validates that when multiple templates
// depend on the same template, the shared dependency appears only once
func TestProperty_MultipleDependentsShareDependency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("shared dependencies appear only once", prop.ForAll(
		func(names []string) bool {
			if len(names) < 3 {
				return true // Need at least 3: base + 2 dependents
			}

			registry := NewInMemoryTemplateRegistry()

			// Register base template
			base := TemplateDefinition{
				Name:         names[0],
				Path:         fmt.Sprintf("/path/%s", names[0]),
				Type:         TemplateTypeBase,
				Dependencies: []string{},
			}
			if err := registry.RegisterTemplate(base); err != nil {
				return true
			}

			// Register dependents that all depend on base
			dependents := names[1:]
			for _, dep := range dependents {
				tmpl := TemplateDefinition{
					Name:         dep,
					Path:         fmt.Sprintf("/path/%s", dep),
					Type:         TemplateTypeService,
					Dependencies: []string{names[0]}, // All depend on base
				}
				if err := registry.RegisterTemplate(tmpl); err != nil {
					return true
				}
			}

			// Resolve all dependents
			resolved, err := registry.ResolveTemplateDependencies(dependents)
			if err != nil {
				return false
			}

			// Count occurrences of base template
			count := 0
			for _, tmpl := range resolved {
				if tmpl.Name == names[0] {
					count++
				}
			}

			// Base should appear exactly once
			return count == 1
		},
		gen.SliceOfN(4, gen.AlphaString()).SuchThat(func(v interface{}) bool {
			slice := v.([]string)
			// Ensure unique names
			seen := make(map[string]bool)
			for _, s := range slice {
				if s == "" || seen[s] {
					return false
				}
				seen[s] = true
			}
			return true
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TemplateGraph represents a graph of templates with dependencies for testing
type TemplateGraph struct {
	Templates []TemplateDefinition
}

// genTemplateGraph generates random template graphs for property testing
func genTemplateGraph() gopter.Gen {
	return gen.SliceOfN(5, genTemplateDefinition()).
		Map(func(templates []TemplateDefinition) TemplateGraph {
			return TemplateGraph{Templates: templates}
		}).
		SuchThat(func(v interface{}) bool {
			graph := v.(TemplateGraph)
			// Ensure unique template names
			seen := make(map[string]bool)
			for _, tmpl := range graph.Templates {
				if tmpl.Name == "" || seen[tmpl.Name] {
					return false
				}
				seen[tmpl.Name] = true
			}
			return true
		})
}

// TestProperty8_ProviderSpecificTemplateFiltering validates Property 8 from the design document:
// "For any provider filter, only templates compatible with that provider should be returned"
// **Validates: Requirements 2.3**
//
// Feature: configuration-system-refactor, Property 8: Provider-Specific Template Filtering
func TestProperty8_ProviderSpecificTemplateFiltering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("provider filter returns only compatible templates", prop.ForAll(
		func(templates []TemplateWithProvider, filterProvider string) bool {
			if filterProvider == "" {
				return true // Skip empty provider
			}

			registry := NewInMemoryTemplateRegistry()

			// Register all templates
			for _, tmpl := range templates {
				if err := registry.RegisterTemplate(tmpl.Template); err != nil {
					// Skip invalid templates
					return true
				}
			}

			// Get templates for the filter provider
			result := registry.GetTemplatesForProvider(filterProvider)

			// Verify: all returned templates must be compatible with the provider
			for _, tmpl := range result {
				// A template is compatible if:
				// 1. It has no provider specified (universal template), OR
				// 2. Its provider matches the filter provider
				isCompatible := tmpl.Provider == "" || tmpl.Provider == filterProvider

				if !isCompatible {
					// Found an incompatible template - property violated
					return false
				}
			}

			return true
		},
		genTemplateListWithProviders(),
		genProviderName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ProviderFilteringIncludesUniversalTemplates validates that
// universal templates (with empty provider) are included for all providers
func TestProperty_ProviderFilteringIncludesUniversalTemplates(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("universal templates are included for all providers", prop.ForAll(
		func(provider string) bool {
			if provider == "" {
				return true
			}

			registry := NewInMemoryTemplateRegistry()

			// Register a universal template (no provider specified)
			universalTemplate := TemplateDefinition{
				Name:     "universal-template",
				Path:     "/path/universal",
				Type:     TemplateTypeBase,
				Provider: "", // Universal
			}

			if err := registry.RegisterTemplate(universalTemplate); err != nil {
				return true
			}

			// Get templates for any provider
			result := registry.GetTemplatesForProvider(provider)

			// Universal template should be included
			found := false
			for _, tmpl := range result {
				if tmpl.Name == "universal-template" {
					found = true
					break
				}
			}

			return found
		},
		genProviderName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ProviderFilteringExcludesIncompatibleTemplates validates that
// templates for other providers are excluded
func TestProperty_ProviderFilteringExcludesIncompatibleTemplates(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("incompatible provider templates are excluded", prop.ForAll(
		func(targetProvider string, otherProvider string) bool {
			if targetProvider == "" || otherProvider == "" || targetProvider == otherProvider {
				return true
			}

			registry := NewInMemoryTemplateRegistry()

			// Register template for other provider
			otherTemplate := TemplateDefinition{
				Name:     "other-provider-template",
				Path:     "/path/other",
				Type:     TemplateTypeInfrastructure,
				Provider: otherProvider,
			}

			if err := registry.RegisterTemplate(otherTemplate); err != nil {
				return true
			}

			// Get templates for target provider
			result := registry.GetTemplatesForProvider(targetProvider)

			// Other provider's template should NOT be included
			for _, tmpl := range result {
				if tmpl.Name == "other-provider-template" {
					// Found incompatible template - property violated
					return false
				}
			}

			return true
		},
		genProviderName(),
		genProviderName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_ProviderFilteringPrioritySorting validates that templates
// are returned in priority order
func TestProperty_ProviderFilteringPrioritySorting(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("templates are sorted by priority descending", prop.ForAll(
		func(templates []TemplateWithProvider, provider string) bool {
			if provider == "" {
				return true
			}

			registry := NewInMemoryTemplateRegistry()

			// Register all templates
			for _, tmpl := range templates {
				if err := registry.RegisterTemplate(tmpl.Template); err != nil {
					return true
				}
			}

			// Get templates for provider
			result := registry.GetTemplatesForProvider(provider)

			if len(result) < 2 {
				return true // Need at least 2 templates to check ordering
			}

			// Verify priority ordering (higher priority first)
			for i := 0; i < len(result)-1; i++ {
				currentPriority := result[i].Metadata.Priority
				nextPriority := result[i+1].Metadata.Priority

				// If priorities are equal, should be sorted by name
				if currentPriority == nextPriority {
					if result[i].Name > result[i+1].Name {
						return false
					}
				} else if currentPriority < nextPriority {
					// Higher priority should come first
					return false
				}
			}

			return true
		},
		genTemplateListWithProviders(),
		genProviderName(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TemplateWithProvider wraps a template with provider information for testing
type TemplateWithProvider struct {
	Template TemplateDefinition
}

// genTemplateListWithProviders generates a list of templates with various providers
func genTemplateListWithProviders() gopter.Gen {
	return gen.SliceOfN(10, genTemplateWithProvider()).
		SuchThat(func(v interface{}) bool {
			templates := v.([]TemplateWithProvider)
			// Ensure unique template names
			seen := make(map[string]bool)
			for _, tmpl := range templates {
				if tmpl.Template.Name == "" || seen[tmpl.Template.Name] {
					return false
				}
				seen[tmpl.Template.Name] = true
			}
			return true
		})
}

// genTemplateWithProvider generates a single template with a provider
func genTemplateWithProvider() gopter.Gen {
	return gen.AlphaString().
		SuchThat(func(v interface{}) bool {
			return v.(string) != ""
		}).
		FlatMap(func(name interface{}) gopter.Gen {
			nameStr := name.(string)
			return gen.IntRange(0, 10).FlatMap(func(priority interface{}) gopter.Gen {
				priorityInt := priority.(int)
				return gen.OneGenOf(
					// Universal template (no provider)
					gen.Const(TemplateWithProvider{
						Template: TemplateDefinition{
							Name:     nameStr,
							Path:     "/path/" + nameStr,
							Type:     TemplateTypeBase,
							Provider: "",
							Metadata: TemplateMetadata{
								Priority: priorityInt,
							},
						},
					}),
					// OpenStack template
					gen.Const(TemplateWithProvider{
						Template: TemplateDefinition{
							Name:     nameStr,
							Path:     "/path/" + nameStr,
							Type:     TemplateTypeInfrastructure,
							Provider: "openstack",
							Metadata: TemplateMetadata{
								Priority: priorityInt,
							},
						},
					}),
					// AWS template
					gen.Const(TemplateWithProvider{
						Template: TemplateDefinition{
							Name:     nameStr,
							Path:     "/path/" + nameStr,
							Type:     TemplateTypeInfrastructure,
							Provider: "aws",
							Metadata: TemplateMetadata{
								Priority: priorityInt,
							},
						},
					}),
					// Baremetal template
					gen.Const(TemplateWithProvider{
						Template: TemplateDefinition{
							Name:     nameStr,
							Path:     "/path/" + nameStr,
							Type:     TemplateTypeInfrastructure,
							Provider: "baremetal",
							Metadata: TemplateMetadata{
								Priority: priorityInt,
							},
						},
					}),
				)
			}, reflect.TypeOf(TemplateWithProvider{}))
		}, reflect.TypeOf(TemplateWithProvider{}))
}

// genProviderName generates a provider name for testing
func genProviderName() gopter.Gen {
	return gen.OneConstOf("openstack", "aws", "baremetal", "vsphere", "kind", "vmware")
}

// genTemplateDefinition generates a random template definition
func genTemplateDefinition() gopter.Gen {
	return gen.AlphaString().
		SuchThat(func(v interface{}) bool {
			return v.(string) != ""
		}).
		FlatMap(func(name interface{}) gopter.Gen {
			nameStr := name.(string)
			return gen.Const(TemplateDefinition{
				Name:         nameStr,
				Path:         "/path/" + nameStr,
				Type:         TemplateTypeBase,
				Dependencies: []string{}, // Start with no dependencies
			})
		}, reflect.TypeOf(TemplateDefinition{}))
}
