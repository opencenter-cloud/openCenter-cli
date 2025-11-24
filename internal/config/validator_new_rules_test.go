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
)

// TestNewValidationRules tests the three new validation rules:
// 1. Secrets are validated only if the service/managed service is enabled
// 2. Only one CNI can be enabled at a time
// 3. S3 bucket name must be lowercase and defaults to organization
func TestNewValidationRules(t *testing.T) {
	ctx := context.Background()

	t.Run("SecretsValidatedOnlyWhenServiceEnabled", func(t *testing.T) {
		// Test that secrets are NOT required when service is disabled
		config := NewDefault("test-cluster")
		config.OpenCenter.Services["headlamp"] = ServiceCfg{Enabled: false}
		config.OpenCenter.Services["cert-manager"] = ServiceCfg{Enabled: false}
		config.OpenCenter.Services["keycloak"] = ServiceCfg{Enabled: false}
		config.OpenCenter.Services["kube-prometheus-stack"] = ServiceCfg{Enabled: false}
		config.OpenCenter.Services["weave-gitops"] = ServiceCfg{Enabled: false}
		config.OpenCenter.ManagedService["alert-proxy"] = ServiceCfg{Enabled: false}

		// Clear all secrets
		config.Secrets = Secrets{}

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should be valid since all services are disabled
		if !result.Valid {
			t.Errorf("Configuration should be valid when services are disabled, but got errors: %v", result.Errors)
		}

		// Test that secrets ARE required when service is enabled
		config.OpenCenter.Services["headlamp"] = ServiceCfg{Enabled: true}
		result = validator.Validate(ctx, &config)

		// Should be invalid since headlamp is enabled but secrets are missing
		if result.Valid {
			t.Error("Configuration should be invalid when headlamp is enabled without secrets")
		}

		foundHeadlampError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Field, "headlamp") {
				foundHeadlampError = true
				break
			}
		}

		if !foundHeadlampError {
			t.Errorf("Expected error about missing headlamp secrets, but got: %v", result.Errors)
		}
	})

	t.Run("OnlyOneCNICanBeEnabled", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecrets(&config)

		// Enable multiple CNIs
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = true
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = true
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = false

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should be invalid since multiple CNIs are enabled
		if result.Valid {
			t.Error("Configuration should be invalid when multiple CNIs are enabled")
		}

		foundCNIError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "only one network plugin can be enabled") {
				foundCNIError = true
				break
			}
		}

		if !foundCNIError {
			t.Errorf("Expected error about multiple CNIs enabled, but got: %v", result.Errors)
		}

		// Test that exactly one CNI is valid
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled = true
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled = false
		config.OpenCenter.Cluster.Kubernetes.NetworkPlugin.KubeOVN.Enabled = false

		result = validator.Validate(ctx, &config)

		// Should be valid with only one CNI enabled
		if !result.Valid {
			t.Errorf("Configuration should be valid with only one CNI enabled, but got errors: %v", result.Errors)
		}
	})

	t.Run("S3BucketMustBeLowercase", func(t *testing.T) {
		config := NewDefault("test-cluster")
		addDefaultSecrets(&config)

		// Enable OpenTofu with S3 backend
		config.OpenTofu.Enabled = true
		config.OpenTofu.Backend.Type = "s3"
		config.OpenTofu.Backend.S3.Bucket = "MyBucket"
		config.OpenTofu.Backend.S3.Key = "terraform.tfstate"
		config.OpenTofu.Backend.S3.Region = "us-west-2"

		// Add AWS credentials for S3 backend
		config.OpenCenter.Cluster.AWSAccessKey = "test-key"
		config.OpenCenter.Cluster.AWSSecretAccessKey = "test-secret"

		validator := NewConfigValidator(false)
		result := validator.Validate(ctx, &config)

		// Should be invalid since bucket name has uppercase letters
		if result.Valid {
			t.Error("Configuration should be invalid when S3 bucket name has uppercase letters")
		}

		foundBucketError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Field, "s3.bucket") && strings.Contains(err.Message, "lowercase") {
				foundBucketError = true
				break
			}
		}

		if !foundBucketError {
			t.Errorf("Expected error about S3 bucket name not being lowercase, but got: %v", result.Errors)
		}

		// Test that lowercase bucket name is valid
		config.OpenTofu.Backend.S3.Bucket = "mybucket"

		result = validator.Validate(ctx, &config)

		// Should be valid with lowercase bucket name
		if !result.Valid {
			t.Errorf("Configuration should be valid with lowercase S3 bucket name, but got errors: %v", result.Errors)
		}
	})

	t.Run("S3BucketDefaultsToOrganization", func(t *testing.T) {
		// Test that S3 bucket defaults to organization name (lowercase)
		config := NewDefault("test-cluster")
		config.OpenCenter.Meta.Organization = "MyOrganization"
		
		// Set backend to S3 and initialize bucket with cluster name (simulating old default)
		config.OpenTofu.Backend.Type = "s3"
		config.OpenTofu.Backend.S3.Bucket = strings.ToLower(config.ClusterName())
		config.OpenTofu.Backend.S3.Key = "terraform.tfstate"
		config.OpenTofu.Backend.S3.Region = "us-west-2"

		// Simulate loading config which applies organization defaults
		applyOrganizationDefaults(&config)

		// Bucket should be set to lowercase organization name
		expectedBucket := "myorganization"
		if config.OpenTofu.Backend.S3.Bucket != expectedBucket {
			t.Errorf("Expected S3 bucket to be '%s', but got '%s'", expectedBucket, config.OpenTofu.Backend.S3.Bucket)
		}
	})
}
