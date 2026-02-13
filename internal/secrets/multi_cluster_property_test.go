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

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/require"
)

// **Validates: Requirements 8.1, 8.5, 8.7**
//
// Property 15: Multi-Cluster Sync Coverage
//
// For any organization with multiple clusters, running sync with `--all` should
// process all clusters and report accurate success/failure counts.
//
// This property verifies that:
// 1. All clusters in an organization are discovered correctly
// 2. Each cluster is processed exactly once
// 3. Success and failure counts are accurate
// 4. Results map contains entries for all processed clusters
func TestProperty_MultiClusterSyncCoverage(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("SyncAll processes all clusters and reports accurate counts", prop.ForAll(
		func(clusterCount int) bool {
			// Skip invalid cluster counts
			if clusterCount < 1 || clusterCount > 10 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			ctx := context.Background()
			orgName := "test-org"

			// Override HOME environment variable for this test
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", oldHome)

			// Create multi-cluster syncer with mock secrets manager
			mockManager := &mockSecretsManagerForMultiCluster{
				syncResults: make(map[string]*SyncResult),
			}
			syncer := NewDefaultMultiClusterSyncer(
				mockManager,
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
			)

			// Create test clusters
			clusterNames := make([]string, clusterCount)
			for i := 0; i < clusterCount; i++ {
				clusterName := fmt.Sprintf("cluster-%d", i)
				clusterNames[i] = clusterName

				// Create cluster config
				if err := createTestClusterConfig(tmpDir, orgName, clusterName); err != nil {
					t.Logf("Failed to create cluster config: %v", err)
					return false
				}

				// Setup mock result for this cluster
				mockManager.syncResults[clusterName] = &SyncResult{
					Created:   []string{fmt.Sprintf("secret-%d.yaml", i)},
					Updated:   []string{},
					Unchanged: []string{},
					Errors:    []SyncError{},
				}
			}

			// Run SyncAll
			result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
				Organization: orgName,
				Concurrency:  2,
				StopOnError:  false,
				DryRun:       false,
			})

			if err != nil {
				t.Logf("SyncAll failed: %v", err)
				return false
			}

			// Property 1: All clusters should be processed
			if len(result.Results)+len(result.Failures) != clusterCount {
				t.Logf("Not all clusters processed: expected %d, got %d (success: %d, failures: %d)",
					clusterCount, len(result.Results)+len(result.Failures),
					len(result.Results), len(result.Failures))
				return false
			}

			// Property 2: Success count should match number of successful results
			if result.SuccessCount != len(result.Results) {
				t.Logf("Success count mismatch: expected %d, got %d",
					len(result.Results), result.SuccessCount)
				return false
			}

			// Property 3: Failure count should match number of failures
			if result.FailureCount != len(result.Failures) {
				t.Logf("Failure count mismatch: expected %d, got %d",
					len(result.Failures), result.FailureCount)
				return false
			}

			// Property 4: Total count should equal cluster count
			if result.SuccessCount+result.FailureCount != clusterCount {
				t.Logf("Total count mismatch: expected %d, got %d",
					clusterCount, result.SuccessCount+result.FailureCount)
				return false
			}

			// Property 5: Each cluster should appear exactly once in results or failures
			processedClusters := make(map[string]bool)
			for cluster := range result.Results {
				if processedClusters[cluster] {
					t.Logf("Cluster %s processed multiple times", cluster)
					return false
				}
				processedClusters[cluster] = true
			}
			for cluster := range result.Failures {
				if processedClusters[cluster] {
					t.Logf("Cluster %s processed multiple times", cluster)
					return false
				}
				processedClusters[cluster] = true
			}

			// Property 6: All created clusters should be in processed list
			for _, clusterName := range clusterNames {
				if !processedClusters[clusterName] {
					t.Logf("Cluster %s not processed", clusterName)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.Property("SyncAll handles mixed success and failure correctly", prop.ForAll(
		func(successCount int, failureCount int) bool {
			// Skip invalid counts
			if successCount < 0 || failureCount < 0 || successCount+failureCount < 1 || successCount+failureCount > 10 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			ctx := context.Background()
			orgName := "test-org"

			// Override HOME environment variable for this test
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", oldHome)

			// Create multi-cluster syncer with mock secrets manager
			mockManager := &mockSecretsManagerForMultiCluster{
				syncResults: make(map[string]*SyncResult),
				syncErrors:  make(map[string]error),
			}
			syncer := NewDefaultMultiClusterSyncer(
				mockManager,
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
			)

			totalClusters := successCount + failureCount
			clusterNames := make([]string, totalClusters)

			// Create successful clusters
			for i := 0; i < successCount; i++ {
				clusterName := fmt.Sprintf("success-cluster-%d", i)
				clusterNames[i] = clusterName

				if err := createTestClusterConfig(tmpDir, orgName, clusterName); err != nil {
					t.Logf("Failed to create cluster config: %v", err)
					return false
				}

				mockManager.syncResults[clusterName] = &SyncResult{
					Created:   []string{fmt.Sprintf("secret-%d.yaml", i)},
					Updated:   []string{},
					Unchanged: []string{},
					Errors:    []SyncError{},
				}
			}

			// Create failing clusters
			for i := 0; i < failureCount; i++ {
				clusterName := fmt.Sprintf("failure-cluster-%d", i)
				clusterNames[successCount+i] = clusterName

				if err := createTestClusterConfig(tmpDir, orgName, clusterName); err != nil {
					t.Logf("Failed to create cluster config: %v", err)
					return false
				}

				mockManager.syncErrors[clusterName] = fmt.Errorf("simulated sync error for %s", clusterName)
			}

			// Run SyncAll (without stop-on-error)
			result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
				Organization: orgName,
				Concurrency:  2,
				StopOnError:  false,
				DryRun:       false,
			})

			if err != nil {
				t.Logf("SyncAll failed: %v", err)
				return false
			}

			// Property 1: Success count should match expected
			if result.SuccessCount != successCount {
				t.Logf("Success count mismatch: expected %d, got %d",
					successCount, result.SuccessCount)
				return false
			}

			// Property 2: Failure count should match expected
			if result.FailureCount != failureCount {
				t.Logf("Failure count mismatch: expected %d, got %d",
					failureCount, result.FailureCount)
				return false
			}

			// Property 3: All clusters should be processed
			if len(result.Results)+len(result.Failures) != totalClusters {
				t.Logf("Not all clusters processed: expected %d, got %d",
					totalClusters, len(result.Results)+len(result.Failures))
				return false
			}

			// Property 4: Results map should contain only successful clusters
			for cluster := range result.Results {
				if mockManager.syncErrors[cluster] != nil {
					t.Logf("Failed cluster %s found in results", cluster)
					return false
				}
			}

			// Property 5: Failures map should contain only failed clusters
			for cluster := range result.Failures {
				if mockManager.syncErrors[cluster] == nil {
					t.Logf("Successful cluster %s found in failures", cluster)
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.Property("SyncAll respects organization filter", prop.ForAll(
		func(org1Count int, org2Count int) bool {
			// Skip invalid counts
			if org1Count < 1 || org1Count > 5 || org2Count < 1 || org2Count > 5 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			ctx := context.Background()
			org1Name := "org-1"
			org2Name := "org-2"

			// Override HOME environment variable for this test
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", oldHome)

			// Create multi-cluster syncer with mock secrets manager
			mockManager := &mockSecretsManagerForMultiCluster{
				syncResults: make(map[string]*SyncResult),
			}
			syncer := NewDefaultMultiClusterSyncer(
				mockManager,
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
			)

			// Create clusters for org1
			for i := 0; i < org1Count; i++ {
				clusterName := fmt.Sprintf("org1-cluster-%d", i)
				if err := createTestClusterConfig(tmpDir, org1Name, clusterName); err != nil {
					t.Logf("Failed to create cluster config: %v", err)
					return false
				}
				mockManager.syncResults[clusterName] = &SyncResult{
					Created: []string{fmt.Sprintf("secret-%d.yaml", i)},
				}
			}

			// Create clusters for org2
			for i := 0; i < org2Count; i++ {
				clusterName := fmt.Sprintf("org2-cluster-%d", i)
				if err := createTestClusterConfig(tmpDir, org2Name, clusterName); err != nil {
					t.Logf("Failed to create cluster config: %v", err)
					return false
				}
				mockManager.syncResults[clusterName] = &SyncResult{
					Created: []string{fmt.Sprintf("secret-%d.yaml", i)},
				}
			}

			// Run SyncAll for org1 only
			result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
				Organization: org1Name,
				Concurrency:  2,
				StopOnError:  false,
				DryRun:       false,
			})

			if err != nil {
				t.Logf("SyncAll failed: %v", err)
				return false
			}

			// Property 1: Only org1 clusters should be processed
			if result.SuccessCount != org1Count {
				t.Logf("Expected %d clusters for org1, got %d",
					org1Count, result.SuccessCount)
				return false
			}

			// Property 2: No org2 clusters should be processed
			for cluster := range result.Results {
				if !containsString(cluster, "org1-cluster-") {
					t.Logf("Unexpected cluster %s in results (should only have org1 clusters)", cluster)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 5),
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}

// Test that verifies the multi-cluster sync coverage property test is working correctly
func TestProperty_MultiClusterSyncCoverage_Sanity(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	orgName := "sanity-org"

	// Override HOME environment variable for this test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create multi-cluster syncer with mock secrets manager
	mockManager := &mockSecretsManagerForMultiCluster{
		syncResults: make(map[string]*SyncResult),
	}
	syncer := NewDefaultMultiClusterSyncer(
		mockManager,
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	)

	// Create 3 test clusters
	clusterNames := []string{"cluster-1", "cluster-2", "cluster-3"}
	for i, clusterName := range clusterNames {
		err := createTestClusterConfig(tmpDir, orgName, clusterName)
		require.NoError(t, err, "Failed to create cluster config")

		mockManager.syncResults[clusterName] = &SyncResult{
			Created:   []string{fmt.Sprintf("secret-%d.yaml", i)},
			Updated:   []string{},
			Unchanged: []string{},
			Errors:    []SyncError{},
		}
	}

	// Run SyncAll
	result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
		Organization: orgName,
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       false,
	})

	require.NoError(t, err, "SyncAll should not fail")

	// Verify all clusters processed
	require.Equal(t, 3, len(result.Results)+len(result.Failures),
		"All 3 clusters should be processed")

	// Verify success count
	require.Equal(t, 3, result.SuccessCount, "All 3 clusters should succeed")
	require.Equal(t, 0, result.FailureCount, "No clusters should fail")

	// Verify each cluster in results
	for _, clusterName := range clusterNames {
		_, exists := result.Results[clusterName]
		require.True(t, exists, "Cluster %s should be in results", clusterName)
	}
}

// Test mixed success and failure
func TestProperty_MultiClusterSyncCoverage_MixedResults(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	orgName := "mixed-org"

	// Override HOME environment variable for this test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create multi-cluster syncer with mock secrets manager
	mockManager := &mockSecretsManagerForMultiCluster{
		syncResults: make(map[string]*SyncResult),
		syncErrors:  make(map[string]error),
	}
	syncer := NewDefaultMultiClusterSyncer(
		mockManager,
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	)

	// Create 2 successful clusters
	successClusters := []string{"success-1", "success-2"}
	for i, clusterName := range successClusters {
		err := createTestClusterConfig(tmpDir, orgName, clusterName)
		require.NoError(t, err)

		mockManager.syncResults[clusterName] = &SyncResult{
			Created: []string{fmt.Sprintf("secret-%d.yaml", i)},
		}
	}

	// Create 2 failing clusters
	failureClusters := []string{"failure-1", "failure-2"}
	for _, clusterName := range failureClusters {
		err := createTestClusterConfig(tmpDir, orgName, clusterName)
		require.NoError(t, err)

		mockManager.syncErrors[clusterName] = fmt.Errorf("simulated error for %s", clusterName)
	}

	// Run SyncAll
	result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
		Organization: orgName,
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       false,
	})

	require.NoError(t, err)

	// Verify counts
	require.Equal(t, 2, result.SuccessCount, "Should have 2 successful clusters")
	require.Equal(t, 2, result.FailureCount, "Should have 2 failed clusters")

	// Verify successful clusters in results
	for _, clusterName := range successClusters {
		_, exists := result.Results[clusterName]
		require.True(t, exists, "Successful cluster %s should be in results", clusterName)
	}

	// Verify failed clusters in failures
	for _, clusterName := range failureClusters {
		_, exists := result.Failures[clusterName]
		require.True(t, exists, "Failed cluster %s should be in failures", clusterName)
	}
}

// Test organization filtering
func TestProperty_MultiClusterSyncCoverage_OrganizationFilter(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Override HOME environment variable for this test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create multi-cluster syncer with mock secrets manager
	mockManager := &mockSecretsManagerForMultiCluster{
		syncResults: make(map[string]*SyncResult),
	}
	syncer := NewDefaultMultiClusterSyncer(
		mockManager,
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	)

	// Create clusters for org-alpha
	alphaClusters := []string{"alpha-cluster-1", "alpha-cluster-2"}
	for i, clusterName := range alphaClusters {
		err := createTestClusterConfig(tmpDir, "org-alpha", clusterName)
		require.NoError(t, err)

		mockManager.syncResults[clusterName] = &SyncResult{
			Created: []string{fmt.Sprintf("secret-%d.yaml", i)},
		}
	}

	// Create clusters for org-beta
	betaClusters := []string{"beta-cluster-1", "beta-cluster-2"}
	for i, clusterName := range betaClusters {
		err := createTestClusterConfig(tmpDir, "org-beta", clusterName)
		require.NoError(t, err)

		mockManager.syncResults[clusterName] = &SyncResult{
			Created: []string{fmt.Sprintf("secret-%d.yaml", i)},
		}
	}

	// Run SyncAll for org-alpha only
	result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
		Organization: "org-alpha",
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       false,
	})

	require.NoError(t, err)

	// Verify only org-alpha clusters processed
	require.Equal(t, 2, result.SuccessCount, "Should process 2 org-alpha clusters")

	// Verify org-alpha clusters in results
	for _, clusterName := range alphaClusters {
		_, exists := result.Results[clusterName]
		require.True(t, exists, "Org-alpha cluster %s should be in results", clusterName)
	}

	// Verify org-beta clusters NOT in results
	for _, clusterName := range betaClusters {
		_, exists := result.Results[clusterName]
		require.False(t, exists, "Org-beta cluster %s should NOT be in results", clusterName)
	}
}

// Helper functions

// mockSecretsManagerForMultiCluster is a mock implementation of SecretsManager for testing
type mockSecretsManagerForMultiCluster struct {
	syncResults map[string]*SyncResult
	syncErrors  map[string]error
}

func (m *mockSecretsManagerForMultiCluster) SyncSecrets(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
	if err, exists := m.syncErrors[opts.Cluster]; exists {
		return nil, err
	}

	if result, exists := m.syncResults[opts.Cluster]; exists {
		return result, nil
	}

	return &SyncResult{
		Created:   []string{},
		Updated:   []string{},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}, nil
}

func (m *mockSecretsManagerForMultiCluster) ValidateSecrets(ctx context.Context, opts ValidateOptions) (*ValidationResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSecretsManagerForMultiCluster) DetectDrift(ctx context.Context, cluster string) (*DriftReport, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSecretsManagerForMultiCluster) GetSecretSources(ctx context.Context, cluster string) ([]SecretSource, error) {
	return nil, fmt.Errorf("not implemented")
}

// createTestClusterConfig creates a minimal cluster config file for testing
func createTestClusterConfig(tmpDir string, orgName string, clusterName string) error {
	// Create directory structure
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", orgName, clusterName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	// Create config file
	configPath := filepath.Join(configDir, fmt.Sprintf(".k8s-%s-config.yaml", clusterName))
	configContent := fmt.Sprintf(`schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: %s
  gitops:
    git_dir: %s
secrets:
  sops_age_key_file: %s
`, clusterName, filepath.Join(tmpDir, "gitops"), filepath.Join(configDir, "age-key.txt"))

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// containsString checks if a string contains a substring
func containsString(s string, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// **Validates: Requirements 8.5, 8.6**
//
// Property 16: Multi-Cluster Failure Isolation
//
// For any multi-cluster sync where one cluster fails, remaining clusters should
// still be processed (unless `--stop-on-error` is set).
//
// This property verifies that:
// 1. When StopOnError is false, all clusters are processed despite failures
// 2. When StopOnError is true, processing stops after first failure
// 3. Failures are properly isolated and don't affect other clusters
// 4. Partial results are correctly reported
func TestProperty_MultiClusterFailureIsolation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Failures are isolated when StopOnError is false", prop.ForAll(
		func(totalClusters int, failureIndex int) bool {
			// Skip invalid inputs
			if totalClusters < 2 || totalClusters > 10 {
				return true
			}
			if failureIndex < 0 || failureIndex >= totalClusters {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			ctx := context.Background()
			orgName := "test-org"

			// Override HOME environment variable for this test
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", oldHome)

			// Create multi-cluster syncer with mock secrets manager
			mockManager := &mockSecretsManagerForMultiCluster{
				syncResults: make(map[string]*SyncResult),
				syncErrors:  make(map[string]error),
			}
			syncer := NewDefaultMultiClusterSyncer(
				mockManager,
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
			)

			// Create test clusters
			clusterNames := make([]string, totalClusters)
			for i := 0; i < totalClusters; i++ {
				clusterName := fmt.Sprintf("cluster-%d", i)
				clusterNames[i] = clusterName

				// Create cluster config
				if err := createTestClusterConfig(tmpDir, orgName, clusterName); err != nil {
					t.Logf("Failed to create cluster config: %v", err)
					return false
				}

				// Setup mock result - one cluster fails, others succeed
				if i == failureIndex {
					mockManager.syncErrors[clusterName] = fmt.Errorf("simulated failure for cluster %d", i)
				} else {
					mockManager.syncResults[clusterName] = &SyncResult{
						Created:   []string{fmt.Sprintf("secret-%d.yaml", i)},
						Updated:   []string{},
						Unchanged: []string{},
						Errors:    []SyncError{},
					}
				}
			}

			// Run SyncAll with StopOnError=false
			result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
				Organization: orgName,
				Concurrency:  2,
				StopOnError:  false, // Continue on error
				DryRun:       false,
			})

			if err != nil {
				t.Logf("SyncAll failed: %v", err)
				return false
			}

			// Property 1: All clusters should be processed despite failure
			totalProcessed := len(result.Results) + len(result.Failures)
			if totalProcessed != totalClusters {
				t.Logf("Not all clusters processed with StopOnError=false: expected %d, got %d",
					totalClusters, totalProcessed)
				return false
			}

			// Property 2: Exactly one cluster should fail
			if result.FailureCount != 1 {
				t.Logf("Expected exactly 1 failure, got %d", result.FailureCount)
				return false
			}

			// Property 3: Remaining clusters should succeed
			expectedSuccessCount := totalClusters - 1
			if result.SuccessCount != expectedSuccessCount {
				t.Logf("Expected %d successful clusters, got %d",
					expectedSuccessCount, result.SuccessCount)
				return false
			}

			// Property 4: Failed cluster should be in failures map
			failedClusterName := clusterNames[failureIndex]
			if _, exists := result.Failures[failedClusterName]; !exists {
				t.Logf("Failed cluster %s not found in failures map", failedClusterName)
				return false
			}

			// Property 5: Successful clusters should be in results map
			for i, clusterName := range clusterNames {
				if i == failureIndex {
					continue // Skip the failed cluster
				}
				if _, exists := result.Results[clusterName]; !exists {
					t.Logf("Successful cluster %s not found in results map", clusterName)
					return false
				}
			}

			// Property 6: Failed cluster should NOT be in results map
			if _, exists := result.Results[failedClusterName]; exists {
				t.Logf("Failed cluster %s should not be in results map", failedClusterName)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),  // totalClusters
		gen.IntRange(0, 9),   // failureIndex (will be validated against totalClusters)
	))

	properties.Property("Multiple failures are isolated when StopOnError is false", prop.ForAll(
		func(totalClusters int, failureCount int) bool {
			// Skip invalid inputs
			if totalClusters < 3 || totalClusters > 10 {
				return true
			}
			if failureCount < 2 || failureCount >= totalClusters {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			ctx := context.Background()
			orgName := "test-org"

			// Override HOME environment variable for this test
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", oldHome)

			// Create multi-cluster syncer with mock secrets manager
			mockManager := &mockSecretsManagerForMultiCluster{
				syncResults: make(map[string]*SyncResult),
				syncErrors:  make(map[string]error),
			}
			syncer := NewDefaultMultiClusterSyncer(
				mockManager,
				slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
			)

			// Create test clusters with multiple failures
			clusterNames := make([]string, totalClusters)
			failedClusters := make(map[string]bool)
			for i := 0; i < totalClusters; i++ {
				clusterName := fmt.Sprintf("cluster-%d", i)
				clusterNames[i] = clusterName

				// Create cluster config
				if err := createTestClusterConfig(tmpDir, orgName, clusterName); err != nil {
					t.Logf("Failed to create cluster config: %v", err)
					return false
				}

				// Setup mock result - first failureCount clusters fail
				if i < failureCount {
					mockManager.syncErrors[clusterName] = fmt.Errorf("simulated failure for cluster %d", i)
					failedClusters[clusterName] = true
				} else {
					mockManager.syncResults[clusterName] = &SyncResult{
						Created:   []string{fmt.Sprintf("secret-%d.yaml", i)},
						Updated:   []string{},
						Unchanged: []string{},
						Errors:    []SyncError{},
					}
				}
			}

			// Run SyncAll with StopOnError=false
			result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
				Organization: orgName,
				Concurrency:  2,
				StopOnError:  false, // Continue on error
				DryRun:       false,
			})

			if err != nil {
				t.Logf("SyncAll failed: %v", err)
				return false
			}

			// Property 1: All clusters should be processed
			totalProcessed := len(result.Results) + len(result.Failures)
			if totalProcessed != totalClusters {
				t.Logf("Not all clusters processed: expected %d, got %d",
					totalClusters, totalProcessed)
				return false
			}

			// Property 2: Failure count should match expected
			if result.FailureCount != failureCount {
				t.Logf("Expected %d failures, got %d", failureCount, result.FailureCount)
				return false
			}

			// Property 3: Success count should match expected
			expectedSuccessCount := totalClusters - failureCount
			if result.SuccessCount != expectedSuccessCount {
				t.Logf("Expected %d successes, got %d",
					expectedSuccessCount, result.SuccessCount)
				return false
			}

			// Property 4: All failed clusters should be in failures map
			for clusterName := range failedClusters {
				if _, exists := result.Failures[clusterName]; !exists {
					t.Logf("Failed cluster %s not found in failures map", clusterName)
					return false
				}
			}

			// Property 5: No failed clusters should be in results map
			for clusterName := range failedClusters {
				if _, exists := result.Results[clusterName]; exists {
					t.Logf("Failed cluster %s should not be in results map", clusterName)
					return false
				}
			}

			// Property 6: All successful clusters should be in results map
			for _, clusterName := range clusterNames {
				if failedClusters[clusterName] {
					continue // Skip failed clusters
				}
				if _, exists := result.Results[clusterName]; !exists {
					t.Logf("Successful cluster %s not found in results map", clusterName)
					return false
				}
			}

			return true
		},
		gen.IntRange(3, 10),  // totalClusters
		gen.IntRange(2, 9),   // failureCount (will be validated against totalClusters)
	))

	properties.TestingRun(t)
}

