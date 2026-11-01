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
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newClusterObserveCmd() *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:   "observe [cluster]",
		Short: "Report differences between the deployed cluster and its config",
		Long: `Report what the deployed cluster knows that the cluster configuration does not.

observe is read-only. It never writes the configuration file; use
'opencenter cluster drift reconcile --to-config' to write the reported values.

Two evidence sources are merged into one report:

  terraform  the applied OpenTofu state, i.e. resources the CLI created. Needs
             the infrastructure tree rendered by lifecycle contract 3, which is
             what exposes opencenter_network_id / opencenter_subnet_id.
  live       the running Kubernetes cluster, read through kubectl and Flux.

The terraform source is what recovers a network OpenTofu created. 'cluster
provider openstack' clears network_id/subnet_id so OpenTofu can own them, and
until contract 3 nothing wrote the created IDs back, so the CCM rendered its
[LoadBalancer] subnet-id blank on a tofu-managed cluster.

A field the config already holds with a different value is reported as a
conflict, never as a promotable value: a populated subnet_id may be an operator
selection made with --create-internal-network=false.

Exit code 1 when findings exist, so observe can gate a pipeline.`,
		Example: `  # Report everything observable about the active cluster
  opencenter cluster observe

  # Only what OpenTofu created
  opencenter cluster observe acme/prod --source terraform

  # Machine-readable
  opencenter cluster observe acme/prod --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sources, err := parseObserveSources(source)
			if err != nil {
				return NewExitError(2, err.Error(), nil)
			}
			identifier, err := resolveClusterNameForCommand(cmd, args, true)
			if err != nil {
				return err
			}
			target, err := loadObservationTarget(cmd.Context(), identifier)
			if err != nil {
				return err
			}
			report, err := collectObservations(cmd.Context(), target, sources)
			if err != nil {
				return err
			}

			// --output is the root persistent flag (text|json|yaml); the old
			// per-command --json is a removed GA flag and must not reappear.
			switch getGlobalOptions(cmd).Output {
			case OutputJSON:
				encoded, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("encode observation report: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			default:
				renderObservationReport(cmd, report)
			}
			if report.hasFindings() {
				// Diff-style exit: 1 means "the cluster knows something the config
				// does not", so scripts can gate on it. The message is required --
				// main.go prints `Error: %s` for any non-nil return, and an empty
				// one renders as a bare, alarming `Error:` line.
				return NewExitError(1, "deployed cluster differs from its configuration", nil)
			}
			return nil
		},
	}

	cmd.Flags().String("source", observeSourceAll, "evidence source: terraform, live, or all")

	return cmd
}

func renderObservationReport(cmd *cobra.Command, report *observationReport) {
	fmt.Fprintf(cmd.OutOrStdout(), "Observations for %s\n\n", report.ClusterName)
	if !report.hasFindings() {
		fmt.Fprintln(cmd.OutOrStdout(), "No differences found between the deployed cluster and its configuration.")
	}
	if len(report.Promotable) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Promotable into config (%d):\n", len(report.Promotable))
		for _, field := range report.Promotable {
			fmt.Fprintf(cmd.OutOrStdout(), "  + %s = %v  [%s/%s]\n", field.Path, field.Value, field.Origin, field.Confidence)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
	}
	if len(report.Conflicts) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Conflicts needing review (%d):\n", len(report.Conflicts))
		for _, conflict := range report.Conflicts {
			fmt.Fprintf(cmd.OutOrStdout(), "  ! %s: config=%v deployed=%v (recommend %s)\n",
				conflict.Path, conflict.GitOpsValue, conflict.LiveValue, conflict.Recommended)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
	}
	for _, note := range report.Notes {
		fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", note)
	}
	if len(report.Promotable) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nApply with: opencenter cluster drift reconcile %s --to-config\n", report.ClusterName)
	}
}

// newClusterPromoteCmd is the deprecated alias of
// 'cluster drift reconcile --to-config'. The promotion pipeline it once
// implemented now lives in cluster_promotion.go and is shared with drift
// reconcile; this shell only maps the old flags onto the shared pipeline and
// points at the replacement. It is hidden from help and removed at the next
// major.
func newClusterPromoteCmd() *cobra.Command {
	var source string
	var dryRun bool
	var fields []string

	cmd := &cobra.Command{
		Use:    "promote [cluster]",
		Short:  "Deprecated alias for 'cluster drift reconcile --to-config'",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "deprecated: 'opencenter cluster promote' moved to 'opencenter cluster drift reconcile --to-config'; this alias is removed at the next major")
			sources, err := parseObserveSources(source)
			if err != nil {
				return NewExitError(2, err.Error(), nil)
			}
			identifier, err := resolveClusterNameForCommand(cmd, args, true)
			if err != nil {
				return err
			}
			return reconcileDriftToConfig(cmd, identifier, dryRun, sources, fields)
		},
	}

	cmd.Flags().String("source", observeSourceAll, "evidence source: terraform, live, or all")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the patch without writing it")
	cmd.Flags().StringSliceVar(&fields, "field", nil, "promote only these config paths")

	return cmd
}
