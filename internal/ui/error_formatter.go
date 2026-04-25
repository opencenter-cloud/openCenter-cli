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

package ui

import (
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
)

// Severity represents the severity level of an error
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

// ErrorInfo contains metadata about an error code
// Requirements: 15.2, 15.3
type ErrorInfo struct {
	Code        string
	Title       string
	Description string
	Fix         string
	FixCommand  string
	Hint        string
	DocsURL     string
	Severity    Severity
}

// ErrorFormatter formats errors with credential masking
// Requirements: 3.5, 15.1, 15.2, 15.4, 15.8
type ErrorFormatter interface {
	Format(err error) string
	FormatWithFix(err error, fix string) string
	FormatWithCode(err error, code string) string
	FormatWithErrorInfo(err error, info ErrorInfo) string
	GetErrorInfo(code string) (ErrorInfo, bool)
}

// DefaultErrorFormatter implements ErrorFormatter interface
// Requirements: 15.1, 15.2, 15.3, 15.4, 15.5, 15.6, 15.7, 15.8
type DefaultErrorFormatter struct {
	errorHandler  *errors.DefaultErrorHandler
	masker        security.CredentialMasker
	errorRegistry map[string]ErrorInfo
}

// NewDefaultErrorFormatter creates a new error formatter
// Requirements: 15.1, 15.2, 15.3
func NewDefaultErrorFormatter() *DefaultErrorFormatter {
	formatter := &DefaultErrorFormatter{
		errorHandler:  errors.NewDefaultErrorHandlerWithoutMasking(),
		masker:        security.NewDefaultCredentialMasker(),
		errorRegistry: make(map[string]ErrorInfo),
	}
	formatter.initializeErrorRegistry()
	return formatter
}

// Format formats an error with credential masking
// Requirements: 3.5
func (f *DefaultErrorFormatter) Format(err error) string {
	if err == nil {
		return ""
	}

	// Use error handler to format the error
	formatted := f.errorHandler.FormatError(err)

	// Mask any credentials in the formatted output
	masked := f.masker.MaskString(formatted)

	return masked
}

// FormatWithFix formats an error with a fix suggestion and credential masking
// Requirements: 3.5
func (f *DefaultErrorFormatter) FormatWithFix(err error, fix string) string {
	if err == nil {
		return ""
	}

	// Format the base error
	formatted := f.Format(err)

	// Add fix suggestion
	if fix != "" {
		// Mask the fix suggestion as well
		maskedFix := f.masker.MaskString(fix)
		formatted += fmt.Sprintf("\n\nFix:\n  %s", maskedFix)
	}

	return formatted
}

// FormatWithCode formats an error with an error code and credential masking
// Requirements: 3.5
func (f *DefaultErrorFormatter) FormatWithCode(err error, code string) string {
	if err == nil {
		return ""
	}

	// Format the base error
	formatted := f.Format(err)

	// Add error code
	if code != "" {
		formatted = fmt.Sprintf("[%s] %s", code, formatted)
	}

	return formatted
}

// FormatMultiple formats multiple errors with credential masking
// Requirements: 3.5
func (f *DefaultErrorFormatter) FormatMultiple(errs []error, maxErrors int) string {
	if len(errs) == 0 {
		return ""
	}

	var parts []string

	// Limit the number of errors displayed
	limit := len(errs)
	if maxErrors > 0 && limit > maxErrors {
		limit = maxErrors
	}

	for i := 0; i < limit; i++ {
		if errs[i] != nil {
			formatted := f.Format(errs[i])
			parts = append(parts, fmt.Sprintf("%d. %s", i+1, formatted))
		}
	}

	result := strings.Join(parts, "\n\n")

	// Add note if there are more errors
	if len(errs) > limit {
		result += fmt.Sprintf("\n\n... and %d more errors (use --log-level debug to see all)", len(errs)-limit)
	}

	return result
}

// FormatWithContext formats an error with additional context and credential masking
// Requirements: 3.5
func (f *DefaultErrorFormatter) FormatWithContext(err error, context map[string]interface{}) string {
	if err == nil {
		return ""
	}

	// Format the base error
	formatted := f.Format(err)

	// Add context if provided
	if len(context) > 0 {
		formatted += "\n\nContext:"
		for key, value := range context {
			// Mask the context values
			valueStr := fmt.Sprintf("%v", value)
			maskedValue := f.masker.MaskString(valueStr)
			formatted += fmt.Sprintf("\n  %s: %s", key, maskedValue)
		}
	}

	return formatted
}

