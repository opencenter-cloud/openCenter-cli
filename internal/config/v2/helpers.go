package v2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// ClusterName returns the cluster's canonical name.
func (c Config) ClusterName() string {
	if value := strings.TrimSpace(c.OpenCenter.Cluster.ClusterName); value != "" {
		return value
	}
	return strings.TrimSpace(c.OpenCenter.Meta.Name)
}

// Organization returns the cluster organization.
func (c Config) Organization() string {
	return strings.TrimSpace(c.OpenCenter.Meta.Organization)
}

// Provider returns the normalized infrastructure provider name.
func (c Config) Provider() string {
	return strings.TrimSpace(c.OpenCenter.Infrastructure.Provider)
}

// GitOps returns the GitOps configuration block.
func (c Config) GitOps() GitOpsConfig {
	return c.OpenCenter.GitOps
}

// GetAWSCredentials resolves service credentials with fallback to infrastructure credentials.
func (c Config) GetAWSCredentials(serviceAccessKey, serviceSecretKey string) (accessKey, secretKey string) {
	if serviceAccessKey != "" && serviceSecretKey != "" {
		return serviceAccessKey, serviceSecretKey
	}

	return c.Secrets.Global.AWS.Infrastructure.AccessKey, c.Secrets.Global.AWS.Infrastructure.SecretAccessKey
}

// GetAWSApplicationCredentials resolves application credentials with fallback to
// infrastructure credentials. The init placeholder counts as unset: a fresh
// cluster ships CHANGEME in the application block, and treating it as a real
// key made every consumer that trusts this getter (cert-manager Route53
// resolution included) believe a credential existed.
func (c Config) GetAWSApplicationCredentials() (accessKey, secretKey string) {
	accessKey, secretKey = c.Secrets.Global.AWS.Application.AccessKey, c.Secrets.Global.AWS.Application.SecretAccessKey
	if isMissingSecret(accessKey) || isMissingSecret(secretKey) {
		return c.Secrets.Global.AWS.Infrastructure.AccessKey, c.Secrets.Global.AWS.Infrastructure.SecretAccessKey
	}
	return accessKey, secretKey
}

// certManagerCredentialName is the credential name derived from a DNS provider
// for the single-credential flow: `dns_provider: cloudflare` renders the
// ClusterIssuer named after the DNS zone plus the provider, so the provider
// name is the fallback credential name. A hand-written credential that already
// carries the provider's name keeps its own name instead of being shadowed by a
// duplicate.
func certManagerCredentialName(provider string, names []string) string {
	for _, name := range names {
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(provider)) {
			return name
		}
	}
	return strings.TrimSpace(provider)
}

