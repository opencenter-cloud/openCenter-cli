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
	"context"
	"os"
)

// GlobalLogger is the global logger instance for the application
// This replaces the logrus global logger
var GlobalLogger Logger

// InitializeGlobalLogger initializes the global logger with the given configuration
func InitializeGlobalLogger(config LoggerConfig) {
	GlobalLogger = NewDefaultLogger(config)
}

// GetGlobalLogger returns the global logger instance
// If not initialized, it creates a default logger
func GetGlobalLogger() Logger {
	if GlobalLogger == nil {
		GlobalLogger = NewDefaultLoggerWithDefaults()
	}
	return GlobalLogger
}

// WithField creates a logger with a single field
// This provides compatibility with logrus-style logging
func WithField(key string, value interface{}) Logger {
	return GetGlobalLogger().WithFields(Field{Key: key, Value: value})
}

// WithFields creates a logger with multiple fields
// This provides compatibility with logrus-style logging
func WithFields(fields map[string]interface{}) Logger {
	logFields := make([]Field, 0, len(fields))
	for k, v := range fields {
		logFields = append(logFields, Field{Key: k, Value: v})
	}
	return GetGlobalLogger().WithFields(logFields...)
}

// Debug logs a debug message using the global logger
func Debug(msg string, fields ...Field) {
	GetGlobalLogger().Debug(msg, fields...)
}

// Info logs an info message using the global logger
func Info(msg string, fields ...Field) {
	GetGlobalLogger().Info(msg, fields...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, fields ...Field) {
	GetGlobalLogger().Warn(msg, fields...)
}

// Error logs an error message using the global logger
func Error(msg string, fields ...Field) {
	GetGlobalLogger().Error(msg, fields...)
}

// ContextKey is the type for context keys
type ContextKey string

const (
	// CorrelationIDKey is the context key for correlation IDs
	CorrelationIDKey ContextKey = "correlation_id"
	// ClusterKey is the context key for cluster name
	ClusterKey ContextKey = "cluster"
	// OperationKey is the context key for operation name
	OperationKey ContextKey = "operation"
)

// FromContext creates a logger from context values
// It extracts correlation ID and other fields from the context
func FromContext(ctx context.Context) Logger {
	logger := GetGlobalLogger()

	// Extract correlation ID
	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok && correlationID != "" {
		logger = logger.WithCorrelationID(correlationID)
	}

	// Extract other common fields
	fields := make([]Field, 0)
	if cluster, ok := ctx.Value(ClusterKey).(string); ok && cluster != "" {
		fields = append(fields, Field{Key: "cluster", Value: cluster})
	}
	if operation, ok := ctx.Value(OperationKey).(string); ok && operation != "" {
		fields = append(fields, Field{Key: "operation", Value: operation})
	}

	if len(fields) > 0 {
		logger = logger.WithFields(fields...)
	}

	return logger
}

// SetGlobalLogLevel sets the log level for the global logger
func SetGlobalLogLevel(level LogLevel) {
	if dl, ok := GlobalLogger.(*DefaultLogger); ok {
		dl.SetLevel(level)
	}
}

// SetGlobalLogFormat sets the log format for the global logger
func SetGlobalLogFormat(format LogFormat) {
	if dl, ok := GlobalLogger.(*DefaultLogger); ok {
		dl.SetFormat(format)
	}
}

// SetGlobalLogShipper sets the log shipper for the global logger
func SetGlobalLogShipper(shipper LogShipper) {
	if dl, ok := GlobalLogger.(*DefaultLogger); ok {
		dl.SetShipper(shipper)
	}
}

// CloseGlobalLogger closes the global logger and any associated shippers
func CloseGlobalLogger() error {
	if dl, ok := GlobalLogger.(*DefaultLogger); ok {
		return dl.Close()
	}
	return nil
}

// ParseLogLevel parses a log level string
func ParseLogLevel(level string) (LogLevel, error) {
	switch level {
	case "debug", "DEBUG":
		return DebugLevel, nil
	case "info", "INFO":
		return InfoLevel, nil
	case "warn", "warning", "WARN", "WARNING":
		return WarnLevel, nil
	case "error", "ERROR":
		return ErrorLevel, nil
	default:
		return InfoLevel, nil
	}
}

// InitializeLoggingFromConfig initializes logging from a configuration
func InitializeLoggingFromConfig(level, format string, shipper LogShipper) error {
	// Parse log level
	logLevel, err := ParseLogLevel(level)
	if err != nil {
		logLevel = InfoLevel
	}

	// Parse log format
	var logFormat LogFormat
	switch format {
	case "json", "JSON":
		logFormat = JSONFormat
	default:
		logFormat = TextFormat
	}

	// Initialize global logger
	InitializeGlobalLogger(LoggerConfig{
		Level:   logLevel,
		Format:  logFormat,
		Output:  os.Stdout,
		Shipper: shipper,
	})

	return nil
}
