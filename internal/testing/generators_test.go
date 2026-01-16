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

package testing

import (
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigGenerator_GenerateConfig(t *testing.T) {
	gen := NewConfigGenerator(42)

	t.Run("generates valid OpenStack config", func(t *testing.T) {
		cfg := gen.GenerateConfig("openstack")

		assert.Equal(t, "openstack", cfg.OpenCenter.Infrastructure.Provider)
		assert.NotEmpty(t, cfg.OpenCenter.Meta.Organization)
		assert.NotEmpty(t, cfg.OpenCenter.Meta.Name)
		assert.NotEmpty(t, cfg.OpenCenter.Cluster.Kubernetes.Version)

		// Verify OpenStack-specific fields
		require.NotNil(t, cfg.OpenCenter.Infrastructure.Cloud.OpenStack)
		assert.NotEmpty(t, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL)
		assert.NotEmpty(t, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
		assert.NotEmpty(t, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName)
	})

	t.Run("generates valid AWS config", func(t *testing.T) {
		cfg := gen.GenerateConfig("aws")

		assert.Equal(t, "aws", cfg.OpenCenter.Infrastructure.Provider)
		assert.NotEmpty(t, cfg.OpenCenter.Meta.Organization)
		assert.NotEmpty(t, cfg.OpenCenter.Meta.Name)

		// Verify AWS-specific fields
		require.NotNil(t, cfg.OpenCenter.Infrastructure.Cloud.AWS)
		assert.NotEmpty(t, cfg.OpenCenter.Infrastructure.Cloud.AWS.Region)
	})

	t.Run("generates valid networking config", func(t *testing.T) {
		cfg := gen.GenerateConfig("openstack")

		assert.NotEmpty(t, cfg.OpenCenter.Cluster.Kubernetes.Networking.SubnetPods)
		assert.NotEmpty(t, cfg.OpenCenter.Cluster.Kubernetes.Networking.SubnetServices)
		assert.NotEmpty(t, cfg.OpenCenter.Cluster.Kubernetes.Networking.DNSNameservers)
		assert.Greater(t, len(cfg.OpenCenter.Cluster.Kubernetes.Networking.DNSNameservers), 0)
	})

	t.Run("generates valid secrets config", func(t *testing.T) {
		cfg := gen.GenerateConfig("openstack")

		assert.NotEmpty(t, cfg.Secrets.SopsAgeKeyFile)
		assert.NotEmpty(t, cfg.Secrets.SSHKey.Public)
	})

	t.Run("generates valid security config", func(t *testing.T) {
		cfg := gen.GenerateConfig("openstack")

		assert.True(t, cfg.Security.K8sHardening)
		assert.True(t, cfg.Security.OSHardening)
	})
}

func TestConfigGenerator_Deterministic(t *testing.T) {
	// Same seed should produce same results
	gen1 := NewConfigGenerator(123)
	gen2 := NewConfigGenerator(123)

	cfg1 := gen1.GenerateConfig("openstack")
	cfg2 := gen2.GenerateConfig("openstack")

	assert.Equal(t, cfg1.OpenCenter.Meta.Organization, cfg2.OpenCenter.Meta.Organization)
	assert.Equal(t, cfg1.OpenCenter.Meta.Name, cfg2.OpenCenter.Meta.Name)
	assert.Equal(t, cfg1.OpenCenter.Cluster.Kubernetes.Version, cfg2.OpenCenter.Cluster.Kubernetes.Version)
}

func TestConfigGenerator_Variety(t *testing.T) {
	// Different seeds should produce different results
	gen1 := NewConfigGenerator(123)
	gen2 := NewConfigGenerator(456)

	cfg1 := gen1.GenerateConfig("openstack")
	cfg2 := gen2.GenerateConfig("openstack")

	// At least some fields should be different
	different := cfg1.OpenCenter.Meta.Organization != cfg2.OpenCenter.Meta.Organization ||
		cfg1.OpenCenter.Meta.Name != cfg2.OpenCenter.Meta.Name ||
		cfg1.OpenCenter.Cluster.Kubernetes.Version != cfg2.OpenCenter.Cluster.Kubernetes.Version

	assert.True(t, different, "Different seeds should produce different configurations")
}

func TestTemplateDataGenerator_GenerateTemplateData(t *testing.T) {
	gen := NewTemplateDataGenerator(42)

	t.Run("generates complete template data", func(t *testing.T) {
		data := gen.GenerateTemplateData()

		assert.NotEmpty(t, data["ClusterName"])
		assert.NotEmpty(t, data["Namespace"])
		assert.NotEmpty(t, data["Version"])
		assert.NotEmpty(t, data["Image"])
		assert.NotEmpty(t, data["Environment"])

		// Verify numeric fields
		replicas, ok := data["Replicas"].(int)
		assert.True(t, ok)
		assert.Greater(t, replicas, 0)

		port, ok := data["Port"].(int)
		assert.True(t, ok)
		assert.GreaterOrEqual(t, port, 8000)
		assert.Less(t, port, 9000)
	})

	t.Run("generates valid labels", func(t *testing.T) {
		data := gen.GenerateTemplateData()

		labels, ok := data["Labels"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, labels["app"])
		assert.NotEmpty(t, labels["environment"])
		assert.NotEmpty(t, labels["version"])
	})

	t.Run("generates valid annotations", func(t *testing.T) {
		data := gen.GenerateTemplateData()

		annotations, ok := data["Annotations"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, annotations["description"])
		assert.NotEmpty(t, annotations["timestamp"])
	})

	t.Run("generates valid resources", func(t *testing.T) {
		data := gen.GenerateTemplateData()

		resources, ok := data["Resources"].(map[string]interface{})
		require.True(t, ok)

		requests, ok := resources["requests"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, requests["cpu"])
		assert.NotEmpty(t, requests["memory"])

		limits, ok := resources["limits"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, limits["cpu"])
		assert.NotEmpty(t, limits["memory"])
	})
}

func TestServiceDataGenerator_GenerateServiceDefinition(t *testing.T) {
	gen := NewServiceDataGenerator(42)

	t.Run("generates complete service definition", func(t *testing.T) {
		svc := gen.GenerateServiceDefinition()

		assert.NotEmpty(t, svc["name"])
		assert.NotEmpty(t, svc["type"])
		assert.NotEmpty(t, svc["version"])

		enabled, ok := svc["enabled"].(bool)
		assert.True(t, ok)
		_ = enabled // enabled can be true or false

		dependencies, ok := svc["dependencies"].([]string)
		assert.True(t, ok)
		_ = dependencies // dependencies can be empty or have items
	})

	t.Run("generates valid service config", func(t *testing.T) {
		svc := gen.GenerateServiceDefinition()

		config, ok := svc["config"].(map[string]interface{})
		require.True(t, ok)

		replicas, ok := config["replicas"].(int)
		assert.True(t, ok)
		assert.Greater(t, replicas, 0)

		assert.NotEmpty(t, config["namespace"])

		resources, ok := config["resources"].(map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, resources["cpu"])
		assert.NotEmpty(t, resources["memory"])
	})
}

func TestGitOpsDataGenerator_GenerateGitOpsConfig(t *testing.T) {
	gen := NewGitOpsDataGenerator(42)

	t.Run("generates complete GitOps config", func(t *testing.T) {
		cfg := gen.GenerateGitOpsConfig()

		enabled, ok := cfg["enabled"].(bool)
		assert.True(t, ok)
		assert.True(t, enabled)

		assert.NotEmpty(t, cfg["repository"])
		assert.NotEmpty(t, cfg["branch"])
		assert.NotEmpty(t, cfg["path"])

		sync, ok := cfg["sync"].(map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, sync["interval"])
		assert.NotEmpty(t, sync["timeout"])
	})

	t.Run("generates valid repository URL", func(t *testing.T) {
		cfg := gen.GenerateGitOpsConfig()

		repo, ok := cfg["repository"].(string)
		require.True(t, ok)
		assert.Contains(t, repo, "github.com")
		assert.Contains(t, repo, ".git")
	})
}

func TestGenerators_Realistic(t *testing.T) {
	t.Run("config generator produces realistic values", func(t *testing.T) {
		gen := NewConfigGenerator(42)
		cfg := gen.GenerateConfig("openstack")

		// Kubernetes version should be realistic
		assert.Regexp(t, `^1\.\d+\.\d+$`, cfg.OpenCenter.Cluster.Kubernetes.Version)

		// DNS servers should be valid IPs
		for _, dns := range cfg.OpenCenter.Cluster.Kubernetes.Networking.DNSNameservers {
			assert.Regexp(t, `^\d+\.\d+\.\d+\.\d+$`, dns)
		}

		// SSH key should have correct format
		assert.Contains(t, cfg.Secrets.SSHKey.Public, "ssh-rsa")
	})

	t.Run("template generator produces realistic values", func(t *testing.T) {
		gen := NewTemplateDataGenerator(42)
		data := gen.GenerateTemplateData()

		// Version should follow semver pattern
		version, ok := data["Version"].(string)
		require.True(t, ok)
		assert.Regexp(t, `^v\d+\.\d+\.\d+$`, version)

		// Port should be in valid range
		port, ok := data["Port"].(int)
		require.True(t, ok)
		assert.GreaterOrEqual(t, port, 8000)
		assert.Less(t, port, 9000)

		// Image should have valid format
		image, ok := data["Image"].(string)
		require.True(t, ok)
		assert.Contains(t, image, ":")
	})
}

func BenchmarkConfigGenerator(b *testing.B) {
	gen := NewConfigGenerator(42)

	b.Run("GenerateConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateConfig("openstack")
		}
	})
}

