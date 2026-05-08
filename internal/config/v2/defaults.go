package v2

import (
	cryptorand "crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	registrydefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

const (
	defaultSchemaVersion              = "2.0"
	defaultOrganization               = "opencenter"
	defaultProvider                   = "openstack"
	defaultRegion                     = "dfw3"
	defaultEnvironment                = "dev"
	defaultBaseDomain                 = "k8s.opencenter.cloud"
	defaultGitBranch                  = "main"
	defaultGitURLPlaceholder          = "ssh://git@example.com/opencenter/cluster-config.git"
	defaultHTTPSGitURLPlaceholder     = "https://github.com/opencenter/cluster-config.git"
	defaultGitBaseRepoURL             = "ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git"
	defaultGitBaseRepoRelease         = "2026.01"
	defaultTopsAuthMethod             = "token"
	defaultDefaultStorageClass        = "Performance"
	defaultWorkerVolumeType           = "Performance"
	defaultOpenStackProjectID         = "project-id-placeholder"
	defaultOpenStackProjectName       = "project-name-placeholder"
	defaultOpenStackNetworkID         = ""
	defaultOpenStackSubnetID          = ""
	defaultOpenStackExternalNetworkID = "external-network-id-placeholder"
	defaultOpenStackFloatingPool      = "PUBLICNET"
	defaultOpenStackImageID           = ""
	defaultVMwareVCenter              = "vcenter.example.com"
	defaultVMwareDatacenter           = "Datacenter1"
	defaultVMwareCluster              = "Cluster1"
	defaultVMwareDatastore            = "datastore1"
	defaultVMwareNetwork              = "VM Network"
	defaultVMwareTemplate             = "ubuntu-2404-template"
	defaultSSHKeyPlaceholder          = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExamplePublicKeyDataHere user@example.com"
	defaultAllocationPoolStart        = "10.2.128.10"
	defaultAllocationPoolEnd          = "10.2.131.250"
	defaultGateway                    = "10.2.128.1"
	defaultVRRPIP                     = "10.2.128.5"
	defaultCiliumVersion              = "1.19.3"

	// Kind provider defaults — kept in sync with internal/config/defaults/kind.yaml.
	kindDefaultKubernetesVersion = "1.36.0"
	kindDefaultAPIPort           = 6443
	kindDefaultControlPlaneCount = 1
	kindDefaultWorkerCount       = 2
	kindDefaultPodSubnet         = "10.244.0.0/16"
	kindDefaultServiceSubnet     = "10.96.0.0/16"
)

type cliDefaults struct {
	Provider          string   `yaml:"provider"`
	Region            string   `yaml:"region"`
	Environment       string   `yaml:"environment"`
	TopsAuthMethod    string   `yaml:"tops_auth_method"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys"`
	BaseDomain        string   `yaml:"base_domain"`
	AdminEmail        string   `yaml:"admin_email"`
	KubernetesVersion string   `yaml:"kubernetes_version"`
	CNI               string   `yaml:"cni"`
	SSHUser           string   `yaml:"ssh_user"`
}

type cliConfig struct {
	ClusterDefaults cliDefaults `yaml:"cluster_defaults"`
}

// NewV2Default creates a schema-valid v2 configuration with init defaults.
func NewV2Default(name, provider string) (*Config, error) {
	defaults := loadCLIDefaults()
	selectedProvider := canonicalInfrastructureProvider(strings.TrimSpace(provider))
	if selectedProvider == "" {
		selectedProvider = canonicalInfrastructureProvider(defaults.Provider)
	}
	if selectedProvider == "" {
		selectedProvider = defaultProvider
	}

	region := strings.TrimSpace(defaults.Region)
	if region == "" {
		region = defaultRegion
	}
	environment := strings.TrimSpace(defaults.Environment)
	if environment == "" {
		environment = defaultEnvironment
	}

	clusterFQDN := fmt.Sprintf("%s.%s.%s", name, region, defaultBaseDomain)
	sshKeyBase := fmt.Sprintf("%s-%s-%s", name, environment, region)
	sshKeyPath := filepath.ToSlash(filepath.Join("secrets", "ssh", sshKeyBase))
	sopsAgeKeyPath := filepath.ToSlash(filepath.Join("secrets", "age", "keys", fmt.Sprintf("%s-key.txt", name)))
	now := time.Now().Format(time.RFC3339Nano)
	availabilityZone := "az1"
	providerDefaults, err := lookupProviderDefaults(selectedProvider, region)
	if err == nil {
		azs := providerDefaults.GetAvailabilityZones()
		if len(azs) > 0 {
			availabilityZone = azs[0]
		}
	}

	bastionEnabled := selectedProvider != "kind" && selectedProvider != "baremetal"
	bastionFlavor := defaultFlavorForProvider(selectedProvider, region, "bastion")
	if bastionFlavor == "" {
		bastionEnabled = false
	}

	grafanaAdminPassword, err := randomSecret(16)
	if err != nil {
		return nil, fmt.Errorf("generating grafana admin password: %w", err)
	}

	cfg := &Config{
		SchemaVersion: defaultSchemaVersion,
		Metadata: ConfigMetadata{
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: currentUser(),
		},
		OpenCenter: OpenCenterConfig{
			Meta: MetaConfig{
				Name:         name,
				Organization: defaultOrganization,
				Env:          environment,
				Region:       region,
				Stage:        "init",
				Status:       "success",
			},
			Secrets: OpenCenterSecrets{
				Backend: "sops",
			},
			Identity: IdentityConfig{
				OIDC: IdentityOIDCConfig{
					Enabled:  true,
					Source:   OIDCSourceInternal,
					Provider: OIDCProviderKeycloak,
				},
			},
			Cluster: ClusterConfig{
				ClusterName: name,
				BaseDomain:  defaultBaseDomain,
				ClusterFQDN: clusterFQDN,
				AdminEmail:  "admin@example.com",
				Kubernetes: KubernetesConfig{
					Version:        "1.36.0",
					APIPort:        443,
					KubeVIPEnabled: selectedProvider != "kind",
					SubnetPods:     "10.42.0.0/16",
					SubnetServices: "10.43.0.0/16",
					NetworkPlugin: NetworkPluginConfig{
						Calico: &CalicoConfig{
							Enabled:       true,
							Version:       "3.32.0",
							VXLANMode:     "Always",
							NetworkPolicy: true,
							InstallMethod: "helm",
						},
					},
					StoragePlugin: storagePluginDefaults(selectedProvider),
					Security: KubernetesSecurityConfig{
						PodSecurityStandards: "baseline",
						AuditLogging:         true,
						EncryptionAtRest:     true,
					},
					OIDC: OIDCConfig{
						Enabled:       false,
						UsernameClaim: "sub",
						GroupsClaim:   "groups",
					},
				},
			},
			Infrastructure: InfrastructureConfig{
				Provider: selectedProvider,
				SSH: SSHConfig{
					AuthorizedKeys: defaultAuthorizedKeys(defaults),
					Username:       "ubuntu",
					User:           "ubuntu",
					KeyPath:        sshKeyPath,
				},
				OSVersion:           "24",
				ServerGroupAffinity: []string{"anti-affinity"},
				NodeNaming: NodeNamingConfig{
					Prefix: name,
					Suffix: region,
				},
				Bastion: BastionConfig{
					Enabled: bastionEnabled,
					Flavor:  bastionFlavor,
					Image:   defaultImageForProvider(selectedProvider, region),
				},
				Networking: NetworkingConfig{
					SubnetNodes:          "10.2.128.0/22",
					AllocationPoolStart:  defaultAllocationPoolStart,
					AllocationPoolEnd:    defaultAllocationPoolEnd,
					Gateway:              defaultGateway,
					VRRPEnabled:          selectedProvider != "kind",
					VRRPIP:               defaultVRRPIP,
					LoadbalancerProvider: "ovn",
					UseDesignate:         false,
					DNSZoneName:          clusterFQDN,
					DNSNameservers:       defaultDNSServers(selectedProvider, region),
					NTPServers:           defaultNTPServers(selectedProvider, region),
					Security: NetworkSecurityConfig{
						AllowedCIDRs: []string{"0.0.0.0/0"},
						DenyAll:      false,
					},
				},
				Compute: ComputeConfig{
					FlavorBastion:       bastionFlavor,
					FlavorMaster:        defaultFlavorForProvider(selectedProvider, region, "master"),
					FlavorWorker:        defaultFlavorForProvider(selectedProvider, region, "worker"),
					FlavorWorkerWindows: defaultFlavorForProvider(selectedProvider, region, "worker-windows"),
					MasterCount:         3,
					WorkerCount:         2,
					WorkerCountWindows:  0,
				},
				Storage: StorageConfig{
					DefaultStorageClass:             defaultStorageClass(selectedProvider, region),
					WorkerVolumeSize:                40,
					WorkerVolumeDestinationType:     "volume",
					WorkerVolumeSourceType:          "image",
					WorkerVolumeType:                defaultStorageType(selectedProvider),
					WorkerVolumeDeleteOnTermination: false,
					MasterVolumeSize:                40,
					MasterVolumeDestinationType:     "volume",
					MasterVolumeSourceType:          "image",
					MasterVolumeType:                defaultStorageType(selectedProvider),
					MasterVolumeDeleteOnTermination: false,
				},
			},
			GitOps: GitOpsConfig{
				Repository: GitOpsRepository{
					Branch:   defaultGitBranch,
					Path:     filepath.ToSlash(filepath.Join("clusters", name)),
					LocalDir: filepath.ToSlash(filepath.Join("clusters", defaultOrganization)),
				},
				BaseRepo: GitOpsBaseRepo{
					URL:     defaultGitBaseRepoURL,
					Release: defaultGitBaseRepoRelease,
					Branch:  defaultGitBranch,
				},
				Auth: GitOpsAuth{},
				Flux: GitOpsFluxConfig{
					Interval: "5m",
					Prune:    true,
				},
			},
			ManagedServices: ServiceMap{
				"alert-proxy": &services.AlertProxyConfig{
					BaseConfig: services.BaseConfig{
						Enabled:  false,
						Hostname: fmt.Sprintf("alerts.%s", clusterFQDN),
					},
					AlertManagerBaseUrl: "",
					HTTPRouteFQDN:       fmt.Sprintf("alerts.%s", clusterFQDN),
				},
			},
			Services: defaultServiceMap(clusterFQDN),
		},
		Deployment: DeploymentConfig{
			AutoDeploy: true,
			Method:     "kubespray",
			Kubespray: &KubesprayConfig{
				Version: "2.31.0",
				Modules: map[string]ModuleConfig{
					"kubespray": {
						Enabled: true,
						Source:  "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/provider/kubespray?ref=main",
					},
				},
			},
		},
		OpenTofu: OpenTofuConfig{
			Enabled: selectedProvider != "kind",
			Path:    "",
			Backend: BackendConfig{
				Type: "local",
				Local: &LocalBackendConfig{
					Path: fmt.Sprintf(".opentofu-local-%s/terraform.tfstate", name),
				},
			},
		},
		Secrets: SecretsConfig{
			SSHKey: SSHKeyConfig{
				Private: sshKeyPath,
				Public:  sshKeyPath + ".pub",
				Cypher:  "ed25519",
			},
			SopsAgeKeyFile: sopsAgeKeyPath,
			Global:         GlobalSecrets{},
			Keycloak: KeycloakSecrets{
				ClientSecret:  PlaceholderSecret,
				AdminPassword: PlaceholderSecret,
			},
			Headlamp: HeadlampSecrets{
				OIDCClientSecret: PlaceholderSecret,
			},
			Grafana: GrafanaSecrets{
				AdminPassword: grafanaAdminPassword,
			},
			Loki: LokiSecrets{
				SwiftApplicationCredentialSecret: PlaceholderSecret,
			},
			Tempo: TempoSecrets{
				SwiftApplicationCredentialSecret: PlaceholderSecret,
			},
			SOPSConfig: SOPSConfig{
				Enabled:        true,
				AgeKeyFile:     sopsAgeKeyPath,
				EncryptedRegex: "^(data|stringData|secret)$",
			},
		},
	}

	applyProviderCloudDefaults(cfg, availabilityZone)
	applyProviderBehaviorDefaults(cfg)
	applyGitOpsAuthDefaults(cfg, defaults.TopsAuthMethod, sshKeyPath)

	return cfg, nil
}

// NewV2FullTemplate returns a more explicit v2 template for --full-schema output.
func NewV2FullTemplate(name, provider string) (*Config, error) {
	cfg, err := NewV2Default(name, provider)
	if err != nil {
		return nil, err
	}

	cfg.Metadata.Version = "template"
	cfg.Metadata.Labels = map[string]string{
		"environment": cfg.OpenCenter.Meta.Env,
		"owner":       "platform-team",
	}
	cfg.Metadata.Annotations = map[string]string{
		"note": "Replace placeholder values before deployment.",
	}

	cfg.OpenCenter.Secrets.Barbican = BarbicanConfig{
		AuthURL:           "https://barbican.example.com/v1",
		ProjectID:         defaultOpenStackProjectID,
		Region:            cfg.OpenCenter.Meta.Region,
		UserDomainName:    "default",
		ProjectDomainName: "default",
		CACert:            "/etc/ssl/certs/ca.pem",
	}

	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium = &CiliumConfig{
		Enabled:       false,
		Version:       "1.16.0",
		TunnelMode:    "vxlan",
		Hubble:        true,
		NetworkPolicy: true,
	}
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN = &KubeOVNConfig{
		Enabled:       false,
		Version:       "1.13.0",
		NetworkPolicy: true,
	}
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.AwsEbsCsi = &AwsEbsCsiConfig{Enabled: false, Version: "1.37.0"}
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.AzureDiskCsi = &AzureDiskCsiConfig{Enabled: false, Version: "1.30.0"}
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Ceph = &CephCsiConfig{Enabled: false, Version: "3.11.0"}
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.GcpComputeCsi = &GcpComputeCsiConfig{Enabled: false, Version: "1.13.0"}
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Trident = &TridentCsiConfig{Enabled: false, Version: "24.06.0"}
	cfg.OpenCenter.Cluster.Kubernetes.Security.AdmissionControllers = []string{"NodeRestriction"}
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.UsernameClaim = "sub"
	cfg.OpenCenter.Cluster.Kubernetes.OIDC.GroupsClaim = "groups"
	cfg.OpenCenter.Infrastructure.Networking.VLAN = VLANConfig{
		Enabled: false,
		ID:      100,
	}
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = []WorkerPoolConfig{
		{
			Name:   "gpu-workers",
			Count:  1,
			Flavor: "worker-gpu-placeholder",
			Image:  defaultImageForProvider(cfg.OpenCenter.Infrastructure.Provider, cfg.OpenCenter.Meta.Region),
			BootVolume: VolumeConfig{
				Size:                80,
				Type:                defaultStorageType(cfg.OpenCenter.Infrastructure.Provider),
				DestinationType:     "volume",
				SourceType:          "image",
				DeleteOnTermination: false,
			},
			Labels: map[string]string{"accelerator": "gpu"},
		},
	}
	cfg.OpenCenter.Infrastructure.Storage.AdditionalBlockDevices = []BlockDeviceConfig{
		{
			Name:                "logs",
			Size:                100,
			Type:                defaultStorageType(cfg.OpenCenter.Infrastructure.Provider),
			MountPath:           "/var/log",
			DeleteOnTermination: false,
		},
	}
	// BaseRepo settings are already set in NewV2Default, but we can override here if needed
	cfg.OpenCenter.GitOps.BaseRepo.Release = defaultGitBaseRepoRelease
	cfg.OpenCenter.GitOps.BaseRepo.Branch = defaultGitBranch
	cfg.OpenCenter.GitOps.BaseRepo.URL = defaultGitBaseRepoURL

	return cfg, nil
}

// RenderFullTemplateYAML renders a commented, schema-valid full template.
func RenderFullTemplateYAML(name, provider string) ([]byte, error) {
	cfg, err := NewV2FullTemplate(name, provider)
	if err != nil {
		return nil, err
	}

	return RenderFullTemplateYAMLFromConfig(cfg)
}

// RenderFullTemplateYAMLFromConfig renders a commented, schema-valid full template
// from an already-populated v2 config.
func RenderFullTemplateYAMLFromConfig(cfg *Config) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal v2 full template: %w", err)
	}

	header := strings.Join([]string{
		"# Full v2 cluster configuration template",
		"# Replace placeholder values before deployment.",
		"# Dotted overrides use native v2 paths such as:",
		"#   --opencenter.infrastructure.compute.master_count=5",
		"#   --opencenter.infrastructure.cloud.openstack.project_id=<project-id>",
		"#   --opencenter.gitops.git_url=ssh://git@example.com/org/repo.git",
		"",
	}, "\n")

	return append([]byte(header), ensureDocumentStart(data)...), nil
}

