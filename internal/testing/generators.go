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
	"fmt"
	"math/rand"
	"time"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// ConfigGenerator provides methods for generating realistic test configurations
type ConfigGenerator struct {
	rand *rand.Rand
}

// NewConfigGenerator creates a new configuration generator with a seeded random source
func NewConfigGenerator(seed int64) *ConfigGenerator {
	return &ConfigGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateConfig creates a realistic cluster configuration with random but valid values
func (g *ConfigGenerator) GenerateConfig(provider string) v2.Config {
	cfg := g.GenerateMinimalConfig(provider)
	cfg.OpenCenter.Meta.Organization = g.randomOrganization()
	cfg.OpenCenter.Meta.Env = g.randomEnvironment()
	cfg.OpenCenter.Meta.Region = g.randomRegionForProvider(provider)
	cfg.OpenCenter.Infrastructure.Cloud = g.generateCloudConfig(provider)
	cfg.OpenCenter.Infrastructure.Networking = g.generateNetworking(provider)
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Cluster.Kubernetes.Version = g.randomKubernetesVersion()
	cfg.OpenCenter.Cluster.Kubernetes.Security = g.generateSecurity()
	cfg.OpenCenter.Services = g.generateServices()
	cfg.OpenCenter.GitOps.Repository.URL = g.randomGitRepository()
	cfg.OpenCenter.GitOps.Repository.Branch = g.randomBranch()
	cfg.Deployment = g.generateDeployment()
	return cfg
}

// GenerateMinimalConfig creates a minimal but valid configuration
func (g *ConfigGenerator) GenerateMinimalConfig(provider string) v2.Config {
	clusterName := g.randomClusterName()
	cfgPtr, err := v2.NewV2Default(clusterName, provider)
	if err != nil {
		panic(fmt.Sprintf("generate minimal config: %v", err))
	}

	cfg := *cfgPtr
	cfg.OpenCenter.Meta.Organization = "test-org"
	cfg.OpenCenter.Meta.Env = "dev"
	cfg.OpenCenter.Meta.Region = g.randomRegionForProvider(provider)
	cfg.OpenCenter.Meta.Status = "pending"
	cfg.OpenCenter.Infrastructure.Provider = provider
	cfg.OpenCenter.Infrastructure.Cloud = g.generateCloudConfig(provider)
	cfg.OpenCenter.Infrastructure.Networking = g.generateNetworking(provider)
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 1
	cfg.OpenCenter.Cluster.ClusterName = clusterName
	cfg.OpenCenter.Cluster.BaseDomain = "example.com"
	cfg.OpenCenter.Cluster.ClusterFQDN = fmt.Sprintf("%s.example.com", clusterName)
	cfg.OpenCenter.Cluster.AdminEmail = "admin@example.com"
	cfg.OpenCenter.Cluster.Kubernetes.Version = "1.28.0"
	cfg.OpenCenter.Cluster.Kubernetes.SubnetPods = "10.42.0.0/16"
	cfg.OpenCenter.Cluster.Kubernetes.SubnetServices = "10.43.0.0/16"
	cfg.OpenCenter.Cluster.Kubernetes.Security = g.generateSecurity()
	cfg.OpenCenter.Services = make(v2.ServiceMap)
	cfg.OpenTofu.Enabled = true
	cfg.OpenTofu.Backend = v2.BackendConfig{
		Type:  "local",
		Local: &v2.LocalBackendConfig{Path: "terraform.tfstate"},
	}
	cfg.Secrets.SopsAgeKeyFile = "/path/to/age/key.txt"
	cfg.Secrets.SSHKey = v2.SSHKeyConfig{
		Private: "/path/to/ssh/private",
		Public:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC test@host",
		Cypher:  "rsa",
	}
	cfg.Deployment = g.generateDeployment()
	if cfg.Metadata.Labels == nil {
		cfg.Metadata.Labels = make(map[string]string)
	}

	return cfg
}

// GenerateComplexConfig creates a complex configuration with many services enabled
func (g *ConfigGenerator) GenerateComplexConfig(provider string) v2.Config {
	cfg := g.GenerateConfig(provider)

	// Enable all services
	cfg.OpenCenter.Services = v2.ServiceMap{
		"cert-manager": map[string]interface{}{
			"enabled": true,
			"version": "v1.12.0",
		},
		"prometheus-stack": map[string]interface{}{
			"enabled": true,
			"version": "v0.65.0",
		},
		"loki": map[string]interface{}{
			"enabled": true,
			"version": "v2.8.0",
		},
		"velero": map[string]interface{}{
			"enabled": true,
			"version": "v1.11.0",
		},
		"keycloak": map[string]interface{}{
			"enabled": true,
			"version": "v21.0.0",
		},
		"headlamp": map[string]interface{}{
			"enabled": true,
		},
		"weave-gitops": map[string]interface{}{
			"enabled": true,
		},
	}

	if cfg.Metadata.Labels == nil {
		cfg.Metadata.Labels = make(map[string]string)
	}
	cfg.Metadata.Labels["kubernetes.apiserver.extraArgs.audit-log-maxage"] = "30"
	cfg.Metadata.Labels["kubernetes.apiserver.extraArgs.audit-log-maxbackup"] = "10"
	cfg.Metadata.Labels["networking.cni"] = "calico"

	return cfg
}

// generateCloudConfig creates cloud-specific configuration
func (g *ConfigGenerator) generateCloudConfig(provider string) v2.CloudConfig {
	cloudConfig := v2.CloudConfig{}
	region := g.randomRegionForProvider(provider)

	switch provider {
	case "openstack":
		cloudConfig.OpenStack = &v2.OpenStackCloudConfig{
			AuthURL:     "https://identity.example.com/v3",
			Region:      region,
			ProjectID:   "project-id",
			ProjectName: "project-name",
			ImageID:     "image-id",
			NetworkID:   "network-id",
			SubnetID:    "subnet-id",
		}
	case "aws":
		cloudConfig.AWS = &v2.AWSCloudConfig{
			Region:    region,
			VPCID:     "vpc-12345",
			SubnetIDs: []string{"subnet-12345"},
			AMIID:     "ami-12345",
		}
	}

	return cloudConfig
}

// generateSecrets creates a realistic secrets configuration
func (g *ConfigGenerator) generateNetworking(provider string) v2.NetworkingConfig {
	return v2.NetworkingConfig{
		SubnetNodes:          g.randomCIDR("10.0.1.0/24"),
		AllocationPoolStart:  "10.0.1.10",
		AllocationPoolEnd:    "10.0.1.250",
		Gateway:              "10.0.1.1",
		DNSZoneName:          "cluster.example.com",
		DNSNameservers:       g.randomDNSServers(),
		NTPServers:           []string{"time.google.com"},
		LoadbalancerProvider: "ovn",
		VRRPEnabled:          provider != "kind",
		VRRPIP:               "10.0.1.5",
		Security: v2.NetworkSecurityConfig{
			AllowedCIDRs: []string{"0.0.0.0/0"},
		},
	}
}

// generateSecurity creates a realistic security configuration
func (g *ConfigGenerator) generateSecurity() v2.KubernetesSecurityConfig {
	return v2.KubernetesSecurityConfig{
		PodSecurityStandards: "baseline",
		AuditLogging:         true,
		EncryptionAtRest:     true,
	}
}

// generateDeployment creates a realistic deployment configuration
func (g *ConfigGenerator) generateDeployment() v2.DeploymentConfig {
	return v2.DeploymentConfig{
		AutoDeploy: g.randomBool(),
		Method:     "kubespray",
		Kubespray: &v2.KubesprayConfig{
			Version: "2.26.0",
		},
	}
}

// generateServices creates a realistic services configuration
func (g *ConfigGenerator) generateServices() v2.ServiceMap {
	services := make(v2.ServiceMap)

	// Randomly enable some services
	if g.randomBool() {
		services["cert-manager"] = map[string]interface{}{
			"enabled": true,
		}
	}

	if g.randomBool() {
		services["prometheus-stack"] = map[string]interface{}{
			"enabled": true,
		}
	}

	if g.randomBool() {
		services["loki"] = map[string]interface{}{
			"enabled": true,
		}
	}

	if g.randomBool() {
		services["velero"] = map[string]interface{}{
			"enabled": true,
		}
	}

	return services
}

func (g *ConfigGenerator) randomRegionForProvider(provider string) string {
	switch provider {
	case "aws":
		return g.randomAWSRegion()
	default:
		return g.randomRegion()
	}
}

// Random value generators

func (g *ConfigGenerator) randomOrganization() string {
	orgs := []string{"acme-corp", "example-inc", "test-org", "demo-company", "sample-enterprise"}
	return orgs[g.rand.Intn(len(orgs))]
}

func (g *ConfigGenerator) randomEnvironment() string {
	envs := []string{"dev", "staging", "production"}
	return envs[g.rand.Intn(len(envs))]
}

func (g *ConfigGenerator) randomClusterName() string {
	prefixes := []string{"prod", "staging", "dev", "test", "qa"}
	suffixes := []string{"cluster", "k8s", "kube", "platform"}
	return fmt.Sprintf("%s-%s-%d", prefixes[g.rand.Intn(len(prefixes))], suffixes[g.rand.Intn(len(suffixes))], g.rand.Intn(100))
}

func (g *ConfigGenerator) randomKubernetesVersion() string {
	versions := []string{"1.28.0", "1.29.0", "1.30.0", "1.31.0"}
	return versions[g.rand.Intn(len(versions))]
}


func (g *ConfigGenerator) randomRegion() string {
	regions := []string{"RegionOne", "RegionTwo", "us-east", "us-west", "eu-central"}
	return regions[g.rand.Intn(len(regions))]
}

func (g *ConfigGenerator) randomAWSRegion() string {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}
	return regions[g.rand.Intn(len(regions))]
}

