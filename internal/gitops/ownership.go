package gitops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
)

// GeneratedManifestFile is a legacy sentinel. Legacy manifests are refused;
// ownership is recorded only in the v2 ledgers below.
const GeneratedManifestFile = ".opencenter-generated.json"
const ownershipManifestVersion = 2
const CustomDirName = "custom"

const (
	ownershipDir            = ".opencenter/ownership"
	ownershipClustersDir    = ownershipDir + "/clusters"
	ownershipGlobalFile     = ownershipDir + "/global.json"
	ownershipClusterPrefix  = "applications/overlays/"
	secretsSyncLockFilename = ".opencenter-secrets.lock"
)

var globalOwnershipFiles = map[string]bool{
	".gitignore":                       true,
	"README.md":                        true,
	"applications/overlays/.gitkeep":   true,
	"infrastructure/clusters/.gitkeep": true,
}

var generatorOwnedOverlayRoots = []string{"services", "managed-services", "customer-managed"}
var generatorOwnedOverlayFiles = []string{"kustomization.yaml", ".sops.yaml"}
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// GeneratedManifest is the v2 ownership ledger. File names are repository
// relative, while mode is the permission-bit preimage recorded at promotion.
type GeneratedManifest struct {
	Version     int                          `json:"version"`
	Scope       string                       `json:"scope"`
	Identity    string                       `json:"identity"`
	Files       map[string]string            `json:"-"` // legacy migration API
	FileRecords map[string]GeneratedFileInfo `json:"-"`
}

type GeneratedFileInfo struct {
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

func (m GeneratedManifest) MarshalJSON() ([]byte, error) {
	files := m.FileRecords
	if files == nil {
		files = make(map[string]GeneratedFileInfo, len(m.Files))
		for path, hash := range m.Files {
			files[path] = GeneratedFileInfo{SHA256: hash, Mode: 0o644}
		}
	}
	return json.Marshal(struct {
		Version  int                          `json:"version"`
		Scope    string                       `json:"scope"`
		Identity string                       `json:"identity"`
		Files    map[string]GeneratedFileInfo `json:"files"`
	}{m.Version, m.Scope, m.Identity, files})
}

func (m *GeneratedManifest) UnmarshalJSON(data []byte) error {
	var wire struct {
		Version  int                        `json:"version"`
		Scope    string                     `json:"scope"`
		Identity string                     `json:"identity"`
		Files    map[string]json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	m.Version, m.Scope, m.Identity = wire.Version, wire.Scope, wire.Identity
	m.Files = make(map[string]string)
	m.FileRecords = make(map[string]GeneratedFileInfo)
	for path, raw := range wire.Files {
		var record GeneratedFileInfo
		if err := json.Unmarshal(raw, &record); err == nil && record.SHA256 != "" {
			m.FileRecords[path] = record
			continue
		}
		var legacy string
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return fmt.Errorf("invalid file record %q", path)
		}
		m.Files[path] = legacy
	}
	return nil
}

type PromoteOptions struct {
	Prune          *bool
	Force          bool // compatibility only; never bypasses ownership checks
	DryRun         bool
	Scope          []string
	AdoptGenerated bool
	BeforePromote  func() error
}

func (o PromoteOptions) pruneEnabled() bool { return o.Prune == nil || *o.Prune }

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
	if err := validateClusterName(clusterName); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(targetOverlayDir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to scan symlinked target overlay %s", targetOverlayDir)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect target overlay: %w", err)
	}
	repoRoot, err := repositoryRootForOverlay(targetOverlayDir, clusterName)
	if err != nil {
		return nil, err
	}
	planned, seeds, warnings, err := plannedOverlayFiles(workspaceOverlayDir)
	if err != nil {
		return nil, err
	}
	plannedTree := make(map[string]generatedTreeFile, len(planned))
	for path, data := range planned {
		plannedTree[repositoryOverlayPath(clusterName, path)] = generatedTreeFile{data: data, mode: 0o644}
	}
	seedTree := make(map[string]generatedTreeFile, len(seeds))
	for path, data := range seeds {
		seedTree[repositoryOverlayPath(clusterName, path)] = generatedTreeFile{data: data, mode: 0o644}
	}
	plan, err := planRepositoryPromotion(repoRoot, clusterName, plannedTree, seedTree, warnings, overlayPromotionScopes(clusterName, opts.Scope), opts)
	if err != nil {
		return nil, err
	}
	if opts.DryRun {
		return plan.result, nil
	}
	if opts.BeforePromote != nil {
		if err := opts.BeforePromote(); err != nil {
			return nil, fmt.Errorf("pre-promotion preparation failed: %w", err)
		}
	}
	if err := applyGeneratedTreePlan(repoRoot, plan); err != nil {
		return nil, err
	}
	return plan.result, nil
}

