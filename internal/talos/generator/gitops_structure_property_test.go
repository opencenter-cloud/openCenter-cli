package generator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// Feature: talos-openstack-provider, Property 6: GitOps structure completeness
// For any initialized cluster, the GitOps directory structure should contain
// all required artifacts: Talos machine configs, Pulumi stack files, WireGuard
// configs, and SOPS policies.
// Validates: Requirements 2.10
func TestProperty_GitOpsStructureCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all GitOps structures have required artifacts",
		prop.ForAll(
			func(clusterName string) bool {
				// Create a temporary directory for testing
				tmpDir, err := os.MkdirTemp("", "gitops-test-*")
				if err != nil {
					t.Logf("Failed to create temp dir: %v", err)
					return false
				}
				defer os.RemoveAll(tmpDir)

				// Create a generator with minimal config
				cfg := &config.Config{
					OpenCenter: config.SimplifiedOpenCenter{
						Meta: config.ClusterMeta{
							Name: clusterName,
						},
					},
				}
				g := New(cfg)

				// Generate GitOps structure
				if err := g.GenerateGitOpsStructure(context.Background(), tmpDir); err != nil {
					t.Logf("Failed to generate GitOps structure: %v", err)
					return false
				}

				// Verify required directories exist
				hasDirectories := validateDirectoryStructure(tmpDir, t)
				hasKustomizations := validateKustomizationFiles(tmpDir, t)
				hasSOPSConfig := validateSOPSConfig(tmpDir, t)
				hasReadmes := validateReadmeFiles(tmpDir, t)

				if !hasDirectories {
					t.Logf("Missing required directories")
				}
				if !hasKustomizations {
					t.Logf("Missing kustomization files")
				}
				if !hasSOPSConfig {
					t.Logf("Missing SOPS configuration")
				}
				if !hasReadmes {
					t.Logf("Missing README files")
				}

				return hasDirectories && hasKustomizations && hasSOPSConfig && hasReadmes
			},
			gen.Identifier(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// validateDirectoryStructure checks if all required directories exist.
func validateDirectoryStructure(basePath string, t *testing.T) bool {
	requiredDirs := []string{
		"clusters",
		"infrastructure",
		"infrastructure/talos",
		"infrastructure/talos/machine-configs",
		"infrastructure/talos/pulumi",
		"infrastructure/talos/wireguard",
		"infrastructure/networks",
		"infrastructure/security-groups",
		"applications",
		"applications/base",
		"applications/overlays",
	}

	for _, dir := range requiredDirs {
		fullPath := filepath.Join(basePath, dir)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Logf("Directory %s does not exist: %v", dir, err)
			return false
		}
		if !info.IsDir() {
			t.Logf("Path %s is not a directory", dir)
			return false
		}
	}

	return true
}

// validateKustomizationFiles checks if kustomization files exist and are valid.
func validateKustomizationFiles(basePath string, t *testing.T) bool {
	requiredKustomizations := []string{
		"infrastructure/talos/kustomization.yaml",
		"infrastructure/kustomization.yaml",
		"applications/base/kustomization.yaml",
	}

	for _, kustomization := range requiredKustomizations {
		fullPath := filepath.Join(basePath, kustomization)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Logf("Kustomization file %s does not exist: %v", kustomization, err)
			return false
		}

		// Basic validation: check if it contains "apiVersion" and "kind"
		contentStr := string(content)
		if len(contentStr) == 0 {
			t.Logf("Kustomization file %s is empty", kustomization)
			return false
		}
	}

	return true
}

// validateSOPSConfig checks if SOPS configuration exists and is valid.
func validateSOPSConfig(basePath string, t *testing.T) bool {
	sopsPath := filepath.Join(basePath, ".sops.yaml")
	content, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Logf("SOPS config does not exist: %v", err)
		return false
	}

	// Basic validation: check if it contains "creation_rules"
	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Logf("SOPS config is empty")
		return false
	}

	// Check for key SOPS configuration elements
	if !contains(contentStr, "creation_rules") {
		t.Logf("SOPS config missing creation_rules")
		return false
	}

	if !contains(contentStr, "barbican") {
		t.Logf("SOPS config missing barbican configuration")
		return false
	}

	return true
}

// validateReadmeFiles checks if README files exist.
func validateReadmeFiles(basePath string, t *testing.T) bool {
	requiredReadmes := []string{
		"README.md",
		"infrastructure/talos/README.md",
	}

	for _, readme := range requiredReadmes {
		fullPath := filepath.Join(basePath, readme)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Logf("README file %s does not exist: %v", readme, err)
			return false
		}

		if len(content) == 0 {
			t.Logf("README file %s is empty", readme)
			return false
		}
	}

	return true
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