// sortedCredentialNames returns credential names in the deterministic order the
// renderer and the default issuer use: lexicographic, first name wins.
func sortedCredentialNames[V any](credentials map[string]V) []string {
	names := make([]string, 0, len(credentials))
	for name := range credentials {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolvedCertManagerAWSCredentials returns the Route53 credentials cert-manager
// renders, keyed by credential name. Enabled entries of
// secrets.cert_manager.aws pass through verbatim; when none is enabled and
// services.cert-manager.dns_provider is route53, the legacy flat
// secrets.cert_manager.aws_access_key/aws_secret_access_key pair -- or the AWS
// application credentials -- are surfaced as a single enabled credential named
// after the provider. Without that derivation, setting dns_provider alone
// rendered no ClusterIssuer at all.
func (c Config) ResolvedCertManagerAWSCredentials() map[string]CertManagerAWSCredential {
	result := make(map[string]CertManagerAWSCredential)
	enabled := make([]string, 0, len(c.Secrets.CertManager.AWS))
	for name, cred := range c.Secrets.CertManager.AWS {
		if cred.Enabled {
			result[name] = cred
			enabled = append(enabled, name)
		}
	}
	// dns_provider: none disables ACME, so the legacy flat-credential
	// derivation must not resurrect an issuer the operator just turned off.
	// Explicitly enabled secrets.cert_manager.aws entries still pass through
	// above; validateCertManagerCredentials rejects that contradiction.
	if len(result) > 0 || c.ResolvedDNSProvider() != "route53" {
		return result
	}

	accessKey, secretKey := c.certManagerRoute53Keys()
	if accessKey == "" || secretKey == "" {
		return result
	}
	result[certManagerCredentialName("route53", enabled)] = CertManagerAWSCredential{
		Enabled:            true,
		AWSAccessKey:       accessKey,
		AWSSecretAccessKey: secretKey,
		Region:             c.certManagerRoute53Region(),
	}
	return result
}

// ResolvedCertManagerCloudflareCredentials returns the Cloudflare credentials
// cert-manager renders, keyed by credential name. Enabled entries of
// secrets.cert_manager.cloudflare pass through verbatim; when none is enabled
// and services.cert-manager.dns_provider is cloudflare, the legacy flat
// secrets.cert_manager.cloudflare_api_token is surfaced as a single enabled
// credential named after the provider. Without that derivation, setting
// dns_provider alone rendered no ClusterIssuer at all.
func (c Config) ResolvedCertManagerCloudflareCredentials() map[string]CertManagerCloudflareCredential {
	result := make(map[string]CertManagerCloudflareCredential)
	enabled := make([]string, 0, len(c.Secrets.CertManager.Cloudflare))
	for name, cred := range c.Secrets.CertManager.Cloudflare {
		if cred.Enabled {
			result[name] = cred
			enabled = append(enabled, name)
		}
	}
	if len(result) > 0 || c.ResolvedDNSProvider() != "cloudflare" {
		return result
	}

	token := strings.TrimSpace(c.Secrets.CertManager.CloudflareAPIToken)
	if isMissingSecret(token) {
		return result
	}
	result[certManagerCredentialName("cloudflare", enabled)] = CertManagerCloudflareCredential{
		Enabled:  true,
		APIToken: token,
	}
	return result
}

// ResolvedDNSProvider returns the lower-cased cert-manager DNS provider. An
// explicit value always wins; an empty one falls back to the provider the
// registry auto-selects for the infrastructure, because that is the provider
// the renderers and readiness treat as in force. Returns "" when cert-manager
// is absent or disabled.
//
// The disable sentinel is normalized here: dns_provider: off/disabled/internal
// all collapse to "none", so every consumer can compare against
// services.DNSProviderNone without re-knowing the alias table.
// AutoSelectDNSProvider never yields "none", so the sentinel is strictly
// opt-in.
func (c Config) ResolvedDNSProvider() string {
	certManager, ok := c.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	if !ok || certManager == nil || !certManager.Enabled {
		return ""
	}
	if provider := strings.ToLower(strings.TrimSpace(certManager.DNSProvider)); provider != "" {
		if services.IsDNSProviderNone(provider) {
			return string(services.DNSProviderNone)
		}
		return provider
	}
	return strings.ToLower(string(services.AutoSelectDNSProvider(c.Provider(), c.ClusterName(), c.designateEnabled())))
}

// DNSProviderDisabled reports whether ACME issuance is switched off for this
// cluster, either by dns_provider: none (or an alias) or by cert-manager being
// absent/disabled.
func (c Config) DNSProviderDisabled() bool {
	provider := c.ResolvedDNSProvider()
	return provider == "" || provider == string(services.DNSProviderNone)
}

// ResolvedDNSCredential picks the DNS credential a DNS-consuming service should
// use and returns its name ("" when there is no credential to pick, e.g.
// designate or ACME is disabled).
//
// provider is advisory, not a filter. cert-manager can run several DNS-01
// solvers, and an operator can enable an AWS credential while dns_provider
// auto-selects cloudflare; filtering strictly by provider would find nothing and
// silently drop the Gateway back to the internal CA. So an explicit name is
// honored wherever it lives, then the named provider's credentials are
// preferred, then any other enabled credential.
//
// Precedence, highest first:
//  1. requested  - the per-consumer dns_credential override
//  2. zone match - a credential whose dns_zones cover hostname, when a hostname
//     is supplied. Without this step the resolver can name an issuer whose
//     selector.dnsZones cannot solve for the Certificate's zone: an issuer
//     exists, matches nothing, and the Certificate never becomes Ready.
//  3. services.cert-manager.default_credential - the cluster-wide default
//  4. the sole enabled credential
//  5. the lowest-sorted enabled name
//
// Step 5 preserves the historical behaviour (gatewayDefaultIssuer took the
// lowest-sorted AWS name, else Cloudflare) so existing clusters see no change.
// Steps 1-4 make the choice explicit instead of leaving it to name collation,
// which silently follows a credential rename.
func (c Config) ResolvedDNSCredential(provider, requested, hostname string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == string(services.DNSProviderNone) || provider == string(services.DNSProviderDesignate) {
		return ""
	}

	// Advisory order: the named provider first, then any other. An empty
	// provider (cert-manager disabled) still scans, so an explicitly named
	// credential resolves rather than falling through to the internal CA.
	ordered := []string{"route53", "cloudflare"}
	if provider == "cloudflare" {
		ordered = []string{"cloudflare", "route53"}
	}

	zonesByProvider := map[string]map[string][]string{}
	namesByProvider := map[string][]string{}
	var all []string
	for _, name := range ordered {
		names, zones := c.dnsCredentialIndex(name)
		if len(names) == 0 {
			continue
		}
		namesByProvider[name] = names
		zonesByProvider[name] = zones
		all = append(all, names...)
	}
	if len(all) == 0 {
		return ""
	}

	certManager, _ := c.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig)
	clusterDefault := ""
	if certManager != nil {
		clusterDefault = strings.TrimSpace(certManager.DefaultCredential)
	}

	// 1 and 3: explicit names, searched across providers so a credential that
	// lives under a different provider than the auto-selected one is still
	// selectable.
	for _, candidate := range []string{strings.TrimSpace(requested), clusterDefault} {
		if candidate == "" {
			continue
		}
		if containsName(all, candidate) {
			return candidate
		}
	}

	// 2: zone coverage, preferring the named provider.
	if hostname = strings.ToLower(strings.TrimSpace(hostname)); hostname != "" {
		for _, name := range ordered {
			for _, credential := range namesByProvider[name] {
				if zoneCoversHostname(zonesByProvider[name][credential], hostname) {
					return credential
				}
			}
		}
	}

	// 4 and 5.
	return all[0]
}

// dnsCredentialIndex returns the enabled credential names for a provider in
// sorted order, plus each credential's declared DNS zones.
func (c Config) dnsCredentialIndex(provider string) ([]string, map[string][]string) {
	zones := map[string][]string{}
	var names []string
	switch provider {
	case "route53":
		for name, cred := range c.ResolvedCertManagerAWSCredentials() {
			names = append(names, name)
			zones[name] = cred.DNSZones
		}
	case "cloudflare":
		for name, cred := range c.ResolvedCertManagerCloudflareCredentials() {
			names = append(names, name)
			zones[name] = cred.DNSZones
		}
	default:
		return nil, nil
	}
	sort.Strings(names)
	return names, zones
}

// containsName reports whether names holds name.
func containsName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// zoneCoversHostname reports whether any zone is the hostname itself or a
// parent suffix of it, which is the relation cert-manager's selector.dnsZones
// actually applies.
func zoneCoversHostname(zones []string, hostname string) bool {
	for _, zone := range zones {
		zone = strings.ToLower(strings.TrimSpace(zone))
		if zone == "" {
			continue
		}
		if hostname == zone || strings.HasSuffix(hostname, "."+zone) {
			return true
		}
	}
	return false
}

// designateEnabled reports whether the cluster uses OpenStack Designate, the
// signal the registry needs to choose between designate and cloudflare.
func (c Config) designateEnabled() bool {
	if openstack := c.OpenCenter.Infrastructure.Cloud.OpenStack; openstack != nil && openstack.UseDesignate {
		return true
	}
	return c.OpenCenter.Infrastructure.Networking.UseDesignate
}

// certManagerRoute53Keys resolves the Route53 key pair: the legacy flat
// cert-manager secrets first, then the AWS application credentials.
func (c Config) certManagerRoute53Keys() (accessKey, secretKey string) {
	flatAccess := strings.TrimSpace(c.Secrets.CertManager.AWSAccessKey)
	flatSecret := strings.TrimSpace(c.Secrets.CertManager.AWSSecretAccessKey)
	if !isMissingSecret(flatAccess) && !isMissingSecret(flatSecret) {
		return flatAccess, flatSecret
	}
	accessKey, secretKey = c.GetAWSApplicationCredentials()
	return strings.TrimSpace(accessKey), strings.TrimSpace(secretKey)
}

// certManagerRoute53Region resolves the Route53 region: the cert-manager
// service region, then the AWS application region, then the cluster region.
func (c Config) certManagerRoute53Region() string {
	if certManager, ok := c.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig); ok && certManager != nil {
		if region := strings.TrimSpace(certManager.Region); region != "" {
			return region
		}
	}
	if region := strings.TrimSpace(c.Secrets.Global.AWS.Application.Region); region != "" {
		return region
	}
	return strings.TrimSpace(c.OpenCenter.Meta.Region)
}

