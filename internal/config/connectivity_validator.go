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
	"net/http"
	"net/url"
	"strings"
	"time"

	configvalidation "github.com/opencenter-cloud/opencenter-cli/internal/config/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
)

// ConnectivityValidator provides pre-flight connectivity checks for cloud providers.
type ConnectivityValidator struct {
	httpClient *http.Client
	timeout    time.Duration
}

// NewConnectivityValidator creates a new connectivity validator.
func NewConnectivityValidator(timeout time.Duration) *ConnectivityValidator {
	return &ConnectivityValidator{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// ValidateCloudProviderConnectivity performs comprehensive connectivity validation for cloud providers.
func (cv *ConnectivityValidator) ValidateCloudProviderConnectivity(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	provider := config.OpenCenter.Infrastructure.Provider

	switch provider {
	case "openstack":
		errors := cv.validateOpenStackConnectivity(ctx, config)
		validationErrors = append(validationErrors, errors...)
	case "aws":
		errors := cv.validateAWSConnectivity(ctx, config)
		validationErrors = append(validationErrors, errors...)
	case "kind":
		// Kind runs locally, no connectivity checks needed
		break
	default:
		validationErrors = append(validationErrors, errors.CreateCloudError(
			provider,
			"connectivity validation",
			fmt.Sprintf("unknown cloud provider: %s", provider),
			nil,
		))
	}

	return validationErrors
}

// validateOpenStackConnectivity validates connectivity to OpenStack services.
func (cv *ConnectivityValidator) validateOpenStackConnectivity(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	os := config.OpenCenter.Infrastructure.Cloud.OpenStack

	if os.AuthURL == "" {
		return validationErrors // Can't test connectivity without auth URL
	}

	// Test connectivity to Keystone
	if err := cv.testHTTPConnectivity(ctx, os.AuthURL); err != nil {
		validationErrors = append(validationErrors, errors.CreateCloudError(
			"OpenStack",
			"Keystone connectivity",
			fmt.Sprintf("failed to connect to auth URL %s: %v", os.AuthURL, err),
			err,
		))
	}

	// Test DNS resolution for auth URL
	if err := cv.testDNSResolution(ctx, os.AuthURL); err != nil {
		validationErrors = append(validationErrors, errors.CreateCloudError(
			"OpenStack",
			"DNS resolution",
			fmt.Sprintf("failed to resolve auth URL %s: %v", os.AuthURL, err),
			err,
		))
	}

	return validationErrors
}

// validateAWSConnectivity validates connectivity to AWS services.
func (cv *ConnectivityValidator) validateAWSConnectivity(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	aws := config.OpenCenter.Infrastructure.Cloud.AWS

	if aws.Region == "" {
		return validationErrors // Can't test connectivity without region
	}

	// Test connectivity to AWS endpoints
	awsEndpoints := []string{
		fmt.Sprintf("https://ec2.%s.amazonaws.com", aws.Region),
		fmt.Sprintf("https://s3.%s.amazonaws.com", aws.Region),
		fmt.Sprintf("https://iam.amazonaws.com"),
	}

	for _, endpoint := range awsEndpoints {
		if err := cv.testHTTPConnectivity(ctx, endpoint); err != nil {
			validationErrors = append(validationErrors, errors.CreateCloudError(
				"AWS",
				"service connectivity",
				fmt.Sprintf("failed to connect to %s: %v", endpoint, err),
				err,
			))
		}
	}

	// Test DNS resolution for AWS endpoints
	for _, endpoint := range awsEndpoints {
		if err := cv.testDNSResolution(ctx, endpoint); err != nil {
			validationErrors = append(validationErrors, errors.CreateCloudError(
				"AWS",
				"DNS resolution",
				fmt.Sprintf("failed to resolve %s: %v", endpoint, err),
				err,
			))
		}
	}

	return validationErrors
}

// testHTTPConnectivity tests HTTP connectivity to a given URL.
func (cv *ConnectivityValidator) testHTTPConnectivity(ctx context.Context, rawURL string) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", rawURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := cv.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Accept any response that indicates the server is reachable
	// Even 4xx errors indicate the server is responding
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: HTTP %d", resp.StatusCode)
	}

	return nil
}

