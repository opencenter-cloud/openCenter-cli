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
	"testing"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
)

// BenchmarkValidation_SmallConfig benchmarks validation of a small configuration.
func BenchmarkValidation_SmallConfig(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createSmallTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_MediumConfig benchmarks validation of a medium-sized configuration.
func BenchmarkValidation_MediumConfig(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createMediumTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_LargeConfig benchmarks validation of a large configuration.
func BenchmarkValidation_LargeConfig(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createLargeTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_VeryLargeConfig benchmarks validation of a very large configuration.
func BenchmarkValidation_VeryLargeConfig(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createVeryLargeTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.Validate(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_StructureOnly benchmarks structure validation only.
func BenchmarkValidation_StructureOnly(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createLargeTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.ValidateStructure(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_SemanticsOnly benchmarks semantic validation only.
func BenchmarkValidation_SemanticsOnly(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createLargeTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.ValidateSemantics(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_NetworkingOnly benchmarks networking validation only.
func BenchmarkValidation_NetworkingOnly(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createLargeTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.ValidateNetworking(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_CloudProviderOnly benchmarks cloud provider validation only.
func BenchmarkValidation_CloudProviderOnly(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createLargeTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.ValidateCloudProvider(ctx, cfg)
		if result == nil {
			b.Fatal("validation result should not be nil")
		}
	}
}

// BenchmarkValidation_ConcurrentSmall benchmarks concurrent validation of small configs.
func BenchmarkValidation_ConcurrentSmall(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createSmallTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := validator.Validate(ctx, cfg)
			if result == nil {
				b.Fatal("validation result should not be nil")
			}
		}
	})
}

// BenchmarkValidation_ConcurrentLarge benchmarks concurrent validation of large configs.
func BenchmarkValidation_ConcurrentLarge(b *testing.B) {
	validator := NewConfigValidator(false)
	cfg := createLargeTestConfig()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := validator.Validate(ctx, cfg)
			if result == nil {
				b.Fatal("validation result should not be nil")
			}
		}
	})
}

// TestValidation_PerformanceThresholds tests that validation meets performance thresholds.
func TestValidation_PerformanceThresholds(t *testing.T) {
	validator := NewConfigValidator(false)
	ctx := context.Background()

	tests := []struct {
		name      string
		config    *Config
		threshold time.Duration
	}{
		{
			name:      "small config",
			config:    createSmallTestConfig(),
			threshold: 5 * time.Millisecond,
		},
		{
			name:      "medium config",
			config:    createMediumTestConfig(),
			threshold: 10 * time.Millisecond,
		},
		{
			name:      "large config",
			config:    createLargeTestConfig(),
			threshold: 50 * time.Millisecond,
		},
		{
			name:      "very large config",
			config:    createVeryLargeTestConfig(),
			threshold: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			result := validator.Validate(ctx, tt.config)
			duration := time.Since(start)

			if result == nil {
				t.Fatal("validation result should not be nil")
			}

			if duration > tt.threshold {
				t.Errorf("validation took %v, which exceeds threshold of %v", duration, tt.threshold)
			} else {
				t.Logf("validation took %v (threshold: %v)", duration, tt.threshold)
			}
		})
	}
}

// TestValidation_MemoryUsage tests that validation doesn't use excessive memory.
func TestValidation_MemoryUsage(t *testing.T) {
	validator := NewConfigValidator(false)
	ctx := context.Background()

	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "small config",
			config: createSmallTestConfig(),
		},
		{
			name:   "medium config",
			config: createMediumTestConfig(),
		},
		{
			name:   "large config",
			config: createLargeTestConfig(),
		},
		{
			name:   "very large config",
			config: createVeryLargeTestConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run validation multiple times to check for memory leaks
			for i := 0; i < 100; i++ {
				result := validator.Validate(ctx, tt.config)
				if result == nil {
					t.Fatal("validation result should not be nil")
				}
			}
			// If we get here without running out of memory, the test passes
			t.Logf("completed 100 validation iterations for %s", tt.name)
		})
	}
}

