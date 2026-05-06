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
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

// newClusterStatusCmd creates the "cluster status" command.
func newClusterStatusCmd() *cobra.Command {
	var showPaths bool
	var quiet bool
	var syncStatus bool
	var refresh bool
	var syncTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show cluster status information",
		Long: `Show cluster status information.

This command displays:
- The requested cluster, or the currently active cluster when no name is passed
- Basic cluster metadata (environment, region, organization)
- Cluster lifecycle state (stage and status)
- Network and node inventory from local configuration and OpenTofu state
- Key file paths (with --paths flag)

By default this command is offline and does not contact Kubernetes or provider APIs.
Use --refresh to collect live Kubernetes node IPs and API readiness.

If no cluster is requested and no cluster is active, it will show available clusters
and suggest using 'opencenter cluster use' to set one.`,
		Example: `  # Show active cluster status
  opencenter cluster status

  # Show a specific cluster
  opencenter cluster status my-cluster

  # Show active cluster with file paths
  opencenter cluster status --paths

  # Refresh status from live Kubernetes/provider checks
  opencenter cluster status my-cluster --refresh

  # Sync service status from the live cluster into configuration
  opencenter cluster status my-cluster --sync

  # Quiet output (just the cluster name)
  opencenter cluster status --quiet`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := getGlobalOptions(cmd)
			structuredOutput := opts.Output == OutputJSON || opts.Output == OutputYAML

			if syncStatus {
				return runClusterStatusSync(cmd, args, opts.DryRun, opts.Output, syncTimeout)
			}

			ctx := cmd.Context()
			requestedCluster := strings.TrimSpace("")
			if len(args) > 0 {
				requestedCluster = strings.TrimSpace(args[0])
			}

			activeCluster := ""
			if requestedCluster == "" {
				var err error
				activeCluster, err = getActiveCluster()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
			}

			clusterName := requestedCluster
			if clusterName == "" {
				clusterName = activeCluster
			}

			if clusterName == "" {
				if quiet {
					return nil
				}

				clusters, listErr := listClusters(ctx)
				if structuredOutput {
					payload := clusterStatusOutput{
						Cluster:           "",
						ConfigValid:       false,
						Message:           "no active cluster set",
						AvailableClusters: clusters,
					}
					if listErr != nil {
						payload.Message = fmt.Sprintf("no active cluster set; failed to list clusters: %v", listErr)
					}
					return writeStructuredOutput(cmd, opts.Output, payload)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "No active cluster set\n\n")

				if listErr == nil && len(clusters) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Available clusters:\n")
					for _, cluster := range clusters {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", cluster)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'opencenter cluster use <name>' to set an active cluster\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No clusters found. Use 'opencenter cluster init <name>' to create one.\n")
				}
				return nil
			}

			if quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", clusterName)
				return nil
			}

			cfg, err := loadConfig(ctx, clusterName)
			if err != nil {
				if structuredOutput {
					return writeStructuredOutput(cmd, opts.Output, clusterStatusOutput{
						Cluster:     clusterName,
						ConfigValid: false,
						Message:     "configuration not found or invalid",
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", clusterName)
				fmt.Fprintf(cmd.OutOrStdout(), "Status: Configuration not found or invalid\n")
				return nil
			}

			pathResolver := paths.NewPathResolver(config.ResolveClustersDir())
			resolvedClusterPaths, _ := pathResolver.Resolve(ctx, cfg.ClusterName(), cfg.OpenCenter.Meta.Organization)

			if structuredOutput {
				payload := buildClusterStatusOutput(ctx, clusterName, activeCluster, &cfg, resolvedClusterPaths, showPaths, refresh)
				return writeStructuredOutput(cmd, opts.Output, payload)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", clusterName)
			if activeCluster != "" && activeCluster == clusterName {
				fmt.Fprintf(cmd.OutOrStdout(), "  Active:       yes\n")
			} else if activeCluster != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Active:       no (active cluster: %s)\n", activeCluster)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Name:         %s\n", cfg.OpenCenter.Meta.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "  Environment:  %s\n", cfg.OpenCenter.Meta.Env)
			fmt.Fprintf(cmd.OutOrStdout(), "  Region:       %s\n", cfg.OpenCenter.Meta.Region)
			fmt.Fprintf(cmd.OutOrStdout(), "  Stage:        %s\n", displayLifecycleValue(cfg.OpenCenter.Meta.Stage))
			fmt.Fprintf(cmd.OutOrStdout(), "  Status:       %s\n", cfg.OpenCenter.Meta.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "  Organization: %s\n", cfg.OpenCenter.Meta.Organization)
			fmt.Fprintf(cmd.OutOrStdout(), "  Provider:     %s\n", cfg.OpenCenter.Infrastructure.Provider)

			if showPaths && resolvedClusterPaths != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "\nCluster Paths:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Config Directory:  %s\n", resolvedClusterPaths.ClusterDir)
				fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key:          %s\n", resolvedClusterPaths.SOPSKeyPath)
				fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Directory:  %s\n", resolvedClusterPaths.GitOpsDir)

				if _, err := os.Stat(resolvedClusterPaths.SOPSKeyPath); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key Status:   ✓ Present\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key Status:   ✗ Missing\n")
				}

				if _, err := os.Stat(resolvedClusterPaths.GitOpsDir); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Status:     ✓ Initialized\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Status:     ✗ Not initialized\n")
				}

				if _, err := os.Stat(resolvedClusterPaths.KubeconfigPath); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        ✓ Present\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        ✗ Missing\n")
				}
			}

			infraDir := clusterInfrastructureDir(&cfg)
			kubeconfigPath := ""
			if resolvedClusterPaths != nil {
				kubeconfigPath = resolvedClusterPaths.KubeconfigPath
			}
			inventory := buildClusterInventory(ctx, &cfg, infraDir, kubeconfigPath, refresh)
			renderClusterInventoryText(cmd.OutOrStdout(), inventory)

			if strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "kind") && resolvedClusterPaths != nil {
				renderedKindConfigPath := filepath.Join(resolvedClusterPaths.ClusterDir, "kind-config.yaml")
				gitOpsReady := pathExists(cfg.OpenCenter.GitOps.Repository.LocalDir)
				kindConfigReady := pathExists(renderedKindConfigPath)
				kubeconfigReady := pathExists(resolvedClusterPaths.KubeconfigPath)
				var clusterExists, apiReady bool
				var endpoint, providerError string
				if refresh {
					clusterExists, apiReady, endpoint, providerError = kindClusterStatus(ctx, &cfg, resolvedClusterPaths.KubeconfigPath)
				}
				defaultCNIStatus := "Enabled"
				if cfg.OpenCenter.Infrastructure.Kind != nil && cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
					defaultCNIStatus = "Disabled"
				}

				fmt.Fprintf(cmd.OutOrStdout(), "\nKind Status:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Config:            ✓ Present\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Default CNI:       %s\n", defaultCNIStatus)
				fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Setup:      %s\n", statusLabel(gitOpsReady, "Ready", "Not ready"))
				fmt.Fprintf(cmd.OutOrStdout(), "  kind-config.yaml:  %s\n", statusLabel(kindConfigReady, "Present", "Missing"))
				fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        %s\n", statusLabel(kubeconfigReady, "Present", "Missing"))
				if refresh {
					fmt.Fprintf(cmd.OutOrStdout(), "  Cluster Exists:    %s\n", statusLabel(clusterExists, "Present", "Missing"))
					fmt.Fprintf(cmd.OutOrStdout(), "  API Ready:         %s\n", statusLabel(apiReady, "Ready", "Not ready"))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  Cluster Exists:    skipped (use --refresh)\n")
					fmt.Fprintf(cmd.OutOrStdout(), "  API Ready:         skipped (use --refresh)\n")
				}
				if endpoint != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  API Endpoint:      %s\n", endpoint)
				}
				if providerError != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Provider Check:    %s\n", providerError)
				}
			}
			if strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "openstack") && resolvedClusterPaths != nil {
				gitOpsReady := pathExists(cfg.OpenCenter.GitOps.Repository.LocalDir)
				infraReady := pathExists(infraDir)
				kubeconfigReady := pathExists(resolvedClusterPaths.KubeconfigPath)
				var apiReady bool
				var endpoint, providerError string
				if refresh {
					apiReady, endpoint, providerError = cloudClusterStatus(ctx, resolvedClusterPaths.KubeconfigPath)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "\nOpenStack Status:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Config:            ✓ Present\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Repo:       %s\n", statusLabel(gitOpsReady, "Ready", "Not ready"))
				fmt.Fprintf(cmd.OutOrStdout(), "  Infrastructure:    %s\n", statusLabel(infraReady, "Rendered", "Missing"))
				fmt.Fprintf(cmd.OutOrStdout(), "  OpenTofu State:    %s\n", openTofuStateStatus(&cfg, infraDir))
				fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        %s\n", statusLabel(kubeconfigReady, "Present", "Missing"))
				if refresh {
					fmt.Fprintf(cmd.OutOrStdout(), "  API Ready:         %s\n", statusLabel(apiReady, "Ready", "Not ready"))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  API Ready:         skipped (use --refresh)\n")
				}
				if endpoint != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  API Endpoint:      %s\n", endpoint)
				}
				if providerError != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Provider Check:    %s\n", providerError)
				}
			}
				stage := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Stage))
			status := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Status))
			nextSteps := nextStepsForCluster(clusterName, stage, status)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext Steps:\n")
			for _, step := range nextSteps {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", step)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showPaths, "paths", false, "show cluster file paths and their status")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (just the cluster name)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "refresh live Kubernetes/provider status")
	cmd.Flags().BoolVar(&syncStatus, "sync", false, "sync service status from the live cluster into configuration")
	cmd.Flags().DurationVar(&syncTimeout, "sync-timeout", 30*time.Second, "timeout for live cluster status sync")

	return cmd
}

