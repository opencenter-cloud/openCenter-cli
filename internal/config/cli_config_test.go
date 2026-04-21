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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	corePaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/testenv"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func TestDefaultCLIConfig(t *testing.T) {
	testenv.SetIsolatedCLIDirs(t)

	config := DefaultCLIConfig()

	// Test default values
	if config.Logging.Level != "warn" {
		t.Errorf("Expected default log level 'warn', got '%s'", config.Logging.Level)
	}

	if config.Logging.Format != "text" {
		t.Errorf("Expected default log format 'text', got '%s'", config.Logging.Format)
	}

	if config.Logging.Output != "stderr" {
		t.Errorf("Expected default log output 'stderr', got '%s'", config.Logging.Output)
	}

	if config.Behavior.AutoConfirm != false {
		t.Errorf("Expected default autoConfirm false, got %v", config.Behavior.AutoConfirm)
	}

	if config.Defaults.Provider != "openstack" {
		t.Errorf("Expected default provider 'openstack', got '%s'", config.Defaults.Provider)
	}

	if config.Paths.StateDir == "" {
		t.Error("Expected default stateDir to be populated")
	}
}

func TestConfigValidator(t *testing.T) {
	testenv.SetIsolatedCLIDirs(t)

	validator := &ConfigValidator{autoRepair: false}

	// Test valid configuration
	validConfig := DefaultCLIConfig()
	if err := validator.Validate(validConfig); err != nil {
		t.Errorf("Valid configuration should not produce error: %v", err)
	}

	// Test invalid log level
	invalidConfig := DefaultCLIConfig()
	invalidConfig.Logging.Level = "invalid"
	if err := validator.Validate(invalidConfig); err == nil {
		t.Error("Invalid log level should produce error")
	}

	// Test invalid log format
	invalidConfig = DefaultCLIConfig()
	invalidConfig.Logging.Format = "invalid"
	if err := validator.Validate(invalidConfig); err == nil {
		t.Error("Invalid log format should produce error")
	}
}

func TestConfigValidatorWithResult(t *testing.T) {
	testenv.SetIsolatedCLIDirs(t)

	// Test without auto-repair
	validator := &ConfigValidator{autoRepair: false}

	invalidConfig := DefaultCLIConfig()
	invalidConfig.Logging.Level = "invalid"
	invalidConfig.Logging.Format = "invalid"
	invalidConfig.Logging.File.MaxSize = -1

	result := validator.ValidateWithResult(invalidConfig)

	if result.Valid {
		t.Error("Invalid configuration should not be valid")
	}

	if len(result.Errors) < 3 {
		t.Errorf("Expected at least 3 errors, got %d", len(result.Errors))
	}

	if len(result.Repaired) != 0 {
		t.Errorf("Expected 0 repairs without auto-repair, got %d", len(result.Repaired))
	}
}

func TestConfigValidatorAutoRepair(t *testing.T) {
	testenv.SetIsolatedCLIDirs(t)

	// Test with auto-repair
	validator := &ConfigValidator{autoRepair: true}

	invalidConfig := DefaultCLIConfig()
	invalidConfig.Logging.Level = "invalid"
	invalidConfig.Logging.Format = "invalid"
	invalidConfig.Logging.File.MaxSize = -1

	result := validator.ValidateWithResult(invalidConfig)

	if !result.Valid {
		t.Error("Configuration should be valid after auto-repair")
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors after auto-repair, got %d", len(result.Errors))
	}

	if len(result.Repaired) < 3 {
		t.Errorf("Expected at least 3 repairs, got %d", len(result.Repaired))
	}

	// Verify that values were actually repaired
	defaults := DefaultCLIConfig()
	if invalidConfig.Logging.Level != defaults.Logging.Level {
		t.Errorf("Log level was not repaired to default")
	}

	if invalidConfig.Logging.Format != defaults.Logging.Format {
		t.Errorf("Log format was not repaired to default")
	}

	if invalidConfig.Logging.File.MaxSize != defaults.Logging.File.MaxSize {
		t.Errorf("MaxSize was not repaired to default")
	}
}

