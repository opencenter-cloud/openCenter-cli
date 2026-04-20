// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// Helper to populate required infrastructure fields for validation
func populateInfraFields(cfg *Config) {
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://auth.example.com"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "test-region"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
	cfg.OpenCenter.Secrets.Barbican.AuthURL = "https://barbican.example.com"
}

func TestResolveConfigDir(t *testing.T) {
	// Unset env var to test default behavior
	os.Unsetenv("OPENCENTER_CONFIG_DIR")

	dir, err := ResolveConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "opencenter")
	if dir != expected {
		t.Errorf("expected config dir %s, but got %s", expected, dir)
	}

	// Set env var to test override (use repo testdata)
	testDir := "testdata/opencenter-test"
	os.Setenv("OPENCENTER_CONFIG_DIR", testDir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	dir, err = ResolveConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	// ResolveConfigDir returns an absolute path; compare absolute forms.
	absExpected, _ := filepath.Abs(testDir)
	if dir != absExpected {
		t.Errorf("expected config dir %s, but got %s", absExpected, dir)
	}
}

func TestResolveStateDir(t *testing.T) {
	t.Run("default behavior", func(t *testing.T) {
		os.Unsetenv("OPENCENTER_STATE_DIR")
		os.Unsetenv("XDG_STATE_HOME")

		dir, err := ResolveStateDir()
		if err != nil {
			t.Fatal(err)
		}

		var expected string
		switch runtime.GOOS {
		case "windows":
			base := os.Getenv("LOCALAPPDATA")
			if base == "" {
				base = os.Getenv("APPDATA")
			}
			if base == "" {
				base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
			}
			expected = filepath.Join(base, "opencenter", "state")
		default:
			home, _ := os.UserHomeDir()
			expected = filepath.Join(home, ".local", "state", "opencenter")
		}

		if dir != expected {
			t.Errorf("expected state dir %s, but got %s", expected, dir)
		}
	})

	t.Run("env override", func(t *testing.T) {
		testDir := "testdata/opencenter-state-test"
		os.Setenv("OPENCENTER_STATE_DIR", testDir)
		defer os.Unsetenv("OPENCENTER_STATE_DIR")

		dir, err := ResolveStateDir()
		if err != nil {
			t.Fatal(err)
		}

		absExpected, _ := filepath.Abs(testDir)
		if dir != absExpected {
			t.Errorf("expected state dir %s, but got %s", absExpected, dir)
		}
	})
}

func TestConfigHelperMethods(t *testing.T) {
	cfg := NewDefault("test-cluster")

	// Test ClusterName method
	if cfg.ClusterName() != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %s", cfg.ClusterName())
	}

	// Test GitOps method
	gitops := cfg.GitOps()
	if gitops.Repository.Branch != "main" {
		t.Errorf("expected git branch 'main', got %s", gitops.Repository.Branch)
	}
}

func TestConfigToJSON(t *testing.T) {
	cfg := NewDefault("test-cluster")
	cfg.OpenCenter.GitOps.Repository.LocalDir = "test-dir"

	jsonData, err := cfg.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's valid JSON by unmarshaling
	var result map[string]any
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("generated JSON is invalid: %v", err)
	}

	// Check that some expected fields are present
	if result["opencenter"] == nil {
		t.Error("expected 'opencenter' field in JSON output")
	}
	if result["opentofu"] == nil {
		t.Error("expected 'opentofu' field in JSON output")
	}
	if result["secrets"] == nil {
		t.Error("expected 'secrets' field in JSON output")
	}
}

func TestListEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	defer os.Unsetenv("OPENCENTER_CONFIG_DIR")

	manager, err := NewConfigurationManager()
	if err != nil {
		t.Fatal(err)
	}

	names, err := manager.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestActiveClusterOperations(t *testing.T) {
	dir := t.TempDir()
	oldConfigDir := os.Getenv("OPENCENTER_CONFIG_DIR")
	oldActiveCluster := os.Getenv("OPENCENTER_CLUSTER")
	oldSessionFile := os.Getenv("OPENCENTER_SESSION_FILE")
	os.Setenv("OPENCENTER_CONFIG_DIR", dir)
	os.Unsetenv("OPENCENTER_CLUSTER")
	os.Unsetenv("OPENCENTER_SESSION_FILE")
	defer func() {
		if oldConfigDir != "" {
			os.Setenv("OPENCENTER_CONFIG_DIR", oldConfigDir)
		} else {
			os.Unsetenv("OPENCENTER_CONFIG_DIR")
		}
		if oldActiveCluster != "" {
			os.Setenv("OPENCENTER_CLUSTER", oldActiveCluster)
		} else {
			os.Unsetenv("OPENCENTER_CLUSTER")
		}
		if oldSessionFile != "" {
			os.Setenv("OPENCENTER_SESSION_FILE", oldSessionFile)
		} else {
			os.Unsetenv("OPENCENTER_SESSION_FILE")
		}
	}()

	manager, err := NewConfigurationManager()
	if err != nil {
		t.Fatal(err)
	}

	// Test getting active when no active cluster is set
	active, err := manager.GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if active != "" {
		t.Errorf("expected empty active cluster, got %s", active)
	}

	// Test setting active cluster
	if err := manager.SetActive("test-cluster"); err != nil {
		t.Fatal(err)
	}

	// Test getting active cluster
	active, err = manager.GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if active != "test-cluster" {
		t.Errorf("expected active cluster 'test-cluster', got %s", active)
	}

	// Test clearing active cluster
	if err := manager.SetActive(""); err != nil {
		t.Fatal(err)
	}

	active, err = manager.GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if active != "" {
		t.Errorf("expected empty active cluster after clearing, got %s", active)
	}
}

