package gitops

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

// secretsSyncLockFilename mirrors the secrets package's syncLockFilename (kept as
// a local literal to avoid an import cycle). It is a transient lock written by
// `secrets sync` and is not part of the generator-managed tree.
const secretsSyncLockFilename = ".opencenter-secrets.lock"

type generatedTreeFile struct {
	data []byte
	mode os.FileMode
}

type generatedTreePlan struct {
	result         *PromoteResult
	planned        map[string]generatedTreeFile
	seedWrites     map[string]generatedTreeFile
	deletes        []string
	adoptedBackups []backupFile
	manifest       GeneratedManifest
	legacyManifest string
}

type generatedTreeSnapshot struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

// generatedTreeMutationHook is a serial-test fault injection seam. Production
// leaves it nil.
var generatedTreeMutationHook func(string) error

// loadOwnedSecretTreePaths returns the set of secret-artifact-owned file paths,
// expressed relative to the git-dir tree root (i.e. prefixed with
// applications/overlays/<cluster>/), for paths whose on-disk content still
// matches the recorded hash in the secret-artifacts ownership ledger. These are
// managed by `secrets sync`, not the generator, so the complete-tree preflight
// must not treat them as foreign or prune them. Mirrors promoteOverlay's
// ownedSecretPaths logic, re-based from overlay-relative to tree-relative paths.
func loadOwnedSecretTreePaths(targetRoot, clusterName string) map[string]bool {
	owned := make(map[string]bool)
	if clusterName == "" {
		return owned
	}
	overlayDir := filepath.Join(targetRoot, "applications", "overlays", clusterName)
	state, _, err := secretartifacts.LoadOwnershipState(overlayDir)
	if err != nil {
		// A missing/unreadable ledger simply means no owned secret paths; the
		// preflight then behaves as before for those files.
		return owned
	}
	overlayPrefix := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName))
	for _, record := range state.Artifacts {
		full := filepath.Join(overlayDir, filepath.FromSlash(record.Path))
		data, readErr := os.ReadFile(full)
		if readErr == nil && secretartifacts.HashBytes(data) == record.Hash {
			treePath := filepath.ToSlash(filepath.Join(overlayPrefix, record.Path))
			owned[treePath] = true
		}
	}
	return owned
}

func promoteGeneratedTree(stageRoot, targetRoot, clusterName string, opts PromoteOptions) (*PromoteResult, error) {
	plan, err := planGeneratedTree(stageRoot, targetRoot, clusterName, opts)
	if err != nil {
		return nil, err
	}
	if opts.DryRun {
		return plan.result, nil
	}
	if err := applyGeneratedTreePlan(targetRoot, plan); err != nil {
		return nil, err
	}
	return plan.result, nil
}

