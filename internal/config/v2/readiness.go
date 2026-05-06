package v2

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// ValidationSeverity identifies whether a readiness issue blocks deployment.
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
)

// ValidationCategory groups readiness issues by validation subsystem.
type ValidationCategory string

const (
	CategorySchema       ValidationCategory = "schema"
	CategoryProvider     ValidationCategory = "provider"
	CategoryGitOps       ValidationCategory = "gitops"
	CategoryServices     ValidationCategory = "services"
	CategoryConnectivity ValidationCategory = "connectivity"
)

// ValidationIssue is a structured validation finding.
type ValidationIssue struct {
	Severity   ValidationSeverity `json:"severity"`
	Category   ValidationCategory `json:"category"`
	Path       string             `json:"path,omitempty"`
	Message    string             `json:"message"`
	Suggestion string             `json:"suggestion,omitempty"`
}

// ReadinessReport contains deterministic, offline deployment-readiness findings.
type ReadinessReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidateReadiness validates cross-field deployment-readiness rules that are not
// covered by schema tags. It does not contact cloud providers or Git remotes.
func ValidateReadiness(cfg *Config) ReadinessReport {
	r := readinessBuilder{report: ReadinessReport{Valid: true}}
	if cfg == nil {
		r.addError(CategorySchema, "", "configuration is nil", "")
		return r.report
	}

	r.validateProvider(cfg)
	r.validateNetworkPlugin(cfg)
	r.validateGitOps(cfg)
	r.validateServiceSecrets(cfg)

	return r.report
}

type readinessBuilder struct {
	report ReadinessReport
}

func (r *readinessBuilder) addError(category ValidationCategory, path, message, suggestion string) {
	r.addIssue(SeverityError, category, path, message, suggestion)
}

func (r *readinessBuilder) addWarning(category ValidationCategory, path, message, suggestion string) {
	r.addIssue(SeverityWarning, category, path, message, suggestion)
}

func (r *readinessBuilder) addIssue(severity ValidationSeverity, category ValidationCategory, path, message, suggestion string) {
	if severity == SeverityError {
		r.report.Valid = false
	}
	r.report.Issues = append(r.report.Issues, ValidationIssue{
		Severity:   severity,
		Category:   category,
		Path:       path,
		Message:    message,
		Suggestion: suggestion,
	})
}

func (r *readinessBuilder) validateProvider(cfg *Config) {
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	switch provider {
	case "openstack":
		r.validateOpenStackProvider(cfg)
	case "kind", "aws", "gcp", "azure", "baremetal", "vsphere", "vmware":
		return
	case "":
		r.addError(CategoryProvider, "opencenter.infrastructure.provider", "provider is required", "Set infrastructure.provider to a supported provider.")
	default:
		r.addError(CategoryProvider, "opencenter.infrastructure.provider", fmt.Sprintf("unsupported provider %q", provider), "Use one of: openstack, aws, gcp, azure, baremetal, vsphere, vmware, kind.")
	}
}