// TestDefaultConfigNewFields tests that NewDefault populates all new configuration fields correctly
func TestDefaultConfigNewFields(t *testing.T) {
	t.Run("CheckTestMode", func(t *testing.T) {
		// Use test fixture instead of environment variable
		cfg := testModeConfig("test-mode-cluster")

		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "https://identity.example.com/v3" {
			t.Errorf("Expected AuthURL to be populated in test mode")
		}
		// Service-specific secrets (like CertManager) are intentionally NOT populated in test mode
		// Only infrastructure-level AWS credentials are populated
		if cfg.Secrets.Global.AWS.Infrastructure.AccessKey != "test-aws-access-key" {
			t.Errorf("Expected Infrastructure AWS AccessKey to be populated in test mode")
		}
	})

	cfg := NewDefault("test-cluster")

	// Test ClusterConfig new fields
	t.Run("ClusterConfig fields", func(t *testing.T) {
		if cfg.OpenCenter.Cluster.BaseDomain != "k8s.opencenter.cloud" {
			t.Errorf("expected BaseDomain 'k8s.opencenter.cloud', got %s", cfg.OpenCenter.Cluster.BaseDomain)
		}

		expectedFQDN := "test-cluster.sjc3.k8s.opencenter.cloud"
		if cfg.OpenCenter.Cluster.ClusterFQDN != expectedFQDN {
			t.Errorf("expected ClusterFQDN '%s', got %s", expectedFQDN, cfg.OpenCenter.Cluster.ClusterFQDN)
		}

		// AdminEmail should have default value
		if cfg.OpenCenter.Cluster.AdminEmail != "admin@example.com" {
			t.Errorf("expected AdminEmail 'admin@example.com', got %s", cfg.OpenCenter.Cluster.AdminEmail)
		}
	})

	// Test GitOpsConfig new fields
	t.Run("GitOpsConfig fields", func(t *testing.T) {
		if cfg.OpenCenter.GitOps.BaseRepo.URL != "ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git" {
			t.Errorf("expected BaseRepo.URL 'ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git', got %s", cfg.OpenCenter.GitOps.BaseRepo.URL)
		}

		if cfg.OpenCenter.GitOps.BaseRepo.Release != "v0.1.0" {
			t.Errorf("expected BaseRepo.Release 'v0.1.0', got %s", cfg.OpenCenter.GitOps.BaseRepo.Release)
		}

		if cfg.OpenCenter.GitOps.Repository.Branch != "main" {
			t.Errorf("expected Repository.Branch 'main', got %s", cfg.OpenCenter.GitOps.Repository.Branch)
		}
	})

	// Test StorageConfig
	t.Run("StorageConfig fields", func(t *testing.T) {
		if cfg.OpenCenter.Storage.DefaultStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected DefaultStorageClass 'csi-cinder-sc-delete', got %s", cfg.OpenCenter.Storage.DefaultStorageClass)
		}
	})

	// Test Service fields
	t.Run("Service fields", func(t *testing.T) {
		// Test cert-manager
		certManagerAny, ok := cfg.OpenCenter.Services["cert-manager"]
		if !ok {
			t.Fatal("cert-manager service not found")
		}
		certManager, ok := certManagerAny.(*services.CertManagerConfig)
		if !ok {
			t.Fatalf("cert-manager service is not of type *services.CertManagerConfig, got %T", certManagerAny)
		}
		if certManager.LetsEncryptServer != "https://acme-v02.api.letsencrypt.org/directory" {
			t.Errorf("expected LetsEncryptServer 'https://acme-v02.api.letsencrypt.org/directory', got %s", certManager.LetsEncryptServer)
		}
		if certManager.Region != "us-east-1" {
			t.Errorf("expected cert-manager Region 'us-east-1', got %s", certManager.Region)
		}

		// Test loki
		lokiAny, ok := cfg.OpenCenter.Services["loki"]
		if !ok {
			t.Fatal("loki service not found")
		}
		loki, ok := lokiAny.(*services.LokiConfig)
		if !ok {
			t.Fatalf("loki service is not of type *services.LokiConfig, got %T", lokiAny)
		}
		if loki.VolumeSize != 20 {
			t.Errorf("expected VolumeSize 20, got %d", loki.VolumeSize)
		}
		if loki.StorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected StorageClass 'csi-cinder-sc-delete', got %s", loki.StorageClass)
		}

		// Test kube-prometheus-stack
		promStackAny, ok := cfg.OpenCenter.Services["kube-prometheus-stack"]
		if !ok {
			t.Fatal("kube-prometheus-stack service not found")
		}
		promStack, ok := promStackAny.(*services.PrometheusStackConfig)
		if !ok {
			t.Fatalf("kube-prometheus-stack service is not of type *services.PrometheusStackConfig, got %T", promStackAny)
		}
		if promStack.PrometheusVolumeSize != 50 {
			t.Errorf("expected PrometheusVolumeSize 50, got %d", promStack.PrometheusVolumeSize)
		}
		if promStack.PrometheusStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected PrometheusStorageClass 'csi-cinder-sc-delete', got %s", promStack.PrometheusStorageClass)
		}
		if promStack.GrafanaVolumeSize != 10 {
			t.Errorf("expected GrafanaVolumeSize 10, got %d", promStack.GrafanaVolumeSize)
		}
		if promStack.GrafanaStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected GrafanaStorageClass 'csi-cinder-sc-delete', got %s", promStack.GrafanaStorageClass)
		}
		if promStack.AlertmanagerVolumeSize != 10 {
			t.Errorf("expected AlertmanagerVolumeSize 10, got %d", promStack.AlertmanagerVolumeSize)
		}
		if promStack.AlertmanagerStorageClass != "csi-cinder-sc-delete" {
			t.Errorf("expected AlertmanagerStorageClass 'csi-cinder-sc-delete', got %s", promStack.AlertmanagerStorageClass)
		}
	})

	// Test Secrets section initialization
	t.Run("Secrets section", func(t *testing.T) {
		// All service-specific secrets should be empty (must be provided by user)
		if cfg.Secrets.CertManager.AWSAccessKey != "" {
			t.Error("expected CertManager.AWSAccessKey to be empty")
		}
		if cfg.Secrets.CertManager.AWSSecretAccessKey != "" {
			t.Error("expected CertManager.AWSSecretAccessKey to be empty")
		}
		if cfg.Secrets.Loki.SwiftPassword != "" {
			t.Error("expected Loki.SwiftPassword to be empty")
		}
		if cfg.Secrets.Keycloak.ClientSecret != "" {
			t.Error("expected Keycloak.ClientSecret to be empty")
		}
		if cfg.Secrets.Keycloak.AdminPassword != "" {
			t.Error("expected Keycloak.AdminPassword to be empty")
		}
		if cfg.Secrets.Headlamp.OIDCClientSecret != "" {
			t.Error("expected Headlamp.OIDCClientSecret to be empty")
		}
		if cfg.Secrets.WeaveGitOps.Password != "" {
			t.Error("expected WeaveGitOps.Password to be empty")
		}
		if cfg.Secrets.WeaveGitOps.PasswordHash != "" {
			t.Error("expected WeaveGitOps.PasswordHash to be empty")
		}
		if cfg.Secrets.Grafana.AdminPassword != "" {
			t.Error("expected Grafana.AdminPassword to be empty")
		}
		if cfg.Secrets.AlertProxy.CoreDeviceId != "" {
			t.Error("expected AlertProxy.CoreDeviceId to be empty")
		}
		if cfg.Secrets.AlertProxy.AccountServiceToken != "" {
			t.Error("expected AlertProxy.AccountServiceToken to be empty")
		}
		if cfg.Secrets.AlertProxy.CoreAccountNumber != "" {
			t.Error("expected AlertProxy.CoreAccountNumber to be empty")
		}
	})
}

