// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law of a agreed to in writing, software
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
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newClusterServiceCmd creates the top-level "cluster service" command.
func newClusterServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage cluster services",
		Long: `The service command allows adding and removing services from a cluster's configuration.

Services can be either standard services or managed services. When adding a service,
it may require additional parameters or secrets. If these are not provided, the
command will fail and provide an example of the correct usage.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newClusterServiceEnableCmd())
	cmd.AddCommand(newClusterServiceDisableCmd())
	cmd.AddCommand(newClusterServiceStatusCmd())
	cmd.AddCommand(newClusterServiceOptionsCmd())
	cmd.AddCommand(newClusterServiceStorageCmd())
	return cmd
}

// newClusterServiceEnableCmd creates the "cluster service enable" command.
func newClusterServiceEnableCmd() *cobra.Command {
	var (
		isManaged      bool
		params         []string
		secrets        []string
		cluster        string
		force          bool
		render         bool
		prune          bool
		adoptGenerated bool
	)
	cmd := &cobra.Command{
		Use:   "enable <service-name>",
		Short: "Enable a service in the cluster configuration",
		Long: `This command enables a service in the cluster configuration.
If the service requires additional parameters or secrets, they must be provided
as flags. If they are missing, the command will return an error with an example.

Examples:
  # Enable the 'cert-manager' service with a required email parameter
  opencenter cluster service enable cert-manager --param="email=admin@example.com"

  # Enable a managed service with a secret
  opencenter cluster service enable my-managed-service --managed --secret="api_key=some_secret_value"

  # Force re-enable (re-render) an already enabled service
  opencenter cluster service enable prometheus --force

  # Enable a service and immediately render its templates
  opencenter cluster service enable loki --render`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]

			// Resolve cluster name from flag or active cluster
			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}

			// Load configuration
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration for '%s': %w", clusterName, err)
			}
			serviceMap := ensureServiceMap(&cfg, isManaged)
			existingService, exists := serviceMap[serviceName]
			wasEnabled := exists && isEnabled(existingService)
			serviceLabel := serviceTargetLabel(isManaged)

			if exists && wasEnabled && !force {
				return fmt.Errorf("%s '%s' is already enabled. Use --force to update or re-render", serviceLabel, serviceName)
			}

			serviceCfg := existingService
			var explicitFields map[string]any
			if exists {
				explicitFields, err = loadExplicitServiceFields(cmd.Context(), cfg, isManaged, serviceName)
				if err != nil {
					return fmt.Errorf("failed to inspect existing service config: %w", err)
				}
			}
			if !exists {
				serviceCfg, _ = v2.NewDefaultServiceConfig(serviceName, cfg.OpenCenter.Cluster.ClusterFQDN)
				if serviceCfg == nil {
					serviceCfg = newServiceConfig(serviceName)
				}
			}
			serviceCfg, err = materializeServiceConfig(serviceName, serviceCfg)
			if err != nil {
				return fmt.Errorf("failed to prepare service config: %w", err)
			}
			if exists {
				serviceCfg, err = hydrateBuiltInServiceConfig(serviceName, serviceCfg, cfg.OpenCenter.Cluster.ClusterFQDN, explicitFields)
				if err != nil {
					return fmt.Errorf("failed to hydrate service defaults: %w", err)
				}
			}

			// Set Enabled = true
			if err := setEnabled(serviceCfg, true); err != nil {
				return fmt.Errorf("failed to enable service: %w", err)
			}

			// Process parameters
			if err := processParams(params, serviceCfg); err != nil {
				return err
			}
			// Process secrets
			if err := processSecrets(secrets, serviceName, &cfg.Secrets); err != nil {
				return err
			}
			// Custom validation logic (validate before saving). Include the candidate
			// service in the config so provider-aware backend resolution sees it.
			serviceMap[serviceName] = serviceCfg
			if err := validateServiceWithConfig(serviceName, serviceCfg, &cfg.Secrets, &cfg); err != nil {
				return err
			}

			if !isManaged {
				if err := validateServiceDependencies(cfg.OpenCenter.Services); err != nil {
					return err
				}
			}

			switch {
			case exists && wasEnabled:
				fmt.Fprintf(cmd.OutOrStdout(), "Updating enabled %s '%s' in cluster '%s'...\n", serviceLabel, serviceName, clusterName)
			case exists:
				fmt.Fprintf(cmd.OutOrStdout(), "Re-enabling %s '%s' in cluster '%s'...\n", serviceLabel, serviceName, clusterName)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "Enabling %s '%s' in cluster '%s'...\n", serviceLabel, serviceName, clusterName)
			}
			// Save the updated configuration
			if err := saveConfig(cmd.Context(), cfg); err != nil {
				return fmt.Errorf("failed to save updated configuration: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully enabled %s '%s' in cluster '%s'.\n", serviceLabel, serviceName, clusterName)

			// Render the service if --render flag is set
			if render {
				// Validate git_dir is set before rendering
				if cfg.OpenCenter.GitOps.Repository.LocalDir == "" {
					return fmt.Errorf("git_dir is not configured. Run 'opencenter cluster generate' first or set git_dir in the configuration")
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Rendering service '%s'...\n", serviceName)

				promotion, err := renderSingleServiceEncryptedResult(cmd.Context(), cfg, serviceName, isManaged, prune, adoptGenerated)
				if err != nil {
					return fmt.Errorf("failed to render service: %w", err)
				}
				printPromotionSummary(cmd, promotion)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&isManaged, "managed", false, "Enable the service as a managed service")
	// StringArrayVar deliberately preserves each value verbatim. StringSliceVar
	// splits on commas, which corrupts JSON arrays and objects passed to --param.
	cmd.Flags().StringArrayVar(&params, "param", []string{}, "Set a service parameter (e.g., --param key=value)")
	cmd.Flags().StringSliceVar(&secrets, "secret", []string{}, "Set a service secret (e.g., --secret key=value)")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Specify the cluster name")
	cmd.Flags().BoolVar(&force, "force", false, "Force re-enable an already enabled service to re-render configuration")
	cmd.Flags().BoolVar(&render, "render", false, "Render the service templates immediately after enabling")
	cmd.Flags().BoolVar(&prune, "prune", true, "Remove stale generated files during --render")
	cmd.Flags().BoolVar(&adoptGenerated, "adopt-generated", false, "Claim differing planned files after creating backups during --render")
	return cmd
}

// newClusterServiceDisableCmd creates the "cluster service disable" command.
func newClusterServiceDisableCmd() *cobra.Command {
	var (
		isManaged      bool
		cluster        string
		render         bool
		prune          bool
		adoptGenerated bool
	)
	cmd := &cobra.Command{
		Use:   "disable <service-name>",
		Short: "Disable a service in the cluster configuration",
		Long: `This command disables a service in the cluster configuration by setting its enabled flag to false.

