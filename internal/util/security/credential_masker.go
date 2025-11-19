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
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaskString is the string used to mask sensitive data
	MaskString = "***REDACTED***"

	// PartialMaskString shows partial information
	PartialMaskString = "***"
)

// DefaultCredentialMasker implements CredentialMasker interface
type DefaultCredentialMasker struct {
	sensitivePatterns []*regexp.Regexp
	sensitiveFields   map[string]bool
}

// NewDefaultCredentialMasker creates a new credential masker with default patterns
func NewDefaultCredentialMasker() *DefaultCredentialMasker {
	masker := &DefaultCredentialMasker{
		sensitivePatterns: make([]*regexp.Regexp, 0),
		sensitiveFields:   make(map[string]bool),
	}

	// Initialize default sensitive patterns
	masker.initializeDefaultPatterns()
	masker.initializeDefaultFields()

	return masker
}

// initializeDefaultPatterns adds common credential patterns
func (m *DefaultCredentialMasker) initializeDefaultPatterns() {
	patterns := []string{
		// API Keys and tokens
		`(?i)(api[_-]?key|apikey)[\s:=]+['"]*([a-zA-Z0-9_\-]{20,})`,
		`(?i)(access[_-]?token|accesstoken)[\s:=]+['"]*([a-zA-Z0-9_\-\.]{20,})`,
		`(?i)(secret[_-]?key|secretkey)[\s:=]+['"]*([a-zA-Z0-9_\-]{20,})`,
		`(?i)(auth[_-]?token|authtoken)[\s:=]+['"]*([a-zA-Z0-9_\-\.]{20,})`,

		// Passwords
		`(?i)(password|passwd|pwd)[\s:=]+['"]*([^\s'"]{8,})`,

		// AWS credentials
		`(?i)(aws[_-]?access[_-]?key[_-]?id)[\s:=]+['"]*([A-Z0-9]{20})`,
		`(?i)(aws[_-]?secret[_-]?access[_-]?key)[\s:=]+['"]*([a-zA-Z0-9/+=]{40})`,

		// Private keys (PEM format)
		`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(?:RSA\s+)?PRIVATE\s+KEY-----`,

		// Age keys
		`AGE-SECRET-KEY-[A-Z0-9]{59}`,

		// Generic secrets in environment variable format
		`(?i)([A-Z_]+(?:PASSWORD|SECRET|TOKEN|KEY|CREDENTIAL)[A-Z_]*)=([^\s]+)`,

		// Bearer tokens
		`(?i)bearer\s+([a-zA-Z0-9_\-\.]{20,})`,

		// Basic auth
		`(?i)basic\s+([a-zA-Z0-9+/=]{20,})`,

		// Connection strings with passwords
		`(?i)(postgres|mysql|mongodb)://[^:]+:([^@]+)@`,
	}

	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			m.sensitivePatterns = append(m.sensitivePatterns, re)
		}
	}
}

// initializeDefaultFields adds common sensitive field names
func (m *DefaultCredentialMasker) initializeDefaultFields() {
	fields := []string{
		"password", "passwd", "pwd",
		"secret", "secret_key", "secretkey",
		"api_key", "apikey", "api-key",
		"access_token", "accesstoken", "access-token",
		"auth_token", "authtoken", "auth-token",
		"private_key", "privatekey", "private-key",
		"credential", "credentials",
		"token",
		"age_key", "agekey", "age-key",
		"sops_key", "sopskey", "sops-key",
		"encryption_key", "encryptionkey", "encryption-key",
		"bearer_token", "bearertoken", "bearer-token",
		"client_secret", "clientsecret", "client-secret",
		"aws_secret_access_key",
		"aws_access_key_id",
		"openstack_password",
		"vsphere_password",
	}

	for _, field := range fields {
		m.sensitiveFields[strings.ToLower(field)] = true
	}
}

