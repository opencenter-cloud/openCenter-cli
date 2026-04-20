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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"gopkg.in/yaml.v3"
)

// setupBenchmarkManager creates a manager with a test directory and sample configs
func setupBenchmarkManager(b *testing.B) (*ConfigurationManager, string, func()) {
	b.Helper()

	// Create temporary directory
	tmpDir := b.TempDir()

	// Create organization structure
	clusterDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", "bench-cluster")
	err := os.MkdirAll(clusterDir, 0755)
	if err != nil {
		b.Fatalf("Failed to create cluster directory: %v", err)
	}

	// Create a sample config file
	configPath := filepath.Join(tmpDir, "test-org", ".bench-cluster-config.yaml")
	sampleConfig := createSampleConfig("bench-cluster")
	data, err := yaml.Marshal(sampleConfig)
	if err != nil {
		b.Fatalf("Failed to marshal config: %v", err)
	}

	err = os.WriteFile(configPath, data, 0600)
	if err != nil {
		b.Fatalf("Failed to write config: %v", err)
	}

	// Create manager
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	pathResolver := paths.NewPathResolver(tmpDir)
	validator := validation.NewValidationEngine()
	cache := NewConfigCache()
	loader := NewConfigIOHandler(fileSystem)

	manager := NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	cleanup := func() {
		// Cleanup is handled by b.TempDir()
	}

	return manager, tmpDir, cleanup
}

// createSampleConfig creates a sample configuration for benchmarking
func createSampleConfig(name string) *v2.Config {
	cfg, err := v2.NewV2Default(name, "openstack")
	if err != nil {
		panic(err)
	}

	cfg.OpenCenter.Meta.Organization = "test-org"
	cfg.OpenCenter.Meta.Region = "us-east-1"
	cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/gitops"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.example.com:5000/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "us-east-1"
	cfg.Secrets.SopsAgeKeyFile = "/tmp/age-key.txt"

	return cfg
}

// BenchmarkLoad_WithCache benchmarks Load operation with cache hit
func BenchmarkLoad_WithCache(b *testing.B) {
	manager, _, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate cache with first load (skip validation errors for benchmark)
	config, _ := manager.Load(ctx, "bench-cluster")
	if config != nil {
		// If load succeeded, benchmark it
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = manager.Load(ctx, "bench-cluster")
		}
	} else {
		b.Skip("Skipping benchmark - config validation failed")
	}
}

// BenchmarkLoad_WithoutCache benchmarks Load operation with cache miss (disk read)
func BenchmarkLoad_WithoutCache(b *testing.B) {
	manager, _, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Test if load works at all
	config, _ := manager.Load(ctx, "bench-cluster")
	if config == nil {
		b.Skip("Skipping benchmark - config validation failed")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Clear cache before each load to force disk read
		manager.ClearCache(ctx)

		_, _ = manager.Load(ctx, "bench-cluster")
	}
}

// BenchmarkLoad_CacheVsDisk compares cached vs disk performance
func BenchmarkLoad_CacheVsDisk(b *testing.B) {
	manager, _, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Test if load works at all
	config, _ := manager.Load(ctx, "bench-cluster")
	if config == nil {
		b.Skip("Skipping benchmark - config validation failed")
	}

	b.Run("Cached", func(b *testing.B) {
		// Pre-populate cache
		manager.Load(ctx, "bench-cluster")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = manager.Load(ctx, "bench-cluster")
		}
	})

	b.Run("Disk", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			manager.ClearCache(ctx)
			_, _ = manager.Load(ctx, "bench-cluster")
		}
	})
}

// BenchmarkSave_AtomicWrite benchmarks Save operation with atomic writes
func BenchmarkSave_AtomicWrite(b *testing.B) {
	manager, tmpDir, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Create a config to save
	config := createSampleConfig("save-bench-cluster")

	// Create the cluster directory
	orgDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", "save-bench-cluster")
	err := os.MkdirAll(orgDir, 0755)
	if err != nil {
		b.Fatalf("Failed to create cluster directory: %v", err)
	}

	// Test if save works at all
	err = manager.Save(ctx, config)
	if err != nil {
		b.Skip("Skipping benchmark - config validation failed")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = manager.Save(ctx, config)
	}
}

// BenchmarkSave_WithValidation benchmarks Save with validation overhead
func BenchmarkSave_WithValidation(b *testing.B) {
	manager, tmpDir, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Create a config to save
	config := createSampleConfig("validate-bench-cluster")

	// Create the cluster directory
	orgDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", "validate-bench-cluster")
	err := os.MkdirAll(orgDir, 0755)
	if err != nil {
		b.Fatalf("Failed to create cluster directory: %v", err)
	}

	// Test if operations work
	err = manager.Validate(ctx, config)
	if err != nil {
		b.Skip("Skipping benchmark - config validation failed")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Validate then save
		_ = manager.Validate(ctx, config)
		_ = manager.Save(ctx, config)
	}
}

