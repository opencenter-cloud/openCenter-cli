package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClusterMigrateLayoutDryRunPrintsMoveDiffWithoutChanges(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	legacyOrgDir := createLegacyLayoutForMigrationTest(t, dir, "acme", "prod")

	var out bytes.Buffer
	err := runClusterMigrateLayout(t.Context(), &out, migrateLayoutOptions{
		organization: "acme",
		dryRun:       true,
	})
	if err != nil {
		t.Fatalf("migrate layout dry-run failed: %v", err)
	}

	output := out.String()
	for _, want := range []string{
		"Dry run: no files will be changed",
		"MOVE " + filepath.Join(legacyOrgDir, ".prod-config.yaml") + " -> " + filepath.Join(dir, "clusters", "state", "acme", "prod", "prod-config.yaml"),
		"MOVE " + filepath.Join(legacyOrgDir, "secrets", "age", "keys", "prod-key.txt") + " -> " + filepath.Join(dir, "clusters", "secrets", "acme", "prod", "age", "keys", "prod-key.txt"),
		"MOVE " + filepath.Join(legacyOrgDir, "infrastructure") + " -> " + filepath.Join(dir, "clusters", "gitops", "acme", "infrastructure"),
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}

	if _, err := os.Stat(filepath.Join(legacyOrgDir, ".prod-config.yaml")); err != nil {
		t.Fatalf("dry-run moved legacy config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "clusters", "state", "acme", "prod", "prod-config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created destination config, stat error = %v", err)
	}
}

func TestClusterMigrateLayoutMovesZonesAndRewritesConfig(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	legacyOrgDir := createLegacyLayoutForMigrationTest(t, dir, "acme", "prod")

	var out bytes.Buffer
	err := runClusterMigrateLayout(t.Context(), &out, migrateLayoutOptions{
		organization: "acme",
	})
	if err != nil {
		t.Fatalf("migrate layout failed: %v\n%s", err, out.String())
	}

	stateConfig := filepath.Join(dir, "clusters", "state", "acme", "prod", "prod-config.yaml")
	data, err := os.ReadFile(stateConfig)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}

	migratedConfig := string(data)
	for _, want := range []string{
		"local_dir: " + filepath.Join(dir, "clusters", "gitops", "acme"),
		"sops_age_key_file: " + filepath.Join(dir, "clusters", "secrets", "acme", "prod", "age", "keys", "prod-key.txt"),
		"private: " + filepath.Join(dir, "clusters", "secrets", "acme", "prod", "ssh", "prod"),
		"public_key: " + filepath.Join(dir, "clusters", "secrets", "acme", "prod", "ssh", "prod") + ".pub",
	} {
		if !strings.Contains(migratedConfig, want) {
			t.Fatalf("migrated config missing %q:\n%s", want, migratedConfig)
		}
	}

	for _, want := range []string{
		filepath.Join(dir, "clusters", "gitops", "acme", ".git"),
		filepath.Join(dir, "clusters", "gitops", "acme", "infrastructure", "clusters", "prod", "kustomization.yaml"),
		filepath.Join(dir, "clusters", "secrets", "acme", "prod", "age", "keys", "prod-key.txt"),
		filepath.Join(dir, "clusters", "secrets", "acme", "prod", "ssh", "prod"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Fatalf("migrated path %s missing: %v", want, err)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "clusters", "gitops", "acme", "infrastructure", "clusters", "prod", "kustomization.yaml")); err != nil {
		t.Fatalf("stat migrated GitOps file: %v", err)
	} else if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("migrated GitOps file mode = %#o, want 0644", got)
	}

	if _, err := os.Stat(filepath.Join(legacyOrgDir, ".prod-config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("legacy config still exists, stat error = %v", err)
	}
}

func createLegacyLayoutForMigrationTest(t *testing.T, configDir, organization, clusterName string) string {
	t.Helper()

	legacyOrgDir := filepath.Join(configDir, "clusters", organization)
	mustMkdirAll(t, filepath.Join(legacyOrgDir, ".git"), 0o755)
	mustMkdirAll(t, filepath.Join(legacyOrgDir, "infrastructure", "clusters", clusterName), 0o755)
	mustMkdirAll(t, filepath.Join(legacyOrgDir, "applications", "overlays", clusterName), 0o755)
	mustMkdirAll(t, filepath.Join(legacyOrgDir, "secrets", "age", "keys"), 0o700)
	mustMkdirAll(t, filepath.Join(legacyOrgDir, "secrets", "ssh"), 0o700)

	mustWriteFile(t, filepath.Join(legacyOrgDir, "infrastructure", "clusters", clusterName, "kustomization.yaml"), []byte("resources: []\n"), 0o644)
	mustWriteFile(t, filepath.Join(legacyOrgDir, "secrets", "age", "keys", clusterName+"-key.txt"), []byte("AGE-SECRET-KEY-1TEST\n"), 0o600)
	mustWriteFile(t, filepath.Join(legacyOrgDir, "secrets", "age", "keys", clusterName+"-key.txt.pub"), []byte("age1test\n"), 0o644)
	mustWriteFile(t, filepath.Join(legacyOrgDir, "secrets", "ssh", clusterName), []byte("private\n"), 0o600)
	mustWriteFile(t, filepath.Join(legacyOrgDir, "secrets", "ssh", clusterName+".pub"), []byte("public\n"), 0o644)

	legacyConfig := []byte(`schema_version: "2.0"
opencenter:
  meta:
    name: ` + clusterName + `
    organization: ` + organization + `
  gitops:
    repository:
      local_dir: ` + legacyOrgDir + `
    auth:
      ssh:
        private_key: ` + filepath.Join(legacyOrgDir, "secrets", "ssh", clusterName) + `
        public_key: ` + filepath.Join(legacyOrgDir, "secrets", "ssh", clusterName) + `.pub
  infrastructure:
    ssh:
      key_path: ` + filepath.Join(legacyOrgDir, "secrets", "ssh", clusterName) + `
secrets:
  sops_age_key_file: ` + filepath.Join(legacyOrgDir, "secrets", "age", "keys", clusterName+"-key.txt") + `
  sops:
    age_key_file: ` + filepath.Join(legacyOrgDir, "secrets", "age", "keys", clusterName+"-key.txt") + `
  ssh_key:
    private: ` + filepath.Join(legacyOrgDir, "secrets", "ssh", clusterName) + `
    public: ` + filepath.Join(legacyOrgDir, "secrets", "ssh", clusterName) + `.pub
`)
	mustWriteFile(t, filepath.Join(legacyOrgDir, "."+clusterName+"-config.yaml"), legacyConfig, 0o600)
	return legacyOrgDir
}

func mustMkdirAll(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, mode); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