func TestConfigValidatorPathValidation(t *testing.T) {
	testenv.SetIsolatedCLIDirs(t)

	validator := &ConfigValidator{autoRepair: false}

	// Test empty paths
	invalidConfig := DefaultCLIConfig()
	invalidConfig.Paths.ConfigDir = ""
	invalidConfig.Paths.ClustersDir = ""
	invalidConfig.Paths.StateDir = ""

	result := validator.ValidateWithResult(invalidConfig)

	if result.Valid {
		t.Error("Configuration with empty paths should not be valid")
	}

	if len(result.Errors) < 3 {
		t.Errorf("Expected at least 3 path errors, got %d", len(result.Errors))
	}
}

func TestConfigValidatorWarnings(t *testing.T) {
	testenv.SetIsolatedCLIDirs(t)

	validator := &ConfigValidator{autoRepair: false}

	// Test configuration that should generate warnings
	config := DefaultCLIConfig()
	config.Behavior.AutoConfirm = true
	config.Behavior.DryRun = false
	config.Defaults.Provider = "unknown-provider"
	config.Defaults.Environment = "unknown-env"

	result := validator.ValidateWithResult(config)

	if len(result.Warnings) == 0 {
		t.Error("Expected warnings for potentially problematic configuration")
	}
}

func TestConfigManagerSetGetValue(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Test setting and getting values
	tests := []struct {
		key   string
		value interface{}
	}{
		{"logging.level", "debug"},
		{"logging.format", "json"},
		{"behavior.dryRun", true},
		{"defaults.provider", "aws"},
		{"logging.file.maxSize", 200},
		{"paths.stateDir", filepath.Join(tmpDir, "state")},
	}

	for _, test := range tests {
		// Set value
		if err := cm.SetValue(test.key, test.value); err != nil {
			t.Errorf("Failed to set %s = %v: %v", test.key, test.value, err)
			continue
		}

		// Get value
		result, err := cm.GetValue(test.key)
		if err != nil {
			t.Errorf("Failed to get %s: %v", test.key, err)
			continue
		}

		if result != test.value {
			t.Errorf("GetValue(%s) = %v, expected %v", test.key, result, test.value)
		}
	}
}

func TestConfigError(t *testing.T) {
	err := &ConfigError{
		Type:    "validation",
		Field:   "logging.level",
		Value:   "invalid",
		Message: "invalid log level",
	}

	expectedError := "validation error in field 'logging.level': invalid log level"
	if err.Error() != expectedError {
		t.Errorf("ConfigError.Error() = %s, expected %s", err.Error(), expectedError)
	}

	suggestions := err.Suggestions()
	if len(suggestions) == 0 {
		t.Error("ConfigError should provide suggestions")
	}

	// Test repaired error
	repairedErr := &ConfigError{
		Type:     "validation",
		Field:    "logging.level",
		Value:    "warn",
		Message:  "repaired to default",
		Repaired: true,
	}

	expectedRepairedError := "[AUTO-REPAIRED] validation error in field 'logging.level': repaired to default"
	if repairedErr.Error() != expectedRepairedError {
		t.Errorf("Repaired ConfigError.Error() = %s, expected %s", repairedErr.Error(), expectedRepairedError)
	}

	repairedSuggestions := repairedErr.Suggestions()
	if len(repairedSuggestions) == 0 || repairedSuggestions[0] != "Configuration was automatically repaired with default values" {
		t.Error("Repaired ConfigError should provide repair confirmation")
	}
}

func TestConfigManagerMergeWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Create partial config
	partialConfig := &CLIConfig{
		Logging: LoggingConfig{
			Level: "debug",
		},
		Behavior: BehaviorConfig{
			DryRun: true,
		},
	}

	// Merge with defaults
	merged := cm.mergeWithDefaults(partialConfig)

	// Check that specified values are preserved
	if merged.Logging.Level != "debug" {
		t.Errorf("Expected merged log level 'debug', got '%s'", merged.Logging.Level)
	}

	if merged.Behavior.DryRun != true {
		t.Errorf("Expected merged dryRun true, got %v", merged.Behavior.DryRun)
	}

	// Check that default values are filled in
	if merged.Logging.Format != "text" {
		t.Errorf("Expected default log format 'text', got '%s'", merged.Logging.Format)
	}

	if merged.Defaults.Provider != "openstack" {
		t.Errorf("Expected default provider 'openstack', got '%s'", merged.Defaults.Provider)
	}
}

