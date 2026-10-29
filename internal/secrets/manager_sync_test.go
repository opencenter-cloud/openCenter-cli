/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncServiceManifest(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("creates new manifest when it doesn't exist", func(t *testing.T) {
		service := "test-service"
		secrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}
		manifestPath := filepath.Join(tmpDir, "services", service, "secret.yaml")
		ageKey := "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"

		// In dry-run mode, should return true but not create file
		changed, err := manager.syncServiceManifest(
			context.Background(),
			service,
			secrets,
			manifestPath,
			ageKey,
			&v2.Config{Secrets: v2.SecretsConfig{SopsAgeKeyFile: filepath.Join(tmpDir, "missing-key.txt")}},
			true,  // dry-run
			false, // force
		)

		require.NoError(t, err)
		assert.True(t, changed)
		assert.NoFileExists(t, manifestPath)
	})

	t.Run("skips update when manifest unchanged and not forced", func(t *testing.T) {
		// This test verifies that when a manifest exists and the secrets haven't changed,
		// the sync operation correctly detects this and skips the update
		service := "test-service-unchanged"
		secrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}
		manifestPath := filepath.Join(tmpDir, "services", service, "secret.yaml")

		// Create the directory
		err := os.MkdirAll(filepath.Dir(manifestPath), 0755)
		require.NoError(t, err)

		// Create an existing manifest (unencrypted for testing)
		existingManifest := `apiVersion: v1
kind: Secret
metadata:
  name: opencenter-test-service-unchanged-secret
data:
  username: test-user
  password: test-pass
`
		err = os.WriteFile(manifestPath, []byte(existingManifest), 0644)
		require.NoError(t, err)

		// Mock the Age key path lookup to fail (so it can't decrypt for comparison)
		// This simulates the case where we can't verify changes
		ageKey := "age1nonexistent"

		// Without force, should skip update when it can't verify changes
		changed, err := manager.syncServiceManifest(
			context.Background(),
			service,
			secrets,
			manifestPath,
			ageKey,
			&v2.Config{Secrets: v2.SecretsConfig{SopsAgeKeyFile: filepath.Join(tmpDir, "missing-key.txt")}},
			false, // dry-run
			false, // force
		)

		require.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("updates manifest when forced", func(t *testing.T) {
		service := "test-service-forced"
		secrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}
		manifestPath := filepath.Join(tmpDir, "services", service, "secret.yaml")

		// Create the directory
		err := os.MkdirAll(filepath.Dir(manifestPath), 0755)
		require.NoError(t, err)

		// Create an existing manifest
		existingManifest := `apiVersion: v1
kind: Secret
metadata:
  name: opencenter-test-service-forced-secret
data:
  username: old-user
  password: old-pass
`
		err = os.WriteFile(manifestPath, []byte(existingManifest), 0644)
		require.NoError(t, err)

		ageKey := "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"

		// With force=true, should always update (in dry-run mode)
		changed, err := manager.syncServiceManifest(
			context.Background(),
			service,
			secrets,
			manifestPath,
			ageKey,
			&v2.Config{Secrets: v2.SecretsConfig{SopsAgeKeyFile: filepath.Join(tmpDir, "missing-key.txt")}},
			true, // dry-run
			true, // force
		)

		require.NoError(t, err)
		assert.True(t, changed)
	})
}

func TestWriteEncryptedManifest(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("creates directory if it doesn't exist", func(t *testing.T) {
		service := "test-service-newdir"
		secrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}
		manifestPath := filepath.Join(tmpDir, "new", "nested", "dir", "secret.yaml")
		ageKey := "age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p"

		// This will fail because SOPS encryptor is nil in test environment,
		// but we can verify the directory creation logic by checking the error
		_, err := manager.writeEncryptedManifest(
			context.Background(),
			service,
			secrets,
			manifestPath,
			ageKey,
			nil,
		)

		// Expect error due to nil encryptor
		assert.Error(t, err)

		// Directory should have been created before the error
		assert.DirExists(t, filepath.Dir(manifestPath))
	})

	t.Run("preserves existing manifest metadata", func(t *testing.T) {
		service := "test-service-metadata"
		secrets := map[string]interface{}{
			"username": "test-user",
			"password": "test-pass",
		}

		existingManifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "custom-name",
				"namespace": "custom-namespace",
				"labels": map[string]interface{}{
					"app": "test-app",
				},
			},
		}

		// Generate new manifest with existing metadata
		newManifest := manager.generateSecretManifest(service, secrets, existingManifest, serviceScopedSecretPath(service))

		// Verify metadata is preserved
		metadata, ok := newManifest["metadata"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "custom-name", metadata["name"])
		assert.Equal(t, "custom-namespace", metadata["namespace"])
		assert.NotNil(t, metadata["labels"])
	})
}

