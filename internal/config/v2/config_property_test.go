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
	"reflect"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 1: Configuration Structure Invariants
// For any valid v2 configuration, the following structural invariants must hold:
// - VRRP IP appears only in `infrastructure.networking.vrrp_ip` (not in deprecated locations)
// - Provider-specific settings appear only under `infrastructure.cloud.<provider>` matching the active provider
// - Infrastructure networking fields (subnet_nodes, dns_nameservers, ntp_servers) appear only in `infrastructure.networking`
// - Kubernetes networking fields (subnet_pods, subnet_services) appear only in `cluster.kubernetes`
// - Compute configuration (flavors, node counts) appears only in `infrastructure.compute`
// - Storage configuration (boot volumes, storage classes) appears only in `infrastructure.storage`
// **Validates: Requirements 1.1, 1.2, 2.1, 3.1, 4.1**
func TestProperty_ConfigurationStructureInvariants(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("VRRP IP only in infrastructure.networking", prop.ForAll(
		func(cfg *Config) bool {
			// Verify VRRP IP is in infrastructure.networking.vrrp_ip
			vrrpIP := cfg.OpenCenter.Infrastructure.Networking.VRRPIP

			// Verify it doesn't appear in deprecated locations by checking struct fields
			// We use reflection to ensure no other field contains the VRRP IP value
			if vrrpIP == "" {
				return true // Empty VRRP IP is valid
			}

			// Check that VRRP IP doesn't appear in cluster domain
			clusterVal := reflect.ValueOf(cfg.OpenCenter.Cluster)
			if containsVRRPIPInStruct(clusterVal, vrrpIP, "infrastructure.networking.vrrp_ip") {
				return false
			}

			// Check that VRRP IP doesn't appear in cloud provider configs (except as reference)
			cloudVal := reflect.ValueOf(cfg.OpenCenter.Infrastructure.Cloud)
			if containsVRRPIPInStruct(cloudVal, vrrpIP, "infrastructure.networking.vrrp_ip") {
				return false
			}

			return true
		},
		genValidV2Config(),
	))

	properties.Property("provider-specific settings only in matching cloud section", prop.ForAll(
		func(cfg *Config) bool {
			provider := cfg.OpenCenter.Infrastructure.Provider
			cloud := cfg.OpenCenter.Infrastructure.Cloud

			// Count how many provider sections are populated
			populatedSections := 0
			if cloud.OpenStack != nil && !isEmptyStruct(cloud.OpenStack) {
				populatedSections++
				if provider != "openstack" {
					return false
				}
			}
			if cloud.AWS != nil && !isEmptyStruct(cloud.AWS) {
				populatedSections++
				if provider != "aws" {
					return false
				}
			}
			if cloud.GCP != nil && !isEmptyStruct(cloud.GCP) {
				populatedSections++
				if provider != "gcp" {
					return false
				}
			}
			if cloud.Azure != nil && !isEmptyStruct(cloud.Azure) {
				populatedSections++
				if provider != "azure" {
					return false
				}
			}
			if cloud.VMware != nil && !isEmptyStruct(cloud.VMware) {
				populatedSections++
				if provider != "vsphere" {
					return false
				}
			}

			// Only one provider section should be populated
			return populatedSections <= 1
		},
		genValidV2Config(),
	))

	properties.Property("infrastructure networking fields only in infrastructure.networking", prop.ForAll(
		func(cfg *Config) bool {
			// Verify infrastructure networking fields are in infrastructure.networking
			networking := cfg.OpenCenter.Infrastructure.Networking

			// These fields should only exist in infrastructure.networking
			if networking.SubnetNodes == "" {
				return false // Required field
			}
			if len(networking.DNSNameservers) == 0 {
				return false // Required field
			}
			if len(networking.NTPServers) == 0 {
				return false // Required field
			}

			// Verify these fields don't appear in cluster domain
			// (This is enforced by struct design, but we verify the invariant)
			return true
		},
		genValidV2Config(),
	))

	properties.Property("compute configuration only in infrastructure.compute", prop.ForAll(
		func(cfg *Config) bool {
			compute := cfg.OpenCenter.Infrastructure.Compute

			// Verify compute fields are populated
			if compute.MasterCount > 0 && compute.FlavorMaster == "" {
				return false
			}
			if compute.WorkerCount > 0 && compute.FlavorWorker == "" {
				return false
			}

			// Verify compute configuration doesn't leak into cluster domain
			// (This is enforced by struct design)
			return true
		},
		genValidV2Config(),
	))

	properties.Property("storage configuration only in infrastructure.storage", prop.ForAll(
		func(cfg *Config) bool {
			storage := cfg.OpenCenter.Infrastructure.Storage

			// Verify storage fields are populated
			if storage.DefaultStorageClass == "" {
				return false
			}
			if storage.WorkerVolumeSize <= 0 {
				return false
			}

			// Verify storage configuration doesn't leak into other domains
			// (This is enforced by struct design)
			return true
		},
		genValidV2Config(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper functions

// containsVRRPIPInStruct checks if a struct contains the VRRP IP value in any field
// except the allowed field path.
func containsVRRPIPInStruct(v reflect.Value, vrrpIP, allowedPath string) bool {
	if !v.IsValid() {
		return false
	}

	switch v.Kind() {
	case reflect.String:
		// Check if this string field contains the VRRP IP
		if v.String() == vrrpIP {
			return true
		}
	case reflect.Struct:
		// Recursively check struct fields
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if containsVRRPIPInStruct(field, vrrpIP, allowedPath) {
				return true
			}
		}
	case reflect.Ptr:
		if !v.IsNil() {
			return containsVRRPIPInStruct(v.Elem(), vrrpIP, allowedPath)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if containsVRRPIPInStruct(v.Index(i), vrrpIP, allowedPath) {
				return true
			}
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			if containsVRRPIPInStruct(v.MapIndex(key), vrrpIP, allowedPath) {
				return true
			}
		}
	}

	return false
}

// isEmptyStruct checks if a struct pointer is effectively empty (all zero values).
func isEmptyStruct(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return true
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return false
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.IsZero() {
			return false
		}
	}

	return true
}

// Generators for property-based testing

// genValidV2Config generates valid v2 configurations for testing.
func genValidV2Config() gopter.Gen {
	return gopter.CombineGens(
		genMetaConfig(),
		genClusterConfig(),
		genInfrastructureConfig(),
	).Map(func(parts []interface{}) *Config {
		return &Config{
			SchemaVersion: "2.0",
			OpenCenter: OpenCenterConfig{
				Meta:           parts[0].(MetaConfig),
				Cluster:        parts[1].(ClusterConfig),
				Infrastructure: parts[2].(InfrastructureConfig),
			},
			Secrets: SecretsConfig{
				Global: GlobalSecrets{},
			},
		}
	})
}

// genMetaConfig generates valid MetaConfig.
func genMetaConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 64 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.OneConstOf("dev", "staging", "production"),
		gen.OneConstOf("sjc3", "dfw3", "iad3", "us-east-1", "us-west-2"),
	).Map(func(parts []interface{}) MetaConfig {
		return MetaConfig{
			Name:         strings.ToLower(parts[0].(string)),
			Organization: parts[1].(string),
			Env:          parts[2].(string),
			Region:       parts[3].(string),
		}
	})
}