func applyProviderCloudDefaults(cfg *Config, availabilityZone string) {
	switch canonicalInfrastructureProvider(cfg.OpenCenter.Infrastructure.Provider) {
	case "openstack":
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &OpenStackCloudConfig{
			AuthURL:                     fmt.Sprintf("https://keystone.api.%s.rackspacecloud.com/v3/", strings.ToLower(cfg.OpenCenter.Meta.Region)),
			Region:                      cfg.OpenCenter.Meta.Region,
			ProjectID:                   defaultOpenStackProjectID,
			ProjectName:                 defaultOpenStackProjectName,
			ApplicationCredentialID:     PlaceholderSecret,
			ApplicationCredentialSecret: PlaceholderSecret,
			UserDomainName:              "default",
			ProjectDomainName:           "default",
			ImageID:                     defaultImageForProvider("openstack", cfg.OpenCenter.Meta.Region),
			ImageIDWindows:              defaultImageForProvider("openstack", cfg.OpenCenter.Meta.Region),
			NetworkID:                   defaultOpenStackNetworkID,
			SubnetID:                    defaultOpenStackSubnetID,
			FloatingIPPool:              defaultOpenStackFloatingPool,
			RouterExternalNetworkID:     defaultOpenStackExternalNetworkID,
			AvailabilityZone:            availabilityZone,
			AvailabilityZones:           []string{availabilityZone},
			UseOctavia:                  false,
			UseDesignate:                false,
			Networking: &OpenStackNetworkingConfig{
				FloatingIPPool:          defaultOpenStackFloatingPool,
				NetworkID:               defaultOpenStackNetworkID,
				RouterExternalNetworkID: defaultOpenStackExternalNetworkID,
				SubnetID:                defaultOpenStackSubnetID,
				K8sAPIPortACL:           []string{"0.0.0.0/0"},
				Designate: DesignateConfig{
					DNSZoneName: cfg.OpenCenter.Cluster.ClusterFQDN,
				},
				VLAN: VLANConfigLegacy{
					Provider: "physnet1",
				},
			},
			Modules: OpenStackModulesConfig{
				OpenstackNova: OpenstackNovaModuleConfig{
					Source: "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cloud/openstack/openstack-nova?ref=main",
				},
			},
		}
	case "vmware":
		cfg.OpenCenter.Infrastructure.Cloud.VMware = &VMwareCloudConfig{
			VCenterServer: defaultVMwareVCenter,
			Datacenter:    defaultVMwareDatacenter,
			Cluster:       defaultVMwareCluster,
			Datastore:     defaultVMwareDatastore,
			Network:       defaultVMwareNetwork,
			Template:      defaultVMwareTemplate,
			Folder:        "/vm/opencenter",
		}
		cfg.OpenCenter.Infrastructure.Bastion.Enabled = false
	default:
		cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{}
	}
}

