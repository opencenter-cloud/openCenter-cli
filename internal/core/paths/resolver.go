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

package paths

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PathResolver manages dynamic path resolution with organization support.
// It provides a single source of truth for all cluster path resolution,
// with caching and fallback strategies for backward compatibility.
//
// PathResolver is thread-safe and can be used concurrently from multiple goroutines.
// It uses a read-write mutex to protect internal state and a thread-safe cache
// for resolved paths.
//
// Example usage:
//
//	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")
//	paths, err := resolver.Resolve(ctx, "my-cluster", "my-org")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Config path:", paths.ConfigPath)
//
// For clusters without a known organization, use ResolveWithFallback:
//
//	paths, err := resolver.ResolveWithFallback(ctx, "my-cluster")
//
// The resolver automatically caches results for performance. To invalidate:
//
//	resolver.InvalidateCache("my-cluster")
type PathResolver struct {
	// baseDir is the base directory for all clusters
	baseDir string

	// roots contains the secure zone roots for GitOps, cluster state, and secrets.
	roots PathRoots

	// strategies contains all resolution strategies, sorted by priority
	strategies []ResolutionStrategy

	// cache provides thread-safe caching of resolved paths
	cache *PathCache

	// mu protects concurrent access to the resolver
	mu sync.RWMutex

	// options contains resolution options
	options ResolutionOptions
}

// NewPathResolver creates a new path resolver with the given base directory.
//
// The base directory is the root directory containing organization subdirectories.
// Typically this is "~/.config/opencenter/clusters".
//
// The resolver is created with default options:
//   - Organization: "opencenter"
//   - CacheResults: true
//   - ValidatePaths: false
//
// Example:
//
//	resolver := paths.NewPathResolver("~/.config/opencenter/clusters")
//
// For custom options, use NewPathResolverWithOptions.
func NewPathResolver(baseDir string) *PathResolver {
	return NewPathResolverWithOptions(baseDir, DefaultResolutionOptions())
}

// NewPathResolverWithOptions creates a new path resolver with custom options.
//
// This constructor allows fine-grained control over resolver behavior:
//   - Organization: Default organization name when not specified
//   - CacheResults: Enable/disable result caching
//   - ValidatePaths: Enable/disable path validation (expensive)
//
// Example:
//
//	opts := paths.ResolutionOptions{
//	    Organization: "my-company",
//	    CacheResults: true,
//	    ValidatePaths: true,
//	}
//	resolver := paths.NewPathResolverWithOptions("~/.config/opencenter/clusters", opts)
func NewPathResolverWithOptions(baseDir string, options ResolutionOptions) *PathResolver {
	roots := DefaultPathRoots(baseDir)
	return NewPathResolverWithRoots(roots.ClustersDir, roots.GitOpsDir, roots.ClusterStateDir, roots.SecretsDir, options)
}

// NewPathResolverWithRoots creates a resolver with explicit secure zone roots.
func NewPathResolverWithRoots(baseDir, gitopsRoot, clusterStateRoot, secretsRoot string, options ResolutionOptions) *PathResolver {
	roots := expandPathRoots(PathRoots{
		ClustersDir:     baseDir,
		GitOpsDir:       gitopsRoot,
		ClusterStateDir: clusterStateRoot,
		SecretsDir:      secretsRoot,
	})

	// Create organization-based strategy only
	strategy := NewOrgBasedStrategyWithRoots(roots)

	// Create cache if enabled
	var cache *PathCache
	if options.CacheResults {
		cache = DefaultPathCache()
	}

	return &PathResolver{
		baseDir:    roots.ClustersDir,
		roots:      roots,
		strategies: []ResolutionStrategy{strategy},
		cache:      cache,
		options:    options,
	}
}

