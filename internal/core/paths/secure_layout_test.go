package paths

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathResolverSecureLayoutCreatesScopedZones(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	gitopsRoot := filepath.Join(root, "gitops-root")
	stateRoot := filepath.Join(root, "state-root")
	secretsRoot := filepath.Join(root, "secrets-root")

	resolver := NewPathResolverWithRoots(clustersRoot, gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())
	if err := resolver.CreateClusterDirectories(context.Background(), "demo-cluster", "acme"); err != nil {
		t.Fatalf("CreateClusterDirectories() error = %v", err)
	}

	clusterPaths, err := resolver.Resolve(context.Background(), "demo-cluster", "acme")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	assertPath := func(label, got, want string) {
		t.Helper()
		if got != want {
			t.Fatalf("%s = %q, want %q", label, got, want)
		}
	}

	assertPath("OrganizationDir", clusterPaths.OrganizationDir, filepath.Join(gitopsRoot, "acme"))
	assertPath("GitOpsDir", clusterPaths.GitOpsDir, filepath.Join(gitopsRoot, "acme"))
	assertPath("ClusterStateDir", clusterPaths.ClusterStateDir, filepath.Join(stateRoot, "acme", "demo-cluster"))
	assertPath("SecretsDir", clusterPaths.SecretsDir, filepath.Join(secretsRoot, "acme", "demo-cluster"))
	assertPath("ConfigPath", clusterPaths.ConfigPath, filepath.Join(stateRoot, "acme", "demo-cluster", "demo-cluster-config.yaml"))
	assertPath("KubeconfigPath", clusterPaths.KubeconfigPath, filepath.Join(stateRoot, "acme", "demo-cluster", "kubeconfig.yaml"))
	assertPath("InventoryPath", clusterPaths.InventoryPath, filepath.Join(stateRoot, "acme", "demo-cluster", "inventory"))
	assertPath("VenvPath", clusterPaths.VenvPath, filepath.Join(stateRoot, "acme", "demo-cluster", "venv"))
	assertPath("BinPath", clusterPaths.BinPath, filepath.Join(stateRoot, "acme", "demo-cluster", ".bin"))
	assertPath("ClusterDir", clusterPaths.ClusterDir, filepath.Join(gitopsRoot, "acme", "infrastructure", "clusters", "demo-cluster"))
	assertPath("ApplicationsDir", clusterPaths.ApplicationsDir, filepath.Join(gitopsRoot, "acme", "applications", "overlays", "demo-cluster"))
	assertPath("SOPSConfigPath", clusterPaths.SOPSConfigPath, filepath.Join(gitopsRoot, "acme", ".sops.yaml"))
	assertPath("SOPSKeyPath", clusterPaths.SOPSKeyPath, filepath.Join(secretsRoot, "acme", "demo-cluster", "age", "keys", "demo-cluster-key.txt"))
	assertPath("SSHKeyPath", clusterPaths.SSHKeyPath, filepath.Join(secretsRoot, "acme", "demo-cluster", "ssh", "demo-cluster"))

	if err := clusterPaths.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestClusterPathsValidateRejectsGitOpsContainment(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	gitopsDir := filepath.Join(root, "gitops", "acme")
	if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	valid := ClusterPaths{
		GitOpsDir:       gitopsDir,
		ClusterStateDir: filepath.Join(root, "state", "acme", "demo"),
		SecretsDir:      filepath.Join(root, "secrets", "acme", "demo"),
		ConfigPath:      filepath.Join(root, "state", "acme", "demo", "demo-config.yaml"),
		SOPSKeyPath:     filepath.Join(root, "secrets", "acme", "demo", "age", "keys", "demo-key.txt"),
		SSHKeyPath:      filepath.Join(root, "secrets", "acme", "demo", "ssh", "demo"),
	}

	tests := []struct {
		name   string
		mutate func(*ClusterPaths)
	}{
		{
			name: "state dir equals gitops dir",
			mutate: func(p *ClusterPaths) {
				p.ClusterStateDir = gitopsDir
			},
		},
		{
			name: "config path under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.ConfigPath = filepath.Join(gitopsDir, "demo-config.yaml")
			},
		},
		{
			name: "secrets dir under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.SecretsDir = filepath.Join(gitopsDir, "secrets")
			},
		},
		{
			name: "sops key under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.SOPSKeyPath = filepath.Join(gitopsDir, "keys.txt")
			},
		},
		{
			name: "ssh key under gitops dir",
			mutate: func(p *ClusterPaths) {
				p.SSHKeyPath = filepath.Join(gitopsDir, "id_ed25519")
			},
		},
		{
			name: "common prefix sibling stays valid",
			mutate: func(p *ClusterPaths) {
				p.ClusterStateDir = gitopsDir + "-state"
				p.ConfigPath = filepath.Join(gitopsDir+"-state", "demo-config.yaml")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid
			tt.mutate(&p)
			err := p.Validate()
			if tt.name == "common prefix sibling stays valid" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() error = nil, want containment error")
			}
		})
	}
}

