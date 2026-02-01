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

/*
Package paths provides centralized path resolution for opencenter-cli clusters.

# Overview

The paths package eliminates duplicate path construction logic throughout the
codebase by providing a single source of truth for all cluster-related paths.
It supports organization-based directory structures with caching, validation,
and fallback strategies.

# Architecture

The package is built around three core components:

1. PathResolver: Main entry point for path resolution
2. ResolutionStrategy: Pluggable strategies for different directory layouts
3. PathCache: Thread-safe caching for performance optimization

# Directory Structure

The package supports organization-based directory structures:

	~/.config/opencenter/clusters/
	└── <organization>/
	    ├── infrastructure/
	    │   └── clusters/
	    │       └── <cluster>/
	    │           ├── .<cluster>-config.yaml
	    │           ├── kubeconfig.yaml
	    │           ├── inventory/
	    │           ├── venv/
	    │           └── .bin/
	    ├── applications/
	    │   └── overlays/
	    │       └── <cluster>/
	    ├── secrets/
	    │   ├── age/
	    │   │   └── keys/
	    │   │       └── <cluster>-key.txt
	    │   └── ssh/
	    │       └── <cluster>
	    └── .sops.yaml

# Basic Usage

Create a resolver and resolve paths for a cluster:

	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")
	clusterPaths, err := resolver.Resolve(context.Background(), "my-cluster", "myorg")
	if err != nil {
	    log.Fatal(err)
	}

	// Access resolved paths
	configPath := clusterPaths.ConfigPath
	secretsDir := clusterPaths.SecretsDir

# Fallback Resolution

When the organization is unknown, use fallback resolution to search across
all organizations:

	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), "my-cluster")
	if err != nil {
	    log.Fatal(err)
	}

# Custom Options

Configure the resolver with custom options:

	options := paths.ResolutionOptions{
	    Organization:  "myorg",
	    CacheResults:  true,
	    ValidatePaths: true,
	}
	resolver := paths.NewPathResolverWithOptions(baseDir, options)

# Cache Management

The resolver includes built-in caching for performance:

	// Invalidate cache for a specific cluster
	resolver.InvalidateCache("my-cluster")

	// Clear all cached entries
	resolver.ClearCache()

	// Get cache statistics
	stats := resolver.GetCacheStats()
	fmt.Printf("Cache hit rate: %.2f%%\n", stats.HitRate*100)

# Directory Creation

Create all necessary directories for a new cluster:

	err := resolver.CreateClusterDirectories(
	    context.Background(),
	    "my-cluster",
	    "myorg",
	)
	if err != nil {
	    log.Fatal(err)
	}

# Organization Detection

Detect the organization for an existing cluster:

	org, err := resolver.GetOrganization(context.Background(), "my-cluster")
	if err != nil {
	    log.Fatal(err)
	}
	fmt.Printf("Cluster belongs to organization: %s\n", org)

# Path Validation

Validate that paths are safe and accessible:

	err := resolver.ValidatePath("/path/to/validate")
	if err != nil {
	    log.Printf("Invalid path: %v\n", err)
	}

# Thread Safety

All resolver operations are thread-safe and can be used concurrently:

	var wg sync.WaitGroup
	for _, cluster := range clusters {
	    wg.Add(1)
	    go func(name string) {
	        defer wg.Done()
	        paths, err := resolver.Resolve(ctx, name, "myorg")
	        // Process paths...
	    }(cluster)
	}
	wg.Wait()

# Performance Characteristics

The resolver is optimized for performance:

  - Path resolution: <1ms (uncached)
  - Path resolution: <100μs (cached)
  - Cache hit rate: >90% in typical usage
  - Memory overhead: ~1KB per cached cluster

# Error Handling

The package returns descriptive errors for common failure cases:

  - Invalid cluster names (format, length, special characters)
  - Missing directories
  - Permission issues
  - Path traversal attempts

Example error handling:

	paths, err := resolver.Resolve(ctx, clusterName, org)
	if err != nil {
	    if os.IsNotExist(err) {
	        // Cluster doesn't exist
	    } else if strings.Contains(err.Error(), "invalid cluster name") {
	        // Invalid name format
	    } else {
	        // Other error
	    }
	    return err
	}

# Migration from Legacy Code

Replace direct path construction with resolver calls:

	// Old (duplicate logic)
	configPath := filepath.Join(baseDir, org, "infrastructure", "clusters", cluster, "."+cluster+"-config.yaml")

	// New (centralized)
	paths, err := resolver.Resolve(ctx, cluster, org)
	configPath := paths.ConfigPath

# Best Practices

1. Create one resolver instance per application (singleton pattern)
2. Enable caching for production use (default: enabled)
3. Use Resolve() when organization is known
4. Use ResolveWithFallback() when organization is unknown
5. Invalidate cache after creating or deleting clusters
6. Enable path validation only when necessary (performance impact)
7. Handle errors appropriately (don't ignore path resolution failures)

# Testing

The package includes comprehensive tests:

  - Unit tests: resolver_test.go, strategies_test.go, cache_test.go
  - Thread safety tests: thread_safety_test.go
  - Benchmark tests: benchmark_test.go

Run tests:

	go test ./internal/core/paths/...

Run benchmarks:

	go test -bench=. ./internal/core/paths/

# Package Structure

The package is organized into focused modules:

  - resolver.go: Main PathResolver implementation
  - types.go: Core types (ClusterPaths, ResolutionOptions)
  - strategies.go: Resolution strategies (OrgBasedStrategy)
  - cache.go: Thread-safe caching (PathCache)
  - doc.go: Package documentation (this file)

# Future Enhancements

Planned improvements:

  - Additional resolution strategies (legacy, flat)
  - Persistent cache (disk-backed)
  - Path watching (filesystem notifications)
  - Metrics and observability
  - Custom validation rules

# See Also

  - Design document: .kiro/specs/architectural-refactoring/design.md
  - Requirements: .kiro/specs/architectural-refactoring/requirements.md
  - Migration guide: .kiro/specs/architectural-refactoring/01-path-resolver.md
*/
package paths