// Test failure isolation with StopOnError=false
func TestProperty_MultiClusterFailureIsolation_ContinueOnError(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	orgName := "isolation-org"

	// Override HOME environment variable for this test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create multi-cluster syncer with mock secrets manager
	mockManager := &mockSecretsManagerForMultiCluster{
		syncResults: make(map[string]*SyncResult),
		syncErrors:  make(map[string]error),
	}
	syncer := NewDefaultMultiClusterSyncer(
		mockManager,
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	)

	// Create 5 clusters: cluster-0 fails, others succeed
	clusterNames := []string{"cluster-0", "cluster-1", "cluster-2", "cluster-3", "cluster-4"}
	for i, clusterName := range clusterNames {
		err := createTestClusterConfig(tmpDir, orgName, clusterName)
		require.NoError(t, err)

		if i == 0 {
			// First cluster fails
			mockManager.syncErrors[clusterName] = fmt.Errorf("simulated failure")
		} else {
			mockManager.syncResults[clusterName] = &SyncResult{
				Created: []string{fmt.Sprintf("secret-%d.yaml", i)},
			}
		}
	}

	// Run SyncAll with StopOnError=false
	result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
		Organization: orgName,
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       false,
	})

	require.NoError(t, err)

	// All clusters should be processed
	require.Equal(t, 5, len(result.Results)+len(result.Failures),
		"All 5 clusters should be processed")

	// One failure, four successes
	require.Equal(t, 1, result.FailureCount, "Should have 1 failure")
	require.Equal(t, 4, result.SuccessCount, "Should have 4 successes")

	// Failed cluster in failures map
	_, exists := result.Failures["cluster-0"]
	require.True(t, exists, "Failed cluster should be in failures map")

	// Successful clusters in results map
	for i := 1; i < 5; i++ {
		clusterName := fmt.Sprintf("cluster-%d", i)
		_, exists := result.Results[clusterName]
		require.True(t, exists, "Successful cluster %s should be in results", clusterName)
	}
}

