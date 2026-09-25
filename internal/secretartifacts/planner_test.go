package secretartifacts

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

func TestPlanRoutesAndNormalizesArtifacts(t *testing.T) {
	cfg := &v2.Config{
		Secrets: v2.SecretsConfig{
			Grafana: v2.GrafanaSecrets{AdminPassword: " grafana-password "},
			ServiceSecrets: map[string]any{
				"my_service": map[string]any{"token": "token", "empty": "  "},
			},
		},
	}

	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	require.Len(t, artifacts, 2)

	var grafana, arbitrary Artifact
	for _, artifact := range artifacts {
		switch artifact.LogicalService {
		case "grafana":
			grafana = artifact
		case "my-service":
			arbitrary = artifact
		}
	}
	require.Equal(t, "kube-prometheus-stack", grafana.TargetService)
	require.Equal(t, "services/kube-prometheus-stack/secret.yaml", grafana.Path)
	require.Equal(t, " grafana-password ", grafana.Payload["admin_password"])
	require.Equal(t, "my-service", arbitrary.TargetService)
	require.Equal(t, "services/my-service/secret.yaml", arbitrary.Path)
	require.NotContains(t, arbitrary.Payload, "empty")
}

func TestPlanRejectsUnsafeServiceNames(t *testing.T) {
	_, err := Plan(&v2.Config{Secrets: v2.SecretsConfig{
		ServiceSecrets: map[string]any{"../outside": map[string]any{"token": "value"}},
	}})
	require.Error(t, err)
}

func TestPlanDefaultGrafanaSecretContainsUserAndPassword(t *testing.T) {
	cfg, err := v2.NewV2Default("grafana-defaults", "kind")
	require.NoError(t, err)

	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	var grafana *Artifact
	for i := range artifacts {
		if artifacts[i].LogicalService == "grafana" {
			grafana = &artifacts[i]
			break
		}
	}
	require.NotNil(t, grafana)
	require.Equal(t, "admin", grafana.Payload["admin_user"])
	require.Contains(t, grafana.Payload, "admin_password")
	require.NotEmpty(t, grafana.Payload["admin_password"])
}

func TestPlanIncludesEtcdBackupAndVeleroWorkloadSecrets(t *testing.T) {
	cfg := &v2.Config{
		OpenCenter: v2.OpenCenterConfig{Services: map[string]any{
			"etcd-backup": &services.EtcdBackupConfig{BaseConfig: services.BaseConfig{Enabled: true}, S3Endpoint: "https://s3.example/v1", S3BucketName: "etcd-backups", S3Region: "RegionOne"},
			"velero":      &services.VeleroConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		}},
		Secrets: v2.SecretsConfig{
			EtcdBackup: v2.EtcdBackupSecrets{AccessKeyID: "etcd-access", SecretAccessKey: "etcd-secret"},
			Velero:     v2.VeleroSecrets{AccessKeyID: "velero-access", SecretAccessKey: "velero-secret"},
		},
	}
	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	byService := map[string]Artifact{}
	for _, artifact := range artifacts {
		byService[artifact.TargetService] = artifact
	}
	etcd := byService["etcd-backup"]
	require.Equal(t, "etcd-access", etcd.Payload["ACCESS_KEY"])
	require.Equal(t, "https://s3.example/v1", etcd.Payload["S3_HOST"])
	require.Equal(t, "RegionOne", etcd.Payload["S3_REGION"])
	require.Equal(t, "etcd-backups", etcd.Payload["S3_BUCKET_NAME"])
	require.NotContains(t, etcd.Payload, "S3CredentialID")
	velero := byService["velero"]
	require.Equal(t, "[default]\naws_access_key_id=velero-access\naws_secret_access_key=velero-secret\n", velero.Payload["cloud"])
}

func TestPlanIncludesOnlyApplicableMimirS3Credentials(t *testing.T) {
	cfg := &v2.Config{
		OpenCenter: v2.OpenCenterConfig{Services: map[string]any{
			"mimir": &services.MimirConfig{BaseConfig: services.BaseConfig{Enabled: true}, StorageType: "s3"},
		}},
		Secrets: v2.SecretsConfig{Mimir: v2.MimirSecrets{
			SwiftApplicationCredentialSecret: "swift-secret",
			S3AccessKeyID:                    `access: [mimir-prod]`,
			S3SecretAccessKey:                `secret: "quoted # value"`,
		}},
	}

	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	require.Equal(t, "mimir", artifacts[0].TargetService)
	require.Equal(t, `access: [mimir-prod]`, artifacts[0].Payload["s3-access-key-id"])
	require.Equal(t, `secret: "quoted # value"`, artifacts[0].Payload["s3-secret-access-key"])
	require.NotContains(t, artifacts[0].Payload, "swift-application-credential-secret")
}

