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

package flags

import (
	"fmt"
	"os"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
	"github.com/rackerlabs/opencenter-cli/internal/util/security"
)

// SecurityFlagHandler handles security-related flags and warnings
type SecurityFlagHandler struct {
	masker     security.CredentialMasker
	fileSystem fs.FileSystem
}

// NewSecurityFlagHandler creates a new security flag handler
func NewSecurityFlagHandler() *SecurityFlagHandler {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	return &SecurityFlagHandler{
		masker:     security.NewDefaultCredentialMasker(),
		fileSystem: fileSystem,
	}
}

// CanHandle returns true if this handler can process the given flag
func (h *SecurityFlagHandler) CanHandle(flagName string) bool {
	securityFlags := []string{
		"--secure-template-var",
		"--mask-sensitive",
		"--security-warnings",
		"--sops-config",
		"--encrypted-config",
	}

	for _, flag := range securityFlags {
		if strings.HasPrefix(flagName, flag) {
			return true
		}
	}

	return false
}

// ParseFlag processes a single security flag
func (h *SecurityFlagHandler) ParseFlag(flagName, value string) (interface{}, error) {
	switch {
	case strings.HasPrefix(flagName, "--secure-template-var"):
		return h.parseSecureTemplateVar(flagName, value)
	case strings.HasPrefix(flagName, "--mask-sensitive"):
		return h.parseMaskSensitive(flagName, value)
	case strings.HasPrefix(flagName, "--security-warnings"):
		return h.parseSecurityWarnings(flagName, value)
	case strings.HasPrefix(flagName, "--sops-config"):
		return h.parseSOPSConfig(flagName, value)
	case strings.HasPrefix(flagName, "--encrypted-config"):
		return h.parseEncryptedConfig(flagName, value)
	default:
		return nil, fmt.Errorf("unsupported security flag: %s", flagName)
	}
}

// GetFlagType returns the type of flags this handler processes
func (h *SecurityFlagHandler) GetFlagType() FlagType {
	return FlagTypeSecurity
}

// parseSecureTemplateVar handles secure template variable flags
func (h *SecurityFlagHandler) parseSecureTemplateVar(flagName, value string) (*SecureTemplateVarFlag, error) {
	// Parse format: --secure-template-var KEY=value or --secure-template-var KEY=@file
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid secure template variable format: %s (expected KEY=value or KEY=@file)", value)
	}

	key := strings.TrimSpace(parts[0])
	valueSpec := strings.TrimSpace(parts[1])

	if key == "" {
		return nil, fmt.Errorf("template variable key cannot be empty")
	}

	var varValue string
	var isFile bool

	if strings.HasPrefix(valueSpec, "@") {
		// Load from file
		filePath := valueSpec[1:]
		if filePath == "" {
			return nil, fmt.Errorf("file path cannot be empty for secure template variable %s", key)
		}

		content, err := h.fileSystem.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read secure template variable file %s: %w", filePath, err)
		}

		varValue = strings.TrimSpace(string(content))
		isFile = true
	} else {
		varValue = valueSpec
		isFile = false
	}

	// Warn about command history exposure for non-file sources
	if !isFile {
		h.warnCommandHistoryExposure(key)
	}

	return &SecureTemplateVarFlag{
		Key:    key,
		Value:  varValue,
		IsFile: isFile,
	}, nil
}

// parseMaskSensitive handles sensitive data masking flags
func (h *SecurityFlagHandler) parseMaskSensitive(flagName, value string) (*MaskSensitiveFlag, error) {
	enabled := true
	if value != "" {
		switch strings.ToLower(value) {
		case "true", "yes", "1", "on":
			enabled = true
		case "false", "no", "0", "off":
			enabled = false
		default:
			return nil, fmt.Errorf("invalid mask-sensitive value: %s (expected true/false)", value)
		}
	}

	return &MaskSensitiveFlag{
		Enabled: enabled,
	}, nil
}

// parseSecurityWarnings handles security warnings flags
func (h *SecurityFlagHandler) parseSecurityWarnings(flagName, value string) (*SecurityWarningsFlag, error) {
	enabled := true
	if value != "" {
		switch strings.ToLower(value) {
		case "true", "yes", "1", "on":
			enabled = true
		case "false", "no", "0", "off":
			enabled = false
		default:
			return nil, fmt.Errorf("invalid security-warnings value: %s (expected true/false)", value)
		}
	}

	return &SecurityWarningsFlag{
		Enabled: enabled,
	}, nil
}

// parseSOPSConfig handles SOPS configuration flags
func (h *SecurityFlagHandler) parseSOPSConfig(flagName, value string) (*SOPSConfigFlag, error) {
	if value == "" {
		return nil, fmt.Errorf("SOPS config path cannot be empty")
	}

	// Validate that the SOPS config file exists
	if _, err := os.Stat(value); os.IsNotExist(err) {
		return nil, fmt.Errorf("SOPS config file does not exist: %s", value)
	}

	return &SOPSConfigFlag{
		ConfigPath: value,
	}, nil
}

// parseEncryptedConfig handles encrypted configuration file flags
func (h *SecurityFlagHandler) parseEncryptedConfig(flagName, value string) (*EncryptedConfigFlag, error) {
	if value == "" {
		return nil, fmt.Errorf("encrypted config path cannot be empty")
	}

	// Validate that the encrypted config file exists
	if _, err := os.Stat(value); os.IsNotExist(err) {
		return nil, fmt.Errorf("encrypted config file does not exist: %s", value)
	}

	return &EncryptedConfigFlag{
		ConfigPath: value,
	}, nil
}

