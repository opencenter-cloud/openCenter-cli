package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestValidateForDeploymentRejectsMimirSwiftCredentialPlaceholder(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.Enabled = true
	mimir.StorageType = "swift"
	cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = PlaceholderSecret

	err := ValidateForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "resolved Swift storage backend") {
		t.Fatalf("ValidateForDeployment() error = %v, want non-OpenStack Swift rejection", err)
	}
}

func TestValidateForDeploymentRejectsMimirS3CredentialPlaceholders(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.Enabled = true
	mimir.StorageType = "s3"
	cfg.Secrets.Mimir.S3AccessKeyID = PlaceholderSecret
	cfg.Secrets.Mimir.S3SecretAccessKey = "mimir-secret"

	err := ValidateForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "secrets.mimir.s3_access_key_id") {
		t.Fatalf("ValidateForDeployment() error = %v, want Mimir S3 access key path", err)
	}
}

func TestSwiftApplicationCredentialAccessorsFallBackToGlobalApplicationSecret(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.Secrets.Global.AWS.Application.SecretAccessKey = "global-app-secret"
	cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = ""
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = ""
	cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = ""

	for name, got := range map[string]string{
		"Mimir": cfg.GetMimirSwiftApplicationCredentialSecret(),
		"Loki":  cfg.GetLokiSwiftApplicationCredentialSecret(),
		"Tempo": cfg.GetTempoSwiftApplicationCredentialSecret(),
	} {
		if got != "global-app-secret" {
			t.Errorf("Get%sSwiftApplicationCredentialSecret() = %q, want global-app-secret", name, got)
		}
	}
}

func TestValidateForDeploymentRejectsHarborS3CredentialPlaceholders(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = true
	cfg.Secrets.Harbor = HarborSecrets{
		AdminPassword:     "harbor-admin",
		RegistryPassword:  "harbor-registry",
		DatabasePassword:  "harbor-database",
		S3AccessKeyID:     PlaceholderSecret,
		S3SecretAccessKey: PlaceholderSecret,
	}

	err := ValidateForDeployment(cfg)
	if err == nil {
		t.Fatal("ValidateForDeployment() accepted enabled Harbor with placeholder S3 credentials")
	}
	for _, path := range []string{
		"secrets.harbor.s3_access_key_id",
		"secrets.harbor.s3_secret_access_key",
	} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("ValidateForDeployment() error = %v, missing %q", err, path)
		}
	}
}
