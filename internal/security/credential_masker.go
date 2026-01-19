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
	"regexp"
	"sync"
)

// CredentialMasker detects and masks credentials in all output streams
type CredentialMasker interface {
	MaskString(input string) string
	MaskBytes(input []byte) []byte
	RegisterPattern(name string, pattern *regexp.Regexp)
	GetMaskedCount() int
}

// DefaultCredentialMasker implements CredentialMasker interface
type DefaultCredentialMasker struct {
	// patterns maps pattern names to compiled regex patterns
	patterns map[string]*regexp.Regexp
	// maskedCount tracks the number of credentials masked
	maskedCount int
	// mu protects concurrent access to maskedCount
	mu sync.RWMutex
}

// NewDefaultCredentialMasker creates a new credential masker with default patterns
func NewDefaultCredentialMasker() *DefaultCredentialMasker {
	masker := &DefaultCredentialMasker{
		patterns:    make(map[string]*regexp.Regexp),
		maskedCount: 0,
	}

	// Register default patterns
	masker.registerDefaultPatterns()

	return masker
}

// registerDefaultPatterns registers the default credential patterns
// Requirements: 3.2, 3.8
func (m *DefaultCredentialMasker) registerDefaultPatterns() {
	// AWS Access Key ID pattern: AKIA followed by 16 alphanumeric characters
	m.patterns["aws_access_key"] = regexp.MustCompile(`AKIA[A-Z0-9]{16}`)

	// AWS Secret Access Key pattern: 40 character base64-like string
	m.patterns["aws_secret_key"] = regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key['":\s=]+([A-Za-z0-9/+=]{40})`)

	// Age secret key pattern: AGE-SECRET-KEY- followed by 59 base64-like characters
	m.patterns["age_secret_key"] = regexp.MustCompile(`AGE-SECRET-KEY-[A-Z0-9a-z]{59}`)

	// Generic password patterns (after password=, PASSWORD=, pwd=)
	m.patterns["password_equals"] = regexp.MustCompile(`(?i)(password|passwd|pwd)[=:]\s*["']?([^"'\s]{3,})["']?`)

	// Generic token patterns (after token=, TOKEN=, bearer)
	m.patterns["token_equals"] = regexp.MustCompile(`(?i)(token|bearer)[=:\s]+["']?([A-Za-z0-9_\-\.]{10,})["']?`)

	// Private key blocks (PEM format)
	m.patterns["private_key_block"] = regexp.MustCompile(`-----BEGIN[A-Z\s]+PRIVATE KEY-----[\s\S]*?-----END[A-Z\s]+PRIVATE KEY-----`)

	// Generic API key patterns (32+ character alphanumeric strings after api_key/api-key)
	m.patterns["generic_api_key"] = regexp.MustCompile(`(?i)api[_-]?key['":\s=]+["']?([A-Za-z0-9]{32,})["']?`)

	// OpenStack application credential secret
	m.patterns["openstack_app_cred"] = regexp.MustCompile(`(?i)application[_-]?credential[_-]?secret['":\s=]+([A-Za-z0-9]{20,})`)

	// Generic secret patterns
	m.patterns["generic_secret"] = regexp.MustCompile(`(?i)secret['":\s=]+["']?([A-Za-z0-9_\-\.]{16,})["']?`)
}

// MaskString masks credentials in a string
// Requirements: 3.1, 3.2, 3.3, 3.4, 3.5
//
// This function scans the input string for credential patterns and replaces them
// with masked versions. For debugging purposes, it preserves the first 4 and last 4
// characters of certain credential types (like AWS keys).
func (m *DefaultCredentialMasker) MaskString(input string) string {
	if input == "" {
		return input
	}

	result := input
	masked := false

	// AWS Access Keys - preserve first 4 and last 4 characters
	if matches := m.patterns["aws_access_key"].FindAllString(result, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) >= 8 {
				replacement := match[:4] + "****" + match[len(match)-4:]
				result = regexp.MustCompile(regexp.QuoteMeta(match)).ReplaceAllString(result, replacement)
			} else {
				result = regexp.MustCompile(regexp.QuoteMeta(match)).ReplaceAllString(result, "***MASKED***")
			}
			masked = true
		}
	}

	// AWS Secret Keys - mask completely
	if matches := m.patterns["aws_secret_key"].FindAllStringSubmatch(result, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				result = regexp.MustCompile(regexp.QuoteMeta(match[1])).ReplaceAllString(result, "***MASKED***")
				masked = true
			}
		}
	}

	// Age Secret Keys - preserve prefix for identification
	if matches := m.patterns["age_secret_key"].FindAllString(result, -1); len(matches) > 0 {
		for _, match := range matches {
			replacement := "AGE-SECRET-KEY-****"
			result = regexp.MustCompile(regexp.QuoteMeta(match)).ReplaceAllString(result, replacement)
			masked = true
		}
	}

	// Password patterns - mask the password value
	if matches := m.patterns["password_equals"].FindAllStringSubmatch(result, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 2 {
				result = regexp.MustCompile(regexp.QuoteMeta(match[2])).ReplaceAllString(result, "***MASKED***")
				masked = true
			}
		}
	}

	// Token patterns - mask the token value
	if matches := m.patterns["token_equals"].FindAllStringSubmatch(result, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 2 {
				result = regexp.MustCompile(regexp.QuoteMeta(match[2])).ReplaceAllString(result, "***MASKED***")
				masked = true
			}
		}
	}

	// Private key blocks - mask entire block
	if matches := m.patterns["private_key_block"].FindAllString(result, -1); len(matches) > 0 {
		for _, match := range matches {
			// Preserve the header and footer for context
			replacement := "-----BEGIN PRIVATE KEY-----\n***MASKED***\n-----END PRIVATE KEY-----"
			result = regexp.MustCompile(regexp.QuoteMeta(match)).ReplaceAllString(result, replacement)
			masked = true
		}
	}

	// Generic API key patterns - mask the key value
	if matches := m.patterns["generic_api_key"].FindAllStringSubmatch(result, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				result = regexp.MustCompile(regexp.QuoteMeta(match[1])).ReplaceAllString(result, "***MASKED***")
				masked = true
			}
		}
	}

	// OpenStack application credential secret - mask the secret value
	if matches := m.patterns["openstack_app_cred"].FindAllStringSubmatch(result, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				result = regexp.MustCompile(regexp.QuoteMeta(match[1])).ReplaceAllString(result, "***MASKED***")
				masked = true
			}
		}
	}

	// Generic secret patterns - mask the secret value
	if matches := m.patterns["generic_secret"].FindAllStringSubmatch(result, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				result = regexp.MustCompile(regexp.QuoteMeta(match[1])).ReplaceAllString(result, "***MASKED***")
				masked = true
			}
		}
	}

	// Update masked count if any credentials were masked
	if masked {
		m.mu.Lock()
		m.maskedCount++
		m.mu.Unlock()
	}

	return result
}

// MaskBytes masks credentials in a byte slice
// Requirements: 3.1, 3.2, 3.3, 3.4, 3.5
//
// This function converts the byte slice to a string, masks credentials,
// and converts back to bytes.
func (m *DefaultCredentialMasker) MaskBytes(input []byte) []byte {
	if len(input) == 0 {
		return input
	}

	masked := m.MaskString(string(input))
	return []byte(masked)
}

// RegisterPattern registers a custom credential pattern
// Requirements: 3.8
//
// This function allows adding custom patterns for credential detection.
// The pattern should be a compiled regular expression that matches the
// credential format.
func (m *DefaultCredentialMasker) RegisterPattern(name string, pattern *regexp.Regexp) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.patterns[name] = pattern
}

// GetMaskedCount returns the number of credentials that have been masked
// Requirements: 3.7
//
// This function provides visibility into how many credentials have been
// masked, which can be useful for security auditing and monitoring.
func (m *DefaultCredentialMasker) GetMaskedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.maskedCount
}