// TestDefaultConfigMatchesSpecifications tests that default values match the specifications
func TestDefaultConfigMatchesSpecifications(t *testing.T) {
	tests := []struct {
		name     string
		getValue func(Config) any
		expected any
	}{
		{
			name:     "BaseDomain default",
			getValue: func(c Config) any { return c.OpenCenter.Cluster.BaseDomain },
			expected: "k8s.opencenter.cloud",
		},
		{
			name:     "AdminEmail default",
			getValue: func(c Config) any { return c.OpenCenter.Cluster.AdminEmail },
			expected: "admin@example.com",
		},
		{
			name:     "GitOpsBaseRepo default",
			getValue: func(c Config) any { return c.OpenCenter.GitOps.BaseRepo.URL },
			expected: "ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git",
		},
		{
			name:     "GitOpsBaseRelease default",
			getValue: func(c Config) any { return c.OpenCenter.GitOps.BaseRepo.Release },
			expected: "v0.1.0",
		},
		{
			name:     "GitOpsBranch default",
			getValue: func(c Config) any { return c.OpenCenter.GitOps.Repository.Branch },
			expected: "main",
		},
		{
			name:     "DefaultStorageClass default",
			getValue: func(c Config) any { return c.OpenCenter.Storage.DefaultStorageClass },
			expected: "csi-cinder-sc-delete",
		},
		{
			name: "LetsEncryptServer default",
			getValue: func(c Config) any {
				return c.OpenCenter.Services["cert-manager"].(*services.CertManagerConfig).LetsEncryptServer
			},
			expected: "https://acme-v02.api.letsencrypt.org/directory",
		},
		{
			name:     "VolumeSize default",
			getValue: func(c Config) any { return c.OpenCenter.Services["loki"].(*services.LokiConfig).VolumeSize },
			expected: 20,
		},
		{
			name: "PrometheusVolumeSize default",
			getValue: func(c Config) any {
				return c.OpenCenter.Services["kube-prometheus-stack"].(*services.PrometheusStackConfig).PrometheusVolumeSize
			},
			expected: 50,
		},
		{
			name: "GrafanaVolumeSize default",
			getValue: func(c Config) any {
				return c.OpenCenter.Services["kube-prometheus-stack"].(*services.PrometheusStackConfig).GrafanaVolumeSize
			},
			expected: 10,
		},
	}

	cfg := NewDefault("test-cluster")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.getValue(cfg)
			if actual != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

// TestValidateEmailFormat tests email validation
// DISABLED: This test uses old API - validation is now in internal/core/validation/validators
func TestValidateEmailFormat_DISABLED(t *testing.T) {
	t.Skip("Test disabled - uses old API")
}

// TestValidateDomainFormat tests domain validation
// DISABLED: This test uses old API - validation is now in internal/core/validation/validators
func TestValidateDomainFormat_DISABLED(t *testing.T) {
	t.Skip("Test disabled - uses old API")
}

// TestValidateServiceSpecificRequirements tests service-specific validation
// DISABLED: This test uses old API - validation is now in internal/core/validation/validators
func TestValidateServiceSpecificRequirements_DISABLED(t *testing.T) {
	t.Skip("Test disabled - uses old API")
}

// TestTemplateRenderingWithNewFields tests that new configuration fields can be used in templates
func TestTemplateRenderingWithNewFields(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		setupConfig  func() Config
		expected     string
	}{
		{
			name:         "BaseDomain rendering",
			templateText: "domain: {{ .OpenCenter.Cluster.BaseDomain }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.BaseDomain = "k8s.example.com"
				return cfg
			},
			expected: "domain: k8s.example.com",
		},
		{
			name:         "ClusterFQDN rendering",
			templateText: "fqdn: {{ .OpenCenter.Cluster.ClusterFQDN }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterFQDN = "test.sjc3.k8s.example.com"
				return cfg
			},
			expected: "fqdn: test.sjc3.k8s.example.com",
		},
		{
			name:         "AdminEmail rendering",
			templateText: "email: {{ .OpenCenter.Cluster.AdminEmail }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.AdminEmail = "admin@example.com"
				return cfg
			},
			expected: "email: admin@example.com",
		},
		{
			name:         "GitOpsBaseRepo rendering",
			templateText: "repo: {{ .OpenCenter.GitOps.BaseRepo.URL }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.GitOps.BaseRepo.URL = "ssh://git@github.com/example/repo.git"
				return cfg
			},
			expected: "repo: ssh://git@github.com/example/repo.git",
		},
		{
			name:         "DefaultStorageClass rendering",
			templateText: "storageClass: {{ .OpenCenter.Storage.DefaultStorageClass }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Storage.DefaultStorageClass = "csi-cinder-sc-delete"
				return cfg
			},
			expected: "storageClass: csi-cinder-sc-delete",
		},
		{
			name:         "LetsEncryptServer rendering",
			templateText: `server: {{ (index .OpenCenter.Services "cert-manager").LetsEncryptServer }}`,
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{
					LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
				}
				return cfg
			},
			expected: "server: https://acme-v02.api.letsencrypt.org/directory",
		},
		{
			name:         "VolumeSize rendering",
			templateText: "size: {{ .OpenCenter.Services.loki.VolumeSize }}Gi",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["loki"] = &services.LokiConfig{
					VolumeSize: 20,
				}
				return cfg
			},
			expected: "size: 20Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()

			// Use text/template to render
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestTemplateRenderingWithSecrets tests that secret values are properly rendered
func TestTemplateRenderingWithSecrets(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		setupConfig  func() Config
		expected     string
	}{
		{
			name:         "CertManager AWS access key",
			templateText: "key: {{ .Secrets.CertManager.AWSAccessKey }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.CertManager.AWSAccessKey = "AKIA..."
				return cfg
			},
			expected: "key: AKIA...",
		},
		{
			name:         "Loki Swift password",
			templateText: "password: {{ .Secrets.Loki.SwiftPassword }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Loki.SwiftPassword = "secret-password"
				return cfg
			},
			expected: "password: secret-password",
		},
		{
			name:         "Keycloak admin password",
			templateText: "password: {{ .Secrets.Keycloak.AdminPassword }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Keycloak.AdminPassword = "admin-secret"
				return cfg
			},
			expected: "password: admin-secret",
		},
		{
			name:         "Grafana admin password",
			templateText: "password: {{ .Secrets.Grafana.AdminPassword }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Grafana.AdminPassword = "grafana-secret"
				return cfg
			},
			expected: "password: grafana-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()

			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestTemplateRenderingWithSprigFunctions tests Sprig function usage
