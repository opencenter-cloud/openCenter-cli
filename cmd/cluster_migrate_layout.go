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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type migrateLayoutOptions struct {
	organization string
	cluster      string
	dryRun       bool
	force        bool
	custom       bool
	apply        bool
}

type layoutMove struct {
	from          string
	to            string
	mode          os.FileMode
	parentMode    os.FileMode
	rewriteConfig bool
}

func newClusterMigrateLayoutCmd() *cobra.Command {
	opts := migrateLayoutOptions{}

	cmd := &cobra.Command{
		Use:   "migrate-layout --org <organization>",
		Short: "Migrate legacy cluster files into secure GitOps, state, and secrets zones",
		Long: `Migrate the legacy mixed org-root layout into the secure layout.

This is the only command allowed to read the old layout where a Git repository,
cluster state files, and private secrets share the same organization directory.
Normal cluster commands reject that layout.

The command moves GitOps content to the configured GitOps root, cluster config
and local state files to the cluster state root, and private keys to the secrets
root. Use --dry-run to print the move diff without changing files.

Use --custom with --cluster to migrate unknown hand-authored overlay files into
service custom/ directories. Custom migration is a dry run by default; use
--apply explicitly to change files.`,
		Example: `  # Preview migration for the acme organization
  opencenter cluster migrate-layout --org acme --dry-run

  # Perform migration, refusing to overwrite destinations
  opencenter cluster migrate-layout --org acme

  # Perform migration and replace existing destinations
  opencenter cluster migrate-layout --org acme --force

  # Preview hand-authored overlay migration for one cluster
  opencenter cluster migrate-layout --custom --org acme --cluster prod

  # Apply the hand-authored overlay migration
  opencenter cluster migrate-layout --custom --org acme --cluster prod --apply`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.organization) == "" {
				return fmt.Errorf("--org is required")
			}
			if opts.custom {
				if strings.TrimSpace(opts.cluster) == "" {
					return fmt.Errorf("--cluster is required with --custom")
				}
				if opts.force {
					return fmt.Errorf("--force cannot be used with --custom; destination collisions are always refused")
				}
				if opts.apply && opts.dryRun {
					return fmt.Errorf("--apply and --dry-run cannot be used together")
				}
				return runClusterMigrateCustom(cmd.Context(), cmd.OutOrStdout(), opts)
			}
			if opts.apply {
				return fmt.Errorf("--apply requires --custom")
			}
			if strings.TrimSpace(opts.cluster) != "" {
				return fmt.Errorf("--cluster requires --custom")
			}
			return runClusterMigrateLayout(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.organization, "org", "", "Organization name to migrate")
	cmd.Flags().StringVar(&opts.cluster, "cluster", "", "Cluster name for --custom migration")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print planned moves without changing files")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Overwrite existing destination files")
	cmd.Flags().BoolVar(&opts.custom, "custom", false, "Migrate hand-authored overlay files into custom/ (dry run by default)")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply --custom migration; without this flag it is a dry run")

	return cmd
}

func runClusterMigrateLayout(ctx context.Context, out io.Writer, opts migrateLayoutOptions) error {
	_ = ctx

	org := strings.TrimSpace(opts.organization)
	legacyOrgDir := filepath.Join(config.ResolveClustersDir(), org)
	if err := ensureLegacyLayoutForMigration(legacyOrgDir); err != nil {
		return err
	}

	clusters, err := discoverLegacyLayoutClusters(legacyOrgDir)
	if err != nil {
		return err
	}
	if len(clusters) == 0 {
		return fmt.Errorf("no legacy clusters found in %s", legacyOrgDir)
	}

	movePlan, err := buildLayoutMigrationPlan(legacyOrgDir, org, clusters)
	if err != nil {
		return err
	}
	if len(movePlan) == 0 {
		return fmt.Errorf("legacy layout at %s did not contain files that need migration", legacyOrgDir)
	}

	if err := validateLayoutMovePlan(movePlan, opts.force); err != nil {
		return err
	}

	fmt.Fprintf(out, "Migrating legacy layout for organization %s\n", org)
	if opts.dryRun {
		fmt.Fprintln(out, "Dry run: no files will be changed")
	}
	for _, move := range movePlan {
		fmt.Fprintf(out, "MOVE %s -> %s\n", move.from, move.to)
		if move.rewriteConfig && opts.dryRun {
			fmt.Fprintf(out, "  CONFIG REWRITE: paths updated to secure layout (gitops, sops_age_key_file, ssh key paths)\n")
		}
	}

	if opts.dryRun {
		return nil
	}

	for _, move := range movePlan {
		if err := applyLayoutMove(move, opts.force); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "Migrated %d paths into secure layout\n", len(movePlan))
	return nil
}