func applyProviderBehaviorDefaults(cfg *Config) {
	switch canonicalInfrastructureProvider(cfg.OpenCenter.Infrastructure.Provider) {
	case "kind":
		cfg.OpenCenter.Infrastructure.Kind = &KindCompatibilityConfig{
			ClusterNameOverride:  cfg.ClusterName(),
			KubernetesVersion:    kindDefaultKubernetesVersion,
			ControlPlaneCount:    kindDefaultControlPlaneCount,
			WorkerCount:          kindDefaultWorkerCount,
			APIServerAddress:     "127.0.0.1",
			APIServerPort:        kindDefaultAPIPort,
			PodSubnet:            kindDefaultPodSubnet,
			ServiceSubnet:        kindDefaultServiceSubnet,
			DisableDefaultCNI:    false,
			IngressEnabled:       true,
			KubeconfigPathPolicy: "cluster-owned",
			Registry: KindRegistryConfig{
				Enabled: false,
				Name:    "kind-registry",
				Port:    5001,
			},
		}
		cfg.OpenCenter.Infrastructure.Bastion.Enabled = false
		cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = false
		cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = false
		cfg.OpenCenter.Infrastructure.Networking.VRRPIP = ""
		cfg.OpenCenter.Infrastructure.Networking.DNSZoneName = "cluster.local"

		// Kind-specific defaults aligned with internal/config/defaults/kind.yaml.
		// These override the OpenStack-oriented base values so that
		// `cluster init --type kind` produces a config that works out of the box.
		cfg.OpenCenter.Cluster.Kubernetes.Version = kindDefaultKubernetesVersion
		cfg.OpenCenter.Cluster.Kubernetes.APIPort = kindDefaultAPIPort
		cfg.OpenCenter.Cluster.Kubernetes.SubnetPods = kindDefaultPodSubnet
		cfg.OpenCenter.Cluster.Kubernetes.SubnetServices = kindDefaultServiceSubnet
		cfg.OpenCenter.Infrastructure.Compute.MasterCount = kindDefaultControlPlaneCount
		cfg.OpenCenter.Infrastructure.Compute.WorkerCount = kindDefaultWorkerCount

		// Kind uses token-based HTTPS auth against local Gitea, not SSH.
		// Clear release fields so FluxCD GitRepository sources use branch only.
		cfg.OpenCenter.GitOps.Auth.SSH = nil
		cfg.OpenCenter.GitOps.Auth.Token = &GitOpsTokenAuth{
			Provider: "gitea",
		}
		cfg.OpenCenter.GitOps.BaseRepo.Release = ""

		// The upstream gitops-base repo is public; use HTTPS so no deploy key
		// or opencenter-base secret is needed inside the Kind cluster.
		cfg.OpenCenter.GitOps.BaseRepo.URL = "https://github.com/opencenter-cloud/openCenter-gitops-base.git"

		// Kind-specific service defaults: enable OLM and postgres-operator
		// which are required dependencies for keycloak.
		if svc, ok := cfg.OpenCenter.Services["olm"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = true
			}
		}
		if svc, ok := cfg.OpenCenter.Services["postgres-operator"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = true
			}
		}
		// Disable OpenStack-specific services for Kind provider
		if svc, ok := cfg.OpenCenter.Services["openstack-ccm"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = false
			}
		}
		if svc, ok := cfg.OpenCenter.Services["openstack-csi"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = false
			}
		}
		if svc, ok := cfg.OpenCenter.Services["velero"]; ok {
			if veleroSvc, ok := svc.(*services.VeleroConfig); ok {
				veleroSvc.Enabled = false
			}
		}
	case "baremetal":
		cfg.OpenCenter.Infrastructure.Bastion.Enabled = false
		// Disable OpenStack-specific services for baremetal provider
		if svc, ok := cfg.OpenCenter.Services["openstack-ccm"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = false
			}
		}
		if svc, ok := cfg.OpenCenter.Services["openstack-csi"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = false
			}
		}
		if svc, ok := cfg.OpenCenter.Services["velero"]; ok {
			if veleroSvc, ok := svc.(*services.VeleroConfig); ok {
				veleroSvc.Enabled = false
			}
		}
	case "vmware":
		cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi = &VSphereCsiConfig{
			Enabled: true,
			Version: "3.3.0",
		}
		// Disable OpenStack-specific services for VMware provider
		if svc, ok := cfg.OpenCenter.Services["openstack-ccm"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = false
			}
		}
		if svc, ok := cfg.OpenCenter.Services["openstack-csi"]; ok {
			if defaultSvc, ok := svc.(*services.DefaultServiceConfig); ok {
				defaultSvc.Enabled = false
			}
		}
		if svc, ok := cfg.OpenCenter.Services["velero"]; ok {
			if veleroSvc, ok := svc.(*services.VeleroConfig); ok {
				veleroSvc.Enabled = false
			}
		}
	case "openstack":
		cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi = &CinderCsiConfig{
			Enabled: true,
			Version: "1.30.0",
		}
	}
}

