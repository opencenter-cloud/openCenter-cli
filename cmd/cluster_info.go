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
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	corepaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/spf13/cobra"
)

func newClusterInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [name]",
		Short: "Show configuration for a cluster",
		Long: `Show configuration for a cluster.

The cluster name can be specified in two formats:
  - cluster-name (uses organization from config)
  - organization/cluster-name (explicit organization)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			identifier, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			var isActiveCluster bool
			if len(args) == 0 {
				isActiveCluster = true
			}

			ctx := cmd.Context()
			cfg, clusterName, organization, err := loadConfigWithIdentifier(ctx, identifier)
			if err != nil {
				return err
			}

			// Handle --export-only flag
			exportOnly, _ := cmd.Flags().GetBool("export-only")
			if exportOnly {
				shellOverride, _ := cmd.Flags().GetString("shell")
				return handleExportOnly(cmd, clusterName, shellOverride)
			}

			// Handle --validate flag
			validate, _ := cmd.Flags().GetBool("validate")
			if validate {
				nativeCfg, _, _, _, err := loadNativeV2ConfigWithIdentifier(ctx, identifier)
				if err != nil {
					return fmt.Errorf("failed to load native config: %w", err)
				}
				if err := validateNativeV2Config(nativeCfg); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					return fmt.Errorf("validation failed")
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Validation successful.")
				return nil
			}

			// Get the full path to the config file
			configPath, err := getConfigPath(ctx, clusterName, organization)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			// Check if we're in the git directory to show "Active cluster" prefix
			isInGitDir := false
			if cfg.OpenCenter.GitOps.GitDir != "" {
				cwd, err := os.Getwd()
				if err == nil {
					gitDir := corepaths.ExpandPath(cfg.OpenCenter.GitOps.GitDir)

					if absGitDir, err := filepath.Abs(gitDir); err == nil {
						if absCwd, err := filepath.Abs(cwd); err == nil {
							isInGitDir = (absCwd == absGitDir)
						}
					}
				}
			}

			// Output format
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				// Print full config in JSON format including cluster_name
				output := map[string]any{
					"config_path":  configPath,
					"cluster_name": cfg.OpenCenter.Cluster.ClusterName,
					"organization": cfg.OpenCenter.Meta.Organization,
					"provider":     cfg.OpenCenter.Infrastructure.Provider,
					"metadata":     cfg.OpenCenter.Meta,
					"git_dir":      cfg.OpenCenter.GitOps.GitDir,
					"git_url":      cfg.OpenCenter.GitOps.GitURL,
				}
				b, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			// Print metadata and config path in human-readable format
			// Show "Active cluster:" if this is the active cluster or we're in the git directory
			displayName := identifier
			if isActiveCluster || isInGitDir {
				fmt.Fprintf(cmd.OutOrStdout(), "Active cluster: %s\n", displayName)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", displayName)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Config Path: %s\n\n", configPath)

			// Print GitOps configuration
			if cfg.OpenCenter.GitOps.GitDir != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "git_dir: %s\n", cfg.OpenCenter.GitOps.GitDir)
			}
			if cfg.OpenCenter.GitOps.GitURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "git_url: %s\n", cfg.OpenCenter.GitOps.GitURL)
			}
			fmt.Fprintln(cmd.OutOrStdout())

			fmt.Fprintln(cmd.OutOrStdout(), "Metadata:")

			// Create a combined metadata output that includes both Meta and cluster_name
			metadataOutput := map[string]any{
				"name":         cfg.OpenCenter.Meta.Name,
				"cluster_name": cfg.OpenCenter.Cluster.ClusterName,
				"organization": cfg.OpenCenter.Meta.Organization,
				"provider":     cfg.OpenCenter.Infrastructure.Provider,
				"env":          cfg.OpenCenter.Meta.Env,
				"region":       cfg.OpenCenter.Meta.Region,
				"status":       cfg.OpenCenter.Meta.Status,
			}

			data, err := yaml.Marshal(metadataOutput)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))

			// Print enabled services
			if err := printEnabledServices(cmd, &cfg); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nWarning: Failed to retrieve service information: %v\n", err)
			}

			// Print GitOps status if cluster is in bootstrap stage or later
			status := strings.ToLower(cfg.OpenCenter.Meta.Status)
			if status == "deployed" || status == "bootstrap" {
				if err := printGitOpsStatus(cmd, &cfg, clusterName); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "\nWarning: Failed to retrieve GitOps status: %v\n", err)
				}
			}

			// Check lock status with detailed information
			lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
			if err == nil {
				lockInfo, err := lockMgr.GetLockInfo(clusterName)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "\nWarning: Failed to read lock info: %v\n", err)
				} else if lockInfo != nil {
					// Lock exists - show detailed information
					fmt.Fprintln(cmd.OutOrStdout(), "\nLock Status:")
					fmt.Fprintln(cmd.OutOrStdout(), "  status: locked")

					// Show operation if available
					if operation, ok := lockInfo.Metadata["operation"]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "  operation: %s\n", operation)
					}
					if command, ok := lockInfo.Metadata["command"]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "  command: %s\n", command)
					}

					// Parse owner to extract hostname and PID
					owner := lockInfo.Owner
					hostname := owner
					pid := ""
					if colonIdx := -1; colonIdx < len(owner) {
						for i := 0; i < len(owner); i++ {
							if owner[i] == ':' {
								colonIdx = i
								break
							}
						}
						if colonIdx > 0 {
							hostname = owner[:colonIdx]
							pid = owner[colonIdx+1:]
						}
					}

					fmt.Fprintf(cmd.OutOrStdout(), "  owner: %s\n", owner)
					if hostname != owner {
						fmt.Fprintf(cmd.OutOrStdout(), "  hostname: %s\n", hostname)
						fmt.Fprintf(cmd.OutOrStdout(), "  pid: %s\n", pid)
					}

					// Show timestamps
					if !lockInfo.AcquiredAt.IsZero() {
						fmt.Fprintf(cmd.OutOrStdout(), "  acquired_at: %s\n", lockInfo.AcquiredAt.Format(time.RFC3339))
						fmt.Fprintf(cmd.OutOrStdout(), "  acquired_ago: %s\n", time.Since(lockInfo.AcquiredAt).Round(time.Second))
					}

					if !lockInfo.ExpiresAt.IsZero() {
						fmt.Fprintf(cmd.OutOrStdout(), "  expires_at: %s\n", lockInfo.ExpiresAt.Format(time.RFC3339))
						if time.Now().Before(lockInfo.ExpiresAt) {
							fmt.Fprintf(cmd.OutOrStdout(), "  expires_in: %s\n", time.Until(lockInfo.ExpiresAt).Round(time.Second))
						} else {
							fmt.Fprintln(cmd.OutOrStdout(), "  expires_in: expired (stale lock)")
						}
					}

					if lockInfo.TTL > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "  ttl: %s\n", lockInfo.TTL)
					}

					// Add helpful message
					if operation, ok := lockInfo.Metadata["operation"]; ok {
						fmt.Fprintf(cmd.OutOrStdout(), "  message: %s operation is in progress on this cluster\n", operation)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "  message: Another operation is in progress on this cluster")
					}

					// Check if process is still running (on Unix-like systems)
					if pid != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  hint: Check if process %s is still running, or remove stale lock file\n", pid)
					}
				} else {
					// No lock exists
					fmt.Fprintln(cmd.OutOrStdout(), "\nLock Status:")
					fmt.Fprintln(cmd.OutOrStdout(), "  status: available")
				}
			}

			return nil
		},
	}
	cmd.Flags().Bool("validate", false, "validate cluster configuration invariants")
	cmd.Flags().Bool("json", false, "output JSON instead of YAML")
	cmd.Flags().Bool("export-only", false, "only output export commands for shell evaluation")
	cmd.Flags().String("shell", "", "override shell detection (bash, zsh, fish, powershell)")
	return cmd
}

// handleExportOnly handles the --export-only flag for cluster info command.
func handleExportOnly(cmd *cobra.Command, clusterName string, shellOverride string) error {
	// Generate cluster select output to get export commands
	output, err := generateClusterSelectOutput(clusterName, shellOverride)
	if err != nil {
		return err
	}

	// Only output export commands
	for _, command := range output.ExportCommands {
		fmt.Fprintln(cmd.OutOrStdout(), command)
	}

	return nil
}

// printEnabledServices prints the list of enabled services from the configuration
func printEnabledServices(cmd *cobra.Command, cfg *config.Config) error {
	fmt.Fprintln(cmd.OutOrStdout(), "\nEnabled Services:")

	// Collect enabled services from cfg.OpenCenter.Services.
	// After YAML unmarshaling, values are typed service config structs
	// (e.g. *services.CertManagerConfig) that embed BaseConfig and
	// implement the IsEnabled()/GetStatus() interface.
	enabledServices := []string{}
	for serviceName, serviceConfig := range cfg.OpenCenter.Services {
		if svc, ok := serviceConfig.(interface{ IsEnabled() bool }); ok && svc.IsEnabled() {
			status := "unknown"
			if statusGetter, ok := serviceConfig.(interface{ GetStatus() string }); ok {
				if s := statusGetter.GetStatus(); s != "" {
					status = s
				}
			}
			enabledServices = append(enabledServices, fmt.Sprintf("  - %s (status: %s)", serviceName, status))
		}
	}

	// Sort for consistent output
	if len(enabledServices) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  No services enabled")
	} else {
		// Sort alphabetically
		sortStrings(enabledServices)
		for _, service := range enabledServices {
			fmt.Fprintln(cmd.OutOrStdout(), service)
		}
	}

	return nil
}

// sortStrings sorts a slice of strings in place
func sortStrings(s []string) {
	// Simple bubble sort for small lists
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// printGitOpsStatus prints the GitOps reconciliation status using kubectl
func printGitOpsStatus(cmd *cobra.Command, cfg *config.Config, clusterName string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "\nGitOps Status:")

	// Check if kubeconfig exists
	kubeconfigPath := getKubeconfigPath(cfg, clusterName)
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		fmt.Fprintln(cmd.OutOrStdout(), "  Kubeconfig not found - cluster may not be deployed yet")
		return nil
	}

	// Try to get Flux Kustomization status
	ctx := cmd.Context()
	kustomizations, err := getFluxKustomizations(ctx, kubeconfigPath)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  Unable to retrieve GitOps status: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "  Hint: Ensure kubectl is installed and cluster is accessible")
		return nil
	}

	if len(kustomizations) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  No Flux Kustomizations found - FluxCD may not be bootstrapped yet")
		return nil
	}

	// Print kustomization status
	fmt.Fprintf(cmd.OutOrStdout(), "  Kustomizations: %d total\n", len(kustomizations))

	readyCount := 0
	for _, k := range kustomizations {
		if k.Ready {
			readyCount++
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "  Ready: %d/%d\n", readyCount, len(kustomizations))

	// Show details of non-ready kustomizations
	if readyCount < len(kustomizations) {
		fmt.Fprintln(cmd.OutOrStdout(), "  Not Ready:")
		for _, k := range kustomizations {
			if !k.Ready {
				fmt.Fprintf(cmd.OutOrStdout(), "    - %s/%s: %s\n", k.Namespace, k.Name, k.Message)
			}
		}
	}

	return nil
}

// getKubeconfigPath returns the path to the kubeconfig file for the cluster
func getKubeconfigPath(cfg *config.Config, clusterName string) string {
	// Try to get from GitOps directory first
	if cfg.OpenCenter.GitOps.GitDir != "" {
		gitDir := corepaths.ExpandPath(cfg.OpenCenter.GitOps.GitDir)

		kubeconfigPath := filepath.Join(gitDir, "infrastructure", "clusters", clusterName, "kubeconfig.yaml")
		if _, err := os.Stat(kubeconfigPath); err == nil {
			return kubeconfigPath
		}
	}

	// Fallback to default location
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}

// FluxKustomization represents a Flux Kustomization resource
type FluxKustomization struct {
	Name      string
	Namespace string
	Ready     bool
	Message   string
}

// getFluxKustomizations retrieves Flux Kustomization status using kubectl
func getFluxKustomizations(ctx context.Context, kubeconfigPath string) ([]FluxKustomization, error) {
	// Use kubectl to get kustomizations
	cmd, err := security.GetDefaultCommandRunner().PrepareCommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfigPath,
		"get", "kustomizations.kustomize.toolkit.fluxcd.io",
		"-A",
		"-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare kubectl command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl command failed: %w", err)
	}

	// Parse JSON output
	var result struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type    string `json:"type"`
					Status  string `json:"status"`
					Message string `json:"message"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse kubectl output: %w", err)
	}

	kustomizations := make([]FluxKustomization, 0, len(result.Items))
	for _, item := range result.Items {
		k := FluxKustomization{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Ready:     false,
			Message:   "Unknown",
		}

		// Check Ready condition
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" {
				k.Ready = (cond.Status == "True")
				k.Message = cond.Message
				break
			}
		}

		kustomizations = append(kustomizations, k)
	}

	return kustomizations, nil
}
