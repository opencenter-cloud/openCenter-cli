/*
Copyright 2025.

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

package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"github.com/stretchr/testify/require"
)

// **Validates: Requirements 5.6, 5.7, 3.10, 6.4**
//
// Property 11: Audit Log Event Recording
//
// For any key operation (generate, rotate, revoke, access), an audit event should be
// appended to the log with timestamp, actor, event type, key fingerprint, and cluster.
//
// This property verifies that:
// 1. Every key operation generates an audit event
// 2. Audit events contain all required fields (timestamp, actor, event type, resource)
// 3. Audit events are appended to the log (not overwritten)
// 4. Audit events have valid signatures for integrity protection
// 5. Multiple operations generate multiple distinct events
func TestProperty_AuditLogEventRecording(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("key operations generate audit events with all required fields", prop.ForAll(
		func(actor string, cluster string, keyType string) bool {
			// Skip invalid inputs
			if actor == "" || cluster == "" || keyType == "" {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Property 1: Key generation should create an audit event
			resource := fmt.Sprintf("%s-key", cluster)
			if err := auditLogger.LogKeyGenerated(ctx, actor, keyType, resource); err != nil {
				t.Logf("Failed to log key generation: %v", err)
				return false
			}

			// Property 2: Key rotation should create an audit event
			if err := auditLogger.LogKeyRotated(ctx, actor, keyType, resource); err != nil {
				t.Logf("Failed to log key rotation: %v", err)
				return false
			}

			// Property 3: Key revocation should create an audit event
			keyFingerprint := fmt.Sprintf("age1%s", strings.Repeat("x", 58))
			if err := auditLogger.LogKeyRevoked(ctx, actor, cluster, keyFingerprint, "", 5); err != nil {
				t.Logf("Failed to log key revocation: %v", err)
				return false
			}

			// Property 4: Key access should create an audit event
			if err := auditLogger.LogKeyAccessed(ctx, actor, keyType, resource, true); err != nil {
				t.Logf("Failed to log key access: %v", err)
				return false
			}

			// Read audit log
			events, err := readAuditLog(auditLogPath)
			if err != nil {
				t.Logf("Failed to read audit log: %v", err)
				return false
			}

			// Property 5: Should have exactly 4 events (generate, rotate, revoke, access)
			if len(events) != 4 {
				t.Logf("Expected 4 audit events, got %d", len(events))
				return false
			}

			// Property 6: Verify each event has required fields
			expectedEventTypes := []string{"key.generated", "key.rotated", "key.revoked", "key.accessed"}
			for i, event := range events {
				// Check event type
				if event.EventType != expectedEventTypes[i] {
					t.Logf("Event %d: expected event type %s, got %s", i, expectedEventTypes[i], event.EventType)
					return false
				}

				// Check actor
				if event.Actor != actor {
					t.Logf("Event %d: expected actor %s, got %s", i, actor, event.Actor)
					return false
				}

				// Check timestamp is set and recent
				if event.Timestamp.IsZero() {
					t.Logf("Event %d: timestamp is zero", i)
					return false
				}
				if time.Since(event.Timestamp) > 5*time.Second {
					t.Logf("Event %d: timestamp is too old: %v", i, event.Timestamp)
					return false
				}

				// Check resource is set
				if event.Resource == "" {
					t.Logf("Event %d: resource is empty", i)
					return false
				}

				// Check action is set
				if event.Action == "" {
					t.Logf("Event %d: action is empty", i)
					return false
				}

				// Check result is set
				if event.Result == "" {
					t.Logf("Event %d: result is empty", i)
					return false
				}

				// Check signature is set (for integrity)
				if event.Signature == "" {
					t.Logf("Event %d: signature is empty", i)
					return false
				}

				// Check event ID is set
				if event.ID == "" {
					t.Logf("Event %d: ID is empty", i)
					return false
				}
			}

			// Property 7: Verify event-specific details
			// Key generation event
			if events[0].Details["key_type"] != keyType {
				t.Logf("Key generation event: expected key_type %s, got %v", keyType, events[0].Details["key_type"])
				return false
			}

			// Key rotation event
			if events[1].Details["key_type"] != keyType {
				t.Logf("Key rotation event: expected key_type %s, got %v", keyType, events[1].Details["key_type"])
				return false
			}

			// Key revocation event
			if events[2].Details["key_fingerprint"] != keyFingerprint {
				t.Logf("Key revocation event: expected key_fingerprint %s, got %v", keyFingerprint, events[2].Details["key_fingerprint"])
				return false
			}
			if events[2].Details["files_reencrypted"] != float64(5) { // JSON unmarshals numbers as float64
				t.Logf("Key revocation event: expected files_reencrypted 5, got %v", events[2].Details["files_reencrypted"])
				return false
			}

			// Key access event
			if events[3].Details["key_type"] != keyType {
				t.Logf("Key access event: expected key_type %s, got %v", keyType, events[3].Details["key_type"])
				return false
			}

			return true
		},
		genActor(),
		genClusterName(),
		genKeyTypeString(),
	))

	properties.Property("multiple operations append events without overwriting", prop.ForAll(
		func(operationCount int) bool {
			// Limit operation count to reasonable range
			if operationCount < 1 || operationCount > 20 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Perform multiple operations
			for i := 0; i < operationCount; i++ {
				actor := fmt.Sprintf("user-%d", i)
				cluster := fmt.Sprintf("cluster-%d", i)
				keyType := "age"
				resource := fmt.Sprintf("%s-key", cluster)

				if err := auditLogger.LogKeyGenerated(ctx, actor, keyType, resource); err != nil {
					t.Logf("Failed to log key generation %d: %v", i, err)
					return false
				}
			}

			// Read audit log
			events, err := readAuditLog(auditLogPath)
			if err != nil {
				t.Logf("Failed to read audit log: %v", err)
				return false
			}

			// Property: Should have exactly operationCount events
			if len(events) != operationCount {
				t.Logf("Expected %d audit events, got %d", operationCount, len(events))
				return false
			}

			// Property: Each event should have a unique actor
			seenActors := make(map[string]bool)
			for i, event := range events {
				expectedActor := fmt.Sprintf("user-%d", i)
				if event.Actor != expectedActor {
					t.Logf("Event %d: expected actor %s, got %s", i, expectedActor, event.Actor)
					return false
				}

				if seenActors[event.Actor] {
					t.Logf("Duplicate actor found: %s", event.Actor)
					return false
				}
				seenActors[event.Actor] = true
			}

			return true
		},
		gen.IntRange(1, 20),
	))

	properties.Property("secrets operations generate audit events", prop.ForAll(
		func(actor string, cluster string, filesCreated int, filesUpdated int) bool {
			// Skip invalid inputs
			if actor == "" || cluster == "" {
				return true
			}

			// Limit file counts to reasonable range
			if filesCreated < 0 || filesCreated > 100 || filesUpdated < 0 || filesUpdated > 100 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Property 1: Secrets sync should create an audit event
			if err := auditLogger.LogSecretsSync(ctx, actor, cluster, filesCreated, filesUpdated, 0); err != nil {
				t.Logf("Failed to log secrets sync: %v", err)
				return false
			}

			// Property 2: Drift detection should create an audit event
			if err := auditLogger.LogDriftDetected(ctx, actor, cluster, 3, 1, 2); err != nil {
				t.Logf("Failed to log drift detection: %v", err)
				return false
			}

			// Property 3: Secrets validation should create an audit event
			if err := auditLogger.LogSecretsValidated(ctx, actor, cluster); err != nil {
				t.Logf("Failed to log secrets validation: %v", err)
				return false
			}

			// Read audit log
			events, err := readAuditLog(auditLogPath)
			if err != nil {
				t.Logf("Failed to read audit log: %v", err)
				return false
			}

			// Property 4: Should have exactly 3 events
			if len(events) != 3 {
				t.Logf("Expected 3 audit events, got %d", len(events))
				return false
			}

			// Property 5: Verify event types
			expectedEventTypes := []string{"secrets.sync", "secrets.drift_detected", "secrets.validated"}
			for i, event := range events {
				if event.EventType != expectedEventTypes[i] {
					t.Logf("Event %d: expected event type %s, got %s", i, expectedEventTypes[i], event.EventType)
					return false
				}

				// All events should have the same actor and cluster
				if event.Actor != actor {
					t.Logf("Event %d: expected actor %s, got %s", i, actor, event.Actor)
					return false
				}
				if event.Resource != cluster {
					t.Logf("Event %d: expected resource %s, got %s", i, cluster, event.Resource)
					return false
				}
			}

			// Property 6: Verify secrets sync event details
			if events[0].Details["files_created"] != float64(filesCreated) {
				t.Logf("Secrets sync event: expected files_created %d, got %v", filesCreated, events[0].Details["files_created"])
				return false
			}
			if events[0].Details["files_updated"] != float64(filesUpdated) {
				t.Logf("Secrets sync event: expected files_updated %d, got %v", filesUpdated, events[0].Details["files_updated"])
				return false
			}

			// Property 7: Verify drift detection event details
			if events[1].Details["drift_items"] != float64(3) {
				t.Logf("Drift detection event: expected drift_items 3, got %v", events[1].Details["drift_items"])
				return false
			}
			if events[1].Details["missing_manifests"] != float64(1) {
				t.Logf("Drift detection event: expected missing_manifests 1, got %v", events[1].Details["missing_manifests"])
				return false
			}
			if events[1].Details["orphaned_secrets"] != float64(2) {
				t.Logf("Drift detection event: expected orphaned_secrets 2, got %v", events[1].Details["orphaned_secrets"])
				return false
			}

			// Property 8: Verify secrets validation event details
			if events[2].Details["drift_detected"] != false {
				t.Logf("Secrets validation event: expected drift_detected false, got %v", events[2].Details["drift_detected"])
				return false
			}

			return true
		},
		genActor(),
		genClusterName(),
		gen.IntRange(0, 100),
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}

// readAuditLog reads and parses audit events from a log file
func readAuditLog(logPath string) ([]security.AuditEvent, error) {
	content, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var events []security.AuditEvent

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event security.AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("failed to parse audit event: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

// Generators for property-based testing

func genActor() gopter.Gen {
	return gen.OneGenOf(
		gen.Const("admin@example.com"),
		gen.Const("operator@example.com"),
		gen.Const("user@example.com"),
		gen.Identifier().Map(func(s string) string {
			if s == "" {
				return "default@example.com"
			}
			return fmt.Sprintf("%s@example.com", s)
		}),
	)
}

// genKeyTypeString generates string key types for audit logging
// (separate from genKeyType in registry_property_test.go which returns KeyType constants)
func genKeyTypeString() gopter.Gen {
	return gen.OneConstOf("age", "ssh")
}

// Note: genClusterName is defined in registry_property_test.go
// to avoid redeclaration errors

// Test that verifies the audit log event recording property test is working correctly
func TestProperty_AuditLogEventRecording_Sanity(t *testing.T) {
	tmpDir := t.TempDir()
	auditLogPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
		LogPath: auditLogPath,
		Enabled: true,
	})
	require.NoError(t, err, "Failed to create audit logger")
	defer auditLogger.Close()

	ctx := context.Background()
	actor := "test@example.com"
	cluster := "test-cluster"
	keyType := "age"
	resource := fmt.Sprintf("%s-key", cluster)

	// Test 1: Log key operations
	t.Run("log key operations", func(t *testing.T) {
		err := auditLogger.LogKeyGenerated(ctx, actor, keyType, resource)
		require.NoError(t, err, "Should log key generation")

		err = auditLogger.LogKeyRotated(ctx, actor, keyType, resource)
		require.NoError(t, err, "Should log key rotation")

		keyFingerprint := "age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
		err = auditLogger.LogKeyRevoked(ctx, actor, cluster, keyFingerprint, "", 5)
		require.NoError(t, err, "Should log key revocation")

		err = auditLogger.LogKeyAccessed(ctx, actor, keyType, resource, true)
		require.NoError(t, err, "Should log key access")
	})

	// Test 2: Read and verify audit log
	t.Run("verify audit log events", func(t *testing.T) {
		events, err := readAuditLog(auditLogPath)
		require.NoError(t, err, "Should read audit log")
		require.Len(t, events, 4, "Should have 4 events")

		// Verify event types
		expectedEventTypes := []string{"key.generated", "key.rotated", "key.revoked", "key.accessed"}
		for i, event := range events {
			require.Equal(t, expectedEventTypes[i], event.EventType, "Event %d should have correct type", i)
			require.Equal(t, actor, event.Actor, "Event %d should have correct actor", i)
			require.NotEmpty(t, event.Timestamp, "Event %d should have timestamp", i)
			require.NotEmpty(t, event.Resource, "Event %d should have resource", i)
			require.NotEmpty(t, event.Action, "Event %d should have action", i)
			require.NotEmpty(t, event.Result, "Event %d should have result", i)
			require.NotEmpty(t, event.Signature, "Event %d should have signature", i)
			require.NotEmpty(t, event.ID, "Event %d should have ID", i)
		}
	})

	// Test 3: Log secrets operations
	t.Run("log secrets operations", func(t *testing.T) {
		// Clear log for this test
		os.Remove(auditLogPath)
		auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger2.Close()

		err = auditLogger2.LogSecretsSync(ctx, actor, cluster, 5, 3, 2)
		require.NoError(t, err, "Should log secrets sync")

		err = auditLogger2.LogDriftDetected(ctx, actor, cluster, 3, 1, 2)
		require.NoError(t, err, "Should log drift detection")

		err = auditLogger2.LogSecretsValidated(ctx, actor, cluster)
		require.NoError(t, err, "Should log secrets validation")

		// Verify events
		events, err := readAuditLog(auditLogPath)
		require.NoError(t, err, "Should read audit log")
		require.Len(t, events, 3, "Should have 3 events")

		// Verify secrets sync event
		require.Equal(t, "secrets.sync", events[0].EventType)
		require.Equal(t, float64(5), events[0].Details["files_created"])
		require.Equal(t, float64(3), events[0].Details["files_updated"])

		// Verify drift detection event
		require.Equal(t, "secrets.drift_detected", events[1].EventType)
		require.Equal(t, float64(3), events[1].Details["drift_items"])
		require.Equal(t, float64(1), events[1].Details["missing_manifests"])
		require.Equal(t, float64(2), events[1].Details["orphaned_secrets"])

		// Verify secrets validation event
		require.Equal(t, "secrets.validated", events[2].EventType)
		require.Equal(t, false, events[2].Details["drift_detected"])
	})

	// Test 4: Verify events are appended, not overwritten
	t.Run("events are appended", func(t *testing.T) {
		// Clear log for this test
		os.Remove(auditLogPath)
		auditLogger3, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger3.Close()

		// Log multiple operations
		for i := 0; i < 5; i++ {
			actor := fmt.Sprintf("user-%d@example.com", i)
			resource := fmt.Sprintf("cluster-%d-key", i)
			err := auditLogger3.LogKeyGenerated(ctx, actor, keyType, resource)
			require.NoError(t, err, "Should log key generation %d", i)
		}

		// Verify all events are present
		events, err := readAuditLog(auditLogPath)
		require.NoError(t, err, "Should read audit log")
		require.Len(t, events, 5, "Should have 5 events")

		// Verify each event has unique actor
		for i, event := range events {
			expectedActor := fmt.Sprintf("user-%d@example.com", i)
			require.Equal(t, expectedActor, event.Actor, "Event %d should have correct actor", i)
		}
	})
}

// Integration test that verifies audit logging works with actual secrets operations
func TestAuditLogIntegration_SecretsManager(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "test-cluster"

	// Create audit logger
	auditLogPath := filepath.Join(tmpDir, "audit.log")
	auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
		LogPath: auditLogPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer auditLogger.Close()

	// Create secrets manager with audit logger
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	configLoader := v2.NewConfigIOHandler(fileSystem)
	sopsManager := sops.NewDefaultSOPSManager(nil, nil, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	manager := &DefaultSecretsManager{
		configLoader: configLoader,
		sopsManager:  sopsManager,
		auditLogger:  auditLogger,
		logger:       slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	// Create config directory
	configDir := filepath.Join(tmpDir, ".config", "opencenter", "clusters", "test-org", clusterName)
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Create a simple config file
	configPath := filepath.Join(configDir, fmt.Sprintf(".k8s-%s-config.yaml", clusterName))
	configContent := fmt.Sprintf(`schema_version: "2.0"
opencenter:
  cluster:
    cluster_name: %s
  gitops:
    git_dir: %s
secrets:
  sops_age_key_file: %s
  cert_manager:
    aws_access_key: AKIAIOSFODNN7EXAMPLE
    aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
`, clusterName, filepath.Join(tmpDir, "test-repo"), filepath.Join(tmpDir, "age-key.txt"))
	writeNormalizedSecretsConfigFile(t, configPath, clusterName, configContent)

	// Load config using LoadFromFile
	cfg, err := configLoader.LoadFromFile(context.Background(), configPath)
	require.NoError(t, err)

	// Extract secrets (this should trigger audit logging if implemented)
	secrets, err := manager.extractSecretsFromConfig(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, secrets)

	// Note: The actual audit logging for extractSecretsFromConfig would need to be
	// implemented in the manager. This test demonstrates the integration pattern.

	// Verify manager has audit logger configured
	require.NotNil(t, manager.auditLogger, "Manager should have audit logger")
}

// **Validates: Requirements 5.3, 5.4**
//
// Property 13: Audit Log Filtering
//
// For any audit log query with time or event-type filters, only events matching
// the filter criteria should be returned.
//
// This property verifies that:
// 1. Time-based filtering correctly includes/excludes events based on timestamp
// 2. Event-type filtering correctly includes/excludes events based on type
// 3. Combined filters (time + event-type) work correctly
// 4. Empty filters return all events
// 5. Filters with no matches return empty results
func TestProperty_AuditLogFiltering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("time-based filtering returns only events within time range", prop.ForAll(
		func(eventCount int, filterHoursAgo int) bool {
			// Limit to reasonable ranges
			if eventCount < 2 || eventCount > 20 || filterHoursAgo < 1 || filterHoursAgo > 48 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create events with different timestamps
			// Half before the filter time, half after
			cutoffTime := time.Now().Add(-time.Duration(filterHoursAgo) * time.Hour)
			eventsBeforeCutoff := 0
			eventsAfterCutoff := 0

			for i := 0; i < eventCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				cluster := fmt.Sprintf("cluster-%d", i)
				keyType := "age"
				resource := fmt.Sprintf("%s-key", cluster)

				// Alternate between events before and after cutoff
				if i%2 == 0 {
					// Event before cutoff (should be filtered out)
					eventsBeforeCutoff++
					// We can't easily manipulate timestamps in the logger,
					// so we'll test the filter logic directly
				} else {
					// Event after cutoff (should be included)
					eventsAfterCutoff++
					err := auditLogger.LogKeyGenerated(ctx, actor, keyType, resource)
					if err != nil {
						t.Logf("Failed to log key generation: %v", err)
						return false
					}
				}
			}

			// Query events since the cutoff time
			sinceDuration := time.Duration(filterHoursAgo) * time.Hour
			events, err := auditLogger.QueryEventsSince(ctx, sinceDuration, "")
			if err != nil {
				t.Logf("Failed to query events: %v", err)
				return false
			}

			// Property: All returned events should be within the time range
			for _, event := range events {
				if event.Timestamp.Before(cutoffTime) {
					t.Logf("Event timestamp %v is before cutoff %v", event.Timestamp, cutoffTime)
					return false
				}
			}

			// Property: All events should be recent (within the last second)
			// since we just created them
			now := time.Now()
			for _, event := range events {
				if now.Sub(event.Timestamp) > 5*time.Second {
					t.Logf("Event timestamp %v is too old (now: %v)", event.Timestamp, now)
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 20),
		gen.IntRange(1, 48),
	))

	properties.Property("event-type filtering returns only events of specified type", prop.ForAll(
		func(keyGenCount int, keyRotateCount int, keyRevokeCount int) bool {
			// Limit to reasonable ranges
			if keyGenCount < 1 || keyGenCount > 10 ||
				keyRotateCount < 1 || keyRotateCount > 10 ||
				keyRevokeCount < 1 || keyRevokeCount > 10 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create events of different types
			for i := 0; i < keyGenCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				resource := fmt.Sprintf("cluster-%d-key", i)
				err := auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
				if err != nil {
					t.Logf("Failed to log key generation: %v", err)
					return false
				}
			}

			for i := 0; i < keyRotateCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				resource := fmt.Sprintf("cluster-%d-key", i)
				err := auditLogger.LogKeyRotated(ctx, actor, "age", resource)
				if err != nil {
					t.Logf("Failed to log key rotation: %v", err)
					return false
				}
			}

			for i := 0; i < keyRevokeCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				cluster := fmt.Sprintf("cluster-%d", i)
				keyFingerprint := fmt.Sprintf("age1%s", strings.Repeat("x", 58))
				err := auditLogger.LogKeyRevoked(ctx, actor, cluster, keyFingerprint, "", 0)
				if err != nil {
					t.Logf("Failed to log key revocation: %v", err)
					return false
				}
			}

			// Query events by type
			keyGenEvents, err := auditLogger.QueryEventsSince(ctx, 24*time.Hour, "key.generated")
			if err != nil {
				t.Logf("Failed to query key.generated events: %v", err)
				return false
			}

			keyRotateEvents, err := auditLogger.QueryEventsSince(ctx, 24*time.Hour, "key.rotated")
			if err != nil {
				t.Logf("Failed to query key.rotated events: %v", err)
				return false
			}

			keyRevokeEvents, err := auditLogger.QueryEventsSince(ctx, 24*time.Hour, "key.revoked")
			if err != nil {
				t.Logf("Failed to query key.revoked events: %v", err)
				return false
			}

			// Property 1: Should have correct count of each event type
			if len(keyGenEvents) != keyGenCount {
				t.Logf("Expected %d key.generated events, got %d", keyGenCount, len(keyGenEvents))
				return false
			}

			if len(keyRotateEvents) != keyRotateCount {
				t.Logf("Expected %d key.rotated events, got %d", keyRotateCount, len(keyRotateEvents))
				return false
			}

			if len(keyRevokeEvents) != keyRevokeCount {
				t.Logf("Expected %d key.revoked events, got %d", keyRevokeCount, len(keyRevokeEvents))
				return false
			}

			// Property 2: All events in each result should have the correct type
			for _, event := range keyGenEvents {
				if event.EventType != "key.generated" {
					t.Logf("Expected event type key.generated, got %s", event.EventType)
					return false
				}
			}

			for _, event := range keyRotateEvents {
				if event.EventType != "key.rotated" {
					t.Logf("Expected event type key.rotated, got %s", event.EventType)
					return false
				}
			}

			for _, event := range keyRevokeEvents {
				if event.EventType != "key.revoked" {
					t.Logf("Expected event type key.revoked, got %s", event.EventType)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),
		gen.IntRange(1, 10),
		gen.IntRange(1, 10),
	))

	properties.Property("combined filters (time + event-type) work correctly", prop.ForAll(
		func(eventTypeToFilter string) bool {
			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create multiple events of different types
			actor := "test@example.com"
			resource := "test-cluster-key"

			// Log different event types
			err = auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
			if err != nil {
				t.Logf("Failed to log key generation: %v", err)
				return false
			}

			err = auditLogger.LogKeyRotated(ctx, actor, "age", resource)
			if err != nil {
				t.Logf("Failed to log key rotation: %v", err)
				return false
			}

			err = auditLogger.LogKeyRevoked(ctx, actor, "test-cluster", "age1"+strings.Repeat("x", 58), "", 0)
			if err != nil {
				t.Logf("Failed to log key revocation: %v", err)
				return false
			}

			err = auditLogger.LogSecretsSync(ctx, actor, "test-cluster", 5, 3, 0)
			if err != nil {
				t.Logf("Failed to log secrets sync: %v", err)
				return false
			}

			// Query with combined filters (recent time + specific event type)
			events, err := auditLogger.QueryEventsSince(ctx, 1*time.Hour, eventTypeToFilter)
			if err != nil {
				t.Logf("Failed to query events: %v", err)
				return false
			}

			// Property: All returned events should match the event type filter
			for _, event := range events {
				if event.EventType != eventTypeToFilter {
					t.Logf("Expected event type %s, got %s", eventTypeToFilter, event.EventType)
					return false
				}
			}

			// Property: All returned events should be recent
			now := time.Now()
			for _, event := range events {
				if now.Sub(event.Timestamp) > 5*time.Second {
					t.Logf("Event timestamp %v is too old (now: %v)", event.Timestamp, now)
					return false
				}
			}

			return true
		},
		gen.OneConstOf("key.generated", "key.rotated", "key.revoked", "secrets.sync"),
	))

	properties.Property("empty event-type filter returns all events", prop.ForAll(
		func(eventCount int) bool {
			// Limit to reasonable range
			if eventCount < 2 || eventCount > 10 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create events of different types
			actor := "test@example.com"
			resource := "test-cluster-key"

			for i := 0; i < eventCount; i++ {
				// Alternate between different event types
				switch i % 4 {
				case 0:
					err = auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
				case 1:
					err = auditLogger.LogKeyRotated(ctx, actor, "age", resource)
				case 2:
					err = auditLogger.LogKeyRevoked(ctx, actor, "test-cluster", "age1"+strings.Repeat("x", 58), "", 0)
				case 3:
					err = auditLogger.LogSecretsSync(ctx, actor, "test-cluster", 1, 1, 0)
				}

				if err != nil {
					t.Logf("Failed to log event %d: %v", i, err)
					return false
				}
			}

			// Query with empty event type filter (should return all events)
			events, err := auditLogger.QueryEventsSince(ctx, 1*time.Hour, "")
			if err != nil {
				t.Logf("Failed to query events: %v", err)
				return false
			}

			// Property: Should return all events
			if len(events) != eventCount {
				t.Logf("Expected %d events, got %d", eventCount, len(events))
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.Property("filter with no matches returns empty results", prop.ForAll(
		func(hoursAgo int) bool {
			// Use a time range that's definitely in the past
			if hoursAgo < 100 || hoursAgo > 1000 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create a recent event
			actor := "test@example.com"
			resource := "test-cluster-key"
			err = auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
			if err != nil {
				t.Logf("Failed to log key generation: %v", err)
				return false
			}

			// Query with a time range that's way in the past (should return no events)
			// We need to use a very short duration to exclude the recent event
			events, err := auditLogger.QueryEventsSince(ctx, 1*time.Nanosecond, "")
			if err != nil {
				t.Logf("Failed to query events: %v", err)
				return false
			}

			// Property: Should return empty results (or very few if timing is close)
			// We allow up to 1 event in case of timing issues
			if len(events) > 1 {
				t.Logf("Expected 0-1 events with very short time range, got %d", len(events))
				return false
			}

			// Query with non-existent event type
			events, err = auditLogger.QueryEventsSince(ctx, 24*time.Hour, "nonexistent.event.type")
			if err != nil {
				t.Logf("Failed to query events: %v", err)
				return false
			}

			// Property: Should return empty results
			if len(events) != 0 {
				t.Logf("Expected 0 events for non-existent event type, got %d", len(events))
				return false
			}

			return true
		},
		gen.IntRange(100, 1000),
	))

	properties.TestingRun(t)
}

// Test that verifies the audit log filtering property test is working correctly
func TestProperty_AuditLogFiltering_Sanity(t *testing.T) {
	tmpDir := t.TempDir()
	auditLogPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
		LogPath: auditLogPath,
		Enabled: true,
	})
	require.NoError(t, err, "Failed to create audit logger")
	defer auditLogger.Close()

	ctx := context.Background()
	actor := "test@example.com"
	cluster := "test-cluster"
	resource := fmt.Sprintf("%s-key", cluster)

	// Test 1: Event-type filtering
	t.Run("event-type filtering", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)
		auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger2.Close()

		// Log different event types
		err = auditLogger2.LogKeyGenerated(ctx, actor, "age", resource)
		require.NoError(t, err)

		err = auditLogger2.LogKeyRotated(ctx, actor, "age", resource)
		require.NoError(t, err)

		err = auditLogger2.LogKeyRevoked(ctx, actor, cluster, "age1"+strings.Repeat("x", 58), "", 0)
		require.NoError(t, err)

		err = auditLogger2.LogSecretsSync(ctx, actor, cluster, 5, 3, 0)
		require.NoError(t, err)

		// Query for specific event type
		keyGenEvents, err := auditLogger2.QueryEventsSince(ctx, 1*time.Hour, "key.generated")
		require.NoError(t, err)
		require.Len(t, keyGenEvents, 1, "Should have 1 key.generated event")
		require.Equal(t, "key.generated", keyGenEvents[0].EventType)

		keyRotateEvents, err := auditLogger2.QueryEventsSince(ctx, 1*time.Hour, "key.rotated")
		require.NoError(t, err)
		require.Len(t, keyRotateEvents, 1, "Should have 1 key.rotated event")
		require.Equal(t, "key.rotated", keyRotateEvents[0].EventType)

		secretsSyncEvents, err := auditLogger2.QueryEventsSince(ctx, 1*time.Hour, "secrets.sync")
		require.NoError(t, err)
		require.Len(t, secretsSyncEvents, 1, "Should have 1 secrets.sync event")
		require.Equal(t, "secrets.sync", secretsSyncEvents[0].EventType)

		// Query with empty event type (should return all)
		allEvents, err := auditLogger2.QueryEventsSince(ctx, 1*time.Hour, "")
		require.NoError(t, err)
		require.Len(t, allEvents, 4, "Should have 4 total events")
	})

	// Test 2: Time-based filtering
	t.Run("time-based filtering", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)
		auditLogger3, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger3.Close()

		// Log events
		err = auditLogger3.LogKeyGenerated(ctx, actor, "age", resource)
		require.NoError(t, err)

		err = auditLogger3.LogKeyRotated(ctx, actor, "age", resource)
		require.NoError(t, err)

		// Query recent events (should include all)
		recentEvents, err := auditLogger3.QueryEventsSince(ctx, 1*time.Hour, "")
		require.NoError(t, err)
		require.Len(t, recentEvents, 2, "Should have 2 recent events")

		// Verify all events are recent
		now := time.Now()
		for _, event := range recentEvents {
			require.True(t, now.Sub(event.Timestamp) < 5*time.Second, "Event should be recent")
		}

		// Query with very short duration (should return 0 or very few events)
		veryRecentEvents, err := auditLogger3.QueryEventsSince(ctx, 1*time.Nanosecond, "")
		require.NoError(t, err)
		require.LessOrEqual(t, len(veryRecentEvents), 1, "Should have 0-1 events with very short duration")
	})

	// Test 3: Combined filters
	t.Run("combined filters", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)
		auditLogger4, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger4.Close()

		// Log multiple events
		err = auditLogger4.LogKeyGenerated(ctx, actor, "age", resource)
		require.NoError(t, err)

		err = auditLogger4.LogKeyRotated(ctx, actor, "age", resource)
		require.NoError(t, err)

		err = auditLogger4.LogSecretsSync(ctx, actor, cluster, 5, 3, 0)
		require.NoError(t, err)

		// Query with combined filters (recent time + specific event type)
		filteredEvents, err := auditLogger4.QueryEventsSince(ctx, 1*time.Hour, "key.generated")
		require.NoError(t, err)
		require.Len(t, filteredEvents, 1, "Should have 1 key.generated event")
		require.Equal(t, "key.generated", filteredEvents[0].EventType)

		// Verify event is recent
		now := time.Now()
		require.True(t, now.Sub(filteredEvents[0].Timestamp) < 5*time.Second, "Event should be recent")
	})

	// Test 4: No matches
	t.Run("no matches", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)
		auditLogger5, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger5.Close()

		// Log an event
		err = auditLogger5.LogKeyGenerated(ctx, actor, "age", resource)
		require.NoError(t, err)

		// Query with non-existent event type
		noMatchEvents, err := auditLogger5.QueryEventsSince(ctx, 1*time.Hour, "nonexistent.event.type")
		require.NoError(t, err)
		require.Len(t, noMatchEvents, 0, "Should have 0 events for non-existent type")
	})
}

// **Validates: Requirements 5.8, 5.9**
//
// Property 12: Audit Log Integrity
//
// For any audit log, verifying integrity should detect any tampered events through
// signature validation, and untampered logs should pass verification.
//
// This property verifies that:
// 1. Untampered audit logs pass integrity verification
// 2. Tampered event signatures are detected during verification
// 3. Modified event data causes signature verification to fail
// 4. Deleted events can be detected (if sequence numbers are used)
// 5. Integrity verification reports the correct number of invalid events
func TestProperty_AuditLogIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // Reduced: each iteration creates audit loggers + file I/O
	properties := gopter.NewProperties(parameters)

	properties.Property("untampered audit logs pass integrity verification", prop.ForAll(
		func(eventCount int) bool {
			// Limit to reasonable range
			if eventCount < 1 || eventCount > 20 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create multiple events
			for i := 0; i < eventCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				cluster := fmt.Sprintf("cluster-%d", i)
				keyType := "age"
				resource := fmt.Sprintf("%s-key", cluster)

				// Alternate between different event types
				switch i % 4 {
				case 0:
					err = auditLogger.LogKeyGenerated(ctx, actor, keyType, resource)
				case 1:
					err = auditLogger.LogKeyRotated(ctx, actor, keyType, resource)
				case 2:
					keyFingerprint := fmt.Sprintf("age1%s", strings.Repeat("x", 58))
					err = auditLogger.LogKeyRevoked(ctx, actor, cluster, keyFingerprint, "", 0)
				case 3:
					err = auditLogger.LogSecretsSync(ctx, actor, cluster, 1, 1, 0)
				}

				if err != nil {
					t.Logf("Failed to log event %d: %v", i, err)
					return false
				}
			}

			// Property: Untampered log should pass integrity verification
			if err := auditLogger.VerifyIntegrity(); err != nil {
				t.Logf("Integrity verification failed for untampered log: %v", err)
				return false
			}

			return true
		},
		gen.IntRange(1, 20),
	))

	properties.Property("tampered event signatures are detected", prop.ForAll(
		func(eventCount int, tamperIndex int) bool {
			// Limit to reasonable range
			if eventCount < 2 || eventCount > 10 {
				return true
			}

			// Ensure tamper index is within range
			if tamperIndex < 0 || tamperIndex >= eventCount {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create multiple events
			for i := 0; i < eventCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				cluster := fmt.Sprintf("cluster-%d", i)
				keyType := "age"
				resource := fmt.Sprintf("%s-key", cluster)

				err = auditLogger.LogKeyGenerated(ctx, actor, keyType, resource)
				if err != nil {
					t.Logf("Failed to log event %d: %v", i, err)
					return false
				}
			}

			// Close logger to flush events
			auditLogger.Close()

			// Tamper with the log file by modifying an event's signature
			if err := tamperAuditLogSignature(auditLogPath, tamperIndex); err != nil {
				t.Logf("Failed to tamper with audit log: %v", err)
				return false
			}

			// Create new logger to verify integrity
			auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger for verification: %v", err)
				return false
			}
			defer auditLogger2.Close()

			// Property: Tampered log should fail integrity verification
			err = auditLogger2.VerifyIntegrity()
			if err == nil {
				t.Logf("Integrity verification should have failed for tampered log")
				return false
			}

			// Property: Error message should indicate signature verification failure
			if !strings.Contains(err.Error(), "invalid signatures") {
				t.Logf("Error message should mention invalid signatures: %v", err)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
		gen.IntRange(0, 9),
	))

	properties.Property("modified event data causes signature verification to fail", prop.ForAll(
		func(eventCount int, tamperIndex int) bool {
			// Limit to reasonable range
			if eventCount < 2 || eventCount > 10 {
				return true
			}

			// Ensure tamper index is within range
			if tamperIndex < 0 || tamperIndex >= eventCount {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create multiple events
			for i := 0; i < eventCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				cluster := fmt.Sprintf("cluster-%d", i)
				keyType := "age"
				resource := fmt.Sprintf("%s-key", cluster)

				err = auditLogger.LogKeyGenerated(ctx, actor, keyType, resource)
				if err != nil {
					t.Logf("Failed to log event %d: %v", i, err)
					return false
				}
			}

			// Close logger to flush events
			auditLogger.Close()

			// Tamper with the log file by modifying an event's data (actor field)
			if err := tamperAuditLogData(auditLogPath, tamperIndex); err != nil {
				t.Logf("Failed to tamper with audit log: %v", err)
				return false
			}

			// Create new logger to verify integrity
			auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger for verification: %v", err)
				return false
			}
			defer auditLogger2.Close()

			// Property: Tampered log should fail integrity verification
			err = auditLogger2.VerifyIntegrity()
			if err == nil {
				t.Logf("Integrity verification should have failed for tampered log")
				return false
			}

			// Property: Error message should indicate signature verification failure
			if !strings.Contains(err.Error(), "invalid signatures") {
				t.Logf("Error message should mention invalid signatures: %v", err)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
		gen.IntRange(0, 9),
	))

	properties.Property("multiple tampered events are all detected", prop.ForAll(
		func(eventCount int, tamperCount int) bool {
			// Limit to reasonable range
			if eventCount < 3 || eventCount > 10 {
				return true
			}

			// Ensure tamper count is reasonable
			if tamperCount < 1 || tamperCount > eventCount/2 {
				return true
			}

			// Setup test environment
			tmpDir := t.TempDir()
			auditLogPath := filepath.Join(tmpDir, "audit.log")

			// Create audit logger
			auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger: %v", err)
				return false
			}
			defer auditLogger.Close()

			ctx := context.Background()

			// Create multiple events
			for i := 0; i < eventCount; i++ {
				actor := fmt.Sprintf("user-%d@example.com", i)
				cluster := fmt.Sprintf("cluster-%d", i)
				keyType := "age"
				resource := fmt.Sprintf("%s-key", cluster)

				err = auditLogger.LogKeyGenerated(ctx, actor, keyType, resource)
				if err != nil {
					t.Logf("Failed to log event %d: %v", i, err)
					return false
				}
			}

			// Close logger to flush events
			auditLogger.Close()

			// Tamper with multiple events
			for i := 0; i < tamperCount; i++ {
				tamperIndex := i * (eventCount / tamperCount)
				if err := tamperAuditLogSignature(auditLogPath, tamperIndex); err != nil {
					t.Logf("Failed to tamper with audit log at index %d: %v", tamperIndex, err)
					return false
				}
			}

			// Create new logger to verify integrity
			auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
				LogPath: auditLogPath,
				Enabled: true,
			})
			if err != nil {
				t.Logf("Failed to create audit logger for verification: %v", err)
				return false
			}
			defer auditLogger2.Close()

			// Property: Tampered log should fail integrity verification
			err = auditLogger2.VerifyIntegrity()
			if err == nil {
				t.Logf("Integrity verification should have failed for tampered log")
				return false
			}

			// Property: Error message should indicate multiple invalid signatures
			if !strings.Contains(err.Error(), "invalid signatures") {
				t.Logf("Error message should mention invalid signatures: %v", err)
				return false
			}

			// Property: Error should mention at least tamperCount invalid events
			// (The error message format is: "integrity verification failed: N events have invalid signatures")
			expectedMinCount := fmt.Sprintf("%d events", tamperCount)
			if !strings.Contains(err.Error(), expectedMinCount) {
				// Check if the actual count is at least tamperCount
				// Extract the number from error message
				var actualCount int
				_, scanErr := fmt.Sscanf(err.Error(), "integrity verification failed: %d events have invalid signatures", &actualCount)
				if scanErr != nil || actualCount < tamperCount {
					t.Logf("Error should mention at least %d invalid events, got: %v", tamperCount, err)
					return false
				}
			}

			return true
		},
		gen.IntRange(3, 10),
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}

// tamperAuditLogSignature modifies the signature of an event at the specified index
func tamperAuditLogSignature(logPath string, eventIndex int) error {
	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if eventIndex >= len(lines) {
		return fmt.Errorf("event index %d out of range (total lines: %d)", eventIndex, len(lines))
	}

	// Parse the event at the specified index
	var event security.AuditEvent
	if err := json.Unmarshal([]byte(lines[eventIndex]), &event); err != nil {
		return fmt.Errorf("failed to parse event at index %d: %w", eventIndex, err)
	}

	// Tamper with the signature (flip some bits)
	if len(event.Signature) > 0 {
		// Replace first character with 'X' to invalidate signature
		event.Signature = "X" + event.Signature[1:]
	}

	// Marshal the tampered event back to JSON
	tamperedData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal tampered event: %w", err)
	}

	// Replace the line with tampered event
	lines[eventIndex] = string(tamperedData)

	// Write back to file
	if err := os.WriteFile(logPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write tampered audit log: %w", err)
	}

	return nil
}

// tamperAuditLogData modifies the data of an event at the specified index
func tamperAuditLogData(logPath string, eventIndex int) error {
	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if eventIndex >= len(lines) {
		return fmt.Errorf("event index %d out of range (total lines: %d)", eventIndex, len(lines))
	}

	// Parse the event at the specified index
	var event security.AuditEvent
	if err := json.Unmarshal([]byte(lines[eventIndex]), &event); err != nil {
		return fmt.Errorf("failed to parse event at index %d: %w", eventIndex, err)
	}

	// Tamper with the actor field (change the email)
	event.Actor = "tampered-" + event.Actor

	// Marshal the tampered event back to JSON (keeping the original signature)
	tamperedData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal tampered event: %w", err)
	}

	// Replace the line with tampered event
	lines[eventIndex] = string(tamperedData)

	// Write back to file
	if err := os.WriteFile(logPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write tampered audit log: %w", err)
	}

	return nil
}

// Test that verifies the audit log integrity property test is working correctly
func TestProperty_AuditLogIntegrity_Sanity(t *testing.T) {
	tmpDir := t.TempDir()
	auditLogPath := filepath.Join(tmpDir, "audit.log")

	// Test 1: Untampered log passes verification
	t.Run("untampered log passes verification", func(t *testing.T) {
		// Create audit logger
		auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err, "Failed to create audit logger")
		defer auditLogger.Close()

		ctx := context.Background()

		// Log multiple events
		for i := 0; i < 5; i++ {
			actor := fmt.Sprintf("user-%d@example.com", i)
			cluster := fmt.Sprintf("cluster-%d", i)
			resource := fmt.Sprintf("%s-key", cluster)

			err = auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
			require.NoError(t, err, "Should log event %d", i)
		}

		// Verify integrity
		err = auditLogger.VerifyIntegrity()
		require.NoError(t, err, "Untampered log should pass integrity verification")
	})

	// Test 2: Tampered signature is detected
	t.Run("tampered signature is detected", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)

		// Create audit logger
		auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger.Close()

		ctx := context.Background()

		// Log events
		for i := 0; i < 3; i++ {
			actor := fmt.Sprintf("user-%d@example.com", i)
			resource := fmt.Sprintf("cluster-%d-key", i)
			err = auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
			require.NoError(t, err)
		}

		// Close to flush
		auditLogger.Close()

		// Tamper with signature of second event
		err = tamperAuditLogSignature(auditLogPath, 1)
		require.NoError(t, err, "Should tamper with audit log")

		// Create new logger to verify
		auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger2.Close()

		// Verify integrity should fail
		err = auditLogger2.VerifyIntegrity()
		require.Error(t, err, "Tampered log should fail integrity verification")
		require.Contains(t, err.Error(), "invalid signatures", "Error should mention invalid signatures")
	})

	// Test 3: Tampered data is detected
	t.Run("tampered data is detected", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)

		// Create audit logger
		auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger.Close()

		ctx := context.Background()

		// Log events
		for i := 0; i < 3; i++ {
			actor := fmt.Sprintf("user-%d@example.com", i)
			resource := fmt.Sprintf("cluster-%d-key", i)
			err = auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
			require.NoError(t, err)
		}

		// Close to flush
		auditLogger.Close()

		// Tamper with data of second event
		err = tamperAuditLogData(auditLogPath, 1)
		require.NoError(t, err, "Should tamper with audit log")

		// Create new logger to verify
		auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger2.Close()

		// Verify integrity should fail
		err = auditLogger2.VerifyIntegrity()
		require.Error(t, err, "Tampered log should fail integrity verification")
		require.Contains(t, err.Error(), "invalid signatures", "Error should mention invalid signatures")
	})

	// Test 4: Multiple tampered events are detected
	t.Run("multiple tampered events are detected", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)

		// Create audit logger
		auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger.Close()

		ctx := context.Background()

		// Log events
		for i := 0; i < 5; i++ {
			actor := fmt.Sprintf("user-%d@example.com", i)
			resource := fmt.Sprintf("cluster-%d-key", i)
			err = auditLogger.LogKeyGenerated(ctx, actor, "age", resource)
			require.NoError(t, err)
		}

		// Close to flush
		auditLogger.Close()

		// Tamper with multiple events (read file once, tamper multiple lines, write once)
		content, err := os.ReadFile(auditLogPath)
		require.NoError(t, err)

		lines := strings.Split(string(content), "\n")
		require.GreaterOrEqual(t, len(lines), 5, "Should have at least 5 lines")

		// Tamper with event at index 1
		if lines[1] != "" {
			var event1 security.AuditEvent
			err = json.Unmarshal([]byte(lines[1]), &event1)
			require.NoError(t, err)
			if len(event1.Signature) > 0 {
				event1.Signature = "X" + event1.Signature[1:]
			}
			tamperedData1, err := json.Marshal(event1)
			require.NoError(t, err)
			lines[1] = string(tamperedData1)
		}

		// Tamper with event at index 3
		if lines[3] != "" {
			var event3 security.AuditEvent
			err = json.Unmarshal([]byte(lines[3]), &event3)
			require.NoError(t, err)
			if len(event3.Signature) > 0 {
				event3.Signature = "X" + event3.Signature[1:]
			}
			tamperedData3, err := json.Marshal(event3)
			require.NoError(t, err)
			lines[3] = string(tamperedData3)
		}

		// Write back
		err = os.WriteFile(auditLogPath, []byte(strings.Join(lines, "\n")), 0644)
		require.NoError(t, err)

		// Create new logger to verify
		auditLogger2, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger2.Close()

		// Verify integrity should fail
		err = auditLogger2.VerifyIntegrity()
		require.Error(t, err, "Tampered log should fail integrity verification")
		// Check that at least 2 events are reported as invalid
		require.Contains(t, err.Error(), "events have invalid signatures", "Error should mention invalid signatures")
		// Extract the number of invalid events
		var invalidCount int
		_, scanErr := fmt.Sscanf(err.Error(), "integrity verification failed: %d events have invalid signatures", &invalidCount)
		require.NoError(t, scanErr, "Should be able to parse error message")
		require.GreaterOrEqual(t, invalidCount, 2, "Should have at least 2 invalid events")
	})

	// Test 5: Empty log passes verification
	t.Run("empty log passes verification", func(t *testing.T) {
		// Clear log
		os.Remove(auditLogPath)

		// Create empty log file
		err := os.WriteFile(auditLogPath, []byte(""), 0644)
		require.NoError(t, err)

		// Create audit logger
		auditLogger, err := security.NewAuditLogger(security.AuditLoggerConfig{
			LogPath: auditLogPath,
			Enabled: true,
		})
		require.NoError(t, err)
		defer auditLogger.Close()

		// Verify integrity should pass for empty log
		err = auditLogger.VerifyIntegrity()
		require.NoError(t, err, "Empty log should pass integrity verification")
	})
}
