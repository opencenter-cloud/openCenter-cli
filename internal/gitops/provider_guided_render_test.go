package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestRenderClusterAppsCertManagerCloudflare(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("cloudflare-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "cloudflare-guided.sjc3.k8s.opencenter.cloud"
	cfg.Secrets.CertManager.CloudflareAPIToken = "cf-token"

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.DNSProvider = "cloudflare"
	certManager.Email = "ops@example.com"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")

	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	if !strings.Contains(kustomization, "./opencenter-cloudflare-credentials-secret.yaml") {
		t.Fatalf("expected Cloudflare secret in kustomization:\n%s", kustomization)
	}
	if strings.Contains(kustomization, "./opencenter-aws-credentials-secret.yaml") {
		t.Fatalf("did not expect Route53 AWS secret in Cloudflare kustomization:\n%s", kustomization)
	}

	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-issuer.yaml"))
	if !strings.Contains(issuer, "cloudflare:") {
		t.Fatalf("expected Cloudflare solver in issuer:\n%s", issuer)
	}
	if !strings.Contains(issuer, "apiTokenSecretRef") {
		t.Fatalf("expected Cloudflare token secret ref in issuer:\n%s", issuer)
	}

	secret := mustReadFile(t, filepath.Join(base, "opencenter-cloudflare-credentials-secret.yaml"))
	if !strings.Contains(secret, "api-token: cf-token") {
		t.Fatalf("expected Cloudflare API token in secret:\n%s", secret)
	}
}

func TestRenderClusterAppsCertManagerDesignate(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("designate-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.ClusterFQDN = "designate-guided.sjc3.k8s.opencenter.cloud"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.api.example.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "sjc3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectID = "project-123"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectName = "project-name"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = "rackspace"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-cred-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-cred-secret"

	certManager := cfg.OpenCenter.Services["cert-manager"].(*configservices.CertManagerConfig)
	certManager.DNSProvider = "designate"
	certManager.Email = "ops@example.com"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	base := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")

	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	if !strings.Contains(kustomization, "./opencenter-openstack-designate-credentials-secret.yaml") {
		t.Fatalf("expected Designate secret in kustomization:\n%s", kustomization)
	}

	issuer := mustReadFile(t, filepath.Join(base, "letsencrypt-issuer.yaml"))
	if !strings.Contains(issuer, "webhook:") {
		t.Fatalf("expected webhook solver in issuer:\n%s", issuer)
	}
	if !strings.Contains(issuer, "groupName: acme.syseleven.de") {
		t.Fatalf("expected Designate webhook group name in issuer:\n%s", issuer)
	}
	if !strings.Contains(issuer, "solverName: designatedns") {
		t.Fatalf("expected Designate webhook solver name in issuer:\n%s", issuer)
	}

	secret := mustReadFile(t, filepath.Join(base, "opencenter-openstack-designate-credentials-secret.yaml"))
	if !strings.Contains(secret, "OS_APPLICATION_CREDENTIAL_ID: app-cred-id") {
		t.Fatalf("expected Designate application credential id in secret:\n%s", secret)
	}
	if !strings.Contains(secret, "OS_APPLICATION_CREDENTIAL_SECRET: app-cred-secret") {
		t.Fatalf("expected Designate application credential secret in secret:\n%s", secret)
	}
}

func TestRenderClusterAppsLokiSwift(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("loki-swift-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Services["loki"] = &configservices.LokiConfig{
		BaseConfig:                   configservices.BaseConfig{Enabled: true},
		StorageType:                  "swift",
		BucketName:                   "loki-container",
		SwiftAuthURL:                 "https://identity.api.example.com/v3",
		SwiftRegion:                  "SJC3",
		SwiftAuthVersion:             3,
		SwiftApplicationCredentialID: "app-cred-id",
		SwiftContainerName:           "loki-container",
		SwiftUserDomainName:          "rackspace",
		SwiftDomainName:              "rackspace",
	}
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "swift-secret"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	overrideValues := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "loki", "helm-values", "override-values.yaml"))
	if !strings.Contains(overrideValues, "object_store: swift") {
		t.Fatalf("expected swift object store in Loki values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "type: swift") {
		t.Fatalf("expected swift storage type in Loki values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "application_credential_secret: swift-secret") {
		t.Fatalf("expected swift application credential secret in Loki values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "container_name: loki-container") {
		t.Fatalf("expected swift container name in Loki values:\n%s", overrideValues)
	}
}

func TestRenderClusterAppsTempoSwift(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("tempo-swift-guided")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Services["tempo"] = &configservices.TempoConfig{
		BaseConfig:                   configservices.BaseConfig{Enabled: true},
		StorageType:                  "swift",
		BucketName:                   "tempo-container",
		SwiftAuthURL:                 "https://identity.api.example.com/v3",
		SwiftRegion:                  "SJC3",
		SwiftAuthVersion:             3,
		SwiftApplicationCredentialID: "app-cred-id",
		SwiftContainerName:           "tempo-container",
		SwiftUserDomainName:          "rackspace",
		SwiftDomainName:              "rackspace",
	}
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = "swift-secret"

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	overrideValues := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "tempo", "helm-values", "override-values.yaml"))
	if !strings.Contains(overrideValues, "backend: swift") {
		t.Fatalf("expected swift backend in Tempo values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "application_credential_secret: swift-secret") {
		t.Fatalf("expected swift application credential secret in Tempo values:\n%s", overrideValues)
	}
	if !strings.Contains(overrideValues, "container_name: tempo-container") {
		t.Fatalf("expected swift container name in Tempo values:\n%s", overrideValues)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
