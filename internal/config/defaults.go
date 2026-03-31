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
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

const (
	// DefaultSSHAuthorizedKeyPlaceholder keeps freshly initialized configs valid
	// until a user-provided key or generated key replaces it.
	DefaultSSHAuthorizedKeyPlaceholder = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExamplePublicKeyDataHere user@example.com"
	defaultVRRPIP                      = "10.2.128.5"
)

// getDefaultSSHKeys returns SSH keys from CLI defaults or an empty string array as fallback.
func getDefaultSSHKeys(cliDefaults *DefaultsConfig) []string {
	if cliDefaults != nil && len(cliDefaults.SSHAuthorizedKeys) > 0 {
		return cliDefaults.SSHAuthorizedKeys
	}
	return []string{DefaultSSHAuthorizedKeyPlaceholder}
}

// getDefaultProvider returns provider from CLI defaults or "openstack" as fallback.
func getDefaultProvider(cliDefaults *DefaultsConfig) string {
	if cliDefaults != nil && cliDefaults.Provider != "" {
		return cliDefaults.Provider
	}
	return "openstack"
}

// getDefaultEnvironment returns environment from CLI defaults or empty string as fallback.
func getDefaultEnvironment(cliDefaults *DefaultsConfig) string {
	if cliDefaults != nil && cliDefaults.Environment != "" {
		return cliDefaults.Environment
	}
	return ""
}