func applyGitOpsAuthDefaults(cfg *Config, authMethod, sshKeyPath string) {
	switch normalizeTopsAuthMethod(authMethod) {
	case "ssh":
		cfg.OpenCenter.GitOps.Repository.URL = defaultGitURLPlaceholder
		cfg.OpenCenter.GitOps.Auth.SSH = &GitOpsSSHAuth{
			PrivateKey: sshKeyPath,
			PublicKey:  sshKeyPath + ".pub",
		}
		cfg.OpenCenter.GitOps.Auth.Token = nil
	default:
		cfg.OpenCenter.GitOps.Repository.URL = defaultHTTPSGitURLPlaceholder
		cfg.OpenCenter.GitOps.Auth.SSH = nil
		cfg.OpenCenter.GitOps.Auth.Token = &GitOpsTokenAuth{
			Provider: "github",
			Token:    PlaceholderSecret,
		}
	}
}

func normalizeTopsAuthMethod(authMethod string) string {
	switch strings.ToLower(strings.TrimSpace(authMethod)) {
	case "ssh":
		return "ssh"
	case "token", "":
		return "token"
	default:
		return defaultTopsAuthMethod
	}
}

func defaultServiceMap(clusterFQDN string) ServiceMap {
	return ServiceMap{
		"calico":               &services.CalicoConfig{BaseConfig: services.BaseConfig{Enabled: true}, KubeAPIServer: ""},
		"cert-manager":         &services.CertManagerConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"etcd-backup":          &services.EtcdBackupConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"external-snapshotter": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"fluxcd":               &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"gateway":              &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"gateway-api":          &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"headlamp": &services.HeadlampConfig{
			BaseConfig: services.BaseConfig{
				Enabled:  true,
				Hostname: fmt.Sprintf("dashboard.%s", clusterFQDN),
			},
		},
		"keycloak": &services.KeycloakConfig{
			BaseConfig: services.BaseConfig{
				Enabled:  true,
				Hostname: fmt.Sprintf("auth.%s", clusterFQDN),
			},
		},
		// Required by keycloak (keycloak-postgres dependsOn postgres-operator-base).
		"postgres-operator": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		// Required by keycloak (oidc-rbac dependsOn rbac-manager-base).
		"rbac-manager": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		// The sources FluxCD Kustomization deploys GitRepository objects for all services.
		"sources":               &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"kube-prometheus-stack": &services.PrometheusStackConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"kyverno":               &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"loki":                  &services.LokiConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"openstack-ccm":         &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"openstack-csi":         &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"tempo":                 &services.TempoConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"velero":                &services.VeleroConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		// Present (disabled) so template conditionals can safely index the key.
		"harbor":  &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		"metallb": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		// Required by keycloak (keycloak-operator is managed by OLM).
		"olm":                      &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		"kafka-cluster":            &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		"vsphere-csi":              &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		"weave-gitops":             &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		"longhorn":                 &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		"mimir":                    &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		"opentelemetry-kube-stack": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		"sealed-secrets":           &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: false}},
	}
}

