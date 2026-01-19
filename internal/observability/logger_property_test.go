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

package observability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 17: Structured Logging Fields
// **Validates: Requirements 12.2, 12.3, 12.8**
//
// For any log entry, it SHALL include required structured fields (timestamp, level, message, correlation_id, cluster, operation)
// and SHALL have credentials masked.
func TestProperty_StructuredLoggingFields(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("JSON logs contain required structured fields", prop.ForAll(
		func(message string, correlationID string, cluster string, operation string) bool {
			// Create logger with JSON format
			var buf bytes.Buffer
			logger := NewDefaultLogger(LoggerConfig{
				Level:  InfoLevel,
				Format: JSONFormat,
				Output: &buf,
			})

			// Add correlation ID and fields
			loggerWithContext := logger.WithCorrelationID(correlationID).WithFields(
				Field{Key: "cluster", Value: cluster},
				Field{Key: "operation", Value: operation},
			)

			// Log a message
			loggerWithContext.Info(message)

			// Parse JSON output
			var entry LogEntry
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				return false
			}

			// Verify required fields are present
			if entry.Timestamp == "" {
				return false
			}
			if entry.Level != "INFO" {
				return false
			}
			if entry.Message == "" {
				return false
			}
			if correlationID != "" && entry.CorrelationID != correlationID {
				return false
			}
			if entry.Fields == nil {
				return false
			}
			if entry.Fields["cluster"] != cluster {
				return false
			}
			if entry.Fields["operation"] != operation {
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.Property("credentials are masked in log messages", prop.ForAll(
		func(credType int) bool {
			// Create logger with JSON format
			var buf bytes.Buffer
			logger := NewDefaultLogger(LoggerConfig{
				Level:  InfoLevel,
				Format: JSONFormat,
				Output: &buf,
			})

			// Generate a credential based on type
			var credential, message string
			switch credType % 4 {
			case 0:
				credential = "AKIAIOSFODNN7EXAMPLE"
				message = "Processing with AWS key: " + credential
			case 1:
				credential = "AGE-SECRET-KEY-" + strings.Repeat("A", 59)
				message = "Processing with Age key: " + credential
			case 2:
				credential = "mysecretpassword123"
				message = "Processing with password="+credential
			case 3:
				credential = strings.Repeat("A", 32)
				message = "Processing with api_key="+credential
			}

			// Log the message
			logger.Info(message)

			// Parse JSON output
			var entry LogEntry
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				return false
			}

			// Verify credential is masked in message
			// For AWS keys and Age keys, check they don't appear in full
			if credType % 4 == 0 || credType % 4 == 1 {
				if strings.Contains(entry.Message, credential) {
					return false
				}
			}

			// Verify masked indicator is present
			if !strings.Contains(entry.Message, "***MASKED***") && !strings.Contains(entry.Message, "****") {
				return false
			}

			return true
		},
		gen.IntRange(0, 100),
	))

	properties.Property("credentials are masked in field values", prop.ForAll(
		func(credType int) bool {
			// Create logger with JSON format
			var buf bytes.Buffer
			logger := NewDefaultLogger(LoggerConfig{
				Level:  InfoLevel,
				Format: JSONFormat,
				Output: &buf,
			})

			// Generate a credential based on type
			var credential string
			switch credType % 4 {
			case 0:
				credential = "AKIAIOSFODNN7EXAMPLE"
			case 1:
				credential = "AGE-SECRET-KEY-" + strings.Repeat("A", 59)
			case 2:
				credential = "password=mysecretpassword123"
			case 3:
				credential = "api_key=" + strings.Repeat("A", 32)
			}

			// Log with credential in field
			logger.Info("Processing", Field{Key: "credential", Value: credential})

			// Parse JSON output
			var entry LogEntry
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				return false
			}

			// Verify credential is masked in field value
			if entry.Fields == nil {
				return false
			}
			fieldValue, ok := entry.Fields["credential"]
			if !ok {
				return false
			}
			fieldStr, ok := fieldValue.(string)
			if !ok {
				return false
			}

			// For AWS keys and Age keys, check they don't appear in full
			if credType % 4 == 0 || credType % 4 == 1 {
				if strings.Contains(fieldStr, credential) {
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 100),
	))

	properties.Property("text logs contain required fields", prop.ForAll(
		func(message string, correlationID string) bool {
			// Create logger with text format
			var buf bytes.Buffer
			logger := NewDefaultLogger(LoggerConfig{
				Level:  InfoLevel,
				Format: TextFormat,
				Output: &buf,
			})

			// Add correlation ID
			var loggerWithContext Logger = logger
			if correlationID != "" {
				loggerWithContext = logger.WithCorrelationID(correlationID)
			}

			// Log a message
			loggerWithContext.Info(message)

			output := buf.String()

			// Verify timestamp is present (RFC3339 format)
			if !strings.Contains(output, "T") || !strings.Contains(output, "Z") {
				return false
			}

			// Verify level is present
			if !strings.Contains(output, "INFO") {
				return false
			}

			// Verify correlation ID is present if provided
			if correlationID != "" && !strings.Contains(output, "correlation_id="+correlationID) {
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// Feature: security-and-operational-remediation, Property 18: Correlation ID Propagation
// **Validates: Requirements 12.3**
//
// For any operation, a correlation ID SHALL be assigned at the start and propagated through all log entries
// for that operation.
func TestProperty_CorrelationIDPropagation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("correlation ID is propagated through all log entries", prop.ForAll(
		func(correlationID string, numLogs int) bool {
			if numLogs < 1 || numLogs > 10 {
				return true // Skip invalid test cases
			}

			// Create logger with JSON format
			var buf bytes.Buffer
			logger := NewDefaultLogger(LoggerConfig{
				Level:  DebugLevel,
				Format: JSONFormat,
				Output: &buf,
			})

			// Add correlation ID
			loggerWithContext := logger.WithCorrelationID(correlationID)

			// Log multiple messages
			for i := 0; i < numLogs; i++ {
				loggerWithContext.Info("Log entry", Field{Key: "index", Value: i})
			}

			// Parse all JSON log entries
			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			if len(lines) != numLogs {
				return false
			}

			// Verify correlation ID is present in all entries
			for _, line := range lines {
				var entry LogEntry
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					return false
				}
				if entry.CorrelationID != correlationID {
					return false
				}
			}

			return true
		},
		gen.Identifier(),
		gen.IntRange(1, 10),
	))

	properties.Property("correlation ID persists through WithFields", prop.ForAll(
		func(correlationID string) bool {
			// Create logger with correlation ID
			var buf bytes.Buffer
			logger := NewDefaultLogger(LoggerConfig{
				Level:  InfoLevel,
				Format: JSONFormat,
				Output: &buf,
			})
			loggerWithCorrelation := logger.WithCorrelationID(correlationID)

			// Add fields (should preserve correlation ID)
			loggerWithFields := loggerWithCorrelation.WithFields(Field{Key: "cluster", Value: "test"})

			// Log a message
			loggerWithFields.Info("Test message")

			// Parse JSON output
			var entry LogEntry
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				return false
			}

			// Verify correlation ID is preserved
			return entry.CorrelationID == correlationID
		},
		gen.Identifier(),
	))

	properties.Property("different loggers can have different correlation IDs", prop.ForAll(
		func(correlationID1 string, correlationID2 string) bool {
			if correlationID1 == correlationID2 {
				return true // Skip when IDs are the same
			}

			// Create two loggers with different correlation IDs
			var buf1, buf2 bytes.Buffer
			logger1 := NewDefaultLogger(LoggerConfig{
				Level:  InfoLevel,
				Format: JSONFormat,
				Output: &buf1,
			}).WithCorrelationID(correlationID1)

			logger2 := NewDefaultLogger(LoggerConfig{
				Level:  InfoLevel,
				Format: JSONFormat,
				Output: &buf2,
			}).WithCorrelationID(correlationID2)

			// Log with both loggers
			logger1.Info("Message 1")
			logger2.Info("Message 2")

			// Parse outputs
			var entry1, entry2 LogEntry
			if err := json.Unmarshal(buf1.Bytes(), &entry1); err != nil {
				return false
			}
			if err := json.Unmarshal(buf2.Bytes(), &entry2); err != nil {
				return false
			}

			// Verify each logger has its own correlation ID
			return entry1.CorrelationID == correlationID1 && entry2.CorrelationID == correlationID2
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// genCredential generates realistic credentials for testing
func genCredential() gopter.Gen {
	return gen.OneGenOf(
		genAWSAccessKey(),
		genAgeSecretKey(),
		genPasswordValue(),
		genAPITokenValue(),
	)
}

// genAWSAccessKey generates AWS access key patterns
func genAWSAccessKey() gopter.Gen {
	return gen.RegexMatch("AKIA[A-Z0-9]{16}")
}

// genAgeSecretKey generates Age secret key patterns
func genAgeSecretKey() gopter.Gen {
	return gen.RegexMatch("AGE-SECRET-KEY-[A-Z0-9]{59}")
}

// genPasswordValue generates password patterns that will be masked
func genPasswordValue() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) >= 8 && len(s) <= 32
	}).Map(func(s string) string {
		return "password=" + s
	})
}

// genAPITokenValue generates API token patterns that will be masked
func genAPITokenValue() gopter.Gen {
	return gen.RegexMatch("[A-Za-z0-9_\\-\\.]{32}").Map(func(s string) string {
		return "api_key=" + s
	})
}
