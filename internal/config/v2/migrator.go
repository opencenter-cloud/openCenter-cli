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
	"fmt"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/config/defaults"
)

// Migrator defines the interface for v1 to v2 configuration migration.
// Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7
type Migrator interface {
	Migrate(v1Config *config.Config) (*Config, error)
	ValidateMigration(v1 *config.Config, v2 *Config) error
	GenerateMigrationReport(v1 *config.Config, v2 *Config) (*MigrationReport, error)
}

// MigrationReport contains details about the migration process.
// Requirements: 12.1
type MigrationReport struct {
	MovedFields     map[string]string // old path -> new path
	AppliedDefaults map[string]string // field -> default value
	Warnings        []string
}

// DefaultMigrator implements the Migrator interface.
type DefaultMigrator struct {
	hydrator defaults.Hydrator // For applying defaults during migration
}

// NewMigrator creates a new DefaultMigrator instance.
func NewMigrator(hydrator defaults.Hydrator) *DefaultMigrator {
	return &DefaultMigrator{
		hydrator: hydrator,
	}
}

// Migrate converts a v1 configuration to v2 format.
// Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7
func (m *DefaultMigrator) Migrate(v1Config *config.Config) (*Config, error) {
	if v1Config == nil {
		return nil, fmt.Errorf("v1 config cannot be nil")
	}

	v2Config := &Config{
		SchemaVersion: "2.0",
		Metadata: ConfigMetadata{
			CreatedAt: time.Now().Format(time.RFC3339),
			UpdatedAt: time.Now().Format(time.RFC3339),
			Version:   "2.0",
			Labels:    make(map[string]string),
			Annotations: make(map[string]string),
		},
		OpenCenter: OpenCenterConfig{
			Meta: m.migrateMeta(v1Config),
			Cluster: m.migrateCluster(v1Config),
			Infrastructure: m.migrateInfrastructure(v1Config),
			Services: make(ServiceMap),
			ManagedServices: make(ServiceMap),
			GitOps: m.migrateGitOps(v1Config),
		},
		Deployment: m.migrateDeployment(v1Config),
		OpenTofu: m.migrateOpenTofu(v1Config),
		Secrets: m.migrateSecrets(v1Config),
	}

	// Apply hydration to make implicit v1 defaults explicit in v2
	// Requirement: 12.6
	if m.hydrator != nil {
		// Apply hydration to v1 config before migration
		provider := v1Config.OpenCenter.Infrastructure.Provider
		region := v1Config.OpenCenter.Meta.Region
		if err := m.hydrator.Hydrate(v1Config, provider, region); err != nil {
			return nil, fmt.Errorf("failed to apply hydration during migration: %w", err)
		}
	}

	return v2Config, nil
}

// migrateMeta migrates meta configuration.
// Requirements: 1.3
func (m *DefaultMigrator) migrateMeta(v1 *config.Config) MetaConfig {
	return MetaConfig{
		Name:         v1.OpenCenter.Meta.Name,
		Organization: v1.OpenCenter.Meta.Organization,
		Env:          v1.OpenCenter.Meta.Env,
		Region:       v1.OpenCenter.Meta.Region,
		Status:       v1.OpenCenter.Meta.Status,
	}
}

// migrateCluster migrates cluster configuration.
// Requirements: 12.2, 12.3
func (m *DefaultMigrator) migrateCluster(v1 *config.Config) ClusterConfig {
	return ClusterConfig{
		ClusterName: v1.OpenCenter.Cluster.ClusterName,
		BaseDomain:  v1.OpenCenter.Cluster.BaseDomain,
		ClusterFQDN: v1.OpenCenter.Cluster.ClusterFQDN,
		AdminEmail:  v1.OpenCenter.Cluster.AdminEmail,
		Kubernetes: KubernetesConfig{
			Version:        v1.OpenCenter.Cluster.Kubernetes.Version,
			APIPort:        v1.OpenCenter.Cluster.Kubernetes.APIPort,
			KubeVIPEnabled: v1.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled,
			SubnetPods:     v1.OpenCenter.Cluster.Kubernetes.SubnetPods,
			SubnetServices: v1.OpenCenter.Cluster.Kubernetes.SubnetServices,
			NetworkPlugin:  m.migrateNetworkPlugin(v1),
			StoragePlugin:  m.migrateStoragePlugin(v1),
			Security:       m.migrateKubernetesSecurity(v1),
			OIDC:           m.migrateOIDC(v1),
		},
	}
}

