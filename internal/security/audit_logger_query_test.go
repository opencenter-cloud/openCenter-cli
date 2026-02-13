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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogger_QueryEventsSince(t *testing.T) {
	// Create temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Log events at different times
	now := time.Now()

	// Event 1: 2 hours ago
	event1 := AuditEvent{
		Timestamp: now.Add(-2 * time.Hour),
		EventType: "secrets.sync",
		Actor:     "user@example.com",
		Resource:  "cluster-1",
		Action:    "sync",
		Result:    "success",
	}
	err = logger.LogEvent(ctx, event1)
	require.NoError(t, err)

	// Event 2: 30 minutes ago
	event2 := AuditEvent{
		Timestamp: now.Add(-30 * time.Minute),
		EventType: "key.rotated",
		Actor:     "user@example.com",
		Resource:  "cluster-1",
		Action:    "rotate",
		Result:    "success",
	}
	err = logger.LogEvent(ctx, event2)
	require.NoError(t, err)

	// Event 3: 5 minutes ago
	event3 := AuditEvent{
		Timestamp: now.Add(-5 * time.Minute),
		EventType: "secrets.validated",
		Actor:     "user@example.com",
		Resource:  "cluster-1",
		Action:    "validate",
		Result:    "success",
	}
	err = logger.LogEvent(ctx, event3)
	require.NoError(t, err)

	// Query events from last hour (should get event2 and event3)
	events, err := logger.QueryEventsSince(ctx, 1*time.Hour, "")
	require.NoError(t, err)
	assert.Len(t, events, 2, "Should return 2 events from last hour")

	// Verify the events are the correct ones
	assert.Equal(t, "key.rotated", events[0].EventType)
	assert.Equal(t, "secrets.validated", events[1].EventType)

	// Query events from last 3 hours (should get all 3)
	events, err = logger.QueryEventsSince(ctx, 3*time.Hour, "")
	require.NoError(t, err)
	assert.Len(t, events, 3, "Should return all 3 events from last 3 hours")

	// Query events from last 10 minutes (should get only event3)
	events, err = logger.QueryEventsSince(ctx, 10*time.Minute, "")
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should return 1 event from last 10 minutes")
	assert.Equal(t, "secrets.validated", events[0].EventType)
}

func TestAuditLogger_QueryEventsSinceWithEventType(t *testing.T) {
	// Create temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Log multiple events of different types
	now := time.Now()

	events := []AuditEvent{
		{
			Timestamp: now.Add(-2 * time.Hour),
			EventType: "secrets.sync",
			Actor:     "user@example.com",
			Resource:  "cluster-1",
			Action:    "sync",
			Result:    "success",
		},
		{
			Timestamp: now.Add(-1 * time.Hour),
			EventType: "key.rotated",
			Actor:     "user@example.com",
			Resource:  "cluster-1",
			Action:    "rotate",
			Result:    "success",
		},
		{
			Timestamp: now.Add(-30 * time.Minute),
			EventType: "secrets.sync",
			Actor:     "user@example.com",
			Resource:  "cluster-2",
			Action:    "sync",
			Result:    "success",
		},
		{
			Timestamp: now.Add(-10 * time.Minute),
			EventType: "key.revoked",
			Actor:     "admin@example.com",
			Resource:  "cluster-1",
			Action:    "revoke",
			Result:    "success",
		},
	}

	for _, event := range events {
		err = logger.LogEvent(ctx, event)
		require.NoError(t, err)
	}

	// Query only secrets.sync events from last 3 hours
	syncEvents, err := logger.QueryEventsSince(ctx, 3*time.Hour, "secrets.sync")
	require.NoError(t, err)
	assert.Len(t, syncEvents, 2, "Should return 2 secrets.sync events")

	for _, event := range syncEvents {
		assert.Equal(t, "secrets.sync", event.EventType)
	}

	// Query only key.rotated events from last 3 hours
	rotateEvents, err := logger.QueryEventsSince(ctx, 3*time.Hour, "key.rotated")
	require.NoError(t, err)
	assert.Len(t, rotateEvents, 1, "Should return 1 key.rotated event")
	assert.Equal(t, "key.rotated", rotateEvents[0].EventType)

	// Query key.revoked events from last 15 minutes
	revokeEvents, err := logger.QueryEventsSince(ctx, 15*time.Minute, "key.revoked")
	require.NoError(t, err)
	assert.Len(t, revokeEvents, 1, "Should return 1 key.revoked event")
	assert.Equal(t, "key.revoked", revokeEvents[0].EventType)

	// Query non-existent event type
	noEvents, err := logger.QueryEventsSince(ctx, 3*time.Hour, "nonexistent.event")
	require.NoError(t, err)
	assert.Len(t, noEvents, 0, "Should return 0 events for non-existent type")
}

