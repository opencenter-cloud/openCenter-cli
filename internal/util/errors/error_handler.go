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

package errors

import (
	"fmt"
	"os"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/security"
)

// DefaultErrorHandler implements ErrorHandler interface
type DefaultErrorHandler struct {
	suggestionMap map[ErrorType][]string
	masker        security.CredentialMasker
}

// NewDefaultErrorHandler creates a new default error handler
func NewDefaultErrorHandler() *DefaultErrorHandler {
	handler := &DefaultErrorHandler{
		suggestionMap: make(map[ErrorType][]string),
		masker:        security.NewDefaultCredentialMasker(),
	}
	handler.initializeSuggestions()
	return handler
}

// HandleError converts an error to a structured error
func (h *DefaultErrorHandler) HandleError(err error) *StructuredError {
	if err == nil {
		return nil
	}

	// Check if it's already a structured error
	if structuredErr, ok := err.(*StructuredError); ok {
		// Mask the error message
		structuredErr.Message = h.masker.MaskString(structuredErr.Message)
		return structuredErr
	}

	// Determine error type based on error message
	errorType := h.determineErrorType(err)

	// Mask the error message
	maskedMessage := h.masker.MaskString(err.Error())

	return &StructuredError{
		Type:        errorType,
		Message:     maskedMessage,
		Cause:       err,
		Suggestions: h.GetSuggestions(err),
		Retryable:   h.IsRetryable(err),
	}
}

// FormatError formats an error for display
func (h *DefaultErrorHandler) FormatError(err error) string {
	if err == nil {
		return ""
	}

	structuredErr := h.HandleError(err)

	var parts []string

	// Add error type if not user error
	if structuredErr.Type != UserError {
		parts = append(parts, fmt.Sprintf("[%s]", strings.ToUpper(string(structuredErr.Type))))
	}

	// Add operation if present
	if structuredErr.Operation != "" {
		parts = append(parts, fmt.Sprintf("Operation '%s':", structuredErr.Operation))
	}

	// Add file context if present
	if structuredErr.FilePath != "" {
		fileContext := structuredErr.FilePath
		if structuredErr.LineNumber > 0 {
			fileContext += fmt.Sprintf(":%d", structuredErr.LineNumber)
			if structuredErr.ColumnNumber > 0 {
				fileContext += fmt.Sprintf(":%d", structuredErr.ColumnNumber)
			}
		}
		parts = append(parts, fmt.Sprintf("at %s:", fileContext))
	}

	// Add field if present
	if structuredErr.Field != "" {
		parts = append(parts, fmt.Sprintf("Field '%s':", structuredErr.Field))
	}

	// Add masked message
	maskedMessage := h.masker.MaskString(structuredErr.Message)
	parts = append(parts, maskedMessage)

	result := strings.Join(parts, " ")

	// Add suggestions if available
	if len(structuredErr.Suggestions) > 0 {
		result += "\n\nSuggestions:"
		for _, suggestion := range structuredErr.Suggestions {
			result += "\n  - " + suggestion
		}
	}

	return result
}

