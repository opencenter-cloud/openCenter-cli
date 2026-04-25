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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	corepaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

// ServiceSyncResult holds the result of syncing a single service's status.
type ServiceSyncResult struct {
	Name      string `json:"name"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	Changed   bool   `json:"changed"`
	Error     string `json:"error,omitempty"`
}

// SyncStatusResult holds the overall result of the sync-status operation.
type SyncStatusResult struct {
	ClusterName    string              `json:"cluster_name"`
	ServicesTotal  int                 `json:"services_total"`
	Servicessynced int                 `json:"services_synced"`
	ServicesFailed int                 `json:"services_failed"`
	Results        []ServiceSyncResult `json:"results"`
}

func newClusterSyncStatusCmd() *cobra.Command {
	var dryRun bool
	var outputJSON bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "sync-status [name]",
		Short: "Sync service status from live cluster to configuration",
		Long: `Sync service status from the live Kubernetes cluster back to the configuration file.

This command queries Flux HelmReleases and Kustomizations for each enabled service
and updates the 'status' field in the cluster configuration file.

Status mapping:
  - Ready=True                    → success
  - Ready=False + Reconciling     → running
  - Ready=False + Error/Stalled   → failed
  - Resource not found            → pending

The command requires:
  - A deployed cluster with accessible kubeconfig
  - FluxCD installed and managing services`,
		Example: `  # Sync status for active cluster
  opencenter cluster sync-status

  # Sync status for a specific cluster
  opencenter cluster sync-status my-cluster

  # Preview changes without saving (dry-run)
  opencenter cluster sync-status my-cluster --dry-run

  # Output results as JSON
  opencenter cluster sync-status my-cluster --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			cfg, clusterName, _, err := loadConfigWithIdentifier(ctx, identifier)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Get kubeconfig path
			kubeconfigPath := getKubeconfigPathForSync(&cfg, clusterName)
			if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
				return fmt.Errorf("kubeconfig not found at %s - cluster may not be deployed yet", kubeconfigPath)
			}

			// Collect enabled services
			enabledServices := collectEnabledServices(&cfg)

			result := SyncStatusResult{
				ClusterName:   clusterName,
				ServicesTotal: len(enabledServices),
				Results:       make([]ServiceSyncResult, 0, len(enabledServices)),
			}

			if len(enabledServices) == 0 {
				if outputJSON {
					return outputSyncResultJSON(cmd, &result)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No enabled services found in configuration")
				return nil
			}

			// Query live status for each service
			for serviceName, serviceConfig := range enabledServices {
				syncResult := syncServiceStatus(ctx, kubeconfigPath, serviceName, serviceConfig)
				result.Results = append(result.Results, syncResult)

				if syncResult.Error != "" {
					result.ServicesFailed++
				} else if syncResult.Changed {
					result.Servicessynced++
				}
			}

			if !outputJSON {
				printSyncResults(cmd, &result, dryRun)
			}

			// Save config if not dry-run and there were changes
			if !dryRun && result.Servicessynced > 0 {
				// Apply status updates to config
				for _, sr := range result.Results {
					if sr.Changed && sr.Error == "" {
						updateServiceStatus(&cfg, sr.Name, sr.NewStatus)
					}
				}

				// Update metadata timestamp
				cfg.Metadata.UpdatedAt = time.Now().Format(time.RFC3339Nano)

				if err := saveConfig(ctx, cfg); err != nil {
					return fmt.Errorf("failed to save configuration: %w", err)
				}

				if !outputJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "\nConfiguration saved with updated service statuses\n")
				}
			} else if dryRun && result.Servicessynced > 0 {
				if !outputJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "\nDry-run mode: no changes saved\n")
				}
			}

			if outputJSON {
				return outputSyncResultJSON(cmd, &result)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without saving to configuration")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "output results as JSON")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "timeout for cluster queries")

	return cmd
}

