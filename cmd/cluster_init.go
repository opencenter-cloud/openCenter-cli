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
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/sops"
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
		Long: `Initialize a new cluster configuration with default values.

This command creates a new cluster configuration file with sensible defaults
based on the JSON schema. You can override any configuration value using
command-line flags with dot notation.

The configuration is created in an organization-based directory structure:
  ~/.config/openCenter/clusters/<organization>/<cluster>/

By default, the cluster name is used as the organization name. Use --org to
specify a different organization.

A SOPS Age encryption key is automatically generated unless --no-sops-keygen
is specified. The key is stored in the cluster's secrets directory.

Configuration Override:
  Use --org flag or dot notation to set organization:
    --org myorg
    --opencenter.meta.organization=myorg
  
  Use dot notation to override any configuration value:
    --opencenter.meta.env=prod
    --opencenter.cluster.kubernetes.version=1.31.4
    --opencenter.infrastructure.provider=aws

Troubleshooting:
  • If cluster already exists, use --force to overwrite
  • Use --strict to enable validation during initialization
  • Check ~/.config/openCenter/clusters/ for created files`,
		Example: `  # Initialize with defaults (uses cluster name as organization)
  openCenter cluster init my-cluster

  # Initialize with organization using --org flag
  openCenter cluster init my-cluster --org myorg

  # Initialize with organization using dot notation
  openCenter cluster init my-cluster --opencenter.meta.organization=myorg

  # Initialize with custom values
  openCenter cluster init my-cluster \
    --org production \
    --opencenter.meta.env=prod \
    --opencenter.cluster.kubernetes.version=1.31.4 \
    --opencenter.infrastructure.provider=aws

  # Initialize without SOPS key generation
  openCenter cluster init my-cluster --no-sops-keygen

  # Force overwrite existing configuration
  openCenter cluster init my-cluster --force

  # Initialize with strict validation
  openCenter cluster init my-cluster --strict`,
		Args:  cobra.ExactArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// name is required as a positional argument (initial seed)
			name := args[0]

			// Initialize CLI configuration manager
			configManager, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to initialize configuration manager: %w", err)
			}

			// Initialize path resolver
			pathResolver := config.NewPathResolver(configManager)

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

			// Determine organization from --org flag, configuration, or use cluster name as default
			orgFlag, _ := cmd.Flags().GetString("org")
			organization := orgFlag
			if organization == "" {
				organization = cfg.OpenCenter.Meta.Organization
			}
			if organization == "" {
				organization = name
			}

			// Always update configuration with the determined organization
			cfg.OpenCenter.Meta.Organization = organization
			// Also update the map
			if opencenter, ok := configMap["opencenter"].(map[string]any); ok {
				if meta, ok := opencenter["meta"].(map[string]any); ok {
					meta["organization"] = organization
				} else {
					opencenter["meta"] = map[string]any{
						"organization": organization,
					}
				}
			} else {
				configMap["opencenter"] = map[string]any{
					"meta": map[string]any{
						"organization": organization,
					},
				}
			}

			// Resolve cluster paths using organization structure
			clusterPaths := pathResolver.ResolveClusterPaths(name, organization)

			// Handle --force
			force, _ := cmd.Flags().GetBool("force")
			
			// Check if cluster directory exists in organization structure
			if _, err := os.Stat(clusterPaths.ClusterDir); err == nil {
				if !force {
					return fmt.Errorf("cluster configuration directory '%s' already exists in organization '%s', use --force to overwrite", name, organization)
				}
				
				// Force flag is set, perform cleanup and overwrite
				if err := cleanupClusterDirectory(clusterPaths.ClusterDir); err != nil {
					return fmt.Errorf("failed to cleanup existing cluster directory '%s': %w", clusterPaths.ClusterDir, err)
				}
			}

			// Validate organization name for directory creation
			if err := validateOrganizationName(organization); err != nil {
				return fmt.Errorf("invalid organization name '%s': %w", organization, err)
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

			// Create organization structure
			if err := pathResolver.CreateOrganizationStructure(organization); err != nil {
				return fmt.Errorf("failed to create organization structure: %w", err)
			}

			// Create cluster directories
			if err := pathResolver.CreateClusterDirectories(name, organization); err != nil {
				return fmt.Errorf("failed to create cluster directories: %w", err)
			}

			// Persist config
			// If no SOPS key location provided, generate one using organization structure
			disableKeygen, _ := cmd.Flags().GetBool("no-sops-keygen")
			if !disableKeygen && cfg.Secrets.SopsAgeKeyFile == "" && name != "" {
				if err := generateOrganizationSOPSKey(name, organization, &cfg, pathResolver); err != nil {
					return fmt.Errorf("failed to generate organization SOPS key: %w", err)
				}
			}

			// Update GitOps directory to point to organization root
			// Check if user explicitly set a custom git_dir via command line flags
			userSetCustomGitDir := false
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "--opencenter.gitops.git_dir=") || strings.HasPrefix(arg, "--gitops.git_dir=") {
					userSetCustomGitDir = true
					break
				}
			}
			
			// If user didn't explicitly set a custom git_dir, use organization-based path
			if !userSetCustomGitDir {
				cfg.OpenCenter.GitOps.GitDir = clusterPaths.GitOpsDir
				if opencenter, ok := configMap["opencenter"].(map[string]any); ok {
					if gitops, ok := opencenter["gitops"].(map[string]any); ok {
						gitops["git_dir"] = clusterPaths.GitOpsDir
					} else {
						opencenter["gitops"] = map[string]any{
							"git_dir": clusterPaths.GitOpsDir,
						}
					}
				}
			}
			// If user specified a custom git_dir, keep it as-is

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

			// Get the config path at organization level
			configPath := filepath.Join(clusterPaths.OrganizationDir, "."+name+"-config.yaml")

			// Write the config file with proper permissions (0600 for files)
			if err := os.WriteFile(configPath, finalYAML, 0o600); err != nil {
				return fmt.Errorf("failed to write cluster configuration file to '%s': %w", configPath, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created cluster configuration in organization '%s' at '%s'\n", organization, clusterPaths.ClusterDir)
			fmt.Fprintf(cmd.OutOrStdout(), "GitOps repository root: %s\n", clusterPaths.GitOpsDir)
			fmt.Fprintf(cmd.OutOrStdout(), "SOPS key location: %s\n", clusterPaths.SOPSKeyPath)
			return nil
		},
	}
	cmd.Flags().String("org", "", "organization name (defaults to cluster name if not specified)")
	cmd.Flags().Bool("strict", false, "fail if required values are missing")
	cmd.Flags().Bool("force", false, "overwrite existing file")
	cmd.Flags().Bool("no-sops-keygen", false, "do not auto-generate a SOPS age key when secrets.sops_age_key_file is unset")
	return cmd
}

