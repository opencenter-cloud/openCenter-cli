package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestHarborSecretCatalogRoundTrip(t *testing.T) {
	cfg := &v2.Config{}
	entry, err := findConfigSecretEntry("harbor-credentials")
	if err != nil {
		t.Fatalf("findConfigSecretEntry() error = %v", err)
	}

	payload := []byte("admin_password: 'admin:# password'\nregistry_password: 'registry:password'\ndatabase_password: 'database-password'\ns3_access_key_id: 'access-key'\ns3_secret_access_key: 'secret-key'\n")
	if err := entry.Set(cfg, payload); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !entry.Present(cfg) {
		t.Fatal("Harbor catalog entry is not present after Set()")
	}

	encoded, err := marshalConfigSecretPayload(entry, cfg)
	if err != nil {
		t.Fatalf("marshalConfigSecretPayload() error = %v", err)
	}
	if !strings.Contains(string(encoded), "admin_password") || !strings.Contains(string(encoded), "registry_password") || !strings.Contains(string(encoded), "database_password") || !strings.Contains(string(encoded), "s3_access_key_id") || !strings.Contains(string(encoded), "s3_secret_access_key") {
		t.Fatalf("encoded Harbor payload = %q", encoded)
	}

	var listed bytes.Buffer
	if err := listConfigMappedSecrets(&listed, cfg, "table"); err != nil {
		t.Fatalf("listConfigMappedSecrets() error = %v", err)
	}
	if !strings.Contains(listed.String(), "harbor-credentials") {
		t.Fatalf("secret list = %q, missing Harbor catalog entry", listed.String())
	}

	entry.Delete(cfg)
	if entry.Present(cfg) {
		t.Fatal("Harbor catalog entry is present after Delete()")
	}
}

func TestClusterServiceHarborSecretOptions(t *testing.T) {
	secrets := getServiceSecrets("harbor")
	if len(secrets) != 5 {
		t.Fatalf("Harbor secret options = %#v, want five options", secrets)
	}
	for _, want := range []string{"admin_password", "registry_password", "database_password", "s3_access_key_id", "s3_secret_access_key"} {
		found := false
		for _, option := range secrets {
			if option.Name == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Harbor secret options = %#v, missing %q", secrets, want)
		}
	}
	for _, option := range secrets {
		if (option.Name == "s3_access_key_id" || option.Name == "s3_secret_access_key") && option.Required {
			t.Fatalf("Harbor S3 secret option %q is universally required; filesystem storage must not require it", option.Name)
		}
	}

	cfg := &v2.SecretsConfig{}
	if err := processSecrets([]string{
		"admin_password=admin",
		"registry_password=registry",
		"database_password=database",
		"s3_access_key_id=access-key",
		"s3_secret_access_key=secret-key",
	}, "harbor", cfg); err != nil {
		t.Fatalf("processSecrets() error = %v", err)
	}
	if cfg.Harbor.AdminPassword != "admin" || cfg.Harbor.RegistryPassword != "registry" || cfg.Harbor.DatabasePassword != "database" || cfg.Harbor.S3AccessKeyID != "access-key" || cfg.Harbor.S3SecretAccessKey != "secret-key" {
		t.Fatalf("processed Harbor secrets = %#v", cfg.Harbor)
	}
}

func TestHarborSecretCatalogPartialRotationPreservesOmittedFields(t *testing.T) {
	entry, err := findConfigSecretEntry("harbor-credentials")
	if err != nil {
		t.Fatalf("findConfigSecretEntry() error = %v", err)
	}
	cfg := &v2.Config{}
	cfg.Secrets.Harbor = v2.HarborSecrets{
		AdminPassword:     "admin-original",
		RegistryPassword:  "registry-original",
		DatabasePassword:  "database-original",
		S3AccessKeyID:     "access-original",
		S3SecretAccessKey: "secret-original",
	}

	if err := entry.Set(cfg, []byte("registry_password: registry-rotated\n")); err != nil {
		t.Fatalf("Set() partial rotation error = %v", err)
	}
	want := v2.HarborSecrets{
		AdminPassword:     "admin-original",
		RegistryPassword:  "registry-rotated",
		DatabasePassword:  "database-original",
		S3AccessKeyID:     "access-original",
		S3SecretAccessKey: "secret-original",
	}
	if cfg.Secrets.Harbor != want {
		t.Fatalf("Harbor secrets after partial rotation = %#v, want %#v", cfg.Secrets.Harbor, want)
	}
}

func TestHarborSecretCatalogRejectsInvalidPatchWithoutMutation(t *testing.T) {
	entry, err := findConfigSecretEntry("harbor-credentials")
	if err != nil {
		t.Fatalf("findConfigSecretEntry() error = %v", err)
	}
	original := v2.HarborSecrets{
		AdminPassword:     "admin-original",
		RegistryPassword:  "registry-original",
		DatabasePassword:  "database-original",
		S3AccessKeyID:     "access-original",
		S3SecretAccessKey: "secret-original",
	}

	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{name: "unknown field", payload: "admin_pasword: rotated\n", wantErr: "field admin_pasword not found"},
		{name: "malformed YAML", payload: "admin_password: [unterminated\n", wantErr: "failed to parse Harbor credentials payload"},
		{name: "empty value", payload: "registry_password: ''\n", wantErr: "secrets.harbor.registry_password"},
		{name: "placeholder value", payload: "database_password: changeme\n", wantErr: "secrets.harbor.database_password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &v2.Config{}
			cfg.Secrets.Harbor = original
			err := entry.Set(cfg, []byte(tt.payload))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Set() error = %v, want error containing %q", err, tt.wantErr)
			}
			if cfg.Secrets.Harbor != original {
				t.Fatalf("Set() mutated Harbor secrets on rejection: got %#v, want %#v", cfg.Secrets.Harbor, original)
			}
		})
	}
}

func TestClusterServiceHarborRejectsPartialS3Credentials(t *testing.T) {
	secrets := &v2.SecretsConfig{}
	secrets.Harbor.S3AccessKeyID = "access-only"
	if err := validateService("harbor", &services.HarborConfig{}, secrets); err == nil || !strings.Contains(err.Error(), "both Harbor S3") {
		t.Fatalf("validateService() error = %v, want partial Harbor S3 credential rejection", err)
	}
}

func TestClusterServiceHarborFilesystemNeedsNoS3Credentials(t *testing.T) {
	secrets := &v2.SecretsConfig{}
	harbor := &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}, StorageType: "filesystem", RegistryVolumeSize: 10, JobserviceVolumeSize: 10, DatabaseVolumeSize: 10, RedisVolumeSize: 10, TrivyVolumeSize: 10}
	if err := validateService("harbor", harbor, secrets); err != nil {
		t.Fatalf("validateService() error = %v, want filesystem Harbor without S3 credentials to be valid", err)
	}
}