type customMigrationCandidate struct {
	rel  string
	from string
	to   string
}

type customMigrationReport struct {
	moved            []string
	alreadyCustom    []string
	tracked          []string
	trackedModified  []string
	refused          []string
	unknown          []string
	symlinks         []string
	gitMoveFallbacks []string
}

func runClusterMigrateCustom(ctx context.Context, out io.Writer, opts migrateLayoutOptions) error {
	org := strings.TrimSpace(opts.organization)
	cluster := strings.TrimSpace(opts.cluster)
	if err := validateMigrationComponent(org, "organization"); err != nil {
		return err
	}
	if err := validateMigrationComponent(cluster, "cluster"); err != nil {
		return err
	}

	overlayDir := filepath.Clean(migrationClusterPaths(org, cluster).ApplicationsDir)
	if !migrationPathWithin(overlayDir, filepath.Dir(overlayDir)) {
		return fmt.Errorf("resolved overlay path escapes its parent: %s", overlayDir)
	}
	info, err := os.Lstat(overlayDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("cluster overlay not found at %s", overlayDir)
	}
	if err != nil {
		return fmt.Errorf("inspect cluster overlay %s: %w", overlayDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to scan symlinked cluster overlay %s", overlayDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("cluster overlay is not a directory: %s", overlayDir)
	}

	manifest, err := loadCustomMigrationManifest(overlayDir)
	if err != nil {
		return err
	}
	report := customMigrationReport{}
	candidates := make([]customMigrationCandidate, 0)
	candidateDestinations := make(map[string]string)
	err = filepath.WalkDir(overlayDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(overlayDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			report.symlinks = append(report.symlinks, rel)
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if rel == gitops.GeneratedManifestFile || !isMigrationOwnedPath(rel) {
			return nil
		}
		if isMigrationCustomPath(rel) {
			report.alreadyCustom = append(report.alreadyCustom, rel)
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read overlay file %s: %w", rel, err)
		}
		expected, tracked := manifest.Files[rel]
		if tracked {
			actual := migrationHash(data)
			if actual == expected {
				report.tracked = append(report.tracked, rel)
			} else {
				report.trackedModified = append(report.trackedModified, rel)
			}
			return nil
		}

		destination, ok := customMigrationDestination(overlayDir, rel)
		if !ok {
			report.unknown = append(report.unknown, rel)
			return nil
		}
		if filepath.Base(rel) == "kustomization.yaml" {
			report.refused = append(report.refused, fmt.Sprintf("%s (reserved custom kustomization destination)", rel))
			return nil
		}
		if !migrationPathWithin(destination, overlayDir) {
			return fmt.Errorf("refusing destination outside cluster overlay: %s", destination)
		}
		if previous, exists := candidateDestinations[destination]; exists {
			report.refused = append(report.refused, fmt.Sprintf("%s (destination also needed by %s)", rel, previous))
			return nil
		}
		if destinationInfo, err := os.Lstat(destination); err == nil {
			destinationRel := migrationRelativeOrAbsolute(overlayDir, destination)
			if destinationInfo.Mode()&os.ModeSymlink != 0 {
				report.symlinks = append(report.symlinks, filepath.ToSlash(filepath.Join("destination", destinationRel)))
			} else {
				report.refused = append(report.refused, fmt.Sprintf("%s (destination exists: %s)", rel, filepath.ToSlash(destinationRel)))
			}
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect destination for %s: %w", rel, err)
		}
		if err := validateMigrationParents(overlayDir, filepath.Dir(destination)); err != nil {
			report.symlinks = append(report.symlinks, fmt.Sprintf("%s (%v)", rel, err))
			return nil
		}
		candidateDestinations[destination] = rel
		candidates = append(candidates, customMigrationCandidate{rel: rel, from: path, to: destination})
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan cluster overlay: %w", err)
	}

	sort.Strings(report.alreadyCustom)
	sort.Strings(report.tracked)
	sort.Strings(report.trackedModified)
	sort.Strings(report.refused)
	sort.Strings(report.unknown)
	sort.Strings(report.symlinks)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].rel < candidates[j].rel })

	kustomizationUpdates := make(map[string][]byte)
	resourcesByKustomization := make(map[string][]string)
	for _, candidate := range candidates {
		kustomizationPath := filepath.Join(filepath.Dir(candidate.to), "kustomization.yaml")
		resourcesByKustomization[kustomizationPath] = append(resourcesByKustomization[kustomizationPath], filepath.Base(candidate.to))
	}
	for path, resources := range resourcesByKustomization {
		content, changed, err := prepareCustomKustomization(path, resources)
		if err != nil {
			return err
		}
		if changed {
			kustomizationUpdates[path] = content
		}
	}

	fmt.Fprintf(out, "Migrating hand-authored overlay files for %s/%s\n", org, cluster)
	if !opts.apply {
		fmt.Fprintln(out, "Dry run: no files will be changed")
	}
	for _, candidate := range candidates {
		fmt.Fprintf(out, "MOVE %s -> %s\n", candidate.rel, filepath.ToSlash(migrationRelativeOrAbsolute(overlayDir, candidate.to)))
	}
	printCustomMigrationReport(out, report, len(candidates))
	if len(kustomizationUpdates) > 0 {
		for path := range kustomizationUpdates {
			fmt.Fprintf(out, "ENSURE %s includes moved resources\n", filepath.ToSlash(migrationRelativeOrAbsolute(overlayDir, path)))
		}
	}
	if !opts.apply {
		return nil
	}

	for _, candidate := range candidates {
		if err := os.MkdirAll(filepath.Dir(candidate.to), 0o755); err != nil {
			return fmt.Errorf("create custom destination for %s: %w", candidate.rel, err)
		}
		if err := applyCustomMigrationMove(ctx, candidate.from, candidate.to, overlayDir, &report); err != nil {
			return fmt.Errorf("move %s: %w", candidate.rel, err)
		}
		report.moved = append(report.moved, candidate.rel)
	}
	for path, content := range kustomizationUpdates {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create custom kustomization directory: %w", err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write custom kustomization %s: %w", path, err)
		}
	}
	fmt.Fprintf(out, "Applied custom overlay migration: moved %d files\n", len(report.moved))
	if len(report.gitMoveFallbacks) > 0 {
		for _, fallback := range report.gitMoveFallbacks {
			fmt.Fprintf(out, "GIT-MV-FALLBACK %s\n", fallback)
		}
	}
	return nil
}

