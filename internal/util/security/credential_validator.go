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
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DefaultCredentialValidator implements CredentialValidator interface
type DefaultCredentialValidator struct {
	masker *DefaultCredentialMasker
}

// NewDefaultCredentialValidator creates a new credential validator
func NewDefaultCredentialValidator() *DefaultCredentialValidator {
	return &DefaultCredentialValidator{
		masker: NewDefaultCredentialMasker(),
	}
}

// ValidateNoCredentialsInConfig validates that a config file doesn't contain plaintext credentials
func (v *DefaultCredentialValidator) ValidateNoCredentialsInConfig(configPath string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	matches := v.ScanForCredentials(string(content))
	if len(matches) > 0 {
		return fmt.Errorf("found %d potential credentials in config file %s", len(matches), configPath)
	}

	return nil
}

// ValidateNoCredentialsInLogs validates that a log file doesn't contain credentials
func (v *DefaultCredentialValidator) ValidateNoCredentialsInLogs(logPath string) error {
	content, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	matches := v.ScanForCredentials(string(content))
	if len(matches) > 0 {
		return fmt.Errorf("found %d potential credentials in log file %s", len(matches), logPath)
	}

	return nil
}

// ScanForCredentials scans content for potential credentials
func (v *DefaultCredentialValidator) ScanForCredentials(content string) []CredentialMatch {
	var matches []CredentialMatch

	// Define credential patterns with severity
	patterns := []struct {
		pattern  string
		credType string
		severity string
	}{
		{`AGE-SECRET-KEY-[A-Z0-9]{59}`, "age_secret_key", "critical"},
		{`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----`, "private_key", "critical"},
		{`(?i)(aws[_-]?secret[_-]?access[_-]?key)[\s:=]+['"]*([a-zA-Z0-9/+=]{40})`, "aws_secret", "critical"},
		{`(?i)(password|passwd)[\s:=]+['"]*([^\s'"]{8,})`, "password", "high"},
		{`(?i)(api[_-]?key)[\s:=]+['"]*([a-zA-Z0-9_\-]{20,})`, "api_key", "high"},
		{`(?i)(secret[_-]?key)[\s:=]+['"]*([a-zA-Z0-9_\-]{20,})`, "secret_key", "high"},
		{`(?i)(token)[\s:=]+['"]*([a-zA-Z0-9_\-\.]{20,})`, "token", "medium"},
		{`(?i)bearer\s+([a-zA-Z0-9_\-\.]{20,})`, "bearer_token", "high"},
	}

	lines := strings.Split(content, "\n")

	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)

		for lineNum, line := range lines {
			if re.MatchString(line) {
				// Find all matches in the line
				indices := re.FindStringIndex(line)
				if indices != nil {
					// Get context (mask the actual credential)
					context := line
					if len(context) > 100 {
						start := indices[0]
						if start > 50 {
							start = start - 50
						} else {
							start = 0
						}
						end := indices[1]
						if end+50 < len(context) {
							end = end + 50
						} else {
							end = len(context)
						}
						context = context[start:end]
					}

					// Mask the credential in context
					context = v.masker.MaskString(context)

					matches = append(matches, CredentialMatch{
						Type:     p.credType,
						Line:     lineNum + 1,
						Column:   indices[0] + 1,
						Context:  context,
						Severity: p.severity,
					})
				}
			}
		}
	}

	return matches
}

// ValidateEnvironmentVariables checks for credentials in environment variables
func (v *DefaultCredentialValidator) ValidateEnvironmentVariables() error {
	var issues []string

	// Check for common credential environment variables that should be encrypted
	sensitiveEnvVars := []string{
		"AWS_SECRET_ACCESS_KEY",
		"OPENSTACK_PASSWORD",
		"VSPHERE_PASSWORD",
		"SOPS_AGE_KEY",
		"AGE_SECRET_KEY",
	}

	for _, envVar := range sensitiveEnvVars {
		if value := os.Getenv(envVar); value != "" {
			// Check if the value looks like it might be plaintext
			if !strings.HasPrefix(value, "ENC[") && !strings.Contains(value, "sops") {
				issues = append(issues, fmt.Sprintf("environment variable %s may contain plaintext credential", envVar))
			}
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("credential validation issues: %s", strings.Join(issues, "; "))
	}

	return nil
}

// ScanFileForCredentials scans a file and returns detailed credential matches
func ScanFileForCredentials(filePath string) ([]CredentialMatch, error) {
	validator := NewDefaultCredentialValidator()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan file: %w", err)
	}

	return validator.ScanForCredentials(content.String()), nil
}

// ValidateConfigSecurity performs comprehensive security validation on a config file
func ValidateConfigSecurity(configPath string) error {
	validator := NewDefaultCredentialValidator()

	// Check for plaintext credentials
	if err := validator.ValidateNoCredentialsInConfig(configPath); err != nil {
		return err
	}

	// Check file permissions
	info, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	// Warn if file is world-readable
	mode := info.Mode()
	if mode&0004 != 0 {
		return fmt.Errorf("config file %s is world-readable (permissions: %s)", configPath, mode)
	}

	return nil
}