func (g *ConfigGenerator) randomCIDR(defaultCIDR string) string {
	if g.randomBool() {
		return defaultCIDR
	}
	return fmt.Sprintf("10.%d.0.0/%d", g.rand.Intn(256), 12+g.rand.Intn(5)*4)
}

func (g *ConfigGenerator) randomDNSServers() []string {
	servers := [][]string{
		{"8.8.8.8", "8.8.4.4"},
		{"1.1.1.1", "1.0.0.1"},
		{"9.9.9.9", "149.112.112.112"},
	}
	return servers[g.rand.Intn(len(servers))]
}

func (g *ConfigGenerator) randomGitRepository() string {
	return fmt.Sprintf("https://github.com/%s/%s-gitops.git", g.randomOrganization(), g.randomClusterName())
}

func (g *ConfigGenerator) randomBranch() string {
	branches := []string{"main", "master", "develop", "staging"}
	return branches[g.rand.Intn(len(branches))]
}

func (g *ConfigGenerator) randomBool() bool {
	return g.rand.Intn(2) == 1
}

// TemplateDataGenerator provides methods for generating realistic template test data
type TemplateDataGenerator struct {
	rand *rand.Rand
}

// NewTemplateDataGenerator creates a new template data generator
func NewTemplateDataGenerator(seed int64) *TemplateDataGenerator {
	return &TemplateDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateTemplateData creates realistic data for template rendering tests
func (g *TemplateDataGenerator) GenerateTemplateData() map[string]interface{} {
	return map[string]interface{}{
		"ClusterName": g.randomClusterName(),
		"Namespace":   g.randomNamespace(),
		"Version":     g.randomVersion(),
		"Replicas":    g.randomReplicas(),
		"Image":       g.randomImage(),
		"Port":        g.randomPort(),
		"Environment": g.randomEnvironment(),
		"Labels":      g.randomLabels(),
		"Annotations": g.randomAnnotations(),
		"Resources":   g.randomResources(),
	}
}

func (g *TemplateDataGenerator) randomClusterName() string {
	return fmt.Sprintf("cluster-%d", g.rand.Intn(100))
}

func (g *TemplateDataGenerator) randomNamespace() string {
	namespaces := []string{"default", "kube-system", "monitoring", "logging", "ingress"}
	return namespaces[g.rand.Intn(len(namespaces))]
}

func (g *TemplateDataGenerator) randomVersion() string {
	return fmt.Sprintf("v%d.%d.%d", g.rand.Intn(3), g.rand.Intn(20), g.rand.Intn(10))
}

func (g *TemplateDataGenerator) randomReplicas() int {
	return 1 + g.rand.Intn(5)
}

func (g *TemplateDataGenerator) randomImage() string {
	images := []string{
		"nginx:latest",
		"redis:alpine",
		"postgres:14",
		"mysql:8.0",
		"mongodb:latest",
	}
	return images[g.rand.Intn(len(images))]
}

func (g *TemplateDataGenerator) randomPort() int {
	return 8000 + g.rand.Intn(1000)
}

func (g *TemplateDataGenerator) randomEnvironment() string {
	envs := []string{"development", "staging", "production", "test"}
	return envs[g.rand.Intn(len(envs))]
}

func (g *TemplateDataGenerator) randomLabels() map[string]string {
	return map[string]string{
		"app":         fmt.Sprintf("app-%d", g.rand.Intn(100)),
		"environment": g.randomEnvironment(),
		"version":     g.randomVersion(),
	}
}

func (g *TemplateDataGenerator) randomAnnotations() map[string]string {
	return map[string]string{
		"description": fmt.Sprintf("Generated test annotation %d", g.rand.Intn(1000)),
		"timestamp":   time.Now().Format(time.RFC3339),
	}
}

func (g *TemplateDataGenerator) randomResources() map[string]interface{} {
	return map[string]interface{}{
		"requests": map[string]string{
			"cpu":    fmt.Sprintf("%dm", 100+g.rand.Intn(900)),
			"memory": fmt.Sprintf("%dMi", 128+g.rand.Intn(896)),
		},
		"limits": map[string]string{
			"cpu":    fmt.Sprintf("%dm", 500+g.rand.Intn(1500)),
			"memory": fmt.Sprintf("%dMi", 256+g.rand.Intn(1792)),
		},
	}
}

// ServiceDataGenerator provides methods for generating realistic service test data
type ServiceDataGenerator struct {
	rand *rand.Rand
}

// NewServiceDataGenerator creates a new service data generator
func NewServiceDataGenerator(seed int64) *ServiceDataGenerator {
	return &ServiceDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateServiceDefinition creates a realistic service definition for testing
func (g *ServiceDataGenerator) GenerateServiceDefinition() map[string]interface{} {
	return map[string]interface{}{
		"name":         g.randomServiceName(),
		"type":         g.randomServiceType(),
		"enabled":      g.randomBool(),
		"version":      g.randomVersion(),
		"dependencies": g.randomDependencies(),
		"config":       g.randomServiceConfig(),
	}
}

func (g *ServiceDataGenerator) randomServiceName() string {
	services := []string{
		"cert-manager",
		"prometheus-stack",
		"loki",
		"velero",
		"keycloak",
		"headlamp",
		"weave-gitops",
	}
	return services[g.rand.Intn(len(services))]
}

func (g *ServiceDataGenerator) randomServiceType() string {
	types := []string{"monitoring", "security", "backup", "identity", "ui", "gitops"}
	return types[g.rand.Intn(len(types))]
}

func (g *ServiceDataGenerator) randomBool() bool {
	return g.rand.Intn(2) == 1
}

func (g *ServiceDataGenerator) randomVersion() string {
	return fmt.Sprintf("v%d.%d.%d", g.rand.Intn(3), g.rand.Intn(20), g.rand.Intn(10))
}

func (g *ServiceDataGenerator) randomDependencies() []string {
	allDeps := []string{"cert-manager", "prometheus-stack", "loki", "storage-class"}
	count := g.rand.Intn(3)
	deps := make([]string, 0, count)
	for i := 0; i < count; i++ {
		deps = append(deps, allDeps[g.rand.Intn(len(allDeps))])
	}
	return deps
}

func (g *ServiceDataGenerator) randomServiceConfig() map[string]interface{} {
	return map[string]interface{}{
		"replicas":  1 + g.rand.Intn(3),
		"namespace": fmt.Sprintf("service-%d", g.rand.Intn(100)),
		"resources": map[string]interface{}{
			"cpu":    fmt.Sprintf("%dm", 100+g.rand.Intn(400)),
			"memory": fmt.Sprintf("%dMi", 128+g.rand.Intn(384)),
		},
	}
}

// GitOpsDataGenerator provides methods for generating realistic GitOps test data
type GitOpsDataGenerator struct {
	rand *rand.Rand
}

// NewGitOpsDataGenerator creates a new GitOps data generator
func NewGitOpsDataGenerator(seed int64) *GitOpsDataGenerator {
	return &GitOpsDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateGitOpsConfig creates a realistic GitOps configuration for testing
func (g *GitOpsDataGenerator) GenerateGitOpsConfig() map[string]interface{} {
	return map[string]interface{}{
		"enabled":    true,
		"repository": g.randomRepository(),
		"branch":     g.randomBranch(),
		"path":       g.randomPath(),
		"sync": map[string]interface{}{
			"interval": g.randomInterval(),
			"timeout":  g.randomTimeout(),
		},
	}
}

func (g *GitOpsDataGenerator) randomRepository() string {
	orgs := []string{"acme", "example", "test", "demo"}
	repos := []string{"gitops", "k8s-config", "cluster-config", "infrastructure"}
	return fmt.Sprintf("https://github.com/%s/%s.git",
		orgs[g.rand.Intn(len(orgs))],
		repos[g.rand.Intn(len(repos))])
}

func (g *GitOpsDataGenerator) randomBranch() string {
	branches := []string{"main", "master", "develop", "staging", "production"}
	return branches[g.rand.Intn(len(branches))]
}

func (g *GitOpsDataGenerator) randomPath() string {
	paths := []string{"clusters", "infrastructure", "applications", "services"}
	return fmt.Sprintf("%s/cluster-%d", paths[g.rand.Intn(len(paths))], g.rand.Intn(100))
}

func (g *GitOpsDataGenerator) randomInterval() string {
	intervals := []string{"1m", "5m", "10m", "30m", "1h"}
	return intervals[g.rand.Intn(len(intervals))]
}

func (g *GitOpsDataGenerator) randomTimeout() string {
	timeouts := []string{"30s", "1m", "2m", "5m"}
	return timeouts[g.rand.Intn(len(timeouts))]
}

// NetworkingDataGenerator provides methods for generating realistic networking test data
type NetworkingDataGenerator struct {
	rand *rand.Rand
}

// NewNetworkingDataGenerator creates a new networking data generator
func NewNetworkingDataGenerator(seed int64) *NetworkingDataGenerator {
	return &NetworkingDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateNetworkingConfig creates a realistic networking configuration for testing
func (g *NetworkingDataGenerator) GenerateNetworkingConfig() map[string]interface{} {
	return map[string]interface{}{
		"subnet_nodes":    g.randomSubnet("10.0"),
		"subnet_pods":     g.randomSubnet("10.42"),
		"subnet_services": g.randomSubnet("10.43"),
		"dns_servers":     g.randomDNSServers(),
		"mtu":             g.randomMTU(),
		"vrrp_enabled":    g.randomBool(),
		"load_balancer":   g.randomLoadBalancer(),
	}
}

func (g *NetworkingDataGenerator) randomSubnet(prefix string) string {
	return fmt.Sprintf("%s.%d.0/%d", prefix, g.rand.Intn(256), 16+g.rand.Intn(9))
}

func (g *NetworkingDataGenerator) randomDNSServers() []string {
	servers := [][]string{
		{"8.8.8.8", "8.8.4.4"},
		{"1.1.1.1", "1.0.0.1"},
		{"9.9.9.9", "149.112.112.112"},
		{"208.67.222.222", "208.67.220.220"},
	}
	return servers[g.rand.Intn(len(servers))]
}

func (g *NetworkingDataGenerator) randomMTU() int {
	mtus := []int{1500, 9000, 9100}
	return mtus[g.rand.Intn(len(mtus))]
}

func (g *NetworkingDataGenerator) randomBool() bool {
	return g.rand.Intn(2) == 1
}

func (g *NetworkingDataGenerator) randomLoadBalancer() string {
	lbs := []string{"ovn", "metallb", "haproxy", "nginx"}
	return lbs[g.rand.Intn(len(lbs))]
}

// InfrastructureDataGenerator provides methods for generating realistic infrastructure test data
type InfrastructureDataGenerator struct {
	rand *rand.Rand
}

// NewInfrastructureDataGenerator creates a new infrastructure data generator
func NewInfrastructureDataGenerator(seed int64) *InfrastructureDataGenerator {
	return &InfrastructureDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateNodePool creates a realistic node pool configuration for testing
func (g *InfrastructureDataGenerator) GenerateNodePool() map[string]interface{} {
	pool := map[string]interface{}{
		"name":      g.randomNodePoolName(),
		"count":     g.randomNodeCount(),
		"flavor":    g.randomFlavor(),
		"disk_size": g.randomDiskSize(),
		"labels":    g.randomNodeLabels(),
		"taints":    g.randomTaints(),
	}

	// Only add auto_scaling if it's not nil
	if autoScaling := g.randomAutoScaling(); autoScaling != nil {
		pool["auto_scaling"] = autoScaling
	}

	return pool
}

func (g *InfrastructureDataGenerator) randomNodePoolName() string {
	types := []string{"master", "worker", "infra", "compute", "storage"}
	return fmt.Sprintf("%s-pool-%d", types[g.rand.Intn(len(types))], g.rand.Intn(10))
}

func (g *InfrastructureDataGenerator) randomNodeCount() int {
	return 1 + g.rand.Intn(10)
}

func (g *InfrastructureDataGenerator) randomFlavor() string {
	flavors := []string{"m1.small", "m1.medium", "m1.large", "m1.xlarge", "c1.large", "r1.large"}
	return flavors[g.rand.Intn(len(flavors))]
}

func (g *InfrastructureDataGenerator) randomDiskSize() int {
	sizes := []int{20, 40, 80, 100, 200, 500}
	return sizes[g.rand.Intn(len(sizes))]
}

func (g *InfrastructureDataGenerator) randomNodeLabels() map[string]string {
	labels := map[string]string{
		"node-role": []string{"master", "worker", "infra"}[g.rand.Intn(3)],
		"zone":      fmt.Sprintf("zone-%d", g.rand.Intn(3)),
	}
	if g.rand.Intn(2) == 1 {
		labels["workload"] = []string{"general", "compute", "storage"}[g.rand.Intn(3)]
	}
	return labels
}

func (g *InfrastructureDataGenerator) randomTaints() []map[string]string {
	if g.rand.Intn(3) == 0 {
		return nil // No taints
	}
	return []map[string]string{
		{
			"key":    "dedicated",
			"value":  []string{"master", "infra", "storage"}[g.rand.Intn(3)],
			"effect": "NoSchedule",
		},
	}
}

func (g *InfrastructureDataGenerator) randomAutoScaling() map[string]interface{} {
	if g.rand.Intn(2) == 0 {
		return nil // No auto-scaling
	}
	minNodes := 1 + g.rand.Intn(3)
	maxNodes := minNodes + 1 + g.rand.Intn(10)
	return map[string]interface{}{
		"enabled":   true,
		"min_nodes": minNodes,
		"max_nodes": maxNodes,
	}
}

// SecurityDataGenerator provides methods for generating realistic security test data
type SecurityDataGenerator struct {
	rand *rand.Rand
}

// NewSecurityDataGenerator creates a new security data generator
func NewSecurityDataGenerator(seed int64) *SecurityDataGenerator {
	return &SecurityDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateSecurityConfig creates a realistic security configuration for testing
func (g *SecurityDataGenerator) GenerateSecurityConfig() map[string]interface{} {
	return map[string]interface{}{
		"k8s_hardening":         g.randomBool(),
		"os_hardening":          g.randomBool(),
		"pod_security":          g.randomPodSecurity(),
		"network_policies":      g.randomBool(),
		"encryption_at_rest":    g.randomBool(),
		"audit_logging":         g.randomBool(),
		"rbac_enabled":          true, // Always enabled in modern clusters
		"admission_controllers": g.randomAdmissionControllers(),
	}
}

func (g *SecurityDataGenerator) randomBool() bool {
	return g.rand.Intn(2) == 1
}

func (g *SecurityDataGenerator) randomPodSecurity() string {
	levels := []string{"privileged", "baseline", "restricted"}
	return levels[g.rand.Intn(len(levels))]
}

func (g *SecurityDataGenerator) randomAdmissionControllers() []string {
	all := []string{
		"PodSecurityPolicy",
		"LimitRanger",
		"ResourceQuota",
		"MutatingAdmissionWebhook",
		"ValidatingAdmissionWebhook",
	}
	count := 2 + g.rand.Intn(4)
	selected := make([]string, 0, count)
	for i := 0; i < count && i < len(all); i++ {
		selected = append(selected, all[i])
	}
	return selected
}

// WorkloadDataGenerator provides methods for generating realistic workload test data
type WorkloadDataGenerator struct {
	rand *rand.Rand
}

// NewWorkloadDataGenerator creates a new workload data generator
func NewWorkloadDataGenerator(seed int64) *WorkloadDataGenerator {
	return &WorkloadDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateDeployment creates a realistic Kubernetes deployment for testing
func (g *WorkloadDataGenerator) GenerateDeployment() map[string]interface{} {
	return map[string]interface{}{
		"name":      g.randomWorkloadName(),
		"namespace": g.randomNamespace(),
		"replicas":  g.randomReplicas(),
		"image":     g.randomImage(),
		"ports":     g.randomPorts(),
		"env":       g.randomEnvVars(),
		"volumes":   g.randomVolumes(),
		"resources": g.randomResources(),
		"probes":    g.randomProbes(),
	}
}

func (g *WorkloadDataGenerator) randomWorkloadName() string {
	prefixes := []string{"web", "api", "worker", "cache", "db"}
	return fmt.Sprintf("%s-app-%d", prefixes[g.rand.Intn(len(prefixes))], g.rand.Intn(100))
}

func (g *WorkloadDataGenerator) randomNamespace() string {
	namespaces := []string{"default", "production", "staging", "development", "monitoring", "logging"}
	return namespaces[g.rand.Intn(len(namespaces))]
}

func (g *WorkloadDataGenerator) randomReplicas() int {
	return 1 + g.rand.Intn(5)
}

func (g *WorkloadDataGenerator) randomImage() string {
	images := []string{
		"nginx:1.21",
		"redis:7-alpine",
		"postgres:14",
		"mysql:8.0",
		"mongodb:6.0",
		"node:18-alpine",
		"python:3.11-slim",
	}
	return images[g.rand.Intn(len(images))]
}

func (g *WorkloadDataGenerator) randomPorts() []map[string]interface{} {
	ports := []map[string]interface{}{
		{
			"name":          "http",
			"containerPort": 8080,
			"protocol":      "TCP",
		},
	}
	if g.rand.Intn(2) == 1 {
		ports = append(ports, map[string]interface{}{
			"name":          "metrics",
			"containerPort": 9090,
			"protocol":      "TCP",
		})
	}
	return ports
}

func (g *WorkloadDataGenerator) randomEnvVars() []map[string]string {
	envs := []map[string]string{
		{"name": "LOG_LEVEL", "value": []string{"info", "debug", "warn"}[g.rand.Intn(3)]},
		{"name": "ENVIRONMENT", "value": []string{"production", "staging", "development"}[g.rand.Intn(3)]},
	}
	if g.rand.Intn(2) == 1 {
		envs = append(envs, map[string]string{
			"name":  "DATABASE_URL",
			"value": "postgresql://localhost:5432/mydb",
		})
	}
	return envs
}

func (g *WorkloadDataGenerator) randomVolumes() []map[string]interface{} {
	if g.rand.Intn(3) == 0 {
		return nil // No volumes
	}
	return []map[string]interface{}{
		{
			"name":      "data",
			"mountPath": "/data",
			"size":      fmt.Sprintf("%dGi", 1+g.rand.Intn(100)),
		},
	}
}

func (g *WorkloadDataGenerator) randomResources() map[string]interface{} {
	cpuRequest := 100 + g.rand.Intn(900)
	memRequest := 128 + g.rand.Intn(896)
	return map[string]interface{}{
		"requests": map[string]string{
			"cpu":    fmt.Sprintf("%dm", cpuRequest),
			"memory": fmt.Sprintf("%dMi", memRequest),
		},
		"limits": map[string]string{
			"cpu":    fmt.Sprintf("%dm", cpuRequest*2),
			"memory": fmt.Sprintf("%dMi", memRequest*2),
		},
	}
}

func (g *WorkloadDataGenerator) randomProbes() map[string]interface{} {
	return map[string]interface{}{
		"liveness": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": 8080,
			},
			"initialDelaySeconds": 10 + g.rand.Intn(20),
			"periodSeconds":       5 + g.rand.Intn(10),
		},
		"readiness": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/ready",
				"port": 8080,
			},
			"initialDelaySeconds": 5 + g.rand.Intn(10),
			"periodSeconds":       3 + g.rand.Intn(7),
		},
	}
}

// ScenarioGenerator provides methods for generating complete realistic test scenarios
type ScenarioGenerator struct {
	rand *rand.Rand
}

// NewScenarioGenerator creates a new scenario generator
func NewScenarioGenerator(seed int64) *ScenarioGenerator {
	return &ScenarioGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateProductionScenario creates a realistic production cluster scenario
func (g *ScenarioGenerator) GenerateProductionScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	infraGen := &InfrastructureDataGenerator{rand: g.rand}
	securityGen := &SecurityDataGenerator{rand: g.rand}
	networkGen := &NetworkingDataGenerator{rand: g.rand}

	return map[string]interface{}{
		"config": configGen.GenerateComplexConfig(provider),
		"node_pools": []map[string]interface{}{
			infraGen.GenerateNodePool(), // Master nodes
			infraGen.GenerateNodePool(), // Worker nodes
			infraGen.GenerateNodePool(), // Infra nodes
		},
		"security":           securityGen.GenerateSecurityConfig(),
		"networking":         networkGen.GenerateNetworkingConfig(),
		"environment":        "production",
		"high_availability":  true,
		"backup_enabled":     true,
		"monitoring_enabled": true,
	}
}

// GenerateDevelopmentScenario creates a realistic development cluster scenario
func (g *ScenarioGenerator) GenerateDevelopmentScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	infraGen := &InfrastructureDataGenerator{rand: g.rand}

	return map[string]interface{}{
		"config": configGen.GenerateMinimalConfig(provider),
		"node_pools": []map[string]interface{}{
			infraGen.GenerateNodePool(), // Single node pool
		},
		"environment":        "development",
		"high_availability":  false,
		"backup_enabled":     false,
		"monitoring_enabled": false,
	}
}

// GenerateStagingScenario creates a realistic staging cluster scenario
func (g *ScenarioGenerator) GenerateStagingScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	infraGen := &InfrastructureDataGenerator{rand: g.rand}
	securityGen := &SecurityDataGenerator{rand: g.rand}

	return map[string]interface{}{
		"config": configGen.GenerateConfig(provider),
		"node_pools": []map[string]interface{}{
			infraGen.GenerateNodePool(), // Master nodes
			infraGen.GenerateNodePool(), // Worker nodes
		},
		"security":           securityGen.GenerateSecurityConfig(),
		"environment":        "staging",
		"high_availability":  true,
		"backup_enabled":     true,
		"monitoring_enabled": true,
	}
}

// GenerateMultiRegionScenario creates a realistic multi-region deployment scenario
func (g *ScenarioGenerator) GenerateMultiRegionScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}

	regions := []string{"us-east-1", "us-west-2", "eu-central-1"}
	clusters := make([]map[string]interface{}, 0, len(regions))

	for _, region := range regions {
		cfg := configGen.GenerateConfig(provider)
		cfg.OpenCenter.Meta.Region = region
		clusters = append(clusters, map[string]interface{}{
			"region": region,
			"config": cfg,
		})
	}

	return map[string]interface{}{
		"clusters":            clusters,
		"multi_region":        true,
		"replication_enabled": true,
	}
}

