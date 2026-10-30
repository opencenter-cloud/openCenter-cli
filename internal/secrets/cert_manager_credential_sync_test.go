// Copyright 2025.
// Licensed under the Apache License, Version 2.0.

package secrets

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/crypto"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// The cert-manager DNS credentials that `cluster configure` writes into the
// named maps (secrets.cert_manager.cloudflare.<name>.api_token and
// secrets.cert_manager.aws.<name>) used to reach the GitOps repository as
// plaintext. Two independent gaps produced that result:
//
//   - secretartifacts.Plan only knew the deprecated flat fields, so a named
//     credential never became a sync artifact and was therefore never
//     encrypted. SOPS' encrypted_regex could not help: it selects keys WITHIN
//     a file sync has already decided to write, and this file was never in
//     that set.
//   - the cert-manager renderer emitted the Secret itself from a Go template,
//     and the only encryption hook in the generate pipeline
//     (EncryptServiceOverrideValues) is scoped to helm-values/override-values.yaml.
//
// These tests pin the contract that closes both: the planner owns the
// credential Secret, sync encrypts it, and neither side can leave a plaintext
// credential behind.

const (
	testCloudflareToken = "cf-token-super-secret-value"
	testAWSAccessKey    = "AKIAIOSFODNN7EXAMPLE"
	testAWSSecretKey    = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
)

// TestSyncSecretsEncryptsNamedCloudflareCredential is the end-to-end proof for
// the reported failure: dns_provider: cloudflare plus a named credential must
// produce an ENCRYPTED opencenter-cloudflare-credentials-secret-<name>.yaml.
func TestSyncSecretsEncryptsNamedCloudflareCredential(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "cf-cred-cluster")
	env.configure(t, "cloudflare", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
			"cloudflare": {Enabled: true, APIToken: testCloudflareToken},
		}
	})

	env.sync(t, ctx)
	secretPath := env.credentialSecretPath(t, "cloudflare", "cloudflare")
	requireFileExists(t, secretPath)

	data := mustReadFileForSyncTest(t, secretPath)
	require.NotContainsf(t, data, testCloudflareToken,
		"Cloudflare API token leaked in plaintext into the GitOps repository:\n%s", data)
	require.Contains(t, data, "ENC[AES256_GCM,data:", "credential Secret must be SOPS-encrypted")
	require.Contains(t, data, "encrypted_regex: ^(data|stringData)$",
		"only the credential subtree may be encrypted; metadata must stay readable for Flux")

	// The file must decrypt with the cluster key and carry the exact key the
	// ClusterIssuer's apiTokenSecretRef resolves against.
	manifest := env.decryptManifest(t, ctx, secretPath)
	require.Equal(t, "opencenter-cloudflare-credentials-secret-cloudflare",
		metadataOfManifest(t, manifest)["name"],
		"the Secret name must match the name the ClusterIssuer references")
	stringData := stringDataOfManifest(t, manifest)
	require.Equal(t, testCloudflareToken, stringData["api-token"],
		"the encrypted key name must match issuerCloudflareTemplate's apiTokenSecretRef.key")
}

// TestSyncSecretsEncryptsNamedRoute53Credential covers the AWS half of the same
// bug. Both keys must land under the names issuerAWSTemplate references.
func TestSyncSecretsEncryptsNamedRoute53Credential(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "r53-cred-cluster")
	env.configure(t, "route53", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
			"route53": {
				Enabled:            true,
				AWSAccessKey:       testAWSAccessKey,
				AWSSecretAccessKey: testAWSSecretKey,
				Region:             "us-east-1",
			},
		}
	})

	env.sync(t, ctx)
	secretPath := env.credentialSecretPath(t, "aws", "route53")
	requireFileExists(t, secretPath)

	data := mustReadFileForSyncTest(t, secretPath)
	require.NotContains(t, data, testAWSAccessKey, "AWS access key leaked in plaintext")
	require.NotContains(t, data, testAWSSecretKey, "AWS secret key leaked in plaintext")
	require.Contains(t, data, "ENC[AES256_GCM,data:")

	stringData := stringDataOfManifest(t, env.decryptManifest(t, ctx, secretPath))
	require.Equal(t, testAWSAccessKey, stringData["aws-access-key"])
	require.Equal(t, testAWSSecretKey, stringData["aws-secret-access-key"])
}

