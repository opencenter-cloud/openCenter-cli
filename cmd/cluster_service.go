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
			// Create new service config
			newService := config.ServiceCfg{Enabled: true}
			// Process parameters
			if err := processParams(params, &newService); err != nil {
				return err
			}
			// Process secrets
			if err := processSecrets(secrets, serviceName, &cfg.Secrets); err != nil {
				return err
			}
			// Custom validation logic (validate before checking if already enabled)
			if err := validateService(serviceName, &newService, &cfg.Secrets); err != nil {
				return err
			}
			// Check if service already exists and is enabled (unless --force is used)
			if !force {
				if svc, exists := cfg.OpenCenter.Services[serviceName]; exists && svc.Enabled {
					return fmt.Errorf("service '%s' is already enabled. Use --force to re-enable", serviceName)
				}
				if svc, exists := cfg.OpenCenter.ManagedService[serviceName]; exists && svc.Enabled {
					return fmt.Errorf("managed service '%s' is already enabled. Use --force to re-enable", serviceName)
				}
			}
			// Enable service in the appropriate map
			if isManaged {
				if cfg.OpenCenter.ManagedService == nil {
					cfg.OpenCenter.ManagedService = make(map[string]config.ServiceCfg)
				}
				cfg.OpenCenter.ManagedService[serviceName] = newService
				if force {
					fmt.Fprintf(cmd.OutOrStdout(), "Re-enabling managed service '%s' in cluster '%s'...\n", serviceName, clusterName)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Enabling managed service '%s' in cluster '%s'...\n", serviceName, clusterName)
				}
			} else {
				if cfg.OpenCenter.Services == nil {
					cfg.OpenCenter.Services = make(map[string]config.ServiceCfg)
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
				if !svc.Enabled {
					return fmt.Errorf("managed service '%s' is already disabled", serviceName)
				}
				svc.Enabled = false
				cfg.OpenCenter.ManagedService[serviceName] = svc
				fmt.Fprintf(cmd.OutOrStdout(), "Disabling managed service '%s' in cluster '%s'...\n", serviceName, clusterName)
			} else {
				svc, exists := cfg.OpenCenter.Services[serviceName]
				if !exists {
					return fmt.Errorf("service '%s' not found", serviceName)
				}
				if !svc.Enabled {
					return fmt.Errorf("service '%s' is already disabled", serviceName)
				}
				svc.Enabled = false
				cfg.OpenCenter.Services[serviceName] = svc
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
func processParams(params []string, serviceCfg *config.ServiceCfg) error {
	v := reflect.ValueOf(serviceCfg).Elem()
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
func validateService(serviceName string, serviceCfg *config.ServiceCfg, secretsCfg *config.Secrets) error {
	switch serviceName {
	case "cert-manager":
		if serviceCfg.Email == "" {
			return fmt.Errorf("missing required parameter 'email' for service 'cert-manager'.\nExample: --param=\"email=your-email@example.com\"")
		}
	case "loki":
		storageType := serviceCfg.LokiStorageType
		if storageType == "" {
			storageType = "swift" // default
		}

		if storageType == "swift" {
			// Check for application credentials (recommended) or legacy credentials
			hasAppCreds := serviceCfg.SwiftApplicationCredentialID != "" && secretsCfg.Loki.SwiftApplicationCredentialSecret != ""
			hasLegacyCreds := serviceCfg.SwiftUsername != "" && secretsCfg.Loki.SwiftPassword != ""

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
				if svc.Enabled {
					enabledStr = "enabled"
				}
				status := svc.Status
				if status == "" {
					status = "-"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-15s %-15s\n", name, enabledStr, status)
			}

			// Print managed services
			for name, svc := range cfg.OpenCenter.ManagedService {
				enabledStr := "disabled"
				if svc.Enabled {
					enabledStr = "enabled"
				}
				status := svc.Status
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