func TestConfigManagerValidation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Test validation of valid config
	result := cm.ValidateConfig()
	if !result.Valid {
		t.Error("Default configuration should be valid")
	}

	// Test validation summary
	summary := cm.GetValidationSummary()
	if !strings.Contains(summary, "✓ Configuration is valid") {
		t.Error("Validation summary should indicate valid configuration")
	}
}

func TestConfigManagerRepair(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Introduce invalid values
	cm.config.Logging.Level = "invalid"
	cm.config.Logging.Format = "invalid"
	cm.config.Logging.File.MaxSize = -1

	// Test repair
	result, err := cm.RepairConfig()
	if err != nil {
		t.Errorf("RepairConfig should not return error: %v", err)
	}

	if !result.Valid {
		t.Error("Configuration should be valid after repair")
	}

	if len(result.Repaired) == 0 {
		t.Error("Expected repairs to be made")
	}

	// Verify values were actually repaired
	defaults := DefaultCLIConfig()
	if cm.config.Logging.Level != defaults.Logging.Level {
		t.Error("Log level should be repaired to default")
	}
}

func TestConfigManagerGracefulDegradation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config file with invalid content
	invalidYAML := `
logging:
  level: invalid_level
  format: invalid_format
  file:
    maxSize: -1
paths:
  configDir: ""
  clustersDir: ""
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0600); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	// ConfigManager should handle this gracefully with auto-repair
	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("ConfigManager should handle invalid config gracefully: %v", err)
	}

	// Configuration should be valid after auto-repair
	result := cm.ValidateConfig()
	if !result.Valid {
		t.Error("Configuration should be valid after auto-repair during load")
	}

	// Check that default values were applied
	defaults := DefaultCLIConfig()
	if cm.config.Logging.Level != defaults.Logging.Level {
		t.Error("Invalid log level should be repaired to default")
	}
}

func TestLoggingInitialization(t *testing.T) {
	// Test default logging initialization
	defaultConfig := DefaultCLIConfig()
	err := InitializeLogging(&defaultConfig.Logging)
	if err != nil {
		t.Errorf("Failed to initialize default logging: %v", err)
	}

	logger := GetGlobalLogger()
	if logger == nil {
		t.Error("Global logger should not be nil after initialization")
	}

	// Test that log level is set correctly
	if logger.Level.String() != "warning" {
		t.Errorf("Expected log level 'warning', got '%s'", logger.Level.String())
	}
}

func TestLoggingValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      LoggingConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
				File: FileConfig{
					MaxSize:    100,
					MaxBackups: 3,
					MaxAge:     28,
					Compress:   true,
				},
			},
			expectError: false,
		},
		{
			name: "invalid log level",
			config: LoggingConfig{
				Level:  "invalid",
				Format: "text",
				Output: "stderr",
				File: FileConfig{
					MaxSize:    100,
					MaxBackups: 3,
					MaxAge:     28,
					Compress:   true,
				},
			},
			expectError: true,
		},
		{
			name: "invalid log format",
			config: LoggingConfig{
				Level:  "info",
				Format: "invalid",
				Output: "stderr",
				File: FileConfig{
					MaxSize:    100,
					MaxBackups: 3,
					MaxAge:     28,
					Compress:   true,
				},
			},
			expectError: true,
		},
		{
			name: "invalid file config",
			config: LoggingConfig{
				Level:  "info",
				Format: "text",
				Output: "stderr",
				File: FileConfig{
					MaxSize:    -1,
					MaxBackups: 3,
					MaxAge:     28,
					Compress:   true,
				},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateLoggingConfig(&test.config)
			if test.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestLoggingFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := LoggingConfig{
		Level:  "debug",
		Format: "text",
		Output: logFile,
		File: FileConfig{
			MaxSize:    1,
			MaxBackups: 2,
			MaxAge:     1,
			Compress:   false,
		},
	}

	err := InitializeLogging(&config)
	if err != nil {
		t.Errorf("Failed to initialize file logging: %v", err)
	}

	// Test logging to file
	Debug("Test debug message")
	Info("Test info message")
	Warn("Test warning message")
	Error("Test error message")

	// Check that log file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should have been created")
	}
}

func TestLoggingFormats(t *testing.T) {
	formats := []string{"text", "json", "yaml"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			config := LoggingConfig{
				Level:  "info",
				Format: format,
				Output: "stderr",
				File: FileConfig{
					MaxSize:    100,
					MaxBackups: 3,
					MaxAge:     28,
					Compress:   true,
				},
			}

			err := InitializeLogging(&config)
			if err != nil {
				t.Errorf("Failed to initialize logging with format %s: %v", format, err)
			}

			// Test that we can log without errors
			Info("Test message for format: " + format)
		})
	}
}

func TestSetLogLevel(t *testing.T) {
	// Initialize with default config
	defaultConfig := DefaultCLIConfig()
	err := InitializeLogging(&defaultConfig.Logging)
	if err != nil {
		t.Errorf("Failed to initialize logging: %v", err)
	}

	// Test setting valid log levels
	levels := []string{"debug", "info", "warn", "error"}
	for _, level := range levels {
		err := SetLogLevel(level)
		if err != nil {
			t.Errorf("Failed to set log level to %s: %v", level, err)
		}

		logger := GetGlobalLogger()
		expectedLevel := level
		if level == "warn" {
			expectedLevel = "warning"
		}
		if logger.Level.String() != expectedLevel {
			t.Errorf("Expected log level %s, got %s", expectedLevel, logger.Level.String())
		}
	}

	// Test setting invalid log level
	err = SetLogLevel("invalid")
	if err == nil {
		t.Error("Expected error when setting invalid log level")
	}
}

func TestSetLogFormat(t *testing.T) {
	// Initialize with default config
	defaultConfig := DefaultCLIConfig()
	err := InitializeLogging(&defaultConfig.Logging)
	if err != nil {
		t.Errorf("Failed to initialize logging: %v", err)
	}

	// Test setting valid log formats
	formats := []string{"text", "json", "yaml"}
	for _, format := range formats {
		err := SetLogFormat(format)
		if err != nil {
			t.Errorf("Failed to set log format to %s: %v", format, err)
		}
		// We can't easily test the formatter type, but we can ensure no error occurred
	}

	// Test setting invalid log format
	err = SetLogFormat("invalid")
	if err == nil {
		t.Error("Expected error when setting invalid log format")
	}
}

func TestYAMLFormatter(t *testing.T) {
	formatter := &YAMLFormatter{}

	// Create a test log entry
	entry := &logrus.Entry{
		Time:    time.Now(),
		Level:   logrus.InfoLevel,
		Message: "Test message",
		Data: logrus.Fields{
			"key1": "value1",
			"key2": 42,
		},
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Errorf("YAML formatter failed: %v", err)
	}

	outputStr := string(output)

	// Check that output contains expected YAML elements
	if !strings.Contains(outputStr, "timestamp:") {
		t.Error("YAML output should contain timestamp")
	}

	if !strings.Contains(outputStr, "level: info") {
		t.Error("YAML output should contain log level")
	}

	if !strings.Contains(outputStr, "message: \"Test message\"") {
		t.Error("YAML output should contain message")
	}

	if !strings.Contains(outputStr, "fields:") {
		t.Error("YAML output should contain fields section")
	}

	if !strings.Contains(outputStr, "key1: value1") {
		t.Error("YAML output should contain field data")
	}

	if !strings.Contains(outputStr, "---") {
		t.Error("YAML output should end with document separator")
	}
}

func TestLoggingHelperFunctions(t *testing.T) {
	// Initialize with debug level to capture all messages
	config := LoggingConfig{
		Level:  "debug",
		Format: "text",
		Output: "stderr",
		File: FileConfig{
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
	}

	err := InitializeLogging(&config)
	if err != nil {
		t.Errorf("Failed to initialize logging: %v", err)
	}

	// Test all logging helper functions (they should not panic)
	Debug("Debug message")
	Debugf("Debug message with format: %s", "test")
	Info("Info message")
	Infof("Info message with format: %d", 42)
	Warn("Warning message")
	Warnf("Warning message with format: %v", true)
	Error("Error message")
	Errorf("Error message with format: %s", "error")

	// Test WithField and WithFields
	WithField("key", "value").Info("Message with field")
	WithFields(logrus.Fields{
		"key1": "value1",
		"key2": 42,
	}).Info("Message with fields")
}

func TestEnvironmentExpansion(t *testing.T) {
	// Set up allowlisted path environment variables
	t.Setenv("HOME", "/tmp/test-home")
	t.Setenv("OPENCENTER_CONFIG_DIR", "/tmp/opencenter")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config with environment variables
	configYAML := `