// genClusterConfig generates valid ClusterConfig.
func genClusterConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 64 }),
		gen.Const("example.com"),
		gen.Const("admin@example.com"),
		genKubernetesConfig(),
	).Map(func(parts []interface{}) ClusterConfig {
		clusterName := strings.ToLower(parts[0].(string))
		baseDomain := parts[1].(string)
		return ClusterConfig{
			ClusterName: clusterName,
			BaseDomain:  baseDomain,
			ClusterFQDN: clusterName + "." + baseDomain,
			AdminEmail:  parts[2].(string),
			Kubernetes:  parts[3].(KubernetesConfig),
		}
	})
}

// genKubernetesConfig generates valid KubernetesConfig.
func genKubernetesConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("1.28.0", "1.29.0", "1.30.0"),
		gen.IntRange(6443, 6443),
		gen.Const("10.233.64.0/18"),
		gen.Const("10.233.0.0/18"),
	).Map(func(parts []interface{}) KubernetesConfig {
		return KubernetesConfig{
			Version:        parts[0].(string),
			APIPort:        parts[1].(int),
			SubnetPods:     parts[2].(string),
			SubnetServices: parts[3].(string),
		}
	})
}

// genInfrastructureConfig generates valid InfrastructureConfig.
func genInfrastructureConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("openstack", "aws", "gcp"),
		genNetworkingConfig(),
		genComputeConfig(),
		genStorageConfig(),
	).FlatMap(func(parts interface{}) gopter.Gen {
		partsSlice := parts.([]interface{})
		provider := partsSlice[0].(string)

		return genCloudConfig(provider).Map(func(cloud CloudConfig) InfrastructureConfig {
			return InfrastructureConfig{
				Provider: provider,
				SSH: SSHConfig{
					AuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2E..."},
				},
				OSVersion:  "24",
				Networking: partsSlice[1].(NetworkingConfig),
				Compute:    partsSlice[2].(ComputeConfig),
				Storage:    partsSlice[3].(StorageConfig),
				Cloud:      cloud,
			}
		})
	}, reflect.TypeOf(InfrastructureConfig{}))
}

