package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestValidateReadinessOpenStackOfflineRules(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "http://openstack.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkID = ""
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkName = ""
	cfg.OpenCenter.Infrastructure.Compute.FlavorWorker = ""
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = []WorkerPoolConfig{
		{Name: "gpu", Count: 1, Flavor: ""},
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityWarning, CategoryProvider, "opencenter.infrastructure.cloud.openstack.auth_url")
	assertNoIssue(t, report, "opencenter.infrastructure.cloud.openstack.network_id")
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.flavor_worker")
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.additional_server_pools_worker[0].flavor")
	if report.Valid {
		t.Fatalf("expected readiness validation to fail, got valid report: %#v", report)
	}
}

func TestValidateReadinessOpenStackAllowsOpenTofuManagedNetwork(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkID = ""
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.NetworkName = ""

	report := ValidateReadiness(cfg)

	assertNoIssue(t, report, "opencenter.infrastructure.cloud.openstack.network_id")
}

func TestValidateReadinessNetworkPluginAllowsOneEnabledPlugin(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")

	report := ValidateReadiness(cfg)

	assertNoIssue(t, report, "opencenter.cluster.kubernetes.network_plugin")
	assertNoIssue(t, report, "opencenter.cluster.kubernetes.network_plugin.calico.install_method")
	if !report.Valid {
		t.Fatalf("expected readiness validation to pass, got:\n%s", renderIssues(report.Issues))
	}
}

func TestValidateReadinessNetworkPluginRequiresOneEnabledPlugin(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategorySchema, "opencenter.cluster.kubernetes.network_plugin")
}

func TestValidateReadinessNetworkPluginRejectsMultipleEnabledPlugins(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium = &CiliumConfig{Enabled: true}

	report := ValidateReadiness(cfg)

	issue := assertIssue(t, report, SeverityError, CategorySchema, "opencenter.cluster.kubernetes.network_plugin")
	if !strings.Contains(issue.Message, "calico") || !strings.Contains(issue.Message, "cilium") {
		t.Fatalf("expected multiple-plugin issue to name enabled plugins, got: %q", issue.Message)
	}
}

func TestValidateReadinessOpenStackRejectsKubesprayNetworkPluginInstall(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = "kubespray"

	report := ValidateReadiness(cfg)

	issue := assertIssue(t, report, SeverityError, CategorySchema, "opencenter.cluster.kubernetes.network_plugin.calico.install_method")
	if !strings.Contains(issue.Message, "kubespray") || !strings.Contains(issue.Suggestion, "helm") || !strings.Contains(issue.Suggestion, "kustomize-helm") {
		t.Fatalf("expected kubespray migration guidance, got issue: %#v", issue)
	}
}

func TestValidateReadinessOpenStackAcceptsKustomizeHelmNetworkPluginInstall(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = "kustomize-helm"

	report := ValidateReadiness(cfg)

	assertNoIssue(t, report, "opencenter.cluster.kubernetes.network_plugin.calico.install_method")
	if !report.Valid {
		t.Fatalf("expected readiness validation to pass, got:\n%s", renderIssues(report.Issues))
	}
}

func TestValidateReadinessOpenStackRejectsUnsupportedNetworkPluginInstall(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.InstallMethod = "flux"

	report := ValidateReadiness(cfg)

	issue := assertIssue(t, report, SeverityError, CategorySchema, "opencenter.cluster.kubernetes.network_plugin.calico.install_method")
	if !strings.Contains(issue.Message, "flux") || !strings.Contains(issue.Message, "helm") || !strings.Contains(issue.Message, "kustomize-helm") {
		t.Fatalf("expected unsupported install method issue to name accepted values, got: %#v", issue)
	}
}

func TestValidateReadinessGitOpsHTTPSRequiresMatchingTokenProvider(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.GitOps.Repository.URL = "https://github.com/example/cluster.git"
	cfg.OpenCenter.GitOps.Auth.SSH = nil
	cfg.OpenCenter.GitOps.Auth.Token = &GitOpsTokenAuth{
		Provider:  "gitlab",
		TokenFile: "secrets/github-token.txt",
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryGitOps, "opencenter.gitops.auth.token.provider")
}

