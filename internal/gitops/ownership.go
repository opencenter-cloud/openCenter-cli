package gitops

import (
	"crypto/sha256"
	"encoding/hex"
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

const GeneratedManifestFile = ".opencenter-generated.json"
const ownershipManifestVersion = 1
const CustomDirName = "custom"

var generatorOwnedOverlayRoots = []string{"services", "managed-services", "customer-managed"}
var generatorOwnedOverlayFiles = []string{"kustomization.yaml", ".sops.yaml"}

type GeneratedManifest struct {
	Version int               `json:"version"`
	Cluster string            `json:"cluster"`
	Files   map[string]string `json:"files"`
}

type PromoteOptions struct {
	// Prune defaults to true when nil for backward compatibility. A non-nil
	// false value reports prune candidates without deleting them.
	Prune          *bool
	Force          bool // retained for API compatibility; never enables adoption
	DryRun         bool
	Scope          []string
	AdoptGenerated bool
	BeforePromote  func() error
}

func (o PromoteOptions) pruneEnabled() bool {
	return o.Prune == nil || *o.Prune
}

type PromoteResult struct {
	Added, Updated, Unchanged, Pruned, PruneCandidates, Seeded []string
	Renamed, Adopted                                           []string
	BackupPaths                                                []string
	Warnings                                                   []string
}

type scannedOverlay struct {
	files    map[string][]byte
	warnings []string
}

func promoteOverlay(workspaceOverlayDir, targetOverlayDir, clusterName string, opts PromoteOptions) (*PromoteResult, error) {
	if info, err := os.Lstat(targetOverlayDir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to scan symlinked target overlay %s", targetOverlayDir)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect target overlay: %w", err)
	}
	planned, seeds, plannedWarnings, err := plannedOverlayFiles(workspaceOverlayDir)
	if err != nil {
		return nil, err
	}

	manifest, bootstrap, err := loadGeneratedManifest(targetOverlayDir)
	if err != nil {
		return nil, err
	}
	if manifest.Files == nil {
		manifest.Files = make(map[string]string)
	}

	existing, err := scanGeneratorOwnedOverlay(targetOverlayDir)
	if err != nil {
		return nil, err
	}

	secretState, _, err := secretartifacts.LoadOwnershipState(targetOverlayDir)
	if err != nil {
		return nil, err
	}
	ownedSecretPaths := make(map[string]bool)
	for _, record := range secretState.Artifacts {
		full := filepath.Join(targetOverlayDir, filepath.FromSlash(record.Path))
		data, readErr := os.ReadFile(full)
		if readErr == nil && secretartifacts.HashBytes(data) == record.Hash {
			ownedSecretPaths[record.Path] = true
		}
	}

	result := &PromoteResult{Warnings: append(plannedWarnings, existing.warnings...)}
	adoptionCandidates := make(map[string]bool)
	unknown := make([]string, 0)
	for path, onDisk := range existing.files {
		if !scopeAllows(path, opts.Scope) {
			continue
		}
		if ownedSecretPaths[path] {
			continue
		}
		if expectedHash, tracked := manifest.Files[path]; tracked {
			if hashBytes(onDisk) != expectedHash {
				return nil, fmt.Errorf("ownership conflict: refusing to overwrite modified tracked file %s", path)
			}
			continue
		}
		plannedData, planned := planned[path]
		if planned {
			if hashBytes(onDisk) == hashBytes(plannedData) {
				// An identical planned file is safe to claim without requiring an
				// explicit adoption flag.
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
			return nil, fmt.Errorf("refusing to regenerate: user-authored files found in generator-owned paths:\n  %s\nmove these under a \"custom/\" directory in the service overlay, or run \"opencenter cluster migrate-layout %s\"", strings.Join(unknown, "\n  "), clusterName)
		}
	}

	if err := validatePlannedTargets(targetOverlayDir, planned); err != nil {
		return nil, err
	}

	seedWrites := make(map[string][]byte)
	for path, data := range seeds {
		full := filepath.Join(targetOverlayDir, filepath.FromSlash(path))
		if _, err := os.Lstat(full); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect seed target %s: %w", path, err)
		}
		seedWrites[path] = data
		result.Seeded = append(result.Seeded, path)
	}
	if err := validatePlannedTargets(targetOverlayDir, seedWrites); err != nil {
		return nil, err
	}

	adoptedBackups := make([]backupFile, 0)
	for path, data := range planned {
		onDisk, exists := existing.files[path]
		if !exists {
			result.Added = append(result.Added, path)
			continue
		}
		if hashBytes(onDisk) == hashBytes(data) {
			result.Unchanged = append(result.Unchanged, path)
		} else {
			result.Updated = append(result.Updated, path)
			if adoptionCandidates[path] {
				result.Adopted = append(result.Adopted, path)
				adoptedBackups = append(adoptedBackups, backupFile{path: path, data: onDisk})
			}
		}
	}

	prune := make([]string, 0)
	for path, expectedHash := range manifest.Files {
		if !isGeneratorOwnedPath(path) || isCustomPath(path) || !scopeAllows(path, opts.Scope) {
			continue
		}
		if _, keep := planned[path]; keep {
			continue
		}
		if onDisk, exists := existing.files[path]; exists {
			if hashBytes(onDisk) != expectedHash {
				return nil, fmt.Errorf("ownership conflict: refusing to prune modified generated file %s", path)
			}
			prune = append(prune, path)
		}
	}
	sort.Strings(prune)
	renameSources := make(map[string]bool)
	if opts.pruneEnabled() {
		result.Renamed = detectSafeRenames(prune, result.Added, planned, existing.files, manifest.Files)
		if len(result.Renamed) > 0 {
			renameTo := make(map[string]bool)
			for _, rename := range result.Renamed {
				parts := strings.SplitN(rename, " -> ", 2)
				if len(parts) == 2 {
					renameSources[parts[0]] = true
					renameTo[parts[1]] = true
				}
			}
			filteredPrune := prune[:0]
			for _, path := range prune {
				if !renameSources[path] {
					filteredPrune = append(filteredPrune, path)
				}
			}
			prune = filteredPrune
			filteredAdded := result.Added[:0]
			for _, path := range result.Added {
				if !renameTo[path] {
					filteredAdded = append(filteredAdded, path)
				}
			}
			result.Added = filteredAdded
		}
	}
	if opts.pruneEnabled() {
		result.Pruned = append(result.Pruned, prune...)
	} else {
		result.PruneCandidates = append(result.PruneCandidates, prune...)
	}
	sort.Strings(result.Added)
	sort.Strings(result.Updated)
	sort.Strings(result.Unchanged)
	sort.Strings(result.Seeded)
	sort.Strings(result.Pruned)
	sort.Strings(result.PruneCandidates)
	sort.Strings(result.Renamed)
	sort.Strings(result.Adopted)
	if !opts.pruneEnabled() && len(prune) > 0 {
		result.Warnings = append(result.Warnings, "prune disabled; candidates were reported but retained")
	}
	sort.Strings(result.Warnings)

	if opts.DryRun {
		return result, nil
	}
	if opts.BeforePromote != nil {
		if err := opts.BeforePromote(); err != nil {
			return nil, fmt.Errorf("pre-promotion preparation failed: %w", err)
		}
	}
	if err := os.MkdirAll(targetOverlayDir, 0o755); err != nil {
		return nil, fmt.Errorf("create target overlay: %w", err)
	}

	backupPaths, err := writeAdoptionBackups(targetOverlayDir, adoptedBackups)
	if err != nil {
		return nil, err
	}
	result.BackupPaths = append(result.BackupPaths, backupPaths...)
	for _, path := range result.Adopted {
		result.Warnings = append(result.Warnings, fmt.Sprintf("adopted %s; original backed up", path))
	}
	if opts.pruneEnabled() {
		deletePaths := append([]string(nil), prune...)
		for path := range renameSources {
			deletePaths = append(deletePaths, path)
		}
		sort.Strings(deletePaths)
		for _, path := range deletePaths {
			expectedHash := manifest.Files[path]
			if err := removeVerifiedFileMatching(targetOverlayDir, filepath.Join(targetOverlayDir, filepath.FromSlash(path)), expectedHash); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("prune %s: %w", path, err)
			}
		}
		if err := removeEmptyOwnedDirs(targetOverlayDir); err != nil {
			return nil, err
		}
	}
	for path, data := range planned {
		dst := filepath.Join(targetOverlayDir, filepath.FromSlash(path))
		if err := atomicWriteVerified(targetOverlayDir, dst, data, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
	}
	for path, data := range seedWrites {
		dst := filepath.Join(targetOverlayDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, fmt.Errorf("create parent for seed %s: %w", path, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, fmt.Errorf("write seed %s: %w", path, err)
		}
	}

	for path, data := range planned {
		manifest.Files[path] = hashBytes(data)
	}
	for _, path := range prune {
		if opts.pruneEnabled() {
			delete(manifest.Files, path)
		}
	}
	if opts.pruneEnabled() {
		for _, rename := range result.Renamed {
			if parts := strings.SplitN(rename, " -> ", 2); len(parts) == 2 {
				delete(manifest.Files, parts[0])
			}
		}
	}
	manifest.Version = ownershipManifestVersion
	manifest.Cluster = clusterName
	if err := writeGeneratedManifest(targetOverlayDir, manifest); err != nil {
		return nil, err
	}
	return result, nil
}

type backupFile struct {
	path string
	data []byte
}

func plannedOverlayFiles(root string) (map[string][]byte, map[string][]byte, []string, error) {
	planned := make(map[string][]byte)
	seeds := make(map[string][]byte)
	warnings := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := safeRelativePath(root, path)
		if err != nil {
			return err
		}
		if rel == "." || isTempPath(rel) || rel == GeneratedManifestFile {
			return nil
		}
		custom := isCustomPath(rel)
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
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read planned file %s: %w", rel, err)
		}
		if !isGeneratorOwnedPath(rel) {
			return nil
		}
		if custom {
			seeds[rel] = data
			return nil
		}
		planned[rel] = data
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scan workspace overlay: %w", err)
	}
	sort.Strings(warnings)
	return planned, seeds, warnings, nil
}

