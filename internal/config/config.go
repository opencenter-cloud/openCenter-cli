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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the simplified root configuration for a cluster based on the new schema.
// The structure matches the testdata/schema.yaml format with opencenter, opentofu, cloud, and secrets sections.
// IAC field is included for backward compatibility with existing templates.
type Config struct {
	OpenCenter SimplifiedOpenCenter `yaml:"opencenter" json:"opencenter"`
	OpenTofu   SimplifiedOpenTofu   `yaml:"opentofu" json:"opentofu"`
	Secrets    Secrets              `yaml:"secrets" json:"secrets"`
	Overrides  map[string]any       `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	IAC        IAC                  `yaml:"-" json:"-"` // Hidden from YAML output, for template compatibility
}

// ClusterMeta holds high-level metadata about the cluster.
type ClusterMeta struct {
	Name   string `yaml:"name" json:"name"`
	Env    string `yaml:"env" json:"env"`
	Region string `yaml:"region" json:"region"`
	Status string `yaml:"status" json:"status"`
}

// OpenTofu holds OpenTofu-specific settings.
type OpenTofu struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Path    string      `yaml:"path" json:"path"`
	Backend TofuBackend `yaml:"backend" json:"backend"`
}

// TofuBackend describes the state backend configuration for OpenTofu.
// Type can be "local" or "s3". When "local", Backend.Local.Path is used.
// When "s3", Backend.S3 fields are used.
type TofuBackend struct {
	Type  string    `yaml:"type" json:"type"`
	Local TofuLocal `yaml:"local" json:"local"`
	S3    TofuS3    `yaml:"s3" json:"s3"`
}

type TofuLocal struct {
	Path string `yaml:"path" json:"path"`
}

type TofuS3 struct {
	Bucket   string `yaml:"bucket" json:"bucket"`
	Key      string `yaml:"key" json:"key"`
	Region   string `yaml:"region" json:"region"`
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Profile  string `yaml:"profile,omitempty" json:"profile,omitempty"`
	Encrypt  bool   `yaml:"encrypt,omitempty" json:"encrypt,omitempty"`
}

// OpenCenter holds global openCenter-level settings and secrets.
// The AWS credentials here are used by the OpenTofu S3 backend when provided.
type OpenCenter struct {
	AWSAccessKey       string `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key" json:"aws_secret_access_key"`
}

// Ansible holds Ansible-specific settings.
type Ansible struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	Path      string   `yaml:"path" json:"path"`
	Inventory string   `yaml:"inventory,omitempty" json:"inventory,omitempty"`
	Playbooks []string `yaml:"playbooks,omitempty" json:"playbooks,omitempty"`
}

// GitOpsConfig holds configuration related to GitOps scaffolding and repositories.
type GitOpsConfig struct {
	GitDir    string     `yaml:"git_dir" json:"git_dir"`
	GitURL    string     `yaml:"git_url" json:"git_url"`
	GitSSHKey string     `yaml:"git_ssh_key,omitempty" json:"git_ssh_key,omitempty"`
	GitSSHPub string     `yaml:"git_ssh_pub,omitempty" json:"git_ssh_pub,omitempty"`
	GitBranch string     `yaml:"git_branch,omitempty" json:"git_branch,omitempty"`
	Flux      GitOpsFlux `yaml:"flux,omitempty" json:"flux,omitempty"`
}

// GitOpsFlux holds optional FluxCD settings for reconciliation behavior.
type GitOpsFlux struct {
	Interval string `yaml:"interval" json:"interval"`
	Prune    bool   `yaml:"prune" json:"prune"`
}

// Kubernetes groups settings for the Kubernetes cluster.
// It nests further objects for counts, images, flavors, and networking.
// Default values are applied at load time.
// IAC groups settings for infrastructure-as-code driven cluster provisioning.
// It retains the detailed node/layout fields and adds engine/stack selectors.
type IAC struct {
	// Deprecated raw HCL for main.tf; prefer structured fields below
	MainTF string `yaml:"main_tf,omitempty" json:"main_tf,omitempty"`
	// Main contains the values for the Terraform locals (rendered into main.tf)
	Main map[string]any `yaml:"main,omitempty" json:"main,omitempty"`
	// Modules contains per-module attribute maps (rendered into main.tf)
	Modules map[string]any `yaml:"modules,omitempty" json:"modules,omitempty"`
}

// Storage describes block volume storage sizes and types for various node roles.
type Storage struct {
	MasterNodeBFV        BFV `yaml:"master_node_bfv" json:"master_node_bfv"`
	WorkerNodeBFV        BFV `yaml:"worker_node_bfv" json:"worker_node_bfv"`
	WorkerNodeBFVWindows BFV `yaml:"worker_node_bfv_windows" json:"worker_node_bfv_windows"`
}