func TestTemplateRenderingWithSprigFunctions(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		setupConfig  func() Config
		expected     string
	}{
		{
			name:         "b64enc function",
			templateText: "encoded: {{ .Secrets.Grafana.AdminPassword | b64enc }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.Secrets.Grafana.AdminPassword = "password"
				return cfg
			},
			expected: "encoded: cGFzc3dvcmQ=",
		},
		{
			name:         "default function with value",
			templateText: "size: {{ .OpenCenter.Services.loki.VolumeSize | default 20 }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["loki"] = &services.LokiConfig{
					VolumeSize: 50,
				}
				return cfg
			},
			expected: "size: 50",
		},
		{
			name:         "default function with empty value",
			templateText: "size: {{ .OpenCenter.Services.loki.VolumeSize | default 20 }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Services["loki"] = &services.LokiConfig{
					VolumeSize: 0,
				}
				return cfg
			},
			expected: "size: 20",
		},
		{
			name:         "printf function",
			templateText: "url: {{ printf \"https://auth.%s\" .OpenCenter.Cluster.ClusterFQDN }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterFQDN = "test.example.com"
				return cfg
			},
			expected: "url: https://auth.test.example.com",
		},
		{
			name:         "nested default with printf",
			templateText: "bucket: {{ .OpenCenter.Services.loki.BucketName | default (printf \"%s-loki\" .OpenCenter.Cluster.ClusterName) }}",
			setupConfig: func() Config {
				cfg := NewDefault("test")
				cfg.OpenCenter.Cluster.ClusterName = "my-cluster"
				cfg.OpenCenter.Services["loki"] = &services.LokiConfig{
					BucketName: "",
				}
				return cfg
			},
			expected: "bucket: my-cluster-loki",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()

			// Use Sprig functions
			tmpl, err := template.New("test").Funcs(sprig.TxtFuncMap()).Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestTemplateRenderingDefaultValues tests that default values work correctly in templates
func TestTemplateRenderingDefaultValues(t *testing.T) {
	cfg := NewDefault("test-cluster")

	tests := []struct {
		name         string
		templateText string
		expected     string
	}{
		{
			name:         "LetsEncryptServer with default",
			templateText: `{{ (index .OpenCenter.Services "cert-manager").LetsEncryptServer | default "https://acme-v02.api.letsencrypt.org/directory" }}`,
			expected:     "https://acme-v02.api.letsencrypt.org/directory",
		},
		{
			name:         "StorageClass with default",
			templateText: "{{ .OpenCenter.Services.loki.StorageClass | default .OpenCenter.Storage.DefaultStorageClass | default \"csi-cinder-sc-delete\" }}",
			expected:     "csi-cinder-sc-delete",
		},
		{
			name:         "VolumeSize with default",
			templateText: "{{ .OpenCenter.Services.loki.VolumeSize | default 20 }}Gi",
			expected:     "20Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Funcs(sprig.TxtFuncMap()).Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var buf strings.Builder
			if err := tmpl.Execute(&buf, cfg); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}

			result := buf.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestAWSCredentialFallback tests the new AWS credential structure with fallback logic
func TestAWSCredentialFallback(t *testing.T) {
	t.Run("GetAWSCredentials with new infrastructure credentials", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "infra-access-key"
		cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "infra-secret-key"

		accessKey, secretKey := cfg.GetAWSCredentials("", "")
		if accessKey != "infra-access-key" {
			t.Errorf("expected infra-access-key, got %s", accessKey)
		}
		if secretKey != "infra-secret-key" {
			t.Errorf("expected infra-secret-key, got %s", secretKey)
		}
	})

	t.Run("GetAWSApplicationCredentials with application credentials", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Application.AccessKey = "app-access-key"
		cfg.Secrets.Global.AWS.Application.SecretAccessKey = "app-secret-key"
		cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "infra-access-key"
		cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "infra-secret-key"

		accessKey, secretKey := cfg.GetAWSApplicationCredentials()
		if accessKey != "app-access-key" {
			t.Errorf("expected app-access-key, got %s", accessKey)
		}
		if secretKey != "app-secret-key" {
			t.Errorf("expected app-secret-key, got %s", secretKey)
		}
	})

	t.Run("GetAWSApplicationCredentials fallback to infrastructure", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "infra-access-key"
		cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "infra-secret-key"

		accessKey, secretKey := cfg.GetAWSApplicationCredentials()
		if accessKey != "infra-access-key" {
			t.Errorf("expected infra-access-key, got %s", accessKey)
		}
		if secretKey != "infra-secret-key" {
			t.Errorf("expected infra-secret-key, got %s", secretKey)
		}
	})

	t.Run("Service-specific credentials override global", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "infra-access-key"
		cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "infra-secret-key"

		accessKey, secretKey := cfg.GetAWSCredentials("service-access-key", "service-secret-key")
		if accessKey != "service-access-key" {
			t.Errorf("expected service-access-key, got %s", accessKey)
		}
		if secretKey != "service-secret-key" {
			t.Errorf("expected service-secret-key, got %s", secretKey)
		}
	})
}

