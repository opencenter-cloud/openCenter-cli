package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	overlaycfg "github.com/opencenter-cloud/opencenter-cli/internal/config/overlay"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

type clusterSpec struct {
	Name                  string
	Region                string
	EnableServices        []string
	EnableManagedServices []string
	SOPSEnabled           bool
	CustomerManaged       bool
	CustomerManagedSecret bool
	HarborEnabled         bool
	HarborEmitCertificate bool
}

var relayPointSpecs = []clusterSpec{
	{
		Name:   "k8s-dev",
		Region: "iad3",
		EnableServices: []string{
			"calico",
			"cert-manager",
			"etcd-backup",
			"external-snapshotter",
			"gateway",
			"gateway-api",
			"harbor",
			"headlamp",
			"kafka-cluster",
			"keycloak",
			"kube-prometheus-stack",
			"kyverno",
			"loki",
			"metallb",
			"mimir",
			"olm",
			"openstack-ccm",
			"openstack-csi",
			"opentelemetry-kube-stack",
			"postgres-operator",
			"rbac-manager",
			"tempo",
			"velero",
			"vsphere-csi",
			"weave-gitops",
		},
		EnableManagedServices: []string{"alert-proxy"},
		SOPSEnabled:           true,
		CustomerManaged:       true,
		HarborEnabled:         true,
		HarborEmitCertificate: true,
	},
	{
		Name:   "k8s-dr",
		Region: "iad3",
		EnableServices: []string{
			"calico",
			"cert-manager",
			"external-snapshotter",
			"gateway",
			"gateway-api",
			"harbor",
			"headlamp",
			"kafka-cluster",
			"keycloak",
			"kube-prometheus-stack",
			"loki",
			"metallb",
			"mimir",
			"olm",
			"openstack-ccm",
			"openstack-csi",
			"opentelemetry-kube-stack",
			"postgres-operator",
			"rbac-manager",
			"sealed-secrets",
			"tempo",
			"velero",
			"vsphere-csi",
			"weave-gitops",
		},
		EnableManagedServices: []string{"alert-proxy"},
		SOPSEnabled:           true,
		CustomerManaged:       true,
		HarborEnabled:         true,
	},
	{
		Name:   "k8s-prod",
		Region: "ord1",
		EnableServices: []string{
			"calico",
			"cert-manager",
			"gateway",
			"gateway-api",
			"headlamp",
			"kafka-cluster",
			"keycloak",
			"kube-prometheus-stack",
			"loki",
			"metallb",
			"mimir",
			"olm",
			"opentelemetry-kube-stack",
			"postgres-operator",
			"rbac-manager",
			"tempo",
			"velero",
			"vsphere-csi",
		},
		EnableManagedServices: []string{"alert-proxy"},
	},
	{
		Name:   "k8s-qa",
		Region: "ord1",
		EnableServices: []string{
			"calico",
			"cert-manager",
			"external-snapshotter",
			"gateway",
			"gateway-api",
			"headlamp",
			"kafka-cluster",
			"keycloak",
			"kube-prometheus-stack",
			"kyverno",
			"loki",
			"longhorn",
			"metallb",
			"mimir",
			"olm",
			"openstack-ccm",
			"openstack-csi",
			"opentelemetry-kube-stack",
			"postgres-operator",
			"rbac-manager",
			"sealed-secrets",
			"tempo",
			"velero",
			"vsphere-csi",
			"weave-gitops",
		},
		EnableManagedServices: []string{"alert-proxy"},
		CustomerManaged:       true,
	},
	{
		Name:   "k8s-uat",
		Region: "ord1",
		EnableServices: []string{
			"calico",
			"cert-manager",
			"external-snapshotter",
			"gateway",
			"gateway-api",
			"headlamp",
			"kafka-cluster",
			"keycloak",
			"kube-prometheus-stack",
			"kyverno",
			"loki",
			"metallb",
			"mimir",
			"olm",
			"openstack-ccm",
			"openstack-csi",
			"opentelemetry-kube-stack",
			"postgres-operator",
			"rbac-manager",
			"tempo",
			"velero",
			"vsphere-csi",
			"weave-gitops",
		},
		EnableManagedServices: []string{"alert-proxy"},
		CustomerManaged:       true,
		CustomerManagedSecret: true,
	},
}

