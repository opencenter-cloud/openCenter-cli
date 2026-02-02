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
	"runtime"
	"runtime/pprof"
	"testing"
)

// BenchmarkValidation_Complete benchmarks full validation pipeline
func BenchmarkValidation_Complete(b *testing.B) {
	validator := NewMultiLayerValidator()
	cfg := createValidTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(cfg)
	}
}

// BenchmarkValidation_SchemaOnly benchmarks schema validation layer
func BenchmarkValidation_SchemaOnly(b *testing.B) {
	validator := NewMultiLayerValidator()
	cfg := createValidTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateSchema(cfg)
	}
}

// BenchmarkValidation_BusinessRules benchmarks business rules validation layer
func BenchmarkValidation_BusinessRules(b *testing.B) {
	validator := NewMultiLayerValidator()
	cfg := createValidTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateBusinessRules(cfg)
	}
}

// BenchmarkValidation_Provider benchmarks provider-specific validation layer
func BenchmarkValidation_Provider(b *testing.B) {
	validator := NewMultiLayerValidator()
	cfg := createValidTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateProvider(cfg)
	}
}

// BenchmarkValidation_Services benchmarks service dependency validation layer
func BenchmarkValidation_Services(b *testing.B) {
	validator := NewMultiLayerValidator()
	cfg := createValidTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateServices(cfg)
	}
}

// BenchmarkValidation_WithErrors benchmarks validation with errors
func BenchmarkValidation_WithErrors(b *testing.B) {
	validator := NewMultiLayerValidator()
	cfg := createInvalidTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(cfg)
	}
}

// BenchmarkValidation_LargeConfig benchmarks validation with large config
func BenchmarkValidation_LargeConfig(b *testing.B) {
	validator := NewMultiLayerValidator()
	cfg := createLargeTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(cfg)
	}
}

// TestValidation_CPUProfile generates CPU profile for validation
func TestValidation_CPUProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CPU profiling in short mode")
	}

	// Create CPU profile file
	f, err := os.Create("validation_cpu.prof")
	if err != nil {
		t.Fatalf("could not create CPU profile: %v", err)
	}
	defer f.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(f); err != nil {
		t.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	// Run validation many times
	validator := NewMultiLayerValidator()
	cfg := createValidTestConfig()

	for i := 0; i < 10000; i++ {
		_ = validator.Validate(cfg)
	}

	t.Log("CPU profile written to validation_cpu.prof")
	t.Log("Analyze with: go tool pprof -http=:8080 validation_cpu.prof")
}

// TestValidation_MemoryProfile generates memory profile for validation
func TestValidation_MemoryProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profiling in short mode")
	}

	// Run validation many times
	validator := NewMultiLayerValidator()
	cfg := createValidTestConfig()

	for i := 0; i < 10000; i++ {
		_ = validator.Validate(cfg)
	}

	// Force garbage collection
	runtime.GC()

	// Create memory profile file
	f, err := os.Create("validation_mem.prof")
	if err != nil {
		t.Fatalf("could not create memory profile: %v", err)
	}
	defer f.Close()

	// Write heap profile
	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatalf("could not write memory profile: %v", err)
	}

	t.Log("Memory profile written to validation_mem.prof")
	t.Log("Analyze with: go tool pprof -http=:8080 validation_mem.prof")
}

