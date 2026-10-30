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

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/spf13/cobra"
)

// resolveGitopsAuthMethod resolves the effective GitOps auth method from the
// --gitops-auth flag, falling back to cluster_defaults.gitops_auth_method from
// the CLI settings, then to the built-in default ("token"). Settings errors
// are returned rather than silently changing the selected authentication mode.
func resolveGitopsAuthMethod(cmd *cobra.Command) (string, error) {
	flagVal, err := cmd.Flags().GetString("gitops-auth")
	if err != nil {
		return "", fmt.Errorf("reading --gitops-auth: %w", err)
	}

	if strings.TrimSpace(flagVal) != "" {
		return resolveGitopsAuthMethodValues(flagVal, "")
	}

	cm, err := config.NewConfigManager("")
	if err != nil {
		return "", fmt.Errorf("loading CLI settings for gitops auth: %w", err)
	}
	return resolveGitopsAuthMethodValues("", cm.GetConfig().ClusterDefaults.GitopsAuthMethod)
}

// resolveGitopsAuthMethodValues contains the deterministic precedence and
// validation rules independently of settings I/O so callers and tests cannot
// accidentally apply different behavior.
func resolveGitopsAuthMethodValues(flagValue, configuredValue string) (string, error) {
	method := strings.ToLower(strings.TrimSpace(flagValue))
	if method == "" {
		method = strings.ToLower(strings.TrimSpace(configuredValue))
	}
	if method == "" {
		method = config.GitopsAuthMethodToken
	}
	if err := config.ValidateGitopsAuthMethod(method); err != nil {
		if strings.TrimSpace(flagValue) != "" {
			return "", fmt.Errorf("invalid --gitops-auth value: %w", err)
		}
		return "", err
	}
	return method, nil
}

// applyGitopsAuthOverride adjusts the in-memory generation config only. It
// carries the selected method explicitly to renderers and derives the base
// repository URL scheme from the same value.
func applyGitopsAuthOverride(cfg *v2.Config, authMethod string) {
	cfg.OpenCenter.GitOps.ResolvedAuthMethod = authMethod
	switch authMethod {
	case config.GitopsAuthMethodSSH:
		cfg.OpenCenter.GitOps.BaseRepo.URL = v2.DefaultGitBaseRepoURLSSH
	default:
		cfg.OpenCenter.GitOps.BaseRepo.URL = v2.DefaultGitBaseRepoURLHTTPS
	}
}

// newClusterGenerateCmd creates the command for generating a cluster's GitOps repository.
func newClusterGenerateCmd() *cobra.Command {
	var renderOnly bool

	cmd := &cobra.Command{
		Use:   "generate [name]",
		Short: "Generate the GitOps repository and rendered manifests",
		Long: `Generate the customer GitOps repository and rendered manifests for a cluster.

This command creates or updates the repository structure, infrastructure templates,
Flux manifests, and application overlays based on the cluster configuration.

Use --render-only to render templates without running the full repository setup flow.
Use --gitops-auth to select the GitOps authentication method for the base repository
sources (ssh or token). Defaults to cluster_defaults.gitops_auth_method from settings.

base_domain provenance: a cluster whose opencenter.cluster.managed.base_domain is
"default" follows cluster_defaults.base_domain from settings automatically at every
load. A pinned (or hand-edited) domain is never rewritten; generate warns when the
settings domain changed since the last successful generation. Use --sync-base-domain
to adopt the settings domain into a pinned cluster config (rewrites the derived
cluster_fqdn).`,
		Example: `  # Generate assets for the active cluster
  opencenter cluster generate

  # Generate assets for a specific cluster
  opencenter cluster generate my-cluster

  # Preview what would be generated
  opencenter cluster generate my-cluster --dry-run

  # Render templates only
  opencenter cluster generate my-cluster --render-only

  # Generate with explicit GitOps auth method
  opencenter cluster generate my-cluster --gitops-auth=token
  opencenter cluster generate my-cluster --gitops-auth=ssh

  # Adopt a changed cluster_defaults.base_domain into the cluster config
  opencenter cluster generate my-cluster --sync-base-domain`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if renderOnly {
				return runClusterGenerateRenderOnly(cmd, args)
			}
			return runClusterGenerate(cmd, args)
		},
	}

	cmd.Flags().Bool("force", false, "overwrite existing GitOps repository")
	cmd.Flags().Bool("prune", true, "remove stale generated files (use --prune=false to report but retain them)")
	cmd.Flags().Bool("adopt-generated", false, "claim differing planned files after creating backups")
	cmd.Flags().Bool("skip-validation", false, "skip configuration validation before generation")
	cmd.Flags().BoolVar(&renderOnly, "render-only", false, "render templates without running repository setup")
	cmd.Flags().String("gitops-auth", "", "GitOps authentication method for base repo sources (ssh, token); defaults to cluster_defaults.gitops_auth_method")
	cmd.Flags().Bool("sync-base-domain", false, "adopt cluster_defaults.base_domain from settings into the cluster config (rewrites the derived cluster_fqdn) before generating")

	return cmd
}

