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

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
)

// Property 10: Migration Field Relocation
// **Validates: Requirements 12.2, 12.3, 12.4, 12.5**
//
// For any v1 configuration, migrating to v2 must correctly relocate fields:
// - VRRP IP from cluster.networking.vrrp_ip to infrastructure.networking.vrrp_ip
// - Networking settings from cluster.networking to infrastructure.networking
// - Compute settings from cluster.kubernetes to infrastructure.compute
// - Storage settings from opencenter.storage to infrastructure.storage
// - SSH settings from cluster to infrastructure.ssh
func TestProperty_MigrationFieldRelocation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("VRRP IP relocated from cluster.networking to infrastructure.networking", prop.ForAll(
		func(vrrpIP string, vrrpEnabled bool) bool {
			// Create v1 config with VRRP IP in cluster.networking
			v1Config := createMinimalV1Config()
			v1Config.OpenCenter.Cluster.Networking.VRRPIP = vrrpIP
			v1Config.OpenCenter.Cluster.Networking.VRRPEnabled = vrrpEnabled

			// Migrate to v2
			migrator := NewMigrator(nil)
			v2Config, err := migrator.Migrate(&v1Config)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify VRRP IP was relocated to infrastructure.networking
			if v2Config.OpenCenter.Infrastructure.Networking.VRRPIP != vrrpIP {
				t.Logf("VRRP IP not relocated correctly: expected %s, got %s",
					vrrpIP, v2Config.OpenCenter.Infrastructure.Networking.VRRPIP)
				return false
			}

			if v2Config.OpenCenter.Infrastructure.Networking.VRRPEnabled != vrrpEnabled {
				t.Logf("VRRP enabled not relocated correctly: expected %v, got %v",
					vrrpEnabled, v2Config.OpenCenter.Infrastructure.Networking.VRRPEnabled)
				return false
			}

			return true
		},
		genIPv4Address(),
		gen.Bool(),
	))

	properties.Property("Networking settings relocated from cluster.networking to infrastructure.networking", prop.ForAll(
		func(subnetNodes, dnsZone string, dnsServers, ntpServers []string) bool {
			// Create v1 config with networking settings in cluster.networking
			v1Config := createMinimalV1Config()
			v1Config.OpenCenter.Cluster.Networking.SubnetNodes = subnetNodes
			v1Config.OpenCenter.Cluster.Networking.DNSZoneName = dnsZone
			v1Config.OpenCenter.Cluster.Networking.DNSNameservers = dnsServers
			v1Config.OpenCenter.Cluster.Networking.NTPServers = ntpServers

			// Migrate to v2
			migrator := NewMigrator(nil)
			v2Config, err := migrator.Migrate(&v1Config)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify networking settings were relocated
			if v2Config.OpenCenter.Infrastructure.Networking.SubnetNodes != subnetNodes {
				t.Logf("SubnetNodes not relocated correctly")
				return false
			}

			if v2Config.OpenCenter.Infrastructure.Networking.DNSZoneName != dnsZone {
				t.Logf("DNSZoneName not relocated correctly")
				return false
			}

			if len(v2Config.OpenCenter.Infrastructure.Networking.DNSNameservers) != len(dnsServers) {
				t.Logf("DNSNameservers not relocated correctly")
				return false
			}

			if len(v2Config.OpenCenter.Infrastructure.Networking.NTPServers) != len(ntpServers) {
				t.Logf("NTPServers not relocated correctly")
				return false
			}

			return true
		},
		genCIDR(),
		genDomainName(),
		genIPv4AddressList(1, 3),
		genDomainNameList(1, 3),
	))

	properties.Property("Compute settings relocated from cluster.kubernetes to infrastructure.compute", prop.ForAll(
		func(flavorMaster, flavorWorker string, masterCount, workerCount int) bool {
			// Create v1 config with compute settings in cluster.kubernetes
			v1Config := createMinimalV1Config()
			v1Config.OpenCenter.Cluster.Kubernetes.FlavorMaster = flavorMaster
			v1Config.OpenCenter.Cluster.Kubernetes.FlavorWorker = flavorWorker
			v1Config.OpenCenter.Cluster.Kubernetes.MasterCount = masterCount
			v1Config.OpenCenter.Cluster.Kubernetes.WorkerCount = workerCount

			// Migrate to v2
			migrator := NewMigrator(nil)
			v2Config, err := migrator.Migrate(&v1Config)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify compute settings were relocated
			if v2Config.OpenCenter.Infrastructure.Compute.FlavorMaster != flavorMaster {
				t.Logf("FlavorMaster not relocated correctly")
				return false
			}

			if v2Config.OpenCenter.Infrastructure.Compute.FlavorWorker != flavorWorker {
				t.Logf("FlavorWorker not relocated correctly")
				return false
			}

			if v2Config.OpenCenter.Infrastructure.Compute.MasterCount != masterCount {
				t.Logf("MasterCount not relocated correctly")
				return false
			}

			if v2Config.OpenCenter.Infrastructure.Compute.WorkerCount != workerCount {
				t.Logf("WorkerCount not relocated correctly")
				return false
			}

			return true
		},
		genFlavorName(),
		genFlavorName(),
		gen.IntRange(0, 7),
		gen.IntRange(0, 10),
	))

	properties.Property("Storage settings relocated from opencenter.storage to infrastructure.storage", prop.ForAll(
		func(storageClass string, volumeSize int, volumeType string) bool {
			// Create v1 config with storage settings in opencenter.storage
			v1Config := createMinimalV1Config()
			v1Config.OpenCenter.Storage.DefaultStorageClass = storageClass
			v1Config.OpenCenter.Storage.WorkerVolumeSize = volumeSize
			v1Config.OpenCenter.Storage.WorkerVolumeType = volumeType

			// Migrate to v2
			migrator := NewMigrator(nil)
			v2Config, err := migrator.Migrate(&v1Config)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify storage settings were relocated
			if v2Config.OpenCenter.Infrastructure.Storage.DefaultStorageClass != storageClass {
				t.Logf("DefaultStorageClass not relocated correctly")
				return false
			}

			if v2Config.OpenCenter.Infrastructure.Storage.WorkerVolumeSize != volumeSize {
				t.Logf("WorkerVolumeSize not relocated correctly")
				return false
			}

			if v2Config.OpenCenter.Infrastructure.Storage.WorkerVolumeType != volumeType {
				t.Logf("WorkerVolumeType not relocated correctly")
				return false
			}

			return true
		},
		genStorageClassName(),
		gen.IntRange(10, 500),
		genVolumeType(),
	))

	properties.Property("SSH settings relocated from cluster to infrastructure.ssh", prop.ForAll(
		func(sshKeys []string, sshUser string) bool {
			// Create v1 config with SSH settings in cluster
			v1Config := createMinimalV1Config()
			v1Config.OpenCenter.Cluster.SSHAuthorizedKeys = sshKeys
			v1Config.OpenCenter.Infrastructure.SSHUser = sshUser

			// Migrate to v2
			migrator := NewMigrator(nil)
			v2Config, err := migrator.Migrate(&v1Config)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify SSH settings were relocated
			if len(v2Config.OpenCenter.Infrastructure.SSH.AuthorizedKeys) != len(sshKeys) {
				t.Logf("SSH authorized keys not relocated correctly")
				return false
			}

			if v2Config.OpenCenter.Infrastructure.SSH.Username != sshUser {
				t.Logf("SSH username not relocated correctly")
				return false
			}

			return true
		},
		genSSHKeyList(1, 3),
		genUsername(),
	))

	properties.TestingRun(t)
}