// Resolve resolves all paths for the given cluster and organization.
// This is the primary method for path resolution.
//
// The method performs the following steps:
//  1. Validates cluster name and organization name
//  2. Checks cache for existing resolution (if caching enabled)
//  3. Uses organization-based strategy to resolve paths
//  4. Caches the result for future calls
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - clusterName: Name of the cluster (must be valid DNS label)
//   - organization: Organization name (uses default if empty)
//
// Returns:
//   - *ClusterPaths: Resolved paths for the cluster
//   - error: Validation error or resolution failure
//
// Example:
//
//	paths, err := resolver.Resolve(ctx, "prod-cluster", "acme-corp")
//	if err != nil {
//	    return fmt.Errorf("failed to resolve paths: %w", err)
//	}
//	// Use paths.ConfigPath, paths.SecretsDir, etc.
func (r *PathResolver) Resolve(ctx context.Context, clusterName, organization string) (*ClusterPaths, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// Validate cluster name (fast path - no allocations for valid names)
	if err := r.validateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Use default organization if not specified
	r.mu.RLock()
	if organization == "" {
		organization = r.options.Organization
	}
	r.mu.RUnlock()

	// Validate organization name
	if err := r.validateClusterName(organization); err != nil {
		return nil, fmt.Errorf("invalid organization name: %w", err)
	}

	// Check cache first (fast path)
	if r.cache != nil {
		if paths := r.cache.Get(clusterName, organization); paths != nil {
			return paths, nil
		}
	}

	// Use organization-based strategy (slow path)
	r.mu.RLock()
	strategy := r.strategies[0]
	r.mu.RUnlock()

	canResolve, err := strategy.CanResolve(ctx, clusterName, organization)
	if err != nil {
		return nil, fmt.Errorf("failed to check if cluster exists: %w", err)
	}

	if !canResolve {
		return nil, fmt.Errorf("cluster %s not found in organization %s", clusterName, organization)
	}

	paths, err := strategy.Resolve(ctx, clusterName, organization)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve paths: %w", err)
	}

	// Cache the result
	if r.cache != nil {
		r.cache.Set(clusterName, organization, strategy.Name(), paths)
	}

	return paths, nil
}

// ResolveWithFallback resolves paths for a cluster without knowing its organization.
// It searches only the secure cluster-state zone. Legacy org-root layouts are
// not a compatibility path.
//
// The search process:
//  1. Checks cache first
//  2. Scans all organization directories in baseDir
//  3. Returns paths for the first matching cluster found
//  4. Caches the result with empty organization key
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - clusterName: Name of the cluster to find
//
// Returns:
//   - *ClusterPaths: Resolved paths for the cluster
//   - error: Cluster not found or validation error
//
// Example:
//
//	// When you don't know the organization
//	paths, err := resolver.ResolveWithFallback(ctx, "my-cluster")
//	if err != nil {
//	    return fmt.Errorf("cluster not found: %w", err)
//	}
func (r *PathResolver) ResolveWithFallback(ctx context.Context, clusterName string) (*ClusterPaths, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// Validate cluster name
	if err := r.validateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Check cache first (with empty organization for fallback)
	if r.cache != nil {
		if paths := r.cache.Get(clusterName, ""); paths != nil {
			return paths, nil
		}
	}

	// Search for cluster in all organization directories under the state root.
	r.mu.RLock()
	stateRoot := r.roots.ClusterStateDir
	r.mu.RUnlock()

	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		if os.IsNotExist(err) {
			if legacyErr := r.detectLegacyLayoutForCluster(clusterName); legacyErr != nil {
				return nil, legacyErr
			}
			return nil, fmt.Errorf("cluster %s not found in any organization (cluster state directory does not exist: %s)", clusterName, stateRoot)
		}
		return nil, fmt.Errorf("failed to read cluster state directory: %w", err)
	}

	var matches []*ClusterPaths
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		orgName := entry.Name()
		stateDir := filepath.Join(stateRoot, orgName, clusterName)
		configFile := filepath.Join(stateDir, clusterName+"-config.yaml")
		if _, err := os.Stat(stateDir); err == nil {
			paths, err := r.Resolve(ctx, clusterName, orgName)
			if err == nil {
				matches = append(matches, paths)
				continue
			}
			if _, ok := err.(*LegacyLayoutError); ok {
				return nil, err
			}
		} else if _, err := os.Stat(configFile); err == nil {
			paths, err := r.Resolve(ctx, clusterName, orgName)
			if err == nil {
				matches = append(matches, paths)
				continue
			}
		}
	}

	if legacyErr := r.detectLegacyLayoutForCluster(clusterName); legacyErr != nil {
		return nil, legacyErr
	}

	if len(matches) > 1 {
		var orgs []string
		for _, match := range matches {
			orgs = append(orgs, filepath.Base(match.GitOpsDir))
		}
		return nil, fmt.Errorf("cluster %s found in multiple organizations: %s", clusterName, strings.Join(orgs, ", "))
	}
	if len(matches) == 1 {
		if r.cache != nil {
			r.cache.Set(clusterName, "", "state-search", matches[0])
		}
		return matches[0], nil
	}
	return nil, fmt.Errorf("cluster %s not found in any organization", clusterName)
}

// InvalidateCache invalidates the cache for a specific cluster.
//
// Call this method after:
//   - Creating new cluster directories
//   - Moving a cluster to a different organization
//   - Modifying cluster structure
//
// Example:
//
//	resolver.CreateClusterDirectories(ctx, "new-cluster", "my-org")
//	resolver.InvalidateCache("new-cluster") // Force fresh resolution
func (r *PathResolver) InvalidateCache(clusterName string) {
	if r.cache != nil {
		r.cache.InvalidateCluster(clusterName)
	}
}