// GetLokiS3Credentials resolves Loki S3 credentials.
func (c Config) GetLokiS3Credentials() (accessKey, secretKey string) {
	if c.Secrets.Loki.S3AccessKeyID != "" && c.Secrets.Loki.S3SecretAccessKey != "" {
		return c.Secrets.Loki.S3AccessKeyID, c.Secrets.Loki.S3SecretAccessKey
	}

	return c.GetAWSApplicationCredentials()
}

// GetTempoS3Credentials resolves Tempo S3 credentials.
func (c Config) GetTempoS3Credentials() (accessKey, secretKey string) {
	if c.Secrets.Tempo.AccessKey != "" && c.Secrets.Tempo.SecretKey != "" {
		return c.Secrets.Tempo.AccessKey, c.Secrets.Tempo.SecretKey
	}

	return c.GetAWSApplicationCredentials()
}

// GetHarborS3Credentials resolves Harbor S3 credentials.
func (c Config) GetHarborS3Credentials() (accessKey, secretKey string) {
	if c.Secrets.Harbor.S3AccessKeyID != "" && c.Secrets.Harbor.S3SecretAccessKey != "" {
		return c.Secrets.Harbor.S3AccessKeyID, c.Secrets.Harbor.S3SecretAccessKey
	}

	return c.GetAWSApplicationCredentials()
}

