package services_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rackerlabs/openCenter-cli/internal/services"
)

// Example demonstrates the basic usage of the service plugin system
func Example() {
	// Create a temporary directory for manifests
	tmpDir, _ := os.MkdirTemp("", "services-example")
	defer os.RemoveAll(tmpDir)

	// Create a simple service manifest
	manifest := `name: example-service
version: 1.0.0
type: monitoring
description: Example monitoring service
`
	manifestPath := filepath.Join(tmpDir, "example.yaml")
	os.WriteFile(manifestPath, []byte(manifest), 0644)

	// Create a service registry
	registry := services.NewServiceRegistry()

	// Load manifests from directory
	err := registry.LoadManifestsFromDirectory(tmpDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// List all loaded services
	serviceList := registry.ListServices()
	for _, svc := range serviceList {
		fmt.Printf("Loaded: %s v%s (%s)\n", svc.Name, svc.Version, svc.Type)
	}

	// Output:
	// Loaded: example-service v1.0.0 (monitoring)
}

// Example_dependencyResolution demonstrates dependency resolution
func Example_dependencyResolution() {
	tmpDir, _ := os.MkdirTemp("", "services-deps")
	defer os.RemoveAll(tmpDir)

	// Create services with dependencies
	manifests := map[string]string{
		"core.yaml": `name: core
version: 1.0.0
type: core
`,
		"storage.yaml": `name: storage
version: 1.0.0
type: storage
dependencies:
  - core
`,
		"monitoring.yaml": `name: monitoring
version: 1.0.0
type: monitoring
dependencies:
  - core
  - storage
`,
	}

	for filename, content := range manifests {
		path := filepath.Join(tmpDir, filename)
		os.WriteFile(path, []byte(content), 0644)
	}

	registry := services.NewServiceRegistry()
	registry.LoadManifestsFromDirectory(tmpDir)

	// Resolve dependencies for monitoring service
	resolved, err := registry.ResolveDependencies([]string{"monitoring"})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print services in dependency order
	fmt.Println("Installation order:")
	for i, svc := range resolved {
		fmt.Printf("%d. %s\n", i+1, svc.Name)
	}

	// Output:
	// Installation order:
	// 1. core
	// 2. storage
	// 3. monitoring
}

// Example_validation demonstrates dependency validation
func Example_validation() {
	tmpDir, _ := os.MkdirTemp("", "services-validation")
	defer os.RemoveAll(tmpDir)

	// Create a service with a missing dependency
	manifest := `name: app
version: 1.0.0
type: custom
dependencies:
  - missing-service
`
	manifestPath := filepath.Join(tmpDir, "app.yaml")
	os.WriteFile(manifestPath, []byte(manifest), 0644)

	registry := services.NewServiceRegistry()
	registry.LoadManifestsFromDirectory(tmpDir)

	// Try to validate dependencies
	err := registry.ValidateDependencies([]string{"app"})
	if err != nil {
		fmt.Println("Validation failed: missing dependency")
	}

	// Output:
	// Validation failed: missing dependency
}