// BFV describes the block volume size and type.
type BFV struct {
	Size int    `yaml:"size" json:"size"`
	Type string `yaml:"type" json:"type"`
}

// Windows holds Windows-specific settings.
type Windows struct {
	User          string `yaml:"user" json:"user"`
	AdminPassword string `yaml:"admin_password" json:"admin_password"`
}

// Networking groups network settings and options around VRRP and service networks.
type Networking struct {
	SubnetNodes          string   `yaml:"subnet_nodes" json:"subnet_nodes"`
	AllocationPoolStart  string   `yaml:"allocation_pool_start" json:"allocation_pool_start"`
	AllocationPoolEnd    string   `yaml:"allocation_pool_end" json:"allocation_pool_end"`
	VRRPEnabled          bool     `yaml:"vrrp_enabled" json:"vrrp_enabled"`
	VRRPIP               string   `yaml:"vrrp_ip" json:"vrrp_ip"`
	SubnetServices       string   `yaml:"subnet_services" json:"subnet_services"`
	SubnetPods           string   `yaml:"subnet_pods" json:"subnet_pods"`
	UseOctavia           bool     `yaml:"use_octavia" json:"use_octavia"`
	LoadbalancerProvider string   `yaml:"loadbalancer_provider" json:"loadbalancer_provider"`
	UseDesignate         bool     `yaml:"use_designate" json:"use_designate"`
	DNSZoneName          string   `yaml:"dns_zone_name" json:"dns_zone_name"`
	DNSNameservers       []string `yaml:"dns_nameservers" json:"dns_nameservers"`
	VLAN                 VLAN     `yaml:"vlan" json:"vlan"`
}

// VLAN describes VLAN settings for the cluster.
type VLAN struct {
	ID       string `yaml:"id" json:"id"`
	MTU      int    `yaml:"mtu" json:"mtu"`
	Provider string `yaml:"provider" json:"provider"`
}

// Cloud holds provider-specific configuration. Currently, only OpenStack is supported.
type Cloud struct {
	Provider  string         `yaml:"provider" json:"provider"`
	OpenStack OpenStackCloud `yaml:"openstack" json:"openstack"`
	AWS       AWSCloud       `yaml:"aws" json:"aws"`
}

// OpenStackCloud contains options for connecting to an OpenStack deployment.
type OpenStackCloud struct {
	AuthURL                 string `yaml:"auth_url" json:"auth_url"`
	Insecure                bool   `yaml:"insecure" json:"insecure"`
	Region                  string `yaml:"region" json:"region"`
	UserName                string `yaml:"user_name" json:"user_name"`
	UserPassword            string `yaml:"user_password" json:"user_password"`
	AdminPassword           string `yaml:"admin_password" json:"admin_password"`
	ProjectDomainName       string `yaml:"project_domain_name" json:"project_domain_name"`
	UserDomainName          string `yaml:"user_domain_name" json:"user_domain_name"`
	TenantName              string `yaml:"tenant_name" json:"tenant_name"`
	AvailabilityZone        string `yaml:"availability_zone" json:"availability_zone"`
	FloatingIPPool          string `yaml:"floatingip_pool" json:"floatingip_pool"`
	RouterExternalNetworkID string `yaml:"router_external_network_id" json:"router_external_network_id"`
	DisableBastion          bool   `yaml:"disable_bastion" json:"disable_bastion"`
	CA                      string `yaml:"ca" json:"ca"`
	ExternalNetwork         string `yaml:"external_network" json:"external_network"`
	UseOctavia              bool   `yaml:"use_octavia" json:"use_octavia"`
	VRRPIP                  string `yaml:"vrrp_ip" json:"vrrp_ip"`
}

// AWSCloud contains options for connecting to AWS environments.
type AWSCloud struct {
	Profile        string   `yaml:"profile" json:"profile"`
	Region         string   `yaml:"region" json:"region"`
	VPCID          string   `yaml:"vpc_id" json:"vpc_id"`
	PrivateSubnets []string `yaml:"private_subnets" json:"private_subnets"`
	PublicSubnets  []string `yaml:"public_subnets" json:"public_subnets"`
}

// Secrets holds paths or settings for secret management tools.
type Secrets struct {
	SopsAgeKeyFile string `yaml:"sops_age_key_file" json:"sops_age_key_file"`
}

// Simplified structures based on testdata/schema.yaml

