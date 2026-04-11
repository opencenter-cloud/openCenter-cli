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
	"testing"
)

func TestOrgBasedStrategy_Name(t *testing.T) {
	strategy := NewOrgBasedStrategy("/tmp/test")
	if strategy.Name() != "org-based" {
		t.Errorf("expected strategy name 'org-based', got '%s'", strategy.Name())
	}
}

func TestOrgBasedStrategy_CanResolve(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name         string
		setup        func() string
		clusterName  string
		organization string
		want         bool
		wantErr      bool
	}{
		{
			name: "cluster exists in organization",
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
			want:         true,
			wantErr:      false,
		},
		{
			name: "cluster does not exist",
			setup: func() string {
				orgDir := filepath.Join(tmpDir, "test-org")
				if err := os.MkdirAll(orgDir, 0755); err != nil {
					t.Fatal(err)
				}
				return tmpDir
			},
			clusterName:  "nonexistent-cluster",
			organization: "test-org",
			want:         false,
			wantErr:      false,
		},
		{
			name: "organization does not exist",
			setup: func() string {
				return tmpDir
			},
			clusterName:  "test-cluster",
			organization: "nonexistent-org",
			want:         false,
			wantErr:      false,
		},
		{
			name: "default organization when empty",
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
			want:         true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := tt.setup()
			strategy := NewOrgBasedStrategy(baseDir)

			got, err := strategy.CanResolve(ctx, tt.clusterName, tt.organization)
			if (err != nil) != tt.wantErr {
				t.Errorf("CanResolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CanResolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrgBasedStrategy_Resolve(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name         string
		clusterName  string
		organization string
		validate     func(*testing.T, *ClusterPaths)
	}{
		{
			name:         "resolve with explicit organization",
			clusterName:  "test-cluster",
			organization: "test-org",
			validate: func(t *testing.T, paths *ClusterPaths) {
				expectedOrgDir := filepath.Join(tmpDir, "test-org")
				if paths.OrganizationDir != expectedOrgDir {
					t.Errorf("OrganizationDir = %s, want %s", paths.OrganizationDir, expectedOrgDir)
				}

				expectedClusterDir := filepath.Join(expectedOrgDir, "infrastructure", "clusters", "test-cluster")
				if paths.ClusterDir != expectedClusterDir {
					t.Errorf("ClusterDir = %s, want %s", paths.ClusterDir, expectedClusterDir)
				}

				expectedAppsDir := filepath.Join(expectedOrgDir, "applications", "overlays", "test-cluster")
				if paths.ApplicationsDir != expectedAppsDir {
					t.Errorf("ApplicationsDir = %s, want %s", paths.ApplicationsDir, expectedAppsDir)
				}

				expectedSecretsDir := filepath.Join(expectedOrgDir, "secrets")
				if paths.SecretsDir != expectedSecretsDir {
					t.Errorf("SecretsDir = %s, want %s", paths.SecretsDir, expectedSecretsDir)
				}

				expectedSOPSKey := filepath.Join(expectedSecretsDir, "age", "keys", "test-cluster-key.txt")
				if paths.SOPSKeyPath != expectedSOPSKey {
					t.Errorf("SOPSKeyPath = %s, want %s", paths.SOPSKeyPath, expectedSOPSKey)
				}

				expectedSOPSConfig := filepath.Join(expectedOrgDir, ".sops.yaml")
				if paths.SOPSConfigPath != expectedSOPSConfig {
					t.Errorf("SOPSConfigPath = %s, want %s", paths.SOPSConfigPath, expectedSOPSConfig)
				}

				expectedKubeconfig := filepath.Join(expectedClusterDir, "kubeconfig.yaml")
				if paths.KubeconfigPath != expectedKubeconfig {
					t.Errorf("KubeconfigPath = %s, want %s", paths.KubeconfigPath, expectedKubeconfig)
				}

				expectedInventory := filepath.Join(expectedClusterDir, "inventory")
				if paths.InventoryPath != expectedInventory {
					t.Errorf("InventoryPath = %s, want %s", paths.InventoryPath, expectedInventory)
				}

				expectedVenv := filepath.Join(expectedClusterDir, "venv")
				if paths.VenvPath != expectedVenv {
					t.Errorf("VenvPath = %s, want %s", paths.VenvPath, expectedVenv)
				}

				expectedBin := filepath.Join(expectedClusterDir, ".bin")
				if paths.BinPath != expectedBin {
					t.Errorf("BinPath = %s, want %s", paths.BinPath, expectedBin)
				}

				expectedConfig := filepath.Join(expectedOrgDir, ".test-cluster-config.yaml")
				if paths.ConfigPath != expectedConfig {
					t.Errorf("ConfigPath = %s, want %s", paths.ConfigPath, expectedConfig)
				}

				expectedSSHKey := filepath.Join(expectedSecretsDir, "ssh", "test-cluster")
				if paths.SSHKeyPath != expectedSSHKey {
					t.Errorf("SSHKeyPath = %s, want %s", paths.SSHKeyPath, expectedSSHKey)
				}

				if paths.GitOpsDir != expectedOrgDir {
					t.Errorf("GitOpsDir = %s, want %s", paths.GitOpsDir, expectedOrgDir)
				}
			},
		},
		{
			name:         "resolve with default organization",
			clusterName:  "prod-cluster",
			organization: "",
			validate: func(t *testing.T, paths *ClusterPaths) {
				expectedOrgDir := filepath.Join(tmpDir, "opencenter")
				if paths.OrganizationDir != expectedOrgDir {
					t.Errorf("OrganizationDir = %s, want %s (default org)", paths.OrganizationDir, expectedOrgDir)
				}

				expectedClusterDir := filepath.Join(expectedOrgDir, "infrastructure", "clusters", "prod-cluster")
				if paths.ClusterDir != expectedClusterDir {
					t.Errorf("ClusterDir = %s, want %s", paths.ClusterDir, expectedClusterDir)
				}
			},
		},
		{
			name:         "resolve with hyphenated cluster name",
			clusterName:  "my-test-cluster",
			organization: "my-org",
			validate: func(t *testing.T, paths *ClusterPaths) {
				expectedConfig := filepath.Join(tmpDir, "my-org", ".my-test-cluster-config.yaml")
				if paths.ConfigPath != expectedConfig {
					t.Errorf("ConfigPath = %s, want %s", paths.ConfigPath, expectedConfig)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewOrgBasedStrategy(tmpDir)
			paths, err := strategy.Resolve(ctx, tt.clusterName, tt.organization)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if paths == nil {
				t.Fatal("Resolve() returned nil paths")
			}
			tt.validate(t, paths)
		})
	}
}

func TestOrgBasedStrategy_PathExpansion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "absolute path",
			baseDir: "/tmp/test",
			wantErr: false,
		},
		{
			name:    "tilde expansion",
			baseDir: "~/test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewOrgBasedStrategy(tt.baseDir)
			paths, err := strategy.Resolve(ctx, "test-cluster", "test-org")
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && paths == nil {
				t.Error("Resolve() returned nil paths")
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, string)
	}{
		{
			name:  "absolute path unchanged",
			input: "/tmp/test",
			validate: func(t *testing.T, result string) {
				if result != "/tmp/test" {
					t.Errorf("expandPath() = %s, want /tmp/test", result)
				}
			},
		},
		{
			name:  "tilde expansion",
			input: "~/test",
			validate: func(t *testing.T, result string) {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					t.Skip("cannot get home directory")
				}
				expected := filepath.Join(homeDir, "test")
				if result != expected {
					t.Errorf("expandPath() = %s, want %s", result, expected)
				}
			},
		},
		{
			name:  "environment variable expansion",
			input: "$HOME/test",
			validate: func(t *testing.T, result string) {
				homeDir := os.Getenv("HOME")
				if homeDir == "" {
					t.Skip("HOME environment variable not set")
				}
				expected := filepath.Join(homeDir, "test")
				if result != expected {
					t.Errorf("expandPath() = %s, want %s", result, expected)
				}
			},
		},
		{
			name:  "allowlisted config dir expansion",
			input: "${OPENCENTER_CONFIG_DIR}/test",
			validate: func(t *testing.T, result string) {
				configDir := t.TempDir()
				t.Setenv("OPENCENTER_CONFIG_DIR", configDir)

				expected := filepath.Join(configDir, "test")
				actual := ExpandPath("${OPENCENTER_CONFIG_DIR}/test")
				if actual != expected {
					t.Errorf("ExpandPath() = %s, want %s", actual, expected)
				}
			},
		},
		{
			name:  "allowlisted state dir expansion",
			input: "${OPENCENTER_STATE_DIR}/test",
			validate: func(t *testing.T, result string) {
				stateDir := t.TempDir()
				t.Setenv("OPENCENTER_STATE_DIR", stateDir)

				expected := filepath.Join(stateDir, "test")
				actual := ExpandPath("${OPENCENTER_STATE_DIR}/test")
				if actual != expected {
					t.Errorf("ExpandPath() = %s, want %s", actual, expected)
				}
			},
		},
		{
			name:  "allowlisted xdg state home expansion",
			input: "${XDG_STATE_HOME}/test",
			validate: func(t *testing.T, result string) {
				stateHome := t.TempDir()
				t.Setenv("XDG_STATE_HOME", stateHome)

				expected := filepath.Join(stateHome, "test")
				actual := ExpandPath("${XDG_STATE_HOME}/test")
				if actual != expected {
					t.Errorf("ExpandPath() = %s, want %s", actual, expected)
				}
			},
		},
		{
			name:  "non-allowlisted environment variable remains literal",
			input: "${AWS_SECRET_ACCESS_KEY}/test",
			validate: func(t *testing.T, result string) {
				if result != "${AWS_SECRET_ACCESS_KEY}/test" {
					t.Errorf("ExpandPath() = %s, want literal path", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			tt.validate(t, result)
		})
	}
}

// Additional comprehensive tests for OrgBasedStrategy

func TestOrgBasedStrategy_MultipleOrganizations(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Setup multiple organizations with clusters
	orgs := []string{"org1", "org2", "org3"}
	clusters := []string{"cluster-a", "cluster-b"}

	for _, org := range orgs {
		for _, cluster := range clusters {
			clusterDir := filepath.Join(tmpDir, org, "infrastructure", "clusters", cluster)
			if err := os.MkdirAll(clusterDir, 0755); err != nil {
				t.Fatal(err)
			}
		}
	}

	strategy := NewOrgBasedStrategy(tmpDir)

	// Test that each organization can resolve its clusters
	for _, org := range orgs {
		for _, cluster := range clusters {
			canResolve, err := strategy.CanResolve(ctx, cluster, org)
			if err != nil {
				t.Errorf("CanResolve(%s, %s) error = %v", cluster, org, err)
			}
			if !canResolve {
				t.Errorf("CanResolve(%s, %s) = false, want true", cluster, org)
			}

			paths, err := strategy.Resolve(ctx, cluster, org)
			if err != nil {
				t.Errorf("Resolve(%s, %s) error = %v", cluster, org, err)
			}
			if paths == nil {
				t.Errorf("Resolve(%s, %s) returned nil paths", cluster, org)
			}
		}
	}
}

func TestOrgBasedStrategy_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	tests := []struct {
		name         string
		setup        func()
		clusterName  string
		organization string
		wantResolve  bool
		wantErr      bool
	}{
		{
			name: "cluster with special characters in name",
			setup: func() {
				clusterDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", "test_cluster-123")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
			},
			clusterName:  "test_cluster-123",
			organization: "test-org",
			wantResolve:  true,
			wantErr:      false,
		},
		{
			name: "organization with hyphens",
			setup: func() {
				clusterDir := filepath.Join(tmpDir, "my-test-org", "infrastructure", "clusters", "cluster1")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
			},
			clusterName:  "cluster1",
			organization: "my-test-org",
			wantResolve:  true,
			wantErr:      false,
		},
		{
			name: "organization with underscores",
			setup: func() {
				clusterDir := filepath.Join(tmpDir, "my_test_org", "infrastructure", "clusters", "cluster2")
				if err := os.MkdirAll(clusterDir, 0755); err != nil {
					t.Fatal(err)
				}
			},
			clusterName:  "cluster2",
			organization: "my_test_org",
			wantResolve:  true,
			wantErr:      false,
		},
		{
			name:         "empty organization defaults to opencenter",
			setup:        func() {},
			clusterName:  "test-cluster",
			organization: "",
			wantResolve:  false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			strategy := NewOrgBasedStrategy(tmpDir)

			canResolve, err := strategy.CanResolve(ctx, tt.clusterName, tt.organization)
			if (err != nil) != tt.wantErr {
				t.Errorf("CanResolve() error = %v, wantErr %v", err, tt.wantErr)
			}
			if canResolve != tt.wantResolve {
				t.Errorf("CanResolve() = %v, want %v", canResolve, tt.wantResolve)
			}
		})
	}
}

