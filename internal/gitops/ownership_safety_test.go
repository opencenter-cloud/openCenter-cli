package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromoteOverlayRejectsSymlinkedTargetOverlay(t *testing.T) {
	repo := t.TempDir()
	workspace := t.TempDir()
	realTarget := filepath.Join(repo, "real-overlay")
	if err := os.MkdirAll(realTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repo, "applications", "overlays", "cluster")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realTarget, target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "generated.yaml"), "generated")
	if _, err := promoteOverlay(workspace, target, "cluster", PromoteOptions{}); err == nil || !strings.Contains(err.Error(), "symlinked target overlay") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
}

func TestPromoteOverlayRejectsSymlinkInActiveGeneratedScope(t *testing.T) {
	repo := t.TempDir()
	workspace := t.TempDir()
	target := filepath.Join(repo, "applications", "overlays", "cluster")
	if err := os.MkdirAll(filepath.Join(target, "services"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(repo, "outside")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(linked, filepath.Join(target, "services", "metallb")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeTestFile(t, filepath.Join(workspace, "services", "metallb", "generated.yaml"), "generated")
	if _, err := promoteOverlay(workspace, target, "cluster", PromoteOptions{}); err == nil || !strings.Contains(err.Error(), "symlinked generated path") {
		t.Fatalf("expected active-scope symlink refusal, got %v", err)
	}
}

func TestPromoteOverlayDoesNotPruneModifiedTrackedFile(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "applications", "overlays", "cluster")
	workspace := t.TempDir()
	path := filepath.Join(workspace, "services", "metallb", "generated.yaml")
	writeTestFile(t, path, "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(root, "services", "metallb", "generated.yaml")
	writeTestFile(t, tracked, "modified")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{Force: true}); err == nil || !strings.Contains(err.Error(), "ownership conflict") {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
}

func TestPromoteOverlayModeDriftIsOwnershipConflict(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "applications", "overlays", "cluster")
	workspace := t.TempDir()
	path := filepath.Join(workspace, "services", "one", "generated.yaml")
	writeTestFile(t, path, "generated")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "services", "one", "generated.yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err == nil || !strings.Contains(err.Error(), "modified tracked file") {
		t.Fatalf("expected mode-drift refusal, got %v", err)
	}
}

func TestPromoteOverlayRechecksPreimageBeforeOverwrite(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "applications", "overlays", "cluster")
	workspace := t.TempDir()
	path := "services/one/generated.yaml"
	writeTestFile(t, filepath.Join(workspace, path), "v1")
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(workspace, path), "v2")
	generatedTreeMutationHook = func(path string) error {
		if strings.HasSuffix(filepath.ToSlash(path), "/services/one/generated.yaml") {
			return os.WriteFile(path, []byte("raced"), 0o644)
		}
		return nil
	}
	defer func() { generatedTreeMutationHook = nil }()
	if _, err := promoteOverlay(workspace, root, "cluster", PromoteOptions{}); err == nil || !strings.Contains(err.Error(), "preimage changed") {
		t.Fatalf("expected preimage refusal, got %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, path)); err != nil || string(got) != "v1" {
		t.Fatalf("preimage race changed target: %q, %v", got, err)
	}
}

func TestLoadGeneratedManifestRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "manifest-target.json")
	writeTestFile(t, target, "{}")
	if err := os.Symlink(target, filepath.Join(root, GeneratedManifestFile)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := loadGeneratedManifest(root); err == nil || !strings.Contains(err.Error(), "symlinked generated manifest") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
}
