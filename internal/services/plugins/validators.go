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

package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
)

// CertManagerValidator validates cert-manager service configuration
type CertManagerValidator struct {
	*validators.ServiceValidator
}

// NewCertManagerValidator creates a new cert-manager validator
func NewCertManagerValidator() *CertManagerValidator {
	baseValidator := validators.NewServiceValidator("cert-manager")
	validator := &CertManagerValidator{
		ServiceValidator: baseValidator,
	}
	// Set self as extension validator to handle cert-manager specific validation
	baseValidator.SetExtensionValidator(validator)
	return validator
}

// Name returns the validator name
func (v *CertManagerValidator) Name() string {
	return "service:cert-manager"
}

// Validate validates cert-manager configuration
func (v *CertManagerValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := validation.NewValidationResult()

	cfg, ok := value.(*services.CertManagerConfig)
	if !ok {
		result.AddError("cert-manager", "invalid config type for cert-manager: expected *CertManagerConfig")
		return result, nil
	}

	// Validate LetsEncrypt server URL if specified
	if cfg.LetsEncryptServer != "" {
		if !strings.HasPrefix(cfg.LetsEncryptServer, "https://") {
			result.AddError("cert-manager.letsencrypt_server",
				"letsencrypt_server must be an HTTPS URL",
				"Use HTTPS URL for LetsEncrypt server",
				"Example: https://acme-v02.api.letsencrypt.org/directory")
		}
	}

	// Validate email if specified
	if cfg.Email != "" && !strings.Contains(cfg.Email, "@") {
		result.AddError("cert-manager.email",
			"email must be a valid email address",
			"Provide a valid email address",
			"Example: admin@example.com")
	}

	return result, nil
}

// KeycloakValidator validates Keycloak service configuration
type KeycloakValidator struct {
	*validators.ServiceValidator
}

// NewKeycloakValidator creates a new Keycloak validator
func NewKeycloakValidator() *KeycloakValidator {
	baseValidator := validators.NewServiceValidator("keycloak")
	validator := &KeycloakValidator{
		ServiceValidator: baseValidator,
	}
	baseValidator.SetExtensionValidator(validator)
	return validator
}

// Name returns the validator name
func (v *KeycloakValidator) Name() string {
	return "service:keycloak"
}

// Validate validates Keycloak configuration
func (v *KeycloakValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := validation.NewValidationResult()

	cfg, ok := value.(*services.KeycloakConfig)
	if !ok {
		result.AddError("keycloak", "invalid config type for keycloak: expected *KeycloakConfig")
		return result, nil
	}

	if !cfg.IsEnabled() {
		return result, nil
	}

	// Validate production mode settings
	if cfg.StartOptimized && cfg.Instances < 2 {
		result.AddError("keycloak.instances",
			"production mode (start_optimized=true) requires at least 2 instances for high availability",
			"Increase instances to at least 2",
			"Or disable production mode: start_optimized: false")
	}

	// Validate instance count
	if cfg.Instances < 1 {
		result.AddError("keycloak.instances",
			"instances must be at least 1",
			"Set instances to at least 1")
	}

	// Validate autoscaling configuration
	if cfg.MinReplicas > 0 && cfg.MaxReplicas > 0 {
		if cfg.MinReplicas > cfg.MaxReplicas {
			result.AddError("keycloak.autoscaling",
				fmt.Sprintf("min_replicas (%d) cannot exceed max_replicas (%d)", cfg.MinReplicas, cfg.MaxReplicas),
				"Set min_replicas less than or equal to max_replicas")
		}
		if cfg.MinReplicas < 2 {
			result.AddWarning("keycloak.autoscaling",
				"min_replicas should be at least 2 for high availability",
				"Consider setting min_replicas to 2 or more")
		}
	}

	// Validate DB pool configuration
	if cfg.DBPoolMaxSize > 0 && cfg.DBPoolMinSize > cfg.DBPoolMaxSize {
		result.AddError("keycloak.db_pool",
			fmt.Sprintf("db_pool_min_size (%d) cannot exceed db_pool_max_size (%d)", cfg.DBPoolMinSize, cfg.DBPoolMaxSize),
			"Set db_pool_min_size less than or equal to db_pool_max_size")
	}

	// Validate frontend URL - require HTTPS in production
	if cfg.FrontendURL != "" {
		if !strings.HasPrefix(cfg.FrontendURL, "https://") && !strings.HasPrefix(cfg.FrontendURL, "http://") {
			result.AddError("keycloak.frontend_url",
				"keycloak_frontend_url must be a valid HTTP(S) URL",
				"Provide a valid HTTP or HTTPS URL",
				"Example: https://keycloak.example.com")
		}
		if cfg.StartOptimized && !strings.HasPrefix(cfg.FrontendURL, "https://") {
			result.AddError("keycloak.frontend_url",
				"keycloak_frontend_url must use HTTPS when start_optimized is enabled",
				"Use HTTPS URL for production mode",
				"Or disable production mode: start_optimized: false")
		}
	}

	// Validate log level
	if cfg.LogLevel != "" {
		validLevels := map[string]bool{"INFO": true, "DEBUG": true, "WARN": true, "ERROR": true, "TRACE": true}
		if !validLevels[strings.ToUpper(cfg.LogLevel)] {
			result.AddError("keycloak.log_level",
				fmt.Sprintf("invalid log_level: %s (must be INFO, DEBUG, WARN, ERROR, or TRACE)", cfg.LogLevel),
				"Use one of: INFO, DEBUG, WARN, ERROR, TRACE")
		}
	}

	// Validate log format
	if cfg.LogFormat != "" {
		validFormats := map[string]bool{"default": true, "json": true}
		if !validFormats[cfg.LogFormat] {
			result.AddError("keycloak.log_format",
				fmt.Sprintf("invalid log_format: %s (must be 'default' or 'json')", cfg.LogFormat),
				"Use 'default' or 'json'")
		}
	}

	// Validate cache stack
	if cfg.CacheStack != "" {
		validStacks := map[string]bool{"kubernetes": true, "ispn": true}
		if !validStacks[cfg.CacheStack] {
			result.AddError("keycloak.cache_stack",
				fmt.Sprintf("invalid cache_stack: %s (must be 'kubernetes' or 'ispn')", cfg.CacheStack),
				"Use 'kubernetes' or 'ispn'")
		}
	}

	return result, nil
}
