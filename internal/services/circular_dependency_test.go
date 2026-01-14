package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircularDependencyDetection_Comprehensive tests various circular dependency scenarios
func TestCircularDependencyDetection_Comprehensive(t *testing.T) {
	tests := []struct {
		name        string
		services    []ServiceDefinition
		resolve     []string
		expectError bool
		errorMsg    string
		description string
	}{
		{
			name: "simple two-node cycle",
			services: []ServiceDefinition{
				{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B"}},
				{Name: "B", Type: ServiceTypeCore, Dependencies: []string{"A"}},
			},
			resolve:     []string{"A"},
			expectError: true,
			errorMsg:    "circular dependency",
			description: "A -> B -> A forms a simple cycle",
		},
		{
			name: "three-node cycle",
			services: []ServiceDefinition{
				{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B"}},
				{Name: "B", Type: ServiceTypeCore, Dependencies: []string{"C"}},
				{Name: "C", Type: ServiceTypeCore, Dependencies: []string{"A"}},
			},
			resolve:     []string{"A"},
			expectError: true,
			errorMsg:    "circular dependency",
			description: "A -> B -> C -> A forms a three-node cycle",
		},
		{
			name: "complex graph with cycle",
			services: []ServiceDefinition{
				{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B", "C"}},
				{Name: "B", Type: ServiceTypeCore, Dependencies: []string{"D"}},
				{Name: "C", Type: ServiceTypeCore, Dependencies: []string{"D"}},
				{Name: "D", Type: ServiceTypeCore, Dependencies: []string{"E"}},
				{Name: "E", Type: ServiceTypeCore, Dependencies: []string{"B"}}, // Creates cycle: B -> D -> E -> B
			},
			resolve:     []string{"A"},
			expectError: true,
			errorMsg:    "circular dependency",
			description: "Complex graph with cycle in sub-dependencies",
		},
		{
			name: "valid complex graph without cycle",
			services: []ServiceDefinition{
				{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B", "C"}},
				{Name: "B", Type: ServiceTypeCore, Dependencies: []string{"D"}},
				{Name: "C", Type: ServiceTypeCore, Dependencies: []string{"D"}},
				{Name: "D", Type: ServiceTypeCore, Dependencies: []string{"E"}},
				{Name: "E", Type: ServiceTypeCore, Dependencies: []string{}},
			},
			resolve:     []string{"A"},
			expectError: false,
			description: "Complex graph with diamond pattern but no cycles",
		},
		{
			name: "cycle not in requested path",
			services: []ServiceDefinition{
				{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B"}},
				{Name: "B", Type: ServiceTypeCore, Dependencies: []string{}},
				{Name: "X", Type: ServiceTypeCore, Dependencies: []string{"Y"}},
				{Name: "Y", Type: ServiceTypeCore, Dependencies: []string{"X"}}, // Cycle in X-Y
			},
			resolve:     []string{"A"},
			expectError: false,
			description: "Cycle exists but not in the requested dependency path",
		},
		{
			name: "multiple independent cycles",
			services: []ServiceDefinition{
				{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B"}},
				{Name: "B", Type: ServiceTypeCore, Dependencies: []string{"A"}},
				{Name: "X", Type: ServiceTypeCore, Dependencies: []string{"Y"}},
				{Name: "Y", Type: ServiceTypeCore, Dependencies: []string{"X"}},
			},
			resolve:     []string{"A", "X"},
			expectError: true,
			errorMsg:    "circular dependency",
			description: "Multiple independent cycles in the graph",
		},
		{
			name: "long cycle chain",
			services: []ServiceDefinition{
				{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B"}},
				{Name: "B", Type: ServiceTypeCore, Dependencies: []string{"C"}},
				{Name: "C", Type: ServiceTypeCore, Dependencies: []string{"D"}},
				{Name: "D", Type: ServiceTypeCore, Dependencies: []string{"E"}},
				{Name: "E", Type: ServiceTypeCore, Dependencies: []string{"F"}},
				{Name: "F", Type: ServiceTypeCore, Dependencies: []string{"A"}}, // Long cycle back to A
			},
			resolve:     []string{"A"},
			expectError: true,
			errorMsg:    "circular dependency",
			description: "Long chain that cycles back to the start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()

			// Register all services
			for _, service := range tt.services {
				err := registry.RegisterService(service)
				require.NoError(t, err, "Failed to register service %s", service.Name)
			}

			// Attempt to resolve dependencies
			resolved, err := registry.ResolveDependencies(tt.resolve)

			if tt.expectError {
				assert.Error(t, err, "Expected error for: %s", tt.description)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain '%s'", tt.errorMsg)
				}
				t.Logf("✓ Correctly detected circular dependency: %s", tt.description)
			} else {
				assert.NoError(t, err, "Should not error for: %s", tt.description)
				assert.NotNil(t, resolved, "Resolved services should not be nil")
				t.Logf("✓ Correctly resolved dependencies: %s", tt.description)
			}
		})
	}
}

// TestCircularDependencyErrorMessage verifies error messages are informative
func TestCircularDependencyErrorMessage(t *testing.T) {
	registry := NewServiceRegistry()

	// Create a simple cycle
	services := []ServiceDefinition{
		{Name: "service-a", Type: ServiceTypeCore, Dependencies: []string{"service-b"}},
		{Name: "service-b", Type: ServiceTypeCore, Dependencies: []string{"service-c"}},
		{Name: "service-c", Type: ServiceTypeCore, Dependencies: []string{"service-a"}},
	}

	for _, service := range services {
		err := registry.RegisterService(service)
		require.NoError(t, err)
	}

	// Attempt to resolve
	_, err := registry.ResolveDependencies([]string{"service-a"})
	require.Error(t, err)

	// Verify error message contains useful information
	assert.Contains(t, err.Error(), "circular dependency detected")
	
	// The error should show the cycle path
	errMsg := err.Error()
	t.Logf("Error message: %s", errMsg)
	
	// Verify the cycle is shown in the error
	assert.Contains(t, errMsg, "[")
	assert.Contains(t, errMsg, "]")
}

// TestValidateDependencies_CircularDetection tests ValidateDependencies method
func TestValidateDependencies_CircularDetection(t *testing.T) {
	registry := NewServiceRegistry()

	// Create services with circular dependency
	services := []ServiceDefinition{
		{Name: "monitoring", Type: ServiceTypeMonitoring, Dependencies: []string{"logging"}},
		{Name: "logging", Type: ServiceTypeLogging, Dependencies: []string{"storage"}},
		{Name: "storage", Type: ServiceTypeStorage, Dependencies: []string{"monitoring"}},
	}

	for _, service := range services {
		err := registry.RegisterService(service)
		require.NoError(t, err)
	}

	// ValidateDependencies should detect the cycle
	err := registry.ValidateDependencies([]string{"monitoring"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

// TestSelfDependencyPrevention verifies self-dependencies are caught at registration
func TestSelfDependencyPrevention(t *testing.T) {
	registry := NewServiceRegistry()

	service := ServiceDefinition{
		Name:         "self-referencing",
		Type:         ServiceTypeCore,
		Dependencies: []string{"self-referencing"},
	}

	err := registry.RegisterService(service)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot depend on itself")
}

// TestDependencyResolutionOrder verifies correct topological ordering
func TestDependencyResolutionOrder(t *testing.T) {
	registry := NewServiceRegistry()

	// Create a dependency graph: A -> B -> C, A -> D -> C
	services := []ServiceDefinition{
		{Name: "C", Type: ServiceTypeCore, Dependencies: []string{}},
		{Name: "B", Type: ServiceTypeCore, Dependencies: []string{"C"}},
		{Name: "D", Type: ServiceTypeCore, Dependencies: []string{"C"}},
		{Name: "A", Type: ServiceTypeCore, Dependencies: []string{"B", "D"}},
	}

	for _, service := range services {
		err := registry.RegisterService(service)
		require.NoError(t, err)
	}

	resolved, err := registry.ResolveDependencies([]string{"A"})
	require.NoError(t, err)
	require.Len(t, resolved, 4)

	// Verify topological order: C must come before B and D, B and D must come before A
	positions := make(map[string]int)
	for i, service := range resolved {
		positions[service.Name] = i
	}

	assert.Less(t, positions["C"], positions["B"], "C should come before B")
	assert.Less(t, positions["C"], positions["D"], "C should come before D")
	assert.Less(t, positions["B"], positions["A"], "B should come before A")
	assert.Less(t, positions["D"], positions["A"], "D should come before A")

	t.Logf("Resolved order: %v", getServiceNames(resolved))
}

// Helper function to get service names from definitions
func getServiceNames(services []ServiceDefinition) []string {
	names := make([]string, len(services))
	for i, service := range services {
		names[i] = service.Name
	}
	return names
}
