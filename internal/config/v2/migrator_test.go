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

package v2

import (
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
)

// Test VRRP IP relocation from cluster.networking to infrastructure.networking
// Requirements: 12.2
func TestMigrator_VRRPIPRelocation(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Cluster.Networking.VRRPIP = "10.2.128.100"
	v1Config.OpenCenter.Cluster.Networking.VRRPEnabled = true

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify VRRP IP was relocated to infrastructure.networking
	if v2Config.OpenCenter.Infrastructure.Networking.VRRPIP != "10.2.128.100" {
		t.Errorf("VRRP IP not relocated correctly: expected 10.2.128.100, got %s",
			v2Config.OpenCenter.Infrastructure.Networking.VRRPIP)
	}

	if !v2Config.OpenCenter.Infrastructure.Networking.VRRPEnabled {
		t.Error("VRRP enabled not relocated correctly")
	}
}

// Test compute settings relocation from cluster.kubernetes to infrastructure.compute
// Requirements: 12.4
func TestMigrator_ComputeSettingsRelocation(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Cluster.Kubernetes.FlavorMaster = "gp.0.4.8"
	v1Config.OpenCenter.Cluster.Kubernetes.FlavorWorker = "gp.0.4.16"
	v1Config.OpenCenter.Cluster.Kubernetes.MasterCount = 3
	v1Config.OpenCenter.Cluster.Kubernetes.WorkerCount = 5

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify compute settings were relocated
	if v2Config.OpenCenter.Infrastructure.Compute.FlavorMaster != "gp.0.4.8" {
		t.Errorf("FlavorMaster not relocated correctly: expected gp.0.4.8, got %s",
			v2Config.OpenCenter.Infrastructure.Compute.FlavorMaster)
	}

	if v2Config.OpenCenter.Infrastructure.Compute.FlavorWorker != "gp.0.4.16" {
		t.Errorf("FlavorWorker not relocated correctly: expected gp.0.4.16, got %s",
			v2Config.OpenCenter.Infrastructure.Compute.FlavorWorker)
	}

	if v2Config.OpenCenter.Infrastructure.Compute.MasterCount != 3 {
		t.Errorf("MasterCount not relocated correctly: expected 3, got %d",
			v2Config.OpenCenter.Infrastructure.Compute.MasterCount)
	}

	if v2Config.OpenCenter.Infrastructure.Compute.WorkerCount != 5 {
		t.Errorf("WorkerCount not relocated correctly: expected 5, got %d",
			v2Config.OpenCenter.Infrastructure.Compute.WorkerCount)
	}
}

// Test storage settings relocation from opencenter.storage to infrastructure.storage
// Requirements: 12.4
func TestMigrator_StorageSettingsRelocation(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Storage.DefaultStorageClass = "csi-cinder-sc-delete"
	v1Config.OpenCenter.Storage.WorkerVolumeSize = 50
	v1Config.OpenCenter.Storage.WorkerVolumeType = "HA-Performance"

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify storage settings were relocated
	if v2Config.OpenCenter.Infrastructure.Storage.DefaultStorageClass != "csi-cinder-sc-delete" {
		t.Errorf("DefaultStorageClass not relocated correctly: expected csi-cinder-sc-delete, got %s",
			v2Config.OpenCenter.Infrastructure.Storage.DefaultStorageClass)
	}

	if v2Config.OpenCenter.Infrastructure.Storage.WorkerVolumeSize != 50 {
		t.Errorf("WorkerVolumeSize not relocated correctly: expected 50, got %d",
			v2Config.OpenCenter.Infrastructure.Storage.WorkerVolumeSize)
	}

	if v2Config.OpenCenter.Infrastructure.Storage.WorkerVolumeType != "HA-Performance" {
		t.Errorf("WorkerVolumeType not relocated correctly: expected HA-Performance, got %s",
			v2Config.OpenCenter.Infrastructure.Storage.WorkerVolumeType)
	}
}

// Test SSH settings relocation from cluster to infrastructure.ssh
// Requirements: 12.5
func TestMigrator_SSHSettingsRelocation(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Cluster.SSHAuthorizedKeys = []string{
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...",
	}
	v1Config.OpenCenter.Infrastructure.SSHUser = "ubuntu"

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify SSH settings were relocated
	if len(v2Config.OpenCenter.Infrastructure.SSH.AuthorizedKeys) != 2 {
		t.Errorf("SSH authorized keys not relocated correctly: expected 2 keys, got %d",
			len(v2Config.OpenCenter.Infrastructure.SSH.AuthorizedKeys))
	}

	if v2Config.OpenCenter.Infrastructure.SSH.Username != "ubuntu" {
		t.Errorf("SSH username not relocated correctly: expected ubuntu, got %s",
			v2Config.OpenCenter.Infrastructure.SSH.Username)
	}
}