// defaultConfig returns a Config pre-populated with the default
// values based on the simplified schema. This function can be used to
// initialise new cluster configurations.
func defaultConfig(name string) Config {
	authURL := ""
	region := "sjc3" // Default region
	tenantName := ""
	barbicanAuthURL := ""

	// Load CLI defaults if available
	var cliDefaults *DefaultsConfig
	if cm, err := NewConfigManager(""); err == nil {
		if cliConfig := cm.GetConfig(); cliConfig != nil {
			cliDefaults = &cliConfig.Defaults
		}
	}

	// Apply CLI defaults for region if available (skip template strings)
	if cliDefaults != nil && cliDefaults.Region != "" && !strings.Contains(cliDefaults.Region, "{{") {
		region = cliDefaults.Region
	}

	cfg := Config{
		SchemaVersion: SchemaVersion,
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         name,
				Env:          getDefaultEnvironment(cliDefaults),
				Region:       region,
				Status:       "",
				Organization: "opencenter",
			},
			Secrets: OpenCenterSecrets{
				Backend: "sops",
				Barbican: BarbicanConfig{
					AuthURL:           barbicanAuthURL,
					ProjectID:         "",
					Region:            "",
					UserDomainName:    "",
					ProjectDomainName: "",
					CACert:            "",
				},
			},
			Infrastructure: Infrastructure{
				Provider:            getDefaultProvider(cliDefaults),
				SSHUser:             "ubuntu",
				OSVersion:           "24",
				ServerGroupAffinity: []string{"anti-affinity"},
				NodeNaming: NodeNaming{
					Worker:        "wn",
					Master:        "cp",
					WorkerWindows: "win",
				},
				Bastion: BastionConfig{
					Address: "localhost",
				},
				K8sAPIIP: "",
				Cloud: CloudConfig{
					AWS: SimplifiedAWSCloud{
						Profile:        "",
						Region:         "",
						VPCID:          "",
						PrivateSubnets: []string{},
						PublicSubnets:  []string{},
					},
					OpenStack: SimplifiedOpenStackCloud{
						AuthURL:                     authURL,
						Insecure:                    false,
						Region:                      region,
						ApplicationCredentialID:     "",
						ApplicationCredentialSecret: "",
						Domain:                      "",
						TenantName:                  tenantName,
						AvailabilityZone:            "az1",
						ProjectDomainName:           "rackspace_cloud_domain",
						UserDomainName:              "rackspace_cloud_domain",
						CA:                          "",
						ImageID:                     "799dcf97-3656-4361-8187-13ab1b295e33",
						ImageIDWindows:              "a2083759-f341-445b-b717-dafb5e31fa6b",
						Networking: OpenStackNetworkingConfig{
							FloatingIPPool:          "PUBLICNET",
							FloatingNetworkId:       "",
							NetworkID:               "",
							RouterExternalNetworkID: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79",
							SubnetId:                "",
							K8sAPIPortACL:           []string{"0.0.0.0/0"},
							Designate: DesignateConfig{
								DNSZoneName: "",
							},
							VLAN: VLAN{
								ID:       "",
								MTU:      0,
								Provider: "physnet1",
							},
						},
						Modules: OpenStackModulesConfig{
							OpenstackNova: OpenstackNovaModuleConfig{
								Source: "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cloud/openstack/openstack-nova?ref=main",
							},
						},
					},
				},
			},
			Cluster: ClusterConfig{
				ClusterName:        name,
				AWSAccessKey:       "",
				AWSSecretAccessKey: "",
				SSHAuthorizedKeys:  getDefaultSSHKeys(cliDefaults),
				BaseDomain:         "k8s.opencenter.cloud",
				ClusterFQDN:        fmt.Sprintf("%s.%s.k8s.opencenter.cloud", name, region),
				AdminEmail:         "admin@example.com",
				Networking: ClusterNetworkingConfig{
					NTPServers:     []string{fmt.Sprintf("time.%s.rackspace.com", strings.ToLower(region)), fmt.Sprintf("time2.%s.rackspace.com", strings.ToLower(region))},
					DNSNameservers: []string{"8.8.8.8", "8.8.4.4"},
					Security: ClusterSecurityConfig{
						CACertificates: "",
						OSHardening:    true,
					},
					// Network topology defaults
					SubnetNodes:         "10.2.128.0/22",
					AllocationPoolStart: "",
					AllocationPoolEnd:   "",
					// VRRP defaults
					VRRPIP:      defaultVRRPIP,
					VRRPEnabled: true,
					// Load balancer defaults
					UseOctavia:           false,
					LoadbalancerProvider: "ovn",
					// DNS defaults
					UseDesignate: false,
					DNSZoneName:  "",
					// VLAN defaults
					VLAN: VLAN{
						ID:       "",
						MTU:      0,
						Provider: "physnet1",
					},
				},
				Kubernetes: KubernetesConfig{
					Version:                  "1.33.5",
					KubesprayVersion:         "v2.29.1",
					APIPort:                  443,
					KubeVIPEnabled:           true,
					KubeletRotateServerCerts: false,
					FlavorBastion:            "gp.0.2.2",
					FlavorMaster:             "gp.0.4.8",
					FlavorWorker:             "gp.0.4.16",
					FlavorWorkerWindows:      "gp.5.4.16",
					SubnetPods:               "10.42.0.0/16",
					SubnetServices:           "10.43.0.0/16",
					LoadbalancerProvider:     "ovn",
					MasterCount:              3,
					WorkerCount:              2,
					WorkerCountWindows:       0,
					Security: KubernetesSecurityConfig{
						K8sHardening:          true,
						PodSecurityExemptions: []string{"trivy-temp", "tigera-operator", "kube-system"},
					},
					NetworkPlugin: NetworkPlugin{
						Calico: CalicoConfig{
							Enabled:                   true,
							CNIIface:                  "enp3s0",
							CalicoInterfaceAutodetect: "interface",
							AutodetectCIDR:            "",
							EncapsulationType:         "VXLAN",
							NATOutgoing:               true,
							Modules: CalicoModulesConfig{
								Calico: CalicoModuleConfig{
									Source: "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/calico?ref=main",
								},
							},
						},
						Cilium: CiliumConfig{
							Enabled:              false,
							OperatorEnabled:      true,
							KubeProxyReplacement: true,
							Modules: CiliumModulesConfig{
								Cilium: CiliumModuleConfig{
									Source: "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/cilium?ref=main",
								},
							},
						},
						KubeOVN: KubeOVNConfig{
							Enabled:           false,
							CiliumIntegration: true,
							Modules: KubeOVNModulesConfig{
								KubeOVN: KubeOVNModuleConfig{
									Source: "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cni/kube-ovn?ref=main",
								},
							},
						},
					},
					Modules: KubernetesModulesConfig{
						KubesprayCluster: KubesprayClusterModuleConfig{
							Source: "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/provider/kubespray?ref=main",
						},
					},
					OIDC: OIDCConfig{
						Enabled:                false,
						KubeOIDCURL:            "",
						KubeOIDCClientID:       "kubernetes",
						KubeOIDCCAFile:         "",
						KubeOIDCUsernameClaim:  "sub",
						KubeOIDCUsernamePrefix: "oidc:",
						KubeOIDCGroupsClaim:    "groups",
						KubeOIDCGroupsPrefix:   "oidc:",
					},
					WindowsWorkers: WindowsWorkers{
						Enabled:                  false,
						WindowsUser:              "Administrator",
						WindowsAdminPassword:     "",
						WorkerNodeBFVSizeWindows: 0,
						WorkerNodeBFVTypeWindows: "",
					},
				},
			},
			GitOps: GitOpsConfig{
				GitDir:            fmt.Sprintf("./testdata/test-git-repo-%s", name),
				GitURL:            "",
				GitSSHKey:         "",
				GitSSHPub:         "",
				GitBranch:         "main",
				GitOpsBaseRepo:    "ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git",
				GitOpsBaseRelease: "v0.1.0",
				GitOpsBranch:      "main",
				Flux: GitOpsFlux{
					Interval: "5m",
					Prune:    true,
				},
			},
			Storage: StorageConfig{
				DefaultStorageClass:         "csi-cinder-sc-delete",
				WorkerVolumeSize:            40,
				WorkerVolumeDestinationType: "volume",
				WorkerVolumeSourceType:      "image",
				WorkerVolumeType:            "HA-Standard",
				AdditionalBlockDevices:      []map[string]any{},
			},
			Talos: nil, // Talos is disabled by default, can be enabled by user
			ManagedService: ServiceMap{
				"alert-proxy": &services.AlertProxyConfig{
					BaseConfig: services.BaseConfig{
						Enabled:             false, // Disabled by default - requires device ID, service token, and account number
						ImageRepository:     "ghcr.io/opencenter-cloud/alert-proxy",
						ImageTag:            "latest",
						GitOpsSourceRepo:    "ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git",
						GitOpsSourceRelease: "v0.1.0",
						GitOpsSourceBranch:  "main",
					},
					AlertManagerBaseUrl: "",
					HTTPRouteFQDN:       fmt.Sprintf("https://alerts.%s.%s.k8s.opencenter.cloud", name, region),
				},
			},
			Services: ServiceMap{
				"calico": &services.CalicoConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true,
					},
					KubeAPIServer: fmt.Sprintf("https://api.%s.%s.k8s.opencenter.cloud:6443", name, region),
				},
				"cert-manager": &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true, // Enabled by default for OpenStack provider
					},
					Email:             "mpk-support@rackspace.com",
					Region:            "us-east-1",
					LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
				},
				"etcd-backup": &services.EtcdBackupConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true,
					},
					S3Host:   "https://swift.api.dfw3.rackspacecloud.com",
					S3Region: "DFW3",
				},
				"external-snapshotter": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"fluxcd":               &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"gateway":              &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"gateway-api":          &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"harbor": &services.HarborConfig{
					BaseConfig: services.BaseConfig{
						Enabled: false,
					},
				},
				"headlamp": &services.HeadlampConfig{
					BaseConfig: services.BaseConfig{
						Enabled:  true, // Enabled by default for OpenStack provider
						Hostname: fmt.Sprintf("dashboard.%s.%s.k8s.opencenter.cloud", name, region),
					},
					OIDCIssuerURL: fmt.Sprintf("https://auth.%s.%s.k8s.opencenter.cloud/realms/opencenter", name, region),
					OIDCClientID:  "kubernetes",
				},
				"keycloak": &services.KeycloakConfig{
					BaseConfig: services.BaseConfig{
						Enabled:  true, // Enabled by default for OpenStack provider
						Hostname: fmt.Sprintf("auth.%s.%s.k8s.opencenter.cloud", name, region),
					},
					Realm:       "opencenter",
					ClientID:    "kubernetes",
					FrontendURL: fmt.Sprintf("https://auth.%s.%s.k8s.opencenter.cloud", name, region),
				},
				"kafka-cluster": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
				"kube-prometheus-stack": &services.PrometheusStackConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true, // Enabled by default for OpenStack provider
					},
					PrometheusVolumeSize:     50,
					PrometheusStorageClass:   "csi-cinder-sc-delete",
					GrafanaVolumeSize:        10,
					GrafanaStorageClass:      "csi-cinder-sc-delete",
					AlertmanagerVolumeSize:   10,
					AlertmanagerStorageClass: "csi-cinder-sc-delete",
				},
				"kyverno": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"loki": &services.LokiConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true, // Enabled by default for OpenStack provider
					},
					VolumeSize:      20,
					StorageClass:    "csi-cinder-sc-delete",
					BucketName:      fmt.Sprintf("%s-loki", name),
					SwiftAuthURL:    fmt.Sprintf("https://keystone.api.%s.rackspacecloud.com/v3/", strings.ToLower(region)),
					SwiftRegion:     strings.ToUpper(region),
					SwiftDomainName: "Default",
				},
				"longhorn": &services.LonghornConfig{
					BaseConfig: services.BaseConfig{
						Enabled: false,
					},
				},
				"metallb": &services.MetalLBConfig{
					BaseConfig: services.BaseConfig{
						Enabled: false,
					},
				},
				"mimir":         &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
				"olm":           &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"openstack-ccm": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"openstack-csi": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"opentelemetry-kube-stack": &services.OpenTelemetryConfig{
					BaseConfig: services.BaseConfig{
						Enabled: false,
					},
				},
				"postgres-operator": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"rbac-manager":      &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"sealed-secrets":    &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
				"sources":           &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"tempo": &services.TempoConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true, // Enabled by default for OpenStack provider
					},
					StorageType:      "s3",
					BucketName:       fmt.Sprintf("%s-tempo", name),
					VolumeSize:       10,
					StorageClass:     "csi-cinder-sc-delete",
					S3Endpoint:       fmt.Sprintf("https://swift.api.%s.rackspacecloud.com", strings.ToLower(region)),
					S3Region:         strings.ToUpper(region),
					S3ForcePathStyle: false,
					S3Insecure:       false,
				},
				"velero": &services.VeleroConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true, // Enabled by default for OpenStack provider
					},
					BackupBucket: fmt.Sprintf("%s-backups", name),
					Region:       "us-east-1",
				},
				"vsphere-csi": &services.VSphereCSIConfig{
					BaseConfig: services.BaseConfig{
						Enabled:         false, // Disabled by default, only for VMware environments
						ImageRepository: "registry.k8s.io/csi-vsphere",
						ImageTag:        "v3.3.0",
					},
				},
				"weave-gitops": &services.WeaveGitOpsConfig{
					BaseConfig: services.BaseConfig{
						Enabled:  false,
						Hostname: fmt.Sprintf("gitops.%s.%s.k8s.opencenter.cloud", name, region),
					},
				},
			},
		},
		OpenTofu: SimplifiedOpenTofu{
			Enabled: true,
			Path:    "opentofu",
			Backend: SimplifiedTofuBackend{
				Type: "local",
				Local: SimplifiedTofuLocal{
					Path: fmt.Sprintf(".opentofu-local-%s/terraform.tfstate", name),
				},
				S3: SimplifiedTofuS3{
					Bucket: "",
					Key:    "",
					Region: "",
				},
			},
		},
		Deployment: Deployment{
			AutoDeploy: true,
			Method:     "kubespray",
		},
		Metadata: NewConfigMetadata(),
		Secrets: Secrets{
			SopsAgeKeyFile: "",
			SSHKey: SSHKey{
				Private: fmt.Sprintf("./testdata/test-git-repo-%s/%s/secrets/ssh/%s", name, name, name),
				Public:  fmt.Sprintf("./testdata/test-git-repo-%s/%s/secrets/ssh/%s.pub", name, name, name),
				Cypher:  "ed25519",
			},
			// Global secrets organized by scope
			Global: GlobalSecrets{
				AWS: AWSGlobalSecrets{
					Infrastructure: AWSSecrets{
						AccessKey:       "",
						SecretAccessKey: "",
						Region:          "us-east-1",
					},
					Application: AWSSecrets{
						AccessKey:       "",
						SecretAccessKey: "",
						Region:          "",
					},
				},
			},
			// Service-specific secrets - must be provided by user
			// These are intentionally left empty even in test mode to ensure
			// validation tests work correctly
			CertManager: CertManagerSecrets{
				AWSAccessKey:       "",
				AWSSecretAccessKey: "",
			},
			Loki: LokiSecrets{
				SwiftPassword: "",
			},
			Keycloak: KeycloakSecrets{
				ClientSecret:  "",
				AdminPassword: "",
			},
			Headlamp: HeadlampSecrets{
				OIDCClientSecret: "",
			},
			WeaveGitOps: WeaveGitOpsSecrets{
				Password:     "",
				PasswordHash: "",
			},
			Grafana: GrafanaSecrets{
				AdminPassword: "",
			},
			Tempo: TempoSecrets{
				AccessKey: "",
				SecretKey: "",
			},
			AlertProxy: AlertProxySecrets{
				CoreDeviceId:        "",
				AccountServiceToken: "",
				CoreAccountNumber:   "",
			},
			VSphereCsi: VSphereCsiSecrets{
				VCenterHost:  "",
				Username:     "",
				Password:     "",
				Datacenters:  "",
				InsecureFlag: "false",
				Port:         "443",
			},
		},
	}

	return cfg
}