// testDNSResolution tests DNS resolution for a given URL.
func (cv *ConnectivityValidator) testDNSResolution(ctx context.Context, rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("no hostname in URL")
	}

	// Test DNS resolution
	resolver := &net.Resolver{}
	_, err = resolver.LookupHost(ctx, hostname)
	if err != nil {
		return fmt.Errorf("DNS resolution failed: %w", err)
	}

	return nil
}

// ValidateCredentialFormat validates the format of cloud provider credentials.
func (cv *ConnectivityValidator) ValidateCredentialFormat(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	provider := config.OpenCenter.Infrastructure.Provider

	switch provider {
	case "openstack":
		errors := cv.validateOpenStackCredentialFormat(config)
		validationErrors = append(validationErrors, errors...)
	case "aws":
		errors := cv.validateAWSCredentialFormat(config)
		validationErrors = append(validationErrors, errors...)
	}

	return validationErrors
}

// validateOpenStackCredentialFormat validates OpenStack credential format.
func (cv *ConnectivityValidator) validateOpenStackCredentialFormat(config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	os := config.OpenCenter.Infrastructure.Cloud.OpenStack

	// Validate application credential ID format (should be UUID)
	if os.ApplicationCredentialID != "" && !cv.isValidUUID(os.ApplicationCredentialID) {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"OpenStack",
			"opencenter.infrastructure.cloud.openstack.application_credential_id",
			"application credential ID must be a valid UUID",
			nil,
		))
	}

	// Validate application credential secret (should not be empty if ID is provided)
	if os.ApplicationCredentialID != "" && os.ApplicationCredentialSecret == "" {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"OpenStack",
			"opencenter.infrastructure.cloud.openstack.application_credential_secret",
			"application credential secret is required when ID is provided",
			nil,
		))
	}

	return validationErrors
}

// validateAWSCredentialFormat validates AWS credential format.
func (cv *ConnectivityValidator) validateAWSCredentialFormat(config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	// Validate AWS access key format
	if config.OpenCenter.Cluster.AWSAccessKey != "" && !cv.isValidAWSAccessKey(config.OpenCenter.Cluster.AWSAccessKey) {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_access_key",
			"invalid AWS access key format",
			nil,
		))
	}

	// Validate AWS secret access key format
	if config.OpenCenter.Cluster.AWSSecretAccessKey != "" && !cv.isValidAWSSecretKey(config.OpenCenter.Cluster.AWSSecretAccessKey) {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_secret_access_key",
			"invalid AWS secret access key format",
			nil,
		))
	}

	return validationErrors
}

// Helper methods for credential format validation

func (cv *ConnectivityValidator) isValidUUID(uuid string) bool {
	return configvalidation.IsValidUUID(uuid)
}

func (cv *ConnectivityValidator) isValidAWSAccessKey(key string) bool {
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

func (cv *ConnectivityValidator) isValidAWSSecretKey(key string) bool {
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

// ValidateCredentialSecurity checks for common security issues with credentials.
func (cv *ConnectivityValidator) ValidateCredentialSecurity(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	// Check for hardcoded credentials (security warning)
	if config.OpenCenter.Cluster.AWSAccessKey != "" && !strings.Contains(config.OpenCenter.Cluster.AWSAccessKey, "ENC[") {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_access_key",
			"AWS access key appears to be in plaintext",
			fmt.Errorf("credential security issue"),
		))
	}

	if config.OpenCenter.Cluster.AWSSecretAccessKey != "" && !strings.Contains(config.OpenCenter.Cluster.AWSSecretAccessKey, "ENC[") {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"AWS",
			"opencenter.cluster.aws_secret_access_key",
			"AWS secret access key appears to be in plaintext",
			fmt.Errorf("credential security issue"),
		))
	}

	os := config.OpenCenter.Infrastructure.Cloud.OpenStack
	if os.ApplicationCredentialSecret != "" && !strings.Contains(os.ApplicationCredentialSecret, "ENC[") {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"OpenStack",
			"opencenter.infrastructure.cloud.openstack.application_credential_secret",
			"OpenStack application credential secret appears to be in plaintext",
			fmt.Errorf("credential security issue"),
		))
	}

	return validationErrors
}