// cleanupClusterDirectory removes the existing cluster directory and all its contents
// to prepare for overwriting with new configuration when --force flag is used.
func cleanupClusterDirectory(clusterDir string) error {
	// Remove the entire cluster directory and all its contents
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("failed to remove existing cluster directory and contents: %w", err)
	}
	return nil
}

// generateDefaultSOPSKey creates an age key file under the cluster-specific secrets directory
// at <cluster-dir>/secrets/age/keys/<cluster>-key.txt and updates cfg.Secrets.SopsAgeKeyFile
// to point to the generated file using the new SOPS key manager.
func generateDefaultSOPSKey(cluster string, cfg *config.Config) error {
	// Get the cluster-specific secrets directory path
	secretsDir, err := config.ClusterSecretsPath(cluster)
	if err != nil {
		return fmt.Errorf("failed to get cluster secrets directory path: %w", err)
	}
	
	// Create the secrets directory with proper permissions (0755 for directories)
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cluster secrets directory '%s': %w", secretsDir, err)
	}
	
	// Generate the key file path
	keyFile := filepath.Join(secretsDir, fmt.Sprintf("%s-key.txt", cluster))
	
	// Use the new SOPS key manager to generate a proper Age key
	km := sops.NewKeyManager(secretsDir)
	keyPair, err := km.GenerateAgeKey()
	if err != nil {
		return fmt.Errorf("failed to generate Age key pair: %w", err)
	}
	
	// Write the private key file with proper permissions (0600 for files)
	// Ensure the key ends with a newline for proper format
	keyContent := keyPair.PrivateKey
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}
	if err := os.WriteFile(keyFile, []byte(keyContent), 0o600); err != nil {
		return fmt.Errorf("failed to write SOPS key file to cluster directory '%s': %w", keyFile, err)
	}
	
	cfg.Secrets.SopsAgeKeyFile = keyFile
	return nil
}

// generateOrganizationSOPSKey creates an age key file using the organization-based directory structure
// and updates cfg.Secrets.SopsAgeKeyFile to point to the generated file.
func generateOrganizationSOPSKey(cluster, organization string, cfg *config.Config, pathResolver *config.PathResolver) error {
	// Resolve cluster paths for the organization
	clusterPaths := pathResolver.ResolveClusterPaths(cluster, organization)
	
	// Create the secrets directory with proper permissions (0755 for directories)
	secretsKeyDir := filepath.Dir(clusterPaths.SOPSKeyPath)
	if err := os.MkdirAll(secretsKeyDir, 0o755); err != nil {
		return fmt.Errorf("failed to create organization secrets directory '%s': %w", secretsKeyDir, err)
	}
	
	// Use the new SOPS key manager to generate a proper Age key
	km := sops.NewKeyManager(secretsKeyDir)
	keyPair, err := km.GenerateAgeKey()
	if err != nil {
		return fmt.Errorf("failed to generate Age key pair: %w", err)
	}
	
	// Write the private key file with proper permissions (0600 for files)
	// Ensure the key ends with a newline for proper format
	keyContent := keyPair.PrivateKey
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}
	if err := os.WriteFile(clusterPaths.SOPSKeyPath, []byte(keyContent), 0o600); err != nil {
		return fmt.Errorf("failed to write SOPS key file to organization directory '%s': %w", clusterPaths.SOPSKeyPath, err)
	}
	
	// Create or update the SOPS configuration file for the organization
	if err := createOrganizationSOPSConfig(clusterPaths.SOPSConfigPath, keyPair.PublicKey); err != nil {
		return fmt.Errorf("failed to create organization SOPS config: %w", err)
	}
	
	cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
	return nil
}

// createOrganizationSOPSConfig creates or updates the .sops.yaml configuration file for the organization.
func createOrganizationSOPSConfig(sopsConfigPath, publicKey string) error {
	// Create a proper SOPS configuration with the Age public key
	sopsConfig := fmt.Sprintf(`creation_rules:
  - path_regex: .*\.yaml$
    age: >-
      %s
  - path_regex: .*\.json$
    age: >-
      %s
`, publicKey, publicKey)

	// Write the SOPS configuration file
	if err := os.WriteFile(sopsConfigPath, []byte(sopsConfig), 0o600); err != nil {
		return fmt.Errorf("failed to write SOPS config file: %w", err)
	}
	
	return nil
}

// validateOrganizationName validates that an organization name is safe for use as a directory name.
func validateOrganizationName(organization string) error {
	if organization == "" {
		return fmt.Errorf("organization name cannot be empty")
	}
	
	// Use the same validation as cluster names since they both become directory names
	return config.ValidateClusterName(organization)
}