type ownershipState struct {
	cluster, global         GeneratedManifest
	clusterPath, globalPath string
	clusterBoot, globalBoot bool
	globalActive            bool
}

func validateClusterName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("invalid cluster name %q", name)
	}
	for _, r := range name {
		if r < 0x20 || r == filepath.Separator {
			return fmt.Errorf("invalid cluster name %q", name)
		}
	}
	return nil
}

func repositoryRootForOverlay(targetOverlayDir, clusterName string) (string, error) {
	if err := validateClusterName(clusterName); err != nil {
		return "", err
	}
	clean := filepath.Clean(targetOverlayDir)
	if filepath.Base(clean) != clusterName || filepath.Base(filepath.Dir(clean)) != "overlays" || filepath.Base(filepath.Dir(filepath.Dir(clean))) != "applications" {
		return "", fmt.Errorf("target overlay %s is not applications/overlays/%s", targetOverlayDir, clusterName)
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(clean))), nil
}

func repositoryOverlayPath(clusterName, path string) string {
	return filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, filepath.FromSlash(path)))
}

func repositoryClusterScopes(clusterName string) []string {
	return []string{
		filepath.ToSlash(filepath.Join("applications", "overlays", clusterName)),
		filepath.ToSlash(filepath.Join("infrastructure", "clusters", clusterName)),
		filepath.ToSlash(filepath.Join("clusters", clusterName)),
	}
}

func overlayPromotionScopes(clusterName string, scopes []string) []string {
	base := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName))
	if len(scopes) == 0 {
		return []string{base}
	}
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = filepath.ToSlash(filepath.Clean(scope))
		if scope == base || strings.HasPrefix(scope, base+"/") {
			result = append(result, scope)
		} else {
			result = append(result, filepath.ToSlash(filepath.Join(base, scope)))
		}
	}
	return result
}

func ownershipPathAllowed(path, clusterName string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	if globalOwnershipFiles[path] {
		return true
	}
	for _, scope := range repositoryClusterScopes(clusterName) {
		if path == scope || strings.HasPrefix(path, scope+"/") {
			if isGeneratedTreeCustomPath(path) {
				return false
			}
			fluxPrefix := filepath.ToSlash(filepath.Join("clusters", clusterName, "flux-system"))
			if path == fluxPrefix || strings.HasPrefix(path, fluxPrefix+"/") {
				return false
			}
			return true
		}
	}
	return false
}

func ownershipPathInScopes(path string, scopes []string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, scope := range scopes {
		scope = filepath.ToSlash(filepath.Clean(scope))
		if path == scope || strings.HasPrefix(path, scope+"/") {
			return true
		}
	}
	return false
}

func ownershipScopesMayContain(path string, scopes []string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, scope := range scopes {
		scope = filepath.ToSlash(filepath.Clean(scope))
		if path == scope || strings.HasPrefix(path, scope+"/") || strings.HasPrefix(scope, path+"/") {
			return true
		}
	}
	return false
}

func globalScopeActive(scopes []string) bool {
	for path := range globalOwnershipFiles {
		if ownershipPathInScopes(path, scopes) {
			return true
		}
	}
	return false
}

func ownershipManifestPath(root, clusterName string) (string, string) {
	return filepath.Join(root, filepath.FromSlash(filepath.Join(ownershipClustersDir, clusterName+".json"))), filepath.Join(root, filepath.FromSlash(ownershipGlobalFile))
}

