package v2

import (
	"fmt"
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

// GetAWSApplicationCredentials resolves application credentials with fallback to infrastructure credentials.
func (c Config) GetAWSApplicationCredentials() (accessKey, secretKey string) {
	if c.Secrets.Global.AWS.Application.AccessKey != "" && c.Secrets.Global.AWS.Application.SecretAccessKey != "" {
		return c.Secrets.Global.AWS.Application.AccessKey, c.Secrets.Global.AWS.Application.SecretAccessKey
	}

	return c.Secrets.Global.AWS.Infrastructure.AccessKey, c.Secrets.Global.AWS.Infrastructure.SecretAccessKey
}

// GetCertManagerAWSCredentials resolves cert-manager Route53 credentials.
// Deprecated: Use EnabledCertManagerAWSCredentials() for multi-credential support.
func (c Config) GetCertManagerAWSCredentials() (accessKey, secretKey string) {
	if c.Secrets.CertManager.AWSAccessKey != "" && c.Secrets.CertManager.AWSSecretAccessKey != "" {
		return c.Secrets.CertManager.AWSAccessKey, c.Secrets.CertManager.AWSSecretAccessKey
	}
	return "", ""
}

// EnabledCertManagerAWSCredentials returns all enabled AWS credentials for cert-manager,
// keyed by their configured name.
func (c Config) EnabledCertManagerAWSCredentials() map[string]CertManagerAWSCredential {
	result := make(map[string]CertManagerAWSCredential)
	for name, cred := range c.Secrets.CertManager.AWS {
		if cred.Enabled {
			result[name] = cred
		}
	}
	return result
}

// EnabledCertManagerCloudflareCredentials returns all enabled Cloudflare credentials for cert-manager,
// keyed by their configured name.
func (c Config) EnabledCertManagerCloudflareCredentials() map[string]CertManagerCloudflareCredential {
	result := make(map[string]CertManagerCloudflareCredential)
	for name, cred := range c.Secrets.CertManager.Cloudflare {
		if cred.Enabled {
			result[name] = cred
		}
	}
	return result
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

// GetCertManagerAWSAccessKey returns the cert-manager AWS access key for templates.
// Deprecated: Use EnabledCertManagerAWSCredentials() for multi-credential support.
func (c Config) GetCertManagerAWSAccessKey() string {
	accessKey, _ := c.GetCertManagerAWSCredentials()
	return accessKey
}

// GetCertManagerAWSSecretKey returns the cert-manager AWS secret key for templates.
// Deprecated: Use EnabledCertManagerAWSCredentials() for multi-credential support.
func (c Config) GetCertManagerAWSSecretKey() string {
	_, secretKey := c.GetCertManagerAWSCredentials()
	return secretKey
}

// GetCertManagerCloudflareAPIToken returns the Cloudflare API token for cert-manager.
// Deprecated: Use EnabledCertManagerCloudflareCredentials() for multi-credential support.
func (c Config) GetCertManagerCloudflareAPIToken() string {
	if strings.TrimSpace(c.Secrets.CertManager.CloudflareAPIToken) != "" {
		return strings.TrimSpace(c.Secrets.CertManager.CloudflareAPIToken)
	}
	return ""
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

// ToJSON marshals the public configuration to indented JSON.
func (c Config) ToJSON() ([]byte, error) {
	return MarshalPublicConfigJSON(&c)
}