Examples:
  # Disable the 'cert-manager' service
  opencenter cluster service disable cert-manager

  # Disable a managed service
  opencenter cluster service disable my-managed-service --managed

  # Disable a service and immediately update the rendered manifests
  opencenter cluster service disable loki --render`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]

			// Resolve cluster name from flag or active cluster
			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}

			// Load configuration
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration for '%s': %w", clusterName, err)
			}
			serviceLabel := serviceTargetLabel(isManaged)
			// Disable the service in the appropriate map
			if isManaged {
				svc, exists := cfg.OpenCenter.ManagedServices[serviceName]
				if !exists {
					return fmt.Errorf("%s '%s' not found", serviceLabel, serviceName)
				}
				if !isEnabled(svc) {
					return fmt.Errorf("%s '%s' is already disabled", serviceLabel, serviceName)
				}
				if err := setEnabled(svc, false); err != nil {
					return fmt.Errorf("failed to disable service: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Disabling %s '%s' in cluster '%s'...\n", serviceLabel, serviceName, clusterName)
			} else {
				svc, exists := cfg.OpenCenter.Services[serviceName]
				if !exists {
					return fmt.Errorf("%s '%s' not found", serviceLabel, serviceName)
				}
				if !isEnabled(svc) {
					return fmt.Errorf("%s '%s' is already disabled", serviceLabel, serviceName)
				}
				if err := setEnabled(svc, false); err != nil {
					return fmt.Errorf("failed to disable service: %w", err)
				}
				if err := validateServiceDependencies(cfg.OpenCenter.Services); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Disabling %s '%s' in cluster '%s'...\n", serviceLabel, serviceName, clusterName)
			}
			// Save the updated configuration
			if err := saveConfig(cmd.Context(), cfg); err != nil {
				return fmt.Errorf("failed to save updated configuration: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully disabled %s '%s' in cluster '%s'.\n", serviceLabel, serviceName, clusterName)

			if render {
				if cfg.OpenCenter.GitOps.Repository.LocalDir == "" {
					return fmt.Errorf("git_dir is not configured. Run 'opencenter cluster generate' first or set git_dir in the configuration")
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Rendering cluster apps after disabling '%s'...\n", serviceName)
				promotion, err := renderClusterAppsEncryptedResult(cmd.Context(), cfg, prune, adoptGenerated)
				if err != nil {
					return fmt.Errorf("failed to render cluster apps: %w", err)
				}
				printPromotionSummary(cmd, promotion)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&isManaged, "managed", false, "Disable the service from the managed services list")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Specify the cluster name")
	cmd.Flags().BoolVar(&render, "render", false, "Render the cluster application manifests immediately after disabling")
	cmd.Flags().BoolVar(&prune, "prune", true, "Remove stale generated files during --render")
	cmd.Flags().BoolVar(&adoptGenerated, "adopt-generated", false, "Claim differing planned files after creating backups during --render")
	return cmd
}

func newServiceConfig(serviceName string) any {
	configType := registry.GetServiceConfigType(serviceName)
	if configType == nil {
		configType = reflect.TypeOf(services.DefaultServiceConfig{})
	}
	return reflect.New(configType).Interface()
}

func ensureServiceMap(cfg *v2.Config, isManaged bool) v2.ServiceMap {
	if isManaged {
		if cfg.OpenCenter.ManagedServices == nil {
			cfg.OpenCenter.ManagedServices = make(v2.ServiceMap)
		}
		return cfg.OpenCenter.ManagedServices
	}

	if cfg.OpenCenter.Services == nil {
		cfg.OpenCenter.Services = make(v2.ServiceMap)
	}
	return cfg.OpenCenter.Services
}

func serviceTargetLabel(isManaged bool) string {
	if isManaged {
		return "managed service"
	}
	return "service"
}

func validateServiceDependencies(serviceMap v2.ServiceMap) error {
	validator := services.NewDependencyValidator()
	errors := validator.ValidateDependencies(map[string]any(serviceMap))
	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errors, "\n"))
}

func processParams(params []string, serviceCfg any) error {
	assignments := make([]configAssignment, 0, len(params))
	for _, p := range params {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid parameter format: '%s'. Expected key=value", p)
		}
		if strings.TrimSpace(parts[0]) == "" {
			return fmt.Errorf("invalid parameter format: '%s'. Parameter path cannot be empty", p)
		}
		assignments = append(assignments, configAssignment{Path: parts[0], Value: parts[1]})
	}
	if err := applyConfigAssignments(serviceCfg, assignments); err != nil {
		return fmt.Errorf("failed to set service parameter: %w", err)
	}
	return nil
}

func materializeServiceConfig(serviceName string, serviceCfg any) (any, error) {
	if serviceCfg == nil {
		return newServiceConfig(serviceName), nil
	}

	switch serviceCfg.(type) {
	case map[string]any:
		configType := registry.GetServiceConfigType(serviceName)
		if configType == nil {
			return serviceCfg, nil
		}

		data, err := yaml.Marshal(serviceCfg)
		if err != nil {
			return nil, fmt.Errorf("marshal map-backed service config: %w", err)
		}

		typedCfg := reflect.New(configType).Interface()
		if err := yaml.Unmarshal(data, typedCfg); err != nil {
			return nil, fmt.Errorf("decode typed service config: %w", err)
		}
		return typedCfg, nil
	default:
		return serviceCfg, nil
	}
}

func loadExplicitServiceFields(ctx context.Context, cfg v2.Config, managed bool, serviceName string) (map[string]any, error) {
	configPath, err := getConfigPath(ctx, cfg.ClusterName(), cfg.OpenCenter.Meta.Organization)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	opencenter, _ := document["opencenter"].(map[string]any)
	section := "services"
	if managed {
		section = "managed_services"
	}
	serviceMap, _ := opencenter[section].(map[string]any)
	fields, _ := serviceMap[serviceName].(map[string]any)
	if fields == nil {
		return map[string]any{}, nil
	}
	return fields, nil
}

func hydrateBuiltInServiceConfig(serviceName string, existing any, clusterFQDN string, explicitFields map[string]any) (any, error) {
	defaults, builtIn := v2.NewDefaultServiceConfig(serviceName, clusterFQDN)
	if !builtIn {
		return existing, nil
	}
	if existing == nil {
		return defaults, nil
	}

	destination := reflect.ValueOf(existing)
	source := reflect.ValueOf(defaults)
	if destination.Type() != source.Type() {
		return nil, fmt.Errorf("existing type %T does not match canonical default type %T", existing, defaults)
	}
	if err := mergeMissingServiceDefaults(destination, source, explicitFields); err != nil {
		return nil, err
	}
	return existing, nil
}

func mergeMissingServiceDefaults(destination, defaults reflect.Value, explicitFields map[string]any) error {
	for destination.Kind() == reflect.Pointer {
		if destination.IsNil() || defaults.IsNil() {
			return fmt.Errorf("service config pointers must not be nil")
		}
		destination = destination.Elem()
		defaults = defaults.Elem()
	}
	if destination.Kind() != reflect.Struct || defaults.Kind() != reflect.Struct {
		return fmt.Errorf("service configs must be structs")
	}

	typeInfo := destination.Type()
	for i := 0; i < destination.NumField(); i++ {
		fieldInfo := typeInfo.Field(i)
		fieldName := strings.Split(fieldInfo.Tag.Get("yaml"), ",")[0]
		if fieldInfo.Anonymous && (fieldName == "" || fieldName == "-") {
			if err := mergeMissingServiceDefaults(destination.Field(i), defaults.Field(i), explicitFields); err != nil {
				return err
			}
			continue
		}
		if fieldName == "" || fieldName == "-" {
			continue
		}

		explicitValue, present := explicitFields[fieldName]
		if present {
			if nested, ok := explicitValue.(map[string]any); ok && destination.Field(i).Kind() == reflect.Struct {
				if err := mergeMissingServiceDefaults(destination.Field(i), defaults.Field(i), nested); err != nil {
					return err
				}
			}
			continue
		}
		if destination.Field(i).CanSet() {
			destination.Field(i).Set(defaults.Field(i))
		}
	}
	return nil
}

func processSecrets(secrets []string, serviceName string, secretsCfg *v2.SecretsConfig) error {
	secretMap := make(map[string]string)
	for _, s := range secrets {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid secret format: '%s'. Expected key=value", s)
		}
		secretMap[parts[0]] = parts[1]
	}
	if len(secretMap) == 0 {
		return nil
	}
	// Find the correct nested secret struct based on serviceName
	var targetStruct reflect.Value
	secretsVal := reflect.ValueOf(secretsCfg).Elem()
	// Map service name to the field name in the Secrets struct
	// e.g., "cert-manager" -> "CertManager", "weave-gitops" -> "WeaveGitOps"
	// A simple mapping approach
	serviceToField := map[string]string{
		"cert-manager": "CertManager",
		"loki":         "Loki",
		"keycloak":     "Keycloak",
		"headlamp":     "Headlamp",
		"weave-gitops": "WeaveGitOps",
		"grafana":      "Grafana",
		"tempo":        "Tempo",
		"etcd-backup":  "EtcdBackup",
		"velero":       "Velero",
		"harbor":       "Harbor",
		"alert-proxy":  "AlertProxy",
		"vsphere-csi":  "VSphereCsi",
	}
	fieldName, ok := serviceToField[serviceName]
	if !ok {
		return fmt.Errorf("no secret configuration found for service '%s'", serviceName)
	}
	targetStruct = secretsVal.FieldByName(fieldName)
	if !targetStruct.IsValid() || targetStruct.Kind() != reflect.Struct {
		return fmt.Errorf("internal error: invalid secret struct for service '%s'", serviceName)
	}
	targetStruct = targetStruct.Addr() // Get pointer to the struct to modify it
	v := targetStruct.Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if val, ok := secretMap[jsonTag]; ok {
			fieldVal := v.Field(i)
			if !fieldVal.CanSet() {
				continue
			}
			if err := setFieldValue(fieldVal, val); err != nil {
				return fmt.Errorf("failed to set secret '%s': %w", jsonTag, err)
			}
		}
	}
	return nil
}

// validateService performs custom validation for specific services.
func validateServiceLegacy(serviceName string, serviceCfg any, secretsCfg *v2.SecretsConfig) error {
	switch serviceName {
	case "cert-manager":
		if cfg, ok := serviceCfg.(*services.CertManagerConfig); ok {
			if cfg.Email == "" {
				return fmt.Errorf("missing required parameter 'email' for service 'cert-manager'.\nExample: --param=\"email=your-email@example.com\"")
			}
		}
	case "loki":
		if cfg, ok := serviceCfg.(*services.LokiConfig); ok {
			storageType := cfg.StorageType
			if storageType == "" {
				storageType = "s3"
			}
			if storageType != "s3" && storageType != "none" {
				return fmt.Errorf("unsupported storage_type %q for service 'loki'; platform bulk data must use S3-compatible storage", storageType)
			}
			if storageType == "s3" {
				// S3 credentials are optional (can use IAM roles), but if provided, both must be set
				hasS3Creds := secretsCfg.Loki.S3AccessKeyID != "" || secretsCfg.Loki.S3SecretAccessKey != ""
				if hasS3Creds && (secretsCfg.Loki.S3AccessKeyID == "" || secretsCfg.Loki.S3SecretAccessKey == "") {
					return fmt.Errorf("both S3 access key and secret key must be provided for service 'loki'.\nExample: --secret=\"s3_access_key_id=AKIA...\" --secret=\"s3_secret_access_key=your-secret\"")
				}
			}
		}
	case "keycloak":
		if secretsCfg.Keycloak.AdminPassword == "" {
			return fmt.Errorf("missing required secret 'admin_password' for service 'keycloak'.\nExample: --secret=\"admin_password=your-password\"")
		}
	case "harbor":
		if harbor, ok := serviceCfg.(*services.HarborConfig); ok && !strings.EqualFold(strings.TrimSpace(harbor.StorageType), "filesystem") {
			accessMissing := strings.TrimSpace(secretsCfg.Harbor.S3AccessKeyID) == "" || strings.EqualFold(strings.TrimSpace(secretsCfg.Harbor.S3AccessKeyID), v2.PlaceholderSecret)
			secretMissing := strings.TrimSpace(secretsCfg.Harbor.S3SecretAccessKey) == "" || strings.EqualFold(strings.TrimSpace(secretsCfg.Harbor.S3SecretAccessKey), v2.PlaceholderSecret)
			if accessMissing != secretMissing {
				return fmt.Errorf("both Harbor S3 access key and secret key must be provided.\nExample: --secret=\"s3_access_key_id=ACCESS\" --secret=\"s3_secret_access_key=SECRET\"")
			}
		}
	}
	return nil
}

func validateService(serviceName string, serviceCfg any, secretsCfg *v2.SecretsConfig) error {
	return validateServiceWithConfig(serviceName, serviceCfg, secretsCfg, nil)
}

func validateServiceWithConfig(serviceName string, serviceCfg any, secretsCfg *v2.SecretsConfig, cfg *v2.Config) error {
	if serviceName == "harbor" {
		if harbor, ok := serviceCfg.(*services.HarborConfig); ok {
			filesystem := strings.EqualFold(strings.TrimSpace(harbor.StorageType), "filesystem")
			managedObjectStorage := cfg != nil && v2.UsesManagedObjectStorage(cfg)
			accessMissing := strings.TrimSpace(secretsCfg.Harbor.S3AccessKeyID) == "" || strings.EqualFold(strings.TrimSpace(secretsCfg.Harbor.S3AccessKeyID), v2.PlaceholderSecret)
			secretMissing := strings.TrimSpace(secretsCfg.Harbor.S3SecretAccessKey) == "" || strings.EqualFold(strings.TrimSpace(secretsCfg.Harbor.S3SecretAccessKey), v2.PlaceholderSecret)
			if !filesystem && !managedObjectStorage && accessMissing != secretMissing {
				return fmt.Errorf("both Harbor S3 access key and secret key must be provided.\nExample: --secret=\"s3_access_key_id=ACCESS\" --secret=\"s3_secret_access_key=SECRET\"")
			}
			if err := v2.ValidateHarborConfig(harbor); err != nil {
				return err
			}
		}
		return nil
	}
	if serviceName != "loki" && serviceName != "tempo" {
		return validateServiceLegacy(serviceName, serviceCfg, secretsCfg)
	}

	backend := ""
	if cfg != nil {
		backend = v2.ResolveObjectStorageBackend(cfg, serviceName)
	}
	if backend == "" {
		switch typed := serviceCfg.(type) {
		case *services.LokiConfig:
			backend = strings.ToLower(strings.TrimSpace(typed.StorageType))
			if backend == "" {
				backend = "s3"
			}
		case *services.TempoConfig:
			backend = strings.ToLower(strings.TrimSpace(typed.StorageType))
			if backend == "" {
				backend = "s3"
			}
		}
	}

	switch backend {
	case "none":
		return nil
	case "swift":
		return fmt.Errorf("unsupported storage backend swift for service '%s'; migrate to the S3-compatible storage profile", serviceName)
	case "s3":
		var endpoint, access, secret string
		switch typed := serviceCfg.(type) {
		case *services.LokiConfig:
			endpoint, access, secret = typed.S3Endpoint, secretsCfg.Loki.S3AccessKeyID, secretsCfg.Loki.S3SecretAccessKey
		case *services.TempoConfig:
			endpoint, access, secret = typed.S3Endpoint, secretsCfg.Tempo.AccessKey, secretsCfg.Tempo.SecretKey
		}
		if err := v2.ValidateS3Endpoint(endpoint); err != nil {
			return fmt.Errorf("service '%s' requires a configured S3 endpoint (s3_endpoint): %w", serviceName, err)
		}
		if (access == "") != (secret == "") {
			return fmt.Errorf("both S3 access key and secret key must be provided for service '%s'", serviceName)
		}
	default:
		return fmt.Errorf("unsupported storage backend %q for service '%s'", backend, serviceName)
	}
	return nil
}

// newClusterServiceStatusCmd creates the "cluster service status" command.
func newClusterServiceStatusCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display state of all services in the cluster configuration",
		Long: `Display all services (standard and managed) with their enabled/disabled state
