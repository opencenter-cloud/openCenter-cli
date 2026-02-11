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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

const (
	// MaxLogSize is the maximum size of a log file before rotation (100MB)
	MaxLogSize = 100 * 1024 * 1024
	// LogRetentionDays is the number of days to retain rotated logs
	LogRetentionDays = 30
)

// AuditEvent represents a security-relevant event with integrity protection
type AuditEvent struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	EventType     string                 `json:"event_type"`
	Actor         string                 `json:"actor"`
	Resource      string                 `json:"resource"`
	Action        string                 `json:"action"`
	Result        string                 `json:"result"`
	Details       map[string]interface{} `json:"details,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Signature     string                 `json:"signature"`
}

// AuditLogger provides tamper-evident audit logging for security events
type AuditLogger struct {
	logPath     string
	signingKey  []byte
	masker      *DefaultCredentialMasker
	fileSystem  fs.FileSystem
	mu          sync.Mutex
	file        *os.File
	enabled     bool
	currentSize int64
}

// AuditLoggerConfig represents configuration for audit logger
type AuditLoggerConfig struct {
	LogPath    string
	SigningKey []byte
	Enabled    bool
}

// NewAuditLogger creates a new audit logger with HMAC integrity protection
func NewAuditLogger(config AuditLoggerConfig) (*AuditLogger, error) {
	// Generate signing key if not provided
	signingKey := config.SigningKey
	if len(signingKey) == 0 {
		signingKey = make([]byte, 32)
		if _, err := rand.Read(signingKey); err != nil {
			return nil, fmt.Errorf("failed to generate signing key: %w", err)
		}
	}

	// Create FileSystem with error handler
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)

	logger := &AuditLogger{
		logPath:    config.LogPath,
		signingKey: signingKey,
		masker:     NewDefaultCredentialMasker(),
		fileSystem: fileSystem,
		enabled:    config.Enabled,
	}

	if !logger.enabled {
		return logger, nil
	}

	// Create log directory if it doesn't exist
	if logger.logPath != "" {
		logDir := filepath.Dir(logger.logPath)
		if err := os.MkdirAll(logDir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %w", err)
		}

		// Open log file with append mode and secure permissions
		file, err := os.OpenFile(logger.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}
		logger.file = file

		// Get current file size
		info, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to stat audit log file: %w", err)
		}
		logger.currentSize = info.Size()
	}

	return logger, nil
}

// LogEvent logs a security event with HMAC signature
func (l *AuditLogger) LogEvent(ctx context.Context, event AuditEvent) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Generate event ID if not provided
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Extract correlation ID from context if available
	if event.CorrelationID == "" {
		if correlationID, ok := ctx.Value("correlation_id").(string); ok {
			event.CorrelationID = correlationID
		}
	}

	// Mask sensitive data in event details
	if event.Details != nil {
		event.Details = l.maskMap(event.Details)
	}

	// Sign the event for integrity protection
	if err := l.signEvent(&event); err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}

	// Write event to log
	if err := l.writeEvent(event); err != nil {
		return err
	}

	// Check if rotation is needed
	if l.currentSize >= MaxLogSize {
		if err := l.rotateLog(); err != nil {
			// Log rotation failure shouldn't stop the operation
			fmt.Fprintf(os.Stderr, "Warning: failed to rotate audit log: %v\n", err)
		}
	}

	return nil
}

// LogKeyGenerated logs a key generation event
func (l *AuditLogger) LogKeyGenerated(ctx context.Context, actor, keyType, resource string) error {
	event := AuditEvent{
		EventType: "key.generated",
		Actor:     actor,
		Resource:  resource,
		Action:    "generate",
		Result:    "success",
		Details: map[string]interface{}{
			"key_type": keyType,
		},
	}
	return l.LogEvent(ctx, event)
}

// LogKeyAccessed logs a key access event
func (l *AuditLogger) LogKeyAccessed(ctx context.Context, actor, keyType, resource string, success bool) error {
	result := "success"
	if !success {
		result = "failure"
	}

	event := AuditEvent{
		EventType: "key.accessed",
		Actor:     actor,
		Resource:  resource,
		Action:    "access",
		Result:    result,
		Details: map[string]interface{}{
			"key_type": keyType,
		},
	}
	return l.LogEvent(ctx, event)
}

// LogKeyRotated logs a key rotation event
func (l *AuditLogger) LogKeyRotated(ctx context.Context, actor, keyType, resource string) error {
	event := AuditEvent{
		EventType: "key.rotated",
		Actor:     actor,
		Resource:  resource,
		Action:    "rotate",
		Result:    "success",
		Details: map[string]interface{}{
			"key_type": keyType,
		},
	}
	return l.LogEvent(ctx, event)
}

// LogValidationFailed logs a validation failure event
func (l *AuditLogger) LogValidationFailed(ctx context.Context, actor, resource, reason string, details map[string]interface{}) error {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["reason"] = reason

	event := AuditEvent{
		EventType: "validation.failed",
		Actor:     actor,
		Resource:  resource,
		Action:    "validate",
		Result:    "failure",
		Details:   details,
	}
	return l.LogEvent(ctx, event)
}

// LogInputRejected logs a rejected input event
func (l *AuditLogger) LogInputRejected(ctx context.Context, actor, inputType, reason string) error {
	event := AuditEvent{
		EventType: "input.rejected",
		Actor:     actor,
		Resource:  inputType,
		Action:    "validate",
		Result:    "rejected",
		Details: map[string]interface{}{
			"input_type": inputType,
			"reason":     reason,
		},
	}
	return l.LogEvent(ctx, event)
}

// LogTemplateValidationFailed logs a template validation failure
func (l *AuditLogger) LogTemplateValidationFailed(ctx context.Context, actor, templateName, reason string) error {
	event := AuditEvent{
		EventType: "template.validation.failed",
		Actor:     actor,
		Resource:  templateName,
		Action:    "validate",
		Result:    "failure",
		Details: map[string]interface{}{
			"template": templateName,
			"reason":   reason,
		},
	}
	return l.LogEvent(ctx, event)
}

// QueryEvents retrieves audit log entries based on filter
func (l *AuditLogger) QueryEvents(ctx context.Context, filter EventFilter) ([]AuditEvent, error) {
	if !l.enabled || l.logPath == "" {
		return nil, fmt.Errorf("audit logging is not enabled or log path not configured")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Read log file using FileSystem
	content, err := l.fileSystem.ReadFile(l.logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	// Parse log entries
	var events []AuditEvent
	lines := splitLines(string(content))

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip invalid entries
		}

		// Verify signature
		if !l.verifySignature(&event) {
			// Log tampering detected but continue processing
			fmt.Fprintf(os.Stderr, "Warning: signature verification failed for event %s\n", event.ID)
		}

		// Apply filters
		if !matchesFilter(event, filter) {
			continue
		}

		events = append(events, event)

		// Apply limit if specified
		if filter.Limit > 0 && len(events) >= filter.Limit {
			break
		}
	}

	return events, nil
}

// VerifyIntegrity verifies the integrity of all audit log entries
func (l *AuditLogger) VerifyIntegrity() error {
	if !l.enabled || l.logPath == "" {
		return fmt.Errorf("audit logging is not enabled or log path not configured")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Read log file using FileSystem
	content, err := l.fileSystem.ReadFile(l.logPath)
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	lines := splitLines(string(content))
	invalidCount := 0

	for i, line := range lines {
		if line == "" {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("failed to parse event at line %d: %w", i+1, err)
		}

		if !l.verifySignature(&event) {
			invalidCount++
			fmt.Fprintf(os.Stderr, "Integrity check failed for event %s at line %d\n", event.ID, i+1)
		}
	}

	if invalidCount > 0 {
		return fmt.Errorf("integrity verification failed: %d events have invalid signatures", invalidCount)
	}

	return nil
}

// Close closes the audit logger
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}

	return nil
}

// signEvent generates an HMAC signature for the event
func (l *AuditLogger) signEvent(event *AuditEvent) error {
	// Create signature data from event fields
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		event.Timestamp.Format(time.RFC3339Nano),
		event.EventType,
		event.Actor,
		event.Resource,
		event.Action,
		event.Result)

	// Generate HMAC signature
	mac := hmac.New(sha256.New, l.signingKey)
	mac.Write([]byte(data))
	event.Signature = hex.EncodeToString(mac.Sum(nil))

	return nil
}

// verifySignature verifies the HMAC signature of an event
func (l *AuditLogger) verifySignature(event *AuditEvent) bool {
	// Recreate signature data
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		event.Timestamp.Format(time.RFC3339Nano),
		event.EventType,
		event.Actor,
		event.Resource,
		event.Action,
		event.Result)

	// Generate expected signature
	mac := hmac.New(sha256.New, l.signingKey)
	mac.Write([]byte(data))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	return hmac.Equal([]byte(event.Signature), []byte(expectedSig))
}

// writeEvent writes an event to the log file
func (l *AuditLogger) writeEvent(event AuditEvent) error {
	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write to file if configured
	if l.file != nil {
		n, err := l.file.Write(append(data, '\n'))
		if err != nil {
			return fmt.Errorf("failed to write to audit log: %w", err)
		}

		// Update current size
		l.currentSize += int64(n)

		// Sync to ensure data is written
		if err := l.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync audit log: %w", err)
		}
	}

	return nil
}

// rotateLog rotates the audit log file
func (l *AuditLogger) rotateLog() error {
	// Close current file
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("failed to close audit log: %w", err)
		}
	}

	// Rename current log file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", l.logPath, timestamp)

	if err := os.Rename(l.logPath, rotatedPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to rotate audit log: %w", err)
		}
	}

	// Open new log file
	file, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open new audit log: %w", err)
	}
	l.file = file
	l.currentSize = 0

	// Clean up old logs
	if err := l.cleanupOldLogs(); err != nil {
		// Log cleanup failure shouldn't stop the operation
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old logs: %v\n", err)
	}

	return nil
}

// cleanupOldLogs removes audit logs older than retention period
func (l *AuditLogger) cleanupOldLogs() error {
	logDir := filepath.Dir(l.logPath)
	logBase := filepath.Base(l.logPath)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -LogRetentionDays)

	for _, entry := range entries {
		// Check if this is a rotated log file
		if !entry.IsDir() && len(entry.Name()) > len(logBase) && entry.Name()[:len(logBase)] == logBase {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Delete if older than retention period
			if info.ModTime().Before(cutoff) {
				logPath := filepath.Join(logDir, entry.Name())
				if err := os.Remove(logPath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove old log %s: %v\n", logPath, err)
				}
			}
		}
	}

	return nil
}

// maskMap masks sensitive data in a map using the credential masker
func (l *AuditLogger) maskMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	// List of sensitive field names that should always be masked
	sensitiveFields := map[string]bool{
		"password":                      true,
		"passwd":                        true,
		"pwd":                           true,
		"secret":                        true,
		"token":                         true,
		"api_key":                       true,
		"api-key":                       true,
		"apikey":                        true,
		"private_key":                   true,
		"private-key":                   true,
		"privatekey":                    true,
		"age_key":                       true,
		"age-key":                       true,
		"agekey":                        true,
		"aws_access_key_id":             true,
		"aws_secret_access_key":         true,
		"application_credential_secret": true,
		"application-credential-secret": true,
		"bearer":                        true,
		"authorization":                 true,
	}

	masked := make(map[string]interface{})
	for key, value := range data {
		// Check if this is a sensitive field name
		lowerKey := strings.ToLower(key)
		isSensitive := sensitiveFields[lowerKey]

		switch v := value.(type) {
		case string:
			if isSensitive && v != "" {
				// Mask the entire value for sensitive fields
				masked[key] = "***MASKED***"
			} else {
				// Otherwise use pattern-based masking
				masked[key] = l.masker.MaskString(v)
			}
		case map[string]interface{}:
			masked[key] = l.maskMap(v)
		case []interface{}:
			masked[key] = l.maskSlice(v)
		default:
			masked[key] = v
		}
	}
	return masked
}

// maskSlice masks sensitive data in a slice
func (l *AuditLogger) maskSlice(data []interface{}) []interface{} {
	if data == nil {
		return nil
	}

	masked := make([]interface{}, len(data))
	for i, item := range data {
		switch v := item.(type) {
		case string:
			masked[i] = l.masker.MaskString(v)
		case map[string]interface{}:
			masked[i] = l.maskMap(v)
		case []interface{}:
			masked[i] = l.maskSlice(v)
		default:
			masked[i] = v
		}
	}
	return masked
}

// EventFilter represents filters for querying audit logs
type EventFilter struct {
	StartTime     time.Time
	EndTime       time.Time
	EventType     string
	Actor         string
	Resource      string
	Action        string
	Result        string
	CorrelationID string
	Limit         int
}

// matchesFilter checks if an event matches the filter criteria
func matchesFilter(event AuditEvent, filter EventFilter) bool {
	// Check time range
	if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
		return false
	}

	// Check event type
	if filter.EventType != "" && event.EventType != filter.EventType {
		return false
	}

	// Check actor
	if filter.Actor != "" && event.Actor != filter.Actor {
		return false
	}

	// Check resource
	if filter.Resource != "" && event.Resource != filter.Resource {
		return false
	}

	// Check action
	if filter.Action != "" && event.Action != filter.Action {
		return false
	}

	// Check result
	if filter.Result != "" && event.Result != filter.Result {
		return false
	}

	// Check correlation ID
	if filter.CorrelationID != "" && event.CorrelationID != filter.CorrelationID {
		return false
	}

	return true
}

// splitLines splits content into lines
func splitLines(content string) []string {
	var lines []string
	var current string

	for _, char := range content {
		if char == '\n' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

// generateEventID generates a unique event ID
func generateEventID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetDefaultAuditLogPath returns the default audit log path
func GetDefaultAuditLogPath() string {
	configDir := config.GetConfigDir()
	return filepath.Join(configDir, "audit", "audit.log")
}

// NewDefaultAuditLogger creates an audit logger with default settings
func NewDefaultAuditLogger() (*AuditLogger, error) {
	config := AuditLoggerConfig{
		LogPath: GetDefaultAuditLogPath(),
		Enabled: true,
	}

	return NewAuditLogger(config)
}
