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
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.cloud.openstack.network_id")
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.flavor_worker")
	assertIssue(t, report, SeverityError, CategoryProvider, "opencenter.infrastructure.compute.additional_server_pools_worker[0].flavor")
	if report.Valid {
		t.Fatalf("expected readiness validation to fail, got valid report: %#v", report)
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
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = "tempo-swift-secret"
	cfg.Secrets.Tempo.AccessKey = "tempo-s3-access"
	cfg.Secrets.Tempo.SecretKey = "tempo-s3-secret"

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

func assertIssue(t *testing.T, report ReadinessReport, severity ValidationSeverity, category ValidationCategory, path string) {
	t.Helper()
	for _, issue := range report.Issues {
		if issue.Severity == severity && issue.Category == category && issue.Path == path {
			return
		}
	}
	t.Fatalf("expected %s %s issue at %s, got:\n%s", severity, category, path, renderIssues(report.Issues))
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
