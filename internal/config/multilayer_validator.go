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
	"fmt"
	"net"
	"strings"

	"github.com/go-playground/validator/v10"
)

// V2ValidationError represents a structured validation error with code, field path, and message
type V2ValidationError struct {
	Code    string // Error code (E001-E013)
	Field   string // Field path (e.g., "infrastructure.networking.vrrp_ip")
	Message string // Human-readable error message
}

// Error implements the error interface
func (e V2ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Field, e.Message)
}

// V2Validator interface defines multi-layered validation methods
type V2Validator interface {
	// Validate performs all validation layers
	Validate(cfg *Config) []V2ValidationError
	
	// ValidateSchema performs schema validation (required fields, data types, enum values)
	ValidateSchema(cfg *Config) []V2ValidationError
	
	// ValidateBusinessRules performs business rule validation (cross-field dependencies, value ranges)
	ValidateBusinessRules(cfg *Config) []V2ValidationError
	
	// ValidateProvider performs provider-specific validation
	ValidateProvider(cfg *Config) []V2ValidationError
	
	// ValidateDeployment performs deployment-method validation
	ValidateDeployment(cfg *Config) []V2ValidationError
	
	// ValidateServices performs service dependency validation
	ValidateServices(cfg *Config) []V2ValidationError
}

// multiLayerValidator implements the V2Validator interface
type multiLayerValidator struct {
	validate *validator.Validate
}

// NewMultiLayerValidator creates a new multi-layered validator
func NewMultiLayerValidator() V2Validator {
	v := validator.New()
	
	// Register custom validation functions
	registerCustomValidations(v)
	
	return &multiLayerValidator{
		validate: v,
	}
}

// registerCustomValidations registers custom validation functions
func registerCustomValidations(v *validator.Validate) {
	// Register dns1123 validation
	v.RegisterValidation("dns1123", validateDNS1123)
	
	// Register cidrv4 validation
	v.RegisterValidation("cidrv4", validateCIDRv4)
	
	// Register semver validation
	v.RegisterValidation("semver", validateSemVer)
	
	// Register fqdn validation (already built-in, but we can override)
	// validator already has fqdn, so we don't need to register it
}

// validateDNS1123 validates DNS-1123 compliant names
func validateDNS1123(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // Empty is valid for optional fields
	}
	
	// DNS-1123 label: lowercase alphanumeric, hyphens, max 63 chars
	// Must start and end with alphanumeric
	if len(value) > 63 {
		return false
	}
	
	for i, c := range value {
		if i == 0 || i == len(value)-1 {
			// First and last must be alphanumeric
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				return false
			}
		} else {
			// Middle can be alphanumeric or hyphen
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	
	return true
}

// validateCIDRv4 validates IPv4 CIDR notation
func validateCIDRv4(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // Empty is valid for optional fields
	}
	
	_, _, err := net.ParseCIDR(value)
	return err == nil
}

// validateSemVer validates semantic versioning
func validateSemVer(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // Empty is valid for optional fields
	}
	
	// Basic semver pattern: v?X.Y.Z or v?X.Y.Z-suffix
	// Remove leading 'v' if present
	value = strings.TrimPrefix(value, "v")
	
	// Split by '.' and '-'
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return false
	}
	
	// Check that first parts are numeric
	for i := 0; i < 2 && i < len(parts); i++ {
		part := parts[i]
		// May have suffix after hyphen
		if strings.Contains(part, "-") {
			part = strings.Split(part, "-")[0]
		}
		if part == "" {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	
	return true
}

// Validate performs all validation layers
func (v *multiLayerValidator) Validate(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// Layer 1: Schema validation
	errors = append(errors, v.ValidateSchema(cfg)...)
	
	// Layer 2: Business rules validation
	errors = append(errors, v.ValidateBusinessRules(cfg)...)
	
	// Layer 3: Provider-specific validation
	errors = append(errors, v.ValidateProvider(cfg)...)
	
	// Layer 4: Deployment-method validation
	errors = append(errors, v.ValidateDeployment(cfg)...)
	
	// Layer 5: Service dependency validation
	errors = append(errors, v.ValidateServices(cfg)...)
	
	return errors
}

// ValidateSchema performs schema validation using go-playground/validator
func (v *multiLayerValidator) ValidateSchema(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	err := v.validate.Struct(cfg)
	if err == nil {
		return errors
	}
	
	// Convert validator errors to V2ValidationError
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		errors = append(errors, V2ValidationError{
			Code:    "E001",
			Field:   "config",
			Message: fmt.Sprintf("validation error: %v", err),
		})
		return errors
	}
	
	for _, fieldErr := range validationErrors {
		errors = append(errors, V2ValidationError{
			Code:    getErrorCode(fieldErr.Tag()),
			Field:   fieldErr.Namespace(),
			Message: getErrorMessage(fieldErr),
		})
	}
	
	return errors
}

