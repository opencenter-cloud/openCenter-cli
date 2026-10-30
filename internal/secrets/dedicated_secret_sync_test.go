// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

package secrets

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// syncFixtureOverlay lays down a cluster whose config drives all three dedicated
// renderer Secrets, and returns the overlay root.
//
// The dedicated files are deliberately ABSENT. That is the state `secrets sync`
// reaches on a fresh cluster, and it is the only state that exercises naming: with
// no file to read, sync has no existing metadata to preserve and must derive the
// Kubernetes name from the artifact path. A pre-seeded name would take the
// preserve-existing-metadata branch and pass whatever the fallback does.
//
// A real keypair, because sync encrypts what it writes; a public-key-only stub
// leaves the SOPS encryptor unavailable and every artifact errors out.
func syncFixtureOverlay(t *testing.T, tmpDir, cluster string) string {
	t.Helper()

	// Pin the blueprint root at the temp dir. ResolveClustersDir prefers the
	// developer's real CLI config clustersDir over OPENCENTER_CONFIG_DIR, so
	// without this writeManagerTestConfig would create the cluster under the
	// workstation config, and sync -- which then resolves git_dir relative to a
	// config it reads from a different root -- would materialize its ownership
	// ledger inside the repository.
	t.Setenv("OPENCENTER_CLUSTERS_DIR", filepath.Join(tmpDir, ".config", "opencenter", "clusters"))

	// EvalSymlinks, not the raw temp dir: on macOS t.TempDir lives under /var,
	// whose first component is a symlink, and sync's symlink guard then refuses
	// every write in the tree before the naming path is reached.
	repoDir := evalSymlinks(t, filepath.Join(tmpDir, "repo"))
	keyPath := filepath.Join(tmpDir, ".config", "sops", "age", "test-key.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(keyPath), 0o755))
	// A real keypair: sync encrypts what it writes, and a public-key-only stub
	// leaves the SOPS encryptor unavailable so every artifact errors out.
	keyPair, err := crypto.NewDefaultKeyManager(filepath.Dir(keyPath)).GenerateAgeKey()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath,
		[]byte("# public key: "+keyPair.PublicKey+"\n"+keyPair.PrivateKey+"\n"), 0o600))

	writeManagerTestConfig(t, cluster, `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: `+cluster+`
  gitops:
    git_dir: `+repoDir+`
  identity:
    oidc:
      enabled: true
  services:
    kube-prometheus-stack:
      enabled: true
    keycloak:
      enabled: true
secrets:
  sops_age_key_file: `+keyPath+`
  grafana:
    admin_user: admin
    admin_password: real-admin-password
  keycloak:
    client_secret: real-opencenter-secret
    grafana_client_secret: real-grafana-secret
`)

	overlay := filepath.Join(repoDir, "applications", "overlays", cluster)
	// The directories exist so sync writes into them; the manifests themselves do
	// not, which is what puts sync on its create path.
	for _, directory := range []string{"kube-prometheus-stack", "oidc-security-policies"} {
		require.NoError(t, os.MkdirAll(filepath.Join(overlay, "services", directory), 0o755))
	}
	return overlay
}

// fixtureAgePublicKey derives the recipient the fixture's manifests are encrypted
// to, through the same helper the CLI uses.
func fixtureAgePublicKey(t *testing.T, tmpDir string) string {
	t.Helper()
	publicKey, err := sops.AgePublicKey(filepath.Join(tmpDir, ".config", "sops", "age", "test-key.txt"))
	require.NoError(t, err)
	return publicKey
}

// evalSymlinks creates path and returns it with every symlink resolved.
func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o755))
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return resolved
}

// After sync, every dedicated Secret must still carry the name its consumer was
// rendered with. Sync used to derive the name from the logical owner because the
// dedicated-path guard was fed a basename it could never match, so both Grafana
// Secrets came back named opencenter-kube-prometheus-stack-secret. Kustomize
// registers resources by group+kind+namespace+name and rejects a build that
// declares one identity twice, so kube-prometheus-stack-override failed with
// "may not add resource with an already registered id" and never applied. Because
// kube-prometheus-stack-base installs the monitoring.coreos.com CRDs, that single
// duplicate wedged kube-prometheus-stack-base, rbac-manager-base, oidc-rbac,
// flux-monitoring and oidc-security-policies on DependencyNotReady, and the tempo
// HelmRelease failed on missing ServiceMonitor CRDs.
func TestSyncPreservesDedicatedSecretNames(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()
	// setupTestManager injects a nil encryptor for unit isolation; this test needs
	// sync to actually write encrypted manifests.
	manager.sopsManager = sops.NewDefaultSOPSManager(
		crypto.NewDefaultKeyManager(filepath.Join(tmpDir, ".config", "sops", "age")),
		sops.NewDefaultEncryptor(nil, nil), slog.Default())

	const cluster = "dedicated-names"
	overlay := syncFixtureOverlay(t, tmpDir, cluster)

	result, err := manager.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster})
	require.NoError(t, err)
	for _, syncErr := range result.Errors {
		require.NoErrorf(t, syncErr.Error, "sync failed for %s", syncErr.FilePath)
	}

	want := map[string]string{
		"services/kube-prometheus-stack/" + secretartifacts.GrafanaAdminPasswordSecretFilename: "grafana-admin-password",
		"services/kube-prometheus-stack/" + secretartifacts.GrafanaOAuthSecretFilename:         "grafana-oauth-secret",
		"services/oidc-security-policies/" + secretartifacts.GatewayOIDCSecretFilename:         "gateway-oidc-secret",
	}
	names := make(map[string]string, len(want))
	for relative, expected := range want {
		path := filepath.Join(overlay, filepath.FromSlash(relative))
		data, readErr := os.ReadFile(path)
		require.NoErrorf(t, readErr, "sync did not leave %s", relative)

		var manifest struct {
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
		}
		require.NoErrorf(t, yaml.Unmarshal(data, &manifest), "%s is not parseable", relative)
		require.Equalf(t, expected, manifest.Metadata.Name,
			"sync renamed %s; its consumer resolves %s by that exact name", relative, expected)
		names[relative] = manifest.Metadata.Name
	}

	// The two Grafana Secrets share one Kustomization directory, so equality here
	// is a build-time constraint rather than a style preference.
	require.NotEqual(t,
		names["services/kube-prometheus-stack/"+secretartifacts.GrafanaAdminPasswordSecretFilename],
		names["services/kube-prometheus-stack/"+secretartifacts.GrafanaOAuthSecretFilename],
		"one Kustomize build declared the same Secret identity twice")
}

