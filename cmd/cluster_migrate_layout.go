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
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type migrateLayoutOptions struct {
	organization string
	dryRun       bool
	force        bool
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
root. Use --dry-run to print the move diff without changing files.`,
		Example: `  # Preview migration for the acme organization
  opencenter cluster migrate-layout --org acme --dry-run

  # Perform migration, refusing to overwrite destinations
  opencenter cluster migrate-layout --org acme

  # Perform migration and replace existing destinations
  opencenter cluster migrate-layout --org acme --force`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(opts.organization) == "" {
				return fmt.Errorf("--org is required")
			}
			return runClusterMigrateLayout(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.organization, "org", "", "Organization name to migrate")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print planned moves without changing files")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Overwrite existing destination files")

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
		if movePlan[i].from == movePlan[j].from {
			return movePlan[i].to < movePlan[j].to
		}
		return movePlan[i].from < movePlan[j].from
	})
	return movePlan, nil
}

func migrationClusterPaths(organization, clusterName string) *paths.ClusterPaths {
	gitopsDir := filepath.Join(config.GetGitOpsDir(), organization)
	clusterStateDir := filepath.Join(config.GetClusterStateDir(), organization, clusterName)
	secretsDir := filepath.Join(config.GetSecretsDir(), organization, clusterName)

	return &paths.ClusterPaths{
		OrganizationDir: gitopsDir,
		GitOpsDir:       gitopsDir,
		ClusterStateDir: clusterStateDir,
		ClusterDir:      filepath.Join(gitopsDir, "infrastructure", "clusters", clusterName),
		ApplicationsDir: filepath.Join(gitopsDir, "applications", "overlays", clusterName),
		SecretsDir:      secretsDir,
		SOPSKeyPath:     filepath.Join(secretsDir, "age", "keys", clusterName+"-key.txt"),
		SOPSConfigPath:  filepath.Join(gitopsDir, ".sops.yaml"),
		KubeconfigPath:  filepath.Join(clusterStateDir, "kubeconfig.yaml"),
		InventoryPath:   filepath.Join(clusterStateDir, "inventory"),
		VenvPath:        filepath.Join(clusterStateDir, "venv"),
		BinPath:         filepath.Join(clusterStateDir, ".bin"),
		ConfigPath:      filepath.Join(clusterStateDir, clusterName+"-config.yaml"),
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
	defer src.Close()

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