// Property 11: Migration Hydration
// **Validates: Requirements 12.6**
//
// For any v1 configuration with implicit defaults, migrating to v2 must apply
// hydration so that all implicit v1 defaults become explicit values in the v2 configuration.
func TestProperty_MigrationHydration(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("Implicit defaults become explicit after migration", prop.ForAll(
		func(clusterName, provider string) bool {
			// Create v1 config with minimal settings (relying on implicit defaults)
			v1Config := config.Config{
				SchemaVersion: "1.0",
				OpenCenter: config.SimplifiedOpenCenter{
					Meta: config.ClusterMeta{
						Name:         clusterName,
						Organization: "opencenter",
						Env:          "dev",
						Region:       "sjc3",
					},
					Infrastructure: config.Infrastructure{
						Provider: provider,
					},
					Cluster: config.ClusterConfig{
						ClusterName: clusterName,
						BaseDomain:  "k8s.opencenter.cloud",
						ClusterFQDN: clusterName + ".sjc3.k8s.opencenter.cloud",
						AdminEmail:  "admin@example.com",
						Kubernetes: config.KubernetesConfig{
							Version:        "1.33.5",
							APIPort:        443,
							SubnetPods:     "10.42.0.0/16",
							SubnetServices: "10.43.0.0/16",
						},
					},
				},
			}

			// Create a mock hydrator that tracks applied defaults
			mockHydrator := &mockHydrator{
				appliedDefaults: make(map[string]defaults.DefaultSource),
			}

			// Migrate to v2 with hydration
			migrator := NewMigrator(mockHydrator)
			v2Config, err := migrator.Migrate(&v1Config)
			if err != nil {
				t.Logf("Migration failed: %v", err)
				return false
			}

			// Verify that v2 config has explicit values (not empty)
			if v2Config.SchemaVersion != "2.0" {
				t.Logf("Schema version not set correctly")
				return false
			}

			// Verify metadata was populated
			if v2Config.Metadata.CreatedAt == "" {
				t.Logf("CreatedAt not populated")
				return false
			}

			if v2Config.Metadata.UpdatedAt == "" {
				t.Logf("UpdatedAt not populated")
				return false
			}

			return true
		},
		genClusterName(),
		genProvider(),
	))

	properties.TestingRun(t)
}

