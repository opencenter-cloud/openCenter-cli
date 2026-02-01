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
type PathResolver struct {
	// baseDir is the base directory for all clusters
	baseDir string

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
func NewPathResolver(baseDir string) *PathResolver {
	return NewPathResolverWithOptions(baseDir, DefaultResolutionOptions())
}

// NewPathResolverWithOptions creates a new path resolver with custom options.
func NewPathResolverWithOptions(baseDir string, options ResolutionOptions) *PathResolver {
	baseDir = expandPath(baseDir)

	// Create organization-based strategy only
	strategy := NewOrgBasedStrategy(baseDir)

	// Create cache if enabled
	var cache *PathCache
	if options.CacheResults {
		cache = DefaultPathCache()
	}

	return &PathResolver{
		baseDir:    baseDir,
		strategies: []ResolutionStrategy{strategy},
		cache:      cache,
		options:    options,
	}
}

// Resolve resolves all paths for the given cluster and organization.
// This is the primary method for path resolution.
func (r *PathResolver) Resolve(ctx context.Context, clusterName, organization string) (*ClusterPaths, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// Validate cluster name
	if err := r.validateClusterName(clusterName); err != nil {
		return nil, fmt.Errorf("invalid cluster name: %w", err)
	}

	// Use default organization if not specified
	if organization == "" {
		organization = r.options.Organization
	}

	// Validate organization name
	if err := r.validateClusterName(organization); err != nil {
		return nil, fmt.Errorf("invalid organization name: %w", err)
	}

	// Check cache first
	if r.cache != nil {
		if paths := r.cache.Get(clusterName, organization); paths != nil {
			return paths, nil
		}
	}

	// Use organization-based strategy
	strategy := r.strategies[0]
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

// ResolveWithFallback resolves paths for a cluster.
// If organization is not specified, searches across all organizations.
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

	// Search for cluster in all organization directories
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("clusters directory does not exist: %s", r.baseDir)
		}
		return nil, fmt.Errorf("failed to read clusters directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		orgName := entry.Name()
		clusterDir := filepath.Join(r.baseDir, orgName, "infrastructure", "clusters", clusterName)
		if _, err := os.Stat(clusterDir); err == nil {
			// Found the cluster, resolve with this organization
			paths, err := r.Resolve(ctx, clusterName, orgName)
			if err == nil {
				// Cache the result
				if r.cache != nil {
					r.cache.Set(clusterName, "", "org-search", paths)
				}
				return paths, nil
			}
		}
	}

	return nil, fmt.Errorf("cluster %s not found in any organization", clusterName)
}

// InvalidateCache invalidates the cache for a specific cluster.
func (r *PathResolver) InvalidateCache(clusterName string) {
	if r.cache != nil {
		r.cache.InvalidateCluster(clusterName)
	}
}

// ClearCache clears all cached path resolutions.
func (r *PathResolver) ClearCache() {
	if r.cache != nil {
		r.cache.Clear()
	}
}

// GetCacheStats returns cache statistics.
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

	// Check if cluster exists in organization structure
	strategy := r.strategies[0]
	canResolve, err := strategy.CanResolve(ctx, clusterName, "")
	if err != nil {
		return StructureTypeUnknown, err
	}

	if canResolve {
		return StructureTypeOrganization, nil
	}

	return StructureTypeUnknown, nil
}

// GetOrganization determines the organization for a cluster.
// Returns empty string if the cluster uses legacy structure.
func (r *PathResolver) GetOrganization(ctx context.Context, clusterName string) (string, error) {
	if err := r.validateClusterName(clusterName); err != nil {
		return "", fmt.Errorf("invalid cluster name: %w", err)
	}

	// Check if cluster exists in organization structure
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		orgName := entry.Name()
		clusterDir := filepath.Join(r.baseDir, orgName, "infrastructure", "clusters", clusterName)
		if _, err := os.Stat(clusterDir); err == nil {
			return orgName, nil
		}
	}

	return "", nil
}

// CreateClusterDirectories creates all necessary directories for a cluster.
func (r *PathResolver) CreateClusterDirectories(ctx context.Context, clusterName, organization string) error {
	if err := r.validateClusterName(clusterName); err != nil {
		return fmt.Errorf("invalid cluster name: %w", err)
	}

	if organization == "" {
		organization = r.options.Organization
	}

	if err := r.validateClusterName(organization); err != nil {
		return fmt.Errorf("invalid organization name: %w", err)
	}

	// Resolve paths using organization-based strategy
	strategy := r.strategies[0]
	paths, err := strategy.Resolve(ctx, clusterName, organization)
	if err != nil {
		return fmt.Errorf("failed to resolve paths: %w", err)
	}

	// Create all directories
	dirs := []string{
		paths.OrganizationDir,
		filepath.Join(paths.OrganizationDir, "infrastructure"),
		filepath.Join(paths.OrganizationDir, "infrastructure", "clusters"),
		paths.ClusterDir,
		filepath.Join(paths.OrganizationDir, "applications"),
		filepath.Join(paths.OrganizationDir, "applications", "overlays"),
		paths.ApplicationsDir,
		paths.SecretsDir,
		filepath.Join(paths.SecretsDir, "age"),
		filepath.Dir(paths.SOPSKeyPath), // age/keys directory
		paths.InventoryPath,
		paths.VenvPath,
		paths.BinPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Verify the directory was created
		if stat, err := os.Stat(dir); err != nil {
			return fmt.Errorf("failed to verify directory %s: %w", dir, err)
		} else if !stat.IsDir() {
			return fmt.Errorf("path %s exists but is not a directory", dir)
		}

		// Validate permissions if requested
		if r.options.ValidatePaths {
			if err := r.validateDirectoryPermissions(dir); err != nil {
				return fmt.Errorf("directory %s has insufficient permissions: %w", dir, err)
			}
		}
	}

	// Invalidate cache for this cluster
	r.InvalidateCache(clusterName)

	return nil
}

// ValidatePath validates that a path is safe and accessible.
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

// GetBaseDir returns the base directory for clusters.
func (r *PathResolver) GetBaseDir() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseDir
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
