package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupReconcileTest(t *testing.T, cluster, recipients string) (*DefaultKeyReconciler, *MockKeyRegistry, *DefaultSecretsManager, string, func()) {
	t.Helper()
	rotator, registry, manager, tmpDir, cleanup := setupTestRotator(t)
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0o755))
	writeRotationTestConfig(t, cluster, `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: `+cluster+`
  meta:
    organization: test-org
  gitops:
    git_dir: `+repoDir+`
secrets:
  sops_age_key_file: ~/.config/sops/age/test-key.txt
`)
	overlayPath := filepath.Join(repoDir, "applications", "overlays", cluster)
	require.NoError(t, os.MkdirAll(overlayPath, 0o755))
	sopsPath := filepath.Join(overlayPath, ".sops.yaml")
	require.NoError(t, os.WriteFile(sopsPath, []byte("creation_rules:\n  - path_regex: .*\\.yaml$\n    age: "+recipients+"\n"), 0o644))
	_ = rotator
	reconciler := NewDefaultKeyReconciler(registry, manager, nil)
	return reconciler, registry, manager, sopsPath, cleanup
}

func registerReconcileKey(t *testing.T, registry *MockKeyRegistry, cluster, recipient string, status KeyStatus) {
	t.Helper()
	require.NoError(t, registry.RegisterKey(context.Background(), KeyEntry{
		Cluster: cluster, KeyType: KeyTypeAge, Fingerprint: recipient, PublicKey: recipient,
		Status: status, CreatedAt: time.Now(),
	}))
}

func TestReconcileNoDrift(t *testing.T) {
	reconciler, registry, _, _, cleanup := setupReconcileTest(t, "reconcile-no-drift", "age1a,age1b")
	defer cleanup()
	registerReconcileKey(t, registry, "reconcile-no-drift", "age1a", KeyStatusActive)
	registerReconcileKey(t, registry, "reconcile-no-drift", "age1b", KeyStatusActive)

	report, err := reconciler.Reconcile(context.Background(), "reconcile-no-drift", false)
	require.NoError(t, err)
	assert.False(t, report.HasDrift())
	assert.Empty(t, report.Imported)
}

func TestReconcileImportsMissingRecipient(t *testing.T) {
	reconciler, registry, _, _, cleanup := setupReconcileTest(t, "reconcile-import", "age1known,age1missing")
	defer cleanup()
	registerReconcileKey(t, registry, "reconcile-import", "age1known", KeyStatusActive)

	report, err := reconciler.Reconcile(context.Background(), "reconcile-import", false)
	require.NoError(t, err)
	require.Equal(t, []string{"age1missing"}, report.OnlyInSOPSConfig)

	report, err = reconciler.Reconcile(context.Background(), "reconcile-import", true)
	require.NoError(t, err)
	assert.Equal(t, []string{"age1missing"}, report.Imported)
	keys, err := registry.ListKeys(context.Background(), "reconcile-import")
	require.NoError(t, err)
	var imported KeyEntry
	for _, key := range keys {
		if key.Fingerprint == "age1missing" {
			imported = key
		}
	}
	assert.Equal(t, KeyStatusActive, imported.Status)
	assert.False(t, imported.Primary)

	report, err = reconciler.Reconcile(context.Background(), "reconcile-import", false)
	require.NoError(t, err)
	assert.False(t, report.HasDrift())
}

func TestReconcileOnlyInRegistryDoesNotBlock(t *testing.T) {
	reconciler, registry, _, _, cleanup := setupReconcileTest(t, "reconcile-registry-only", "age1in-sops")
	defer cleanup()
	registerReconcileKey(t, registry, "reconcile-registry-only", "age1in-sops", KeyStatusActive)
	registerReconcileKey(t, registry, "reconcile-registry-only", "age1registry-only", KeyStatusActive)

	report, err := reconciler.Reconcile(context.Background(), "reconcile-registry-only", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"age1registry-only"}, report.OnlyInRegistry)
	assert.True(t, report.HasDrift())
}

func TestReconcileReportsRevokedRecipient(t *testing.T) {
	reconciler, registry, _, _, cleanup := setupReconcileTest(t, "reconcile-revoked", "age1revoked")
	defer cleanup()
	registerReconcileKey(t, registry, "reconcile-revoked", "age1revoked", KeyStatusRevoked)

	report, err := reconciler.Reconcile(context.Background(), "reconcile-revoked", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"age1revoked"}, report.OnlyInSOPSConfig)
	assert.Equal(t, []string{"age1revoked"}, report.RecipientsRevokedButStillInSOPSConfig)
	assert.True(t, report.HasDrift())
}

type duplicateReconcileRegistry struct {
	*MockKeyRegistry
	entries []KeyEntry
}

func (r *duplicateReconcileRegistry) ListKeys(context.Context, string) ([]KeyEntry, error) {
	return r.entries, nil
}

func TestReconcileReportsDuplicateFingerprints(t *testing.T) {
	_, registry, manager, _, cleanup := setupReconcileTest(t, "reconcile-duplicates", "age1one")
	defer cleanup()
	duplicateRegistry := &duplicateReconcileRegistry{
		MockKeyRegistry: registry,
		entries: []KeyEntry{
			{Cluster: "reconcile-duplicates", KeyType: KeyTypeAge, Fingerprint: "age1duplicate", PublicKey: "age1duplicate", Status: KeyStatusActive},
			{Cluster: "reconcile-duplicates", KeyType: KeyTypeAge, Fingerprint: "age1duplicate", PublicKey: "age1duplicate", Status: KeyStatusArchived},
		},
	}
	reconciler := NewDefaultKeyReconciler(duplicateRegistry, manager, nil)
	report, err := reconciler.Reconcile(context.Background(), "reconcile-duplicates", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"age1duplicate"}, report.DuplicateFingerprints)
}

