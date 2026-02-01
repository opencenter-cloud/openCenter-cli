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

package paths_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
)

// Example_basicUsage demonstrates basic path resolution.
func Example_basicUsage() {
	// Create a resolver with default options
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// Resolve paths for a cluster
	clusterPaths, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		log.Fatal(err)
	}

	// Access resolved paths
	fmt.Println("Config path:", clusterPaths.ConfigPath)
	fmt.Println("Secrets dir:", clusterPaths.SecretsDir)
	fmt.Println("GitOps dir:", clusterPaths.GitOpsDir)
}

// Example_fallbackResolution demonstrates fallback resolution when organization is unknown.
func Example_fallbackResolution() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// Search for cluster across all organizations
	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), "my-cluster")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found cluster in organization: %s\n", filepath.Base(clusterPaths.OrganizationDir))
}

// Example_customOptions demonstrates using custom resolution options.
func Example_customOptions() {
	// Configure custom options
	options := paths.ResolutionOptions{
		Organization:  "myorg",
		CacheResults:  true,
		ValidatePaths: true, // Enable path validation
	}

	resolver := paths.NewPathResolverWithOptions(
		"~/.config/opencenter/clusters",
		options,
	)

	clusterPaths, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Resolved with validation:", clusterPaths.ConfigPath)
}

// Example_cacheManagement demonstrates cache operations.
func Example_cacheManagement() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// First resolution (cache miss)
	_, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		log.Fatal(err)
	}

	// Second resolution (cache hit)
	_, err = resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		log.Fatal(err)
	}

	// Get cache statistics
	stats := resolver.GetCacheStats()
	fmt.Printf("Cache hit rate: %.2f%%\n", stats.HitRate*100)
	fmt.Printf("Total entries: %d\n", stats.Entries)

	// Invalidate cache for a specific cluster
	resolver.InvalidateCache("my-cluster")

	// Clear all cache entries
	resolver.ClearCache()
}

// Example_directoryCreation demonstrates creating cluster directories.
func Example_directoryCreation() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// Create all necessary directories for a new cluster
	err := resolver.CreateClusterDirectories(
		context.Background(),
		"new-cluster",
		"myorg",
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Created all cluster directories")
}

// Example_organizationDetection demonstrates detecting cluster organization.
func Example_organizationDetection() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// Detect organization for an existing cluster
	org, err := resolver.GetOrganization(context.Background(), "my-cluster")
	if err != nil {
		log.Fatal(err)
	}

	if org == "" {
		fmt.Println("Cluster uses legacy structure")
	} else {
		fmt.Printf("Cluster belongs to organization: %s\n", org)
	}
}

// Example_pathValidation demonstrates path validation.
func Example_pathValidation() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// Validate a safe path
	err := resolver.ValidatePath("/home/user/.config/opencenter/clusters/myorg")
	if err != nil {
		fmt.Println("Invalid path:", err)
	} else {
		fmt.Println("Path is valid")
	}

	// Validate a path with traversal attempt
	err = resolver.ValidatePath("/home/user/../../../etc/passwd")
	if err != nil {
		fmt.Println("Invalid path:", err)
	}
}

// Example_structureDetection demonstrates detecting directory structure type.
func Example_structureDetection() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// Detect structure type for a cluster
	structureType, err := resolver.DetectStructureType(context.Background(), "my-cluster")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Structure type: %s\n", structureType)
}

// Example_errorHandling demonstrates proper error handling.
func Example_errorHandling() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	clusterPaths, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		// Check for specific error types
		if os.IsNotExist(err) {
			fmt.Println("Cluster directory does not exist")
			return
		}

		// Check for validation errors
		if err.Error() == "invalid cluster name" {
			fmt.Println("Cluster name format is invalid")
			return
		}

		// Other errors
		log.Fatal(err)
	}

	fmt.Println("Successfully resolved:", clusterPaths.ConfigPath)
}

// Example_concurrentUsage demonstrates thread-safe concurrent usage.
func Example_concurrentUsage() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	clusters := []string{"cluster1", "cluster2", "cluster3"}
	results := make(chan *paths.ClusterPaths, len(clusters))

	// Resolve paths concurrently
	for _, cluster := range clusters {
		go func(name string) {
			clusterPaths, err := resolver.Resolve(context.Background(), name, "myorg")
			if err != nil {
				log.Printf("Failed to resolve %s: %v", name, err)
				return
			}
			results <- clusterPaths
		}(cluster)
	}

	// Collect results
	for i := 0; i < len(clusters); i++ {
		clusterPaths := <-results
		fmt.Printf("Resolved: %s\n", clusterPaths.ClusterDir)
	}
}