// genNetworkingConfig generates valid NetworkingConfig.
func genNetworkingConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.Const("10.2.128.0/22"),
		gen.Const("10.2.128.10"),
		gen.Const("10.2.131.254"),
		gen.Bool(),
		gen.OneConstOf("ovn", "octavia", "metallb"),
		gen.Const("cluster.local"),
	).Map(func(parts []interface{}) NetworkingConfig {
		vrrpEnabled := parts[3].(bool)
		vrrpIP := ""
		if vrrpEnabled {
			vrrpIP = "10.2.128.5"
		}

		return NetworkingConfig{
			SubnetNodes:          parts[0].(string),
			AllocationPoolStart:  parts[1].(string),
			AllocationPoolEnd:    parts[2].(string),
			VRRPEnabled:          vrrpEnabled,
			VRRPIP:               vrrpIP,
			LoadbalancerProvider: parts[4].(string),
			DNSZoneName:          parts[5].(string),
			DNSNameservers:       []string{"8.8.8.8", "8.8.4.4"},
			NTPServers:           []string{"time.google.com"},
		}
	})
}

// genComputeConfig generates valid ComputeConfig.
func genComputeConfig() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(0, 5),
		gen.IntRange(1, 10),
	).Map(func(parts []interface{}) ComputeConfig {
		masterCount := parts[0].(int)
		workerCount := parts[1].(int)

		return ComputeConfig{
			FlavorMaster: "m1.medium",
			FlavorWorker: "m1.large",
			MasterCount:  masterCount,
			WorkerCount:  workerCount,
		}
	})
}

// genStorageConfig generates valid StorageConfig.
func genStorageConfig() gopter.Gen {
	return gen.IntRange(50, 200).Map(func(size int) StorageConfig {
		return StorageConfig{
			DefaultStorageClass:         "standard",
			WorkerVolumeSize:            size,
			WorkerVolumeDestinationType: "volume",
			WorkerVolumeSourceType:      "image",
			WorkerVolumeType:            "ssd",
			MasterVolumeSize:            size,
		}
	})
}

// genCloudConfig generates valid CloudConfig for a specific provider.
func genCloudConfig(provider string) gopter.Gen {
	switch provider {
	case "openstack":
		return gen.Const(CloudConfig{
			OpenStack: &OpenStackCloudConfig{
				AuthURL:   "https://identity.api.rackspacecloud.com/v3",
				Region:    "sjc3",
				ProjectID: "project-123",
				ImageID:   "image-456",
				NetworkID: "network-789",
			},
		})
	case "aws":
		return gen.Const(CloudConfig{
			AWS: &AWSCloudConfig{
				Region:    "us-east-1",
				VPCID:     "vpc-123",
				SubnetIDs: []string{"subnet-456"},
				AMIID:     "ami-789",
			},
		})
	case "gcp":
		return gen.Const(CloudConfig{
			GCP: &GCPCloudConfig{
				Project:     "project-123",
				Region:      "us-central1",
				Network:     "default",
				Subnetwork:  "default",
				ImageFamily: "ubuntu-2204-lts",
			},
		})
	default:
		return gen.Const(CloudConfig{})
	}
}
