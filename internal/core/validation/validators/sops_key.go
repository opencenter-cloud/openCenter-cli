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

package validators

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

// SOPSKeyValidator validates SOPS encryption keys.
//
// This validator checks:
//   - Key file exists and is readable
//   - Age key format (starts with "AGE-SECRET-KEY-")
//   - File permissions (warns if not 0600)
//
// Example usage:
//
//	validator := validators.NewSOPSKeyValidator(fileSystem)
//	result, err := validator.Validate(ctx, "/path/to/key.txt")
type SOPSKeyValidator struct {
	fileSystem fs.FileSystem
}

// NewSOPSKeyValidator creates a new SOPS key validator.
//
// Parameters:
//   - fileSystem: FileSystem implementation for file operations
//
// Returns:
//   - *SOPSKeyValidator: New validator instance
func NewSOPSKeyValidator(fileSystem fs.FileSystem) *SOPSKeyValidator {
	return &SOPSKeyValidator{
		fileSystem: fileSystem,
	}
}

// Name returns the validator name.
func (v *SOPSKeyValidator) Name() string {
	return "sops-key"
}

// Priority returns the validator priority.
// SOPS key validation involves file I/O, so it has low priority.
func (v *SOPSKeyValidator) Priority() int {
	return validation.PriorityLow
}

// Validate validates a SOPS key file.
//
// The value parameter should be a string containing the path to the key file.
//
// Validation checks:
//  1. File exists
//  2. File is readable
//  3. Key format is valid (starts with "AGE-SECRET-KEY-")
//  4. File permissions are secure (warns if not 0600)
//
// Parameters:
//   - ctx: Context for cancellation
//   - value: Key file path (string)
//
// Returns:
//   - *ValidationResult: Validation result with errors/warnings
//   - error: Execution error (not validation failure)
func (v *SOPSKeyValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	keyPath, ok := value.(string)
	if !ok {
		result := validation.NewValidationResult()
		result.AddError("sops_key", "invalid data type for SOPS key validation (expected string)")
		return result, nil
	}

	result := validation.NewValidationResult()

	// Check if key file exists
	if !v.fileSystem.Exists(keyPath) {
		result.AddError(
			"sops_key",
			fmt.Sprintf("SOPS key file not found: %s", keyPath),
			"Generate a new Age key: age-keygen -o "+keyPath,
			"Or use: opencenter sops generate-key",
			"Verify the file path is correct",
		)
		return result, nil
	}

	// Read key file
	keyData, err := v.fileSystem.ReadFile(keyPath)
	if err != nil {
		result.AddError(
			"sops_key",
			fmt.Sprintf("cannot read SOPS key file: %v", err),
			"Check file permissions (should be 0600)",
			"Verify you have read access to the file",
		)
		return result, nil
	}

	// Validate Age key format
	keyContent := strings.TrimSpace(string(keyData))
	if !strings.HasPrefix(keyContent, "AGE-SECRET-KEY-") {
		result.AddError(
			"sops_key",
			"invalid Age key format",
			"Age keys must start with 'AGE-SECRET-KEY-'",
			"Generate a new key: age-keygen -o "+keyPath,
		)
		return result, nil
	}

	// Check file permissions
	info, err := v.fileSystem.Stat(keyPath)
	if err == nil {
		mode := info.Mode().Perm()
		if mode != 0600 {
			result.AddWarning(
				"sops_key",
				fmt.Sprintf("insecure key file permissions: %o (should be 0600)", mode),
				fmt.Sprintf("Fix permissions: chmod 600 %s", keyPath),
			)
		}
	}

	return result, nil
}
