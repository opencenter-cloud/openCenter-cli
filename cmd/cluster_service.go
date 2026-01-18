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
	"fmt"
	"reflect"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	"github.com/rackerlabs/openCenter-cli/internal/gitops"
	"github.com/spf13/cobra"
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
	return cmd
}

// newClusterServiceEnableCmd creates the "cluster service enable" command.
func newClusterServiceEnableCmd() *cobra.Command {
	var (
		isManaged bool
		params    []string
		secrets   []string
		cluster   string
		force     bool
		render    bool
	)
	cmd := &cobra.Command{
		Use:   "enable <service-name>",
		Short: "Enable a service in the cluster configuration",
		Long: `This command enables a service in the cluster configuration.
If the service requires additional parameters or secrets, they must be provided
as flags. If they are missing, the command will return an error with an example.

Examples:
  # Enable the 'cert-manager' service with a required email parameter
  openCenter cluster service enable cert-manager --param="email=admin@example.com"

  # Enable a managed service with a secret
  openCenter cluster service enable my-managed-service --managed --secret="api_key=some_secret_value"

  # Force re-enable (re-render) an already enabled service
  openCenter cluster service enable prometheus --force

  # Enable a service and immediately render its templates
  openCenter cluster service enable loki --render`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]
			// Determine cluster name
			var clusterName string
			if cluster != "" {
				clusterName = cluster
			} else {
				active, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if active == "" {
					return fmt.Errorf("no cluster selected. Use --cluster flag or 'openCenter cluster select' to select a cluster")
				}
				clusterName = active
			}
			// Load configuration
			cfg, err := config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration for '%s': %w", clusterName, err)
			}
			// Create new service config using registry to get correct type
			configType := registry.GetServiceConfigType(serviceName)
			if configType == nil {
				configType = reflect.TypeOf(services.DefaultServiceConfig{})
			}
			// Create new instance
			newService := reflect.New(configType).Interface()

			// Set Enabled = true
			if err := setEnabled(newService, true); err != nil {
				return fmt.Errorf("failed to enable service: %w", err)
			}

			// Process parameters
			if err := processParams(params, newService); err != nil {
				return err
			}
			// Process secrets
			if err := processSecrets(secrets, serviceName, &cfg.Secrets); err != nil {
				return err
			}
			// Custom validation logic (validate before checking if already enabled)
			if err := validateService(serviceName, newService, &cfg.Secrets); err != nil {
				return err
			}
			// Check if service already exists and is enabled (unless --force is used)
			if !force {
				if svc, exists := cfg.OpenCenter.Services[serviceName]; exists && isEnabled(svc) {
					return fmt.Errorf("service '%s' is already enabled. Use --force to re-enable", serviceName)
				}
				if svc, exists := cfg.OpenCenter.ManagedService[serviceName]; exists && isEnabled(svc) {
					return fmt.Errorf("managed service '%s' is already enabled. Use --force to re-enable", serviceName)
				}
			}
			// Enable service in the appropriate map
			if isManaged {
				if cfg.OpenCenter.ManagedService == nil {
					cfg.OpenCenter.ManagedService = make(config.ServiceMap)
				}
				cfg.OpenCenter.ManagedService[serviceName] = newService
				if force {
					fmt.Fprintf(cmd.OutOrStdout(), "Re-enabling managed service '%s' in cluster '%s'...\n", serviceName, clusterName)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Enabling managed service '%s' in cluster '%s'...\n", serviceName, clusterName)
				}
			} else {
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(config.ServiceMap)
				}
				cfg.OpenCenter.Services[serviceName] = newService
				if force {
					fmt.Fprintf(cmd.OutOrStdout(), "Re-enabling service '%s' in cluster '%s'...\n", serviceName, clusterName)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Enabling service '%s' in cluster '%s'...\n", serviceName, clusterName)
				}
			}
			// Save the updated configuration
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save updated configuration: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully enabled service '%s' in cluster '%s'.\n", serviceName, clusterName)

			// Render the service if --render flag is set
			if render {
				// Validate git_dir is set before rendering
				if cfg.OpenCenter.GitOps.GitDir == "" {
					return fmt.Errorf("git_dir is not configured. Run 'openCenter cluster setup' first or set git_dir in the configuration")
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Rendering service '%s'...\n", serviceName)

				if err := gitops.RenderSingleService(cfg, serviceName, isManaged); err != nil {
					return fmt.Errorf("failed to render service: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Service '%s' rendered successfully.\n", serviceName)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&isManaged, "managed", false, "Enable the service as a managed service")
	cmd.Flags().StringSliceVar(&params, "param", []string{}, "Set a service parameter (e.g., --param key=value)")
	cmd.Flags().StringSliceVar(&secrets, "secret", []string{}, "Set a service secret (e.g., --secret key=value)")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Specify the cluster name")
	cmd.Flags().BoolVar(&force, "force", false, "Force re-enable an already enabled service to re-render configuration")
	cmd.Flags().BoolVar(&render, "render", false, "Render the service templates immediately after enabling")
	return cmd
}

// newClusterServiceDisableCmd creates the "cluster service disable" command.
func newClusterServiceDisableCmd() *cobra.Command {
	var (
		isManaged bool
		cluster   string
	)
	cmd := &cobra.Command{
		Use:   "disable <service-name>",
		Short: "Disable a service in the cluster configuration",
		Long: `This command disables a service in the cluster configuration by setting its enabled flag to false.

Examples:
  # Disable the 'cert-manager' service
  openCenter cluster service disable cert-manager

  # Disable a managed service
  openCenter cluster service disable my-managed-service --managed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]
			// Determine cluster name
			var clusterName string
			if cluster != "" {
				clusterName = cluster
			} else {
				active, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if active == "" {
					return fmt.Errorf("no cluster selected. Use --cluster flag or 'openCenter cluster select' to select a cluster")
				}
				clusterName = active
			}
			// Load configuration
			cfg, err := config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration for '%s': %w", clusterName, err)
			}
			// Disable the service in the appropriate map
			if isManaged {
				svc, exists := cfg.OpenCenter.ManagedService[serviceName]
				if !exists {
					return fmt.Errorf("managed service '%s' not found", serviceName)
				}
				if !isEnabled(svc) {
					return fmt.Errorf("managed service '%s' is already disabled", serviceName)
				}
				if err := setEnabled(svc, false); err != nil {
					return fmt.Errorf("failed to disable service: %w", err)
				}
				// Map holds a pointer/interface so modifying it modifies the value in the map if it's a pointer.
				// But svc is 'any'. If it's a pointer, we are good.
				// ServiceMap values are pointers to structs.
				fmt.Fprintf(cmd.OutOrStdout(), "Disabling managed service '%s' in cluster '%s'...\n", serviceName, clusterName)
			} else {
				svc, exists := cfg.OpenCenter.Services[serviceName]
				if !exists {
					return fmt.Errorf("service '%s' not found", serviceName)
				}
				if !isEnabled(svc) {
					return fmt.Errorf("service '%s' is already disabled", serviceName)
				}
				if err := setEnabled(svc, false); err != nil {
					return fmt.Errorf("failed to disable service: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Disabling service '%s' in cluster '%s'...\n", serviceName, clusterName)
			}
			// Save the updated configuration
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save updated configuration: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully disabled service '%s' in cluster '%s'.\n", serviceName, clusterName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&isManaged, "managed", false, "Disable the service from the managed services list")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Specify the cluster name")
	return cmd
}
func processParams(params []string, serviceCfg any) error {
	v := reflect.ValueOf(serviceCfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	paramMap := make(map[string]string)
	for _, p := range params {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid parameter format: '%s'. Expected key=value", p)
		}
		paramMap[parts[0]] = parts[1]
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
		if val, ok := paramMap[jsonTag]; ok {
			fieldVal := v.Field(i)
			if !fieldVal.CanSet() {
				continue
			}
			if err := setFieldValue(fieldVal, val); err != nil {
				return fmt.Errorf("failed to set parameter '%s': %w", jsonTag, err)
			}
		}
	}
	return nil
}
func processSecrets(secrets []string, serviceName string, secretsCfg *config.Secrets) error {
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
func validateService(serviceName string, serviceCfg any, secretsCfg *config.Secrets) error {
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
				storageType = "swift" // default
			}

			if storageType == "swift" {
				// Check for application credentials (recommended) or legacy credentials
				hasAppCreds := cfg.SwiftApplicationCredentialID != "" && secretsCfg.Loki.SwiftApplicationCredentialSecret != ""
				hasLegacyCreds := cfg.SwiftUsername != "" && secretsCfg.Loki.SwiftPassword != ""

				if !hasAppCreds && !hasLegacyCreds {
					return fmt.Errorf("missing required Swift credentials for service 'loki'.\nRecommended: --param=\"swift_application_credential_id=your-app-cred-id\" --secret=\"swift_application_credential_secret=your-secret\"\nOr legacy: --param=\"swift_username=your-username\" --secret=\"swift_password=your-password\"")
				}
			} else if storageType == "s3" {
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
	}
	return nil
}

// newClusterServiceStatusCmd creates the "cluster service status" command.
func newClusterServiceStatusCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display status of all services in the cluster configuration",
		Long: `This command displays the status of all services (both standard and managed) 
in a three-column format showing service name, enabled/disabled state, and deployment status.

Examples:
  # Show status of all services in the active cluster
  openCenter cluster service status

  # Show status for a specific cluster
  openCenter cluster service status --cluster my-cluster`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine cluster name
			var clusterName string
			if cluster != "" {
				clusterName = cluster
			} else {
				active, err := config.GetActive()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
				if active == "" {
					return fmt.Errorf("no cluster selected. Use --cluster flag or 'openCenter cluster select' to select a cluster")
				}
				clusterName = active
			}

			// Load configuration
			cfg, err := config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration for '%s': %w", clusterName, err)
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-15s\n", "SERVICE NAME", "ENABLED", "STATUS")
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-15s\n", strings.Repeat("-", 30), strings.Repeat("-", 15), strings.Repeat("-", 15))

			// Print standard services
			for name, svc := range cfg.OpenCenter.Services {
				enabledStr := "disabled"
				if isEnabled(svc) {
					enabledStr = "enabled"
				}
				status := getStatus(svc)
				if status == "" {
					status = "-"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-15s\n", name, enabledStr, status)
			}

			// Print managed services
			for name, svc := range cfg.OpenCenter.ManagedService {
				enabledStr := "disabled"
				if isEnabled(svc) {
					enabledStr = "enabled"
				}
				status := getStatus(svc)
				if status == "" {
					status = "-"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-15s\n", name+" (managed)", enabledStr, status)
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
  openCenter cluster service options cert-manager

  # Show options for loki
  openCenter cluster service options loki

  # Show options for a managed service
  openCenter cluster service options alert-proxy --managed`,
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
			fmt.Fprintln(cmd.OutOrStdout(), "  status (string) - Service deployment status (pending/running/success/failed)")
			fmt.Fprintln(cmd.OutOrStdout(), "  release (string) - Release version or tag (mutually exclusive with branch)")
			fmt.Fprintln(cmd.OutOrStdout(), "  branch (string) - Git branch (mutually exclusive with release)")
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
				fmt.Fprintf(cmd.OutOrStdout(), "  openCenter cluster service enable %s --param=\"%s=value\"\n", serviceName, exampleParam)
			}
			if len(secrets) > 0 {
				exampleSecret := secrets[0].Name
				fmt.Fprintf(cmd.OutOrStdout(), "  openCenter cluster service enable %s --secret=\"%s=secret-value\"\n", serviceName, exampleSecret)
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
			{Name: "loki_storage_type", Type: "string", Description: "Storage backend type (s3 or swift)", Required: false},
			{Name: "loki_bucket_name", Type: "string", Description: "Storage bucket/container name", Required: true},
			{Name: "loki_volume_size", Type: "integer", Description: "Persistent volume size in GB", Required: false},
			{Name: "loki_storage_class", Type: "string", Description: "Storage class", Required: false},
			{Name: "swift_auth_url", Type: "string", Description: "Swift Keystone V3 authentication URL (for Swift storage)", Required: false},
			{Name: "swift_region", Type: "string", Description: "Swift region name (for Swift storage)", Required: false},
			{Name: "swift_auth_version", Type: "integer", Description: "Swift authentication version (default: 3)", Required: false},
			{Name: "swift_application_credential_id", Type: "string", Description: "Swift application credential ID (recommended)", Required: false},
			{Name: "swift_container_name", Type: "string", Description: "Swift container name", Required: false},
			{Name: "loki_s3_endpoint", Type: "string", Description: "S3 endpoint URL (for S3 storage, e.g., MinIO)", Required: false},
			{Name: "loki_s3_region", Type: "string", Description: "S3 region (for S3 storage)", Required: false},
			{Name: "loki_s3_force_path_style", Type: "boolean", Description: "Force S3 path style (required for MinIO)", Required: false},
			{Name: "loki_s3_insecure", Type: "boolean", Description: "Allow insecure S3 connections", Required: false},
		}
	case "keycloak":
		return []ServiceOption{
			{Name: "keycloak_realm", Type: "string", Description: "Keycloak realm name", Required: false},
			{Name: "keycloak_frontend_url", Type: "string", Description: "Keycloak frontend URL", Required: false},
			{Name: "keycloak_client_id", Type: "string", Description: "Keycloak client ID", Required: false},
		}
	case "headlamp":
		return []ServiceOption{
			{Name: "headlamp_oidc_issuer_url", Type: "string", Description: "Headlamp OIDC issuer URL", Required: false},
			{Name: "headlamp_oidc_client_id", Type: "string", Description: "Headlamp OIDC client ID", Required: false},
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
			{Name: "velero_backup_bucket", Type: "string", Description: "Velero backup bucket name", Required: false},
			{Name: "velero_region", Type: "string", Description: "Velero backup region", Required: false},
		}
	case "alert-proxy":
		return []ServiceOption{
			{Name: "alert_manager_base_url", Type: "string", Description: "Alert manager base URL", Required: false},
			{Name: "http_route_fqdn", Type: "string", Description: "HTTPRoute fully qualified domain name", Required: false},
		}
	case "calico":
		return []ServiceOption{
			{Name: "calico_kube_api_server", Type: "string", Description: "Calico Kubernetes API server address", Required: false},
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
func getStatus(svc any) string {
	val := reflect.ValueOf(svc)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		statusField := val.FieldByName("Status")
		if statusField.IsValid() && statusField.Kind() == reflect.String {
			return statusField.String()
		}
	}
	return ""
}
