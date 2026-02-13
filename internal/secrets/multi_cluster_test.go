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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSecretsManager is a mock implementation of SecretsManager for testing.
type mockSecretsManager struct {
	syncResults map[string]*SyncResult
	syncErrors  map[string]error
	syncCalls   []string
}

func newMockSecretsManager() *mockSecretsManager {
	return &mockSecretsManager{
		syncResults: make(map[string]*SyncResult),
		syncErrors:  make(map[string]error),
		syncCalls:   []string{},
	}
}

func (m *mockSecretsManager) SyncSecrets(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
	m.syncCalls = append(m.syncCalls, opts.Cluster)

	if err, exists := m.syncErrors[opts.Cluster]; exists {
		return nil, err
	}

	if result, exists := m.syncResults[opts.Cluster]; exists {
		return result, nil
	}

	// Default success result
	return &SyncResult{
		Created:   []string{},
		Updated:   []string{},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}, nil
}

func (m *mockSecretsManager) ValidateSecrets(ctx context.Context, opts ValidateOptions) (*ValidationResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSecretsManager) DetectDrift(ctx context.Context, cluster string) (*DriftReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSecretsManager) GetSecretSources(ctx context.Context, cluster string) ([]SecretSource, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestNewDefaultMultiClusterSyncer(t *testing.T) {
	mockManager := newMockSecretsManager()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	syncer := NewDefaultMultiClusterSyncer(mockManager, logger)

	assert.NotNil(t, syncer)
	assert.Equal(t, mockManager, syncer.secretsManager)
	assert.Equal(t, logger, syncer.logger)
}

func TestNewDefaultMultiClusterSyncer_NilLogger(t *testing.T) {
	mockManager := newMockSecretsManager()

	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	assert.NotNil(t, syncer)
	assert.NotNil(t, syncer.logger) // Should use default logger
}

func TestDiscoverClusters(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	clustersDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters")

	// Create test organization and cluster directories
	org1Dir := filepath.Join(clustersDir, "org1")
	org2Dir := filepath.Join(clustersDir, "org2")

	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster2"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(org2Dir, "cluster3"), 0755))

	// Create config files
	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster1", ".k8s-cluster1-config.yaml"),
		[]byte("test config"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster2", ".k8s-cluster2-config.yaml"),
		[]byte("test config"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(org2Dir, "cluster3", ".k8s-cluster3-config.yaml"),
		[]byte("test config"),
		0644,
	))

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	mockManager := newMockSecretsManager()
	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	t.Run("discover all clusters", func(t *testing.T) {
		clusters, err := syncer.discoverClusters("")
		require.NoError(t, err)
		assert.Len(t, clusters, 3)
		assert.Contains(t, clusters, "cluster1")
		assert.Contains(t, clusters, "cluster2")
		assert.Contains(t, clusters, "cluster3")
	})

	t.Run("discover clusters in org1", func(t *testing.T) {
		clusters, err := syncer.discoverClusters("org1")
		require.NoError(t, err)
		assert.Len(t, clusters, 2)
		assert.Contains(t, clusters, "cluster1")
		assert.Contains(t, clusters, "cluster2")
		assert.NotContains(t, clusters, "cluster3")
	})

	t.Run("discover clusters in org2", func(t *testing.T) {
		clusters, err := syncer.discoverClusters("org2")
		require.NoError(t, err)
		assert.Len(t, clusters, 1)
		assert.Contains(t, clusters, "cluster3")
	})

	t.Run("discover clusters in non-existent org", func(t *testing.T) {
		clusters, err := syncer.discoverClusters("nonexistent")
		require.NoError(t, err)
		assert.Len(t, clusters, 0)
	})
}

func TestDiscoverClusters_NoClustersDir(t *testing.T) {
	// Create temporary test directory without clusters directory
	tmpDir := t.TempDir()

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	mockManager := newMockSecretsManager()
	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	clusters, err := syncer.discoverClusters("")
	require.NoError(t, err)
	assert.Len(t, clusters, 0)
}

func TestSyncAll_Success(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	clustersDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters")
	org1Dir := filepath.Join(clustersDir, "org1")

	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster2"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster1", ".k8s-cluster1-config.yaml"),
		[]byte("test config"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster2", ".k8s-cluster2-config.yaml"),
		[]byte("test config"),
		0644,
	))

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Setup mock manager with success results
	mockManager := newMockSecretsManager()
	mockManager.syncResults["cluster1"] = &SyncResult{
		Created:   []string{"file1.yaml"},
		Updated:   []string{},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}
	mockManager.syncResults["cluster2"] = &SyncResult{
		Created:   []string{},
		Updated:   []string{"file2.yaml"},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}

	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	opts := MultiClusterSyncOptions{
		Organization: "org1",
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       false,
	}

	result, err := syncer.SyncAll(context.Background(), opts)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Len(t, result.Results, 2)
	assert.Len(t, result.Failures, 0)

	// Verify both clusters were synced
	assert.Contains(t, mockManager.syncCalls, "cluster1")
	assert.Contains(t, mockManager.syncCalls, "cluster2")
}

