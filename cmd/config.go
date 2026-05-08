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

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigCmd creates the top-level "config" command for CLI configuration management.
// It provides subcommands for viewing, setting, getting, resetting, and locating
// the CLI configuration file.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration settings",
		Long: `Manage CLI configuration settings including logging, paths, behavior, and cluster defaults.

The configuration file is stored at ~/.config/opencenter/config.yaml by default,
or at the location specified by the OPENCENTER_CONFIG_DIR environment variable.

Configuration values can be accessed and modified using dot notation (e.g., logging.level).
The cluster_defaults section controls values injected into new cluster configs during
"opencenter cluster init".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newConfigViewCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigResetCmd())
	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigEditCmd())
	cmd.AddCommand(newConfigExplainCmd())
	cmd.AddCommand(newConfigIDECmd())

	return cmd
}

// newConfigViewCmd creates the "config view" command to display the current configuration.
func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Display the current CLI configuration",
		Long: `Display the current CLI configuration in YAML format.

This shows the configuration file content exactly as it would appear in an editor.
Use 'opencenter config edit' to modify the configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the configuration file path
			cm, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			configPath := cm.GetConfigPath()

			// Read the raw file content
			data, err := os.ReadFile(configPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("configuration file not found at %s. Run 'opencenter config edit' to create it", configPath)
				}
				return fmt.Errorf("failed to read configuration file: %w", err)
			}

			// Print the raw content
			fmt.Print(string(data))
			return nil
		},
	}
}

// newConfigSetCmd creates the "config set" command to modify configuration values.
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value using dot notation",
		Long: `Set a configuration value using dot notation.

Examples:
  opencenter config set logging.level debug
  opencenter config set paths.clustersDir ~/my-clusters
  opencenter config set paths.pluginsDir ~/my-plugins
  opencenter config set paths.stateDir ~/.local/state/opencenter
  opencenter config set behavior.autoConfirm true
  opencenter config set behavior.validation online
  opencenter config set cluster_defaults.provider openstack
  opencenter config set cluster_defaults.gitops_auth_method token
  opencenter config set cluster_defaults.base_domain k8s.example.com
  opencenter config set cluster_defaults.kubernetes_version 1.34.3

Supported configuration sections:
  - logging.level (debug, info, warn, error)
  - logging.format (text, json, yaml)
  - logging.output (stdout, stderr, or file path)
  - logging.file.maxSize (integer, MB)
  - logging.file.maxBackups (integer)
  - logging.file.maxAge (integer, days)
  - logging.file.compress (boolean)
  - paths.configDir (string)
  - paths.clustersDir (string)
  - paths.pluginsDir (string)
  - paths.stateDir (string)
  - behavior.autoConfirm (boolean)
  - behavior.dryRun (boolean)
  - behavior.validation (string: offline, online)
  - cluster_defaults.provider (string)
  - cluster_defaults.region (string)
  - cluster_defaults.environment (string)
  - cluster_defaults.gitops_auth_method (string: ssh, token)
  - cluster_defaults.base_domain (string)
  - cluster_defaults.admin_email (string)
  - cluster_defaults.kubernetes_version (string)
  - cluster_defaults.cni (string: calico, cilium, kube-ovn)
  - cluster_defaults.ssh_user (string)`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			// Create config manager
			cm, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Convert string value to appropriate type based on the key
			convertedValue, err := convertConfigValue(key, value)
			if err != nil {
				return fmt.Errorf("invalid value for key '%s': %w", key, err)
			}

			// Set the value
			if err := cm.SetValue(key, convertedValue); err != nil {
				return fmt.Errorf("failed to set configuration value: %w", err)
			}

			// Save the configuration
			if err := cm.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Configuration updated: %s = %v\n", key, convertedValue)
			return nil
		},
	}
}