func planGeneratedTree(stageRoot, targetRoot, clusterName string, opts PromoteOptions) (*generatedTreePlan, error) {
	if err := validateOwnershipRoot(targetRoot); err != nil {
		return nil, err
	}
	planned, seeds, warnings, err := scanStagedGeneratedTree(stageRoot)
	if err != nil {
		return nil, err
	}
	manifest, bootstrap, legacyManifest, err := loadGeneratedTreeManifest(targetRoot, clusterName)
	if err != nil {
		return nil, err
	}
	if manifest.Files == nil {
		manifest.Files = make(map[string]string)
	}

	// The generated-tree manifest is shared by all cluster generations in a
	// repository. Only ownership entries in this invocation's cluster scope may
	// participate in scanning and pruning; the complete manifest is retained so
	// applying this plan cannot discard sibling-cluster ownership.
	ownershipManifest := manifest
	ownershipManifest.Files = filterGeneratedTreeManifest(manifest.Files, clusterName)
	roots, rootFiles := generatedTreeNamespaces(planned, seeds, ownershipManifest.Files, clusterName)
	existing, existingModes, scanWarnings, err := scanLiveGeneratedTree(targetRoot, roots, rootFiles)
	if err != nil {
		return nil, err
	}
	result := &PromoteResult{Warnings: append(warnings, scanWarnings...)}
	adoptionCandidates := make(map[string]bool)
	unknown := make([]string, 0)

	// Secret artifacts (services/<svc>/secret.yaml) are owned by `secrets sync`,
	// not by the generator, so they are absent from both the tree manifest and the
	// freshly-planned set. Without this, a second `cluster generate` after a
	// `secrets sync` would misclassify them as user-authored files in
	// generator-owned paths and refuse to regenerate. Consult the secret-artifacts
	// ownership ledger (same approach as promoteOverlay) and treat hash-verified
	// secret paths as owned. The ledger lives in the overlay and records
	// overlay-relative paths, while this tree path is rooted at the git dir, so
	// re-base each ledger path under applications/overlays/<cluster>/.
	ownedSecretPaths := loadOwnedSecretTreePaths(targetRoot, clusterName)

	for path, onDisk := range existing {
		if ownedSecretPaths[path] {
			continue
		}
		if expectedHash, tracked := manifest.Files[path]; tracked {
			if hashBytes(onDisk) != expectedHash {
				return nil, fmt.Errorf("ownership conflict: refusing to overwrite modified tracked file %s", path)
			}
			continue
		}
		plannedFile, isPlanned := planned[path]
		if isPlanned {
			if hashBytes(onDisk) == hashBytes(plannedFile.data) {
				continue
			}
			if bootstrap || opts.AdoptGenerated {
				adoptionCandidates[path] = true
				continue
			}
			return nil, fmt.Errorf("ownership conflict: refusing to overwrite untracked planned file %s; rerun with --adopt-generated to back it up and claim it", path)
		}
		unknown = append(unknown, path)
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		if bootstrap {
			for _, path := range unknown {
				result.Warnings = append(result.Warnings, fmt.Sprintf("leaving legacy user-authored file untouched: %s", path))
			}
		} else {
			return nil, fmt.Errorf("refusing to regenerate: user-authored files found in generator-owned paths:\n  %s", strings.Join(unknown, "\n  "))
		}
	}

	plannedBytes := make(map[string][]byte, len(planned))
	for path, file := range planned {
		plannedBytes[path] = file.data
	}
	if err := validatePlannedTargets(targetRoot, plannedBytes); err != nil {
		return nil, err
	}

	seedWrites := make(map[string]generatedTreeFile)
	for path, file := range seeds {
		full := filepath.Join(targetRoot, filepath.FromSlash(path))
		if _, err := os.Lstat(full); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect seed target %s: %w", path, err)
		}
		seedWrites[path] = file
		result.Seeded = append(result.Seeded, path)
	}

	adoptedBackups := make([]backupFile, 0)
	for path, file := range planned {
		onDisk, exists := existing[path]
		if !exists {
			result.Added = append(result.Added, path)
			continue
		}
		if hashBytes(onDisk) == hashBytes(file.data) {
			result.Unchanged = append(result.Unchanged, path)
			continue
		}
		result.Updated = append(result.Updated, path)
		if adoptionCandidates[path] {
			result.Adopted = append(result.Adopted, path)
			adoptedBackups = append(adoptedBackups, backupFile{path: path, data: onDisk})
		}
	}

	prune := make([]string, 0)
	for path := range ownershipManifest.Files {
		if isGeneratedTreeCustomPath(path) {
			continue
		}
		if ownedSecretPaths[path] {
			// Never prune secret-artifact-owned files; they are managed by secrets sync.
			continue
		}
		if _, keep := planned[path]; keep {
			continue
		}
		if _, exists := existing[path]; exists {
			prune = append(prune, path)
		}
	}
	sort.Strings(prune)

	existingBytes := make(map[string][]byte, len(existing))
	for path, data := range existing {
		existingBytes[path] = data
	}
	renameSources := make(map[string]bool)
	if opts.pruneEnabled() {
		result.Renamed = detectSafeRenames(prune, result.Added, plannedBytes, existingBytes, ownershipManifest.Files)
		if len(result.Renamed) > 0 {
			renameTargets := make(map[string]bool)
			for _, rename := range result.Renamed {
				parts := strings.SplitN(rename, " -> ", 2)
				if len(parts) == 2 {
					renameSources[parts[0]] = true
					renameTargets[parts[1]] = true
				}
			}
			prune = filterTreePaths(prune, renameSources)
			result.Added = filterTreePaths(result.Added, renameTargets)
		}
		result.Pruned = append(result.Pruned, prune...)
	} else {
		result.PruneCandidates = append(result.PruneCandidates, prune...)
		if len(prune) > 0 {
			result.Warnings = append(result.Warnings, "prune disabled; candidates were reported but retained")
		}
	}

	deletes := append([]string(nil), result.Pruned...)
	for path := range renameSources {
		deletes = append(deletes, path)
	}
	if legacyManifest != "" {
		deletes = append(deletes, legacyManifest)
	}
	sort.Strings(deletes)

	for path, file := range planned {
		manifest.Files[path] = hashBytes(file.data)
	}
	if opts.pruneEnabled() {
		for _, path := range deletes {
			delete(manifest.Files, path)
		}
	}
	manifest.Version = ownershipManifestVersion
	manifest.Cluster = clusterName

	sort.Strings(result.Added)
	sort.Strings(result.Updated)
	sort.Strings(result.Unchanged)
	sort.Strings(result.Seeded)
	sort.Strings(result.Pruned)
	sort.Strings(result.PruneCandidates)
	sort.Strings(result.Renamed)
	sort.Strings(result.Adopted)
	sort.Strings(result.Warnings)

	_ = existingModes
	return &generatedTreePlan{
		result:         result,
		planned:        planned,
		seedWrites:     seedWrites,
		deletes:        deletes,
		adoptedBackups: adoptedBackups,
		manifest:       manifest,
		legacyManifest: legacyManifest,
	}, nil
}

