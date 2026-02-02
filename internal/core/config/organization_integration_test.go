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

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/config/strategies"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
)

// TestOrganizationBasedConfigLoading tests the complete workflow of loading
// configurations from organization-based directory structures.
func TestOrganizationBasedConfigLoading(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name         string
		setup        func() (string, string, string) // Returns: baseDir, clusterName, organization
		wantErr      bool
		validateFunc func(*testing.T, *internalconfig.Config, string)
	}{
		{
			name: "load config from explicit organization",
			setup: func() (string, string, string) {
				org := "test-org"
				cluster := "test-cluster"

				// Create organization-based directory structure
				orgDir := filepath.Join(tmpDir, org)
				clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", cluster)
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}

				// Create config file
				configPath := filepath.Join(clusterDir, "."+cluster+"-config.yaml")
				configContent := `schema_version: "1.0"
cluster:
  name: test-cluster
  fqdn: test.example.com
opencenter:
  meta:
    organization: test-org
  infrastructure:
    provider: openstack
`
				if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
					t.Fatal(err)
				}

				return tmpDir, cluster, org
			},
			wantErr: false,
			validateFunc: func(t *testing.T, cfg *internalconfig.Config, org string) {
				if cfg.OpenCenter.Meta.Organization != org {
					t.Errorf("expected organization %q, got %q", org, cfg.OpenCenter.Meta.Organization)
				}
			},
		},
		{
			name: "load config from default organization",
			setup: func() (string, string, string) {
				org := "opencenter"
				cluster := "default-cluster"

				// Create organization-based directory structure
				orgDir := filepath.Join(tmpDir, org)
				clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", cluster)
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}

				// Create config file
				configPath := filepath.Join(clusterDir, "."+cluster+"-config.yaml")
				configContent := `schema_version: "1.0"
cluster:
  name: default-cluster
  fqdn: default.example.com
opencenter:
  meta:
    organization: opencenter
  infrastructure:
    provider: openstack
`
				if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
					t.Fatal(err)
				}

				return tmpDir, cluster, org
			},
			wantErr: false,
			validateFunc: func(t *testing.T, cfg *internalconfig.Config, org string) {
				if cfg.OpenCenter.Meta.Organization != "opencenter" {
					t.Errorf("expected organization %q, got %q", "opencenter", cfg.OpenCenter.Meta.Organization)
				}
			},
		},
		{
			name: "load config from multiple organizations",
			setup: func() (string, string, string) {
				// Create multiple organizations
				orgs := []string{"org-alpha", "org-beta", "org-gamma"}
				cluster := "multi-org-cluster"

				for _, org := range orgs {
					orgDir := filepath.Join(tmpDir, org)
					clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", cluster)
					if err := os.MkdirAll(clusterDir, 0755); err != nil {
						t.Fatal(err)
					}

					// Create config file in each org
					configPath := filepath.Join(clusterDir, "."+cluster+"-config.yaml")
					configContent := `schema_version: "1.0"
cluster:
  name: multi-org-cluster
  fqdn: multi.example.com
opencenter:
  meta:
    organization: ` + org + `
  infrastructure:
    provider: openstack
`
					if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
						t.Fatal(err)
					}
				}

				// Return the first org for testing
				return tmpDir, cluster, orgs[0]
			},
			wantErr: false,
			validateFunc: func(t *testing.T, cfg *internalconfig.Config, org string) {
				// Should load from one of the organizations
				if cfg.OpenCenter.Meta.Organization == "" {
					t.Error("organization should not be empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir, clusterName, organization := tt.setup()

			// Create PathResolver
			pathResolver := paths.NewPathResolver(baseDir)

			// Resolve paths using organization
			clusterPaths, err := pathResolver.Resolve(ctx, clusterName, organization)
			if err != nil {
				t.Fatalf("PathResolver.Resolve() error = %v", err)
			}

			// Construct config file path
			configPath := filepath.Join(clusterPaths.ClusterDir, "."+clusterName+"-config.yaml")

			// Create ConfigManager and load config
			manager := NewConfigManager()
			manager.RegisterStrategy(strategies.NewV1Strategy())
			manager.RegisterStrategy(strategies.NewV2Strategy())
			manager.RegisterStrategy(strategies.NewLegacyStrategy())

			cfg, err := manager.Load(configPath, LoadOptions{
				AutoMigrate: false,
				Validate:    false,
				SkipCache:   false,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigManager.Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validateFunc != nil {
				tt.validateFunc(t, cfg, organization)
			}
		})
	}
}

// TestOrganizationSearchFunctionality tests the PathResolver's ability to
// search for clusters across multiple organizations.
func TestOrganizationSearchFunctionality(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup: Create multiple organizations with different clusters
	organizations := map[string][]string{
		"org-alpha": {"cluster-a1", "cluster-a2"},
		"org-beta":  {"cluster-b1", "cluster-b2"},
		"org-gamma": {"cluster-g1", "cluster-g2"},
	}

	for org, clusters := range organizations {
		for _, cluster := range clusters {
			clusterDir := filepath.Join(tmpDir, org, "infrastructure", "clusters", cluster)
			if err := os.MkdirAll(clusterDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Create a marker file to verify the cluster exists
			markerPath := filepath.Join(clusterDir, ".cluster-marker")
			if err := os.WriteFile(markerPath, []byte(cluster), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}

	pathResolver := paths.NewPathResolver(tmpDir)

	tests := []struct {
		name        string
		clusterName string
		wantErr     bool
		validateOrg func(*testing.T, string)
	}{
		{
			name:        "find cluster in first organization",
			clusterName: "cluster-a1",
			wantErr:     false,
			validateOrg: func(t *testing.T, org string) {
				if org != "org-alpha" {
					t.Errorf("expected organization %q, got %q", "org-alpha", org)
				}
			},
		},
		{
			name:        "find cluster in middle organization",
			clusterName: "cluster-b2",
			wantErr:     false,
			validateOrg: func(t *testing.T, org string) {
				if org != "org-beta" {
					t.Errorf("expected organization %q, got %q", "org-beta", org)
				}
			},
		},
		{
			name:        "find cluster in last organization",
			clusterName: "cluster-g1",
			wantErr:     false,
			validateOrg: func(t *testing.T, org string) {
				if org != "org-gamma" {
					t.Errorf("expected organization %q, got %q", "org-gamma", org)
				}
			},
		},
		{
			name:        "cluster not found in any organization",
			clusterName: "nonexistent-cluster",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use ResolveWithFallback to search across organizations
			clusterPaths, err := pathResolver.ResolveWithFallback(ctx, tt.clusterName)

			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveWithFallback() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Extract organization from the resolved path
				org := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(clusterPaths.ClusterDir))))

				if tt.validateOrg != nil {
					tt.validateOrg(t, org)
				}

				// Verify the cluster directory exists
				if _, err := os.Stat(clusterPaths.ClusterDir); os.IsNotExist(err) {
					t.Errorf("cluster directory does not exist: %s", clusterPaths.ClusterDir)
				}
			}
		})
	}
}

