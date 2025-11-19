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
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// GlobalFlags represents the global flags for testing (mirrors cmd package structure)
type GlobalFlags struct {
	Config   string
	DryRun   bool
	LogLevel string
	Set      []string
	Verbose  bool
}

func TestGlobalFlagOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create base configuration
	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	baseConfig := cm.GetConfig()

	tests := []struct {
		name          string
		globalFlags   *GlobalFlags
		expectLevel   string
		expectDryRun  bool
		expectVerbose bool
	}{
		{
			name: "verbose flag overrides log level",
			globalFlags: &GlobalFlags{
				Verbose:  true,
				LogLevel: "warn", // Should be overridden by verbose
			},
			expectLevel:   "debug",
			expectVerbose: true,
		},
		{
			name: "explicit log level",
			globalFlags: &GlobalFlags{
				LogLevel: "info",
				Verbose:  false,
			},
			expectLevel:   "info",
			expectVerbose: false,
		},
		{
			name: "dry-run flag",
			globalFlags: &GlobalFlags{
				DryRun: true,
			},
			expectDryRun: true,
		},
		{
			name: "multiple overrides",
			globalFlags: &GlobalFlags{
				LogLevel: "error",
				DryRun:   true,
				Verbose:  false,
			},
			expectLevel:   "error",
			expectDryRun:  true,
			expectVerbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of base config
			testConfig := *baseConfig

			// Apply global flag overrides (simulating the cmd package logic)
			if tt.globalFlags.LogLevel != "" && tt.globalFlags.LogLevel != "warn" {
				testConfig.Logging.Level = tt.globalFlags.LogLevel
			}
			if tt.globalFlags.Verbose {
				testConfig.Logging.Level = "debug"
				testConfig.Behavior.Verbose = true
			}
			if tt.globalFlags.DryRun {
				testConfig.Behavior.DryRun = true
			}

			// Verify expected values
			if testConfig.Logging.Level != tt.expectLevel && tt.expectLevel != "" {
				t.Errorf("Expected log level '%s', got '%s'", tt.expectLevel, testConfig.Logging.Level)
			}
			if testConfig.Behavior.DryRun != tt.expectDryRun {
				t.Errorf("Expected dry-run %v, got %v", tt.expectDryRun, testConfig.Behavior.DryRun)
			}
			if testConfig.Behavior.Verbose != tt.expectVerbose {
				t.Errorf("Expected verbose %v, got %v", tt.expectVerbose, testConfig.Behavior.Verbose)
			}
		})
	}
}

func TestSetFlagParsing(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	tests := []struct {
		name        string
		setFlags    []string
		expectError bool
		verifyFunc  func(*CLIConfig) error
	}{
		{
			name:     "set string value",
			setFlags: []string{"defaults.provider=aws"},
			verifyFunc: func(config *CLIConfig) error {
				if config.Defaults.Provider != "aws" {
					t.Errorf("Expected provider 'aws', got '%s'", config.Defaults.Provider)
				}
				return nil
			},
		},
		{
			name:     "set boolean value true",
			setFlags: []string{"behavior.verbose=true"},
			verifyFunc: func(config *CLIConfig) error {
				if !config.Behavior.Verbose {
					t.Errorf("Expected verbose true, got %v", config.Behavior.Verbose)
				}
				return nil
			},
		},
		{
			name:     "set boolean value false",
			setFlags: []string{"behavior.autoConfirm=false"},
			verifyFunc: func(config *CLIConfig) error {
				if config.Behavior.AutoConfirm {
					t.Errorf("Expected autoConfirm false, got %v", config.Behavior.AutoConfirm)
				}
				return nil
			},
		},
		{
			name:     "set integer value",
			setFlags: []string{"logging.file.maxSize=200"},
			verifyFunc: func(config *CLIConfig) error {
				if config.Logging.File.MaxSize != 200 {
					t.Errorf("Expected maxSize 200, got %d", config.Logging.File.MaxSize)
				}
				return nil
			},
		},
		{
			name: "set multiple values",
			setFlags: []string{
				"logging.level=debug",
				"defaults.region=us-west-2",
				"behavior.dryRun=true",
			},
			verifyFunc: func(config *CLIConfig) error {
				if config.Logging.Level != "debug" {
					t.Errorf("Expected log level 'debug', got '%s'", config.Logging.Level)
				}
				if config.Defaults.Region != "us-west-2" {
					t.Errorf("Expected region 'us-west-2', got '%s'", config.Defaults.Region)
				}
				if !config.Behavior.DryRun {
					t.Errorf("Expected dryRun true, got %v", config.Behavior.DryRun)
				}
				return nil
			},
		},
		{
			name:        "invalid format missing equals",
			setFlags:    []string{"logging.level"},
			expectError: true,
		},
		{
			name:        "invalid format multiple equals",
			setFlags:    []string{"logging.level=debug=extra"},
			expectError: true, // Should fail validation because "debug=extra" is not a valid log level
		},
		{
			name:        "invalid key path",
			setFlags:    []string{"invalid.path=value"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh config for each test
			testConfig := *cm.GetConfig()

			// Simulate the --set flag processing logic from cmd package
			err := applySetFlagOverrides(&testConfig, tt.setFlags)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for set flags %v, but got none", tt.setFlags)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for set flags %v, but got: %v", tt.setFlags, err)
				return
			}

			// Run verification function if provided
			if tt.verifyFunc != nil {
				if err := tt.verifyFunc(&testConfig); err != nil {
					t.Errorf("Verification failed: %v", err)
				}
			}
		})
	}
}