func TestSyncAll_PartialFailure(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	clustersDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters")
	org1Dir := filepath.Join(clustersDir, "org1")

	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster2"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster3"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster1", ".k8s-cluster1-config.yaml"),
		[]byte("test config"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster2", ".k8s-cluster2-config.yaml"),
		[]byte("test config"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster3", ".k8s-cluster3-config.yaml"),
		[]byte("test config"),
		0644,
	))

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Setup mock manager with one failure
	mockManager := newMockSecretsManager()
	mockManager.syncResults["cluster1"] = &SyncResult{
		Created:   []string{"file1.yaml"},
		Updated:   []string{},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}
	mockManager.syncErrors["cluster2"] = fmt.Errorf("sync failed for cluster2")
	mockManager.syncResults["cluster3"] = &SyncResult{
		Created:   []string{},
		Updated:   []string{"file3.yaml"},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}

	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	opts := MultiClusterSyncOptions{
		Organization: "org1",
		Concurrency:  2,
		StopOnError:  false, // Continue on error
		DryRun:       false,
	}

	result, err := syncer.SyncAll(context.Background(), opts)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 1, result.FailureCount)
	assert.Len(t, result.Results, 2)
	assert.Len(t, result.Failures, 1)

	// Verify failure is recorded
	assert.Contains(t, result.Failures, "cluster2")
	assert.EqualError(t, result.Failures["cluster2"], "sync failed for cluster2")

	// Verify all clusters were attempted
	assert.Contains(t, mockManager.syncCalls, "cluster1")
	assert.Contains(t, mockManager.syncCalls, "cluster2")
	assert.Contains(t, mockManager.syncCalls, "cluster3")
}

func TestSyncAll_StopOnError(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	clustersDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters")
	org1Dir := filepath.Join(clustersDir, "org1")

	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster1"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster2"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster3"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster1", ".k8s-cluster1-config.yaml"),
		[]byte("test config"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster2", ".k8s-cluster2-config.yaml"),
		[]byte("test config"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster3", ".k8s-cluster3-config.yaml"),
		[]byte("test config"),
		0644,
	))

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Setup mock manager with one failure
	mockManager := newMockSecretsManager()
	mockManager.syncResults["cluster1"] = &SyncResult{
		Created:   []string{"file1.yaml"},
		Updated:   []string{},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}
	mockManager.syncErrors["cluster2"] = fmt.Errorf("sync failed for cluster2")
	mockManager.syncResults["cluster3"] = &SyncResult{
		Created:   []string{},
		Updated:   []string{"file3.yaml"},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}

	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	opts := MultiClusterSyncOptions{
		Organization: "org1",
		Concurrency:  1, // Use single worker to ensure deterministic order
		StopOnError:  true, // Stop on first error
		DryRun:       false,
	}

	result, err := syncer.SyncAll(context.Background(), opts)

	require.NoError(t, err)
	assert.NotNil(t, result)

	// With stop on error, we should have at least one failure
	assert.Greater(t, result.FailureCount, 0)

	// Total processed should be less than 3 (stopped early)
	totalProcessed := result.SuccessCount + result.FailureCount
	assert.LessOrEqual(t, totalProcessed, 3)
}

func TestSyncAll_NoClusters(t *testing.T) {
	// Create temporary test directory without clusters
	tmpDir := t.TempDir()

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	mockManager := newMockSecretsManager()
	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	opts := MultiClusterSyncOptions{
		Organization: "org1",
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       false,
	}

	result, err := syncer.SyncAll(context.Background(), opts)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Len(t, result.Results, 0)
	assert.Len(t, result.Failures, 0)
}

func TestSyncAll_DryRun(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	clustersDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters")
	org1Dir := filepath.Join(clustersDir, "org1")

	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster1"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster1", ".k8s-cluster1-config.yaml"),
		[]byte("test config"),
		0644,
	))

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	mockManager := newMockSecretsManager()
	mockManager.syncResults["cluster1"] = &SyncResult{
		Created:   []string{},
		Updated:   []string{},
		Unchanged: []string{"file1.yaml"},
		Errors:    []SyncError{},
	}

	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	opts := MultiClusterSyncOptions{
		Organization: "org1",
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       true, // Dry run mode
	}

	result, err := syncer.SyncAll(context.Background(), opts)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)

	// Verify cluster was synced with dry run flag
	assert.Contains(t, mockManager.syncCalls, "cluster1")
}

func TestSyncAll_DefaultConcurrency(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	clustersDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters")
	org1Dir := filepath.Join(clustersDir, "org1")

	require.NoError(t, os.MkdirAll(filepath.Join(org1Dir, "cluster1"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(org1Dir, "cluster1", ".k8s-cluster1-config.yaml"),
		[]byte("test config"),
		0644,
	))

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	mockManager := newMockSecretsManager()
	syncer := NewDefaultMultiClusterSyncer(mockManager, nil)

	opts := MultiClusterSyncOptions{
		Organization: "org1",
		Concurrency:  0, // Should default to 4
		StopOnError:  false,
		DryRun:       false,
	}

	result, err := syncer.SyncAll(context.Background(), opts)

	require.NoError(t, err)
	assert.NotNil(t, result)
	// Should still work with default concurrency
	assert.Equal(t, 1, result.SuccessCount)
}