func TestGetAgeKeyPath(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("resolves the configured key file", func(t *testing.T) {
		keyPath := filepath.Join(tmpDir, "doritos-key.txt")
		keyContent := `# created: 2024-01-01T00:00:00Z
# public key: age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
AGE-SECRET-KEY-1GFPYYSJL7VYMDXVJZ4QQZZ7JQJQJQJQJQJQJQJQJQJQJQJQJQJQJQJQ
`
		require.NoError(t, os.WriteFile(keyPath, []byte(keyContent), 0600))

		cfg := &v2.Config{Secrets: v2.SecretsConfig{SopsAgeKeyFile: keyPath}}
		resolvedPath, err := manager.getAgeKeyPath(cfg)
		require.NoError(t, err)
		assert.Equal(t, keyPath, resolvedPath)
	})

	t.Run("returns ErrKeyNotFound when the configured key file is missing", func(t *testing.T) {
		cfg := &v2.Config{Secrets: v2.SecretsConfig{SopsAgeKeyFile: filepath.Join(tmpDir, "missing-key.txt")}}

		_, err := manager.getAgeKeyPath(cfg)

		assert.Error(t, err)
		var notFound *ErrKeyNotFound
		assert.True(t, errors.As(err, &notFound))
	})

	t.Run("returns ErrKeyNotFound when no key file is configured", func(t *testing.T) {
		_, err := manager.getAgeKeyPath(&v2.Config{})

		assert.Error(t, err)
		var notFound *ErrKeyNotFound
		assert.True(t, errors.As(err, &notFound))
	})
}

func TestHasSecretsChangedEdgeCases(t *testing.T) {
	manager, _, cleanup := setupTestManager(t)
	defer cleanup()

	t.Run("handles nil values", func(t *testing.T) {
		newSecrets := map[string]interface{}{
			"key1": nil,
		}

		existingSecrets := map[string]interface{}{
			"key1": nil,
		}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		assert.False(t, changed)
	})

	t.Run("handles different types", func(t *testing.T) {
		newSecrets := map[string]interface{}{
			"key1": "string-value",
			"key2": 123,
			"key3": true,
		}

		existingSecrets := map[string]interface{}{
			"key1": "string-value",
			"key2": 123,
			"key3": true,
		}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		assert.False(t, changed)
	})

	t.Run("detects type changes as strings", func(t *testing.T) {
		// Note: Our implementation converts all values to strings for comparison,
		// so "123" and 123 are considered equal. This is intentional for secret comparison.
		newSecrets := map[string]interface{}{
			"key1": "123",
		}

		existingSecrets := map[string]interface{}{
			"key1": 123,
		}

		changed := manager.hasSecretsChanged(newSecrets, existingSecrets)
		// Should be false because both convert to "123" as strings
		assert.False(t, changed)
	})
}

func TestReconcileArtifactStateRejectsUnsafeOwnedPaths(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()
	overlay := filepath.Join(tmpDir, "overlay")
	require.NoError(t, os.MkdirAll(filepath.Join(overlay, "services", "old"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(overlay, "services", "old", "secret.yaml"), []byte("owned"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(overlay, "services", "user"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(overlay, "services", "user", "secret.yaml"), []byte("user"), 0o600))
	state := artifactState{Paths: []string{"services/old/secret.yaml", "../outside/secret.yaml"}}
	data, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(overlay, artifactStateFilename), data, 0o600))

	_, err = manager.loadArtifactState(overlay)
	require.ErrorContains(t, err, "invalid path")
	assert.FileExists(t, filepath.Join(overlay, "services", "old", "secret.yaml"))
	assert.FileExists(t, filepath.Join(overlay, "services", "user", "secret.yaml"))
}

func TestReconcileArtifactStateFilteredSyncPreservesOtherService(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()
	overlay := filepath.Join(tmpDir, "overlay")
	for _, service := range []string{"one", "two"} {
		path := filepath.Join(overlay, "services", service, "secret.yaml")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(service), 0o600))
	}
	result := &SyncResult{}
	manager.reconcileArtifactState(overlay, map[string]bool{
		"services/one/secret.yaml": true,
		"services/two/secret.yaml": true,
	}, nil, []string{"one"}, map[string]bool{}, result, false)
	require.Empty(t, result.Errors)
	assert.NoFileExists(t, filepath.Join(overlay, "services", "one", "secret.yaml"))
	assert.FileExists(t, filepath.Join(overlay, "services", "two", "secret.yaml"))
}