func TestValidateReadinessGitOpsSSHRequiresKeyPaths(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.OpenCenter.GitOps.Repository.URL = "ssh://git@github.com/example/cluster.git"
	cfg.OpenCenter.GitOps.Auth.Token = nil
	cfg.OpenCenter.GitOps.Auth.SSH = &GitOpsSSHAuth{PrivateKey: "", PublicKey: PlaceholderSecret}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryGitOps, "opencenter.gitops.auth.ssh.private_key")
	assertIssue(t, report, SeverityError, CategoryGitOps, "opencenter.gitops.auth.ssh.public_key")
}

func TestValidateReadinessServiceSecretsOnlyForEnabledServices(t *testing.T) {
	cfg := validReadinessConfig(t, "openstack")
	cfg.Secrets.Keycloak.AdminPassword = ""
	cfg.Secrets.Grafana.AdminPassword = PlaceholderSecret

	if svc, ok := cfg.OpenCenter.Services["weave-gitops"].(*services.DefaultServiceConfig); ok {
		svc.Enabled = true
	}
	cfg.Secrets.WeaveGitOps.Password = ""
	cfg.Secrets.WeaveGitOps.PasswordHash = ""

	if svc, ok := cfg.OpenCenter.Services["kube-prometheus-stack"].(*services.PrometheusStackConfig); ok {
		svc.Enabled = false
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryServices, "secrets.keycloak.admin_password")
	assertIssue(t, report, SeverityError, CategoryServices, "secrets.weave_gitops.password")
	assertNoIssue(t, report, "secrets.grafana.admin_password")
}

func TestValidateReadinessInternalOIDCDefersBootstrapGeneratedClientSecrets(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Identity.OIDC.Enabled = true
	cfg.OpenCenter.Identity.OIDC.Source = OIDCSourceInternal
	cfg.OpenCenter.Identity.OIDC.Provider = OIDCProviderKeycloak
	cfg.Secrets.Keycloak.ClientSecret = ""
	cfg.Secrets.Keycloak.AdminPassword = ""
	cfg.Secrets.Headlamp.OIDCClientSecret = ""

	report := ValidateReadiness(cfg)

	assertNoIssue(t, report, "secrets.keycloak.client_secret")
	assertNoIssue(t, report, "secrets.headlamp.oidc_client_secret")
	assertIssue(t, report, SeverityError, CategoryServices, "secrets.keycloak.admin_password")
}

func TestValidateReadinessExternalOIDCRequiresOperatorProvidedClientSecrets(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Identity.OIDC.Enabled = true
	cfg.OpenCenter.Identity.OIDC.Source = OIDCSourceExternal
	cfg.OpenCenter.Identity.OIDC.Provider = OIDCProviderGeneric
	cfg.Secrets.Keycloak.ClientSecret = ""
	cfg.Secrets.Headlamp.OIDCClientSecret = ""

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryServices, "secrets.keycloak.client_secret")
	assertIssue(t, report, SeverityError, CategoryServices, "secrets.headlamp.oidc_client_secret")
	assertNoIssue(t, report, "secrets.keycloak.admin_password")
}

func TestValidateForDeploymentInternalOIDCSkipsBootstrapClientSecretPlaceholders(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Identity.OIDC.Enabled = true
	cfg.OpenCenter.Identity.OIDC.Source = OIDCSourceInternal
	cfg.OpenCenter.Identity.OIDC.Provider = OIDCProviderKeycloak
	cfg.Secrets.Keycloak.ClientSecret = PlaceholderSecret
	cfg.Secrets.Headlamp.OIDCClientSecret = PlaceholderSecret

	if err := ValidateForDeployment(cfg); err != nil {
		t.Fatalf("ValidateForDeployment() returned unexpected error: %v", err)
	}
}

func TestValidateForDeploymentExternalOIDCRequiresClientSecretPlaceholders(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Identity.OIDC.Enabled = true
	cfg.OpenCenter.Identity.OIDC.Source = OIDCSourceExternal
	cfg.OpenCenter.Identity.OIDC.Provider = OIDCProviderGeneric
	cfg.Secrets.Keycloak.ClientSecret = PlaceholderSecret
	cfg.Secrets.Headlamp.OIDCClientSecret = PlaceholderSecret

	err := ValidateForDeployment(cfg)
	if err == nil {
		t.Fatal("expected ValidateForDeployment() to fail for external OIDC client secret placeholders")
	}
	errMsg := err.Error()
	for _, want := range []string{
		"secrets.keycloak.client_secret",
		"secrets.headlamp.oidc_client_secret",
	} {
		if !strings.Contains(errMsg, want) {
			t.Fatalf("expected error to contain %q, got: %v", want, err)
		}
	}
	if strings.Contains(errMsg, "secrets.keycloak.admin_password") {
		t.Fatalf("did not expect admin password placeholder error, got: %v", err)
	}
}