func TestReconcileApplyFalseDoesNotMutate(t *testing.T) {
	reconciler, registry, _, sopsPath, cleanup := setupReconcileTest(t, "reconcile-no-apply", "age1missing")
	defer cleanup()
	before, err := os.ReadFile(sopsPath)
	require.NoError(t, err)

	report, err := reconciler.Reconcile(context.Background(), "reconcile-no-apply", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"age1missing"}, report.OnlyInSOPSConfig)
	after, err := os.ReadFile(sopsPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	keys, err := registry.ListKeys(context.Background(), "reconcile-no-apply")
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestRevocationPreflightAbortsBothPaths(t *testing.T) {
	for _, mode := range []string{"fingerprint", "user"} {
		t.Run(mode, func(t *testing.T) {
			cluster := "reconcile-revoke-" + mode
			reconciler, registry, manager, sopsPath, cleanup := setupReconcileTest(t, cluster, "age1target,age1remaining,age1unregistered")
			defer cleanup()
			registerReconcileKey(t, registry, cluster, "age1target", KeyStatusActive)
			registerReconcileKey(t, registry, cluster, "age1remaining", KeyStatusActive)
			beforeSOPS, err := os.ReadFile(sopsPath)
			require.NoError(t, err)
			manifestPath := filepath.Join(filepath.Dir(sopsPath), "manifest.yaml")
			beforeManifest := []byte("manifest: unchanged\n")
			require.NoError(t, os.WriteFile(manifestPath, beforeManifest, 0o644))

			revoker := NewDefaultKeyRevoker(registry, &MockKeyRotator{}, manager, nil, nil)
			_ = reconciler
			var result *RevocationResult
			if mode == "fingerprint" {
				result, err = revoker.RevokeByFingerprint(context.Background(), RevokeOptions{Cluster: cluster, Fingerprint: "age1target"})
			} else {
				keys, listErr := registry.ListKeys(context.Background(), cluster)
				require.NoError(t, listErr)
				for index := range keys {
					if keys[index].Fingerprint == "age1target" {
						keys[index].UserEmail = "departing@example.com"
						require.NoError(t, registry.UpdateKey(context.Background(), keys[index]))
						break
					}
				}
				result, err = revoker.RevokeByUser(context.Background(), RevokeOptions{Cluster: cluster, User: "departing@example.com"})
			}
			assert.Nil(t, result)
			var driftErr *ErrRegistryDriftDetected
			assert.True(t, errors.As(err, &driftErr))
			afterSOPS, readErr := os.ReadFile(sopsPath)
			require.NoError(t, readErr)
			afterManifest, readErr := os.ReadFile(manifestPath)
			require.NoError(t, readErr)
			assert.Equal(t, beforeSOPS, afterSOPS)
			assert.Equal(t, beforeManifest, afterManifest)
		})
	}
}

func TestRevokeAfterApplyPreservesPreviouslyUnregisteredRecipient(t *testing.T) {
	cluster := "reconcile-revoke-after-apply"
	reconciler, registry, manager, sopsPath, cleanup := setupReconcileTest(t, cluster, "age1target,age1remaining,age1unregistered")
	defer cleanup()
	registerReconcileKey(t, registry, cluster, "age1remaining", KeyStatusActive)
	registerReconcileKey(t, registry, cluster, "age1target", KeyStatusActive)

	report, err := reconciler.Reconcile(context.Background(), cluster, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"age1unregistered"}, report.Imported)

	revoker := NewDefaultKeyRevoker(registry, &MockKeyRotator{}, manager, nil, nil)
	_, err = revoker.RevokeByFingerprint(context.Background(), RevokeOptions{Cluster: cluster, Fingerprint: "age1target"})
	require.NoError(t, err)
	data, err := os.ReadFile(sopsPath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "age1target")
	assert.Contains(t, string(data), "age1unregistered")
}

func TestReconcileApplySelectsSoleActiveAgeKey(t *testing.T) {
	reconciler, registry, _, _, cleanup := setupReconcileTest(t, "reconcile-singular", "age1sole")
	defer cleanup()
	report, err := reconciler.Reconcile(context.Background(), "reconcile-singular", true)
	require.NoError(t, err)
	require.Equal(t, []string{"age1sole"}, report.Imported)
	primary, err := registry.GetPrimaryKey(context.Background(), "reconcile-singular", KeyTypeAge)
	require.NoError(t, err)
	assert.Equal(t, "age1sole", primary.Fingerprint)
}

func TestReconcileApplyLeavesAmbiguousAgeKeysUnselected(t *testing.T) {
	reconciler, registry, _, _, cleanup := setupReconcileTest(t, "reconcile-ambiguous", "age1one,age1two")
	defer cleanup()
	report, err := reconciler.Reconcile(context.Background(), "reconcile-ambiguous", true)
	require.NoError(t, err)
	require.Len(t, report.Imported, 2)
	_, err = registry.GetPrimaryKey(context.Background(), "reconcile-ambiguous", KeyTypeAge)
	require.Error(t, err)
}