func rejectLegacyOwnershipManifests(root, clusterName string) error {
	paths := []string{filepath.Join(root, GeneratedManifestFile)}
	if clusterName != "" {
		paths = append(paths, filepath.Join(root, filepath.FromSlash(repositoryOverlayPath(clusterName, GeneratedManifestFile))))
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("legacy ownership manifest %s is unsupported; remove it before promotion", filepath.ToSlash(path))
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect legacy ownership manifest %s: %w", path, err)
		}
	}
	return nil
}

func loadOwnershipManifest(path, expectedIdentity, expectedScope string, global bool) (GeneratedManifest, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return GeneratedManifest{Version: ownershipManifestVersion, Scope: expectedScope, Identity: expectedIdentity, FileRecords: make(map[string]GeneratedFileInfo)}, true, nil
	}
	if err != nil {
		return GeneratedManifest{}, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return GeneratedManifest{}, false, fmt.Errorf("refusing to read non-regular ownership manifest %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return GeneratedManifest{}, false, err
	}
	var manifest GeneratedManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return GeneratedManifest{}, false, fmt.Errorf("read ownership manifest %s: corrupt JSON: %w", path, err)
	}
	if manifest.Version != ownershipManifestVersion {
		return GeneratedManifest{}, false, fmt.Errorf("ownership manifest %s version %d is unsupported", path, manifest.Version)
	}
	if manifest.Scope != expectedScope || manifest.Identity != expectedIdentity {
		return GeneratedManifest{}, false, fmt.Errorf("ownership manifest %s has scope identity %q/%q; expected %q/%q", path, manifest.Scope, manifest.Identity, expectedScope, expectedIdentity)
	}
	if len(manifest.Files) != 0 {
		return GeneratedManifest{}, false, fmt.Errorf("ownership manifest %s contains legacy string file records", path)
	}
	if manifest.FileRecords == nil {
		manifest.FileRecords = make(map[string]GeneratedFileInfo)
	}
	for rel, record := range manifest.FileRecords {
		clean, pathErr := normalizeOwnershipPath(rel)
		validPath := pathErr == nil && clean == filepath.ToSlash(rel)
		validPath = validPath && ((global && globalOwnershipFiles[rel]) || (!global && ownershipPathAllowed(rel, expectedIdentity) && !globalOwnershipFiles[rel]))
		if !validPath || !sha256Pattern.MatchString(record.SHA256) || record.Mode&^0o777 != 0 {
			return GeneratedManifest{}, false, fmt.Errorf("ownership manifest %s contains invalid file record %q", path, rel)
		}
	}
	return manifest, false, nil
}

func loadOwnershipState(root, clusterName string, globalRequested ...bool) (ownershipState, error) {
	if err := validateClusterName(clusterName); err != nil {
		return ownershipState{}, err
	}
	if err := validateOwnershipRoot(root); err != nil {
		return ownershipState{}, err
	}
	if err := rejectLegacyOwnershipManifests(root, clusterName); err != nil {
		return ownershipState{}, err
	}
	includeGlobal := len(globalRequested) > 0 && globalRequested[0]
	clusterPath, globalPath := ownershipManifestPath(root, clusterName)
	cluster, clusterBoot, err := loadOwnershipManifest(clusterPath, clusterName, "cluster", false)
	if err != nil {
		return ownershipState{}, err
	}
	state := ownershipState{cluster: cluster, clusterPath: clusterPath, globalPath: globalPath, clusterBoot: clusterBoot, globalActive: includeGlobal}
	if includeGlobal {
		state.global, state.globalBoot, err = loadOwnershipManifest(globalPath, "repository", "global", true)
		if err != nil {
			return ownershipState{}, err
		}
	}
	return state, nil
}