// GetSuggestions returns suggestions for resolving an error
func (h *DefaultErrorHandler) GetSuggestions(err error) []string {
	if err == nil {
		return nil
	}

	// Check if it's a structured error with existing suggestions
	if structuredErr, ok := err.(*StructuredError); ok {
		if len(structuredErr.Suggestions) > 0 {
			return structuredErr.Suggestions
		}
	}

	errorType := h.determineErrorType(err)
	suggestions := make([]string, 0)

	// Start with type-specific suggestions
	if typeSuggestions, ok := h.suggestionMap[errorType]; ok {
		suggestions = append(suggestions, typeSuggestions...)
	}

	// Add context-specific suggestions based on error content
	errorMsg := strings.ToLower(err.Error())

	// Permission errors
	if strings.Contains(errorMsg, "permission denied") || strings.Contains(errorMsg, "access denied") {
		suggestions = h.addUnique(suggestions, "Run: chmod +w <file> to grant write permissions")
		suggestions = h.addUnique(suggestions, "Check ownership with: ls -la <file>")
		suggestions = h.addUnique(suggestions, "Ensure you're running with appropriate user privileges")
		if strings.Contains(errorMsg, "directory") {
			suggestions = h.addUnique(suggestions, "For directories, use: chmod +wx <directory>")
		}
	}

	// File not found errors
	if strings.Contains(errorMsg, "no such file or directory") || strings.Contains(errorMsg, "file not found") {
		suggestions = h.addUnique(suggestions, "Verify the path exists: ls -la <path>")
		suggestions = h.addUnique(suggestions, "Check for typos in the file or directory name")
		suggestions = h.addUnique(suggestions, "Ensure parent directories exist: mkdir -p <parent-dir>")
		if strings.Contains(errorMsg, "config") {
			suggestions = h.addUnique(suggestions, "Initialize configuration with: opencenter cluster init")
		}
	}

	// Validation errors
	if strings.Contains(errorMsg, "invalid") || strings.Contains(errorMsg, "validation") {
		suggestions = h.addUnique(suggestions, "Run: opencenter cluster validate to check configuration")
		suggestions = h.addUnique(suggestions, "View schema with: opencenter cluster schema")
		suggestions = h.addUnique(suggestions, "Check documentation at: https://docs.opencenter.io")
		if strings.Contains(errorMsg, "yaml") || strings.Contains(errorMsg, "syntax") {
			suggestions = h.addUnique(suggestions, "Validate YAML syntax with: yamllint <file>")
		}
	}

	// Connection errors
	if strings.Contains(errorMsg, "connection refused") || strings.Contains(errorMsg, "connection failed") {
		suggestions = h.addUnique(suggestions, "Test connectivity with: ping <host>")
		suggestions = h.addUnique(suggestions, "Check if service is running: systemctl status <service>")
		suggestions = h.addUnique(suggestions, "Verify firewall rules: sudo iptables -L")
		suggestions = h.addUnique(suggestions, "Check network configuration: ip addr show")
	}

	// Timeout errors
	if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "timed out") {
		suggestions = h.addUnique(suggestions, "Retry the operation after a brief wait")
		suggestions = h.addUnique(suggestions, "Increase timeout value if configurable")
		suggestions = h.addUnique(suggestions, "Check service health and response time")
		suggestions = h.addUnique(suggestions, "Verify network latency: ping -c 5 <host>")
	}

	// SOPS/encryption errors
	if strings.Contains(errorMsg, "sops") || strings.Contains(errorMsg, "encryption") || strings.Contains(errorMsg, "age") {
		suggestions = h.addUnique(suggestions, "Verify SOPS installation: sops --version")
		suggestions = h.addUnique(suggestions, "Check age key file: cat $SOPS_AGE_KEY_FILE")
		suggestions = h.addUnique(suggestions, "Set key file path: export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt")
		suggestions = h.addUnique(suggestions, "Generate new age key: age-keygen -o ~/.config/sops/age/keys.txt")
	}

	// Template errors
	if strings.Contains(errorMsg, "template") {
		suggestions = h.addUnique(suggestions, "Check template syntax for Go template errors")
		suggestions = h.addUnique(suggestions, "Verify all template variables are defined")
		suggestions = h.addUnique(suggestions, "Test template rendering with: opencenter cluster render")
		suggestions = h.addUnique(suggestions, "Review template documentation for required variables")
	}

	// Cloud provider errors
	if strings.Contains(errorMsg, "openstack") || strings.Contains(errorMsg, "aws") || strings.Contains(errorMsg, "cloud") {
		suggestions = h.addUnique(suggestions, "Verify cloud credentials are set correctly")
		suggestions = h.addUnique(suggestions, "Check credential environment variables")
		suggestions = h.addUnique(suggestions, "Test API connectivity with provider CLI tools")
		suggestions = h.addUnique(suggestions, "Run preflight checks: opencenter cluster preflight")
	}

	// Configuration errors
	if strings.Contains(errorMsg, "config") {
		suggestions = h.addUnique(suggestions, "Edit configuration: opencenter cluster edit")
		suggestions = h.addUnique(suggestions, "View current config: opencenter cluster info")
		suggestions = h.addUnique(suggestions, "Validate configuration: opencenter cluster validate")
		suggestions = h.addUnique(suggestions, "Check configuration schema: opencenter cluster schema")
	}

	// Service errors
	if strings.Contains(errorMsg, "service") {
		suggestions = h.addUnique(suggestions, "List available services in documentation")
		suggestions = h.addUnique(suggestions, "Check service dependencies are enabled")
		suggestions = h.addUnique(suggestions, "Verify service configuration in cluster config")
		suggestions = h.addUnique(suggestions, "Review service-specific requirements")
	}

	// GitOps generation errors
	if strings.Contains(errorMsg, "gitops") || strings.Contains(errorMsg, "generation") {
		suggestions = h.addUnique(suggestions, "Check workspace permissions: ls -la <workspace>")
		suggestions = h.addUnique(suggestions, "Verify template files exist and are readable")
		suggestions = h.addUnique(suggestions, "Ensure sufficient disk space: df -h")
		suggestions = h.addUnique(suggestions, "Review generation logs for detailed errors")
	}

	// Disk space errors
	if strings.Contains(errorMsg, "no space left") || strings.Contains(errorMsg, "disk full") {
		suggestions = h.addUnique(suggestions, "Check disk space: df -h")
		suggestions = h.addUnique(suggestions, "Clean up temporary files: rm -rf /tmp/*")
		suggestions = h.addUnique(suggestions, "Remove old logs or unused files")
		suggestions = h.addUnique(suggestions, "Consider expanding disk or moving to larger volume")
	}

	// Network errors
	if strings.Contains(errorMsg, "network") || strings.Contains(errorMsg, "dns") {
		suggestions = h.addUnique(suggestions, "Test DNS resolution: nslookup <hostname>")
		suggestions = h.addUnique(suggestions, "Check network interfaces: ip link show")
		suggestions = h.addUnique(suggestions, "Verify routing table: ip route show")
		suggestions = h.addUnique(suggestions, "Test connectivity: curl -v <url>")
	}

	// If no specific suggestions were added, provide general guidance
	if len(suggestions) == 0 {
		suggestions = []string{
			"Review the error message for specific details",
			"Check system logs for additional context",
			"Consult documentation at https://docs.opencenter.io",
			"Run with --debug flag for verbose output",
		}
	}

	return suggestions
}