func TestAuditLogger_ExportEventsToJSON(t *testing.T) {
	// Create temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Log some events
	events := []AuditEvent{
		{
			EventType: "secrets.sync",
			Actor:     "user@example.com",
			Resource:  "cluster-1",
			Action:    "sync",
			Result:    "success",
			Details: map[string]interface{}{
				"files_created": 5,
				"files_updated": 3,
			},
		},
		{
			EventType: "key.rotated",
			Actor:     "admin@example.com",
			Resource:  "cluster-1",
			Action:    "rotate",
			Result:    "success",
			Details: map[string]interface{}{
				"key_type": "age",
			},
		},
	}

	for _, event := range events {
		err = logger.LogEvent(ctx, event)
		require.NoError(t, err)
	}

	// Query all events
	queriedEvents, err := logger.QueryEvents(ctx, EventFilter{})
	require.NoError(t, err)
	require.Len(t, queriedEvents, 2)

	// Export to JSON file
	exportPath := filepath.Join(tmpDir, "export.json")
	err = logger.ExportEventsToJSON(queriedEvents, exportPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(exportPath)
	require.NoError(t, err)

	// Read and parse the exported JSON
	data, err := os.ReadFile(exportPath)
	require.NoError(t, err)

	var exportedEvents []AuditEvent
	err = json.Unmarshal(data, &exportedEvents)
	require.NoError(t, err)

	// Verify exported events match original
	assert.Len(t, exportedEvents, 2)
	assert.Equal(t, "secrets.sync", exportedEvents[0].EventType)
	assert.Equal(t, "key.rotated", exportedEvents[1].EventType)

	// Verify details are preserved
	assert.Equal(t, float64(5), exportedEvents[0].Details["files_created"])
	assert.Equal(t, float64(3), exportedEvents[0].Details["files_updated"])
	assert.Equal(t, "age", exportedEvents[1].Details["key_type"])
}

func TestAuditLogger_ExportEventsToJSON_EmptyEvents(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	// Export empty events list
	exportPath := filepath.Join(tmpDir, "empty_export.json")
	err = logger.ExportEventsToJSON([]AuditEvent{}, exportPath)
	require.NoError(t, err)

	// Read and verify
	data, err := os.ReadFile(exportPath)
	require.NoError(t, err)

	var exportedEvents []AuditEvent
	err = json.Unmarshal(data, &exportedEvents)
	require.NoError(t, err)
	assert.Len(t, exportedEvents, 0)
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "hours",
			input:    "24h",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "minutes",
			input:    "30m",
			expected: 30 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "days",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "single day",
			input:    "1d",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "30 days",
			input:    "30d",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "combined hours and minutes",
			input:    "1h30m",
			expected: 1*time.Hour + 30*time.Minute,
			wantErr:  false,
		},
		{
			name:     "seconds",
			input:    "300s",
			expected: 300 * time.Second,
			wantErr:  false,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid days format",
			input:    "abcd",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDuration(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAuditLogger_QueryEventsSince_NoEvents(t *testing.T) {
	// Create temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Query events when log is empty
	events, err := logger.QueryEventsSince(ctx, 1*time.Hour, "")
	require.NoError(t, err)
	assert.Len(t, events, 0, "Should return 0 events when log is empty")
}

func TestAuditLogger_ExportEventsToJSON_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	// Try to export to invalid path (directory that doesn't exist)
	invalidPath := filepath.Join(tmpDir, "nonexistent", "export.json")
	err = logger.ExportEventsToJSON([]AuditEvent{}, invalidPath)
	assert.Error(t, err, "Should fail when exporting to invalid path")
}

func TestAuditLogger_QueryEventsSince_MultipleEventTypes(t *testing.T) {
	// Create temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	now := time.Now()

	// Log various event types
	eventTypes := []string{
		"secrets.sync",
		"secrets.drift_detected",
		"secrets.validated",
		"key.generated",
		"key.rotated",
		"key.revoked",
		"key.accessed",
		"key.expired",
	}

	for i, eventType := range eventTypes {
		event := AuditEvent{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			EventType: eventType,
			Actor:     "user@example.com",
			Resource:  "cluster-1",
			Action:    "test",
			Result:    "success",
		}
		err = logger.LogEvent(ctx, event)
		require.NoError(t, err)
	}

	// Query all events
	allEvents, err := logger.QueryEventsSince(ctx, 1*time.Hour, "")
	require.NoError(t, err)
	assert.Len(t, allEvents, len(eventTypes), "Should return all events")

	// Query specific event types
	for _, eventType := range eventTypes {
		events, err := logger.QueryEventsSince(ctx, 1*time.Hour, eventType)
		require.NoError(t, err)
		assert.Len(t, events, 1, "Should return 1 event for type %s", eventType)
		assert.Equal(t, eventType, events[0].EventType)
	}
}