func runClusterStatusSync(cmd *cobra.Command, args []string, dryRun bool, outputFormat OutputFormat, timeout time.Duration) error {
	if outputFormat == OutputYAML {
		return fmt.Errorf("cluster status --sync does not support yaml output yet; use --output text or --output json")
	}

	syncArgs := append([]string{}, args...)
	if dryRun {
		syncArgs = append(syncArgs, "--dry-run")
	}
	if outputFormat == OutputJSON {
		syncArgs = append(syncArgs, "--json")
	}
	if timeout > 0 {
		syncArgs = append(syncArgs, "--timeout", timeout.String())
	}

	syncCmd := newClusterSyncStatusCmd()
	syncCmd.SetContext(cmd.Context())
	syncCmd.SetOut(cmd.OutOrStdout())
	syncCmd.SetErr(cmd.ErrOrStderr())
	syncCmd.SetArgs(syncArgs)
	return syncCmd.Execute()
}

// getKubeconfigPathForSync returns the kubeconfig path for the cluster.
func getKubeconfigPathForSync(cfg *v2.Config, clusterName string) string {
	if cfg.OpenCenter.GitOps.Repository.LocalDir != "" {
		gitDir := corepaths.ExpandPath(cfg.OpenCenter.GitOps.Repository.LocalDir)
		kubeconfigPath := filepath.Join(gitDir, "infrastructure", "clusters", clusterName, "kubeconfig.yaml")
		if _, err := os.Stat(kubeconfigPath); err == nil {
			return kubeconfigPath
		}
	}

	// Fallback to default location
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}

// collectEnabledServices returns a map of enabled services from the config.
func collectEnabledServices(cfg *v2.Config) map[string]any {
	enabled := make(map[string]any)

	for serviceName, serviceConfig := range cfg.OpenCenter.Services {
		if svc, ok := serviceConfig.(interface{ IsEnabled() bool }); ok && svc.IsEnabled() {
			enabled[serviceName] = serviceConfig
		}
	}

	return enabled
}

// syncServiceStatus queries the live cluster for a service's status.
func syncServiceStatus(ctx context.Context, kubeconfigPath, serviceName string, serviceConfig any) ServiceSyncResult {
	result := ServiceSyncResult{
		Name: serviceName,
	}

	// Get current status from config
	if statusGetter, ok := serviceConfig.(interface{ GetStatus() string }); ok {
		result.OldStatus = statusGetter.GetStatus()
		if result.OldStatus == "" {
			result.OldStatus = "unknown"
		}
	} else {
		result.OldStatus = "unknown"
	}

	// Get namespace for the service
	namespace := getServiceNamespace(serviceName, serviceConfig)

	// Query HelmRelease status first
	newStatus, err := queryFluxHelmReleaseStatus(ctx, kubeconfigPath, serviceName, namespace)
	if err != nil {
		// Try Kustomization if HelmRelease not found
		newStatus, err = queryFluxKustomizationStatus(ctx, kubeconfigPath, serviceName)
		if err != nil {
			// Resource not found - mark as pending
			newStatus = "pending"
		}
	}

	result.NewStatus = newStatus
	result.Changed = result.OldStatus != result.NewStatus

	return result
}

// getServiceNamespace returns the namespace for a service.
func getServiceNamespace(serviceName string, serviceConfig any) string {
	// Try to get namespace from config
	if nsGetter, ok := serviceConfig.(interface{ GetNamespace() string }); ok {
		if ns := nsGetter.GetNamespace(); ns != "" {
			return ns
		}
	}

	// Default namespace mappings for common services
	namespaceMap := map[string]string{
		"cert-manager":      "cert-manager",
		"calico":            "calico-system",
		"cilium":            "kube-system",
		"gateway":           "gateway-system",
		"gateway-api":       "gateway-system",
		"keycloak":          "keycloak",
		"headlamp":          "headlamp",
		"prometheus-stack":  "monitoring",
		"loki":              "monitoring",
		"tempo":             "monitoring",
		"grafana":           "monitoring",
		"harbor":            "harbor",
		"velero":            "velero",
		"metallb":           "metallb-system",
		"olm":               "olm",
		"rbac-manager":      "rbac-manager",
		"postgres-operator": "postgres-operator",
		"fluxcd":            "flux-system",
		"sources":           "flux-system",
	}

	if ns, ok := namespaceMap[serviceName]; ok {
		return ns
	}

	// Default to service name as namespace
	return serviceName
}

// queryFluxHelmReleaseStatus queries the status of a Flux HelmRelease.
func queryFluxHelmReleaseStatus(ctx context.Context, kubeconfigPath, name, namespace string) (string, error) {
	runner := security.GetDefaultCommandRunner()

	cmd, err := runner.PrepareCommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "helmrelease", name,
		"-n", namespace,
		"-o", "json")
	if err != nil {
		return "", fmt.Errorf("failed to prepare kubectl command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("helmrelease not found: %w", err)
	}

	return parseFluxResourceStatus(output)
}