func BenchmarkTemplateDataGenerator(b *testing.B) {
	gen := NewTemplateDataGenerator(42)

	b.Run("GenerateTemplateData", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateTemplateData()
		}
	})
}

func BenchmarkServiceDataGenerator(b *testing.B) {
	gen := NewServiceDataGenerator(42)

	b.Run("GenerateServiceDefinition", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateServiceDefinition()
		}
	})
}

func BenchmarkGitOpsDataGenerator(b *testing.B) {
	gen := NewGitOpsDataGenerator(42)

	b.Run("GenerateGitOpsConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateGitOpsConfig()
		}
	})
}

func TestNetworkingDataGenerator_GenerateNetworkingConfig(t *testing.T) {
	gen := NewNetworkingDataGenerator(42)

	t.Run("generates complete networking config", func(t *testing.T) {
		cfg := gen.GenerateNetworkingConfig()

		assert.NotEmpty(t, cfg["subnet_nodes"])
		assert.NotEmpty(t, cfg["subnet_pods"])
		assert.NotEmpty(t, cfg["subnet_services"])
		assert.NotEmpty(t, cfg["dns_servers"])
		assert.NotEmpty(t, cfg["mtu"])
		assert.NotEmpty(t, cfg["load_balancer"])

		// Verify DNS servers is a slice
		dnsServers, ok := cfg["dns_servers"].([]string)
		require.True(t, ok)
		assert.Greater(t, len(dnsServers), 0)

		// Verify MTU is in valid range
		mtu, ok := cfg["mtu"].(int)
		require.True(t, ok)
		assert.Contains(t, []int{1500, 9000, 9100}, mtu)
	})

	t.Run("generates valid subnet formats", func(t *testing.T) {
		cfg := gen.GenerateNetworkingConfig()

		subnetNodes, ok := cfg["subnet_nodes"].(string)
		require.True(t, ok)
		assert.Regexp(t, `^\d+\.\d+\.\d+\.\d+/\d+$`, subnetNodes)

		subnetPods, ok := cfg["subnet_pods"].(string)
		require.True(t, ok)
		assert.Regexp(t, `^\d+\.\d+\.\d+\.\d+/\d+$`, subnetPods)
	})
}

