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
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPathResolver_Resolve(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name         string
		setup        func() string
		clusterName  string
		organization string
		wantErr      bool
		validate     func(*testing.T, *ClusterPaths)
	}{
		{
			name: "resolve with explicit organization",
			setup: func() string {
				orgDir := filepath.Join(tmpDir, "test-org")
				clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName:  "test-cluster",
			organization: "test-org",
			wantErr:      false,
			validate: func(t *testing.T, paths *ClusterPaths) {
				if paths == nil {
					t.Fatal("paths is nil")
				}
				expectedOrgDir := filepath.Join(tmpDir, "test-org")
				if paths.OrganizationDir != expectedOrgDir {
					t.Errorf("OrganizationDir = %s, want %s", paths.OrganizationDir, expectedOrgDir)
				}
			},
		},
		{
			name: "resolve with default organization",
			setup: func() string {
				orgDir := filepath.Join(tmpDir, "opencenter")
				clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName:  "test-cluster",
			organization: "",
			wantErr:      false,
			validate: func(t *testing.T, paths *ClusterPaths) {
				if paths == nil {
					t.Fatal("paths is nil")
				}
				expectedOrgDir := filepath.Join(tmpDir, "opencenter")
				if paths.OrganizationDir != expectedOrgDir {
					t.Errorf("OrganizationDir = %s, want %s", paths.OrganizationDir, expectedOrgDir)
				}
			},
		},
		{
			name: "cluster not found",
			setup: func() string {
				return tmpDir
			},
			clusterName:  "nonexistent",
			organization: "test-org",
			wantErr:      true,
		},
		{
			name: "empty cluster name",
			setup: func() string {
				return tmpDir
			},
			clusterName:  "",
			organization: "test-org",
			wantErr:      true,
		},
		{
			name: "invalid cluster name",
			setup: func() string {
				return tmpDir
			},
			clusterName:  "test@cluster",
			organization: "test-org",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := tt.setup()
			resolver := NewPathResolver(baseDir)

			paths, err := resolver.Resolve(ctx, tt.clusterName, tt.organization)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, paths)
			}
		})
	}
}

func TestPathResolver_ResolveWithFallback(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func() string
		clusterName string
		wantErr     bool
		validate    func(*testing.T, *ClusterPaths)
	}{
		{
			name: "find cluster in first organization",
			setup: func() string {
				orgDir := filepath.Join(tmpDir, "org1")
				clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName: "test-cluster",
			wantErr:     false,
			validate: func(t *testing.T, paths *ClusterPaths) {
				if paths == nil {
					t.Fatal("paths is nil")
				}
				expectedOrgDir := filepath.Join(tmpDir, "org1")
				if paths.OrganizationDir != expectedOrgDir {
					t.Errorf("OrganizationDir = %s, want %s", paths.OrganizationDir, expectedOrgDir)
				}
			},
		},
		{
			name: "find cluster in organization",
			setup: func() string {
				// Create org-alpha without the cluster
				orgAlphaDir := filepath.Join(tmpDir, "org-alpha")
				if err := os.MkdirAll(orgAlphaDir, 0755); err != nil {
					t.Fatal(err)
				}
				// Create org-beta with the cluster
				orgBetaDir := filepath.Join(tmpDir, "org-beta")
				clusterDir := filepath.Join(orgBetaDir, "infrastructure", "clusters", "test-cluster")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName: "test-cluster",
			wantErr:     false,
			validate: func(t *testing.T, paths *ClusterPaths) {
				if paths == nil {
					t.Fatal("paths is nil")
				}
				// Should find in one of the organizations
				// We can't guarantee which one due to directory iteration order
				// Just verify it found the cluster
				if paths.ClusterDir == "" {
					t.Error("ClusterDir should not be empty")
				}
			},
		},
		{
			name: "cluster not found in any organization",
			setup: func() string {
				org1Dir := filepath.Join(tmpDir, "org1")
				if err := os.MkdirAll(org1Dir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName: "nonexistent",
			wantErr:     true,
		},
		{
			name: "empty cluster name",
			setup: func() string {
				return tmpDir
			},
			clusterName: "",
			wantErr:     true,
		},
		{
			name: "clusters directory does not exist",
			setup: func() string {
				nonexistent := filepath.Join(tmpDir, "nonexistent")
				return nonexistent
			},
			clusterName: "test-cluster",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := tt.setup()
			resolver := NewPathResolver(baseDir)

			paths, err := resolver.ResolveWithFallback(ctx, tt.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveWithFallback() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, paths)
			}
		})
	}
}

func TestPathResolver_InvalidateCache(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup test cluster
	orgDir := filepath.Join(tmpDir, "test-org")
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	resolver := NewPathResolver(tmpDir)

	// First resolve to populate cache
	_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Verify cache has entry
	stats := resolver.GetCacheStats()
	if stats.Entries == 0 {
		t.Error("cache should have entries after Resolve()")
	}

	// Invalidate cache
	resolver.InvalidateCache("test-cluster")

	// Verify cache was cleared for this cluster
	stats = resolver.GetCacheStats()
	if stats.Entries != 0 {
		t.Errorf("cache should be empty after InvalidateCache(), got %d entries", stats.Entries)
	}
}

func TestPathResolver_ClearCache(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup multiple test clusters
	for _, org := range []string{"org1", "org2"} {
		for _, cluster := range []string{"cluster1", "cluster2"} {
			clusterDir := filepath.Join(tmpDir, org, "infrastructure", "clusters", cluster)
			if err := os.MkdirAll(clusterDir, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	resolver := NewPathResolver(tmpDir)

	// Resolve multiple clusters to populate cache
	_, _ = resolver.Resolve(ctx, "cluster1", "org1")
	_, _ = resolver.Resolve(ctx, "cluster2", "org1")
	_, _ = resolver.Resolve(ctx, "cluster1", "org2")

	// Verify cache has entries
	stats := resolver.GetCacheStats()
	if stats.Entries == 0 {
		t.Error("cache should have entries after multiple Resolve() calls")
	}

	// Clear entire cache
	resolver.ClearCache()

	// Verify cache is empty
	stats = resolver.GetCacheStats()
	if stats.Entries != 0 {
		t.Errorf("cache should be empty after ClearCache(), got %d entries", stats.Entries)
	}
	if stats.Hits != 0 {
		t.Errorf("hits should be reset after ClearCache(), got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("misses should be reset after ClearCache(), got %d", stats.Misses)
	}
}

func TestPathResolver_ThreadSafety(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup test cluster
	orgDir := filepath.Join(tmpDir, "test-org")
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	resolver := NewPathResolver(tmpDir)

	// Run concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Test concurrent Resolve calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, _ = resolver.Resolve(ctx, "test-cluster", "test-org")
			}
		}()
	}

	// Test concurrent InvalidateCache calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				resolver.InvalidateCache("test-cluster")
			}
		}()
	}

	// Test concurrent GetCacheStats calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = resolver.GetCacheStats()
			}
		}()
	}

	// Test concurrent GetBaseDir calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = resolver.GetBaseDir()
			}
		}()
	}

	wg.Wait()

	// If we get here without deadlock or race conditions, test passes
	t.Log("Thread safety test completed successfully")
}