// ClearCache clears all cached path resolutions.
//
// Use this to force fresh resolution for all clusters, typically:
//   - After bulk operations
//   - During testing
//   - When directory structure changes
//
// Example:
//
//	resolver.ClearCache() // Clear all cached paths
func (r *PathResolver) ClearCache() {
	if r.cache != nil {
		r.cache.Clear()
	}
}

// GetCacheStats returns cache statistics for monitoring and debugging.
//
// Returns:
//   - CacheStats: Hit rate, miss rate, and entry count
//
// Example:
//
//	stats := resolver.GetCacheStats()
//	fmt.Printf("Cache hit rate: %.2f%%\n", stats.HitRate()*100)
func (r *PathResolver) GetCacheStats() CacheStats {
	if r.cache != nil {
		return r.cache.Stats()
	}
	return CacheStats{}
}

// DetectStructureType detects the directory structure type for a cluster.
func (r *PathResolver) DetectStructureType(ctx context.Context, clusterName string) (StructureType, error) {
	if err := r.validateClusterName(clusterName); err != nil {
		return StructureTypeUnknown, fmt.Errorf("invalid cluster name: %w", err)
	}

	if _, err := r.ResolveWithFallback(ctx, clusterName); err == nil {
		return StructureTypeOrganization, nil
	} else if _, ok := err.(*LegacyLayoutError); ok {
		return StructureTypeUnknown, err
	}

	return StructureTypeUnknown, nil
}

// GetOrganization determines the organization for a cluster.
// Returns empty string if the cluster cannot be found in the secure layout.
func (r *PathResolver) GetOrganization(ctx context.Context, clusterName string) (string, error) {
	if err := r.validateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	r.mu.RLock()
	stateRoot := r.roots.ClusterStateDir
	r.mu.RUnlock()

	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		if os.IsNotExist(err) {
			if legacyErr := r.detectLegacyLayoutForCluster(clusterName); legacyErr != nil {
				return "", legacyErr
			}
			return "", nil
		}
		return "", err
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		orgName := entry.Name()
		stateDir := filepath.Join(stateRoot, orgName, clusterName)
		if _, err := os.Stat(stateDir); err == nil {
			matches = append(matches, orgName)
		}
	}
	if legacyErr := r.detectLegacyLayoutForCluster(clusterName); legacyErr != nil {
		return "", legacyErr
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("cluster %s found in multiple organizations: %s", clusterName, strings.Join(matches, ", "))
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	return "", nil
}

// CreateClusterDirectories creates all necessary directories for a cluster.
//
// This method creates the complete directory structure required for a cluster:
//   - Organization directory
//   - Infrastructure/clusters/<cluster>
//   - Applications/overlays/<cluster>
//   - Secrets directories (age, ssh)
//   - Inventory and bin directories
//
// All directories are created with 0755 permissions. If ValidatePaths option
// is enabled, write permissions are verified for each directory.
//
// Parameters:
//   - ctx: Context for cancellation
//   - clusterName: Name of the cluster
//   - organization: Organization name (uses default if empty)
//
// Returns:
//   - error: Directory creation or validation failure
//
// Example:
//
//	err := resolver.CreateClusterDirectories(ctx, "new-cluster", "my-org")
//	if err != nil {
//	    return fmt.Errorf("failed to create directories: %w", err)
//	}
func (r *PathResolver) CreateClusterDirectories(ctx context.Context, clusterName, organization string) error {
	if err := r.validateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	r.mu.RLock()
	if organization == "" {
		organization = r.options.Organization
	}
	r.mu.RUnlock()

	if err := r.validateClusterName(organization); err != nil {
		return fmt.Errorf("invalid organization name: %w", err)
	}

	// Resolve paths using organization-based strategy
	// NOTE: We call Resolve() directly on the strategy, bypassing CanResolve() check
	// because we're creating a NEW cluster that doesn't exist yet
	r.mu.RLock()
	strategy := r.strategies[0]
	validatePaths := r.options.ValidatePaths
	r.mu.RUnlock()

	paths, err := strategy.Resolve(ctx, clusterName, organization)
	if err != nil {
		return fmt.Errorf("failed to resolve paths: %w", err)
	}

	// Create all directories with zone-specific baseline permissions. Strict
	// post-write mode checks are handled by init when secrets and state files
	// are created. Order matters: parents must be created before children so
	// that MkdirAll does not silently assign the child's mode to intermediate
	// directories.
	type dirEntry struct {
		path string
		mode os.FileMode
	}
	dirs := []dirEntry{
		{paths.OrganizationDir, 0o755},
		{filepath.Join(paths.OrganizationDir, "infrastructure"), 0o755},
		{filepath.Join(paths.OrganizationDir, "infrastructure", "clusters"), 0o755},
		{paths.ClusterDir, 0o755},
		{filepath.Join(paths.OrganizationDir, "applications"), 0o755},
		{filepath.Join(paths.OrganizationDir, "applications", "overlays"), 0o755},
		{paths.ApplicationsDir, 0o755},
		{paths.ClusterStateDir, 0o700},
		{paths.InventoryPath, 0o700},
		{paths.VenvPath, 0o700},
		{paths.BinPath, 0o700},
		{paths.SecretsDir, 0o700},
		{filepath.Join(paths.SecretsDir, "age"), 0o700},
		{filepath.Dir(paths.SOPSKeyPath), 0o700},
		{filepath.Join(paths.SecretsDir, "ssh"), 0o700},
		{filepath.Dir(paths.SSHKeyPath), 0o700},
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d.path, err)
		}

		// Verify the directory was created
		if stat, err := os.Stat(d.path); err != nil {
			return fmt.Errorf("failed to verify directory %s: %w", d.path, err)
		} else if !stat.IsDir() {
			return fmt.Errorf("path %s exists but is not a directory", d.path)
		}

		// Validate permissions if requested
		if validatePaths {
			if err := r.validateDirectoryPermissions(d.path); err != nil {
				return fmt.Errorf("directory %s has insufficient permissions: %w", d.path, err)
			}
		}
	}

	// Invalidate cache for this cluster
	r.InvalidateCache(clusterName)

	return nil
}

