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
	"os"
	"path/filepath"
	"testing"

	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/config/strategies"
)

// BenchmarkConfigManager_Load benchmarks the Load operation
func BenchmarkConfigManager_Load(b *testing.B) {
	// Create a temporary directory for test files
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "benchmark-config.yaml")

	// Create a realistic test configuration
	configContent := `schema_version: "1.0"
cluster:
  name: benchmark-cluster
  fqdn: benchmark.example.com
opencenter:
  infrastructure:
    provider: openstack
    openstack:
      auth_url: https://auth.example.com:5000/v3
      project_name: benchmark-project
      domain_name: default
  meta:
    region: us-east-1
    env: production
    organization: benchmark-org
  cluster:
    cluster_name: benchmark-cluster
    base_domain: k8s.example.com
    kubernetes_version: "1.28.0"
  networking:
    pod_cidr: 10.244.0.0/16
    service_cidr: 10.96.0.0/12
  services:
    cert-manager:
      enabled: true
    gateway:
      enabled: true
    harbor:
      enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		b.Fatalf("Failed to write test config: %v", err)
	}

	// Create ConfigManager and register strategies
	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())
	manager.RegisterStrategy(strategies.NewV2Strategy())
	manager.RegisterStrategy(strategies.NewLegacyStrategy())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.Load(configPath, LoadOptions{
			AutoMigrate: false,
			Validate:    false,
			SkipCache:   true, // Skip cache to measure actual load time
		})
		if err != nil {
			b.Fatalf("Load failed: %v", err)
		}
	}
}

// BenchmarkConfigManager_LoadCached benchmarks cached Load operations
func BenchmarkConfigManager_LoadCached(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "cached-config.yaml")

	configContent := `schema_version: "1.0"
cluster:
  name: cached-cluster
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		b.Fatalf("Failed to write test config: %v", err)
	}

	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())

	// Pre-load to populate cache
	_, err := manager.Load(configPath, LoadOptions{})
	if err != nil {
		b.Fatalf("Initial load failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.Load(configPath, LoadOptions{
			SkipCache: false, // Use cache
		})
		if err != nil {
			b.Fatalf("Cached load failed: %v", err)
		}
	}
}

// BenchmarkConfigManager_Save benchmarks the Save operation
func BenchmarkConfigManager_Save(b *testing.B) {
	tmpDir := b.TempDir()
	manager := NewConfigManager()

	// Create a realistic config
	cfg := internalconfig.NewDefault("benchmark-cluster")
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Meta.Region = "us-east-1"
	cfg.OpenCenter.Meta.Env = "production"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use a valid filename format
		configPath := filepath.Join(tmpDir, "save-benchmark.yaml")
		err := manager.Save(configPath, &cfg)
		if err != nil {
			b.Fatalf("Save failed: %v", err)
		}
	}
}

// BenchmarkConfigManager_DetectStrategy benchmarks version detection
func BenchmarkConfigManager_DetectStrategy(b *testing.B) {
	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())
	manager.RegisterStrategy(strategies.NewV2Strategy())
	manager.RegisterStrategy(strategies.NewLegacyStrategy())

	testData := []byte(`schema_version: "1.0"
cluster:
  name: test-cluster
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.detectStrategy(testData)
		if err != nil {
			b.Fatalf("detectStrategy failed: %v", err)
		}
	}
}

// BenchmarkConfigManager_ExtractClusterName benchmarks cluster name extraction
func BenchmarkConfigManager_ExtractClusterName(b *testing.B) {
	manager := NewConfigManager()
	testPath := "/path/to/clusters/org/.test-cluster-config.yaml"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.extractClusterName(testPath)
	}
}

// BenchmarkConfigManager_ConcurrentLoad benchmarks concurrent Load operations
func BenchmarkConfigManager_ConcurrentLoad(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "concurrent-config.yaml")

	configContent := `schema_version: "1.0"
cluster:
  name: concurrent-cluster
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		b.Fatalf("Failed to write test config: %v", err)
	}

	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := manager.Load(configPath, LoadOptions{})
			if err != nil {
				b.Fatalf("Concurrent load failed: %v", err)
			}
		}
	})
}

// BenchmarkV1Strategy_Load benchmarks V1 strategy loading
func BenchmarkV1Strategy_Load(b *testing.B) {
	strategy := strategies.NewV1Strategy()
	data := []byte(`schema_version: "1.0"
cluster:
  name: v1-benchmark
opencenter:
  infrastructure:
    provider: openstack
  meta:
    region: us-east-1
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := strategy.Load(data, "v1-benchmark")
		if err != nil {
			b.Fatalf("V1 load failed: %v", err)
		}
	}
}

// BenchmarkV1Strategy_CanLoad benchmarks V1 version detection
func BenchmarkV1Strategy_CanLoad(b *testing.B) {
	strategy := strategies.NewV1Strategy()
	data := []byte(`schema_version: "1.0"
cluster:
  name: test
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := strategy.CanLoad(data)
		if err != nil {
			b.Fatalf("CanLoad failed: %v", err)
		}
	}
}

// BenchmarkLegacyStrategy_Load benchmarks Legacy strategy loading
func BenchmarkLegacyStrategy_Load(b *testing.B) {
	strategy := strategies.NewLegacyStrategy()
	data := []byte(`cluster_name: legacy-benchmark
provider: openstack
region: us-east-1
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := strategy.Load(data, "legacy-benchmark")
		if err != nil {
			b.Fatalf("Legacy load failed: %v", err)
		}
	}
}

// BenchmarkConfigManager_LoadWithoutValidation benchmarks Load without validation
func BenchmarkConfigManager_LoadWithoutValidation(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "no-validation-config.yaml")

	// Create a simple config
	configContent := `schema_version: "1.0"
cluster:
  name: no-validation-cluster
  fqdn: validation.example.com
opencenter:
  infrastructure:
    provider: openstack
  meta:
    region: us-east-1
    env: dev
  cluster:
    cluster_name: no-validation-cluster
    base_domain: example.com
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		b.Fatalf("Failed to write test config: %v", err)
	}

	manager := NewConfigManager()
	manager.RegisterStrategy(strategies.NewV1Strategy())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.Load(configPath, LoadOptions{
			Validate:  false, // Skip validation to focus on load performance
			SkipCache: true,
		})
		if err != nil {
			b.Fatalf("Load without validation failed: %v", err)
		}
	}
}
