package plugins

import (
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
	"github.com/stretchr/testify/assert"
)

// TestServiceStatusReporting tests that service status reporting provides accurate information
// This validates Task 3.3 acceptance criteria: "Service status reporting provides accurate information"
func TestServiceStatusReporting(t *testing.T) {
	tests := []struct {
		name           string
		plugin         svc.ServicePlugin
		config         interface{}
		expectedState  string
		expectedMsg    string
		shouldHaveDetails bool
	}{
		{
			name:   "Disabled service returns disabled status",
			plugin: NewVeleroPlugin(),
			config: &services.VeleroConfig{
				BaseConfig: services.BaseConfig{
					Enabled: false,
				},
			},
			expectedState: "disabled",
			expectedMsg:   "Service is disabled",
			shouldHaveDetails: false,
		},
		{
			name:   "Enabled service with no status returns pending",
			plugin: NewVeleroPlugin(),
			config: &services.VeleroConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
					Status:  "", // Empty status
				},
				BackupBucket: "my-bucket",
			},
			expectedState: "pending",
			expectedMsg:   "Velero",
			shouldHaveDetails: true,
		},
		{
			name:   "Enabled service with running status returns running",
			plugin: NewVeleroPlugin(),
			config: &services.VeleroConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
					Status:  "running",
				},
				BackupBucket: "my-bucket",
			},
			expectedState: "running",
			expectedMsg:   "Velero",
			shouldHaveDetails: true,
		},
		{
			name:   "Enabled service with success status returns success",
			plugin: NewPrometheusStackPlugin(),
			config: &services.PrometheusStackConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
					Status:  "success",
				},
			},
			expectedState: "success",
			expectedMsg:   "Prometheus",
			shouldHaveDetails: true,
		},
		{
			name:   "Enabled service with failed status returns failed",
			plugin: NewLokiPlugin(),
			config: &services.LokiConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
					Status:  "failed",
				},
			},
			expectedState: "failed",
			expectedMsg:   "Loki",
			shouldHaveDetails: true,
		},
		{
			name:   "Invalid config type returns failed status",
			plugin: NewVeleroPlugin(),
			config: &services.LokiConfig{}, // Wrong config type
			expectedState: "failed",
			expectedMsg:   "Invalid configuration type",
			shouldHaveDetails: false,
		},
		{
			name:   "CertManager with pending status",
			plugin: NewCertManagerPlugin(),
			config: &services.CertManagerConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
				},
				LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
				Email:             "admin@example.com",
			},
			expectedState: "pending",
			expectedMsg:   "Cert-manager",
			shouldHaveDetails: true,
		},
		{
			name:   "Keycloak with running status and details",
			plugin: NewKeycloakPlugin(),
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
					Status:  "running",
				},
				Realm:       "myrealm",
				FrontendURL: "https://keycloak.example.com",
				ClientID:    "myclient",
			},
			expectedState: "running",
			expectedMsg:   "Keycloak",
			shouldHaveDetails: true,
		},
		{
			name:   "Loki with storage details",
			plugin: NewLokiPlugin(),
			config: &services.LokiConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
					Status:  "running",
				},
				StorageType: "s3",
				BucketName:  "loki-logs",
				VolumeSize:  100,
			},
			expectedState: "running",
			expectedMsg:   "Loki",
			shouldHaveDetails: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.plugin.Status(tt.config)

			// Verify state is accurate
			assert.Equal(t, tt.expectedState, status.State, "Status state should match expected")

			// Verify message contains expected text
			assert.Contains(t, status.Message, tt.expectedMsg, "Status message should contain expected text")

			// Verify details are present when expected
			if tt.shouldHaveDetails {
				assert.NotNil(t, status.Details, "Status should include details for enabled services")
			}
		})
	}
}

