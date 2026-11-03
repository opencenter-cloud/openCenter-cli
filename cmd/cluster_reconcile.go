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
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/spf13/cobra"
)

func newClusterReconcileCmd() *cobra.Command {
	var (
		services   []string
		all        bool
		message    string
		remote     string
		pushOnly   bool
		skipFlux   bool
		fluxOnly   bool
		generate   bool
		wait       bool
		timeout    time.Duration
		kubeconfig string
	)

	cmd := &cobra.Command{
		Use:   "reconcile [name]",
		Short: "Push GitOps manifests and reconcile Flux resources",
		Long: `Stage, validate, commit, and push cluster GitOps changes to remote repository,
and reconcile affected Flux resources in-cluster.

This command provides a generic Day-2 convergence workflow for GitOps updates:
  1. Optionally re-renders/re-encrypts target service overlay (--generate)
  2. Scopes and stages cluster changes, validating that no unencrypted secrets are committed
  3. Commits and pushes to the GitOps remote repository
  4. Triggers in-cluster Flux reconciliation for GitRepository source, Kustomizations, and HelmReleases

Examples:
  # Reconcile all uncommitted cluster changes for active cluster
  opencenter cluster reconcile

  # Reconcile specific cluster and service
  opencenter cluster reconcile opensnack/doritos --service openstack-ccm

  # Re-render service values and reconcile
  opencenter cluster reconcile opensnack/doritos -s openstack-ccm --generate

  # Preview execution plan without mutating git or cluster
  opencenter cluster reconcile opensnack/doritos --dry-run

  # Only commit and push without in-cluster Flux reconciliation
  opencenter cluster reconcile opensnack/doritos --push-only

  # Only trigger in-cluster Flux reconciliation without Git commit/push
  opencenter cluster reconcile opensnack/doritos --flux-only`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := resolveClusterNameForCommand(cmd, args, true)
			if err != nil {
				return err
			}

			globalOpts := getGlobalOptions(cmd)
			dryRun := globalOpts.DryRun

			if pushOnly {
				skipFlux = true
			}
			if skipFlux && fluxOnly {
				return fmt.Errorf("--push-only/--skip-flux and --flux-only cannot be used together")
			}

			pathResolver := paths.NewPathResolver(config.ResolveClustersDir())
			reconciler := cluster.NewReconcileService(pathResolver)

			opts := cluster.ReconcileOptions{
				ClusterName:    name,
				Services:       services,
				All:            all,
				Message:        message,
				Remote:         remote,
				SkipFlux:       skipFlux,
				PushOnly:       pushOnly,
				FluxOnly:       fluxOnly,
				Generate:       generate,
				Wait:           wait,
				Timeout:        timeout,
				DryRun:         dryRun,
				KubeconfigPath: kubeconfig,
			}

			result, err := reconciler.Reconcile(cmd.Context(), opts)
			if err != nil {
				return err
			}

			if dryRun && result.Plan != nil {
				cluster.PrintReconcilePlan(cmd.OutOrStdout(), result.Plan)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Reconciliation complete for cluster %s in %v\n", name, result.Duration.Round(time.Millisecond))
			if result.CommitSHA != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Commit SHA: %s\n", result.CommitSHA)
			}
			if result.Pushed {
				fmt.Fprintln(cmd.OutOrStdout(), "  GitOps repository pushed to remote")
			}
			if result.FluxReconciled {
				fmt.Fprintln(cmd.OutOrStdout(), "  Flux in-cluster resources reconciled")
			}
			if len(result.Services) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Services: %s\n", strings.Join(result.Services, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&services, "service", "s", nil, "target service(s) to reconcile (repeatable or comma-separated)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "reconcile all enabled services for the cluster")
	cmd.Flags().StringVarP(&message, "message", "m", "", "custom git commit message")
	cmd.Flags().StringVar(&remote, "remote", "", "git remote to push to (defaults to tracking remote, upstream, or origin)")
	cmd.Flags().BoolVar(&pushOnly, "push-only", false, "execute GitOps commit & push only; skip Flux in-cluster reconciliation")
	cmd.Flags().BoolVar(&skipFlux, "skip-flux", false, "skip Flux in-cluster reconciliation")
	cmd.Flags().BoolVar(&fluxOnly, "flux-only", false, "execute Flux in-cluster reconciliation only; skip Git commit & push")
	cmd.Flags().BoolVarP(&generate, "generate", "g", false, "re-render and re-encrypt target service overlay before reconciling")
	cmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for Flux resources to become Ready")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "timeout for Flux reconciliation and rollout")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig (defaults to cluster-owned kubeconfig)")

	return cmd
}