func scanGeneratorOwnedOverlay(root string) (scannedOverlay, error) {
	result := scannedOverlay{files: make(map[string][]byte)}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return result, nil
	} else if err != nil {
		return result, err
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := safeRelativePath(root, path)
		if err != nil {
			return err
		}
		if rel == "." || rel == GeneratedManifestFile || !isGeneratorOwnedPath(rel) {
			if rel != "." && entry.IsDir() && !isGeneratorOwnedPath(rel) {
				return fs.SkipDir
			}
			return nil
		}
		if isGeneratedBackupPath(rel) {
			return nil
		}
		if isCustomPath(rel) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			result.warnings = append(result.warnings, fmt.Sprintf("skipping symlink in target: %s", rel))
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read target file %s: %w", rel, err)
		}
		result.files[rel] = data
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("scan target overlay: %w", err)
	}
	sort.Strings(result.warnings)
	return result, nil
}

func loadGeneratedManifest(root string) (GeneratedManifest, bool, error) {
	if err := validateOwnershipRoot(root); err != nil {
		return GeneratedManifest{}, false, err
	}
	path := filepath.Join(root, GeneratedManifestFile)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return GeneratedManifest{Version: ownershipManifestVersion, Files: make(map[string]string)}, true, nil
	}
	if err != nil {
		return GeneratedManifest{}, false, fmt.Errorf("inspect generated manifest: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return GeneratedManifest{}, false, fmt.Errorf("refusing to read symlinked generated manifest %s", path)
	}
	if !info.Mode().IsRegular() {
		return GeneratedManifest{}, false, fmt.Errorf("refusing to read non-regular generated manifest %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return GeneratedManifest{}, false, fmt.Errorf("read generated manifest: %w", err)
	}
	var manifest GeneratedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return GeneratedManifest{}, false, fmt.Errorf("read generated manifest: corrupt JSON: %w", err)
	}
	if manifest.Version > ownershipManifestVersion {
		return GeneratedManifest{}, false, fmt.Errorf("generated manifest version %d is newer than this CLI supports; upgrade the CLI", manifest.Version)
	}
	if manifest.Version != ownershipManifestVersion {
		return GeneratedManifest{}, false, fmt.Errorf("generated manifest version %d is unsupported; upgrade the CLI", manifest.Version)
	}
	for path := range manifest.Files {
		clean, err := safeRelativePath(root, filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || clean != path || !isGeneratorOwnedPath(path) {
			return GeneratedManifest{}, false, fmt.Errorf("generated manifest contains invalid path %q", path)
		}
	}
	return manifest, false, nil
}

func writeGeneratedManifest(root string, manifest GeneratedManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal generated manifest: %w", err)
	}
	data = append(data, '\n')
	if err := validateOwnershipRoot(root); err != nil {
		return err
	}
	path := filepath.Join(root, GeneratedManifestFile)
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlinked generated manifest %s", path)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect generated manifest: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".opencenter-generated-")
	if err != nil {
		return fmt.Errorf("create generated manifest temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write generated manifest temporary file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod generated manifest temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close generated manifest temporary file: %w", err)
	}
	if err := validateOwnershipRoot(root); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlinked generated manifest %s", path)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect generated manifest: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace generated manifest: %w", err)
	}
	return nil
}