func TestClusterPathsValidateRejectsSymlinkedParentsIntoGitOps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on many Windows test hosts")
	}

	root := t.TempDir()
	gitopsDir := filepath.Join(root, "gitops", "acme")
	if err := os.MkdirAll(gitopsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(root, "state-link")
	if err := os.Symlink(gitopsDir, linkRoot); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	p := ClusterPaths{
		GitOpsDir:       gitopsDir,
		ClusterStateDir: filepath.Join(linkRoot, "demo"),
		SecretsDir:      filepath.Join(root, "secrets", "acme", "demo"),
		ConfigPath:      filepath.Join(linkRoot, "demo", "demo-config.yaml"),
		SOPSKeyPath:     filepath.Join(root, "secrets", "acme", "demo", "age", "keys", "demo-key.txt"),
		SSHKeyPath:      filepath.Join(root, "secrets", "acme", "demo", "ssh", "demo"),
	}

	err := p.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want symlink containment error")
	}
	if !strings.Contains(err.Error(), "gitops") {
		t.Fatalf("Validate() error = %v, want gitops containment context", err)
	}
}

func TestResolveWithFallbackUsesOnlyNewStateLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	gitopsRoot := filepath.Join(root, "gitops")
	stateRoot := filepath.Join(root, "state")
	secretsRoot := filepath.Join(root, "secrets")
	resolver := NewPathResolverWithRoots(clustersRoot, gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())

	newStateDir := filepath.Join(stateRoot, "org1", "demo")
	if err := os.MkdirAll(newStateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newStateDir, "demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyOrgDir := filepath.Join(clustersRoot, "legacy-org")
	if err := os.MkdirAll(filepath.Join(legacyOrgDir, "infrastructure", "clusters", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyOrgDir, ".demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	clusterPaths, err := resolver.ResolveWithFallback(context.Background(), "demo")
	if err != nil {
		t.Fatalf("ResolveWithFallback() error = %v", err)
	}
	if clusterPaths.ClusterStateDir != newStateDir {
		t.Fatalf("ClusterStateDir = %q, want %q", clusterPaths.ClusterStateDir, newStateDir)
	}

	ambiguousDir := filepath.Join(stateRoot, "org2", "demo")
	if err := os.MkdirAll(ambiguousDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ambiguousDir, "demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver.ClearCache()

	if _, err := resolver.ResolveWithFallback(context.Background(), "demo"); err == nil {
		t.Fatal("ResolveWithFallback() error = nil, want ambiguity error")
	}
}

func TestResolveWithFallbackRejectsLegacyLayoutEvenWhenSecureStateExists(t *testing.T) {
	root := t.TempDir()
	clustersRoot := filepath.Join(root, "clusters")
	gitopsRoot := filepath.Join(root, "gitops")
	stateRoot := filepath.Join(root, "state")
	secretsRoot := filepath.Join(root, "secrets")
	resolver := NewPathResolverWithRoots(clustersRoot, gitopsRoot, stateRoot, secretsRoot, DefaultResolutionOptions())

	stateDir := filepath.Join(stateRoot, "acme", "demo")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	legacyOrgDir := filepath.Join(clustersRoot, "acme")
	if err := os.MkdirAll(filepath.Join(legacyOrgDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyOrgDir, ".demo-config.yaml"), []byte("schema_version: \"2.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := resolver.ResolveWithFallback(context.Background(), "demo"); err == nil {
		t.Fatal("ResolveWithFallback() error = nil, want legacy layout error")
	} else if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("ResolveWithFallback() error = %T %v, want LegacyLayoutError", err, err)
	}

	if _, err := resolver.GetOrganization(context.Background(), "demo"); err == nil {
		t.Fatal("GetOrganization() error = nil, want legacy layout error")
	} else if _, ok := err.(*LegacyLayoutError); !ok {
		t.Fatalf("GetOrganization() error = %T %v, want LegacyLayoutError", err, err)
	}
}