// Example_migrationFromLegacy demonstrates migrating from legacy path construction.
func Example_migrationFromLegacy() {
	// Old approach (duplicate logic, error-prone)
	oldConfigPath := func(baseDir, org, cluster string) string {
		return filepath.Join(
			baseDir,
			org,
			"infrastructure",
			"clusters",
			cluster,
			"."+cluster+"-config.yaml",
		)
	}

	// New approach (centralized, cached, validated)
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")
	clusterPaths, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		log.Fatal(err)
	}

	// Compare paths
	oldPath := oldConfigPath("~/.config/opencenter/clusters", "myorg", "my-cluster")
	newPath := clusterPaths.ConfigPath

	fmt.Println("Old path:", oldPath)
	fmt.Println("New path:", newPath)
}

// Example_allPathTypes demonstrates accessing all path types.
func Example_allPathTypes() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	clusterPaths, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		log.Fatal(err)
	}

	// Access all available paths
	fmt.Println("Organization Dir:", clusterPaths.OrganizationDir)
	fmt.Println("GitOps Dir:", clusterPaths.GitOpsDir)
	fmt.Println("Cluster Dir:", clusterPaths.ClusterDir)
	fmt.Println("Applications Dir:", clusterPaths.ApplicationsDir)
	fmt.Println("Secrets Dir:", clusterPaths.SecretsDir)
	fmt.Println("SOPS Key Path:", clusterPaths.SOPSKeyPath)
	fmt.Println("SOPS Config Path:", clusterPaths.SOPSConfigPath)
	fmt.Println("Kubeconfig Path:", clusterPaths.KubeconfigPath)
	fmt.Println("Inventory Path:", clusterPaths.InventoryPath)
	fmt.Println("Venv Path:", clusterPaths.VenvPath)
	fmt.Println("Bin Path:", clusterPaths.BinPath)
	fmt.Println("Config Path:", clusterPaths.ConfigPath)
	fmt.Println("SSH Key Path:", clusterPaths.SSHKeyPath)
}

// Example_singletonPattern demonstrates the recommended singleton pattern.
func Example_singletonPattern() {
	// Create a single resolver instance for the application
	var (
		resolverInstance *paths.PathResolver
		resolverOnce     = &struct{}{}
	)

	getResolver := func() *paths.PathResolver {
		// In real code, use sync.Once for thread-safe initialization
		if resolverInstance == nil {
			resolverInstance = paths.NewPathResolver("~/.config/opencenter/clusters")
		}
		return resolverInstance
	}

	// Use the singleton throughout the application
	resolver := getResolver()
	clusterPaths, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Config path:", clusterPaths.ConfigPath)
	_ = resolverOnce // Suppress unused warning
}

// Example_contextCancellation demonstrates context cancellation support.
func Example_contextCancellation() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate cancellation
	go func() {
		// In real code, this would be triggered by user action or timeout
		cancel()
	}()

	// Resolve with cancellable context
	_, err := resolver.Resolve(ctx, "my-cluster", "myorg")
	if err != nil {
		if ctx.Err() == context.Canceled {
			fmt.Println("Resolution cancelled")
			return
		}
		log.Fatal(err)
	}
}

// Example_batchResolution demonstrates resolving paths for multiple clusters.
func Example_batchResolution() {
	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")

	clusters := []struct {
		name string
		org  string
	}{
		{"cluster1", "org1"},
		{"cluster2", "org1"},
		{"cluster3", "org2"},
	}

	// Resolve all clusters
	for _, cluster := range clusters {
		clusterPaths, err := resolver.Resolve(
			context.Background(),
			cluster.name,
			cluster.org,
		)
		if err != nil {
			log.Printf("Failed to resolve %s: %v", cluster.name, err)
			continue
		}

		fmt.Printf("Resolved %s: %s\n", cluster.name, clusterPaths.ConfigPath)
	}

	// Check cache efficiency
	stats := resolver.GetCacheStats()
	fmt.Printf("Cache hit rate: %.2f%%\n", stats.HitRate*100)
}
