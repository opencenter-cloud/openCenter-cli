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
	"math/rand"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 3: Credential Masking Across All Output
// For any output stream (logs, stdout, stderr, audit logs, error messages), if the output contains
// credential patterns (API keys, passwords, tokens, private keys, Age keys), then the system SHALL
// mask them before display.
// **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 12.8**
func TestProperty_CredentialMaskingAcrossAllOutput(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	masker := NewDefaultCredentialMasker()

	// Property 3.1: AWS Access Keys are masked in all output
	properties.Property("AWS access keys are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic AWS access key
			awsKey := genAWSAccessKey()

			// Create message with credential embedded
			message := prefix + awsKey + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original key should not appear in masked output
			return !strings.Contains(masked, awsKey)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.2: AWS Secret Keys are masked in all output
	properties.Property("AWS secret keys are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic AWS secret key
			secretKey := genAWSSecretKey()

			// Create message with credential embedded
			message := prefix + "aws_secret_access_key=" + secretKey + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original secret should not appear in masked output
			return !strings.Contains(masked, secretKey)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.3: Age Secret Keys are masked in all output
	properties.Property("Age secret keys are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic Age secret key
			ageKey := genAgeSecretKey()

			// Create message with credential embedded
			message := prefix + ageKey + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original key should not appear in masked output
			return !strings.Contains(masked, ageKey)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.4: Passwords are masked in all output
	properties.Property("passwords are masked in all output", prop.ForAll(
		func(prefix string, password string, suffix string) bool {
			// Skip empty passwords
			if len(password) < 3 {
				return true
			}

			// Create message with password embedded
			message := prefix + "password=" + password + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original password should not appear in masked output
			return !strings.Contains(masked, password)
		},
		gen.AnyString(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 3 }),
		gen.AnyString(),
	))

	// Property 3.5: Tokens are masked in all output
	properties.Property("tokens are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic token
			token := genToken()

			// Create message with token embedded
			message := prefix + "token=" + token + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original token should not appear in masked output
			return !strings.Contains(masked, token)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.6: Bearer tokens are masked in all output
	properties.Property("bearer tokens are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic bearer token (JWT-like)
			token := genJWTToken()

			// Create message with bearer token embedded
			message := prefix + "Authorization: bearer " + token + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original token should not appear in masked output
			return !strings.Contains(masked, token)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.7: Private keys are masked in all output
	properties.Property("private keys are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic private key block
			privateKey := genPrivateKey()

			// Create message with private key embedded
			message := prefix + privateKey + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The key content should not appear in masked output
			// Extract the key content (between BEGIN and END)
			keyContent := extractKeyContent(privateKey)
			if keyContent == "" {
				return true
			}

			return !strings.Contains(masked, keyContent)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.8: OpenStack application credentials are masked in all output
	properties.Property("OpenStack application credentials are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic OpenStack app credential secret
			secret := genOpenStackAppCredSecret()

			// Create message with credential embedded
			message := prefix + "application_credential_secret=" + secret + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original secret should not appear in masked output
			return !strings.Contains(masked, secret)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.9: Generic API keys are masked in all output
	properties.Property("generic API keys are masked in all output", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a realistic API key (32+ characters)
			apiKey := genAPIKey()

			// Create message with API key embedded
			message := prefix + "api_key=" + apiKey + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original API key should not appear in masked output
			return !strings.Contains(masked, apiKey)
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.10: Multiple credentials in same message are all masked
	properties.Property("multiple credentials in same message are all masked", prop.ForAll(
		func(separator string) bool {
			// Generate multiple different credential types
			awsKey := genAWSAccessKey()
			ageKey := genAgeSecretKey()
			password := genPassword()
			token := genToken()

			// Create message with multiple credentials
			message := fmt.Sprintf("Config: AWS_KEY=%s%sAGE_KEY=%s%spassword=%s%stoken=%s",
				awsKey, separator, ageKey, separator, password, separator, token)

			// Mask the message
			masked := masker.MaskString(message)

			// None of the original credentials should appear in masked output
			return !strings.Contains(masked, awsKey) &&
				!strings.Contains(masked, ageKey) &&
				!strings.Contains(masked, password) &&
				!strings.Contains(masked, token)
		},
		gen.OneConstOf(" ", "\n", ", ", "; "),
	))

	// Property 3.11: Masked output contains masking indicators
	properties.Property("masked output contains masking indicators", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a credential
			awsKey := genAWSAccessKey()

			// Create message with credential
			message := prefix + awsKey + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// If the message contained a credential, masked output should contain masking indicator
			if message != masked {
				// Should contain either "****" or "***MASKED***"
				return strings.Contains(masked, "****") || strings.Contains(masked, "***MASKED***")
			}
			return true
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.12: MaskBytes produces same result as MaskString
	properties.Property("MaskBytes produces same result as MaskString", prop.ForAll(
		func(prefix string, suffix string) bool {
			// Generate a credential
			awsKey := genAWSAccessKey()
			message := prefix + awsKey + suffix

			// Mask using both methods
			maskedString := masker.MaskString(message)
			maskedBytes := string(masker.MaskBytes([]byte(message)))

			// Results should be identical
			return maskedString == maskedBytes
		},
		gen.AnyString(),
		gen.AnyString(),
	))

	// Property 3.13: Empty input returns empty output
	properties.Property("empty input returns empty output", prop.ForAll(
		func(_ bool) bool {
			masked := masker.MaskString("")
			return masked == ""
		},
		gen.Const(true),
	))

	// Property 3.14: Input without credentials is unchanged
	properties.Property("input without credentials is unchanged", prop.ForAll(
		func(message string) bool {
			// Generate a message that doesn't contain credential patterns
			safeMessage := genSafeMessage(message)

			// Mask the message
			masked := masker.MaskString(safeMessage)

			// Should be unchanged
			return masked == safeMessage
		},
		gen.AlphaString(),
	))

	// Property 3.15: Credentials at start of string are masked
	properties.Property("credentials at start of string are masked", prop.ForAll(
		func(suffix string) bool {
			// Generate a credential at the start
			awsKey := genAWSAccessKey()
			message := awsKey + suffix

			// Mask the message
			masked := masker.MaskString(message)

			// The original key should not appear
			return !strings.Contains(masked, awsKey)
		},
		gen.AnyString(),
	))

	// Property 3.16: Credentials at end of string are masked
	properties.Property("credentials at end of string are masked", prop.ForAll(
		func(prefix string) bool {
			// Generate a credential at the end
			awsKey := genAWSAccessKey()
			message := prefix + awsKey

			// Mask the message
			masked := masker.MaskString(message)

			// The original key should not appear
			return !strings.Contains(masked, awsKey)
		},
		gen.AnyString(),
	))

	// Property 3.17: Credentials in log format are masked
	properties.Property("credentials in log format are masked", prop.ForAll(
		func(logLevel string, logMessage string) bool {
			// Generate a credential
			password := genPassword()

			// Create log-formatted message
			message := fmt.Sprintf("[%s] %s password=%s", logLevel, logMessage, password)

			// Mask the message
			masked := masker.MaskString(message)

			// The original password should not appear
			return !strings.Contains(masked, password)
		},
		gen.OneConstOf("INFO", "WARN", "ERROR", "DEBUG"),
		gen.AlphaString(),
	))

	// Property 3.18: Credentials in JSON format are masked
	properties.Property("credentials in JSON format are masked", prop.ForAll(
		func(key string) bool {
			// Skip empty keys
			if key == "" {
				return true
			}

			// Generate a credential
			apiKey := genAPIKey()

			// Create JSON-formatted message
			message := fmt.Sprintf(`{"config": {"api_key": "%s"}}`, apiKey)

			// Mask the message
			masked := masker.MaskString(message)

			// The original API key should not appear
			return !strings.Contains(masked, apiKey)
		},
		gen.AlphaString(),
	))

	// Property 3.19: Credentials in YAML format are masked
	properties.Property("credentials in YAML format are masked", prop.ForAll(
		func(indent string) bool {
			// Generate a credential
			secret := genOpenStackAppCredSecret()

			// Create YAML-formatted message
			message := fmt.Sprintf("%sapplication_credential_secret: %s", indent, secret)

			// Mask the message
			masked := masker.MaskString(message)

			// The original secret should not appear
			return !strings.Contains(masked, secret)
		},
		gen.OneConstOf("", "  ", "    "),
	))

	// Property 3.20: Credentials in error messages are masked
	properties.Property("credentials in error messages are masked", prop.ForAll(
		func(errorMsg string) bool {
			// Generate a credential
			token := genToken()

			// Create error message with credential
			message := fmt.Sprintf("Error: authentication failed with token=%s: %s", token, errorMsg)

			// Mask the message
			masked := masker.MaskString(message)

			// The original token should not appear
			return !strings.Contains(masked, token)
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Custom generators for realistic credentials

// genAWSAccessKey generates a realistic AWS access key (AKIA + 16 alphanumeric chars)
func genAWSAccessKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key := make([]byte, 16)
	for i := range key {
		key[i] = charset[rand.Intn(len(charset))]
	}
	return "AKIA" + string(key)
}

// genAWSSecretKey generates a realistic AWS secret key (40 base64-like chars)
func genAWSSecretKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789/+="
	key := make([]byte, 40)
	for i := range key {
		key[i] = charset[rand.Intn(len(charset))]
	}
	return string(key)
}

// genAgeSecretKey generates a realistic Age secret key
func genAgeSecretKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	key := make([]byte, 59)
	for i := range key {
		key[i] = charset[rand.Intn(len(charset))]
	}
	return "AGE-SECRET-KEY-" + string(key)
}

// genPassword generates a realistic password (8-20 chars)
func genPassword() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	length := 8 + rand.Intn(13) // 8-20 chars
	password := make([]byte, length)
	for i := range password {
		password[i] = charset[rand.Intn(len(charset))]
	}
	return string(password)
}

// genToken generates a realistic token (32-64 alphanumeric chars)
func genToken() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-."
	length := 32 + rand.Intn(33) // 32-64 chars
	token := make([]byte, length)
	for i := range token {
		token[i] = charset[rand.Intn(len(charset))]
	}
	return string(token)
}

// genJWTToken generates a realistic JWT token (3 base64 parts separated by dots)
func genJWTToken() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

	genPart := func(length int) string {
		part := make([]byte, length)
		for i := range part {
			part[i] = charset[rand.Intn(len(charset))]
		}
		return string(part)
	}

	return genPart(36) + "." + genPart(200) + "." + genPart(43)
}

// genPrivateKey generates a realistic private key block
func genPrivateKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="

	// Generate key content (multiple lines of base64)
	lines := make([]string, 5)
	for i := range lines {
		line := make([]byte, 64)
		for j := range line {
			line[j] = charset[rand.Intn(len(charset))]
		}
		lines[i] = string(line)
	}

	return fmt.Sprintf("-----BEGIN RSA PRIVATE KEY-----\n%s\n-----END RSA PRIVATE KEY-----",
		strings.Join(lines, "\n"))
}

// genOpenStackAppCredSecret generates a realistic OpenStack application credential secret
func genOpenStackAppCredSecret() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	length := 20 + rand.Intn(21) // 20-40 chars
	secret := make([]byte, length)
	for i := range secret {
		secret[i] = charset[rand.Intn(len(charset))]
	}
	return string(secret)
}

// genAPIKey generates a realistic API key (32-64 alphanumeric chars)
func genAPIKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	length := 32 + rand.Intn(33) // 32-64 chars
	key := make([]byte, length)
	for i := range key {
		key[i] = charset[rand.Intn(len(charset))]
	}
	return string(key)
}

// genSafeMessage generates a message that doesn't contain credential patterns
func genSafeMessage(input string) string {
	// Remove any potential credential patterns
	safe := input

	// Remove anything that looks like credential keywords
	credentialKeywords := []string{
		"password", "passwd", "pwd", "token", "bearer", "api_key", "api-key",
		"secret", "credential", "AKIA", "AGE-SECRET-KEY",
	}

	for _, keyword := range credentialKeywords {
		safe = strings.ReplaceAll(safe, keyword, "config")
		safe = strings.ReplaceAll(safe, strings.ToUpper(keyword), "CONFIG")
	}

	// Remove special characters that might be part of credential patterns
	safe = strings.ReplaceAll(safe, "=", " ")
	safe = strings.ReplaceAll(safe, ":", " ")

	return safe
}

// extractKeyContent extracts the content between BEGIN and END markers in a private key
func extractKeyContent(privateKey string) string {
	lines := strings.Split(privateKey, "\n")
	var content []string

	inKey := false
	for _, line := range lines {
		if strings.Contains(line, "BEGIN") {
			inKey = true
			continue
		}
		if strings.Contains(line, "END") {
			break
		}
		if inKey && line != "" {
			content = append(content, line)
		}
	}

	return strings.Join(content, "\n")
}
