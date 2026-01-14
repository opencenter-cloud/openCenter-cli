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
	"strings"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
)

// TestCrossFieldValidation tests all cross-field validation rules
// to ensure configuration conflicts are caught
func TestCrossFieldValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("WindowsWorkersEnabledButCountIsZero", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Windows workers but set count to 0
		config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows = 0
		config.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled = true

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when Windows workers enabled but count is 0")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "Windows workers enabled but worker_count_windows is 0") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about Windows workers conflict, got: %v", result.Errors)
		}
	})

	t.Run("WindowsWorkersDisabledWithPositiveCount", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Set Windows worker count but disable Windows workers
		config.OpenCenter.Cluster.Kubernetes.WorkerCountWindows = 2
		config.OpenCenter.Cluster.Kubernetes.WindowsWorkers.Enabled = false

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// This should be valid (count is set but feature is disabled)
		// No error expected - this is a valid configuration
		if !result.Valid {
			// Check if the only errors are unrelated to Windows workers
			for _, err := range result.Errors {
				if strings.Contains(err.Field, "windows") {
					t.Errorf("Unexpected Windows workers error: %s", err.Message)
				}
			}
		}
	})

	t.Run("S3BackendWithoutAWSCredentials", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable S3 backend without AWS credentials
		config.OpenTofu.Enabled = true
		config.OpenTofu.Backend.Type = "s3"
		config.OpenTofu.Backend.S3.Bucket = "mybucket"
		config.OpenTofu.Backend.S3.Key = "terraform.tfstate"
		config.OpenTofu.Backend.S3.Region = "us-west-2"
		config.OpenCenter.Cluster.AWSAccessKey = ""
		config.OpenCenter.Cluster.AWSSecretAccessKey = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when S3 backend is used without AWS credentials")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "AWS credentials required for S3 backend") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing AWS credentials for S3 backend, got: %v", result.Errors)
		}
	})

	t.Run("S3BackendWithAWSCredentials", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable S3 backend with AWS credentials
		config.OpenTofu.Enabled = true
		config.OpenTofu.Backend.Type = "s3"
		config.OpenTofu.Backend.S3.Bucket = "mybucket"
		config.OpenTofu.Backend.S3.Key = "terraform.tfstate"
		config.OpenTofu.Backend.S3.Region = "us-west-2"
		config.OpenCenter.Cluster.AWSAccessKey = "test-key"
		config.OpenCenter.Cluster.AWSSecretAccessKey = "test-secret"

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should not have S3 backend credential errors
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "AWS credentials required for S3 backend") {
				t.Errorf("Unexpected error about AWS credentials: %s", err.Message)
			}
		}
	})

	t.Run("KubeOVNCiliumIntegrationWithoutCilium", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Kube-OVN with Cilium integration but don't enable Cilium
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = false
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = true
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.CiliumIntegration = true

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should have a warning about Cilium integration without Cilium enabled
		foundWarning := false
		for _, warn := range result.Warnings {
			if strings.Contains(warn.Message, "Cilium integration enabled but Cilium is not enabled") {
				foundWarning = true
				break
			}
		}

		if !foundWarning {
			t.Errorf("Expected warning about Cilium integration without Cilium, got warnings: %v", result.Warnings)
		}
	})

	t.Run("KubeOVNCiliumIntegrationWithCilium", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Kube-OVN with Cilium integration and enable Cilium
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = true
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = false
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.CiliumIntegration = true

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should not have warning about Cilium integration
		for _, warn := range result.Warnings {
			if strings.Contains(warn.Message, "Cilium integration enabled but Cilium is not enabled") {
				t.Errorf("Unexpected warning about Cilium integration: %s", warn.Message)
			}
		}
	})

	t.Run("LokiSwiftStorageWithS3Config", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Loki with Swift storage type but provide S3 config
		lokiConfig := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "swift",
			BucketName:  "loki-chunks",
			// Swift config
			SwiftAuthURL:                  "https://auth.example.com/v3",
			SwiftRegion:                   "RegionOne",
			SwiftApplicationCredentialID:  "test-cred-id",
			// S3 config (should conflict)
			S3Region:   "us-west-2",
			S3Endpoint: "https://s3.example.com",
		}
		config.OpenCenter.Services["loki"] = lokiConfig
		config.Secrets.Loki.SwiftApplicationCredentialSecret = "test-secret"

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when both Swift and S3 configs are present")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "Cannot configure both S3 and Swift storage backends") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about conflicting storage backends, got: %v", result.Errors)
		}
	})

	t.Run("LokiS3StorageWithSwiftConfig", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Loki with S3 storage type but provide Swift config
		lokiConfig := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			BucketName:  "loki-chunks",
			// S3 config
			S3Region:   "us-west-2",
			S3Endpoint: "https://s3.example.com",
			// Swift config (should conflict)
			SwiftAuthURL:                  "https://auth.example.com/v3",
			SwiftRegion:                   "RegionOne",
			SwiftApplicationCredentialID:  "test-cred-id",
		}
		config.OpenCenter.Services["loki"] = lokiConfig
		config.Secrets.Loki.SwiftApplicationCredentialSecret = "test-secret"

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when both Swift and S3 configs are present")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "Cannot configure both S3 and Swift storage backends") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about conflicting storage backends, got: %v", result.Errors)
		}
	})

	t.Run("LokiStorageTypeMismatchSwift", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Set storage type to swift but only provide S3 config
		lokiConfig := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "swift",
			BucketName:  "loki-chunks",
			// Only S3 config
			S3Region:   "us-west-2",
			S3Endpoint: "https://s3.example.com",
		}
		config.OpenCenter.Services["loki"] = lokiConfig

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when storage type is swift but only S3 config is present")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "Storage type is set to 'swift' but only S3 configuration is present") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about storage type mismatch, got: %v", result.Errors)
		}
	})

	t.Run("LokiStorageTypeMismatchS3", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Set storage type to s3 but only provide Swift config
		lokiConfig := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType: "s3",
			BucketName:  "loki-chunks",
			// Only Swift config
			SwiftAuthURL:                  "https://auth.example.com/v3",
			SwiftRegion:                   "RegionOne",
			SwiftApplicationCredentialID:  "test-cred-id",
		}
		config.OpenCenter.Services["loki"] = lokiConfig
		config.Secrets.Loki.SwiftApplicationCredentialSecret = "test-secret"

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when storage type is s3 but only Swift config is present")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "Storage type is set to 's3' but only Swift configuration is present") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about storage type mismatch, got: %v", result.Errors)
		}
	})

	t.Run("LokiSwiftWithAppCredentials", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Loki with Swift storage and application credentials
		lokiConfig := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType:                   "swift",
			BucketName:                    "loki-chunks",
			SwiftAuthURL:                  "https://auth.example.com/v3",
			SwiftRegion:                   "RegionOne",
			SwiftApplicationCredentialID:  "test-cred-id",
		}
		config.OpenCenter.Services["loki"] = lokiConfig
		config.Secrets.Loki.SwiftApplicationCredentialSecret = "test-secret"

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should not have errors about Swift authentication
		for _, err := range result.Errors {
			if strings.Contains(err.Field, "loki") && strings.Contains(err.Message, "authentication credentials") {
				t.Errorf("Unexpected error about Swift authentication: %s", err.Message)
			}
		}
	})

	t.Run("LokiSwiftWithLegacyCredentials", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Loki with Swift storage and legacy username/password
		lokiConfig := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType:      "swift",
			BucketName:       "loki-chunks",
			SwiftAuthURL:     "https://auth.example.com/v3",
			SwiftRegion:      "RegionOne",
			SwiftUsername:    "test-user",
			SwiftProjectName: "test-project",
		}
		config.OpenCenter.Services["loki"] = lokiConfig
		config.Secrets.Loki.SwiftPassword = "test-password"

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should not have errors about Swift authentication
		for _, err := range result.Errors {
			if strings.Contains(err.Field, "loki") && strings.Contains(err.Message, "authentication credentials") {
				t.Errorf("Unexpected error about Swift authentication: %s", err.Message)
			}
		}
	})

	t.Run("LokiSwiftWithoutCredentials", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable Loki with Swift storage but no credentials
		lokiConfig := &services.LokiConfig{
			BaseConfig: services.BaseConfig{
				Enabled: true,
			},
			StorageType:  "swift",
			BucketName:   "loki-chunks",
			SwiftAuthURL: "https://auth.example.com/v3",
			SwiftRegion:  "RegionOne",
		}
		config.OpenCenter.Services["loki"] = lokiConfig

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when Swift storage is used without credentials")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "Swift authentication credentials are required") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing Swift credentials, got: %v", result.Errors)
		}
	})

	t.Run("VRRPValidationConflict", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable VRRP without Octavia and without IP
		config.Networking.UseOctavia = false
		config.Networking.VRRPEnabled = true
		config.Networking.VRRPIP = ""

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when VRRP is enabled without Octavia and without IP")
		}

		foundError := false
		for _, err := range result.Errors {
			if err.Field == "networking.vrrp_ip" && strings.Contains(err.Message, "vrrp_ip must be set when use_octavia is false") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about missing VRRP IP, got: %v", result.Errors)
		}
	})

	t.Run("MultipleNetworkPluginsEnabled", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Enable multiple network plugins
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = true
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = true
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = false

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when multiple network plugins are enabled")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "only one network plugin can be enabled") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about multiple network plugins, got: %v", result.Errors)
		}
	})

	t.Run("NoNetworkPluginEnabled", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecretsAndInfra(&config)

		// Disable all network plugins
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = false
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = false
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = false

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		if result.Valid {
			t.Error("Expected validation to fail when no network plugin is enabled")
		}

		foundError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "at least one network plugin must be enabled") {
				foundError = true
				break
			}
		}

		if !foundError {
			t.Errorf("Expected error about no network plugin enabled, got: %v", result.Errors)
		}
	})
}
