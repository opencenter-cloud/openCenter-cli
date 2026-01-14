package plugins

import (
	"context"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuiltInServicesIntegration tests the complete integration of built-in services
func TestBuiltInServicesIntegration(t *testing.T) {
	registry := svc.NewServiceRegistry()

	// Register all built-in services
	err := RegisterBuiltInServices(registry)
	require.NoError(t, err)

	t.Run("All services are registered", func(t *testing.T) {
		services := registry.ListServices()
		assert.GreaterOrEqual(t, len(services), 20)

		// Verify each service has a valid plugin
		for _, service := range services {
			assert.NotEmpty(t, service.Name)
			assert.NotNil(t, service.Plugin)
			assert.NotEmpty(t, service.Type)
			assert.Equal(t, service.Name, service.Plugin.Name())
			assert.Equal(t, service.Type, service.Plugin.Type())
		}
	})

	t.Run("Service dependencies are valid", func(t *testing.T) {
		services := registry.ListServices()

		for _, service := range services {
			// Verify all dependencies exist
			for _, dep := range service.Dependencies {
				_, err := registry.GetService(dep)
				assert.NoError(t, err, "Service %s depends on %s which should exist", service.Name, dep)
			}
		}
	})

	t.Run("Dependency resolution works", func(t *testing.T) {
		// Test resolving keycloak which depends on cert-manager
		resolved, err := registry.ResolveDependencies([]string{"keycloak"})
		require.NoError(t, err)
		assert.Len(t, resolved, 2) // cert-manager + keycloak

		// Verify cert-manager comes before keycloak
		var certManagerIndex, keycloakIndex int
		for i, service := range resolved {
			if service.Name == "cert-manager" {
				certManagerIndex = i
			}
			if service.Name == "keycloak" {
				keycloakIndex = i
			}
		}
		assert.Less(t, certManagerIndex, keycloakIndex, "cert-manager should come before keycloak")
	})

	t.Run("Plugin validation works", func(t *testing.T) {
		// Test Prometheus Stack validation
		prometheusService, err := registry.GetService("kube-prometheus-stack")
		require.NoError(t, err)

		validConfig := &services.PrometheusStackConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			GrafanaVolumeSize: 10,
		}
		err = prometheusService.Plugin.Validate(validConfig)
		assert.NoError(t, err)

		invalidConfig := &services.PrometheusStackConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			GrafanaVolumeSize: -1,
		}
		err = prometheusService.Plugin.Validate(invalidConfig)
		assert.Error(t, err)
	})

	t.Run("Plugin status reporting works", func(t *testing.T) {
		// Test Velero status
		veleroService, err := registry.GetService("velero")
		require.NoError(t, err)

		config := &services.VeleroConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
				Status:  "running",
			},
			BackupBucket: "my-bucket",
		}

		status := veleroService.Plugin.Status(config)
		assert.Equal(t, "running", status.State)
		assert.Contains(t, status.Message, "Velero")
		assert.NotNil(t, status.Details)
	})

	t.Run("Plugin rendering works", func(t *testing.T) {
		// Test that render doesn't error (even though it's a placeholder)
		certManagerService, err := registry.GetService("cert-manager")
		require.NoError(t, err)

		config := &services.CertManagerConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
		}

		err = certManagerService.Plugin.Render(context.Background(), config, nil)
		assert.NoError(t, err)
	})

	t.Run("Complex dependency resolution", func(t *testing.T) {
		// Test resolving multiple services with overlapping dependencies
		services := []string{"keycloak", "alert-proxy", "weave-gitops"}
		resolved, err := registry.ResolveDependencies(services)
		require.NoError(t, err)

		// Should include: cert-manager, keycloak, kube-prometheus-stack, alert-proxy, fluxcd, weave-gitops
		assert.GreaterOrEqual(t, len(resolved), 6)

		// Verify dependencies come before dependents
		serviceOrder := make(map[string]int)
		for i, service := range resolved {
			serviceOrder[service.Name] = i
		}

		// cert-manager should come before keycloak
		assert.Less(t, serviceOrder["cert-manager"], serviceOrder["keycloak"])

		// kube-prometheus-stack should come before alert-proxy
		assert.Less(t, serviceOrder["kube-prometheus-stack"], serviceOrder["alert-proxy"])

		// fluxcd should come before weave-gitops
		assert.Less(t, serviceOrder["fluxcd"], serviceOrder["weave-gitops"])
	})

	t.Run("Service types are correct", func(t *testing.T) {
		testCases := []struct {
			name         string
			expectedType svc.ServiceType
		}{
			{"kube-prometheus-stack", svc.ServiceTypeMonitoring},
			{"cert-manager", svc.ServiceTypeSecurity},
			{"velero", svc.ServiceTypeStorage},
			{"loki", svc.ServiceTypeLogging},
			{"calico", svc.ServiceTypeNetworking},
			{"fluxcd", svc.ServiceTypeGitOps},
			{"headlamp", svc.ServiceTypeCore},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				service, err := registry.GetService(tc.name)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, service.Type)
			})
		}
	})

	t.Run("All built-in service names are accessible", func(t *testing.T) {
		names := GetBuiltInServiceNames()

		for _, name := range names {
			service, err := registry.GetService(name)
			require.NoError(t, err, "Service %s should be registered", name)
			assert.Equal(t, name, service.Name)
		}
	})
}