func main() {
	root := filepath.Join("testdata", "relaypoint-logistics-shared")
	for _, spec := range relayPointSpecs {
		cfg := buildRelayPointConfig(spec)
		data, err := config.MarshalConfigOptimized(&cfg)
		if err != nil {
			panic(fmt.Errorf("marshal %s config: %w", spec.Name, err))
		}

		path := filepath.Join(root, "."+spec.Name+"-config.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			panic(fmt.Errorf("write %s: %w", path, err))
		}
		fmt.Printf("wrote %s\n", path)
	}
}

func buildRelayPointConfig(spec clusterSpec) config.Config {
	cfg := config.NewDefault(spec.Name)
	cfg.Metadata = config.ConfigMetadata{
		CreatedAt: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		CreatedBy: "fixture-generator",
	}

	clusterFQDN := fmt.Sprintf("fcc.%s.%s.k8s.opencenter.cloud", spec.Name, spec.Region)
	baseDomain := fmt.Sprintf("%s.k8s.opencenter.cloud", spec.Region)

	cfg.OpenCenter.Meta.Name = spec.Name
	cfg.OpenCenter.Meta.Env = strings.TrimPrefix(spec.Name, "k8s-")
	cfg.OpenCenter.Meta.Region = spec.Region
	cfg.OpenCenter.Meta.Organization = "opencenter"

	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = spec.Region
	cfg.OpenCenter.Cluster.ClusterName = spec.Name
	cfg.OpenCenter.Cluster.ClusterFQDN = clusterFQDN
	cfg.OpenCenter.Cluster.BaseDomain = baseDomain
	cfg.OpenCenter.Cluster.SSHAuthorizedKeys = []string{config.DefaultSSHAuthorizedKeyPlaceholder}

	cfg.OpenCenter.GitOps.GitDir = "./testdata/relaypoint-logistics-shared"
	cfg.OpenCenter.GitOps.GitURL = "ssh://git@github.com/rpc-environments/7742901-RelayPoint-Logistics.git"
	cfg.OpenCenter.GitOps.GitOpsBaseRepo = "ssh://git@github.com/rackerlabs/openCenter-gitops-base.git"
	cfg.OpenCenter.GitOps.GitOpsBaseRelease = ""
	cfg.OpenCenter.GitOps.GitOpsBranch = "main"
	cfg.OpenCenter.GitOps.GitBranch = "main"

	for name, service := range cfg.OpenCenter.Services {
		setEnabled(service, false)
		cfg.OpenCenter.Services[name] = service
	}
	for _, name := range spec.EnableServices {
		service := cfg.OpenCenter.Services[name]
		if service == nil {
			continue
		}
		setEnabled(service, true)
		cfg.OpenCenter.Services[name] = service
	}

	for name, service := range cfg.OpenCenter.ManagedService {
		setEnabled(service, false)
		cfg.OpenCenter.ManagedService[name] = service
	}
	for _, name := range spec.EnableManagedServices {
		service := cfg.OpenCenter.ManagedService[name]
		if service == nil {
			continue
		}
		setEnabled(service, true)
		cfg.OpenCenter.ManagedService[name] = service
	}

	if service, ok := cfg.OpenCenter.Services["headlamp"].(*services.HeadlampConfig); ok {
		service.Hostname = fmt.Sprintf("headlamp.%s", clusterFQDN)
		service.OIDCIssuerURL = fmt.Sprintf("https://auth.%s/realms/opencenter", clusterFQDN)
	}
	if service, ok := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig); ok {
		service.Hostname = fmt.Sprintf("auth.%s", clusterFQDN)
		service.FrontendURL = fmt.Sprintf("https://auth.%s", clusterFQDN)
	}
	if service, ok := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig); ok {
		service.Hostname = fmt.Sprintf("harbor.%s", clusterFQDN)
		service.EmitCertificate = spec.HarborEmitCertificate
		setEnabled(service, spec.HarborEnabled)
	}

	cfg.OpenCenter.GitOps.OverlayUnits.SOPS = overlaycfg.SOPSGenerationConfig{
		Enabled: spec.SOPSEnabled,
		Rules: []overlaycfg.SOPSGenerationRule{
			{
				PathRegex:     `^managed-services/.*/helm-values/.*\.ya?ml$`,
				AgeRecipients: []string{"age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn"},
			},
			{
				PathRegex:     `^services/.*/helm-values/.*\.ya?ml$`,
				AgeRecipients: []string{"age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn"},
			},
			{
				PathRegex:      `^services/.*/.*\.ya?ml$`,
				AgeRecipients:  []string{"age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn"},
				EncryptedRegex: "^(data|stringData|credentials)$",
			},
		},
	}
	if !spec.SOPSEnabled {
		cfg.OpenCenter.GitOps.OverlayUnits.SOPS.Rules = nil
	}

	cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
		Enabled:        spec.CustomerManaged,
		RepositoryName: "customer-repository-rpl-apps-flux-k8s",
		RepositoryURL:  "ssh://relaypointlogistics@git.relaypointlogistics.com/rpl/apps-flux-k8s.git",
		Branch:         "main",
		Interval:       "15m",
		SecretName:     "customer-repository-rpl-apps-flux-k8s",
		EmitSecret:     spec.CustomerManagedSecret,
		Kustomizations: []overlaycfg.CustomerManagedKustomization{
			{
				Name:      fmt.Sprintf("git.relaypointlogistics.com-rpl-apps-flux-k8s-policies-%s", cfg.OpenCenter.Meta.Env),
				Path:      fmt.Sprintf("/policies/%s", cfg.OpenCenter.Meta.Env),
				DependsOn: []string{"customer-managed-sources"},
			},
			{
				Name:      fmt.Sprintf("git.relaypointlogistics.com-rpl-apps-flux-k8s-infrastructure-%s", cfg.OpenCenter.Meta.Env),
				Path:      fmt.Sprintf("/infrastructure/%s", cfg.OpenCenter.Meta.Env),
				DependsOn: []string{fmt.Sprintf("git.relaypointlogistics.com-rpl-apps-flux-k8s-policies-%s", cfg.OpenCenter.Meta.Env)},
			},
			{
				Name:      fmt.Sprintf("git.relaypointlogistics.com-rpl-apps-flux-k8s-apps-%s", cfg.OpenCenter.Meta.Env),
				Path:      fmt.Sprintf("/apps/%s", cfg.OpenCenter.Meta.Env),
				DependsOn: []string{fmt.Sprintf("git.relaypointlogistics.com-rpl-apps-flux-k8s-infrastructure-%s", cfg.OpenCenter.Meta.Env)},
			},
		},
	}
	if !spec.CustomerManaged {
		cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged.Kustomizations = nil
	}

	cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "PLACEHOLDER-AWS-INFRA-ACCESS-KEY"
	cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "PLACEHOLDER-AWS-INFRA-SECRET-KEY"
	cfg.Secrets.Global.AWS.Application.AccessKey = "PLACEHOLDER-AWS-APP-ACCESS-KEY"
	cfg.Secrets.Global.AWS.Application.SecretAccessKey = "PLACEHOLDER-AWS-APP-SECRET-KEY"
	cfg.Secrets.CertManager.AWSAccessKey = "PLACEHOLDER-CERT-MANAGER-AWS-ACCESS-KEY"
	cfg.Secrets.CertManager.AWSSecretAccessKey = "PLACEHOLDER-CERT-MANAGER-AWS-SECRET-KEY"
	cfg.Secrets.Grafana.AdminPassword = "PLACEHOLDER-GRAFANA-ADMIN-PASSWORD"
	cfg.Secrets.Headlamp.OIDCClientSecret = "PLACEHOLDER-HEADLAMP-OIDC-CLIENT-SECRET"
	cfg.Secrets.Keycloak.AdminPassword = "PLACEHOLDER-KEYCLOAK-ADMIN-PASSWORD"
	cfg.Secrets.Keycloak.ClientSecret = "PLACEHOLDER-KEYCLOAK-CLIENT-SECRET"
	cfg.Secrets.Loki.SwiftPassword = "PLACEHOLDER-LOKI-SWIFT-PASSWORD"
	cfg.Secrets.Tempo.AccessKey = "PLACEHOLDER-TEMPO-ACCESS-KEY"
	cfg.Secrets.Tempo.SecretKey = "PLACEHOLDER-TEMPO-SECRET-KEY"
	cfg.Secrets.AlertProxy.CoreDeviceId = "PLACEHOLDER-ALERT-PROXY-CORE-DEVICE-ID"
	cfg.Secrets.AlertProxy.AccountServiceToken = "PLACEHOLDER-ALERT-PROXY-ACCOUNT-SERVICE-TOKEN"
	cfg.Secrets.AlertProxy.CoreAccountNumber = "PLACEHOLDER-ALERT-PROXY-CORE-ACCOUNT-NUMBER"
	cfg.Secrets.VSphereCsi.VCenterHost = "PLACEHOLDER-VSPHERE-VCENTER-HOST"
	cfg.Secrets.VSphereCsi.Username = "PLACEHOLDER-VSPHERE-USERNAME"
	cfg.Secrets.VSphereCsi.Password = "PLACEHOLDER-VSPHERE-PASSWORD"
	cfg.Secrets.VSphereCsi.Datacenters = "PLACEHOLDER-VSPHERE-DATACENTERS"

	cfg.Secrets.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedSecrets{
		Identity:    "PLACEHOLDER-NOT-A-REAL-KEY",
		IdentityPub: "PLACEHOLDER-NOT-A-REAL-PUBLIC-KEY",
		KnownHosts:  "PLACEHOLDER-NOT-A-REAL-KNOWN-HOST",
	}
	if !spec.CustomerManagedSecret {
		cfg.Secrets.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedSecrets{}
	}

	if service, ok := cfg.OpenCenter.ManagedService["alert-proxy"].(*services.AlertProxyConfig); ok {
		service.Uri = "https://github.com/rackerlabs/alert-proxy.git"
		service.Branch = "main"
		service.ImageTag = "1761602071"
		service.AlertManagerBaseUrl = "http://observability-kube-prometh-alertmanager.observability.svc.cluster.local:9093/api/v2/alerts"
		service.HTTPRouteFQDN = clusterFQDN
	}

	if service, ok := cfg.OpenCenter.Services["weave-gitops"].(*services.WeaveGitOpsConfig); ok {
		service.Hostname = fmt.Sprintf("gitops.%s", clusterFQDN)
	}

	sortServiceMaps(cfg.OpenCenter.Services)
	sortServiceMaps(cfg.OpenCenter.ManagedService)

	return cfg
}

func setEnabled(service any, enabled bool) {
	value := reflect.ValueOf(service)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return
	}

	value = value.Elem()
	baseField := value.FieldByName("BaseConfig")
	if baseField.IsValid() && baseField.CanAddr() {
		enabledField := baseField.FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.CanSet() {
			enabledField.SetBool(enabled)
			return
		}
	}

	enabledField := value.FieldByName("Enabled")
	if enabledField.IsValid() && enabledField.CanSet() && enabledField.Kind() == reflect.Bool {
		enabledField.SetBool(enabled)
	}
}

func sortServiceMaps(serviceMap config.ServiceMap) {
	keys := make([]string, 0, len(serviceMap))
	for key := range serviceMap {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	ordered := make(config.ServiceMap, len(serviceMap))
	for _, key := range keys {
		ordered[key] = serviceMap[key]
	}

	clear(serviceMap)
	for _, key := range keys {
		serviceMap[key] = ordered[key]
	}
}