type clusterStatusOutput struct {
	Cluster           string            `json:"cluster" yaml:"cluster"`
	Active            bool              `json:"active" yaml:"active"`
	ActiveCluster     string            `json:"active_cluster,omitempty" yaml:"active_cluster,omitempty"`
	Name              string            `json:"name,omitempty" yaml:"name,omitempty"`
	Environment       string            `json:"environment,omitempty" yaml:"environment,omitempty"`
	Region            string            `json:"region,omitempty" yaml:"region,omitempty"`
	Stage             string            `json:"stage,omitempty" yaml:"stage,omitempty"`
	Status            string            `json:"status,omitempty" yaml:"status,omitempty"`
	Organization      string            `json:"organization,omitempty" yaml:"organization,omitempty"`
	Provider          string            `json:"provider,omitempty" yaml:"provider,omitempty"`
	ConfigValid       bool              `json:"config_valid" yaml:"config_valid"`
	Message           string            `json:"message,omitempty" yaml:"message,omitempty"`
	AvailableClusters []string          `json:"available_clusters,omitempty" yaml:"available_clusters,omitempty"`
	Paths             map[string]any    `json:"paths,omitempty" yaml:"paths,omitempty"`
	ProviderStatus    map[string]any    `json:"provider_status,omitempty" yaml:"provider_status,omitempty"`
	Inventory         *clusterInventory `json:"inventory,omitempty" yaml:"inventory,omitempty"`
	NextSteps         []string          `json:"next_steps,omitempty" yaml:"next_steps,omitempty"`
}