func TestInfrastructureDataGenerator_GenerateNodePool(t *testing.T) {
	gen := NewInfrastructureDataGenerator(42)

	t.Run("generates complete node pool", func(t *testing.T) {
		pool := gen.GenerateNodePool()

		assert.NotEmpty(t, pool["name"])
		assert.NotEmpty(t, pool["flavor"])

		count, ok := pool["count"].(int)
		require.True(t, ok)
		assert.Greater(t, count, 0)

		diskSize, ok := pool["disk_size"].(int)
		require.True(t, ok)
		assert.Greater(t, diskSize, 0)

		labels, ok := pool["labels"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, labels)
	})

	t.Run("generates valid node labels", func(t *testing.T) {
		pool := gen.GenerateNodePool()

		labels, ok := pool["labels"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, labels["node-role"])
		assert.NotEmpty(t, labels["zone"])
	})

	t.Run("generates optional auto-scaling", func(t *testing.T) {
		// Generate multiple pools to test randomness
		hasAutoScaling := false
		noAutoScaling := false

		for i := 0; i < 20; i++ {
			gen := NewInfrastructureDataGenerator(int64(i))
			pool := gen.GenerateNodePool()

			if pool["auto_scaling"] != nil {
				hasAutoScaling = true
				autoScaling, ok := pool["auto_scaling"].(map[string]interface{})
				require.True(t, ok, "auto_scaling should be map[string]interface{}")

				minNodes, ok := autoScaling["min_nodes"].(int)
				require.True(t, ok, "min_nodes should be int")
				assert.Greater(t, minNodes, 0)

				maxNodes, ok := autoScaling["max_nodes"].(int)
				require.True(t, ok, "max_nodes should be int")
				assert.Greater(t, maxNodes, minNodes)
			} else {
				noAutoScaling = true
			}
		}

		// Verify we get both cases
		assert.True(t, hasAutoScaling, "Should generate some pools with auto-scaling")
		assert.True(t, noAutoScaling, "Should generate some pools without auto-scaling")
	})
}

