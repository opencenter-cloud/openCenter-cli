// Package logging provides the global logging system for opencenter-cli.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	corePaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger   *logrus.Logger
	loggerMu sync.RWMutex
)

// LoggingConfig controls logging behavior including level, format, output, and file rotation.
type LoggingConfig struct {
	Level  string     `yaml:"level"`
	Format string     `yaml:"format"`
	Output string     `yaml:"output"`
	File   FileConfig `yaml:"file"`
}

// FileConfig controls file logging rotation settings.
type FileConfig struct {
	MaxSize    int  `yaml:"maxSize"`
	MaxBackups int  `yaml:"maxBackups"`
	MaxAge     int  `yaml:"maxAge"`
	Compress   bool `yaml:"compress"`
}

// LoggerManager manages the logging configuration and setup.
type LoggerManager struct {
	config *LoggingConfig
	logger *logrus.Logger
}

// YAMLFormatter is a custom formatter that outputs logs in YAML format.
type YAMLFormatter struct{}

// Format formats the log entry as YAML.
func (f *YAMLFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var output strings.Builder
	fmt.Fprintf(&output, "timestamp: %s\n", entry.Time.Format("2006-01-02T15:04:05.000Z07:00"))
	fmt.Fprintf(&output, "level: %s\n", entry.Level.String())
	fmt.Fprintf(&output, "message: %q\n", entry.Message)
	if len(entry.Data) > 0 {
		output.WriteString("fields:\n")
		for key, value := range entry.Data {
			fmt.Fprintf(&output, "  %s: %v\n", key, value)
		}
	}
	output.WriteString("---\n")
	return []byte(output.String()), nil
}

// NewLoggerManager creates a new logger manager with the given configuration.
func NewLoggerManager(config *LoggingConfig) (*LoggerManager, error) {
	lm := &LoggerManager{config: config, logger: logrus.New()}
	if err := lm.Configure(); err != nil {
		return nil, fmt.Errorf("failed to configure logger: %w", err)
	}
	return lm, nil
}

// Configure configures the logger based on the logging configuration.
func (lm *LoggerManager) Configure() error {
	level, err := logrus.ParseLevel(lm.config.Level)
	if err != nil {
		return fmt.Errorf("invalid log level '%s': %w", lm.config.Level, err)
	}
	lm.logger.SetLevel(level)

	switch strings.ToLower(lm.config.Format) {
	case "json":
		lm.logger.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02T15:04:05.000Z07:00"})
	case "yaml":
		lm.logger.SetFormatter(&YAMLFormatter{})
	case "text":
		lm.logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02T15:04:05.000Z07:00"})
	default:
		return fmt.Errorf("invalid log format '%s', must be one of: text, json, yaml", lm.config.Format)
	}

	output, err := lm.getOutputWriter()
	if err != nil {
		return fmt.Errorf("failed to configure output: %w", err)
	}
	lm.logger.SetOutput(output)
	SetGlobalLogger(lm.logger)
	return nil
}

func (lm *LoggerManager) getOutputWriter() (io.Writer, error) {
	switch strings.ToLower(lm.config.Output) {
	case "stdout":
		return os.Stdout, nil
	case "stderr", "":
		return os.Stderr, nil
	default:
		return lm.getFileWriter(lm.config.Output)
	}
}

func (lm *LoggerManager) getFileWriter(filePath string) (io.Writer, error) {
	expandedPath := corePaths.ExpandPath(filePath)
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", dir, err)
	}
	return &lumberjack.Logger{
		Filename:   expandedPath,
		MaxSize:    lm.config.File.MaxSize,
		MaxBackups: lm.config.File.MaxBackups,
		MaxAge:     lm.config.File.MaxAge,
		Compress:   lm.config.File.Compress,
	}, nil
}

// GetLogger returns the configured logger instance.
func (lm *LoggerManager) GetLogger() *logrus.Logger { return lm.logger }

// Reconfigure reconfigures the logger with new settings.
func (lm *LoggerManager) Reconfigure(config *LoggingConfig) error {
	lm.config = config
	return lm.Configure()
}

