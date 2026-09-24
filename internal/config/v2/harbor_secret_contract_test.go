package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"gopkg.in/yaml.v3"
)

func TestNewV2DefaultHarborSecretsUseDeterministicPlaceholders(t *testing.T) {
	first, err := NewV2Default("harbor-defaults", "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	second, err := NewV2Default("harbor-defaults", "kind")
	if err != nil {
		t.Fatalf("second NewV2Default() error = %v", err)
	}

	if first.Secrets.Harbor != (HarborSecrets{
		AdminPassword:     PlaceholderSecret,
		RegistryPassword:  PlaceholderSecret,
		DatabasePassword:  PlaceholderSecret,
		S3AccessKeyID:     PlaceholderSecret,
		S3SecretAccessKey: PlaceholderSecret,
	}) {
		t.Fatalf("Harbor defaults = %#v, want all CHANGEME", first.Secrets.Harbor)
	}
	if first.Secrets.Harbor != second.Secrets.Harbor {
		t.Fatalf("Harbor defaults are not deterministic: %#v != %#v", first.Secrets.Harbor, second.Secrets.Harbor)
	}
}

func TestHarborStorageValidationRejectsUnsupportedStorageAndEndpoint(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	harbor.Enabled = true
	harbor.StorageType = "filesystem"
	if err := NewValidator().Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v, want non-production filesystem to be valid", err)
	}
	cfg.OpenCenter.Infrastructure.Storage.Profile.Lifecycle = StorageLifecycleProduction
	if err := NewValidator().Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v, want production filesystem to be valid", err)
	}

	harbor.StorageType = "s3"
	harbor.S3Endpoint = "https://swift.example/v1/AUTH_project"
	if err := ValidateHarborConfig(harbor); err == nil || !strings.Contains(err.Error(), "Swift") {
		t.Fatalf("ValidateHarborConfig() error = %v, want Swift endpoint rejection", err)
	}
}

func TestHarborStorageValidationRejectsNonPositiveExplicitPVCSize(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*services.HarborConfig)
	}{
		{name: "registry", set: func(cfg *services.HarborConfig) { cfg.RegistryVolumeSize = -1 }},
		{name: "jobservice", set: func(cfg *services.HarborConfig) { cfg.JobserviceVolumeSize = -1 }},
		{name: "database", set: func(cfg *services.HarborConfig) { cfg.DatabaseVolumeSize = -1 }},
		{name: "redis", set: func(cfg *services.HarborConfig) { cfg.RedisVolumeSize = -1 }},
		{name: "trivy", set: func(cfg *services.HarborConfig) { cfg.TrivyVolumeSize = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReadinessConfig(t, "kind")
			harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
			harbor.RegistryVolumeSize = 100
			harbor.JobserviceVolumeSize = 5
			harbor.DatabaseVolumeSize = 10
			harbor.RedisVolumeSize = 5
			harbor.TrivyVolumeSize = 5
			tc.set(harbor)
			if err := ValidateHarborConfig(harbor); err == nil || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("ValidateHarborConfig() error = %v, want %s size rejection", err, tc.name)
			}
		})
	}
}

func TestHarborYAMLDefaultsPreserveExplicitNonPositiveValuesForValidation(t *testing.T) {
	var omitted services.HarborConfig
	if err := yaml.Unmarshal([]byte("storage_type: s3\n"), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.RegistryVolumeSize != 100 || omitted.JobserviceVolumeSize != 10 || omitted.DatabaseVolumeSize != 10 || omitted.RedisVolumeSize != 10 || omitted.TrivyVolumeSize != 10 {
		t.Fatalf("omitted Harbor PVC defaults = %#v", omitted)
	}

	var explicit services.HarborConfig
	if err := yaml.Unmarshal([]byte("registry_volume_size: 0\n"), &explicit); err != nil {
		t.Fatal(err)
	}
	if explicit.RegistryVolumeSize != 0 {
		t.Fatalf("explicit zero was defaulted: %#v", explicit)
	}
	if err := ValidateHarborConfig(&explicit); err == nil || !strings.Contains(err.Error(), "registry") {
		t.Fatalf("ValidateHarborConfig() error = %v, want explicit zero rejection", err)
	}
}

func TestHarborSecretValidationDependsOnServiceEnabled(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	harbor.Enabled = true
	cfg.Secrets.Harbor = HarborSecrets{
		AdminPassword:     PlaceholderSecret,
		RegistryPassword:  "",
		DatabasePassword:  PlaceholderSecret,
		S3AccessKeyID:     PlaceholderSecret,
		S3SecretAccessKey: PlaceholderSecret,
	}

	report := ValidateReadiness(cfg)
	for _, path := range []string{
		"secrets.harbor.admin_password",
		"secrets.harbor.registry_password",
		"secrets.harbor.database_password",
		"secrets.harbor.s3_access_key_id",
		"secrets.harbor.s3_secret_access_key",
	} {
		assertIssue(t, report, SeverityError, CategoryServices, path)
	}

	err := ValidateForDeployment(cfg)
	if err == nil {
		t.Fatal("ValidateForDeployment() accepted enabled Harbor with missing secrets")
	}
	for _, path := range []string{
		"secrets.harbor.admin_password",
		"secrets.harbor.registry_password",
		"secrets.harbor.database_password",
		"secrets.harbor.s3_access_key_id",
		"secrets.harbor.s3_secret_access_key",
	} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("ValidateForDeployment() error = %v, missing %q", err, path)
		}
	}

	harbor.Enabled = false
	report = ValidateReadiness(cfg)
	for _, path := range []string{
		"secrets.harbor.admin_password",
		"secrets.harbor.registry_password",
		"secrets.harbor.database_password",
		"secrets.harbor.s3_access_key_id",
		"secrets.harbor.s3_secret_access_key",
	} {
		assertNoIssue(t, report, path)
	}
	if err := ValidateForDeployment(cfg); err != nil {
		t.Fatalf("ValidateForDeployment() rejected disabled Harbor placeholders: %v", err)
	}
}

func TestValidateHarborForDeploymentChecksRegularHarborSecrets(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["harbor"] = &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}}
	cfg.Secrets.Harbor = HarborSecrets{
		AdminPassword:     PlaceholderSecret,
		RegistryPassword:  "registry-configured",
		DatabasePassword:  "database-configured",
		S3AccessKeyID:     "ec2-access-key",
		S3SecretAccessKey: "ec2-secret-key",
	}

	err := ValidateHarborForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "secrets.harbor.admin_password") {
		t.Fatalf("ValidateHarborForDeployment() error = %v", err)
	}

	cfg.Secrets.Harbor.AdminPassword = "admin-configured"
	if err := ValidateHarborForDeployment(cfg); err != nil {
		t.Fatalf("ValidateHarborForDeployment() rejected configured regular Harbor: %v", err)
	}
}

func TestValidateHarborForDeploymentRejectsManagedHarbor(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = false
	cfg.OpenCenter.ManagedServices = ServiceMap{
		"harbor": &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	}
	cfg.Secrets.Harbor = HarborSecrets{
		AdminPassword:     "admin-configured",
		RegistryPassword:  "registry-configured",
		DatabasePassword:  "database-configured",
		S3AccessKeyID:     "ec2-access-key",
		S3SecretAccessKey: "ec2-secret-key",
	}

	err := ValidateHarborForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "managed Harbor is not supported") {
		t.Fatalf("ValidateHarborForDeployment() error = %v", err)
	}
}