func TestSecurityDataGenerator_GenerateSecurityConfig(t *testing.T) {
	gen := NewSecurityDataGenerator(42)

	t.Run("generates complete security config", func(t *testing.T) {
		cfg := gen.GenerateSecurityConfig()

		assert.NotNil(t, cfg["k8s_hardening"])
		assert.NotNil(t, cfg["os_hardening"])
		assert.NotEmpty(t, cfg["pod_security"])
		assert.NotNil(t, cfg["network_policies"])
		assert.NotNil(t, cfg["encryption_at_rest"])
		assert.NotNil(t, cfg["audit_logging"])

		rbacEnabled, ok := cfg["rbac_enabled"].(bool)
		require.True(t, ok)
		assert.True(t, rbacEnabled, "RBAC should always be enabled")
	})

	t.Run("generates valid pod security level", func(t *testing.T) {
		cfg := gen.GenerateSecurityConfig()

		podSecurity, ok := cfg["pod_security"].(string)
		require.True(t, ok)
		assert.Contains(t, []string{"privileged", "baseline", "restricted"}, podSecurity)
	})

	t.Run("generates admission controllers", func(t *testing.T) {
		cfg := gen.GenerateSecurityConfig()

		controllers, ok := cfg["admission_controllers"].([]string)
		require.True(t, ok)
		assert.Greater(t, len(controllers), 0)
		assert.LessOrEqual(t, len(controllers), 5)
	})
}

func TestWorkloadDataGenerator_GenerateDeployment(t *testing.T) {
	gen := NewWorkloadDataGenerator(42)

	t.Run("generates complete deployment", func(t *testing.T) {
		dep := gen.GenerateDeployment()

		assert.NotEmpty(t, dep["name"])
		assert.NotEmpty(t, dep["namespace"])
		assert.NotEmpty(t, dep["image"])

		replicas, ok := dep["replicas"].(int)
		require.True(t, ok)
		assert.Greater(t, replicas, 0)
	})

	t.Run("generates valid ports", func(t *testing.T) {
		dep := gen.GenerateDeployment()

		ports, ok := dep["ports"].([]map[string]interface{})
		require.True(t, ok)
		assert.Greater(t, len(ports), 0)

		for _, port := range ports {
			assert.NotEmpty(t, port["name"])
			assert.NotEmpty(t, port["containerPort"])
			assert.NotEmpty(t, port["protocol"])
		}
	})

	t.Run("generates valid environment variables", func(t *testing.T) {
		dep := gen.GenerateDeployment()

		envVars, ok := dep["env"].([]map[string]string)
		require.True(t, ok)
		assert.Greater(t, len(envVars), 0)

		for _, env := range envVars {
			assert.NotEmpty(t, env["name"])
			assert.NotEmpty(t, env["value"])
		}
	})

	t.Run("generates valid resources", func(t *testing.T) {
		dep := gen.GenerateDeployment()

		resources, ok := dep["resources"].(map[string]interface{})
		require.True(t, ok)

		requests, ok := resources["requests"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, requests["cpu"])
		assert.NotEmpty(t, requests["memory"])

		limits, ok := resources["limits"].(map[string]string)
		require.True(t, ok)
		assert.NotEmpty(t, limits["cpu"])
		assert.NotEmpty(t, limits["memory"])
	})

	t.Run("generates valid probes", func(t *testing.T) {
		dep := gen.GenerateDeployment()

		probes, ok := dep["probes"].(map[string]interface{})
		require.True(t, ok)

		liveness, ok := probes["liveness"].(map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, liveness["httpGet"])
		assert.NotEmpty(t, liveness["initialDelaySeconds"])
		assert.NotEmpty(t, liveness["periodSeconds"])

		readiness, ok := probes["readiness"].(map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, readiness["httpGet"])
		assert.NotEmpty(t, readiness["initialDelaySeconds"])
		assert.NotEmpty(t, readiness["periodSeconds"])
	})
}

