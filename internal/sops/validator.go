/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

// DefaultValidator implements Validator interface
type DefaultValidator struct{}

// NewDefaultValidator creates a new default validator
func NewDefaultValidator() *DefaultValidator {
	return &DefaultValidator{}
}

// ValidateEncryption validates that files are properly encrypted
func (v *DefaultValidator) ValidateEncryption(overlayPath string, cfg *config.Config) error {
	filesToCheck := v.getFilesToEncrypt(overlayPath, cfg)

	var validationErrors []error

	for _, file := range filesToCheck {
		filePath := filepath.Join(overlayPath, file)

		// Skip if file doesn't exist
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}

		if err := v.ValidateEncryptedFile(filePath); err != nil {
			validationErrors = append(validationErrors, &errors.StructuredError{
				Type:    errors.SOPSError,
				Field:   file,
				Message: "File validation failed",
				Cause:   err,
				Suggestions: []string{
					"Check if the file is properly encrypted with SOPS",
					"Verify the SOPS configuration is correct",
					"Ensure the age key is accessible",
				},
			})
		}
	}

	if len(validationErrors) > 0 {
		return &errors.ErrorCollection{Errors: validationErrors}
	}

	return nil
}

// ValidateKeyForProduction validates that a key is not a placeholder
func (v *DefaultValidator) ValidateKeyForProduction(key string) error {
	// Check for placeholder key pattern
	if strings.Contains(key, "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx") {
		return &errors.StructuredError{
			Type:    errors.ValidationError,
			Message: "Placeholder key detected - this should not be used in production",
			Suggestions: []string{
				"Generate a proper age key using 'openCenter sops generate-key'",
				"Import an existing age key",
				"Check the SOPS configuration",
			},
		}
	}
	
	// Validate age key format in the configuration content
	lines := strings.Split(key, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "age:") {
			ageKey := strings.TrimSpace(strings.TrimPrefix(line, "age:"))
			if err := v.validateAgeKeyFormat(ageKey); err != nil {
				return &errors.StructuredError{
					Type:    errors.ValidationError,
					Message: "Invalid age key format in configuration",
					Cause:   err,
					Suggestions: []string{
						"Ensure the age key follows the correct format (age1...)",
						"Generate a new age key if the current one is invalid",
						"Check the SOPS configuration syntax",
					},
				}
			}
		}
	}
	
	return nil
}

// ValidateSOPSConfig validates a SOPS configuration file
func (v *DefaultValidator) ValidateSOPSConfig(configPath string) error {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &errors.StructuredError{
			Type:    errors.FileError,
			Field:   ".sops.yaml",
			Message: "SOPS configuration file not found",
			Suggestions: []string{
				"Create a .sops.yaml configuration file",
				"Check the file path is correct",
				"Ensure the file has proper permissions",
			},
		}
	}

	// Read and validate config content
	content, err := os.ReadFile(configPath)
	if err != nil {
		return &errors.StructuredError{
			Type:    errors.FileError,
			Field:   ".sops.yaml",
			Message: "Failed to read SOPS configuration file",
			Cause:   err,
			Suggestions: []string{
				"Check file permissions",
				"Ensure the file is not corrupted",
				"Verify disk access",
			},
		}
	}

	// Validate configuration content
	if err := v.ValidateKeyForProduction(string(content)); err != nil {
		return err
	}

	// Check for required sections
	contentStr := string(content)
	if !strings.Contains(contentStr, "creation_rules") {
		return &errors.StructuredError{
			Type:    errors.ValidationError,
			Field:   ".sops.yaml",
			Message: "SOPS configuration missing creation_rules section",
			Suggestions: []string{
				"Add creation_rules section to .sops.yaml",
				"Check SOPS configuration documentation",
				"Use a valid SOPS configuration template",
			},
		}
	}

	return nil
}

// ValidateEncryptedFile validates that a file is properly encrypted
func (v *DefaultValidator) ValidateEncryptedFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)
	
	// Check for SOPS metadata
	if !strings.Contains(contentStr, "sops:") {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Message: "File does not contain SOPS metadata",
			Suggestions: []string{
				"Encrypt the file using SOPS",
				"Check if the file was properly encrypted",
				"Verify SOPS configuration",
			},
		}
	}

	// Check for encryption keys
	if !strings.Contains(contentStr, "age:") && !strings.Contains(contentStr, "pgp:") {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Message: "File does not contain valid encryption keys",
			Suggestions: []string{
				"Re-encrypt the file with proper age or PGP keys",
				"Check SOPS configuration for valid keys",
				"Verify the encryption process completed successfully",
			},
		}
	}

	// Check for encrypted data
	if !strings.Contains(contentStr, "ENC[") {
		return &errors.StructuredError{
			Type:    errors.SOPSError,
			Message: "File does not contain encrypted data",
			Suggestions: []string{
				"Ensure the file contains sensitive data to encrypt",
				"Check the encrypted_regex pattern in SOPS config",
				"Verify the encryption process worked correctly",
			},
		}
	}

	return nil
}

// Helper methods

// getFilesToEncrypt returns the list of files that should be encrypted
func (v *DefaultValidator) getFilesToEncrypt(overlayPath string, cfg *config.Config) []string {
	var files []string

	// Standard encrypted files
	files = append(files,
		"flux-system/gotk-sync.yaml",
		"managed-services/sources/base-repo.yaml",
	)

	// Provider-specific encrypted files
	switch cfg.OpenCenter.Infrastructure.Provider {
	case "openstack":
		files = append(files, "secrets/openstack-credentials.yaml")
	case "vsphere":
		files = append(files,
			"secrets/vsphere-credentials.yaml",
			"customer-managed/services/cloud-provider-vsphere/secret.yaml",
		)
	}

	return files
}

// validateAgeKeyFormat validates an age key format
func (v *DefaultValidator) validateAgeKeyFormat(key string) error {
	// Age public keys start with "age1" and are 62 characters long
	if !strings.HasPrefix(key, "age1") || len(key) != 62 {
		return fmt.Errorf("invalid age key format: %s", key)
	}
	
	return nil
}