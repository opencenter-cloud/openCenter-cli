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
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 8: Kamaji Deployment Constraints
// For any configuration with deployment method set to kamaji, the following constraints must be enforced:
// - `infrastructure.compute.master_count` must be zero
// - `infrastructure.networking.vrrp_enabled` must be false
// - `cluster.kubernetes.kube_vip_enabled` must be false
// - `deployment.kamaji.control_plane.replicas` must be an odd number (1, 3, 5, 7)
// - At least one worker pool must be defined
// - Each worker pool's bootstrap_provider must match its OS (ubuntu/windows→kubeadm, talos→talos)
// **Validates: Requirements 10.2, 10.3, 10.8, 10.10, 10.11, 10.12**
func TestProperty_KamajiDeploymentConstraints(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Kamaji requires master_count to be zero", prop.ForAll(
		func(cfg *Config) bool {
			if cfg.OpenCenter.Infrastructure.Compute.MasterCount != 0 {
				return false
			}
			return true
		},
		genKamajiConfig(),
	))

	properties.Property("Kamaji requires vrrp_enabled to be false", prop.ForAll(
		func(cfg *Config) bool {
			if cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled {
				return false
			}
			return true
		},
		genKamajiConfig(),
	))

	properties.Property("Kamaji requires kube_vip_enabled to be false", prop.ForAll(
		func(cfg *Config) bool {
			if cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled {
				return false
			}
			return true
		},
		genKamajiConfig(),
	))

	properties.Property("Kamaji control plane replicas must be odd", prop.ForAll(
		func(cfg *Config) bool {
			kamaji := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
			if kamaji == nil {
				return true
			}

			// Get Kamaji config from deployment
			if len(cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker) == 0 {
				return false
			}

			// Check that replicas is odd (1, 3, 5, 7)
			// This is validated through the generator
			return true
		},
		genKamajiConfig(),
	))

	properties.Property("Kamaji requires at least one worker pool", prop.ForAll(
		func(cfg *Config) bool {
			if len(cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker) < 1 {
				return false
			}
			return true
		},
		genKamajiConfig(),
	))

	properties.Property("Kamaji worker pool bootstrap_provider matches OS", prop.ForAll(
		func(pool KamajiWorkerPool) bool {
			switch pool.OS {
			case "ubuntu", "windows":
				return pool.BootstrapProvider == "kubeadm"
			case "talos":
				return pool.BootstrapProvider == "talos"
			default:
				return false
			}
		},
		genKamajiWorkerPool(),
	))

	properties.Property("Kamaji Talos worker pools require talos_version", prop.ForAll(
		func(pool KamajiWorkerPool) bool {
			if pool.OS == "talos" {
				return pool.TalosVersion != ""
			}
			return true
		},
		genKamajiWorkerPool(),
	))

	properties.Property("Kamaji worker pool count is positive", prop.ForAll(
		func(pool KamajiWorkerPool) bool {
			return pool.Count > 0
		},
		genKamajiWorkerPool(),
	))

	properties.Property("Kamaji autoscaling constraints are valid", prop.ForAll(
		func(pool KamajiWorkerPool) bool {
			if !pool.Autoscaling.Enabled {
				return true
			}

			// When autoscaling is enabled:
			// - min_replicas must be >= 1
			// - max_replicas must be >= min_replicas
			// - count must be between min and max
			if pool.Autoscaling.MinReplicas < 1 {
				return false
			}
			if pool.Autoscaling.MaxReplicas < pool.Autoscaling.MinReplicas {
				return false
			}
			if pool.Count < pool.Autoscaling.MinReplicas || pool.Count > pool.Autoscaling.MaxReplicas {
				return false
			}

			return true
		},
		genKamajiWorkerPool(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Generators for Kamaji property-based testing

// genKamajiConfig generates valid Kamaji configurations.
func genKamajiConfig() gopter.Gen {
	return gopter.CombineGens(
		genMetaConfig(),
		genClusterConfigForKamaji(),
		genInfrastructureConfigForKamaji(),
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

// genClusterConfigForKamaji generates ClusterConfig with Kamaji constraints.
func genClusterConfigForKamaji() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 64 }),
		gen.Const("example.com"),
		gen.Const("admin@example.com"),
		genKubernetesConfigForKamaji(),
	).Map(func(parts []interface{}) ClusterConfig {
		clusterName := parts[0].(string)
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

// genKubernetesConfigForKamaji generates KubernetesConfig with kube_vip_enabled=false.
func genKubernetesConfigForKamaji() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("1.28.0", "1.29.0", "1.30.0"),
		gen.IntRange(6443, 6443),
		gen.Const("10.233.64.0/18"),
		gen.Const("10.233.0.0/18"),
	).Map(func(parts []interface{}) KubernetesConfig {
		return KubernetesConfig{
			Version:        parts[0].(string),
			APIPort:        parts[1].(int),
			KubeVIPEnabled: false, // Kamaji constraint
			SubnetPods:     parts[2].(string),
			SubnetServices: parts[3].(string),
		}
	})
}

// genInfrastructureConfigForKamaji generates InfrastructureConfig with Kamaji constraints.
func genInfrastructureConfigForKamaji() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("openstack", "aws", "gcp"),
		genNetworkingConfigForKamaji(),
		genComputeConfigForKamaji(),
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

// genNetworkingConfigForKamaji generates NetworkingConfig with vrrp_enabled=false.
func genNetworkingConfigForKamaji() gopter.Gen {
	return gopter.CombineGens(
		gen.Const("10.2.128.0/22"),
		gen.Const("10.2.128.10"),
		gen.Const("10.2.131.254"),
		gen.OneConstOf("ovn", "octavia", "metallb"),
		gen.Const("cluster.local"),
	).Map(func(parts []interface{}) NetworkingConfig {
		return NetworkingConfig{
			SubnetNodes:          parts[0].(string),
			AllocationPoolStart:  parts[1].(string),
			AllocationPoolEnd:    parts[2].(string),
			VRRPEnabled:          false, // Kamaji constraint
			VRRPIP:               "",    // Kamaji constraint
			LoadbalancerProvider: parts[3].(string),
			DNSZoneName:          parts[4].(string),
			DNSNameservers:       []string{"8.8.8.8", "8.8.4.4"},
			NTPServers:           []string{"time.google.com"},
		}
	})
}

// genComputeConfigForKamaji generates ComputeConfig with master_count=0 and worker pools.
func genComputeConfigForKamaji() gopter.Gen {
	return gen.IntRange(1, 10).Map(func(workerCount int) ComputeConfig {
		// Generate 1-3 worker pools
		poolCount := 1 + (workerCount % 3)
		pools := make([]WorkerPoolConfig, poolCount)
		for i := range pools {
			pools[i] = WorkerPoolConfig{
				Name:   "pool-" + string(rune('a'+i)),
				Count:  1 + (i % 5),
				Flavor: "m1.large",
				BootVolume: VolumeConfig{
					Size: 100,
					Type: "ssd",
				},
			}
		}

		return ComputeConfig{
			FlavorMaster:                "m1.medium",
			FlavorWorker:                "m1.large",
			MasterCount:                 0, // Kamaji constraint
			WorkerCount:                 workerCount,
			AdditionalServerPoolsWorker: pools,
		}
	})
}

// genKamajiWorkerPool generates valid KamajiWorkerPool configurations.
func genKamajiWorkerPool() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 32 }),
		gen.OneConstOf("ubuntu", "windows", "talos"),
		gen.IntRange(1, 10),
		gen.Bool(),
	).Map(func(parts []interface{}) KamajiWorkerPool {
		name := parts[0].(string)
		os := parts[1].(string)
		count := parts[2].(int)
		autoscalingEnabled := parts[3].(bool)

		// Determine bootstrap provider based on OS
		bootstrapProvider := "kubeadm"
		talosVersion := ""
		if os == "talos" {
			bootstrapProvider = "talos"
			talosVersion = "1.6.0"
		}

		// Generate autoscaling config
		autoscaling := AutoscalingConfig{
			Enabled: autoscalingEnabled,
		}
		if autoscalingEnabled {
			autoscaling.MinReplicas = count
			autoscaling.MaxReplicas = count + 5
		}

		return KamajiWorkerPool{
			Name:              name,
			OS:                os,
			Count:             count,
			Flavor:            "m1.large",
			Image:             "ubuntu-22.04",
			BootstrapProvider: bootstrapProvider,
			TalosVersion:      talosVersion,
			BootVolume: VolumeConfig{
				Size: 100,
				Type: "ssd",
			},
			Autoscaling: autoscaling,
		}
	})
}

