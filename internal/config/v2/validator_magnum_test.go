package v2

import (
	"strings"
	"testing"
)

func validMagnumV2Config() *Config {
	cfg := newValidV2TestConfig("magnum")
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
	return cfg
}

func TestDefaultValidatorMagnumMinimalConfigPasses(t *testing.T) {
	if err := NewValidator().Validate(validMagnumV2Config()); err != nil {
		t.Fatalf("valid minimal Magnum configuration failed validation: %v", err)
	}
}

func TestDefaultValidatorMagnumRequiresCloudConfiguration(t *testing.T) {
	cfg := validMagnumV2Config()
	cfg.OpenCenter.Infrastructure.Cloud = CloudConfig{}

	err := NewValidator().Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "cloud.magnum") {
		t.Fatalf("expected missing cloud.magnum validation error, got %v", err)
	}
}

func TestDefaultValidatorMagnumRequiresTemplateAndCredentials(t *testing.T) {
	cfg := validMagnumV2Config()
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ClusterTemplate = ""
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.ApplicationCredentialSecret = ""

	err := NewValidator().Validate(cfg)
	if err == nil || (!strings.Contains(err.Error(), "ApplicationCredential") && !strings.Contains(err.Error(), "ClusterTemplate")) {
		t.Fatalf("expected Magnum credential or template validation error, got %v", err)
	}
}