logging:
  level: info
  format: text
  output: stderr
paths:
  configDir: ${HOME}/test-config
  clustersDir: ${OPENCENTER_CONFIG_DIR}/clusters
behavior:
  autoConfirm: false
  dryRun: false
defaults:
  provider: openstack
  region: "sjc3"
  environment: dev
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load configuration
	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	config := cm.GetConfig()

	// Verify allowlisted path variables were expanded
	if config.Paths.ConfigDir != "/tmp/test-home/test-config" {
		t.Errorf("Expected configDir '/tmp/test-home/test-config', got '%s'", config.Paths.ConfigDir)
	}

	if config.Paths.ClustersDir != "/tmp/opencenter/clusters" {
		t.Errorf("Expected clustersDir '/tmp/opencenter/clusters', got '%s'", config.Paths.ClustersDir)
	}
}

func TestConfigurationPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create base configuration file
	baseConfig := DefaultCLIConfig()
	baseConfig.Logging.Level = "info"
	baseConfig.Behavior.DryRun = false

	data, err := yaml.Marshal(baseConfig)
	if err != nil {
		t.Fatalf("Failed to marshal base config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write base config: %v", err)
	}

	// Test 1: Configuration file values
	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	config := cm.GetConfig()
	if config.Logging.Level != "info" {
		t.Errorf("Expected log level 'info' from config file, got '%s'", config.Logging.Level)
	}

	// Test 2: Environment variable override
	os.Setenv("OPENCENTER_LOG_LEVEL", "debug")
	defer os.Unsetenv("OPENCENTER_LOG_LEVEL")

	// Create config with environment variable in YAML
	configWithEnv := `
logging:
  level: ${OPENCENTER_LOG_LEVEL}
  format: text
  output: stderr
paths:
  configDir: ` + baseConfig.Paths.ConfigDir + `
  clustersDir: ` + baseConfig.Paths.ClustersDir + `
behavior:
  autoConfirm: false
  dryRun: false
defaults:
  provider: openstack
  region: "sjc3"
  environment: dev
`

	// Expand environment variables before writing
	expandedConfig := os.ExpandEnv(configWithEnv)
	if err := os.WriteFile(configPath, []byte(expandedConfig), 0600); err != nil {
		t.Fatalf("Failed to write config with env var: %v", err)
	}

	cm2, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager with env: %v", err)
	}

	config2 := cm2.GetConfig()
	if config2.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug' from environment variable, got '%s'", config2.Logging.Level)
	}
}

func TestConfigManagerDotNotationEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	tests := []struct {
		name        string
		key         string
		value       interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "invalid key format",
			key:         "invalid",
			value:       "test",
			expectError: true,
			errorMsg:    "path must have at least two parts",
		},
		{
			name:        "unknown section",
			key:         "unknown.field",
			value:       "test",
			expectError: true,
			errorMsg:    "unknown configuration section: unknown",
		},
		{
			name:        "unknown logging field",
			key:         "logging.unknown",
			value:       "test",
			expectError: true,
			errorMsg:    "unknown logging field: unknown",
		},
		{
			name:        "invalid type for boolean field",
			key:         "behavior.dryRun",
			value:       "not-a-boolean",
			expectError: true,
			errorMsg:    "dryRun must be a boolean",
		},
		{
			name:        "valid nested file config",
			key:         "logging.file.maxSize",
			value:       200,
			expectError: false,
		},
		{
			name:        "string number conversion",
			key:         "logging.file.maxBackups",
			value:       "5",
			expectError: false,
		},
		{
			name:        "string boolean conversion",
			key:         "behavior.autoConfirm",
			value:       "true",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.SetValue(tt.key, tt.value)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for key '%s', but got none", tt.key)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for key '%s', but got: %v", tt.key, err)
					return
				}

				// Verify the value was set correctly
				result, err := cm.GetValue(tt.key)
				if err != nil {
					t.Errorf("Failed to get value for key '%s': %v", tt.key, err)
					return
				}

				// Convert expected value for comparison
				var expectedValue interface{}
				switch tt.key {
				case "logging.file.maxSize", "logging.file.maxBackups":
					if str, ok := tt.value.(string); ok {
						if intVal, err := strconv.Atoi(str); err == nil {
							expectedValue = intVal
						} else {
							expectedValue = tt.value
						}
					} else {
						expectedValue = tt.value
					}
				case "behavior.autoConfirm":
					if str, ok := tt.value.(string); ok {
						if boolVal, err := strconv.ParseBool(str); err == nil {
							expectedValue = boolVal
						} else {
							expectedValue = tt.value
						}
					} else {
						expectedValue = tt.value
					}
				default:
					expectedValue = tt.value
				}

				if result != expectedValue {
					t.Errorf("GetValue(%s) = %v, expected %v", tt.key, result, expectedValue)
				}
			}
		})
	}
}

