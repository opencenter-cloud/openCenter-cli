package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LegacyLayoutError indicates that a command encountered the old unsafe
// org-root Git repository layout. Only the explicit migration command should
// read from this layout.
type LegacyLayoutError struct {
	Path string
}

func (e *LegacyLayoutError) Error() string {
	return fmt.Sprintf("legacy mixed GitOps/state/secrets layout detected at %s; run 'opencenter cluster migrate-layout' before using this cluster", e.Path)
}

// DefaultPathRoots derives secure zone roots from the historical clusters root.
func DefaultPathRoots(baseDir string) PathRoots {
	baseDir = expandPath(baseDir)
	return PathRoots{
		ClustersDir:     baseDir,
		GitOpsDir:       filepath.Join(baseDir, "gitops"),
		ClusterStateDir: filepath.Join(baseDir, "state"),
		SecretsDir:      filepath.Join(baseDir, "secrets"),
	}
}

func expandPathRoots(roots PathRoots) PathRoots {
	if roots.ClustersDir != "" {
		roots.ClustersDir = expandPath(roots.ClustersDir)
	}
	if roots.GitOpsDir != "" {
		roots.GitOpsDir = expandPath(roots.GitOpsDir)
	}
	if roots.ClusterStateDir != "" {
		roots.ClusterStateDir = expandPath(roots.ClusterStateDir)
	}
	if roots.SecretsDir != "" {
		roots.SecretsDir = expandPath(roots.SecretsDir)
	}
	if roots.ClustersDir == "" {
		roots.ClustersDir = "."
	}
	defaults := DefaultPathRoots(roots.ClustersDir)
	if roots.GitOpsDir == "" {
		roots.GitOpsDir = defaults.GitOpsDir
	}
	if roots.ClusterStateDir == "" {
		roots.ClusterStateDir = defaults.ClusterStateDir
	}
	if roots.SecretsDir == "" {
		roots.SecretsDir = defaults.SecretsDir
	}
	return roots
}

// Validate enforces that local state and secrets cannot resolve into the
// GitOps worktree, including through pre-existing symlinked parents.
func (p *ClusterPaths) Validate() error {
	gitopsDir, err := secureAbs(p.GitOpsDir)
	if err != nil {
		return fmt.Errorf("resolving gitops dir: %w", err)
	}

	checks := map[string]string{
		"cluster state dir": p.ClusterStateDir,
		"secrets dir":       p.SecretsDir,
		"config path":       p.ConfigPath,
		"SOPS key path":     p.SOPSKeyPath,
		"SSH key path":      p.SSHKeyPath,
	}
	for label, candidate := range checks {
		resolved, err := secureAbs(candidate)
		if err != nil {
			return fmt.Errorf("resolving %s %q: %w", label, candidate, err)
		}
		if sameOrSubpath(gitopsDir, resolved) {
			return fmt.Errorf("%s %q must not be equal to or inside gitops dir %q", label, candidate, p.GitOpsDir)
		}
	}

	return nil
}

func secureAbs(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	expanded := expandPath(path)
	abs, err := filepath.Abs(filepath.Clean(expanded))
	if err != nil {
		return "", err
	}

	resolved, err := resolveExistingPrefix(abs)
	if err != nil {
		return "", err
	}
	return normalizeForComparison(resolved), nil
}

func resolveExistingPrefix(path string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved), nil
	}

	var missing []string
	current := filepath.Clean(path)
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}

	return filepath.Clean(path), nil
}

func sameOrSubpath(root, candidate string) bool {
	root = normalizeForComparison(filepath.Clean(root))
	candidate = normalizeForComparison(filepath.Clean(candidate))
	if root == candidate {
		return true
	}

	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

func normalizeForComparison(path string) string {
	clean := filepath.Clean(path)
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}