// The kit-kat overlay is already corrupted on disk and in Git, so the repair has to
// run against files that exist and already carry the wrong name. Sync decrypts the
// existing manifest, finds the credential values unchanged, and would skip the
// rewrite entirely -- which is why the operator needs --force -- and then must
// re-derive the name rather than read the bad one back out of the preserved
// metadata. Without both halves the pushed tree can never converge.
func TestSyncRepairsCorruptedDedicatedSecretNames(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()
	manager.sopsManager = sops.NewDefaultSOPSManager(
		crypto.NewDefaultKeyManager(filepath.Join(tmpDir, ".config", "sops", "age")),
		sops.NewDefaultEncryptor(nil, nil), slog.Default())

	const cluster = "corrupted-names"
	overlay := syncFixtureOverlay(t, tmpDir, cluster)

	// First sync materializes the files with the fix in place.
	_, err := manager.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster})
	require.NoError(t, err)

	const corrupted = "opencenter-kube-prometheus-stack-secret"
	corruptedNames := map[string]string{
		"services/kube-prometheus-stack/" + secretartifacts.GrafanaAdminPasswordSecretFilename: corrupted,
		"services/kube-prometheus-stack/" + secretartifacts.GrafanaOAuthSecretFilename:         corrupted,
	}
	encryptor := sops.NewDefaultEncryptor(nil, nil)
	// The sops binary locates its identity through this variable; sync sets it
	// around its own decrypt, and this test drives the binary directly.
	t.Setenv("SOPS_AGE_KEY_FILE", filepath.Join(tmpDir, ".config", "sops", "age", "test-key.txt"))
	for relative := range corruptedNames {
		path := filepath.Join(overlay, filepath.FromSlash(relative))
		// Decrypt, rename, re-encrypt: the corruption must stay readable by sync,
		// or it is skipped before naming is ever considered.
		require.NoError(t, encryptor.DecryptFile(context.Background(), path, path))
		plain, readErr := os.ReadFile(path)
		require.NoErrorf(t, readErr, "first sync did not create %s", relative)
		var manifest map[string]interface{}
		require.NoError(t, yaml.Unmarshal(plain, &manifest))
		metadata, ok := manifest["metadata"].(map[string]interface{})
		require.True(t, ok)
		metadata["name"] = corrupted
		rewritten, marshalErr := yaml.Marshal(manifest)
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(path, rewritten, 0o600))
		require.NoError(t, encryptor.EncryptFile(context.Background(), path, sops.EncryptionConfig{
			AgeKeys:        []string{fixtureAgePublicKey(t, tmpDir)},
			EncryptedRegex: "^(data|stringData)$",
			InPlace:        true,
		}))

		// Re-record the ledger hash, exactly as the previous sync left it.
		require.NoError(t, repairOwnershipHash(overlay, relative))
	}

	result, err := manager.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster, Force: true})
	require.NoError(t, err)
	for _, syncErr := range result.Errors {
		require.NoErrorf(t, syncErr.Error, "forced sync failed for %s", syncErr.FilePath)
	}

	for relative := range corruptedNames {
		data, readErr := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(relative)))
		require.NoError(t, readErr)
		var manifest struct {
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
		}
		require.NoError(t, yaml.Unmarshal(data, &manifest))
		require.NotEqualf(t, corrupted, manifest.Metadata.Name,
			"%s still carries the corrupted name; sync preserved it instead of re-deriving", relative)
		require.NotEmptyf(t, manifest.Metadata.Name, "%s lost its name entirely", relative)
	}
}

// repairOwnershipHash re-stamps the ownership ledger entry for one artifact after a
// test rewrites the file behind sync's back.
func repairOwnershipHash(overlay, relative string) error {
	state, _, err := secretartifacts.LoadOwnershipState(overlay)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(relative)))
	if err != nil {
		return err
	}
	for i := range state.Artifacts {
		if state.Artifacts[i].Path == relative {
			state.Artifacts[i].Hash = secretartifacts.HashBytes(data)
			return testOwnershipStateWriter(overlay, state)
		}
	}
	return nil
}