// ValidatePath validates that a path is safe and accessible.
//
// Validation checks:
//   - Path is not empty
//   - No directory traversal attempts (..)
//   - Path is absolute after expansion
//
// Parameters:
//   - path: Path to validate (can contain ~ and environment variables)
//
// Returns:
//   - error: Validation failure with specific reason
//
// Example:
//
//	if err := resolver.ValidatePath("~/.config/opencenter"); err != nil {
//	    return fmt.Errorf("invalid path: %w", err)
//	}
func (r *PathResolver) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Expand the path first
	expandedPath := expandPath(path)

	// Check for path traversal attempts
	if strings.Contains(expandedPath, "..") {
		return fmt.Errorf("path contains directory traversal elements: %s", path)
	}

	// Check if the path is absolute after expansion
	if !filepath.IsAbs(expandedPath) {
		return fmt.Errorf("path must be absolute after expansion: %s", expandedPath)
	}

	return nil
}

// validateClusterName validates a cluster or organization name.
func (r *PathResolver) validateClusterName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	// Check length
	if len(name) > 63 {
		return fmt.Errorf("name must be 63 characters or less")
	}

	// Check format (alphanumeric, hyphens, underscores)
	for i, c := range name {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_') {
			return fmt.Errorf("name contains invalid character at position %d: %c", i, c)
		}
	}

	// Check that it doesn't start or end with hyphen or underscore
	if name[0] == '-' || name[0] == '_' || name[len(name)-1] == '-' || name[len(name)-1] == '_' {
		return fmt.Errorf("name cannot start or end with hyphen or underscore")
	}

	return nil
}

// validateDirectoryPermissions validates that a directory has proper read/write permissions.
func (r *PathResolver) validateDirectoryPermissions(dir string) error {
	// Test write permissions by creating a temporary file
	testFile := filepath.Join(dir, ".opencenter_permission_test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %w", err)
	}
	file.Close()

	// Clean up test file
	if err := os.Remove(testFile); err != nil {
		// Log warning but don't fail - the directory is writable
		fmt.Printf("Warning: failed to remove test file %s: %v\n", testFile, err)
	}

	return nil
}

func (r *PathResolver) detectLegacyLayoutForCluster(clusterName string) error {
	r.mu.RLock()
	baseDir := r.baseDir
	r.mu.RUnlock()

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		orgDir := filepath.Join(baseDir, entry.Name())
		if _, err := os.Stat(filepath.Join(orgDir, ".git")); err != nil {
			continue
		}
		legacyPaths := []string{
			filepath.Join(orgDir, "secrets"),
			filepath.Join(orgDir, "."+clusterName+"-config.yaml"),
			filepath.Join(orgDir, "infrastructure", "clusters", clusterName),
		}
		for _, path := range legacyPaths {
			if _, err := os.Stat(path); err == nil {
				return &LegacyLayoutError{Path: orgDir}
			}
		}
	}
	return nil
}

// GetBaseDir returns the base directory for clusters.
func (r *PathResolver) GetBaseDir() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseDir
}

// GetRoots returns the configured secure zone roots.
func (r *PathResolver) GetRoots() PathRoots {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.roots
}

// GetStrategies returns all registered resolution strategies.
func (r *PathResolver) GetStrategies() []ResolutionStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.strategies
}

// GetOptions returns the current resolution options.
func (r *PathResolver) GetOptions() ResolutionOptions {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.options
}