func TestReconcileSuccessfulChangesRetainsStaleRecordsAfterSiblingError(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()
	overlay := filepath.Join(tmpDir, "overlay")
	stalePath := filepath.Join(overlay, "services", "stale", "secret.yaml")
	createdPath := filepath.Join(overlay, "services", "created", "secret.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(stalePath), 0o755))
	require.NoError(t, os.WriteFile(stalePath, []byte("stale"), 0o640))
	staleRecord := secretartifacts.OwnershipArtifact{Path: "services/stale/secret.yaml", Owners: []string{"stale"}, Hash: secretartifacts.HashBytes([]byte("stale"))}
	createdRecord := secretartifacts.OwnershipArtifact{Path: "services/created/secret.yaml", Owners: []string{"created"}, Hash: secretartifacts.HashBytes([]byte("created"))}
	require.NoError(t, os.MkdirAll(filepath.Dir(createdPath), 0o755))
	require.NoError(t, os.WriteFile(createdPath, []byte("created"), 0o600))
	manager.ownershipStateWriter = testOwnershipStateWriter
	journal := &secretMutationJournal{}
	journal.record(secretFileSnapshot{path: createdPath}, createdPath, createdRecord.Hash, false)
	result := &SyncResult{Errors: []SyncError{{FilePath: "sibling", Error: errors.New("sibling failed")}}}
	manager.reconcileArtifactStateWithRecordsAndJournal(
		overlay,
		map[string]secretartifacts.OwnershipArtifact{staleRecord.Path: staleRecord},
		[]secretartifacts.Artifact{{Path: createdRecord.Path, LogicalService: "created", TargetService: "created"}},
		nil,
		map[string]secretartifacts.OwnershipArtifact{createdRecord.Path: createdRecord},
		result,
		false,
		journal,
	)
	require.Len(t, result.Errors, 1)
	assert.FileExists(t, stalePath)
	assert.FileExists(t, createdPath)
	state, _, err := secretartifacts.LoadOwnershipState(overlay)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{staleRecord.Path, createdRecord.Path}, ownershipPaths(state))

	previous := state.ByPath()
	retry := &SyncResult{}
	manager.reconcileArtifactStateWithRecordsAndJournal(overlay, previous, []secretartifacts.Artifact{{Path: createdRecord.Path, LogicalService: "created", TargetService: "created"}}, nil, nil, retry, false, &secretMutationJournal{})
	require.Empty(t, retry.Errors)
	assert.NoFileExists(t, stalePath)
	assert.FileExists(t, createdPath)
}

