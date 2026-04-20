package config

import (
	_ "embed"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

//go:embed defaults/kind.yaml
var kindDefaultsYAML []byte

type kindDefaultsFile struct {
	Locals kindDefaultsLocals `yaml:"locals"`
}

type kindDefaultsLocals struct {
	KubernetesVersion string            `yaml:"kubernetes_version"`
	WorkerCount       int               `yaml:"worker_count"`
	ControlPlaneCount int               `yaml:"control_plane_count"`
	RegistryEnabled   bool              `yaml:"registry_enabled"`
	RegistryName      string            `yaml:"registry_name"`
	RegistryPort      int               `yaml:"registry_port"`
	IngressEnabled    bool              `yaml:"ingress_enabled"`
	ExtraMounts       []KindMount       `yaml:"extra_mounts"`
	PortMappings      []KindPortMapping `yaml:"port_mappings"`
	ServiceSubnet     string            `yaml:"service_subnet"`
	PodSubnet         string            `yaml:"pod_subnet"`
	DisableDefaultCNI bool              `yaml:"disable_default_cni"`
	APIServerAddress  string            `yaml:"api_server_address"`
	APIServerPort     int               `yaml:"api_server_port"`
	GitURL            string            `yaml:"git_url"`
	GitTokenProvider  string            `yaml:"git_token_provider"`
}

func applyProviderDefaults(cfg *Config, provider string) error {
	switch provider {
	case "openstack", "":
		return applyOpenStackDefaults(cfg)
	case "kind":
		return applyKindDefaults(cfg)
	default:
		return nil
	}
}

// ApplyProviderDefaults overlays provider-specific runtime defaults onto a config.
func ApplyProviderDefaults(cfg *Config, provider string) error {
	return applyProviderDefaults(cfg, provider)
}

func applyKindDefaults(cfg *Config) error {
	var defaults kindDefaultsFile
	if err := yaml.Unmarshal(kindDefaultsYAML, &defaults); err != nil {
		return fmt.Errorf("parse embedded kind defaults: %w", err)
	}

	kind := &KindConfig{
		ClusterNameOverride:  cfg.ClusterName(),
		KubernetesVersion:    defaults.Locals.KubernetesVersion,
		ControlPlaneCount:    defaults.Locals.ControlPlaneCount,
		WorkerCount:          defaults.Locals.WorkerCount,
		APIServerAddress:     defaults.Locals.APIServerAddress,
		APIServerPort:        defaults.Locals.APIServerPort,
		PodSubnet:            defaults.Locals.PodSubnet,
		ServiceSubnet:        defaults.Locals.ServiceSubnet,
		DisableDefaultCNI:    defaults.Locals.DisableDefaultCNI,
		IngressEnabled:       defaults.Locals.IngressEnabled,
		KubeconfigPathPolicy: "cluster-owned",
		Registry: KindRegistryConfig{
			Enabled: defaults.Locals.RegistryEnabled,
			Name:    defaults.Locals.RegistryName,
			Port:    defaults.Locals.RegistryPort,
		},
		ExtraPortMappings: defaults.Locals.PortMappings,
		ExtraMounts:       defaults.Locals.ExtraMounts,
	}

	cfg.OpenCenter.Meta.Region = "local"
	cfg.OpenCenter.Infrastructure.Provider = "kind"
	cfg.OpenCenter.Infrastructure.Kind = kind
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = SimplifiedOpenStackCloud{}
	cfg.OpenCenter.Infrastructure.Cloud.AWS = SimplifiedAWSCloud{}
	cfg.OpenCenter.Infrastructure.Cloud.VMware = VMwareCloud{}

	cfg.OpenCenter.Cluster.Kubernetes.Version = kind.KubernetesVersion
	cfg.OpenCenter.Cluster.Kubernetes.APIPort = kind.APIServerPort
	cfg.OpenCenter.Cluster.Kubernetes.SubnetPods = kind.PodSubnet
	cfg.OpenCenter.Cluster.Kubernetes.SubnetServices = kind.ServiceSubnet
	cfg.OpenCenter.Cluster.Kubernetes.MasterCount = kind.ControlPlaneCount
	cfg.OpenCenter.Cluster.Kubernetes.WorkerCount = kind.WorkerCount
	cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = false
	cfg.OpenCenter.Cluster.Networking.VRRPEnabled = false
	cfg.OpenCenter.Cluster.Networking.VRRPIP = ""

	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Cinder.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Vsphere.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.Longhorn.Enabled = false

	cfg.OpenTofu.Enabled = false

	// Set Gitea-based Git defaults for local Kind clusters.
	if defaults.Locals.GitURL != "" {
		cfg.OpenCenter.GitOps.Repository.URL = defaults.Locals.GitURL
	}
	if defaults.Locals.GitTokenProvider != "" {
		if cfg.OpenCenter.GitOps.Auth.Token == nil {
			cfg.OpenCenter.GitOps.Auth.Token = &GitOpsTokenAuth{}
		}
		cfg.OpenCenter.GitOps.Auth.Token.Provider = defaults.Locals.GitTokenProvider
	}

	// Kind-specific service defaults for local development.
	// These reduce resource requirements and disable HA features.
	applyKindServiceDefaults(cfg)

	return nil
}

// applyKindServiceDefaults configures services for local Kind development.
// Reduces resource requirements and disables HA features not needed locally.
func applyKindServiceDefaults(cfg *Config) {
	// Keycloak: single instance, reduced resources, local cache
	if svc, ok := cfg.OpenCenter.Services["keycloak"]; ok {
		if keycloakCfg, ok := svc.(*services.KeycloakConfig); ok {
			keycloakCfg.Instances = 1
			keycloakCfg.MinReplicas = 0 // Disable HPA
			keycloakCfg.MaxReplicas = 0 // Disable HPA
			keycloakCfg.StartOptimized = false
			keycloakCfg.CacheEnabled = true
			keycloakCfg.CacheStack = "local" // Local cache instead of distributed
			keycloakCfg.ResourceRequestsCPU = "250m"
			keycloakCfg.ResourceRequestsMemory = "512M"
			keycloakCfg.ResourceLimitsCPU = "1"
			keycloakCfg.ResourceLimitsMemory = "1024M"
			keycloakCfg.DBPoolMinSize = 5
			keycloakCfg.DBPoolInitialSize = 5
			keycloakCfg.DBPoolMaxSize = 10
			keycloakCfg.BackupEnabled = false // No backup for local dev
		}
	}

	// Use standard storage class for Kind (local-path provisioner)
	cfg.OpenCenter.Storage.DefaultStorageClass = "standard"
}