func (r *readinessBuilder) validateNetworkPlugin(cfg *Config) {
	plugins := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin
	type enabledPlugin struct {
		name   string
		path   string
		method string
	}

	var enabled []enabledPlugin
	if plugins.Calico != nil && plugins.Calico.Enabled {
		enabled = append(enabled, enabledPlugin{
			name:   "calico",
			path:   "opencenter.cluster.kubernetes.network_plugin.calico.install_method",
			method: plugins.Calico.InstallMethod,
		})
	}
	if plugins.Cilium != nil && plugins.Cilium.Enabled {
		enabled = append(enabled, enabledPlugin{
			name:   "cilium",
			path:   "opencenter.cluster.kubernetes.network_plugin.cilium.install_method",
			method: plugins.Cilium.InstallMethod,
		})
	}
	if plugins.KubeOVN != nil && plugins.KubeOVN.Enabled {
		enabled = append(enabled, enabledPlugin{
			name:   "kube-ovn",
			path:   "opencenter.cluster.kubernetes.network_plugin.kube-ovn.install_method",
			method: plugins.KubeOVN.InstallMethod,
		})
	}

	switch len(enabled) {
	case 0:
		r.addError(CategorySchema, "opencenter.cluster.kubernetes.network_plugin", "exactly one network_plugin must be enabled.", "Enable exactly one of calico, cilium, or kube-ovn.")
		return
	case 1:
	default:
		names := make([]string, 0, len(enabled))
		for _, plugin := range enabled {
			names = append(names, plugin.name)
		}
		r.addError(CategorySchema, "opencenter.cluster.kubernetes.network_plugin", fmt.Sprintf("only one network_plugin may be enabled; enabled: %s.", strings.Join(names, ", ")), "Disable all but one of calico, cilium, or kube-ovn.")
		return
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))

	plugin := enabled[0]
	method := strings.ToLower(strings.TrimSpace(plugin.method))
	if method == "" {
		method = "helm"
	}

	if provider != "openstack" {
		return
	}

	switch method {
	case "helm", "kustomize-helm":
		return
	case "kubespray":
		r.addError(CategorySchema, plugin.path, fmt.Sprintf("OpenStack network_plugin %s cannot use install_method %q.", plugin.name, method), "Use install_method: helm or install_method: kustomize-helm.")
	default:
		r.addError(CategorySchema, plugin.path, fmt.Sprintf("unsupported OpenStack network_plugin install_method %q; accepted values are helm and kustomize-helm.", method), "Use install_method: helm or install_method: kustomize-helm.")
	}
}

func (r *readinessBuilder) validateOpenStackProvider(cfg *Config) {
	cloud := cfg.OpenCenter.Infrastructure.Cloud
	os := cloud.OpenStack
	if os == nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack", "openstack provider requires openstack cloud configuration", "Add opencenter.infrastructure.cloud.openstack.")
		return
	}

	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.auth_url", os.AuthURL, "OpenStack auth URL is required.", "Set the Keystone auth_url endpoint.")
	if parsed, err := url.Parse(strings.TrimSpace(os.AuthURL)); strings.TrimSpace(os.AuthURL) != "" && (err != nil || parsed.Scheme == "" || parsed.Host == "") {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.auth_url", "OpenStack auth URL must be a valid absolute URL.", "Use the full Keystone URL, for example https://keystone.example.com/v3.")
	} else if parsed != nil && parsed.Scheme == "http" {
		r.addWarning(CategoryProvider, "opencenter.infrastructure.cloud.openstack.auth_url", "OpenStack auth URL is using plain HTTP.", "Use HTTPS for Keystone endpoints when possible.")
	}
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.region", os.Region, "OpenStack region is required.", "Set the OpenStack region.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.project_id", os.ProjectID, "OpenStack project ID is required.", "Set the OpenStack project ID.")
	r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.cloud.openstack.image_id", os.ImageID, "OpenStack image ID is required.", "Set an image ID that exists in Glance.")

	hasAppCredID := valueSet(os.ApplicationCredentialID)
	hasAppCredSecret := valueSet(os.ApplicationCredentialSecret)
	if hasAppCredID != hasAppCredSecret {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.application_credential_id", "OpenStack application credential ID and secret must be set together.", "Set both application_credential_id and application_credential_secret, or use username/password credentials.")
	}
	if isMissingSecret(os.ApplicationCredentialID) {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.application_credential_id", "OpenStack application credential ID is required for readiness validation.", "Create an OpenStack application credential and set its ID.")
	}
	if isMissingSecret(os.ApplicationCredentialSecret) {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud.openstack.application_credential_secret", "OpenStack application credential secret is required for readiness validation.", "Set the OpenStack application credential secret.")
	}

	compute := cfg.OpenCenter.Infrastructure.Compute
	if compute.MasterCount > 0 {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_master", compute.FlavorMaster, "OpenStack master flavor is required when master_count is greater than zero.", "Set compute.flavor_master.")
	}
	if compute.WorkerCount > 0 {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_worker", compute.FlavorWorker, "OpenStack worker flavor is required when worker_count is greater than zero.", "Set compute.flavor_worker.")
	}
	if compute.WorkerCountWindows > 0 {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_worker_windows", compute.FlavorWorkerWindows, "OpenStack Windows worker flavor is required when worker_count_windows is greater than zero.", "Set compute.flavor_worker_windows.")
	}
	if cfg.OpenCenter.Infrastructure.Bastion.Enabled {
		r.requireNonPlaceholder(CategoryProvider, "opencenter.infrastructure.compute.flavor_bastion", compute.FlavorBastion, "OpenStack bastion flavor is required when bastion is enabled.", "Set compute.flavor_bastion.")
	}
	for i, pool := range compute.AdditionalServerPoolsWorker {
		if pool.Count > 0 {
			r.requireNonPlaceholder(CategoryProvider, fmt.Sprintf("opencenter.infrastructure.compute.additional_server_pools_worker[%d].flavor", i), pool.Flavor, "OpenStack worker pool flavor is required when pool count is greater than zero.", "Set each worker pool flavor.")
		}
	}

	if cloud.AWS != nil || cloud.GCP != nil || cloud.Azure != nil || cloud.VMware != nil {
		r.addError(CategoryProvider, "opencenter.infrastructure.cloud", "inactive provider cloud sections are configured alongside openstack.", "Remove cloud sections for providers that are not active.")
	}
}

