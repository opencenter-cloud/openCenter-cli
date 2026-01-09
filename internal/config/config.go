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
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
)

// Config represents the simplified root configuration for a cluster based on the new schema.
// The structure matches the testdata/schema.yaml format with opencenter, opentofu, cloud, and secrets sections.
// IAC field is included for backward compatibility with existing templates.
type Config struct {
	OpenCenter SimplifiedOpenCenter `yaml:"opencenter" json:"opencenter"`
	OpenTofu   SimplifiedOpenTofu   `yaml:"opentofu" json:"opentofu"`
	Secrets    Secrets              `yaml:"secrets" json:"secrets"`
	Networking Networking           `yaml:"networking,omitempty" json:"networking,omitempty"`
	Security   Security             `yaml:"security,omitempty" json:"security,omitempty"`
	Deployment Deployment           `yaml:"deployment,omitempty" json:"deployment,omitempty"`
	Overrides  map[string]any       `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	IAC        IAC                  `yaml:"-" json:"-"` // Hidden from YAML output, for template compatibility
}

// Cluster Stages
const (
	StageInit      = "init"
	StagePreflight = "preflight"
	StageSetup     = "setup"
	StageBootstrap = "bootstrap"
	StageValidate  = "validate"
	StageDestroy   = "destroy"
	StageRender    = "render"
	StagePlan      = "plan"
	StageApply     = "apply"
)

// Cluster Statuses
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// Kubernetes groups settings for the Kubernetes cluster.
// It nests further objects for counts, images, flavors, and networking.
// Default values are applied at load time.
// IAC groups settings for infrastructure-as-code driven cluster provisioning.
// It retains the detailed node/layout fields and adds engine/stack selectors.
type IAC struct {
	// Main contains the values for the Terraform locals (rendered into main.tf)
	Main map[string]any `yaml:"main,omitempty" json:"main,omitempty"`
	// Modules contains per-module attribute maps (rendered into main.tf)
	Modules map[string]any `yaml:"modules,omitempty" json:"modules,omitempty"`
}

// defaultConfig returns a Config pre-populated with the default
// values based on the simplified schema. This function can be used to
// initialise new cluster configurations.
func defaultConfig(name string) Config {
	// Check if running in test mode
	isTestMode := os.Getenv("OPENCENTER_TEST_MODE") == "true"

	authURL := ""
	region := ""
	tenantName := ""
	barbicanAuthURL := ""

	// Dummy secrets for test mode
	awsAccessKey := ""
	awsSecretKey := ""
	certManagerAccessKey := ""
	certManagerSecretKey := ""
	lokiSwiftPassword := ""
	keycloakClientSecret := ""
	keycloakAdminPassword := ""
	headlampClientSecret := ""
	weaveGitOpsPassword := ""
	weaveGitOpsHash := ""
	grafanaAdminPassword := ""
	alertProxyDeviceID := ""
	alertProxyToken := ""
	alertProxyAccount := ""

	if isTestMode {
		authURL = "https://identity.example.com/v3"
		region = "RegionOne"
		tenantName = "admin"
		barbicanAuthURL = "https://identity.example.com/v3"

		awsAccessKey = "test-aws-access-key"
		awsSecretKey = "test-aws-secret-key"
		certManagerAccessKey = "test-access-key"
		certManagerSecretKey = "test-secret-key"
		lokiSwiftPassword = "test-password"
		keycloakClientSecret = "test-client-secret"
		keycloakAdminPassword = "test-admin-password"
		headlampClientSecret = "test-client-secret"
		weaveGitOpsPassword = "test-password"
		weaveGitOpsHash = "test-hash"
		grafanaAdminPassword = "test-password"
		alertProxyDeviceID = "test-device-id"
		alertProxyToken = "test-token"
		alertProxyAccount = "test-account"
	}

	cfg := Config{
		OpenCenter: SimplifiedOpenCenter{
			Meta: ClusterMeta{
				Name:         name,
				Env:          "",
				Region:       "",
				Status:       "",
				Organization: "opencenter",
			},
			Secrets: OpenCenterSecrets{
				Backend: "barbican",
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
				Provider:            "openstack",
				SSHUser:             "ubuntu",
				OSVersion:           "24",
				ServerGroupAffinity: []string{"anti-affinity"},
				NodeNaming: NodeNaming{
					Worker:        "wn",
					Master:        "cp",
					WorkerWindows: "win",
				},
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
						FloatingNetworkId:           "",
						SubnetId:                    "",
						AvailabilityZone:            "az1",
						ProjectDomainName:           "rackspace_cloud_domain",
						UserDomainName:              "rackspace_cloud_domain",
						FloatingIPPool:              "PUBLICNET",
						RouterExternalNetworkID:     "723f8fa2-dbf7-4cec-8d5f-017e62c12f79",
						CA:                          "",
						ImageID:                     "799dcf97-3656-4361-8187-13ab1b295e33",
						ImageIDWindows:              "a2083759-f341-445b-b717-dafb5e31fa6b",
					},
				},
			},
			Cluster: ClusterConfig{
				ClusterName:        name,
				AWSAccessKey:       "",
				AWSSecretAccessKey: "",
				K8sAPIPortACL:      []string{"0.0.0.0/0"},
				SSHAuthorizedKeys:  []string{""},
				BaseDomain:         "k8s.opencenter.cloud",
				ClusterFQDN:        fmt.Sprintf("%s.sjc3.k8s.opencenter.cloud", name),
				AdminEmail:         "",
				Kubernetes: KubernetesConfig{
					Version:              "1.31.4",
					KubesprayVersion:     "v2.28.1",
					APIPort:              443,
					KubeVIPEnabled:       true,
					FlavorBastion:        "gp.0.2.2",
					FlavorMaster:         "gp.0.4.8",
					FlavorWorker:         "gp.0.4.16",
					FlavorWorkerWindows:  "gp.5.4.16",
					SubnetPods:           "10.42.0.0/16",
					SubnetServices:       "10.43.0.0/16",
					LoadbalancerProvider: "ovn",
					DNSZoneName:          fmt.Sprintf("%s.sjc3.k8s.opencenter.cloud", name),
					MasterCount:          3,
					WorkerCount:          2,
					WorkerCountWindows:   0,
					NetworkPlugin: NetworkPlugin{
						Calico: CalicoConfig{
							Enabled:                   true,
							CNIIface:                  "enp3s0",
							CalicoInterfaceAutodetect: "interface",
							AutodetectCIDR:            "",
							EncapsulationType:         "VXLAN",
							NATOutgoing:               true,
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
				GitOpsBaseRepo:    "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
				GitOpsBaseRelease: "v0.1.0",
				GitOpsBranch:      "main",
				Flux: GitOpsFlux{
					Interval: "15m",
					Prune:    true,
				},
			},
			Storage: StorageConfig{
				DefaultStorageClass:         "csi-cinder-sc-delete",
				WorkerVolumeSize:            100,
				WorkerVolumeDestinationType: "volume",
				WorkerVolumeSourceType:      "image",
				WorkerVolumeType:            "HA-Performance",
				AdditionalBlockDevices:      []map[string]any{},
			},
			Talos: nil, // Talos is disabled by default, can be enabled by user
			ManagedService: ServiceMap{
				"alert-proxy": &services.AlertProxyConfig{
					BaseConfig: services.BaseConfig{
						Enabled:             true,
						ImageRepository:     "ghcr.io/rackerlabs/alert-proxy",
						ImageTag:            "latest",
						GitOpsSourceRepo:    "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git",
						GitOpsSourceRelease: "v0.1.0",
						GitOpsSourceBranch:  "main",
					},
					AlertManagerBaseUrl: "",
					HTTPRouteFQDN:       fmt.Sprintf("https://alerts.%s.sjc3.k8s.opencenter.cloud", name),
				},
			},
			Services: ServiceMap{
				"calico": &services.CalicoConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true,
					},
					KubeAPIServer: fmt.Sprintf("https://api.%s.sjc3.k8s.opencenter.cloud:6443", name),
				},
				"cert-manager": &services.CertManagerConfig{
					BaseConfig: services.BaseConfig{
						Enabled: false,
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
				"headlamp": &services.HeadlampConfig{
					BaseConfig: services.BaseConfig{
						Enabled:  true,
						Hostname: fmt.Sprintf("dashboard.%s.sjc3.k8s.opencenter.cloud", name),
					},
					OIDCIssuerURL: fmt.Sprintf("https://auth.%s.sjc3.k8s.opencenter.cloud/realms/opencenter", name),
					OIDCClientID:  "kubernetes",
				},
				"keycloak": &services.KeycloakConfig{
					BaseConfig: services.BaseConfig{
						Enabled:  false,
						Hostname: fmt.Sprintf("auth.%s.sjc3.k8s.opencenter.cloud", name),
					},
					Realm:       "opencenter",
					ClientID:    "kubernetes",
					FrontendURL: fmt.Sprintf("https://auth.%s.sjc3.k8s.opencenter.cloud", name),
				},
				"kube-prometheus-stack": &services.PrometheusStackConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true,
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
						Enabled: false,
					},
					VolumeSize:      20,
					StorageClass:    "csi-cinder-sc-delete",
					BucketName:      fmt.Sprintf("%s-loki", name),
					SwiftAuthURL:    "https://keystone.api.sjc3.rackspacecloud.com/v3/",
					SwiftRegion:     "SJC3",
					SwiftDomainName: "Default",
				},
				"olm":               &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"openstack-ccm":     &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"openstack-csi":     &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"postgres-operator": &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"rbac-manager":      &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"sources":           &services.DefaultServiceConfig{BaseConfig: services.BaseConfig{Enabled: true}},
				"velero": &services.VeleroConfig{
					BaseConfig: services.BaseConfig{
						Enabled: true,
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
					// Namespace is in BaseConfig? No, BaseConfig has Namespace.
				},
				"weave-gitops": &services.WeaveGitOpsConfig{
					BaseConfig: services.BaseConfig{
						Enabled:  true,
						Hostname: fmt.Sprintf("gitops.%s.sjc3.k8s.opencenter.cloud", name),
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
					Path: fmt.Sprintf("./testdata/test-git-repo-%s/terraform.tfstate", name),
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
			SSHKey: SSHKey{
				Private: fmt.Sprintf("./testdata/test-git-repo-%s/%s/secrets/ssh/%s", name, name, name),
				Public:  fmt.Sprintf("./testdata/test-git-repo-%s/%s/secrets/ssh/%s.pub", name, name, name),
				Cypher:  "ed25519",
			},
			// Global AWS credentials (fallback for services)
			AWS: AWSSecrets{
				AccessKey:       awsAccessKey,
				SecretAccessKey: awsSecretKey,
				Region:          "us-east-1",
			},
			// Service-specific secrets - must be provided by user
			CertManager: CertManagerSecrets{
				AWSAccessKey:       certManagerAccessKey,
				AWSSecretAccessKey: certManagerSecretKey,
			},
			Loki: LokiSecrets{
				SwiftPassword: lokiSwiftPassword,
			},
			Keycloak: KeycloakSecrets{
				ClientSecret:  keycloakClientSecret,
				AdminPassword: keycloakAdminPassword,
			},
			Headlamp: HeadlampSecrets{
				OIDCClientSecret: headlampClientSecret,
			},
			WeaveGitOps: WeaveGitOpsSecrets{
				Password:     weaveGitOpsPassword,
				PasswordHash: weaveGitOpsHash,
			},
			Grafana: GrafanaSecrets{
				AdminPassword: grafanaAdminPassword,
			},
			Tempo: TempoSecrets{
				AccessKey: "",
				SecretKey: "",
			},
			AlertProxy: AlertProxySecrets{
				CoreDeviceId:        alertProxyDeviceID,
				AccountServiceToken: alertProxyToken,
				CoreAccountNumber:   alertProxyAccount,
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
		Networking: Networking{
			SubnetNodes:          "10.2.128.0/22",
			AllocationPoolStart:  "",
			AllocationPoolEnd:    "10.2.131.254",
			VRRPEnabled:          true,
			VRRPIP:               "",
			SubnetServices:       "10.43.0.0/16",
			SubnetPods:           "10.42.0.0/16",
			UseOctavia:           false,
			LoadbalancerProvider: "amphora",
			UseDesignate:         false,
			DNSZoneName:          fmt.Sprintf("gdo.prod.sjc3.k8s.opencenter.cloud"),
			DNSNameservers:       []string{"1.1.1.1", "8.8.8.8"},
			NTPServers:           []string{"time.dfw3.rackspace.com", "time2.dfw3.rackspace.com"},
			VLAN: VLAN{
				ID:       "",
				MTU:      0,
				Provider: "physnet1",
			},
		},
		Security: Security{
			CACertificates:        "",
			K8sHardening:          true,
			OSHardening:           true,
			KubeletRotateCerts:    true,
			PodSecurityExemptions: []string{"trivy-temp"},
		},
		Deployment: Deployment{
			AutoDeploy: true,
		},
	}

	// Populate IAC field from defaults
	if err := populateIAC(&cfg); err != nil {
		// If IAC population fails, return config with minimal IAC structure
		cfg.IAC = IAC{
			Main: map[string]any{
				"cluster_name":    name,
				"master_count":    3,
				"worker_count":    2,
				"subnet_nodes":    "10.2.184.0/22",
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
		}
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
	return &TalosConfig{
		Enabled:        true,
		Version:        "v1.8.0",
		ImageURL:       "https://github.com/siderolabs/talos/releases/download/v1.8.0/openstack-amd64.raw.xz",
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

// ParseClusterIdentifier parses a cluster identifier which can be in one of two formats:
// 1. "cluster" - just the cluster name (uses default "opencenter" organization)
// 2. "organization/cluster" - organization and cluster name
//
// Inputs:
//   - identifier: The cluster identifier to parse.
//
// Outputs:
//   - organization: The organization name (or "opencenter" if not specified).
//   - clusterName: The cluster name.
//   - error: An error if the identifier is invalid.
func ParseClusterIdentifier(identifier string) (organization string, clusterName string, err error) {
	if identifier == "" {
		return "", "", errors.New("cluster identifier cannot be empty")
	}

	// Check for organization/cluster format
	if strings.Contains(identifier, "/") {
		parts := strings.SplitN(identifier, "/", 2)
		if len(parts) != 2 {
			return "", "", errors.New("invalid cluster identifier format: expected 'organization/cluster'")
		}
		organization = parts[0]
		clusterName = parts[1]

		// Validate both parts
		if err := ValidateClusterName(organization); err != nil {
			return "", "", fmt.Errorf("invalid organization name: %w", err)
		}
		if err := ValidateClusterName(clusterName); err != nil {
			return "", "", fmt.Errorf("invalid cluster name: %w", err)
		}

		return organization, clusterName, nil
	}

	// Just cluster name, use default organization
	if err := ValidateClusterName(identifier); err != nil {
		return "", "", err
	}

	return "opencenter", identifier, nil
}

// ValidateClusterName validates and sanitizes a cluster name to ensure it's safe for use as a directory name.
// It checks for valid characters and prevents directory traversal attacks.
// Note: This validates individual cluster or organization names, not the full "org/cluster" format.
// Use ParseClusterIdentifier for validating the full identifier format.
//
// Inputs:
//   - name: The cluster name to validate.
//
// Outputs:
//   - error: An error if the name is invalid.
func ValidateClusterName(name string) error {
	if name == "" {
		return errors.New("cluster name cannot be empty for directory creation")
	}

	// Check for path separators and special characters that could cause issues
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errors.New("cluster name cannot contain path separators (/ or \\) for directory structure")
	}

	// Check for relative path components
	if name == "." || name == ".." || strings.HasPrefix(name, ".") && (strings.Contains(name, "/") || strings.Contains(name, "\\")) {
		return errors.New("cluster name cannot be a relative path component for security reasons")
	}

	// Allow alphanumeric characters, hyphens, underscores, and dots (but not starting with dot)
	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	if !validName.MatchString(name) {
		return errors.New("cluster name must start with alphanumeric character and contain only alphanumeric characters, dots, hyphens, and underscores for directory naming")
	}

	// Prevent excessively long names that could cause filesystem issues
	if len(name) > 255 {
		return errors.New("cluster name cannot exceed 255 characters for filesystem compatibility")
	}

	return nil
}

// ClusterDirectoryPath returns the absolute path to a cluster's directory within the clusters subdirectory.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - string: The absolute path to the cluster directory.
//   - error: An error if one occurred.
func ClusterDirectoryPath(name string) (string, error) {
	if err := ValidateClusterName(name); err != nil {
		return "", fmt.Errorf("invalid cluster name for directory creation: %w", err)
	}

	dir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory for cluster '%s': %w", name, err)
	}

	return filepath.Join(dir, "clusters", name), nil
}

// ClusterSecretsPath returns the absolute path to a cluster's secrets directory for SOPS key storage.
//
// Inputs:
//   - name: The name of the cluster.
//
// Outputs:
//   - string: The absolute path to the cluster's secrets directory.
//   - error: An error if one occurred.
func ClusterSecretsPath(name string) (string, error) {
	clusterDir, err := ClusterDirectoryPath(name)
	if err != nil {
		return "", fmt.Errorf("failed to get cluster directory for secrets path: %w", err)
	}

	return filepath.Join(clusterDir, "secrets", "age", "keys"), nil
}

// ConfigPath returns the absolute path to a cluster's configuration file.
// It implements a fallback strategy to support both organization-based and legacy structures.
// The name parameter can be in "cluster" or "organization/cluster" format.
// If no organization is specified, it searches all organizations for the cluster.
//
// Inputs:
//   - name: The name of the cluster (can be "cluster" or "organization/cluster").
//
// Outputs:
//   - string: The absolute path to the configuration file.
//   - error: An error if one occurred.
func ConfigPath(name string) (string, error) {
	// Parse the cluster identifier to extract organization and cluster name
	organization, clusterName, err := ParseClusterIdentifier(name)
	if err != nil {
		return "", fmt.Errorf("invalid cluster identifier: %w", err)
	}

	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	// Load CLI configuration to get the configured clustersDir
	cliConfigManager, err := NewConfigManager("")
	var clustersDir string
	if err == nil {
		clustersDir = cliConfigManager.GetConfig().Paths.ClustersDir
		if clustersDir == "" {
			clustersDir = filepath.Join(configDir, "clusters")
		}
		clustersDir = ExpandPath(clustersDir)
	} else {
		clustersDir = filepath.Join(configDir, "clusters")
	}

	// Priority 1: If organization was explicitly specified, check organization-based paths
	if strings.Contains(name, "/") {
		if cliConfigManager != nil {
			pathResolver := NewPathResolver(cliConfigManager)
			paths := pathResolver.ResolveClusterPaths(clusterName, organization)

			// Check for config file at organization level (primary location)
			orgConfigPath := filepath.Join(paths.OrganizationDir, "."+clusterName+"-config.yaml")
			if _, statErr := os.Stat(orgConfigPath); statErr == nil {
				return orgConfigPath, nil
			}

			// Check for config file at cluster directory level (alternative location)
			clusterConfigPath := filepath.Join(paths.ClusterDir, "."+clusterName+"-config.yaml")
			if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
				return clusterConfigPath, nil
			}
		}
		// If explicitly specified org/cluster not found, return error
		return "", fmt.Errorf("cluster configuration file not found for cluster %s", name)
	}

	// Priority 2: No organization specified - search organization-based paths first
	if entries, readErr := os.ReadDir(clustersDir); readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				orgName := entry.Name()

				// Check for config file at organization level (primary location)
				orgConfigPath := filepath.Join(clustersDir, orgName, "."+clusterName+"-config.yaml")
				if _, statErr := os.Stat(orgConfigPath); statErr == nil {
					return orgConfigPath, nil
				}

				// Check for config file at cluster directory level (alternative location)
				clusterConfigPath := filepath.Join(clustersDir, orgName, "infrastructure", "clusters", clusterName, "."+clusterName+"-config.yaml")
				if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
					return clusterConfigPath, nil
				}
			}
		}
	}

	// Priority 3: Check for flat config file (backward compatibility)
	flatConfigPath := filepath.Join(configDir, clusterName+".yaml")
	if _, statErr := os.Stat(flatConfigPath); statErr == nil {
		return flatConfigPath, nil
	}

	// Priority 4: Fall back to legacy directory structure (backward compatibility)
	clusterDir, err := ClusterDirectoryPath(clusterName)
	if err == nil {
		legacyConfigPath := filepath.Join(clusterDir, "."+clusterName+"-config.yaml")
		if _, statErr := os.Stat(legacyConfigPath); statErr == nil {
			return legacyConfigPath, nil
		}
	}

	// Config file not found anywhere
	return "", fmt.Errorf("cluster configuration file not found for cluster %s", name)
}

// Load reads and unmarshals a YAML configuration file for the given cluster name.
// Default values are applied for any omitted fields.
// It supports both organization-based and legacy directory structures.
// The name parameter can be in "cluster" or "organization/cluster" format.
//
// Inputs:
//   - name: The name of the cluster (can be "cluster" or "organization/cluster").
//
// Outputs:
//   - Config: The loaded configuration.
//   - error: An error if the file does not exist or cannot be parsed.
func Load(name string) (Config, error) {
	// Parse the cluster identifier - ConfigPath will handle validation
	_, clusterName, err := ParseClusterIdentifier(name)
	if err != nil {
		return Config{}, fmt.Errorf("invalid cluster identifier: %w", err)
	}

	path, err := ConfigPath(name)
	if err != nil {
		return Config{}, fmt.Errorf("failed to resolve configuration path for cluster '%s': %w", name, err)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return Config{}, fmt.Errorf("failed to read cluster configuration file '%s': %w", path, readErr)
	}

	// Expand environment variables in the raw YAML data
	// This allows users to use ${VAR} or $VAR in their config file to reference secrets
	// stored in environment variables, avoiding plaintext secrets in the file.
	expandedData := []byte(os.ExpandEnv(string(data)))

	// Unmarshal YAML then overlay onto default config (use actual cluster name, not full identifier)
	cfg := defaultConfig(clusterName)
	if unmarshalErr := yaml.Unmarshal(expandedData, &cfg); unmarshalErr != nil {
		return Config{}, fmt.Errorf("failed to parse YAML configuration from '%s': %w", path, unmarshalErr)
	}

	// Apply organization-based defaults if not explicitly set
	applyOrganizationDefaults(&cfg)

	// Populate IAC field from defaults and user configuration
	if err := populateIAC(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to populate IAC configuration: %w", err)
	}

	return cfg, nil
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

// populateIAC populates the IAC field from default YAML data and user configuration.
// It merges the default IAC structure with values from the user's configuration.
func populateIAC(cfg *Config) error {
	// Parse the default IAC YAML structure
	var defaultIAC struct {
		Locals  map[string]any `yaml:"locals"`
		Modules map[string]any `yaml:"modules"`
	}

	if err := yaml.Unmarshal([]byte(defaultIACYAML), &defaultIAC); err != nil {
		return fmt.Errorf("failed to parse default IAC YAML: %w", err)
	}

	// Initialize IAC field
	cfg.IAC = IAC{
		Main:    make(map[string]any),
		Modules: make(map[string]any),
	}

	// Copy default locals to IAC.Main
	for k, v := range defaultIAC.Locals {
		cfg.IAC.Main[k] = v
	}

	// Copy default modules to IAC.Modules
	for k, v := range defaultIAC.Modules {
		cfg.IAC.Modules[k] = v
	}

	// Override with user configuration values
	if err := mergeUserConfigIntoIAC(cfg); err != nil {
		return fmt.Errorf("failed to merge user config into IAC: %w", err)
	}

	return nil
}

// mergeUserConfigIntoIAC merges user configuration values into the IAC structure.
func mergeUserConfigIntoIAC(cfg *Config) error {
	// Map user configuration to IAC locals
	if cfg.OpenCenter.Cluster.ClusterName != "" {
		cfg.IAC.Main["cluster_name"] = cfg.OpenCenter.Cluster.ClusterName
	}

	// Map OpenStack configuration
	if cfg.OpenCenter.Infrastructure.Provider == "openstack" {
		os := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
		if os.AuthURL != "" {
			cfg.IAC.Main["openstack_auth_url"] = os.AuthURL
		}
		if os.Region != "" {
			cfg.IAC.Main["openstack_region"] = os.Region
		}
		if os.TenantName != "" {
			cfg.IAC.Main["openstack_tenant_name"] = os.TenantName
		}
		if os.Domain != "" {
			cfg.IAC.Main["openstack_project_domain_name"] = os.Domain
			cfg.IAC.Main["openstack_user_domain_name"] = os.Domain
		}
		// Map additional OpenStack fields
		if os.ProjectDomainName != "" {
			cfg.IAC.Main["openstack_project_domain_name"] = os.ProjectDomainName
		}
		if os.UserDomainName != "" {
			cfg.IAC.Main["openstack_user_domain_name"] = os.UserDomainName
		}
		if os.AvailabilityZone != "" {
			cfg.IAC.Main["availability_zone"] = os.AvailabilityZone
		}
		if os.FloatingIPPool != "" {
			cfg.IAC.Main["floatingip_pool"] = os.FloatingIPPool
		}
		if os.RouterExternalNetworkID != "" {
			cfg.IAC.Main["router_external_network_id"] = os.RouterExternalNetworkID
		}
		if os.NetworkID != "" {
			cfg.IAC.Main["network_id"] = os.NetworkID
		}
		if os.ImageID != "" {
			cfg.IAC.Main["image_id"] = os.ImageID
		}
		if os.ImageIDWindows != "" {
			cfg.IAC.Main["image_id_windows"] = os.ImageIDWindows
		}
		if os.CA != "" {
			cfg.IAC.Main["openstack_ca"] = os.CA
		}
		cfg.IAC.Main["openstack_insecure"] = os.Insecure
		if os.ApplicationCredentialID != "" {
			cfg.IAC.Main["openstack_user_name"] = os.ApplicationCredentialID
		}
		if os.ApplicationCredentialSecret != "" {
			cfg.IAC.Main["openstack_user_password"] = os.ApplicationCredentialSecret
		}
		if os.FloatingNetworkId != "" {
			cfg.IAC.Main["router_external_network_id"] = os.FloatingNetworkId
		}
	}

	// Map Networking configuration
	if cfg.Networking.SubnetNodes != "" {
		cfg.IAC.Main["subnet_nodes"] = cfg.Networking.SubnetNodes
	}
	if cfg.Networking.AllocationPoolEnd != "" {
		cfg.IAC.Main["allocation_pool_end"] = cfg.Networking.AllocationPoolEnd
	}
	cfg.IAC.Main["vrrp_enabled"] = cfg.Networking.VRRPEnabled
	cfg.IAC.Main["use_octavia"] = cfg.Networking.UseOctavia
	cfg.IAC.Main["use_designate"] = cfg.Networking.UseDesignate
	if len(cfg.Networking.DNSNameservers) > 0 {
		cfg.IAC.Main["dns_nameservers"] = cfg.Networking.DNSNameservers
	}
	if len(cfg.Networking.NTPServers) > 0 {
		cfg.IAC.Main["ntp_servers"] = cfg.Networking.NTPServers
	}
	if cfg.Networking.VLAN.ID != "" {
		cfg.IAC.Main["vlan_id"] = cfg.Networking.VLAN.ID
	}
	if cfg.Networking.VLAN.MTU > 0 {
		cfg.IAC.Main["mtu"] = cfg.Networking.VLAN.MTU
	}
	if cfg.Networking.VLAN.Provider != "" {
		cfg.IAC.Main["network_provider"] = cfg.Networking.VLAN.Provider
	}

	// Map Infrastructure configuration
	if cfg.OpenCenter.Infrastructure.SSHUser != "" {
		cfg.IAC.Main["ssh_user"] = cfg.OpenCenter.Infrastructure.SSHUser
	}
	if cfg.OpenCenter.Infrastructure.OSVersion != "" {
		cfg.IAC.Main["ub_version"] = cfg.OpenCenter.Infrastructure.OSVersion
	}
	if cfg.OpenCenter.Infrastructure.NodeNaming.Worker != "" {
		cfg.IAC.Main["node_worker"] = cfg.OpenCenter.Infrastructure.NodeNaming.Worker
	}
	if cfg.OpenCenter.Infrastructure.NodeNaming.Master != "" {
		cfg.IAC.Main["node_master"] = cfg.OpenCenter.Infrastructure.NodeNaming.Master
	}
	if cfg.OpenCenter.Infrastructure.NodeNaming.WorkerWindows != "" {
		cfg.IAC.Main["node_worker_windows"] = cfg.OpenCenter.Infrastructure.NodeNaming.WorkerWindows
	}

	// Map Security configuration
	if cfg.Security.CACertificates != "" {
		cfg.IAC.Main["ca_certificates"] = cfg.Security.CACertificates
	}
	cfg.IAC.Main["k8s_hardening_enabled"] = cfg.Security.K8sHardening
	cfg.IAC.Main["os_hardening_enabled"] = cfg.Security.OSHardening
	cfg.IAC.Main["kubelet_rotate_server_certificates"] = cfg.Security.KubeletRotateCerts
	if len(cfg.Security.PodSecurityExemptions) > 0 {
		cfg.IAC.Main["kube_pod_security_exemptions_namespaces"] = cfg.Security.PodSecurityExemptions
	}

	// Map Storage configuration
	if cfg.OpenCenter.Storage.WorkerVolumeSize > 0 {
		cfg.IAC.Main["worker_node_bfv_volume_size"] = cfg.OpenCenter.Storage.WorkerVolumeSize
	}
	if cfg.OpenCenter.Storage.WorkerVolumeDestinationType != "" {
		cfg.IAC.Main["worker_node_bfv_destination_type"] = cfg.OpenCenter.Storage.WorkerVolumeDestinationType
	}
	if cfg.OpenCenter.Storage.WorkerVolumeSourceType != "" {
		cfg.IAC.Main["worker_node_bfv_source_type"] = cfg.OpenCenter.Storage.WorkerVolumeSourceType
	}
	if cfg.OpenCenter.Storage.WorkerVolumeType != "" {
		cfg.IAC.Main["worker_node_bfv_volume_type"] = cfg.OpenCenter.Storage.WorkerVolumeType
	}
	if len(cfg.OpenCenter.Storage.AdditionalBlockDevices) > 0 {
		cfg.IAC.Main["additional_block_devices_worker"] = cfg.OpenCenter.Storage.AdditionalBlockDevices
	}

	// Map Deployment configuration
	cfg.IAC.Main["deploy_cluster"] = cfg.Deployment.AutoDeploy

	// Map Kubernetes configuration
	k8s := cfg.OpenCenter.Cluster.Kubernetes
	if k8s.Version != "" {
		cfg.IAC.Main["kubernetes_version"] = k8s.Version
	}
	if k8s.KubesprayVersion != "" {
		cfg.IAC.Main["kubespray_version"] = k8s.KubesprayVersion
	}
	if k8s.APIPort > 0 {
		cfg.IAC.Main["k8s_api_port"] = k8s.APIPort
	}
	cfg.IAC.Main["kube_vip_enabled"] = k8s.KubeVIPEnabled
	if k8s.MasterCount > 0 {
		cfg.IAC.Main["master_count"] = k8s.MasterCount
	}
	if k8s.WorkerCount > 0 {
		cfg.IAC.Main["worker_count"] = k8s.WorkerCount
	}
	if k8s.WorkerCountWindows > 0 {
		cfg.IAC.Main["worker_count_windows"] = k8s.WorkerCountWindows
	}
	if k8s.FlavorBastion != "" {
		cfg.IAC.Main["flavor_bastion"] = k8s.FlavorBastion
	}
	if k8s.FlavorMaster != "" {
		cfg.IAC.Main["flavor_master"] = k8s.FlavorMaster
	}
	if k8s.FlavorWorker != "" {
		cfg.IAC.Main["flavor_worker"] = k8s.FlavorWorker
	}
	if k8s.FlavorWorkerWindows != "" {
		cfg.IAC.Main["flavor_worker_windows"] = k8s.FlavorWorkerWindows
	}
	if k8s.SubnetPods != "" {
		cfg.IAC.Main["subnet_pods"] = k8s.SubnetPods
	}
	if k8s.SubnetServices != "" {
		cfg.IAC.Main["subnet_services"] = k8s.SubnetServices
	}
	if k8s.LoadbalancerProvider != "" {
		cfg.IAC.Main["loadbalancer_provider"] = k8s.LoadbalancerProvider
	}
	if k8s.DNSZoneName != "" {
		cfg.IAC.Main["dns_zone_name"] = k8s.DNSZoneName
	}

	// Map network plugin configuration with proper conditional logic
	if k8s.NetworkPlugin.Calico.Enabled {
		cfg.IAC.Main["network_plugin"] = "calico"
		if k8s.NetworkPlugin.Calico.CNIIface != "" {
			cfg.IAC.Main["cni_iface"] = k8s.NetworkPlugin.Calico.CNIIface
		}
		if k8s.NetworkPlugin.Calico.CalicoInterfaceAutodetect != "" {
			cfg.IAC.Main["calico_interface_autodetect"] = k8s.NetworkPlugin.Calico.CalicoInterfaceAutodetect
		}
		if k8s.NetworkPlugin.Calico.AutodetectCIDR != "" {
			cfg.IAC.Main["calico_interface_autodetect_cidr"] = k8s.NetworkPlugin.Calico.AutodetectCIDR
		}
		if k8s.NetworkPlugin.Calico.EncapsulationType != "" {
			cfg.IAC.Main["calico_encapsulation_type"] = k8s.NetworkPlugin.Calico.EncapsulationType
		}
		cfg.IAC.Main["calico_nat_outgoing"] = k8s.NetworkPlugin.Calico.NATOutgoing
	} else if k8s.NetworkPlugin.Cilium.Enabled {
		cfg.IAC.Main["network_plugin"] = "cilium"
		cfg.IAC.Main["cilium_operator_enabled"] = k8s.NetworkPlugin.Cilium.OperatorEnabled
		cfg.IAC.Main["cilium_kube_proxy_replacement"] = k8s.NetworkPlugin.Cilium.KubeProxyReplacement
	} else if k8s.NetworkPlugin.KubeOVN.Enabled {
		cfg.IAC.Main["network_plugin"] = "kube-ovn"
		cfg.IAC.Main["kube_ovn_cilium_integration"] = k8s.NetworkPlugin.KubeOVN.CiliumIntegration
	}

	// Map SSH authorized keys
	if len(cfg.OpenCenter.Cluster.SSHAuthorizedKeys) > 0 {
		cfg.IAC.Main["ssh_authorized_keys"] = cfg.OpenCenter.Cluster.SSHAuthorizedKeys
	}

	// Map baremetal node configurations
	if len(k8s.MasterNodes) > 0 {
		cfg.IAC.Main["master_nodes"] = k8s.MasterNodes
	}
	if len(k8s.WorkerNodes) > 0 {
		cfg.IAC.Main["worker_nodes"] = k8s.WorkerNodes
	}

	// Map additional server pools
	if len(k8s.AdditionalServerPoolsWorker) > 0 {
		cfg.IAC.Main["additional_server_pools_worker"] = k8s.AdditionalServerPoolsWorker
	}
	if len(k8s.AdditionalServerPoolsWorkerWindows) > 0 {
		cfg.IAC.Main["additional_server_pools_worker_windows"] = k8s.AdditionalServerPoolsWorkerWindows
	}

	return nil
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
	return saveConfig(cfg, false)
}

// SaveWithOmitEmpty writes the configuration to a YAML file, omitting empty fields.
// The file is saved with 0600 permissions to protect sensitive data.
// This is useful for cleaning up configurations by removing fields with zero values.
//
// Inputs:
//   - cfg: The configuration to save.
//
// Outputs:
//   - error: An error if the configuration cannot be saved.
func SaveWithOmitEmpty(cfg Config) error {
	return saveConfig(cfg, true)
}

// saveConfig is the internal implementation for saving configurations.
func saveConfig(cfg Config, omitEmpty bool) error {
	if cfg.ClusterName() == "" {
		return errors.New("cluster_name must not be empty")
	}

	// Try to get existing config path first
	path, err := ConfigPath(cfg.ClusterName())
	if err != nil {
		// If config doesn't exist, determine where to create it based on organization
		path, err = getConfigPathForSave(cfg)
		if err != nil {
			return err
		}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var data []byte
	var marshalErr error

	if omitEmpty {
		// Marshal to map first, then clean empty values
		var configMap map[string]any
		tempData, err := yaml.Marshal(&cfg)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(tempData, &configMap); err != nil {
			return err
		}

		// Remove empty values recursively
		cleanEmptyValues(configMap)

		data, marshalErr = yaml.Marshal(configMap)
	} else {
		// Standard marshal
		data, marshalErr = yaml.Marshal(&cfg)
	}

	if marshalErr != nil {
		return marshalErr
	}
	// Write with 0600 permissions
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		return writeErr
	}
	return nil
}

// cleanEmptyValues recursively removes empty values from a map.
// Empty values include: nil, empty strings, empty slices, empty maps, and zero numbers.
func cleanEmptyValues(m map[string]any) {
	for key, value := range m {
		if isEmpty(value) {
			delete(m, key)
			continue
		}

		// Recursively clean nested maps
		if nestedMap, ok := value.(map[string]any); ok {
			cleanEmptyValues(nestedMap)
			// Remove the nested map if it became empty after cleaning
			if len(nestedMap) == 0 {
				delete(m, key)
			}
		}
	}
}

// isEmpty checks if a value is considered empty.
func isEmpty(v any) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case bool:
		return false // Keep boolean values even if false
	case int, int8, int16, int32, int64:
		return val == 0
	case uint, uint8, uint16, uint32, uint64:
		return val == 0
	case float32, float64:
		return val == 0
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}

// getConfigPathForSave determines where to save a new cluster configuration.
// It uses organization structure if organization is set, otherwise uses flat file structure.
func getConfigPathForSave(cfg Config) (string, error) {
	configDir, err := ResolveConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve config directory: %w", err)
	}

	organization := cfg.OpenCenter.Meta.Organization
	clusterName := cfg.ClusterName()

	if organization != "" && organization != "opencenter" {
		// Use organization structure: clusters/<org>/.<cluster>-config.yaml
		return filepath.Join(configDir, "clusters", organization, "."+clusterName+"-config.yaml"), nil
	}

	// Use flat file structure for backward compatibility and default organization
	return filepath.Join(configDir, clusterName+".yaml"), nil
}

// List returns a sorted list of cluster names from the configuration directory.
// It looks for cluster directories within the configured clustersDir.
// It supports both organization-based and legacy directory structures.
//
// Outputs:
//   - []string: A list of cluster names.
//   - error: An error if the directory cannot be read.
func List() ([]string, error) {
	dir, err := ResolveConfigDir()
	if err != nil {
		Debugf("List: failed to resolve config directory: %v", err)
		return nil, fmt.Errorf("failed to resolve configuration directory: %w", err)
	}
	Debugf("List: resolved config directory: %s", dir)

	// Load CLI configuration to get the configured clustersDir
	configManager, err := NewConfigManager("")
	if err != nil {
		Debugf("List: failed to load CLI config manager: %v", err)
		// Fall back to default behavior if CLI config can't be loaded
	}

	var clustersDir string
	if configManager != nil {
		clustersDir = configManager.GetConfig().Paths.ClustersDir
		Debugf("List: using clustersDir from CLI config: %s", clustersDir)
	} else {
		// Fallback to default
		clustersDir = filepath.Join(dir, "clusters")
		Debugf("List: using default clustersDir: %s", clustersDir)
	}

	// Expand environment variables and tilde in clustersDir
	clustersDir = ExpandPath(clustersDir)
	Debugf("List: expanded clustersDir: %s", clustersDir)

	var names []string
	nameSet := make(map[string]bool) // Use set to avoid duplicates

	// First, check for flat YAML files in the config directory (for backward compatibility and tests)
	Debugf("List: checking for flat config files in: %s", dir)
	if flatEntries, flatErr := os.ReadDir(dir); flatErr == nil {
		for _, flatEntry := range flatEntries {
			if !flatEntry.IsDir() && strings.HasSuffix(flatEntry.Name(), ".yaml") {
				// Extract cluster name by removing .yaml extension
				clusterName := strings.TrimSuffix(flatEntry.Name(), ".yaml")
				// Skip the CLI config file itself
				if clusterName != "" && clusterName != "config" && !nameSet[clusterName] {
					Debugf("List: found flat config file: %s (cluster: %s)", flatEntry.Name(), clusterName)
					names = append(names, clusterName)
					nameSet[clusterName] = true
				}
			}
		}
	}
	Debugf("List: found %d flat config clusters", len(names))

	// Check clusters directory for legacy and organization-based structures
	Debugf("List: checking clusters directory: %s", clustersDir)
	entries, readErr := os.ReadDir(clustersDir)
	if readErr != nil {
		// If clusters directory doesn't exist, just return flat config files
		if os.IsNotExist(readErr) {
			Debugf("List: clusters directory does not exist, returning %d flat config clusters", len(names))
			// Sort lexically
			if len(names) > 1 {
				sortStrings(names)
			}
			return names, nil
		}
		Debugf("List: failed to read clusters directory: %v", readErr)
		return nil, fmt.Errorf("failed to read clusters directory: %w", readErr)
	}
	Debugf("List: found %d entries in clusters directory", len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			entryName := entry.Name()
			Debugf("List: processing directory entry: %s", entryName)

			// Check for legacy structure first: clustersDir/clusterName/.clusterName-config.yaml
			// This is for backward compatibility with old flat structure
			legacyConfigFile := filepath.Join(clustersDir, entryName, "."+entryName+"-config.yaml")
			Debugf("List: checking for legacy config file: %s", legacyConfigFile)
			if _, err := os.Stat(legacyConfigFile); err == nil {
				Debugf("List: found legacy config file for: %s", entryName)
				// Check if this is truly legacy (no infrastructure/clusters subdirs OR no applications subdirs)
				infraDir := filepath.Join(clustersDir, entryName, "infrastructure", "clusters")
				appsDir := filepath.Join(clustersDir, entryName, "applications", "overlays")
				hasInfra := false
				hasApps := false
				if _, err := os.Stat(infraDir); err == nil {
					hasInfra = true
					Debugf("List: %s has infrastructure directory", entryName)
				}
				if _, err := os.Stat(appsDir); err == nil {
					hasApps = true
					Debugf("List: %s has applications directory", entryName)
				}

				// If it has neither infrastructure nor applications subdirs, it's legacy flat structure
				if !hasInfra && !hasApps {
					Debugf("List: %s is legacy flat structure (no infra/apps dirs)", entryName)
					if !nameSet[entryName] {
						Debugf("List: adding legacy cluster: %s", entryName)
						names = append(names, entryName)
						nameSet[entryName] = true
					}
					continue // Skip organization check for this entry
				} else {
					Debugf("List: %s has subdirs (infra=%v, apps=%v), treating as organization", entryName, hasInfra, hasApps)
				}
			} else {
				Debugf("List: no legacy config file found for: %s", entryName)
			}

			// Check for organization-based structure
			// Look for clusters in: clustersDir/organization/infrastructure/clusters/<cluster>/.<cluster>-config.yaml
			orgDir := filepath.Join(clustersDir, entryName)
			infraClustersDir := filepath.Join(orgDir, "infrastructure", "clusters")
			Debugf("List: checking organization infrastructure/clusters directory: %s", infraClustersDir)

			if infraEntries, err := os.ReadDir(infraClustersDir); err == nil {
				Debugf("List: found %d entries in infrastructure/clusters directory for org: %s", len(infraEntries), entryName)
				for _, clusterEntry := range infraEntries {
					if clusterEntry.IsDir() {
						clusterName := clusterEntry.Name()
						// Check for config file at cluster directory level
						clusterConfigPath := filepath.Join(infraClustersDir, clusterName, "."+clusterName+"-config.yaml")
						Debugf("List: checking for config file: %s", clusterConfigPath)
						if _, statErr := os.Stat(clusterConfigPath); statErr == nil {
							Debugf("List: found cluster config file for: %s", clusterName)
							// Format as organization/cluster
							fullName := entryName + "/" + clusterName
							if !nameSet[fullName] {
								Debugf("List: adding organization cluster: %s", fullName)
								names = append(names, fullName)
								nameSet[fullName] = true
							} else {
								Debugf("List: skipping duplicate cluster: %s", fullName)
							}
						}
					}
				}
			} else {
				Debugf("List: infrastructure/clusters directory does not exist for org %s: %v", entryName, err)
			}

			// Also check for config files at organization level (alternative location)
			if orgFiles, err := os.ReadDir(orgDir); err == nil {
				Debugf("List: found %d files in organization directory: %s", len(orgFiles), entryName)
				for _, orgFile := range orgFiles {
					if !orgFile.IsDir() && strings.HasPrefix(orgFile.Name(), ".") && strings.HasSuffix(orgFile.Name(), "-config.yaml") {
						Debugf("List: found organization-level config file: %s", orgFile.Name())
						// Extract cluster name from .<cluster>-config.yaml
						clusterName := strings.TrimPrefix(orgFile.Name(), ".")
						clusterName = strings.TrimSuffix(clusterName, "-config.yaml")
						Debugf("List: extracted cluster name: %s from file: %s", clusterName, orgFile.Name())
						if clusterName != "" {
							// Format as organization/cluster
							fullName := entryName + "/" + clusterName
							if !nameSet[fullName] {
								Debugf("List: adding organization cluster: %s", fullName)
								names = append(names, fullName)
								nameSet[fullName] = true
							} else {
								Debugf("List: skipping duplicate cluster: %s", fullName)
							}
						} else {
							Debugf("List: skipping cluster (name is empty)")
						}
					}
				}
			} else {
				Debugf("List: failed to read organization directory %s: %v", orgDir, err)
			}
		} else {
			Debugf("List: skipping non-directory entry: %s", entry.Name())
		}
	}

	// Sort lexically
	Debugf("List: sorting %d cluster names", len(names))
	if len(names) > 1 {
		sortStrings(names)
	}
	Debugf("List: returning %d total clusters", len(names))
	for i, name := range names {
		Debugf("List: final result[%d]: %s", i, name)
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
		case "s3", "aws":
			s3 := cfg.OpenTofu.Backend.S3
			if s3.Bucket == "" || s3.Key == "" || s3.Region == "" {
				errs = append(errs, "opentofu.backend.s3 requires bucket, key, and region")
			}
			// When using S3/AWS backend, AWS credentials must be provided via opencenter cluster or global AWS secrets
			clusterAccessKey := strings.TrimSpace(cfg.OpenCenter.Cluster.AWSAccessKey)
			clusterSecretKey := strings.TrimSpace(cfg.OpenCenter.Cluster.AWSSecretAccessKey)
			globalAccessKey := strings.TrimSpace(cfg.Secrets.AWS.AccessKey)
			globalSecretKey := strings.TrimSpace(cfg.Secrets.AWS.SecretAccessKey)
			
			if (clusterAccessKey == "" && globalAccessKey == "") || (clusterSecretKey == "" && globalSecretKey == "") {
				errs = append(errs, "AWS credentials required for S3/AWS backend: either set opencenter.cluster.aws_access_key/aws_secret_access_key or secrets.aws.access_key/secret_access_key")
			}
		default:
			errs = append(errs, "opentofu.backend.type must be 'local', 's3', or 'aws'")
		}
	}
	// iac validation is intentionally minimal for variables.tf-aligned shape

	// Network plugin validation - ensure only one is enabled
	networkPlugins := []struct {
		name    string
		enabled bool
	}{
		{"Calico", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled},
		{"Cilium", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled},
		{"Kube-OVN", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled},
	}

	enabledCount := 0
	var enabledPlugins []string
	for _, plugin := range networkPlugins {
		if plugin.enabled {
			enabledCount++
			enabledPlugins = append(enabledPlugins, plugin.name)
		}
	}

	if enabledCount == 0 {
		errs = append(errs, "at least one network plugin (Calico, Cilium, or Kube-OVN) must be enabled")
	} else if enabledCount > 1 {
		errs = append(errs, fmt.Sprintf("only one network plugin can be enabled at a time, but found: %s", strings.Join(enabledPlugins, ", ")))
	}

	// Windows node validation - exclude Windows blocks when worker_count_windows = 0
	if cfg.OpenCenter.Cluster.Kubernetes.WorkerCountWindows == 0 {
		// Windows workers should be disabled when count is 0
		if cfg.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled {
			errs = append(errs, "windows_workers.enabled must be false when worker_count_windows is 0")
		}
	}

	// Validate services: only one of release or branch can be set
	for serviceName, serviceCfgAny := range cfg.OpenCenter.Services {
		// All services embed BaseConfig, but we can't cast directly to *BaseConfig
		// because they are different types. We use reflection to access the fields.
		val := reflect.ValueOf(serviceCfgAny)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			// Check if struct has BaseConfig embedded or Release/Branch fields directly
			// Since BaseConfig is embedded, its fields are promoted
			releaseField := val.FieldByName("Release")
			branchField := val.FieldByName("Branch")

			if releaseField.IsValid() && branchField.IsValid() {
				release := releaseField.String()
				branch := branchField.String()

				if release != "" && branch != "" {
					errs = append(errs, fmt.Sprintf("service '%s': only one of 'release' or 'branch' can be set, not both", serviceName))
				}
			}
		}
	}

	// Validate managed services: only one of release or branch can be set
	for serviceName, serviceCfgAny := range cfg.OpenCenter.ManagedService {
		val := reflect.ValueOf(serviceCfgAny)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			releaseField := val.FieldByName("Release")
			branchField := val.FieldByName("Branch")

			if releaseField.IsValid() && branchField.IsValid() {
				release := releaseField.String()
				branch := branchField.String()

				if release != "" && branch != "" {
					errs = append(errs, fmt.Sprintf("managed-service '%s': only one of 'release' or 'branch' can be set, not both", serviceName))
				}
			}
		}
	}

	// Validate GitOps: only one of release or branch can be set
	if cfg.OpenCenter.GitOps.Release != "" && cfg.OpenCenter.GitOps.Branch != "" {
		errs = append(errs, "gitops: only one of 'release' or 'branch' can be set, not both")
	}

	// Validate service secrets
	errs = append(errs, validateServiceSecretsSimple(cfg)...)

	// Validate OpenStack provider configuration
	if cfg.OpenCenter.Infrastructure.Provider == "openstack" {
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL == "" {
			errs = append(errs, "opencenter.infrastructure.cloud.openstack.auth_url must be set when provider is openstack")
		}
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region == "" {
			errs = append(errs, "opencenter.infrastructure.cloud.openstack.region must be set when provider is openstack")
		}
	}
	// Validate Barbican configuration if enabled
	if cfg.OpenCenter.Secrets.Backend == "barbican" {
		if cfg.OpenCenter.Secrets.Barbican.AuthURL == "" {
			errs = append(errs, "opencenter.secrets.barbican.auth_url must be set when secrets backend is barbican")
		}
	}

	return errs
}

// validateServiceSecretsSimple validates service-specific secrets configuration.
// This function checks that required secrets are present when corresponding services are enabled.
func validateServiceSecretsSimple(cfg Config) []string {
	var errs []string

	isEnabled := func(name string) bool {
		svc, exists := cfg.OpenCenter.Services[name]
		if !exists {
			return false
		}
		if svcConf, ok := svc.(services.ServiceConfig); ok {
			return svcConf.IsEnabled()
		}
		return false
	}

	// Validate cert-manager secrets
	if isEnabled("cert-manager") {
		accessKey, secretKey := cfg.GetCertManagerAWSCredentials()
		if accessKey == "" {
			errs = append(errs, "AWS credentials required for cert-manager: either set secrets.cert_manager.aws_access_key or secrets.aws.access_key")
		}
		if secretKey == "" {
			errs = append(errs, "AWS credentials required for cert-manager: either set secrets.cert_manager.aws_secret_access_key or secrets.aws.secret_access_key")
		}
	}

	// Validate loki secrets
	if isEnabled("loki") {
		// Check for Swift credentials (legacy)
		if cfg.Secrets.Loki.SwiftPassword == "" {
			// If no Swift password, check for S3 credentials (with fallback)
			accessKey, secretKey := cfg.GetLokiS3Credentials()
			if accessKey == "" || secretKey == "" {
				errs = append(errs, "Loki requires either Swift password (secrets.loki.swift_password) or S3 credentials (secrets.loki.s3_access_key_id/secrets.loki.s3_secret_access_key or secrets.aws.access_key/secrets.aws.secret_access_key)")
			}
		}
	}

	// Validate tempo secrets
	if isEnabled("tempo") {
		accessKey, secretKey := cfg.GetTempoS3Credentials()
		if accessKey == "" {
			errs = append(errs, "S3 credentials required for Tempo: either set secrets.tempo.access_key or secrets.aws.access_key")
		}
		if secretKey == "" {
			errs = append(errs, "S3 credentials required for Tempo: either set secrets.tempo.secret_key or secrets.aws.secret_access_key")
		}
	}

	// Validate keycloak secrets
	if isEnabled("keycloak") {
		if cfg.Secrets.Keycloak.AdminPassword == "" {
			errs = append(errs, "secrets.keycloak.admin_password is required when keycloak is enabled")
		}
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

// GetAWSCredentials returns AWS credentials with fallback logic.
// If service-specific credentials are not provided, it falls back to global AWS credentials.
//
// Parameters:
//   - serviceAccessKey: Service-specific AWS access key
//   - serviceSecretKey: Service-specific AWS secret access key
//
// Returns:
//   - accessKey: The resolved AWS access key
//   - secretKey: The resolved AWS secret access key
func (c Config) GetAWSCredentials(serviceAccessKey, serviceSecretKey string) (accessKey, secretKey string) {
	// Use service-specific credentials if provided
	if serviceAccessKey != "" && serviceSecretKey != "" {
		return serviceAccessKey, serviceSecretKey
	}
	
	// Fall back to global AWS credentials
	return c.Secrets.AWS.AccessKey, c.Secrets.AWS.SecretAccessKey
}

// GetCertManagerAWSCredentials returns cert-manager AWS credentials with fallback to global AWS credentials.
func (c Config) GetCertManagerAWSCredentials() (accessKey, secretKey string) {
	return c.GetAWSCredentials(c.Secrets.CertManager.AWSAccessKey, c.Secrets.CertManager.AWSSecretAccessKey)
}

// GetLokiS3Credentials returns Loki S3 credentials with fallback to global AWS credentials.
func (c Config) GetLokiS3Credentials() (accessKey, secretKey string) {
	return c.GetAWSCredentials(c.Secrets.Loki.S3AccessKeyID, c.Secrets.Loki.S3SecretAccessKey)
}

// GetTempoS3Credentials returns Tempo S3 credentials with fallback to global AWS credentials.
func (c Config) GetTempoS3Credentials() (accessKey, secretKey string) {
	return c.GetAWSCredentials(c.Secrets.Tempo.AccessKey, c.Secrets.Tempo.SecretKey)
}

// GetS3BackendCredentials returns S3 backend credentials with fallback to global AWS credentials.
func (c Config) GetS3BackendCredentials() (accessKey, secretKey string) {
	return c.GetAWSCredentials(c.OpenCenter.Cluster.AWSAccessKey, c.OpenCenter.Cluster.AWSSecretAccessKey)
}

// Template-friendly functions that return single values for use in Go templates

// GetCertManagerAWSAccessKey returns cert-manager AWS access key with fallback.
func (c Config) GetCertManagerAWSAccessKey() string {
	accessKey, _ := c.GetCertManagerAWSCredentials()
	return accessKey
}

// GetCertManagerAWSSecretKey returns cert-manager AWS secret key with fallback.
func (c Config) GetCertManagerAWSSecretKey() string {
	_, secretKey := c.GetCertManagerAWSCredentials()
	return secretKey
}

// GetLokiS3AccessKey returns Loki S3 access key with fallback.
func (c Config) GetLokiS3AccessKey() string {
	accessKey, _ := c.GetLokiS3Credentials()
	return accessKey
}

// GetLokiS3SecretKey returns Loki S3 secret key with fallback.
func (c Config) GetLokiS3SecretKey() string {
	_, secretKey := c.GetLokiS3Credentials()
	return secretKey
}

// GetTempoS3AccessKey returns Tempo S3 access key with fallback.
func (c Config) GetTempoS3AccessKey() string {
	accessKey, _ := c.GetTempoS3Credentials()
	return accessKey
}

// GetTempoS3SecretKey returns Tempo S3 secret key with fallback.
func (c Config) GetTempoS3SecretKey() string {
	_, secretKey := c.GetTempoS3Credentials()
	return secretKey
}

// GetS3BackendAccessKey returns S3 backend access key with fallback.
func (c Config) GetS3BackendAccessKey() string {
	accessKey, _ := c.GetS3BackendCredentials()
	return accessKey
}

// GetS3BackendSecretKey returns S3 backend secret key with fallback.
func (c Config) GetS3BackendSecretKey() string {
	_, secretKey := c.GetS3BackendCredentials()
	return secretKey
}
