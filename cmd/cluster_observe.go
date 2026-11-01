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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/importer"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const (
	observeSourceTerraform = "terraform"
	observeSourceLive      = "live"
	observeSourceAll       = "all"
)

// observationTarget is the cluster an observation runs against.
type observationTarget struct {
	Config         *v2.Config
	Identifier     string
	KubeconfigPath string
}

// observationReport is the merged view of what the deployed cluster knows that
// the cluster config does not.
type observationReport struct {
	ClusterName string                          `json:"cluster_name"`
	Promotable  []importer.FieldInferenceResult `json:"promotable"`
	Conflicts   []importer.FieldConflict        `json:"conflicts"`
	Sources     []string                        `json:"sources"`
	Notes       []string                        `json:"notes,omitempty"`
}

func (r *observationReport) hasFindings() bool {
	return len(r.Promotable) > 0 || len(r.Conflicts) > 0
}

// loadObservationTarget loads the cluster config plus the kubeconfig the live
// source reads. The config is loaded through the native v2 path so the module
// ref backfill and defaults hydration apply, matching what deploy renders with.
func loadObservationTarget(ctx context.Context, identifier string) (*observationTarget, error) {
	cfg, _, _, clusterPaths, err := loadNativeV2ConfigWithIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster configuration: %w", err)
	}
	return &observationTarget{
		Config:         cfg,
		Identifier:     identifier,
		KubeconfigPath: clusterPaths.KubeconfigPath,
	}, nil
}

// collectObservations merges every requested evidence source into one report.
// Terraform evidence answers "what did OpenTofu create"; live evidence answers
// "what is running in the cluster". They are reported together so an operator
// reads one divergence list rather than reconciling two tools.
func collectObservations(ctx context.Context, target *observationTarget, sources []string) (*observationReport, error) {
	report := &observationReport{
		ClusterName: target.Identifier,
		Promotable:  []importer.FieldInferenceResult{},
		Conflicts:   []importer.FieldConflict{},
		Sources:     sources,
	}

	for _, source := range sources {
		switch source {
		case observeSourceTerraform:
			inferred, conflicts, err := importer.ObserveTerraformFromConfig(ctx, target.Config, "")
			if err != nil {
				// A broken tofu invocation is reported, not fatal: the live
				// source may still be readable and the operator should see it.
				report.Notes = append(report.Notes, fmt.Sprintf("terraform source unavailable: %v", err))
				continue
			}
			report.Promotable = append(report.Promotable, inferred...)
			report.Conflicts = append(report.Conflicts, conflicts...)
			if len(inferred) == 0 && len(conflicts) == 0 {
				// "Nothing to report" has two causes the observer cannot tell apart
				// -- the state has no network outputs (never applied, or a module
				// predating lifecycle contract 3) and the outputs match the config
				// already. Saying only the first would send an in-sync operator to
				// upgrade their module ref, so the note stays neutral and lists both.
				report.Notes = append(report.Notes, "terraform source reported nothing to change (either the state exposes no network outputs -- never applied, or a module predating lifecycle contract 3 -- or they already match the config)")
			}
		case observeSourceLive:
			live, err := importer.BuildLiveImportResult(ctx, target.Config, target.KubeconfigPath)
			if err != nil {
				report.Notes = append(report.Notes, fmt.Sprintf("live source unavailable: %v", err))
				continue
			}
			report.Promotable = append(report.Promotable, live.FieldResults...)
			report.Conflicts = append(report.Conflicts, live.Conflicts...)
			for _, service := range live.ServiceResults {
				report.Promotable = append(report.Promotable, service.Fields...)
			}
		}
	}

	sort.SliceStable(report.Promotable, func(i, j int) bool {
		return report.Promotable[i].Path < report.Promotable[j].Path
	})
	sort.SliceStable(report.Conflicts, func(i, j int) bool {
		return report.Conflicts[i].Path < report.Conflicts[j].Path
	})
	return report, nil
}

// dedupeObservations keeps one entry per config path. Both sources can describe
// the same field, and a write plan that patches a path twice in one pass would
// apply the second patch to already-patched content.
func dedupeObservations(fields []importer.FieldInferenceResult) []importer.FieldInferenceResult {
	seen := make(map[string]struct{}, len(fields))
	out := make([]importer.FieldInferenceResult, 0, len(fields))
	for _, field := range fields {
		key := strings.TrimSpace(field.Path)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, field)
	}
	return out
}