func (r *readinessBuilder) validateGitOps(cfg *Config) {
	repoURL := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL)
	if repoURL == "" {
		r.addError(CategoryGitOps, "opencenter.gitops.repository.url", "GitOps repository URL is required.", "Set gitops.repository.url.")
		return
	}

	auth := cfg.OpenCenter.GitOps.Auth
	if auth.SSH != nil && auth.Token != nil {
		r.addError(CategoryGitOps, "opencenter.gitops.auth", "GitOps SSH and token auth are both configured.", "Configure exactly one GitOps auth method.")
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		r.addError(CategoryGitOps, "opencenter.gitops.repository.url", "GitOps repository URL is not valid.", "Use an https:// or ssh:// Git repository URL.")
		return
	}

	switch strings.ToLower(parsed.Scheme) {
	case "https":
		r.validateGitOpsHTTPS(parsed, auth)
	case "ssh":
		r.validateGitOpsSSH(auth)
	default:
		r.addError(CategoryGitOps, "opencenter.gitops.repository.url", "GitOps repository URL must use https or ssh.", "Use https:// for token auth or ssh:// for deploy-key auth.")
	}
}

func (r *readinessBuilder) validateGitOpsHTTPS(parsed *url.URL, auth GitOpsAuth) {
	if auth.Token == nil {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.token", "HTTPS GitOps repository requires token auth.", "Configure gitops.auth.token for HTTPS repositories.")
		return
	}
	if isMissingSecret(auth.Token.Token) && isMissingSecret(auth.Token.TokenFile) {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.token.token", "HTTPS GitOps repository requires a token value or token_file.", "Set gitops.auth.token.token or gitops.auth.token.token_file.")
	}
	expectedProvider := gitProviderForHost(parsed.Hostname())
	if expectedProvider != "" && strings.ToLower(strings.TrimSpace(auth.Token.Provider)) != expectedProvider {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.token.provider", fmt.Sprintf("GitOps token provider must be %q for host %q.", expectedProvider, parsed.Hostname()), "Set gitops.auth.token.provider to match the repository host.")
	}
}

func (r *readinessBuilder) validateGitOpsSSH(auth GitOpsAuth) {
	if auth.SSH == nil {
		r.addError(CategoryGitOps, "opencenter.gitops.auth.ssh", "SSH GitOps repository requires SSH key auth.", "Configure gitops.auth.ssh for SSH repositories.")
		return
	}
	r.requireNonPlaceholder(CategoryGitOps, "opencenter.gitops.auth.ssh.private_key", auth.SSH.PrivateKey, "SSH GitOps repository requires a private key path.", "Set gitops.auth.ssh.private_key.")
	r.requireNonPlaceholder(CategoryGitOps, "opencenter.gitops.auth.ssh.public_key", auth.SSH.PublicKey, "SSH GitOps repository requires a public key path.", "Set gitops.auth.ssh.public_key.")
}

func gitProviderForHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		return "github"
	case host == "gitlab.com" || strings.Contains(host, "gitlab"):
		return "gitlab"
	case host != "":
		return "gitea"
	default:
		return ""
	}
}