func storagePluginDefaults(provider string) StoragePluginConfig {
	switch canonicalInfrastructureProvider(provider) {
	case "openstack":
		return StoragePluginConfig{
			CinderCsi: &CinderCsiConfig{Enabled: true, Version: "1.30.0"},
		}
	case "vmware":
		return StoragePluginConfig{
			VSphereCsi: &VSphereCsiConfig{Enabled: true, Version: "3.3.0"},
		}
	default:
		return StoragePluginConfig{}
	}
}

func loadCLIDefaults() cliDefaults {
	defaults := cliDefaults{
		Provider:       defaultProvider,
		Region:         defaultRegion,
		Environment:    defaultEnvironment,
		TopsAuthMethod: defaultTopsAuthMethod,
	}

	configPath, err := defaultCLIConfigPath()
	if err != nil {
		return defaults
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return defaults
	}

	var cfg cliConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return defaults
	}

	if strings.TrimSpace(cfg.ClusterDefaults.Provider) != "" {
		defaults.Provider = cfg.ClusterDefaults.Provider
	}
	if strings.TrimSpace(cfg.ClusterDefaults.Region) != "" {
		defaults.Region = cfg.ClusterDefaults.Region
	}
	if strings.TrimSpace(cfg.ClusterDefaults.Environment) != "" {
		defaults.Environment = cfg.ClusterDefaults.Environment
	}
	if strings.TrimSpace(cfg.ClusterDefaults.TopsAuthMethod) != "" {
		defaults.TopsAuthMethod = normalizeTopsAuthMethod(cfg.ClusterDefaults.TopsAuthMethod)
	}
	if len(cfg.ClusterDefaults.SSHAuthorizedKeys) > 0 {
		defaults.SSHAuthorizedKeys = cfg.ClusterDefaults.SSHAuthorizedKeys
	}
	if strings.TrimSpace(cfg.ClusterDefaults.BaseDomain) != "" {
		defaults.BaseDomain = cfg.ClusterDefaults.BaseDomain
	}
	if strings.TrimSpace(cfg.ClusterDefaults.AdminEmail) != "" {
		defaults.AdminEmail = cfg.ClusterDefaults.AdminEmail
	}
	if strings.TrimSpace(cfg.ClusterDefaults.KubernetesVersion) != "" {
		defaults.KubernetesVersion = cfg.ClusterDefaults.KubernetesVersion
	}
	if strings.TrimSpace(cfg.ClusterDefaults.CNI) != "" {
		defaults.CNI = cfg.ClusterDefaults.CNI
	}
	if strings.TrimSpace(cfg.ClusterDefaults.SSHUser) != "" {
		defaults.SSHUser = cfg.ClusterDefaults.SSHUser
	}

	return defaults
}

