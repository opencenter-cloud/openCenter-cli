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
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/di"
	"github.com/rackerlabs/opencenter-cli/internal/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newClusterInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new cluster configuration (non-interactive)",
		Long: `Initialize a new cluster configuration with default values.

This command creates a new cluster configuration file with sensible defaults
based on the JSON schema. You can override any configuration value using
command-line flags with dot notation.

This command generates v2 configuration (schema_version: "2.0") only.
For v1 configurations, use 'cluster migrate-config' to migrate existing v1 configs to v2.

The configuration is created in an organization-based directory structure:
  ~/.config/opencenter/clusters/<organization>/<cluster>/

By default, the organization is set to "opencenter". Use --org to specify a different organization.

SOPS Age encryption keys and SSH key pairs are automatically generated if they
don't already exist, unless --no-keygen is specified.`,
		Example: `  # Initialize with defaults
  opencenter cluster init my-cluster

  # Initialize from existing config file
  opencenter cluster init --config my-cluster-config.yaml

  # Initialize with organization
  opencenter cluster init my-cluster --org myorg

  # Initialize with custom values
  opencenter cluster init my-cluster --org production --type aws

  # Initialize without key generation
  opencenter cluster init my-cluster --no-keygen

  # Overwrite existing config
  opencenter cluster init my-cluster --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterInit,
	}

	cmd.Flags().String("org", "", "organization name (defaults to 'opencenter')")
	cmd.Flags().String("type", "openstack", "cluster type: openstack, baremetal, kind, vmware")
	cmd.Flags().String("schema-version", "2.0", "configuration schema version (v2 only)")
	cmd.Flags().String("config", "", "load configuration from file")
	cmd.Flags().Bool("strict", false, "fail if required values are missing")
	cmd.Flags().Bool("force", false, "overwrite existing config file")
	cmd.Flags().Bool("no-keygen", false, "do not auto-generate keys")
	cmd.Flags().Bool("no-sops-keygen", false, "do not auto-generate SOPS keys")
	cmd.Flags().Bool("regenerate-keys", false, "regenerate keys even if they exist")
	cmd.Flags().Bool("full-schema", false, "generate configuration with all available fields")
	cmd.Flags().StringArray("server-pool", []string{}, "additional server pool configuration")

	return cmd
}

func runClusterInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize DI container
	container := di.NewContainer()
	if err := setupContainer(container); err != nil {
		return fmt.Errorf("setting up DI container: %w", err)
	}

	// Resolve InitService from container
	var initService *cluster.InitService
	if err := container.ResolveAs("init-service", &initService); err != nil {
		return fmt.Errorf("resolving init service: %w", err)
	}

	// Parse command-line options
	opts, err := parseInitOptions(cmd, args)
	if err != nil {
		return err
	}

	// Execute initialization
	result, err := initService.Initialize(ctx, opts)
	if err != nil {
		return err
	}

	// Display results
	fmt.Fprint(cmd.OutOrStdout(), result.Message)
	return nil
}

// setupContainer initializes the DI container with all required services
func setupContainer(container di.Container) error {
	pathResolver, err := di.ProvidePathResolver("")
	if err != nil {
		return err
	}
	if err := container.Singleton("path-resolver", func() (*paths.PathResolver, error) {
		return pathResolver, nil
	}); err != nil {
		return err
	}
	if err := container.Singleton("config-manager", di.ProvideConfigManager); err != nil {
		return err
	}
	if err := container.Singleton("validation-engine", di.ProvideValidationEngine); err != nil {
		return err
	}
	if err := container.Singleton("init-service", di.ProvideInitService); err != nil {
		return err
	}
	return container.Initialize()
}

// parseInitOptions parses command-line flags into InitOptions
func parseInitOptions(cmd *cobra.Command, args []string) (cluster.InitOptions, error) {
	opts := cluster.InitOptions{}

	// Get cluster name from args, config file, or active cluster
	if len(args) > 0 {
		opts.ClusterName = args[0]
	} else if configFile, _ := cmd.Flags().GetString("config"); configFile != "" {
		name, err := extractClusterNameFromConfig(configFile)
		if err != nil {
			return opts, err
		}
		opts.ClusterName = name
		opts.ConfigFile = configFile
	} else {
		activeName, err := config.GetActive()
		if err != nil || activeName == "" {
			return opts, fmt.Errorf("no cluster name provided and no active cluster set")
		}
		opts.ClusterName = activeName
	}

	// Parse flags
	opts.Organization, _ = cmd.Flags().GetString("org")
	opts.Provider, _ = cmd.Flags().GetString("type")
	opts.Force, _ = cmd.Flags().GetBool("force")
	opts.Strict, _ = cmd.Flags().GetBool("strict")
	opts.NoKeyGen, _ = cmd.Flags().GetBool("no-keygen")
	opts.NoSOPSKeyGen, _ = cmd.Flags().GetBool("no-sops-keygen")
	opts.RegenerateKeys, _ = cmd.Flags().GetBool("regenerate-keys")
	opts.FullSchema, _ = cmd.Flags().GetBool("full-schema")
	opts.SchemaVersion, _ = cmd.Flags().GetString("schema-version")
	opts.ServerPools, _ = cmd.Flags().GetStringArray("server-pool")

	if opts.SchemaVersion != "2.0" {
		return opts, fmt.Errorf("invalid schema version '%s': only v2.0 is supported", opts.SchemaVersion)
	}

	return opts, nil
}

// extractClusterNameFromConfig extracts the cluster name from a config file
func extractClusterNameFromConfig(configFile string) (string, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return "", fmt.Errorf("reading config file: %w", err)
	}

	var tempCfg struct {
		OpenCenter struct {
			Cluster struct {
				ClusterName string `yaml:"cluster_name"`
			} `yaml:"cluster"`
		} `yaml:"opencenter"`
	}

	if err := yaml.Unmarshal(data, &tempCfg); err != nil {
		return "", fmt.Errorf("parsing config file: %w", err)
	}

	if tempCfg.OpenCenter.Cluster.ClusterName == "" {
		return "", fmt.Errorf("cluster name not found in config file")
	}

	return tempCfg.OpenCenter.Cluster.ClusterName, nil
}

// setField sets a field in a struct using a dot-notation path.
// It uses reflection to traverse the struct and set the value.
func setField(obj any, path string, value string) error {
	v := reflect.ValueOf(obj).Elem() // We expect a pointer to a struct
	parts := strings.Split(path, ".")

	for i, part := range parts {
		// Find field by yaml tag
		field := util.FindField(v, part)

		if !field.IsValid() {
			// If field is not found, check if the current value is a map.
			// If so, the 'part' might be a key in the map.
			if v.Kind() == reflect.Map {
				// This should be the last part of the path, representing the map key.
				if i != len(parts)-1 {
					return fmt.Errorf("setting nested fields in maps is not supported: %s", path)
				}

				// Ensure map key is a string
				if v.Type().Key().Kind() != reflect.String {
					return fmt.Errorf("map key type must be string for path-based setting, got %s", v.Type().Key().Kind())
				}

				// Get map value type, create a new value, and set it.
				mapValueType := v.Type().Elem()
				newValue := reflect.New(mapValueType).Elem()
				if err := setReflectValue(newValue, value); err != nil {
					return fmt.Errorf("failed to set map value for key '%s': %w", part, err)
				}

				// Set the key-value pair in the map.
				v.SetMapIndex(reflect.ValueOf(part), newValue)
				return nil
			}
			return fmt.Errorf("field not found: '%s' in struct '%s'", part, v.Type().Name())
		}

		// If this is the last part of the path, set the field's value.
		if i == len(parts)-1 {
			return setFieldValue(field, value)
		}

		// If not the last part, we need to traverse deeper.
		// The field must be a struct, a pointer to a struct, or a map.
		if field.Kind() == reflect.Struct {
			v = field
		} else if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			v = field.Elem()
		} else if field.Kind() == reflect.Map {
			if field.IsNil() {
				field.Set(reflect.MakeMap(field.Type()))
			}
			v = field
		} else {
			return fmt.Errorf("field '%s' is not a struct or map, cannot traverse further", part)
		}
	}
	return nil
}

// setFieldValue sets a reflect.Value from a string, with type conversion.
func setFieldValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set field value")
	}
	return setReflectValue(field, value)
}

// setReflectValue converts string value to the field's type and sets it.
func setReflectValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: '%s'", value)
		}
		field.SetInt(i)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: '%s'", value)
		}
		field.SetBool(b)
	case reflect.Interface:
		// Handle interface{} types by storing the appropriately converted value
		if b, err := strconv.ParseBool(value); err == nil {
			field.Set(reflect.ValueOf(b))
		} else if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			field.Set(reflect.ValueOf(i))
		} else {
			field.Set(reflect.ValueOf(value))
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}
	return nil
}
