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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/gitops"
	"github.com/rackerlabs/openCenter-cli/internal/sops"
	// main.tf rendering is handled in tofu.Provision now
	"github.com/rackerlabs/openCenter-cli/internal/tofu"
	"github.com/spf13/cobra"
)

func newClusterSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [name]",
		Short: "Setup GitOps directory (depends on cluster init)",
		Long: `Setup GitOps directory structure for an initialized cluster.

This command performs GitOps repository setup for a cluster that has already been
initialized with 'cluster init'. It includes:
- Rendering all templates to organization structure
- Setting up SOPS configuration using existing keys
- Creating cluster-specific manifests

Prerequisites:
- Cluster must be initialized first with 'cluster init'
- SOPS keys must exist (created during init)
- Organization directory structure must exist (created during init)

For iterative development and template re-rendering, use 'cluster render' instead.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				var err error
				name, err = config.GetActive()
				if err != nil {
					return err
				}
				if name == "" {
					return fmt.Errorf("no active cluster; specify name")
				}
			}

			// Load configuration (validation should be done separately via 'cluster validate')
			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			// Flags
			render, _ := cmd.Flags().GetBool("render")
			force, _ := cmd.Flags().GetBool("force")

			// Validate git_dir is set before proceeding with setup
			if err := validateGitDir(cfg); err != nil {
				return err
			}

			// Get organization from cluster metadata
			organization := cfg.OpenCenter.Meta.Organization
			if organization == "" {
				organization = "opencenter"
			}

			// Initialize organization-based GitOps setup
			if err := setupOrganizationGitOps(cfg, organization, render, force, cmd); err != nil {
				return fmt.Errorf("failed to setup organization GitOps: %w", err)
			}

			// Update stage and status
			if err := config.UpdateStatus(name, config.StageSetup, config.StatusSuccess); err != nil {
				// Don't fail the command if status update fails, just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Setup complete.")
			return nil
		},
	}
	cmd.Flags().Bool("render", false, "render templates (rather than copy)")
	cmd.Flags().Bool("force", false, "overwrite existing files and reinitialize")
	return cmd
}

// setupOrganizationGitOps sets up the organization-based GitOps repository structure.
// This function implements the enhanced GitOps integration with organization support.
func setupOrganizationGitOps(cfg config.Config, organization string, render, force bool, cmd *cobra.Command) error {
	// Get CLI configuration manager for path resolution
	cliConfigManager, err := config.NewConfigManager("")
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create path resolver for organization-based paths
	pathResolver := config.NewPathResolver(cliConfigManager)

	// Get organization-based paths
	clusterName := cfg.ClusterName()
	paths := pathResolver.ResolveClusterPaths(clusterName, organization)

	// Update configuration to use organization-based GitOps directory
	// Only override git_dir if it's not explicitly set to a custom path
	updatedCfg := cfg
	originalGitDir := cfg.GitOps().GitDir

	// If user specified a custom git_dir, use it; otherwise use organization-based path
	if originalGitDir != "" && originalGitDir != paths.GitOpsDir {
		// User has specified a custom git_dir, use it instead of organization path
		updatedCfg.OpenCenter.GitOps.GitDir = originalGitDir
	} else {
		// Use organization-based path
		updatedCfg.OpenCenter.GitOps.GitDir = paths.GitOpsDir
	}

	// Validate that cluster init has been run first
	if err := validateClusterInitCompleted(clusterName, organization, paths); err != nil {
		return fmt.Errorf("cluster init required: %w", err)
	}

	// Check if already initialized unless force is specified
	if !force {
		initialized, err := gitops.IsGitOpsInitialized(updatedCfg.GitOps().GitDir)
		if err != nil {
			return fmt.Errorf("failed to check if GitOps is initialized: %w", err)
		}
		if initialized {
			fmt.Fprintln(cmd.OutOrStdout(), "GitOps directory already initialized")
			fmt.Fprintf(cmd.OutOrStdout(), "GitOps directory: %s\n", updatedCfg.GitOps().GitDir)
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "Options:")
			fmt.Fprintln(cmd.OutOrStdout(), "  --force                    Reinitialize and overwrite existing files")
			fmt.Fprintln(cmd.OutOrStdout(), "  openCenter cluster render  Render templates without Git operations (recommended for development)")
			return nil
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Force flag set: reinitializing GitOps directory")
	}

	// Setup GitOps base structure at organization level
	if err := gitops.CopyBase(updatedCfg, render); err != nil {
		return fmt.Errorf("failed to prepare gitops directory: %w", err)
	}

	// Render cluster-specific templates to organization structure
	if err := gitops.RenderClusterApps(updatedCfg); err != nil {
		return fmt.Errorf("failed to render cluster apps templates: %w", err)
	}

	if err := gitops.RenderInfrastructureCluster(updatedCfg); err != nil {
		return fmt.Errorf("failed to render infrastructure cluster templates: %w", err)
	}

	// Provision OpenTofu (renders main.tf and provider.tf)
	if err := tofu.Provision(updatedCfg); err != nil {
		return fmt.Errorf("failed to provision opentofu: %w", err)
	}

	// Setup shared SOPS configuration at organization level using existing keys
	if err := setupOrganizationSOPS(cfg, paths, organization); err != nil {
		return fmt.Errorf("failed to setup organization SOPS: %w", err)
	}

	// Write .opencenter marker with cluster name
	gitDir := updatedCfg.GitOps().GitDir
	markerPath := filepath.Join(gitDir, ".opencenter")
	markerContent := fmt.Sprintf("organization: %s\nclusters:\n  - %s\n", organization, clusterName)

	// If marker exists, update it to include this cluster
	if existingContent, err := os.ReadFile(markerPath); err == nil {
		// Parse existing content and add cluster if not present
		content := string(existingContent)
		if !strings.Contains(content, clusterName) {
			markerContent = content + "  - " + clusterName + "\n"
		} else {
			markerContent = content // Keep existing content if cluster already listed
		}
	}

	if err := os.WriteFile(markerPath, []byte(markerContent), 0o644); err != nil {
		return fmt.Errorf("failed to write .opencenter marker: %w", err)
	}

	return nil
}

// validateClusterInitCompleted validates that cluster init has been run first.
// It checks for the existence of required files and directories created during init.
func validateClusterInitCompleted(clusterName, organization string, paths config.ClusterPaths) error {
	// Check if cluster configuration file exists at organization level
	configPath := filepath.Join(paths.OrganizationDir, "."+clusterName+"-config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("cluster configuration not found at %s\n\n"+
			"Please run 'openCenter cluster init %s' first to create the cluster configuration", 
			configPath, clusterName)
	}

	// Check if SOPS key exists
	if _, err := os.Stat(paths.SOPSKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("SOPS key not found at %s\n\n"+
			"Please run 'openCenter cluster init %s' first to generate the required SOPS key", 
			paths.SOPSKeyPath, clusterName)
	}

	// Check if organization directory structure exists
	if _, err := os.Stat(paths.OrganizationDir); os.IsNotExist(err) {
		return fmt.Errorf("organization directory not found at %s\n\n"+
			"Please run 'openCenter cluster init %s' first to create the organization structure", 
			paths.OrganizationDir, clusterName)
	}

	return nil
}

// setupOrganizationSOPS sets up shared SOPS configuration for the organization.
func setupOrganizationSOPS(cfg config.Config, paths config.ClusterPaths, organization string) error {
	// Create SOPS key manager for organization secrets directory
	keyManager := sops.NewKeyManager(filepath.Dir(paths.SOPSKeyPath))

	clusterName := cfg.ClusterName()

	// Load existing SOPS key (created during cluster init)
	keyName := clusterName + "-key"
	keyPair, err := keyManager.LoadAgeKey(keyName)
	if err != nil {
		return fmt.Errorf("SOPS key for cluster %s not found: %v\n\n"+
			"Please run 'openCenter cluster init %s' first to create the required SOPS key, "+
			"or ensure the key exists at: %s", clusterName, err, clusterName, paths.SOPSKeyPath)
	}

	fmt.Printf("Using existing SOPS key for cluster %s\n", clusterName)

	// Create or update organization-wide SOPS configuration
	if err := createOrganizationSOPSConfigForSetup(paths.SOPSConfigPath, organization, clusterName, keyPair.PublicKey); err != nil {
		return fmt.Errorf("failed to create organization SOPS config: %w", err)
	}

	return nil
}

// createOrganizationSOPSConfigForSetup creates or updates the organization-wide .sops.yaml configuration.
// Each cluster's key only encrypts files in its specific directories:
// - /applications/overlays/<cluster>/
// - /infrastructure/clusters/<cluster>/
func createOrganizationSOPSConfigForSetup(configPath, organization, clusterName, publicKey string) error {
	// Define the path patterns for this cluster
	clusterRule := fmt.Sprintf(`  - path_regex: (applications/overlays/%s/.*|infrastructure/clusters/%s/.*)\.ya?ml$
    age: >-
      %s`, clusterName, clusterName, publicKey)

	// Check if .sops.yaml already exists
	var existingContent string
	if data, err := os.ReadFile(configPath); err == nil {
		existingContent = string(data)
	}

	// Check if this cluster already has a rule with the same key
	clusterRulePattern := fmt.Sprintf(`path_regex: (applications/overlays/%s/.*|infrastructure/clusters/%s/`, clusterName, clusterName)
	if strings.Contains(existingContent, clusterRulePattern) && strings.Contains(existingContent, publicKey) {
		// Rule already exists with the same key, no need to update
		return nil
	}

	var sopsConfig string
	if existingContent == "" {
		// Create new .sops.yaml with header and first cluster rule
		sopsConfig = fmt.Sprintf(`# SOPS configuration for organization
# Each cluster's key encrypts only its specific directories
creation_rules:
%s
`, clusterRule)
	} else if strings.Contains(existingContent, clusterRulePattern) {
		// Cluster rule exists but with different key, update it
		lines := strings.Split(existingContent, "\n")
		var newLines []string
		skipLines := 0

		for _, line := range lines {
			if skipLines > 0 {
				skipLines--
				continue
			}

			// Check if this is the start of our cluster's rule
			if strings.Contains(line, clusterRulePattern) {
				// Add the new rule
				newLines = append(newLines, clusterRule)
				// Skip the next 2 lines (age: and the key value)
				skipLines = 2
				continue
			}

			newLines = append(newLines, line)
		}
		sopsConfig = strings.Join(newLines, "\n")
	} else {
		// Append new cluster rule to existing config
		existingContent = strings.TrimRight(existingContent, "\n")
		sopsConfig = fmt.Sprintf("%s\n%s\n", existingContent, clusterRule)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create SOPS config directory: %w", err)
	}

	// Write the SOPS configuration file at organization root
	if err := os.WriteFile(configPath, []byte(sopsConfig), 0644); err != nil {
		return fmt.Errorf("failed to write SOPS config file: %w", err)
	}

	return nil
}

// extractAgeKey extracts an age key from a SOPS config line.
func extractAgeKey(line string) string {
	// Simple extraction - look for age1 followed by 58 characters
	if idx := strings.Index(line, "age1"); idx >= 0 {
		key := line[idx:]
		if len(key) >= 62 { // age1 + 58 characters
			return key[:62]
		}
	}
	return ""
}

// validateGitDir validates that the git_dir is set and accessible.
// It returns a clear error message if validation fails.
func validateGitDir(cfg config.Config) error {
	gitDir := cfg.GitOps().GitDir

	// Check if git_dir is set
	if gitDir == "" {
		return fmt.Errorf("opencenter.gitops.git_dir must be set\n\n" +
			"The git_dir field specifies where GitOps manifests will be generated.\n" +
			"Please set opencenter.gitops.git_dir in your cluster configuration.\n\n" +
			"Example:\n" +
			"  opencenter:\n" +
			"    gitops:\n" +
			"      git_dir: /path/to/gitops/repo\n\n" +
			"Then run 'openCenter cluster setup' again.")
	}

	// Expand any environment variables or ~ in the path
	expandedPath := os.ExpandEnv(gitDir)
	if strings.HasPrefix(expandedPath, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand home directory in git_dir path: %w", err)
		}
		expandedPath = filepath.Join(homeDir, expandedPath[1:])
	}

	// Check if the parent directory exists and is accessible
	parentDir := filepath.Dir(expandedPath)
	if parentDir != "." && parentDir != "/" {
		if _, err := os.Stat(parentDir); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("parent directory of git_dir does not exist: %s\n\n"+
					"Please ensure the parent directory exists before running setup.\n"+
					"You can create it with: mkdir -p %s", parentDir, parentDir)
			}
			if os.IsPermission(err) {
				return fmt.Errorf("permission denied accessing parent directory of git_dir: %s\n\n"+
					"Please check directory permissions.", parentDir)
			}
			return fmt.Errorf("failed to access parent directory of git_dir: %w", err)
		}
	}

	// If git_dir already exists, check if it's accessible
	if info, err := os.Stat(expandedPath); err == nil {
		// Directory exists, check if it's actually a directory
		if !info.IsDir() {
			return fmt.Errorf("git_dir path exists but is not a directory: %s\n\n"+
				"Please specify a directory path for git_dir.", expandedPath)
		}

		// Check if we can write to it
		testFile := filepath.Join(expandedPath, ".opencenter-test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("permission denied: cannot write to git_dir: %s\n\n"+
					"Please check directory permissions.", expandedPath)
			}
			return fmt.Errorf("git_dir is not writable: %w", err)
		}
		// Clean up test file
		os.Remove(testFile)
	} else if !os.IsNotExist(err) {
		// Some other error occurred (not "does not exist")
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied accessing git_dir: %s\n\n"+
				"Please check directory permissions.", expandedPath)
		}
		return fmt.Errorf("failed to access git_dir: %w", err)
	}
	// If directory doesn't exist, that's fine - we'll create it during setup

	return nil
}
