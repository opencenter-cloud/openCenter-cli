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
    "os/exec"
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
        Short: "Setup GitOps directory (copy or render templates and initialise git)",
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

            // Get organization from cluster metadata
            organization := cfg.OpenCenter.Meta.Organization
            if organization == "" {
                organization = "opencenter"
            }

            // Initialize organization-based GitOps setup
            if err := setupOrganizationGitOps(cfg, organization, render, force, cmd); err != nil {
                return fmt.Errorf("failed to setup organization GitOps: %w", err)
            }

            fmt.Fprintln(cmd.OutOrStdout(), "Setup complete.")
            return nil
        },
    }
    cmd.Flags().Bool("render", false, "render templates (rather than copy)")
    cmd.Flags().Bool("force", false, "overwrite existing files and reinitialize")
    return cmd
}

// runGit executes a git command in the given directory, streaming
// output to the provided cobra.Command's stdout/stderr. It returns an
// error if the command fails.
func runGit(dir string, args []string, cmd *cobra.Command) error {
    g := exec.Command("git", args...)
    g.Dir = dir
    g.Stdout = cmd.OutOrStdout()
    g.Stderr = cmd.ErrOrStderr()
    return g.Run()
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

    // Check if already initialized unless force is specified
    if !force {
        if _, err := os.Stat(filepath.Join(paths.GitOpsDir, ".git")); err == nil {
            if _, err2 := os.Stat(filepath.Join(paths.GitOpsDir, ".opencenter")); err2 == nil {
                fmt.Fprintln(cmd.OutOrStdout(), "already initialized")
                return nil
            }
        }
    }

    // Create organization directory structure
    if err := pathResolver.CreateOrganizationStructure(organization); err != nil {
        return fmt.Errorf("failed to create organization structure: %w", err)
    }

    // Create cluster-specific directories
    if err := pathResolver.CreateClusterDirectories(clusterName, organization); err != nil {
        return fmt.Errorf("failed to create cluster directories: %w", err)
    }

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

    // Setup shared SOPS configuration at organization level
    if err := setupOrganizationSOPS(cfg, paths, organization); err != nil {
        return fmt.Errorf("failed to setup organization SOPS: %w", err)
    }

    // Initialize Git repository at the configured git_dir location
    gitDir := updatedCfg.GitOps().GitDir
    if err := initializeOrganizationGitRepo(gitDir, cmd); err != nil {
        return fmt.Errorf("failed to initialize git repository: %w", err)
    }

    // Write .opencenter marker with cluster name
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

    // Add and commit changes
    if err := runGit(gitDir, []string{"add", "."}, cmd); err != nil {
        return fmt.Errorf("failed to add files to git: %w", err)
    }

    commitMsg := fmt.Sprintf("Setup cluster %s in organization %s", clusterName, organization)
    if err := runGit(gitDir, []string{"commit", "-m", commitMsg, "--allow-empty"}, cmd); err != nil {
        return fmt.Errorf("failed to commit changes: %w", err)
    }

    return nil
}

// setupOrganizationSOPS sets up shared SOPS configuration for the organization.
func setupOrganizationSOPS(cfg config.Config, paths config.ClusterPaths, organization string) error {
    // Create SOPS key manager for organization secrets directory
    keyManager := sops.NewKeyManager(filepath.Dir(paths.SOPSKeyPath))
    
    // Generate or load SOPS key for this cluster
    clusterName := cfg.ClusterName()
    keyPair, err := keyManager.GenerateKeyForCluster(clusterName)
    if err != nil {
        // If key generation fails, try to load existing key
        if existingKey, loadErr := keyManager.LoadAgeKey(clusterName); loadErr == nil {
            keyPair = existingKey
        } else {
            return fmt.Errorf("failed to generate or load SOPS key for cluster %s: %w", clusterName, err)
        }
    }

    // Create or update organization-wide SOPS configuration
    if err := createOrganizationSOPSConfigForSetup(paths.SOPSConfigPath, organization, clusterName, keyPair.PublicKey); err != nil {
        return fmt.Errorf("failed to create organization SOPS config: %w", err)
    }

    return nil
}

// createOrganizationSOPSConfigForSetup creates or updates the organization-wide .sops.yaml configuration.
func createOrganizationSOPSConfigForSetup(configPath, organization, clusterName, publicKey string) error {
    // Check if SOPS config already exists
    var existingKeys []string
    if existingData, err := os.ReadFile(configPath); err == nil {
        // Parse existing config to extract age keys
        content := string(existingData)
        // Simple parsing to extract existing age keys
        lines := strings.Split(content, "\n")
        for _, line := range lines {
            if strings.Contains(line, "age1") && !strings.Contains(line, publicKey) {
                // Extract the age key from the line
                if key := extractAgeKey(line); key != "" {
                    existingKeys = append(existingKeys, key)
                }
            }
        }
    }

    // Add current cluster's key if not already present
    allKeys := append(existingKeys, publicKey)
    
    // Remove duplicates
    uniqueKeys := make([]string, 0, len(allKeys))
    seen := make(map[string]bool)
    for _, key := range allKeys {
        if !seen[key] {
            uniqueKeys = append(uniqueKeys, key)
            seen[key] = true
        }
    }

    // Generate SOPS configuration content
    config := fmt.Sprintf(`# SOPS configuration for organization: %s
# This configuration is shared across all clusters in the organization
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: >-
      %s
    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
  - path_regex: .*\.json$
    age: >-
      %s
    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
`, organization, strings.Join(uniqueKeys, ",\n      "), strings.Join(uniqueKeys, ",\n      "))

    // Ensure directory exists
    if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
        return fmt.Errorf("failed to create SOPS config directory: %w", err)
    }

    // Write SOPS configuration
    if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
        return fmt.Errorf("failed to write SOPS config: %w", err)
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

// initializeOrganizationGitRepo initializes the Git repository at the organization level.
func initializeOrganizationGitRepo(gitDir string, cmd *cobra.Command) error {
    // Check if git repo already exists
    if _, err := os.Stat(filepath.Join(gitDir, ".git")); err == nil {
        return nil // Already initialized
    }

    // Initialize git repository
    cmdGit := exec.Command("git", "init", "-b", "main")
    cmdGit.Dir = gitDir
    cmdGit.Stdout = cmd.OutOrStdout()
    cmdGit.Stderr = cmd.ErrOrStderr()
    if err := cmdGit.Run(); err != nil {
        return fmt.Errorf("git init failed: %w", err)
    }

    // Create .gitignore for organization
    gitignoreContent := `# SOPS-related files
.sops.yaml.bak
*.dec
*.dec.*
*.tmp

# Terraform/OpenTofu files
*.tfstate
*.tfstate.*
.terraform/
.terraform.lock.hcl

# IDE and editor files
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# Local development files
.env
.env.local
`

    gitignorePath := filepath.Join(gitDir, ".gitignore")
    if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
        return fmt.Errorf("failed to create .gitignore: %w", err)
    }

    fmt.Fprintln(cmd.OutOrStdout(), "Created GitOps repo")
    return nil
}
