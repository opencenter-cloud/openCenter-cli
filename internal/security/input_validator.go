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

package security

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// InputValidator validates and sanitizes user-controlled input to prevent injection attacks
type InputValidator interface {
	ValidateClusterName(name string) error
	ValidateOrganizationName(org string) error
	ValidatePath(path string) error
	ValidateURL(urlStr string) error
	ValidateEnvironmentVariable(name, value string) error
	SanitizeShellInput(input string) (string, error)
	SetAuditLogger(logger *AuditLogger)
	SetActor(actor string)
}

// DefaultInputValidator implements InputValidator interface
type DefaultInputValidator struct {
	// clusterNamePattern validates cluster and organization names
	clusterNamePattern *regexp.Regexp
	// safeEditors is a whitelist of allowed editors
	safeEditors map[string]bool
	// shellMetachars are characters that need to be escaped or rejected
	shellMetachars []string
	// auditLogger logs rejected inputs
	auditLogger *AuditLogger
	// actor is the current user/system performing the validation
	actor string
}

// NewDefaultInputValidator creates a new input validator with default settings
func NewDefaultInputValidator() *DefaultInputValidator {
	return &DefaultInputValidator{
		// Pattern: must start with alphanumeric, then alphanumeric/hyphen/underscore/dot, max 63 chars
		clusterNamePattern: regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`),
		safeEditors: map[string]bool{
			"vim":   true,
			"vi":    true,
			"nvim":  true,
			"nano":  true,
			"emacs": true,
			"code":  true,
			"subl":  true,
			"atom":  true,
			"gedit": true,
		},
		shellMetachars: []string{";", "|", "&", "$", "`", "\n", "\r", "<", ">", "(", ")", "{", "}"},
		actor:          "system",
	}
}

// SetAuditLogger sets the audit logger for logging rejected inputs
func (v *DefaultInputValidator) SetAuditLogger(logger *AuditLogger) {
	v.auditLogger = logger
}

// SetActor sets the actor (user/system) performing the validation
func (v *DefaultInputValidator) SetActor(actor string) {
	v.actor = actor
}

// logRejectedInput logs a rejected input to the audit log if configured
func (v *DefaultInputValidator) logRejectedInput(inputType, reason string) {
	if v.auditLogger != nil {
		// Use background context since this is async logging
		ctx := context.Background()
		_ = v.auditLogger.LogInputRejected(ctx, v.actor, inputType, reason)
	}
}

// ValidateClusterName validates a cluster name against security requirements
// Requirements: 1.5, 1.6, 6.1
func (v *DefaultInputValidator) ValidateClusterName(name string) error {
	if name == "" {
		v.logRejectedInput("cluster_name", "empty cluster name")
		return &ValidationError{
			Field:   "cluster_name",
			Value:   name,
			Message: "cluster name cannot be empty",
		}
	}

	// Check for path traversal sequences
	if strings.Contains(name, "..") {
		v.logRejectedInput("cluster_name", "path traversal detected")
		return &ValidationError{
			Field:   "cluster_name",
			Value:   name,
			Message: "cluster name cannot contain path traversal sequences (..)",
		}
	}

	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		v.logRejectedInput("cluster_name", "path separator detected")
		return &ValidationError{
			Field:   "cluster_name",
			Value:   name,
			Message: "cluster name cannot contain path separators (/ or \\)",
		}
	}

	// Validate against pattern
	if !v.clusterNamePattern.MatchString(name) {
		v.logRejectedInput("cluster_name", "invalid format")
		return &ValidationError{
			Field:   "cluster_name",
			Value:   name,
			Message: "cluster name must start with alphanumeric character and contain only alphanumeric, hyphen, underscore, or dot (max 63 characters)",
		}
	}

	return nil
}