// NewDefault returns a Config initialized with the default values for the given cluster name.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - Config: A new Config object with default values.
func NewDefault(name string) Config {
	return defaultConfig(name)
}

// DefaultTalosConfig returns a TalosConfig initialized with secure default values.
// This function should be called when enabling Talos for a cluster.
//
// Inputs:
//   - clusterName: The name of the cluster.
//
// Outputs:
//   - *TalosConfig: A new TalosConfig object with default values.
func DefaultTalosConfig(clusterName string) *TalosConfig {
	talosVersion := "v1.8.0"
	return &TalosConfig{
		Enabled:        true,
		Version:        talosVersion,
		ImageURL:       fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/openstack-amd64.raw.xz", talosVersion),
		ImageSignature: "",
		MachineConfig: TalosMachineConfig{
			AppArmorEnabled:  true,
			SeccompEnabled:   true,
			DiskEncryption:   true,
			KubePrismEnabled: true,
			SystemExtensions: []string{},
			LogDestination:   "",
		},
		NetworkConfig: TalosNetworkConfig{
			ManagementSubnet: "10.0.1.0/24",
			ControlSubnet:    "10.0.2.0/24",
			DataSubnet:       "10.0.3.0/24",
			WireGuardPort:    51820,
			TalosAPIPort:     50000,
			AllowedCIDRs:     []string{},
		},
		SecurityConfig: TalosSecurityConfig{
			VTPMEnabled:       true,
			BarbicanKeyID:     "",
			ImageVerification: true,
			MFARequired:       true,
			AuditLogEnabled:   true,
		},
		PulumiConfig: TalosPulumiConfig{
			StackName:         fmt.Sprintf("%s-talos", clusterName),
			SwiftContainer:    fmt.Sprintf("%s-pulumi-state", clusterName),
			SwiftPrefix:       clusterName,
			SecretsPassphrase: "",
		},
	}
}

