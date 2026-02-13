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
	"strings"
	"sync"
)

// DefaultMultiClusterSyncer implements the MultiClusterSyncer interface.
// It provides methods for performing secrets operations across multiple clusters
// in parallel with configurable concurrency and error handling.
type DefaultMultiClusterSyncer struct {
	secretsManager SecretsManager
	logger         *slog.Logger
}

// NewDefaultMultiClusterSyncer creates a new DefaultMultiClusterSyncer.
//
// Parameters:
//   - secretsManager: The secrets manager to use for individual cluster operations
//   - logger: Logger for operation tracking
//
// Returns:
//   - *DefaultMultiClusterSyncer: A new multi-cluster syncer instance
func NewDefaultMultiClusterSyncer(
	secretsManager SecretsManager,
	logger *slog.Logger,
) *DefaultMultiClusterSyncer {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultMultiClusterSyncer{
		secretsManager: secretsManager,
		logger:         logger,
	}
}

// SyncAll syncs secrets for all clusters in the organization.
// It discovers clusters from the filesystem, processes them in parallel
// with configurable concurrency, and handles failures according to the options.
//
// The method continues processing remaining clusters on failure by default,
// unless StopOnError is set to true.
func (m *DefaultMultiClusterSyncer) SyncAll(ctx context.Context, opts MultiClusterSyncOptions) (*MultiClusterSyncResult, error) {
	m.logger.Info("Starting multi-cluster secrets sync",
		"organization", opts.Organization,
		"concurrency", opts.Concurrency,
		"stop_on_error", opts.StopOnError,
		"dry_run", opts.DryRun)

	// Discover clusters
	clusters, err := m.discoverClusters(opts.Organization)
	if err != nil {
		return nil, fmt.Errorf("failed to discover clusters: %w", err)
	}

	if len(clusters) == 0 {
		m.logger.Warn("No clusters found", "organization", opts.Organization)
		return &MultiClusterSyncResult{
			Results:      make(map[string]*SyncResult),
			Failures:     make(map[string]error),
			SuccessCount: 0,
			FailureCount: 0,
		}, nil
	}

	m.logger.Info("Discovered clusters", "count", len(clusters), "clusters", clusters)

	// Set default concurrency if not specified
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 4 // Default to 4 parallel operations
	}

	// Initialize result
	result := &MultiClusterSyncResult{
		Results:      make(map[string]*SyncResult),
		Failures:     make(map[string]error),
		SuccessCount: 0,
		FailureCount: 0,
	}

	// Create channels for work distribution
	clusterChan := make(chan string, len(clusters))
	resultChan := make(chan clusterSyncResult, len(clusters))

	// Populate cluster channel
	for _, cluster := range clusters {
		clusterChan <- cluster
	}
	close(clusterChan)

	// Create worker pool
	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	var stopOnce sync.Once

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			m.worker(ctx, workerID, clusterChan, resultChan, stopChan, opts)
		}(i)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for clusterResult := range resultChan {
		if clusterResult.err != nil {
			result.Failures[clusterResult.cluster] = clusterResult.err
			result.FailureCount++
			m.logger.Error("Cluster sync failed",
				"cluster", clusterResult.cluster,
				"error", clusterResult.err)

			// Stop on first error if requested
			if opts.StopOnError {
				m.logger.Warn("Stopping multi-cluster sync due to error", "cluster", clusterResult.cluster)
				stopOnce.Do(func() {
					close(stopChan)
				})
			}
		} else {
			result.Results[clusterResult.cluster] = clusterResult.result
			result.SuccessCount++
			m.logger.Info("Cluster sync completed",
				"cluster", clusterResult.cluster,
				"created", len(clusterResult.result.Created),
				"updated", len(clusterResult.result.Updated),
				"unchanged", len(clusterResult.result.Unchanged))
		}
	}

	m.logger.Info("Multi-cluster secrets sync completed",
		"total_clusters", len(clusters),
		"success_count", result.SuccessCount,
		"failure_count", result.FailureCount)

	return result, nil
}

// clusterSyncResult represents the result of syncing a single cluster.
type clusterSyncResult struct {
	cluster string
	result  *SyncResult
	err     error
}

// worker processes clusters from the channel and sends results.
func (m *DefaultMultiClusterSyncer) worker(
	ctx context.Context,
	workerID int,
	clusterChan <-chan string,
	resultChan chan<- clusterSyncResult,
	stopChan <-chan struct{},
	opts MultiClusterSyncOptions,
) {
	for {
		select {
		case <-stopChan:
			// Stop processing if stop signal received
			m.logger.Debug("Worker stopping due to stop signal", "worker_id", workerID)
			return
		case cluster, ok := <-clusterChan:
			if !ok {
				// Channel closed, no more work
				return
			}

			m.logger.Debug("Worker processing cluster", "worker_id", workerID, "cluster", cluster)

			// Sync the cluster
			syncOpts := SyncOptions{
				Cluster: cluster,
				DryRun:  opts.DryRun,
				Force:   false, // Don't force updates in multi-cluster mode
			}

			result, err := m.secretsManager.SyncSecrets(ctx, syncOpts)

			// Send result
			resultChan <- clusterSyncResult{
				cluster: cluster,
				result:  result,
				err:     err,
			}
		}
	}
}

// discoverClusters discovers all clusters in the organization.
// It scans the ~/.config/opencenter/clusters directory for cluster configurations.
//
// If organization is specified, only clusters in that organization are returned.
// If organization is empty, all clusters are returned.
func (m *DefaultMultiClusterSyncer) discoverClusters(organization string) ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	clustersDir := filepath.Join(homeDir, ".config", "opencenter", "clusters")

	// Check if clusters directory exists
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return []string{}, nil // Return empty list if directory doesn't exist
	}

	var clusters []string

	// Walk the clusters directory
	err = filepath.Walk(clustersDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories with errors
		}

		// Look for cluster config files: .k8s-<cluster>-config.yaml
		if !info.IsDir() && strings.HasPrefix(info.Name(), ".k8s-") && strings.HasSuffix(info.Name(), "-config.yaml") {
			// Extract cluster name from filename
			// Format: .k8s-<cluster>-config.yaml
			clusterName := strings.TrimPrefix(info.Name(), ".k8s-")
			clusterName = strings.TrimSuffix(clusterName, "-config.yaml")

			// Get organization from path
			// Path format: ~/.config/opencenter/clusters/<org>/<cluster>/.k8s-<cluster>-config.yaml
			relPath, err := filepath.Rel(clustersDir, path)
			if err != nil {
				return nil
			}

			pathParts := strings.Split(relPath, string(filepath.Separator))
			if len(pathParts) < 2 {
				return nil // Skip if path structure is unexpected
			}

			clusterOrg := pathParts[0]

			// Filter by organization if specified
			if organization != "" && clusterOrg != organization {
				return nil // Skip clusters not in the specified organization
			}

			clusters = append(clusters, clusterName)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk clusters directory: %w", err)
	}

	return clusters, nil
}
