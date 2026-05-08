// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// TestConfigurationManager_Integration tests the full workflow
func TestConfigurationManager_Integration(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	orgName := "test-org"
	clusterName := "test-cluster"

	// Create a test configuration file
	// Create manager with test directory
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	ctx := context.Background()
	configPath := createSecureConfigTestCluster(t, tmpDir, orgName, clusterName)
	cfg, err := v2.NewV2Default(clusterName, "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.Meta.Organization = orgName
	cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/gitops"
	if err := loader.SaveToFile(ctx, configPath, cfg); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test Load
	t.Run("Load", func(t *testing.T) {
		config, err := manager.Load(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if config == nil {
			t.Fatal("Config is nil")
		}

		if config.ClusterName() != clusterName {
			t.Errorf("Expected cluster name %s, got %s", clusterName, config.ClusterName())
		}
	})

	// Test cache hit
	t.Run("CacheHit", func(t *testing.T) {
		// Load again - should hit cache
		config, err := manager.Load(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to load config from cache: %v", err)
		}

		if config == nil {
			t.Fatal("Config is nil")
		}
	})

	// Test List
	t.Run("List", func(t *testing.T) {
		clusters, err := manager.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list clusters: %v", err)
		}

		if len(clusters) != 1 {
			t.Errorf("Expected 1 cluster, got %d", len(clusters))
		}

		expectedClusterName := orgName + "/" + clusterName
		if len(clusters) > 0 && clusters[0] != expectedClusterName {
			t.Errorf("Expected cluster %s, got %s", expectedClusterName, clusters[0])
		}
	})

	// Test InvalidateCluster
	t.Run("InvalidateCluster", func(t *testing.T) {
		err := manager.InvalidateCluster(ctx, clusterName)
		if err != nil {
			t.Errorf("InvalidateCluster failed: %v", err)
		}

		// Verify cache was invalidated by checking cache size
		if cache.Size() != 0 {
			t.Error("Cache should be empty after invalidation")
		}
	})

	// Test ClearCache
	t.Run("ClearCache", func(t *testing.T) {
		// Load to populate cache
		_, err := manager.Load(ctx, clusterName)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Clear cache
		err = manager.ClearCache(ctx)
		if err != nil {
			t.Errorf("ClearCache failed: %v", err)
		}

		// Verify cache is empty
		if cache.Size() != 0 {
			t.Error("Cache should be empty after clear")
		}
	})
}
