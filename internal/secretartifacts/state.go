package secretartifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	OwnershipStateFilename = ".opencenter-secret-artifacts.json"
	OwnershipStateVersion  = 2
)

// OwnershipArtifact records an artifact path, its logical owners, and the
// hash of the materialized file. It is deliberately kept in this leaf package
// so GitOps can consume ownership without importing the secrets manager.
type OwnershipArtifact struct {
	Path   string   `json:"path"`
	Owners []string `json:"owners,omitempty"`
	Hash   string   `json:"hash"`
}

type OwnershipState struct {
	Version   int                 `json:"version"`
	Artifacts []OwnershipArtifact `json:"artifacts"`
	// Paths is retained only to read pre-phase-4 state files. New files never
	// write this field.
	Paths []string `json:"paths,omitempty"`
}

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SafeArtifactPath reports whether a relative path may hold a sync-owned
// secret. Three shapes are allowed: the generic per-service manifest
// services/<service>/secret.yaml, a cert-manager DNS-credential Secret
// services/cert-manager/<credential-file>.yaml, and a renderer-emitted Secret
// carrying a dedicated filename (see DedicatedSecretFilename).
//
// Every shape is constrained to a filename the planner itself constructs, so a
// corrupt or hostile ownership state still cannot name an arbitrary file for
// sync to write, hash-check, or delete.
func SafeArtifactPath(relative string) bool {
	if relative == "" || filepath.IsAbs(filepath.FromSlash(relative)) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	if clean != relative || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	parts := strings.Split(clean, "/")
	if len(parts) != 3 || parts[1] == "" || parts[1] == "." || parts[1] == ".." {
		return false
	}
	switch parts[0] {
	case "services", "managed-services":
	default:
		return false
	}
	if parts[2] == secretFilename {
		return true
	}
	if parts[0] == "services" && parts[1] == "cert-manager" && IsCertManagerCredentialFilename(parts[2]) {
		return true
	}
	_, _, dedicated := DedicatedSecretFilename(clean)
	return dedicated
}

func LoadOwnershipState(root string) (OwnershipState, bool, error) {
	path := filepath.Join(root, OwnershipStateFilename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return OwnershipState{Version: OwnershipStateVersion, Artifacts: []OwnershipArtifact{}}, false, nil
	}
	if err != nil {
		return OwnershipState{}, false, fmt.Errorf("read secret artifact ownership state: %w", err)
	}
	var state OwnershipState
	if err := json.Unmarshal(data, &state); err != nil {
		return OwnershipState{}, false, fmt.Errorf("read secret artifact ownership state: corrupt JSON: %w", err)
	}
	if state.Version == 0 && len(state.Paths) > 0 {
		for _, path := range state.Paths {
			if !SafeArtifactPath(path) {
				return OwnershipState{}, false, fmt.Errorf("secret artifact ownership state contains invalid path %q", path)
			}
			state.Artifacts = append(state.Artifacts, OwnershipArtifact{Path: path})
		}
		state.Version = OwnershipStateVersion
	}
	if state.Version != OwnershipStateVersion {
		return OwnershipState{}, false, fmt.Errorf("secret artifact ownership state version %d is unsupported", state.Version)
	}
	if len(state.Paths) > 0 {
		return OwnershipState{}, false, fmt.Errorf("secret artifact ownership state contains legacy paths with version %d", state.Version)
	}
	seen := make(map[string]bool, len(state.Artifacts))
	for i := range state.Artifacts {
		artifact := &state.Artifacts[i]
		if !SafeArtifactPath(artifact.Path) || artifact.Hash == "" {
			return OwnershipState{}, false, fmt.Errorf("secret artifact ownership state contains invalid record for %q", artifact.Path)
		}
		if seen[artifact.Path] {
			return OwnershipState{}, false, fmt.Errorf("secret artifact ownership state contains duplicate path %q", artifact.Path)
		}
		seen[artifact.Path] = true
		artifact.Path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(artifact.Path)))
		sort.Strings(artifact.Owners)
	}
	sort.Slice(state.Artifacts, func(i, j int) bool { return state.Artifacts[i].Path < state.Artifacts[j].Path })
	return state, true, nil
}

func (s OwnershipState) ByPath() map[string]OwnershipArtifact {
	result := make(map[string]OwnershipArtifact, len(s.Artifacts))
	for _, artifact := range s.Artifacts {
		result[artifact.Path] = artifact
	}
	return result
}

func WriteOwnershipStateAtomic(root string, state OwnershipState) error {
	state.Version = OwnershipStateVersion
	state.Paths = nil
	sort.Slice(state.Artifacts, func(i, j int) bool { return state.Artifacts[i].Path < state.Artifacts[j].Path })
	for i := range state.Artifacts {
		if !SafeArtifactPath(state.Artifacts[i].Path) || state.Artifacts[i].Hash == "" {
			return fmt.Errorf("invalid secret artifact ownership record %q", state.Artifacts[i].Path)
		}
		sort.Strings(state.Artifacts[i].Owners)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal secret artifact ownership state: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create ownership state parent: %w", err)
	}
	if err := rejectSymlinkPath(root, root, false); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(root, OwnershipStateFilename+".tmp-")
	if err != nil {
		return fmt.Errorf("create ownership state temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write ownership state temporary file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod ownership state temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close ownership state temporary file: %w", err)
	}
	dst := filepath.Join(root, OwnershipStateFilename)
	if err := rejectSymlinkPath(root, dst, true); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("replace ownership state: %w", err)
	}
	return nil
}

func rejectSymlinkPath(root, target string, final bool) error {
	root = filepath.Clean(root)
	if rel, err := filepath.Rel(root, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path %q escapes overlay root", target)
	}
	if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlinked overlay root %s", root)
	}
	for current := filepath.Dir(target); current != root; current = filepath.Dir(current) {
		if info, err := os.Lstat(current); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in ownership state path %s", current)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect ownership state path %s: %w", current, err)
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if final {
		if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink ownership state target %s", target)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect ownership state target: %w", err)
		}
	}
	return nil
}