func validReadinessConfig(t *testing.T, provider string) *Config {
	t.Helper()

	cfg, err := NewV2Default("readiness-test", provider)
	if err != nil {
		t.Fatalf("create config: %v", err)
	}

	cfg.Secrets.Keycloak.ClientSecret = "keycloak-client-secret"
	cfg.Secrets.Keycloak.AdminPassword = "keycloak-admin-password"
	cfg.Secrets.Headlamp.OIDCClientSecret = "headlamp-oidc-secret"
	cfg.Secrets.Grafana.AdminPassword = "grafana-admin-password"
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "loki-swift-secret"
	cfg.Secrets.Loki.S3AccessKeyID = "loki-s3-access"
	cfg.Secrets.Loki.S3SecretAccessKey = "loki-s3-secret"
	cfg.OpenCenter.Services["loki"].(*services.LokiConfig).S3Endpoint = "https://loki-s3.example"
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = "tempo-swift-secret"
	cfg.Secrets.Tempo.AccessKey = "tempo-s3-access"
	cfg.Secrets.Tempo.SecretKey = "tempo-s3-secret"
	cfg.OpenCenter.Services["tempo"].(*services.TempoConfig).S3Endpoint = "https://tempo-s3.example"
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).S3Endpoint = "https://harbor-s3.example"

	cfg.OpenCenter.GitOps.Repository.URL = "ssh://git@github.com/example/cluster.git"
	cfg.OpenCenter.GitOps.Auth.Token = nil
	cfg.OpenCenter.GitOps.Auth.SSH = &GitOpsSSHAuth{
		PrivateKey: "secrets/gitops/id_ed25519",
		PublicKey:  "secrets/gitops/id_ed25519.pub",
	}

	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		os := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
		os.ApplicationCredentialID = "app-cred-id"
		os.ApplicationCredentialSecret = "app-cred-secret"
		os.NetworkID = "network-id"
		os.SubnetID = "subnet-id"
		os.FloatingNetworkID = "external-network-id"
		os.RouterExternalNetworkID = "router-external-network-id"
	}

	return cfg
}

func assertIssue(t *testing.T, report ReadinessReport, severity ValidationSeverity, category ValidationCategory, path string) ValidationIssue {
	t.Helper()
	for _, issue := range report.Issues {
		if issue.Severity == severity && issue.Category == category && issue.Path == path {
			return issue
		}
	}
	t.Fatalf("expected %s %s issue at %s, got:\n%s", severity, category, path, renderIssues(report.Issues))
	return ValidationIssue{}
}

func assertNoIssue(t *testing.T, report ReadinessReport, path string) {
	t.Helper()
	for _, issue := range report.Issues {
		if issue.Path == path {
			t.Fatalf("did not expect issue at %s, got:\n%s", path, renderIssues(report.Issues))
		}
	}
}

func renderIssues(issues []ValidationIssue) string {
	var b strings.Builder
	for _, issue := range issues {
		b.WriteString(string(issue.Severity))
		b.WriteString(" ")
		b.WriteString(string(issue.Category))
		b.WriteString(" ")
		b.WriteString(issue.Path)
		b.WriteString(": ")
		b.WriteString(issue.Message)
		b.WriteString("\n")
	}
	return b.String()
}

// --- Baremetal provider validation tests ---

func TestValidateReadinessBaremetalRequiresMasterNodes(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.MasterNodes = nil

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.master_nodes")
}

func TestValidateReadinessBaremetalRequiresWorkerNodes(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Infrastructure.Compute.WorkerNodes = nil

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.worker_nodes")
}

