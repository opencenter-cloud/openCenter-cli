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

// Package validators provides built-in validators for common validation tasks.
//
// This package contains validators for:
//   - Cluster and organization names (ClusterNameValidator, OrganizationNameValidator)
//   - Configuration values (ConfigValidator)
//   - File paths and operations (FileValidator)
//   - Security-related inputs (SecurityValidator)
//
// # Usage
//
// Validators can be registered with a ValidationEngine:
//
//	engine := validation.NewValidationEngine()
//	engine.Register(validators.NewClusterNameValidator())
//	engine.Register(validators.NewConfigValidator())
//	engine.Register(validators.NewFileValidator())
//	engine.Register(validators.NewSecurityValidator())
//
// Or registered globally:
//
//	validation.Register(validators.NewClusterNameValidator())
//
// # ClusterNameValidator
//
// Validates cluster and organization names according to security requirements:
//   - Must start with alphanumeric character
//   - Can contain alphanumeric, hyphen, underscore, or dot
//   - Maximum 63 characters
//   - No path traversal sequences (..)
//   - No path separators (/ or \)
//
// Example:
//
//	validator := validators.NewClusterNameValidator()
//	result, err := validator.Validate(ctx, "my-cluster-01")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if !result.Valid {
//	    for _, issue := range result.Errors {
//	        fmt.Printf("Error: %s\n", issue.Message)
//	    }
//	}
//
// # ConfigValidator
//
// Validates configuration values based on type:
//   - email: Email address format
//   - domain: Domain name format
//   - fqdn: Fully qualified domain name
//   - url: URL format and security (HTTPS for external)
//   - ip: IP address (IPv4 or IPv6)
//   - cidr: CIDR notation
//   - port: Port number (1-65535)
//   - cluster-name: Delegates to ClusterNameValidator
//
// Example:
//
//	validator := validators.NewConfigValidator()
//	result, err := validator.Validate(ctx, map[string]interface{}{
//	    "type":  "email",
//	    "value": "admin@example.com",
//	})
//
// # FileValidator
//
// Validates file paths and file system operations:
//   - Path traversal prevention
//   - Path length limits
//   - File existence and permissions
//   - Read/write/delete operation validation
//
// Example:
//
//	validator := validators.NewFileValidator()
//	result, err := validator.Validate(ctx, map[string]interface{}{
//	    "operation": "read",
//	    "path":      "/path/to/file.yaml",
//	})
//
// # SecurityValidator
//
// Validates security-related inputs:
//   - Shell input injection prevention
//   - Environment variable validation
//   - Editor whitelist validation
//   - Command safety checks
//   - Secret detection
//
// Example:
//
//	validator := validators.NewSecurityValidator()
//	result, err := validator.Validate(ctx, map[string]interface{}{
//	    "type":  "shell-input",
//	    "value": "user-input",
//	})
//
// # Custom Validators
//
// You can create custom validators by implementing the validation.Validator interface:
//
//	type MyValidator struct{}
//
//	func (v *MyValidator) Name() string {
//	    return "my-validator"
//	}
//
//	func (v *MyValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
//	    result := &validation.ValidationResult{Valid: true}
//	    // Perform validation...
//	    return result, nil
//	}
//
// # Performance
//
// All validators are designed to be fast (<100μs per validation) and thread-safe.
// They can be safely used concurrently from multiple goroutines.
package validators