func applyGeneratedTreePlan(targetRoot string, plan *generatedTreePlan) error {
	manifestData, err := json.MarshalIndent(plan.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal generated tree manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')

	preexistingDirs, err := snapshotGeneratedTreeDirectories(targetRoot)
	if err != nil {
		return err
	}
	snapshots := make(map[string]generatedTreeSnapshot)
	mutationOrder := make([]string, 0)
	capture := func(path string) error {
		if _, captured := snapshots[path]; captured {
			return nil
		}
		if err := validateGeneratedTreeMutationPath(targetRoot, path); err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			snapshots[path] = generatedTreeSnapshot{}
			mutationOrder = append(mutationOrder, path)
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to mutate non-regular generated path %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshots[path] = generatedTreeSnapshot{exists: true, data: data, mode: info.Mode().Perm()}
		mutationOrder = append(mutationOrder, path)
		return nil
	}
	mutate := func(path string, operation func() error) error {
		if err := capture(path); err != nil {
			return err
		}
		if generatedTreeMutationHook != nil {
			if err := generatedTreeMutationHook(path); err != nil {
				return err
			}
		}
		return operation()
	}
	rollback := func(cause error) error {
		var rollbackErrors []string
		for i := len(mutationOrder) - 1; i >= 0; i-- {
			path := mutationOrder[i]
			snapshot := snapshots[path]
			if snapshot.exists {
				if err := atomicWriteVerified(targetRoot, path, snapshot.data, snapshot.mode); err != nil {
					rollbackErrors = append(rollbackErrors, fmt.Sprintf("restore %s: %v", path, err))
				}
			} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				rollbackErrors = append(rollbackErrors, fmt.Sprintf("remove %s: %v", path, err))
			}
		}
		if err := removeNewEmptyGeneratedTreeDirs(targetRoot, preexistingDirs); err != nil {
			rollbackErrors = append(rollbackErrors, err.Error())
		}
		if len(rollbackErrors) > 0 {
			return fmt.Errorf("%w; rollback failed: %s", cause, strings.Join(rollbackErrors, "; "))
		}
		return cause
	}

	for _, rel := range plan.deletes {
		path := filepath.Join(targetRoot, filepath.FromSlash(rel))
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return rollback(err)
		}
		if err := mutate(path, func() error { return os.Remove(path) }); err != nil {
			return rollback(fmt.Errorf("delete %s: %w", rel, err))
		}
	}

	if len(plan.adoptedBackups) > 0 {
		backupRoot := filepath.Join(targetRoot, ".opencenter-backup", time.Now().UTC().Format("20060102T150405.000000000Z07:00"))
		for _, backup := range plan.adoptedBackups {
			path := filepath.Join(backupRoot, filepath.FromSlash(backup.path))
			if err := mutate(path, func() error { return atomicWriteVerified(targetRoot, path, backup.data, 0o600) }); err != nil {
				return rollback(fmt.Errorf("backup adopted file %s: %w", backup.path, err))
			}
			plan.result.BackupPaths = append(plan.result.BackupPaths, path)
			plan.result.Warnings = append(plan.result.Warnings, fmt.Sprintf("adopted %s; original backed up", backup.path))
		}
	}

	unchanged := make(map[string]bool)
	for _, path := range plan.result.Unchanged {
		unchanged[path] = true
	}
	writePaths := make([]string, 0, len(plan.planned))
	for path := range plan.planned {
		if !unchanged[path] {
			writePaths = append(writePaths, path)
		}
	}
	sort.Strings(writePaths)
	for _, rel := range writePaths {
		file := plan.planned[rel]
		path := filepath.Join(targetRoot, filepath.FromSlash(rel))
		if err := mutate(path, func() error { return atomicWriteVerified(targetRoot, path, file.data, file.mode) }); err != nil {
			return rollback(fmt.Errorf("write %s: %w", rel, err))
		}
	}
	seedPaths := make([]string, 0, len(plan.seedWrites))
	for path := range plan.seedWrites {
		seedPaths = append(seedPaths, path)
	}
	sort.Strings(seedPaths)
	for _, rel := range seedPaths {
		file := plan.seedWrites[rel]
		path := filepath.Join(targetRoot, filepath.FromSlash(rel))
		if err := mutate(path, func() error { return atomicWriteVerified(targetRoot, path, file.data, file.mode) }); err != nil {
			return rollback(fmt.Errorf("seed %s: %w", rel, err))
		}
	}
	manifestPath := filepath.Join(targetRoot, GeneratedManifestFile)
	if err := mutate(manifestPath, func() error { return atomicWriteVerified(targetRoot, manifestPath, manifestData, 0o644) }); err != nil {
		return rollback(fmt.Errorf("write generated tree manifest: %w", err))
	}
	sort.Strings(plan.result.BackupPaths)
	sort.Strings(plan.result.Warnings)
	return nil
}