func printCustomMigrationReport(out io.Writer, report customMigrationReport, planned int) {
	fmt.Fprintf(out, "MOVED: %d planned\n", planned)
	fmt.Fprintf(out, "SKIPPED-ALREADY-CUSTOM: %d\n", len(report.alreadyCustom))
	fmt.Fprintf(out, "TRACKED: %d\n", len(report.tracked))
	fmt.Fprintf(out, "TRACKED-MODIFIED (needs manual attention): %d\n", len(report.trackedModified))
	fmt.Fprintf(out, "REFUSED-DESTINATION-EXISTS: %d\n", len(report.refused))
	fmt.Fprintf(out, "UNKNOWN (not under a service directory): %d\n", len(report.unknown))
	fmt.Fprintf(out, "SYMLINKS-SKIPPED: %d\n", len(report.symlinks))
	for _, path := range report.alreadyCustom {
		fmt.Fprintf(out, "  SKIP custom: %s\n", path)
	}
	for _, path := range report.trackedModified {
		fmt.Fprintf(out, "  MANUAL attention: %s\n", path)
	}
	for _, path := range report.refused {
		fmt.Fprintf(out, "  REFUSED: %s\n", path)
	}
	for _, path := range report.unknown {
		fmt.Fprintf(out, "  UNKNOWN: %s\n", path)
	}
	for _, path := range report.symlinks {
		fmt.Fprintf(out, "  SKIP symlink: %s\n", path)
	}
}