func manifestBytes(manifest GeneratedManifest) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func planRepositoryPromotion(root, clusterName string, planned, seeds map[string]generatedTreeFile, warnings, scopes []string, opts PromoteOptions) (*generatedTreePlan, error) {
	state, err := loadOwnershipState(root, clusterName, globalScopeActive(scopes))
	if err != nil {
		return nil, err
	}
	for path := range planned {
		if !ownershipPathAllowed(path, clusterName) || !ownershipPathInScopes(path, scopes) {
			return nil, fmt.Errorf("generated file %q is outside the ownership policy", path)
		}
	}
	for path := range seeds {
		if (!ownershipPathAllowed(path, clusterName) && !isGeneratedTreeCustomPath(path)) || !ownershipPathInScopes(path, scopes) {
			return nil, fmt.Errorf("seed file %q is outside the ownership policy", path)
		}
	}
	known := make(map[string]GeneratedFileInfo, len(state.cluster.FileRecords)+len(state.global.FileRecords))
	for path, record := range state.cluster.FileRecords {
		known[path] = record
	}
	if state.globalActive {
		for path, record := range state.global.FileRecords {
			known[path] = record
		}
	}
	existing, existingModes, scanWarnings, err := scanLiveRepositoryTree(root, clusterName, scopes)
	if err != nil {
		return nil, err
	}
	result := &PromoteResult{Warnings: append(append([]string(nil), warnings...), scanWarnings...)}
	adoption := make(map[string]bool)
	var unknown []string
	for path, onDisk := range existing {
		if ownedSecretTreePath(root, clusterName, path) {
			continue
		}
		if expected, tracked := known[path]; tracked {
			if hashBytes(onDisk) != expected.SHA256 || uint32(existingModes[path].Perm()) != expected.Mode {
				return nil, fmt.Errorf("ownership conflict: refusing to overwrite modified tracked file %s", path)
			}
			continue
		}
		if file, isPlanned := planned[path]; isPlanned {
			if hashBytes(onDisk) == hashBytes(file.data) && uint32(existingModes[path].Perm()) == uint32(file.mode.Perm()) {
				continue // bootstrap may claim an identical planned file
			}
			// Repository initializers may materialize one of the explicit global
			// seed files before the first ownership ledger exists. These exact
			// paths are the only bootstrap exception: claim them with a backup,
			// while continuing to refuse arbitrary generated-file adoption.
			if opts.AdoptGenerated || (state.globalActive && state.globalBoot && globalOwnershipFiles[path]) {
				adoption[path] = true
				continue
			}
			return nil, fmt.Errorf("ownership conflict: refusing to overwrite untracked planned file %s; rerun with --adopt-generated to back it up and claim it", path)
		}
		unknown = append(unknown, path)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("refusing to regenerate: user-authored files found in generator-owned paths:\n  %s", strings.Join(unknown, "\n  "))
	}
	plannedBytes := make(map[string][]byte, len(planned))
	for path, file := range planned {
		plannedBytes[path] = file.data
	}
	if err := validatePlannedTargets(root, plannedBytes); err != nil {
		return nil, err
	}
	seedWrites := make(map[string]generatedTreeFile)
	for path, file := range seeds {
		full := filepath.Join(root, filepath.FromSlash(path))
		if _, err := os.Lstat(full); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect seed target %s: %w", path, err)
		}
		seedWrites[path] = file
		result.Seeded = append(result.Seeded, path)
	}
	var adoptedBackups []backupFile
	for path, file := range planned {
		onDisk, exists := existing[path]
		if !exists {
			result.Added = append(result.Added, path)
		} else if hashBytes(onDisk) == hashBytes(file.data) && uint32(existingModes[path].Perm()) == uint32(file.mode.Perm()) {
			result.Unchanged = append(result.Unchanged, path)
		} else {
			result.Updated = append(result.Updated, path)
			if adoption[path] {
				result.Adopted = append(result.Adopted, path)
				adoptedBackups = append(adoptedBackups, backupFile{path: path, data: onDisk, mode: existingModes[path]})
			}
		}
	}
	prune, staleRecords := staleOwnershipPaths(known, planned, existing, clusterName, scopes)
	if opts.pruneEnabled() {
		result.Renamed = detectSafeRenames(prune, result.Added, plannedBytes, existing, known)
		if len(result.Renamed) > 0 {
			sources, targets := make(map[string]bool), make(map[string]bool)
			for _, rename := range result.Renamed {
				parts := strings.SplitN(rename, " -> ", 2)
				if len(parts) == 2 {
					sources[parts[0]], targets[parts[1]] = true, true
				}
			}
			prune = filterTreePaths(prune, sources)
			result.Added = filterTreePaths(result.Added, targets)
		}
	}
	if opts.pruneEnabled() {
		result.Pruned = append(result.Pruned, prune...)
	} else {
		result.PruneCandidates = append(result.PruneCandidates, prune...)
		if len(prune) > 0 {
			result.Warnings = append(result.Warnings, "prune disabled; candidates were reported but retained")
		}
	}
	clusterManifest, globalManifest := state.cluster, state.global
	for path, file := range planned {
		record := GeneratedFileInfo{SHA256: hashBytes(file.data), Mode: uint32(file.mode.Perm())}
		if globalOwnershipFiles[path] {
			globalManifest.FileRecords[path] = record
		} else {
			clusterManifest.FileRecords[path] = record
		}
	}
	if opts.pruneEnabled() {
		for _, path := range staleRecords {
			if globalOwnershipFiles[path] {
				delete(globalManifest.FileRecords, path)
			} else {
				delete(clusterManifest.FileRecords, path)
			}
		}
	}
	clusterManifest.Version, clusterManifest.Scope, clusterManifest.Identity = ownershipManifestVersion, "cluster", clusterName
	plan := &generatedTreePlan{result: result, planned: planned, seedWrites: seedWrites, adoptedBackups: adoptedBackups, clusterManifest: clusterManifest, globalManifest: globalManifest, clusterManifestPath: state.clusterPath, preimages: make(map[string]generatedTreeSnapshot)}
	if state.globalActive {
		plan.globalManifestPath = state.globalPath
	}
	for path := range planned {
		plan.preimages[filepath.Join(root, filepath.FromSlash(path))] = snapshotPathOrMissing(filepath.Join(root, filepath.FromSlash(path)))
		if _, exists := existing[path]; exists {
			plan.preimages[filepath.Join(root, filepath.FromSlash(path))] = generatedTreeSnapshot{exists: true, data: existing[path], mode: existingModes[path]}
		}
	}
	if opts.pruneEnabled() {
		for _, path := range prune {
			plan.deletes = append(plan.deletes, path)
			plan.preimages[filepath.Join(root, filepath.FromSlash(path))] = generatedTreeSnapshot{exists: true, data: existing[path], mode: existingModes[path]}
		}
	}
	if opts.pruneEnabled() {
		for _, rename := range result.Renamed {
			if parts := strings.SplitN(rename, " -> ", 2); len(parts) == 2 {
				plan.deletes = append(plan.deletes, parts[0])
				path := filepath.Join(root, filepath.FromSlash(parts[0]))
				plan.preimages[path] = generatedTreeSnapshot{exists: true, data: existing[parts[0]], mode: existingModes[parts[0]]}
			}
		}
		sort.Strings(plan.deletes)
	}
	if opts.pruneEnabled() {
		plan.dirDeletes = scopedEmptyDirectoryDeletes(root, plan.deletes, scopes, clusterName)
	}
	for path := range seedWrites {
		plan.preimages[filepath.Join(root, filepath.FromSlash(path))] = snapshotPathOrMissing(filepath.Join(root, filepath.FromSlash(path)))
	}
	for _, manifestPath := range []string{plan.clusterManifestPath, plan.globalManifestPath} {
		if manifestPath == "" {
			continue
		}
		plan.preimages[manifestPath] = snapshotPathOrMissing(manifestPath)
	}
	for _, list := range [][]string{result.Added, result.Updated, result.Unchanged, result.Pruned, result.PruneCandidates, result.Seeded, result.Adopted} {
		sort.Strings(list)
	}
	sort.Strings(result.Warnings)
	return plan, nil
}