func TestValidateReadinessBaremetalValidatesStaticNodeFields(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.MasterNodes = []StaticNode{
		{Name: "", AccessIPv4: ""},
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.master_nodes[0].name")
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.master_nodes[0].access_ip_v4")
}

func TestValidateReadinessBaremetalCountMismatchWarning(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.MasterNodes = []StaticNode{
		{Name: "master-1", AccessIPv4: "10.0.0.1"},
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityWarning, CategoryProvider, "opencenter.infrastructure.compute.master_count")
}

func TestValidateReadinessBaremetalRejectsCloudSections(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &OpenStackCloudConfig{
		AuthURL: "https://keystone.example.com/v3",
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud")
}

func TestValidateReadinessBaremetalBastionWarning(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Infrastructure.Bastion.Enabled = true

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityWarning, CategoryProvider, "opencenter.infrastructure.bastion.enabled")
}

func TestValidateReadinessBaremetalVIPInterfaceWarning(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = true
	cfg.OpenCenter.Infrastructure.Networking.VIPInterface = ""

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityWarning, CategoryProvider, "opencenter.infrastructure.networking.vip_interface")
}

func TestValidateReadinessBaremetalValidConfigPasses(t *testing.T) {
	cfg := validReadinessConfig(t, "baremetal")
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.MasterNodes = []StaticNode{
		{Name: "master-1", AccessIPv4: "10.0.0.1"},
	}
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 1
	cfg.OpenCenter.Infrastructure.Compute.WorkerNodes = []StaticNode{
		{Name: "worker-1", AccessIPv4: "10.0.0.2"},
	}
	cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{}
	cfg.OpenCenter.Infrastructure.Bastion.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = false

	report := ValidateReadiness(cfg)

	// Should have no provider errors (may have other warnings from services etc.)
	for _, issue := range report.Issues {
		if issue.Severity == SeverityError && issue.Category == CategoryProvider {
			t.Fatalf("unexpected provider error: %s — %s", issue.Path, issue.Message)
		}
	}
}

// --- VMware provider validation tests ---

func TestValidateReadinessVMwareRequiresCloudSection(t *testing.T) {
	cfg := validReadinessConfig(t, "vmware")
	cfg.OpenCenter.Infrastructure.Cloud.VMware = nil

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.vmware")
}

func TestValidateReadinessVMwareRequiresVCenterServer(t *testing.T) {
	cfg := validReadinessConfig(t, "vmware")

	if cfg.OpenCenter.Infrastructure.Cloud.VMware == nil {
		cfg.OpenCenter.Infrastructure.Cloud.VMware = &VMwareCloudConfig{}
	}
	cfg.OpenCenter.Infrastructure.Cloud.VMware.VCenterServer = ""

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.vmware.vcenter_server")
}

func TestValidateReadinessVMwareRejectsOtherCloudSections(t *testing.T) {
	cfg := validReadinessConfig(t, "vmware")
	if cfg.OpenCenter.Infrastructure.Cloud.VMware == nil {
		cfg.OpenCenter.Infrastructure.Cloud.VMware = &VMwareCloudConfig{
			VCenterServer: "vcenter.example.com",
			Datacenter:    "DC1",
			Datastore:     "datastore1",
			Network:       "VM Network",
			Template:      "ubuntu-22.04",
		}
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &OpenStackCloudConfig{
		AuthURL: "https://keystone.example.com/v3",
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud")
}

func TestValidateReadinessVMwareWarnsNoVSphereCSI(t *testing.T) {
	cfg := validReadinessConfig(t, "vmware")
	if cfg.OpenCenter.Infrastructure.Cloud.VMware == nil {
		cfg.OpenCenter.Infrastructure.Cloud.VMware = &VMwareCloudConfig{
			VCenterServer: "vcenter.example.com",
			Datacenter:    "DC1",
			Datastore:     "datastore1",
			Network:       "VM Network",
			Template:      "ubuntu-22.04",
		}
	}
	cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi = nil

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityWarning, CategoryProvider, "opencenter.cluster.kubernetes.storage_plugin.vsphere_csi")
}

// --- Kind provider validation tests ---

func TestValidateReadinessKindRejectsBastionEnabled(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Bastion.Enabled = true

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.bastion.enabled")
}

func TestValidateReadinessKindRejectsKubeVIP(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = true

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.cluster.kubernetes.kube_vip_enabled")
}

func TestValidateReadinessKindRejectsVRRP(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = true

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.networking.vrrp_enabled")
}

func TestValidateReadinessKindWarnsCloudSections(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &OpenStackCloudConfig{
		AuthURL: "https://keystone.example.com/v3",
	}

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityWarning, CategoryProvider, "opencenter.infrastructure.cloud")
}