// TestConfigPathResolution tests the complete path resolution workflow
// for configuration files in organization-based structures.
func TestConfigPathResolution(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name         string
		setup        func() (string, string, string) // Returns: baseDir, clusterName, organization
		wantErr      bool
		validatePath func(*testing.T, *paths.ClusterPaths)
	}{
		{
			name: "resolve all paths for organization-based structure",
			setup: func() (string, string, string) {
				org := "test-org"
				cluster := "test-cluster"

				// Create complete organization-based structure
				orgDir := filepath.Join(tmpDir, org)
				dirs := []string{
					filepath.Join(orgDir, "infrastructure", "clusters", cluster),
					filepath.Join(orgDir, "applications", "overlays", cluster),
					filepath.Join(orgDir, "secrets", "age", "keys"),
					filepath.Join(orgDir, "secrets", "ssh"),
				}

				for _, dir := range dirs {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
				}

				return tmpDir, cluster, org
			},
			wantErr: false,
			validatePath: func(t *testing.T, p *paths.ClusterPaths) {
				// Verify all expected paths are set correctly
				if p.OrganizationDir == "" {
					t.Error("OrganizationDir should not be empty")
				}
				if p.ClusterDir == "" {
					t.Error("ClusterDir should not be empty")
				}
				if p.ConfigPath == "" {
					t.Error("ConfigPath should not be empty")
				}
				if p.SecretsDir == "" {
					t.Error("SecretsDir should not be empty")
				}
				if p.SOPSKeyPath == "" {
					t.Error("SOPSKeyPath should not be empty")
				}
				if p.SSHKeyPath == "" {
					t.Error("SSHKeyPath should not be empty")
				}
				if p.GitOpsDir == "" {
					t.Error("GitOpsDir should not be empty")
				}

				// Verify paths follow organization structure
				expectedOrgDir := filepath.Join(tmpDir, "test-org")
				if p.OrganizationDir != expectedOrgDir {
					t.Errorf("OrganizationDir = %s, want %s", p.OrganizationDir, expectedOrgDir)
				}

				expectedClusterDir := filepath.Join(expectedOrgDir, "infrastructure", "clusters", "test-cluster")
				if p.ClusterDir != expectedClusterDir {
					t.Errorf("ClusterDir = %s, want %s", p.ClusterDir, expectedClusterDir)
				}
			},
		},
		{
			name: "resolve paths with default organization",
			setup: func() (string, string, string) {
				org := "opencenter"
				cluster := "default-cluster"

				// Create structure with default organization
				orgDir := filepath.Join(tmpDir, org)
				clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", cluster)
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}

				return tmpDir, cluster, ""
			},
			wantErr: false,
			validatePath: func(t *testing.T, p *paths.ClusterPaths) {
				// Should use default organization
				expectedOrgDir := filepath.Join(tmpDir, "opencenter")
				if p.OrganizationDir != expectedOrgDir {
					t.Errorf("OrganizationDir = %s, want %s (default)", p.OrganizationDir, expectedOrgDir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir, clusterName, organization := tt.setup()

			pathResolver := paths.NewPathResolver(baseDir)

			var clusterPaths *paths.ClusterPaths
			var err error

			if organization != "" {
				clusterPaths, err = pathResolver.Resolve(ctx, clusterName, organization)
			} else {
				clusterPaths, err = pathResolver.ResolveWithFallback(ctx, clusterName)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("PathResolver.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validatePath != nil {
				tt.validatePath(t, clusterPaths)
			}
		})
	}
}

// TestEndToEndOrganizationWorkflow tests the complete workflow from
// path resolution to config loading in an organization-based structure.
func TestEndToEndOrganizationWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup: Create a realistic organization-based structure
	organization := "acme-corp"
	clusterName := "production-cluster"

	// Create directory structure
	orgDir := filepath.Join(tmpDir, organization)
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create config file
	configPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
	configContent := `schema_version: "1.0"
cluster:
  name: production-cluster
  fqdn: prod.acme-corp.com
opencenter:
  meta:
    organization: acme-corp
    region: us-east-1
    env: production
  infrastructure:
    provider: openstack
  cluster:
    cluster_name: production-cluster
    kubernetes_version: "1.28.0"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Step 1: Resolve paths using PathResolver
	pathResolver := paths.NewPathResolver(tmpDir)
	clusterPaths, err := pathResolver.Resolve(ctx, clusterName, organization)
	if err != nil {
		t.Fatalf("PathResolver.Resolve() failed: %v", err)
	}

	// Verify paths are correct
	if clusterPaths.OrganizationDir != orgDir {
		t.Errorf("OrganizationDir = %s, want %s", clusterPaths.OrganizationDir, orgDir)
	}

	// Step 2: Load config using ConfigManager
	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())
	manager.RegisterStrategy(strategies.NewV2Strategy())
	manager.RegisterStrategy(strategies.NewLegacyStrategy())

	cfg, err := manager.Load(clusterPaths.ConfigPath, LoadOptions{
		AutoMigrate: false,
		Validate:    false,
		SkipCache:   false,
	})
	if err != nil {
		t.Fatalf("ConfigManager.Load() failed: %v", err)
	}

	// Step 3: Verify loaded config matches expected values
	if cfg.OpenCenter.Meta.Organization != organization {
		t.Errorf("Organization = %s, want %s", cfg.OpenCenter.Meta.Organization, organization)
	}

	if cfg.OpenCenter.Meta.Region != "us-east-1" {
		t.Errorf("Region = %s, want %s", cfg.OpenCenter.Meta.Region, "us-east-1")
	}

	if cfg.OpenCenter.Meta.Env != "production" {
		t.Errorf("Env = %s, want %s", cfg.OpenCenter.Meta.Env, "production")
	}

	if cfg.OpenCenter.Cluster.ClusterName != clusterName {
		t.Errorf("ClusterName = %s, want %s", cfg.OpenCenter.Cluster.ClusterName, clusterName)
	}

	// Step 4: Verify cache is working
	cachedCfg, err := manager.Load(clusterPaths.ConfigPath, LoadOptions{
		AutoMigrate: false,
		Validate:    false,
		SkipCache:   false,
	})
	if err != nil {
		t.Fatalf("ConfigManager.Load() from cache failed: %v", err)
	}

	if cachedCfg.OpenCenter.Meta.Organization != organization {
		t.Error("Cached config does not match original")
	}

	// Step 5: Test cache invalidation
	manager.InvalidateCache(clusterPaths.ConfigPath)

	// Load again after invalidation
	reloadedCfg, err := manager.Load(clusterPaths.ConfigPath, LoadOptions{
		AutoMigrate: false,
		Validate:    false,
		SkipCache:   false,
	})
	if err != nil {
		t.Fatalf("ConfigManager.Load() after invalidation failed: %v", err)
	}

	if reloadedCfg.OpenCenter.Meta.Organization != organization {
		t.Error("Reloaded config does not match original")
	}
}