func scanStagedGeneratedTree(root string) (map[string]generatedTreeFile, map[string]generatedTreeFile, []string, error) {
	planned := make(map[string]generatedTreeFile)
	seeds := make(map[string]generatedTreeFile)
	var warnings []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := safeRelativePath(root, path)
		if err != nil {
			return err
		}
		if rel == "." || isTempPath(rel) || filepath.Base(rel) == GeneratedManifestFile {
			if entry.IsDir() && isTempPath(rel) {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			warnings = append(warnings, fmt.Sprintf("skipping symlink in workspace: %s", rel))
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file := generatedTreeFile{data: data, mode: info.Mode().Perm()}
		if isGeneratedTreeCustomPath(rel) {
			seeds[rel] = file
		} else {
			planned[rel] = file
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scan staged generated tree: %w", err)
	}
	sort.Strings(warnings)
	return planned, seeds, warnings, nil
}

func scanLiveGeneratedTree(root string, roots, rootFiles map[string]bool) (map[string][]byte, map[string]os.FileMode, []string, error) {
	files := make(map[string][]byte)
	modes := make(map[string]os.FileMode)
	var warnings []string
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return files, modes, warnings, nil
	} else if err != nil {
		return nil, nil, nil, err
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := safeRelativePath(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() && (skipGeneratedTreeDir(rel) || isGeneratedTreeCustomPath(rel)) {
			return fs.SkipDir
		}
		// Skip generator/tooling state files that are not part of the managed tree:
		// the generated-tree manifest, generated backups, the secret-artifacts
		// ownership ledger, and the secrets-sync lock file (all written by tooling
		// such as `secrets sync`, not the generator).
		base := filepath.Base(rel)
		if base == GeneratedManifestFile || base == secretartifacts.OwnershipStateFilename || base == secretsSyncLockFilename || isGeneratedBackupPath(rel) {
			return nil
		}
		if !isGeneratedTreeNamespace(rel, roots, rootFiles) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to use symlinked generated path %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = data
		modes[rel] = info.Mode().Perm()
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scan live generated tree: %w", err)
	}
	sort.Strings(warnings)
	return files, modes, warnings, nil
}

func loadGeneratedTreeManifest(root, clusterName string) (GeneratedManifest, bool, string, error) {
	path := filepath.Join(root, GeneratedManifestFile)
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return GeneratedManifest{}, false, "", fmt.Errorf("refusing to read non-regular generated tree manifest %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return GeneratedManifest{}, false, "", err
		}
		var manifest GeneratedManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return GeneratedManifest{}, false, "", fmt.Errorf("read generated tree manifest: corrupt JSON: %w", err)
		}
		if manifest.Version != ownershipManifestVersion {
			return GeneratedManifest{}, false, "", fmt.Errorf("generated tree manifest version %d is unsupported", manifest.Version)
		}
		for rel := range manifest.Files {
			clean, err := safeRelativePath(root, filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil || clean != rel || isTempPath(rel) || isGeneratedTreeCustomPath(rel) {
				return GeneratedManifest{}, false, "", fmt.Errorf("generated tree manifest contains invalid path %q", rel)
			}
		}
		return manifest, false, "", nil
	}
	if err != nil && !os.IsNotExist(err) {
		return GeneratedManifest{}, false, "", err
	}

	manifest := GeneratedManifest{Version: ownershipManifestVersion, Cluster: clusterName, Files: make(map[string]string)}
	overlayRel := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName))
	overlayRoot := filepath.Join(root, filepath.FromSlash(overlayRel))
	legacyPath := filepath.Join(overlayRoot, GeneratedManifestFile)
	if _, err := os.Lstat(legacyPath); err == nil {
		legacy, _, err := loadGeneratedManifest(overlayRoot)
		if err != nil {
			return GeneratedManifest{}, false, "", err
		}
		for rel, hash := range legacy.Files {
			manifest.Files[filepath.ToSlash(filepath.Join(overlayRel, rel))] = hash
		}
		return manifest, true, filepath.ToSlash(filepath.Join(overlayRel, GeneratedManifestFile)), nil
	} else if err != nil && !os.IsNotExist(err) {
		return GeneratedManifest{}, false, "", err
	}
	return manifest, true, "", nil
}

