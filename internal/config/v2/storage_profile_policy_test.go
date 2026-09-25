package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestNewV2DefaultUsesExplicitExternalStorageProfile(t *testing.T) {
	cfg, err := NewV2Default("storage-profile-default", "kind")
	if err != nil {
		t.Fatalf("NewV2Default: %v", err)
	}
	profile := cfg.OpenCenter.Infrastructure.Storage.Profile
	if profile.Lifecycle != StorageLifecycleNonProduction || profile.PVCProvider != StoragePVCProviderExternal || profile.ObjectStorageProvider != StorageObjectProviderExternalS3 {
		t.Fatalf("unexpected default storage profile: %#v", profile)
	}
	if backend := ResolveObjectStorageBackend(cfg, "loki"); backend != "s3" {
		t.Fatalf("object storage backend = %q, want s3", backend)
	}
}

func TestStorageProfilePolicy(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*Config)
		issuePaths []string
	}{
		{
			name: "production requires external S3",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderRustFS}
				cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true
			},
			issuePaths: []string{"opencenter.infrastructure.storage.profile.object_storage_provider"},
		},
		{
			name: "RustFS requires Longhorn provider and service",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleNonProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderRustFS}
			},
			issuePaths: []string{"opencenter.infrastructure.storage.profile.pvc_provider", "opencenter.services.longhorn"},
		},
		{
			name: "external S3 requires an endpoint for every enabled consumer",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3}
				velero := cfg.OpenCenter.Services["velero"].(*services.VeleroConfig)
				velero.Enabled = true
				velero.S3Endpoint = ""
			},
			issuePaths: []string{"opencenter.services.velero.s3_endpoint"},
		},
		{
			name: "typed Mimir S3 is valid in production",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3}
				mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
				mimir.Enabled = true
				mimir.StorageType = "s3"
				mimir.S3Endpoint = "https://mimir-s3.example"
				cfg.Secrets.Mimir.S3AccessKeyID = "mimir-access"
				cfg.Secrets.Mimir.S3SecretAccessKey = "mimir-secret"
			},
			issuePaths: nil,
		},
		{
			name: "Mimir S3 rejects a non-root endpoint path",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderExternal, ObjectStorageProvider: StorageObjectProviderExternalS3}
				mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
				mimir.Enabled = true
				mimir.StorageType = "s3"
				mimir.S3Endpoint = "https://seaweedfs.example:8333/s3"
				cfg.Secrets.Mimir.S3AccessKeyID = "mimir-access"
				cfg.Secrets.Mimir.S3SecretAccessKey = "mimir-secret"
			},
			issuePaths: []string{"opencenter.services.mimir.s3_endpoint"},
		},
		{
			name: "Swift migration is explicit",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Services["loki"].(*services.LokiConfig).StorageType = "swift"
			},
			issuePaths: []string{"opencenter.services.loki.storage_type"},
		},
		{
			name: "managed RustFS still requires effective Mimir S3 inputs",
			configure: func(cfg *Config) {
				cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleNonProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderRustFS}
				cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true
				cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).Enabled = true
				mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
				mimir.Enabled = true
				mimir.StorageType = "s3"
				mimir.S3Endpoint = ""
				cfg.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig).Enabled = true
				cfg.Secrets.Loki.S3AccessKeyID = ""
				cfg.Secrets.Loki.S3SecretAccessKey = ""
				cfg.Secrets.Tempo.AccessKey = ""
				cfg.Secrets.Tempo.SecretKey = ""
				cfg.Secrets.Harbor.S3AccessKeyID = ""
				cfg.Secrets.Harbor.S3SecretAccessKey = ""
				cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = ""
				cfg.Secrets.Mimir.S3AccessKeyID = ""
				cfg.Secrets.Mimir.S3SecretAccessKey = ""
				cfg.Secrets.EtcdBackup.AccessKeyID = ""
				cfg.Secrets.EtcdBackup.SecretAccessKey = ""
			},
			issuePaths: []string{
				"opencenter.services.mimir.s3_endpoint",
				"secrets.mimir.s3_access_key_id",
				"secrets.mimir.s3_secret_access_key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validReadinessConfig(t, "kind")
			tt.configure(cfg)
			report := ValidateReadiness(cfg)
			for _, path := range tt.issuePaths {
				assertIssue(t, report, SeverityError, CategoryServices, path)
			}
			if len(tt.issuePaths) == 0 {
				for _, issue := range report.Issues {
					if strings.Contains(issue.Path, "s3_") || strings.Contains(issue.Path, "swift_") {
						t.Fatalf("managed RustFS profile must not require external object-storage field %q: %s", issue.Path, issue.Message)
					}
				}
			}
		})
	}
}

func TestStorageProfilePolicyAppliesToDeploymentValidation(t *testing.T) {
	cfg := validReadinessConfig(t, "kind")
	cfg.OpenCenter.Infrastructure.Storage.Profile = StorageProfileConfig{Lifecycle: StorageLifecycleProduction, PVCProvider: StoragePVCProviderLonghorn, ObjectStorageProvider: StorageObjectProviderRustFS}
	cfg.OpenCenter.Services["longhorn"].(*services.LonghornConfig).Enabled = true

	err := ValidateForDeployment(cfg)
	if err == nil || !strings.Contains(err.Error(), "RustFS is non-production only") {
		t.Fatalf("expected production RustFS policy error, got %v", err)
	}
}