func defaultCLIConfigPath() (string, error) {
	configDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if configDir == "" {
		switch runtime.GOOS {
		case "windows":
			base := os.Getenv("APPDATA")
			if base == "" {
				base = os.Getenv("LOCALAPPDATA")
			}
			if base == "" {
				base = os.Getenv("USERPROFILE")
			}
			configDir = filepath.Join(base, "opencenter")
		default:
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(home, ".config", "opencenter")
		}
	}

	return filepath.Join(configDir, "config.yaml"), nil
}

func lookupProviderDefaults(provider, region string) (registrydefaults.ProviderDefaults, error) {
	registry := registrydefaults.GetGlobalRegistry()
	return registry.GetDefaults(canonicalInfrastructureProvider(provider), region)
}

func defaultDNSServers(provider, region string) []string {
	providerDefaults, err := lookupProviderDefaults(provider, region)
	if err == nil {
		if values := providerDefaults.GetDNSNameservers(); len(values) > 0 {
			return append([]string(nil), values...)
		}
	}
	return []string{"8.8.8.8", "8.8.4.4"}
}

func defaultNTPServers(provider, region string) []string {
	providerDefaults, err := lookupProviderDefaults(provider, region)
	if err == nil {
		if values := providerDefaults.GetNTPServers(); len(values) > 0 {
			return append([]string(nil), values...)
		}
	}
	return []string{
		fmt.Sprintf("time.%s.example.com", strings.ToLower(region)),
		fmt.Sprintf("time2.%s.example.com", strings.ToLower(region)),
	}
}