func buildClusterStatusOutput(ctx context.Context, clusterName, activeCluster string, cfg *v2.Config, resolvedClusterPaths *paths.ClusterPaths, showPaths, refresh bool) clusterStatusOutput {
	stage := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Stage))
	status := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Status))
	infraDir := clusterInfrastructureDir(cfg)
	kubeconfigPath := ""
	if resolvedClusterPaths != nil {
		kubeconfigPath = resolvedClusterPaths.KubeconfigPath
	}
	inventory := buildClusterInventory(ctx, cfg, infraDir, kubeconfigPath, refresh)
	output := clusterStatusOutput{
		Cluster:       clusterName,
		Active:        activeCluster != "" && activeCluster == clusterName,
		ActiveCluster: activeCluster,
		Name:          cfg.OpenCenter.Meta.Name,
		Environment:   cfg.OpenCenter.Meta.Env,
		Region:        cfg.OpenCenter.Meta.Region,
		Stage:         displayLifecycleValue(cfg.OpenCenter.Meta.Stage),
		Status:        cfg.OpenCenter.Meta.Status,
		Organization:  cfg.OpenCenter.Meta.Organization,
		Provider:      cfg.OpenCenter.Infrastructure.Provider,
		ConfigValid:   true,
		Inventory:     &inventory,
		NextSteps:     nextStepsForCluster(clusterName, stage, status),
	}

	if showPaths && resolvedClusterPaths != nil {
		output.Paths = map[string]any{
			"config_directory":   resolvedClusterPaths.ClusterDir,
			"sops_key":           resolvedClusterPaths.SOPSKeyPath,
			"gitops_directory":   resolvedClusterPaths.GitOpsDir,
			"sops_key_present":   pathExists(resolvedClusterPaths.SOPSKeyPath),
			"gitops_initialized": pathExists(resolvedClusterPaths.GitOpsDir),
			"kubeconfig_present": pathExists(resolvedClusterPaths.KubeconfigPath),
		}
	}

	if resolvedClusterPaths == nil {
		return output
	}

	switch strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) {
	case "kind":
		renderedKindConfigPath := filepath.Join(resolvedClusterPaths.ClusterDir, "kind-config.yaml")
		gitOpsReady := pathExists(cfg.OpenCenter.GitOps.Repository.LocalDir)
		kindConfigReady := pathExists(renderedKindConfigPath)
		kubeconfigReady := pathExists(resolvedClusterPaths.KubeconfigPath)
		var clusterExists, apiReady bool
		var endpoint, providerError string
		if refresh {
			clusterExists, apiReady, endpoint, providerError = kindClusterStatus(ctx, cfg, resolvedClusterPaths.KubeconfigPath)
		}
		defaultCNIStatus := "enabled"
		if cfg.OpenCenter.Infrastructure.Kind != nil && cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
			defaultCNIStatus = "disabled"
		}
		output.ProviderStatus = map[string]any{
			"provider":               "kind",
			"config_present":         true,
			"default_cni":            defaultCNIStatus,
			"gitops_setup_ready":     gitOpsReady,
			"kind_config_present":    kindConfigReady,
			"kubeconfig_present":     kubeconfigReady,
			"cluster_exists_checked": refresh,
			"cluster_exists":         clusterExists,
			"api_ready_checked":      refresh,
			"api_ready":              apiReady,
			"api_endpoint":           endpoint,
			"provider_check":         providerError,
		}
	case "openstack":
		gitOpsReady := pathExists(cfg.OpenCenter.GitOps.Repository.LocalDir)
		infraReady := pathExists(infraDir)
		kubeconfigReady := pathExists(resolvedClusterPaths.KubeconfigPath)
		var apiReady bool
		var endpoint, providerError string
		if refresh {
			apiReady, endpoint, providerError = cloudClusterStatus(ctx, resolvedClusterPaths.KubeconfigPath)
		}
		output.ProviderStatus = map[string]any{
			"provider":             "openstack",
			"config_present":       true,
			"gitops_repo_ready":    gitOpsReady,
			"infrastructure_ready": infraReady,
			"opentofu_state":       openTofuStateStatus(cfg, infraDir),
			"kubeconfig_present":   kubeconfigReady,
			"api_ready_checked":    refresh,
			"api_ready":            apiReady,
			"api_endpoint":         endpoint,
			"provider_check":       providerError,
		}
	}
	return output
}