func loadCustomMigrationManifest(root string) (gitops.GeneratedManifest, error) {
	manifest := gitops.GeneratedManifest{Files: make(map[string]string)}
	data, err := os.ReadFile(filepath.Join(root, gitops.GeneratedManifestFile))
	if os.IsNotExist(err) {
		return manifest, nil
	}
	if err != nil {
		return manifest, fmt.Errorf("read generated manifest: %w", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("read generated manifest: corrupt JSON: %w", err)
	}
	for path := range manifest.Files {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
		if clean != path || !migrationPathWithin(filepath.Join(root, filepath.FromSlash(path)), root) || !isMigrationOwnedPath(path) {
			return manifest, fmt.Errorf("generated manifest contains invalid path %q", path)
		}
	}
	if manifest.Files == nil {
		manifest.Files = make(map[string]string)
	}
	return manifest, nil
}

func migrationHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateMigrationComponent(value, label string) error {
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, `/\\`) {
		return fmt.Errorf("invalid %s %q", label, value)
	}
	return nil
}

func migrationPathWithin(path, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func validateMigrationParents(root, dir string) error {
	dir = filepath.Clean(dir)
	if !migrationPathWithin(dir, root) {
		return fmt.Errorf("path escapes overlay root")
	}
	for current := dir; migrationPathWithin(current, root) && filepath.Clean(current) != filepath.Clean(root); current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in destination parent %s", filepath.ToSlash(migrationRelativeOrAbsolute(root, current)))
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func isMigrationOwnedPath(path string) bool {
	path = filepath.ToSlash(path)
	if path == "kustomization.yaml" || path == ".sops.yaml" {
		return true
	}
	for _, root := range []string{"services", "managed-services", "customer-managed"} {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func isMigrationCustomPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 2 {
		return false
	}
	if parts[0] != "services" && parts[0] != "managed-services" && parts[0] != "customer-managed" {
		return false
	}
	for _, part := range parts[1:] {
		if part == gitops.CustomDirName {
			return true
		}
	}
	return false
}

func customMigrationDestination(root, rel string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 || (parts[0] != "services" && parts[0] != "managed-services" && parts[0] != "customer-managed") {
		return "", false
	}
	return filepath.Clean(filepath.Join(root, parts[0], parts[1], gitops.CustomDirName, filepath.Base(rel))), true
}

func migrationRelativeOrAbsolute(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return path
}

func prepareCustomKustomization(path string, resources []string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		node := yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
		root := node.Content[0]
		setMigrationYAMLValue(root, "apiVersion", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "kustomize.config.k8s.io/v1beta1"})
		setMigrationYAMLValue(root, "kind", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "Kustomization"})
		setMigrationYAMLValue(root, "resources", migrationResourceNode(resources))
		content, err := yaml.Marshal(&node)
		return content, true, err
	}
	if err != nil {
		return nil, false, fmt.Errorf("read custom kustomization %s: %w", path, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, false, fmt.Errorf("parse custom kustomization %s: %w", path, err)
	}
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
		return nil, false, fmt.Errorf("custom kustomization %s is not a YAML mapping", path)
	}
	root := node.Content[0]
	resourcesNode := findMigrationYAMLValue(root, "resources")
	if resourcesNode == nil {
		resourcesNode = migrationResourceNode(nil)
		setMigrationYAMLValue(root, "resources", resourcesNode)
	}
	if resourcesNode.Kind != yaml.SequenceNode {
		return nil, false, fmt.Errorf("custom kustomization %s has non-sequence resources", path)
	}
	seen := make(map[string]bool)
	for _, item := range resourcesNode.Content {
		if item.Kind == yaml.ScalarNode {
			seen[filepath.ToSlash(filepath.Clean(strings.TrimPrefix(item.Value, "./")))] = true
		}
	}
	changed := false
	for _, resource := range resources {
		if !seen[resource] {
			resourcesNode.Content = append(resourcesNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: resource})
			seen[resource] = true
			changed = true
		}
	}
	if !changed {
		return data, false, nil
	}
	content, err := yaml.Marshal(&node)
	return content, true, err
}