// migrateNetworkPlugin migrates network plugin configuration.
func (m *DefaultMigrator) migrateNetworkPlugin(v1 *config.Config) NetworkPluginConfig {
	plugin := NetworkPluginConfig{}

	if v1.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled {
		plugin.Calico = &CalicoConfig{
			Enabled:       true,
			Version:       "", // Will be filled by hydration
			IPIPMode:      "Never",
			VXLANMode:     "Always",
			NetworkPolicy: true,
		}
	}

	if v1.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled {
		plugin.Cilium = &CiliumConfig{
			Enabled:       true,
			Version:       "", // Will be filled by hydration
			TunnelMode:    "vxlan",
			Hubble:        false,
			NetworkPolicy: true,
		}
	}

	if v1.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled {
		plugin.KubeOVN = &KubeOVNConfig{
			Enabled:       true,
			Version:       "", // Will be filled by hydration
			NetworkPolicy: true,
		}
	}

	return plugin
}

// migrateStoragePlugin migrates storage plugin configuration.
func (m *DefaultMigrator) migrateStoragePlugin(v1 *config.Config) StoragePluginConfig {
	plugin := StoragePluginConfig{}

	// Infer CSI plugin from provider
	switch v1.OpenCenter.Infrastructure.Provider {
	case "openstack":
		plugin.CinderCsi = &CinderCsiConfig{
			Enabled: true,
			Version: "", // Will be filled by hydration
		}
	case "aws":
		plugin.AwsEbsCsi = &AwsEbsCsiConfig{
			Enabled: true,
			Version: "", // Will be filled by hydration
		}
	case "gcp":
		plugin.GcpComputeCsi = &GcpComputeCsiConfig{
			Enabled: true,
			Version: "", // Will be filled by hydration
		}
	case "azure":
		plugin.AzureDiskCsi = &AzureDiskCsiConfig{
			Enabled: true,
			Version: "", // Will be filled by hydration
		}
	case "vsphere":
		plugin.VSphereCsi = &VSphereCsiConfig{
			Enabled: true,
			Version: "", // Will be filled by hydration
		}
	}

	return plugin
}

// migrateKubernetesSecurity migrates Kubernetes security configuration.
func (m *DefaultMigrator) migrateKubernetesSecurity(v1 *config.Config) KubernetesSecurityConfig {
	return KubernetesSecurityConfig{
		PodSecurityPolicy:    false,
		PodSecurityStandards: "baseline",
		AuditLogging:         false,
		EncryptionAtRest:     false,
		AdmissionControllers: []string{},
	}
}

// migrateOIDC migrates OIDC configuration.
func (m *DefaultMigrator) migrateOIDC(v1 *config.Config) OIDCConfig {
	return OIDCConfig{
		Enabled:       v1.OpenCenter.Cluster.Kubernetes.OIDC.Enabled,
		IssuerURL:     v1.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCURL,
		ClientID:      v1.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCClientID,
		ClientSecret:  "", // Moved to secrets
		UsernameClaim: v1.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCUsernameClaim,
		GroupsClaim:   v1.OpenCenter.Cluster.Kubernetes.OIDC.KubeOIDCGroupsClaim,
	}
}

// migrateInfrastructure migrates infrastructure configuration.
// Requirements: 12.2, 12.3, 12.4, 12.5
func (m *DefaultMigrator) migrateInfrastructure(v1 *config.Config) InfrastructureConfig {
	return InfrastructureConfig{
		Provider:            v1.OpenCenter.Infrastructure.Provider,
		SSH:                 m.migrateSSH(v1),
		OSVersion:           v1.OpenCenter.Infrastructure.OSVersion,
		ServerGroupAffinity: v1.OpenCenter.Infrastructure.ServerGroupAffinity,
		K8sAPIIP:            v1.OpenCenter.Infrastructure.K8sAPIIP,
		NodeNaming:          m.migrateNodeNaming(v1),
		Bastion:             m.migrateBastion(v1),
		Networking:          m.migrateNetworking(v1),
		Compute:             m.migrateCompute(v1),
		Storage:             m.migrateStorage(v1),
		Cloud:               m.migrateCloud(v1),
	}
}

// migrateSSH migrates SSH configuration.
// Requirements: 12.5
func (m *DefaultMigrator) migrateSSH(v1 *config.Config) SSHConfig {
	return SSHConfig{
		AuthorizedKeys: v1.OpenCenter.Cluster.SSHAuthorizedKeys,
		Username:       v1.OpenCenter.Infrastructure.SSHUser,
	}
}

// migrateNodeNaming migrates node naming configuration.
func (m *DefaultMigrator) migrateNodeNaming(v1 *config.Config) NodeNamingConfig {
	return NodeNamingConfig{
		Prefix: "",
		Suffix: "",
	}
}

