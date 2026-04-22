package importer

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/testenv"
)

func TestArtifactStoreSaveAndLoadLatest(t *testing.T) {
	dirs := testenv.SetIsolatedCLIDirs(t)

	store, err := NewArtifactStore()
	if err != nil {
		t.Fatalf("NewArtifactStore() error = %v", err)
	}

	repoPath := filepath.Join(dirs.Root, "repos", "customer")
	result := &ImportScanResult{
		RepoPath: repoPath,
		Clusters: []ClusterImportResult{
			{ClusterName: "k8s-dev"},
			{ClusterName: "k8s-prod"},
		},
		Summary: ImportSummary{
			ClustersDiscovered: 2,
		},
	}

	saved, err := store.Save(repoPath, result, time.Date(2026, 4, 21, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if saved.Path == "" {
		t.Fatal("expected saved artifact path to be set")
	}
	if saved.RepoHash == "" {
		t.Fatal("expected repo hash to be set")
	}

	latest, err := store.LoadLatest(repoPath)
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}

	if latest.Result.RepoPath != repoPath {
		t.Fatalf("expected repo path %q, got %q", repoPath, latest.Result.RepoPath)
	}
	if len(latest.Result.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(latest.Result.Clusters))
	}
	if latest.Result.Clusters[0].ClusterName != "k8s-dev" {
		t.Fatalf("unexpected first cluster %q", latest.Result.Clusters[0].ClusterName)
	}
	if latest.Result.Summary.ClustersDiscovered != 2 {
		t.Fatalf("expected summary to record 2 discovered clusters, got %d", latest.Result.Summary.ClustersDiscovered)
	}
}

func TestArtifactStoreRepoHashIsStable(t *testing.T) {
	hash1 := RepoHash("/tmp/example-repo")
	hash2 := RepoHash("/tmp/example-repo")
	hash3 := RepoHash("/tmp/another-repo")

	if hash1 == "" {
		t.Fatal("expected repo hash to be non-empty")
	}
	if hash1 != hash2 {
		t.Fatalf("expected repo hash to be stable, got %q and %q", hash1, hash2)
	}
	if hash1 == hash3 {
		t.Fatalf("expected different repos to hash differently, both were %q", hash1)
	}
}

func TestNamespaceRegistryUsesDefaultsAndOverrides(t *testing.T) {
	registry := NewNamespaceRegistry()

	defaults := registry.NamespacesFor("cert-manager")
	if len(defaults) != 1 || defaults[0] != "cert-manager" {
		t.Fatalf("expected cert-manager default namespace, got %#v", defaults)
	}

	if err := registry.ApplyOverrides([]string{
		"cert-manager=custom-certs,platform-certs",
		"keycloak=identity",
	}); err != nil {
		t.Fatalf("ApplyOverrides() error = %v", err)
	}

	overridden := registry.NamespacesFor("cert-manager")
	if len(overridden) != 2 || overridden[0] != "custom-certs" || overridden[1] != "platform-certs" {
		t.Fatalf("expected override namespaces for cert-manager, got %#v", overridden)
	}

	keycloak := registry.NamespacesFor("keycloak")
	if len(keycloak) != 1 || keycloak[0] != "identity" {
		t.Fatalf("expected keycloak override namespace, got %#v", keycloak)
	}

	unknown := registry.NamespacesFor("unknown-service")
	if len(unknown) != 1 || unknown[0] != "unknown-service" {
		t.Fatalf("expected unknown service to fall back to its service name, got %#v", unknown)
	}
}