func migrationResourceNode(resources []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, resource := range resources {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: resource})
	}
	return node
}

func findMigrationYAMLValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMigrationYAMLValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func applyCustomMigrationMove(ctx context.Context, from, to, overlayRoot string, report *customMigrationReport) error {
	repoCmd := exec.CommandContext(ctx, "git", "-C", overlayRoot, "rev-parse", "--show-toplevel")
	repoOutput, repoErr := repoCmd.Output()
	if repoErr == nil {
		repoRoot := strings.TrimSpace(string(repoOutput))
		fromRel, fromErr := filepath.Rel(repoRoot, from)
		toRel, toErr := filepath.Rel(repoRoot, to)
		if fromErr == nil && toErr == nil && migrationPathWithin(from, repoRoot) && migrationPathWithin(to, repoRoot) {
			trackedCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "ls-files", "--error-unmatch", "--", filepath.ToSlash(fromRel))
			if trackedErr := trackedCmd.Run(); trackedErr == nil {
				gitMove := exec.CommandContext(ctx, "git", "-C", repoRoot, "mv", "--", filepath.ToSlash(fromRel), filepath.ToSlash(toRel))
				if err := gitMove.Run(); err == nil {
					return nil
				} else {
					report.gitMoveFallbacks = append(report.gitMoveFallbacks, fmt.Sprintf("%s -> %s (%v)", filepath.ToSlash(fromRel), filepath.ToSlash(toRel), err))
				}
			}
		}
	}
	if err := os.Rename(from, to); err != nil {
		return err
	}
	return nil
}

func ensureLegacyLayoutForMigration(legacyOrgDir string) error {
	if _, err := os.Stat(filepath.Join(legacyOrgDir, ".git")); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("legacy Git repository not found at %s", legacyOrgDir)
		}
		return fmt.Errorf("checking legacy Git repository: %w", err)
	}

	hasLegacyMarker := false
	markers := []string{
		filepath.Join(legacyOrgDir, "secrets"),
		filepath.Join(legacyOrgDir, "infrastructure", "clusters"),
	}
	for _, marker := range markers {
		if _, err := os.Stat(marker); err == nil {
			hasLegacyMarker = true
			break
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("checking legacy marker %s: %w", marker, err)
		}
	}
	if matches, err := filepath.Glob(filepath.Join(legacyOrgDir, ".*-config.yaml")); err == nil && len(matches) > 0 {
		hasLegacyMarker = true
	} else if err != nil {
		return fmt.Errorf("checking legacy config files: %w", err)
	}

	if !hasLegacyMarker {
		return fmt.Errorf("legacy mixed layout markers were not found at %s", legacyOrgDir)
	}
	return nil
}