func TestGenerators_RealisticScenarios(t *testing.T) {
	t.Run("networking generator produces realistic scenarios", func(t *testing.T) {
		gen := NewNetworkingDataGenerator(42)
		cfg := gen.GenerateNetworkingConfig()

		// Verify realistic subnet ranges
		subnetNodes, _ := cfg["subnet_nodes"].(string)
		assert.Contains(t, subnetNodes, "10.0")

		subnetPods, _ := cfg["subnet_pods"].(string)
		assert.Contains(t, subnetPods, "10.42")

		// Verify realistic DNS servers
		dnsServers, _ := cfg["dns_servers"].([]string)
		for _, dns := range dnsServers {
			assert.Regexp(t, `^\d+\.\d+\.\d+\.\d+$`, dns)
		}
	})

	t.Run("infrastructure generator produces realistic scenarios", func(t *testing.T) {
		gen := NewInfrastructureDataGenerator(42)
		pool := gen.GenerateNodePool()

		// Verify realistic node counts
		count, _ := pool["count"].(int)
		assert.GreaterOrEqual(t, count, 1)
		assert.LessOrEqual(t, count, 11)

		// Verify realistic disk sizes
		diskSize, _ := pool["disk_size"].(int)
		assert.Contains(t, []int{20, 40, 80, 100, 200, 500}, diskSize)

		// Verify realistic flavors
		flavor, _ := pool["flavor"].(string)
		assert.Regexp(t, `^[mrc]1\.(small|medium|large|xlarge)$`, flavor)
	})

	t.Run("workload generator produces realistic scenarios", func(t *testing.T) {
		gen := NewWorkloadDataGenerator(42)
		dep := gen.GenerateDeployment()

		// Verify realistic image format
		image, _ := dep["image"].(string)
		assert.Contains(t, image, ":")

		// Verify realistic replica counts
		replicas, _ := dep["replicas"].(int)
		assert.GreaterOrEqual(t, replicas, 1)
		assert.LessOrEqual(t, replicas, 6)

		// Verify realistic resource requests
		resources, _ := dep["resources"].(map[string]interface{})
		requests, _ := resources["requests"].(map[string]string)
		cpuRequest := requests["cpu"]
		assert.Regexp(t, `^\d+m$`, cpuRequest)
		memRequest := requests["memory"]
		assert.Regexp(t, `^\d+Mi$`, memRequest)
	})
}

func BenchmarkNetworkingDataGenerator(b *testing.B) {
	gen := NewNetworkingDataGenerator(42)

	b.Run("GenerateNetworkingConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateNetworkingConfig()
		}
	})
}

func BenchmarkInfrastructureDataGenerator(b *testing.B) {
	gen := NewInfrastructureDataGenerator(42)

	b.Run("GenerateNodePool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateNodePool()
		}
	})
}

func BenchmarkSecurityDataGenerator(b *testing.B) {
	gen := NewSecurityDataGenerator(42)

	b.Run("GenerateSecurityConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateSecurityConfig()
		}
	})
}

func BenchmarkWorkloadDataGenerator(b *testing.B) {
	gen := NewWorkloadDataGenerator(42)

	b.Run("GenerateDeployment", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateDeployment()
		}
	})
}

func TestConfigGenerator_GenerateMinimalConfig(t *testing.T) {
	gen := NewConfigGenerator(42)

	t.Run("generates minimal valid config", func(t *testing.T) {
		cfg := gen.GenerateMinimalConfig("openstack")

		assert.Equal(t, "openstack", cfg.OpenCenter.Infrastructure.Provider)
		assert.NotEmpty(t, cfg.OpenCenter.Meta.Name)
		assert.Equal(t, "development", cfg.OpenCenter.Meta.Env)
		assert.Equal(t, "local", cfg.OpenTofu.Backend.Type)
		assert.Empty(t, cfg.OpenCenter.Services)
	})

	t.Run("minimal config has required fields", func(t *testing.T) {
		cfg := gen.GenerateMinimalConfig("aws")

		assert.NotEmpty(t, cfg.OpenCenter.Meta.Organization)
		assert.NotEmpty(t, cfg.OpenCenter.Cluster.ClusterName)
		assert.NotEmpty(t, cfg.OpenCenter.Cluster.Kubernetes.Version)
		assert.NotEmpty(t, cfg.Secrets.SopsAgeKeyFile)
	})
}

