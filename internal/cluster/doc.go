// Package cluster provides domain services for cluster lifecycle management.
//
// This package contains business logic for cluster operations, separated from
// the CLI layer to enable testability and reusability. Each service handles
// a specific aspect of cluster management:
//
//   - InitService: Cluster initialization and configuration creation
//   - ValidateService: Configuration and connectivity validation
//   - SetupService: GitOps repository generation and manifest creation
//   - BootstrapService: Infrastructure provisioning and cluster deployment
//
// Services use dependency injection to receive their dependencies (PathResolver,
// ConfigManager, ValidationEngine) and have no direct dependencies on the CLI
// framework (Cobra), making them easy to test and reuse in other contexts.
//
// Example usage:
//
//	// Create service with dependencies
//	initService := cluster.NewInitService(pathResolver, configManager, validationEngine)
//
//	// Initialize a cluster
//	result, err := initService.Initialize(ctx, cluster.InitOptions{
//		ClusterName:  "my-cluster",
//		Organization: "my-org",
//		Provider:     "openstack",
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Config created at: %s\n", result.ConfigPath)
//
// Design Principles:
//
//   - Single Responsibility: Each service handles one aspect of cluster lifecycle
//   - Dependency Injection: Services receive dependencies via constructor
//   - Context-Aware: All operations accept context.Context for cancellation
//   - Error Wrapping: Errors are wrapped with context using fmt.Errorf
//   - Testability: No CLI dependencies, easy to mock dependencies
package cluster
