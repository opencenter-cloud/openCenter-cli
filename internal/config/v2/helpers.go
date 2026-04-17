package v2

import (
	"encoding/json"
	"strings"
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
func (c Config) GetCertManagerAWSCredentials() (accessKey, secretKey string) {
	if c.Secrets.CertManager.AWSAccessKey != "" && c.Secrets.CertManager.AWSSecretAccessKey != "" {
		return c.Secrets.CertManager.AWSAccessKey, c.Secrets.CertManager.AWSSecretAccessKey
	}

	return c.GetAWSApplicationCredentials()
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

// GetLokiSwiftApplicationCredentialSecret returns the Loki Swift application credential secret.
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
	return ""
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
	return ""
}

// GetS3BackendCredentials resolves backend S3 credentials using infrastructure credentials.
func (c Config) GetS3BackendCredentials() (accessKey, secretKey string) {
	return c.GetAWSCredentials("", "")
}

// GetCertManagerAWSAccessKey returns the cert-manager AWS access key for templates.
func (c Config) GetCertManagerAWSAccessKey() string {
	accessKey, _ := c.GetCertManagerAWSCredentials()
	return accessKey
}

// GetCertManagerAWSSecretKey returns the cert-manager AWS secret key for templates.
func (c Config) GetCertManagerAWSSecretKey() string {
	_, secretKey := c.GetCertManagerAWSCredentials()
	return secretKey
}

// GetCertManagerCloudflareAPIToken returns the Cloudflare API token for cert-manager.
func (c Config) GetCertManagerCloudflareAPIToken() string {
	if strings.TrimSpace(c.Secrets.CertManager.CloudflareAPIToken) != "" {
		return strings.TrimSpace(c.Secrets.CertManager.CloudflareAPIToken)
	}
	if raw, ok := c.Secrets.ServiceSecrets["cert_manager"]; ok {
		if mapped, ok := raw.(map[string]any); ok {
			if token, ok := mapped["cloudflare_api_token"].(string); ok {
				return strings.TrimSpace(token)
			}
		}
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
	return strings.TrimSpace(c.OpenCenter.GitOps.GitDir)
}

// ConfiguredGitURL returns the Git URL only when it has been explicitly set
// to something other than the schema default placeholder.
func (c Config) ConfiguredGitURL() string {
	value := strings.TrimSpace(c.OpenCenter.GitOps.GitURL)
	if value == "" || value == defaultGitURLPlaceholder {
		return ""
	}
	return value
}

// GitBranchOrDefault returns the configured Git branch, defaulting to main.
func (c Config) GitBranchOrDefault() string {
	if branch := strings.TrimSpace(c.OpenCenter.GitOps.GitBranch); branch != "" {
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

// ToJSON marshals the configuration to indented JSON.
func (c Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