func (r *readinessBuilder) validateServiceSecrets(cfg *Config) {
	if serviceEnabled(cfg, "keycloak") {
		if !oidcClientSecretsProvidedInternally(cfg) {
			r.requireSecret("secrets.keycloak.client_secret", cfg.Secrets.Keycloak.ClientSecret, "Keycloak client secret is required when keycloak is enabled.")
		}
		r.requireSecret("secrets.keycloak.admin_password", cfg.Secrets.Keycloak.AdminPassword, "Keycloak admin password is required when keycloak is enabled.")
	}
	if serviceEnabled(cfg, "headlamp") && headlampUsesOIDC(cfg) && !oidcClientSecretsProvidedInternally(cfg) {
		r.requireSecret("secrets.headlamp.oidc_client_secret", cfg.Secrets.Headlamp.OIDCClientSecret, "Headlamp OIDC client secret is required when Headlamp OIDC is enabled.")
	}
	if serviceEnabled(cfg, "kube-prometheus-stack") {
		r.requireSecret("secrets.grafana.admin_password", cfg.Secrets.Grafana.AdminPassword, "Grafana admin password is required when kube-prometheus-stack is enabled.")
	}
	r.validateCertManagerSecrets(cfg)
	r.validateLokiSecrets(cfg)
	r.validateTempoSecrets(cfg)
	if serviceEnabled(cfg, "weave-gitops") {
		if isMissingSecret(cfg.Secrets.WeaveGitOps.Password) && isMissingSecret(cfg.Secrets.WeaveGitOps.PasswordHash) {
			r.addError(CategoryServices, "secrets.weave_gitops.password", "Weave GitOps requires password or password_hash when enabled.", "Set secrets.weave_gitops.password_hash or secrets.weave_gitops.password.")
		}
	}
	if serviceEnabled(cfg, "alert-proxy") {
		r.requireSecret("secrets.alert_proxy.core_device_id", cfg.Secrets.AlertProxy.CoreDeviceId, "Alert proxy core device ID is required when alert-proxy is enabled.")
		r.requireSecret("secrets.alert_proxy.account_service_token", cfg.Secrets.AlertProxy.AccountServiceToken, "Alert proxy account service token is required when alert-proxy is enabled.")
		r.requireSecret("secrets.alert_proxy.core_account_number", cfg.Secrets.AlertProxy.CoreAccountNumber, "Alert proxy core account number is required when alert-proxy is enabled.")
	}
	if vsphereCSIEnabled(cfg) {
		r.requireSecret("secrets.vsphere_csi.vcenter_host", cfg.Secrets.VSphereCsi.VCenterHost, "vSphere CSI vCenter host is required when vSphere CSI is enabled.")
		r.requireSecret("secrets.vsphere_csi.username", cfg.Secrets.VSphereCsi.Username, "vSphere CSI username is required when vSphere CSI is enabled.")
		r.requireSecret("secrets.vsphere_csi.password", cfg.Secrets.VSphereCsi.Password, "vSphere CSI password is required when vSphere CSI is enabled.")
	}
}

func (r *readinessBuilder) validateCertManagerSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "cert-manager") {
		return
	}
	certManager, _ := cfg.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	if certManager == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(certManager.DNSProvider)) {
	case "route53":
		r.requireSecret("secrets.cert_manager.aws_access_key", cfg.Secrets.CertManager.AWSAccessKey, "cert-manager Route53 DNS requires AWS access key.")
		r.requireSecret("secrets.cert_manager.aws_secret_access_key", cfg.Secrets.CertManager.AWSSecretAccessKey, "cert-manager Route53 DNS requires AWS secret access key.")
	case "cloudflare":
		r.requireSecret("secrets.cert_manager.cloudflare_api_token", cfg.Secrets.CertManager.CloudflareAPIToken, "cert-manager Cloudflare DNS requires API token.")
	}
}

