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
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/config/flags"
	"github.com/rackerlabs/openCenter-cli/internal/sops"
	"github.com/rackerlabs/openCenter-cli/internal/util"
	"github.com/rackerlabs/openCenter-cli/internal/util/crypto"
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
		Use:   "init [name]",
		Short: "Initialize a new cluster configuration (non-interactive)",
		Long: `Initialize a new cluster configuration with default values.

This command creates a new cluster configuration file with sensible defaults
based on the JSON schema. You can override any configuration value using
command-line flags with dot notation.

The configuration is created in an organization-based directory structure:
  ~/.config/openCenter/clusters/<organization>/<cluster>/

	By default, the organization is set to "opencenter". Use --org to
	specify a different organization.

SOPS Age encryption keys and SSH key pairs are automatically generated if they
don't already exist, unless --no-keygen is specified. Use --regenerate-keys to
force regeneration of existing keys. Keys are stored in the cluster's secrets
directory.

Configuration Override:
  Use --org flag or dot notation to set organization:
    --org myorg
    --opencenter.meta.organization=myorg
  
  Use --type flag to specify cluster type:
    --type baremetal
    --type openstack (default)
  
  Use dot notation to override any configuration value:
    --opencenter.meta.env=prod
    --opencenter.cluster.kubernetes.version=1.31.4
    --opencenter.infrastructure.provider=aws

Troubleshooting:
  • If cluster already exists, use --force to overwrite
  • Use --strict to enable validation during initialization
  • Check ~/.config/openCenter/clusters/ for created files`,
		Example: `  # Initialize with defaults (uses "opencenter" as organization)
	  openCenter cluster init my-cluster

  # Initialize bare metal cluster
  openCenter cluster init my-cluster --org myorg --type baremetal

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

  # Initialize without key generation (SOPS and SSH)
  openCenter cluster init my-cluster --no-keygen

  # Regenerate keys even if they already exist
  openCenter cluster init my-cluster --regenerate-keys

  # Force overwrite existing configuration
  openCenter cluster init my-cluster --force

  # Force overwrite active cluster configuration
  openCenter cluster init --force

  # Initialize with strict validation
  openCenter cluster init my-cluster --strict`,
		Args: cobra.MaximumNArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				// No name provided, try to use active cluster
				activeName, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if activeName == "" {
					return fmt.Errorf("no cluster name provided and no active cluster set; specify a cluster name or use 'openCenter cluster use <name>' to set an active cluster")
				}
				name = activeName
			}

			// Initialize CLI configuration manager
			configManager, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to initialize configuration manager: %w", err)
			}

			// Initialize path resolver
			pathResolver := config.NewPathResolverImpl(configManager)

			// Check if a configuration file was provided via --config flag
			var cfg config.Config
			var configMap map[string]any
			
			configFile, _ := cmd.Flags().GetString("config")
			if configFile != "" {
				// Load configuration from the specified file
				data, err := os.ReadFile(configFile)
				if err != nil {
					return fmt.Errorf("failed to read configuration file '%s': %w", configFile, err)
				}
				
				loader := config.NewConfigLoader(pathResolver)
				loadedCfg, err := loader.LoadFromBytes(context.Background(), data, name)
				if err != nil {
					return fmt.Errorf("failed to load configuration from file '%s': %w", configFile, err)
				}
				cfg = *loadedCfg
				
				// Convert loaded config to map for YAML output
				cfgYAML, err := yaml.Marshal(cfg)
				if err != nil {
					return fmt.Errorf("failed to marshal loaded configuration: %w", err)
				}
				if err := yaml.Unmarshal(cfgYAML, &configMap); err != nil {
					return fmt.Errorf("failed to parse loaded configuration to map: %w", err)
				}
			} else {
				// Generate configuration using schema-based defaults to match testdata/schema.yaml structure
				schemaDefaultYAML, err := config.GenerateDefaultFromSchema(name)
				if err != nil {
					return fmt.Errorf("failed to generate schema-based defaults: %w", err)
				}

				if err := yaml.Unmarshal(schemaDefaultYAML, &configMap); err != nil {
					return fmt.Errorf("failed to parse schema defaults to map: %w", err)
				}

				// Also create a struct version for validation (unmarshal the schema defaults into a Config struct)
				if err := yaml.Unmarshal(schemaDefaultYAML, &cfg); err != nil {
					return fmt.Errorf("failed to parse schema defaults to struct: %w", err)
				}
			}

			// Apply overrides from flags to the config struct (for validation) and map (for output)
			// Use enhanced flag parser for better flag handling while maintaining backward compatibility
			flagIntegration, err := flags.NewCLIIntegration()
			if err != nil {
				return fmt.Errorf("failed to initialize flag integration: %w", err)
			}

			// Filter out cobra-handled flags from os.Args
			var filteredArgs []string
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "--") {
					parts := strings.SplitN(strings.TrimPrefix(arg, "--"), "=", 2)
					if len(parts) == 2 {
						key := parts[0]
						// Skip flags that are handled by cobra
						if cmd.Flags().Lookup(key) != nil {
							continue
						}
						filteredArgs = append(filteredArgs, arg)
					}
				}
			}

			// Process flags using enhanced parser
			if err := flagIntegration.ProcessFlags(filteredArgs, &cfg, configMap); err != nil {
				return fmt.Errorf("error processing flags: %w", err)
			}

			// Determine organization from --org flag, configuration, or use cluster name as default
			orgFlag, _ := cmd.Flags().GetString("org")
			organization := orgFlag
			if organization == "" {
				organization = cfg.OpenCenter.Meta.Organization
			}
			if organization == "" {
				organization = "opencenter"
			}

			// Always update configuration with the determined organization
			cfg.OpenCenter.Meta.Organization = organization

			// Set initial stage and status
			cfg.OpenCenter.Meta.Stage = config.StageInit
			cfg.OpenCenter.Meta.Status = config.StatusSuccess

			// Handle --type flag to set infrastructure provider
			typeFlag, _ := cmd.Flags().GetString("type")
			if typeFlag != "" {
				cfg.OpenCenter.Infrastructure.Provider = typeFlag
				// Also update the map
				if opencenter, ok := configMap["opencenter"].(map[string]any); ok {
					if infrastructure, ok := opencenter["infrastructure"].(map[string]any); ok {
						infrastructure["provider"] = typeFlag
					} else {
						opencenter["infrastructure"] = map[string]any{
							"provider": typeFlag,
						}
					}
				}
			}

			// Handle --server-pool flags to set additional server pools
			serverPools, _ := cmd.Flags().GetStringArray("server-pool")
			if len(serverPools) > 0 {
				var additionalServerPools []config.AdditionalServerPool
				for _, poolStr := range serverPools {
					pool, err := parseServerPoolString(poolStr)
					if err != nil {
						return fmt.Errorf("failed to parse server pool '%s': %w", poolStr, err)
					}
					additionalServerPools = append(additionalServerPools, pool)
				}
				
				// Update the configuration struct
				cfg.OpenCenter.Cluster.Kubernetes.AdditionalServerPoolsWorker = additionalServerPools
				
				// Also update the map for consistency
				if opencenter, ok := configMap["opencenter"].(map[string]any); ok {
					if cluster, ok := opencenter["cluster"].(map[string]any); ok {
						if kubernetes, ok := cluster["kubernetes"].(map[string]any); ok {
							kubernetes["additional_server_pools_worker"] = convertServerPoolsToMap(additionalServerPools)
						} else {
							cluster["kubernetes"] = map[string]any{
								"additional_server_pools_worker": convertServerPoolsToMap(additionalServerPools),
							}
						}
					} else {
						opencenter["cluster"] = map[string]any{
							"kubernetes": map[string]any{
								"additional_server_pools_worker": convertServerPoolsToMap(additionalServerPools),
							},
						}
					}
				}
			}
			// Also update the map
			if opencenter, ok := configMap["opencenter"].(map[string]any); ok {
				if meta, ok := opencenter["meta"].(map[string]any); ok {
					meta["organization"] = organization
					meta["stage"] = config.StageInit
					meta["status"] = config.StatusSuccess
				} else {
					opencenter["meta"] = map[string]any{
						"organization": organization,
						"stage":        config.StageInit,
						"status":       config.StatusSuccess,
					}
				}
			} else {
				configMap["opencenter"] = map[string]any{
					"meta": map[string]any{
						"organization": organization,
						"stage":        config.StageInit,
						"status":       config.StatusSuccess,
					},
				}
			}

			// Resolve cluster paths using organization structure
			clusterPaths, err := pathResolver.ResolveClusterPaths(context.Background(), name, organization)
			if err != nil {
				return fmt.Errorf("failed to resolve cluster paths: %w", err)
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

			// Handle --force
			force, _ := cmd.Flags().GetBool("force")

			// Check if cluster directory exists in organization structure
			if _, err := os.Stat(clusterPaths.ClusterDir); err == nil {
				// Check if the cluster is marked as destroyed
				isDestroyed := false
				if existingCfg, err := config.Load(name); err == nil {
					if existingCfg.OpenCenter.Meta.Stage == config.StageDestroy {
						isDestroyed = true
					}
				}

				if !force && !isDestroyed {
					return fmt.Errorf("cluster configuration directory '%s' already exists in organization '%s', use --force to overwrite", name, organization)
				}

				// Force flag is set or cluster is destroyed, perform cleanup and overwrite
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
				// In strict mode, clear default values for required fields if user didn't explicitly set them
				// This ensures validation fails if required fields are missing
				// In strict mode, validation ensures required fields are present
				// We no longer clear default values as they are now correctly resolved before validation

				if errs := config.Validate(cfg); len(errs) > 0 {
					for _, e := range errs {
						fmt.Fprintln(cmd.ErrOrStderr(), e)
					}
					return fmt.Errorf("validation failed")
				}
			}

			// Create organization structure
			if err := pathResolver.CreateOrganizationStructure(context.Background(), organization); err != nil {
				return fmt.Errorf("failed to create organization structure: %w", err)
			}

			// Create .gitignore at organization level
			if err := createOrganizationGitignore(clusterPaths.OrganizationDir); err != nil {
				return fmt.Errorf("failed to create organization .gitignore: %w", err)
			}

			// Create cluster directories
			if err := pathResolver.CreateClusterDirectories(context.Background(), name, organization); err != nil {
				return fmt.Errorf("failed to create cluster directories: %w", err)
			}

			// Persist config
			// Generate SOPS key only if regenerate-keys flag is set or if key doesn't exist
			regenerateKeys, _ := cmd.Flags().GetBool("regenerate-keys")

			// Check if SOPS key already exists
			sopsKeyExists := false
			if clusterPaths.SOPSKeyPath != "" {
				if _, err := os.Stat(clusterPaths.SOPSKeyPath); err == nil {
					sopsKeyExists = true
				}
			}

			// Generate SOPS key if:
			// 1. regenerate-keys flag is set, OR
			// 2. key doesn't exist and no-keygen is not set
			disableKeygen, _ := cmd.Flags().GetBool("no-keygen")
			disableSopsKeygen, _ := cmd.Flags().GetBool("no-sops-keygen")
			shouldGenerateSOPS := (regenerateKeys || !sopsKeyExists) && !disableKeygen && !disableSopsKeygen

			if shouldGenerateSOPS && cfg.Secrets.SopsAgeKeyFile == "" && name != "" {
				if err := generateOrganizationSOPSKey(name, organization, &cfg, pathResolver); err != nil {
					return fmt.Errorf("failed to generate organization SOPS key: %w", err)
				}
			} else if sopsKeyExists && !regenerateKeys {
				// Use existing SOPS key
				cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
				fmt.Fprintf(cmd.OutOrStdout(), "Using existing SOPS key at %s\n", clusterPaths.SOPSKeyPath)
			}

			// GitOps directory is already updated to point to organization root
			// If user specified a custom git_dir, keep it as-is

			// Update SSH key paths to point to organization secrets directory
			// Format: <clustersDir>/<organization>/secrets/ssh/<cluster>-<env>-<region>
			env := cfg.OpenCenter.Meta.Env
			region := cfg.OpenCenter.Meta.Region
			if env == "" {
				env = "dev"
			}
			if region == "" {
				region = "local"
			}

			// Use the already-resolved secrets directory from clusterPaths
			// This respects any custom clustersDir configuration
			sshKeyBaseName := fmt.Sprintf("%s-%s-%s", name, env, region)
			sshKeyPath := filepath.Join(clusterPaths.SecretsDir, "ssh", sshKeyBaseName)
			sshPubKeyPath := sshKeyPath + ".pub"

			// Check if user explicitly set SSH keys via command line flags
			userSetSSHKeys := false
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "--opencenter.gitops.git_ssh_key=") ||
					strings.HasPrefix(arg, "--gitops.git_ssh_key=") ||
					strings.HasPrefix(arg, "--opencenter.gitops.git_ssh_pub=") ||
					strings.HasPrefix(arg, "--gitops.git_ssh_pub=") {
					userSetSSHKeys = true
					break
				}
			}

			// Only set SSH key paths if user didn't explicitly provide them
			if !userSetSSHKeys {
				cfg.OpenCenter.GitOps.GitSSHKey = sshKeyPath
				cfg.OpenCenter.GitOps.GitSSHPub = sshPubKeyPath
				if opencenter, ok := configMap["opencenter"].(map[string]any); ok {
					if gitops, ok := opencenter["gitops"].(map[string]any); ok {
						gitops["git_ssh_key"] = sshKeyPath
						gitops["git_ssh_pub"] = sshPubKeyPath
					} else {
						opencenter["gitops"] = map[string]any{
							"git_ssh_key": sshKeyPath,
							"git_ssh_pub": sshPubKeyPath,
						}
					}
				}
			}

			// Set secrets.ssh_key paths using the same pattern as GitOps SSH keys
			// Reuse sshKeyBaseName from above
			secretsSSHKeyPath := filepath.Join(clusterPaths.SecretsDir, "ssh", sshKeyBaseName)
			secretsSSHPubKeyPath := secretsSSHKeyPath + ".pub"

			// Check if user provided a cypher type, otherwise default to ed25519
			sshKeyCypher := "ed25519"
			if secretsMap, ok := configMap["secrets"].(map[string]any); ok {
				if sshKeyMap, ok := secretsMap["ssh_key"].(map[string]any); ok {
					if cypher, ok := sshKeyMap["cypher"].(string); ok && cypher != "" {
						sshKeyCypher = cypher
					}
				}
			}

			cfg.Secrets.SSHKey.Private = secretsSSHKeyPath
			cfg.Secrets.SSHKey.Public = secretsSSHPubKeyPath
			cfg.Secrets.SSHKey.Cypher = sshKeyCypher

			// Check if SSH key already exists
			sshKeyExists := false
			if _, err := os.Stat(secretsSSHKeyPath); err == nil {
				sshKeyExists = true
			}

			// Generate SSH key pair if:
			// 1. regenerate-keys flag is set, OR
			// 2. key doesn't exist and no-keygen is not set
			shouldGenerateSSH := (regenerateKeys || !sshKeyExists) && !disableKeygen

			if shouldGenerateSSH {
				// Create SSH directory if it doesn't exist
				sshDir := filepath.Dir(secretsSSHKeyPath)
				if err := os.MkdirAll(sshDir, 0o700); err != nil {
					return fmt.Errorf("failed to create SSH directory: %w", err)
				}

				// Create SSH key comment in format: <organization>-<cluster>-<region>
				sshKeyComment := fmt.Sprintf("%s-%s-%s", organization, name, region)

				// Generate SSH key pair using the specified cipher with comment
				keyPair, err := crypto.GenerateSSHKeyWithComment(cfg.Secrets.SSHKey.Cypher, sshKeyComment)
				if err != nil {
					return fmt.Errorf("failed to generate SSH key pair: %w", err)
				}

				// Write private key with restrictive permissions
				if err := os.WriteFile(secretsSSHKeyPath, keyPair.PrivateKey, 0o600); err != nil {
					return fmt.Errorf("failed to write SSH private key: %w", err)
				}

				// Write public key
				if err := os.WriteFile(secretsSSHPubKeyPath, keyPair.PublicKey, 0o644); err != nil {
					return fmt.Errorf("failed to write SSH public key: %w", err)
				}

				if regenerateKeys && sshKeyExists {
					fmt.Fprintf(cmd.OutOrStdout(), "Regenerated %s SSH key pair at %s\n", cfg.Secrets.SSHKey.Cypher, secretsSSHKeyPath)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Generated %s SSH key pair at %s\n", cfg.Secrets.SSHKey.Cypher, secretsSSHKeyPath)
				}
			} else if sshKeyExists && !regenerateKeys {
				fmt.Fprintf(cmd.OutOrStdout(), "Using existing SSH key pair at %s\n", secretsSSHKeyPath)
			}

			// Update the map with any SOPS key changes from the struct
			if secretsMap, ok := configMap["secrets"].(map[string]any); ok {
				if cfg.Secrets.SopsAgeKeyFile != "" {
					secretsMap["sops_age_key_file"] = cfg.Secrets.SopsAgeKeyFile
				}
				// Add SSH key configuration
				secretsMap["ssh_key"] = map[string]any{
					"private": secretsSSHKeyPath,
					"public":  secretsSSHPubKeyPath,
					"cypher":  sshKeyCypher,
				}
			} else {
				// Create secrets map if it doesn't exist
				secretsConfig := map[string]any{
					"ssh_key": map[string]any{
						"private": secretsSSHKeyPath,
						"public":  secretsSSHPubKeyPath,
						"cypher":  sshKeyCypher,
					},
				}
				if cfg.Secrets.SopsAgeKeyFile != "" {
					secretsConfig["sops_age_key_file"] = cfg.Secrets.SopsAgeKeyFile
				}
				configMap["secrets"] = secretsConfig
			}

			// Convert the map to YAML for final output
			finalYAML, err := yaml.Marshal(configMap)
			if err != nil {
				return fmt.Errorf("failed to marshal final config: %w", err)
			}

			// Get the config path at organization level (per structure document)
			configPath := filepath.Join(clusterPaths.OrganizationDir, "."+name+"-config.yaml")

			// Write the config file with proper permissions (0600 for files)
			if err := os.WriteFile(configPath, finalYAML, 0o600); err != nil {
				return fmt.Errorf("failed to write cluster configuration file to '%s': %w", configPath, err)
			}

			// Initialize git repository in the GitOps directory
			if err := initializeGitRepository(clusterPaths.GitOpsDir, cmd); err != nil {
				return fmt.Errorf("failed to initialize git repository: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created cluster configuration in organization '%s' at '%s'\n", organization, clusterPaths.ClusterDir)
			fmt.Fprintf(cmd.OutOrStdout(), "GitOps repository root: %s\n", clusterPaths.GitOpsDir)
			fmt.Fprintf(cmd.OutOrStdout(), "SOPS key location: %s\n", clusterPaths.SOPSKeyPath)
			return nil
		},
	}
	cmd.Flags().String("org", "", "organization name (defaults to cluster name if not specified)")
	cmd.Flags().String("type", "openstack", "cluster type: openstack, baremetal, kind, vmware (defaults to openstack)")
	cmd.Flags().Bool("strict", false, "fail if required values are missing")
	cmd.Flags().Bool("force", false, "overwrite existing file")
	cmd.Flags().Bool("no-keygen", false, "do not auto-generate SOPS age keys and SSH key pairs")
	cmd.Flags().Bool("no-sops-keygen", false, "do not auto-generate SOPS age keys (alias for no-keygen)")
	cmd.Flags().Bool("regenerate-keys", false, "regenerate SOPS age keys and SSH key pairs even if they already exist")
	cmd.Flags().StringArray("server-pool", []string{}, "additional server pool configuration (format: name=value,key=value,...)")
	return cmd
}