func staleOwnershipPaths(known map[string]GeneratedFileInfo, planned map[string]generatedTreeFile, existing map[string][]byte, clusterName string, scopes []string) (existingPaths, records []string) {
	for path := range known {
		if !ownershipPathInScopes(path, scopes) || !ownershipPathAllowed(path, clusterName) || isGeneratedTreeCustomPath(path) {
			continue
		}
		if _, keep := planned[path]; keep {
			continue
		}
		records = append(records, path)
		if _, exists := existing[path]; exists {
			existingPaths = append(existingPaths, path)
		}
	}
	sort.Strings(existingPaths)
	sort.Strings(records)
	return existingPaths, records
}

func mergedManifest(cluster, global GeneratedManifest) GeneratedManifest {
	merged := GeneratedManifest{Version: ownershipManifestVersion, Scope: "cluster", Identity: cluster.Identity, FileRecords: make(map[string]GeneratedFileInfo, len(cluster.FileRecords)+len(global.FileRecords))}
	for path, record := range cluster.FileRecords {
		merged.FileRecords[path] = record
	}
	for path, record := range global.FileRecords {
		merged.FileRecords[path] = record
	}
	return merged
}

func ownedSecretTreePath(root, clusterName, path string) bool {
	return loadOwnedSecretTreePaths(root, clusterName)[path]
}