func (r *readinessBuilder) validateLokiSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "loki") {
		return
	}
	loki, _ := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	storageType := ""
	if loki != nil {
		storageType = strings.ToLower(strings.TrimSpace(firstNonEmptyReadiness(loki.StorageType, loki.LokiStorageType)))
	}
	if storageType == "" && strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "openstack") {
		storageType = "swift"
	}
	switch storageType {
	case "swift":
		r.requireSecret("secrets.loki.swift_application_credential_secret", cfg.Secrets.Loki.SwiftApplicationCredentialSecret, "Loki Swift storage requires an application credential secret.")
	case "s3":
		r.requireSecret("secrets.loki.s3_access_key_id", cfg.Secrets.Loki.S3AccessKeyID, "Loki S3 storage requires an access key ID.")
		r.requireSecret("secrets.loki.s3_secret_access_key", cfg.Secrets.Loki.S3SecretAccessKey, "Loki S3 storage requires a secret access key.")
	}
}

func (r *readinessBuilder) validateTempoSecrets(cfg *Config) {
	if !serviceEnabled(cfg, "tempo") {
		return
	}
	tempo, _ := cfg.OpenCenter.Services["tempo"].(*services.TempoConfig)
	storageType := ""
	if tempo != nil {
		storageType = strings.ToLower(strings.TrimSpace(tempo.StorageType))
	}
	if storageType == "" && strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "openstack") {
		storageType = "swift"
	}
	switch storageType {
	case "swift":
		r.requireSecret("secrets.tempo.swift_application_credential_secret", cfg.Secrets.Tempo.SwiftApplicationCredentialSecret, "Tempo Swift storage requires an application credential secret.")
	case "s3":
		r.requireSecret("secrets.tempo.access_key", cfg.Secrets.Tempo.AccessKey, "Tempo S3 storage requires an access key.")
		r.requireSecret("secrets.tempo.secret_key", cfg.Secrets.Tempo.SecretKey, "Tempo S3 storage requires a secret key.")
	}
}

func (r *readinessBuilder) requireSecret(path, value, message string) {
	r.requireNonPlaceholder(CategoryServices, path, value, message, "Set a non-placeholder secret value.")
}

func (r *readinessBuilder) requireNonPlaceholder(category ValidationCategory, path, value, message, suggestion string) {
	if isMissingSecret(value) {
		r.addError(category, path, message, suggestion)
	}
}

func serviceEnabled(cfg *Config, serviceName string) bool {
	return serviceEnabledInMap(cfg.OpenCenter.Services, serviceName) || serviceEnabledInMap(cfg.OpenCenter.ManagedServices, serviceName)
}

func serviceEnabledInMap(servicesMap ServiceMap, serviceName string) bool {
	if svc, ok := servicesMap[serviceName]; ok {
		if enabler, ok := svc.(interface{ IsEnabled() bool }); ok {
			return enabler.IsEnabled()
		}
	}
	return false
}

func headlampUsesOIDC(cfg *Config) bool {
	if cfg.OpenCenter.Identity.OIDC.Enabled {
		return true
	}
	if cfg.OpenCenter.Cluster.Kubernetes.OIDC.Enabled {
		return true
	}
	headlamp, _ := cfg.OpenCenter.Services["headlamp"].(*services.HeadlampConfig)
	if headlamp == nil {
		return false
	}
	return strings.TrimSpace(headlamp.OIDCIssuerURL) != "" || strings.TrimSpace(headlamp.OIDCClientID) != "" || serviceEnabled(cfg, "keycloak")
}

func oidcClientSecretsProvidedInternally(cfg *Config) bool {
	oidc := cfg.OpenCenter.Identity.OIDC
	if !oidc.Enabled {
		return false
	}
	source := strings.ToLower(strings.TrimSpace(oidc.Source))
	if source == "" {
		source = OIDCSourceInternal
	}
	provider := strings.ToLower(strings.TrimSpace(oidc.Provider))
	if provider == "" {
		provider = OIDCProviderKeycloak
	}
	return source == OIDCSourceInternal && provider == OIDCProviderKeycloak && serviceEnabled(cfg, "keycloak")
}

func vsphereCSIEnabled(cfg *Config) bool {
	if serviceEnabled(cfg, "vsphere-csi") {
		return true
	}
	plugin := cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi
	return plugin != nil && plugin.Enabled
}

func isMissingSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || strings.EqualFold(trimmed, PlaceholderSecret)
}

func valueSet(value string) bool {
	return !isMissingSecret(value)
}

func firstNonEmptyReadiness(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