// GenerateUpgradeScenario creates a realistic cluster upgrade scenario
func (g *ScenarioGenerator) GenerateUpgradeScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}

	oldConfig := configGen.GenerateConfig(provider)
	oldConfig.OpenCenter.Cluster.Kubernetes.Version = "1.28.0"

	newConfig := configGen.GenerateConfig(provider)
	newConfig.OpenCenter.Cluster.Kubernetes.Version = "1.29.0"
	newConfig.OpenCenter.Meta = oldConfig.OpenCenter.Meta // Keep same cluster identity

	return map[string]interface{}{
		"old_config":       oldConfig,
		"new_config":       newConfig,
		"upgrade_type":     "minor",
		"rollback_enabled": true,
	}
}

// GenerateMigrationScenario creates a realistic provider migration scenario
func (g *ScenarioGenerator) GenerateMigrationScenario(fromProvider, toProvider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}

	sourceConfig := configGen.GenerateConfig(fromProvider)
	targetConfig := configGen.GenerateConfig(toProvider)
	targetConfig.OpenCenter.Meta = sourceConfig.OpenCenter.Meta // Keep same cluster identity

	return map[string]interface{}{
		"source_config":           sourceConfig,
		"target_config":           targetConfig,
		"migration_type":          "provider",
		"data_migration_required": true,
	}
}