// SimplifiedOpenCenter represents the opencenter section of the new simplified schema
type SimplifiedOpenCenter struct {
	Infrastructure Infrastructure        `yaml:"infrastructure" json:"infrastructure"`
	Cluster        ClusterConfig         `yaml:"cluster" json:"cluster"`
	GitOps         GitOpsConfig          `yaml:"gitops" json:"gitops"`
	ManagedService map[string]ServiceCfg `yaml:"managed-service" json:"managed-service"`
	Services       map[string]ServiceCfg `yaml:"services" json:"services"`
}

// Infrastructure represents the infrastructure configuration block.
type Infrastructure struct {
	Provider string      `yaml:"provider" json:"provider"`
	Cloud    CloudConfig `yaml:"cloud" json:"cloud"`
}

// ServiceCfg captures the on/off toggle plus optional metadata for a service.
type ServiceCfg struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	Email             string `yaml:"email,omitempty" json:"email,omitempty"`
	Region            string `yaml:"region,omitempty" json:"region,omitempty"`
	S3Host            string `yaml:"s3_host,omitempty" json:"s3_host,omitempty"`
	S3Region          string `yaml:"s3_region,omitempty" json:"s3_region,omitempty"`
	AWSAccessKey      string `yaml:"aws_access_key,omitempty" json:"aws_access_key,omitempty"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key,omitempty" json:"aws_secret_access_key,omitempty"`
}

// CloudConfig represents the cloud configuration within opencenter
type CloudConfig struct {
	AWS       SimplifiedAWSCloud       `yaml:"aws" json:"aws"`
	OpenStack SimplifiedOpenStackCloud `yaml:"openstack" json:"openstack"`
}