// queryFluxKustomizationStatus queries the status of a Flux Kustomization.
func queryFluxKustomizationStatus(ctx context.Context, kubeconfigPath, name string) (string, error) {
	runner := security.GetDefaultCommandRunner()

	cmd, err := runner.PrepareCommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "kustomization", name,
		"-n", "flux-system",
		"-o", "json")
	if err != nil {
		return "", fmt.Errorf("failed to prepare kubectl command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kustomization not found: %w", err)
	}

	return parseFluxResourceStatus(output)
}

// parseFluxResourceStatus parses the status from a Flux resource JSON.
func parseFluxResourceStatus(jsonData []byte) (string, error) {
	var resource struct {
		Status struct {
			Conditions []struct {
				Type    string `json:"type"`
				Status  string `json:"status"`
				Reason  string `json:"reason"`
				Message string `json:"message"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.Unmarshal(jsonData, &resource); err != nil {
		return "", fmt.Errorf("failed to parse resource status: %w", err)
	}

	// Check conditions for Ready status
	for _, cond := range resource.Status.Conditions {
		if cond.Type == "Ready" {
			if cond.Status == "True" {
				return "success", nil
			}

			// Check reason for more specific status
			reason := strings.ToLower(cond.Reason)
			switch {
			case strings.Contains(reason, "progress") || strings.Contains(reason, "reconcil"):
				return "running", nil
			case strings.Contains(reason, "fail") || strings.Contains(reason, "error") || strings.Contains(reason, "stall"):
				return "failed", nil
			default:
				return "running", nil
			}
		}
	}

	// No Ready condition found
	return "pending", nil
}

// updateServiceStatus updates the status field of a service in the config.
func updateServiceStatus(cfg *v2.Config, serviceName, status string) {
	if cfg.OpenCenter.Services == nil {
		return
	}

	serviceConfig, exists := cfg.OpenCenter.Services[serviceName]
	if !exists {
		return
	}

	// All service configs embed BaseConfig which has the Status field.
	// Use reflection to find and set the Status field in the embedded BaseConfig.
	setStatusViaReflection(serviceConfig, status)
}

// setStatusViaReflection sets the Status field on a service config using reflection.
// It handles both direct BaseConfig and embedded BaseConfig in other structs.
func setStatusViaReflection(serviceConfig any, status string) {
	val := reflect.ValueOf(serviceConfig)

	// Dereference pointer if needed
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return
	}

	// Try to find Status field directly
	statusField := val.FieldByName("Status")
	if statusField.IsValid() && statusField.CanSet() && statusField.Kind() == reflect.String {
		statusField.SetString(status)
		return
	}

	// Try to find BaseConfig embedded field and set Status on it
	baseConfigField := val.FieldByName("BaseConfig")
	if baseConfigField.IsValid() && baseConfigField.Kind() == reflect.Struct {
		statusField = baseConfigField.FieldByName("Status")
		if statusField.IsValid() && statusField.CanSet() && statusField.Kind() == reflect.String {
			statusField.SetString(status)
		}
	}
}

// printSyncResults prints the sync results in human-readable format.
func printSyncResults(cmd *cobra.Command, result *SyncStatusResult, dryRun bool) {
	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Sync Status (dry-run) for cluster: %s\n\n", result.ClusterName)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Sync Status for cluster: %s\n\n", result.ClusterName)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Services: %d total, %d changed, %d failed\n\n",
		result.ServicesTotal, result.Servicessynced, result.ServicesFailed)

	// Print changed services
	hasChanges := false
	for _, sr := range result.Results {
		if sr.Changed {
			hasChanges = true
			if sr.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s: %s → %s (error: %s)\n",
					sr.Name, sr.OldStatus, sr.NewStatus, sr.Error)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s: %s → %s\n",
					sr.Name, sr.OldStatus, sr.NewStatus)
			}
		}
	}

	if !hasChanges {
		fmt.Fprintln(cmd.OutOrStdout(), "  No status changes detected")
	}
}

// outputSyncResultJSON outputs the sync result as JSON.
func outputSyncResultJSON(cmd *cobra.Command, result *SyncStatusResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