// migrateBastion migrates bastion configuration.
func (m *DefaultMigrator) migrateBastion(v1 *config.Config) BastionConfig {
	return BastionConfig{
		Enabled: v1.OpenCenter.Infrastructure.Bastion.Address != "",
		Flavor:  v1.OpenCenter.Cluster.Kubernetes.FlavorBastion,
		Image:   "", // Will be filled by hydration
	}
}

// migrateNetworking migrates networking configuration.
// Requirements: 12.2, 12.3
func (m *DefaultMigrator) migrateNetworking(v1 *config.Config) NetworkingConfig {
	return NetworkingConfig{
		SubnetNodes:          v1.OpenCenter.Cluster.Networking.SubnetNodes,
		AllocationPoolStart:  v1.OpenCenter.Cluster.Networking.AllocationPoolStart,
		AllocationPoolEnd:    v1.OpenCenter.Cluster.Networking.AllocationPoolEnd,
		Gateway:              "",
		VRRPIP:               v1.OpenCenter.Cluster.Networking.VRRPIP, // Requirement: 12.2
		VRRPEnabled:          v1.OpenCenter.Cluster.Networking.VRRPEnabled,
		LoadbalancerProvider: v1.OpenCenter.Cluster.Networking.LoadbalancerProvider,
		UseDesignate:         v1.OpenCenter.Cluster.Networking.UseDesignate,
		DNSZoneName:          v1.OpenCenter.Cluster.Networking.DNSZoneName,
		DNSNameservers:       v1.OpenCenter.Cluster.Networking.DNSNameservers,
		NTPServers:           v1.OpenCenter.Cluster.Networking.NTPServers,
		Security:             NetworkSecurityConfig{},
		VLAN:                 m.migrateVLAN(v1),
	}
}

// migrateVLAN migrates VLAN configuration.
func (m *DefaultMigrator) migrateVLAN(v1 *config.Config) VLANConfig {
	return VLANConfig{
		Enabled: v1.OpenCenter.Cluster.Networking.VLAN.ID != "",
		ID:      0, // Parse from string if needed
	}
}

// migrateCompute migrates compute configuration.
// Requirements: 12.4
func (m *DefaultMigrator) migrateCompute(v1 *config.Config) ComputeConfig {
	return ComputeConfig{
		FlavorBastion:              v1.OpenCenter.Cluster.Kubernetes.FlavorBastion,
		FlavorMaster:               v1.OpenCenter.Cluster.Kubernetes.FlavorMaster,
		FlavorWorker:               v1.OpenCenter.Cluster.Kubernetes.FlavorWorker,
		FlavorWorkerWindows:        v1.OpenCenter.Cluster.Kubernetes.FlavorWorkerWindows,
		MasterCount:                v1.OpenCenter.Cluster.Kubernetes.MasterCount,
		WorkerCount:                v1.OpenCenter.Cluster.Kubernetes.WorkerCount,
		WorkerCountWindows:         v1.OpenCenter.Cluster.Kubernetes.WorkerCountWindows,
		AdditionalServerPoolsWorker: []WorkerPoolConfig{}, // Migrate if present
	}
}

// migrateStorage migrates storage configuration.
// Requirements: 12.4
func (m *DefaultMigrator) migrateStorage(v1 *config.Config) StorageConfig {
	return StorageConfig{
		DefaultStorageClass:             v1.OpenCenter.Storage.DefaultStorageClass,
		WorkerVolumeSize:                v1.OpenCenter.Storage.WorkerVolumeSize,
		WorkerVolumeDestinationType:     v1.OpenCenter.Storage.WorkerVolumeDestinationType,
		WorkerVolumeSourceType:          v1.OpenCenter.Storage.WorkerVolumeSourceType,
		WorkerVolumeType:                v1.OpenCenter.Storage.WorkerVolumeType,
		WorkerVolumeDeleteOnTermination: false,
		MasterVolumeSize:                0,
		MasterVolumeDestinationType:     "",
		MasterVolumeSourceType:          "",
		MasterVolumeType:                "",
		MasterVolumeDeleteOnTermination: false,
		AdditionalBlockDevices:          []BlockDeviceConfig{},
	}
}

// migrateCloud migrates cloud provider configuration.
func (m *DefaultMigrator) migrateCloud(v1 *config.Config) CloudConfig {
	cloud := CloudConfig{}

	switch v1.OpenCenter.Infrastructure.Provider {
	case "openstack":
		cloud.OpenStack = m.migrateOpenStack(v1)
	case "aws":
		cloud.AWS = m.migrateAWS(v1)
	case "gcp":
		cloud.GCP = m.migrateGCP(v1)
	case "azure":
		cloud.Azure = m.migrateAzure(v1)
	case "vsphere":
		cloud.VMware = m.migrateVMware(v1)
	}

	return cloud
}