func clusterInfrastructureDir(cfg *v2.Config) string {
	if cfg == nil {
		return ""
	}
	return filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", cfg.ClusterName())
}

func displayLifecycleValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}

	return value
}

func nextStepsForCluster(clusterName, stage, status string) []string {
	switch {
	case stage == "" || stage == config.StageInit:
		if status == config.StatusFailed {
			return []string{
				fmt.Sprintf("Review the cluster config and rerun 'opencenter cluster init %s --force' if needed", clusterName),
			}
		}
		return []string{
			fmt.Sprintf("Run 'opencenter cluster generate %s' to generate the GitOps repository", clusterName),
			fmt.Sprintf("Run 'opencenter cluster validate %s' to validate configuration", clusterName),
		}
	case stage == config.StageSetup:
		if status == config.StatusRunning {
			return []string{
				fmt.Sprintf("Wait for 'opencenter cluster generate %s' to finish", clusterName),
			}
		}
		if status == config.StatusFailed {
			return []string{
				fmt.Sprintf("Fix the generate error and rerun 'opencenter cluster generate %s'", clusterName),
			}
		}
		return []string{
			fmt.Sprintf("Run 'opencenter cluster deploy %s' to provision the cluster", clusterName),
		}
	case stage == config.StageBootstrap:
		if status == config.StatusRunning {
			return []string{
				fmt.Sprintf("Wait for 'opencenter cluster deploy %s' to finish", clusterName),
			}
		}
		if status == config.StatusFailed {
			return []string{
				fmt.Sprintf("Fix the deploy error and rerun 'opencenter cluster deploy %s'", clusterName),
			}
		}
		return []string{
			fmt.Sprintf("Run 'eval $(opencenter cluster env %s)' or set KUBECONFIG to the cluster-owned kubeconfig", clusterName),
			"Use 'kubectl' to interact with the cluster",
		}
	default:
		if status == config.StatusFailed {
			return []string{"Check cluster logs and rerun the last failed lifecycle command"}
		}
		return []string{"Check cluster documentation for next steps"}
	}
}