func runClusterGenerate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve gitops auth method early so we fail fast on invalid values.
	gitopsAuth, err := resolveGitopsAuthMethod(cmd)
	if err != nil {
		return err
	}

	// Resolve cluster name from args or active cluster
	name, err := resolveClusterNameForCommand(cmd, args, true)
	if err != nil {
		return err
	}

	organization := ""

	// Reject planned providers that are not yet available
	cfg, err := loadCanonicalConfig(name)
	if err == nil {
		organization = cfg.OpenCenter.Meta.Organization
		if err := checkProviderAvailability(cfg.OpenCenter.Infrastructure.Provider); err != nil {
			return err
		}
	}

	// Extract just the cluster name (without organization prefix) for path resolution
	actualClusterName := extractClusterName(name)

	app, err := GetApp(cmd.Context())
	if err != nil {
		return err
	}
	setupService := app.SetupService

	// Parse flags into SetupOptions
	force, _ := cmd.Flags().GetBool("force")
	prune, _ := cmd.Flags().GetBool("prune")
	adoptGenerated, _ := cmd.Flags().GetBool("adopt-generated")
	dryRun := getGlobalOptions(cmd).DryRun
	skipValidation, _ := cmd.Flags().GetBool("skip-validation")
	syncBaseDomain, _ := cmd.Flags().GetBool("sync-base-domain")

	opts := cluster.SetupOptions{
		ClusterName:      actualClusterName,
		Organization:     organization,
		DryRun:           dryRun,
		SkipValidation:   skipValidation,
		Force:            force,
		Prune:            &prune,
		AdoptGenerated:   adoptGenerated,
		GitopsAuthMethod: gitopsAuth,
		SyncBaseDomain:   syncBaseDomain,
	}

	if !dryRun {
		if err := config.UpdateStatus(name, v2.StageSetup, v2.StatusRunning); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
	}

	// Execute setup
	result, err := setupService.Setup(ctx, opts)
	if err != nil {
		if !dryRun {
			if statusErr := config.UpdateStatus(name, v2.StageSetup, v2.StatusFailed); statusErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", statusErr)
			}
		}
		return err
	}

	// Print result summary
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Generate dry-run complete\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Generate complete\n")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "GitOps path:       %s\n", result.GitOpsPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Manifests created: %d\n", result.ManifestsCreated)
	if result.BaseDomainSynced {
		fmt.Fprintf(cmd.OutOrStdout(), "Base domain synced:  %s -> %s (cluster config updated)\n",
			result.BaseDomainSyncedFrom, result.BaseDomainSyncedTo)
	}
	if len(result.RepairedHostnameFields) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nHostname repair (values were still the `cluster init` placeholder identity):\n")
		for _, field := range result.RepairedHostnameFields {
			fmt.Fprintf(cmd.OutOrStdout(), "  ↻ %s\n", field)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Re-run 'opencenter secrets sync %s' if a certificate DNS name changed.\n", name)
	}

	// Report warnings (non-fatal)
	if len(result.Warnings) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nWarnings:\n")
		for _, w := range result.Warnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ⚠ %s\n", w)
		}
	}

	if !dryRun {
		// Stamp the rendered base_domain so the next generate can tell a
		// settings change from a deliberate cluster-side edit. Non-fatal: a
		// stamp failure only degrades drift classification.
		if stampErr := config.StampGeneratedBaseDomain(name, result.BaseDomainDrift.RenderedBaseDomain); stampErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to stamp generated base domain: %v\n", stampErr)
		}
		if err := config.UpdateStatus(name, v2.StageSetup, v2.StatusSuccess); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter secrets sync %s       # encrypt secrets\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster validate %s   # verify readiness\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster deploy %s     # deploy cluster\n", name)
	}

	return nil
}