// ClusterConfig represents the cluster configuration section
type ClusterConfig struct {
	ClusterName        string           `yaml:"cluster_name" json:"cluster_name"`
	AWSAccessKey       string           `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretAccessKey string           `yaml:"aws_secret_access_key" json:"aws_secret_access_key"`
	K8sAPIPortACL      []string         `yaml:"k8s_api_port_acl" json:"k8s_api_port_acl"`
	SSHAuthorizedKeys  []string         `yaml:"ssh_authorized_keys" json:"ssh_authorized_keys"`
	Kubernetes         KubernetesConfig `yaml:"kubernetes" json:"kubernetes"`
}

// KubernetesConfig represents the kubernetes configuration
type KubernetesConfig struct {
	Version              string         `yaml:"version" json:"version"`
	FlavorBastion        string         `yaml:"flavor_bastion" json:"flavor_bastion"`
	FlavorMaster         string         `yaml:"flavor_master" json:"flavor_master"`
	FlavorWorker         string         `yaml:"flavor_worker" json:"flavor_worker"`
	SubnetPods           string         `yaml:"subnet_pods" json:"subnet_pods"`
	SubnetServices       string         `yaml:"subnet_services" json:"subnet_services"`
	LoadbalancerProvider string         `yaml:"loadbalancer_provider" json:"loadbalancer_provider"`
	DNSZoneName          string         `yaml:"dns_zone_name" json:"dns_zone_name"`
	MasterCount          int            `yaml:"master_count" json:"master_count"`
	WorkerCount          int            `yaml:"worker_count" json:"worker_count"`
	WorkerCountWindows   int            `yaml:"worker_count_windows" json:"worker_count_windows"`
	NetworkPlugin        NetworkPlugin  `yaml:"network_plugin" json:"network_plugin"`
	OIDC                 OIDCConfig     `yaml:"oidc" json:"oidc"`
	WindowsWorkers       WindowsWorkers `yaml:"windows_workers" json:"windows_workers"`
}

// NetworkPlugin represents the network plugin configuration
type NetworkPlugin struct {
	Calico  CalicoConfig  `yaml:"calico" json:"calico"`
	Cilium  CiliumConfig  `yaml:"cilium" json:"cilium"`
	KubeOVN KubeOVNConfig `yaml:"kube-ovn" json:"kube-ovn"`
}

// CalicoConfig represents the Calico configuration
type CalicoConfig struct {
	Enabled                   bool   `yaml:"enabled" json:"enabled"`
	CNIIface                  string `yaml:"cni_iface" json:"cni_iface"`
	CalicoInterfaceAutodetect string `yaml:"calico_interface_autodetect" json:"calico_interface_autodetect"`
}

// CiliumConfig represents the Cilium configuration
type CiliumConfig struct {
	Enabled              bool `yaml:"enabled" json:"enabled"`
	OperatorEnabled      bool `yaml:"operator_enabled" json:"operator_enabled"`
	KubeProxyReplacement bool `yaml:"kubeProxyReplacement" json:"kubeProxyReplacement"`
}

// KubeOVNConfig represents the Kube-OVN configuration
type KubeOVNConfig struct {
	Enabled           bool `yaml:"enabled" json:"enabled"`
	CiliumIntegration bool `yaml:"cilium_integration" json:"cilium_integration"`
}

// OIDCConfig represents the OIDC configuration
type OIDCConfig struct {
	Enabled                bool   `yaml:"enabled" json:"enabled"`
	KubeOIDCURL            string `yaml:"kube_oidc_url" json:"kube_oidc_url"`
	KubeOIDCClientID       string `yaml:"kube_oidc_client_id" json:"kube_oidc_client_id"`
	KubeOIDCCAFile         string `yaml:"kube_oidc_ca_file" json:"kube_oidc_ca_file"`
	KubeOIDCUsernameClaim  string `yaml:"kube_oidc_username_claim" json:"kube_oidc_username_claim"`
	KubeOIDCUsernamePrefix string `yaml:"kube_oidc_username_prefix" json:"kube_oidc_username_prefix"`
	KubeOIDCGroupsClaim    string `yaml:"kube_oidc_groups_claim" json:"kube_oidc_groups_claim"`
	KubeOIDCGroupsPrefix   string `yaml:"kube_oidc_groups_prefix" json:"kube_oidc_groups_prefix"`
}

// WindowsWorkers represents the Windows workers configuration
type WindowsWorkers struct {
	Enabled                  bool   `yaml:"enabled" json:"enabled"`
	WindowsUser              string `yaml:"windows_user" json:"windows_user"`
	WindowsAdminPassword     string `yaml:"windows_admin_password" json:"windows_admin_password"`
	WorkerNodeBFVSizeWindows int    `yaml:"worker_node_bfv_size_windows" json:"worker_node_bfv_size_windows"`
	WorkerNodeBFVTypeWindows string `yaml:"worker_node_bfv_type_windows" json:"worker_node_bfv_type_windows"`
}

// SimplifiedOpenTofu represents the opentofu section
type SimplifiedOpenTofu struct {
	Enabled bool                  `yaml:"enabled" json:"enabled"`
	Path    string                `yaml:"path" json:"path"`
	Backend SimplifiedTofuBackend `yaml:"backend" json:"backend"`
}

// SimplifiedTofuBackend represents the backend configuration
type SimplifiedTofuBackend struct {
	Type  string              `yaml:"type" json:"type"`
	Local SimplifiedTofuLocal `yaml:"local" json:"local"`
	S3    SimplifiedTofuS3    `yaml:"s3" json:"s3"`
}

// SimplifiedTofuLocal represents the local backend
type SimplifiedTofuLocal struct {
	Path string `yaml:"path" json:"path"`
}

// SimplifiedTofuS3 represents the S3 backend
type SimplifiedTofuS3 struct {
	Bucket string `yaml:"bucket" json:"bucket"`
	Key    string `yaml:"key" json:"key"`
	Region string `yaml:"region" json:"region"`
}

// SimplifiedCloud represents the cloud section
type SimplifiedCloud struct {
	Provider  string                   `yaml:"provider" json:"provider"`
	OpenStack SimplifiedOpenStackCloud `yaml:"openstack" json:"openstack"`
	AWS       SimplifiedAWSCloud       `yaml:"aws" json:"aws"`
}

// SimplifiedOpenStackCloud represents the OpenStack configuration
type SimplifiedOpenStackCloud struct {
	AuthURL                     string `yaml:"auth_url" json:"auth_url"`
	Insecure                    bool   `yaml:"insecure" json:"insecure"`
	Region                      string `yaml:"region" json:"region"`
	ApplicationCredentialID     string `yaml:"application_credential_id" json:"application_credential_id"`
	ApplicationCredentialSecret string `yaml:"application_credential_secret" json:"application_credential_secret"`
}

// SimplifiedAWSCloud represents the AWS configuration
type SimplifiedAWSCloud struct {
	Profile        string   `yaml:"profile" json:"profile"`
	Region         string   `yaml:"region" json:"region"`
	VPCID          string   `yaml:"vpc_id" json:"vpc_id"`
	PrivateSubnets []string `yaml:"private_subnets" json:"private_subnets"`
	PublicSubnets  []string `yaml:"public_subnets" json:"public_subnets"`
}

// defaultConfig returns a Config pre-populated with the default
// values based on the simplified schema. This function can be used to
// initialise new cluster configurations.
func defaultConfig(name string) Config {
	cfg := Config{
		OpenCenter: SimplifiedOpenCenter{
			Infrastructure: Infrastructure{
				Provider: "openstack",
				Cloud: CloudConfig{
					AWS: SimplifiedAWSCloud{
						Profile:        "",
						Region:         "",
						VPCID:          "",
						PrivateSubnets: []string{},
						PublicSubnets:  []string{},
					},
					OpenStack: SimplifiedOpenStackCloud{
						AuthURL:                     "",
						Insecure:                    false,
						Region:                      "",
						ApplicationCredentialID:     "",
						ApplicationCredentialSecret: "",
					},
				},
			},
			Cluster: ClusterConfig{
				ClusterName:        name,
				AWSAccessKey:       "",
				AWSSecretAccessKey: "",
				K8sAPIPortACL:      []string{""},
				SSHAuthorizedKeys:  []string{""},
				Kubernetes: KubernetesConfig{
					Version:              "1.32.8",
					FlavorBastion:        "gp.5.2.2",
					FlavorMaster:         "gp.5.4.4",
					FlavorWorker:         "gp.5.4.8",
					SubnetPods:           "10.42.0.0/16",
					SubnetServices:       "10.43.0.0/16",
					LoadbalancerProvider: "ovn",
					DNSZoneName:          "",
					MasterCount:          3,
					WorkerCount:          4,
					WorkerCountWindows:   0,
					NetworkPlugin: NetworkPlugin{
						Calico: CalicoConfig{
							Enabled:                   true,
							CNIIface:                  "enp3s0",
							CalicoInterfaceAutodetect: "interface",
						},
						Cilium: CiliumConfig{
							Enabled:              false,
							OperatorEnabled:      true,
							KubeProxyReplacement: true,
						},
						KubeOVN: KubeOVNConfig{
							Enabled:           false,
							CiliumIntegration: true,
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
						WorkerNodeBFVTypeWindows: "local",
					},
				},
			},
			GitOps: GitOpsConfig{
				GitDir:    fmt.Sprintf("./testdata/local-git-repo-%s", name),
				GitURL:    "",
				GitSSHKey: "~/.ssh/id_ed25519-flux",
				GitSSHPub: "~/.ssh/id_ed25519-flux.pub",
				GitBranch: "main",
				Flux: GitOpsFlux{
					Interval: "15m",
					Prune:    true,
				},
			},
			ManagedService: map[string]ServiceCfg{
				"alert-proxy": {
					Enabled: true,
				},
			},
			Services: map[string]ServiceCfg{
				"calico":                {Enabled: true},
				"cert-manager":          {Enabled: true, Email: "mpk-support@rackspace.com", Region: "us-east-1", AWSAccessKey: "", AWSSecretAccessKey: ""},
				"etcd-backup":           {Enabled: true, S3Host: "https://swift.api.dfw3.rackspacecloud.com", S3Region: "DFW3", AWSAccessKey: "", AWSSecretAccessKey: ""},
				"external-snapshotter":  {Enabled: true},
				"fluxcd":                {Enabled: true},
				"gateway":               {Enabled: true},
				"gateway-api":           {Enabled: true},
				"headlamp":              {Enabled: true},
				"keycloak":              {Enabled: true},
				"kube-prometheus-stack": {Enabled: true},
				"kyverno":               {Enabled: true},
				"olm":                   {Enabled: true},
				"openstack-ccm":         {Enabled: true},
				"openstack-csi":         {Enabled: true},
				"postgres-operator":     {Enabled: true},
				"rbac-manager":          {Enabled: true},
				"sources":               {Enabled: true},
				"velero":                {Enabled: true},
				"weave-gitops":          {Enabled: true},
			},
		},
		OpenTofu: SimplifiedOpenTofu{
			Enabled: true,
			Path:    "opentofu",
			Backend: SimplifiedTofuBackend{
				Type: "local",
				Local: SimplifiedTofuLocal{
					Path: "terraform.tfstate",
				},
				S3: SimplifiedTofuS3{
					Bucket: "",
					Key:    "",
					Region: "",
				},
			},
		},
		Secrets: Secrets{
			SopsAgeKeyFile: "",
		},
		IAC: IAC{
			Main: map[string]any{
				"cluster_name":    name,
				"master_count":    3,
				"worker_count":    4,
				"subnet_pods":     "10.42.0.0/16",
				"subnet_services": "10.43.0.0/16",
			},
			Modules: map[string]any{
				"calico": map[string]any{
					"source": "",
				},
				"kubespray-cluster": map[string]any{
					"source": "",
				},
				"openstack-nova": map[string]any{
					"source": "",
				},
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

// Helper methods for backward compatibility

// ClusterName returns the cluster name from the simplified structure
func (c Config) ClusterName() string {
	return c.OpenCenter.Cluster.ClusterName
}

// GitOps returns the GitOps configuration from the simplified structure
func (c Config) GitOps() GitOpsConfig {
	return c.OpenCenter.GitOps
}

// ResolveConfigDir resolves the configuration directory based on the OPENCENTER_CONFIG_DIR
// environment variable. If the variable is not set, it falls back to the user's
// standard config directory (e.g., ~/.config/openCenter on Linux).
// The directory is created if it does not exist.
//
// Outputs:
//   - string: The absolute path to the configuration directory.
//   - error: An error if one occurred.
func ResolveConfigDir() (string, error) {
	var err error
	dir := os.Getenv("OPENCENTER_CONFIG_DIR")
	if dir == "" {
		// Determine OS-specific config directory
		switch runtime.GOOS {
		case "windows":
			base := os.Getenv("APPDATA")
			if base == "" {
				base = os.Getenv("LOCALAPPDATA")
			}
			if base == "" {
				base = os.Getenv("USERPROFILE")
			}
			dir = filepath.Join(base, "openCenter")
		default:
			home, herr := os.UserHomeDir()
			if herr != nil {
				err = herr
				return "", err
			}
			dir = filepath.Join(home, ".config", "openCenter")
		}
	}
	// Ensure absolute path
	if !filepath.IsAbs(dir) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return "", err
		}
	}
	// Create directory if not exists
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		err = mkErr
		return "", err
	}
	return dir, err
}

// ConfigPath returns the absolute path to a cluster's configuration file.
//
// Inputs:
//   - name: The name of the cluster (without the .yaml extension).
//
// Outputs:
//   - string: The absolute path to the configuration file.
//   - error: An error if one occurred.
func ConfigPath(name string) (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".yaml"), nil
}

// Load reads and unmarshals a YAML configuration file for the given cluster name.
// Default values are applied for any omitted fields.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - Config: The loaded configuration.
//   - error: An error if the file does not exist or cannot be parsed.
func Load(name string) (Config, error) {
	path, err := ConfigPath(name)
	if err != nil {
		return Config{}, err
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return Config{}, fmt.Errorf("failed to read %s: %w", path, readErr)
	}
	// Unmarshal YAML then overlay onto default config
	cfg := defaultConfig(name)
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		return Config{}, fmt.Errorf("failed to parse YAML: %w", unmarshalErr)
	}
	return cfg, nil
}

// GenerateCompleteConfig generates a complete configuration by merging schema defaults
// with the actual cluster configuration. The opencenter values take precedence over
// schema defaults.
//
// Inputs:
//   - name: The cluster name to load configuration for.
//
// Outputs:
//   - Config: The complete merged configuration.
//   - error: An error if the configuration cannot be generated.
func GenerateCompleteConfig(name string) (Config, error) {
	// Generate schema defaults as YAML
	defaultYAML, err := GenerateDefaultFromSchema(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to generate schema defaults: %w", err)
	}

	// Read the actual cluster configuration file directly as YAML
	path, err := ConfigPath(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get config path: %w", err)
	}
	actualYAML, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read cluster config: %w", err)
	}

	// Parse both as generic maps to preserve all structure
	var schemaDefaults map[string]any
	if err := yaml.Unmarshal(defaultYAML, &schemaDefaults); err != nil {
		return Config{}, fmt.Errorf("failed to parse schema defaults: %w", err)
	}

	var actualConfig map[string]any
	if err := yaml.Unmarshal(actualYAML, &actualConfig); err != nil {
		return Config{}, fmt.Errorf("failed to parse actual config: %w", err)
	}

	// Merge the configurations with actual config taking precedence
	mergedConfig := mergeYAMLMaps(schemaDefaults, actualConfig)

	// Marshal back to YAML then unmarshal into Config struct
	mergedYAML, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return Config{}, fmt.Errorf("failed to marshal merged config: %w", err)
	}

	var completeCfg Config
	if err := yaml.Unmarshal(mergedYAML, &completeCfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse merged config into struct: %w", err)
	}

	return completeCfg, nil
}

// mergeYAMLMaps recursively merges two YAML maps, with values from 'override' taking precedence
func mergeYAMLMaps(base, override map[string]any) map[string]any {
	result := make(map[string]any)

	// Start with all base values
	for k, v := range base {
		result[k] = v
	}

	// Override with values from override map
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// If both values are maps, merge them recursively
			if baseMap, baseIsMap := baseVal.(map[string]any); baseIsMap {
				if overrideMap, overrideIsMap := v.(map[string]any); overrideIsMap {
					result[k] = mergeYAMLMaps(baseMap, overrideMap)
					continue
				}
			}
		}
		// Otherwise, override value takes precedence
		result[k] = v
	}

	return result
}

// GenerateCompleteConfigYAML generates a complete configuration YAML by merging schema defaults
// with the actual cluster configuration, preserving all YAML structure.
//
// Inputs:
//   - name: The cluster name to load configuration for.
//
// Outputs:
//   - []byte: The complete merged configuration as YAML.
//   - error: An error if the configuration cannot be generated.
func GenerateCompleteConfigYAML(name string) ([]byte, error) {
	// Generate schema defaults as YAML
	defaultYAML, err := GenerateDefaultFromSchema(name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema defaults: %w", err)
	}

	// Read the actual cluster configuration file directly as YAML
	path, err := ConfigPath(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	actualYAML, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config: %w", err)
	}

	// Parse both as generic maps to preserve all structure
	var schemaDefaults map[string]any
	if err := yaml.Unmarshal(defaultYAML, &schemaDefaults); err != nil {
		return nil, fmt.Errorf("failed to parse schema defaults: %w", err)
	}

	var actualConfig map[string]any
	if err := yaml.Unmarshal(actualYAML, &actualConfig); err != nil {
		return nil, fmt.Errorf("failed to parse actual config: %w", err)
	}

	// Merge the configurations with actual config taking precedence
	mergedConfig := mergeYAMLMaps(schemaDefaults, actualConfig)

	// Marshal back to YAML
	mergedYAML, err := yaml.Marshal(mergedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged config: %w", err)
	}

	return mergedYAML, nil
}

// SaveDebugConfig saves a complete configuration to the GitOps directory as .openCenter.yaml
// for debugging purposes. This is only called when OPENCENTER_DEBUG environment variable exists.
//
// Inputs:
//   - clusterName: The cluster name to generate complete config for.
//   - gitDir: The GitOps directory where to save the debug config.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func SaveDebugConfig(clusterName, gitDir string) error {
	if gitDir == "" {
		return fmt.Errorf("git directory is empty")
	}

	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return fmt.Errorf("failed to create git directory %s: %w", gitDir, err)
	}

	debugPath := filepath.Join(gitDir, ".openCenter.yaml")

	// Generate the complete config YAML
	data, err := GenerateCompleteConfigYAML(clusterName)
	if err != nil {
		return fmt.Errorf("failed to generate complete config: %w", err)
	}

	// Write the debug config file with 0600 permissions
	if err := os.WriteFile(debugPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write debug config to %s: %w", debugPath, err)
	}

	return nil
}

// Save writes the configuration to a YAML file. The file is saved with 0600
// permissions to protect sensitive data.
//
// Inputs:
//   - cfg: The configuration to save.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func Save(cfg Config) error {
	if cfg.ClusterName() == "" {
		return errors.New("cluster_name must not be empty")
	}
	path, err := ConfigPath(cfg.ClusterName())
	if err != nil {
		return err
	}
	// Marshal YAML with indentation
	data, marshalErr := yaml.Marshal(&cfg)
	if marshalErr != nil {
		return marshalErr
	}
	// Write with 0600 permissions
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		return writeErr
	}
	return nil
}

// List returns a sorted list of cluster names from the configuration directory.
// It ignores any files that do not have a .yaml extension.
//
// Outputs:
//   - []string: A list of cluster names.
//   - error: An error if the directory cannot be read.
func List() ([]string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return nil, err
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil, readErr
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") {
			names = append(names, strings.TrimSuffix(name, ".yaml"))
		}
	}
	// Sort lexically
	if len(names) > 1 {
		sortStrings(names)
	}
	return names, nil
}

// simple string sorter to avoid pulling in a larger dependency.
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// activeClusterPath returns the absolute path to the file tracking
// the active cluster. This file stores the cluster name as plain
// text.
func activeClusterPath() (string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".active"), nil
}

// SetActive writes the given cluster name into the active marker file.
// If the name is empty, the marker file is removed.
//
// Inputs:
//   - name: The name of the cluster to set as active.
//
// Outputs:
//   - error: An error if the file cannot be written.
func SetActive(name string) error {
	path, err := activeClusterPath()
	if err != nil {
		return err
	}
	if name == "" {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(name), 0o600)
}

// GetActive reads the active cluster name from the marker file.
// If the file does not exist or is empty, it returns an empty string.
//
// Outputs:
//   - string: The active cluster name.
//   - error: An error if the file cannot be read.
func GetActive() (string, error) {
	path, err := activeClusterPath()
	if err != nil {
		return "", err
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, fs.ErrNotExist) {
			return "", nil
		}
		return "", readErr
	}
	return strings.TrimSpace(string(data)), nil
}

// Validate performs a set of invariant checks on the configuration.
//
// Inputs:
//   - cfg: The configuration to validate.
//
// Outputs:
//   - []string: A list of error messages describing any validation failures.
//     If the list is empty, the configuration is valid.
func Validate(cfg Config) []string {
	var errs []string
	// Required cluster name and opencenter.gitops.git_dir
	if cfg.ClusterName() == "" {
		errs = append(errs, "opencenter.cluster.cluster_name must be set")
	}
	if cfg.GitOps().GitDir == "" {
		errs = append(errs, "opencenter.gitops.git_dir must be set")
	}
	// OpenTofu validation
	if cfg.OpenTofu.Enabled {
		if cfg.OpenTofu.Path == "" {
			errs = append(errs, "opentofu.path must be set when opentofu.enabled=true")
		}
		bt := strings.ToLower(strings.TrimSpace(cfg.OpenTofu.Backend.Type))
		if bt == "" {
			bt = "local"
		}
		switch bt {
		case "local":
			if cfg.OpenTofu.Backend.Local.Path == "" {
				errs = append(errs, "opentofu.backend.local.path must be set for local backend")
			}
		case "s3":
			s3 := cfg.OpenTofu.Backend.S3
			if s3.Bucket == "" || s3.Key == "" || s3.Region == "" {
				errs = append(errs, "opentofu.backend.s3 requires bucket, key, and region")
			}
			// When using S3 backend, AWS credentials must be provided via opencenter
			if strings.TrimSpace(cfg.OpenCenter.Cluster.AWSAccessKey) == "" || strings.TrimSpace(cfg.OpenCenter.Cluster.AWSSecretAccessKey) == "" {
				errs = append(errs, "opencenter.cluster.aws_access_key and opencenter.cluster.aws_secret_access_key must be set when opentofu.backend.type=s3")
			}
		default:
			errs = append(errs, "opentofu.backend.type must be 'local' or 's3'")
		}
	}
	// iac validation is intentionally minimal for variables.tf-aligned shape

	// Legacy VRRP validation for backward-compat features:
	// If Octavia is not used, a VRRP IP must be provided. Prefer cloud.openstack.*
	// but also honor legacy iac.networking.* when present in the YAML file.
	// This keeps older workflows passing until fully migrated.
	// Note: UseOctavia and VRRPIP are not in the simplified schema, commenting out for now
	// useOctavia := cfg.OpenCenter.Infrastructure.Cloud.OpenStack.UseOctavia
	// vrrpIP := strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.VRRPIP)
	useOctavia := false
	vrrpIP := ""
	vrrpEnabled := false
	// Fallback to reading networking from the YAML on disk if not set
	if vrrpIP == "" || !useOctavia {
		// Attempt to locate YAML file and parse minimal fields
		// Ignore errors silently; this is a best-effort compatibility shim.
		if dir, err := ResolveConfigDir(); err == nil {
			path := filepath.Join(dir, cfg.ClusterName()+".yaml")
			if data, rerr := os.ReadFile(path); rerr == nil {
				var raw map[string]any
				if yerr := yaml.Unmarshal(data, &raw); yerr == nil {
					// Check top-level networking section first
					if netw, ok := raw["networking"].(map[string]any); ok {
						if uo, ok := netw["use_octavia"].(bool); ok {
							useOctavia = uo
						}
						if ve, ok := netw["vrrp_enabled"].(bool); ok {
							vrrpEnabled = ve
						}
						if v, ok := netw["vrrp_ip"].(string); ok && strings.TrimSpace(v) != "" {
							vrrpIP = strings.TrimSpace(v)
						}
					}
					// Fallback to iac.networking for backward compatibility
					if iac, ok := raw["iac"].(map[string]any); ok {
						if netw, ok := iac["networking"].(map[string]any); ok {
							if uo, ok := netw["use_octavia"].(bool); ok && !useOctavia {
								useOctavia = uo
							}
							if ve, ok := netw["vrrp_enabled"].(bool); ok && !vrrpEnabled {
								vrrpEnabled = ve
							}
							if v, ok := netw["vrrp_ip"].(string); ok && strings.TrimSpace(v) != "" && vrrpIP == "" {
								vrrpIP = strings.TrimSpace(v)
							}
						}
					}
				}
			}
		}
	}
	if vrrpEnabled && !useOctavia && vrrpIP == "" {
		errs = append(errs, "vrrp_ip must be set when use_octavia is false")
	}
	return errs
}

// ToJSON marshals the configuration to JSON. This is used for generating
// the JSON schema and for other tools that consume JSON.
//
// Outputs:
//   - []byte: The JSON-encoded configuration.
//   - error: An error if the configuration cannot be marshaled.
func (c Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}