func TestValidateReadinessKindValidConfigPasses(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Bastion.Enabled = false
	cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled = false
	cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled = false
	cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{}

	report := ValidateReadiness(cfg)

	for _, issue := range report.Issues {
		if issue.Severity == SeverityError && issue.Category == CategoryProvider {
			t.Fatalf("unexpected provider error: %s — %s", issue.Path, issue.Message)
		}
	}
}

// --- AWS provider validation tests ---

func TestValidateReadinessAWSRequiresCloudSection(t *testing.T) {
	cfg := validReadinessConfig(t, "aws")
	cfg.OpenCenter.Infrastructure.Cloud.AWS = nil

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.aws")
}

func TestValidateReadinessAWSRequiresRegion(t *testing.T) {
	cfg := validReadinessConfig(t, "aws")
	if cfg.OpenCenter.Infrastructure.Cloud.AWS == nil {
		cfg.OpenCenter.Infrastructure.Cloud.AWS = &AWSCloudConfig{}
	}
	cfg.OpenCenter.Infrastructure.Cloud.AWS.Region = ""

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.aws.region")
}

// --- GCP provider validation tests ---

func TestValidateReadinessGCPRequiresCloudSection(t *testing.T) {
	cfg := validReadinessConfig(t, "gcp")
	cfg.OpenCenter.Infrastructure.Cloud.GCP = nil

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.gcp")
}

// --- Azure provider validation tests ---

func TestValidateReadinessAzureRequiresCloudSection(t *testing.T) {
	cfg := validReadinessConfig(t, "azure")
	cfg.OpenCenter.Infrastructure.Cloud.Azure = nil

	report := ValidateReadiness(cfg)

	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.azure")
}

func TestValidateReadinessMagnumUsesManagedCloudContract(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Provider = "magnum"
	cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{
		Magnum: &MagnumCloudConfig{
			AuthURL:                     "https://keystone.example.com/v3",
			Region:                      "RegionOne",
			ProjectID:                   "project-id",
			ApplicationCredentialID:     "application-credential-id",
			ApplicationCredentialSecret: "application-credential-secret",
			ClusterTemplate:             "kubernetes-template",
		},
	}

	report := ValidateReadiness(cfg)
	for _, issue := range report.Issues {
		if issue.Severity == SeverityError && issue.Category == CategoryProvider {
			t.Fatalf("unexpected Magnum provider error: %s — %s", issue.Path, issue.Message)
		}
	}
}

func TestValidateReadinessMagnumRequiresCloudAndCredentials(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Provider = "magnum"
	cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{Magnum: &MagnumCloudConfig{}}

	report := ValidateReadiness(cfg)
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.magnum.auth_url")
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.magnum.application_credential_id")
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.magnum.cluster_template")
}

func TestValidateReadinessKeycloakStrictHostnameSchedulingCapacity(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	keycloak := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig)
	keycloak.Instances = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = nil

	report := ValidateReadiness(cfg)
	issue := assertIssue(t, report, SeverityError, CategoryServices, "opencenter.services.keycloak.instances")
	if !strings.Contains(issue.Message, "3") || !strings.Contains(issue.Message, "2") || !strings.Contains(issue.Message, "DoNotSchedule") {
		t.Fatalf("capacity issue = %#v, want replicas, workers, and strict scheduling semantics", issue)
	}
}

func TestValidateReadinessKeycloakCapacityIncludesAdditionalLinuxWorkerPools(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig).Instances = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 2
	cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = []WorkerPoolConfig{{Name: "extra", Count: 1, Flavor: "worker"}}

	report := ValidateReadiness(cfg)
	assertNoIssue(t, report, "opencenter.services.keycloak.instances")
}

func TestValidateReadinessDoesNotApplyStrictCapacityToDisabledKeycloak(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	keycloak := cfg.OpenCenter.Services["keycloak"].(*services.KeycloakConfig)
	keycloak.Enabled = false
	keycloak.Instances = 10
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 1

	report := ValidateReadiness(cfg)
	assertNoIssue(t, report, "opencenter.services.keycloak.instances")
}