func TestConfigValidatorComprehensive(t *testing.T) {
	testenv.SetIsolatedCLIDirs(t)

	tests := []struct {
		name           string
		config         *CLIConfig
		autoRepair     bool
		expectValid    bool
		expectErrors   int
		expectWarnings int
		expectRepairs  int
	}{
		{
			name: "completely invalid config without auto-repair",
			config: &CLIConfig{
				Logging: LoggingConfig{
					Level:  "invalid-level",
					Format: "invalid-format",
					Output: "invalid-output",
					File: FileConfig{
						MaxSize:    -1,
						MaxBackups: -1,
						MaxAge:     -1,
					},
				},
				Paths: PathsConfig{
					ConfigDir:   "",
					ClustersDir: "",
				},
				Behavior: BehaviorConfig{
					AutoConfirm: true,
					DryRun:      false, // This should generate a warning
				},
				Defaults: DefaultsConfig{
					Provider:    "unknown-provider",
					Region:      "x",
					Environment: "unknown-env",
				},
			},
			autoRepair:     false,
			expectValid:    false,
			expectErrors:   9, // level, format, maxSize, maxBackups, maxAge, configDir, clustersDir, pluginsDir, stateDir
			expectWarnings: 8, // autoConfirm without dryRun, provider, region, environment, disk space warnings (configDir, clustersDir, pluginsDir, stateDir)
			expectRepairs:  0,
		},
		{
			name: "completely invalid config with auto-repair",
			config: &CLIConfig{
				Logging: LoggingConfig{
					Level:  "invalid-level",
					Format: "invalid-format",
					Output: "invalid-output",
					File: FileConfig{
						MaxSize:    -1,
						MaxBackups: -1,
						MaxAge:     -1,
					},
				},
				Paths: PathsConfig{
					ConfigDir:   "",
					ClustersDir: "",
				},
				Behavior: BehaviorConfig{
					AutoConfirm: true,
					DryRun:      false,
				},
				Defaults: DefaultsConfig{
					Provider:    "unknown-provider",
					Region:      "x",
					Environment: "unknown-env",
				},
			},
			autoRepair:     true,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 4, // Warnings are not auto-repaired (excluding disk space warnings which are repaired)
			expectRepairs:  9, // All validation errors should be repaired (including pluginsDir and stateDir)
		},
		{
			name:           "valid config",
			config:         DefaultCLIConfig(),
			autoRepair:     false,
			expectValid:    true,
			expectErrors:   0,
			expectWarnings: 0,
			expectRepairs:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &ConfigValidator{autoRepair: tt.autoRepair}
			result := validator.ValidateWithResult(tt.config)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, result.Valid)
			}

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}

			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}

			if len(result.Repaired) != tt.expectRepairs {
				t.Errorf("Expected %d repairs, got %d: %v", tt.expectRepairs, len(result.Repaired), result.Repaired)
			}
		})
	}
}