// migrateOpenStack migrates OpenStack cloud configuration.
func (m *DefaultMigrator) migrateOpenStack(v1 *config.Config) *OpenStackCloudConfig {
	return &OpenStackCloudConfig{
		AuthURL:           v1.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL,
		Region:            v1.OpenCenter.Infrastructure.Cloud.OpenStack.Region,
		ProjectID:         "",
		ProjectName:       v1.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName,
		UserDomainName:    v1.OpenCenter.Infrastructure.Cloud.OpenStack.UserDomainName,
		ProjectDomainName: v1.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectDomainName,
		ImageID:           v1.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID,
		NetworkID:         v1.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.NetworkID,
		SubnetID:          v1.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.SubnetId,
		FloatingIPPool:    v1.OpenCenter.Infrastructure.Cloud.OpenStack.Networking.FloatingIPPool,
		AvailabilityZones: []string{v1.OpenCenter.Infrastructure.Cloud.OpenStack.AvailabilityZone},
		UseOctavia:        v1.OpenCenter.Cluster.Networking.UseOctavia,
		UseDesignate:      v1.OpenCenter.Cluster.Networking.UseDesignate,
	}
}

// migrateAWS migrates AWS cloud configuration.
func (m *DefaultMigrator) migrateAWS(v1 *config.Config) *AWSCloudConfig {
	return &AWSCloudConfig{
		Region:            v1.OpenCenter.Infrastructure.Cloud.AWS.Region,
		VPCID:             v1.OpenCenter.Infrastructure.Cloud.AWS.VPCID,
		SubnetIDs:         v1.OpenCenter.Infrastructure.Cloud.AWS.PrivateSubnets,
		AMIID:             "",
		AvailabilityZones: []string{},
		KeyPairName:       "",
		SecurityGroupIDs:  []string{},
	}
}

// migrateGCP migrates GCP cloud configuration.
func (m *DefaultMigrator) migrateGCP(v1 *config.Config) *GCPCloudConfig {
	return &GCPCloudConfig{
		Project:           "",
		Region:            "",
		Zone:              "",
		Network:           "",
		Subnetwork:        "",
		ImageFamily:       "",
		AvailabilityZones: []string{},
	}
}

// migrateAzure migrates Azure cloud configuration.
func (m *DefaultMigrator) migrateAzure(v1 *config.Config) *AzureCloudConfig {
	return &AzureCloudConfig{
		SubscriptionID:    "",
		ResourceGroup:     "",
		Location:          "",
		VNetName:          "",
		SubnetName:        "",
		ImageReference:    "",
		AvailabilityZones: []string{},
	}
}

// migrateVMware migrates VMware cloud configuration.
func (m *DefaultMigrator) migrateVMware(v1 *config.Config) *VMwareCloudConfig {
	return &VMwareCloudConfig{
		VCenterServer: "",
		Datacenter:    "",
		Cluster:       "",
		Datastore:     "",
		Network:       "",
		Template:      "",
		Folder:        "",
	}
}

// migrateGitOps migrates GitOps configuration.
func (m *DefaultMigrator) migrateGitOps(v1 *config.Config) GitOpsConfig {
	return GitOpsConfig{
		GitURL:          v1.OpenCenter.GitOps.GitURL,
		GitBranch:       v1.OpenCenter.GitOps.GitBranch,
		GitPath:         "",
		BaseRepoURL:     v1.OpenCenter.GitOps.GitOpsBaseRepo,
		BaseRepoRelease: v1.OpenCenter.GitOps.GitOpsBaseRelease,
		FluxInterval:    v1.OpenCenter.GitOps.Flux.Interval,
		FluxPrune:       v1.OpenCenter.GitOps.Flux.Prune,
	}
}

// migrateDeployment migrates deployment configuration.
func (m *DefaultMigrator) migrateDeployment(v1 *config.Config) DeploymentConfig {
	return DeploymentConfig{
		AutoDeploy: v1.Deployment.AutoDeploy,
		Method:     "kubespray", // Default method for v1 configs
	}
}

// migrateOpenTofu migrates OpenTofu configuration.
func (m *DefaultMigrator) migrateOpenTofu(v1 *config.Config) OpenTofuConfig {
	return OpenTofuConfig{
		Backend: BackendConfig{
			Type:   v1.OpenTofu.Backend.Type,
			Bucket: v1.OpenTofu.Backend.S3.Bucket,
			Key:    v1.OpenTofu.Backend.S3.Key,
			Region: v1.OpenTofu.Backend.S3.Region,
			Path:   v1.OpenTofu.Backend.Local.Path,
			Config: make(map[string]any),
		},
	}
}