func scanLiveRepositoryTree(root, clusterName string, scopes []string) (map[string][]byte, map[string]os.FileMode, []string, error) {
	files, modes := make(map[string][]byte), make(map[string]os.FileMode)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return files, modes, nil, nil
	}
	var warnings []string
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
		if entry.Type()&os.ModeSymlink != 0 {
			if ownershipScopesMayContain(rel, scopes) {
				return fmt.Errorf("refusing to use symlinked generated path %s", rel)
			}
			return nil
		}
		if entry.IsDir() {
			if rel == ownershipDir || strings.HasPrefix(rel, ownershipDir+"/") || skipGeneratedTreeDir(rel) || isGeneratedTreeCustomPath(rel) || !ownershipScopesMayContain(rel, scopes) {
				return fs.SkipDir
			}
			return nil
		}
		if !ownershipPathInScopes(rel, scopes) || !ownershipPathAllowed(rel, clusterName) || isGeneratedBackupPath(rel) {
			return nil
		}
		if filepath.Base(rel) == secretartifacts.OwnershipStateFilename || filepath.Base(rel) == secretsSyncLockFilename {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files[rel], modes[rel] = data, info.Mode().Perm()
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scan target repository: %w", err)
	}
	return files, modes, warnings, nil
}

type backupFile struct {
	path string
	data []byte
	mode os.FileMode
}

func plannedOverlayFiles(root string) (map[string][]byte, map[string][]byte, []string, error) {
	planned, seeds := make(map[string][]byte), make(map[string][]byte)
	var warnings []string
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
		if isCustomPath(rel) {
			seeds[rel] = data
		} else {
			planned[rel] = data
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scan workspace overlay: %w", err)
	}
	sort.Strings(warnings)
	return planned, seeds, warnings, nil
}