func TestOrgBasedStrategy_PathConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	strategy := NewOrgBasedStrategy(tmpDir)

	clusterName := "test-cluster"
	organization := "test-org"

	// Resolve paths multiple times
	paths1, err := strategy.Resolve(ctx, clusterName, organization)
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}

	paths2, err := strategy.Resolve(ctx, clusterName, organization)
	if err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}

	// Verify paths are consistent
	if paths1.OrganizationDir != paths2.OrganizationDir {
		t.Errorf("OrganizationDir inconsistent: %s != %s", paths1.OrganizationDir, paths2.OrganizationDir)
	}
	if paths1.ClusterDir != paths2.ClusterDir {
		t.Errorf("ClusterDir inconsistent: %s != %s", paths1.ClusterDir, paths2.ClusterDir)
	}
	if paths1.ApplicationsDir != paths2.ApplicationsDir {
		t.Errorf("ApplicationsDir inconsistent: %s != %s", paths1.ApplicationsDir, paths2.ApplicationsDir)
	}
	if paths1.SecretsDir != paths2.SecretsDir {
		t.Errorf("SecretsDir inconsistent: %s != %s", paths1.SecretsDir, paths2.SecretsDir)
	}
	if paths1.SOPSKeyPath != paths2.SOPSKeyPath {
		t.Errorf("SOPSKeyPath inconsistent: %s != %s", paths1.SOPSKeyPath, paths2.SOPSKeyPath)
	}
}