func validateOwnershipRoot(root string) error {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve overlay root %s: %w", root, err)
	}
	info, err := os.Lstat(absolute)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect overlay path %s: %w", absolute, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to use symlinked overlay path %s", absolute)
	}
	if !info.IsDir() {
		return fmt.Errorf("overlay root %s is not a directory", root)
	}
	return nil
}

func writeAdoptionBackups(target string, files []backupFile) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(target)))
	backupRoot := filepath.Join(repoRoot, ".opencenter-backup", time.Now().UTC().Format("20060102T150405.000000000Z07:00"))
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(backupRoot, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create backup for %s: %w", file.path, err)
		}
		if err := os.WriteFile(path, file.data, 0o644); err != nil {
			return nil, fmt.Errorf("write backup for %s: %w", file.path, err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func detectSafeRenames(pruned, added []string, planned, existing map[string][]byte, manifest map[string]string) []string {
	byHash := make(map[string][]string)
	for _, path := range added {
		byHash[hashBytes(planned[path])] = append(byHash[hashBytes(planned[path])], path)
	}
	staleByHash := make(map[string][]string)
	for _, path := range pruned {
		data, ok := existing[path]
		if !ok || manifest[path] != hashBytes(data) {
			continue
		}
		staleByHash[hashBytes(data)] = append(staleByHash[hashBytes(data)], path)
	}
	renamed := make([]string, 0)
	for hash, stale := range staleByHash {
		candidates := byHash[hash]
		if len(stale) == 1 && len(candidates) == 1 {
			renamed = append(renamed, stale[0]+" -> "+candidates[0])
		}
	}
	sort.Strings(renamed)
	return renamed
}

func validatePlannedTargets(root string, planned map[string][]byte) error {
	for path := range planned {
		full := filepath.Join(root, filepath.FromSlash(path))
		for current := filepath.Dir(full); current != root && current != "."; current = filepath.Dir(current) {
			info, err := os.Lstat(current)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to write through symlink in target path %s", path)
			}
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("inspect target path %s: %w", path, err)
			}
			if filepath.Dir(current) == current {
				break
			}
		}
		if info, err := os.Lstat(full); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink in target path %s", path)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect target file %s: %w", path, err)
		} else if err == nil && info.IsDir() {
			return fmt.Errorf("refusing to overwrite directory with generated file %s", path)
		}
	}
	return nil
}

