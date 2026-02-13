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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogger_LogSecretsSync(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.LogSecretsSync(ctx, "test-user", "test-cluster", 5, 3, 2)
	require.NoError(t, err)

	// Read and verify the log entry
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var event AuditEvent
	err = json.Unmarshal(content, &event)
	require.NoError(t, err)

	assert.Equal(t, "secrets.sync", event.EventType)
	assert.Equal(t, "test-user", event.Actor)
	assert.Equal(t, "test-cluster", event.Resource)
	assert.Equal(t, "sync", event.Action)
	assert.Equal(t, "success", event.Result)
	assert.Equal(t, float64(5), event.Details["files_created"])
	assert.Equal(t, float64(3), event.Details["files_updated"])
	assert.Equal(t, float64(2), event.Details["files_unchanged"])
	assert.NotEmpty(t, event.Signature)
}

func TestAuditLogger_LogSecretsSyncFailed(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.LogSecretsSyncFailed(ctx, "test-user", "test-cluster", "encryption failed")
	require.NoError(t, err)

	// Read and verify the log entry
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var event AuditEvent
	err = json.Unmarshal(content, &event)
	require.NoError(t, err)

	assert.Equal(t, "secrets.sync", event.EventType)
	assert.Equal(t, "test-user", event.Actor)
	assert.Equal(t, "test-cluster", event.Resource)
	assert.Equal(t, "sync", event.Action)
	assert.Equal(t, "failure", event.Result)
	assert.Equal(t, "encryption failed", event.Details["reason"])
}

func TestAuditLogger_LogDriftDetected(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.LogDriftDetected(ctx, "test-user", "test-cluster", 3, 1, 2)
	require.NoError(t, err)

	// Read and verify the log entry
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var event AuditEvent
	err = json.Unmarshal(content, &event)
	require.NoError(t, err)

	assert.Equal(t, "secrets.drift_detected", event.EventType)
	assert.Equal(t, "test-user", event.Actor)
	assert.Equal(t, "test-cluster", event.Resource)
	assert.Equal(t, "validate", event.Action)
	assert.Equal(t, "drift_detected", event.Result)
	assert.Equal(t, float64(3), event.Details["drift_items"])
	assert.Equal(t, float64(1), event.Details["missing_manifests"])
	assert.Equal(t, float64(2), event.Details["orphaned_secrets"])
}

func TestAuditLogger_LogSecretsValidated(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.LogSecretsValidated(ctx, "test-user", "test-cluster")
	require.NoError(t, err)

	// Read and verify the log entry
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var event AuditEvent
	err = json.Unmarshal(content, &event)
	require.NoError(t, err)

	assert.Equal(t, "secrets.validated", event.EventType)
	assert.Equal(t, "test-user", event.Actor)
	assert.Equal(t, "test-cluster", event.Resource)
	assert.Equal(t, "validate", event.Action)
	assert.Equal(t, "success", event.Result)
	assert.Equal(t, false, event.Details["drift_detected"])
}

func TestAuditLogger_LogKeyRevoked(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.LogKeyRevoked(ctx, "admin-user", "test-cluster", "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn", "departed-user@example.com", 15)
	require.NoError(t, err)

	// Read and verify the log entry
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var event AuditEvent
	err = json.Unmarshal(content, &event)
	require.NoError(t, err)

	assert.Equal(t, "key.revoked", event.EventType)
	assert.Equal(t, "admin-user", event.Actor)
	assert.Equal(t, "test-cluster", event.Resource)
	assert.Equal(t, "revoke", event.Action)
	assert.Equal(t, "success", event.Result)
	assert.Equal(t, "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn", event.Details["key_fingerprint"])
	assert.Equal(t, "departed-user@example.com", event.Details["revoked_user"])
	assert.Equal(t, float64(15), event.Details["files_reencrypted"])
}

func TestAuditLogger_LogKeyRevocationFailed(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.LogKeyRevocationFailed(ctx, "admin-user", "test-cluster", "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn", "only key remaining")
	require.NoError(t, err)

	// Read and verify the log entry
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var event AuditEvent
	err = json.Unmarshal(content, &event)
	require.NoError(t, err)

	assert.Equal(t, "key.revoked", event.EventType)
	assert.Equal(t, "admin-user", event.Actor)
	assert.Equal(t, "test-cluster", event.Resource)
	assert.Equal(t, "revoke", event.Action)
	assert.Equal(t, "failure", event.Result)
	assert.Equal(t, "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn", event.Details["key_fingerprint"])
	assert.Equal(t, "only key remaining", event.Details["reason"])
}