func TestOrgBasedStrategy_AllPathsPopulated(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	strategy := NewOrgBasedStrategy(tmpDir)

	paths, err := strategy.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Verify all paths are populated (non-empty)
	if paths.OrganizationDir == "" {
		t.Error("OrganizationDir is empty")
	}
	if paths.GitOpsDir == "" {
		t.Error("GitOpsDir is empty")
	}
	if paths.ClusterDir == "" {
		t.Error("ClusterDir is empty")
	}
	if paths.ApplicationsDir == "" {
		t.Error("ApplicationsDir is empty")
	}
	if paths.SecretsDir == "" {
		t.Error("SecretsDir is empty")
	}
	if paths.SOPSKeyPath == "" {
		t.Error("SOPSKeyPath is empty")
	}
	if paths.SOPSConfigPath == "" {
		t.Error("SOPSConfigPath is empty")
	}
	if paths.KubeconfigPath == "" {
		t.Error("KubeconfigPath is empty")
	}
	if paths.InventoryPath == "" {
		t.Error("InventoryPath is empty")
	}
	if paths.VenvPath == "" {
		t.Error("VenvPath is empty")
	}
	if paths.BinPath == "" {
		t.Error("BinPath is empty")
	}
	if paths.ConfigPath == "" {
		t.Error("ConfigPath is empty")
	}
	if paths.SSHKeyPath == "" {
		t.Error("SSHKeyPath is empty")
	}
}