func statusLabel(ok bool, okLabel, missingLabel string) string {
	if ok {
		return "✓ " + okLabel
	}

	return "✗ " + missingLabel
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}

	_, err := os.Stat(path)
	return err == nil
}

func openTofuStateStatus(cfg *v2.Config, infraDir string) string {
	if cfg == nil || !cfg.OpenTofu.Enabled {
		return "✗ Disabled"
	}

	backendType := strings.ToLower(strings.TrimSpace(cfg.OpenTofu.Backend.Type))
	switch backendType {
	case "", "local":
		statePath := ""
		if cfg.OpenTofu.Backend.Local != nil {
			statePath = strings.TrimSpace(cfg.OpenTofu.Backend.Local.Path)
		}
		if statePath == "" {
			statePath = fmt.Sprintf(".opentofu-local-%s/terraform.tfstate", cfg.ClusterName())
		}
		if !filepath.IsAbs(statePath) {
			statePath = filepath.Join(infraDir, statePath)
		}
		return statusLabel(pathExists(statePath), "Present", "Missing")
	case "s3", "aws":
		if cfg.OpenTofu.Backend.S3 != nil &&
			strings.TrimSpace(cfg.OpenTofu.Backend.S3.Bucket) != "" &&
			strings.TrimSpace(cfg.OpenTofu.Backend.S3.Key) != "" {
			return "✓ Remote backend configured"
		}
		return "✗ Remote backend unconfigured"
	default:
		return fmt.Sprintf("✗ Unsupported backend (%s)", backendType)
	}
}

func cloudClusterStatus(ctx context.Context, kubeconfigPath string) (bool, string, string) {
	if !pathExists(kubeconfigPath) {
		return false, "", ""
	}

	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	runner := security.GetDefaultCommandRunner()

	clusterInfoCmd, err := runner.PrepareCommandContext(statusCtx, "kubectl", "--kubeconfig", kubeconfigPath, "cluster-info")
	if err != nil {
		return false, "", err.Error()
	}
	if err := clusterInfoCmd.Run(); err != nil {
		return false, "", err.Error()
	}

	endpointCmd, err := runner.PrepareCommandContext(statusCtx, "kubectl", "--kubeconfig", kubeconfigPath, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	if err != nil {
		return false, "", err.Error()
	}
	output, err := endpointCmd.Output()
	if err != nil {
		return false, "", err.Error()
	}

	return true, strings.TrimSpace(string(output)), ""
}

func kindClusterStatus(ctx context.Context, cfg *v2.Config, kubeconfigPath string) (bool, bool, string, string) {
	env := kindprovider.BuildEnvironment("")

	provider := kindprovider.NewProvider()
	clusterName := cfg.ClusterName()

	exists, err := provider.ClusterExists(ctx, clusterName, env)
	if err != nil {
		return false, false, "", err.Error()
	}
	if !exists || !pathExists(kubeconfigPath) {
		return exists, false, "", ""
	}

	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	ready, endpoint, err := provider.APIReady(statusCtx, kubeconfigPath)
	if err != nil {
		return true, false, "", err.Error()
	}

	return true, ready, endpoint, ""
}