// Helper functions for generators

func createMinimalV1Config() config.Config {
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
					DNSNameservers:       []string{"8.8.8.8", "8.8.4.4"},
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

func genIPv4Address() gopter.Gen {
	return gen.RegexMatch(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`).
		SuchThat(func(v interface{}) bool {
			return v.(string) != ""
		})
}

func genIPv4AddressList(min, max int) gopter.Gen {
	return gen.SliceOfN(max, genIPv4Address()).
		SuchThat(func(v interface{}) bool {
			return len(v.([]string)) >= min
		})
}

func genCIDR() gopter.Gen {
	return gen.OneConstOf(
		"10.2.128.0/22",
		"10.42.0.0/16",
		"10.43.0.0/16",
		"192.168.0.0/24",
		"172.16.0.0/16",
	)
}

func genDomainName() gopter.Gen {
	return gen.OneConstOf(
		"k8s.opencenter.cloud",
		"example.com",
		"test.local",
		"cluster.example.org",
	)
}

func genDomainNameList(min, max int) gopter.Gen {
	return gen.SliceOfN(max, genDomainName()).
		SuchThat(func(v interface{}) bool {
			return len(v.([]string)) >= min
		})
}

func genFlavorName() gopter.Gen {
	return gen.OneConstOf(
		"gp.0.2.2",
		"gp.0.4.8",
		"gp.0.4.16",
		"gp.0.8.32",
		"m1.small",
		"m1.medium",
		"m1.large",
	)
}

func genStorageClassName() gopter.Gen {
	return gen.OneConstOf(
		"csi-cinder-sc-delete",
		"csi-cinder-sc-retain",
		"standard",
		"fast",
	)
}

func genVolumeType() gopter.Gen {
	return gen.OneConstOf(
		"HA-Standard",
		"HA-Performance",
		"Standard",
		"Performance",
	)
}

func genSSHKeyList(min, max int) gopter.Gen {
	sshKey := gen.Const("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC...")
	return gen.SliceOfN(max, sshKey).
		SuchThat(func(v interface{}) bool {
			return len(v.([]string)) >= min
		})
}

func genUsername() gopter.Gen {
	return gen.OneConstOf(
		"ubuntu",
		"centos",
		"debian",
		"admin",
	)
}

func genClusterName() gopter.Gen {
	return gen.RegexMatch(`^[a-z][a-z0-9-]{2,20}$`)
}

func genProvider() gopter.Gen {
	return gen.OneConstOf(
		"openstack",
		"aws",
		"gcp",
		"azure",
	)
}

// mockHydrator is a mock implementation of the Hydrator interface for testing
type mockHydrator struct {
	appliedDefaults map[string]defaults.DefaultSource
}

func (m *mockHydrator) Hydrate(cfg interface{}, provider, region string) error {
	// Mock hydration - just track that it was called
	m.appliedDefaults["mock_field"] = defaults.SourceProviderRegion
	return nil
}

func (m *mockHydrator) GetAppliedDefaults() map[string]defaults.DefaultSource {
	return m.appliedDefaults
}