// applySetFlagOverrides simulates the --set flag processing from cmd package
func applySetFlagOverrides(cliConfig *CLIConfig, setFlags []string) error {
	// Create a temporary config manager to use its SetValue method
	tempManager, err := NewConfigManagerWithConfig(cliConfig)
	if err != nil {
		return err
	}

	for _, setFlag := range setFlags {
		// Parse key=value format
		parts := []string{}
		if equalIndex := findFirstEquals(setFlag); equalIndex != -1 {
			parts = []string{
				setFlag[:equalIndex],
				setFlag[equalIndex+1:],
			}
		}

		if len(parts) != 2 {
			return &ConfigError{
				Type:    "validation",
				Field:   "set-flag",
				Value:   setFlag,
				Message: "invalid --set format, expected key=value",
			}
		}

		key := parts[0]
		value := parts[1]

		// Parse the value (try to detect type)
		var parsedValue interface{}
		if value == "true" {
			parsedValue = true
		} else if value == "false" {
			parsedValue = false
		} else {
			// Try to parse as number
			if intVal := tryParseInt(value); intVal != nil {
				parsedValue = *intVal
			} else {
				// Treat as string
				parsedValue = value
			}
		}

		// Apply the override using the configuration manager's dot notation
		if err := tempManager.SetValue(key, parsedValue); err != nil {
			return err
		}
	}

	// Get the updated configuration back
	*cliConfig = *tempManager.GetConfig()
	return nil
}

// findFirstEquals finds the first equals sign in a string
func findFirstEquals(s string) int {
	for i, c := range s {
		if c == '=' {
			return i
		}
	}
	return -1
}

// tryParseInt attempts to parse a string as an integer
func tryParseInt(s string) *int {
	var result int
	if n, err := fmt.Sscanf(s, "%d", &result); err == nil && n == 1 {
		return &result
	}
	return nil
}

func TestConfigurationPrecedenceOrder(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test the precedence order: --set flags > command flags > config file > defaults

	// 1. Start with defaults
	defaultConfig := DefaultCLIConfig()
	if defaultConfig.Logging.Level != "warn" {
		t.Errorf("Expected default log level 'warn', got '%s'", defaultConfig.Logging.Level)
	}

	// 2. Create config file with different value
	fileConfig := DefaultCLIConfig()
	fileConfig.Logging.Level = "info"
	fileConfig.Behavior.DryRun = false

	data, err := yaml.Marshal(fileConfig)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config from file
	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	config := cm.GetConfig()
	if config.Logging.Level != "info" {
		t.Errorf("Expected log level 'info' from config file, got '%s'", config.Logging.Level)
	}

	// 3. Apply global flag overrides (simulating command flags)
	globalFlags := &GlobalFlags{
		LogLevel: "debug",
		DryRun:   true,
	}

	// Apply overrides
	overriddenConfig := *config
	if globalFlags.LogLevel != "" && globalFlags.LogLevel != "warn" {
		overriddenConfig.Logging.Level = globalFlags.LogLevel
	}
	if globalFlags.DryRun {
		overriddenConfig.Behavior.DryRun = true
	}

	if overriddenConfig.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug' from global flags, got '%s'", overriddenConfig.Logging.Level)
	}
	if !overriddenConfig.Behavior.DryRun {
		t.Errorf("Expected dry-run true from global flags, got %v", overriddenConfig.Behavior.DryRun)
	}

	// 4. Apply --set flag overrides (highest precedence)
	setFlags := []string{"logging.level=error"}
	if err := applySetFlagOverrides(&overriddenConfig, setFlags); err != nil {
		t.Fatalf("Failed to apply --set overrides: %v", err)
	}

	if overriddenConfig.Logging.Level != "error" {
		t.Errorf("Expected log level 'error' from --set flags, got '%s'", overriddenConfig.Logging.Level)
	}

	// Verify that dry-run is still true (not overridden by --set)
	if !overriddenConfig.Behavior.DryRun {
		t.Errorf("Expected dry-run to remain true after --set override")
	}
}