func TestPathResolver_CacheHitRate(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup test cluster
	orgDir := filepath.Join(tmpDir, "test-org")
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	resolver := NewPathResolver(tmpDir)

	// First call should be a cache miss
	_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	stats := resolver.GetCacheStats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 cache miss, got %d", stats.Misses)
	}
	if stats.Hits != 0 {
		t.Errorf("expected 0 cache hits, got %d", stats.Hits)
	}

	// Second call should be a cache hit
	_, err = resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	stats = resolver.GetCacheStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 cache hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 cache miss, got %d", stats.Misses)
	}

	// Hit rate should be 50%
	expectedHitRate := 0.5
	if stats.HitRate != expectedHitRate {
		t.Errorf("expected hit rate %.2f, got %.2f", expectedHitRate, stats.HitRate)
	}
}

func TestPathResolver_ValidateClusterName(t *testing.T) {
	resolver := NewPathResolver("/tmp/test")

	tests := []struct {
		name        string
		clusterName string
		wantErr     bool
	}{
		{
			name:        "valid alphanumeric",
			clusterName: "test-cluster",
			wantErr:     false,
		},
		{
			name:        "valid with underscores",
			clusterName: "test_cluster",
			wantErr:     false,
		},
		{
			name:        "valid mixed case",
			clusterName: "TestCluster",
			wantErr:     false,
		},
		{
			name:        "empty name",
			clusterName: "",
			wantErr:     true,
		},
		{
			name:        "too long",
			clusterName: "this-is-a-very-long-cluster-name-that-exceeds-the-maximum-length-of-63-characters",
			wantErr:     true,
		},
		{
			name:        "starts with hyphen",
			clusterName: "-test-cluster",
			wantErr:     true,
		},
		{
			name:        "ends with hyphen",
			clusterName: "test-cluster-",
			wantErr:     true,
		},
		{
			name:        "starts with underscore",
			clusterName: "_test-cluster",
			wantErr:     true,
		},
		{
			name:        "ends with underscore",
			clusterName: "test-cluster_",
			wantErr:     true,
		},
		{
			name:        "contains special characters",
			clusterName: "test@cluster",
			wantErr:     true,
		},
		{
			name:        "contains spaces",
			clusterName: "test cluster",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.validateClusterName(tt.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateClusterName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathResolver_GetOrganization(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func() string
		clusterName string
		wantOrg     string
		wantErr     bool
	}{
		{
			name: "cluster in organization",
			setup: func() string {
				orgDir := filepath.Join(tmpDir, "test-org")
				clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName: "test-cluster",
			wantOrg:     "test-org",
			wantErr:     false,
		},
		{
			name: "cluster not found",
			setup: func() string {
				orgDir := filepath.Join(tmpDir, "test-org")
				if err := os.MkdirAll(orgDir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName: "nonexistent",
			wantOrg:     "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := tt.setup()
			resolver := NewPathResolver(baseDir)

			org, err := resolver.GetOrganization(ctx, tt.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOrganization() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if org != tt.wantOrg {
				t.Errorf("GetOrganization() = %s, want %s", org, tt.wantOrg)
			}
		})
	}
}

// Additional comprehensive tests for PathResolver

func TestPathResolver_CreateClusterDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name         string
		clusterName  string
		organization string
		wantErr      bool
		validate     func(*testing.T, string)
	}{
		{
			name:         "create all directories",
			clusterName:  "test-cluster",
			organization: "test-org",
			wantErr:      false,
			validate: func(t *testing.T, baseDir string) {
				expectedDirs := []string{
					filepath.Join(baseDir, "test-org"),
					filepath.Join(baseDir, "test-org", "infrastructure"),
					filepath.Join(baseDir, "test-org", "infrastructure", "clusters"),
					filepath.Join(baseDir, "test-org", "infrastructure", "clusters", "test-cluster"),
					filepath.Join(baseDir, "test-org", "applications"),
					filepath.Join(baseDir, "test-org", "applications", "overlays"),
					filepath.Join(baseDir, "test-org", "applications", "overlays", "test-cluster"),
					filepath.Join(baseDir, "test-org", "secrets"),
					filepath.Join(baseDir, "test-org", "secrets", "age"),
					filepath.Join(baseDir, "test-org", "secrets", "age", "keys"),
					filepath.Join(baseDir, "test-org", "infrastructure", "clusters", "test-cluster", "inventory"),
					filepath.Join(baseDir, "test-org", "infrastructure", "clusters", "test-cluster", "venv"),
					filepath.Join(baseDir, "test-org", "infrastructure", "clusters", "test-cluster", ".bin"),
				}

				for _, dir := range expectedDirs {
					if stat, err := os.Stat(dir); err != nil {
						t.Errorf("directory %s does not exist: %v", dir, err)
					} else if !stat.IsDir() {
						t.Errorf("path %s exists but is not a directory", dir)
					}
				}
			},
		},
		{
			name:         "create with default organization",
			clusterName:  "test-cluster",
			organization: "",
			wantErr:      false,
			validate: func(t *testing.T, baseDir string) {
				expectedDir := filepath.Join(baseDir, "opencenter", "infrastructure", "clusters", "test-cluster")
				if stat, err := os.Stat(expectedDir); err != nil {
					t.Errorf("directory %s does not exist: %v", expectedDir, err)
				} else if !stat.IsDir() {
					t.Errorf("path %s exists but is not a directory", expectedDir)
				}
			},
		},
		{
			name:         "invalid cluster name",
			clusterName:  "test@cluster",
			organization: "test-org",
			wantErr:      true,
		},
		{
			name:         "empty cluster name",
			clusterName:  "",
			organization: "test-org",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewPathResolver(tmpDir)
			err := resolver.CreateClusterDirectories(ctx, tt.clusterName, tt.organization)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateClusterDirectories() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, tmpDir)
			}
		})
	}
}

func TestPathResolver_ValidatePath(t *testing.T) {
	resolver := NewPathResolver("/tmp/test")

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid absolute path",
			path:    "/tmp/test/cluster",
			wantErr: false,
		},
		{
			name:    "valid path with tilde",
			path:    "~/test/cluster",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "path with traversal",
			path:    "/tmp/../etc/passwd",
			wantErr: true,
		},
		{
			name:    "relative path",
			path:    "relative/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathResolver_DetectStructureType(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func()
		clusterName string
		want        StructureType
		wantErr     bool
	}{
		{
			name: "organization structure with default org",
			setup: func() {
				clusterDir := filepath.Join(tmpDir, "opencenter", "infrastructure", "clusters", "test-cluster")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
			},
			clusterName: "test-cluster",
			want:        StructureTypeOrganization,
			wantErr:     false,
		},
		{
			name:        "unknown structure",
			setup:       func() {},
			clusterName: "nonexistent",
			want:        StructureTypeUnknown,
			wantErr:     false,
		},
		{
			name:        "invalid cluster name",
			setup:       func() {},
			clusterName: "test@cluster",
			want:        StructureTypeUnknown,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			resolver := NewPathResolver(tmpDir)

			got, err := resolver.DetectStructureType(ctx, tt.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectStructureType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DetectStructureType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathResolver_GetBaseDir(t *testing.T) {
	baseDir := "/tmp/test"
	resolver := NewPathResolver(baseDir)

	got := resolver.GetBaseDir()
	if got != baseDir {
		t.Errorf("GetBaseDir() = %s, want %s", got, baseDir)
	}
}

func TestPathResolver_GetStrategies(t *testing.T) {
	resolver := NewPathResolver("/tmp/test")

	strategies := resolver.GetStrategies()
	if len(strategies) == 0 {
		t.Error("GetStrategies() returned empty slice")
	}

	// Verify we have the org-based strategy
	found := false
	for _, s := range strategies {
		if s.Name() == "org-based" {
			found = true
			break
		}
	}
	if !found {
		t.Error("org-based strategy not found in strategies")
	}
}

func TestPathResolver_GetOptions(t *testing.T) {
	options := ResolutionOptions{
		Organization:  "custom-org",
		CacheResults:  false,
		ValidatePaths: true,
	}

	resolver := NewPathResolverWithOptions("/tmp/test", options)
	got := resolver.GetOptions()

	if got.Organization != options.Organization {
		t.Errorf("Organization = %s, want %s", got.Organization, options.Organization)
	}
	if got.CacheResults != options.CacheResults {
		t.Errorf("CacheResults = %v, want %v", got.CacheResults, options.CacheResults)
	}
	if got.ValidatePaths != options.ValidatePaths {
		t.Errorf("ValidatePaths = %v, want %v", got.ValidatePaths, options.ValidatePaths)
	}
}

func TestPathResolver_WithCustomOptions(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup cluster with custom organization
	orgDir := filepath.Join(tmpDir, "custom-org")
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	options := ResolutionOptions{
		Organization:  "custom-org",
		CacheResults:  true,
		ValidatePaths: false,
	}

	resolver := NewPathResolverWithOptions(tmpDir, options)

	// Resolve without specifying organization (should use custom-org from options)
	paths, err := resolver.Resolve(ctx, "test-cluster", "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expectedOrgDir := filepath.Join(tmpDir, "custom-org")
	if paths.OrganizationDir != expectedOrgDir {
		t.Errorf("OrganizationDir = %s, want %s", paths.OrganizationDir, expectedOrgDir)
	}
}

func TestPathResolver_CacheDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup test cluster
	orgDir := filepath.Join(tmpDir, "test-org")
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	options := ResolutionOptions{
		Organization:  "opencenter",
		CacheResults:  false,
		ValidatePaths: false,
	}

	resolver := NewPathResolverWithOptions(tmpDir, options)

	// Resolve multiple times
	_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}

	_, err = resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}

	// Verify cache is not used (stats should be zero)
	stats := resolver.GetCacheStats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("cache should not be used when disabled, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}
}

func TestPathResolver_MultipleClustersSameOrganization(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup multiple clusters in same organization
	clusters := []string{"cluster1", "cluster2", "cluster3"}
	organization := "test-org"

	for _, cluster := range clusters {
		clusterDir := filepath.Join(tmpDir, organization, "infrastructure", "clusters", cluster)
		if err := os.MkdirAll(clusterDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	resolver := NewPathResolver(tmpDir)

	// Resolve all clusters
	for _, cluster := range clusters {
		paths, err := resolver.Resolve(ctx, cluster, organization)
		if err != nil {
			t.Errorf("Resolve(%s) error = %v", cluster, err)
			continue
		}

		expectedClusterDir := filepath.Join(tmpDir, organization, "infrastructure", "clusters", cluster)
		if paths.ClusterDir != expectedClusterDir {
			t.Errorf("ClusterDir for %s = %s, want %s", cluster, paths.ClusterDir, expectedClusterDir)
		}
	}
}

func TestPathResolver_ResolveAfterInvalidate(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup test cluster
	orgDir := filepath.Join(tmpDir, "test-org")
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	resolver := NewPathResolver(tmpDir)

	// First resolve (cache miss)
	_, err := resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}

	// Second resolve (cache hit)
	_, err = resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}

	stats := resolver.GetCacheStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 cache hit before invalidation, got %d", stats.Hits)
	}

	// Invalidate cache
	resolver.InvalidateCache("test-cluster")

	// Third resolve (cache miss after invalidation)
	_, err = resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("third Resolve() error = %v", err)
	}

	stats = resolver.GetCacheStats()
	if stats.Misses != 2 {
		t.Errorf("expected 2 cache misses after invalidation, got %d", stats.Misses)
	}
}