// MaskError masks credentials in an error message
// Requirements: 3.5
func (f *DefaultErrorFormatter) MaskError(err error) error {
	if err == nil {
		return nil
	}

	maskedMessage := f.masker.MaskString(err.Error())
	return fmt.Errorf("%s", maskedMessage)
}

// FormatWithErrorInfo formats an error with complete error information
// Requirements: 15.1, 15.2, 15.4, 15.8
func (f *DefaultErrorFormatter) FormatWithErrorInfo(err error, info ErrorInfo) string {
	if err == nil {
		return ""
	}

	var parts []string

	// Error code and title
	if info.Code != "" {
		parts = append(parts, fmt.Sprintf("Error: %s (%s)", info.Title, info.Code))
	} else {
		parts = append(parts, fmt.Sprintf("Error: %s", info.Title))
	}

	// Description
	if info.Description != "" {
		parts = append(parts, "")
		parts = append(parts, info.Description)
	}

	// Context from error
	maskedErr := f.masker.MaskString(err.Error())
	if maskedErr != "" && maskedErr != info.Title {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("Details: %s", maskedErr))
	}

	// Fix suggestion
	if info.Fix != "" {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("Fix: %s", info.Fix))
	}

	// Fix command
	if info.FixCommand != "" {
		parts = append(parts, fmt.Sprintf("  %s", info.FixCommand))
	}

	// Hint
	if info.Hint != "" {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("Hint: %s", info.Hint))
	}

	// Documentation link
	if info.DocsURL != "" {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("Learn more: %s", info.DocsURL))
	}

	return strings.Join(parts, "\n")
}

// GetErrorInfo retrieves error information by code
// Requirements: 15.2, 15.3
func (f *DefaultErrorFormatter) GetErrorInfo(code string) (ErrorInfo, bool) {
	info, ok := f.errorRegistry[code]
	return info, ok
}

// FormatMultipleWithLimit formats multiple errors with a limit
// Requirements: 15.5, 15.6
func (f *DefaultErrorFormatter) FormatMultipleWithLimit(errs []error, maxErrors int, verbose bool) string {
	if len(errs) == 0 {
		return ""
	}

	var parts []string

	// Determine limit
	limit := len(errs)
	if !verbose && maxErrors > 0 && limit > maxErrors {
		limit = maxErrors
	}

	// Group errors by severity if possible
	criticalErrs := []error{}
	warningErrs := []error{}
	infoErrs := []error{}

	for _, err := range errs {
		// Try to determine severity from error type
		if structuredErr, ok := err.(*errors.StructuredError); ok {
			switch structuredErr.Type {
			case errors.ValidationError, errors.PermissionError, errors.CredentialError:
				criticalErrs = append(criticalErrs, err)
			case errors.ConfigError, errors.PathError:
				warningErrs = append(warningErrs, err)
			default:
				infoErrs = append(infoErrs, err)
			}
		} else {
			infoErrs = append(infoErrs, err)
		}
	}

	// Format critical errors first
	errorCount := 0
	if len(criticalErrs) > 0 {
		parts = append(parts, "Critical Errors:")
		for i, err := range criticalErrs {
			if errorCount >= limit {
				break
			}
			formatted := f.Format(err)
			parts = append(parts, fmt.Sprintf("\n%d. %s", i+1, formatted))
			errorCount++
		}
	}

	// Then warnings
	if len(warningErrs) > 0 && errorCount < limit {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, "Warnings:")
		for i, err := range warningErrs {
			if errorCount >= limit {
				break
			}
			formatted := f.Format(err)
			parts = append(parts, fmt.Sprintf("\n%d. %s", i+1, formatted))
			errorCount++
		}
	}

	// Then info
	if len(infoErrs) > 0 && errorCount < limit {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, "Additional Errors:")
		for i, err := range infoErrs {
			if errorCount >= limit {
				break
			}
			formatted := f.Format(err)
			parts = append(parts, fmt.Sprintf("\n%d. %s", i+1, formatted))
			errorCount++
		}
	}

	result := strings.Join(parts, "\n")

	// Add note if there are more errors
	if len(errs) > limit {
		result += fmt.Sprintf("\n\n... and %d more errors (use --log-level debug to see all)", len(errs)-limit)
	}

	return result
}