// TestSyncSecretsRecordsCredentialOwnership makes the fix self-sustaining: once
// the credential Secret is in the ownership ledger, a later sync sees it as
// unchanged instead of rewriting or ignoring it.
func TestSyncSecretsRecordsCredentialOwnership(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "owned-cred-cluster")
	env.configure(t, "cloudflare", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
			"prod": {Enabled: true, APIToken: testCloudflareToken},
		}
	})

	env.sync(t, ctx)

	relative, err := secretartifacts.CertManagerCredentialArtifactPath("cloudflare", "prod")
	require.NoError(t, err)
	secretPath := filepath.Join(env.overlayPath, filepath.FromSlash(relative))

	records := env.loadOwnershipRecords(t)
	record, ok := records[relative]
	require.Truef(t, ok, "credential Secret must be recorded as owned; have %v", records)
	require.NotEmpty(t, record.Hash)
	// The owner is the credential name, and the target service is
	// cert-manager; `secrets sync --services cert-manager` matches on the
	// target, so the credential is reachable by the service users name.
	require.Equal(t, []string{"prod"}, record.Owners)

	// Second sync: unchanged, and still encrypted.
	second := env.sync(t, ctx)
	require.Contains(t, second.Unchanged, secretPath,
		"a synced credential must not be rewritten every run")

	data := mustReadFileForSyncTest(t, secretPath)
	require.NotContains(t, data, testCloudflareToken)
	require.Contains(t, data, "ENC[AES256_GCM,data:")
}

// TestGenerateSecretManifestNamesCredentialSecretAfterItsFile pins the naming
// contract independent of SOPS: the ClusterIssuer references the credential
// Secret by the name derived from the filename, so a manifest written to
// services/cert-manager/<credential-file> must carry that name rather than the
// service-derived opencenter-cert-manager-secret.
func TestGenerateSecretManifestNamesCredentialSecretAfterItsFile(t *testing.T) {
	manager := &DefaultSecretsManager{}
	secrets := map[string]interface{}{"api-token": testCloudflareToken}

	credential := manager.generateSecretManifest("cert-manager", secrets, nil,
		certManagerCredentialSecretPath(t, "cloudflare", "prod"))
	require.Equal(t, "opencenter-cloudflare-credentials-secret-prod",
		metadataOfManifest(t, credential)["name"])

	// The generic per-service route keeps its service-derived name.
	generic := manager.generateSecretManifest("cert-manager", secrets, nil,
		serviceScopedSecretPath("cert-manager"))
	require.Equal(t, "opencenter-cert-manager-secret", metadataOfManifest(t, generic)["name"])

	// The filename is the contract, so a stale name on disk is corrected rather
	// than preserved: preserve-first made the corrupted Grafana Secrets permanent,
	// because sync kept reading the bad name back out of the file it was rewriting.
	// See TestGenerateSecretManifestCorrectsExistingDedicatedSecretName.
	corrected := manager.generateSecretManifest("cert-manager", secrets, map[string]interface{}{
		"metadata": map[string]interface{}{"name": "hand-named-issuer-secret"},
	}, certManagerCredentialSecretPath(t, "cloudflare", "prod"))
	require.Equal(t, "opencenter-cloudflare-credentials-secret-prod",
		metadataOfManifest(t, corrected)["name"])
}