// createValidTestConfig creates a valid test configuration
func createValidTestConfig() *Config {
	return &Config{
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         "test-cluster",
				Organization: "opencenter",
				Region:       "us-east-1",
			},
			Infrastructure: Infrastructure{
				Provider:  "openstack",
				SSHUser:   "ubuntu",
				OSVersion: "24",
				NodeNaming: NodeNaming{
					Worker: "wn",
					Master: "cp",
				},
				Bastion: BastionConfig{
					Address: "localhost",
				},
				Cloud: CloudConfig{
					OpenStack: SimplifiedOpenStackCloud{
						AuthURL: "https://identity.example.com/v3",
						Region:  "us-east-1",
					},
				},
			},
			Cluster: ClusterConfig{
				ClusterName:       "test-cluster",
				SSHAuthorizedKeys: []string{"ssh-rsa AAAA..."},
				BaseDomain:        "k8s.example.com",
				ClusterFQDN:       "test.k8s.example.com",
				AdminEmail:        "admin@example.com",
				Kubernetes: KubernetesConfig{
					Version:              "1.33.5",
					KubesprayVersion:     "v2.29.1",
					APIPort:              443,
					FlavorBastion:        "gp.0.2.2",
					SubnetPods:           "10.42.0.0/16",
					SubnetServices:       "10.43.0.0/16",
					LoadbalancerProvider: "ovn",
					MasterCount:          3,
					WorkerCount:          3,
					NetworkPlugin:        NetworkPlugin{},
				},
				Networking: ClusterNetworkingConfig{
					SubnetNodes:          "10.0.0.0/24",
					AllocationPoolStart:  "10.0.0.10",
					AllocationPoolEnd:    "10.0.0.100",
					NTPServers:           []string{"time.example.com"},
					DNSNameservers:       []string{"8.8.8.8"},
					LoadbalancerProvider: "ovn",
					VRRPEnabled:          true,
					VRRPIP:               "10.0.0.5",
				},
			},
			GitOps: GitOpsConfig{
				GitDir:            "./testdata/test-git-repo",
				GitOpsBaseRepo:    "ssh://git@github.com/example/repo.git",
				GitOpsBaseRelease: "v0.1.0",
				GitOpsBranch:      "main",
			},
			Storage: StorageConfig{
				DefaultStorageClass:         "csi-cinder-sc-delete",
				WorkerVolumeSize:            40,
				WorkerVolumeDestinationType: "volume",
				WorkerVolumeSourceType:      "image",
				WorkerVolumeType:            "HA-Standard",
			},
			Services: ServiceMap{},
		},
		OpenTofu: SimplifiedOpenTofu{
			Path: "opentofu",
			Backend: SimplifiedTofuBackend{
				Type: "local",
				Local: SimplifiedTofuLocal{
					Path: "./terraform.tfstate",
				},
			},
		},
		Secrets: Secrets{},
	}
}

// createInvalidTestConfig creates an invalid test configuration
func createInvalidTestConfig() *Config {
	return &Config{
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				// Missing Name
				Organization: "opencenter",
				Region:       "us-east-1",
			},
			Infrastructure: Infrastructure{
				Provider: "invalid-provider",
			},
			Cluster: ClusterConfig{
				Networking: ClusterNetworkingConfig{
					SubnetNodes:         "invalid-cidr",
					AllocationPoolStart: "10.0.1.10",
					AllocationPoolEnd:   "10.0.1.100",
					VRRPEnabled:         true,
					VRRPIP:              "",
				},
				Kubernetes: KubernetesConfig{
					MasterCount: -1,
					WorkerCount: -1,
				},
			},
		},
	}
}

// createLargeTestConfig creates a large test configuration with many services
func createLargeTestConfig() *Config {
	cfg := createValidTestConfig()

	// Add many services to test service validation performance
	cfg.OpenCenter.Services = ServiceMap{
		"cert-manager": map[string]interface{}{
			"enabled": true,
		},
		"external-dns": map[string]interface{}{
			"enabled": true,
		},
		"gateway": map[string]interface{}{
			"enabled": true,
		},
		"harbor": map[string]interface{}{
			"enabled": true,
		},
		"keycloak": map[string]interface{}{
			"enabled": true,
		},
		"loki": map[string]interface{}{
			"enabled": true,
		},
		"tempo": map[string]interface{}{
			"enabled": true,
		},
		"grafana": map[string]interface{}{
			"enabled": true,
		},
		"prometheus": map[string]interface{}{
			"enabled": true,
		},
		"alertmanager": map[string]interface{}{
			"enabled": true,
		},
	}

	return cfg
}