func TestAuditLogger_LogKeyExpired(t *testing.T) {
	tests := []struct {
		name            string
		daysRemaining   int
		expectedResult  string
	}{
		{
			name:           "warning - 7 days remaining",
			daysRemaining:  7,
			expectedResult: "warning",
		},
		{
			name:           "expired - negative days",
			daysRemaining:  -5,
			expectedResult: "expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			require.NoError(t, err)
			defer logger.Close()

			ctx := context.Background()
			err = logger.LogKeyExpired(ctx, "system", "test-cluster", "age", "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn", tt.daysRemaining)
			require.NoError(t, err)

			// Read and verify the log entry
			content, err := os.ReadFile(logPath)
			require.NoError(t, err)

			var event AuditEvent
			err = json.Unmarshal(content, &event)
			require.NoError(t, err)

			assert.Equal(t, "key.expired", event.EventType)
			assert.Equal(t, "system", event.Actor)
			assert.Equal(t, "test-cluster", event.Resource)
			assert.Equal(t, "check_expiration", event.Action)
			assert.Equal(t, tt.expectedResult, event.Result)
			assert.Equal(t, "age", event.Details["key_type"])
			assert.Equal(t, "age15n3dugqfej2hk8cqz2kcx78v6lxwllk5gruu4ermz2hu539xrgwq0w7dyn", event.Details["key_fingerprint"])
			assert.Equal(t, float64(tt.daysRemaining), event.Details["days_remaining"])
		})
	}
}

func TestAuditLogger_LogSecretDecrypted(t *testing.T) {
	tests := []struct {
		name           string
		success        bool
		expectedResult string
	}{
		{
			name:           "successful decryption",
			success:        true,
			expectedResult: "success",
		},
		{
			name:           "failed decryption",
			success:        false,
			expectedResult: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "audit.log")

			logger, err := NewAuditLogger(AuditLoggerConfig{
				LogPath: logPath,
				Enabled: true,
			})
			require.NoError(t, err)
			defer logger.Close()

			ctx := context.Background()
			err = logger.LogSecretDecrypted(ctx, "test-user", "test-cluster", "applications/overlays/test-cluster/services/harbor/secret.yaml", tt.success)
			require.NoError(t, err)

			// Read and verify the log entry
			content, err := os.ReadFile(logPath)
			require.NoError(t, err)

			var event AuditEvent
			err = json.Unmarshal(content, &event)
			require.NoError(t, err)

			assert.Equal(t, "secret.decrypted", event.EventType)
			assert.Equal(t, "test-user", event.Actor)
			assert.Equal(t, "test-cluster", event.Resource)
			assert.Equal(t, "decrypt", event.Action)
			assert.Equal(t, tt.expectedResult, event.Result)
			assert.Equal(t, "applications/overlays/test-cluster/services/harbor/secret.yaml", event.Details["manifest_path"])
		})
	}
}

func TestAuditLogger_MultipleSecretsEvents(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Log multiple events
	err = logger.LogSecretsSync(ctx, "user1", "cluster1", 5, 3, 2)
	require.NoError(t, err)

	err = logger.LogDriftDetected(ctx, "user2", "cluster2", 2, 1, 0)
	require.NoError(t, err)

	err = logger.LogKeyRevoked(ctx, "admin", "cluster1", "age15abc", "user@example.com", 10)
	require.NoError(t, err)

	// Read and verify all entries
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := splitLines(string(content))
	assert.Len(t, lines, 3)

	// Verify first event
	var event1 AuditEvent
	err = json.Unmarshal([]byte(lines[0]), &event1)
	require.NoError(t, err)
	assert.Equal(t, "secrets.sync", event1.EventType)

	// Verify second event
	var event2 AuditEvent
	err = json.Unmarshal([]byte(lines[1]), &event2)
	require.NoError(t, err)
	assert.Equal(t, "secrets.drift_detected", event2.EventType)

	// Verify third event
	var event3 AuditEvent
	err = json.Unmarshal([]byte(lines[2]), &event3)
	require.NoError(t, err)
	assert.Equal(t, "key.revoked", event3.EventType)
}

func TestAuditLogger_SecretsEventsSignatureVerification(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Log events
	err = logger.LogSecretsSync(ctx, "user1", "cluster1", 5, 3, 2)
	require.NoError(t, err)

	err = logger.LogKeyRevoked(ctx, "admin", "cluster1", "age15abc", "user@example.com", 10)
	require.NoError(t, err)

	// Verify integrity
	err = logger.VerifyIntegrity()
	assert.NoError(t, err)
}