func TestConfigGenerator_GenerateComplexConfig(t *testing.T) {
	gen := NewConfigGenerator(42)

	t.Run("generates complex config with all services", func(t *testing.T) {
		cfg := gen.GenerateComplexConfig("openstack")

		assert.Equal(t, "openstack", cfg.OpenCenter.Infrastructure.Provider)
		assert.NotEmpty(t, cfg.OpenCenter.Services)

		// Verify all services are enabled
		services := []string{"cert-manager", "prometheus-stack", "loki", "velero", "keycloak", "headlamp", "weave-gitops"}
		for _, svc := range services {
			assert.Contains(t, cfg.OpenCenter.Services, svc)
			svcConfig, ok := cfg.OpenCenter.Services[svc].(map[string]interface{})
			require.True(t, ok)
			assert.True(t, svcConfig["enabled"].(bool))
		}
	})

	t.Run("complex config has overrides", func(t *testing.T) {
		cfg := gen.GenerateComplexConfig("aws")

		assert.NotEmpty(t, cfg.Overrides)
		assert.Contains(t, cfg.Overrides, "kubernetes.apiserver.extraArgs")
		assert.Contains(t, cfg.Overrides, "networking.cni")
	})
}

func TestScenarioGenerator_GenerateProductionScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates complete production scenario", func(t *testing.T) {
		scenario := gen.GenerateProductionScenario("openstack")

		assert.NotNil(t, scenario["config"])
		assert.NotNil(t, scenario["node_pools"])
		assert.NotNil(t, scenario["security"])
		assert.NotNil(t, scenario["networking"])
		assert.Equal(t, "production", scenario["environment"])
		assert.True(t, scenario["high_availability"].(bool))
		assert.True(t, scenario["backup_enabled"].(bool))
		assert.True(t, scenario["monitoring_enabled"].(bool))
	})

	t.Run("production scenario has multiple node pools", func(t *testing.T) {
		scenario := gen.GenerateProductionScenario("aws")

		nodePools, ok := scenario["node_pools"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 3, len(nodePools), "Production should have 3 node pools")
	})
}

func TestScenarioGenerator_GenerateDevelopmentScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates minimal development scenario", func(t *testing.T) {
		scenario := gen.GenerateDevelopmentScenario("openstack")

		assert.NotNil(t, scenario["config"])
		assert.Equal(t, "development", scenario["environment"])
		assert.False(t, scenario["high_availability"].(bool))
		assert.False(t, scenario["backup_enabled"].(bool))
		assert.False(t, scenario["monitoring_enabled"].(bool))
	})

	t.Run("development scenario has single node pool", func(t *testing.T) {
		scenario := gen.GenerateDevelopmentScenario("aws")

		nodePools, ok := scenario["node_pools"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 1, len(nodePools), "Development should have 1 node pool")
	})
}

func TestScenarioGenerator_GenerateStagingScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates staging scenario", func(t *testing.T) {
		scenario := gen.GenerateStagingScenario("openstack")

		assert.NotNil(t, scenario["config"])
		assert.NotNil(t, scenario["security"])
		assert.Equal(t, "staging", scenario["environment"])
		assert.True(t, scenario["high_availability"].(bool))
	})

	t.Run("staging scenario has two node pools", func(t *testing.T) {
		scenario := gen.GenerateStagingScenario("aws")

		nodePools, ok := scenario["node_pools"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(nodePools), "Staging should have 2 node pools")
	})
}

func TestScenarioGenerator_GenerateMultiRegionScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates multi-region scenario", func(t *testing.T) {
		scenario := gen.GenerateMultiRegionScenario("aws")

		assert.True(t, scenario["multi_region"].(bool))
		assert.True(t, scenario["replication_enabled"].(bool))

		clusters, ok := scenario["clusters"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 3, len(clusters), "Should have 3 regional clusters")

		// Verify each cluster has different region
		regions := make(map[string]bool)
		for _, cluster := range clusters {
			region, ok := cluster["region"].(string)
			require.True(t, ok)
			assert.NotEmpty(t, region)
			regions[region] = true
		}
		assert.Equal(t, 3, len(regions), "Should have 3 unique regions")
	})
}