func TestOrgBasedStrategy_PathStructure(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	strategy := NewOrgBasedStrategy(tmpDir)

	clusterName := "my-cluster"
	organization := "my-org"

	paths, err := strategy.Resolve(ctx, clusterName, organization)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Verify path structure follows organization-based layout
	expectedOrgDir := filepath.Join(tmpDir, organization)
	if paths.OrganizationDir != expectedOrgDir {
		t.Errorf("OrganizationDir = %s, want %s", paths.OrganizationDir, expectedOrgDir)
	}

	// Verify ClusterDir is under infrastructure/clusters
	if !filepath.HasPrefix(paths.ClusterDir, filepath.Join(expectedOrgDir, "infrastructure", "clusters")) {
		t.Errorf("ClusterDir %s is not under infrastructure/clusters", paths.ClusterDir)
	}

	// Verify ApplicationsDir is under applications/overlays
	if !filepath.HasPrefix(paths.ApplicationsDir, filepath.Join(expectedOrgDir, "applications", "overlays")) {
		t.Errorf("ApplicationsDir %s is not under applications/overlays", paths.ApplicationsDir)
	}

	// Verify SecretsDir is under organization
	if !filepath.HasPrefix(paths.SecretsDir, expectedOrgDir) {
		t.Errorf("SecretsDir %s is not under organization directory", paths.SecretsDir)
	}

	// Verify SOPSKeyPath is under secrets/age/keys
	if !filepath.HasPrefix(paths.SOPSKeyPath, filepath.Join(expectedOrgDir, "secrets", "age", "keys")) {
		t.Errorf("SOPSKeyPath %s is not under secrets/age/keys", paths.SOPSKeyPath)
	}
}