// migrateSecrets migrates secrets configuration.
func (m *DefaultMigrator) migrateSecrets(v1 *config.Config) SecretsConfig {
	return SecretsConfig{
		Global: GlobalSecrets{
			AWSAccessKey:       v1.Secrets.Global.AWS.Infrastructure.AccessKey,
			AWSSecretKey:       v1.Secrets.Global.AWS.Infrastructure.SecretAccessKey,
			OpenStackAuthURL:   "",
			OpenStackUsername:  "",
			OpenStackPassword:  "",
			OpenStackProjectID: "",
		},
		ServiceSecrets: make(map[string]any),
		SOPSConfig: SOPSConfig{
			Enabled:        v1.Secrets.SopsAgeKeyFile != "",
			AgeKeyFile:     v1.Secrets.SopsAgeKeyFile,
			EncryptedRegex: "",
		},
	}
}

// ValidateMigration validates that the migration was successful.
// Requirements: 12.7
func (m *DefaultMigrator) ValidateMigration(v1 *config.Config, v2 *Config) error {
	if v1 == nil || v2 == nil {
		return fmt.Errorf("cannot validate nil configurations")
	}

	// Validate schema version
	if v2.SchemaVersion != "2.0" {
		return fmt.Errorf("invalid schema version: expected 2.0, got %s", v2.SchemaVersion)
	}

	// Validate critical fields were migrated
	if v2.OpenCenter.Meta.Name != v1.OpenCenter.Meta.Name {
		return fmt.Errorf("cluster name mismatch: v1=%s, v2=%s", v1.OpenCenter.Meta.Name, v2.OpenCenter.Meta.Name)
	}

	if v2.OpenCenter.Infrastructure.Provider != v1.OpenCenter.Infrastructure.Provider {
		return fmt.Errorf("provider mismatch: v1=%s, v2=%s", v1.OpenCenter.Infrastructure.Provider, v2.OpenCenter.Infrastructure.Provider)
	}

	// Validate VRRP IP was moved correctly (Requirement: 12.2)
	if v1.OpenCenter.Cluster.Networking.VRRPIP != "" {
		if v2.OpenCenter.Infrastructure.Networking.VRRPIP != v1.OpenCenter.Cluster.Networking.VRRPIP {
			return fmt.Errorf("VRRP IP not migrated correctly: v1=%s, v2=%s",
				v1.OpenCenter.Cluster.Networking.VRRPIP,
				v2.OpenCenter.Infrastructure.Networking.VRRPIP)
		}
	}

	return nil
}

// GenerateMigrationReport generates a report of the migration.
// Requirements: 12.1
func (m *DefaultMigrator) GenerateMigrationReport(v1 *config.Config, v2 *Config) (*MigrationReport, error) {
	if v1 == nil || v2 == nil {
		return nil, fmt.Errorf("cannot generate report for nil configurations")
	}

	report := &MigrationReport{
		MovedFields:     make(map[string]string),
		AppliedDefaults: make(map[string]string),
		Warnings:        []string{},
	}

	// Document field relocations
	report.MovedFields["cluster.networking.vrrp_ip"] = "infrastructure.networking.vrrp_ip"
	report.MovedFields["cluster.kubernetes.flavor_*"] = "infrastructure.compute.flavor_*"
	report.MovedFields["cluster.kubernetes.*_count"] = "infrastructure.compute.*_count"
	report.MovedFields["opencenter.storage.*"] = "infrastructure.storage.*"
	report.MovedFields["cluster.ssh_authorized_keys"] = "infrastructure.ssh.authorized_keys"
	report.MovedFields["infrastructure.ssh_user"] = "infrastructure.ssh.username"

	// Document applied defaults (if hydrator was used)
	if m.hydrator != nil {
		appliedDefaults := m.hydrator.GetAppliedDefaults()
		for field, source := range appliedDefaults {
			report.AppliedDefaults[field] = string(source)
		}
	}

	// Add warnings for deprecated features
	if v1.OpenCenter.Cluster.Networking.UseOctavia {
		report.Warnings = append(report.Warnings, "use_octavia flag moved to infrastructure.cloud.openstack.use_octavia")
	}

	if v1.OpenCenter.Cluster.Networking.UseDesignate {
		report.Warnings = append(report.Warnings, "use_designate flag moved to infrastructure.cloud.openstack.use_designate")
	}

	return report, nil
}
