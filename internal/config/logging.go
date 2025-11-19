// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger is the global logger instance
var Logger *logrus.Logger

// LoggerManager manages the logging configuration and setup
type LoggerManager struct {
	config *LoggingConfig
	logger *logrus.Logger
}

// NewLoggerManager creates a new logger manager with the given configuration
func NewLoggerManager(config *LoggingConfig) (*LoggerManager, error) {
	lm := &LoggerManager{
		config: config,
		logger: logrus.New(),
	}

	if err := lm.Configure(); err != nil {
		return nil, fmt.Errorf("failed to configure logger: %w", err)
	}

	return lm, nil
}

// Configure configures the logger based on the logging configuration
func (lm *LoggerManager) Configure() error {
	// Set log level
	level, err := logrus.ParseLevel(lm.config.Level)
	if err != nil {
		return fmt.Errorf("invalid log level '%s': %w", lm.config.Level, err)
	}
	lm.logger.SetLevel(level)

	// Set log format
	switch strings.ToLower(lm.config.Format) {
	case "json":
		lm.logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	case "yaml":
		// Custom YAML formatter
		lm.logger.SetFormatter(&YAMLFormatter{})
	case "text":
		lm.logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		return fmt.Errorf("invalid log format '%s', must be one of: text, json, yaml", lm.config.Format)
	}

	// Set output destination
	output, err := lm.getOutputWriter()
	if err != nil {
		return fmt.Errorf("failed to configure output: %w", err)
	}
	lm.logger.SetOutput(output)

	// Set the global logger
	Logger = lm.logger

	return nil
}

// getOutputWriter returns the appropriate writer based on the output configuration
func (lm *LoggerManager) getOutputWriter() (io.Writer, error) {
	switch strings.ToLower(lm.config.Output) {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	case "":
		// Default to stderr if not specified
		return os.Stderr, nil
	default:
		// Treat as file path
		return lm.getFileWriter(lm.config.Output)
	}
}

// getFileWriter creates a file writer with rotation support
func (lm *LoggerManager) getFileWriter(filePath string) (io.Writer, error) {
	// Expand the path
	expandedPath := ExpandPath(filePath)

	// Ensure the directory exists
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", dir, err)
	}

	// Create lumberjack logger for rotation
	return &lumberjack.Logger{
		Filename:   expandedPath,
		MaxSize:    lm.config.File.MaxSize,    // megabytes
		MaxBackups: lm.config.File.MaxBackups, // number of backups
		MaxAge:     lm.config.File.MaxAge,     // days
		Compress:   lm.config.File.Compress,   // compress rotated files
	}, nil
}

// GetLogger returns the configured logger instance
func (lm *LoggerManager) GetLogger() *logrus.Logger {
	return lm.logger
}

// Reconfigure reconfigures the logger with new settings
func (lm *LoggerManager) Reconfigure(config *LoggingConfig) error {
	lm.config = config
	return lm.Configure()
}

// YAMLFormatter is a custom formatter that outputs logs in YAML format
type YAMLFormatter struct{}

// Format formats the log entry as YAML
func (f *YAMLFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var output strings.Builder

	// Write timestamp
	output.WriteString(fmt.Sprintf("timestamp: %s\n", entry.Time.Format("2006-01-02T15:04:05.000Z07:00")))

	// Write level
	output.WriteString(fmt.Sprintf("level: %s\n", entry.Level.String()))

	// Write message
	output.WriteString(fmt.Sprintf("message: %q\n", entry.Message))

	// Write fields if any
	if len(entry.Data) > 0 {
		output.WriteString("fields:\n")
		for key, value := range entry.Data {
			output.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
	}

	output.WriteString("---\n")

	return []byte(output.String()), nil
}

// InitializeLogging initializes the global logging system with the given configuration
func InitializeLogging(config *LoggingConfig) error {
	loggerManager, err := NewLoggerManager(config)
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}

	// Set the global logger
	Logger = loggerManager.GetLogger()

	return nil
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *logrus.Logger {
	if Logger == nil {
		// Initialize with default configuration if not already initialized
		defaultConfig := DefaultCLIConfig()
		if err := InitializeLogging(&defaultConfig.Logging); err != nil {
			// Fallback to basic logger
			Logger = logrus.New()
			Logger.SetLevel(logrus.WarnLevel)
		}
	}
	return Logger
}

// ValidateLoggingConfig validates the logging configuration
func ValidateLoggingConfig(config *LoggingConfig) error {
	// Validate log level
	if _, err := logrus.ParseLevel(config.Level); err != nil {
		return fmt.Errorf("invalid log level '%s': %w", config.Level, err)
	}

	// Validate log format
	validFormats := []string{"text", "json", "yaml"}
	if !contains(validFormats, strings.ToLower(config.Format)) {
		return fmt.Errorf("invalid log format '%s', must be one of: %s", config.Format, strings.Join(validFormats, ", "))
	}

	// Validate output
	validOutputs := []string{"stdout", "stderr"}
	if !contains(validOutputs, strings.ToLower(config.Output)) && config.Output != "" {
		// If it's not a standard output, validate as file path
		expandedPath := ExpandPath(config.Output)
		dir := filepath.Dir(expandedPath)

		// Check if directory exists or can be created
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create log directory %s: %w", dir, err)
		}

		// Try to create/open the file
		file, err := os.OpenFile(expandedPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("cannot write to log file %s: %w", expandedPath, err)
		}
		file.Close()
	}

	// Validate file configuration
	if config.File.MaxSize <= 0 {
		return fmt.Errorf("file.maxSize must be greater than 0, got %d", config.File.MaxSize)
	}

	if config.File.MaxBackups < 0 {
		return fmt.Errorf("file.maxBackups must be greater than or equal to 0, got %d", config.File.MaxBackups)
	}

	if config.File.MaxAge < 0 {
		return fmt.Errorf("file.maxAge must be greater than or equal to 0, got %d", config.File.MaxAge)
	}

	return nil
}

// SetLogLevel sets the log level for the global logger
func SetLogLevel(level string) error {
	logger := GetGlobalLogger()

	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level '%s': %w", level, err)
	}

	logger.SetLevel(parsedLevel)
	return nil
}

// SetLogFormat sets the log format for the global logger
func SetLogFormat(format string) error {
	logger := GetGlobalLogger()

	switch strings.ToLower(format) {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	case "yaml":
		logger.SetFormatter(&YAMLFormatter{})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		return fmt.Errorf("invalid log format '%s', must be one of: text, json, yaml", format)
	}

	return nil
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	GetGlobalLogger().Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	GetGlobalLogger().Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	GetGlobalLogger().Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	GetGlobalLogger().Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	GetGlobalLogger().Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	GetGlobalLogger().Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	GetGlobalLogger().Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	GetGlobalLogger().Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	GetGlobalLogger().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	GetGlobalLogger().Fatalf(format, args...)
}

// WithField creates a logger with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return GetGlobalLogger().WithField(key, value)
}

// WithFields creates a logger with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetGlobalLogger().WithFields(fields)
}