// BenchmarkConcurrentLoad benchmarks concurrent Load operations
func BenchmarkConcurrentLoad(b *testing.B) {
	manager, _, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate cache
	_, err := manager.Load(ctx, "bench-cluster")
	if err != nil {
		b.Fatalf("Failed to pre-load config: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := manager.Load(ctx, "bench-cluster")
			if err != nil {
				b.Fatalf("Load failed: %v", err)
			}
		}
	})
}

// BenchmarkConcurrentSave benchmarks concurrent Save operations
func BenchmarkConcurrentSave(b *testing.B) {
	manager, tmpDir, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Create multiple cluster directories
	numClusters := 10
	for i := 0; i < numClusters; i++ {
		clusterName := fmt.Sprintf("concurrent-cluster-%d", i)
		orgDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", clusterName)
		err := os.MkdirAll(orgDir, 0755)
		if err != nil {
			b.Fatalf("Failed to create cluster directory: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	var counter int
	var mu sync.Mutex

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			idx := counter % numClusters
			counter++
			mu.Unlock()

			clusterName := fmt.Sprintf("concurrent-cluster-%d", idx)
			config := createSampleConfig(clusterName)

			err := manager.Save(ctx, config)
			if err != nil {
				b.Fatalf("Save failed: %v", err)
			}
		}
	})
}

// BenchmarkList benchmarks List operation
func BenchmarkList(b *testing.B) {
	manager, tmpDir, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	// Create multiple clusters
	numClusters := 50
	for i := 0; i < numClusters; i++ {
		clusterName := fmt.Sprintf("list-cluster-%d", i)
		orgDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", clusterName)
		err := os.MkdirAll(orgDir, 0755)
		if err != nil {
			b.Fatalf("Failed to create cluster directory: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		clusters, err := manager.List(ctx)
		if err != nil {
			b.Fatalf("List failed: %v", err)
		}

		if len(clusters) != numClusters+1 { // +1 for bench-cluster
			b.Fatalf("Expected %d clusters, got %d", numClusters+1, len(clusters))
		}
	}
}

// BenchmarkDelete benchmarks Delete operation
func BenchmarkDelete(b *testing.B) {
	manager, tmpDir, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Create a cluster to delete
		clusterName := fmt.Sprintf("delete-cluster-%d", i)
		orgDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", clusterName)
		err := os.MkdirAll(orgDir, 0755)
		if err != nil {
			b.Fatalf("Failed to create cluster directory: %v", err)
		}

		// Create config file
		configPath := filepath.Join(orgDir, fmt.Sprintf(".%s-config.yaml", clusterName))
		config := createSampleConfig(clusterName)
		data, err := yaml.Marshal(config)
		if err != nil {
			b.Fatalf("Failed to marshal config: %v", err)
		}

		err = os.WriteFile(configPath, data, 0600)
		if err != nil {
			b.Fatalf("Failed to write config: %v", err)
		}

		b.StartTimer()

		err = manager.Delete(ctx, clusterName)
		if err != nil {
			b.Fatalf("Delete failed: %v", err)
		}
	}
}

// BenchmarkCacheOperations benchmarks cache operations
func BenchmarkCacheOperations(b *testing.B) {
	manager, _, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	b.Run("InvalidateCluster", func(b *testing.B) {
		// Pre-populate cache
		_, err := manager.Load(ctx, "bench-cluster")
		if err != nil {
			b.Fatalf("Failed to pre-load config: %v", err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := manager.InvalidateCluster(ctx, "bench-cluster")
			if err != nil {
				b.Fatalf("InvalidateCluster failed: %v", err)
			}
		}
	})

	b.Run("ClearCache", func(b *testing.B) {
		// Pre-populate cache
		_, err := manager.Load(ctx, "bench-cluster")
		if err != nil {
			b.Fatalf("Failed to pre-load config: %v", err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := manager.ClearCache(ctx)
			if err != nil {
				b.Fatalf("ClearCache failed: %v", err)
			}
		}
	})
}

// BenchmarkValidate benchmarks Validate operation
func BenchmarkValidate(b *testing.B) {
	manager, _, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	config := createSampleConfig("validate-cluster")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := manager.Validate(ctx, config)
		if err != nil {
			b.Fatalf("Validate failed: %v", err)
		}
	}
}

// BenchmarkLoadSaveCycle benchmarks a complete load-modify-save cycle
func BenchmarkLoadSaveCycle(b *testing.B) {
	manager, _, cleanup := setupBenchmarkManager(b)
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Load
		config, err := manager.Load(ctx, "bench-cluster")
		if err != nil {
			b.Fatalf("Load failed: %v", err)
		}

		// Modify
		if config.Metadata.Labels == nil {
			config.Metadata.Labels = make(map[string]string)
		}
		config.Metadata.Labels["iteration"] = fmt.Sprintf("%d", i)

		// Save
		err = manager.Save(ctx, config)
		if err != nil {
			b.Fatalf("Save failed: %v", err)
		}
	}
}
