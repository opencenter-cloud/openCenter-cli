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
	"regexp"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// SecurityValidator validates security-related inputs.
type SecurityValidator struct {
	shellMetachars    []string
	dangerousPatterns []*regexp.Regexp
	safeEditors       map[string]bool
}

// NewSecurityValidator creates a new security validator.
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{
		shellMetachars: []string{";", "|", "&", "$", "`", "\n", "\r", "<", ">", "(", ")", "{", "}"},
		dangerousPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\$\([^)]*\)`),                    // Command substitution $(...)
			regexp.MustCompile("`[^`]*`"),                        // Command substitution `...`
			regexp.MustCompile(`\${[^}]*}`),                      // Variable expansion ${...}
			regexp.MustCompile(`&&|\|\||;`),                      // Command chaining
			regexp.MustCompile(`>\s*[/\w]|<\s*[/\w]`),            // Redirection
			regexp.MustCompile(`\b(rm|del|format|mkfs)\s+-[rf]`), // Dangerous commands
		},
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
	}
}

// Name returns the validator name.
func (v *SecurityValidator) Name() string {
	return "security"
}

// Validate validates security-related inputs.
// The value should be a map with "type" and "value" keys.
func (v *SecurityValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := &validation.ValidationResult{
		Valid:    true,
		Errors:   []*validation.ValidationIssue{},
		Warnings: []*validation.ValidationIssue{},
		Info:     []*validation.ValidationIssue{},
	}

	securityMap, ok := value.(map[string]interface{})
	if !ok {
		result.AddError("security", "value must be a map with 'type' and 'value' keys")
		return result, nil
	}

	securityType, ok := securityMap["type"].(string)
	if !ok {
		result.AddError("security", "missing or invalid 'type' field")
		return result, nil
	}

	securityValue := securityMap["value"]

	switch securityType {
	case "shell-input":
		v.validateShellInput(result, securityValue)
	case "environment-variable":
		v.validateEnvironmentVariable(result, securityMap)
	case "editor":
		v.validateEditor(result, securityValue)
	case "command":
		v.validateCommand(result, securityValue)
	case "secret":
		v.validateSecret(result, securityValue)
	default:
		result.AddWarning("security", fmt.Sprintf("unknown security type '%s', skipping validation", securityType))
	}

	return result, nil
}

// validateShellInput validates shell input for injection attacks.
func (v *SecurityValidator) validateShellInput(result *validation.ValidationResult, value interface{}) {
	input, ok := value.(string)
	if !ok {
		result.AddError("shell_input", "shell input must be a string")
		return
	}

	if input == "" {
		return // Empty input is safe
	}

	// Check for dangerous metacharacters
	dangerousChars := []string{";", "|", "&", "`", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(input, char) {
			result.AddError("shell_input",
				fmt.Sprintf("input contains dangerous shell metacharacter: %s", char),
				"Remove shell metacharacters from the input",
				"Use proper escaping or avoid shell execution")
			return
		}
	}

	// Check for dangerous patterns
	for _, pattern := range v.dangerousPatterns {
		if pattern.MatchString(input) {
			result.AddError("shell_input",
				fmt.Sprintf("input contains dangerous pattern: %s", pattern.String()),
				"Remove command substitution and shell expansion patterns",
				"Use direct function calls instead of shell execution")
			return
		}
	}

	// Warn about other metacharacters
	for _, metachar := range v.shellMetachars {
		if strings.Contains(input, metachar) {
			result.AddWarning("shell_input",
				fmt.Sprintf("input contains shell metacharacter: %s", metachar),
				"Consider escaping or removing this character",
				"Ensure proper quoting if this input is used in shell commands")
			break
		}
	}
}