// ValidateBusinessRules performs business rule validation
func (v *multiLayerValidator) ValidateBusinessRules(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// Validate CIDR containment (allocation pool within subnet)
	if cfg.OpenCenter.Cluster.Networking.SubnetNodes != "" {
		_, subnet, err := net.ParseCIDR(cfg.OpenCenter.Cluster.Networking.SubnetNodes)
		if err == nil {
			// Check allocation pool start
			if cfg.OpenCenter.Cluster.Networking.AllocationPoolStart != "" {
				startIP := net.ParseIP(cfg.OpenCenter.Cluster.Networking.AllocationPoolStart)
				if startIP != nil && !subnet.Contains(startIP) {
					errors = append(errors, V2ValidationError{
						Code:    "E005",
						Field:   "opencenter.cluster.networking.allocation_pool_start",
						Message: "allocation pool start IP is not within subnet_nodes range",
					})
				}
			}
			
			// Check allocation pool end
			if cfg.OpenCenter.Cluster.Networking.AllocationPoolEnd != "" {
				endIP := net.ParseIP(cfg.OpenCenter.Cluster.Networking.AllocationPoolEnd)
				if endIP != nil && !subnet.Contains(endIP) {
					errors = append(errors, V2ValidationError{
						Code:    "E005",
						Field:   "opencenter.cluster.networking.allocation_pool_end",
						Message: "allocation pool end IP is not within subnet_nodes range",
					})
				}
			}
		}
	}
	
	// Validate VRRP IP is required when VRRP is enabled
	if cfg.OpenCenter.Cluster.Networking.VRRPEnabled && cfg.OpenCenter.Cluster.Networking.VRRPIP == "" {
		errors = append(errors, V2ValidationError{
			Code:    "E006",
			Field:   "opencenter.cluster.networking.vrrp_ip",
			Message: "VRRP IP is required when VRRP is enabled",
		})
	}
	
	// Validate node counts
	if cfg.OpenCenter.Cluster.Kubernetes.MasterCount < 0 {
		errors = append(errors, V2ValidationError{
			Code:    "E007",
			Field:   "opencenter.cluster.kubernetes.master_count",
			Message: "master count cannot be negative",
		})
	}
	
	if cfg.OpenCenter.Cluster.Kubernetes.WorkerCount < 0 {
		errors = append(errors, V2ValidationError{
			Code:    "E007",
			Field:   "opencenter.cluster.kubernetes.worker_count",
			Message: "worker count cannot be negative",
		})
	}
	
	return errors
}

// ValidateProvider performs provider-specific validation
func (v *multiLayerValidator) ValidateProvider(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	provider := cfg.OpenCenter.Infrastructure.Provider
	
	switch provider {
	case "openstack":
		errors = append(errors, v.validateOpenStackProvider(cfg)...)
	case "aws":
		errors = append(errors, v.validateAWSProvider(cfg)...)
	case "gcp":
		errors = append(errors, v.validateGCPProvider(cfg)...)
	case "azure":
		errors = append(errors, v.validateAzureProvider(cfg)...)
	default:
		errors = append(errors, V2ValidationError{
			Code:    "E008",
			Field:   "opencenter.infrastructure.provider",
			Message: fmt.Sprintf("unsupported provider: %s", provider),
		})
	}
	
	return errors
}

// validateOpenStackProvider validates OpenStack-specific configuration
func (v *multiLayerValidator) validateOpenStackProvider(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// Check that OpenStack configuration is provided
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL == "" {
		errors = append(errors, V2ValidationError{
			Code:    "E009",
			Field:   "opencenter.infrastructure.cloud.openstack.auth_url",
			Message: "OpenStack auth_url is required when provider is openstack",
		})
	}
	
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region == "" {
		errors = append(errors, V2ValidationError{
			Code:    "E009",
			Field:   "opencenter.infrastructure.cloud.openstack.region",
			Message: "OpenStack region is required when provider is openstack",
		})
	}
	
	return errors
}

// validateAWSProvider validates AWS-specific configuration
func (v *multiLayerValidator) validateAWSProvider(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// Check that AWS configuration is provided
	if cfg.OpenCenter.Infrastructure.Cloud.AWS.Region == "" {
		errors = append(errors, V2ValidationError{
			Code:    "E009",
			Field:   "opencenter.infrastructure.cloud.aws.region",
			Message: "AWS region is required when provider is aws",
		})
	}
	
	return errors
}

// validateGCPProvider validates GCP-specific configuration
func (v *multiLayerValidator) validateGCPProvider(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// GCP validation would go here
	// For now, just a placeholder
	
	return errors
}

// validateAzureProvider validates Azure-specific configuration
func (v *multiLayerValidator) validateAzureProvider(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// Azure validation would go here
	// For now, just a placeholder
	
	return errors
}

// ValidateDeployment performs deployment-method validation
func (v *multiLayerValidator) ValidateDeployment(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// Deployment validation would go here
	// For now, just a placeholder
	
	return errors
}

// ValidateServices performs service dependency validation
func (v *multiLayerValidator) ValidateServices(cfg *Config) []V2ValidationError {
	var errors []V2ValidationError
	
	// Service dependency validation would go here
	// For now, just a placeholder
	
	return errors
}

// getErrorCode maps validation tags to error codes
func getErrorCode(tag string) string {
	switch tag {
	case "required":
		return "E001"
	case "oneof":
		return "E002"
	case "cidrv4":
		return "E003"
	case "ipv4":
		return "E004"
	case "min", "max":
		return "E007"
	case "email":
		return "E010"
	case "fqdn":
		return "E011"
	case "url":
		return "E012"
	case "dns1123":
		return "E013"
	default:
		return "E000"
	}
}

// getErrorMessage generates a human-readable error message
func getErrorMessage(fieldErr validator.FieldError) string {
	field := fieldErr.Field()
	tag := fieldErr.Tag()
	param := fieldErr.Param()
	
	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, param)
	case "cidrv4":
		return fmt.Sprintf("%s must be a valid IPv4 CIDR notation", field)
	case "ipv4":
		return fmt.Sprintf("%s must be a valid IPv4 address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, param)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "fqdn":
		return fmt.Sprintf("%s must be a valid fully qualified domain name", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "dns1123":
		return fmt.Sprintf("%s must be a valid DNS-1123 name", field)
	default:
		return fmt.Sprintf("%s failed validation: %s", field, tag)
	}
}