// GetLokiSwiftPassword returns the Loki Swift Keystone password for username/password auth.
func (c Config) GetLokiSwiftPassword() string {
	if value := strings.TrimSpace(c.Secrets.Loki.SwiftPassword); value != "" {
		return value
	}
	if raw, ok := c.Secrets.ServiceSecrets["loki"]; ok {
		if mapped, ok := raw.(map[string]any); ok {
			if value, ok := mapped["swift_password"].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return strings.TrimSpace(c.Secrets.Global.AWS.Application.SecretAccessKey)
}

// GetLokiSwiftApplicationCredentialSecret returns the Loki Swift application credential secret.
//
// Deprecated: Loki's Swift driver uses username/password auth (GetLokiSwiftPassword);
// this getter is retained for backward compatibility and is no longer used by lokiTemplate.
func (c Config) GetLokiSwiftApplicationCredentialSecret() string {
	if value := strings.TrimSpace(c.Secrets.Loki.SwiftApplicationCredentialSecret); value != "" {
		return value
	}
	if raw, ok := c.Secrets.ServiceSecrets["loki"]; ok {
		if mapped, ok := raw.(map[string]any); ok {
			if value, ok := mapped["swift_application_credential_secret"].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
			if value, ok := mapped["swift_password"].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return strings.TrimSpace(c.Secrets.Global.AWS.Application.SecretAccessKey)
}

// ResolveMimirSwiftCredentials resolves the Mimir Swift application credential
// pair atomically. A service-specific ID and secret must be supplied together;
// otherwise both values must come from the global OpenStack pair.
func (c Config) ResolveMimirSwiftCredentials() (string, string, error) {
	serviceID, serviceSecret := "", ""
	if service, ok := c.OpenCenter.Services["mimir"].(*services.MimirConfig); ok && service != nil {
		serviceID = strings.TrimSpace(service.SwiftApplicationCredentialID)
		serviceSecret = strings.TrimSpace(c.Secrets.Mimir.SwiftApplicationCredentialSecret)
	}
	serviceIDSet := credentialOverrideSet(serviceID)
	serviceSecretSet := credentialOverrideSet(serviceSecret)
	if serviceIDSet != serviceSecretSet {
		return "", "", fmt.Errorf("Mimir Swift service-specific application credential ID and secret must be set together")
	}
	if serviceIDSet {
		return serviceID, serviceSecret, nil
	}

	globalID, globalSecret := "", ""
	if openstack := c.OpenCenter.Infrastructure.Cloud.OpenStack; openstack != nil {
		globalID = strings.TrimSpace(openstack.ApplicationCredentialID)
		globalSecret = strings.TrimSpace(openstack.ApplicationCredentialSecret)
	}
	if (globalID == "") != (globalSecret == "") {
		return "", "", fmt.Errorf("global OpenStack application credential ID and secret must be set together")
	}
	if globalID != "" {
		return globalID, globalSecret, nil
	}
	return "", "", nil
}

// ValidateMimirSwiftCredentialPair validates pairing without requiring the
// credentials to be non-placeholder; placeholder checks remain deployment
// readiness concerns.
func (c Config) ValidateMimirSwiftCredentialPair() error {
	_, _, err := c.ResolveMimirSwiftCredentials()
	return err
}

func credentialOverrideSet(value string) bool {
	return strings.TrimSpace(value) != "" && !strings.EqualFold(strings.TrimSpace(value), PlaceholderSecret)
}

// GetMimirSwiftApplicationCredentialSecret returns the resolved Mimir Swift
// secret. Legacy global AWS fallback is retained only when no OpenStack pair or
// service-specific override is present; partial pairs return an empty value so
// CLI validation cannot combine credentials from different scopes.
func (c Config) GetMimirSwiftApplicationCredentialSecret() string {
	if mimirSwiftCredentialPairPartial(c) {
		return ""
	}
	if _, secret, err := c.ResolveMimirSwiftCredentials(); err == nil && secret != "" {
		return secret
	}
	return strings.TrimSpace(c.Secrets.Global.AWS.Application.SecretAccessKey)
}

// GetMimirSwiftApplicationCredentialID returns the resolved Mimir credential ID.
func (c Config) GetMimirSwiftApplicationCredentialID() string {
	if mimirSwiftCredentialPairPartial(c) {
		return ""
	}
	if id, _, err := c.ResolveMimirSwiftCredentials(); err == nil && id != "" {
		return id
	}
	return ""
}

func mimirSwiftCredentialPairPartial(c Config) bool {
	serviceID, serviceSecret := "", ""
	if service, ok := c.OpenCenter.Services["mimir"].(*services.MimirConfig); ok && service != nil {
		serviceID = strings.TrimSpace(service.SwiftApplicationCredentialID)
		serviceSecret = strings.TrimSpace(c.Secrets.Mimir.SwiftApplicationCredentialSecret)
	}
	if credentialOverrideSet(serviceID) != credentialOverrideSet(serviceSecret) {
		return true
	}
	if credentialOverrideSet(serviceID) && credentialOverrideSet(serviceSecret) {
		return false
	}
	openstack := c.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil {
		return false
	}
	globalID := strings.TrimSpace(openstack.ApplicationCredentialID)
	globalSecret := strings.TrimSpace(openstack.ApplicationCredentialSecret)
	return (globalID == "") != (globalSecret == "")
}

// GetTempoSwiftApplicationCredentialSecret returns the Tempo Swift application credential secret.
func (c Config) GetTempoSwiftApplicationCredentialSecret() string {
	if value := strings.TrimSpace(c.Secrets.Tempo.SwiftApplicationCredentialSecret); value != "" {
		return value
	}
	if raw, ok := c.Secrets.ServiceSecrets["tempo"]; ok {
		if mapped, ok := raw.(map[string]any); ok {
			if value, ok := mapped["swift_application_credential_secret"].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
			if value, ok := mapped["swift_password"].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return strings.TrimSpace(c.Secrets.Global.AWS.Application.SecretAccessKey)
}

// GetS3BackendCredentials resolves backend S3 credentials using infrastructure credentials.
func (c Config) GetS3BackendCredentials() (accessKey, secretKey string) {
	return c.GetAWSCredentials("", "")
}

// GetLokiS3AccessKey returns the Loki S3 access key for templates.
func (c Config) GetLokiS3AccessKey() string {
	accessKey, _ := c.GetLokiS3Credentials()
	return accessKey
}

// GetLokiS3SecretKey returns the Loki S3 secret key for templates.
func (c Config) GetLokiS3SecretKey() string {
	_, secretKey := c.GetLokiS3Credentials()
	return secretKey
}

// GetHarborS3AccessKey returns the Harbor S3 access key for templates.
func (c Config) GetHarborS3AccessKey() string {
	accessKey, _ := c.GetHarborS3Credentials()
	return accessKey
}

// GetHarborS3SecretKey returns the Harbor S3 secret key for templates.
func (c Config) GetHarborS3SecretKey() string {
	_, secretKey := c.GetHarborS3Credentials()
	return secretKey
}

// GetTempoS3AccessKey returns the Tempo S3 access key for templates.
func (c Config) GetTempoS3AccessKey() string {
	accessKey, _ := c.GetTempoS3Credentials()
	return accessKey
}

// GetTempoS3SecretKey returns the Tempo S3 secret key for templates.
func (c Config) GetTempoS3SecretKey() string {
	_, secretKey := c.GetTempoS3Credentials()
	return secretKey
}

// GetS3BackendAccessKey returns the backend S3 access key for templates.
func (c Config) GetS3BackendAccessKey() string {
	accessKey, _ := c.GetS3BackendCredentials()
	return accessKey
}

// GetS3BackendSecretKey returns the backend S3 secret key for templates.
func (c Config) GetS3BackendSecretKey() string {
	_, secretKey := c.GetS3BackendCredentials()
	return secretKey
}

// GitDir returns the configured GitOps working directory.
func (c Config) GitDir() string {
	return strings.TrimSpace(c.OpenCenter.GitOps.Repository.LocalDir)
}

// ConfiguredGitURL returns the Git URL only when it has been explicitly set
// to something other than the schema default placeholder.
func (c Config) ConfiguredGitURL() string {
	value := strings.TrimSpace(c.OpenCenter.GitOps.Repository.URL)
	if value == "" || value == defaultGitURLPlaceholder || value == defaultHTTPSGitURLPlaceholder {
		return ""
	}
	return value
}

// GitBranchOrDefault returns the configured Git branch, defaulting to main.
func (c Config) GitBranchOrDefault() string {
	if branch := strings.TrimSpace(c.OpenCenter.GitOps.Repository.Branch); branch != "" {
		return branch
	}
	return "main"
}

// IsKind reports whether the cluster uses the kind provider.
func (c Config) IsKind() bool {
	return strings.EqualFold(c.Provider(), "kind")
}

// KindDisableDefaultCNI reports whether kind should disable its default CNI.
func (c Config) KindDisableDefaultCNI() bool {
	return c.OpenCenter.Infrastructure.Kind != nil && c.OpenCenter.Infrastructure.Kind.DisableDefaultCNI
}

// FluxBootstrapPath returns the repository path where Flux should be bootstrapped.
// Defaults to "applications/overlays/" + clusterName, matching the standard
// overlay layout, unless overridden by opencenter.gitops.repository.path.
func (c Config) FluxBootstrapPath() string {
	if configPath := strings.TrimSpace(c.OpenCenter.GitOps.Repository.Path); configPath != "" {
		return configPath
	}
	clusterName := c.ClusterName()
	if clusterName == "" {
		return ""
	}
	return "applications/overlays/" + clusterName
}

// ShouldRenderFluxBridge reports whether the Flux bridge under clusters/<cluster>/
// is needed. When the Flux bootstrap path is under applications/overlays/<cluster>,
// the root overlay kustomization already references ./services/fluxcd, so the bridge
// is redundant. It is only needed when the repository path is explicitly configured
// to a legacy location under clusters/.
func (c Config) ShouldRenderFluxBridge() bool {
	p := strings.TrimSpace(c.OpenCenter.GitOps.Repository.Path)
	return strings.HasPrefix(p, "clusters/")
}

// ToJSON marshals the public configuration to indented JSON.
func (c Config) ToJSON() ([]byte, error) {
	return MarshalPublicConfigJSON(&c)
}