// validateEnvironmentVariable validates environment variable names and values.
func (v *SecurityValidator) validateEnvironmentVariable(result *validation.ValidationResult, securityMap map[string]interface{}) {
	nameVal, ok := securityMap["name"]
	if !ok {
		result.AddError("environment_variable", "missing 'name' field")
		return
	}

	name, ok := nameVal.(string)
	if !ok {
		result.AddError("environment_variable", "name must be a string")
		return
	}

	if name == "" {
		result.AddError("environment_variable", "environment variable name cannot be empty")
		return
	}

	// Validate variable name format
	validNamePattern := regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	if !validNamePattern.MatchString(name) {
		result.AddWarning("environment_variable",
			"environment variable name should be uppercase with underscores",
			"Example: MY_VARIABLE, API_KEY, DATABASE_URL")
	}

	// Get value
	valueVal := securityMap["value"]
	value, ok := valueVal.(string)
	if !ok {
		// Value might not be a string (could be number, bool, etc.)
		return
	}

	// Special handling for EDITOR variable
	if name == "EDITOR" || name == "VISUAL" {
		v.validateEditor(result, value)
		return
	}

	// Check for shell metacharacters in value
	for _, metachar := range v.shellMetachars {
		if strings.Contains(value, metachar) {
			result.AddError("environment_variable",
				fmt.Sprintf("environment variable value contains shell metacharacter: %s", metachar),
				"Remove shell metacharacters from the value",
				"Use proper escaping if metacharacters are required")
			return
		}
	}

	// Warn about potential secrets
	secretKeywords := []string{"password", "secret", "key", "token", "credential", "auth"}
	lowerName := strings.ToLower(name)
	for _, keyword := range secretKeywords {
		if strings.Contains(lowerName, keyword) {
			result.AddWarning("environment_variable",
				fmt.Sprintf("environment variable name suggests it contains a secret: %s", name),
				"Ensure secrets are properly encrypted and not logged",
				"Consider using a secrets management system")
			break
		}
	}
}

// validateEditor validates the EDITOR environment variable.
func (v *SecurityValidator) validateEditor(result *validation.ValidationResult, value interface{}) {
	editor, ok := value.(string)
	if !ok {
		result.AddError("editor", "editor must be a string")
		return
	}

	if editor == "" {
		return // Empty is acceptable, system will use default
	}

	// Check for shell metacharacters
	for _, metachar := range v.shellMetachars {
		if strings.Contains(editor, metachar) {
			result.AddError("editor",
				fmt.Sprintf("editor value contains shell metacharacter: %s", metachar),
				"Remove shell metacharacters from the editor value",
				"Use only the editor command name")
			return
		}
	}

	// Extract command name (remove path and arguments)
	editorCmd := editor
	if idx := strings.Index(editor, " "); idx != -1 {
		editorCmd = editor[:idx]
	}
	if idx := strings.LastIndex(editorCmd, "/"); idx != -1 {
		editorCmd = editorCmd[idx+1:]
	}
	if idx := strings.LastIndex(editorCmd, "\\"); idx != -1 {
		editorCmd = editorCmd[idx+1:]
	}

	// Check against whitelist
	if !v.safeEditors[editorCmd] {
		result.AddWarning("editor",
			fmt.Sprintf("editor '%s' is not in the safe editors whitelist", editorCmd),
			"Allowed editors: vim, vi, nvim, nano, emacs, code, subl, atom, gedit",
			"Ensure this editor is safe and trusted")
	}
}

// validateCommand validates a command for security issues.
func (v *SecurityValidator) validateCommand(result *validation.ValidationResult, value interface{}) {
	command, ok := value.(string)
	if !ok {
		result.AddError("command", "command must be a string")
		return
	}

	if command == "" {
		result.AddError("command", "command cannot be empty")
		return
	}

	// Check for dangerous patterns
	for _, pattern := range v.dangerousPatterns {
		if pattern.MatchString(command) {
			result.AddError("command",
				fmt.Sprintf("command contains dangerous pattern: %s", pattern.String()),
				"Remove command substitution and shell expansion patterns",
				"Use direct function calls instead of shell execution")
			return
		}
	}

	// Check for dangerous commands
	dangerousCommands := []string{
		"rm -rf /",
		"mkfs",
		"dd if=",
		":(){ :|:& };:",
		"chmod -R 777",
		"chown -R",
	}

	for _, dangerous := range dangerousCommands {
		if strings.Contains(command, dangerous) {
			result.AddError("command",
				fmt.Sprintf("command contains dangerous operation: %s", dangerous),
				"This command could cause data loss or security issues",
				"Review the command carefully before execution")
			return
		}
	}

	// Warn about sudo usage
	if strings.HasPrefix(strings.TrimSpace(command), "sudo ") {
		result.AddWarning("command",
			"command uses sudo (elevated privileges)",
			"Ensure elevated privileges are necessary",
			"Review the command for security implications")
	}

	// Warn about command chaining
	if strings.Contains(command, "&&") || strings.Contains(command, "||") || strings.Contains(command, ";") {
		result.AddWarning("command",
			"command contains chaining operators (&&, ||, ;)",
			"Consider splitting into separate commands for better error handling",
			"Ensure all chained commands are safe")
	}
}

