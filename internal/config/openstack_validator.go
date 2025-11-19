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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

// OpenStackValidator validates OpenStack-specific configuration.
type OpenStackValidator struct {
	httpClient *http.Client
}

// NewOpenStackValidator creates a new OpenStack validator.
func NewOpenStackValidator() *OpenStackValidator {
	return &OpenStackValidator{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidateCredentials validates OpenStack credentials.
func (v *OpenStackValidator) ValidateCredentials(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	os := config.OpenCenter.Infrastructure.Cloud.OpenStack

	// Validate application credentials
	if os.ApplicationCredentialID == "" || os.ApplicationCredentialSecret == "" {
		validationErrors = append(validationErrors, errors.CreateCredentialError(
			"OpenStack",
			"opencenter.infrastructure.cloud.openstack.application_credential_id",
			"application credentials are required",
			nil,
		))
	} else {
		// Validate application credential format
		if !v.isValidUUID(os.ApplicationCredentialID) {
			validationErrors = append(validationErrors, errors.CreateCredentialError(
				"OpenStack",
				"opencenter.infrastructure.cloud.openstack.application_credential_id",
				"application credential ID must be a valid UUID",
				nil,
			))
		}
	}

	return validationErrors
}

// ValidateConfiguration validates OpenStack configuration.
func (v *OpenStackValidator) ValidateConfiguration(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	os := config.OpenCenter.Infrastructure.Cloud.OpenStack

	// Validate auth URL
	if os.AuthURL == "" {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.auth_url",
			"OpenStack auth URL is required",
			"Set auth_url to your OpenStack Keystone endpoint",
			"Example: https://keystone.api.example.com/v3/",
		))
	} else if !v.isValidURL(os.AuthURL) {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.auth_url",
			"invalid auth URL format",
			"Ensure auth_url is a valid HTTP/HTTPS URL",
			"Example: https://keystone.api.example.com/v3/",
		))
	} else if !strings.Contains(os.AuthURL, "/v3") {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.auth_url",
			"auth URL should use Keystone v3 API",
			"Ensure auth_url includes '/v3' path",
			"Example: https://keystone.api.example.com/v3/",
		))
	}

	// Validate region
	if os.Region == "" {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.region",
			"OpenStack region is required",
			"Set region to your OpenStack region name",
			"Check with your OpenStack administrator for available regions",
		))
	}

	// Validate tenant/project
	if os.TenantName == "" {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.tenant_name",
			"OpenStack tenant name is required",
			"Set tenant_name to your OpenStack project/tenant name",
		))
	}

	// Validate domain information
	if os.Domain == "" {
		validationErrors = append(validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.domain",
			"domain is required",
			"Set domain to your OpenStack domain",
			"Use 'Default' for default domain",
		))
	}

	// Validate network configuration
	v.validateNetworkConfiguration(os, &validationErrors)

	return validationErrors
}

// ValidateConnectivity validates connectivity to OpenStack services.
func (v *OpenStackValidator) ValidateConnectivity(ctx context.Context, config *Config) []*errors.StructuredError {
	var validationErrors []*errors.StructuredError

	os := config.OpenCenter.Infrastructure.Cloud.OpenStack

	if os.AuthURL == "" {
		return validationErrors // Can't test connectivity without auth URL
	}

	// Test connectivity to Keystone
	req, err := http.NewRequestWithContext(ctx, "GET", os.AuthURL, nil)
	if err != nil {
		validationErrors = append(validationErrors, errors.CreateCloudError(
			"OpenStack",
			"connectivity test",
			fmt.Sprintf("failed to create request: %v", err),
			err,
		))
		return validationErrors
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		validationErrors = append(validationErrors, errors.CreateCloudError(
			"OpenStack",
			"connectivity test",
			fmt.Sprintf("failed to connect to auth URL: %v", err),
			err,
		))
		return validationErrors
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		validationErrors = append(validationErrors, errors.CreateCloudError(
			"OpenStack",
			"connectivity test",
			fmt.Sprintf("auth URL returned status %d", resp.StatusCode),
			fmt.Errorf("HTTP %d", resp.StatusCode),
		))
	}

	return validationErrors
}

// GetRequiredFields returns the list of required fields for OpenStack.
func (v *OpenStackValidator) GetRequiredFields() []string {
	return []string{
		"opencenter.infrastructure.cloud.openstack.auth_url",
		"opencenter.infrastructure.cloud.openstack.region",
		"opencenter.infrastructure.cloud.openstack.tenant_name",
		"opencenter.infrastructure.cloud.openstack.user_domain_name",
		"opencenter.infrastructure.cloud.openstack.project_domain_name",
	}
}

// validateNetworkConfiguration validates OpenStack network configuration.
func (v *OpenStackValidator) validateNetworkConfiguration(os SimplifiedOpenStackCloud, validationErrors *[]*errors.StructuredError) {
	// Validate floating network ID
	if os.FloatingNetworkId == "" {
		*validationErrors = append(*validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.floating_network_id",
			"floating network ID is required",
			"Set floating_network_id to your OpenStack external network ID",
			"Use 'openstack network list --external' to see external networks",
		))
	} else if !v.isValidUUID(os.FloatingNetworkId) {
		*validationErrors = append(*validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.floating_network_id",
			"invalid floating network ID format",
			"Use valid UUID format for floating_network_id",
			"Check OpenStack console for correct network ID",
		))
	}

	// Validate subnet ID if provided
	if os.SubnetId != "" && !v.isValidUUID(os.SubnetId) {
		*validationErrors = append(*validationErrors, errors.CreateValidationError(
			"opencenter.infrastructure.cloud.openstack.subnet_id",
			"invalid subnet ID format",
			"Use valid UUID format for subnet_id",
			"Check OpenStack console for correct subnet ID",
		))
	}
}

// Helper methods

func (v *OpenStackValidator) isValidUUID(uuid string) bool {
	// Basic UUID format validation
	if len(uuid) != 36 {
		return false
	}

	// Check for proper hyphen placement
	if uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		return false
	}

	// Check for valid hex characters
	validChars := "0123456789abcdefABCDEF-"
	for _, char := range uuid {
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

func (v *OpenStackValidator) isValidURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return parsedURL.Scheme == "http" || parsedURL.Scheme == "https"
}

func (v *OpenStackValidator) isValidIP(ip string) bool {
	// Simple IP validation - could be enhanced with net.ParseIP
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		// Check for IPv6 (basic check)
		return strings.Contains(ip, ":")
	}

	// IPv4 validation
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}

		num := 0
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
			num = num*10 + int(char-'0')
		}

		if num > 255 {
			return false
		}
	}

	return true
}