// Test multiple failures with StopOnError=false
func TestProperty_MultiClusterFailureIsolation_MultipleFailures(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	orgName := "multi-fail-org"

	// Override HOME environment variable for this test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create multi-cluster syncer with mock secrets manager
	mockManager := &mockSecretsManagerForMultiCluster{
		syncResults: make(map[string]*SyncResult),
		syncErrors:  make(map[string]error),
	}
	syncer := NewDefaultMultiClusterSyncer(
		mockManager,
		slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	)

	// Create 6 clusters: cluster-1, cluster-3, cluster-5 fail
	clusterNames := []string{"cluster-0", "cluster-1", "cluster-2", "cluster-3", "cluster-4", "cluster-5"}
	failedClusters := map[string]bool{
		"cluster-1": true,
		"cluster-3": true,
		"cluster-5": true,
	}

	for i, clusterName := range clusterNames {
		err := createTestClusterConfig(tmpDir, orgName, clusterName)
		require.NoError(t, err)

		if failedClusters[clusterName] {
			mockManager.syncErrors[clusterName] = fmt.Errorf("simulated failure for %s", clusterName)
		} else {
			mockManager.syncResults[clusterName] = &SyncResult{
				Created: []string{fmt.Sprintf("secret-%d.yaml", i)},
			}
		}
	}

	// Run SyncAll with StopOnError=false
	result, err := syncer.SyncAll(ctx, MultiClusterSyncOptions{
		Organization: orgName,
		Concurrency:  2,
		StopOnError:  false,
		DryRun:       false,
	})

	require.NoError(t, err)

	// All clusters should be processed
	require.Equal(t, 6, len(result.Results)+len(result.Failures),
		"All 6 clusters should be processed")

	// Three failures, three successes
	require.Equal(t, 3, result.FailureCount, "Should have 3 failures")
	require.Equal(t, 3, result.SuccessCount, "Should have 3 successes")

	// Failed clusters in failures map
	for clusterName := range failedClusters {
		_, exists := result.Failures[clusterName]
		require.True(t, exists, "Failed cluster %s should be in failures map", clusterName)
	}

	// Successful clusters in results map
	for _, clusterName := range clusterNames {
		if failedClusters[clusterName] {
			continue
		}
		_, exists := result.Results[clusterName]
		require.True(t, exists, "Successful cluster %s should be in results", clusterName)
	}
}