// GenerateDisasterRecoveryScenario creates a realistic DR scenario
func (g *ScenarioGenerator) GenerateDisasterRecoveryScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}

	primaryConfig := configGen.GenerateComplexConfig(provider)
	primaryConfig.OpenCenter.Meta.Region = "us-east-1"

	drConfig := configGen.GenerateComplexConfig(provider)
	drConfig.OpenCenter.Meta.Region = "us-west-2"
	drConfig.OpenCenter.Meta.Name = primaryConfig.OpenCenter.Meta.Name + "-dr"

	return map[string]interface{}{
		"primary_config":      primaryConfig,
		"dr_config":           drConfig,
		"replication_enabled": true,
		"failover_enabled":    true,
		"rpo_minutes":         15,
		"rto_minutes":         30,
	}
}

// GenerateEdgeCaseScenario creates scenarios with edge cases for testing
func (g *ScenarioGenerator) GenerateEdgeCaseScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	cfg := configGen.GenerateConfig(provider)

	// Add edge case configurations
	edgeCases := []string{
		"empty_services",
		"max_node_count",
		"min_resources",
		"special_characters_in_names",
		"long_cluster_name",
	}

	selectedCase := edgeCases[g.rand.Intn(len(edgeCases))]

	switch selectedCase {
	case "empty_services":
		cfg.OpenCenter.Services = make(v2.ServiceMap)
	case "max_node_count":
		// Simulate maximum node count scenario
		cfg.OpenCenter.Infrastructure.Compute.MasterCount = 50
		cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 50
	case "min_resources":
		// Simulate minimal resource allocation
		cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize = 20
	case "special_characters_in_names":
		cfg.OpenCenter.Meta.Name = "test-cluster_v2.0-final"
	case "long_cluster_name":
		cfg.OpenCenter.Meta.Name = "very-long-cluster-name-that-tests-length-limits-and-validation"
	}

	return map[string]interface{}{
		"config":    cfg,
		"edge_case": selectedCase,
		"test_type": "boundary",
	}
}

