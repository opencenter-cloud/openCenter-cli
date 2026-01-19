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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/security"
)

// Logger provides structured logging with credential masking
// Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7, 12.8
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	WithFields(fields ...Field) Logger
	WithCorrelationID(id string) Logger
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value interface{}
}

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogFormat represents the output format for log messages
type LogFormat int

const (
	// TextFormat outputs logs in human-readable text format
	TextFormat LogFormat = iota
	// JSONFormat outputs logs in structured JSON format
	JSONFormat
)

// DefaultLogger implements Logger interface with credential masking
type DefaultLogger struct {
	level         LogLevel
	format        LogFormat
	output        io.Writer
	masker        security.CredentialMasker
	fields        []Field
	correlationID string
	shipper       LogShipper
	mu            sync.Mutex
}

// LoggerConfig represents configuration for the logger
type LoggerConfig struct {
	Level   LogLevel
	Format  LogFormat
	Output  io.Writer
	Shipper LogShipper
}

// LogEntry represents a structured log entry for JSON output
// Requirements: 12.2, 12.3
type LogEntry struct {
	Timestamp     string                 `json:"timestamp"`
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
}

// NewDefaultLogger creates a new logger with credential masking
func NewDefaultLogger(config LoggerConfig) *DefaultLogger {
	output := config.Output
	if output == nil {
		output = os.Stdout
	}

	return &DefaultLogger{
		level:   config.Level,
		format:  config.Format,
		output:  output,
		masker:  security.NewDefaultCredentialMasker(),
		fields:  make([]Field, 0),
		shipper: config.Shipper,
	}
}

// NewDefaultLoggerWithDefaults creates a logger with default settings
func NewDefaultLoggerWithDefaults() *DefaultLogger {
	return NewDefaultLogger(LoggerConfig{
		Level:   InfoLevel,
		Format:  TextFormat,
		Output:  os.Stdout,
		Shipper: nil,
	})
}

// Debug logs a debug message
// Requirements: 12.2, 12.4, 12.8
func (l *DefaultLogger) Debug(msg string, fields ...Field) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, msg, fields...)
	}
}

// Info logs an info message
// Requirements: 12.2, 12.4, 12.8
func (l *DefaultLogger) Info(msg string, fields ...Field) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, msg, fields...)
	}
}

// Warn logs a warning message
// Requirements: 12.2, 12.4, 12.8
func (l *DefaultLogger) Warn(msg string, fields ...Field) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, msg, fields...)
	}
}

// Error logs an error message
// Requirements: 12.2, 12.4, 12.8
func (l *DefaultLogger) Error(msg string, fields ...Field) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, msg, fields...)
	}
}

// WithFields returns a new logger with additional fields
// Requirements: 12.2, 12.3
func (l *DefaultLogger) WithFields(fields ...Field) Logger {
	newLogger := &DefaultLogger{
		level:         l.level,
		format:        l.format,
		output:        l.output,
		masker:        l.masker,
		fields:        make([]Field, len(l.fields)+len(fields)),
		correlationID: l.correlationID,
		shipper:       l.shipper,
	}

	copy(newLogger.fields, l.fields)
	copy(newLogger.fields[len(l.fields):], fields)

	return newLogger
}

// WithCorrelationID returns a new logger with a correlation ID
// Requirements: 12.3
func (l *DefaultLogger) WithCorrelationID(id string) Logger {
	newLogger := &DefaultLogger{
		level:         l.level,
		format:        l.format,
		output:        l.output,
		masker:        l.masker,
		fields:        make([]Field, len(l.fields)),
		correlationID: id,
		shipper:       l.shipper,
	}

	copy(newLogger.fields, l.fields)

	return newLogger
}

// log writes a log entry with credential masking
// Requirements: 12.2, 12.3, 12.4, 12.8
func (l *DefaultLogger) log(level LogLevel, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Mask the message
	maskedMsg := l.masker.MaskString(msg)

	// Combine persistent fields with additional fields
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	// Format based on configured format
	if l.format == JSONFormat {
		l.logJSON(level, maskedMsg, allFields)
	} else {
		l.logText(level, maskedMsg, allFields)
	}
}

// logJSON writes a log entry in JSON format
// Requirements: 12.2, 12.3, 12.8
func (l *DefaultLogger) logJSON(level LogLevel, msg string, fields []Field) {
	entry := LogEntry{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Level:         level.String(),
		Message:       msg,
		CorrelationID: l.correlationID,
	}

	// Add fields if present
	if len(fields) > 0 {
		entry.Fields = make(map[string]interface{})
		for _, field := range fields {
			// Mask field values
			maskedValue := l.maskFieldValue(field.Value)
			entry.Fields[field.Key] = maskedValue
		}
	}

	// Ship to external system if configured
	if l.shipper != nil {
		// Ship asynchronously to avoid blocking
		go func() {
			_ = l.shipper.Ship(entry)
		}()
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		// Fallback to text format if JSON marshaling fails
		l.logText(level, msg, fields)
		return
	}

	// Write JSON with newline
	_, _ = l.output.Write(jsonBytes)
	_, _ = l.output.Write([]byte("\n"))
}

// logText writes a log entry in text format
// Requirements: 12.2, 12.3, 12.8
func (l *DefaultLogger) logText(level LogLevel, msg string, fields []Field) {
	// Build log entry
	timestamp := time.Now().UTC().Format(time.RFC3339)
	entry := fmt.Sprintf("[%s] %s: %s", timestamp, level.String(), msg)

	// Add correlation ID if present
	if l.correlationID != "" {
		entry += fmt.Sprintf(" [correlation_id=%s]", l.correlationID)
	}

	// Add fields
	if len(fields) > 0 {
		entry += " "
		for i, field := range fields {
			if i > 0 {
				entry += " "
			}
			// Mask field values
			maskedValue := l.maskFieldValue(field.Value)
			entry += fmt.Sprintf("%s=%v", field.Key, maskedValue)
		}
	}

	entry += "\n"

	// Write to output
	_, _ = l.output.Write([]byte(entry))
}

// maskFieldValue masks a field value based on its type
func (l *DefaultLogger) maskFieldValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return l.masker.MaskString(v)
	case error:
		if v != nil {
			return l.masker.MaskString(v.Error())
		}
		return nil
	default:
		// For other types, convert to string and mask
		str := fmt.Sprintf("%v", v)
		return l.masker.MaskString(str)
	}
}

// SetLevel sets the log level
func (l *DefaultLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current log level
func (l *DefaultLogger) GetLevel() LogLevel {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// SetFormat sets the log format
func (l *DefaultLogger) SetFormat(format LogFormat) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.format = format
}

// GetFormat returns the current log format
func (l *DefaultLogger) GetFormat() LogFormat {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.format
}

// SetShipper sets the log shipper
func (l *DefaultLogger) SetShipper(shipper LogShipper) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.shipper = shipper
}

// Close closes the logger and any associated shippers
func (l *DefaultLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.shipper != nil {
		return l.shipper.Close()
	}
	return nil
}