// ValidateOrganizationName validates an organization name against security requirements
// Requirements: 6.2
func (v *DefaultInputValidator) ValidateOrganizationName(org string) error {
	if org == "" {
		return &ValidationError{
			Field:   "organization_name",
			Value:   org,
			Message: "organization name cannot be empty",
		}
	}

	// Check for path traversal sequences
	if strings.Contains(org, "..") {
		return &ValidationError{
			Field:   "organization_name",
			Value:   org,
			Message: "organization name cannot contain path traversal sequences (..)",
		}
	}

	if strings.Contains(org, "/") || strings.Contains(org, "\\") {
		return &ValidationError{
			Field:   "organization_name",
			Value:   org,
			Message: "organization name cannot contain path separators (/ or \\)",
		}
	}

	// Validate against same pattern as cluster name
	if !v.clusterNamePattern.MatchString(org) {
		return &ValidationError{
			Field:   "organization_name",
			Value:   org,
			Message: "organization name must start with alphanumeric character and contain only alphanumeric, hyphen, underscore, or dot (max 63 characters)",
		}
	}

	return nil
}

// ValidatePath validates a file path to prevent path traversal attacks
// Requirements: 1.8, 6.3
func (v *DefaultInputValidator) ValidatePath(path string) error {
	if path == "" {
		return &ValidationError{
			Field:   "path",
			Value:   path,
			Message: "path cannot be empty",
		}
	}

	// Check for path traversal sequences
	if strings.Contains(path, "..") {
		return &ValidationError{
			Field:   "path",
			Value:   path,
			Message: "path cannot contain path traversal sequences (..)",
		}
	}

	// Clean the path and check if it changed (indicates suspicious input)
	cleanPath := filepath.Clean(path)

	// Check for absolute paths outside expected directories
	if filepath.IsAbs(path) {
		// For absolute paths, we need to ensure they're within expected directories
		// This is a basic check - in production, you'd compare against allowed base paths
		if strings.Contains(path, "..") {
			return &ValidationError{
				Field:   "path",
				Value:   path,
				Message: "absolute path contains suspicious sequences",
			}
		}
	}

	// Check for symlink indicators (basic check)
	// Note: Full symlink validation requires filesystem access
	if strings.Contains(cleanPath, "..") {
		return &ValidationError{
			Field:   "path",
			Value:   path,
			Message: "path resolves to suspicious location",
		}
	}

	return nil
}

// ValidateURL validates a URL to ensure it uses HTTPS for external URLs
// Requirements: 6.6
func (v *DefaultInputValidator) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return &ValidationError{
			Field:   "url",
			Value:   urlStr,
			Message: "URL cannot be empty",
		}
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &ValidationError{
			Field:   "url",
			Value:   urlStr,
			Message: fmt.Sprintf("invalid URL format: %v", err),
		}
	}

	// Check scheme first
	scheme := strings.ToLower(parsedURL.Scheme)

	// Validate scheme is http or https
	if scheme != "http" && scheme != "https" {
		// If scheme is empty, it's likely a malformed URL
		if scheme == "" {
			return &ValidationError{
				Field:   "url",
				Value:   urlStr,
				Message: "invalid URL format: missing or invalid scheme",
			}
		}
		return &ValidationError{
			Field:   "url",
			Value:   urlStr,
			Message: fmt.Sprintf("unsupported URL scheme: %s (must be http or https)", scheme),
		}
	}

	// For external URLs (not localhost or local IPs), require HTTPS
	host := strings.ToLower(parsedURL.Hostname())
	isLocal := host == "localhost" ||
		host == "127.0.0.1" ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "172.16.") ||
		strings.HasPrefix(host, "172.17.") ||
		strings.HasPrefix(host, "172.18.") ||
		strings.HasPrefix(host, "172.19.") ||
		strings.HasPrefix(host, "172.20.") ||
		strings.HasPrefix(host, "172.21.") ||
		strings.HasPrefix(host, "172.22.") ||
		strings.HasPrefix(host, "172.23.") ||
		strings.HasPrefix(host, "172.24.") ||
		strings.HasPrefix(host, "172.25.") ||
		strings.HasPrefix(host, "172.26.") ||
		strings.HasPrefix(host, "172.27.") ||
		strings.HasPrefix(host, "172.28.") ||
		strings.HasPrefix(host, "172.29.") ||
		strings.HasPrefix(host, "172.30.") ||
		strings.HasPrefix(host, "172.31.")

	if !isLocal && scheme != "https" {
		return &ValidationError{
			Field:   "url",
			Value:   urlStr,
			Message: "external URLs must use HTTPS scheme",
		}
	}

	return nil
}