func parseObserveSources(raw string) ([]string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || normalized == observeSourceAll {
		return []string{observeSourceTerraform, observeSourceLive}, nil
	}
	allowed := map[string]struct{}{observeSourceTerraform: {}, observeSourceLive: {}}
	var sources []string
	for _, part := range strings.Split(normalized, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := allowed[part]; !ok {
			return nil, fmt.Errorf("unknown source %q; expected terraform, live, or all", part)
		}
		sources = append(sources, part)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("--source requires at least one of terraform, live, or all")
	}
	return sources, nil
}

func filterFieldsByPath(fields []importer.FieldInferenceResult, wanted []string) []importer.FieldInferenceResult {
	allow := make(map[string]struct{}, len(wanted))
	for _, path := range wanted {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			allow[trimmed] = struct{}{}
		}
	}
	out := make([]importer.FieldInferenceResult, 0, len(fields))
	for _, field := range fields {
		if _, ok := allow[strings.TrimSpace(field.Path)]; ok {
			out = append(out, field)
		}
	}
	return out
}

func newClusterObserveCmd() *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:   "observe [cluster]",
		Short: "Report differences between the deployed cluster and its config",
		Long: `Report what the deployed cluster knows that the cluster configuration does not.

observe is read-only. It never writes the configuration file; use
'opencenter cluster promote' to write the reported values.

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
		fmt.Fprintf(cmd.OutOrStdout(), "\nApply with: opencenter cluster promote %s\n", report.ClusterName)
	}
}

func newClusterPromoteCmd() *cobra.Command {
	var source string
	var dryRun bool
	var fields []string

	cmd := &cobra.Command{
		Use:   "promote [cluster]",
		Short: "Write observed deployed-cluster values into the cluster config",
		Long: `Write values observed from the deployed cluster into the cluster configuration.

promote is the write half of 'opencenter cluster observe'. It re-observes the
cluster, then patches only the reported paths through the same write plan the
import flow uses, which means:

  - the config file is backed up before it is written
  - protected fields (credentials, tokens, keys) are never written
  - conflicting fields are skipped and reported, never overwritten
  - only high-confidence observations are written

Pass --dry-run to see the patch without writing it. Use --field to promote a
subset of the reported paths.

After promoting infrastructure identity, regenerate the GitOps tree so the
rendered manifests pick the values up:

  opencenter cluster promote acme/prod
  opencenter cluster generate acme/prod

The CCM [LoadBalancer] subnet-id in particular is rendered from
networking.subnet_id at generate time, so a promoted subnet does not reach the
cluster until the overlay is re-rendered and pushed.`,
		Example: `  # Preview the patch
  opencenter cluster promote acme/prod --dry-run

  # Write every promotable value
  opencenter cluster promote acme/prod

  # Write only the network identity
  opencenter cluster promote acme/prod \
    --field opencenter.infrastructure.cloud.openstack.networking.subnet_id`,
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

			promotable := dedupeObservations(report.Promotable)
			if len(fields) > 0 {
				promotable = filterFieldsByPath(promotable, fields)
			}
			if len(promotable) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to promote")
				for _, note := range report.Notes {
					fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", note)
				}
				return nil
			}

			// Route through the import write plan so backups, protected-field
			// refusal, and public-YAML patching are inherited rather than
			// reimplemented here.
			result := importer.ClusterImportResult{
				ClusterName:    target.Config.ClusterName(),
				Organization:   target.Config.OpenCenter.Meta.Organization,
				ProposedConfig: target.Config,
				FieldResults:   promotable,
				Conflicts:      report.Conflicts,
			}
			plan, err := importer.PrepareClusterWritePlan(cmd.Context(), result)
			if err != nil {
				return fmt.Errorf("prepare config promotion plan: %w", err)
			}
			if strings.TrimSpace(plan.Diff) == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No promotable change detected")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Config promotion plan for cluster %s:\n\n%s\n", plan.ClusterName, plan.Diff)
			for _, skipped := range plan.SkippedFields {
				fmt.Fprintf(cmd.OutOrStdout(), "skipped %s: %s\n", skipped.Path, skipped.Reason)
			}
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "\nDry run: no changes written")
				return nil
			}
			if err := importer.ApplyClusterWritePlan(plan); err != nil {
				return fmt.Errorf("apply config promotion plan: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nPromoted %d field(s) into %s\n", len(plan.ApprovedFields), plan.ConfigPath)
			fmt.Fprintln(cmd.OutOrStdout(), "Regenerate the GitOps tree so rendered manifests pick the values up:")
			fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster generate %s\n", plan.ClusterName)
			return nil
		},
	}

	cmd.Flags().String("source", observeSourceAll, "evidence source: terraform, live, or all")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the patch without writing it")
	cmd.Flags().StringSliceVar(&fields, "field", nil, "promote only these config paths")

	return cmd
}