func TestPathValidationEdgeCases(t *testing.T) {
	config := DefaultCLIConfig()
	_, err := NewConfigManagerWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "path with double dots",
			path:        "/tmp/../etc/passwd",
			expectError: true,
		},
		{
			name:        "relative path",
			path:        "relative/path",
			expectError: true,
		},
		{
			name:        "valid absolute path",
			path:        "/tmp/valid/path",
			expectError: false,
		},
		{
			name:        "path with tilde",
			path:        "~/valid/path",
			expectError: false,
		},
		{
			name:        "path with environment variable",
			path:        "${HOME}/valid/path",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test direct path validation using ExpandPath and basic checks
			expandedPath := corePaths.ExpandPath(tt.path)

			var hasError bool
			var errorMsg string

			// Check for path traversal
			if strings.Contains(expandedPath, "..") {
				hasError = true
				errorMsg = "contains directory traversal"
			}

			// Check for empty path
			if tt.path == "" {
				hasError = true
				errorMsg = "empty path"
			}

			// Check if path is absolute after expansion (for non-empty paths)
			if tt.path != "" && !filepath.IsAbs(expandedPath) {
				hasError = true
				errorMsg = "not absolute after expansion"
			}

			if tt.expectError && !hasError {
				t.Errorf("Expected path validation error for '%s', but got none", tt.path)
			}

			if !tt.expectError && hasError {
				t.Errorf("Expected no path validation error for '%s', but got: %s", tt.path, errorMsg)
			}
		})
	}
}

func TestConfigManagerConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// Test concurrent access to configuration
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Alternate between reading and writing
				if j%2 == 0 {
					// Read operation
					_, err := cm.GetValue("logging.level")
					if err != nil {
						t.Errorf("Goroutine %d: GetValue failed: %v", id, err)
						return
					}
				} else {
					// Write operation
					level := "info"
					if j%4 == 1 {
						level = "debug"
					}
					err := cm.SetValue("logging.level", level)
					if err != nil {
						t.Errorf("Goroutine %d: SetValue failed: %v", id, err)
						return
					}
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final state is still valid
	result := cm.ValidateConfig()
	if !result.Valid {
		t.Errorf("Configuration became invalid after concurrent operations: %v", result.Errors)
	}
}

func TestGetStateDirPrecedence(t *testing.T) {
	t.Run("environment override wins", func(t *testing.T) {
		configDir := t.TempDir()
		stateDir := t.TempDir()
		t.Setenv("OPENCENTER_CONFIG_DIR", configDir)
		t.Setenv("OPENCENTER_STATE_DIR", stateDir)

		if actual := GetStateDir(); actual != stateDir {
			t.Fatalf("GetStateDir() = %s, want %s", actual, stateDir)
		}
	})

	t.Run("cli config path used when env unset", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("OPENCENTER_CONFIG_DIR", configDir)
		t.Setenv("OPENCENTER_STATE_DIR", "")

		cm, err := NewConfigManager("")
		if err != nil {
			t.Fatalf("Failed to create config manager: %v", err)
		}

		stateDir := filepath.Join(t.TempDir(), "state-root")
		if err := cm.SetValue("paths.stateDir", stateDir); err != nil {
			t.Fatalf("SetValue(paths.stateDir) error = %v", err)
		}
		if err := cm.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		if actual := GetStateDir(); actual != stateDir {
			t.Fatalf("GetStateDir() = %s, want %s", actual, stateDir)
		}
	})
}
