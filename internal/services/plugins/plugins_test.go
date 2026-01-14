package plugins

import (
	"context"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusStackPlugin(t *testing.T) {
	plugin := NewPrometheusStackPlugin()

	assert.Equal(t, "kube-prometheus-stack", plugin.Name())
	assert.Equal(t, svc.ServiceTypeMonitoring, plugin.Type())

	t.Run("Validate valid config", func(t *testing.T) {
		cfg := &services.PrometheusStackConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			GrafanaVolumeSize:      10,
			PrometheusVolumeSize:   50,
			AlertmanagerVolumeSize: 10,
		}

		err := plugin.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Validate negative volume size", func(t *testing.T) {
		cfg := &services.PrometheusStackConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			GrafanaVolumeSize: -1,
		}

		err := plugin.Validate(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "grafana_volume_size must be non-negative")
	})

	t.Run("Status for enabled service", func(t *testing.T) {
		cfg := &services.PrometheusStackConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
				Status:  "running",
			},
			GrafanaVolumeSize: 10,
		}

		status := plugin.Status(cfg)
		assert.Equal(t, "running", status.State)
		assert.Contains(t, status.Message, "Prometheus")
	})
}

func TestCertManagerPlugin(t *testing.T) {
	plugin := NewCertManagerPlugin()

	assert.Equal(t, "cert-manager", plugin.Name())
	assert.Equal(t, svc.ServiceTypeSecurity, plugin.Type())

	t.Run("Validate valid config", func(t *testing.T) {
		cfg := &services.CertManagerConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
			Email:             "admin@example.com",
		}

		err := plugin.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Validate invalid URL", func(t *testing.T) {
		cfg := &services.CertManagerConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			LetsEncryptServer: "http://acme.example.com",
		}

		err := plugin.Validate(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be an HTTPS URL")
	})

	t.Run("Validate invalid email", func(t *testing.T) {
		cfg := &services.CertManagerConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			Email: "invalid-email",
		}

		err := plugin.Validate(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "valid email address")
	})
}

func TestVeleroPlugin(t *testing.T) {
	plugin := NewVeleroPlugin()

	assert.Equal(t, "velero", plugin.Name())
	assert.Equal(t, svc.ServiceTypeStorage, plugin.Type())

	t.Run("Validate enabled service requires bucket", func(t *testing.T) {
		cfg := &services.VeleroConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
		}

		err := plugin.Validate(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "velero_backup_bucket is required")
	})

	t.Run("Validate valid config", func(t *testing.T) {
		cfg := &services.VeleroConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			BackupBucket: "my-backup-bucket",
			Region:       "us-east-1",
		}

		err := plugin.Validate(cfg)
		assert.NoError(t, err)
	})
}

func TestLokiPlugin(t *testing.T) {
	plugin := NewLokiPlugin()

	assert.Equal(t, "loki", plugin.Name())
	assert.Equal(t, svc.ServiceTypeLogging, plugin.Type())

	t.Run("Validate Swift storage", func(t *testing.T) {
		cfg := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType:        "swift",
			SwiftAuthURL:       "https://keystone.example.com:5000/v3",
			SwiftContainerName: "loki-logs",
		}

		err := plugin.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Validate Swift missing auth URL", func(t *testing.T) {
		cfg := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "swift",
		}

		err := plugin.Validate(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "swift_auth_url is required")
	})

	t.Run("Validate S3 storage", func(t *testing.T) {
		cfg := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			S3Endpoint:  "https://s3.amazonaws.com",
			BucketName:  "loki-logs",
		}

		err := plugin.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Validate invalid storage type", func(t *testing.T) {
		cfg := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "invalid",
		}

		err := plugin.Validate(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be 's3' or 'swift'")
	})
}

