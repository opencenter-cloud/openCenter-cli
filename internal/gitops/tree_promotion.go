package gitops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type generatedTreeFile struct {
	data []byte
	mode os.FileMode
}

type generatedTreePlan struct {
	result              *PromoteResult
	planned             map[string]generatedTreeFile
	seedWrites          map[string]generatedTreeFile
	deletes             []string
	dirDeletes          []string
	adoptedBackups      []backupFile
	clusterManifest     GeneratedManifest
	globalManifest      GeneratedManifest
	clusterManifestPath string
	globalManifestPath  string
	preimages           map[string]generatedTreeSnapshot
}

type generatedTreeSnapshot struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

var generatedTreeMutationHook func(string) error

// generatedTreePostMutationHook is a test-only fault-injection seam for
// verifying rollback after a destructive mutation has completed. Production
// leaves it nil.
var generatedTreePostMutationHook func(string) error

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
	if err := validateClusterName(clusterName); err != nil {
		return nil, err
	}
	if err := validateOwnershipRoot(targetRoot); err != nil {
		return nil, err
	}
	planned, seeds, warnings, err := scanStagedGeneratedTree(stageRoot)
	if err != nil {
		return nil, err
	}
	active := append(repositoryClusterScopes(clusterName), ownershipMapKeys(globalOwnershipFiles)...)
	return planRepositoryPromotion(targetRoot, clusterName, planned, seeds, warnings, active, opts)
}

func ownershipMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func applyGeneratedTreePlan(targetRoot string, plan *generatedTreePlan) error {
	clusterData, err := manifestBytes(plan.clusterManifest)
	if err != nil {
		return fmt.Errorf("marshal cluster ownership manifest: %w", err)
	}
	var globalData []byte
	if plan.globalManifestPath != "" {
		globalData, err = manifestBytes(plan.globalManifest)
		if err != nil {
			return fmt.Errorf("marshal global ownership manifest: %w", err)
		}
	}
	snapshots := make(map[string]generatedTreeSnapshot)
	mutationOrder := make([]string, 0)
	removedDirs := make([]struct {
		path string
		mode os.FileMode
	}, 0, len(plan.dirDeletes))
	verify := func(path string) error {
		expected, ok := plan.preimages[path]
		if !ok {
			return nil
		}
		actual := snapshotPathOrMissing(path)
		if actual.exists != expected.exists || actual.mode.Perm() != expected.mode.Perm() || string(actual.data) != string(expected.data) {
			return fmt.Errorf("preimage changed for %s", path)
		}
		return nil
	}
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
	mutate := func(path string, operation func() error, checkPreimage bool) error {
		if checkPreimage {
			if err := verify(path); err != nil {
				return err
			}
		}
		if err := capture(path); err != nil {
			return err
		}
		if generatedTreeMutationHook != nil {
			if err := generatedTreeMutationHook(path); err != nil {
				return err
			}
		}
		if checkPreimage {
			if err := verify(path); err != nil {
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
		for i := len(removedDirs) - 1; i >= 0; i-- {
			dir := removedDirs[i]
			if err := os.MkdirAll(dir.path, dir.mode.Perm()); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Sprintf("restore directory %s: %v", dir.path, err))
			} else if err := os.Chmod(dir.path, dir.mode.Perm()); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Sprintf("restore directory mode %s: %v", dir.path, err))
			}
		}
		if len(rollbackErrors) > 0 {
			return fmt.Errorf("%w; rollback failed: %s", cause, strings.Join(rollbackErrors, "; "))
		}
		return cause
	}

	for _, rel := range plan.deletes {
		path := filepath.Join(targetRoot, filepath.FromSlash(rel))
		if err := mutate(path, func() error { return os.Remove(path) }, true); err != nil {
			return rollback(fmt.Errorf("delete %s: %w", rel, err))
		}
	}
	for _, dir := range plan.dirDeletes {
		if err := validateGeneratedTreeMutationPath(targetRoot, dir); err != nil {
			return rollback(err)
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return rollback(fmt.Errorf("inspect directory %s: %w", dir, err))
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return rollback(fmt.Errorf("refusing to remove non-regular generated directory %s", dir))
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return rollback(fmt.Errorf("read directory %s: %w", dir, err))
		}
		if len(entries) != 0 {
			return rollback(fmt.Errorf("generated directory %s is no longer empty", dir))
		}
		if generatedTreeMutationHook != nil {
			if err := generatedTreeMutationHook(dir); err != nil {
				return rollback(err)
			}
		}
		entries, err = os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			if err == nil {
				err = fmt.Errorf("directory is no longer empty")
			}
			return rollback(fmt.Errorf("remove directory %s: %w", dir, err))
		}
		removedDirs = append(removedDirs, struct {
			path string
			mode os.FileMode
		}{dir, info.Mode().Perm()})
		if err := os.Remove(dir); err != nil {
			return rollback(fmt.Errorf("remove directory %s: %w", dir, err))
		}
		if generatedTreePostMutationHook != nil {
			if err := generatedTreePostMutationHook(dir); err != nil {
				return rollback(err)
			}
		}
	}
	if len(plan.adoptedBackups) > 0 {
		backupRoot := filepath.Join(targetRoot, ".opencenter-backup", time.Now().UTC().Format("20060102T150405.000000000Z07:00"))
		for _, backup := range plan.adoptedBackups {
			path := filepath.Join(backupRoot, filepath.FromSlash(backup.path))
			if err := mutate(path, func() error { return atomicWriteVerified(targetRoot, path, backup.data, backup.mode) }, false); err != nil {
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
		file, path := plan.planned[rel], filepath.Join(targetRoot, filepath.FromSlash(rel))
		if err := mutate(path, func() error { return atomicWriteVerified(targetRoot, path, file.data, file.mode) }, true); err != nil {
			return rollback(fmt.Errorf("write %s: %w", rel, err))
		}
	}
	seedPaths := make([]string, 0, len(plan.seedWrites))
	for path := range plan.seedWrites {
		seedPaths = append(seedPaths, path)
	}
	sort.Strings(seedPaths)
	for _, rel := range seedPaths {
		file, path := plan.seedWrites[rel], filepath.Join(targetRoot, filepath.FromSlash(rel))
		if err := mutate(path, func() error { return atomicWriteVerified(targetRoot, path, file.data, file.mode) }, true); err != nil {
			return rollback(fmt.Errorf("seed %s: %w", rel, err))
		}
	}
	manifestWrites := []struct {
		path string
		data []byte
	}{{plan.clusterManifestPath, clusterData}}
	if plan.globalManifestPath != "" {
		manifestWrites = append(manifestWrites, struct {
			path string
			data []byte
		}{plan.globalManifestPath, globalData})
	}
	for _, write := range manifestWrites {
		if err := mutate(write.path, func() error { return atomicWriteVerified(targetRoot, write.path, write.data, 0o644) }, true); err != nil {
			return rollback(fmt.Errorf("write ownership manifest: %w", err))
		}
	}
	sort.Strings(plan.result.BackupPaths)
	sort.Strings(plan.result.Warnings)
	return nil
}

func scanStagedGeneratedTree(root string) (map[string]generatedTreeFile, map[string]generatedTreeFile, []string, error) {
	planned, seeds := make(map[string]generatedTreeFile), make(map[string]generatedTreeFile)
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

func loadGeneratedTreeManifest(root, clusterName string) (GeneratedManifest, bool, string, error) {
	state, err := loadOwnershipState(root, clusterName, true)
	if err != nil {
		return GeneratedManifest{}, false, "", err
	}
	return mergedManifest(state.cluster, state.global), state.clusterBoot || state.globalBoot, "", nil
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
