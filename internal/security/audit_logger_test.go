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
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLogger_LogEvent(t *testing.T) {
	// Create temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Log an event
	event := AuditEvent{
		EventType: "test.event",
		Actor:     "test-user",
		Resource:  "test-resource",
		Action:    "test-action",
		Result:    "success",
	}

	ctx := context.Background()
	if err := logger.LogEvent(ctx, event); err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatal("Log file was not created")
	}

	// Query the logged event
	events, err := logger.QueryEvents(ctx, EventFilter{
		EventType: "test.event",
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	loggedEvent := events[0]
	if loggedEvent.EventType != event.EventType {
		t.Errorf("Expected event type %s, got %s", event.EventType, loggedEvent.EventType)
	}
	if loggedEvent.Actor != event.Actor {
		t.Errorf("Expected actor %s, got %s", event.Actor, loggedEvent.Actor)
	}
	if loggedEvent.Signature == "" {
		t.Error("Event signature is empty")
	}
}

func TestAuditLogger_LogKeyGenerated(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	if err := logger.LogKeyGenerated(ctx, "test-user", "age", "test-cluster"); err != nil {
		t.Fatalf("Failed to log key generation: %v", err)
	}

	events, err := logger.QueryEvents(ctx, EventFilter{
		EventType: "key.generated",
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.EventType != "key.generated" {
		t.Errorf("Expected event type key.generated, got %s", event.EventType)
	}
	if event.Action != "generate" {
		t.Errorf("Expected action generate, got %s", event.Action)
	}
}

func TestAuditLogger_VerifyIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Log multiple events
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		event := AuditEvent{
			EventType: "test.event",
			Actor:     "test-user",
			Resource:  "test-resource",
			Action:    "test-action",
			Result:    "success",
		}
		if err := logger.LogEvent(ctx, event); err != nil {
			t.Fatalf("Failed to log event: %v", err)
		}
	}

	// Verify integrity
	if err := logger.VerifyIntegrity(); err != nil {
		t.Errorf("Integrity verification failed: %v", err)
	}
}

func TestAuditLogger_CredentialMasking(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Log event with credential in details
	ctx := context.Background()
	event := AuditEvent{
		EventType: "test.event",
		Actor:     "test-user",
		Resource:  "test-resource",
		Action:    "test-action",
		Result:    "success",
		Details: map[string]interface{}{
			"aws_key": "AKIAIOSFODNN7EXAMPLE",
		},
	}

	if err := logger.LogEvent(ctx, event); err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}

	// Query the event
	events, err := logger.QueryEvents(ctx, EventFilter{
		EventType: "test.event",
	})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	// Verify credential is masked
	loggedEvent := events[0]
	if awsKey, ok := loggedEvent.Details["aws_key"].(string); ok {
		if awsKey == "AKIAIOSFODNN7EXAMPLE" {
			t.Error("AWS key was not masked in audit log")
		}
	}
}

func TestAuditLogger_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create disabled logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Try to log an event
	ctx := context.Background()
	event := AuditEvent{
		EventType: "test.event",
		Actor:     "test-user",
		Resource:  "test-resource",
		Action:    "test-action",
		Result:    "success",
	}

	if err := logger.LogEvent(ctx, event); err != nil {
		t.Fatalf("Failed to log event: %v", err)
	}

	// Verify log file was not created
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("Log file was created even though logger is disabled")
	}
}