// addUnique adds a suggestion only if it's not already in the list
func (h *DefaultErrorHandler) addUnique(suggestions []string, newSuggestion string) []string {
	for _, existing := range suggestions {
		if existing == newSuggestion {
			return suggestions
		}
	}
	return append(suggestions, newSuggestion)
}

// IsRetryable determines if an error is retryable
func (h *DefaultErrorHandler) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := strings.ToLower(err.Error())

	// Network-related errors are usually retryable
	if strings.Contains(errorMsg, "timeout") ||
		strings.Contains(errorMsg, "connection refused") ||
		strings.Contains(errorMsg, "network") ||
		strings.Contains(errorMsg, "temporary") {
		return true
	}

	// File system errors that might be temporary
	if strings.Contains(errorMsg, "resource temporarily unavailable") ||
		strings.Contains(errorMsg, "device busy") {
		return true
	}

	// Validation and permission errors are usually not retryable
	if strings.Contains(errorMsg, "invalid") ||
		strings.Contains(errorMsg, "permission denied") ||
		strings.Contains(errorMsg, "access denied") {
		return false
	}

	return false
}

// determineErrorType determines the error type based on error content
func (h *DefaultErrorHandler) determineErrorType(err error) ErrorType {
	if err == nil {
		return SystemError
	}

	errorMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errorMsg, "validation") || strings.Contains(errorMsg, "invalid"):
		return ValidationError

	case strings.Contains(errorMsg, "path") || strings.Contains(errorMsg, "directory") || strings.Contains(errorMsg, "file"):
		return PathError

	case strings.Contains(errorMsg, "permission") || strings.Contains(errorMsg, "access"):
		return PermissionError

	case strings.Contains(errorMsg, "template"):
		return TemplateError

	case strings.Contains(errorMsg, "sops") || strings.Contains(errorMsg, "encryption"):
		return SOPSError

	case strings.Contains(errorMsg, "config"):
		return ConfigError

	case strings.Contains(errorMsg, "network") || strings.Contains(errorMsg, "connection"):
		return NetworkError

	case strings.Contains(errorMsg, "cloud") || strings.Contains(errorMsg, "aws") || strings.Contains(errorMsg, "openstack"):
		return CloudError

	case strings.Contains(errorMsg, "credential") || strings.Contains(errorMsg, "authentication") || strings.Contains(errorMsg, "unauthorized"):
		return CredentialError

	case strings.Contains(errorMsg, "service") || strings.Contains(errorMsg, "plugin"):
		return ServiceError

	case strings.Contains(errorMsg, "generation") || strings.Contains(errorMsg, "gitops") || strings.Contains(errorMsg, "workspace"):
		return GenerationError

	default:
		return SystemError
	}
}