// Test networking settings relocation from cluster.networking to infrastructure.networking
// Requirements: 12.3
func TestMigrator_NetworkingSettingsRelocation(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Cluster.Networking.SubnetNodes = "10.2.128.0/22"
	v1Config.OpenCenter.Cluster.Networking.DNSZoneName = "k8s.example.com"
	v1Config.OpenCenter.Cluster.Networking.DNSNameservers = []string{"8.8.8.8", "8.8.4.4"}
	v1Config.OpenCenter.Cluster.Networking.NTPServers = []string{"time.example.com"}

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify networking settings were relocated
	if v2Config.OpenCenter.Infrastructure.Networking.SubnetNodes != "10.2.128.0/22" {
		t.Errorf("SubnetNodes not relocated correctly")
	}

	if v2Config.OpenCenter.Infrastructure.Networking.DNSZoneName != "k8s.example.com" {
		t.Errorf("DNSZoneName not relocated correctly")
	}

	if len(v2Config.OpenCenter.Infrastructure.Networking.DNSNameservers) != 2 {
		t.Errorf("DNSNameservers not relocated correctly")
	}

	if len(v2Config.OpenCenter.Infrastructure.Networking.NTPServers) != 1 {
		t.Errorf("NTPServers not relocated correctly")
	}
}

// Test hydration applied during migration
// Requirements: 12.6
func TestMigrator_HydrationApplied(t *testing.T) {
	v1Config := createTestV1Config()

	// Create a mock hydrator
	mockHydrator := &mockTestHydrator{
		hydrateCalled: false,
	}

	migrator := NewMigrator(mockHydrator)
	_, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify hydration was called
	if !mockHydrator.hydrateCalled {
		t.Error("Hydration was not called during migration")
	}
}

// Test migration report generation
// Requirements: 12.6
func TestMigrator_MigrationReportGeneration(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Cluster.Networking.VRRPIP = "10.2.128.100"

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Generate migration report
	report, err := migrator.GenerateMigrationReport(&v1Config, v2Config)
	if err != nil {
		t.Fatalf("Failed to generate migration report: %v", err)
	}

	// Verify report contains field relocations
	if len(report.MovedFields) == 0 {
		t.Error("Migration report should contain moved fields")
	}

	// Verify specific field relocations are documented
	if _, ok := report.MovedFields["cluster.networking.vrrp_ip"]; !ok {
		t.Error("Migration report should document VRRP IP relocation")
	}

	if _, ok := report.MovedFields["cluster.kubernetes.flavor_*"]; !ok {
		t.Error("Migration report should document flavor relocation")
	}
}

// Test validation of migrated configuration
// Requirements: 12.7
func TestMigrator_ValidateMigration(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Meta.Name = "test-cluster"
	v1Config.OpenCenter.Infrastructure.Provider = "openstack"
	v1Config.OpenCenter.Cluster.Networking.VRRPIP = "10.2.128.100"

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Validate migration
	err = migrator.ValidateMigration(&v1Config, v2Config)
	if err != nil {
		t.Errorf("Migration validation failed: %v", err)
	}

	// Verify schema version
	if v2Config.SchemaVersion != "2.0" {
		t.Errorf("Schema version not set correctly: expected 2.0, got %s", v2Config.SchemaVersion)
	}

	// Verify critical fields
	if v2Config.OpenCenter.Meta.Name != "test-cluster" {
		t.Error("Cluster name not migrated correctly")
	}

	if v2Config.OpenCenter.Infrastructure.Provider != "openstack" {
		t.Error("Provider not migrated correctly")
	}

	if v2Config.OpenCenter.Infrastructure.Networking.VRRPIP != "10.2.128.100" {
		t.Error("VRRP IP not migrated correctly")
	}
}