func TestPlanIncludesOnlyApplicableMimirSwiftCredential(t *testing.T) {
	cfg := &v2.Config{
		OpenCenter: v2.OpenCenterConfig{Services: map[string]any{
			"mimir": &services.MimirConfig{BaseConfig: services.BaseConfig{Enabled: true}, StorageType: "swift", SwiftApplicationCredentialID: "mimir-swift-id"},
		}},
		Secrets: v2.SecretsConfig{Mimir: v2.MimirSecrets{
			SwiftApplicationCredentialSecret: "swift-secret",
			S3AccessKeyID:                    "mimir-access",
			S3SecretAccessKey:                "mimir-secret",
		}},
	}

	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	require.Equal(t, "mimir", artifacts[0].TargetService)
	require.Equal(t, "swift-secret", artifacts[0].Payload["swift-application-credential-secret"])
	require.NotContains(t, artifacts[0].Payload, "s3-access-key-id")
	require.NotContains(t, artifacts[0].Payload, "s3-secret-access-key")
}

func TestPlanOmitsMimirSwiftArtifactForPartialCredentialOverrides(t *testing.T) {
	for _, test := range []struct {
		name       string
		serviceID  string
		serviceKey string
	}{
		{name: "service ID only", serviceID: "mimir-swift-id"},
		{name: "service secret only", serviceKey: "mimir-swift-secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := &v2.Config{
				OpenCenter: v2.OpenCenterConfig{Services: map[string]any{
					"mimir": &services.MimirConfig{
						BaseConfig:                   services.BaseConfig{Enabled: true},
						StorageType:                  "swift",
						SwiftApplicationCredentialID: test.serviceID,
					},
				}},
				Secrets: v2.SecretsConfig{Mimir: v2.MimirSecrets{
					SwiftApplicationCredentialSecret: test.serviceKey,
				}},
			}
			artifacts, err := Plan(cfg)
			require.NoError(t, err)
			require.Empty(t, artifacts)
		})
	}
}

func TestPlanOmitsNoneStorageArtifacts(t *testing.T) {
	cfg := &v2.Config{
		OpenCenter: v2.OpenCenterConfig{Services: map[string]any{
			"loki":        &services.LokiConfig{BaseConfig: services.BaseConfig{Enabled: true}, StorageType: "none"},
			"etcd-backup": &services.EtcdBackupConfig{BaseConfig: services.BaseConfig{Enabled: true}, StorageType: "none"},
			"velero":      &services.VeleroConfig{BaseConfig: services.BaseConfig{Enabled: true}, StorageType: "none"},
		}},
		Secrets: v2.SecretsConfig{
			Loki:       v2.LokiSecrets{S3AccessKeyID: "loki-access", S3SecretAccessKey: "loki-secret"},
			EtcdBackup: v2.EtcdBackupSecrets{AccessKeyID: "etcd-access", SecretAccessKey: "etcd-secret"},
			Velero:     v2.VeleroSecrets{AccessKeyID: "velero-access", SecretAccessKey: "velero-secret"},
		},
	}

	artifacts, err := Plan(cfg)
	require.NoError(t, err)
	for _, artifact := range artifacts {
		require.NotContains(t, []string{"loki", "velero"}, artifact.TargetService)
	}
}

// TestPlanCertManagerExcludesNestedCredentialMaps verifies the cert-manager
// secret.yaml payload never contains the nested AWS/Cloudflare credential maps.
// Those are rendered as flat per-credential Secrets by the cert-manager renderer;
// dumping the map into stringData produced a nested value that Kubernetes rejects
// (stringData must be flat strings), failing the override dry-run — even when the
// credential is disabled.
func TestPlanCertManagerExcludesNestedCredentialMaps(t *testing.T) {
	cfg := &v2.Config{
		Secrets: v2.SecretsConfig{
			CertManager: v2.CertManagerSecrets{
				// A disabled multi-credential map entry — must NOT leak into secret.yaml.
				AWS: map[string]v2.CertManagerAWSCredential{
					"default": {
						Enabled:            false,
						AWSAccessKey:       "AKIAEXAMPLE",
						AWSSecretAccessKey: "secretexample",
						Region:             "us-east-1",
					},
				},
				// A flat legacy value that IS valid flat stringData.
				CloudflareAPIToken: "cf-token",
			},
		},
	}

	artifacts, err := Plan(cfg)
	require.NoError(t, err)

	var certManager *Artifact
	for i := range artifacts {
		if artifacts[i].TargetService == "cert-manager" {
			certManager = &artifacts[i]
			break
		}
	}
	require.NotNil(t, certManager, "cert-manager artifact should exist for the flat legacy token")

	// The nested map key must be absent; only flat strings allowed.
	require.NotContains(t, certManager.Payload, "aws",
		"cert-manager secret.yaml must not contain the nested aws credential map")
	require.NotContains(t, certManager.Payload, "cloudflare",
		"cert-manager secret.yaml must not contain the nested cloudflare credential map")
	require.Equal(t, "cf-token", certManager.Payload["cloudflare_api_token"])
	for key, value := range certManager.Payload {
		_, isString := value.(string)
		require.Truef(t, isString, "cert-manager stringData value for %q must be a flat string, got %T", key, value)
	}
}