func discoverLegacyLayoutClusters(legacyOrgDir string) ([]string, error) {
	clusters := make(map[string]struct{})

	configFiles, err := filepath.Glob(filepath.Join(legacyOrgDir, ".*-config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("discovering legacy config files: %w", err)
	}
	for _, path := range configFiles {
		name := filepath.Base(path)
		name = strings.TrimPrefix(name, ".")
		name = strings.TrimSuffix(name, "-config.yaml")
		if name != "" {
			clusters[name] = struct{}{}
		}
	}

	infraDir := filepath.Join(legacyOrgDir, "infrastructure", "clusters")
	entries, err := os.ReadDir(infraDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "" {
				clusters[entry.Name()] = struct{}{}
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading legacy infrastructure clusters: %w", err)
	}

	names := make([]string, 0, len(clusters))
	for name := range clusters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func buildLayoutMigrationPlan(legacyOrgDir, organization string, clusters []string) ([]layoutMove, error) {
	var movePlan []layoutMove
	gitopsDir := filepath.Join(config.GetGitOpsDir(), organization)

	for _, clusterName := range clusters {
		clusterPaths := migrationClusterPaths(organization, clusterName)
		if err := clusterPaths.Validate(); err != nil {
			return nil, fmt.Errorf("invalid secure layout for %s/%s: %w", organization, clusterName, err)
		}

		legacyConfigPath := filepath.Join(legacyOrgDir, "."+clusterName+"-config.yaml")
		if fileExists(legacyConfigPath) {
			movePlan = append(movePlan, layoutMove{
				from:          legacyConfigPath,
				to:            clusterPaths.ConfigPath,
				mode:          0o600,
				rewriteConfig: true,
			})
		}

		legacyKubeconfig := filepath.Join(legacyOrgDir, "infrastructure", "clusters", clusterName, "kubeconfig.yaml")
		if fileExists(legacyKubeconfig) {
			movePlan = append(movePlan, layoutMove{from: legacyKubeconfig, to: clusterPaths.KubeconfigPath, mode: 0o600})
		}

		legacySOPSKey := filepath.Join(legacyOrgDir, "secrets", "age", "keys", clusterName+"-key.txt")
		if fileExists(legacySOPSKey) {
			movePlan = append(movePlan, layoutMove{from: legacySOPSKey, to: clusterPaths.SOPSKeyPath, mode: 0o600})
		}
		legacySOPSPublicKey := legacySOPSKey + ".pub"
		if fileExists(legacySOPSPublicKey) {
			movePlan = append(movePlan, layoutMove{from: legacySOPSPublicKey, to: clusterPaths.SOPSKeyPath + ".pub", mode: 0o644})
		}

		movePlan = append(movePlan, legacySSHKeyMoves(legacyOrgDir, clusterName, clusterPaths)...)
	}

	gitOpsMoves, err := legacyGitOpsMoves(legacyOrgDir, gitopsDir)
	if err != nil {
		return nil, err
	}
	movePlan = append(movePlan, gitOpsMoves...)

	sort.SliceStable(movePlan, func(i, j int) bool {
		if strings.HasPrefix(movePlan[j].from, movePlan[i].from+string(filepath.Separator)) {
			return false
		}
		if strings.HasPrefix(movePlan[i].from, movePlan[j].from+string(filepath.Separator)) {
			return true
		}
		if movePlan[i].from == movePlan[j].from {
			return movePlan[i].to < movePlan[j].to
		}
		return movePlan[i].from < movePlan[j].from
	})
	return movePlan, nil
}

func migrationClusterPaths(organization, clusterName string) *paths.ClusterPaths {
	gitopsDir := filepath.Join(config.GetGitOpsDir(), organization)
	blueprintsDir := filepath.Join(config.GetBlueprintsDir(), organization, clusterName)
	secretsDir := filepath.Join(config.GetSecretsDir(), organization, clusterName)
	clusterDir := filepath.Join(gitopsDir, "infrastructure", "clusters", clusterName)

	return &paths.ClusterPaths{
		OrganizationDir: gitopsDir,
		GitOpsDir:       gitopsDir,
		ClusterStateDir: clusterDir,
		ClusterDir:      clusterDir,
		ApplicationsDir: filepath.Join(gitopsDir, "applications", "overlays", clusterName),
		SecretsDir:      secretsDir,
		SOPSKeyPath:     filepath.Join(secretsDir, "age", "keys", clusterName+"-key.txt"),
		SOPSConfigPath:  filepath.Join(gitopsDir, ".sops.yaml"),
		KubeconfigPath:  filepath.Join(secretsDir, "kubernetes", "kubeconfig.yaml"),
		InventoryPath:   filepath.Join(clusterDir, "inventory"),
		VenvPath:        filepath.Join(clusterDir, "venv"),
		BinPath:         filepath.Join(clusterDir, ".bin"),
		ConfigPath:      filepath.Join(blueprintsDir, clusterName+"-config.yaml"),
		SSHKeyPath:      filepath.Join(secretsDir, "ssh", clusterName),
	}
}

func legacySSHKeyMoves(legacyOrgDir, clusterName string, clusterPaths *paths.ClusterPaths) []layoutMove {
	sshDir := filepath.Join(legacyOrgDir, "secrets", "ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil
	}

	var moves []layoutMove
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name != clusterName && name != clusterName+".pub" && !strings.HasPrefix(name, clusterName+"-") {
			continue
		}

		dest := filepath.Join(filepath.Dir(clusterPaths.SSHKeyPath), name)
		mode := os.FileMode(0o600)
		if strings.HasSuffix(name, ".pub") {
			mode = 0o644
		}
		moves = append(moves, layoutMove{from: filepath.Join(sshDir, name), to: dest, mode: mode})
	}
	return moves
}

func legacyGitOpsMoves(legacyOrgDir, gitopsDir string) ([]layoutMove, error) {
	entries, err := os.ReadDir(legacyOrgDir)
	if err != nil {
		return nil, fmt.Errorf("reading legacy organization directory: %w", err)
	}

	var moves []layoutMove

	// Move .git first to preserve commit history in the new GitOps zone.
	legacyGitDir := filepath.Join(legacyOrgDir, ".git")
	if fileExists(legacyGitDir) {
		moves = append(moves, layoutMove{
			from:       legacyGitDir,
			to:         filepath.Join(gitopsDir, ".git"),
			parentMode: 0o755,
		})
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip .git (handled above), secrets dir, and legacy config files.
		if name == ".git" || name == "secrets" || (strings.HasSuffix(name, "-config.yaml") && strings.HasPrefix(name, ".")) {
			continue
		}

		from := filepath.Join(legacyOrgDir, name)
		to := filepath.Join(gitopsDir, name)
		moves = append(moves, layoutMove{from: from, to: to, parentMode: 0o755})
	}
	return moves, nil
}

func validateLayoutMovePlan(movePlan []layoutMove, force bool) error {
	for _, move := range movePlan {
		if !fileExists(move.from) {
			return fmt.Errorf("migration source disappeared: %s", move.from)
		}
		if fileExists(move.to) && !force {
			return fmt.Errorf("migration destination already exists: %s (use --force to overwrite)", move.to)
		}
	}
	return nil
}

func applyLayoutMove(move layoutMove, force bool) error {
	parentMode := move.parentMode
	if parentMode == 0 {
		parentMode = 0o700
	}
	if err := os.MkdirAll(filepath.Dir(move.to), parentMode); err != nil {
		return fmt.Errorf("creating destination directory for %s: %w", move.to, err)
	}

	if force {
		if err := os.RemoveAll(move.to); err != nil {
			return fmt.Errorf("removing existing destination %s: %w", move.to, err)
		}
	}

	if move.rewriteConfig {
		if err := rewriteLegacyClusterConfig(move.from, move.to, move.mode); err != nil {
			return err
		}
		if err := os.Remove(move.from); err != nil {
			return fmt.Errorf("removing migrated config %s: %w", move.from, err)
		}
		return nil
	}

	if err := moveLayoutPath(move.from, move.to); err != nil {
		return fmt.Errorf("moving %s to %s: %w", move.from, move.to, err)
	}
	if move.mode != 0 {
		if err := os.Chmod(move.to, move.mode); err != nil {
			return fmt.Errorf("setting permissions on %s: %w", move.to, err)
		}
	}
	return nil
}

func moveLayoutPath(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		if !stderrors.Is(err, syscall.EXDEV) {
			return err
		}
		if err := copyLayoutPath(from, to); err != nil {
			return err
		}
		return os.RemoveAll(from)
	}
	return nil
}

func copyLayoutPath(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(from)
		if err != nil {
			return err
		}
		return os.Symlink(target, to)
	}
	if info.IsDir() {
		return copyLayoutDir(from, to, info.Mode().Perm())
	}
	return copyLayoutFile(from, to, info.Mode().Perm())
}