// TestSyncSecretsRefusesPlaintextCredentialArtifact is the guard that keeps the
// original failure from recurring silently. If a credential Secret ever reaches
// sync still in plaintext, sync must not report it as synchronized: recording
// the hash of a readable file makes every later sync call it unchanged,
// freezing the leak in place.
func TestSyncSecretsRefusesPlaintextCredentialArtifact(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "plaintext-cred-cluster")
	env.configure(t, "cloudflare", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
			"cloudflare": {Enabled: true, APIToken: testCloudflareToken},
		}
	})

	// Pre-seed the exact plaintext file the pre-fix renderer produced, as if a
	// repository generated by an older binary is being synced, and claim it in
	// the ownership ledger with that file's hash -- the state a pre-fix sync
	// would have left behind.
	relative, err := secretartifacts.CertManagerCredentialArtifactPath("cloudflare", "cloudflare")
	require.NoError(t, err)
	secretPath := filepath.Join(env.overlayPath, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(secretPath), 0o755))
	plaintext := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: opencenter-cloudflare-credentials-secret-cloudflare\n" +
		"type: Opaque\nstringData:\n  api-token: " + testCloudflareToken + "\n"
	require.NoError(t, os.WriteFile(secretPath, []byte(plaintext), 0o600))
	require.NoError(t, writeOwnershipStateForTest(t, env.overlayPath, []secretartifacts.OwnershipArtifact{{
		Path:   relative,
		Owners: []string{"cert-manager"},
		Hash:   secretartifacts.HashBytes([]byte(plaintext)),
	}}))

	result := env.sync(t, ctx)

	// Either sync encrypted it in place, or it refused. Reporting success with
	// the plaintext file intact is the failure mode under test.
	data := mustReadFileForSyncTest(t, secretPath)
	if strings.Contains(data, testCloudflareToken) {
		require.NotEmpty(t, result.Errors,
			"sync accepted a plaintext credential Secret without complaint")
		require.NotContains(t, result.Unchanged, secretPath)
		return
	}
	require.Contains(t, data, "ENC[AES256_GCM,data:")
}

// TestSyncSecretsSkipsPlaceholderCredential keeps sync from materializing a
// Secret for a credential the renderer will not emit an issuer for.
func TestSyncSecretsSkipsPlaceholderCredential(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "placeholder-cred-cluster")
	env.configure(t, "cloudflare", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.Cloudflare = map[string]v2.CertManagerCloudflareCredential{
			"cloudflare": {Enabled: true, APIToken: v2.PlaceholderSecret},
		}
	})

	env.sync(t, ctx)
	requireNoCredentialSecret(t, env.overlayPath, "cloudflare-credentials-secret")
}

// TestSyncSecretsSkipsIncompleteRoute53Credential pins planner/renderer
// agreement: the renderer requires both halves of the key pair, so a half-filled
// credential must not produce an orphan Secret.
func TestSyncSecretsSkipsIncompleteRoute53Credential(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "half-cred-cluster")
	env.configure(t, "route53", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.AWS = map[string]v2.CertManagerAWSCredential{
			"route53": {Enabled: true, AWSAccessKey: testAWSAccessKey, AWSSecretAccessKey: ""},
		}
	})

	env.sync(t, ctx)
	requireNoCredentialSecret(t, env.overlayPath, "aws-credentials-secret")
}

// TestSyncSecretsMaterializesLegacyFlatCloudflareCredential keeps the deprecated
// flat field working: it has no name, so the credential falls back to the
// provider-derived name and must still be encrypted.
func TestSyncSecretsMaterializesLegacyFlatCloudflareCredential(t *testing.T) {
	requireSOPSAvailable(t)
	ctx := context.Background()
	env := newCredentialSyncEnv(t, "legacy-flat-cluster")
	env.configure(t, "cloudflare", func(cfg *v2.Config) {
		cfg.Secrets.CertManager.Cloudflare = nil
		//lint:ignore SA1019 exercising the deprecated field's migration path.
		//nolint:staticcheck // SA1019: legacy flat credential compatibility.
		cfg.Secrets.CertManager.CloudflareAPIToken = testCloudflareToken
	})

	env.sync(t, ctx)
	secretPath := env.credentialSecretPath(t, "cloudflare", "cloudflare")
	requireFileExists(t, secretPath)
	data := mustReadFileForSyncTest(t, secretPath)
	require.NotContains(t, data, testCloudflareToken)
	require.Contains(t, data, "ENC[AES256_GCM,data:")
}

