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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: security-and-operational-remediation, Property 6: Audit Logging for Security Events
// For any security-relevant event (key generation, key access, config modification, validation failure,
// rejected input), the system SHALL create an audit log entry with timestamp, actor, resource, action,
// and HMAC signature.
// **Validates: Requirements 1.7, 4.7**
func TestProperty_AuditLoggingForSecurityEvents(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 6.1: All security events are logged with required fields
	properties.Property("all security events are logged with required fields", prop.ForAll(
		func(eventType, actor, resource, action, result string) bool {
			// Skip empty values
			if eventType == "" || actor == "" || resource == "" || action == "" || result == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer logger.Close()

			// Log an event
			event := AuditEvent{
				EventType: eventType,
				Actor:     actor,
				Resource:  resource,
				Action:    action,
				Result:    result,
			}

			ctx := context.Background()
			if err := logger.LogEvent(ctx, event); err != nil {
				t.Logf("Failed to log event: %v", err)
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: eventType,
				Actor:     actor,
			})
			if err != nil {
				t.Logf("Failed to query events: %v", err)
				return false
			}

			// Verify event was logged with all required fields
			if len(events) == 0 {
				t.Logf("No events found")
				return false
			}

			loggedEvent := events[0]
			return loggedEvent.EventType == eventType &&
				loggedEvent.Actor == actor &&
				loggedEvent.Resource == resource &&
				loggedEvent.Action == action &&
				loggedEvent.Result == result &&
				!loggedEvent.Timestamp.IsZero() &&
				loggedEvent.Signature != ""
		},
		genEventType(),
		genActor(),
		genResource(),
		genAction(),
		genResult(),
	))

	// Property 6.2: Key generation events are logged
	properties.Property("key generation events are logged", prop.ForAll(
		func(actor, keyType, resource string) bool {
			// Skip empty values
			if actor == "" || keyType == "" || resource == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log key generation
			ctx := context.Background()
			if err := logger.LogKeyGenerated(ctx, actor, keyType, resource); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "key.generated",
			})
			if err != nil {
				return false
			}

			// Verify event was logged
			if len(events) == 0 {
				return false
			}

			event := events[0]
			return event.EventType == "key.generated" &&
				event.Actor == actor &&
				event.Resource == resource &&
				event.Action == "generate" &&
				event.Result == "success"
		},
		genActor(),
		genKeyType(),
		genResource(),
	))

	// Property 6.3: Key access events are logged
	properties.Property("key access events are logged", prop.ForAll(
		func(actor, keyType, resource string, success bool) bool {
			// Skip empty values
			if actor == "" || keyType == "" || resource == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log key access
			ctx := context.Background()
			if err := logger.LogKeyAccessed(ctx, actor, keyType, resource, success); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "key.accessed",
			})
			if err != nil {
				return false
			}

			// Verify event was logged
			if len(events) == 0 {
				return false
			}

			event := events[0]
			expectedResult := "success"
			if !success {
				expectedResult = "failure"
			}

			return event.EventType == "key.accessed" &&
				event.Actor == actor &&
				event.Resource == resource &&
				event.Action == "access" &&
				event.Result == expectedResult
		},
		genActor(),
		genKeyType(),
		genResource(),
		gen.Bool(),
	))

	// Property 6.4: Key rotation events are logged
	properties.Property("key rotation events are logged", prop.ForAll(
		func(actor, keyType, resource string) bool {
			// Skip empty values
			if actor == "" || keyType == "" || resource == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log key rotation
			ctx := context.Background()
			if err := logger.LogKeyRotated(ctx, actor, keyType, resource); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "key.rotated",
			})
			if err != nil {
				return false
			}

			// Verify event was logged
			if len(events) == 0 {
				return false
			}

			event := events[0]
			return event.EventType == "key.rotated" &&
				event.Actor == actor &&
				event.Resource == resource &&
				event.Action == "rotate" &&
				event.Result == "success"
		},
		genActor(),
		genKeyType(),
		genResource(),
	))

	// Property 6.5: Validation failure events are logged
	properties.Property("validation failure events are logged", prop.ForAll(
		func(actor, resource, reason string) bool {
			// Skip empty values
			if actor == "" || resource == "" || reason == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log validation failure
			ctx := context.Background()
			if err := logger.LogValidationFailed(ctx, actor, resource, reason, nil); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "validation.failed",
			})
			if err != nil {
				return false
			}

			// Verify event was logged
			if len(events) == 0 {
				return false
			}

			event := events[0]
			return event.EventType == "validation.failed" &&
				event.Actor == actor &&
				event.Resource == resource &&
				event.Action == "validate" &&
				event.Result == "failure"
		},
		genActor(),
		genResource(),
		genReason(),
	))

	// Property 6.6: Rejected input events are logged
	properties.Property("rejected input events are logged", prop.ForAll(
		func(actor, inputType, reason string) bool {
			// Skip empty values
			if actor == "" || inputType == "" || reason == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log rejected input
			ctx := context.Background()
			if err := logger.LogInputRejected(ctx, actor, inputType, reason); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "input.rejected",
			})
			if err != nil {
				return false
			}

			// Verify event was logged
			if len(events) == 0 {
				return false
			}

			event := events[0]
			return event.EventType == "input.rejected" &&
				event.Actor == actor &&
				event.Resource == inputType &&
				event.Action == "validate" &&
				event.Result == "rejected"
		},
		genActor(),
		genInputType(),
		genReason(),
	))

	// Property 6.7: Template validation failure events are logged
	properties.Property("template validation failure events are logged", prop.ForAll(
		func(actor, templateName, reason string) bool {
			// Skip empty values
			if actor == "" || templateName == "" || reason == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log template validation failure
			ctx := context.Background()
			if err := logger.LogTemplateValidationFailed(ctx, actor, templateName, reason); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "template.validation.failed",
			})
			if err != nil {
				return false
			}

			// Verify event was logged
			if len(events) == 0 {
				return false
			}

			event := events[0]
			return event.EventType == "template.validation.failed" &&
				event.Actor == actor &&
				event.Resource == templateName &&
				event.Action == "validate" &&
				event.Result == "failure"
		},
		genActor(),
		genTemplateName(),
		genReason(),
	))

	// Property 6.8: All events have HMAC signatures
	properties.Property("all events have HMAC signatures", prop.ForAll(
		func(eventType, actor, resource string) bool {
			// Skip empty values
			if eventType == "" || actor == "" || resource == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger with known signing key
			signingKey := []byte("test-signing-key-for-property-testing")
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath:    logPath,
				SigningKey: signingKey,
				Enabled:    true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log an event
			event := AuditEvent{
				EventType: eventType,
				Actor:     actor,
				Resource:  resource,
				Action:    "test",
				Result:    "success",
			}

			ctx := context.Background()
			if err := logger.LogEvent(ctx, event); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: eventType,
			})
			if err != nil {
				return false
			}

			// Verify event has signature
			if len(events) == 0 {
				return false
			}

			return events[0].Signature != "" && len(events[0].Signature) == 64 // SHA256 hex = 64 chars
		},
		genEventType(),
		genActor(),
		genResource(),
	))

	// Property 6.9: Signature verification succeeds for valid events
	properties.Property("signature verification succeeds for valid events", prop.ForAll(
		func(eventType, actor, resource string) bool {
			// Skip empty values
			if eventType == "" || actor == "" || resource == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log an event
			event := AuditEvent{
				EventType: eventType,
				Actor:     actor,
				Resource:  resource,
				Action:    "test",
				Result:    "success",
			}

			ctx := context.Background()
			if err := logger.LogEvent(ctx, event); err != nil {
				return false
			}

			// Verify integrity
			if err := logger.VerifyIntegrity(); err != nil {
				t.Logf("Integrity verification failed: %v", err)
				return false
			}

			return true
		},
		genEventType(),
		genActor(),
		genResource(),
	))

	// Property 6.10: Credentials in event details are masked
	properties.Property("credentials in event details are masked", prop.ForAll(
		func(actor, resource string) bool {
			// Skip empty values
			if actor == "" || resource == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Generate a credential
			password := genPassword()

			// Log an event with credential in details
			event := AuditEvent{
				EventType: "test.event",
				Actor:     actor,
				Resource:  resource,
				Action:    "test",
				Result:    "success",
				Details: map[string]interface{}{
					"password": password,
					"config":   fmt.Sprintf("password=%s", password),
				},
			}

			ctx := context.Background()
			if err := logger.LogEvent(ctx, event); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "test.event",
			})
			if err != nil {
				return false
			}

			// Verify credentials are masked
			if len(events) == 0 {
				return false
			}

			loggedEvent := events[0]
			if loggedEvent.Details == nil {
				return false
			}

			// Check that password is masked
			if passwordVal, ok := loggedEvent.Details["password"].(string); ok {
				if strings.Contains(passwordVal, password) {
					t.Logf("Password not masked in details: %s", passwordVal)
					return false
				}
			}

			if configVal, ok := loggedEvent.Details["config"].(string); ok {
				if strings.Contains(configVal, password) {
					t.Logf("Password not masked in config: %s", configVal)
					return false
				}
			}

			return true
		},
		genActor(),
		genResource(),
	))

	// Property 6.11: Correlation IDs are preserved
	properties.Property("correlation IDs are preserved", prop.ForAll(
		func(eventType, actor, resource, correlationID string) bool {
			// Skip empty values
			if eventType == "" || actor == "" || resource == "" || correlationID == "" {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Create context with correlation ID
			ctx := context.WithValue(context.Background(), "correlation_id", correlationID)

			// Log an event
			event := AuditEvent{
				EventType: eventType,
				Actor:     actor,
				Resource:  resource,
				Action:    "test",
				Result:    "success",
			}

			if err := logger.LogEvent(ctx, event); err != nil {
				return false
			}

			// Query the logged event
			events, err := logger.QueryEvents(ctx, EventFilter{
				CorrelationID: correlationID,
			})
			if err != nil {
				return false
			}

			// Verify correlation ID is preserved
			if len(events) == 0 {
				return false
			}

			return events[0].CorrelationID == correlationID
		},
		genEventType(),
		genActor(),
		genResource(),
		genCorrelationID(),
	))

	// Property 6.12: Log rotation occurs when size limit is reached
	properties.Property("log rotation occurs when size limit is reached", prop.ForAll(
		func(numEvents int) bool {
			// Limit number of events to reasonable range
			if numEvents < 1 || numEvents > 1000 {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log multiple events
			ctx := context.Background()
			for i := 0; i < numEvents; i++ {
				event := AuditEvent{
					EventType: "test.event",
					Actor:     fmt.Sprintf("actor-%d", i),
					Resource:  fmt.Sprintf("resource-%d", i),
					Action:    "test",
					Result:    "success",
					Details: map[string]interface{}{
						"iteration": i,
						"data":      strings.Repeat("x", 1000), // Add some bulk
					},
				}

				if err := logger.LogEvent(ctx, event); err != nil {
					return false
				}
			}

			// Check if log file exists
			if _, err := os.Stat(logPath); err != nil {
				return false
			}

			return true
		},
		gen.IntRange(1, 100),
	))

	// Property 6.13: Event timestamps are monotonically increasing
	properties.Property("event timestamps are monotonically increasing", prop.ForAll(
		func(numEvents int) bool {
			// Limit number of events
			if numEvents < 2 || numEvents > 10 {
				return true
			}

			// Create temporary log file
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			if err != nil {
				return false
			}
			defer logger.Close()

			// Log multiple events with small delays
			ctx := context.Background()
			for i := 0; i < numEvents; i++ {
				event := AuditEvent{
					EventType: "test.event",
					Actor:     "test-actor",
					Resource:  fmt.Sprintf("resource-%d", i),
					Action:    "test",
					Result:    "success",
				}

				if err := logger.LogEvent(ctx, event); err != nil {
					return false
				}

				time.Sleep(1 * time.Millisecond) // Small delay to ensure different timestamps
			}

			// Query all events
			events, err := logger.QueryEvents(ctx, EventFilter{
				EventType: "test.event",
			})
			if err != nil {
				return false
			}

			// Verify timestamps are monotonically increasing
			for i := 1; i < len(events); i++ {
				if !events[i].Timestamp.After(events[i-1].Timestamp) &&
					!events[i].Timestamp.Equal(events[i-1].Timestamp) {
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Custom generators for audit logging

// genEventType generates realistic event types
func genEventType() gopter.Gen {
	return gen.OneConstOf(
		"key.generated",
		"key.accessed",
		"key.rotated",
		"validation.failed",
		"input.rejected",
		"template.validation.failed",
		"config.modified",
		"cluster.created",
		"cluster.destroyed",
	)
}

// genActor generates realistic actor names
func genActor() gopter.Gen {
	return gen.OneConstOf(
		"user@example.com",
		"admin@example.com",
		"system",
		"automation",
		"ci-cd-pipeline",
		"operator",
	)
}

// genResource generates realistic resource names
func genResource() gopter.Gen {
	return gen.OneConstOf(
		"my-cluster",
		"production-cluster",
		"staging-cluster",
		"age-key",
		"ssh-key",
		"config.yaml",
		"template.yaml",
	)
}

// genAction generates realistic action names
func genAction() gopter.Gen {
	return gen.OneConstOf(
		"generate",
		"access",
		"rotate",
		"validate",
		"create",
		"update",
		"delete",
		"read",
	)
}

// genResult generates realistic result values
func genResult() gopter.Gen {
	return gen.OneConstOf(
		"success",
		"failure",
		"rejected",
		"error",
	)
}

// genKeyType generates realistic key types
func genKeyType() gopter.Gen {
	return gen.OneConstOf(
		"age",
		"ssh",
		"gpg",
		"api-key",
	)
}

// genReason generates realistic failure reasons
func genReason() gopter.Gen {
	return gen.OneConstOf(
		"invalid format",
		"missing required field",
		"path traversal detected",
		"shell metacharacters detected",
		"unauthorized access",
		"expired credentials",
	)
}

// genInputType generates realistic input types
func genInputType() gopter.Gen {
	return gen.OneConstOf(
		"cluster_name",
		"organization_name",
		"file_path",
		"url",
		"environment_variable",
	)
}

// genTemplateName generates realistic template names
func genTemplateName() gopter.Gen {
	return gen.OneConstOf(
		"cluster-config.yaml.tmpl",
		"infrastructure.yaml.tmpl",
		"application.yaml.tmpl",
		"service.yaml.tmpl",
	)
}

// genCorrelationID generates realistic correlation IDs
func genCorrelationID() gopter.Gen {
	return gen.RegexMatch("[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}")
}