// GeneratePerformanceTestScenario creates scenarios for performance testing
func (g *ScenarioGenerator) GeneratePerformanceTestScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	workloadGen := &WorkloadDataGenerator{rand: g.rand}

	// Generate multiple configurations
	configs := make([]v2.Config, 0, 10)
	for i := 0; i < 10; i++ {
		configs = append(configs, configGen.GenerateConfig(provider))
	}

	// Generate multiple workloads
	workloads := make([]map[string]interface{}, 0, 50)
	for i := 0; i < 50; i++ {
		workloads = append(workloads, workloadGen.GenerateDeployment())
	}

	return map[string]interface{}{
		"configs":               configs,
		"workloads":             workloads,
		"test_type":             "performance",
		"concurrent_operations": 10,
		"duration_minutes":      30,
	}
}

// GenerateSecurityAuditScenario creates scenarios for security testing
func (g *ScenarioGenerator) GenerateSecurityAuditScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	securityGen := &SecurityDataGenerator{rand: g.rand}

	cfg := configGen.GenerateConfig(provider)
	securityConfig := securityGen.GenerateSecurityConfig()

	// Add security-specific test data
	vulnerabilities := []string{
		"exposed_api_server",
		"weak_rbac_policies",
		"unencrypted_secrets",
		"missing_network_policies",
		"outdated_kubernetes_version",
	}

	return map[string]interface{}{
		"config":                cfg,
		"security_config":       securityConfig,
		"test_vulnerabilities":  vulnerabilities,
		"compliance_frameworks": []string{"CIS", "PCI-DSS", "HIPAA"},
		"test_type":             "security_audit",
	}
}