// TestBuiltInServicesWithRealConfigs tests plugins with realistic configurations
func TestBuiltInServicesWithRealConfigs(t *testing.T) {
	registry := svc.NewServiceRegistry()
	err := RegisterBuiltInServices(registry)
	require.NoError(t, err)

	t.Run("Loki with Swift storage", func(t *testing.T) {
		lokiService, err := registry.GetService("loki")
		require.NoError(t, err)

		config := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled:   true,
				Namespace: "logging",
			},
			StorageType:        "swift",
			SwiftAuthURL:       "https://keystone.example.com:5000/v3",
			SwiftRegion:        "RegionOne",
			SwiftContainerName: "loki-logs",
			VolumeSize:         100,
		}

		err = lokiService.Plugin.Validate(config)
		assert.NoError(t, err)

		status := lokiService.Plugin.Status(config)
		// Status state should default to "pending" if not explicitly set in config
		assert.Equal(t, "pending", status.State)
		assert.Contains(t, status.Message, "Loki")
	})

	t.Run("Prometheus Stack with storage configuration", func(t *testing.T) {
		prometheusService, err := registry.GetService("kube-prometheus-stack")
		require.NoError(t, err)

		config := &services.PrometheusStackConfig{
			BaseConfig: services.BaseConfig{
				Enabled:   true,
				Namespace: "monitoring",
			},
			GrafanaVolumeSize:      10,
			GrafanaStorageClass:    "standard",
			PrometheusVolumeSize:   50,
			PrometheusStorageClass: "fast",
			AlertmanagerVolumeSize: 10,
			WebhookURL:             "https://alerts.example.com/webhook",
		}

		err = prometheusService.Plugin.Validate(config)
		assert.NoError(t, err)

		status := prometheusService.Plugin.Status(config)
		// Status state should default to "pending" if not explicitly set in config
		assert.Equal(t, "pending", status.State)
		assert.Contains(t, status.Message, "Prometheus")
		assert.NotNil(t, status.Details)
	})

	t.Run("Keycloak with OIDC configuration", func(t *testing.T) {
		keycloakService, err := registry.GetService("keycloak")
		require.NoError(t, err)

		config := &services.KeycloakConfig{
			BaseConfig: services.BaseConfig{
				Enabled:   true,
				Namespace: "auth",
			},
			Realm:       "kubernetes",
			FrontendURL: "https://keycloak.example.com",
			ClientID:    "kubernetes-client",
		}

		err = keycloakService.Plugin.Validate(config)
		assert.NoError(t, err)

		status := keycloakService.Plugin.Status(config)
		// Status state should default to "pending" if not explicitly set in config
		assert.Equal(t, "pending", status.State)
		assert.Contains(t, status.Message, "Keycloak")
	})

	t.Run("Velero with backup configuration", func(t *testing.T) {
		veleroService, err := registry.GetService("velero")
		require.NoError(t, err)

		config := &services.VeleroConfig{
			BaseConfig: services.BaseConfig{
				Enabled:   true,
				Namespace: "velero",
			},
			BackupBucket: "cluster-backups",
			Region:       "us-east-1",
		}

		err = veleroService.Plugin.Validate(config)
		assert.NoError(t, err)

		status := veleroService.Plugin.Status(config)
		// Status state should default to "pending" if not explicitly set in config
		assert.Equal(t, "pending", status.State)
		assert.Contains(t, status.Message, "Velero")
	})
}

// TestBuiltInServicesErrorHandling tests error handling in plugins
func TestBuiltInServicesErrorHandling(t *testing.T) {
	registry := svc.NewServiceRegistry()
	err := RegisterBuiltInServices(registry)
	require.NoError(t, err)

	t.Run("Invalid config type", func(t *testing.T) {
		prometheusService, err := registry.GetService("kube-prometheus-stack")
		require.NoError(t, err)

		// Pass wrong config type
		wrongConfig := &services.VeleroConfig{}
		err = prometheusService.Plugin.Validate(wrongConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid config type")
	})

	t.Run("Missing required fields", func(t *testing.T) {
		veleroService, err := registry.GetService("velero")
		require.NoError(t, err)

		// Enabled but missing backup bucket
		config := &services.VeleroConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
		}

		err = veleroService.Plugin.Validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "velero_backup_bucket is required")
	})

	t.Run("Invalid field values", func(t *testing.T) {
		lokiService, err := registry.GetService("loki")
		require.NoError(t, err)

		// Invalid storage type
		config := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "invalid-type",
		}

		err = lokiService.Plugin.Validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be 's3' or 'swift'")
	})
}
