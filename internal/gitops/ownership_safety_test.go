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

// TestPromoteExemptsDeployManagedInventory reproduces OCTR-786: kubespray
// populates infrastructure/clusters/<cluster>/inventory/ at deploy time, so
// those files are never in the generator ownership ledger. A subsequent
// generate must not reject them as "user-authored files found in
// generator-owned paths". Files elsewhere in the generator-owned infrastructure
// scope must still be protected.
func TestPromoteExemptsDeployManagedInventory(t *testing.T) {
	repo := t.TempDir()
	stage := t.TempDir()
	cluster := "inv-cluster"
	infraRel := filepath.Join("infrastructure", "clusters", cluster)

	// Stage only what the generator actually produces for the inventory subtree:
	// a .gitkeep. Promote it to seed the ownership ledger.
	writeTestFile(t, filepath.Join(stage, infraRel, "inventory", ".gitkeep"), "")
	if _, err := promoteGeneratedTree(stage, repo, cluster, PromoteOptions{}); err != nil {
		t.Fatalf("initial promotion failed: %v", err)
	}

	// Simulate kubespray populating the inventory subtree at deploy time. These
	// files are not in the ledger and not in the staged/planned set.
	writeTestFile(t, filepath.Join(repo, infraRel, "inventory", "inventory.yaml"), "# populated by kubespray\n")
	writeTestFile(t, filepath.Join(repo, infraRel, "inventory", "os_hardening_playbook.yml"), "# hardening\n")
	writeTestFile(t, filepath.Join(repo, infraRel, "inventory", "group_vars", "k8s_cluster", "k8s-cluster.yml"), "kube_version: v1.35\n")

	// Re-promote: must succeed (inventory subtree is deploy-managed, exempt).
	if _, err := promoteGeneratedTree(stage, repo, cluster, PromoteOptions{Force: true}); err != nil {
		t.Fatalf("promotion rejected deploy-managed inventory files (OCTR-786): %v", err)
	}
	// The inventory files must be left intact (not clobbered/pruned).
	if got := readTestFile(t, filepath.Join(repo, infraRel, "inventory", "inventory.yaml")); got != "# populated by kubespray\n" {
		t.Fatalf("deploy-managed inventory file was modified: %q", got)
	}

	// terraform init writes .terraform.lock.hcl at the cluster root at deploy
	// time; it is not in the ledger. Re-promote must succeed and leave it intact
	// (OCTR-786, tenant-reported sibling of the inventory case).
	lockContents := "# This file is maintained automatically by \"terraform init\".\n"
	writeTestFile(t, filepath.Join(repo, infraRel, ".terraform.lock.hcl"), lockContents)
	if _, err := promoteGeneratedTree(stage, repo, cluster, PromoteOptions{Force: true}); err != nil {
		t.Fatalf("promotion rejected deploy-managed .terraform.lock.hcl (OCTR-786): %v", err)
	}
	if got := readTestFile(t, filepath.Join(repo, infraRel, ".terraform.lock.hcl")); got != lockContents {
		t.Fatalf("deploy-managed .terraform.lock.hcl was modified: %q", got)
	}

	// terraform init also writes the .terraform module cache directory and the
	// kubespray keypair provisioner drops id_rsa / id_rsa.pub into the cluster
	// dir at deploy time; all of these are deploy-managed, not generated.
	writeTestFile(t, filepath.Join(repo, infraRel, ".terraform", "modules", "mod.json"), "{ \"Modules\": [] }\n")
	writeTestFile(t, filepath.Join(repo, infraRel, "id_rsa"), "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----\n")
	writeTestFile(t, filepath.Join(repo, infraRel, "id_rsa.pub"), "ssh-rsa AAAA... example\n")
	if _, err := promoteGeneratedTree(stage, repo, cluster, PromoteOptions{Force: true}); err != nil {
		t.Fatalf("promotion rejected deploy-managed .terraform/ or SSH key files (OCTR-786): %v", err)
	}
	if got := readTestFile(t, filepath.Join(repo, infraRel, "id_rsa")); got != "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----\n" {
		t.Fatalf("deploy-managed id_rsa was modified: %q", got)
	}

	// Negative case: a user file in the infrastructure scope but OUTSIDE the
	// exempted deploy-managed paths must still be refused, proving the exemption
	// is scoped (not a blanket cluster-dir exemption).
	writeTestFile(t, filepath.Join(repo, infraRel, "rogue-user-file.yaml"), "rogue\n")
	if _, err := promoteGeneratedTree(stage, repo, cluster, PromoteOptions{Force: true}); err == nil ||
		!strings.Contains(err.Error(), "user-authored") {
		t.Fatalf("expected refusal for non-inventory user file, got %v", err)
	}
}