// Test validation fails for mismatched configurations
func TestMigrator_ValidateMigration_Mismatch(t *testing.T) {
	v1Config := createTestV1Config()
	v1Config.OpenCenter.Meta.Name = "cluster-a"

	migrator := NewMigrator(nil)
	v2Config, err := migrator.Migrate(&v1Config)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Modify v2 config to create mismatch
	v2Config.OpenCenter.Meta.Name = "cluster-b"

	// Validate migration should fail
	err = migrator.ValidateMigration(&v1Config, v2Config)
	if err == nil {
		t.Error("Validation should fail for mismatched cluster names")
	}
}

// Test nil config handling
func TestMigrator_NilConfig(t *testing.T) {
	migrator := NewMigrator(nil)

	// Test nil v1 config
	_, err := migrator.Migrate(nil)
	if err == nil {
		t.Error("Migration should fail for nil v1 config")
	}

	// Test nil validation
	err = migrator.ValidateMigration(nil, nil)
	if err == nil {
		t.Error("Validation should fail for nil configs")
	}

	// Test nil report generation
	_, err = migrator.GenerateMigrationReport(nil, nil)
	if err == nil {
		t.Error("Report generation should fail for nil configs")
	}
}

// Helper functions

func createTestV1Config() config.Config {
	return config.Config{
		SchemaVersion: "1.0",
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         "test-cluster",
				Organization: "opencenter",
				Env:          "dev",
				Region:       "sjc3",
			},
			Infrastructure: config.Infrastructure{
				Provider:  "openstack",
				SSHUser:   "ubuntu",
				OSVersion: "24",
				Cloud: config.CloudConfig{
					OpenStack: config.SimplifiedOpenStackCloud{
						AuthURL:    "https://identity.example.com/v3",
						Region:     "sjc3",
						TenantName: "admin",
					},
				},
			},
			Cluster: config.ClusterConfig{
				ClusterName:       "test-cluster",
				BaseDomain:        "k8s.opencenter.cloud",
				ClusterFQDN:       "test-cluster.sjc3.k8s.opencenter.cloud",
				AdminEmail:        "admin@example.com",
				SSHAuthorizedKeys: []string{"ssh-rsa AAAAB3..."},
				Networking: config.ClusterNetworkingConfig{
					SubnetNodes:          "10.2.128.0/22",
					AllocationPoolStart:  "10.2.128.10",
					AllocationPoolEnd:    "10.2.131.254",
					DNSNameservers:       []string{"8.8.8.8"},
					NTPServers:           []string{"time.sjc3.rackspace.com"},
					LoadbalancerProvider: "ovn",
					DNSZoneName:          "k8s.opencenter.cloud",
				},
				Kubernetes: config.KubernetesConfig{
					Version:        "1.33.5",
					APIPort:        443,
					FlavorBastion:  "gp.0.2.2",
					FlavorMaster:   "gp.0.4.8",
					FlavorWorker:   "gp.0.4.16",
					MasterCount:    3,
					WorkerCount:    2,
					SubnetPods:     "10.42.0.0/16",
					SubnetServices: "10.43.0.0/16",
				},
			},
			Storage: config.StorageConfig{
				DefaultStorageClass:         "csi-cinder-sc-delete",
				WorkerVolumeSize:            40,
				WorkerVolumeDestinationType: "volume",
				WorkerVolumeSourceType:      "image",
				WorkerVolumeType:            "HA-Standard",
			},
			GitOps: config.GitOpsConfig{
				GitURL:            "git@github.com:example/cluster.git",
				GitBranch:         "main",
				GitOpsBaseRepo:    "ssh://git@github.com/rackerlabs/opencenter-gitops-base.git",
				GitOpsBaseRelease: "v0.1.0",
			},
		},
		OpenTofu: config.SimplifiedOpenTofu{
			Enabled: true,
			Backend: config.SimplifiedTofuBackend{
				Type: "local",
				Local: config.SimplifiedTofuLocal{
					Path: ".opentofu-local/terraform.tfstate",
				},
			},
		},
		Deployment: config.Deployment{
			AutoDeploy: true,
		},
		Secrets: config.Secrets{
			SopsAgeKeyFile: "/path/to/age/key.txt",
		},
	}
}

// mockTestHydrator is a mock hydrator for testing
type mockTestHydrator struct {
	hydrateCalled bool
}

func (m *mockTestHydrator) Hydrate(cfg interface{}, provider, region string) error {
	m.hydrateCalled = true
	return nil
}

func (m *mockTestHydrator) GetAppliedDefaults() map[string]defaults.DefaultSource {
	return make(map[string]defaults.DefaultSource)
}