func defaultStorageClass(provider, region string) string {
	providerDefaults, err := lookupProviderDefaults(provider, region)
	if err == nil {
		if value := strings.TrimSpace(providerDefaults.GetDefaultStorageClass()); value != "" {
			return value
		}
	}

	switch canonicalInfrastructureProvider(provider) {
	case "openstack":
		return "csi-cinder-sc-delete"
	case "vmware":
		return "vsphere-csi"
	default:
		return defaultDefaultStorageClass
	}
}

func defaultFlavorForProvider(provider, region, role string) string {
	providerDefaults, err := lookupProviderDefaults(provider, region)
	if err == nil {
		flavors := providerDefaults.GetDefaultFlavors()
		switch role {
		case "bastion":
			if flavors.Bastion != "" {
				return flavors.Bastion
			}
		case "master":
			if flavors.Master != "" {
				return flavors.Master
			}
		case "worker":
			if flavors.Worker != "" {
				return flavors.Worker
			}
		case "worker-windows":
			if flavors.WorkerWindows != "" {
				return flavors.WorkerWindows
			}
		}
	}

	switch canonicalInfrastructureProvider(provider) {
	case "vmware":
		switch role {
		case "bastion":
			return "vmware-bastion"
		case "master":
			return "vmware-master"
		case "worker":
			return "vmware-worker"
		case "worker-windows":
			return "vmware-worker-windows"
		}
	case "baremetal":
		switch role {
		case "bastion":
			return ""
		case "master":
			return "baremetal-master"
		case "worker":
			return "baremetal-worker"
		case "worker-windows":
			return "baremetal-worker-windows"
		}
	case "kind":
		return ""
	}

	return ""
}