// TestAllPluginsProvideStatus ensures all service plugins implement status reporting correctly
func TestAllPluginsProvideStatus(t *testing.T) {
	plugins := []struct {
		name   string
		plugin svc.ServicePlugin
		config interface{}
	}{
		{
			name:   "Velero",
			plugin: NewVeleroPlugin(),
			config: &services.VeleroConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
				BackupBucket: "test-bucket",
			},
		},
		{
			name:   "CertManager",
			plugin: NewCertManagerPlugin(),
			config: &services.CertManagerConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "Keycloak",
			plugin: NewKeycloakPlugin(),
			config: &services.KeycloakConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "Loki",
			plugin: NewLokiPlugin(),
			config: &services.LokiConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "PrometheusStack",
			plugin: NewPrometheusStackPlugin(),
			config: &services.PrometheusStackConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "Calico",
			plugin: NewCalicoPlugin(),
			config: &services.CalicoConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "Headlamp",
			plugin: NewHeadlampPlugin(),
			config: &services.HeadlampConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "WeaveGitOps",
			plugin: NewWeaveGitOpsPlugin(),
			config: &services.WeaveGitOpsConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "AlertProxy",
			plugin: NewAlertProxyPlugin(),
			config: &services.AlertProxyConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "EtcdBackup",
			plugin: NewEtcdBackupPlugin(),
			config: &services.EtcdBackupConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
		{
			name:   "VSphereCSI",
			plugin: NewVSphereCSIPlugin(),
			config: &services.VSphereCSIConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
			},
		},
	}

	for _, tt := range plugins {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.plugin.Status(tt.config)

			// All plugins should return a valid status
			assert.NotEmpty(t, status.State, "Plugin should return a status state")
			assert.NotEmpty(t, status.Message, "Plugin should return a status message")

			// When enabled with no explicit status, should default to pending
			assert.Equal(t, "pending", status.State, "Enabled service with no status should default to pending")
		})
	}
}

// TestStatusStateTransitions tests that status correctly reflects different states
func TestStatusStateTransitions(t *testing.T) {
	plugin := NewVeleroPlugin()

	states := []string{"pending", "running", "success", "failed"}

	for _, state := range states {
		t.Run("State_"+state, func(t *testing.T) {
			config := &services.VeleroConfig{
				BaseConfig: services.BaseConfig{
					Enabled: true,
					Status:  state,
				},
				BackupBucket: "test-bucket",
			}

			status := plugin.Status(config)
			assert.Equal(t, state, status.State, "Status should accurately reflect the configured state")
		})
	}
}

// TestStatusDetailsAccuracy tests that status details contain accurate configuration information
func TestStatusDetailsAccuracy(t *testing.T) {
	t.Run("Velero details include backup configuration", func(t *testing.T) {
		plugin := NewVeleroPlugin()
		config := &services.VeleroConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
				Status:  "running",
			},
			BackupBucket: "my-backup-bucket",
			Region:       "us-east-1",
		}

		status := plugin.Status(config)
		assert.Equal(t, "my-backup-bucket", status.Details["backup_bucket"])
		assert.Equal(t, "us-east-1", status.Details["region"])
	})

	t.Run("Loki details include storage configuration", func(t *testing.T) {
		plugin := NewLokiPlugin()
		config := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
				Status:  "running",
			},
			StorageType: "s3",
			BucketName:  "loki-storage",
			VolumeSize:  200,
		}

		status := plugin.Status(config)
		assert.Equal(t, "s3", status.Details["storage_type"])
		assert.Equal(t, "loki-storage", status.Details["bucket_name"])
		assert.Equal(t, 200, status.Details["volume_size"])
	})

	t.Run("Prometheus details include volume sizes", func(t *testing.T) {
		plugin := NewPrometheusStackPlugin()
		config := &services.PrometheusStackConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
				Status:  "running",
			},
			GrafanaVolumeSize:      50,
			PrometheusVolumeSize:   100,
			AlertmanagerVolumeSize: 25,
		}

		status := plugin.Status(config)
		assert.Equal(t, 50, status.Details["grafana_volume_size"])
		assert.Equal(t, 100, status.Details["prometheus_volume_size"])
		assert.Equal(t, 25, status.Details["alertmanager_volume_size"])
	})
}
