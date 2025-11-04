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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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

// setMapField sets a field in a map using dot-notation path, similar to setField but for maps
func setMapField(obj map[string]any, path string, value string) error {
	parts := strings.Split(path, ".")
	current := obj

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part, set the value
			current[part] = convertStringValue(value)
			return nil
		}

		// Navigate deeper
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]any); ok {
				current = nextMap
			} else {
				return fmt.Errorf("field '%s' is not a map, cannot traverse further", part)
			}
		} else {
			// Create new map
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}
	return nil
}

// convertStringValue converts a string to the appropriate type (string, int, bool)
func convertStringValue(value string) any {
	// Try to parse as bool
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	// Try to parse as int
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	// Default to string
	return value
}

func newClusterInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Initialize a new cluster configuration (non-interactive)",
		Args:  cobra.ExactArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// name is required as a positional argument (initial seed)
			name := args[0]

			// Generate configuration using schema-based defaults to match testdata/schema.yaml structure
			schemaDefaultYAML, err := config.GenerateDefaultFromSchema(name)
			if err != nil {
				return fmt.Errorf("failed to generate schema-based defaults: %w", err)
			}

			var configMap map[string]any
			if err := yaml.Unmarshal(schemaDefaultYAML, &configMap); err != nil {
				return fmt.Errorf("failed to parse schema defaults to map: %w", err)
			}

			// Also create a struct version for validation (unmarshal the schema defaults into a Config struct)
			cfg := config.Config{}
			if err := yaml.Unmarshal(schemaDefaultYAML, &cfg); err != nil {
				return fmt.Errorf("failed to parse schema defaults to struct: %w", err)
			}

			// Apply overrides from flags to the config struct (for validation) and map (for output)
			// We parse os.Args manually here because cobra does not support
			// unknown flags in a way that allows us to capture them.
			// FParseErrWhitelist.UnknownFlags = true makes cobra ignore them,
			// but it does not provide a way to access them.
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "--") {
					parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
					if len(parts) == 2 {
						key, value := parts[0], parts[1]
						// Skip flags that are handled by cobra
						if cmd.Flags().Lookup(key) != nil {
							continue
						}
						if err := setField(&cfg, key, value); err != nil {
							return fmt.Errorf("error setting config from flag '%s': %w", key, err)
						}
						// Also apply to the map for final output
						if err := setMapField(configMap, key, value); err != nil {
							return fmt.Errorf("error setting config map from flag '%s': %w", key, err)
						}
					}
				}
			}

			// Interactive wizard has been removed; name must be provided

			// Use the name parameter directly since cluster_name is no longer top-level
			// The name is set from the positional argument and may be overridden by flags

			// Handle --force
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				// Check if cluster directory exists instead of just the config file
				clusterDir, err := config.ClusterDirectoryPath(name)
				if err == nil {
					if _, err := os.Stat(clusterDir); err == nil {
						return fmt.Errorf("cluster configuration directory '%s' already exists, use --force to overwrite", name)
					}
				}
			}

			// Handle --strict
			strict, _ := cmd.Flags().GetBool("strict")
			if strict {
				if errs := config.Validate(cfg); len(errs) > 0 {
					for _, e := range errs {
						fmt.Fprintln(cmd.ErrOrStderr(), e)
					}
					return fmt.Errorf("validation failed")
				}
			}

			// Persist config
			// If no SOPS key location provided, generate one named after cluster
			disableKeygen, _ := cmd.Flags().GetBool("no-sops-keygen")
			if !disableKeygen && cfg.Secrets.SopsAgeKeyFile == "" && name != "" {
				if err := generateDefaultSOPSKey(name, &cfg); err != nil {
					return fmt.Errorf("failed to generate default SOPS key: %w", err)
				}
			}

			// Update the map with any SOPS key changes from the struct
			if cfg.Secrets.SopsAgeKeyFile != "" {
				if secretsMap, ok := configMap["secrets"].(map[string]any); ok {
					secretsMap["sops_age_key_file"] = cfg.Secrets.SopsAgeKeyFile
				} else {
					// Create secrets map if it doesn't exist
					configMap["secrets"] = map[string]any{
						"sops_age_key_file": cfg.Secrets.SopsAgeKeyFile,
					}
				}
			}

			// Convert the map to YAML for final output
			finalYAML, err := yaml.Marshal(configMap)
			if err != nil {
				return fmt.Errorf("failed to marshal final config: %w", err)
			}

			// Get the config path (this will create the cluster directory structure)
			path, err := config.ConfigPath(name)
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}

			// Write the config file with proper permissions (0600 for files)
			if err := os.WriteFile(path, finalYAML, 0o600); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created cluster configuration directory and file for %s\n", name)
			return nil
		},
	}
	cmd.Flags().Bool("strict", false, "fail if required values are missing")
	cmd.Flags().Bool("force", false, "overwrite existing file")
	cmd.Flags().Bool("no-sops-keygen", false, "do not auto-generate a SOPS age key when secrets.sops_age_key_file is unset")
	return cmd
}

// generateDefaultSOPSKey creates an age key file under the cluster-specific secrets directory
// at <cluster-dir>/secrets/age/keys/<cluster>-key.txt and updates cfg.Secrets.SopsAgeKeyFile
// to point to the generated file. The key is a placeholder that starts with
// AGE-SECRET-KEY-1 followed by random bytes; file perms are 0600.
func generateDefaultSOPSKey(cluster string, cfg *config.Config) error {
	// Get the cluster-specific secrets directory path
	secretsDir, err := config.ClusterSecretsPath(cluster)
	if err != nil {
		return fmt.Errorf("failed to get cluster secrets path: %w", err)
	}
	
	// Create the secrets directory with proper permissions (0755 for directories)
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cluster secrets directory: %w", err)
	}
	
	// Generate the key file path
	keyFile := filepath.Join(secretsDir, fmt.Sprintf("%s-key.txt", cluster))
	
	// Generate a key
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}
	key := fmt.Sprintf("AGE-SECRET-KEY-1%s\n", hex.EncodeToString(b[:]))
	
	// Write the key file with proper permissions (0600 for files)
	if err := os.WriteFile(keyFile, []byte(key), 0o600); err != nil {
		return fmt.Errorf("failed to write SOPS key file: %w", err)
	}
	
	cfg.Secrets.SopsAgeKeyFile = keyFile
	return nil
}