func defaultStorageType(provider string) string {
	switch canonicalInfrastructureProvider(provider) {
	case "openstack":
		return "HA-Standard"
	case "vmware":
		return "vsphere-default"
	case "kind":
		return "local-path"
	default:
		return defaultWorkerVolumeType
	}
}

func defaultImageForProvider(provider, region string) string {
	providerDefaults, err := lookupProviderDefaults(provider, region)
	if err == nil {
		if value := strings.TrimSpace(providerDefaults.GetImageID("24")); value != "" {
			return value
		}
	}
	return defaultOpenStackImageID
}

func defaultAuthorizedKeys(defaults cliDefaults) []string {
	if len(defaults.SSHAuthorizedKeys) > 0 {
		return append([]string(nil), defaults.SSHAuthorizedKeys...)
	}
	return []string{defaultSSHKeyPlaceholder}
}

func randomSecret(length int) (string, error) {
	if length < 16 {
		length = 16
	}

	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	randomBytes := make([]byte, length)
	if _, err := cryptorand.Read(randomBytes); err != nil {
		return "", err
	}

	out := make([]byte, length)
	for i, b := range randomBytes {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

func currentUser() string {
	if user := strings.TrimSpace(os.Getenv("USER")); user != "" {
		return user
	}
	if user := strings.TrimSpace(os.Getenv("USERNAME")); user != "" {
		return user
	}
	return "unknown"
}