// credentialSyncEnv is a sync sandbox with a real age key pair and the real
// sops binary, so assertions observe SOPS output rather than a mock echoing its
// input.
type credentialSyncEnv struct {
	t           *testing.T
	clusterName string
	ageKeyPath  string
	overlayPath string
	cfg         *v2.Config
	manager     *DefaultSecretsManager
}

func newCredentialSyncEnv(t *testing.T, clusterName string) *credentialSyncEnv {
	t.Helper()

	// t.TempDir is symlinked on macOS (under /var/folders); the ownership-state
	// writer refuses a symlinked overlay root, and the config resolver compares
	// lexical paths. Resolve the real path so every root agrees.
	root := resolveRealPathForSyncTest(t, t.TempDir())
	configDir := filepath.Join(root, ".config", "opencenter")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	for _, entry := range []struct{ key, value string }{
		{"OPENCENTER_CONFIG_DIR", configDir},
		{"OPENCENTER_CLUSTERS_DIR", filepath.Join(configDir, "clusters")},
		{"OPENCENTER_BLUEPRINTS_DIR", filepath.Join(configDir, "clusters", "blueprints")},
		{"OPENCENTER_GITOPS_DIR", filepath.Join(configDir, "clusters", "gitops")},
		{"OPENCENTER_CLUSTER_STATE_DIR", filepath.Join(configDir, "clusters", "state")},
		{"OPENCENTER_SECRETS_DIR", filepath.Join(configDir, "clusters", "secrets")},
		// The key manager falls back to $HOME when no key directory is set.
		{"HOME", root},
	} {
		t.Setenv(entry.key, entry.value)
	}

	ageKeyPath := filepath.Join(root, "age-key.txt")
	generateAgeKeyPairForSyncTest(t, ageKeyPath)

	cfg, err := v2.NewV2Default(clusterName, "openstack")
	require.NoError(t, err)
	cfg.OpenCenter.Meta.Organization = credentialSyncTestOrg
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(root, "test-repo")
	cfg.Secrets.SopsAgeKeyFile = ageKeyPath
	cfg.Secrets.SOPSConfig = v2.SOPSConfig{Enabled: true, AgeKeyFile: ageKeyPath}

	env := &credentialSyncEnv{
		t:           t,
		clusterName: clusterName,
		ageKeyPath:  ageKeyPath,
		cfg:         cfg,
		overlayPath: filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir,
			"applications", "overlays", clusterName),
	}
	require.NoError(t, os.MkdirAll(env.overlayPath, 0o755))
	return env
}

