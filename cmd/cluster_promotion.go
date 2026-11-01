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
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/importer"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// The deployed-cluster → config promotion pipeline lives here, in one place.
// `cluster observe` reports it, `cluster drift reconcile --to-config` writes
// it, and the deprecated `cluster promote` delegates to it. Before this file,
// the same load → collect → dedupe → write-plan sequence existed three times
// (drift reconcile, the promotion preview, and promote) with subtly different
// source coverage.

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

// promotionPlan is one pending deployed-cluster → config write: the merged
// observations, the prepared write plan (backup, protected-field refusal,
// public-YAML patching all inherited from the import flow), and the report
// notes explaining anything the sources could not read.
type promotionPlan struct {
	Report *observationReport
	Plan   *importer.ClusterWritePlan
}

// buildPromotionPlan is the single promotion pipeline: load the cluster,
// collect the requested evidence sources, keep one entry per path, optionally
// restrict to --field paths, and prepare the write plan. A plan with an empty
// Diff means nothing is promotable.
func buildPromotionPlan(ctx context.Context, identifier string, sources, fields []string) (*promotionPlan, error) {
	target, err := loadObservationTarget(ctx, identifier)
	if err != nil {
		return nil, err
	}
	report, err := collectObservations(ctx, target, sources)
	if err != nil {
		return nil, err
	}
	promotable := dedupeObservations(report.Promotable)
	if len(fields) > 0 {
		promotable = filterFieldsByPath(promotable, fields)
	}
	if len(promotable) == 0 {
		return &promotionPlan{Report: report}, nil
	}
	// Route through the import write plan so backups, protected-field refusal,
	// and public-YAML patching are inherited rather than reimplemented here.
	result := importer.ClusterImportResult{
		ClusterName:    target.Config.ClusterName(),
		Organization:   target.Config.OpenCenter.Meta.Organization,
		ProposedConfig: target.Config,
		FieldResults:   promotable,
		Conflicts:      report.Conflicts,
	}
	plan, err := importer.PrepareClusterWritePlan(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("prepare config promotion plan: %w", err)
	}
	return &promotionPlan{Report: report, Plan: plan}, nil
}