// GenerateComplianceScenario creates scenarios for compliance testing
func (g *ScenarioGenerator) GenerateComplianceScenario(provider string, framework string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	securityGen := &SecurityDataGenerator{rand: g.rand}

	cfg := configGen.GenerateComplexConfig(provider)
	securityConfig := securityGen.GenerateSecurityConfig()

	// Ensure compliance-required settings
	cfg.OpenCenter.Cluster.Kubernetes.Security.AuditLogging = true
	cfg.OpenCenter.Cluster.Kubernetes.Security.EncryptionAtRest = true
	cfg.OpenCenter.Infrastructure.Networking.Security.AllowedCIDRs = []string{"0.0.0.0/0"}

	return map[string]interface{}{
		"config":                cfg,
		"security_config":       securityConfig,
		"compliance_framework":  framework,
		"audit_logging_enabled": true,
		"encryption_at_rest":    true,
		"test_type":             "compliance",
	}
}

// GenerateIntegrationTestScenario creates scenarios for integration testing
func (g *ScenarioGenerator) GenerateIntegrationTestScenario(provider string) map[string]interface{} {
	configGen := &ConfigGenerator{rand: g.rand}
	gitopsGen := &GitOpsDataGenerator{rand: g.rand}
	serviceGen := &ServiceDataGenerator{rand: g.rand}

	cfg := configGen.GenerateConfig(provider)
	gitopsConfig := gitopsGen.GenerateGitOpsConfig()

	// Generate multiple services
	services := make([]map[string]interface{}, 0, 5)
	for i := 0; i < 5; i++ {
		services = append(services, serviceGen.GenerateServiceDefinition())
	}

	return map[string]interface{}{
		"config":        cfg,
		"gitops_config": gitopsConfig,
		"services":      services,
		"test_type":     "integration",
		"test_phases":   []string{"provision", "configure", "deploy", "validate"},
	}
}