func validatePlannedTargets(root string, planned map[string][]byte) error {
	for path := range planned {
		full := filepath.Join(root, filepath.FromSlash(path))
		for current := filepath.Dir(full); current != filepath.Clean(root) && current != "."; current = filepath.Dir(current) {
			info, err := os.Lstat(current)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing to write through symlink in target path %s", path)
			}
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("inspect target path %s: %w", path, err)
			}
		}
		if info, err := os.Lstat(full); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink in target path %s", path)
		} else if err == nil && info.IsDir() {
			return fmt.Errorf("refusing to overwrite directory with generated file %s", path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
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
	for _, root := range generatorOwnedOverlayRoots {
		if parts[0] == root {
			for _, part := range parts[1:] {
				if part == CustomDirName {
					return true
				}
			}
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
func isTempPath(path string) bool            { return path == ".tmp" || strings.HasPrefix(path, ".tmp/") }
func hashBytes(data []byte) string           { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func isGeneratedBackupPath(path string) bool { return strings.Contains(filepath.Base(path), ".bak-") }

func snapshotPathOrMissing(path string) generatedTreeSnapshot {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return generatedTreeSnapshot{}
	}
	if err != nil {
		return generatedTreeSnapshot{exists: true}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return generatedTreeSnapshot{exists: true, mode: info.Mode()}
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return generatedTreeSnapshot{exists: true, mode: info.Mode()}
	}
	return generatedTreeSnapshot{exists: true, data: data, mode: info.Mode().Perm()}
}

func atomicWriteVerified(root, dst string, data []byte, mode os.FileMode) error {
	if err := validatePlannedTargets(root, map[string][]byte{mustRelative(root, dst): data}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
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
	if err := tmp.Chmod(mode.Perm()); err != nil {
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

func loadOwnedSecretTreePaths(targetRoot, clusterName string) map[string]bool {
	owned := make(map[string]bool)
	overlayDir := filepath.Join(targetRoot, "applications", "overlays", clusterName)
	state, _, err := secretartifacts.LoadOwnershipState(overlayDir)
	if err != nil {
		return owned
	}
	prefix := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName))
	for _, record := range state.Artifacts {
		full := filepath.Join(overlayDir, filepath.FromSlash(record.Path))
		if data, readErr := os.ReadFile(full); readErr == nil && secretartifacts.HashBytes(data) == record.Hash {
			owned[filepath.ToSlash(filepath.Join(prefix, record.Path))] = true
		}
	}
	return owned
}

func loadGeneratedManifest(root string) (GeneratedManifest, bool, error) {
	path := filepath.Join(root, GeneratedManifestFile)
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return GeneratedManifest{}, false, fmt.Errorf("refusing to read symlinked generated manifest %s", path)
	}
	return GeneratedManifest{}, false, fmt.Errorf("legacy generated manifest %s is unsupported", path)
}

func writeGeneratedManifest(root string, _ GeneratedManifest) error {
	path := filepath.Join(root, GeneratedManifestFile)
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlinked generated manifest %s", path)
	}
	return fmt.Errorf("legacy generated manifest %s is unsupported", path)
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
		if err := os.WriteFile(path, file.data, file.mode.Perm()); err != nil {
			return nil, fmt.Errorf("write backup for %s: %w", file.path, err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func detectSafeRenames(pruned, added []string, planned map[string][]byte, existing map[string][]byte, manifest map[string]GeneratedFileInfo) []string {
	byHash := make(map[string][]string)
	for _, path := range added {
		byHash[hashBytes(planned[path])] = append(byHash[hashBytes(planned[path])], path)
	}
	staleByHash := make(map[string][]string)
	for _, path := range pruned {
		data, ok := existing[path]
		if !ok || manifest[path].SHA256 != hashBytes(data) {
			continue
		}
		staleByHash[hashBytes(data)] = append(staleByHash[hashBytes(data)], path)
	}
	var renamed []string
	for hash, stale := range staleByHash {
		if candidates := byHash[hash]; len(stale) == 1 && len(candidates) == 1 {
			renamed = append(renamed, stale[0]+" -> "+candidates[0])
		}
	}
	sort.Strings(renamed)
	return renamed
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

// scopedEmptyDirectoryDeletes derives only the ancestors of files that are
// already scheduled for deletion. It never walks the repository, and it will
// not remove a directory containing an unplanned file, symlink, or custom
// subtree. This is intentionally narrower than generic empty-directory
// cleanup: disabling one service may remove that service's now-empty scope,
// but cannot sweep unrelated user directories.
func scopedEmptyDirectoryDeletes(root string, deletes, scopes []string, clusterName string) []string {
	candidates := make(map[string]bool)
	for _, rel := range deletes {
		current := filepath.Dir(filepath.Join(root, filepath.FromSlash(rel)))
		for current != filepath.Clean(root) && current != "." {
			relDir, err := safeRelativePath(root, current)
			if err != nil || !ownershipScopesMayContain(relDir, scopes) {
				break
			}
			overlayPrefix := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName)) + "/"
			generatedRel := strings.TrimPrefix(relDir, overlayPrefix)
			if isGeneratorOwnedPath(generatedRel) && generatedRel != "services" && generatedRel != "managed-services" && generatedRel != "customer-managed" && !isGeneratedTreeCustomPath(relDir) {
				candidates[current] = true
			}
			current = filepath.Dir(current)
		}
	}
	deleteSet := make(map[string]bool, len(deletes))
	for _, rel := range deletes {
		deleteSet[filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))] = true
	}
	dirs := make([]string, 0, len(candidates))
	for dir := range candidates {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	var result []string
	removableDirs := make(map[string]bool)
	for _, dir := range dirs {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		removable := true
		for _, entry := range entries {
			entryPath := filepath.Join(dir, entry.Name())
			if entry.Type()&os.ModeSymlink != 0 {
				removable = false
				break
			}
			if entry.IsDir() {
				if !removableDirs[entryPath] || isGeneratedTreeCustomPath(mustRelative(root, entryPath)) {
					removable = false
					break
				}
				continue
			}
			if !deleteSet[filepath.Clean(entryPath)] {
				removable = false
				break
			}
		}
		if removable {
			result = append(result, dir)
			removableDirs[dir] = true
		}
	}
	return result
}