func TestScenarioGenerator_GenerateUpgradeScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates upgrade scenario", func(t *testing.T) {
		scenario := gen.GenerateUpgradeScenario("openstack")

		assert.NotNil(t, scenario["old_config"])
		assert.NotNil(t, scenario["new_config"])
		assert.Equal(t, "minor", scenario["upgrade_type"])
		assert.True(t, scenario["rollback_enabled"].(bool))

		oldConfig := scenario["old_config"].(config.Config)
		newConfig := scenario["new_config"].(config.Config)

		// Verify versions are different
		assert.NotEqual(t, oldConfig.OpenCenter.Cluster.Kubernetes.Version, newConfig.OpenCenter.Cluster.Kubernetes.Version)

		// Verify cluster identity is preserved
		assert.Equal(t, oldConfig.OpenCenter.Meta.Name, newConfig.OpenCenter.Meta.Name)
		assert.Equal(t, oldConfig.OpenCenter.Meta.Organization, newConfig.OpenCenter.Meta.Organization)
	})
}

func TestScenarioGenerator_GenerateMigrationScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates provider migration scenario", func(t *testing.T) {
		scenario := gen.GenerateMigrationScenario("openstack", "aws")

		assert.NotNil(t, scenario["source_config"])
		assert.NotNil(t, scenario["target_config"])
		assert.Equal(t, "provider", scenario["migration_type"])
		assert.True(t, scenario["data_migration_required"].(bool))

		sourceConfig := scenario["source_config"].(config.Config)
		targetConfig := scenario["target_config"].(config.Config)

		// Verify providers are different
		assert.Equal(t, "openstack", sourceConfig.OpenCenter.Infrastructure.Provider)
		assert.Equal(t, "aws", targetConfig.OpenCenter.Infrastructure.Provider)

		// Verify cluster identity is preserved
		assert.Equal(t, sourceConfig.OpenCenter.Meta.Name, targetConfig.OpenCenter.Meta.Name)
	})
}

func TestScenarioGenerator_GenerateDisasterRecoveryScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates DR scenario", func(t *testing.T) {
		scenario := gen.GenerateDisasterRecoveryScenario("aws")

		assert.NotNil(t, scenario["primary_config"])
		assert.NotNil(t, scenario["dr_config"])
		assert.True(t, scenario["replication_enabled"].(bool))
		assert.True(t, scenario["failover_enabled"].(bool))
		assert.Equal(t, 15, scenario["rpo_minutes"])
		assert.Equal(t, 30, scenario["rto_minutes"])

		primaryConfig := scenario["primary_config"].(config.Config)
		drConfig := scenario["dr_config"].(config.Config)

		// Verify different regions
		assert.NotEqual(t, primaryConfig.OpenCenter.Meta.Region, drConfig.OpenCenter.Meta.Region)

		// Verify DR naming convention
		assert.Contains(t, drConfig.OpenCenter.Meta.Name, "-dr")
	})
}

func TestScenarioGenerator_GenerateEdgeCaseScenario(t *testing.T) {
	t.Run("generates edge case scenarios", func(t *testing.T) {
		// Generate multiple scenarios to test different edge cases
		edgeCases := make(map[string]bool)

		for i := 0; i < 20; i++ {
			gen := NewScenarioGenerator(int64(i))
			scenario := gen.GenerateEdgeCaseScenario("openstack")

			assert.NotNil(t, scenario["config"])
			assert.NotEmpty(t, scenario["edge_case"])
			assert.Equal(t, "boundary", scenario["test_type"])

			edgeCase, ok := scenario["edge_case"].(string)
			require.True(t, ok)
			edgeCases[edgeCase] = true
		}

		// Verify we get variety of edge cases
		assert.Greater(t, len(edgeCases), 1, "Should generate different edge cases")
	})
}

func TestScenarioGenerator_GeneratePerformanceTestScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates performance test scenario", func(t *testing.T) {
		scenario := gen.GeneratePerformanceTestScenario("openstack")

		assert.Equal(t, "performance", scenario["test_type"])
		assert.Equal(t, 10, scenario["concurrent_operations"])
		assert.Equal(t, 30, scenario["duration_minutes"])

		configs := scenario["configs"].([]config.Config)
		assert.Equal(t, 10, len(configs))

		workloads, ok := scenario["workloads"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 50, len(workloads))
	})
}