// applyOrganizationDefaults applies organization-based defaults to the configuration.
// This ensures that S3 bucket names use the organization name (lowercase) by default.
func applyOrganizationDefaults(cfg *Config) {
	// Set S3 bucket to organization name (lowercase) if not explicitly set by user
	// Only apply if the bucket is still the default (cluster name)
	organization := cfg.OpenCenter.Meta.Organization
	if organization == "" {
		organization = cfg.ClusterName()
	}

	// If S3 bucket is set to the cluster name (default), update it to organization
	if cfg.OpenTofu.Backend.S3.Bucket == strings.ToLower(cfg.ClusterName()) {
		cfg.OpenTofu.Backend.S3.Bucket = strings.ToLower(organization)
	}

	// Ensure bucket name is always lowercase
	if cfg.OpenTofu.Backend.S3.Bucket != "" {
		cfg.OpenTofu.Backend.S3.Bucket = strings.ToLower(cfg.OpenTofu.Backend.S3.Bucket)
	}
}

// applyCLIDefaults applies CLI configuration defaults to the cluster configuration.
// This allows users to set default values in their CLI config that will be applied
// to cluster configurations when they are loaded.
func applyCLIDefaults(cfg *Config) {
	// Try to load CLI config manager
	cm, err := NewConfigManager("")
	if err != nil {
		// If CLI config can't be loaded, skip applying defaults
		return
	}

	cliConfig := cm.GetConfig()
	if cliConfig == nil {
		return
	}

	// Apply provider default if not set in cluster config (skip template strings)
	if cfg.OpenCenter.Infrastructure.Provider == "" && cliConfig.Defaults.Provider != "" && !strings.Contains(cliConfig.Defaults.Provider, "{{") {
		cfg.OpenCenter.Infrastructure.Provider = cliConfig.Defaults.Provider
	}

	// Apply region default if not set in cluster config (skip template strings)
	if cfg.OpenCenter.Meta.Region == "" && cliConfig.Defaults.Region != "" && !strings.Contains(cliConfig.Defaults.Region, "{{") {
		cfg.OpenCenter.Meta.Region = cliConfig.Defaults.Region
	}

	// Apply environment default if not set in cluster config (skip template strings)
	if cfg.OpenCenter.Meta.Env == "" && cliConfig.Defaults.Environment != "" && !strings.Contains(cliConfig.Defaults.Environment, "{{") {
		cfg.OpenCenter.Meta.Env = cliConfig.Defaults.Environment
	}

	// Apply SSH authorized keys default if not set in cluster config
	// Check if SSH keys are empty or contain only empty strings
	hasValidKeys := false
	for _, key := range cfg.OpenCenter.Cluster.SSHAuthorizedKeys {
		if key != "" {
			hasValidKeys = true
			break
		}
	}
	if !hasValidKeys && len(cliConfig.Defaults.SSHAuthorizedKeys) > 0 {
		cfg.OpenCenter.Cluster.SSHAuthorizedKeys = cliConfig.Defaults.SSHAuthorizedKeys
	}
}

// ApplyDefaults applies default values to a configuration.
// This includes CLI defaults, organization defaults, and provider defaults.
//
// Inputs:
//   - cfg: The configuration to apply defaults to
func ApplyDefaults(cfg *Config) {
	applyOrganizationDefaults(cfg)
	applyCLIDefaults(cfg)
}