// validateSecret validates that a value doesn't contain plaintext secrets.
func (v *SecurityValidator) validateSecret(result *validation.ValidationResult, value interface{}) {
	secret, ok := value.(string)
	if !ok {
		result.AddError("secret", "secret must be a string")
		return
	}

	if secret == "" {
		result.AddError("secret", "secret cannot be empty")
		return
	}

	// Check for common secret patterns
	secretPatterns := map[string]*regexp.Regexp{
		"AWS Access Key":     regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"AWS Secret Key":     regexp.MustCompile(`^[0-9a-zA-Z/+]{40}$`),
		"GitHub Token":       regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`),
		"Generic API Key":    regexp.MustCompile(`[aA][pP][iI]_?[kK][eE][yY].*[0-9a-zA-Z]{32,}`),
		"Private Key":        regexp.MustCompile(`-----BEGIN.*PRIVATE KEY-----`),
		"Password in URL":    regexp.MustCompile(`://[^:]+:[^@]+@`),
		"JWT Token":          regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`),
		"Generic Secret":     regexp.MustCompile(`[sS][eE][cC][rR][eE][tT].*[0-9a-zA-Z]{16,}`),
		"Generic Token":      regexp.MustCompile(`[tT][oO][kK][eE][nN].*[0-9a-zA-Z]{16,}`),
		"Generic Credential": regexp.MustCompile(`[cC][rR][eE][dD][eE][nN][tT][iI][aA][lL].*[0-9a-zA-Z]{16,}`),
	}

	for name, pattern := range secretPatterns {
		if pattern.MatchString(secret) {
			result.AddError("secret",
				fmt.Sprintf("value appears to contain a plaintext %s", name),
				"Never store secrets in plaintext",
				"Use SOPS encryption or a secrets management system",
				"Rotate the secret immediately if it was exposed")
			return
		}
	}

	// Check for SOPS encryption markers
	if strings.Contains(secret, "ENC[") || strings.Contains(secret, "sops:") {
		result.AddInfo("secret", "value appears to be SOPS encrypted")
		return
	}

	// Warn about long random-looking strings
	if len(secret) >= 32 {
		// Check if it looks like a random string (high entropy)
		hasUpper := strings.ContainsAny(secret, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		hasLower := strings.ContainsAny(secret, "abcdefghijklmnopqrstuvwxyz")
		hasDigit := strings.ContainsAny(secret, "0123456789")
		hasSpecial := strings.ContainsAny(secret, "!@#$%^&*()_+-=[]{}|;:,.<>?")

		entropyScore := 0
		if hasUpper {
			entropyScore++
		}
		if hasLower {
			entropyScore++
		}
		if hasDigit {
			entropyScore++
		}
		if hasSpecial {
			entropyScore++
		}

		if entropyScore >= 3 {
			result.AddWarning("secret",
				"value looks like a secret (long random string)",
				"Ensure this secret is properly encrypted",
				"Use SOPS or another secrets management system")
		}
	}
}

// SetSafeEditors sets the list of safe editors.
func (v *SecurityValidator) SetSafeEditors(editors []string) {
	v.safeEditors = make(map[string]bool)
	for _, editor := range editors {
		v.safeEditors[editor] = true
	}
}

// AddDangerousPattern adds a dangerous pattern to check for.
func (v *SecurityValidator) AddDangerousPattern(pattern *regexp.Regexp) {
	v.dangerousPatterns = append(v.dangerousPatterns, pattern)
}