func copyLayoutDir(from, to string, mode os.FileMode) error {
	if err := os.MkdirAll(to, mode); err != nil {
		return err
	}
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(to, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, dest)
		}
		if entry.IsDir() {
			return os.MkdirAll(dest, info.Mode().Perm())
		}
		return copyLayoutFile(path, dest, info.Mode().Perm())
	})
}

func copyLayoutFile(from, to string, mode os.FileMode) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Chmod(to, mode)
}

func rewriteLegacyClusterConfig(from, to string, mode os.FileMode) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return fmt.Errorf("reading legacy config %s: %w", from, err)
	}

	var configMap map[string]any
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("parsing legacy config %s: %w", from, err)
	}
	if configMap == nil {
		configMap = make(map[string]any)
	}

	clusterName := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(from), "."), "-config.yaml")
	organization := inferOrganizationFromLegacyConfigPath(from)
	clusterPaths := migrationClusterPaths(organization, clusterName)

	setNestedMigrationConfigValue(configMap, clusterPaths.GitOpsDir, "opencenter", "gitops", "repository", "local_dir")
	setNestedMigrationConfigValue(configMap, clusterPaths.SOPSKeyPath, "secrets", "sops_age_key_file")
	setNestedMigrationConfigValue(configMap, clusterPaths.SOPSKeyPath, "secrets", "sops", "age_key_file")
	setNestedMigrationConfigValue(configMap, clusterPaths.SSHKeyPath, "secrets", "ssh_key", "private")
	setNestedMigrationConfigValue(configMap, clusterPaths.SSHKeyPath+".pub", "secrets", "ssh_key", "public")
	setNestedMigrationConfigValue(configMap, clusterPaths.SSHKeyPath, "opencenter", "infrastructure", "ssh", "key_path")

	if hasNestedMigrationConfigValue(configMap, "opencenter", "gitops", "auth", "ssh") {
		setNestedMigrationConfigValue(configMap, clusterPaths.SSHKeyPath, "opencenter", "gitops", "auth", "ssh", "private_key")
		setNestedMigrationConfigValue(configMap, clusterPaths.SSHKeyPath+".pub", "opencenter", "gitops", "auth", "ssh", "public_key")
	}

	rewritten, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("marshaling migrated config %s: %w", to, err)
	}
	rewritten, err = v2.SanitizePublicYAML(rewritten)
	if err != nil {
		return fmt.Errorf("sanitizing migrated config %s: %w", to, err)
	}
	if err := os.WriteFile(to, rewritten, mode); err != nil {
		return fmt.Errorf("writing migrated config %s: %w", to, err)
	}
	return nil
}

func inferOrganizationFromLegacyConfigPath(path string) string {
	return filepath.Base(filepath.Dir(path))
}

func setNestedMigrationConfigValue(configMap map[string]any, value any, parts ...string) {
	if len(parts) == 0 {
		return
	}

	current := configMap
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func hasNestedMigrationConfigValue(configMap map[string]any, parts ...string) bool {
	current := any(configMap)
	for _, part := range parts {
		next, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current = next[part]
	}
	return current != nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