// configure sets the DNS provider, applies a credential mutation, persists the
// blueprint where the secrets manager resolves it, and builds the manager.
func (e *credentialSyncEnv) configure(t *testing.T, dnsProvider string, mutate func(*v2.Config)) {
	t.Helper()

	certManager, ok := e.cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	require.Truef(t, ok, "cert-manager service config missing from init defaults")
	certManager.Enabled = true
	certManager.DNSProvider = dnsProvider
	certManager.Email = "ops@example.com"
	mutate(e.cfg)

	configLoader := v2.NewConfigIOHandler(fs.NewDefaultFileSystem(errors.NewDefaultErrorHandlerWithoutMasking()))
	configPath := filepath.Join(config.GetBlueprintsDir(), credentialSyncTestOrg, e.clusterName, e.clusterName+"-config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, configLoader.SaveToFile(context.Background(), configPath, e.cfg))

	e.manager = NewDefaultSecretsManager(configLoader, sops.NewSOPSManager(), nil, newQuietSyncTestLogger())
}

// sync runs a real (non-dry-run) sync and fails the test on any per-file error.
func (e *credentialSyncEnv) sync(t *testing.T, ctx context.Context) *SyncResult {
	t.Helper()
	result, err := e.manager.SyncSecrets(ctx, SyncOptions{Cluster: e.clusterName})
	require.NoError(t, err)
	require.Emptyf(t, result.Errors, "sync errors: %+v", result.Errors)
	return result
}

// credentialSecretPath resolves the expected path through the planner, so a
// test can never pass against a filename the planner no longer produces.
func (e *credentialSyncEnv) credentialSecretPath(t *testing.T, provider, name string) string {
	t.Helper()
	relative, err := secretartifacts.CertManagerCredentialArtifactPath(provider, name)
	require.NoError(t, err)
	return filepath.Join(e.overlayPath, filepath.FromSlash(relative))
}

func (e *credentialSyncEnv) loadOwnershipRecords(t *testing.T) map[string]secretartifacts.OwnershipArtifact {
	t.Helper()
	state, found, err := secretartifacts.LoadOwnershipState(e.overlayPath)
	require.NoError(t, err)
	require.True(t, found, "ownership state must exist after sync")
	return state.ByPath()
}

// decryptManifest decrypts a materialized Secret with the cluster key so a test
// can assert on the exact key names the ClusterIssuer resolves against.
func (e *credentialSyncEnv) decryptManifest(t *testing.T, ctx context.Context, manifestPath string) map[string]any {
	t.Helper()
	t.Setenv("SOPS_AGE_KEY_FILE", e.ageKeyPath)
	plaintext := filepath.Join(t.TempDir(), "decrypted.yaml")
	require.NoError(t, e.manager.sopsManager.GetEncryptor().DecryptFile(ctx, manifestPath, plaintext))
	var manifest map[string]any
	require.NoError(t, yaml.Unmarshal(mustReadFileBytesForSyncTest(t, plaintext), &manifest))
	return manifest
}

const credentialSyncTestOrg = "test-org"

func writeOwnershipStateForTest(t *testing.T, overlayPath string, artifacts []secretartifacts.OwnershipArtifact) error {
	t.Helper()
	data, err := json.Marshal(secretartifacts.OwnershipState{
		Version:   secretartifacts.OwnershipStateVersion,
		Artifacts: artifacts,
	})
	require.NoError(t, err)
	return os.WriteFile(filepath.Join(overlayPath, secretartifacts.OwnershipStateFilename), data, 0o600)
}

func generateAgeKeyPairForSyncTest(t *testing.T, path string) string {
	t.Helper()
	pair, err := crypto.NewAgeKeyGenerator().GenerateAgeKey()
	require.NoError(t, err)
	// getAgeKey reads the recipient from the comment `age-keygen` writes.
	require.NoError(t, os.WriteFile(path,
		[]byte("# created by opencenter test\n# public key: "+pair.PublicKey+"\n"+pair.PrivateKey+"\n"), 0o600))
	return pair.PublicKey
}

func resolveRealPathForSyncTest(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return resolved
}

func requireSOPSAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("sops binary not installed; encryption is exercised through the real binary")
	}
}

func requireFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoErrorf(t, err, "expected %s to exist", path)
	require.False(t, info.IsDir())
}

func requireNoCredentialSecret(t *testing.T, overlayPath, substring string) {
	t.Helper()
	var found []string
	require.NoError(t, filepath.Walk(overlayPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(info.Name(), substring) {
			found = append(found, path)
		}
		return nil
	}))
	require.Emptyf(t, found, "a credential without usable material must not be materialized as a Secret: %v", found)
}

func mustReadFileForSyncTest(t *testing.T, path string) string {
	t.Helper()
	return string(mustReadFileBytesForSyncTest(t, path))
}

func mustReadFileBytesForSyncTest(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func metadataOfManifest(t *testing.T, manifest map[string]any) map[string]any {
	t.Helper()
	metadata, ok := manifest["metadata"].(map[string]any)
	require.Truef(t, ok, "manifest has no metadata: %v", manifest)
	return metadata
}

func stringDataOfManifest(t *testing.T, manifest map[string]any) map[string]any {
	t.Helper()
	stringData, ok := manifest["stringData"].(map[string]any)
	require.Truef(t, ok, "encrypted Secret must carry stringData: %v", manifest)
	return stringData
}

func newQuietSyncTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}