func TestScenarioGenerator_GenerateSecurityAuditScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates security audit scenario", func(t *testing.T) {
		scenario := gen.GenerateSecurityAuditScenario("openstack")

		assert.NotNil(t, scenario["config"])
		assert.NotNil(t, scenario["security_config"])
		assert.Equal(t, "security_audit", scenario["test_type"])

		vulnerabilities, ok := scenario["test_vulnerabilities"].([]string)
		require.True(t, ok)
		assert.Greater(t, len(vulnerabilities), 0)

		frameworks, ok := scenario["compliance_frameworks"].([]string)
		require.True(t, ok)
		assert.Contains(t, frameworks, "CIS")
		assert.Contains(t, frameworks, "PCI-DSS")
		assert.Contains(t, frameworks, "HIPAA")
	})
}

func TestScenarioGenerator_GenerateComplianceScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates compliance scenario", func(t *testing.T) {
		scenario := gen.GenerateComplianceScenario("openstack", "CIS")

		assert.NotNil(t, scenario["config"])
		assert.NotNil(t, scenario["security_config"])
		assert.Equal(t, "CIS", scenario["compliance_framework"])
		assert.Equal(t, "compliance", scenario["test_type"])
		assert.True(t, scenario["audit_logging_enabled"].(bool))
		assert.True(t, scenario["encryption_at_rest"].(bool))

		cfg := scenario["config"].(config.Config)
		assert.True(t, cfg.Security.K8sHardening)
		assert.True(t, cfg.Security.OSHardening)
		assert.True(t, cfg.OpenCenter.Cluster.Kubernetes.KubeletRotateServerCerts)
	})
}

func TestScenarioGenerator_GenerateIntegrationTestScenario(t *testing.T) {
	gen := NewScenarioGenerator(42)

	t.Run("generates integration test scenario", func(t *testing.T) {
		scenario := gen.GenerateIntegrationTestScenario("openstack")

		assert.NotNil(t, scenario["config"])
		assert.NotNil(t, scenario["gitops_config"])
		assert.NotNil(t, scenario["services"])
		assert.Equal(t, "integration", scenario["test_type"])

		services, ok := scenario["services"].([]map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(services))

		phases, ok := scenario["test_phases"].([]string)
		require.True(t, ok)
		assert.Contains(t, phases, "provision")
		assert.Contains(t, phases, "configure")
		assert.Contains(t, phases, "deploy")
		assert.Contains(t, phases, "validate")
	})
}

func TestScenarioGenerator_RealisticScenarios(t *testing.T) {
	t.Run("all scenarios produce valid configurations", func(t *testing.T) {
		gen := NewScenarioGenerator(42)

		scenarios := []struct {
			name     string
			generate func() map[string]interface{}
		}{
			{"production", func() map[string]interface{} { return gen.GenerateProductionScenario("openstack") }},
			{"development", func() map[string]interface{} { return gen.GenerateDevelopmentScenario("openstack") }},
			{"staging", func() map[string]interface{} { return gen.GenerateStagingScenario("openstack") }},
			{"multi-region", func() map[string]interface{} { return gen.GenerateMultiRegionScenario("aws") }},
			{"upgrade", func() map[string]interface{} { return gen.GenerateUpgradeScenario("openstack") }},
			{"migration", func() map[string]interface{} { return gen.GenerateMigrationScenario("openstack", "aws") }},
			{"dr", func() map[string]interface{} { return gen.GenerateDisasterRecoveryScenario("aws") }},
			{"edge-case", func() map[string]interface{} { return gen.GenerateEdgeCaseScenario("openstack") }},
			{"performance", func() map[string]interface{} { return gen.GeneratePerformanceTestScenario("openstack") }},
			{"security-audit", func() map[string]interface{} { return gen.GenerateSecurityAuditScenario("openstack") }},
			{"compliance", func() map[string]interface{} { return gen.GenerateComplianceScenario("openstack", "CIS") }},
			{"integration", func() map[string]interface{} { return gen.GenerateIntegrationTestScenario("openstack") }},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				result := scenario.generate()
				assert.NotNil(t, result, "Scenario should not be nil")
				assert.NotEmpty(t, result, "Scenario should not be empty")
			})
		}
	})
}

func BenchmarkScenarioGenerator(b *testing.B) {
	gen := NewScenarioGenerator(42)

	b.Run("GenerateProductionScenario", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateProductionScenario("openstack")
		}
	})

	b.Run("GenerateDevelopmentScenario", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateDevelopmentScenario("openstack")
		}
	})

	b.Run("GenerateMultiRegionScenario", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GenerateMultiRegionScenario("aws")
		}
	})

	b.Run("GeneratePerformanceTestScenario", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = gen.GeneratePerformanceTestScenario("openstack")
		}
	})
}