func TestReconcileStateWriteFailureRollsBackAllMutationsAndRetry(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()
	overlay := filepath.Join(tmpDir, "overlay")
	createdPath := filepath.Join(overlay, "services", "created", "secret.yaml")
	updatedPath := filepath.Join(overlay, "services", "updated", "secret.yaml")
	stalePath := filepath.Join(overlay, "services", "stale", "secret.yaml")
	for _, path := range []string{createdPath, updatedPath, stalePath} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	}
	require.NoError(t, os.WriteFile(updatedPath, []byte("old updated"), 0o640))
	require.NoError(t, os.WriteFile(stalePath, []byte("old stale"), 0o600))
	updatedRecord := secretartifacts.OwnershipArtifact{Path: "services/updated/secret.yaml", Owners: []string{"updated"}, Hash: secretartifacts.HashBytes([]byte("new updated"))}
	staleRecord := secretartifacts.OwnershipArtifact{Path: "services/stale/secret.yaml", Owners: []string{"stale"}, Hash: secretartifacts.HashBytes([]byte("old stale"))}
	createdRecord := secretartifacts.OwnershipArtifact{Path: "services/created/secret.yaml", Owners: []string{"created"}, Hash: secretartifacts.HashBytes([]byte("new created"))}
	previous := map[string]secretartifacts.OwnershipArtifact{updatedRecord.Path: {Path: updatedRecord.Path, Owners: updatedRecord.Owners, Hash: secretartifacts.HashBytes([]byte("old updated"))}, staleRecord.Path: staleRecord}
	oldState := secretartifacts.OwnershipState{Version: secretartifacts.OwnershipStateVersion}
	for _, record := range previous {
		oldState.Artifacts = append(oldState.Artifacts, record)
	}
	require.NoError(t, testOwnershipStateWriter(overlay, oldState))
	oldStateBytes, err := os.ReadFile(filepath.Join(overlay, artifactStateFilename))
	require.NoError(t, err)

	journal := &secretMutationJournal{}
	createdBefore, err := snapshotSecretFile(createdPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(createdPath, []byte("new created"), 0o600))
	journal.record(createdBefore, createdPath, createdRecord.Hash, false)
	updatedBefore, err := snapshotSecretFile(updatedPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(updatedPath, []byte("new updated"), 0o644))
	journal.record(updatedBefore, updatedPath, updatedRecord.Hash, false)

	result := &SyncResult{
		Created:   []string{createdPath},
		Updated:   []string{updatedPath},
		Unchanged: []string{"unchanged"},
	}
	manager.ownershipStateWriter = func(string, secretartifacts.OwnershipState) error { return errors.New("state write failed") }
	manager.reconcileArtifactStateWithRecordsAndJournal(
		overlay,
		previous,
		[]secretartifacts.Artifact{{Path: createdRecord.Path}, {Path: updatedRecord.Path}},
		nil,
		map[string]secretartifacts.OwnershipArtifact{createdRecord.Path: createdRecord, updatedRecord.Path: updatedRecord},
		result,
		false,
		journal,
	)
	require.NotEmpty(t, result.Errors)
	assert.Empty(t, result.Created)
	assert.Empty(t, result.Updated)
	assert.Equal(t, []string{"unchanged"}, result.Unchanged)
	assert.NoFileExists(t, createdPath)
	updatedData, err := os.ReadFile(updatedPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("old updated"), updatedData)
	updatedInfo, err := os.Stat(updatedPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), updatedInfo.Mode().Perm())
	staleData, err := os.ReadFile(stalePath)
	require.NoError(t, err)
	assert.Equal(t, []byte("old stale"), staleData)
	stateBytes, err := os.ReadFile(filepath.Join(overlay, artifactStateFilename))
	require.NoError(t, err)
	assert.Equal(t, oldStateBytes, stateBytes)

	// A retry can write the artifacts again, prune the stale artifact, and commit state.
	require.NoError(t, os.WriteFile(createdPath, []byte("new created"), 0o600))
	require.NoError(t, os.WriteFile(updatedPath, []byte("new updated"), 0o644))
	retryJournal := &secretMutationJournal{}
	retryCreatedBefore := secretFileSnapshot{path: createdPath}
	retryJournal.record(retryCreatedBefore, createdPath, createdRecord.Hash, false)
	retryUpdatedBefore, err := snapshotSecretFile(updatedPath)
	require.NoError(t, err)
	retryJournal.record(retryUpdatedBefore, updatedPath, updatedRecord.Hash, false)
	manager.ownershipStateWriter = testOwnershipStateWriter
	retry := &SyncResult{}
	manager.reconcileArtifactStateWithRecordsAndJournal(overlay, previous, []secretartifacts.Artifact{{Path: createdRecord.Path}, {Path: updatedRecord.Path}}, nil, map[string]secretartifacts.OwnershipArtifact{createdRecord.Path: createdRecord, updatedRecord.Path: updatedRecord}, retry, false, retryJournal)
	require.Empty(t, retry.Errors)
	assert.NoFileExists(t, stalePath)
	state, _, err := secretartifacts.LoadOwnershipState(overlay)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{createdRecord.Path, updatedRecord.Path}, ownershipPaths(state))
}