// initializeSuggestions initializes the suggestion map with actionable guidance
func (h *DefaultErrorHandler) initializeSuggestions() {
	h.suggestionMap[ValidationError] = []string{
		"Run: opencenter cluster validate to check configuration",
		"View schema with: opencenter cluster schema",
		"Check documentation at: https://docs.opencenter.io",
	}

	h.suggestionMap[PathError] = []string{
		"Verify the path exists: ls -la <path>",
		"Check for typos in the path",
		"Ensure parent directories exist: mkdir -p <parent-dir>",
	}

	h.suggestionMap[PermissionError] = []string{
		"Check file permissions: ls -la <file>",
		"Grant write access: chmod +w <file>",
		"Verify ownership: chown <user>:<group> <file>",
	}

	h.suggestionMap[TemplateError] = []string{
		"Check template syntax for Go template errors",
		"Verify all template variables are defined",
		"Test template rendering: opencenter cluster render",
	}

	h.suggestionMap[SOPSError] = []string{
		"Verify SOPS installation: sops --version",
		"Check age key file: cat $SOPS_AGE_KEY_FILE",
		"Set key file path: export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt",
		"Generate new age key: age-keygen -o ~/.config/sops/age/keys.txt",
	}

	h.suggestionMap[ConfigError] = []string{
		"Edit configuration: opencenter cluster edit",
		"Validate configuration: opencenter cluster validate",
		"View current config: opencenter cluster info",
	}

	h.suggestionMap[NetworkError] = []string{
		"Test connectivity: ping <host>",
		"Check DNS resolution: nslookup <hostname>",
		"Verify firewall rules: sudo iptables -L",
		"Test with curl: curl -v <url>",
	}

	h.suggestionMap[FileError] = []string{
		"Check file exists: ls -la <file>",
		"Verify file permissions: stat <file>",
		"Check disk space: df -h",
	}

	h.suggestionMap[SystemError] = []string{
		"Check system resources: top or htop",
		"Review system logs: journalctl -xe",
		"Verify dependencies: which <command>",
	}

	h.suggestionMap[CloudError] = []string{
		"Verify cloud credentials are set correctly",
		"Test API connectivity with provider CLI tools",
		"Run preflight checks: opencenter cluster preflight",
		"Check cloud provider service status",
	}

	h.suggestionMap[CredentialError] = []string{
		"Verify credentials are correctly configured",
		"Check credential environment variables: env | grep <PROVIDER>",
		"Ensure credentials haven't expired",
		"Use SOPS to encrypt sensitive credentials",
	}

	h.suggestionMap[ServiceError] = []string{
		"Check service configuration in cluster config",
		"Verify service dependencies are enabled",
		"Review service documentation for requirements",
		"List available services in documentation",
	}

	h.suggestionMap[GenerationError] = []string{
		"Check workspace permissions: ls -la <workspace>",
		"Verify template files are accessible",
		"Ensure sufficient disk space: df -h",
		"Review generation logs with --debug flag",
	}
}