// warnCommandHistoryExposure warns about potential command history exposure
func (h *SecurityFlagHandler) warnCommandHistoryExposure(key string) {
	fmt.Fprintf(os.Stderr, "WARNING: Template variable '%s' provided via command line may be exposed in command history.\n", key)
	fmt.Fprintf(os.Stderr, "Consider using --secure-template-var %s=@file to load from a file instead.\n", key)
}

// MaskFlagValue masks sensitive data in flag values for logging/output
func (h *SecurityFlagHandler) MaskFlagValue(flagName, value string) string {
	// Return empty values as-is
	if value == "" {
		return value
	}

	// Check if this is a sensitive flag
	if h.isSensitiveFlag(flagName) {
		// For sensitive flags, always mask the entire value
		return security.MaskString
	}

	// Check if the value contains sensitive patterns
	masked := h.masker.MaskString(value)
	if masked != value {
		return masked
	}

	return value
}

// isSensitiveFlag checks if a flag name indicates sensitive data
func (h *SecurityFlagHandler) isSensitiveFlag(flagName string) bool {
	// Use the credential masker's field detection logic
	return h.masker.IsSensitiveField(flagName)
}

// ValidateSecurityConfiguration validates security-related configuration
func (h *SecurityFlagHandler) ValidateSecurityConfiguration(config map[string]interface{}) []SecurityWarning {
	var warnings []SecurityWarning

	// Check for potential credential exposure
	warnings = append(warnings, h.scanForCredentials(config)...)

	// Check for insecure template variable usage
	warnings = append(warnings, h.checkTemplateVariables(config)...)

	// Check for missing SOPS configuration
	warnings = append(warnings, h.checkSOPSConfiguration(config)...)

	return warnings
}

// scanForCredentials scans configuration for potential credential exposure
func (h *SecurityFlagHandler) scanForCredentials(config map[string]interface{}) []SecurityWarning {
	var warnings []SecurityWarning

	for key, value := range config {
		if h.masker.IsSensitiveField(key) {
			if strValue, ok := value.(string); ok && strValue != "" {
				// Check if the value looks like a credential
				if h.looksLikeCredential(strValue) {
					warnings = append(warnings, SecurityWarning{
						Type:       "credential_exposure",
						Severity:   "high",
						Message:    fmt.Sprintf("Potential credential found in field '%s'", key),
						Field:      key,
						Suggestion: "Consider using SOPS encryption or secure template variables",
					})
				}
			}
		}

		// Recursively check nested structures
		if nestedMap, ok := value.(map[string]interface{}); ok {
			warnings = append(warnings, h.scanForCredentials(nestedMap)...)
		}
	}

	return warnings
}

// checkTemplateVariables checks for insecure template variable usage
func (h *SecurityFlagHandler) checkTemplateVariables(config map[string]interface{}) []SecurityWarning {
	var warnings []SecurityWarning

	// This would be implemented to check for template variables that might contain sensitive data
	// For now, return empty slice as this is a placeholder

	return warnings
}

// checkSOPSConfiguration checks for missing SOPS configuration
func (h *SecurityFlagHandler) checkSOPSConfiguration(config map[string]interface{}) []SecurityWarning {
	var warnings []SecurityWarning

	// Check if there are sensitive fields but no SOPS configuration
	hasSensitiveData := h.hasSensitiveData(config)
	hasSOPSConfig := h.hasSOPSConfig(config)

	if hasSensitiveData && !hasSOPSConfig {
		warnings = append(warnings, SecurityWarning{
			Type:       "missing_encryption",
			Severity:   "medium",
			Message:    "Configuration contains sensitive data but no SOPS encryption is configured",
			Suggestion: "Consider using --sops-config to specify SOPS configuration for encrypting sensitive data",
		})
	}

	return warnings
}

// hasSensitiveData checks if configuration contains sensitive data
func (h *SecurityFlagHandler) hasSensitiveData(config map[string]interface{}) bool {
	for key, value := range config {
		if h.masker.IsSensitiveField(key) {
			return true
		}

		if nestedMap, ok := value.(map[string]interface{}); ok {
			if h.hasSensitiveData(nestedMap) {
				return true
			}
		}
	}

	return false
}

// hasSOPSConfig checks if SOPS configuration is present
func (h *SecurityFlagHandler) hasSOPSConfig(config map[string]interface{}) bool {
	// Check for SOPS-related configuration
	sopsFields := []string{"sops", "encryption", "age_key", "sops_config"}

	for _, field := range sopsFields {
		if _, exists := config[field]; exists {
			return true
		}
	}

	return false
}

// looksLikeCredential checks if a string looks like a credential
func (h *SecurityFlagHandler) looksLikeCredential(value string) bool {
	// Simple heuristics to detect potential credentials
	if len(value) < 8 {
		return false
	}

	// Check for common credential patterns
	credentialPatterns := []string{
		"sk-", "pk-", "-----BEGIN", "AKIA", "AGE-SECRET-KEY",
	}

	for _, pattern := range credentialPatterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}

	// Check for high entropy (potential random keys/tokens)
	return h.hasHighEntropy(value)
}

// hasHighEntropy checks if a string has high entropy (potential random key)
func (h *SecurityFlagHandler) hasHighEntropy(value string) bool {
	if len(value) < 20 {
		return false
	}

	// Simple entropy check: count unique characters
	charMap := make(map[rune]bool)
	for _, char := range value {
		charMap[char] = true
	}

	// If more than 60% of characters are unique, consider it high entropy
	uniqueRatio := float64(len(charMap)) / float64(len(value))
	return uniqueRatio > 0.6
}
