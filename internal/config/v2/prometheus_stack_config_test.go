package v2

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

func TestPrometheusStackDefaultIncludesReleaseName(t *testing.T) {
	service, ok := NewDefaultServiceConfig("kube-prometheus-stack", "cluster.example.com")
	if !ok {
		t.Fatal("NewDefaultServiceConfig did not recognize kube-prometheus-stack")
	}

	stack, ok := service.(*services.PrometheusStackConfig)
	if !ok {
		t.Fatalf("default service type = %T, want *PrometheusStackConfig", service)
	}
	if stack.ReleaseName != services.DefaultPrometheusStackReleaseName {
		t.Fatalf("release_name = %q, want %q", stack.ReleaseName, services.DefaultPrometheusStackReleaseName)
	}
	if stack.Namespace != "observability" {
		t.Fatalf("namespace = %q, want observability", stack.Namespace)
	}
}

func TestConfigNormalizeHydratesLegacyPrometheusStackReleaseName(t *testing.T) {
	legacy := &services.PrometheusStackConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "legacy-monitoring"},
	}
	cfg := &Config{
		OpenCenter: OpenCenterConfig{
			Services: ServiceMap{"kube-prometheus-stack": legacy},
		},
	}

	loader := &ConfigLoader{}
	if err := loader.normalize(cfg); err != nil {
		t.Fatalf("normalize() error = %v", err)
	}

	if legacy.ReleaseName != services.DefaultPrometheusStackReleaseName {
		t.Fatalf("legacy release_name = %q, want %q", legacy.ReleaseName, services.DefaultPrometheusStackReleaseName)
	}
	if legacy.Namespace != "legacy-monitoring" {
		t.Fatalf("legacy namespace = %q, want existing namespace to remain authoritative", legacy.Namespace)
	}
}

func TestValidateServicesRejectsInvalidPrometheusStackReleaseName(t *testing.T) {
	cfg := &Config{
		OpenCenter: OpenCenterConfig{
			Services: ServiceMap{
				"kube-prometheus-stack": &services.PrometheusStackConfig{
					BaseConfig:  services.BaseConfig{Enabled: true},
					ReleaseName: "bad.release",
				},
			},
		},
	}

	if err := NewValidator().ValidateServices(cfg); err == nil {
		t.Fatal("ValidateServices accepted an invalid Prometheus stack release name")
	}
}

func TestValidateServicesRejectsInvalidPrometheusStackNamespace(t *testing.T) {
	cfg := &Config{
		OpenCenter: OpenCenterConfig{
			Services: ServiceMap{
				"kube-prometheus-stack": &services.PrometheusStackConfig{
					BaseConfig: services.BaseConfig{
						Enabled:   true,
						Namespace: "monitoring.namespace",
					},
				},
			},
		},
	}

	if err := NewValidator().ValidateServices(cfg); err == nil {
		t.Fatal("ValidateServices accepted an invalid Prometheus stack namespace")
	}
}