// CreateValidationError creates a validation error with suggestions
func CreateValidationError(field, message string, suggestions ...string) *StructuredError {
	return &StructuredError{
		Type:        ValidationError,
		Field:       field,
		Message:     message,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// CreatePathError creates a path-related error with suggestions
func CreatePathError(path, message string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Verify the path exists: ls -la %s", path),
		"Check for typos in the path",
		fmt.Sprintf("Ensure parent directories exist: mkdir -p %s", path),
		"Check path permissions and ownership",
	}

	return &StructuredError{
		Type:        PathError,
		Field:       "path",
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"path": path},
		Retryable:   false,
	}
}

// CreatePermissionError creates a permission-related error
func CreatePermissionError(resource, operation string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Check permissions: ls -la %s", resource),
		fmt.Sprintf("Grant %s access: chmod +w %s", operation, resource),
		fmt.Sprintf("Verify ownership: chown <user>:<group> %s", resource),
		"Run with appropriate privileges if needed (e.g., sudo)",
	}

	return &StructuredError{
		Type:        PermissionError,
		Field:       "permissions",
		Message:     fmt.Sprintf("Permission denied for %s operation on %s", operation, resource),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"resource": resource, "operation": operation},
		Retryable:   false,
	}
}

// CreateSOPSError creates a SOPS-related error with suggestions
func CreateSOPSError(operation, message string, cause error) *StructuredError {
	suggestions := []string{
		"Verify SOPS installation: sops --version",
		"Check age key file: cat $SOPS_AGE_KEY_FILE",
		"Set key file path: export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt",
		"Generate new age key: age-keygen -o ~/.config/sops/age/keys.txt",
		"Validate key file permissions: chmod 600 $SOPS_AGE_KEY_FILE",
	}

	return &StructuredError{
		Type:        SOPSError,
		Field:       "sops",
		Message:     fmt.Sprintf("SOPS %s failed: %s", operation, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"operation": operation},
		Retryable:   false,
	}
}

// CreateConfigError creates a configuration-related error
func CreateConfigError(field, message string, cause error) *StructuredError {
	suggestions := []string{
		"Edit configuration: opencenter cluster edit",
		"Validate configuration: opencenter cluster validate",
		"View schema: opencenter cluster schema",
		"Check documentation at: https://docs.opencenter.io",
	}

	return &StructuredError{
		Type:        ConfigError,
		Field:       field,
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		Retryable:   false,
	}
}

// IsFileNotFoundError checks if an error is a file not found error
func IsFileNotFoundError(err error) bool {
	return os.IsNotExist(err)
}

// IsPermissionError checks if an error is a permission error
func IsPermissionError(err error) bool {
	return os.IsPermission(err)
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

// CreateCloudError creates a cloud provider-related error
func CreateCloudError(provider, operation, message string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Verify %s credentials: check environment variables", provider),
		fmt.Sprintf("Test %s API connectivity with provider CLI", provider),
		"Run preflight checks: opencenter cluster preflight",
		fmt.Sprintf("Check %s service status page", provider),
		"Verify network connectivity to cloud APIs",
	}

	return &StructuredError{
		Type:        CloudError,
		Field:       "cloud_provider",
		Message:     fmt.Sprintf("%s %s failed: %s", provider, operation, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"provider": provider, "operation": operation},
		Retryable:   true, // Cloud operations are often retryable
	}
}