func TestKeycloakPlugin(t *testing.T) {
	plugin := NewKeycloakPlugin()

	assert.Equal(t, "keycloak", plugin.Name())
	assert.Equal(t, svc.ServiceTypeSecurity, plugin.Type())

	t.Run("Validate valid config", func(t *testing.T) {
		cfg := &services.KeycloakConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			Realm:       "master",
			FrontendURL: "https://keycloak.example.com",
			ClientID:    "my-client",
		}

		err := plugin.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Validate invalid URL", func(t *testing.T) {
		cfg := &services.KeycloakConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			FrontendURL: "invalid-url",
		}

		err := plugin.Validate(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "valid HTTP(S) URL")
	})
}

func TestDefaultServicePlugin(t *testing.T) {
	plugin := NewDefaultServicePlugin("test-service", svc.ServiceTypeCore)

	assert.Equal(t, "test-service", plugin.Name())
	assert.Equal(t, svc.ServiceTypeCore, plugin.Type())

	t.Run("Validate", func(t *testing.T) {
		cfg := &services.DefaultServiceConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
		}

		err := plugin.Validate(cfg)
		assert.NoError(t, err)
	})

	t.Run("Render", func(t *testing.T) {
		cfg := &services.DefaultServiceConfig{}
		err := plugin.Render(context.Background(), cfg, nil)
		assert.NoError(t, err)
	})

	t.Run("Status", func(t *testing.T) {
		cfg := &services.DefaultServiceConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
				Status:  "running",
			},
		}

		status := plugin.Status(cfg)
		assert.Equal(t, "running", status.State)
		assert.Contains(t, status.Message, "test-service")
	})
}

func TestRegisterBuiltInServices(t *testing.T) {
	registry := svc.NewServiceRegistry()

	err := RegisterBuiltInServices(registry)
	require.NoError(t, err)

	// Verify all services are registered
	services := registry.ListServices()
	assert.GreaterOrEqual(t, len(services), 20, "Expected at least 20 built-in services")

	// Verify specific services
	expectedServices := []string{
		"kube-prometheus-stack",
		"cert-manager",
		"velero",
		"loki",
		"keycloak",
		"calico",
		"headlamp",
		"weave-gitops",
		"fluxcd",
	}

	for _, name := range expectedServices {
		service, err := registry.GetService(name)
		require.NoError(t, err, "Service %s should be registered", name)
		assert.Equal(t, name, service.Name)
		assert.NotNil(t, service.Plugin)
	}
}

func TestBuiltInServiceDependencies(t *testing.T) {
	registry := svc.NewServiceRegistry()

	err := RegisterBuiltInServices(registry)
	require.NoError(t, err)

	// Test that keycloak depends on cert-manager
	keycloak, err := registry.GetService("keycloak")
	require.NoError(t, err)
	assert.Contains(t, keycloak.Dependencies, "cert-manager")

	// Test that alert-proxy depends on kube-prometheus-stack
	alertProxy, err := registry.GetService("alert-proxy")
	require.NoError(t, err)
	assert.Contains(t, alertProxy.Dependencies, "kube-prometheus-stack")

	// Test that weave-gitops depends on fluxcd
	weaveGitops, err := registry.GetService("weave-gitops")
	require.NoError(t, err)
	assert.Contains(t, weaveGitops.Dependencies, "fluxcd")

	// Test that gateway depends on gateway-api
	gateway, err := registry.GetService("gateway")
	require.NoError(t, err)
	assert.Contains(t, gateway.Dependencies, "gateway-api")
}

func TestGetBuiltInServiceNames(t *testing.T) {
	names := GetBuiltInServiceNames()

	assert.GreaterOrEqual(t, len(names), 20, "Expected at least 20 built-in service names")

	// Verify some expected services are in the list
	expectedServices := []string{
		"kube-prometheus-stack",
		"cert-manager",
		"velero",
		"loki",
		"keycloak",
	}

	for _, expected := range expectedServices {
		assert.Contains(t, names, expected, "Expected service %s to be in built-in services list", expected)
	}
}
