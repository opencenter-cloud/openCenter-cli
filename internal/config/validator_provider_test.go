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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderSpecificValidation_OpenStack tests that OpenStack-specific validation is enforced
func TestProviderSpecificValidation_OpenStack(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	t.Run("missing auth URL", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = ""
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.auth_url"))
	})

	t.Run("missing region", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3/"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = ""
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.region"))
	})

	t.Run("missing tenant name", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3/"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = ""

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.tenant_name"))
	})

	t.Run("missing domain", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3/"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = ""

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.domain"))
	})

	t.Run("invalid auth URL format", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "not-a-valid-url"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.auth_url"))
	})

	t.Run("auth URL without v3", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.auth_url"))
	})

	t.Run("missing floating network ID", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3/"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = "Default"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.FloatingNetworkId = ""

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.floating_network_id"))
	})

	t.Run("invalid floating network ID format", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3/"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = "Default"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.FloatingNetworkId = "not-a-uuid"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.floating_network_id"))
	})

	t.Run("valid OpenStack configuration", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "openstack"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://keystone.example.com/v3/"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "RegionOne"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName = "test-tenant"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = "Default"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.FloatingNetworkId = "12345678-1234-1234-1234-123456789012"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "12345678-1234-1234-1234-123456789012"
		config.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "test-secret"

		result := validator.Validate(ctx, &config)

		// May have other validation errors, but should not have OpenStack-specific errors
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.auth_url"))
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.region"))
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.tenant_name"))
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.domain"))
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.openstack.floating_network_id"))
	})
}

// TestProviderSpecificValidation_AWS tests that AWS-specific validation is enforced
func TestProviderSpecificValidation_AWS(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	t.Run("missing region", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = ""

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.region"))
	})

	t.Run("invalid region format", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "invalid-region"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.region"))
	})

	t.Run("invalid VPC ID format", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
		config.OpenCenter.Infrastructure.Cloud.AWS.VPCID = "invalid-vpc-id"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.vpc_id"))
	})

	t.Run("invalid private subnet CIDR", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
		config.OpenCenter.Infrastructure.Cloud.AWS.PrivateSubnets = []string{"invalid-cidr"}

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.private_subnets[0]"))
	})

	t.Run("invalid public subnet CIDR", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
		config.OpenCenter.Infrastructure.Cloud.AWS.PublicSubnets = []string{"invalid-cidr"}

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.public_subnets[0]"))
	})

	t.Run("overlapping subnets", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
		config.OpenCenter.Infrastructure.Cloud.AWS.PrivateSubnets = []string{"10.0.1.0/24"}
		config.OpenCenter.Infrastructure.Cloud.AWS.PublicSubnets = []string{"10.0.1.0/24"}

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.private_subnets"))
	})

	t.Run("missing AWS credentials", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
		config.OpenCenter.Cluster.AWSAccessKey = ""
		config.OpenCenter.Cluster.AWSSecretAccessKey = ""

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.cluster.aws_access_key"))
	})

	t.Run("invalid AWS access key format", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
		config.OpenCenter.Cluster.AWSAccessKey = "invalid-key"
		config.OpenCenter.Cluster.AWSSecretAccessKey = "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890ABCD"

		result := validator.Validate(ctx, &config)

		require.False(t, result.Valid)
		assert.True(t, hasErrorForField(result.Errors, "opencenter.cluster.aws_access_key"))
	})

	t.Run("valid AWS configuration", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "aws"
		config.OpenCenter.Infrastructure.Cloud.AWS.Region = "us-east-1"
		config.OpenCenter.Infrastructure.Cloud.AWS.VPCID = "vpc-12345678"
		config.OpenCenter.Infrastructure.Cloud.AWS.PrivateSubnets = []string{"10.0.1.0/24"}
		config.OpenCenter.Infrastructure.Cloud.AWS.PublicSubnets = []string{"10.0.101.0/24"}
		config.OpenCenter.Cluster.AWSAccessKey = "AKIAIOSFODNN7EXAMPLE"
		config.OpenCenter.Cluster.AWSSecretAccessKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

		result := validator.Validate(ctx, &config)

		// May have other validation errors, but should not have AWS-specific errors
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.region"))
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.vpc_id"))
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.private_subnets[0]"))
		assert.False(t, hasErrorForField(result.Errors, "opencenter.infrastructure.cloud.aws.public_subnets[0]"))
	})
}

// TestProviderSpecificValidation_Kind tests that Kind provider has minimal requirements
func TestProviderSpecificValidation_Kind(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	t.Run("kind provider has minimal requirements", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "kind"

		result := validator.Validate(ctx, &config)

		// Kind should not have provider-specific validation errors
		// (it may have other validation errors from general validation)
		for _, err := range result.Errors {
			assert.NotContains(t, err.Field, "opencenter.infrastructure.cloud.kind")
		}
	})
}

// TestProviderSpecificValidation_UnknownProvider tests handling of unknown providers
func TestProviderSpecificValidation_UnknownProvider(t *testing.T) {
	ctx := context.Background()
	validator := NewConfigValidator(false)

	t.Run("unknown provider generates warning", func(t *testing.T) {
		config := NewDefault("test-cluster")
		config.OpenCenter.Infrastructure.Provider = "unknown-provider"

		result := validator.Validate(ctx, &config)

		// Should have a warning about unknown provider
		assert.True(t, hasWarningForField(result.Warnings, "opencenter.infrastructure.provider"))
	})
}

// Helper function to check if an error exists for a specific field
func hasErrorForField(errors []*ConfigValidationError, field string) bool {
	for _, err := range errors {
		if err.Field == field {
			return true
		}
	}
	return false
}

// Helper function to check if a warning exists for a specific field
func hasWarningForField(warnings []*ConfigValidationError, field string) bool {
	for _, warn := range warnings {
		if warn.Field == field {
			return true
		}
	}
	return false
}
