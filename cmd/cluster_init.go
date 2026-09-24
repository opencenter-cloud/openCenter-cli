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

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/di"
	"github.com/opencenter-cloud/opencenter-cli/internal/util"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const (
	kindDisableDefaultCNIFlagName = "kind-disable-default-cni"
	kindDisableDefaultCNIPath     = "opencenter.infrastructure.kind.disable_default_cni"
)

func newClusterInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new cluster configuration (non-interactive)",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		Long: `Initialize a new cluster configuration with default values.

This command creates a new cluster configuration file with sensible defaults
based on the JSON schema. You can override any configuration value using
command-line flags with dot notation.

This command generates schema version "2.0" configuration only.

The configuration is created in an organization-based directory structure:
  ~/.config/opencenter/clusters/<organization>/<cluster>/

By default, the organization is set to "opencenter". Use --org to specify a different organization.

SOPS Age encryption keys and SSH key pairs are automatically generated if they
don't already exist, unless --no-keygen is specified.`,
		Example: `  # Initialize with defaults
  opencenter cluster init my-cluster

  # Initialize from existing config file
  opencenter cluster init --config-file my-cluster-config.yaml

  # Initialize with organization
  opencenter cluster init my-cluster --org myorg

  # Initialize a VMware cluster
  opencenter cluster init my-cluster --org production --type vmware

  # Initialize a Kind cluster with the built-in CNI disabled
  opencenter cluster init my-cluster --org local --type kind --kind-disable-default-cni

  # Initialize without key generation
  opencenter cluster init my-cluster --no-keygen

  # Overwrite existing config
  opencenter cluster init my-cluster --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterInit,
	}

	cmd.Flags().String("org", "", "organization name (defaults to 'opencenter')")
	cmd.Flags().String("type", "openstack", "cluster type: openstack, baremetal, kind, vmware, magnum")
	cmd.Flags().String("config-file", "", "load configuration from file")
	cmd.Flags().String("config", "", "load configuration from file")
	_ = cmd.Flags().MarkHidden("config")
	cmd.Flags().Bool("strict", false, "fail if required values are missing")
	cmd.Flags().Bool("force", false, "overwrite existing config file")
	cmd.Flags().Bool("no-keygen", false, "do not auto-generate keys")
	cmd.Flags().Bool("no-sops-keygen", false, "do not auto-generate SOPS keys")
	cmd.Flags().Bool("regenerate-keys", false, "regenerate keys even if they exist")
	cmd.Flags().Bool("full-schema", false, "generate configuration with all available fields")
	cmd.Flags().Bool(kindDisableDefaultCNIFlagName, false, "disable Kind's default CNI so cluster networking is managed by openCenter")
	cmd.Flags().StringArray("server-pool", []string{}, "additional server pool configuration")

	return cmd
}

func runClusterInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	app, err := di.NewApp(config.ResolveClustersDir())
	if err != nil {
		return fmt.Errorf("initialize application graph: %w", err)
	}
	initService := app.InitService

	// Parse command-line options
	opts, err := parseInitOptions(cmd, args)
	if err != nil {
		return err
	}

	// Reject planned providers that are not yet available
	if err := checkProviderAvailability(opts.Provider); err != nil {
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
	pathResolver, err := di.ProvidePathResolver(config.ResolveClustersDir())
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
	} else if configFile := getInitConfigFileFlag(cmd); configFile != "" {
		name, err := extractClusterNameFromConfig(configFile)
		if err != nil {
			return opts, err
		}
		opts.ClusterName = name
		opts.ConfigFile = configFile
	} else {
		activeName, err := getActiveCluster()
		if err != nil || activeName == "" {
			return opts, fmt.Errorf("no cluster name provided and no active cluster set")
		}
		opts.ClusterName = activeName
	}

	// Parse flags
	opts.Organization, _ = cmd.Flags().GetString("org")
	opts.Provider, _ = cmd.Flags().GetString("type")
	opts.Provider = strings.ToLower(strings.TrimSpace(opts.Provider))
	opts.DeploymentMethod = "kubespray"

	// Handle org/cluster identifier format: extract organization if embedded
	// in the cluster name and no explicit --org was provided
	if strings.Contains(opts.ClusterName, "/") {
		parts := strings.SplitN(opts.ClusterName, "/", 2)
		if opts.Organization == "" {
			opts.Organization = parts[0]
		}
		opts.ClusterName = parts[1]
	}
	opts.Force, _ = cmd.Flags().GetBool("force")
	opts.Strict, _ = cmd.Flags().GetBool("strict")
	opts.NoKeyGen, _ = cmd.Flags().GetBool("no-keygen")
	opts.NoSOPSKeyGen, _ = cmd.Flags().GetBool("no-sops-keygen")
	opts.RegenerateKeys, _ = cmd.Flags().GetBool("regenerate-keys")
	opts.FullSchema, _ = cmd.Flags().GetBool("full-schema")
	opts.ServerPools, _ = cmd.Flags().GetStringArray("server-pool")

	flagOverrides, deprecatedOrg, err := parseInitFlagOverrides(cmd)
	if err != nil {
		return opts, err
	}
	opts.FlagOverrides = flagOverrides
	if opts.Organization == "" && deprecatedOrg != "" {
		opts.Organization = deprecatedOrg
	}

	if enabled, changed, err := kindDisableDefaultCNIValue(cmd, opts.Provider); err != nil {
		return opts, err
	} else if changed {
		opts.KindDisableDefaultCNI = &enabled
	}

	opts.SchemaVersion = "2.0"
	opts.OrganizationExplicit = opts.Organization != ""

	return opts, nil
}

func getInitConfigFileFlag(cmd *cobra.Command) string {
	if configFile, _ := cmd.Flags().GetString("config-file"); configFile != "" {
		return configFile
	}
	configFile, _ := cmd.Flags().GetString("config")
	return configFile
}

func kindDisableDefaultCNIValue(cmd *cobra.Command, provider string) (bool, bool, error) {
	if cmd == nil || !cmd.Flags().Changed(kindDisableDefaultCNIFlagName) {
		return false, false, nil
	}

	if !strings.EqualFold(strings.TrimSpace(provider), "kind") {
		if strings.TrimSpace(provider) == "" {
			return false, false, fmt.Errorf("--%s is only valid for kind clusters", kindDisableDefaultCNIFlagName)
		}
		return false, false, fmt.Errorf("--%s is only valid for kind clusters (provider: %s)", kindDisableDefaultCNIFlagName, provider)
	}

	enabled, err := cmd.Flags().GetBool(kindDisableDefaultCNIFlagName)
	if err != nil {
		return false, false, fmt.Errorf("read --%s: %w", kindDisableDefaultCNIFlagName, err)
	}

	return enabled, true, nil
}

func parseInitFlagOverrides(cmd *cobra.Command) ([]string, string, error) {
	rawArgs := rawCommandArgs(cmd)
	overrides := make([]string, 0)
	deprecatedOrganization := ""

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			continue
		}

		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")

		if flag := lookupCommandFlag(cmd, name); flag != nil {
			if !hasValue && flag.NoOptDefVal == "" && i+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[i+1], "-") {
				i++
			}
			continue
		}

		if !strings.Contains(name, ".") {
			return nil, "", fmt.Errorf("unknown flag: --%s", name)
		}

		if !hasValue {
			if i+1 >= len(rawArgs) || strings.HasPrefix(rawArgs[i+1], "-") {
				return nil, "", fmt.Errorf("flag needs an argument: --%s", name)
			}
			value = rawArgs[i+1]
			i++
		}

		if name == "opencenter.meta.organization" {
			deprecatedOrganization = value
			continue
		}

		overrides = append(overrides, "--"+name+"="+value)
	}

	return overrides, deprecatedOrganization, nil
}

func lookupCommandFlag(cmd *cobra.Command, name string) *pflag.Flag {
	if flag := cmd.Flags().Lookup(name); flag != nil {
		return flag
	}
	if flag := cmd.InheritedFlags().Lookup(name); flag != nil {
		return flag
	}
	return nil
}

func rawCommandArgs(cmd *cobra.Command) []string {
	if cmd != nil {
		value := reflect.ValueOf(cmd)
		if value.IsValid() && value.Kind() == reflect.Ptr {
			argsField := value.Elem().FieldByName("args")
			if argsField.IsValid() && argsField.Kind() == reflect.Slice {
				args := make([]string, 0, argsField.Len())
				for i := 0; i < argsField.Len(); i++ {
					args = append(args, argsField.Index(i).String())
				}
				if len(args) > 0 {
					return args
				}
			}
		}
	}

	if len(os.Args) > 1 {
		return append([]string(nil), os.Args[1:]...)
	}

	return nil
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
	return setFieldPath(v, strings.Split(path, "."), value, path)
}

// setFieldPath recursively follows a field path. Map entries are copied into
// addressable values before traversal, then written back to the map so that
// nested fields can be changed even though reflect map elements are not
// addressable. Pointer entries retain their existing addressability and are
// initialized when needed.
func setFieldPath(v reflect.Value, parts []string, value, path string) error {
	if len(parts) == 0 {
		return nil
	}

	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return fmt.Errorf("setting nested fields in maps is not supported: %s", path)
		}
		entry := reflect.New(v.Elem().Type()).Elem()
		entry.Set(v.Elem())
		if err := setFieldPath(entry, parts, value, path); err != nil {
			return err
		}
		v.Set(entry)
		return nil
	case reflect.Ptr:
		if v.IsNil() {
			if !v.CanSet() {
				return fmt.Errorf("cannot set field value")
			}
			v.Set(reflect.New(v.Type().Elem()))
		}
		return setFieldPath(v.Elem(), parts, value, path)
	case reflect.Map:
		if v.IsNil() {
			if !v.CanSet() {
				return fmt.Errorf("cannot set field value")
			}
			v.Set(reflect.MakeMap(v.Type()))
		}
		if v.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("map key type must be string for path-based setting, got %s", v.Type().Key().Kind())
		}

		key := reflect.New(v.Type().Key()).Elem()
		key.SetString(parts[0])
		if len(parts) == 1 {
			mapValueType := v.Type().Elem()
			newValue := reflect.New(mapValueType).Elem()
			if err := setReflectValue(newValue, value); err != nil {
				return fmt.Errorf("failed to set map value for key '%s': %w", parts[0], err)
			}
			v.SetMapIndex(key, newValue)
			return nil
		}

		entry := v.MapIndex(key)
		if !entry.IsValid() {
			return fmt.Errorf("setting nested fields in maps is not supported: %s", path)
		}
		addressableEntry := reflect.New(entry.Type()).Elem()
		addressableEntry.Set(entry)
		if err := setFieldPath(addressableEntry, parts[1:], value, path); err != nil {
			return err
		}
		v.SetMapIndex(key, addressableEntry)
		return nil
	case reflect.Struct:
		field := findField(v, parts[0])
		if !field.IsValid() {
			return fmt.Errorf("field not found: '%s' in struct '%s'", parts[0], v.Type().Name())
		}
		if len(parts) == 1 {
			return setFieldValue(field, value)
		}
		if field.Kind() != reflect.Struct && field.Kind() != reflect.Ptr && field.Kind() != reflect.Map && field.Kind() != reflect.Interface {
			return fmt.Errorf("field '%s' is not a struct or map, cannot traverse further", parts[0])
		}
		return setFieldPath(field, parts[1:], value, path)
	default:
		return fmt.Errorf("field '%s' is not a struct or map, cannot traverse further", parts[0])
	}
}

// findField also checks anonymous embedded structs, whose tagged fields are
// promoted by Go but are not returned by util.FindField.
func findField(v reflect.Value, name string) reflect.Value {
	if field := util.FindField(v, name); field.IsValid() {
		return field
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if !t.Field(i).Anonymous {
			continue
		}
		embedded := v.Field(i)
		if embedded.Kind() == reflect.Ptr {
			if embedded.Type().Elem().Kind() != reflect.Struct {
				continue
			}
			if embedded.IsNil() {
				if !embedded.CanSet() {
					continue
				}
				embedded.Set(reflect.New(embedded.Type().Elem()))
			}
			embedded = embedded.Elem()
		}
		if embedded.Kind() == reflect.Struct {
			if field := findField(embedded, name); field.IsValid() {
				return field
			}
		}
	}
	return reflect.Value{}
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
	case reflect.Ptr:
		if value == "null" {
			field.SetZero()
			return nil
		}
		return fmt.Errorf("unsupported field type: %s", field.Type())
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
