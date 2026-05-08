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
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
)

func newClusterValidateManifestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-manifests [cluster-name]",
		Short: "Validate generated GitOps manifests for common issues",
		Long: `Validate generated GitOps manifests for common issues documented in lessons-learned.

This command checks for:
- Proper YAML indentation (2 spaces)
- Correct interval values (5m for Kustomizations, 15m for GitRepositories)
- No hardcoded cluster names (dev-cluster, stage-cluster)
- Proper repository URL capitalization (openCenter not opencenter)
- Branch-based refs (not tag-based)
- Base64 encoded secrets
- Correct domain names and hostname formats
- Proper snapshotter versions and registries
- Valid IP address ranges

Examples:
  # Validate manifests for current cluster
  opencenter cluster validate --manifests

  # Validate manifests for specific cluster
  opencenter cluster validate my-cluster --manifests
`,
		RunE: runClusterValidateManifests,
	}

	cmd.Flags().String("repo-path", "", "Path to GitOps repository to validate")
	cmd.Flags().Bool("staged", false, "Validate staged Git blobs instead of worktree files")
	cmd.Flags().Bool("security-only", false, "Run only GitOps secret and credential scanning")

	return cmd
}

func runClusterValidateManifests(cmd *cobra.Command, args []string) error {
	repoPath, _ := cmd.Flags().GetString("repo-path")
	staged, _ := cmd.Flags().GetBool("staged")
	securityOnly, _ := cmd.Flags().GetBool("security-only")

	if strings.TrimSpace(repoPath) != "" {
		return runGitOpsManifestValidationForRepo(cmd, repoPath, staged, securityOnly)
	}

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterNameForCommand(cmd, args, true)
	if err != nil {
		return err
	}

	// Load configuration
	cfg, err := loadConfig(cmd.Context(), name)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get git directory
	gitDir := cfg.GitDir()
	if gitDir == "" {
		return fmt.Errorf("git_dir not configured")
	}

	fmt.Printf("Validating GitOps manifests in: %s\n\n", gitDir)

	// Create validator
	validator := gitops.NewManifestValidator(gitDir)

	// Run validation
	if err := validator.Validate(); err != nil {
		fmt.Printf("❌ Validation failed:\n\n%v\n", err)
		return fmt.Errorf("manifest validation failed")
	}

	fmt.Printf("✅ All manifests validated successfully\n")
	return nil
}

func runGitOpsManifestValidationForRepo(cmd *cobra.Command, repoPath string, staged bool, securityOnly bool) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Validating GitOps repository: %s\n\n", repoPath)

	if securityOnly {
		findings, err := gitops.ScanGitOpsSecretsWithOptions(context.Background(), gitops.SecretScanOptions{
			Root:   repoPath,
			Staged: staged,
		})
		if err != nil {
			return fmt.Errorf("security scan failed: %w", err)
		}
		if len(findings) > 0 {
			for _, finding := range findings {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s: %s\n", finding.Path, finding.Rule, finding.Message)
			}
			return fmt.Errorf("GitOps security validation failed with %d finding(s)", len(findings))
		}
		fmt.Fprintln(cmd.OutOrStdout(), "GitOps security validation passed")
		return nil
	}

	validator := gitops.NewManifestValidator(repoPath)
	if err := validator.Validate(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Validation failed:\n\n%v\n", err)
		return fmt.Errorf("manifest validation failed")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "All manifests validated successfully")
	return nil
}