// newConfigGetCmd creates the "config get" command to retrieve configuration values.
func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value using dot notation",
		Long: `Get a configuration value using dot notation.

Examples:
  opencenter config get logging.level
  opencenter config get paths.clustersDir
  opencenter config get paths.pluginsDir
  opencenter config get paths.stateDir
  opencenter config get behavior.autoConfirm

Use dot notation to access nested configuration values. If the key doesn't exist,
an error will be returned.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Create a new config manager
			cm, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Get the value
			value, err := cm.GetValue(key)
			if err != nil {
				return fmt.Errorf("failed to get configuration value: %w", err)
			}

			// Format the output based on the value type
			switch v := value.(type) {
			case string:
				fmt.Println(v)
			case bool:
				fmt.Println(v)
			case int:
				fmt.Println(v)
			default:
				// For complex types, marshal to YAML
				data, err := yaml.Marshal(v)
				if err != nil {
					return fmt.Errorf("failed to format value: %w", err)
				}
				fmt.Print(string(data))
			}

			return nil
		},
	}
}

// newConfigResetCmd creates the "config reset" command to restore default configuration.
func newConfigResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to default values",
		Long: `Reset the CLI configuration to default values.

This will overwrite the current configuration file with default values.
All custom settings will be lost.

Default values:
  - logging.level: warn
  - logging.format: text
  - logging.output: stderr
  - paths.configDir: ~/.config/opencenter
  - paths.clustersDir: ~/.config/opencenter/clusters
  - paths.pluginsDir: ~/.config/opencenter/plugins
  - paths.stateDir: ~/.local/state/opencenter
  - behavior.autoConfirm: false
  - behavior.dryRun: false
  - cluster_defaults.provider: openstack
  - cluster_defaults.region: dfw3
  - cluster_defaults.environment: dev
  - cluster_defaults.gitops_auth_method: token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create config manager
			cm, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Reset to defaults
			if err := cm.Reset(); err != nil {
				return fmt.Errorf("failed to reset configuration: %w", err)
			}

			fmt.Println("Configuration reset to default values")
			return nil
		},
	}
}

// newConfigPathCmd creates the "config path" command to show the configuration file location.
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show the path to the configuration file",
		Long: `Show the absolute path to the CLI configuration file.

The configuration file location is determined by:
1. OPENCENTER_CONFIG_DIR environment variable (if set)
2. Default OS-specific config directory (~/.config/opencenter on Linux/macOS)

The configuration file is named 'config.yaml' within the configuration directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create config manager to get the path
			cm, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to determine configuration path: %w", err)
			}

			fmt.Println(cm.GetConfigPath())
			return nil
		},
	}
}

// convertConfigValue converts a string value to the appropriate type based on the configuration key.
func convertConfigValue(key, value string) (interface{}, error) {
	// Determine the expected type based on the key
	switch {
	case key == "logging.level":
		// Validate log level
		validLevels := []string{"debug", "info", "warn", "error"}
		for _, level := range validLevels {
			if value == level {
				return value, nil
			}
		}
		return nil, fmt.Errorf("invalid log level '%s', must be one of: %s", value, strings.Join(validLevels, ", "))
	case key == "logging.format":
		// Validate log format
		validFormats := []string{"text", "json", "yaml"}
		for _, format := range validFormats {
			if value == format {
				return value, nil
			}
		}
		return nil, fmt.Errorf("invalid log format '%s', must be one of: %s", value, strings.Join(validFormats, ", "))
	case key == "logging.output":
		// Validate log output (stdout, stderr, or file path)
		if value == "stdout" || value == "stderr" {
			return value, nil
		}
		// For file paths, just return as string - validation will happen in ConfigManager
		return value, nil
	case key == "behavior.validation":
		mode := strings.ToLower(strings.TrimSpace(value))
		if err := config.ValidateBehaviorValidationMode(mode); err != nil {
			return nil, err
		}
		return mode, nil
	case key == "cluster_defaults.gitops_auth_method":
		method := strings.ToLower(strings.TrimSpace(value))
		if err := config.ValidateGitopsAuthMethod(method); err != nil {
			return nil, err
		}
		return method, nil
	case strings.HasSuffix(key, ".compress") ||
		strings.HasPrefix(key, "behavior."):
		// Boolean fields
		switch strings.ToLower(value) {
		case "true", "yes", "1", "on":
			return true, nil
		case "false", "no", "0", "off":
			return false, nil
		default:
			return nil, fmt.Errorf("expected boolean value (true/false), got: %s", value)
		}
	case strings.HasSuffix(key, ".maxSize") ||
		strings.HasSuffix(key, ".maxBackups") ||
		strings.HasSuffix(key, ".maxAge"):
		// Integer fields
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err != nil {
			return nil, fmt.Errorf("expected integer value, got: %s", value)
		}
		return intValue, nil
	default:
		// String fields (default)
		return value, nil
	}
}
