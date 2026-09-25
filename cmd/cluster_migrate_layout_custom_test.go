package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"gopkg.in/yaml.v3"
)

func TestClusterMigrateCustomDryRunDoesNotChangeDisk(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	overlay := customMigrationTestOverlay(t, dir, "acme", "prod")
	source := filepath.Join(overlay, "services", "metallb", "ipaddresspool.yaml")
	mustWriteFile(t, source, []byte("apiVersion: metallb.io/v1beta1\nkind: IPAddressPool\n"), 0o644)

	var out bytes.Buffer
	err := runClusterMigrateCustom(t.Context(), &out, migrateLayoutOptions{organization: "acme", cluster: "prod"})
	if err != nil {
		t.Fatalf("custom migration dry-run failed: %v", err)
	}
	if !strings.Contains(out.String(), "Dry run: no files will be changed") || !strings.Contains(out.String(), "ipaddresspool.yaml") {
		t.Fatalf("dry-run report missing candidate: %s", out.String())
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("dry-run changed source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(overlay, "services", "metallb", "custom")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created custom directory, stat error = %v", err)
	}
}

func TestClusterMigrateCustomApplyMovesAndListsResource(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	overlay := customMigrationTestOverlay(t, dir, "acme", "prod")
	source := filepath.Join(overlay, "services", "metallb", "pool.yaml")
	mustWriteFile(t, source, []byte("kind: IPAddressPool\n"), 0o644)

	var out bytes.Buffer
	if err := runClusterMigrateCustom(t.Context(), &out, migrateLayoutOptions{organization: "acme", cluster: "prod", apply: true}); err != nil {
		t.Fatalf("custom migration apply failed: %v\n%s", err, out.String())
	}
	destination := filepath.Join(overlay, "services", "metallb", "custom", "pool.yaml")
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists, stat error = %v", err)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("destination missing: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(destination), "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read custom kustomization: %v", err)
	}
	var kustomization struct {
		Resources []string `yaml:"resources"`
	}
	if err := yaml.Unmarshal(data, &kustomization); err != nil {
		t.Fatalf("parse custom kustomization: %v", err)
	}
	if len(kustomization.Resources) != 1 || kustomization.Resources[0] != "pool.yaml" {
		t.Fatalf("resources = %#v, want [pool.yaml]", kustomization.Resources)
	}
}

func TestClusterMigrateCustomOwnershipAndCollisionStates(t *testing.T) {
	t.Run("already custom is skipped", func(t *testing.T) {
		dir := t.TempDir()
		prepareCommandTestEnv(t, dir)
		overlay := customMigrationTestOverlay(t, dir, "acme", "prod")
		path := filepath.Join(overlay, "services", "metallb", "custom", "pool.yaml")
		mustMkdirAll(t, filepath.Dir(path), 0o755)
		mustWriteFile(t, path, []byte("kind: IPAddressPool\n"), 0o644)
		var out bytes.Buffer
		if err := runClusterMigrateCustom(t.Context(), &out, migrateLayoutOptions{organization: "acme", cluster: "prod", apply: true}); err != nil {
			t.Fatalf("migration failed: %v", err)
		}
		if !strings.Contains(out.String(), "SKIPPED-ALREADY-CUSTOM: 1") {
			t.Fatalf("custom skip missing: %s", out.String())
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("custom file changed: %v", err)
		}
	})

	t.Run("tracked generated and modified files", func(t *testing.T) {
		dir := t.TempDir()
		prepareCommandTestEnv(t, dir)
		overlay := customMigrationTestOverlay(t, dir, "acme", "prod")
		generated := filepath.Join(overlay, "services", "metallb", "generated.yaml")
		modified := filepath.Join(overlay, "services", "metallb", "modified.yaml")
		generatedData := []byte("kind: Generated\n")
		modifiedData := []byte("kind: Modified\n")
		mustWriteFile(t, generated, generatedData, 0o644)
		mustWriteFile(t, modified, modifiedData, 0o644)
		manifest := struct {
			Version int               `json:"version"`
			Cluster string            `json:"cluster"`
			Files   map[string]string `json:"files"`
		}{
			Version: 1,
			Cluster: "prod",
			Files: map[string]string{
				"services/metallb/generated.yaml": migrationHash(generatedData),
				"services/metallb/modified.yaml":  migrationHash([]byte("kind: Original\n")),
			},
		}
		manifestData, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		mustWriteFile(t, filepath.Join(overlay, gitops.GeneratedManifestFile), manifestData, 0o644)
		var out bytes.Buffer
		if err := runClusterMigrateCustom(t.Context(), &out, migrateLayoutOptions{organization: "acme", cluster: "prod", apply: true}); err != nil {
			t.Fatalf("migration failed: %v", err)
		}
		if !strings.Contains(out.String(), "TRACKED: 1") || !strings.Contains(out.String(), "TRACKED-MODIFIED (needs manual attention): 1") {
			t.Fatalf("ownership report missing: %s", out.String())
		}
		for _, path := range []string{generated, modified} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("tracked file moved: %s: %v", path, err)
			}
		}
	})

	t.Run("existing destination is refused", func(t *testing.T) {
		dir := t.TempDir()
		prepareCommandTestEnv(t, dir)
		overlay := customMigrationTestOverlay(t, dir, "acme", "prod")
		source := filepath.Join(overlay, "services", "metallb", "pool.yaml")
		destination := filepath.Join(overlay, "services", "metallb", "custom", "pool.yaml")
		mustWriteFile(t, source, []byte("source\n"), 0o644)
		mustMkdirAll(t, filepath.Dir(destination), 0o755)
		mustWriteFile(t, destination, []byte("destination\n"), 0o644)
		var out bytes.Buffer
		if err := runClusterMigrateCustom(t.Context(), &out, migrateLayoutOptions{organization: "acme", cluster: "prod", apply: true}); err != nil {
			t.Fatalf("migration failed: %v", err)
		}
		if !strings.Contains(out.String(), "REFUSED-DESTINATION-EXISTS: 1") {
			t.Fatalf("collision report missing: %s", out.String())
		}
		if got, err := os.ReadFile(source); err != nil || string(got) != "source\n" {
			t.Fatalf("source changed: %q, %v", got, err)
		}
	})
}

func TestClusterMigrateCustomIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	overlay := customMigrationTestOverlay(t, dir, "acme", "prod")
	source := filepath.Join(overlay, "services", "metallb", "pool.yaml")
	mustWriteFile(t, source, []byte("kind: IPAddressPool\n"), 0o644)
	for i := 0; i < 2; i++ {
		var out bytes.Buffer
		if err := runClusterMigrateCustom(t.Context(), &out, migrateLayoutOptions{organization: "acme", cluster: "prod", apply: true}); err != nil {
			t.Fatalf("migration run %d failed: %v", i+1, err)
		}
		if i == 1 && strings.Contains(out.String(), "MOVE services/metallb/custom/pool.yaml") {
			t.Fatalf("second run attempted to move custom file: %s", out.String())
		}
	}
}

func TestClusterMigrateCustomRejectsTraversalAndSkipsSymlink(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	overlay := customMigrationTestOverlay(t, dir, "acme", "prod")
	target := filepath.Join(t.TempDir(), "outside.yaml")
	mustWriteFile(t, target, []byte("outside\n"), 0o644)
	if err := os.Symlink(target, filepath.Join(overlay, "services", "metallb", "linked.yaml")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	var out bytes.Buffer
	if err := runClusterMigrateCustom(t.Context(), &out, migrateLayoutOptions{organization: "acme", cluster: "prod", apply: true}); err != nil {
		t.Fatalf("symlink migration failed: %v", err)
	}
	if !strings.Contains(out.String(), "SYMLINKS-SKIPPED: 1") {
		t.Fatalf("symlink report missing: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(overlay, "services", "metallb", "custom", "linked.yaml")); !os.IsNotExist(err) {
		t.Fatalf("symlink was followed or moved: %v", err)
	}
	var traversalOut bytes.Buffer
	if err := runClusterMigrateCustom(t.Context(), &traversalOut, migrateLayoutOptions{organization: "acme", cluster: "../outside", apply: true}); err == nil {
		t.Fatal("path traversal cluster was accepted")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
}

func customMigrationTestOverlay(t *testing.T, dir, organization, cluster string) string {
	t.Helper()
	overlay := filepath.Join(dir, "clusters", "gitops", organization, "applications", "overlays", cluster)
	if err := os.MkdirAll(filepath.Join(overlay, "services", "metallb"), 0o755); err != nil {
		t.Fatalf("create overlay: %v", err)
	}
	return overlay
}