// MaskString masks sensitive data in a string
func (m *DefaultCredentialMasker) MaskString(input string) string {
	if input == "" {
		return input
	}

	masked := input

	// Apply all sensitive patterns
	for _, pattern := range m.sensitivePatterns {
		masked = pattern.ReplaceAllStringFunc(masked, func(match string) string {
			// Keep the field name but mask the value
			parts := pattern.FindStringSubmatch(match)
			if len(parts) >= 2 {
				// parts[1] is the field name, parts[2] is the value
				if len(parts) >= 3 {
					return parts[1] + "=" + MaskString
				}
				return MaskString
			}
			return MaskString
		})
	}

	return masked
}

// MaskMap masks sensitive data in a map
func (m *DefaultCredentialMasker) MaskMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	masked := make(map[string]interface{})

	for key, value := range data {
		if m.IsSensitiveField(key) {
			// Mask the entire value for sensitive fields
			masked[key] = MaskString
		} else {
			// Recursively mask nested structures
			switch v := value.(type) {
			case map[string]interface{}:
				masked[key] = m.MaskMap(v)
			case []interface{}:
				masked[key] = m.maskSlice(v)
			case string:
				masked[key] = m.MaskString(v)
			default:
				masked[key] = value
			}
		}
	}

	return masked
}

// maskSlice masks sensitive data in a slice
func (m *DefaultCredentialMasker) maskSlice(data []interface{}) []interface{} {
	masked := make([]interface{}, len(data))

	for i, item := range data {
		switch v := item.(type) {
		case map[string]interface{}:
			masked[i] = m.MaskMap(v)
		case []interface{}:
			masked[i] = m.maskSlice(v)
		case string:
			masked[i] = m.MaskString(v)
		default:
			masked[i] = item
		}
	}

	return masked
}

// MaskError masks sensitive data in error messages
func (m *DefaultCredentialMasker) MaskError(err error) error {
	if err == nil {
		return nil
	}

	maskedMessage := m.MaskString(err.Error())
	return fmt.Errorf("%s", maskedMessage)
}

// AddSensitivePattern adds a custom sensitive pattern
func (m *DefaultCredentialMasker) AddSensitivePattern(pattern string) {
	if re, err := regexp.Compile(pattern); err == nil {
		m.sensitivePatterns = append(m.sensitivePatterns, re)
	}
}

// AddSensitiveField adds a custom sensitive field name
func (m *DefaultCredentialMasker) AddSensitiveField(fieldName string) {
	m.sensitiveFields[strings.ToLower(fieldName)] = true
}

// IsSensitiveField checks if a field name is sensitive
func (m *DefaultCredentialMasker) IsSensitiveField(fieldName string) bool {
	normalized := strings.ToLower(fieldName)

	// Direct match
	if m.sensitiveFields[normalized] {
		return true
	}

	// Check if field name contains sensitive keywords
	for sensitiveField := range m.sensitiveFields {
		if strings.Contains(normalized, sensitiveField) {
			return true
		}
	}

	return false
}

// MaskPartial masks a string but shows partial information (first and last few chars)
func MaskPartial(input string, showChars int) string {
	if input == "" {
		return input
	}

	if len(input) <= showChars*2 {
		return MaskString
	}

	return input[:showChars] + PartialMaskString + input[len(input)-showChars:]
}

// MaskEmail masks an email address but shows domain
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return MaskString
	}

	username := parts[0]
	domain := parts[1]

	if len(username) <= 2 {
		return MaskString + "@" + domain
	}

	return username[:1] + PartialMaskString + "@" + domain
}

// MaskURL masks sensitive parts of a URL (password in connection strings)
func MaskURL(url string) string {
	// Mask password in URLs like: protocol://user:password@host:port/path
	re := regexp.MustCompile(`(://[^:]+:)([^@]+)(@)`)
	return re.ReplaceAllString(url, "${1}"+MaskString+"${3}")
}
