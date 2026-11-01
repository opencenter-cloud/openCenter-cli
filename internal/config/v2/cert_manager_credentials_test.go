package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

func dnsProviderConfig(provider string) Config {
	return Config{
		OpenCenter: OpenCenterConfig{
			Meta: MetaConfig{Region: "DFW3"},
			Cluster: ClusterConfig{
				ClusterName: "ed-2",
				BaseDomain:  "example.com",
				ClusterFQDN: "ed-2.dfw3.example.com",
			},
			Services: ServiceMap{
				"cert-manager": &services.CertManagerConfig{
					BaseConfig:  services.BaseConfig{Enabled: true},
					DNSProvider: provider,
				},
			},
		},
	}
}

// dns_provider alone must yield a credential, otherwise no ClusterIssuer is
// rendered and the provider silently does nothing.
func TestResolvedCertManagerCloudflareCredentialsFromFlatToken(t *testing.T) {
	cfg := dnsProviderConfig("cloudflare")
	cfg.Secrets.CertManager.CloudflareAPIToken = "flat-token"

	credentials := cfg.ResolvedCertManagerCloudflareCredentials()
	require.Equal(t, map[string]CertManagerCloudflareCredential{
		"cloudflare": {Enabled: true, APIToken: "flat-token"},
	}, credentials)
	require.Empty(t, cfg.ResolvedCertManagerAWSCredentials())
}

func TestResolvedCertManagerAWSCredentialsFromApplicationCredentials(t *testing.T) {
	cfg := dnsProviderConfig("route53")
	cfg.Secrets.Global.AWS.Application = AWSScopedSecrets{
		AccessKey:       "AKIAAPP",
		SecretAccessKey: "app-secret",
		Region:          "us-east-2",
	}

	credentials := cfg.ResolvedCertManagerAWSCredentials()
	require.Equal(t, map[string]CertManagerAWSCredential{
		"route53": {Enabled: true, AWSAccessKey: "AKIAAPP", AWSSecretAccessKey: "app-secret", Region: "us-east-2"},
	}, credentials)
	require.Empty(t, cfg.ResolvedCertManagerCloudflareCredentials())
}

// The init placeholder is not a credential: resolving it would render an issuer
// whose keys are literally CHANGEME.
func TestResolvedCredentialsIgnorePlaceholders(t *testing.T) {
	cfg := dnsProviderConfig("cloudflare")
	cfg.Secrets.CertManager.CloudflareAPIToken = PlaceholderSecret
	cfg.Secrets.Global.AWS.Application = AWSScopedSecrets{
		AccessKey:       PlaceholderSecret,
		SecretAccessKey: PlaceholderSecret,
	}

	require.Empty(t, cfg.ResolvedCertManagerCloudflareCredentials())
	require.Empty(t, cfg.ResolvedCertManagerAWSCredentials())
}

// An enabled named credential wins over the flat-field derivation and keeps its
// own name, so hand-written multi-credential configs render unchanged.
func TestResolvedCredentialsPreferEnabledNamedEntries(t *testing.T) {
	cfg := dnsProviderConfig("cloudflare")
	cfg.Secrets.CertManager.CloudflareAPIToken = "flat-token"
	cfg.Secrets.CertManager.Cloudflare = map[string]CertManagerCloudflareCredential{
		"prod":    {Enabled: true, APIToken: "prod-token", DNSZones: []string{"example.com"}},
		"staging": {Enabled: false, APIToken: "staging-token"},
	}

	require.Equal(t, map[string]CertManagerCloudflareCredential{
		"prod": {Enabled: true, APIToken: "prod-token", DNSZones: []string{"example.com"}},
	}, cfg.ResolvedCertManagerCloudflareCredentials())
}

// A named credential that carries the provider's name is reused instead of
// duplicated, so dns_provider: route53 does not add a second identical issuer.
func TestResolvedCredentialsReuseProviderNamedCredential(t *testing.T) {
	cfg := dnsProviderConfig("route53")
	cfg.Secrets.CertManager.AWS = map[string]CertManagerAWSCredential{
		"route53": {Enabled: false, AWSAccessKey: "AKIANAMED", AWSSecretAccessKey: "named-secret", Region: "us-west-1"},
	}
	cfg.Secrets.Global.AWS.Application = AWSScopedSecrets{AccessKey: "AKIAAPP", SecretAccessKey: "app-secret"}

	credentials := cfg.ResolvedCertManagerAWSCredentials()
	require.Len(t, credentials, 1)
	require.Equal(t, "AKIAAPP", credentials["route53"].AWSAccessKey)
}

func TestResolvedDNSProviderFallsBackToInfrastructureDefault(t *testing.T) {
	// Cloudflare is the default for every infrastructure provider; Designate
	// remains the one infrastructure-derived override.
	cfg := dnsProviderConfig("")
	cfg.OpenCenter.Infrastructure.Provider = "aws"
	require.Equal(t, "cloudflare", cfg.ResolvedDNSProvider())

	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	require.Equal(t, "cloudflare", cfg.ResolvedDNSProvider())

	cfg.OpenCenter.Infrastructure.Networking.UseDesignate = true
	require.Equal(t, "designate", cfg.ResolvedDNSProvider())

	// An explicit value always wins.
	cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{
		BaseConfig:  services.BaseConfig{Enabled: true},
		DNSProvider: "CloudFlare",
	}
	require.Equal(t, "cloudflare", cfg.ResolvedDNSProvider())

	// A disabled service has no provider, so the gateway keeps rackspace-ca.
	cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{BaseConfig: services.BaseConfig{Enabled: false}}
	require.Equal(t, "", cfg.ResolvedDNSProvider())
}

// Readiness must accept either storage shape and reject a provider with no
// credential at all.
func TestReadinessCertManagerCredentials(t *testing.T) {
	paths := func(cfg *Config) []string {
		report := ValidateReadiness(cfg)
		var paths []string
		for _, issue := range report.Issues {
			if issue.Severity == SeverityError {
				paths = append(paths, issue.Path)
			}
		}
		return paths
	}
	certManagerPaths := func(cfg *Config) []string {
		var found []string
		for _, path := range paths(cfg) {
			if strings.HasPrefix(path, "secrets.cert_manager") {
				found = append(found, path)
			}
		}
		return found
	}

	// Named credential only: ready.
	named := dnsProviderConfig("cloudflare")
	named.Secrets.CertManager.Cloudflare = map[string]CertManagerCloudflareCredential{
		"cloudflare": {Enabled: true, APIToken: "tok"},
	}
	require.Empty(t, certManagerPaths(&named))

	// Legacy flat token only: ready.
	flat := dnsProviderConfig("cloudflare")
	flat.Secrets.CertManager.CloudflareAPIToken = "tok"
	require.Empty(t, certManagerPaths(&flat))

	// No credential at all: reported against the map that would fix it.
	none := dnsProviderConfig("cloudflare")
	require.Equal(t, []string{"secrets.cert_manager.cloudflare"}, certManagerPaths(&none))
}
