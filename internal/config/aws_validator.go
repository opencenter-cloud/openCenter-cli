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
	"fmt"
	"net"
	"regexp"

	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

// AWSValidator validates AWS-specific configuration.
type AWSValidator struct{}

// NewAWSValidator creates a new AWS validator.
func NewAWSValidator() *AWSValidator {
	return &AWSValidator{}
}

// ValidateCredentials validates AWS credentials.
func (v *AWSValidator) ValidateCredentials(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	// Validate AWS access key
	if config.OpenCenter.Cluster.AWSAccessKey == "" {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_access_key",
			"AWS access key is required",
			nil,
		))
	} else if !v.isValidAWSAccessKey(config.OpenCenter.Cluster.AWSAccessKey) {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_access_key",
			"invalid AWS access key format",
			nil,
		))
	}

	// Validate AWS secret access key
	if config.OpenCenter.Cluster.AWSSecretAccessKey == "" {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_secret_access_key",
			"AWS secret access key is required",
			nil,
		))
	} else if !v.isValidAWSSecretKey(config.OpenCenter.Cluster.AWSSecretAccessKey) {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_secret_access_key",
			"invalid AWS secret access key format",
			nil,
		))
	}

	return validationErrors
}

// ValidateConfiguration validates AWS configuration.
func (v *AWSValidator) ValidateConfiguration(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	aws := config.OpenCenter.Infrastructure.Cloud.AWS

	// Validate region
	if aws.Region == "" {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.aws.region",
			"AWS region is required",
			"Set region to an AWS region (e.g., 'us-east-1')",
			"Check AWS documentation for available regions",
		))
	} else if !v.isValidAWSRegion(aws.Region) {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.aws.region",
			fmt.Sprintf("invalid AWS region format: %s", aws.Region),
			"Use standard AWS region format (e.g., 'us-east-1', 'eu-west-1')",
			"Check AWS documentation for valid region names",
		))
	}

	// Validate VPC configuration
	v.validateVPCConfiguration(aws, &validationErrors)

	// Validate subnet configuration
	v.validateSubnetConfiguration(aws, &validationErrors)

	return validationErrors
}

// ValidateConnectivity validates connectivity to AWS services.
func (v *AWSValidator) ValidateConnectivity(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	// Note: Actual AWS API connectivity testing would require AWS SDK
	// For now, we'll perform basic validation that doesn't require network calls

	aws := config.OpenCenter.Infrastructure.Cloud.AWS

	// Validate that region is accessible (basic check)
	if aws.Region != "" && !v.isValidAWSRegion(aws.Region) {
		validationErrors = append(validationErrors, errors.CreateCloudError(
			"AWS",
			"region validation",
			fmt.Sprintf("region %s may not be accessible", aws.Region),
			nil,
		))
	}

	return validationErrors
}

// GetRequiredFields returns the list of required fields for AWS.
func (v *AWSValidator) GetRequiredFields() []string {
	return []string{
		"opencenter.infrastructure.cloud.aws.region",
		"opencenter.cluster.aws_access_key",
		"opencenter.cluster.aws_secret_access_key",
	}
}

// validateVPCConfiguration validates AWS VPC configuration.
func (v *AWSValidator) validateVPCConfiguration(aws SimplifiedAWSCloud, validationErrors *[]*errors.StructuredError) {
	// VPC ID validation
	if aws.VPCID != "" && !v.isValidVPCID(aws.VPCID) {
		*validationErrors = append(*validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.aws.vpc_id",
			fmt.Sprintf("invalid VPC ID format: %s", aws.VPCID),
			"Use valid VPC ID format (vpc-xxxxxxxxx)",
			"Check AWS console for correct VPC ID",
		))
	}
}

// validateSubnetConfiguration validates AWS subnet configuration.
func (v *AWSValidator) validateSubnetConfiguration(aws SimplifiedAWSCloud, validationErrors *[]*errors.StructuredError) {
	// Validate private subnets
	for i, subnet := range aws.PrivateSubnets {
		if !v.isValidCIDR(subnet) {
			*validationErrors = append(*validationErrors, errors.CreateValidationError(
				fmt.Sprintf("opencenter.infrastructure.cloud.aws.private_subnets[%d]", i),
				fmt.Sprintf("invalid private subnet CIDR: %s", subnet),
				"Use valid CIDR notation for private subnets",
				"Example: '10.0.1.0/24'",
			))
		}
	}

	// Validate public subnets
	for i, subnet := range aws.PublicSubnets {
		if !v.isValidCIDR(subnet) {
			*validationErrors = append(*validationErrors, errors.CreateValidationError(
				fmt.Sprintf("opencenter.infrastructure.cloud.aws.public_subnets[%d]", i),
				fmt.Sprintf("invalid public subnet CIDR: %s", subnet),
				"Use valid CIDR notation for public subnets",
				"Example: '10.0.101.0/24'",
			))
		}
	}

	// Check for subnet overlap
	allSubnets := append(aws.PrivateSubnets, aws.PublicSubnets...)
	for i, subnet1 := range allSubnets {
		for j, subnet2 := range allSubnets {
			if i != j && v.subnetsOverlap(subnet1, subnet2) {
				*validationErrors = append(*validationErrors, errors.CreateValidationError(
					"opencenter.infrastructure.cloud.aws.private_subnets",
					fmt.Sprintf("subnets overlap: %s and %s", subnet1, subnet2),
					"Ensure all subnet CIDRs are non-overlapping",
					"Use subnet calculator to plan CIDR ranges",
				))
				break
			}
		}
	}
}

// Helper methods for AWS validation

func (v *AWSValidator) isValidAWSAccessKey(key string) bool {
	// AWS access keys are typically 20 characters, alphanumeric
	if len(key) != 20 {
		return false
	}

	for _, char := range key {
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
	}

	return true
}

func (v *AWSValidator) isValidAWSSecretKey(key string) bool {
	// AWS secret keys are typically 40 characters, base64-like
	if len(key) != 40 {
		return false
	}

	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	for _, char := range key {
		found := false
		for _, validChar := range validChars {
			if char == validChar {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (v *AWSValidator) isValidAWSRegion(region string) bool {
	// Basic AWS region format validation
	regionPattern := regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)
	return regionPattern.MatchString(region)
}

func (v *AWSValidator) isValidVPCID(vpcID string) bool {
	// VPC ID format: vpc-xxxxxxxxx
	vpcPattern := regexp.MustCompile(`^vpc-[a-f0-9]{8,17}$`)
	return vpcPattern.MatchString(vpcID)
}

func (v *AWSValidator) isValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func (v *AWSValidator) subnetsOverlap(cidr1, cidr2 string) bool {
	_, net1, err1 := net.ParseCIDR(cidr1)
	_, net2, err2 := net.ParseCIDR(cidr2)

	if err1 != nil || err2 != nil {
		return false
	}

	return net1.Contains(net2.IP) || net2.Contains(net1.IP)
}
