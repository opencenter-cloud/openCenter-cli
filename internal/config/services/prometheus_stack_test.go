package services

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPrometheusStackDefaultsPreserveNamespace(t *testing.T) {
	config := &PrometheusStackConfig{}

	config.ApplyDefaults()

	if config.ReleaseName != DefaultPrometheusStackReleaseName {
		t.Fatalf("release_name = %q, want %q", config.ReleaseName, DefaultPrometheusStackReleaseName)
	}
	if config.Namespace != DefaultPrometheusStackNamespace {
		t.Fatalf("namespace = %q, want %q", config.Namespace, DefaultPrometheusStackNamespace)
	}

	config.Namespace = "custom-observability"
	config.ApplyDefaults()
	if config.Namespace != "custom-observability" {
		t.Fatalf("namespace = %q, want existing namespace to remain authoritative", config.Namespace)
	}
}

func TestPrometheusStackLegacyYAMLReceivesReleaseNameDefault(t *testing.T) {
	var config PrometheusStackConfig
	if err := yaml.Unmarshal([]byte("namespace: legacy-monitoring\n"), &config); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if config.ReleaseName != DefaultPrometheusStackReleaseName {
		t.Fatalf("legacy release_name = %q, want %q", config.ReleaseName, DefaultPrometheusStackReleaseName)
	}
	if config.Namespace != "legacy-monitoring" {
		t.Fatalf("legacy namespace = %q, want existing namespace to remain authoritative", config.Namespace)
	}
}

func TestValidatePrometheusStackReleaseName(t *testing.T) {
	valid := DefaultPrometheusStackReleaseName
	if err := ValidatePrometheusStackReleaseName(valid); err != nil {
		t.Fatalf("default release name should be valid: %v", err)
	}

	for _, name := range []string{
		"Uppercase",
		"release.name",
		"-leading-hyphen",
		"trailing-hyphen-",
		"release_name",
		"this-release-name-is-way-too-long",
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePrometheusStackReleaseName(name); err == nil {
				t.Fatalf("release name %q should be rejected", name)
			}
		})
	}
}

func TestValidatePrometheusStackNamespace(t *testing.T) {
	if err := ValidatePrometheusStackNamespace("monitoring-prod"); err != nil {
		t.Fatalf("valid namespace rejected: %v", err)
	}

	for _, namespace := range []string{
		"Monitoring",
		"monitoring.namespace",
		"-monitoring",
		"monitoring-",
		"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmno",
	} {
		t.Run(namespace, func(t *testing.T) {
			if err := ValidatePrometheusStackNamespace(namespace); err == nil {
				t.Fatalf("namespace %q should be rejected", namespace)
			}
		})
	}
}