// CreateCredentialError creates a credential-related error
func CreateCredentialError(credentialType, field, message string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Verify %s credentials are set: env | grep %s", credentialType, strings.ToUpper(credentialType)),
		"Check credential expiration dates",
		"Ensure proper permissions for the credentials",
		"Use SOPS to encrypt sensitive credentials: sops encrypt <file>",
		fmt.Sprintf("Refer to %s documentation for credential setup", credentialType),
	}

	return &StructuredError{
		Type:        CredentialError,
		Field:       field,
		Message:     fmt.Sprintf("%s credential error: %s", credentialType, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"credential_type": credentialType},
		Retryable:   false, // Credential errors usually require manual fix
	}
}

// CreateServiceError creates a service-related error
func CreateServiceError(serviceName, operation, message string, cause error) *StructuredError {
	suggestions := []string{
		fmt.Sprintf("Check %s service configuration in cluster config", serviceName),
		"Verify service dependencies are enabled",
		fmt.Sprintf("Review %s service documentation for requirements", serviceName),
		"List available services in documentation",
		"Validate service configuration: opencenter cluster validate",
	}

	return &StructuredError{
		Type:        ServiceError,
		Field:       "service",
		Message:     fmt.Sprintf("%s service %s failed: %s", serviceName, operation, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"service": serviceName, "operation": operation},
		Retryable:   false,
	}
}

// CreateTemplateError creates a template-related error with file context
func CreateTemplateError(templatePath string, lineNumber int, message string, cause error) *StructuredError {
	suggestions := []string{
		"Verify template syntax is correct",
		"Check that all required variables are provided",
		fmt.Sprintf("Review template file: cat %s", templatePath),
		"Test template rendering: opencenter cluster render",
	}

	return &StructuredError{
		Type:        TemplateError,
		Field:       "template",
		Message:     message,
		Cause:       cause,
		Suggestions: suggestions,
		FilePath:    templatePath,
		LineNumber:  lineNumber,
		Operation:   "template_rendering",
		Retryable:   false,
	}
}

// CreateTemplateErrorWithColumn creates a template error with full file context
func CreateTemplateErrorWithColumn(templatePath string, lineNumber, columnNumber int, message string, cause error) *StructuredError {
	suggestions := []string{
		"Verify template syntax is correct",
		"Check that all required variables are provided",
		fmt.Sprintf("Review template file: cat %s", templatePath),
		"Test template rendering: opencenter cluster render",
	}

	return &StructuredError{
		Type:         TemplateError,
		Field:        "template",
		Message:      message,
		Cause:        cause,
		Suggestions:  suggestions,
		FilePath:     templatePath,
		LineNumber:   lineNumber,
		ColumnNumber: columnNumber,
		Operation:    "template_rendering",
		Retryable:    false,
	}
}

// CreateGenerationError creates a GitOps generation-related error
func CreateGenerationError(stage, message string, cause error) *StructuredError {
	suggestions := []string{
		"Check GitOps workspace permissions: ls -la <workspace>",
		"Verify template files are accessible",
		"Ensure sufficient disk space: df -h",
		"Review generation logs with --debug flag",
		"Validate configuration: opencenter cluster validate",
	}

	return &StructuredError{
		Type:        GenerationError,
		Field:       "generation",
		Message:     fmt.Sprintf("GitOps generation failed at stage '%s': %s", stage, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"stage": stage},
		Operation:   "gitops_generation",
		Retryable:   false,
	}
}

// CreateGenerationErrorWithFile creates a GitOps generation error with file context
func CreateGenerationErrorWithFile(stage, filePath string, message string, cause error) *StructuredError {
	suggestions := []string{
		"Check GitOps workspace permissions: ls -la <workspace>",
		fmt.Sprintf("Verify template file exists: ls -la %s", filePath),
		"Ensure sufficient disk space: df -h",
		"Review generation logs with --debug flag",
		"Validate configuration: opencenter cluster validate",
	}

	return &StructuredError{
		Type:        GenerationError,
		Field:       "generation",
		Message:     fmt.Sprintf("GitOps generation failed at stage '%s': %s", stage, message),
		Cause:       cause,
		Suggestions: suggestions,
		Context:     map[string]interface{}{"stage": stage},
		FilePath:    filePath,
		Operation:   "gitops_generation",
		Retryable:   false,
	}
}