// filterGeneratedTreeManifest returns the ownership entries visible to one
// cluster generation. Shared entries remain visible, while entries rooted in
// another cluster's overlay, infrastructure, or Flux bridge are excluded
// from both live scanning and prune planning.
func filterGeneratedTreeManifest(files map[string]string, clusterName string) map[string]string {
	filtered := make(map[string]string, len(files))
	targetScopes := map[string]bool{
		filepath.ToSlash(filepath.Join("applications", "overlays", clusterName)):   true,
		filepath.ToSlash(filepath.Join("infrastructure", "clusters", clusterName)): true,
		filepath.ToSlash(filepath.Join("clusters", clusterName)):                   true,
	}
	for path, hash := range files {
		if scope, clusterSpecific := generatedTreeClusterScope(path); clusterSpecific && !targetScopes[scope] {
			continue
		}
		filtered[path] = hash
	}
	return filtered
}

func generatedTreeClusterScope(path string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	if len(parts) >= 3 &&
		((parts[0] == "applications" && parts[1] == "overlays") ||
			(parts[0] == "infrastructure" && parts[1] == "clusters")) {
		return strings.Join(parts[:3], "/"), true
	}
	if len(parts) >= 2 && parts[0] == "clusters" {
		return strings.Join(parts[:2], "/"), true
	}
	return "", false
}