func TestEnvironmentVariableExpansionInSetFlags(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_PROVIDER", "aws")
	os.Setenv("TEST_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("TEST_PROVIDER")
		os.Unsetenv("TEST_REGION")
	}()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	config := cm.GetConfig()

	// Test --set flags with environment variable values
	setFlags := []string{
		"defaults.provider=${TEST_PROVIDER}",
		"defaults.region=${TEST_REGION}",
	}

	// Expand environment variables in set flags before applying
	expandedSetFlags := make([]string, len(setFlags))
	for i, flag := range setFlags {
		expandedSetFlags[i] = os.ExpandEnv(flag)
	}

	if err := applySetFlagOverrides(config, expandedSetFlags); err != nil {
		t.Fatalf("Failed to apply --set overrides: %v", err)
	}

	if config.Defaults.Provider != "aws" {
		t.Errorf("Expected provider 'aws' from environment variable, got '%s'", config.Defaults.Provider)
	}

	if config.Defaults.Region != "us-east-1" {
		t.Errorf("Expected region 'us-east-1' from environment variable, got '%s'", config.Defaults.Region)
	}
}

func TestGlobalFlagValidation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cm, err := NewConfigManager(configPath)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	tests := []struct {
		name        string
		globalFlags *GlobalFlags
		expectError bool
		errorField  string
	}{
		{
			name: "valid log level",
			globalFlags: &GlobalFlags{
				LogLevel: "debug",
			},
			expectError: false,
		},
		{
			name: "invalid log level via set flag",
			globalFlags: &GlobalFlags{
				Set: []string{"logging.level=invalid"},
			},
			expectError: true,
			errorField:  "logging.level",
		},
		{
			name: "valid boolean via set flag",
			globalFlags: &GlobalFlags{
				Set: []string{"behavior.verbose=true"},
			},
			expectError: false,
		},
		{
			name: "invalid boolean via set flag",
			globalFlags: &GlobalFlags{
				Set: []string{"behavior.verbose=not-a-boolean"},
			},
			expectError: true,
			errorField:  "behavior.verbose",
		},
		{
			name: "valid integer via set flag",
			globalFlags: &GlobalFlags{
				Set: []string{"logging.file.maxSize=100"},
			},
			expectError: false,
		},
		{
			name: "invalid integer via set flag",
			globalFlags: &GlobalFlags{
				Set: []string{"logging.file.maxSize=not-a-number"},
			},
			expectError: true,
			errorField:  "logging.file.maxSize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := *cm.GetConfig()

			// Apply global flag overrides
			if tt.globalFlags.LogLevel != "" {
				config.Logging.Level = tt.globalFlags.LogLevel
			}

			var err error
			if len(tt.globalFlags.Set) > 0 {
				err = applySetFlagOverrides(&config, tt.globalFlags.Set)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for global flags, but got none")
					return
				}
				if tt.errorField != "" {
					configErr, ok := err.(*ConfigError)
					if !ok {
						t.Errorf("Expected ConfigError, got %T", err)
						return
					}
					if configErr.Field != tt.errorField {
						t.Errorf("Expected error in field '%s', got '%s'", tt.errorField, configErr.Field)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for global flags, but got: %v", err)
				}
			}
		})
	}
}