// initializeGitRepository initializes a git repository in the specified directory
// and makes an initial commit with the configuration files.
func initializeGitRepository(gitDir string, cmd *cobra.Command) error {
	// Check if git repository already exists
	if _, err := os.Stat(filepath.Join(gitDir, ".git")); err == nil {
		// Git repository already exists, skip initialization
		return nil
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return fmt.Errorf("failed to create git directory: %w", err)
	}

	// Initialize git repository
	if err := runGitCommand(gitDir, []string{"init", "-b", "main"}, cmd); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Create a basic .gitignore file
	gitignoreContent := `# SOPS-related files
.sops.yaml.bak
*.dec
*.dec.*
*.tmp

# Terraform/OpenTofu files
*.tfstate
*.tfstate.*
.terraform/
.terraform.lock.hcl

# IDE and editor files
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# Local development files
.env
.env.local
`

	gitignorePath := filepath.Join(gitDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	// Add all files to git
	if err := runGitCommand(gitDir, []string{"add", "."}, cmd); err != nil {
		return fmt.Errorf("failed to add files to git: %w", err)
	}

	// Make initial commit
	if err := runGitCommand(gitDir, []string{"commit", "-m", "initializing git repo", "--allow-empty"}, cmd); err != nil {
		return fmt.Errorf("failed to make initial commit: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized git repository at %s\n", gitDir)
	return nil
}

// runGitCommand executes a git command in the specified directory
func runGitCommand(dir string, args []string, cmd *cobra.Command) error {
	gitCmd := exec.Command("git", args...)
	gitCmd.Dir = dir
	gitCmd.Stdout = cmd.OutOrStdout()
	gitCmd.Stderr = cmd.ErrOrStderr()
	return gitCmd.Run()
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
func generateOrganizationSOPSKey(cluster, organization string, cfg *config.Config, pathResolver config.PathResolverInterface) error {
	// Resolve cluster paths for the organization
	clusterPaths, err := pathResolver.ResolveClusterPaths(context.Background(), cluster, organization)
	if err != nil {
		return fmt.Errorf("failed to resolve cluster paths: %w", err)
	}

	// Create the secrets directory with proper permissions (0755 for directories)
	secretsKeyDir := filepath.Dir(clusterPaths.SOPSKeyPath)
	if err := os.MkdirAll(secretsKeyDir, 0o755); err != nil {
		return fmt.Errorf("failed to create organization secrets directory '%s': %w", secretsKeyDir, err)
	}

	// Use the new SOPS key manager to generate and save a proper Age key
	km := sops.NewKeyManager(secretsKeyDir)
	keyPair, err := km.GenerateAgeKey()
	if err != nil {
		return fmt.Errorf("failed to generate Age key pair: %w", err)
	}

	// Save the key using the key manager (creates <cluster>-key.txt and <cluster>-key.pub)
	keyName := cluster + "-key"
	if err := km.SaveAgeKey(keyPair, keyName); err != nil {
		return fmt.Errorf("failed to save Age key pair: %w", err)
	}

	// Create or update the SOPS configuration file for the organization
	if err := createOrganizationSOPSConfig(clusterPaths.SOPSConfigPath, keyPair.PublicKey, cluster); err != nil {
		return fmt.Errorf("failed to create organization SOPS config: %w", err)
	}

	cfg.Secrets.SopsAgeKeyFile = clusterPaths.SOPSKeyPath
	return nil
}

// createOrganizationSOPSConfig creates or updates the .sops.yaml configuration file at the organization root.
// Each cluster's key only encrypts files in its specific directories:
// - /applications/overlays/<cluster>/
// - /infrastructure/clusters/<cluster>/
func createOrganizationSOPSConfig(sopsConfigPath, publicKey string, clusterName string) error {
	// sopsConfigPath is already the full path to .sops.yaml at organization root
	rootSOPSConfigPath := sopsConfigPath

	// Define the path patterns for this cluster
	clusterRule := fmt.Sprintf(`  - path_regex: (applications/overlays/%s/.*|infrastructure/clusters/%s/.*)\.ya?ml$
    age: >-
      %s`, clusterName, clusterName, publicKey)

	// Check if .sops.yaml already exists at organization root
	var existingContent string
	if data, err := os.ReadFile(rootSOPSConfigPath); err == nil {
		existingContent = string(data)
	}

	var sopsConfig string
	if existingContent == "" {
		// Create new .sops.yaml with header and first cluster rule
		sopsConfig = fmt.Sprintf(`# SOPS configuration for organization
# Each cluster's key encrypts only its specific directories
creation_rules:
%s
`, clusterRule)
	} else {
		// Check if this cluster already has a rule
		clusterRulePattern := fmt.Sprintf(`path_regex: \(applications/overlays/%s/`, clusterName)
		if strings.Contains(existingContent, clusterRulePattern) {
			// Cluster rule already exists, update it
			// Find and replace the existing rule for this cluster
			lines := strings.Split(existingContent, "\n")
			var newLines []string
			skipNext := false
			inClusterRule := false

			for i, line := range lines {
				if skipNext {
					skipNext = false
					continue
				}

				// Check if this is the start of our cluster's rule
				if strings.Contains(line, clusterRulePattern) {
					inClusterRule = true
					// Add the new rule
					newLines = append(newLines, clusterRule)
					// Skip the next line (age key line)
					if i+1 < len(lines) && strings.Contains(lines[i+1], "age:") {
						skipNext = true
					}
					continue
				}

				if inClusterRule && strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "age:") {
					inClusterRule = false
				}

				if !inClusterRule {
					newLines = append(newLines, line)
				}
			}
			sopsConfig = strings.Join(newLines, "\n")
		} else {
			// Append new cluster rule to existing config
			// Insert before the last newline if it exists
			existingContent = strings.TrimRight(existingContent, "\n")
			sopsConfig = fmt.Sprintf("%s\n%s\n", existingContent, clusterRule)
		}
	}

	// Write the SOPS configuration file at organization root
	if err := os.WriteFile(rootSOPSConfigPath, []byte(sopsConfig), 0o600); err != nil {
		return fmt.Errorf("failed to write SOPS config file: %w", err)
	}

	return nil
}

// createOrganizationGitignore creates a .gitignore file at the organization level.
// This file is copied from the embedded gitops-base-dir/.gitignore template.
func createOrganizationGitignore(organizationDir string) error {
	gitignorePath := filepath.Join(organizationDir, ".gitignore")

	// Check if .gitignore already exists
	if _, err := os.Stat(gitignorePath); err == nil {
		// .gitignore already exists, don't overwrite it
		return nil
	}

	// Read the embedded .gitignore template from gitops-base-dir
	gitignoreContent := `# Python
__pycache__/
*.pyc
*.pyd
*.pyo
*.egg-info/
.Python
build/
dist/
venv/
.env
.pytest_cache/
htmlcov/
.tox/
.mypy_cache/
.ipynb_checkpoints/

# Terraform
.terraform/
*.tfstate
*.tfstate.*
.terraform.lock.hcl
crash.log
override.tf
override_*.tf
*.tfvars
*.tfvars.json
.terraformrc
terraform.rc

# macOS
.DS_Store
.AppleDouble
.LSOverride
# Thumbnails
._*
# Spotlight files
.Spotlight-V100
# Temporary files
.Trashes
Icon?
# Xcode
build/
*.pbxuser
!default.pbxuser
*.mode1v3
!default.mode1v3
*.mode2v3
!default.mode2v3
*.xcscmblueprint
.xcscmblueprint
*.xccheckout
# Finder
.localized

# Linux
# Temporary files
*~
.#*
*.swp
*.bak
*.tmp

#RMPK specific
.bin/
id_rsa
id_rsa.pub
kubeconfig.yaml
ca.crt
ca.key
kubespray/
ansible-hardening/
.python-version
credentials/
.mygitignore
`

	// Write the .gitignore file with proper permissions (0644 for files)
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
		return fmt.Errorf("failed to write .gitignore file to organization directory '%s': %w", gitignorePath, err)
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
// parseServerPoolString parses a server pool configuration string in the format:
// "name=value,key=value,..." and returns an AdditionalServerPool struct
func parseServerPoolString(poolStr string) (config.AdditionalServerPool, error) {
	pool := config.AdditionalServerPool{}
	
	// Split by comma to get key=value pairs
	pairs := strings.Split(poolStr, ",")
	for _, pair := range pairs {
		// Split by = to get key and value
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return pool, fmt.Errorf("invalid key=value pair: %s", pair)
		}
		
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		
		// Map the key to the appropriate field
		switch key {
		case "name":
			pool.Name = value
		case "worker_count":
			count, err := strconv.Atoi(value)
			if err != nil {
				return pool, fmt.Errorf("invalid worker_count '%s': %w", value, err)
			}
			pool.WorkerCount = count
		case "flavor_worker":
			pool.FlavorWorker = value
		case "node_worker":
			pool.NodeWorker = value
		case "server_group_affinity":
			pool.ServerGroupAffinity = value
		case "image_id":
			pool.ImageID = value
		case "image_name":
			pool.ImageName = value
		case "worker_node_bfv_volume_size":
			size, err := strconv.Atoi(value)
			if err != nil {
				return pool, fmt.Errorf("invalid worker_node_bfv_volume_size '%s': %w", value, err)
			}
			pool.WorkerNodeBFVVolumeSize = size
		case "worker_node_bfv_destination_type":
			pool.WorkerNodeBFVDestinationType = value
		case "worker_node_bfv_source_type":
			pool.WorkerNodeBFVSourceType = value
		case "worker_node_bfv_volume_type":
			pool.WorkerNodeBFVVolumeType = value
		case "worker_node_bfv_delete_on_termination":
			delete, err := strconv.ParseBool(value)
			if err != nil {
				return pool, fmt.Errorf("invalid worker_node_bfv_delete_on_termination '%s': %w", value, err)
			}
			pool.WorkerNodeBFVDeleteOnTermination = delete
		case "pf9_onboard":
			onboard, err := strconv.ParseBool(value)
			if err != nil {
				return pool, fmt.Errorf("invalid pf9_onboard '%s': %w", value, err)
			}
			pool.PF9Onboard = onboard
		case "subnet_id":
			pool.SubnetID = value
		default:
			return pool, fmt.Errorf("unknown server pool field: %s", key)
		}
	}
	
	// Validate required fields
	if pool.Name == "" {
		return pool, fmt.Errorf("server pool name is required")
	}
	if pool.WorkerCount == 0 {
		return pool, fmt.Errorf("server pool worker_count is required and must be > 0")
	}
	if pool.FlavorWorker == "" {
		return pool, fmt.Errorf("server pool flavor_worker is required")
	}
	if pool.NodeWorker == "" {
		return pool, fmt.Errorf("server pool node_worker is required")
	}
	
	return pool, nil
}

// convertServerPoolsToMap converts a slice of AdditionalServerPool structs to a slice of maps
// for use in the configuration map structure
func convertServerPoolsToMap(pools []config.AdditionalServerPool) []map[string]any {
	var result []map[string]any
	for _, pool := range pools {
		poolMap := map[string]any{
			"name":         pool.Name,
			"worker_count": pool.WorkerCount,
			"flavor_worker": pool.FlavorWorker,
			"node_worker":  pool.NodeWorker,
		}
		
		// Add optional fields if they're set
		if pool.ServerGroupAffinity != "" {
			poolMap["server_group_affinity"] = pool.ServerGroupAffinity
		}
		if pool.ImageID != "" {
			poolMap["image_id"] = pool.ImageID
		}
		if pool.ImageName != "" {
			poolMap["image_name"] = pool.ImageName
		}
		if pool.WorkerNodeBFVVolumeSize > 0 {
			poolMap["worker_node_bfv_volume_size"] = pool.WorkerNodeBFVVolumeSize
		}
		if pool.WorkerNodeBFVDestinationType != "" {
			poolMap["worker_node_bfv_destination_type"] = pool.WorkerNodeBFVDestinationType
		}
		if pool.WorkerNodeBFVSourceType != "" {
			poolMap["worker_node_bfv_source_type"] = pool.WorkerNodeBFVSourceType
		}
		if pool.WorkerNodeBFVVolumeType != "" {
			poolMap["worker_node_bfv_volume_type"] = pool.WorkerNodeBFVVolumeType
		}
		if pool.WorkerNodeBFVDeleteOnTermination {
			poolMap["worker_node_bfv_delete_on_termination"] = pool.WorkerNodeBFVDeleteOnTermination
		}
		if pool.PF9Onboard {
			poolMap["pf9_onboard"] = pool.PF9Onboard
		}
		if pool.SubnetID != "" {
			poolMap["subnet_id"] = pool.SubnetID
		}
		
		result = append(result, poolMap)
	}
	return result
}