// ValidateEnvironmentVariable validates environment variable names and values
// Requirements: 1.1, 1.2
func (v *DefaultInputValidator) ValidateEnvironmentVariable(name, value string) error {
	if name == "" {
		return &ValidationError{
			Field:   "environment_variable",
			Value:   name,
			Message: "environment variable name cannot be empty",
		}
	}

	// Special handling for EDITOR variable
	if name == "EDITOR" || name == "VISUAL" {
		return v.validateEditor(value)
	}

	// Check for shell metacharacters in value
	for _, metachar := range v.shellMetachars {
		if strings.Contains(value, metachar) {
			return &ValidationError{
				Field:   name,
				Value:   value,
				Message: fmt.Sprintf("environment variable value contains shell metacharacter: %s", metachar),
			}
		}
	}

	return nil
}

// validateEditor validates the EDITOR environment variable against whitelist
func (v *DefaultInputValidator) validateEditor(editor string) error {
	if editor == "" {
		return nil // Empty is acceptable, system will use default
	}

	// Check for shell metacharacters first
	for _, metachar := range v.shellMetachars {
		if strings.Contains(editor, metachar) {
			v.logRejectedInput("EDITOR", fmt.Sprintf("shell metacharacter detected: %s", metachar))
			return &ValidationError{
				Field:   "EDITOR",
				Value:   editor,
				Message: fmt.Sprintf("EDITOR value contains shell metacharacter: %s", metachar),
			}
		}
	}

	// Extract just the command name (remove path and arguments)
	editorCmd := filepath.Base(editor)
	editorCmd = strings.Split(editorCmd, " ")[0]

	if !v.safeEditors[editorCmd] {
		v.logRejectedInput("EDITOR", fmt.Sprintf("editor not in whitelist: %s", editorCmd))
		return &ValidationError{
			Field:   "EDITOR",
			Value:   editor,
			Message: fmt.Sprintf("editor '%s' is not in the safe editors whitelist (allowed: vim, vi, nvim, nano, emacs, code, subl, atom, gedit)", editorCmd),
		}
	}

	return nil
}

// SanitizeShellInput escapes shell metacharacters in user input
// Requirements: 1.2, 1.3
func (v *DefaultInputValidator) SanitizeShellInput(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	// Check for dangerous metacharacters that should be rejected outright
	dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(input, char) {
			return "", &ValidationError{
				Field:   "shell_input",
				Value:   input,
				Message: fmt.Sprintf("input contains dangerous shell metacharacter: %s", char),
			}
		}
	}

	// Escape other special characters
	sanitized := input
	sanitized = strings.ReplaceAll(sanitized, "$", "\\$")
	sanitized = strings.ReplaceAll(sanitized, "<", "\\<")
	sanitized = strings.ReplaceAll(sanitized, ">", "\\>")
	sanitized = strings.ReplaceAll(sanitized, "(", "\\(")
	sanitized = strings.ReplaceAll(sanitized, ")", "\\)")
	sanitized = strings.ReplaceAll(sanitized, "{", "\\{")
	sanitized = strings.ReplaceAll(sanitized, "}", "\\}")

	return sanitized, nil
}

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	// Mask the value if it looks sensitive
	displayValue := e.Value
	if len(displayValue) > 50 {
		displayValue = displayValue[:20] + "..." + displayValue[len(displayValue)-10:]
	}
	return fmt.Sprintf("validation failed for field '%s': %s (value: %s)", e.Field, e.Message, displayValue)
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}