// Initialize initializes the global logging system with the given configuration.
func Initialize(config *LoggingConfig) error {
	lm, err := NewLoggerManager(config)
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}
	SetGlobalLogger(lm.GetLogger())
	return nil
}

// GetGlobalLogger returns the global logger instance.
func GetGlobalLogger() *logrus.Logger {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()
	if l != nil {
		return l
	}
	fallback := logrus.New()
	fallback.SetLevel(logrus.WarnLevel)
	SetGlobalLogger(fallback)
	return fallback
}

// SetGlobalLogger sets the global logger instance.
func SetGlobalLogger(l *logrus.Logger) {
	loggerMu.Lock()
	logger = l
	loggerMu.Unlock()
}

// SetLogLevel sets the log level for the global logger.
func SetLogLevel(level string) error {
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level '%s': %w", level, err)
	}
	GetGlobalLogger().SetLevel(parsedLevel)
	return nil
}

// SetLogFormat sets the log format for the global logger.
func SetLogFormat(format string) error {
	l := GetGlobalLogger()
	switch strings.ToLower(format) {
	case "json":
		l.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02T15:04:05.000Z07:00"})
	case "yaml":
		l.SetFormatter(&YAMLFormatter{})
	case "text":
		l.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02T15:04:05.000Z07:00"})
	default:
		return fmt.Errorf("invalid log format '%s', must be one of: text, json, yaml", format)
	}
	return nil
}

// ValidateLoggingConfig validates the logging configuration.
func ValidateLoggingConfig(config *LoggingConfig) error {
	if _, err := logrus.ParseLevel(config.Level); err != nil {
		return fmt.Errorf("invalid log level '%s': %w", config.Level, err)
	}
	validFormats := []string{"text", "json", "yaml"}
	if !sliceContains(validFormats, strings.ToLower(config.Format)) {
		return fmt.Errorf("invalid log format '%s', must be one of: %s", config.Format, strings.Join(validFormats, ", "))
	}
	validOutputs := []string{"stdout", "stderr"}
	if !sliceContains(validOutputs, strings.ToLower(config.Output)) && config.Output != "" {
		expandedPath := corePaths.ExpandPath(config.Output)
		dir := filepath.Dir(expandedPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create log directory %s: %w", dir, err)
		}
		file, err := os.OpenFile(expandedPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("cannot write to log file %s: %w", expandedPath, err)
		}
		file.Close()
	}
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

// Debug logs a debug message.
func Debug(args ...interface{}) { GetGlobalLogger().Debug(args...) }

// Debugf logs a formatted debug message.
func Debugf(format string, args ...interface{}) { GetGlobalLogger().Debugf(format, args...) }

// Info logs an info message.
func Info(args ...interface{}) { GetGlobalLogger().Info(args...) }

// Infof logs a formatted info message.
func Infof(format string, args ...interface{}) { GetGlobalLogger().Infof(format, args...) }

// Warn logs a warning message.
func Warn(args ...interface{}) { GetGlobalLogger().Warn(args...) }

// Warnf logs a formatted warning message.
func Warnf(format string, args ...interface{}) { GetGlobalLogger().Warnf(format, args...) }

// Error logs an error message.
func Error(args ...interface{}) { GetGlobalLogger().Error(args...) }

// Errorf logs a formatted error message.
func Errorf(format string, args ...interface{}) { GetGlobalLogger().Errorf(format, args...) }

// Fatal logs a fatal message and exits.
func Fatal(args ...interface{}) { GetGlobalLogger().Fatal(args...) }

// Fatalf logs a formatted fatal message and exits.
func Fatalf(format string, args ...interface{}) { GetGlobalLogger().Fatalf(format, args...) }

// WithField creates a logger with a single field.
func WithField(key string, value interface{}) *logrus.Entry {
	return GetGlobalLogger().WithField(key, value)
}

// WithFields creates a logger with multiple fields.
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetGlobalLogger().WithFields(fields)
}

func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
