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

package v2

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Validator performs multi-layered validation of v2 configurations.
// Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7
type Validator interface {
	Validate(cfg *Config) error
	ValidateSchema(cfg *Config) error
	ValidateBusinessRules(cfg *Config) error
	ValidateProvider(cfg *Config) error
	ValidateDeployment(cfg *Config) error
	ValidateServices(cfg *Config) error
}

// defaultValidator implements the Validator interface.
type defaultValidator struct {
	schemaValidator *validator.Validate
}

// NewValidator creates a new v2 configuration validator.
func NewValidator() Validator {
	v := validator.New()
	
	// Register custom validation tags
	// dns1123: validates DNS-1123 subdomain names (lowercase alphanumeric, hyphens, dots)
	v.RegisterValidation("dns1123", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		if value == "" {
			return true // Empty is valid (use 'required' tag to enforce non-empty)
		}
		// DNS-1123 subdomain: lowercase alphanumeric, hyphens, dots, max 253 chars
		// Must start and end with alphanumeric
		if len(value) > 253 {
			return false
		}
		for i, c := range value {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
				return false
			}
			if (i == 0 || i == len(value)-1) && (c == '-' || c == '.') {
				return false
			}
		}
		return true
	})
	
	return &defaultValidator{
		schemaValidator: v,
	}
}

// Validate performs all validation layers.
// Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7
func (v *defaultValidator) Validate(cfg *Config) error {
	// Schema validation
	if err := v.ValidateSchema(cfg); err != nil {
		return err
	}

	// Business rules validation
	if err := v.ValidateBusinessRules(cfg); err != nil {
		return err
	}

	// Provider-specific validation
	if err := v.ValidateProvider(cfg); err != nil {
		return err
	}

	// Deployment-method validation
	if err := v.ValidateDeployment(cfg); err != nil {
		return err
	}

	// Service validation
	if err := v.ValidateServices(cfg); err != nil {
		return err
	}

	return nil
}

// ValidateSchema validates required fields, data types, and enum values.
// Requirements: 11.1
func (v *defaultValidator) ValidateSchema(cfg *Config) error {
	if err := v.schemaValidator.Struct(cfg); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

// ValidateBusinessRules validates cross-field dependencies and value ranges.
// Requirements: 11.2
func (v *defaultValidator) ValidateBusinessRules(cfg *Config) error {
	// Placeholder for business rules validation
	// This will be implemented in subsequent tasks
	return nil
}

// ValidateProvider validates provider-specific requirements.
// Requirements: 11.3
func (v *defaultValidator) ValidateProvider(cfg *Config) error {
	// Placeholder for provider validation
	// This will be implemented in subsequent tasks
	return nil
}

// ValidateDeployment validates deployment-method requirements.
// Requirements: 11.4
func (v *defaultValidator) ValidateDeployment(cfg *Config) error {
	// Placeholder for deployment validation
	// This will be implemented in subsequent tasks
	return nil
}

// ValidateServices validates service dependencies and required secrets.
// Requirements: 11.5
func (v *defaultValidator) ValidateServices(cfg *Config) error {
	// Placeholder for service validation
	// This will be implemented in subsequent tasks
	return nil
}