// generatedTreeNamespaces returns recursive scopes for the live-tree scan and
// exact paths for top-level files. The current cluster overlay is deliberately
// a recursive scope, but shared scopes are derived from paths that are
// actually planned, seeded, or recorded in the manifest. In particular, do
// not use "applications" as a recursive scope: doing so would inspect every
// cluster overlay while generating one cluster.
func generatedTreeNamespaces(planned, seeds map[string]generatedTreeFile, manifest map[string]string, clusterName string) (map[string]bool, map[string]bool) {
	roots := make(map[string]bool)
	rootFiles := make(map[string]bool)
	targetOverlay := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName))
	// The target overlay is always protected recursively, including when a
	// particular render happens to produce no files beneath it.
	roots[targetOverlay] = true
	add := func(path string) {
		path = filepath.ToSlash(filepath.Clean(path))
		if path == targetOverlay || strings.HasPrefix(path, targetOverlay+"/") {
			roots[targetOverlay] = true
			return
		}
		parts := strings.Split(path, "/")
		if len(parts) == 1 {
			rootFiles[path] = true
			return
		}
		scope := strings.Join(parts[:len(parts)-1], "/")
		// A shared file directly under a tree's cluster collection must remain
		// an exact-file scope. None of these parents may become a recursive
		// scope covering sibling clusters or overlays.
		if scope == "applications" || scope == "applications/overlays" ||
			scope == "infrastructure" || scope == "infrastructure/clusters" || scope == "clusters" {
			rootFiles[path] = true
			return
		}
		roots[scope] = true
	}
	for path := range planned {
		add(path)
	}
	for path := range seeds {
		add(path)
	}
	for path := range manifest {
		add(path)
	}
	return roots, rootFiles
}

func isGeneratedTreeNamespace(path string, roots, rootFiles map[string]bool) bool {
	path = filepath.ToSlash(path)
	if rootFiles[path] || roots[path] {
		return true
	}
	for scope := range roots {
		if strings.HasPrefix(scope, path+"/") || strings.HasPrefix(path, scope+"/") {
			return true
		}
	}
	for file := range rootFiles {
		if strings.HasPrefix(file, path+"/") {
			return true
		}
	}
	return false
}

func isGeneratedTreeCustomPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == CustomDirName {
			return true
		}
	}
	return false
}

func skipGeneratedTreeDir(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch part {
		case ".git", ".opencenter-backup", ".terraform", ".bin", "venv", "kubespray", ".tmp":
			return true
		}
		if strings.HasPrefix(part, ".opentofu-local") {
			return true
		}
	}
	return false
}

func isGeneratedBackupPath(path string) bool {
	return strings.Contains(filepath.Base(path), ".bak-")
}

func filterTreePaths(paths []string, excluded map[string]bool) []string {
	filtered := paths[:0]
	for _, path := range paths {
		if !excluded[path] {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func snapshotGeneratedTreeDirectories(root string) (map[string]bool, error) {
	dirs := make(map[string]bool)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return dirs, nil
	} else if err != nil {
		return nil, err
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			dirs[path] = true
		}
		return nil
	})
	return dirs, err
}

func removeNewEmptyGeneratedTreeDirs(root string, preexisting map[string]bool) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	var dirs []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && !preexisting[path] {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func validateGeneratedTreeMutationPath(root, path string) error {
	rel, err := safeRelativePath(root, path)
	if err != nil || rel == "." {
		return fmt.Errorf("generated tree path escapes target root: %s", path)
	}
	for current := filepath.Dir(path); current != filepath.Clean(root); current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to mutate through symlinked generated path %s", rel)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to mutate symlinked generated path %s", rel)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