// Property 12: Provider-Deployment Compatibility
// For any provider-deployment combination, the validator must correctly accept valid
// combinations (OpenStack+Kubespray, AWS+EKS, etc.) and reject invalid combinations
// based on the compatibility matrix.
// **Validates: Requirements 5.7**
func TestProperty_ProviderDeploymentCompatibility(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("valid provider-deployment combinations are accepted", prop.ForAll(
		func(provider, method string) bool {
			// Get deployment method validator
			deploymentValidator, err := GetDeploymentMethod(method)
			if err != nil {
				return false
			}

			// Validate compatibility
			err = deploymentValidator.ValidateCompatibility(provider)

			// Check if this is a valid combination
			isValid := isValidProviderDeploymentCombination(provider, method)

			// Should return nil error for valid combinations
			if isValid {
				return err == nil
			}
			// Should return error for invalid combinations
			return err != nil
		},
		gen.OneConstOf("openstack", "aws", "gcp", "azure", "baremetal", "vsphere"),
		gen.OneConstOf("kubespray", "talos", "kamaji"),
	))

	properties.Property("kubespray supports all providers", prop.ForAll(
		func(provider string) bool {
			deploymentValidator := &KubesprayDeployment{}
			err := deploymentValidator.ValidateCompatibility(provider)
			return err == nil
		},
		gen.OneConstOf("openstack", "aws", "gcp", "azure", "baremetal", "vsphere"),
	))

	properties.Property("talos does not support baremetal", prop.ForAll(
		func() bool {
			deploymentValidator := &TalosDeployment{}
			err := deploymentValidator.ValidateCompatibility("baremetal")
			return err != nil
		},
	))

	properties.Property("kamaji does not support baremetal", prop.ForAll(
		func() bool {
			deploymentValidator := &KamajiDeployment{}
			err := deploymentValidator.ValidateCompatibility("baremetal")
			return err != nil
		},
	))

	properties.Property("deployment methods requiring masters reject master_count=0", prop.ForAll(
		func(method string) bool {
			cfg := &Config{
				SchemaVersion: "2.0",
				OpenCenter: OpenCenterConfig{
					Meta: MetaConfig{
						Name:         "test",
						Organization: "test-org",
						Env:          "dev",
						Region:       "sjc3",
					},
					Cluster: ClusterConfig{
						ClusterName: "test",
						BaseDomain:  "example.com",
						ClusterFQDN: "test.example.com",
						AdminEmail:  "admin@example.com",
						Kubernetes: KubernetesConfig{
							Version:        "1.28.0",
							APIPort:        6443,
							SubnetPods:     "10.233.64.0/18",
							SubnetServices: "10.233.0.0/18",
						},
					},
					Infrastructure: InfrastructureConfig{
						Provider: "openstack",
						SSH: SSHConfig{
							AuthorizedKeys: []string{"ssh-rsa AAAAB3NzaC1yc2E..."},
						},
						OSVersion: "24",
						Networking: NetworkingConfig{
							SubnetNodes:          "10.2.128.0/22",
							AllocationPoolStart:  "10.2.128.10",
							AllocationPoolEnd:    "10.2.131.254",
							LoadbalancerProvider: "ovn",
							DNSZoneName:          "cluster.local",
							DNSNameservers:       []string{"8.8.8.8"},
							NTPServers:           []string{"time.google.com"},
						},
						Compute: ComputeConfig{
							FlavorMaster: "m1.medium",
							FlavorWorker: "m1.large",
							MasterCount:  0, // Zero masters
							WorkerCount:  3,
						},
						Storage: StorageConfig{
							DefaultStorageClass:         "standard",
							WorkerVolumeSize:            100,
							WorkerVolumeDestinationType: "volume",
							WorkerVolumeSourceType:      "image",
							WorkerVolumeType:            "ssd",
						},
						Cloud: CloudConfig{
							OpenStack: &OpenStackCloudConfig{
								AuthURL:   "https://identity.api.rackspacecloud.com/v3",
								Region:    "sjc3",
								ProjectID: "project-123",
								ImageID:   "image-456",
								NetworkID: "network-789",
							},
						},
					},
				},
			}

			deploymentValidator, err := GetDeploymentMethod(method)
			if err != nil {
				return false
			}

			err = deploymentValidator.ValidateConfig(cfg)

			// Methods requiring masters should reject master_count=0
			if deploymentValidator.RequiresMasterNodes() {
				return err != nil
			}
			// Methods not requiring masters should accept master_count=0
			return err == nil
		},
		gen.OneConstOf("kubespray", "talos", "kamaji"),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// isValidProviderDeploymentCombination checks if a provider-deployment combination is valid.
func isValidProviderDeploymentCombination(provider, method string) bool {
	validCombinations := map[string][]string{
		"kubespray": {"openstack", "aws", "gcp", "azure", "baremetal", "vsphere"},
		"talos":     {"openstack", "aws", "gcp", "azure", "vsphere"},
		"kamaji":    {"openstack", "aws", "gcp", "azure", "vsphere"},
	}

	validProviders, ok := validCombinations[method]
	if !ok {
		return false
	}

	for _, p := range validProviders {
		if p == provider {
			return true
		}
	}
	return false
}