// TestApplicationServicesUseApplicationCredentials tests that cert-manager, Loki, and Tempo use application credentials
func TestApplicationServicesUseApplicationCredentials(t *testing.T) {
	t.Run("cert-manager uses application credentials", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Application.AccessKey = "app-access-key"
		cfg.Secrets.Global.AWS.Application.SecretAccessKey = "app-secret-key"
		cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "infra-access-key"
		cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "infra-secret-key"

		accessKey, secretKey := cfg.GetCertManagerAWSCredentials()
		if accessKey != "app-access-key" {
			t.Errorf("expected app-access-key, got %s", accessKey)
		}
		if secretKey != "app-secret-key" {
			t.Errorf("expected app-secret-key, got %s", secretKey)
		}
	})

	t.Run("Loki uses application credentials", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Application.AccessKey = "app-access-key"
		cfg.Secrets.Global.AWS.Application.SecretAccessKey = "app-secret-key"
		cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "infra-access-key"
		cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "infra-secret-key"

		accessKey, secretKey := cfg.GetLokiS3Credentials()
		if accessKey != "app-access-key" {
			t.Errorf("expected app-access-key, got %s", accessKey)
		}
		if secretKey != "app-secret-key" {
			t.Errorf("expected app-secret-key, got %s", secretKey)
		}
	})

	t.Run("Tempo uses application credentials", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Application.AccessKey = "app-access-key"
		cfg.Secrets.Global.AWS.Application.SecretAccessKey = "app-secret-key"
		cfg.Secrets.Global.AWS.Infrastructure.AccessKey = "infra-access-key"
		cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey = "infra-secret-key"

		accessKey, secretKey := cfg.GetTempoS3Credentials()
		if accessKey != "app-access-key" {
			t.Errorf("expected app-access-key, got %s", accessKey)
		}
		if secretKey != "app-secret-key" {
			t.Errorf("expected app-secret-key, got %s", secretKey)
		}
	})

	t.Run("Service-specific credentials still take priority", func(t *testing.T) {
		cfg := NewDefault("test")
		cfg.Secrets.Global.AWS.Application.AccessKey = "app-access-key"
		cfg.Secrets.Global.AWS.Application.SecretAccessKey = "app-secret-key"
		cfg.Secrets.CertManager.AWSAccessKey = "cert-manager-access-key"
		cfg.Secrets.CertManager.AWSSecretAccessKey = "cert-manager-secret-key"

		accessKey, secretKey := cfg.GetCertManagerAWSCredentials()
		if accessKey != "cert-manager-access-key" {
			t.Errorf("expected cert-manager-access-key, got %s", accessKey)
		}
		if secretKey != "cert-manager-secret-key" {
			t.Errorf("expected cert-manager-secret-key, got %s", secretKey)
		}
	})
}