func removeEmptyOwnedDirs(root string) error {
	var dirs []string
	for _, owned := range generatorOwnedOverlayRoots {
		path := filepath.Join(root, owned)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				rel, err := safeRelativePath(root, current)
				if err != nil {
					return err
				}
				if isCustomPath(rel) {
					return fs.SkipDir
				}
				dirs = append(dirs, current)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], string(filepath.Separator)) > strings.Count(dirs[j], string(filepath.Separator))
	})
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if len(entries) == 0 && filepath.Clean(dir) != filepath.Clean(root) {
			if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func safeRelativePath(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("path %q escapes overlay root", path)
	}
	return filepath.ToSlash(clean), nil
}

func normalizeOwnershipPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("path %q escapes overlay root", path)
	}
	return safeRelativePath(".", path)
}

func isGeneratorOwnedPath(path string) bool {
	path = filepath.ToSlash(path)
	for _, file := range generatorOwnedOverlayFiles {
		if path == file {
			return true
		}
	}
	for _, root := range generatorOwnedOverlayRoots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func isCustomPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 2 {
		return false
	}
	ownedRoot := false
	for _, root := range generatorOwnedOverlayRoots {
		if parts[0] == root {
			ownedRoot = true
			break
		}
	}
	if !ownedRoot {
		return false
	}
	for _, part := range parts[1:] {
		if part == CustomDirName {
			return true
		}
	}
	return false
}

func scopeAllows(path string, scopes []string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, scope := range scopes {
		scope = filepath.ToSlash(filepath.Clean(scope))
		if path == scope || strings.HasPrefix(path, scope+"/") || strings.HasPrefix(scope, path+"/") {
			return true
		}
	}
	return false
}

func isTempPath(path string) bool {
	return path == ".tmp" || strings.HasPrefix(path, ".tmp/")
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func atomicWriteVerified(root, dst string, data []byte, mode os.FileMode) error {
	if err := validatePlannedTargets(root, map[string][]byte{mustRelative(root, dst): data}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := validatePlannedTargets(root, map[string][]byte{mustRelative(root, dst): data}); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".opencenter-write-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink %s", dst)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, dst)
}

func mustRelative(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
}

func removeVerifiedFileMatching(root, path, expectedHash string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || !isGeneratorOwnedPath(filepath.ToSlash(rel)) {
		return fmt.Errorf("path escapes overlay")
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to delete symlink %s", path)
	} else if err != nil {
		return err
	}
	for current := filepath.Dir(path); current != filepath.Clean(root); current = filepath.Dir(current) {
		if info, err := os.Lstat(current); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to delete through symlink %s", current)
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if expectedHash != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if hashBytes(data) != expectedHash {
			return fmt.Errorf("ownership conflict: refusing to prune modified generated file %s", filepath.ToSlash(rel))
		}
	}
	return os.Remove(path)
}
