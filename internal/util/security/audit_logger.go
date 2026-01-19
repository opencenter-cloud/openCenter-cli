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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/security"
)

// DefaultAuditLogger implements AuditLogger interface
type DefaultAuditLogger struct {
	logPath  string
	logLevel string
	masker   security.CredentialMasker
	mu       sync.Mutex
	file     *os.File
	enabled  bool
}

// AuditLoggerConfig represents configuration for audit logger
type AuditLoggerConfig struct {
	LogPath  string
	LogLevel string
	Enabled  bool
}

// NewDefaultAuditLogger creates a new audit logger
func NewDefaultAuditLogger(config AuditLoggerConfig) (*DefaultAuditLogger, error) {
	logger := &DefaultAuditLogger{
		logPath:  config.LogPath,
		logLevel: config.LogLevel,
		masker:   security.NewDefaultCredentialMasker(),
		enabled:  config.Enabled,
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
	}

	return logger, nil
}

// LogSecurityEvent logs a security event
func (l *DefaultAuditLogger) LogSecurityEvent(ctx context.Context, event SecurityEvent) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Mask sensitive data in event details
	if event.Details != nil {
		event.Details = l.maskMap(event.Details)
	}

	// Add context information if available
	if ctx != nil {
		if user, ok := ctx.Value("user").(string); ok && event.User == "" {
			event.User = user
		}
		if source, ok := ctx.Value("source").(string); ok && event.Source == "" {
			event.Source = source
		}
	}

	// Set default source if not provided
	if event.Source == "" {
		event.Source = "openCenter"
	}

	return l.writeEvent(event)
}

// maskMap masks sensitive data in a map using the credential masker
func (l *DefaultAuditLogger) maskMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	masked := make(map[string]interface{})
	for key, value := range data {
		switch v := value.(type) {
		case string:
			masked[key] = l.masker.MaskString(v)
		case map[string]interface{}:
			masked[key] = l.maskMap(v)
		case []interface{}:
			masked[key] = l.maskSlice(v)
		default:
			// For other types, convert to string, mask, and keep original type if possible
			str := fmt.Sprintf("%v", v)
			maskedStr := l.masker.MaskString(str)
			if str == maskedStr {
				// No masking occurred, keep original value
				masked[key] = v
			} else {
				// Masking occurred, use masked string
				masked[key] = maskedStr
			}
		}
	}
	return masked
}

// maskSlice masks sensitive data in a slice
func (l *DefaultAuditLogger) maskSlice(data []interface{}) []interface{} {
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
			// For other types, convert to string, mask, and keep original type if possible
			str := fmt.Sprintf("%v", v)
			maskedStr := l.masker.MaskString(str)
			if str == maskedStr {
				// No masking occurred, keep original value
				masked[i] = v
			} else {
				// Masking occurred, use masked string
				masked[i] = maskedStr
			}
		}
	}
	return masked
}

// LogSOPSOperation logs a SOPS-related operation
func (l *DefaultAuditLogger) LogSOPSOperation(ctx context.Context, operation, keyName string, success bool) error {
	event := SecurityEvent{
		Timestamp: time.Now(),
		EventType: "sops_operation",
		Operation: operation,
		Resource:  keyName,
		Success:   success,
		Severity:  "high",
		Details: map[string]interface{}{
			"operation": operation,
			"key_name":  keyName,
		},
	}

	return l.LogSecurityEvent(ctx, event)
}

// LogConfigChange logs a configuration change
func (l *DefaultAuditLogger) LogConfigChange(ctx context.Context, configPath, operation string, changes map[string]interface{}) error {
	event := SecurityEvent{
		Timestamp: time.Now(),
		EventType: "config_change",
		Operation: operation,
		Resource:  configPath,
		Success:   true,
		Severity:  "medium",
		Details: map[string]interface{}{
			"config_path": configPath,
			"operation":   operation,
			"changes":     changes,
		},
	}

	return l.LogSecurityEvent(ctx, event)
}

// LogCredentialAccess logs credential access
func (l *DefaultAuditLogger) LogCredentialAccess(ctx context.Context, credentialType, operation string) error {
	event := SecurityEvent{
		Timestamp: time.Now(),
		EventType: "credential_access",
		Operation: operation,
		Resource:  credentialType,
		Success:   true,
		Severity:  "high",
		Details: map[string]interface{}{
			"credential_type": credentialType,
			"operation":       operation,
		},
	}

	return l.LogSecurityEvent(ctx, event)
}

// LogAuthenticationAttempt logs an authentication attempt
func (l *DefaultAuditLogger) LogAuthenticationAttempt(ctx context.Context, user, method string, success bool) error {
	severity := "medium"
	if !success {
		severity = "high"
	}

	event := SecurityEvent{
		Timestamp: time.Now(),
		EventType: "authentication",
		Operation: "authenticate",
		User:      user,
		Success:   success,
		Severity:  severity,
		Details: map[string]interface{}{
			"user":   user,
			"method": method,
		},
	}

	return l.LogSecurityEvent(ctx, event)
}

// GetAuditLog retrieves audit log entries based on filter
func (l *DefaultAuditLogger) GetAuditLog(ctx context.Context, filter AuditFilter) ([]SecurityEvent, error) {
	if !l.enabled || l.logPath == "" {
		return nil, fmt.Errorf("audit logging is not enabled or log path not configured")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Read log file
	content, err := os.ReadFile(l.logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	// Parse log entries
	var events []SecurityEvent
	lines := splitLines(string(content))

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event SecurityEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip invalid entries
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

// writeEvent writes an event to the log file
func (l *DefaultAuditLogger) writeEvent(event SecurityEvent) error {
	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write to file if configured
	if l.file != nil {
		if _, err := l.file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write to audit log: %w", err)
		}

		// Sync to ensure data is written
		if err := l.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync audit log: %w", err)
		}
	}

	// Also write to stdout for debugging if log level is debug
	if l.logLevel == "debug" {
		fmt.Fprintf(os.Stdout, "[AUDIT] %s\n", string(data))
	}

	return nil
}

// Close closes the audit logger
func (l *DefaultAuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}

	return nil
}

// matchesFilter checks if an event matches the filter criteria
func matchesFilter(event SecurityEvent, filter AuditFilter) bool {
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

	// Check operation
	if filter.Operation != "" && event.Operation != filter.Operation {
		return false
	}

	// Check user
	if filter.User != "" && event.User != filter.User {
		return false
	}

	// Check resource
	if filter.Resource != "" && event.Resource != filter.Resource {
		return false
	}

	// Check success
	if filter.Success != nil && event.Success != *filter.Success {
		return false
	}

	// Check severity
	if filter.Severity != "" && event.Severity != filter.Severity {
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

// RotateAuditLog rotates the audit log file
func (l *DefaultAuditLogger) RotateAuditLog() error {
	if !l.enabled || l.logPath == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

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

	return nil
}

// GetDefaultAuditLogPath returns the default audit log path
func GetDefaultAuditLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/opencenter-audit.log"
	}

	return filepath.Join(homeDir, ".config", "openCenter", "audit", "audit.log")
}

// NewDefaultAuditLoggerWithDefaults creates an audit logger with default settings
func NewDefaultAuditLoggerWithDefaults() (*DefaultAuditLogger, error) {
	config := AuditLoggerConfig{
		LogPath:  GetDefaultAuditLogPath(),
		LogLevel: "info",
		Enabled:  true,
	}

	return NewDefaultAuditLogger(config)
}