and adoption mode. For live deployment status, use 'opencenter cluster status --sync'.

Examples:
  # Show state of all services in the active cluster
  opencenter cluster service status

  # Show state for a specific cluster
  opencenter cluster service status --cluster my-cluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from flag or active cluster
			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}

			// Load configuration
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration for '%s': %w", clusterName, err)
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-15s\n", "SERVICE NAME", "STATE", "ADOPTION MODE")
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-15s\n", strings.Repeat("-", 30), strings.Repeat("-", 12), strings.Repeat("-", 15))

			// Print standard services
			for name, svc := range cfg.OpenCenter.Services {
				state := "disabled"
				if isEnabled(svc) {
					state = "enabled"
				}
				mode := getAdoptionMode(svc)
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-15s\n", name, state, mode)
			}

			// Print managed services
			for name, svc := range cfg.OpenCenter.ManagedServices {
				state := "disabled"
				if isEnabled(svc) {
					state = "enabled"
				}
				mode := getAdoptionMode(svc)
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-15s\n", name+" (managed)", state, mode)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "Specify the cluster name")
	return cmd
}

// newClusterServiceOptionsCmd creates the "cluster service options" command.
func newClusterServiceOptionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options <service-name>",
		Short: "Display available configuration options for a service",
		Long: `This command displays all available configuration parameters and secrets for a service.
It shows the field names, types, descriptions, and whether they are required.

Examples:
  # Show options for cert-manager
  opencenter cluster service options cert-manager

  # Show options for loki
  opencenter cluster service options loki

  # Show options for a managed service
  opencenter cluster service options alert-proxy --managed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]
			isManaged, _ := cmd.Flags().GetBool("managed")

			// Get service-specific options
			options := getServiceOptions(serviceName)
			secrets := getServiceSecrets(serviceName)

			fmt.Fprintf(cmd.OutOrStdout(), "Configuration options for service '%s':\n\n", serviceName)

			// Display common fields
			fmt.Fprintln(cmd.OutOrStdout(), "Common Fields:")
			fmt.Fprintln(cmd.OutOrStdout(), "  enabled (boolean) - Enable or disable this service")
			fmt.Fprintln(cmd.OutOrStdout(), "  adoption_mode (string) - How Flux interacts with this service (managed/external/sync/deferred/takeover)")
			fmt.Fprintln(cmd.OutOrStdout(), "  namespace (string) - Kubernetes namespace for the service")
			fmt.Fprintln(cmd.OutOrStdout(), "  uri (string) - Git repository URI")

			if isManaged {
				fmt.Fprintln(cmd.OutOrStdout(), "\nManaged Service Fields:")
				fmt.Fprintln(cmd.OutOrStdout(), "  gitops_source_repo (string) - GitOps source repository URL")
				fmt.Fprintln(cmd.OutOrStdout(), "  gitops_source_release (string) - GitOps source release tag")
				fmt.Fprintln(cmd.OutOrStdout(), "  gitops_source_branch (string) - GitOps source branch")
			}

			// Display service-specific parameters
			if len(options) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nService-Specific Parameters:")
				for _, opt := range options {
					required := ""
					if opt.Required {
						required = " [REQUIRED]"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) - %s%s\n", opt.Name, opt.Type, opt.Description, required)
				}
			}

			// Display service-specific secrets
			if len(secrets) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nService-Specific Secrets:")
				for _, secret := range secrets {
					required := ""
					if secret.Required {
						required = " [REQUIRED]"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) - %s%s\n", secret.Name, secret.Type, secret.Description, required)
				}
			}

			// Display usage examples
			fmt.Fprintln(cmd.OutOrStdout(), "\nUsage Examples:")
			if len(options) > 0 {
				exampleParam := options[0].Name
				fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster service enable %s --param=\"%s=value\"\n", serviceName, exampleParam)
			}
			if len(secrets) > 0 {
				exampleSecret := secrets[0].Name
				fmt.Fprintf(cmd.OutOrStdout(), "  opencenter cluster service enable %s --secret=\"%s=secret-value\"\n", serviceName, exampleSecret)
			}

			return nil
		},
	}
	cmd.Flags().Bool("managed", false, "Show options for a managed service")
	return cmd
}

// ServiceOption represents a configuration option for a service
type ServiceOption struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

// getServiceOptions returns the service-specific configuration options
func getServiceOptions(serviceName string) []ServiceOption {
	switch serviceName {
	case "cert-manager":
		return []ServiceOption{
			{Name: "email", Type: "string", Description: "Email address for Let's Encrypt certificate notifications", Required: true},
			{Name: "letsencrypt_server", Type: "string", Description: "LetsEncrypt ACME server URL", Required: false},
			{Name: "region", Type: "string", Description: "AWS region for Route53 DNS validation", Required: false},
		}
	case "loki":
		return []ServiceOption{
			{Name: "storage_type", Type: "string", Description: "Storage backend type (s3 or swift)", Required: false},
			{Name: "bucket_name", Type: "string", Description: "Storage bucket/container name", Required: true},
			{Name: "volume_size", Type: "integer", Description: "Persistent volume size in GB", Required: false},
			{Name: "storage_class", Type: "string", Description: "Storage class", Required: false},
			{Name: "swift_auth_url", Type: "string", Description: "Swift Keystone V3 authentication URL (for Swift storage)", Required: false},
			{Name: "swift_region", Type: "string", Description: "Swift region name (for Swift storage)", Required: false},
			{Name: "swift_auth_version", Type: "integer", Description: "Swift authentication version (default: 3)", Required: false},
			{Name: "swift_application_credential_id", Type: "string", Description: "Swift application credential ID (recommended)", Required: false},
			{Name: "swift_container_name", Type: "string", Description: "Swift container name", Required: false},
			{Name: "s3_endpoint", Type: "string", Description: "S3 endpoint URL (for S3 storage, e.g., MinIO)", Required: false},
			{Name: "s3_region", Type: "string", Description: "S3 region (for S3 storage)", Required: false},
			{Name: "s3_force_path_style", Type: "boolean", Description: "Force S3 path style (required for MinIO)", Required: false},
			{Name: "s3_insecure", Type: "boolean", Description: "Allow insecure S3 connections", Required: false},
		}
	case "tempo":
		return []ServiceOption{
			{Name: "storage_type", Type: "string", Description: "Storage backend type (s3)", Required: false},
			{Name: "bucket_name", Type: "string", Description: "S3 bucket name", Required: false},
			{Name: "s3_endpoint", Type: "string", Description: "S3 endpoint URL", Required: false},
			{Name: "s3_region", Type: "string", Description: "S3 region", Required: false},
		}
	case "harbor":
		return []ServiceOption{
			{Name: "storage_type", Type: "string", Description: "Storage backend type (s3 or filesystem)", Required: false},
			{Name: "s3_bucket", Type: "string", Description: "S3 bucket name for image storage", Required: false},
			{Name: "s3_endpoint", Type: "string", Description: "S3 endpoint URL for image storage", Required: false},
			{Name: "s3_region", Type: "string", Description: "S3 region for image storage", Required: false},
		}
	case "keycloak":
		return []ServiceOption{
			{Name: "realm", Type: "string", Description: "Keycloak realm name", Required: false},
			{Name: "frontend_url", Type: "string", Description: "Keycloak frontend URL", Required: false},
			{Name: "client_id", Type: "string", Description: "Keycloak client ID", Required: false},
		}
	case "headlamp":
		return []ServiceOption{
			{Name: "oidc_issuer_url", Type: "string", Description: "Headlamp OIDC issuer URL", Required: false},
			{Name: "oidc_client_id", Type: "string", Description: "Headlamp OIDC client ID", Required: false},
		}
	case "kube-prometheus-stack":
		return []ServiceOption{
			{Name: "grafana_volume_size", Type: "integer", Description: "Grafana persistent volume size in GB", Required: false},
			{Name: "grafana_storage_class", Type: "string", Description: "Grafana storage class", Required: false},
			{Name: "prometheus_volume_size", Type: "integer", Description: "Prometheus persistent volume size in GB", Required: false},
			{Name: "prometheus_storage_class", Type: "string", Description: "Prometheus storage class", Required: false},
			{Name: "alertmanager_volume_size", Type: "integer", Description: "Alertmanager persistent volume size in GB", Required: false},
			{Name: "alertmanager_storage_class", Type: "string", Description: "Alertmanager storage class", Required: false},
		}
	case "velero":
		return []ServiceOption{
			{Name: "storage_type", Type: "string", Description: "Storage backend type (s3 or none)", Required: false},
			{Name: "backup_bucket", Type: "string", Description: "Velero backup bucket name", Required: false},
			{Name: "region", Type: "string", Description: "Velero backup region", Required: false},
		}
	case "etcd-backup":
		return []ServiceOption{
			{Name: "storage_type", Type: "string", Description: "Storage backend type (s3 or none)", Required: false},
			{Name: "s3_bucket_name", Type: "string", Description: "etcd backup bucket name", Required: false},
			{Name: "s3_endpoint", Type: "string", Description: "S3 endpoint URL", Required: false},
			{Name: "s3_region", Type: "string", Description: "S3 region", Required: false},
		}
	case "alert-proxy":
		return []ServiceOption{
			{Name: "alert_manager_base_url", Type: "string", Description: "Alert manager base URL", Required: false},
			{Name: "http_route_fqdn", Type: "string", Description: "HTTPRoute fully qualified domain name", Required: false},
		}
	case "calico":
		return []ServiceOption{
			{Name: "kube_api_server", Type: "string", Description: "Calico Kubernetes API server address", Required: false},
		}
	default:
		return []ServiceOption{
			{Name: "namespace", Type: "string", Description: "Kubernetes namespace for the service", Required: false},
			{Name: "hostname", Type: "string", Description: "Hostname for HTTPRoute configuration", Required: false},
			{Name: "image_repository", Type: "string", Description: "Container image repository", Required: false},
			{Name: "image_tag", Type: "string", Description: "Container image tag", Required: false},
		}
	}
}

// getServiceSecrets returns the service-specific secrets
func getServiceSecrets(serviceName string) []ServiceOption {
	switch serviceName {
	case "cert-manager":
		return []ServiceOption{
			{Name: "aws_access_key", Type: "string", Description: "AWS access key for Route53 DNS validation", Required: false},
			{Name: "aws_secret_access_key", Type: "string", Description: "AWS secret access key for Route53 DNS validation", Required: false},
		}
	case "loki":
		return []ServiceOption{
			{Name: "swift_application_credential_secret", Type: "string", Description: "Swift application credential secret (recommended for Swift)", Required: false},
			{Name: "swift_password", Type: "string", Description: "Swift password (legacy, deprecated)", Required: false},
			{Name: "s3_access_key_id", Type: "string", Description: "S3 access key ID (for S3 storage)", Required: false},
			{Name: "s3_secret_access_key", Type: "string", Description: "S3 secret access key (for S3 storage)", Required: false},
		}
	case "tempo":
		return []ServiceOption{
			{Name: "access_key", Type: "string", Description: "S3 access key (for S3 storage)", Required: false},
			{Name: "secret_key", Type: "string", Description: "S3 secret key (for S3 storage)", Required: false},
		}
	case "harbor":
		return []ServiceOption{
			{Name: "admin_password", Type: "string", Description: "Harbor administrator password", Required: true},
			{Name: "registry_password", Type: "string", Description: "Harbor registry password", Required: true},
			{Name: "database_password", Type: "string", Description: "Harbor database password", Required: true},
			{Name: "s3_access_key_id", Type: "string", Description: "Externally issued S3 access key ID for Harbor image storage", Required: false},
			{Name: "s3_secret_access_key", Type: "string", Description: "Externally issued S3 secret access key for Harbor image storage", Required: false},
		}
	case "velero":
		return []ServiceOption{
			{Name: "access_key_id", Type: "string", Description: "S3 access key ID (for S3 storage)", Required: false},
			{Name: "secret_access_key", Type: "string", Description: "S3 secret access key (for S3 storage)", Required: false},
		}
	case "etcd-backup":
		return []ServiceOption{
			{Name: "access_key_id", Type: "string", Description: "S3 access key ID (for S3 storage)", Required: false},
			{Name: "secret_access_key", Type: "string", Description: "S3 secret access key (for S3 storage)", Required: false},
		}
	case "keycloak":
		return []ServiceOption{
			{Name: "admin_password", Type: "string", Description: "Keycloak admin user password", Required: true},
			{Name: "client_secret", Type: "string", Description: "Keycloak OIDC client secret", Required: false},
		}
	case "headlamp":
		return []ServiceOption{
			{Name: "oidc_client_secret", Type: "string", Description: "Headlamp OIDC client secret", Required: false},
		}
	case "weave-gitops":
		return []ServiceOption{
			{Name: "password_hash", Type: "string", Description: "Weave GitOps admin password hash (bcrypt)", Required: true},
			{Name: "password", Type: "string", Description: "Weave GitOps admin password", Required: false},
		}
	case "kube-prometheus-stack":
		return []ServiceOption{
			{Name: "admin_password", Type: "string", Description: "Grafana admin password", Required: true},
		}
	case "alert-proxy":
		return []ServiceOption{
			{Name: "core_device_id", Type: "string", Description: "Alert proxy core device ID", Required: true},
			{Name: "account_service_token", Type: "string", Description: "Alert proxy account service token", Required: true},
			{Name: "core_account_number", Type: "string", Description: "Alert proxy core account number", Required: true},
		}
	case "vsphere-csi":
		return []ServiceOption{
			{Name: "vcenter_host", Type: "string", Description: "vCenter server hostname or IP address", Required: true},
			{Name: "username", Type: "string", Description: "vCenter username", Required: true},
			{Name: "password", Type: "string", Description: "vCenter password", Required: true},
			{Name: "datacenters", Type: "string", Description: "Comma-separated list of datacenters", Required: true},
			{Name: "insecure_flag", Type: "string", Description: "Skip SSL certificate verification (true/false)", Required: false},
			{Name: "port", Type: "string", Description: "vCenter port (default: 443)", Required: false},
		}
	default:
		return []ServiceOption{}
	}
}

// isEnabled checks if a service is enabled using reflection
func isEnabled(svc any) bool {
	val := reflect.ValueOf(svc)
	if svcMap, ok := svc.(map[string]any); ok {
		if enabled, ok := svcMap["enabled"].(bool); ok {
			return enabled
		}
	}
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		enabledField := val.FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.Kind() == reflect.Bool {
			return enabledField.Bool()
		}
	}
	return false
}

// setEnabled sets the Enabled field of a service using reflection
func setEnabled(svc any, enabled bool) error {
	if svcMap, ok := svc.(map[string]any); ok {
		svcMap["enabled"] = enabled
		return nil
	}

	val := reflect.ValueOf(svc)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	} else {
		return fmt.Errorf("service config must be a pointer to set fields")
	}

	if val.Kind() == reflect.Struct {
		enabledField := val.FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.CanSet() && enabledField.Kind() == reflect.Bool {
			enabledField.SetBool(enabled)
			return nil
		}
	}
	return fmt.Errorf("cannot set Enabled field")
}

// getStatus gets the Status field of a service using reflection
// getAdoptionMode extracts the adoption mode from a service config using reflection.
func getAdoptionMode(svc any) string {
	val := reflect.ValueOf(svc)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		// Check BaseConfig.AdoptionMode
		baseField := val.FieldByName("BaseConfig")
		if baseField.IsValid() && baseField.Kind() == reflect.Struct {
			modeField := baseField.FieldByName("AdoptionMode")
			if modeField.IsValid() && modeField.Kind() == reflect.String {
				mode := modeField.String()
				if mode != "" {
					return mode
				}
			}
		}
		// Direct field
		modeField := val.FieldByName("AdoptionMode")
		if modeField.IsValid() && modeField.Kind() == reflect.String {
			mode := modeField.String()
			if mode != "" {
				return mode
			}
		}
	}
	return "managed"
}