func TestSyncSecretsSerializesConcurrentTransactions(t *testing.T) {
	manager, tmpDir, cleanup := setupTestManager(t)
	defer cleanup()

	cluster := "concurrent-sync"
	repoDir := filepath.Join(tmpDir, "repo")
	keyPath := filepath.Join(tmpDir, ".config", "sops", "age", "test-key.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(keyPath), 0o755))
	require.NoError(t, os.WriteFile(keyPath, []byte("# public key: age1test-key\n"), 0o600))
	configData := `schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: ` + cluster + `
  gitops:
    git_dir: ` + repoDir + `
secrets:
  sops_age_key_file: ` + keyPath + `
  cert_manager:
    aws_access_key: cert-manager
  grafana:
    admin_password: grafana
`
	writeManagerTestConfig(t, cluster, configData)
	overlay := filepath.Join(repoDir, "applications", "overlays", cluster)
	paths := map[string]string{
		"services/cert-manager/secret.yaml": "cert-manager manifest",
		// Grafana's admin credential is planned at the dedicated manifest, not the
		// generic per-service one: kube-prometheus-stack is declared by the config,
		// so it replaces services/kube-prometheus-stack/secret.yaml.
		"services/kube-prometheus-stack/grafana-admin-password.yaml": "grafana manifest",
	}
	state := artifactState{Version: secretartifacts.OwnershipStateVersion}
	for relative, contents := range paths {
		fullPath := filepath.Join(overlay, filepath.FromSlash(relative))
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(contents), 0o600))
		state.Artifacts = append(state.Artifacts, secretartifacts.OwnershipArtifact{
			Path: relative,
			Hash: secretartifacts.HashBytes([]byte(contents)),
		})
	}
	require.NoError(t, testOwnershipStateWriter(overlay, state))

	managerB := NewDefaultSecretsManager(manager.configLoader, manager.sopsManager, nil, manager.logger)
	enteredA := make(chan struct{})
	releaseA := make(chan struct{})
	enteredB := make(chan struct{})
	manager.ownershipStateWriter = func(root string, state secretartifacts.OwnershipState) error {
		close(enteredA)
		<-releaseA
		return testOwnershipStateWriter(root, state)
	}
	managerB.ownershipStateWriter = func(root string, state secretartifacts.OwnershipState) error {
		close(enteredB)
		return testOwnershipStateWriter(root, state)
	}

	type syncOutcome struct {
		result *SyncResult
		err    error
	}
	aDone := make(chan syncOutcome, 1)
	go func() {
		result, err := manager.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster, Services: []string{"cert-manager"}})
		aDone <- syncOutcome{result: result, err: err}
	}()
	select {
	case <-enteredA:
	case outcome := <-aDone:
		require.NoError(t, outcome.err)
		t.Fatal("manager A completed before reaching the ownership-state writer")
	case <-time.After(time.Second):
		t.Fatal("manager A did not reach the ownership-state writer")
	}

	bDone := make(chan syncOutcome, 1)
	go func() {
		result, err := managerB.SyncSecrets(context.Background(), SyncOptions{Cluster: cluster, Services: []string{"grafana"}})
		bDone <- syncOutcome{result: result, err: err}
	}()
	select {
	case <-enteredB:
		t.Fatal("manager B reached the ownership-state writer while manager A held the lock")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseA)
	outcomeA := <-aDone
	outcomeB := <-bDone
	require.NoError(t, outcomeA.err)
	require.NoError(t, outcomeB.err)
	require.Empty(t, outcomeA.result.Errors)
	require.Empty(t, outcomeB.result.Errors)

	finalState, _, err := secretartifacts.LoadOwnershipState(overlay)
	require.NoError(t, err)
	finalByPath := finalState.ByPath()
	for relative := range paths {
		record, ok := finalByPath[relative]
		require.True(t, ok, "missing final ownership record for %s", relative)
		data, err := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(relative)))
		require.NoError(t, err)
		assert.Equal(t, secretartifacts.HashBytes(data), record.Hash)
	}
}

func ownershipPaths(state secretartifacts.OwnershipState) []string {
	paths := make([]string, 0, len(state.Artifacts))
	for _, artifact := range state.Artifacts {
		paths = append(paths, artifact.Path)
	}
	return paths
}

func testOwnershipStateWriter(root string, state secretartifacts.OwnershipState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, artifactStateFilename), append(data, '\n'), 0o600)
}