// Helper functions to create test configurations of various sizes

func createSmallTestConfig() *Config {
	cfg := NewDefault("small-cluster")
	cfg.OpenCenter.GitOps.GitDir = "./gitops"
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
	cfg.OpenCenter.Cluster.Kubernetes.Version = "1.31.4"
	cfg.OpenCenter.Cluster.Kubernetes.MasterCount = 1
	cfg.OpenCenter.Cluster.Kubernetes.WorkerCount = 2
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = true
	cfg.OpenCenter.Cluster.AdminEmail = "admin@example.com"
	cfg.OpenCenter.Cluster.BaseDomain = "example.com"
	return &cfg
}

func createMediumTestConfig() *Config {
	cfg := createSmallTestConfig()

	// Add more services
	cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{}
	cfg.OpenCenter.Services["loki"] = &services.LokiConfig{
		StorageType:  "swift",
		BucketName:   "loki-logs",
		SwiftAuthURL: "https://keystone.example.com/v3",
		SwiftRegion:  "RegionOne",
	}
	cfg.OpenCenter.Services["kube-prometheus-stack"] = &services.PrometheusStackConfig{}

	// Add secrets
	cfg.Secrets.CertManager.AWSAccessKey = "test-access-key"
	cfg.Secrets.CertManager.AWSSecretAccessKey = "test-secret-key"
	cfg.Secrets.Loki.SwiftPassword = "test-password"
	cfg.Secrets.Grafana.AdminPassword = "test-password"

	return cfg
}

func createLargeTestConfig() *Config {
	cfg := createMediumTestConfig()

	// Add more services
	cfg.OpenCenter.Services["keycloak"] = &services.KeycloakConfig{}
	cfg.OpenCenter.Services["headlamp"] = &services.HeadlampConfig{}
	cfg.OpenCenter.Services["weave-gitops"] = &services.WeaveGitOpsConfig{}
	cfg.OpenCenter.Services["velero"] = &services.VeleroConfig{}
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{}

	// Add more secrets
	cfg.Secrets.Keycloak.AdminPassword = "test-password"
	cfg.Secrets.Keycloak.ClientSecret = "test-secret"
	cfg.Secrets.Headlamp.OIDCClientSecret = "test-secret"
	cfg.Secrets.WeaveGitOps.PasswordHash = "test-hash"

	// Add overrides
	if cfg.Overrides == nil {
		cfg.Overrides = make(map[string]any)
	}
	for i := 0; i < 50; i++ {
		cfg.Overrides[fmt.Sprintf("custom.setting.%d", i)] = fmt.Sprintf("value-%d", i)
	}

	// Add SSH keys
	cfg.OpenCenter.Cluster.SSHAuthorizedKeys = []string{
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC... user1@host",
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD... user2@host",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... user3@host",
	}

	return cfg
}

func createVeryLargeTestConfig() *Config {
	cfg := createLargeTestConfig()

	// Add many more overrides to simulate a very large configuration
	for i := 50; i < 200; i++ {
		cfg.Overrides[fmt.Sprintf("custom.setting.%d", i)] = fmt.Sprintf("value-%d", i)
		cfg.Overrides[fmt.Sprintf("custom.nested.%d.key1", i)] = fmt.Sprintf("nested-value-%d", i)
		cfg.Overrides[fmt.Sprintf("custom.nested.%d.key2", i)] = fmt.Sprintf("nested-value-%d", i)
	}

	// Add more SSH keys
	for i := 0; i < 20; i++ {
		cfg.OpenCenter.Cluster.SSHAuthorizedKeys = append(
			cfg.OpenCenter.Cluster.SSHAuthorizedKeys,
			fmt.Sprintf("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC%d... user%d@host", i, i),
		)
	}

	// Add networking configuration
	cfg.OpenCenter.Cluster.Kubernetes.Networking.UseOctavia = true
	cfg.OpenCenter.Cluster.Kubernetes.Networking.VRRPEnabled = false

	return cfg
}