// initializeErrorRegistry initializes the error code registry
// Requirements: 15.2, 15.3
func (f *DefaultErrorFormatter) initializeErrorRegistry() {
	// E1xxx: Validation errors
	f.errorRegistry["E1001"] = ErrorInfo{
		Code:        "E1001",
		Title:       "OpenStack region not configured",
		Description: "The OpenStack provider requires a region to be specified.",
		Fix:         "Add region to your configuration:",
		FixCommand:  "opencenter cluster set {cluster} opencenter.infrastructure.cloud.openstack.region=RegionOne",
		Hint:        "List available regions: openstack region list",
		DocsURL:     "https://docs.opencenter.cloud/errors/E1001",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E1002"] = ErrorInfo{
		Code:        "E1002",
		Title:       "SOPS key not found",
		Description: "The SOPS Age encryption key for this cluster could not be found.",
		Fix:         "Generate a new SOPS key:",
		FixCommand:  "opencenter secrets keys generate",
		Hint:        "Check if the key file exists: ls -la ~/.config/opencenter/clusters/{org}/{cluster}/secrets/age/",
		DocsURL:     "https://docs.opencenter.cloud/errors/E1002",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E1003"] = ErrorInfo{
		Code:        "E1003",
		Title:       "Invalid cluster name",
		Description: "Cluster names must start with a letter and contain only alphanumeric characters, hyphens, and underscores (max 63 characters).",
		Fix:         "Use a valid cluster name:",
		FixCommand:  "opencenter cluster init my-cluster",
		Hint:        "Valid examples: my-cluster, prod_cluster, cluster123",
		DocsURL:     "https://docs.opencenter.cloud/errors/E1003",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E1004"] = ErrorInfo{
		Code:        "E1004",
		Title:       "Configuration validation failed",
		Description: "The cluster configuration contains validation errors.",
		Fix:         "Run validation to see specific errors:",
		FixCommand:  "opencenter cluster validate {cluster}",
		Hint:        "Generate the configuration schema: opencenter config ide --schema-only",
		DocsURL:     "https://docs.opencenter.cloud/errors/E1004",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E1005"] = ErrorInfo{
		Code:        "E1005",
		Title:       "Required field missing",
		Description: "A required configuration field is missing or empty.",
		Fix:         "Edit the configuration and add the required field:",
		FixCommand:  "opencenter cluster edit {cluster}",
		Hint:        "Generate the schema for required fields: opencenter config ide --schema-only",
		DocsURL:     "https://docs.opencenter.cloud/errors/E1005",
		Severity:    SeverityCritical,
	}

	// E2xxx: Security errors
	f.errorRegistry["E2001"] = ErrorInfo{
		Code:        "E2001",
		Title:       "Command injection attempt detected",
		Description: "The input contains shell metacharacters that could be used for command injection.",
		Fix:         "Remove shell metacharacters from the input:",
		Hint:        "Avoid using characters like: ; | & < > $ ` \\ in names and paths",
		DocsURL:     "https://docs.opencenter.cloud/errors/E2001",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E2002"] = ErrorInfo{
		Code:        "E2002",
		Title:       "Template injection attempt detected",
		Description: "The template contains dangerous functions that are not allowed.",
		Fix:         "Remove dangerous template functions:",
		Hint:        "Allowed functions: upper, lower, trim, replace, split, join, printf, quote",
		DocsURL:     "https://docs.opencenter.cloud/errors/E2002",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E2003"] = ErrorInfo{
		Code:        "E2003",
		Title:       "Path traversal attempt detected",
		Description: "The path contains sequences that could be used for path traversal attacks.",
		Fix:         "Use a path without .. or absolute paths:",
		Hint:        "Paths should be relative to the configuration directory",
		DocsURL:     "https://docs.opencenter.cloud/errors/E2003",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E2004"] = ErrorInfo{
		Code:        "E2004",
		Title:       "Invalid EDITOR environment variable",
		Description: "The EDITOR environment variable contains an unsafe value.",
		Fix:         "Set EDITOR to a safe editor:",
		FixCommand:  "export EDITOR=vim",
		Hint:        "Allowed editors: vim, nano, emacs, vi, code, subl",
		DocsURL:     "https://docs.opencenter.cloud/errors/E2004",
		Severity:    SeverityCritical,
	}

	// E3xxx: Network errors
	f.errorRegistry["E3001"] = ErrorInfo{
		Code:        "E3001",
		Title:       "Network timeout",
		Description: "The operation timed out while waiting for a network response.",
		Fix:         "Check network connectivity and retry:",
		FixCommand:  "ping {host}",
		Hint:        "Verify firewall rules and network configuration",
		DocsURL:     "https://docs.opencenter.cloud/errors/E3001",
		Severity:    SeverityWarning,
	}

	f.errorRegistry["E3002"] = ErrorInfo{
		Code:        "E3002",
		Title:       "Connection refused",
		Description: "The connection to the remote service was refused.",
		Fix:         "Verify the service is running:",
		FixCommand:  "systemctl status {service}",
		Hint:        "Check if the service is listening on the expected port",
		DocsURL:     "https://docs.opencenter.cloud/errors/E3002",
		Severity:    SeverityWarning,
	}

	// E4xxx: File system errors
	f.errorRegistry["E4001"] = ErrorInfo{
		Code:        "E4001",
		Title:       "File not found",
		Description: "The specified file or directory does not exist.",
		Fix:         "Verify the path exists:",
		FixCommand:  "ls -la {path}",
		Hint:        "Check for typos in the file path",
		DocsURL:     "https://docs.opencenter.cloud/errors/E4001",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E4002"] = ErrorInfo{
		Code:        "E4002",
		Title:       "Permission denied",
		Description: "You do not have permission to access this file or directory.",
		Fix:         "Grant appropriate permissions:",
		FixCommand:  "chmod +w {path}",
		Hint:        "Check file ownership: ls -la {path}",
		DocsURL:     "https://docs.opencenter.cloud/errors/E4002",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E4003"] = ErrorInfo{
		Code:        "E4003",
		Title:       "Disk space exhausted",
		Description: "There is not enough disk space to complete the operation.",
		Fix:         "Free up disk space:",
		FixCommand:  "df -h",
		Hint:        "Remove old logs or unused files",
		DocsURL:     "https://docs.opencenter.cloud/errors/E4003",
		Severity:    SeverityCritical,
	}

	// E5xxx: Provider errors
	f.errorRegistry["E5001"] = ErrorInfo{
		Code:        "E5001",
		Title:       "OpenStack API error",
		Description: "An error occurred while communicating with the OpenStack API.",
		Fix:         "Verify OpenStack credentials and connectivity:",
		FixCommand:  "openstack server list",
		Hint:        "Run preflight checks: opencenter cluster doctor {cluster}",
		DocsURL:     "https://docs.opencenter.cloud/errors/E5001",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E5002"] = ErrorInfo{
		Code:        "E5002",
		Title:       "AWS API error",
		Description: "An error occurred while communicating with the AWS API.",
		Fix:         "Verify AWS credentials:",
		FixCommand:  "aws sts get-caller-identity",
		Hint:        "Check AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables",
		DocsURL:     "https://docs.opencenter.cloud/errors/E5002",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E5003"] = ErrorInfo{
		Code:        "E5003",
		Title:       "Provider authentication failed",
		Description: "Authentication with the cloud provider failed.",
		Fix:         "Verify your credentials are correct:",
		Hint:        "Check credential expiration and permissions",
		DocsURL:     "https://docs.opencenter.cloud/errors/E5003",
		Severity:    SeverityCritical,
	}

	// E6xxx: Operational errors
	f.errorRegistry["E6001"] = ErrorInfo{
		Code:        "E6001",
		Title:       "Drift detection failed",
		Description: "Unable to detect configuration drift for the cluster.",
		Fix:         "Verify cluster is accessible:",
		FixCommand:  "opencenter cluster describe {cluster}",
		Hint:        "Check cloud provider connectivity",
		DocsURL:     "https://docs.opencenter.cloud/errors/E6001",
		Severity:    SeverityWarning,
	}

	f.errorRegistry["E6002"] = ErrorInfo{
		Code:        "E6002",
		Title:       "Backup creation failed",
		Description: "Unable to create a backup of the cluster configuration.",
		Fix:         "Check disk space and permissions:",
		FixCommand:  "df -h && ls -la ~/.config/opencenter/backups/",
		Hint:        "Ensure the backup directory is writable",
		DocsURL:     "https://docs.opencenter.cloud/errors/E6002",
		Severity:    SeverityCritical,
	}

	f.errorRegistry["E6003"] = ErrorInfo{
		Code:        "E6003",
		Title:       "Lock acquisition failed",
		Description: "Unable to acquire a lock for the cluster operation.",
		Fix:         "Wait for the current operation to complete or break the lock:",
		FixCommand:  "opencenter cluster lock break {cluster}",
		Hint:        "Check if another operation is in progress",
		DocsURL:     "https://docs.opencenter.cloud/errors/E6003",
		Severity:    SeverityWarning,
	}

	f.errorRegistry["E6004"] = ErrorInfo{
		Code:        "E6004",
		Title:       "Retry budget exhausted",
		Description: "The operation failed after exhausting all retry attempts.",
		Fix:         "Check the underlying error and retry manually:",
		Hint:        "The service may be experiencing issues",
		DocsURL:     "https://docs.opencenter.cloud/errors/E6004",
		Severity:    SeverityCritical,
	}
}
